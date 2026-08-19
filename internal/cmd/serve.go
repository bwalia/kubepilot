package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/internal/dashboard"
	"github.com/kubepilot/kubepilot/internal/version"
	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/autopilot"
	"github.com/kubepilot/kubepilot/pkg/jobs"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/mcp/server"
	"github.com/kubepilot/kubepilot/pkg/observability"
	"github.com/kubepilot/kubepilot/pkg/runbooks"
	"github.com/kubepilot/kubepilot/pkg/telemetry"
)

// newServeCmd returns the 'serve' subcommand which starts the full
// all-in-one KubePilot binary: MCP server + AI engine + dashboard.
func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the KubePilot server (MCP + AI engine + dashboard)",
		Long: `Start all KubePilot components in a single binary:
  • MCP server for multi-cluster agent coordination
  • AI engine for natural language cluster management
  • Kubernetes Troubleshooting dashboard (AI-powered) on localhost:8383`,
		RunE: runServe,
	}

	cmd.Flags().Int("mcp-port", 9090, "MCP server port")
	cmd.Flags().Int("dashboard-port", 8383, "Dashboard port (Kubernetes Troubleshooting UI)")
	cmd.Flags().String("kubeconfig", "", "path to kubeconfig (defaults to in-cluster config)")
	cmd.Flags().String("ollama-base-url", "", "Ollama API base URL (or set KUBEPILOT_OLLAMA_BASE_URL, default: http://localhost:11434/v1)")
	cmd.Flags().String("ollama-api-key", "", "Ollama API key — optional, only needed if Ollama is behind an auth proxy (or set KUBEPILOT_OLLAMA_API_KEY)")
	cmd.Flags().String("ollama-model", "", "Ollama model name, e.g. llama3, mistral, codellama (default: llama3)")
	cmd.Flags().String("prometheus-url", "", "Prometheus server URL for metrics-based anomaly detection (optional)")
	cmd.Flags().Bool("dashboard-auth-enabled", false, "Enable dashboard/API auth middleware")
	cmd.Flags().String("dashboard-auth-token", "", "Bearer token for dashboard/API auth (optional if username/password is set)")
	cmd.Flags().String("dashboard-auth-username", "", "Basic auth username for dashboard/API auth")
	cmd.Flags().String("dashboard-auth-password", "", "Basic auth password for dashboard/API auth")
	cmd.Flags().Bool("enable-kubeconfig-mutations", false, "Enable kubeconfig mutation endpoints (upload/switch/add)")
	cmd.Flags().Bool("enable-action-mutations", false, "Enable action mutation endpoints (execute-action/remediate)")
	cmd.Flags().String("cors-allowed-origins", "http://localhost:8383,http://127.0.0.1:8383", "Comma-separated CORS allowed origins")
	cmd.Flags().String("runbooks-dir", "", "Directory containing user runbook YAML files (watched for changes; auto-created if missing)")
	cmd.Flags().String("state-dir", "", "Directory to persist RCA reports + anomalies to SQLite (state.db). Empty = in-memory only")
	cmd.Flags().Duration("state-retention", 30*24*time.Hour, "Retention window for persisted RCA history (0 = keep forever)")
	cmd.Flags().String("slack-webhook-url", "", "Slack incoming webhook URL for high-severity anomaly notifications")
	cmd.Flags().String("slack-min-severity", "high", "Minimum severity to notify on (critical|high|medium|low|info)")
	cmd.Flags().String("notifier-dashboard-url", "", "Public dashboard URL included in Slack messages (e.g. https://kubepilot.example.com)")

	// OpenTelemetry — traces and metrics. Export is off until an endpoint is
	// set; the local Prometheus endpoint is on by default and costs nothing
	// until something scrapes it.
	cmd.Flags().String("otel-endpoint", "", "OTLP collector endpoint, e.g. otel-collector:4317 (empty disables trace/metric export)")
	cmd.Flags().String("otel-protocol", "grpc", "OTLP protocol: grpc | http/protobuf")
	cmd.Flags().Bool("otel-insecure", true, "Send OTLP without TLS (typical for a collector on a trusted LAN or in-cluster)")
	cmd.Flags().String("otel-headers", "", "Extra OTLP headers as comma-separated key=value pairs (e.g. auth tokens)")
	cmd.Flags().Float64("otel-sample-ratio", 1.0, "Head-sampling probability for root spans (0.0-1.0)")
	cmd.Flags().Duration("otel-metric-interval", 60*time.Second, "How often metrics are pushed over OTLP")
	cmd.Flags().String("otel-service-name", "kubepilot", "Value reported as service.name")
	cmd.Flags().String("otel-service-namespace", "", "Value reported as service.namespace, e.g. home-lab")
	cmd.Flags().String("otel-environment", "", "Value reported as deployment.environment, e.g. int | test | prod")
	cmd.Flags().Bool("metrics-enabled", true, "Expose Prometheus metrics on the dashboard's /metrics route")
	cmd.Flags().Bool("metrics-require-auth", false, "Require dashboard credentials to scrape /metrics")

	// Autopilot — closed-loop AI self-healing.
	cmd.Flags().String("autopilot-mode", "off", "Autopilot self-healing mode: off | dry-run | active (active mutates the cluster)")
	cmd.Flags().Float64("autopilot-min-confidence", 0.80, "Minimum RCA confidence (0.0-1.0) before autopilot may act")
	cmd.Flags().String("autopilot-allowed-actions", "delete_pod,restart", "Comma-separated remediation actions autopilot may auto-apply")
	cmd.Flags().String("autopilot-max-risk", "safe", "Highest remediation risk autopilot may auto-apply: safe | moderate | high")
	cmd.Flags().String("autopilot-allowed-namespaces", "", "Comma-separated namespaces autopilot may act in (empty = all except blocked)")
	cmd.Flags().String("autopilot-blocked-namespaces", "kube-system,kube-public,kube-node-lease,kubepilot-system", "Comma-separated namespaces autopilot must never touch")
	cmd.Flags().Duration("autopilot-cooldown", 10*time.Minute, "Minimum time between autopilot actions on the same resource")
	cmd.Flags().Int("autopilot-max-actions-per-hour", 10, "Maximum autopilot actions in any rolling hour (blast-radius cap)")

	_ = viper.BindPFlag("mcp_port", cmd.Flags().Lookup("mcp-port"))
	_ = viper.BindPFlag("dashboard_port", cmd.Flags().Lookup("dashboard-port"))
	_ = viper.BindPFlag("kubeconfig", cmd.Flags().Lookup("kubeconfig"))
	_ = viper.BindPFlag("ollama_base_url", cmd.Flags().Lookup("ollama-base-url"))
	_ = viper.BindPFlag("ollama_api_key", cmd.Flags().Lookup("ollama-api-key"))
	_ = viper.BindPFlag("ollama_model", cmd.Flags().Lookup("ollama-model"))
	_ = viper.BindPFlag("prometheus_url", cmd.Flags().Lookup("prometheus-url"))
	_ = viper.BindPFlag("dashboard_auth_enabled", cmd.Flags().Lookup("dashboard-auth-enabled"))
	_ = viper.BindPFlag("dashboard_auth_token", cmd.Flags().Lookup("dashboard-auth-token"))
	_ = viper.BindPFlag("dashboard_auth_username", cmd.Flags().Lookup("dashboard-auth-username"))
	_ = viper.BindPFlag("dashboard_auth_password", cmd.Flags().Lookup("dashboard-auth-password"))
	_ = viper.BindPFlag("enable_kubeconfig_mutations", cmd.Flags().Lookup("enable-kubeconfig-mutations"))
	_ = viper.BindPFlag("enable_action_mutations", cmd.Flags().Lookup("enable-action-mutations"))
	_ = viper.BindPFlag("cors_allowed_origins", cmd.Flags().Lookup("cors-allowed-origins"))
	_ = viper.BindPFlag("runbooks_dir", cmd.Flags().Lookup("runbooks-dir"))
	_ = viper.BindPFlag("state_dir", cmd.Flags().Lookup("state-dir"))
	_ = viper.BindPFlag("state_retention", cmd.Flags().Lookup("state-retention"))
	_ = viper.BindPFlag("slack_webhook_url", cmd.Flags().Lookup("slack-webhook-url"))
	_ = viper.BindPFlag("slack_min_severity", cmd.Flags().Lookup("slack-min-severity"))
	_ = viper.BindPFlag("notifier_dashboard_url", cmd.Flags().Lookup("notifier-dashboard-url"))
	_ = viper.BindPFlag("otel_endpoint", cmd.Flags().Lookup("otel-endpoint"))
	_ = viper.BindPFlag("otel_protocol", cmd.Flags().Lookup("otel-protocol"))
	_ = viper.BindPFlag("otel_insecure", cmd.Flags().Lookup("otel-insecure"))
	_ = viper.BindPFlag("otel_headers", cmd.Flags().Lookup("otel-headers"))
	_ = viper.BindPFlag("otel_sample_ratio", cmd.Flags().Lookup("otel-sample-ratio"))
	_ = viper.BindPFlag("otel_metric_interval", cmd.Flags().Lookup("otel-metric-interval"))
	_ = viper.BindPFlag("otel_service_name", cmd.Flags().Lookup("otel-service-name"))
	_ = viper.BindPFlag("otel_service_namespace", cmd.Flags().Lookup("otel-service-namespace"))
	_ = viper.BindPFlag("otel_environment", cmd.Flags().Lookup("otel-environment"))
	_ = viper.BindPFlag("metrics_enabled", cmd.Flags().Lookup("metrics-enabled"))
	_ = viper.BindPFlag("metrics_require_auth", cmd.Flags().Lookup("metrics-require-auth"))
	_ = viper.BindPFlag("autopilot_mode", cmd.Flags().Lookup("autopilot-mode"))
	_ = viper.BindPFlag("autopilot_min_confidence", cmd.Flags().Lookup("autopilot-min-confidence"))
	_ = viper.BindPFlag("autopilot_allowed_actions", cmd.Flags().Lookup("autopilot-allowed-actions"))
	_ = viper.BindPFlag("autopilot_max_risk", cmd.Flags().Lookup("autopilot-max-risk"))
	_ = viper.BindPFlag("autopilot_allowed_namespaces", cmd.Flags().Lookup("autopilot-allowed-namespaces"))
	_ = viper.BindPFlag("autopilot_blocked_namespaces", cmd.Flags().Lookup("autopilot-blocked-namespaces"))
	_ = viper.BindPFlag("autopilot_cooldown", cmd.Flags().Lookup("autopilot-cooldown"))
	_ = viper.BindPFlag("autopilot_max_actions_per_hour", cmd.Flags().Lookup("autopilot-max-actions-per-hour"))

	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("Starting KubePilot",
		zap.String("version", version.Version),
		zap.String("build", version.BuildTime),
		zap.String("commit", version.Commit),
	)

	// Telemetry comes up before anything else so that every later subsystem —
	// the Kubernetes client, the AI engine, the watcher — is instrumented from
	// its first call.
	tel, err := telemetry.Init(ctx, telemetryConfig(), log)
	if err != nil {
		// Never let a telemetry misconfiguration stop the server from serving.
		// An unreachable collector is an observability problem, not an outage.
		log.Warn("Telemetry disabled: initialisation failed", zap.Error(err))
		tel = telemetry.Default()
	}
	defer func() {
		if err := tel.Shutdown(context.Background()); err != nil {
			log.Warn("Telemetry shutdown error", zap.Error(err))
		}
	}()

	// Instrument every Kubernetes client built from here on, including the ones
	// the dashboard creates later when the operator switches cluster.
	k8s.SetTransportWrapper(tel.WrapKubernetesTransport)

	// Resolve kubeconfig from this command's own flag. We can't use
	// viper.GetString("kubeconfig") here: the serve/agent/operator subcommands
	// all BindPFlag the same global "kubeconfig" key, so the last binding wins
	// and shadows serve's flag. Fall back to the standard KUBECONFIG env var.
	kubeconfigPath, _ := cmd.Flags().GetString("kubeconfig")
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
	}

	// Build Kubernetes client.
	k8sClient, err := k8s.NewClient(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("building kubernetes client: %w", err)
	}

	// Build AI engine — Ollama API key is optional for local instances.
	aiEngine := ai.NewEngine(ai.Config{
		OllamaAPIKey:  viper.GetString("ollama_api_key"),
		OllamaBaseURL: viper.GetString("ollama_base_url"),
		Model:         viper.GetString("ollama_model"),
	}, k8sClient, log)

	// Build job scheduler.
	scheduler := jobs.NewScheduler(aiEngine, k8sClient, log)
	scheduler.Start(ctx)

	// Start continuous cluster watcher for anomaly detection and auto-RCA.
	rcaStore := observability.NewRCAStore(1000)

	// Optional persistence: SQLite-backed durability for RCA history.
	var persistence *observability.Persistence
	if stateDir := strings.TrimSpace(viper.GetString("state_dir")); stateDir != "" {
		dbPath := stateDir + "/state.db"
		p, err := observability.NewPersistence(ctx, dbPath, rcaStore, viper.GetDuration("state_retention"), log)
		if err != nil {
			log.Sugar().Warnf("RCA persistence disabled: %v", err)
		} else {
			persistence = p
			defer func() { _ = persistence.Close() }()
		}
	}

	// Optional Slack notifier for high-severity events.
	var notifier observability.AnomalyNotifier
	if slack := observability.NewSlackNotifier(observability.SlackConfig{
		WebhookURL:   viper.GetString("slack_webhook_url"),
		MinSeverity:  viper.GetString("slack_min_severity"),
		DashboardURL: viper.GetString("notifier_dashboard_url"),
	}, log); slack != nil {
		notifier = slack
		log.Info("Slack notifier enabled", zap.String("min_severity", viper.GetString("slack_min_severity")))
	}

	// Build the autopilot self-healing controller. It is the closed loop that
	// turns AI diagnoses into safe, policy-gated remediation. Off unless the
	// operator opts in via --autopilot-mode. The executor runs with DryRun=false;
	// the controller's own Mode decides whether a step is actually applied. The
	// nil guard is safe because autopilot only ever runs steps that do NOT require
	// a CR code (CR-gated steps are escalated to a human instead).
	autopilotPolicy := autopilot.Policy{
		Mode:              autopilot.Mode(strings.TrimSpace(viper.GetString("autopilot_mode"))),
		MinConfidence:     viper.GetFloat64("autopilot_min_confidence"),
		AllowedActions:    parseCSV(viper.GetString("autopilot_allowed_actions")),
		MaxRisk:           strings.TrimSpace(viper.GetString("autopilot_max_risk")),
		AllowedNamespaces: parseCSV(viper.GetString("autopilot_allowed_namespaces")),
		BlockedNamespaces: parseCSV(viper.GetString("autopilot_blocked_namespaces")),
		Cooldown:          viper.GetDuration("autopilot_cooldown"),
		MaxActionsPerHour: viper.GetInt("autopilot_max_actions_per_hour"),
	}
	if autopilotPolicy.Mode == "" {
		autopilotPolicy.Mode = autopilot.ModeOff
	}
	autopilotCtl := autopilot.New(autopilot.Config{
		Policy: autopilotPolicy,
		Executor: ai.NewRemediationExecutor(k8sClient, nil, ai.RemediationConfig{
			DryRun: false,
		}, log),
	}, log)
	if autopilotPolicy.Mode != autopilot.ModeOff {
		log.Info("Autopilot self-healing enabled",
			zap.String("mode", string(autopilotPolicy.Mode)),
			zap.Float64("min_confidence", autopilotPolicy.MinConfidence),
			zap.Strings("allowed_actions", autopilotPolicy.AllowedActions),
		)
	}

	watcher := observability.NewClusterWatcher(k8sClient, aiEngine.RCA(), rcaStore, observability.WatcherConfig{
		Persistence: persistence,
		Notifier:    notifier,
		ReportHook:  autopilotCtl.HandleReport,
	}, log)
	go watcher.Start(ctx)

	// Build runbook execution engine and optionally hot-reload user runbooks
	// from a directory of YAML files.
	runbookEngine := runbooks.NewEngine(k8sClient, aiEngine, log)
	if runbooksDir := strings.TrimSpace(viper.GetString("runbooks_dir")); runbooksDir != "" {
		if err := runbooks.WatchDir(ctx, runbooksDir, runbookEngine, log); err != nil {
			log.Sugar().Warnf("Runbook directory watcher failed: %v", err)
		}
	}

	// Start MCP server.
	mcpServer := server.New(server.Config{
		Port:      viper.GetInt("mcp_port"),
		AIEngine:  aiEngine,
		Scheduler: scheduler,
		K8sClient: k8sClient,
	}, log)

	go func() {
		if err := mcpServer.Start(ctx); err != nil {
			log.Sugar().Errorf("MCP server error: %v", err)
		}
	}()

	// Start dashboard server.
	authEnabled := viper.GetBool("dashboard_auth_enabled")
	authToken := strings.TrimSpace(viper.GetString("dashboard_auth_token"))
	authUsername := strings.TrimSpace(viper.GetString("dashboard_auth_username"))
	authPassword := strings.TrimSpace(viper.GetString("dashboard_auth_password"))
	if authEnabled {
		hasBearer := authToken != ""
		hasBasic := authUsername != "" && authPassword != ""
		if !hasBearer && !hasBasic {
			return fmt.Errorf("dashboard auth is enabled but no credentials were configured; set dashboard_auth_token or dashboard_auth_username+dashboard_auth_password")
		}
	}

	dashServer := dashboard.NewServer(dashboard.Config{
		Port:           viper.GetInt("dashboard_port"),
		AIEngine:       aiEngine,
		Scheduler:      scheduler,
		K8sClient:      k8sClient,
		RCAStore:       rcaStore,
		RunbookEngine:  runbookEngine,
		Autopilot:      autopilotCtl,
		KubeconfigPath: kubeconfigPath,
		Auth: dashboard.AuthConfig{
			Enabled:            authEnabled,
			Token:              authToken,
			Username:           authUsername,
			Password:           authPassword,
			MetricsRequireAuth: viper.GetBool("metrics_require_auth"),
		},
		EnableKubeconfigMutationEndpoints: viper.GetBool("enable_kubeconfig_mutations"),
		EnableActionMutationEndpoints:     viper.GetBool("enable_action_mutations"),
		AllowedCORSOrigins:                parseCSV(viper.GetString("cors_allowed_origins")),
		Telemetry:                         tel,
	}, log)

	go func() {
		if err := dashServer.Start(ctx); err != nil {
			log.Sugar().Errorf("Dashboard server error: %v", err)
		}
	}()

	log.Sugar().Infof("KubePilot running — dashboard: http://localhost:%d | MCP: :%d",
		viper.GetInt("dashboard_port"), viper.GetInt("mcp_port"))

	<-ctx.Done()
	log.Info("KubePilot shutting down")
	return nil
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

// telemetryConfig assembles the telemetry settings from viper.
//
// KubePilot's own KUBEPILOT_OTEL_* settings take precedence, falling back to
// the standard OTEL_* environment variables. Honouring both matters because
// sidecars and operators inject the standard names automatically, while the
// systemd and launchd units here configure everything through KUBEPILOT_*.
func telemetryConfig() telemetry.Config {
	endpoint := strings.TrimSpace(viper.GetString("otel_endpoint"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}

	protocol := strings.TrimSpace(viper.GetString("otel_protocol"))
	if env := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")); env != "" && !viper.IsSet("otel_protocol") {
		protocol = env
	}

	serviceName := strings.TrimSpace(viper.GetString("otel_service_name"))
	if env := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); env != "" {
		serviceName = env
	}

	return telemetry.Config{
		ServiceName:       serviceName,
		ServiceNamespace:  strings.TrimSpace(viper.GetString("otel_service_namespace")),
		Environment:       strings.TrimSpace(viper.GetString("otel_environment")),
		Endpoint:          endpoint,
		Protocol:          protocol,
		Insecure:          viper.GetBool("otel_insecure"),
		Headers:           parseKeyValues(viper.GetString("otel_headers")),
		SampleRatio:       viper.GetFloat64("otel_sample_ratio"),
		MetricInterval:    viper.GetDuration("otel_metric_interval"),
		PrometheusEnabled: viper.GetBool("metrics_enabled"),
	}
}

// parseKeyValues parses "k1=v1,k2=v2" into a map. Malformed pairs are skipped
// rather than rejected: a stray comma in a header list should not prevent the
// server from starting.
func parseKeyValues(raw string) map[string]string {
	pairs := parseCSV(raw)
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		k, v, ok := strings.Cut(pair, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			continue
		}
		out[k] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

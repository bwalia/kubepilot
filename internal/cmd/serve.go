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
	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/jobs"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/mcp/server"
	"github.com/kubepilot/kubepilot/pkg/observability"
	"github.com/kubepilot/kubepilot/pkg/runbooks"
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

	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("Starting KubePilot")

	// Build Kubernetes client.
	k8sClient, err := k8s.NewClient(viper.GetString("kubeconfig"))
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

	watcher := observability.NewClusterWatcher(k8sClient, aiEngine.RCA(), rcaStore, observability.WatcherConfig{
		Persistence: persistence,
		Notifier:    notifier,
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
		KubeconfigPath: viper.GetString("kubeconfig"),
		Auth: dashboard.AuthConfig{
			Enabled:  authEnabled,
			Token:    authToken,
			Username: authUsername,
			Password: authPassword,
		},
		EnableKubeconfigMutationEndpoints: viper.GetBool("enable_kubeconfig_mutations"),
		EnableActionMutationEndpoints:     viper.GetBool("enable_action_mutations"),
		AllowedCORSOrigins:                parseCSV(viper.GetString("cors_allowed_origins")),
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

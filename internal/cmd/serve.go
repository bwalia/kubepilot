package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kubepilot/kubepilot/internal/dashboard"
	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/jobs"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/mcp/server"
	"github.com/kubepilot/kubepilot/pkg/observability"
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
  • Kubernetes Cockpit dashboard on localhost:8383`,
		RunE: runServe,
	}

	cmd.Flags().Int("mcp-port", 9090, "MCP server port")
	cmd.Flags().Int("dashboard-port", 8383, "Kubernetes Cockpit dashboard port")
	cmd.Flags().String("kubeconfig", "", "path to kubeconfig (defaults to in-cluster config)")
	cmd.Flags().String("ollama-base-url", "", "Ollama API base URL (or set KUBEPILOT_OLLAMA_BASE_URL, default: http://localhost:11434/v1)")
	cmd.Flags().String("ollama-api-key", "", "Ollama API key — optional, only needed if Ollama is behind an auth proxy (or set KUBEPILOT_OLLAMA_API_KEY)")
	cmd.Flags().String("ollama-model", "", "Ollama model name, e.g. llama3, mistral, codellama (default: llama3)")
	cmd.Flags().String("prometheus-url", "", "Prometheus server URL for metrics-based anomaly detection (optional)")

	_ = viper.BindPFlag("mcp_port", cmd.Flags().Lookup("mcp-port"))
	_ = viper.BindPFlag("dashboard_port", cmd.Flags().Lookup("dashboard-port"))
	_ = viper.BindPFlag("kubeconfig", cmd.Flags().Lookup("kubeconfig"))
	_ = viper.BindPFlag("ollama_base_url", cmd.Flags().Lookup("ollama-base-url"))
	_ = viper.BindPFlag("ollama_api_key", cmd.Flags().Lookup("ollama-api-key"))
	_ = viper.BindPFlag("ollama_model", cmd.Flags().Lookup("ollama-model"))
	_ = viper.BindPFlag("prometheus_url", cmd.Flags().Lookup("prometheus-url"))

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
	watcher := observability.NewClusterWatcher(k8sClient, aiEngine.RCA(), rcaStore, observability.WatcherConfig{}, log)
	go watcher.Start(ctx)

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
	dashServer := dashboard.NewServer(dashboard.Config{
		Port:      viper.GetInt("dashboard_port"),
		AIEngine:  aiEngine,
		Scheduler: scheduler,
		K8sClient: k8sClient,
		RCAStore:  rcaStore,
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

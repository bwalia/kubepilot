package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/mcp/client"
)

// newAgentCmd returns the 'agent' subcommand that runs a lightweight
// MCP client agent connecting this cluster back to the central MCP server.
// Deploy this in remote clusters to enable multi-cluster management.
func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run a KubePilot MCP agent in a remote cluster",
		Long: `Run a lightweight KubePilot agent that:
  • Connects this cluster to a central KubePilot MCP server
  • Streams cluster events, metrics, and pod status upstream
  • Executes AI-approved actions locally after CR code validation`,
		RunE: runAgent,
	}

	cmd.Flags().String("mcp-server", "", "address of the central KubePilot MCP server (host:port)")
	cmd.Flags().String("cluster-name", "", "unique name for this cluster (required)")
	cmd.Flags().String("kubeconfig", "", "path to kubeconfig")

	_ = cmd.MarkFlagRequired("mcp-server")
	_ = cmd.MarkFlagRequired("cluster-name")

	_ = viper.BindPFlag("agent.mcp_server", cmd.Flags().Lookup("mcp-server"))
	_ = viper.BindPFlag("agent.cluster_name", cmd.Flags().Lookup("cluster-name"))
	_ = viper.BindPFlag("kubeconfig", cmd.Flags().Lookup("kubeconfig"))

	return cmd
}

func runAgent(_ *cobra.Command, _ []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	k8sClient, err := k8s.NewClient(viper.GetString("kubeconfig"))
	if err != nil {
		return err
	}

	agent := client.NewAgent(client.Config{
		MCPServerAddr: viper.GetString("agent.mcp_server"),
		ClusterName:   viper.GetString("agent.cluster_name"),
		K8sClient:     k8sClient,
	}, log)

	return agent.Connect(ctx)
}

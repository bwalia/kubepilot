package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kubepilot/kubepilot/pkg/operator"
)

// newOperatorCmd returns the 'operator' subcommand that runs the
// KubePilot Kubernetes operator for CRD lifecycle management.
func newOperatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run the KubePilot Kubernetes operator",
		Long: `Run the KubePilot operator that:
  • Reconciles KubePilotCluster, KubePilotJob, and KubePilotCRCode CRDs
  • Installs/updates KubePilot agents in registered clusters
  • Enforces CR code validation before production-impacting jobs
  • Manages multi-cluster lifecycle`,
		RunE: runOperator,
	}

	cmd.Flags().String("metrics-addr", ":8081", "address to expose operator metrics")
	cmd.Flags().String("health-probe-addr", ":8082", "address to expose health probes")
	cmd.Flags().Bool("leader-elect", true, "enable leader election for HA deployments")
	cmd.Flags().String("kubeconfig", "", "path to kubeconfig")

	_ = viper.BindPFlag("operator.metrics_addr", cmd.Flags().Lookup("metrics-addr"))
	_ = viper.BindPFlag("operator.health_probe_addr", cmd.Flags().Lookup("health-probe-addr"))
	_ = viper.BindPFlag("operator.leader_elect", cmd.Flags().Lookup("leader-elect"))
	_ = viper.BindPFlag("kubeconfig", cmd.Flags().Lookup("kubeconfig"))

	return cmd
}

func runOperator(_ *cobra.Command, _ []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mgr, err := operator.NewManager(operator.Config{
		MetricsAddr:    viper.GetString("operator.metrics_addr"),
		HealthProbeAddr: viper.GetString("operator.health_probe_addr"),
		LeaderElect:    viper.GetBool("operator.leader_elect"),
		KubeConfig:     viper.GetString("kubeconfig"),
	}, log)
	if err != nil {
		return err
	}

	return mgr.Start(ctx)
}

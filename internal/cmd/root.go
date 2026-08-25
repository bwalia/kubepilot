// Package cmd provides the CLI command tree for KubePilot.
// All subcommands (serve, agent, operator) are registered here.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/internal/utils"
	"github.com/kubepilot/kubepilot/internal/version"
)

var (
	cfgFile string
	log     *zap.Logger
)

// rootCmd is the base command — running kubepilot without subcommands prints help.
var rootCmd = &cobra.Command{
	Use:   "kubepilot",
	Short: "KubePilot – AI-driven Kubernetes autopilot",
	Long: `KubePilot is an AI-powered Kubernetes management platform that provides
automated troubleshooting, scaling, multi-agent orchestration, and production-safe
change management via CR code validation.

Run 'kubepilot serve' to start the all-in-one binary (MCP server + AI engine + dashboard).`,
}

// Execute runs the root command and propagates any errors to the caller.
func Execute() error {
	return rootCmd.Execute()
}

// newVersionCmd prints the build provenance (version, build number, commit).
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, build number, and commit",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
		},
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Enables `kubepilot --version`. Cobra prefixes "kubepilot version ", so keep
	// this concise rather than reusing version.String() (which repeats the name).
	rootCmd.Version = fmt.Sprintf("%s (build %s, commit %s)", version.Version, version.BuildTime, version.Commit)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.kubepilot.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level: debug | info | warn | error")
	_ = viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))

	// Register subcommands.
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newOperatorCmd())
	rootCmd.AddCommand(newTroubleshootCmd())
	rootCmd.AddCommand(newRCACmd())
	rootCmd.AddCommand(newWatchCmd())
	rootCmd.AddCommand(newNodeAgentCmd())
	rootCmd.AddCommand(newVersionCmd())
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.kubepilot")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("KUBEPILOT")

	if err := viper.ReadInConfig(); err == nil {
		// Config file found and loaded; errors are non-fatal (defaults apply).
		_ = err
	}

	log = utils.NewLogger(viper.GetString("log_level"))
}

// Package cmd provides the CLI command tree for KubePilot.
// All subcommands (serve, agent, operator) are registered here.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/internal/utils"
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

func init() {
	cobra.OnInitialize(initConfig)

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

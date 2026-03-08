package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

func newTroubleshootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "troubleshoot <namespace/pod>",
		Short: "Run AI-powered root cause analysis on a pod",
		Long: `Perform deep troubleshooting on a specific pod using AI-powered
root cause analysis. Collects pod diagnostics, events, logs, and
resource metrics, then sends them to the LLM for structured analysis.

Example:
  kubepilot troubleshoot default/nginx-abc123
  kubepilot troubleshoot awx/awx-operator-controller-manager-xxx`,
		Args: cobra.ExactArgs(1),
		RunE: runTroubleshoot,
	}

	cmd.Flags().Bool("json", false, "Output as JSON")
	return cmd
}

func runTroubleshoot(cmd *cobra.Command, args []string) error {
	parts := strings.SplitN(args[0], "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected format: namespace/pod-name, got: %q", args[0])
	}
	namespace, podName := parts[0], parts[1]

	ctx := cmd.Context()

	k8sClient, err := k8s.NewClient(viper.GetString("kubeconfig"))
	if err != nil {
		return fmt.Errorf("building kubernetes client: %w", err)
	}

	aiEngine := ai.NewEngine(ai.Config{
		OllamaAPIKey:  viper.GetString("ollama_api_key"),
		OllamaBaseURL: viper.GetString("ollama_base_url"),
		Model:         viper.GetString("ollama_model"),
	}, k8sClient, log)

	jsonOutput, _ := cmd.Flags().GetBool("json")

	fmt.Fprintf(os.Stderr, "Analyzing pod %s/%s...\n", namespace, podName)

	report, err := aiEngine.TroubleshootPod(ctx, namespace, podName)
	if err != nil {
		return fmt.Errorf("troubleshooting failed: %w", err)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Pretty-print the report.
	fmt.Printf("\n=== RCA Report: %s/%s ===\n", namespace, podName)
	fmt.Printf("Root Cause: %s\n", report.RootCause)
	fmt.Printf("Analysis:   %s\n", report.Analysis)

	if report.RCA != nil {
		fmt.Printf("Severity:   %s\n", report.RCA.Severity)
		fmt.Printf("Category:   %s\n", report.RCA.RootCause.Category)
		fmt.Printf("Confidence: %.0f%%\n", report.RCA.Confidence*100)

		if len(report.RCA.EvidenceChain) > 0 {
			fmt.Printf("\nEvidence:\n")
			for _, e := range report.RCA.EvidenceChain {
				fmt.Printf("  [%s] %s\n", e.Source, e.Relevance)
			}
		}

		if len(report.RCA.Remediation) > 0 {
			fmt.Printf("\nRemediation Steps:\n")
			for _, r := range report.RCA.Remediation {
				crFlag := ""
				if r.RequiresCR {
					crFlag = " [CR CODE REQUIRED]"
				}
				fmt.Printf("  %d. [%s] %s (risk: %s)%s\n",
					r.Order, r.Action, r.Description, r.Risk, crFlag)
			}
		}
	}

	if len(report.Actions) > 0 {
		fmt.Printf("\nSuggested Actions:\n")
		for i, a := range report.Actions {
			fmt.Printf("  %d. [%s] %s\n", i+1, a.Type, a.Explanation)
		}
	}

	return nil
}

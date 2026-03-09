package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newRCACmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rca",
		Short: "Manage and query RCA reports",
		Long:  `List, get, and search Root Cause Analysis reports stored by the KubePilot server.`,
	}

	cmd.AddCommand(newRCAListCmd())
	cmd.AddCommand(newRCAGetCmd())
	return cmd
}

func newRCAListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent RCA reports",
		RunE:  runRCAList,
	}
	cmd.Flags().String("severity", "", "Filter by severity (critical, high, medium, low)")
	cmd.Flags().String("namespace", "", "Filter by namespace")
	cmd.Flags().String("since", "24h", "Show reports since duration (e.g. 1h, 24h, 7d)")
	cmd.Flags().String("server", "http://localhost:8383", "KubePilot server URL")
	return cmd
}

func newRCAGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <report-id>",
		Short: "Get a specific RCA report by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runRCAGet,
	}
}

func runRCAList(cmd *cobra.Command, _ []string) error {
	serverURL, _ := cmd.Flags().GetString("server")
	severity, _ := cmd.Flags().GetString("severity")
	namespace, _ := cmd.Flags().GetString("namespace")
	sinceStr, _ := cmd.Flags().GetString("since")

	url := fmt.Sprintf("%s/api/v1/rca?severity=%s&namespace=%s&since=%s",
		serverURL, severity, namespace, sinceStr)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("connecting to KubePilot server: %w", err)
	}
	defer resp.Body.Close()

	var reports []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&reports); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if len(reports) == 0 {
		fmt.Println("No RCA reports found.")
		return nil
	}

	fmt.Printf("%-30s %-10s %-15s %-12s %s\n", "ID", "SEVERITY", "CATEGORY", "CONFIDENCE", "TIMESTAMP")
	fmt.Println("-----------------------------------------------------------------------------------------------")
	for _, r := range reports {
		id, _ := r["id"].(string)
		sev, _ := r["severity"].(string)
		rootCause, _ := r["root_cause"].(map[string]interface{})
		category := ""
		if rootCause != nil {
			category, _ = rootCause["category"].(string)
		}
		confidence, _ := r["confidence"].(float64)
		ts, _ := r["timestamp"].(string)
		parsedTS, _ := time.Parse(time.RFC3339, ts)

		fmt.Printf("%-30s %-10s %-15s %-12.0f%% %s\n",
			truncateStr(id, 30), sev, category, confidence*100, parsedTS.Format("2006-01-02 15:04"))
	}

	return nil
}

func runRCAGet(cmd *cobra.Command, args []string) error {
	serverURL := "http://localhost:8383"
	url := fmt.Sprintf("%s/api/v1/rca/%s", serverURL, args[0])

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("connecting to KubePilot server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("RCA report %q not found", args[0])
	}

	var report map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

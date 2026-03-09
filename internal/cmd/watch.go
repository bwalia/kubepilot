package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch cluster anomalies in real-time",
		Long: `Tail anomalies detected by the KubePilot cluster watcher.
Polls the server for new anomalies and displays them as they appear.

Example:
  kubepilot watch
  kubepilot watch --namespace=production --interval=10s`,
		RunE: runWatch,
	}

	cmd.Flags().String("namespace", "", "Filter by namespace")
	cmd.Flags().Duration("interval", 5*time.Second, "Poll interval")
	cmd.Flags().String("server", "http://localhost:8383", "KubePilot server URL")
	return cmd
}

func runWatch(cmd *cobra.Command, _ []string) error {
	serverURL, _ := cmd.Flags().GetString("server")
	namespace, _ := cmd.Flags().GetString("namespace")
	interval, _ := cmd.Flags().GetDuration("interval")

	fmt.Printf("Watching for anomalies (polling every %s)...\n", interval)
	fmt.Printf("%-20s %-10s %-15s %-12s %s\n", "TIMESTAMP", "SEVERITY", "RESOURCE", "RULE", "DESCRIPTION")
	fmt.Println("-------------------------------------------------------------------------------------------")

	lastSeen := time.Now().Add(-1 * time.Minute) // Start with anomalies from the last minute.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cmd.Context().Done():
			return nil
		case <-ticker.C:
			url := fmt.Sprintf("%s/api/v1/anomalies?namespace=%s&since=%s",
				serverURL, namespace, lastSeen.Format(time.RFC3339))

			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("[ERROR] %v\n", err)
				continue
			}

			var anomalies []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&anomalies); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			for _, a := range anomalies {
				ts, _ := a["detected_at"].(string)
				parsedTS, _ := time.Parse(time.RFC3339, ts)
				if !parsedTS.After(lastSeen) {
					continue
				}

				severity, _ := a["severity"].(string)
				resource, _ := a["resource"].(map[string]interface{})
				resourceName := ""
				if resource != nil {
					ns, _ := resource["namespace"].(string)
					name, _ := resource["name"].(string)
					if ns != "" {
						resourceName = ns + "/" + name
					} else {
						resourceName = name
					}
				}
				rule, _ := a["rule"].(string)
				desc, _ := a["description"].(string)

				fmt.Printf("%-20s %-10s %-15s %-12s %s\n",
					parsedTS.Format("15:04:05"),
					severity,
					truncateStr(resourceName, 15),
					truncateStr(rule, 12),
					truncateStr(desc, 60),
				)

				if parsedTS.After(lastSeen) {
					lastSeen = parsedTS
				}
			}
		}
	}
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kubepilot/kubepilot/pkg/hostinfo"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Annotation and label keys the agent owns. The dashboard reads them back:
// hardware and serial are shown next to the node name, everything under the
// hwPrefix fills the node's Hardware tab, and lanIPLabel fixes the LAN address
// on WireGuard-meshed clusters where Kubernetes only knows the tunnel IP.
const (
	hardwareAnnotation = k8s.AnnotationHardware
	serialAnnotation   = k8s.AnnotationSerial
	hwPrefix           = k8s.AnnotationHWPrefix
	lanIPLabel         = k8s.LabelLANIP
)

// newNodeAgentCmd returns the 'node-agent' subcommand: a DaemonSet-shaped loop
// that publishes the physical facts about the machine it runs on onto its own
// Node object.
//
// It exists because Kubernetes has no idea what hardware a node is. It reports
// the OS and kernel and stops there, so a rack of identically-named workers is
// indistinguishable from the API. The agent reads DMI, /proc and /sys — the
// same sources dmidecode and lshw use — and writes them back as annotations,
// needing no packages on the host and no SSH access.
func newNodeAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node-agent",
		Short: "Publish this node's hardware facts as node annotations",
		Long: `Run the KubePilot node agent (one pod per node, as a DaemonSet).

Each pass it:
  • reads vendor, model, serial, BIOS, chassis, CPU, memory, disks and NICs
    from the host's /sys and /proc (mounted read-only into the pod)
  • detects the node's real LAN IP, which on a WireGuard mesh is not the
    address Kubernetes knows about
  • patches its own Node object with the results

Nothing is installed on the host and nothing is read that a user with root on
the box could not read with dmidecode.`,
		RunE: runNodeAgent,
	}

	cmd.Flags().String("node", os.Getenv("NODE_NAME"), "name of the node to annotate (defaults to $NODE_NAME)")
	cmd.Flags().Duration("interval", 15*time.Minute, "how often to re-publish; facts self-heal if the node object is edited")
	cmd.Flags().String("sysfs", "/host/sys", "path to the host's sysfs")
	cmd.Flags().String("procfs", "/host/proc", "path to the host's procfs")
	cmd.Flags().String("kubeconfig", "", "path to kubeconfig (defaults to in-cluster config)")
	cmd.Flags().Bool("once", false, "publish a single time and exit")
	return cmd
}

func runNodeAgent(cmd *cobra.Command, _ []string) error {
	nodeName, _ := cmd.Flags().GetString("node")
	interval, _ := cmd.Flags().GetDuration("interval")
	sysfs, _ := cmd.Flags().GetString("sysfs")
	procfs, _ := cmd.Flags().GetString("procfs")
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	once, _ := cmd.Flags().GetBool("once")

	if nodeName == "" {
		return fmt.Errorf("node name is required: pass --node or set NODE_NAME (the DaemonSet sets it from spec.nodeName)")
	}

	client, err := k8s.NewClient(kubeconfig)
	if err != nil {
		return fmt.Errorf("connecting to the cluster: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for {
		if err := publishNodeFacts(ctx, client, nodeName, sysfs, procfs); err != nil {
			// A failed pass is not fatal: the API server may be rolling, or the
			// node may have just been cordoned. The next tick retries.
			log.Warn("publishing node facts failed", zap.String("node", nodeName), zap.Error(err))
		}
		if once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

// publishNodeFacts collects the hardware facts and merge-patches them onto the
// node. A merge patch only touches the keys it names, so labels and annotations
// set by anything else — including a hand-written kubepilot.io/hardware
// override on a machine with unreadable DMI — are left alone.
func publishNodeFacts(ctx context.Context, client *k8s.Client, nodeName, sysfs, procfs string) error {
	facts := hostinfo.Collect(sysfs, procfs)
	annotations := map[string]string{}
	for key, value := range facts {
		switch key {
		case hostinfo.KeyHardware:
			annotations[hardwareAnnotation] = value
		case hostinfo.KeySerial:
			annotations[serialAnnotation] = value
		default:
			annotations[hwPrefix+key] = value
		}
	}

	labels := map[string]string{}
	if ip := detectLANIP(); ip != "" {
		labels[lanIPLabel] = ip
	}

	if len(annotations) == 0 && len(labels) == 0 {
		return fmt.Errorf("no hardware facts readable under %s and %s", sysfs, procfs)
	}

	metadata := map[string]any{}
	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}
	if len(labels) > 0 {
		metadata["labels"] = labels
	}
	patch, err := json.Marshal(map[string]any{"metadata": metadata})
	if err != nil {
		return fmt.Errorf("building node patch: %w", err)
	}

	if _, err := client.Core.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patching node %s: %w", nodeName, err)
	}

	log.Info("published node facts",
		zap.String("node", nodeName),
		zap.String("hardware", facts[hostinfo.KeyHardware]),
		zap.String("lan_ip", labels[lanIPLabel]),
		zap.String("facts", summarise(facts)))
	return nil
}

// detectLANIP returns the address the host would use to reach the internet —
// the physical LAN IP, not the WireGuard or flannel overlay address. Opening a
// UDP socket sends no packets; it just asks the kernel's routing table which
// source address a default-route packet would carry, which is what
// `ip route get 1.1.1.1` prints. It needs no iproute2 in the image.
//
// Only private addresses are returned: on a cloud node the answer is the public
// IP, which Kubernetes already reports as an ExternalIP.
func detectLANIP() string {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil || !addr.IP.IsPrivate() {
		return ""
	}
	return addr.IP.String()
}

// summarise renders the collected facts as a stable, greppable log line.
func summarise(facts hostinfo.Facts) string {
	keys := make([]string, 0, len(facts))
	for key := range facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+facts[key])
	}
	return strings.Join(parts, " | ")
}

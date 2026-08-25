// Package hostinfo reads the physical facts about the machine a node runs on —
// vendor, model, serial, BIOS, CPU, memory, disks, NICs — straight from sysfs
// and procfs.
//
// Kubernetes never looks at any of this: status.nodeInfo stops at the OS and
// kernel, so a cluster of identical hostnames gives no clue which physical box
// is which. KubePilot's node agent collects these facts on every node and
// publishes them as node annotations, which the dashboard then shows.
package hostinfo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Facts is the collected hardware description, keyed by annotation suffix
// ("cpu", "memory", …). Empty values are never stored — a missing key means
// "this machine doesn't report it", which the dashboard renders as absent
// rather than as a blank row.
type Facts map[string]string

// Well-known keys. Hardware and Serial get their own top-level annotations
// (kubepilot.io/hardware, kubepilot.io/serial) because the dashboard shows them
// next to the node name; everything else lands under kubepilot.io/hw.<key>.
const (
	KeyHardware = "hardware"
	KeySerial   = "serial"
)

// Collect reads every fact it can from the given sysfs and procfs roots. In a
// pod these are the host's /sys and /proc mounted read-only (see the node-agent
// DaemonSet); in tests they are a fixture directory.
//
// Unreadable files are skipped rather than failing the whole collection: a
// Raspberry Pi has no DMI at all, a VM has no disk model, and an unprivileged
// read of product_serial returns nothing. Partial hardware detail beats none.
func Collect(sysRoot, procRoot string) Facts {
	f := Facts{}
	dmi := func(name string) string { return readDMI(filepath.Join(sysRoot, "class/dmi/id", name)) }

	vendor, model := dmi("sys_vendor"), dmi("product_name")
	// product_version is a SKU code on some vendors ("SBKPF") and the friendly
	// name on others (Lenovo puts "ThinkPad X1 Carbon 6th" there), so it gets
	// its own row instead of being glued onto the model line.
	hardware := dedupePrefix(vendor, model)
	if hardware == "" {
		// ARM boards (Raspberry Pi and friends) have no DMI, only a device tree.
		hardware = readFirstLine(filepath.Join(sysRoot, "firmware/devicetree/base/model"))
	}
	f.set(KeyHardware, hardware)
	f.set(KeySerial, firstNonEmpty(dmi("product_serial"), dmi("board_serial")))
	f.set("version", dmi("product_version"))
	// board_version is usually the keyboard-controller firmware revision, which
	// nobody wants on screen; the board's vendor and name are the useful part.
	f.set("board", dedupePrefix(dmi("board_vendor"), dmi("board_name")))
	f.set("bios", joinNonEmpty(" ", dedupePrefix(dmi("bios_vendor"), dmi("bios_version")), parens(dmi("bios_date"))))
	f.set("chassis", chassisType(dmi("chassis_type")))
	f.set("virtualization", virtualization(vendor, model))

	cpu, cores := cpuInfo(filepath.Join(procRoot, "cpuinfo"))
	if cpu == "" {
		cpu = hardware // ARM cpuinfo has no model name; the board name is the best we have.
	}
	f.set("cpu", cpu)
	if cores > 0 {
		f.set("cores", strconv.Itoa(cores))
	}
	f.set("memory", memTotal(filepath.Join(procRoot, "meminfo")))
	f.set("disks", disks(filepath.Join(sysRoot, "block")))
	f.set("nics", nics(filepath.Join(sysRoot, "class/net")))
	return f
}

func (f Facts) set(key, value string) {
	if value = strings.TrimSpace(value); value != "" {
		f[key] = value
	}
}

// placeholders are the strings vendors ship when a DMI field was never
// programmed. They are worse than an empty value because they look like data.
var placeholders = []string{
	"to be filled by o.e.m.", "to be filled by oem", "default string",
	"system product name", "system serial number", "system version",
	"not specified", "not applicable", "unknown", "none", "n/a", "oem",
	"chassis manufacture", "0",
}

// readDMI reads a single DMI field, dropping the placeholder strings.
func readDMI(path string) string {
	v := readFirstLine(path)
	lower := strings.ToLower(v)
	for _, p := range placeholders {
		if lower == p || strings.Contains(lower, "to be filled") {
			return ""
		}
	}
	if strings.Trim(v, "0") == "" { // all-zero serials
		return ""
	}
	return v
}

func readFirstLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Device-tree strings are NUL-terminated; DMI strings sometimes carry
	// trailing whitespace.
	line := strings.TrimSpace(strings.Trim(string(data), "\x00"))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// dedupePrefix avoids "HP HP ZBook 17 G5": plenty of vendors repeat their own
// name in product_name.
func dedupePrefix(vendor, model string) string {
	if model == "" {
		return vendor
	}
	if vendor != "" && strings.HasPrefix(strings.ToLower(model), strings.ToLower(vendor)) {
		return model
	}
	return joinNonEmpty(" ", vendor, model)
}

func joinNonEmpty(sep string, values ...string) string {
	kept := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			kept = append(kept, v)
		}
	}
	return strings.Join(kept, sep)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parens(v string) string {
	if v == "" {
		return ""
	}
	return "(" + v + ")"
}

// chassisTypes is the SMBIOS chassis enumeration, trimmed to the values a real
// fleet actually reports. It answers "is this a laptop, a desktop or a rack
// server?" — the thing you want to know when a node name means nothing.
var chassisTypes = map[string]string{
	"1": "Other", "3": "Desktop", "4": "Low Profile Desktop", "5": "Pizza Box",
	"6": "Mini Tower", "7": "Tower", "8": "Portable", "9": "Laptop",
	"10": "Notebook", "11": "Hand Held", "13": "All In One", "14": "Sub Notebook",
	"15": "Space-saving", "16": "Lunch Box", "17": "Main Server Chassis",
	"18": "Expansion Chassis", "21": "Peripheral Chassis", "22": "RAID Chassis",
	"23": "Rack Mount Chassis", "24": "Sealed-case PC", "28": "Blade",
	"29": "Blade Enclosure", "30": "Tablet", "31": "Convertible", "32": "Detachable",
	"35": "Mini PC", "36": "Stick PC",
}

func chassisType(raw string) string {
	if name, ok := chassisTypes[raw]; ok {
		return name
	}
	return ""
}

// virtualization names the hypervisor when the DMI strings give it away. A
// blank result means "looks like real metal" — which is exactly what you want
// to know before walking to the rack to find a machine.
func virtualization(vendor, model string) string {
	hay := strings.ToLower(vendor + " " + model)
	for _, hv := range []struct{ match, name string }{
		{"qemu", "QEMU/KVM"}, {"kvm", "QEMU/KVM"}, {"vmware", "VMware"},
		{"virtualbox", "VirtualBox"}, {"innotek", "VirtualBox"}, {"xen", "Xen"},
		{"microsoft corporation virtual", "Hyper-V"}, {"bochs", "Bochs"},
		{"parallels", "Parallels"}, {"amazon ec2", "AWS EC2"}, {"google", "Google Compute Engine"},
		{"digitalocean", "DigitalOcean"}, {"hetzner", "Hetzner"}, {"scaleway", "Scaleway"},
	} {
		if strings.Contains(hay, hv.match) {
			return hv.name
		}
	}
	return ""
}

// cpuInfo returns the CPU model and the number of logical cores.
func cpuInfo(path string) (string, int) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer file.Close()

	var model string
	cores := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := splitField(scanner.Text())
		if !ok {
			continue
		}
		switch key {
		case "model name":
			if model == "" {
				model = value
			}
		case "processor":
			cores++
		}
	}
	return model, cores
}

func memTotal(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := splitField(scanner.Text())
		if !ok || key != "memtotal" {
			continue
		}
		kb, err := strconv.ParseFloat(strings.Fields(value)[0], 64)
		if err != nil {
			return ""
		}
		return fmt.Sprintf("%.1f GB", kb/(1024*1024))
	}
	return ""
}

// splitField parses one "key: value" line of /proc/cpuinfo or /proc/meminfo.
// Keys are lower-cased so callers don't have to care about "MemTotal" vs
// "model name".
func splitField(line string) (key, value string, ok bool) {
	key, value, ok = strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	return strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value), true
}

// disks lists the real block devices with their size and model, e.g.
// "sda 500 GB (Samsung SSD 860), nvme0n1 1.0 TB (WD_BLACK SN770)". Loop, RAM
// and device-mapper devices are skipped — they aren't hardware.
func disks(blockDir string) string {
	entries, err := os.ReadDir(blockDir)
	if err != nil {
		return ""
	}

	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "zram") ||
			strings.HasPrefix(name, "sr") {
			continue
		}
		// size is in 512-byte sectors regardless of the device's own block size.
		sectors, err := strconv.ParseInt(readFirstLine(filepath.Join(blockDir, name, "size")), 10, 64)
		if err != nil || sectors == 0 {
			continue
		}
		model := readFirstLine(filepath.Join(blockDir, name, "device/model"))
		// Longhorn and other iSCSI volumes show up in /sys/block exactly like a
		// disk. They are storage attached to the node, not part of the machine,
		// and they come and go with the pods.
		// ponytail: matched by vendor/model string; a node whose iSCSI target
		// reports something else would still be listed.
		if model == "VIRTUAL-DISK" || isVirtualDiskVendor(readFirstLine(filepath.Join(blockDir, name, "device/vendor"))) {
			continue
		}
		entryText := name + " " + humanBytes(sectors*512)
		if model != "" {
			entryText += " (" + model + ")"
		}
		found = append(found, entryText)
	}
	sort.Strings(found)
	return strings.Join(found, ", ")
}

// nics lists the physical network interfaces with their link speed and MAC.
// Only interfaces backed by a real device are listed, which drops the pile of
// veth/cni/flannel/wireguard interfaces a Kubernetes node accumulates.
func nics(netDir string) string {
	entries, err := os.ReadDir(netDir)
	if err != nil {
		return ""
	}

	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if _, err := os.Stat(filepath.Join(netDir, name, "device")); err != nil {
			continue
		}
		text := name
		if speed := readFirstLine(filepath.Join(netDir, name, "speed")); speed != "" && speed != "-1" {
			text += " " + speed + "Mb/s"
		}
		if mac := readFirstLine(filepath.Join(netDir, name, "address")); mac != "" {
			text += " " + mac
		}
		found = append(found, text)
	}
	sort.Strings(found)
	return strings.Join(found, ", ")
}

// isVirtualDiskVendor reports whether a block device's SCSI vendor string marks
// it as a network-attached volume rather than a disk in the machine.
func isVirtualDiskVendor(vendor string) bool {
	switch strings.ToUpper(strings.TrimSpace(vendor)) {
	case "IET", "LIO-ORG", "SYNOLOGY", "TRUENAS", "QEMU":
		return true
	}
	return false
}

func humanBytes(b int64) string {
	const unit = 1000 // disks are sold in decimal units; 500 GB should read as 500 GB
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTP"[exp])
}

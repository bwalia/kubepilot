package hostinfo

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree materialises a fake sysfs/procfs from a path→contents map.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, contents := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCollectLaptop(t *testing.T) {
	sys := writeTree(t, map[string]string{
		"class/dmi/id/sys_vendor":      "HP\n",
		"class/dmi/id/product_name":    "HP ZBook 17 G5\n",
		"class/dmi/id/product_serial":  "5CD9131NFM\n",
		"class/dmi/id/board_vendor":    "HP\n",
		"class/dmi/id/board_name":      "842A\n",
		"class/dmi/id/bios_vendor":     "HP\n",
		"class/dmi/id/bios_version":    "Q71 Ver. 01.24.00\n",
		"class/dmi/id/bios_date":       "05/16/2023\n",
		"class/dmi/id/chassis_type":    "10\n",
		"block/sda/size":               "976773168\n",
		"block/sda/device/model":       "Samsung SSD 860\n",
		"block/loop0/size":             "12345\n",
		"class/net/eth0/address":       "3c:52:82:11:22:33\n",
		"class/net/eth0/speed":         "1000\n",
		"class/net/eth0/device/uevent": "PCI\n",
		"class/net/flannel.1/address":  "aa:bb:cc:dd:ee:ff\n",
	})
	proc := writeTree(t, map[string]string{
		"cpuinfo": "processor\t: 0\nmodel name\t: Intel(R) Xeon(R) E-2176M CPU @ 2.70GHz\n\nprocessor\t: 1\nmodel name\t: Intel(R) Xeon(R) E-2176M CPU @ 2.70GHz\n",
		"meminfo": "MemTotal:       16283132 kB\nMemFree:         123456 kB\n",
	})

	f := Collect(sys, proc)

	want := map[string]string{
		KeyHardware: "HP ZBook 17 G5", // not "HP HP ZBook 17 G5"
		KeySerial:   "5CD9131NFM",
		"chassis":   "Notebook",
		"cores":     "2",
		"cpu":       "Intel(R) Xeon(R) E-2176M CPU @ 2.70GHz",
		"memory":    "15.5 GB",
		"bios":      "HP Q71 Ver. 01.24.00 (05/16/2023)",
		"disks":     "sda 500.1 GB (Samsung SSD 860)",  // loop0 dropped
		"nics":      "eth0 1000Mb/s 3c:52:82:11:22:33", // flannel.1 dropped: no device
	}
	for key, wantValue := range want {
		if f[key] != wantValue {
			t.Errorf("%s = %q, want %q", key, f[key], wantValue)
		}
	}
	if _, ok := f["virtualization"]; ok {
		t.Errorf("virtualization = %q, want absent on bare metal", f["virtualization"])
	}
}

func TestCollectDropsPlaceholdersAndKeepsPartialData(t *testing.T) {
	sys := writeTree(t, map[string]string{
		"class/dmi/id/sys_vendor":     "Notebook\n",
		"class/dmi/id/product_name":   "PCx0Dx\n",
		"class/dmi/id/product_serial": "Not Applicable\n",
		"class/dmi/id/board_serial":   "To Be Filled By O.E.M.\n",
		"class/dmi/id/bios_vendor":    "Default string\n",
		"class/dmi/id/chassis_type":   "3\n",
	})

	f := Collect(sys, t.TempDir())

	if f[KeyHardware] != "Notebook PCx0Dx" {
		t.Errorf("hardware = %q", f[KeyHardware])
	}
	if _, ok := f[KeySerial]; ok {
		t.Errorf("serial = %q, want absent (both sources are placeholders)", f[KeySerial])
	}
	if _, ok := f["bios"]; ok {
		t.Errorf("bios = %q, want absent", f["bios"])
	}
	if f["chassis"] != "Desktop" {
		t.Errorf("chassis = %q, want Desktop", f["chassis"])
	}
}

func TestCollectRaspberryPiAndVM(t *testing.T) {
	pi := writeTree(t, map[string]string{
		"firmware/devicetree/base/model": "Raspberry Pi 5 Model B Rev 1.0\x00",
		// arm64 /proc/cpuinfo has no "model name" — only implementer/part IDs —
		// so the core has to come from the device tree.
		"devices/system/cpu/cpu0/of_node/compatible": "arm,cortex-a76\x00",
	})
	f := Collect(pi, t.TempDir())
	if f[KeyHardware] != "Raspberry Pi 5 Model B Rev 1.0" {
		t.Errorf("pi hardware = %q", f[KeyHardware])
	}
	if f["cpu"] != "Cortex-A76" {
		t.Errorf("pi cpu = %q, want Cortex-A76 from the device tree", f["cpu"])
	}

	// No device tree either: the board name is the last resort.
	bare := writeTree(t, map[string]string{"class/dmi/id/product_name": "OptiPlex 3050"})
	if f := Collect(bare, t.TempDir()); f["cpu"] != f[KeyHardware] {
		t.Errorf("cpu = %q, want the board name %q", f["cpu"], f[KeyHardware])
	}

	vm := writeTree(t, map[string]string{
		"class/dmi/id/sys_vendor":   "QEMU\n",
		"class/dmi/id/product_name": "Standard PC (i440FX + PIIX, 1996)\n",
	})
	if got := Collect(vm, t.TempDir())["virtualization"]; got != "QEMU/KVM" {
		t.Errorf("virtualization = %q, want QEMU/KVM", got)
	}
}

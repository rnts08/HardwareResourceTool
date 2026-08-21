package collect

import (
	"os"
	"path/filepath"
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestReadUSBDevicesSkipsInterfacesAndRootHubs(t *testing.T) {
	dir := t.TempDir()
	write := func(name, file, content string) {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, file), []byte(content+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("1-2", "idVendor", "8087")
	write("1-2", "idProduct", "0026")
	write("1-2", "manufacturer", "Intel Corp.")
	write("1-2", "product", "AX201 Bluetooth")
	write("1-2", "serial", "ABCD")
	write("1-2", "speed", "12")
	write("1-2:1.0", "idVendor", "ignored")
	write("usb3", "idVendor", "1d6b")
	devices := readUSBDevices(dir)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d: %+v", len(devices), devices)
	}
	device := devices[0]
	if device.BusID != "1-2" || device.VendorID != "8087" || device.ProductID != "0026" || device.SpeedMbps != 12 || device.Manufacturer != "Intel Corp." || device.Serial != "ABCD" {
		t.Fatalf("unexpected device: %+v", device)
	}
}

func TestReadUSBDevicesEmptyDir(t *testing.T) {
	if devices := readUSBDevices(t.TempDir()); len(devices) != 0 {
		t.Fatalf("expected no devices, got %+v", devices)
	}
}

func TestReadCPUPolicies(t *testing.T) {
	base := t.TempDir()
	policyDir := filepath.Join(base, "policy0")
	if err := os.MkdirAll(policyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"scaling_governor":                         "powersave\n",
		"scaling_available_governors":              "performance powersave\n",
		"energy_performance_preference":            "balance_performance\n",
		"energy_performance_available_preferences": "performance balance_performance power\n",
		"related_cpus":                             "0 1 2 3\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(policyDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	policies := readCPUPolicies(base)
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	policy := policies[0]
	if policy.Governor != "powersave" || policy.CPUs != "0 1 2 3" || policy.EPP != "balance_performance" {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	if len(policy.AvailableGovernors) != 2 || policy.AvailableGovernors[0] != "performance" {
		t.Fatalf("unexpected governors: %+v", policy.AvailableGovernors)
	}
	if len(policy.AvailableEPP) != 3 || policy.AvailableEPP[0] != "balance_performance" {
		t.Fatalf("unexpected EPP list: %+v", policy.AvailableEPP)
	}
}

func TestReadDMInfo(t *testing.T) {
	blockDir := t.TempDir()
	dmDir := filepath.Join(blockDir, "dm")
	slavesDir := filepath.Join(blockDir, "slaves")
	if err := os.MkdirAll(dmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(slavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmDir, "name"), []byte("vg0-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmDir, "uuid"), []byte("LVM-abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, slave := range []string{"sda2", "sdb2"} {
		if err := os.WriteFile(filepath.Join(slavesDir, slave), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var disk model.Disk
	readDMInfo(blockDir, &disk)
	if disk.DMName != "vg0-root" || disk.DMUUID != "LVM-abc123" {
		t.Fatalf("unexpected dm identity: %+v", disk)
	}
	if len(disk.Slaves) != 2 || disk.Slaves[0] != "sda2" || disk.Slaves[1] != "sdb2" {
		t.Fatalf("unexpected slaves: %+v", disk.Slaves)
	}
}

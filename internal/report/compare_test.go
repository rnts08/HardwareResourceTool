package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"hardware-resources-tool/internal/model"
)

func TestCompareReportsFindingChanges(t *testing.T) {
	older := model.Report{GeneratedAt: time.Unix(100, 0).UTC(), Findings: []model.Finding{{Title: "CPU idle critical", Severity: "critical", Evidence: "idle 1.0%"}, {Title: "Memory used critical", Severity: "critical", Evidence: "used 97%"}}}
	newer := model.Report{GeneratedAt: time.Unix(200, 0).UTC(), Findings: []model.Finding{{Title: "Memory used critical", Severity: "critical", Evidence: "used 97%"}, {Title: "I/O wait warning", Severity: "warning", Evidence: "iowait 12%"}}}
	comparison := Compare(older, newer)
	if len(comparison.NewFindings) != 1 || comparison.NewFindings[0].Title != "I/O wait warning" {
		t.Fatalf("unexpected new findings: %#v", comparison.NewFindings)
	}
	if len(comparison.ClearedFindings) != 1 || comparison.ClearedFindings[0].Title != "CPU idle critical" {
		t.Fatalf("unexpected cleared findings: %#v", comparison.ClearedFindings)
	}
}

func TestCompareReportsRateDeltasPerCategory(t *testing.T) {
	older := model.Report{
		GeneratedAt: time.Unix(100, 0).UTC(),
		Snapshot: model.Snapshot{
			CPU:      model.CPU{UserPercent: 10, IdlePercent: 70, Load1: 1.0},
			Memory:   model.Memory{UsedPercent: 50, AvailableBytes: 8 << 30},
			Disks:    []model.Disk{{Name: "sda", ReadBytesPerSec: 10 << 20, WriteBytesPerSec: 5 << 20}},
			Networks: []model.Network{{Name: "eth0", RXBytesPerSec: 100 << 20, TXBytesPerSec: 50 << 20}},
		},
	}
	newer := model.Report{
		GeneratedAt: time.Unix(200, 0).UTC(),
		Snapshot: model.Snapshot{
			CPU:      model.CPU{UserPercent: 25, IdlePercent: 50, Load1: 2.5},
			Memory:   model.Memory{UsedPercent: 75, AvailableBytes: 4 << 30},
			Disks:    []model.Disk{{Name: "sda", ReadBytesPerSec: 40 << 20, WriteBytesPerSec: 5 << 20}},
			Networks: []model.Network{{Name: "eth0", RXBytesPerSec: 150 << 20, TXBytesPerSec: 60 << 20}},
		},
	}
	comparison := Compare(older, newer)
	var buffer bytes.Buffer
	if err := WriteComparison(&buffer, comparison); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	for _, expected := range []string{"CPU", "Memory", "Disks", "Networks", "sda read", "eth0 rx", "user", "idle"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("comparison missing %q: %s", expected, output)
		}
	}
	if !strings.Contains(output, "+15.0 pts") {
		t.Fatalf("comparison missing user +15.0 pts delta: %s", output)
	}
	if !strings.Contains(output, "30.0 MiB/s") {
		t.Fatalf("comparison missing sda read delta: %s", output)
	}
}

func TestCompareReportsNewAndRemovedResources(t *testing.T) {
	older := model.Report{
		GeneratedAt: time.Unix(100, 0).UTC(),
		Snapshot: model.Snapshot{
			Disks:          []model.Disk{{Name: "sdb"}},
			Networks:       []model.Network{{Name: "eth0"}},
			Thermal:        model.Thermal{Zones: []model.ThermalZone{{Name: "x86_pkg_temp", Current: 40}}},
			Virtualization: model.Virtualization{VirtualMachines: []model.VirtualMachine{{Name: "guest-a"}}},
		},
	}
	newer := model.Report{
		GeneratedAt: time.Unix(200, 0).UTC(),
		Snapshot: model.Snapshot{
			Disks:          []model.Disk{{Name: "sda"}},
			Networks:       []model.Network{{Name: "eth0"}, {Name: "eth1"}},
			Thermal:        model.Thermal{Zones: []model.ThermalZone{{Name: "x86_pkg_temp", Current: 55}}},
			Virtualization: model.Virtualization{VirtualMachines: []model.VirtualMachine{{Name: "guest-a"}, {Name: "guest-b"}}},
		},
	}
	var buffer bytes.Buffer
	if err := WriteComparison(&buffer, Compare(older, newer)); err != nil {
		t.Fatal(err)
	}
	output := buffer.String()
	for _, expected := range []string{"new disk sda", "disk sdb no longer present", "new network eth1", "x86_pkg_temp temp"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("comparison missing %q: %s", expected, output)
		}
	}
}

func TestReadReportRoundTrip(t *testing.T) {
	original := model.Report{SchemaVersion: "1", GeneratedAt: time.Now().UTC(), Snapshot: model.Snapshot{CPU: model.CPU{UserPercent: 5}}, Findings: []model.Finding{{Title: "x", Severity: "info"}}}
	var buffer bytes.Buffer
	if err := WriteJSON(&buffer, original); err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadReport(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != original.SchemaVersion || decoded.Snapshot.CPU.UserPercent != 5 || len(decoded.Findings) != 1 {
		t.Fatalf("round trip mismatch: %#v", decoded)
	}
}

func TestCompareGPUCategory(t *testing.T) {
	older := model.Report{Snapshot: model.Snapshot{GPUs: []model.GPU{
		{Address: "0000:01:00.0", Name: "A100", NVML: true, UtilizationPercent: 10, TemperatureCelsius: 50, PowerWatts: 100, MemoryUsedBytes: 1 << 30},
	}}}
	newer := model.Report{Snapshot: model.Snapshot{GPUs: []model.GPU{
		{Address: "0000:01:00.0", Name: "A100", NVML: true, UtilizationPercent: 90, TemperatureCelsius: 70, PowerWatts: 250, MemoryUsedBytes: 8 << 30},
		{Address: "0000:02:00.0", Name: "A100", NVML: false},
	}}}
	comparison := Compare(older, newer)
	var gpus CategoryDelta
	found := false
	for _, category := range comparison.Categories {
		if category.Name == "GPUs" {
			gpus, found = category, true
		}
	}
	if !found {
		t.Fatal("GPU category missing")
	}
	if len(gpus.Values) != 4 {
		t.Fatalf("expected 4 GPU value deltas, got %d", len(gpus.Values))
	}
	if !strings.Contains(gpus.Summary, "new GPU 0000:02:00.0") {
		t.Fatalf("new GPU missing from summary: %q", gpus.Summary)
	}

	removed := older
	removed.Snapshot.GPUs = []model.GPU{{Address: "0000:09:00.0", Name: "old"}}
	summary := Compare(removed, newer)
	for _, category := range summary.Categories {
		if category.Name == "GPUs" && !strings.Contains(category.Summary, "no longer present") {
			t.Fatalf("removed GPU missing: %q", category.Summary)
		}
	}
}

func TestCompareUSBCategory(t *testing.T) {
	older := model.Report{Snapshot: model.Snapshot{USB: []model.USBDevice{
		{BusID: "1-2", VendorID: "8087", ProductID: "0026", Product: "AX201"},
	}}}
	newer := model.Report{Snapshot: model.Snapshot{USB: []model.USBDevice{
		{BusID: "1-2", VendorID: "8087", ProductID: "0026", Product: "AX201"},
		{BusID: "2-1", VendorID: "0781", ProductID: "55ab", Product: "SanDisk"},
	}}}
	comparison := Compare(older, newer)
	found := false
	for _, category := range comparison.Categories {
		if category.Name == "USB" {
			found = true
			if !strings.Contains(category.Summary, "new USB device 2-1 0781:55ab (SanDisk)") {
				t.Fatalf("unexpected USB summary: %q", category.Summary)
			}
		}
	}
	if !found {
		t.Fatal("USB category missing")
	}

	unchanged := Compare(newer, newer)
	for _, category := range unchanged.Categories {
		if category.Name == "USB" {
			t.Fatal("USB category emitted with no changes")
		}
	}
}

func TestCompareCpufreqCategory(t *testing.T) {
	older := model.Report{Snapshot: model.Snapshot{CPUPower: []model.CPUPolicy{
		{Policy: "policy0", Governor: "performance", EPP: "performance"},
	}}}
	newer := model.Report{Snapshot: model.Snapshot{CPUPower: []model.CPUPolicy{
		{Policy: "policy0", Governor: "powersave", EPP: "power"},
	}}}
	comparison := Compare(older, newer)
	found := false
	for _, category := range comparison.Categories {
		if category.Name == "CPU frequency policy" {
			found = true
			if !strings.Contains(category.Summary, "policy0 governor performance -> powersave") ||
				!strings.Contains(category.Summary, "policy0 EPP performance -> power") {
				t.Fatalf("cpufreq changes missing: %q", category.Summary)
			}
		}
	}
	if !found {
		t.Fatal("cpufreq category missing")
	}
}

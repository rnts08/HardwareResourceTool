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

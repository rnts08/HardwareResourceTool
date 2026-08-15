package analyze

import (
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestFindingsDetectResourcePressure(t *testing.T) {
	snapshot := model.Snapshot{
		CPU:      model.CPU{LogicalCPUs: 8, IdlePercent: 4, IOWaitPercent: 20},
		Memory:   model.Memory{UsedPercent: 95, SwapOutPerSec: 3},
		Networks: []model.Network{{Name: "eth0", RXErrors: 1}},
	}

	findings := Findings(snapshot)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(findings))
	}
	if findings[0].Severity != "critical" {
		t.Fatalf("expected CPU finding to be critical, got %q", findings[0].Severity)
	}
}

func TestFindingsReportCollectionErrors(t *testing.T) {
	findings := Findings(model.Snapshot{Errors: []string{"missing metric"}})
	if len(findings) != 1 || findings[0].Category != "collection" {
		t.Fatalf("expected collection finding, got %#v", findings)
	}
}

func TestFindingsDetectSysctlConfiguration(t *testing.T) {
	findings := Findings(model.Snapshot{System: model.SystemSettings{Sysctls: map[string]string{"vm.overcommit_memory": "2", "vm.dirty_ratio": "25"}}})
	if len(findings) != 2 {
		t.Fatalf("expected 2 sysctl findings, got %d", len(findings))
	}
}

func TestPCIeFindings(t *testing.T) {
	findings := Findings(model.Snapshot{PCI: []model.PCIDevice{{
		Address: "0000:01:00.0", PCIeCapabilityMaxSpeed: "16.0 GT/s PCIe", PCIeCapabilityMaxWidth: 16,
		PCIeNegotiatedSpeed: "8.0 GT/s PCIe", PCIeNegotiatedWidth: 8, PCIePathBottleneck: "0000:00:01.0",
		PCIePathMinSpeed: "8.0 GT/s PCIe", PCIePathMinWidth: 8, PCIePathBandwidthGbps: 63.0,
		AERCorrectableStatus: 1,
	}}})
	if len(findings) != 3 || findings[0].Category != "pcie" || findings[1].Title != "PCIe path has a narrower bandwidth bottleneck" || findings[2].Title != "PCIe AER errors are present" {
		t.Fatalf("unexpected PCIe findings: %#v", findings)
	}
}

func TestVirtualizationUsageFindings(t *testing.T) {
	findings := Findings(model.Snapshot{Virtualization: model.Virtualization{QEMUDetected: true, VirtualMachines: []model.VirtualMachine{{Name: "guest-a", CgroupAvailable: true, MemoryCurrentBytes: 950, MemoryMaxBytes: 1000, QMPStatus: "paused"}}}})
	if len(findings) != 2 || findings[0].Category != "virtualization" || findings[1].Title != "QEMU domain is paused" {
		t.Fatalf("unexpected virtualization findings: %#v", findings)
	}
}

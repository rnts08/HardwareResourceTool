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

func TestPCICrossNUMAPathFinding(t *testing.T) {
	snapshot := model.Snapshot{PCI: []model.PCIDevice{
		{Address: "0000:01:00.0", NUMANode: 0, PCIePath: []string{"0000:01:00.0", "0000:00:01.0", "0000:00:00.0"}},
		{Address: "0000:00:01.0", NUMANode: 1},
		{Address: "0000:00:00.0", NUMANode: 1},
	}}
	findings := Findings(snapshot)
	matched := 0
	for _, finding := range findings {
		if finding.Title == "PCIe path crosses a NUMA boundary" {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("expected one cross-NUMA path finding, got %d: %#v", matched, findings)
	}
}

func TestIOMMUGroupSpansNUMANodesFinding(t *testing.T) {
	snapshot := model.Snapshot{PCI: []model.PCIDevice{
		{Address: "0000:01:00.0", IOMMUGroup: "10", NUMANode: 0},
		{Address: "0000:01:00.1", IOMMUGroup: "10", NUMANode: 1},
	}}
	findings := Findings(snapshot)
	matched := 0
	for _, finding := range findings {
		if finding.Title == "IOMMU group spans NUMA nodes" {
			if finding.Severity != "warning" {
				t.Fatalf("expected warning severity, got %q", finding.Severity)
			}
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("expected IOMMU group spans-NUMA finding, got %#v", findings)
	}
}

func TestPassthroughNUMAMismatchFinding(t *testing.T) {
	snapshot := model.Snapshot{
		PCI: []model.PCIDevice{{Address: "0000:65:00.0", NUMANode: 1}},
		Virtualization: model.Virtualization{QEMUDetected: true, VirtualMachines: []model.VirtualMachine{
			{Name: "gpu-vm", NUMANodes: []int{0}, PCIAddresses: []string{"0000:65:00.0"}},
		}},
	}
	findings := Findings(snapshot)
	if len(findings) != 1 || findings[0].Title != "Passthrough device is on a different NUMA node than the VM" {
		t.Fatalf("unexpected passthrough NUMA findings: %#v", findings)
	}
}

func TestUnboundEndpointFinding(t *testing.T) {
	snapshot := model.Snapshot{PCI: []model.PCIDevice{{Address: "0000:02:00.0", Class: "0x030000", IOMMUGroup: "5"}}}
	findings := Findings(snapshot)
	if len(findings) != 1 || findings[0].Title != "PCIe endpoint has no bound driver" {
		t.Fatalf("unexpected unbound-endpoint findings: %#v", findings)
	}
}

func TestNUMAResidencyEscalation(t *testing.T) {
	gib := uint64(1024 * 1024 * 1024)
	snapshot := model.Snapshot{Virtualization: model.Virtualization{QEMUDetected: true, VirtualMachines: []model.VirtualMachine{
		{Name: "pinned-vm", NUMANodes: []int{0}, RuntimeNUMABytes: map[int]uint64{0: gib, 1: 4 * gib}, RuntimeAvailable: true},
	}}}
	findings := Findings(snapshot)
	var residency *model.Finding
	for i := range findings {
		if findings[i].Title == "QEMU memory is resident outside configured NUMA nodeset" {
			residency = &findings[i]
		}
	}
	if residency == nil {
		t.Fatalf("expected residency finding, got %#v", findings)
	}
	if residency.Severity != "warning" {
		t.Errorf("expected warning for 80%% remote residency, got %s", residency.Severity)
	}
}

func TestHugepageConfiguredUnusedFinding(t *testing.T) {
	snapshot := model.Snapshot{Virtualization: model.Virtualization{QEMUDetected: true, VirtualMachines: []model.VirtualMachine{
		{Name: "huge-vm", Hugepages: true, Running: true, RuntimeAvailable: true},
	}}}
	findings := Findings(snapshot)
	found := false
	for _, finding := range findings {
		if finding.Title == "VM configured for hugepages uses none at runtime" && finding.Severity == "info" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unused-hugepage finding, got %#v", findings)
	}
}

func TestHugetlbPoolExhaustedFinding(t *testing.T) {
	snapshot := model.Snapshot{
		Memory:         model.Memory{HugepagesTotal: 16, HugepagesFree: 0},
		Virtualization: model.Virtualization{QEMUDetected: true, VirtualMachines: []model.VirtualMachine{{Name: "huge-vm", Hugepages: true, Running: true}}},
	}
	findings := Findings(snapshot)
	found := false
	for _, finding := range findings {
		if finding.Title == "Host hugetlb pool has no free pages" && finding.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hugetlb-pool finding, got %#v", findings)
	}
}

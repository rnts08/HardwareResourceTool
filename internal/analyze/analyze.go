package analyze

import (
	"fmt"
	"strconv"

	"hardware-resources-tool/internal/model"
)

type Thresholds struct {
	CPUIdleCritical        float64
	IOWaitWarning          float64
	MemoryUsedCritical     float64
	FilesystemUsedWarning  float64
	FilesystemUsedCritical float64
}

var DefaultThresholds = Thresholds{
	CPUIdleCritical:        10,
	IOWaitWarning:          15,
	MemoryUsedCritical:     90,
	FilesystemUsedWarning:  85,
	FilesystemUsedCritical: 95,
}

func Findings(s model.Snapshot) []model.Finding {
	return FindingsWithThresholds(s, DefaultThresholds)
}

func FindingsWithThresholds(s model.Snapshot, thresholds Thresholds) []model.Finding {
	findings := []model.Finding{}
	if s.CPU.LogicalCPUs > 0 && s.CPU.IdlePercent < thresholds.CPUIdleCritical {
		findings = append(findings, model.Finding{Severity: "critical", Category: "cpu", Title: "CPU capacity is heavily utilized", Evidence: fmt.Sprintf("Only %.1f%% idle CPU was observed", s.CPU.IdlePercent), Recommendation: "Inspect vCPU overcommitment and guest CPU demand; consider reducing host overcommitment or moving workloads."})
	}
	if s.CPU.IOWaitPercent > thresholds.IOWaitWarning {
		findings = append(findings, model.Finding{Severity: "warning", Category: "storage", Title: "CPU is waiting on I/O", Evidence: fmt.Sprintf("CPU iowait is %.1f%%", s.CPU.IOWaitPercent), Recommendation: "Inspect storage latency, queue depth, and contention on the backing devices."})
	}
	if s.Memory.UsedPercent > thresholds.MemoryUsedCritical {
		findings = append(findings, model.Finding{Severity: "critical", Category: "memory", Title: "Memory capacity is nearly exhausted", Evidence: fmt.Sprintf("Memory utilization is %.1f%%", s.Memory.UsedPercent), Recommendation: "Reduce guest memory overcommitment or add capacity; verify reclaim and swap activity."})
	}
	if s.Memory.SwapInPerSec > 0 || s.Memory.SwapOutPerSec > 0 {
		findings = append(findings, model.Finding{Severity: "warning", Category: "memory", Title: "Swap activity detected", Evidence: fmt.Sprintf("Swap in: %d/s, swap out: %d/s", s.Memory.SwapInPerSec, s.Memory.SwapOutPerSec), Recommendation: "Investigate memory pressure and consider reserving more host memory for virtualization workloads."})
	}
	for _, filesystem := range s.Filesystems {
		if filesystem.UsedPercent > thresholds.FilesystemUsedCritical {
			findings = append(findings, model.Finding{Severity: "critical", Category: "storage", Title: "Filesystem capacity is nearly exhausted", Evidence: fmt.Sprintf("%s is %.1f%% full", filesystem.MountPoint, filesystem.UsedPercent), Recommendation: "Remove or relocate data, expand the filesystem, and keep adequate free space for host and guest I/O."})
		} else if filesystem.UsedPercent > thresholds.FilesystemUsedWarning {
			findings = append(findings, model.Finding{Severity: "warning", Category: "storage", Title: "Filesystem capacity is high", Evidence: fmt.Sprintf("%s is %.1f%% full", filesystem.MountPoint, filesystem.UsedPercent), Recommendation: "Monitor growth and plan cleanup or capacity expansion before the filesystem becomes a bottleneck."})
		}
	}
	for _, network := range s.Networks {
		if network.State != "down" && (network.RXErrors > 0 || network.TXErrors > 0 || network.RXDrops > 0 || network.TXDrops > 0) {
			findings = append(findings, model.Finding{Severity: "warning", Category: "network", Title: "Network errors or drops detected", Evidence: fmt.Sprintf("%s errors rx=%d tx=%d, drops rx=%d tx=%d", network.Name, network.RXErrors, network.TXErrors, network.RXDrops, network.TXDrops), Recommendation: "Inspect NIC health, link negotiation, driver counters, queue sizing, and host network contention."})
		}
	}
	for _, device := range s.PCI {
		if device.PCIeCapabilityMaxSpeed != "" && device.PCIeNegotiatedSpeed != "" && (device.PCIeCapabilityMaxSpeed != device.PCIeNegotiatedSpeed || device.PCIeCapabilityMaxWidth > device.PCIeNegotiatedWidth) {
			findings = append(findings, model.Finding{Severity: "warning", Category: "pcie", Title: "PCIe link is negotiated below capability", Evidence: fmt.Sprintf("%s negotiated %s x%d; capability is %s x%d", device.Address, device.PCIeNegotiatedSpeed, device.PCIeNegotiatedWidth, device.PCIeCapabilityMaxSpeed, device.PCIeCapabilityMaxWidth), Recommendation: "Check slot wiring, bifurcation, firmware policy, bridge compatibility, and link training before treating the endpoint as fully provisioned."})
		}
		if device.PCIePathBottleneck != "" && device.PCIePathBottleneck != device.Address {
			findings = append(findings, model.Finding{Severity: "warning", Category: "pcie", Title: "PCIe path has a narrower bandwidth bottleneck", Evidence: fmt.Sprintf("%s path minimum is %s x%d at %s (%0.2f Gb/s aggregate)", device.Address, device.PCIePathMinSpeed, device.PCIePathMinWidth, device.PCIePathBottleneck, device.PCIePathBandwidthGbps), Recommendation: "Inspect the upstream bridge and slot topology; the endpoint may be limited even when its own negotiated link appears wide."})
		}
		if device.AERUncorrectableStatus != 0 || device.AERCorrectableStatus != 0 {
			findings = append(findings, model.Finding{Severity: "warning", Category: "pcie", Title: "PCIe AER errors are present", Evidence: fmt.Sprintf("%s AER uncorrectable=0x%08x correctable=0x%08x", device.Address, device.AERUncorrectableStatus, device.AERCorrectableStatus), Recommendation: "Correlate AER status with kernel logs, slot/bridge health, power, cabling, and firmware before clearing counters."})
		}
	}
	groups := map[string][]string{}
	for _, device := range s.PCI {
		if device.IOMMUGroup != "" {
			groups[device.IOMMUGroup] = append(groups[device.IOMMUGroup], device.Address)
		}
	}
	for group, addresses := range groups {
		if len(addresses) > 1 {
			findings = append(findings, model.Finding{Severity: "info", Category: "pcie", Title: "IOMMU group contains multiple PCI functions", Evidence: fmt.Sprintf("group %s contains %d functions: %v", group, len(addresses), addresses), Recommendation: "Review the complete IOMMU group before assigning or isolating one function for passthrough; all grouped functions may share isolation boundaries."})
		}
	}
	for _, device := range s.PCI {
		if device.PCIeParentAddress == "" || device.NUMANode < 0 {
			continue
		}
		for _, parent := range s.PCI {
			if parent.Address == device.PCIeParentAddress && parent.NUMANode >= 0 && parent.NUMANode != device.NUMANode {
				findings = append(findings, model.Finding{Severity: "info", Category: "pcie", Title: "PCIe endpoint and upstream bridge use different NUMA nodes", Evidence: fmt.Sprintf("%s is NUMA %d; parent bridge %s is NUMA %d", device.Address, device.NUMANode, parent.Address, parent.NUMANode), Recommendation: "Validate workload placement and interrupt/CPU affinity when latency or PCIe traffic locality matters."})
			}
		}
	}
	if s.Virtualization.QEMUDetected {
		if s.Virtualization.VCPUOvercommitRatio > 1 {
			findings = append(findings, model.Finding{Severity: "warning", Category: "virtualization", Title: "Configured vCPUs exceed host logical CPUs", Evidence: fmt.Sprintf("%d allocated vCPUs over %d logical host CPUs (%.2fx)", s.Virtualization.AllocatedVCPUs, s.CPU.LogicalCPUs, s.Virtualization.VCPUOvercommitRatio), Recommendation: "Validate scheduler latency and workload demand before increasing vCPU allocation or placing additional guests on this host."})
		}
		if s.Virtualization.MemoryOvercommitRatio > 1 {
			findings = append(findings, model.Finding{Severity: "warning", Category: "virtualization", Title: "Configured guest memory exceeds host memory", Evidence: fmt.Sprintf("%.1f GiB allocated over %.1f GiB host memory (%.2fx)", float64(s.Virtualization.AllocatedMemoryBytes)/(1024*1024*1024), float64(s.Memory.TotalBytes)/(1024*1024*1024), s.Virtualization.MemoryOvercommitRatio), Recommendation: "Account for ballooning, paging, reservations, and host overhead before treating configured guest memory as physically available."})
		}
		for _, vm := range s.Virtualization.VirtualMachines {
			if len(vm.NUMANodes) > 0 && s.NUMA.Nodes > 0 {
				invalid := []int{}
				for _, node := range vm.NUMANodes {
					if node < 0 || node >= s.NUMA.Nodes {
						invalid = append(invalid, node)
					}
				}
				if len(invalid) > 0 {
					findings = append(findings, model.Finding{Severity: "warning", Category: "numa", Title: "VM NUMA nodeset is outside host topology", Evidence: fmt.Sprintf("%s requests nodes %v but host reports %d NUMA nodes", vm.Name, invalid, s.NUMA.Nodes), Recommendation: "Correct the libvirt NUMA policy to use host node indexes; an invalid nodeset can prevent startup or defeat locality."})
				}
			}
			if vm.CgroupAvailable && vm.MemoryMaxBytes > 0 && float64(vm.MemoryCurrentBytes)/float64(vm.MemoryMaxBytes) > 0.9 {
				findings = append(findings, model.Finding{Severity: "warning", Category: "virtualization", Title: "VM cgroup memory is near its limit", Evidence: fmt.Sprintf("%s uses %.1f%% of its cgroup memory limit", vm.Name, float64(vm.MemoryCurrentBytes)/float64(vm.MemoryMaxBytes)*100), Recommendation: "Review guest memory pressure, ballooning, host reclaim, and the domain memory limit before adding workload or increasing overcommit."})
			}
			if vm.QMPStatus == "paused" {
				findings = append(findings, model.Finding{Severity: "warning", Category: "virtualization", Title: "QEMU domain is paused", Evidence: vm.Name + " reports paused through read-only QMP status", Recommendation: "Inspect the domain and host logs to determine whether the pause is intentional or caused by an I/O, memory, or device condition."})
			}
		}
	}
	if s.System.THP != "" && len(s.System.THP) > 0 && s.System.THP[0:1] != "[" {
		findings = append(findings, model.Finding{Severity: "info", Category: "configuration", Title: "Transparent huge pages are not shown as active", Evidence: s.System.THP, Recommendation: "Review transparent huge page policy against the requirements of the virtualization platform."})
	}
	if s.System.Sysctls["vm.overcommit_memory"] == "2" {
		findings = append(findings, model.Finding{Severity: "info", Category: "configuration", Title: "Strict memory overcommit policy is enabled", Evidence: "vm.overcommit_memory=2", Recommendation: "Confirm this policy matches the virtualization platform's memory reservation and guest allocation model."})
	}
	if value := s.System.Sysctls["vm.dirty_ratio"]; value != "" {
		if ratio, err := strconv.ParseInt(value, 10, 64); err == nil && ratio > 20 {
			findings = append(findings, model.Finding{Severity: "info", Category: "configuration", Title: "High dirty-page writeback threshold", Evidence: fmt.Sprintf("vm.dirty_ratio=%d", ratio), Recommendation: "Review dirty-page thresholds if latency-sensitive storage workloads experience bursty writeback."})
		}
	}
	if len(s.Errors) > 0 {
		findings = append(findings, model.Finding{Severity: "warning", Category: "collection", Title: "Some metrics were unavailable", Evidence: fmt.Sprintf("%d collector errors were reported", len(s.Errors)), Recommendation: "Review collector_errors in the JSON report before treating this assessment as complete."})
	}
	return findings
}

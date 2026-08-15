package analyze

import (
	"fmt"
	"strconv"

	"hardware-resources-tool/internal/model"
)

func Findings(s model.Snapshot) []model.Finding {
	findings := []model.Finding{}
	if s.CPU.LogicalCPUs > 0 && s.CPU.IdlePercent < 10 {
		findings = append(findings, model.Finding{Severity: "critical", Category: "cpu", Title: "CPU capacity is heavily utilized", Evidence: fmt.Sprintf("Only %.1f%% idle CPU was observed", s.CPU.IdlePercent), Recommendation: "Inspect vCPU overcommitment and guest CPU demand; consider reducing host overcommitment or moving workloads."})
	}
	if s.CPU.IOWaitPercent > 15 {
		findings = append(findings, model.Finding{Severity: "warning", Category: "storage", Title: "CPU is waiting on I/O", Evidence: fmt.Sprintf("CPU iowait is %.1f%%", s.CPU.IOWaitPercent), Recommendation: "Inspect storage latency, queue depth, and contention on the backing devices."})
	}
	if s.Memory.UsedPercent > 90 {
		findings = append(findings, model.Finding{Severity: "critical", Category: "memory", Title: "Memory capacity is nearly exhausted", Evidence: fmt.Sprintf("Memory utilization is %.1f%%", s.Memory.UsedPercent), Recommendation: "Reduce guest memory overcommitment or add capacity; verify reclaim and swap activity."})
	}
	if s.Memory.SwapInPerSec > 0 || s.Memory.SwapOutPerSec > 0 {
		findings = append(findings, model.Finding{Severity: "warning", Category: "memory", Title: "Swap activity detected", Evidence: fmt.Sprintf("Swap in: %d/s, swap out: %d/s", s.Memory.SwapInPerSec, s.Memory.SwapOutPerSec), Recommendation: "Investigate memory pressure and consider reserving more host memory for virtualization workloads."})
	}
	for _, filesystem := range s.Filesystems {
		if filesystem.UsedPercent > 95 {
			findings = append(findings, model.Finding{Severity: "critical", Category: "storage", Title: "Filesystem capacity is nearly exhausted", Evidence: fmt.Sprintf("%s is %.1f%% full", filesystem.MountPoint, filesystem.UsedPercent), Recommendation: "Remove or relocate data, expand the filesystem, and keep adequate free space for host and guest I/O."})
		} else if filesystem.UsedPercent > 85 {
			findings = append(findings, model.Finding{Severity: "warning", Category: "storage", Title: "Filesystem capacity is high", Evidence: fmt.Sprintf("%s is %.1f%% full", filesystem.MountPoint, filesystem.UsedPercent), Recommendation: "Monitor growth and plan cleanup or capacity expansion before the filesystem becomes a bottleneck."})
		}
	}
	for _, network := range s.Networks {
		if network.State != "down" && (network.RXErrors > 0 || network.TXErrors > 0 || network.RXDrops > 0 || network.TXDrops > 0) {
			findings = append(findings, model.Finding{Severity: "warning", Category: "network", Title: "Network errors or drops detected", Evidence: fmt.Sprintf("%s errors rx=%d tx=%d, drops rx=%d tx=%d", network.Name, network.RXErrors, network.TXErrors, network.RXDrops, network.TXDrops), Recommendation: "Inspect NIC health, link negotiation, driver counters, queue sizing, and host network contention."})
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

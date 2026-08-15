package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/model"
)

func Collect(c *collect.Collector, duration time.Duration) model.Report {
	return CollectWithThresholds(c, duration, analyze.DefaultThresholds)
}

func CollectWithThresholds(c *collect.Collector, duration time.Duration, thresholds analyze.Thresholds) model.Report {
	started := time.Now()
	snapshot := c.Snapshot()
	if duration > 0 {
		timer := time.NewTimer(duration)
		<-timer.C
		snapshot = c.Snapshot()
	}
	return model.Report{SchemaVersion: model.SchemaVersion, GeneratedAt: time.Now().UTC(), DurationMS: time.Since(started).Milliseconds(), Snapshot: snapshot, Findings: analyze.FindingsWithThresholds(snapshot, thresholds)}
}

func WriteJSON(w io.Writer, result model.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteText(w io.Writer, result model.Report) error {
	if _, err := fmt.Fprintf(w, "Hardware Resources Report (%s)\n\n", result.GeneratedAt.Format(time.RFC3339)); err != nil {
		return err
	}
	fmt.Fprintf(w, "CPU: %.1f%% user, %.1f%% system, %.1f%% iowait, %.1f%% idle; load %.2f/%.2f/%.2f\n", result.Snapshot.CPU.UserPercent, result.Snapshot.CPU.SystemPercent, result.Snapshot.CPU.IOWaitPercent, result.Snapshot.CPU.IdlePercent, result.Snapshot.CPU.Load1, result.Snapshot.CPU.Load5, result.Snapshot.CPU.Load15)
	fmt.Fprintf(w, "Memory: %.1f%% used, %.1f GiB available\n", result.Snapshot.Memory.UsedPercent, float64(result.Snapshot.Memory.AvailableBytes)/(1024*1024*1024))
	fmt.Fprintf(w, "System: governor %q, THP %q, swappiness %d, NUMA nodes %d, remote events %d/s\n", result.Snapshot.System.CPUGovernor, result.Snapshot.System.THP, result.Snapshot.System.Swappiness, result.Snapshot.NUMA.Nodes, result.Snapshot.NUMA.RemoteEvents)
	fmt.Fprintf(w, "Limits: current files %d, host/init files %d, current processes %d, host/init processes %d\n", result.Snapshot.System.OpenFiles, result.Snapshot.System.HostLimits.OpenFiles, result.Snapshot.System.MaxProcesses, result.Snapshot.System.HostLimits.MaxProcesses)
	if len(result.Snapshot.System.Sysctls) > 0 {
		fmt.Fprintf(w, "Sysctls: %v\n", result.Snapshot.System.Sysctls)
	}
	for _, filesystem := range result.Snapshot.Filesystems {
		mode := "rw"
		if filesystem.ReadOnly {
			mode = "ro"
		}
		fmt.Fprintf(w, "Filesystem: %s %s %.1f%% used, %.1f GiB available\n", filesystem.MountPoint, mode, filesystem.UsedPercent, float64(filesystem.AvailableBytes)/(1024*1024*1024))
	}
	for _, network := range result.Snapshot.Networks {
		fmt.Fprintf(w, "Network: %s %s %.1f/%.1f KiB/s, speed %d Mb/s, rings %d/%d\n", network.Name, network.State, network.RXBytesPerSec/1024, network.TXBytesPerSec/1024, network.LinkSpeedMbps, network.RXRingSize, network.TXRingSize)
	}
	fmt.Fprintf(w, "Hardware: %d PCI devices, %d NVIDIA GPUs, %d memory devices\n", len(result.Snapshot.PCI), len(result.Snapshot.GPUs), len(result.Snapshot.MemoryDevices))
	fmt.Fprintf(w, "Findings: %d\n", len(result.Findings))
	for _, finding := range result.Findings {
		fmt.Fprintf(w, "- [%s] %s: %s\n  Recommendation: %s\n", finding.Severity, finding.Title, finding.Evidence, finding.Recommendation)
	}
	if len(result.Snapshot.Errors) > 0 {
		fmt.Fprintf(w, "Collector errors: %d\n", len(result.Snapshot.Errors))
	}
	return nil
}

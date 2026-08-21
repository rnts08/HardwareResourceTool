package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

// ReadReport decodes a capture written by WriteJSON. Unknown fields from a
// capture produced by a newer schema version are ignored so historical
// comparison stays tolerant.
func ReadReport(r io.Reader) (model.Report, error) {
	var result model.Report
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&result); err != nil {
		return model.Report{}, err
	}
	return result, nil
}

func powerAlarm(power model.PowerSensor) string {
	if power.Alarm {
		return ", ALARM"
	}
	return ""
}

func WriteText(w io.Writer, result model.Report) error {
	if _, err := fmt.Fprintf(w, "Hardware Resources Report (%s)\n\n", result.GeneratedAt.Format(time.RFC3339)); err != nil {
		return err
	}
	fmt.Fprintf(w, "CPU: %.1f%% user, %.1f%% system, %.1f%% iowait, %.1f%% idle; load %.2f/%.2f/%.2f\n", result.Snapshot.CPU.UserPercent, result.Snapshot.CPU.SystemPercent, result.Snapshot.CPU.IOWaitPercent, result.Snapshot.CPU.IdlePercent, result.Snapshot.CPU.Load1, result.Snapshot.CPU.Load5, result.Snapshot.CPU.Load15)
	fmt.Fprintf(w, "Memory: %.1f%% used, %.1f GiB available\n", result.Snapshot.Memory.UsedPercent, float64(result.Snapshot.Memory.AvailableBytes)/(1024*1024*1024))
	if result.Snapshot.Memory.HugepagesTotal > 0 {
		fmt.Fprintf(w, "Hugetlb pool: %d/%d free x %.1f MiB\n", result.Snapshot.Memory.HugepagesFree, result.Snapshot.Memory.HugepagesTotal, float64(result.Snapshot.Memory.HugepageSizeBytes)/(1024*1024))
	}
	for _, node := range result.Snapshot.NUMA.NodeHugepages {
		fmt.Fprintf(w, "  NUMA node %d hugetlb: %d/%d free x %.1f MiB\n", node.Node, node.Free, node.Total, float64(node.SizeBytes)/(1024*1024))
	}
	fmt.Fprintf(w, "System: governor %q, THP %q, swappiness %d, NUMA nodes %d, remote events %d/s\n", result.Snapshot.System.CPUGovernor, result.Snapshot.System.THP, result.Snapshot.System.Swappiness, result.Snapshot.NUMA.Nodes, result.Snapshot.NUMA.RemoteEvents)
	if events := result.Snapshot.System.KernelEvents; len(events.Recent) > 0 || events.OOM+events.IOErrors+events.PCIeErrors+events.Hardware+events.NVIDIA+events.StorageResets+events.LinkFailures > 0 {
		fmt.Fprintf(w, "Kernel events: OOM %d, I/O %d, PCIe %d, hardware %d, NVIDIA %d, storage resets %d, link failures %d\n", events.OOM, events.IOErrors, events.PCIeErrors, events.Hardware, events.NVIDIA, events.StorageResets, events.LinkFailures)
		for _, event := range events.Recent {
			fmt.Fprintf(w, "  Kernel event: %s\n", event)
		}
	}
	if result.Snapshot.Virtualization.QEMUDetected || result.Snapshot.Virtualization.KVMAvailable || len(result.Snapshot.Virtualization.VirtualMachines) > 0 {
		fmt.Fprintf(w, "Virtualization: %s, VMs %d, allocated vCPU %d (%.2fx), memory %.1f GiB (%.2fx)\n", result.Snapshot.Virtualization.Hypervisor, len(result.Snapshot.Virtualization.VirtualMachines), result.Snapshot.Virtualization.AllocatedVCPUs, result.Snapshot.Virtualization.VCPUOvercommitRatio, float64(result.Snapshot.Virtualization.AllocatedMemoryBytes)/(1024*1024*1024), result.Snapshot.Virtualization.MemoryOvercommitRatio)
		for _, vm := range result.Snapshot.Virtualization.VirtualMachines {
			fmt.Fprintf(w, "VM: %s %s running=%t vCPU %d/%d CPU %.1f%% cgroup %.1f%% memory %.1f GiB current %.1f GiB RSS %.1f MiB huge/hugetlb %.1f/%.1f MiB QMP %s base/plug %.1f/%.1f GiB I/O %.1f/%.1f MiB balloon actual %.1f GiB reclaimed %.1f GiB target %.1f GiB committed %.1f GiB available %.1f GiB source %s status %s NUMA %v hugepages=%t\n", vm.Name, vm.Source, vm.Running, vm.ConfiguredVCPUs, vm.QMPEnabledVCPUs, vm.CPUPercent, vm.CgroupCPUPercent, float64(vm.ConfiguredMemoryBytes)/(1024*1024*1024), float64(vm.MemoryCurrentBytes)/(1024*1024*1024), float64(vm.ProcessRSSBytes)/(1024*1024), float64(vm.RuntimeAnonHugeBytes)/(1024*1024), float64(vm.RuntimeHugetlbBytes)/(1024*1024), vm.QMPVersion, float64(vm.QMPBaseMemoryBytes)/(1024*1024*1024), float64(vm.QMPPluggedMemoryBytes)/(1024*1024*1024), float64(vm.ReadBytes)/(1024*1024), float64(vm.WriteBytes)/(1024*1024), float64(vm.BalloonActualBytes)/(1024*1024*1024), float64(vm.BalloonReclaimedBytes)/(1024*1024*1024), float64(vm.BalloonTargetBytes)/(1024*1024*1024), float64(vm.BalloonCommittedBytes)/(1024*1024*1024), float64(vm.BalloonAvailableBytes)/(1024*1024*1024), vm.BalloonSource, vm.QMPStatus, vm.NUMANodes, vm.Hugepages)
			for _, disk := range vm.Disks {
				fmt.Fprintf(w, "  VM disk: %s %s (%s)\n", disk.Target, disk.Source, disk.Bus)
			}
			if len(vm.QMPBlockDevices) > 0 {
				fmt.Fprintf(w, "  VM QMP block I/O: read %.1f MiB/%d ops, write %.1f MiB/%d ops\n", float64(vm.QMPBlockReadBytes)/(1024*1024), vm.QMPBlockReadOps, float64(vm.QMPBlockWriteBytes)/(1024*1024), vm.QMPBlockWriteOps)
				for _, stat := range vm.QMPBlockDevices {
					fmt.Fprintf(w, "    %s node=%s rd %.1f MiB/%d ops wr %.1f MiB/%d ops\n", stat.Device, stat.NodeName, float64(stat.ReadBytes)/(1024*1024), stat.ReadOps, float64(stat.WriteBytes)/(1024*1024), stat.WriteOps)
				}
			}
			for _, nic := range vm.NICs {
				fmt.Fprintf(w, "  VM NIC: %s %s host=%s rx %.1f/tx %.1f KiB/s MAC %s\n", nic.Target, nic.Source, nic.HostNetwork, nic.RXBytesPerSecond/1024, nic.TXBytesPerSecond/1024, nic.MAC)
			}
			if len(vm.PCIAddresses) > 0 {
				fmt.Fprintf(w, "  VM PCI devices: %v\n", vm.PCIAddresses)
			}
		}
	}
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
		fmt.Fprintf(w, "Network: %s %s PCI %s %.1f/%.1f KiB/s, speed %d Mb/s, driver %s v%s fw %s bus %s, port %s phy %d xcvr %s mdix %s, duplex %s, FEC %s, rings %d/%d, channels %d/%d/%d, pause %t/%t, timestamping %t PHC %d, features %d/%d/%d, coalesce %dus/%dus, rss %s, stats %d\n", network.Name, network.State, network.PCIAddress, network.RXBytesPerSec/1024, network.TXBytesPerSec/1024, network.LinkSpeedMbps, network.Driver, network.DriverVersion, network.FWVersion, network.BusInfo, network.LinkPort, network.PHYAddress, network.Transceiver, network.TPMDIX, network.LinkDuplex, network.FECActive, network.RXRingSize, network.TXRingSize, network.MaxRXChannels, network.MaxTXChannels, network.MaxCombinedChannels, network.RXPause, network.TXPause, network.Timestamping, network.PHCIndex, len(network.FeaturesHardware), len(network.FeaturesActive), len(network.FeaturesWanted), network.CoalesceRXUsecs, network.CoalesceTXUsecs, network.RSSHashFunc, len(network.DriverStats))
	}
	if result.Snapshot.VirtualNetworkCount > 0 {
		fmt.Fprintf(w, "Virtual/device-less network interfaces filtered: %d\n", result.Snapshot.VirtualNetworkCount)
	}
	for _, device := range result.Snapshot.PCI {
		if len(device.Capabilities) > 0 || device.AERUncorrectableStatus != 0 || device.AERCorrectableStatus != 0 || device.PCIePathBottleneck != "" {
			fmt.Fprintf(w, "PCIe: %s capabilities %v, link %s x%d negotiated %s x%d, path minimum %s x%d @%s, BARs %d/%d bytes%s, PF %s, VFs %v, payload %d/%d bytes, AER UE 0x%08x CE 0x%08x\n", device.Address, device.Capabilities, device.PCIeCapabilityMaxSpeed, device.PCIeCapabilityMaxWidth, device.PCIeNegotiatedSpeed, device.PCIeNegotiatedWidth, device.PCIePathMinSpeed, device.PCIePathMinWidth, device.PCIePathBottleneck, device.BARCount, device.BARTotalBytes, pciBARReportSuffix(device), device.PCIePFAddress, device.PCIeVFAddresses, device.PCIeMaxPayloadBytes, device.PCIeMaxReadRequestBytes, device.AERUncorrectableStatus, device.AERCorrectableStatus)
		}
	}
	for _, gpu := range result.Snapshot.GPUs {
		if gpu.PassedThrough {
			fmt.Fprintf(w, "GPU: %s %s passed through (%s); host NVML telemetry unavailable\n", gpu.Address, gpu.Name, gpu.NVMLStatus)
		} else if gpu.NVML {
			fmt.Fprintf(w, "GPU: %s %s NVML available memory %.1f/%.1f GiB (%s; process %.1f GiB) util %.1f%% temp %.1fC power %.1fW ECC %t %d/%d MIG %t max-instances %d\n", gpu.Address, gpu.Name, float64(gpu.MemoryUsedBytes)/(1024*1024*1024), float64(gpu.MemoryBytes)/(1024*1024*1024), gpu.MemorySource, float64(gpu.MemoryProcessBytes)/(1024*1024*1024), gpu.UtilizationPercent, gpu.TemperatureCelsius, gpu.PowerWatts, gpu.ECCEnabled, gpu.ECCCorrected, gpu.ECCUncorrected, gpu.MIGEnabled, gpu.MIGMaxInstances)
			for _, mig := range gpu.MIGInstances {
				fmt.Fprintf(w, "  MIG instance %d: profile %s gi %d memory %.1f/%.1f GiB (process %.1f GiB) util %.1f%% temp %.1fC power %.1fW\n", mig.Index, mig.Profile, mig.GPUInstanceID, float64(mig.MemoryUsedBytes)/(1024*1024*1024), float64(mig.MemoryBytes)/(1024*1024*1024), float64(mig.ProcessMemoryBytes)/(1024*1024*1024), mig.UtilizationPercent, mig.TemperatureCelsius, mig.PowerWatts)
			}
			if gpu.NvLinkCount > 0 {
				fmt.Fprintf(w, "  NVLink: version %s links %d nominal %d GB/s per link\n", gpu.NvLinkVersion, gpu.NvLinkCount, gpu.NvLinkBandwidthGBps)
				for _, link := range gpu.NvLinks {
					remote := link.RemotePCI
					if remote == "" {
						remote = "n/a"
					}
					fmt.Fprintf(w, "  NVLink %d: %s -> %s (%s) read %.1f GiB write %.1f GiB\n", link.Index, linkStateLabel(link.Active), remote, link.RemoteDevice, float64(link.ReadBytes)/(1024*1024*1024), float64(link.WriteBytes)/(1024*1024*1024))
				}
			}
		} else {
			fmt.Fprintf(w, "GPU: %s NVML unavailable (%s)\n", gpu.Address, gpu.NVMLStatus)
		}
	}
	fmt.Fprintf(w, "Hardware: %d PCI devices, %d NVIDIA GPUs, %d memory devices\n", len(result.Snapshot.PCI), len(result.Snapshot.GPUs), len(result.Snapshot.MemoryDevices))
	if len(result.Snapshot.Thermal.Zones) > 0 || len(result.Snapshot.Thermal.Sensors) > 0 || len(result.Snapshot.Thermal.Fans) > 0 || len(result.Snapshot.Thermal.Power) > 0 {
		fmt.Fprintf(w, "Thermal: %d zones, %d temperature sensors, %d fans, %d power/energy sensors\n", len(result.Snapshot.Thermal.Zones), len(result.Snapshot.Thermal.Sensors), len(result.Snapshot.Thermal.Fans), len(result.Snapshot.Thermal.Power))
		for _, zone := range result.Snapshot.Thermal.Zones {
			fmt.Fprintf(w, "  Thermal zone: %s %s %.1f C, critical %.1f C, passive %.1f C, policy %s, mode %s\n", zone.Name, zone.Type, zone.Current, zone.Critical, zone.Passive, zone.Policy, zone.Mode)
		}
		for _, sensor := range result.Snapshot.Thermal.Sensors {
			alarm := ""
			if sensor.Alarm {
				alarm = ", ALARM"
			}
			fmt.Fprintf(w, "  Temperature: %s %s %s %.1f C, max %.1f C, critical %.1f C%s\n", sensor.Name, sensor.Label, sensor.Source, sensor.Current, sensor.Max, sensor.Critical, alarm)
		}
		for _, fan := range result.Snapshot.Thermal.Fans {
			fmt.Fprintf(w, "  Fan: %s %s %d RPM, min %d, max %d\n", fan.Name, fan.Label, fan.Input, fan.Min, fan.Max)
		}
		for _, power := range result.Snapshot.Thermal.Power {
			if power.InputWatts > 0 {
				fmt.Fprintf(w, "  Power: %s %s %.1f W, cap %.1f W, cap-max %.1f W%s\n", power.Name, power.Label, power.InputWatts, power.CapWatts, power.CapMaxWatts, powerAlarm(power))
			}
			if power.InputJoules > 0 {
				fmt.Fprintf(w, "  Energy: %s %s %.1f J\n", power.Name, power.Label, power.InputJoules)
			}
		}
	}
	fmt.Fprintf(w, "Findings: %d\n", len(result.Findings))
	for _, finding := range result.Findings {
		fmt.Fprintf(w, "- [%s] %s: %s\n  Recommendation: %s\n", finding.Severity, finding.Title, finding.Evidence, finding.Recommendation)
	}
	if len(result.Snapshot.Errors) > 0 {
		fmt.Fprintf(w, "Collector errors: %d\n", len(result.Snapshot.Errors))
	}
	return nil
}

// pciBARReportSuffix summarizes the BAR composition for the text report: ROM
// presence, prefetchable/64-bit/IO BAR counts, and bridge resource windows.
func pciBARReportSuffix(device model.PCIDevice) string {
	var parts []string
	if device.ROM {
		parts = append(parts, "rom")
	}
	prefetch, memory64, io := 0, 0, 0
	for _, bar := range device.BARs {
		if bar.Prefetchable {
			prefetch++
		}
		switch bar.Type {
		case "io":
			io++
		case "64-bit memory":
			memory64++
		}
	}
	if prefetch > 0 {
		parts = append(parts, fmt.Sprintf("prefetch %d", prefetch))
	}
	if memory64 > 0 {
		parts = append(parts, fmt.Sprintf("64-bit %d", memory64))
	}
	if io > 0 {
		parts = append(parts, fmt.Sprintf("io %d", io))
	}
	if len(device.ResourceWindows) > 0 {
		parts = append(parts, fmt.Sprintf("windows %d", len(device.ResourceWindows)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// linkStateLabel renders an NVLink link state for the text report.
func linkStateLabel(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

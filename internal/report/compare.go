package report

import (
	"fmt"
	"io"
	"sort"

	"hardware-resources-tool/internal/model"
)

// Comparison holds the results of comparing an older and a newer capture.
type Comparison struct {
	Older           model.Report
	Newer           model.Report
	NewFindings     []model.Finding
	ClearedFindings []model.Finding
	Categories      []CategoryDelta
}

type CategoryDelta struct {
	Name    string
	Values  []ValueDelta
	Summary string
}

type ValueDelta struct {
	Name string
	Old  float64
	New  float64
	Unit string
	Note string
}

// Compare diffs two captures by finding title and by per-resource rate.
func Compare(older, newer model.Report) Comparison {
	comparison := Comparison{Older: older, Newer: newer}
	oldIndex := findingsByTitle(older.Findings)
	for _, finding := range newer.Findings {
		if _, ok := oldIndex[finding.Title]; !ok {
			comparison.NewFindings = append(comparison.NewFindings, finding)
		}
	}
	newIndex := findingsByTitle(newer.Findings)
	for _, finding := range older.Findings {
		if _, ok := newIndex[finding.Title]; !ok {
			comparison.ClearedFindings = append(comparison.ClearedFindings, finding)
		}
	}

	olderS, newerS := older.Snapshot, newer.Snapshot
	comparison.Categories = append(comparison.Categories,
		cpuCategory(olderS, newerS),
		memoryCategory(olderS, newerS),
		systemCategory(olderS, newerS),
		virtualizationCategory(olderS, newerS),
	)
	if disks := diskCategory(olderS, newerS); len(disks.Values) > 0 || disks.Summary != "" {
		comparison.Categories = append(comparison.Categories, disks)
	}
	if networks := networkCategory(olderS, newerS); len(networks.Values) > 0 || networks.Summary != "" {
		comparison.Categories = append(comparison.Categories, networks)
	}
	if thermal := thermalCategory(olderS, newerS); len(thermal.Values) > 0 || thermal.Summary != "" {
		comparison.Categories = append(comparison.Categories, thermal)
	}
	if gpus := gpuCategory(olderS, newerS); len(gpus.Values) > 0 || gpus.Summary != "" {
		comparison.Categories = append(comparison.Categories, gpus)
	}
	if usb := usbCategory(olderS, newerS); usb.Summary != "" {
		comparison.Categories = append(comparison.Categories, usb)
	}
	if cpufreq := cpufreqCategory(olderS, newerS); cpufreq.Summary != "" {
		comparison.Categories = append(comparison.Categories, cpufreq)
	}
	return comparison
}

func findingsByTitle(findings []model.Finding) map[string]model.Finding {
	result := make(map[string]model.Finding, len(findings))
	for _, finding := range findings {
		result[finding.Title] = finding
	}
	return result
}

func cpuCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "CPU"}
	delta.Values = []ValueDelta{
		{Name: "user", Old: older.CPU.UserPercent, New: newer.CPU.UserPercent, Unit: "%"},
		{Name: "system", Old: older.CPU.SystemPercent, New: newer.CPU.SystemPercent, Unit: "%"},
		{Name: "iowait", Old: older.CPU.IOWaitPercent, New: newer.CPU.IOWaitPercent, Unit: "%"},
		{Name: "idle", Old: older.CPU.IdlePercent, New: newer.CPU.IdlePercent, Unit: "%"},
		{Name: "load1", Old: older.CPU.Load1, New: newer.CPU.Load1},
		{Name: "ctxt/s", Old: float64(older.CPU.ContextSwitch), New: float64(newer.CPU.ContextSwitch), Unit: "/s"},
		{Name: "intr/s", Old: float64(older.CPU.Interrupts), New: float64(newer.CPU.Interrupts), Unit: "/s"},
	}
	return delta
}

func memoryCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "Memory"}
	delta.Values = []ValueDelta{
		{Name: "used", Old: older.Memory.UsedPercent, New: newer.Memory.UsedPercent, Unit: "%"},
		{Name: "available", Old: bytesGiB(older.Memory.AvailableBytes), New: bytesGiB(newer.Memory.AvailableBytes), Unit: "GiB"},
		{Name: "swap in/s", Old: float64(older.Memory.SwapInPerSec), New: float64(newer.Memory.SwapInPerSec), Unit: "/s"},
		{Name: "swap out/s", Old: float64(older.Memory.SwapOutPerSec), New: float64(newer.Memory.SwapOutPerSec), Unit: "/s"},
	}
	return delta
}

func systemCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "System"}
	delta.Values = []ValueDelta{
		{Name: "NUMA remote/s", Old: float64(older.NUMA.RemoteEvents), New: float64(newer.NUMA.RemoteEvents), Unit: "/s"},
	}
	oldEvents, newEvents := older.System.KernelEvents, newer.System.KernelEvents
	delta.Values = append(delta.Values,
		ValueDelta{Name: "kernel OOM", Old: float64(oldEvents.OOM), New: float64(newEvents.OOM)},
		ValueDelta{Name: "kernel I/O errors", Old: float64(oldEvents.IOErrors), New: float64(newEvents.IOErrors)},
		ValueDelta{Name: "kernel PCIe errors", Old: float64(oldEvents.PCIeErrors), New: float64(newEvents.PCIeErrors)},
		ValueDelta{Name: "kernel hardware", Old: float64(oldEvents.Hardware), New: float64(newEvents.Hardware)},
		ValueDelta{Name: "kernel storage resets", Old: float64(oldEvents.StorageResets), New: float64(newEvents.StorageResets)},
		ValueDelta{Name: "kernel link failures", Old: float64(oldEvents.LinkFailures), New: float64(newEvents.LinkFailures)},
	)
	return delta
}

func virtualizationCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "Virtualization"}
	delta.Values = []ValueDelta{
		{Name: "VMs", Old: float64(len(older.Virtualization.VirtualMachines)), New: float64(len(newer.Virtualization.VirtualMachines))},
		{Name: "allocated vCPU", Old: float64(older.Virtualization.AllocatedVCPUs), New: float64(newer.Virtualization.AllocatedVCPUs)},
		{Name: "allocated memory", Old: bytesGiB(older.Virtualization.AllocatedMemoryBytes), New: bytesGiB(newer.Virtualization.AllocatedMemoryBytes), Unit: "GiB"},
		{Name: "vCPU overcommit", Old: older.Virtualization.VCPUOvercommitRatio, New: newer.Virtualization.VCPUOvercommitRatio, Unit: "x"},
		{Name: "memory overcommit", Old: older.Virtualization.MemoryOvercommitRatio, New: newer.Virtualization.MemoryOvercommitRatio, Unit: "x"},
	}
	return delta
}

func diskCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "Disks"}
	oldDisks := make(map[string]model.Disk)
	for _, disk := range older.Disks {
		oldDisks[disk.Name] = disk
	}
	newDisks := make(map[string]model.Disk)
	for _, disk := range newer.Disks {
		newDisks[disk.Name] = disk
	}
	for _, disk := range newer.Disks {
		if previous, ok := oldDisks[disk.Name]; ok {
			delta.Values = append(delta.Values,
				ValueDelta{Name: disk.Name + " read", Old: bytesPerSecMiB(previous.ReadBytesPerSec), New: bytesPerSecMiB(disk.ReadBytesPerSec), Unit: "MiB/s"},
				ValueDelta{Name: disk.Name + " write", Old: bytesPerSecMiB(previous.WriteBytesPerSec), New: bytesPerSecMiB(disk.WriteBytesPerSec), Unit: "MiB/s"},
			)
		} else {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("new disk %s (%.1f MiB/s read, %.1f MiB/s write)", disk.Name, bytesPerSecMiB(disk.ReadBytesPerSec), bytesPerSecMiB(disk.WriteBytesPerSec)))
		}
	}
	for _, disk := range older.Disks {
		if _, ok := newDisks[disk.Name]; !ok {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("disk %s no longer present", disk.Name))
		}
	}
	return delta
}

func networkCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "Networks"}
	oldNetworks := make(map[string]model.Network)
	for _, network := range older.Networks {
		oldNetworks[network.Name] = network
	}
	newNetworks := make(map[string]model.Network)
	for _, network := range newer.Networks {
		newNetworks[network.Name] = network
	}
	for _, network := range newer.Networks {
		if previous, ok := oldNetworks[network.Name]; ok {
			delta.Values = append(delta.Values,
				ValueDelta{Name: network.Name + " rx", Old: bytesPerSecMiB(previous.RXBytesPerSec), New: bytesPerSecMiB(network.RXBytesPerSec), Unit: "MiB/s"},
				ValueDelta{Name: network.Name + " tx", Old: bytesPerSecMiB(previous.TXBytesPerSec), New: bytesPerSecMiB(network.TXBytesPerSec), Unit: "MiB/s"},
			)
		} else {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("new network %s (state %s)", network.Name, network.State))
		}
	}
	for _, network := range older.Networks {
		if _, ok := newNetworks[network.Name]; !ok {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("network %s no longer present", network.Name))
		}
	}
	return delta
}

func thermalCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "Thermal"}
	oldZones := make(map[string]model.ThermalZone)
	for _, zone := range older.Thermal.Zones {
		oldZones[zone.Name] = zone
	}
	for _, zone := range newer.Thermal.Zones {
		if previous, ok := oldZones[zone.Name]; ok {
			delta.Values = append(delta.Values, ValueDelta{Name: zone.Name + " temp", Old: previous.Current, New: zone.Current, Unit: "C"})
		}
	}
	return delta
}

func bytesGiB(bytes uint64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024)
}

// gpuCategory diffs per-GPU telemetry by PCI address and reports GPUs that
// appeared or disappeared between captures. Hosts without NVML on either
// side contribute no values.
func gpuCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "GPUs"}
	oldGPUs := make(map[string]model.GPU)
	for _, gpu := range older.GPUs {
		oldGPUs[gpu.Address] = gpu
	}
	newGPUs := make(map[string]model.GPU)
	for _, gpu := range newer.GPUs {
		newGPUs[gpu.Address] = gpu
	}
	for _, gpu := range newer.GPUs {
		previous, ok := oldGPUs[gpu.Address]
		if !ok {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("new GPU %s (%s)", gpu.Address, orNone(gpu.Name)))
			continue
		}
		if !previous.NVML && !gpu.NVML {
			continue
		}
		delta.Values = append(delta.Values,
			ValueDelta{Name: gpu.Address + " util", Old: previous.UtilizationPercent, New: gpu.UtilizationPercent, Unit: "%"},
			ValueDelta{Name: gpu.Address + " temp", Old: previous.TemperatureCelsius, New: gpu.TemperatureCelsius, Unit: "C"},
			ValueDelta{Name: gpu.Address + " power", Old: previous.PowerWatts, New: gpu.PowerWatts, Unit: "W"},
			ValueDelta{Name: gpu.Address + " fb used", Old: bytesGiB(previous.MemoryUsedBytes), New: bytesGiB(gpu.MemoryUsedBytes), Unit: "GiB"},
		)
	}
	for _, gpu := range older.GPUs {
		if _, ok := newGPUs[gpu.Address]; !ok {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("GPU %s no longer present (%s)", gpu.Address, orNone(gpu.Name)))
		}
	}
	return delta
}

// usbCategory reports USB devices that appeared or disappeared between
// captures. Speeds are not diffed because they change with negotiation.
func usbCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "USB"}
	key := func(device model.USBDevice) string {
		return device.BusID + " " + device.VendorID + ":" + device.ProductID + " " + usbLabel(device)
	}
	oldDevices := make(map[string]bool)
	for _, device := range older.USB {
		oldDevices[key(device)] = true
	}
	newDevices := make(map[string]bool)
	for _, device := range newer.USB {
		newDevices[key(device)] = true
	}
	for _, device := range newer.USB {
		id := key(device)
		if !oldDevices[id] {
			delta.Summary = appendSummary(delta.Summary, "new USB device "+id)
		}
	}
	for _, device := range older.USB {
		id := key(device)
		if !newDevices[id] {
			delta.Summary = appendSummary(delta.Summary, "USB device no longer present: "+id)
		}
	}
	return delta
}

func usbLabel(device model.USBDevice) string {
	if device.Product != "" {
		return "(" + device.Product + ")"
	}
	return "(unknown device)"
}

// cpufreqCategory reports governor and EPP changes per policy so power-policy
// drift between captures is visible.
func cpufreqCategory(older, newer model.Snapshot) CategoryDelta {
	delta := CategoryDelta{Name: "CPU frequency policy"}
	oldPolicies := make(map[string]model.CPUPolicy)
	for _, policy := range older.CPUPower {
		oldPolicies[policy.Policy] = policy
	}
	for _, policy := range newer.CPUPower {
		previous, ok := oldPolicies[policy.Policy]
		if !ok {
			continue
		}
		if previous.Governor != policy.Governor {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("%s governor %s -> %s", policy.Policy, orNone(previous.Governor), orNone(policy.Governor)))
		}
		if previous.EPP != policy.EPP {
			delta.Summary = appendSummary(delta.Summary, fmt.Sprintf("%s EPP %s -> %s", policy.Policy, orNone(previous.EPP), orNone(policy.EPP)))
		}
	}
	return delta
}

// orNone renders an empty value as (none) for comparison summaries.
func orNone(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

func bytesPerSecMiB(bytesPerSecond float64) float64 {
	return bytesPerSecond / (1024 * 1024)
}

func appendSummary(summary, entry string) string {
	if summary == "" {
		return entry
	}
	return summary + "; " + entry
}

// WriteComparison writes a human-readable before/after comparison.
func WriteComparison(w io.Writer, comparison Comparison) error {
	older, newer := comparison.Older, comparison.Newer
	if _, err := fmt.Fprintf(w, "Capture comparison: %s -> %s\n\n", older.GeneratedAt.Format("2006-01-02 15:04:05"), newer.GeneratedAt.Format("2006-01-02 15:04:05")); err != nil {
		return err
	}
	if len(comparison.NewFindings) == 0 && len(comparison.ClearedFindings) == 0 {
		fmt.Fprintf(w, "Findings: unchanged (%d present in both captures)\n", len(older.Findings))
	} else {
		if len(comparison.NewFindings) > 0 {
			fmt.Fprintf(w, "New findings: %d\n", len(comparison.NewFindings))
			for _, finding := range comparison.NewFindings {
				fmt.Fprintf(w, "- [%s] %s: %s\n", finding.Severity, finding.Title, finding.Evidence)
			}
		}
		if len(comparison.ClearedFindings) > 0 {
			fmt.Fprintf(w, "Cleared findings: %d\n", len(comparison.ClearedFindings))
			for _, finding := range comparison.ClearedFindings {
				fmt.Fprintf(w, "- [%s] %s\n", finding.Severity, finding.Title)
			}
		}
	}
	fmt.Fprintln(w)
	for _, category := range comparison.Categories {
		displayed := make([]ValueDelta, 0, len(category.Values))
		for _, value := range category.Values {
			if value.Old != 0 || value.New != 0 {
				displayed = append(displayed, value)
			}
		}
		if len(displayed) == 0 && category.Summary == "" {
			continue
		}
		fmt.Fprintf(w, "%s\n", category.Name)
		sort.Slice(displayed, func(i, j int) bool { return displayed[i].Name < displayed[j].Name })
		for _, value := range displayed {
			oldValue := formatDeltaValue(value.Old, value.Unit)
			newValue := formatDeltaValue(value.New, value.Unit)
			var deltaText string
			switch value.Unit {
			case "%":
				deltaText = fmt.Sprintf("%+.1f pts", value.New-value.Old)
			case "":
				deltaText = fmt.Sprintf("%+.1f", value.New-value.Old)
			default:
				deltaText = fmt.Sprintf("%+.1f %s", value.New-value.Old, value.Unit)
			}
			note := ""
			if value.Note != "" {
				note = "  " + value.Note
			}
			fmt.Fprintf(w, "  %-20s %10s -> %10s  %s%s\n", value.Name, oldValue, newValue, deltaText, note)
		}
		if category.Summary != "" {
			fmt.Fprintf(w, "  %s\n", category.Summary)
		}
	}
	return nil
}

func formatDeltaValue(value float64, unit string) string {
	text := fmt.Sprintf("%.1f", value)
	if unit == "" {
		return text
	}
	return text + " " + unit
}

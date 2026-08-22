package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"hardware-resources-tool/internal/model"
)

func render(t *testing.T, result model.Report) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteText(&buf, result); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestWriteTextEmptyHypervisorShowsNoneDetected(t *testing.T) {
	result := model.Report{Snapshot: model.Snapshot{Virtualization: model.Virtualization{KVMAvailable: true}}}
	view := render(t, result)
	if !strings.Contains(view, "Virtualization: platform none detected, VMs 0") {
		t.Fatalf("empty platform not handled: %s", view)
	}
}

func TestWriteTextOmitsInactiveVMSegments(t *testing.T) {
	quiet := model.Report{Snapshot: model.Snapshot{Virtualization: model.Virtualization{
		QEMUDetected: true,
		VirtualMachines: []model.VirtualMachine{{
			Name: "vm1", Source: "proc", Running: true, ConfiguredVCPUs: 4, CPUPercent: 12.5,
			ConfiguredMemoryBytes: 8 << 30,
		}},
	}}}
	view := render(t, quiet)
	vmLine := view[strings.Index(view, "VM: "):]
	vmLine = vmLine[:strings.Index(vmLine, "\n")]
	for _, noise := range []string{"QMP", "balloon", "huge/hugetlb", "I/O ", "NUMA"} {
		if strings.Contains(vmLine, noise) {
			t.Fatalf("inactive segment %q leaked into VM line: %s", noise, vmLine)
		}
	}
	rich := quiet
	vm := rich.Snapshot.Virtualization.VirtualMachines[0]
	vm.QMPVersion = "8.2.0"
	vm.BalloonActualBytes = 4 << 30
	vm.NUMANodes = []int{0}
	vm.Hugepages = true
	rich.Snapshot.Virtualization.VirtualMachines = []model.VirtualMachine{vm}
	view = render(t, rich)
	for _, expected := range []string{"QMP 8.2.0 base/plug", "balloon actual 4.0 GiB", "NUMA [0]", "hugepages=true"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("active segment %q missing: %s", expected, view)
		}
	}
}

func TestWriteTextSortsSysctlsAndFormatsLimits(t *testing.T) {
	result := model.Report{Snapshot: model.Snapshot{System: model.SystemSettings{
		OpenFiles:    1024,
		MaxProcesses: 100,
		Sysctls: map[string]string{
			"vm.dirty_ratio":            "20",
			"kernel.nmi_watchdog":       "1",
			"vm.dirty_background_ratio": "10",
		},
	}}}
	view := render(t, result)
	sysctlLine := view[strings.Index(view, "Sysctls:"):]
	sysctlLine = sysctlLine[:strings.Index(sysctlLine, "\n")]
	expected := "Sysctls: kernel.nmi_watchdog=1, vm.dirty_background_ratio=10, vm.dirty_ratio=20"
	if sysctlLine != expected {
		t.Fatalf("sysctls unsorted or malformed: %q", sysctlLine)
	}
	if !strings.Contains(view, "Limits: current files 1024, current processes 100\n") {
		t.Fatalf("limits line wrong when host limits absent: %s", view)
	}
}

func TestWriteTextDetailsCollectorErrors(t *testing.T) {
	result := model.Report{Snapshot: model.Snapshot{Errors: []string{"disks: no such file"}}}
	view := render(t, result)
	if !strings.Contains(view, "Collector errors: 1\n  - disks: no such file\n") {
		t.Fatalf("collector error detail missing: %s", view)
	}
}

func TestWriteTextThermalOmitsZeroLimits(t *testing.T) {
	result := model.Report{Snapshot: model.Snapshot{Thermal: model.Thermal{
		Zones:   []model.ThermalZone{{Name: "thermal_zone0", Type: "acpitz", Current: 45, Policy: "step_wise"}},
		Sensors: []model.Temperature{{Name: "hwmon0", Label: "Composite", Source: "nvme", Current: 33.9}},
	}}}
	view := render(t, result)
	for _, noise := range []string{"critical 0.0 C", "passive 0.0 C", "max 0.0 C"} {
		if strings.Contains(view, noise) {
			t.Fatalf("zero thermal limit %q printed: %s", noise, view)
		}
	}
	if !strings.Contains(view, "Thermal zone: thermal_zone0 acpitz 45.0 C, policy step_wise") {
		t.Fatalf("zone line malformed: %s", view)
	}
	if !strings.Contains(view, "Temperature: hwmon0 Composite nvme 33.9 C") {
		t.Fatalf("sensor line malformed: %s", view)
	}
}

func TestWriteTextUSBSection(t *testing.T) {
	result := model.Report{Snapshot: model.Snapshot{
		USB:             []model.USBDevice{{BusID: "1-2", VendorID: "8087", ProductID: "0026", Product: "AX201 Bluetooth", Manufacturer: "Intel Corp.", SpeedMbps: 12, Serial: "SN1"}},
		USBMonAvailable: true,
	}}
	view := render(t, result)
	if !strings.Contains(view, "USB: bus 1-2 id 8087:0026 AX201 Bluetooth (Intel Corp.) 12 Mb/s sn SN1") {
		t.Fatalf("usb line malformed: %s", view)
	}
	if !strings.Contains(view, "usbmon available") {
		t.Fatalf("usbmon line missing: %s", view)
	}
}

func TestWriteTextCpufreqPolicies(t *testing.T) {
	result := model.Report{Snapshot: model.Snapshot{CPUPower: []model.CPUPolicy{{
		Policy: "policy0", CPUs: "0-7", Governor: "performance", EPP: "balance_performance",
	}}}}
	view := render(t, result)
	if !strings.Contains(view, "Cpufreq: policy0 governor performance, EPP balance_performance, cpus 0-7") {
		t.Fatalf("cpufreq line missing: %s", view)
	}
}

func TestWriteTextCompressesIdenticalPolicies(t *testing.T) {
	policies := make([]model.CPUPolicy, 0, 3)
	for i := 0; i < 3; i++ {
		policies = append(policies, model.CPUPolicy{Policy: fmt.Sprintf("policy%d", i), CPUs: fmt.Sprintf("%d", i), Governor: "powersave", EPP: "performance"})
	}
	policies = append(policies, model.CPUPolicy{Policy: "policy3", CPUs: "3", Governor: "performance"})
	result := model.Report{Snapshot: model.Snapshot{CPUPower: policies}}
	view := render(t, result)
	if !strings.Contains(view, "Cpufreq: policies 0-2 governor powersave, EPP performance\n") {
		t.Fatalf("identical policies not compressed: %s", view)
	}
	if !strings.Contains(view, "Cpufreq: policy3 governor performance, cpus 3\n") {
		t.Fatalf("distinct policy lost: %s", view)
	}
	if strings.Count(view, "Cpufreq:") != 2 {
		t.Fatalf("expected 2 cpufreq lines: %s", view)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[uint64]string{
		0:       "0 B",
		512:     "512 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		1 << 20: "1.0 MiB",
		1 << 30: "1.0 GiB",
		3 << 30: "3.0 GiB",
	}
	for value, want := range cases {
		if got := humanBytes(value); got != want {
			t.Fatalf("humanBytes(%d) = %q, want %q", value, got, want)
		}
	}
}

func TestWriteTextHeaderIncludesDuration(t *testing.T) {
	result := model.Report{DurationMS: 3000}
	view := render(t, result)
	if !strings.Contains(view, ", collected over 3.0s)") {
		t.Fatalf("duration missing from header: %s", view[:120])
	}
}

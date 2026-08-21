package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/model"
)

func init() {
	lipgloss.SetColorProfile(termenv.ANSI)
}

func TestSparklineClampsAndPreservesSamples(t *testing.T) {
	got := sparkline([]float64{-10, 0, 50, 100, 110}, 0, 100)
	if got != "▁▁▄█"+"█" {
		t.Fatalf("unexpected sparkline %q", got)
	}
}

func TestViewShowsTabsAndEmptyState(t *testing.T) {
	view := stripANSI((modelState{tab: 1, thresholds: analyze.DefaultThresholds}).View())
	for _, expected := range []string{"[2 Processes]", "No process samples available.", "1-7: tabs"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func TestFindingsShortcutShowsFindings(t *testing.T) {
	view := stripANSI((modelState{findingsMode: true, thresholds: analyze.DefaultThresholds}).View())
	for _, expected := range []string{"No findings.", "f findings"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func TestViewShowsThermalTabAndEmptyState(t *testing.T) {
	view := stripANSI((modelState{tab: 6, thresholds: analyze.DefaultThresholds}).View())
	for _, expected := range []string{"[7 Thermal]", "No thermal sensors reported."} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

// stripANSI removes SGR sequences and inline emphasis sentinels so tests can
// assert on visible text.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		switch r {
		case '\x1b':
			inEscape = true
		case '\x03', '\x04':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestKernelEventDeltas(t *testing.T) {
	previous := model.KernelEvents{OOM: 2, IOErrors: 5, StorageResets: 1}
	current := model.KernelEvents{OOM: 2, IOErrors: 8, PCIeErrors: 3, Hardware: 1, StorageResets: 1}
	delta := kernelEventDeltas(previous, current)
	if delta.OOM != 0 || delta.IOErrors != 3 || delta.PCIeErrors != 3 || delta.Hardware != 1 || delta.StorageResets != 0 || delta.LinkFailures != 0 {
		t.Fatalf("unexpected kernel event deltas: %#v", delta)
	}
}

func TestViewTopShowsProcesses(t *testing.T) {
	snapshot := model.Snapshot{TopProcesses: []model.ProcessSample{{PID: 42, Name: "qemu-system-x86_64", CPUPercent: 95.5, RSSBytes: 1 << 30, State: "R"}}}
	view := stripANSI(viewTop(snapshot, nil, 0, false))
	for _, expected := range []string{"qemu-system-x86_64", "pid      42", "cpu   95.5%", "1.0 GiB"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func TestViewTopEmptyState(t *testing.T) {
	if !contains(viewTop(model.Snapshot{}, nil, 0, false), "No process samples available.") {
		t.Fatal("empty top view missing hint")
	}
}

func TestViewTopMarksQEMUProcesses(t *testing.T) {
	snapshot := model.Snapshot{
		TopProcesses: []model.ProcessSample{
			{PID: 42, Name: "qemu-system-x86_64", CPUPercent: 10},
			{PID: 99, Name: "worker", CPUPercent: 5},
			{PID: 1234, Name: "qemu", CPUPercent: 3},
		},
		Virtualization: model.Virtualization{VirtualMachines: []model.VirtualMachine{{Name: "guest-a", PID: 1234}}},
	}
	view := stripANSI(viewTop(snapshot, nil, 0, false))
	if count := strings.Count(view, "[QEMU]"); count != 2 {
		t.Fatalf("expected 2 QEMU markers, got %d: %s", count, view)
	}
	if !contains(view, "qemu-system-x86_64       pid      42") {
		t.Fatalf("qemu-system missing from view: %s", view)
	}
}

func TestPickerBuildsItemsForDevicesAndVMs(t *testing.T) {
	snapshot := model.Snapshot{
		Virtualization: model.Virtualization{VirtualMachines: []model.VirtualMachine{{Name: "vm-100", VMID: "100", PID: 1234, Running: true, Source: "proxmox"}}},
		GPUs:           []model.GPU{{Address: "0000:65:00.0", Name: "Tesla T4", NVMLStatus: "ok"}},
		PCI:            []model.PCIDevice{{Address: "0000:00:03.0", Class: "Ethernet", VendorID: "8086", DeviceID: "10fb"}},
		MemoryDevices:  []model.MemoryDevice{{Locator: "DIMM_A1", SizeBytes: 32 << 30, Type: "DDR4"}},
	}
	items := buildPicker(snapshot)
	if len(items) != 4 {
		t.Fatalf("expected 4 picker items, got %d: %#v", len(items), items)
	}
	if items[0].kind != "vm" || items[1].kind != "gpu" || items[2].kind != "pci" || items[3].kind != "memory" {
		t.Fatalf("unexpected picker order: %#v", items)
	}
}

func TestDetailForVMShowsBreakdown(t *testing.T) {
	vm := model.VirtualMachine{
		Name: "vm-100", VMID: "100", PID: 1234, Running: true, Source: "proxmox",
		ConfiguredVCPUs: 4, QMPEnabledVCPUs: 4, ConfiguredMemoryBytes: 8 << 30,
		CPUPercent: 25, MemoryCurrentBytes: 4 << 30, ProcessRSSBytes: 2 << 30,
		BalloonEnabled: true, BalloonActualBytes: 3 << 30, QMPAvailable: true, QMPStatus: "running",
		RuntimeAvailable: true, RuntimeNUMABytes: map[int]uint64{0: 1 << 30, 1: 2 << 30},
		Disks:        []model.VirtualDisk{{Target: "vda", Bus: "virtio", Source: "/var/lib/vz/images/100/disk.qcow2"}},
		NICs:         []model.VirtualNIC{{Type: "virtio", Source: "vmbr0", HostNetwork: "enp3s0", MAC: "52:54:00:00:00:01"}},
		PCIAddresses: []string{"0000:65:00.0"},
	}
	title, lines := vmDetail(vm)
	if title != "VM vm-100" {
		t.Fatalf("unexpected title: %q", title)
	}
	joined := strings.Join(lines, "\n")
	for _, expected := range []string{"balloon", "node0:1.0 GiB", "node1:2.0 GiB", "/var/lib/vz/images/100/disk.qcow2", "enp3s0", "0000:65:00.0", "QMP"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("detail missing %q: %s", expected, joined)
		}
	}
}

func TestTabAtXMapsTabs(t *testing.T) {
	m := modelState{tab: 3}
	columns := 0
	for i := range tabs {
		if tab := m.tabAtX(columns); tab != i {
			t.Fatalf("expected tab %d at column %d, got %d", i, columns, tab)
		}
		if tab := m.tabAtX(columns + 1); tab != i {
			t.Fatalf("expected tab %d at column %d, got %d", i, columns+1, tab)
		}
		columns += len(fmt.Sprintf(" %d %s  ", i+1, tabs[i]))
	}
	if tab := m.tabAtX(columns + 5); tab != -1 {
		t.Fatalf("expected no tab after the bar, got %d", tab)
	}
}

func TestViewThermalShowsPowerAndEnergy(t *testing.T) {
	snapshot := model.Snapshot{Thermal: model.Thermal{Power: []model.PowerSensor{{Name: "hwmon0", Sensor: "1", Label: "CPU package", InputWatts: 45, CapWatts: 125}, {Name: "hwmon0", Sensor: "1", InputJoules: 123.45}}}}
	view := stripANSI(viewThermal(snapshot))
	for _, expected := range []string{"Power / energy", "[    45.0] W", "cap  125.0 W", "123.5 J"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func contains(value, expected string) bool {
	for i := 0; i+len(expected) <= len(value); i++ {
		if value[i:i+len(expected)] == expected {
			return true
		}
	}
	return false
}

func TestHistoryIsBounded(t *testing.T) {
	m := modelState{history: make([]model.Snapshot, historyLimit+1)}
	if len(m.history) != historyLimit+1 {
		t.Fatal("test setup failed")
	}
	m.history = m.history[len(m.history)-historyLimit:]
	if len(m.history) != historyLimit {
		t.Fatalf("expected history limit %d, got %d", historyLimit, len(m.history))
	}
}

func TestRenderScrolledClipsHeightAndWidth(t *testing.T) {
	got := renderScrolled("H", "one\ntwo\nthree\nfour", "F", 4, 4, 0, 0)
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %#v", lines)
	}
	for _, line := range lines {
		if len([]rune(line)) > 4 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}

func TestRenderScrolledVerticalOffset(t *testing.T) {
	got := renderScrolled("H", "one\ntwo\nthree\nfour", "F", 0, 4, 2, 0)
	if !contains(got, "three") || !contains(got, "four") || contains(got, "one") {
		t.Fatalf("unexpected scroll window: %q", got)
	}
}

func TestRenderScrolledHorizontalOffset(t *testing.T) {
	if got := renderScrolled("", "abcdef", "", 4, 0, 0, 2); got != "cdef" {
		t.Fatalf("unexpected horizontal scroll: %q", got)
	}
	if got := renderScrolled("", "abcdef", "", 4, 0, 0, 4); got != "cdef" {
		t.Fatalf("unexpected horizontal truncation: %q", got)
	}
	if got := renderScrolled("", "abcdef", "", 3, 0, 0, 2); got != "cd…" {
		t.Fatalf("unexpected horizontal truncation: %q", got)
	}
}

func TestRenderScrolledHandlesTinyTerminal(t *testing.T) {
	got := renderScrolled("", "one\ntwo", "footer", 20, 1, 0, 0)
	if got != "one" {
		t.Fatalf("tiny terminal view = %q", got)
	}
}

func TestRenderScrolledClampsOffsetPastEnd(t *testing.T) {
	got := renderScrolled("", "one\ntwo\nthree", "", 0, 3, 99, 0)
	if !contains(got, "three") {
		t.Fatalf("offset was not clamped: %q", got)
	}
}

func TestFindingsSeverityColoring(t *testing.T) {
	for _, line := range []string{"  [critical] CPU is saturated", "  [warning] swap activity", "  [info] THP not active"} {
		if got := applyColor("", line); !strings.HasPrefix(got, "\x1b[") {
			t.Fatalf("finding was not colored: %q", got)
		}
	}
	if got := applyColor("", "  no severity"); strings.HasPrefix(got, "\x1b[") {
		t.Fatalf("plain line should not be colored: %q", got)
	}
}

func TestMarkerSurvivesHorizontalScroll(t *testing.T) {
	scrolled, marker := scrollLine(markCritical+"abcdef", 0, 3)
	if scrolled != "def" || marker != markCritical {
		t.Fatalf("marker did not survive scroll: text=%q marker=%q", scrolled, marker)
	}
}

func TestSwitchToPreservesScrollPositions(t *testing.T) {
	m := modelState{tab: 0, offset: 12, xoffset: 8}
	m.switchTo(3)
	if m.tab != 3 || m.offset != 0 || m.xoffset != 0 {
		t.Fatalf("unexpected switch state: tab %d offset %d xoffset %d", m.tab, m.offset, m.xoffset)
	}
	m.offset = 44
	m.switchTo(0)
	if m.tab != 0 || m.offset != 12 || m.xoffset != 8 {
		t.Fatalf("scroll position not restored: %#v", m)
	}
	m.switchTo(3)
	if m.offset != 44 {
		t.Fatalf("scroll position not remembered for target tab: %d", m.offset)
	}
}

func TestRenderScrolledPadsShortContentToFullHeight(t *testing.T) {
	view := renderScrolled("header\n", "one line", "footer", 80, 10, 0, 0)
	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines for a 10-row terminal, got %d", len(lines))
	}
}

func TestTabCountMatchesTabs(t *testing.T) {
	if len(tabs) != tabCount {
		t.Fatalf("tabs has %d entries but tabCount is %d", len(tabs), tabCount)
	}
}

func TestIndicatorMarksActiveMode(t *testing.T) {
	if got := stripANSI(indicator(true, "C")); got != "[C]" {
		t.Fatalf("active indicator = %q", got)
	}
	if got := stripANSI(indicator(false, "C")); got != " C " {
		t.Fatalf("inactive indicator = %q", got)
	}
}

func TestScrollLinePreservesEmphasis(t *testing.T) {
	line := "temp " + emph("[61.0]") + " ok"
	scrolled, _ := scrollLine(line, 80, 0)
	if !strings.Contains(scrolled, "\x1b[1m[61.0]\x1b[0m") {
		t.Fatalf("emphasis lost without scrolling: %q", scrolled)
	}
	// Scrolling to the exact start of the emphasized span keeps it intact.
	scrolled, _ = scrollLine(line, 80, 5)
	if !strings.Contains(stripANSI(scrolled), "[61.0]") {
		t.Fatalf("emphasis content lost when scrolled: %q", scrolled)
	}
}

func TestScrollLineDropsSplitEmphasis(t *testing.T) {
	line := strings.Repeat("x", 10) + emph("[61.0]") + " tail"
	// Cut lands inside the bold span; the sentinel pair is unbalanced and
	// must be dropped instead of leaking control runes.
	scrolled, _ := scrollLine(line, 14, 0)
	if strings.ContainsAny(scrolled, "\x03\x04") {
		t.Fatalf("unbalanced sentinels leaked: %q", scrolled)
	}
}

func TestThermalValuesAreBracketed(t *testing.T) {
	snapshot := model.Snapshot{Thermal: model.Thermal{
		Zones: []model.ThermalZone{{Name: "acpitz", Type: "critical", Current: 45.5, Critical: 100}},
	}}
	view := viewThermal(snapshot)
	if !strings.Contains(view, emph("[  45.5]")) {
		t.Fatalf("zone current not bracketed+emphasized: %q", view)
	}
}

func TestProcessesViewSortModes(t *testing.T) {
	snapshot := model.Snapshot{TopProcesses: []model.ProcessSample{
		{PID: 1, Name: "cpu-hog", CPUPercent: 80, RSSBytes: 100, Jiffies: 10},
		{PID: 2, Name: "mem-hog", CPUPercent: 1, RSSBytes: 900, Jiffies: 5000},
	}}
	cpuOrder := viewTop(snapshot, nil, 'C', false)
	if strings.Index(cpuOrder, "cpu-hog") > strings.Index(cpuOrder, "mem-hog") {
		t.Fatalf("cpu sort wrong: %s", cpuOrder)
	}
	memOrder := viewTop(snapshot, nil, 'M', false)
	if strings.Index(memOrder, "mem-hog") > strings.Index(memOrder, "cpu-hog") {
		t.Fatalf("memory sort wrong: %s", memOrder)
	}
	lifeOrder := viewTop(snapshot, nil, 'L', false)
	if strings.Index(lifeOrder, "mem-hog") > strings.Index(lifeOrder, "cpu-hog") {
		t.Fatalf("lifetime sort wrong: %s", lifeOrder)
	}
}

func TestProcessesViewIndicatorsAndStates(t *testing.T) {
	snapshot := model.Snapshot{TopProcesses: []model.ProcessSample{{PID: 3, Name: "sleeper", State: "S"}}}
	view := stripANSI(viewTop(snapshot, nil, 0, false))
	for _, expected := range []string{"[C] cpu", "Sleeping (S)"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func TestProcessesViewCmdlineToggle(t *testing.T) {
	snapshot := model.Snapshot{TopProcesses: []model.ProcessSample{{PID: 4, Name: "kvm", Cmdline: "/usr/bin/kvm -name guest=vm1"}}}
	plain := stripANSI(viewTop(snapshot, nil, 0, false))
	if contains(plain, "/usr/bin/kvm -name guest=vm1") {
		t.Fatalf("cmdline shown while toggle off: %s", plain)
	}
	full := stripANSI(viewTop(snapshot, nil, 0, true))
	if !contains(full, "/usr/bin/kvm -name guest=vm1") {
		t.Fatalf("cmdline missing while toggle on: %s", full)
	}
}

func TestProcessStateLabelUnknown(t *testing.T) {
	if got := stripANSI(processStateLabel("X")); got != "X" {
		t.Fatalf("unknown state label = %q", got)
	}
	if got := processStateLabel(""); got != "Unknown" {
		t.Fatalf("empty state label = %q", got)
	}
}

func TestIsPCIUnknownClassification(t *testing.T) {
	cases := []struct {
		device model.PCIDevice
		want   bool
	}{
		{model.PCIDevice{Driver: "nvme", Class: "0x010802", VendorID: "0x144d"}, false},
		{model.PCIDevice{Class: "0xff0000", VendorID: "0x10de"}, true},
		{model.PCIDevice{Class: "0x010802", VendorID: "0xffff"}, true},
		{model.PCIDevice{Class: "0x010802", VendorID: ""}, true},
		{model.PCIDevice{Class: "", VendorID: "0x8086"}, true},
		{model.PCIDevice{Class: "0xffffff", VendorID: "0x8086"}, true},
	}
	for _, tc := range cases {
		if got := isPCIUnknown(tc.device); got != tc.want {
			t.Fatalf("isPCIUnknown(%+v) = %t, want %t", tc.device, got, tc.want)
		}
	}
}

func TestHardwareViewShowsUnknownPCIAndUSB(t *testing.T) {
	snapshot := model.Snapshot{
		PCI: []model.PCIDevice{
			{Address: "0000:07:00.0", VendorID: "0x10de", DeviceID: "0x2684", Class: "0xff0000"},
			{Address: "0000:01:00.0", VendorID: "0x144d", DeviceID: "0xa808", Class: "0x010802", Driver: "nvme"},
		},
		USB:             []model.USBDevice{{BusID: "1-2", VendorID: "8087", ProductID: "0026", Product: "AX201 Bluetooth", Manufacturer: "Intel Corp.", SpeedMbps: 12}},
		USBMonAvailable: true,
	}
	view := stripANSI(viewHardware(snapshot))
	for _, expected := range []string{
		"Unknown / unclaimed PCI devices",
		"0000:07:00.0",
		"no driver bound",
		"USB devices",
		"1-2      8087:0026  AX201 Bluetooth (Intel Corp.)  12 Mb/s",
		"usbmon available: yes",
	} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
	if !contains(view, "0000:01:00.0") {
		t.Fatalf("driven device missing from main PCIe table: %s", view)
	}
	unknownSection := view[strings.Index(view, "Unknown / unclaimed PCI devices"):strings.Index(view, "USB devices")]
	if contains(unknownSection, "0000:01:00.0") {
		t.Fatalf("driven device leaked into unknown section: %s", unknownSection)
	}
}

func TestStorageViewShowsDMDevices(t *testing.T) {
	snapshot := model.Snapshot{Disks: []model.Disk{{Name: "dm-0", DMName: "vg0-root", Slaves: []string{"sda2", "sdb2"}}}}
	view := stripANSI(viewStorage(snapshot))
	if !contains(view, "dm vg0-root <- sda2+sdb2") {
		t.Fatalf("dm annotation missing: %s", view)
	}
}

func TestRenderPowerAdvisorSuggestions(t *testing.T) {
	policies := []model.CPUPolicy{{
		Policy:             "policy0",
		CPUs:               "0-7",
		Governor:           "powersave",
		AvailableGovernors: []string{"performance", "powersave"},
		EPP:                "power",
		AvailableEPP:       []string{"balance_performance", "performance", "power"},
	}}
	view := stripANSI(renderPowerAdvisor(policies))
	for _, expected := range []string{
		"CPU power advisor",
		"governor powersave",
		"echo performance | sudo tee /sys/devices/system/cpu/cpufreq/policy0/scaling_governor",
		"echo balance_performance | sudo tee /sys/devices/system/cpu/cpufreq/policy0/energy_performance_preference",
	} {
		if !contains(view, expected) {
			t.Fatalf("advisor missing %q: %s", expected, view)
		}
	}
	empty := stripANSI(renderPowerAdvisor(nil))
	if !contains(empty, "No cpufreq policies") {
		t.Fatalf("empty advisor state missing: %s", empty)
	}
}

func TestPowerModeToggleKey(t *testing.T) {
	m := modelState{tab: 2}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	state := updated.(modelState)
	if !state.powerMode {
		t.Fatal("p did not enable power mode")
	}
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if updated.(modelState).powerMode {
		t.Fatal("second p did not disable power mode")
	}
	// p is inert on other tabs.
	other := modelState{tab: 0}
	updated, _ = other.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if updated.(modelState).powerMode {
		t.Fatal("p toggled power mode outside CPU/Memory tab")
	}
}

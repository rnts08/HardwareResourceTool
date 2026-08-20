package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/model"
)

const historyLimit = 60

type tickMsg time.Time

type modelState struct {
	collector   *collect.Collector
	interval    time.Duration
	thresholds  analyze.Thresholds
	snapshot    model.Snapshot
	history     []model.Snapshot
	findings    []model.Finding
	err         error
	tab         int
	width       int
	height      int
	offset      int
	xoffset     int
	collecting  bool
	paused      bool
	showHelp    bool
	collectTime time.Duration
	pickerMode  bool
	detailMode  bool
	pickerItems []pickerItem
	pickerSel   int
	detailTitle string
	detailLines []string
}

type pickerItem struct {
	kind  string
	index int
	label string
}

var tabs = []string{"Overview", "Storage", "Network", "Findings", "Hardware", "Thermal", "Top"}

var (
	criticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// Control markers carried at the start of a plain-text line so the renderer
// can colorize it after horizontal scrolling has been applied.
const (
	markCritical = "\x01c\x02"
	markWarning  = "\x01w\x02"
	markInfo     = "\x01i\x02"
)

func Run(collector *collect.Collector, interval time.Duration, thresholds analyze.Thresholds) error {
	if interval < 500*time.Millisecond {
		interval = 500 * time.Millisecond
	}
	m := modelState{collector: collector, interval: interval, thresholds: thresholds, collecting: true}
	_, err := tea.NewProgram(m, tea.WithMouseCellMotion()).Run()
	return err
}

func (m modelState) Init() tea.Cmd { return tea.Batch(collectNow(m.collector), tick(m.interval)) }

func (m modelState) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.detailMode {
				m.detailMode = false
			} else if m.pickerMode {
				m.pickerMode = false
			}
			return m, nil
		case "enter":
			if m.pickerMode {
				m.openDetail()
			}
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7":
			if m.pickerMode || m.detailMode {
				return m, nil
			}
			m.tab = int(msg.String()[0] - '1')
			m.offset, m.xoffset = 0, 0
		case "d":
			if m.tab == 4 && !m.pickerMode && !m.detailMode {
				m.pickerMode = true
				m.pickerItems = buildPicker(m.snapshot)
				m.pickerSel = 0
				m.offset, m.xoffset = 0, 0
			}
		case "j", "down":
			if m.pickerMode {
				if m.pickerSel+1 < len(m.pickerItems) {
					m.pickerSel++
				}
				return m, nil
			}
			m.offset += pageStep(m.height, msg.String())
		case "k", "up":
			if m.pickerMode {
				if m.pickerSel > 0 {
					m.pickerSel--
				}
				return m, nil
			}
			m.offset -= pageStep(m.height, msg.String())
			if m.offset < 0 {
				m.offset = 0
			}
		case "pgdown", "ctrl+d", "ctrl+f":
			m.offset += pageStep(m.height, msg.String())
		case "pgup", "ctrl+u", "ctrl+b":
			m.offset -= pageStep(m.height, msg.String())
			if m.offset < 0 {
				m.offset = 0
			}
		case "tab", "right", "l":
			if m.pickerMode || m.detailMode {
				return m, nil
			}
			m.tab = (m.tab + 1) % len(tabs)
			m.offset, m.xoffset = 0, 0
		case "shift+tab", "left", "h":
			if m.pickerMode || m.detailMode {
				return m, nil
			}
			m.tab = (m.tab + len(tabs) - 1) % len(tabs)
			m.offset, m.xoffset = 0, 0
		case "?":
			m.showHelp = !m.showHelp
		case " ":
			m.paused = !m.paused
			m.collecting = false
			if !m.paused {
				return m, tea.Batch(collectNow(m.collector), tick(m.interval))
			}
		case "shift+right", ">":
			m.xoffset += 8
		case "shift+left", "<":
			m.xoffset -= 8
			if m.xoffset < 0 {
				m.xoffset = 0
			}
		case "r":
			if !m.paused && !m.collecting {
				m.collecting = true
				return m, collectNow(m.collector)
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.offset -= 3
			if m.offset < 0 {
				m.offset = 0
			}
		case tea.MouseWheelDown:
			m.offset += 3
		case tea.MouseLeft:
			if !m.showHelp && !m.pickerMode && !m.detailMode && msg.Y == 2 {
				if tab := m.tabAtX(msg.X); tab >= 0 {
					m.tab = tab
					m.offset, m.xoffset = 0, 0
				}
			}
		}
	case model.Snapshot:
		m.collecting = false
		m.snapshot, m.findings, m.err = msg, analyze.FindingsWithThresholds(msg, m.thresholds), nil
		m.collectTime = time.Duration(msg.CollectDurationMS) * time.Millisecond
		if m.pickerMode {
			m.pickerItems = buildPicker(msg)
			if m.pickerSel >= len(m.pickerItems) {
				m.pickerSel = 0
			}
		}
		m.history = append(m.history, msg)
		if len(m.history) > historyLimit {
			m.history = m.history[len(m.history)-historyLimit:]
		}
		// The timer chain is owned by tickMsg. Scheduling another timer here
		// would accumulate redraws when collection completes after a tick.
		return m, nil
	case tickMsg:
		// Paused and in-flight collections never start a second snapshot, so
		// slow collection cannot race or backlog. The tick chain continues so
		// the dashboard resumes automatically.
		if m.paused {
			return m, nil
		}
		if m.collecting {
			return m, tick(m.interval)
		}
		m.collecting = true
		return m, tea.Batch(collectNow(m.collector), tick(m.interval))
	}
	return m, nil
}

func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func pageStep(height int, key string) int {
	switch key {
	case "pgdown", "pgup":
		if height > 3 {
			return height - 3
		}
		return 12
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b":
		return 6
	default:
		return 1
	}
}

func (m modelState) View() string {
	if m.showHelp {
		return renderScrolled("", renderHelp(m.thresholds), "", m.width, m.height, 0, 0)
	}
	var header strings.Builder
	header.WriteString("Hardware Resources — live host view\n")
	header.WriteString("──────────────────────────────────────────────────────────────────────────────\n")
	for i, tab := range tabs {
		if i == m.tab {
			fmt.Fprintf(&header, "[%d %s] ", i+1, tab)
		} else {
			fmt.Fprintf(&header, " %d %s  ", i+1, tab)
		}
	}
	status := fmt.Sprintf("Samples: %d/%d  Updated: %s  interval %s", len(m.history), historyLimit, updatedAt(m.snapshot), m.interval)
	if m.collecting {
		status += "  collecting…"
	}
	if m.paused {
		status += "  [paused]"
	}
	if m.collectTime > 0 {
		status += fmt.Sprintf("  last collect %s", m.collectTime.Round(time.Millisecond))
	}
	header.WriteString("\n" + status + "\n\n")

	content := m.tabContent()
	if m.pickerMode {
		content = m.pickerContent()
	}
	if m.detailMode {
		content = m.detailTitle + "\n" + strings.Join(m.detailLines, "\n") + "\n"
	}

	var footer strings.Builder
	if m.err != nil {
		fmt.Fprintf(&footer, "Error: %v\n", m.err)
	}
	if len(m.snapshot.Errors) > 0 {
		errors := strings.Join(m.snapshot.Errors, "; ")
		if runes := []rune(errors); len(runes) > 400 {
			errors = string(runes[:397]) + "..."
		}
		footer.WriteString("Collector errors: " + errors + "\n")
	}
	if m.detailMode {
		footer.WriteString("esc: back to picker  j/k scroll")
	} else if m.pickerMode {
		footer.WriteString("j/k select  enter open  esc close  d on Hardware opens this picker")
	} else {
		footer.WriteString("1-7: tabs  h/l prev/next  j/k scroll  </> horizontal  d detail  space pause  ? help  q quit")
	}

	return renderScrolled(header.String(), content, footer.String(), m.width, m.height, m.offset, m.xoffset)
}

func (m modelState) openDetail() {
	if m.pickerSel < 0 || m.pickerSel >= len(m.pickerItems) {
		return
	}
	item := m.pickerItems[m.pickerSel]
	title, lines := detailFor(m.snapshot, item)
	if len(lines) == 0 {
		return
	}
	m.detailTitle, m.detailLines = title, lines
	m.detailMode = true
	m.offset, m.xoffset = 0, 0
}

func (m modelState) pickerContent() string {
	var b strings.Builder
	b.WriteString("Select an item to expand (esc to close)\n")
	if len(m.pickerItems) == 0 {
		b.WriteString("\n  No devices or VMs reported in this capture.\n")
		return b.String()
	}
	for i, item := range m.pickerItems {
		cursor := " "
		if i == m.pickerSel {
			cursor = ">"
		}
		fmt.Fprintf(&b, "  %s %s\n", cursor, item.label)
	}
	return b.String()
}

func buildPicker(snapshot model.Snapshot) []pickerItem {
	items := []pickerItem{}
	for i, vm := range snapshot.Virtualization.VirtualMachines {
		vmid := ""
		if vm.VMID != "" {
			vmid = " vmid " + vm.VMID
		}
		items = append(items, pickerItem{kind: "vm", index: i, label: fmt.Sprintf("VM    %-20s%s pid %d running=%t source=%s", vm.Name, vmid, vm.PID, vm.Running, vm.Source)})
	}
	for i, gpu := range snapshot.GPUs {
		items = append(items, pickerItem{kind: "gpu", index: i, label: fmt.Sprintf("GPU   %-12s %-24s nvml=%s", gpu.Address, gpu.Name, gpu.NVMLStatus)})
	}
	for i, device := range snapshot.PCI {
		items = append(items, pickerItem{kind: "pci", index: i, label: fmt.Sprintf("PCI   %-12s %-14s %s:%s", device.Address, device.Class, device.VendorID, device.DeviceID)})
	}
	for i, memory := range snapshot.MemoryDevices {
		items = append(items, pickerItem{kind: "memory", index: i, label: fmt.Sprintf("DIMM  %-16s %6.1f GiB %s", memory.Locator, float64(memory.SizeBytes)/(1024*1024*1024), memory.Type)})
	}
	return items
}

func (m modelState) tabAtX(x int) int {
	column := 0
	for i, tab := range tabs {
		var token string
		if i == m.tab {
			token = fmt.Sprintf("[%d %s] ", i+1, tab)
		} else {
			token = fmt.Sprintf(" %d %s  ", i+1, tab)
		}
		if x >= column && x < column+len(token) {
			return i
		}
		column += len(token)
	}
	return -1
}

func detailFor(snapshot model.Snapshot, item pickerItem) (string, []string) {
	switch item.kind {
	case "vm":
		return vmDetail(snapshot.Virtualization.VirtualMachines[item.index])
	case "gpu":
		return gpuDetail(snapshot.GPUs[item.index])
	case "pci":
		return pciDetail(snapshot.PCI[item.index])
	case "memory":
		device := snapshot.MemoryDevices[item.index]
		lines := []string{
			fmt.Sprintf("  locator          %s", device.Locator),
			fmt.Sprintf("  manufacturer     %s", device.Manufacturer),
			fmt.Sprintf("  part number      %s", device.PartNumber),
			fmt.Sprintf("  serial           %s", device.Serial),
			fmt.Sprintf("  type             %s", device.Type),
			fmt.Sprintf("  size             %s", formatBytes(device.SizeBytes)),
			fmt.Sprintf("  speed            %d MT/s", device.SpeedMTs),
			fmt.Sprintf("  configured speed %d MT/s", device.ConfiguredSpeedMTs),
			fmt.Sprintf("  corrected errors %d", device.CorrectedErrors),
			fmt.Sprintf("  uncorrected errs %d", device.UncorrectedErrors),
		}
		return "DIMM " + device.Locator, lines
	}
	return "", nil
}

func vmDetail(vm model.VirtualMachine) (string, []string) {
	runtime := make([]string, 0, len(vm.RuntimeNUMABytes))
	for node, bytes := range vm.RuntimeNUMABytes {
		runtime = append(runtime, fmt.Sprintf("node%d:%s", node, formatBytes(bytes)))
	}
	numaResidency := "unavailable"
	if vm.RuntimeAvailable {
		numaResidency = "none"
		if len(runtime) > 0 {
			numaResidency = strings.Join(runtime, " ")
		}
	}
	lines := []string{
		fmt.Sprintf("  name             %s", vm.Name),
		fmt.Sprintf("  vmid             %s", vm.VMID),
		fmt.Sprintf("  source           %s", vm.Source),
		fmt.Sprintf("  pid              %d", vm.PID),
		fmt.Sprintf("  running          %t", vm.Running),
		fmt.Sprintf("  configured vCPUs %d", vm.ConfiguredVCPUs),
		fmt.Sprintf("  QMP enabled vCPU %d", vm.QMPEnabledVCPUs),
		fmt.Sprintf("  configured memory %s", formatBytes(vm.ConfiguredMemoryBytes)),
		fmt.Sprintf("  process CPU      %.1f%%  cgroup CPU %.1f%%", vm.CPUPercent, vm.CgroupCPUPercent),
		fmt.Sprintf("  process RSS      %s", formatBytes(vm.ProcessRSSBytes)),
		fmt.Sprintf("  cgroup current   %s  max %s", formatBytes(vm.MemoryCurrentBytes), formatBytes(vm.MemoryMaxBytes)),
		fmt.Sprintf("  cgroup path      %s", vm.CgroupPath),
		fmt.Sprintf("  read/write       %s / %s", formatBytes(vm.ReadBytes), formatBytes(vm.WriteBytes)),
		fmt.Sprintf("  hugepages        %t  page %s", vm.Hugepages, formatBytes(vm.HugepageBytes)),
		fmt.Sprintf("  runtime huge     anon %s hugetlb %s", formatBytes(vm.RuntimeAnonHugeBytes), formatBytes(vm.RuntimeHugetlbBytes)),
		fmt.Sprintf("  NUMA residency   %s", numaResidency),
		fmt.Sprintf("  NUMA nodes       %v", vm.NUMANodes),
	}
	qmpStatus := "unavailable"
	if vm.QMPAvailable {
		qmpStatus = vm.QMPStatus
	}
	if vm.QMPError != "" {
		qmpStatus = vm.QMPError
	}
	lines = append(lines,
		fmt.Sprintf("  QMP              %s  version %s", qmpStatus, vm.QMPVersion),
		fmt.Sprintf("  QMP base/plugged %s / %s", formatBytes(vm.QMPBaseMemoryBytes), formatBytes(vm.QMPPluggedMemoryBytes)),
		fmt.Sprintf("  balloon          enabled=%t reported=%t guest=%t", vm.BalloonEnabled, vm.BalloonReported, vm.BalloonGuestReport),
		fmt.Sprintf("  balloon actual   %s  target %s  reclaimed %s", formatBytes(vm.BalloonActualBytes), formatBytes(vm.BalloonTargetBytes), formatBytes(vm.BalloonReclaimedBytes)),
		fmt.Sprintf("  balloon commit   %s  available %s", formatBytes(vm.BalloonCommittedBytes), formatBytes(vm.BalloonAvailableBytes)),
	)
	lines = append(lines, "  disks")
	for _, disk := range vm.Disks {
		lines = append(lines, fmt.Sprintf("    %-4s %-6s %s", disk.Bus, disk.Target, disk.Source))
	}
	if len(vm.Disks) == 0 {
		lines = append(lines, "    none reported")
	}
	lines = append(lines, "  NICs")
	for _, nic := range vm.NICs {
		host := nic.HostNetwork
		if host == "" {
			host = "unresolved"
		}
		lines = append(lines, fmt.Sprintf("    %-8s source %-12s host %-10s rx %s/s tx %s/s mac %s", nic.Type, nic.Source, host, formatRate(nic.RXBytesPerSecond), formatRate(nic.TXBytesPerSecond), nic.MAC))
	}
	if len(vm.NICs) == 0 {
		lines = append(lines, "    none reported")
	}
	lines = append(lines, "  PCI attachments", fmt.Sprintf("    %v", vm.PCIAddresses))
	return "VM " + vm.Name, lines
}

func gpuDetail(gpu model.GPU) (string, []string) {
	lines := []string{
		fmt.Sprintf("  address          %s", gpu.Address),
		fmt.Sprintf("  ids              %s:%s", gpu.VendorID, gpu.DeviceID),
		fmt.Sprintf("  name             %s", gpu.Name),
		fmt.Sprintf("  NVML             %t  status %s", gpu.NVML, gpu.NVMLStatus),
		fmt.Sprintf("  uuid             %s", gpu.UUID),
		fmt.Sprintf("  memory           used %s / total %s", formatBytes(gpu.MemoryUsedBytes), formatBytes(gpu.MemoryBytes)),
		fmt.Sprintf("  process memory   %s (%s)", formatBytes(gpu.MemoryProcessBytes), gpu.MemorySource),
		fmt.Sprintf("  utilization      %.1f%%", gpu.UtilizationPercent),
		fmt.Sprintf("  temperature      %.1f C", gpu.TemperatureCelsius),
		fmt.Sprintf("  power            %.1f W", gpu.PowerWatts),
		fmt.Sprintf("  ECC              enabled=%t corrected=%d uncorrected=%d", gpu.ECCEnabled, gpu.ECCCorrected, gpu.ECCUncorrected),
		fmt.Sprintf("  MIG              enabled=%t max instances=%d", gpu.MIGEnabled, gpu.MIGMaxInstances),
	}
	if gpu.PassedThrough {
		lines = append(lines, fmt.Sprintf("  passthrough      %s", gpu.PassedThroughVM))
	}
	return "GPU " + gpu.Name + " " + gpu.Address, lines
}

func pciDetail(device model.PCIDevice) (string, []string) {
	lines := []string{
		fmt.Sprintf("  address          %s", device.Address),
		fmt.Sprintf("  ids              %s:%s", device.VendorID, device.DeviceID),
		fmt.Sprintf("  class            %s", device.Class),
		fmt.Sprintf("  driver           %s", device.Driver),
		fmt.Sprintf("  NUMA node        %d", device.NUMANode),
		fmt.Sprintf("  IOMMU group      %s", device.IOMMUGroup),
		fmt.Sprintf("  link             negotiated %s x%d  max %s x%d", device.CurrentLinkSpeed, device.CurrentLinkWidth, device.MaxLinkSpeed, device.MaxLinkWidth),
		fmt.Sprintf("  PCIe capability  %s x%d  max payload %d  max read req %d", device.PCIeCapabilityMaxSpeed, device.PCIeCapabilityMaxWidth, device.PCIeMaxPayloadBytes, device.PCIeMaxReadRequestBytes),
		fmt.Sprintf("  negotiated       %s x%d", device.PCIeNegotiatedSpeed, device.PCIeNegotiatedWidth),
		fmt.Sprintf("  path bandwidth   %s  bottleneck %s", formatGbps(device.PCIePathBandwidthGbps), device.PCIePathBottleneck),
		fmt.Sprintf("  parent           %s  PF %s", device.PCIeParentAddress, device.PCIePFAddress),
		fmt.Sprintf("  VFs              %v", device.PCIeVFAddresses),
		fmt.Sprintf("  BAR total        %s (%d) above4g=%t", formatBytes(device.BARTotalBytes), device.BARCount, device.BARAbove4G),
		fmt.Sprintf("  BAR layout       %s", pciBARSummary(device)),
		fmt.Sprintf("  ROM              %t  resource windows %v", device.ROM, device.ResourceWindows),
		fmt.Sprintf("  AER              uncorrectable %d  correctable %d", device.AERUncorrectableStatus, device.AERCorrectableStatus),
		fmt.Sprintf("  SR-IOV total VFs %d  resizable BAR %t", device.SRIOVTotalVFs, device.ResizableBAR),
	}
	if len(device.Capabilities) > 0 {
		lines = append(lines, fmt.Sprintf("  capabilities     %v", device.Capabilities))
	}
	if len(device.PCIePath) > 0 {
		lines = append(lines, fmt.Sprintf("  path             %v", device.PCIePath))
	}
	return "PCI " + device.Address, lines
}

func pciBARSummary(device model.PCIDevice) string {
	if len(device.BARs) == 0 {
		return "none"
	}
	memory, memory64, io, prefetch := 0, 0, 0, 0
	for _, bar := range device.BARs {
		switch bar.Type {
		case "io":
			io++
		case "64-bit memory":
			memory64++
		case "memory":
			memory++
		}
		if bar.Prefetchable {
			prefetch++
		}
	}
	return fmt.Sprintf("mem %d  64-bit %d  io %d  prefetch %d", memory, memory64, io, prefetch)
}

func pciBARSummarySuffix(device model.PCIDevice) string {
	suffix := ""
	if device.ROM {
		suffix += " rom"
	}
	if len(device.ResourceWindows) > 0 {
		suffix += fmt.Sprintf(" win%d", len(device.ResourceWindows))
	}
	return suffix
}

func (m modelState) tabContent() string {
	switch m.tab {
	case 1:
		return viewStorage(m.snapshot)
	case 2:
		return viewNetwork(m.snapshot)
	case 3:
		return viewFindings(m.findings)
	case 4:
		return viewHardware(m.snapshot)
	case 5:
		return viewThermal(m.snapshot)
	case 6:
		return viewTop(m.snapshot)
	default:
		return viewOverview(m.snapshot, m.history, m.thresholds)
	}
}

// renderScrolled clips content to the terminal. The header and footer stay
// fixed; only the content region scrolls vertically by offset and every line
// scrolls horizontally by xoffset. Colored lines are colorized only after
// horizontal scrolling so ANSI sequences never interfere with slicing.
func renderScrolled(header, content, footer string, width, height, offset, xoffset int) string {
	headerLines := splitLines(header)
	contentLines := splitLines(content)
	footerLines := splitLines(footer)

	body := contentLines
	if height > 0 {
		avail := height - len(headerLines) - len(footerLines)
		if avail < 1 {
			avail = 1
		}
		if len(contentLines) > avail {
			maxOffset := len(contentLines) - avail
			if offset > maxOffset {
				offset = maxOffset
			}
			if offset < 0 {
				offset = 0
			}
			end := offset + avail
			if end > len(contentLines) {
				end = len(contentLines)
			}
			body = contentLines[offset:end]
		} else {
			body = contentLines
		}
	}

	all := append(append(append([]string{}, headerLines...), body...), footerLines...)

	maxLine := 0
	for _, line := range all {
		if runes := len([]rune(line)); runes > maxLine {
			maxLine = runes
		}
	}
	maxX := 0
	if width > 0 {
		maxX = maxLine - width
		if maxX < 0 {
			maxX = 0
		}
	}
	if xoffset > maxX {
		xoffset = maxX
	}
	if xoffset < 0 {
		xoffset = 0
	}

	out := make([]string, 0, len(all))
	for _, line := range all {
		scrolled, marker := scrollLine(line, width, xoffset)
		out = append(out, applyColor(marker, scrolled))
	}

	if height > 0 && len(out) > height {
		out = out[:height]
	}
	return strings.Join(out, "\n")
}

// scrollLine slices a plain-text line horizontally and separates any leading
// color marker so it survives the slice.
func scrollLine(line string, width, xoffset int) (string, string) {
	marker := ""
	for _, candidate := range []string{markCritical, markWarning, markInfo} {
		if strings.HasPrefix(line, candidate) {
			marker = candidate
			line = strings.TrimPrefix(line, candidate)
			break
		}
	}
	runes := []rune(line)
	if xoffset > 0 {
		if xoffset >= len(runes) {
			runes = []rune{}
		} else {
			runes = runes[xoffset:]
		}
	}
	if width > 0 && len(runes) > width {
		if width <= 1 {
			return "…", marker
		}
		return string(runes[:width-1]) + "…", marker
	}
	return string(runes), marker
}

func applyColor(marker, line string) string {
	switch marker {
	case markCritical:
		return criticalStyle.Render(line)
	case markWarning:
		return warningStyle.Render(line)
	case markInfo:
		return infoStyle.Render(line)
	}
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "[critical]"):
		return criticalStyle.Render(line)
	case strings.HasPrefix(trimmed, "[warning]"):
		return warningStyle.Render(line)
	case strings.HasPrefix(trimmed, "[info]"):
		return infoStyle.Render(line)
	}
	return line
}

func renderHelp(thresholds analyze.Thresholds) string {
	return strings.Join([]string{
		"Help",
		"",
		"  1-7 or h/l/tab     switch tabs (Overview, Storage, Network, Findings, Hardware, Thermal, Top)",
		"  j/k, PgUp/PgDn     scroll the active tab vertically",
		"  </>, shift+arrows  scroll the active tab horizontally",
		"  d                  on Hardware: pick a VM/GPU/PCI/DIMM to expand",
		"  enter              open the selected item's detail pane",
		"  esc                close the detail pane or the picker",
		"  space              pause/resume live collection",
		"  r                  force a refresh now",
		"  ?                  toggle this help",
		"  q / Ctrl+C         quit",
		"",
		fmt.Sprintf("  thresholds: cpu-idle critical <%.1f  iowait warning >%.1f", thresholds.CPUIdleCritical, thresholds.IOWaitWarning),
		fmt.Sprintf("  memory-used critical >%.1f  filesystem warning >%.1f  filesystem critical >%.1f", thresholds.MemoryUsedCritical, thresholds.FilesystemUsedWarning, thresholds.FilesystemUsedCritical),
		"",
		"  Rates (throughput, percentages, swap activity) need two samples;",
		"  the first snapshot establishes the counter baseline.",
	}, "\n")
}

func viewOverview(snapshot model.Snapshot, history []model.Snapshot, thresholds analyze.Thresholds) string {
	cpuIdle := historyValues(history, func(s model.Snapshot) float64 { return s.CPU.IdlePercent })
	memoryUsed := historyValues(history, func(s model.Snapshot) float64 { return s.Memory.UsedPercent })
	var b strings.Builder
	cpuLine := fmt.Sprintf("CPU     %d logical  user %5.1f%%  system %5.1f%%  iowait %5.1f%%  idle %5.1f%%\n", snapshot.CPU.LogicalCPUs, snapshot.CPU.UserPercent, snapshot.CPU.SystemPercent, snapshot.CPU.IOWaitPercent, snapshot.CPU.IdlePercent)
	if snapshot.CPU.LogicalCPUs > 0 && snapshot.CPU.IdlePercent < thresholds.CPUIdleCritical {
		cpuLine = markCritical + cpuLine
	}
	b.WriteString(cpuLine)
	fmt.Fprintf(&b, "        load %.2f %.2f %.2f  ctxt %d/s  interrupts %d/s\n", snapshot.CPU.Load1, snapshot.CPU.Load5, snapshot.CPU.Load15, snapshot.CPU.ContextSwitch, snapshot.CPU.Interrupts)
	fmt.Fprintf(&b, "        idle   %s\n", sparkline(cpuIdle, 0, 100))
	memoryLine := fmt.Sprintf("Memory  used %5.1f%%  available %6.1f GiB  swap in/out %d/%d per sec\n", snapshot.Memory.UsedPercent, float64(snapshot.Memory.AvailableBytes)/(1024*1024*1024), snapshot.Memory.SwapInPerSec, snapshot.Memory.SwapOutPerSec)
	if snapshot.Memory.TotalBytes > 0 && snapshot.Memory.UsedPercent > thresholds.MemoryUsedCritical {
		memoryLine = markCritical + memoryLine
	}
	b.WriteString(memoryLine)
	fmt.Fprintf(&b, "        used   %s\n", sparkline(memoryUsed, 0, 100))
	fmt.Fprintf(&b, "System  governor %q  THP %q  swappiness %d  NUMA nodes %d remote %d/s\n", snapshot.System.CPUGovernor, snapshot.System.THP, snapshot.System.Swappiness, snapshot.NUMA.Nodes, snapshot.NUMA.RemoteEvents)
	fmt.Fprintf(&b, "        open files %d  processes %d  stack %s  locked memory %s\n", snapshot.System.OpenFiles, snapshot.System.MaxProcesses, formatBytes(snapshot.System.MaxStack), formatBytes(snapshot.System.MaxLocked))
	fmt.Fprintf(&b, "        host/init files %d  host/init processes %d\n", snapshot.System.HostLimits.OpenFiles, snapshot.System.HostLimits.MaxProcesses)
	if events := snapshot.System.KernelEvents; len(events.Recent) > 0 || events.OOM+events.IOErrors+events.PCIeErrors+events.Hardware+events.NVIDIA+events.StorageResets+events.LinkFailures > 0 {
		label, deltas := "cumulative", events
		if len(history) >= 2 {
			deltas = kernelEventDeltas(history[len(history)-2].System.KernelEvents, events)
			label = "since last sample"
		}
		fmt.Fprintf(&b, "        kernel events (%s) OOM %d  I/O %d  PCIe %d  HW %d  NVIDIA %d  storage resets %d  link failures %d\n", label, deltas.OOM, deltas.IOErrors, deltas.PCIeErrors, deltas.Hardware, deltas.NVIDIA, deltas.StorageResets, deltas.LinkFailures)
	}
	if snapshot.Virtualization.QEMUDetected || snapshot.Virtualization.KVMAvailable || len(snapshot.Virtualization.VirtualMachines) > 0 {
		fmt.Fprintf(&b, "Virtualization  %s  VMs %d  allocated vCPU %d (%.2fx)  memory %s (%.2fx)\n", snapshot.Virtualization.Hypervisor, len(snapshot.Virtualization.VirtualMachines), snapshot.Virtualization.AllocatedVCPUs, snapshot.Virtualization.VCPUOvercommitRatio, formatBytes(snapshot.Virtualization.AllocatedMemoryBytes), snapshot.Virtualization.MemoryOvercommitRatio)
	}
	if len(snapshot.System.Sysctls) > 0 {
		fmt.Fprintf(&b, "        sysctls %v\n", snapshot.System.Sysctls)
	}
	if len(history) < 2 && snapshot.CPU.UserPercent == 0 && snapshot.CPU.SystemPercent == 0 && snapshot.CPU.IdlePercent == 0 {
		b.WriteString("  Rates appear after the second sample; keep the dashboard running or press r to refresh.\n")
	}
	return b.String()
}

func kernelEventDeltas(previous, current model.KernelEvents) model.KernelEvents {
	delta := model.KernelEvents{}
	if current.OOM > previous.OOM {
		delta.OOM = current.OOM - previous.OOM
	}
	if current.IOErrors > previous.IOErrors {
		delta.IOErrors = current.IOErrors - previous.IOErrors
	}
	if current.PCIeErrors > previous.PCIeErrors {
		delta.PCIeErrors = current.PCIeErrors - previous.PCIeErrors
	}
	if current.Hardware > previous.Hardware {
		delta.Hardware = current.Hardware - previous.Hardware
	}
	if current.NVIDIA > previous.NVIDIA {
		delta.NVIDIA = current.NVIDIA - previous.NVIDIA
	}
	if current.StorageResets > previous.StorageResets {
		delta.StorageResets = current.StorageResets - previous.StorageResets
	}
	if current.LinkFailures > previous.LinkFailures {
		delta.LinkFailures = current.LinkFailures - previous.LinkFailures
	}
	return delta
}

func viewStorage(snapshot model.Snapshot) string {
	var b strings.Builder
	b.WriteString("Storage\n")
	if len(snapshot.Disks) == 0 {
		b.WriteString("  No block devices reported.\n")
	}
	for _, disk := range snapshot.Disks {
		fmt.Fprintf(&b, "  %-16s read %10s/s  write %10s/s  ops %.1f/%.1f  in-flight %d\n", disk.Name, formatRate(disk.ReadBytesPerSec), formatRate(disk.WriteBytesPerSec), disk.ReadsPerSec, disk.WritesPerSec, disk.InFlight)
	}
	b.WriteString("\nFilesystems\n")
	for _, filesystem := range snapshot.Filesystems {
		mode := "rw"
		if filesystem.ReadOnly {
			mode = "ro"
		}
		fmt.Fprintf(&b, "  %-24s %-3s %5.1f%% used  available %10s  %s\n", filesystem.MountPoint, mode, filesystem.UsedPercent, formatBytes(filesystem.AvailableBytes), filesystem.Type)
	}
	if snapshot.VirtualNetworkCount > 0 {
		fmt.Fprintf(&b, "\n  Virtual/device-less interfaces filtered: %d\n", snapshot.VirtualNetworkCount)
	}
	return b.String()
}

func viewNetwork(snapshot model.Snapshot) string {
	var b strings.Builder
	b.WriteString("Network\n")
	if len(snapshot.Networks) == 0 {
		b.WriteString("  No network interfaces reported.\n")
	}
	for _, network := range snapshot.Networks {
		fmt.Fprintf(&b, "  %-16s %-8s pci %-16s rx %10s/s  tx %10s/s  speed %dMb/s  mtu %d  queues %d/%d  rings %d/%d  errors %d/%d  drops %d/%d\n", network.Name, network.State, network.PCIAddress, formatRate(network.RXBytesPerSec), formatRate(network.TXBytesPerSec), network.LinkSpeedMbps, network.MTU, network.RXQueues, network.TXQueues, network.RXRingSize, network.TXRingSize, network.RXErrors, network.TXErrors, network.RXDrops, network.TXDrops)
		if network.Driver != "" || network.LinkDuplex != "" || network.FECActive != "" {
			fmt.Fprintf(&b, "    driver %s v%s  fw %s  bus %s  port %s phy %d  xcvr %s  mdix %s  duplex %s  autoneg %s  fec %s  modes %d/%d  peer %d  max channels %d/%d/%d  pause %t/%t  ts %t phc %d  stats %d\n", network.Driver, network.DriverVersion, network.FWVersion, network.BusInfo, network.LinkPort, network.PHYAddress, network.Transceiver, network.TPMDIX, network.LinkDuplex, network.AutoNegotiation, network.FECActive, len(network.SupportedLinkModes), len(network.AdvertisedLinkModes), len(network.PeerLinkModes), network.MaxRXChannels, network.MaxTXChannels, network.MaxCombinedChannels, network.RXPause, network.TXPause, network.Timestamping, network.PHCIndex, len(network.DriverStats))
		}
		if len(network.FeaturesActive) > 0 || network.RSSHashFunc != "" || network.CoalesceRXUsecs > 0 || network.CoalesceTXUsecs > 0 {
			fmt.Fprintf(&b, "    features hw %d active %d wanted %d fixed %d  coalesce rx %dus/%df tx %dus/%df adaptive %t/%t  rss %s indir %d key %d\n", len(network.FeaturesHardware), len(network.FeaturesActive), len(network.FeaturesWanted), len(network.FeaturesNoChange), network.CoalesceRXUsecs, network.CoalesceRXMaxFrames, network.CoalesceTXUsecs, network.CoalesceTXMaxFrames, network.CoalesceAdaptiveRX, network.CoalesceAdaptiveTX, network.RSSHashFunc, network.RSSIndirSize, network.RSSKeySize)
		}
	}
	return b.String()
}

func viewFindings(findings []model.Finding) string {
	var b strings.Builder
	b.WriteString("Findings\n")
	if len(findings) == 0 {
		b.WriteString("  No findings.\n")
		return b.String()
	}
	for _, finding := range findings {
		fmt.Fprintf(&b, "  [%s] %s\n    %s\n    Recommendation: %s\n", finding.Severity, finding.Title, finding.Evidence, finding.Recommendation)
	}
	return b.String()
}

func viewHardware(snapshot model.Snapshot) string {
	var b strings.Builder
	b.WriteString("PCIe devices\n")
	for _, device := range snapshot.PCI {
		fmt.Fprintf(&b, "  %-16s %-8s:%-8s class %-8s NUMA %d  link %s x%d/%s x%d  path %s x%d @%s  BARs %d/%s%s  caps %s  driver %s\n", device.Address, device.VendorID, device.DeviceID, device.Class, device.NUMANode, device.CurrentLinkSpeed, device.CurrentLinkWidth, device.MaxLinkSpeed, device.MaxLinkWidth, device.PCIePathMinSpeed, device.PCIePathMinWidth, device.PCIePathBottleneck, device.BARCount, formatBytes(device.BARTotalBytes), pciBARSummarySuffix(device), strings.Join(device.Capabilities, ","), device.Driver)
	}
	b.WriteString("\nNVIDIA GPUs\n")
	for _, gpu := range snapshot.GPUs {
		if gpu.PassedThrough {
			fmt.Fprintf(&b, "  %-16s %s:%s %s PASSED THROUGH (%s); host NVML unavailable\n", gpu.Address, gpu.VendorID, gpu.DeviceID, gpu.Name, gpu.NVMLStatus)
		} else if gpu.NVML {
			fmt.Fprintf(&b, "  %-16s %s:%s %s NVML memory %.1f/%.1f GiB (%s; process %.1f GiB)  util %.1f%%  temp %.1fC  power %.1fW  ECC %t %d/%d  MIG %t max %d\n", gpu.Address, gpu.VendorID, gpu.DeviceID, gpu.Name, float64(gpu.MemoryUsedBytes)/(1024*1024*1024), float64(gpu.MemoryBytes)/(1024*1024*1024), gpu.MemorySource, float64(gpu.MemoryProcessBytes)/(1024*1024*1024), gpu.UtilizationPercent, gpu.TemperatureCelsius, gpu.PowerWatts, gpu.ECCEnabled, gpu.ECCCorrected, gpu.ECCUncorrected, gpu.MIGEnabled, gpu.MIGMaxInstances)
		} else {
			fmt.Fprintf(&b, "  %-16s %s:%s NVML unavailable (%s)\n", gpu.Address, gpu.VendorID, gpu.DeviceID, gpu.NVMLStatus)
		}
	}
	if snapshot.Virtualization.QEMUDetected || len(snapshot.Virtualization.VirtualMachines) > 0 {
		b.WriteString("\nKVM/QEMU domains\n")
		for _, vm := range snapshot.Virtualization.VirtualMachines {
			fmt.Fprintf(&b, "  %-20s run=%t vCPU %d/%d CPU %5.1f/%5.1f%% memory %6.1f/%6.1f GiB RSS %6.1f MiB huge/hugetlb %6.1f/%6.1f MiB QMP %s base/plug %6.1f/%6.1f GiB NUMA %v\n", vm.Name, vm.Running, vm.ConfiguredVCPUs, vm.QMPEnabledVCPUs, vm.CPUPercent, vm.CgroupCPUPercent, float64(vm.MemoryCurrentBytes)/(1024*1024*1024), float64(vm.ConfiguredMemoryBytes)/(1024*1024*1024), float64(vm.ProcessRSSBytes)/(1024*1024), float64(vm.RuntimeAnonHugeBytes)/(1024*1024), float64(vm.RuntimeHugetlbBytes)/(1024*1024), vm.QMPVersion, float64(vm.QMPBaseMemoryBytes)/(1024*1024*1024), float64(vm.QMPPluggedMemoryBytes)/(1024*1024*1024), vm.NUMANodes)
		}
	}
	b.WriteString("\nMemory devices\n")
	for _, memory := range snapshot.MemoryDevices {
		fmt.Fprintf(&b, "  %-16s %6.1f GiB %-12s %d MT/s configured %d  CE/UE %d/%d\n", memory.Locator, float64(memory.SizeBytes)/(1024*1024*1024), memory.Type, memory.SpeedMTs, memory.ConfiguredSpeedMTs, memory.CorrectedErrors, memory.UncorrectedErrors)
	}
	return b.String()
}

func viewThermal(snapshot model.Snapshot) string {
	var b strings.Builder
	b.WriteString("Thermal\n")
	if len(snapshot.Thermal.Zones) == 0 && len(snapshot.Thermal.Sensors) == 0 && len(snapshot.Thermal.Fans) == 0 && len(snapshot.Thermal.Power) == 0 {
		b.WriteString("  No thermal sensors reported.\n")
		return b.String()
	}
	for _, zone := range snapshot.Thermal.Zones {
		fmt.Fprintf(&b, "  %-14s %-18s current %6.1f C  critical %6.1f C  passive %6.1f C  policy %s  mode %s\n", zone.Name, zone.Type, zone.Current, zone.Critical, zone.Passive, zone.Policy, zone.Mode)
	}
	if len(snapshot.Thermal.Sensors) > 0 {
		b.WriteString("\nTemperature sensors\n")
		for _, sensor := range snapshot.Thermal.Sensors {
			alarm := ""
			if sensor.Alarm {
				alarm = "  ALARM"
			}
			line := fmt.Sprintf("  %-8s %-8s %-20s %-10s current %6.1f C  max %6.1f C  critical %6.1f C%s\n", sensor.Name, sensor.Sensor, sensor.Label, sensor.Source, sensor.Current, sensor.Max, sensor.Critical, alarm)
			if sensor.Alarm || (sensor.Critical > 0 && sensor.Current >= sensor.Critical*0.9) {
				line = markWarning + line
			}
			b.WriteString(line)
		}
	}
	if len(snapshot.Thermal.Fans) > 0 {
		b.WriteString("\nFans\n")
		for _, fan := range snapshot.Thermal.Fans {
			line := fmt.Sprintf("  %-8s %-8s %-20s %6d RPM  min %d  max %d\n", fan.Name, fan.Sensor, fan.Label, fan.Input, fan.Min, fan.Max)
			if fan.Input == 0 && (fan.Min > 0 || fan.Max > 0) {
				line = markWarning + line
			}
			b.WriteString(line)
		}
	}
	if len(snapshot.Thermal.Power) > 0 {
		b.WriteString("\nPower / energy\n")
		for _, power := range snapshot.Thermal.Power {
			if power.InputWatts > 0 {
				line := fmt.Sprintf("  %-8s %-8s %-20s %8.1f W  cap %6.1f W  cap-max %6.1f W\n", power.Name, power.Sensor, power.Label, power.InputWatts, power.CapWatts, power.CapMaxWatts)
				if power.Alarm {
					line = markWarning + line
				}
				b.WriteString(line)
			}
			if power.InputJoules > 0 {
				fmt.Fprintf(&b, "  %-8s %-8s %-20s %8.1f J\n", power.Name, power.Sensor, power.Label, power.InputJoules)
			}
		}
	}
	return b.String()
}

func viewTop(snapshot model.Snapshot) string {
	var b strings.Builder
	b.WriteString("Top processes by CPU\n")
	if len(snapshot.TopProcesses) == 0 {
		b.WriteString("  No process samples available.\n")
		return b.String()
	}
	for _, process := range snapshot.TopProcesses {
		qemu := ""
		if isQEMUSample(process, snapshot.Virtualization.VirtualMachines) {
			qemu = "  [QEMU]"
		}
		line := fmt.Sprintf("  %-24s pid %7d  cpu %6.1f%%  rss %10s  state %s%s\n", process.Name, process.PID, process.CPUPercent, formatBytes(process.RSSBytes), process.State, qemu)
		if process.CPUPercent >= 90 {
			line = markWarning + line
		}
		b.WriteString(line)
	}
	b.WriteString("\n  CPU percent is the rate between the last two samples.\n")
	return b.String()
}

func isQEMUSample(process model.ProcessSample, vms []model.VirtualMachine) bool {
	if strings.HasPrefix(process.Name, "qemu-system-") || process.Name == "qemu-kvm" {
		return true
	}
	for _, vm := range vms {
		if vm.PID == process.PID {
			return true
		}
	}
	return false
}

func historyValues(history []model.Snapshot, value func(model.Snapshot) float64) []float64 {
	values := make([]float64, 0, len(history))
	for _, snapshot := range history {
		values = append(values, value(snapshot))
	}
	return values
}

func sparkline(values []float64, min, max float64) string {
	levels := []rune("▁▂▃▄▅▆▇█")
	if len(values) == 0 {
		return "—"
	}
	var b strings.Builder
	for _, value := range values {
		position := 0
		if max > min {
			position = int((value - min) / (max - min) * float64(len(levels)-1))
		}
		if position < 0 {
			position = 0
		}
		if position >= len(levels) {
			position = len(levels) - 1
		}
		b.WriteRune(levels[position])
	}
	return b.String()
}

func formatRate(bytesPerSecond float64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	value := bytesPerSecond
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func formatBytes(bytes uint64) string {
	return formatRate(float64(bytes))
}

func formatGbps(gbps float64) string {
	if gbps <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%.1f Gb/s", gbps)
}

func updatedAt(snapshot model.Snapshot) string {
	if snapshot.CollectedAt.IsZero() {
		return "waiting"
	}
	return snapshot.CollectedAt.Local().Format("15:04:05")
}

func collectNow(collector *collect.Collector) tea.Cmd {
	return func() tea.Msg {
		snapshot := collector.Snapshot()
		snapshot.CollectDurationMS = time.Since(snapshot.CollectedAt).Milliseconds()
		return snapshot
	}
}

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

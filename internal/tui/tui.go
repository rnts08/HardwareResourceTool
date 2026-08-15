package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/model"
)

const historyLimit = 60

type tickMsg time.Time

type modelState struct {
	collector *collect.Collector
	interval  time.Duration
	snapshot  model.Snapshot
	history   []model.Snapshot
	findings  []model.Finding
	err       error
	tab       int
	width     int
	height    int
}

var tabs = []string{"Overview", "Storage", "Network", "Findings"}

func Run(collector *collect.Collector, interval time.Duration) error {
	_, err := tea.NewProgram(modelState{collector: collector, interval: interval}).Run()
	return err
}

func (m modelState) Init() tea.Cmd { return tea.Batch(collectNow(m.collector), tick(m.interval)) }

func (m modelState) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1", "2", "3", "4":
			m.tab = int(msg.String()[0] - '1')
		case "tab", "right", "l":
			m.tab = (m.tab + 1) % len(tabs)
		case "shift+tab", "left", "h":
			m.tab = (m.tab + len(tabs) - 1) % len(tabs)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case model.Snapshot:
		m.snapshot, m.findings, m.err = msg, analyze.Findings(msg), nil
		m.history = append(m.history, msg)
		if len(m.history) > historyLimit {
			m.history = m.history[len(m.history)-historyLimit:]
		}
		return m, tick(m.interval)
	case tickMsg:
		return m, tea.Batch(collectNow(m.collector), tick(m.interval))
	}
	return m, nil
}

func (m modelState) View() string {
	var b strings.Builder
	b.WriteString("Hardware Resources — live host view\n")
	b.WriteString("──────────────────────────────────────────────────────────────────────────────\n")
	for i, tab := range tabs {
		if i == m.tab {
			fmt.Fprintf(&b, "[%d %s] ", i+1, tab)
		} else {
			fmt.Fprintf(&b, " %d %s  ", i+1, tab)
		}
	}
	fmt.Fprintf(&b, "\nSamples: %d/%d  Updated: %s\n\n", len(m.history), historyLimit, updatedAt(m.snapshot))

	switch m.tab {
	case 1:
		viewStorage(&b, m.snapshot)
	case 2:
		viewNetwork(&b, m.snapshot)
	case 3:
		viewFindings(&b, m.findings)
	default:
		viewOverview(&b, m.snapshot, m.history)
	}

	if len(m.snapshot.Errors) > 0 {
		fmt.Fprintf(&b, "\nCollector errors: %s\n", strings.Join(m.snapshot.Errors, "; "))
	}
	b.WriteString("\n1-4: tabs  h/left: previous  l/right/tab: next  q: quit")
	return fitView(b.String(), m.width, m.height)
}

func fitView(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if height > 0 && len(lines) > height {
		footer := lines[len(lines)-1]
		lines = append(lines[:height-2], "… output truncated; switch tabs for detail", footer)
	}
	if width > 0 {
		for i, line := range lines {
			lines[i] = truncate(line, width)
		}
	}
	return strings.Join(lines, "\n")
}

func truncate(value string, width int) string {
	if width <= 0 || len([]rune(value)) <= width {
		return value
	}
	runes := []rune(value)
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func viewOverview(b *strings.Builder, snapshot model.Snapshot, history []model.Snapshot) {
	cpuIdle := historyValues(history, func(s model.Snapshot) float64 { return s.CPU.IdlePercent })
	memoryUsed := historyValues(history, func(s model.Snapshot) float64 { return s.Memory.UsedPercent })
	fmt.Fprintf(b, "CPU     %d logical  user %5.1f%%  system %5.1f%%  iowait %5.1f%%  idle %5.1f%%\n", snapshot.CPU.LogicalCPUs, snapshot.CPU.UserPercent, snapshot.CPU.SystemPercent, snapshot.CPU.IOWaitPercent, snapshot.CPU.IdlePercent)
	fmt.Fprintf(b, "        load %.2f %.2f %.2f  ctxt %d/s  interrupts %d/s\n", snapshot.CPU.Load1, snapshot.CPU.Load5, snapshot.CPU.Load15, snapshot.CPU.ContextSwitch, snapshot.CPU.Interrupts)
	fmt.Fprintf(b, "        idle   %s\n", sparkline(cpuIdle, 0, 100))
	fmt.Fprintf(b, "Memory  used %5.1f%%  available %6.1f GiB  swap in/out %d/%d per sec\n", snapshot.Memory.UsedPercent, float64(snapshot.Memory.AvailableBytes)/(1024*1024*1024), snapshot.Memory.SwapInPerSec, snapshot.Memory.SwapOutPerSec)
	fmt.Fprintf(b, "        used   %s\n", sparkline(memoryUsed, 0, 100))
	fmt.Fprintf(b, "System  governor %q  THP %q  swappiness %d  NUMA nodes %d remote %d/s\n", snapshot.System.CPUGovernor, snapshot.System.THP, snapshot.System.Swappiness, snapshot.NUMA.Nodes, snapshot.NUMA.RemoteEvents)
	fmt.Fprintf(b, "        open files %d  processes %d  stack %s  locked memory %s\n", snapshot.System.OpenFiles, snapshot.System.MaxProcesses, formatBytes(snapshot.System.MaxStack), formatBytes(snapshot.System.MaxLocked))
	fmt.Fprintf(b, "        host/init files %d  host/init processes %d\n", snapshot.System.HostLimits.OpenFiles, snapshot.System.HostLimits.MaxProcesses)
	if len(snapshot.System.Sysctls) > 0 {
		fmt.Fprintf(b, "        sysctls %v\n", snapshot.System.Sysctls)
	}
}

func viewStorage(b *strings.Builder, snapshot model.Snapshot) {
	b.WriteString("Storage\n")
	if len(snapshot.Disks) == 0 {
		b.WriteString("  No block devices reported.\n")
	}
	for _, disk := range snapshot.Disks {
		fmt.Fprintf(b, "  %-16s read %10s/s  write %10s/s  ops %.1f/%.1f  in-flight %d\n", disk.Name, formatRate(disk.ReadBytesPerSec), formatRate(disk.WriteBytesPerSec), disk.ReadsPerSec, disk.WritesPerSec, disk.InFlight)
	}
	b.WriteString("\nFilesystems\n")
	for _, filesystem := range snapshot.Filesystems {
		mode := "rw"
		if filesystem.ReadOnly {
			mode = "ro"
		}
		fmt.Fprintf(b, "  %-24s %-3s %5.1f%% used  available %10s  %s\n", filesystem.MountPoint, mode, filesystem.UsedPercent, formatBytes(filesystem.AvailableBytes), filesystem.Type)
	}
}

func viewNetwork(b *strings.Builder, snapshot model.Snapshot) {
	b.WriteString("Network\n")
	if len(snapshot.Networks) == 0 {
		b.WriteString("  No network interfaces reported.\n")
	}
	for _, network := range snapshot.Networks {
		fmt.Fprintf(b, "  %-16s %-8s rx %10s/s  tx %10s/s  speed %dMb/s  mtu %d  queues %d/%d  rings %d/%d  errors %d/%d  drops %d/%d\n", network.Name, network.State, formatRate(network.RXBytesPerSec), formatRate(network.TXBytesPerSec), network.LinkSpeedMbps, network.MTU, network.RXQueues, network.TXQueues, network.RXRingSize, network.TXRingSize, network.RXErrors, network.TXErrors, network.RXDrops, network.TXDrops)
	}
}

func viewFindings(b *strings.Builder, findings []model.Finding) {
	b.WriteString("Findings\n")
	if len(findings) == 0 {
		b.WriteString("  No findings.\n")
		return
	}
	for _, finding := range findings {
		fmt.Fprintf(b, "  [%s] %s\n    %s\n    Recommendation: %s\n", finding.Severity, finding.Title, finding.Evidence, finding.Recommendation)
	}
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

func updatedAt(snapshot model.Snapshot) string {
	if snapshot.CollectedAt.IsZero() {
		return "waiting"
	}
	return snapshot.CollectedAt.Local().Format("15:04:05")
}

func collectNow(collector *collect.Collector) tea.Cmd {
	return func() tea.Msg { return collector.Snapshot() }
}

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/model"
)

type tickMsg time.Time
type modelState struct {
	collector *collect.Collector
	interval  time.Duration
	snapshot  model.Snapshot
	findings  []model.Finding
	err       error
}

func Run(collector *collect.Collector, interval time.Duration) error {
	_, err := tea.NewProgram(modelState{collector: collector, interval: interval}).Run()
	return err
}

func (m modelState) Init() tea.Cmd { return tea.Batch(collectNow(m.collector), tick(m.interval)) }

func (m modelState) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case model.Snapshot:
		m.snapshot, m.findings, m.err = msg, analyze.Findings(msg), nil
		return m, tick(m.interval)
	case tickMsg:
		return m, tea.Batch(collectNow(m.collector), tick(m.interval))
	}
	return m, nil
}

func (m modelState) View() string {
	var b strings.Builder
	b.WriteString("Hardware Resources — live host view\n\n")
	fmt.Fprintf(&b, "CPU     user %5.1f%%  system %5.1f%%  iowait %5.1f%%  idle %5.1f%%  load %.2f %.2f %.2f\n", m.snapshot.CPU.UserPercent, m.snapshot.CPU.SystemPercent, m.snapshot.CPU.IOWaitPercent, m.snapshot.CPU.IdlePercent, m.snapshot.CPU.Load1, m.snapshot.CPU.Load5, m.snapshot.CPU.Load15)
	fmt.Fprintf(&b, "Memory  used %5.1f%%  available %6.1f GiB  swap in/out %d/%d per sec\n", m.snapshot.Memory.UsedPercent, float64(m.snapshot.Memory.AvailableBytes)/(1024*1024*1024), m.snapshot.Memory.SwapInPerSec, m.snapshot.Memory.SwapOutPerSec)
	b.WriteString("\nStorage\n")
	for _, disk := range m.snapshot.Disks {
		fmt.Fprintf(&b, "  %-12s read %8.1f KiB/s  write %8.1f KiB/s  in-flight %d\n", disk.Name, disk.ReadBytesPerSec/1024, disk.WriteBytesPerSec/1024, disk.InFlight)
	}
	b.WriteString("\nNetwork\n")
	for _, network := range m.snapshot.Networks {
		fmt.Fprintf(&b, "  %-12s %-8s rx %8.1f KiB/s  tx %8.1f KiB/s  errors/drops %d/%d\n", network.Name, network.State, network.RXBytesPerSec/1024, network.TXBytesPerSec/1024, network.RXErrors+network.TXErrors, network.RXDrops+network.TXDrops)
	}
	b.WriteString("\nFindings\n")
	if len(m.findings) == 0 {
		b.WriteString("  No findings yet.\n")
	}
	for _, finding := range m.findings {
		fmt.Fprintf(&b, "  [%s] %s — %s\n", finding.Severity, finding.Title, finding.Evidence)
	}
	if len(m.snapshot.Errors) > 0 {
		fmt.Fprintf(&b, "\nCollector errors: %s\n", strings.Join(m.snapshot.Errors, "; "))
	}
	b.WriteString("\nPress q to quit")
	return b.String()
}

func collectNow(collector *collect.Collector) tea.Cmd {
	return func() tea.Msg { return collector.Snapshot() }
}
func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

package tui

import (
	"strings"
	"testing"

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
	view := (modelState{tab: 3, thresholds: analyze.DefaultThresholds}).View()
	for _, expected := range []string{"[4 Findings]", "No findings.", "1-7: tabs"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func TestViewShowsThermalTabAndEmptyState(t *testing.T) {
	view := (modelState{tab: 5, thresholds: analyze.DefaultThresholds}).View()
	for _, expected := range []string{"[6 Thermal]", "No thermal sensors reported."} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
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
	view := viewTop(snapshot)
	for _, expected := range []string{"qemu-system-x86_64", "pid      42", "cpu   95.5%", "1.0 GiB"} {
		if !contains(view, expected) {
			t.Fatalf("view missing %q: %s", expected, view)
		}
	}
}

func TestViewTopEmptyState(t *testing.T) {
	if !contains(viewTop(model.Snapshot{}), "No process samples available.") {
		t.Fatal("empty top view missing hint")
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

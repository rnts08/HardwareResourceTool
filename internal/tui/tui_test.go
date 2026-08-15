package tui

import (
	"strings"
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestSparklineClampsAndPreservesSamples(t *testing.T) {
	got := sparkline([]float64{-10, 0, 50, 100, 110}, 0, 100)
	if got != "▁▁▄█"+"█" {
		t.Fatalf("unexpected sparkline %q", got)
	}
}

func TestViewShowsTabsAndEmptyState(t *testing.T) {
	view := (modelState{tab: 3}).View()
	for _, expected := range []string{"[4 Findings]", "No findings.", "1-4: tabs"} {
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

func TestFitViewRespectsTerminalSize(t *testing.T) {
	view := fitView("one\ntwo\nthree\nfooter", 4, 3)
	if !contains(view, "… o") || !contains(view, "foo") {
		t.Fatalf("view was not clipped as expected: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if len([]rune(line)) > 4 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}

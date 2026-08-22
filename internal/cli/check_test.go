package cli

import (
	"strings"
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestCheckExitCodeTable(t *testing.T) {
	cases := []struct {
		name     string
		result   model.Report
		wantCode int
		wantSub  string
	}{
		{"pass", model.Report{Findings: []model.Finding{{Severity: "info"}}}, 0, "PASS"},
		{"warning", model.Report{Findings: []model.Finding{{Severity: "warning"}}}, 1, "WARNING"},
		{"critical", model.Report{Findings: []model.Finding{{Severity: "warning"}, {Severity: "critical"}}}, 2, "CRITICAL"},
		{"collector errors outrank findings", model.Report{
			Findings: []model.Finding{{Severity: "critical"}},
			Snapshot: model.Snapshot{Errors: []string{"memory: boom"}},
		}, 3, "COLLECTION ERRORS"},
	}
	for _, tc := range cases {
		code, summary := checkExitCode(tc.result)
		if code != tc.wantCode {
			t.Fatalf("%s: code = %d, want %d (%s)", tc.name, code, tc.wantCode, summary)
		}
		if !strings.Contains(summary, tc.wantSub) {
			t.Fatalf("%s: summary %q missing %q", tc.name, summary, tc.wantSub)
		}
	}
}

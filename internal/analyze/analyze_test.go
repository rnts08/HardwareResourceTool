package analyze

import (
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestFindingsDetectResourcePressure(t *testing.T) {
	snapshot := model.Snapshot{
		CPU:      model.CPU{LogicalCPUs: 8, IdlePercent: 4, IOWaitPercent: 20},
		Memory:   model.Memory{UsedPercent: 95, SwapOutPerSec: 3},
		Networks: []model.Network{{Name: "eth0", RXErrors: 1}},
	}

	findings := Findings(snapshot)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings, got %d", len(findings))
	}
	if findings[0].Severity != "critical" {
		t.Fatalf("expected CPU finding to be critical, got %q", findings[0].Severity)
	}
}

func TestFindingsReportCollectionErrors(t *testing.T) {
	findings := Findings(model.Snapshot{Errors: []string{"missing metric"}})
	if len(findings) != 1 || findings[0].Category != "collection" {
		t.Fatalf("expected collection finding, got %#v", findings)
	}
}

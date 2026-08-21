package collect

import (
	"path/filepath"
	"testing"

	"hardware-resources-tool/internal/model"
)

// TestCollectThermalDropsImplausibleLimits is a regression for NVMe drives
// that report garbage drive-side maxima (observed: temp2_max = 65261850,
// i.e. 65261.85 C). Such limits must be surfaced as unknown (0), while sane
// values pass through unchanged.
func TestCollectThermalDropsImplausibleLimits(t *testing.T) {
	sys := t.TempDir()
	hwmon := filepath.Join(sys, "class/hwmon/hwmon0")
	writeFixture(t, hwmon, "name", "nvme\n")
	writeFixture(t, hwmon, "temp1_input", "33900\n")
	writeFixture(t, hwmon, "temp1_max", "82800\n")
	writeFixture(t, hwmon, "temp1_crit", "84800\n")
	writeFixture(t, hwmon, "temp2_input", "27850\n")
	writeFixture(t, hwmon, "temp2_max", "65261850\n")

	snapshot := model.Snapshot{}
	if err := (&Collector{sysRoot: sys}).collectThermal(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Thermal.Sensors) != 2 {
		t.Fatalf("expected 2 sensors, got %#v", snapshot.Thermal.Sensors)
	}
	sane := snapshot.Thermal.Sensors[0]
	if sane.Max != 82.8 || sane.Critical != 84.8 {
		t.Fatalf("plausible limits altered: %#v", sane)
	}
	bogus := snapshot.Thermal.Sensors[1]
	if bogus.Max != 0 {
		t.Fatalf("implausible limit not dropped: %#v", bogus)
	}
}

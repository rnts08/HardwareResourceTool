package collect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestEnrichPCITopologyFindsMinimumUpstreamLink(t *testing.T) {
	root := t.TempDir()
	physical := filepath.Join(root, "devices", "pci0000:00", "0000:00:01.0", "0000:01:00.0")
	if err := os.MkdirAll(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	devicePath := filepath.Join(root, "bus", "pci", "devices", "0000:01:00.0")
	if err := os.MkdirAll(filepath.Dir(devicePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, devicePath); err != nil {
		t.Fatal(err)
	}
	devices := []model.PCIDevice{
		{Address: "0000:01:00.0", PCIeNegotiatedSpeed: "16.0 GT/s PCIe", PCIeNegotiatedWidth: 16},
		{Address: "0000:00:01.0", PCIeNegotiatedSpeed: "8.0 GT/s PCIe", PCIeNegotiatedWidth: 8},
	}
	enrichPCITopology([]string{devicePath}, devices)
	if !reflect.DeepEqual(devices[0].PCIePath, []string{"0000:01:00.0", "0000:00:01.0"}) {
		t.Fatalf("PCIe path = %#v", devices[0].PCIePath)
	}
	if devices[0].PCIePathBottleneck != "0000:00:01.0" || devices[0].PCIePathMinWidth != 8 || devices[0].PCIePathBandwidthGbps < 63.0 || devices[0].PCIePathBandwidthGbps > 63.2 {
		t.Fatalf("PCIe path minimum = %#v", devices[0])
	}
}

func TestPCIEBandwidthRejectsIncompleteLinks(t *testing.T) {
	if value, ok := pcieBandwidth("", 16); ok || value != 0 {
		t.Fatalf("empty speed = %v, %v", value, ok)
	}
	if value, ok := pcieBandwidth("not-a-speed", 16); ok || value != 0 {
		t.Fatalf("invalid speed = %v, %v", value, ok)
	}
}

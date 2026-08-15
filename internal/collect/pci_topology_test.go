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

func TestCollectPCIResourcesAndVirtualFunctions(t *testing.T) {
	sys := t.TempDir()
	device := filepath.Join(sys, "bus/pci/devices/0000:01:00.0")
	if err := os.MkdirAll(device, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"vendor": "0x8086\n", "device": "0x1234\n", "class": "0x020000\n", "resource": "0000000100000000 0000000100000fff 0000000000000200\n00000000 00000000 00000000\n"} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("0000:01:00.0", filepath.Join(device, "physfn")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("0000:01:00.1", filepath.Join(device, "virtfn0")); err != nil {
		t.Fatal(err)
	}
	snapshot := model.Snapshot{PCI: []model.PCIDevice{}, GPUs: []model.GPU{}}
	if err := (&Collector{sysRoot: sys}).collectPCI(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PCI) != 1 || snapshot.PCI[0].PCIePFAddress != "0000:01:00.0" || !reflect.DeepEqual(snapshot.PCI[0].PCIeVFAddresses, []string{"0000:01:00.1"}) || snapshot.PCI[0].BARCount != 1 || snapshot.PCI[0].BARTotalBytes != 4096 || !snapshot.PCI[0].BARAbove4G {
		t.Fatalf("unexpected PCI resource relationships: %#v", snapshot.PCI)
	}
}

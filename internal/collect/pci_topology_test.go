package collect

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestPCIBarFromResourceFlags(t *testing.T) {
	cases := []struct {
		flags    uint64
		typ      string
		prefetch bool
		rom      bool
	}{
		{0x00000200, "memory", false, false},
		{0x00001200, "memory", true, false},
		{0x00100200, "64-bit memory", false, false},
		{0x00101200, "64-bit memory", true, false},
		{0x00000100, "io", false, false},
		{0x00002200, "memory", false, true},
	}
	for _, c := range cases {
		bar := pciBarFromResource(0, 0x1000, 0x1fff, c.flags)
		if bar.Type != c.typ || bar.Prefetchable != c.prefetch || bar.ROM != c.rom {
			t.Fatalf("flags 0x%x -> %#v", c.flags, bar)
		}
	}
}

func TestCollectPCIResourcesStructuredBarsAndWindows(t *testing.T) {
	sys := t.TempDir()
	devicePath := filepath.Join(sys, "bus/pci/devices/0000:02:00.0")
	if err := os.MkdirAll(devicePath, 0o755); err != nil {
		t.Fatal(err)
	}
	resource := strings.Join([]string{
		"00000000e0000000 00000000e0ffffff 0000000000001200", // memory, prefetchable
		"0000001000000000 0000001000000fff 0000000000100200", // 64-bit memory
		"000000000000c000 000000000000c0ff 0000000000000100", // io
		"00000000 00000000 00000000",
		"00000000 00000000 00000000",
		"00000000 00000000 00000000",
		"00000000f0000000 00000000f00fffff 0000000000002200", // expansion ROM
		"00000000e2000000 00000000e2ffffff 0000000000000200", // bridge memory window
		"00000000f1000000 00000000f10fffff 0000000000001200", // bridge prefetch window
	}, "\n") + "\n"
	for name, content := range map[string]string{"vendor": "0x8086\n", "device": "0x5678\n", "class": "0x060400\n", "resource": resource} {
		if err := os.WriteFile(filepath.Join(devicePath, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := model.Snapshot{PCI: []model.PCIDevice{}, GPUs: []model.GPU{}}
	if err := (&Collector{sysRoot: sys}).collectPCI(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PCI) != 1 {
		t.Fatalf("expected one PCI device, got %d", len(snapshot.PCI))
	}
	dev := snapshot.PCI[0]
	if dev.BARCount != 3 || dev.BARTotalBytes != 0x1000000+0x1000+0x100 {
		t.Fatalf("BAR totals = %d/%d", dev.BARCount, dev.BARTotalBytes)
	}
	if !dev.BARAbove4G {
		t.Fatal("expected an above-4G BAR")
	}
	if len(dev.BARs) != 3 {
		t.Fatalf("structured BARs = %#v", dev.BARs)
	}
	if dev.BARs[0].Type != "memory" || !dev.BARs[0].Prefetchable {
		t.Fatalf("BAR[0] = %#v", dev.BARs[0])
	}
	if dev.BARs[1].Type != "64-bit memory" || dev.BARs[1].Prefetchable {
		t.Fatalf("BAR[1] = %#v", dev.BARs[1])
	}
	if dev.BARs[2].Type != "io" || dev.BARs[2].ROM {
		t.Fatalf("BAR[2] = %#v", dev.BARs[2])
	}
	if !dev.ROM {
		t.Fatal("expected expansion ROM to be detected")
	}
	if !reflect.DeepEqual(dev.ResourceWindows, []string{"00000000e2000000-00000000e2ffffff", "00000000f1000000-00000000f10fffff"}) {
		t.Fatalf("resource windows = %#v", dev.ResourceWindows)
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

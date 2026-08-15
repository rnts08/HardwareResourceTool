package collect

import (
	"os"
	"path/filepath"
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestCollectCPUCountersAndRates(t *testing.T) {
	proc := t.TempDir()
	writeFixture(t, proc, "stat", "cpu 100 20 30 400 5 6 7 8 0 0\nctxt 1000\nintr 2000\n")
	writeFixture(t, proc, "loadavg", "1.00 2.00 3.00 1/100 42\n")
	c := &Collector{procRoot: proc}
	firstRaw := rawCounters{}
	if err := c.collectCPU(&model.Snapshot{}, &firstRaw); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, proc, "stat", "cpu 200 40 60 800 15 16 17 18 0 0\nctxt 1200\nintr 2400\n")
	secondRaw := rawCounters{}
	snapshot := model.Snapshot{}
	if err := c.collectCPU(&snapshot, &secondRaw); err != nil {
		t.Fatal(err)
	}
	applyRates(&snapshot, &firstRaw, &secondRaw, 2)
	if snapshot.CPU.ContextSwitch != 100 || snapshot.CPU.Interrupts != 200 {
		t.Fatalf("unexpected rates: ctxt=%d interrupts=%d", snapshot.CPU.ContextSwitch, snapshot.CPU.Interrupts)
	}
	if snapshot.CPU.IdlePercent <= 0 || snapshot.CPU.SystemPercent <= 0 {
		t.Fatalf("expected CPU percentages, got %#v", snapshot.CPU)
	}
}

func TestCollectMemoryReadsVMStat(t *testing.T) {
	proc := t.TempDir()
	writeFixture(t, proc, "meminfo", "MemTotal:       1024 kB\nMemAvailable:    512 kB\nSwapTotal:       256 kB\nSwapFree:        128 kB\n")
	writeFixture(t, proc, "vmstat", "pswpin 11\npswpout 7\n")
	raw := rawCounters{}
	snapshot := model.Snapshot{}
	if err := (&Collector{procRoot: proc}).collectMemory(&snapshot, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.swapIn != 11 || raw.swapOut != 7 {
		t.Fatalf("unexpected swap counters: %d/%d", raw.swapIn, raw.swapOut)
	}
}

func TestCollectNetworkMetadata(t *testing.T) {
	sys := t.TempDir()
	base := filepath.Join(sys, "class/net/eth0")
	for _, key := range []string{"rx_bytes", "tx_bytes", "rx_packets", "tx_packets", "rx_errors", "tx_errors", "rx_dropped", "tx_dropped"} {
		writeFixture(t, filepath.Join(base, "statistics"), key, "1\n")
	}
	writeFixture(t, base, "operstate", "up\n")
	writeFixture(t, base, "speed", "1000\n")
	writeFixture(t, base, "mtu", "1500\n")
	if err := os.MkdirAll(filepath.Join(base, "queues/rx-0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "queues/tx-0"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := model.Snapshot{Networks: []model.Network{}}
	if err := (&Collector{sysRoot: sys}).collectNetworks(&snapshot, &rawCounters{networks: map[string]networkCounter{}}); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Networks) != 1 || snapshot.Networks[0].LinkSpeedMbps != 1000 || snapshot.Networks[0].RXQueues != 1 {
		t.Fatalf("unexpected network metadata: %#v", snapshot.Networks)
	}
}

func TestCollectFilesystemCapacity(t *testing.T) {
	proc := t.TempDir()
	mountPoint := t.TempDir()
	writeFixture(t, proc, "mounts", "fixture "+mountPoint+" tmpfs rw,nosuid 0 0\n")
	snapshot := model.Snapshot{Filesystems: []model.Filesystem{}}
	if err := (&Collector{procRoot: proc}).collectFilesystems(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Filesystems) != 1 || snapshot.Filesystems[0].MountPoint != mountPoint || snapshot.Filesystems[0].ReadOnly {
		t.Fatalf("unexpected filesystems: %#v", snapshot.Filesystems)
	}
}

func TestCollectSystemReadsSelectedSysctls(t *testing.T) {
	proc := t.TempDir()
	sys := t.TempDir()
	writeFixture(t, filepath.Join(proc, "sys/vm"), "overcommit_memory", "2\n")
	writeFixture(t, filepath.Join(proc, "sys/vm"), "dirty_ratio", "25\n")
	writeFixture(t, filepath.Join(proc, "sys/vm"), "dirty_background_ratio", "10\n")
	writeFixture(t, filepath.Join(proc, "sys/kernel"), "nmi_watchdog", "1\n")
	snapshot := model.Snapshot{}
	if err := (&Collector{procRoot: proc, sysRoot: sys}).collectSystem(&snapshot, &rawCounters{}); err != nil {
		t.Fatal(err)
	}
	if snapshot.System.Sysctls["vm.overcommit_memory"] != "2" || snapshot.System.Sysctls["vm.dirty_ratio"] != "25" {
		t.Fatalf("unexpected sysctls: %#v", snapshot.System.Sysctls)
	}
}

func TestParseSMBIOSMemoryDevice(t *testing.T) {
	formatted := make([]byte, 0x22)
	formatted[0] = 17
	formatted[1] = 0x22
	formatted[0x0c] = 0x00
	formatted[0x0d] = 0x20 // 8192 MiB
	formatted[0x10] = 1    // locator
	formatted[0x12] = 2    // memory type
	formatted[0x15] = 0x00
	formatted[0x16] = 0x0c // 3072 MT/s
	formatted[0x17] = 3    // manufacturer
	formatted[0x18] = 4    // serial
	formatted[0x1a] = 5    // part number
	formatted[0x20] = 0x00
	formatted[0x21] = 0x0c
	data := append(formatted, []byte("DIMM_A1\x00DDR5\x00Vendor\x001234\x00PART-1\x00\x00")...)
	devices := parseSMBIOSMemory(data)
	if len(devices) != 1 || devices[0].Locator != "DIMM_A1" || devices[0].SizeBytes != 8*1024*1024*1024 || devices[0].ConfiguredSpeedMTs != 3072 {
		t.Fatalf("unexpected SMBIOS devices: %#v", devices)
	}
}

func TestCollectPCIAndNVIDIAIdentity(t *testing.T) {
	sys := t.TempDir()
	device := filepath.Join(sys, "bus/pci/devices/0000:01:00.0")
	writeFixture(t, device, "vendor", "0x10de\n")
	writeFixture(t, device, "device", "0x1db6\n")
	writeFixture(t, device, "class", "0x030200\n")
	writeFixture(t, device, "numa_node", "1\n")
	writeFixture(t, device, "current_link_speed", "16.0 GT/s PCIe\n")
	writeFixture(t, device, "current_link_width", "16\n")
	snapshot := model.Snapshot{PCI: []model.PCIDevice{}, GPUs: []model.GPU{}}
	if err := (&Collector{sysRoot: sys}).collectPCI(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PCI) != 1 || len(snapshot.GPUs) != 1 || snapshot.GPUs[0].Address != "0000:01:00.0" {
		t.Fatalf("unexpected PCI/GPU inventory: %#v %#v", snapshot.PCI, snapshot.GPUs)
	}
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func FuzzParseUint(f *testing.F) {
	for _, seed := range []string{"0", "42", " 18446744073709551615 ", "not-a-number", "-1"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = parseUint(value)
	})
}

func FuzzDecodeMountField(f *testing.F) {
	for _, seed := range []string{"/var/lib", `/path\040with\040spaces`, `/path\011tab`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		_ = decodeMountField(value)
	})
}

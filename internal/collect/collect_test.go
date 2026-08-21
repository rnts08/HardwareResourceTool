package collect

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
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
	if err := os.MkdirAll(filepath.Join(sys, "devices/pci0000:00/0000:00:03.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(sys, "devices/pci0000:00/0000:00:03.0"), filepath.Join(base, "device")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sys, "class/net/veth0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "queues/rx-0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "queues/tx-0"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := model.Snapshot{Networks: []model.Network{}}
	if err := (&Collector{sysRoot: sys}).collectNetworks(&snapshot, &rawCounters{networks: map[string]networkCounter{}}, true); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Networks) != 1 || snapshot.Networks[0].LinkSpeedMbps != 1000 || snapshot.Networks[0].RXQueues != 1 || !snapshot.Networks[0].Physical || snapshot.VirtualNetworkCount != 1 {
		t.Fatalf("unexpected network metadata: %#v", snapshot.Networks)
	}
}

func TestCollectFilesystemCapacity(t *testing.T) {
	proc := t.TempDir()
	sys := t.TempDir()
	mountPoint := t.TempDir()
	writePhysicalBlockFixture(t, sys, "sda1", false)
	writeFixture(t, proc, "mounts", "/dev/sda1 "+mountPoint+" ext4 rw,relatime 0 0\n")
	snapshot := model.Snapshot{Filesystems: []model.Filesystem{}}
	if err := (&Collector{procRoot: proc, sysRoot: sys}).collectFilesystems(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Filesystems) != 1 || snapshot.Filesystems[0].MountPoint != mountPoint || snapshot.Filesystems[0].ReadOnly {
		t.Fatalf("unexpected filesystems: %#v", snapshot.Filesystems)
	}
}

func TestCollectFilesystemFiltersPseudoFilesystems(t *testing.T) {
	proc := t.TempDir()
	sys := t.TempDir()
	mountPoint := t.TempDir()
	writePhysicalBlockFixture(t, sys, "sda1", false)
	writeFixture(t, proc, "mounts", "proc /proc proc rw,nosuid 0 0\n/dev/sda1 "+mountPoint+" ext4 rw,nosuid 0 0\n/dev/shm /dev/shm tmpfs rw,nosuid 0 0\n")
	snapshot := model.Snapshot{Filesystems: []model.Filesystem{}}
	if err := (&Collector{procRoot: proc, sysRoot: sys}).collectFilesystems(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Filesystems) != 1 || snapshot.Filesystems[0].MountPoint != mountPoint {
		t.Fatalf("pseudo filesystem was not filtered: %#v", snapshot.Filesystems)
	}
}

func TestFilesystemPolicyExcludesRemovableAndRuntimeMounts(t *testing.T) {
	proc, sys, etc := t.TempDir(), t.TempDir(), t.TempDir()
	point := t.TempDir()
	networkPoint := filepath.Join(t.TempDir(), "archive")
	if err := os.MkdirAll(networkPoint, 0o755); err != nil {
		t.Fatal(err)
	}
	writePhysicalBlockFixture(t, sys, "sdb1", true)
	writePhysicalBlockFixture(t, sys, "sda1", false)
	writeFixture(t, etc, "fstab", "server:/export "+networkPoint+" nfs4 _netdev,defaults 0 0\n")
	writeFixture(t, proc, "mounts", "/dev/sdb1 "+point+" ext4 rw 0 0\n/dev/sda1 /run/data ext4 rw 0 0\n/dev/sda1 /var/lib/docker ext4 rw 0 0\nserver:/export "+networkPoint+" nfs4 rw,relatime 0 0\nserver:/other /mnt/other nfs4 rw,relatime 0 0\n")
	snapshot := model.Snapshot{Filesystems: []model.Filesystem{}}
	if err := (&Collector{procRoot: proc, sysRoot: sys, etcRoot: etc}).collectFilesystems(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Filesystems) != 1 || snapshot.Filesystems[0].MountPoint != networkPoint {
		t.Fatalf("unexpected filtered filesystems: %#v", snapshot.Filesystems)
	}
}

func writePhysicalBlockFixture(t *testing.T, sys, name string, usb bool) {
	t.Helper()
	base := filepath.Join(sys, "class/block", name)
	device := filepath.Join(sys, "devices", "pci0000:00")
	if usb {
		device = filepath.Join(sys, "devices", "pci0000:00", "usb1", "1-1")
	}
	if err := os.MkdirAll(device, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(device, filepath.Join(base, "device")); err != nil {
		t.Fatal(err)
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

func TestCollectKernelEventsIsBoundedAndClassified(t *testing.T) {
	logs := t.TempDir()
	writeFixture(t, logs, "kern.log", "Jan 1 kernel: Out of memory: Killed process 123\nJan 1 nvme nvme0: I/O error, reset controller\nJan 1 pcieport 0000:00:1c.0: AER: Corrected error received\nJan 1 NVRM: Xid (79)\n")
	events := collectKernelEvents(logs)
	if events.OOM != 1 || events.IOErrors != 1 || events.StorageResets != 1 || events.PCIeErrors != 1 || events.NVIDIA != 1 || len(events.Recent) != 4 {
		t.Fatalf("unexpected kernel events: %#v", events)
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
	config := make([]byte, 0x120)
	config[0x06] = 0x10 // capabilities list present
	config[0x34] = 0x40
	config[0x40] = 0x10 // PCI Express
	config[0x41] = 0x00
	config[0x44] = 0x02                                            // max payload: 512 bytes
	config[0x45] = 0x20                                            // max read request: 512 bytes
	binary.LittleEndian.PutUint32(config[0x4c:0x50], 0x00000104)   // Gen4 x16 capability
	binary.LittleEndian.PutUint16(config[0x52:0x54], 0x00000083)   // Gen3 x8 status
	binary.LittleEndian.PutUint32(config[0x100:0x104], 0x00000001) // AER
	binary.LittleEndian.PutUint32(config[0x104:0x108], 0x00000004)
	binary.LittleEndian.PutUint32(config[0x110:0x114], 0x00000008)
	writeFixtureBytes(t, device, "config", config)
	snapshot := model.Snapshot{PCI: []model.PCIDevice{}, GPUs: []model.GPU{}}
	if err := (&Collector{sysRoot: sys}).collectPCI(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PCI) != 1 || len(snapshot.GPUs) != 1 || snapshot.GPUs[0].Address != "0000:01:00.0" {
		t.Fatalf("unexpected PCI/GPU inventory: %#v %#v", snapshot.PCI, snapshot.GPUs)
	}
	deviceInfo := snapshot.PCI[0]
	if !reflect.DeepEqual(deviceInfo.Capabilities, []string{"pcie", "aer"}) || deviceInfo.PCIeMaxPayloadBytes != 512 || deviceInfo.PCIeMaxReadRequestBytes != 512 || deviceInfo.PCIeCapabilityMaxSpeed != "16.0 GT/s PCIe" || deviceInfo.PCIeCapabilityMaxWidth != 16 || deviceInfo.PCIeNegotiatedSpeed != "8.0 GT/s PCIe" || deviceInfo.PCIeNegotiatedWidth != 8 || deviceInfo.AERUncorrectableStatus != 4 || deviceInfo.AERCorrectableStatus != 8 {
		t.Fatalf("unexpected PCI capabilities: %#v", deviceInfo)
	}
}

func TestNormalizePCIAddress(t *testing.T) {
	if got := normalizePCIAddress("00000000:65:00.0"); got != "0000:65:00.0" {
		t.Fatalf("normalized PCI address = %q", got)
	}
	if got := normalizePCIAddress("0000:65:00.0"); got != "0000:65:00.0" {
		t.Fatalf("canonical PCI address changed = %q", got)
	}
}

func TestWalkPCICapabilitiesBoundsMalformedChains(t *testing.T) {
	data := make([]byte, 0x100)
	data[0x06] = 0x10
	data[0x34] = 0x40
	data[0x40] = 0x10
	data[0x41] = 0x40 // self-loop must terminate
	if got := walkPCICapabilities(data); len(got) != 1 || got[0].offset != 0x40 {
		t.Fatalf("self-loop capabilities = %#v", got)
	}

	truncated := make([]byte, 0x104)
	binary.LittleEndian.PutUint32(truncated[0x100:], 0x00000001)
	if got := walkPCICapabilities(truncated); len(got) != 1 || !got[0].extended {
		t.Fatalf("truncated extended capabilities = %#v", got)
	}
}

func TestCollectThermalReadsZonesSensorsFansAndPower(t *testing.T) {
	sys := t.TempDir()
	zone := filepath.Join(sys, "class/thermal/thermal_zone0")
	writeFixture(t, zone, "type", "x86_pkg_temp\n")
	writeFixture(t, zone, "policy", "step_wise\n")
	writeFixture(t, zone, "mode", "enabled\n")
	writeFixture(t, zone, "temp", "45000\n")
	writeFixture(t, zone, "trip_point_0_temp", "100000\n")
	writeFixture(t, zone, "trip_point_0_type", "critical\n")
	hwmon := filepath.Join(sys, "class/hwmon/hwmon0")
	writeFixture(t, hwmon, "name", "coretemp\n")
	writeFixture(t, hwmon, "temp1_label", "Package id 0\n")
	writeFixture(t, hwmon, "temp1_input", "45200\n")
	writeFixture(t, hwmon, "temp1_max", "80000\n")
	writeFixture(t, hwmon, "temp1_crit", "100000\n")
	writeFixture(t, hwmon, "temp1_alarm", "1\n")
	writeFixture(t, hwmon, "fan1_input", "2400\n")
	writeFixture(t, hwmon, "fan1_min", "600\n")
	writeFixture(t, hwmon, "fan1_max", "5000\n")
	writeFixture(t, hwmon, "power1_input", "45000000\n")
	writeFixture(t, hwmon, "power1_cap", "125000000\n")
	writeFixture(t, hwmon, "energy1_input", "123456789\n")
	gpu := filepath.Join(sys, "class/hwmon/hwmon1")
	writeFixture(t, gpu, "name", "amdgpu\n")
	writeFixture(t, gpu, "temp1_input", "58000\n")
	if err := os.MkdirAll(filepath.Join(sys, "devices/pci0000:00/0000:00:03.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(sys, "devices/pci0000:00/0000:00:03.0"), filepath.Join(gpu, "device")); err != nil {
		t.Fatal(err)
	}
	snapshot := model.Snapshot{}
	if err := (&Collector{sysRoot: sys}).collectThermal(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Thermal.Zones) != 1 {
		t.Fatalf("expected 1 thermal zone, got %#v", snapshot.Thermal.Zones)
	}
	zoneData := snapshot.Thermal.Zones[0]
	if zoneData.Type != "x86_pkg_temp" || zoneData.Current != 45 || zoneData.Critical != 100 || zoneData.Policy != "step_wise" {
		t.Fatalf("unexpected thermal zone: %#v", zoneData)
	}
	if len(snapshot.Thermal.Sensors) != 2 {
		t.Fatalf("expected 2 temperature sensors, got %#v", snapshot.Thermal.Sensors)
	}
	sensor := snapshot.Thermal.Sensors[0]
	if sensor.Label != "Package id 0" || sensor.Source != "coretemp" || sensor.Kind != "cpu" || sensor.Current != 45.2 || sensor.Critical != 100 || !sensor.Alarm {
		t.Fatalf("unexpected temperature sensor: %#v", sensor)
	}
	if snapshot.Thermal.Sensors[1].Kind != "gpu" || snapshot.Thermal.Sensors[1].PCIPath != "0000:00:03.0" {
		t.Fatalf("unexpected gpu sensor with PCI correlation: %#v", snapshot.Thermal.Sensors[1])
	}
	if len(snapshot.Thermal.Fans) != 1 {
		t.Fatalf("expected 1 fan, got %#v", snapshot.Thermal.Fans)
	}
	if snapshot.Thermal.Fans[0].Input != 2400 || snapshot.Thermal.Fans[0].Min != 600 {
		t.Fatalf("unexpected fan: %#v", snapshot.Thermal.Fans[0])
	}
	if len(snapshot.Thermal.Power) != 2 {
		t.Fatalf("expected 2 power/energy sensors, got %#v", snapshot.Thermal.Power)
	}
	power := snapshot.Thermal.Power[0]
	if power.Sensor != "1" || power.Label != "" || power.InputWatts != 45 || power.CapWatts != 125 || power.InputJoules != 0 {
		t.Fatalf("unexpected power sensor: %#v", power)
	}
	energy := snapshot.Thermal.Power[1]
	if energy.InputJoules != 123.456789 || energy.InputWatts != 0 {
		t.Fatalf("unexpected energy sensor: %#v", energy)
	}
}

func TestCorrelateGPUThermalMergesHwmon(t *testing.T) {
	snapshot := model.Snapshot{
		Thermal: model.Thermal{
			Sensors: []model.Temperature{{Name: "hwmon1", Kind: "gpu", Current: 58, PCIPath: "0000:00:03.0"}},
			Power:   []model.PowerSensor{{Name: "hwmon1", Kind: "gpu", InputWatts: 150, PCIPath: "0000:00:03.0"}},
		},
		GPUs: []model.GPU{{Address: "0000:00:03.0", Name: "GPU A", NVML: false}},
	}
	correlateGPUThermal(&snapshot)
	if snapshot.GPUs[0].TemperatureCelsius != 58 || snapshot.GPUs[0].PowerWatts != 150 {
		t.Fatalf("expected merged thermal on GPU, got %#v", snapshot.GPUs[0])
	}
}

func TestCorrelateGPUThermalKeepsNVMLValues(t *testing.T) {
	snapshot := model.Snapshot{
		Thermal: model.Thermal{
			Sensors: []model.Temperature{{Name: "hwmon1", Kind: "gpu", Current: 58, PCIPath: "0000:00:03.0"}},
		},
		GPUs: []model.GPU{{Address: "0000:00:03.0", TemperatureCelsius: 65, PowerWatts: 200, NVML: true}},
	}
	correlateGPUThermal(&snapshot)
	if snapshot.GPUs[0].TemperatureCelsius != 65 || snapshot.GPUs[0].PowerWatts != 200 {
		t.Fatalf("NVML values must win, got %#v", snapshot.GPUs[0])
	}
}

func TestCollectThermalEmptyWhenNoSensors(t *testing.T) {
	sys := t.TempDir()
	snapshot := model.Snapshot{}
	if err := (&Collector{sysRoot: sys}).collectThermal(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Thermal.Zones) != 0 || len(snapshot.Thermal.Sensors) != 0 || len(snapshot.Thermal.Fans) != 0 || len(snapshot.Thermal.Power) != 0 {
		t.Fatalf("expected empty thermal, got %#v", snapshot.Thermal)
	}
}

func TestEthtoolCacheReusedOnLightSnapshots(t *testing.T) {
	sys := t.TempDir()
	base := filepath.Join(sys, "class/net/eth0")
	for _, key := range []string{"rx_bytes", "tx_bytes", "rx_packets", "tx_packets", "rx_errors", "tx_errors", "rx_dropped", "tx_dropped"} {
		writeFixture(t, filepath.Join(base, "statistics"), key, "1\n")
	}
	writeFixture(t, base, "operstate", "up\n")
	writeFixture(t, base, "speed", "1000\n")
	writeFixture(t, base, "mtu", "1500\n")
	if err := os.MkdirAll(filepath.Join(sys, "devices/pci0000:00/0000:00:03.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(sys, "devices/pci0000:00/0000:00:03.0"), filepath.Join(base, "device")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sys, "class/net/veth0"), 0o755); err != nil {
		t.Fatal(err)
	}
	collector := &Collector{sysRoot: sys}
	first := model.Snapshot{Networks: []model.Network{}}
	if err := collector.collectNetworks(&first, &rawCounters{networks: map[string]networkCounter{}}, true); err != nil {
		t.Fatal(err)
	}
	cached := collector.ethtoolCache
	if cached == nil {
		t.Fatal("expected ethtool cache to be populated")
	}
	second := model.Snapshot{Networks: []model.Network{}}
	if err := collector.collectNetworks(&second, &rawCounters{networks: map[string]networkCounter{}}, false); err != nil {
		t.Fatal(err)
	}
	if len(collector.ethtoolCache) != len(cached) {
		t.Fatal("light snapshot must not refresh the ethtool cache")
	}
	if len(first.Networks) != 1 || len(second.Networks) != 1 {
		t.Fatalf("unexpected network counts: %d/%d", len(first.Networks), len(second.Networks))
	}
}

func TestCollectTopProcessesRates(t *testing.T) {
	proc := t.TempDir()
	writeFixture(t, filepath.Join(proc, "1"), "comm", "init\n")
	writeFixture(t, filepath.Join(proc, "1"), "stat", "1 (init) S 0 1 1 0 -1 4194560 100 0 0 0 1 1 0 0 20 0 1 0 1 0 0\n")
	writeFixture(t, filepath.Join(proc, "1"), "statm", "100 50 0 1 0 3 0\n")
	writeFixture(t, filepath.Join(proc, "2"), "comm", "worker\n")
	writeFixture(t, filepath.Join(proc, "2"), "stat", "2 (worker) S 1 2 2 0 -1 4194560 100 0 0 0 11 11 0 0 20 0 1 0 2 0 0\n")
	writeFixture(t, filepath.Join(proc, "2"), "statm", "200 100 0 1 0 3 0\n")
	collector := &Collector{procRoot: proc}
	raw := &rawCounters{processes: map[int]uint64{}}
	snapshot := model.Snapshot{TopProcesses: []model.ProcessSample{}}
	if err := collector.collectTopProcesses(&snapshot, raw, 0); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.TopProcesses) != 2 || snapshot.TopProcesses[0].CPUPercent != 0 || snapshot.TopProcesses[0].RSSBytes == 0 {
		t.Fatalf("unexpected first sample: %#v", snapshot.TopProcesses)
	}
	writeFixture(t, filepath.Join(proc, "2"), "stat", "2 (worker) S 1 2 2 0 -1 4194560 100 0 0 0 21 21 0 0 20 0 1 0 3 0 0\n")
	if err := collector.collectTopProcesses(&snapshot, raw, 1); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.TopProcesses) != 2 {
		t.Fatalf("unexpected second sample count: %#v", snapshot.TopProcesses)
	}
	worker := snapshot.TopProcesses[0]
	if worker.PID != 2 || worker.CPUPercent < 19 || worker.CPUPercent > 23 || worker.Name != "worker" || worker.State != "S" {
		t.Fatalf("unexpected top consumer: %#v", worker)
	}
}

func BenchmarkParseProcessJiffies(b *testing.B) {
	value := "1234 (qemu-system-x86_64) S 0 1 1 0 -1 4194560 100 0 0 0 999 888 0 0 20 0 1 0 12345 0 0"
	for i := 0; i < b.N; i++ {
		parseProcessJiffies(value)
	}
}

func BenchmarkParseProcessIO(b *testing.B) {
	value := "rchar: 123456\nwchar: 654321\nsyscr: 10\nsyscw: 20\nread_bytes: 1000\nwrite_bytes: 2000\ncancelled_write_bytes: 0\n"
	for i := 0; i < b.N; i++ {
		parseProcessIO(value)
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

func writeFixtureBytes(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
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

func TestCollectNodeHugepages(t *testing.T) {
	dir := t.TempDir()
	nodeDir := filepath.Join(dir, "node1")
	hugeDir := filepath.Join(nodeDir, "hugepages", "hugepages-2048kB")
	if err := os.MkdirAll(hugeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hugeDir, "nr_hugepages"), []byte("16\n"), 0o644); err != nil {
		t.Fatalf("write nr: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hugeDir, "free_hugepages"), []byte("4\n"), 0o644); err != nil {
		t.Fatalf("write free: %v", err)
	}
	result := collectNodeHugepages(nodeDir)
	if len(result) != 1 {
		t.Fatalf("expected 1 hugepage pool entry, got %d", len(result))
	}
	entry := result[0]
	if entry.Node != 1 || entry.SizeBytes != 2048*1024 || entry.Total != 16 || entry.Free != 4 {
		t.Errorf("unexpected node hugepage entry: %#v", entry)
	}
}

package collect

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"hardware-resources-tool/internal/model"
)

type Collector struct {
	procRoot      string
	sysRoot       string
	etcRoot       string
	logRoot       string
	prev          *rawCounters
	prevAt        time.Time
	hardware      model.Snapshot
	hardwareReady bool
	snapshotCount int
	heavyInterval int
	ethtoolCache  map[string]ethtoolData
	vmHeavy       map[int]vmHeavyTelemetry
}

// defaultHeavyInterval is the number of snapshots between heavy per-snapshot
// collections (QMP, ethtool netlink, and per-VM runtime memory maps). The
// first snapshot is always heavy so a fresh capture is complete.
const defaultHeavyInterval = 5

type rawCounters struct {
	cpuUser, cpuNice, cpuSystem, cpuIOWait, cpuIdle uint64
	cpuIRQ, cpuSoftIRQ, cpuSteal                    uint64
	contextSwitches, interrupts                     uint64
	swapIn, swapOut                                 uint64
	numaRemote                                      uint64
	disks                                           map[string]diskCounter
	networks                                        map[string]networkCounter
	virtualProcesses                                map[int]virtualProcessCounter
	processes                                       map[int]uint64
}

type diskCounter struct{ reads, sectorsRead, writes, sectorsWritten, inFlight uint64 }
type networkCounter struct{ rxBytes, txBytes, rxPackets, txPackets, rxErrors, txErrors, rxDrops, txDrops uint64 }

func New() *Collector {
	return &Collector{procRoot: "/proc", sysRoot: "/sys", etcRoot: "/etc", logRoot: "/var/log", heavyInterval: defaultHeavyInterval, ethtoolCache: map[string]ethtoolData{}, vmHeavy: map[int]vmHeavyTelemetry{}}
}

func (c *Collector) heavyDue() bool {
	if c.heavyInterval <= 0 {
		return true
	}
	return c.snapshotCount%c.heavyInterval == 0
}

func (c *Collector) Snapshot() model.Snapshot {
	now := time.Now().UTC()
	snapshot := model.Snapshot{CollectedAt: now, Disks: []model.Disk{}, Filesystems: []model.Filesystem{}, Networks: []model.Network{}, PCI: []model.PCIDevice{}, MemoryDevices: []model.MemoryDevice{}, GPUs: []model.GPU{}, Errors: []string{}}
	current := rawCounters{disks: map[string]diskCounter{}, networks: map[string]networkCounter{}, virtualProcesses: map[int]virtualProcessCounter{}, processes: map[int]uint64{}}
	heavy := c.heavyDue()
	c.snapshotCount++

	if err := c.collectCPU(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectMemory(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectDisks(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectNetworks(&snapshot, &current, heavy); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectFilesystems(&snapshot); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectSystem(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectThermal(&snapshot); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectVirtualization(&snapshot, &current, now.Sub(c.prevAt).Seconds(), heavy); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectTopProcesses(&snapshot, &current, now.Sub(c.prevAt).Seconds()); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if !c.hardwareReady {
		if err := c.collectHardware(&c.hardware); err != nil {
			snapshot.Errors = append(snapshot.Errors, err.Error())
		}
		c.hardwareReady = true
	}
	snapshot.PCI = append(snapshot.PCI, c.hardware.PCI...)
	snapshot.MemoryDevices = append(snapshot.MemoryDevices, c.hardware.MemoryDevices...)
	snapshot.GPUs = append(snapshot.GPUs, c.hardware.GPUs...)
	collectGPUTelemetry(&snapshot)
	correlateGPUThermal(&snapshot)

	if c.prev != nil {
		seconds := now.Sub(c.prevAt).Seconds()
		if seconds > 0 {
			applyRates(&snapshot, c.prev, &current, seconds)
		}
	}
	c.prev, c.prevAt = &current, now
	return snapshot
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func parseUint(value string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(value), 10, 64)
}
func parseInt(value string) (int64, error) { return strconv.ParseInt(strings.TrimSpace(value), 10, 64) }

func (c *Collector) collectCPU(s *model.Snapshot, raw *rawCounters) error {
	lines, err := readLines(filepath.Join(c.procRoot, "stat"))
	if err != nil {
		return fmt.Errorf("cpu: %w", err)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		// user, nice, system, idle, iowait, irq, softirq, steal
		if len(fields) < 9 {
			return fmt.Errorf("cpu: expected at least 8 counters")
		}
		values := make([]uint64, 8)
		for i := range values {
			values[i], err = parseUint(fields[i+1])
			if err != nil {
				return fmt.Errorf("cpu counter: %w", err)
			}
		}
		raw.cpuUser, raw.cpuNice, raw.cpuSystem = values[0], values[1], values[2]
		raw.cpuIdle, raw.cpuIOWait = values[3], values[4]
		raw.cpuIRQ, raw.cpuSoftIRQ, raw.cpuSteal = values[5], values[6], values[7]
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "ctxt":
			raw.contextSwitches, _ = parseUint(fields[1])
		case "intr":
			raw.interrupts, _ = parseUint(fields[1])
		}
	}
	load, err := readLines(filepath.Join(c.procRoot, "loadavg"))
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}
	if len(load) > 0 {
		parts := strings.Fields(load[0])
		if len(parts) >= 3 {
			s.CPU.Load1, _ = strconv.ParseFloat(parts[0], 64)
			s.CPU.Load5, _ = strconv.ParseFloat(parts[1], 64)
			s.CPU.Load15, _ = strconv.ParseFloat(parts[2], 64)
		}
	}
	s.CPU.LogicalCPUs = int64(runtimeCPUCount())
	return nil
}

func (c *Collector) collectMemory(s *model.Snapshot, raw *rawCounters) error {
	lines, err := readLines(filepath.Join(c.procRoot, "meminfo"))
	if err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	values := map[string]uint64{}
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			value, parseErr := parseUint(parts[1])
			if parseErr == nil {
				values[strings.TrimSuffix(parts[0], ":")] = value * 1024
			}
		}
	}
	s.Memory.TotalBytes = values["MemTotal"]
	s.Memory.AvailableBytes = values["MemAvailable"]
	s.Memory.SwapTotalBytes = values["SwapTotal"]
	s.Memory.SwapFreeBytes = values["SwapFree"]
	s.Memory.HugepagesTotal = values["HugePages_Total"]
	s.Memory.HugepagesFree = values["HugePages_Free"]
	s.Memory.HugepageSizeBytes = values["Hugepagesize"]
	s.Memory.HugetlbUsedBytes = values["Hugetlb"]
	if s.Memory.TotalBytes > 0 {
		s.Memory.UsedPercent = float64(s.Memory.TotalBytes-s.Memory.AvailableBytes) / float64(s.Memory.TotalBytes) * 100
	}
	if vmstat, vmstatErr := readLines(filepath.Join(c.procRoot, "vmstat")); vmstatErr == nil {
		for _, line := range vmstat {
			parts := strings.Fields(line)
			if len(parts) != 2 {
				continue
			}
			switch parts[0] {
			case "pswpin":
				raw.swapIn, _ = parseUint(parts[1])
			case "pswpout":
				raw.swapOut, _ = parseUint(parts[1])
			}
		}
	}
	return nil
}

// collectNodeHugepages reads the per-node hugetlb pool state from sysfs. The
// node directory looks like .../node1/hugepages/hugepages-2048kB/ with
// nr_hugepages and free_hugepages accounting files.
func collectNodeHugepages(nodeDir string) []model.NUMANodeHugepages {
	base := filepath.Base(nodeDir)
	node, err := strconv.Atoi(strings.TrimPrefix(base, "node"))
	if err != nil {
		return nil
	}
	var result []model.NUMANodeHugepages
	for _, dir := range glob(filepath.Join(nodeDir, "hugepages/hugepages-*kB")) {
		sizePart := strings.TrimSuffix(filepath.Base(dir), "kB")
		sizePart = strings.TrimPrefix(sizePart, "hugepages-")
		sizeKB, err := strconv.ParseUint(sizePart, 10, 64)
		if err != nil {
			continue
		}
		entry := model.NUMANodeHugepages{Node: node, SizeBytes: sizeKB * 1024}
		if data, err := os.ReadFile(filepath.Join(dir, "nr_hugepages")); err == nil {
			entry.Total, _ = parseUint(strings.TrimSpace(string(data)))
		}
		if data, err := os.ReadFile(filepath.Join(dir, "free_hugepages")); err == nil {
			entry.Free, _ = parseUint(strings.TrimSpace(string(data)))
		}
		result = append(result, entry)
	}
	return result
}

func (c *Collector) collectDisks(s *model.Snapshot, raw *rawCounters) error {
	lines, err := readLines(filepath.Join(c.procRoot, "diskstats"))
	if err != nil {
		return fmt.Errorf("disks: %w", err)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		values := make([]uint64, 6)
		valid := true
		for i, index := range []int{3, 5, 7, 9, 11, 13} {
			values[i], err = parseUint(fields[index])
			if err != nil {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		raw.disks[name] = diskCounter{reads: values[0], sectorsRead: values[1], writes: values[2], sectorsWritten: values[3], inFlight: values[4]}
		s.Disks = append(s.Disks, model.Disk{Name: name, ReadBytes: values[1] * 512, WriteBytes: values[3] * 512, InFlight: int64(values[4])})
	}
	return nil
}

func (c *Collector) collectNetworks(s *model.Snapshot, raw *rawCounters, heavy bool) error {
	entries, err := os.ReadDir(filepath.Join(c.sysRoot, "class/net"))
	if err != nil {
		return fmt.Errorf("networks: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		physical, _ := networkDeviceInfo(c.sysRoot, name)
		if !physical {
			s.VirtualNetworkCount++
			continue
		}
		names = append(names, name)
	}
	ethtoolDataByName := c.ethToolData(names, heavy)
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		if _, ok := ethtoolDataByName[name]; !ok {
			continue
		}
		base := filepath.Join(c.sysRoot, "class/net", name, "statistics")
		values := map[string]uint64{}
		for _, key := range []string{"rx_bytes", "tx_bytes", "rx_packets", "tx_packets", "rx_errors", "tx_errors", "rx_dropped", "tx_dropped"} {
			data, readErr := os.ReadFile(filepath.Join(base, key))
			if readErr != nil {
				continue
			}
			values[key], _ = parseUint(string(data))
		}
		state, _ := os.ReadFile(filepath.Join(c.sysRoot, "class/net", name, "operstate"))
		linkSpeed, _ := readSysInt(filepath.Join(c.sysRoot, "class/net", name, "speed"))
		if linkSpeed < 0 {
			linkSpeed = 0
		}
		mtu, _ := readSysInt(filepath.Join(c.sysRoot, "class/net", name, "mtu"))
		rxQueues := int64(len(glob(filepath.Join(c.sysRoot, "class/net", name, "queues/rx-*"))))
		txQueues := int64(len(glob(filepath.Join(c.sysRoot, "class/net", name, "queues/tx-*"))))
		rxRing, txRing := networkRingSizes(name)
		driver := symlinkBase(filepath.Join(c.sysRoot, "class/net", name, "device/driver"))
		ethtoolData := ethtoolDataByName[name]
		_, pciAddress := networkDeviceInfo(c.sysRoot, name)
		raw.networks[name] = networkCounter{rxBytes: values["rx_bytes"], txBytes: values["tx_bytes"], rxPackets: values["rx_packets"], txPackets: values["tx_packets"], rxErrors: values["rx_errors"], txErrors: values["tx_errors"], rxDrops: values["rx_dropped"], txDrops: values["tx_dropped"]}
		s.Networks = append(s.Networks, model.Network{Name: name, State: strings.TrimSpace(string(state)), RXBytes: values["rx_bytes"], TXBytes: values["tx_bytes"], RXPackets: int64(values["rx_packets"]), TXPackets: int64(values["tx_packets"]), RXErrors: int64(values["rx_errors"]), TXErrors: int64(values["tx_errors"]), RXDrops: int64(values["rx_dropped"]), TXDrops: int64(values["tx_dropped"]), LinkSpeedMbps: linkSpeed, MTU: mtu, RXQueues: rxQueues, TXQueues: txQueues, RXRingSize: rxRing, TXRingSize: txRing, Driver: driver, DriverVersion: ethtoolData.DriverVersion, FWVersion: ethtoolData.FWVersion, BusInfo: ethtoolData.BusInfo, LinkPort: ethtoolData.LinkPort, Transceiver: ethtoolData.Transceiver, PHYAddress: ethtoolData.PHYAddress, TPMDIX: ethtoolData.TPMDIX, LinkDuplex: ethtoolData.Duplex, AutoNegotiation: ethtoolData.Autoneg, LinkUp: ethtoolData.LinkUp, SupportedLinkModes: ethtoolData.Supported, AdvertisedLinkModes: ethtoolData.Advertised, PeerLinkModes: ethtoolData.Peer, FECActive: ethtoolData.FECActive, FECSupported: ethtoolData.FECSupported, MaxRXChannels: ethtoolData.MaxRXChannels, MaxTXChannels: ethtoolData.MaxTXChannels, MaxCombinedChannels: ethtoolData.MaxCombinedChannels, PauseAutoneg: ethtoolData.PauseAutoneg, RXPause: ethtoolData.RXPause, TXPause: ethtoolData.TXPause, Timestamping: ethtoolData.Timestamping, PHCIndex: ethtoolData.PHCIndex, FeaturesActive: ethtoolData.Features.Active, FeaturesWanted: ethtoolData.Features.Wanted, FeaturesHardware: ethtoolData.Features.Hardware, FeaturesNoChange: ethtoolData.Features.NoChange, CoalesceRXUsecs: ethtoolData.CoalesceRXUsecs, CoalesceTXUsecs: ethtoolData.CoalesceTXUsecs, CoalesceRXMaxFrames: ethtoolData.CoalesceRXMaxFrames, CoalesceTXMaxFrames: ethtoolData.CoalesceTXMaxFrames, CoalesceAdaptiveRX: ethtoolData.CoalesceAdaptiveRX, CoalesceAdaptiveTX: ethtoolData.CoalesceAdaptiveTX, RSSHashFunc: ethtoolData.RSSHashFunc, RSSIndirSize: ethtoolData.RSSIndirSize, RSSKeySize: ethtoolData.RSSKeySize, EthtoolError: ethtoolData.Error, Physical: true, PCIAddress: pciAddress})
	}
	return nil
}

func (c *Collector) ethToolData(names []string, heavy bool) map[string]ethtoolData {
	if heavy || c.ethtoolCache == nil {
		c.ethtoolCache = enrichNetworks(names)
	}
	return c.ethtoolCache
}

func (c *Collector) collectFilesystems(s *model.Snapshot) error {
	lines, err := readLines(filepath.Join(c.procRoot, "mounts"))
	if err != nil {
		return fmt.Errorf("filesystems: %w", err)
	}
	etcRoot := c.etcRoot
	if etcRoot == "" {
		etcRoot = "/etc"
	}
	sysRoot := c.sysRoot
	if sysRoot == "" {
		sysRoot = "/sys"
	}
	fstabNetworks := readFstabNetworkMounts(filepath.Join(etcRoot, "fstab"))
	seen := map[string]bool{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mountPoint := decodeMountField(fields[1])
		fsType := fields[2]
		if isPseudoFilesystem(fsType) || excludedFilesystemMount(mountPoint) {
			continue
		}
		source := decodeMountField(fields[0])
		if isNetworkFilesystem(fsType) {
			if !fstabNetworks[mountPoint] {
				continue
			}
		} else {
			physical, usb := isPhysicalBlockDevice(sysRoot, source)
			if !physical || usb {
				continue
			}
		}
		if seen[mountPoint] {
			continue
		}
		seen[mountPoint] = true
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &stat); err != nil {
			continue
		}
		total := uint64(stat.Blocks) * uint64(stat.Bsize)
		available := uint64(stat.Bavail) * uint64(stat.Bsize)
		used := float64(0)
		if total > 0 && available <= total {
			used = float64(total-available) / float64(total) * 100
		}
		readOnly := false
		for _, option := range strings.Split(fields[3], ",") {
			if option == "ro" {
				readOnly = true
				break
			}
		}
		s.Filesystems = append(s.Filesystems, model.Filesystem{Device: source, MountPoint: mountPoint, Type: fsType, TotalBytes: total, AvailableBytes: available, UsedPercent: used, ReadOnly: readOnly})
	}
	return nil
}

func readFstabNetworkMounts(path string) map[string]bool {
	result := map[string]bool{}
	lines, err := readLines(path)
	if err != nil {
		return result
	}
	for _, line := range lines {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		fields := strings.Fields(line)
		if len(fields) >= 4 && isNetworkFilesystem(fields[2]) {
			result[decodeMountField(fields[1])] = true
		}
	}
	return result
}

func (c *Collector) collectSystem(s *model.Snapshot, raw *rawCounters) error {
	var errs []string
	if data, err := os.ReadFile(filepath.Join(c.sysRoot, "kernel/mm/transparent_hugepage/enabled")); err == nil {
		s.System.THP = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile(filepath.Join(c.sysRoot, "vm/swappiness")); err == nil {
		s.System.Swappiness, _ = parseInt(string(data))
	}
	s.System.Sysctls = map[string]string{}
	for name, path := range map[string]string{
		"vm.overcommit_memory":      "vm/overcommit_memory",
		"vm.dirty_ratio":            "vm/dirty_ratio",
		"vm.dirty_background_ratio": "vm/dirty_background_ratio",
		"kernel.nmi_watchdog":       "kernel/nmi_watchdog",
	} {
		if data, err := os.ReadFile(filepath.Join(c.procRoot, "sys", path)); err == nil {
			s.System.Sysctls[name] = strings.TrimSpace(string(data))
		}
	}
	if len(s.System.Sysctls) == 0 {
		s.System.Sysctls = nil
	}
	s.System.KernelEvents = collectKernelEvents(c.logRoot)
	governors := map[string]bool{}
	for _, path := range glob(filepath.Join(c.sysRoot, "devices/system/cpu/cpu*/cpufreq/scaling_governor")) {
		if data, err := os.ReadFile(path); err == nil {
			governors[strings.TrimSpace(string(data))] = true
		}
	}
	if len(governors) > 0 {
		values := make([]string, 0, len(governors))
		for governor := range governors {
			values = append(values, governor)
		}
		s.System.CPUGovernor = strings.Join(values, ",")
	}
	s.NUMA.Nodes = len(glob(filepath.Join(c.sysRoot, "devices/system/node/node[0-9]*")))
	for _, node := range glob(filepath.Join(c.sysRoot, "devices/system/node/node[0-9]*")) {
		if data, err := os.ReadFile(filepath.Join(node, "numastat")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && (fields[0] == "numa_miss" || fields[0] == "numa_foreign") {
					value, parseErr := parseUint(fields[1])
					if parseErr == nil {
						raw.numaRemote += value
					}
				}
			}
		}
		s.NUMA.NodeHugepages = append(s.NUMA.NodeHugepages, collectNodeHugepages(node)...)
	}
	if limits, err := readLimits("/proc/self/limits"); err == nil {
		s.System.OpenFiles, s.System.MaxLocked, s.System.MaxProcesses, s.System.MaxStack = limits.OpenFiles, limits.MaxLocked, limits.MaxProcesses, limits.MaxStack
	} else {
		errs = append(errs, err.Error())
	}
	if limits, err := readLimits("/proc/1/limits"); err == nil {
		s.System.HostLimits = limits
	}
	if len(errs) > 0 {
		return errors.Join(errors.New("system settings"), errors.Join(toErrors(errs)...))
	}
	return nil
}

const maxLogTailBytes = 256 * 1024

func collectKernelEvents(logRoot string) model.KernelEvents {
	if logRoot == "" {
		logRoot = "/var/log"
	}
	result := model.KernelEvents{}
	seen := map[string]bool{}
	for _, path := range []string{filepath.Join(logRoot, "kern.log"), filepath.Join(logRoot, "syslog"), filepath.Join(logRoot, "messages")} {
		data, err := readLogTail(path, maxLogTailBytes)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			categories := classifyKernelEvents(line)
			if len(categories) == 0 {
				continue
			}
			for _, category := range categories {
				switch category {
				case "oom":
					result.OOM++
				case "io":
					result.IOErrors++
				case "pcie":
					result.PCIeErrors++
				case "hardware":
					result.Hardware++
				case "nvidia":
					result.NVIDIA++
				case "storage":
					result.StorageResets++
				case "link":
					result.LinkFailures++
				}
			}
			if len(result.Recent) < 12 {
				if len(line) > 240 {
					line = line[:240]
				}
				result.Recent = append(result.Recent, line)
			}
		}
	}
	if len(result.Recent) == 0 {
		result.Recent = nil
	}
	return result
}

func readLogTail(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	start := int64(0)
	if info.Size() > limit {
		start = info.Size() - limit
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(file, limit))
}

func classifyKernelEvents(line string) []string {
	value := strings.ToLower(line)
	categories := []string{}
	if strings.Contains(value, "out of memory") || strings.Contains(value, "oom-kill") || strings.Contains(value, "oom killer") || strings.Contains(value, "killed process") {
		categories = append(categories, "oom")
	}
	if strings.Contains(value, "aer:") || strings.Contains(value, "pcie bus error") || strings.Contains(value, "uncorrected error received") {
		categories = append(categories, "pcie")
	}
	if strings.Contains(value, "nvrm: xid") || strings.Contains(value, "nvidia") && strings.Contains(value, "xid") {
		categories = append(categories, "nvidia")
	}
	if strings.Contains(value, "machine check") || strings.Contains(value, "mce:") || strings.Contains(value, "edac") || strings.Contains(value, "hardware error") {
		categories = append(categories, "hardware")
	}
	if strings.Contains(value, "nvme") && (strings.Contains(value, "reset") || strings.Contains(value, "timeout")) || strings.Contains(value, "reset controller") || strings.Contains(value, "ata") && strings.Contains(value, "reset") {
		categories = append(categories, "storage")
	}
	if strings.Contains(value, "i/o error") || strings.Contains(value, "blk_update_request") || strings.Contains(value, "buffer i/o error") {
		categories = append(categories, "io")
	}
	if strings.Contains(value, "link is down") || strings.Contains(value, "link down") || strings.Contains(value, "carrier lost") {
		categories = append(categories, "link")
	}
	return categories
}

func readLimits(path string) (model.Limits, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Limits{}, err
	}
	var limits model.Limits
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] == "unlimited" {
			continue
		}
		value, parseErr := parseUint(fields[3])
		if parseErr != nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Max open files"):
			limits.OpenFiles = value
		case strings.HasPrefix(line, "Max locked memory"):
			limits.MaxLocked = value * 1024
		case strings.HasPrefix(line, "Max processes"):
			limits.MaxProcesses = value
		case strings.HasPrefix(line, "Max stack size"):
			limits.MaxStack = value * 1024
		}
	}
	return limits, nil
}

func readSysInt(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return parseInt(string(data))
}

func glob(pattern string) []string {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	return paths
}

func decodeMountField(value string) string {
	value = strings.ReplaceAll(value, `\040`, " ")
	value = strings.ReplaceAll(value, `\011`, "\t")
	return strings.ReplaceAll(value, `\134`, `\`)
}

func toErrors(messages []string) []error {
	result := make([]error, 0, len(messages))
	for _, message := range messages {
		result = append(result, errors.New(message))
	}
	return result
}

func runtimeCPUCount() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "processor\t:")
}

func applyRates(s *model.Snapshot, previous *rawCounters, current *rawCounters, seconds float64) {
	user := delta(current.cpuUser, previous.cpuUser)
	nice := delta(current.cpuNice, previous.cpuNice)
	system := delta(current.cpuSystem, previous.cpuSystem)
	iowait := delta(current.cpuIOWait, previous.cpuIOWait)
	idle := delta(current.cpuIdle, previous.cpuIdle)
	irq := delta(current.cpuIRQ, previous.cpuIRQ)
	softIRQ := delta(current.cpuSoftIRQ, previous.cpuSoftIRQ)
	steal := delta(current.cpuSteal, previous.cpuSteal)
	total := float64(user + nice + system + iowait + idle + irq + softIRQ + steal)
	if total > 0 {
		s.CPU.UserPercent = float64(user) / total * 100
		s.CPU.SystemPercent = float64(system+irq+softIRQ) / total * 100
		s.CPU.IOWaitPercent = float64(iowait) / total * 100
		s.CPU.IdlePercent = float64(idle) / total * 100
		s.CPU.ContextSwitch = int64(float64(delta(current.contextSwitches, previous.contextSwitches)) / seconds)
		s.CPU.Interrupts = int64(float64(delta(current.interrupts, previous.interrupts)) / seconds)
	}
	swapIn := delta(current.swapIn, previous.swapIn)
	swapOut := delta(current.swapOut, previous.swapOut)
	s.Memory.SwapInPerSec, s.Memory.SwapOutPerSec = int64(float64(swapIn)/seconds), int64(float64(swapOut)/seconds)
	s.NUMA.RemoteEvents = int64(float64(delta(current.numaRemote, previous.numaRemote)) / seconds)
	for i := range s.Disks {
		currentDisk, ok := current.disks[s.Disks[i].Name]
		previousDisk := previous.disks[s.Disks[i].Name]
		if !ok {
			continue
		}
		s.Disks[i].ReadsPerSec = float64(delta(currentDisk.reads, previousDisk.reads)) / seconds
		s.Disks[i].WritesPerSec = float64(delta(currentDisk.writes, previousDisk.writes)) / seconds
		s.Disks[i].ReadBytesPerSec = float64(delta(currentDisk.sectorsRead, previousDisk.sectorsRead)*512) / seconds
		s.Disks[i].WriteBytesPerSec = float64(delta(currentDisk.sectorsWritten, previousDisk.sectorsWritten)*512) / seconds
	}
	for i := range s.Networks {
		currentNet, ok := current.networks[s.Networks[i].Name]
		previousNet := previous.networks[s.Networks[i].Name]
		if !ok {
			continue
		}
		s.Networks[i].RXBytesPerSec = float64(delta(currentNet.rxBytes, previousNet.rxBytes)) / seconds
		s.Networks[i].TXBytesPerSec = float64(delta(currentNet.txBytes, previousNet.txBytes)) / seconds
	}
}

func delta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

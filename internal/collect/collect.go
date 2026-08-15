package collect

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hardware-resources-tool/internal/model"
)

type Collector struct {
	procRoot string
	sysRoot  string
	prev     *rawCounters
	prevAt   time.Time
}

type rawCounters struct {
	cpuUser, cpuSystem, cpuIOWait, cpuIdle uint64
	contextSwitches, interrupts            uint64
	swapIn, swapOut                        uint64
	disks                                  map[string]diskCounter
	networks                               map[string]networkCounter
}

type diskCounter struct{ reads, sectorsRead, writes, sectorsWritten, inFlight uint64 }
type networkCounter struct{ rxBytes, txBytes, rxPackets, txPackets, rxErrors, txErrors, rxDrops, txDrops uint64 }

func New() *Collector {
	return &Collector{procRoot: "/proc", sysRoot: "/sys"}
}

func (c *Collector) Snapshot() model.Snapshot {
	now := time.Now().UTC()
	snapshot := model.Snapshot{CollectedAt: now, Disks: []model.Disk{}, Networks: []model.Network{}, Errors: []string{}}
	current := rawCounters{disks: map[string]diskCounter{}, networks: map[string]networkCounter{}}

	if err := c.collectCPU(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectMemory(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectDisks(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectNetworks(&snapshot, &current); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}
	if err := c.collectSystem(&snapshot); err != nil {
		snapshot.Errors = append(snapshot.Errors, err.Error())
	}

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
		values := make([]uint64, 5)
		for i := range values {
			values[i], err = parseUint(fields[i+1])
			if err != nil {
				return fmt.Errorf("cpu counter: %w", err)
			}
		}
		raw.cpuUser, raw.cpuSystem, raw.cpuIdle, raw.cpuIOWait = values[0], values[2], values[3], values[4]
		break
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
	if s.Memory.TotalBytes > 0 {
		s.Memory.UsedPercent = float64(s.Memory.TotalBytes-s.Memory.AvailableBytes) / float64(s.Memory.TotalBytes) * 100
	}
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == "pswpin" {
			raw.swapIn, _ = parseUint(parts[1])
		}
		if len(parts) == 2 && parts[0] == "pswpout" {
			raw.swapOut, _ = parseUint(parts[1])
		}
	}
	return nil
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

func (c *Collector) collectNetworks(s *model.Snapshot, raw *rawCounters) error {
	entries, err := os.ReadDir(filepath.Join(c.sysRoot, "class/net"))
	if err != nil {
		return fmt.Errorf("networks: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
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
		raw.networks[name] = networkCounter{rxBytes: values["rx_bytes"], txBytes: values["tx_bytes"], rxPackets: values["rx_packets"], txPackets: values["tx_packets"], rxErrors: values["rx_errors"], txErrors: values["tx_errors"], rxDrops: values["rx_dropped"], txDrops: values["tx_dropped"]}
		s.Networks = append(s.Networks, model.Network{Name: name, State: strings.TrimSpace(string(state)), RXBytes: values["rx_bytes"], TXBytes: values["tx_bytes"], RXPackets: int64(values["rx_packets"]), TXPackets: int64(values["tx_packets"]), RXErrors: int64(values["rx_errors"]), TXErrors: int64(values["tx_errors"]), RXDrops: int64(values["rx_dropped"]), TXDrops: int64(values["tx_dropped"])})
	}
	return nil
}

func (c *Collector) collectSystem(s *model.Snapshot) error {
	var errs []string
	if data, err := os.ReadFile(filepath.Join(c.sysRoot, "kernel/mm/transparent_hugepage/enabled")); err == nil {
		s.System.THP = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile(filepath.Join(c.sysRoot, "vm/swappiness")); err == nil {
		s.System.Swappiness, _ = parseInt(string(data))
	}
	if data, err := os.ReadFile("/proc/self/limits"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "Max open files") {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					s.System.OpenFiles, _ = parseUint(fields[3])
				}
			}
			if strings.HasPrefix(line, "Max locked memory") {
				fields := strings.Fields(line)
				if len(fields) >= 4 && fields[3] != "unlimited" {
					s.System.MaxLocked, _ = parseUint(fields[3])
					s.System.MaxLocked *= 1024
				}
			}
		}
	} else {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return errors.Join(errors.New("system settings"), errors.Join(toErrors(errs)...))
	}
	return nil
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
	total := float64((current.cpuUser + current.cpuSystem + current.cpuIOWait + current.cpuIdle) - (previous.cpuUser + previous.cpuSystem + previous.cpuIOWait + previous.cpuIdle))
	if total > 0 {
		s.CPU.UserPercent = float64(current.cpuUser-previous.cpuUser) / total * 100
		s.CPU.SystemPercent = float64(current.cpuSystem-previous.cpuSystem) / total * 100
		s.CPU.IOWaitPercent = float64(current.cpuIOWait-previous.cpuIOWait) / total * 100
		s.CPU.IdlePercent = float64(current.cpuIdle-previous.cpuIdle) / total * 100
	}
	swapIn := delta(current.swapIn, previous.swapIn)
	swapOut := delta(current.swapOut, previous.swapOut)
	s.Memory.SwapInPerSec, s.Memory.SwapOutPerSec = int64(float64(swapIn)/seconds), int64(float64(swapOut)/seconds)
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

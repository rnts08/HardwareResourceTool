package collect

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
)

type libvirtDomainXML struct {
	Name    string           `xml:"name"`
	Memory  memoryXML        `xml:"memory"`
	VCPU    string           `xml:"vcpu"`
	Devices domainDevicesXML `xml:"devices"`
}

type memoryXML struct {
	Unit  string `xml:"unit,attr"`
	Value uint64 `xml:",chardata"`
}

type qemuProcess struct {
	Name       string
	PID        int
	VCPU       int64
	Memory     uint64
	RSS        uint64
	CPUJiffies uint64
	ReadBytes  uint64
	WriteBytes uint64
}

type virtualProcessCounter struct{ cpuJiffies, readBytes, writeBytes uint64 }

type domainDevicesXML struct {
	Disks    []domainDiskXML    `xml:"disk"`
	NICs     []domainNICXML     `xml:"interface"`
	Hostdevs []domainHostdevXML `xml:"hostdev"`
}

type domainDiskXML struct {
	Device string `xml:"device,attr"`
	Source struct {
		File string `xml:"file,attr"`
		Dev  string `xml:"dev,attr"`
	} `xml:"source"`
	Target struct {
		Dev string `xml:"dev,attr"`
		Bus string `xml:"bus,attr"`
	} `xml:"target"`
}

type domainNICXML struct {
	Type string `xml:"type,attr"`
	MAC  struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
	Source struct {
		Bridge  string `xml:"bridge,attr"`
		Network string `xml:"network,attr"`
		Dev     string `xml:"dev,attr"`
	} `xml:"source"`
	Target struct {
		Dev string `xml:"dev,attr"`
	} `xml:"target"`
}

type domainHostdevXML struct {
	Type   string `xml:"type,attr"`
	Source struct {
		Address struct {
			Domain   string `xml:"domain,attr"`
			Bus      string `xml:"bus,attr"`
			Slot     string `xml:"slot,attr"`
			Function string `xml:"function,attr"`
		} `xml:"address"`
	} `xml:"source"`
}

func (c *Collector) collectVirtualization(s *model.Snapshot, raw *rawCounters, seconds float64) error {
	virt := model.Virtualization{VirtualMachines: []model.VirtualMachine{}}
	if _, err := os.Stat(filepath.Join(c.sysRoot, "module/kvm")); err == nil {
		virt.KVMAvailable = true
	} else if _, err := os.Stat("/dev/kvm"); err == nil {
		virt.KVMAvailable = true
	}
	root := c.etcRoot
	if root == "" {
		root = "/etc"
	}
	for _, path := range glob(filepath.Join(root, "libvirt/qemu/*.xml")) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var domain libvirtDomainXML
		if err := xml.Unmarshal(data, &domain); err != nil || domain.Name == "" {
			continue
		}
		vcpus, _ := strconv.ParseInt(strings.TrimSpace(domain.VCPU), 10, 64)
		vm := model.VirtualMachine{Name: domain.Name, ConfiguredVCPUs: vcpus, ConfiguredMemoryBytes: memoryBytes(domain.Memory), Source: "libvirt", Disks: []model.VirtualDisk{}, NICs: []model.VirtualNIC{}, PCIAddresses: []string{}}
		for _, disk := range domain.Devices.Disks {
			if disk.Device == "cdrom" || disk.Device == "floppy" {
				continue
			}
			source := disk.Source.File
			if source == "" {
				source = disk.Source.Dev
			}
			vm.Disks = append(vm.Disks, model.VirtualDisk{Target: disk.Target.Dev, Source: source, Bus: disk.Target.Bus})
		}
		for _, nic := range domain.Devices.NICs {
			source := nic.Source.Bridge
			if source == "" {
				source = nic.Source.Network
			}
			if source == "" {
				source = nic.Source.Dev
			}
			vm.NICs = append(vm.NICs, model.VirtualNIC{Type: nic.Type, Source: source, Target: nic.Target.Dev, MAC: nic.MAC.Address})
		}
		for _, hostdev := range domain.Devices.Hostdevs {
			address := hostdev.Source.Address
			if address.Domain != "" && address.Bus != "" && address.Slot != "" && address.Function != "" {
				vm.PCIAddresses = append(vm.PCIAddresses, fmt.Sprintf("%s:%s:%s.%s", strings.TrimPrefix(address.Domain, "0x"), strings.TrimPrefix(address.Bus, "0x"), strings.TrimPrefix(address.Slot, "0x"), strings.TrimPrefix(address.Function, "0x")))
			}
		}
		virt.VirtualMachines = append(virt.VirtualMachines, vm)
	}
	if len(virt.VirtualMachines) > 0 {
		virt.Hypervisor = "kvm/libvirt"
	}
	processes, err := discoverQEMUProcesses(c.procRoot)
	if err != nil {
		return fmt.Errorf("virtualization: %w", err)
	}
	virt.QEMUDetected = len(processes) > 0
	if virt.QEMUDetected {
		virt.Hypervisor = "kvm/qemu"
	}
	for _, process := range processes {
		matched := false
		for i := range virt.VirtualMachines {
			if virt.VirtualMachines[i].Name == process.Name {
				virt.VirtualMachines[i].PID = process.PID
				virt.VirtualMachines[i].ProcessRSSBytes = process.RSS
				virt.VirtualMachines[i].Running = true
				virt.VirtualMachines[i].CPUPercent = processCPUPercent(process, raw, seconds, s.CPU.LogicalCPUs)
				virt.VirtualMachines[i].ReadBytes = process.ReadBytes
				virt.VirtualMachines[i].WriteBytes = process.WriteBytes
				virt.VirtualMachines[i].MemoryCurrentBytes, virt.VirtualMachines[i].MemoryMaxBytes, virt.VirtualMachines[i].CgroupPath, virt.VirtualMachines[i].CgroupAvailable = readQEMUCgroup(c.procRoot, c.sysRoot, process.PID)
				matched = true
				break
			}
		}
		if !matched {
			current, maximum, cgroupPath, cgroupAvailable := readQEMUCgroup(c.procRoot, c.sysRoot, process.PID)
			virt.VirtualMachines = append(virt.VirtualMachines, model.VirtualMachine{Name: process.Name, PID: process.PID, ConfiguredVCPUs: process.VCPU, ConfiguredMemoryBytes: process.Memory, ProcessRSSBytes: process.RSS, CPUPercent: processCPUPercent(process, raw, seconds, s.CPU.LogicalCPUs), ReadBytes: process.ReadBytes, WriteBytes: process.WriteBytes, Running: true, Source: "qemu-process", MemoryCurrentBytes: current, MemoryMaxBytes: maximum, CgroupPath: cgroupPath, CgroupAvailable: cgroupAvailable})
		}
	}
	for i := range virt.VirtualMachines {
		vm := &virt.VirtualMachines[i]
		if vm.PID > 0 {
			current, maximum, cgroupPath, available := readQEMUCgroup(c.procRoot, c.sysRoot, vm.PID)
			vm.MemoryCurrentBytes, vm.MemoryMaxBytes, vm.CgroupPath, vm.CgroupAvailable = current, maximum, cgroupPath, available
			if available {
				if readBytes, writeBytes := readQEMUCGIO(c.sysRoot, cgroupPath); readBytes > 0 || writeBytes > 0 {
					vm.ReadBytes, vm.WriteBytes = readBytes, writeBytes
				}
			}
		}
		for j := range vm.NICs {
			vm.NICs[j].HostNetwork = resolveVirtualNICHost(c.sysRoot, vm.NICs[j], s.Networks)
			for _, network := range s.Networks {
				if network.Name == vm.NICs[j].HostNetwork {
					vm.NICs[j].RXBytesPerSecond, vm.NICs[j].TXBytesPerSecond = network.RXBytesPerSec, network.TXBytesPerSec
				}
			}
		}
	}
	for _, vm := range virt.VirtualMachines {
		virt.AllocatedVCPUs += vm.ConfiguredVCPUs
		virt.AllocatedMemoryBytes += vm.ConfiguredMemoryBytes
	}
	if s.CPU.LogicalCPUs > 0 {
		virt.VCPUOvercommitRatio = float64(virt.AllocatedVCPUs) / float64(s.CPU.LogicalCPUs)
	}
	if s.Memory.TotalBytes > 0 {
		virt.MemoryOvercommitRatio = float64(virt.AllocatedMemoryBytes) / float64(s.Memory.TotalBytes)
	}
	s.Virtualization = virt
	return nil
}

func memoryBytes(memory memoryXML) uint64 {
	multiplier := uint64(1024 * 1024)
	switch strings.ToLower(strings.TrimSpace(memory.Unit)) {
	case "b", "bytes":
		multiplier = 1
	case "k", "kb", "kib":
		multiplier = 1024
	case "g", "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "t", "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	}
	return memory.Value * multiplier
}

func discoverQEMUProcesses(procRoot string) ([]qemuProcess, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	result := []qemuProcess{}
	pageSize := uint64(os.Getpagesize())
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(procRoot, entry.Name(), "comm"))
		if err != nil || !isQEMUExecutable(strings.TrimSpace(string(comm))) {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join(procRoot, entry.Name(), "cmdline"))
		if err != nil || len(cmdline) == 0 {
			continue
		}
		fields := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
		if len(fields) == 0 || !isQEMUExecutable(fields[0]) {
			continue
		}
		process := qemuProcess{PID: pid, Name: strings.Split(qemuArgument(fields, "-name"), ",")[0], VCPU: parseQEMUCPU(fields), Memory: parseQEMUMemory(fields)}
		if process.Name == "" {
			process.Name = fmt.Sprintf("qemu-%d", pid)
		}
		if statm, readErr := os.ReadFile(filepath.Join(procRoot, entry.Name(), "statm")); readErr == nil {
			values := strings.Fields(string(statm))
			if len(values) > 1 {
				pages, _ := strconv.ParseUint(values[1], 10, 64)
				process.RSS = pages * pageSize
			}
		}
		if stat, readErr := os.ReadFile(filepath.Join(procRoot, entry.Name(), "stat")); readErr == nil {
			process.CPUJiffies = parseProcessJiffies(string(stat))
		}
		if ioData, readErr := os.ReadFile(filepath.Join(procRoot, entry.Name(), "io")); readErr == nil {
			process.ReadBytes, process.WriteBytes = parseProcessIO(string(ioData))
		}
		result = append(result, process)
	}
	return result, nil
}

func parseProcessJiffies(value string) uint64 {
	closeParen := strings.LastIndex(value, ")")
	if closeParen < 0 {
		return 0
	}
	fields := strings.Fields(value[closeParen+1:])
	if len(fields) < 13 {
		return 0
	}
	user, _ := strconv.ParseUint(fields[11], 10, 64)
	system, _ := strconv.ParseUint(fields[12], 10, 64)
	return user + system
}

func parseProcessIO(value string) (uint64, uint64) {
	var readBytes, writeBytes uint64
	for _, line := range strings.Split(value, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		amount, _ := strconv.ParseUint(fields[1], 10, 64)
		switch strings.TrimSuffix(fields[0], ":") {
		case "read_bytes":
			readBytes = amount
		case "write_bytes":
			writeBytes = amount
		}
	}
	return readBytes, writeBytes
}

func processCPUPercent(process qemuProcess, raw *rawCounters, seconds float64, logicalCPUs int64) float64 {
	if raw.virtualProcesses == nil {
		raw.virtualProcesses = map[int]virtualProcessCounter{}
	}
	if seconds <= 0 || logicalCPUs <= 0 {
		raw.virtualProcesses[process.PID] = virtualProcessCounter{process.CPUJiffies, process.ReadBytes, process.WriteBytes}
		return 0
	}
	previous, ok := raw.virtualProcesses[process.PID]
	raw.virtualProcesses[process.PID] = virtualProcessCounter{process.CPUJiffies, process.ReadBytes, process.WriteBytes}
	if !ok || process.CPUJiffies < previous.cpuJiffies {
		return 0
	}
	return float64(process.CPUJiffies-previous.cpuJiffies) / (seconds * 100.0) * 100
}

func readQEMUCgroup(procRoot, sysRoot string, pid int) (uint64, uint64, string, bool) {
	cgroup, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return 0, 0, "", false
	}
	path := ""
	for _, line := range strings.Split(string(cgroup), "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) == 3 && fields[0] == "0" {
			path = fields[2]
			break
		}
	}
	if path == "" {
		return 0, 0, "", false
	}
	base := filepath.Join(sysRoot, "fs/cgroup", path)
	current, currentErr := readCgroupUint(filepath.Join(base, "memory.current"))
	max, maxErr := readCgroupUint(filepath.Join(base, "memory.max"))
	if currentErr != nil && maxErr != nil {
		return 0, 0, path, false
	}
	return current, max, path, true
}

func readQEMUCGIO(sysRoot, cgroupPath string) (uint64, uint64) {
	data, err := os.ReadFile(filepath.Join(sysRoot, "fs/cgroup", cgroupPath, "io.stat"))
	if err != nil {
		return 0, 0
	}
	var readBytes, writeBytes uint64
	for _, line := range strings.Split(string(data), "\n") {
		for _, field := range strings.Fields(line) {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) != 2 {
				continue
			}
			value, _ := strconv.ParseUint(parts[1], 10, 64)
			switch parts[0] {
			case "rbytes":
				readBytes += value
			case "wbytes":
				writeBytes += value
			}
		}
	}
	return readBytes, writeBytes
}

func resolveVirtualNICHost(sysRoot string, nic model.VirtualNIC, networks []model.Network) string {
	for _, network := range networks {
		if network.Name == nic.Target || network.Name == nic.Source {
			return network.Name
		}
	}
	if nic.Source != "" {
		for _, member := range glob(filepath.Join(sysRoot, "class/net", nic.Source, "brif/*")) {
			name := filepath.Base(member)
			if physical, _ := networkDeviceInfo(sysRoot, name); physical {
				return name
			}
		}
	}
	return ""
}

func readCgroupUint(path string) (uint64, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" || trimmed == "max" {
		return 0, nil
	}
	return strconv.ParseUint(trimmed, 10, 64)
}

func isQEMUExecutable(value string) bool {
	base := filepath.Base(value)
	return strings.HasPrefix(base, "qemu-system-") || base == "qemu-kvm"
}

func qemuArgument(fields []string, key string) string {
	for i, field := range fields {
		if field == key && i+1 < len(fields) {
			return fields[i+1]
		}
		if strings.HasPrefix(field, key+"=") {
			return strings.TrimPrefix(field, key+"=")
		}
	}
	return ""
}

func parseQEMUCPU(fields []string) int64 {
	value := qemuArgument(fields, "-smp")
	if value == "" {
		return 0
	}
	if strings.Contains(value, ",") {
		for _, part := range strings.Split(value, ",") {
			if strings.HasPrefix(part, "cpus=") {
				value = strings.TrimPrefix(part, "cpus=")
				break
			}
		}
	}
	result, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return result
}

func parseQEMUMemory(fields []string) uint64 {
	value := qemuArgument(fields, "-m")
	if value == "" {
		return 0
	}
	multiplier := uint64(1024 * 1024)
	last := value[len(value)-1]
	if last == 'K' || last == 'k' {
		multiplier = 1024
		value = value[:len(value)-1]
	}
	if last == 'G' || last == 'g' {
		multiplier = 1024 * 1024 * 1024
		value = value[:len(value)-1]
	}
	if last == 'T' || last == 't' {
		multiplier = 1024 * 1024 * 1024 * 1024
		value = value[:len(value)-1]
	}
	value = strings.TrimPrefix(value, "size=")
	amount, _ := strconv.ParseUint(value, 10, 64)
	return amount * multiplier
}

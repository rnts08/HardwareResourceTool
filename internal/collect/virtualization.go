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
	Name   string    `xml:"name"`
	Memory memoryXML `xml:"memory"`
	VCPU   string    `xml:"vcpu"`
}

type memoryXML struct {
	Unit  string `xml:"unit,attr"`
	Value uint64 `xml:",chardata"`
}

type qemuProcess struct {
	Name   string
	PID    int
	VCPU   int64
	Memory uint64
	RSS    uint64
}

func (c *Collector) collectVirtualization(s *model.Snapshot) error {
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
		virt.VirtualMachines = append(virt.VirtualMachines, model.VirtualMachine{Name: domain.Name, ConfiguredVCPUs: vcpus, ConfiguredMemoryBytes: memoryBytes(domain.Memory), Source: "libvirt"})
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
				matched = true
				break
			}
		}
		if !matched {
			virt.VirtualMachines = append(virt.VirtualMachines, model.VirtualMachine{Name: process.Name, PID: process.PID, ConfiguredVCPUs: process.VCPU, ConfiguredMemoryBytes: process.Memory, ProcessRSSBytes: process.RSS, Running: true, Source: "qemu-process"})
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
		result = append(result, process)
	}
	return result, nil
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

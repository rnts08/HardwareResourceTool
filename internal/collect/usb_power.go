package collect

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
)

// collectUSB gathers static USB device inventory from sysfs once per
// collector lifetime. Interface directories (containing ':') and root hubs
// ("usbN") are excluded.
func (c *Collector) collectUSB(s *model.Snapshot) {
	devices := readUSBDevices(filepath.Join(c.sysRoot, "bus", "usb", "devices"))
	s.USB = devices
	_, err := os.Stat(filepath.Join(c.sysRoot, "kernel", "debug", "usb", "usbmon"))
	s.USBMonAvailable = err == nil
}

func readUSBDevices(dir string) []model.USBDevice {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, ":") {
			continue
		}
		if isRootHubName(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	devices := make([]model.USBDevice, 0, len(names))
	for _, name := range names {
		base := filepath.Join(dir, name)
		device := model.USBDevice{BusID: name}
		device.VendorID = readSysString(base, "idVendor")
		device.ProductID = readSysString(base, "idProduct")
		device.Manufacturer = readSysString(base, "manufacturer")
		device.Product = readSysString(base, "product")
		device.Serial = readSysString(base, "serial")
		device.Class = readSysString(base, "bDeviceClass")
		if speed := readSysString(base, "speed"); speed != "" {
			if mbps, parseErr := strconv.ParseFloat(speed, 64); parseErr == nil {
				device.SpeedMbps = int64(mbps)
			}
		}
		if device.VendorID == "" && device.ProductID == "" && device.Product == "" {
			continue
		}
		devices = append(devices, device)
	}
	return devices
}

func isRootHubName(name string) bool {
	if !strings.HasPrefix(name, "usb") || len(name) < 4 {
		return false
	}
	_, err := strconv.Atoi(name[3:])
	return err == nil
}

func readSysString(base, file string) string {
	data, err := os.ReadFile(filepath.Join(base, file))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// collectCPUPower reads cpufreq policy tunables every snapshot so governor
// changes are reflected quickly.
func (c *Collector) collectCPUPower(s *model.Snapshot) {
	s.CPUPower = readCPUPolicies(filepath.Join(c.sysRoot, "devices", "system", "cpu", "cpufreq"))
}

func readCPUPolicies(base string) []model.CPUPolicy {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	policies := make([]model.CPUPolicy, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "policy") {
			continue
		}
		dir := filepath.Join(base, entry.Name())
		policy := model.CPUPolicy{Policy: entry.Name()}
		policy.Governor = readSysString(dir, "scaling_governor")
		policy.AvailableGovernors = splitFields(readSysString(dir, "scaling_available_governors"))
		policy.EPP = readSysString(dir, "energy_performance_preference")
		policy.AvailableEPP = splitFields(readSysString(dir, "energy_performance_available_preferences"))
		cpus := readSysString(dir, "related_cpus")
		if cpus == "" {
			cpus = readSysString(dir, "affected_cpus")
		}
		policy.CPUs = cpus
		policies = append(policies, policy)
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Policy < policies[j].Policy })
	return policies
}

func splitFields(value string) []string {
	if value == "" {
		return nil
	}
	fields := strings.Fields(value)
	sort.Strings(fields)
	return fields
}

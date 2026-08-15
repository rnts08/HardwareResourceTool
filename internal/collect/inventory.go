package collect

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
)

func (c *Collector) collectHardware(s *model.Snapshot) error {
	if err := c.collectPCI(s); err != nil {
		return err
	}
	if err := c.collectMemoryDevices(s); err != nil {
		return err
	}
	return nil
}

func (c *Collector) collectPCI(s *model.Snapshot) error {
	paths, err := filepath.Glob(filepath.Join(c.sysRoot, "bus/pci/devices/*"))
	if err != nil {
		return fmt.Errorf("pci: %w", err)
	}
	for _, path := range paths {
		address := filepath.Base(path)
		vendor, vendorErr := readTrimmed(filepath.Join(path, "vendor"))
		device, deviceErr := readTrimmed(filepath.Join(path, "device"))
		class, classErr := readTrimmed(filepath.Join(path, "class"))
		if vendorErr != nil || deviceErr != nil || classErr != nil {
			continue
		}
		d := model.PCIDevice{Address: address, VendorID: vendor, DeviceID: device, Class: class, NUMANode: -1}
		d.Driver = symlinkBase(filepath.Join(path, "driver"))
		d.IOMMUGroup = symlinkBase(filepath.Join(path, "iommu_group"))
		d.CurrentLinkSpeed, _ = readTrimmed(filepath.Join(path, "current_link_speed"))
		d.MaxLinkSpeed, _ = readTrimmed(filepath.Join(path, "max_link_speed"))
		d.CurrentLinkWidth, _ = readSysInt(filepath.Join(path, "current_link_width"))
		d.MaxLinkWidth, _ = readSysInt(filepath.Join(path, "max_link_width"))
		d.NUMANode, _ = readSysInt(filepath.Join(path, "numa_node"))
		enrichPCICapabilities(path, &d)
		s.PCI = append(s.PCI, d)
		if strings.EqualFold(vendor, "0x10de") && (strings.HasPrefix(class, "0x03") || strings.HasPrefix(class, "0x12")) {
			s.GPUs = append(s.GPUs, model.GPU{Address: address, VendorID: vendor, DeviceID: device})
		}
	}
	return nil
}

func (c *Collector) collectMemoryDevices(s *model.Snapshot) error {
	data, err := os.ReadFile(filepath.Join(c.sysRoot, "firmware/dmi/tables/DMI"))
	if err == nil {
		for _, device := range parseSMBIOSMemory(data) {
			s.MemoryDevices = append(s.MemoryDevices, device)
		}
	}
	labels := map[string]int{}
	for i, device := range s.MemoryDevices {
		if device.Locator != "" {
			labels[device.Locator] = i
		}
	}
	for _, path := range glob(filepath.Join(c.sysRoot, "devices/system/edac/mc/mc*/dimm*")) {
		label, _ := readTrimmed(filepath.Join(path, "dimm_label"))
		if label == "" {
			label = filepath.Base(path)
		}
		corrected := readUint(filepath.Join(path, "dimm_ce_count"))
		uncorrected := readUint(filepath.Join(path, "dimm_ue_count"))
		if index, ok := labels[label]; ok {
			s.MemoryDevices[index].CorrectedErrors = corrected
			s.MemoryDevices[index].UncorrectedErrors = uncorrected
			continue
		}
		labels[label] = len(s.MemoryDevices)
		s.MemoryDevices = append(s.MemoryDevices, model.MemoryDevice{Locator: label, CorrectedErrors: corrected, UncorrectedErrors: uncorrected})
	}
	return nil
}

func parseSMBIOSMemory(data []byte) []model.MemoryDevice {
	devices := []model.MemoryDevice{}
	for offset := 0; offset+4 <= len(data); {
		length := int(data[offset+1])
		if length < 4 || offset+length > len(data) {
			break
		}
		end := offset + length
		for end+1 < len(data) && !(data[end] == 0 && data[end+1] == 0) {
			end++
		}
		stringsArea := data[offset+length : min(end+2, len(data))]
		if data[offset] == 17 {
			if device, ok := parseSMBIOSMemoryDevice(data[offset:offset+length], stringsArea); ok {
				devices = append(devices, device)
			}
		}
		if end+2 > offset {
			offset = end + 2
		} else {
			break
		}
	}
	return devices
}

func parseSMBIOSMemoryDevice(formatted, stringsArea []byte) (model.MemoryDevice, bool) {
	if len(formatted) < 0x1b {
		return model.MemoryDevice{}, false
	}
	size := binary.LittleEndian.Uint16(formatted[0x0c:0x0e])
	if size == 0 || size == 0xffff {
		return model.MemoryDevice{}, false
	}
	bytes := uint64(0)
	if size == 0x7fff && len(formatted) >= 0x20 {
		bytes = uint64(binary.LittleEndian.Uint32(formatted[0x1c:0x20])) * 1024 * 1024
	} else if size&0x8000 != 0 {
		bytes = uint64(size&0x7fff) * 1024
	} else {
		bytes = uint64(size) * 1024 * 1024
	}
	device := model.MemoryDevice{SizeBytes: bytes, Type: smbiosString(formatted[0x12], stringsArea), SpeedMTs: uint64(binary.LittleEndian.Uint16(formatted[0x15:0x17])), Manufacturer: smbiosString(formatted[0x17], stringsArea), Serial: smbiosString(formatted[0x18], stringsArea), PartNumber: smbiosString(formatted[0x1a], stringsArea)}
	if len(formatted) >= 0x22 {
		device.ConfiguredSpeedMTs = uint64(binary.LittleEndian.Uint16(formatted[0x20:0x22]))
	}
	device.Locator = smbiosString(formatted[0x10], stringsArea)
	return device, true
}

func smbiosString(index byte, area []byte) string {
	if index == 0 {
		return ""
	}
	parts := bytesSplitZero(area)
	if int(index) > len(parts) {
		return ""
	}
	return string(parts[index-1])
}

func bytesSplitZero(data []byte) [][]byte {
	parts := [][]byte{}
	start := 0
	for i, value := range data {
		if value == 0 {
			if i > start {
				parts = append(parts, data[start:i])
			}
			start = i + 1
		}
	}
	return parts
}

func readTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.TrimSpace(string(data)), err
}

func readUint(path string) uint64 {
	value, err := readTrimmed(path)
	if err != nil {
		return 0
	}
	parsed, _ := strconv.ParseUint(value, 10, 64)
	return parsed
}

func symlinkBase(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

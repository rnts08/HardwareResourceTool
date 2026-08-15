package collect

import (
	"encoding/binary"
	"os"

	"hardware-resources-tool/internal/model"
)

const (
	pciConfigMinSize       = 256
	pciExtendedConfigStart = 0x100
)

var pciCapabilityNames = map[uint8]string{
	0x01: "power_management",
	0x05: "msi",
	0x10: "pcie",
	0x11: "msi_x",
}

var pciExtendedCapabilityNames = map[uint16]string{
	0x0001: "aer",
	0x000d: "acs",
	0x000e: "ari",
	0x0010: "sriov",
	0x0015: "resizable_bar",
	0x001d: "dpc",
	0x001e: "l1ss",
	0x002e: "doe",
}

type pciCapability struct {
	id       uint16
	offset   int
	extended bool
}

func enrichPCICapabilities(path string, device *model.PCIDevice) {
	data, err := os.ReadFile(path + "/config")
	if err != nil || len(data) < pciConfigMinSize {
		return
	}
	capabilities := walkPCICapabilities(data)
	seen := map[string]bool{}
	for _, capability := range capabilities {
		name := ""
		if capability.extended {
			name = pciExtendedCapabilityNames[capability.id]
		} else {
			name = pciCapabilityNames[uint8(capability.id)]
		}
		if name != "" && !seen[name] {
			device.Capabilities = append(device.Capabilities, name)
			seen[name] = true
		}
		if capability.extended {
			switch capability.id {
			case 0x0001:
				parseAERCapability(data, capability.offset, device)
			case 0x0010:
				parseSRIOVCapability(data, capability.offset, device)
			case 0x0015:
				device.ResizableBAR = true
			}
		} else if capability.id == 0x10 {
			parsePCIeCapability(data, capability.offset, device)
		}
	}
}

func walkPCICapabilities(data []byte) []pciCapability {
	result := []pciCapability{}
	if len(data) > 0x34 && data[0x06]&0x10 != 0 {
		offset := int(data[0x34] & 0xfc)
		seen := map[int]bool{}
		for offset >= 0x40 && offset+2 <= min(len(data), pciConfigMinSize) && !seen[offset] {
			seen[offset] = true
			id := uint16(data[offset])
			if id == 0 {
				break
			}
			result = append(result, pciCapability{id: id, offset: offset})
			next := int(data[offset+1] & 0xfc)
			if next == 0 || next <= offset {
				break
			}
			offset = next
		}
	}
	if len(data) < pciExtendedConfigStart+4 {
		return result
	}
	for offset := pciExtendedConfigStart; offset+4 <= len(data); {
		header := binary.LittleEndian.Uint32(data[offset : offset+4])
		id := uint16(header & 0xffff)
		if id == 0 || id == 0xffff {
			break
		}
		result = append(result, pciCapability{id: id, offset: offset, extended: true})
		next := int((header >> 20) & 0xfff)
		if next < pciExtendedConfigStart || next <= offset {
			break
		}
		offset = next
	}
	return result
}

func parsePCIeCapability(data []byte, offset int, device *model.PCIDevice) {
	if offset < 0 || offset+0x0e > len(data) {
		return
	}
	deviceCap := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
	device.PCIeMaxPayloadBytes = int64(128 << ((deviceCap >> 0) & 0x7))
	device.PCIeMaxReadRequestBytes = int64(128 << ((deviceCap >> 12) & 0x7))
}

func parseAERCapability(data []byte, offset int, device *model.PCIDevice) {
	if offset < 0 || offset+0x14 > len(data) {
		return
	}
	device.AERUncorrectableStatus = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
	device.AERCorrectableStatus = binary.LittleEndian.Uint32(data[offset+0x10 : offset+0x14])
}

func parseSRIOVCapability(data []byte, offset int, device *model.PCIDevice) {
	if offset < 0 || offset+0x12 > len(data) {
		return
	}
	device.SRIOVTotalVFs = int64(binary.LittleEndian.Uint16(data[offset+0x0e : offset+0x10]))
}

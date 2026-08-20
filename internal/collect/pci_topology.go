package collect

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
)

const (
	ioresourceIO       = 0x00000100
	ioresourceMem      = 0x00000200
	ioresourcePrefetch = 0x00001000
	ioresourceReadonly = 0x00002000
	ioresourceMem64    = 0x00100000
)

var pciAddressPattern = regexp.MustCompile(`^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]$`)

func enrichPCITopology(paths []string, devices []model.PCIDevice) {
	byAddress := make(map[string]*model.PCIDevice, len(devices))
	for i := range devices {
		byAddress[devices[i].Address] = &devices[i]
	}
	for _, path := range paths {
		address := filepath.Base(path)
		device := byAddress[address]
		if device == nil {
			continue
		}
		device.PCIePath = pciParentPath(path)
		if len(device.PCIePath) > 1 {
			device.PCIeParentAddress = device.PCIePath[1]
		}
		device.PCIePFAddress = symlinkBase(filepath.Join(path, "physfn"))
		device.PCIeVFAddresses = pciVirtualFunctions(path)
		collectPCIResources(path, device)
		for _, node := range device.PCIePath {
			candidate := byAddress[node]
			if candidate == nil {
				continue
			}
			bandwidth, ok := pcieBandwidth(candidate.PCIeNegotiatedSpeed, candidate.PCIeNegotiatedWidth)
			if !ok {
				continue
			}
			if device.PCIePathBandwidthGbps == 0 || bandwidth < device.PCIePathBandwidthGbps {
				device.PCIePathBandwidthGbps = bandwidth
				device.PCIePathMinSpeed = candidate.PCIeNegotiatedSpeed
				device.PCIePathMinWidth = candidate.PCIeNegotiatedWidth
				device.PCIePathBottleneck = candidate.Address
			}
		}
	}
}

func pciVirtualFunctions(path string) []string {
	result := []string{}
	for _, link := range glob(filepath.Join(path, "virtfn*")) {
		if address := symlinkBase(link); pciAddressPattern.MatchString(address) {
			result = append(result, address)
		}
	}
	return result
}

func collectPCIResources(path string, device *model.PCIDevice) {
	file, err := os.Open(filepath.Join(path, "resource"))
	if err != nil {
		return
	}
	defer file.Close()
	index := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			index++
			continue
		}
		start, startErr := strconv.ParseUint(strings.TrimPrefix(fields[0], "0x"), 16, 64)
		end, endErr := strconv.ParseUint(strings.TrimPrefix(fields[1], "0x"), 16, 64)
		flags, flagsErr := strconv.ParseUint(strings.TrimPrefix(fields[2], "0x"), 16, 64)
		if startErr != nil || endErr != nil || flagsErr != nil || end < start || (start == 0 && end == 0) {
			index++
			continue
		}
		switch {
		case index >= 0 && index < 6: // BARs 0..5
			device.BARs = append(device.BARs, pciBarFromResource(index, start, end, flags))
			device.BARCount++
			device.BARTotalBytes += end - start + 1
			if start > uint64(^uint32(0)) || end > uint64(^uint32(0)) {
				device.BARAbove4G = true
			}
		case index == 6: // expansion ROM
			device.ROM = true
		case index >= 7: // bridge resource windows
			device.ResourceWindows = append(device.ResourceWindows, fmt.Sprintf("%s-%s", fields[0], fields[1]))
		}
		index++
	}
}

func pciBarFromResource(index int, start, end, flags uint64) model.PCIBar {
	bar := model.PCIBar{Index: index, Start: start, End: end}
	switch flags & 0x0f00 {
	case ioresourceIO:
		bar.Type = "io"
	case ioresourceMem:
		if flags&ioresourceMem64 != 0 {
			bar.Type = "64-bit memory"
		} else {
			bar.Type = "memory"
		}
	default:
		bar.Type = fmt.Sprintf("0x%x", flags&0x0f00)
	}
	bar.Prefetchable = flags&ioresourcePrefetch != 0
	bar.ROM = flags&ioresourceReadonly != 0
	return bar
}

func pciParentPath(path string) []string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return []string{filepath.Base(path)}
	}
	result := []string{filepath.Base(path)}
	current := resolved
	for len(result) < 64 {
		parent := filepath.Dir(current)
		name := filepath.Base(parent)
		if name == filepath.Base(current) || parent == current || name == "" {
			break
		}
		if pciAddressPattern.MatchString(name) {
			result = append(result, name)
		}
		current = parent
		if !pciAddressPattern.MatchString(name) {
			break
		}
	}
	return result
}

func pcieBandwidth(speed string, width int64) (float64, bool) {
	if width <= 0 {
		return 0, false
	}
	value := strings.TrimSpace(speed)
	if value == "" {
		return 0, false
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, false
	}
	gt, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	efficiency := 0.8
	if gt >= 8 {
		efficiency = 128.0 / 130.0
	}
	if gt >= 64 {
		efficiency = 1
	}
	return gt * efficiency * float64(width), true
}

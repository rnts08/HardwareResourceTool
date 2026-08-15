package collect

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
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

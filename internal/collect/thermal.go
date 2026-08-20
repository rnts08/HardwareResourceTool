package collect

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
)

// collectThermal reads read-only thermal state from /sys/class/thermal
// thermal zones and /sys/class/hwmon sensors. hwmon devices expose
// temperature, fan, and (in some drivers) power/energy sensors; thermal zones
// expose kernel thermal-governor state with trip-point thresholds. Both are
// optional and are left empty when the kernel exposes nothing.
func (c *Collector) collectThermal(s *model.Snapshot) error {
	sysRoot := c.sysRoot
	if sysRoot == "" {
		sysRoot = "/sys"
	}
	thermal := model.Thermal{Zones: []model.ThermalZone{}, Sensors: []model.Temperature{}, Fans: []model.FanSpeed{}}

	for _, path := range glob(filepath.Join(sysRoot, "class/thermal/thermal_zone[0-9]*")) {
		zone := model.ThermalZone{Name: filepath.Base(path)}
		zone.Type, _ = readTrimmed(filepath.Join(path, "type"))
		zone.Policy, _ = readTrimmed(filepath.Join(path, "policy"))
		zone.Mode, _ = readTrimmed(filepath.Join(path, "mode"))
		zone.Current, _ = readMilliCelsius(filepath.Join(path, "temp"))
		for _, trip := range glob(filepath.Join(path, "trip_point_[0-9]*_temp")) {
			kind, _ := readTrimmed(filepath.Join(path, strings.TrimSuffix(filepath.Base(trip), "_temp")+"_type"))
			if celsius, ok := readMilliCelsius(trip); ok {
				switch kind {
				case "critical":
					zone.Critical = celsius
				case "passive":
					zone.Passive = celsius
				}
			}
		}
		thermal.Zones = append(thermal.Zones, zone)
	}

	for _, path := range glob(filepath.Join(sysRoot, "class/hwmon/hwmon[0-9]*")) {
		source, _ := readTrimmed(filepath.Join(path, "name"))
		kind := thermalSensorKind(source)
		for _, input := range glob(filepath.Join(path, "temp[0-9]*_input")) {
			base := strings.TrimSuffix(input, "_input")
			current, ok := readMilliCelsius(input)
			if !ok {
				continue
			}
			sensor := strings.TrimPrefix(filepath.Base(base), "temp")
			label, _ := readTrimmed(base + "_label")
			maximum, _ := readMilliCelsius(base + "_max")
			critical, _ := readMilliCelsius(base + "_crit")
			alarm := readUint(filepath.Join(base+"_alarm")) > 0
			thermal.Sensors = append(thermal.Sensors, model.Temperature{Name: filepath.Base(path), Sensor: sensor, Label: label, Source: source, Kind: kind, Current: current, Max: maximum, Critical: critical, Alarm: alarm})
		}
		for _, input := range glob(filepath.Join(path, "fan[0-9]*_input")) {
			base := strings.TrimSuffix(input, "_input")
			value, ok := readUintFile(input)
			if !ok {
				continue
			}
			sensor := strings.TrimPrefix(filepath.Base(base), "fan")
			label, _ := readTrimmed(base + "_label")
			minimum, _ := readUintFile(base + "_min")
			maximum, _ := readUintFile(base + "_max")
			thermal.Fans = append(thermal.Fans, model.FanSpeed{Name: filepath.Base(path), Sensor: sensor, Label: label, Source: source, Input: value, Min: minimum, Max: maximum})
		}
	}

	if len(thermal.Zones) == 0 {
		thermal.Zones = nil
	}
	if len(thermal.Sensors) == 0 {
		thermal.Sensors = nil
	}
	if len(thermal.Fans) == 0 {
		thermal.Fans = nil
	}
	s.Thermal = thermal
	return nil
}

// thermalSensorKind classifies an hwmon device by its kernel name so sensors
// can be grouped as CPU, GPU, disk, or board. The name is the hwmon `name`
// attribute, which is more stable than inferring from the device path.
func thermalSensorKind(name string) string {
	switch {
	case strings.Contains(name, "core"), strings.Contains(name, "k10"), strings.Contains(name, "zen"), strings.Contains(name, "pkg"), strings.Contains(name, "cpu"):
		return "cpu"
	case strings.Contains(name, "gpu"), strings.Contains(name, "amdgpu"), strings.Contains(name, "nvidia"), strings.Contains(name, "radeon"):
		return "gpu"
	case strings.Contains(name, "nvme"), strings.Contains(name, "drivetemp"), strings.Contains(name, "ahci"), strings.Contains(name, "scsi"), strings.Contains(name, "ata"):
		return "disk"
	case strings.Contains(name, "acpi"), strings.Contains(name, "board"), strings.Contains(name, "jc42"), strings.Contains(name, "dell_smm"), strings.Contains(name, "thinkpad"), strings.Contains(name, "tmp"), strings.Contains(name, "pch"):
		return "board"
	default:
		return "other"
	}
}

func readUintFile(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func readMilliCelsius(path string) (float64, bool) {
	value, ok := readUintFile(path)
	if !ok {
		return 0, false
	}
	return float64(value) / 1000.0, true
}

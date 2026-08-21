//go:build !linux || !cgo

package collect

import (
	"errors"

	"hardware-resources-tool/internal/model"
)

type nvmlGPUData struct {
	BusID, Name, UUID                      string
	MemoryTotal, MemoryUsed, MemoryProcess uint64
	Utilization                            float64
	Temperature                            float64
	PowerWatts                             float64
	ECCEnabled                             bool
	ECCCorrected                           uint64
	ECCUncorrected                         uint64
	MIGEnabled                             bool
	MIGMaxInstances                        uint64
	MIGInstances                           []model.MIGInstance
	NvLinkVersion                          int
	NvLinkCount                            int
	NvLinks                                []model.NvLink
}

// nvmlNvLinkDeviceTypeName maps an NVML NvLink remote device type to a label.
func nvmlNvLinkDeviceTypeName(remoteType uint32) string {
	switch remoteType {
	case 0:
		return "gpu"
	case 1:
		return "switch"
	default:
		return "unknown"
	}
}

// nvlinkNominalGBps maps the NVLink major version to its nominal per-link,
// per-direction transfer rate. Versions outside the well-established range
// return zero so callers do not overstate bandwidth.
func nvlinkNominalGBps(version int) int64 {
	switch version {
	case 1:
		return 20
	case 2:
		return 25
	case 3:
		return 50
	case 4:
		return 100
	default:
		return 0
	}
}

func collectNVML() ([]nvmlGPUData, error) {
	return nil, errors.New("NVML unavailable on this build or platform")
}

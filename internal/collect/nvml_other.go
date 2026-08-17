//go:build !linux || !cgo

package collect

import "errors"

type nvmlGPUData struct {
	BusID, Name, UUID                      string
	MemoryTotal, MemoryUsed, MemoryProcess uint64
	Utilization                            float64
	Temperature                            float64
	PowerWatts                             float64
}

func collectNVML() ([]nvmlGPUData, error) {
	return nil, errors.New("NVML unavailable on this build or platform")
}

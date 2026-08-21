package collect

import (
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestApplyNVMLToGPUCopiesAllTelemetry(t *testing.T) {
	target := &model.GPU{}
	applyNVMLToGPU(target, nvmlGPUData{
		Name: "NVIDIA A100", UUID: "GPU-abc",
		MemoryTotal: 100, MemoryUsed: 40, MemoryProcess: 30,
		Utilization: 55, Temperature: 61, PowerWatts: 210,
		ECCEnabled: true, ECCCorrected: 3, ECCUncorrected: 1,
		MIGEnabled: true, MIGMaxInstances: 7,
		MIGInstances:  []model.MIGInstance{{Index: 0, Profile: "1g.5gb"}},
		NvLinkVersion: 3, NvLinkCount: 12,
		NvLinks: []model.NvLink{{Index: 0, Active: true}},
	})
	if target.Name != "NVIDIA A100" || target.UUID != "GPU-abc" {
		t.Errorf("identity not copied: %#v", target)
	}
	if target.MemoryBytes != 100 || target.MemoryUsedBytes != 40 || target.MemoryProcessBytes != 30 || target.MemorySource != "device" {
		t.Errorf("memory not copied: %#v", target)
	}
	if target.UtilizationPercent != 55 || target.TemperatureCelsius != 61 || target.PowerWatts != 210 {
		t.Errorf("telemetry not copied: %#v", target)
	}
	if !target.ECCEnabled || target.ECCCorrected != 3 || target.ECCUncorrected != 1 {
		t.Errorf("ECC fields dropped: %#v", target)
	}
	if !target.MIGEnabled || target.MIGMaxInstances != 7 || len(target.MIGInstances) != 1 || target.MIGInstances[0].Profile != "1g.5gb" {
		t.Errorf("MIG fields dropped: %#v", target)
	}
	if target.NvLinkCount != 12 || target.NvLinkVersion != "3.0" || target.NvLinkBandwidthGBps != 50 || len(target.NvLinks) != 1 || !target.NvLinks[0].Active {
		t.Errorf("NVLink fields dropped: %#v", target)
	}
	if !target.NVML || target.NVMLStatus != "available" {
		t.Errorf("availability not set: %#v", target)
	}
}

func TestApplyNVMLToGPUPrefersProcessAccounting(t *testing.T) {
	target := &model.GPU{}
	applyNVMLToGPU(target, nvmlGPUData{MemoryTotal: 100, MemoryUsed: 20, MemoryProcess: 50})
	if target.MemoryUsedBytes != 50 || target.MemorySource != "process-accounting" {
		t.Errorf("process accounting not preferred: %#v", target)
	}
	target = &model.GPU{}
	applyNVMLToGPU(target, nvmlGPUData{MemoryTotal: 100, MemoryUsed: 60, MemoryProcess: 200})
	if target.MemoryUsedBytes != 60 || target.MemorySource != "device" {
		t.Errorf("implausible process accounting accepted: %#v", target)
	}
}

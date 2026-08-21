//go:build linux && cgo

package collect

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>

typedef void* nvmlDevice_t;
typedef int nvmlReturn_t;
typedef nvmlReturn_t (*nvmlInitFn)(void);
typedef nvmlReturn_t (*nvmlShutdownFn)(void);
typedef nvmlReturn_t (*nvmlGetCountFn)(unsigned int*);
typedef nvmlReturn_t (*nvmlGetHandleFn)(unsigned int, nvmlDevice_t*);
typedef nvmlReturn_t (*nvmlGetStringFn)(nvmlDevice_t, char*, unsigned int);
typedef nvmlReturn_t (*nvmlGetMemoryFn)(nvmlDevice_t, void*);
typedef nvmlReturn_t (*nvmlGetUtilFn)(nvmlDevice_t, void*);
typedef nvmlReturn_t (*nvmlGetTempFn)(nvmlDevice_t, int, unsigned int*);
typedef nvmlReturn_t (*nvmlGetPowerFn)(nvmlDevice_t, unsigned int*);
typedef nvmlReturn_t (*nvmlGetPciFn)(nvmlDevice_t, void*);
typedef nvmlReturn_t (*nvmlGetEccModeFn)(nvmlDevice_t, unsigned int*, unsigned int*);
typedef nvmlReturn_t (*nvmlGetEccErrorsFn)(nvmlDevice_t, unsigned int, unsigned int, unsigned long long*);
typedef nvmlReturn_t (*nvmlGetMigModeFn)(nvmlDevice_t, unsigned int*, unsigned int*);
typedef nvmlReturn_t (*nvmlGetMigCountFn)(nvmlDevice_t, unsigned int*);
typedef nvmlReturn_t (*nvmlGetMigHandleFn)(nvmlDevice_t, unsigned int, nvmlDevice_t*);
typedef nvmlReturn_t (*nvmlGetGpuInstanceIdFn)(nvmlDevice_t, unsigned int*);
typedef nvmlReturn_t (*nvmlGetGpuInstanceByIdFn)(nvmlDevice_t, unsigned int, void*);
typedef nvmlReturn_t (*nvmlGpuInstanceGetInfoFn)(void*, void*);
typedef nvmlReturn_t (*nvmlGetProcessesFn)(nvmlDevice_t, unsigned int*, void*);
typedef nvmlReturn_t (*nvmlGetNvLinkCountFn)(nvmlDevice_t, unsigned int*);
typedef nvmlReturn_t (*nvmlGetNvLinkVersionFn)(nvmlDevice_t, unsigned int*);
typedef nvmlReturn_t (*nvmlGetNvLinkStateFn)(nvmlDevice_t, unsigned int, unsigned int*);
typedef nvmlReturn_t (*nvmlGetNvLinkRemotePciFn)(nvmlDevice_t, unsigned int, void*);
typedef nvmlReturn_t (*nvmlGetNvLinkRemoteTypeFn)(nvmlDevice_t, unsigned int, unsigned int*);
typedef nvmlReturn_t (*nvmlGetNvLinkUtilFn)(nvmlDevice_t, unsigned int, unsigned int, unsigned long long*, unsigned long long*);

typedef struct { unsigned long long total, free, used; } hwtMemory;
typedef struct { unsigned int gpu, memory; } hwtUtilization;
typedef struct { char busId[32]; unsigned char reserved[64]; } hwtPciInfo;
typedef struct {
    char busId[32];
    unsigned int domain;
    unsigned int bus;
    unsigned int device;
    unsigned int pciDeviceId;
    unsigned int pciSubSystemId;
    char busIdLegacy[16];
    unsigned char reserved[64];
} hwtNvLinkPciInfo;
typedef struct {
    unsigned int type;
    unsigned int uuid[16];
    char profileName[96];
} hwtGpuInstanceInfo;
#define HWT_MAX_MIG 32
#define HWT_MAX_LINKS 16
typedef struct {
    unsigned int index;
    unsigned int gpuInstanceId;
    char profile[96];
    unsigned long long memoryTotal;
    unsigned long long memoryUsed;
    unsigned int utilization;
    unsigned int temperature;
    unsigned int powerMilliwatts;
    unsigned long long processMemory;
} hwtMigInstance;
typedef struct {
    unsigned int index;
    unsigned int active;
    char remoteBusId[32];
    unsigned int remoteType;
    unsigned long long readBytes;
    unsigned long long writeBytes;
} hwtNvLink;
typedef struct {
    char busId[32];
    char name[96];
    char uuid[96];
    unsigned long long memoryTotal;
    unsigned long long memoryUsed;
    unsigned int utilization;
    unsigned int temperature;
    unsigned int powerMilliwatts;
    unsigned int eccEnabled;
    unsigned long long eccCorrected;
    unsigned long long eccUncorrected;
    unsigned int migEnabled;
    unsigned int migInstances;
    unsigned long long processMemory;
    unsigned int nvlinkVersion;
    unsigned int nvlinkCount;
    unsigned int migInstanceCount;
    hwtMigInstance migInstanceList[HWT_MAX_MIG];
    hwtNvLink nvlinkList[HWT_MAX_LINKS];
} hwtNvmlGPU;

typedef struct {
    unsigned int pid;
    unsigned int reserved;
    unsigned long long usedGpuMemory;
    unsigned int gpuInstanceId;
    unsigned int computeInstanceId;
} hwtProcessInfo;

static void hwtError(char* error, unsigned int length, const char* prefix, int code) {
    if (length > 0) snprintf(error, length, "%s (%d)", prefix, code);
}

static unsigned int hwtNvmlCollect(hwtNvmlGPU* output, unsigned int capacity, char* error, unsigned int errorLength) {
    void* library = dlopen("libnvidia-ml.so.1", RTLD_LAZY | RTLD_LOCAL);
    if (!library) library = dlopen("libnvidia-ml.so", RTLD_LAZY | RTLD_LOCAL);
    if (!library) {
        if (errorLength > 0) snprintf(error, errorLength, "NVML library unavailable");
        return 0;
    }
    nvmlInitFn init = (nvmlInitFn)dlsym(library, "nvmlInit_v2");
    nvmlShutdownFn shutdown = (nvmlShutdownFn)dlsym(library, "nvmlShutdown");
    nvmlGetCountFn getCount = (nvmlGetCountFn)dlsym(library, "nvmlDeviceGetCount_v2");
    nvmlGetHandleFn getHandle = (nvmlGetHandleFn)dlsym(library, "nvmlDeviceGetHandleByIndex_v2");
    nvmlGetStringFn getName = (nvmlGetStringFn)dlsym(library, "nvmlDeviceGetName");
    nvmlGetStringFn getUUID = (nvmlGetStringFn)dlsym(library, "nvmlDeviceGetUUID");
    nvmlGetMemoryFn getMemory = (nvmlGetMemoryFn)dlsym(library, "nvmlDeviceGetMemoryInfo");
    nvmlGetUtilFn getUtil = (nvmlGetUtilFn)dlsym(library, "nvmlDeviceGetUtilizationRates");
    nvmlGetTempFn getTemp = (nvmlGetTempFn)dlsym(library, "nvmlDeviceGetTemperature");
    nvmlGetPowerFn getPower = (nvmlGetPowerFn)dlsym(library, "nvmlDeviceGetPowerUsage");
    nvmlGetPciFn getPci = (nvmlGetPciFn)dlsym(library, "nvmlDeviceGetPciInfo_v3");
    if (!getPci) getPci = (nvmlGetPciFn)dlsym(library, "nvmlDeviceGetPciInfo_v2");
    nvmlGetEccModeFn getEccMode = (nvmlGetEccModeFn)dlsym(library, "nvmlDeviceGetEccMode");
    nvmlGetEccErrorsFn getEccErrors = (nvmlGetEccErrorsFn)dlsym(library, "nvmlDeviceGetTotalEccErrors");
    nvmlGetMigModeFn getMigMode = (nvmlGetMigModeFn)dlsym(library, "nvmlDeviceGetMigMode");
    nvmlGetMigCountFn getMigCount = (nvmlGetMigCountFn)dlsym(library, "nvmlDeviceGetMaxMigDeviceCount");
    nvmlGetMigHandleFn getMigHandle = (nvmlGetMigHandleFn)dlsym(library, "nvmlDeviceGetMigDeviceHandleByIndex");
    nvmlGetGpuInstanceIdFn getGpuInstanceId = (nvmlGetGpuInstanceIdFn)dlsym(library, "nvmlDeviceGetGpuInstanceId");
    nvmlGetGpuInstanceByIdFn getGpuInstanceById = (nvmlGetGpuInstanceByIdFn)dlsym(library, "nvmlDeviceGetGpuInstanceById");
    nvmlGpuInstanceGetInfoFn getGpuInstanceInfo = (nvmlGpuInstanceGetInfoFn)dlsym(library, "nvmlGpuInstanceGetInfo");
    nvmlGetNvLinkCountFn getNvLinkCount = (nvmlGetNvLinkCountFn)dlsym(library, "nvmlDeviceGetNvLinkCount");
    nvmlGetNvLinkVersionFn getNvLinkVersion = (nvmlGetNvLinkVersionFn)dlsym(library, "nvmlDeviceGetNvLinkVersion");
    nvmlGetNvLinkStateFn getNvLinkState = (nvmlGetNvLinkStateFn)dlsym(library, "nvmlDeviceGetNvLinkState");
    nvmlGetNvLinkRemotePciFn getNvLinkRemotePci = (nvmlGetNvLinkRemotePciFn)dlsym(library, "nvmlDeviceGetNvLinkRemotePciInfo");
    nvmlGetNvLinkRemoteTypeFn getNvLinkRemoteType = (nvmlGetNvLinkRemoteTypeFn)dlsym(library, "nvmlDeviceGetNvLinkRemoteDeviceType");
    nvmlGetNvLinkUtilFn getNvLinkUtil = (nvmlGetNvLinkUtilFn)dlsym(library, "nvmlDeviceGetNvLinkUtilization");
    nvmlGetProcessesFn getComputeProcesses = (nvmlGetProcessesFn)dlsym(library, "nvmlDeviceGetComputeRunningProcesses_v3");
    nvmlGetProcessesFn getGraphicsProcesses = (nvmlGetProcessesFn)dlsym(library, "nvmlDeviceGetGraphicsRunningProcesses_v3");
    nvmlGetProcessesFn getMPSProcesses = (nvmlGetProcessesFn)dlsym(library, "nvmlDeviceGetMPSComputeRunningProcesses_v3");
    if (!init || !shutdown || !getCount || !getHandle || !getPci) {
        if (errorLength > 0) snprintf(error, errorLength, "NVML required symbols unavailable");
        dlclose(library);
        return 0;
    }
    int status = init();
    if (status != 0) {
        hwtError(error, errorLength, "NVML initialization failed", status);
        dlclose(library);
        return 0;
    }
    unsigned int count = 0;
    status = getCount(&count);
    if (status != 0) {
        hwtError(error, errorLength, "NVML device count failed", status);
        shutdown();
        dlclose(library);
        return 0;
    }
    if (count > capacity) count = capacity;
    for (unsigned int i = 0; i < count; i++) {
        nvmlDevice_t device = NULL;
        hwtPciInfo pci;
        memset(&pci, 0, sizeof(pci));
        if (getHandle(i, &device) != 0 || getPci(device, &pci) != 0) continue;
        hwtNvmlGPU* gpu = &output[i];
        memset(gpu, 0, sizeof(*gpu));
        strncpy(gpu->busId, pci.busId, sizeof(gpu->busId) - 1);
        if (getName) getName(device, gpu->name, sizeof(gpu->name));
        if (getUUID) getUUID(device, gpu->uuid, sizeof(gpu->uuid));
        hwtMemory memory;
        memset(&memory, 0, sizeof(memory));
        if (getMemory && getMemory(device, &memory) == 0) {
            gpu->memoryTotal = memory.total;
            gpu->memoryUsed = memory.used;
        }
        hwtUtilization utilization;
        memset(&utilization, 0, sizeof(utilization));
        if (getUtil && getUtil(device, &utilization) == 0) gpu->utilization = utilization.gpu;
        if (getTemp) getTemp(device, 0, &gpu->temperature);
        if (getPower) getPower(device, &gpu->powerMilliwatts);
        if (getEccMode) {
            unsigned int current = 0, pending = 0;
            if (getEccMode(device, &current, &pending) == 0) gpu->eccEnabled = current != 0 || pending != 0;
        }
        if (getEccErrors) {
            // NVML memory error type: corrected=0, uncorrected=1;
            // aggregate counter type: 1. Both calls are read-only.
            getEccErrors(device, 0, 1, &gpu->eccCorrected);
            getEccErrors(device, 1, 1, &gpu->eccUncorrected);
        }
        if (getMigMode) {
            unsigned int current = 0, pending = 0;
            if (getMigMode(device, &current, &pending) == 0) gpu->migEnabled = current != 0 || pending != 0;
        }
        if (getMigCount) getMigCount(device, &gpu->migInstances);
        // Some driver/platform combinations expose a stale or incomplete
        // device-wide memory.used value. Process accounting is read-only and
        // provides a useful cross-check (and is what nvtop commonly presents).
        unsigned int processCount;
        hwtProcessInfo processInfo[256];
        unsigned int processPids[256];
        unsigned long long processBytes[256];
        unsigned int uniqueProcesses = 0;
        nvmlGetProcessesFn processQueries[3] = {getComputeProcesses, getGraphicsProcesses, getMPSProcesses};
        memset(processPids, 0, sizeof(processPids));
        memset(processBytes, 0, sizeof(processBytes));
        for (unsigned int query = 0; query < 3; query++) {
            if (!processQueries[query]) continue;
            processCount = 256;
            memset(processInfo, 0, sizeof(processInfo));
            if (processQueries[query](device, &processCount, processInfo) != 0) continue;
            if (processCount > 256) processCount = 256;
            for (unsigned int p = 0; p < processCount; p++) {
                if (processInfo[p].usedGpuMemory == (unsigned long long)-1) continue;
                unsigned int index = uniqueProcesses;
                for (unsigned int existing = 0; existing < uniqueProcesses; existing++) {
                    if (processPids[existing] == processInfo[p].pid) { index = existing; break; }
                }
                if (index == uniqueProcesses && uniqueProcesses < 256) {
                    processPids[index] = processInfo[p].pid;
                    uniqueProcesses++;
                }
                if (index < 256 && processInfo[p].usedGpuMemory > processBytes[index]) processBytes[index] = processInfo[p].usedGpuMemory;
            }
        }
        for (unsigned int p = 0; p < uniqueProcesses; p++) gpu->processMemory += processBytes[p];
        // NVLink topology and cumulative counters. counterType 0 reads without
        // resetting, so this stays strictly read-only.
        if (getNvLinkCount) {
            unsigned int linkCount = 0;
            if (getNvLinkCount(device, &linkCount) == 0 && linkCount > 0) {
                if (linkCount > HWT_MAX_LINKS) linkCount = HWT_MAX_LINKS;
                gpu->nvlinkCount = linkCount;
                if (getNvLinkVersion) getNvLinkVersion(device, &gpu->nvlinkVersion);
                for (unsigned int link = 0; link < linkCount; link++) {
                    hwtNvLink* nv = &gpu->nvlinkList[link];
                    memset(nv, 0, sizeof(*nv));
                    nv->index = link;
                    unsigned int active = 0;
                    if (getNvLinkState && getNvLinkState(device, link, &active) == 0) nv->active = active;
                    hwtNvLinkPciInfo remote;
                    memset(&remote, 0, sizeof(remote));
                    if (getNvLinkRemotePci && getNvLinkRemotePci(device, link, &remote) == 0) {
                        strncpy(nv->remoteBusId, remote.busId, sizeof(nv->remoteBusId) - 1);
                    }
                    if (getNvLinkRemoteType) getNvLinkRemoteType(device, link, &nv->remoteType);
                    if (getNvLinkUtil) getNvLinkUtil(device, link, 0, &nv->readBytes, &nv->writeBytes);
                }
            }
        }
        // Per-instance MIG inventory through MIG device handles.
        gpu->migInstanceCount = 0;
        if (gpu->migEnabled && getMigHandle) {
            unsigned int maxInstances = gpu->migInstances;
            if (maxInstances > HWT_MAX_MIG) maxInstances = HWT_MAX_MIG;
            for (unsigned int m = 0; m < maxInstances; m++) {
                nvmlDevice_t migDevice = NULL;
                if (getMigHandle(device, m, &migDevice) != 0) continue;
                hwtMigInstance* mi = &gpu->migInstanceList[gpu->migInstanceCount];
                memset(mi, 0, sizeof(*mi));
                mi->index = m;
                if (getGpuInstanceId) getGpuInstanceId(migDevice, &mi->gpuInstanceId);
                if (getGpuInstanceById && getGpuInstanceInfo && mi->gpuInstanceId) {
                    void* gpuInstance = NULL;
                    if (getGpuInstanceById(device, mi->gpuInstanceId, &gpuInstance) == 0 && gpuInstance) {
                        hwtGpuInstanceInfo info;
                        memset(&info, 0, sizeof(info));
                        if (getGpuInstanceInfo(gpuInstance, &info) == 0) {
                            strncpy(mi->profile, info.profileName, sizeof(mi->profile) - 1);
                        }
                    }
                }
                if (mi->profile[0] == '\0' && getName) getName(migDevice, mi->profile, sizeof(mi->profile));
                hwtMemory migMemory;
                memset(&migMemory, 0, sizeof(migMemory));
                if (getMemory && getMemory(migDevice, &migMemory) == 0) {
                    mi->memoryTotal = migMemory.total;
                    mi->memoryUsed = migMemory.used;
                }
                hwtUtilization migUtil;
                memset(&migUtil, 0, sizeof(migUtil));
                if (getUtil && getUtil(migDevice, &migUtil) == 0) mi->utilization = migUtil.gpu;
                if (getTemp) getTemp(migDevice, 0, &mi->temperature);
                if (getPower) getPower(migDevice, &mi->powerMilliwatts);
                unsigned int migProcessCount = 0;
                hwtProcessInfo migProcessInfo[256];
                unsigned int migProcessPids[256];
                unsigned long long migProcessBytes[256];
                unsigned int uniqueMigProcesses = 0;
                nvmlGetProcessesFn migProcessQueries[3] = {getComputeProcesses, getGraphicsProcesses, getMPSProcesses};
                memset(migProcessPids, 0, sizeof(migProcessPids));
                memset(migProcessBytes, 0, sizeof(migProcessBytes));
                for (unsigned int query = 0; query < 3; query++) {
                    if (!migProcessQueries[query]) continue;
                    migProcessCount = 256;
                    memset(migProcessInfo, 0, sizeof(migProcessInfo));
                    if (migProcessQueries[query](migDevice, &migProcessCount, migProcessInfo) != 0) continue;
                    if (migProcessCount > 256) migProcessCount = 256;
                    for (unsigned int p = 0; p < migProcessCount; p++) {
                        if (migProcessInfo[p].usedGpuMemory == (unsigned long long)-1) continue;
                        unsigned int migIndex = uniqueMigProcesses;
                        for (unsigned int existing = 0; existing < uniqueMigProcesses; existing++) {
                            if (migProcessPids[existing] == migProcessInfo[p].pid) { migIndex = existing; break; }
                        }
                        if (migIndex == uniqueMigProcesses && uniqueMigProcesses < 256) {
                            migProcessPids[migIndex] = migProcessInfo[p].pid;
                            uniqueMigProcesses++;
                        }
                        if (migIndex < 256 && migProcessInfo[p].usedGpuMemory > migProcessBytes[migIndex]) migProcessBytes[migIndex] = migProcessInfo[p].usedGpuMemory;
                    }
                }
                for (unsigned int p = 0; p < uniqueMigProcesses; p++) mi->processMemory += migProcessBytes[p];
                gpu->migInstanceCount++;
            }
        }
    }
    shutdown();
    dlclose(library);
    return count;
}
*/
import "C"

import (
	"fmt"
	"unsafe"

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

func collectNVML() ([]nvmlGPUData, error) {
	const capacity = 64
	output := make([]C.hwtNvmlGPU, capacity)
	errorText := make([]C.char, 256)
	count := C.hwtNvmlCollect(&output[0], C.uint(capacity), &errorText[0], C.uint(len(errorText)))
	if count == 0 {
		message := C.GoString(&errorText[0])
		if message == "" {
			message = "NVML returned no devices"
		}
		return nil, fmt.Errorf("%s", message)
	}
	result := make([]nvmlGPUData, 0, int(count))
	for i := 0; i < int(count); i++ {
		gpu := output[i]
		entry := nvmlGPUData{
			BusID:       C.GoString((*C.char)(unsafe.Pointer(&gpu.busId[0]))),
			Name:        C.GoString((*C.char)(unsafe.Pointer(&gpu.name[0]))),
			UUID:        C.GoString((*C.char)(unsafe.Pointer(&gpu.uuid[0]))),
			MemoryTotal: uint64(gpu.memoryTotal), MemoryUsed: uint64(gpu.memoryUsed), MemoryProcess: uint64(gpu.processMemory),
			Utilization: float64(gpu.utilization), Temperature: float64(gpu.temperature),
			PowerWatts: float64(gpu.powerMilliwatts) / 1000,
			ECCEnabled: bool(gpu.eccEnabled != 0), ECCCorrected: uint64(gpu.eccCorrected), ECCUncorrected: uint64(gpu.eccUncorrected),
			MIGEnabled: bool(gpu.migEnabled != 0), MIGMaxInstances: uint64(gpu.migInstances),
			NvLinkVersion: int(gpu.nvlinkVersion), NvLinkCount: int(gpu.nvlinkCount),
		}
		for m := 0; m < int(gpu.migInstanceCount); m++ {
			mig := gpu.migInstanceList[m]
			entry.MIGInstances = append(entry.MIGInstances, model.MIGInstance{
				Index:              int(mig.index),
				GPUInstanceID:      int(mig.gpuInstanceId),
				Profile:            C.GoString((*C.char)(unsafe.Pointer(&mig.profile[0]))),
				MemoryBytes:        uint64(mig.memoryTotal),
				MemoryUsedBytes:    uint64(mig.memoryUsed),
				UtilizationPercent: float64(mig.utilization),
				TemperatureCelsius: float64(mig.temperature),
				PowerWatts:         float64(mig.powerMilliwatts) / 1000,
				ProcessMemoryBytes: uint64(mig.processMemory),
			})
		}
		for l := 0; l < int(gpu.nvlinkCount); l++ {
			link := gpu.nvlinkList[l]
			entry.NvLinks = append(entry.NvLinks, model.NvLink{
				Index:        int(link.index),
				Active:       link.active != 0,
				RemoteDevice: nvmlNvLinkDeviceTypeName(uint32(link.remoteType)),
				RemotePCI:    C.GoString((*C.char)(unsafe.Pointer(&link.remoteBusId[0]))),
				ReadBytes:    uint64(link.readBytes),
				WriteBytes:   uint64(link.writeBytes),
			})
		}
		result = append(result, entry)
	}
	return result, nil
}

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

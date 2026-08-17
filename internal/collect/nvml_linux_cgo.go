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
typedef nvmlReturn_t (*nvmlGetProcessesFn)(nvmlDevice_t, unsigned int*, void*);

typedef struct { unsigned long long total, free, used; } hwtMemory;
typedef struct { unsigned int gpu, memory; } hwtUtilization;
typedef struct { char busId[32]; unsigned char reserved[64]; } hwtPciInfo;
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
		result = append(result, nvmlGPUData{
			BusID:       C.GoString((*C.char)(unsafe.Pointer(&gpu.busId[0]))),
			Name:        C.GoString((*C.char)(unsafe.Pointer(&gpu.name[0]))),
			UUID:        C.GoString((*C.char)(unsafe.Pointer(&gpu.uuid[0]))),
			MemoryTotal: uint64(gpu.memoryTotal), MemoryUsed: uint64(gpu.memoryUsed), MemoryProcess: uint64(gpu.processMemory),
			Utilization: float64(gpu.utilization), Temperature: float64(gpu.temperature),
			PowerWatts: float64(gpu.powerMilliwatts) / 1000,
			ECCEnabled: bool(gpu.eccEnabled != 0), ECCCorrected: uint64(gpu.eccCorrected), ECCUncorrected: uint64(gpu.eccUncorrected),
			MIGEnabled: bool(gpu.migEnabled != 0), MIGMaxInstances: uint64(gpu.migInstances),
		})
	}
	return result, nil
}

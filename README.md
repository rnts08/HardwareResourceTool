# Hardware Resources Tool

Hardware Resources Tool is a read-only Linux host diagnostic CLI for virtualization servers. It measures CPU, memory, storage, network, and system-limit usage, then reports bottlenecks and configuration findings to an operator.

## Primary Goal
Show actual hardware usage out of actual capacity to identify bottlenecks

## Secondary Goal
Find limits and other factors in system settings and configuration that may limit or slow down the machine

## Third Goal
Suggest changes to speed up or free up bottlenecks, including numactl, ioctl, sysctl and other available tunable settings.

## Interface

The v1 interface provides a live Bubble Tea dashboard and JSON/text reports:

```sh
sudo hardware-resources tui
sudo hardware-resources report --json --duration 5s
sudo hardware-resources check
hardware-resources version
```

The TUI refreshes continuously and provides four views: Overview, Storage,
Network, and Findings. Use `1`–`4` or `h`/`l` (arrow keys and Tab also work)
to navigate, and `q` to quit. Overview retains the last 60 samples for compact
CPU and memory sparklines.

The tool requires root for consistent host diagnostics. It never changes host settings and only provides advisory recommendations.

## Tech

The implementation uses Go, direct reads from `/proc` and `/sys`, Cobra for the CLI, and Bubble Tea for the TUI. Collectors are bounded and read-only, and unavailable kernel metrics are reported instead of causing a crash.

Build and test:

```sh
make linux
make check
```

`make linux` builds with NVML dynamic loading enabled, embeds version/build
metadata, applies `-trimpath -s -w`, and strips the resulting Linux amd64
binary. Use `make help` for build, install, coverage, live-test, and cleanup
targets. Set `VERSION`, `PREFIX`, or `DESTDIR` to override defaults.

For a root-only live smoke test that captures text and JSON output under `/tmp`:

```sh
sudo ./scripts/live-collection-test.sh --duration 5s
```

The current v1 focuses on core host bottlenecks. NVIDIA NVML telemetry and
core PCIe topology analysis are available; deeper PCIe semantics, ECC/MIG/
NVLink, and Redfish inventory remain deferred. NIC link state, modes, FEC,
driver identity, queues, and ring information are collected through read-only
Linux interfaces.

Static PCIe, SMBIOS, EDAC, and GPU identity inventory is collected once per
collector lifetime. Dynamic counters continue to refresh at the selected
interval, keeping the monitor overhead low enough for the default two-second
TUI refresh.

Release version: `0.5.2`. Release builds can override it with:

```sh
make linux VERSION=0.5.2
```
 

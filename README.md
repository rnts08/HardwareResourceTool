# Hardware Resources Tool

Hardware Resources Tool is a read-only Linux host diagnostic CLI for virtualization servers. It measures CPU, memory, storage, network, and system-limit usage, then reports bottlenecks and configuration findings to an operator.

## Primary Goal
Show actual hardware usage out of actual capacity to identify bottlenecks

## Secondary Goal
Find limits and other factors in system settings and configuration that may limit or slow down the machine

## Third Goal
Suggest chaanges to speed up or free up bottlenecks, including numactl, ioctl, sysctl and all other available tunable settings.

## Interface

The v1 interface provides a live Bubble Tea dashboard and JSON/text reports:

```sh
sudo hardware-resources tui
sudo hardware-resources report --json --duration 5s
sudo hardware-resources check
hardware-resources version
```

The tool requires root for consistent host diagnostics. It never changes host settings and only provides advisory recommendations.

## Tech

The implementation uses Go, direct reads from `/proc` and `/sys`, Cobra for the CLI, and Bubble Tea for the TUI. Collectors are bounded and read-only, and unavailable kernel metrics are reported instead of causing a crash.

Build and test:

```sh
go build -o hardware-resources ./cmd/hardware-resources
go test ./...
```

The current v1 focuses on core host bottlenecks. GPU, deep PCIe analysis, and automatic tuning are intentionally deferred.
 

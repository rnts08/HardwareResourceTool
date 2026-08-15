# Hardware Resources Tool — Workfile

This file is the handoff context for resuming development on a Linux host.

## Project context

Hardware Resources Tool is a read-only diagnostic CLI for Linux x86_64 virtualization hosts. Its purpose is to compare observed host resource usage with capacity, identify bottlenecks or limiting configuration, and provide advisory remediation guidance.

The v1 scope is deliberately focused on core host resources:

- CPU utilization, load, iowait, interrupts, and context switches.
- Memory, swap, and memory pressure indicators.
- Block-device I/O rates and in-flight I/O.
- Network throughput, errors, and drops.
- Selected transparent huge page, swappiness, and process limit data.
- Findings with severity, evidence, and recommendations.

The tool requires root for consistent diagnostics and never changes host settings.

## Current implementation

- Go module: `hardware-resources-tool`.
- CLI entrypoint: `cmd/hardware-resources/main.go`.
- CLI commands: `tui`, `report`, `check`, and `version`.
- Linux collection: `internal/collect`.
- Shared types: `internal/model`.
- Finding analysis: `internal/analyze`.
- Report rendering: `internal/report`.
- Bubble Tea dashboard: `internal/tui`.
- Basic analysis tests: `internal/analyze/analyze_test.go`.

The collectors read directly from `/proc` and `/sys`; they do not invoke shell commands. Missing metrics are recorded in `collector_errors` rather than causing the process to crash.

## Build and validation on Linux

From the repository root:

```sh
go test ./...
go vet ./...
go build -o hardware-resources ./cmd/hardware-resources
sudo ./hardware-resources check
sudo ./hardware-resources report --json --duration 5s > report.json
sudo ./hardware-resources tui
```

The development machine used for the initial implementation was macOS, so the Linux collector paths were only cross-compiled, not exercised against a Linux `/proc` and `/sys` tree. Linux validation is the next important step.

## Resume checklist

1. Confirm the working tree and read this file plus `README.md`.
2. Run `go test ./...` and `go vet ./...`.
3. Run the binary as root on a Linux virtualization host.
4. Compare CPU and disk rates with `vmstat`, `iostat`, and `sar` where available.
5. Check JSON output on hosts with multiple NUMA nodes, NVMe/SATA disks, swap disabled, and virtual/bonded network interfaces.
6. Add fixture-backed collector tests before expanding the collector set.

## Known limitations and likely next work

- CPU governor is modeled but not yet collected from sysfs.
- NUMA node discovery and remote-memory event collection are not yet implemented.
- Network offload, queue, ring-buffer, and link-speed details are not yet collected.
- Filesystem capacity, mount options, ulimits beyond the current process limits, and broader sysctl checks should be added.
- GPU and deep PCIe diagnostics are intentionally deferred.
- TUI history graphs and subsystem navigation are not yet implemented; the current TUI is a live textual dashboard.
- Thresholds are hard-coded in `internal/analyze/analyze.go`; make them configurable after real-host validation.
- Add parser fixtures and fuzz tests for malformed kernel data.
- Add build metadata through `-ldflags` for release versioning.

## Safety constraints

- Keep collection read-only.
- Do not execute remediation commands automatically.
- Avoid unbounded reads, unbounded history, and shelling out from collectors.
- Preserve explicit collector errors in JSON output.
- Treat kernel/device files as optional because virtualization hosts differ significantly.

## Intended future extensions

The collector and model boundaries should allow adding GPU, PCIe, richer NUMA, NIC offload, and virtualization-platform-specific checks without coupling them to the TUI. Both JSON and TUI should continue consuming the same snapshot and analysis pipeline.

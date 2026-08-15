# Changelog

## 0.8.0 — 2026-08-15

- Added cgroup v2 CPU usage and memory-pressure accounting per QEMU domain.
- Added libvirt hugepage and NUMA nodeset metadata.
- Added bounded read-only QMP queries for domain status and balloon state when
  a QMP Unix socket is exposed through the QEMU command line.
- Added findings for cgroup memory pressure and paused QEMU domains.

## 0.7.0 — 2026-08-15

- Added per-domain QEMU CPU, process RSS, cgroup memory, and read/write I/O
  accounting when the host exposes those read-only interfaces.
- Parsed libvirt disk, NIC, and PCI host-device attachments.
- Correlated VM bridge NICs with physical host NICs where bridge membership is
  available.
- Expanded text and TUI output to distinguish configured, current, and host
  process resource values.

## 0.6.0 — 2026-08-15

- Added read-only KVM/QEMU awareness from `/sys`, libvirt domain XML, and
  `/proc` QEMU process metadata.
- Added configured guest vCPU/memory totals, host overcommit ratios, running
  QEMU process RSS, VM names/PIDs, and source attribution.
- Added advisory findings when configured guest CPU or memory exceeds host
  capacity.

## 0.5.2 — 2026-08-15

- Filtered pseudo-filesystems such as `/proc`, `/sys`, cgroups, debugfs, and
  other non-capacity mounts from filesystem capacity output.
- Restricted the primary NIC report to sysfs device-backed hardware interfaces,
  exposed their PCI addresses, and reported the count of filtered virtual
  interfaces separately.
- Rendered unavailable NVML telemetry as explicit status instead of zero-valued
  measurements.

## 0.5.1 — 2026-08-15

- Added a Makefile for stripped Linux builds, testing, vetting, formatting,
  coverage, installation, live validation, and cleanup.
- Updated the documented release workflow to preserve optional NVML loading.

## 0.5.0 — 2026-08-15

- Added optional dynamically loaded NVIDIA NVML telemetry for GPU identity,
  UUID, memory, utilization, temperature, and power.
- NVML absence, initialization failure, and unmatched devices are reported as
  availability status without failing collection or requiring NVIDIA libraries.
- Reduced TUI redraw duplication, changed the default refresh interval to two
  seconds, enforced a 500 ms minimum, and hardened tiny-terminal clipping.

## 0.4.1 — 2026-08-15

- Added PF/VF relationship discovery from `physfn` and `virtfn*` sysfs links.
- Added PCI BAR count, aggregate size, above-4G detection, and report/TUI output.
- Added IOMMU-group sharing and endpoint/bridge NUMA locality findings.

## 0.4.0 — 2026-08-15

- Added PCIe endpoint-to-bridge path discovery from resolved sysfs links.
- Added negotiated-link bandwidth normalization and minimum-path bottleneck
  reporting.
- Added findings for downgraded links, upstream bottlenecks, and AER status.
- Exposed PCIe path data in JSON, text reports, and the TUI.

## 0.3.1 — 2026-08-15

- Added PCIe capability maximum and negotiated link speed/width decoding.
- Exposed link capability/status details in text and TUI hardware views.

## 0.3.0 — 2026-08-15

- Added bounded PCI standard and extended capability parsing from read-only
  sysfs config space.
- Added PCIe payload/read-request limits, capability presence, AER status,
  SR-IOV VF count, and Resizable BAR metadata to reports and the TUI.
- Added malformed and truncated capability-chain fixture coverage.

## 0.2.5 — 2026-08-15

- Added a read-only Linux ethtool generic-netlink session per collection.
- Added NIC driver, link state, duplex, autonegotiation, link modes, peer modes,
  and FEC fields to JSON, text, and TUI output.
- Preserved unsupported-device ethtool errors without failing collection.
- Added the ordered technical backlog for remaining NIC, PCIe, and telemetry work.

## 0.2.4 — 2026-08-15

- Cached static PCIe, SMBIOS, EDAC, and NVIDIA identity inventory after the first sample.
- Reduced measured snapshot overhead from 16–27 ms to approximately 4.4 ms on the Linux validation host.

## 0.2.3 — 2026-08-15

- Added PCIe device topology and current/max link information.
- Added SMBIOS Type 17 DIMM inventory and EDAC error counters.
- Added NVIDIA GPU PCI identity discovery and a hardware inventory TUI tab.
- Added hardware implementation documentation and vendor-neutral data-source boundaries.

## 0.2.2 — 2026-08-15

- Added configurable analysis thresholds to `check` and `report`.
- Added init-process host limits alongside current-process limits.
- Added Linux NIC ring-size probing through a read-only ethtool ioctl.
- Added terminal-size-aware TUI clipping and commit/build metadata in `version`.

## 0.2.1 — 2026-08-15

- Added selected VM and kernel sysctl values to JSON, text, and TUI reports.
- Added advisory findings for strict memory overcommit and high dirty-page thresholds.
- Added fixture coverage for sysctl collection.

## 0.2.0 — 2026-08-15

This milestone completes the first usable live interface and expands Linux host
coverage.

- Added navigable TUI tabs with bounded CPU and memory history sparklines.
- Added CPU governor, NUMA, filesystem, NIC speed/MTU/queue, and process-limit data.
- Added context-switch, interrupt, swap, and full CPU counter rates.
- Added filesystem-capacity findings and fixture/fuzz coverage for kernel parsers.
- Added the root live-collection capture script and documented controls.

The workfile remains a local handoff document and is intentionally excluded from
Git commits.

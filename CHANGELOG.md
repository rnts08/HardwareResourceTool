# Changelog

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

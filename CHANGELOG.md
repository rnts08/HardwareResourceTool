# Changelog

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

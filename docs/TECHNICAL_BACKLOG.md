# Technical backlog

This backlog is ordered by dependency and risk. Each milestone produces one
version bump and one release commit. `WORKFILE.md` remains a local handoff and
is not committed.

## M0.2.5 — generic ethtool netlink reads

Goal: replace the limited NIC metadata path with a vendor-neutral read-only
generic-netlink client. Version `0.2.5` delivers the first link/FEC slice;
remaining GET families stay as explicit work items below.

Work items:

- Add a Linux-only generic-netlink transport and an unsupported-platform stub.
- Resolve the `ethtool` family and use `ETHTOOL_A_HEADER_DEV_NAME` or ifindex.
- Implement attribute encoding/decoding with nested attributes and compact bitsets.
- Read link modes, link state, duplex, and autonegotiation. Driver identity is
  read from PCI sysfs; firmware, link-info port, and PHY details remain open.
- Read active/wanted/hardware/no-change feature bitsets using `STRSET_GET` names.
- Read channels, rings, coalescing, pause, FEC, RSS, timestamping, and standard stats.
- Never send `*_SET`, `*_ACT`, module EEPROM, cable-test, or firmware operations.
- Preserve per-interface unsupported/error state in `collector_errors` or interface metadata.
- Add fixture tests for nested replies, compact bitsets, malformed attributes, and unsupported devices.

Acceptance for the first slice: JSON contains stable generic link/FEC fields;
no external `ethtool` process is executed; virtual interfaces remain valid.

## M0.2.6 — PCI configuration and capability parser

Status: implemented in `0.3.0`; topology and bandwidth reasoning remains M0.2.7.

- Read PCI config space read-only from `/sys/bus/pci/devices/<addr>/config`.
- Walk standard and extended capability linked lists with bounds checking.
- Decode PCIe device/link/slot/root capability and status registers.
- Decode AER, ACS, ARI, SR-IOV, Resizable BAR, DPC, L1SS, and DOE presence/status.
- Add fixtures for malformed chains, truncated config space, and known capability layouts.

Acceptance: reports distinguish capability limits from negotiated link limits and
never trust malformed offsets.

## M0.2.7 — PCIe topology and bandwidth analysis

Status: implemented in `0.4.0` and `0.4.1`; deeper BAR/resource semantics and
broader NUMA/isolation findings remain follow-up work.

- Build parent-bridge chains and PF/VF relationships from sysfs.
- Convert PCIe generations/encoding to comparable raw bandwidth.
- Find the minimum link on every endpoint-to-root path.
- Compare endpoint capability, current negotiation, upstream bridges, NUMA locality,
  IOMMU groups, and BAR/resource layout.
- Add findings for downgraded links, narrow upstream paths, AER errors, and unsafe
  isolation conditions with conservative evidence.
- Validate against GPU, NVMe, NIC, bridge, SR-IOV, and virtual PCI devices.

Acceptance: each finding identifies the exact limiting link or reports that the
path could not be determined.

## M0.2.8 — hardware telemetry enrichment

Status: core optional NVML enrichment implemented in `0.5.0`; read-only ECC
and MIG-mode telemetry implemented in `0.10.0`; per-instance MIG, NVLink, and
Redfish inventory remain open.

- Add optional dynamically loaded NVIDIA NVML collector.
- Collect GPU UUID/name, framebuffer memory, utilization, temperature, power,
  ECC, MIG, NVLink, and PCI identity.
- Merge NVML data with PCI inventory by normalized bus ID.
- Add optional Redfish inventory adapter for processors, memory, PCIe devices,
  network adapters, ports, and health metrics.
- Keep local collection functional without GPU libraries, BMC credentials, or network.

Acceptance: absent optional providers produce explicit availability state and no crash.

## M0.2.9 — KVM/QEMU resource accounting

Status: basic accounting implemented in `0.6.0`; deeper read-only usage and
attachment accounting implemented in `0.7.0`; newer-QEMU balloon and NUMA
validation implemented in `0.9.0`.

- Discover KVM availability and QEMU processes without shelling out to `virsh`.
- Parse libvirt domain XML when readable, with QEMU command-line fallback.
- Report configured vCPU/memory allocation separately from host physical
  capacity, QEMU process RSS/CPU/I/O, and cgroup current/max memory.
- Identify running versus configured domains and preserve source/uncertainty.
- Add conservative CPU/memory overcommit findings; do not represent guest
  configured memory as actual resident guest memory.

QMP memory-size summary and vCPU state counts, plus runtime `smaps_rollup`
hugepage and `/proc/<pid>/numa_maps` placement data, are now implemented as
optional read-only metrics in `0.10.0`. Remaining work is richer QMP statistics and
broader runtime placement correlation. The QEMU 8.2+ Hyper-V balloon report is
guest-reported and is kept separate from host cgroup/RSS values. QMP remains
optional because the collector must not send mutating commands or assume
socket access.

## M0.2.10 — TUI experience and collection performance

Status: partially implemented in `0.11.0`; the remaining items are the
detail pane, mouse tab clicks, capture comparison, and per-VM availability
surface.

Goal: make the live dashboard reliable on large hosts and actually usable for
inspection. Collection on a busy host (many VMs and NICs) can exceed the
refresh interval, so the TUI must gate work, expose data that currently gets
clipped away, and make abnormal state visually obvious.

Work items:

- Gate concurrent collection: track an in-progress snapshot and skip or
  coalesce refreshes when the previous collection has not finished. Today a
  new `collectNow` fires on every `tickMsg` regardless of completion, so
  overlapping `Snapshot()` calls can race on the collector's `prev`/`prevAt`/
  `hardware` state when collection time exceeds the interval. Implemented in
  `0.11.0`; the tick chain now waits while a collection is in flight.
- Throttle heavy per-snapshot telemetry (QMP queries, ethtool netlink,
  kernel-log tails, and per-VM `numa_maps`/`smaps_rollup`) to every Nth
  snapshot and cache static ethtool metadata so fast metrics stay fresh while
  slow ones do not stall the dashboard. Remaining work.
- Add vertical paging to the Findings, Hardware, Network, and Storage views
  (`j`/`k`, PgUp/PgDn, and mouse wheel). The current `fitView` truncation
  message says to switch tabs for detail, but every tab clips the same way, so
  the detail is unreachable. Implemented in `0.11.0`.
- Add horizontal scrolling so long Network and VM lines are revealed instead
  of being silently ellipsized and lost on normal terminal widths.
  Implemented in `0.11.0`.
- Add severity and abnormal-value colorization: critical/warning/info
  findings plus notable values such as swap activity, near-full filesystems,
  and non-zero ECC/AER/kernel-event counters. Findings severity and overview
  abnormal lines implemented in `0.11.0`; abnormal value colorization beyond
  the overview remains open.
- On the first snapshot, show a "waiting for second sample" indicator instead
  of zero rates and a flat sparkline; rates are only meaningful after two
  samples. Implemented in `0.11.0`.
- Add a pause/freeze hotkey so an operator can hold a snapshot for inspection
  without the view churning underneath them. Implemented in `0.11.0` (`Space`).
- Add a `?` help overlay showing the full keymap, refresh interval, and
  active thresholds. Implemented in `0.11.0`.
- Enable mouse support (tab clicks and wheel scrolling) through bubbletea's
  mouse options. Wheel scrolling implemented in `0.11.0`; tab clicks remain
  open.
- Accept the same threshold flags as `check`/`report` so live findings match
  report findings instead of always using hardcoded defaults. Implemented in
  `0.11.0`.
- Add a focused detail pane for devices and VMs (selected item expands to show
  balloon/QMP/disks/NICs/NUMA breakdown) instead of one enormous line per item.
  Remaining work.
- TUI polish: render the stored `err` field, bound and summarize the
  collector-errors line so it cannot crowd the layout, and show collection
  duration/refresh rate in the header. Implemented in `0.11.0`.

Additional collection and reporting recommendations:

- Cache static ethtool metadata and stale heavy telemetry (QMP, ethtool, log
  tails, `numa_maps`) to cut per-snapshot cost in `report`/`check` as well as
  the TUI. Remaining work.
- Add a top CPU/memory consumers view (per-process, including which QEMU
  process is hot) as a natural next diagnostic. Remaining work.
- Add capture-to-capture comparison for two `report --json` outputs, or longer
  per-tab history, for before/after maintenance and migration reviews.
  Remaining work.
- Report kernel events as per-interval deltas rather than cumulative totals in
  live views. Remaining work.
- Surface per-VM availability when QMP/cgroup telemetry is unavailable instead
  of silently dropping fields. Remaining work.

Acceptance: no concurrent collection on a slow host, all list views are
scrollable, long lines are reachable horizontally, findings and abnormal values
are visually distinct, and the TUI reports thresholds consistent with
`check`/`report`.

## M0.3.0 — thermal telemetry

Status: implemented in `0.11.0`; power/energy sensor reads and thermal sensor
correlation with PCI/NUMA/GPU inventory remain open.

Goal: add read-only host thermal collection and presentation. Temperature,
fan speed, and related thermal state are obvious gaps for a host diagnostics
tool that already reports GPU temperature through NVML.

Work items:

- Read CPU package/socket, board, GPU, and drive temperatures from hwmon
  (`/sys/class/hwmon`) and thermal zones (`/sys/class/thermal/thermal_zone*`),
  including zone type, current/critical temperatures, and throttling state.
  Implemented in `0.11.0`.
- Read fan speeds (RPM) and fan counts when hwmon exposes them, plus any
  available power/energy sensors from the same devices. Fan speeds implemented
  in `0.11.0`; power/energy sensors remain open.
- Correlate thermal sensors with the existing PCI/NUMA/GPU inventory where
  possible. Remaining work.
- Present thermal values in `check`/`report` text and JSON output and in the
  TUI, with advisory findings for temperature approaching critical thresholds.
  Implemented in `0.11.0` (including a Thermal tab).
- Keep everything read-only; absent sensors and drivers must produce explicit
  unknown/availability state and no crash. Implemented in `0.11.0`.

Acceptance: reports and the TUI show available temperatures and fan speeds with
clear availability state, and no external `sensors` process is executed.

## Cross-cutting completion work

- Add integration captures from Intel, Broadcom, NVIDIA/Mellanox, Marvell, and
  virtual NICs.
- Add performance benchmarks for one-second sampling and bound all allocations/history.
- Add release build metadata and a reproducible release command.
- Keep all collectors read-only and update README, WORKFILE, and changelog every milestone.

## Release sequencing plan

Remaining backlog work, ordered by dependency and risk so each release is
testable and self-contained. Every release: bump `VERSION` in the Makefile,
run `make check` and `go test -race ./...`, update README, USERS_MANUAL, and
CHANGELOG, then commit as `release: <version> <summary>` and push. Collection
performance lands first because the TUI depth items are only usable on large
hosts when a snapshot is cheap.

### 0.12.0 — collection performance (prereq for TUI depth)

- Throttle heavy per-snapshot telemetry (QMP queries, ethtool netlink,
  kernel-log tails, per-VM `numa_maps`/`smaps_rollup`) to every Nth snapshot
  and cache static ethtool metadata, in the TUI and in `report`/`check`.
- Report kernel events as per-interval deltas rather than cumulative totals in
  live views.
- Surface per-VM availability when QMP/cgroup telemetry is unavailable instead
  of silently dropping fields.
- Add one-second-sampling performance benchmarks and bound allocations/history.

Acceptance: a large-host snapshot completes within one interval; benchmarks
show the throttled path is faster; VM rows state what is unknown.

### 0.13.0 — TUI interaction depth

- Add a focused detail pane for devices and VMs (selected item expands to show
  balloon/QMP/disks/NICs/NUMA breakdown) instead of one enormous line per item.
- Add mouse tab clicks alongside the existing wheel scrolling.
- Add a top CPU/memory consumers view (per-process, including which QEMU
  process is hot).

Acceptance: an operator can select a VM or device, see its full breakdown, and
jump straight to the hottest processes without reading JSON.

### 0.14.0 — historical comparison

- Add capture-to-capture comparison for two `report --json` outputs, or longer
  per-tab history, for before/after maintenance and migration reviews.

Acceptance: comparing two captures reports changed findings and rate deltas
per resource category.

### 0.15.0 — thermal completion

- Read hwmon power/energy sensors from the same devices that expose
  temperature and fans.
- Correlate thermal sensors with the existing PCI/NUMA/GPU inventory where
  possible.

Acceptance: JSON/text/TUI expose power/energy alongside temperature, and GPU
temperature merges with the GPU inventory.

### 0.16.0 — NIC metadata depth

- Read active/wanted/hardware/no-change feature bitsets using `STRSET_GET`
  names, plus coalescing and RSS GET families.
- Read firmware, link-info port, and PHY details where the driver exposes them.
- Never send `*_SET`/`*_ACT` or mutating operations; keep virtual interfaces
  valid.

Acceptance: JSON contains stable feature/coalescing/RSS fields and per-interface
error state without shelling out to `ethtool`.

### 0.17.0 — PCIe follow-up

- Deeper BAR/resource semantics (resource windows, ROM, prefetch) and broader
  NUMA/isolation findings beyond the current downgraded-link and path checks.

Acceptance: findings distinguish BAR/resource limits from link limits and cover
cross-NUMA and isolation hazards with conservative evidence.

### 0.18.0 — GPU enrichment

- Per-instance MIG inventory and NVLink topology/bandwidth from NVML, merged
  with the existing PCI identity and passthrough state.

Acceptance: absent NVML still yields PCI identity with explicit availability;
with NVML, per-instance MIG and NVLink appear in JSON/text/TUI.

### 0.19.0 — QEMU depth

- Richer read-only QMP statistics (e.g. block/io-stats where non-mutating) and
  broader runtime placement correlation against host NUMA and hugepage state.

Acceptance: VM rows gain the extra QMP statistics and placement correlation
only when the running QEMU exposes them; unknown stays unknown.

### 0.20.0 — optional Redfish inventory

- Optional Redfish adapter for processors, memory, PCIe devices, network
  adapters, ports, and health metrics, kept separate from local collection.

Acceptance: absent BMC credentials or network produce explicit availability
state and no crash; local collection works without any BMC access.

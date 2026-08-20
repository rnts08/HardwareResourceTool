# Changelog

## 0.13.0 — 2026-08-20

- Added a detail pane for devices and VMs. Press `d` on the Hardware tab to
  open a picker of every VM, GPU, PCI device, and DIMM; `j`/`k` select an
  entry and Enter expands it into a full field-by-field breakdown (balloon
  and QMP state, disks, NICs with host correlation and rates, NUMA residency,
  PCI links/BARs/AER, NVML and MIG state, DIMM speed and ECC counters).
  Esc closes the pane or picker.
- Added mouse tab clicks: clicking a tab header row switches views, alongside
  the existing mouse-wheel scrolling.
- The Top processes view now flags which entries are QEMU/KVM processes
  (`[QEMU]`), so the hot guest host-process is visible at a glance.

## 0.12.0 — 2026-08-20

- Added a heavy-telemetry throttle so the per-snapshot cost of QMP queries,
  ethtool netlink reads, and per-VM `smaps_rollup`/`numa_maps` scans is paid
  only once every five snapshots; the first snapshot is always heavy so a
  fresh capture is complete. Lightweight accounting (cgroup counters, `/proc`
  stat, sysfs statistics) still runs on every snapshot, keeping rates live.
- Cached static ethtool metadata (link modes, channels, rings, FEC, pause,
  timestamping) between heavy snapshots; link up/down state remains visible
  every snapshot through sysfs `operstate`.
- Reported kernel/system events as per-interval deltas ("since last sample")
  in the TUI Overview instead of cumulative totals, computed against the
  previous snapshot in the bounded history.
- Surfaced per-VM QMP and runtime-placement availability explicitly: new
  `qmp_available`, `qmp_error`, and `runtime_available` fields distinguish
  "no QMP socket" from "QMP socket present but unreachable", instead of
  silently dropping the fields.
- Added a Top processes tab (key `7`) listing the ten highest-CPU host
  processes with per-second CPU rate, resident set size, and state, sampled
  from `/proc` counters and exposed as `top_processes` in JSON.
- Added performance benchmarks for the per-snapshot `/proc` parsers
  (`parseProcessJiffies`, `parseProcessIO`), runnable with `go test -bench`.

## 0.11.0 — 2026-08-20

- Added read-only host thermal telemetry from `/sys/class/thermal` thermal
  zones (type, current/critical/passive trip points, policy, mode) and
  `/sys/class/hwmon` sensors (per-sensor temperature, max/critical limits,
  alarm flags, and fan speed in RPM), with sensor kind classification
  (cpu/gpu/disk/board) and exposure in JSON, text, and TUI output.
- Added thermal advisory findings for temperatures approaching critical
  thresholds, active thermal alarms, and fans reporting no rotation.
- Added a Thermal tab (key `6`) to the TUI.
- Reworked the TUI refresh loop so a slow snapshot cannot overlap or race the
  next one; collection is gated by an in-progress flag while the timer chain
  keeps the dashboard live.
- Added vertical (`j`/`k`, PgUp/PgDn, mouse wheel) and horizontal (`<`/`>`)
  scrolling to every tab so previously clipped content is reachable.
- Colorized findings and abnormal overview values by severity (critical,
  warning, info) using the existing lipgloss dependency.
- Added TUI pause/resume (`Space`), force refresh (`r`), and a `?` help
  overlay.
- Added `--cpu-idle-critical`, `--iowait-warning`, `--memory-used-critical`,
  `--filesystem-used-warning`, and `--filesystem-used-critical` flags to the
  `tui` command so live findings match `check`/`report` thresholds.
- Added a first-sample "rates appear after the second sample" hint, a
  collection-duration readout, and a bounded collector-errors line in the
  header/footer.

## 0.10.6 — 2026-08-17

- Added `INSTALL.md` covering Makefile installation, sudoers, file
  capabilities, setuid risks, privilege boundaries, verification, and
  uninstall procedures.

## 0.10.5 — 2026-08-17

- Added bounded, read-only classification of existing kernel/system log tails
  for OOM, I/O, PCIe/AER, hardware/EDAC/MCE, NVIDIA Xid, storage-reset, and
  link-failure events.
- Exposed event counts and a maximum of twelve recent matching lines in JSON,
  text, and TUI output.
- Avoided `/proc/kmsg`, `/dev/kmsg`, `dmesg`, journal traversal, and any log
  mutation; missing or inaccessible logs remain non-fatal.
- Added bounded log fixture coverage and documentation of passive collection.

## 0.10.4 — 2026-08-17

- Detect NVIDIA GPUs bound to `vfio-pci`/`pci-stub` or assigned through
  Proxmox `hostpciN` configuration.
- Report passed-through GPUs as available PCI hardware while explicitly
  explaining that host NVML telemetry is unavailable and naming the guest when
  it can be correlated.
- Added passthrough detection fixture coverage and updated user-facing output
  and documentation.

## 0.10.3 — 2026-08-17

- Added read-only Proxmox VE VM configuration discovery from
  `/etc/pve/qemu-server/*.conf`, including VMID, configured vCPU count,
  maximum memory, and balloon minimum memory.
- Correlated Proxmox VM configuration rows with running QEMU processes by VMID
  so stopped and running guests contribute consistently to allocation totals.
- Added NVML running-process framebuffer accounting and use it as a validated
  fallback when device-wide memory accounting is incomplete, with the source
  and process total exposed in JSON/text/TUI output.
- Added Proxmox allocation fixture coverage and updated field documentation.

## 0.10.2 — 2026-08-17

- Restricted filesystem capacity reporting to physical non-USB block-backed
  filesystems and mounted network filesystems declared in `/etc/fstab`.
- Filtered `/run`, `/dev/shm`, `/var/lib/docker`, overlay, snap, tmpfs,
  pseudo-filesystem, loop, and removable USB mounts from capacity output.
- Added filesystem-policy fixture coverage and documentation references.

## 0.10.1 — 2026-08-17

- Added first-run Linux capture instructions and JSON inspection examples to
  the README and user manual.

## 0.10.0 — 2026-08-17

- Added read-only ethtool channel, pause, hardware timestamping/PHC, and
  driver-statistics collection.
- Added optional NVML ECC counters, ECC mode, MIG mode, and maximum MIG device
  count telemetry.
- Added QMP version, memory-size summary, and enabled/total vCPU metrics.
- Added QEMU `smaps_rollup` anonymous hugepage/hugetlb totals and per-node
  `numa_maps` residency accounting with NUMA placement findings.
- Expanded text/TUI output, JSON fields, tests, and hardware documentation.

## 0.9.1 — 2026-08-17

- Replaced the short README usage notes with Linux build, command, TUI, output,
  safety, and limitation guidance.
- Added `USERS_MANUAL.md` with field-by-field interpretation and safe finding
  follow-up procedures.

## 0.9.0 — 2026-08-15

- Refined KVM/QEMU accounting for newer QMP implementations used by current
  Proxmox releases.
- Added read-only QEMU 8.2+ Hyper-V balloon status reporting for guest
  committed and available memory, while preserving the distinct logical
  `query-balloon` value and its source.
- Added libvirt NUMA nodeset exclusion parsing and findings for node indexes
  outside the host NUMA topology.
- Expanded VM text/TUI output with balloon semantics, NUMA placement, and
  hugepage declarations.

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

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

Status: core optional NVML enrichment implemented in `0.5.0`; ECC, MIG, NVLink,
and Redfish inventory remain open.

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
attachment accounting implemented in `0.7.0`.

- Discover KVM availability and QEMU processes without shelling out to `virsh`.
- Parse libvirt domain XML when readable, with QEMU command-line fallback.
- Report configured vCPU/memory allocation separately from host physical
  capacity, QEMU process RSS/CPU/I/O, and cgroup current/max memory.
- Identify running versus configured domains and preserve source/uncertainty.
- Add conservative CPU/memory overcommit findings; do not represent guest
  configured memory as actual resident guest memory.

Remaining: cgroup CPU aggregation, balloon state, hugepage backing, NUMA
placement, and QMP metrics. QMP remains optional because the collector must
not send mutating commands or assume socket access.

## Cross-cutting completion work

- Add integration captures from Intel, Broadcom, NVIDIA/Mellanox, Marvell, and
  virtual NICs.
- Add performance benchmarks for one-second sampling and bound all allocations/history.
- Add release build metadata and a reproducible release command.
- Keep all collectors read-only and update README, WORKFILE, and changelog every milestone.

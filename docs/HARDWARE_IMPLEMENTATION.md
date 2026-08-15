# Hardware inventory implementation

The hardware collectors are deliberately layered. Local Linux interfaces are
the primary source; optional vendor APIs enrich the result without becoming a
requirement for a valid report.

## PCIe and CPU topology

`/sys/bus/pci/devices/*` provides the observed PCI function inventory and
current/max link speed and width. The collector also reads PCI config space to
walk standard and extended capabilities, including PCIe payload limits, AER,
ACS, ARI, SR-IOV, Resizable BAR, DPC, L1SS, and DOE presence. It records
vendor/device IDs, driver, NUMA node, and IOMMU group. PCIe lane counts advertised by Intel Xeon
or AMD EPYC product documentation are reference capabilities only; actual
platform wiring, bifurcation, socket count, and CXL usage determine the
observed link topology.

Resolved PCI device symlinks are used to build endpoint-to-bridge paths. For
each path the collector estimates aggregate bandwidth from negotiated GT/s and
lane width, retains the minimum link and exact bottleneck address, and leaves
the result unknown when a virtual or incomplete topology cannot be resolved.
Analysis reports downgraded endpoint links, upstream path bottlenecks, and
non-zero AER status conservatively.

PF/VF relationships are read from `physfn` and `virtfn*`; resource windows are
summarized from the read-only `resource` file, including aggregate BAR bytes
and above-4G placement. IOMMU-group sharing and endpoint/bridge NUMA mismatch
are reported as advisory conditions rather than automatic isolation failures.

References:

- Linux PCI sysfs: <https://docs.kernel.org/7.1/PCI/sysfs-pci.html>
- Linux PCI link speed/width: <https://docs.kernel.org/6.12/driver-api/pci/pci.html>
- Intel Xeon specifications: <https://www.intel.com/content/www/us/en/support/articles/000031588/processors/intel-xeon-processors.html>
- AMD EPYC specifications: <https://www.amd.com/en/products/specifications/server-processor.html>

## Memory

SMBIOS Type 17 records are parsed from
`/sys/firmware/dmi/tables/DMI` when available. The collector reports DIMM
locator, size, type, vendor, part number, serial, rated speed, and configured
speed. EDAC `mc*/dimm*` entries enrich those records with corrected and
uncorrected error counters. Missing SMBIOS or EDAC support is normal on virtual
machines and is represented by an empty or partial inventory.

Reference: <https://docs.kernel.org/next/admin-guide/ras.html>

## NVIDIA GPUs

NVIDIA display and 3D PCI functions (`vendor=0x10de`) are identified from the
PCI inventory and enriched by an optional dynamically loaded NVML library:
UUID, name, framebuffer memory, utilization, temperature, and power are read
through NVML getters. NVML is never linked at build time and no `nvidia-smi`
subprocess is used. Missing libraries, unavailable devices, and unsupported
getters remain explicit availability status in the model. ECC, MIG, and NVLink
remain future enrichment fields.

Reference: <https://docs.nvidia.com/deploy/nvml-api/index.html>

## NICs

The base collector uses `/sys/class/net`, but the primary NIC list is limited
to interfaces with a sysfs `device` backing path; bridges, veth, tap, loopback,
and other device-less virtual interfaces are counted separately rather than
reported as physical hardware. Read-only ethtool ioctl data supplies ring
sizes, and the Linux ethtool generic-netlink `*_GET` operations supply link state,
duplex/autonegotiation, interface/peer link modes, and FEC. Driver identity is
read from the PCI sysfs driver link. Unsupported virtual devices retain an
error string instead of failing the snapshot. Feature bitsets, channels,
coalescing, pause, RSS, timestamping, and detailed statistics remain in the
technical backlog. No `SET` operation or external command is allowed.

Reference: <https://cdn.kernel.org/doc/html/latest/networking/ethtool-netlink.html>

## KVM/QEMU

KVM availability is detected from the read-only `/sys/module/kvm` or
`/dev/kvm` presence. Domain allocations come from readable libvirt XML under
`/etc/libvirt/qemu`; when XML is unavailable, running QEMU command lines under
`/proc` provide a fallback for VM name, vCPU, and memory arguments. QEMU
process RSS is reported separately as host process usage. When cgroup v2 is
available, current/max memory and aggregate read/write I/O are added; QEMU
process CPU and `/proc/<pid>/io` are used as fallbacks. Disk, NIC, and PCI
host-device attachments are retained, and bridge members are mapped back to
physical NICs where possible. Libvirt hugepage declarations and NUMA nodesets
are retained, and nodeset exclusions are validated against the host node count.
QMP access is bounded and uses only protocol negotiation plus `query-status`,
`query-balloon`, and the optional QEMU 8.2+ `query-hv-balloon-status-report`;
no guest or host mutation is performed.

`query-balloon.actual` is QEMU's logical VM size after ballooning, not resident
host memory. The newer Hyper-V report is separate: `committed` includes guest
use plus unusable/offline memory, while `available` is guest memory available
for new allocations. When configured memory exceeds `actual`, the report also
shows the derived reclaimed amount (`configured - actual`). Host cgroup current
memory and QEMU RSS remain separate measurements.

## Server vendors

Dell PowerEdge/iDRAC, HPE ProLiant/iLO, Lenovo ThinkSystem/XClarity, and other
servers should be supported through an optional Redfish adapter rather than
vendor-specific local collectors. Redfish exposes standard Processor, Memory,
PCIeDevice, NetworkAdapter, NetworkPort, and metric schemas. Local collection
must continue to work without BMC credentials or network access.

Reference: <https://redfish.dmtf.org/redfish/schema_index>

# Hardware inventory implementation

The hardware collectors are deliberately layered. Local Linux interfaces are
the primary source; optional vendor APIs enrich the result without becoming a
requirement for a valid report.

## PCIe and CPU topology

`/sys/bus/pci/devices/*` provides the observed PCI function inventory and
current/max link speed and width. The collector also records vendor/device IDs,
driver, NUMA node, and IOMMU group. PCIe lane counts advertised by Intel Xeon
or AMD EPYC product documentation are reference capabilities only; actual
platform wiring, bifurcation, socket count, and CXL usage determine the
observed link topology.

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
PCI inventory. The `GPU` model is ready for optional NVML enrichment: UUID,
memory, utilization, temperature, ECC, power, MIG, and NVLink should come from
the dynamically loaded NVML library rather than an `nvidia-smi` subprocess.

Reference: <https://docs.nvidia.com/deploy/nvml-api/index.html>

## NICs

The base collector uses `/sys/class/net`, read-only ethtool ioctl data for ring
sizes, and the Linux ethtool generic-netlink `*_GET` operations for link state,
duplex/autonegotiation, interface/peer link modes, and FEC. Driver identity is
read from the PCI sysfs driver link. Unsupported virtual devices retain an
error string instead of failing the snapshot. Feature bitsets, channels,
coalescing, pause, RSS, timestamping, and detailed statistics remain in the
technical backlog. No `SET` operation or external command is allowed.

Reference: <https://cdn.kernel.org/doc/html/latest/networking/ethtool-netlink.html>

## Server vendors

Dell PowerEdge/iDRAC, HPE ProLiant/iLO, Lenovo ThinkSystem/XClarity, and other
servers should be supported through an optional Redfish adapter rather than
vendor-specific local collectors. Redfish exposes standard Processor, Memory,
PCIeDevice, NetworkAdapter, NetworkPort, and metric schemas. Local collection
must continue to work without BMC credentials or network access.

Reference: <https://redfish.dmtf.org/redfish/schema_index>

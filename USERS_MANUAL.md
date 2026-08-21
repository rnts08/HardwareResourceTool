# Hardware Resources Tool — User Manual

This manual describes release 0.15.0. Hardware Resources Tool is a read-only
Linux host diagnostic CLI for bare-metal and virtualization servers,
especially KVM/QEMU and Proxmox environments. It measures the host and
reports evidence; it does not implement changes.

## 1. Operating model

The collector reads Linux interfaces such as /proc, /sys, cgroup v2, libvirt
XML, and selected Linux APIs. Most dynamic values are counters. The first
snapshot establishes a baseline; the next snapshot calculates rates and
percentages from counter differences.

Keep these categories separate:

- Capacity: a detected or configured limit, such as total RAM, logical CPUs,
  link width, or filesystem size.
- Counter: a cumulative value, such as bytes read, packets received, or
  interrupts. It becomes a rate only with two samples.
- Current: a point-in-time value, such as MemAvailable, QEMU RSS, cgroup
  memory, or GPU temperature.
- Configured: a policy or allocation, such as VM vCPU count, VM memory, NUMA
  nodeset, hugepage declaration, or PCIe capability.
- Unknown: the source did not expose a value. Unknown is not zero.

Recommendations are advisory. Validate them against workload behavior, vendor
guidance, maintenance policy, and the virtualization platform before changing
anything.

## 2. Building and running

The Linux workflow is:

    go mod download
    make check
    make linux
    sudo ./hardware-resources-linux-amd64 check

make linux produces an amd64 Linux binary with version, git commit, and UTC
build-date metadata, linker reduction, and optional strip. make check runs
format validation, all Go tests, and go vet. NVML is loaded dynamically, so
an NVIDIA SDK is not required to build.

Useful targets:

    make help       # list targets
    make linux      # stripped Linux amd64 release binary
    make build      # release-style binary named hardware-resources
    make test       # go test ./...
    make vet        # go vet ./...
    make check      # formatting check, tests, and vet
    make coverage   # coverage.out and coverage.html
    make install    # install under /usr/local/bin
    make clean      # remove generated binaries and coverage files

Build variables can be overridden:

    make linux VERSION=0.15.0 LINUX_TARGET=/tmp/hardware-resources-linux-amd64
    make install PREFIX=/opt/hardware-resources DESTDIR=/staging

Diagnostic commands require root so /proc, /sys, PCI metadata, cgroups,
libvirt XML, and QEMU processes are consistently visible:

    sudo ./hardware-resources-linux-amd64 check
    sudo ./hardware-resources-linux-amd64 report --duration 10s
    sudo ./hardware-resources-linux-amd64 report --json --duration 10s > report.json
    sudo ./hardware-resources-linux-amd64 tui --interval 2s
    ./hardware-resources-linux-amd64 version

The live helper builds and reruns itself as root, then writes metadata.txt,
summary.txt, check.txt, report.txt, and report.json under a printed
/tmp/hardware-resources-live.* directory:

    ./scripts/live-collection-test.sh --duration 5s

### First live validation capture

For the first host run, use a ten-second interval so rates are stable:

    make linux
    sudo ./hardware-resources-linux-amd64 check
    sudo ./hardware-resources-linux-amd64 report --duration 10s
    sudo ./hardware-resources-linux-amd64 report --json --duration 10s > /tmp/hardware-resources-report.json

The repeatable helper is preferable when results will be shared:

    ./scripts/live-collection-test.sh --duration 10s

It prints a directory such as /tmp/hardware-resources-live.ABC123. Review
summary.txt first, then report.txt for readable output and report.json for all
fields. Useful checks are:

    jq '.findings' /tmp/hardware-resources-live.ABC123/report.json
    jq '.snapshot.networks[] | {name,driver,driver_stats,ethtool_error}' /tmp/hardware-resources-live.ABC123/report.json
    jq '.snapshot.gpus[] | {address,name,nvml_status,ecc_enabled,ecc_corrected,ecc_uncorrected,mig_enabled,mig_max_instances,mig_instances,nvlink_version,nvlink_count,nvlink_bandwidth_gbps,nvlinks}' /tmp/hardware-resources-live.ABC123/report.json
    jq '.snapshot.virtualization.virtual_machines[] | {name,running,qmp_version,qmp_available,qmp_error,runtime_available,qmp_base_memory_bytes,qmp_vcpus,qmp_block_read_bytes,qmp_block_write_bytes,qmp_block_devices,runtime_anon_huge_bytes,runtime_hugetlb_bytes,runtime_numa_bytes}' /tmp/hardware-resources-live.ABC123/report.json
    jq '.snapshot.top_processes' /tmp/hardware-resources-live.ABC123/report.json

Replace ABC123 with the actual directory name. If a field is absent, check
collector_errors and the corresponding availability/status fields. A missing
NVML library, QMP socket, cgroup file, or unsupported ethtool GET operation is
an expected capability difference, not automatically a fault.

## 3. Commands and flags

### check

check performs a one-second two-sample collection and prints a human-readable
report. It supports these finding thresholds:

| Flag | Default | Meaning |
| --- | ---: | --- |
| --cpu-idle-critical | 10 | Critical below this idle CPU percentage |
| --iowait-warning | 15 | Warning above this iowait percentage |
| --memory-used-critical | 90 | Critical above this memory-used percentage |
| --filesystem-used-warning | 85 | Warning above this filesystem-used percentage |
| --filesystem-used-critical | 95 | Critical above this filesystem-used percentage |

Thresholds affect findings only; they do not alter collection or host policy.

### report

report takes an initial sample, waits for --duration, takes a final sample,
and prints text or JSON:

    sudo hardware-resources report
    sudo hardware-resources report --duration 30s
    sudo hardware-resources report --json --duration 5s > report.json

Use a non-zero interval when interpreting rates. --duration accepts 500ms,
5s, or 1m. The five threshold flags above are also available.

### tui

tui starts the live dashboard. The interval has a 500 ms minimum. The
dashboard keeps at most 60 snapshots for sparklines. Collection is gated so a
slow snapshot cannot overlap the next one, and it accepts the same threshold
flags as `check` and `report` so live findings match report findings.

### compare

compare OLDER.json NEWER.json reads two captures written by `report --json`
and writes a before/after diff: findings that appeared or cleared between the
captures, rate deltas per category (CPU, memory, kernel events,
virtualization overcommit, per-disk and per-network throughput, thermal-zone
temperatures), and newly added or removed disks and networks. Rows with no
observable change are omitted. compare --json emits the same diff as JSON and
does not require root.

### version

version prints application version, operating system, architecture, commit,
and UTC build time. It does not require root.

## 4. TUI windows and controls

The seven windows are Overview, Storage, Network, Findings, Hardware,
Thermal, and Top. Select them with 1–7. Tab, Right arrow, and l move forward.
Shift+Tab, Left arrow, and h move backward. j/k and Page Up/Down scroll the
active window vertically; < and > or Shift+arrows scroll it horizontally;
d on Hardware opens a picker of VMs, GPUs, PCI devices, and DIMMs where j/k
select and Enter expands a field-by-field detail pane; Esc closes the pane or
picker; clicking a tab header row switches views; Space pauses and resumes
collection; r forces an immediate refresh; ? shows help; and q or Ctrl+C
exits. Findings are color-coded by severity (critical, warning, info).

### Overview

Overview shows logical CPU count; user, system, iowait, and idle percentages;
one-, five-, and fifteen-minute load; context switches/s; interrupts/s; and
recent idle history. It also shows memory used percentage, available GiB,
swap in/out per second, memory history, CPU governor, THP policy, swappiness,
NUMA node count, NUMA remote events/s, current process limits, PID 1 limits,
selected sysctls, and KVM/QEMU allocation and overcommit ratios. Kernel and
system event counts (OOM, I/O, PCIe/AER, hardware/EDAC/MCE, NVIDIA Xid,
storage resets, link failures) are shown as deltas since the previous sample
rather than cumulative totals.

High load is not automatically a fault. Compare load with logical CPU count,
idle, iowait, guest demand, and scheduling behavior. High iowait points toward
storage or backing-device contention. Sustained swap activity is stronger
evidence of memory pressure than used percentage alone.

### Storage

Disks show device name, read/write throughput per second, read/write
operations per second, and in-flight I/O. Loop and RAM pseudo devices are
filtered. These are host block-device rates, not necessarily guest filesystem
rates.

Filesystems show mount point, read/write mode, used percentage, available
bytes, and type. The capacity list includes physical non-USB block-backed
filesystems and currently mounted network filesystems only when their mount
point and type are declared in `/etc/fstab`. `/proc`, `/sys`, cgroups, `/run`,
`/dev/shm`, `/var/lib/docker`, overlay mounts, snap filesystems, tmpfs, and
other runtime or pseudo-filesystems are omitted. Removable USB block devices
are omitted from the stable host-capacity baseline.

This is intentional: tmpfs stores files in virtual memory, `/dev/shm` is used
for POSIX shared memory, OverlayFS is a merged upper/lower view, and fstab is
the system’s static filesystem configuration. The collector reports mounted
network filesystems only when they are also part of that persistent
configuration. See the Linux kernel documentation for [tmpfs](https://docs.kernel.org/filesystems/tmpfs.html)
and [OverlayFS](https://docs.kernel.org/filesystems/overlayfs.html), and the
Linux `fstab(5)` reference for [static filesystem entries](https://man7.org/linux/man-pages/man5/fstab.5.html).

### Network

The main list contains sysfs-backed physical or hardware-represented NICs.
Bridges, taps, bonds, and device-less virtual interfaces are counted
separately rather than presented as physical ports.

Rows show interface name, state, PCI address, RX/TX throughput, speed, MTU,
RX/TX queues, RX/TX ring sizes, errors, and drops. The secondary line shows
driver and driver/firmware versions, bus info, link port, PHY address,
transceiver, MDI/MDIX, duplex, autonegotiation, FEC, supported/advertised
modes, and peer mode count. A further line shows hardware/wanted/active/fixed
feature-bit count, coalescing parameters, and the RSS hash function with
indirection-table and key sizes.

Errors and drops are cumulative counters; rising values over repeated reports
matter more than one historical non-zero value. Zero speed may mean link down
or unsupported reporting. Empty optional fields mean the device or kernel did
not expose them. The implementation performs read-only generic-netlink
ethtool and ETHTOOL_G* reads, including channels, pause parameters,
timestamping, feature bitsets, coalescing, RSS, and driver-info strings. It
does not send SET, firmware-update, EEPROM, or cable-test operations.

### Findings

Findings have critical, warning, or info severity and include category, title,
evidence, and recommendation. They are advisories, not proof of causation.
Current findings cover CPU pressure and iowait, memory exhaustion and swap,
filesystem capacity, NIC errors/drops, PCIe negotiation/path/AER issues,
IOMMU-group sharing and cross-NUMA groups, whole-path PCI/bridge NUMA
mismatch, unbound endpoints, passthrough-device NUMA mismatch, VM CPU/memory
overcommit, cgroup pressure, paused QEMU domains, invalid VM NUMA nodesets,
THP and selected sysctl observations, and collection errors.

### Hardware

PCI rows show address, vendor/device IDs, class, driver, NUMA node, current and
maximum link speed/width, path minimum and bottleneck, BAR count/size plus
composition (memory/64-bit/I-O, prefetchable, ROM), resource windows, and
capabilities. The path is the resolved upstream PCIe chain where sysfs makes
it available.

NVIDIA rows always show PCI identity when discovered. With NVML, they add name,
framebuffer used/total, process-accounted framebuffer memory, utilization, temperature, and power. When MIG is
enabled, each MIG instance is listed with its profile, GPU instance ID,
memory used/total and per-instance process memory, utilization, temperature,
and power. NVLink links report the NVLink version, active state, remote
device type (GPU or switch) with its PCI address, cumulative read/write byte
counters (read without resetting), and the nominal per-link bandwidth derived
from the version. If NVML is
missing or cannot initialize, the GPU remains visible with an explicit status;
zero values do not mean zero GPU load. If the device is bound to `vfio-pci` or
`pci-stub`, or assigned through a Proxmox `hostpciN` entry, it is reported as
passed through to a KVM guest; host NVML usage cannot be collected in that
state.

KVM/QEMU rows show VMID where available, running state, configured vCPUs, process/cgroup CPU,
configured/current memory, QEMU RSS, I/O, balloon values, guest-reported
memory, and NUMA nodes. When QMP is reachable they also add cumulative block
statistics per device (read/write bytes and operations from the read-only
`query-blockstats` accounting query). Memory-device rows show locator, size,
type, speed, configured speed, and EDAC corrected/uncorrected errors where
available.

Pressing `d` opens a picker of every VM, GPU, PCI device, and DIMM reported in
this capture. `j`/`k` move the selection and Enter opens a detail pane that
shows the full field breakdown for the chosen item: VMs expand to balloon and
QMP state, per-disk sources, per-NIC host correlation and rates, NUMA
residency, and PCI attachments; GPUs to NVML/MIG/ECC state; PCI devices to
link/capability/BAR/AER fields; DIMMs to speed and ECC counters. Esc returns
from the pane to the picker, and Esc again closes the picker.

### Thermal

Thermal zones show zone name, type (for example `x86_pkg_temp`), current
temperature, critical and passive trip thresholds, governor policy, and mode.
Temperature-sensor rows show the hwmon device and source, the sensor index,
label, kind (cpu/gpu/disk/board), current/max/critical temperatures, and an
ALARM marker when the sensor raises one. Fan rows show the fan index, label,
and current/min/max speed in RPM. Power/energy rows show the sensor index,
label, input watts (with cap and cap-max when the driver exposes them) and
cumulative input joules. A zero fan speed with a defined range is
flagged as a warning, and sensors at or above 90% of their critical threshold
are highlighted. Absent sensors produce an empty window rather than an error.
Sensors whose device is PCI-attached show the backing PCI address, and GPU
temperature/power from hwmon is merged into the GPU inventory only when NVML
does not provide a reading.

### Top

Top processes lists the ten highest-CPU host processes. Rows show the process
name, PID, CPU rate between the last two samples, resident set size, and
state. The first sample cannot produce a CPU rate, so it is shown as zero
until a second snapshot is taken. Processes above 90% CPU are highlighted as
a warning, and QEMU/KVM host processes are flagged with a `[QEMU]` marker so
the hot guest host-process is identifiable at a glance.

## 5. Text report interpretation

Text output contains timestamp, CPU, memory/system, virtualization, limits and
sysctls, real filesystems, physical networks, PCIe devices with useful data,
GPUs, inventory totals, findings, and collector-error count. It is intended
for operators and incident records; JSON is the complete machine interface.

Text units are percentages, GiB for memory, MiB for VM RSS/I/O, KiB/s for VM
NIC rates, and labeled bytes/rates. Empty optional fields, unavailable status,
and zero optional values must be checked against JSON and collector_errors.

VM memory labels:

| Label | Meaning |
| --- | --- |
| memory | configured VM memory followed by cgroup current memory |
| RSS | resident memory of the QEMU host process, not guest working set |
| balloon actual | QMP logical VM size after ballooning |
| balloon reclaimed | configured memory minus actual, when positive |
| balloon target | target only when exposed by QMP |
| committed | QEMU 8.2+ guest report: use plus unusable/offline memory |
| available | QEMU 8.2+ guest report: memory available for new guest allocations |
| source | QMP source for balloon values |

balloon actual is not host resident memory. Compare it with cgroup current
memory, QEMU RSS, configured memory, and guest committed/available values.

## 6. JSON field dictionary

The JSON top level is schema_version, generated_at, duration_ms, snapshot, and
findings[]. The current schema version is 1. Fields marked omitempty disappear
when the source has no value; absence is not proof that hardware or a feature
is absent.

### Snapshot, CPU, and memory

| Field | Meaning |
| --- | --- |
| collected_at | UTC time of final snapshot |
| cpu.logical_cpus | logical CPU capacity |
| cpu.user_percent, system_percent, iowait_percent, idle_percent | host interval CPU time |
| cpu.load_1m, load_5m, load_15m | Linux load averages |
| cpu.context_switches_per_second | context-switch rate |
| cpu.interrupts_per_second | interrupt rate |
| top_processes | up to ten highest-CPU host processes: pid, name, cpu_percent, rss_bytes, state |

The cpu_percent in top_processes is the rate between the last two samples;
the first sample of a run reports zero until a baseline exists.
| memory.total_bytes | host memory capacity |
| memory.available_bytes | Linux MemAvailable |
| memory.used_percent | total minus available, divided by total |
| memory.swap_total_bytes, swap_free_bytes | swap capacity/free bytes |
| memory.swap_in_per_second, swap_out_per_second | swap activity rates |

### Disks, filesystems, and networks

disks entries contain name, cumulative read_bytes/write_bytes,
reads_per_second, writes_per_second, throughput rates, and in_flight.
filesystems entries contain device, mount_point, type, total_bytes,
available_bytes, used_percent, and read_only.

networks entries contain name, state, physical, pci_address, driver,
driver_version, firmware_version, bus_info, link_port, phy_address,
transceiver, tp_mdix, traffic counters/rates, packets, errors, drops,
link_speed_mbps, link_duplex, autonegotiation, link_up, mtu, RX/TX queues and
rings, maximum channel counts, pause state, hardware timestamping, PHC index,
supported/advertised/peer link modes, FEC fields, features_active,
features_wanted, features_hardware, features_no_change, coalesce_* fields,
rss_hash_func, rss_indirection_size, rss_key_size, read-only driver_stats, and
ethtool_error.
virtual_network_count is the number of
filtered virtual/device-less interfaces.

### PCI and GPU

pci_devices entries include address, vendor/device/class, driver, numa_node,
iommu_group, current/max link speed/width, decoded capabilities, PCIe
payload/read-request limits, endpoint capability and negotiated values,
resolved pcie_path, minimum path link and bandwidth, bottleneck and parent
addresses, PF/VF relationships, BAR totals/count/above-4G and structured bars
(index, start/end, memory/64-bit/I-O type, prefetchable, ROM), rom presence,
resource_windows, AER statuses, SR-IOV total VFs, and Resizable BAR presence.

gpus entries include PCI identity plus name, uuid, framebuffer total/used,
process-accounted framebuffer bytes and memory_source, utilization, temperature, power, ECC enabled/corrected/uncorrected counts,
MIG mode and maximum-instance information plus per-instance mig_instances
(index, gpu_instance_id, profile, memory, process memory, utilization,
temperature, power), NVLink version/count/nominal bandwidth and per-link
nvlinks (index, active, remote device type and PCI address, cumulative
read/write bytes), passed_through, passed_through_vm,
nvml_available, and nvml_status. PCI
discovery is independent of NVML runtime telemetry.

### Virtualization

virtualization contains kvm_available, qemu_detected, hypervisor,
virtual_machines, allocated_vcpus, allocated_memory_bytes,
vcpu_overcommit_ratio, and memory_overcommit_ratio. A ratio above 1 is
configured overcommit, not proof of failure.

Each virtual_machines entry contains name, vmid where available, pid, source, running, configured
vCPUs/memory, process and cgroup CPU, process RSS, cgroup current/max/path and
availability, process/cgroup I/O, hugepages and hugepage_bytes, runtime
anonymous hugepage and hugetlb bytes, per-node runtime numa_maps bytes, parsed
numa_nodes, QMP version, base/plugged memory, total/enabled vCPU counts,
cumulative QMP block statistics (qmp_block_read_bytes, qmp_block_write_bytes,
qmp_block_read_ops, qmp_block_write_ops) plus the per-device
qmp_block_devices list with device/node names,
qmp_status, qmp_available, qmp_error, runtime_available, balloon
enabled/reported/guest-report/source flags, balloon
actual/target/reclaimed/committed/available bytes, and disks, nics, and
pci_addresses.

qmp_available is false when a VM has no QMP socket or its socket is
unreachable; qmp_error records "QMP socket unavailable" when a configured
socket cannot be queried, so the two cases are distinguishable.
runtime_available is true when smaps_rollup/numa_maps data was read
successfully. Because QMP and runtime-placement reads are throttled to every
fifth snapshot, these fields reflect the most recent heavy collection.

VM nics include guest type/source/target/MAC, host bridge/NIC correlation,
and host RX/TX rates when resolvable. VM PCI addresses identify attachments;
they do not indicate device activity.

### NUMA and system

numa.nodes is the host node count. numa.remote_events_per_second is derived
from Linux node numastat remote/foreign counters and is a locality signal, not
a direct latency measurement. numa.node_hugepages lists the per-node hugetlb
pool (node, page size, total, and free counts) read from sysfs when exposed.
memory.hugepages_total/hugepages_free/hugepage_size_bytes/hugetlb_used_bytes
report the global pool from `/proc/meminfo` when hugepages are configured.

system contains cpu_governor, THP policy, swappiness, current process limits,
PID 1 host_limits, selected sysctls for overcommit, dirty-page ratios, and the
NMI watchdog, plus bounded kernel_events counts and recent matching log lines.
The event scan reads only the tails of existing text logs under `/var/log`; it
does not invoke `dmesg`, read kernel message streams, traverse the journal, or
write anything. collector_errors lists non-fatal source failures.

### Thermal

thermal.thermal_zones is a list of kernel thermal zones with name, type,
current_celsius, critical_celsius, passive_celsius, policy, and mode.
thermal.temperature_sensors is a list of hwmon temperature sensors with
device name, sensor index, label, source (the hwmon `name`), kind
(cpu/gpu/disk/board), current/max/critical_celsius, alarm, and pci (the PCI
address backing the device when resolvable).
thermal.fans is a list of hwmon fans with device name, sensor index, label,
source, input_rpm/min_rpm/max_rpm, and pci.
thermal.power_sensors is a list of hwmon power/energy inputs with device name,
sensor index, label, source, kind, input_watts, cap_watts, cap_max_watts,
input_joules, alarm, and pci. powerN_input values are stored in watts and
energyN_input values in joules; a driver may expose either or both. Absent
values use omitempty and are not proof that the sensor does not exist.

## 7. Applying findings safely

The tool never applies changes. Use this workflow:

1. Save a JSON report before changing anything.
2. Confirm the finding repeats over a suitable interval.
3. Check collector_errors and source availability.
4. Correlate with Proxmox task history, libvirt XML, QEMU configuration,
   guest metrics, kernel logs, and vendor tools.
5. Change one relevant item through the normal management system with rollback
   and maintenance procedures ready.
6. Collect another report and compare metrics and workload behavior.

| Finding | Investigate first | Potential improvement area |
| --- | --- | --- |
| CPU pressure | guest demand, scheduling, load, pinning | rebalance guests, reduce unnecessary vCPUs, add capacity |
| High iowait | backing-device latency, queueing, storage path | fix contention or storage design |
| Memory/swap pressure | available memory, swap, cgroups, balloon state | reduce overcommit, reclaim safely, add RAM, resize guests |
| VM cgroup near limit | cgroup limit versus guest allocation | correct platform memory policy |
| NIC errors/drops | time trend, driver/firmware, optics/cabling, switch | repair link or correct network design |
| PCIe downgrade | path bottleneck, slot, bifurcation, firmware, AER | correct topology or relocate hardware |
| Paused QEMU | QMP status, task history, guest/device/storage errors | resolve through the platform |
| NUMA issue | host node map, XML policy, placement evidence | correct placement and validate latency |
| NVML unavailable | driver, library path, permissions, GPU support | restore telemetry; PCI identity remains usable |

Do not infer that a finding authorizes a specific sysctl, ethtool, numactl,
Proxmox, libvirt, or QMP command. Such changes can affect guests,
availability, migration, latency, and data integrity.

## 8. Current limitations

Not yet implemented are Redfish/BMC inventory and broad vendor-specific
validation captures. Use platform-native sources when a field is unavailable.

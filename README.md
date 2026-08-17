# Hardware Resources Tool

Hardware Resources Tool is a read-only Linux host diagnostic CLI for physical
and virtualization servers. It compares observed resource use with host
capacity, identifies bottlenecks, and produces advisory findings that an
operator can investigate and apply separately.

The current release is `0.10.3` (`v0.10.3`). It is focused on Linux hosts,
especially KVM/QEMU and Proxmox-style virtualization servers. It does not
change kernel settings, device settings, guest settings, storage, networking,
or QEMU state.

For the complete interpretation guide, see [USERS_MANUAL.md](USERS_MANUAL.md).

## What is implemented

- CPU utilization, load, iowait, interrupts, context switches, and bounded
  sampling rates.
- Memory availability, swap totals, swap activity, memory utilization, and
  selected pressure-related indicators.
- Block-device throughput, operations, and in-flight I/O.
- Capacity filesystem reporting limited to physical non-USB block-backed
  filesystems and currently mounted network filesystems declared in `/etc/fstab`.
  Runtime mounts such as `/run`, `/dev/shm`, `/var/lib/docker`, overlays,
  snap filesystems, pseudo-filesystems, and removable USB storage are filtered.
- Physical NIC throughput, link state, speed, duplex, autonegotiation, driver,
  FEC, supported/advertised/peer modes, queues, rings, errors, and drops.
  Additional read-only GET telemetry includes maximum channel counts, pause
  state, hardware timestamping availability, and PHC index.
  Virtual or device-less interfaces are counted separately rather than being
  presented as physical hardware.
- PCI inventory and read-only capability parsing, negotiated versus maximum
  PCIe links, upstream path bottlenecks, BAR totals, SR-IOV relationships,
  IOMMU groups, AER status, and PCI/bridge NUMA locality.
- SMBIOS memory-device inventory and EDAC corrected/uncorrected counters when
  Linux exposes them.
- NVIDIA PCI identity plus optional dynamically loaded NVML identity, UUID,
  device and running-process framebuffer memory accounting, utilization,
  temperature, power, ECC, and MIG-mode telemetry.
- KVM/QEMU discovery from Proxmox VE VM configuration, `/sys`, libvirt XML, `/proc`, cgroup v2, and bounded
  read-only QMP queries. VM data separates configured allocation, QEMU host
  process usage, cgroup usage, balloon values, guest-reported memory, device
  attachments, and physical NIC correlation.
- Runtime QEMU process memory placement from `smaps_rollup` and `numa_maps`,
  including anonymous hugepages, hugetlb pages, and per-host-node residency.
- Read-only QMP QEMU version, memory-size summary, and vCPU state counts when
  the running QEMU exposes those commands.
- Advisory findings with severity, evidence, and recommendations.

Unavailable kernel files, unsupported devices, absent NVML libraries, missing
libvirt XML, inaccessible cgroups, and unavailable QMP sockets are treated as
unknown or recorded in `collector_errors`; they should not make the program
crash.

## Requirements

Build and run on Linux with:

- Go 1.23 or newer, matching the module toolchain declaration.
- A Linux amd64 host for the release Makefile target. Plain `go build` can be
  used for another Linux architecture if required.
- Root privileges for the diagnostic commands. Root is required so that
  `/proc`, `/sys`, PCI configuration metadata, cgroups, libvirt XML, and QEMU
  process data are consistently visible.
- Optional: NVIDIA drivers and NVML if GPU runtime telemetry is wanted. The
  binary loads NVML dynamically and does not require an NVIDIA SDK at build
  time.

The collector uses direct reads and Linux APIs. It does not shell out to
`virsh`, `ethtool`, `lspci`, `numactl`, `iostat`, or other diagnostic tools.

## Build on Linux

From the repository root:

```sh
go version
go mod download
make linux
```

This produces `hardware-resources-linux-amd64`, embeds the version, git commit,
and UTC build date, applies linker size reduction, and runs `strip --strip-all`
when `strip` is installed. The binary is intentionally ignored by git.

Useful Makefile targets:

```sh
make help       # list targets
make linux      # stripped Linux amd64 release binary
make build      # release-style binary named hardware-resources
make test       # go test ./...
make vet        # go vet ./...
make check      # formatting check, tests, and vet
make coverage   # coverage.out and coverage.html
make install    # install the Linux binary under /usr/local/bin
make clean      # remove generated binaries and coverage files
```

Build variables can be overridden:

```sh
make linux VERSION=0.10.3 LINUX_TARGET=/tmp/hardware-resources-linux-amd64
make install PREFIX=/opt/hardware-resources DESTDIR=/staging
```

Verify the build metadata:

```sh
./hardware-resources-linux-amd64 version
```

## Basic use

Run the installed or locally built binary as root:

```sh
sudo ./hardware-resources-linux-amd64 check
sudo ./hardware-resources-linux-amd64 report
sudo ./hardware-resources-linux-amd64 report --json --duration 5s > report.json
sudo ./hardware-resources-linux-amd64 tui
```

Commands:

- `check` performs a one-second two-sample check and writes a human-readable
  report with findings.
- `report` performs a configurable two-sample collection and writes text by
  default or JSON with `--json`.
- `tui` starts the live terminal dashboard.
- `version` prints release, platform, commit, and build timestamp metadata and
  does not require root.

The report interval is the time between the initial and final samples. A
duration of zero produces a single snapshot, so rate fields may be unavailable
or zero. For useful rates, use at least one second; five seconds is a good
initial diagnostic interval.

Report/check thresholds can be overridden without changing the host:

```sh
sudo ./hardware-resources report \
  --duration 10s \
  --cpu-idle-critical 5 \
  --iowait-warning 10 \
  --memory-used-critical 90 \
  --filesystem-used-warning 80 \
  --filesystem-used-critical 95
```

Threshold flags control findings only; they do not alter collection or system
behavior.

## TUI controls

```sh
sudo ./hardware-resources tui --interval 2s
```

The interval has a 500 ms minimum. The dashboard keeps at most 60 snapshots
for its CPU-idle and memory-used sparklines. Use:

- `1`–`5` to select Overview, Storage, Network, Findings, or Hardware.
- `Tab`, Right arrow, or `l` for the next view.
- `Shift+Tab`, Left arrow, or `h` for the previous view.
- `q` or `Ctrl+C` to exit.

The view renderer clips narrow or short terminals instead of assuming a fixed
terminal size. Collection errors are shown at the bottom of the dashboard.

## Root live capture

The helper builds a Linux binary, reruns itself through `sudo`, and writes
repeatable captures under a private `/tmp` directory:

```sh
./scripts/live-collection-test.sh --duration 5s
```

It creates `metadata.txt`, `summary.txt`, `check.txt`, `report.txt`, and
`report.json`. It prints the result directory, for example
`/tmp/hardware-resources-live.XXXXXX`. The helper is diagnostic only; it does
not install the binary or alter the host.

## Run it now on a Linux host

From the repository root, the recommended first capture is:

```sh
make linux
sudo ./hardware-resources-linux-amd64 check
sudo ./hardware-resources-linux-amd64 report --duration 10s
sudo ./hardware-resources-linux-amd64 report --json --duration 10s > /tmp/hardware-resources-report.json
```

For a complete repeatable capture, use the helper and save the directory it
prints:

```sh
./scripts/live-collection-test.sh --duration 10s
```

Inspect the most useful new telemetry with jq:

```sh
jq '.findings' /tmp/hardware-resources-live.XXXXXX/report.json
jq '.snapshot.networks[] | {name,driver,driver_stats,ethtool_error}' /tmp/hardware-resources-live.XXXXXX/report.json
jq '.snapshot.gpus[] | {address,name,nvml_status,ecc_enabled,ecc_corrected,ecc_uncorrected,mig_enabled,mig_max_instances}' /tmp/hardware-resources-live.XXXXXX/report.json
jq '.snapshot.virtualization.virtual_machines[] | {name,running,qmp_version,qmp_base_memory_bytes,qmp_vcpus,runtime_anon_huge_bytes,runtime_hugetlb_bytes,runtime_numa_bytes}' /tmp/hardware-resources-live.XXXXXX/report.json
```

Replace `XXXXXX` with the actual directory printed by the script. Without
root, the program intentionally exits instead of returning a partial report.
Missing optional NVML, QMP, cgroup, or ethtool features appear as unavailable
fields or collector errors rather than synthetic zero values.

## Output choices

Use text output for an operator’s quick review. Use JSON for automation,
comparison, archival, and detailed field inspection:

```sh
sudo ./hardware-resources report --json --duration 10s > report.json
jq '.findings, .snapshot.virtualization, .snapshot.networks' report.json
```

The JSON top level includes `schema_version`, generation time, collection
duration, `snapshot`, and `findings`. The schema version is currently `1`.
Fields marked with `omitempty` are absent when the source does not provide a
value; an absent field is not proof that the resource does not exist.

## Safety and limitations

All collectors are read-only. Recommendations are advisory and must be
validated against workload requirements, vendor guidance, maintenance policy,
and the virtualization platform before applying changes. The tool never runs
remediation commands such as `sysctl`, `ethtool --set-*`, `numactl`, `virsh`,
QMP mutating commands, or service restarts.

The current backlog still includes ethtool offload/coalescing/RSS/statistics
reads, NVML NVLink and per-instance MIG inventory, Redfish inventory, and
broader vendor integration. See
[docs/TECHNICAL_BACKLOG.md](docs/TECHNICAL_BACKLOG.md) and
[docs/HARDWARE_IMPLEMENTATION.md](docs/HARDWARE_IMPLEMENTATION.md).

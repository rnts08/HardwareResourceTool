# Installation and Privilege Options

Hardware Resources Tool is a Linux-only, read-only diagnostic program. Build
and install it on the Linux host where the resources will be inspected.

## Build and install

Use the Makefile targets:

```sh
make fmt
make check
make linux
sudo make install
```

The default installation path is `/usr/local/bin/hardware-resources`. Use a
different prefix when required:

```sh
make install PREFIX=/opt/hardware-resources
```

The `linux` target builds a Linux amd64 binary and strips it. The binary is
ignored by git and should not be copied into the source repository for a
commit.

Verify the installation:

```sh
/usr/local/bin/hardware-resources version
sudo /usr/local/bin/hardware-resources check
sudo /usr/local/bin/hardware-resources report --json --duration 10s > report.json
```

## Recommended privilege method: sudo

The simplest and safest operational method is to run the installed binary
through `sudo`:

```sh
sudo hardware-resources report
sudo hardware-resources tui --interval 2s
```

For a controlled monitoring account, add a narrowly scoped sudoers rule using
`visudo`:

```sudoers
monitor ALL=(root) NOPASSWD: /usr/local/bin/hardware-resources report *
monitor ALL=(root) NOPASSWD: /usr/local/bin/hardware-resources check *
monitor ALL=(root) NOPASSWD: /usr/local/bin/hardware-resources tui *
```

Keep the executable, its parent directories, and any configuration files
root-owned and non-writable by the monitoring account.

## File capabilities

File capabilities avoid making the entire process setuid root, but they still
grant extra authority and must be reviewed against the host’s security policy.
Start with the smallest capability set that provides the required visibility:

```sh
sudo install -o root -g root -m 0755 hardware-resources \
  /usr/local/libexec/hardware-resources
sudo setcap cap_dac_read_search,cap_sys_ptrace+ep \
  /usr/local/libexec/hardware-resources
getcap /usr/local/libexec/hardware-resources
```

`CAP_DAC_READ_SEARCH` allows traversal and reading of otherwise inaccessible
host files. `CAP_SYS_PTRACE` may be needed for protected `/proc/<pid>` data.
Do not add `CAP_SYS_ADMIN`, `CAP_NET_ADMIN`, or `CAP_SYS_RAWIO` unless a
specific collector has been tested and documented as requiring it.

Remove capabilities with:

```sh
sudo setcap -r /usr/local/libexec/hardware-resources
```

Capabilities may not work on `nosuid` mounts, inside some containers, or under
runtime security policies that strip file capabilities.

## Setuid root: compatibility option, not recommended

The binary can technically be installed setuid root:

```sh
sudo install -o root -g root -m 0755 hardware-resources \
  /usr/local/libexec/hardware-resources
sudo chmod 4755 /usr/local/libexec/hardware-resources
ls -l /usr/local/libexec/hardware-resources
```

The mode should begin with `-rwsr-xr-x`. Remove setuid immediately when it is
no longer needed:

```sh
sudo chmod 0755 /usr/local/libexec/hardware-resources
```

Setuid is not the preferred deployment for this program. A vulnerability in
the binary or any linked library would become a root compromise. The current
collector reads sensitive data including QEMU command lines, cgroups,
`/etc/pve`, QMP sockets, process metadata, and kernel logs. Before approving a
setuid deployment, verify all of the following:

- The binary and every parent directory are owned by root and not writable by
  untrusted users.
- No external commands, plugins, or user-controlled libraries are executed.
- Dynamic libraries, including optional NVML, resolve only from trusted system
  locations.
- QMP paths and input files cannot be redirected through untrusted writable
  locations.
- Kernel-log lines are treated as untrusted text and control characters are
  sanitized before terminal output.
- Privileged collection is separated from unprivileged report rendering where
  practical.

A root-owned systemd service or a small root helper that performs only the
required file reads is preferable to setuid for long-running monitoring.

## Privilege boundaries and safety

The program performs read-only collection. It does not change sysctls, device
settings, QEMU state, guest state, storage, networking, or service state. QMP
queries are limited to read-only commands. Kernel event collection reads
bounded tails of existing text logs; it does not invoke `dmesg`, read
`/proc/kmsg` or `/dev/kmsg`, traverse the journal, or write logs.

Root or elevated access improves visibility into:

- Proxmox VM configuration and QEMU process/cgroup data
- protected `/proc` process metrics
- PCI, AER, IOMMU, and VFIO metadata
- NVML process framebuffer accounting
- root-owned kernel and system logs

If an input is inaccessible, the corresponding field should remain unknown or
be recorded in `collector_errors`; do not broaden privileges automatically.

## Uninstall

Remove the installed binary and any optional privilege grants:

```sh
sudo setcap -r /usr/local/libexec/hardware-resources 2>/dev/null || true
sudo chmod 0755 /usr/local/libexec/hardware-resources 2>/dev/null || true
sudo rm /usr/local/libexec/hardware-resources
sudo rm /usr/local/bin/hardware-resources
```

Remove any corresponding sudoers entry or systemd unit separately. The source
tree, `WORKFILE.md`, and generated binaries are not modified by uninstalling.

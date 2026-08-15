package collect

import (
	"os"
	"path/filepath"
	"testing"

	"hardware-resources-tool/internal/model"
)

func TestCollectVirtualizationMergesLibvirtAndQEMUProcess(t *testing.T) {
	proc := t.TempDir()
	qemu := filepath.Join(proc, "1234")
	if err := os.MkdirAll(qemu, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(qemu, "comm"), []byte("qemu-system-x86_64\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(qemu, "cmdline"), []byte("/usr/bin/qemu-system-x86_64\x00-name\x00guest-a,debug-threads=on\x00-smp\x004\x00-m\x008192\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(qemu, "statm"), []byte("100 10 0 0 0 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(qemu, "cgroup"), []byte("0::machine.slice/guest-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	etc := t.TempDir()
	xmlPath := filepath.Join(etc, "libvirt/qemu/guest-a.xml")
	if err := os.MkdirAll(filepath.Dir(xmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xmlPath, []byte(`<domain><name>guest-a</name><memory unit="MiB">8192</memory><vcpu placement="static">4</vcpu><devices><disk device="disk"><source file="/var/lib/libvirt/images/guest-a.qcow2"/><target dev="vda" bus="virtio"/></disk><interface type="bridge"><mac address="52:54:00:00:00:01"/><source bridge="br0"/><target dev="tap0"/></interface><hostdev type="pci"><source><address domain="0x0000" bus="0x65" slot="0x00" function="0x0"/></source></hostdev></devices></domain>`), 0o644); err != nil {
		t.Fatal(err)
	}
	sys := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sys, "fs/cgroup/machine.slice/guest-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"memory.current": "4294967296\n", "memory.max": "8589934592\n", "io.stat": "8:0 rbytes=1000 wbytes=2000\n"} {
		if err := os.WriteFile(filepath.Join(sys, "fs/cgroup/machine.slice/guest-a", name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := model.Snapshot{CPU: model.CPU{LogicalCPUs: 8}, Memory: model.Memory{TotalBytes: 16 * 1024 * 1024 * 1024}}
	collector := &Collector{procRoot: proc, sysRoot: sys, etcRoot: etc}
	if err := collector.collectVirtualization(&snapshot, &rawCounters{virtualProcesses: map[int]virtualProcessCounter{}}, 0); err != nil {
		t.Fatal(err)
	}
	virt := snapshot.Virtualization
	if !virt.QEMUDetected || virt.Hypervisor != "kvm/qemu" || len(virt.VirtualMachines) != 1 || virt.AllocatedVCPUs != 4 || virt.AllocatedMemoryBytes != 8192*1024*1024 {
		t.Fatalf("unexpected virtualization inventory: %#v", virt)
	}
	if virt.VirtualMachines[0].PID != 1234 || !virt.VirtualMachines[0].Running || virt.VirtualMachines[0].ProcessRSSBytes == 0 {
		t.Fatalf("unexpected VM process data: %#v", virt.VirtualMachines[0])
	}
	vm := virt.VirtualMachines[0]
	if !vm.CgroupAvailable || vm.MemoryCurrentBytes != 4294967296 || vm.ReadBytes != 1000 || vm.WriteBytes != 2000 || len(vm.Disks) != 1 || len(vm.NICs) != 1 || len(vm.PCIAddresses) != 1 {
		t.Fatalf("unexpected deep virtualization data: %#v", vm)
	}
}

func TestQEMUArgumentParsing(t *testing.T) {
	fields := []string{"qemu-system-x86_64", "-smp", "cpus=6,maxcpus=8", "-m", "size=4G"}
	if got := parseQEMUCPU(fields); got != 6 {
		t.Fatalf("vCPUs = %d", got)
	}
	if got := parseQEMUMemory(fields); got != 4*1024*1024*1024 {
		t.Fatalf("memory = %d", got)
	}
}

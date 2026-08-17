package collect

import (
	"os"
	"path/filepath"
	"strings"
)

func splitPath(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
}

func isPseudoFilesystem(fsType string) bool {
	switch fsType {
	case "proc", "sysfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "pstore", "debugfs", "tracefs", "securityfs", "configfs", "efivarfs", "hugetlbfs", "mqueue", "fusectl", "binfmt_misc", "ramfs", "bpf", "autofs", "squashfs", "overlay", "tmpfs", "fuse.snapfuse", "fuse.snapfs", "fuse.portal", "fuse.gvfsd-fuse":
		return true
	default:
		return false
	}
}

func isNetworkFilesystem(fsType string) bool {
	switch strings.ToLower(fsType) {
	case "nfs", "nfs4", "cifs", "smb3", "smbfs", "ceph", "glusterfs", "9p", "afs", "davfs", "fuse.sshfs", "sshfs":
		return true
	default:
		return false
	}
}

func excludedFilesystemMount(mountPoint string) bool {
	clean := filepath.Clean(mountPoint)
	for _, prefix := range []string{"/run", "/dev/shm", "/var/lib/docker"} {
		if clean == prefix || strings.HasPrefix(clean, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// isPhysicalBlockDevice follows device-mapper slaves so a filesystem on a
// logical volume is accepted when its underlying storage is physical. USB
// storage is deliberately rejected because it is removable and unsuitable as
// a stable host-capacity baseline.
func isPhysicalBlockDevice(sysRoot, source string) (physical, usb bool) {
	if !strings.HasPrefix(source, "/dev/") {
		return false, false
	}
	name := filepath.Base(source)
	if name == "" || strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
		return false, false
	}
	visited := map[string]bool{}
	return inspectBlockDevice(sysRoot, name, visited)
}

func inspectBlockDevice(sysRoot, name string, visited map[string]bool) (physical, usb bool) {
	if visited[name] {
		return false, false
	}
	visited[name] = true
	base := filepath.Join(sysRoot, "class/block", name)
	if _, err := os.Lstat(base); err != nil {
		return false, false
	}
	devicePath, err := filepath.EvalSymlinks(filepath.Join(base, "device"))
	if err == nil && strings.Contains(filepath.ToSlash(devicePath), "/usb") {
		return true, true
	}
	physical = err == nil && !strings.Contains(filepath.ToSlash(devicePath), "/virtual/")
	for _, slave := range glob(filepath.Join(base, "slaves/*")) {
		childPhysical, childUSB := inspectBlockDevice(sysRoot, filepath.Base(slave), visited)
		physical = physical || childPhysical
		usb = usb || childUSB
	}
	return physical, usb
}

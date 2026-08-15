package collect

import "strings"

func splitPath(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
}

func isPseudoFilesystem(fsType string) bool {
	switch fsType {
	case "proc", "sysfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "pstore", "debugfs", "tracefs", "securityfs", "configfs", "efivarfs", "hugetlbfs", "mqueue", "fusectl", "binfmt_misc", "ramfs", "bpf", "autofs", "squashfs", "fuse.portal", "fuse.gvfsd-fuse":
		return true
	default:
		return false
	}
}

//go:build linux

package collect

import (
	"os"
	"path/filepath"
)

func networkDeviceInfo(sysRoot, name string) (bool, string) {
	path := filepath.Join(sysRoot, "class/net", name, "device")
	if _, err := os.Stat(path); err != nil {
		return false, ""
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return true, ""
	}
	for _, part := range splitPath(resolved) {
		if pciAddressPattern.MatchString(part) {
			return true, part
		}
	}
	return true, ""
}

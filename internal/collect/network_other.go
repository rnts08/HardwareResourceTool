//go:build !linux

package collect

func networkDeviceInfo(string, string) (bool, string) { return true, "" }

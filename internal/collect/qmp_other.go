//go:build !linux

package collect

func queryQMP(string) (string, uint64, uint64, bool) { return "", 0, 0, false }

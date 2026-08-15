//go:build !linux

package collect

func networkRingSizes(string) (int64, int64) { return 0, 0 }

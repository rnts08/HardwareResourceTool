//go:build linux && cgo

package collect

import "testing"

func TestNvlinkNominalGBps(t *testing.T) {
	cases := map[int]int64{
		1: 20, 2: 25, 3: 50, 4: 100,
		0: 0, 5: 0, -1: 0,
	}
	for version, want := range cases {
		if got := nvlinkNominalGBps(version); got != want {
			t.Errorf("nvlinkNominalGBps(%d) = %d, want %d", version, got, want)
		}
	}
}

func TestNvmlNvLinkDeviceTypeName(t *testing.T) {
	cases := map[uint32]string{
		0: "gpu", 1: "switch", 2: "unknown", 99: "unknown",
	}
	for remoteType, want := range cases {
		if got := nvmlNvLinkDeviceTypeName(remoteType); got != want {
			t.Errorf("nvmlNvLinkDeviceTypeName(%d) = %q, want %q", remoteType, got, want)
		}
	}
}

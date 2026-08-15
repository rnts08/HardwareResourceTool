//go:build linux

package collect

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	siocEthtool       = 0x8946
	ethtoolGRingParam = 0x00000010
)

// networkRingSizes queries the current RX/TX ring sizes without invoking an
// external ethtool process. Unsupported virtual devices simply return zeros.
func networkRingSizes(name string) (int64, int64) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return 0, 0
	}
	defer unix.Close(fd)

	var ring ethtoolRingParam
	ring.Command = ethtoolGRingParam
	var request ifreqData
	copy(request.Name[:], name)
	request.Data = uintptr(unsafe.Pointer(&ring))
	_, _, errno := unix.Syscall6(unix.SYS_IOCTL, uintptr(fd), siocEthtool, uintptr(unsafe.Pointer(&request)), 0, 0, 0)
	if errno != 0 {
		return 0, 0
	}
	return int64(ring.RXPending), int64(ring.TXPending)
}

type ifreqData struct {
	Name [unix.IFNAMSIZ]byte
	Data uintptr
}

type ethtoolRingParam struct {
	Command        uint32
	RXMaxPending   uint32
	RXMiniMax      uint32
	RXJumboMax     uint32
	TXMaxPending   uint32
	RXPending      uint32
	RXMiniPending  uint32
	RXJumboPending uint32
	TXPending      uint32
}

//go:build linux

package collect

import (
	"encoding/binary"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	siocEthtool       = 0x8946
	ethtoolGRingParam = 0x00000010
	ethtoolGChannels  = 0x0000003c
	ethtoolGPause     = 0x00000012
	ethtoolGTsInfo    = 0x00000041
	ethtoolGDrvInfo   = 0x00000003
	ethtoolGStrings   = 0x0000001b
	ethtoolGStats     = 0x0000001d
	ethSSStats        = 1
)

type ethtoolReadOnly struct {
	MaxRXChannels int64
	MaxTXChannels int64
	MaxCombined   int64
	PauseAutoneg  bool
	RXPause       bool
	TXPause       bool
	Timestamping  bool
	PHCIndex      int64
	Error         string
	DriverStats   map[string]uint64
}

// readEthtoolReadOnly uses only ETHTOOL_G* ioctls. It deliberately does not
// expose or call any ETHTOOL_S* operation.
func readEthtoolReadOnly(name string) ethtoolReadOnly {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return ethtoolReadOnly{}
	}
	defer unix.Close(fd)
	result := ethtoolReadOnly{}
	var channels ethtoolChannels
	if err := ethtoolIoctl(fd, name, &channels); err == nil {
		result.MaxRXChannels = int64(channels.MaxRX)
		result.MaxTXChannels = int64(channels.MaxTX)
		result.MaxCombined = int64(channels.MaxCombined)
	} else {
		result.Error = appendEtHToolError(result.Error, "channels", err)
	}
	var pause ethtoolPauseParam
	if err := ethtoolIoctl(fd, name, &pause); err == nil {
		result.PauseAutoneg = pause.Autoneg != 0
		result.RXPause = pause.RXPause != 0
		result.TXPause = pause.TXPause != 0
	} else {
		result.Error = appendEtHToolError(result.Error, "pause", err)
	}
	var timestamp ethtoolTSInfo
	if err := ethtoolIoctl(fd, name, &timestamp); err == nil {
		result.Timestamping = timestamp.SoTimestamping != 0 || timestamp.TxTypes != 0 || timestamp.RxFilters != 0
		result.PHCIndex = int64(timestamp.PHCIndex)
	} else {
		result.Error = appendEtHToolError(result.Error, "timestamping", err)
	}
	result.DriverStats, err = readEthtoolStats(fd, name)
	if err != nil {
		result.Error = appendEtHToolError(result.Error, "driver stats", err)
	}
	return result
}

func readEthtoolStats(fd int, name string) (map[string]uint64, error) {
	info := make([]byte, 256)
	binary.LittleEndian.PutUint32(info[0:4], ethtoolGDrvInfo)
	if err := ethtoolRawIoctl(fd, name, &info[0]); err != nil {
		return nil, err
	}
	if len(info) < 184 {
		return nil, unix.EINVAL
	}
	count := binary.LittleEndian.Uint32(info[180:184])
	if count == 0 {
		return nil, nil
	}
	if count > 4096 {
		return nil, unix.E2BIG
	}
	stringsBuf := make([]byte, 12+int(count)*32)
	binary.LittleEndian.PutUint32(stringsBuf[0:4], ethtoolGStrings)
	binary.LittleEndian.PutUint32(stringsBuf[4:8], ethSSStats)
	binary.LittleEndian.PutUint32(stringsBuf[8:12], count)
	if err := ethtoolRawIoctl(fd, name, &stringsBuf[0]); err != nil {
		return nil, err
	}
	valuesBuf := make([]byte, 8+int(count)*8)
	binary.LittleEndian.PutUint32(valuesBuf[0:4], ethtoolGStats)
	binary.LittleEndian.PutUint32(valuesBuf[4:8], count)
	if err := ethtoolRawIoctl(fd, name, &valuesBuf[0]); err != nil {
		return nil, err
	}
	stats := make(map[string]uint64)
	for i := uint32(0); i < count; i++ {
		nameStart := 12 + int(i)*32
		statName := strings.TrimRight(string(stringsBuf[nameStart:nameStart+32]), "\x00")
		if statName == "" {
			continue
		}
		stats[statName] = binary.LittleEndian.Uint64(valuesBuf[8+int(i)*8 : 16+int(i)*8])
	}
	return stats, nil
}

func ethtoolRawIoctl(fd int, name string, data *byte) error {
	var request ifreqData
	copy(request.Name[:], name)
	request.Data = uintptr(unsafe.Pointer(data))
	_, _, errno := unix.Syscall6(unix.SYS_IOCTL, uintptr(fd), siocEthtool, uintptr(unsafe.Pointer(&request)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func ethtoolIoctl(fd int, name string, data interface{ commandSet() }) error {
	data.commandSet()
	var request ifreqData
	copy(request.Name[:], name)
	request.Data = uintptr(unsafe.Pointer(dataPointer(data)))
	_, _, errno := unix.Syscall6(unix.SYS_IOCTL, uintptr(fd), siocEthtool, uintptr(unsafe.Pointer(&request)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// dataPointer and command plumbing keep the ioctl request layout explicit
// while allowing the read-only helper to share one syscall path.
func dataPointer(data interface{ commandSet() }) unsafe.Pointer {
	switch value := data.(type) {
	case *ethtoolChannels:
		return unsafe.Pointer(value)
	case *ethtoolPauseParam:
		return unsafe.Pointer(value)
	case *ethtoolTSInfo:
		return unsafe.Pointer(value)
	default:
		return nil
	}
}

type ethtoolChannels struct {
	Command     uint32
	MaxRX       uint32
	MaxTX       uint32
	MaxOther    uint32
	MaxCombined uint32
	RX          uint32
	TX          uint32
	Other       uint32
	Combined    uint32
}

func (*ethtoolChannels) command() uint32 { return ethtoolGChannels }
func (v *ethtoolChannels) commandSet()   { v.Command = ethtoolGChannels }

type ethtoolPauseParam struct {
	Command uint32
	Autoneg uint32
	RXPause uint32
	TXPause uint32
}

func (*ethtoolPauseParam) command() uint32 { return ethtoolGPause }
func (v *ethtoolPauseParam) commandSet()   { v.Command = ethtoolGPause }

type ethtoolTSInfo struct {
	Command        uint32
	SoTimestamping uint32
	PHCIndex       int32
	TxTypes        uint32
	TxReserved     [3]uint32
	RxFilters      uint32
}

func (*ethtoolTSInfo) command() uint32 { return ethtoolGTsInfo }
func (v *ethtoolTSInfo) commandSet()   { v.Command = ethtoolGTsInfo }

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

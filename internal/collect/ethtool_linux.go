//go:build linux

package collect

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	siocEthtool          = 0x8946
	ethtoolGSettings     = 0x00000001
	ethtoolGRingParam    = 0x00000010
	ethtoolGChannels     = 0x0000003c
	ethtoolGPause        = 0x00000012
	ethtoolGTsInfo       = 0x00000041
	ethtoolGDrvInfo      = 0x00000003
	ethtoolGStrings      = 0x0000001b
	ethtoolGStats        = 0x0000001d
	ethtoolGCoalesce     = 0x0000000e
	ethtoolGSSetInfo     = 0x00000037
	ethtoolGFeatures     = 0x0000003a
	ethtoolGRSSH         = 0x0000003e
	ethSSStats           = 1
	ethSSFeatures        = 4
	ethStringLen         = 32
	ethtoolFeatureBlocks = 4
	ethRXFHIndirSizeMax  = 2048
	ethRXFHKeySizeMax    = 40
)

type ethtoolFeatures struct {
	Active   []string
	Wanted   []string
	Hardware []string
	NoChange []string
}

type ethtoolReadOnly struct {
	MaxRXChannels       int64
	MaxTXChannels       int64
	MaxCombined         int64
	PauseAutoneg        bool
	RXPause             bool
	TXPause             bool
	Timestamping        bool
	PHCIndex            int64
	Driver              string
	DriverVersion       string
	FWVersion           string
	BusInfo             string
	LinkPort            string
	Transceiver         string
	PHYAddress          int64
	TPMDIX              string
	Features            ethtoolFeatures
	CoalesceRXUsecs     int64
	CoalesceTXUsecs     int64
	CoalesceRXMaxFrames int64
	CoalesceTXMaxFrames int64
	CoalesceAdaptiveRX  bool
	CoalesceAdaptiveTX  bool
	RSSHashFunc         string
	RSSIndirSize        int64
	RSSKeySize          int64
	Error               string
	DriverStats         map[string]uint64
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
	if driver, version, fw, bus, err := readEthtoolDriverInfo(fd, name); err == nil {
		result.Driver = driver
		result.DriverVersion = version
		result.FWVersion = fw
		result.BusInfo = bus
	} else {
		result.Error = appendEtHToolError(result.Error, "driver info", err)
	}
	if port, transceiver, mdix, phy, err := readEthtoolSettings(fd, name); err == nil {
		result.LinkPort = port
		result.Transceiver = transceiver
		result.TPMDIX = mdix
		result.PHYAddress = phy
	} else {
		result.Error = appendEtHToolError(result.Error, "settings", err)
	}
	if features, err := readEthtoolFeatures(fd, name); err == nil {
		result.Features = features
	} else {
		result.Error = appendEtHToolError(result.Error, "features", err)
	}
	var coalesce ethtoolCoalesce
	if err := ethtoolIoctl(fd, name, &coalesce); err == nil {
		result.CoalesceRXUsecs = int64(coalesce.RXUsecs)
		result.CoalesceTXUsecs = int64(coalesce.TXUsecs)
		result.CoalesceRXMaxFrames = int64(coalesce.RXMaxFrames)
		result.CoalesceTXMaxFrames = int64(coalesce.TXMaxFrames)
		result.CoalesceAdaptiveRX = coalesce.UseAdaptiveRX != 0
		result.CoalesceAdaptiveTX = coalesce.UseAdaptiveTX != 0
	} else {
		result.Error = appendEtHToolError(result.Error, "coalescing", err)
	}
	if hfunc, indir, key, err := readEthtoolRSSH(fd, name); err == nil {
		result.RSSHashFunc = hfunc
		result.RSSIndirSize = indir
		result.RSSKeySize = key
	} else {
		result.Error = appendEtHToolError(result.Error, "rss", err)
	}
	result.DriverStats, err = readEthtoolStats(fd, name)
	if err != nil {
		result.Error = appendEtHToolError(result.Error, "driver stats", err)
	}
	return result
}

// readEthtoolDriverInfo reads struct ethtool_drvinfo through a raw buffer so
// the string fields are trimmed without requiring a full struct mirror.
func readEthtoolDriverInfo(fd int, name string) (driver, version, fwVersion, busInfo string, err error) {
	info := make([]byte, 256)
	binary.LittleEndian.PutUint32(info[0:4], ethtoolGDrvInfo)
	if err := ethtoolRawIoctl(fd, name, &info[0]); err != nil {
		return "", "", "", "", err
	}
	if len(info) < 132 {
		return "", "", "", "", unix.EINVAL
	}
	return strings.TrimRight(string(info[4:36]), "\x00"),
		strings.TrimRight(string(info[36:68]), "\x00"),
		strings.TrimRight(string(info[68:100]), "\x00"),
		strings.TrimRight(string(info[100:132]), "\x00"), nil
}

// readEthtoolSettings reads struct ethtool_cmd through ETHTOOL_GSET and
// retains the port, PHY address, transceiver, and MDI/MDIX state. Speed and
// autoneg remain sourced from the generic-netlink link-mode read.
func readEthtoolSettings(fd int, name string) (port, transceiver, mdix string, phyAddress int64, err error) {
	cmd := make([]byte, 44)
	binary.LittleEndian.PutUint32(cmd[0:4], ethtoolGSettings)
	if err := ethtoolRawIoctl(fd, name, &cmd[0]); err != nil {
		return "", "", "", 0, err
	}
	return ethtoolPortName(cmd[15]),
		ethtoolTransceiverName(cmd[17]),
		ethtoolMDIXName(cmd[30]),
		int64(cmd[16]), nil
}

func ethtoolPortName(port byte) string {
	switch port {
	case 0x00:
		return "twisted-pair"
	case 0x01:
		return "aui"
	case 0x02:
		return "mii"
	case 0x03:
		return "fibre"
	case 0x04:
		return "bnc"
	case 0x05:
		return "direct-attach"
	case 0xff:
		return "none"
	default:
		return fmt.Sprintf("0x%02x", port)
	}
}

func ethtoolTransceiverName(transceiver byte) string {
	switch transceiver {
	case 0x00:
		return "internal"
	case 0x01:
		return "external"
	default:
		return fmt.Sprintf("0x%02x", transceiver)
	}
}

func ethtoolMDIXName(mdix byte) string {
	switch mdix {
	case 0x00:
		return "invalid"
	case 0x01:
		return "mdi"
	case 0x02:
		return "mdi-x"
	case 0x03:
		return "auto"
	default:
		return fmt.Sprintf("0x%02x", mdix)
	}
}

// readEthtoolFeatures resolves ETH_SS_FEATURES names and maps the
// available/requested/active/never-changed bit blocks onto them.
func readEthtoolFeatures(fd int, name string) (ethtoolFeatures, error) {
	names, err := ethtoolFeatureNames(fd, name)
	if err != nil {
		return ethtoolFeatures{}, err
	}
	buf := make([]byte, 8+ethtoolFeatureBlocks*16)
	binary.LittleEndian.PutUint32(buf[0:4], ethtoolGFeatures)
	binary.LittleEndian.PutUint32(buf[4:8], ethtoolFeatureBlocks)
	if err := ethtoolRawIoctl(fd, name, &buf[0]); err != nil {
		return ethtoolFeatures{}, err
	}
	blocks := binary.LittleEndian.Uint32(buf[4:8])
	if blocks > ethtoolFeatureBlocks {
		blocks = ethtoolFeatureBlocks
	}
	return parseFeatureBlocks(buf[8:8+int(blocks)*16], names), nil
}

// parseFeatureBlocks decodes the ETHTOOL_GFEATURES block array returned by the
// kernel. Each block carries 32 features; the bit index within a block is the
// feature's position in the ETH_SS_FEATURES name set.
func parseFeatureBlocks(blocks []byte, names []string) ethtoolFeatures {
	var features ethtoolFeatures
	for b := 0; b*16+16 <= len(blocks); b++ {
		off := b * 16
		available := binary.LittleEndian.Uint32(blocks[off : off+4])
		requested := binary.LittleEndian.Uint32(blocks[off+4 : off+8])
		active := binary.LittleEndian.Uint32(blocks[off+8 : off+12])
		neverChanged := binary.LittleEndian.Uint32(blocks[off+12 : off+16])
		for i := 0; i < 32; i++ {
			idx := b*32 + i
			if idx >= len(names) {
				break
			}
			bit := uint32(1) << uint(i)
			if available&bit != 0 {
				features.Hardware = append(features.Hardware, names[idx])
			}
			if requested&bit != 0 {
				features.Wanted = append(features.Wanted, names[idx])
			}
			if active&bit != 0 {
				features.Active = append(features.Active, names[idx])
			}
			if neverChanged&bit != 0 {
				features.NoChange = append(features.NoChange, names[idx])
			}
		}
	}
	return features
}

// ethtoolFeatureNames queries the ETH_SS_FEATURES string set through
// ETHTOOL_GSSET_INFO followed by ETHTOOL_GSTRINGS.
func ethtoolFeatureNames(fd int, name string) ([]string, error) {
	sset := make([]byte, 24)
	binary.LittleEndian.PutUint32(sset[0:4], ethtoolGSSetInfo)
	binary.LittleEndian.PutUint64(sset[8:16], 1<<ethSSFeatures)
	if err := ethtoolRawIoctl(fd, name, &sset[0]); err != nil {
		return nil, err
	}
	count := binary.LittleEndian.Uint32(sset[16:20])
	if count == 0 {
		return nil, nil
	}
	if count > 512 {
		return nil, unix.E2BIG
	}
	stringsBuf := make([]byte, 12+int(count)*ethStringLen)
	binary.LittleEndian.PutUint32(stringsBuf[0:4], ethtoolGStrings)
	binary.LittleEndian.PutUint32(stringsBuf[4:8], ethSSFeatures)
	binary.LittleEndian.PutUint32(stringsBuf[8:12], count)
	if err := ethtoolRawIoctl(fd, name, &stringsBuf[0]); err != nil {
		return nil, err
	}
	return extractFeatureNames(stringsBuf, int(count)), nil
}

// extractFeatureNames trims the fixed-width GSTRINGS payload into a name list.
func extractFeatureNames(buf []byte, count int) []string {
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		start := 12 + i*ethStringLen
		if start+ethStringLen > len(buf) {
			break
		}
		names = append(names, strings.TrimRight(string(buf[start:start+ethStringLen]), "\x00"))
	}
	return names
}

// readEthtoolRSSH queries struct ethtool_rxfh. The caller supplies buffers for
// the indirection table and hash key so the kernel can always return the
// actual sizes; only the header fields are retained.
func readEthtoolRSSH(fd int, name string) (string, int64, int64, error) {
	buf := make([]byte, 52+ethRXFHIndirSizeMax*4+ethRXFHKeySizeMax)
	binary.LittleEndian.PutUint32(buf[0:4], ethtoolGRSSH)
	binary.LittleEndian.PutUint32(buf[4:8], 0)
	binary.LittleEndian.PutUint32(buf[8:12], ethRXFHIndirSizeMax)
	binary.LittleEndian.PutUint32(buf[12:16], ethRXFHKeySizeMax)
	if err := ethtoolRawIoctl(fd, name, &buf[0]); err != nil {
		return "", 0, 0, err
	}
	return rssHashFuncName(buf[16]), int64(binary.LittleEndian.Uint32(buf[8:12])), int64(binary.LittleEndian.Uint32(buf[12:16])), nil
}

func rssHashFuncName(hfunc byte) string {
	switch hfunc {
	case 0x01:
		return "toeplitz"
	case 0x02:
		return "xor"
	case 0x04:
		return "crc32"
	default:
		return fmt.Sprintf("0x%02x", hfunc)
	}
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
	case *ethtoolCoalesce:
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

// ethtoolCoalesce mirrors struct ethtool_coalesce. Only ETHTOOL_GCOALESCE is
// issued; the request is never mutated toward the device.
type ethtoolCoalesce struct {
	Command            uint32
	RXMaxFrames        uint32
	RXMaxFramesIRQ     uint32
	TXMaxFrames        uint32
	TXMaxFramesIRQ     uint32
	RXUsecs            uint32
	RXUsecsIRQ         uint32
	TXUsecs            uint32
	TXUsecsIRQ         uint32
	StatsBlockUsecs    uint32
	UseAdaptiveRX      uint32
	UseAdaptiveTX      uint32
	PktRateLow         uint32
	RXUsecsLow         uint32
	RXMaxFramesLow     uint32
	TXUsecsLow         uint32
	TXMaxFramesLow     uint32
	PktRateHigh        uint32
	RXUsecsHigh        uint32
	RXMaxFramesHigh    uint32
	TXUsecsHigh        uint32
	TXMaxFramesHigh    uint32
	RateSampleInterval uint32
}

func (*ethtoolCoalesce) command() uint32 { return ethtoolGCoalesce }
func (v *ethtoolCoalesce) commandSet()   { v.Command = ethtoolGCoalesce }

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

//go:build linux

package collect

import (
	"fmt"

	"github.com/mdlayher/ethtool"
)

type ethtoolData struct {
	Duplex              string
	Autoneg             string
	LinkUp              bool
	Supported           []string
	Advertised          []string
	Peer                []string
	FECActive           string
	FECSupported        string
	MaxRXChannels       int64
	MaxTXChannels       int64
	MaxCombinedChannels int64
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
	DriverStats         map[string]uint64
	Error               string
}

// enrichNetworks performs one read-only generic-netlink client session for a
// collection. The package exposes the kernel's "ours" link-mode set, which is
// the interface's advertised/capable set; it is retained in Supported and
// Advertised for compatibility with common ethtool terminology.
func enrichNetworks(names []string) map[string]ethtoolData {
	result := make(map[string]ethtoolData, len(names))
	client, err := ethtool.New()
	if err != nil {
		for _, name := range names {
			result[name] = ethtoolData{Error: err.Error()}
		}
		return result
	}
	defer client.Close()

	for _, name := range names {
		data := ethtoolData{}
		readOnly := readEthtoolReadOnly(name)
		data.MaxRXChannels = readOnly.MaxRXChannels
		data.MaxTXChannels = readOnly.MaxTXChannels
		data.MaxCombinedChannels = readOnly.MaxCombined
		data.PauseAutoneg = readOnly.PauseAutoneg
		data.RXPause = readOnly.RXPause
		data.TXPause = readOnly.TXPause
		data.Timestamping = readOnly.Timestamping
		data.PHCIndex = readOnly.PHCIndex
		data.Driver = readOnly.Driver
		data.DriverVersion = readOnly.DriverVersion
		data.FWVersion = readOnly.FWVersion
		data.BusInfo = readOnly.BusInfo
		data.LinkPort = readOnly.LinkPort
		data.Transceiver = readOnly.Transceiver
		data.PHYAddress = readOnly.PHYAddress
		data.TPMDIX = readOnly.TPMDIX
		data.Features = readOnly.Features
		data.CoalesceRXUsecs = readOnly.CoalesceRXUsecs
		data.CoalesceTXUsecs = readOnly.CoalesceTXUsecs
		data.CoalesceRXMaxFrames = readOnly.CoalesceRXMaxFrames
		data.CoalesceTXMaxFrames = readOnly.CoalesceTXMaxFrames
		data.CoalesceAdaptiveRX = readOnly.CoalesceAdaptiveRX
		data.CoalesceAdaptiveTX = readOnly.CoalesceAdaptiveTX
		data.RSSHashFunc = readOnly.RSSHashFunc
		data.RSSIndirSize = readOnly.RSSIndirSize
		data.RSSKeySize = readOnly.RSSKeySize
		data.DriverStats = readOnly.DriverStats
		if readOnly.Error != "" {
			data.Error = appendEtHToolError(data.Error, "read-only details", fmt.Errorf("%s", readOnly.Error))
		}
		iface := ethtool.Interface{Name: name}
		if mode, modeErr := client.LinkMode(iface); modeErr == nil {
			data.Duplex = mode.Duplex.String()
			data.Autoneg = mode.Autoneg.String()
			data.Supported = linkModeNames(mode.Ours)
			data.Advertised = append([]string(nil), data.Supported...)
			data.Peer = linkModeNames(mode.Peer)
		} else {
			data.Error = modeErr.Error()
		}
		if state, stateErr := client.LinkState(iface); stateErr == nil {
			data.LinkUp = state.Link
		} else {
			data.Error = appendEtHToolError(data.Error, "link state", stateErr)
		}
		if fec, fecErr := client.FEC(iface); fecErr == nil {
			data.FECActive = fec.Active.String()
			data.FECSupported = fec.Modes.String()
		} else {
			data.Error = appendEtHToolError(data.Error, "fec", fecErr)
		}
		result[name] = data
	}
	return result
}

func linkModeNames(modes []ethtool.AdvertisedLinkMode) []string {
	if len(modes) == 0 {
		return nil
	}
	result := make([]string, 0, len(modes))
	for _, mode := range modes {
		if mode.Name != "" {
			result = append(result, mode.Name)
		}
	}
	return result
}

func appendEtHToolError(existing, operation string, err error) string {
	if err == nil {
		return existing
	}
	message := fmt.Sprintf("%s: %v", operation, err)
	if existing == "" {
		return message
	}
	return existing + "; " + message
}

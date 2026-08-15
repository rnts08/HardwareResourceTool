//go:build linux

package collect

import (
	"fmt"

	"github.com/mdlayher/ethtool"
)

type ethtoolData struct {
	Duplex       string
	Autoneg      string
	LinkUp       bool
	Supported    []string
	Advertised   []string
	Peer         []string
	FECActive    string
	FECSupported string
	Error        string
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

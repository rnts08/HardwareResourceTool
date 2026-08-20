//go:build !linux

package collect

type ethtoolFeatures struct {
	Active   []string
	Wanted   []string
	Hardware []string
	NoChange []string
}

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

func enrichNetworks(names []string) map[string]ethtoolData {
	result := make(map[string]ethtoolData, len(names))
	for _, name := range names {
		result[name] = ethtoolData{}
	}
	return result
}

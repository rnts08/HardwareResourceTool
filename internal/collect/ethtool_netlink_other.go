//go:build !linux

package collect

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

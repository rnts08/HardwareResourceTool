//go:build !linux

package collect

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

func enrichNetworks(names []string) map[string]ethtoolData {
	result := make(map[string]ethtoolData, len(names))
	for _, name := range names {
		result[name] = ethtoolData{}
	}
	return result
}

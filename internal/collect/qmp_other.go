//go:build !linux

package collect

import "fmt"

type qmpBalloonData struct {
	status, source, version              string
	actual, target, committed, available uint64
	baseMemory, pluggedMemory            uint64
	vcpus, enabledVCPUs                  int64
	reported, guestReport                bool
}

func queryQMP(string) (qmpBalloonData, bool) { return qmpBalloonData{}, false }

func qmpVersion(value map[string]interface{}) string {
	major, majorOK := value["major"].(float64)
	minor, minorOK := value["minor"].(float64)
	micro, microOK := value["micro"].(float64)
	if !majorOK || !minorOK || !microOK || major < 0 || minor < 0 || micro < 0 {
		return ""
	}
	return fmt.Sprintf("%.0f.%.0f.%.0f", major, minor, micro)
}

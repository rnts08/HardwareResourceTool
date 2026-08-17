//go:build linux

package collect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type qmpBalloonData struct {
	status, source            string
	version                   string
	actual, target            uint64
	committed, available      uint64
	baseMemory, pluggedMemory uint64
	vcpus, enabledVCPUs       int64
	reported, guestReport     bool
}

func queryQMP(path string) (qmpBalloonData, bool) {
	data := qmpBalloonData{}
	if path == "" {
		return data, false
	}
	conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err != nil {
		return data, false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(300 * time.Millisecond))
	reader := bufio.NewReader(conn)
	if _, ok := readQMPMessage(reader); !ok {
		return data, false
	}
	if _, err := writeQMPCommand(conn, reader, "qmp_capabilities"); err != nil {
		return data, false
	}
	if response, err := writeQMPCommand(conn, reader, "query-version"); err == nil {
		if qemu, ok := response["qemu"].(map[string]interface{}); ok {
			data.version = qmpVersion(qemu)
		}
	}
	if response, err := writeQMPCommand(conn, reader, "query-status"); err == nil {
		if value, ok := response["status"].(string); ok {
			data.status = value
		}
	}
	if response, err := writeQMPCommand(conn, reader, "query-balloon"); err == nil {
		if value, ok := qmpUint(response["actual"]); ok {
			data.actual, data.reported = value, true
			data.source = "query-balloon"
		}
		if value, ok := qmpUint(response["target"]); ok {
			data.target = value
		}
	}
	// QEMU 8.2+ can expose the guest's committed and available memory through
	// the Hyper-V balloon status report. It is an optional, read-only query.
	if response, err := writeQMPCommand(conn, reader, "query-hv-balloon-status-report"); err == nil {
		if value, ok := qmpUint(response["committed"]); ok {
			data.committed = value
			data.guestReport = true
			data.reported = true
		}
		if value, ok := qmpUint(response["available"]); ok {
			data.available = value
			data.guestReport = true
			data.reported = true
		}
		if data.guestReport {
			data.source = "query-hv-balloon-status-report"
		}
	}
	if response, err := writeQMPCommand(conn, reader, "query-memory-size-summary"); err == nil {
		data.baseMemory, _ = qmpUint(response["base-memory"])
		data.pluggedMemory, _ = qmpUint(response["plugged-memory"])
	}
	if response, err := writeQMPCommandAny(conn, reader, "query-cpus-fast"); err == nil {
		if cpus, ok := response.([]interface{}); ok {
			data.vcpus = int64(len(cpus))
			for _, item := range cpus {
				if cpu, ok := item.(map[string]interface{}); ok {
					if enabled, ok := cpu["enabled"].(bool); !ok || enabled {
						data.enabledVCPUs++
					}
				}
			}
		}
	}
	return data, data.status != "" || data.reported || data.version != "" || data.baseMemory != 0 || data.vcpus != 0
}

func writeQMPCommand(conn net.Conn, reader *bufio.Reader, command string) (map[string]interface{}, error) {
	message, err := writeQMPCommandAny(conn, reader, command)
	if err != nil {
		return nil, err
	}
	value, ok := message.(map[string]interface{})
	if !ok {
		return nil, net.ErrClosed
	}
	return value, nil
}

func writeQMPCommandAny(conn net.Conn, reader *bufio.Reader, command string) (interface{}, error) {
	payload, _ := json.Marshal(map[string]interface{}{"execute": command})
	payload = append(payload, '\n')
	if _, err := conn.Write(payload); err != nil {
		return nil, err
	}
	for {
		message, ok := readQMPMessage(reader)
		if !ok {
			return nil, net.ErrClosed
		}
		if value, ok := message["return"]; ok {
			return value, nil
		}
		if _, ok := message["error"]; ok {
			return nil, net.ErrClosed
		}
	}
}

func qmpVersion(value map[string]interface{}) string {
	major, majorOK := qmpUint(value["major"])
	minor, minorOK := qmpUint(value["minor"])
	micro, microOK := qmpUint(value["micro"])
	if !majorOK || !minorOK || !microOK {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, micro)
}

func readQMPMessage(reader *bufio.Reader) (map[string]interface{}, bool) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, false
	}
	message := map[string]interface{}{}
	if json.Unmarshal(line, &message) != nil {
		return nil, false
	}
	return message, true
}

func qmpUint(value interface{}) (uint64, bool) {
	number, ok := value.(float64)
	return uint64(number), ok && number >= 0
}

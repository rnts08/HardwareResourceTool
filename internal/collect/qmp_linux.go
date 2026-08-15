//go:build linux

package collect

import (
	"bufio"
	"encoding/json"
	"net"
	"time"
)

type qmpBalloonData struct {
	status, source        string
	actual, target        uint64
	committed, available  uint64
	reported, guestReport bool
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
	return data, data.status != "" || data.reported
}

func writeQMPCommand(conn net.Conn, reader *bufio.Reader, command string) (map[string]interface{}, error) {
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
		if value, ok := message["return"].(map[string]interface{}); ok {
			return value, nil
		}
		if _, ok := message["error"]; ok {
			return nil, net.ErrClosed
		}
	}
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

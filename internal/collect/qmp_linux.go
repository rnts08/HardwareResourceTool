//go:build linux

package collect

import (
	"bufio"
	"encoding/json"
	"net"
	"time"
)

func queryQMP(path string) (string, uint64, uint64, bool) {
	if path == "" {
		return "", 0, 0, false
	}
	conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err != nil {
		return "", 0, 0, false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(300 * time.Millisecond))
	reader := bufio.NewReader(conn)
	if _, ok := readQMPMessage(reader); !ok {
		return "", 0, 0, false
	}
	if _, err := writeQMPCommand(conn, reader, "qmp_capabilities"); err != nil {
		return "", 0, 0, false
	}
	status := ""
	actual, target := uint64(0), uint64(0)
	if response, err := writeQMPCommand(conn, reader, "query-status"); err == nil {
		if value, ok := response["status"].(string); ok {
			status = value
		}
	}
	if response, err := writeQMPCommand(conn, reader, "query-balloon"); err == nil {
		if value, ok := qmpUint(response["actual"]); ok {
			actual = value
		}
		if value, ok := qmpUint(response["target"]); ok {
			target = value
		}
	}
	return status, actual, target, status != "" || actual != 0 || target != 0
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

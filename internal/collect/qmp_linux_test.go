//go:build linux

package collect

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
)

func TestQueryQMPBlockStats(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "qmp.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	responses := map[string]map[string]interface{}{
		"qmp_capabilities": {"return": map[string]interface{}{}},
		"query-version":    {"return": map[string]interface{}{"qemu": map[string]interface{}{"major": float64(8), "minor": float64(2), "micro": float64(0)}}},
		"query-status":     {"return": map[string]interface{}{"status": "running"}},
		"query-balloon":    {"return": map[string]interface{}{}},
		"query-blockstats": {"return": []interface{}{
			map[string]interface{}{
				"device": "drive-virtio0",
				"stats": map[string]interface{}{
					"rd_bytes": float64(1048576), "wr_bytes": float64(2097152),
					"rd_operations": float64(10), "wr_operations": float64(20),
				},
			},
			map[string]interface{}{
				"qdev":      "virtio-disk1",
				"node-name": "drive-node1",
				"stats": map[string]interface{}{
					"rd_bytes": float64(512), "wr_bytes": float64(1024),
					"rd_operations": float64(1), "wr_operations": float64(2),
				},
			},
		}},
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("{\"QMP\": {\"version\": {}}}\n")); err != nil {
			return
		}
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var request map[string]interface{}
			if json.Unmarshal(line, &request) != nil {
				continue
			}
			command, _ := request["execute"].(string)
			response, ok := responses[command]
			if !ok {
				response = map[string]interface{}{"error": map[string]interface{}{"desc": "unsupported command"}}
			}
			payload, _ := json.Marshal(response)
			if _, err := conn.Write(append(payload, '\n')); err != nil {
				return
			}
		}
	}()

	data, ok := queryQMP(socket)
	if !ok {
		t.Fatal("queryQMP reported no usable data")
	}
	if len(data.blockDevices) != 2 {
		t.Fatalf("expected 2 block devices, got %d: %#v", len(data.blockDevices), data.blockDevices)
	}
	first, second := data.blockDevices[0], data.blockDevices[1]
	if first.device != "drive-virtio0" || first.readBytes != 1048576 || first.writeBytes != 2097152 || first.readOps != 10 || first.writeOps != 20 {
		t.Errorf("unexpected first block stat: %#v", first)
	}
	if second.device != "virtio-disk1" || second.nodeName != "drive-node1" || second.readBytes != 512 || second.writeOps != 2 {
		t.Errorf("unexpected second block stat: %#v", second)
	}
}

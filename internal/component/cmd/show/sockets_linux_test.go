//go:build linux

package show

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestParseProcNetTCP(t *testing.T) {
	line := "   0: 0100007F:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 12345 1 0000000000000000 100 0 0 10 0"
	entry := parseProcNetLine(line, "tcp")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry["local-addr"] != "127.0.0.1" {
		t.Errorf("local-addr = %v, want 127.0.0.1", entry["local-addr"])
	}
	if entry["local-port"] != 80 {
		t.Errorf("local-port = %v, want 80", entry["local-port"])
	}
	if entry["remote-addr"] != "0.0.0.0" {
		t.Errorf("remote-addr = %v, want 0.0.0.0", entry["remote-addr"])
	}
	if entry["remote-port"] != 0 {
		t.Errorf("remote-port = %v, want 0", entry["remote-port"])
	}
	if entry["state"] != "LISTEN" {
		t.Errorf("state = %v, want LISTEN", entry["state"])
	}
	if entry["protocol"] != "tcp" {
		t.Errorf("protocol = %v, want tcp", entry["protocol"])
	}
	if entry["tx-queue"] != 0 {
		t.Errorf("tx-queue = %v, want 0", entry["tx-queue"])
	}
	if entry["rx-queue"] != 0 {
		t.Errorf("rx-queue = %v, want 0", entry["rx-queue"])
	}
}

func TestParseProcNetTCP_Short(t *testing.T) {
	entry := parseProcNetLine("too short", "tcp")
	if entry != nil {
		t.Error("expected nil for short line")
	}
}

func TestShowSystemSockets_Wiring(t *testing.T) {
	resp, err := handleShowSystemSockets(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatal("expected map response")
	}
	if _, exists := data["sockets"]; !exists {
		t.Error("missing sockets field")
	}
	if _, exists := data["count"]; !exists {
		t.Error("missing count field")
	}
}

func TestSocketsPortBoundary(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"port-0", []string{"tcp", "port", "0"}, false},
		{"port-65535", []string{"tcp", "port", "65535"}, false},
		{"port-65536", []string{"tcp", "port", "65536"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleShowSystemSockets(nil, tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}
		})
	}
}

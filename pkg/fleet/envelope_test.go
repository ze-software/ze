package fleet

import (
	"encoding/json"
	"testing"
)

func TestConfigFetchRequestMarshal(t *testing.T) {
	// VALIDATES: Config fetch request marshals to JSON with kebab-case keys.
	// PREVENTS: Wrong field names breaking protocol.
	req := ConfigFetchRequest{Version: "abcdef0123456789"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	want := `{"version":"abcdef0123456789"}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestConfigFetchRequestEmpty(t *testing.T) {
	// VALIDATES: Empty version (first boot) marshals correctly.
	// PREVENTS: Omitempty swallowing the version field.
	req := ConfigFetchRequest{Version: ""}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	want := `{"version":""}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestConfigFetchResponseFull(t *testing.T) {
	// VALIDATES: Full config response with version + config marshals correctly.
	// PREVENTS: Missing fields in response.
	resp := ConfigFetchResponse{
		Version: "abcdef0123456789",
		Config:  "cGx1Z2luIHsgfQ==",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["version"] != "abcdef0123456789" {
		t.Fatalf("version: got %v", got["version"])
	}
	if got["config"] != "cGx1Z2luIHsgfQ==" {
		t.Fatalf("config: got %v", got["config"])
	}
}

func TestConfigFetchResponseCurrent(t *testing.T) {
	// VALIDATES: "current" response has status field and no config.
	// PREVENTS: Client receiving empty config when versions match.
	resp := ConfigFetchResponse{Status: "current"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "current" {
		t.Fatalf("status: got %v", got["status"])
	}
}

func TestConfigEnvelopeRoundTrip(t *testing.T) {
	// VALIDATES: Marshal then unmarshal preserves all fields.
	// PREVENTS: Field loss during serialization.
	tests := []struct {
		name string
		resp ConfigFetchResponse
	}{
		{
			name: "full config",
			resp: ConfigFetchResponse{
				Version: "abcdef0123456789",
				Config:  "cGx1Z2luIHsgfQ==",
			},
		},
		{
			name: "current",
			resp: ConfigFetchResponse{
				Status: "current",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got ConfigFetchResponse
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Version != tt.resp.Version {
				t.Errorf("version: got %q, want %q", got.Version, tt.resp.Version)
			}
			if got.Config != tt.resp.Config {
				t.Errorf("config: got %q, want %q", got.Config, tt.resp.Config)
			}
			if got.Status != tt.resp.Status {
				t.Errorf("status: got %q, want %q", got.Status, tt.resp.Status)
			}
		})
	}
}

func TestConfigChangedMarshal(t *testing.T) {
	// VALIDATES: Config-changed notification marshals with kebab-case.
	// PREVENTS: Hub sending wrong field names to client.
	n := ConfigChanged{Version: "abcdef0123456789"}
	data, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	want := `{"version":"abcdef0123456789"}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestConfigAckMarshal(t *testing.T) {
	// VALIDATES: Config-ack marshals with ok and optional error fields.
	// PREVENTS: Missing error message on rejection.
	tests := []struct {
		name string
		ack  ConfigAck
		want string
	}{
		{
			name: "accepted",
			ack:  ConfigAck{Version: "abcdef0123456789", OK: true},
			want: `{"version":"abcdef0123456789","ok":true}`,
		},
		{
			name: "rejected",
			ack:  ConfigAck{Version: "abcdef0123456789", OK: false, Error: "invalid bgp block"},
			want: `{"version":"abcdef0123456789","ok":false,"error":"invalid bgp block"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.ack)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got := string(data)
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestConfigAckRoundTrip(t *testing.T) {
	// VALIDATES: Ack round-trips through JSON preserving all fields.
	// PREVENTS: Error message loss during serialization.
	ack := ConfigAck{Version: "abcdef0123456789", OK: false, Error: "parse error"}
	data, err := json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got ConfigAck
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Version != ack.Version {
		t.Errorf("version: got %q, want %q", got.Version, ack.Version)
	}
	if got.OK != ack.OK {
		t.Errorf("ok: got %v, want %v", got.OK, ack.OK)
	}
	if got.Error != ack.Error {
		t.Errorf("error: got %q, want %q", got.Error, ack.Error)
	}
}

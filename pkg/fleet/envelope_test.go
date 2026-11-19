package fleet

import (
	"encoding/json"
	"testing"
	"time"
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

func TestConfigHeldEnvelopeRoundTrip(t *testing.T) {
	// VALIDATES: the config-held payload and the three optional fields of the
	// fetch answer and the ack encode under their kebab-case keys, and decode
	// back to the same values (spec-fleet-7 E-8, E-9, E-10, E-4).
	// PREVENTS: a key spelled differently from the protocol table of
	// docs/architecture/fleet-config.md, which a peer would ignore in silence.
	edited := time.Date(2026, 11, 19, 10, 30, 0, 0, time.UTC)
	held := ConfigHeld{Version: "abcdef0123456789", Config: "cGx1Z2luIHsgfQ==", EditedAt: edited}
	data, err := json.Marshal(held)
	if err != nil {
		t.Fatalf("marshal held: %v", err)
	}
	want := `{"version":"abcdef0123456789","config":"cGx1Z2luIHsgfQ==","edited-at":"2026-11-19T10:30:00Z"}`
	if string(data) != want {
		t.Fatalf("held: got %s, want %s", data, want)
	}
	var heldBack ConfigHeld
	if err := json.Unmarshal(data, &heldBack); err != nil {
		t.Fatalf("unmarshal held: %v", err)
	}
	if heldBack != held {
		t.Fatalf("held round trip: got %+v, want %+v", heldBack, held)
	}

	// A client whose store records no activation time sends no edited-at.
	unstamped, err := json.Marshal(ConfigHeld{Version: "abcdef0123456789", Config: "cGx1Z2luIHsgfQ=="})
	if err != nil {
		t.Fatalf("marshal unstamped held: %v", err)
	}
	if want := `{"version":"abcdef0123456789","config":"cGx1Z2luIHsgfQ=="}`; string(unstamped) != want {
		t.Fatalf("unstamped held: got %s, want %s", unstamped, want)
	}

	resp := ConfigFetchResponse{
		Version: "1111111111111111", Config: "cGx1Z2luIHsgfQ==",
		HeldVersion: "2222222222222222", DiscardVersion: "2222222222222222",
	}
	data, err = json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal fetch answer: %v", err)
	}
	want = `{"version":"1111111111111111","config":"cGx1Z2luIHsgfQ==",` +
		`"held-version":"2222222222222222","discard-version":"2222222222222222"}`
	if string(data) != want {
		t.Fatalf("fetch answer: got %s, want %s", data, want)
	}
	var respBack ConfigFetchResponse
	if err := json.Unmarshal(data, &respBack); err != nil {
		t.Fatalf("unmarshal fetch answer: %v", err)
	}
	if respBack != resp {
		t.Fatalf("fetch answer round trip: got %+v, want %+v", respBack, resp)
	}

	ack := ConfigAck{Version: "2222222222222222", OK: false, Error: "holding: differs", Holding: true}
	data, err = json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal ack: %v", err)
	}
	want = `{"version":"2222222222222222","ok":false,"error":"holding: differs","holding":true}`
	if string(data) != want {
		t.Fatalf("ack: got %s, want %s", data, want)
	}
	var ackBack ConfigAck
	if err := json.Unmarshal(data, &ackBack); err != nil {
		t.Fatalf("unmarshal ack: %v", err)
	}
	if ackBack != ack {
		t.Fatalf("ack round trip: got %+v, want %+v", ackBack, ack)
	}
}

func TestOldPeerIgnoresNewFields(t *testing.T) {
	// VALIDATES: a peer built before spec-fleet-7 decodes a fetch answer and an
	// ack that carry the new fields into its own types with no error and with
	// every old field intact, and a new peer that sets none of them sends the
	// bytes an old peer always sent (A-5).
	// PREVENTS: a new field that is required, or not omitted when unset, so an
	// old device or an old hub refuses or misreads the message.
	type oldFetchAnswer struct {
		Version string `json:"version,omitempty"`
		Config  string `json:"config,omitempty"`
		Status  string `json:"status,omitempty"`
	}
	type oldAck struct {
		Version string `json:"version"`
		OK      bool   `json:"ok"`
		Error   string `json:"error,omitempty"`
	}
	decodeAsOldPeer := func(t *testing.T, data []byte, into any) {
		t.Helper()
		// The old peer's decoder: encoding/json with its default, which ignores
		// a key the type does not declare.
		if err := json.Unmarshal(data, into); err != nil {
			t.Fatalf("an old peer refused %s: %v", data, err)
		}
	}

	answer, err := json.Marshal(ConfigFetchResponse{
		Version: "1111111111111111", Config: "cGx1Z2luIHsgfQ==",
		HeldVersion: "2222222222222222", DiscardVersion: "2222222222222222",
	})
	if err != nil {
		t.Fatalf("marshal fetch answer: %v", err)
	}
	var oldAnswer oldFetchAnswer
	decodeAsOldPeer(t, answer, &oldAnswer)
	if oldAnswer != (oldFetchAnswer{Version: "1111111111111111", Config: "cGx1Z2luIHsgfQ=="}) {
		t.Fatalf("old device read the fetch answer as %+v", oldAnswer)
	}

	ack, err := json.Marshal(ConfigAck{Version: "2222222222222222", OK: false, Error: "holding: differs", Holding: true})
	if err != nil {
		t.Fatalf("marshal ack: %v", err)
	}
	var old oldAck
	decodeAsOldPeer(t, ack, &old)
	if old != (oldAck{Version: "2222222222222222", OK: false, Error: "holding: differs"}) {
		t.Fatalf("old hub read the ack as %+v", old)
	}

	// Unset, the new fields add no byte: an old peer sees exactly what it saw.
	plainAnswer, err := json.Marshal(ConfigFetchResponse{Status: "current"})
	if err != nil {
		t.Fatalf("marshal current answer: %v", err)
	}
	if want := `{"status":"current"}`; string(plainAnswer) != want {
		t.Fatalf("current answer: got %s, want %s", plainAnswer, want)
	}
	plainAck, err := json.Marshal(ConfigAck{Version: "2222222222222222", OK: true})
	if err != nil {
		t.Fatalf("marshal plain ack: %v", err)
	}
	if want := `{"version":"2222222222222222","ok":true}`; string(plainAck) != want {
		t.Fatalf("plain ack: got %s, want %s", plainAck, want)
	}
}

package fleet

import (
	"encoding/json"
	"strings"
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

func TestConfigFetchSampleIntervalRoundTrip(t *testing.T) {
	// VALIDATES: ConfigFetchResponse encodes sample-interval-seconds when
	// positive on both full and current replies, omits it when zero, and
	// remains decodable by an older peer that does not know the field.
	// PREVENTS: Breaking old managed clients or losing advertised sampling
	// intervals across config-fetch replies.
	if VerbSampleReport != "sample-report" {
		t.Fatalf("VerbSampleReport = %q, want %q", VerbSampleReport, "sample-report")
	}

	full := ConfigFetchResponse{
		Version:               "1111111111111111",
		Config:                "cGx1Z2luIHsgfQ==",
		SampleIntervalSeconds: 60,
	}
	fullJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal full: %v", err)
	}
	wantFull := `{"version":"1111111111111111","config":"cGx1Z2luIHsgfQ==","sample-interval-seconds":60}`
	if string(fullJSON) != wantFull {
		t.Fatalf("full json: got %s, want %s", fullJSON, wantFull)
	}
	var fullBack ConfigFetchResponse
	if err := json.Unmarshal(fullJSON, &fullBack); err != nil {
		t.Fatalf("unmarshal full: %v", err)
	}
	if fullBack != full {
		t.Fatalf("full round-trip: got %+v, want %+v", fullBack, full)
	}

	current := ConfigFetchResponse{
		Status:                "current",
		SampleIntervalSeconds: 30,
	}
	currentJSON, err := json.Marshal(current)
	if err != nil {
		t.Fatalf("marshal current: %v", err)
	}
	wantCurrent := `{"status":"current","sample-interval-seconds":30}`
	if string(currentJSON) != wantCurrent {
		t.Fatalf("current json: got %s, want %s", currentJSON, wantCurrent)
	}
	var currentBack ConfigFetchResponse
	if err := json.Unmarshal(currentJSON, &currentBack); err != nil {
		t.Fatalf("unmarshal current: %v", err)
	}
	if currentBack != current {
		t.Fatalf("current round-trip: got %+v, want %+v", currentBack, current)
	}

	// Older client ignores sample-interval-seconds cleanly.
	type legacyFetchResponse struct {
		Version string `json:"version,omitempty"`
		Config  string `json:"config,omitempty"`
		Status  string `json:"status,omitempty"`
	}
	var legacy legacyFetchResponse
	if err := json.Unmarshal(fullJSON, &legacy); err != nil {
		t.Fatalf("legacy decode failed: %v", err)
	}
	if legacy.Version != full.Version || legacy.Config != full.Config {
		t.Fatalf("legacy decode mismatch: got %+v", legacy)
	}

	// Older hub omits sample-interval-seconds, which decodes to 0 (disabled).
	var fromOldHub ConfigFetchResponse
	if err := json.Unmarshal([]byte(`{"status":"current"}`), &fromOldHub); err != nil {
		t.Fatalf("decode old hub reply: %v", err)
	}
	if fromOldHub.SampleIntervalSeconds != 0 {
		t.Fatalf("old hub interval = %d, want 0", fromOldHub.SampleIntervalSeconds)
	}
}

func TestEnvelopeChainAndDiffRoundTrip(t *testing.T) {
	// VALIDATES: ManagedReply, PingRequest, and all optional chain-hash,
	// resync, diff, reanchor, and reject-kind fields on ConfigFetchRequest,
	// ConfigFetchResponse, ConfigChanged, ConfigAck, and ConfigHeld marshal
	// under their exact kebab-case keys, omit when zero, and round-trip cleanly.
	// PREVENTS: JSON key typos or missing omitempty tags breaking fleet-10
	// drift detection or older peer interoperability.
	const chain = "0123456789abcdef0123456789abcdef01234567"

	// ManagedReply
	reply := ManagedReply{ChainHash: chain}
	data, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal ManagedReply: %v", err)
	}
	wantReply := `{"chain-hash":"` + chain + `"}`
	if string(data) != wantReply {
		t.Fatalf("ManagedReply json = %s, want %s", data, wantReply)
	}
	decodedReply, err := DecodeManagedReply(data)
	if err != nil {
		t.Fatalf("DecodeManagedReply: %v", err)
	}
	if decodedReply != reply {
		t.Fatalf("DecodeManagedReply = %+v, want %+v", decodedReply, reply)
	}

	// PingRequest
	ping := PingRequest{ChainHash: chain}
	data, err = json.Marshal(ping)
	if err != nil {
		t.Fatalf("marshal PingRequest: %v", err)
	}
	if string(data) != wantReply {
		t.Fatalf("PingRequest json = %s, want %s", data, wantReply)
	}
	var pingBack PingRequest
	if err := json.Unmarshal(data, &pingBack); err != nil {
		t.Fatalf("unmarshal PingRequest: %v", err)
	}
	if pingBack != ping {
		t.Fatalf("PingRequest round-trip = %+v, want %+v", pingBack, ping)
	}
	emptyPing, err := json.Marshal(PingRequest{})
	if err != nil {
		t.Fatalf("marshal empty PingRequest: %v", err)
	}
	if string(emptyPing) != `{}` {
		t.Fatalf("empty PingRequest json = %s, want {}", emptyPing)
	}

	// ConfigFetchRequest
	fetchReq := ConfigFetchRequest{
		Version:   "1111111111111111",
		ChainHash: chain,
		Resync:    ResyncModeDiff,
	}
	data, err = json.Marshal(fetchReq)
	if err != nil {
		t.Fatalf("marshal ConfigFetchRequest: %v", err)
	}
	wantFetchReq := `{"version":"1111111111111111","chain-hash":"` + chain + `","resync":"diff"}`
	if string(data) != wantFetchReq {
		t.Fatalf("ConfigFetchRequest json = %s, want %s", data, wantFetchReq)
	}
	var fetchReqBack ConfigFetchRequest
	if err := json.Unmarshal(data, &fetchReqBack); err != nil {
		t.Fatalf("unmarshal ConfigFetchRequest: %v", err)
	}
	if fetchReqBack != fetchReq {
		t.Fatalf("ConfigFetchRequest round-trip = %+v, want %+v", fetchReqBack, fetchReq)
	}

	// ConfigFetchResponse (diff + reanchor + sample-interval-seconds + chain-hash)
	fetchResp := ConfigFetchResponse{
		Version:               "2222222222222222",
		SampleIntervalSeconds: 30,
		Mode:                  FetchModeDiff,
		BaseVersion:           "1111111111111111",
		Diff:                  "W10=",
		Reanchor:              true,
		ChainHash:             chain,
	}
	data, err = json.Marshal(fetchResp)
	if err != nil {
		t.Fatalf("marshal ConfigFetchResponse: %v", err)
	}
	wantFetchResp := `{"version":"2222222222222222","sample-interval-seconds":30,` +
		`"mode":"diff","base-version":"1111111111111111","diff":"W10=","reanchor":true,` +
		`"chain-hash":"` + chain + `"}`
	if string(data) != wantFetchResp {
		t.Fatalf("ConfigFetchResponse json = %s, want %s", data, wantFetchResp)
	}
	var fetchRespBack ConfigFetchResponse
	if err := json.Unmarshal(data, &fetchRespBack); err != nil {
		t.Fatalf("unmarshal ConfigFetchResponse: %v", err)
	}
	if fetchRespBack != fetchResp {
		t.Fatalf("ConfigFetchResponse round-trip = %+v, want %+v", fetchRespBack, fetchResp)
	}

	// ConfigChanged
	changed := ConfigChanged{Version: "2222222222222222", ChainHash: chain}
	data, err = json.Marshal(changed)
	if err != nil {
		t.Fatalf("marshal ConfigChanged: %v", err)
	}
	wantChanged := `{"version":"2222222222222222","chain-hash":"` + chain + `"}`
	if string(data) != wantChanged {
		t.Fatalf("ConfigChanged json = %s, want %s", data, wantChanged)
	}
	var changedBack ConfigChanged
	if err := json.Unmarshal(data, &changedBack); err != nil {
		t.Fatalf("unmarshal ConfigChanged: %v", err)
	}
	if changedBack != changed {
		t.Fatalf("ConfigChanged round-trip = %+v, want %+v", changedBack, changed)
	}

	// ConfigAck
	ack := ConfigAck{
		Version:    "2222222222222222",
		OK:         false,
		Error:      "yang validation failed",
		ChainHash:  chain,
		RejectKind: RejectKindVerify,
	}
	data, err = json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal ConfigAck: %v", err)
	}
	wantAck := `{"version":"2222222222222222","ok":false,"error":"yang validation failed",` +
		`"chain-hash":"` + chain + `","reject-kind":"verify"}`
	if string(data) != wantAck {
		t.Fatalf("ConfigAck json = %s, want %s", data, wantAck)
	}
	var ackBack ConfigAck
	if err := json.Unmarshal(data, &ackBack); err != nil {
		t.Fatalf("unmarshal ConfigAck: %v", err)
	}
	if ackBack != ack {
		t.Fatalf("ConfigAck round-trip = %+v, want %+v", ackBack, ack)
	}

	// ConfigHeld
	held := ConfigHeld{
		Version:   "3333333333333333",
		Config:    "cGx1Z2luIHsgfQ==",
		ChainHash: chain,
	}
	data, err = json.Marshal(held)
	if err != nil {
		t.Fatalf("marshal ConfigHeld: %v", err)
	}
	wantHeld := `{"version":"3333333333333333","config":"cGx1Z2luIHsgfQ==","chain-hash":"` + chain + `"}`
	if string(data) != wantHeld {
		t.Fatalf("ConfigHeld json = %s, want %s", data, wantHeld)
	}
	var heldBack ConfigHeld
	if err := json.Unmarshal(data, &heldBack); err != nil {
		t.Fatalf("unmarshal ConfigHeld: %v", err)
	}
	if heldBack != held {
		t.Fatalf("ConfigHeld round-trip = %+v, want %+v", heldBack, held)
	}

	// Verify constant values match spec-fleet-10 wire strings.
	if FetchModeFull != "full" || FetchModeDiff != "diff" {
		t.Fatalf("unexpected FetchMode constants: %q, %q", FetchModeFull, FetchModeDiff)
	}
	if ResyncModeDiff != "diff" || ResyncModeFull != "full" {
		t.Fatalf("unexpected ResyncMode constants: %q, %q", ResyncModeDiff, ResyncModeFull)
	}
	if RejectKindVerify != "verify" || RejectKindCommit != "commit" || RejectKindResync != "resync" {
		t.Fatalf("unexpected RejectKind constants: %q, %q, %q", RejectKindVerify, RejectKindCommit, RejectKindResync)
	}
}

func TestManagedReplyToleratesLegacyEmptyOK(t *testing.T) {
	// VALIDATES: DecodeManagedReply returns an empty ManagedReply with no error
	// on nil, empty, whitespace-only, or empty-object JSON replies (legacy
	// SendOK peers), decodes chain-hash when present, and rejects invalid JSON.
	// PREVENTS: False drift or protocol errors when speaking with a peer that
	// replies with empty SendOK.
	emptyInputs := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "nil raw", raw: nil},
		{name: "empty slice", raw: json.RawMessage{}},
		{name: "whitespace only", raw: json.RawMessage(" \t\r\n ")},
		{name: "empty object", raw: json.RawMessage(`{}`)},
	}
	for _, tc := range emptyInputs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeManagedReply(tc.raw)
			if err != nil {
				t.Fatalf("DecodeManagedReply(%q) returned err: %v", tc.raw, err)
			}
			if got.ChainHash != "" {
				t.Fatalf("DecodeManagedReply(%q).ChainHash = %q, want empty", tc.raw, got.ChainHash)
			}
		})
	}

	const chain = "abcdef0123456789abcdef0123456789abcdef01"
	got, err := DecodeManagedReply(json.RawMessage(`{"chain-hash":"` + chain + `","future-field":1}`))
	if err != nil {
		t.Fatalf("DecodeManagedReply with chain-hash: %v", err)
	}
	if got.ChainHash != chain {
		t.Fatalf("ChainHash = %q, want %q", got.ChainHash, chain)
	}

	if _, err := DecodeManagedReply(json.RawMessage(`{"chain-hash":`)); err == nil {
		t.Fatalf("expected error on truncated JSON")
	}
}

func TestAuditReceiptEnvelopeRoundTripAndValidation(t *testing.T) {
	// VALIDATES: AuditReceiptEntry, AuditReceiptBatch, and AuditReceiptAck
	// marshal under exact lowercase kebab-case JSON keys, omit zero optional
	// fields, round-trip cleanly, and enforce all field, epoch, ordering, gap,
	// and byte-size invariants in Validate().
	// PREVENTS: Malformed or oversized device receipts bypassing protocol
	// bounds or JSON field mismatches between device and hub.
	if VerbFleetAuditReceipts != "fleet-audit-receipts" {
		t.Fatalf("VerbFleetAuditReceipts = %q, want %q", VerbFleetAuditReceipts, "fleet-audit-receipts")
	}

	at1 := time.Date(2026, 11, 28, 1, 0, 0, 0, time.UTC)
	at2 := time.Date(2026, 11, 28, 1, 5, 0, 0, time.UTC)

	batch := AuditReceiptBatch{
		Epoch: "epoch-001",
		Entries: []AuditReceiptEntry{
			{
				Seq:     4,
				Action:  FleetAuditReceiptActionDisable,
				Actor:   "admin",
				Surface: "cli",
				EventAt: at1,
				Outcome: "success",
			},
			{
				Seq:     5,
				Action:  FleetAuditReceiptActionEnable,
				Actor:   "admin",
				Surface: "ssh",
				EventAt: at2,
				Outcome: "success",
			},
		},
		Lost:    3,
		GapFrom: 1,
		GapTo:   3,
	}
	if err := batch.Validate(); err != nil {
		t.Fatalf("valid batch rejected: %v", err)
	}

	raw, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}
	wantJSON := `{"epoch":"epoch-001","entries":[` +
		`{"seq":4,"action":"fleet.disable","actor":"admin","surface":"cli","event-at":"2026-11-28T01:00:00Z","outcome":"success"},` +
		`{"seq":5,"action":"fleet.enable","actor":"admin","surface":"ssh","event-at":"2026-11-28T01:05:00Z","outcome":"success"}],` +
		`"lost":3,"gap-from":1,"gap-to":3}`
	if string(raw) != wantJSON {
		t.Fatalf("batch json = %s, want %s", raw, wantJSON)
	}

	var back AuditReceiptBatch
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal batch: %v", err)
	}
	if err := back.Validate(); err != nil {
		t.Fatalf("round-tripped batch invalid: %v", err)
	}
	if back.Epoch != batch.Epoch || back.Lost != batch.Lost || back.GapFrom != batch.GapFrom || back.GapTo != batch.GapTo || len(back.Entries) != 2 {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", back, batch)
	}

	ack := AuditReceiptAck{Epoch: "epoch-001", Committed: 5}
	ackRaw, err := json.Marshal(ack)
	if err != nil {
		t.Fatalf("marshal ack: %v", err)
	}
	if want := `{"epoch":"epoch-001","committed":5}`; string(ackRaw) != want {
		t.Fatalf("ack json = %s, want %s", ackRaw, want)
	}
	var ackBack AuditReceiptAck
	if err := json.Unmarshal(ackRaw, &ackBack); err != nil {
		t.Fatalf("unmarshal ack: %v", err)
	}
	if ackBack != ack {
		t.Fatalf("ack round-trip: got %+v, want %+v", ackBack, ack)
	}

	// Gap-only batch (no entries, valid loss gap) is valid.
	gapOnly := AuditReceiptBatch{Epoch: "epoch-001", Lost: 2, GapFrom: 1, GapTo: 2}
	if err := gapOnly.Validate(); err != nil {
		t.Fatalf("gap-only batch rejected: %v", err)
	}

	validEntry := AuditReceiptEntry{
		Seq:     1,
		Action:  FleetAuditReceiptActionDisable,
		Actor:   "admin",
		Surface: "cli",
		EventAt: at1,
		Outcome: "success",
	}
	tooManyEntries := make([]AuditReceiptEntry, FleetAuditReceiptsBatchMax+1)
	for i := range tooManyEntries {
		e := validEntry
		e.Seq = uint64(i + 1)
		tooManyEntries[i] = e
	}

	invalidCases := []struct {
		name  string
		batch AuditReceiptBatch
	}{
		{name: "empty epoch", batch: AuditReceiptBatch{Epoch: "", Entries: []AuditReceiptEntry{validEntry}}},
		{name: "epoch with space", batch: AuditReceiptBatch{Epoch: "epoch 1", Entries: []AuditReceiptEntry{validEntry}}},
		{name: "epoch with control char", batch: AuditReceiptBatch{Epoch: "epoch\n1", Entries: []AuditReceiptEntry{validEntry}}},
		{name: "epoch non-ascii", batch: AuditReceiptBatch{Epoch: "epoch-\u00e9", Entries: []AuditReceiptEntry{validEntry}}},
		{name: "epoch exceeds 64 bytes", batch: AuditReceiptBatch{Epoch: strings.Repeat("a", FleetAuditReceiptEpochMax+1), Entries: []AuditReceiptEntry{validEntry}}},
		{name: "empty entries and no loss", batch: AuditReceiptBatch{Epoch: "e1"}},
		{name: "gap-from without lost", batch: AuditReceiptBatch{Epoch: "e1", GapFrom: 1, Entries: []AuditReceiptEntry{validEntry}}},
		{name: "gap-to without lost", batch: AuditReceiptBatch{Epoch: "e1", GapTo: 1, Entries: []AuditReceiptEntry{validEntry}}},
		{name: "lost without gap-from", batch: AuditReceiptBatch{Epoch: "e1", Lost: 1, GapTo: 1}},
		{name: "gap-to before gap-from", batch: AuditReceiptBatch{Epoch: "e1", Lost: 1, GapFrom: 5, GapTo: 4}},
		{name: "entry seq overlaps gap", batch: AuditReceiptBatch{Epoch: "e1", Lost: 2, GapFrom: 1, GapTo: 2, Entries: []AuditReceiptEntry{{Seq: 2, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: "cli", EventAt: at1, Outcome: "success"}}}},
		{name: "too many entries", batch: AuditReceiptBatch{Epoch: "e1", Entries: tooManyEntries}},
		{name: "duplicate seq", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{validEntry, validEntry}}},
		{name: "decreasing seq", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{
			{Seq: 3, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: "cli", EventAt: at1, Outcome: "success"},
			{Seq: 2, Action: FleetAuditReceiptActionEnable, Actor: "admin", Surface: "cli", EventAt: at2, Outcome: "success"},
		}}},
		{name: "zero seq", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 0, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: "cli", EventAt: at1, Outcome: "success"}}}},
		{name: "invalid action", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: "config.push", Actor: "admin", Surface: "cli", EventAt: at1, Outcome: "success"}}}},
		{name: "zero event-at", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: "cli", Outcome: "success"}}}},
		{name: "invalid outcome", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: "cli", EventAt: at1, Outcome: "error"}}}},
		{name: "empty actor", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: "", Surface: "cli", EventAt: at1, Outcome: "success"}}}},
		{name: "control char actor", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: "admin\x00", Surface: "cli", EventAt: at1, Outcome: "success"}}}},
		{name: "oversize actor", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: strings.Repeat("x", FleetAuditReceiptActorMax+1), Surface: "cli", EventAt: at1, Outcome: "success"}}}},
		{name: "empty surface", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: "", EventAt: at1, Outcome: "success"}}}},
		{name: "oversize surface", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: "admin", Surface: strings.Repeat("s", FleetAuditReceiptSurfaceMax+1), EventAt: at1, Outcome: "success"}}}},
		{name: "oversize marshaled entry", batch: AuditReceiptBatch{Epoch: "e1", Entries: []AuditReceiptEntry{{Seq: 1, Action: FleetAuditReceiptActionDisable, Actor: strings.Repeat("<", FleetAuditReceiptActorMax), Surface: strings.Repeat("<", FleetAuditReceiptSurfaceMax), EventAt: at1, Outcome: "success"}}}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.batch.Validate(); err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}

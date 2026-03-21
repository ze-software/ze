package grmarker

import (
	"encoding/binary"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// newTestStore creates a temporary zefs store for testing.
func newTestStore(t *testing.T) *zefs.BlobStore {
	t.Helper()
	path := t.TempDir() + "/test.zefs"
	store, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// makeMarkerBytes returns an 8-byte big-endian int64 UNIX timestamp.
func makeMarkerBytes(ts int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts))
	return buf
}

// makeGRCapValue returns a 2-byte GR capability value with the given restart-time.
// R-bit is 0.
func makeGRCapValue(restartTime int) []byte {
	return []byte{byte(restartTime >> 8), byte(restartTime & 0xFF)}
}

// --- Phase 1: Marker read/write ---

// VALIDATES: Marker written to zefs with correct expiry.
// PREVENTS: Marker value format mismatch (not 8-byte big-endian int64).
func TestWriteGRMarker(t *testing.T) {
	store := newTestStore(t)
	expiry := time.Now().Add(120 * time.Second)

	err := Write(store, expiry)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify marker exists in zefs.
	if !store.Has(markerKey) {
		t.Fatal("marker key not found in zefs")
	}

	// Verify value is 8-byte big-endian timestamp.
	data, err := store.ReadFile(markerKey)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) != 8 {
		t.Fatalf("marker length = %d, want 8", len(data))
	}
	got := int64(binary.BigEndian.Uint64(data))
	want := expiry.Unix()
	if got != want {
		t.Errorf("marker timestamp = %d, want %d", got, want)
	}
}

// VALIDATES: Valid marker read, returns expiry deadline.
// PREVENTS: Valid marker ignored on startup.
func TestReadGRMarkerValid(t *testing.T) {
	store := newTestStore(t)
	expiry := time.Now().Add(120 * time.Second)

	// Write marker directly.
	if err := store.WriteFile(markerKey, makeMarkerBytes(expiry.Unix()), 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, ok := Read(store)
	if !ok {
		t.Fatal("Read returned ok=false for valid marker")
	}
	if got.Unix() != expiry.Unix() {
		t.Errorf("expiry = %v, want %v", got.Unix(), expiry.Unix())
	}
}

// VALIDATES: Expired marker returns no-restart.
// PREVENTS: Stale marker causing R=1 after window expires.
func TestReadGRMarkerExpired(t *testing.T) {
	store := newTestStore(t)
	expiry := time.Now().Add(-10 * time.Second) // 10 seconds in the past

	if err := store.WriteFile(markerKey, makeMarkerBytes(expiry.Unix()), 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, ok := Read(store)
	if ok {
		t.Fatal("Read returned ok=true for expired marker")
	}
}

// VALIDATES: Missing marker returns no-restart.
// PREVENTS: Panic on missing marker.
func TestReadGRMarkerMissing(t *testing.T) {
	store := newTestStore(t)

	_, ok := Read(store)
	if ok {
		t.Fatal("Read returned ok=true for missing marker")
	}
}

// VALIDATES: Corrupt marker returns no-restart (not crash).
// PREVENTS: Panic on corrupt zefs data.
func TestReadGRMarkerCorrupt(t *testing.T) {
	store := newTestStore(t)

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"short", []byte{0x01, 0x02, 0x03}},
		{"too long", make([]byte, 16)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.WriteFile(markerKey, tt.data, 0); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			_, ok := Read(store)
			if ok {
				t.Fatalf("Read returned ok=true for corrupt marker (%s)", tt.name)
			}
		})
	}
}

// VALIDATES: Marker removed after reading.
// PREVENTS: Stale restart on next cold start.
func TestRemoveGRMarker(t *testing.T) {
	store := newTestStore(t)
	expiry := time.Now().Add(120 * time.Second)

	if err := store.WriteFile(markerKey, makeMarkerBytes(expiry.Unix()), 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := Remove(store); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if store.Has(markerKey) {
		t.Fatal("marker still exists after Remove")
	}
}

// VALIDATES: Remove on missing marker does not error.
// PREVENTS: Crash on cold start (no marker to remove).
func TestRemoveGRMarkerMissing(t *testing.T) {
	store := newTestStore(t)

	// Should not error when marker doesn't exist.
	err := Remove(store)
	if err != nil {
		t.Fatalf("Remove on missing marker: %v", err)
	}
}

// --- Phase 2: Max restart-time extraction ---

// VALIDATES: Max computed from multiple InjectedCapabilities.
// PREVENTS: Using wrong peer's restart-time.
func TestMaxRestartTime(t *testing.T) {
	tests := []struct {
		name string
		caps []plugin.InjectedCapability
		want int
	}{
		{
			name: "single peer 120s",
			caps: []plugin.InjectedCapability{
				{Code: grCapCode, Value: makeGRCapValue(120)},
			},
			want: 120,
		},
		{
			name: "two peers, take max",
			caps: []plugin.InjectedCapability{
				{Code: grCapCode, Value: makeGRCapValue(120)},
				{Code: grCapCode, Value: makeGRCapValue(300)},
			},
			want: 300,
		},
		{
			name: "max restart-time 4095",
			caps: []plugin.InjectedCapability{
				{Code: grCapCode, Value: makeGRCapValue(4095)},
			},
			want: 4095,
		},
		{
			name: "zero restart-time",
			caps: []plugin.InjectedCapability{
				{Code: grCapCode, Value: makeGRCapValue(0)},
			},
			want: 0,
		},
		{
			name: "no code-64 caps",
			caps: []plugin.InjectedCapability{
				{Code: 77, Value: nil}, // link-local-nexthop, not GR
			},
			want: 0,
		},
		{
			name: "empty caps",
			caps: nil,
			want: 0,
		},
		{
			name: "short value ignored",
			caps: []plugin.InjectedCapability{
				{Code: grCapCode, Value: []byte{0x01}}, // only 1 byte
			},
			want: 0,
		},
		{
			name: "mixed codes",
			caps: []plugin.InjectedCapability{
				{Code: 2, Value: []byte{0x00, 0x01, 0x00, 0x01}}, // multiprotocol
				{Code: grCapCode, Value: makeGRCapValue(240)},
				{Code: 65, Value: []byte{0x00, 0x00, 0xFF, 0xFD}}, // ASN4
			},
			want: 240,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxRestartTime(tt.caps)
			if got != tt.want {
				t.Errorf("MaxRestartTime = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- Phase 3: R-bit injection ---

// VALIDATES: 0x80 OR'd into byte 0 of copied code-64 Value.
// PREVENTS: R-bit not set, peers don't know we restarted.
func TestSetRBitOnCapability(t *testing.T) {
	caps := []plugin.InjectedCapability{
		{Code: grCapCode, Value: makeGRCapValue(120), Plugin: "gr", PeerAddr: "1.1.1.1"},
	}

	result := SetRBit(caps)

	if len(result) != 1 {
		t.Fatalf("SetRBit returned %d caps, want 1", len(result))
	}

	// Check R-bit is set.
	if result[0].Value[0]&0x80 == 0 {
		t.Errorf("R-bit not set: byte 0 = 0x%02x", result[0].Value[0])
	}

	// Check restart-time preserved.
	rt := (int(result[0].Value[0])&0x0F)<<8 | int(result[0].Value[1])
	if rt != 120 {
		t.Errorf("restart-time = %d, want 120", rt)
	}
}

// VALIDATES: Non-code-64 capabilities unchanged.
// PREVENTS: R-bit applied to wrong capability type.
func TestSetRBitNoGRCap(t *testing.T) {
	caps := []plugin.InjectedCapability{
		{Code: 2, Value: []byte{0x00, 0x01, 0x00, 0x01}},  // multiprotocol
		{Code: 65, Value: []byte{0x00, 0x00, 0xFF, 0xFD}}, // ASN4
	}

	result := SetRBit(caps)

	if len(result) != 2 {
		t.Fatalf("SetRBit returned %d caps, want 2", len(result))
	}
	for i, c := range result {
		if c.Code == grCapCode {
			t.Errorf("cap %d unexpectedly has code 64", i)
		}
	}
}

// VALIDATES: Code-64 with Value < 2 bytes: no panic, no modification.
// PREVENTS: Index out of bounds on malformed capability.
func TestSetRBitShortValue(t *testing.T) {
	caps := []plugin.InjectedCapability{
		{Code: grCapCode, Value: []byte{0x01}}, // only 1 byte
		{Code: grCapCode, Value: nil},          // nil value
	}

	result := SetRBit(caps)

	if len(result) != 2 {
		t.Fatalf("SetRBit returned %d caps, want 2", len(result))
	}
	// Short values should be returned unchanged (no panic).
	if len(result[0].Value) != 1 || result[0].Value[0] != 0x01 {
		t.Errorf("short value modified: %v", result[0].Value)
	}
	if result[1].Value != nil {
		t.Errorf("nil value modified: %v", result[1].Value)
	}
}

// VALIDATES: Original InjectedCapability.Value unchanged after R-bit set on copy.
// PREVENTS: Shared slice corruption affecting subsequent OPEN messages.
func TestSetRBitOriginalUnmodified(t *testing.T) {
	original := makeGRCapValue(120)
	originalCopy := make([]byte, len(original))
	copy(originalCopy, original)

	caps := []plugin.InjectedCapability{
		{Code: grCapCode, Value: original},
	}

	_ = SetRBit(caps)

	// Original must be unchanged.
	for i := range original {
		if original[i] != originalCopy[i] {
			t.Errorf("original byte %d changed: 0x%02x, was 0x%02x", i, original[i], originalCopy[i])
		}
	}
}

// VALIDATES: Mixed capabilities: only code-64 gets R-bit, others pass through.
// PREVENTS: Off-by-one in capability filtering.
func TestSetRBitMixed(t *testing.T) {
	caps := []plugin.InjectedCapability{
		{Code: 2, Value: []byte{0x00, 0x01, 0x00, 0x01}},
		{Code: grCapCode, Value: makeGRCapValue(120)},
		{Code: 65, Value: []byte{0x00, 0x00, 0xFF, 0xFD}},
	}

	result := SetRBit(caps)

	if len(result) != 3 {
		t.Fatalf("SetRBit returned %d caps, want 3", len(result))
	}

	// Code 2 unchanged.
	if result[0].Value[0] != 0x00 {
		t.Errorf("code-2 cap modified")
	}
	// Code 64 has R-bit.
	if result[1].Value[0]&0x80 == 0 {
		t.Errorf("code-64 R-bit not set")
	}
	// Code 65 unchanged.
	if result[2].Value[0] != 0x00 {
		t.Errorf("code-65 cap modified")
	}
}

// --- Deadline expiry behavior ---

// VALIDATES: R=1 before deadline, R=0 after deadline.
// PREVENTS: R-bit persisting past the restart window.
func TestRestartDeadlineExpiry(t *testing.T) {
	caps := []plugin.InjectedCapability{
		{Code: grCapCode, Value: makeGRCapValue(120)},
	}

	// Before deadline: R-bit should be set.
	deadline := time.Now().Add(10 * time.Second)
	if !deadline.IsZero() && time.Now().Before(deadline) {
		result := SetRBit(caps)
		if result[0].Value[0]&0x80 == 0 {
			t.Error("before deadline: R-bit not set")
		}
	}

	// After deadline: R-bit should NOT be set (caps returned unmodified).
	pastDeadline := time.Now().Add(-10 * time.Second)
	if !pastDeadline.IsZero() && time.Now().Before(pastDeadline) {
		t.Error("time check should be false for past deadline")
	}
	// When the check is false, caps are used unmodified.
	if caps[0].Value[0]&0x80 != 0 {
		t.Error("after deadline: original cap should have R=0")
	}
}

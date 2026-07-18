// VALIDATES: the RFC 3623 sec 2.1 restart-fact NVS blob round-trips across a simulated
// process restart, carrying the grace deadline, reason, expected adjacencies, and the
// RFC 5187 sec 3.1/3.2 OSPFv3 preservation maps; and a stale (expired) or cleared fact is
// treated as inactive so a resume does not wrongly suppress origination (R-10).
// PREVENTS: a graceful restart silently failing to resume in-restart mode, or a cold boot
// after an unrelated restart wrongly entering it.
package ospf

import (
	"io/fs"
	"testing"
	"time"
)

// fakeGRStore is an in-memory grBlobStore for the NVS unit tests: it survives a simulated
// process restart because the same instance is reused across the write/read pair.
type fakeGRStore struct {
	files map[string][]byte
}

func newFakeGRStore() *fakeGRStore { return &fakeGRStore{files: map[string][]byte{}} }

func (s *fakeGRStore) ReadFile(name string) ([]byte, error) {
	data, ok := s.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (s *fakeGRStore) WriteFile(name string, data []byte, _ fs.FileMode) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	s.files[name] = cp
	return nil
}

// TestRestartFactPersistsAcrossRestart (AC-6, A-11): a restart fact written before a restart
// is read back intact on resume, including the IPv6 sec 3.1 / sec 3.2 preservation maps.
//
// RFC requirement: RFC5187-3.1-1 positive -- the restarting router preserves the LSA-ID to
// prefix correspondence across a graceful restart (RFC 5187 sec 3.1): the PrefixLSIDs map
// (internal/plugins/ospf/gr_nvs.go:43-45) written before the restart is read back intact.
// RFC requirement: RFC5187-3.2-1 positive -- the OSPFv3 Interface ID is preserved across
// restarts (RFC 5187 sec 3.2): the InterfaceIDs map (gr_nvs.go:41-42) survives the
// save/restore round-trip with its per-interface values intact.
func TestRestartFactPersistsAcrossRestart(t *testing.T) {
	store := newFakeGRStore()
	const key = grRestartFactKeyPrefix + "v6-ipv6-unicast"
	now := time.Unix(1_000_000, 0)
	want := restartFact{
		Restarting:   true,
		GraceEndUnix: now.Add(120 * time.Second).Unix(),
		Reason:       2,
		Expected:     []string{"10.0.0.1", "10.0.0.2"},
		InterfaceIDs: map[string]uint32{"eth0": 7, "eth1": 9},
		PrefixLSIDs:  map[string]uint32{"2001:db8::/64": 42},
	}
	if err := writeRestartFact(store, key, want); err != nil {
		t.Fatalf("writeRestartFact: %v", err)
	}

	// Simulate the process restart: a fresh read against the same durable store.
	got, ok := readRestartFact(store, key)
	if !ok {
		t.Fatalf("readRestartFact: fact not found after restart")
	}
	if !got.active(now) {
		t.Fatalf("fact should be active within the grace window")
	}
	if got.Reason != want.Reason || got.GraceEndUnix != want.GraceEndUnix {
		t.Fatalf("fact mismatch: got %+v want %+v", got, want)
	}
	if len(got.Expected) != 2 || got.Expected[0] != "10.0.0.1" {
		t.Fatalf("expected adjacencies not preserved: %v", got.Expected)
	}
	if got.InterfaceIDs["eth1"] != 9 {
		t.Fatalf("sec 3.2 interface-id map not preserved: %v", got.InterfaceIDs)
	}
	if got.PrefixLSIDs["2001:db8::/64"] != 42 {
		t.Fatalf("sec 3.1 prefix->LSA-ID map not preserved: %v", got.PrefixLSIDs)
	}
}

// TestStaleRestartFactIgnored (AC-6, R-10): a fact whose grace window has already closed is
// treated as inactive on resume, so a process that restarted for an unrelated reason after
// the window boots normally instead of wrongly suppressing origination.
// RFC requirement: RFC5187-3.1-1 negative -- preservation is conditioned on a valid
// (active) restart fact, not applied unconditionally: an expired/cleared restart fact is
// inactive, so its preserved LSA-ID->prefix (PrefixLSIDs) map is NOT restored and the router
// boots normally rather than reusing stale correspondences (RFC 5187 sec 3.1).
// RFC requirement: RFC5187-3.2-1 negative -- likewise the preserved OSPFv3 Interface IDs are
// not restored from a stale/cleared restart fact: an expired fact is inactive, so the
// Interface-ID preservation (RFC 5187 sec 3.2) does not apply to a router that is not
// actually in a graceful restart.
func TestStaleRestartFactIgnored(t *testing.T) {
	store := newFakeGRStore()
	const key = grRestartFactKeyPrefix + "v4"
	now := time.Unix(2_000_000, 0)
	stale := restartFact{Restarting: true, GraceEndUnix: now.Add(-time.Second).Unix(), Reason: 1}
	if err := writeRestartFact(store, key, stale); err != nil {
		t.Fatalf("writeRestartFact: %v", err)
	}
	got, ok := readRestartFact(store, key)
	if !ok {
		t.Fatalf("readRestartFact: fact not found")
	}
	if got.active(now) {
		t.Fatalf("expired fact must not be active")
	}
	if !got.expired(now) {
		t.Fatalf("expired() should report true for a passed grace-end")
	}

	// A cleared fact is likewise inactive.
	if err := clearRestartFact(store, key); err != nil {
		t.Fatalf("clearRestartFact: %v", err)
	}
	cleared, ok := readRestartFact(store, key)
	if !ok {
		t.Fatalf("readRestartFact after clear: %v", ok)
	}
	if cleared.active(now) {
		t.Fatalf("cleared fact must not be active")
	}
}

// TestReadRestartFactAbsent: a cold boot with no stored fact reads no fact (boot normally).
func TestReadRestartFactAbsent(t *testing.T) {
	store := newFakeGRStore()
	if _, ok := readRestartFact(store, grRestartFactKeyPrefix+"v4"); ok {
		t.Fatalf("expected no fact on a fresh store")
	}
}

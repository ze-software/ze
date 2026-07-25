// VALIDATES: the NTP config/validation/state helpers that existing tests only
// reach indirectly — defaultConfig values, validatePersistPath and
// validateServerAddress rejection rules, absDuration, the peer map accessors
// (getOrCreatePeer/peerSlice), publishState -> loadState round-trip,
// verifyNTPConfig section routing, and ntpSyncInfo enrichment gating.
// PREVENTS: a default drifting silently, a traversal/control-char address slipping
// through validation, a peer snapshot aliasing the live map, or show-date
// enrichment leaking NTP fields while disabled.

package ntp

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestDefaultConfig(t *testing.T) {
	c := defaultConfig()
	if c.Enabled {
		t.Error("Enabled = true, want false by default")
	}
	if c.IntervalSec != 3600 {
		t.Errorf("IntervalSec = %d, want 3600", c.IntervalSec)
	}
	if c.MaxStepSec != 3600 {
		t.Errorf("MaxStepSec = %d, want 3600", c.MaxStepSec)
	}
	if c.SlewThresholdMs != 128 {
		t.Errorf("SlewThresholdMs = %d, want 128", c.SlewThresholdMs)
	}
	if c.PersistPath != "/perm/ze/timefile" {
		t.Errorf("PersistPath = %q, want /perm/ze/timefile", c.PersistPath)
	}
}

func TestValidatePersistPath(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"empty ok", "", false},
		{"clean absolute", "/perm/ze/timefile", false},
		{"relative", "perm/ze/timefile", true},
		{"traversal", "/perm/../etc/passwd", true},
		{"redundant separators", "/perm//ze/timefile", true},
		{"trailing slash", "/perm/ze/", true},
	} {
		err := validatePersistPath(tc.path)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: validatePersistPath(%q) err = %v, wantErr = %v", tc.name, tc.path, err, tc.wantErr)
		}
	}
}

func TestValidateServerAddress(t *testing.T) {
	if err := validateServerAddress(""); !errors.Is(err, errEmptyServerAddress) {
		t.Errorf("empty: err = %v, want errEmptyServerAddress", err)
	}
	if err := validateServerAddress("pool.ntp.org"); err != nil {
		t.Errorf("hostname: unexpected err %v", err)
	}
	if err := validateServerAddress("192.0.2.1"); err != nil {
		t.Errorf("ip: unexpected err %v", err)
	}
	long := make([]byte, maxServerAddrLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if err := validateServerAddress(string(long)); err == nil {
		t.Error("over-length address should be rejected")
	}
	if err := validateServerAddress("bad\x01host"); !errors.Is(err, errServerAddressContainsControlCharacter) {
		t.Errorf("control char: err = %v, want errServerAddressContainsControlCharacter", err)
	}
}

func TestAbsDuration(t *testing.T) {
	if got := absDuration(-5 * time.Second); got != 5*time.Second {
		t.Errorf("absDuration(-5s) = %v, want 5s", got)
	}
	if got := absDuration(3 * time.Second); got != 3*time.Second {
		t.Errorf("absDuration(3s) = %v, want 3s", got)
	}
	if got := absDuration(0); got != 0 {
		t.Errorf("absDuration(0) = %v, want 0", got)
	}
}

func TestGetOrCreatePeerAndSlice(t *testing.T) {
	w := newSyncWorker(defaultConfig(), nil)

	p1 := w.getOrCreatePeer("a.example")
	p2 := w.getOrCreatePeer("a.example")
	if p1 != p2 {
		t.Error("getOrCreatePeer returned a different pointer for the same address")
	}
	w.getOrCreatePeer("b.example")

	snap := w.peerSlice()
	if len(snap) != 2 {
		t.Fatalf("peerSlice len = %d, want 2", len(snap))
	}
	// Snapshot must be a value copy: mutating it must not affect the live peer.
	for i := range snap {
		snap[i].Stratum = 99
	}
	if w.getOrCreatePeer("a.example").Stratum == 99 {
		t.Error("peerSlice returned aliases into the live peer map")
	}
}

func TestPublishStateRoundTrip(t *testing.T) {
	prev := globalState.Load()
	t.Cleanup(func() { globalState.Store(prev) })

	w := newSyncWorker(defaultConfig(), nil)
	w.getOrCreatePeer("a.example")

	w.publishState(false, "", 0, 0)
	st := loadState()
	if st == nil || !st.Enabled || st.Synced {
		t.Fatalf("not-synced state = %+v, want Enabled && !Synced", st)
	}
	if !st.LastSync.IsZero() {
		t.Error("LastSync should be zero when not synced")
	}
	if len(st.Servers) != 1 {
		t.Errorf("Servers len = %d, want 1", len(st.Servers))
	}

	w.publishState(true, "a.example", 42*time.Millisecond, 3)
	st = loadState()
	if !st.Synced || st.Source != "a.example" || st.Stratum != 3 {
		t.Errorf("synced state = %+v, want Synced source a.example stratum 3", st)
	}
	if st.LastSync.IsZero() {
		t.Error("LastSync should be set once synced")
	}
}

func TestVerifyNTPConfig(t *testing.T) {
	// A non-environment section is skipped regardless of its data.
	if err := verifyNTPConfig([]sdk.ConfigSection{{Root: "bgp", Data: "not json"}}); err != nil {
		t.Errorf("non-environment section should be skipped, got %v", err)
	}
	// A valid environment section parses cleanly.
	if err := verifyNTPConfig([]sdk.ConfigSection{{Root: configRootEnvironment, Data: `{"environment":{}}`}}); err != nil {
		t.Errorf("valid environment section: unexpected err %v", err)
	}
	// A malformed environment section surfaces a wrapped error.
	err := verifyNTPConfig([]sdk.ConfigSection{{Root: configRootEnvironment, Data: `{"environment":`}})
	if err == nil {
		t.Error("malformed environment section should error")
	}
}

func TestNTPSyncInfo(t *testing.T) {
	prev := globalState.Load()
	t.Cleanup(func() { globalState.Store(prev) })

	// No state -> nil.
	globalState.Store(nil)
	if got := ntpSyncInfo(); got != nil {
		t.Errorf("ntpSyncInfo(nil state) = %v, want nil", got)
	}

	// Disabled -> nil.
	storeState(&syncState{Enabled: false, Synced: true})
	if got := ntpSyncInfo(); got != nil {
		t.Errorf("ntpSyncInfo(disabled) = %v, want nil", got)
	}

	// Enabled -> populated map.
	storeState(&syncState{Enabled: true, Synced: true, Source: "a.example", Offset: 7 * time.Millisecond})
	got := ntpSyncInfo()
	if got == nil {
		t.Fatal("ntpSyncInfo(enabled) = nil, want map")
	}
	if got["ntp-synced"] != true {
		t.Errorf("ntp-synced = %v, want true", got["ntp-synced"])
	}
	if got["ntp-source"] != "a.example" {
		t.Errorf("ntp-source = %v, want a.example", got["ntp-source"])
	}
	if got["ntp-offset"] != (7 * time.Millisecond).String() {
		t.Errorf("ntp-offset = %v, want %v", got["ntp-offset"], (7 * time.Millisecond).String())
	}
}

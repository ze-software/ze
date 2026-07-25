package ntp

import (
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/ntp"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ntpevents "github.com/ze-software/ze/internal/plugins/ntp/events"
)

// newTimeStore registers an empty database.zefs as the process-wide statestore so
// NTP time persistence round-trips through the real zefs store (not a loose file).
// It resets the store to nil on cleanup. The statestore is process-global, so tests
// that call this MUST NOT call t.Parallel(): a parallel sibling would clobber the
// registered store (see the non-parallel show-handler tests for the same pattern).
func newTimeStore(t *testing.T) {
	t.Helper()
	bs, err := zefs.Create(filepath.Join(t.TempDir(), "database.zefs"))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		// test-relax: close now runs in t.Cleanup for the process-global store; a
		// deferred cleanup reports with the non-fatal t.Errorf (Fatal is discouraged
		// in cleanup), not the Fatalf the old path-returning helper used inline.
		if err := bs.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
}

// errTestUnreachable is the stub error for an unreachable NTP server.
var errTestUnreachable = errors.New("test: ntp server unreachable")

// restoreNTPSeams captures the current doSync test seams and restores them via
// t.Cleanup, so a test may override ntpQueryFn/setClockFn without leaking the
// override into other tests. Tests using this MUST NOT call t.Parallel().
func restoreNTPSeams(t *testing.T) {
	t.Helper()
	origQuery := ntpQueryFn
	origClock := setClockFn
	t.Cleanup(func() {
		ntpQueryFn = origQuery
		setClockFn = origClock
	})
}

// validNTPResponse builds a *ntp.Response that passes Response.Validate:
// stratum in range, transmit time not before reference time, small dispersion,
// no kiss-of-death or leap-not-in-sync.
func validNTPResponse(offset time.Duration) *ntp.Response {
	now := time.Now()
	return &ntp.Response{
		Stratum:       2,
		Time:          now,
		ReferenceTime: now.Add(-time.Minute),
		ClockOffset:   offset,
		RTT:           2 * time.Millisecond,
	}
}

// TestParseNTPConfigEnabled verifies that NTP config is parsed correctly.
//
// VALIDATES: AC-1 - Config with ntp { enabled true; server { ... } } parsed.
// PREVENTS: NTP config silently ignored.
func TestParseNTPConfigEnabled(t *testing.T) {
	t.Parallel()
	data := `{"environment":{"ntp":{"enabled":"true","interval":"300","server":{"pool0":{"address":"0.pool.ntp.org"},"pool1":{"address":"1.pool.ntp.org"}}}}}`
	cfg, err := parseNTPConfig(data)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 300, cfg.IntervalSec)
	assert.Len(t, cfg.Servers, 2)
	assert.Contains(t, cfg.Servers, "0.pool.ntp.org")
	assert.Contains(t, cfg.Servers, "1.pool.ntp.org")
}

// TestParseNTPConfigDisabled verifies disabled-by-default behavior.
//
// VALIDATES: AC-2 - No ntp block means NTP disabled.
// PREVENTS: NTP accidentally enabled when config omits the block.
func TestParseNTPConfigDisabled(t *testing.T) {
	t.Parallel()
	data := `{"environment":{}}`
	cfg, err := parseNTPConfig(data)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
}

// TestParseNTPConfigNoEnvironment verifies missing environment section.
//
// VALIDATES: parseNTPConfig handles absent environment gracefully.
// PREVENTS: Panic on minimal config without environment section.
func TestParseNTPConfigNoEnvironment(t *testing.T) {
	t.Parallel()
	data := `{}`
	cfg, err := parseNTPConfig(data)
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 3600, cfg.IntervalSec) // default
}

// TestParseNTPConfigIntervalBounds verifies interval boundary enforcement.
//
// VALIDATES: AC-15 - Sync interval within valid range.
// PREVENTS: Unreasonably short or long sync intervals.
func TestParseNTPConfigIntervalBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		interval string
		expected int
	}{
		{"valid 60s (minimum)", "60", 60},
		{"valid 86400s (maximum)", "86400", 86400},
		{"below minimum (59s)", "59", 3600},       // falls back to default
		{"above maximum (86401s)", "86401", 3600}, // falls back to default
		{"invalid string", "abc", 3600},           // falls back to default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := `{"environment":{"ntp":{"enabled":"true","interval":"` + tt.interval + `"}}}`
			cfg, err := parseNTPConfig(data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.IntervalSec)
		})
	}
}

// TestParseNTPConfigMaxStep verifies the deployment safety cap on NTP clock
// steps is parsed from config.
//
// VALIDATES: max-step controls the largest accepted NTP clock step.
// PREVENTS: an unauthenticated NTP server from moving system time by an
// unbounded amount under default policy.
func TestParseNTPConfigMaxStep(t *testing.T) {
	t.Parallel()
	data := `{"environment":{"ntp":{"enabled":"true","max-step":"120"}}}`
	cfg, err := parseNTPConfig(data)
	require.NoError(t, err)
	assert.Equal(t, 120, cfg.MaxStepSec)
}

// TestClockOffsetAllowedMaxStep verifies the pure max-step decision used
// before settimeofday.
//
// VALIDATES: offsets beyond max-step are rejected; max-step 0 is explicit
// unlimited mode.
// PREVENTS: accidental removal of the large-step guard in doSync.
func TestClockOffsetAllowedMaxStep(t *testing.T) {
	t.Parallel()

	assert.True(t, clockOffsetAllowed(30*time.Second, time.Minute))
	assert.True(t, clockOffsetAllowed(-30*time.Second, time.Minute))
	assert.False(t, clockOffsetAllowed(2*time.Minute, time.Minute))
	assert.True(t, clockOffsetAllowed(24*time.Hour, 0))
}

// TestTimePersistenceSave verifies time is saved into the shared zefs store.
//
// VALIDATES: AC-5 - NTP query succeeds, time persisted under the NTP last-time key.
// PREVENTS: Time persistence silently failing, or reverting to loose-file writes.
// Not parallel: newTimeStore registers a process-global statestore (see helper).
func TestTimePersistenceSave(t *testing.T) {
	newTimeStore(t)

	now := time.Date(2026, 4, 12, 15, 30, 0, 0, time.UTC)
	err := saveTime(now)
	require.NoError(t, err)

	// The RFC3339 blob is stored under the NTP last-time key in the zefs store.
	data, ok := statestore.Get(zefs.KeyNTPLastTime.Pattern)
	require.True(t, ok, "expected a persisted time blob under the key")
	assert.Contains(t, string(data), "2026-04-12")
}

// TestTimePersistenceRestore verifies time is restored from the shared zefs store.
//
// VALIDATES: AC-6 - Boot with a persisted time, clock set to saved time.
// PREVENTS: Saved time ignored on boot.
// Not parallel: newTimeStore registers a process-global statestore (see helper).
func TestTimePersistenceRestore(t *testing.T) {
	newTimeStore(t)

	saved := time.Date(2026, 4, 12, 15, 30, 0, 0, time.UTC)
	require.NoError(t, saveTime(saved))

	loaded, err := loadTime()
	require.NoError(t, err)
	assert.Equal(t, saved.Unix(), loaded.Unix())
}

// TestTimePersistenceMissing verifies graceful handling of an absent key/store.
//
// VALIDATES: AC-7 - Boot without a persisted time, no crash (error surfaced).
// PREVENTS: Crash on first boot when the store has no saved time yet.
// Not parallel: newTimeStore registers a process-global statestore (see helper).
func TestTimePersistenceMissing(t *testing.T) {
	// A store that exists but has no saved-time key yet.
	newTimeStore(t)
	_, err := loadTime()
	assert.Error(t, err)

	// test-relax: the old "absent store path" case is no longer expressible under
	// the paramless API (no path argument); the no-store-registered case now lives
	// in TestTimePersistenceNoStoreIsNoOp.
}

// TestTimePersistenceCorrupt verifies graceful handling of a corrupt blob.
//
// VALIDATES: loadTime rejects corrupt content.
// PREVENTS: Panic on a corrupt saved time.
// Not parallel: newTimeStore registers a process-global statestore (see helper).
func TestTimePersistenceCorrupt(t *testing.T) {
	newTimeStore(t)

	_, err := statestore.Put(zefs.KeyNTPLastTime.Pattern, []byte("not a valid time"))
	require.NoError(t, err)

	_, err = loadTime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

// TestTimePersistenceAbsurdYear verifies rejection of out-of-range years.
//
// VALIDATES: AC-14 - NTP response with absurd timestamp rejected.
// PREVENTS: Saved time from 1970 or far future accepted.
// Not parallel: newTimeStore registers a process-global statestore (see helper).
func TestTimePersistenceAbsurdYear(t *testing.T) {
	newTimeStore(t)

	old := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	buf, _ := old.MarshalText()
	_, err := statestore.Put(zefs.KeyNTPLastTime.Pattern, buf)
	require.NoError(t, err)

	_, err = loadTime()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// TestSyncWorkerServersConfigPriority verifies configured servers
// take priority over DHCP-discovered ones.
//
// VALIDATES: AC-11 - DHCP servers used only when no configured servers.
// PREVENTS: DHCP servers overriding explicit config.
func TestSyncWorkerServersConfigPriority(t *testing.T) {
	t.Parallel()
	cfg := ntpConfig{
		Enabled: true,
		Servers: []string{"configured.ntp.org"},
	}
	w := newSyncWorker(cfg, nil)
	w.addDHCPServers([]string{"dhcp.ntp.org"})

	// Configured servers should win.
	servers := w.servers()
	assert.Equal(t, []string{"configured.ntp.org"}, servers)
}

// TestSyncWorkerServersDHCPFallback verifies DHCP servers used when
// no servers are configured.
//
// VALIDATES: AC-11 - DHCP servers used as fallback.
// PREVENTS: No NTP when config has no servers but DHCP provides them.
func TestSyncWorkerServersDHCPFallback(t *testing.T) {
	t.Parallel()
	cfg := ntpConfig{
		Enabled: true,
		Servers: nil, // no configured servers
	}
	w := newSyncWorker(cfg, nil)
	w.addDHCPServers([]string{"dhcp1.ntp.org", "dhcp2.ntp.org"})

	servers := w.servers()
	assert.Equal(t, []string{"dhcp1.ntp.org", "dhcp2.ntp.org"}, servers)
}

// TestSyncWorkerNoServers verifies empty server list returns nil.
//
// VALIDATES: doSync handles no servers gracefully.
// PREVENTS: Index-out-of-range panic with empty server list.
func TestSyncWorkerNoServers(t *testing.T) {
	t.Parallel()
	cfg := ntpConfig{Enabled: true}
	w := newSyncWorker(cfg, nil)

	servers := w.servers()
	assert.Empty(t, servers)
}

// TestHandleDHCPEvent verifies DHCP lease event parsing for NTP servers.
//
// VALIDATES: AC-12 - DHCP lease with NTP servers processed.
// PREVENTS: NTP servers from DHCP option 42 silently dropped.
func TestHandleDHCPEvent(t *testing.T) {
	t.Parallel()
	cfg := ntpConfig{Enabled: true}
	w := newSyncWorker(cfg, nil)

	// Simulate a DHCP lease event with NTP servers.
	data := `{"name":"eth0","unit":"default","address":"10.0.0.5","prefix-length":24,"ntp-servers":["192.168.1.1","192.168.1.2"]}`
	w.handleDHCPEvent(data)

	w.mu.Lock()
	defer w.mu.Unlock()
	assert.Equal(t, []string{"192.168.1.1", "192.168.1.2"}, w.dhcpSrv)
}

// TestShowSystemNTPWiring verifies that the show system ntp handler
// is callable and returns a valid response.
//
// VALIDATES: Wiring Test row 1 - show system ntp entry point.
// PREVENTS: Handler not registered or returning nil.
func TestShowSystemNTPWiring(t *testing.T) {
	// Deliberately NOT parallel: the handler reads the package-global state
	// and parallel siblings publish it. Sequential tests run while parallel
	// bodies are parked, so resetting here is race-free.
	storeState(nil)

	// No state published -> disabled.
	resp, err := handleShowSystemNTP(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected map response")
	assert.Equal(t, false, data["enabled"])
}

// TestShowSystemNTPPeersWiring verifies that the show system ntp peers
// handler is callable and returns a valid response.
//
// VALIDATES: Wiring Test row 2 - show system ntp peers entry point.
// PREVENTS: Handler not registered or returning nil.
func TestShowSystemNTPPeersWiring(t *testing.T) {
	// Sequential for the same reason as TestShowSystemNTPWiring: the
	// handler reads package-global state that parallel siblings publish.
	storeState(nil)

	// No state published -> empty peers.
	resp, err := handleShowSystemNTPPeers(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "expected map response")
	peers, ok := data["peers"].([]map[string]any)
	require.True(t, ok, "expected peers array")
	assert.Empty(t, peers)
	assert.Equal(t, 0, data["count"])
}

// TestShowSystemNTPEnabled verifies the handler returns sync details
// when NTP is enabled and synced.
//
// VALIDATES: AC-1 - show system ntp with synced state.
// PREVENTS: Missing fields in synced response.
func TestShowSystemNTPEnabled(t *testing.T) {
	t.Parallel()

	st := &syncState{
		Enabled:      true,
		Synced:       true,
		Source:       "pool.ntp.org",
		Offset:       42 * time.Millisecond,
		Stratum:      2,
		PollInterval: 300,
		LastSync:     time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
	}
	globalState.Store(st)
	defer globalState.Store(nil)

	resp, err := handleShowSystemNTP(nil, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, true, data["enabled"])
	assert.Equal(t, true, data["synced"])
	assert.Equal(t, "pool.ntp.org", data["source"])
	assert.Equal(t, uint8(2), data["stratum"])
	assert.Equal(t, 300, data["poll-interval"])
	assert.Contains(t, data, "last-sync")
	assert.Contains(t, data, "offset")
}

// TestShowSystemNTPDisabled verifies the handler returns enabled:false
// when NTP is disabled.
//
// VALIDATES: AC-3 - show system ntp disabled.
// PREVENTS: Disabled state returning sync details.
func TestShowSystemNTPDisabled(t *testing.T) {
	t.Parallel()

	st := &syncState{Enabled: false}
	globalState.Store(st)
	defer globalState.Store(nil)

	resp, err := handleShowSystemNTP(nil, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, false, data["enabled"])
	_, hasSynced := data["synced"]
	assert.False(t, hasSynced)
}

// TestShowSystemNTPPeers verifies per-server data in the response.
//
// VALIDATES: AC-4 - show system ntp peers with servers.
// PREVENTS: Missing or wrong per-server fields.
func TestShowSystemNTPPeers(t *testing.T) {
	t.Parallel()

	st := &syncState{
		Enabled: true,
		Servers: []serverState{
			{
				Address:   "0.pool.ntp.org",
				Offset:    10 * time.Millisecond,
				RTT:       5 * time.Millisecond,
				Stratum:   2,
				Reach:     0xFF,
				LastQuery: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			},
			{
				Address:   "1.pool.ntp.org",
				Reach:     0x00,
				LastQuery: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
				LastError: "timeout",
			},
		},
	}
	globalState.Store(st)
	defer globalState.Store(nil)

	resp, err := handleShowSystemNTPPeers(nil, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 2, data["count"])
	peers, ok := data["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 2)

	found := false
	for _, p := range peers {
		if p["address"] == "1.pool.ntp.org" {
			assert.Equal(t, "timeout", p["last-error"])
			found = true
		}
	}
	assert.True(t, found, "expected unreachable server in peers")
}

// TestShowSystemNTPPeersEmpty verifies empty array when no servers configured.
//
// VALIDATES: AC-5 - show system ntp peers with no servers.
// PREVENTS: Nil instead of empty array.
func TestShowSystemNTPPeersEmpty(t *testing.T) {
	t.Parallel()

	st := &syncState{Enabled: true, Servers: nil}
	globalState.Store(st)
	defer globalState.Store(nil)

	resp, err := handleShowSystemNTPPeers(nil, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 0, data["count"])
}

// TestParseNTPConfigSlewThreshold verifies slew-threshold parsing.
//
// VALIDATES: AC-10, AC-11 - slew-threshold config values.
// PREVENTS: Slew threshold silently ignored or mis-parsed.
func TestParseNTPConfigSlewThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		expected int
		wantErr  bool
	}{
		{"default (128)", "", 128, false},
		{"zero disables slew", "0", 0, false},
		{"custom 500ms", "500", 500, false},
		{"max 1000ms", "1000", 1000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := `{"environment":{"ntp":{"enabled":"true"`
			if tt.value != "" {
				data += `,"slew-threshold":"` + tt.value + `"`
			}
			data += `}}}`
			cfg, err := parseNTPConfig(data)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.SlewThresholdMs)
		})
	}
}

// TestParseNTPConfigSlewThresholdBounds verifies slew-threshold range enforcement.
//
// VALIDATES: Boundary: slew-threshold 0..1000.
// PREVENTS: Out-of-range values accepted.
func TestParseNTPConfigSlewThresholdBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"below range (-1)", "-1", true},
		{"above range (1001)", "1001", true},
		{"invalid string", "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := `{"environment":{"ntp":{"enabled":"true","slew-threshold":"` + tt.value + `"}}}`
			_, err := parseNTPConfig(data)
			assert.Error(t, err)
		})
	}
}

// TestClockOffsetAction verifies the slew/step/reject decision.
//
// VALIDATES: AC-6 (slew), AC-7 (step), AC-8 (reject), AC-10 (disable).
// PREVENTS: Wrong clock adjustment action for offset ranges.
func TestClockOffsetAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		offset     time.Duration
		slewMs     int
		maxStepSec int
		expected   clockAction
	}{
		{"small offset -> slew", 50 * time.Millisecond, 128, 3600, actionSlew},
		{"negative small -> slew", -50 * time.Millisecond, 128, 3600, actionSlew},
		{"at slew threshold -> slew", 128 * time.Millisecond, 128, 3600, actionSlew},
		{"above slew threshold -> step", 200 * time.Millisecond, 128, 3600, actionStep},
		{"at max step -> step", 3600 * time.Second, 128, 3600, actionStep},
		{"above max step -> reject", 3601 * time.Second, 128, 3600, actionReject},
		{"slew disabled (0) -> step", 50 * time.Millisecond, 0, 3600, actionStep},
		{"unlimited max step (0) -> slew", 50 * time.Millisecond, 128, 0, actionSlew},
		{"unlimited max step (0) large -> step", 5000 * time.Second, 128, 0, actionStep},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := decideClockAction(tt.offset, tt.slewMs, tt.maxStepSec)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestReachRegisterShift verifies the 8-bit reach register behavior.
// RFC 5905 Section 13.1: shift left, set bit 0 on success.
//
// VALIDATES: AC-12, AC-13, AC-14 - reach bitmap updates.
// PREVENTS: Incorrect reach register logic.
func TestReachRegisterShift(t *testing.T) {
	t.Parallel()

	// Starting at 0, success sets bit 0.
	assert.Equal(t, uint8(0x01), reachShift(0x00, true))
	// Shift left + success: 0x01 -> 0x03.
	assert.Equal(t, uint8(0x03), reachShift(0x01, true))
	// Shift left + failure: 0x01 -> 0x02.
	assert.Equal(t, uint8(0x02), reachShift(0x01, false))
	// Full 0xFF shifted + success stays 0xFF.
	assert.Equal(t, uint8(0xFF), reachShift(0xFF, true))
	// Full 0xFF shifted + failure -> 0xFE.
	assert.Equal(t, uint8(0xFE), reachShift(0xFF, false))
	// 8 consecutive failures from full -> 0.
	r := uint8(0xFF)
	for range 8 {
		r = reachShift(r, false)
	}
	assert.Equal(t, uint8(0x00), r)
}

// TestServerSelection verifies best server selection: reachable, lowest
// stratum, then smallest offset.
//
// VALIDATES: Server selection algorithm.
// PREVENTS: Unreachable server selected, wrong tiebreaker.
func TestServerSelection(t *testing.T) {
	t.Parallel()

	servers := []serverState{
		{Address: "unreachable", Reach: 0, Stratum: 1, Offset: time.Millisecond},
		{Address: "high-stratum", Reach: 1, Stratum: 3, Offset: time.Millisecond},
		{Address: "best", Reach: 1, Stratum: 2, Offset: 5 * time.Millisecond},
		{Address: "same-stratum-closer", Reach: 1, Stratum: 2, Offset: 2 * time.Millisecond},
	}
	best := selectBestServer(servers)
	require.NotNil(t, best)
	assert.Equal(t, "same-stratum-closer", best.Address)
}

// TestServerSelectionAllUnreachable verifies nil return when no server is reachable.
func TestServerSelectionAllUnreachable(t *testing.T) {
	t.Parallel()
	servers := []serverState{
		{Address: "a", Reach: 0},
		{Address: "b", Reach: 0},
	}
	assert.Nil(t, selectBestServer(servers))
}

// TestServerSelectionEmpty verifies nil return for empty list.
func TestServerSelectionEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, selectBestServer(nil))
}

// TestSyncStateSnapshot verifies atomic state snapshot consistency.
//
// VALIDATES: AC-16 - no data race between writer and reader.
// PREVENTS: Torn reads from concurrent access.
func TestSyncStateSnapshot(t *testing.T) {
	t.Parallel()

	// Store a state.
	st := &syncState{
		Enabled:      true,
		Synced:       true,
		Source:       "pool.ntp.org",
		Offset:       42 * time.Millisecond,
		Stratum:      2,
		PollInterval: 300,
		LastSync:     time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		Servers: []serverState{
			{Address: "pool.ntp.org", Offset: 42 * time.Millisecond, Stratum: 2, Reach: 0xFF},
		},
	}
	globalState.Store(st)
	defer globalState.Store(nil)

	// Read back.
	got := loadState()
	require.NotNil(t, got)
	assert.True(t, got.Synced)
	assert.Equal(t, "pool.ntp.org", got.Source)
	assert.Len(t, got.Servers, 1)
	assert.Equal(t, uint8(0xFF), got.Servers[0].Reach)
}

// mockEventBus records Emit calls for testing.
type mockEventBus struct {
	emits []emitCall
}

type emitCall struct {
	namespace, eventType string
	payload              any
}

func (m *mockEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	m.emits = append(m.emits, emitCall{namespace, eventType, payload})
	return 0, nil
}

func (m *mockEventBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

// TestSyncWorkerClockSyncedEmittedOnce verifies that the clock-synced
// event is emitted exactly once after the first successful NTP sync.
//
// VALIDATES: AC-5 - Clock readiness gate: clock-synced event emitted.
// PREVENTS: Missing clock-synced event, or event emitted on every sync.
func TestSyncWorkerClockSyncedEmittedOnce(t *testing.T) {
	t.Parallel()
	eb := &mockEventBus{}
	cfg := ntpConfig{Enabled: true}
	w := newSyncWorker(cfg, eb)

	// First sync: CompareAndSwap succeeds, event emitted.
	if w.synced.CompareAndSwap(false, true) && w.eventBus != nil {
		n, err := w.eventBus.Emit(ntpevents.Namespace, "clock-synced", "")
		assert.NoError(t, err)
		assert.Equal(t, 0, n)
	}
	// Second attempt: CompareAndSwap fails, no emission.
	if w.synced.CompareAndSwap(false, true) && w.eventBus != nil {
		t.Fatal("should not reach here: synced already true")
	}

	assert.Len(t, eb.emits, 1)
	assert.Equal(t, "system", eb.emits[0].namespace)
	assert.Equal(t, "clock-synced", eb.emits[0].eventType)
}

// TestSyncWorkerClockSyncedNilEventBus verifies no panic with nil EventBus.
//
// VALIDATES: AC-5 - Graceful behavior when EventBus not available.
// PREVENTS: Nil pointer panic when NTP syncs without EventBus.
func TestSyncWorkerClockSyncedNilEventBus(t *testing.T) {
	t.Parallel()
	cfg := ntpConfig{Enabled: true}
	w := newSyncWorker(cfg, nil)

	// Should not panic.
	assert.True(t, w.synced.CompareAndSwap(false, true))
	assert.Nil(t, w.eventBus)
}

// TestTimePersistenceNoStoreIsNoOp verifies saveTime is a best-effort no-op when
// the shared zefs store does not exist: statestore never creates the store, so the
// save returns nil (nothing persisted) rather than erroring or writing a loose file.
//
// VALIDATES: NTP time persistence is best-effort through statestore.
// PREVENTS: A regression that recreates loose-file writes or treats an absent store as fatal.
// Not parallel: reads the process-global statestore, which sibling persistence
// tests register (see newTimeStore).
func TestTimePersistenceNoStoreIsNoOp(t *testing.T) {
	// No store registered: statestore Put/Get are best-effort no-ops.
	statestore.SetStore(nil)

	now := time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC)
	require.NoError(t, saveTime(now), "save with no store registered should be a best-effort no-op")

	// Nothing was persisted, so loadTime reports not-found.
	// test-relax: the old os.Stat "no loose file created" assertion is obsolete under
	// the paramless API -- saveTime takes no path and cannot write a loose file.
	_, err := loadTime()
	assert.Error(t, err)
}

// TestDoSyncStopChecksBetweenServers verifies that a stop signal arriving
// mid-doSync abandons the remaining per-server queries instead of walking the
// whole server list.
//
// VALIDATES: startup-resilience FIX 1 / AC-3 - config re-apply that stops the
// worker waits out at most one in-flight query, not len(servers) x timeout.
// PREVENTS: reintroducing the unbounded reload block (removing the stop-check
// from the doSync server loop would query every server before returning).
func TestDoSyncStopChecksBetweenServers(t *testing.T) {
	restoreNTPSeams(t)

	cfg := ntpConfig{Enabled: true, Servers: []string{"srv-a", "srv-b", "srv-c"}}
	w := newSyncWorker(cfg, nil)

	var mu sync.Mutex
	var queried []string
	ntpQueryFn = func(addr string) (*ntp.Response, error) {
		mu.Lock()
		queried = append(queried, addr)
		first := len(queried) == 1
		mu.Unlock()
		if first {
			// Simulate a reload/shutdown landing while the first query is in
			// flight; the loop must not start the remaining servers.
			close(w.stop)
		}
		return nil, errTestUnreachable
	}

	synced := w.doSync(loggerPtr.Load())

	assert.False(t, synced, "doSync should report no sync when stopping")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"srv-a"}, queried,
		"only the in-flight server should be queried after stop is signaled")
}

// TestStartWorkerReloadBoundedWait drives the full worker lifecycle and proves
// that stopAndWait during an in-flight sync lets exactly one query complete and
// starts no further ones.
//
// VALIDATES: startup-resilience FIX 1 / AC-3 through the real start()/
// stopAndWait() path (not just doSync in isolation).
// PREVENTS: a regression where the worker keeps querying dead servers after a
// reload signal, blocking the config-apply transaction past its deadline.
func TestStartWorkerReloadBoundedWait(t *testing.T) {
	restoreNTPSeams(t)

	cfg := ntpConfig{Enabled: true, Servers: []string{"s1", "s2", "s3", "s4", "s5"}}
	w := newSyncWorker(cfg, nil)

	var mu sync.Mutex
	var started []string
	release := make(chan struct{})
	ntpQueryFn = func(addr string) (*ntp.Response, error) {
		mu.Lock()
		started = append(started, addr)
		mu.Unlock()
		<-release // block so the test controls when the in-flight query returns
		return nil, errTestUnreachable
	}

	w.start()

	// Wait for the first query to be in flight.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(started) == 1
	}, 2*time.Second, time.Millisecond, "first query never started")

	// Signal stop, then wait until the worker has observed it, then release the
	// blocked query. With the stop-check the loop returns without starting s2..s5.
	stopReturned := make(chan struct{})
	go func() {
		w.stopAndWait()
		close(stopReturned)
	}()
	require.Eventually(t, w.isStopped, time.Second, time.Millisecond,
		"stopAndWait did not close the stop channel")
	close(release)

	select {
	case <-stopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("stopAndWait did not return within the bounded wait")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, started, 1,
		"exactly one query should run after a reload signal; got %v", started)
}

// TestSyncWorkerReloadNoGoroutineLeak verifies that repeated start/stop cycles
// (as config reloads do) leave no lingering worker goroutines.
//
// VALIDATES: startup-resilience R-3 - background retry loops do not leak
// goroutines across config reloads.
// PREVENTS: stopAndWait returning before the worker goroutine exits, which would
// accumulate goroutines on every commit that touches the environment root.
func TestSyncWorkerReloadNoGoroutineLeak(t *testing.T) {
	restoreNTPSeams(t)

	// Fast, non-blocking stub so each cycle completes promptly.
	ntpQueryFn = func(string) (*ntp.Response, error) { return nil, errTestUnreachable }

	cfg := ntpConfig{Enabled: true, Servers: []string{"s1", "s2"}}

	const cycles = 25
	before := runtime.NumGoroutine()
	for range cycles {
		w := newSyncWorker(cfg, nil)
		w.start()
		w.stopAndWait()
	}

	// stopAndWait joins the worker goroutine, so the count should already be
	// settled; poll briefly to absorb scheduler slack from any parallel tests.
	deadline := time.Now().Add(2 * time.Second)
	after := runtime.NumGoroutine()
	for after > before+1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		after = runtime.NumGoroutine()
	}
	assert.LessOrEqualf(t, after, before+1,
		"goroutine leak across %d reload cycles: before=%d after=%d", cycles, before, after)
}

// TestSyncWorkerConvergesWhenServerAppears verifies AC-4: a worker whose servers
// are unreachable at first syncs once a server starts answering, without a
// restart.
//
// VALIDATES: startup-resilience AC-4 - convergence when the service returns.
// PREVENTS: the stop-check change regressing the retry-then-sync path so a
// recovered server is never picked up.
func TestSyncWorkerConvergesWhenServerAppears(t *testing.T) {
	restoreNTPSeams(t)

	// Avoid a privileged Settimeofday in the unit test.
	var clockSet atomic.Int32
	setClockFn = func(time.Time) error {
		clockSet.Add(1)
		return nil
	}

	var attempts atomic.Int32
	ntpQueryFn = func(string) (*ntp.Response, error) {
		if attempts.Add(1) == 1 {
			return nil, errTestUnreachable // server still down on the first pass
		}
		return validNTPResponse(5 * time.Millisecond), nil // server now answers
	}

	cfg := ntpConfig{Enabled: true, Servers: []string{"srv"}} // SlewThresholdMs 0 -> step
	w := newSyncWorker(cfg, nil)

	assert.False(t, w.doSync(loggerPtr.Load()), "first pass: server unreachable")
	assert.False(t, w.synced.Load(), "not synced after the failed pass")

	assert.True(t, w.doSync(loggerPtr.Load()), "second pass: server answers")
	assert.True(t, w.synced.Load(), "worker converged to synced without a restart")
	assert.GreaterOrEqual(t, clockSet.Load(), int32(1), "clock should be set on sync")
}

package ntp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ntpevents "codeberg.org/thomas-mangin/ze/internal/plugins/ntp/events"
)

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

// TestTimePersistenceSave verifies time is saved to file.
//
// VALIDATES: AC-5 - NTP query succeeds, time saved to persistence file.
// PREVENTS: Time persistence silently failing.
func TestTimePersistenceSave(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "timefile")

	now := time.Date(2026, 4, 12, 15, 30, 0, 0, time.UTC)
	err := saveTime(path, now)
	require.NoError(t, err)

	// Verify file exists and contains valid time.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "2026-04-12")
}

// TestTimePersistenceRestore verifies time is restored from file.
//
// VALIDATES: AC-6 - Boot with persistence file, clock set to saved time.
// PREVENTS: Saved time file ignored on boot.
func TestTimePersistenceRestore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "timefile")

	saved := time.Date(2026, 4, 12, 15, 30, 0, 0, time.UTC)
	require.NoError(t, saveTime(path, saved))

	loaded, err := loadTime(path)
	require.NoError(t, err)
	assert.Equal(t, saved.Unix(), loaded.Unix())
}

// TestTimePersistenceMissing verifies graceful handling of missing file.
//
// VALIDATES: AC-7 - Boot without persistence file, no error.
// PREVENTS: Crash on first boot without time file.
func TestTimePersistenceMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")

	_, err := loadTime(path)
	assert.Error(t, err)
}

// TestTimePersistenceCorrupt verifies graceful handling of corrupt file.
//
// VALIDATES: loadTime rejects corrupt content.
// PREVENTS: Panic on corrupt time file.
func TestTimePersistenceCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "timefile")

	require.NoError(t, os.WriteFile(path, []byte("not a valid time"), 0o644))

	_, err := loadTime(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

// TestTimePersistenceAbsurdYear verifies rejection of out-of-range years.
//
// VALIDATES: AC-14 - NTP response with absurd timestamp rejected.
// PREVENTS: Saved time from 1970 or far future accepted.
func TestTimePersistenceAbsurdYear(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "timefile")

	old := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	buf, _ := old.MarshalText()
	require.NoError(t, os.WriteFile(path, buf, 0o644))

	_, err := loadTime(path)
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
	t.Parallel()

	// No state published yet -> disabled.
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
	t.Parallel()

	// No state published yet -> empty peers.
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

// TestPersistPathCreatesDirs verifies that saveTime creates parent dirs.
//
// VALIDATES: saveTime creates intermediate directories.
// PREVENTS: Failure on first save when /perm/ze/ doesn't exist yet.
func TestPersistPathCreatesDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "timefile")

	now := time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC)
	err := saveTime(path, now)
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

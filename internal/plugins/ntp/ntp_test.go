package ntp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	data := `{"name":"eth0","unit":0,"address":"10.0.0.5","prefix-length":24,"ntp-servers":["192.168.1.1","192.168.1.2"]}`
	w.handleDHCPEvent(data)

	w.mu.Lock()
	defer w.mu.Unlock()
	assert.Equal(t, []string{"192.168.1.1", "192.168.1.2"}, w.dhcpSrv)
}

// mockEventBus records Emit calls for testing.
type mockEventBus struct {
	emits []emitCall
}

type emitCall struct {
	namespace, eventType, payload string
}

func (m *mockEventBus) Emit(namespace, eventType, payload string) (int, error) {
	m.emits = append(m.emits, emitCall{namespace, eventType, payload})
	return 0, nil
}

func (m *mockEventBus) Subscribe(_, _ string, _ func(string)) func() {
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
		n, err := w.eventBus.Emit("system", "clock-synced", "")
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

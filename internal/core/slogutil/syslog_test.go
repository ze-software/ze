package slogutil

import (
	"bytes"
	"log/syslog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syslogEnvKeys are the keys the syslog tests set, cleared before each one so
// a developer's own environment cannot decide the answer.
var syslogEnvKeys = []string{
	"ze.log", "ze.log.backend", "ze.log.destination", envKeySyslogFacility,
	"ze.log.syslog.bgp", "ze.log.syslog.bgp.server", "ze.log.syslog.web", "ze.log.bgp.server",
}

// clearSyslogEnv removes every spelling of syslogEnvKeys and sets each to
// empty through t.Setenv, so a key ApplyLogConfig writes with env.Set is
// restored when the test ends instead of leaking into the next one.
func clearSyslogEnv(t *testing.T) {
	t.Helper()
	resetEnvCache(t, syslogEnvKeys...)
	for _, key := range syslogEnvKeys {
		t.Setenv(key, "")
	}
	resetEnvCache(t)
}

// TestFacilityForWalksSubsystemNames proves the facility of a subsystem is
// read from the longest ze.log.syslog.<name> key to the shortest, then from
// ze.log.syslog.facility, then daemon, and that a word naming no facility is
// skipped rather than taken.
//
// Method: set keys at each depth and ask facilityFor for subsystems that hit
// each one, plus one that hits none.
func TestFacilityForWalksSubsystemNames(t *testing.T) {
	clearSyslogEnv(t)
	assert.Equal(t, syslog.LOG_DAEMON, facilityFor("bgp.server"), "no key: daemon")

	t.Setenv(envKeySyslogFacility, "local3")
	t.Setenv("ze.log.syslog.bgp", "local7")
	t.Setenv("ze.log.syslog.web", "nonsense")
	resetEnvCache(t)

	assert.Equal(t, syslog.LOG_LOCAL7, facilityFor("bgp.server"), "parent name covers the child")
	assert.Equal(t, syslog.LOG_LOCAL7, facilityFor("bgp"), "exact name")
	assert.Equal(t, syslog.LOG_LOCAL3, facilityFor("plugin"), "unmapped subsystem takes the facility leaf")
	assert.Equal(t, syslog.LOG_LOCAL3, facilityFor("web.server"), "a word naming no facility is skipped")

	t.Setenv("ze.log.syslog.bgp.server", "LOCAL5")
	resetEnvCache(t)
	assert.Equal(t, syslog.LOG_LOCAL5, facilityFor("bgp.server"), "most specific wins, case-insensitive")
}

// TestApplyLogConfigSyslogFacility proves the environment/log/syslog section
// reaches the env keys facilityFor reads, and that a word naming no facility
// is refused with a warning naming the accepted words.
func TestApplyLogConfigSyslogFacility(t *testing.T) {
	clearSyslogEnv(t)

	var warnings bytes.Buffer
	applyLogConfigTo(map[string]map[string]string{
		"log.syslog": {"facility": "local3", "bgp": "local7", "web": "nonsense"},
	}, &warnings)

	assert.Equal(t, syslog.LOG_LOCAL7, facilityFor("bgp.server"))
	assert.Equal(t, syslog.LOG_LOCAL3, facilityFor("web.server"), "the refused word was not applied")
	assert.Contains(t, warnings.String(), `invalid syslog facility "nonsense"`)
	assert.Contains(t, warnings.String(), "local7")
	assert.NotContains(t, warnings.String(), `"local7"`, "a valid word is not warned about")
}

// TestSyslogBackendFacilityAndSeverity proves, through Logger with the
// syslog backend, that each datagram carries the facility of its subsystem
// and the severity of its slog level in its PRI field (facility*8+severity,
// RFC 5424 Section 6.2.1), and that a subsystem's level still decides what is
// sent.
//
// Method: a UDP listener stands in for the collector. bgp is mapped to local7
// (23) and bgp.server is at debug; the rest stays on daemon (3) at info.
func TestSyslogBackendFacilityAndSeverity(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	clearSyslogEnv(t)
	t.Setenv("ze.log", "info")
	t.Setenv("ze.log.bgp.server", "debug")
	t.Setenv("ze.log.backend", "syslog")
	t.Setenv("ze.log.destination", conn.LocalAddr().String())
	t.Setenv("ze.log.syslog.bgp", "local7")
	resetEnvCache(t)
	t.Cleanup(ResetLevelRegistry)

	bgpLog := Logger("bgp.server")
	otherLog := Logger("test.syslog")

	bgpLog.Debug("bgp-debug")
	assert.Equal(t, "<191>", readPRI(t, conn, "bgp-debug"), "local7 debug")
	bgpLog.Error("bgp-error")
	assert.Equal(t, "<187>", readPRI(t, conn, "bgp-error"), "local7 err")
	otherLog.Debug("other-debug") // below info: never sent
	otherLog.Info("other-info")
	assert.Equal(t, "<30>", readPRI(t, conn, "other-info"), "daemon info; the debug record before it was not sent")
	otherLog.Warn("other-warn")
	assert.Equal(t, "<28>", readPRI(t, conn, "other-warn"), "daemon warning")
}

// readPRI reads the next datagram, checks it carries want, and answers its
// leading <PRI> field.
func readPRI(t *testing.T, conn net.PacketConn, want string) string {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	buf := make([]byte, 65535)
	n, _, err := conn.ReadFrom(buf)
	require.NoError(t, err, "no datagram for %q", want)
	msg := string(buf[:n])
	require.Contains(t, msg, want, "datagram out of order")
	end := strings.IndexByte(msg, '>')
	require.Positive(t, end, "datagram has no PRI field: %q", msg)
	return msg[:end+1]
}

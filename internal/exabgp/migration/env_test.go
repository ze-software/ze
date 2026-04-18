package migration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseExaBGPEnv verifies INI file parsed into section/key/value triples.
// VALIDATES: AC-5 -- ExaBGP env file parsed correctly.
// PREVENTS: Malformed INI lines silently dropped or mis-parsed.
func TestParseExaBGPEnv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []ExaEnvEntry
		wantErr bool
	}{
		{
			"basic section and key",
			"[exabgp.daemon]\nuser = nobody\n",
			[]ExaEnvEntry{{Section: "daemon", Key: "user", Value: "nobody"}},
			false,
		},
		{
			"multiple keys",
			"[exabgp.log]\npackets = true\nrib = false\n",
			[]ExaEnvEntry{
				{Section: "log", Key: "packets", Value: "true"},
				{Section: "log", Key: "rib", Value: "false"},
			},
			false,
		},
		{
			"blank lines and comments",
			"# comment\n\n[exabgp.daemon]\n; another comment\nuser = nobody\n",
			[]ExaEnvEntry{{Section: "daemon", Key: "user", Value: "nobody"}},
			false,
		},
		{
			"value with spaces",
			"[exabgp.log]\ndestination = /var/log/exabgp.log\n",
			[]ExaEnvEntry{{Section: "log", Key: "destination", Value: "/var/log/exabgp.log"}},
			false,
		},
		{
			"non-exabgp section ignored",
			"[other]\nfoo = bar\n[exabgp.daemon]\nuser = root\n",
			[]ExaEnvEntry{{Section: "daemon", Key: "user", Value: "root"}},
			false,
		},
		{
			"empty input",
			"",
			nil,
			false,
		},
		{
			"key without section",
			"user = nobody\n",
			nil,
			true,
		},
		{
			"nested section",
			"[exabgp.tcp]\nbind = 0.0.0.0\nport = 179\n",
			[]ExaEnvEntry{
				{Section: "tcp", Key: "bind", Value: "0.0.0.0"},
				{Section: "tcp", Key: "port", Value: "179"},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseExaBGPEnv(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, entries)
		})
	}
}

// TestEnvListenerMapping verifies tcp.bind/port produce "no longer supported"
// comments after spec-env-cleanup.
//
// VALIDATES: Listener keys emit drop comments (migrator does not translate).
// PREVENTS: Listener keys silently converted to invalid Ze config.
func TestEnvListenerMapping(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "tcp", Key: "bind", Value: "0.0.0.0"},
		{Section: "tcp", Key: "port", Value: "179"},
	}

	output := MapEnvToZe(entries)
	assert.Contains(t, output, "# tcp.bind")
	assert.Contains(t, output, "# tcp.port")
	assert.Contains(t, output, "no longer supported")
}

// TestEnvLogMapping verifies per-topic booleans mapped to subsystem levels.
// VALIDATES: AC-6 -- `log.packets = true` -> output contains `bgp.wire debug`.
// PREVENTS: ExaBGP log booleans lost during env migration.
func TestEnvLogMapping(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantSub string
		wantLvl string
	}{
		{"packets true", "packets", "true", "bgp.wire", "debug"},
		{"packets false", "packets", "false", "bgp.wire", "disabled"},
		{"bgp rib true", "rib", "true", "plugin.bgp-rib", "debug"},
		{"configuration true", "configuration", "true", "config", "debug"},
		{"daemon true", "daemon", "true", "daemon", "debug"},
		{"processes true", "processes", "true", "plugin", "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []ExaEnvEntry{
				{Section: "log", Key: tt.key, Value: tt.value},
			}
			output := MapEnvToZe(entries)
			assert.Contains(t, output, tt.wantSub)
			assert.Contains(t, output, tt.wantLvl)
		})
	}
}

// TestEnvCommentOnly verifies unsupported keys produce "no longer supported"
// drop comments.
//
// VALIDATES: AC-23 `debug.pdb = true` -> output contains the drop comment.
// PREVENTS: Unsupported keys silently dropped without user notice.
func TestEnvCommentOnly(t *testing.T) {
	tests := []struct {
		name    string
		section string
		key     string
		value   string
	}{
		{"debug.pdb", "debug", "pdb", "true"},
		{"bgp.connect", "bgp", "connect", "true"},
		{"bgp.accept", "bgp", "accept", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []ExaEnvEntry{
				{Section: tt.section, Key: tt.key, Value: tt.value},
			}
			output := MapEnvToZe(entries)
			assert.Contains(t, output, "#")
			assert.Contains(t, output, "no longer supported")
		})
	}
}

// TestEnvDroppedComment verifies dropped ExaBGP keys emit the
// "no longer supported" comment rather than disappearing silently.
//
// VALIDATES: AC-23/AC-24 drop-comment format.
// PREVENTS: Operators unaware that their config line is ignored.
func TestEnvDroppedComment(t *testing.T) {
	tests := []struct {
		name    string
		section string
		key     string
		value   string
	}{
		{"daemon.drop", "daemon", "drop", "true"},
		{"daemon.daemonize", "daemon", "daemonize", "true"},
		{"daemon.umask", "daemon", "umask", "0137"},
		{"cache.attributes", "cache", "attributes", "true"},
		{"cache.nexthops", "cache", "nexthops", "true"},
		{"api.encoder", "api", "encoder", "json"},
		{"api.respawn", "api", "respawn", "true"},
		{"tcp.attempts", "tcp", "attempts", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []ExaEnvEntry{
				{Section: tt.section, Key: tt.key, Value: tt.value},
			}
			output := MapEnvToZe(entries)
			assert.Contains(t, output, "# "+tt.section+"."+tt.key)
			assert.Contains(t, output, "no longer supported")
		})
	}
}

// TestEnvSurvivingKey verifies surviving keys produce YANG config blocks.
//
// VALIDATES: AC-20 `bgp.openwait` -> YANG `environment { bgp { openwait ...; } }`.
// VALIDATES: AC-21 `tcp.delay` -> `environment { bgp { announce-delay ...m; } }`.
// VALIDATES: AC-22 `daemon.user` -> YANG `environment { daemon { user ...; } }`.
func TestEnvSurvivingKey(t *testing.T) {
	tests := []struct {
		name    string
		section string
		key     string
		value   string
		wantAll []string
	}{
		{"daemon.user", "daemon", "user", "nobody", []string{"environment", "daemon", "user", "nobody"}},
		{"daemon.pid", "daemon", "pid", "/var/run/ze.pid", []string{"environment", "daemon", "pid", "/var/run/ze.pid"}},
		{"bgp.openwait", "bgp", "openwait", "60", []string{"environment", "bgp", "openwait", "60"}},
		{"tcp.delay converts to minutes", "tcp", "delay", "5", []string{"environment", "bgp", "announce-delay", "5m"}},
		{"debug.pprof", "debug", "pprof", ":6060", []string{"environment", "pprof", ":6060"}},
		{"api.ack", "api", "ack", "false", []string{"environment", "exabgp", "api", "ack", "false"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []ExaEnvEntry{
				{Section: tt.section, Key: tt.key, Value: tt.value},
			}
			output := MapEnvToZe(entries)
			for _, want := range tt.wantAll {
				assert.Contains(t, output, want,
					"output for %s.%s should contain %q", tt.section, tt.key, want)
			}
		})
	}
}

// TestEnvUnknownKey verifies unknown keys produce a comment for user review.
// VALIDATES: Unrecognized env keys emit "unknown ExaBGP setting" comment.
// PREVENTS: Unknown keys silently dropped without user notice.
func TestEnvUnknownKey(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "custom", Key: "thing", Value: "x"},
	}
	output := MapEnvToZe(entries)
	assert.Contains(t, output, "unknown ExaBGP setting")
	assert.Contains(t, output, "custom.thing")
}

// TestEnvDaemonUser verifies daemon.user maps to Ze config.
// VALIDATES: daemon.user produces environment { daemon { user ... } } output.
// PREVENTS: daemon.user entry lost or mapped to wrong config path.
func TestEnvDaemonUser(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "daemon", Key: "user", Value: "nobody"},
	}
	output := MapEnvToZe(entries)
	assert.Contains(t, output, "daemon")
	assert.Contains(t, output, "user")
	assert.Contains(t, output, "nobody")
}

// TestEnvLogLevel verifies log.level maps to Ze log level.
// VALIDATES: log.level produces level entry in merged log block.
// PREVENTS: Log level silently dropped during env migration.
func TestEnvLogLevel(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "log", Key: "level", Value: "DEBUG"},
	}
	output := MapEnvToZe(entries)
	assert.Contains(t, output, "level")
	assert.Contains(t, output, "debug")
}

// TestEnvLogDestination verifies log.destination maps to Ze config.
// VALIDATES: log.destination produces destination entry in merged log block.
// PREVENTS: Log destination silently dropped during env migration.
func TestEnvLogDestination(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "log", Key: "destination", Value: "syslog"},
	}
	output := MapEnvToZe(entries)
	assert.Contains(t, output, "destination")
	assert.Contains(t, output, "syslog")
}

// TestEnvFullFile verifies a complete env file produces valid output.
// VALIDATES: End-to-end env file migration with all key types.
// PREVENTS: Key interactions causing missing or duplicated output.
func TestEnvFullFile(t *testing.T) {
	input := strings.Join([]string{
		"[exabgp.daemon]",
		"user = nobody",
		"",
		"[exabgp.log]",
		"level = INFO",
		"packets = true",
		"rib = false",
		"destination = syslog",
		"",
		"[exabgp.tcp]",
		"bind = 0.0.0.0",
		"port = 179",
		"",
		"[exabgp.debug]",
		"pdb = true",
		"",
	}, "\n")

	entries, err := ParseExaBGPEnv(input)
	require.NoError(t, err)
	assert.Len(t, entries, 8)

	output := MapEnvToZe(entries)
	// Should have drop comments for tcp.bind, tcp.port, debug.pdb.
	assert.Contains(t, output, "# tcp.bind")
	assert.Contains(t, output, "# tcp.port")
	assert.Contains(t, output, "# debug.pdb")
	assert.Contains(t, output, "no longer supported")
	// Should have subsystem log entries.
	assert.Contains(t, output, "bgp.wire debug")
	assert.Contains(t, output, "plugin.bgp-rib disabled")
}

// TestEnvPortBoundary verifies port values are validated.
// VALIDATES: Boundary test for tcp.port 1-65535.
// PREVENTS: Invalid port values silently accepted.
func TestEnvPortBoundary(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid port 1", "1", false},
		{"valid port 179", "179", false},
		{"valid port 65535", "65535", false},
		{"invalid port 0", "0", true},
		{"invalid port 65536", "65536", true},
		{"invalid port negative", "-1", true},
		{"invalid port text", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []ExaEnvEntry{
				{Section: "tcp", Key: "port", Value: tt.value},
			}
			err := ValidateEnvEntries(entries)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

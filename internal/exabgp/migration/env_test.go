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

// TestEnvListenerMapping verifies tcp.bind/port produce comments.
// VALIDATES: AC-5 -- tcp.port/bind emit comments.
// PREVENTS: Listener keys silently converted to invalid Ze config.
func TestEnvListenerMapping(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "tcp", Key: "bind", Value: "0.0.0.0"},
		{Section: "tcp", Key: "port", Value: "179"},
	}

	output := MapEnvToZe(entries)
	assert.Contains(t, output, "# tcp.bind")
	assert.Contains(t, output, "# tcp.port")
	assert.Contains(t, output, "per-peer")
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
		{"rib true", "rib", "true", "plugin.rib", "debug"},
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

// TestEnvCommentOnly verifies unsupported keys produce comments.
// VALIDATES: AC-7 -- `debug.pdb = true` -> output contains comment about Python-only.
// PREVENTS: Unsupported keys silently dropped without user notice.
func TestEnvCommentOnly(t *testing.T) {
	tests := []struct {
		name    string
		section string
		key     string
		value   string
		want    string
	}{
		{"debug.pdb", "debug", "pdb", "true", "Python-only"},
		{"bgp.connect", "bgp", "connect", "true", "per-peer"},
		{"bgp.accept", "bgp", "accept", "true", "per-peer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := []ExaEnvEntry{
				{Section: tt.section, Key: tt.key, Value: tt.value},
			}
			output := MapEnvToZe(entries)
			assert.Contains(t, output, "#")
			assert.Contains(t, output, tt.want)
		})
	}
}

// TestEnvDaemonUser verifies daemon.user maps to Ze config.
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
func TestEnvLogLevel(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "log", Key: "level", Value: "DEBUG"},
	}
	output := MapEnvToZe(entries)
	assert.Contains(t, output, "level")
	assert.Contains(t, output, "debug")
}

// TestEnvLogDestination verifies log.destination maps to Ze config.
func TestEnvLogDestination(t *testing.T) {
	entries := []ExaEnvEntry{
		{Section: "log", Key: "destination", Value: "syslog"},
	}
	output := MapEnvToZe(entries)
	assert.Contains(t, output, "destination")
	assert.Contains(t, output, "syslog")
}

// TestEnvFullFile verifies a complete env file produces valid output.
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
	// Should have comments for tcp.bind, tcp.port, debug.pdb
	assert.Contains(t, output, "# tcp.bind")
	assert.Contains(t, output, "# tcp.port")
	assert.Contains(t, output, "Python-only")
	// Should have config entries
	assert.Contains(t, output, "bgp.wire debug")
	assert.Contains(t, output, "plugin.rib disabled")
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

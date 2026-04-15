package l2tp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp/schema"
)

// TestConfig_MissingBlockReturnsZero ensures absence of any l2tp config
// yields a disabled, empty Parameters value.
//
// VALIDATES: Parameters zero value is a disabled subsystem (Start is a no-op).
func TestConfig_MissingBlockReturnsZero(t *testing.T) {
	tree := zeconfig.NewTree()
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.False(t, p.Enabled)
	assert.Empty(t, p.ListenAddrs)
	assert.Equal(t, uint16(0), p.MaxTunnels)
}

// TestConfig_MinimalListen uses `enabled true` as a filler in an otherwise
// empty l2tp{} block.
//
// VALIDATES: AC-1 -- minimal listen config produces one AddrPort.
func TestConfig_MinimalListen(t *testing.T) {
	const src = `l2tp {
	enabled true
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.True(t, p.Enabled)
	require.Len(t, p.ListenAddrs, 1)
	assert.Equal(t, "127.0.0.1:1701", p.ListenAddrs[0].String())
	assert.Equal(t, 60*time.Second, p.HelloInterval)
}

// TestConfig_PresenceImpliesEnabled verifies that l2tp{} with any setting
// but no explicit `enabled` is still enabled.
//
// VALIDATES: presence of l2tp{} block implies enabled.
func TestConfig_PresenceImpliesEnabled(t *testing.T) {
	const src = `l2tp {
	shared-secret s3cr3t
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.True(t, p.Enabled, "l2tp{} with content but no enabled leaf should be enabled")
	assert.Equal(t, "s3cr3t", p.SharedSecret)
}

// TestConfig_ExplicitDisable verifies that `enabled false` disables even
// when other settings are present.
//
// VALIDATES: enabled false overrides the presence-implies-enabled default.
func TestConfig_ExplicitDisable(t *testing.T) {
	const src = `l2tp {
	enabled false
	shared-secret s3cr3t
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.False(t, p.Enabled)
}

// TestConfig_MultipleServers parses multiple list entries.
//
// VALIDATES: list server { ... } is ordered and all addresses collected.
func TestConfig_MultipleServers(t *testing.T) {
	const src = `l2tp {
	enabled true
}
environment {
	l2tp {
		server a {
			ip 127.0.0.1
			port 1701
		}
		server b {
			ip 127.0.0.2
			port 1702
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	require.Len(t, p.ListenAddrs, 2)
	got := []string{p.ListenAddrs[0].String(), p.ListenAddrs[1].String()}
	assert.Contains(t, got, "127.0.0.1:1701")
	assert.Contains(t, got, "127.0.0.2:1702")
}

// TestConfig_BadPortRejected exercises the parseListen error path with
// port=0.
//
// VALIDATES: boundary -- port 0 is the first invalid value below the range.
func TestConfig_BadPortRejected(t *testing.T) {
	const src = `l2tp {
	enabled true
}
environment {
	l2tp {
		server bad {
			ip 127.0.0.1
			port 0
		}
	}
}`
	tree := loadTree(t, src)
	_, err := l2tp.ExtractParameters(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

// TestConfig_HelloIntervalOverride honors a custom hello-interval.
func TestConfig_HelloIntervalOverride(t *testing.T) {
	const src = `l2tp {
	enabled true
	hello-interval 30
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, p.HelloInterval)
}

// TestConfig_HelloIntervalZeroRejected rejects the zero boundary.
//
// VALIDATES: boundary -- hello-interval=0 invalid below; 1 is last valid.
func TestConfig_HelloIntervalZeroRejected(t *testing.T) {
	const src = `l2tp {
	enabled true
	hello-interval 0
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	_, err := l2tp.ExtractParameters(tree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hello-interval")
}

// TestConfig_MaxTunnels passes the integer through.
func TestConfig_MaxTunnels(t *testing.T) {
	const src = `l2tp {
	enabled true
	max-tunnels 1024
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.Equal(t, uint16(1024), p.MaxTunnels)
}

// TestConfig_MaxTunnelsZeroIsUnbounded captures the contract that
// `max-tunnels 0` (and the unset default) both mean "no ze-side limit".
// The YANG description documents this; the test locks it in.
//
// VALIDATES: max-tunnels=0 semantic documented in ze-l2tp-conf.yang.
func TestConfig_MaxTunnelsZeroIsUnbounded(t *testing.T) {
	const src = `l2tp {
	enabled true
	max-tunnels 0
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.Equal(t, uint16(0), p.MaxTunnels, "max-tunnels=0 should parse as zero and be interpreted as unbounded by consumers")
}

// TestConfig_MaxSessions passes the integer through.
//
// VALIDATES: max-sessions config extraction.
func TestConfig_MaxSessions(t *testing.T) {
	const src = `l2tp {
	enabled true
	max-sessions 100
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.Equal(t, uint16(100), p.MaxSessions)
}

// TestConfig_MaxSessionsZeroIsUnbounded captures the contract that
// max-sessions 0 (and the unset default) means "no limit per tunnel".
func TestConfig_MaxSessionsZeroIsUnbounded(t *testing.T) {
	const src = `l2tp {
	enabled true
	max-sessions 0
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	assert.Equal(t, uint16(0), p.MaxSessions)
}

// TestConfig_IPv6Listener — symmetry check for the IPv4 test.
//
// VALIDATES: AC-2 (partial) -- YANG zt:listener accepts IPv6 addresses
// and ExtractParameters produces a correctly-scoped AddrPort.
func TestConfig_IPv6Listener(t *testing.T) {
	const src = `l2tp {
	enabled true
}
environment {
	l2tp {
		server main {
			ip ::1
			port 1701
		}
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	require.Len(t, p.ListenAddrs, 1)
	assert.True(t, p.ListenAddrs[0].Addr().Is6(), "listener address should be IPv6")
	assert.Equal(t, uint16(1701), p.ListenAddrs[0].Port())
}

// TestConfig_PortBoundary covers the last valid and first invalid-above
// for UDP port. Port 1 is tested implicitly via ephemeral test binds; 0
// and 65536 are boundary misses that must reject.
//
// VALIDATES: boundary -- port 1..65535 accepted; 65536 rejected at parse
// time (ze-config's uint16 coercion catches it before YANG or ze-l2tp).
func TestConfig_PortBoundary(t *testing.T) {
	// Last valid (65535) goes all the way through.
	srcLast := `l2tp {
	enabled true
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 65535
		}
	}
}`
	tree := loadTree(t, srcLast)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	require.Len(t, p.ListenAddrs, 1)
	assert.Equal(t, uint16(65535), p.ListenAddrs[0].Port())

	// First invalid above: 65536 is rejected by the ze-config parser's
	// uint16 coercion before ExtractParameters runs. We call LoadConfig
	// directly (bypassing loadTree's require.NoError) to capture the
	// parse-time rejection.
	srcOver := `l2tp {
	enabled true
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 65536
		}
	}
}`
	_, loadErr := zeconfig.LoadConfig(srcOver, "test.conf", nil)
	require.Error(t, loadErr, "port 65536 should be rejected at config parse time")
}

// TestConfig_HelloIntervalBoundary covers the numeric boundary for
// hello-interval. Test is YANG-agnostic: runs through ExtractParameters
// directly with hello-interval at 1 (last valid below) and 3600 (last
// valid above).
//
// VALIDATES: boundary -- hello-interval positive integer within
// parseable uint16 range produces the expected Duration.
func TestConfig_HelloIntervalBoundary(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		expect  time.Duration
		wantErr bool
	}{
		{"minimum-valid", "1", 1 * time.Second, false},
		{"recommended-default", "60", 60 * time.Second, false},
		{"upper-typical", "3600", 3600 * time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `l2tp {
	enabled true
	hello-interval ` + tc.value + `
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
			tree := loadTree(t, src)
			p, err := l2tp.ExtractParameters(tree)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expect, p.HelloInterval)
		})
	}
}

// loadTree parses the given ze-config source via the public LoadConfig API
// and returns the resulting tree. It centralizes YANG loading so each
// test stays focused on assertions.
func loadTree(t *testing.T, src string) *zeconfig.Tree {
	t.Helper()
	res, err := zeconfig.LoadConfig(src, "test.conf", nil)
	require.NoError(t, err)
	return res.Tree
}

package l2tp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/cmd/clear/yang"
	_ "github.com/ze-software/ze/internal/component/cmd/show/yang"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
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

// TestConfig_DefaultSafetyCaps ensures an enabled L2TP subsystem has finite
// admission caps even when the operator does not set max-tunnels or
// max-sessions.
//
// VALIDATES: deployment default resource caps are non-zero.
// PREVENTS: an omitted L2TP cap from leaving tunnel or session admission
// unbounded in production configs.
func TestConfig_DefaultSafetyCaps(t *testing.T) {
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
	assert.Equal(t, l2tp.DefaultMaxTunnels, p.MaxTunnels)
	assert.Equal(t, l2tp.DefaultMaxSessions, p.MaxSessions)
}

// TestConfig_HelloRetries verifies the dead-peer detection threshold leaf:
// it defaults to DefaultHelloRetries when unset, parses an explicit value,
// and accepts 0 (which disables dead-peer detection).
//
// VALIDATES: spec-l2tp-dead-peer-detection AC-5.
func TestConfig_HelloRetries(t *testing.T) {
	t.Run("default when unset", func(t *testing.T) {
		tree := loadTree(t, "l2tp {\n\tenabled true\n}")
		p, err := l2tp.ExtractParameters(tree)
		require.NoError(t, err)
		assert.Equal(t, l2tp.DefaultHelloRetries, p.HelloRetries)
	})
	t.Run("explicit value", func(t *testing.T) {
		tree := loadTree(t, "l2tp {\n\tenabled true\n\thello-retries 4\n}")
		p, err := l2tp.ExtractParameters(tree)
		require.NoError(t, err)
		assert.Equal(t, uint8(4), p.HelloRetries)
	})
	t.Run("zero disables", func(t *testing.T) {
		tree := loadTree(t, "l2tp {\n\tenabled true\n\thello-retries 0\n}")
		p, err := l2tp.ExtractParameters(tree)
		require.NoError(t, err)
		assert.Equal(t, uint8(0), p.HelloRetries)
	})
}

// TestConfig_DefaultAuthRequiresCHAP verifies that enabling L2TP defaults to
// a real PPP Auth-Protocol and does not allow no-auth fallback unless the
// operator opts in.
//
// VALIDATES: mandatory PPP auth is the deployment default.
// PREVENTS: L2TP sessions from starting with AuthMethodNone by omission.
func TestConfig_DefaultAuthRequiresCHAP(t *testing.T) {
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
	assert.Equal(t, ppp.AuthMethodCHAPMD5, p.AuthMethod)
	assert.False(t, p.AllowNoAuth)
}

// TestConfig_AllowNoAuthRequiresExplicitLeaf keeps no-auth possible for lab
// peers while making the deployment opt-in visible in config.
//
// VALIDATES: allow-no-auth true is parsed explicitly.
// PREVENTS: implicit no-auth fallback when the leaf is absent.
func TestConfig_AllowNoAuthRequiresExplicitLeaf(t *testing.T) {
	const src = `l2tp {
	enabled true
	allow-no-auth true
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
	assert.True(t, p.AllowNoAuth)
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
	_, err := zeconfig.LoadConfig(src, "test.conf", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hello-interval")
}

// TestConfig_MaxTunnels passes the integer through.
func TestConfig_MaxTunnels(t *testing.T) {
	const src = `l2tp {
	enabled true
	max-tunnels 2048
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
	assert.Equal(t, uint16(2048), p.MaxTunnels)
}

// TestConfig_MaxTunnelsZeroIsUnbounded captures the contract that an
// explicit `max-tunnels 0` means "no ze-side limit". The unset default
// remains finite for deployment safety.
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

// TestConfig_MaxSessionsZeroIsUnbounded captures the contract that an
// explicit max-sessions 0 means "no limit per tunnel". The unset default
// remains finite for deployment safety.
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

// --- Authentication container tests ---

func TestConfig_AuthTimeoutDefault(t *testing.T) {
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
	assert.Equal(t, 30*time.Second, p.AuthTimeout)
	assert.Equal(t, time.Duration(0), p.ReauthInterval)
}

func TestConfig_AuthTimeoutOverride(t *testing.T) {
	const src = `l2tp {
	enabled true
	authentication {
		timeout 60
	}
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
	assert.Equal(t, 60*time.Second, p.AuthTimeout)
}

func TestConfig_AuthTimeoutBoundary(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		expect  time.Duration
		wantErr bool
	}{
		{"minimum-valid", "1", 1 * time.Second, false},
		{"maximum-valid", "3600", 3600 * time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `l2tp {
	enabled true
	authentication {
		timeout ` + tc.value + `
	}
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
			assert.Equal(t, tc.expect, p.AuthTimeout)
		})
	}
}

func TestConfig_AuthTimeoutZeroRejected(t *testing.T) {
	const src = `l2tp {
	enabled true
	authentication {
		timeout 0
	}
}`
	_, err := zeconfig.LoadConfig(src, "test.conf", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestConfig_ReauthIntervalOverride(t *testing.T) {
	const src = `l2tp {
	enabled true
	authentication {
		reauth-interval 300
	}
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
	assert.Equal(t, 300*time.Second, p.ReauthInterval)
}

func TestConfig_ReauthIntervalBoundary(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		expect  time.Duration
		wantErr bool
	}{
		{"disabled", "0", 0, false},
		{"floor", "5", 5 * time.Second, false},
		{"just-above-floor", "6", 6 * time.Second, false},
		{"just-below-max", "86399", 86399 * time.Second, false},
		{"maximum", "86400", 86400 * time.Second, false},
		{"above-max", "86401", 0, true},
		{"far-above-max", "100000", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `l2tp {
	enabled true
	authentication {
		reauth-interval ` + tc.value + `
	}
}
environment {
	l2tp {
		server main {
			ip 127.0.0.1
			port 1701
		}
	}
}`
			res, err := zeconfig.LoadConfig(src, "test.conf", nil)
			if tc.wantErr {
				require.Error(t, err, "reauth-interval=%s must be rejected by YANG range 0|5..86400", tc.value)
				return
			}
			require.NoError(t, err)
			p, err := l2tp.ExtractParameters(res.Tree)
			require.NoError(t, err)
			assert.Equal(t, tc.expect, p.ReauthInterval)
		})
	}
}

func TestConfig_ReauthIntervalGapRejected(t *testing.T) {
	for _, val := range []string{"1", "2", "3", "4"} {
		t.Run("value-"+val, func(t *testing.T) {
			src := `l2tp {
	enabled true
	authentication {
		reauth-interval ` + val + `
	}
}`
			_, err := zeconfig.LoadConfig(src, "test.conf", nil)
			require.Error(t, err, "reauth-interval=%s must be rejected (gap between 0 and 5)", val)
		})
	}
}

// --- NCP container tests ---

func TestConfig_NCPDefaults(t *testing.T) {
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
	assert.True(t, p.EnableIPCP)
	assert.True(t, p.EnableIPv6CP)
	assert.Equal(t, 30*time.Second, p.NCPTimeout)
}

func TestConfig_NCPDisableIPCP(t *testing.T) {
	const src = `l2tp {
	enabled true
	ncp {
		enable-ipcp false
	}
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
	assert.False(t, p.EnableIPCP)
	assert.True(t, p.EnableIPv6CP)
}

func TestConfig_NCPDisableIPv6CP(t *testing.T) {
	const src = `l2tp {
	enabled true
	ncp {
		enable-ipv6cp false
	}
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
	assert.True(t, p.EnableIPCP)
	assert.False(t, p.EnableIPv6CP)
}

func TestConfig_NCPTimeoutOverride(t *testing.T) {
	const src = `l2tp {
	enabled true
	ncp {
		timeout 120
	}
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
	assert.Equal(t, 120*time.Second, p.NCPTimeout)
}

func TestConfig_NCPTimeoutBoundary(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		expect time.Duration
	}{
		{"minimum-valid", "1", 1 * time.Second},
		{"maximum-valid", "3600", 3600 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `l2tp {
	enabled true
	ncp {
		timeout ` + tc.value + `
	}
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
			assert.Equal(t, tc.expect, p.NCPTimeout)
		})
	}
}

func TestConfig_NCPTimeoutZeroRejected(t *testing.T) {
	const src = `l2tp {
	enabled true
	ncp {
		timeout 0
	}
}`
	_, err := zeconfig.LoadConfig(src, "test.conf", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// loadTree parses the given ze-config source via the public LoadConfig API
// and returns the resulting tree. It centralizes YANG loading so each
// test stays focused on assertions.
// -----------------------------------------------------------------
// Dial targets + relay bindings (spec-followup-l2tp-call AC-6)
// -----------------------------------------------------------------

// TestConfig_RemoteBasic parses a fully-specified remote dial target.
//
// VALIDATES: AC-6 -- remote name/address/port/shared-secret/outgoing-calls.
func TestConfig_RemoteBasic(t *testing.T) {
	const src = `l2tp {
	enabled true
	remote lns1 {
		address 203.0.113.5
		port 1701
		shared-secret hunter2
		outgoing-calls true
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	require.Len(t, p.Remotes, 1)
	r := p.Remotes[0]
	assert.Equal(t, "lns1", r.Name)
	assert.Equal(t, "203.0.113.5:1701", r.Address.String())
	assert.Equal(t, "hunter2", r.SharedSecret)
	assert.True(t, r.OutgoingCalls)
	got, ok := p.LookupRemote("lns1")
	require.True(t, ok)
	assert.Equal(t, r, got)
	_, ok = p.LookupRemote("nope")
	assert.False(t, ok)
}

// TestConfig_RemoteDefaultPort omits port and expects the RFC 2661 default.
//
// VALIDATES: AC-6 -- port defaults to 1701; outgoing-calls defaults false.
func TestConfig_RemoteDefaultPort(t *testing.T) {
	const src = `l2tp {
	enabled true
	remote r {
		address 198.51.100.9
	}
}`
	tree := loadTree(t, src)
	p, err := l2tp.ExtractParameters(tree)
	require.NoError(t, err)
	require.Len(t, p.Remotes, 1)
	assert.Equal(t, "198.51.100.9:1701", p.Remotes[0].Address.String())
	assert.False(t, p.Remotes[0].OutgoingCalls)
	assert.Empty(t, p.Remotes[0].SharedSecret)
}

// TestConfig_RemotePortBoundary exercises the remote port range 1..65535.
//
// VALIDATES: boundary -- 65535 is last valid; 0 is invalid below.
func TestConfig_RemotePortBoundary(t *testing.T) {
	last := `l2tp {
	enabled true
	remote r {
		address 198.51.100.9
		port 65535
	}
}`
	p, err := l2tp.ExtractParameters(loadTree(t, last))
	require.NoError(t, err)
	require.Len(t, p.Remotes, 1)
	assert.Equal(t, uint16(65535), p.Remotes[0].Address.Port())

	// port 0 is rejected by the YANG native range constraint at config load
	// (max-native validation, config-option.md), before ExtractParameters.
	invalid := `l2tp {
	enabled true
	remote r {
		address 198.51.100.9
		port 0
	}
}`
	_, err = zeconfig.LoadConfig(invalid, "test.conf", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

// TestConfig_RemoteMissingAddress rejects a remote with no address.
//
// VALIDATES: AC-6 -- address is mandatory (a dial target needs an endpoint).
func TestConfig_RemoteMissingAddress(t *testing.T) {
	const src = `l2tp {
	enabled true
	remote r {
		port 1701
	}
}`
	_, err := l2tp.ExtractParameters(loadTree(t, src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "address")
}

// TestConfig_MultipleRemotes parses several dial targets and preserves order.
func TestConfig_MultipleRemotes(t *testing.T) {
	const src = `l2tp {
	enabled true
	remote alpha {
		address 10.0.0.1
	}
	remote beta {
		address 10.0.0.2
		port 1702
	}
}`
	p, err := l2tp.ExtractParameters(loadTree(t, src))
	require.NoError(t, err)
	require.Len(t, p.Remotes, 2)
	names := []string{p.Remotes[0].Name, p.Remotes[1].Name}
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}

// TestConfig_RelayBinding parses a relay binding referencing a declared remote.
//
// VALIDATES: AC-6 -- per-PPPoE-service relay binding maps service -> remote.
func TestConfig_RelayBinding(t *testing.T) {
	const src = `l2tp {
	enabled true
	remote retail {
		address 203.0.113.7
	}
	relay wholesale {
		remote retail
	}
}`
	p, err := l2tp.ExtractParameters(loadTree(t, src))
	require.NoError(t, err)
	require.Len(t, p.Relays, 1)
	assert.Equal(t, "wholesale", p.Relays[0].Service)
	assert.Equal(t, "retail", p.Relays[0].Remote)
}

// TestConfig_RelayUnknownRemote rejects a relay that names no declared remote.
//
// VALIDATES: AC-6 -- referential integrity enforced in Go (no YANG leafref).
func TestConfig_RelayUnknownRemote(t *testing.T) {
	const src = `l2tp {
	enabled true
	relay wholesale {
		remote ghost
	}
}`
	_, err := l2tp.ExtractParameters(loadTree(t, src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown remote")
}

func loadTree(t *testing.T, src string) *zeconfig.Tree {
	t.Helper()
	res, err := zeconfig.LoadConfig(src, "test.conf", nil)
	require.NoError(t, err)
	return res.Tree
}

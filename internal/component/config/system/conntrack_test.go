package system_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/system"
)

func TestConntrackConfigParse(t *testing.T) {
	tree := config.NewTree()
	sys := tree.GetOrCreateContainer("system")
	ct := sys.GetOrCreateContainer("conntrack")
	ct.SetSlice("module", []string{"ftp", "sip"})
	ct.Set("table-size", "262144")
	ct.Set("hash-size", "65536")
	ct.Set("expect-max", "1024")
	ct.Set("accounting", "")
	ct.Set("timestamp", "")
	ct.Set("log-invalid", "tcp")

	timeout := ct.GetOrCreateContainer("timeout")
	timeout.Set("generic", "600")

	tcp := timeout.GetOrCreateContainer("tcp")
	tcp.Set("established", "432000")
	tcp.Set("syn-sent", "120")

	udp := timeout.GetOrCreateContainer("udp")
	udp.Set("timeout", "30")
	udp.Set("stream", "120")

	tcpBehavior := ct.GetOrCreateContainer("tcp")
	tcpBehavior.Set("be-liberal", "false")
	tcpBehavior.Set("loose", "true")
	tcpBehavior.Set("max-retrans", "3")

	sc := system.ExtractSystemConfig(tree)
	cc := sc.Conntrack

	assert.Equal(t, []string{"ftp", "sip"}, cc.Modules)
	assert.Equal(t, 262144, cc.TableSize)
	assert.Equal(t, 65536, cc.HashSize)
	assert.Equal(t, 1024, cc.ExpectMax)
	assert.True(t, cc.Accounting)
	assert.True(t, cc.Timestamp)
	assert.Equal(t, "tcp", cc.LogInvalid)
	assert.Equal(t, 600, cc.TimeoutGeneric)
	assert.Equal(t, 432000, cc.TimeoutTCP.Established)
	assert.Equal(t, 120, cc.TimeoutTCP.SynSent)
	assert.Equal(t, 30, cc.TimeoutUDP.Timeout)
	assert.Equal(t, 120, cc.TimeoutUDP.Stream)

	require.NotNil(t, cc.TCPBehavior.BeLiberal)
	assert.False(t, *cc.TCPBehavior.BeLiberal)
	require.NotNil(t, cc.TCPBehavior.Loose)
	assert.True(t, *cc.TCPBehavior.Loose)
	assert.Equal(t, 3, cc.TCPBehavior.MaxRetrans)
}

func TestConntrackConfigParse_NoBlock(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system")

	sc := system.ExtractSystemConfig(tree)
	assert.Empty(t, sc.Conntrack.Modules)
	assert.Zero(t, sc.Conntrack.TableSize)
	assert.False(t, sc.Conntrack.HasConfig())
}

func TestConntrackConfigParseFromMap(t *testing.T) {
	tree := map[string]any{
		"system": map[string]any{
			"conntrack": map[string]any{
				"module":      []any{"ftp", "sip", "h323"},
				"table-size":  "262144",
				"hash-size":   "65536",
				"expect-max":  "1024",
				"accounting":  "",
				"timestamp":   "",
				"log-invalid": "tcp",
				"timeout": map[string]any{
					"generic": "600",
					"tcp": map[string]any{
						"established": "432000",
						"close-wait":  "60",
					},
					"udp": map[string]any{
						"timeout": "30",
						"stream":  "120",
					},
					"icmp": map[string]any{
						"timeout": "30",
					},
					"icmpv6": map[string]any{
						"timeout": "30",
					},
					"gre": map[string]any{
						"timeout": "30",
						"stream":  "180",
					},
					"sctp": map[string]any{
						"established": "210",
						"closed":      "10",
					},
					"dccp": map[string]any{
						"open":    "43200",
						"request": "240",
					},
				},
				"tcp": map[string]any{
					"be-liberal":         "false",
					"loose":              "true",
					"max-retrans":        "3",
					"ignore-invalid-rst": "false",
				},
			},
		},
	}

	cc := system.ExtractConntrackFromMap(tree)

	assert.Equal(t, []string{"ftp", "sip", "h323"}, cc.Modules)
	assert.Equal(t, 262144, cc.TableSize)
	assert.Equal(t, 65536, cc.HashSize)
	assert.Equal(t, 1024, cc.ExpectMax)
	assert.True(t, cc.Accounting)
	assert.True(t, cc.Timestamp)
	assert.Equal(t, "tcp", cc.LogInvalid)
	assert.Equal(t, 600, cc.TimeoutGeneric)
	assert.Equal(t, 432000, cc.TimeoutTCP.Established)
	assert.Equal(t, 60, cc.TimeoutTCP.CloseWait)
	assert.Equal(t, 30, cc.TimeoutUDP.Timeout)
	assert.Equal(t, 120, cc.TimeoutUDP.Stream)
	assert.Equal(t, 30, cc.TimeoutICMP.Timeout)
	assert.Equal(t, 30, cc.TimeoutICMPv6.Timeout)
	assert.Equal(t, 30, cc.TimeoutGRE.Timeout)
	assert.Equal(t, 180, cc.TimeoutGRE.Stream)
	assert.Equal(t, 210, cc.TimeoutSCTP.Established)
	assert.Equal(t, 10, cc.TimeoutSCTP.Closed)
	assert.Equal(t, 43200, cc.TimeoutDCCP.Open)
	assert.Equal(t, 240, cc.TimeoutDCCP.Request)

	require.NotNil(t, cc.TCPBehavior.BeLiberal)
	assert.False(t, *cc.TCPBehavior.BeLiberal)
	require.NotNil(t, cc.TCPBehavior.Loose)
	assert.True(t, *cc.TCPBehavior.Loose)
	assert.Equal(t, 3, cc.TCPBehavior.MaxRetrans)
	require.NotNil(t, cc.TCPBehavior.IgnoreInvalidRST)
	assert.False(t, *cc.TCPBehavior.IgnoreInvalidRST)
}

func TestConntrackConfigParseFromMap_Empty(t *testing.T) {
	cc1 := system.ExtractConntrackFromMap(map[string]any{})
	assert.False(t, cc1.HasConfig())
	cc2 := system.ExtractConntrackFromMap(map[string]any{
		"system": map[string]any{},
	})
	assert.False(t, cc2.HasConfig())
}

func TestConntrackModuleValidation(t *testing.T) {
	valid := []string{"ftp", "h323", "sip", "pptp", "tftp", "nfs", "sane", "irc", "amanda", "netbios-ns", "snmp", "sqlnet"}
	for _, m := range valid {
		assert.True(t, system.ValidConntrackModule(m), "expected %q to be valid", m)
	}

	invalid := []string{"broadcast", "unknown", "", "ftp; rm -rf /"}
	for _, m := range invalid {
		assert.False(t, system.ValidConntrackModule(m), "expected %q to be invalid", m)
	}
}

func TestConntrackValidateModules(t *testing.T) {
	cc := system.ConntrackConfig{Modules: []string{"ftp", "sip"}}
	assert.NoError(t, cc.ValidateModules())

	cc.Modules = []string{"ftp", "sqlnet"}
	assert.NoError(t, cc.ValidateModules())

	cc.Modules = []string{"ftp", "nonexistent"}
	assert.Error(t, cc.ValidateModules())
}

func TestConntrackSysctlMapping_TableSize(t *testing.T) {
	cc := system.ConntrackConfig{TableSize: 262144}
	keys := cc.ConntrackSysctlKeys()
	assert.Equal(t, "262144", keys["net.netfilter.nf_conntrack_max"])
}

func TestConntrackSysctlMapping_HashSize(t *testing.T) {
	cc := system.ConntrackConfig{HashSize: 65536}
	keys := cc.ConntrackSysctlKeys()
	assert.Equal(t, "65536", keys["net.netfilter.nf_conntrack_buckets"])
}

func TestConntrackSysctlMapping_ExpectMax(t *testing.T) {
	cc := system.ConntrackConfig{ExpectMax: 1024}
	keys := cc.ConntrackSysctlKeys()
	assert.Equal(t, "1024", keys["net.netfilter.nf_conntrack_expect_max"])
}

func TestConntrackSysctlMapping_GlobalFlags(t *testing.T) {
	cc := system.ConntrackConfig{
		Accounting: true,
		Timestamp:  true,
		Checksum:   true,
	}
	keys := cc.ConntrackSysctlKeys()
	assert.Equal(t, "1", keys["net.netfilter.nf_conntrack_acct"])
	assert.Equal(t, "1", keys["net.netfilter.nf_conntrack_timestamp"])
	assert.Equal(t, "1", keys["net.netfilter.nf_conntrack_checksum"])
}

func TestConntrackSysctlMapping_LogInvalid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tcp", "6"},
		{"udp", "17"},
		{"icmp", "1"},
		{"icmpv6", "58"},
		{"all", "255"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cc := system.ConntrackConfig{LogInvalid: tt.input}
			keys := cc.ConntrackSysctlKeys()
			assert.Equal(t, tt.expected, keys["net.netfilter.nf_conntrack_log_invalid"])
		})
	}
}

func TestConntrackSysctlMapping_Timeouts(t *testing.T) {
	cc := system.ConntrackConfig{
		TimeoutGeneric: 600,
		TimeoutTCP: system.TCPTimeouts{
			Established: 432000,
			SynSent:     120,
			SynRecv:     60,
			FinWait:     120,
			CloseWait:   60,
			LastAck:     30,
			TimeWait:    120,
			Close:       10,
		},
		TimeoutUDP:    system.UDPTimeouts{Timeout: 30, Stream: 120},
		TimeoutICMP:   system.ICMPTimeouts{Timeout: 30},
		TimeoutICMPv6: system.ICMPTimeouts{Timeout: 30},
		TimeoutGRE:    system.GRETimeouts{Timeout: 30, Stream: 180},
		TimeoutSCTP: system.SCTPTimeouts{
			Closed:      10,
			CookieWait:  3,
			Established: 210,
		},
		TimeoutDCCP: system.DCCPTimeouts{
			Request: 240,
			Open:    43200,
		},
	}
	keys := cc.ConntrackSysctlKeys()

	assert.Equal(t, "600", keys["net.netfilter.nf_conntrack_generic_timeout"])
	assert.Equal(t, "432000", keys["net.netfilter.nf_conntrack_tcp_timeout_established"])
	assert.Equal(t, "120", keys["net.netfilter.nf_conntrack_tcp_timeout_syn_sent"])
	assert.Equal(t, "60", keys["net.netfilter.nf_conntrack_tcp_timeout_syn_recv"])
	assert.Equal(t, "120", keys["net.netfilter.nf_conntrack_tcp_timeout_fin_wait"])
	assert.Equal(t, "60", keys["net.netfilter.nf_conntrack_tcp_timeout_close_wait"])
	assert.Equal(t, "30", keys["net.netfilter.nf_conntrack_tcp_timeout_last_ack"])
	assert.Equal(t, "120", keys["net.netfilter.nf_conntrack_tcp_timeout_time_wait"])
	assert.Equal(t, "10", keys["net.netfilter.nf_conntrack_tcp_timeout_close"])
	assert.Equal(t, "30", keys["net.netfilter.nf_conntrack_udp_timeout"])
	assert.Equal(t, "120", keys["net.netfilter.nf_conntrack_udp_timeout_stream"])
	assert.Equal(t, "30", keys["net.netfilter.nf_conntrack_icmp_timeout"])
	assert.Equal(t, "30", keys["net.netfilter.nf_conntrack_icmpv6_timeout"])
	assert.Equal(t, "30", keys["net.netfilter.nf_conntrack_gre_timeout"])
	assert.Equal(t, "180", keys["net.netfilter.nf_conntrack_gre_timeout_stream"])
	assert.Equal(t, "10", keys["net.netfilter.nf_conntrack_sctp_timeout_closed"])
	assert.Equal(t, "3", keys["net.netfilter.nf_conntrack_sctp_timeout_cookie_wait"])
	assert.Equal(t, "210", keys["net.netfilter.nf_conntrack_sctp_timeout_established"])
	assert.Equal(t, "240", keys["net.netfilter.nf_conntrack_dccp_timeout_request"])
	assert.Equal(t, "43200", keys["net.netfilter.nf_conntrack_dccp_timeout_open"])
}

func TestConntrackSysctlMapping_TCPBehavior(t *testing.T) {
	beLiberal := true
	loose := false
	ignoreRST := false
	cc := system.ConntrackConfig{
		TCPBehavior: system.TCPBehavior{
			BeLiberal:        &beLiberal,
			Loose:            &loose,
			MaxRetrans:       3,
			IgnoreInvalidRST: &ignoreRST,
		},
	}
	keys := cc.ConntrackSysctlKeys()
	assert.Equal(t, "1", keys["net.netfilter.nf_conntrack_tcp_be_liberal"])
	assert.Equal(t, "0", keys["net.netfilter.nf_conntrack_tcp_loose"])
	assert.Equal(t, "3", keys["net.netfilter.nf_conntrack_tcp_max_retrans"])
	assert.Equal(t, "0", keys["net.netfilter.nf_conntrack_tcp_ignore_invalid_rst"])
}

func TestConntrackSysctlMapping_Empty(t *testing.T) {
	cc := system.ConntrackConfig{}
	keys := cc.ConntrackSysctlKeys()
	assert.Empty(t, keys)
}

func TestConntrackManagedKeys(t *testing.T) {
	managed := system.ConntrackManagedKeys()
	assert.Equal(t, "table-size", managed["net.netfilter.nf_conntrack_max"])
	assert.Equal(t, "hash-size", managed["net.netfilter.nf_conntrack_buckets"])
	assert.Equal(t, "timeout tcp established", managed["net.netfilter.nf_conntrack_tcp_timeout_established"])
	assert.Equal(t, "tcp be-liberal", managed["net.netfilter.nf_conntrack_tcp_be_liberal"])
	assert.NotEmpty(t, managed)
}

func TestConntrackHasConfig(t *testing.T) {
	assert.False(t, (&system.ConntrackConfig{}).HasConfig())
	assert.True(t, (&system.ConntrackConfig{Modules: []string{"ftp"}}).HasConfig())
	assert.True(t, (&system.ConntrackConfig{TableSize: 262144}).HasConfig())
	assert.True(t, (&system.ConntrackConfig{Accounting: true}).HasConfig())
	assert.True(t, (&system.ConntrackConfig{TimeoutGeneric: 600}).HasConfig())
}

func TestAllConntrackModules(t *testing.T) {
	modules := system.AllConntrackModules()
	assert.Len(t, modules, 12)
	for _, m := range modules {
		assert.True(t, system.ValidConntrackModule(m))
	}
}

package iface

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// raUnitJSON wraps a router-advertisement container body in the interface tree
// so each test states only the part it is about.
func raUnitJSON(body string) string {
	return `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv6": {
								"router-advertisement": ` + body + `
							}
						}
					}
				}
			}
		}
	}`
}

func mustParseRA(t *testing.T, body string) *raUnitConfig {
	t.Helper()
	cfg := mustParseIfaceJSON(t, raUnitJSON(body))
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.IPv6, "ipv6 settings")
	require.NotNil(t, u.IPv6.RouterAdvertisement, "router-advertisement settings")
	return u.IPv6.RouterAdvertisement
}

func mustRejectRA(t *testing.T, body string) string {
	t.Helper()
	_, err := parseIfaceConfig(raUnitJSON(body))
	require.Error(t, err, "config should be rejected")
	return err.Error()
}

// VALIDATES: the router-advertisement container parses into raUnitConfig, and
// every leaf the operator leaves out takes the default its YANG leaf declares.
// PREVENTS: a leaf accepted by the schema, delivered, and then ignored by the
// parser, which advertises zeros on the wire instead of the documented default.
func TestRAConfigParse(t *testing.T) {
	t.Run("defaults when only enabled is set", func(t *testing.T) {
		ra := mustParseRA(t, `{"enabled": "true"}`)
		assert.True(t, ra.Enabled)
		assert.Equal(t, uint16(raDefaultMaximumInterval), ra.MaximumInterval)
		assert.Equal(t, uint16(raDefaultMinimumInterval), ra.MinimumInterval)
		assert.Equal(t, uint16(raDefaultRouterLifetime), ra.RouterLifetime)
		assert.Equal(t, uint8(raDefaultHopLimit), ra.HopLimit)
		assert.False(t, ra.Managed)
		assert.False(t, ra.OtherConfig)
		assert.Zero(t, ra.ReachableTime)
		assert.Zero(t, ra.RetransmitTimer)
		assert.Empty(t, ra.Prefixes)
		assert.Empty(t, ra.RDNSS)
		assert.Nil(t, ra.RDNSSLifetime, "unset rdnss lifetime stays unset")
	})

	t.Run("absent container leaves the field nil", func(t *testing.T) {
		cfg := mustParseIfaceJSON(t, `{
			"interface": {"ethernet": {"eth0": {"unit": {"0": {
				"ipv6": {"dhcpv6": {"enabled": "true"}}
			}}}}}
		}`)
		u := cfg.Ethernet[0].Units[0]
		require.NotNil(t, u.IPv6)
		assert.Nil(t, u.IPv6.RouterAdvertisement)
	})

	t.Run("every leaf carried through", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"maximum-interval": "900",
			"minimum-interval": "300",
			"router-lifetime": "2700",
			"hop-limit": "32",
			"managed": "true",
			"other-config": "true",
			"reachable-time": "30000",
			"retransmit-timer": "1000"
		}`)
		assert.True(t, ra.Enabled)
		assert.Equal(t, uint16(900), ra.MaximumInterval)
		assert.Equal(t, uint16(300), ra.MinimumInterval)
		assert.Equal(t, uint16(2700), ra.RouterLifetime)
		assert.Equal(t, uint8(32), ra.HopLimit)
		assert.True(t, ra.Managed)
		assert.True(t, ra.OtherConfig)
		assert.Equal(t, uint32(30000), ra.ReachableTime)
		assert.Equal(t, uint32(1000), ra.RetransmitTimer)
	})

	t.Run("prefix list with defaults", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"prefix": {"2001:db8:1::/64": {}}
		}`)
		require.Len(t, ra.Prefixes, 1)
		p := ra.Prefixes[0]
		assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/64"), p.Prefix)
		assert.True(t, p.OnLink, "on-link defaults to true")
		assert.True(t, p.Autonomous, "autonomous defaults to true")
		assert.Equal(t, uint32(raDefaultValidLifetime), p.ValidLifetime)
		assert.Equal(t, uint32(raDefaultPreferredLifetime), p.PreferredLifetime)
	})

	t.Run("prefix flags can be cleared", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"prefix": {"2001:db8:1::/64": {
				"on-link": "false",
				"autonomous": "false",
				"valid-lifetime": "7200",
				"preferred-lifetime": "3600"
			}}
		}`)
		require.Len(t, ra.Prefixes, 1)
		p := ra.Prefixes[0]
		assert.False(t, p.OnLink)
		assert.False(t, p.Autonomous)
		assert.Equal(t, uint32(7200), p.ValidLifetime)
		assert.Equal(t, uint32(3600), p.PreferredLifetime)
	})

	t.Run("prefixes are ordered so the wire is reproducible", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"prefix": {
				"2001:db8:3::/64": {},
				"2001:db8:1::/64": {},
				"2001:db8:2::/64": {}
			}
		}`)
		require.Len(t, ra.Prefixes, 3)
		got := []string{
			ra.Prefixes[0].Prefix.String(),
			ra.Prefixes[1].Prefix.String(),
			ra.Prefixes[2].Prefix.String(),
		}
		assert.Equal(t, []string{"2001:db8:1::/64", "2001:db8:2::/64", "2001:db8:3::/64"}, got,
			"config delivers a list as an unordered map, so the parser must order it")
	})

	t.Run("rdnss servers and lifetime", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"rdnss": {
				"server": ["2001:4860:4860::8888", "2001:4860:4860::8844"],
				"lifetime": "1800"
			}
		}`)
		require.Len(t, ra.RDNSS, 2)
		assert.Equal(t, netip.MustParseAddr("2001:4860:4860::8888"), ra.RDNSS[0])
		assert.Equal(t, netip.MustParseAddr("2001:4860:4860::8844"), ra.RDNSS[1])
		require.NotNil(t, ra.RDNSSLifetime)
		assert.Equal(t, uint32(1800), *ra.RDNSSLifetime)
	})
}

// VALIDATES: spec AC-13 and AC-14. A router lifetime of 0 and an RDNSS lifetime
// of 0 are protocol values an operator can set, not invalid input: RFC 4861
// Section 4.2 gives 0 the meaning "not a default router" and RFC 8106
// Section 5.1 gives it "stop using these resolvers".
// PREVENTS: the VyOS T9084 defect, where a 1..7200 constraint on the resolver
// lifetime banned the 0 the product's own help text documented.
func TestRAConfigZeroLifetimesAccepted(t *testing.T) {
	t.Run("router lifetime zero", func(t *testing.T) {
		ra := mustParseRA(t, `{"enabled": "true", "router-lifetime": "0"}`)
		assert.Equal(t, uint16(0), ra.RouterLifetime)
	})

	t.Run("router lifetime zero with any interval", func(t *testing.T) {
		// 0 is exempt from the "at least maximum-interval" rule of RFC 4861
		// Section 6.2.1, which reads "either zero or between MaxRtrAdvInterval
		// and 9000 seconds".
		ra := mustParseRA(t, `{"enabled": "true", "router-lifetime": "0", "maximum-interval": "1800"}`)
		assert.Equal(t, uint16(0), ra.RouterLifetime)
		assert.Equal(t, uint16(1800), ra.MaximumInterval)
	})

	t.Run("rdnss lifetime zero is kept, not treated as unset", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"rdnss": {"server": ["2001:4860:4860::8888"], "lifetime": "0"}
		}`)
		require.NotNil(t, ra.RDNSSLifetime, "an explicit 0 must survive parsing")
		assert.Equal(t, uint32(0), *ra.RDNSSLifetime)
	})

	t.Run("prefix lifetimes of zero", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"prefix": {"2001:db8:1::/64": {"valid-lifetime": "0", "preferred-lifetime": "0"}}
		}`)
		require.Len(t, ra.Prefixes, 1)
		assert.Equal(t, uint32(0), ra.Prefixes[0].ValidLifetime)
		assert.Equal(t, uint32(0), ra.Prefixes[0].PreferredLifetime)
	})
}

// VALIDATES: spec AC-6. Every cross-field rule RFC 4861 states is enforced at
// config verify, with an error naming the leaves in conflict.
// PREVENTS: an advertisement that a conforming receiver rejects, or that
// disrupts a LAN, reaching the wire because only single-leaf ranges were
// checked.
func TestRAConfigCrossFieldReject(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			// RFC 4861 Section 6.2.1: MinRtrAdvInterval must be no greater
			// than 0.75 * MaxRtrAdvInterval.
			name: "minimum interval above three quarters of maximum",
			body: `{"enabled": "true", "maximum-interval": "600", "minimum-interval": "451"}`,
			want: "minimum-interval",
		},
		{
			// RFC 4861 Section 6.2.1: AdvDefaultLifetime must be zero or
			// between MaxRtrAdvInterval and 9000.
			name: "router lifetime below maximum interval",
			body: `{"enabled": "true", "maximum-interval": "600", "router-lifetime": "599"}`,
			want: "router-lifetime",
		},
		{
			// RFC 4861 Section 4.6.2 and RFC 4862 Section 5.5.3.
			name: "preferred lifetime above valid lifetime",
			body: `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "3600", "preferred-lifetime": "3601"}}}`,
			want: "preferred-lifetime",
		},
		{
			name: "maximum interval below the RFC floor",
			body: `{"enabled": "true", "maximum-interval": "3", "minimum-interval": "3"}`,
			want: "maximum-interval",
		},
		{
			name: "maximum interval above the RFC ceiling",
			body: `{"enabled": "true", "maximum-interval": "1801"}`,
			want: "maximum-interval",
		},
		{
			name: "minimum interval below the RFC floor",
			body: `{"enabled": "true", "minimum-interval": "2"}`,
			want: "minimum-interval",
		},
		{
			name: "router lifetime above the RFC ceiling",
			body: `{"enabled": "true", "router-lifetime": "9001"}`,
			want: "router-lifetime",
		},
		{
			name: "reachable time above the RFC ceiling",
			body: `{"enabled": "true", "reachable-time": "3600001"}`,
			want: "reachable-time",
		},
		{
			name: "hop limit above a uint8",
			body: `{"enabled": "true", "hop-limit": "256"}`,
			want: "hop-limit",
		},
		{
			// RFC 4861 Section 4.6.2: a router should not send a prefix option
			// for the link-local prefix.
			name: "link-local prefix",
			body: `{"enabled": "true", "prefix": {"fe80::/64": {}}}`,
			want: "link-local",
		},
		{
			name: "IPv4 prefix",
			body: `{"enabled": "true", "prefix": {"192.0.2.0/24": {}}}`,
			want: "prefix",
		},
		{
			name: "prefix with host bits is not silently masked",
			body: `{"enabled": "true", "prefix": {"2001:db8:1::1/64": {}}}`,
			want: "host bits",
		},
		{
			name: "resolver that is not an IPv6 address",
			body: `{"enabled": "true", "rdnss": {"server": ["192.0.2.1"]}}`,
			want: "server",
		},
		{
			name: "prefix lifetime above a uint32",
			body: `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "4294967296"}}}`,
			want: "valid-lifetime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := mustRejectRA(t, tt.body)
			assert.Contains(t, msg, tt.want, "error should name the leaf in conflict")
		})
	}
}

// VALIDATES: the boundary table in the spec's TDD plan: the last valid value on
// each side of every numeric range is accepted.
// PREVENTS: an off-by-one in a range check rejecting a value the RFC allows,
// which is the failure mode a reject-only test cannot see.
func TestRAConfigBoundariesAccepted(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		assert func(t *testing.T, ra *raUnitConfig)
	}{
		{
			name: "maximum interval at the floor",
			body: `{"enabled": "true", "maximum-interval": "4", "minimum-interval": "3", "router-lifetime": "0"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint16(4), ra.MaximumInterval)
			},
		},
		{
			name: "maximum interval at the ceiling",
			body: `{"enabled": "true", "maximum-interval": "1800", "minimum-interval": "1350", "router-lifetime": "9000"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint16(1800), ra.MaximumInterval)
				assert.Equal(t, uint16(1350), ra.MinimumInterval)
				assert.Equal(t, uint16(9000), ra.RouterLifetime)
			},
		},
		{
			name: "minimum interval exactly three quarters of maximum",
			body: `{"enabled": "true", "maximum-interval": "600", "minimum-interval": "450"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint16(450), ra.MinimumInterval)
			},
		},
		{
			name: "router lifetime exactly at maximum interval",
			body: `{"enabled": "true", "maximum-interval": "600", "router-lifetime": "600"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint16(600), ra.RouterLifetime)
			},
		},
		{
			name: "hop limit at the ceiling",
			body: `{"enabled": "true", "hop-limit": "255"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint8(255), ra.HopLimit)
			},
		},
		{
			name: "hop limit zero means unspecified",
			body: `{"enabled": "true", "hop-limit": "0"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint8(0), ra.HopLimit)
			},
		},
		{
			name: "reachable time at the ceiling",
			body: `{"enabled": "true", "reachable-time": "3600000"}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				assert.Equal(t, uint32(3600000), ra.ReachableTime)
			},
		},
		{
			name: "prefix length 128",
			body: `{"enabled": "true", "prefix": {"2001:db8::1/128": {}}}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				require.Len(t, ra.Prefixes, 1)
				assert.Equal(t, 128, ra.Prefixes[0].Prefix.Bits())
			},
		},
		{
			name: "infinite prefix lifetimes",
			body: `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "4294967295", "preferred-lifetime": "4294967295"}}}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				require.Len(t, ra.Prefixes, 1)
				assert.Equal(t, uint32(4294967295), ra.Prefixes[0].ValidLifetime)
				assert.Equal(t, uint32(4294967295), ra.Prefixes[0].PreferredLifetime)
			},
		},
		{
			name: "preferred lifetime equal to valid lifetime",
			body: `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "3600", "preferred-lifetime": "3600"}}}`,
			assert: func(t *testing.T, ra *raUnitConfig) {
				require.Len(t, ra.Prefixes, 1)
				assert.Equal(t, uint32(3600), ra.Prefixes[0].PreferredLifetime)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, mustParseRA(t, tt.body))
		})
	}
}

// VALIDATES: the RA config turns into the ndp encoder input, with the RDNSS
// lifetime RFC 8106 Section 5.1 recommends when the operator sets none.
// PREVENTS: the sender inventing its own defaults, which would make the
// advertised values disagree with what config verify accepted.
func TestRASenderConfigFromUnit(t *testing.T) {
	t.Run("unset rdnss lifetime becomes three times the maximum interval", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"maximum-interval": "600",
			"rdnss": {"server": ["2001:4860:4860::8888"]}
		}`)
		assert.Equal(t, uint32(1800), ra.EffectiveRDNSSLifetime())
	})

	t.Run("explicit zero rdnss lifetime is honored", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"maximum-interval": "600",
			"rdnss": {"server": ["2001:4860:4860::8888"], "lifetime": "0"}
		}`)
		assert.Equal(t, uint32(0), ra.EffectiveRDNSSLifetime())
	})

	t.Run("three times a large maximum interval does not overflow", func(t *testing.T) {
		ra := mustParseRA(t, `{
			"enabled": "true",
			"maximum-interval": "1800",
			"router-lifetime": "9000",
			"minimum-interval": "1350",
			"rdnss": {"server": ["2001:4860:4860::8888"]}
		}`)
		assert.Equal(t, uint32(5400), ra.EffectiveRDNSSLifetime())
	})
}

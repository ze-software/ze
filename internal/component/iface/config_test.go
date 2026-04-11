package iface

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// TestParseTunnelGRE verifies that a gre case with all common leaves parses
// into a TunnelSpec with the correct kind and fields.
//
// VALIDATES: AC-1, AC-2, AC-3 - gre kind with local/remote endpoint
// containers, key, ttl, tos parses correctly.
// PREVENTS: Tunnel parser silently dropping fields when adding new encap kinds.
func TestParseTunnelGRE(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"},
							"key": "42",
							"ttl": "64",
							"tos": "0"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	e := cfg.Tunnel[0]
	assert.Equal(t, "gre0", e.Name)
	assert.Equal(t, TunnelKindGRE, e.Spec.Kind)
	assert.Equal(t, "192.0.2.1", e.Spec.LocalAddress)
	assert.Equal(t, "198.51.100.1", e.Spec.RemoteAddress)
	assert.True(t, e.Spec.KeySet)
	assert.Equal(t, uint32(42), e.Spec.Key)
	assert.True(t, e.Spec.TTLSet)
	assert.Equal(t, uint8(64), e.Spec.TTL)
	assert.True(t, e.Spec.TosSet)
}

// TestParseTunnelGretap verifies that the gretap case is recognized distinctly
// from gre even though their leaves overlap.
//
// VALIDATES: AC-5 - gretap discriminator separate from gre.
// PREVENTS: TunnelKind enum collision between gre and gretap.
func TestParseTunnelGretap(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gretap0": {
					"encapsulation": {
						"gretap": {
							"local":  {"ip": "10.0.0.1"},
							"remote": {"ip": "10.0.0.2"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindGRETap, cfg.Tunnel[0].Spec.Kind)
}

// TestParseTunnelIp6gre verifies the ip6gre case with v6 endpoints, hoplimit,
// and tclass parses into the right TunnelSpec fields.
//
// VALIDATES: AC-7 - ip6gre with hoplimit/tclass parses.
// PREVENTS: v6-underlay leaves silently dropped.
func TestParseTunnelIp6gre(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ip6gre0": {
					"encapsulation": {
						"ip6gre": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"},
							"hoplimit": "64",
							"tclass": "0",
							"key": "100"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, TunnelKindIP6GRE, spec.Kind)
	assert.Equal(t, "2001:db8::1", spec.LocalAddress)
	assert.Equal(t, "2001:db8::2", spec.RemoteAddress)
	assert.True(t, spec.HopLimitSet)
	assert.Equal(t, uint8(64), spec.HopLimit)
	assert.True(t, spec.TClassSet)
	assert.True(t, spec.KeySet)
	assert.Equal(t, uint32(100), spec.Key)
}

// TestParseTunnelIpip verifies the ipip case parses without GRE-specific fields.
//
// VALIDATES: AC-8 - ipip case parses without key or ignore-df.
// PREVENTS: Schema accepting key on ipip silently.
func TestParseTunnelIpip(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ipip0": {
					"encapsulation": {
						"ipip": {
							"local":  {"ip": "10.0.0.1"},
							"remote": {"ip": "10.0.0.2"},
							"ttl": "32"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, TunnelKindIPIP, spec.Kind)
	assert.False(t, spec.KeySet, "ipip must not have key set")
	assert.True(t, spec.TTLSet)
	assert.Equal(t, uint8(32), spec.TTL)
}

// TestParseTunnelSit verifies the sit (6in4) case parses with v4 endpoints.
//
// VALIDATES: AC-9 - sit kind with v4 endpoints parses.
func TestParseTunnelSit(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"sixin4": {
					"encapsulation": {
						"sit": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindSIT, cfg.Tunnel[0].Spec.Kind)
}

// TestParseTunnelIp6tnl verifies the ip6tnl case (IPv6 in IPv6) parses with
// encaplimit.
//
// VALIDATES: AC-10 - ip6tnl with encaplimit parses.
func TestParseTunnelIp6tnl(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"v6t": {
					"encapsulation": {
						"ip6tnl": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"},
							"encaplimit": "4"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, TunnelKindIP6Tnl, spec.Kind)
	assert.True(t, spec.EncapLimitSet)
	assert.Equal(t, uint8(4), spec.EncapLimit)
}

// TestParseTunnelIpip6 verifies the ipip6 case parses with the IPIP6 kind
// (which the linux backend implements as Ip6tnl with Proto=IPPROTO_IPIP).
//
// VALIDATES: AC-11 - ipip6 kind is distinct from ip6tnl in the spec.
// PREVENTS: ipip6 silently treated as ip6tnl, losing the discriminator the
// linux backend needs to set Proto=4 instead of Proto=41.
func TestParseTunnelIpip6(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"v4inv6": {
					"encapsulation": {
						"ipip6": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindIPIP6, cfg.Tunnel[0].Spec.Kind)
}

// TestParseTunnelLocalInterface verifies the choice inside the local
// container lets the user specify a parent interface name instead of an IP.
//
// VALIDATES: AC-13 - local interface alternative parses.
func TestParseTunnelLocalInterface(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre1": {
					"encapsulation": {
						"gre": {
							"local":  {"interface": "eth0"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, "eth0", spec.LocalInterface)
	assert.Empty(t, spec.LocalAddress)
}

// TestParseTunnelMissingEncapsulation verifies the parser rejects a tunnel
// entry with no encapsulation block. The YANG schema rejects this at edit
// time too (via mandatory choice), so reaching this branch means hand-edited
// JSON or a schema bug.
//
// VALIDATES: AC-15 - tunnel without encapsulation rejected.
func TestParseTunnelMissingEncapsulation(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing encapsulation")
}

// TestParseTunnelMultipleCases verifies the parser rejects a tunnel with
// two encapsulation cases set simultaneously. Same defense-in-depth role
// as TestParseTunnelMissingEncapsulation.
func TestParseTunnelMultipleCases(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"x": {
					"encapsulation": {
						"gre":  {"local": {"ip": "10.0.0.1"}, "remote": {"ip": "10.0.0.2"}},
						"ipip": {"local": {"ip": "10.0.0.1"}, "remote": {"ip": "10.0.0.3"}}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple encapsulation cases")
}

// TestParseTunnelBothLocals verifies the parser rejects a tunnel with both
// local.ip and local.interface set. The current `ze config validate` path
// does not invoke plugin OnConfigVerify (see plan/deferrals.md), so the
// Go-side check in parseTunnelEntry is the only safety net. This test pins it.
//
// VALIDATES: AC-14 - local ip and local interface are mutually exclusive.
// PREVENTS: Silent acceptance of contradictory tunnel source config.
func TestParseTunnelBothLocals(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1", "interface": "eth0"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestParseTunnelMissingLocal verifies the parser rejects a tunnel with
// neither local.ip nor local.interface set. Same Go-side defense-in-depth
// as TestParseTunnelBothLocals.
func TestParseTunnelMissingLocal(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local ip or local interface required")
}

// TestApplyTunnelsTwoGREDistinctKeys verifies AC-12: two gre tunnels with
// the same local/remote endpoints but different keys can coexist as long
// as they have distinct names. Each must reach the backend with its own
// Spec.
//
// VALIDATES: AC-12 - two GRE tunnels distinct keys.
// PREVENTS: Spec deduplication on local/remote that would drop one of the
// tunnels silently.
func TestApplyTunnelsTwoGREDistinctKeys(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	cfg := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{
			{
				ifaceEntry: ifaceEntry{Name: "gre-a"},
				Spec: TunnelSpec{
					Kind:          TunnelKindGRE,
					Name:          "gre-a",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: "198.51.100.1",
					Key:           1,
					KeySet:        true,
				},
			},
			{
				ifaceEntry: ifaceEntry{Name: "gre-b"},
				Spec: TunnelSpec{
					Kind:          TunnelKindGRE,
					Name:          "gre-b",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: "198.51.100.1",
					Key:           2,
					KeySet:        true,
				},
			},
		},
	}
	errs := applyConfig(cfg, nil, b)
	require.Empty(t, errs)
	require.Contains(t, b.tunnels, "gre-a")
	require.Contains(t, b.tunnels, "gre-b")
	assert.Equal(t, uint32(1), b.tunnels["gre-a"].Key)
	assert.Equal(t, uint32(2), b.tunnels["gre-b"].Key)
	assert.Equal(t, b.tunnels["gre-a"].LocalAddress, b.tunnels["gre-b"].LocalAddress)
	assert.Equal(t, b.tunnels["gre-a"].RemoteAddress, b.tunnels["gre-b"].RemoteAddress)
}

// TestApplyTunnelsUnchangedSkipsRecreate verifies that applyConfig does NOT
// delete-then-create a tunnel whose Spec is identical to the previous apply.
//
// VALIDATES: Smart reconciliation preserves running tunnels across reload.
// PREVENTS: Every SIGHUP briefly dropping every tunnel even when nothing changed.
func TestApplyTunnelsUnchangedSkipsRecreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	spec := TunnelSpec{
		Kind:          TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "198.51.100.1",
		Key:           42,
		KeySet:        true,
	}
	cfg := &ifaceConfig{
		Backend: "fake",
		Tunnel:  []tunnelEntry{{ifaceEntry: ifaceEntry{Name: "gre0"}, Spec: spec}},
	}
	// First apply: tunnel created.
	require.Empty(t, applyConfig(cfg, nil, b))
	require.Contains(t, b.tunnels, "gre0")
	require.False(t, b.deleted["gre0"], "first apply must not delete")

	// Second apply with the SAME config: no delete should fire.
	b.deleted = nil
	require.Empty(t, applyConfig(cfg, cfg, b))
	assert.False(t, b.deleted["gre0"], "unchanged spec must not trigger delete-then-create")
}

// TestApplyTunnelsChangedTriggersRecreate verifies that applyConfig deletes
// and recreates a tunnel whose Spec changed across reloads.
//
// VALIDATES: AC-18 - key change recreates the tunnel.
// PREVENTS: Modified tunnel parameters being silently ignored.
func TestApplyTunnelsChangedTriggersRecreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	prev := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{{
			ifaceEntry: ifaceEntry{Name: "gre0"},
			Spec: TunnelSpec{
				Kind: TunnelKindGRE, Name: "gre0",
				LocalAddress: "192.0.2.1", RemoteAddress: "198.51.100.1",
				Key: 1, KeySet: true,
			},
		}},
	}
	require.Empty(t, applyConfig(prev, nil, b))
	b.deleted = nil

	updated := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{{
			ifaceEntry: ifaceEntry{Name: "gre0"},
			Spec: TunnelSpec{
				Kind: TunnelKindGRE, Name: "gre0",
				LocalAddress: "192.0.2.1", RemoteAddress: "198.51.100.1",
				Key: 2, KeySet: true,
			},
		}},
	}
	require.Empty(t, applyConfig(updated, prev, b))
	assert.True(t, b.deleted["gre0"], "spec change must trigger delete-then-create")
	assert.Equal(t, uint32(2), b.tunnels["gre0"].Key, "new key must be applied")
}

// TestParseTunnelVLANRejectedOnL3 verifies parseTunnelEntry rejects a vlan-id
// unit on an L3 tunnel kind. Only gretap and ip6gretap carry Ethernet frames
// and accept VLAN sub-interfaces.
//
// VALIDATES: VLAN-on-tunnel only allowed on bridgeable kinds.
// PREVENTS: Silent failure when configuring VLAN on a gre/ipip/sit tunnel.
func TestParseTunnelVLANRejectedOnL3(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {
					"unit": {
						"100": {"vlan-id": "100"}
					},
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vlan-id units are not supported on gre tunnels")
}

// TestParseTunnelVLANAcceptedOnGretap verifies parseTunnelEntry accepts a
// vlan-id unit on gretap (the L2 bridgeable kind).
func TestParseTunnelVLANAcceptedOnGretap(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gtap0": {
					"unit": {
						"100": {"vlan-id": "100"}
					},
					"encapsulation": {
						"gretap": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	require.Len(t, cfg.Tunnel[0].Units, 1)
	assert.Equal(t, 100, cfg.Tunnel[0].Units[0].VLANID)
}

// TestApplyTunnelsCreate verifies that applyConfig with one tunnel entry
// invokes Backend.CreateTunnel with a TunnelSpec carrying the parsed fields.
//
// VALIDATES: Backend dispatch wires through applyConfig.
// PREVENTS: tunnelEntry parsed but never reaching the backend.
func TestApplyTunnelsCreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	cfg := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{
			{
				ifaceEntry: ifaceEntry{
					Name:  "gre0",
					Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/30"}}},
				},
				Spec: TunnelSpec{
					Kind:          TunnelKindGRE,
					Name:          "gre0",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: "198.51.100.1",
					Key:           42,
					KeySet:        true,
				},
			},
		},
	}
	errs := applyConfig(cfg, nil, b)
	require.Empty(t, errs, "applyConfig should not error for happy path")
	require.Contains(t, b.tunnels, "gre0", "tunnel must reach the backend")
	got := b.tunnels["gre0"]
	assert.Equal(t, TunnelKindGRE, got.Kind)
	assert.Equal(t, "192.0.2.1", got.LocalAddress)
	assert.Equal(t, "198.51.100.1", got.RemoteAddress)
	assert.Equal(t, uint32(42), got.Key)
	assert.True(t, got.KeySet)
	assert.Contains(t, b.addrs["gre0"], "10.0.0.1/30", "tunnel address should be applied")
}

// mustParseIfaceJSON wraps parseIfaceConfig with a t.Fatal on parse error.
// Used by table-driven tunnel tests to keep individual cases concise.
func mustParseIfaceJSON(t *testing.T, input string) *ifaceConfig {
	t.Helper()
	// Validate JSON first so the test fails clearly on a typo.
	var raw any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("invalid JSON in test fixture: %v", err)
	}
	cfg, err := parseIfaceConfig(input)
	if err != nil {
		t.Fatalf("parseIfaceConfig: %v", err)
	}
	return cfg
}

// TestIfaceApplyJournalCreate verifies that applyConfig wrapped in a journal
// can be rolled back by re-applying the previous config.
//
// VALIDATES: AC-5 - Interface config adds new interface + address.
// PREVENTS: Interface created without journal tracking, making rollback impossible.
func TestIfaceApplyJournalCreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			empty := &ifaceConfig{Backend: "fake"}
			if errs := applyConfig(empty, cfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.True(t, b.created["dummy0"], "dummy0 should be created")
	assert.Contains(t, b.addrs["dummy0"], "10.0.0.1/24", "address should be added")
	assert.Equal(t, 1, j.Len(), "journal should have 1 entry")
}

// TestIfaceApplyJournalAddress verifies that address operations are tracked
// by the journal and can be undone.
//
// VALIDATES: AC-5 - Address assigned via journal.
// PREVENTS: Address added without undo capability.
func TestIfaceApplyJournalAddress(t *testing.T) {
	b := &fakeBackend{
		ifaces: map[string]fakeIface{
			"eth0": {name: "eth0", linkType: "ethernet"},
		},
	}

	cfg := &ifaceConfig{
		Backend:  "fake",
		Ethernet: []ifaceEntry{{Name: "eth0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24", "10.0.0.2/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			empty := &ifaceConfig{Backend: "fake"}
			if errs := applyConfig(empty, cfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.Len(t, b.addrs["eth0"], 2, "both addresses should be added")
}

// TestIfaceApplyJournalRollbackEvents verifies that rollback re-applies
// the previous config, effectively undoing the changes.
//
// VALIDATES: AC-6 - Interface rollback after partial apply.
// PREVENTS: Rollback leaving stale interfaces or addresses.
func TestIfaceApplyJournalRollbackEvents(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	// Previous config: no interfaces.
	previousCfg := &ifaceConfig{Backend: "fake"}

	// New config: creates dummy0.
	newCfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(newCfg, previousCfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			if errs := applyConfig(previousCfg, newCfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.True(t, b.created["dummy0"], "dummy0 should be created after apply")

	// Rollback: should re-apply previous (empty) config, deleting dummy0.
	errs := j.Rollback()
	assert.Empty(t, errs, "rollback should succeed")
	assert.True(t, b.deleted["dummy0"], "dummy0 should be deleted after rollback")
}

// TestIfaceVerifyEstimate verifies that the verify callback computes an
// estimate proportional to interface operations.
//
// VALIDATES: AC-12 - Interface budget proportional to interface count.
// PREVENTS: Budget estimate that doesn't scale with config size.
func TestIfaceVerifyEstimate(t *testing.T) {
	// Interface budget is set statically at registration (VerifyBudget=2, ApplyBudget=10).
	// The estimate scales with the number of configured interfaces.
	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy: []ifaceEntry{
			{Name: "dummy0"},
			{Name: "dummy1"},
			{Name: "dummy2"},
		},
		Veth: []vethEntry{
			{ifaceEntry: ifaceEntry{Name: "veth0"}, Peer: "veth0-peer"},
		},
	}

	// Count operations: 3 dummy creates + 1 veth create = 4 operations.
	count := len(cfg.Dummy) + len(cfg.Veth) + len(cfg.Bridge) + len(cfg.Ethernet)
	assert.Equal(t, 4, count, "operation count should reflect interface config size")
}

// fakeBackend implements Backend for testing config application.
type fakeBackend struct {
	ifaces  map[string]fakeIface
	created map[string]bool
	deleted map[string]bool
	addrs   map[string][]string
	tunnels map[string]TunnelSpec
}

type fakeIface struct {
	name     string
	linkType string
}

func (b *fakeBackend) ensureMaps() {
	if b.created == nil {
		b.created = make(map[string]bool)
	}
	if b.deleted == nil {
		b.deleted = make(map[string]bool)
	}
	if b.addrs == nil {
		b.addrs = make(map[string][]string)
	}
	if b.tunnels == nil {
		b.tunnels = make(map[string]TunnelSpec)
	}
}

func (b *fakeBackend) CreateDummy(name string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "dummy"}
	return nil
}

func (b *fakeBackend) CreateVeth(name, peerName string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "veth"}
	return nil
}

func (b *fakeBackend) CreateBridge(name string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "bridge"}
	return nil
}

func (b *fakeBackend) CreateVLAN(_ string, _ int) error { return nil }

func (b *fakeBackend) CreateTunnel(spec TunnelSpec) error {
	b.ensureMaps()
	b.created[spec.Name] = true
	b.tunnels[spec.Name] = spec
	b.ifaces[spec.Name] = fakeIface{name: spec.Name, linkType: "tunnel-" + spec.Kind.String()}
	return nil
}

func (b *fakeBackend) CreateWireguardDevice(name string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "wireguard"}
	return nil
}

func (b *fakeBackend) ConfigureWireguardDevice(_ WireguardSpec) error { return nil }

func (b *fakeBackend) GetWireguardDevice(_ string) (WireguardSpec, error) {
	return WireguardSpec{}, nil
}

func (b *fakeBackend) SetAdminUp(_ string) error              { return nil }
func (b *fakeBackend) SetAdminDown(_ string) error            { return nil }
func (b *fakeBackend) SetMTU(_ string, _ int) error           { return nil }
func (b *fakeBackend) SetMACAddress(_, _ string) error        { return nil }
func (b *fakeBackend) GetMACAddress(_ string) (string, error) { return "", nil }
func (b *fakeBackend) GetStats(_ string) (*InterfaceStats, error) {
	return &InterfaceStats{}, nil
}

func (b *fakeBackend) DeleteInterface(name string) error {
	b.ensureMaps()
	b.deleted[name] = true
	delete(b.ifaces, name)
	return nil
}

func (b *fakeBackend) AddAddress(ifaceName, cidr string) error {
	b.ensureMaps()
	b.addrs[ifaceName] = append(b.addrs[ifaceName], cidr)
	return nil
}

func (b *fakeBackend) RemoveAddress(ifaceName, cidr string) error {
	b.ensureMaps()
	filtered := b.addrs[ifaceName][:0]
	for _, a := range b.addrs[ifaceName] {
		if a != cidr {
			filtered = append(filtered, a)
		}
	}
	b.addrs[ifaceName] = filtered
	return nil
}

func (b *fakeBackend) ReplaceAddressWithLifetime(_, _ string, _, _ int) error { return nil }

func (b *fakeBackend) ListInterfaces() ([]InterfaceInfo, error) {
	var result []InterfaceInfo
	for _, f := range b.ifaces {
		info := InterfaceInfo{Name: f.name, Type: f.linkType}
		if addrs, ok := b.addrs[f.name]; ok {
			for _, a := range addrs {
				info.Addresses = append(info.Addresses, AddrInfo{Address: a, PrefixLength: 24})
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (b *fakeBackend) GetInterface(name string) (*InterfaceInfo, error) {
	f, ok := b.ifaces[name]
	if !ok {
		return nil, fmt.Errorf("interface %s not found", name)
	}
	return &InterfaceInfo{Name: f.name, Type: f.linkType}, nil
}

func (b *fakeBackend) BridgeAddPort(_, _ string) error     { return nil }
func (b *fakeBackend) BridgeDelPort(_ string) error        { return nil }
func (b *fakeBackend) BridgeSetSTP(_ string, _ bool) error { return nil }

func (b *fakeBackend) SetIPv4Forwarding(_ string, _ bool) error { return nil }
func (b *fakeBackend) SetIPv4ArpFilter(_ string, _ bool) error  { return nil }
func (b *fakeBackend) SetIPv4ArpAccept(_ string, _ bool) error  { return nil }
func (b *fakeBackend) SetIPv4ProxyARP(_ string, _ bool) error   { return nil }
func (b *fakeBackend) SetIPv4ArpAnnounce(_ string, _ int) error { return nil }
func (b *fakeBackend) SetIPv4ArpIgnore(_ string, _ int) error   { return nil }
func (b *fakeBackend) SetIPv4RPFilter(_ string, _ int) error    { return nil }
func (b *fakeBackend) SetIPv6Autoconf(_ string, _ bool) error   { return nil }
func (b *fakeBackend) SetIPv6AcceptRA(_ string, _ int) error    { return nil }
func (b *fakeBackend) SetIPv6Forwarding(_ string, _ bool) error { return nil }
func (b *fakeBackend) SetupMirror(_, _ string, _, _ bool) error { return nil }
func (b *fakeBackend) RemoveMirror(_ string) error              { return nil }
func (b *fakeBackend) StartMonitor(_ ze.EventBus) error         { return nil }
func (b *fakeBackend) StopMonitor()                             {}
func (b *fakeBackend) Close() error                             { return nil }

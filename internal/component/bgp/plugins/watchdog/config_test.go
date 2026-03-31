package watchdog

import (
	"testing"
)

// VALIDATES: OnConfigure extracts watchdog routes from JSON config tree
// PREVENTS: Config delivery produces empty or wrong route store

func TestParseConfigBasic(t *testing.T) {
	// Mimics the JSON produced by ResolveBGPTree + ToMap for a peer with one watchdog update block.
	// Peers are keyed by name with remote.ip inside.
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "127.0.0.1", "as": "65533"}},
				"update": {
					"default": {
						"attribute": {
							"origin": "igp",
							"next-hop": "1.2.3.4",
							"local-preference": "100"
						},
						"nlri": {
							"ipv4/unicast": {
								"content": "add 77.77.77.77/32"
							}
						},
						"watchdog": {
							"name": "dnsr",
							"withdraw": "true"
						}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	// Pool key is the remote IP, not the peer name.
	pools, ok := peerPools["127.0.0.1"]
	if !ok {
		t.Fatal("no pools for 127.0.0.1")
	}

	pool := pools.GetPool("dnsr")
	if pool == nil {
		t.Fatal("no pool named dnsr")
		return
	}

	routes := pool.Routes()
	if len(routes) != 1 {
		t.Fatalf("route count = %d, want 1", len(routes))
	}

	entry := routes[0]
	if entry.Key != "77.77.77.77/32#0" {
		t.Errorf("Key = %q, want 77.77.77.77/32#0", entry.Key)
	}

	// Verify announce command contains expected attributes (long-form keywords via shared.FormatAnnounceCommand)
	wantAnnounce := "update text origin igp local-preference 100 nhop 1.2.3.4 nlri ipv4/unicast add 77.77.77.77/32"
	if entry.AnnounceCmd != wantAnnounce {
		t.Errorf("AnnounceCmd:\n  got  %q\n  want %q", entry.AnnounceCmd, wantAnnounce)
	}

	wantWithdraw := "update text nlri ipv4/unicast del 77.77.77.77/32"
	if entry.WithdrawCmd != wantWithdraw {
		t.Errorf("WithdrawCmd:\n  got  %q\n  want %q", entry.WithdrawCmd, wantWithdraw)
	}
}

// VALIDATES: Routes with withdraw flag start as not-initially-announced
// PREVENTS: Withdrawn routes sent prematurely on session up

func TestParseConfigWithdrawFlag(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "10.0.0.1"},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}},
						"watchdog": {"name": "dns", "withdraw": "true"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pools := peerPools["10.0.0.1"]
	pool := pools.GetPool("dns")
	routes := pool.Routes()

	// Route should exist but NOT be announced for any peer
	if routes[0].announced["10.0.0.1"] {
		t.Error("withdraw=true route should not be initially announced")
	}
}

// VALIDATES: Multiple peers with different watchdog groups
// PREVENTS: Cross-peer route contamination

func TestParseConfigMultiplePeers(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "10.0.0.1"},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}},
						"watchdog": {"name": "dns"}
					}
				}
			},
			"peer2": {
				"connection": {"remote": {"ip": "10.0.0.2"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "10.0.0.2"},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.1.0/24"}},
						"watchdog": {"name": "web"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	if len(peerPools) != 2 {
		t.Fatalf("peer count = %d, want 2", len(peerPools))
	}

	// Pools keyed by remote IP
	if peerPools["10.0.0.1"].GetPool("dns") == nil {
		t.Error("10.0.0.1 missing dns pool")
	}
	if peerPools["10.0.0.1"].GetPool("web") != nil {
		t.Error("10.0.0.1 should not have web pool")
	}

	if peerPools["10.0.0.2"].GetPool("web") == nil {
		t.Error("10.0.0.2 missing web pool")
	}
}

// VALIDATES: Update blocks without watchdog are skipped
// PREVENTS: Non-watchdog routes captured by plugin

func TestParseConfigSkipsNonWatchdog(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "10.0.0.1"},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}}
					},
					"default#1": {
						"attribute": {"origin": "igp", "next-hop": "10.0.0.1"},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.1.0/24"}},
						"watchdog": {"name": "dns"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pools := peerPools["10.0.0.1"]
	if pools == nil {
		t.Fatal("no pools for 10.0.0.1")
		return
	}

	pool := pools.GetPool("dns")
	if pool == nil {
		t.Fatal("missing dns pool")
		return
	}

	routes := pool.Routes()
	if len(routes) != 1 {
		t.Errorf("route count = %d, want 1 (only watchdog route)", len(routes))
	}
}

// VALIDATES: Multiple NLRI prefixes in same update block
// PREVENTS: Only first prefix captured, rest lost

func TestParseConfigMultiplePrefixes(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "1.2.3.4"},
						"nlri": {
							"ipv4/unicast": {"content": "add 10.0.0.0/24 10.0.1.0/24"}
						},
						"watchdog": {"name": "dns"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pool := peerPools["10.0.0.1"].GetPool("dns")
	routes := pool.Routes()
	if len(routes) != 2 {
		t.Fatalf("route count = %d, want 2", len(routes))
	}
}

// VALIDATES: Config with nhop self
// PREVENTS: Self next-hop not passed through to text command

func TestParseConfigNhopSelf(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "self"},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}},
						"watchdog": {"name": "dns"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pool := peerPools["10.0.0.1"].GetPool("dns")
	route := pool.Routes()[0]

	if route.AnnounceCmd != "update text origin igp nhop self nlri ipv4/unicast add 10.0.0.0/24" {
		t.Errorf("AnnounceCmd = %q, expected nhop self", route.AnnounceCmd)
	}
}

// VALIDATES: Bare IP addresses are normalized to CIDR notation
// PREVENTS: "invalid prefix: 77.77.77.77" from text command parser (ExaBGP migration produces bare IPs)

func TestParseConfigBareIPNormalized(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "127.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "1.2.3.4"},
						"nlri": {"ipv4/unicast": {"content": "add 77.77.77.77"}},
						"watchdog": {"name": "dnsr", "withdraw": "true"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pool := peerPools["127.0.0.1"].GetPool("dnsr")
	route := pool.Routes()[0]

	// Bare IP must be normalized to /32
	if route.Key != "77.77.77.77/32#0" {
		t.Errorf("Key = %q, want 77.77.77.77/32#0", route.Key)
	}

	wantAnnounce := "update text origin igp nhop 1.2.3.4 nlri ipv4/unicast add 77.77.77.77/32"
	if route.AnnounceCmd != wantAnnounce {
		t.Errorf("AnnounceCmd:\n  got  %q\n  want %q", route.AnnounceCmd, wantAnnounce)
	}

	wantWithdraw := "update text nlri ipv4/unicast del 77.77.77.77/32"
	if route.WithdrawCmd != wantWithdraw {
		t.Errorf("WithdrawCmd:\n  got  %q\n  want %q", route.WithdrawCmd, wantWithdraw)
	}
}

// VALIDATES: Bare IPv6 addresses are normalized to /128
// PREVENTS: IPv6 host routes from ExaBGP migration fail text command parser

func TestParseConfigBareIPv6Normalized(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {"origin": "igp", "next-hop": "::1"},
						"nlri": {"ipv6/unicast": {"content": "add 2001:db8::1"}},
						"watchdog": {"name": "dns"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pool := peerPools["10.0.0.1"].GetPool("dns")
	route := pool.Routes()[0]

	if route.Key != "2001:db8::1/128#0" {
		t.Errorf("Key = %q, want 2001:db8::1/128#0", route.Key)
	}
}

// VALIDATES: Config with all attributes
// PREVENTS: Attribute parsing regression

func TestParseConfigAllAttributes(t *testing.T) {
	jsonData := `{
		"bgp": {
			"peer": {
			"peer1": {
				"connection": {"remote": {"ip": "10.0.0.1"}},
				"update": {
					"default": {
						"attribute": {
							"origin": "igp",
							"next-hop": "10.0.0.1",
							"local-preference": "200",
							"med": "50",
							"as-path": "65001 65002",
							"community": "65000:100 65000:200",
							"large-community": "65000:1:2",
							"extended-community": "target:65000:100"
						},
						"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}},
						"watchdog": {"name": "dns"}
					}
				}
			}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pool := peerPools["10.0.0.1"].GetPool("dns")
	route := pool.Routes()[0]

	want := "update text origin igp as-path [65001 65002] med 50 local-preference 200 community [65000:100 65000:200] large-community [65000:1:2] extended-community [target:65000:100] nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24"
	if route.AnnounceCmd != want {
		t.Errorf("AnnounceCmd:\n  got  %q\n  want %q", route.AnnounceCmd, want)
	}
}

// TestParseConfigGroupAndPeerBothHaveWatchdog verifies both group-level and peer-level
// watchdog routes are collected when both levels have update blocks.
//
// VALIDATES: Group watchdog routes and peer watchdog routes are both present.
// PREVENTS: Peer update block shadowing group watchdog when both levels set.
func TestParseConfigGroupAndPeerBothHaveWatchdog(t *testing.T) {
	// Group has watchdog route 10.0.0.0/24 in pool "health".
	// Peer has its own watchdog route 20.0.0.0/24 in pool "dns".
	// Both should be collected.
	jsonData := `{
		"bgp": {
			"group": {
				"test-group": {
					"update": {
						"default": {
							"attribute": {"origin": "igp", "next-hop": "1.1.1.1"},
							"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}},
							"watchdog": {"name": "health"}
						}
					},
					"peer": {
						"peer1": {
							"connection": {"remote": {"ip": "10.0.0.1"}},
							"update": {
								"default": {
									"attribute": {"origin": "igp", "next-hop": "2.2.2.2"},
									"nlri": {"ipv4/unicast": {"content": "add 20.0.0.0/24"}},
									"watchdog": {"name": "dns"}
								}
							}
						}
					}
				}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pools, ok := peerPools["10.0.0.1"]
	if !ok {
		t.Fatal("no pools for 10.0.0.1")
	}

	// Both pools should be present.
	healthPool := pools.GetPool("health")
	if healthPool == nil {
		t.Fatal("missing 'health' pool from group-level watchdog")
		return
	}
	if len(healthPool.Routes()) != 1 {
		t.Fatalf("health pool: want 1 route, got %d", len(healthPool.Routes()))
	}

	dnsPool := pools.GetPool("dns")
	if dnsPool == nil {
		t.Fatal("missing 'dns' pool from peer-level watchdog")
		return
	}
	if len(dnsPool.Routes()) != 1 {
		t.Fatalf("dns pool: want 1 route, got %d", len(dnsPool.Routes()))
	}
}

// TestParseConfigGroupWatchdogPeerNoUpdate verifies group watchdog is inherited
// when the peer has no update block at all.
//
// VALIDATES: Group watchdog routes apply to peers without their own updates.
// PREVENTS: Group-level watchdog silently ignored for peers without update blocks.
func TestParseConfigGroupWatchdogPeerNoUpdate(t *testing.T) {
	jsonData := `{
		"bgp": {
			"group": {
				"test-group": {
					"update": {
						"default": {
							"attribute": {"origin": "igp", "next-hop": "1.1.1.1"},
							"nlri": {"ipv4/unicast": {"content": "add 10.0.0.0/24"}},
							"watchdog": {"name": "health"}
						}
					},
					"peer": {
						"peer1": {
							"connection": {"remote": {"ip": "10.0.0.1"}}
						}
					}
				}
			}
		}
	}`

	peerPools, err := parseConfig(jsonData)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	pools, ok := peerPools["10.0.0.1"]
	if !ok {
		t.Fatal("no pools for 10.0.0.1 -- group watchdog not inherited")
	}

	healthPool := pools.GetPool("health")
	if healthPool == nil {
		t.Fatal("missing 'health' pool from group-level watchdog")
		return
	}
	if len(healthPool.Routes()) != 1 {
		t.Fatalf("health pool: want 1 route, got %d", len(healthPool.Routes()))
	}
}

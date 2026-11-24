// Design: docs/architecture/config/syntax.md — BGP peer-group bus block inheritance
// Related: resolve.go — ResolveBGPTree and schema-guided ze:replace-whole merging

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestBusBlockInheritance verifies per-item group-to-peer inheritance and
// schema-driven ze:replace-whole semantics across every direction-grouped bus
// block field and graceful-restart (S6 / M-14 / A-8).
//
// VALIDATES:
//   - peer send > to replaces group send > to whole while inheriting group
//     receive > from, send > allow, receive > allow, and unwritten messages
//   - peer receive > from replaces group receive > from whole while inheriting
//     group send > to and allow-lists
//   - peer send > allow and receive > allow replace group allow-lists whole
//   - writing `none` on peer replaces a non-empty group list with "none"
//   - peer send > message <m> and receive > message <m> replace the group's
//     entry for <m> whole (not merging group's to/from/tag for <m>) while
//     sibling message entries not written on the peer stay inherited
//   - list tag carries ze:replace-whole so writing tag <t> replaces group's
//     tag <t> whole while preserving sibling tags
//   - graceful-restart replaces group's graceful-restart container whole via
//     ze:replace-whole without a hardcoded replacePaths map
//   - sibling peers in the same group do not alias or corrupt group state
//
// PREVENTS: group bus lists or per-message entries leaking into a peer that
// replaced them, or peer overrides wiping sibling bus fields not written by
// the peer.
func TestBusBlockInheritance(t *testing.T) {
	restoreTargets := yang.ResetBusParticipantTargetsForTest()
	t.Cleanup(restoreTargets)
	yang.RegisterBusParticipantTarget("/bgp/peer")
	yang.RegisterBusParticipantTarget("/bgp/group")
	yang.RegisterBusParticipantTarget("/bgp/group/peer")

	schema, err := config.YANGSchema()
	require.NoError(t, err)

	// Confirm the schema itself carries ReplaceWhole on bus lists and graceful-restart.
	bgpNode, ok := schema.Get("bgp").(*config.ContainerNode)
	require.True(t, ok)
	peerList, ok := bgpNode.Get("peer").(*config.ListNode)
	require.True(t, ok)
	busNode, ok := peerList.Get("bus").(*config.ContainerNode)
	require.True(t, ok, "bgp > peer must carry synthesized container bus")
	sendNode, ok := busNode.Get("send").(*config.ContainerNode)
	require.True(t, ok)
	sendMsgList, ok := sendNode.Get("message").(*config.ListNode)
	require.True(t, ok)
	assert.True(t, sendMsgList.ReplaceWhole, "bus > send > message must carry ze:replace-whole")
	sendTagList, ok := sendMsgList.Get("tag").(*config.ListNode)
	require.True(t, ok)
	assert.True(t, sendTagList.ReplaceWhole, "bus > send > message > tag must carry ze:replace-whole")

	input := `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	group transit {
		session {
			asn { remote 65001; }
			family { ipv4/unicast { } }
			capability {
				graceful-restart {
					short-lived { restart-time 120; }
					long-lived { family { ipv4/unicast 7200; } }
				}
			}
		}
		bus {
			send {
				allow [ bgp:update bgp:state ];
				to [ /bus/endpoint/bgp-rib /bus/endpoint/collector ];
				message bgp:update {
					to [ /bus/endpoint/bgp-rib ];
					tag direction {
						value [ received ];
					}
				}
				message bgp:state {
					to [ /bus/endpoint/collector ];
				}
			}
			receive {
				allow [ bgp:update bgp:refresh ];
				from [ /bus/endpoint/bgp-rib /bus/endpoint/collector ];
				message bgp:update {
					from [ /bus/endpoint/bgp-rib ];
					tag direction {
						value [ sent ];
					}
				}
				message bgp:refresh {
					from [ /bus/endpoint/collector ];
				}
			}
		}

		peer inherit_all {
			connection { remote { ip 10.0.0.1; } }
		}

		peer override_send_to {
			connection { remote { ip 10.0.0.2; } }
			bus {
				send {
					to [ /bus/endpoint/only-one ];
				}
			}
		}

		peer override_receive_from_and_allow {
			connection { remote { ip 10.0.0.3; } }
			bus {
				send {
					allow [ bgp:state ];
				}
				receive {
					allow [ bgp:update ];
					from [ /bus/endpoint/only-sender ];
				}
			}
		}

		peer override_none {
			connection { remote { ip 10.0.0.4; } }
			bus {
				send {
					to none;
					allow none;
				}
				receive {
					from none;
					allow none;
				}
			}
		}

		peer override_messages {
			connection { remote { ip 10.0.0.5; } }
			bus {
				send {
					message bgp:update {
						to [ /bus/endpoint/custom-sink ];
					}
				}
				receive {
					message bgp:update {
						tag direction {
							value [ received ];
						}
					}
				}
			}
		}

		peer override_gr {
			connection { remote { ip 10.0.0.6; } }
			session {
				capability {
					graceful-restart {
						short-lived { restart-time 45; }
					}
				}
			}
		}
	}
}
`

	tree, err := config.NewParser(schema).Parse(input)
	require.NoError(t, err)

	resolved, err := ResolveBGPTree(tree)
	require.NoError(t, err)

	peers, ok := resolved["peer"].(map[string]any)
	require.True(t, ok)

	busOf := func(peerName string) (sendMap, recvMap map[string]any) {
		t.Helper()
		p, ok := peers[peerName].(map[string]any)
		require.True(t, ok, "missing peer %s", peerName)
		b, ok := p["bus"].(map[string]any)
		require.True(t, ok, "missing bus block on peer %s", peerName)
		s, ok := b["send"].(map[string]any)
		require.True(t, ok, "missing bus > send on peer %s", peerName)
		r, ok := b["receive"].(map[string]any)
		require.True(t, ok, "missing bus > receive on peer %s", peerName)
		return s, r
	}

	t.Run("inherit_all keeps every group bus field", func(t *testing.T) {
		s, r := busOf("inherit_all")
		assert.Equal(t, []string{"bgp:update", "bgp:state"}, s["allow"])
		assert.Equal(t, []string{"/bus/endpoint/bgp-rib", "/bus/endpoint/collector"}, s["to"])
		assert.Equal(t, []string{"bgp:update", "bgp:refresh"}, r["allow"])
		assert.Equal(t, []string{"/bus/endpoint/bgp-rib", "/bus/endpoint/collector"}, r["from"])
		sMsgs := s["message"].(map[string]any)
		assert.Contains(t, sMsgs, "bgp:update")
		assert.Contains(t, sMsgs, "bgp:state")
	})

	t.Run("override_send_to replaces send > to whole while inheriting siblings", func(t *testing.T) {
		s, r := busOf("override_send_to")
		assert.Equal(t, "/bus/endpoint/only-one", s["to"])
		assert.Equal(t, []string{"bgp:update", "bgp:state"}, s["allow"])
		assert.Equal(t, []string{"/bus/endpoint/bgp-rib", "/bus/endpoint/collector"}, r["from"])
		assert.Equal(t, []string{"bgp:update", "bgp:refresh"}, r["allow"])
	})

	t.Run("override_receive_from_and_allow replaces written lists and keeps send > to", func(t *testing.T) {
		s, r := busOf("override_receive_from_and_allow")
		assert.Equal(t, "bgp:state", s["allow"])
		assert.Equal(t, []string{"/bus/endpoint/bgp-rib", "/bus/endpoint/collector"}, s["to"])
		assert.Equal(t, "bgp:update", r["allow"])
		assert.Equal(t, "/bus/endpoint/only-sender", r["from"])
	})

	t.Run("override_none replaces group lists with none", func(t *testing.T) {
		s, r := busOf("override_none")
		assert.Equal(t, "none", s["to"])
		assert.Equal(t, "none", s["allow"])
		assert.Equal(t, "none", r["from"])
		assert.Equal(t, "none", r["allow"])
	})

	t.Run("override_messages replaces written message entry whole and inherits sibling messages", func(t *testing.T) {
		s, r := busOf("override_messages")

		sMsgs := s["message"].(map[string]any)
		require.Contains(t, sMsgs, "bgp:update")
		require.Contains(t, sMsgs, "bgp:state", "unwritten sibling send > message must inherit from group")
		sUpdate := sMsgs["bgp:update"].(map[string]any)
		assert.Equal(t, "/bus/endpoint/custom-sink", sUpdate["to"])
		assert.NotContains(t, sUpdate, "tag", "replaced send > message entry must not inherit group's tag")
		sState := sMsgs["bgp:state"].(map[string]any)
		assert.Equal(t, "/bus/endpoint/collector", sState["to"])

		rMsgs := r["message"].(map[string]any)
		require.Contains(t, rMsgs, "bgp:update")
		require.Contains(t, rMsgs, "bgp:refresh", "unwritten sibling receive > message must inherit from group")
		rUpdate := rMsgs["bgp:update"].(map[string]any)
		assert.NotContains(t, rUpdate, "from", "replaced receive > message entry must not inherit group's from")
		rTag := rUpdate["tag"].(map[string]any)["direction"].(map[string]any)
		assert.Equal(t, "received", rTag["value"])
		rRefresh := rMsgs["bgp:refresh"].(map[string]any)
		assert.Equal(t, "/bus/endpoint/collector", rRefresh["from"])
	})

	t.Run("tag list replaces written tag entry whole while inheriting sibling tags", func(t *testing.T) {
		dst := map[string]any{
			"tag": map[string]any{
				"direction": map[string]any{"value": []string{"received", "sent"}},
				"source":    map[string]any{"value": "bgp"},
			},
		}
		src := map[string]any{
			"tag": map[string]any{
				"direction": map[string]any{"value": "received"},
			},
		}
		deepMergeMapsWithSchema(dst, src, nil, sendMsgList)
		tags := dst["tag"].(map[string]any)
		assert.Equal(t, "received", tags["direction"].(map[string]any)["value"])
		assert.Equal(t, "bgp", tags["source"].(map[string]any)["value"], "unwritten sibling tag must stay inherited")
	})

	t.Run("graceful-restart replaces group container whole via ze:replace-whole", func(t *testing.T) {
		p := peers["override_gr"].(map[string]any)
		gr := p["session"].(map[string]any)["capability"].(map[string]any)["graceful-restart"].(map[string]any)
		assert.Contains(t, gr, "short-lived")
		assert.Equal(t, "45", gr["short-lived"].(map[string]any)["restart-time"])
		assert.NotContains(t, gr, "long-lived", "peer graceful-restart replaces group container whole")
	})
}

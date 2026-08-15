package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// graphFromConfig runs the whole config path a running daemon runs -- parse,
// ResolveBGPTree's group merge, PeersFromTree, then the index build -- and hands
// back the index the plugin server would hold.
func graphFromConfig(t *testing.T, input string) *pluginserver.DeliveryGraph {
	t.Helper()
	r, err := LoadReactor(input)
	require.NoError(t, err)

	settings := make([]*reactor.PeerSettings, 0)
	for _, p := range r.Peers() {
		settings = append(settings, p.Settings())
	}
	srv := &pluginserver.Server{}
	srv.UpdateDeliveryGraph(bgpevents.Namespace, reactor.DeliveryPeersFromSettings(settings))
	return srv.DeliveryGraph()
}

// fedBy returns the processes a peer feeds for one event type, asked exactly as
// the peer-scoped delivery sites ask it (bgp/server/events.go).
func fedBy(g *pluginserver.DeliveryGraph, eventType string, dir events.Direction, peerAddr string) []string {
	return g.Receivers(
		events.LookupNamespaceID(bgpevents.Namespace),
		events.LookupEventTypeID(eventType),
		dir,
		peerAddr,
	)
}

// TestGraphMemberListReplacesGroupList verifies AC-6b in BOTH directions: the
// member that restates a process replaces its group's list for itself, and every
// other member of the group keeps the group's list.
//
// VALIDATES: AC-6b, and the merge semantic the spec fixed rather than changed --
// `receive` is a leaf-list outside cumulativePaths, so a member's list replaces
// rather than unions (bgp/config/resolve.go).
// PREVENTS: an implementation that drops the group's list entirely, which passes
// a test asserting only the member's half.
func TestGraphMemberListReplacesGroupList(t *testing.T) {
	g := graphFromConfig(t, `
plugin {
    external looking-glass {
        run /bin/true;
    }
}
bgp {
    router-id 10.0.0.1;
    session {
        asn {
            local 65000;
        }
    }
    group transit {
        attach process looking-glass {
            receive [ update-received state ];
        }
        peer inherits {
            connection {
                remote {
                    ip 192.0.2.1;
                }
                local {
                    ip auto;
                }
            }
            session {
                asn {
                    remote 65001;
                }
            }
        }
        peer restates {
            connection {
                remote {
                    ip 192.0.2.2;
                }
                local {
                    ip auto;
                }
            }
            session {
                asn {
                    remote 65002;
                }
            }
            attach process looking-glass {
                receive [ state ];
            }
        }
    }
}
`)

	// The member that restates the block: its own list replaces the group's, so
	// state survives and the group's update grant does not.
	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.2"))
	assert.Empty(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.2"),
		"the member's list replaces the group's, so the group's update grant is gone")

	// The other member keeps the group's list whole. A build that dropped the
	// group's bindings, or that let the member's list overwrite the group's for
	// everybody, fails here.
	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.1"))
}

// TestGraphInheritsGroupAttachBlock verifies a peer that states no attach block
// of its own is fed through the one its group states.
//
// VALIDATES: the index is built from RESOLVED settings. config/graph.go's
// addProcessBindings reads the peer's own subtree and misses every inherited
// binding, so building from the document would feed this peer nothing.
// PREVENTS: a group-configured fleet going silent the moment delivery consults
// the index.
func TestGraphInheritsGroupAttachBlock(t *testing.T) {
	g := graphFromConfig(t, `
bgp {
    router-id 10.0.0.1;
    session {
        asn {
            local 65000;
        }
    }
    group transit {
        attach process looking-glass {
            receive [ update-received ];
            send [ update ];
        }
        peer first {
            connection {
                remote {
                    ip 192.0.2.1;
                }
                local {
                    ip auto;
                }
            }
            session {
                asn {
                    remote 65001;
                }
            }
        }
    }
    peer standalone {
        connection {
            remote {
                ip 198.51.100.1;
            }
            local {
                ip auto;
            }
        }
        session {
            asn {
                remote 65003;
            }
        }
    }
}
`)

	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	assert.Empty(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "198.51.100.1"),
		"a peer that attaches nothing feeds nobody")

	peers := g.Inspect()
	require.Len(t, peers, 2)
	byAddr := map[string][]pluginserver.ProcessEdges{}
	for _, p := range peers {
		byAddr[p.Peer] = p.Processes
	}
	assert.Equal(t, []pluginserver.ProcessEdges{{
		Process: "looking-glass",
		Receive: []string{"update-received"},
		Send:    []string{"update"},
	}}, byAddr["192.0.2.1"])
	assert.Empty(t, byAddr["198.51.100.1"])
}

// storyOneConfig is End-to-End User Story 1 from the spec, with the leaves every
// peer needs added: the story states the relationships and elides `connection`
// and `session`, which no peer parses without.
const storyOneConfig = `
plugin {
    external looking-glass {
        run /bin/true;
    }
    external route-injector {
        run /bin/true;
    }
}
bgp {
    router-id 10.0.0.1;
    session {
        asn {
            local 65000;
        }
    }
    group transit {
        attach process looking-glass {
            receive [ update state ];
        }
        peer transit-a {
            connection {
                remote {
                    ip 192.0.2.1;
                }
                local {
                    ip auto;
                }
            }
            session {
                asn {
                    remote 65001;
                }
            }
        }
        peer transit-b {
            connection {
                remote {
                    ip 192.0.2.2;
                }
                local {
                    ip auto;
                }
            }
            session {
                asn {
                    remote 65002;
                }
            }
        }
        peer transit-c {
            connection {
                remote {
                    ip 192.0.2.3;
                }
                local {
                    ip auto;
                }
            }
            session {
                asn {
                    remote 65003;
                }
            }
            attach process looking-glass {
                receive [ state ];
            }
        }
    }
    peer both-programs {
        connection {
            remote {
                ip 198.51.100.1;
            }
            local {
                ip auto;
            }
        }
        session {
            asn {
                remote 65004;
            }
        }
        attach process looking-glass {
            receive [ update state ];
        }
        attach process route-injector {
            send [ update ];
        }
    }
    peer injector-only {
        connection {
            remote {
                ip 198.51.100.2;
            }
            local {
                ip auto;
            }
        }
        session {
            asn {
                remote 65005;
            }
        }
        attach process route-injector {
            receive [ state ];
            send [ update ];
        }
    }
    peer unattached {
        connection {
            remote {
                ip 203.0.113.1;
            }
            local {
                ip auto;
            }
        }
        session {
            asn {
                remote 65006;
            }
        }
    }
}
`

// TestStoryOneProducesItsDeliveryTable runs End-to-End User Story 1 and asserts
// the exact table printed beside it: which program each of the six peers feeds,
// and which may announce to it.
//
// VALIDATES: the Critical Review Checklist's feature-completeness row. A story
// whose config cannot be run, or whose table nothing checks, is prose.
// PREVENTS: a design the spec describes and the code does not produce.
func TestStoryOneProducesItsDeliveryTable(t *testing.T) {
	g := graphFromConfig(t, storyOneConfig)

	byAddr := map[string]map[string]pluginserver.ProcessEdges{}
	for _, peer := range g.Inspect() {
		procs := map[string]pluginserver.ProcessEdges{}
		for _, p := range peer.Processes {
			procs[p.Process] = p
		}
		byAddr[peer.Peer] = procs
	}
	require.Len(t, byAddr, 6)

	for _, row := range []struct {
		addr             string
		glassFed         []string
		injectorFed      []string
		injectorAnnounce []string
	}{
		{addr: "192.0.2.1", glassFed: []string{"state", "update"}},
		{addr: "192.0.2.2", glassFed: []string{"state", "update"}},
		{addr: "192.0.2.3", glassFed: []string{"state"}},
		{addr: "198.51.100.1", glassFed: []string{"state", "update"}, injectorAnnounce: []string{"update"}},
		{addr: "198.51.100.2", injectorFed: []string{"state"}, injectorAnnounce: []string{"update"}},
		{addr: "203.0.113.1"},
	} {
		procs := byAddr[row.addr]
		assert.Equal(t, row.glassFed, procs["looking-glass"].Receive, "%s feeds looking-glass", row.addr)
		assert.Equal(t, row.injectorFed, procs["route-injector"].Receive, "%s feeds route-injector", row.addr)
		assert.Equal(t, row.injectorAnnounce, procs["route-injector"].Send, "%s lets route-injector announce", row.addr)
		assert.Empty(t, procs["looking-glass"].Send, "%s lets looking-glass announce", row.addr)
	}
}

// TestStoryTwoProducesItsTwoIndependentDirections runs End-to-End User Story 2:
// one peer, one program, two independent directions.
//
// VALIDATES: the Critical Review Checklist's feature-completeness row for the
// second story, and the pair that makes the block expressive -- `receive` is what
// the program is fed, `send` is what it may originate.
// PREVENTS: the two lists being read as one relationship.
func TestStoryTwoProducesItsTwoIndependentDirections(t *testing.T) {
	g := graphFromConfig(t, `
plugin {
    external policy-engine {
        run /bin/true;
    }
}
bgp {
    router-id 10.0.0.1;
    session {
        asn {
            local 65000;
        }
    }
    peer policy {
        connection {
            remote {
                ip 198.51.100.3;
            }
            local {
                ip auto;
            }
        }
        session {
            asn {
                remote 65007;
            }
        }
        attach process policy-engine {
            receive [ update state ];
            send [ update ];
        }
    }
}
`)

	peers := g.Inspect()
	require.Len(t, peers, 1)
	require.Len(t, peers[0].Processes, 1)
	assert.Equal(t, "198.51.100.3", peers[0].Peer)
	assert.Equal(t, pluginserver.ProcessEdges{
		Process: "policy-engine",
		Receive: []string{"state", "update"},
		Send:    []string{"update"},
	}, peers[0].Processes[0])

	// A plain token grants BOTH directions, so the program is fed the UPDATEs ze
	// sends as well as the ones it receives.
	assert.Equal(t, []string{"policy-engine"}, fedBy(g, bgpevents.EventUpdate, events.DirSent, "198.51.100.3"))
	assert.Equal(t, []string{"policy-engine"}, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "198.51.100.3"))
}

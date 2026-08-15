package bgpconfig

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// parityConfig declares one static peer and one dynamic group whose blocks state
// the same thing, under a bgp level that states policy for both.
//
// Every leaf here is one an IXP route server sets, and each was reported dropped
// on the dynamic path on 2026-08-13: the filter chains, MD5, BFD, capture, the
// community send list, as-override, route-reflector-client, cluster-id,
// accept-srv6-prefix-sid, link-local, the local-as options, the process
// bindings, and the timers with their RFC 4271 Section 4.2 and RFC 9687 validation.
const parityConfig = `
bgp {
	router-id 1.2.3.4;

	policy {
		prefix-list MEMBERS {
			entry 10.0.0.0/8 {
				action accept;
			}
		}
		loop-detection LOOPS {
			allow-own-as 2;
			cluster-id 9.9.9.9;
		}
	}

	filter {
		import [ MEMBERS ];
	}

	session {
		asn { local 65000; }
	}

	group ix {
		connection {
			remote { ip dynamic; connect false; range 10.0.0.0/8; }
			local  { ip 10.0.0.1; accept true; }
			md5 { password ixp-secret; }
			ttl { max 2; }
			bfd { mode single-hop; }
		}
		session {
			asn { local 65010; local-options [ no-prepend replace-as ]; }
			router-id 5.6.7.8;
			as-override true;
			route-reflector-client true;
			cluster-id 8.8.8.8;
			accept-srv6-prefix-sid true;
			link-local fe80::1;
			community { send [ standard large ]; }
			family {
				ipv4/unicast {
					mode enable;
					prefix { maximum 10000; }
				}
			}
			capability { route-refresh enable; }
		}
		timer {
			receive-hold-time 90;
			keepalive 30;
			connect-retry 7;
		}
		filter {
			import [ MEMBERS ];
		}
		attach process rib {
			receive [ update ];
			send [ update ];
		}
	}

	group static-twin {
		connection {
			local  { ip 10.0.0.1; accept true; }
			md5 { password ixp-secret; }
			ttl { max 2; }
			bfd { mode single-hop; }
		}
		session {
			asn { local 65010; local-options [ no-prepend replace-as ]; }
			router-id 5.6.7.8;
			as-override true;
			route-reflector-client true;
			cluster-id 8.8.8.8;
			accept-srv6-prefix-sid true;
			link-local fe80::1;
			community { send [ standard large ]; }
			family {
				ipv4/unicast {
					mode enable;
					prefix { maximum 10000; }
				}
			}
			capability { route-refresh enable; }
		}
		timer {
			receive-hold-time 90;
			keepalive 30;
			connect-retry 7;
		}
		filter {
			import [ MEMBERS ];
		}
		attach process rib {
			receive [ update ];
			send [ update ];
		}

		peer member {
			connection {
				remote { ip 10.0.0.2; connect false; }
			}
			session {
				asn { remote 65020; }
			}
		}
	}
}
`

// TestDynamicGroupTemplateMatchesAStaticPeer parses one config holding a static
// peer and a dynamic group that state the same settings, and refuses any field
// on which the two disagree.
//
// VALIDATES: a dynamic group's template is built by the same parser and the same
// post-parse layers as a statically configured peer.
// PREVENTS: the defect CLASS. The dynamic group had its own parser and its own
// walk of the config tree, so a leaf read for a static peer and not for a group
// was dropped in silence. The four bugs of 2026-08-13 were each one such leaf,
// and the severe one is in this config: no `ImportFilters` assignment was
// reachable from the dynamic path at all, so an IXP route server applied NO
// import or export policy to any member -- no prefix filtering, no AS-path
// filtering, no community handling, no IRR validation.
//
// A leaf added to the peer parser and not to the group's makes this test fail on
// the field it lands in, which is the point: the next reader gets the field name
// rather than a report from an operator.
func TestDynamicGroupTemplateMatchesAStaticPeer(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(parityConfig)
	require.NoError(t, err)

	peers, groups, err := peersAndDynamicGroups(tree)
	require.NoError(t, err)
	require.Len(t, groups, 1, "one dynamic group")
	require.Len(t, peers, 1, "one static peer")

	dyn, static := groups[0].Settings, peers[0]

	// What the two cannot agree on, and why. Everything else must match.
	diverge := map[string]string{
		"Name":       "the group's name against the peer's name",
		"GroupName":  "each belongs to the group it was declared in",
		"Address":    "the group states `ip dynamic`; a member's address arrives with its connection",
		"PeerAS":     "RFC 4271 Section 4.2: a dynamic member's AS arrives in its OPEN",
		"IsDynamic":  "the flag that separates the two populations",
		"Connection": "a dynamic group only ever accepts; the static peer's mode is its own",
	}

	dv, sv := reflect.ValueOf(*dyn), reflect.ValueOf(*static)
	typ := dv.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if _, ok := diverge[name]; ok {
			continue
		}
		assert.Equalf(t, sv.Field(i).Interface(), dv.Field(i).Interface(),
			"PeerSettings.%s differs between a static peer and a dynamic group configured identically.\n"+
				"Both are parsed by reactor.parsePeerSettings and both walk peersAndDynamicGroups, so a "+
				"difference here means one path reads a leaf the other does not. Fix the parser or the "+
				"walk; do not special-case the field.", name)
	}
}

// TestDynamicGroupInheritsTheFilterChain is the severe case on its own, stated
// as the operator's outcome rather than as a struct comparison.
//
// VALIDATES: the bgp-level and group-level import and export chains reach a
// dynamic group's members, canonicalized and with the loop-detection default
// prepended, exactly as they reach a static peer.
// PREVENTS: an IXP route server accepting every member and filtering none of
// them. The config above states `filter import [ MEMBERS ]` at two levels, and
// until 2026-08-13 a dynamic member's chain was empty.
func TestDynamicGroupInheritsTheFilterChain(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(parityConfig)
	require.NoError(t, err)

	_, groups, err := peersAndDynamicGroups(tree)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	dyn := groups[0].Settings

	names := make([]string, 0, len(dyn.ImportFilters))
	for _, ref := range dyn.ImportFilters {
		names = append(names, ref.Name)
	}

	// LOOPS first: prependDefaultFilters puts loop detection at the head of every
	// import chain (RFC 4271 Section 9). Then MEMBERS twice, once from the bgp
	// level and once from the group, because the chains accumulate.
	require.Equal(t, []string{"LOOPS", "bgp-filter-prefix:MEMBERS", "bgp-filter-prefix:MEMBERS"}, names)

	// The loop-detection entry's own settings landed too, which is the layer that
	// reads the chain it was just given.
	assert.Equal(t, uint8(2), dyn.LoopAllowOwnAS, "allow-own-as from the loop-detection entry named in the chain")
	assert.Equal(t, uint32(0x09090909), dyn.LoopClusterID, "cluster-id from the same entry")
}

// TestDynamicGroupRefusesAHoldTimeTheStaticPathRefuses drives RFC 4271
// Section 4.2 timer validation from the dynamic-group entry point.
//
// RFC 4271 Section 4.2: "Hold Time ... MUST be either zero or at least three
// seconds."
//
// VALIDATES: the group's timers are validated by the same code as a peer's.
// PREVENTS: a dynamic group accepting a hold time the static path refuses. The
// group had its own timer parser, which read the four leaves and validated none
// of them, so `receive-hold-time 2` was accepted and advertised.
func TestDynamicGroupRefusesAHoldTimeTheStaticPathRefuses(t *testing.T) {
	_, err := reactor.ParseDynamicGroupTemplate("ix", map[string]any{
		"connection": map[string]any{"local": map[string]any{"ip": "10.0.0.1"}},
		"session":    map[string]any{"asn": map[string]any{"local": "65000"}},
		"timer":      map[string]any{"receive-hold-time": "2"},
	}, 65000, 0x01020304)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "RFC 4271")
}

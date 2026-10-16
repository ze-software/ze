package config

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestRFC7950MandatoryRefusesBGPConfig proves RFC7950-7.6.5-1 at LoadConfig,
// the <copy-config> window: bgp, bgp/session and bgp/session/asn are
// non-presence containers, so bgp/session/asn/local has no ancestor that is
// not one and MUST exist whenever the bgp section exists.
//
// VALIDATES: a bgp section with no local ASN anywhere is refused, naming the
// leaf and the set command that supplies it; one with the leaf is accepted.
// Both sections carry one peer, because a bgp section with no session
// does nothing and requires neither leaf (owner, 2026-10-16)
// (TestMandatoryBGPLeafNotRequiredWithNoSession).
// PREVENTS: the walk stopping at the absent session container, which let the
// config load with no local ASN.
//
// Method: load the two configs through LoadConfig and read the refusal.
//
// RFC requirement: RFC7950-7.6.5-1 negative -- LoadConfig refuses a bgp section whose mandatory bgp/session/asn/local sits under absent non-presence containers, naming the path and the set command.
// RFC requirement: RFC7950-7.6.5-1 positive -- LoadConfig accepts the same section once bgp/session/asn/local is present.
func TestRFC7950MandatoryRefusesBGPConfig(t *testing.T) {
	peer := "\tpeer p1 {\n\t\tconnection { remote { ip 192.0.2.2 } }\n\t\tsession { asn { remote 65001; } }\n\t}\n"
	_, err := LoadConfig("bgp {\n\trouter-id 192.0.2.1\n"+peer+"}\n", "test.conf", nil)
	if err == nil {
		t.Fatal("LoadConfig accepted a bgp section with no local ASN")
	}
	if !errors.Is(err, ErrCustomValidation) {
		t.Fatalf("refusal is not a validation refusal: %v", err)
	}
	for _, want := range []string{"bgp/session/asn/local", "`set bgp session asn local <value>`"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not carry %q", err, want)
		}
	}

	complete := "bgp {\n\trouter-id 192.0.2.1\n\tsession {\n\t\tasn {\n\t\t\tlocal 65000\n\t\t}\n\t}\n" + peer + "}\n"
	if _, err := LoadConfig(complete, "test.conf", nil); err != nil {
		t.Errorf("LoadConfig refused a bgp section carrying every mandatory leaf: %v", err)
	}
}

// TestMandatoryBGPLeafInheritedBySessions proves the owner's ruling of
// 2026-10-12: a global mandatory leaf that every session resolves from its
// group or carries itself counts as present, and one session without it
// brings the refusal back.
//
// VALIDATES: router-id and the local ASN supplied per peer or per group are
// accepted with no global value; a peer that resolves neither is refused.
// PREVENTS: refusing the per-peer and per-group configs Ze's inheritance was
// built for, and accepting a session left with no router-id.
//
// Method: LoadConfig over three configs.
func TestMandatoryBGPLeafInheritedBySessions(t *testing.T) {
	perPeer := `bgp {
	peer p1 {
		connection { remote { ip 192.0.2.2 } }
		session {
			asn { local 65000; remote 65001; }
			router-id 192.0.2.1
		}
	}
}
`
	if _, err := LoadConfig(perPeer, "test.conf", nil); err != nil {
		t.Errorf("a peer carrying router-id and local ASN was refused: %v", err)
	}

	perGroup := `bgp {
	group g {
		session {
			asn { local 65000; }
			router-id 192.0.2.1
		}
		peer p1 {
			connection { remote { ip 192.0.2.2 } }
			session { asn { remote 65001; } }
		}
	}
}
`
	if _, err := LoadConfig(perGroup, "test.conf", nil); err != nil {
		t.Errorf("a peer inheriting router-id and local ASN from its group was refused: %v", err)
	}

	oneWithout := `bgp {
	peer p1 {
		connection { remote { ip 192.0.2.2 } }
		session {
			asn { local 65000; remote 65001; }
			router-id 192.0.2.1
		}
	}
	peer p2 {
		connection { remote { ip 192.0.2.3 } }
		session { asn { local 65000; remote 65002; } }
	}
}
`
	_, err := LoadConfig(oneWithout, "test.conf", nil)
	if err == nil {
		t.Fatal("a peer resolving no router-id was accepted with no global router-id")
	}
	if !strings.Contains(err.Error(), "bgp/router-id") {
		t.Errorf("refusal %q does not name bgp/router-id", err)
	}
}

// TestMandatoryBGPLeafNotRequiredWithNoSession proves the owner's reading of
// 2026-10-16: a bgp section with no peer and no group starts no session and
// does nothing, so no
// session consumes bgp/router-id or bgp/session/asn/local and neither is
// required; the first peer brings the requirement back.
//
// VALIDATES: an empty bgp section, and one holding only an unrelated leaf,
// load; the same section with one peer resolving neither leaf is refused
// naming both.
// PREVENTS: refusing the `bgp { }` the ExaBGP migration writes for a config
// with processes and no neighbor, and the inverse, a peer accepted with no
// router-id or local ASN.
//
// Method: LoadConfig over three configs.
func TestMandatoryBGPLeafNotRequiredWithNoSession(t *testing.T) {
	for _, config := range []string{"bgp {\n}\n", "bgp {\n\trouter-id 192.0.2.1\n}\n"} {
		if _, err := LoadConfig(config, "test.conf", nil); err != nil {
			t.Errorf("a bgp section with no session was refused: %v\n%s", err, config)
		}
	}

	onePeer := `bgp {
	peer p1 {
		connection { remote { ip 192.0.2.2 } }
		session { asn { remote 65001; } }
	}
}
`
	_, err := LoadConfig(onePeer, "test.conf", nil)
	if err == nil {
		t.Fatal("a peer resolving no router-id and no local ASN was accepted")
	}
	for _, want := range []string{"bgp/router-id", "bgp/session/asn/local"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
}

// TestMandatoryBGPNoSessionExcusesOnlySessionLeaves pins the scope of the
// no-session answer.
//
// VALIDATES: a bgp section with no peer and no group still reports a global
// mandatory leaf bgpSessionPathOf does not name (bgp/id in the test module);
// a section holding only an empty, non-dynamic group with no router-id is
// refused naming bgp/router-id, because a group is a session source.
// PREVENTS: EverySessionCarries answering true for every leaf when no source
// exists, or counting a group with no peer as no source, either of which would
// load a config whose session or feature lacks a mandatory value.
//
// Method: bgpMandatoryFailures over the test module with an empty bgp tree,
// and LoadConfig over a section holding one empty group.
func TestMandatoryBGPNoSessionExcusesOnlySessionLeaves(t *testing.T) {
	empty := peerMandatoryFailurePaths(t, func(*Tree) {})
	assert.Equal(t, []string{"bgp/id"}, empty,
		"a global mandatory leaf no session consumes stays required with no session")

	emptyGroup := "bgp {\n\tsession { asn { local 65000; } }\n\tgroup g {\n\t}\n}\n"
	_, err := LoadConfig(emptyGroup, "test.conf", nil)
	if err == nil {
		t.Fatal("a section whose only group resolves no router-id was accepted")
	}
	if !strings.Contains(err.Error(), "bgp/router-id") {
		t.Errorf("refusal %q does not name bgp/router-id", err)
	}
}

// TestMandatoryBGPLeafDynamicGroupCarriesItself proves a dynamic group is a
// session source of its own, members or not.
//
// VALIDATES: a dynamic group (connection remote ip dynamic) whose static member
// peers carry router-id and the local ASN, but which carries neither itself, is
// refused with no global value; the same group carrying both loads.
// PREVENTS: counting the members as the group's sessions. A dynamic session is
// built from the bgp defaults and the group's own fields (resolveDynamicGroup),
// never from a member, so it would start with no router-id.
//
// Method: LoadConfig over two configs.
func TestMandatoryBGPLeafDynamicGroupCarriesItself(t *testing.T) {
	member := `		peer p1 {
			connection { remote { ip 192.0.2.2 } }
			session {
				asn { local 65000; remote 65001; }
				router-id 192.0.2.1
			}
		}
`
	bare := "bgp {\n\tgroup g {\n\t\tconnection { remote { ip dynamic } }\n" +
		"\t\tsession { asn { remote 65001; } }\n" + member + "\t}\n}\n"
	_, err := LoadConfig(bare, "test.conf", nil)
	if err == nil {
		t.Fatal("a dynamic group carrying no router-id was accepted because its members carry one")
	}
	for _, want := range []string{"bgp/router-id", "bgp/session/asn/local"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}

	carried := "bgp {\n\tgroup g {\n\t\tconnection { remote { ip dynamic } }\n" +
		"\t\tsession {\n\t\t\tasn { local 65000; remote 65001; }\n\t\t\trouter-id 192.0.2.1\n\t\t}\n" +
		member + "\t}\n}\n"
	if _, err := LoadConfig(carried, "test.conf", nil); err != nil {
		t.Errorf("a dynamic group carrying router-id and local ASN was refused: %v", err)
	}
}

// peerMandatoryModule declares a bgp section whose peer list, at both levels,
// holds a mandatory leaf, beside a global mandatory leaf a peer may carry. No
// live module has a mandatory node inside a peer or a group yet, so the test
// supplies one.
const peerMandatoryModule = `module t-bgp-conf {
	namespace "urn:t-bgp-conf"; prefix t;
	grouping peer-body {
		leaf name { type string; }
		leaf required { type string; mandatory true; }
		leaf id { type string; }
	}
	container bgp {
		leaf id { type string; mandatory true; }
		list peer { key name; uses peer-body; }
		list group {
			key name;
			leaf name { type string; }
			leaf id { type string; }
			list peer { key name; uses peer-body; }
		}
	}
}`

// peerMandatoryFailurePaths runs bgpMandatoryFailures over a bgp section built
// by fill, against peerMandatoryModule, and returns the path of every failure.
func peerMandatoryFailurePaths(t *testing.T, fill func(bgp *Tree)) []string {
	t.Helper()
	loader := yang.NewLoader()
	require.NoError(t, loader.AddModuleFromText("t-bgp-conf.yang", peerMandatoryModule))
	schema, err := loader.Resolve()
	require.NoError(t, err)

	tree := NewTree()
	bgp := NewTree()
	fill(bgp)
	tree.SetContainer(string(ConfigTypeBGP), bgp)

	var paths []string
	for _, failure := range bgpMandatoryFailures(yang.NewValidator(schema), tree) {
		paths = append(paths, failure.Err.Path)
	}
	slices.Sort(paths)
	return paths
}

// peerEntry builds one peer list entry, carrying each leaf in leaves.
func peerEntry(leaves map[string]string) *Tree {
	entry := NewTree()
	for name, value := range leaves {
		entry.Set(name, value)
	}
	return entry
}

// TestMandatoryBGPPeerAndGroupEntriesAreWalked proves the mandatory check
// reaches inside bgp peer and group entries, and that the session inheritance
// still answers only for a global leaf.
//
// VALIDATES: a standalone peer and a grouped peer each missing their mandatory
// leaf are refused at their own entry path; the same entries carrying it pass;
// a global mandatory leaf every peer carries still counts as present.
// PREVENTS: bgpMandatoryFailures removing peer and group before the walk, so a
// mandatory node a module declares inside an entry was never required.
//
// Method: a test module with a mandatory leaf in the peer list at both levels,
// walked through bgpMandatoryFailures over hand-built trees.
func TestMandatoryBGPPeerAndGroupEntriesAreWalked(t *testing.T) {
	missing := peerMandatoryFailurePaths(t, func(bgp *Tree) {
		bgp.AddListEntry("peer", "p1", peerEntry(map[string]string{"id": "x"}))
		group := peerEntry(map[string]string{"id": "x"})
		group.AddListEntry("peer", "p2", peerEntry(map[string]string{"id": "x"}))
		bgp.AddListEntry("group", "g", group)
	})
	assert.Equal(t, []string{"bgp/group[g]/peer[p2]/required", "bgp/peer[p1]/required"}, missing,
		"a peer entry missing its mandatory leaf must be refused at its own path")

	carried := peerMandatoryFailurePaths(t, func(bgp *Tree) {
		bgp.AddListEntry("peer", "p1", peerEntry(map[string]string{"id": "x", "required": "y"}))
		group := peerEntry(map[string]string{"id": "x"})
		group.AddListEntry("peer", "p2", peerEntry(map[string]string{"required": "y"}))
		bgp.AddListEntry("group", "g", group)
	})
	assert.Empty(t, carried, "every entry carries its mandatory leaf and every session resolves bgp/id")

	oneWithoutID := peerMandatoryFailurePaths(t, func(bgp *Tree) {
		bgp.AddListEntry("peer", "p1", peerEntry(map[string]string{"required": "y"}))
	})
	assert.Equal(t, []string{"bgp/id"}, oneWithoutID,
		"a global mandatory leaf no session carries must still be refused")
}

// TestMandatoryBGPPeerWalkAcceptsACompletePeer proves walking peer entries
// refuses nothing the shipped modules leave optional.
//
// VALIDATES: a bgp section with a router-id and one peer carrying its
// connection and both ASNs loads.
// PREVENTS: the peer walk turning a leaf that is mandatory only in a peer
// sub-container the config leaves out into a refusal of an ordinary peer.
//
// Method: LoadConfig over the config `ze config validate` is tested with.
func TestMandatoryBGPPeerWalkAcceptsACompletePeer(t *testing.T) {
	complete := `bgp {
	router-id 203.0.113.253;
	peer peer1 {
		connection {
			remote { ip 127.0.0.1; }
			local { ip 127.0.0.1; }
		}
		session { asn { remote 65533; local 65533; } }
	}
}`
	if _, err := LoadConfig(complete, "test.conf", nil); err != nil {
		t.Errorf("a complete peer was refused: %v", err)
	}
}

// TestEverySessionCarriesNeverMatchesAnEntryPath proves the guard
// bgpMandatoryFailures relies on: a missing-node path inside a peer or group
// entry is never counted as carried, so an entry-level miss stays a failure.
//
// VALIDATES: with every session and the group carrying "required", the global
// path bgp/required counts as carried, while the entry paths
// bgp/peer[p1]/required and bgp/group[g]/peer[p2]/required do not.
// PREVENTS: EverySessionCarries learning to resolve an entry path (for example
// by stripping its name[key] segments, or by reading the group's value for a
// member), which would let session inheritance excuse a leaf RFC 7950 Section
// 7.6.5 requires inside the entry itself.
//
// Method: EverySessionCarries over a hand-built map in the Tree.ToMap shape,
// with the global path as the positive control.
func TestEverySessionCarriesNeverMatchesAnEntryPath(t *testing.T) {
	data := map[string]any{
		"peer": map[string]any{
			"p1": map[string]any{"required": "y"},
		},
		"group": map[string]any{
			"g": map[string]any{
				"required": "y",
				"peer": map[string]any{
					"p2": map[string]any{"required": "y"},
				},
			},
		},
	}
	require.True(t, EverySessionCarries(data, "bgp/required"),
		"positive control: every session carries the leaf at the global path")
	for _, entryPath := range []string{"bgp/peer[p1]/required", "bgp/group[g]/peer[p2]/required", "bgp/group[g]/required"} {
		assert.False(t, EverySessionCarries(data, entryPath),
			"%s names a node inside a list entry and must never count as carried", entryPath)
	}
}

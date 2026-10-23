// Design: docs/architecture/config/syntax.md — explicit BGP policy final decisions.

package bgpconfig

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/env"
)

// TestPolicyFinalConfiguration drives the public loader, including original
// parent layers with no peers and parents that inheritance could otherwise repair.
func TestPolicyFinalConfiguration(t *testing.T) {
	for _, direction := range []string{"import", "export"} {
		for _, final := range []string{"accept", "reject"} {
			t.Run(direction+"/"+final, func(t *testing.T) {
				_, err := config.LoadConfig("bgp { filter { "+direction+" { final "+final+"; } } }", "", nil)
				if err != nil {
					t.Fatalf("valid final-only policy refused: %v", err)
				}
			})
		}
		for _, tc := range []struct {
			name string
			body string
			path string
		}{
			{"missing-global-no-peers", "filter { " + direction + " {} }", "bgp/filter/" + direction + "/final"},
			{"missing-group-no-peers", "group g { filter { " + direction + " {} } }", "bgp/group/g/filter/" + direction + "/final"},
			{"missing-peer", "peer p { filter { " + direction + " {} } }", "bgp/peer/p/filter/" + direction + "/final"},
			{"parent-not-repaired", "filter { " + direction + " {} } peer p { filter { " + direction + " { final accept; } } }", "bgp/filter/" + direction + "/final"},
			{"group-not-repaired", "group g { filter { " + direction + " {} } peer p { filter { " + direction + " { final reject; } } } }", "bgp/group/g/filter/" + direction + "/final"},
		} {
			t.Run(direction+"/"+tc.name, func(t *testing.T) {
				_, err := config.LoadConfig("bgp { "+tc.body+" }", "", nil)
				if err == nil {
					t.Fatal("invalid original policy loaded")
				}
				if !strings.Contains(err.Error(), tc.path) || !strings.Contains(err.Error(), "final accept") || !strings.Contains(err.Error(), "final reject") {
					t.Fatalf("refusal lacks original path and correction: %v", err)
				}
			})
		}
		for _, body := range []string{direction + " [ test:one ];", direction + " { final permit; }"} {
			_, err := config.LoadConfig("bgp { filter { "+body+" } }", "", nil)
			if err == nil {
				t.Fatalf("obsolete or unknown-final policy loaded: %s", body)
			}
			if !strings.Contains(err.Error(), "bgp/filter/"+direction) || !strings.Contains(err.Error(), "write:") || !strings.Contains(err.Error(), "accept|reject") {
				t.Fatalf("refusal lacks path and schema-driven correction: %v", err)
			}
		}
	}
}

// TestPolicyOriginalTreeValidation checks callers that supply a tree directly,
// rather than passing through the public loader's registered boundary.
func TestPolicyOriginalTreeValidation(t *testing.T) {
	for _, direction := range []string{"import", "export"} {
		for _, final := range []string{"", "permit"} {
			tree := config.NewTree()
			policy := tree.GetOrCreateContainer("bgp").GetOrCreateContainer("filter").GetOrCreateContainer(direction)
			if final != "" {
				policy.Set("final", final)
			}
			if _, err := PeersFromConfigTree(tree); err == nil {
				t.Fatalf("direct original tree accepted %s final %q", direction, final)
			}
		}
	}
}

// TestPolicyInheritance preserves cumulative order and inactive references while
// taking the most-specific present final, including an explicitly empty child.
func TestPolicyInheritance(t *testing.T) {
	const src = `bgp {
		router-id 192.0.2.1; session { asn { local 65000; } }
		filter { import { filters [ ext:global ]; final reject; } export { final reject; } }
		group g {
			filter { import { filters [ ext:group ]; inactive: filters ext:group; final accept; } }
			peer p { connection { remote { ip 192.0.2.2; } local { ip auto; } }
				session { asn { remote 65001; } }
				filter { import { final reject; } }
			}
		}
	}`
	tree, err := config.ParseTreeWithYANG(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := PeersFromConfigTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 {
		t.Fatalf("peers = %d, want 1", len(peers))
	}
	want := []filterapi.FilterRef{{Name: "ext:global"}, {Name: "ext:group", Inactive: true}}
	if !slices.Equal(peers[0].ImportPolicy.Filters(), want) || peers[0].ImportPolicy.Final() != filterapi.FinalReject {
		t.Fatalf("import inheritance = %+v", peers[0].ImportPolicy)
	}
	if !peers[0].ExportPolicy.Configured() || peers[0].ExportPolicy.Final() != filterapi.FinalReject {
		t.Fatalf("absent child failed to inherit export: %+v", peers[0].ExportPolicy)
	}
}

// TestPolicyInactiveContainerIsAbsent proves deactivation removes the whole
// original policy before its mandatory final is judged.
func TestPolicyInactiveContainerIsAbsent(t *testing.T) {
	for _, body := range []string{
		"inactive: filter { import {} }",
		"filter { inactive: import {} export { final reject; } }",
		"inactive: group g { filter { export {} } }",
	} {
		if _, err := config.LoadConfig("bgp { "+body+" }", "", nil); err != nil {
			t.Fatalf("inactive policy was validated: %v", err)
		}
	}
}

// TestPolicyFinalOnlyPeer preserves policy presence without any references and
// leaves the other direction absent, rather than treating both as empty chains.
func TestPolicyFinalOnlyPeer(t *testing.T) {
	for _, direction := range []string{"import", "export"} {
		for _, final := range []string{"accept", "reject"} {
			tree := config.NewTree()
			bgp := buildBGPBlock()
			peer := buildMinimalPeer("192.0.2.2", "65001", "auto")
			peer.GetOrCreateContainer("filter").GetOrCreateContainer(direction).Set("final", final)
			bgp.AddListEntry("peer", "edge", peer)
			tree.SetContainer("bgp", bgp)
			peers, err := PeersFromConfigTree(tree)
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != 1 {
				t.Fatalf("peers = %d, want 1", len(peers))
			}
			policy, absent := peers[0].ImportPolicy, peers[0].ExportPolicy
			if direction == "export" {
				policy, absent = absent, policy
			}
			if !policy.Configured() || len(policy.Filters()) != 0 || policy.AcceptsAtEnd() != (final == "accept") {
				t.Fatalf("%s final %s lost: %+v", direction, final, policy)
			}
			if absent.Configured() || !absent.AcceptsAtEnd() {
				t.Fatalf("absent direction changed behavior: %+v", absent)
			}
		}
	}
}

// TestPolicyDynamicTemplateFinal proves a group's final replaces the global
// final without losing cumulative filters or automatic loop detection.
func TestPolicyDynamicTemplateFinal(t *testing.T) {
	for _, final := range []string{"accept", "reject"} {
		tree, err := config.ParseTreeWithYANG(parityConfig, nil)
		if err != nil {
			t.Fatal(err)
		}
		bgp := tree.GetContainer("bgp")
		bgp.GetContainer("filter").GetContainer("import").Set("final", "reject")
		for _, group := range bgp.GetListOrdered("group") {
			group.Value.GetContainer("filter").GetContainer("import").Set("final", final)
		}
		_, groups, err := peersAndDynamicGroups(tree)
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 1 {
			t.Fatalf("dynamic groups = %d, want 1", len(groups))
		}
		policy := groups[0].Settings.ImportPolicy
		if !policy.Configured() || policy.AcceptsAtEnd() != (final == "accept") {
			t.Fatalf("dynamic final %s lost: %+v", final, policy)
		}
		want := []filterapi.FilterRef{{Name: "LOOPS"}, {Name: "bgp-filter-prefix:MEMBERS"}, {Name: "bgp-filter-prefix:MEMBERS"}}
		if !slices.Equal(policy.Filters(), want) {
			t.Fatalf("dynamic filters = %+v, want %+v", policy.Filters(), want)
		}
	}
}

// TestPolicyCandidateCanBeCompleted keeps parsing separate from validation: an
// editor may add filters first, but cannot load or apply that unfinished policy.
func TestPolicyCandidateCanBeCompleted(t *testing.T) {
	tree, err := config.ParseTreeWithYANG("bgp { filter { import { filters [ ext:one ]; } } }", nil)
	if err != nil {
		t.Fatalf("incomplete editable candidate could not parse: %v", err)
	}
	if _, err := config.ValidateCustomSections(tree); err == nil {
		t.Fatal("incomplete candidate passed validation")
	}
	tree.GetContainer("bgp").GetContainer("filter").GetContainer("import").Set("final", "accept")
	failures, err := config.ValidateCustomSections(tree)
	if err != nil {
		t.Fatalf("completed candidate refused: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("completed candidate has schema failures: %v", failures)
	}
}

// TestPolicySetSyntaxRefusals checks set-form loading rather than only the
// hierarchical parser, retaining the strict refusal through migration fallback.
func TestPolicySetSyntaxRefusals(t *testing.T) {
	for _, statement := range []string{
		"set bgp filter import [ ext:one ]",
		"set bgp filter import ext:one",
		"set bgp filter export final permit",
	} {
		_, err := config.LoadConfig(statement+"\n", "", nil)
		if err == nil {
			t.Fatalf("invalid set policy loaded: %s", statement)
		}
		for _, want := range []string{"bgp/filter/", "write:", "accept|reject"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("set refusal lacks %q: %v", want, err)
			}
		}
	}
}

// TestPolicyRefusalDoesNotPublishEnvironment proves load-time validation runs
// before publishing environment settings from a refused candidate.
func TestPolicyRefusalDoesNotPublishEnvironment(t *testing.T) {
	t.Setenv("ZE_BGP_OPENWAIT", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	before := env.Get("ze.bgp.openwait")
	_, err := config.LoadConfig("environment { bgp { openwait 60; } }\nbgp { filter { import {} } }", "", nil)
	if err == nil {
		t.Fatal("missing-final policy loaded")
	}
	if got := env.Get("ze.bgp.openwait"); got != before {
		t.Fatalf("refused candidate changed environment from %q to %q", before, got)
	}
}

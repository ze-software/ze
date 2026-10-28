// Design: docs/architecture/config/syntax.md -- named in-process policy references.

package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// TestLoopPolicyBinding exercises the real config producer and external-chain
// consumer. Every spelling must retain one reference, its decision and settings,
// without sending the named protocol check to a nonexistent plugin.
func TestLoopPolicyBinding(t *testing.T) {
	for _, spelling := range []string{"", "AUTO", "loop-detection:AUTO", "loop:AUTO"} {
		for _, final := range []string{"", "accept", "reject"} {
			if spelling != "" && final == "" {
				continue // A written direction requires its explicit final.
			}
			for _, inactive := range []bool{false, true} {
				if spelling == "" && inactive {
					continue // Automatic references have no inactive member marker.
				}
				t.Run(spelling+"/"+final+"/"+map[bool]string{false: "active", true: "inactive"}[inactive], func(t *testing.T) {
					tree := config.NewTree()
					bgp := buildBGPBlock()
					entry := config.NewTree()
					entry.Set("allow-own-as", "2")
					entry.Set("cluster-id", "9.9.9.9")
					bgp.GetOrCreateContainer("policy").AddListEntry("loop-detection", "AUTO", entry)
					peer := buildMinimalPeer("192.0.2.2", "65001", "auto")
					if final != "" {
						policy := peer.GetOrCreateContainer("filter").GetOrCreateContainer("import")
						policy.Set("final", final)
						if spelling != "" {
							policy.SetSlice("filters", []string{spelling})
							if inactive {
								if err := policy.DeactivateMultiValue("filters", spelling); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
					bgp.AddListEntry("peer", "edge", peer)
					tree.SetContainer("bgp", bgp)
					peers, err := PeersFromConfigTree(tree)
					if err != nil {
						t.Fatal(err)
					}
					if len(peers) != 1 {
						t.Fatalf("peers = %d, want one", len(peers))
					}
					settings := peers[0]
					policy := settings.ImportPolicy
					if len(policy.Filters()) != 1 {
						t.Fatalf("references = %+v, want exactly one", policy.Filters())
					}
					ref := policy.Filters()[0]
					if ref.Name != "loop:AUTO" || !ref.InProcess() || ref.ActiveExternal() || ref.Inactive != inactive {
						t.Fatalf("bound reference = %+v", ref)
					}
					if policy.Configured() != (final != "") {
						t.Fatalf("presence changed: %+v", policy)
					}
					if final == "" && policy.Final() != 0 {
						t.Fatalf("automatic reference invented final %v", policy.Final())
					}
					if settings.ExportPolicy.Configured() || len(settings.ExportPolicy.Filters()) != 0 {
						t.Fatalf("automatic import changed export: %+v", settings.ExportPolicy)
					}
					if settings.LoopDisabled != inactive {
						t.Fatalf("LoopDisabled = %v, want %v", settings.LoopDisabled, inactive)
					}
					if !inactive {
						if settings.LoopAllowOwnAS != 2 || settings.LoopClusterID != 0x09090909 || settings.ClusterID != 0x09090909 {
							t.Fatalf("loop settings lost: allow=%d loop-cluster=%x session-cluster=%x", settings.LoopAllowOwnAS, settings.LoopClusterID, settings.ClusterID)
						}
					}
					result := reactor.PolicyFilterChain(policy, settings.IsEBGP(), "import", "192.0.2.2", 65001, "origin igp", func(string, string, string, string, uint32, string) reactor.PolicyResponse {
						t.Fatal("configured internal reference reached external filter-update")
						return reactor.PolicyResponse{Action: reactor.PolicyReject, Failed: true}
					})
					if (result.Action == reactor.PolicyAccept) != (final == "accept") {
						t.Fatalf("final %q produced %+v", final, result)
					}
				})
			}
		}
	}
}

// TestLoopPolicyExternalCollision keeps an external process reference distinct
// from a same-named internal instance. Unknown external providers remain guards.
func TestLoopPolicyExternalCollision(t *testing.T) {
	reg := &FilterRegistry{entries: map[string]FilterEntry{
		"AUTO":  {Name: "AUTO", Type: "loop-detection"},
		"OTHER": {Name: "OTHER", Type: "prefix-list"},
	}}
	for _, name := range []string{"external:AUTO", "loop:MISSING", "loop-detection:MISSING", "loop:OTHER", "loop-detection:OTHER"} {
		refs := canonicalizeFilterRefs([]filterapi.FilterRef{{Name: name}}, reg)
		if refs[0].InProcess() || !refs[0].ActiveExternal() {
			t.Fatalf("unresolved reference bypassed external dispatch: %+v", refs[0])
		}
		calls := 0
		result := reactor.PolicyFilterChain(filterapi.InternalPolicy(refs), true, "import", "192.0.2.2", 65001, "origin igp", func(string, string, string, string, uint32, string) reactor.PolicyResponse {
			calls++
			return reactor.PolicyResponse{Action: reactor.PolicyReject, Failed: true}
		})
		if calls != 1 || result.Action != reactor.PolicyReject || !result.Failed {
			t.Fatalf("external guard calls=%d result=%+v", calls, result)
		}
	}
}

// TestLoopPolicyDynamicParity checks every reference spelling on static peers
// and dynamic templates through their shared producer, including deactivation.
func TestLoopPolicyDynamicParity(t *testing.T) {
	for _, spelling := range []string{"LOOPS", "loop-detection:LOOPS", "loop:LOOPS"} {
		for _, inactive := range []bool{false, true} {
			tree, err := config.ParseTreeWithYANG(parityConfig, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, group := range tree.GetContainer("bgp").GetListOrdered("group") {
				policy := group.Value.GetContainer("filter").GetContainer("import")
				policy.SetSlice("filters", []string{spelling})
				policy.Set("final", "reject")
				if inactive {
					if err := policy.DeactivateMultiValue("filters", spelling); err != nil {
						t.Fatal(err)
					}
				}
			}
			peers, groups, err := peersAndDynamicGroups(tree)
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != 1 || len(groups) != 1 {
				t.Fatalf("static/dynamic population = %d/%d, want 1/1", len(peers), len(groups))
			}
			for _, settings := range []*reactor.PeerSettings{peers[0], groups[0].Settings} {
				policy := settings.ImportPolicy
				if !policy.Configured() || policy.Final() != filterapi.FinalReject {
					t.Fatalf("%s lost explicit final: %+v", spelling, policy)
				}
				loops := 0
				for _, ref := range policy.Filters() {
					if ref.InProcess() {
						loops++
						if ref.Name != "loop:LOOPS" || ref.Inactive != inactive {
							t.Fatalf("%s bound reference = %+v", spelling, ref)
						}
					}
				}
				if loops != 1 || settings.LoopDisabled != inactive {
					t.Fatalf("%s loops=%d disabled=%v, want one loop and disabled=%v", spelling, loops, settings.LoopDisabled, inactive)
				}
				if !inactive {
					if settings.LoopAllowOwnAS != 2 || settings.LoopClusterID != 0x09090909 {
						t.Fatalf("%s lost loop settings: allow=%d cluster=%x", spelling, settings.LoopAllowOwnAS, settings.LoopClusterID)
					}
				}
			}
		}
	}
}

// TestLoopPolicyUnsupportedDirection refuses an active ingress-only check on
// export instead of silently skipping it. Inactive remains an explicit decision.
func TestLoopPolicyUnsupportedDirection(t *testing.T) {
	for _, spelling := range []string{"AUTO", "loop-detection:AUTO", "loop:AUTO"} {
		for _, inactive := range []bool{false, true} {
			tree := config.NewTree()
			bgp := buildBGPBlock()
			bgp.GetOrCreateContainer("policy").AddListEntry("loop-detection", "AUTO", config.NewTree())
			peer := buildMinimalPeer("192.0.2.2", "65001", "auto")
			export := peer.GetOrCreateContainer("filter").GetOrCreateContainer("export")
			export.Set("final", "reject")
			export.SetSlice("filters", []string{spelling})
			if inactive {
				if err := export.DeactivateMultiValue("filters", spelling); err != nil {
					t.Fatal(err)
				}
			}
			bgp.AddListEntry("peer", "edge", peer)
			tree.SetContainer("bgp", bgp)
			peers, err := PeersFromConfigTree(tree)
			if !inactive {
				if err == nil {
					t.Fatalf("active export %q accepted without an egress hook", spelling)
				}
				if !strings.Contains(err.Error(), "export filter loop:AUTO is only supported on import") {
					t.Fatalf("wrong export refusal for %q: %v", spelling, err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("inactive export %q refused: %v", spelling, err)
			}
			if len(peers) != 1 {
				t.Fatalf("peers = %d, want one", len(peers))
			}
			policy := peers[0].ExportPolicy
			if policy.Final() != filterapi.FinalReject || len(policy.Filters()) != 1 || !policy.Filters()[0].Inactive {
				t.Fatalf("inactive export decision lost: %+v", policy)
			}
			if peers[0].ImportPolicy.Configured() || len(peers[0].ImportPolicy.Filters()) != 1 {
				t.Fatalf("export reference changed automatic import: %+v", peers[0].ImportPolicy)
			}
		}
	}
}

// TestLoopPolicyDynamicExportRefusal reaches the dynamic-template population:
// the static twin has no export ref and cannot produce the expected refusal.
func TestLoopPolicyDynamicExportRefusal(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(parityConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	group := tree.GetContainer("bgp").GetList("group")["ix"]
	export := group.GetOrCreateContainer("filter").GetOrCreateContainer("export")
	export.Set("final", "accept")
	export.SetSlice("filters", []string{"loop:LOOPS"})
	if _, _, err := peersAndDynamicGroups(tree); err == nil {
		t.Fatal("dynamic export accepted without an egress hook")
	} else if !strings.Contains(err.Error(), "export filter loop:LOOPS is only supported on import") {
		t.Fatalf("wrong dynamic export refusal: %v", err)
	}
}

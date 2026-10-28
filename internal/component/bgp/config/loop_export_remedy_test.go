// Design: docs/architecture/config/syntax.md -- original-scope export remediation.

package bgpconfig

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
)

// TestLoopPolicyExportRemedy applies the command from the real config refusal
// through the editor, proving it addresses the written layer and alias rather
// than an affected peer's inherited, canonical reference.
func TestLoopPolicyExportRemedy(t *testing.T) {
	for _, scope := range []string{"root", "group", "grouped-peer", "peer", "dynamic"} {
		for _, prefix := range []string{"", "loop-detection:", "loop:"} {
			for _, quoted := range []bool{false, true} {
				name := scope + "/" + prefix + "/plain"
				if quoted {
					name = scope + "/" + prefix + "/quoted"
				}
				t.Run(name, func(t *testing.T) {
					tree, target, path, instance := loopExportRemedyConfig(t, scope, quoted)
					before := loopExportRemedySettings(t, tree.Clone())
					spelling := prefix + instance
					export := target.GetContainer("filter").GetContainer("export")
					export.SetSlice("filters", append(export.GetMultiValues("filters"), spelling))
					_, _, err := peersAndDynamicGroups(tree.Clone())
					if err == nil {
						t.Fatal("active internal export reference was accepted")
					}
					var command string
					for _, line := range strings.Split(err.Error(), "\n") {
						if strings.HasPrefix(strings.TrimSpace(line), "deactivate ") {
							if command != "" {
								t.Fatalf("ambiguous remediation commands: %v", err)
							}
							command = strings.TrimSpace(line)
						}
					}
					if command == "" {
						t.Fatalf("refusal has no executable deactivation command: %v", err)
					}
					tokens, parseErr := config.ParseEditPath(command)
					if parseErr != nil {
						t.Fatal(parseErr)
					}
					want := append([]string{"deactivate"}, path...)
					want = append(want, "filter", "export", "filters", spelling)
					if !slices.Equal(tokens, want) {
						t.Fatalf("remediation tokens = %q, want original location and spelling %q", tokens, want)
					}
					schema, schemaErr := config.YANGSchema()
					if schemaErr != nil {
						t.Fatal(schemaErr)
					}
					editor, editorErr := cli.NewEditorFromContent([]byte(config.Serialize(tree, schema)), "-")
					if editorErr != nil {
						t.Fatal(editorErr)
					}
					t.Cleanup(func() {
						if closeErr := editor.Close(); closeErr != nil {
							t.Error(closeErr)
						}
					})
					if _, applyErr := editor.ApplyActivation(tokens[1:], false); applyErr != nil {
						t.Fatalf("suggested command does not execute: %v", applyErr)
					}
					after := loopExportRemedySettings(t, editor.Tree().Clone())
					if len(after) != len(before) {
						t.Fatalf("remediation changed population: %d -> %d", len(before), len(after))
					}
					inactive := 0
					for i, settings := range after {
						policy := settings.ExportPolicy
						var external []filterapi.FilterRef
						for _, ref := range policy.Filters() {
							if !ref.InProcess() {
								external = append(external, ref)
								continue
							}
							if ref.Name != "loop:"+instance || !ref.Inactive {
								t.Fatalf("remediation lost inactive reference: %+v", ref)
							}
							inactive++
						}
						if !reflect.DeepEqual(policy.WithFilters(external), before[i].ExportPolicy) {
							t.Fatalf("remediation changed unrelated export refs or final: %+v -> %+v", before[i].ExportPolicy, policy)
						}
						if !reflect.DeepEqual(settings.ImportPolicy, before[i].ImportPolicy) {
							t.Fatal("export remediation changed the import policy")
						}
					}
					if inactive == 0 {
						t.Fatal("remediation removed rather than deactivated the reference")
					}
				})
			}
		}
	}
}

// TestLoopPolicyExportInactiveAncestors proves source-layer validation runs on
// the active tree: a disabled member, leaf, direction, filter, peer or group
// cannot reject the unrelated live peers or dynamic template.
func TestLoopPolicyExportInactiveAncestors(t *testing.T) {
	for _, ancestor := range []string{"member", "filters", "export", "filter", "peer", "group"} {
		t.Run(ancestor, func(t *testing.T) {
			tree, target, path, instance := loopExportRemedyConfig(t, "grouped-peer", false)
			export := target.GetContainer("filter").GetContainer("export")
			export.SetSlice("filters", []string{instance})
			fullPath := append(slices.Clone(path), "filter", "export", "filters", instance)
			switch ancestor {
			case "filters":
				fullPath = fullPath[:len(fullPath)-1]
			case "export":
				fullPath = fullPath[:len(fullPath)-2]
			case "filter":
				fullPath = fullPath[:len(fullPath)-3]
			case "peer":
				fullPath = path
			case "group":
				fullPath = path[:3]
			case "member":
				// The complete path targets only the written member.
			default:
				t.Fatal("unknown ancestor")
			}
			schema, err := config.YANGSchema()
			if err != nil {
				t.Fatal(err)
			}
			editor, err := cli.NewEditorFromContent([]byte(config.Serialize(tree, schema)), "-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := editor.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := editor.ApplyActivation(fullPath, false); err != nil {
				t.Fatal(err)
			}
			settings := loopExportRemedySettings(t, editor.Tree().Clone())
			if len(settings) < 2 {
				t.Fatalf("unrelated static/dynamic population lost: %d", len(settings))
			}
		})
	}
}

// loopExportRemedyConfig builds distinct inherited policies so a correction at
// the wrong layer cannot accidentally pass by removing an equivalent policy.
func loopExportRemedyConfig(t *testing.T, scope string, quoted bool) (*config.Tree, *config.Tree, []string, string) {
	t.Helper()
	groupName, peerName, dynamicName, instance := "static-twin", "member", "ix", "LOOPS"
	if quoted {
		instance = `LOOPS west\"edge`
	}
	content := strings.NewReplacer(
		"group static-twin", "group "+config.FormatEditPath([]string{groupName}),
		"peer member", "peer "+config.FormatEditPath([]string{peerName}),
		"group ix", "group "+config.FormatEditPath([]string{dynamicName}),
		"loop-detection LOOPS", "loop-detection "+config.FormatEditPath([]string{instance}),
	).Replace(parityConfig)
	tree, err := config.ParseTreeWithYANG(content, nil)
	if err != nil {
		t.Fatal(err)
	}
	bgp := tree.GetContainer("bgp")
	group := bgp.GetList("group")[groupName]
	groupedPeer := group.GetList("peer")[peerName]
	dynamic := bgp.GetList("group")[dynamicName]
	peer := buildMinimalPeer("192.0.2.2", "65001", "auto")
	bgp.AddListEntry("peer", "standalone-"+peerName, peer)
	for i, layer := range []*config.Tree{bgp, group, groupedPeer, peer, dynamic} {
		export := layer.GetOrCreateContainer("filter").GetOrCreateContainer("export")
		export.SetSlice("filters", []string{"external:keep"})
		export.Set("final", "accept")
		if i%2 == 0 {
			export.Set("final", "reject")
		}
	}
	switch scope {
	case "root":
		return tree, bgp, []string{"bgp"}, instance
	case "group":
		return tree, group, []string{"bgp", "group", groupName}, instance
	case "grouped-peer":
		return tree, groupedPeer, []string{"bgp", "group", groupName, "peer", peerName}, instance
	case "peer":
		return tree, peer, []string{"bgp", "peer", "standalone-" + peerName}, instance
	case "dynamic":
		return tree, dynamic, []string{"bgp", "group", dynamicName}, instance
	default:
		t.Fatalf("unknown scope %q", scope)
		return nil, nil, nil, ""
	}
}

func loopExportRemedySettings(t *testing.T, tree *config.Tree) []*reactor.PeerSettings {
	t.Helper()
	peers, groups, err := peersAndDynamicGroups(tree)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		peers = append(peers, group.Settings)
	}
	return peers
}

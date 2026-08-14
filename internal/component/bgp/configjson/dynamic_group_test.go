package configjson

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callerDisposition records how one non-test ForEachPeer caller treats the
// template visit a dynamic group produces. keyedBy names the configjson helper
// the caller must use to key that visit; an empty keyedBy means the caller
// deliberately keys nothing, and abstains says why.
type callerDisposition struct {
	keyedBy  string
	abstains string
}

// forEachPeerCallers is the accounting the spec's "Every caller handled" review
// row asserts, moved out of review and into a gate. Every non-test caller of
// ForEachPeer appears here exactly once.
//
// The defect this whole package change exists to fix was silent: a dynamic group
// reached no plugin and the operator got no error. A twelfth caller that ignored
// PeerOrigin would restore that silence for its own plugin, and nothing else in
// the tree would say so.
var forEachPeerCallers = map[string]callerDisposition{
	"internal/component/bgp/plugins/role/config.go":                       {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/softver/softver.go":                   {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/gr/gr.go":                             {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/gr/gr_llgr.go":                        {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/llnh/llnh.go":                         {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/hostname/hostname.go":                 {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/filter_community/filter_community.go": {keyedBy: "CapabilitySelector"},
	"internal/component/bgp/plugins/rpki/rpki_config.go":                  {keyedBy: "KeyFor"},
	"internal/component/bgp/blackholecfg/blackholecfg.go":                 {keyedBy: "GroupKey"},
	"internal/component/bgp/plugins/filter_irr/config.go": {abstains: "IRR resolution keys on the peer's remote ASN, " +
		"which parsePeerIRR reads from session.asn.remote. A listen-range group states none: its members declare " +
		"theirs in the OPEN, after the config is parsed. The template carries nothing this plugin could key."},
	"internal/component/bgp/plugins/filter_family/config.go": {abstains: "this visitor validates and stores nothing, " +
		"so it keys no template. The template visit still carries the group's map, so a dynamic group's export " +
		"chain is now validated where a group with no peer list produced no visit at all."},
}

// repoRootFromPackage walks up from this package to the ze checkout root.
func repoRootFromPackage(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "go.mod"), "walked to the wrong root")
	return root
}

// TestEveryForEachPeerCallerIsAccountedFor is R-1's mitigation: the risk that the
// visitor change touches many callers and one that ignores the new template visit
// keeps the defect silently.
//
// VALIDATES: the set of non-test ForEachPeer call sites in the tree equals the set
// recorded above, and every caller that claims to key the template names the
// configjson helper that produces its key.
// PREVENTS: a new caller silently dropping a dynamic group's config. Adding one
// fails this test until its author records which key the template lands under, or
// writes down why the plugin needs none.
func TestEveryForEachPeerCallerIsAccountedFor(t *testing.T) {
	root := repoRootFromPackage(t)

	var found []string
	for _, tree := range []string{"internal", "cmd", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if !strings.Contains(string(src), "configjson.ForEachPeer(") {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			found = append(found, filepath.ToSlash(rel))
			return nil
		})
		require.NoError(t, err)
	}
	sort.Strings(found)

	recorded := make([]string, 0, len(forEachPeerCallers))
	for file := range forEachPeerCallers {
		recorded = append(recorded, file)
	}
	sort.Strings(recorded)

	require.Equal(t, recorded, found,
		"a ForEachPeer caller appeared or vanished: record how it treats a dynamic group's "+
			"template visit in forEachPeerCallers, or remove its row")

	for file, disposition := range forEachPeerCallers {
		src, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err)

		if disposition.keyedBy == "" {
			assert.NotEmpty(t, disposition.abstains,
				"%s keys no template, so it must say why", file)
			continue
		}
		assert.Contains(t, string(src), "configjson."+disposition.keyedBy,
			"%s is recorded as keying the template with %s, and no longer calls it",
			file, disposition.keyedBy)
	}
}

// indexByKey walks one bgp subtree the way every ForEachPeer caller does: it
// stores each visit's effective config under the key KeyFor returns, so a runtime
// reader finds it from an identity it holds. leaf names the plugin container the
// caller reads (role, rpki, blackhole), and a peer's own statement wins over its
// group's, which is the bgp -> group -> peer order PeerRemoteIP already applies.
//
// It is deliberately the whole of what a caller does with a visit. The tests below
// are about the delivery layer, so they assert what ForEachPeer, KeyFor and
// LookupPeerConfig produce together, never what a plugin later decides with it.
func indexByKey(t *testing.T, tree map[string]any, leaf string) map[PeerConfigKey]map[string]any {
	t.Helper()
	index := make(map[PeerConfigKey]map[string]any)
	ForEachPeer(tree, func(name string, peerMap, groupMap map[string]any, origin PeerOrigin) {
		key, ok := KeyFor(name, peerMap, groupMap, origin)
		if !ok {
			return
		}
		for _, m := range []map[string]any{peerMap, groupMap} {
			if cfg, stated := m[leaf].(map[string]any); stated {
				index[key] = cfg
				return
			}
		}
	})
	return index
}

// TestDynamicGroupPluginConfigMatchesAStaticPeer verifies AC-8 at the delivery
// layer, which is the layer all 11 ForEachPeer callers share.
//
// VALIDATES: one config states the same plugin leaves on a static peer and on a
// dynamic group. Both are keyed reachably, and the config a member of the group
// resolves is the config the static peer resolves, field for field.
// PREVENTS: a dynamic group's template diverging from a static peer where the
// operator stated no difference. The member's own address and its generated name
// ("dyn-<addr>", reactor.buildDynamicPeerSettings) appear nowhere in the document,
// so the group name is the only identity that can answer for it. Before the
// template visit existed, nothing answered at all: the operator got no error and
// no enforcement.
func TestDynamicGroupPluginConfigMatchesAStaticPeer(t *testing.T) {
	tree := map[string]any{
		"peer": map[string]any{
			"transit-one": map[string]any{
				"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}},
				"role":       map[string]any{"import": "rs", "strict": true},
			},
		},
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{
					"ip":    "dynamic",
					"range": []any{"198.51.100.0/24"},
				}},
				"role": map[string]any{"import": "rs", "strict": true},
			},
		},
	}

	index := indexByKey(t, tree, "role")

	// The static peer, found from the address every filter chain passes
	// (filterapi.PeerFilterInfo.Address).
	static, ok := LookupPeerConfig(index, "192.0.2.1", "transit-one", "")
	require.True(t, ok, "a configured peer resolves from its remote address")

	// A member of the group, found from PeerFilterInfo.GroupName alone.
	member, ok := LookupPeerConfig(index, "198.51.100.7", "dyn-198.51.100.7", "ix")
	require.True(t, ok, "a member of a dynamic group resolves its group's config")

	assert.Equal(t, static, member,
		"the same statement on a group and on a peer must deliver the same config")
	assert.Equal(t, map[string]any{"import": "rs", "strict": true}, member,
		"the delivered config is what the operator stated, not an empty map two misses would also match")
}

// TestAPeerInsideADynamicGroupKeepsItsOwnConfig verifies AC-9 at the delivery
// layer.
//
// VALIDATES: a peer listed inside a dynamic group is keyed under its own address,
// so its own statement is what a reader finds, while the group's template keeps a
// separate key and still answers for the members built from it.
// PREVENTS: the template visit answering for a peer that stated something else.
// The two configurations are both in force, and an IXP that overrides one named
// member must keep that override.
func TestAPeerInsideADynamicGroupKeepsItsOwnConfig(t *testing.T) {
	tree := map[string]any{
		"group": map[string]any{
			"ix": map[string]any{
				"connection": map[string]any{"remote": map[string]any{
					"ip":    "dynamic",
					"range": []any{"198.51.100.0/24"},
				}},
				"role": map[string]any{"import": "rs"},
				"peer": map[string]any{
					"well-known": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": "198.51.100.2"}},
						"role":       map[string]any{"import": "peer"},
					},
				},
			},
		},
	}

	index := indexByKey(t, tree, "role")
	require.Len(t, index, 2, "the template and the named peer are two configurations, both in force")

	named, ok := LookupPeerConfig(index, "198.51.100.2", "well-known", "ix")
	require.True(t, ok)
	assert.Equal(t, map[string]any{"import": "peer"}, named,
		"a peer's own statement still beats its group's")

	member, ok := LookupPeerConfig(index, "198.51.100.9", "dyn-198.51.100.9", "ix")
	require.True(t, ok)
	assert.Equal(t, map[string]any{"import": "rs"}, member,
		"a peer built from the template still gets what the group states")
}

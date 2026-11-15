// Design: docs/architecture/api/process-protocol.md -- plugin state ownership.
package statestore

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// isolateGrants gives the test an empty grant registry and restores the one it
// found when the test ends, so a grant one test registers never reaches the
// next test.
func isolateGrants(t *testing.T) {
	t.Helper()
	grants.Lock()
	saved := grants.owners
	grants.owners = make(map[string][]stateGrant)
	grants.Unlock()
	t.Cleanup(func() {
		grants.Lock()
		grants.owners = saved
		grants.Unlock()
	})
}

// TestPluginKeyOwners proves the owner reader answers, for a concrete key,
// exactly the process names whose grants admit it.
//
// Method: each case registers its grants into an isolated registry, then asks
// for the owners of one key. The cases cover a literal grant, a key matched by
// a templated grant, a template parameter that would span two segments, a key
// two names are granted (registered out of order, answered sorted), and a key
// no grant admits, whose answer is empty. Each case also checks that
// PluginKeyWritable agrees with the answer for every registered name, since the
// two readers must judge a grant by the same rule.
func TestPluginKeyOwners(t *testing.T) {
	type grant struct {
		owner string
		keys  []zefs.KeyEntry
	}
	cases := []struct {
		name   string
		grants []grant
		key    string
		want   []string
	}{
		{
			name:   "literal grant",
			grants: []grant{{"owner-literal", []zefs.KeyEntry{zefs.KeyOSPFAuthBootCount}}},
			key:    "meta/ospf/auth/boot-count",
			want:   []string{"owner-literal"},
		},
		{
			name:   "templated grant",
			grants: []grant{{"owner-template", []zefs.KeyEntry{zefs.KeyBFDAuthSeq}}},
			key:    "meta/bfd/auth/session-1",
			want:   []string{"owner-template"},
		},
		{
			name:   "template parameter spans one segment only",
			grants: []grant{{"owner-template", []zefs.KeyEntry{zefs.KeyBFDAuthSeq}}},
			key:    "meta/bfd/auth/session-1/extra",
			want:   nil,
		},
		{
			name: "two names granted",
			grants: []grant{
				{"owner-b", []zefs.KeyEntry{zefs.KeyGRMarker}},
				{"owner-a", []zefs.KeyEntry{zefs.KeyGRMarker}},
				{"owner-other", []zefs.KeyEntry{zefs.KeyHistoryMax}},
			},
			key:  "meta/bgp/gr-marker",
			want: []string{"owner-a", "owner-b"},
		},
		{
			name:   "ungranted key",
			grants: []grant{{"owner-literal", []zefs.KeyEntry{zefs.KeyOSPFAuthBootCount}}},
			key:    "meta/history/max",
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateGrants(t)
			for _, g := range tc.grants {
				RegisterPluginKeys(g.owner, g.keys...)
			}
			got := PluginKeyOwners(tc.key)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("PluginKeyOwners(%q) = %q, want %q", tc.key, got, tc.want)
			}
			for _, g := range tc.grants {
				allowed := PluginKeyWritable(g.owner, tc.key)
				if allowed != slices.Contains(tc.want, g.owner) {
					t.Fatalf("PluginKeyWritable(%q, %q) = %v, disagrees with owners %q", g.owner, tc.key, allowed, got)
				}
			}
		})
	}
}

// TestPluginKeyOwnersAnswerIsACopy proves a caller that changes the answer
// changes nothing the next reader sees.
//
// Method: register two names for one key, overwrite the first answer's
// elements, and ask again.
func TestPluginKeyOwnersAnswerIsACopy(t *testing.T) {
	isolateGrants(t)
	RegisterPluginKeys("owner-a", zefs.KeyGRMarker)
	RegisterPluginKeys("owner-b", zefs.KeyGRMarker)
	first := PluginKeyOwners("meta/bgp/gr-marker")
	for i := range first {
		first[i] = "changed"
	}
	second := PluginKeyOwners("meta/bgp/gr-marker")
	if !slices.Equal(second, []string{"owner-a", "owner-b"}) {
		t.Fatalf("second answer = %q after the first was changed, want [owner-a owner-b]", second)
	}
}

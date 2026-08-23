// VALIDATES: RFC 7770 router-information floods at exactly the scopes the
// operator listed, whether they list one or several.
// PREVENTS: a wrong value rather than a missing one. Tree.ToMap collapses a
// one-member leaf-list to a bare string, and the parser asserted []any on
// `scope`, so an operator who listed exactly one scope got an EMPTY scope list.
// The enabled-with-no-scope default then substituted area AND AS, so the
// router advertised at a scope the operator had deliberately left out. A
// multi-scope test passes with that bug in place and proves nothing.

package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRouterInformationSingleScope(t *testing.T) {
	for _, tc := range []struct {
		name        string
		scope       any
		wantHave    []OpaqueScope
		wantNotHave []OpaqueScope
	}{
		{
			name:        "one scope, the bare string ToMap emits at count one",
			scope:       "area",
			wantHave:    []OpaqueScope{OpaqueScopeArea},
			wantNotHave: []OpaqueScope{OpaqueScopeAS, OpaqueScopeLink},
		},
		{
			name:        "one scope, link only",
			scope:       "link",
			wantHave:    []OpaqueScope{OpaqueScopeLink},
			wantNotHave: []OpaqueScope{OpaqueScopeArea, OpaqueScopeAS},
		},
		{
			name:        "two scopes, the array ToMap emits at count two",
			scope:       []any{"area", "as"},
			wantHave:    []OpaqueScope{OpaqueScopeArea, OpaqueScopeAS},
			wantNotHave: []OpaqueScope{OpaqueScopeLink},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultOSPFConfig()
			tree := map[string]any{
				"router-information": map[string]any{"enabled": "true", "scope": tc.scope},
			}
			require.NoError(t, applyTree(&cfg, tree))

			require.True(t, cfg.RouterInformation.Enabled)
			for _, s := range tc.wantHave {
				assert.True(t, cfg.RouterInformation.HasScope(s), "scope %v was configured and must be present", s)
			}
			for _, s := range tc.wantNotHave {
				assert.False(t, cfg.RouterInformation.HasScope(s), "scope %v was NOT configured and must not be added", s)
			}
		})
	}
}

// TestParseRouterInformationNoScopeKeepsTheDefault holds the RFC 7770 sec 2.7
// default in place: enabling the feature without naming a scope still floods at
// area and AS, which is what the common Segment-Routing deployment needs. It is
// the case the single-scope fix must not disturb.
func TestParseRouterInformationNoScopeKeepsTheDefault(t *testing.T) {
	cfg := defaultOSPFConfig()
	tree := map[string]any{"router-information": map[string]any{"enabled": "true"}}
	require.NoError(t, applyTree(&cfg, tree))

	assert.True(t, cfg.RouterInformation.HasScope(OpaqueScopeArea))
	assert.True(t, cfg.RouterInformation.HasScope(OpaqueScopeAS))
	assert.False(t, cfg.RouterInformation.HasScope(OpaqueScopeLink))
}

// TestConfigInstanceIDsReadsEveryProducerShape covers the NON-STRING leaf-list,
// the seam configvalue.LeafList cannot serve because it yields strings.
//
// VALIDATES: `instance-id` is read at one member and at several, on both the
// in-process and the JSON delivery path.
// PREVENTS: the divergence a second coercion always grows. The []string arm was
// missing, so the whole slice fell to the scalar branch and became one item no
// number parser could read: every configured instance-id was lost, not just one.
func TestConfigInstanceIDsReadsEveryProducerShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
		want []uint8
	}{
		{"one member, the bare scalar ToMap emits at count one", "5", []uint8{5}},
		{"several members in process", []string{"5", "7"}, []uint8{5, 7}},
		{"several members over JSON", []any{"5", "7"}, []uint8{5, 7}},
		{"absent leaf", nil, nil},
		{"duplicates collapse", []string{"5", "5"}, []uint8{5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := configInstanceIDs(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestConfigInstanceIDsRejectsOutOfRange keeps the RFC 6549 8-bit bound loud.
//
// VALIDATES: a value above 255 is an error, never a truncation.
// PREVENTS: instance 256 silently becoming instance 0, the base instance.
func TestConfigInstanceIDsRejectsOutOfRange(t *testing.T) {
	_, err := configInstanceIDs([]string{"5", "256"})
	require.Error(t, err)
}

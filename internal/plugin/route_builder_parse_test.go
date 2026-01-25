package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
)

// TestParseCommonAttributeBuilder verifies Builder-based attribute parsing.
//
// VALIDATES: parseCommonAttributeBuilder correctly populates Builder.
// PREVENTS: Regression during PathAttributes → Builder migration.
func TestParseCommonAttributeBuilder(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, b *attribute.Builder)
	}{
		{
			name: "origin_igp",
			args: []string{"origin", "igp"},
			check: func(t *testing.T, b *attribute.Builder) {
				wire := b.Build()
				assert.Equal(t, byte(0), wire[3]) // IGP
			},
		},
		{
			name: "origin_egp",
			args: []string{"origin", "egp"},
			check: func(t *testing.T, b *attribute.Builder) {
				wire := b.Build()
				assert.Equal(t, byte(1), wire[3]) // EGP
			},
		},
		{
			name: "local_preference",
			args: []string{"local-preference", "200"},
			check: func(t *testing.T, b *attribute.Builder) {
				attrs := b.ToAttributes()
				for _, a := range attrs {
					if lp, ok := a.(attribute.LocalPref); ok {
						assert.Equal(t, attribute.LocalPref(200), lp)
						return
					}
				}
				t.Fatal("LOCAL_PREF not found")
			},
		},
		{
			name: "med",
			args: []string{"med", "100"},
			check: func(t *testing.T, b *attribute.Builder) {
				attrs := b.ToAttributes()
				for _, a := range attrs {
					if med, ok := a.(attribute.MED); ok {
						assert.Equal(t, attribute.MED(100), med)
						return
					}
				}
				t.Fatal("MED not found")
			},
		},
		{
			name: "as_path_bracketed",
			args: []string{"as-path", "[65001", "65002]"},
			check: func(t *testing.T, b *attribute.Builder) {
				asPath := b.ASPathSlice()
				assert.Equal(t, []uint32{65001, 65002}, asPath)
			},
		},
		{
			name: "community_single",
			args: []string{"community", "65000:100"},
			check: func(t *testing.T, b *attribute.Builder) {
				attrs := b.ToAttributes()
				for _, a := range attrs {
					if comms, ok := a.(attribute.Communities); ok {
						require.Len(t, comms, 1)
						assert.Equal(t, attribute.Community(0xFDE80064), comms[0])
						return
					}
				}
				t.Fatal("COMMUNITY not found")
			},
		},
		{
			name: "community_bracketed",
			args: []string{"community", "[65000:100", "65000:200]"},
			check: func(t *testing.T, b *attribute.Builder) {
				attrs := b.ToAttributes()
				for _, a := range attrs {
					if comms, ok := a.(attribute.Communities); ok {
						require.Len(t, comms, 2)
						return
					}
				}
				t.Fatal("COMMUNITY not found")
			},
		},
		{
			name: "large_community",
			args: []string{"large-community", "65000:1:2"},
			check: func(t *testing.T, b *attribute.Builder) {
				attrs := b.ToAttributes()
				for _, a := range attrs {
					if lc, ok := a.(attribute.LargeCommunities); ok {
						require.Len(t, lc, 1)
						assert.Equal(t, uint32(65000), lc[0].GlobalAdmin)
						return
					}
				}
				t.Fatal("LARGE_COMMUNITY not found")
			},
		},
		{
			name: "extended_community",
			args: []string{"extended-community", "target:65000:100"},
			check: func(t *testing.T, b *attribute.Builder) {
				attrs := b.ToAttributes()
				for _, a := range attrs {
					if ec, ok := a.(attribute.ExtendedCommunities); ok {
						require.Len(t, ec, 1)
						return
					}
				}
				t.Fatal("EXTENDED_COMMUNITY not found")
			},
		},
		{
			name:    "missing_origin_value",
			args:    []string{"origin"},
			wantErr: true,
		},
		{
			name:    "invalid_origin",
			args:    []string{"origin", "invalid"},
			wantErr: true,
		},
		{
			name: "unknown_keyword",
			args: []string{"unknown", "value"},
			check: func(t *testing.T, b *attribute.Builder) {
				// Should return 0 consumed, no error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := attribute.NewBuilder()
			consumed, err := parseCommonAttributeBuilder(tt.args[0], tt.args, 0, b)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, b)
			}

			// For unknown keywords, consumed should be 0
			if tt.args[0] == "unknown" {
				assert.Equal(t, 0, consumed)
			}
		})
	}
}

// TestParseASPathEmptyString verifies empty AS_PATH behavior.
//
// VALIDATES: Empty string doesn't modify Builder state.
// PREVENTS: Regression from old PathAttributes behavior.
func TestParseASPathEmptyString(t *testing.T) {
	b := attribute.NewBuilder()
	b.SetASPath([]uint32{65001}) // Set initial value

	err := b.ParseASPath("")
	require.NoError(t, err)

	// Empty string should not modify existing AS_PATH
	asPath := b.ASPathSlice()
	assert.Equal(t, []uint32{65001}, asPath)
}

// TestParseCommunityAppendBehavior verifies community append semantics.
//
// VALIDATES: ParseCommunity appends, not replaces.
// PREVENTS: Unexpected behavior when parsing multiple communities.
func TestParseCommunityAppendBehavior(t *testing.T) {
	b := attribute.NewBuilder()

	err := b.ParseCommunity("65000:100")
	require.NoError(t, err)

	err = b.ParseCommunity("65000:200")
	require.NoError(t, err)

	attrs := b.ToAttributes()
	for _, a := range attrs {
		if comms, ok := a.(attribute.Communities); ok {
			assert.Len(t, comms, 2, "Should have both communities")
			return
		}
	}
	t.Fatal("COMMUNITY not found")
}

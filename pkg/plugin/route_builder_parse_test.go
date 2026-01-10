package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
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

// TestParseCommonAttributeBuilderEquivalence verifies Builder produces same attributes as PathAttributes.
//
// VALIDATES: Builder path produces equivalent results to PathAttributes path.
// PREVENTS: Behavioral differences during migration.
func TestParseCommonAttributeBuilderEquivalence(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "origin", args: []string{"origin", "igp"}},
		{name: "med", args: []string{"med", "100"}},
		{name: "local_pref", args: []string{"local-preference", "200"}},
		{name: "community", args: []string{"community", "65000:100"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse with PathAttributes (old way)
			var pa PathAttributes
			_, errOld := parseCommonAttribute(tt.args[0], tt.args, 0, &pa)
			require.NoError(t, errOld)

			// Parse with Builder (new way)
			b := attribute.NewBuilder()
			_, errNew := parseCommonAttributeBuilder(tt.args[0], tt.args, 0, b)
			require.NoError(t, errNew)

			// Compare results via ToAttributes
			builderAttrs := b.ToAttributes()

			// Check equivalence based on attribute type
			switch tt.args[0] {
			case "origin":
				require.NotNil(t, pa.Origin)
				// Builder produces ORIGIN in attrs
				for _, a := range builderAttrs {
					if o, ok := a.(attribute.Origin); ok {
						assert.Equal(t, *pa.Origin, uint8(o))
						return
					}
				}
				t.Fatal("ORIGIN not found in builder output")

			case "med":
				require.NotNil(t, pa.MED)
				for _, a := range builderAttrs {
					if m, ok := a.(attribute.MED); ok {
						assert.Equal(t, *pa.MED, uint32(m))
						return
					}
				}
				t.Fatal("MED not found in builder output")

			case "local-preference":
				require.NotNil(t, pa.LocalPreference)
				for _, a := range builderAttrs {
					if lp, ok := a.(attribute.LocalPref); ok {
						assert.Equal(t, *pa.LocalPreference, uint32(lp))
						return
					}
				}
				t.Fatal("LOCAL_PREF not found in builder output")

			case "community":
				require.NotEmpty(t, pa.Communities)
				for _, a := range builderAttrs {
					if comms, ok := a.(attribute.Communities); ok {
						require.Len(t, comms, len(pa.Communities))
						for i, c := range pa.Communities {
							assert.Equal(t, attribute.Community(c), comms[i])
						}
						return
					}
				}
				t.Fatal("COMMUNITY not found in builder output")
			}
		})
	}
}

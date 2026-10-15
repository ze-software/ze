package cos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearProfiles installs no profile, so a test leaves the registry empty.
func clearProfiles() { Replace(nil) }

func TestRegistryReplaceLookup(t *testing.T) {
	t.Cleanup(clearProfiles)

	p := Profile{
		IngressMap: map[uint32]uint32{0: 0, 6: 6, 7: 7},
		EgressMap:  map[uint32]uint32{0: 0, 6: 6, 7: 7},
	}
	Replace(map[string]Profile{"residential": p})

	got, ok := Lookup("residential")
	require.True(t, ok)
	assert.Equal(t, p.IngressMap, got.IngressMap)
	assert.Equal(t, p.EgressMap, got.EgressMap)
}

func TestRegistryLookupMissing(t *testing.T) {
	t.Cleanup(clearProfiles)

	_, ok := Lookup("nonexistent")
	assert.False(t, ok)
}

// TestRegistryReplaceDropsOmitted proves Replace is a whole-set replace: a
// profile the new set omits is gone, and the new set's profiles are present.
func TestRegistryReplaceDropsOmitted(t *testing.T) {
	t.Cleanup(clearProfiles)

	Replace(map[string]Profile{"stale": {IngressMap: map[uint32]uint32{1: 1}}})
	Replace(map[string]Profile{"fresh": {EgressMap: map[uint32]uint32{2: 2}}})

	_, ok := Lookup("stale")
	assert.False(t, ok)
	_, ok = Lookup("fresh")
	assert.True(t, ok)
}

func TestAll(t *testing.T) {
	t.Cleanup(clearProfiles)

	Replace(map[string]Profile{
		"a": {IngressMap: map[uint32]uint32{0: 0}},
		"b": {EgressMap: map[uint32]uint32{6: 6}},
	})

	all := All()
	assert.Len(t, all, 2)
	assert.Equal(t, map[uint32]uint32{0: 0}, all["a"].IngressMap)
	assert.Equal(t, map[uint32]uint32{6: 6}, all["b"].EgressMap)
}

func TestParseFilterID(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantOK   bool
	}{
		{"cos:residential", "residential", true},
		{"cos:business", "business", true},
		{"10mbit", "", false},
		{"", "", false},
		{"cos:", "", false},
	}
	for _, tt := range tests {
		name, ok := ParseFilterID(tt.input)
		assert.Equal(t, tt.wantOK, ok, "input=%q", tt.input)
		assert.Equal(t, tt.wantName, name, "input=%q", tt.input)
	}
}

func TestResolveNoResolver(t *testing.T) {
	t.Cleanup(ClearResolver)

	ingress, egress, err := Resolve(nil, "residential", "", false)
	assert.NoError(t, err)
	assert.Nil(t, ingress)
	assert.Nil(t, egress)
}

// TestResolveWithResolver proves Resolve hands the resolver the profile set
// it was given, not the installed registry, which here is empty.
func TestResolveWithResolver(t *testing.T) {
	t.Cleanup(func() { clearProfiles(); ClearResolver() })

	set := map[string]Profile{"res": {
		IngressMap: map[uint32]uint32{6: 6},
		EgressMap:  map[uint32]uint32{6: 6},
	}}
	RegisterResolver(func(given map[string]Profile, parentCoS, unitCoS string, _ bool) (map[uint32]uint32, map[uint32]uint32, error) {
		name := unitCoS
		if name == "" {
			name = parentCoS
		}
		p, ok := given[name]
		if !ok {
			return nil, nil, nil
		}
		return p.IngressMap, p.EgressMap, nil
	})

	ingress, egress, err := Resolve(set, "res", "", false)
	require.NoError(t, err)
	assert.Equal(t, map[uint32]uint32{6: 6}, ingress)
	assert.Equal(t, map[uint32]uint32{6: 6}, egress)
}

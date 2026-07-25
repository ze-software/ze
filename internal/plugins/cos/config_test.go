package cos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCos "github.com/ze-software/ze/internal/core/cos"
)

// VALIDATES: AC-1 -- valid profile with ingress + egress maps parses and registers.
// PREVENTS: regression in profile parsing or registry wiring.
func TestCoSProfileParse(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	data := `{"class-of-service":{"ieee-802.1p":{"residential":{
		"ingress":{"pcp":{"0":{"priority":"0"},"5":{"priority":"5"},"6":{"priority":"6"}}},
		"egress":{"priority":{"0":{"pcp":"0"},"5":{"pcp":"5"},"6":{"pcp":"6"}}}
	}}}}`
	err := parseAndRegisterProfiles(data)
	require.NoError(t, err)

	p, ok := coreCos.Lookup("residential")
	require.True(t, ok)
	assert.Equal(t, map[uint32]uint32{0: 0, 5: 5, 6: 6}, p.IngressMap)
	assert.Equal(t, map[uint32]uint32{0: 0, 5: 5, 6: 6}, p.EgressMap)
}

// VALIDATES: AC-2 -- out-of-range PCP/priority values rejected.
// PREVENTS: invalid 802.1p values (>7) passing through validation.
func TestCoSProfileParseInvalid(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	tests := []struct {
		name string
		data string
	}{
		{
			name: "ingress pcp 8",
			data: `{"class-of-service":{"ieee-802.1p":{"bad":{"ingress":{"pcp":{"8":{"priority":"0"}}}}}}}`,
		},
		{
			name: "ingress priority 8",
			data: `{"class-of-service":{"ieee-802.1p":{"bad":{"ingress":{"pcp":{"0":{"priority":"8"}}}}}}}`,
		},
		{
			name: "egress priority 8",
			data: `{"class-of-service":{"ieee-802.1p":{"bad":{"egress":{"priority":{"8":{"pcp":"0"}}}}}}}`,
		},
		{
			name: "egress pcp 8",
			data: `{"class-of-service":{"ieee-802.1p":{"bad":{"egress":{"priority":{"0":{"pcp":"8"}}}}}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parseAndRegisterProfiles(tt.data)
			assert.Error(t, err)
		})
	}
}

// VALIDATES: empty profile (no ingress/egress) parses as empty maps.
// PREVENTS: nil map panic when applying an empty profile.
func TestCoSProfileParseEmpty(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	data := `{"class-of-service":{"ieee-802.1p":{"empty":{}}}}`
	err := parseAndRegisterProfiles(data)
	require.NoError(t, err)

	p, ok := coreCos.Lookup("empty")
	require.True(t, ok)
	assert.Empty(t, p.IngressMap)
	assert.Empty(t, p.EgressMap)
}

// VALIDATES: resolver handles inheritance, "none", mutual exclusion, and lookup.
// PREVENTS: resolution logic broken when moved from iface to cos plugin.
func TestResolveCoSForUnit(t *testing.T) {
	t.Cleanup(coreCos.Clear)
	coreCos.Register("res", coreCos.Profile{
		IngressMap: map[uint32]uint32{6: 6},
		EgressMap:  map[uint32]uint32{6: 6},
	})

	in, eg, err := resolveCoSForUnit("res", "", false)
	require.NoError(t, err)
	assert.Equal(t, map[uint32]uint32{6: 6}, in)
	assert.Equal(t, map[uint32]uint32{6: 6}, eg)

	in, eg, err = resolveCoSForUnit("res", "none", false)
	require.NoError(t, err)
	assert.Nil(t, in)
	assert.Nil(t, eg)

	in, eg, err = resolveCoSForUnit("", "", false)
	require.NoError(t, err)
	assert.Nil(t, in)
	assert.Nil(t, eg)

	_, _, err = resolveCoSForUnit("res", "", true)
	assert.ErrorContains(t, err, "mutually exclusive")

	_, _, err = resolveCoSForUnit("missing", "", false)
	assert.ErrorContains(t, err, "not found")
}

// VALIDATES: boundary values 0 and 7 are both accepted.
// PREVENTS: off-by-one in range validation.
func TestCoSProfileParseBoundary(t *testing.T) {
	t.Cleanup(coreCos.Clear)

	data := `{"class-of-service":{"ieee-802.1p":{"boundary":{
		"ingress":{"pcp":{"0":{"priority":"7"},"7":{"priority":"0"}}},
		"egress":{"priority":{"0":{"pcp":"7"},"7":{"pcp":"0"}}}
	}}}}`
	err := parseAndRegisterProfiles(data)
	require.NoError(t, err)

	p, ok := coreCos.Lookup("boundary")
	require.True(t, ok)
	assert.Equal(t, map[uint32]uint32{0: 7, 7: 0}, p.IngressMap)
	assert.Equal(t, map[uint32]uint32{0: 7, 7: 0}, p.EgressMap)
}

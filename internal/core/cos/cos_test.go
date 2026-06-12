package cos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryRegisterLookup(t *testing.T) {
	t.Cleanup(Clear)

	p := Profile{
		IngressMap: map[uint32]uint32{0: 0, 6: 6, 7: 7},
		EgressMap:  map[uint32]uint32{0: 0, 6: 6, 7: 7},
	}
	Register("residential", p)

	got, ok := Lookup("residential")
	require.True(t, ok)
	assert.Equal(t, p.IngressMap, got.IngressMap)
	assert.Equal(t, p.EgressMap, got.EgressMap)
}

func TestRegistryLookupMissing(t *testing.T) {
	t.Cleanup(Clear)

	_, ok := Lookup("nonexistent")
	assert.False(t, ok)
}

func TestRegistryClear(t *testing.T) {
	t.Cleanup(Clear)

	Register("test", Profile{IngressMap: map[uint32]uint32{1: 1}})
	Clear()

	_, ok := Lookup("test")
	assert.False(t, ok)
}

func TestResolveNoResolver(t *testing.T) {
	t.Cleanup(ClearResolver)

	ingress, egress, err := Resolve("residential", "", false)
	assert.NoError(t, err)
	assert.Nil(t, ingress)
	assert.Nil(t, egress)
}

func TestResolveWithResolver(t *testing.T) {
	t.Cleanup(func() { Clear(); ClearResolver() })

	Register("res", Profile{
		IngressMap: map[uint32]uint32{6: 6},
		EgressMap:  map[uint32]uint32{6: 6},
	})
	RegisterResolver(func(parentCoS, unitCoS string, _ bool) (map[uint32]uint32, map[uint32]uint32, error) {
		name := unitCoS
		if name == "" {
			name = parentCoS
		}
		p, ok := Lookup(name)
		if !ok {
			return nil, nil, nil
		}
		return p.IngressMap, p.EgressMap, nil
	})

	ingress, egress, err := Resolve("res", "", false)
	require.NoError(t, err)
	assert.Equal(t, map[uint32]uint32{6: 6}, ingress)
	assert.Equal(t, map[uint32]uint32{6: 6}, egress)
}

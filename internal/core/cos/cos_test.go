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

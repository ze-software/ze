// Design: (none -- new component, predates documentation)

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// TestDecoratorRegistry verifies that decorators can be registered and looked up by name.
// VALIDATES: AC-8 -- multiple decorators registered, each leaf uses its own based on extension argument.
// PREVENTS: Decorator lookup fails silently or returns wrong decorator.
func TestDecoratorRegistry(t *testing.T) {
	reg := NewDecoratorRegistry()

	called := false
	d := DecoratorFunc("test-decorator", func(value string) (string, error) {
		called = true
		return "decorated:" + value, nil
	})
	reg.Register(d)

	// Lookup by name.
	got := reg.Get("test-decorator")
	require.NotNil(t, got, "registered decorator should be found")
	assert.Equal(t, "test-decorator", got.Name())

	result, err := got.Decorate("42")
	require.NoError(t, err)
	assert.Equal(t, "decorated:42", result)
	assert.True(t, called)

	// Multiple decorators.
	d2 := DecoratorFunc("other", func(value string) (string, error) {
		return "other:" + value, nil
	})
	reg.Register(d2)

	got2 := reg.Get("other")
	require.NotNil(t, got2)
	result2, err := got2.Decorate("x")
	require.NoError(t, err)
	assert.Equal(t, "other:x", result2)

	// Original still works.
	got1 := reg.Get("test-decorator")
	require.NotNil(t, got1)
}

// TestDecoratorUnknownName verifies that looking up an unregistered decorator returns nil.
// VALIDATES: AC-3 -- unknown decorator name is silent, no error.
// PREVENTS: Panic or error on missing decorator.
func TestDecoratorUnknownName(t *testing.T) {
	reg := NewDecoratorRegistry()

	got := reg.Get("nonexistent")
	assert.Nil(t, got, "unregistered decorator should return nil")
}

// TestBuildFieldMetaPropagatesDecorate verifies that buildFieldMeta copies
// LeafNode.Decorate into FieldMeta.DecoratorName.
// VALIDATES: AC-1 -- decorator name flows from YANG schema through to FieldMeta.
// PREVENTS: Decorator name silently dropped during field building.
func TestBuildFieldMetaPropagatesDecorate(t *testing.T) {
	leaf := &config.LeafNode{
		Type:     config.TypeUint32,
		Decorate: "asn-name",
	}
	meta := buildFieldMeta("as", leaf, "64500", false, "bgp/peer/upstream/remote")
	assert.Equal(t, "asn-name", meta.DecoratorName, "DecoratorName should be copied from leaf.Decorate")
	assert.Equal(t, "64500", meta.Value)
	assert.Equal(t, "as", meta.Leaf)

	// Leaf without decorator.
	leafPlain := &config.LeafNode{Type: config.TypeString}
	metaPlain := buildFieldMeta("name", leafPlain, "test", false, "bgp")
	assert.Empty(t, metaPlain.DecoratorName, "plain leaf should have empty DecoratorName")
}

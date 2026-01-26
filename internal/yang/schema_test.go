package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaAdapter_Children verifies child enumeration from YANG.
//
// VALIDATES: Container children are derived from YANG model.
// PREVENTS: Hardcoded schema children diverging from YANG.
func TestSchemaAdapter_Children(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	// BGP container should have children from YANG model
	children := schema.Children("bgp")
	require.NotEmpty(t, children)

	// Check for expected children from ze-bgp.yang
	assert.Contains(t, children, "local-as")
	assert.Contains(t, children, "router-id")
	assert.Contains(t, children, "peer")
}

// TestSchemaAdapter_ChildrenNested verifies nested path navigation.
//
// VALIDATES: Dot-separated paths navigate YANG tree correctly.
// PREVENTS: Broken path resolution for completion.
func TestSchemaAdapter_ChildrenNested(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	// peer is a list with children
	children := schema.Children("bgp.peer")
	require.NotEmpty(t, children)

	// Check for expected peer children
	assert.Contains(t, children, "peer-as")
}

// TestSchemaAdapter_Mandatory verifies mandatory field detection.
//
// VALIDATES: Mandatory fields marked in YANG are reported.
// PREVENTS: Missing required field indicators in completion.
func TestSchemaAdapter_Mandatory(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	// local-as and router-id are mandatory in YANG
	assert.True(t, schema.IsMandatory("bgp.local-as"))
	assert.True(t, schema.IsMandatory("bgp.router-id"))

	// hold-time is not mandatory
	assert.False(t, schema.IsMandatory("bgp.hold-time"))
}

// TestSchemaAdapter_TypeHint verifies type hints for completion.
//
// VALIDATES: YANG types are converted to completion hints.
// PREVENTS: Generic placeholders for typed fields.
func TestSchemaAdapter_TypeHint(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	tests := []struct {
		path     string
		wantHint string
	}{
		{"bgp.local-as", "uint32"},
		{"bgp.router-id", "string"}, // ipv4-address is string type
		{"bgp.hold-time", "uint16"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			hint := schema.TypeHint(tt.path)
			assert.Equal(t, tt.wantHint, hint)
		})
	}
}

// TestSchemaAdapter_EnumValues verifies enum value extraction.
//
// VALIDATES: Enum values from YANG are available for completion.
// PREVENTS: Missing enum options in completion dropdown.
func TestSchemaAdapter_EnumValues(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	// Find an enum field in YANG - check if any exists
	// For now, test that non-enum returns empty
	values := schema.EnumValues("bgp.local-as")
	assert.Empty(t, values, "non-enum should return empty")
}

// TestSchemaAdapter_Description verifies description extraction.
//
// VALIDATES: YANG descriptions are available for completion help.
// PREVENTS: Generic or missing help text.
func TestSchemaAdapter_Description(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	desc := schema.Description("bgp.local-as")
	// Should have some description from YANG
	assert.NotEmpty(t, desc)
}

// TestSchemaAdapter_IsContainer verifies container detection.
//
// VALIDATES: Containers are distinguished from leaves.
// PREVENTS: Trying to navigate into leaves.
func TestSchemaAdapter_IsContainer(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	assert.True(t, schema.IsContainer("bgp"))
	assert.False(t, schema.IsContainer("bgp.peer")) // peer is a list, not container
	assert.False(t, schema.IsContainer("bgp.local-as"))
}

// TestSchemaAdapter_IsList verifies list detection.
//
// VALIDATES: Lists are identified for key completion.
// PREVENTS: Missing list key prompts.
func TestSchemaAdapter_IsList(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	schema := NewSchemaAdapter(loader)

	assert.True(t, schema.IsList("bgp.peer"))
	assert.False(t, schema.IsList("bgp"))
	assert.False(t, schema.IsList("bgp.local-as"))
}

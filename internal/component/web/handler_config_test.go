package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// buildTestSchemaAndTree constructs a schema and tree resembling a BGP config
// for use across multiple tests. The schema has:
//
//	bgp {
//	  router-id (TypeIPv4, default "0.0.0.0")
//	  as (TypeUint32)
//	  peer (list, TypeString key) {
//	    remote-as (TypeUint32)
//	    enabled (TypeBool)
//	    description (TypeString)
//	    local { as (TypeUint32) }
//	    timer { hold-time (TypeUint16) }
//	  }
//	}
//
// The tree has:
//
//	bgp {
//	  router-id = "1.2.3.4"
//	  as = "65001"
//	  peer {
//	    "192.168.1.1" { remote-as = "65002", enabled = "true" }
//	    "10.0.0.1" { remote-as = "65003" }
//	  }
//	}
func buildTestSchemaAndTree() (*config.Schema, *config.Tree) {
	schema := config.NewSchema()

	peerList := config.List(config.TypeString,
		config.Field("remote-as", config.Leaf(config.TypeUint32)),
		config.Field("enabled", config.Leaf(config.TypeBool)),
		config.Field("description", config.Leaf(config.TypeString)),
		config.Field("local", config.Container(
			config.Field("as", config.Leaf(config.TypeUint32)),
		)),
		config.Field("timer", config.Container(
			config.Field("hold-time", config.Leaf(config.TypeUint16)),
		)),
	)

	bgpContainer := config.Container(
		config.Field("router-id", config.LeafWithDefault(config.TypeIPv4, "0.0.0.0")),
		config.Field("as", config.Leaf(config.TypeUint32)),
		config.Field("peer", peerList),
	)

	schema.Define("bgp", bgpContainer)

	// Build the tree.
	tree := config.NewTree()

	bgpTree := config.NewTree()
	bgpTree.Set("router-id", "1.2.3.4")
	bgpTree.Set("as", "65001")

	peer1 := config.NewTree()
	peer1.Set("remote-as", "65002")
	peer1.Set("enabled", "true")

	peer2 := config.NewTree()
	peer2.Set("remote-as", "65003")

	bgpTree.AddListEntry("peer", "192.168.1.1", peer1)
	bgpTree.AddListEntry("peer", "10.0.0.1", peer2)

	tree.SetContainer("bgp", bgpTree)

	return schema, tree
}

// TestNodeKindToTemplate verifies that each NodeKind maps to the correct
// template name for rendering.
// VALIDATES: AC-2 (container), AC-3 (list), AC-21 (flex), AC-22 (freeform), AC-23 (inline list).
// PREVENTS: Wrong template selected for a node kind.
func TestNodeKindToTemplate(t *testing.T) {
	tests := []struct {
		name     string
		kind     config.NodeKind
		wantTmpl string
	}{
		{
			name:     "container maps to container template",
			kind:     config.NodeContainer,
			wantTmpl: "container.html",
		},
		{
			name:     "list maps to list template",
			kind:     config.NodeList,
			wantTmpl: "list.html",
		},
		{
			name:     "leaf maps to leaf template",
			kind:     config.NodeLeaf,
			wantTmpl: "leaf.html",
		},
		{
			name:     "flex maps to flex template",
			kind:     config.NodeFlex,
			wantTmpl: "flex.html",
		},
		{
			name:     "freeform maps to freeform template",
			kind:     config.NodeFreeform,
			wantTmpl: "freeform.html",
		},
		{
			name:     "inline list maps to inline_list template",
			kind:     config.NodeInlineList,
			wantTmpl: "inline_list.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeKindToTemplate(tt.kind)
			assert.Equal(t, tt.wantTmpl, got)
		})
	}
}

// TestBuildBreadcrumbs verifies that URL path segments are converted into
// breadcrumb navigation segments with correct names and URLs.
// VALIDATES: AC-16 (breadcrumb shows path with clickable links).
// PREVENTS: Broken breadcrumb URLs or missing segments.
func TestBuildBreadcrumbs(t *testing.T) {
	segments := buildBreadcrumbs([]string{"bgp", "peer", "192.168.1.1"})

	require.Len(t, segments, 4, "root + 3 path segments")

	// Root segment.
	assert.Equal(t, "/", segments[0].Name)
	assert.Equal(t, "/show/", segments[0].URL)
	assert.False(t, segments[0].Active)

	// "bgp" segment.
	assert.Equal(t, "bgp", segments[1].Name)
	assert.Equal(t, "/show/bgp/", segments[1].URL)
	assert.False(t, segments[1].Active)

	// "peer" segment.
	assert.Equal(t, "peer", segments[2].Name)
	assert.Equal(t, "/show/bgp/peer/", segments[2].URL)
	assert.False(t, segments[2].Active)

	// "192.168.1.1" segment (last = active).
	assert.Equal(t, "192.168.1.1", segments[3].Name)
	assert.Equal(t, "/show/bgp/peer/192.168.1.1/", segments[3].URL)
	assert.True(t, segments[3].Active)
}

// TestBreadcrumbRoot verifies that an empty path produces only a root segment
// with no back button (only one segment total).
// VALIDATES: AC-18 (back button behavior at root).
// PREVENTS: Panic on empty path, spurious back button at root level.
func TestBreadcrumbRoot(t *testing.T) {
	segments := buildBreadcrumbs(nil)

	require.Len(t, segments, 1, "root only")

	assert.Equal(t, "/", segments[0].Name)
	assert.Equal(t, "/show/", segments[0].URL)
	assert.True(t, segments[0].Active, "root is active when it is the only segment")
}

// TestSchemaWalkContainer verifies that walking a schema path to a container
// node returns a ContainerNode.
// VALIDATES: AC-2 (container view for valid path).
// PREVENTS: Schema walk returning wrong node type.
func TestSchemaWalkContainer(t *testing.T) {
	schema, tree := buildTestSchemaAndTree()

	node, _, err := walkConfigPath(schema, tree, []string{"bgp"})
	require.NoError(t, err)
	require.NotNil(t, node)

	assert.Equal(t, config.NodeContainer, node.Kind())
}

// TestSchemaWalkListKey verifies that walking through a list and a key value
// returns the list's child schema (inside the entry).
// VALIDATES: AC-4 (list entry view at peer/192.168.1.1).
// PREVENTS: List key not consuming the correct number of path segments.
func TestSchemaWalkListKey(t *testing.T) {
	schema, tree := buildTestSchemaAndTree()

	node, subtree, err := walkConfigPath(schema, tree, []string{"bgp", "peer", "192.168.1.1"})
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, subtree)

	// After consuming "peer" (list name) + "192.168.1.1" (key value),
	// the schema node should be the ListNode (we are inside an entry).
	assert.Equal(t, config.NodeList, node.Kind())

	// The subtree should have the peer's configured values.
	val, ok := subtree.Get("remote-as")
	assert.True(t, ok)
	assert.Equal(t, "65002", val)
}

// TestSchemaWalkInvalidPath verifies that walking a nonexistent schema path
// returns an error.
// VALIDATES: schema walk rejects unknown path elements.
// PREVENTS: Silent nil dereference on bad paths.
func TestSchemaWalkInvalidPath(t *testing.T) {
	schema, tree := buildTestSchemaAndTree()

	_, _, err := walkConfigPath(schema, tree, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestBuildConfigViewDataContainer verifies that building view data for a
// container path produces leaf fields and child links.
// VALIDATES: AC-2 (container view: leaves as form fields, sub-containers/lists as links).
// PREVENTS: Missing leaves or children in container view data.
func TestBuildConfigViewDataContainer(t *testing.T) {
	schema, tree := buildTestSchemaAndTree()

	data, err := buildConfigViewData(schema, tree, []string{"bgp"})
	require.NoError(t, err)

	// LeafFields should contain router-id and as.
	leafNames := make(map[string]bool)
	for _, lf := range data.LeafFields {
		leafNames[lf.Name] = true
	}
	assert.True(t, leafNames["router-id"], "router-id should be in leaf fields")
	assert.True(t, leafNames["as"], "as should be in leaf fields")

	// Children should contain peer (it is a list).
	childNames := make(map[string]bool)
	for _, ch := range data.Children {
		childNames[ch.Name] = true
	}
	assert.True(t, childNames["peer"], "peer should be in children")
}

// TestBuildConfigViewDataList verifies that building view data for a list path
// produces the list keys.
// VALIDATES: AC-3 (list view: left panel shows peer key names).
// PREVENTS: Missing or disordered list keys in view data.
func TestBuildConfigViewDataList(t *testing.T) {
	schema, tree := buildTestSchemaAndTree()

	data, err := buildConfigViewData(schema, tree, []string{"bgp", "peer"})
	require.NoError(t, err)

	assert.Contains(t, data.Keys, "192.168.1.1")
	assert.Contains(t, data.Keys, "10.0.0.1")
	assert.Len(t, data.Keys, 2)
}

// TestBuildConfigViewDataListEntry verifies that building view data for a
// specific list entry path produces the entry's leaf fields with values.
// VALIDATES: AC-4 (list entry view: right panel with peer's leaves).
// PREVENTS: Leaf values not populated from tree for selected entry.
func TestBuildConfigViewDataListEntry(t *testing.T) {
	schema, tree := buildTestSchemaAndTree()

	data, err := buildConfigViewData(schema, tree, []string{"bgp", "peer", "192.168.1.1"})
	require.NoError(t, err)

	// Find the remote-as leaf field and check its value.
	leafByName := make(map[string]LeafField)
	for _, lf := range data.LeafFields {
		leafByName[lf.Name] = lf
	}

	remoteAS, ok := leafByName["remote-as"]
	require.True(t, ok, "remote-as should be in leaf fields")
	assert.Equal(t, "65002", remoteAS.Value)
	assert.True(t, remoteAS.IsConfigured)

	enabled, ok := leafByName["enabled"]
	require.True(t, ok, "enabled should be in leaf fields")
	assert.Equal(t, "true", enabled.Value)
	assert.True(t, enabled.IsConfigured)
}

// TestLeafInputTypeMapping verifies that all 10 ValueTypes map to the correct
// HTML input type, min/max constraints, and pattern attributes.
// VALIDATES: AC-5 through AC-15 (input type mapping for all ValueTypes).
// PREVENTS: Wrong HTML input type or missing constraints for a ValueType.
func TestLeafInputTypeMapping(t *testing.T) {
	tests := []struct {
		name      string
		valueType config.ValueType
		wantInput string
		wantMin   string
		wantMax   string
		wantPat   string // pattern substring (empty means no pattern expected)
	}{
		{
			name:      "TypeString maps to text",
			valueType: config.TypeString,
			wantInput: "text",
		},
		{
			name:      "TypeBool maps to checkbox",
			valueType: config.TypeBool,
			wantInput: "checkbox",
		},
		{
			name:      "TypeUint16 maps to number with 0-65535 range",
			valueType: config.TypeUint16,
			wantInput: "number",
			wantMin:   "0",
			wantMax:   "65535",
		},
		{
			name:      "TypeUint32 maps to number with 0-4294967295 range",
			valueType: config.TypeUint32,
			wantInput: "number",
			wantMin:   "0",
			wantMax:   "4294967295",
		},
		{
			name:      "TypeIPv4 maps to text with dotted-quad pattern",
			valueType: config.TypeIPv4,
			wantInput: "text",
			wantPat:   ".", // pattern includes dots for IPv4
		},
		{
			name:      "TypeIPv6 maps to text with colon pattern",
			valueType: config.TypeIPv6,
			wantInput: "text",
			wantPat:   ":", // pattern includes colons for IPv6
		},
		{
			name:      "TypeIP maps to text",
			valueType: config.TypeIP,
			wantInput: "text",
		},
		{
			name:      "TypePrefix maps to text with CIDR pattern",
			valueType: config.TypePrefix,
			wantInput: "text",
			wantPat:   "/", // CIDR notation includes slash
		},
		{
			name:      "TypeDuration maps to text",
			valueType: config.TypeDuration,
			wantInput: "text",
		},
		{
			name:      "TypeInt maps to number (signed)",
			valueType: config.TypeInt,
			wantInput: "number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := leafInputType(tt.valueType)
			assert.Equal(t, tt.wantInput, info.InputType, "InputType")

			if tt.wantMin != "" {
				assert.Equal(t, tt.wantMin, info.Min, "Min")
			}
			if tt.wantMax != "" {
				assert.Equal(t, tt.wantMax, info.Max, "Max")
			}
			if tt.wantPat != "" {
				assert.Contains(t, info.Pattern, tt.wantPat, "Pattern")
			}
		})
	}
}

// TestDefaultValuePlaceholder verifies that an unconfigured leaf with a schema
// default value produces the correct placeholder and IsConfigured=false.
// VALIDATES: AC-19 (default value shown as placeholder, visually distinct).
// PREVENTS: Default values shown as configured, or missing entirely.
func TestDefaultValuePlaceholder(t *testing.T) {
	schema, _ := buildTestSchemaAndTree()

	// Build a tree where router-id is NOT configured, so the default should show.
	emptyBGP := config.NewTree()
	emptyBGP.Set("as", "65001")
	// Do NOT set router-id -- it has a default of "0.0.0.0" in the schema.

	emptyTree := config.NewTree()
	emptyTree.SetContainer("bgp", emptyBGP)

	data, err := buildConfigViewData(schema, emptyTree, []string{"bgp"})
	require.NoError(t, err)

	leafByName := make(map[string]LeafField)
	for _, lf := range data.LeafFields {
		leafByName[lf.Name] = lf
	}

	rid, ok := leafByName["router-id"]
	require.True(t, ok, "router-id should be in leaf fields")

	assert.False(t, rid.IsConfigured, "router-id should not be marked as configured")
	assert.Equal(t, "0.0.0.0", rid.Default, "default value should be 0.0.0.0")
}

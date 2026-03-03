package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetParserSimpleLeaf verifies parsing a simple set command.
//
// VALIDATES: Top-level leaves are set correctly.
//
// PREVENTS: Lost simple configuration values.
func TestSetParserSimpleLeaf(t *testing.T) {
	input := `set router-id 1.2.3.4`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)
	require.NotNil(t, tree)

	val, ok := tree.Get("router-id")
	require.True(t, ok)
	require.Equal(t, "1.2.3.4", val)
}

// TestSetParserNeighborLeaf verifies setting a neighbor field.
//
// VALIDATES: List entry fields are set via path.
//
// PREVENTS: Lost nested configuration.
func TestSetParserNeighborLeaf(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 router-id 1.2.3.4
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 1)

	n := neighbors["192.0.2.1"]
	require.NotNil(t, n)

	val, _ := n.Get("local-as")
	require.Equal(t, "65000", val)

	val, _ = n.Get("peer-as")
	require.Equal(t, "65001", val)

	val, _ = n.Get("router-id")
	require.Equal(t, "1.2.3.4", val)
}

// TestSetParserMultipleNeighbors verifies multiple list entries.
//
// VALIDATES: Multiple neighbors are created correctly.
//
// PREVENTS: Overwritten neighbor configs.
func TestSetParserMultipleNeighbors(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.2 local-as 65000
set neighbor 192.0.2.2 peer-as 65002
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	require.Len(t, neighbors, 2)

	n1 := neighbors["192.0.2.1"]
	val, _ := n1.Get("peer-as")
	require.Equal(t, "65001", val)

	n2 := neighbors["192.0.2.2"]
	val, _ = n2.Get("peer-as")
	require.Equal(t, "65002", val)
}

// TestSetParserNestedContainer verifies nested container paths.
//
// VALIDATES: Nested containers are created via path.
//
// PREVENTS: Flat structure instead of nested.
func TestSetParserNestedContainer(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 family ipv4 unicast true
set neighbor 192.0.2.1 family ipv6 unicast true
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	family := n.GetContainer("family")
	require.NotNil(t, family)

	ipv4 := family.GetContainer("ipv4")
	require.NotNil(t, ipv4)

	val, _ := ipv4.Get("unicast")
	require.Equal(t, "true", val)

	ipv6 := family.GetContainer("ipv6")
	require.NotNil(t, ipv6)

	val, _ = ipv6.Get("unicast")
	require.Equal(t, "true", val)
}

// TestSetParserNestedList verifies nested list paths.
//
// VALIDATES: Lists inside containers work.
//
// PREVENTS: Lost nested list entries.
func TestSetParserNestedList(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
set neighbor 192.0.2.1 static route 10.0.0.0/8 next-hop 192.0.2.1
set neighbor 192.0.2.1 static route 172.16.0.0/12 next-hop 192.0.2.1
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	static := n.GetContainer("static")
	require.NotNil(t, static)

	routes := static.GetList("route")
	require.Len(t, routes, 2)

	r1 := routes["10.0.0.0/8"]
	val, _ := r1.Get("next-hop")
	require.Equal(t, "192.0.2.1", val)
}

// TestSetParserProcess verifies string-keyed list.
//
// VALIDATES: String keys work for lists.
//
// PREVENTS: Only IP-keyed lists working.
func TestSetParserProcess(t *testing.T) {
	input := `
set process announce-routes run "/usr/bin/exabgp-announce"
set process announce-routes encoder json
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	procs := tree.GetList("process")
	require.Len(t, procs, 1)

	proc := procs["announce-routes"]
	require.NotNil(t, proc)

	val, _ := proc.Get("run")
	require.Equal(t, "/usr/bin/exabgp-announce", val)

	val, _ = proc.Get("encoder")
	require.Equal(t, "json", val)
}

// TestSetParserComments verifies comment handling.
//
// VALIDATES: Comments are ignored.
//
// PREVENTS: Comments parsed as commands.
func TestSetParserComments(t *testing.T) {
	input := `
# This is a comment
set router-id 1.2.3.4
# Another comment
set local-as 65000
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	val, _ := tree.Get("router-id")
	require.Equal(t, "1.2.3.4", val)

	val, _ = tree.Get("local-as")
	require.Equal(t, "65000", val)
}

// TestSetParser_NoValidateValue verifies SetParser accepts values without type checking.
// YANG validates types later in the pipeline — SetParser only does structural navigation.
//
// VALIDATES: SetParser accepts any string value for leaves (no own type checking).
// PREVENTS: SetParser rejecting values that YANG should validate.
func TestSetParser_NoValidateValue(t *testing.T) {
	input := `set neighbor 192.0.2.1 local-as not-a-number`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	// SetParser no longer calls ValidateValue — it accepts any value.
	// Type validation is deferred to YANG.
	require.NoError(t, err)

	entries := tree.GetList("neighbor")
	require.NotNil(t, entries)
	entry := entries["192.0.2.1"]
	require.NotNil(t, entry)
	val, ok := entry.Get("local-as")
	require.True(t, ok)
	assert.Equal(t, "not-a-number", val)
}

// TestSetParserUnknownPath verifies unknown path rejection.
//
// VALIDATES: Unknown paths are rejected.
//
// PREVENTS: Silent config typos.
func TestSetParserUnknownPath(t *testing.T) {
	input := `set neighbor 192.0.2.1 unknown-field value`

	p := NewSetParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

// TestSetParserQuotedValues verifies quoted string handling.
//
// VALIDATES: Quoted strings preserve spaces.
//
// PREVENTS: Broken paths or descriptions.
func TestSetParserQuotedValues(t *testing.T) {
	input := `set neighbor 192.0.2.1 description "My BGP Peer"`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	val, _ := n.Get("description")
	require.Equal(t, "My BGP Peer", val)
}

// TestSetParserLineNumbers verifies error line reporting.
//
// VALIDATES: Errors include line numbers.
//
// PREVENTS: Hard-to-find config errors.
func TestSetParserLineNumbers(t *testing.T) {
	input := `
set router-id 1.2.3.4
set neighbor 192.0.2.1 unknown-field value
`

	p := NewSetParser(testSchema())
	_, err := p.Parse(input)

	require.Error(t, err)
	require.Contains(t, err.Error(), "line 3")
}

// TestSetParserEmptyLines verifies empty line handling.
//
// VALIDATES: Empty lines are ignored.
//
// PREVENTS: Errors on blank lines.
func TestSetParserEmptyLines(t *testing.T) {
	input := `

set router-id 1.2.3.4

set local-as 65000

`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	val, _ := tree.Get("router-id")
	require.Equal(t, "1.2.3.4", val)
}

// TestSetParserDelete verifies delete command.
//
// VALIDATES: Delete removes values.
//
// PREVENTS: Inability to unset config.
func TestSetParserDelete(t *testing.T) {
	input := `
set neighbor 192.0.2.1 local-as 65000
set neighbor 192.0.2.1 peer-as 65001
delete neighbor 192.0.2.1 peer-as
`

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)

	require.NoError(t, err)

	neighbors := tree.GetList("neighbor")
	n := neighbors["192.0.2.1"]

	val, ok := n.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", val)

	_, ok = n.Get("peer-as")
	require.False(t, ok, "peer-as should be deleted")
}

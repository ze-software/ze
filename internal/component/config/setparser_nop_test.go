package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseNopLeaf verifies that "nop <path> <value>" sets the value and
// marks the leaf inactive in one step.
//
// VALIDATES: AC-1 -- nop bgp router-id 10.0.0.1 parsed -> tree has value marked inactive.
// PREVENTS: nop command rejected or value lost.
func TestParseNopLeaf(t *testing.T) {
	p := NewSetParser(testSchema())
	tree, err := p.Parse("nop router-id 10.0.0.1")
	require.NoError(t, err)

	v, ok := tree.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", v)
	assert.True(t, tree.IsLeafInactive("router-id"))
}

// TestParseNopNestedLeaf verifies nop through list entry and container paths.
//
// VALIDATES: AC-1 -- nop with nested path marks the correct leaf inactive.
// PREVENTS: inactive marker applied to wrong tree level.
func TestParseNopNestedLeaf(t *testing.T) {
	p := NewSetParser(testSchema())
	tree, err := p.Parse("nop neighbor 192.0.2.1 description \"test peer\"")
	require.NoError(t, err)

	entry := tree.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, entry)

	v, ok := entry.Get("description")
	require.True(t, ok)
	assert.Equal(t, "test peer", v)
	assert.True(t, entry.IsLeafInactive("description"))
}

// TestSerializeSetNopLeaf verifies that an inactive leaf serializes with nop prefix.
//
// VALIDATES: AC-2 -- tree with inactive leaf outputs "nop <path> <value>".
// PREVENTS: inactive leaf emitting "set" + separate "inactive" line.
func TestSerializeSetNopLeaf(t *testing.T) {
	tree := NewTree()
	tree.Set("router-id", "10.0.0.1")
	tree.SetLeafInactive("router-id", true)
	tree.Set("local-as", "65000")

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "nop router-id 10.0.0.1\n")
	assert.Contains(t, output, "set local-as 65000\n")
	assert.NotContains(t, output, "inactive ")
}

// TestNopRoundTrip verifies parse -> serialize -> parse produces identical output.
//
// VALIDATES: AC-3 -- nop line round-trips.
// PREVENTS: data loss through nop serialization.
func TestNopRoundTrip(t *testing.T) {
	input := "nop router-id 10.0.0.1\nset local-as 65000\n"

	schema := testSchema()
	p := NewSetParser(schema)

	tree1, err := p.Parse(input)
	require.NoError(t, err)

	output := SerializeSet(tree1, schema)

	tree2, err := p.Parse(output)
	require.NoError(t, err)

	output2 := SerializeSet(tree2, schema)
	assert.Equal(t, output, output2)
}

// TestParseInactiveBackwardCompat verifies old "set" + "inactive" lines still parse.
//
// VALIDATES: AC-4 -- old config with set + inactive loads correctly.
// PREVENTS: backward compatibility broken.
func TestParseInactiveBackwardCompat(t *testing.T) {
	input := "set router-id 10.0.0.1\ninactive router-id\n"

	p := NewSetParser(testSchema())
	tree, err := p.Parse(input)
	require.NoError(t, err)

	v, ok := tree.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", v)
	assert.True(t, tree.IsLeafInactive("router-id"))
}

// TestInactiveMigratesToNop verifies old set+inactive config saves as nop.
//
// VALIDATES: AC-5 -- old config loaded then saved uses nop lines.
// PREVENTS: old inactive format persisted on save.
func TestInactiveMigratesToNop(t *testing.T) {
	input := "set router-id 10.0.0.1\ninactive router-id\nset local-as 65000\n"

	schema := testSchema()
	p := NewSetParser(schema)
	tree, err := p.Parse(input)
	require.NoError(t, err)

	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "nop router-id 10.0.0.1\n")
	assert.Contains(t, output, "set local-as 65000\n")
	assert.NotContains(t, output, "inactive ")
}

// TestNopLeafListMember verifies per-member nop for leaf-lists.
//
// VALIDATES: AC-6 -- deactivated member as nop, active as set, individual lines.
// PREVENTS: bracket form used when members have mixed activation state.
func TestNopLeafListMember(t *testing.T) {
	tree := NewTree()
	tree.AddMultiValueMember("name-server", "8.8.8.8")
	tree.AddMultiValueMember("name-server", "1.1.1.1")
	require.NoError(t, tree.DeactivateMultiValue("name-server", "1.1.1.1"))

	schema := testSchema()
	output := SerializeSet(tree, schema)

	assert.Contains(t, output, "set name-server 8.8.8.8\n")
	assert.Contains(t, output, "nop name-server 1.1.1.1\n")
	assert.NotContains(t, output, "inactive ")
	assert.NotContains(t, output, "[")
}

// TestNopLeafListMemberRoundTrip verifies leaf-list member nop round-trips.
//
// VALIDATES: AC-6 + AC-3 -- nop member lines parse and re-serialize identically.
// PREVENTS: member activation state lost through round-trip.
func TestNopLeafListMemberRoundTrip(t *testing.T) {
	input := "set name-server 8.8.8.8\nnop name-server 1.1.1.1\n"

	schema := testSchema()
	p := NewSetParser(schema)

	tree, err := p.Parse(input)
	require.NoError(t, err)

	items := tree.GetMultiValues("name-server")
	require.Len(t, items, 2)
	assert.Equal(t, "8.8.8.8", items[0])
	assert.Equal(t, "inactive:1.1.1.1", items[1])

	output := SerializeSet(tree, schema)
	assert.Equal(t, input, output)
}

// TestDetectFormatNop verifies nop lines are detected as FormatSet.
//
// VALIDATES: AC-7 -- DetectFormat on file with nop lines returns FormatSet.
// PREVENTS: nop files misidentified as hierarchical.
func TestDetectFormatNop(t *testing.T) {
	assert.Equal(t, FormatSet, DetectFormat("nop router-id 10.0.0.1\n"))
	assert.Equal(t, FormatSet, DetectFormat("nop router-id 10.0.0.1\nset local-as 65000\n"))
}

// TestFilterSetByPathNop verifies nop lines are matched by FilterSetByPath.
//
// VALIDATES: AC-8 -- FilterSetByPath matches nop lines same as set lines.
// PREVENTS: nop lines dropped from path-filtered output.
func TestFilterSetByPathNop(t *testing.T) {
	input := "set router-id 1.2.3.4\n" +
		"nop neighbor 192.0.2.1 description \"test\"\n" +
		"set neighbor 192.0.2.1 peer-as 65001\n" +
		"set neighbor 198.51.100.1 peer-as 65002\n"

	filtered := FilterSetByPath(input, []string{"neighbor", "192.0.2.1"})
	assert.Contains(t, filtered, "nop neighbor 192.0.2.1 description \"test\"\n")
	assert.Contains(t, filtered, "set neighbor 192.0.2.1 peer-as 65001\n")
	assert.NotContains(t, filtered, "router-id")
	assert.NotContains(t, filtered, "198.51.100.1")
}

// TestNopContainerStructural verifies structural nop for inactive containers.
//
// VALIDATES: AC-9 -- inactive container emits structural "nop <path>" line,
// children retain their own set/nop state.
// PREVENTS: container deactivation flattening all descendants to nop.
func TestNopContainerStructural(t *testing.T) {
	tree := NewTree()
	entry := NewTree()
	entry.Set("peer-as", "65001")
	entry.Set("description", "test peer")
	entry.SetLeafInactive("description", true)
	tree.AddListEntry("neighbor", "192.0.2.1", entry)
	entry.SetInactive(true)

	schema := testSchema()
	output := SerializeSet(tree, schema)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	assert.Equal(t, "nop neighbor 192.0.2.1", lines[0])
	assert.Contains(t, output, "nop neighbor 192.0.2.1 description \"test peer\"\n")
	assert.Contains(t, output, "set neighbor 192.0.2.1 peer-as 65001\n")
}

// TestNopContainerRoundTrip verifies structural nop round-trips.
//
// VALIDATES: AC-9 + AC-3 -- structural nop parsed and re-serialized identically.
// PREVENTS: structural nop lost on re-parse.
func TestNopContainerRoundTrip(t *testing.T) {
	input := "nop neighbor 192.0.2.1\n" +
		"set neighbor 192.0.2.1 peer-as 65001\n"

	schema := testSchema()
	p := NewSetParser(schema)

	tree, err := p.Parse(input)
	require.NoError(t, err)

	entry := tree.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, entry)
	assert.True(t, entry.IsInactive())

	v, _ := entry.Get("peer-as")
	assert.Equal(t, "65001", v)

	output := SerializeSet(tree, schema)
	assert.Equal(t, input, output)
}

// TestParseNopUnknownField verifies nop with unknown path is rejected.
//
// VALIDATES: nop validates paths like set does.
// PREVENTS: silent acceptance of typos in nop lines.
func TestParseNopUnknownField(t *testing.T) {
	p := NewSetParser(testSchema())
	_, err := p.Parse("nop bogus-field value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

// TestParseNopErrorMessage verifies the error message includes nop.
//
// VALIDATES: error message lists nop as a valid command.
// PREVENTS: confusing error when user sees "expected set/delete/inactive".
func TestParseNopErrorMessage(t *testing.T) {
	p := NewSetParser(testSchema())
	_, err := p.Parse("bogus router-id 1.2.3.4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nop")
}

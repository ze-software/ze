package fleetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// builtinView answers the built-in tree view named name.
func builtinView(t *testing.T, name string) TreeView {
	t.Helper()

	view, err := ResolveTreeView(name, nil)
	require.NoError(t, err)

	return view
}

// nodeSummary is what a test reads of one tree node.
type nodeSummary struct {
	title     string
	connected int
	total     int
	attention bool
}

// summarize answers the summaries of nodes, in their order.
func summarize(nodes []TreeNode) []nodeSummary {
	summaries := make([]nodeSummary, 0, len(nodes))
	for index := range nodes {
		node := &nodes[index]
		summaries = append(summaries, nodeSummary{node.Title, node.Connected, node.Total, node.Attention})
	}

	return summaries
}

// deviceNames answers the names of the device rows of a node.
func deviceNames(node *TreeNode) []string {
	names := make([]string, 0, len(node.Devices))
	for _, device := range node.Devices {
		names = append(names, device.Name)
	}

	return names
}

// TestTreeGrouping groups the test estate by site, then role.
//
// VALIDATES: devices group by the view's levels, each node counts its
// connected devices over its total, the node of the devices that lack the
// label is named "(no site)" and listed last, a node is flagged when a device
// below it is not healthy, and device rows come in name order.
// PREVENTS: a device hidden because it lacks a label, and a count that
// disagrees with the rows under it.
func TestTreeGrouping(t *testing.T) {
	tree, err := BuildTree("hub-lon-01", builtinView(t, TreeViewDefault), testViews())
	require.NoError(t, err)

	assert.Equal(t, nodeSummary{"hub-lon-01", 6, 7, true}, summarize([]TreeNode{tree.Root})[0])
	assert.Empty(t, tree.Root.Path)

	assert.Equal(t, []nodeSummary{
		{"ams", 1, 2, true},
		{"lon", 3, 3, true},
		{"(no site)", 2, 2, true},
	}, summarize(tree.Root.Children))

	lon := &tree.Root.Children[1]
	assert.Equal(t, []GroupLevel{{Key: "site", Value: "lon"}}, lon.Path)
	assert.Equal(t, []nodeSummary{
		{"core", 1, 1, true},
		{"edge", 2, 2, false},
	}, summarize(lon.Children))
	assert.Empty(t, lon.Devices, "only a node of the last level holds device rows")

	edge := &lon.Children[1]
	assert.Equal(t, []GroupLevel{{Key: "site", Value: "lon"}, {Key: "role", Value: "edge"}}, edge.Path)
	assert.Equal(t, []string{edgeLon01, edgeLon02}, deviceNames(edge))

	lacking := &tree.Root.Children[2]
	assert.Equal(t, GroupLevel{Key: "site", Missing: true}, lacking.Level)
	assert.Equal(t, []nodeSummary{{"cpe", 2, 2, true}}, summarize(lacking.Children))
	assert.Equal(t, []string{cpe0113, cpe0200}, deviceNames(&lacking.Children[0]))
}

// TestTreeDeviceRows reads the device rows of one node.
//
// VALIDATES: a device row carries its display state, its open alarm count
// and whether a task targets it.
// PREVENTS: a tree row that shows another state than the table row of the
// same device.
func TestTreeDeviceRows(t *testing.T) {
	tree, err := BuildTree("hub", builtinView(t, "role"), testViews())
	require.NoError(t, err)

	core, err := tree.Find([]GroupLevel{{Key: "role", Value: "core"}})
	require.NoError(t, err)
	assert.Equal(t, []TreeDevice{
		{Name: coreAms01, State: DisplayStateOffline, AlarmsOpen: 1},
		{Name: coreLon01, State: DisplayStateDown, AlarmsOpen: 2},
	}, core.Devices)

	edge, err := tree.Find([]GroupLevel{{Key: "role", Value: "edge"}})
	require.NoError(t, err)
	assert.Equal(t, []TreeDevice{
		{Name: edgeLon01, State: DisplayStateHealthy, Busy: true},
		{Name: edgeLon02, State: DisplayStateHealthy, Busy: true},
	}, edge.Devices)
}

// TestTreeByVersion groups the test estate by software version.
//
// VALIDATES: the version level reads the registry field, lists the newest
// version first, and puts the device that reported no version last.
// PREVENTS: versions listed in text order, where 1.10 sorts before 1.9.
func TestTreeByVersion(t *testing.T) {
	tree, err := BuildTree("hub", builtinView(t, "version"), testViews())
	require.NoError(t, err)

	assert.Equal(t, []nodeSummary{
		{"1.4.2", 3, 3, true},
		{"1.4.1", 1, 2, true},
		{"1.3.9", 1, 1, true},
		{"(no version)", 1, 1, true},
	}, summarize(tree.Root.Children))
}

// TestTreeByState groups the test estate by display state.
//
// VALIDATES: the state level lists the worst state first, shows the state's
// word, and addresses the node by the state's key.
// PREVENTS: a state node whose address is a word that changes with wording.
func TestTreeByState(t *testing.T) {
	tree, err := BuildTree("hub", builtinView(t, "state"), testViews())
	require.NoError(t, err)

	titles := make([]string, 0, len(tree.Root.Children))
	for index := range tree.Root.Children {
		titles = append(titles, tree.Root.Children[index].Title)
	}
	assert.Equal(t, []string{"Down", "Offline", "Diverged", "Degraded", "Unknown", "Healthy"}, titles)

	healthy := &tree.Root.Children[5]
	assert.Equal(t, GroupLevel{Key: "state", Value: "healthy"}, healthy.Level)
	assert.False(t, healthy.Attention)
	assert.Equal(t, []string{edgeLon01, edgeLon02}, deviceNames(healthy))
}

// TestTreeFindByPath looks nodes up by the path of a group address.
//
// VALIDATES: the empty path is the root, a path of one and of two levels
// finds its node, the node of the devices that lack a label is found by its
// own path, and a path the tree does not hold is refused as not found.
// PREVENTS: a group address that opens another node than the tree row it was
// copied from.
func TestTreeFindByPath(t *testing.T) {
	tree, err := BuildTree("hub", builtinView(t, TreeViewDefault), testViews())
	require.NoError(t, err)

	root, err := tree.Find(nil)
	require.NoError(t, err)
	assert.Equal(t, "hub", root.Title)

	ams, err := tree.Find([]GroupLevel{{Key: "site", Value: "ams"}})
	require.NoError(t, err)
	assert.Equal(t, 2, ams.Total)

	routeServer, err := tree.Find([]GroupLevel{{Key: "site", Value: "ams"}, {Key: "role", Value: "route-server"}})
	require.NoError(t, err)
	assert.Equal(t, []string{rsAms01}, deviceNames(routeServer))

	lacking, err := tree.Find([]GroupLevel{{Key: "site", Missing: true}})
	require.NoError(t, err)
	assert.Equal(t, 2, lacking.Total)

	_, err = tree.Find([]GroupLevel{{Key: "site", Value: "par"}})
	refusal := requireRefusal(t, err, ErrObjectUnknown)
	assert.Contains(t, refusal.Problem(), "site=par")

	_, err = tree.Find([]GroupLevel{{Key: "role", Value: "edge"}})
	requireRefusal(t, err, ErrObjectUnknown)
}

// TestGroupDevices answers the devices of group nodes by their path.
//
// VALIDATES: the devices of a node are those that hold every level of its
// path, the node of the devices that lack a label holds exactly those, and a
// path no device holds is refused as not found.
// PREVENTS: a group page that counts other devices than its tree node.
func TestGroupDevices(t *testing.T) {
	views := testViews()

	lon, err := GroupDevices(views, []GroupLevel{{Key: "site", Value: "lon"}})
	require.NoError(t, err)
	assert.Equal(t, []string{edgeLon01, edgeLon02, coreLon01}, namesOf(lon))

	lonCore, err := GroupDevices(views, []GroupLevel{{Key: "site", Value: "lon"}, {Key: "role", Value: "core"}})
	require.NoError(t, err)
	assert.Equal(t, []string{coreLon01}, namesOf(lonCore))

	lacking, err := GroupDevices(views, []GroupLevel{{Key: "site", Missing: true}})
	require.NoError(t, err)
	assert.Equal(t, []string{cpe0113, cpe0200}, namesOf(lacking))

	down, err := GroupDevices(views, []GroupLevel{{Key: "state", Value: "down"}})
	require.NoError(t, err)
	assert.Equal(t, []string{coreLon01}, namesOf(down))

	_, err = GroupDevices(views, []GroupLevel{{Key: "site", Value: "par"}})
	requireRefusal(t, err, ErrObjectUnknown)

	_, err = GroupDevices(views, nil)
	requireRefusal(t, err, ErrObjectUnknown)
}

// TestFilterTreeKeepsAncestors narrows the tree with the tree filter.
//
// VALIDATES: the filter keeps the devices whose name or any label value
// holds the text, ignoring case, and the tree built from them holds their
// ancestors and no other branch.
// PREVENTS: a filtered tree that shows a device with no branch above it, or
// a branch with no device below it.
func TestFilterTreeKeepsAncestors(t *testing.T) {
	views := testViews()

	kept := FilterTree(views, " CORE ")
	assert.Equal(t, []string{coreLon01, coreAms01}, namesOf(kept))

	tree, err := BuildTree("hub", builtinView(t, TreeViewDefault), kept)
	require.NoError(t, err)
	assert.Equal(t, []nodeSummary{
		{"ams", 0, 1, true},
		{"lon", 1, 1, true},
	}, summarize(tree.Root.Children))

	assert.Equal(t, namesOf(views), namesOf(FilterTree(views, "")))
	assert.Empty(t, FilterTree(views, "no-such-text"))
}

// TestResolveTreeView resolves view names against the built-in views and the
// declared ones.
//
// VALIDATES: the four built-in views exist without configuration, a declared
// view is found after them, and an unknown name is refused with the list of
// views.
// PREVENTS: a tree silently regrouped by the default view when the address
// names a view that does not exist.
func TestResolveTreeView(t *testing.T) {
	assert.Equal(t, []string{"site", "role"}, builtinView(t, "site-role").Levels)
	assert.Equal(t, []string{"role"}, builtinView(t, "role").Levels)
	assert.Equal(t, []string{"version"}, builtinView(t, "version").Levels)
	assert.Equal(t, []string{"state"}, builtinView(t, "state").Levels)

	declared := []TreeView{{Name: "customer", Title: "Customer", Levels: []string{"customer", "site"}}}

	view, err := ResolveTreeView("customer", declared)
	require.NoError(t, err)
	assert.Equal(t, []string{"customer", "site"}, view.Levels)

	_, err = ResolveTreeView("nothing", declared)
	refusal := requireRefusal(t, err, ErrTreeViewUnknown)
	assert.Contains(t, refusal.Problem(), `"nothing"`)
	assert.Contains(t, refusal.Fix(), "site-role, role, version, state, customer")

	_, err = ResolveTreeView("", nil)
	requireRefusal(t, err, ErrTreeViewUnknown)
}

// TestTreeDepthBoundary builds a tree of exactly the most levels and of one
// more.
//
// VALIDATES: a view of TreeLevelsMax levels builds and one level more is
// refused.
// PREVENTS: a tree whose depth a view's configuration decides without bound.
func TestTreeDepthBoundary(t *testing.T) {
	levels := make([]string, 0, TreeLevelsMax+1)
	for range TreeLevelsMax {
		levels = append(levels, "site")
	}

	_, err := BuildTree("hub", TreeView{Name: "deep", Levels: levels}, testViews())
	require.NoError(t, err)

	_, err = BuildTree("hub", TreeView{Name: "deeper", Levels: append(levels, "site")}, testViews())
	require.Error(t, err)
}

// Design: docs/architecture/fleet-interface.md -- the estate tree of the sidebar and its two fragments
// Related: tree.go -- BuildTree and FilterTree, which group the devices
// Related: shell_view.go -- TreePaneView and TreeRowView
// Related: tree.templ -- the markup of the tree
//
// The tree is built for every page from the snapshot of its request. The
// rows down to the second level are always drawn. The children of a deeper
// row are left out and load when the operator first expands it, unless the
// row leads to the selected node, or a filter is at work: a filter draws
// everything it keeps, expanded.

package fleetui

import (
	"context"
	"net/url"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// treeEagerDepth is the depth down to which rows are always drawn: the
// estate is at depth zero, so this is the second level under it.
const treeEagerDepth = 2

// The keys of the rows that are not group nodes.
const (
	treeKeyEstate = "estate"
	treeKeyDevice = "device/"
)

// treeBuild holds what every row of one tree is built against.
type treeBuild struct {
	request *PageRequest
	// selected is the path of the selected group node, or of the node the
	// selected device sits under. It means nothing while selects is false.
	selected []GroupLevel
	// device is the name of the selected device, or empty.
	device string
	// selects reports that the address selects a row at all: the estate, a
	// group node or a device.
	selects bool
	// node reports that the selected row is the estate or a group node,
	// the one at selected.
	node bool
	// expandAll reports that a filter is at work, which draws every row it
	// keeps, expanded.
	expandAll bool
}

// newTreeBuild reads which row of the tree the request selects. selects is
// false for an address that names no page.
func newTreeBuild(request *PageRequest, selects bool, filtering bool) *treeBuild {
	build := &treeBuild{request: request, expandAll: filtering}
	if !selects {
		return build
	}

	address := request.Address

	switch address.Kind {
	case ObjectEstate:
		build.selects = true
		build.node = true
	case ObjectGroup:
		build.selects = true
		build.node = true
		build.selected = address.Group
	case ObjectDevice:
		view, err := FindDevice(request.Snapshot.Views, address.Device)
		if err != nil {
			// The address names a device the estate does not hold: no
			// row is selected, and the page shows the not-found state.
			return build
		}
		build.selects = true
		build.device = address.Device
		build.selected = devicePath(request, view)
	case ObjectTasks, ObjectAdmin:
		// Neither is a row of the tree.
	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}

	return build
}

// leads reports whether the node at path is the selected node or one of its
// ancestors, or the node the selected device sits under.
func (build *treeBuild) leads(path []GroupLevel) bool {
	if !build.selects {
		return false
	}
	if len(path) > len(build.selected) {
		return false
	}

	return slices.Equal(path, build.selected[:len(path)])
}

// opens reports whether the node at path is drawn expanded: the estate
// always, every node under a filter, and the nodes on the way to the selected
// row.
func (build *treeBuild) opens(path []GroupLevel) bool {
	if len(path) == 0 {
		return true
	}
	if build.expandAll {
		return true
	}
	if !build.leads(path) {
		return false
	}
	if len(path) < len(build.selected) {
		return true
	}

	// The node is the one at the selected path. It opens for the device it
	// holds, and stays as it was when it is the selected row itself.
	return build.device != ""
}

// nodeKey answers the key of a group node: the tail of its address, which
// the tree branch fragment reads back.
func nodeKey(path []GroupLevel) string {
	if len(path) == 0 {
		return treeKeyEstate
	}

	return Address{Kind: ObjectGroup, Group: path, Tab: TabDashboard}.objectPath()
}

// nodeBusy reports whether a task targets a device below node. It recurses
// once per level of the tree view, which BuildTree bounds by TreeLevelsMax.
func nodeBusy(node *TreeNode) bool {
	for index := range node.Devices {
		if node.Devices[index].Busy {
			return true
		}
	}
	for index := range node.Children {
		if nodeBusy(&node.Children[index]) {
			return true
		}
	}

	return false
}

// nodeRow builds the row of the estate or of a group node, at depth levels
// below the estate. It recurses once per level of the tree view, which
// BuildTree bounds by TreeLevelsMax.
func (build *treeBuild) nodeRow(node *TreeNode, depth int) TreeRowView {
	request := build.request

	address := estateAddress(request)
	icon := iconEstate
	if depth > 0 {
		address = groupAddress(request, node.Path)
		icon = iconGroup
	}

	row := TreeRowView{
		Key:        nodeKey(node.Path),
		Title:      node.Title,
		Href:       address.String(),
		Icon:       ObjectIconView{Icon: icon},
		Busy:       nodeBusy(node),
		Count:      textbuf.New().Int(int64(node.Connected)).Byte('/').Int(int64(node.Total)).Slice(),
		Attention:  node.Attention,
		Expandable: node.Total > 0,
		Expanded:   build.opens(node.Path),
		Menu:       objectMenu(request, address, node.Title),
	}
	if build.node {
		row.Selected = build.selects && slices.Equal(node.Path, build.selected)
	}

	if !row.Expandable {
		return row
	}

	// The children of a deep row that is drawn closed are left out. The
	// script loads them when the operator first expands the row.
	if depth >= treeEagerDepth {
		if !row.Expanded {
			row.LazyURL = FragmentLink(fragmentTreeBranch, estateAddress(request), row.Key)
			return row
		}
	}

	row.Children = build.childRows(node, depth)

	return row
}

// childRows builds the rows under node: its group nodes, then its devices.
func (build *treeBuild) childRows(node *TreeNode, depth int) []TreeRowView {
	rows := make([]TreeRowView, 0, len(node.Children)+len(node.Devices))
	for index := range node.Children {
		rows = append(rows, build.nodeRow(&node.Children[index], depth+1))
	}
	for index := range node.Devices {
		rows = append(rows, build.deviceRow(&node.Devices[index]))
	}

	return rows
}

// deviceRow builds the row of one device.
func (build *treeBuild) deviceRow(device *TreeDevice) TreeRowView {
	address := deviceAddress(build.request, device.Name)

	return TreeRowView{
		Key:      treeKeyDevice + device.Name,
		Title:    device.Name,
		Href:     address.String(),
		Icon:     ObjectIconView{Icon: iconDevice, Status: DisplayStatus(device.State), Marked: true},
		Busy:     device.Busy,
		Alarms:   device.AlarmsOpen,
		Selected: build.device == device.Name,
		Menu:     objectMenu(build.request, address, device.Name),
	}
}

// buildTree groups the devices the filter keeps under the tree view of the
// request.
func buildTree(request *PageRequest, filter string) (Tree, error) {
	views := request.Snapshot.Views
	if filter != "" {
		views = FilterTree(views, filter)
	}

	return BuildTree(request.Snapshot.Estate.Name, request.treeView, views)
}

// treeRowsView builds the tree narrowed by filter, which is the text of the
// filter box as typed. selects is false for an address that names no page.
func treeRowsView(request *PageRequest, filter string, selects bool) (TreeRowsView, error) {
	text := strings.TrimSpace(filter)

	tree, err := buildTree(request, text)
	if err != nil {
		return TreeRowsView{}, err
	}

	build := newTreeBuild(request, selects, text != "")

	return TreeRowsView{
		ViewName: request.treeView.Name,
		Rows:     []TreeRowView{build.nodeRow(&tree.Root, 0)},
		Filter:   text,
		Empty:    tree.Root.Total == 0,
	}, nil
}

// treePaneView builds the Estate tab of the sidebar with its unfiltered
// tree. The filter box starts empty on every page: the filter narrows the
// tree in place and is no part of an address.
func treePaneView(request *PageRequest, selects bool) (TreePaneView, error) {
	rows, err := treeRowsView(request, "", selects)
	if err != nil {
		return TreePaneView{}, err
	}

	views := append(BuiltinTreeViews(), request.Snapshot.TreeViews...)
	options := make([]TreeViewOptionView, 0, len(views))
	for _, view := range views {
		target := request.Address
		target.View.TreeView = view.Name
		options = append(options, TreeViewOptionView{
			Title:    view.Title,
			Href:     target.String(),
			Selected: view.Name == request.treeView.Name,
		})
	}

	return TreePaneView{Options: options, Rows: rows}, nil
}

// treeFragment answers the tree narrowed by the text of the filter box,
// which the fragment address carries as its item.
func treeFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	rows, err := treeRowsView(request, item, true)
	if err != nil {
		return nil, err
	}

	return treeRows(rows), nil
}

// branchUnknown is the refusal for a tree row key that names no group node.
func branchUnknown(key string) error {
	problem := textbuf.New().Str("The estate tree has no branch ").Quoted(key).Str(".").Slice()

	return opererr.Wrap(ErrObjectUnknown, problem,
		"The branch was asked for by a key no row of the tree carries, so it has no rows to show.",
		"open "+fleetPath)
}

// treeBranchFragment answers the rows under one group node, for a row whose
// children were left out of the tree. The fragment address carries the row's
// key as its item.
func treeBranchFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	target, err := url.Parse(fleetPath + item)
	if err != nil {
		return nil, branchUnknown(item)
	}

	branch, err := ParseAddress(target)
	if err != nil {
		return nil, branchUnknown(item)
	}
	if branch.Kind != ObjectGroup {
		return nil, branchUnknown(item)
	}

	tree, err := buildTree(request, "")
	if err != nil {
		return nil, err
	}

	node, err := tree.Find(branch.Group)
	if err != nil {
		return nil, err
	}

	build := newTreeBuild(request, true, false)

	return treeBranch(TreeRowsView{Rows: build.childRows(node, len(branch.Group))}), nil
}

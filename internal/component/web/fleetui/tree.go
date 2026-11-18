// Design: docs/architecture/fleet-interface.md -- the estate tree, computed from labels
// Related: derive.go -- the device views the tree groups
// Related: url.go -- GroupLevel, the path of a group node in an address
// Related: register_devices.go -- the tree levels that read a registry field
//
// The tree is computed, never stored. A tree view is an ordered list of level
// keys, and each key is one level of the tree: the devices that share a value
// for the key form one node. A key is a label key, unless a tree level is
// registered under that name, which then reads a field of the device, as the
// software version and the display state do. The devices that lack the key of
// a level sit under one node of their own, listed last.

package fleetui

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The label keys the built-in tree views group on.
const (
	labelSite = "site"
	labelRole = "role"
)

// TreeViewDefault is the name of the tree view of an address that names
// none.
const TreeViewDefault = "site-role"

// ErrTreeViewUnknown means an address names a tree view the hub does not
// have.
var ErrTreeViewUnknown = errors.New("no such tree view")

// BuiltinTreeViews answers the tree views every hub has without any
// configuration: by site then role, by role, by software version, by state.
// The first is the default view.
func BuiltinTreeViews() []TreeView {
	return []TreeView{
		{Name: TreeViewDefault, Title: "Site, then role", Levels: []string{labelSite, labelRole}},
		{Name: labelRole, Title: "Role", Levels: []string{labelRole}},
		{Name: fieldVersion, Title: "Software version", Levels: []string{fieldVersion}},
		{Name: fieldState, Title: "State", Levels: []string{fieldState}},
	}
}

// ResolveTreeView answers the tree view an address names, among the built-in
// views and then the views the hub declares. ParseViewParams answers
// TreeViewDefault for an address with no view parameter, so name is never
// empty for a parsed address. A name neither set holds is refused with an
// error wrapping ErrTreeViewUnknown that lists the views.
func ResolveTreeView(name string, declared []TreeView) (TreeView, error) {
	views := append(BuiltinTreeViews(), declared...)
	names := make([]string, 0, len(views))
	for _, view := range views {
		if view.Name == name {
			return view, nil
		}
		names = append(names, view.Name)
	}

	problem := textbuf.New().Str("This hub has no tree view named ").Quoted(name).Str(".").Slice()
	fix := textbuf.New().Str("choose one of: ").Join(names, ", ").Slice()

	return TreeView{}, opererr.Wrap(ErrTreeViewUnknown, problem,
		"The tree cannot be grouped by a view that does not exist, so it was not changed.", fix)
}

// TreeLevel is one level of a tree that reads a field of the device where a
// plain level reads a label. Register one with MustRegisterTreeLevel.
type TreeLevel struct {
	// Key is the level's name in a tree view and in a group address. It
	// takes precedence over a label of the same key.
	Key string
	// Value answers the value a device has for the level, or false for a
	// device that has none.
	Value func(view *DeviceView) (string, bool)
	// Compare orders the nodes of the level by their values. A nil Compare
	// orders them as text.
	Compare func(a, b string) int
	// Title answers what a node of the level shows for a value. A nil Title
	// shows the value itself.
	Title func(value string) string
}

// treeLevelRegistry holds the registered tree levels. The zero value is an
// empty registry, ready for use. Safe for concurrent use.
type treeLevelRegistry struct {
	mu     sync.RWMutex
	levels map[string]TreeLevel
}

// treeLevels is the tree level registry of this build.
var treeLevels treeLevelRegistry

// bugTreeLevel is what a refused registration of this file names.
const bugTreeLevel = "the tree level"

// treeSubject is the set a tree level joins, as a refused registration names
// it.
const treeSubject = "the estate tree"

// MustRegisterTreeLevel adds a tree level that reads a field of the device.
// Call it from init() in a register file. It panics on a level with no key or
// no value function and on a key already registered: each is a defect of the
// build's composition.
func MustRegisterTreeLevel(level TreeLevel) {
	if level.Key == "" {
		registrationBug(bugTreeLevel, level.Key, treeSubject, bugUnnamed)
	}
	if level.Value == nil {
		registrationBug(bugTreeLevel, level.Key, treeSubject, "has no value function")
	}

	treeLevels.mu.Lock()
	defer treeLevels.mu.Unlock()

	if _, held := treeLevels.levels[level.Key]; held {
		registrationBug(bugTreeLevel, level.Key, treeSubject, bugTwice)
	}
	if treeLevels.levels == nil {
		treeLevels.levels = make(map[string]TreeLevel)
	}
	treeLevels.levels[level.Key] = level
}

// levelOf answers the level a key names: the registered level of that key,
// or the level that reads the label of that key.
func levelOf(key string) TreeLevel {
	treeLevels.mu.RLock()
	level, registered := treeLevels.levels[key]
	treeLevels.mu.RUnlock()

	if registered {
		return level
	}

	return TreeLevel{
		Key:   key,
		Value: func(view *DeviceView) (string, bool) { return view.Label(key) },
	}
}

// compare orders two values of the level.
func (level *TreeLevel) compare(a, b string) int {
	if level.Compare == nil {
		return strings.Compare(a, b)
	}

	return level.Compare(a, b)
}

// title answers what a node shows for one value of the level.
func (level *TreeLevel) title(value string) string {
	if level.Title == nil {
		return value
	}

	return level.Title(value)
}

// holds reports whether a device belongs under the node a group level names.
func (group GroupLevel) holds(view *DeviceView) bool {
	level := levelOf(group.Key)
	value, present := level.Value(view)
	if group.Missing {
		return !present
	}

	return present && value == group.Value
}

// Tree is the estate grouped by one tree view.
type Tree struct {
	View TreeView
	// Root is the estate itself. Its children are the nodes of the first
	// level, or it holds the devices when the view has no level.
	Root TreeNode
}

// TreeNode is one row of the tree that is not a device: the estate, or the
// devices that share the values of one branch.
type TreeNode struct {
	// Level is the last level of the node's path. It is the zero value for
	// the root.
	Level GroupLevel
	// Title is what the row shows: the estate's name, the value the devices
	// share, or "(no <key>)" for the devices that lack the key.
	Title string
	// Path is the node's path from the root, one level per tree level, as a
	// group address carries it. It is empty for the root.
	Path []GroupLevel
	// Connected counts the connected devices below the node, over Total.
	Connected int
	Total     int
	// Attention reports that a device below the node is not healthy. The
	// row's count then takes the warning color.
	Attention bool
	// Children holds the nodes of the next level, ordered by the level, the
	// node of the devices that lack the key last.
	Children []TreeNode
	// Devices holds the device rows of a node of the last level, in name
	// order.
	Devices []TreeDevice
}

// TreeDevice is one device row of the tree.
type TreeDevice struct {
	Name  string
	State DisplayState
	// AlarmsOpen is the number the row's badge shows when it is not zero.
	AlarmsOpen int
	// Busy reports that a task targets the device, so a spinner replaces
	// the row's state icon.
	Busy bool
}

// BuildTree groups views into the tree of one view, under a root named after
// the estate. It refuses a view with more than TreeLevelsMax levels.
func BuildTree(estate string, view TreeView, views []DeviceView) (Tree, error) {
	if len(view.Levels) > TreeLevelsMax {
		problem := textbuf.New().Str("The tree view ").Quoted(view.Name).Str(" has more levels than a tree can hold.").Slice()

		return Tree{}, opererr.New(problem,
			"A tree deeper than the limit cannot be shown, so the tree was not built.",
			"remove levels from the tree view until it has "+textbuf.StringInt(TreeLevelsMax)+" or fewer")
	}

	levels := make([]TreeLevel, 0, len(view.Levels))
	for _, key := range view.Levels {
		levels = append(levels, levelOf(key))
	}

	members := make([]*DeviceView, len(views))
	for index := range views {
		members[index] = &views[index]
	}

	tree := Tree{View: view, Root: TreeNode{Title: estate}}
	buildNode(&tree.Root, members, levels)

	return tree, nil
}

// buildNode fills one node from the devices below it, then builds its
// children for the next level. It recurses once per level of the view, and
// BuildTree refuses a view deeper than TreeLevelsMax, so the depth is bounded
// by that constant.
func buildNode(node *TreeNode, members []*DeviceView, levels []TreeLevel) {
	node.Total = len(members)
	for _, member := range members {
		if member.Device.Connection.Connected {
			node.Connected++
		}
		if member.State != DisplayStateHealthy {
			node.Attention = true
		}
	}

	if len(levels) == 0 {
		node.Devices = treeDevices(members)
		return
	}

	level := &levels[0]
	branches := branchesOf(members, level)

	node.Children = make([]TreeNode, len(branches))
	for index := range branches {
		branch := &branches[index]
		child := &node.Children[index]

		child.Level = GroupLevel{Key: level.Key, Value: branch.value, Missing: branch.missing}
		child.Title = level.title(branch.value)
		if branch.missing {
			child.Title = "(no " + level.Key + ")"
		}
		child.Path = append(slices.Clone(node.Path), child.Level)

		buildNode(child, branch.members, levels[1:])
	}
}

// branch is the devices that share one value of a level, or that lack it.
type branch struct {
	value   string
	missing bool
	members []*DeviceView
}

// branchesOf splits members by their value for level, in the order of the
// level, with the branch of the devices that lack the level last.
func branchesOf(members []*DeviceView, level *TreeLevel) []branch {
	valued := make(map[string][]*DeviceView)
	var lacking []*DeviceView

	for _, member := range members {
		value, present := level.Value(member)
		if present {
			valued[value] = append(valued[value], member)
			continue
		}
		lacking = append(lacking, member)
	}

	values := slices.SortedFunc(maps.Keys(valued), level.compare)

	branches := make([]branch, 0, len(values)+1)
	for _, value := range values {
		branches = append(branches, branch{value: value, members: valued[value]})
	}
	if len(lacking) > 0 {
		branches = append(branches, branch{missing: true, members: lacking})
	}

	return branches
}

// treeDevices answers the device rows of members, in name order.
func treeDevices(members []*DeviceView) []TreeDevice {
	rows := make([]TreeDevice, 0, len(members))
	for _, member := range members {
		rows = append(rows, TreeDevice{
			Name:       member.Device.Name,
			State:      member.State,
			AlarmsOpen: member.AlarmsOpen,
			Busy:       member.TasksActive > 0,
		})
	}
	slices.SortFunc(rows, func(a, b TreeDevice) int { return cmp.Compare(a.Name, b.Name) })

	return rows
}

// Find answers the node at path, which is the path of a group address. The
// empty path is the root. A path the tree does not hold is refused with an
// error wrapping ErrObjectUnknown.
func (tree *Tree) Find(path []GroupLevel) (*TreeNode, error) {
	node := &tree.Root
	for _, level := range path {
		child, held := node.child(level)
		if !held {
			return nil, groupUnknown(path)
		}
		node = child
	}

	return node, nil
}

// child answers the child of the node at one level, or false.
func (node *TreeNode) child(level GroupLevel) (*TreeNode, bool) {
	for index := range node.Children {
		if node.Children[index].Level == level {
			return &node.Children[index], true
		}
	}

	return nil, false
}

// GroupDevices answers the devices of the group node at path, in the order of
// views: the devices that hold every level of the path. A group node exists
// only while it holds a device, so a path no device holds is refused with an
// error wrapping ErrObjectUnknown, as is the empty path.
func GroupDevices(views []DeviceView, path []GroupLevel) ([]DeviceView, error) {
	if len(path) == 0 {
		return nil, groupUnknown(path)
	}

	held := make([]DeviceView, 0, len(views))
	for index := range views {
		if holdsEvery(&views[index], path) {
			held = append(held, views[index])
		}
	}
	if len(held) == 0 {
		return nil, groupUnknown(path)
	}

	return held, nil
}

// holdsEvery reports whether a device belongs under every level of path.
func holdsEvery(view *DeviceView, path []GroupLevel) bool {
	for _, level := range path {
		if !level.holds(view) {
			return false
		}
	}

	return true
}

// groupUnknown is the refusal for a group path no device of the estate holds.
func groupUnknown(path []GroupLevel) error {
	name := textbuf.New()
	for index, level := range path {
		if index > 0 {
			name.Str(", ")
		}
		name.Str(level.Key)
		if !level.Missing {
			name.Str(groupSeparator).Str(level.Value)
		}
	}

	problem := textbuf.New().Str("No device of the estate is in the group ").Quoted(name.String()).Str(".").Slice()

	return opererr.Wrap(ErrObjectUnknown, problem,
		"A group exists only while a device carries its labels, so there is nothing to show.",
		"open "+fleetPath)
}

// FilterTree answers the devices the tree filter keeps: those whose name or
// any label value holds text, ignoring case. The tree built from them keeps
// their ancestors and nothing else. The empty text keeps every device.
func FilterTree(views []DeviceView, text string) []DeviceView {
	needle := strings.ToLower(strings.TrimSpace(text))

	kept := make([]DeviceView, 0, len(views))
	for index := range views {
		if views[index].contains(needle) {
			kept = append(kept, views[index])
		}
	}

	return kept
}

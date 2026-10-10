// Design: docs/guide/config-editor.md -- load merge and replace
// Related: editor_commands.go -- the set, delete and toggle primitives a load applies
// Related: ../config/parser.go -- ParseAt, which parses a fragment at a context

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ParseLoad parses load input for contextPath. At the root it takes either
// format a config file holds, hierarchical or set lines. At a context it takes
// a hierarchical body, checked against the context node's children
// (config.Parser.ParseAt); set lines name paths from the root, so a relative
// load refuses them rather than guess where they land.
func (e *Editor) ParseLoad(content string, contextPath []string) (*config.Tree, error) {
	if len(contextPath) == 0 {
		tree, _, err := parseConfigWithFormat(content, e.schema)
		if err != nil {
			return nil, fmt.Errorf("load: %w", err)
		}
		return tree, nil
	}
	if config.DetectFormat(content) != config.FormatHierarchical {
		return nil, errors.New("load: a relative load takes a hierarchical body; load set lines with 'absolute'")
	}
	tree, err := config.NewParser(e.schema).ParseAt(content, contextPath)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return tree, nil
}

// LoadMerge merges loaded, a parsed tree rooted at contextPath, into the
// configuration. Every leaf, leaf-list member, container, list entry and
// inactive marker it carries that the configuration does not already hold is
// applied through the editor's own primitives, so a session load records the
// same per-leaf change entries a typed `set` does and a file-mode load edits
// the same tree. Nothing outside contextPath changes.
// Not safe for concurrent use: the caller holds the editor.
func (e *Editor) LoadMerge(contextPath []string, loaded *config.Tree) error {
	return e.load(contextPath, loaded, false)
}

// LoadReplace makes the node at contextPath hold exactly loaded: what
// LoadMerge applies, plus a delete for every leaf, member, container, list and
// list entry the node holds and loaded omits.
// Not safe for concurrent use: the caller holds the editor.
func (e *Editor) LoadReplace(contextPath []string, loaded *config.Tree) error {
	return e.load(contextPath, loaded, true)
}

// load applies loaded all or nothing. It snapshots the tree, the metadata and
// the dirty state first; a session load then stages every change-file write in
// memory under one lock (loadStaged) and writes the file once at the end. Any
// failure, a refused entry or an I/O error, restores the snapshot, so the
// candidate is exactly what it was and the error says so.
func (e *Editor) load(contextPath []string, loaded *config.Tree, replace bool) error {
	if loaded == nil {
		return fmt.Errorf("load: no configuration to load")
	}
	if !e.treeValid {
		return fmt.Errorf("load: the configuration did not parse, so there is no tree to load into")
	}
	tree := e.tree.Clone()
	var meta *config.MetaTree
	if e.meta != nil {
		meta = e.meta.Clone()
	}
	dirty, draftSaved := e.dirty.Load(), e.draftSaved

	err := e.loadStaged(contextPath, loaded, replace)
	if err == nil {
		return nil
	}
	e.tree, e.meta, e.draftSaved = tree, meta, draftSaved
	e.dirty.Store(dirty)
	return fmt.Errorf("load refused, candidate unchanged: %w", err)
}

// loadStaged runs loadApply. In a session it holds the write-through lock for
// the whole load and routes every step through a loadStage, which reads and
// parses the change file and the committed config once, lets every step edit
// that one parsed copy, then serializes and writes the change file once.
// Nothing reaches the disk before that single write, so a failure before it
// leaves the change file untouched.
func (e *Editor) loadStaged(contextPath []string, loaded *config.Tree, replace bool) error {
	if e.session == nil {
		return e.loadApply(contextPath, loaded, replace)
	}
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return fmt.Errorf("write-through lock: %w", err)
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths
	guard.SetModifier(e.session.ID)

	stage := &loadStage{guard: guard, changePath: ChangePath(e.originalPath, e.session.User)}
	e.loadStage = stage
	err = e.loadApply(contextPath, loaded, replace)
	e.loadStage = nil
	if err != nil {
		return err
	}
	return stage.flush(e.schema)
}

// loadApply checks the context against the schema before the first edit, so
// a refused context writes nothing, then walks loaded against the current tree.
func (e *Editor) loadApply(contextPath []string, loaded *config.Tree, replace bool) error {
	path := slices.Clip(slices.Clone(contextPath))
	if len(path) == 0 {
		return e.loadTree(path, e.tree, loaded, replace)
	}
	var tb textbuf.Buffer
	node := e.schema.LookupTokenPath(path)
	if node == nil {
		return fmt.Errorf("load: unknown context path: %s", tb.Join(path, " ").String())
	}
	//exhaustive:ignore // A load lands in a container or a list entry; every other kind is refused below.
	switch node.Kind() {
	case config.NodeContainer, config.NodeList:
	default:
		return fmt.Errorf("load: context %s is not a container or list entry", tb.Join(path, " ").String())
	}
	current := e.WalkPath(path)
	if current == nil && node.Kind() == config.NodeList {
		if err := e.CreateEntry(path); err != nil {
			return fmt.Errorf("load: create %s: %w", tb.Join(path, " ").String(), err)
		}
	}
	return e.loadTree(path, current, loaded, replace)
}

// loadTree applies the difference between current (nil when the node does not
// exist yet) and want at path. Recursion is bounded by the schema's nesting
// depth: each call descends one container or list entry of a parsed tree.
func (e *Editor) loadTree(path []string, current, want *config.Tree, replace bool) error {
	if current == nil {
		current = config.NewTree()
	}
	if replace {
		if err := e.loadDeletes(path, current, want); err != nil {
			return err
		}
	}
	if err := e.loadLeaves(path, current, want, replace); err != nil {
		return err
	}
	for _, name := range want.ContainerNames() {
		child := append(path, name) //nolint:gocritic // path is clipped, so append copies.
		if err := e.loadTree(slices.Clip(child), current.GetContainer(name), want.GetContainer(name), replace); err != nil {
			return err
		}
		if err := e.loadNodeState(child, current.GetContainer(name), want.GetContainer(name), replace); err != nil {
			return err
		}
	}
	for _, name := range want.ListNames() {
		have := current.GetList(name)
		for _, key := range want.ListKeys(name) {
			entry := slices.Clip(append(path, name, key)) //nolint:gocritic // path is clipped, so append copies.
			if have[key] == nil {
				if err := e.CreateEntry(entry); err != nil {
					return loadError(entry, err)
				}
			}
			wantEntry := want.GetList(name)[key]
			if err := e.loadTree(entry, have[key], wantEntry, replace); err != nil {
				return err
			}
			if err := e.loadNodeState(entry, have[key], wantEntry, replace); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadLeaves sets each leaf and adds each leaf-list member want holds and
// current lacks, then applies the leaf inactive markers.
func (e *Editor) loadLeaves(path []string, current, want *config.Tree, replace bool) error {
	multi := want.MultiValueNames()
	for _, name := range want.Values() {
		if slices.Contains(multi, name) {
			continue // A leaf-list's members are applied below, one by one.
		}
		value, _ := want.Get(name)
		if have, ok := current.Get(name); ok && have == value {
			continue
		}
		if err := e.SetValue(path, name, value); err != nil {
			return loadError(append(slices.Clip(path), name), err)
		}
	}
	for _, name := range multi {
		if !e.isValueOrArrayLeaf(path, name) {
			var tb textbuf.Buffer
			return fmt.Errorf("load: %s holds several values in a leaf kind load cannot apply member by member", tb.Join(path, " ").Byte(' ').Str(name).String())
		}
		have := current.GetMultiValues(name)
		for _, member := range want.GetMultiValues(name) {
			if slices.Contains(have, member) {
				continue
			}
			if err := e.SetValue(path, name, member); err != nil {
				return loadError(append(slices.Clip(path), name, member), err)
			}
		}
	}
	for _, name := range slices.Concat(want.Values(), multi) {
		if err := e.loadLeafState(path, name, current.IsLeafInactive(name), want.IsLeafInactive(name), replace); err != nil {
			return err
		}
	}
	return nil
}

// loadLeafState deactivates a leaf loaded as inactive; a replace also
// activates a leaf loaded as active that the configuration holds inactive.
func (e *Editor) loadLeafState(path []string, name string, wasInactive, wantInactive, replace bool) error {
	if wasInactive == wantInactive {
		return nil
	}
	if wantInactive {
		return loadErrorIf(append(slices.Clip(path), name), e.DeactivateLeaf(path, name))
	}
	if !replace {
		return nil // A merge never clears a marker the input does not name.
	}
	return loadErrorIf(append(slices.Clip(path), name), e.ActivateLeaf(path, name))
}

// loadNodeState does for a container or list entry what loadLeafState does
// for a leaf.
func (e *Editor) loadNodeState(path []string, current, want *config.Tree, replace bool) error {
	wasInactive := current != nil && current.IsInactive()
	wantInactive := want.IsInactive()
	if wasInactive == wantInactive {
		return nil
	}
	if wantInactive {
		return loadErrorIf(path, e.DeactivatePath(path))
	}
	if !replace {
		return nil // A merge never clears a marker the input does not name.
	}
	return loadErrorIf(path, e.ActivatePath(path))
}

// loadDeletes removes what current holds at path and want omits: a leaf, a
// leaf-list member, a container, a whole list, or one list entry.
func (e *Editor) loadDeletes(path []string, current, want *config.Tree) error {
	multi := current.MultiValueNames()
	for _, name := range current.Values() {
		if slices.Contains(multi, name) {
			continue
		}
		if _, ok := want.Get(name); ok {
			continue
		}
		if err := e.DeleteValue(path, name); err != nil {
			return loadError(append(slices.Clip(path), name), err)
		}
	}
	for _, name := range multi {
		keep := want.GetMultiValues(name)
		for _, member := range current.GetMultiValues(name) {
			if slices.Contains(keep, member) {
				continue
			}
			if err := e.deleteLeafListValue(path, name, member); err != nil {
				return loadError(append(slices.Clip(path), name, member), err)
			}
		}
	}
	for _, name := range current.ContainerNames() {
		if want.GetContainer(name) != nil {
			continue
		}
		if err := e.DeleteContainer(path, name); err != nil {
			return loadError(append(slices.Clip(path), name), err)
		}
	}
	for _, name := range current.ListNames() {
		keep := want.GetList(name)
		if len(keep) == 0 {
			if err := e.DeleteList(path, name); err != nil {
				return loadError(append(slices.Clip(path), name), err)
			}
			continue
		}
		for _, key := range current.ListKeys(name) {
			if keep[key] != nil {
				continue
			}
			if err := e.DeleteListEntry(path, name, key); err != nil {
				return loadError(append(slices.Clip(path), name, key), err)
			}
		}
	}
	return nil
}

// loadError names the node a load step failed on.
func loadError(path []string, err error) error {
	var tb textbuf.Buffer
	return fmt.Errorf("load %s: %w", tb.Join(path, " ").String(), err)
}

// loadErrorIf is loadError for a step whose error may be nil.
func loadErrorIf(path []string, err error) error {
	if err == nil {
		return nil
	}
	return loadError(path, err)
}

// draftLock acquires the write-through lock, or answers the running load's
// stage, which already holds it: the store's lock is not re-entrant.
func (e *Editor) draftLock() (storage.WriteGuard, error) {
	if e.loadStage != nil {
		return e.loadStage, nil
	}
	return e.store.AcquireLock(e.originalPath)
}

// probeTree answers a tree a write-through step may create a path in to prove
// the path valid. Outside a load that is a clone, so a refused path leaves the
// tree as it was. Inside a load it is the tree itself: a refused step fails
// the whole load, which restores its snapshot, and a clone per step would make
// the load's cost grow with the square of its size.
func (e *Editor) probeTree() *config.Tree {
	if e.loadStage != nil {
		return e.tree
	}
	return e.tree.Clone()
}

// errLoadStage refuses a guard operation a write-through step never makes
// while a load is staged, so a step that starts making one fails the load
// rather than reaching the disk behind the stage.
var errLoadStage = errors.New("not available while a load is staged")

// loadStage is the WriteGuard every write-through step of a session load
// uses. It holds the load's real guard and the session's change file, read and
// parsed on first use; every step edits that parsed copy in place
// (Editor.openChangeFile, Editor.writeChangeFile), and flush serializes and
// writes it once. It also reads the committed config once for every step's
// Previous value. A direct write through the guard is refused, so the flush is
// the load's only disk write.
// Not safe for concurrent use: one load owns it.
type loadStage struct {
	guard      storage.WriteGuard
	changePath string

	// opened is set once tree, meta and ops hold the parsed change file.
	opened bool
	tree   *config.Tree
	meta   *config.MetaTree
	ops    []config.StructuralOp
	// written is set once a step recorded an edit, so flush has a file to write.
	written bool

	// committedRead is set once committed holds readCommittedTree's answer,
	// which may be nil: a committed config that does not read has no Previous.
	committedRead bool
	committed     *config.Tree
}

var _ storage.WriteGuard = (*loadStage)(nil)

// changeFile answers the staged change file, reading and parsing it on the
// first call. A change path other than the session's own is refused: a load
// edits one change file.
func (s *loadStage) changeFile(e *Editor, changePath string) (*config.Tree, *config.MetaTree, []config.StructuralOp, error) {
	if changePath != s.changePath {
		return nil, nil, nil, fmt.Errorf("change file %s: %w", changePath, errLoadStage)
	}
	if !s.opened {
		tree, meta, ops, err := e.readChangeFile(s.guard, changePath)
		if err != nil {
			return nil, nil, nil, err
		}
		s.tree, s.meta, s.ops, s.opened = tree, meta, ops, true
	}
	return s.tree, s.meta, s.ops, nil
}

// record keeps a step's edited change file until flush.
func (s *loadStage) record(changePath string, tree *config.Tree, meta *config.MetaTree, ops []config.StructuralOp) error {
	if changePath != s.changePath {
		return fmt.Errorf("change file %s: %w", changePath, errLoadStage)
	}
	s.tree, s.meta, s.ops, s.written = tree, meta, ops, true
	return nil
}

// committedTree answers the committed config, read once per load.
func (s *loadStage) committedTree(e *Editor) *config.Tree {
	if !s.committedRead {
		s.committed, s.committedRead = e.readCommittedTreeFrom(s.guard), true
	}
	return s.committed
}

// flush serializes and writes the staged change file, if any step edited it.
func (s *loadStage) flush(schema *config.Schema) error {
	if !s.written {
		return nil
	}
	output := config.SerializeChangeFile(s.tree, s.meta, s.ops, schema)
	if err := s.guard.WriteFile(s.changePath, []byte(output), 0o600); err != nil {
		return fmt.Errorf("write-through write: %w", err)
	}
	return nil
}

// ReadFile reads through the real guard: the change file and the committed
// config are served parsed by changeFile and committedTree, never as bytes.
func (s *loadStage) ReadFile(name string) ([]byte, error) {
	return s.guard.ReadFile(name)
}

// WriteFile is refused; see errLoadStage.
func (s *loadStage) WriteFile(name string, _ []byte, _ fs.FileMode) error {
	return fmt.Errorf("write %s: %w", name, errLoadStage)
}

// Has answers through the real guard.
func (s *loadStage) Has(name string) bool {
	return s.guard.Has(name)
}

// Release is a no-op: loadStaged releases the real guard after the flush.
func (s *loadStage) Release() error { return nil }

// SetModifier is a no-op: loadStaged set the modifier on the real guard.
func (s *loadStage) SetModifier(string) {}

// Remove is refused; see errLoadStage.
func (s *loadStage) Remove(name string) error {
	return fmt.Errorf("remove %s: %w", name, errLoadStage)
}

// List is refused; see errLoadStage.
func (s *loadStage) List(name string) ([]string, error) {
	return nil, fmt.Errorf("list %s: %w", name, errLoadStage)
}

// WriteVersion is refused; see errLoadStage.
func (s *loadStage) WriteVersion(name string, _ []byte, _ time.Time) error {
	return fmt.Errorf("write version %s: %w", name, errLoadStage)
}

// ReadKey is refused; see errLoadStage.
func (s *loadStage) ReadKey(key string) ([]byte, error) {
	return nil, fmt.Errorf("read key %s: %w", key, errLoadStage)
}

// ListKeys is refused; see errLoadStage.
func (s *loadStage) ListKeys(prefix string) ([]string, error) {
	return nil, fmt.Errorf("list keys %s: %w", prefix, errLoadStage)
}

// WriteKey is refused; see errLoadStage.
func (s *loadStage) WriteKey(key string, _ []byte) error {
	return fmt.Errorf("write key %s: %w", key, errLoadStage)
}

// RemoveKey is refused; see errLoadStage.
func (s *loadStage) RemoveKey(key string) error {
	return fmt.Errorf("remove key %s: %w", key, errLoadStage)
}

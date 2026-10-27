// Design: docs/architecture/config/change-file-structural-ops.md -- owned entry creation.
// Related: editor_draft.go -- recording and replay.
// Related: editor_commit.go -- conflicts and recoverable discard.

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// creationPlan keeps recorded intent separate from its replay. Moves of entries
// created by this session become creations at their final addresses. The original
// records remain untouched: discarding a rename can still restore its creation.
type creationPlan struct {
	ops            []config.StructuralOp
	createdRenames []createdRename
}

type createdRename struct {
	index int
	op    config.StructuralOp
}

func planCreations(schema *config.Schema, ops []config.StructuralOp) (creationPlan, error) {
	if !slices.ContainsFunc(ops, func(op config.StructuralOp) bool {
		return op.Type == config.StructuralOpCreateEntry
	}) {
		return creationPlan{ops: ops}, nil
	}
	if !slices.ContainsFunc(ops, func(op config.StructuralOp) bool {
		return op.Type == config.StructuralOpRename || op.Type == config.StructuralOpCopyEntry
	}) {
		return creationPlan{ops: ops}, nil
	}
	plan := creationPlan{ops: make([]config.StructuralOp, 0, len(ops))}
	for index, op := range ops {
		if op.Type != config.StructuralOpRename && op.Type != config.StructuralOpCopyEntry {
			plan.ops = append(plan.ops, op)
			continue
		}
		source, err := structuralEntryPath(schema, op, op.OldKey)
		if err != nil {
			return creationPlan{}, err
		}
		destination, err := structuralEntryPath(schema, op, op.NewKey)
		if err != nil {
			return creationPlan{}, err
		}
		owned := false
		for _, previous := range plan.ops {
			if previous.Type == config.StructuralOpCreateEntry && previous.SourcePath(schema) == source {
				owned = true
			}
			if previous.Type == config.StructuralOpDeleteEntry && previous.SourcePath(schema) == source {
				owned = false
			}
		}
		if op.Type == config.StructuralOpCopyEntry {
			// A new source has no committed subtree to copy. Copy its creation
			// program instead; pending leaf values are mirrored by writeThroughCopy.
			var copied []config.StructuralOp
			for _, previous := range plan.ops {
				// Member operations are snapshotted when the copy is recorded;
				// later source edits may rebase or remove their original records.
				if isMemberOp(previous) {
					continue
				}
				within, err := config.EditPathHasPrefix(schema, previous.SourcePath(schema), source)
				if err != nil {
					return creationPlan{}, err
				}
				if !within {
					continue
				}
				rebased, err := rebaseCreationOperation(schema, previous, source, destination)
				if err != nil {
					return creationPlan{}, err
				}
				copied = append(copied, rebased)
			}
			if !owned {
				copyOp, before, err := copyBeforeRenames(schema, plan.ops, op)
				if err != nil {
					return creationPlan{}, err
				}
				plan.ops = slices.Insert(plan.ops, before, copyOp)
			}
			plan.ops = append(plan.ops, copied...)
			continue
		}
		// An existing ancestor is still renamed against committed state.
		// Move the complete descendant program afterward, in its original
		// order: moving only creations would resurrect deleted descendants.
		var moved []config.StructuralOp
		kept := plan.ops[:0]
		for _, previous := range plan.ops {
			within, err := config.EditPathHasPrefix(schema, previous.SourcePath(schema), source)
			if err != nil {
				return creationPlan{}, err
			}
			if within {
				rebased, err := rebaseCreationOperation(schema, previous, source, destination)
				if err != nil {
					return creationPlan{}, err
				}
				moved = append(moved, rebased)
			} else {
				kept = append(kept, previous)
			}
		}
		plan.ops = kept
		if owned {
			plan.createdRenames = append(plan.createdRenames, createdRename{index: index, op: op})
		} else {
			// Recorded history remains intact, but replay and stale checks
			// need the original committed source and the final destination.
			// Descendant operations have already moved to that destination.
			merged := false
			for i := range plan.ops {
				previous := &plan.ops[i]
				if previous.Type != config.StructuralOpRename || previous.SessionKey() != op.SessionKey() {
					continue
				}
				previousDestination, err := structuralEntryPath(schema, *previous, previous.NewKey)
				if err != nil {
					return creationPlan{}, err
				}
				if previousDestination == source {
					previous.NewKey = op.NewKey
					merged = true
					break
				}
			}
			if !merged {
				plan.ops = append(plan.ops, op)
			}
		}
		plan.ops = append(plan.ops, moved...)
	}
	return plan, nil
}

func creationReplayOps(schema *config.Schema, ops []config.StructuralOp) ([]config.StructuralOp, error) {
	plan, err := planCreations(schema, ops)
	if err != nil {
		return nil, err
	}
	identity := func(op config.StructuralOp) bool {
		return op.Type == config.StructuralOpRename && op.OldKey == op.NewKey
	}
	first := slices.IndexFunc(plan.ops, identity)
	if first < 0 {
		return plan.ops, nil
	}
	replay := make([]config.StructuralOp, 0, len(plan.ops)-1)
	replay = append(replay, plan.ops[:first]...)
	for _, op := range plan.ops[first+1:] {
		if !identity(op) {
			replay = append(replay, op)
		}
	}
	return replay, nil
}

// copyBeforeRenames lowers an intermediate-name reader to the original source
// before its producing moves. Its descendant snapshot is replayed separately,
// so later source changes cannot alter the copy-time effects.
func copyBeforeRenames(schema *config.Schema, ops []config.StructuralOp, op config.StructuralOp) (config.StructuralOp, int, error) {
	source, err := structuralEntryTokens(schema, op, op.OldKey)
	if err != nil {
		return op, 0, err
	}
	destination, err := structuralEntryTokens(schema, op, op.NewKey)
	if err != nil {
		return op, 0, err
	}
	before := len(ops)
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].Type != config.StructuralOpRename {
			continue
		}
		from, err := structuralEntryTokens(schema, ops[i], ops[i].NewKey)
		if err != nil {
			return op, 0, err
		}
		to, err := structuralEntryTokens(schema, ops[i], ops[i].OldKey)
		if err != nil {
			return op, 0, err
		}
		rebasedSource, err := config.RebaseEditTokens(schema, source, from, to)
		if err != nil {
			return op, 0, err
		}
		rebasedDestination, err := config.RebaseEditTokens(schema, destination, from, to)
		if err != nil {
			return op, 0, err
		}
		if !slices.Equal(source, rebasedSource) || !slices.Equal(destination, rebasedDestination) {
			source, destination = rebasedSource, rebasedDestination
			before = i
		}
	}
	if before == len(ops) {
		return op, before, nil
	}
	sourceEntry, ok, err := config.WalkListEntryPath(schema, source)
	if err != nil {
		return op, 0, err
	}
	if !ok {
		return op, 0, fmt.Errorf("copy source is not an entry: %s", source)
	}
	targetEntry, ok, err := config.WalkListEntryPath(schema, destination)
	if err != nil {
		return op, 0, err
	}
	if !ok || sourceEntry.ListName != targetEntry.ListName || !slices.Equal(sourceEntry.ParentPath, targetEntry.ParentPath) {
		return op, 0, fmt.Errorf("copy endpoints do not share a list: %s to %s", source, destination)
	}
	op.ParentPath = sourceEntry.ParentPath
	op.ListName = sourceEntry.ListName
	op.OldKey = sourceEntry.Address.Key
	op.NewKey = targetEntry.Address.Key
	return op, before, nil
}

// entryMove describes one recorded address transition, not a persisted replay
// cursor. Draft replay locates that lineage in the actual materialized tree.
type entryMove struct {
	source      config.EditTokens
	destination config.EditTokens
}

// prepareDraftReplay separates committed intent from effects still needed in
// the shared draft. It never moves an entry back through an old address: that
// address may now belong to another session.
func (e *Editor) prepareDraftReplay(guard storage.WriteGuard, tree *config.Tree, meta *config.MetaTree, ops []config.StructuralOp) ([]config.StructuralOp, []config.SessionEntry, error) {
	plan, err := planCreations(e.schema, ops)
	if err != nil {
		return nil, nil, err
	}
	var history []entryMove
	nextCreated := 0
	for i, op := range ops {
		if nextCreated < len(plan.createdRenames) && plan.createdRenames[nextCreated].index == i {
			nextCreated++
			continue
		}
		if op.Type != config.StructuralOpRename {
			continue
		}
		source, err := structuralEntryTokens(e.schema, op, op.OldKey)
		if err != nil {
			return nil, nil, err
		}
		destination, err := structuralEntryTokens(e.schema, op, op.NewKey)
		if err != nil {
			return nil, nil, err
		}
		history = append(history, entryMove{source: source, destination: destination})
	}
	if len(history) == 0 {
		saved, err := applyStructuralOpsToMeta(meta, e.schema, ops, true)
		return plan.ops, saved, err
	}
	var owners []config.PendingChange
	appendOwners := func(pending *config.MetaTree) {
		for _, sessionID := range pending.AllSessions() {
			for _, entry := range pending.SessionEntries(e.schema, sessionID) {
				owners = append(owners, config.PendingChangeFromSessionEntry(entry))
			}
		}
	}
	appendOwners(meta)
	for _, path := range e.listChangeFiles(guard) {
		_, pending, pendingOps, err := e.readChangeFile(guard, path)
		if err != nil {
			return nil, nil, err
		}
		appendOwners(pending)
		for _, op := range pendingOps {
			owners = append(owners, op.PendingChange(e.schema))
		}
	}
	replay := make([]config.StructuralOp, 0, len(plan.ops))
	var applied []entryMove
	for _, op := range plan.ops {
		if op.Type != config.StructuralOpRename && op.Type != config.StructuralOpCopyEntry {
			replay = append(replay, op)
			continue
		}
		source, err := structuralEntryTokens(e.schema, op, op.OldKey)
		if err != nil {
			return nil, nil, err
		}
		source, err = materializedEntrySource(tree, e.schema, source, history, owners, op.SessionKey())
		if err != nil {
			return nil, nil, err
		}
		// Copies already present retain their earlier snapshot, even when
		// the original source has since moved or changed.
		if op.Type == config.StructuralOpCopyEntry {
			entry, ok, err := config.WalkListEntryPath(e.schema, source)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				return nil, nil, fmt.Errorf("draft copy source is not an entry: %s", source)
			}
			currentCopy := op
			currentCopy.ParentPath = entry.ParentPath
			currentCopy.OldKey = entry.Address.Key
			destination, err := structuralEntryTokens(e.schema, currentCopy, currentCopy.NewKey)
			if err != nil {
				return nil, nil, err
			}
			independent, err := entryHasIndependentOwner(e.schema, destination, owners, op.SessionKey())
			if err != nil {
				return nil, nil, err
			}
			if walkPath(tree, e.schema, destination) != nil {
				if independent {
					return nil, nil, fmt.Errorf("draft copy destination is independently owned: %s", destination)
				}
				continue
			}
		}
		// The lookup above uses the input tree. Earlier planned moves will
		// have changed an ancestor before this operation executes.
		for _, move := range applied {
			source, err = config.RebaseEditTokens(e.schema, source, move.source, move.destination)
			if err != nil {
				return nil, nil, err
			}
		}
		entry, ok, err := config.WalkListEntryPath(e.schema, source)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("draft move source is not an entry: %s", source)
		}
		op.ParentPath = entry.ParentPath
		op.OldKey = entry.Address.Key
		if op.Type == config.StructuralOpRename {
			if op.OldKey == op.NewKey {
				continue
			}
			destination, err := structuralEntryTokens(e.schema, op, op.NewKey)
			if err != nil {
				return nil, nil, err
			}
			applied = append(applied, entryMove{source: source, destination: destination})
		}
		replay = append(replay, op)
	}
	if _, err := applyStructuralOpsToMeta(meta, e.schema, replay, true); err != nil {
		return nil, nil, err
	}
	// Existing entries and their metadata now share the effective draft
	// moves. Creator-only metadata follows its recorded moves at those
	// final ancestor addresses, without moving another owner's old entry.
	var movedSessions map[string]bool
	for _, rename := range plan.createdRenames {
		op := rename.op
		for _, move := range history {
			op.ParentPath, err = config.RebaseEditTokens(e.schema, op.ParentPath, move.source, move.destination)
			if err != nil {
				return nil, nil, err
			}
		}
		if err := rebaseCreatedMetadata(meta, e.schema, op); err != nil {
			return nil, nil, err
		}
		if movedSessions == nil {
			movedSessions = make(map[string]bool)
		}
		movedSessions[op.SessionKey()] = true
	}
	var saved []config.SessionEntry
	for sessionID := range movedSessions {
		saved = append(saved, meta.SessionEntries(e.schema, sessionID)...)
	}
	return replay, saved, nil
}

func materializedEntrySource(tree *config.Tree, schema *config.Schema, path config.EditTokens, history []entryMove, owners []config.PendingChange, sessionID string) (config.EditTokens, error) {
	original := path
	var err error
	for _, move := range slices.Backward(history) {
		original, err = config.RebaseEditTokens(schema, original, move.destination, move.source)
		if err != nil {
			return nil, err
		}
	}
	candidates := []config.EditTokens{original}
	current := original
	for _, move := range history {
		next, err := config.RebaseEditTokens(schema, current, move.source, move.destination)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(current, next) {
			candidates = append(candidates, next)
			current = next
		}
	}
	occupied := false
	for _, candidate := range slices.Backward(candidates) {
		if walkPath(tree, schema, candidate) == nil {
			continue
		}
		occupied = true
		independent, err := entryHasIndependentOwner(schema, candidate, owners, sessionID)
		if err != nil {
			return nil, err
		}
		if !independent {
			return candidate, nil
		}
	}
	if occupied {
		return nil, fmt.Errorf("draft entry lineage is independently owned: %s", path)
	}
	// A preceding creation or copy may produce this source during replay.
	// Otherwise the ordinary structural executor reports the missing entry.
	return path, nil
}

func entryHasIndependentOwner(schema *config.Schema, path config.EditTokens, owners []config.PendingChange, sessionID string) (bool, error) {
	entry, err := config.ResolveEditPath(schema, path.String())
	if err != nil {
		return false, err
	}
	for _, owner := range owners {
		if owner.SessionID == sessionID {
			continue
		}
		for _, path := range owner.ConflictPaths() {
			owned, err := config.ResolveEditPath(schema, path)
			if err != nil {
				return false, fmt.Errorf("resolve pending owner %s: %w", owner.SessionID, err)
			}
			if owned.HasPrefix(entry) || entry.HasPrefix(owned) {
				return true, nil
			}
		}
	}
	return false, nil
}

func structuralEntryTokens(schema *config.Schema, op config.StructuralOp, key string) (config.EditTokens, error) {
	path := append(slices.Clone(op.ParentPath), op.ListName)
	node, ok := schema.LookupTokenPath(path).(*config.ListNode)
	if !ok {
		return nil, fmt.Errorf("entry operation does not name a list: %s", config.FormatEditPath(path))
	}
	path = append(path, config.ListKeyTokens(node, key)...)
	return config.CanonicalEditPath(schema, path)
}

func structuralEntryPath(schema *config.Schema, op config.StructuralOp, key string) (string, error) {
	path, err := structuralEntryTokens(schema, op, key)
	return path.String(), err
}

func rebaseCreationOperation(schema *config.Schema, op config.StructuralOp, source, destination string) (config.StructuralOp, error) {
	if op.Type == config.StructuralOpCreateEntry || op.Type == config.StructuralOpDeactivatePath || op.Type == config.StructuralOpActivatePath {
		path, err := config.RebaseEditPath(schema, op.SourcePath(schema), source, destination)
		if err != nil {
			return op, err
		}
		tokens, err := config.ParseEditPath(path)
		if err != nil {
			return op, err
		}
		op.ParentPath = tokens[:len(tokens)-1]
		op.ListName = tokens[len(tokens)-1]
		return op, nil
	}
	if op.Type == config.StructuralOpDeleteEntry {
		path, err := structuralEntryPath(schema, op, op.OldKey)
		if err != nil {
			return op, err
		}
		path, err = config.RebaseEditPath(schema, path, source, destination)
		if err != nil {
			return op, err
		}
		tokens, err := config.ParseEditPath(path)
		if err != nil {
			return op, err
		}
		target, ok, err := config.WalkListEntryPath(schema, tokens)
		if err != nil {
			return op, err
		}
		if !ok {
			return op, fmt.Errorf("rebased delete does not name an entry: %s", path)
		}
		op.ParentPath = target.ParentPath
		op.ListName = target.ListName
		op.OldKey = target.Address.Key
		return op, nil
	}
	parent, err := config.RebaseEditPath(schema, op.ParentPath.String(), source, destination)
	if err != nil {
		return op, err
	}
	op.ParentPath, err = config.ParseEditPath(parent)
	return op, err
}

// Only explicitly created sources bypass the committed source check. An
// implicitly materialized ancestor can never hide another user's deletion.
func structuralOpConflicts(tree *config.Tree, schema *config.Schema, ops []config.StructuralOp) ([]Conflict, error) {
	plan, err := planCreations(schema, ops)
	if err != nil {
		return nil, err
	}
	var conflicts []Conflict
	for _, op := range plan.ops {
		conflicts = appendStructuralOpConflict(conflicts, tree, schema, op)
	}
	for _, rename := range plan.createdRenames {
		op := rename.op
		path, err := structuralEntryPath(schema, op, op.NewKey)
		if err != nil {
			return nil, err
		}
		tokens, err := config.ParseEditPath(path)
		if err != nil {
			return nil, err
		}
		if walkPath(tree, schema, tokens) != nil {
			conflicts = append(conflicts, Conflict{
				Path: path, Type: ConflictStale, MyValue: op.PendingChange(schema).Summary(schema),
				OtherValue: "rename destination already exists",
			})
		}
	}
	return conflicts, nil
}

// rebaseCreatedMetadata moves only the creator's saved leaf ownership. Another
// session may have acquired the old materialization after the rename was
// recorded; its metadata and values must stay at that source.
func rebaseCreatedMetadata(meta *config.MetaTree, schema *config.Schema, op config.StructuralOp) error {
	source, err := structuralEntryTokens(schema, op, op.OldKey)
	if err != nil {
		return err
	}
	destination, err := structuralEntryTokens(schema, op, op.NewKey)
	if err != nil {
		return err
	}
	for _, entry := range meta.SessionEntries(schema, op.SessionKey()) {
		path, err := config.RebaseEditTokens(schema, entry.Path, source, destination)
		if err != nil {
			return err
		}
		if slices.Equal(path, entry.Path) {
			continue
		}
		oldParent := walkMetaReadOnly(meta, schema, entry.Path[:len(entry.Path)-1])
		if oldParent == nil {
			return fmt.Errorf("saved creation metadata path missing: %s", entry.Path)
		}
		oldParent.RemoveSessionEntry(entry.Path[len(entry.Path)-1], op.SessionKey())
		newParent := walkOrCreateMeta(meta, schema, path[:len(path)-1])
		newParent.SetEntry(path[len(path)-1], entry.Entry)
	}
	return nil
}

// creationFootprint includes the list-entry ancestors materialized by each
// creation. Historical footprints also include sources superseded by renames.
func creationFootprint(schema *config.Schema, ops []config.StructuralOp, historical bool) (map[string]bool, error) {
	if !slices.ContainsFunc(ops, func(op config.StructuralOp) bool {
		return op.Type == config.StructuralOpCreateEntry
	}) {
		return nil, nil // A program without creations has no creation footprint.
	}
	plan, err := planCreations(schema, ops)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]bool)
	for _, op := range plan.ops {
		if op.Type == config.StructuralOpCreateEntry {
			if err := addCreationAncestors(schema, paths, op.SourcePath(schema)); err != nil {
				return nil, err
			}
			continue
		}
		if historical || (op.Type != config.StructuralOpDeleteEntry && op.Type != config.StructuralOpDeleteContainer && op.Type != config.StructuralOpDeleteList) {
			continue
		}
		for path := range paths {
			within, err := config.EditPathHasPrefix(schema, path, op.SourcePath(schema))
			if err != nil {
				return nil, err
			}
			if within {
				delete(paths, path)
			}
		}
	}
	if historical {
		for _, rename := range plan.createdRenames {
			if err := addCreationAncestors(schema, paths, rename.op.SourcePath(schema)); err != nil {
				return nil, err
			}
		}
	}
	return paths, nil
}

func addCreationAncestors(schema *config.Schema, paths map[string]bool, path string) error {
	tokens, err := config.ParseEditPath(path)
	if err != nil {
		return err
	}
	var current config.ChildProvider = schema
	canonical := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); {
		name := tokens[i]
		node := current.Get(name)
		canonical = append(canonical, name)
		i++
		switch n := node.(type) {
		case *config.ContainerNode:
			current = n
		case *config.FlexNode:
			current = n
		case *config.ListNode:
			address, err := config.ListPathStep(name, n, tokens[i:])
			if err != nil {
				return err
			}
			if address.Tokens != 0 {
				if n.HasMultiLeafKey() {
					canonical = append(canonical, config.ListKeyTokens(n, address.Key)...)
				} else {
					canonical = append(canonical, address.Key)
				}
				paths[config.FormatEditPath(canonical)] = true
			}
			i += address.Tokens
			current = n
		default:
			return fmt.Errorf("creation path does not name a list entry: %s", path)
		}
	}
	return nil
}

// pruneDiscardedCreations writes the pruned draft BEFORE the caller consumes
// its change records. Failure leaves those records available for an exact retry.
// The owner's replacement ops/meta are considered without publishing them yet.
func (e *Editor) pruneDiscardedCreations(guard storage.WriteGuard, changePath string, before, after []config.StructuralOp, afterMeta *config.MetaTree) error {
	if !slices.ContainsFunc(before, func(op config.StructuralOp) bool {
		return op.Type == config.StructuralOpCreateEntry
	}) {
		return nil
	}
	draftPath := DraftPath(e.originalPath)
	data, err := guard.ReadFile(draftPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // Unsaved changes have no materialized draft to prune.
	}
	if err != nil {
		return err
	}
	tree, meta, err := config.NewSetParser(e.schema).ParseWithMeta(string(data))
	if err != nil {
		return err
	}
	changed, err := e.pruneCreationMaterializations(guard, changePath, before, after, afterMeta, tree, meta)
	if err != nil || !changed {
		return err
	}
	return guard.WriteFile(draftPath, []byte(config.SerializeSetWithMeta(tree, meta, e.schema)), 0o600)
}

// pruneCreationMaterializations is shared by save and discard. It removes only
// historical materializations no longer required by the resulting program,
// committed configuration, or another owner.
func (e *Editor) pruneCreationMaterializations(guard storage.WriteGuard, changePath string, before, after []config.StructuralOp, afterMeta *config.MetaTree, tree *config.Tree, meta *config.MetaTree) (bool, error) {
	oldPaths, err := creationFootprint(e.schema, before, true)
	if err != nil {
		return false, err
	}
	newPaths, err := creationFootprint(e.schema, after, false)
	if err != nil {
		return false, err
	}
	for path := range newPaths {
		delete(oldPaths, path)
	}
	if len(oldPaths) == 0 {
		return false, nil
	}
	committedData, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return false, err
	}
	committed, _, err := parseConfigWithFormat(string(committedData), e.schema)
	if err != nil {
		return false, err
	}
	var owners []config.PendingChange
	appendMeta := func(pending *config.MetaTree) {
		for _, sid := range pending.AllSessions() {
			for _, entry := range pending.SessionEntries(e.schema, sid) {
				owners = append(owners, config.PendingChangeFromSessionEntry(entry))
			}
		}
	}
	appendMeta(meta)
	// List returns storage keys; ChangePath may use the original absolute
	// spelling. Compare the existing config-scoped owner identity.
	replacedUser := changeUser(e.originalPath, changePath)
	for _, path := range e.listChangeFiles(guard) {
		pending, ops := afterMeta, after
		if changeUser(e.originalPath, path) != replacedUser {
			_, pending, ops, err = e.readChangeFile(guard, path)
			if err != nil {
				return false, err
			}
		} else {
			ops, err = creationReplayOps(e.schema, ops)
			if err != nil {
				return false, err
			}
		}
		appendMeta(pending)
		for _, op := range ops {
			owners = append(owners, op.PendingChange(e.schema))
		}
	}
	changed := false
	for path := range oldPaths {
		tokens, err := config.ParseEditPath(path)
		if err != nil {
			return false, err
		}
		if walkPath(committed, e.schema, tokens) != nil {
			continue
		}
		owned, err := creationHasOwner(e.schema, path, owners)
		if err != nil {
			return false, err
		}
		if owned {
			continue
		}
		target, ok, err := config.WalkListEntryPath(e.schema, tokens)
		if err != nil {
			return false, err
		}
		if !ok || target.End != len(tokens) {
			return false, fmt.Errorf("discard creation %s: path no longer names a list entry", path)
		}
		if parent := walkPath(tree, e.schema, target.ParentPath); parent != nil {
			parent.RemoveListEntry(target.ListName, target.Address.Key)
			changed = true
		}
	}
	return changed, nil
}

func creationHasOwner(schema *config.Schema, path string, owners []config.PendingChange) (bool, error) {
	entry, err := config.ResolveEditPath(schema, path)
	if err != nil {
		return false, err
	}
	for _, owner := range owners {
		for _, ownedPath := range owner.ConflictPaths() {
			ownerPath, err := config.ResolveEditPath(schema, ownedPath)
			if err != nil {
				return false, fmt.Errorf("resolve pending owner %s: %w", owner.SessionID, err)
			}
			if ownerPath.HasPrefix(entry) {
				return true, nil
			}
		}
	}
	return false, nil
}

// entryCreation reports the value-less set emitted only by create-entry.
func entryCreation(change config.PendingChange) bool {
	return change.Kind == config.PendingChangeSet && change.Value == "" && change.Member == ""
}

// structuralPreSaveConflicts refuses a stale rename before SaveDraft tries to
// materialize its source. The locked commit scan repeats this check afterward;
// this early check changes no files and cannot consume session ownership.
func (e *Editor) structuralPreSaveConflicts() ([]Conflict, error) {
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return nil, err
	}
	defer guard.Release() //nolint:errcheck // Best effort unlock on all paths.
	_, _, ops, err := e.readChangeFile(guard, ChangePath(e.originalPath, e.session.User))
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(ops, func(op config.StructuralOp) bool {
		return op.Type == config.StructuralOpRename && op.SessionKey() == e.session.ID
	}) {
		return nil, nil // No rename needs a committed source before materialization.
	}
	ops = filterStructuralOps(ops, e.session.ID)
	data, err := guard.ReadFile(e.originalPath)
	if err != nil {
		return nil, err
	}
	tree, _, err := parseConfigWithFormat(string(data), e.schema)
	if err != nil {
		return nil, err
	}
	return structuralOpConflicts(tree, e.schema, ops)
}

// creationChangesConflict resolves both ownership addresses before answering,
// including different-looking paths. Unknown schema paths cannot masquerade
// as independent edits and pass through a normal or forced commit.
func creationChangesConflict(schema *config.Schema, created, other config.PendingChange) (bool, error) {
	if !entryCreation(created) {
		created, other = other, created
	}
	entry, err := config.ResolveEditPath(schema, created.Path)
	if err != nil {
		return false, err
	}
	for _, path := range other.ConflictPaths() {
		target, err := config.ResolveEditPath(schema, path)
		if err != nil {
			return false, err
		}
		if isEntryMove(other.Kind) {
			if entry.HasPrefix(target) || target.HasPrefix(entry) {
				return true, nil
			}
		} else if other.Kind == config.PendingChangeDelete {
			if entry.HasPrefix(target) {
				return true, nil
			}
		} else if entry.Equal(target) && (other.Kind != created.Kind || other.Value != created.Value || other.Member != "") {
			return true, nil
		}
	}
	return false, nil
}

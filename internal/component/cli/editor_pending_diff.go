// Design: docs/architecture/config/change-file-structural-ops.md -- session review.
// Related: editor_pending_snapshot.go -- durable discard audit without recovery.
package cli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// PendingReview renders only this session's intent against the current
// committed configuration. The caller MUST serialize access to the editor.
// Reading review does not rebuild its candidate or change conflict metadata.
func (e *Editor) PendingReview(path []string) (string, error) {
	if e.session == nil {
		return "", errNoSessionSet
	}
	if e.store == nil {
		return "", pendingReviewFailure(storage.ErrNoStore)
	}
	guard, err := e.store.AcquireLock(e.originalPath)
	if err != nil {
		return "", pendingReviewFailure(err)
	}
	changes, readErr := e.pendingReviewChanges(guard, path)
	releaseErr := guard.Release()
	if err := errors.Join(readErr, releaseErr); err != nil {
		return "", pendingReviewFailure(err)
	}
	return RenderPendingChanges(e.schema, changes)
}

func pendingReviewFailure(err error) error {
	return opererr.Wrap(err,
		"Your pending configuration changes could not be compared with the committed configuration.",
		"Your edits remain pending; repair the reported configuration or storage problem, then run show | compare again",
		"run show doctor")
}

// pendingReviewChanges reads committed content and the session's saved and
// unsaved ownership under one guard. Metadata and operations bound every loop.
// Previous is replaced on presentation copies only; commit retains its original
// conflict baseline in both the metadata tree and the durable change file.
func (e *Editor) pendingReviewChanges(guard storage.WriteGuard, scope []string) ([]contract.PendingChange, error) {
	data, err := storage.ReadActiveConfigWithGuard(e.store, guard, e.originalPath)
	if err != nil {
		return nil, fmt.Errorf("read committed configuration: %w", err)
	}
	committed, _, err := parseConfigWithFormat(string(data), e.schema)
	if err != nil {
		return nil, fmt.Errorf("parse committed configuration: %w", err)
	}
	_, meta, ops, err := e.readChangeFile(guard, ChangePath(e.originalPath, e.session.User))
	if err != nil {
		return nil, err
	}
	ops = filterStructuralOps(ops, e.session.ID)
	changes := e.pendingSessionChanges(meta, ops, e.session.ID)
	reordered, err := e.unchangedPlainListReorders(committed, ops, changes)
	if err != nil {
		return nil, err
	}
	var review []contract.PendingChange
	for _, change := range changes {
		selected, err := pendingReviewAtPath(e.schema, change, scope)
		if err != nil {
			return nil, err
		}
		if !selected {
			continue
		}
		elided, err := isPlainListReorderChange(e.schema, change, reordered)
		if err != nil {
			return nil, err
		}
		if elided {
			continue
		}
		if change.Member == "" {
			// Only assignment kinds carry a previous scalar value. Structural
			// intent remains one row even when its current target is absent.
			switch change.Kind {
			case config.PendingChangeSet, config.PendingChangeDelete:
				path, err := config.ParseEditPath(change.Path)
				if err != nil {
					return nil, err
				}
				original, err := committedPathOf(e.schema, ops, path)
				if err != nil {
					return nil, err
				}
				change.Previous = getValueAtPath(committed, e.schema, original)
			case config.PendingChangeRename, config.PendingChangeCopy, config.PendingChangeDeactivate, config.PendingChangeActivate:
			}
		}
		review = append(review, contract.PendingChange{
			User: change.User, Kind: contract.PendingChangeKind(change.Kind),
			Path: change.Path, Previous: change.Previous, Value: change.Value,
			OldPath: change.OldPath, NewPath: change.NewPath, Member: change.Member,
		})
	}
	return review, nil
}

// unchangedPlainListReorders finds plain (non-user-ordered) list entries that
// this session deleted and recreated with semantically identical content against
// committed. Commit still replays the pending records to preserve the recorded
// list order, while semantic comparison reports no diff.
func (e *Editor) unchangedPlainListReorders(committed *config.Tree, ops []config.StructuralOp, changes []config.PendingChange) ([]config.EditPath, error) {
	if !e.treeValid {
		return nil, nil
	}
	if e.tree == nil {
		return nil, nil
	}
	if committed == nil {
		return nil, nil
	}
	var reordered []config.EditPath
	for _, change := range changes {
		if change.Kind != config.PendingChangeDelete {
			continue
		}
		if change.Member != "" {
			continue
		}
		path, err := config.ParseEditPath(change.Path)
		if err != nil {
			return nil, err
		}
		original, err := committedPathOf(e.schema, ops, path)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(original, path) {
			continue
		}
		workingEntry, workingNode := e.walkPathWithSchemaFrom(e.tree, path)
		if workingEntry == nil {
			continue
		}
		listNode, ok := workingNode.(*config.ListNode)
		if !ok {
			continue
		}
		if listNode.UserOrdered {
			continue
		}
		committedEntry, _ := e.walkPathWithSchemaFrom(committed, original)
		if committedEntry == nil {
			continue
		}
		if !config.SubtreesEqual(workingEntry, committedEntry, listNode) {
			continue
		}
		resolved, err := config.ResolveEditPath(e.schema, change.Path)
		if err != nil {
			return nil, err
		}
		reordered = append(reordered, resolved)
	}
	return reordered, nil
}

func isPlainListReorderChange(schema *config.Schema, change config.PendingChange, reordered []config.EditPath) (bool, error) {
	if len(reordered) == 0 {
		return false, nil
	}
	switch change.Kind {
	case config.PendingChangeDelete, config.PendingChangeSet:
	default:
		return false, nil
	}
	resolved, err := config.ResolveEditPath(schema, change.Path)
	if err != nil {
		return false, err
	}
	for _, entry := range reordered {
		if resolved.HasPrefix(entry) {
			return true, nil
		}
	}
	return false, nil
}

func pendingReviewAtPath(schema *config.Schema, change config.PendingChange, scope []string) (bool, error) {
	if len(scope) == 0 {
		return true, nil
	}
	for _, path := range change.ConflictPaths() {
		overlaps, err := pathOverlaps(schema, path, config.FormatEditPath(scope))
		if err != nil {
			return false, err
		}
		if overlaps {
			return true, nil
		}
	}
	return false, nil
}

// RenderPendingChanges is the single masked path-qualified renderer for SSH
// session review, browser review and discard audit. Callers MUST supply editor
// records, preserving their typed kinds, path quoting, member and ordering.
// Raw records are validated here; an invalid record returns an operator error
// without including its values or any partial review.
func RenderPendingChanges(schema *config.Schema, changes []contract.PendingChange) (string, error) {
	var b textbuf.Buffer
	for _, change := range changes {
		path, err := pendingReviewAddress(schema, change.Path)
		if err != nil {
			return "", err
		}
		previous := config.DisplayValueAtPath(schema, path, change.Previous)
		value := config.DisplayValueAtPath(schema, path, change.Value)
		switch change.Kind {
		case contract.PendingChangeRename:
			if _, err := pendingReviewAddress(schema, change.OldPath); err != nil {
				return "", err
			}
			if _, err := pendingReviewAddress(schema, change.NewPath); err != nil {
				return "", err
			}
			b.Str("~ rename ").Str(change.OldPath).Str(" to ").Str(change.NewPath).Byte('\n')
		case contract.PendingChangeCopy:
			if _, err := pendingReviewAddress(schema, change.OldPath); err != nil {
				return "", err
			}
			if _, err := pendingReviewAddress(schema, change.NewPath); err != nil {
				return "", err
			}
			b.Str("+ copy ").Str(change.OldPath).Str(" to ").Str(change.NewPath).Byte('\n')
		case contract.PendingChangeDelete:
			b.Str("- ").Str(change.Path).Byte(' ')
			if change.Member == "" {
				b.Str(previous)
			} else {
				b.Str(config.DisplayValueAtPath(schema, path, change.Member))
			}
			b.Byte('\n')
		case contract.PendingChangeDeactivate:
			writePendingMemberDiff(&b, schema, "~ deactivate ", change, path)
		case contract.PendingChangeActivate:
			writePendingMemberDiff(&b, schema, "~ activate ", change, path)
		case contract.PendingChangeSet:
			if change.Previous != "" {
				b.Str("- ").Str(change.Path).Byte(' ').Str(previous).Byte('\n')
			}
			b.Str("+ ").Str(change.Path).Byte(' ').Str(value).Byte('\n')
		default:
			// PendingChange is an open adapter record, not a validated enum.
			return "", pendingReviewFailure(errors.New("unrecognized pending change kind"))
		}
	}
	return b.String(), nil
}

// pendingReviewAddress validates raw adapter addresses before schema masking.
// Errors omit the raw address because an invalid record can carry secret data.
func pendingReviewAddress(schema *config.Schema, address string) ([]string, error) {
	path, err := config.ParseEditPath(address)
	if err != nil {
		return nil, pendingReviewFailure(errors.New("pending change address cannot be decoded"))
	}
	if len(path) == 0 {
		return nil, pendingReviewFailure(errors.New("pending change address is empty"))
	}
	path, err = config.CanonicalEditPath(schema, path)
	if err != nil {
		return nil, pendingReviewFailure(errors.New("pending change address cannot be resolved"))
	}
	return path, nil
}

func writePendingMemberDiff(b *textbuf.Buffer, schema *config.Schema, verb string, change contract.PendingChange, path []string) {
	b.Str(verb).Str(change.Path)
	if change.Member != "" {
		b.Byte(' ').Str(config.DisplayValueAtPath(schema, path, change.Member))
	}
	b.Byte('\n')
}

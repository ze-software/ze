// Design: docs/architecture/storage-backends.md -- complete stored-config rename.
// Related: store.go -- the low-level single-entry Rename keeps its own contract.
// Related: editor_record.go -- the suffixes that identify saved editor work.

package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/ze-software/ze/pkg/zefs"
)

// RenameConfig moves a stored configuration's mirrors, version entries,
// pointers and saved editor records under one owner guard. Shared content
// objects and global command/audit history remain unchanged. Names may be
// bare or paths accepted by the store; raw storage keys are not config names.
//
// The destination must own no configuration state. An unfinished file commit
// or confirmed window refuses the rename, because its intent retains the old
// identity. A complete destination is published before any source removal.
// Failed source restoration retains that destination and reports both causes.
// Per-key durability applies; an interruption can leave both names present.
// Safe for concurrent use. The caller retains ownership of source.
func RenameConfig(source Storage, oldName, newName string) (err error) {
	owned, ok := ownedStore(source)
	if !ok {
		return fmt.Errorf("rename configuration: unsupported storage %T: %w", source, errors.ErrUnsupported)
	}
	if owned == nil {
		return errors.New("rename configuration: no store handle")
	}
	old, err := StoredConfigName(source, oldName)
	if err != nil {
		return err
	}
	new, err := StoredConfigName(source, newName)
	if err != nil {
		return err
	}
	if old == new {
		return fmt.Errorf("rename configuration %s: destination is the source: %w", old, fs.ErrExist)
	}
	held, err := source.AcquireLock(oldName)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(held, err) }()
	guard, ok := held.(*guard)
	if !ok {
		return fmt.Errorf("rename configuration: unsupported write guard %T: %w", held, errors.ErrUnsupported)
	}
	plan, err := prepareConfigRename(owned, guard, old, new)
	if err != nil {
		return err
	}
	return plan.publish(guard)
}

// RenameConfig is the owner's complete-config operation, distinct from the
// Storage contract's low-level mirror rename. Both public entry points use it.
func (s *store) RenameConfig(oldName, newName string) error {
	return RenameConfig(s, oldName, newName)
}

type configRenameEntry struct {
	source string
	target string
	data   []byte
	meta   FileMeta
}

// configRenamePlan is admitted only after every source byte has been read and
// every destination collision checked. Entries are bounded by the store's
// finite key census; snapshots own their bytes even on a remapped blob.
type configRenamePlan struct {
	oldName string
	newName string
	entries []configRenameEntry
}

func prepareConfigRename(owned *store, held *guard, oldName, newName string) (*configRenamePlan, error) {
	keys, err := held.ListKeys(zefs.KeyFileVersion.Prefix())
	if err != nil {
		return nil, err
	}
	pointers, err := held.ListKeys(zefs.KeyConfigActive.Prefix())
	if err != nil {
		return nil, err
	}
	keys = append(keys, pointers...)
	slices.Sort(keys)
	present := make(map[string]bool, len(keys))
	explicit := make(map[string]bool)
	for _, key := range keys {
		present[key] = true
		if _, name, version := historyEntry(key); version {
			explicit[name] = true
		}
		if tail, metadata := strings.CutPrefix(key, zefs.KeyConfigActive.Prefix()); metadata {
			if name, _, named := strings.Cut(tail, "/"); named {
				if name != "" {
					explicit[name] = true
				}
			}
		}
	}
	if !present[zefs.KeyFileActive.Key(oldName)] {
		if !present[zefs.KeyConfigActive.Key(oldName)] {
			return nil, fmt.Errorf("rename configuration %s: no active mirror or pointer: %w", oldName, fs.ErrNotExist)
		}
	}
	for _, key := range []string{zefs.KeyConfigFileCommit.Key(oldName), zefs.KeyConfigConfirmPending.Key(oldName)} {
		if present[key] {
			return nil, fmt.Errorf("rename configuration %s: unfinished identity transaction at %s; finish or recover it before renaming", oldName, key)
		}
	}
	for _, key := range keys {
		if _, owned := configRenameTarget(key, newName, newName, explicit); owned {
			return nil, fmt.Errorf("rename configuration %s to %s: destination owns %s: %w", oldName, newName, key, fs.ErrExist)
		}
	}
	plan := &configRenamePlan{oldName: oldName, newName: newName}
	for _, key := range keys {
		target, matches := configRenameTarget(key, oldName, newName, explicit)
		if !matches {
			continue
		}
		if present[target] {
			return nil, fmt.Errorf("rename configuration %s to %s: destination owns %s: %w", oldName, newName, target, fs.ErrExist)
		}
		data, err := held.ReadKey(key)
		if err != nil {
			return nil, fmt.Errorf("rename configuration %s: read %s: %w", oldName, key, err)
		}
		owned.mu.RLock()
		meta := owned.metas[key]
		owned.mu.RUnlock()
		plan.entries = append(plan.entries, configRenameEntry{
			source: key, target: target, data: slices.Clone(data), meta: meta,
		})
	}
	return plan, nil
}

// configRenameTarget follows the existing key grammar, including editor-owned
// siblings. Version or identity metadata makes an editor-like name independent.
func configRenameTarget(key, oldName, newName string, explicit map[string]bool) (string, bool) {
	prefix := zefs.KeyConfigActive.Prefix() + oldName + "/"
	if tail, matches := strings.CutPrefix(key, prefix); matches {
		return zefs.KeyConfigActive.Prefix() + newName + "/" + tail, true
	}
	rest, file := strings.CutPrefix(key, zefs.KeyFileVersion.Prefix())
	if !file {
		return "", false
	}
	namespace, name, found := strings.Cut(rest, "/")
	if !found {
		return "", false
	}
	if strings.Contains(name, "/") {
		return "", false
	}
	if name == oldName {
		return zefs.KeyFileVersion.Prefix() + namespace + "/" + newName, true
	}
	if explicit[name] {
		return "", false
	}
	if name == oldName+EditorDraftSuffix {
		return zefs.KeyFileVersion.Prefix() + namespace + "/" + newName + EditorDraftSuffix, true
	}
	if account, matches := strings.CutPrefix(name, oldName+EditorChangeSeparator); matches {
		if account != "" {
			return zefs.KeyFileVersion.Prefix() + namespace + "/" + newName + EditorChangeSeparator + account, true
		}
	}
	return "", false
}

func (plan *configRenamePlan) publish(held *guard) error {
	for index, entry := range plan.entries {
		if err := held.write(entry.target, entry.data, entry.meta.ModTime); err != nil {
			// The source has not been touched. Include the attempted key because
			// a publication can return an error after the bytes have landed.
			return errors.Join(fmt.Errorf("rename configuration %s to %s: copy %s: %w",
				plan.oldName, plan.newName, entry.target, err), removeConfigRenameTargets(held, plan.entries[:index+1]))
		}
		held.pending[entry.target] = entry.meta
	}
	// Remove pointers before their referenced history, and the active mirror
	// last. An interruption retains the complete destination and never leaves
	// a source pointer naming an entry this cleanup already removed.
	slices.SortFunc(plan.entries, func(a, b configRenameEntry) int {
		metaPrefix := zefs.KeyConfigActive.Prefix()
		aMeta, bMeta := strings.HasPrefix(a.source, metaPrefix), strings.HasPrefix(b.source, metaPrefix)
		if aMeta != bMeta {
			if aMeta {
				return -1
			}
			return 1
		}
		mirror := zefs.KeyFileActive.Key(plan.oldName)
		if a.source == mirror {
			if b.source != mirror {
				return 1
			}
		}
		if b.source == mirror {
			if a.source != mirror {
				return -1
			}
		}
		return strings.Compare(a.source, b.source)
	})
	for _, entry := range plan.entries {
		if err := held.remove(entry.source); err != nil {
			failure := fmt.Errorf("rename configuration %s to %s: remove %s: %w", plan.oldName, plan.newName, entry.source, err)
			if restoreErr := restoreConfigRenameSource(held, plan.entries); restoreErr != nil {
				return errors.Join(failure, fmt.Errorf("source restoration failed; complete configuration and saved work retained as %s: %w", plan.newName, restoreErr))
			}
			return errors.Join(failure, removeConfigRenameTargets(held, plan.entries))
		}
	}
	return nil
}

func restoreConfigRenameSource(held *guard, entries []configRenameEntry) error {
	var failures []error
	for _, entry := range entries {
		if err := held.write(entry.source, entry.data, entry.meta.ModTime); err != nil {
			failures = append(failures, fmt.Errorf("restore source %s: %w", entry.source, err))
			continue
		}
		// The original metadata remains unpublished under the guard. Successful
		// compensation is a net-unchanged source and records no write event.
		delete(held.pending, entry.source)
		delete(held.removed, entry.source)
	}
	return errors.Join(failures...)
}

func removeConfigRenameTargets(held *guard, entries []configRenameEntry) error {
	var failures []error
	for _, entry := range entries {
		if err := held.remove(entry.target); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				failures = append(failures, fmt.Errorf("remove unaccepted destination %s: %w", entry.target, err))
			}
		}
	}
	return errors.Join(failures...)
}

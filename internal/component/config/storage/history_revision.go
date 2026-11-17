// Design: docs/architecture/storage-backends.md -- one indexed history order on every command surface.
// Related: store.go -- ListVersions uses the same key-to-version ordering.

package storage

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/ze-software/ze/pkg/zefs"
)

// ReadRollbackRevision selects the one-based revision in ListVersions order
// and verifies its content object under one owner guard. source MUST be an
// open writable owner handle. The returned bytes belong to the caller.
// Pending confirmation or file-publication intents refuse this pre-write read.
// General read-only history inspection remains ListVersions and ReadVersion.
// Safe for concurrent use; the caller retains ownership of source.
func ReadRollbackRevision(source Storage, name string, revision int) (selected VersionInfo, data []byte, err error) {
	base, err := StoredConfigName(source, name)
	if err != nil {
		return VersionInfo{}, nil, err
	}
	held, err := source.AcquireLock(name)
	if err != nil {
		return VersionInfo{}, nil, err
	}
	defer func() { err = releaseGuard(held, err) }()
	if err := admitConfigRollbackLocked(held, base); err != nil {
		return VersionInfo{}, nil, err
	}
	return readRollbackRevisionLocked(held, base, revision)
}

func (s *store) ReadRollbackRevision(name string, revision int) (VersionInfo, []byte, error) {
	return ReadRollbackRevision(s, name, revision)
}

func readRollbackRevisionLocked(held WriteGuard, name string, revision int) (VersionInfo, []byte, error) {
	keys, err := held.ListKeys(zefs.KeyFileVersion.Prefix())
	if err != nil {
		return VersionInfo{}, nil, err
	}
	versions, err := versionsForName(keys, name)
	if err != nil {
		return VersionInfo{}, nil, err
	}
	if revision < 1 {
		return VersionInfo{}, nil, fmt.Errorf("revision %d not found (have %d revisions): %w", revision, len(versions), fs.ErrNotExist)
	}
	if revision > len(versions) {
		return VersionInfo{}, nil, fmt.Errorf("revision %d not found (have %d revisions): %w", revision, len(versions), fs.ErrNotExist)
	}
	selected := versions[revision-1]
	data, err := readVersionEntry(held.ReadKey, selected.Path)
	if err != nil {
		return VersionInfo{}, nil, fmt.Errorf("read rollback revision %d of %s: %w", revision, name, err)
	}
	return selected, slices.Clone(data), nil
}

// versionsForName is the existing ListVersions order, shared rather than
// independently sorting history in an adapter. keys is the store's finite
// file-key census and bounds this walk and its result.
func versionsForName(keys []string, name string) ([]VersionInfo, error) {
	var result []VersionInfo
	for _, key := range keys {
		stamp, entryName, ok := historyEntry(key)
		if !ok {
			continue
		}
		if entryName != name {
			continue
		}
		date, err := parseVersionStamp(stamp)
		if err != nil {
			return nil, fmt.Errorf("list versions: %s: %w", key, err)
		}
		result = append(result, VersionInfo{Stamp: stamp, Date: date, Path: key})
	}
	slices.SortFunc(result, func(a, b VersionInfo) int { return b.Date.Compare(a.Date) })
	return result, nil
}

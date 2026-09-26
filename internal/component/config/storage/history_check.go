// Design: docs/architecture/storage-backends.md -- content-addressed config history
// Related: history.go -- entry and object encoding the reachability walk reads

package storage

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/pkg/zefs"
)

// HistorySeverity grades a history finding. The zero value is never a valid
// grade, so a finding nobody graded cannot pass as a warning.
type HistorySeverity uint8

const (
	HistorySeverityUnspecified HistorySeverity = iota
	HistorySeverityWarning
	HistorySeverityError
)

// History finding kinds. `ze data check` prints the kind as the first field of
// each row, so `| match orphan` and `| match dangling` select them.
const (
	HistoryOrphanObject    = "orphan-object"
	HistoryDanglingEntry   = "dangling-entry"
	HistoryMalformedEntry  = "malformed-entry"
	HistoryWrongHashObject = "wrong-hash-object"
	HistoryDanglingPointer = "dangling-pointer"
)

// HistoryFinding is one reachability defect of content-addressed history.
type HistoryFinding struct {
	Kind     string
	Key      string
	Severity HistorySeverity
	Detail   string
}

// CheckHistory walks every key of s and reports what the frame check cannot
// see: an object whose bytes do not hash to its name, a dated entry that is
// malformed or names an absent object, an object no entry names (a warning,
// the residue of an interrupted write), and a config pointer naming a stamp
// whose entry does not resolve. zefs.Check is key-agnostic by design; the
// MEANING of a key is this walk's question. A frame that fails to read is left
// to the frame check, which already reports it corrupt.
func CheckHistory(s Storage) ([]HistoryFinding, error) {
	return inspectHistory(s.ListKeys, s.ReadKey)
}

// RepairHistory drops every dangling or malformed entry and every wrong-hash
// object under one guard, and reports them with the rest of the walk. Orphan
// objects are kept: they are harmless and a later write of the same content
// reuses them. A pointer left naming a dropped entry is reported and NEVER
// retargeted, because choosing another version is the operator's decision.
func RepairHistory(s Storage) (findings []HistoryFinding, err error) {
	guard, err := s.AcquireLock("")
	if err != nil {
		return nil, err
	}
	defer func() { err = releaseGuard(guard, err) }()

	findings, err = inspectHistory(guard.ListKeys, guard.ReadKey)
	if err != nil {
		return nil, err
	}
	for _, finding := range findings {
		switch finding.Kind {
		case HistoryWrongHashObject, HistoryDanglingEntry, HistoryMalformedEntry:
			if err := guard.RemoveKey(finding.Key); err != nil {
				return nil, fmt.Errorf("repair history: drop %s: %w", finding.Key, err)
			}
		}
	}
	return findings, nil
}

// inspectHistory is the walk CheckHistory and RepairHistory share. list and
// read are the raw-key pair of the caller's context.
func inspectHistory(list func(string) ([]string, error), read func(string) ([]byte, error)) ([]HistoryFinding, error) {
	keys, err := list("")
	if err != nil {
		return nil, fmt.Errorf("check history: list keys: %w", err)
	}
	var findings []HistoryFinding
	objects := make(map[string]bool)
	for _, key := range keys {
		digest, ok := strings.CutPrefix(key, zefs.KeyObject.Prefix())
		if !ok {
			continue
		}
		data, err := read(key)
		if err != nil {
			continue
		}
		stored := contentDigest(data)
		if stored != digest {
			findings = append(findings, HistoryFinding{Kind: HistoryWrongHashObject, Key: key, Severity: HistorySeverityError,
				Detail: "named " + entryDigestPrefix + digest + ", holds " + entryDigestPrefix + stored})
			continue
		}
		objects[digest] = true
	}

	referenced := make(map[string]bool)
	resolved := make(map[string]bool)
	for _, key := range keys {
		if _, _, ok := historyEntry(key); !ok {
			continue
		}
		value, err := read(key)
		if err != nil {
			continue
		}
		digest, err := entryDigest(key, value)
		if err != nil {
			findings = append(findings, HistoryFinding{Kind: HistoryMalformedEntry, Key: key, Severity: HistorySeverityError, Detail: err.Error()})
			continue
		}
		referenced[digest] = true
		if !objects[digest] {
			findings = append(findings, HistoryFinding{Kind: HistoryDanglingEntry, Key: key, Severity: HistorySeverityError,
				Detail: "names " + entryDigestPrefix + digest + ", " + objectKey(digest) + " is absent or does not hash to its name"})
			continue
		}
		resolved[key] = true
	}

	for _, key := range keys {
		digest, ok := strings.CutPrefix(key, zefs.KeyObject.Prefix())
		if !ok {
			continue
		}
		if objects[digest] && !referenced[digest] {
			findings = append(findings, HistoryFinding{Kind: HistoryOrphanObject, Key: key, Severity: HistorySeverityWarning,
				Detail: "no history entry names it"})
		}
	}

	for _, key := range keys {
		name, pointer, ok := configPointerKey(key)
		if !ok {
			continue
		}
		value, err := read(key)
		if err != nil {
			continue
		}
		stamp := strings.TrimSpace(string(value))
		if _, err := parseVersionStamp(stamp); err != nil {
			findings = append(findings, HistoryFinding{Kind: HistoryDanglingPointer, Key: key, Severity: HistorySeverityError,
				Detail: string(pointer) + " pointer does not hold a version stamp: " + err.Error()})
			continue
		}
		entry := zefs.KeyFileVersion.Key(stamp, name)
		if !resolved[entry] {
			findings = append(findings, HistoryFinding{Kind: HistoryDanglingPointer, Key: key, Severity: HistorySeverityError,
				Detail: "names " + stamp + ", " + entry + " does not resolve"})
		}
	}
	return findings, nil
}

// configPointerKey reports whether key is meta/config/<name>/<pointer> for
// one of the four version pointers, and returns the name and the pointer.
func configPointerKey(key string) (string, pointerName, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[0] != "meta" {
		return "", "", false
	}
	if parts[1] != "config" {
		return "", "", false
	}
	pointer := pointerName(parts[3])
	if !pointer.valid() {
		return "", "", false
	}
	return parts[2], pointer, true
}

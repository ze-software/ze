// Design: docs/architecture/web-interface.md -- explicit corrupt-change recovery.
// Related: editor_draft.go -- normal readers refuse unreadable change files.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// CorruptChangeFile binds explicit recovery to the inspected bytes and account.
// It holds no storage guard or raw content between requests. Its zero value is
// invalid; only InspectCorruptChangeFile constructs a usable recovery.
// Safe for concurrent use while the borrowed store and schema remain available.
type CorruptChangeFile struct {
	store       storage.Storage
	schema      *config.Schema
	configPath  string
	changePath  string
	fingerprint [sha256.Size]byte
}

// ChangeRecoveryOutcome separates an unchanged file from a completed removal
// and a storage failure that prevents determining whether removal happened.
type ChangeRecoveryOutcome uint8

const (
	ChangeRecoveryUnspecified ChangeRecoveryOutcome = iota
	ChangeRecoveryKept
	ChangeRecoveryRemoved
	ChangeRecoveryUncertain
)

// IsCorruptChangeFile distinguishes unreadable pending syntax from a storage
// failure or an invalid leaf value. It also recognizes wrapped editor errors.
func IsCorruptChangeFile(err error) bool {
	return errors.Is(err, errChangeFileUnparsable)
}

// InspectCorruptChangeFile reads without opening an editor or changing custody.
// Only syntax/path corruption qualifies: invalid candidate leaf values remain
// editable through the ordinary editor. The caller MUST authorize recovery.
func InspectCorruptChangeFile(store storage.Storage, configPath string, schema *config.Schema, username string) (*CorruptChangeFile, error) {
	if err := ValidateUser(username); err != nil {
		return nil, changeRecoveryFailure(err)
	}
	if store == nil {
		return nil, changeRecoveryFailure(storage.ErrNoStore)
	}
	guard, err := store.AcquireLock(configPath)
	if err != nil {
		return nil, changeRecoveryFailure(err)
	}
	path := ChangePath(configPath, username)
	data, readErr := guard.ReadFile(path)
	var recovery *CorruptChangeFile
	if readErr == nil {
		if _, _, _, parseErr := config.ParseCandidateChangeFile(string(data), schema); parseErr == nil {
			readErr = errors.New("this account's pending change file is readable; use normal review and discard")
		} else {
			recovery = &CorruptChangeFile{store: store, schema: schema, configPath: configPath,
				changePath: path, fingerprint: sha256.Sum256(data)}
		}
	}
	if err := errors.Join(readErr, guard.Release()); err != nil {
		return nil, changeRecoveryFailure(err)
	}
	return recovery, nil
}

// Fingerprint identifies the bytes the operator must confirm. It contains no
// file content, even when that content holds an unparseable secret assignment.
func (c *CorruptChangeFile) Fingerprint() string {
	return hex.EncodeToString(c.fingerprint[:])
}

// Discard removes only the inspected unreadable file. It rechecks identity and
// corruption under one guard; saved drafts and committed config are untouched.
// The caller MUST authorize the account and verify explicit confirmation first.
// Callers MUST account for confirmed removal even with an error, and MUST
// refresh or block a cached candidate after removed or uncertain outcomes.
func (c *CorruptChangeFile) Discard() (ChangeRecoveryOutcome, error) {
	if c == nil {
		return ChangeRecoveryKept, changeRecoveryFailure(errors.New("no unreadable change file was inspected"))
	}
	if c.store == nil {
		return ChangeRecoveryKept, changeRecoveryFailure(errors.New("no unreadable change file was inspected"))
	}
	guard, err := c.store.AcquireLock(c.configPath)
	if err != nil {
		return ChangeRecoveryKept, changeRecoveryFailure(err)
	}
	outcome, removeErr := c.discardGuarded(guard)
	err = errors.Join(removeErr, guard.Release())
	if outcome == ChangeRecoveryKept {
		return outcome, changeRecoveryFailure(err)
	}
	if outcome == ChangeRecoveryUncertain {
		return outcome, opererr.Wrap(err,
			"Recovery could not confirm whether your unreadable pending change file was removed.",
			"A storage error prevented checking the result; the saved draft file and running configuration were not changed",
			"run show doctor")
	}
	if err != nil {
		return outcome, opererr.Wrap(err,
			"Your unreadable pending change file was removed, but storage reported an error.",
			"The saved draft file and running configuration were preserved; check storage before editing again",
			"run show doctor")
	}
	return outcome, nil
}

func (c *CorruptChangeFile) discardGuarded(guard storage.WriteGuard) (ChangeRecoveryOutcome, error) {
	data, err := guard.ReadFile(c.changePath)
	if err != nil {
		return ChangeRecoveryKept, err
	}
	if sha256.Sum256(data) != c.fingerprint {
		return ChangeRecoveryKept, errors.New("the pending change file changed after inspection; review recovery again")
	}
	if _, _, _, err := config.ParseCandidateChangeFile(string(data), c.schema); err == nil {
		return ChangeRecoveryKept, errors.New("the pending change file is now readable; use normal review and discard")
	}
	if err := guard.Remove(c.changePath); err != nil {
		// A tree store can unlink successfully then fail its durability barrier.
		// Read back under the same guard before claiming the file was kept.
		_, readErr := guard.ReadFile(c.changePath)
		if readErr == nil {
			return ChangeRecoveryKept, fmt.Errorf("remove unreadable pending changes: %w", err)
		}
		if errors.Is(readErr, fs.ErrNotExist) {
			return ChangeRecoveryRemoved, err
		}
		return ChangeRecoveryUncertain, errors.Join(err, readErr)
	}
	return ChangeRecoveryRemoved, nil
}

func changeRecoveryFailure(cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"Your unreadable pending changes could not be discarded.",
		"Recovery kept the file; only an unchanged, unreadable change file can be discarded here",
		"Open /config/recover")
}

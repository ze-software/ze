// Design: docs/architecture/storage-backends.md -- restore modes.
// Related: backup.go -- Backup writes the artifact a restore reads.
// Related: pointer.go -- the candidate and promote sequence a restore commits through.

package storage

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// ErrRestoreSource reports an artifact that holds no config a restore can
// select. The message names what the artifact does hold.
var ErrRestoreSource = errors.New("restore source")

// RestoreSource is the config a restore selected from an artifact.
type RestoreSource struct {
	Name string // the config name inside the artifact
	Data []byte // the bytes of its active version, or of its file/active mirror
}

// ReadRestoreSource checks the artifact at path and returns the config a
// config-mode restore commits. Selection follows the spec's R-4 order: the
// sourceName the operator named, else the artifact's only config, else the
// config named like the device, else a refusal that lists every name the
// artifact holds beside the device's name. The bytes are the selected
// config's active version, or its file/active mirror when the artifact has no
// pointer for it (a seed or a hand-built blob).
func ReadRestoreSource(path, sourceName, deviceName string) (RestoreSource, error) {
	if err := checkArtifact(path); err != nil {
		return RestoreSource{}, err
	}
	source, err := OpenBlob(path, false)
	if err != nil {
		return RestoreSource{}, fmt.Errorf("restore %s: %w", path, err)
	}
	defer source.Close() //nolint:errcheck // read-only artifact handle.

	names, err := sourceConfigNames(source)
	if err != nil {
		return RestoreSource{}, fmt.Errorf("restore %s: %w", path, err)
	}
	selected, err := selectRestoreName(names, sourceName, deviceName)
	if err != nil {
		return RestoreSource{}, fmt.Errorf("restore %s: %w", path, err)
	}
	data, err := ReadActiveConfig(source, selected)
	if err != nil {
		return RestoreSource{}, fmt.Errorf("restore %s: read config %s: %w", path, selected, err)
	}
	return RestoreSource{Name: selected, Data: data}, nil
}

// selectRestoreName applies R-4. A name is never guessed: every refusal lists
// what the artifact holds.
func selectRestoreName(names []string, sourceName, deviceName string) (string, error) {
	held := strings.Join(names, ", ")
	if len(names) == 0 {
		return "", fmt.Errorf("%w: the artifact holds no config", ErrRestoreSource)
	}
	if sourceName != "" {
		if slices.Contains(names, sourceName) {
			return sourceName, nil
		}
		return "", fmt.Errorf("%w: the artifact holds no config %s; it holds: %s", ErrRestoreSource, sourceName, held)
	}
	if len(names) == 1 {
		return names[0], nil
	}
	if slices.Contains(names, deviceName) {
		return deviceName, nil
	}
	return "", fmt.Errorf("%w: the artifact holds several configs (%s) and none is named %s like this device; select one with name <source-name>", ErrRestoreSource, held, deviceName)
}

// sourceConfigNames returns the sorted config names an artifact holds: every
// name with an active pointer, and every name with a file/active mirror.
func sourceConfigNames(source Storage) ([]string, error) {
	pointerPrefix := zefs.KeyConfigActive.Prefix()
	pointerSuffix := strings.TrimPrefix(zefs.KeyConfigActive.Pattern, pointerPrefix+"{name}")
	pointers, err := source.ListKeys(pointerPrefix)
	if err != nil {
		return nil, err
	}
	mirrorPrefix := zefs.KeyFileActive.Prefix()
	mirrors, err := source.ListKeys(mirrorPrefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, key := range pointers {
		name, ok := strings.CutSuffix(strings.TrimPrefix(key, pointerPrefix), pointerSuffix)
		if !ok || name == "" || strings.Contains(name, "/") {
			continue
		}
		names = append(names, name)
	}
	for _, key := range mirrors {
		name := strings.TrimPrefix(key, mirrorPrefix)
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

// RestoreConfig commits data as a new version of the config named
// deviceName, under one guard: the version is written, the active pointer
// moves to it, the previous active becomes rollback, and file/active/<name>
// mirrors the new bytes. Nothing else in the store changes. A pending
// candidate refuses with ErrCandidateExists, because promoting over it would
// discard an edit somebody staged. This is the offline path; a running daemon
// restores through its own reload so the new config is accepted before it is
// promoted.
func RestoreConfig(target Storage, deviceName string, data []byte) (stamp string, err error) {
	guard, err := target.AcquireLock(deviceName)
	if err != nil {
		return "", err
	}
	defer func() { err = releaseGuard(guard, err) }()

	stamp, err = WriteCandidateVersionWithGuard(target, guard, deviceName, data, time.Now())
	if err != nil {
		return "", fmt.Errorf("restore config %s: %w", deviceName, err)
	}
	if err := promoteCandidateLocked(target, guard, deviceName); err != nil {
		// A written active pointer is the commit point: past it the version is
		// live and MUST stay. Before it, the candidate this restore staged is
		// withdrawn, so no later reload or startup applies a half-finished one.
		active, hasActive, readErr := readPointerLocked(target, guard, deviceName, pointerActive)
		if readErr != nil {
			return "", errors.Join(fmt.Errorf("restore config %s: %w", deviceName, err), readErr)
		}
		if hasActive {
			if active == stamp {
				return "", fmt.Errorf("restore config %s: %w", deviceName, err)
			}
		}
		clearErr := clearPointerLocked(target, guard, deviceName, pointerCandidate)
		_, removeErr := removeVersionLocked(target, guard, deviceName, stamp)
		return "", errors.Join(fmt.Errorf("restore config %s: %w", deviceName, err), clearErr, removeErr)
	}
	return stamp, nil
}

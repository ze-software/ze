// Design: docs/contributing/running-commands.md -- job admission, and "When the disk is full"
// Overview: registry.go -- the registry entries this reads
// Related: ../scratch/cacheclean.go -- the full cache empty that refuses while a job runs

package job

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Holder is one registry entry as another package reads it: the process that
// holds a slot, and the label it was admitted under. A verify run holds one
// for its worktree (internal/le/verify, admitWorktree), and `le job run` holds
// one for its command, so the registry is where le records that a build or a
// test is running against this checkout's caches.
type Holder struct {
	PID   int    `json:"pid"`
	Label string `json:"label"`
}

// Holders answers every readable entry in the registry of the checkout at
// root, in name order. It does not judge liveness: a dead job's entry remains
// untouched by admission waiters, so the caller matches each PID against a
// process table it scanned. An absent registry holds no
// entry. A registry that exists and cannot be listed answers an error, never
// an empty list: the full cache empty refuses on that error, and an empty list
// would read as "no job is running" (refuseFullEmpty, internal/le/scratch).
// An entry that cannot be read names no process to match; the registry's own
// scan keeps its capacity occupied regardless of age.
//
// Safe for concurrent use: it only reads, and an entry removed between the
// listing and the read is skipped.
func Holders(root string) ([]Holder, error) {
	registry := &Admission{Root: root}
	listed, err := os.ReadDir(registry.jobsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list the job registry: %w", err)
	}
	// os.ReadDir answers in name order, which is the order Holders promises.
	holders := make([]Holder, 0, len(listed))
	for _, listing := range listed {
		if !strings.HasSuffix(listing.Name(), entrySuffix) {
			continue
		}
		held, ok := registry.readEntry(filepath.Join(registry.jobsDir(), listing.Name()))
		if !ok {
			continue
		}
		holders = append(holders, Holder{PID: held.pid, Label: held.label})
	}
	return holders, nil
}

// Design: docs/contributing/running-commands.md -- "When the disk is full", and waiting for another session's job
// Overview: cacheclean.go -- the scratch clean run this row belongs to
// Related: ../job/prune.go -- the judgment of which registry files are dead
//
// The job registry, tmp/.ze-jobs, is not a cache, and scratch clean is still
// what cleans it: no job and no waiter may remove a file another process
// wrote, so the files of dead jobs and the receipt every release leaves had
// no remover at all (plan/journal/artifact-written-with-no-remover.md,
// 2026-11-13: 4,756 receipts beside 13 entries). This action is the
// checkout's cleaner, an operator starts it, and it already reads the
// registry to refuse the full empty.
//
// Both modes prune the same way, with the age floor the cache entries keep.
// The full empty is no reason to remove more: it runs only while no job holds
// a slot, and even then a receipt written seconds ago may be one a follower
// has not read yet, and a job that starts during the empty writes an entry
// the snapshot never saw. The floor is what makes the removal safe, in either
// mode. The store trim does not prune the registry: it bounds stores by their
// bytes, and these files have no budget to hold them to.

package scratch

import (
	"context"
	"maps"
	"path/filepath"
	"slices"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/job"
)

// jobRegistry names the registry's row in the report.
const jobRegistry = "jobs"

// absentRegistrySkip is the reason a checkout with no registry gets a skip.
const absentRegistrySkip = "does not exist, so it holds nothing to clean"

// RegistryClean is the job registry's row of one scratch clean run: what the
// run removed, in bytes and in files, and how many files it kept for each
// reason. Removed counts file lengths, which is far less than the space
// thousands of two-byte receipts hold in blocks and in the directory itself.
//
// Error is set when the registry was not pruned at all, and when a dead file
// could not be removed. Either makes the run exit 1.
type RegistryClean struct {
	Path         string         `json:"path"`
	Removed      int64          `json:"removed,omitempty"`
	FilesRemoved int            `json:"files-removed,omitempty"`
	FilesKept    int            `json:"files-kept,omitempty"`
	Kept         []RegistryKept `json:"kept,omitempty"`
	Skipped      string         `json:"skipped,omitempty"`
	Error        string         `json:"error,omitempty"`
}

// RegistryKept is how many registry files one reason kept.
type RegistryKept struct {
	Reason string `json:"reason"`
	Files  int    `json:"files"`
}

// cleanJobRegistry removes the dead files of the checkout's job registry that
// are older than the cache entry age floor, through job.Prune, which owns the
// file formats and the judgment, and answers the registry's row.
//
// It reads the clock, the floor and the process scanner of the pass, so a
// test replaces each one.
func cleanJobRegistry(ctx context.Context, root string, pass cleanPass) *RegistryClean {
	row := &RegistryClean{Path: filepath.Join(root, filepath.FromSlash(job.JobsDir))}

	pruned, err := job.Prune(ctx, root, job.PrunePolicy{Now: pass.entries.now, Age: pass.entries.floor, Scan: pass.scan})
	if err != nil {
		var text textbuf.Buffer
		row.Error = text.Str("the job registry was not cleaned: ").Err(err).
			Str(". Nothing was removed, so its dead files still lengthen every admission scan and `./le job report`.").
			Str(" Run `./le scratch clean` again once the cause the error names is gone").String()
		return row
	}
	if pruned.Absent {
		row.Skipped = absentRegistrySkip
		return row
	}

	for _, removed := range pruned.Removed {
		row.Removed += removed.Bytes
		row.FilesRemoved++
	}

	kept := map[job.PruneReason]int{}
	failures := 0
	var failed job.PruneRow
	for _, file := range pruned.Kept {
		kept[file.Reason]++
		row.FilesKept++
		if file.Reason != job.PruneKeptRemoveFailed {
			continue
		}
		if failures == 0 {
			failed = file
		}
		failures++
	}
	// The reasons are printed in the order the job package declares them.
	for _, reason := range slices.Sorted(maps.Keys(kept)) {
		row.Kept = append(row.Kept, RegistryKept{Reason: reason.String(), Files: kept[reason]})
	}

	if failures != 0 {
		var text textbuf.Buffer
		row.Error = text.Int(int64(failures)).Str(" dead job registry files could not be removed, the first being ").
			Str(failed.Path).Str(": ").Err(failed.Err).
			Str(". They still lengthen every admission scan. Fix the permission the error names,").
			Str(" then run `./le scratch clean` again").String()
	}
	return row
}

// line renders the registry's row: what was removed and what each reason
// kept. The size is in MiB, because a registry is megabytes of logs at most.
func (row RegistryClean) line() string {
	var line textbuf.Buffer
	line.PadRight(jobRegistry, cacheNameWidth)
	if row.Error != "" {
		return line.Str("REFUSE   ").Str(row.Path).Str(": ").Str(row.Error).String()
	}
	if row.Skipped != "" {
		return line.Str("SKIP     ").Str(row.Path).Str(": ").Str(row.Skipped).String()
	}

	line.PadRight(row.Path, 48).Str(" removed ").Str(mebibytes(row.Removed)).Str(" (").
		Int(int64(row.FilesRemoved)).Str(" dead files older than ").Str(cacheEntryAgeFloor.String()).
		Str("), kept ").Int(int64(row.FilesKept)).Str(" files")
	for index, kept := range row.Kept {
		if index == 0 {
			line.Str(": ")
		} else {
			line.Str(", ")
		}
		line.Int(int64(kept.Files)).Byte(' ').Str(kept.Reason)
	}
	return line.String()
}

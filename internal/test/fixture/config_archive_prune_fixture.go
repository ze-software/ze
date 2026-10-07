// Design: docs/architecture/api/commands.md -- config archive trigger handler
// Related: register_config_archive_prune.go -- the registration

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// configArchivePruneHost is the system host the scenario configures, so the
// archive filenames it expects start with "ze-<host>-" under the default
// "{name}-{host}-{date}-{time}" format, where {name} is "ze" from ze.conf.
const configArchivePruneHost = "ci-archive"

// configArchivePruneUnrelated is a .conf file in the archive directory that no
// archive block wrote. Pruning MUST leave it, because only the files an archive
// block names are counted against commit-revisions.
const configArchivePruneUnrelated = "unrelated.conf"

// configArchivePrune proves `commit-revisions 2` caps a file:// archive location
// at two files, on the operator's path: a running daemon, `ze config archive
// local` over SSH three times, and the files left on disk counted after each.
//
// Method: each archive runs in its own wall-clock second, because the default
// filename resolves time to the second and two archives in one second write one
// file. After the first two archives both files remain; after the third, exactly
// the second and third remain, and the unrelated .conf file is untouched.
func configArchivePrune(ctx context.Context, _ []string) error {
	archiveDir, err := os.MkdirTemp("", "ze-archive-prune-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(archiveDir) //nolint:errcheck // isolated fixture state is best-effort cleanup

	unrelated := filepath.Join(archiveDir, configArchivePruneUnrelated)
	if err := os.WriteFile(unrelated, []byte("not an archive\n"), 0o600); err != nil {
		return err
	}

	daemon, cliEnv, err := startFixtureDaemon(ctx, configArchivePruneConfig(archiveDir))
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a stop failure changes no assertion

	var written []string
	var last time.Time
	for round := 1; round <= 3; round++ {
		if err := configArchivePruneNextSecond(ctx, last); err != nil {
			return err
		}
		before, err := configArchivePruneList(archiveDir)
		if err != nil {
			return err
		}
		code, stdout, stderr, runErr := runCaptured(ctx, cliEnv, "", "ze", "config", "archive", "local")
		last = time.Now()
		if runErr != nil || code != 0 {
			return fmt.Errorf("ze config archive local (round %d) exit=%d: %w\n%s%s", round, code, runErr, stdout, stderr)
		}
		after, err := configArchivePruneList(archiveDir)
		if err != nil {
			return err
		}
		added := configArchivePruneAdded(before, after)
		if len(added) != 1 {
			return fmt.Errorf("round %d: archive wrote %d new files %v, want 1 (before %v, after %v)", round, len(added), added, before, after)
		}
		written = append(written, added[0])
		want := written[max(0, len(written)-2):]
		if !slices.Equal(after, want) {
			return fmt.Errorf("round %d: archive directory holds %v, want the newest %d %v (commit-revisions 2)", round, after, len(want), want)
		}
		fmt.Println("round " + strconv.Itoa(round) + ": archive files " + strings.Join(after, ", "))
	}

	if _, err := os.Stat(unrelated); err != nil {
		return fmt.Errorf("pruning removed %s, a file no archive block wrote: %w", configArchivePruneUnrelated, err)
	}
	fmt.Println("OK: commit-revisions 2 kept " + strings.Join(written[1:], ", ") + "; pruned " + written[0] + "; kept " + configArchivePruneUnrelated)
	return nil
}

// configArchivePruneConfig is the daemon config: one manual file:// archive
// block at dir, commit-revisions 2, and the SSH user the CLI logs in as.
func configArchivePruneConfig(dir string) string {
	return `system {
    host ` + configArchivePruneHost + `
    commit-revisions 2
    archive local {
        location file://` + dir + `
    }
    authentication {
        user ci {
            password "$PASSWORD_HASH"
            profile [ admin ]
        }
    }
}
`
}

// configArchivePruneNextSecond waits until the wall clock is in a later second
// than last, so the next archive's {time} token differs from the previous one.
// The wait is on the clock itself, which is the state the filename reads.
func configArchivePruneNextSecond(ctx context.Context, last time.Time) error {
	if last.IsZero() {
		return nil
	}
	timer := time.NewTimer(time.Until(last.Truncate(time.Second).Add(time.Second)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// configArchivePruneList answers the archive files in dir, sorted. An archive
// file is a .conf whose name starts with the prefix the default format gives.
func configArchivePruneList(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("archive directory is empty: the unrelated file is gone")
	}
	prefix := "ze-" + configArchivePruneHost + "-"
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasSuffix(name, ".conf") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// configArchivePruneAdded answers the names in after that before does not hold.
func configArchivePruneAdded(before, after []string) []string {
	var added []string
	for _, name := range after {
		if !slices.Contains(before, name) {
			added = append(added, name)
		}
	}
	return added
}

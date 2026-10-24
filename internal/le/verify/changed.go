// Design: docs/architecture/testing/verify-freshness-scope.md -- the scoped record a push reads
// Related: lifecycle.go -- the worktree run that writes it
// Related: green.go -- the release check's own record, beside it
//
// `./le verify changed` is the scoped verification: the tests of what the
// change set reaches, run in a throwaway worktree at one commit. It is the
// evidence a push owes, so an agent may start it, and it is admitted through
// the job slots like any job. A green run writes the scoped record, which
// `./le commit push` reads (internal/le/commit/owed.go).

package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// ChangedPath is the record of the last green `./le verify changed` run,
// relative to the checkout root.
const ChangedPath = "tmp/ze-verify-changed.status"

// ChangedCommand is the scoped verification an agent runs for its own
// evidence, and the one a push owes.
const ChangedCommand = "./le verify changed"

// ErrNotScoped marks a record file that exists and proves nothing: a field
// missing or malformed. It is never read as "no record", which is
// os.ErrNotExist.
var ErrNotScoped = errors.New("not a green scoped verification record")

// ScopedRecord is one green scoped verification over one commit: its tree, the
// anchor its change set started from, and the upstream the anchor was computed
// against.
//
// The fields are private so that only NewScopedRecord and ReadScopedRecord make
// one, and both refuse a record with a field missing. Its zero value is not a
// record: WriteScopedRecord refuses it.
type ScopedRecord struct {
	commit         string
	tree           string
	anchor         string
	upstream       string
	upstreamCommit string
	timestamp      string
}

// NewScopedRecord validates one green scoped verification over commit, whose
// git tree id is tree, whose change set ran from anchor.
func NewScopedRecord(commit, tree string, anchor repochanged.Anchor, at time.Time) (ScopedRecord, error) {
	record := ScopedRecord{
		commit: commit, tree: tree, anchor: anchor.Commit, upstream: anchor.Upstream,
		upstreamCommit: anchor.UpstreamCommit, timestamp: at.UTC().Format(time.RFC3339),
	}
	return record, record.valid()
}

// Commit is the commit the run verified.
func (r ScopedRecord) Commit() string { return r.commit }

// Tree is the git tree id of Commit.
func (r ScopedRecord) Tree() string { return r.tree }

// Anchor is the commit the change set started from: every commit in
// Anchor..Commit was in the run's scope.
func (r ScopedRecord) Anchor() string { return r.anchor }

// Upstream is the ref the anchor was computed against.
func (r ScopedRecord) Upstream() string { return r.upstream }

// UpstreamCommit is the commit Upstream named when the run computed its anchor.
func (r ScopedRecord) UpstreamCommit() string { return r.upstreamCommit }

// Timestamp is when the record was made, RFC 3339 in UTC.
func (r ScopedRecord) Timestamp() string { return r.timestamp }

// Text is the record's file form, one key=value per line.
func (r ScopedRecord) Text() string {
	var text textbuf.Buffer
	text.Str("mode=").Str(verifyengine.ChangedMode).Byte('\n').
		Str("commit=").Str(r.commit).Byte('\n').
		Str("tree=").Str(r.tree).Byte('\n').
		Str("anchor=").Str(r.anchor).Byte('\n').
		Str("upstream=").Str(r.upstream).Byte('\n').
		Str("upstream_commit=").Str(r.upstreamCommit).Byte('\n').
		Str("timestamp=").Str(r.timestamp).Byte('\n')
	return text.String()
}

func (r ScopedRecord) valid() error {
	for _, field := range []struct{ name, value string }{
		{"commit", r.commit}, {"tree", r.tree}, {"anchor", r.anchor}, {"upstream_commit", r.upstreamCommit},
	} {
		if !isObjectID(field.value) {
			return fmt.Errorf("%w: %s %q is not a git object id", ErrNotScoped, field.name, field.value)
		}
	}
	if r.upstream == "" {
		return fmt.Errorf("%w: no upstream ref", ErrNotScoped)
	}
	if _, err := time.Parse(time.RFC3339, r.timestamp); err != nil {
		return fmt.Errorf("%w: timestamp %q", ErrNotScoped, r.timestamp)
	}
	return nil
}

// WriteScopedRecord replaces the record at root atomically, so a reader sees
// the old record or the whole new one.
func WriteScopedRecord(root string, record ScopedRecord) error {
	if err := record.valid(); err != nil {
		return err
	}
	if err := writeRecordAtomically(root, ChangedPath, ".le-changed-*", record.Text()); err != nil {
		return fmt.Errorf("write scoped record: %w", err)
	}
	return nil
}

// ReadScopedRecord reads the record at root. An absent record is an error
// wrapping os.ErrNotExist, the normal state before any scoped run; a malformed
// one is an error wrapping ErrNotScoped. A caller MUST tell the two apart, and
// MUST NOT read either as evidence.
func ReadScopedRecord(root string) (ScopedRecord, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ChangedPath))) //nolint:gosec // a fixed path under the checkout root
	if err != nil {
		return ScopedRecord{}, fmt.Errorf("read scoped record: %w", err)
	}
	fields := make(map[string]string, 7)
	for line := range strings.SplitSeq(string(content), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			fields[key] = value
		}
	}
	if fields["mode"] != verifyengine.ChangedMode {
		return ScopedRecord{}, fmt.Errorf("%w: mode %q", ErrNotScoped, fields["mode"])
	}
	record := ScopedRecord{
		commit: fields["commit"], tree: fields["tree"], anchor: fields["anchor"], upstream: fields["upstream"],
		upstreamCommit: fields["upstream_commit"], timestamp: fields["timestamp"],
	}
	if err := record.valid(); err != nil {
		return ScopedRecord{}, err
	}
	return record, nil
}

// changedHere is `le verify changed [commit <rev>]`: the scoped population in a
// fresh detached worktree at the commit. It is not a whole-tree verb, so the
// terminal gate never asks it.
func changedHere(args leaction.Arguments) (any, int) {
	options := Options{Commit: strings.TrimSpace(args.One(commitKeyword)), Mode: verifyengine.ChangedMode}
	return runWorktree(options)
}

// runAnchor computes the anchor of the worktree at path, the one the run's
// selector will compute from the same refs: a detached worktree has no @{u},
// so it is merge-base(commit, origin/HEAD) there.
func runAnchor(ctx context.Context, path string, deps dependencies) (repochanged.Anchor, error) {
	return repochanged.ResolveAnchor(path, func(dir string, args ...string) (string, error) {
		result, err := deps.git(ctx, gitTimeoutMetadata, dir, args...)
		if err != nil {
			return "", err
		}
		if result.Code != 0 {
			return "", errors.New(commandFailure("git "+strings.Join(args, " "), result, nil))
		}
		return result.Output, nil
	})
}

// recordChanged writes the scoped record for a green, completed changed run,
// and answers the line that says what it wrote or why it wrote nothing. A red
// run writes no record and leaves the previous one in place: that record is
// still true about its own commit. A green run that skipped a suite, or whose
// certificate names another commit or a tree that moved, writes none either.
func recordChanged(ctx context.Context, root string, anchor repochanged.Anchor, report *Report, deps dependencies) string {
	verification := report.Verify
	if verification == nil || verification.Attached != nil || report.Code != 0 || !verification.Completed {
		return ""
	}
	if verification.Mode != verifyengine.ChangedMode {
		return ""
	}
	var text textbuf.Buffer
	// A run that skipped a suite did not judge the scope, and a run whose tree
	// moved judged a tree that is not the commit: neither proves the commit,
	// the same two refusals recordGreen makes for the release check.
	if skipped := verifyengine.SkippedSuites(); skipped != "" {
		return text.Str("verify-changed: scoped record not written: suites skipped (ZE_SKIP_SUITES=").
			Str(skipped).Byte(')').String()
	}
	certificate, err := verifyengine.ReadCertificate(root)
	if err != nil {
		return text.Str("verify-changed: scoped record not written: ").Err(err).String()
	}
	if certificate.GitSHA != report.Commit {
		return text.Str("verify-changed: scoped record not written: certificate names ").
			Str(certificate.GitSHA).Str(", this run verified ").Str(report.Commit).String()
	}
	if certificate.TreeMoved() {
		return text.Str("verify-changed: scoped record not written: the tree moved during the run").String()
	}
	tree, err := deps.git(ctx, gitTimeoutMetadata, root, "rev-parse", "--verify", "--quiet", report.Commit+"^{tree}")
	if err != nil || tree.Code != 0 {
		return text.Str("verify-changed: scoped record not written: ").
			Str(commandFailure("git rev-parse", tree, err)).String()
	}
	record, err := NewScopedRecord(report.Commit, strings.TrimSpace(tree.Output), anchor, deps.now())
	if err != nil {
		return text.Str("verify-changed: scoped record not written: ").Err(err).String()
	}
	if err := WriteScopedRecord(root, record); err != nil {
		return text.Str("verify-changed: scoped record not written: ").Err(err).String()
	}
	return text.Str("verify-changed: scoped record written to ").Str(ChangedPath).String()
}

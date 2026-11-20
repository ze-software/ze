// Design: docs/contributing/spec-workflow.md -- spec sets
// Related: specset.go -- GroupSets, which assigns each closed stem to its set
// Related: ../path/closure.go -- ClosedStems, the judgement applied to each commit

package specstatus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/repo/objects"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// The record of closed specs is one file every session of the checkout
// shares, beside the claim markers under tmp/session/. A session's own scratch
// is empty when it starts, so a record kept there would be walked again by
// every session; one shared file is walked once per commit range.
//
// The file is text, one fact per line:
//
//	ze-closed-specs 1
//	commit <full commit id the record was computed at>
//	stem <stem>        (one line per closed spec, sorted)
//	end
//
// The last line is the completion mark: a file cut short by a crash or a
// power loss lacks it and reads as corrupt, never as a shorter list.
//
// A stem is one line, so a stem that cannot be one is never written: an empty
// stem (spec-.md) would be the line "stem ", which the reader refuses, and a
// stem holding a line break would be two lines. A spec so named is never
// recorded closed (writableStem).
//
// The number in closedRecordHeader MUST be bumped whenever specpath.ClosedStems
// or specpath.HistoricalBucket changes what counts as a closure: the record is
// keyed by commit, not by rule, so a record written under the old rule would
// otherwise be extended as if it were current. A bumped header reads as
// corrupt, and the record is recomputed.
const (
	closedRecordName   = ".closed-specs"
	closedRecordHeader = "ze-closed-specs 1"
	closedRecordCommit = "commit "
	closedRecordStem   = "stem "
	closedRecordEnd    = "end"
)

// notComputed opens every reason a record is unknown, so a reader prints
// "not computed" and never an empty list that reads as "no closed specs".
const notComputed = "not computed"

// ClosedSpecs is the record of which specs closed: the stems whose spec file a
// commit removed and did not relocate or rename (specpath.ClosedStems), and the
// commit the record was computed at.
//
// The zero value is the record not computed, and Known answers false for it:
// an empty stem list is an answer only when Known is true.
type ClosedSpecs struct {
	commit string
	stems  []string
	known  bool
	reason string
}

// Known reports whether the record was computed. Stems and Commit answer only
// when it is; Unknown says why it is not.
func (closed ClosedSpecs) Known() bool { return closed.known }

// Commit answers the commit the record was computed at, or "" when it is not
// known. It need not be HEAD: a record read without git names the commit it
// holds, and the reader says so.
func (closed ClosedSpecs) Commit() string { return closed.commit }

// Stems answers the closed stems, sorted. It is empty both when nothing closed
// and when the record is not known, so Known is the question to ask first.
func (closed ClosedSpecs) Stems() []string { return slices.Clone(closed.stems) }

// Unknown answers why the record is not known, starting "not computed", or ""
// when it is known.
func (closed ClosedSpecs) Unknown() string {
	if closed.known {
		return ""
	}
	if closed.reason == "" {
		return notComputed
	}
	return closed.reason
}

// ClosedSpecsNotComputed answers a record that could not be computed, and
// why: a caller whose LoadClosedSpecs failed passes this on rather than the
// record as the file holds it, whose commit may predate a closure that the
// failed walk would have added.
func ClosedSpecsNotComputed(why string) ClosedSpecs {
	return ClosedSpecs{reason: notComputed + ": " + why}
}

// closedRecordPath answers the one shared record file of a checkout.
func closedRecordPath(root string) string {
	return filepath.Join(root, "tmp", "session", closedRecordName)
}

// ReadClosedSpecs answers what the shared record holds and the commit it was
// computed at, running no git: it is the reader for a path with a time budget
// a history walk does not fit, the session-start summary. An absent or
// unreadable record answers a ClosedSpecs that is not Known, with the reason;
// it never fails the caller and never answers an empty list in its place.
func ReadClosedSpecs(root string) ClosedSpecs {
	path := closedRecordPath(root)
	content, err := os.ReadFile(path) //nolint:gosec // the shared record under this checkout's tmp/session
	if errors.Is(err, os.ErrNotExist) {
		return ClosedSpecs{reason: notComputed + ": no record at " + path}
	}
	if err != nil {
		return ClosedSpecs{reason: notComputed + ": cannot read " + path + ": " + err.Error()}
	}
	closed, problem := parseClosedRecord(content)
	if problem != "" {
		return ClosedSpecs{reason: notComputed + ": the record at " + path + " is corrupt: " + problem}
	}
	return closed
}

// LoadClosedSpecs answers the record of closed specs at HEAD, extending the
// shared record from the commit it was computed at, and rewrites the record
// when HEAD moved.
//
// The walk is bounded to the commits the record does not yet cover: from the
// cached commit to HEAD when that commit is an ancestor of HEAD, so no commit
// is read twice. Three states recompute from the whole history instead, and
// none of them fails the caller: no record, a corrupt record (it is replaced),
// and a record computed at a commit that is no ancestor of HEAD, as after a
// rebase or a reset, whose stems may name closures HEAD no longer holds.
//
// Every stem the record will hold, walked or carried over from the record it
// extends, passes writableStem here, before the record is written: a spec
// whose stem is empty or holds a line break is never recorded closed, because
// its stem cannot be one line of the record, and writing it would make every
// later read corrupt and every load rewalk the whole history.
//
// Sessions write the record at once without a lock: each one publishes a
// whole file with one rename (derived.WriteAtomicAll), so a reader sees one
// writer's whole record and never a torn one, and the last writer wins. A
// losing writer's work is not lost but redone: its range is walked again by
// the next reader whose record does not cover it.
func LoadClosedSpecs(ctx context.Context, root string) (ClosedSpecs, error) {
	head, err := objects.Resolve(ctx, root, "HEAD")
	if err != nil {
		return ClosedSpecs{}, fmt.Errorf("read the closed specs: %w", err)
	}
	// A shallow clone's history stops at its graft, so a walk of it would
	// answer a Known record missing every closure before that point.
	shallow, err := closedGit(ctx, root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return ClosedSpecs{}, fmt.Errorf("read the closed specs: %w", err)
	}
	if strings.TrimSpace(shallow) == "true" {
		return ClosedSpecs{reason: notComputed + ": this checkout is a shallow clone, so its history lacks the " +
			"removals before its oldest commit; fetch the whole history with git fetch --unshallow"}, nil
	}
	cached := ReadClosedSpecs(root)
	from := ""
	var stems []string
	if cached.Known() {
		if cached.commit == head {
			return cached, nil
		}
		ancestor, err := isAncestor(ctx, root, cached.commit, head)
		if err != nil {
			return ClosedSpecs{}, err
		}
		if ancestor {
			from = cached.commit
			stems = cached.stems
		}
	}
	walked, err := closedInHistory(ctx, root, from, head)
	if err != nil {
		return ClosedSpecs{}, err
	}
	for _, stem := range walked {
		if !slices.Contains(stems, stem) {
			stems = append(stems, stem)
		}
	}
	stems = slices.DeleteFunc(stems, func(stem string) bool { return !writableStem(stem) })
	slices.Sort(stems)
	closed := ClosedSpecs{commit: head, stems: stems, known: true}
	if err := writeClosedRecord(root, closed); err != nil {
		return ClosedSpecs{}, err
	}
	return closed, nil
}

// isAncestor reports whether commit is an ancestor of head. git answers 1 for
// "no" and 128 for a commit it does not hold, which a pruned rebase leaves:
// both are "no", and the record is recomputed. Any other failure is an error.
func isAncestor(ctx context.Context, root, commit, head string) (bool, error) {
	_, err := closedGit(ctx, root, "merge-base", "--is-ancestor", commit, head)
	if err == nil {
		return true, nil
	}
	if exit, isExit := errors.AsType[*exec.ExitError](err); isExit {
		if exit.ExitCode() == 1 {
			return false, nil
		}
		if exit.ExitCode() == 128 {
			return false, nil
		}
	}
	return false, fmt.Errorf("judge whether the closed-spec record's commit %s is an ancestor of HEAD: %w", commit, err)
}

// writableStem reports whether stem can be one `stem` line of the record. An
// empty stem is written as "stem ", which parseClosedRecord refuses, and a
// stem holding a line break is written as two lines; a carriage return is no
// line break here, because the record splits on "\n" alone.
func writableStem(stem string) bool {
	if stem == "" {
		return false
	}
	return !strings.Contains(stem, "\n")
}

// closedInHistory answers the stems closed by the first-parent commits from
// from (excluded) to head, or by the whole first-parent history of head when
// from is "". The walk reads one git log over specpath.Root, so its output is
// bounded by the commits that touched the plan tree. A commit is judged only
// when it removes a spec. Judging one resolves its parent's tree and reads the
// parent's blob of every spec path the commit carries without removing, to
// learn which are new; for each new spec it reads the commit's blob, and when
// any is new it reads the parent's blob of each removed spec, for the rename
// test.
//
// The first-parent line is the history this checkout's main holds, and a merge
// is judged by its diff against its first parent, which is what it brought to
// that line.
func closedInHistory(ctx context.Context, root, from, head string) ([]string, error) {
	selection := head
	if from != "" {
		selection = from + ".." + head
	}
	shown, err := closedGit(ctx, root, "-c", "core.quotePath=false", "log", "--first-parent",
		"--diff-merges=first-parent", "--no-renames", "--no-color", "--no-ext-diff", "--no-show-signature",
		"--name-status", "--format=%x1e%H%x00%P%x00", selection, "--", specpath.Root)
	if err != nil {
		return nil, fmt.Errorf("read the spec removals in git history: %w", err)
	}
	var stems []string
	for record := range strings.SplitSeq(shown, "\x1e") {
		if strings.TrimSpace(record) == "" {
			continue
		}
		change, err := parseClosedCommit(record)
		if err != nil {
			return nil, err
		}
		if !change.removesSpec() {
			continue
		}
		closed, err := specpath.ClosedStems(change.paths, change.removed, change.sides(ctx, root))
		if err != nil {
			return nil, fmt.Errorf("judge commit %s: %w", change.commit, err)
		}
		stems = append(stems, closed...)
	}
	return stems, nil
}

// closedCommit is one commit of the walk: its id, its first parent ("" for a
// root commit) and the spec-tree paths it carries and removes.
type closedCommit struct {
	commit  string
	parent  string
	paths   []string
	removed []string
}

// parseClosedCommit reads one `%H%x00%P%x00` record and its name-status lines.
// A path git quoted holds a control character or a quote. Unquoted, it is an
// ordinary path and is judged by the same rule as any other: refusing it
// would fail every later load over the same history. A quoting this cannot
// read is skipped when its text cannot name a spec, and refuses when it may,
// rather than skip a removal.
func parseClosedCommit(record string) (closedCommit, error) {
	fields := strings.SplitN(record, "\x00", 3)
	if len(fields) != 3 {
		return closedCommit{}, fmt.Errorf("read the spec removals in git history: a log record that is not a commit, %q", record)
	}
	change := closedCommit{commit: strings.TrimSpace(fields[0])}
	if parents := strings.Fields(fields[1]); len(parents) != 0 {
		change.parent = parents[0]
	}
	for line := range strings.SplitSeq(fields[2], "\n") {
		status, path, tabbed := strings.Cut(strings.TrimSpace(line), "\t")
		if !tabbed {
			continue
		}
		if strings.HasPrefix(path, `"`) {
			unquoted, err := strconv.Unquote(path)
			if err != nil {
				if strings.Contains(path, "spec-") {
					return closedCommit{}, fmt.Errorf("read the spec removals in git history: commit %s names a quoted path %s that may be a spec and cannot be unquoted: %w", change.commit, path, err)
				}
				continue
			}
			path = unquoted
		}
		if strings.HasPrefix(status, "D") {
			change.removed = append(change.removed, path)
			continue
		}
		change.paths = append(change.paths, path)
	}
	return change, nil
}

// removesSpec reports whether the commit deletes a spec file, the only commit
// that can close one.
func (change closedCommit) removesSpec() bool {
	for _, path := range change.removed {
		if _, isSpec := specpath.HistoricalBucket(path); isSpec {
			return true
		}
	}
	return false
}

// sides reads a landed commit's rename evidence from its own blobs: its first
// parent holds the removed spec's text and the commit holds the added spec's.
func (change closedCommit) sides(ctx context.Context, root string) specpath.Sides {
	return specpath.Sides{Before: treeSide(ctx, root, change.parent), After: treeSide(ctx, root, change.commit)}
}

// treeSide answers paths from one commit's tree, resolved once on the first
// read. A root commit's parent side (revision "") holds nothing. Not safe for
// concurrent use.
func treeSide(ctx context.Context, root, revision string) specpath.Side {
	tree, problem := "", ""
	resolved := false
	return func(path string) (string, bool, string) {
		if revision == "" {
			return "", false, ""
		}
		if !resolved {
			resolved = true
			var err error
			if tree, err = objects.ResolveTree(ctx, root, revision); err != nil {
				problem = err.Error()
			}
		}
		if problem != "" {
			return "", false, problem
		}
		content, present, err := objects.ReadPath(ctx, root, tree, path)
		if err != nil {
			return "", false, err.Error()
		}
		return string(content), present, ""
	}
}

// parseClosedRecord reads the record file, and answers why when it is not one
// whole record: a wrong header, a missing or malformed commit line, a line it
// does not know, or no completion mark.
func parseClosedRecord(content []byte) (ClosedSpecs, string) {
	// The mark is the last LINE, never a byte suffix: a file cut short after
	// `stem bsd-pf-firewall-backend` ends in "end\n" too.
	text, terminated := strings.CutSuffix(string(content), "\n")
	if !terminated {
		return ClosedSpecs{}, "no completion mark"
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] != closedRecordEnd {
		return ClosedSpecs{}, "no completion mark"
	}
	lines = lines[:len(lines)-1]
	if len(lines) < 2 {
		return ClosedSpecs{}, "no commit line"
	}
	if lines[0] != closedRecordHeader {
		return ClosedSpecs{}, "header " + strconv.Quote(lines[0])
	}
	commit, found := strings.CutPrefix(lines[1], closedRecordCommit)
	if !found {
		return ClosedSpecs{}, "no commit line"
	}
	if !commitID(commit) {
		return ClosedSpecs{}, "commit " + strconv.Quote(commit) + " is no commit id"
	}
	closed := ClosedSpecs{commit: commit, known: true, stems: []string{}}
	for _, line := range lines[2:] {
		stem, found := strings.CutPrefix(line, closedRecordStem)
		if !found {
			return ClosedSpecs{}, "line " + strconv.Quote(line)
		}
		if stem == "" {
			return ClosedSpecs{}, "an empty stem"
		}
		closed.stems = append(closed.stems, stem)
	}
	if !slices.IsSorted(closed.stems) {
		return ClosedSpecs{}, "stems out of order"
	}
	return closed, ""
}

// commitID reports whether text is a full SHA-1 or SHA-256 object id in
// lower-case hex, the only form LoadClosedSpecs writes.
func commitID(text string) bool {
	if len(text) != 40 {
		if len(text) != 64 {
			return false
		}
	}
	for _, digit := range text {
		if strings.ContainsRune("0123456789abcdef", digit) {
			continue
		}
		return false
	}
	return true
}

// writeClosedRecord publishes the record with one rename. It needs no sync:
// the record is recomputed from git whenever it is absent or corrupt, and a
// file a power loss cut short lacks its completion mark.
func writeClosedRecord(root string, closed ClosedSpecs) error {
	path := closedRecordPath(root)
	// Every agent account on the host shares tmp/session, as lepath creates it.
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil { //nolint:gosec // shared by every agent account, as the session root is
		return fmt.Errorf("save the closed-spec record: %w", err)
	}
	var text bytes.Buffer
	text.WriteString(closedRecordHeader + "\n" + closedRecordCommit + closed.commit + "\n")
	for _, stem := range closed.stems {
		text.WriteString(closedRecordStem + stem + "\n")
	}
	text.WriteString(closedRecordEnd + "\n")
	if err := derived.WriteAtomicAll([]derived.File{{Path: path, Content: text.Bytes()}}); err != nil {
		return fmt.Errorf("save the closed-spec record: %w", err)
	}
	return nil
}

// closedGit runs one read-only git query in root and answers its stdout; a
// failure keeps git's exit status, which isAncestor reads, and its complaint.
func closedGit(ctx context.Context, root string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "-C", root}, args...)...) //nolint:gosec // fixed git queries; every value is an argv operand
	var stdout, complaint bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &complaint
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(complaint.String()))
	}
	return stdout.String(), nil
}

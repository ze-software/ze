// Design: docs/architecture/testing/verify-freshness-scope.md -- the green record a push is judged against
// Related: lifecycle.go -- the run whose own verdict is the only writer of the record
// Related: sweep.go -- every piece of one cut, run in one invocation
package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// GreenPath is the record of the last green, full, unskipped native
// verification over one commit, relative to the checkout root.
//
// It is not the certificate (`tmp/ze-verify.status`). Every run overwrites the
// certificate, red or green, so the last green verification is gone from it the
// moment a later run fails; and the certificate's tree hash is a snapshot
// digest of a working tree, while this record holds the git tree id a push
// compares with `P^{tree}`.
const GreenPath = "tmp/ze-verify-green.status"

// greenMode is the only mode a green record holds. A record naming another is
// not a green record, because only the full population proves a push.
const greenMode = "full"

// uncutName is the cut field of a record the uncut population proved.
const uncutName = "uncut"

// ErrNotGreen marks a record file that exists and does not prove a push: a
// mode other than full, a skipped suite, a field missing or malformed. It is
// never read as "no record", which is os.ErrNotExist.
var ErrNotGreen = errors.New("not a green full verification record")

// GreenRecord is one green, full, unskipped verification over one commit.
//
// The fields are private so that only NewGreenRecord and ReadGreenRecord make
// one, and both refuse a record that does not prove a push. Its zero value is
// not a record: every accessor of a zero GreenRecord answers empty, and
// WriteGreenRecord refuses it.
type GreenRecord struct {
	commit    string
	tree      string
	pieces    int
	timestamp string
}

// NewGreenRecord validates one green verification over commit, whose git tree
// id is tree. pieces is how the population was dealt: 1 for the uncut run (or
// part 1 of 1), m for a cut of m pieces every one of which passed over commit.
func NewGreenRecord(commit, tree string, pieces int, at time.Time) (GreenRecord, error) {
	record := GreenRecord{commit: commit, tree: tree, pieces: pieces, timestamp: at.UTC().Format(time.RFC3339)}
	if err := record.valid(); err != nil {
		return GreenRecord{}, err
	}
	return record, nil
}

// Commit answers the SHA the verified worktree held.
func (r GreenRecord) Commit() string { return r.commit }

// Tree answers `<commit>^{tree}`, the value a push target's tree is compared with.
func (r GreenRecord) Tree() string { return r.tree }

// Pieces answers how many pieces the population was dealt into; 1 is uncut.
func (r GreenRecord) Pieces() int { return r.pieces }

// Timestamp answers when the record was made, in RFC 3339.
func (r GreenRecord) Timestamp() string { return r.timestamp }

// Text renders the record file, one key=value line per field, in the
// certificate's own format.
func (r GreenRecord) Text() string {
	cut := uncutName
	if r.pieces > 1 {
		cut = strconv.Itoa(r.pieces)
	}
	var text textbuf.Buffer
	text.Str("commit=").Str(r.commit).Byte('\n')
	text.Str("tree=").Str(r.tree).Byte('\n')
	text.Str("mode=").Str(greenMode).Byte('\n')
	text.Str("skipped=").Byte('\n')
	text.Str("cut=").Str(cut).Byte('\n')
	text.Str("timestamp=").Str(r.timestamp).Byte('\n')
	return text.String()
}

func (r GreenRecord) valid() error {
	if !isObjectID(r.commit) {
		return fmt.Errorf("%w: commit %q is not a full git object id", ErrNotGreen, r.commit)
	}
	if !isObjectID(r.tree) {
		return fmt.Errorf("%w: tree %q is not a full git object id", ErrNotGreen, r.tree)
	}
	if r.pieces < 1 {
		return fmt.Errorf("%w: cut of %d pieces", ErrNotGreen, r.pieces)
	}
	if _, err := time.Parse(time.RFC3339, r.timestamp); err != nil {
		return fmt.Errorf("%w: timestamp %q: %w", ErrNotGreen, r.timestamp, err)
	}
	return nil
}

// WriteGreenRecord replaces the record at root atomically, so a reader sees the
// old record or the whole new one.
func WriteGreenRecord(root string, record GreenRecord) error {
	if err := record.valid(); err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(GreenPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("write green record: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".le-green-*")
	if err != nil {
		return fmt.Errorf("write green record: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()

	if _, err := file.WriteString(record.Text()); err != nil {
		_ = file.Close()
		return fmt.Errorf("write green record: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write green record: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("write green record: %w", err)
	}
	return nil
}

// ReadGreenRecord reads the record at root. An absent record is an error
// wrapping os.ErrNotExist, the normal state before any green run; a record that
// does not prove a push is an error wrapping ErrNotGreen. A caller MUST tell
// the two apart from a record, and MUST NOT read either as a green tree.
func ReadGreenRecord(root string) (GreenRecord, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(GreenPath))) //nolint:gosec // a fixed path under the checkout root
	if err != nil {
		return GreenRecord{}, fmt.Errorf("read green record: %w", err)
	}
	fields := make(map[string]string, 6)
	for line := range strings.SplitSeq(string(content), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		fields[key] = value
	}
	if fields["mode"] != greenMode {
		return GreenRecord{}, fmt.Errorf("%w: mode %q", ErrNotGreen, fields["mode"])
	}
	if fields["skipped"] != "" {
		return GreenRecord{}, fmt.Errorf("%w: suites skipped: %s", ErrNotGreen, fields["skipped"])
	}
	pieces := 1
	if fields["cut"] != uncutName {
		pieces, err = strconv.Atoi(fields["cut"])
		if err != nil {
			return GreenRecord{}, fmt.Errorf("%w: cut %q", ErrNotGreen, fields["cut"])
		}
	}
	record := GreenRecord{commit: fields["commit"], tree: fields["tree"], pieces: pieces, timestamp: fields["timestamp"]}
	if err := record.valid(); err != nil {
		return GreenRecord{}, err
	}
	return record, nil
}

// isObjectID answers whether value is a full SHA-1 or SHA-256 git object id.
func isObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// partsPath holds the pieces of one commit's cut verification that have
// passed, at the asking checkout root. It sits beside the other
// `tmp/ze-verify-*` artifacts because it is the same kind of thing: a record of
// what a run proved about a tree. It is never written in the worktree, which
// the run removes.
const partsPath = "tmp/ze-verify-parts.json"

// cutParts is the durable record of which pieces of one commit's cut have
// passed.
type cutParts struct {
	Commit  string `json:"commit"`
	Of      int    `json:"of"`
	Passed  []int  `json:"passed"`
	Updated string `json:"updated"`
}

// recordGreen writes what one finished run proved, at the asking root, and
// answers the diagnostic the run prints about it ("" when there is nothing to
// say). It sets report.Proven and report.Green.
//
// The verdict is the run's OWN (report.Verify, report.Code, report.Commit),
// never the published certificate's: another run of this checkout can rewrite
// that certificate between this run's publish and this read. The certificate
// is read for one fact only, whether the tree moved during the run, and when it
// names another commit that fact is not this run's to read, so nothing is
// written.
//
// An uncut green run writes the green record. A green piece of a cut is added
// to the parts record, and the piece that completes 1..m over one commit and
// one m writes the green record with cut m and forgets the parts record.
func recordGreen(ctx context.Context, root string, part verifyengine.Part, report *Report, deps dependencies) string {
	verification := report.Verify
	// A run attached at admission carries no verification: it judged nothing.
	if verification == nil {
		return ""
	}
	// A run the engine attached to another run's verdict judged nothing either,
	// whatever exit code it carries.
	if verification.Attached != nil {
		return ""
	}
	if report.Code != 0 {
		return ""
	}
	if !verification.Completed {
		return ""
	}
	if verification.Mode != part.Name(verifyengine.Mode) {
		return ""
	}
	var text textbuf.Buffer
	if skipped := verifyengine.SkippedSuites(); skipped != "" {
		return text.Str("verify-worktree: green record not written: suites skipped (ZE_SKIP_SUITES=").
			Str(skipped).Byte(')').String()
	}
	certificate, err := verifyengine.ReadCertificate(root)
	if err != nil {
		return text.Str("verify-worktree: green record not written: ").Err(err).String()
	}
	if certificate.GitSHA != report.Commit {
		return text.Str("verify-worktree: green record not written: certificate names ").
			Str(certificate.GitSHA).Str(", this run verified ").Str(report.Commit).String()
	}
	if certificate.TreeMoved() {
		return text.Str("verify-worktree: green record not written: the tree moved during the run").String()
	}

	pieces := 1
	if part.Cut() {
		passed, err := recordPart(root, report.Commit, part, deps.now())
		if err != nil {
			return text.Str("verify-worktree: parts record not written: ").Err(err).String()
		}
		report.Proven = passed
		if !partsComplete(passed, part.Of) {
			return text.Str("verify-worktree: ").Str(shortSHA(report.Commit)).Str(" pieces proven of ").
				Int(int64(part.Of)).Str(": ").Str(pieceList(passed)).String()
		}
		pieces = part.Of
	}
	tree, err := deps.git(ctx, gitTimeoutMetadata, root, "rev-parse", "--verify", "--quiet", report.Commit+"^{tree}")
	if err != nil {
		return text.Str("verify-worktree: green record not written: ").
			Str(commandFailure("git rev-parse", tree, err)).String()
	}
	if tree.Code != 0 {
		return text.Str("verify-worktree: green record not written: ").
			Str(commandFailure("git rev-parse", tree, nil)).String()
	}
	record, err := NewGreenRecord(report.Commit, strings.TrimSpace(tree.Output), pieces, deps.now())
	if err != nil {
		return text.Str("verify-worktree: green record not written: ").Err(err).String()
	}
	if err := WriteGreenRecord(root, record); err != nil {
		return text.Str("verify-worktree: green record not written: ").Err(err).String()
	}
	report.Green = true
	if part.Cut() {
		if err := forgetParts(root); err != nil {
			return text.Str("verify-worktree: green record written to ").Str(GreenPath).
				Str("; the parts record was not removed: ").Err(err).String()
		}
	}
	return text.Str("verify-worktree: green record written to ").Str(GreenPath).String()
}

// recordPart adds one piece that passed to the parts record, and answers every
// piece proven for commit under this cut.
//
// The record is pinned to the COMMIT and to the CUT, and either one moving
// starts it again. A verdict is evidence about the tree it ran on, so pieces of
// two commits describe two trees. And a different piece count deals the stages
// differently, so pieces of two cuts never add up to the population.
//
// Several sessions share this checkout, so the read, the merge and the write
// happen under one advisory lock.
func recordPart(root, commit string, part verifyengine.Part, at time.Time) ([]int, error) {
	path := filepath.Join(root, filepath.FromSlash(partsPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // the path is this checkout's verification artifact
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // the explicit write/sync result owns the verdict
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN) //nolint:errcheck // process exit releases the advisory lock

	content, err := os.ReadFile(path) //nolint:gosec // the path is this checkout's verification artifact
	if err != nil {
		return nil, err
	}
	held := cutParts{Commit: commit, Of: part.Of, Passed: []int{}}
	var previous cutParts
	// An unreadable record is a record of nothing: it starts again rather than
	// crediting pieces nobody can name.
	if len(content) != 0 && json.Unmarshal(content, &previous) == nil &&
		previous.Commit == commit && previous.Of == part.Of {
		held.Passed = previous.Passed
	}
	if !slices.Contains(held.Passed, part.Index) {
		held.Passed = append(held.Passed, part.Index)
	}
	slices.Sort(held.Passed)
	held.Updated = at.UTC().Format(time.RFC3339)

	rendered, err := json.MarshalIndent(held, "", "  ")
	if err != nil {
		return nil, err
	}
	rendered = append(rendered, '\n')
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	if _, err := file.Write(rendered); err != nil {
		return nil, err
	}
	if err := file.Truncate(int64(len(rendered))); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	return held.Passed, nil
}

// provenParts answers the pieces already recorded green for commit under a cut
// of of pieces. A record pinned to another commit or another cut answers
// nothing, the reset rule recordPart states; so does an absent or unreadable
// record, which a sweep reads as "run every piece".
func provenParts(root, commit string, of int) []int {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(partsPath))) //nolint:gosec // the path is this checkout's verification artifact
	if err != nil {
		return nil
	}
	var held cutParts
	if json.Unmarshal(content, &held) != nil {
		return nil
	}
	if held.Commit != commit {
		return nil
	}
	if held.Of != of {
		return nil
	}
	return held.Passed
}

// partsComplete reports whether every piece of a cut into of pieces is proven.
func partsComplete(passed []int, of int) bool {
	for index := 1; index <= of; index++ {
		if !slices.Contains(passed, index) {
			return false
		}
	}
	return true
}

// forgetParts drops the parts record once the green record holds what it
// proved, so the next cut over this commit starts from the green record.
func forgetParts(root string) error {
	err := os.Remove(filepath.Join(root, filepath.FromSlash(partsPath)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

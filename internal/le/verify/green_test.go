package verify

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestGreenRecordCodecRefusesWhatProvesNoPush round-trips one record through
// the file and then writes, by hand, every record shape that proves no push: a
// mode other than full, a skipped suite, a malformed cut, a short object id.
// Each MUST read as ErrNotGreen, and an absent file MUST read as
// os.ErrNotExist, so no caller can take either for a green tree.
func TestGreenRecordCodecRefusesWhatProvesNoPush(t *testing.T) {
	const (
		commit = "0123456789abcdef0123456789abcdef01234567"
		tree   = "89abcdef0123456789abcdef0123456789abcdef"
	)
	root := t.TempDir()
	if _, err := ReadGreenRecord(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an absent record read as %v, want os.ErrNotExist", err)
	}

	written, err := NewGreenRecord(commit, tree, 3, time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteGreenRecord(root, written); err != nil {
		t.Fatal(err)
	}
	read, err := ReadGreenRecord(root)
	if err != nil {
		t.Fatal(err)
	}
	if read != written {
		t.Fatalf("read %+v, wrote %+v", read, written)
	}

	valid := written.Text()
	for _, bad := range []struct{ why, from, to string }{
		{"a fast mode", "mode=full", "mode=fast"},
		{"a skipped suite", "skipped=\n", "skipped=functional\n"},
		{"a cut that is not a number", "cut=3", "cut=three"},
		{"a cut of zero pieces", "cut=3", "cut=0"},
		{"a short commit id", "commit=" + commit, "commit=0123456"},
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(GreenPath)),
			[]byte(strings.Replace(valid, bad.from, bad.to, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadGreenRecord(root); !errors.Is(err, ErrNotGreen) {
			t.Errorf("%s read as %v, want ErrNotGreen", bad.why, err)
		}
	}
	if _, err := NewGreenRecord(commit, tree, 0, time.Now()); !errors.Is(err, ErrNotGreen) {
		t.Errorf("a record of zero pieces was made: %v", err)
	}
}

// greenCertificate writes the certificate a run publishes at root, naming sha
// and the tree hash, so recordGreen can be judged against a certificate this
// test chose: one of this run, one another run wrote, one whose tree moved.
func greenCertificate(t *testing.T, root, sha, treeHash string) {
	t.Helper()
	certificate := verifyengine.Certificate{Exit: 0, Timestamp: "2026-10-23T12:00:00Z",
		Mode: verifyengine.Mode, GitSHA: sha, TreeHash: treeHash}
	path := filepath.Join(root, filepath.FromSlash(verifyengine.StatusPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(certificate.Text()), 0o600); err != nil {
		t.Fatal(err)
	}
}

// greenRunReport is the lifecycle report of a run that judged commit in the
// population part names and passed, as Run builds it after the stages.
func greenRunReport(commit string, part verifyengine.Part) Report {
	return Report{Commit: commit, Code: 0, Verify: &verifyengine.Report{
		Mode: part.Name(verifyengine.Mode), Commit: commit, Completed: true}}
}

// mustHaveNoGreenRecord fails when a green record exists at root.
func mustHaveNoGreenRecord(t *testing.T, root, why string) {
	t.Helper()
	if record, err := ReadGreenRecord(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s: green record %+v (%v), want none", why, record, err)
	}
}

// TestGreenRecordSurvivesRedRun: a red full verification after a green one
// leaves the green record naming the earlier commit and its tree. The
// certificate is overwritten by the red run, which is why the record exists.
//
// MUTATION: write the record from the certificate, or remove it on a red run,
// and this fails: the record names nothing, or the red commit.
func TestGreenRecordSurvivesRedRun(t *testing.T) {
	repo := newFixtureRepo(t)
	first := repo.git(t, "rev-parse", "HEAD")
	if report := Run(context.Background(), repo.root, Options{}, passingRunner); report.Code != 0 {
		t.Fatalf("green run = %#v", report)
	}
	repo.commit(t, "second", "two\n")
	if report := Run(context.Background(), repo.root, Options{}, failingRunner()); report.Code == 0 {
		t.Fatalf("red run answered 0: %#v", report)
	}
	record, err := ReadGreenRecord(repo.root)
	if err != nil {
		t.Fatalf("the red run removed or spoiled the green record: %v", err)
	}
	if record.Commit() != first {
		t.Errorf("the green record names %s after a red run, want the earlier green %s", record.Commit(), first)
	}
	if want := repo.git(t, "rev-parse", first+"^{tree}"); record.Tree() != want {
		t.Errorf("the green record holds tree %s, want %s", record.Tree(), want)
	}
	if record.Pieces() != 1 {
		t.Errorf("an uncut run recorded a cut of %d", record.Pieces())
	}
}

// TestGreenRecordOnlyForFullUnskippedUncutOrCompletedCut holds the piece rule
// over Options.Part and Options.Parts (moved from the debt-clear tests): a piece
// alone proves its own stages and nothing else; the piece completing 1..m over
// ONE commit and ONE m writes the record with cut m and forgets the parts
// record; pieces over two commits or two values of m never add up. A run with
// a suite skipped, a mode that is not the full population, or a tree that
// moved during the run writes nothing.
func TestGreenRecordOnlyForFullUnskippedUncutOrCompletedCut(t *testing.T) {
	t.Run("a cut completed over one commit", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		first := Run(context.Background(), repo.root, Options{Part: 1, Parts: 3}, passingRunner)
		if first.Code != 0 || first.Green || !slices.Equal(first.Proven, []int{1}) {
			t.Fatalf("part 1 of 3: code %d green %v proven %v, want 0 false [1]", first.Code, first.Green, first.Proven)
		}
		mustHaveNoGreenRecord(t, repo.root, "one piece of three")
		second := Run(context.Background(), repo.root, Options{Part: 2, Parts: 3}, passingRunner)
		if second.Green || !slices.Equal(second.Proven, []int{1, 2}) {
			t.Fatalf("part 2 of 3: green %v proven %v, want false [1 2]", second.Green, second.Proven)
		}
		mustHaveNoGreenRecord(t, repo.root, "two pieces of three")
		third := Run(context.Background(), repo.root, Options{Part: 3, Parts: 3}, passingRunner)
		if !third.Green || !slices.Equal(third.Proven, []int{1, 2, 3}) {
			t.Fatalf("part 3 of 3: green %v proven %v, want true [1 2 3]: %v", third.Green, third.Proven, third.Diagnostics)
		}
		record, err := ReadGreenRecord(repo.root)
		if err != nil {
			t.Fatalf("the piece completing 1..3 wrote no green record: %v", err)
		}
		if record.Commit() != head || record.Pieces() != 3 {
			t.Errorf("the green record names %s cut %d, want %s cut 3", record.Commit(), record.Pieces(), head)
		}
		if _, err := os.Stat(filepath.Join(repo.root, filepath.FromSlash(partsPath))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the parts record survived the green record (%v)", err)
		}
	})
	t.Run("pieces over two commits", func(t *testing.T) {
		repo := newFixtureRepo(t)
		if report := Run(context.Background(), repo.root, Options{Part: 1, Parts: 2}, passingRunner); report.Code != 0 {
			t.Fatalf("part 1 of 2 = %#v", report)
		}
		moved := repo.commit(t, "second", "two\n")
		report := Run(context.Background(), repo.root, Options{Part: 2, Parts: 2}, passingRunner)
		if report.Green || !slices.Equal(report.Proven, []int{2}) {
			t.Errorf("after HEAD moved: green %v proven %v, want false [2]", report.Green, report.Proven)
		}
		mustHaveNoGreenRecord(t, repo.root, "pieces of two commits")
		if held := readParts(t, repo.root); held.Commit != moved {
			t.Errorf("the parts record names %s, want the moved HEAD %s", held.Commit, moved)
		}
	})
	t.Run("pieces of two cuts", func(t *testing.T) {
		repo := newFixtureRepo(t)
		if report := Run(context.Background(), repo.root, Options{Part: 1, Parts: 2}, passingRunner); report.Code != 0 {
			t.Fatalf("part 1 of 2 = %#v", report)
		}
		report := Run(context.Background(), repo.root, Options{Part: 2, Parts: 3}, passingRunner)
		if report.Green || !slices.Equal(report.Proven, []int{2}) {
			t.Errorf("a piece of another cut: green %v proven %v, want false [2]", report.Green, report.Proven)
		}
		mustHaveNoGreenRecord(t, repo.root, "pieces of two cuts")
		if held := readParts(t, repo.root); held.Of != 3 {
			t.Errorf("the parts record holds a cut of %d, want the 3 the last piece ran", held.Of)
		}
	})
	t.Run("a suite skipped", func(t *testing.T) {
		t.Setenv("ZE_SKIP_SUITES", "functional")
		repo := newFixtureRepo(t)
		report := Run(context.Background(), repo.root, Options{}, passingRunner)
		if report.Code != 0 {
			t.Fatalf("green run with a skipped suite = %#v", report)
		}
		mustHaveNoGreenRecord(t, repo.root, "a run with ZE_SKIP_SUITES set")
	})
	t.Run("a mode that is not the full population", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		greenCertificate(t, repo.root, head, "clean")
		report := greenRunReport(head, verifyengine.Uncut())
		report.Verify.Mode = "fast"
		recordGreen(context.Background(), repo.root, verifyengine.Uncut(), &report, realDeps())
		mustHaveNoGreenRecord(t, repo.root, "a fast run")
	})
	t.Run("a tree that moved during the run", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		greenCertificate(t, repo.root, head, "tree-moved-during-run")
		report := greenRunReport(head, verifyengine.Uncut())
		recordGreen(context.Background(), repo.root, verifyengine.Uncut(), &report, realDeps())
		mustHaveNoGreenRecord(t, repo.root, "a run whose tree moved")
	})
	t.Run("an uncut green run, the control", func(t *testing.T) {
		repo := newFixtureRepo(t)
		head := repo.git(t, "rev-parse", "HEAD")
		greenCertificate(t, repo.root, head, "clean")
		report := greenRunReport(head, verifyengine.Uncut())
		recordGreen(context.Background(), repo.root, verifyengine.Uncut(), &report, realDeps())
		if record, err := ReadGreenRecord(repo.root); err != nil || record.Commit() != head || !report.Green {
			t.Errorf("the control wrote (%+v, %v) green %v, want a record of %s", record, err, report.Green, head)
		}
	})
}

// TestGreenRecordIgnoresConcurrentCertificate: another run rewrote the
// published certificate with its own commit between this run's publish and
// its record write. The verdict is this run's own, but the certificate cannot
// say whether this run's tree moved, so nothing is written and the run names
// both commits.
//
// MUTATION: drop the git_sha comparison and this fails: the record is written
// beside a certificate describing another commit.
func TestGreenRecordIgnoresConcurrentCertificate(t *testing.T) {
	repo := newFixtureRepo(t)
	head := repo.git(t, "rev-parse", "HEAD")
	other := strings.Repeat("ab", 20)
	greenCertificate(t, repo.root, other, "clean")
	report := greenRunReport(head, verifyengine.Uncut())
	diagnostic := recordGreen(context.Background(), repo.root, verifyengine.Uncut(), &report, realDeps())
	mustHaveNoGreenRecord(t, repo.root, "a certificate another run wrote")
	want := "verify-worktree: green record not written: certificate names " + other + ", this run verified " + head
	if diagnostic != want {
		t.Errorf("the run printed %q, want %q", diagnostic, want)
	}
	if report.Green {
		t.Error("the report says the green record was written")
	}
}

// TestAttachedRunWritesNoGreenRecord: a run that took another run's verdict
// judged nothing, whatever exit code it carries, so it writes no record. Both
// shapes are held: the engine's Attached, and the lifecycle's admission attach
// that leaves Verify nil.
func TestAttachedRunWritesNoGreenRecord(t *testing.T) {
	repo := newFixtureRepo(t)
	head := repo.git(t, "rev-parse", "HEAD")
	greenCertificate(t, repo.root, head, "clean")

	attached := greenRunReport(head, verifyengine.Uncut())
	attached.Verify.Attached = &verifyengine.Attached{Entry: "tmp/job/entry", Log: "tmp/job/log"}
	recordGreen(context.Background(), repo.root, verifyengine.Uncut(), &attached, realDeps())
	mustHaveNoGreenRecord(t, repo.root, "an engine-attached run")

	// A checkout of its own, so a record the first case wrongly wrote cannot
	// fail this one.
	other := newFixtureRepo(t)
	otherHead := other.git(t, "rev-parse", "HEAD")
	greenCertificate(t, other.root, otherHead, "clean")
	admitted := Report{Commit: otherHead, Code: 0}
	recordGreen(context.Background(), other.root, verifyengine.Uncut(), &admitted, realDeps())
	mustHaveNoGreenRecord(t, other.root, "a run attached at admission")
}

// readParts reads the parts record back.
func readParts(t *testing.T, root string) cutParts {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(partsPath)))
	if err != nil {
		t.Fatalf("read the parts record: %v", err)
	}
	var held cutParts
	if err := json.Unmarshal(content, &held); err != nil {
		t.Fatalf("parse the parts record: %v", err)
	}
	return held
}

// TestForgetPartsKeepsAnotherCommitsRecord: the green write for one commit
// forgets the parts record only while that record still names the commit. A
// recordPart for another commit that landed between the green write and the
// forget holds that commit's progress, and the forget leaves it.
//
// MUTATION: drop the commit check in forgetParts and the other commit's piece
// is removed, so provenParts answers nothing for it.
func TestForgetPartsKeepsAnotherCommitsRecord(t *testing.T) {
	root := t.TempDir()
	at := time.Unix(0, 0)
	if _, err := recordPart(root, "green", verifyengine.Part{Index: 1, Of: 2}, at); err != nil {
		t.Fatal(err)
	}
	if _, err := recordPart(root, "other", verifyengine.Part{Index: 1, Of: 2}, at); err != nil {
		t.Fatal(err)
	}
	if err := forgetParts(root, "green", 2); err != nil {
		t.Fatalf("forget the green commit's parts: %v", err)
	}
	if got := provenParts(root, "other", 2); !slices.Equal(got, []int{1}) {
		t.Fatalf("the other commit's parts after the green commit's forget = %v, want [1]", got)
	}
	if err := forgetParts(root, "other", 2); err != nil {
		t.Fatalf("forget the other commit's parts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(partsPath))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the parts record survived its own commit's forget (%v)", err)
	}
}

// TestForgetPartsKeepsAnotherCutsRecord: the green write for one commit cut
// into 3 forgets the parts record only while that record is still pinned to
// that cut. A sweep of all of 4 over the same commit holds its own progress
// there, and removing it would end that sweep saying no green record was
// written while the record names the commit.
//
// MUTATION: drop the cut check in forgetParts and the all of 4 piece is
// removed, so provenParts answers nothing for it.
func TestForgetPartsKeepsAnotherCutsRecord(t *testing.T) {
	root := t.TempDir()
	if _, err := recordPart(root, "green", verifyengine.Part{Index: 1, Of: 4}, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := forgetParts(root, "green", 3); err != nil {
		t.Fatalf("forget the cut of 3: %v", err)
	}
	if got := provenParts(root, "green", 4); !slices.Equal(got, []int{1}) {
		t.Fatalf("the cut of 4's parts after the cut of 3's forget = %v, want [1]", got)
	}
	if err := forgetParts(root, "green", 4); err != nil {
		t.Fatalf("forget the cut of 4: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(partsPath))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the parts record survived its own cut's forget (%v)", err)
	}
}

// TestForgetPartsWaitsForThePartsLock: forgetParts reads and removes the parts
// record under the lock recordPart takes. The test holds that lock, as a
// recordPart would, and forgetParts must not finish while it is held. Under the
// lock the test then rewrites the record for another commit, the piece a
// recordPart lands between the green write and the forget; once the lock is
// released forgetParts reads that record and leaves it.
//
// MUTATION: make forgetParts read and remove without lockParts, and it finishes
// while the lock is held, removing the record the test is about to rewrite.
func TestForgetPartsWaitsForThePartsLock(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(partsPath))
	at := time.Unix(0, 0)
	if _, err := recordPart(root, "green", verifyengine.Part{Index: 1, Of: 2}, at); err != nil {
		t.Fatal(err)
	}

	held, err := lockParts(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close() //nolint:errcheck // the test reads nothing back from it

	done := make(chan error, 1)
	go func() { done <- forgetParts(root, "green", 2) }()
	select {
	case err := <-done:
		t.Fatalf("forgetParts finished while the parts lock was held (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the parts record is gone while the parts lock was held: %v", err)
	}

	other := []byte(`{"commit": "other", "of": 2, "passed": [1], "updated": "1970-01-01T00:00:00Z"}` + "\n")
	if _, err := held.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := held.Write(other); err != nil {
		t.Fatal(err)
	}
	if err := held.Truncate(int64(len(other))); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("forget after the lock was released: %v", err)
	}
	if got := provenParts(root, "other", 2); !slices.Equal(got, []int{1}) {
		t.Errorf("the piece written under the lock after the forget = %v, want [1]", got)
	}
}

// TestHoldPartsRefusesARemovedFile: the interleaving the lock exists for, made
// deterministic by running its steps in order on one goroutine. A recordPart
// opens the parts record, a forgetParts removes it before that recordPart holds
// the lock, and the lock then lands on a file no reader can see. holdParts
// answers false for that file, both while the path is absent and once another
// recordPart has created a new file there, so lockParts opens the path again
// and the piece is written where provenParts reads.
//
// MUTATION: make holdParts answer true when the path is absent and the first
// assertion fails; skip its SameFile check and the second one does.
func TestHoldPartsRefusesARemovedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(partsPath))
	at := time.Unix(0, 0)
	if _, err := recordPart(root, "green", verifyengine.Part{Index: 1, Of: 2}, at); err != nil {
		t.Fatal(err)
	}
	early, err := os.OpenFile(path, os.O_RDWR, 0o600) //nolint:gosec // the test's own temporary record
	if err != nil {
		t.Fatal(err)
	}
	defer early.Close() //nolint:errcheck // the test reads nothing back from it
	if err := forgetParts(root, "green", 2); err != nil {
		t.Fatalf("forget: %v", err)
	}
	named, err := holdParts(early, path)
	if err != nil {
		t.Fatalf("hold the early file: %v", err)
	}
	if named {
		t.Fatal("holdParts took the lock on a removed file while the path is absent")
	}
	if _, err := recordPart(root, "other", verifyengine.Part{Index: 2, Of: 2}, at); err != nil {
		t.Fatal(err)
	}
	named, err = holdParts(early, path)
	if err != nil {
		t.Fatalf("hold the early file again: %v", err)
	}
	if named {
		t.Fatal("holdParts took the lock on a removed file while the path names a new one")
	}
	if got := provenParts(root, "other", 2); !slices.Equal(got, []int{2}) {
		t.Errorf("the piece recorded after the forget = %v, want [2]", got)
	}
}

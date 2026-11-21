package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/le/verify"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// The drivers below prove the derived push debt through the REAL `le` binary
// over a throwaway checkout with a real bare upstream, and judge what git and
// the product's own artifacts say afterwards: the commit the upstream holds,
// the trailers a commit carries, the green record at the asking root.
//
// A push owes no local verification: no agent runs `le verify`, and GitHub CI
// verifies the pushed commit. The push legs therefore run with no verify
// record at all.
//
// Two legs run in process rather than through `le verify worktree`, because a
// native verification over the real population claims this machine's job
// slots for the better part of an hour, and a throwaway checkout cannot pass
// it. Those legs call verify.Run, the lifecycle `le verify worktree` answers
// through, with a fixture action runner that answers every stage green, so
// everything after the stages (publish, the green record, the parts record)
// is the product's own code.

const (
	derivedDebtAuthorisation = "fixture: the owner ordered this push"
	derivedDebtBranchRef     = "refs/heads/" + gitBranchMain
	derivedDebtFormAll       = "all"  // The `le verify worktree` form that names every part.
	derivedDebtFormPart      = "part" // The `le verify worktree` form that names one part.
	derivedDebtSpecPath      = "plan/immediate/spec-derived-debt-fixture.md"
	derivedDebtSpecStem      = "derived-debt-fixture"
	derivedDebtSpecText      = "# Spec: a fixture\n\n| Field | Value |\n|-------|-------|\n" +
		"| Status | in-progress |\n| Scope | tooling |\n\n## Task\n\nA fixture.\n"
)

// commitPushNeedsNoVerifyRecordDriver checks the real nonauthorizing debt
// answer with no verify record, and the piped issuer's write-free refusal.
// Authorized script shape is covered by TestPushScriptPushesPreparedSHAToUpstream.
func commitPushNeedsNoVerifyRecordDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	head, err := scratch.commit(ctx, "a change no verify ran over", "a.txt")
	if err != nil {
		return err
	}
	if err := scratch.expectDebtStatus(ctx, head, 0, "push debt: nothing owed"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "nothing-owed-without-verify-record") //nolint:errcheck // progress output
	if err := scratch.expectPipedAuthorizationRefusal(ctx, argCommit, "push", derivedDebtAuthorisation); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "push-issuer-refused-without-artifacts") //nolint:errcheck // progress output
	return nil
}

// commitPushScriptRechecksDriver checks debt-status against a fixed P after
// local HEAD moves and after the upstream diverges. The actual generated
// script/private-origin publication controls live in runner_ci_contract_test.go.
func commitPushScriptRechecksDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	prepared, err := scratch.commit(ctx, "the prepared change", "a.txt")
	if err != nil {
		return err
	}
	if _, err := scratch.commit(ctx, "a commit after the script was prepared", "b.txt"); err != nil {
		return err
	}
	if err := scratch.expectDebtStatus(ctx, prepared, 0, "push debt: nothing owed", prepared[:7]); err != nil {
		return err
	}
	if err := scratch.expectPipedAuthorizationRefusal(ctx, argCommit, "push", derivedDebtAuthorisation); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "named-debt-target-survived-local-head-move") //nolint:errcheck // progress output
	fmt.Fprintln(os.Stdout, "push-issuer-refused-without-artifacts")      //nolint:errcheck // progress output
	// The upstream moves to a commit the prepared P does not descend from, as
	// a push from the other machine would move it.
	base, err := scratch.git(ctx, "rev-parse", "@{u}")
	if err != nil {
		return err
	}
	tree, err := scratch.git(ctx, "rev-parse", base+"^{tree}")
	if err != nil {
		return err
	}
	moved, err := scratch.git(ctx, "-c", "user.email=t@t", "-c", "user.name=t", "commit-tree", tree, "-p", base,
		"-m", "the other machine")
	if err != nil {
		return err
	}
	if _, err := scratch.git(ctx, "push", "-q", gitRemoteOrigin, moved+":"+derivedDebtBranchRef); err != nil {
		return err
	}
	if _, err := scratch.git(ctx, "fetch", "-q", gitRemoteOrigin); err != nil {
		return err
	}
	if err := scratch.expectDebtStatus(ctx, prepared, 1, "the upstream moved"); err != nil {
		return err
	}
	if err := scratch.upstreamIs(ctx, moved); err != nil {
		return fmt.Errorf("a nonauthorizing debt query: %w", err)
	}
	fmt.Fprintln(os.Stdout, "named-debt-target-refused-after-upstream-diverged") //nolint:errcheck // progress output
	return nil
}

// verifyRecordsAtAskingRootDriver: a green full run writes the green record at
// the asking root, and a green `le verify changed` writes the scoped record
// naming HEAD; `le commit debt-status` answers nothing owed before, between
// and after, because a push reads neither record.
func verifyRecordsAtAskingRootDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	head, err := scratch.commit(ctx, "a change to verify", "a.txt")
	if err != nil {
		return err
	}
	if err := scratch.expect(ctx, "debt-status before any verify", scratch.leBinary, argCommit, "debt-status"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "push-owes-no-verify") //nolint:errcheck // progress output

	if report := verify.Run(ctx, scratch.repo, verify.Options{}, greenActionRunner); report.Code != 0 {
		return fmt.Errorf("the fixture verification over %s exit=%d: %v", head, report.Code, report.Diagnostics)
	}
	record, err := verify.ReadGreenRecord(scratch.repo)
	if err != nil {
		return fmt.Errorf("no green record at the asking root after a green full run: %w", err)
	}
	if record.Commit() != head {
		return fmt.Errorf("the green record names %s, want %s", record.Commit(), head)
	}
	if record.Pieces() != 1 {
		return fmt.Errorf("the green record holds a cut of %d, want 1", record.Pieces())
	}
	fmt.Fprintln(os.Stdout, "green-record-at-asking-root") //nolint:errcheck // progress output

	// The scoped run's worktree is detached, so it anchors on
	// refs/remotes/origin/HEAD, which a clone sets and a pushed scratch
	// repository does not.
	if _, err := scratch.git(ctx, "remote", "set-head", gitRemoteOrigin, gitBranchMain); err != nil {
		return err
	}
	changed := verify.Options{Mode: verifyengine.ChangedMode}
	if report := verify.Run(ctx, scratch.repo, changed, greenActionRunner); report.Code != 0 {
		return fmt.Errorf("the fixture scoped verification over %s exit=%d: %v", head, report.Code, report.Diagnostics)
	}
	scoped, err := verify.ReadScopedRecord(scratch.repo)
	if err != nil {
		return fmt.Errorf("no scoped record at the asking root after a green changed run: %w", err)
	}
	if scoped.Commit() != head {
		return fmt.Errorf("the scoped record names %s, want %s", scoped.Commit(), head)
	}
	fmt.Fprintln(os.Stdout, "scoped-record-at-asking-root") //nolint:errcheck // progress output

	if err := scratch.expect(ctx, "debt-status after both verify records", scratch.leBinary, argCommit, "debt-status"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "push-still-owes-no-verify") //nolint:errcheck // progress output
	return nil
}

// verifyWorktreeCutSweepDriver: the cut grammar on `le verify worktree` refuses
// every malformed form before any work; one piece alone writes no green
// record; the piece completing 1..m over one commit writes it with cut m; the
// `all of <m>` sweep proves every piece and a second sweep skips them.
func verifyWorktreeCutSweepDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	head, err := scratch.commit(ctx, "a change to verify in pieces", "a.txt")
	if err != nil {
		return err
	}
	for _, form := range [][]string{
		{derivedDebtFormAll, derivedDebtFormPart, "2", "of", "3"}, {derivedDebtFormAll}, {derivedDebtFormPart, "2"},
		{derivedDebtFormPart, "0", "of", "3"}, {derivedDebtFormPart, "4", "of", "3"},
	} {
		// A whole-tree verb, so it runs at a terminal, as Thomas runs it: the
		// cut refusal is the body's, which the terminal gate admits it to.
		out, code, err := terminalCommand(ctx, scratch.repo, scratch.env, scratch.leBinary,
			append([]string{"verify", "worktree"}, form...)...)
		if err != nil {
			return err
		}
		if code != 2 {
			return fmt.Errorf("le verify worktree %s exit=%d, want 2: %s", strings.Join(form, " "), code, out)
		}
		if form[0] == derivedDebtFormAll && !strings.Contains(out, "all of <m>") {
			return fmt.Errorf("le verify worktree %s does not name the all of <m> form: %s", strings.Join(form, " "), out)
		}
	}
	if _, err := os.Stat(filepath.Join(scratch.repo, "tmp", "verify-worktree")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("a refused cut created a worktree directory (%w)", err)
	}
	fmt.Fprintln(os.Stdout, "malformed-cuts-refused") //nolint:errcheck // progress output

	if report := verify.Run(ctx, scratch.repo, verify.Options{Part: 1, Parts: 2}, greenActionRunner); report.Code != 0 {
		return fmt.Errorf("piece 1 of 2 exit=%d: %v", report.Code, report.Diagnostics)
	}
	if _, err := verify.ReadGreenRecord(scratch.repo); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("one piece of two wrote or left a green record (%w)", err)
	}
	fmt.Fprintln(os.Stdout, "one-piece-writes-no-green-record") //nolint:errcheck // progress output

	if report := verify.Run(ctx, scratch.repo, verify.Options{Part: 2, Parts: 2}, greenActionRunner); report.Code != 0 {
		return fmt.Errorf("piece 2 of 2 exit=%d: %v", report.Code, report.Diagnostics)
	}
	record, err := verify.ReadGreenRecord(scratch.repo)
	if err != nil {
		return fmt.Errorf("the piece completing 1..2 wrote no green record at the asking root: %w", err)
	}
	if record.Commit() != head {
		return fmt.Errorf("the green record names %s, want %s", record.Commit(), head)
	}
	if record.Pieces() != 2 {
		return fmt.Errorf("the green record holds a cut of %d, want 2", record.Pieces())
	}
	fmt.Fprintln(os.Stdout, "completing-piece-writes-green-record") //nolint:errcheck // progress output

	// The sweep over a commit the green record does not cover runs every piece
	// and writes the record with the cut; a second sweep over the same commit
	// runs nothing. The host preflight answers ready: the fixture runner runs no
	// stage, so no loopback address is bound.
	moved, err := scratch.commit(ctx, "a change to sweep", "b.txt")
	if err != nil {
		return err
	}
	ready := func(verify.Options) error { return nil }
	sweep := verify.Sweep(ctx, scratch.repo, verify.Options{Parts: 2}, greenActionRunner, ready)
	if sweep.Code != 0 {
		return fmt.Errorf("the sweep over %s exit=%d: %s", moved, sweep.Code, sweep.Text())
	}
	if len(sweep.Ran) != 2 {
		return fmt.Errorf("the sweep ran %v, want both pieces: %s", sweep.Ran, sweep.Text())
	}
	if !sweep.Green {
		return fmt.Errorf("the sweep did not write the green record: %s", sweep.Text())
	}
	record, err = verify.ReadGreenRecord(scratch.repo)
	if err != nil {
		return fmt.Errorf("the sweep wrote no green record at the asking root: %w", err)
	}
	if record.Commit() != moved {
		return fmt.Errorf("after the sweep the green record names %s, want %s", record.Commit(), moved)
	}
	if record.Pieces() != 2 {
		return fmt.Errorf("after the sweep the green record holds a cut of %d, want 2", record.Pieces())
	}
	fmt.Fprintln(os.Stdout, "sweep-proves-every-piece") //nolint:errcheck // progress output

	again := verify.Sweep(ctx, scratch.repo, verify.Options{Parts: 2}, greenActionRunner, ready)
	if again.Code != 0 {
		return fmt.Errorf("the second sweep exit=%d: %s", again.Code, again.Text())
	}
	if len(again.Ran) != 0 {
		return fmt.Errorf("the second sweep ran %v, want none", again.Ran)
	}
	if len(again.Skipped) != 2 {
		return fmt.Errorf("the second sweep skipped %v, want both pieces", again.Skipped)
	}
	fmt.Fprintln(os.Stdout, "second-sweep-skips-proven-pieces") //nolint:errcheck // progress output
	return nil
}

// commitAttestEmptyCommitDriver checks the real debt answer over a Review-owed
// closure and rejects piped push/attest issuance without writes. The authorized
// empty commit and exact Owner-attested consumer remain in TestAttestPreparesEmptyCommit.
func commitAttestEmptyCommitDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	owed, err := scratch.commitMessage(ctx, "a closure nobody reviewed\n\nReview-owed: "+derivedDebtSpecStem+": no reviewer free",
		"", derivedDebtSpecPath)
	if err != nil {
		return err
	}
	if err := scratch.expectDebtStatus(ctx, owed, 1, "Review-owed", "attest"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "review-debt-reported-without-authorization") //nolint:errcheck // progress output
	if err := scratch.expectPipedAuthorizationRefusal(ctx, argCommit, "push", derivedDebtAuthorisation); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "push-issuer-refused-without-artifacts") //nolint:errcheck // progress output
	if err := scratch.expectPipedAuthorizationRefusal(ctx, argCommit, "create", "subject", "owner attests the closure",
		"attest", owed, "Thomas reviewed it in person"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "attest-issuer-refused-without-artifacts") //nolint:errcheck // progress output
	return nil
}

// commitReviewOwedTrailerDriver: `le commit create ... review-override` over a
// closure with no clean review writes a `Review-owed:` trailer and no ledger
// file. A nonauthorizing debt query names the trailer and its attestation;
// piped push issuance independently refuses without changing any artifact.
func commitReviewOwedTrailerDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	out, code, err := scratch.le(ctx, argCommit, "create", "subject", "close the fixture spec",
		"remove", derivedDebtSpecPath, "review-override", "no reviewer free", "no-news", "fixture: no operator-visible change")
	if failure := commandAnswered("create review-override", out, code, err); failure != nil {
		return failure
	}
	scriptPath, err := scriptFrom(scratch.repo, out)
	if err != nil {
		return err
	}
	if err := scratch.expect(ctx, "the closure script", "bash", scriptPath); err != nil {
		return err
	}
	message, err := scratch.git(ctx, "log", "-1", "--format=%B")
	if err != nil {
		return err
	}
	if !strings.Contains(message, "Review-owed: "+derivedDebtSpecStem+": no reviewer free") {
		return fmt.Errorf("the closure does not carry its Review-owed trailer:\n%s", message)
	}
	files, err := scratch.git(ctx, "show", "--name-only", "--format=", "HEAD")
	if err != nil {
		return err
	}
	if strings.Contains(files, "plan/verification-debt") {
		return fmt.Errorf("the closure carries a ledger file: %s", files)
	}
	fmt.Fprintln(os.Stdout, "review-owed-trailer-written") //nolint:errcheck // progress output

	if err := scratch.expectDebtStatus(ctx, "", 1, "Review-owed", "attest"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "review-debt-reported-without-authorization") //nolint:errcheck // progress output
	if err := scratch.expectPipedAuthorizationRefusal(ctx, argCommit, "push", derivedDebtAuthorisation); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "push-issuer-refused-without-artifacts") //nolint:errcheck // progress output
	return nil
}

// greenActionRunner answers every stage registered, completed and green. It is
// the fixture runner the in-process legs hand verify.Run.
func greenActionRunner(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
	return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true}
}

// derivedDebtScratch is one throwaway Ze checkout on main with a bare upstream
// holding its baseline (which carries the fixture spec), and `./le` at its
// root naming the native binary, as the generated scripts expect.
type derivedDebtScratch struct {
	repo     string
	leBinary string
	env      []string
}

func newDerivedDebtScratch(ctx context.Context, args []string) (*derivedDebtScratch, error) {
	if len(args) != 0 {
		return nil, errors.New("derived-debt fixtures take no arguments")
	}
	leBinary, err := nativeLEBinary()
	if err != nil {
		return nil, err
	}
	repo, err := os.MkdirTemp("", "ze-derived-debt-")
	if err != nil {
		return nil, err
	}
	scratch := &derivedDebtScratch{repo: repo, leBinary: leBinary, env: envRootedAt(repo)}
	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod: "module fixture/deriveddebt\n\ngo 1.24\n", fileFeatureGates: contentFeatureGate,
		fileGitIgnore: contentGitIgnoreTmp + "/le\n", derivedDebtSpecPath: derivedDebtSpecText,
	}); err != nil {
		scratch.remove()
		return nil, err
	}
	if err := os.Symlink(leBinary, filepath.Join(repo, "le")); err != nil {
		scratch.remove()
		return nil, err
	}
	remote := repo + ".origin.git"
	for _, step := range [][]string{
		{"branch", "-M", gitBranchMain}, {argInit, "-q", "--bare", "--initial-branch=" + gitBranchMain, remote},
		{"remote", argAdd, gitRemoteOrigin, remote}, {"push", "-q", "-u", gitRemoteOrigin, gitBranchMain},
	} {
		if _, err := scratch.git(ctx, step...); err != nil {
			scratch.remove()
			return nil, err
		}
	}
	return scratch, nil
}

func (s *derivedDebtScratch) remove() {
	_ = os.RemoveAll(s.repo)
	_ = os.RemoveAll(s.repo + ".origin.git")
}

func (s *derivedDebtScratch) le(ctx context.Context, args ...string) (string, int, error) {
	return rawCommand(ctx, s.repo, s.env, s.leBinary, args...)
}

// expect runs one command in the scratch checkout and answers nil when it
// exited zero, naming what ran otherwise.
func (s *derivedDebtScratch) expect(ctx context.Context, what, name string, args ...string) error {
	out, code, err := rawCommand(ctx, s.repo, s.env, name, args...)
	return commandAnswered(what, out, code, err)
}

// git runs one git command in the scratch checkout and answers its trimmed
// output, failing on a non-zero exit.
func (s *derivedDebtScratch) git(ctx context.Context, args ...string) (string, error) {
	out, code, err := rawCommand(ctx, s.repo, os.Environ(), "git", args...)
	if failure := commandAnswered("git "+strings.Join(args, " "), out, code, err); failure != nil {
		return "", failure
	}
	return strings.TrimSpace(out), nil
}

// commit writes one file and commits it directly, the way a commit another
// tool made reaches the branch, and answers the SHA.
func (s *derivedDebtScratch) commit(ctx context.Context, subject, path string) (string, error) {
	return s.commitMessage(ctx, subject, path, "")
}

// commitMessage commits a written path, a removed path, or both, under a
// whole message, and answers the SHA.
func (s *derivedDebtScratch) commitMessage(ctx context.Context, message, write, remove string) (string, error) {
	if write != "" {
		if err := os.WriteFile(filepath.Join(s.repo, write), []byte(message+"\n"), 0o600); err != nil {
			return "", err
		}
		if _, err := s.git(ctx, argAdd, "--", write); err != nil {
			return "", err
		}
	}
	if remove != "" {
		if _, err := s.git(ctx, "rm", "-q", "--", remove); err != nil {
			return "", err
		}
	}
	if _, err := s.git(ctx, argCommit, "-q", "--allow-empty", "-m", message); err != nil {
		return "", err
	}
	return s.git(ctx, "rev-parse", "HEAD")
}

// upstreamIs answers nil when the bare upstream's main holds want.
func (s *derivedDebtScratch) upstreamIs(ctx context.Context, want string) error {
	out, err := s.git(ctx, "ls-remote", gitRemoteOrigin, derivedDebtBranchRef)
	if err != nil {
		return err
	}
	if sha, _, _ := strings.Cut(out, "\t"); sha != want {
		return fmt.Errorf("the upstream holds %q, want %s", sha, want)
	}
	return nil
}

// commandAnswered answers nil when a command ran and exited zero, and
// otherwise an error naming what ran, how it ended and what it printed.
func commandAnswered(what, out string, code int, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w %s", what, err, out)
	}
	if code != 0 {
		return fmt.Errorf("%s exit=%d, want 0: %s", what, code, out)
	}
	return nil
}

// scriptFrom reads the `script=` line le printed, resolved against root.
func scriptFrom(root, out string) (string, error) {
	for line := range strings.SplitSeq(out, "\n") {
		path, found := strings.CutPrefix(strings.TrimSpace(line), "script=")
		if !found {
			continue
		}
		if filepath.IsAbs(path) {
			return path, nil
		}
		return filepath.Join(root, path), nil
	}
	return "", fmt.Errorf("le printed no script= line: %s", out)
}

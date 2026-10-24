package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/verify"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// The drivers below prove the derived verification debt through the REAL `le`
// binary over a throwaway checkout with a real bare upstream, and judge what
// git and the product's own artifacts say afterwards: the commit the upstream
// holds, the trailers a commit carries, the green record at the asking root.
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

// commitPushRefusedByDerivedDebtDriver: a commit with no green record is
// refused by `le commit push`, naming the verify; once a green record covers
// HEAD the push-only script is prepared, re-derives against P and pushes P to
// the upstream branch.
func commitPushRefusedByDerivedDebtDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	head, err := scratch.commit(ctx, "an unverified change", "a.txt")
	if err != nil {
		return err
	}
	out, code, err := scratch.le(ctx, argCommit, "push", derivedDebtAuthorisation)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("le commit push over an unverified HEAD was admitted: %s", out)
	}
	if !strings.Contains(out, "./le verify worktree") {
		return fmt.Errorf("le commit push refused an unverified HEAD without naming ./le verify worktree: %s", out)
	}
	fmt.Fprintln(os.Stdout, "push-refused-without-green") //nolint:errcheck // progress output

	if err := scratch.green(ctx, head); err != nil {
		return err
	}
	script, err := scratch.preparePush(ctx)
	if err != nil {
		return err
	}
	for _, want := range []string{"debt-status commit " + head, head + ":" + derivedDebtBranchRef} {
		if !strings.Contains(script, want) {
			return fmt.Errorf("the push-only script does not carry %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "commit-tree") {
		return fmt.Errorf("the push-only script makes a commit:\n%s", script)
	}
	fmt.Fprintln(os.Stdout, "push-script-prepared-for-head") //nolint:errcheck // progress output
	return nil
}

// commitPushScriptRechecksDriver: the script pushes the prepared P even when a
// commit lands after preparation, and refuses, pushing nothing, once the green
// record it was prepared against is gone.
func commitPushScriptRechecksDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	verified, err := scratch.commit(ctx, "the verified change", "a.txt")
	if err != nil {
		return err
	}
	if err := scratch.green(ctx, verified); err != nil {
		return err
	}
	scriptPath, err := scratch.preparePushPath(ctx)
	if err != nil {
		return err
	}
	if _, err := scratch.commit(ctx, "a commit after the script was prepared", "b.txt"); err != nil {
		return err
	}
	if err := scratch.expect(ctx, 0, "the push-only script", "bash", scriptPath); err != nil {
		return err
	}
	if err := scratch.upstreamIs(ctx, verified); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "script-pushed-the-prepared-commit") //nolint:errcheck // progress output

	latest, err := scratch.commit(ctx, "a second verified change", "c.txt")
	if err != nil {
		return err
	}
	if err := scratch.green(ctx, latest); err != nil {
		return err
	}
	scriptPath, err = scratch.preparePushPath(ctx)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(scratch.repo, filepath.FromSlash(verify.GreenPath))); err != nil {
		return err
	}
	out, code, err := scratch.run(ctx, "bash", scriptPath)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("the push-only script ran with its green record gone: %s", out)
	}
	if err := scratch.upstreamIs(ctx, verified); err != nil {
		return fmt.Errorf("a refused script: %w", err)
	}
	fmt.Fprintln(os.Stdout, "script-refused-without-green") //nolint:errcheck // progress output
	return nil
}

// verifyGreenRecordClearsDerivedDebtDriver: a commit owes a verify until a green
// full run over it writes the green record at the asking root, after which
// `le commit debt-status` answers nothing owed.
func verifyGreenRecordClearsDerivedDebtDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	head, err := scratch.commit(ctx, "a change to verify", "a.txt")
	if err != nil {
		return err
	}
	if err := scratch.expect(ctx, 1, "debt-status before any verify", scratch.leBinary, argCommit, "debt-status"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "debt-owed-before-verify") //nolint:errcheck // progress output

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

	if err := scratch.expect(ctx, 0, "debt-status after a green verify of HEAD", scratch.leBinary, argCommit, "debt-status"); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "derived-debt-cleared") //nolint:errcheck // progress output
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

// commitAttestEmptyCommitDriver: a commit carrying `Review-owed:` holds the
// push; `le commit create attest <commit> "<words>"` with no file prepares an
// empty commit carrying `Owner-attested:`; once it lands HEAD's tree is still
// the green tree, so the push is admitted with P = HEAD and no new verify.
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
	if err := scratch.green(ctx, owed); err != nil {
		return err
	}
	out, code, err := scratch.le(ctx, argCommit, "push", derivedDebtAuthorisation)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("le commit push over an open Review-owed was admitted: %s", out)
	}
	if !strings.Contains(out, "Review-owed") {
		return fmt.Errorf("le commit push refused without naming the open Review-owed: %s", out)
	}
	fmt.Fprintln(os.Stdout, "push-refused-by-review-owed") //nolint:errcheck // progress output

	out, code, err = scratch.le(ctx, argCommit, "create", "subject", "owner attests the closure",
		"attest", owed, "Thomas reviewed it in person")
	if failure := commandAnswered("create attest with no file", out, code, 0, err); failure != nil {
		return failure
	}
	scriptPath, err := scriptFrom(scratch.repo, out)
	if err != nil {
		return err
	}
	if err := scratch.expect(ctx, 0, "the attestation script", "bash", scriptPath); err != nil {
		return err
	}
	tree, err := scratch.git(ctx, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return err
	}
	parentTree, err := scratch.git(ctx, "rev-parse", "HEAD^^{tree}")
	if err != nil {
		return err
	}
	if tree != parentTree {
		return fmt.Errorf("the attestation commit changed the tree: %s, parent %s", tree, parentTree)
	}
	message, err := scratch.git(ctx, "log", "-1", "--format=%B")
	if err != nil {
		return err
	}
	if !strings.Contains(message, "Owner-attested: ") {
		return fmt.Errorf("the attestation commit carries no Owner-attested trailer:\n%s", message)
	}
	if !strings.Contains(message, "Thomas reviewed it in person") {
		return fmt.Errorf("the attestation commit does not carry the owner's words:\n%s", message)
	}
	fmt.Fprintln(os.Stdout, "empty-attestation-commit-landed") //nolint:errcheck // progress output

	head, err := scratch.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	script, err := scratch.preparePush(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(script, head+":"+derivedDebtBranchRef) {
		return fmt.Errorf("the push after the attestation does not push HEAD %s:\n%s", head, script)
	}
	fmt.Fprintln(os.Stdout, "push-admitted-without-new-verify") //nolint:errcheck // progress output
	return nil
}

// commitReviewOwedTrailerDriver: `le commit create ... review-override` over a
// closure with no clean review writes a `Review-owed:` trailer and no ledger
// file, and `le commit push` is refused naming the trailer and the attestation.
func commitReviewOwedTrailerDriver(ctx context.Context, args []string) error {
	scratch, err := newDerivedDebtScratch(ctx, args)
	if err != nil {
		return err
	}
	defer scratch.remove()

	out, code, err := scratch.le(ctx, argCommit, "create", "subject", "close the fixture spec",
		"remove", derivedDebtSpecPath, "review-override", "no reviewer free")
	if failure := commandAnswered("create review-override", out, code, 0, err); failure != nil {
		return failure
	}
	scriptPath, err := scriptFrom(scratch.repo, out)
	if err != nil {
		return err
	}
	if err := scratch.expect(ctx, 0, "the closure script", "bash", scriptPath); err != nil {
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

	head, err := scratch.git(ctx, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if err := scratch.green(ctx, head); err != nil {
		return err
	}
	out, code, err = scratch.le(ctx, argCommit, "push", derivedDebtAuthorisation)
	if err != nil {
		return err
	}
	if code == 0 {
		return fmt.Errorf("le commit push over a Review-owed closure was admitted: %s", out)
	}
	if !strings.Contains(out, "Review-owed") {
		return fmt.Errorf("le commit push refused without naming the Review-owed trailer: %s", out)
	}
	if !strings.Contains(out, "attest") {
		return fmt.Errorf("le commit push refused without naming the attestation that answers it: %s", out)
	}
	fmt.Fprintln(os.Stdout, "push-held-until-attested") //nolint:errcheck // progress output
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
		{"remote", "add", gitRemoteOrigin, remote}, {"push", "-q", "-u", gitRemoteOrigin, gitBranchMain},
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

func (s *derivedDebtScratch) run(ctx context.Context, name string, args ...string) (string, int, error) {
	return rawCommand(ctx, s.repo, s.env, name, args...)
}

// expect runs one command in the scratch checkout and answers nil when it
// exited want, naming what ran otherwise.
func (s *derivedDebtScratch) expect(ctx context.Context, want int, what, name string, args ...string) error {
	out, code, err := rawCommand(ctx, s.repo, s.env, name, args...)
	return commandAnswered(what, out, code, want, err)
}

// git runs one git command in the scratch checkout and answers its trimmed
// output, failing on a non-zero exit.
func (s *derivedDebtScratch) git(ctx context.Context, args ...string) (string, error) {
	out, code, err := rawCommand(ctx, s.repo, os.Environ(), "git", args...)
	if failure := commandAnswered("git "+strings.Join(args, " "), out, code, 0, err); failure != nil {
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

// green writes the record a green full uncut verification over commit leaves,
// standing in for that verification where the scenario is about its reader.
func (s *derivedDebtScratch) green(ctx context.Context, commit string) error {
	tree, err := s.git(ctx, "rev-parse", commit+"^{tree}")
	if err != nil {
		return err
	}
	record, err := verify.NewGreenRecord(commit, tree, 1, time.Now())
	if err != nil {
		return err
	}
	return verify.WriteGreenRecord(s.repo, record)
}

// preparePushPath runs `le commit push` and answers the script path it prints.
func (s *derivedDebtScratch) preparePushPath(ctx context.Context) (string, error) {
	out, code, err := s.le(ctx, argCommit, "push", derivedDebtAuthorisation)
	if failure := commandAnswered("le commit push", out, code, 0, err); failure != nil {
		return "", failure
	}
	return scriptFrom(s.repo, out)
}

// preparePush runs `le commit push` and answers the script it wrote.
func (s *derivedDebtScratch) preparePush(ctx context.Context) (string, error) {
	path, err := s.preparePushPath(ctx)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(path) //nolint:gosec // the script le printed for this scratch checkout
	if err != nil {
		return "", err
	}
	return string(content), nil
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

// commandAnswered answers nil when a command ran and exited want, and
// otherwise an error naming what ran, how it ended and what it printed.
func commandAnswered(what, out string, code, want int, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w %s", what, err, out)
	}
	if code != want {
		return fmt.Errorf("%s exit=%d, want %d: %s", what, code, want, out)
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

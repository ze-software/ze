// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree verification lifecycle
// Package verifyworktree materializes a commit in a fresh detached worktree and
// runs the native pre-commit stages there.
package verifyworktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/verify"
)

const (
	gateName   = "ze-verify-worktree"
	gitTimeout = 30 * time.Second
)

var runSequence atomic.Uint64

// Options selects the commit and whether its detached worktree survives the
// run. An empty Commit means HEAD.
type Options struct {
	Commit string `json:"commit,omitempty"`
	Keep   bool   `json:"keep"`
}

// CleanupFailure is one failed cleanup operation. Cleanup attempts continue so
// one failure cannot prevent prune or hide another failure.
type CleanupFailure struct {
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

// Report is the complete lifecycle and gate verdict.
type Report struct {
	Gate        string           `json:"gate"`
	Commit      string           `json:"commit"`
	Worktree    string           `json:"worktree,omitempty"`
	Code        int              `json:"code"`
	Kept        bool             `json:"kept"`
	Swept       []string         `json:"swept,omitempty"`
	Diagnostics []string         `json:"diagnostics"`
	Logs        string           `json:"logs,omitempty"`
	Verify      *verify.Report   `json:"verify,omitempty"`
	Failure     *verify.Failure  `json:"failure,omitempty"`
	Cleanup     []CleanupFailure `json:"cleanup-failures,omitempty"`
}

type commandResult struct {
	Code   int
	Output string
}

type gitRunner func(context.Context, string, ...string) (commandResult, error)

type dependencies struct {
	git   gitRunner
	now   func() time.Time
	pid   func() int
	alive func(int) bool
}

// Run resolves Options.Commit, creates a detached worktree, runs every native
// stage, preserves red logs, and removes and prunes the worktree unless Keep is
// set. Git is the only subprocess boundary.
func Run(ctx context.Context, root string, options Options, gates verify.GateRunner) Report {
	return run(ctx, root, options, gates, dependencies{
		git: runGit, now: time.Now, pid: os.Getpid, alive: processAlive,
	})
}

func run(ctx context.Context, root string, options Options, gates verify.GateRunner, deps dependencies) (report Report) {
	report = Report{Gate: gateName, Code: 1, Diagnostics: []string{}, Cleanup: []CleanupFailure{}}
	var text textbuf.Buffer
	revision := strings.TrimSpace(options.Commit)
	if revision == "" {
		revision = "HEAD"
	}

	sha, err := resolveCommit(ctx, root, revision, deps.git)
	if err != nil {
		report.Failure = &verify.Failure{Kind: "commit-resolution", Message: err.Error()}
		report.Diagnostics = append(report.Diagnostics,
			text.Str("verify-worktree: ").Err(err).String())
		return report
	}
	report.Commit = sha

	base := filepath.Join(root, "tmp", "verify-worktree")
	if err := os.MkdirAll(base, 0o750); err != nil {
		report.Failure = &verify.Failure{Kind: "worktree-setup",
			Message: text.Reset().Str("create worktree directory: ").Err(err).String()}
		return report
	}

	swept, sweepDiagnostics, sweepFailures := sweepAbandoned(ctx, root, base, deps)
	report.Swept = swept
	report.Diagnostics = append(report.Diagnostics, sweepDiagnostics...)
	for _, path := range swept {
		report.Diagnostics = append(report.Diagnostics, text.Reset().
			Str("verify-worktree: swept abandoned ").Str(filepath.Base(path)).String())
	}
	if len(sweepFailures) != 0 {
		report.Cleanup = append(report.Cleanup, sweepFailures...)
		report.Failure = &verify.Failure{Kind: "stale-worktree-cleanup", Message: cleanupMessage(sweepFailures)}
		return report
	}
	if prune := pruneWorktrees(ctx, root, deps.git); prune != nil {
		report.Failure = &verify.Failure{Kind: "stale-registration-cleanup", Message: prune.Error()}
		report.Diagnostics = append(report.Diagnostics,
			text.Reset().Str("verify-worktree: ").Err(prune).String())
		return report
	}

	stamp := text.Reset().Str(deps.now().UTC().Format("20060102T150405.000000000Z")).
		Str("-p").Int(int64(deps.pid())).Str("-r").Uint(runSequence.Add(1)).String()
	path := worktreePath(root, sha, stamp)
	report.Worktree = path
	add, addErr := deps.git(ctx, root, "worktree", "add", "--detach", path, sha)
	if addErr != nil || add.Code != 0 {
		report.Failure = &verify.Failure{Kind: "worktree-add", Message: commandFailure("git worktree add", add, addErr)}
		report.Diagnostics = append(report.Diagnostics, text.Reset().
			Str("verify-worktree: git worktree add failed for ").Str(shortSHA(sha)).
			Str(": ").Str(strings.TrimSpace(add.Output)).String())
		return report
	}

	cleanup := !options.Keep
	defer func() {
		if !cleanup {
			return
		}
		cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), gitTimeout)
		defer cancel()
		report.Cleanup = removeWorktree(cleanupContext, root, path, deps.git)
		if len(report.Cleanup) != 0 {
			report.Code = 1
			report.Failure = &verify.Failure{Kind: "cleanup", Message: cleanupMessage(report.Cleanup)}
			for _, failure := range report.Cleanup {
				report.Diagnostics = append(report.Diagnostics, text.Reset().
					Str("verify-worktree: cleanup ").Str(failure.Operation).
					Str(" failed: ").Str(failure.Message).String())
			}
		}
	}()

	branch, branchErr := deps.git(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr != nil {
		report.Failure = &verify.Failure{Kind: "branch-check", Message: branchErr.Error()}
		return report
	}
	if branch.Code == 0 {
		report.Failure = &verify.Failure{Kind: "branch-refusal", Message: text.Reset().
			Str("worktree is on branch ").Quoted(strings.TrimSpace(branch.Output)).
			Str("; detached worktree required").String()}
		return report
	}
	if branch.Code != 1 {
		report.Failure = &verify.Failure{Kind: "branch-check", Message: commandFailure("git symbolic-ref", branch, nil)}
		return report
	}

	if err := os.Mkdir(filepath.Join(path, "tmp"), 0o750); err != nil && !errors.Is(err, os.ErrExist) {
		report.Failure = &verify.Failure{Kind: "worktree-setup",
			Message: text.Reset().Str("create worktree tmp: ").Err(err).String()}
		return report
	}
	if err := os.WriteFile(ownerMarker(path),
		text.Reset().Int(int64(deps.pid())).Byte('\n').Bytes(), 0o600); err != nil {
		report.Failure = &verify.Failure{Kind: "owner-marker",
			Message: text.Reset().Str("write owner marker: ").Err(err).String()}
		return report
	}

	report.Diagnostics = append(report.Diagnostics,
		text.Reset().Str("verify-worktree: ").Str(shortSHA(sha)).Str(" -> ").Str(path).String(),
		text.Reset().Str("verify-worktree: native ").Str(verify.Mode).String(),
	)
	gateReport := verify.Run(ctx, path, sha, gates)
	report.Verify = &gateReport
	report.Code = gateReport.Code
	report.Diagnostics = append(report.Diagnostics, text.Reset().Str("verify-worktree: ").
		Str(verify.Mode).Str(" exit=").Int(int64(report.Code)).String())
	if gateReport.Failure != nil {
		report.Failure = gateReport.Failure
	} else if gateReport.Code != 0 {
		for _, stage := range gateReport.Stages {
			if stage.Failure != nil {
				report.Failure = stage.Failure
				break
			}
		}
	}
	if gateReport.Code != 0 {
		logs, logErr := saveLogs(root, path, filepath.Base(path))
		switch {
		case logErr != nil:
			report.Code = 1
			report.Failure = &verify.Failure{Kind: "log-save", Message: logErr.Error()}
			report.Diagnostics = append(report.Diagnostics,
				text.Reset().Str("verify-worktree: save logs failed: ").Err(logErr).String())
		case logs == "":
			report.Diagnostics = append(report.Diagnostics, text.Reset().
				Str("verify-worktree: the gate wrote no stage logs (").Str(path).
				Str("/tmp/verify/ is absent), so it went red before the first stage").String())
		default:
			report.Logs = logs
			report.Diagnostics = append(report.Diagnostics,
				text.Reset().Str("verify-worktree: logs saved to ").Str(logs).String())
		}
	}
	if options.Keep {
		cleanup = false
		report.Kept = true
		report.Diagnostics = append(report.Diagnostics,
			text.Reset().Str("verify-worktree: kept ").Str(path).String())
	}
	return report
}

func resolveCommit(ctx context.Context, root, revision string, git gitRunner) (string, error) {
	var text textbuf.Buffer
	result, err := git(ctx, root, "rev-parse", "--verify",
		text.Str(revision).Str("^{commit}").Slice())
	if err != nil || result.Code != 0 {
		return "", fmt.Errorf("%s does not name a commit", revision)
	}
	sha := strings.TrimSpace(result.Output)
	if sha == "" {
		return "", fmt.Errorf("%s resolved to an empty commit", revision)
	}
	return sha, nil
}

func worktreePath(root, sha, stamp string) string {
	var text textbuf.Buffer
	name := text.Str(stamp).Byte('-').Str(shortSHA(sha)).String()
	return filepath.Join(root, "tmp", "verify-worktree", name)
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

func ownerMarker(path string) string {
	var text textbuf.Buffer
	return text.Str(path).Str(".owner").String()
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func runGit(parent context.Context, root string, args ...string) (commandResult, error) {
	ctx, cancel := context.WithTimeout(parent, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- the executable and Git subcommands are fixed; revisions are data arguments.
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	result := commandResult{Output: string(output)}
	if err == nil {
		return result, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		result.Code = gaterun.ExitCode(exit)
		return result, nil
	}

	return result, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

func commandFailure(operation string, result commandResult, err error) string {
	var text textbuf.Buffer
	if err != nil {
		return text.Str(operation).Str(": ").Err(err).String()
	}
	detail := strings.TrimSpace(result.Output)
	if detail == "" {
		return text.Str(operation).Str(" exited ").Int(int64(result.Code)).String()
	}
	return text.Str(operation).Str(" exited ").Int(int64(result.Code)).
		Str(": ").Str(detail).String()
}

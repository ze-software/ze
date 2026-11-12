package fixture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const leJobAdmitsHelperEnv = "ZE_TEST_LE_JOB_ADMITS_HELPER"

// uiLeJobAdmitsCheckout makes the scratch registry tree a Git checkout with one
// commit, because admission fingerprints a job's inputs from HEAD and refuses
// a tree whose HEAD it cannot read (job.InputHash). The commit ignores tmp/,
// as the real checkout does, so the registry the first job writes there is not
// an input change the identical second job would see.
func uiLeJobAdmitsCheckout(ctx context.Context, root string) error {
	if err := os.WriteFile(filepath.Join(root, fileGitIgnore), []byte("tmp/*\n"), 0o600); err != nil {
		return fmt.Errorf("write scratch .gitignore: %w", err)
	}
	env := childEnvironment(os.Environ(), map[string]string{
		envGitConfigGlobal: os.DevNull,
		envGitConfigSystem: "1",
		envGitAuthorName:   gitFixtureName,
		envGitAuthorEmail:  gitFixtureEmail,
		envGitCommitName:   gitFixtureName,
		envGitCommitEmail:  gitFixtureEmail,
	})
	commands := [][]string{
		{argInit, argQuiet, gitInitNoTemplate, gitInitFixtureBranch},
		{argAdd, "--", fileGitIgnore},
		{"-c", "core.hooksPath=" + os.DevNull, "-c", gitCommitNoSign, argCommit, argQuiet, "-m", "Fixture checkout"},
	}
	for _, args := range commands {
		command := exec.CommandContext(ctx, "git", args...) //nolint:gosec // the fixture chooses the program and its arguments
		command.Dir = root
		command.Env = env
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v in the scratch registry tree: %w\n%s", args, err, output)
		}
	}
	return nil
}

func init() {
	Register("ui/le-job-admits", uiDriver(leJobAdmits))
}

type leJobAdmitsResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func leJobAdmits(ctx context.Context) error {
	switch os.Getenv(leJobAdmitsHelperEnv) {
	case "coded":
		os.Exit(3)
		return nil
	case "shared":
		root := os.Getenv("ZE_REPO_ROOT")
		marker := filepath.Join(root, "tmp", "ran")
		file, err := os.OpenFile(marker, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return fmt.Errorf("open shared-run marker: %w", err)
		}
		if _, err := file.WriteString("once\n"); err != nil {
			_ = file.Close()
			return fmt.Errorf("write shared-run marker: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close shared-run marker: %w", err)
		}
		fmt.Fprintln(os.Stdout, "THE-SHARED-OUTPUT") //nolint:errcheck // progress output
		// The holder stays running until the driver closes its stdin, which
		// `le job run` hands to the command it runs. The driver releases it once
		// the follower has attached, so the overlap is a rendezvous on state,
		// never a guessed duration. A follower that ran its own copy reads the
		// null device here and ends at once.
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			return fmt.Errorf("wait for the driver to release the shared run: %w", err)
		}
		os.Exit(3)
		return nil
	case outcomeSuccess:
		os.Exit(0)
		return nil
	}

	checkout := os.Getenv("ZE_REPO_ROOT")
	if checkout == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}

	testBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the le executable: %w", err)
	}
	testBinary, err = filepath.Abs(testBinary)
	if err != nil {
		return fmt.Errorf("make the le executable path absolute: %w", err)
	}

	testDir, err := os.MkdirTemp("", "ze-le-job-admits-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(testDir) //nolint:errcheck // fixture cleanup

	binary, err := nativeLEBinary()
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	scratch := filepath.Join(testDir, "tree")
	if err := os.MkdirAll(filepath.Join(scratch, "tmp"), 0o750); err != nil {
		return fmt.Errorf("create scratch registry tree: %w", err)
	}
	if err := uiLeJobAdmitsCheckout(ctx, scratch); err != nil {
		return err
	}

	environment := withoutEnv(os.Environ(), "ZE_RUN_JOB", leJobAdmitsHelperEnv)
	environment = uiLeJobAdmitsWithEnv(environment, "ZE_REPO_ROOT", scratch)
	environment = uiLeJobAdmitsWithEnv(environment, "MAKEFLAGS", "")

	runLE := func(runCtx context.Context, env []string, args ...string) leJobAdmitsResult {
		command := exec.CommandContext(runCtx, binary, args...) //nolint:gosec // the fixture chooses the program and its arguments
		command.Dir = scratch
		command.Env = env
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		err := command.Run()
		return leJobAdmitsResult{
			stdout:   stdout.String(),
			stderr:   stderr.String(),
			exitCode: commandExitCode(err),
		}
	}

	listing := runLE(ctx, environment, "job", "|", "json")
	var actions struct {
		Actions []struct {
			Verb string `json:"verb"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(listing.stdout), &actions); err != nil {
		return fmt.Errorf("`le job | json` did not answer JSON: %w\n%s", err, listing.stdout)
	}
	hasRun := false
	for _, action := range actions.Actions {
		if action.Verb == actionRun {
			hasRun = true
			break
		}
	}
	if !hasRun {
		return fmt.Errorf("`le job` names no run action: %+v", actions)
	}

	codedEnv := uiLeJobAdmitsWithEnv(environment, leJobAdmitsHelperEnv, "coded")
	coded := runLE(ctx, codedEnv,
		"job", "run", "label", "ui-coded", "command",
		testBinary, "test", "fixture", "ui/le-job-admits",
	)
	if coded.exitCode != 3 {
		return fmt.Errorf("a job that exited 3 answered %d: %s", coded.exitCode, coded.stderr)
	}

	sharedEnv := uiLeJobAdmitsWithEnv(environment, leJobAdmitsHelperEnv, "shared")
	holder := exec.CommandContext(ctx, binary, //nolint:gosec // the fixture chooses the program and its arguments
		"job", "run", "label", leJobAdmitsSharedLabel, "command",
		testBinary, "test", "fixture", "ui/le-job-admits",
	)
	holder.Dir = scratch
	holder.Env = sharedEnv
	// The holder's command blocks on this stdin (the "shared" helper above), so
	// closing it is the one release of the shared run.
	holderStdin, err := holder.StdinPipe()
	if err != nil {
		return fmt.Errorf("open holder stdin: %w", err)
	}
	holderStdout, err := holder.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open holder stdout: %w", err)
	}
	var holderStderr bytes.Buffer
	holder.Stderr = &holderStderr
	if err := holder.Start(); err != nil {
		return fmt.Errorf("start holder: %w", err)
	}
	holderFinished := false
	holderDrained := make(chan struct{})
	holderDraining := false
	defer func() {
		if !holderFinished && holder.Process != nil {
			_ = holderStdin.Close()
			_ = holder.Process.Kill()
			_ = holder.Wait()
		}
		if holderDraining {
			<-holderDrained
		}
	}()

	reader := bufio.NewReader(holderStdout)
	announced, readErr := reader.ReadString('\n')
	if !strings.Contains(announced, "THE-SHARED-OUTPUT") {
		_ = holderStdin.Close()
		_ = holder.Process.Kill()
		_ = holder.Wait()
		holderFinished = true
		if readErr != nil {
			return fmt.Errorf("the first job never reached its command: %q: %w", announced, readErr)
		}
		return fmt.Errorf("the first job never reached its command: %q", announced)
	}
	holderDraining = true
	go leJobAdmitsDrain(reader, holderDrained)

	follower, err := leJobAdmitsStartFollower(ctx, binary, scratch, sharedEnv, testBinary)
	if err != nil {
		return err
	}
	followerFinished := false
	defer func() {
		if !followerFinished {
			_ = follower.command.Process.Kill()
			<-follower.done
		}
	}()

	// The rendezvous: the holder is released when the follower has announced
	// that it attached, when the follower ended without attaching, at the
	// bound, or at the caller's stop. Every arm closes the holder's stdin, so no
	// arm leaves the holder waiting.
	var followerWaitErr error
	release := time.NewTimer(leJobAdmitsAttachWait)
	select {
	case <-follower.attached:
	case followerWaitErr = <-follower.done:
		followerFinished = true
	case <-release.C:
	case <-ctx.Done():
	}
	release.Stop()
	if err := holderStdin.Close(); err != nil {
		return fmt.Errorf("release the holder: %w", err)
	}

	holderWait := make(chan error, 1)
	go leJobAdmitsWait(holder, holderWait)
	var holderWaitErr error
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	select {
	case holderWaitErr = <-holderWait:
		holderFinished = true
	case <-timer.C:
		_ = holder.Process.Kill()
		<-holderWait
		holderFinished = true
		return fmt.Errorf("the holder did not finish within 60s: %s", holderStderr.String())
	case <-ctx.Done():
		_ = holder.Process.Kill()
		<-holderWait
		holderFinished = true
		return ctx.Err()
	}

	if !followerFinished {
		select {
		case followerWaitErr = <-follower.done:
			followerFinished = true
		case <-timer.C:
			_ = follower.command.Process.Kill()
			<-follower.done
			followerFinished = true
			return fmt.Errorf("the follower did not finish within 60s of the release: %s", follower.stderrText())
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	holderCode := commandExitCode(holderWaitErr)
	if holderCode != 3 {
		return fmt.Errorf("the holder answered %d: %s", holderCode, holderStderr.String())
	}
	followerCode := commandExitCode(followerWaitErr)
	if followerCode != 3 {
		return fmt.Errorf("the follower answered %d, want the shared job own 3: %s", followerCode, follower.stderrText())
	}

	marker := filepath.Join(scratch, "tmp", "ran")
	markerContents, err := os.ReadFile(marker) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read shared-run marker: %w", err)
	}
	if string(markerContents) != "once\n" {
		return errors.New("the command ran twice: the follower ran its own copy")
	}
	followerStdout := follower.stdout.String()
	if !strings.Contains(followerStdout, "THE-SHARED-OUTPUT") {
		preview := followerStdout
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return fmt.Errorf("the follower did not replay the shared run: %q", preview)
	}
	if !follower.attachedSeen() {
		return fmt.Errorf("the follower never attached to the running holder: %s", follower.stderrText())
	}

	entries, err := filepath.Glob(filepath.Join(scratch, "tmp", ".ze-jobs", "*.job"))
	if err != nil {
		return fmt.Errorf("inspect finished-job entries: %w", err)
	}
	if len(entries) != 0 {
		return errors.New("a finished job left its entry behind, so its slot is held for ever")
	}

	successEnv := uiLeJobAdmitsWithEnv(environment, leJobAdmitsHelperEnv, "success")
	escaped := runLE(ctx, successEnv,
		"job", "run", "label", "../escape", "command",
		testBinary, "test", "fixture", "ui/le-job-admits",
	)
	if escaped.exitCode == 0 {
		return errors.New("a label that is not a path component was accepted")
	}

	unknown := runLE(ctx, environment, "job", "walk")
	if unknown.exitCode != 2 {
		return fmt.Errorf("an unknown action answered %d, want 2: a caller reads it apart from a failed job", unknown.exitCode)
	}

	fmt.Println("OK")
	return nil
}

// leJobAdmitsSharedLabel names the job the holder and the follower both run.
const leJobAdmitsSharedLabel = "ui-shared"

// leJobAdmitsAttachBanner is the text `le job run` writes to stderr when a job
// attaches to an identical running one. The source is Admission.attach in
// internal/le/job/attach.go: "[<label>] attaching to the <label> already
// running over these inputs (pid <n>)". This copy holds the label-bound span,
// so a change of that banner turns this fixture red rather than vacuous.
const leJobAdmitsAttachBanner = "] attaching to the " + leJobAdmitsSharedLabel + " already running over these inputs"

// leJobAdmitsAttachWait bounds how long the holder is kept running while the
// follower has not attached. The release closes the holder's stdin on every
// arm, so the bound ends a broken rendezvous rather than pacing a working one.
const leJobAdmitsAttachWait = 30 * time.Second

// leJobAdmitsFollower is the second identical job, started while the holder
// runs.
//
// Lifecycle: leJobAdmitsStartFollower starts the process and two goroutines.
// scan copies every stderr line and closes attached at the first line holding
// the attach banner; it closes scanned when the pipe ends, which the process
// exit causes. wait receives on scanned before it calls Wait, because exec
// forbids Wait before the reads from a pipe complete, then sends the result on
// done. The caller MUST receive from done once, killing the process first when
// it gives up; both goroutines have ended by then. stderr is read only after
// done has answered. Not safe for concurrent use beyond that sequence.
type leJobAdmitsFollower struct {
	command  *exec.Cmd
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	attached chan struct{}
	scanned  chan struct{}
	done     chan error
}

func leJobAdmitsStartFollower(ctx context.Context, binary, scratch string, env []string, testBinary string) (*leJobAdmitsFollower, error) {
	follower := &leJobAdmitsFollower{
		attached: make(chan struct{}),
		scanned:  make(chan struct{}),
		done:     make(chan error, 1),
	}
	follower.command = exec.CommandContext(ctx, binary, //nolint:gosec // the fixture chooses the program and its arguments
		"job", "run", "label", leJobAdmitsSharedLabel, "command",
		testBinary, "test", "fixture", "ui/le-job-admits",
	)
	follower.command.Dir = scratch
	follower.command.Env = env
	follower.command.Stdout = &follower.stdout
	stderr, err := follower.command.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open follower stderr: %w", err)
	}
	if err := follower.command.Start(); err != nil {
		return nil, fmt.Errorf("start follower: %w", err)
	}
	go follower.scan(stderr)
	go follower.wait()
	return follower, nil
}

// scan copies the follower's stderr line by line until the pipe ends. A read
// error other than the end of the pipe also ends the copy: the process exit
// closes the pipe either way, and wait then answers.
func (f *leJobAdmitsFollower) scan(stderr io.Reader) {
	defer close(f.scanned)
	reader := bufio.NewReader(stderr)
	signaled := false
	for {
		line, err := reader.ReadString('\n')
		f.stderr.WriteString(line)
		if !signaled && strings.Contains(line, leJobAdmitsAttachBanner) {
			signaled = true
			close(f.attached)
		}
		if err != nil {
			return
		}
	}
}

func (f *leJobAdmitsFollower) wait() {
	<-f.scanned
	f.done <- f.command.Wait()
}

// attachedSeen reports whether scan met the attach banner. MUST be called only
// after done has answered.
func (f *leJobAdmitsFollower) attachedSeen() bool {
	select {
	case <-f.attached:
		return true
	default:
		return false
	}
}

// stderrText is the follower's stderr. MUST be called only after done has
// answered, when scan has stopped writing it.
func (f *leJobAdmitsFollower) stderrText() string {
	return f.stderr.String()
}

// leJobAdmitsDrain discards the holder's stdout after its first line, so the
// holder never blocks on a full pipe, and closes drained when the pipe ends.
// The holder's Wait closes the pipe, so this ends no later than that Wait.
func leJobAdmitsDrain(reader io.Reader, drained chan<- struct{}) {
	defer close(drained)
	// Only the first line is judged, so the error of this copy is discarded.
	_, _ = io.Copy(io.Discard, reader)
}

// leJobAdmitsWait bridges one process's Wait onto a channel the caller selects
// on beside its bound. The channel holds one result, so this ends even when the
// caller has stopped listening.
func leJobAdmitsWait(command *exec.Cmd, result chan<- error) {
	result <- command.Wait()
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func withoutEnv(environment []string, names ...string) []string {
	// The names are compared the way internal/core/env reads them, so a
	// variable this fixture drops leaves under every spelling it arrived in
	// (childEnvironment, fixture.go).
	removed := make(map[string]struct{}, len(names))
	for _, name := range names {
		removed[environmentKeyReading(name)] = struct{}{}
	}
	answer := make([]string, 0, len(environment))
	for _, entry := range environment {
		name := entry
		if before, _, found := strings.Cut(entry, "="); found {
			name = before
		}
		if _, found := removed[environmentKeyReading(name)]; !found {
			answer = append(answer, entry)
		}
	}
	return answer
}

func uiLeJobAdmitsWithEnv(environment []string, name, value string) []string {
	return childEnvironment(environment, map[string]string{name: value})
}

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
		time.Sleep(3 * time.Second)
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
		return fmt.Errorf("locate ze-test executable: %w", err)
	}
	testBinary, err = filepath.Abs(testBinary)
	if err != nil {
		return fmt.Errorf("make ze-test executable path absolute: %w", err)
	}

	testDir, err := os.MkdirTemp("", "ze-le-job-admits-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(testDir) //nolint:errcheck // fixture cleanup

	binary, err := uiLEBinary(checkout)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	scratch := filepath.Join(testDir, "tree")
	if err := os.MkdirAll(filepath.Join(scratch, "tmp"), 0o750); err != nil {
		return fmt.Errorf("create scratch registry tree: %w", err)
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
		testBinary, "fixture", "ui/le-job-admits",
	)
	if coded.exitCode != 3 {
		return fmt.Errorf("a job that exited 3 answered %d: %s", coded.exitCode, coded.stderr)
	}

	sharedEnv := uiLeJobAdmitsWithEnv(environment, leJobAdmitsHelperEnv, "shared")
	holder := exec.CommandContext(ctx, binary, //nolint:gosec // the fixture chooses the program and its arguments
		"job", "run", "label", "ui-shared", "command",
		testBinary, "fixture", "ui/le-job-admits",
	)
	holder.Dir = scratch
	holder.Env = sharedEnv
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
	defer func() {
		if !holderFinished && holder.Process != nil {
			_ = holder.Process.Kill()
			_ = holder.Wait()
		}
	}()

	reader := bufio.NewReader(holderStdout)
	announced, readErr := reader.ReadString('\n')
	if !strings.Contains(announced, "THE-SHARED-OUTPUT") {
		_ = holder.Process.Kill()
		_ = holder.Wait()
		holderFinished = true
		if readErr != nil {
			return fmt.Errorf("the first job never reached its command: %q: %w", announced, readErr)
		}
		return fmt.Errorf("the first job never reached its command: %q", announced)
	}
	go func() {
		_, _ = io.Copy(io.Discard, reader)
	}()

	follower := runLE(ctx, sharedEnv,
		"job", "run", "label", "ui-shared", "command",
		testBinary, "fixture", "ui/le-job-admits",
	)

	holderWait := make(chan error, 1)
	go func() { holderWait <- holder.Wait() }()
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

	holderCode := commandExitCode(holderWaitErr)
	if holderCode != 3 {
		return fmt.Errorf("the holder answered %d: %s", holderCode, holderStderr.String())
	}
	if follower.exitCode != 3 {
		return fmt.Errorf("the follower answered %d, want the shared job own 3: %s", follower.exitCode, follower.stderr)
	}

	marker := filepath.Join(scratch, "tmp", "ran")
	markerContents, err := os.ReadFile(marker) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read shared-run marker: %w", err)
	}
	if string(markerContents) != "once\n" {
		return errors.New("the command ran twice: the follower ran its own copy")
	}
	if !strings.Contains(follower.stdout, "THE-SHARED-OUTPUT") {
		preview := follower.stdout
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return fmt.Errorf("the follower did not replay the shared run: %q", preview)
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
		testBinary, "fixture", "ui/le-job-admits",
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
	removed := make(map[string]struct{}, len(names))
	for _, name := range names {
		removed[name] = struct{}{}
	}
	answer := make([]string, 0, len(environment))
	for _, entry := range environment {
		name := entry
		if before, _, found := strings.Cut(entry, "="); found {
			name = before
		}
		if _, found := removed[name]; !found {
			answer = append(answer, entry)
		}
	}
	return answer
}

func uiLeJobAdmitsWithEnv(environment []string, name, value string) []string {
	answer := withoutEnv(environment, name)
	return append(answer, name+"="+value)
}

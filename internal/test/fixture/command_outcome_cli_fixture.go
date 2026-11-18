// Design: docs/architecture/api/ipc_protocol.md -- completed SSH client output.
// Related: command_outcome_fixture.go -- the actual external SDK provider.
// Related: command_outcome_surfaces_fixture.go -- independent item/fault checks.
// Related: deployment_remote_trust.go -- isolated child environment overrides.

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/command-outcome-cli", commandOutcomeCLI, StopsDaemon)
}

// commandOutcomeCLI reaches the real daemon through the runner's ze executable,
// not an injected stream or a scripted SSH server. A successful provider answer
// precedes each target answer, so a connection failure cannot satisfy the test.
func commandOutcomeCLI(ctx context.Context, args []string) error {
	address, outcome, err := commandOutcomeSurfaceArgs(args)
	if err != nil {
		return err
	}
	return Observe(ctx, "answer-outcome-observer", &sdk.Registration{}, func(ctx context.Context, _ *sdk.Plugin) error {
		binary, err := uiZEBinary()
		if err != nil {
			return err
		}
		binary, err = filepath.Abs(binary)
		if err != nil {
			return err
		}
		root, err := os.MkdirTemp(".", "command-outcome-cli-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(root) //nolint:errcheck // Remove only this fixture's private runner-owned directory.
		root, err = filepath.Abs(root)
		if err != nil {
			return err
		}
		if err := commandOutcomeCLIReady(ctx, address); err != nil {
			return err
		}
		for index, question := range commandOutcomeQuestions(outcome) {
			if err := commandOutcomeCLICheck(ctx, binary, root, address, index, question, false); err != nil {
				return err
			}
		}
		// Preserve every original short/long question, then exercise selection
		// through the same actual external provider and CLI process.
		for index, current := range []string{"healthy", outcome} {
			question := commandOutcomeQuestion{
				command: "show answer-outcome " + current + " long", outcome: current, count: 1,
			}
			if err := commandOutcomeCLICheck(ctx, binary, root, address, index+4, question, true); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "OK: CLI %s outcome retained rows, exit status and transcript across the record threshold\n", outcome)
		return nil
	})
}

// Readiness only means the fixture listener accepts TCP. The following healthy
// provider answer must independently prove authentication and command dispatch.
func commandOutcomeCLIReady(ctx context.Context, address string) error {
	var lastErr error
	if !Poll(ctx, 20, 100*time.Millisecond, func() bool {
		connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		if err != nil {
			lastErr = err
			return false
		}
		lastErr = connection.Close()
		return lastErr == nil
	}) {
		return fmt.Errorf("command outcome SSH listener did not become ready: %w", lastErr)
	}
	return nil
}

// Each of six fixed questions owns a distinct client home and data directory.
// Its one transcript cannot be supplied by a readiness probe or another case.
func commandOutcomeCLICheck(ctx context.Context, binary, root, address string, index int, question commandOutcomeQuestion, first bool) error {
	home := filepath.Join(root, fmt.Sprintf("client-%d", index))
	if err := os.Mkdir(home, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"config", "runtime", "store"} {
		if err := os.Mkdir(filepath.Join(home, name), 0o700); err != nil {
			return err
		}
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	dataHome := filepath.Join(home, "data")
	// Strip inherited plugin startup and daemon overrides before supplying this
	// CLI's private root and credentials. Literal loopback uses the product's
	// existing local-trust policy; this fixture enables no insecure override.
	// Alias handling follows the existing standalone trust fixture.
	environment := remoteTrustEnvironment(os.Environ(), map[string]string{
		envHome:             home,
		"XDG_CONFIG_HOME":   filepath.Join(home, "config"),
		"XDG_DATA_HOME":     dataHome,
		envXDGRuntimeDir:    filepath.Join(home, "runtime"),
		envConfigDir:        filepath.Join(home, "store"),
		envSSHHost:          addrLoopback,
		envSSHPort:          port,
		envSSHUsername:      valueTestUser,
		envSSHPassword:      valueTestPassword,
		"ze.ssh.insecure":   "false",
		"ze.cli.transcript": "true",
	})
	input := question.command + " | raw"
	if first {
		input = question.command + " | first 1 | ndjson"
	}
	result, err := commandOutcomeCLIRun(ctx, binary, home, input, environment)
	if err != nil {
		return fmt.Errorf("CLI %s: %w", input, err)
	}
	if first {
		if err := commandOutcomeCLIFirstVerdict(result, question); err != nil {
			return err
		}
	} else {
		if err := commandOutcomeCLIVerdict(result, question); err != nil {
			return err
		}
	}
	return commandOutcomeCLITranscript(dataHome, input, strings.TrimSuffix(result.stdout, "\n"))
}

// A nonzero exit alone could be connection or authentication failure. The
// independently decoded provider rows, fault causes and exact diagnostic must
// also survive before the child can establish the command outcome.
func commandOutcomeCLIVerdict(result fixture10ProcessResult, question commandOutcomeQuestion) error {
	input := question.command + " | raw"
	wantExit := 1
	if question.outcome == "healthy" {
		wantExit = 0
	}
	if question.outcome == "warning" {
		wantExit = 0
	}
	if result.code != wantExit {
		return fmt.Errorf("CLI %s: exit %d, want %d; stdout=%q stderr=%q", input, result.code, wantExit, result.stdout, result.stderr)
	}
	if err := commandOutcomeSurfaceDocument([]byte(result.stdout), question); err != nil {
		return fmt.Errorf("CLI %s: %w; stderr=%q", input, err, result.stderr)
	}
	if question.outcome == "healthy" {
		if result.stderr != "" {
			return fmt.Errorf("CLI %s: healthy answer has diagnostics %q", input, result.stderr)
		}
	} else if question.outcome == "warning" {
		if result.stderr != "Warning\n" {
			return fmt.Errorf("CLI %s: status-only warning diagnostic %q, want Warning", input, result.stderr)
		}
	} else {
		failure := "Process exited with status 1"
		if question.outcome == "failed" {
			failure = rpc.AnswerFailureUnstated
		}
		if !strings.Contains(result.stderr, "error: "+failure+"\n") {
			return fmt.Errorf("CLI %s: lost its completed failure %q: %q", input, failure, result.stderr)
		}
	}
	if !strings.HasSuffix(result.stdout, "\n") {
		return fmt.Errorf("CLI %s: completed output has no final newline", input)
	}
	return nil
}

// The selected body proves real dispatch; the exact exit and diagnostic prove
// that an unselected failure tail was consumed rather than inferred healthy.
func commandOutcomeCLIFirstVerdict(result fixture10ProcessResult, question commandOutcomeQuestion) error {
	input := question.command + " | first 1 | ndjson"
	wantExit := 1
	if question.outcome == "healthy" {
		wantExit = 0
	}
	if question.outcome == "warning" {
		wantExit = 0
	}
	if result.code != wantExit {
		return fmt.Errorf("CLI %s: selected exit %d, want %d; stdout=%q stderr=%q", input, result.code, wantExit, result.stdout, result.stderr)
	}
	var document struct {
		Items []struct {
			Ordinal *int `json:"ordinal"`
		} `json:"items"`
		Pipe []struct {
			Arg string `json:"arg"`
			Op  string `json:"op"`
		} `json:"pipe"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &document); err != nil {
		return fmt.Errorf("CLI %s: selected output is not one JSON document: %w", input, err)
	}
	if len(document.Items) != 1 {
		return fmt.Errorf("CLI %s: selected document has %d items, want 1", input, len(document.Items))
	}
	if document.Items[0].Ordinal == nil {
		return fmt.Errorf("CLI %s: selected row has no ordinal", input)
	}
	if *document.Items[0].Ordinal != 0 {
		return fmt.Errorf("CLI %s: selected ordinal %d, want 0", input, *document.Items[0].Ordinal)
	}
	if len(document.Errors) != 0 {
		return fmt.Errorf("CLI %s: selected document retained unselected refused rows", input)
	}
	if len(document.Pipe) != 1 {
		return fmt.Errorf("CLI %s: selected document has %d pipe entries, want 1", input, len(document.Pipe))
	}
	if document.Pipe[0].Op != "first" {
		return fmt.Errorf("CLI %s: selected pipe operation %q, want first", input, document.Pipe[0].Op)
	}
	if document.Pipe[0].Arg != "1" {
		return fmt.Errorf("CLI %s: selected pipe argument %q, want 1", input, document.Pipe[0].Arg)
	}
	if question.outcome == "healthy" {
		if result.stderr != "" {
			return fmt.Errorf("CLI %s: selected healthy answer has diagnostics %q", input, result.stderr)
		}
	} else if question.outcome == "warning" {
		if result.stderr != "Warning\n" {
			return fmt.Errorf("CLI %s: status-only warning diagnostic %q, want Warning", input, result.stderr)
		}
	} else {
		failure := "Process exited with status 1"
		if question.outcome == "failed" {
			failure = rpc.AnswerFailureUnstated
		}
		if !strings.Contains(result.stderr, "error: "+failure+"\n") {
			return fmt.Errorf("CLI %s: selected answer lost its completed failure %q: %q", input, failure, result.stderr)
		}
	}
	if !strings.HasSuffix(result.stdout, "\n") {
		return fmt.Errorf("CLI %s: selected output has no final newline", input)
	}
	return nil
}

// The scenario has six fixed commands with at most 259 item records each.
// The child deadline bounds a CLI that stops reading either SSH stream.
func commandOutcomeCLIRun(ctx context.Context, binary, cwd, input string, environment []string) (fixture10ProcessResult, error) {
	commandCtx, cancel := context.WithTimeout(ctx, deploymentBudget(10*time.Second))
	defer cancel()
	command := exec.CommandContext(commandCtx, binary, "cli", "-c", input) //nolint:gosec // Runner-owned executable and fixed fixture command.
	command.Dir, command.Env = cwd, environment
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if commandCtx.Err() != nil {
		return fixture10ProcessResult{}, fmt.Errorf("CLI did not finish: %w", commandCtx.Err())
	}
	code := 0
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return fixture10ProcessResult{}, err
		}
		code = exit.ExitCode()
	}
	return fixture10ProcessResult{code: code, stdout: stdout.String(), stderr: stderr.String()}, nil
}

// The timestamp is variable. The command and every observed body byte must be
// recorded exactly once, without recording the terminal-only final newline.
func commandOutcomeCLITranscript(dataHome, input, body string) error {
	directory := filepath.Join(dataHome, "ze", "transcripts")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("CLI %s: transcript directory: %w", input, err)
	}
	if len(entries) != 1 {
		return fmt.Errorf("CLI %s: %d transcript files, want one", input, len(entries))
	}
	content, err := os.ReadFile(filepath.Join(directory, entries[0].Name()))
	if err != nil {
		return err
	}
	_, record, found := strings.Cut(string(content), "] > ")
	if !found {
		return fmt.Errorf("CLI %s: transcript has no command record", input)
	}
	if record != input+"\n"+body+"\n\n" {
		return fmt.Errorf("CLI %s: transcript changed the command or completed output: %q", input, record)
	}
	return nil
}

// Design: docs/functional-tests.md -- compiled fixtures a .ci drives
// Related: plugin_fixture_04_cli.go -- driveEditor04, the SSH editor pattern this generalises
// Related: register_session_editor.go -- registers the driver

package fixture

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
)

// sessionEditorVerb is one step kind of a session editor script.
type sessionEditorVerb uint8

const (
	sessionEditorUnspecified sessionEditorVerb = iota
	// sessionEditorSend types its text into the editor and presses Enter.
	sessionEditorSend
	// sessionEditorWait reads the editor's output until its text appears.
	sessionEditorWait
	// sessionEditorCLI names the `ze cli -c` command the next has/lacks polls.
	sessionEditorCLI
	// sessionEditorHas polls the command until its output holds the text.
	sessionEditorHas
	// sessionEditorLacks polls the command until its output no longer holds the text.
	sessionEditorLacks
	// sessionEditorKey presses the named key from sessionEditorKeys, with no Enter.
	sessionEditorKey
	// sessionEditorKill kills the editor's SSH client, as a dropped connection
	// does. It takes no text, and no step after it may type into the editor.
	sessionEditorKill
	// sessionEditorStop stops the daemon with `ze signal stop`, so a .ci can
	// start a second daemon on the store this one leaves. It takes no text,
	// follows a kill (no editor is left to quit), and is the last step.
	sessionEditorStop
	// sessionEditorInput types its text into the editor with no Enter, so a
	// following key step (Tab) acts on a partial line.
	sessionEditorInput
)

// sessionEditorWordInput is the script word for sessionEditorInput. It is a
// constant because the package already spells "input" for a payload field
// (fieldInput), and the two mean different things.
const sessionEditorWordInput = "input"

// sessionEditorVerbs maps a script word to its verb. It is the grammar, so the
// parser refuses any word missing here.
var sessionEditorVerbs = map[string]sessionEditorVerb{
	"send":                 sessionEditorSend,
	"wait":                 sessionEditorWait,
	"cli":                  sessionEditorCLI,
	"has":                  sessionEditorHas,
	"lacks":                sessionEditorLacks,
	"key":                  sessionEditorKey,
	"kill":                 sessionEditorKill,
	"stop":                 sessionEditorStop,
	sessionEditorWordInput: sessionEditorInput,
}

// sessionEditorKeys maps a key name a key step accepts to the bytes the
// terminal sends for it. Ctrl-D ends the editor's paste mode; Tab completes
// the line and opens the completion box.
var sessionEditorKeys = map[string]string{
	"ctrl-d": "\x04",
	"tab":    "\t",
}

// sessionEditorStep is one parsed script line.
type sessionEditorStep struct {
	verb sessionEditorVerb
	text string
}

// sessionEditorStepsMax bounds a script, so a runaway file cannot hold the
// driver past its .ci timeout one step at a time.
const sessionEditorStepsMax = 200

// parseSessionEditorScript reads one step per line: a verb, a space, and its
// text. Blank lines and lines starting with # are skipped. A has or lacks
// before any cli line is refused, because it would poll no command.
func parseSessionEditorScript(r io.Reader) ([]sessionEditorStep, error) {
	var steps []sessionEditorStep
	haveCLI := false
	killed := false
	stopped := false
	scanner := bufio.NewScanner(r)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "#") {
			continue
		}
		word, text, _ := strings.Cut(raw, " ")
		verb, known := sessionEditorVerbs[word]
		if !known {
			return nil, fmt.Errorf("script line %d: unknown verb %q", line, word)
		}
		text = strings.TrimSpace(text)
		if stopped {
			return nil, fmt.Errorf("script line %d: %s after the daemon was stopped", line, word)
		}
		if err := sessionEditorCheckStep(verb, text, killed); err != nil {
			return nil, fmt.Errorf("script line %d: %s %w", line, word, err)
		}
		if verb == sessionEditorKill {
			killed = true
		}
		if verb == sessionEditorStop {
			stopped = true
		}
		if verb == sessionEditorCLI {
			haveCLI = true
		}
		if !haveCLI {
			if verb == sessionEditorHas {
				return nil, fmt.Errorf("script line %d: has before any cli line", line)
			}
			if verb == sessionEditorLacks {
				return nil, fmt.Errorf("script line %d: lacks before any cli line", line)
			}
		}
		if len(steps) == sessionEditorStepsMax {
			return nil, fmt.Errorf("script line %d: more than %d steps", line, sessionEditorStepsMax)
		}
		steps = append(steps, sessionEditorStep{verb: verb, text: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, errors.New("script has no steps")
	}
	return steps, nil
}

// sessionEditorCheckStep answers why one step cannot run, or nil. kill and
// stop take no text and every other verb needs one; a key names a known key;
// nothing types into an editor a kill already ended; stop needs that kill.
func sessionEditorCheckStep(verb sessionEditorVerb, text string, killed bool) error {
	if verb == sessionEditorStop {
		if text != "" {
			return errors.New("takes no text")
		}
		if !killed {
			return errors.New("before the editor was killed")
		}
		return nil
	}
	if verb == sessionEditorKill {
		if text != "" {
			return errors.New("takes no text")
		}
		if killed {
			return errors.New("after the editor was killed")
		}
		return nil
	}
	if text == "" {
		return errors.New("needs text")
	}
	if verb == sessionEditorKey {
		if _, known := sessionEditorKeys[text]; !known {
			return fmt.Errorf("names unknown key %q", text)
		}
	}
	if !killed {
		return nil
	}
	if verb == sessionEditorSend {
		return errors.New("after the editor was killed")
	}
	if verb == sessionEditorWait {
		return errors.New("after the editor was killed")
	}
	if verb == sessionEditorKey {
		return errors.New("after the editor was killed")
	}
	if verb == sessionEditorInput {
		return errors.New("after the editor was killed")
	}
	return nil
}

// sessionEditorPendingAfter answers the output after the first needle in buf,
// and whether the needle is there. The rest is what the next wait starts from.
func sessionEditorPendingAfter(buf []byte, needle string) ([]byte, bool) {
	index := bytes.Index(buf, []byte(needle))
	if index < 0 {
		return nil, false
	}
	return buf[index+len(needle):], true
}

// sessionEditorDriver drives `ze config edit <config>` over SSH against the
// running daemon through the steps of a script file, so each .ci of the session
// editor spec states its typing and its assertions on the running daemon in the
// .ci itself. Arguments: <ssh-port> <config> <script> [user <name>].
func sessionEditorDriver(ctx context.Context, args []string) error {
	user, err := sessionEditorUser(args)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(args[2])
	if err != nil {
		return err
	}
	steps, err := parseSessionEditorScript(bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("%s: %w", args[2], err)
	}
	env, err := sessionEditorClientEnv(ctx, args[0], user)
	if err != nil {
		return err
	}
	if err := sessionEditorRun(ctx, env, args[1], steps); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "OK: session editor script completed") //nolint:errcheck // fixture progress line
	return nil
}

// sessionEditorUser answers the SSH user the driver logs in as: admin for the
// three positional arguments, or the name a trailing `user <name>` gives, so a
// .ci can drive a second user (a LIVE conflict needs two). Any other shape is
// refused with the usage line.
func sessionEditorUser(args []string) (string, error) {
	const usage = "usage: session-editor <ssh-port> <config> <script> [user <name>]"
	if len(args) == 3 {
		return "admin", nil
	}
	if len(args) != 5 {
		return "", errors.New(usage)
	}
	if args[3] != "user" {
		return "", errors.New(usage)
	}
	if args[4] == "" {
		return "", errors.New(usage)
	}
	if filepath.Base(args[4]) != args[4] {
		return "", fmt.Errorf("session-editor: user %q is not a plain name", args[4])
	}
	return args[4], nil
}

// sessionEditorClientEnv writes user's SSH credentials with `ze init` into a
// client directory of its own, so two users in one .ci stay apart, reuses
// them when the same user reconnects, and waits until the daemon answers.
func sessionEditorClientEnv(ctx context.Context, port, user string) ([]string, error) {
	clientDir, err := filepath.Abs("client-db-" + user)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(clientDir, 0o750); err != nil {
		return nil, err
	}
	env := overrideEnv04(os.Environ(),
		"ZE_CONFIG_DIR="+clientDir,
		"ZE_SSH_PASSWORD=testpass",
		"ZE_SSH_HOST=127.0.0.1",
		"ZE_SSH_PORT="+port,
		"ZE_SSH_USERNAME="+user,
		"NO_COLOR=1",
		"TERM=xterm",
	)
	// A later script for the same user in one .ci reconnects over the
	// credentials the first one wrote, and `ze init` refuses a database that
	// already exists.
	_, statErr := os.Stat(filepath.Join(clientDir, "database"))
	if errors.Is(statErr, os.ErrNotExist) {
		initInput := user + "\ntestpass\n127.0.0.1\n" + port + "\n\n"
		if _, err := runCommandProcess04(ctx, env, strings.NewReader(initInput), "init"); err != nil {
			return nil, err
		}
	} else if statErr != nil {
		return nil, fmt.Errorf("client database for %s: %w", user, statErr)
	}
	var last string
	if !Poll(ctx, 50, 200*time.Millisecond, func() bool {
		output, err := runCommandProcess04(ctx, env, nil, "cli", "-c", "show version")
		last = output
		return err == nil
	}) {
		return nil, fmt.Errorf("daemon did not become reachable: %s", last)
	}
	return env, nil
}

// sessionEditorRun opens the editor over a PTY, runs the steps, and quits.
// A failure carries the editor transcript, so the .ci log shows the screen the
// step was looking at.
func sessionEditorRun(ctx context.Context, env []string, config string, steps []sessionEditorStep) error {
	cmd := exec.CommandContext(ctx, "ze", "config", "edit", config) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 160})
	if err != nil {
		return err
	}

	defer terminal.Close() //nolint:errcheck // fixture teardown
	defer func() {
		if cmd.ProcessState == nil {
			_, _ = killSessionEditor(cmd, terminal) //nolint:errcheck // fixture teardown after a failed step
		}
	}()

	var transcript bytes.Buffer
	pending, err := sessionEditorWaitFor(terminal, nil, &transcript, "\x1b[?1049h")
	if err != nil {
		return fmt.Errorf("config editor did not draw its first frame: %w\n%s", err, transcript.String())
	}
	command := ""
	killed := false
	for i, step := range steps {
		switch step.verb {
		case sessionEditorSend:
			_, err = terminal.WriteString(step.text + "\r")
		case sessionEditorWait:
			pending, err = sessionEditorWaitFor(terminal, pending, &transcript, step.text)
		case sessionEditorCLI:
			command = step.text
		case sessionEditorHas:
			err = sessionEditorPoll(ctx, env, command, step.text, true)
		case sessionEditorLacks:
			err = sessionEditorPoll(ctx, env, command, step.text, false)
		case sessionEditorKey:
			_, err = terminal.WriteString(sessionEditorKeys[step.text])
		case sessionEditorInput:
			_, err = terminal.WriteString(step.text)
		case sessionEditorKill:
			var tail string
			tail, err = killSessionEditor(cmd, terminal)
			transcript.WriteString(tail)
			killed = true
		case sessionEditorStop:
			_, err = runCommandProcess04(ctx, env, nil, "signal", "stop")
		case sessionEditorUnspecified:
			panic("BUG: parseSessionEditorScript produced an unspecified step")
		}
		if err != nil {
			return fmt.Errorf("step %d (%q): %w\n%s", i+1, step.text, err, transcript.String())
		}
	}
	if killed {
		return nil
	}
	if _, err := terminal.WriteString("quit\r"); err != nil {
		return fmt.Errorf("quit: %w\n%s", err, transcript.String())
	}
	tail, _ := readPTYUntil04(terminal, nil, true) //nolint:errcheck // the exit status below is the answer
	transcript.WriteString(tail)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("config editor exited: %w\n%s", err, transcript.String())
	}
	return nil
}

// sessionEditorWaitFor reads the PTY from pending until needle appears, records
// what it read in transcript, and answers the output after the needle.
func sessionEditorWaitFor(terminal *os.File, pending []byte, transcript *bytes.Buffer, needle string) ([]byte, error) {
	if rest, found := sessionEditorPendingAfter(pending, needle); found {
		return rest, nil
	}
	buf, err := readPTYUntil04(terminal, pending, false, needle)
	transcript.WriteString(buf[len(pending):])
	if err != nil {
		return nil, fmt.Errorf("waiting for %q: %w", needle, err)
	}
	rest, _ := sessionEditorPendingAfter([]byte(buf), needle)
	return rest, nil
}

// sessionEditorPoll runs `ze cli -c command` until its output holds needle
// (want true) or no longer holds it (want false). The daemon applies a commit
// after the editor reports it, so one read could see the previous state.
func sessionEditorPoll(ctx context.Context, env []string, command, needle string, want bool) error {
	var last string
	if Poll(ctx, 100, 100*time.Millisecond, func() bool {
		output, err := runCommandProcess04(ctx, env, nil, "cli", "-c", command)
		last = output
		if err != nil {
			return false
		}
		return strings.Contains(output, needle) == want
	}) {
		return nil
	}
	if want {
		return fmt.Errorf("%q never held %q; last output:\n%s", command, needle, last)
	}
	return fmt.Errorf("%q still held %q; last output:\n%s", command, needle, last)
}

// killSessionEditor kills the client, reads its terminal to the end, and reaps
// it. The read is what lets it exit: a script that only sends and polls the
// daemon leaves the client's output unread, and a process whose controlling
// terminal holds unread output waits on its exit for that output to drain, so
// a bare Kill then Wait never returns. It answers what the drain read.
func killSessionEditor(cmd *exec.Cmd, terminal *os.File) (string, error) {
	// A client that already exited still has to be drained and reaped, so a
	// failed Kill is answered only after both.
	killErr := cmd.Process.Kill()
	tail, _ := readPTYUntil04(terminal, nil, true) //nolint:errcheck // the drain ends at EOF or its deadline; Wait below is the answer
	_ = cmd.Wait()                                 //nolint:errcheck // a killed client exits with the signal, which is the point
	return tail, killErr
}

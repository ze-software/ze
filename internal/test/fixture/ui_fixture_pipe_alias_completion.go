// Design: docs/architecture/api/commands.md -- pipe aliases and where they resolve
// Related: plugin_fixture_11_alias.go -- the daemon, the plugin and the config this reuses
// Related: ui_fixture_display_fill_completion.go -- the ANSI stripper this shares

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
)

const (
	// pipeAliasReadTimeout bounds one wait for a word to appear on the screen.
	// The terminal delivers chunks and never ends, so a reader that will not
	// see its word has to stop on a clock.
	pipeAliasReadTimeout = 30 * time.Second
	// pipeAliasSettleTime is how long the screen is collected after Tab.
	// Nothing terminates a completion offer, so there is no marker to wait for.
	pipeAliasSettleTime = 3 * time.Second
)

func init() {
	registerFixture("ui/pipe-alias-completion", pipeAliasCompletion)
}

// pipeAliasCompletion drives the completion offer a plugin's declared pipe
// alias makes, on the one surface that can make it.
//
// The alias lives in the DAEMON's registry, and an interactive session's pipe
// chain is completed by whichever process runs the Bubble Tea model. `ze cli`
// runs that model in the CLIENT process, so a declared alias is unknown there.
// A plain ssh client with a pseudo-terminal reaches the model the DAEMON hosts
// (buildSessionModelFactory, cmd/ze/hub/session_factory.go), and that model's
// completer reads the registry Stage 1 wrote.
//
// The assertion is the whole of AC-10: after the pipe character the name the
// plugin declared is offered BESIDE the built-in operators, not in place of
// them.
func pipeAliasCompletion(ctx context.Context) error {
	daemon, cliEnv, err := startFixtureDaemon(ctx, aliasConfig(aliasCaseBasic))
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		code, out, _, runErr := cli11(ctx, cliEnv, cmdShowPipealiasCounters+" | json")
		return runErr == nil && code == 0 && strings.Contains(out, fieldVRPCount)
	}) {
		return errors.New("plugin command never answered")
	}

	port, err := pipeAliasSetting(cliEnv, "ZE_SSH_PORT")
	if err != nil {
		return err
	}
	user, err := pipeAliasSetting(cliEnv, "ZE_SSH_USERNAME")
	if err != nil {
		return err
	}
	password, err := pipeAliasSetting(cliEnv, "ZE_SSH_PASSWORD")
	if err != nil {
		return err
	}
	screen, err := pipeAliasTabOffer(ctx, login{port: port, user: user, password: password},
		cmdShowPipealiasCounters+" | ")
	if err != nil {
		return err
	}

	if !strings.Contains(screen, fieldTotals) {
		return fmt.Errorf("completion did not offer the declared alias %q:\n%s", fieldTotals, screen)
	}
	// A built-in operator has to survive beside it. An offer that REPLACED the
	// operators would satisfy the assertion above and break the surface.
	if !strings.Contains(screen, "json") {
		return fmt.Errorf("completion dropped the built-in operators:\n%s", screen)
	}

	fmt.Println("OK")
	return nil
}

// login is what an ssh client needs to reach the daemon this fixture started.
// The daemon takes an ephemeral port and startFixtureDaemon invents the
// account, so nothing outside that call knows any of the three.
type login struct {
	port     string
	user     string
	password string
}

// pipeAliasSetting answers one variable startFixtureDaemon wrote into the
// client environment.
//
// The LAST assignment wins, which is the rule os/exec applies to the same
// slice. startFixtureDaemon appends to the inherited environment, so a variable
// this process already carried is still in front of the one that names this
// daemon.
func pipeAliasSetting(env []string, name string) (string, error) {
	value := ""
	found := false
	for _, entry := range env {
		if setting, matched := strings.CutPrefix(entry, name+"="); matched {
			value = setting
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("the fixture environment carries no %s", name)
	}
	return value, nil
}

// pipeAliasTerminal is one ssh client under a pseudo-terminal, and the reader
// that drains it.
//
// The caller MUST call close after start, which stops the reader by closing the
// terminal and kills the client.
type pipeAliasTerminal struct {
	file       *os.File
	client     *exec.Cmd
	chunks     chan []byte
	transcript string
}

// pipeAliasStart logs a pseudo-terminal into the daemon's own session model.
// The sequence is the one a person performs: OpenSSH asks for the password and
// the hub greets, which lands on the operational prompt where a pipe character
// means anything at all.
func pipeAliasStart(ctx context.Context, account login) (*pipeAliasTerminal, error) {
	client := exec.CommandContext(ctx, "ssh", "-tt", "-p", account.port, //nolint:gosec // the fixture chooses the program and its arguments
		"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
		"-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no",
		"-o", "NumberOfPasswordPrompts=1", "-o", "ConnectTimeout=5", account.user+"@127.0.0.1")
	client.Env = append(os.Environ(), "TERM=xterm-256color")

	file, err := pty.StartWithSize(client, &pty.Winsize{Rows: 40, Cols: 160})
	if err != nil {
		return nil, fmt.Errorf("start the ssh client under a pseudo-terminal: %w", err)
	}
	terminal := &pipeAliasTerminal{file: file, client: client, chunks: make(chan []byte, 16)}
	go terminal.drain()
	return terminal, nil
}

// drain moves what the terminal writes onto the channel until the terminal is
// closed. It is the goroutine close stops.
func (t *pipeAliasTerminal) drain() {
	defer close(t.chunks)
	buffer := make([]byte, 65536)
	for {
		count, err := t.file.Read(buffer)
		if count != 0 {
			t.chunks <- append([]byte(nil), buffer[:count]...)
		}
		if err != nil {
			return
		}
	}
}

// close stops the reader and the client. It MUST be called after
// pipeAliasStart.
//
// The channel is drained after the terminal is closed. drain can be parked on a
// send that nobody is reading once the caller has its answer, and a closed
// terminal alone does not release it.
func (t *pipeAliasTerminal) close() {
	_ = t.file.Close()
	if t.client.Process != nil {
		_ = t.client.Process.Kill()
	}
	for range t.chunks { //nolint:revive // the loop exists for its side effect, which is releasing drain
	}
}

// readUntil collects the screen until wanted answers true, and reports what was
// read when it does not.
func (t *pipeAliasTerminal) readUntil(ctx context.Context, wanted func(string) bool, what string) error {
	timer := time.NewTimer(pipeAliasReadTimeout)
	defer timer.Stop()

	for !wanted(t.transcript) {
		select {
		case chunk, open := <-t.chunks:
			if !open {
				return fmt.Errorf("%s; transcript=%q", what, t.transcript)
			}
			t.transcript += string(chunk)
		case <-timer.C:
			return fmt.Errorf("%s; transcript=%q", what, t.transcript)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// settle collects whatever arrives over the settling window, and answers what
// the screen gained, with its escape sequences removed.
func (t *pipeAliasTerminal) settle(ctx context.Context, from int) (string, error) {
	timer := time.NewTimer(pipeAliasSettleTime)
	defer timer.Stop()

	for {
		select {
		case chunk, open := <-t.chunks:
			if !open {
				return displayFillANSI.ReplaceAllString(t.transcript[from:], ""), nil
			}
			t.transcript += string(chunk)
		case <-timer.C:
			return displayFillANSI.ReplaceAllString(t.transcript[from:], ""), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// pipeAliasTabOffer types text at the operational prompt, presses Tab, and
// answers the offer the session model drew.
func pipeAliasTabOffer(ctx context.Context, account login, text string) (string, error) {
	terminal, err := pipeAliasStart(ctx, account)
	if err != nil {
		return "", err
	}
	defer terminal.close()

	if err := terminal.readUntil(ctx, func(seen string) bool {
		return strings.Contains(strings.ToLower(seen), "password:")
	}, "OpenSSH did not ask for a password"); err != nil {
		return "", err
	}
	if _, err := terminal.file.WriteString(account.password + "\r"); err != nil {
		return "", fmt.Errorf("send the password: %w", err)
	}
	// The hub renders its greeting and the operational prompt in one frame, so
	// there is nothing to leave and nothing else to wait for. Sending `exit`
	// here would close the session rather than change mode.
	if err := terminal.readUntil(ctx, func(seen string) bool {
		return strings.Contains(seen, "ze> ")
	}, "the session model did not reach the operational prompt"); err != nil {
		return "", err
	}

	typed := len(terminal.transcript)
	if _, err := terminal.file.WriteString(text); err != nil {
		return "", fmt.Errorf("type the command: %w", err)
	}
	// The ASCII pipe character is the last thing typed and it is what has to
	// arrive. The whole line cannot be waited for: the model redraws the prompt
	// on every keystroke and the terminal delivers the echo in fragments
	// separated by cursor moves, so `show pipealias counters |` is never
	// contiguous, and the inline suggestion puts a ghost character inside it.
	// The box the hub draws uses U+2502, not the ASCII pipe, so it cannot match.
	if err := terminal.readUntil(ctx, func(seen string) bool {
		return strings.Contains(displayFillANSI.ReplaceAllString(seen[typed:], ""), "|")
	}, "the pipe character never reached the session model"); err != nil {
		return "", err
	}

	offered := len(terminal.transcript)
	if _, err := terminal.file.Write([]byte{'\t'}); err != nil {
		return "", fmt.Errorf("press Tab: %w", err)
	}
	return terminal.settle(ctx, offered)
}

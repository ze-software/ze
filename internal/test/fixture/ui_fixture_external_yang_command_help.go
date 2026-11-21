// Design: docs/plugin-development/schema.md -- an external plugin brings its own command YANG
// Overview: ui_fixture_external_yang_session.go -- the daemon config and the session prompts this reuses
// Related: ui_fixture_pipe_alias_completion.go -- sessionScreenAfterKeys, the ssh terminal and the screen read
// Related: plugin_fixture_external_yang.go -- the `commands` variant, its Stage 1 help texts
// Related: ../externalyang/externalyang.go -- the YANG texts the screen is judged against
//
// The fixtures behind external-yang-command-help.ci (AC-30) and
// external-yang-command-completion.ci (AC-31): a daemon runs the external-yang
// plugin in its `commands` variant, which brings a -cmd module and states its
// own, different, help texts at Stage 1, and a plain ssh client under a
// pseudo-terminal types into the session model the daemon hosts.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/test/externalyang"
)

func init() {
	registerFixture("ui/external-yang-command-help", externalYANGCommandHelp)
	registerFixture("ui/external-yang-command-completion", externalYANGCommandCompletion)
}

// externalYANGCommandConfig is the session daemon of
// ui_fixture_external_yang_session.go with the plugin bringing its commands.
var externalYANGCommandConfig = strings.Replace(externalYANGSessionConfig,
	"plugin/external-yang valid", "plugin/external-yang commands", 1)

// externalYANGCommandSession starts the daemon, waits until the plugin serves
// its command (Stage 5 registered it, so the Stage 1 texts exist in the
// registry beside the YANG ones), and answers the login and the ssh
// credentials. The caller MUST stop the daemon.
func externalYANGCommandSession(ctx context.Context, args []string) (*fixtureDaemon, login, sshclient.Credentials, error) {
	daemon, cliEnv, err := startFixtureDaemon(ctx, args, externalYANGCommandConfig)
	if err != nil {
		return nil, login{}, sshclient.Credentials{}, err
	}
	account, err := localCommandLogin(cliEnv)
	if err != nil {
		daemon.stop() //nolint:errcheck // fixture teardown on the failure path
		return nil, login{}, sshclient.Credentials{}, err
	}
	creds := sshclient.Credentials{Host: addrLoopback, Port: account.port, Username: account.user, Auth: account.password}
	served := Poll(ctx, WaitAttempts(50, 200*time.Millisecond, 100), 200*time.Millisecond, func() bool {
		answer, execErr := sshclient.ExecCommand(creds, externalyang.CommandStatus)
		return execErr == nil && strings.Contains(answer, externalyang.CommandStatus)
	})
	if !served {
		daemon.stop() //nolint:errcheck // fixture teardown on the failure path
		return nil, login{}, sshclient.Credentials{}, errors.New("the plugin never answered " + externalyang.CommandStatus)
	}
	return daemon, account, creds, nil
}

// externalYANGCommandHelp proves AC-30: the help a session of the running ze
// shows for an external plugin's commands is the help its YANG declares, and
// never the different texts the plugin stated at Stage 1.
//
// Four facts, each read from what the daemon drew or answered:
//   - after `show acme-monitor ` and Tab, the row under the menu holds the
//     ze:help summary of the first candidate, status (Model.warningText);
//   - a second Tab moves the selection to target, and the row holds its summary;
//   - `?` after Tab draws the description of status
//     (Model.revealCandidateExplanation);
//   - `system command help`, which the daemon answers from its command
//     registry, gives the ze:help summary and the description of status.
//
// The Stage 1 texts appear on none of the four.
func externalYANGCommandHelp(ctx context.Context, args []string) error {
	daemon, account, creds, err := externalYANGCommandSession(ctx, args)
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	command := func(text, echo, keys string) (string, error) {
		return sessionScreenAfterKeys(ctx, account, sshclient.CLIModeCommand, sessionCommandPrompt, text, echo, keys)
	}
	steps := []struct {
		what, keys, want, never string
	}{
		{"Tab on 'show acme-monitor '", "\t", externalyang.StatusSummary, externalyang.DeclaredSummary},
		{"Tab Tab on 'show acme-monitor '", "\t\t", externalyang.TargetSummary, externalyang.DeclaredSummary},
		{"Tab and ? on 'show acme-monitor '", "\t?", externalyang.StatusExplanation, externalyang.DeclaredExplanation},
	}
	for _, step := range steps {
		screen, screenErr := command("show acme-monitor ", "acme-monitor", step.keys)
		if screenErr != nil {
			return screenErr
		}
		if !strings.Contains(screen, step.want) {
			return fmt.Errorf("%s did not show the YANG text %q:\n%s", step.what, step.want, screen)
		}
		if strings.Contains(screen, step.never) {
			return fmt.Errorf("%s showed the Stage 1 text %q, where the YANG text wins:\n%s", step.what, step.never, screen)
		}
	}

	answer, err := sshclient.ExecCommand(creds, `system command help name "`+externalyang.CommandStatus+`"`)
	if err != nil {
		return fmt.Errorf("ssh exec of system command help: %w", err)
	}
	for _, want := range []string{externalyang.StatusSummary, externalyang.StatusExplanation} {
		if !strings.Contains(answer, want) {
			return fmt.Errorf("system command help of %s lacks the YANG text %q:\n%s", externalyang.CommandStatus, want, answer)
		}
	}
	for _, never := range []string{externalyang.DeclaredSummary, externalyang.DeclaredExplanation} {
		if strings.Contains(answer, never) {
			return fmt.Errorf("system command help of %s answers the Stage 1 text %q, where the YANG text wins:\n%s",
				externalyang.CommandStatus, never, answer)
		}
	}

	fmt.Println("OK: the session showed the YANG summaries of status and target and the description of status, and system command help answered the YANG texts")
	return nil
}

// externalYANGCommandCompletion proves AC-31: Tab in a session of the running
// ze offers an external plugin's commands and the argument leaf its YANG
// declares.
//
// Two facts, each read from the screen the daemon's model drew:
//   - after `show acme-monitor ` and Tab both commands, status and target, are
//     offered;
//   - after `show acme-monitor target ` and Tab the leaf name is offered. A
//     string leaf is offered by its name (TreeCompleter.matchChildren), and
//     the one candidate is applied to the line (Model.handleTab), as for a
//     compiled-in command: the leaf's name is on the screen either way, and
//     the typed text does not hold it.
func externalYANGCommandCompletion(ctx context.Context, args []string) error {
	daemon, account, _, err := externalYANGCommandSession(ctx, args)
	if err != nil {
		return err
	}
	defer daemon.stop() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	command := func(text, echo string) (string, error) {
		return sessionScreenAfterKeys(ctx, account, sshclient.CLIModeCommand, sessionCommandPrompt, text, echo, "\t")
	}
	commands, err := command("show acme-monitor ", "acme-monitor")
	if err != nil {
		return err
	}
	for _, want := range []string{lastCommandWord(externalyang.CommandStatus), lastCommandWord(externalyang.CommandTarget)} {
		if !strings.Contains(commands, want) {
			return fmt.Errorf("tab on 'show acme-monitor ' did not offer %q:\n%s", want, commands)
		}
	}

	leaf, err := command("show acme-monitor target ", leafTarget)
	if err != nil {
		return err
	}
	if !strings.Contains(leaf, "name") {
		return fmt.Errorf("tab on 'show acme-monitor target ' did not offer the argument leaf name:\n%s", leaf)
	}

	fmt.Println("OK: Tab offered status and target, and the argument leaf name of target")
	return nil
}

// lastCommandWord answers the word a command path ends in, the node Tab
// offers under its parent: "status" for externalyang.CommandStatus. It derives
// the word from the path the plugin's module declares, so the two cannot drift.
func lastCommandWord(path string) string {
	return path[strings.LastIndexByte(path, ' ')+1:]
}

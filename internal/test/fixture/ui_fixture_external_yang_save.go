// Design: docs/functional-tests.md — a compiled fixture that drives the daemon's session editor
// Related: ui_fixture_external_yang_session.go — the config, the prompts and the editor facts this fixture builds on
// Related: plugin_fixture_11_alias.go — startFixtureDaemon, launchFixtureDaemon and fixtureDaemon.halt
// Related: plugin_fixture_external_yang.go — the plugin whose FIXTURE lines this fixture reads

package fixture

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	registerFixture("ui/external-yang-session-save", externalYANGSessionSave)
}

// The lines the external-yang fixture plugin writes on the daemon's stderr
// (plugin_fixture_external_yang.go), for the section this fixture commits.
const (
	externalYANGApplyLine      = "FIXTURE: external-yang apply root=acme-monitor"
	externalYANGSavedLeaf      = "acme-monitor/interval=7"
	externalYANGSavedConfigure = "FIXTURE: external-yang configure root=acme-monitor leaves=[acme-monitor/interval=7 acme-monitor/target=probe-a]"
	externalYANGRunningLeaf    = `"interval": "7"`
)

// The line that commits the candidate in the daemon's editor, and its last
// word, which the screen shows once the line was typed: the verb
// command.VerbCommit and the subcommand `now`, the keyword of the commit that
// applies at once (the commitSubcommands table of internal/component/cli/contract).
const (
	externalYANGCommitEcho = "now"
	externalYANGCommitLine = command.VerbCommit + " " + externalYANGCommitEcho
)

// externalYANGSessionSave proves a section under the root of an external
// plugin survives a commit and a restart (AC-13, save).
//
// Four facts, in the order an operator meets them:
//   - `set acme-monitor interval 7` then `commit now`, each typed and run as
//     a line of its own in one session of the configuration editor the daemon
//     hosts, is taken;
//   - `show config running` answers the new value, which is the state the
//     fixture waits on before it stops the daemon;
//   - the plugin was handed the change: its apply line holds the new value;
//   - a second ze started on the same directory, with the same ze.conf
//     argument, hands the plugin the new value at Stage 2, so what the commit
//     saved holds the section and parses again.
func externalYANGSessionSave(ctx context.Context, args []string) error {
	bgpPort, err := parsePort(args, 0)
	if err != nil {
		return err
	}
	daemon, cliEnv, err := startFixtureDaemon(ctx, args, externalYANGSessionConfig)
	if err != nil {
		return err
	}
	// The directory is removed by the stop of whichever daemon runs last.
	last := daemon
	defer func() { _ = last.stop() }() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	account, err := localCommandLogin(cliEnv)
	if err != nil {
		return err
	}
	// One session: the candidate a set makes belongs to the session that
	// typed it. Each line is typed as an operator types it and as the session
	// fixtures that pass do: the line, then its last word seen on the screen,
	// then the "\r" that runs it. The editor takes `commit now`; a bare
	// `commit` is refused for its missing subcommand (contract.ParseCommit).
	//
	// The session stays open until the running config holds the value: the
	// commit runs in the session's own dispatch turn, and closing the terminal
	// ends the session.
	terminal, err := sessionLogin(ctx, account, sshclient.CLIModeEdit, sessionEditPrompt)
	if err != nil {
		return err
	}
	defer terminal.close()
	afterSet, err := terminal.typeAndPress(ctx, "set acme-monitor interval 7", "7", "\r")
	if err != nil {
		return err
	}
	if strings.Contains(afterSet, externalYANGRangeProblem) {
		var problem textbuf.Buffer
		problem.Str("the daemon's editor refused set acme-monitor interval 7:\n").Str(afterSet)
		return errors.New(problem.String())
	}
	afterCommit, err := terminal.typeAndPress(ctx, externalYANGCommitLine, externalYANGCommitEcho, "\r")
	if err != nil {
		return err
	}

	running, err := externalYANGRunningHolds(ctx, cliEnv, externalYANGRunningLeaf)
	if err != nil {
		var problem textbuf.Buffer
		problem.Str("after set and ").Str(externalYANGCommitLine).
			Str(" in the daemon's editor, show config running never held ").
			Str(externalYANGRunningLeaf).Str(": ").Err(err).Str("\nafter the set the editor drew:\n").Str(afterSet).
			Str("\nafter the commit the editor drew:\n").Str(afterCommit).
			Str("\nshow config running answered:\n").Str(running)
		return errors.New(problem.String())
	}

	if err := daemon.halt(); err != nil {
		return err
	}
	applied := daemon.stderr.String()
	for _, want := range []string{externalYANGApplyLine, externalYANGSavedLeaf} {
		if !strings.Contains(applied, want) {
			var problem textbuf.Buffer
			problem.Str("the plugin was not handed the committed change: the daemon's stderr holds no ").
				Str(want).Str(":\n").Str(applied)
			return errors.New(problem.String())
		}
	}

	second, _, err := launchFixtureDaemon(ctx, daemon.workdir, bgpPort)
	if err != nil {
		var problem textbuf.Buffer
		problem.Str("ze did not start again on the directory the commit saved into: ").Err(err)
		return errors.New(problem.String())
	}
	last = second
	if err := second.halt(); err != nil {
		return err
	}
	restarted := second.stderr.String()
	if !strings.Contains(restarted, externalYANGSavedConfigure) {
		var problem textbuf.Buffer
		problem.Str("after the restart the plugin was not configured with the committed value: the daemon's stderr holds no ").
			Str(externalYANGSavedConfigure).Str(":\n").Str(restarted)
		return errors.New(problem.String())
	}

	var done textbuf.Buffer
	done.Str("OK: interval 7 was committed in the daemon's editor, applied by the plugin, answered by show config running, and configured again after a restart\n")
	return done.StdOut()
}

// externalYANGRunningHolds waits until `show config running | json`, asked of
// the daemon, holds want, and answers the last text it read. The error says
// the wait ended without it.
func externalYANGRunningHolds(ctx context.Context, cliEnv []string, want string) (string, error) {
	last := ""
	for range WaitAttempts(75, daemonReadyDelay, daemonReadyFallback) {
		code, out, stderr, err := cli11(ctx, cliEnv, "show config running | json")
		if err == nil && code == 0 {
			if strings.Contains(out, want) {
				return out, nil
			}
		}
		last = out + stderr
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(daemonReadyDelay):
		}
	}
	return last, errors.New("the wait for the running config ended")
}

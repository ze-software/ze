// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Overview: fixture.go — fixture registry and observer helpers
// Related: register_drain.go — the drain responder fixtures these build on
// Related: ../plugins/fakedrain/drain/drain.go — the drain responder both modes share

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/test/plugins/fakedrain/drain"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/drain-signaler", drainSignaler)
	Register("plugin/drain-show", drainShow)
	Register("plugin/drain-exits", drainExits)
	Register("plugin/drain-group", drainGroup)
	Register("plugin/drain-ssh-restart", drainSSHRestart)
}

// drainShowBound is the longest a show command may take during a drain
// (AC-23): the SDK runs the drain handler off the event loop, so the loop
// keeps answering.
const drainShowBound = time.Second

// daemonPID reads the pid the .ci runner publishes for the daemon, the same
// file `le test peer` reads for its sigterm action.
func daemonPID() (int, error) {
	raw, err := os.ReadFile("daemon.pid")
	if err != nil {
		return 0, fmt.Errorf("read daemon.pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("parse daemon.pid: %w", err)
	}
	return pid, nil
}

// signalDaemon sends SIGTERM to the daemon and logs that it did, or logs the
// failure as a FAIL line the .ci rejects.
func signalDaemon() {
	pid, err := daemonPID()
	if err == nil {
		err = syscall.Kill(pid, syscall.SIGTERM)
	}
	if err != nil {
		drain.Log("drain-fixture: FAIL could not signal the daemon: " + err.Error())
		return
	}
	drain.Log("drain-fixture: sent SIGTERM during the drain")
}

// drainSignaler sends the daemon SIGTERM from inside its drain handler and
// never answers, so only the signal can end the round before its deadline.
// After a SIGTERM stop that is the second signal (AC-6 row 1); with the
// argument "request" the fixture starts the stop itself with `request
// shutdown`, and the signal is the first OS signal (AC-6 row 2). Either way
// the daemon MUST abort the drain, still tear down, and exit 0.
func drainSignaler(ctx context.Context, args []string) error {
	request := false
	for _, arg := range args {
		if arg != "request" {
			return errors.New("plugin/drain-signaler takes request, not " + arg)
		}
		request = true
	}
	plugin, err := newObserver("drain-signaler")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	plugin.OnDrain(func(drainCtx context.Context, input rpc.DrainInput) error {
		drain.Log("drain-fixture: drain received kind=" + input.Kind.String() + ", signaling")
		signalDaemon()
		<-drainCtx.Done()
		drain.Log("drain-fixture: drain ended unanswered")
		return drainCtx.Err()
	})
	if request {
		plugin.OnAllPluginsReady(func() error {
			go requestStopAfterSettle(plugin, "request shutdown")
			return nil
		})
	}
	return plugin.Run(ctx, sdk.Registration{DrainBudget: drain.Budget})
}

// drainShowCommand is the drain-show plugin's own command. Only this plugin's
// event loop answers it, so it is the call a drain handler running on that
// loop would leave unanswered.
const drainShowCommand = "show drain-show status"

// drainShow proves management and the event loop keep serving while its
// drain is held (AC-23). Its argument is the daemon's SSH port. Once SSH
// answers, it stops the daemon with SIGTERM itself, so the drain cannot start
// before the SSH server is up. From inside its drain handler it sends three
// commands, each of which MUST be answered within drainShowBound: `show bgp`,
// the BGP summary, over SSH as an operator types it; `show bgp` again through
// the plugin's own dispatch (`show bgp summary` was retired and answers
// "unknown command", plugin_fixture_15_show.go); and drainShowCommand, which
// the engine routes back to this plugin's event loop. A stop that shut or
// blocked the SSH dispatcher before the drain fails the first. An SDK that ran
// the handler on the loop answers the third only after the drain, so it fails.
func drainShow(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/drain-show <ssh-port>")
	}
	creds := sshclient.Credentials{Host: addrLoopback, Port: args[0], Username: valueTestUser, Auth: valueTestPassword}
	plugin, err := newObserver("drain-show")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	plugin.OnExecuteCommand(func(_ string, command string, _ []string, _ string) (string, any, error) {
		if command != drainShowCommand {
			return statusError, nil, fmt.Errorf("unknown command %q", command)
		}
		return statusDone, map[string]any{"draining": true}, nil
	})
	answered := drain.ResponderWith(plugin, func() {
		if drainShowAnswersSSH(creds, "show bgp") {
			drain.Log("drain-fixture: show bgp over SSH answered during the drain")
		}
		if drainShowAnswers(plugin, "show bgp") {
			drain.Log("drain-fixture: show bgp answered during the drain")
		}
		if drainShowAnswers(plugin, drainShowCommand) {
			drain.Log("drain-fixture: plugin command answered during the drain")
		}
	})
	plugin.OnAllPluginsReady(func() error {
		go signalDaemonAfterSSH(ctx, creds)
		return nil
	})
	registration := sdk.Registration{
		DrainBudget: drain.Budget,
		Commands:    []sdk.CommandDecl{{Name: drainShowCommand}},
	}
	return drainAnsweredOrFail(answered, plugin.Run(ctx, registration))
}

// drainShowAnswers dispatches one command during the drain and answers true
// when it came back done within drainShowBound. Every other outcome is logged
// as a FAIL line, which the .ci rejects.
func drainShowAnswers(plugin *sdk.Plugin, command string) bool {
	showCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	status, _, showErr := plugin.DispatchCommand(showCtx, command)
	elapsed := time.Since(start)
	if showErr != nil {
		drain.Log("drain-fixture: FAIL " + command + " during the drain: " + showErr.Error())
		return false
	}
	if status != statusDone {
		drain.Log("drain-fixture: FAIL " + command + " during the drain status=" + status)
		return false
	}
	if elapsed > drainShowBound {
		drain.Log("drain-fixture: FAIL " + command + " took " + elapsed.String() + " during the drain")
		return false
	}
	return true
}

// sshAnswer is one SSH exec's result, handed from execSSHAnswer to its waiter.
type sshAnswer struct {
	output string
	err    error
}

// drainShowAnswersSSH sends one command over SSH during the drain and answers
// true when it succeeded within drainShowBound. The exec runs on its own
// goroutine so a dispatcher that blocks for the whole drain is reported at the
// bound, with a FAIL line the .ci rejects, rather than holding the handler.
func drainShowAnswersSSH(creds sshclient.Credentials, command string) bool {
	result := make(chan sshAnswer, 1)
	start := time.Now()
	go execSSHAnswer(creds, command, result)
	select {
	case answer := <-result:
		if answer.err != nil {
			drain.Log("drain-fixture: FAIL " + command + " over SSH during the drain: " + answer.err.Error())
			return false
		}
		if strings.TrimSpace(answer.output) == "" {
			drain.Log("drain-fixture: FAIL " + command + " over SSH during the drain answered nothing")
			return false
		}
		return true
	case <-time.After(drainShowBound):
		drain.Log("drain-fixture: FAIL " + command + " over SSH not answered within " +
			drainShowBound.String() + " during the drain (waited " +
			time.Since(start).Round(time.Millisecond).String() + ")")
		return false
	}
}

// execSSHAnswer runs one SSH exec and sends its result. The channel has room
// for the one result, so the send never blocks after the waiter gave up; the
// goroutine ends when the exec does.
func execSSHAnswer(creds sshclient.Credentials, command string, result chan<- sshAnswer) {
	output, err := sshclient.ExecCommand(creds, command)
	result <- sshAnswer{output: output, err: err}
}

// signalDaemonAfterSSH waits until the daemon's SSH server answers and the
// .ci peer's session has had drainRequesterSettle to come up, then sends the
// daemon SIGTERM once. One goroutine for the fixture's one stop.
func signalDaemonAfterSSH(ctx context.Context, creds sshclient.Credentials) {
	if _, err := waitSSHCommandExtra3(ctx, creds, "show version"); err != nil {
		drain.Log("drain-fixture: FAIL SSH never answered before the stop: " + err.Error())
		return
	}
	time.Sleep(drainRequesterSettle)
	pid, err := daemonPID()
	if err == nil {
		err = syscall.Kill(pid, syscall.SIGTERM)
	}
	if err != nil {
		drain.Log("drain-fixture: FAIL could not signal the daemon: " + err.Error())
		return
	}
	drain.Log("drain-fixture: SSH answered, sent SIGTERM")
}

// drainExitsMarker counts drain-exits generations in the test directory.
const drainExitsMarker = "drain-exits.generation"

// errDrainExitsLeft is how drain-exits ends its own connection mid-drain.
var errDrainExitsLeft = errors.New("drain-exits left during the drain")

// drainExits declares failure-policy restart and ends its connection the
// moment its drain arrives, which ends the process. The daemon MUST refuse the
// respawn it would otherwise make, so a second generation is the failure
// (AC-17). The .ci holds the round open with a second, answering plugin, so
// the refusal happens inside the drain and not after teardown began.
func drainExits(ctx context.Context, _ []string) error {
	previous, err := os.ReadFile(drainExitsMarker)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", drainExitsMarker, err)
	}
	generation := len(previous) + 1
	if err := os.WriteFile(drainExitsMarker, append(previous, 'x'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", drainExitsMarker, err)
	}
	if generation > 1 {
		drain.Log("drain-fixture: FAIL drain-exits generation " + strconv.Itoa(generation) + " started")
	}
	plugin, err := newObserver("drain-exits")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // the drain handler closed it already; a second close changes nothing

	plugin.OnDrain(func(context.Context, rpc.DrainInput) error {
		drain.Log("drain-fixture: drain-exits leaving during the drain")
		plugin.Close() //nolint:errcheck,gosec // closing is the exit under test; its error changes no assertion
		return errDrainExitsLeft
	})
	runErr := plugin.Run(ctx, sdk.Registration{DrainBudget: drain.Budget, FailurePolicy: sdk.FailureRestart})
	if errors.Is(runErr, errDrainExitsLeft) {
		return nil
	}
	return runErr
}

// drainGroup proves an external drain plugin runs in a process group of its
// own (AC-16): it checks its group differs from ze's, sends SIGTERM to ze's
// whole group as a terminal or gokrazy does, and then answers the drain that
// signal starts. A plugin inside ze's group would be killed by that SIGTERM
// and never answer.
func drainGroup(ctx context.Context, _ []string) error {
	plugin, err := newObserver("drain-group")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	answered := drain.Responder(plugin)
	plugin.OnAllPluginsReady(func() error {
		go signalDaemonGroupAfterSettle()
		return nil
	})
	return drainAnsweredOrFail(answered, plugin.Run(ctx, sdk.Registration{DrainBudget: drain.Budget}))
}

// signalDaemonGroupAfterSettle waits drainRequesterSettle, then signals ze's
// process group once. One goroutine for the fixture's one stop.
func signalDaemonGroupAfterSettle() {
	time.Sleep(drainRequesterSettle)
	pid, err := daemonPID()
	if err != nil {
		drain.Log("drain-fixture: FAIL " + err.Error())
		return
	}
	group, err := syscall.Getpgid(pid)
	if err != nil {
		drain.Log("drain-fixture: FAIL read ze's process group: " + err.Error())
		return
	}
	if own := syscall.Getpgrp(); own == group {
		drain.Log("drain-fixture: FAIL plugin shares ze's process group " + strconv.Itoa(group))
		return
	}
	drain.Log("drain-fixture: plugin process group differs from ze's")
	if err := syscall.Kill(-group, syscall.SIGTERM); err != nil {
		drain.Log("drain-fixture: FAIL signal ze's process group: " + err.Error())
		return
	}
	drain.Log("drain-fixture: sent SIGTERM to ze's process group")
}

// drainSSHRestart is the SSH exec client that restarts the daemon: it waits
// for the SSH server, then sends `restart` once (AC-7). Its argument is the
// SSH port; the credentials are the ones the .ci config declares.
func drainSSHRestart(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: plugin/drain-ssh-restart <ssh-port>")
	}
	creds := sshclient.Credentials{Host: addrLoopback, Port: args[0], Username: valueTestUser, Auth: valueTestPassword}
	if _, err := waitSSHCommandExtra3(ctx, creds, "show version"); err != nil {
		drain.Log("drain-fixture: FAIL SSH never answered, so restart was not sent: " + err.Error())
		return err
	}
	time.Sleep(drainRequesterSettle)
	output, err := sshclient.ExecCommand(creds, "restart")
	drain.Log("drain-fixture: sent restart output=" + strings.TrimSpace(output) + " error=" + errorText(err))
	if err != nil {
		drain.Log("drain-fixture: FAIL SSH restart: " + err.Error())
	}
	return err
}

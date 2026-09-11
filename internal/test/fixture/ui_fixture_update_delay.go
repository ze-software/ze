// Design: docs/architecture/behavior/peer-lifecycle.md -- the startup convergence hold
// Related: internal/component/bgp/plugins/cmd/peer/update_delay.go -- the handler this drives

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/bgp-update-delay", uiDriver(uiUpdateDelay))
}

// uiUpdateDelayConfig is a speaker that holds. TWO peers are configured and
// neither address answers, so convergence is unreachable and the 300-second
// max-delay is far longer than this fixture lives: the daemon is still holding
// when the command runs, which is the state the command exists to report.
const uiUpdateDelayConfig = `bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
    update-delay {
        max-delay 300
    }
    peer peer1 {
        connection {
            remote { ip 192.0.2.1 }
            local  { ip 127.0.0.1 }
        }
        session { asn { remote 65001 } }
    }
    peer peer2 {
        connection {
            remote { ip 192.0.2.2 }
            local  { ip 127.0.0.1 }
        }
        session { asn { remote 65002 } }
    }
}

system {
    authentication {
        user ci {
            password "%s"
            profile [ admin ]
        }
    }
}
`

// uiUpdateDelay drives `show bgp update-delay` through the SSH CLI, which is the
// path an operator's keystrokes take.
//
// Reaching the handler is the whole point. The handler had unit tests and worked;
// the command was unreachable, because its node sat in a module whose name ends
// -api and BuildCommandTree walks -cmd modules alone. Both dispatchers answered
// nothing and said nothing. Only a test that types the command sees that.
func uiUpdateDelay(ctx context.Context) error {
	passwordHash, err := uiUpdateDelayPassword(ctx)
	if err != nil {
		return err
	}

	work, err := os.MkdirTemp("", "ze-ui-update-delay-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	configPath := filepath.Join(work, "update-delay.conf")
	sshAddressPath := filepath.Join(work, "ssh.addr")
	readyPath := filepath.Join(work, "ready")
	if err := os.WriteFile(configPath,
		[]byte(fmt.Sprintf(uiUpdateDelayConfig, passwordHash)), 0o600); err != nil {
		return fmt.Errorf("write update-delay.conf: %w", err)
	}

	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := uiAliasPeersEnv(os.Environ(), map[string]string{
		envSSHEphemeral: sshAddressPath,
		envReadyFile:    readyPath,
		envConfigDir:    work,
		// Leave port 179 alone: this suite runs unprivileged and a bind failure
		// there takes the daemon down before it writes `ready`.
		envTestBGPPort: strconv.Itoa(bgpPort),
	})

	daemon := exec.CommandContext(ctx, "ze", "-f", configPath) //nolint:gosec // the fixture chooses the program and its arguments
	daemon.Dir = work
	daemon.Env = daemonEnv
	daemon.Stdout = os.Stderr
	daemon.Stderr = os.Stderr
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer uiUpdateDelayStop(daemon) //nolint:errcheck // best-effort teardown, the assertion below is the verdict

	cliEnv, err := uiUpdateDelayWaitReady(ctx, work, sshAddressPath, readyPath)
	if err != nil {
		return err
	}

	asJSON, err := uiUpdateDelayCLI(ctx, cliEnv, "show bgp update-delay | json")
	if err != nil {
		return err
	}

	// The fields an operator reads to tell a holding daemon from a wedged one.
	for _, want := range []string{
		`"configured": true`,
		`"holding": true`,
		`"released": false`,
		`"reason": "not-released"`,
		`"expected-peers": 2`,
		`"max-delay-seconds": 300`,
	} {
		if !strings.Contains(asJSON, want) {
			return fmt.Errorf("`show bgp update-delay | json` does not carry %s: %q", want, asJSON)
		}
	}

	// The same answer through the default renderer, so the command is not
	// json-only. `| text` is what an operator sees with no pipe typed.
	text, err := uiUpdateDelayCLI(ctx, cliEnv, "show bgp update-delay | text")
	if err != nil {
		return err
	}
	if !strings.Contains(text, "holding") {
		return fmt.Errorf("`show bgp update-delay | text` does not name the hold: %q", text)
	}

	if _, err := fmt.Fprintln(os.Stdout, "OK"); err != nil {
		return fmt.Errorf("write success marker: %w", err)
	}
	return nil
}

func uiUpdateDelayPassword(ctx context.Context) (string, error) {
	passwd := exec.CommandContext(ctx, "ze", "passwd")
	passwd.Stdin = strings.NewReader("secret\n")
	out, err := passwd.Output()
	if err != nil {
		return "", fmt.Errorf("ze passwd: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// uiUpdateDelayWaitReady blocks until the daemon has written both markers, then
// returns the environment `ze cli` needs to reach its SSH listener.
func uiUpdateDelayWaitReady(ctx context.Context, work, sshAddressPath, readyPath string) ([]string, error) {
	for range 200 {
		if uiAliasPeersExists(sshAddressPath) && uiAliasPeersExists(readyPath) {
			addressBytes, err := os.ReadFile(sshAddressPath) //nolint:gosec // the path is the fixture's own scratch file
			if err != nil {
				return nil, fmt.Errorf("read ssh.addr: %w", err)
			}
			address := strings.TrimSpace(string(addressBytes))
			colon := strings.LastIndexByte(address, ':')
			if colon < 0 {
				return nil, fmt.Errorf("invalid SSH address %q", address)
			}
			return uiAliasPeersEnv(os.Environ(), map[string]string{
				envSSHHost:     address[:colon],
				envSSHPort:     address[colon+1:],
				envSSHUsername: "ci",
				envSSHPassword: valueSecret,
				envConfigDir:   work,
			}), nil
		}

		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("daemon did not become ready")
}

// uiUpdateDelayCLI runs one command and REFUSES a non-zero exit.
//
// A command the CLI cannot resolve exits 1, which is what makes the refusal
// worth having. Observed 2026-09-08 by deleting the node from ze-peer-cmd.yang
// and rebuilding: `show bgp update-delay | json exit=1: error: "show bgp
// update-delay" names no subcommand and no address family: unknown command`. The
// tokens fall back to the `show bgp` parent, which refuses the leftover word.
//
// This is the CLI's exit code, not the test runner's. The runner answers 0 for a
// suite verb it does not know (plan/journal/silent-fall-through.md), so the two
// must not be reasoned about together.
func uiUpdateDelayCLI(ctx context.Context, env []string, command string) (string, error) {
	result, err := uiAliasPeersRun(ctx, []string{"ze", areaCLI, "-c", command}, env)
	if err != nil {
		return "", fmt.Errorf("run %q: %w", command, err)
	}
	if result.code != 0 {
		return "", fmt.Errorf("%s exit=%d: %s%s", command, result.code, result.stdout, result.stderr)
	}
	return result.stdout, nil
}

func uiUpdateDelayStop(daemon *exec.Cmd) error {
	if daemon.Process == nil {
		return nil
	}
	if err := daemon.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate daemon: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- daemon.Wait() }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
	}
	if err := daemon.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill daemon: %w", err)
	}
	<-done
	return nil
}

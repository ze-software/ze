package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/alias-summary", uiDriver(aliasSummaryFixture))
}

type aliasSummaryDaemon struct {
	cmd     *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	done    chan error
	waited  bool
	waitErr error
}

func aliasSummaryFixture(ctx context.Context) error {
	passwordHash, err := aliasSummaryPasswordHash(ctx)
	if err != nil {
		return err
	}

	config := `bgp {
    router-id 10.255.255.254
    session {
        asn {
            local 65000
        }
    }
    group transit {
        peer peer1 {
            connection {
                remote {
                    ip 192.0.2.1
                }
                local {
                    ip 127.0.0.1
                }
            }
            session {
                asn {
                    remote 65001
                }
            }
        }
        peer peer2 {
            connection {
                remote {
                    ip 192.0.2.2
                }
                local {
                    ip 127.0.0.1
                }
            }
            session {
                asn {
                    remote 65002
                }
            }
        }
    }
}

system {
    authentication {
        user ci {
            password "` + passwordHash + `"
            profile [ admin ]
        }
    }
}
`
	work, err := os.MkdirTemp("", "ze-ui-alias-summary-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	configPath := filepath.Join(work, "summary.conf")
	sshAddrPath := filepath.Join(work, "ssh.addr")
	readyPath := filepath.Join(work, "ready")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write summary.conf: %w", err)
	}

	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := aliasSummaryEnvironment(os.Environ(),
		"ZE_SSH_EPHEMERAL="+sshAddrPath,
		"ZE_READY_FILE="+readyPath,
		"ZE_CONFIG_DIR="+work,
		// Leave port 179 alone: the suite runs unprivileged, and a bind
		// failure there can stop the daemon before it writes ready.
		fmt.Sprintf("ze_test_bgp_port=%d", bgpPort),
	)

	daemon := &aliasSummaryDaemon{done: make(chan error, 1)}
	daemon.cmd = exec.CommandContext(ctx, "ze", "-f", configPath) //nolint:gosec // the fixture chooses the program and its arguments
	daemon.cmd.Dir = work
	daemon.cmd.Env = daemonEnv
	daemon.cmd.Stdout = &daemon.stdout
	daemon.cmd.Stderr = &daemon.stderr
	if err := daemon.cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	go func() {
		daemon.done <- daemon.cmd.Wait()
	}()

	testErr := aliasSummaryRunAssertions(ctx, daemon, sshAddrPath, readyPath, work)
	cleanupErr := daemon.stop()
	if cleanupErr != nil {
		return cleanupErr
	}
	if testErr != nil {
		return testErr
	}

	fmt.Println("OK")
	return nil
}

func aliasSummaryPasswordHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Stdin = strings.NewReader("secret\n")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return "", fmt.Errorf("ze passwd exit=%d: %s%s", exitErr.ExitCode(), out, exitErr.Stderr)
	}
	return "", fmt.Errorf("run ze passwd: %w", err)
}

func aliasSummaryRunAssertions(ctx context.Context, daemon *aliasSummaryDaemon, sshAddrPath, readyPath, cwd string) error {
	if err := aliasSummaryWaitReady(ctx, daemon, sshAddrPath, readyPath, 200, 100*time.Millisecond); err != nil {
		return err
	}

	addrBytes, err := os.ReadFile(sshAddrPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]

	cliEnv := aliasSummaryEnvironment(os.Environ(),
		"ZE_SSH_HOST="+host,
		"ZE_SSH_PORT="+port,
		"ZE_SSH_USERNAME=ci",
		"ZE_SSH_PASSWORD=secret",
		"ZE_CONFIG_DIR="+cwd,
	)
	cli := func(command string) (string, error) {
		code, out, stderr, err := aliasSummaryRunCommand(ctx, cliEnv, "ze", "cli", "-c", command)
		if err != nil {
			return "", fmt.Errorf("run %s: %w", command, err)
		}
		if code != 0 {
			return "", fmt.Errorf("%s exit=%d: %s%s", command, code, out, stderr)
		}
		return out, nil
	}

	// Fetch the whole answer first so the later assertions measure a change,
	// rather than an answer that was always narrow.
	whole, err := cli("show bgp | text")
	if err != nil {
		return err
	}
	if !strings.Contains(whole, "192.0.2.1") {
		return fmt.Errorf("the peer rows are missing from the whole answer: %q", whole)
	}
	if !strings.Contains(whole, "router-id") {
		return fmt.Errorf("the summary does not carry router-id: %q", whole)
	}

	only, err := cli("show bgp | summary | text")
	if err != nil {
		return err
	}
	for _, key := range []string{fieldRouterID, fieldLocalAS, fieldPeersConfigured, fieldPeersEstablished} {
		if !strings.Contains(only, key) {
			return fmt.Errorf("the aggregate %s is missing: %q", key, only)
		}
	}
	if !strings.Contains(only, "10.255.255.254") {
		return fmt.Errorf("the router-id value is missing: %q", only)
	}
	if strings.Contains(only, "192.0.2.1") {
		return fmt.Errorf("a peer row survived `| summary`: %q", only)
	}
	if strings.Contains(only, "192.0.2.2") {
		return fmt.Errorf("a peer row survived `| summary`: %q", only)
	}

	// "peers" is a prefix of two aggregate keys, so look for the rows array by
	// checking the key at the start of each non-empty line.
	for line := range strings.SplitSeq(only, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 0 && fields[0] == aliasPeers {
			return fmt.Errorf("the peers array survived `| summary`: %q", only)
		}
	}

	// The alias is a selection, so JSON output carries it too.
	asJSON, err := cli("show bgp | summary | json")
	if err != nil {
		return err
	}
	if !strings.Contains(asJSON, "router-id") {
		return fmt.Errorf("`| summary | json` dropped the aggregates: %q", asJSON)
	}
	if strings.Contains(asJSON, "192.0.2.1") {
		return fmt.Errorf("`| summary | json` kept the peer rows: %q", asJSON)
	}
	return nil
}

func aliasSummaryWaitReady(ctx context.Context, daemon *aliasSummaryDaemon, sshAddrPath, readyPath string, attempts int, delay time.Duration) error {
	for range attempts {
		if exited, _ := daemon.exited(); exited {
			return fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		}
		if aliasSummaryPathExists(sshAddrPath) && aliasSummaryPathExists(readyPath) {
			return nil
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("daemon did not become ready")
}

func aliasSummaryPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func aliasSummaryRunCommand(ctx context.Context, env []string, name string, args ...string) (int, string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String(), nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String(), nil
	}
	return 0, stdout.String(), stderr.String(), err
}

func aliasSummaryEnvironment(base []string, overrides ...string) []string {
	replaced := make(map[string]struct{}, len(overrides))
	for _, item := range overrides {
		if key, _, found := strings.Cut(item, "="); found {
			replaced[key] = struct{}{}
		}
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		if key, _, found := strings.Cut(item, "="); found {
			if _, ok := replaced[key]; ok {
				continue
			}
		}
		out = append(out, item)
	}
	return append(out, overrides...)
}

func (daemon *aliasSummaryDaemon) exited() (bool, error) {
	if daemon.waited {
		return true, daemon.waitErr
	}
	select {
	case daemon.waitErr = <-daemon.done:
		daemon.waited = true
		return true, daemon.waitErr
	default:
		return false, nil
	}
}

func (daemon *aliasSummaryDaemon) waitFor(timeout time.Duration) bool {
	if daemon.waited {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case daemon.waitErr = <-daemon.done:
		daemon.waited = true
		return true
	case <-timer.C:
		return false
	}
}

func (daemon *aliasSummaryDaemon) stop() error {
	if exited, _ := daemon.exited(); exited {
		return nil
	}

	if err := daemon.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		if exited, _ := daemon.exited(); !exited {
			return fmt.Errorf("terminate daemon: %w", err)
		}
	}
	if daemon.waitFor(5 * time.Second) {
		return nil
	}

	if err := daemon.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		if exited, _ := daemon.exited(); !exited {
			return fmt.Errorf("kill daemon: %w", err)
		}
	}
	if !daemon.waitFor(5 * time.Second) {
		return errors.New("daemon did not exit after kill")
	}
	return nil
}

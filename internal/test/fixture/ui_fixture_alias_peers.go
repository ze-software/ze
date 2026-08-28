package fixture

import (
	"bytes"
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
	Register("ui/alias-peers", uiDriver(uiAliasPeers))
}

type uiAliasPeersResult struct {
	code   int
	stdout string
	stderr string
}

func uiAliasPeersRun(ctx context.Context, argv []string, env []string) (uiAliasPeersResult, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := uiAliasPeersResult{
		code:   0,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func uiAliasPeersEnv(base []string, values map[string]string) []string {
	env := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := values[name]; replaced {
				continue
			}
		}
		env = append(env, entry)
	}
	for name, value := range values {
		env = append(env, name+"="+value)
	}
	return env
}

func uiAliasPeersExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func uiAliasPeers(ctx context.Context) error {
	passwd := exec.CommandContext(ctx, "ze", "passwd")
	passwd.Stdin = strings.NewReader("secret\n")
	var passwdOutput bytes.Buffer
	passwd.Stdout = &passwdOutput
	passwd.Stderr = os.Stderr
	if err := passwd.Run(); err != nil {
		return fmt.Errorf("ze passwd: %w", err)
	}
	passwordHash := strings.TrimSpace(passwdOutput.String())

	config := fmt.Sprintf(`bgp {
    router-id 192.0.2.254
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
            password "%s"
            profile [ admin ]
        }
    }
}
`, passwordHash)
	work, err := os.MkdirTemp("", "ze-ui-alias-peers-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work)
	configPath := filepath.Join(work, "summary.conf")
	sshAddressPath := filepath.Join(work, "ssh.addr")
	readyPath := filepath.Join(work, "ready")
	if err := os.WriteFile(configPath, []byte(config), 0o666); err != nil {
		return fmt.Errorf("write summary.conf: %w", err)
	}
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := uiAliasPeersEnv(os.Environ(), map[string]string{
		"ZE_SSH_EPHEMERAL": sshAddressPath,
		"ZE_READY_FILE":    readyPath,
		"ZE_CONFIG_DIR":    work,
		// Leave port 179 alone: this suite runs unprivileged and a bind failure there
		// takes the daemon down before it writes `ready`.
		"ze_test_bgp_port": strconv.Itoa(bgpPort),
	})

	daemon := exec.CommandContext(ctx, "ze", "-f", configPath)
	daemon.Dir = work
	daemon.Env = daemonEnv
	var daemonStdout bytes.Buffer
	var daemonStderr bytes.Buffer
	daemon.Stdout = &daemonStdout
	daemon.Stderr = &daemonStderr
	if err := daemon.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	daemonDone := make(chan error, 1)
	go func() {
		daemonDone <- daemon.Wait()
	}()

	daemonExited := false
	markDaemonExited := func() error {
		daemonExited = true
		return fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemonStdout.String(), daemonStderr.String())
	}

	waitForDaemon := func(timeout time.Duration) bool {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-daemonDone:
			daemonExited = true
			return true
		case <-timer.C:
			return false
		}
	}

	cleanupDaemon := func() error {
		if daemonExited {
			return nil
		}

		if err := daemon.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("terminate daemon: %w", err)
		}
		if waitForDaemon(5 * time.Second) {
			return nil
		}

		if err := daemon.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill daemon: %w", err)
		}
		if !waitForDaemon(5 * time.Second) {
			return errors.New("daemon did not exit after being killed")
		}
		return nil
	}

	testErr := func() error {
		ready := false
		for attempt := 0; attempt < 200; attempt++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-daemonDone:
				return markDaemonExited()
			default:
			}

			if uiAliasPeersExists(sshAddressPath) && uiAliasPeersExists(readyPath) {
				ready = true
				break
			}

			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-daemonDone:
				timer.Stop()
				return markDaemonExited()
			case <-timer.C:
			}
		}
		if !ready {
			return errors.New("daemon did not become ready")
		}

		addressBytes, err := os.ReadFile(sshAddressPath)
		if err != nil {
			return fmt.Errorf("read ssh.addr: %w", err)
		}
		address := strings.TrimSpace(string(addressBytes))
		colon := strings.LastIndexByte(address, ':')
		if colon < 0 {
			return fmt.Errorf("invalid SSH address %q", address)
		}
		host, port := address[:colon], address[colon+1:]
		cliEnv := uiAliasPeersEnv(os.Environ(), map[string]string{
			"ZE_SSH_HOST":     host,
			"ZE_SSH_PORT":     port,
			"ZE_SSH_USERNAME": "ci",
			"ZE_SSH_PASSWORD": "secret",
			"ZE_CONFIG_DIR":   work,
		})

		cli := func(command string) (string, error) {
			result, err := uiAliasPeersRun(ctx, []string{"ze", "cli", "-c", command}, cliEnv)
			if err != nil {
				return "", fmt.Errorf("run %q: %w", command, err)
			}
			if result.code != 0 {
				return "", fmt.Errorf("%s exit=%d: %s%s", command, result.code, result.stdout, result.stderr)
			}
			return result.stdout, nil
		}

		// The whole answer first, so the assertions below measure a CHANGE rather
		// than an answer that was always narrow.
		whole, err := cli("show bgp | text")
		if err != nil {
			return err
		}
		for _, key := range []string{"router-id", "local-as", "peers-configured"} {
			if !strings.Contains(whole, key) {
				return fmt.Errorf("the summary does not carry %s: %q", key, whole)
			}
		}
		if !strings.Contains(whole, "192.0.2.1") || !strings.Contains(whole, "192.0.2.2") {
			return fmt.Errorf("the peer rows are missing: %q", whole)
		}

		rows, err := cli("show bgp | peers | text")
		if err != nil {
			return err
		}
		if !strings.Contains(rows, "192.0.2.1") {
			return fmt.Errorf("the alias dropped the first peer row: %q", rows)
		}
		if !strings.Contains(rows, "192.0.2.2") {
			return fmt.Errorf("the alias dropped the second peer row: %q", rows)
		}
		if strings.Contains(rows, "192.0.2.254") {
			return fmt.Errorf("the router-id survived `| peers`: %q", rows)
		}
		if strings.Contains(rows, "65000") {
			return fmt.Errorf("the local AS survived `| peers`: %q", rows)
		}

		// The rows render as a TABLE, so one header line names the peer columns and
		// each peer sits on a line of its own.
		var header []string
		for _, line := range strings.Split(rows, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 0 || (fields[0] != "address" && fields[0] != "peers") {
				continue
			}
			for _, field := range fields {
				if field != "peers" {
					header = append(header, field)
				}
			}
			break
		}
		if header == nil {
			return fmt.Errorf("the peer rows did not render as a table: %q", rows)
		}
		if len(header) == 0 || header[0] != "address" {
			return fmt.Errorf("the peer table must lead with the peer: %q", header)
		}
		hasState := false
		for _, field := range header {
			if field == "state" {
				hasState = true
				break
			}
		}
		if !hasState {
			return fmt.Errorf("the peer table lost its state column: %q", header)
		}

		// The alias is a selection, so every format carries it. A program reading
		// `| json` sees the same fields the table drew.
		asJSON, err := cli("show bgp | peers | json")
		if err != nil {
			return err
		}
		if !strings.Contains(asJSON, `"address"`) {
			return fmt.Errorf("`| peers | json` dropped the rows: %q", asJSON)
		}
		if strings.Contains(asJSON, "router-id") {
			return fmt.Errorf("`| peers | json` kept the aggregates: %q", asJSON)
		}

		// An alias takes no argument, and a word after it is refused by name. The
		// refusal is asserted on the message rather than on the alias name, which
		// the accepted answer would carry as a column header too.
		refused, err := uiAliasPeersRun(ctx, []string{"ze", "cli", "-c", "show bgp | peers established"}, cliEnv)
		if err != nil {
			return fmt.Errorf("run refused alias chain: %w", err)
		}
		refusal := refused.stdout + refused.stderr
		if !strings.Contains(refusal, "does not accept an argument") {
			return fmt.Errorf("an argument after the alias was accepted in silence: %q %q", refused.stdout, refused.stderr)
		}
		if !strings.Contains(refusal, "peers") {
			return fmt.Errorf("the refusal does not name the alias: %q %q", refused.stdout, refused.stderr)
		}
		if strings.Contains(refusal, "192.0.2.1") {
			return fmt.Errorf("the refused chain answered rows anyway: %q", refusal)
		}
		return nil
	}()

	cleanupErr := cleanupDaemon()
	if testErr != nil {
		return testErr
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	if _, err := fmt.Fprintln(os.Stdout, "OK"); err != nil {
		return fmt.Errorf("write success marker: %w", err)
	}
	return nil
}

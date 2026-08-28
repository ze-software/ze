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
	Register("ui/show-bgp-children-do-not-inherit", uiDriver(showBGPChildrenDoNotInherit))
}

type childResult struct {
	code   int
	stdout string
	stderr string
}

type uiShowBgpChildrenDoNotInheritDaemonProcess struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}
}

func showBGPChildrenDoNotInherit(ctx context.Context) error {
	passwordHash, err := uiShowBgpChildrenDoNotInheritMakePasswordHash(ctx)
	if err != nil {
		return err
	}

	config := `bgp {
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
	workDir, err := os.MkdirTemp("", "ze-ui-show-bgp-children-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	configPath := filepath.Join(workDir, "children.conf")
	sshAddr := filepath.Join(workDir, "ssh.addr")
	readyFile := filepath.Join(workDir, "ready")
	if err := os.WriteFile(configPath, []byte(config), 0o666); err != nil {
		return fmt.Errorf("write children.conf: %w", err)
	}

	// Leave port 179 alone: this suite runs unprivileged and a bind failure there
	// takes the daemon down before it writes ready.
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := uiShowBgpChildrenDoNotInheritReplaceEnv(os.Environ(), map[string]string{
		"ZE_SSH_EPHEMERAL": sshAddr,
		"ZE_READY_FILE":    readyFile,
		"ZE_CONFIG_DIR":    workDir,
		"ze_test_bgp_port": strconv.Itoa(bgpPort),
	})

	daemon, err := uiShowBgpChildrenDoNotInheritStartDaemon(ctx, workDir, daemonEnv)
	if err != nil {
		return err
	}
	defer daemon.stop()

	ready, err := uiShowBgpChildrenDoNotInheritPoll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		select {
		case <-daemon.done:
			return false, fmt.Errorf(
				"daemon exited early\nstdout:\n%s\nstderr:\n%s",
				daemon.stdout.String(), daemon.stderr.String(),
			)
		default:
		}
		return uiShowBgpChildrenDoNotInheritFileExists(sshAddr) && uiShowBgpChildrenDoNotInheritFileExists(readyFile), nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddr)
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH listener address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]

	cliEnv := uiShowBgpChildrenDoNotInheritReplaceEnv(os.Environ(), map[string]string{
		"ZE_SSH_HOST":     host,
		"ZE_SSH_PORT":     port,
		"ZE_SSH_USERNAME": "ci",
		"ZE_SSH_PASSWORD": "secret",
		"ZE_CONFIG_DIR":   workDir,
	})

	// The control. The parent DOES answer both aliases, so a refusal below is a
	// refusal of the inheritance and not of a feature that never ran.
	result, err := uiShowBgpChildrenDoNotInheritRunZE(ctx, cliEnv, "cli", "-c", "show bgp | peers | text")
	if err != nil {
		return err
	}
	if result.code != 0 {
		return fmt.Errorf("show bgp | peers exit=%d: %s%s", result.code, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "192.0.2.1") {
		return fmt.Errorf("the parent alias answered no peer rows: %q", result.stdout)
	}

	result, err = uiShowBgpChildrenDoNotInheritRunZE(ctx, cliEnv, "cli", "-c", "show bgp | summary | text")
	if err != nil {
		return err
	}
	if result.code != 0 {
		return fmt.Errorf("show bgp | summary exit=%d: %s%s", result.code, result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, "router-id") {
		return fmt.Errorf("the parent alias answered no aggregates: %q", result.stdout)
	}

	// Each child is driven with BOTH names, because they are registered together
	// and a repair that reaches one can miss the other.
	children := []string{
		"show bgp peer list",
		"show bgp peer 192.0.2.1 detail",
		"show bgp peer 192.0.2.1 statistics",
		"show bgp peer 192.0.2.1 capabilities",
	}
	for _, child := range children {
		// The child must answer on its own, so the refusals below cannot be a
		// command that was broken before any pipe was parsed.
		result, err = uiShowBgpChildrenDoNotInheritRunZE(ctx, cliEnv, "cli", "-c", child+" | text")
		if err != nil {
			return err
		}
		if result.code != 0 {
			return fmt.Errorf("%s exit=%d: %s%s", child, result.code, result.stdout, result.stderr)
		}
		if strings.TrimSpace(result.stdout) == "" {
			return fmt.Errorf("%s answered nothing: %q", child, result.stdout)
		}

		for _, alias := range []string{"peers", "summary"} {
			command := child + " | " + alias
			result, err = uiShowBgpChildrenDoNotInheritRunZE(ctx, cliEnv, "cli", "-c", command)
			if err != nil {
				return err
			}
			refusal := result.stdout + result.stderr
			if !strings.Contains(refusal, "unknown pipe operator") {
				return fmt.Errorf("%s: the inherited alias was accepted: exit=%d %q", command, result.code, refusal)
			}
			if !strings.Contains(refusal, alias) {
				return fmt.Errorf("%s: the refusal does not name it: %q", command, refusal)
			}
		}
	}

	fmt.Println("OK")
	return nil
}

func uiShowBgpChildrenDoNotInheritMakePasswordHash(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Stdin = strings.NewReader("secret\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func uiShowBgpChildrenDoNotInheritStartDaemon(ctx context.Context, dir string, env []string) (*uiShowBgpChildrenDoNotInheritDaemonProcess, error) {
	d := &uiShowBgpChildrenDoNotInheritDaemonProcess{done: make(chan struct{})}
	d.cmd = exec.CommandContext(ctx, "ze", "-f", "children.conf")
	d.cmd.Dir = dir
	d.cmd.Env = env
	d.cmd.Stdout = &d.stdout
	d.cmd.Stderr = &d.stderr
	if err := d.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}
	go func() {
		_ = d.cmd.Wait()
		close(d.done)
	}()
	return d, nil
}

func (d *uiShowBgpChildrenDoNotInheritDaemonProcess) stop() {
	select {
	case <-d.done:
		return
	default:
	}

	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	if uiShowBgpChildrenDoNotInheritWaitForDone(d.done, 5*time.Second) {
		return
	}

	_ = d.cmd.Process.Kill()
	_ = uiShowBgpChildrenDoNotInheritWaitForDone(d.done, 5*time.Second)
}

func uiShowBgpChildrenDoNotInheritWaitForDone(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func uiShowBgpChildrenDoNotInheritRunZE(ctx context.Context, env []string, args ...string) (childResult, error) {
	cmd := exec.CommandContext(ctx, "ze", args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := childResult{
		code:   0,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return result, fmt.Errorf("run ze %q: %w", args, err)
}

func uiShowBgpChildrenDoNotInheritPoll(ctx context.Context, attempts int, delay time.Duration, check func() (bool, error)) (bool, error) {
	for attempt := 0; attempt < attempts; attempt++ {
		ready, err := check()
		if err != nil || ready {
			return ready, err
		}
		if attempt+1 == attempts {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false, ctx.Err()
		case <-timer.C:
		}
	}
	return false, nil
}

func uiShowBgpChildrenDoNotInheritFileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func uiShowBgpChildrenDoNotInheritReplaceEnv(base []string, values map[string]string) []string {
	out := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if found {
			if _, replace := values[key]; replace {
				continue
			}
		}
		out = append(out, entry)
	}
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}

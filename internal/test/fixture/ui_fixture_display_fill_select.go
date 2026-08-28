package fixture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/display-fill-select", uiDriver(displayFillSelect))
}

type displayFillSelectEnvVar struct {
	name  string
	value string
}

type displayFillSelectDaemon struct {
	cmd     *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	waitc   chan error
	done    bool
	waitErr error
}

func displayFillSelect(ctx context.Context) (retErr error) {
	workDir, err := os.MkdirTemp("", "ze-display-fill-select-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	passwordHash, err := displayFillSelectPasswordHash(ctx, workDir)
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
	if err := os.WriteFile(filepath.Join(workDir, "peers.conf"), []byte(config), 0o666); err != nil {
		return fmt.Errorf("write peers.conf: %w", err)
	}

	sshAddrPath := filepath.Join(workDir, "ssh.addr")
	readyPath := filepath.Join(workDir, "ready")
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := displayFillSelectWithEnv(os.Environ(),
		displayFillSelectEnvVar{"ZE_SSH_EPHEMERAL", sshAddrPath},
		displayFillSelectEnvVar{"ZE_READY_FILE", readyPath},
		displayFillSelectEnvVar{"ZE_CONFIG_DIR", workDir},
		// Leave port 179 alone: this suite runs unprivileged and a bind
		// failure there takes the daemon down before it writes ready.
		displayFillSelectEnvVar{"ze_test_bgp_port", fmt.Sprintf("%d", bgpPort)},
	)

	daemon, err := displayFillSelectStartDaemon(ctx, workDir, daemonEnv)
	if err != nil {
		return err
	}
	stopped := false
	defer func() {
		if stopped {
			return
		}
		if err := daemon.stop(); err != nil {
			if retErr == nil {
				retErr = err
			} else {
				retErr = fmt.Errorf("%w; daemon cleanup failed: %v", retErr, err)
			}
		}
	}()

	ready, err := displayFillSelectPoll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		if daemon.exited() {
			return false, fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		}
		return displayFillSelectExists(sshAddrPath) && displayFillSelectExists(readyPath), nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddrPath)
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH listener address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]
	cliEnv := displayFillSelectWithEnv(os.Environ(),
		displayFillSelectEnvVar{"ZE_SSH_HOST", host},
		displayFillSelectEnvVar{"ZE_SSH_PORT", port},
		displayFillSelectEnvVar{"ZE_SSH_USERNAME", "ci"},
		displayFillSelectEnvVar{"ZE_SSH_PASSWORD", "secret"},
		displayFillSelectEnvVar{"ZE_CONFIG_DIR", workDir},
	)

	out, err := displayFillSelectCLI(ctx, workDir, cliEnv, "show bgp peer list | display state name | text")
	if err != nil {
		return err
	}
	header, err := displayFillSelectTableHeader(out, "state")
	if err != nil {
		return err
	}
	if !displayFillSelectEqualStrings(header, []string{"state", "name"}) {
		return fmt.Errorf("peer columns %q, want the two displayed ones", header)
	}
	for _, absent := range []string{"remote-as", "group", "uptime"} {
		if strings.Contains(out, absent) {
			return fmt.Errorf("a column nobody displayed survived: %q in %q", absent, out)
		}
	}
	for _, peer := range []string{"192.0.2.1", "192.0.2.2"} {
		if !strings.Contains(out, peer) {
			return fmt.Errorf("the address that identifies a row was cut: %q not in %q", peer, out)
		}
	}

	// The same command with no | display still answers with every column, so
	// the assertions above are a cut rather than a command that answers little.
	whole, err := displayFillSelectCLI(ctx, workDir, cliEnv, "show bgp peer list | text")
	if err != nil {
		return err
	}
	for _, present := range []string{"remote-as", "group", "uptime", "name", "state"} {
		if !strings.Contains(whole, present) {
			return fmt.Errorf("the unfiltered answer is missing %q: %q", present, whole)
		}
	}

	// A program reads | json. Selection is data, so it travels; sequence is
	// presentation, so it does not.
	asJSON, err := displayFillSelectCLI(ctx, workDir, cliEnv, "show bgp peer list | display state name | json")
	if err != nil {
		return err
	}
	if strings.Contains(asJSON, "┌") {
		return fmt.Errorf("| json answered a table: %q", asJSON)
	}
	if !strings.Contains(asJSON, `"state"`) || !strings.Contains(asJSON, `"name"`) {
		return fmt.Errorf("| json dropped a displayed field: %q", asJSON)
	}
	for _, absent := range []string{`"remote-as"`, `"group"`, `"uptime"`} {
		if strings.Contains(asJSON, absent) {
			return fmt.Errorf("| json kept %s, which nobody displayed: %q", absent, asJSON)
		}
	}

	if err := daemon.stop(); err != nil {
		return err
	}
	stopped = true
	fmt.Println("OK")
	return nil
}

func displayFillSelectPasswordHash(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("secret\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w: %s%s", err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func displayFillSelectStartDaemon(ctx context.Context, dir string, env []string) (*displayFillSelectDaemon, error) {
	d := &displayFillSelectDaemon{waitc: make(chan error, 1)}
	d.cmd = exec.CommandContext(ctx, "ze", "-f", "peers.conf")
	d.cmd.Dir = dir
	d.cmd.Env = env
	d.cmd.Stdout = &d.stdout
	d.cmd.Stderr = &d.stderr
	if err := d.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}
	go func() {
		d.waitc <- d.cmd.Wait()
	}()
	return d, nil
}

func (d *displayFillSelectDaemon) exited() bool {
	if d.done {
		return true
	}
	select {
	case d.waitErr = <-d.waitc:
		d.done = true
		return true
	default:
		return false
	}
}

func (d *displayFillSelectDaemon) waitFor(timeout time.Duration) bool {
	if d.done {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case d.waitErr = <-d.waitc:
		d.done = true
		return true
	case <-timer.C:
		return false
	}
}

func (d *displayFillSelectDaemon) stop() error {
	if d.exited() {
		return nil
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	if d.waitFor(5 * time.Second) {
		return nil
	}
	_ = d.cmd.Process.Kill()
	if d.waitFor(5 * time.Second) {
		return nil
	}
	return fmt.Errorf("daemon did not exit within five seconds after being killed")
}

func displayFillSelectCLI(ctx context.Context, dir string, env []string, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "cli", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return "", fmt.Errorf("%s exit=%d: %s%s", command, code, stdout.String(), stderr.String())
}

func displayFillSelectTableHeader(text, marker string) ([]string, error) {
	// The peer table is the value of a key, so its header row can share a line
	// with that key. marker names the header unambiguously.
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && (fields[0] == "peers" || fields[0] == "peer") {
			return fields[1:], nil
		}
		return fields, nil
	}
	return nil, fmt.Errorf("no header row carrying %q in: %q", marker, text)
}

func displayFillSelectEqualStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func displayFillSelectExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func displayFillSelectPoll(ctx context.Context, attempts int, delay time.Duration, check func() (bool, error)) (bool, error) {
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

func displayFillSelectWithEnv(base []string, updates ...displayFillSelectEnvVar) []string {
	replaced := make(map[string]struct{}, len(updates))
	for _, update := range updates {
		replaced[update.name] = struct{}{}
	}

	env := make([]string, 0, len(base)+len(updates))
	for _, entry := range base {
		name := entry
		if equals := strings.IndexByte(entry, '='); equals >= 0 {
			name = entry[:equals]
		}
		if _, ok := replaced[name]; !ok {
			env = append(env, entry)
		}
	}
	for _, update := range updates {
		env = append(env, update.name+"="+update.value)
	}
	return env
}

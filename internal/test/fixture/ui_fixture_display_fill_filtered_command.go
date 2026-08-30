package fixture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func init() {
	Register("ui/display-fill-filtered-command", uiDriver(displayFillFilteredCommand))
}

func displayFillFilteredCommand(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "ze-display-fill-filtered-command-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup

	dir, err = filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve fixture directory: %w", err)
	}

	passwordHash, err := uiDisplayFillFilteredCommandMakePasswordHash(ctx, dir)
	if err != nil {
		return err
	}

	config := `plugin {
    internal bgp-rib {
        use bgp-rib
    }
}

bgp {
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
            attach process bgp-rib {
                receive [ update state refresh ]
                send [ update ]
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
	if err := os.WriteFile(filepath.Join(dir, "rib.conf"), []byte(config), 0o600); err != nil {
		return fmt.Errorf("write rib.conf: %w", err)
	}

	sshAddr := filepath.Join(dir, "ssh.addr")
	readyFile := filepath.Join(dir, "ready")
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := setEnvironment(os.Environ(),
		"ZE_SSH_EPHEMERAL", sshAddr,
		"ZE_READY_FILE", readyFile,
		"ZE_CONFIG_DIR", dir,
		// Leave port 179 alone: the suite runs unprivileged, and a bind
		// failure there takes the daemon down before it writes ready.
		"ze_test_bgp_port", fmt.Sprintf("%d", bgpPort),
	)

	daemon, err := uiDisplayFillFilteredCommandStartFixtureDaemon(ctx, dir, daemonEnv)
	if err != nil {
		return err
	}

	runErr := exerciseDisplayAndFill(ctx, daemon, dir, sshAddr, readyFile)
	stopErr := daemon.stop()
	if runErr != nil {
		return runErr
	}
	if stopErr != nil {
		return stopErr
	}

	fmt.Println("OK")
	return nil
}

func uiDisplayFillFilteredCommandMakePasswordHash(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader("secret\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd failed: %w; stdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

type uiDisplayFillFilteredCommandFixtureDaemon struct {
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	done   chan struct{}
}

func uiDisplayFillFilteredCommandStartFixtureDaemon(ctx context.Context, dir string, env []string) (*uiDisplayFillFilteredCommandFixtureDaemon, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "ze", "-f", "rib.conf")
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	d := &uiDisplayFillFilteredCommandFixtureDaemon{
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
		done:   make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(d.done)
	}()
	return d, nil
}

func (d *uiDisplayFillFilteredCommandFixtureDaemon) exited() bool {
	select {
	case <-d.done:
		return true
	default:
		return false
	}
}

func (d *uiDisplayFillFilteredCommandFixtureDaemon) stop() error {
	if !d.exited() {
		_ = d.cmd.Process.Signal(syscall.SIGTERM)
	}
	if uiDisplayFillFilteredCommandWaitForDone(d.done, 5*time.Second) {
		return nil
	}

	_ = d.cmd.Process.Kill()
	if uiDisplayFillFilteredCommandWaitForDone(d.done, 5*time.Second) {
		return nil
	}
	return fmt.Errorf("daemon did not exit after being killed")
}

func uiDisplayFillFilteredCommandWaitForDone(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func exerciseDisplayAndFill(ctx context.Context, daemon *uiDisplayFillFilteredCommandFixtureDaemon, dir, sshAddr, readyFile string) error {
	ready, err := pollFixture(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		if daemon.exited() {
			return false, fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		}
		return uiDisplayFillFilteredCommandPathExists(sshAddr) && uiDisplayFillFilteredCommandPathExists(readyFile), nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddr) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH listener address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]

	cliEnv := setEnvironment(os.Environ(),
		"ZE_SSH_HOST", host,
		"ZE_SSH_PORT", port,
		"ZE_SSH_USERNAME", "ci",
		"ZE_SSH_PASSWORD", "secret",
		"ZE_CONFIG_DIR", dir,
	)

	if _, err := uiDisplayFillFilteredCommandRunCLI(ctx, dir, cliEnv, "request bgp rib inject 192.0.2.1 ipv4/unicast 10.10.1.0/24 aspath 64501,64502"); err != nil {
		return err
	}
	if _, err := uiDisplayFillFilteredCommandRunCLI(ctx, dir, cliEnv, "request bgp rib inject 192.0.2.1 ipv4/unicast 10.20.1.0/24 aspath 64601,64602"); err != nil {
		return err
	}

	whole, err := uiDisplayFillFilteredCommandRunCLI(ctx, dir, cliEnv, "show bgp rib | text")
	if err != nil {
		return err
	}
	if !strings.Contains(whole, "10.10.1.0/24") {
		return fmt.Errorf("the injected route is not in the rib: %q", whole)
	}
	columns, err := headerOf(whole)
	if err != nil {
		return err
	}
	if len(columns) <= 2 {
		return fmt.Errorf("the rib row is too narrow to say anything: %q", columns)
	}

	// An unclassified kind is dropped here in silence, so a passing run needs
	// the answer to change.
	cut, err := uiDisplayFillFilteredCommandRunCLI(ctx, dir, cliEnv, "show bgp rib | display prefix | text")
	if err != nil {
		return err
	}
	cutColumns, err := headerOf(cut)
	if err != nil {
		return err
	}
	if !uiDisplayFillFilteredCommandEqualStrings(cutColumns, []string{columnPrefix}) {
		return fmt.Errorf("the display was dropped on a filtered command: %q", cut)
	}
	if !strings.Contains(cut, "10.10.1.0/24") {
		return fmt.Errorf("the rows went missing with the columns: %q", cut)
	}

	// Fill is the second kind, and it is classified in the same switch.
	filled, err := uiDisplayFillFilteredCommandRunCLI(ctx, dir, cliEnv, "show bgp rib | display prefix | fill alpha | text")
	if err != nil {
		return err
	}
	filledColumns, err := headerOf(filled)
	if err != nil {
		return err
	}
	if filledColumns[0] != columnPrefix {
		return fmt.Errorf("the displayed column must lead: %q", filledColumns)
	}
	if len(filledColumns) != len(columns) {
		return fmt.Errorf("fill did not bring every column back: %q", filledColumns)
	}
	sortedTail := append([]string(nil), filledColumns[1:]...)
	sort.Strings(sortedTail)
	if !uiDisplayFillFilteredCommandEqualStrings(filledColumns[1:], sortedTail) {
		return fmt.Errorf("fill alpha did not order by name: %q", filledColumns)
	}

	// A server-side filter of the command's own still folds beside them.
	both, err := uiDisplayFillFilteredCommandRunCLI(ctx, dir, cliEnv, "show bgp rib | prefix 10.10 | display prefix | text")
	if err != nil {
		return err
	}
	if !strings.Contains(both, "10.10.1.0/24") {
		return fmt.Errorf("the server-side filter dropped its own route: %q", both)
	}
	if strings.Contains(both, "10.20.1.0/24") {
		return fmt.Errorf("the server-side filter stopped working: %q", both)
	}
	return nil
}

func uiDisplayFillFilteredCommandRunCLI(ctx context.Context, dir string, env []string, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "cli", "-c", command) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return "", fmt.Errorf("%s exit=%d: %s%s", command, exitCode, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}

func headerOf(text string) ([]string, error) {
	// Text output writes the header row first and draws no border, so the first
	// non-empty line names the columns.
	for line := range strings.SplitSeq(text, "\n") {
		if fields := strings.Fields(line); len(fields) != 0 {
			return fields, nil
		}
	}
	return nil, fmt.Errorf("no rib table in: %q", text)
}

func uiDisplayFillFilteredCommandEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uiDisplayFillFilteredCommandPathExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func pollFixture(ctx context.Context, attempts int, delay time.Duration, check func() (bool, error)) (bool, error) {
	for attempt := range attempts {
		ok, err := check()
		if err != nil || ok {
			return ok, err
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

func setEnvironment(base []string, pairs ...string) []string {
	keys := make(map[string]struct{}, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		keys[pairs[i]] = struct{}{}
	}

	env := make([]string, 0, len(base)+len(pairs)/2)
	for _, entry := range base {
		key := entry
		if before, _, found := strings.Cut(entry, "="); found {
			key = before
		}
		if _, replaced := keys[key]; !replaced {
			env = append(env, entry)
		}
	}
	for i := 0; i < len(pairs); i += 2 {
		env = append(env, pairs[i]+"="+pairs[i+1])
	}
	return env
}

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	Register("ui/display-fill-remainder", uiDriver(displayFillRemainder))
}

type managedProcess struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
	done   chan struct{}

	mu      sync.Mutex
	waitErr error
}

func startManagedProcess(ctx context.Context, argv []string, dir string, env []string) (*managedProcess, error) {
	p := &managedProcess{done: make(chan struct{})}
	p.cmd = exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	p.cmd.Dir = dir
	p.cmd.Env = env
	p.cmd.Stdout = &p.stdout
	p.cmd.Stderr = &p.stderr
	if err := p.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := p.cmd.Wait()
		p.mu.Lock()
		p.waitErr = err
		p.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

func (p *managedProcess) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *managedProcess) result() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

func (p *managedProcess) output() (string, string) {
	return p.stdout.String(), p.stderr.String()
}

func (p *managedProcess) stop() error {
	if p.exited() {
		return nil
	}

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && !p.exited() {
		return fmt.Errorf("terminate daemon: %w", err)
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		return nil
	case <-timer.C:
	}

	if err := p.cmd.Process.Kill(); err != nil && !p.exited() {
		return fmt.Errorf("kill daemon: %w", err)
	}

	timer.Reset(5 * time.Second)
	select {
	case <-p.done:
		return nil
	case <-timer.C:
		return fmt.Errorf("daemon did not exit within 5s after being killed")
	}
}

func displayFillRemainder(ctx context.Context) (retErr error) {
	dir, err := os.MkdirTemp("", "ze-display-fill-remainder-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // fixture cleanup

	passwordHash, err := zePasswordHash(ctx, dir, "secret\n")
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
	configPath := filepath.Join(dir, "peers.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write peers.conf: %w", err)
	}

	sshAddrPath := filepath.Join(dir, "ssh.addr")
	readyPath := filepath.Join(dir, "ready")
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := uiDisplayFillRemainderEnvironmentWith(os.Environ(), map[string]string{
		envSSHEphemeral: sshAddrPath,
		envReadyFile:    readyPath,
		envConfigDir:    dir,
		// Leave port 179 alone: the suite runs unprivileged, and a bind
		// failure there takes the daemon down before it writes ready.
		envTestBGPPort: fmt.Sprintf("%d", bgpPort),
	})

	daemon, err := startManagedProcess(ctx, []string{"ze", "-f", "peers.conf"}, dir, daemonEnv)
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer func() {
		if err := daemon.stop(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	var readyErr error
	ready := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		if daemon.exited() {
			stdout, stderr := daemon.output()
			readyErr = fmt.Errorf("daemon exited early: %w\nstdout:\n%s\nstderr:\n%s", daemon.result(), stdout, stderr)
			return true
		}
		return uiDisplayFillRemainderPathExists(sshAddrPath) && uiDisplayFillRemainderPathExists(readyPath)
	})
	if readyErr != nil {
		return readyErr
	}
	if !ready {
		return errors.New("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddrPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH listener address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]

	cliEnv := uiDisplayFillRemainderEnvironmentWith(os.Environ(), map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   dir,
	})

	// A bare | fill brings the rest back in the order the command declared.
	// show bgp peer list declares one, and it is not the alphabet.
	out, err := uiDisplayFillRemainderRunCLI(ctx, dir, cliEnv, "show bgp peer list | display state | fill | text")
	if err != nil {
		return err
	}
	header, err := tableHeader(out)
	if err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(len(header) > 0 && header[0] == columnState, "the displayed column must lead: %q", header); err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(len(header) > 2, "nothing was filled in, so this file proves nothing: %q", header); err != nil {
		return err
	}
	declared := append([]string(nil), header[1:]...)
	if err := uiDisplayFillRemainderRequire(!uiDisplayFillRemainderEqualStrings(declared, sortedStrings(declared, false)), "the remainder came back in name order, so the declaration was ignored: %q", header); err != nil {
		return err
	}

	// | fill alpha forces name order over that declaration.
	out, err = uiDisplayFillRemainderRunCLI(ctx, dir, cliEnv, "show bgp peer list | display state | fill alpha | text")
	if err != nil {
		return err
	}
	header, err = tableHeader(out)
	if err != nil {
		return err
	}
	forced := append([]string(nil), header[1:]...)
	if err := uiDisplayFillRemainderRequire(uiDisplayFillRemainderEqualStrings(forced, sortedStrings(forced, false)), "alpha did not force name order: %q", forced); err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(uiDisplayFillRemainderEqualStringSets(forced, declared), "alpha changed which columns appear: %q", forced); err != nil {
		return err
	}

	// | fill alpha brings the rest back by name, behind the displayed column.
	out, err = uiDisplayFillRemainderRunCLI(ctx, dir, cliEnv, "show bgp peer list | display state | fill alpha | text")
	if err != nil {
		return err
	}
	header, err = tableHeader(out)
	if err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(len(header) > 0 && header[0] == columnState, "the displayed column must lead: %q", header); err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(uiDisplayFillRemainderEqualStrings(header[1:], sortedStrings(header[1:], false)), "the remaining columns are not in name order: %q", header); err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(len(header) > 2, "nothing was filled in, so this file proves nothing: %q", header); err != nil {
		return err
	}
	filled := append([]string(nil), header...)

	// reverse flips the way in force, and nothing else.
	out, err = uiDisplayFillRemainderRunCLI(ctx, dir, cliEnv, "show bgp peer list | display state | fill alpha reverse | text")
	if err != nil {
		return err
	}
	reversedHeader, err := tableHeader(out)
	if err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(len(reversedHeader) > 0 && reversedHeader[0] == columnState, "the displayed column must still lead: %q", reversedHeader); err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(uiDisplayFillRemainderEqualStrings(reversedHeader[1:], sortedStrings(reversedHeader[1:], true)), "reverse did not flip the name order: %q", reversedHeader); err != nil {
		return err
	}
	if err := uiDisplayFillRemainderRequire(uiDisplayFillRemainderEqualStringSets(reversedHeader, filled), "reverse changed which columns appear: %q", reversedHeader); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "OK") //nolint:errcheck // progress output
	return nil
}

func zePasswordHash(ctx context.Context, dir, input string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "passwd")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w: %s%s", err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func uiDisplayFillRemainderRunCLI(ctx context.Context, dir string, env []string, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "ze", "cli", "-c", command) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}

	exitCode := -1
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		exitCode = exitErr.ExitCode()
	}
	return "", fmt.Errorf("%s exit=%d: %s%s", command, exitCode, stdout.String(), stderr.String())
}

// tableHeaderMarker names the column that marks the header row of a peer table.
const tableHeaderMarker = "state"

func tableHeader(text string) ([]string, error) {
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.Contains(line, tableHeaderMarker) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && (fields[0] == aliasPeers || fields[0] == fieldPeer) {
			return fields[1:], nil
		}
		return fields, nil
	}
	return nil, fmt.Errorf("no header row carrying %q in: %q", tableHeaderMarker, text)
}

func uiDisplayFillRemainderRequire(condition bool, format string, args ...any) error {
	if condition {
		return nil
	}
	return fmt.Errorf(format, args...)
}

func sortedStrings(values []string, reverse bool) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	if reverse {
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}
	}
	return result
}

func uiDisplayFillRemainderEqualStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func uiDisplayFillRemainderEqualStringSets(left, right []string) bool {
	leftSet := make(map[string]struct{}, len(left))
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range left {
		leftSet[value] = struct{}{}
	}
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	if len(leftSet) != len(rightSet) {
		return false
	}
	for value := range leftSet {
		if _, ok := rightSet[value]; !ok {
			return false
		}
	}
	return true
}

func uiDisplayFillRemainderPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func uiDisplayFillRemainderEnvironmentWith(base []string, overrides map[string]string) []string {
	values := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	maps.Copy(values, overrides)

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

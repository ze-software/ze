package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const peerListColumnOrderFixture = "ui/show-bgp-peer-list-column-order"

var peerListOrder = []string{columnName, columnGroup, columnRemoteAS, columnState, columnUptime}

func init() {
	Register(peerListColumnOrderFixture, uiDriver(runShowBGPPeerListColumnOrder))
}

type uiShowBgpPeerListColumnOrderCommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type synchronizedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type observation struct {
	cmd    *exec.Cmd
	stdout synchronizedBuffer
	stderr synchronizedBuffer
	done   chan struct{}
	err    error
}

type nativeFixture struct{}

// Dispatch runs a compiled product command and captures all of its observable
// process results without involving a command shell.
func (nativeFixture) Dispatch(ctx context.Context, argv, env []string, stdin io.Reader) (uiShowBgpPeerListColumnOrderCommandResult, error) {
	if len(argv) == 0 {
		return uiShowBgpPeerListColumnOrderCommandResult{}, errors.New("cannot dispatch an empty command")
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Env = env
	cmd.Stdin = stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := uiShowBgpPeerListColumnOrderCommandResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if err == nil {
		return result, nil
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

// Observe starts a compiled product process and retains its output and exit
// state so readiness polling can fail immediately if the process exits.
func (nativeFixture) Observe(ctx context.Context, argv, env []string) (*observation, error) {
	if len(argv) == 0 {
		return nil, errors.New("cannot observe an empty command")
	}

	o := &observation{done: make(chan struct{})}
	o.cmd = exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // the fixture chooses the program and its arguments
	o.cmd.Env = env
	o.cmd.Stdout = &o.stdout
	o.cmd.Stderr = &o.stderr
	if err := o.cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		o.err = o.cmd.Wait()
		close(o.done)
	}()
	return o, nil
}

// Poll performs exactly attempts observations, waiting delay only between
// unsuccessful observations.
func (nativeFixture) Poll(ctx context.Context, attempts int, delay time.Duration, observe func() (bool, error)) (bool, error) {
	for attempt := range attempts {
		ready, err := observe()
		if err != nil {
			return false, err
		}
		if ready {
			return true, nil
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

func runShowBGPPeerListColumnOrder(ctx context.Context) (returnErr error) {
	fixture := nativeFixture{}
	baseEnv := os.Environ()

	password, err := fixture.Dispatch(ctx, []string{"ze", "passwd"}, baseEnv, strings.NewReader("secret\n"))
	if err != nil {
		return fmt.Errorf("run ze passwd: %w", err)
	}
	if password.ExitCode != 0 {
		return fmt.Errorf("ze passwd exit=%d: %s%s", password.ExitCode, password.Stdout, password.Stderr)
	}
	passwordHash := strings.TrimSpace(password.Stdout)

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
	cwd, err := os.MkdirTemp("", "ze-ui-show-bgp-peer-list-columns-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(cwd) //nolint:errcheck // fixture cleanup
	configPath := filepath.Join(cwd, "peers.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write peers.conf: %w", err)
	}
	sshAddressFile, err := absolutePath(cwd, "ssh.addr")
	if err != nil {
		return err
	}
	readyFile, err := absolutePath(cwd, "ready")
	if err != nil {
		return err
	}

	// Leave port 179 alone: this suite runs unprivileged and a bind failure
	// there takes the daemon down before it writes the readiness file.
	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := replaceEnvironment(baseEnv,
		environmentValue{envSSHEphemeral, sshAddressFile},
		environmentValue{envReadyFile, readyFile},
		environmentValue{envConfigDir, cwd},
		environmentValue{envTestBGPPort, strconv.Itoa(bgpPort)},
	)

	daemon, err := fixture.Observe(ctx, []string{"ze", "-f", configPath}, daemonEnv)
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer func() {
		if err := stopObservation(daemon, 5*time.Second); err != nil {
			// Cleanup failures supersede an earlier assertion failure, matching a
			// failing mandatory cleanup block.
			returnErr = err
		}
	}()

	ready, err := fixture.Poll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		select {
		case <-daemon.done:
			return false, fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemon.stdout.String(), daemon.stderr.String())
		default:
		}
		return uiShowBgpPeerListColumnOrderPathExists(sshAddressFile) && uiShowBgpPeerListColumnOrderPathExists(readyFile), nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("daemon did not become ready")
	}

	addressBytes, err := os.ReadFile(sshAddressFile) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh address: %w", err)
	}
	address := strings.TrimSpace(string(addressBytes))
	colon := strings.LastIndex(address, ":")
	if colon < 0 {
		return fmt.Errorf("invalid ssh address %q", address)
	}
	host, port := address[:colon], address[colon+1:]

	cliEnv := replaceEnvironment(baseEnv,
		environmentValue{envSSHHost, host},
		environmentValue{envSSHPort, port},
		environmentValue{envSSHUsername, "ci"},
		environmentValue{envSSHPassword, valueSecret},
		environmentValue{envConfigDir, cwd},
	)

	table, err := fixture.Dispatch(ctx, []string{"ze", areaCLI, "-c", "show bgp peer list | table"}, cliEnv, nil)
	if err != nil {
		return fmt.Errorf("run show bgp peer list | table: %w", err)
	}
	if table.ExitCode != 0 {
		return fmt.Errorf("show bgp peer list | table exit=%d: %s%s", table.ExitCode, table.Stdout, table.Stderr)
	}
	if !strings.Contains(table.Stdout, "│") {
		return fmt.Errorf("| table did not answer a table: %q", table.Stdout)
	}

	// remote-as identifies the header row unambiguously: no cell carries it.
	var header []string
	for line := range strings.SplitSeq(table.Stdout, "\n") {
		if strings.Contains(line, "remote-as") {
			header = strings.Fields(strings.ReplaceAll(line, "│", " "))
			break
		}
	}
	if header == nil {
		return fmt.Errorf("no peer table in the answer: %q", table.Stdout)
	}

	expected := make([]string, 0, len(peerListOrder))
	for _, key := range peerListOrder {
		if uiShowBgpPeerListColumnOrderContainsString(header, key) {
			expected = append(expected, key)
		}
	}
	if !uiShowBgpPeerListColumnOrderEqualStrings(header, expected) {
		return fmt.Errorf("peer list columns %q, want %q", header, expected)
	}
	if !uiShowBgpPeerListColumnOrderContainsString(header, "name") || !uiShowBgpPeerListColumnOrderContainsString(header, "group") {
		return fmt.Errorf("the fixture lost the keys the order is about: %q", header)
	}
	alphabetical := append([]string(nil), header...)
	sort.Strings(alphabetical)
	if uiShowBgpPeerListColumnOrderEqualStrings(header, alphabetical) {
		return fmt.Errorf("the peer list columns are still alphabetical: %q", header)
	}

	// A program reads the YAML rendering, so the declared display order must
	// not have reached it.
	yaml, err := fixture.Dispatch(ctx, []string{"ze", areaCLI, "-c", "show bgp peer list | yaml"}, cliEnv, nil)
	if err != nil {
		return fmt.Errorf("run show bgp peer list | yaml: %w", err)
	}
	if yaml.ExitCode != 0 {
		return fmt.Errorf("show bgp peer list | yaml exit=%d: %s%s", yaml.ExitCode, yaml.Stdout, yaml.Stderr)
	}
	if strings.Contains(yaml.Stdout, "│") {
		return fmt.Errorf("| yaml answered a table: %q", yaml.Stdout)
	}
	groupAt := strings.Index(yaml.Stdout, "group:")
	nameAt := strings.Index(yaml.Stdout, "name:")
	if groupAt < 0 || nameAt < 0 {
		return fmt.Errorf("| yaml lost the keys the order is about: %q", yaml.Stdout)
	}
	if groupAt >= nameAt {
		return fmt.Errorf("| yaml took the declared order instead of its alphabetical keys: %q", yaml.Stdout)
	}

	fmt.Println("OK")
	return nil
}

func absolutePath(cwd, name string) (string, error) {
	path := filepath.Join(cwd, name)
	absolute, err := os.Stat(cwd)
	if err != nil {
		return "", fmt.Errorf("stat working directory: %w", err)
	}
	if !absolute.IsDir() {
		return "", fmt.Errorf("working directory %q is not a directory", cwd)
	}
	return path, nil
}

func uiShowBgpPeerListColumnOrderPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type environmentValue struct {
	key   string
	value string
}

func replaceEnvironment(base []string, values ...environmentValue) []string {
	replacements := make(map[string]string, len(values))
	for _, value := range values {
		replacements[value.key] = value.value
	}

	env := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key := entry
		if before, _, found := strings.Cut(entry, "="); found {
			key = before
		}
		if _, replace := replacements[key]; !replace {
			env = append(env, entry)
		}
	}
	for _, value := range values {
		env = append(env, value.key+"="+value.value)
	}
	return env
}

func uiShowBgpPeerListColumnOrderContainsString(values []string, wanted string) bool {
	return slices.Contains(values, wanted)
}

func uiShowBgpPeerListColumnOrderEqualStrings(left, right []string) bool {
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

func stopObservation(o *observation, timeout time.Duration) error {
	select {
	case <-o.done:
		return nil
	default:
	}

	if err := o.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate daemon: %w", err)
	}
	if waitForObservation(o, timeout) {
		return nil
	}

	if err := o.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill daemon: %w", err)
	}
	if !waitForObservation(o, timeout) {
		return errors.New("daemon did not exit within 5s after being killed")
	}
	return nil
}

func waitForObservation(o *observation, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-o.done:
		return true
	case <-timer.C:
		return false
	}
}

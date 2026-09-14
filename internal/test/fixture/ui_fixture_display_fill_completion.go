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
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const readQuantum = 100 * time.Millisecond

var displayFillANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b[()][A-Z0-9]|\x1b[=>]`)

func init() {
	Register("ui/display-fill-completion", uiDriver(displayFillCompletion))
}

type displayFillProcess struct {
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error
}

func displayFillCompletion(ctx context.Context) error {
	var hash bytes.Buffer
	passwd := exec.CommandContext(ctx, "ze", "passwd")
	passwd.Stdin = strings.NewReader("secret\n")
	passwd.Stdout = &hash
	passwd.Stderr = os.Stderr
	if err := passwd.Run(); err != nil {
		return fmt.Errorf("ze passwd: %w", err)
	}
	passwordHash := strings.TrimSpace(hash.String())

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
	work, err := os.MkdirTemp("", "ze-ui-display-fill-completion-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	configPath := filepath.Join(work, "peers.conf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write peers.conf: %w", err)
	}

	sshAddr := filepath.Join(work, "ssh.addr")
	readyFile := filepath.Join(work, "ready")

	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := displayFillEnvironment(os.Environ(), map[string]string{
		envSSHEphemeral: sshAddr,
		envReadyFile:    readyFile,
		envConfigDir:    work,
		envTestBGPPort:  strconv.Itoa(bgpPort),
	})

	var daemonStdout, daemonStderr bytes.Buffer
	daemonCmd := exec.CommandContext(ctx, "ze", "-f", configPath) //nolint:gosec // the fixture chooses the program and its arguments
	daemonCmd.Dir = work
	daemonCmd.Stdin = os.Stdin
	daemonCmd.Stdout = &daemonStdout
	daemonCmd.Stderr = &daemonStderr
	daemonCmd.Env = daemonEnv
	daemon, err := displayFillStart(daemonCmd)
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	defer displayFillStop(daemon, syscall.SIGTERM, 5*time.Second)

	ready, err := displayFillPoll(ctx, 200, 100*time.Millisecond, func() (bool, error) {
		if displayFillExited(daemon) {
			return false, fmt.Errorf("daemon exited early\nstdout:\n%s\nstderr:\n%s", daemonStdout.String(), daemonStderr.String())
		}
		return displayFillExists(sshAddr) && displayFillExists(readyFile), nil
	})
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("daemon did not become ready")
	}

	addrBytes, err := os.ReadFile(sshAddr) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("read ssh.addr: %w", err)
	}
	addr := strings.TrimSpace(string(addrBytes))
	colon := strings.LastIndexByte(addr, ':')
	if colon < 0 {
		return fmt.Errorf("invalid SSH address %q", addr)
	}
	host, port := addr[:colon], addr[colon+1:]

	cliEnv := displayFillEnvironment(os.Environ(), map[string]string{
		envSSHHost:     host,
		envSSHPort:     port,
		envSSHUsername: "ci",
		envSSHPassword: valueSecret,
		envConfigDir:   work,
		envTerm:        "xterm",
		envNoColor:     "1",
	})

	// `show bgp peer list` declares name, group, remote-as, state and uptime.
	first, err := displayFillTabAfter(ctx, cliEnv, "show bgp peer list | display ")
	if err != nil {
		return err
	}
	for _, name := range []string{columnName, columnGroup, columnRemoteAS, columnState, columnUptime} {
		if !strings.Contains(first, name) {
			return fmt.Errorf("tab after `| display ` did not offer %q:\n%s", name, first)
		}
	}

	// The match is on the last token typed, so a second field completes too,
	// and the one already typed is gone from the list.
	second, err := displayFillTabAfter(ctx, cliEnv, "show bgp peer list | display state ")
	if err != nil {
		return err
	}
	for _, name := range []string{columnName, columnGroup, columnRemoteAS, columnUptime} {
		if !strings.Contains(second, name) {
			return fmt.Errorf("tab after a first field did not offer %q:\n%s", name, second)
		}
	}

	// `| fill` completes its keywords and never a field name. `overall` was
	// removed because it required buffering every rendered cell before the
	// first row could be written; it must not be offered.
	ways, err := displayFillTabAfter(ctx, cliEnv, "show bgp peer list | fill ")
	if err != nil {
		return err
	}
	for _, word := range []string{"alpha", "reverse"} {
		if !strings.Contains(ways, word) {
			return fmt.Errorf("tab after `| fill ` did not offer %q:\n%s", word, ways)
		}
	}
	if strings.Contains(ways, "overall") {
		return fmt.Errorf("`| fill` still offers the removed `overall`:\n%s", ways)
	}
	for _, name := range []string{columnRemoteAS, columnUptime} {
		if strings.Contains(ways, name) {
			return fmt.Errorf("`| fill` offered the field name %q:\n%s", name, ways)
		}
	}

	fmt.Println("OK")
	return nil
}

func displayFillTabAfter(ctx context.Context, env []string, text string) (string, error) {
	master, slave, err := displayFillOpenPTY(50, 200)
	if err != nil {
		return "", fmt.Errorf("open pseudo-terminal: %w", err)
	}
	defer master.Close() //nolint:errcheck // fixture teardown

	clientCmd := exec.CommandContext(ctx, "ze", "cli")
	clientCmd.Stdin = slave
	clientCmd.Stdout = slave
	clientCmd.Stderr = slave
	clientCmd.Env = env
	clientCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	client, err := displayFillStart(clientCmd)
	closeErr := slave.Close()
	if err != nil {
		return "", fmt.Errorf("start ze cli: %w", err)
	}
	defer displayFillStop(client, syscall.SIGINT, 3*time.Second)
	if closeErr != nil {
		return "", fmt.Errorf("close pseudo-terminal slave: %w", closeErr)
	}

	if _, err := displayFillReadAvailable(ctx, master, time.Now().Add(10*time.Second)); err != nil {
		return "", err
	}
	if err := displayFillWrite(ctx, master, []byte(text)); err != nil {
		return "", fmt.Errorf("write command to ze cli: %w", err)
	}
	if _, err := displayFillReadAvailable(ctx, master, time.Now().Add(5*time.Second)); err != nil {
		return "", err
	}
	if err := displayFillWrite(ctx, master, []byte{'\t'}); err != nil {
		return "", fmt.Errorf("write Tab to ze cli: %w", err)
	}
	screen, err := displayFillReadAvailable(ctx, master, time.Now().Add(10*time.Second))
	if err != nil {
		return "", err
	}
	return displayFillANSI.ReplaceAllString(screen, ""), nil
}

func displayFillStart(cmd *exec.Cmd) (*displayFillProcess, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &displayFillProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

func displayFillExited(p *displayFillProcess) bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func displayFillStop(p *displayFillProcess, firstSignal os.Signal, timeout time.Duration) {
	if displayFillExited(p) {
		return
	}
	_ = p.cmd.Process.Signal(firstSignal)
	if displayFillWait(p, timeout) {
		return
	}
	_ = p.cmd.Process.Kill()
	_ = displayFillWait(p, timeout)
}

func displayFillWait(p *displayFillProcess, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return true
	case <-timer.C:
		return false
	}
}

func displayFillPoll(ctx context.Context, attempts int, delay time.Duration, check func() (bool, error)) (bool, error) {
	for range attempts {
		ok, err := check()
		if err != nil || ok {
			return ok, err
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

func displayFillExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func displayFillEnvironment(base []string, updates map[string]string) []string {
	env := make([]string, 0, len(base)+len(updates))
	for _, entry := range base {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replace := updates[name]; replace {
				continue
			}
		}
		env = append(env, entry)
	}
	for _, name := range []string{
		envSSHEphemeral, envReadyFile, envConfigDir, envTestBGPPort,
		envSSHHost, envSSHPort, envSSHUsername, envSSHPassword, envTerm, envNoColor,
	} {
		if value, ok := updates[name]; ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// displayFillOpenPTY allocates a pseudo-terminal pair and sizes it.
//
// The allocation goes through creack/pty because the ioctl numbers that do it
// are per-kernel. This function open-coded Linux's: TIOCSPTLCK 0x40045431,
// TIOCGPTN 0x80045430 and TIOCSWINSZ 0x5414, with the slave named
// /dev/pts/<n>. macOS grants and names its slave through TIOCPTYGRANT,
// TIOCPTYUNLK and TIOCPTYGNAME instead, so the first ioctl there returned
// ENOTTY and this whole test failed with "inappropriate ioctl for device" on
// every darwin host. Nothing it asserts is Linux-only: the completion it reads
// comes from the column registry, so the fixture owed a portable terminal
// rather than a platform gate.
func displayFillOpenPTY(rows, cols uint16) (*os.File, *os.File, error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, nil, err
	}
	fail := func(err error) (*os.File, *os.File, error) {
		_ = slave.Close()
		_ = master.Close()
		return nil, nil, err
	}
	if err := pty.Setsize(master, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
		return fail(err)
	}
	// displayFillReadAvailable polls with a deadline of its own and reads
	// EAGAIN as "nothing more yet", so the master has to answer that rather
	// than park inside the runtime. Fd releases the file from Go's poller and
	// hands back the descriptor SetNonblock then marks.
	if err := syscall.SetNonblock(int(master.Fd()), true); err != nil {
		return fail(err)
	}
	return master, slave, nil
}

func displayFillReadAvailable(ctx context.Context, master *os.File, deadline time.Time) (string, error) {
	var result bytes.Buffer
	buf := make([]byte, 65536)
	var lastRead time.Time
	for time.Now().Before(deadline) {
		n, err := master.Read(buf)
		if n > 0 {
			_, _ = result.Write(buf[:n])
			lastRead = time.Now()
		}
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
				if !lastRead.IsZero() && time.Since(lastRead) >= 100*time.Millisecond {
					break
				}
				if err := displayFillPause(ctx, deadline); err != nil {
					return "", err
				}
				continue
			}
			// EOF and EIO are both normal when the slave side closes. The
			// original helper treated every read-side OS error as end of input.
			break
		}
		if n == 0 {
			if !lastRead.IsZero() && time.Since(lastRead) >= 100*time.Millisecond {
				break
			}
			if err := displayFillPause(ctx, deadline); err != nil {
				return "", err
			}
		}
	}
	return strings.ToValidUTF8(result.String(), "\uFFFD"), nil
}

func displayFillPause(ctx context.Context, deadline time.Time) error {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return nil
	}
	if remaining > readQuantum {
		remaining = readQuantum
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func displayFillWrite(ctx context.Context, master *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := master.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err == nil {
			continue
		}
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			timer := time.NewTimer(readQuantum)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()
			case <-timer.C:
				continue
			}
		}
		return err
	}
	return nil
}

var _ io.Reader = (*os.File)(nil)

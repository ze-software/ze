package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	tioCGPTN    = uintptr(0x80045430)
	tioCSPTLCK  = uintptr(0x40045431)
	tioCSWINSZ  = uintptr(0x5414)
	readQuantum = 100 * time.Millisecond
)

var displayFillANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b[()][A-Z0-9]|\x1b[=>]`)

func init() {
	Register("ui/display-fill-completion", uiDriver(displayFillCompletion))
}

type displayFillProcess struct {
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error
}

type displayFillWinsize struct {
	rows   uint16
	cols   uint16
	xpixel uint16
	ypixel uint16
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
	defer os.RemoveAll(work)
	configPath := work + string(os.PathSeparator) + "peers.conf"
	if err := os.WriteFile(configPath, []byte(config), 0o666); err != nil {
		return fmt.Errorf("write peers.conf: %w", err)
	}

	sshAddr := work + string(os.PathSeparator) + "ssh.addr"
	readyFile := work + string(os.PathSeparator) + "ready"

	bgpPort, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	daemonEnv := displayFillEnvironment(os.Environ(), map[string]string{
		"ZE_SSH_EPHEMERAL": sshAddr,
		"ZE_READY_FILE":    readyFile,
		"ZE_CONFIG_DIR":    work,
		"ze_test_bgp_port": strconv.Itoa(bgpPort),
	})

	var daemonStdout, daemonStderr bytes.Buffer
	daemonCmd := exec.CommandContext(ctx, "ze", "-f", configPath)
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

	addrBytes, err := os.ReadFile(sshAddr)
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
		"ZE_SSH_HOST":     host,
		"ZE_SSH_PORT":     port,
		"ZE_SSH_USERNAME": "ci",
		"ZE_SSH_PASSWORD": "secret",
		"ZE_CONFIG_DIR":   work,
		"TERM":            "xterm",
		"NO_COLOR":        "1",
	})

	// `show bgp peer list` declares name, group, remote-as, state and uptime.
	first, err := displayFillTabAfter(ctx, cliEnv, "show bgp peer list | display ")
	if err != nil {
		return err
	}
	for _, name := range []string{"name", "group", "remote-as", "state", "uptime"} {
		if !strings.Contains(first, name) {
			return fmt.Errorf("Tab after `| display ` did not offer %q:\n%s", name, first)
		}
	}

	// The match is on the last token typed, so a second field completes too,
	// and the one already typed is gone from the list.
	second, err := displayFillTabAfter(ctx, cliEnv, "show bgp peer list | display state ")
	if err != nil {
		return err
	}
	for _, name := range []string{"name", "group", "remote-as", "uptime"} {
		if !strings.Contains(second, name) {
			return fmt.Errorf("Tab after a first field did not offer %q:\n%s", name, second)
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
			return fmt.Errorf("Tab after `| fill ` did not offer %q:\n%s", word, ways)
		}
	}
	if strings.Contains(ways, "overall") {
		return fmt.Errorf("`| fill` still offers the removed `overall`:\n%s", ways)
	}
	for _, name := range []string{"remote-as", "uptime"} {
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
	defer master.Close()

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
	for i := 0; i < attempts; i++ {
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
		"ZE_SSH_EPHEMERAL", "ZE_READY_FILE", "ZE_CONFIG_DIR", "ze_test_bgp_port",
		"ZE_SSH_HOST", "ZE_SSH_PORT", "ZE_SSH_USERNAME", "ZE_SSH_PASSWORD", "TERM", "NO_COLOR",
	} {
		if value, ok := updates[name]; ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func displayFillOpenPTY(rows, cols uint16) (*os.File, *os.File, error) {
	masterFD, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	master := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	fail := func(err error) (*os.File, *os.File, error) {
		_ = master.Close()
		return nil, nil, err
	}

	var unlock int32
	if err := displayFillIOCTL(masterFD, tioCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		return fail(err)
	}
	var number uint32
	if err := displayFillIOCTL(masterFD, tioCGPTN, unsafe.Pointer(&number)); err != nil {
		return fail(err)
	}

	slaveName := "/dev/pts/" + strconv.FormatUint(uint64(number), 10)
	slaveFD, err := syscall.Open(slaveName, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fail(err)
	}
	slave := os.NewFile(uintptr(slaveFD), slaveName)

	winsize := displayFillWinsize{rows: rows, cols: cols}
	if err := displayFillIOCTL(slaveFD, tioCSWINSZ, unsafe.Pointer(&winsize)); err != nil {
		_ = slave.Close()
		return fail(err)
	}
	if err := syscall.SetNonblock(masterFD, true); err != nil {
		_ = slave.Close()
		return fail(err)
	}
	return master, slave, nil
}

func displayFillIOCTL(fd int, request uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
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

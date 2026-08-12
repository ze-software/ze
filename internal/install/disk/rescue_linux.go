// Design: docs/architecture/appliance/installer-initrd.md -- Go recovery console + three-branch fatal

//go:build linux && ze_installer

package disk

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/rescueauth"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var retryMu sync.Mutex

type fatalBranch int

const (
	branchGated   fatalBranch = iota // credential present: password-gated console
	branchUngated                    // no credential + ISO: ungated console
	branchReboot                     // no credential + network: 30s reboot
)

const rescueMaxAttempts = 3

func branchName(b fatalBranch) string {
	switch b {
	case branchGated:
		return "gated"
	case branchUngated:
		return "ungated"
	case branchReboot:
		return "reboot"
	default:
		return "unknown"
	}
}

func selectFatalBranch(rescueAuth, source string) fatalBranch {
	if rescueAuth == "" {
		// No credential configured. On ISO media the operator is physically
		// present, so a shell costs nothing they could not already do; on a
		// network install reboot instead, so an unattended box retries rather
		// than sitting at a console nobody is watching.
		if source == sourceISO {
			return branchUngated
		}
		return branchReboot
	}
	// A credential the gate cannot verify against is not a credential.
	// rescueauth.Check fails closed on a malformed value, so gating on one would
	// prompt for a token nothing can satisfy and hang an unattended install
	// forever. Falling through to the no-credential policy is wrong too: that
	// would open an UNGATED shell on ISO media off a single typo. Reboot on
	// either medium, so a bad credential never yields a shell and never hangs.
	if rescueauth.Validate(rescueAuth) != nil {
		return branchReboot
	}
	return branchGated
}

// fatalInitrd implements the three-branch rescue policy from
// tools/installer-initrd/init:217-227 and never returns.
func fatalInitrd(cfg installConfig, msg string) {
	slog.Error("FATAL", "error", msg)

	branch := selectFatalBranch(cfg.RescueAuth, cfg.Source)
	slog.Info("fatal policy", "branch", branchName(branch), "source", cfg.Source, "auth-set", cfg.RescueAuth != "")
	switch branch {
	case branchGated:
		slog.Info("enter the rescue token on any console for a rescue shell")
		rescueOnConsoles(cfg, true)
	case branchUngated:
		slog.Info("dropping to rescue console for debugging")
		rescueOnConsoles(cfg, false)
	case branchReboot:
		slog.Info("no rescue credential configured; rebooting in 30s")
		time.Sleep(30 * time.Second)
	}

	if cfg.Source == sourceISO {
		slog.Info("powering off")
		unix.Sync()
		_ = unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	} else {
		slog.Info("rebooting")
		unix.Sync()
		_ = unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
	}
	select {}
}

func rescueOnConsoles(cfg installConfig, gated bool) {
	data, err := os.ReadFile("/sys/class/tty/console/active")
	if err != nil {
		rescueSession(os.Stdin, os.Stdout, cfg, gated)
		return
	}

	type result struct{}
	done := make(chan result)

	started := 0
	var tb textbuf.Buffer
	for name := range strings.FieldsSeq(strings.TrimSpace(string(data))) {
		path := tb.Reset().Str("/dev/").Str(name).String()
		f, openErr := os.OpenFile(path, os.O_RDWR, 0)
		if openErr != nil {
			continue
		}
		started++
		go func(con *os.File) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("rescue console panic", "console", con.Name(), "panic", r)
				}
				done <- result{}
			}()
			rescueSession(con, con, cfg, gated)
		}(f)
	}

	if started == 0 {
		rescueSession(os.Stdin, os.Stdout, cfg, gated)
		return
	}
	for range started {
		<-done
	}
}

func rescueSession(r io.Reader, w io.Writer, cfg installConfig, gated bool) {
	if gated {
		if !gateWithRescueToken(r, w, cfg.RescueAuth) {
			return
		}
	}
	rescueMenu(r, w, cfg)
}

func gateWithRescueToken(r io.Reader, w io.Writer, rescueAuth string) bool {
	if f, ok := r.(*os.File); ok {
		echoOff(f)
		defer echoOn(f)
	}

	// Scan returns false on EOF, on a read error, and on an over-long line
	// alike, and each one leaves the caller with false: no token, no rescue
	// console. A truncated token fails rescueauth.Check for the same reason.
	scanner := bufio.NewScanner(r)
	for attempt := range rescueMaxAttempts {
		fmt.Fprint(w, "[ze-install] rescue token: ") //nolint:errcheck // console output to recovery terminal
		if !scanner.Scan() {
			return false
		}
		pw := scanner.Text()
		if rescueauth.Check(pw, rescueAuth) {
			fmt.Fprintln(w, "\n[ze-install] authenticated") //nolint:errcheck // console output to recovery terminal
			return true
		}
		fmt.Fprintln(w, "\n[ze-install] incorrect") //nolint:errcheck // console output to recovery terminal
		_ = attempt
	}
	fmt.Fprintln(w, "[ze-install] too many attempts") //nolint:errcheck // console output to recovery terminal
	return false
}

func echoOff(f *os.File) {
	fd := int(f.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	termios.Lflag &^= unix.ECHO
	unix.IoctlSetTermios(fd, unix.TCSETS, termios) //nolint:errcheck // best-effort terminal control
}

func echoOn(f *os.File) {
	fd := int(f.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	termios.Lflag |= unix.ECHO
	unix.IoctlSetTermios(fd, unix.TCSETS, termios) //nolint:errcheck // best-effort terminal control
}

func rescueMenu(r io.Reader, w io.Writer, _ installConfig) {
	fmt.Fprintln(w, "\n[ze-install] Recovery Console") //nolint:errcheck // console output to recovery terminal
	fmt.Fprintln(w, "  1) Retry network + install")    //nolint:errcheck // console output to recovery terminal
	fmt.Fprintln(w, "  2) Show network state")         //nolint:errcheck // console output to recovery terminal
	fmt.Fprintln(w, "  3) Reboot")                     //nolint:errcheck // console output to recovery terminal
	fmt.Fprintln(w, "  4) Power off")                  //nolint:errcheck // console output to recovery terminal

	// A read failure ends the menu, same as EOF. A truncated line matches no
	// case in the switch, so no reboot or reinstall runs on half a command.
	scanner := bufio.NewScanner(r)
	for {
		fmt.Fprint(w, "\nze> ") //nolint:errcheck // console output to recovery terminal
		if !scanner.Scan() {
			return
		}
		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			if !retryMu.TryLock() {
				fmt.Fprintln(w, "[ze-install] retry already in progress on another console") //nolint:errcheck // console output to recovery terminal
				continue
			}
			fmt.Fprintln(w, "[ze-install] retrying...") //nolint:errcheck // console output to recovery terminal
			code := Run(nil)
			retryMu.Unlock()
			if code == 0 {
				return
			}
			fmt.Fprintln(w, "[ze-install] retry failed") //nolint:errcheck // console output to recovery terminal
		case "2":
			showNetworkState(w)
		case "3":
			unix.Sync()
			_ = unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
		case "4":
			unix.Sync()
			_ = unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
		default:
			fmt.Fprintln(w, "unknown option") //nolint:errcheck // console output to recovery terminal
		}
	}
}

func showNetworkState(w io.Writer) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		fmt.Fprintln(w, "cannot read /sys/class/net") //nolint:errcheck // console output to recovery terminal
		return
	}
	var tb textbuf.Buffer
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		carrier := readSysfs(tb.Reset().Str("/sys/class/net/").Str(name).Str("/carrier").String())
		oper := readSysfs(tb.Reset().Str("/sys/class/net/").Str(name).Str("/operstate").String())
		addr := readSysfs(tb.Reset().Str("/sys/class/net/").Str(name).Str("/address").String())
		line := tb.Reset().Str("  ").Str(name).Str(": carrier=").Str(carrier).Str(" oper=").Str(oper).Str(" mac=").Str(addr).String()
		fmt.Fprintln(w, line) //nolint:errcheck // console output to recovery terminal
	}
}

func readSysfs(path string) string {
	data, err := os.ReadFile(path) //nolint:gosec // sysfs path
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}

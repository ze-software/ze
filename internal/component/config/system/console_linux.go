// Design: docs/architecture/config/syntax.md — console serial apply (linux termios)

//go:build linux

package system

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var baudRates = map[int]uint32{
	9600:   unix.B9600,
	19200:  unix.B19200,
	38400:  unix.B38400,
	57600:  unix.B57600,
	115200: unix.B115200,
}

// ConsoleResult reports what the console apply engine did.
type ConsoleResult struct {
	Applied []string
	Skipped []ConsoleSkip
	Errors  []ConsoleError
}

// ConsoleSkip records a device that was intentionally skipped.
type ConsoleSkip struct {
	Device string
	Reason string
}

// ConsoleError records a failed console apply operation.
type ConsoleError struct {
	Device string
	Err    error
}

// ApplyConsole configures serial console devices via termios.
// On systemd hosts, skips devices with an active serial-getty.
func ApplyConsole(devices []ConsoleDeviceEntry) ConsoleResult {
	var result ConsoleResult

	systemctl, _ := exec.LookPath("systemctl")

	for _, dev := range devices {
		if !ValidConsoleDeviceName(dev.Name) {
			result.Skipped = append(result.Skipped, ConsoleSkip{
				Device: dev.Name,
				Reason: "invalid device name",
			})
			continue
		}

		if gettyActive(systemctl, dev.Name) {
			result.Skipped = append(result.Skipped, ConsoleSkip{
				Device: dev.Name,
				Reason: "serial-getty@" + dev.Name + ".service is active",
			})
			continue
		}

		devPath := "/dev/" + dev.Name
		if err := applyTermios(devPath, dev.Speed); err != nil {
			result.Errors = append(result.Errors, ConsoleError{
				Device: dev.Name,
				Err:    err,
			})
			continue
		}

		var bApplied textbuf.Buffer
		result.Applied = append(result.Applied, bApplied.Reset().Str(dev.Name).Str(" at ").Int(int64(dev.Speed)).String())
	}

	return result
}

func applyTermios(path string, speed int) error {
	baud, ok := baudRates[speed]
	if !ok {
		return fmt.Errorf("unsupported baud rate %d", speed)
	}

	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if cerr := unix.Close(fd); cerr != nil {
			slog.Warn("close serial fd failed", "path", path, "error", cerr)
		}
	}()

	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return fmt.Errorf("tcgets %s: %w", path, err)
	}

	// Raw mode: 8N1, no echo, no signal handling, no flow control
	termios.Cflag &^= unix.CBAUD | unix.CSIZE | unix.PARENB | unix.CSTOPB
	termios.Cflag |= baud | unix.CS8 | unix.CLOCAL | unix.CREAD
	termios.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Iflag &^= unix.IXON | unix.IXOFF | unix.ICRNL
	termios.Oflag &^= unix.OPOST

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, termios); err != nil {
		return fmt.Errorf("tcsets %s: %w", path, err)
	}

	return nil
}

// gettyActive checks whether serial-getty@<device>.service is active.
// systemctl is the resolved path from LookPath (empty means not found).
// Returns false if systemctl is absent (gokrazy) or the check fails.
func gettyActive(systemctl, device string) bool {
	if systemctl == "" {
		return false
	}

	serviceName := "serial-getty@" + device + ".service"
	cmd := exec.CommandContext(context.Background(), systemctl, "is-active", "--quiet", serviceName) //nolint:gosec // systemctl is LookPath-resolved; literal args plus a service name, run without a shell
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Env = append(os.Environ(), "LANG=C")

	return cmd.Run() == nil
}

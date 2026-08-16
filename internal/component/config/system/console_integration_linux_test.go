// Design: docs/architecture/config/syntax.md — console serial integration tests

//go:build integration && linux

package system

import (
	"testing"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func openPTY(t *testing.T) (slavePath string) {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("cannot open pty: %v", err)
	}
	t.Cleanup(func() {
		master.Close() //nolint:errcheck // best-effort cleanup
		slave.Close()  //nolint:errcheck // best-effort cleanup
	})
	return slave.Name()
}

func TestIntegrationApplyTermios_BaudRate(t *testing.T) {
	slavePath := openPTY(t)

	err := applyTermios(slavePath, 9600)
	require.NoError(t, err)

	fd, err := unix.Open(slavePath, unix.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	defer unix.Close(fd) //nolint:errcheck // best-effort cleanup

	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	require.NoError(t, err)

	baud := termios.Cflag & unix.CBAUD
	assert.Equal(t, uint32(unix.B9600), baud, "baud rate should be B9600")

	assert.True(t, termios.Cflag&unix.CS8 != 0, "CS8 should be set")
	assert.True(t, termios.Cflag&unix.PARENB == 0, "PARENB should be clear")
	assert.True(t, termios.Cflag&unix.CSTOPB == 0, "CSTOPB should be clear")
	assert.True(t, termios.Cflag&unix.CLOCAL != 0, "CLOCAL should be set")
	assert.True(t, termios.Cflag&unix.CREAD != 0, "CREAD should be set")
}

func TestIntegrationApplyTermios_RawMode(t *testing.T) {
	slavePath := openPTY(t)

	err := applyTermios(slavePath, 115200)
	require.NoError(t, err)

	fd, err := unix.Open(slavePath, unix.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	defer unix.Close(fd) //nolint:errcheck // best-effort cleanup

	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	require.NoError(t, err)

	assert.True(t, termios.Lflag&unix.ECHO == 0, "ECHO should be clear")
	assert.True(t, termios.Lflag&unix.ICANON == 0, "ICANON should be clear")
	assert.True(t, termios.Lflag&unix.ISIG == 0, "ISIG should be clear")
	assert.True(t, termios.Lflag&unix.IEXTEN == 0, "IEXTEN should be clear")
	assert.True(t, termios.Iflag&unix.IXON == 0, "IXON should be clear")
	assert.True(t, termios.Iflag&unix.IXOFF == 0, "IXOFF should be clear")
	assert.True(t, termios.Iflag&unix.ICRNL == 0, "ICRNL should be clear")
	assert.True(t, termios.Oflag&unix.OPOST == 0, "OPOST should be clear")
}

func TestIntegrationApplyTermios_SpeedChange(t *testing.T) {
	slavePath := openPTY(t)

	require.NoError(t, applyTermios(slavePath, 9600))
	require.NoError(t, applyTermios(slavePath, 115200))

	fd, err := unix.Open(slavePath, unix.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	defer unix.Close(fd) //nolint:errcheck // best-effort cleanup

	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	require.NoError(t, err)

	baud := termios.Cflag & unix.CBAUD
	assert.Equal(t, uint32(unix.B115200), baud, "baud rate should be B115200 after reconfigure")
}

func TestIntegrationApplyTermios_BadPath(t *testing.T) {
	err := applyTermios("/dev/nonexistent-console-test-device", 115200)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "open")
}

func TestIntegrationGettyActive_NoSystemd(t *testing.T) {
	active := gettyActive("", "ttyS0")
	assert.False(t, active, "empty systemctl path should return false")
}

func TestIntegrationApplyConsole_InvalidDevice(t *testing.T) {
	result := ApplyConsole([]ConsoleDeviceEntry{
		{Name: "nonexistent-ze-test-serial", Speed: 115200},
	})
	assert.Empty(t, result.Applied)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "nonexistent-ze-test-serial", result.Errors[0].Device)
}

// Design: docs/guide/config-editor.md — live clients use the owning daemon's editor
package sshclient

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// Interactive session request fields shared by the terminal client and server.
const (
	EnvConfigName  = "ZE_CONFIG_NAME"
	EnvCLIMode     = "ZE_CLI_MODE"
	CLIModeEdit    = "edit"
	CLIModeCommand = "command"
)

// RunInteractive runs the daemon's model over a persistent SSH PTY. The daemon
// owns drafts and command history; this client only transports terminal bytes.
// It restores the terminal and joins its resize worker before returning.
func RunInteractive(creds Credentials, configName string, commandMode bool) error {
	connection, err := dialDaemon(creds)
	if err != nil {
		return err
	}
	defer connection.Close() //nolint:errcheck // Session error takes precedence.
	session, err := connection.NewSession()
	if err != nil {
		return err
	}
	defer session.Close() //nolint:errcheck // Session error takes precedence.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open interactive terminal: %w", err)
	}
	defer tty.Close() //nolint:errcheck // Terminal restoration happens first.
	fd := int(tty.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		return err
	}
	if err := session.RequestPty("xterm-256color", height, width, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
		return err
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, state) //nolint:errcheck // Best effort terminal restoration.
	session.Stdin = tty
	session.Stdout = tty
	// RFC 4254 section 6.4 sends channel environment requests before the shell.
	// These two application fields select the daemon model; they never set the
	// daemon process environment. The SSH library encodes their string framing.
	if configName != "" {
		if filepath.Base(configName) != configName || configName == "." || configName == ".." {
			return fmt.Errorf("invalid config name %q: a basename is required", configName)
		}
		if err := session.Setenv(EnvConfigName, configName); err != nil {
			return err
		}
	}
	mode := CLIModeEdit
	if commandMode {
		mode = CLIModeCommand
	}
	if err := session.Setenv(EnvCLIMode, mode); err != nil {
		return err
	}
	session.Stderr = tty
	if err := session.Shell(); err != nil {
		return err
	}
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	defer signal.Stop(changes)
	stop := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		// This worker lives exactly as long as this terminal session.
		for {
			select {
			case <-stop:
				return
			case <-changes:
				width, height, err := term.GetSize(fd)
				if err == nil {
					session.WindowChange(height, width) //nolint:errcheck // A disconnected session is reported by Wait.
				}
			}
		}
	}()
	err = session.Wait()
	close(stop)
	<-joined
	return err
}

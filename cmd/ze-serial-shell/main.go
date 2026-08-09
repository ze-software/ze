// Design: docs/architecture/appliance-serial-login.md -- gokrazy serial shell wrapper
//
// Replaces github.com/gokrazy/serial-busybox. Creates the symlink at
// /tmp/serial-busybox/ash pointing to the ze binary (/user/ze) so that
// gokrazy's tryStartShell execs ze for serial console access. Ze then
// handles authentication before granting the shell.
//
// The renamed busybox binary (ze-recovery-shell) is deployed via extrafiles
// tarballs in _gokrazy/. Only ze knows the renamed path.

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	wellKnownSerialShell = "/tmp/serial-busybox/ash"
	zeBinaryPath         = "/user/ze"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ze-serial-shell: %v\n", err) //nolint:errcheck // startup error
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll(filepath.Dir(wellKnownSerialShell), 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	os.Remove(wellKnownSerialShell) //nolint:errcheck // idempotent; may not exist
	if err := os.Symlink(zeBinaryPath, wellKnownSerialShell); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}
	if os.Getenv("GOKRAZY_FIRST_START") == "1" {
		os.Exit(125)
	}
	return nil
}

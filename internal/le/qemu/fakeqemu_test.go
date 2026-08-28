// Overview: run_test.go -- TestBootTimeoutCleansUpTheQEMUProcess re-execs this
// test binary as the fake qemu-system-x86_64 process it drives.
package qemu

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

// init recognizes when this test binary has been re-exec'd as the fake
// qemu-system-x86_64 process TestBootTimeoutCleansUpTheQEMUProcess drives.
// exec.Command keeps the invoked name as argv[0] regardless of the file it
// resolves to (os/exec.Cmd.Args), so a symlink of this same test binary named
// "qemu-system-x86_64" on PATH reaches this branch instead of running the
// test suite. Follows the self-reexec idiom in
// internal/test/fixture/install_fixture.go.
func init() {
	if filepath.Base(os.Args[0]) == "qemu-system-x86_64" {
		os.Exit(fakeQEMUProcess())
	}
}

// fakeQEMUProcess stands in for a QEMU that never reaches a login prompt, so
// the caller's boot timeout fires and stopVM signals it. It installs its
// signal handler as its first action -- microseconds after exec, with no
// shell to fork or script to parse -- so it is always ready for the SIGINT
// stopVM sends. The /bin/sh fixture it replaces instead had to fork, exec,
// and parse a script before its trap took effect, and lost that race against
// the same signal under full-suite parallel load.
func fakeQEMUProcess() int {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	if marker := os.Getenv("MARKER"); marker != "" {
		os.WriteFile(marker, []byte("stopped"), 0o600) //nolint:errcheck // best-effort marker, matching the shell fixture it replaces
	}
	return 0
}

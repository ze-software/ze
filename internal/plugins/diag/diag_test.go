package diag

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// VALIDATES: RunWgKeypair (diag.go) runs `wg genkey`, pipes THAT private key
//            into `wg pubkey` on stdin, prints both keys, and refuses arguments.
// PREVENTS:  the success path going unexecuted. It used to be guarded by a
//            LookPath("wg") skip, and `wg` is installed on no build host here, so
//            the only path the command exists for was never run. The `wg` fixture
//            below exits non-zero when its stdin is not the genkey output, so the
//            piping contract is asserted, not assumed.
//            End-to-end dispatch through the ze binary:
//            test/parse/cli-generate-wireguard-keypair.ci

// silenceStderr redirects os.Stderr to /dev/null for the duration of
// the test. Restores on cleanup.
func silenceStderr(t *testing.T) {
	t.Helper()
	old := os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}
	os.Stderr = devnull
	t.Cleanup(func() {
		os.Stderr = old
		if err := devnull.Close(); err != nil {
			t.Logf("close devnull: %v", err)
		}
	})
}

// TestRunWgKeypair_RejectsArgs ensures stray arguments exit 1 without
// attempting to exec `wg`.
func TestRunWgKeypair_RejectsArgs(t *testing.T) {
	silenceStderr(t)
	if rc := RunWgKeypair([]string{"extra"}); rc != 1 {
		t.Errorf("RunWgKeypair(extra) = %d, want 1", rc)
	}
}

// Keys the wg fixture below prints. They are not real WireGuard keys and are not
// meant to be: the point is that the exact bytes `wg genkey` produced come back
// out of `wg pubkey`'s stdin and reach stdout.
const (
	fixturePrivateKey = "ZeUnitTestPrivateKey00000000000000000000000="
	fixturePublicKey  = "ZeUnitTestPublicKey000000000000000000000000="
)

// installWgFixture puts a deterministic `wg` on PATH for the duration of the
// test and returns nothing: the fixture IS the assertion. `wg pubkey` exits
// non-zero unless its stdin is exactly what `wg genkey` printed, so a
// RunWgKeypair that stopped piping the private key through cannot pass.
//
// This replaces a LookPath("wg") skip. `wg` is not installed on the build hosts
// or in CI, so the skip fired every time and the success path -- the whole
// reason the command exists -- was never executed (ai/rules/testing.md).
func installWgFixture(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"genkey)\n" +
		"  [ $# -eq 1 ] || { echo \"wg fixture: genkey argv: $*\" >&2; exit 2; }\n" +
		"  echo '" + fixturePrivateKey + "'\n" +
		"  ;;\n" +
		"pubkey)\n" +
		"  [ $# -eq 1 ] || { echo \"wg fixture: pubkey argv: $*\" >&2; exit 2; }\n" +
		"  IFS= read -r GOT\n" +
		"  [ \"$GOT\" = '" + fixturePrivateKey + "' ] || {\n" +
		"    echo \"wg fixture: pubkey stdin '$GOT' is not the genkey output\" >&2; exit 1; }\n" +
		"  echo '" + fixturePublicKey + "'\n" +
		"  ;;\n" +
		"*)\n" +
		"  echo \"wg fixture: unexpected argv: $*\" >&2; exit 2\n" +
		"  ;;\n" +
		"esac\n"
	path := filepath.Join(dir, "wg")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { //nolint:gosec // test fixture must be executable
		t.Fatalf("write wg fixture: %v", err)
	}
	// The fixture dir goes FIRST, so it wins over a real `wg` on a host that has
	// one; the rest of PATH stays so /bin/sh keeps finding what it needs.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// captureStdout runs fn with os.Stdout replaced by a pipe and returns what it
// wrote. RunWgKeypair prints through os.Stdout directly, so this is the only
// place its output can be read.
func captureStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	rc := fn()
	if err := w.Close(); err != nil {
		t.Logf("close pipe writer: %v", err)
	}
	os.Stdout = old
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return rc, string(buf)
}

// TestRunWgKeypair_PipesGenkeyIntoPubkey is the success path: `wg genkey` runs,
// its output is fed to `wg pubkey` on stdin, and both keys reach stdout in the
// documented two-line form.
func TestRunWgKeypair_PipesGenkeyIntoPubkey(t *testing.T) {
	installWgFixture(t)
	silenceStderr(t)

	rc, out := captureStdout(t, func() int { return RunWgKeypair(nil) })
	if rc != 0 {
		t.Fatalf("RunWgKeypair() = %d, want 0; stdout=%q", rc, out)
	}

	want := "private: " + fixturePrivateKey + "\npublic:  " + fixturePublicKey + "\n"
	if out != want {
		t.Errorf("RunWgKeypair() stdout =\n%q\nwant\n%q", out, want)
	}
}

// TestRunWgKeypair_ReportsMissingWg checks the documented failure: with no `wg`
// on PATH the command exits 1 rather than printing an empty or partial keypair.
func TestRunWgKeypair_ReportsMissingWg(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	silenceStderr(t)

	rc, out := captureStdout(t, func() int { return RunWgKeypair(nil) })
	if rc != 1 {
		t.Errorf("RunWgKeypair() with no wg = %d, want 1", rc)
	}
	if out != "" {
		t.Errorf("RunWgKeypair() with no wg wrote %q to stdout, want nothing", out)
	}
	if _, err := exec.LookPath("wg"); err == nil {
		t.Fatal("PATH override failed: wg is still resolvable, so this test proved nothing")
	}
}

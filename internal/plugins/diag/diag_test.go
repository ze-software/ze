package diag

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// VALIDATES: runWgKeypair (diag.go) runs `wg genkey`, pipes THAT private key
//            into `wg pubkey` on stdin, prints both keys, and refuses arguments.
// PREVENTS:  the success path going unexecuted. It used to be guarded by a
//            LookPath("wg") skip, and `wg` is installed on no build host here, so
//            the only path the command exists for was never run. The `wg` fixture
//            below exits non-zero when its stdin is not the genkey output, so the
//            piping contract is asserted, not assumed.
//            End-to-end dispatch through the ze binary:
//            test/parse/cli-generate-wireguard-keypair.ci

// TestRunWgKeypair_RejectsArgs ensures stray arguments are refused with exit
// code 1, as a returned diagnostic, without attempting to exec `wg`.
func TestRunWgKeypair_RejectsArgs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	answer, written := captureOutput(t, func() command.LocalAnswer { return runWgKeypair(commandtest.Args("extra")) })
	if answer.Code != 1 || answer.Payload != nil {
		t.Errorf("runWgKeypair(extra) = code %d payload %v, want a refusal with code 1", answer.Code, answer.Payload)
	}
	if want := "request wireguard keypair: no arguments accepted\n" + wgKeypairUsage; answer.Diagnostic != want {
		t.Errorf("runWgKeypair(extra) diagnostic = %q, want %q", answer.Diagnostic, want)
	}
	if written != "" {
		t.Errorf("runWgKeypair(extra) wrote %q, want nothing", written)
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
// runWgKeypair that stopped piping the private key through cannot pass.
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

// captureOutput runs fn with os.Stdout and os.Stderr replaced by one pipe and
// answers its answer and everything written to either stream.
func captureOutput(t *testing.T, fn func() command.LocalAnswer) (command.LocalAnswer, string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout, os.Stderr = w, w
	answer := fn()
	if err := w.Close(); err != nil {
		t.Logf("close pipe writer: %v", err)
	}
	os.Stdout, os.Stderr = oldOut, oldErr
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return answer, string(buf)
}

// TestRunWgKeypair_PipesGenkeyIntoPubkey is the success path: `wg genkey` runs,
// its output is fed to `wg pubkey` on stdin, the handler answers both keys
// without writing, and the local route prints them in the documented two-line
// form.
func TestRunWgKeypair_PipesGenkeyIntoPubkey(t *testing.T) {
	installWgFixture(t)

	answer, written := captureOutput(t, func() command.LocalAnswer { return runWgKeypair(commandtest.Args()) })
	if answer.Code != 0 || answer.Diagnostic != "" {
		t.Fatalf("runWgKeypair() = code %d diagnostic %q, want 0", answer.Code, answer.Diagnostic)
	}
	if written != "" {
		t.Errorf("runWgKeypair() wrote %q, want nothing", written)
	}
	keypair, ok := answer.Payload.(wgKeypairAnswer)
	if !ok {
		t.Fatalf("payload %T, want wgKeypairAnswer", answer.Payload)
	}
	if keypair.Keypair.PrivateKey != fixturePrivateKey || keypair.Keypair.PublicKey != fixturePublicKey {
		t.Errorf("keypair = %+v, want the fixture keys", keypair.Keypair)
	}

	var code int
	_, out := captureOutput(t, func() command.LocalAnswer {
		code = command.RenderLocalAnswer("request wireguard keypair", answer.Payload)
		return command.LocalAnswer{}
	})
	want := "private: " + fixturePrivateKey + "\npublic:  " + fixturePublicKey + "\n"
	if code != 0 || out != want {
		t.Errorf("local route printed (code %d)\n%q\nwant\n%q", code, out, want)
	}
}

// TestRunWgKeypair_ReportsMissingWg checks the documented failure: with no `wg`
// on PATH the command refuses with exit code 1 rather than answering an empty
// or partial keypair.
func TestRunWgKeypair_ReportsMissingWg(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	answer, written := captureOutput(t, func() command.LocalAnswer { return runWgKeypair(commandtest.Args()) })
	if answer.Code != 1 || answer.Payload != nil {
		t.Errorf("runWgKeypair() with no wg = code %d payload %v, want a refusal with code 1", answer.Code, answer.Payload)
	}
	if !strings.HasPrefix(answer.Diagnostic, "request wireguard keypair: wg genkey failed: ") {
		t.Errorf("diagnostic %q does not name the failed step", answer.Diagnostic)
	}
	if written != "" {
		t.Errorf("runWgKeypair() with no wg wrote %q, want nothing", written)
	}
	if _, err := exec.LookPath("wg"); err == nil {
		t.Fatal("PATH override failed: wg is still resolvable, so this test proved nothing")
	}
}

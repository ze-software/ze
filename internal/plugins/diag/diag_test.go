package diag

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/crypto/curve25519"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// VALIDATES: runWgKeypair (diag.go) makes a WireGuard keypair inside ze with no
//            `wg` on PATH, answers a clamped Curve25519 private key and the
//            public key that private key derives, answers fresh keys on every
//            call, prints both keys in the documented two-line form, and
//            refuses arguments.
// PREVENTS:  the command depending on wireguard-tools, which the appliance image
//            and the build hosts do not carry, and an answer whose public key
//            does not belong to its private key.
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
	want := "request wireguard keypair: no arguments accepted\n" +
		"Usage: ze request wireguard keypair (the keys are made from random bytes alone, so nothing typed after the command is read)\n" +
		"fix: ze request wireguard keypair\n"
	if answer.Diagnostic != want {
		t.Errorf("runWgKeypair(extra) diagnostic = %q, want %q", answer.Diagnostic, want)
	}
	if written != "" {
		t.Errorf("runWgKeypair(extra) wrote %q, want nothing", written)
	}
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

// generateKeypair runs the command with no arguments, checks it answered a
// keypair without writing, and answers the keypair with its decoded private
// and public keys.
func generateKeypair(t *testing.T) (wgKeypair, []byte, []byte) {
	t.Helper()
	answer, written := captureOutput(t, func() command.LocalAnswer { return runWgKeypair(commandtest.Args()) })
	if answer.Code != 0 || answer.Diagnostic != "" {
		t.Fatalf("runWgKeypair() = code %d diagnostic %q, want 0", answer.Code, answer.Diagnostic)
	}
	if written != "" {
		t.Errorf("runWgKeypair() wrote %q, want nothing: the private key leaves only in the answer", written)
	}
	keypair, ok := answer.Payload.(wgKeypairAnswer)
	if !ok {
		t.Fatalf("payload %T, want wgKeypairAnswer", answer.Payload)
	}
	private, err := base64.StdEncoding.DecodeString(keypair.Keypair.PrivateKey)
	if err != nil {
		t.Fatalf("private key %q is not standard base64: %v", keypair.Keypair.PrivateKey, err)
	}
	public, err := base64.StdEncoding.DecodeString(keypair.Keypair.PublicKey)
	if err != nil {
		t.Fatalf("public key %q is not standard base64: %v", keypair.Keypair.PublicKey, err)
	}
	if len(private) != curve25519.ScalarSize || len(public) != curve25519.PointSize {
		t.Fatalf("keys are %d and %d bytes, want %d and %d", len(private), len(public), curve25519.ScalarSize, curve25519.PointSize)
	}
	return keypair.Keypair, private, public
}

// TestRunWgKeypair_GeneratesInProcess is the success path, run with an empty
// PATH so no `wg` can answer. For each of two calls it derives the public key
// from the answered private key with x/crypto's X25519 over the base point,
// independently of the producer's library, and requires the answered public
// key to equal it. It requires the private key clamped as
// https://cr.yp.to/ecdh.html specifies (what `wg genkey` prints), the two
// calls to answer different keys, and the local route to print the documented
// two lines.
func TestRunWgKeypair_GeneratesInProcess(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := exec.LookPath("wg"); err == nil {
		t.Fatal("PATH override failed: wg is still resolvable, so this test proves nothing about running without it")
	}

	first, firstPrivate, firstPublic := generateKeypair(t)
	second, secondPrivate, secondPublic := generateKeypair(t)

	for _, generated := range []struct{ private, public []byte }{
		{firstPrivate, firstPublic},
		{secondPrivate, secondPublic},
	} {
		derived, err := curve25519.X25519(generated.private, curve25519.Basepoint)
		if err != nil {
			t.Fatalf("derive the public key: %v", err)
		}
		if !bytes.Equal(derived, generated.public) {
			t.Errorf("public key %x is not the one private key %x derives (%x)", generated.public, generated.private, derived)
		}
		if generated.private[0]&7 != 0 || generated.private[31]&128 != 0 || generated.private[31]&64 == 0 {
			t.Errorf("private key %x is not clamped (byte 0 low three bits clear, byte 31 top bit clear, bit 6 set)", generated.private)
		}
	}
	if bytes.Equal(firstPrivate, secondPrivate) {
		t.Errorf("two calls answered the same private key %x", firstPrivate)
	}
	if bytes.Equal(firstPublic, secondPublic) {
		t.Errorf("two calls answered the same public key %x", firstPublic)
	}

	var code int
	_, out := captureOutput(t, func() command.LocalAnswer {
		code = command.RenderLocalAnswer("request wireguard keypair", wgKeypairAnswer{Keypair: first})
		return command.LocalAnswer{}
	})
	want := "private: " + first.PrivateKey + "\npublic:  " + first.PublicKey + "\n"
	if code != 0 || out != want {
		t.Errorf("local route printed (code %d)\n%q\nwant\n%q", code, out, want)
	}
	if first == second {
		t.Errorf("two calls answered the same keypair %+v", first)
	}
}

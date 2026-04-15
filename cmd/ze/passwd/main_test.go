package passwd

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestRunImplPipedPlaintext: piped plaintext yields a valid bcrypt hash on stdout.
//
// VALIDATES: AC-13 (echo "secret" | ze passwd prints valid bcrypt).
func TestRunImplPipedPlaintext(t *testing.T) {
	in := strings.NewReader("secret\n")
	var out, errOut bytes.Buffer

	if code := runImpl(in, &out, &errOut); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
	hash := strings.TrimSpace(out.String())
	if hash == "" {
		t.Fatal("expected non-empty hash on stdout")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret")); err != nil {
		t.Errorf("hash %q does not validate against plaintext: %v", hash, err)
	}
}

// TestRunImplEmptyPlaintext: empty input is rejected with exit 1.
//
// VALIDATES: defense against accidentally hashing the empty string.
func TestRunImplEmptyPlaintext(t *testing.T) {
	in := strings.NewReader("\n")
	var out, errOut bytes.Buffer

	code := runImpl(in, &out, &errOut)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0; stdout=%q", out.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout must be empty on error, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "empty password") {
		t.Errorf("stderr should mention empty password, got %q", errOut.String())
	}
}

// TestRunImplCRLFPlaintext: CRLF line endings are stripped before hashing.
//
// PREVENTS: Windows-piped input producing a hash of "secret\r" instead of "secret".
func TestRunImplCRLFPlaintext(t *testing.T) {
	in := strings.NewReader("secret\r\n")
	var out, errOut bytes.Buffer

	if code := runImpl(in, &out, &errOut); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
	hash := strings.TrimSpace(out.String())
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret")); err != nil {
		t.Errorf("CRLF input not stripped; got hash for unexpected plaintext: %v", err)
	}
}

// TestRunImplLongPlaintextWithinBcryptLimit: 72-byte input produces a valid hash.
//
// PREVENTS: regression where bcrypt's 72-byte limit silently truncates input.
func TestRunImplLongPlaintextWithinBcryptLimit(t *testing.T) {
	plain := strings.Repeat("a", 72)
	in := strings.NewReader(plain + "\n")
	var out, errOut bytes.Buffer

	if code := runImpl(in, &out, &errOut); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
	hash := strings.TrimSpace(out.String())
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		t.Errorf("72-byte hash does not validate: %v", err)
	}
}

// TestRunImplOversizePlaintextRejected: 73-byte input is rejected, not silently
// truncated. The scanner buffer accommodates the input; the explicit length
// check in runImpl produces a clear error rather than a misleading hash.
//
// VALIDATES: AC follow-up -- bcrypt 72-byte boundary surfaces as user error.
// PREVENTS: silently producing a hash that only validates the first 72 bytes.
func TestRunImplOversizePlaintextRejected(t *testing.T) {
	plain := strings.Repeat("a", 73)
	in := strings.NewReader(plain + "\n")
	var out, errOut bytes.Buffer

	code := runImpl(in, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit 1 (user error), got %d; stdout=%q", code, out.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout must be empty when input is rejected, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "too long") || !strings.Contains(errOut.String(), "72") {
		t.Errorf("error message should name the limit, got %q", errOut.String())
	}
}

package gokrazyutil

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// TestAuthHeaderIncludesPassword is the failing-first guard for the
// AuthHeader password-omission bug: the credentials must encode
// "gokrazy:<password>", not "gokrazy:". gokrazy authenticates as user
// "gokrazy" with the configured password as the secret, so an empty
// password would 401 against a real gokrazy management interface.
func TestAuthHeaderIncludesPassword(t *testing.T) {
	const pw = "s3cret"
	got := authHeaderFor(pw)
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("gokrazy:"+pw))
	if got != want {
		t.Fatalf("authHeaderFor(%q) = %q, want %q (password must be in credentials)", pw, got, want)
	}

	// Decode-side guard: the header must not carry an empty password.
	raw, err := base64.StdEncoding.DecodeString(got[len("Basic "):])
	if err != nil {
		t.Fatalf("credentials not valid base64: %v", err)
	}
	if string(raw) == "gokrazy:" {
		t.Fatalf("password omitted from credentials: decoded %q", raw)
	}
}

func TestAuthHeaderEmptyPassword(t *testing.T) {
	if got := authHeaderFor(""); got != "" {
		t.Fatalf("authHeaderFor(\"\") = %q, want empty string", got)
	}
}

func TestReadPasswordFromSelectsFirstReadable(t *testing.T) {
	dir := t.TempDir()
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(second, []byte("  hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// First path is absent, so ReadPassword falls through to the second and
	// trims surrounding whitespace/newline.
	got := readPasswordFrom([]string{filepath.Join(dir, "missing.txt"), second})
	if got != "hunter2" {
		t.Fatalf("readPasswordFrom = %q, want %q", got, "hunter2")
	}
}

func TestReadPasswordFromNoneFound(t *testing.T) {
	if got := readPasswordFrom([]string{filepath.Join(t.TempDir(), "nope.txt")}); got != "" {
		t.Fatalf("readPasswordFrom(missing) = %q, want empty string", got)
	}
}

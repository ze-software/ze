package rescueauth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
)

func testSalt(t *testing.T) []byte {
	t.Helper()

	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("read salt: %v", err)
	}
	return salt
}

// VALIDATES: AC-2 -- the correct rescue token verifies against the encoded value.
// VALIDATES: AC-3 -- a wrong token does not.
// PREVENTS: A credential that either never authenticates (locking the operator
// out of the rescue shell) or authenticates anything (no gate at all).
func TestCheckRoundTrip(t *testing.T) {
	salt := testSalt(t)
	value := Value("correct-horse-battery", salt)

	if !Check("correct-horse-battery", value) {
		t.Error("the correct token was refused")
	}
	for _, wrong := range []string{"", "correct-horse-batter", "correct-horse-batteryy", "CORRECT-HORSE-BATTERY"} {
		if Check(wrong, value) {
			t.Errorf("token %q was accepted", wrong)
		}
	}
}

// VALIDATES: AC-1 -- the encoded credential is salted, so the same token yields a
// different value every time it is provisioned.
// PREVENTS: An unsalted digest, which lets one precomputed table cover every
// appliance and lets an observer spot two installs sharing a credential.
func TestValueIsSalted(t *testing.T) {
	first := Value("same-token", testSalt(t))
	second := Value("same-token", testSalt(t))

	if first == second {
		t.Fatal("two provisionings of the same token produced an identical value: the salt is not in play")
	}
	if !Check("same-token", first) || !Check("same-token", second) {
		t.Error("a salted value failed to verify its own token")
	}
}

// VALIDATES: AC-1 -- the encoded value is cmdline-safe and reveals no plaintext.
// PREVENTS: A form that an iPXE script would interpolate ('$') or a kernel
// cmdline would split (whitespace), and a value carrying the token itself.
func TestValueIsCmdlineSafe(t *testing.T) {
	value := Value("s3cret-token", testSalt(t))

	if got, want := len(value), SaltLen*2+1+digestLen*2; got != want {
		t.Errorf("value length = %d, want %d (%q)", got, want, value)
	}
	for _, bad := range []string{" ", "\t", "\n", "$", "\"", "'"} {
		if strings.Contains(value, bad) {
			t.Errorf("value %q contains %q, which is unsafe on a cmdline or in an iPXE script", value, bad)
		}
	}
	if strings.Contains(value, "s3cret-token") {
		t.Errorf("value %q leaks the token", value)
	}
	if strings.ToLower(value) != value {
		t.Errorf("value %q is not lowercase; the cmdline form must have one spelling", value)
	}
}

// VALIDATES: AC-3 -- a malformed ze.rescue-auth never authenticates.
// PREVENTS: A truncated, re-cased, or attacker-supplied cmdline value being
// treated as "no credential to check" and opening the shell
// (ai/rules/fail-closed-guards.md).
func TestCheckFailsClosedOnMalformedValue(t *testing.T) {
	salt := testSalt(t)
	valid := Value("token", salt)
	saltHex, digestHex, _ := strings.Cut(valid, ":")

	malformed := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"no separator", saltHex + digestHex},
		{"salt only", saltHex},
		{"digest only", digestHex},
		{"empty digest", saltHex + ":"},
		{"empty salt", ":" + digestHex},
		{"short salt", saltHex[1:] + ":" + digestHex},
		{"long salt", "a" + saltHex + ":" + digestHex},
		{"short digest", saltHex + ":" + digestHex[1:]},
		{"long digest", saltHex + ":" + digestHex + "a"},
		{"uppercase", strings.ToUpper(valid)},
		{"non hex salt", strings.Repeat("z", len(saltHex)) + ":" + digestHex},
		{"non hex digest", saltHex + ":" + strings.Repeat("z", len(digestHex))},
		{"legacy bare sha256", strings.Repeat("a", 64)},
		{"two separators", saltHex + ":" + digestHex + ":" + digestHex},
	}

	for _, tc := range malformed {
		t.Run(tc.name, func(t *testing.T) {
			if Check("token", tc.value) {
				t.Errorf("malformed value %q authenticated", tc.value)
			}
			if Check("", tc.value) {
				t.Errorf("malformed value %q authenticated an empty token", tc.value)
			}
		})
	}
}

// VALIDATES: AC-1 boundary -- salt and digest hex lengths at, below, and above valid.
// PREVENTS: An off-by-one in the shape check letting a truncated digest through.
func TestValidateShape(t *testing.T) {
	salt := testSalt(t)
	valid := Value("token", salt)
	saltHex, digestHex, _ := strings.Cut(valid, ":")

	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"exact", valid, true},
		{"salt one short", saltHex[1:] + ":" + digestHex, false},
		{"salt one long", saltHex + "a:" + digestHex, false},
		{"digest one short", saltHex + ":" + digestHex[1:], false},
		{"digest one long", saltHex + ":" + digestHex + "a", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.value)
			if tc.valid && err != nil {
				t.Errorf("Validate(%q) = %v, want nil", tc.value, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("Validate(%q) = nil, want an error", tc.value)
			}
		})
	}
}

// VALIDATES: AC-1 -- the encoding is a fixed function of (token, salt), pinned to
// a known vector so the argon2id parameters cannot drift silently.
// PREVENTS: A parameter change that quietly invalidates every already-provisioned
// credential, and desynchronises the QEMU install-evidence harness, which carries
// this exact vector as a constant.
func TestValuePinnedVector(t *testing.T) {
	salt, err := hex.DecodeString(pinnedSaltHex)
	if err != nil {
		t.Fatalf("decode pinned salt: %v", err)
	}

	got := Value(pinnedToken, salt)

	if got != pinnedValue {
		t.Errorf("Value(%q, %s) =\n  %s\nwant\n  %s\n"+
			"if the argon2 parameters changed on purpose, update pinnedValue here AND "+
			"RESCUE_AUTH in scripts/evidence/effective-install-scenarios-qemu.py",
			pinnedToken, pinnedSaltHex, got, pinnedValue)
	}
	if !Check(pinnedToken, pinnedValue) {
		t.Error("the pinned vector does not verify its own token")
	}
}

const (
	pinnedToken   = "ze-rescue-evidence"
	pinnedSaltHex = "5a65726573637565536f6c74303031ff"
	pinnedValue   = "5a65726573637565536f6c74303031ff:fed7b65bb317bc34097440c9bbd0a2ab3749edb8d88d3d37c94abe6cf62e399b"
)

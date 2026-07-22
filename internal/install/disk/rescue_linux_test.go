// VALIDATES: AC-7 (rescue-token auth), AC-7b (ungated ISO), AC-7c (30s reboot for network)
// PREVENTS: wrong fatal branch selection; a rescue gate that accepts the wrong token

// test-relax: the four TestRecoveryAuth* cases exercised checkPassword, which was
// DELETED along with the unsalted-sha256 credential it verified. Their successor
// coverage (correct/wrong/empty token, malformed values, no double-derivation) is
// internal/core/rescueauth's test suite, which is strictly broader. The gate is
// re-covered end to end below against rescueauth.Check.

//go:build linux && ze_installer

package disk

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/rescueauth"
)

// VALIDATES: AC-2/AC-3 -- the installer's rescue gate accepts the provisioned
// token and refuses anything else, through the same call the console makes.
// PREVENTS: The gate being wired to a stale or absent verifier after the
// credential changed shape.
func TestRescueGateAcceptsProvisionedToken(t *testing.T) {
	token, authValue, err := rescueauth.NewValue()
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}

	if !rescueauth.Check(token, authValue) {
		t.Fatal("the provisioned token was refused by the rescue gate")
	}
	for _, wrong := range []string{"", "wrong", token + "x", strings.ToUpper(token)} {
		if rescueauth.Check(wrong, authValue) {
			t.Errorf("rescue gate accepted %q", wrong)
		}
	}
}

// VALIDATES: AC-3 -- a malformed ze.rescue-auth never opens the shell.
// PREVENTS: A cmdline value an attacker can supply on the PXE network being
// treated as "nothing to verify" (ai/rules/fail-closed-guards.md).
func TestRescueGateFailsClosedOnMalformedCmdlineValue(t *testing.T) {
	for _, bad := range []string{"", "garbage", strings.Repeat("a", 64), "aabb:ccdd"} {
		if rescueauth.Check("anything", bad) {
			t.Errorf("malformed ze.rescue-auth %q opened the gate", bad)
		}
	}
}

// validRescueAuth is a well-formed "<saltHex>:<digestHex>" credential. The
// original table used "abcd1234", which is NOT a valid credential in the
// argon2id encoding, so it exercised the malformed case while claiming to
// exercise the present case.
const validRescueAuth = "5a65726573637565536f6c74303031ff:fed7b65bb317bc34097440c9bbd0a2ab3749edb8d88d3d37c94abe6cf62e399b"

func TestFatalPolicyBranch(t *testing.T) {
	tests := []struct {
		name       string
		rescueAuth string
		source     string
		want       fatalBranch
	}{
		{"credential present HTTP", validRescueAuth, sourceHTTP, branchGated},
		{"credential present ISO", validRescueAuth, sourceISO, branchGated},
		{"no cred + ISO", "", sourceISO, branchUngated},
		{"no cred + HTTP", "", sourceHTTP, branchReboot},
	}

	for _, tt := range tests {
		got := selectFatalBranch(tt.rescueAuth, tt.source)
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

// VALIDATES: a ze.rescue-auth the gate cannot verify against must never select
// the gated branch, on either media.
// PREVENTS: An unattended box hanging forever at a rescue prompt. rescueauth.Check
// fails closed on a malformed value, so branchGated would prompt for a token that
// nothing can satisfy; a network install would then wait at a console instead of
// rebooting to retry, which is exactly what the three-branch policy exists to
// avoid. A malformed credential is also never treated as "no credential", because
// that would open an ungated shell on ISO media off a typo.
func TestFatalPolicyBranchRejectsMalformedCredential(t *testing.T) {
	malformed := []string{
		"abcd1234",
		"garbage",
		strings.Repeat("a", 64), // the legacy bare-sha256 form
		validRescueAuth[1:],     // one char short in the salt
		validRescueAuth + "a",   // one char long in the digest
		strings.ToUpper(validRescueAuth),
		":",
	}

	for _, source := range []string{sourceHTTP, sourceISO} {
		for _, auth := range malformed {
			got := selectFatalBranch(auth, source)
			if got == branchGated {
				t.Errorf("source=%s auth=%q selected branchGated: the gate can never accept a token for this value", source, auth)
			}
			if got == branchUngated {
				t.Errorf("source=%s auth=%q selected branchUngated: a malformed credential must not open an unauthenticated shell", source, auth)
			}
		}
	}
}

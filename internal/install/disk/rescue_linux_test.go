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

func TestFatalPolicyBranch(t *testing.T) {
	tests := []struct {
		name       string
		rescueAuth string
		source     string
		want       fatalBranch
	}{
		{"credential present HTTP", "abcd1234", sourceHTTP, branchGated},
		{"credential present ISO", "abcd1234", sourceISO, branchGated},
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

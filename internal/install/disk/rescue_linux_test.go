// VALIDATES: AC-7 (rescue-token auth), AC-7b (ungated ISO), AC-7c (30s reboot for network)
// PREVENTS: wrong fatal branch selection; a rescue gate that accepts the wrong token

// test-relax: the four TestRecoveryAuth* cases exercised checkPassword, which was
// DELETED along with the unsalted-sha256 credential it verified. Successor
// coverage is in two places: internal/core/rescueauth's suite covers the
// encoding and the comparison, and TestRescueGate* below drive THIS package's
// own entry point, gateWithRescueToken -- which the deleted tests reached via
// checkPassword and which an earlier version of this replacement did not cover
// at all.
//
// Note this file is guarded by `ze_installer`, a build tag no plain `go test`
// supplies. It ran nowhere until `make ze-installer-unit-test` was added
// (mk/test-unit.mk) and made a prerequisite of ze-unit-test; before that, every
// test here was inert. See the tag-orphan list in test/health/latest.json.

//go:build linux && ze_installer

package disk

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/rescueauth"
)

// VALIDATES: AC-2/AC-3 -- the installer's own rescue gate accepts the
// provisioned token and refuses everything else, driven through
// gateWithRescueToken, the function the console actually calls.
// PREVENTS: Testing rescueauth.Check directly and calling that "the gate". That
// duplicates internal/core/rescueauth's own suite and proves nothing about this
// package: replacing the rescueauth.Check call at the gate with `return true`
// would leave such a test green while every install dropped to an open shell.
func TestRescueGateAcceptsProvisionedToken(t *testing.T) {
	token, authValue, err := rescueauth.NewValue()
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}

	var out strings.Builder
	if !gateWithRescueToken(strings.NewReader(token+"\n"), &out, authValue) {
		t.Fatalf("the provisioned token was refused by the gate (console said %q)", out.String())
	}
	if !strings.Contains(out.String(), "authenticated") {
		t.Errorf("console did not confirm authentication: %q", out.String())
	}
}

// VALIDATES: AC-3 -- a wrong token is refused, and the gate gives up after
// rescueMaxAttempts rather than prompting forever.
// PREVENTS: An unbounded retry loop at the console, and a gate that accepts on
// the wrong comparison.
func TestRescueGateRefusesWrongToken(t *testing.T) {
	_, authValue, err := rescueauth.NewValue()
	if err != nil {
		t.Fatalf("NewValue: %v", err)
	}

	var out strings.Builder
	// Offer more lines than the gate is allowed to consume.
	attempts := strings.Repeat("wrong\n", rescueMaxAttempts+3)
	if gateWithRescueToken(strings.NewReader(attempts), &out, authValue) {
		t.Fatal("a wrong token opened the rescue shell")
	}
	if got := strings.Count(out.String(), "incorrect"); got != rescueMaxAttempts {
		t.Errorf("gate prompted %d times, want exactly rescueMaxAttempts=%d", got, rescueMaxAttempts)
	}
	if !strings.Contains(out.String(), "too many attempts") {
		t.Errorf("gate did not report giving up: %q", out.String())
	}
}

// VALIDATES: AC-3 -- a malformed ze.rescue-auth never opens the shell, through
// the installer's own gate.
// PREVENTS: A cmdline value an attacker can supply on the PXE network being
// treated as "nothing to verify" (ai/rules/fail-closed-guards.md).
func TestRescueGateFailsClosedOnMalformedCmdlineValue(t *testing.T) {
	for _, bad := range []string{"", "garbage", strings.Repeat("a", 64), "aabb:ccdd"} {
		var out strings.Builder
		if gateWithRescueToken(strings.NewReader("anything\n"), &out, bad) {
			t.Errorf("malformed ze.rescue-auth %q opened the gate", bad)
		}
	}
}

// validRescueAuth is a well-formed "<saltHex>:<digestHex>" credential. The
// original table used "abcd1234", which is NOT valid in the argon2id encoding,
// so it exercised the malformed case while claiming to exercise the present one.
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

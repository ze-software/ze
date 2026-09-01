// VALIDATES: every place in this package that acts on "this auth mode carries a
// password" reads IsEAPPasswordMode and nothing else. The walk covers each mode the
// package declares, so a mode the predicate names is one every site must honor.
// PREVENTS: the state before 2026-09-01. parseEAPUser, parseAuthConfig and
// ValidateRemoteAccess each carried their own `AuthEAPMSCHAPv2 || AuthEAPMD5` list. A
// sixth password method then needed three edits here and two more in the engine. One
// missed edit read no credential and refused nothing (ai/rules/principles.md).
package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// eapPasswordModeSecret is the credential every case below configures. A site that
// stopped reading its leaf answers with the empty string instead.
const eapPasswordModeSecret = "eap-password-mode-secret" //nolint:gosec // test fixture

// declaredAuthModesForPasswordWalk returns every mode the package names, read from
// String() rather than from a list copied here. A copy would disagree with the enum the
// day a mode is added, which is the failure this whole file exists to catch.
func declaredAuthModesForPasswordWalk(t *testing.T) []AuthMode {
	t.Helper()

	var modes []AuthMode
	for raw := AuthMode(1); raw.String() != AuthUnknown.String(); raw++ {
		modes = append(modes, raw)
	}
	if len(modes) < 2 {
		t.Fatalf("the enum walk found %d modes, so no disagreement between sites could show", len(modes))
	}
	return modes
}

// TestEAPPasswordModeDecidesEveryCredentialSite walks the declared modes and checks the
// three sites in this package against the one predicate.
func TestEAPPasswordModeDecidesEveryCredentialSite(t *testing.T) {
	modes := declaredAuthModesForPasswordWalk(t)

	passwordModes, otherModes := 0, 0
	for _, mode := range modes {
		if IsEAPPasswordMode(mode) {
			passwordModes++
		} else {
			otherModes++
		}
		t.Run(mode.String(), func(t *testing.T) {
			assertEAPUserCredential(t, mode)
			assertPeerSharedSecret(t, mode)
			assertRemoteAccessRefusesAnEmptyPassword(t, mode)
		})
	}

	// Without these two counts the assertions above are satisfied by a tree in which
	// the predicate names every mode, or none.
	if passwordModes == 0 {
		t.Error("IsEAPPasswordMode names no declared mode, so no password assertion ran")
	}
	if otherModes == 0 {
		t.Error("IsEAPPasswordMode names every declared mode, so no refusal assertion ran")
	}
}

// assertEAPUserCredential checks parseEAPUser: the password leaf is read for exactly the
// modes IsEAPPasswordMode names.
func assertEAPUserCredential(t *testing.T, mode AuthMode) {
	t.Helper()

	entry := config.NewTree()
	entry.Set("password", eapPasswordModeSecret)
	entry.Set("certificate", "eap-password-mode-cert")

	user, err := parseEAPUser("walker", entry, mode)
	if err != nil {
		t.Fatalf("parseEAPUser for mode %s: %v", mode, err)
	}
	got := user.Password == eapPasswordModeSecret
	if got != IsEAPPasswordMode(mode) {
		t.Errorf("parseEAPUser read the password leaf = %v for mode %s, and IsEAPPasswordMode says %v",
			got, mode, IsEAPPasswordMode(mode))
	}
}

// assertPeerSharedSecret checks parseAuthConfig: the pre-shared-secret leaf is read for
// the pre-shared-secret mode and for exactly the modes IsEAPPasswordMode names.
func assertPeerSharedSecret(t *testing.T, mode AuthMode) {
	t.Helper()

	entry := config.NewTree()
	entry.Set("mode", mode.String())
	entry.Set("pre-shared-secret", eapPasswordModeSecret)

	auth, err := parseAuthConfig("walker", entry)
	if err != nil {
		t.Fatalf("parseAuthConfig for mode %s: %v", mode, err)
	}
	got := auth.PSK == eapPasswordModeSecret
	want := mode == AuthPreSharedSecret || IsEAPPasswordMode(mode)
	if got != want {
		t.Errorf("parseAuthConfig read the pre-shared-secret leaf = %v for mode %s, want %v",
			got, mode, want)
	}
}

// assertRemoteAccessRefusesAnEmptyPassword checks ValidateRemoteAccess: an eap-user with
// no password is refused for exactly the modes IsEAPPasswordMode names.
//
// The user carries a certificate, so the EAP-TLS arm of the same loop stays quiet. The
// refusal this reads can only be the password one.
func assertRemoteAccessRefusesAnEmptyPassword(t *testing.T, mode AuthMode) {
	t.Helper()

	cfg := &IPsecConfig{
		RemoteAccess: &RemoteAccessConfig{
			Auth: AuthConfig{Mode: mode},
			Users: map[string]EAPUser{
				"walker": {Name: "walker", Certificate: "eap-password-mode-cert"},
			},
		},
	}

	err := cfg.ValidateRemoteAccess()
	refused := err != nil && strings.Contains(err.Error(), "password is required")
	if refused != IsEAPPasswordMode(mode) {
		t.Errorf("ValidateRemoteAccess refused an empty password = %v for mode %s, and "+
			"IsEAPPasswordMode says %v (error %v)", refused, mode, IsEAPPasswordMode(mode), err)
	}
}

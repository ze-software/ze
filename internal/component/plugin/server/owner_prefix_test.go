package server

import (
	"errors"
	"testing"
)

// TestOwnerPrefixFollowsThePackage proves the wire-method prefix is derived from
// where the registering package lives, and that a package outside the two
// subsystem roots has no prefix at all.
//
// VALIDATES: AC-10 (spec-rpc-published-name-does-not-reach-its-handler): a
// component owns ze-<component>, a plugin owns ze-<plugin> with a "-cmd"
// suffix dropped, and a package nested under either takes its root's prefix.
// PREVENTS: a prefix chosen by the author rather than derived, which is how 43
// owner directories came to share ze-show.
func TestOwnerPrefixFollowsThePackage(t *testing.T) {
	cases := []struct {
		pkg  string
		want string
	}{
		{"github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer", "ze-bgp"},
		{"github.com/ze-software/ze/internal/component/l2tp/pppoe/cmd", "ze-l2tp"},
		{"github.com/ze-software/ze/internal/component/plugin/server", "ze-plugin"},
		{"github.com/ze-software/ze/internal/plugins/ospf", "ze-ospf"},
		{"github.com/ze-software/ze/internal/plugins/host-cmd/cmd", "ze-host"},
		{"github.com/ze-software/ze/internal/plugins/anomaly/detect", "ze-anomaly"},
		{"github.com/ze-software/ze/internal/test/plugins/fakel2tp/yang", "ze-test-fakel2tp"},
		{"github.com/ze-software/ze/internal/test/plugins/fakeredist", "ze-test-fakeredist"},
	}
	for _, tc := range cases {
		got, err := OwnerPrefix(tc.pkg)
		if err != nil {
			t.Errorf("OwnerPrefix(%q): %v", tc.pkg, err)
			continue
		}
		if got != tc.want {
			t.Errorf("OwnerPrefix(%q) = %q, want %q", tc.pkg, got, tc.want)
		}
	}

	for _, pkg := range []string{
		"",
		"github.com/ze-software/ze/cmd/ze/hub",
		"github.com/ze-software/ze/internal/component",
		"github.com/ze-software/ze/internal/core/ipc",
		"github.com/ze-software/ze/internal/plugins/-cmd",
		"github.com/ze-software/ze/internal/test/plugins",
		"github.com/ze-software/ze/internal/test/fixture",
		"example.com/other/internal/component/bgp",
	} {
		got, err := OwnerPrefix(pkg)
		if !errors.Is(err, ErrNoOwnerPrefix) {
			t.Errorf("OwnerPrefix(%q) = %q, %v; want ErrNoOwnerPrefix", pkg, got, err)
		}
	}
}

// TestRegisterRPCsStampsTheRegistrar proves the registrar is read from the
// caller rather than taken from the registration, so a package cannot claim
// another's prefix by filling the field.
//
// VALIDATES: RegisterRPCs records the caller's import path and overwrites a
// Registrar the caller set.
// PREVENTS: a Registrar copied from a struct literal, which would make the
// prefix check judge a claim instead of a fact.
func TestRegisterRPCsStampsTheRegistrar(t *testing.T) {
	saved := registeredRPCs
	t.Cleanup(func() { registeredRPCs = saved })
	registeredRPCs = nil

	RegisterRPCs(RPCRegistration{
		WireMethod: "ze-plugin:stamp-probe",
		registrar:  "github.com/ze-software/ze/internal/component/bgp",
	})

	if len(registeredRPCs) != 1 {
		t.Fatalf("registered %d rpcs, want 1", len(registeredRPCs))
	}
	const want = "github.com/ze-software/ze/internal/component/plugin/server"
	if got := registeredRPCs[0].Registrar(); got != want {
		t.Errorf("Registrar = %q, want %q", got, want)
	}
}

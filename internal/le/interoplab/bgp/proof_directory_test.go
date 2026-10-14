package bgp

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// proofRetainingCheckers are the checkers that write proof.json before
// teardown. Each one must root that proof at CheckContext.Checkout.
var proofRetainingCheckers = map[string]interoplab.Checker{
	"bgp-parsed-empty-mp-unreach-frr": checkParsedEmptyMPFRR,
	srv6IdentityScenario:              checkSRv6Identity,
	srv6IdentityUnchangedScenario:     checkSRv6Identity,
}

// proofTestNetwork is a well-formed lab network, so a checker reaches its
// evidence queries rather than failing on an address it cannot derive.
var proofTestNetwork = interoplab.Network{
	Name: "missing-evidence",
	IPv4: netip.MustParsePrefix("172.30.44.0/24"),
	IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
}

// TestRetainedProofLandsUnderTheCheckout proves a proof-retaining checker
// writes nothing under the process working directory. Method: the working
// directory is an empty sentinel standing for the package directory `go test`
// runs from, the checkout is a second temporary directory, and the checker runs
// against a lab with no evidence, so it fails and still retains its proof. The
// proof must be found under the checkout's session scratch and the sentinel must
// stay empty. Before the fix the scratch path, which is relative to the root,
// was resolved against the working directory, inside the source tree.
func TestRetainedProofLandsUnderTheCheckout(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "proof-directory-test")
	for name, checker := range proofRetainingCheckers {
		t.Run(name, func(t *testing.T) {
			workingDirectory := t.TempDir()
			t.Chdir(workingDirectory)
			checkout := t.TempDir()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()

			err := checker(ctx, &interoplab.CheckContext{
				Source:   interoplab.ScenarioSource{Name: name},
				Network:  proofTestNetwork,
				Lab:      noEvidenceLab{},
				Checkout: checkout,
			})
			if err == nil {
				t.Fatal("checker passed with no peer evidence")
			}
			entries, readErr := os.ReadDir(workingDirectory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				t.Errorf("checker wrote %s under the working directory", entry.Name())
			}
			pattern := filepath.Join(checkout, "tmp", "session", "*", "scratch", name+"-*", "proof.json")
			proofs, globErr := filepath.Glob(pattern)
			if globErr != nil {
				t.Fatal(globErr)
			}
			if len(proofs) != 1 {
				t.Fatalf("proofs under the checkout = %v, want exactly one matching %s", proofs, pattern)
			}
		})
	}
}

// TestRetainedProofRefusesWithoutCheckout proves a proof-retaining checker
// given no checkout refuses, rather than falling back to the working directory.
// Method: run each checker with an empty Checkout from an empty sentinel
// working directory; the error must name the missing checkout and the sentinel
// must stay empty.
func TestRetainedProofRefusesWithoutCheckout(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "proof-directory-test")
	for name, checker := range proofRetainingCheckers {
		t.Run(name, func(t *testing.T) {
			workingDirectory := t.TempDir()
			t.Chdir(workingDirectory)
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()

			err := checker(ctx, &interoplab.CheckContext{
				Source:  interoplab.ScenarioSource{Name: name},
				Network: proofTestNetwork,
				Lab:     noEvidenceLab{},
			})
			if err == nil || err.Error() != name+": no checkout to retain proof under" {
				t.Fatalf("checker error = %v, want the missing-checkout refusal", err)
			}
			entries, readErr := os.ReadDir(workingDirectory)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				t.Errorf("checker wrote %s under the working directory", entry.Name())
			}
		})
	}
}

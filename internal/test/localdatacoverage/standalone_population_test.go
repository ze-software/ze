// Design: docs/functional-tests.md -- every local-data registration owes runtime evidence.

package localdatacoverage

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestStandaloneParityEvidenceIncludesEveryNewRegistration pins the independently
// identified missing paths, rather than accepting a new total that could hide a
// replacement of an existing proof.
func TestStandaloneParityEvidenceIncludesEveryNewRegistration(t *testing.T) {
	covered := make(map[string]bool)
	for _, invocation := range Evidence() {
		covered[invocation.Evidence] = true
	}
	for _, path := range []string{
		"show config activate", "show config deactivate", "show config environment-validation",
		"show config fix-plan", "show config migrate", "show config replace", "show config set", "show config source",
		"show config supplied activate", "show config supplied completion", "show config supplied deactivate",
		"show config supplied diff", "show config supplied dump", "show config supplied fix-plan", "show config supplied fmt",
		"show config supplied graph", "show config supplied migrate", "show config supplied replace", "show config supplied set",
		"show config supplied source", "show config supplied validation", "show data encode",
		"show exabgp migration config", "show exabgp migration environment", "show secret encode", "show secret decode",
	} {
		if !covered[path] {
			t.Errorf("standalone parity path %q has no executable evidence", path)
		}
	}
}

// TestStandaloneReturnedConfigurationRequiresRequestedValueAndState proves
// the fixture rejects a wrong, missing or inactive value even when it parses.
func TestStandaloneReturnedConfigurationRequiresRequestedValueAndState(t *testing.T) {
	if err := requireStandaloneRouter(standaloneConfig, "203.0.113.11", false); err != nil {
		t.Fatalf("valid active control: %v", err)
	}
	if err := requireStandaloneRouter(standaloneInactive, "203.0.113.11", true); err != nil {
		t.Fatalf("valid inactive control: %v", err)
	}
	for name, content := range map[string]string{
		"wrong value": standaloneReplacement,
		"wrong state": standaloneInactive,
		"no section":  "environment {}\n",
		"no value":    "bgp {}\n",
		"malformed":   "bgp {",
	} {
		t.Run(name, func(t *testing.T) {
			if err := requireStandaloneRouter(content, "203.0.113.11", false); err == nil {
				t.Fatal("fixture accepted an answer without its requested active router-id")
			}
		})
	}
}

// TestStandaloneSuppliedContentPreservesEmptyAndPrivateBytes keeps the caller
// adapter honest independently of the encoded-content producer being tested.
func TestStandaloneSuppliedContentPreservesEmptyAndPrivateBytes(t *testing.T) {
	for _, literal := range []string{"", " \t\n", "private + operand", "\x00\xff\n"} {
		encoded, err := strconv.Unquote(strings.TrimPrefix(suppliedContent(literal), " content "))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if string(decoded) != literal {
			t.Fatalf("caller content changed: got %q, want %q", decoded, literal)
		}
	}
}

// TestStandaloneMigratedPeerRequiresAddressAndASN ensures a converted empty
// section or an unrelated peer cannot satisfy the binary walk's migration proof.
func TestStandaloneMigratedPeerRequiresAddressAndASN(t *testing.T) {
	for _, test := range []struct {
		name, address, asn string
		wantError          bool
	}{
		{"requested peer", "192.0.2.2", "65001", false},
		{"different peer", "192.0.2.3", "65001", true},
		{"different AS", "192.0.2.2", "65002", true},
		{"missing AS", "192.0.2.2", "", true},
		{"empty answer", "", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			bgp, group, peer := config.NewTree(), config.NewTree(), config.NewTree()
			if test.address != "" {
				connection, remote := config.NewTree(), config.NewTree()
				remote.Set("ip", test.address)
				connection.SetContainer("remote", remote)
				peer.SetContainer("connection", connection)
				group.AddListEntry("peer", "edge", peer)
				bgp.AddListEntry("group", "coverage", group)
			}
			if test.asn != "" {
				session, asn := config.NewTree(), config.NewTree()
				asn.Set("remote", test.asn)
				session.SetContainer("asn", asn)
				peer.SetContainer("session", session)
			}
			if err := requireMigratedPeer(bgp); (err != nil) != test.wantError {
				t.Fatalf("requireMigratedPeer() error=%v, want error=%t", err, test.wantError)
			}
		})
	}
}

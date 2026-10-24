package bgpconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// duplicateRemoteIPConfig holds two peers sharing one remote address, a bgp
// tree ResolveBGPTree refuses, so the bgp plugin extractor fails on it.
const duplicateRemoteIPConfig = `bgp {
  session { asn { local 65000; } }
  router-id 1.2.3.4;
  peer peer1 {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65001; } }
  }
  peer peer2 {
    connection { remote { ip 1.1.1.1; } }
    session { asn { remote 65002; } }
  }
}
`

// TestVerifyPluginConfigReportsExtractionFailureOnce proves that
// VerifyPluginConfig reports a plugin extraction failure once, typed and
// carrying the section whose extractor failed, and that the dependency refusal
// does not report the same failure a second time.
//
// VALIDATES: a bgp tree that does not resolve yields one
// *config.PluginExtractionError with Section "bgp" and no other copy of it.
// PREVENTS: the editor and `ze config validate` listing the bgp failure twice,
// and a door being unable to tell the relayed bgp copy from a plugin block
// failure.
func TestVerifyPluginConfigReportsExtractionFailureOnce(t *testing.T) {
	tree := parsedTree(t, duplicateRemoteIPConfig)

	var extracted []*config.PluginExtractionError
	copies := 0
	for _, err := range config.VerifyPluginConfig(tree) {
		if strings.Contains(err.Error(), "duplicate remote IP") {
			copies++
		}
		if extractErr, isExtraction := errors.AsType[*config.PluginExtractionError](err); isExtraction {
			extracted = append(extracted, extractErr)
		}
	}

	if copies != 1 {
		t.Fatalf("duplicate remote IP reported %d times, want once", copies)
	}
	if len(extracted) != 1 {
		t.Fatalf("got %d *config.PluginExtractionError, want 1", len(extracted))
	}
	if extracted[0].Section != string(config.ConfigTypeBGP) {
		t.Fatalf("extraction failure section = %q, want %q", extracted[0].Section, config.ConfigTypeBGP)
	}
}

// TestDoctorReportsBGPResolveFailureOnce proves that `ze doctor` names a bgp
// tree that does not resolve once, from the bgp peer check, and not a second
// time from the config-semantics check relaying the bgp extractor's failure.
//
// VALIDATES: config.ValidateSemantics (the config-semantics doctor check)
// yields no diagnostic for the duplicate remote IP, while
// validatePeersFromTree (the producer doctorCheckBGPPeerConfig reports) still
// refuses the tree with it.
// PREVENTS: the doctor report listing one bgp fault twice, under two codes.
func TestDoctorReportsBGPResolveFailureOnce(t *testing.T) {
	tree := parsedTree(t, duplicateRemoteIPConfig)

	for _, diag := range config.ValidateSemantics(tree) {
		if strings.Contains(diag.Message, "duplicate remote IP") {
			t.Fatalf("config-semantics relays the bgp resolve failure (%s): %s", diag.Code, diag.Message)
		}
	}

	err := validatePeersFromTree(tree.Clone())
	if err == nil {
		t.Fatal("validatePeersFromTree accepted a bgp tree with a duplicate remote IP")
	}
	if !strings.Contains(err.Error(), "duplicate remote IP") {
		t.Fatalf("bgp peer check error = %q, want it to name the duplicate remote IP", err)
	}
}

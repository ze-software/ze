// Design: docs/architecture/core-design.md -- Policy Filter Chain, the export step
// Related: export_library_answer_test.go -- exportLibraryAnswer, the export chain answer tests bind
// Related: filter_library_external_test.go -- textBody, the raw body an edit builds

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// probeExportChain answers an export policy of the one plugin filter probe:f,
// compiled at bgp export with a final accept as bgp/config compiles a peer's
// chain; r.filterTransportSeam (an externalTransport) answers the filter.
func probeExportChain(t *testing.T) filterapi.Policy {
	t.Helper()
	chain, err := policychain.Compile("bgp export", nil, []policychain.Member{{Name: "f", Plugin: "probe"}},
		routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mustPolicy(t, []routepolicy.Ref{{Name: "probe:f"}}, routepolicy.FinalAccept).WithLibrary(chain)
}

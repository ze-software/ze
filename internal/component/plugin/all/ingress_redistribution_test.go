// VALIDATES: no registered in-process ingress filter drops a peer's UPDATE
// because of what the redistribute block says.
//
// The method installs the destination-scoped evaluator the config loader
// builds. It then runs every filter filterapi.IngressOrdered holds over a real
// received UPDATE, and requires each one to accept.
//
// The test lives at the composition root. This package blank-imports the whole
// plugin set, so the pipeline here is the one a daemon runs. A test inside one
// filter's own package cannot see the pipeline. That is how
// bgp-redistribute came to reject every received route with green unit tests
// behind it: those tests built ImportRule values with an empty Destination,
// which the config loader never writes.
//
// PREVENTS: a redistribution rule gating BGP's own Adj-RIB-In. A rule states
// which routes move BETWEEN protocols. The per-consumer decision belongs to
// the orchestrator, which alone holds the importing protocol.

//go:build ze_bgp

package all

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
)

// receivedUpdateBody is the body of the UPDATE a peer sends for 10.0.0.0/24,
// captured from a daemon run of test/draft/plugin/redistribute-bgp-to-ospf-no-plumbing.ci.
// Withdrawn length 0, then 21 octets of attributes (ORIGIN igp, empty AS_PATH,
// NEXT_HOP 10.0.0.1, LOCAL_PREF 100), then the NLRI 10.0.0.0/24.
var receivedUpdateBody = []byte{
	0x00, 0x00,
	0x00, 0x15,
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x00,
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
	0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64,
	0x18, 0x0a, 0x00, 0x00,
}

func TestNoIngressFilterGatesAPeerRouteOnRedistributionConfig(t *testing.T) {
	// The rule the config loader builds for
	// `redistribute { destination ospf { import bgp { family [ ipv4/unicast ] } } }`.
	// Destination is populated for every rule a config produces.
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{
		Source:      "bgp",
		Destination: "ospf",
		Families:    []family.Family{family.IPv4Unicast},
	}}))
	defer configredist.SetGlobal(nil)

	filters := filterapi.IngressOrdered()
	require.NotEmpty(t, filters, "the composition root must register ingress filters")

	src := filterapi.PeerFilterInfo{LocalAS: 65000, PeerAS: 65001}
	for _, f := range filters {
		accept, _ := f.Ingress(src, receivedUpdateBody, make(map[string]any))
		require.Truef(t, accept,
			"ingress filter %q dropped a peer's route while a redistribute rule was configured", f.Name)
	}
}

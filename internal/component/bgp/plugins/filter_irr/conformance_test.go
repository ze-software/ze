package filter_irr

import (
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestIRRPassesConformance proves AC-36 for the IRR evaluator: it honors the
// fast-path contract on both transports (no blocking, the same answer twice,
// no view kept past the call, equal answers on the slot and over the batch,
// concurrent use under -race).
//
// VALIDATES: plug.evaluate, the evaluator OnFilter installs, passes
// sdk.CheckFilterConformance over routes covering each verdict: accept, a
// modify removing the refused prefix, a modify keeping a withdrawal, a whole
// reject, an ASN the config does not enroll, and fields that do not decode.
// PREVENTS: an evaluator correct only while the bridge loop serialized it, or
// one whose answer depends on the readmitter state its own earlier calls left.
func TestIRRPassesConformance(t *testing.T) {
	plug := readmitPlugin(t, fakeIRRv4(t, map[string]string{"AS-TEST": "10.0.0.0/24"}), newFakeReadmitEngine(), time.Hour)
	plug.refreshASN(65001)
	route := func(filter string, fields []byte) sdk.ConformanceRoute {
		return sdk.ConformanceRoute{Input: rpc.FilterUpdateInput{
			Filter: filter, Direction: "import", Peer: readmitTestPeer, PeerAS: 65001, Fields: fields,
		}}
	}
	routes := []sdk.ConformanceRoute{
		route("65001", announceFields("10.0.0.0/24")),
		route("65001", announceFields("10.0.0.0/24", "192.0.2.0/24")),
		route("65001", routeFields(map[string][]string{"ipv4/unicast": {"192.0.2.0/24"}},
			map[string][]string{"ipv4/unicast": {"198.51.100.0/24"}})),
		route("65001", announceFields("192.0.2.0/24")),
		route("65099", announceFields("10.0.0.0/24")),
		route("65001", []byte(`{"prefix":[]}`)),
	}
	if !sdk.CheckFilterConformance(t, "bgp-filter-irr", plug.evaluate, routes) {
		t.Fatal("the IRR evaluator breaks the fast-path contract")
	}
}

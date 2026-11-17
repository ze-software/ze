// Design: docs/architecture/api/process-protocol.md -- portable original attribute encoding.
package reactor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Exercise the actual local-build capture and portable callback projection. The
// fork must not resolve an engine-local numeric ContextID in its own registry.
func TestOriginIntakePreservesPortableOriginalAttributeEncoding(t *testing.T) {
	f, _, producer, source, _ := originReceiptFixture(t)
	asn2, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(false))
	require.NoError(t, err)
	asn4, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	for _, test := range []struct {
		name                 string
		context              bgpctx.ContextID
		known, asn4, builder bool
	}{
		{name: "builder", context: bgpctx.APIContextID, known: true, asn4: true, builder: true},
		{name: "two-octet", context: asn2, known: true},
		{name: "four-octet", context: asn4, known: true, asn4: true},
		{name: "unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			batch := originGapBatch()
			attributes := []byte{0x40, 1, 1, 0}
			if test.builder {
				batch.Attrs = attribute.NewBuilder().SetOrigin(0)
			} else {
				batch.Wire = attribute.NewAttributesWire(attributes, test.context)
			}
			build, err := captureOriginBuild(batch, batch.NLRIs[0])
			require.NoError(t, err)
			value := originIntakeValue(producer, source, build, f.peer, 13, rpc.OriginIntakeAnnounce)
			data, err := json.Marshal(value)
			require.NoError(t, err)
			var remote struct {
				Route map[string]json.RawMessage `json:"route"`
			}
			require.NoError(t, json.Unmarshal(data, &remote))
			require.Contains(t, remote.Route, "as-path-known", "unknown must be explicit, not a remote registry assumption")
			require.Contains(t, remote.Route, "as-path-asn4")
			var known, four bool
			require.NoError(t, json.Unmarshal(remote.Route["as-path-known"], &known))
			require.NoError(t, json.Unmarshal(remote.Route["as-path-asn4"], &four))
			require.Equal(t, test.known, known)
			require.Equal(t, test.asn4, four)
			require.Equal(t, uint16(test.context), value.Route.SourceContext, "portable facts do not replace original engine context")
			require.Equal(t, attributes, value.Route.Attributes)
			require.Equal(t, []byte{24, 10, 0, 0}, value.Route.KeyNLRI)
			attributes[3] = 2
			require.Equal(t, byte(0), value.Route.Attributes[3], "cross-boundary bytes retain independent ownership")
		})
	}
}

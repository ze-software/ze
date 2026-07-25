// VALIDATES: spec-ospf-10 AC-14 producer wiring -- RegisterProtocol("ospf") +
// RegisterProducer put OSPF in redistevents.Producers(); ProtocolIDOf("ospf")
// resolves; the producer reuses the Loc-RIB ProtocolID.
// PREVENTS: regressions where registering only the config source (not the producer)
// leaves OSPF out of Producers() so no route ever reaches the orchestrator.
package ospfredistevents

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/redistevents"
)

func TestOSPFProducerRegistered(t *testing.T) {
	require.True(t, producerRegistered, "package init must register the OSPF producer")

	id, ok := redistevents.ProtocolIDOf(Namespace)
	require.True(t, ok, "ProtocolIDOf(\"ospf\") must resolve")
	assert.Equal(t, ProtocolID, id, "the redistribute producer reuses the Loc-RIB ProtocolID")

	assert.True(t, slices.Contains(redistevents.Producers(), ProtocolID),
		"OSPF ProtocolID must appear in redistevents.Producers()")
}

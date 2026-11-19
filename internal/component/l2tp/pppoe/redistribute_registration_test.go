// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE routes
// enter redistribution under their own configured access source.
package pppoe

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"
)

func TestPPPoERedistributeSourceRegisteredAtInit(t *testing.T) {
	source, ok := redistribute.LookupSource("pppoe")
	require.True(t, ok, "offline configuration needs the PPPoE source before any subsystem starts")
	require.Equal(t, "pppoe", source.Name)
	require.Equal(t, "pppoe", source.Protocol)
	require.Equal(t, "pppoe", source.ConfigRoot)

	producer, ok := redistevents.ProtocolIDOf("pppoe")
	require.True(t, ok, "the PPPoE route topic needs its own protocol identity")
	require.Contains(t, redistevents.Producers(), producer, "the orchestrator must discover the PPPoE producer when it starts")
	l2tp, ok := redistevents.ProtocolIDOf("l2tp")
	require.True(t, ok)
	require.NotEqual(t, l2tp, producer, "PPPoE routes must not enter redistribute l2tp")
	require.ErrorIs(t, redistribute.RegisterSource(redistribute.RouteSource{Name: "pppoe", Protocol: "l2tp"}), redistribute.ErrSourceConflict)
}

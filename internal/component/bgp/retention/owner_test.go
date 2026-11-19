// Design: docs/guide/graceful-restart.md -- owner-bound retention verdicts.
package retention

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"math"
	"testing"
)

// The actual JSON encoder and ledger preserve full-width original owners,
// never a rounded/address-inferred identity, and late verdicts open no life.
func TestRetentionVerdictKeepsSameAddressOwner(t *testing.T) {
	const a, b = uint64(math.MaxUint64 - 1), uint64(math.MaxUint64)
	var ledger Ledger
	require.NoError(t, ledger.Begin(a, a))
	require.NoError(t, ledger.Begin(b, b))
	body, err := Encode(a, a, "192.0.2.240", []family.Family{family.IPv4Unicast})
	require.NoError(t, err)
	var raw Verdict
	require.NoError(t, json.Unmarshal([]byte(body), &raw))
	require.Equal(t, a, raw.PeerOwner)
	require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(body)}))
	sibling, err := ledger.Take(b, b)
	require.NoError(t, err)
	require.Empty(t, sibling, "A's verdict is not B's retention decision")
	kept, err := ledger.Take(a, a)
	require.NoError(t, err)
	require.True(t, kept[family.IPv4Unicast])
	require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(body)}))
	late, err := ledger.Take(a, a)
	require.NoError(t, err)
	require.Empty(t, late, "a late original verdict cannot reopen its consumed lifetime")
	require.Empty(t, ledger.verdicts)
	require.Error(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(`{"peer-address":"192.0.2.240","families":["ipv4/unicast"]}`)}))
	_, err = Encode(0, a, "192.0.2.240", nil)
	require.Error(t, err)
}

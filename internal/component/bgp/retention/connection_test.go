// Design: docs/guide/graceful-restart.md -- physical retention lifetimes.
package retention

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"math"
	"testing"
)

// A late old verdict cannot fill the successor, and a late Take for conn1 cannot
// consume conn2's useful verdict. Both identifiers survive actual JSON at uint64
// width; the admitted owner map remains one entry per actual source owner.
// MUTATION: admit a verdict or consume a lifetime by owner alone.
func TestRetentionSuccessorRejectsOldConnection(t *testing.T) {
	const owner, old, fresh = uint64(math.MaxUint64), uint64(math.MaxUint64 - 1), uint64(math.MaxUint64)
	var ledger Ledger
	require.NoError(t, ledger.Begin(owner, old))
	oldBody, err := Encode(owner, old, "192.0.2.244", []family.Family{family.IPv4Unicast})
	require.NoError(t, err)
	require.NoError(t, ledger.Begin(owner, fresh))
	require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(oldBody)}))
	unexpected, err := ledger.Take(owner, fresh)
	require.NoError(t, err)
	require.Empty(t, unexpected, "late conn1 must not fill conn2 before its own verdict")
	require.NoError(t, ledger.Begin(owner, fresh))
	body, err := Encode(owner, fresh, "192.0.2.244", []family.Family{family.IPv6Unicast})
	require.NoError(t, err)
	var decoded Verdict
	require.NoError(t, json.Unmarshal([]byte(body), &decoded))
	require.Equal(t, fresh, decoded.ConnectionID)
	require.Contains(t, body, `"connection-id":"18446744073709551615"`)
	require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(body)}))
	stale, err := ledger.Take(owner, old)
	require.NoError(t, err)
	require.Empty(t, stale)
	kept, err := ledger.Take(owner, fresh)
	require.NoError(t, err)
	require.Equal(t, map[family.Family]bool{family.IPv6Unicast: true}, kept)
	require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(body)}))
	late, err := ledger.Take(owner, fresh)
	require.NoError(t, err)
	require.Empty(t, late)
	require.Empty(t, ledger.verdicts, "completed lifetimes leave no history")
}

// Invalid boundaries are refused without replacing the already admitted useful
// successor. Missing/zero/string-overflow IDs are never guessed from address.
func TestRetentionRefusesMissingPhysicalConnection(t *testing.T) {
	var ledger Ledger
	require.NoError(t, ledger.Begin(35, 45))
	body, err := Encode(35, 45, "192.0.2.245", []family.Family{family.IPv4Unicast})
	require.NoError(t, err)
	require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(body)}))
	require.Error(t, ledger.Begin(35, 0))
	_, err = ledger.Take(35, 0)
	require.Error(t, err)
	_, err = Encode(35, 0, "192.0.2.245", nil)
	require.Error(t, err)
	for _, bad := range []string{`{"peer-owner":35,"peer-address":"192.0.2.245","families":[]}`, `{"peer-owner":35,"connection-id":"0","peer-address":"192.0.2.245","families":[]}`, `{"peer-owner":35,"connection-id":"18446744073709551616","peer-address":"192.0.2.245","families":[]}`} {
		require.Error(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(bad)}))
	}
	kept, err := ledger.Take(35, 45)
	require.NoError(t, err)
	require.Equal(t, map[family.Family]bool{family.IPv4Unicast: true}, kept)
}

// An owner-terminal removal consumes the departed owner's whole lifetime
// without inventing a physical wildcard; the same-address sibling survives.
func TestRetentionOwnerRemovalKeepsSiblingLifetime(t *testing.T) {
	var ledger Ledger
	require.NoError(t, ledger.Begin(51, 61))
	require.NoError(t, ledger.Begin(52, 62))
	for owner, connection := range map[uint64]uint64{51: 61, 52: 62} {
		body, err := Encode(owner, connection, "192.0.2.248", []family.Family{family.IPv4Unicast})
		require.NoError(t, err)
		require.NoError(t, ledger.Record(rpc.EventEnvelope{Payload: json.RawMessage(body)}))
	}
	require.Error(t, ledger.End(0))
	require.NoError(t, ledger.End(51))
	removed, err := ledger.Take(51, 61)
	require.NoError(t, err)
	require.Empty(t, removed)
	kept, err := ledger.Take(52, 62)
	require.NoError(t, err)
	require.Equal(t, map[family.Family]bool{family.IPv4Unicast: true}, kept)
	require.Empty(t, ledger.verdicts)
}

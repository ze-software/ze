// Design: docs/architecture/plugin/rib-storage-design.md -- empty validation batches retain provider lifecycle admission.
package adj_rib_in

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// emptyBatchAdmissionResult normalizes only the two existing public result
// shapes. It invokes the actual typed provider or its actual registered command.
func emptyBatchAdmissionResult(t *testing.T, r *AdjRIBInManager, typed bool) (*rpc.BatchValidateResult, error) {
	t.Helper()
	if typed {
		return r.handleBatchValidateTyped(nil)
	}
	status, data, err := r.handleCommand("request bgp adj-rib-in batch-validate", nil, "")
	if err != nil {
		require.Equal(t, statusError, status)
		return nil, err
	}
	require.Equal(t, statusDone, status)
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	var result rpc.BatchValidateResult
	require.NoError(t, json.Unmarshal(raw, &result))
	return &result, nil
}

// An empty batch still acknowledges the owner's admission/publication boundary.
// It may not acknowledge a removed, stopped or failed store, nor skip older
// owed publication. The useful sibling stays installed through the retry.
func TestBatchValidateEmptyBatchKeepsProviderAdmission(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "command"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			live := newTestManager(t)
			result, err := emptyBatchAdmissionResult(t, live, typed)
			require.NoError(t, err)
			require.Equal(t, &rpc.BatchValidateResult{}, result)

			t.Run("owed-publication", func(t *testing.T) {
				r := newTestManager(t)
				address := netip.MustParseAddr("192.0.2.1")
				source := receivedPeerOf(address, 1, 41)
				removed := compactRouteKey{Fam: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
				sibling := compactRouteKey{Fam: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.114.0/24")}
				r.ribIn[source] = newSeqMap()
				useful := &RawRoute{ConnectionID: 41, SourceOwner: 1, Family: sibling.Fam, MsgID: 102}
				r.ribIn[source].Put(removed, 101, &RawRoute{ConnectionID: 41, SourceOwner: 1, Family: removed.Fam, MsgID: 101})
				r.ribIn[source].Put(sibling, 102, useful)
				r.validationEnabled, r.validationGate = true, true
				refusal := errors.New("original validation publication refused")
				calls := 0
				r.emitValidation = func([]ribevents.ValidationRoute) error { calls++; return refusal }
				_, _, err := r.rejectRoutesCommand([]string{address.String(), "ipv4/unicast", removed.Prefix.String(), "0"})
				require.ErrorIs(t, err, refusal)
				_, present := r.ribIn[source].Get(removed)
				require.False(t, present, "the genuine provider mutation precedes its refused publication")
				require.Equal(t, 1, calls)
				require.Len(t, r.validationChanges, 1)
				expected := ribevents.ValidationRoute{Peer: address, PeerOwner: 1, ConnectionID: 41, Family: removed.Fam, Prefix: removed.Prefix}
				require.Equal(t, expected, r.validationChanges[0])
				result, err := emptyBatchAdmissionResult(t, r, typed)
				require.ErrorIs(t, err, refusal, "empty validation cannot acknowledge older publication debt")
				require.Nil(t, result)
				require.Equal(t, 2, calls)
				require.Equal(t, []ribevents.ValidationRoute{expected}, r.validationChanges)
				held, present := r.ribIn[source].Get(sibling)
				require.True(t, present)
				require.Same(t, useful, held)
				require.Nil(t, r.validationFailure(), "explicit retry remains recoverable")
				var published []ribevents.ValidationRoute
				r.emitValidation = func(changes []ribevents.ValidationRoute) error {
					calls++
					published = append(published, changes...)
					return nil
				}
				result, err = emptyBatchAdmissionResult(t, r, typed)
				require.NoError(t, err)
				require.Equal(t, &rpc.BatchValidateResult{}, result)
				require.Equal(t, []ribevents.ValidationRoute{expected}, published)
				require.Equal(t, 3, calls)
				require.Empty(t, r.validationChanges)
				require.Empty(t, r.validationOwed)
				require.NoError(t, r.validationEmitErr)
				held, present = r.ribIn[source].Get(sibling)
				require.True(t, present)
				require.Same(t, useful, held)
				_, present = r.ribIn[source].Get(removed)
				require.False(t, present, "retry publishes debt without restoring removed input")
			})
			for _, phase := range []string{"removed", "stopped", "failed"} {
				t.Run(phase, func(t *testing.T) {
					r := newTestManager(t)
					var want error
					switch phase {
					case "removed":
						require.NoError(t, r.removeValidationOwner("removed"))
						want = errReceiveStoreRemoved
					case "stopped":
						r.releaseValidationOwner()
						want = errReceiveStoreStopped
					default:
						want = errors.New("terminal owner publication failure")
						r.failValidationOwner(want)
					}
					result, err := emptyBatchAdmissionResult(t, r, typed)
					require.ErrorIs(t, err, want, "an empty batch cannot acknowledge a terminal receive-store owner")
					require.Nil(t, result)
					require.Empty(t, r.earlyDecisions)
					require.Empty(t, r.validationChanges)
				})
			}
		})
	}
}

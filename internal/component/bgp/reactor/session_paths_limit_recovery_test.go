// Design: docs/architecture/update-building.md -- the limited election a recovery round asks the RIB for
// Related: recovery_owed.go -- decide, which turns an ADD-PATH key into a limited request
// Related: session_paths_limit_elect.go -- pathsLimitHeld, which clears the contested marker

package reactor

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// recoveryLimitedRail is an ADD-PATH recovery rail whose destination session
// enforces a peer PATHS-LIMIT of 2 for IPv4 unicast, set before any route is
// written so every seeded path passes through the send admission.
func recoveryLimitedRail(t *testing.T) (*Reactor, *Peer, *Session, *recordingConn) {
	t.Helper()
	r, destination, conn := recoveryAddPathRail(t)
	session := destination.currentSession()
	session.writeMu.Lock()
	session.pathsLimit = sendBound(t)
	session.writeMu.Unlock()
	return r, destination, session, conn
}

// recoveryRequests records every request the RIB double was asked, in order.
type recoveryRequests struct {
	mu   sync.Mutex
	seen []ribevents.RecoveryRequest
}

func (r *recoveryRequests) add(request ribevents.RecoveryRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, request)
}

func (r *recoveryRequests) limits() []uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	limits := make([]uint16, 0, len(r.seen))
	for i := range r.seen {
		limits = append(limits, r.seen[i].Limit)
	}
	return limits
}

// TestPathsLimitRecoveryFallsBackOnLookupError proves D-8 in decide: when the
// RIB cannot answer a limited election, the round counts the failure, warns,
// and asks again with no limit, so the departed source's paths are still
// withdrawn from a peer that advertised PATHS-LIMIT. Method: a destination
// under a peer limit of 2 holds three identifiers from a source that departs;
// the RIB double refuses every limited request and answers the unlimited one
// with the identifiers it sent.
func TestPathsLimitRecoveryFallsBackOnLookupError(t *testing.T) {
	log := captureSessionLog(t, slog.LevelWarn)
	r, destination, _, conn := recoveryLimitedRail(t)
	want := [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}, {0, 0, 0, 17, 24, 192, 0, 3}}
	var seed [][]byte
	for _, raw := range want {
		seed = append(seed, ownershipRailBody(family.IPv4Unicast, false, raw, 0))
	}
	failed := recoveryAdvertiseFailed(t, r, destination, seed...)
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), len(want), "every seeded path is within the limit")
	recoveryOwnerStart(t, r, destination)
	requests := &recoveryRequests{}
	answer := recoveryAddPathAnswer(map[string][][]byte{
		string(syncOrderPrefixWire): want[:2], string([]byte{24, 192, 0, 3}): want[2:]})
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		requests.add(request)
		if request.Limit != 0 {
			return nil, errors.New("rib unavailable for a limited election")
		}
		return answer(request)
	})
	t.Cleanup(owner.Close)

	require.NoError(t, recoveryDepart(t, r, destination, failed.Settings().Address, "192.0.2.0/24", "192.0.3.0/24"))

	require.Equal(t, []uint16{2, 0}, requests.limits(),
		"the limited lookup fails, then the same keys are asked with no limit")
	var got [][]byte
	for _, written := range recoveryWrittenUpdates(t, conn.written()[len(setup):]) {
		require.Empty(t, written.NLRI)
		got = append(got, written.WithdrawnRoutes)
	}
	require.ElementsMatch(t, want, got, "the unlimited answer withdraws every departed identifier")
	require.Equal(t, 1, strings.Count(log.String(), "could not rank the paths"),
		"the failed election is warned once:\n%s", log.String())
}

// TestPathsLimitRecoveryClearsContestedUnderBound proves that a limited
// election whose RIB answer holds fewer survivors than the peer's limit
// clears the route's contested marker: no refused path is left better than
// what the peer holds, so later writes owe no election for it. Method: two
// paths fill a limit of 2, a third from the same live source is withheld and
// owes an election, and the RIB answers that limited request with no
// survivor. The held paths stay, so the route keeps its admission entry.
func TestPathsLimitRecoveryClearsContestedUnderBound(t *testing.T) {
	r, destination, session, conn := recoveryLimitedRail(t)
	source, _ := newAnnouncePeer(t, "192.0.2.10")
	source.recvCtxID = destination.sendContextID()
	r.peers[source.Settings().PeerKey()] = source
	for _, raw := range [][]byte{{0, 0, 0, 0, 24, 192, 0, 2}, {0, 0, 0, 7, 24, 192, 0, 2}} {
		require.NoError(t, ownershipWriterForward(t, destination, source,
			ownershipRailBody(family.IPv4Unicast, false, raw, 0), false))
	}
	recoveryOwnerStart(t, r, destination)
	attachRecoveryOwner(t, r, source)
	requests := &recoveryRequests{}
	owner := ribevents.PublishRecovery(func(request ribevents.RecoveryRequest) ([]ribevents.RecoveryOutcome, error) {
		requests.add(request)
		outcomes := make([]ribevents.RecoveryOutcome, len(request.Keys))
		for i := range request.Keys {
			outcomes[i] = ribevents.RecoveryOutcome{NLRI: request.Keys[i].NLRI, Withdraw: true}
		}
		return outcomes, nil
	})
	t.Cleanup(owner.Close)
	contested := func() bool {
		session.writeMu.Lock()
		defer session.writeMu.Unlock()
		prefix := electPrefix(t, session.pathsLimit, "192.0.2.0/24")
		require.NotNil(t, prefix, "the route keeps its two admitted paths")
		return prefix.contested
	}
	setup := conn.written()
	require.Len(t, recoveryWrittenUpdates(t, setup), 2, "both paths fit the limit")

	require.NoError(t, ownershipWriterForward(t, destination, source,
		ownershipRailBody(family.IPv4Unicast, false, []byte{0, 0, 0, 9, 24, 192, 0, 2}, 0), false))

	lowEventually(t, func() bool { return len(requests.limits()) == 1 }, "the withheld path owes one election")
	require.Equal(t, []uint16{2}, requests.limits(), "the election is limited to the peer's bound")
	lowEventually(t, func() bool { return !contested() }, "no survivor under the limit clears the contested marker")
	require.Empty(t, recoveryWrittenUpdates(t, conn.written()[len(setup):]),
		"the RIB named no path, so the live source's held paths stay and nothing is written")
}

// Design: docs/architecture/l2tp/subscriber-session-model.md -- removed sessions keep their admission slot
// Related: session_retirement.go -- retained roots, the prune that frees their slots
// Related: session.go -- sessionSlotsFull, the admission check every call path shares

package l2tp

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// cleanupSlotRig is one reactor and one established tunnel it bound, as the
// SCCRQ path and the dial path bind theirs, with max-sessions 2.
type cleanupSlotRig struct {
	reactor *l2tpReactor
	tunnel  *L2TPTunnel
}

func newCleanupSlotRig(t *testing.T, maxTunnels uint16) *cleanupSlotRig {
	t.Helper()
	reactor := &l2tpReactor{
		logger:           slog.Default(),
		params:           reactorParams{MaxSessions: 2, MaxTunnels: maxTunnels},
		tunnelsByLocalID: make(map[uint16]*L2TPTunnel),
		tunnelsByPeer:    make(map[peerKey]*L2TPTunnel),
	}
	tunnel := newEstablishedTunnel(t, 0)
	reactor.bindTunnelAdmissionLocked(tunnel)
	reactor.tunnelsByLocalID[tunnel.localTID] = tunnel
	reactor.tunnelsByPeer[tunnel.peerIndexKey] = tunnel
	t.Cleanup(func() {
		clearCleanupHeldSessionSlots(tunnel.localTID)
		clearCleanupHeldTunnelSlots()
	})
	return &cleanupSlotRig{reactor: reactor, tunnel: tunnel}
}

// fillWithUnconfirmedCleanup admits two sessions whose PPP construction has
// not finished, removes both, and hands the downs to the reactor's retention,
// the path deliverSessionDowns runs. It returns the two constructions, so a
// test can confirm one cleanup.
func (rig *cleanupSlotRig) fillWithUnconfirmedCleanup(t *testing.T) []*pppConstruction {
	t.Helper()
	var constructions []*pppConstruction
	for _, sid := range []uint16{11, 12} {
		construction := newPPPConstruction()
		constructions = append(constructions, construction)
		rig.tunnel.addSession(&L2TPSession{
			localSID: sid, remoteSID: sid + 100, state: L2TPSessionWaitConnect,
			pppConstruction: construction, fsmHistory: newFSMHistoryRing(),
		})
	}
	for _, sid := range []uint16{11, 12} {
		rig.tunnel.removeSession(sid, l2tpevents.TerminateCauseAdminReset)
	}
	_, downs := rig.tunnel.drainPendingTeardowns()
	require.Len(t, downs, 2)
	rig.reactor.retainRemovedSessions(downs)
	require.Zero(t, rig.tunnel.sessionCount(), "the removed sessions left the live map")
	require.Len(t, rig.reactor.retiredSessions, 2, "both unconfirmed roots are retained")
	return constructions
}

// confirmOne finishes one construction with no cleanup error: its PPP
// cleanup is now confirmed, and no native owner exists in this rig.
func confirmOne(construction *pppConstruction) {
	construction.finish(nil, nil)
}

func requireHeldSlotsWarning(t *testing.T, subject string, want bool) {
	t.Helper()
	found := false
	for _, issue := range report.Warnings() {
		if issue.Source != ppp.CleanupReportSource {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorPPPCleanup {
			continue
		}
		if issue.Subject == subject {
			found = true
			require.NotEmpty(t, issue.Fix, "the warning names its fix")
		}
	}
	require.Equal(t, want, found, "doctor-ppp-cleanup warning %q", subject)
}

func icrqBody(remoteSID uint16) []byte {
	return buildICRQ(remoteSID, uint32(remoteSID))
}

func ocrqBody(remoteSID uint16) []byte {
	return bodyOf(MsgOCRQ,
		u16AVP(AVPAssignedSessionID, remoteSID), u32AVP(AVPCallSerialNumber, uint32(remoteSID)),
		u32AVP(AVPMinimumBPS, 9600), u32AVP(AVPMaximumBPS, 64000),
		u32AVP(AVPBearerType, 1), u32AVP(AVPFramingType, 1), strAVP(AVPCalledNumber, "5551000"))
}

// TestSession_UnconfirmedCleanupHoldsSlotICRQ proves a removed session whose
// cleanup is unconfirmed keeps its tunnel's admission slot, as
// ppp.SessionControl.CloseCanceledOwner requires. Two such sessions fill a
// max-sessions-2 tunnel: an ICRQ is refused with CDN Result Code 4, no
// session is added and a doctor-ppp-cleanup warning names the cause. Once one
// cleanup is confirmed, the next ICRQ is admitted at once, with no other
// prune between, and the warning clears when the last slot frees.
func TestSession_UnconfirmedCleanupHoldsSlotICRQ(t *testing.T) {
	rig := newCleanupSlotRig(t, 0)
	constructions := rig.fillWithUnconfirmedCleanup(t)
	now := time.Now()
	logger := slog.Default()

	out := rig.tunnel.handleICRQ(icrqBody(501), now, logger)
	require.Len(t, out, 1, "the refused ICRQ is answered by one CDN")
	requireCDN(t, out[0].bytes, cdnResultNoResources, 0)
	require.Zero(t, rig.tunnel.sessionCount(), "a refused ICRQ adds no session")
	requireHeldSlotsWarning(t, cleanupHeldSessionSlotsSubject(rig.tunnel.localTID), true)

	confirmOne(constructions[0])
	rig.tunnel.handleICRQ(icrqBody(502), now, logger)
	require.Equal(t, 1, rig.tunnel.sessionCount(), "a confirmed cleanup frees its slot at the next admission")
	require.Len(t, rig.reactor.retiredSessions, 1, "only the unconfirmed root stays retained")
	require.Equal(t, 1, rig.tunnel.retainedCleanups)

	confirmOne(constructions[1])
	rig.reactor.tunnelsMu.Lock()
	rig.reactor.pruneRetiredSessionsLocked()
	rig.reactor.tunnelsMu.Unlock()
	require.Zero(t, rig.tunnel.retainedCleanups)
	requireHeldSlotsWarning(t, cleanupHeldSessionSlotsSubject(rig.tunnel.localTID), false)
}

// TestSession_UnconfirmedCleanupHoldsSlotOCRQ is the OCRQ half: the same two
// unconfirmed removals refuse an OCRQ with CDN Result Code 4, and a confirmed
// cleanup admits the next one.
func TestSession_UnconfirmedCleanupHoldsSlotOCRQ(t *testing.T) {
	rig := newCleanupSlotRig(t, 0)
	constructions := rig.fillWithUnconfirmedCleanup(t)
	now := time.Now()
	logger := slog.Default()

	out := rig.tunnel.handleOCRQ(ocrqBody(601), now, logger)
	require.Len(t, out, 1, "the refused OCRQ is answered by one CDN")
	requireCDN(t, out[0].bytes, cdnResultNoResources, 0)
	require.Zero(t, rig.tunnel.sessionCount(), "a refused OCRQ adds no session")

	confirmOne(constructions[1])
	rig.tunnel.handleOCRQ(ocrqBody(602), now, logger)
	require.Equal(t, 1, rig.tunnel.sessionCount(), "a confirmed cleanup frees its slot for an OCRQ")
}

// TestSession_UnconfirmedCleanupHoldsSlotLocalCall is the local initiation
// half: placeIncomingCall and placeOutgoingCall are both refused while the
// unconfirmed removals hold every slot, and a confirmed cleanup lets a call
// be placed.
func TestSession_UnconfirmedCleanupHoldsSlotLocalCall(t *testing.T) {
	rig := newCleanupSlotRig(t, 0)
	constructions := rig.fillWithUnconfirmedCleanup(t)
	now := time.Now()
	logger := slog.Default()
	params := callParams{callSerial: 7, bearerType: 1, framingType: 1, calledNumber: "5559999"}

	localSID, sends := rig.tunnel.placeIncomingCall(now, params, logger)
	require.Zero(t, localSID, "an incoming call is refused while removed sessions hold every slot")
	require.Empty(t, sends)
	localSID, sends = rig.tunnel.placeOutgoingCall(now, params, logger)
	require.Zero(t, localSID, "an outgoing call is refused while removed sessions hold every slot")
	require.Empty(t, sends)
	require.Zero(t, rig.tunnel.sessionCount())

	confirmOne(constructions[0])
	localSID, _ = rig.tunnel.placeIncomingCall(now, params, logger)
	require.NotZero(t, localSID, "a confirmed cleanup frees its slot for a local call")
	require.Equal(t, 1, rig.tunnel.sessionCount())
}

// TestSession_ConfirmedCleanupFreesSlotAtOnce is the negative case: removed
// sessions whose cleanup is already confirmed hold no slot past the next
// admission, so a full tunnel that ends its calls admits new ones.
func TestSession_ConfirmedCleanupFreesSlotAtOnce(t *testing.T) {
	rig := newCleanupSlotRig(t, 0)
	constructions := rig.fillWithUnconfirmedCleanup(t)
	confirmOne(constructions[0])
	confirmOne(constructions[1])
	now := time.Now()
	logger := slog.Default()

	rig.tunnel.handleICRQ(icrqBody(701), now, logger)
	require.Equal(t, 1, rig.tunnel.sessionCount())
	require.Zero(t, rig.tunnel.retainedCleanups)
	require.Empty(t, rig.reactor.retiredSessions)
	requireHeldSlotsWarning(t, cleanupHeldSessionSlotsSubject(rig.tunnel.localTID), false)
}

// TestReactor_DiscardedTunnelWithUnconfirmedCleanupHoldsTunnelSlot proves the
// bound survives the tunnel's own teardown: a discarded tunnel whose removed
// sessions' cleanup is unconfirmed keeps its max-tunnels slot, so a LAC that
// churns tunnels cannot add retained roots past max-tunnels times
// max-sessions. With max-tunnels 1, a new tunnel is refused and the reason is
// a doctor-ppp-cleanup warning; once the cleanup is confirmed, the slot frees
// at the next tunnel admission.
func TestReactor_DiscardedTunnelWithUnconfirmedCleanupHoldsTunnelSlot(t *testing.T) {
	rig := newCleanupSlotRig(t, 1)
	construction := newPPPConstruction()
	rig.tunnel.addSession(&L2TPSession{
		localSID: 21, remoteSID: 121, state: L2TPSessionWaitConnect,
		pppConstruction: construction, fsmHistory: newFSMHistoryRing(),
	})
	peer := slog.String("from", "192.0.2.1:1701")

	rig.reactor.tunnelsMu.Lock()
	_, downs := rig.reactor.discardTunnelLocked(rig.tunnel, "test")
	rig.reactor.tunnelsMu.Unlock()
	require.Len(t, downs, 1)
	rig.reactor.retainRemovedSessions(downs)
	require.Empty(t, rig.reactor.tunnelsByLocalID, "the discarded tunnel left the map")
	require.Equal(t, 1, rig.reactor.retainingTunnels)

	rig.reactor.tunnelsMu.Lock()
	full := rig.reactor.tunnelSlotsFullLocked("SCCRQ rejected", peer)
	rig.reactor.tunnelsMu.Unlock()
	require.True(t, full, "a discarded tunnel holding unconfirmed cleanup keeps its max-tunnels slot")
	requireHeldSlotsWarning(t, cleanupHeldTunnelSlotsSubject, true)

	confirmOne(construction)
	rig.reactor.tunnelsMu.Lock()
	full = rig.reactor.tunnelSlotsFullLocked("SCCRQ rejected", peer)
	rig.reactor.tunnelsMu.Unlock()
	require.False(t, full, "a confirmed cleanup frees the tunnel slot at the next admission")
	require.Zero(t, rig.reactor.retainingTunnels)
	requireHeldSlotsWarning(t, cleanupHeldTunnelSlotsSubject, false)
}

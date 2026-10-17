// Design: docs/architecture/testing/interop.md -- live receive-only attach and BFD opt-in against FRR.
// Related: check_rpki_reload.go -- applyZeReload, the SIGHUP reload these checkers drive.
// Related: check_bfd_profile_edit.go -- the FRR BFD peer readers and the continuity check reused here.
// RFC: rfc/short/rfc2918.md Section 4
package bgp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	attachReceiveOnlyLiveScenario = "bgp-attach-receive-only-live-frr"
	bfdEnableLiveScenario         = "bgp-bfd-enable-live-frr"
	bfdDynamicEnableLiveScenario  = "bgp-bfd-dynamic-enable-live-frr"

	// attachLivePrefix is the route FRR announces before the reload, and
	// attachLiveRouteKey its key in `show bgp adj-rib-in`.
	attachLivePrefix   = "10.77.0.0/24"
	attachLiveRouteKey = "ipv4/unicast:" + attachLivePrefix

	// bfdEnableEditConfig is the second reload of bgp-bfd-enable-live-frr,
	// the profile edit, mounted by prepare.go under its own name.
	bfdEnableEditConfig = "ze-reload-edit.conf"
)

// RFC 2918 Section 4
//
// checkAttachReceiveOnlyLive proves that attaching a receive-only process to a
// running session keeps that session with a foreign speaker and brings the
// process level with what the speaker already sent: FRR receives exactly one
// ROUTE-REFRESH (one negotiated family), re-sends its route, ze's newly
// attached adj-rib-in holds it, and FRR's connectionsEstablished never moves.
func checkAttachReceiveOnlyLive(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("attach receive-only scenario has no selected IPv4 network")
	}
	neighbor := networkHostAddress(check.Network, 2)
	source := networkHostAddress(check.Network, 3)
	var text textbuf.Buffer
	session := []string{cmdVtysh, "-c", text.Str("show bgp neighbor ").Str(neighbor).String()}
	advertised := []string{cmdVtysh, "-c", text.Reset().Str("show bgp ipv4 unicast neighbors ").Str(neighbor).Str(" advertised-routes").String()}
	const scenario = attachReceiveOnlyLiveScenario

	if err := waitContains(ctx, check.Lab, peerFRR, session, 90*time.Second, "BGP state = Established"); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 1, err)
	}
	if err := waitContains(ctx, check.Lab, peerFRR, advertised, 60*time.Second, attachLivePrefix); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 2, fmt.Errorf("FRR never announced %s to ze before the reload: %w", attachLivePrefix, err))
	}
	held, err := zeAdjRIBInHolds(ctx, check.Lab, source, attachLiveRouteKey)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 3, err)
	}
	if held {
		return checkerFailure(ctx, check.Lab, scenario, 3, errors.New("adj-rib-in holds FRR's route before any peer attaches it, so the catch-up would prove nothing"))
	}
	before, err := readFRRSessionCounters(ctx, check.Lab, neighbor)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 4, err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 5, fmt.Errorf("reload attaching adj-rib-in: %w", err))
	}
	if _, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "adj-rib-in holding FRR's re-sent route",
	}, func(probeCtx context.Context) ([]rpkiReloadRoute, error) {
		// The routes, not a bool, so a timeout names what adj-rib-in held instead.
		return zeAdjRIBInRoutes(probeCtx, check.Lab, source)
	}, func(routes []rpkiReloadRoute) bool { return adjRIBInHolds(routes, attachLiveRouteKey) }); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 6, fmt.Errorf("the attached adj-rib-in never received %s: %w", attachLivePrefix, err))
	}
	after, err := readFRRSessionCounters(ctx, check.Lab, neighbor)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 7, err)
	}
	if err := requireOneRefreshSameSession(before, after); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 8, err)
	}
	return nil
}

// RFC 5882 Section 8
//
// checkBFDEnableLive proves a configured peer's non-strict BFD opt-in can be
// added and edited by reload while FRR keeps its BGP session to ze, and
// removed by reload with ze telling FRR it went AdminDown: after the add FRR's
// BFD session is Up at ze's 300ms with connectionsEstablished unmoved, after
// the edit it is the same Up session at 200ms and multiplier 5 (RFC 5880
// Section 6.8.3), and after the removal FRR's BFD session is Down with
// ze's diagnostic Administratively Down (removeAndSignalAdminDown).
func checkBFDEnableLive(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("BFD enable scenario has no selected IPv4 network")
	}
	const scenario = bfdEnableLiveScenario
	lab := newBFDLiveLab(check)
	generation, err := lab.established(ctx)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 1, err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 2, fmt.Errorf("reload adding the opt-in: %w", err))
	}
	if err := waitBFDRemoteTimers(ctx, check.Lab, lab.peer, bfdProfileEditBefore); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 3, fmt.Errorf("FRR's BFD session never came Up at ze's profile after the add: %w", err))
	}
	added, err := readBFDPeerIdentity(ctx, check.Lab, lab.neighbor, lab.peer, lab.counters)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 4, err)
	}
	if err := requireSameBGPGeneration("the add", generation, added.bgpGeneration); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 4, err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedStepReloadConfig(bfdEnableEditConfig)); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 5, fmt.Errorf("reload editing the profile: %w", err))
	}
	if err := waitBFDRemoteTimers(ctx, check.Lab, lab.peer, bfdProfileEditAfter); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 6, fmt.Errorf("FRR never read the edited profile from ze: %w", err))
	}
	edited, err := readBFDPeerIdentity(ctx, check.Lab, lab.neighbor, lab.peer, lab.counters)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 7, err)
	}
	if err := requireBFDProfileEditContinuity(added, edited); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 7, err)
	}
	if err := lab.removeAndSignalAdminDown(ctx); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 8, err)
	}
	return nil
}

// RFC 5882 Section 8
//
// checkBFDDynamicEnableLive proves the same add and removal for a dynamic peer,
// through its group template: FRR connects into ze's dynamic range, the
// template gains the opt-in by reload and FRR's BFD session comes Up with
// connectionsEstablished unmoved, then a reload back to ze.conf removes it and
// FRR's BFD session goes Down on ze's AdminDown (removeAndSignalAdminDown).
func checkBFDDynamicEnableLive(ctx context.Context, check *interoplab.CheckContext) error {
	if !check.Network.IPv4.IsValid() {
		return errors.New("BFD dynamic enable scenario has no selected IPv4 network")
	}
	const scenario = bfdDynamicEnableLiveScenario
	lab := newBFDLiveLab(check)
	generation, err := lab.established(ctx)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 1, err)
	}
	if err := applyZeReload(ctx, check.Lab, zeMountedReloadConfig); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 2, fmt.Errorf("reload adding the template opt-in: %w", err))
	}
	if err := waitBFDRemoteTimers(ctx, check.Lab, lab.peer, bfdProfileEditBefore); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 3, fmt.Errorf("FRR's BFD session never came Up at ze's profile after the add: %w", err))
	}
	added, err := readBFDPeerIdentity(ctx, check.Lab, lab.neighbor, lab.peer, lab.counters)
	if err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 4, err)
	}
	if err := requireSameBGPGeneration("the add", generation, added.bgpGeneration); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 4, err)
	}
	if err := lab.removeAndSignalAdminDown(ctx); err != nil {
		return checkerFailure(ctx, check.Lab, scenario, 5, err)
	}
	return nil
}

// bfdLiveLab holds the FRR commands the BFD keep-session checkers read.
type bfdLiveLab struct {
	check    *interoplab.CheckContext
	neighbor string
	session  []string
	peer     []string
	counters []string
}

func newBFDLiveLab(check *interoplab.CheckContext) bfdLiveLab {
	neighbor := networkHostAddress(check.Network, 2)
	var text textbuf.Buffer
	return bfdLiveLab{
		check:    check,
		neighbor: neighbor,
		session:  []string{cmdVtysh, "-c", text.Str("show bgp neighbor ").Str(neighbor).String()},
		peer:     []string{cmdVtysh, "-c", text.Reset().Str("show bfd peer ").Str(neighbor).Str(" json").String()},
		counters: []string{cmdVtysh, "-c", text.Reset().Str("show bfd peer ").Str(neighbor).Str(" counters json").String()},
	}
}

// established waits for FRR's BGP session to ze and returns its generation.
func (l bfdLiveLab) established(ctx context.Context) (uint64, error) {
	if err := waitContains(ctx, l.check.Lab, peerFRR, l.session, 90*time.Second, "BGP state = Established"); err != nil {
		return 0, err
	}
	return queryFRRSessionGeneration(ctx, l.check.Lab, l.neighbor)
}

// removeAndSignalAdminDown reloads ze back to ze.conf, which has no opt-in,
// and waits until FRR's BFD peer reads Down with ze's diagnostic
// Administratively Down, which is how a peer that released its session must
// leave it rather than going silent.
//
// It does not assert that FRR's BGP session survives the removal. RFC 5882
// Section 4.2 says the opposite of what FRR 10.3.1 does: "If a BFD session
// transitions from Up state to AdminDown, or the session transitions from Up
// to Down because the remote system is indicating that the session is in state
// AdminDown, clients SHOULD NOT take any control protocol action." FRR's bgpd
// sends Cease/BFD Down on ze's AdminDown anyway. That is a recorded peer
// deviation (spec A-5, owner decision 2026-10-13), so the proof stops at
// what ze puts on the wire.
func (l bfdLiveLab) removeAndSignalAdminDown(ctx context.Context) error {
	if err := applyZeReload(ctx, l.check.Lab, zeMountedConfig); err != nil {
		return fmt.Errorf("reload removing the opt-in: %w", err)
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "FRR BFD session Down on ze's AdminDown",
	}, func(probeCtx context.Context) (string, error) {
		return l.check.Lab.Query(probeCtx, peerFRR, l.peer, nil)
	}, func(output string) bool {
		reading, readErr := readFRRBFDDown(output)
		return readErr == nil && reading.adminDownFromRemote()
	})
	if err != nil {
		return fmt.Errorf("FRR's BFD peer never read Down with remote diagnostic %q after ze removed the opt-in: %w", frrDiagAdminDown, err)
	}
	return nil
}

// frrBFDStatusDown is FRR's status for a BFD session in state Down.
const frrBFDStatusDown = "down"

// frrDiagAdminDown is FRR's name for BFD diagnostic 7, Administratively Down
// (RFC 5880 Section 4.1), in `show bfd peer <addr> json`.
const frrDiagAdminDown = "administratively down"

// frrBFDDownReading is FRR's own BFD state for a peer and the diagnostic the
// peer last sent it.
type frrBFDDownReading struct {
	status           string
	remoteDiagnostic string
}

// adminDownFromRemote reports whether FRR's session went Down because the
// remote end, ze, signaled AdminDown. RFC 5880 Section 6.8.6: "If received
// state is AdminDown / If bfd.SessionState is not Down / Set bfd.LocalDiag to
// 3 (Neighbor signaled session down) / Set bfd.SessionState to Down". The
// state is Down either way, so only the remote diagnostic tells ze's AdminDown
// from ze going silent (FRR's own diagnostic then reads detection time
// expired).
func (r frrBFDDownReading) adminDownFromRemote() bool {
	if r.status != frrBFDStatusDown {
		return false
	}
	return r.remoteDiagnostic == frrDiagAdminDown
}

// readFRRBFDDown reads the status and remote-diagnostic fields of FRR's
// `show bfd peer <addr> json`, refusing output that lacks either.
func readFRRBFDDown(output string) (frrBFDDownReading, error) {
	var peer frrBFDPeer
	if err := json.Unmarshal([]byte(output), &peer); err != nil {
		return frrBFDDownReading{}, fmt.Errorf("decode FRR BFD peer JSON: %w; output: %s", err, strings.TrimSpace(output))
	}
	if peer.Status == nil {
		return frrBFDDownReading{}, fmt.Errorf("FRR BFD peer JSON has no status: %s", strings.TrimSpace(output))
	}
	if peer.RemoteDiagnostic == nil {
		return frrBFDDownReading{}, fmt.Errorf("FRR BFD peer JSON has no remote-diagnostic: %s", strings.TrimSpace(output))
	}
	return frrBFDDownReading{status: *peer.Status, remoteDiagnostic: *peer.RemoteDiagnostic}, nil
}

// requireSameBGPGeneration refuses a BGP session FRR re-established across
// the named reload.
func requireSameBGPGeneration(reload string, before, after uint64) error {
	if before == after {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("FRR BGP connectionsEstablished moved from ").Uint(before).Str(" to ").Uint(after).
		Str(" across ").Str(reload).Str(", so ze reset the session").String())
}

// frrSessionCounters are the two FRR neighbor counters the attach checker
// compares across the reload.
type frrSessionCounters struct {
	generation      uint64
	refreshReceived uint64
}

// readFRRSessionCounters reads FRR's connectionsEstablished and the count of
// ROUTE-REFRESH messages it received from neighbor.
func readFRRSessionCounters(ctx context.Context, lab interoplab.CheckerLab, neighbor string) (frrSessionCounters, error) {
	output, err := lab.Query(ctx, peerFRR, []string{cmdVtysh, "-c", "show bgp neighbor " + neighbor + " json"}, nil)
	if err != nil {
		return frrSessionCounters{}, err
	}
	return parseFRRSessionCounters(output, neighbor)
}

// parseFRRSessionCounters decodes connectionsEstablished and
// messageStats.routeRefreshRecv from FRR's neighbor JSON, refusing a session
// that is not Established and a counter FRR did not print.
func parseFRRSessionCounters(output, neighbor string) (frrSessionCounters, error) {
	generation, err := parseFRRSessionGeneration(output, neighbor)
	if err != nil {
		return frrSessionCounters{}, err
	}
	var peers map[string]struct {
		MessageStats map[string]json.RawMessage `json:"messageStats"`
	}
	if err := json.Unmarshal([]byte(output), &peers); err != nil {
		return frrSessionCounters{}, fmt.Errorf("decode FRR neighbor JSON: %w", err)
	}
	stats := peers[neighbor].MessageStats
	raw, ok := stats["routeRefreshRecv"]
	if !ok {
		keys := make([]string, 0, len(stats))
		for key := range stats {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		return frrSessionCounters{}, fmt.Errorf("FRR neighbor %s messageStats has no routeRefreshRecv; keys: %s", neighbor, strings.Join(keys, ","))
	}
	var refreshReceived uint64
	if err := json.Unmarshal(raw, &refreshReceived); err != nil {
		return frrSessionCounters{}, fmt.Errorf("decode FRR routeRefreshRecv %s: %w", raw, err)
	}
	return frrSessionCounters{generation: generation, refreshReceived: refreshReceived}, nil
}

// requireOneRefreshSameSession refuses a re-established session and any
// ROUTE-REFRESH count other than one more: one negotiated family, one refresh.
func requireOneRefreshSameSession(before, after frrSessionCounters) error {
	if err := requireSameBGPGeneration("the attach", before.generation, after.generation); err != nil {
		return err
	}
	if after.refreshReceived == before.refreshReceived+1 {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("FRR received ").Uint(after.refreshReceived - before.refreshReceived).
		Str(" ROUTE-REFRESH across the attach, want exactly 1 (ipv4/unicast is the one negotiated family)").String())
}

// zeAdjRIBInHolds reports whether ze's adj-rib-in holds key for source.
func zeAdjRIBInHolds(ctx context.Context, lab interoplab.CheckerLab, source, key string) (bool, error) {
	routes, err := zeAdjRIBInRoutes(ctx, lab, source)
	if err != nil {
		return false, err
	}
	return adjRIBInHolds(routes, key), nil
}

// zeAdjRIBInRoutes reads the routes ze's adj-rib-in holds for source.
func zeAdjRIBInRoutes(ctx context.Context, lab interoplab.CheckerLab, source string) ([]rpkiReloadRoute, error) {
	var received struct {
		Peers map[string][]rpkiReloadRoute `json:"adj-rib-in"`
	}
	if err := queryRPKIReloadJSON(ctx, lab, "show bgp adj-rib-in", &received); err != nil {
		return nil, err
	}
	return received.Peers[source], nil
}

// adjRIBInHolds reports whether routes carries key.
func adjRIBInHolds(routes []rpkiReloadRoute, key string) bool {
	for _, route := range routes {
		if route.Key == key {
			return true
		}
	}
	return false
}

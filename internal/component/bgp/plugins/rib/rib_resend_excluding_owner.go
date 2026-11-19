// Design: docs/architecture/plugin/rib-storage-design.md -- exact destination exclusion.
// Related: rib_selected_refresh.go -- the existing ordinary destination worker.
// Related: rib_replay.go -- immutable SENT group encoding for auxiliary replay.
package rib

import (
	"cmp"
	"context"
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const ownerExcludedResendCommand = "clear bgp rib out-excluding-owner"

// ownerExcludedResend is a parsed, nonzero destination exclusion. Its zero is
// invalid. A zero family is the explicit optional all-family scope; it is not
// permission to choose a destination or bypass the captured UP receipt.
type ownerExcludedResend struct {
	owner  uint64
	family family.Family
}

func ownerExcludedResendRefusal(cause error) error {
	return opererr.Wrap(cause, "The owner-excluded route replay could not complete",
		"Replay must preserve each destination's original owner and live UP receipt",
		"ze show bgp rib status")
}

func parseOwnerExcludedResend(args []string) (*ownerExcludedResend, error) {
	refuse := func() (*ownerExcludedResend, error) {
		return nil, opererr.New("The owner-excluded route replay has an invalid scope",
			"Supply one nonzero original owner and an optional registered address family",
			"ze clear bgp rib out-excluding-owner owner <nonzero-owner> [family <afi/safi>]")
	}
	if len(args) != 2 {
		if len(args) != 4 {
			return refuse()
		}
	}
	if args[0] != "owner" {
		return refuse()
	}
	owner, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return refuse()
	}
	if owner == 0 {
		return refuse()
	}
	request := &ownerExcludedResend{owner: owner}
	if len(args) == 4 {
		if args[2] != "family" {
			return refuse()
		}
		fam, known := family.LookupFamily(args[3])
		if !known {
			return refuse()
		}
		request.family = fam
	}
	return request, nil
}

func (r *RIBManager) ownerExcludedResendCommand(_ string, args []string) (string, any, error) {
	request, err := parseOwnerExcludedResend(args)
	if err != nil {
		return statusError, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout)
	defer cancel()
	result, err := request.resend(ctx, r)
	if err != nil {
		return statusError, result, ownerExcludedResendRefusal(err)
	}
	return statusDone, result, nil
}

// excludedResendDestination is an immutable current control observation.
// It narrows output only. The final engine writer still grants or refuses each
// captured owner/session pair. Snapshot collection MUST hold peerMu and MUST
// finish before any RPC, selected lock or worker join begins.
type excludedResendDestination struct {
	peer           netip.Addr
	owner, session uint64
}
type excludedSentReplay struct {
	destination excludedResendDestination
	groups      []replayGroup
}
type excludedSelectedReplay struct {
	destination *selectedDestination
	families    []family.Family
}

func (request *ownerExcludedResend) capture(r *RIBManager) ([]excludedResendDestination, []excludedSentReplay, error) {
	r.peerMu.RLock()
	defer r.peerMu.RUnlock()
	var selected []excludedResendDestination
	var sent []excludedSentReplay
	// This operation-local cut is bounded by the existing live peer population.
	for owner, up := range r.peerUp {
		if !up {
			continue
		}
		if owner == 0 {
			return nil, nil, errors.New("a live destination has no original owner")
		}
		if owner == request.owner {
			continue
		}
		meta := r.peerMeta[owner]
		if meta == nil {
			return nil, nil, errors.New("a live destination has no captured control metadata")
		}
		var groups []replayGroup
		if !meta.refreshOrdinary {
			if request.family == (family.Family{}) {
				groups = r.collectGroupedRibOutRoutes(owner)
			} else {
				groups = r.collectGroupedRibOutRoutesForFamily(owner, request.family)
			}
			// A live receive-only peer without any retained SENT state is not an
			// output recipient. It cannot create a control obligation for this replay.
			if len(groups) == 0 {
				continue
			}
		}
		if !meta.Address.IsValid() {
			return nil, nil, errors.New("a live destination has no observed address")
		}
		if meta.refreshOwner != owner {
			return nil, nil, errors.New("a live destination has no matching original owner")
		}
		if meta.refreshSession == 0 {
			return nil, nil, errors.New("a live destination has no captured UP receipt")
		}
		destination := excludedResendDestination{peer: meta.Address, owner: owner, session: meta.refreshSession}
		if meta.refreshOrdinary {
			selected = append(selected, destination)
			continue
		}
		for i := range groups {
			if groups[i].Route.RawAttrs == "" {
				return nil, nil, errors.New("a retained sent route has no immutable attribute snapshot")
			}
		}
		if len(groups) != 0 {
			sent = append(sent, excludedSentReplay{destination: destination, groups: groups})
		}
	}
	slices.SortFunc(selected, func(a, b excludedResendDestination) int {
		if a.owner < b.owner {
			return -1
		}
		if a.owner > b.owner {
			return 1
		}
		return 0
	})
	slices.SortFunc(sent, func(a, b excludedSentReplay) int {
		if a.destination.owner < b.destination.owner {
			return -1
		}
		if a.destination.owner > b.destination.owner {
			return 1
		}
		return 0
	})
	return selected, sent, nil
}

func (request *ownerExcludedResend) selected(r *RIBManager, target excludedResendDestination) (excludedSelectedReplay, error) {
	if r.selectedDispatch == nil {
		return excludedSelectedReplay{}, errors.New("an ordinary destination has no deciding refresh worker")
	}
	families := make(map[family.Family]struct{})
	if request.family == (family.Family{}) {
		// The configured writer also serves families with no selected route.
		// Finish reading SENT state before taking the dispatcher lock.
		r.peerMu.RLock()
		for fam, routes := range r.ribOut[target.owner] {
			for _, route := range routes {
				if route.configuredForSession(target.session) {
					families[fam] = struct{}{}
					break
				}
			}
		}
		r.peerMu.RUnlock()
	} else {
		families[request.family] = struct{}{}
	}
	d := r.selectedDispatch
	d.mu.Lock()
	defer d.mu.Unlock()
	destination := d.destinations[selectedDestinationID{peer: target.peer, session: target.session}]
	if destination == nil {
		return excludedSelectedReplay{}, errors.New("the captured ordinary destination has ended")
	}
	if destination.owner != target.owner {
		return excludedSelectedReplay{}, errors.New("the captured ordinary destination has another owner")
	}
	if request.family == (family.Family{}) {
		for key := range d.current {
			families[key.family] = struct{}{}
		}
		for key := range d.scoped[originScope{peer: target.peer, owner: target.owner}] {
			families[key.family] = struct{}{}
		}
		for key := range destination.acknowledged {
			families[key.family] = struct{}{}
		}
	}
	result := excludedSelectedReplay{destination: destination}
	for fam := range families {
		result.families = append(result.families, fam)
	}
	slices.SortFunc(result.families, resendFamilyOrder)
	return result, nil
}

func resendFamilyOrder(a, b family.Family) int {
	if a.AFI != b.AFI {
		return cmp.Compare(a.AFI, b.AFI)
	}
	return cmp.Compare(a.SAFI, b.SAFI)
}

func (request *ownerExcludedResend) resend(ctx context.Context, r *RIBManager) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	targets, sent, err := request.capture(r)
	if err != nil {
		return nil, err
	}
	selected := make([]excludedSelectedReplay, 0, len(targets))
	for _, target := range targets {
		snapshot, err := request.selected(r, target)
		if err != nil {
			return nil, err
		}
		selected = append(selected, snapshot)
	}
	refreshed, refused := 0, 0
	var failures []error
	// Admit all ordinary workers before joining one, so a held recipient does not
	// prevent unrelated destination workers from making useful progress.
	queueFailures := make([]error, len(selected))
	for i, snapshot := range selected {
		for _, fam := range snapshot.families {
			if err := ctx.Err(); err != nil {
				queueFailures[i] = errors.Join(queueFailures[i], err)
				break
			}
			destination := snapshot.destination
			if err := r.selectedDispatch.requestRefresh(destination.peer, destination.owner, destination.session, fam); err != nil {
				queueFailures[i] = errors.Join(queueFailures[i], err)
			}
		}
	}
	for _, snapshot := range sent {
		if err := r.resendExcludedSent(ctx, snapshot); err != nil {
			refused++
			failures = append(failures, err)
		} else {
			refreshed++
		}
	}
	for i, snapshot := range selected {
		err := errors.Join(queueFailures[i], r.selectedDispatch.barrierDestinations(ctx, []*selectedDestination{snapshot.destination}))
		if err != nil {
			refused++
			failures = append(failures, err)
		} else {
			refreshed++
		}
	}
	return map[string]any{"refreshed-destinations": refreshed, "refused-destinations": refused}, errors.Join(failures...)
}

// resendExcludedSent MUST use only the captured destination. Every route and
// marker MUST preserve that same tuple through the bounded SDK request. No
// readiness report belongs to a same-session operational resend.
func (r *RIBManager) resendExcludedSent(ctx context.Context, snapshot excludedSentReplay) error {
	byFamily := make(map[family.Family][]replayGroup)
	var families []family.Family
	for _, group := range snapshot.groups {
		if _, seen := byFamily[group.Family]; !seen {
			families = append(families, group.Family)
		}
		byFamily[group.Family] = append(byFamily[group.Family], group)
	}
	slices.SortFunc(families, resendFamilyOrder)
	send := func(input rpc.UpdateRouteInput) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := snapshot.destination
		if !r.currentRefreshSession(target.peer, target.owner, target.session, false) {
			return errors.New("the captured auxiliary destination has ended")
		}
		work, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		deadline, _ := work.Deadline()
		input.PeerSelector = target.peer.String()
		input.DestinationOwner = target.owner
		input.DestinationSession = target.session
		input.DeadlineMS = deadline.UnixMilli()
		return r.updateRouteRequest(work, input)
	}
	for _, fam := range families {
		if err := send(rpc.UpdateRouteInput{PeerMarker: "borr", MarkerFamily: fam.String()}); err != nil {
			return err
		}
		for i := range byFamily[fam] {
			group := &byFamily[fam][i]
			meta := map[string]any{metaKeyReplay: true, bgptypes.SentOwnerMessageMeta: strconv.FormatUint(group.MinMsgID, 10)}
			if group.StaleLevel > 0 {
				meta["stale"] = group.StaleLevel
			}
			// Immutable SENT attributes use self-contained hex groups. They never
			// inherit the process/address delta cursor of another alias or operation.
			for _, command := range formatCursorCommands(group, nil) {
				if err := send(rpc.UpdateRouteInput{Command: command, Meta: meta, SourcePolicyEpoch: group.SourcePolicyEpoch}); err != nil {
					return err
				}
			}
		}
		if err := send(rpc.UpdateRouteInput{PeerMarker: "eorr", MarkerFamily: fam.String()}); err != nil {
			return err
		}
	}
	return nil
}

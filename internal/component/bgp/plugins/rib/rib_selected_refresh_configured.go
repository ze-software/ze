// Design: docs/architecture/plugin/rib-storage-design.md -- selected refresh settlement.
// Related: ribout_entry.go -- original configured SENT routes.
package rib

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type selectedConfiguredReplay func(context.Context, rpc.SelectRefreshInput, *selectedConfiguredRefresh) error

// selectedConfiguredRefresh owns only one captured set of configured keys.
// Success consumes each key once. A retry reads the current SENT entry again.
// Only the destination worker reads or changes this state.
type selectedConfiguredRefresh struct {
	pending   []ribOutKey
	unsettled error
}

// The configured marker can be set across several writes on the same peer.
// The final writer supplies LocalOrigin from the actual Adj-RIB-Out owner.
func (entry ribOutEntry) configuredForSession(session uint64) bool {
	return entry.ConfigStatic && entry.LocalOrigin && entry.DestinationSession == session
}

func (r *RIBManager) replayConfiguredRefresh(ctx context.Context, input rpc.SelectRefreshInput, state *selectedConfiguredRefresh) error {
	return r.replayConfiguredSent(ctx, input, state, r.plugin.UpdateRouteRequestResult)
}

// replayConfiguredSent runs after selected route receipts and before EoRR.
// Configured routes are initially sent by the session itself, so the selected
// table cannot replay them. Their original SENT receipts authorize only the
// same recipient, session and family. RFC 2918 Section 4.
func (r *RIBManager) replayConfiguredSent(ctx context.Context, input rpc.SelectRefreshInput, state *selectedConfiguredRefresh,
	send func(context.Context, rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error),
) error {
	if state.unsettled != nil {
		return state.unsettled
	}
	peer, err := parsePeerAddr(input.Destination)
	if err != nil {
		return err
	}
	fam, known := family.LookupFamily(input.Family)
	if !known {
		return errors.New("configured refresh has an unknown address family")
	}
	if state.pending == nil {
		r.peerMu.RLock()
		rows, live := r.configuredRefreshFamilyLocked(peer, input, fam)
		if live {
			state.pending = make([]ribOutKey, 0)
			for key, entry := range rows {
				if entry.configuredForSession(input.DestinationSession) {
					state.pending = append(state.pending, key)
				}
			}
		}
		r.peerMu.RUnlock()
		if !live {
			return errors.New("configured refresh's captured destination has ended")
		}
	}
	for len(state.pending) != 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := state.pending[0]
		r.peerMu.RLock()
		rows, live := r.configuredRefreshFamilyLocked(peer, input, fam)
		entry := rows[key]
		var route *Route
		if entry.configuredForSession(input.DestinationSession) {
			route = reconstructRoute(entry, fam, key)
		}
		r.peerMu.RUnlock()
		if !live {
			return errors.New("configured refresh's captured destination has ended")
		}
		if route != nil {
			if err := sendConfiguredRefreshRoute(ctx, input, route, send); err != nil {
				if _, unsettled := errors.AsType[*rpc.CancellationUnsettledError](err); unsettled {
					// The original writer may still complete. Keep this cut
					// unresolved until the original destination ends.
					state.unsettled = err
				}
				return err
			}
		}
		state.pending = state.pending[1:]
	}
	return ctx.Err()
}

// configuredRefreshFamilyLocked borrows the exact active destination's SENT
// family while peerMu is held. The worker releases peerMu before the SDK call.
func (r *RIBManager) configuredRefreshFamilyLocked(peer netip.Addr, input rpc.SelectRefreshInput, fam family.Family) (map[ribOutKey]ribOutEntry, bool) {
	meta := r.peerMeta[input.PeerOwner]
	if input.PeerOwner == 0 || input.DestinationSession == 0 || !r.peerUp[input.PeerOwner] || meta == nil {
		return nil, false
	}
	if meta.Address != peer || !meta.refreshOrdinary || meta.refreshOwner != input.PeerOwner || meta.refreshSession != input.DestinationSession {
		return nil, false
	}
	return r.ribOut[input.PeerOwner][fam], true
}

func sendConfiguredRefreshRoute(ctx context.Context, input rpc.SelectRefreshInput, route *Route,
	send func(context.Context, rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error),
) error {
	if route.MsgID == 0 || route.RawAttrs == "" {
		return fmt.Errorf("configured refresh route %s has no complete SENT receipt", route.Prefix)
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	deadline, _ := work.Deadline()
	// A configured route has no received-policy epoch. Leaving it zero also
	// makes the final SENT owner check reject any matching learned route,
	// regardless of the observations the plugin received.
	output, err := send(work, rpc.UpdateRouteInput{
		PeerSelector: input.Destination, DestinationOwner: input.PeerOwner, DestinationSession: input.DestinationSession,
		DeadlineMS: deadline.UnixMilli(), Command: formatRouteCommand(route),
		Meta: map[string]any{metaKeyReplay: true, bgptypes.SentOwnerMessageMeta: strconv.FormatUint(route.MsgID, 10)},
	})
	if err != nil {
		return fmt.Errorf("replaying configured refresh route %s: %w", route.Prefix, err)
	}
	if err := work.Err(); err != nil {
		return err
	}
	// Bound maintenance answers only after the final writer. An accepted
	// stale receipt or policy exclusion may correctly leave the wire alone.
	// Missing or refused completion must keep the refresh open.
	if output == nil || output.Announced != 1 || output.Withdrawn != 0 || output.Failure != "" || len(output.Warnings) != 0 {
		return fmt.Errorf("configured refresh route %s did not receive a clean bound replay result", route.Prefix)
	}
	return nil
}

// Design: docs/architecture/api/architecture.md -- the send half of an attach block, owner-relayed peer-up replay
// Related: ../../../component/bgp/plugins/adj_rib_in/rib_commands.go -- replayRoutesCommand, the store that answers pages
// Related: ../../../component/bgp/plugins/rr/rr.go -- replayForPeer, an owner that relays them
// Related: ../../../component/bgp/plugins/rs/server_handlers.go -- replayForPeer, the other owner
// Related: ../../../component/bgp/plugins/adj_rib_in/rib_replay_path.go -- replayPathCommand, the store that answers one path

package ribevents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ReplayRoutesCommand asks the receive store (bgp-adj-rib-in) for the stored
// routes a peer-up replay owes one destination, as one bounded page per call.
//
// The store ANSWERS the routes and relays nothing. The replay owner (bgp-rr,
// bgp-rs) relays each page itself, under its own name, because the
// destination's `send [ update ]` grant is the owner's: the engine keeps an
// owner's `bgp-peer-up-replay` claim for a peer only while the owner holds that
// grant, and the operator gives the store none. A relay sent by the store is
// judged on the store's own grant and refused.
//
// Arguments: "<destination> <from-index> [<max-msg-id>]", the same cursor and
// cut as `request bgp adj-rib-in replay`.
const ReplayRoutesCommand = "request bgp adj-rib-in replay-routes"

// ReplayPathCommand asks the receive store for the current state of one source
// path after its validation changed, for one destination: the stored path
// (ineligible ones included, which the engine's gate turns into a withdrawal),
// or an explicit withdrawal when the store holds none.
//
// Like ReplayRoutesCommand, the store ANSWERS a ReplayPage, here with at most
// one route and More false, and relays nothing: the caller (bgp-rr, bgp-rs)
// relays it under its own send grant, for the reason ReplayRoutesCommand gives.
// The page is empty when the destination is the source itself.
//
// Arguments: "<destination> <source> <family> <prefix> <path-id>".
const ReplayPathCommand = "request bgp adj-rib-in replay-path"

// ReplayPage is one answer to ReplayRoutesCommand or ReplayPathCommand.
//
// Routes are in ascending store sequence, and LastIndex is the sequence of the
// last route in this page, or 0 when the page is empty. More is true when the
// store held routes past LastIndex that this page left out to stay inside
// ReplayPageBudget: the caller asks again from LastIndex. More can be true with
// nothing left to send, when the routes the page left out are deleted before
// the next read; the caller then reads one empty page with More false, which
// costs one round trip and loses nothing.
//
// IngestedMsgID is the store's ingest position, a pointer so "untracked"
// (omitted) stays distinct from the real position 0. bgp-rs waits on it before
// it trusts an empty answer (rs/server_handlers.go, replayProgress.coversCut).
type ReplayPage struct {
	Routes        []rpc.StoredRoute `json:"routes"`
	LastIndex     uint64            `json:"last-index"`
	More          bool              `json:"more"`
	IngestedMsgID *uint64           `json:"ingested-msg-id,omitempty"`
}

// ReplayPageBudget bounds the serialized size of the routes one ReplayPage
// carries. Half the frame ceiling, for two reasons. The page crosses the
// transport twice (store to engine as an answer, engine to owner inside a
// dispatch result), so its envelope is paid twice. And the owner relays the
// page in ONE RelayStoredRoute call, whose frame must also stay under
// rpc.MaxMessageSize, so the page fits that call with room to spare.
const ReplayPageBudget = rpc.MaxMessageSize / 2

// ReplayDispatch sends one command to the engine and returns its status and
// data: the shape of sdk.Plugin.DispatchCommandArgs without the peer argument.
type ReplayDispatch func(ctx context.Context, command string, args ...string) (string, json.RawMessage, error)

// ReplayRelay hands routes to the engine for relay to destination: the shape of
// sdk.Plugin.RelayStoredRoute. The engine judges the CALLER's send grant.
type ReplayRelay func(ctx context.Context, destination string, routes []rpc.StoredRoute) error

// ReplayProgress is what one RelayReplay call delivered.
type ReplayProgress struct {
	// LastIndex is the highest store sequence relayed, or 0 when nothing was.
	LastIndex uint64
	// Replayed counts the routes relayed, across every page.
	Replayed int
	// IngestedMsgID is the store's ingest position from the last page, or nil
	// when the store tracks none.
	IngestedMsgID *uint64
}

// errReplayPageStalled reports a page that claims more routes but does not move
// the cursor, which would ask for the same page forever.
var errReplayPageStalled = errors.New("replay-routes: the store answered more routes without advancing " +
	"last-index, so the replay stopped rather than ask for the same page again")

// RelayReplay walks the store's replay for destination from fromIndex, page by
// page, and relays every page through relay before it asks for the next one.
// cut is the max-msg-id argument, or "" for an unbounded replay.
//
// The loop is bounded: each page past the first starts strictly after the last
// one ended (errReplayPageStalled refuses a page that does not), and the store's
// sequence counter is finite, so the walk ends when the store says More is
// false. An error from the store, from decoding, or from a relay ends the walk;
// the progress returned covers the pages relayed before it.
//
// Safe for concurrent use when dispatch and relay are.
func RelayReplay(ctx context.Context, dispatch ReplayDispatch, relay ReplayRelay,
	destination string, fromIndex uint64, cut string) (ReplayProgress, error) {
	var progress ReplayProgress
	cursor := fromIndex
	for {
		args := []string{destination, textbuf.StringUint(cursor)}
		if cut != "" {
			args = append(args, cut)
		}
		page, err := readReplayPage(ctx, dispatch, ReplayRoutesCommand, args...)
		if err != nil {
			return progress, err
		}
		progress.IngestedMsgID = page.IngestedMsgID
		if len(page.Routes) > 0 {
			if err := relay(ctx, destination, page.Routes); err != nil {
				return progress, fmt.Errorf("relay of %d replayed routes to %s: %w", len(page.Routes), destination, err)
			}
			progress.Replayed += len(page.Routes)
			progress.LastIndex = page.LastIndex
		}
		if !page.More {
			return progress, nil
		}
		if page.LastIndex <= cursor {
			return progress, errReplayPageStalled
		}
		cursor = page.LastIndex
	}
}

// RelayReplayPath reads one source path's current state from the store
// (ReplayPathCommand) and relays it to destination through relay, under the
// caller's own name. It reports how many routes it relayed: 0 when the
// destination is the source, otherwise 1.
//
// The relay leaves StoredRoute.InitialUpdate false: a validation change is a
// live change, so the engine holds it behind a peer-up replay still running
// toward destination, like a forwarded UPDATE.
//
// Safe for concurrent use when dispatch and relay are.
func RelayReplayPath(ctx context.Context, dispatch ReplayDispatch, relay ReplayRelay,
	destination, source, family, prefix, pathID string) (int, error) {
	page, err := readReplayPage(ctx, dispatch, ReplayPathCommand, destination, source, family, prefix, pathID)
	if err != nil {
		return 0, err
	}
	if len(page.Routes) == 0 {
		return 0, nil
	}
	if err := relay(ctx, destination, page.Routes); err != nil {
		return 0, fmt.Errorf("relay of revalidated path %s from %s to %s: %w", prefix, source, destination, err)
	}
	return len(page.Routes), nil
}

// readReplayPage sends command to the store and decodes its ReplayPage. args[0]
// MUST be the destination, which the errors name.
func readReplayPage(ctx context.Context, dispatch ReplayDispatch, command string, args ...string) (ReplayPage, error) {
	var page ReplayPage
	status, data, err := dispatch(ctx, command, args...)
	if err != nil {
		return page, err
	}
	if status != "done" {
		return page, fmt.Errorf("%s for %s answered status %q, want done", command, args[0], status)
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return page, fmt.Errorf("%s for %s: undecodable answer: %w", command, args[0], err)
	}
	return page, nil
}

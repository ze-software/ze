// Design: docs/guide/redistribution.md -- the redistribute root, end to end
// Related: plugin_fixture_12.go -- the init that registers these three drivers
// Related: internal/test/plugins/fakeredist/sink.go -- the synthetic consumer they drive

// The three observers behind spec-fixit-redistribution-chain-drops-silently.
//
// The first two register a synthetic consumer through fakeredist and read back
// what the orchestrator handed it. That is the one thing a .ci scenario cannot
// see from the wire, because a destination that is not BGP puts nothing on a
// session.
//
// The third reads the Adj-RIB-In instead, and its subject is a route
// redistribution has nothing to do with: what a `redistribute` block must NOT
// do to the routes a peer announces.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The two destination names these scenarios register consumers under. Neither
// is `bgp`, so the orchestrator's loop guard leaves them alone, and neither is
// `fakeredist`, which the plugin refuses for the same reason.
//
// redistChainProbe is registered FIRST and exists to prove the dispatcher is
// live. redistChainSink is the LATE consumer the replay has to fill.
const (
	redistChainSink  = "fakedest"
	redistChainProbe = "fakeprobe"
)

// The settle window a route is waited for in. A correct daemon delivers on the
// incremental dispatch or on the replay, and both are synchronous on the
// in-process bus. The window is therefore a margin against a slow BGP session,
// never a place a race can hide.
const (
	redistChainAttempts = 40
	redistChainDelay    = 250 * time.Millisecond
)

// redistChainConsume registers a synthetic consumer and returns once the
// registry holds it.
func redistChainConsume(ctx context.Context, plugin *sdk.Plugin, name string) error {
	var tb textbuf.Buffer
	_, err := plugin01RequireDone(ctx, plugin, tb.Str("request fakeredist consume ").Str(name).String())
	return err
}

// redistChainHeld answers the prefixes one synthetic consumer currently holds.
// A dispatch error is returned rather than read as "nothing yet", so a broken
// command surface fails with its own message instead of timing out.
func redistChainHeld(ctx context.Context, plugin *sdk.Plugin, name string) ([]string, error) {
	var tb textbuf.Buffer
	data, err := plugin01RequireDone(ctx, plugin, tb.Str("show fakeredist consumed ").Str(name).String())
	if err != nil {
		return nil, err
	}
	raw, ok := data["prefixes"].([]any)
	if !ok {
		var tbErr textbuf.Buffer
		return nil, errors.New(tbErr.Str("consumed answered no prefix list for ").Str(name).String())
	}
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		if text, ok := entry.(string); ok {
			out = append(out, text)
		}
	}
	return out, nil
}

// redistChainHolds reports whether one consumer holds prefix.
func redistChainHolds(ctx context.Context, plugin *sdk.Plugin, name, prefix string) (bool, error) {
	held, err := redistChainHeld(ctx, plugin, name)
	if err != nil {
		return false, err
	}
	for _, entry := range held {
		if entry == prefix {
			return true, nil
		}
	}
	return false, nil
}

// redistChainWait polls until one consumer holds prefix, and names what it held
// instead when it never does.
func redistChainWait(ctx context.Context, plugin *sdk.Plugin, name, prefix string) error {
	var pollErr error
	reached := Poll(ctx, redistChainAttempts, redistChainDelay, func() bool {
		holds, err := redistChainHolds(ctx, plugin, name, prefix)
		if err != nil {
			pollErr = err
			return true
		}
		return holds
	})
	if pollErr != nil {
		return pollErr
	}
	if reached {
		return nil
	}
	held, err := redistChainHeld(ctx, plugin, name)
	if err != nil {
		return err
	}
	return redistChainMissing(name, prefix, held)
}

// redistChainEmit emits one route from fakeredist. The emit's own answer says
// nothing about whether a consumer received it: the plugin server counts PLUGIN
// PROCESS subscribers, and the orchestrator subscribes engine-side
// (Server.deliverEvent, internal/component/plugin/server/dispatch.go).
func redistChainEmit(ctx context.Context, plugin *sdk.Plugin, prefix string) error {
	var tb textbuf.Buffer
	_, err := plugin01RequireDone(ctx, plugin,
		tb.Str("request fakeredist emit add ipv4/unicast ").Str(prefix).String())
	return err
}

// redistChainMissing is the failure message both scenarios end on, naming what
// the consumer held instead of what was expected.
func redistChainMissing(name, prefix string, held []string) error {
	var tb textbuf.Buffer
	tb.Str(prefix).Str(" never reached the ").Str(name).Str(" consumer; it holds [")
	tb.Join(held, " ").Byte(']')
	return errors.New(tb.String())
}

// redistChainBGPSourceReachesConsumer drives the whole chain, from a BGP peer's
// UPDATE to a redistribution consumer, over a config that writes no Loc-RIB
// plumbing at all.
//
// The consumer is registered FIRST, so the incremental dispatch carries the
// route on a normal run. The replay is the fallback for an UPDATE that arrived
// first.
//
// Either way the test proves the stage this spec repaired. The Loc-RIB sees the
// peer's UPDATE only because the `import bgp` rule derived that peer's process
// binding.
func redistChainBGPSourceReachesConsumer(ctx context.Context, plugin *sdk.Plugin) error {
	if err := redistChainConsume(ctx, plugin, redistChainSink); err != nil {
		return err
	}
	err := redistChainWait(ctx, plugin, redistChainSink, "10.0.0.0/24")
	if err == nil {
		return nil
	}
	// Name the stage. The chain has two halves either side of the Loc-RIB, and
	// the failure reads the same from the consumer whichever half broke.
	status, statusErr := plugin01DispatchMapOrEmpty(ctx, plugin, "show bgp rib status")
	var tb textbuf.Buffer
	tb.Str(err.Error()).Str("; `show bgp rib status` answered ")
	if statusErr != nil {
		tb.Str("an error: ").Str(statusErr.Error())
	} else {
		tb.Str(fmt.Sprint(status))
	}
	return errors.New(tb.String())
}

// plugin01DispatchMapOrEmpty answers a command's data, or an empty map when the
// command itself failed, so a failure message can still carry what it found.
func plugin01DispatchMapOrEmpty(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	data, _, err := plugin01DispatchMap(ctx, plugin, command)
	if err != nil {
		return map[string]any{}, err
	}
	if data == nil {
		return map[string]any{}, nil
	}
	return data, nil
}

// redistChainLateConsumer registers a synthetic consumer AFTER a producer
// emitted. Nothing in the daemon guarantees against that order, because the
// static plugin and the IS-IS plugin start in the same tier.
//
// The scenario runs in three steps, and the first exists to make the third mean
// something:
//
//  1. register `fakeprobe`, emit 10.98.0.0/24, and WAIT for the probe to hold it
//  2. emit 10.99.0.0/24, which no `fakedest` consumer yet exists to receive
//  3. register `fakedest`, whose replay is the only thing that can fill it
//
// Step 1 is the control. The orchestrator subscribes to its producers on a
// goroutine, so the wait is what rules one failure out. Without it, an emit
// that reached nobody looks exactly like a consumer nothing replayed to.
func redistChainLateConsumer(ctx context.Context, plugin *sdk.Plugin) error {
	if err := redistChainConsume(ctx, plugin, redistChainProbe); err != nil {
		return err
	}
	if err := redistChainEmit(ctx, plugin, "10.98.0.0/24"); err != nil {
		return err
	}
	if err := redistChainWait(ctx, plugin, redistChainProbe, "10.98.0.0/24"); err != nil {
		return err
	}

	if err := redistChainEmit(ctx, plugin, "10.99.0.0/24"); err != nil {
		return err
	}
	if err := redistChainConsume(ctx, plugin, redistChainSink); err != nil {
		return err
	}

	// Read ONCE, with no settle window. The replay is synchronous on the
	// in-process bus, so a daemon that has to be waited for here is delivering
	// by some other path.
	held, err := redistChainHeld(ctx, plugin, redistChainSink)
	if err != nil {
		return err
	}
	for _, want := range []string{"10.98.0.0/24", "10.99.0.0/24"} {
		found := false
		for _, entry := range held {
			if entry == want {
				found = true
				break
			}
		}
		if !found {
			return redistChainMissing(redistChainSink, want, held)
		}
	}
	return nil
}

// redistChainReceivedPrefixes answers the prefixes the Adj-RIB-In holds.
//
// A dispatch error is returned rather than read as an empty table, so a broken
// command surface fails with its own message instead of timing out. An empty
// walk is a different thing and is NOT an error: it marshals as
// `{"routes":null}` (`serializeRouteRows`, internal/component/bgp/plugins/rib/
// rib_pipeline.go), which is the answer during the window before the peer's
// UPDATE arrives.
func redistChainReceivedPrefixes(ctx context.Context, plugin *sdk.Plugin) ([]string, error) {
	data, err := plugin01RequireDone(ctx, plugin, "show bgp rib received")
	if err != nil {
		return nil, err
	}
	raw, present := data["routes"]
	if !present || raw == nil {
		return nil, nil
	}
	rows, ok := raw.([]any)
	if !ok {
		return nil, errors.New("`show bgp rib received` answered a `routes` field that is not a list of rows")
	}
	out := make([]string, 0, len(rows))
	for _, entry := range rows {
		row, isRow := entry.(map[string]any)
		if !isRow {
			continue
		}
		prefix, isText := row["prefix"].(string)
		if !isText {
			continue
		}
		out = append(out, prefix)
	}
	return out, nil
}

// redistChainPeerRouteReachesAdjRIBIn requires the prefix a peer announced to
// be in the Adj-RIB-In, BY NAME.
//
// This is the blast-radius assertion, and its subject is a route that
// redistribution has nothing to do with. The retired `bgp-redistribute` ingress
// filter dropped every received route whenever a `redistribute` block existed,
// whatever that block said, because it asked the redistribution evaluator to
// judge the route and passed an empty importing protocol. So the property under
// test is not "redistribution works": it is that configuring redistribution
// leaves BGP's own route store alone.
//
// One driver serves two scenarios whose configs differ ONLY by the presence of
// a `redistribute` block. Neither config leaves anything to a derived binding:
// both attach bgp-rib to the peer by hand, so a red here is about the ingress
// path and about nothing else.
func redistChainPeerRouteReachesAdjRIBIn(ctx context.Context, plugin *sdk.Plugin) error {
	const prefix = "10.0.0.0/24"
	var held []string
	var pollErr error
	reached := Poll(ctx, redistChainAttempts, redistChainDelay, func() bool {
		found, err := redistChainReceivedPrefixes(ctx, plugin)
		if err != nil {
			pollErr = err
			return true
		}
		held = found
		return slices.Contains(held, prefix)
	})
	if pollErr != nil {
		return pollErr
	}
	if reached {
		fmt.Fprintln(os.Stderr, "OK: the peer's "+prefix+" reached the Adj-RIB-In")
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("the peer announced ").Str(prefix).Str(" and the Adj-RIB-In never held it; `show bgp rib received` holds [")
	tb.Join(held, " ").Byte(']')
	return errors.New(tb.String())
}

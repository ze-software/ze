// Design: docs/architecture/api/commands.md -- a data caller's pipe chain, applied by the engine
// Related: plugin_fixture_16.go -- plugin16Dispatch, plugin16DoneMap, plugin16Number, plugin16WaitEOR
// Related: register_dispatch_pipe_chain_applied.go -- the registration of this observer

package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// plugin16RIBRoutesSettled is the number of routes the .ci's peer sends.
const plugin16RIBRoutesSettled = 5

// plugin16DispatchPipeChainApplied proves a forked plugin's dispatch-command
// that carries a pipe chain is answered with the chain applied: the RIB's
// declared operations reach its handler, and the engine applies the residual
// generic chain and a format (spec-pipe-chain-honored-on-every-command-surface
// AC-2, AC-4, AC-5).
//
// Goal: every assertion the .ci relies on returns an error when it fails, so
// the fixture exits non-zero. Method: wait on the RIB's state through the bare
// `show bgp rib` (no chain, whose answer the change leaves alone), then send
// each piped line once and judge its answer.
func plugin16DispatchPipeChainApplied(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin16WaitRIBRoutes(ctx, p); err != nil {
		return err
	}

	// AC-2: the declared chain, applied by the RIB.
	const declared = "show bgp rib | received | count"
	if err := plugin16ExpectCount(ctx, p, declared, plugin16RIBRoutesSettled); err != nil {
		return err
	}

	// AC-4: an alias and the generic count on a command that declares none;
	// the .ci configures one peer.
	const generic = "show bgp | peers | count"
	if err := plugin16ExpectCount(ctx, p, generic, 1); err != nil {
		return err
	}

	// AC-5: a format on a data caller answers done, its data the rendered text.
	// The data is decoded straight into a string, never through
	// plugin16Dispatch: that helper decodes a string answer again as JSON, so
	// the rendered text "5" would come back as a number. Data that is not one
	// JSON string fails the decode here.
	const formatted = "show bgp rib | received | count | text"
	var text string
	status, err := Dispatch(ctx, p, formatted, &text)
	if err != nil {
		return fmt.Errorf("%q, expected the rendered text as a string: %w", formatted, err)
	}
	if status != statusDone {
		return fmt.Errorf("%q: status=%s data=%q", formatted, status, text)
	}
	if !strings.Contains(text, "5") {
		return fmt.Errorf("%q rendered %q, which holds no count of 5", formatted, text)
	}
	var tb textbuf.Buffer
	tb.Str("OK: ").Quoted(formatted).Str(" answers its rendered text\n").
		StdErr() //nolint:errcheck // fixture output; the .ci reads it

	if !plugin16WaitEOR(ctx, p, "show bgp peer peer1 detail", 40) {
		return fmt.Errorf("ze never sent EOR to the peer")
	}
	return nil
}

// plugin16ExpectCount dispatches one piped line and requires a done answer
// whose data is `{"count": want}`.
func plugin16ExpectCount(ctx context.Context, p *sdk.Plugin, line string, want int64) error {
	data, err := plugin16DoneMap(ctx, p, line, line)
	if err != nil {
		return err
	}
	count, ok := plugin16Number(data["count"])
	if !ok {
		return fmt.Errorf("%q answered %v, which holds no integer count", line, data)
	}
	if count != want {
		return fmt.Errorf("%q answered %v, expected count %d", line, data, want)
	}
	var tb textbuf.Buffer
	tb.Str("OK: ").Quoted(line).Str(" counts ").Int(count).Byte('\n').
		StdErr() //nolint:errcheck // fixture output; the .ci reads it
	return nil
}

// plugin16WaitRIBRoutes polls the bare `show bgp rib` until it lists every
// route the peer sent. The bare command carries no chain, so its rows under
// `routes` (showRows) are the barrier on the RIB's state, never on time.
func plugin16WaitRIBRoutes(ctx context.Context, p *sdk.Plugin) error {
	var last any
	settled := Poll(ctx, 40, 100*time.Millisecond, func() bool {
		r := plugin16Dispatch(ctx, p, "show bgp rib")
		last = r.data
		if r.err != nil {
			return false
		}
		if r.status != statusDone {
			return false
		}
		data, isMap := r.data.(map[string]any)
		if !isMap {
			return false
		}
		routes, isList := data["routes"].([]any)
		return isList && len(routes) == plugin16RIBRoutesSettled
	})
	if !settled {
		return fmt.Errorf("the RIB never listed %d routes, last=%v", plugin16RIBRoutesSettled, last)
	}
	var tb textbuf.Buffer
	tb.Str("OK: the RIB lists ").Int(plugin16RIBRoutesSettled).Str(" routes\n").
		StdErr() //nolint:errcheck // fixture output; the .ci reads it
	return nil
}

package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

type p12FilterState struct {
	calls atomic.Int32
	mu    sync.Mutex
	order []string

	exportModified atomic.Int32
	exportRejected atomic.Int32
	exportPassed   atomic.Int32
}

func p12RegisterFilterFixtures() {
	Register("plugin/redistribute-chain-order", p12FilterChainDriver())
	Register("plugin/redistribute-declare", p12FilterDeclareDriver())
	Register("plugin/redistribute-export-modify", p12FilterExportModifyDriver())
	Register("plugin/redistribute-export-reject", p12FilterExportRejectDriver())
	Register("plugin/redistribute-import-accept", p12FilterImportDriver(
		"filter-accept-test",
		"accept-all",
		[]string{fieldASPath, pipeOrigin},
		sdk.FilterAccept,
		"filter called %d time(s)",
		false,
	))
	Register("plugin/redistribute-import-modify", p12FilterImportDriver(
		"filter-modify-test",
		"set-localpref",
		[]string{fieldLocalPreference},
		sdk.FilterModify,
		"filter modified %d route(s)",
		true,
	))
	Register("plugin/redistribute-import-reject", p12FilterImportDriver(
		"filter-reject-test",
		"reject-all",
		[]string{fieldASPath, pipeOrigin},
		sdk.FilterReject,
		"filter rejected %d route(s)",
		false,
	))
}

func p12FilterDriver(name string, registration *sdk.Registration, handler sdk.FilterEvaluator, scenario p12Scenario) Driver {
	return func(ctx context.Context, args []string) error {
		var marker string
		if name == "filter-export-modify-test" {
			if len(args) != 1 {
				return fmt.Errorf("%s requires a readiness marker path", name)
			}
			marker = args[0]
			_ = os.Remove(marker)
		} else if len(args) != 0 {
			return fmt.Errorf("%s takes no arguments", name)
		}

		plugin, err := newObserver(name)
		if err != nil {
			return fmt.Errorf("connect observer %s: %w", name, err)
		}
		defer plugin.Close() //nolint:errcheck // the run result carries transport failures
		if handler != nil {
			plugin.OnFilter(handler)
		}

		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				var scenarioErr error
				if marker != "" {
					// Quiesce alone cannot prove peer2 has connected: a peer
					// that has not started yet has no pending work to drain.
					// Observe its initial EOR first, then drain before the
					// marker permits the source peer to send its route.
					if !p12WaitPeerCounter(ctx, plugin, "127.0.0.2", "eor-sent", 1) {
						scenarioErr = fmt.Errorf("peer2 never sent its initial end-of-rib before the source peer started")
					} else if status, _, err := plugin.DispatchCommand(ctx, "request quiesce"); err != nil || status != rpc.StatusDone {
						scenarioErr = fmt.Errorf("peer2 did not settle before the source peer started: status=%s: %w", status, err)
					} else if err := os.WriteFile(marker, []byte("ready"), 0o600); err != nil {
						scenarioErr = err
					} else {
						defer os.Remove(marker) //nolint:errcheck // scratch cleanup on exit, so a removal failure changes no assertion
					}
				}
				if scenarioErr == nil {
					scenarioErr = invokeScenario(ctx, plugin, scenario)
				}
				result <- scenarioErr
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
			}()
			return nil
		})
		runErr := plugin.Run(ctx, registration)
		select {
		case scenarioErr := <-result:
			return errorsJoinP12(scenarioErr, runErr)
		default:
			return runErr
		}
	}
}

func p12FilterChainDriver() Driver {
	state := new(p12FilterState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{
		{Name: filterNameFirst, Direction: sdk.FilterImport, Reads: []string{fieldASPath}, OnError: sdk.OnErrorReject},
		{Name: "second", Direction: sdk.FilterImport, Reads: []string{fieldCommunity}, OnError: sdk.OnErrorReject},
	}}
	handler := func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
		state.mu.Lock()
		state.order = append(state.order, route.Filter())
		call := len(state.order)
		state.mu.Unlock()
		fmt.Fprintf(os.Stderr, "filter-update called: filter=%s (call #%d)\n", route.Filter(), call)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool {
			state.mu.Lock()
			defer state.mu.Unlock()
			return len(state.order) >= 2
		})
		if !p12WaitPeerCounter(ctx, plugin, "*", "eor-sent", 1) {
			return fmt.Errorf("ze sent no end-of-rib to peer1 before shutdown")
		}
		if err := p12Quiesce(ctx, plugin); err != nil {
			return err
		}

		state.mu.Lock()
		defer state.mu.Unlock()
		if len(state.order) < 2 {
			return fmt.Errorf("%d filter call(s), expected 2: %v", len(state.order), state.order)
		}
		if state.order[0] != filterNameFirst || state.order[1] != "second" {
			return fmt.Errorf("wrong filter order: %v", state.order)
		}
		fmt.Fprintf(os.Stderr, "OK: filters called in correct order: ['%s']\n", strings.Join(state.order, "', '"))
		return nil
	}
	return p12FilterDriver("filter-chain-test", &registration, handler, scenario)
}

func p12FilterDeclareDriver() Driver {
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:      "accept-all",
		Direction: sdk.FilterImport,
		Reads:     []string{fieldASPath, fieldCommunity},
		OnError:   sdk.OnErrorReject,
	}}}
	return p12FilterDriver("filter-declare-test", &registration, nil, func(ctx context.Context, plugin *sdk.Plugin) error {
		fmt.Fprintln(os.Stderr, "OK: filter declaration accepted, startup complete")
		if !p12WaitPeerCounter(ctx, plugin, "*", "eor-sent", 1) {
			return fmt.Errorf("ze sent no end-of-rib to peer1 before shutdown")
		}
		return nil
	})
}

func p12FilterExportModifyDriver() Driver {
	state := new(p12FilterState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:      "set-export-localpref",
		Direction: sdk.FilterExport,
		Reads:     []string{fieldLocalPreference},
		Sets:      []string{fieldLocalPreference},
		OnError:   sdk.OnErrorReject,
	}}}
	handler := func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
		if route.Direction() == directionExport {
			fields, err := route.Fields()
			if err != nil {
				return nil, fmt.Errorf("set-export-localpref: read the declared fields: %w", err)
			}
			state.exportModified.Add(1)
			p12FilterSaid("filter-update EXPORT MODIFY: peer="+route.Peer()+" fields=", fields)
			return p12SetLocalPreference(), nil
		}
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool { return state.exportModified.Load() > 0 })
		if !p12WaitPeerCounter(ctx, plugin, "127.0.0.2", "updates-sent", 2) {
			return fmt.Errorf("ze never wrote the modified UPDATE to peer2")
		}
		if !p12WaitPeerCounter(ctx, plugin, "127.0.0.1", "eor-sent", 1) {
			return fmt.Errorf("ze sent no end-of-rib to peer1 before shutdown")
		}
		count := state.exportModified.Load()
		if count == 0 {
			return fmt.Errorf("export filter never saw the forward of 10.0.0.0/24 to peer2")
		}
		fmt.Fprintf(os.Stderr, "OK: export filter modified %d forward(s)\n", count)
		return nil
	}
	return p12FilterDriver("filter-export-modify-test", &registration, handler, scenario)
}

func p12FilterExportRejectDriver() Driver {
	state := new(p12FilterState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:      "block-export",
		Direction: sdk.FilterExport,
		Reads:     []string{fieldASPath, p12FieldPrefix},
		OnError:   sdk.OnErrorReject,
	}}}
	handler := func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
		if route.Direction() != directionExport {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
		fields, err := route.Fields()
		if err != nil {
			return nil, fmt.Errorf("block-export: read the declared fields: %w", err)
		}
		announced, err := p12Announces(fields, "10.0.0.0/24")
		if err != nil {
			return nil, err
		}
		if announced {
			state.exportRejected.Add(1)
			p12FilterSaid("filter-update EXPORT REJECT: peer="+route.Peer()+" fields=", fields)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
		}
		state.exportPassed.Add(1)
		p12FilterSaid("filter-update EXPORT PASS (fence): peer="+route.Peer()+" fields=", fields)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool {
			return state.exportRejected.Load() > 0 && state.exportPassed.Load() > 0
		})
		if !p12WaitPeerCounter(ctx, plugin, "127.0.0.2", "updates-sent", 2) {
			return fmt.Errorf("ze never wrote the fence UPDATE to peer2")
		}
		if err := p12WaitPeersEOR(ctx, plugin, peerNameOne); err != nil {
			return err
		}
		rejected, passed := state.exportRejected.Load(), state.exportPassed.Load()
		if rejected == 0 {
			return fmt.Errorf("export filter never saw the forward of 10.0.0.0/24 to peer2")
		}
		if passed == 0 {
			return fmt.Errorf("export filter never saw the fence forward of 10.9.0.0/24 to peer2")
		}
		fmt.Fprintf(os.Stderr, "OK: export filter rejected %d forward(s), passed %d\n", rejected, passed)
		return nil
	}
	return p12FilterDriver("filter-export-reject-test", &registration, handler, scenario)
}

// p12FieldPrefix is the route field naming the announced NLRI
// (docs/plugin-development/protocol.md, "The Route Fields of filter-update").
const p12FieldPrefix = "prefix"

// p12SetLocalPreference is the modify answer of the local-preference filters:
// one Set of local-preference to 200, which the redistribute-*-modify tests
// find on the wire as LOCAL_PREF 0xC8.
func p12SetLocalPreference() *sdk.FilterUpdateOutput {
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{{
		Field:  fieldLocalPreference,
		Action: rpc.FilterSetActionSet,
		Value:  json.RawMessage(`200`),
	}}}
}

// p12Announces answers whether the fields object announces prefix in any
// family. An object that does not decode is an error, so the filter's
// on-error decides the route rather than a verdict the fixture made up.
func p12Announces(fields []byte, prefix string) (bool, error) {
	var route struct {
		Prefix map[string][]string `json:"prefix"`
	}
	if err := json.Unmarshal(fields, &route); err != nil {
		return false, fmt.Errorf("decode the declared fields %s: %w", fields, err)
	}
	for _, announced := range route.Prefix {
		if slices.Contains(announced, prefix) {
			return true, nil
		}
	}
	return false, nil
}

// p12FilterSaid prints one line, a lead then the fields object the filter was
// handed, so a failure carries what the daemon sent.
func p12FilterSaid(lead string, fields []byte) {
	var line textbuf.Buffer
	line.Str(lead).Str(string(fields))
	line.Byte('\n')
	_ = line.StdErr() //nolint:errcheck // a failed write to stderr has nowhere left to report itself
}

// p12FilterImportDriver runs one import filter that declares reads and, for a
// modify filter, the local-preference Set its answer carries.
//
// forwardsToPeerTwo is true when the .ci scripts an internal peer2 at
// 127.0.0.2 that must receive the imported route through the route server.
// The scenario then waits for two UPDATEs sent to peer2 (the route and its
// end-of-rib) before it returns: the quiesce below drains the forward pool
// and peer sync only, so without this wait the shutdown can land between
// the route server receiving the route and forwarding it, and peer2 never
// sees it on a loaded machine.
func p12FilterImportDriver(name, filterName string, reads []string, action sdk.FilterAction, success string, forwardsToPeerTwo bool) Driver {
	state := new(p12FilterState)
	declaration := sdk.FilterDecl{
		Name:      filterName,
		Direction: sdk.FilterImport,
		Reads:     reads,
		OnError:   sdk.OnErrorReject,
	}
	if action == sdk.FilterModify {
		declaration.Sets = []string{fieldLocalPreference}
	}
	registration := sdk.Registration{Filters: []sdk.FilterDecl{declaration}}
	handler := func(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
		state.calls.Add(1)
		var verb string
		switch action {
		case sdk.FilterModify:
			verb = "MODIFY"
		case sdk.FilterReject:
			verb = "REJECT"
		case sdk.FilterWithdraw:
			verb = "WITHDRAW"
		case sdk.FilterAccept, sdk.FilterUnspecified:
			verb = "ACCEPT"
		default:
			panic("BUG: unknown import-filter fixture action")
		}
		fmt.Fprintf(os.Stderr, "filter-update %s: filter=%s peer=%s\n", verb, route.Filter(), route.Peer())
		if action == sdk.FilterModify {
			return p12SetLocalPreference(), nil
		}
		return &sdk.FilterUpdateOutput{Action: action}, nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool { return state.calls.Load() > 0 })
		if !p12WaitPeerCounter(ctx, plugin, "peer1", "eor-sent", 1) {
			return fmt.Errorf("ze sent no end-of-rib to peer1 before shutdown")
		}
		if forwardsToPeerTwo && !p12WaitPeerCounter(ctx, plugin, "127.0.0.2", "updates-sent", 2) {
			return fmt.Errorf("ze never forwarded the imported route to peer2")
		}
		if err := p12Quiesce(ctx, plugin); err != nil {
			return err
		}
		count := state.calls.Load()
		if count == 0 {
			return fmt.Errorf("import filter never called for the route ze-peer sent")
		}
		fmt.Fprintf(os.Stderr, "OK: "+success+"\n", count)
		return nil
	}
	return p12FilterDriver(name, &registration, handler, scenario)
}

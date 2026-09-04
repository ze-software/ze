package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	))
	Register("plugin/redistribute-import-modify", p12FilterImportDriver(
		"filter-modify-test",
		"set-localpref",
		[]string{fieldLocalPreference},
		sdk.FilterModify,
		"filter modified %d route(s)",
	))
	Register("plugin/redistribute-import-reject", p12FilterImportDriver(
		"filter-reject-test",
		"reject-all",
		[]string{fieldASPath, pipeOrigin},
		sdk.FilterReject,
		"filter rejected %d route(s)",
	))
}

func p12FilterDriver(name string, registration sdk.Registration, handler sdk.FilterUpdateHandler, scenario p12Scenario) Driver {
	return func(ctx context.Context, args []string) error {
		var marker string
		if name == "filter-export-modify-test" {
			if len(args) != 1 {
				return fmt.Errorf("%s requires an absolute readiness marker path", name)
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
			plugin.OnFilterUpdate(handler)
		}

		result := make(chan error, 1)
		plugin.OnAllPluginsReady(func() error {
			go func() {
				var scenarioErr error
				if marker != "" {
					if !p12WaitPeerCounter(ctx, plugin, "127.0.0.2", "eor-sent", 1) {
						scenarioErr = fmt.Errorf("peer2 did not finish initial sync before the source peer started")
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
		{Name: filterNameFirst, Direction: sdk.FilterImport, Attributes: []string{fieldASPath}, OnError: sdk.OnErrorReject},
		{Name: "second", Direction: sdk.FilterImport, Attributes: []string{fieldCommunity}, OnError: sdk.OnErrorReject},
	}}
	handler := func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		state.mu.Lock()
		state.order = append(state.order, input.Filter)
		call := len(state.order)
		state.mu.Unlock()
		fmt.Fprintf(os.Stderr, "filter-update called: filter=%s (call #%d)\n", input.Filter, call)
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
	return p12FilterDriver("filter-chain-test", registration, handler, scenario)
}

func p12FilterDeclareDriver() Driver {
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:       "accept-all",
		Direction:  sdk.FilterImport,
		Attributes: []string{fieldASPath, fieldCommunity},
		OnError:    sdk.OnErrorReject,
	}}}
	return p12FilterDriver("filter-declare-test", registration, nil, func(ctx context.Context, plugin *sdk.Plugin) error {
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
		Name:       "set-export-localpref",
		Direction:  sdk.FilterExport,
		Attributes: []string{fieldLocalPreference},
		OnError:    sdk.OnErrorReject,
	}}}
	handler := func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		if input.Direction == directionExport {
			state.exportModified.Add(1)
			fmt.Fprintf(os.Stderr, "filter-update EXPORT MODIFY: peer=%s update=%q\n", input.Peer, input.Update)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Update: "local-preference 200"}, nil
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
	return p12FilterDriver("filter-export-modify-test", registration, handler, scenario)
}

func p12FilterExportRejectDriver() Driver {
	state := new(p12FilterState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:       "block-export",
		Direction:  sdk.FilterExport,
		Attributes: []string{fieldASPath},
		OnError:    sdk.OnErrorReject,
	}}}
	handler := func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		if input.Direction != directionExport {
			return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
		}
		if strings.Contains(input.Update, "10.0.0.0/24") {
			state.exportRejected.Add(1)
			fmt.Fprintf(os.Stderr, "filter-update EXPORT REJECT: peer=%s update=%q\n", input.Peer, input.Update)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}, nil
		}
		state.exportPassed.Add(1)
		fmt.Fprintf(os.Stderr, "filter-update EXPORT PASS (fence): peer=%s update=%q\n", input.Peer, input.Update)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool {
			return state.exportRejected.Load() > 0 && state.exportPassed.Load() > 0
		})
		if !p12WaitPeerCounter(ctx, plugin, "127.0.0.2", "updates-sent", 2) {
			return fmt.Errorf("ze never wrote the fence UPDATE to peer2")
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
	return p12FilterDriver("filter-export-reject-test", registration, handler, scenario)
}

func p12FilterImportDriver(name, filterName string, attributes []string, action sdk.FilterAction, success string) Driver {
	state := new(p12FilterState)
	registration := sdk.Registration{Filters: []sdk.FilterDecl{{
		Name:       filterName,
		Direction:  sdk.FilterImport,
		Attributes: attributes,
		OnError:    sdk.OnErrorReject,
	}}}
	handler := func(input *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		state.calls.Add(1)
		var verb string
		switch action {
		case sdk.FilterModify:
			verb = "MODIFY"
		case sdk.FilterReject:
			verb = "REJECT"
		default:
			verb = "ACCEPT"
		}
		fmt.Fprintf(os.Stderr, "filter-update %s: filter=%s peer=%s\n", verb, input.Filter, input.Peer)
		output := &sdk.FilterUpdateOutput{Action: action}
		if action == sdk.FilterModify {
			output.Update = "local-preference 200"
		}
		return output, nil
	}
	scenario := func(ctx context.Context, plugin *sdk.Plugin) error {
		Poll(ctx, 100, 100*time.Millisecond, func() bool { return state.calls.Load() > 0 })
		if !p12WaitPeerCounter(ctx, plugin, "*", "eor-sent", 1) {
			return fmt.Errorf("ze sent no end-of-rib to peer1 before shutdown")
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
	return p12FilterDriver(name, registration, handler, scenario)
}

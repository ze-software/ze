// Design: docs/architecture/testing/ci-format.md -- compiled functional-test observers

// Package fixture provides compiled helper processes for .ci scenarios.
package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const observerFailure = "ZE-OBSERVER-FAIL"

// Driver runs one named fixture helper.
type Driver func(context.Context, []string) error

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]Driver)
)

// Register adds one compiled fixture driver. Duplicate names are defects.
func Register(name string, driver Driver) {
	if name == "" || strings.ContainsAny(name, " \t") || driver == nil {
		panic("fixture.Register: invalid driver")
	}
	driversMu.Lock()
	defer driversMu.Unlock()
	if _, exists := drivers[name]; exists {
		panic("fixture.Register: duplicate driver " + name)
	}
	drivers[name] = driver
}

// Names returns every registered fixture driver in command order.
func Names() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Run dispatches `ze-test fixture <name> [args...]`.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "fixture requires one of: %s\n", strings.Join(Names(), ", "))
		return 2
	}
	driversMu.RLock()
	driver := drivers[args[0]]
	driversMu.RUnlock()
	if driver == nil {
		fmt.Fprintf(os.Stderr, "unknown fixture %q; use one of: %s\n", args[0], strings.Join(Names(), ", "))
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// The caller has already removed the quotes: a .ci exec= value is split by
	// runner.splitCommand and a shell strips its own. Only the environment is
	// expanded here, so `$ZE_REPO_ROOT` reaches a driver as a path.
	driverArgs := make([]string, len(args)-1)
	for index, argument := range args[1:] {
		driverArgs[index] = os.ExpandEnv(argument)
	}
	if err := invokeDriver(ctx, driver, driverArgs); err != nil {
		ReportFailure(err)
		return 1
	}
	return 0
}

// ObserverScenario runs after all daemon plugins are ready.
type ObserverScenario func(context.Context, *sdk.Plugin) error

// observerSetup installs callbacks and startup subscriptions before Stage 1.
type observerSetup func(*sdk.Plugin) error

// Observe connects as a plugin, completes the five startup stages, runs the
// scenario when every daemon plugin is ready, and requests a clean shutdown.
func Observe(ctx context.Context, name string, registration sdk.Registration, scenario ObserverScenario) error {
	return observeConfigured(ctx, name, registration, nil, scenario)
}

// observeConfigured installs callbacks before startup, completes the five
// stages, runs the scenario when every daemon plugin is ready, and requests a
// clean shutdown.
func observeConfigured(
	ctx context.Context,
	name string,
	registration sdk.Registration,
	setup observerSetup,
	scenario ObserverScenario,
) error {
	plugin, err := newObserver(name)
	if err != nil {
		return fmt.Errorf("connect observer %s: %w", name, err)
	}
	defer plugin.Close() //nolint:errcheck // the run result carries the useful transport error
	if setup != nil {
		if err := setup(plugin); err != nil {
			return fmt.Errorf("configure observer %s: %w", name, err)
		}
	}

	result := make(chan error, 1)
	started := make(chan struct{})
	plugin.OnAllPluginsReady(func() error {
		close(started)
		go func() {
			scenarioErr := invokeScenario(ctx, plugin, scenario)
			if scenarioErr == nil {
				status, _, quiesceErr := plugin.DispatchCommand(ctx, "request quiesce")
				if quiesceErr == nil && status != statusDone {
					scenarioErr = fmt.Errorf("observer quiesce status is %q", status)
				}
			}
			// Reported HERE, before the shutdown below, because the runner reads
			// this sentinel out of the DAEMON's stderr: the engine relays a
			// plugin's stderr while that plugin's process and its own are alive
			// (plugin/process/process.go, relayStderrFrom). Run reports the same
			// error after Run returns, which is after the daemon was asked to
			// stop, and that line reaches the runner only if it wins the race
			// with the shutdown. Measured 2026-09-02: an observer asserting a
			// route that can never arrive still passed
			// test/plugin/rpki-group-action.ci, and reporting here made the same
			// assertion fail.
			ReportFailure(scenarioErr)
			result <- scenarioErr
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _, _ = plugin.DispatchCommand(shutdownCtx, "request shutdown")
		}()
		return nil
	})

	runErr := plugin.Run(ctx, registration)
	return awaitObserverResult(started, result, runErr)
}

// awaitObserverResult preserves a scenario's verdict when the plugin transport
// stops first. A completed scenario owns the verdict. Transport closure after a
// successful scenario is expected because the scenario asks the daemon to stop.
func awaitObserverResult(started <-chan struct{}, result <-chan error, runErr error) error {
	completed := func(scenarioErr error) error {
		if scenarioErr == nil {
			return nil
		}
		return errors.Join(scenarioErr, runErr)
	}
	select {
	case scenarioErr := <-result:
		return completed(scenarioErr)
	default:
	}
	select {
	case <-started:
	case scenarioErr := <-result:
		return completed(scenarioErr)
	default:
		return runErr
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case scenarioErr := <-result:
		return completed(scenarioErr)
	case <-timer.C:
		return errors.Join(errors.New("observer scenario did not finish after its plugin transport stopped"), runErr)
	}
}

// newObserver connects with the plugin name the daemon assigned to this
// process. The caller's name is only the fallback for direct SDK tests.
func newObserver(fallback string) (*sdk.Plugin, error) {
	name := env.Get("ze.plugin.name")
	if name == "" {
		name = fallback
	}
	return sdk.NewFromTLSEnv(name)
}

func invokeDriver(ctx context.Context, driver Driver, args []string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("fixture panic: %v", recovered)
		}
	}()
	return driver(ctx, args)
}

func invokeScenario(ctx context.Context, plugin *sdk.Plugin, scenario ObserverScenario) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("observer panic: %v", recovered)
		}
	}()
	return scenario(ctx, plugin)
}

// Dispatch returns one command's status and decoded JSON value.
func Dispatch(ctx context.Context, plugin *sdk.Plugin, command string, value any) (string, error) {
	status, raw, err := plugin.DispatchCommand(ctx, command)
	if err != nil {
		return status, err
	}
	if value != nil && len(raw) != 0 {
		if err := json.Unmarshal(raw, value); err != nil {
			return status, fmt.Errorf("decode %q: %w", command, err)
		}
	}
	return status, nil
}

// reportedOnce says the sentinel has already been written, so the second report
// of one failure is dropped instead of printed twice.
var reportedOnce atomic.Bool

// ReportFailure emits the sentinel the .ci runner treats as an authoritative
// observer failure. Quoting with %q matches slog's text format.
//
// The FIRST failure is the one emitted, and a later call is dropped. An observer
// scenario reports where it fails, while the daemon is still alive to relay the
// line, and the process then exits through Run, which holds the same error.
func ReportFailure(err error) {
	if err == nil {
		return
	}
	if reportedOnce.Swap(true) {
		return
	}
	fmt.Fprintf(os.Stderr, "time=runtime level=ERROR msg=%q subsystem=test.observer\n", observerFailure+": "+err.Error())
}

// Poll retries predicate until it succeeds, the attempt count is exhausted, or
// the context is canceled.
func Poll(ctx context.Context, attempts int, delay time.Duration, predicate func() bool) bool {
	if attempts < 1 {
		attempts = 1
	}
	for attempt := range attempts {
		if predicate() {
			return true
		}
		if attempt+1 == attempts {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
	return false
}

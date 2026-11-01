// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call
// Related: compiledin/register.go -- the compiled-in registration of this plugin
// Related: cmd/ze-fakefilter/main.go -- the same plugin built as its own binary
//
// Package fakefilter is a test-only filter plugin written against the public
// SDK alone, as a third party's Go filter compiled into the tree would be
// (spec-filter-wire-0-umbrella AC-35, R-13 bounds "compiled in"). It declares
// one import filter, fakefilter:judge, that reads the prefix, rejects a route
// carrying the one prefix a test names and accepts every other, and logs each
// prefix it judged with the transport that carried it. Run internal it is
// called through the typed DirectBridge slot; run forked (`ze plugin
// fakefilter`, or the ze-fakefilter binary that links this package and the
// SDK alone) it answers the batched filter-update message. Both come from
// the one sdk.OnFilter registration.
//
// It carries a stop trigger for AC-33: told to stop after N routes, it
// answers the Nth route and then ends its own run, so later routes meet a
// stopped plugin and take the member's on-error. Once the plugin is closed it
// creates the stopped file, so a test peer can hold its next route until the
// slot is known to answer "unavailable". Told to restart, it declares the
// restart failure policy, so ze starts it again; a run that finds the stopped
// file already there is that second generation: it never stops, and once its
// startup is done it creates the respawned file, so a test peer can hold its
// next route until the new slot answers. Every judged route is logged with the
// generation that judged it.
package fakefilter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// Name is the plugin's registry name and its name on the process protocol.
	Name = "fakefilter"
	// FilterName is the filter a chain references as fakefilter:judge.
	FilterName = "judge"
	// fieldPrefix is the one route field the filter reads (the vocabulary of
	// docs/plugin-development/protocol.md, "The Route Fields of filter-update").
	fieldPrefix = "prefix"

	// The transports a judged route is reported to have arrived on: the
	// engine's typed view through the DirectBridge slot, or the encoded
	// fields of a filter-update batch.
	transportSlot  = "slot"
	transportBatch = "batch"
)

// FilterDecls is the plugin's one filter declaration. It has two readers and
// MUST stay one function: compiledin puts it on registry.Registration.Filters,
// which config load reads before any plugin runs, and Run sends it in the
// Stage 1 registration, which the daemon judges the chain against. On-error is
// reject, so a route the plugin cannot judge, or meets stopped, fails closed.
func FilterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{{
		Name:      FilterName,
		Direction: sdk.FilterImport,
		Reads:     []string{fieldPrefix},
		OnError:   sdk.OnErrorReject,
	}}
}

// judgement is what the evaluator reports for one prefix of a judged route.
type judgement struct {
	generation int // 1, or 2 for the run ze started again
	peer       string
	prefix     string
	transport  string // transportSlot or transportBatch
	verdict    string // "accept" or "reject"
}

// judge is the evaluator's state: the prefix it rejects, how many routes it
// judged, and the stop trigger. Safe for concurrent use: the count is atomic,
// stop is called once, by the route that reaches stopAfter, and report MUST
// be safe for concurrent use itself (the slot runs on every caller's
// goroutine).
type judge struct {
	// rejectPrefix is the prefix whose routes are rejected; "" rejects none.
	rejectPrefix string
	// stopAfter is the route count after which the plugin stops; 0 never.
	stopAfter int64
	// generation is 1, or 2 for the run ze started again after a stop.
	generation int
	judged     atomic.Int64
	// stop ends the plugin's run. It MUST NOT block: it runs inside the slot
	// call, on the caller's goroutine (the fast-path contract).
	stop   func()
	report func(judgement)
}

func newJudge(tr Trigger, stop func(), report func(judgement)) *judge {
	return &judge{rejectPrefix: tr.RejectPrefix, stopAfter: tr.StopAfter, generation: tr.generation,
		stop: stop, report: report}
}

// routeFields is the part of the fields object the filter reads: family to
// the announced NLRI.
type routeFields struct {
	Prefix map[string][]string `json:"prefix"`
}

// evaluate is the plugin's one evaluator. It rejects a route that carries
// rejectPrefix, accepts every other route it can read, and refuses one it
// cannot, so the member's on-error decides that route rather than a verdict
// the fixture made up. It keeps nothing from the route past the call: the
// unmarshal copies what it reports.
func (j *judge) evaluate(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	raw, err := route.Fields()
	if err != nil {
		return nil, fmt.Errorf("fakefilter: read the declared fields: %w", err)
	}
	var fields routeFields
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("fakefilter: decode the declared fields: %w", err)
	}
	action, verdict := sdk.FilterAccept, "accept"
	if routeCarries(&fields, j.rejectPrefix) {
		action, verdict = sdk.FilterReject, "reject"
	}
	// The engine's typed view reaches the evaluator only through the slot;
	// a batched route carries its fields encoded and no view.
	transport := transportBatch
	if route.Engine() != nil {
		transport = transportSlot
	}
	for _, prefixes := range fields.Prefix {
		for _, prefix := range prefixes {
			j.report(judgement{generation: j.generation, peer: route.Peer(), prefix: prefix, transport: transport, verdict: verdict})
		}
	}
	if j.judged.Add(1) == j.stopAfter {
		j.stop()
	}
	return &sdk.FilterUpdateOutput{Action: action}, nil
}

// routeCarries answers whether the route announces prefix; "" is carried by
// no route.
func routeCarries(fields *routeFields, prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, prefixes := range fields.Prefix {
		if slices.Contains(prefixes, prefix) {
			return true
		}
	}
	return false
}

// The trigger's keys, in their dotted spelling. A forked copy is started
// through `sh -c`, which drops a variable whose name is not a shell
// identifier, so a test sets the underscore spelling
// (ze_fakefilter_reject_prefix); no key holds a hyphen, which has no such
// spelling.
const (
	EnvRejectPrefix = "ze.fakefilter.reject.prefix"
	EnvStopAfter    = "ze.fakefilter.stop.after"
	EnvStoppedFile  = "ze.fakefilter.stopped.file"
	EnvRestart      = "ze.fakefilter.restart"
	EnvRespawnFile  = "ze.fakefilter.respawned.file"
)

// TriggerFrom reads the trigger through lookup, which answers a key's value
// or "" when it is unset: the compiled-in copy passes ze's env.Get, the
// binary reads its own environment. A malformed number or boolean is refused.
func TriggerFrom(lookup func(key string) string) (Trigger, error) {
	tr := Trigger{
		RejectPrefix:  lookup(EnvRejectPrefix),
		StoppedFile:   lookup(EnvStoppedFile),
		RespawnedFile: lookup(EnvRespawnFile),
	}
	if value := lookup(EnvStopAfter); value != "" {
		stopAfter, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return Trigger{}, fmt.Errorf("%s %q is not a whole number: %w", EnvStopAfter, value, err)
		}
		tr.StopAfter = stopAfter
	}
	if value := lookup(EnvRestart); value != "" {
		restart, err := strconv.ParseBool(value)
		if err != nil {
			return Trigger{}, fmt.Errorf("%s %q is not true or false: %w", EnvRestart, value, err)
		}
		tr.Restart = restart
	}
	return tr, nil
}

// Trigger is what a test asks of the plugin: reject the routes that carry
// RejectPrefix ("" none), stop after StopAfter routes (0 never), and create
// StoppedFile ("" none) once the plugin is closed.
type Trigger struct {
	RejectPrefix string
	StopAfter    int64
	StoppedFile  string
	// Restart declares the restart failure policy, so ze starts the plugin
	// again once it stops; RespawnedFile ("" none) is created once that second
	// generation's startup is done.
	Restart       bool
	RespawnedFile string
	// generation is set by startGeneration, never by the environment.
	generation int
}

// startGeneration answers the trigger this run obeys. A run that finds the
// stopped file is the generation ze started again after the stop: it never
// stops, so the test sees one stop and one respawn, not a respawn loop.
func startGeneration(tr Trigger) Trigger {
	tr.generation = 1
	if tr.StoppedFile == "" {
		return tr
	}
	if _, err := os.Stat(tr.StoppedFile); err != nil {
		return tr
	}
	tr.generation = 2
	tr.StopAfter = 0
	return tr
}

// Registration is what the plugin declares at Stage 1: its filter, and the
// restart failure policy when the test asks for it.
func Registration(tr Trigger) *sdk.Registration {
	reg := &sdk.Registration{Filters: FilterDecls()}
	if tr.Restart {
		reg.FailurePolicy = sdk.FailureRestart
	}
	return reg
}

// Run is the plugin's engine over p, built on the connection the daemon gives
// it (internal, sdk.NewWithConn) or on the TLS connect-back (forked,
// sdk.NewFromTLSEnv). It returns the exit code.
func Run(p *sdk.Plugin, tr Trigger, log *slog.Logger) int {
	tr = startGeneration(tr)

	signalled, cancelSignals := sdk.SignalContext()
	defer cancelSignals()
	ctx, stop := context.WithCancel(signalled)
	defer stop()

	report := func(j judgement) {
		log.Info("fakefilter judged a route", "filter", FilterName, "generation", j.generation, "peer", j.peer, "prefix", j.prefix,
			"transport", j.transport, "verdict", j.verdict)
	}
	p.OnFilter(newJudge(tr, stop, report).evaluate)
	if tr.generation == 2 && tr.RespawnedFile != "" {
		p.OnStarted(func(context.Context) error {
			log.Info("fakefilter started again", "generation", tr.generation)
			return os.WriteFile(tr.RespawnedFile, nil, 0o600)
		})
	}
	runErr := p.Run(ctx, Registration(tr))
	// Only the first generation stops, so only it creates the stopped file.
	if tr.generation != 1 {
		tr.StoppedFile = ""
	}
	// Closed before the stopped file exists: once it does, the bridge slot
	// answers ErrBridgeClosed, which a test holding on the file relies on.
	closeErr := p.Close()
	// Run answers nil when the engine closed the connection first, its clean
	// shutdown; the Close after it then meets the closed connection, which is
	// that same shutdown and not a failure.
	if errors.Is(closeErr, net.ErrClosed) {
		closeErr = nil
	}
	if ctx.Err() != nil {
		log.Info("fakefilter stopped", "stop-after", tr.StopAfter)
	}
	if tr.StoppedFile != "" {
		if err := os.WriteFile(tr.StoppedFile, nil, 0o600); err != nil {
			log.Error("fakefilter: create the stopped file", "path", tr.StoppedFile, "error", err)
			return 1
		}
	}
	if err := errors.Join(runErr, closeErr); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("fakefilter: run", "error", err)
		return 1
	}
	return 0
}

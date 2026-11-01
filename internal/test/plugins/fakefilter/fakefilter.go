// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call
// Related: register.go -- the compiled-in registration and the stop trigger's env keys
//
// Package fakefilter is a test-only filter plugin written against the public
// SDK alone, as a third party's Go filter compiled into the tree would be
// (spec-filter-wire-0-umbrella AC-35, R-13 bounds "compiled in"). It declares
// one import filter, fakefilter:judge, that reads the prefix, rejects a route
// carrying the one prefix a test names and accepts every other, and logs each
// prefix it judged with the transport that carried it. Run internal it is
// called through the typed DirectBridge slot; run forked (`ze plugin
// fakefilter`) it answers the batched filter-update message. Both come from
// the one sdk.OnFilter registration.
//
// It carries a stop trigger for AC-33: told to stop after N routes, it
// answers the Nth route and then ends its own run, so later routes meet a
// stopped plugin and take the member's on-error. Once the plugin is closed it
// creates the stopped file, so a test peer can hold its next route until the
// slot is known to answer "unavailable".
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

// filterDecls is the plugin's one filter declaration. It has two readers and
// MUST stay one function: register.go puts it on registry.Registration.Filters,
// which config load reads before any plugin runs, and run sends it in the
// Stage 1 registration, which the daemon judges the chain against. On-error is
// reject, so a route the plugin cannot judge, or meets stopped, fails closed.
func filterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{{
		Name:      FilterName,
		Direction: sdk.FilterImport,
		Reads:     []string{fieldPrefix},
		OnError:   sdk.OnErrorReject,
	}}
}

// judgement is what the evaluator reports for one prefix of a judged route.
type judgement struct {
	peer      string
	prefix    string
	transport string // transportSlot or transportBatch
	verdict   string // "accept" or "reject"
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
	judged    atomic.Int64
	// stop ends the plugin's run. It MUST NOT block: it runs inside the slot
	// call, on the caller's goroutine (the fast-path contract).
	stop   func()
	report func(judgement)
}

func newJudge(tr trigger, stop func(), report func(judgement)) *judge {
	return &judge{rejectPrefix: tr.rejectPrefix, stopAfter: tr.stopAfter, stop: stop, report: report}
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
			j.report(judgement{peer: route.Peer(), prefix: prefix, transport: transport, verdict: verdict})
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

// trigger is what a test asks of the plugin: reject the routes that carry
// rejectPrefix ("" none), stop after stopAfter routes (0 never), and create
// stoppedFile ("" none) once the plugin is closed.
type trigger struct {
	rejectPrefix string
	stopAfter    int64
	stoppedFile  string
}

// run is the plugin's engine, over the connection the daemon gives it
// (internal) or the TLS connect-back (forked). It returns the exit code.
func run(conn net.Conn, tr trigger, log *slog.Logger) int {
	p := sdk.NewWithConn(Name, conn)

	signalled, cancelSignals := sdk.SignalContext()
	defer cancelSignals()
	ctx, stop := context.WithCancel(signalled)
	defer stop()

	report := func(j judgement) {
		log.Info("fakefilter judged a route", "filter", FilterName, "peer", j.peer, "prefix", j.prefix,
			"transport", j.transport, "verdict", j.verdict)
	}
	p.OnFilter(newJudge(tr, stop, report).evaluate)
	runErr := p.Run(ctx, &sdk.Registration{Filters: filterDecls()})
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
		log.Info("fakefilter stopped", "stop-after", tr.stopAfter)
	}
	if tr.stoppedFile != "" {
		if err := os.WriteFile(tr.stoppedFile, nil, 0o600); err != nil {
			log.Error("fakefilter: create the stopped file", "path", tr.stoppedFile, "error", err)
			return 1
		}
	}
	if err := errors.Join(runErr, closeErr); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("fakefilter: run", "error", err)
		return 1
	}
	return 0
}

// Design: docs/architecture/rib/unified-locrib.md -- system-RIB routing-view consumer.
// Related: register.go -- config and runtime lifecycle callbacks.

package sysrib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	routeViewConfigurationTimeout             = routeViewSnapshotLifetime
	routeViewConfigurationCompensationTimeout = routeViewSnapshotLifetime
	routeViewConfigurationApplyBudget         = 5 * time.Minute
)

// routeViewClient owns the authenticated consumer lease. Its constructor must
// complete authoritative distance configuration before returning the lease.
type routeViewClient struct {
	plugin        *sdk.Plugin
	rib           *sysRIB
	view          rpc.RouteViewOpenOutput
	reader        routeViewClientReader
	configuration routeViewConfiguration
	distanceOp    uint64
	mu            sync.Mutex
	worker        *routeViewWorker
	health        *routeViewHealth
	failure       error
	replays       map[uint64]uint64
	wake          chan struct{}
}

// The input variant is fixed by authenticated execution placement. A local
// reader carries an initialized authority; a selected reader is synchronized
// by the runtime worker before it constructs a forwarding resolver.
type routeViewClientReader interface {
	routeViewClientReader()
}

type routeViewClientLocal struct{ rib *locrib.RIB }
type routeViewClientSelected struct{}

func (routeViewClientLocal) routeViewClientReader()    {}
func (routeViewClientSelected) routeViewClientReader() {}

func openRouteViewClient(ctx context.Context, p *sdk.Plugin, s *sysRIB, dist map[string]int, permit map[string]bool) (*routeViewClient, error) {
	if p == nil {
		return nil, routeViewRefusal("The system RIB consumer is unavailable", "A configured SDK connection and forwarding state are required", "ze request reload")
	}
	return openRouteViewMonitoredClient(ctx, p, s, dist, permit, newRouteViewHealth(p.MetricsRegistry(), p.InstanceName))
}

func openRouteViewMonitoredClient(ctx context.Context, p *sdk.Plugin, s *sysRIB, dist map[string]int, permit map[string]bool, health *routeViewHealth) (*routeViewClient, error) {
	if s == nil {
		return nil, routeViewRefusal("The system RIB consumer is unavailable", "A configured SDK connection and forwarding state are required", "ze request reload")
	}
	rows, err := routeViewClientDistances(dist)
	if err != nil {
		return nil, err
	}
	var reader routeViewClientReader = routeViewClientSelected{}
	input := rpc.RouteViewOpenInput{}
	if p.IsInternal() {
		local := locrib.Default()
		if local == nil {
			return nil, routeViewRefusal("The internal system RIB has no authoritative reader", "The daemon's Loc-RIB must be initialized before the local consumer starts", "ze request reload")
		}
		reader = routeViewClientLocal{rib: local}
		input.Reader = rpc.RouteViewReaderLocalAuthority
	}
	operation, cancel := context.WithTimeout(ctx, routeViewConfigurationTimeout)
	defer cancel()
	opened, err := p.RouteViewOpen(operation, &input)
	if err != nil {
		return nil, fmt.Errorf("open authoritative system routing view: %w", err)
	}
	if opened.Cursor != 0 {
		return nil, routeViewRefusal("The system routing view has an invalid initial cursor", "A fresh consumer starts at cursor zero", "ze request reload")
	}
	ack, err := p.RouteViewDistances(operation, &rpc.RouteViewDistancesInput{
		Session: opened.Session, Operation: 1, Distances: rows,
	})
	if err != nil {
		// Close is runtime-only. During configure the startup refusal makes the
		// server stop this generation, whose registered cleanup owns the lease.
		return nil, fmt.Errorf("apply authoritative system routing distances: %w", err)
	}
	if ack.Session != opened.Session || ack.Operation != 1 {
		return nil, routeViewRefusal("The system RIB distance acknowledgment does not match its request", "The consumer cannot prove which declaration the daemon applied", "ze request reload")
	}
	if _, local := reader.(routeViewClientLocal); local && ack.Cursor != 0 {
		return nil, routeViewRefusal("The local system RIB received a stream cursor", "An authoritative local reader acknowledges configuration at cursor zero", "ze request reload")
	}
	configuration := routeViewConfiguration{distances: maps.Clone(dist), permitted: maps.Clone(permit)}
	// These are consumer display and FIB permissions only. The acknowledged
	// operation above is the sole publisher of the daemon distance declaration.
	s.mu.Lock()
	s.adminDist = configuration.distances
	s.mu.Unlock()
	s.fibMu.Lock()
	s.fibPermit = configuration.permitted
	clear(s.fibSpoken)
	s.fibMu.Unlock()
	return &routeViewClient{plugin: p, rib: s, view: *opened, reader: reader, configuration: configuration, distanceOp: 1, health: health, replays: make(map[uint64]uint64), wake: make(chan struct{}, 1)}, nil
}

func routeViewClientDistances(distances map[string]int) ([]rpc.RouteViewDistance, error) {
	rows := make([]rpc.RouteViewDistance, 0, len(distances))
	for protocol, value := range distances {
		if value < 0 || value > 255 {
			return nil, routeViewRefusal("The system RIB distance declaration contains an invalid value", "Every administrative distance must fit the declared byte range", "ze config validate -")
		}
		rows = append(rows, rpc.RouteViewDistance{Protocol: protocol, Distance: uint8(value)}) //nolint:gosec // range checked above
	}
	slices.SortFunc(rows, func(a, b rpc.RouteViewDistance) int {
		if a.Protocol < b.Protocol {
			return -1
		}
		if a.Protocol > b.Protocol {
			return 1
		}
		return 0
	})
	return rows, nil
}

// Close MUST cancel and join the consumer worker before closing its lease.
// Its caller MUST keep the SDK connection open until this method returns.
func (client *routeViewClient) Close(ctx context.Context) error {
	client.health.stop()
	client.mu.Lock()
	worker := client.worker
	client.mu.Unlock()
	if worker != nil {
		worker.cancel()
		<-worker.done
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), routeViewConfigurationCompensationTimeout)
	defer cancel()
	ack, err := client.plugin.RouteViewClose(cleanup, &rpc.RouteViewCloseInput{Session: client.view.Session})
	if err != nil {
		// A stopped transport transfers cleanup to the daemon's exact process
		// generation. A live RPC refusal remains an output-cleanup failure.
		if routeViewTransportStopped(err) {
			return nil
		}
		client.health.fail(err)
		return err
	}
	if ack.Session != client.view.Session {
		err := routeViewRefusal("The system RIB close acknowledgment belongs to another session", "Cleanup of the consumer's output was not acknowledged", "ze request reload")
		client.health.fail(err)
		return err
	}
	return nil
}

// routeViewTransportStopped reports whether an engine call failed because this
// plugin's session with the engine ended: the mux reader's terminal error
// (EOF from the peer's close, a closed pipe or socket), a closed mux, or a
// fenced DirectBridge. The worker and Close read it as a normal stop. Only the
// transport produces these values: a live owner's refusal arrives as an
// *rpc.RPCCallError, which carries text and no wrapped cause, so a refusal that
// mentions EOF or a closed connection still fails. Reply decoding uses
// json.Unmarshal, which reports truncated input as a syntax error, never EOF.
func routeViewTransportStopped(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, rpc.ErrMuxConnClosed) {
		return true
	}
	return errors.Is(err, rpc.ErrBridgeClosed)
}

// Failure reports the retained worker cause after Run or Close returns. The
// owner MUST check it before treating a closed SDK loop as a clean shutdown.
func (client *routeViewClient) Failure() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.failure
}

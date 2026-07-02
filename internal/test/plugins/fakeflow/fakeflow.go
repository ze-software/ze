// Design: docs/architecture/core-design.md -- in-process test injector for observation.Feed
//
// Package fakeflow is a test-only internal plugin that publishes synthetic
// flow-byte observations into the process-global observation.Feed, so a `.ci`
// functional test can drive the anomaly facts->judgment->response chain
// (trafficfeature -> anomaly/detect -> anomaly/shape) end to end without real
// kernel traffic. Internal plugins run in-process (goroutines, see
// internal/component/plugin/process/process.go startInternal), so
// observation.Global().Publish reaches the DUT's trafficfeature subscriber.
//
// Command:
//
//	request fakeflow inject <src> <dst> <dst-port> <bytes> [<count>]
//
// The plugin publishes nothing unless its inject command is invoked, so it has
// zero runtime cost when idle. It is loaded only in the zetest DUT build via
// internal/test/plugins/all and cmd/ze/plugins_zetest.go.
package fakeflow

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/observation"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// Name is the canonical plugin name.
const Name = "fakeflow"

// injectMaxCount bounds a single inject invocation so a pathological CLI call
// cannot flood the observation feed and wedge the DUT.
const injectMaxCount = 100_000

var loggerPtr atomic.Pointer[slog.Logger]

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

var errUsageInject = errors.New("usage: request fakeflow inject <src> <dst> <dst-port> <bytes> [<count>]")

// injectParams is a parsed inject request.
type injectParams struct {
	src, dst netip.Addr
	dstPort  uint16
	bytes    float64
	count    int
}

// parseInject splits "<src> <dst> <dst-port> <bytes> [<count>]".
func parseInject(args []string) (injectParams, error) {
	if len(args) < 4 || len(args) > 5 {
		return injectParams{}, errUsageInject
	}
	src, err := netip.ParseAddr(args[0])
	if err != nil {
		return injectParams{}, fmt.Errorf("invalid src %q: %w", args[0], err)
	}
	dst, err := netip.ParseAddr(args[1])
	if err != nil {
		return injectParams{}, fmt.Errorf("invalid dst %q: %w", args[1], err)
	}
	port, err := strconv.ParseUint(args[2], 10, 16)
	if err != nil {
		return injectParams{}, fmt.Errorf("invalid dst-port %q: %w", args[2], err)
	}
	bytes, err := strconv.ParseFloat(args[3], 64)
	if err != nil || bytes <= 0 {
		return injectParams{}, fmt.Errorf("invalid bytes %q (want a number > 0)", args[3])
	}
	count := 1
	if len(args) == 5 {
		count, err = strconv.Atoi(args[4])
		if err != nil || count <= 0 {
			return injectParams{}, fmt.Errorf("invalid count %q (want a positive integer)", args[4])
		}
		if count > injectMaxCount {
			return injectParams{}, fmt.Errorf("count %d exceeds maximum %d", count, injectMaxCount)
		}
	}
	return injectParams{src: src, dst: dst, dstPort: uint16(port), bytes: bytes, count: count}, nil
}

// publishFlow is indirected through a package var so unit tests can capture the
// published observations without the process-global feed.
var publishFlow = func(obs observation.Observation) { observation.Global().Publish(obs) }

// runInject parses the args and publishes count flow-byte observations,
// returning the number published.
func runInject(args []string) (int, error) {
	p, err := parseInject(args)
	if err != nil {
		return 0, err
	}
	obs := observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow:    observation.FlowKey{Src: p.src, Dst: p.dst, DstPort: p.dstPort},
		Value:   p.bytes,
		At:      time.Now(),
	}
	for range p.count {
		publishFlow(obs)
	}
	logger().Debug("fakeflow: injected",
		"src", p.src, "dst", p.dst, "dst-port", p.dstPort, "bytes", p.bytes, "count", p.count)
	return p.count, nil
}

// dispatchCommand is the OnExecuteCommand entry point. The engine routes by
// matched prefix; we receive it as command and the remaining tokens as args.
func dispatchCommand(_, command string, args []string, _ string) (string, any, error) {
	if command == "request fakeflow inject" {
		n, err := runInject(args)
		if err != nil {
			return rpc.StatusError, "", err
		}
		return rpc.StatusDone, map[string]any{"published": n}, nil
	}
	if command == "show fakeflow selfcheck" {
		return rpc.StatusDone, selfCheck(), nil
	}
	if command == "show fakeflow help" {
		return rpc.StatusDone, map[string]any{"help": helpStub()}, nil
	}
	return rpc.StatusError, "", fmt.Errorf("unknown command: %s", command)
}

// selfCheck (diagnostic) subscribes to this process's observation.Global(),
// publishes one flow, and reports how many its own subscription received plus
// the PID -- proving whether Publish reaches feed subscribers in THIS process.
func selfCheck() map[string]any {
	feed := observation.Global()
	var count atomic.Int64
	id := feed.Subscribe("fakeflow-selfcheck", func(observation.Observation) { count.Add(1) })
	defer feed.Unsubscribe(id)
	feed.Publish(observation.Observation{
		Kind:    observation.KindFlow,
		Feature: observation.FeatureFlowBytes,
		Flow:    observation.FlowKey{Src: netip.MustParseAddr("10.0.0.250"), Dst: netip.MustParseAddr("203.0.113.250"), DstPort: 9},
		Value:   1,
	})
	time.Sleep(200 * time.Millisecond) // let async channel delivery run
	return map[string]any{"pid": os.Getpid(), "self_received": count.Load()}
}

func helpStub() string {
	return "request fakeflow inject <src> <dst> <dst-port> <bytes> [<count>]"
}

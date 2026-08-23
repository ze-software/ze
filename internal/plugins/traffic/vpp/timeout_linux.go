// Design: docs/architecture/traffic/fw-7b-backend-hardening.md -- the vppOps seam and its constructor
// Related: ops_linux.go -- govppOps, the type this constructor returns
// Related: backend_linux.go -- Apply, the one caller

//go:build linux

package trafficvpp

import (
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/env"
)

const (
	// defaultReplyTimeout bounds one VPP binary-API round-trip.
	//
	// govpp's own default is core.DefaultReplyTimeout, which is 0 and which
	// govpp documents as disabling the timeout. That is not a usable default
	// here: receiveReplyInternal reads a value at or below zero as maxInt64,
	// about 292 years, and Channel.ReceiveReply takes no context, so the ctx
	// Apply gets from the plugin lifecycle cannot end the wait. Apply holds
	// b.mu across the whole call, so one unanswered reply also stops the
	// backend accepting any further apply. Matches the firewall VPP backend,
	// which bounds one round trip on the same socket.
	defaultReplyTimeout = 10 * time.Second
	// minReplyTimeout is the floor. Zero is deliberately NOT accepted: it is
	// govpp's spelling of "no deadline", the defect this removes.
	minReplyTimeout = 1 * time.Second
	// maxReplyTimeout is the ceiling. 60s matches the two firewall backends,
	// whose clamp docs/architecture/core-design.md publishes under "Firewall
	// reconcile concurrency". That table covers those two backends and not this
	// one, so the number is matched on purpose rather than inherited.
	//
	// It is stated here rather than taken from firewall.MaxBackendDeadline.
	// That constant exists so THREE firewall things agree: the nft clamp, the
	// vpp clamp, and the last finite bucket of the apply-latency histogram
	// (internal/component/firewall/metrics.go). Traffic has none of the three,
	// so the import would buy coupling and no agreement.
	maxReplyTimeout = 60 * time.Second
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.traffic.vpp.reply-timeout",
	Type:        "duration",
	Default:     "10s",
	Description: "Bound on each VPP binary-API round-trip; a wedged dataplane fails the traffic apply instead of blocking the backend for the life of the process",
})

// vppReplyTimeout returns the per-request deadline, clamped to
// [minReplyTimeout, maxReplyTimeout]. An out-of-range or unparseable value
// clamps rather than disabling the bound, because an unbounded call is the
// failure mode this guard exists to prevent.
func vppReplyTimeout() time.Duration {
	d := env.GetDuration("ze.traffic.vpp.reply-timeout", defaultReplyTimeout)
	return min(max(d, minReplyTimeout), maxReplyTimeout)
}

// newGovppOps installs the reply deadline on a channel and then wraps it in the
// ops facade, so the bound is in place before any request can be sent.
//
// The deadline is bound HERE rather than at the call site so it cannot be
// forgotten: every production request goes through a govppOps, and this is the
// only place one is built. The compiler does not hold that -- govppOps is
// unexported, so a bare literal stays legal anywhere in this package -- so
// TestGovppOpsIsBuiltOnlyByItsConstructor (ops_construction_test.go) parses the
// package's own sources and fails on a govppOps built anywhere but here. A
// computed-but-uninstalled deadline is indistinguishable from having none, and
// govpp's default is none.
//
// That test is a RATCHET, not a proof, and the difference matters if you are
// editing this function. It reads the three forms that name the type directly:
// a composite literal, new(govppOps), and a var declaration with no
// initializer. It does NOT see a facade built as part of another value --
// []govppOps{{ch: ch}} elides the inner literal's type, and a struct carrying a
// govppOps field builds one whenever the outer value is built. Seeing those
// needs the type of an expression, so go/types rather than a parse. What the
// test closes is the regression that happened: the inline &govppOps{ch: ch}
// that Apply carried before this constructor existed.
//
// Binding on every construction is what makes the value deterministic. The
// channel comes from a pool on the one Connection every plugin shares, and
// (*Channel).Reset drains the buffers while leaving replyTimeout alone, so a
// pooled channel arrives carrying whatever its previous owner set.
func newGovppOps(ch api.Channel) *govppOps {
	ch.SetReplyTimeout(vppReplyTimeout())
	return &govppOps{ch: ch}
}

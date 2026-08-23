//go:build linux

package trafficvpp

import (
	"testing"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/env"
)

// TestVppReplyTimeoutBounds covers the boundary table for the VPP reply
// deadline: the range 1s to 60s, and the 10s default. The numbers mirror the
// firewall VPP backend, because both bound one round trip on the same socket.
//
// VALIDATES: AC-2 and AC-3 of traffic-vpp-deferred-reply-timeout -- the
// deadline is always bounded, and no operator input can configure it away.
//
// PREVENTS: restoring govpp's own default by accident. core.DefaultReplyTimeout
// is 0, and receiveReplyInternal reads a value at or below zero as maxInt64,
// which is about 292 years. Apply holds b.mu across the whole call, so the
// backend accepts no further apply for as long as VPP stays silent.
func TestVppReplyTimeoutBounds(t *testing.T) {
	for _, tt := range []struct {
		name string
		set  string
		want time.Duration
	}{
		{"unset uses the default", "", 10 * time.Second},
		{"last valid high", "60s", 60 * time.Second},
		{"invalid above clamps down", "61s", 60 * time.Second},
		{"far above clamps down", "10m", 60 * time.Second},
		{"last valid low", "1s", 1 * time.Second},
		{"below the floor clamps up", "500ms", 1 * time.Second},
		{"zero clamps up, never disables the bound", "0s", 1 * time.Second},
		{"negative clamps up", "-1s", 1 * time.Second},
		{"unparseable falls back to the default", "not-a-duration", 10 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ze.traffic.vpp.reply-timeout", tt.set)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			got := vppReplyTimeout()
			if got != tt.want {
				t.Errorf("vppReplyTimeout() = %s, want %s", got, tt.want)
			}
			if got < minReplyTimeout || got > maxReplyTimeout {
				t.Errorf("vppReplyTimeout() = %s, outside the %s..%s bound", got, minReplyTimeout, maxReplyTimeout)
			}
		})
	}
}

// recordingChannel is an api.Channel that records the reply deadline set on it.
// Only SetReplyTimeout carries behavior; the rest satisfy the interface. Every
// unused method panics rather than returning a zero value, so a test that grew
// a request path would say so instead of passing on an invented reply.
type recordingChannel struct {
	timeout time.Duration
	set     bool
}

func (c *recordingChannel) SetReplyTimeout(d time.Duration) { c.timeout, c.set = d, true }
func (c *recordingChannel) SendRequest(api.Message) api.RequestCtx {
	panic("unused")
}
func (c *recordingChannel) SendMultiRequest(api.Message) api.MultiRequestCtx { panic("unused") }
func (c *recordingChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	panic("unused")
}
func (c *recordingChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *recordingChannel) Close()                                 {}

// TestNewGovppOpsBindsReplyTimeout drives the constructor that Apply uses and
// asserts the deadline it installs on the channel it is handed. The channel is
// this test's own fake, not the production one. What ties the assertion to
// production is that Apply builds its facade through this constructor and
// nothing else in the package builds one at all, which
// TestGovppOpsIsBuiltOnlyByItsConstructor holds in place.
//
// What this pins, exactly: the ops facade the production path builds carries a
// bounded, non-zero deadline on its own channel, for every operator input. It
// does NOT pin that a wedged VPP unblocks. The wait itself lives inside govpp's
// receiveReplyInternal, which no fake channel can stand in for, and the traffic
// package has no harness that reaches a live VPP.
//
// The boundary rows run through the CONSTRUCTOR rather than through
// vppReplyTimeout alone, because the value that matters is the one INSTALLED,
// not the one computed. A clamp that returns 1s and a constructor that installs
// nothing look identical from the helper's side.
//
// VALIDATES: AC-1 of traffic-vpp-deferred-reply-timeout -- every binary-API
// reply the traffic backend waits for is bounded.
//
// PREVENTS: computing a deadline and never installing it. A govpp channel keeps
// core.DefaultReplyTimeout (0, disabled) until SetReplyTimeout is called, so an
// uninstalled deadline is indistinguishable from having none. The channel also
// comes from a pool shared with every other plugin, and (*Channel).Reset leaves
// replyTimeout alone, so without this call the traffic backend inherits
// whichever value the previous owner happened to leave behind.
func TestNewGovppOpsBindsReplyTimeout(t *testing.T) {
	for _, tt := range []struct {
		name string
		set  string
		want time.Duration
	}{
		{"an explicit value reaches the channel", "3s", 3 * time.Second},
		{"unset installs the default", "", 10 * time.Second},
		{"zero is clamped before it reaches the channel", "0s", 1 * time.Second},
		{"above the ceiling is clamped before it reaches the channel", "10m", 60 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ze.traffic.vpp.reply-timeout", tt.set)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			ch := &recordingChannel{}
			ops := newGovppOps(ch)

			if ops.ch != ch {
				t.Fatal("newGovppOps did not wire the channel it was given")
			}
			if !ch.set {
				t.Fatal("SetReplyTimeout was never called: the channel keeps govpp's disabled default")
			}
			if ch.timeout != tt.want {
				t.Errorf("reply timeout = %s, want %s", ch.timeout, tt.want)
			}
			if ch.timeout <= 0 {
				t.Errorf("reply timeout = %s: govpp reads a value at or below zero as no deadline at all", ch.timeout)
			}
		})
	}
}

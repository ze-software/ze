//go:build linux

package firewallvpp

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/core"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/env"
)

// TestVppReplyTimeoutBounds covers the boundary table for the VPP reply
// deadline: range 1..60s, default 10s, mirroring the nft backend.
//
// VALIDATES: fixit-firewall-concurrency-deadlock D-2 for the second backend --
// the deadline is always bounded and cannot be configured away.
//
// PREVENTS: restoring govpp's own default. core.DefaultReplyTimeout is 0, which
// govpp documents as "disabled": every ReceiveReply then blocks forever, and
// ApplyAll holds the process-wide reconcileMu across it.
func TestVppReplyTimeoutBounds(t *testing.T) {
	for _, tt := range []struct {
		name string
		set  string
		want time.Duration
	}{
		{"unset uses the default", "", 10 * time.Second},
		{"last valid high", "60s", 60 * time.Second},
		{"invalid above clamps down", "61s", 60 * time.Second},
		{"last valid low", "1s", 1 * time.Second},
		{"zero clamps up, never disables the bound", "0s", 1 * time.Second},
		{"negative clamps up", "-5s", 1 * time.Second},
		{"unparseable falls back to the default", "banana", 10 * time.Second},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ze.firewall.vpp.reply-timeout", tt.set)
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

// recordingChannel is an api.Channel that records the reply timeout set on it.
// Only SetReplyTimeout carries behavior; the rest satisfy the interface.
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

// TestNewGovppOpsBindsReplyTimeout proves the deadline is applied to the
// channel Apply actually sends on, through the constructor Apply uses.
//
// VALIDATES: AC-10 for the vpp backend -- every binary-API reply is bounded.
//
// PREVENTS: computing a deadline and never installing it. govpp's channel keeps
// core.DefaultReplyTimeout (0, disabled) unless SetReplyTimeout is called, so
// an uninstalled deadline is indistinguishable from having none. Driving the
// constructor rather than a bare helper is what ties this to production: there
// is no other way to build the ops Apply passes down.
func TestNewGovppOpsBindsReplyTimeout(t *testing.T) {
	t.Setenv("ze.firewall.vpp.reply-timeout", "3s")
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
	if ch.timeout != 3*time.Second {
		t.Errorf("reply timeout = %s, want 3s", ch.timeout)
	}
}

// TestApplyTagsDataplaneTimeout pins the classification every owner reads.
//
// VALIDATES: a vpp reconcile that fails because VPP did not answer surfaces
// firewall.ErrKernelTimeout, so ze_firewall_apply_timeout_total counts it and
// ddos-local skips its rollback reconcile, exactly as under nft.
//
// PREVENTS: the honesty gap where the counter could only ever move under nft.
// Before this, a wedged VPP filled the latency histogram and left the timeout
// counter at 0, which an operator reads as "no timeouts" rather than "not
// instrumented".
func TestApplyTagsDataplaneTimeout(t *testing.T) {
	for _, tt := range []struct {
		name        string
		dumpErr     error
		wantTimeout bool
	}{
		{"govpp reply timeout is tagged", core.ErrReplyTimeout, true},
		{"wrapped reply timeout is tagged", fmt.Errorf("dump ACLs: %w", core.ErrReplyTimeout), true},
		// VPP absent is NOT wedged: the sentinel makes ddos-local skip its
		// rollback reconcile and ticks the wedged-dataplane counter, and
		// neither is right when the dataplane is simply not running.
		{"an absent dataplane is not a wedged one", context.DeadlineExceeded, false},
		{"a rejected ruleset is not a timeout", errors.New("VPP API returned retval -1"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := newOpsBackend()
			fake := newFakeOps(nil)
			fake.dumpErr = tt.dumpErr

			err := applyWithOpsLocked(b, fake, oneChainTable())
			if err == nil {
				t.Fatal("expected an error when dumpInterfaces fails")
			}
			if got := errors.Is(err, firewall.ErrKernelTimeout); got != tt.wantTimeout {
				t.Errorf("errors.Is(err, ErrKernelTimeout) = %v, want %v (err = %v)", got, tt.wantTimeout, err)
			}
			if !errors.Is(err, tt.dumpErr) {
				t.Errorf("the cause did not survive wrapping: %v", err)
			}
		})
	}
}

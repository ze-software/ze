// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_v4_rig_linux_test.go -- the rig: loop backend, scripted exchanger, fake clock

//go:build linux

package ifacedhcp

import (
	"log/slog"
	"net"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// silentRenewTimes and silentRebindTimes are AC-9's transmissions for a
// 1000 s lease no server answers after the DORA.
var (
	silentRenewTimes  = []time.Duration{seconds(500), seconds(687.5), seconds(781.25), seconds(841.25)}
	silentRebindTimes = []time.Duration{seconds(875), seconds(937.5), seconds(997.5)}
)

// firstAckThenFail answers the first DORA with ack and fails every later one.
func firstAckThenFail(ack *dhcpv4.DHCPv4) func(int) v4Reply {
	return func(n int) v4Reply {
		if n == 0 {
			return ackReply(ack)
		}
		return v4Reply{outcome: v4OutcomeFailed}
	}
}

func assertTimes(t *testing.T, what string, got, want []time.Duration) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s at %v, want %v", what, got, want)
	}
}

// TestDHCPv4LoopHoldsLeaseFiveLeasePeriods is the wiring test: the client is
// built and started as the config factory does, and the lease loop runs on the
// injected clock and exchanger.
//
// VALIDATES: AC-1 -- a 120 s lease the server renews every time is renewed at
// each T1 (60 s after each REQUEST) for five lease periods, with no REBIND, no
// second DORA, no address or route removal and no dhcp-expired, one
// dhcp-renewed per ACK and the address reinstalled with lifetime 120 s each
// time. R-8 -- the goroutine count stays flat across renewals. AC-23 -- a
// renewal granting an infinite lease reinstalls the address infinite and holds
// it until Stop.
// PREVENTS: defect 1, the lease dropped after one renewal.
func TestDHCPv4LoopHoldsLeaseFiveLeasePeriods(t *testing.T) {
	t.Run("renewed every T1", func(t *testing.T) {
		r := newV4Rig(t)
		r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 120*time.Second, loopRouter1))
		r.exch.extendAnswer = func(int, v4Attempt) v4Reply {
			return ackReply(testAck(t, loopServerA, 120*time.Second, loopRouter1))
		}
		r.startWorker()
		r.runUntil(seconds(130))
		goroutinesEarly := runtime.NumGoroutine()
		r.runUntil(seconds(590))
		goroutinesLate := runtime.NumGoroutine()

		want := make([]time.Duration, 0, 9)
		for i := 1; i <= 9; i++ {
			want = append(want, time.Duration(i)*60*time.Second)
		}
		assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), want)
		assertTimes(t, "REBIND", r.extendTimes(v4DeliveryBroadcast), nil)
		assertTimes(t, "DORA", r.acquireTimes(), []time.Duration{0})
		if got := r.countOps("replace eth7 192.0.2.50/24 120 120"); got != 10 {
			t.Errorf("address installed with lifetime 120 s %d times, want 10 (DORA + 9 renewals); ops %v", got, r.ops())
		}
		if got := r.countOps("remove-"); got != 0 {
			t.Errorf("%d removals while the lease was renewed; ops %v", got, r.ops())
		}
		if got := r.countEvents(evRenewed); got != 9 {
			t.Errorf("dhcp-renewed published %d times, want 9", got)
		}
		if got := r.countEvents(evExpired); got != 0 {
			t.Errorf("dhcp-expired published %d times, want 0", got)
		}
		if goroutinesLate > goroutinesEarly+2 {
			t.Errorf("goroutines grew from %d to %d across renewals", goroutinesEarly, goroutinesLate)
		}
	})

	t.Run("renewal to an infinite lease", func(t *testing.T) {
		r := newV4Rig(t)
		r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 120*time.Second, nil))
		r.exch.extendAnswer = func(int, v4Attempt) v4Reply {
			return ackReply(testAck(t, loopServerA, v4LeaseInfinite, nil))
		}
		r.startWorker()
		r.runUntil(seconds(10000))
		assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), []time.Duration{seconds(60)})
		if got := r.countOps("replace eth7 192.0.2.50/24 4294967295 4294967295"); got != 1 {
			t.Errorf("infinite reinstall seen %d times, want 1; ops %v", got, r.ops())
		}
		if got := r.countOps("remove-addr"); got != 0 {
			t.Fatalf("address removed before Stop; ops %v", r.ops())
		}
		r.stop()
		if got := r.countOps("remove-addr eth7 192.0.2.50/24"); got != 1 {
			t.Errorf("address removed %d times at Stop, want 1; ops %v", got, r.ops())
		}
	})
}

// TestDHCPv4ExchangesBindTheResolvedDevice drives a lease through DORA,
// RENEWING, REBINDING, expiry and a second DORA on a logical name the backend
// maps to another kernel device.
//
// VALIDATES: AC-2 -- every exchange is opened on the kernel device
// (iface.ResolveDevice), never on the logical name.
// PREVENTS: defect 2, renewals opened on the logical name.
func TestDHCPv4ExchangesBindTheResolvedDevice(t *testing.T) {
	r := newV4Rig(t)
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 1000*time.Second, loopRouter1))
	r.startWorker()
	r.runUntil(seconds(1005))

	calls := r.exch.snapshot()
	var renews, rebinds, acquires int
	for _, c := range calls {
		if c.device != loopKernelDevice {
			t.Errorf("exchange at %v opened on %q, want the kernel device %q", c.at, c.device, loopKernelDevice)
		}
		switch {
		case c.acquire:
			acquires++
		case c.attempt.delivery == v4DeliveryUnicast:
			renews++
		default:
			rebinds++
		}
	}
	if acquires < 2 || renews == 0 || rebinds == 0 {
		t.Errorf("acquires %d, renews %d, rebinds %d: want every kind exercised", acquires, renews, rebinds)
	}
}

// TestDHCPv4SilentServersExpireOnSchedule holds a 1000 s lease no server
// answers after the DORA.
//
// VALIDATES: AC-9 -- RENEW at 500, 687.5, 781.25 and 841.25 s, REBIND at 875,
// 937.5 and 997.5 s, nothing in between; teardown (address, then route) and
// dhcp-expired at 1000 s, and the next DORA at once. AC-10 -- the REBIND goes
// broadcast with ciaddr the leased address, no option 54, no option 50 and no
// reserved flags bit. AC-12 -- the RENEW is addressed to the server
// identifier from the leased address with the same message shape.
// PREVENTS: defects 6 (one exchange per phase) and 8 (RENEW broadcast).
func TestDHCPv4SilentServersExpireOnSchedule(t *testing.T) {
	r := newV4Rig(t)
	ack := testAck(t, loopServerA, 1000*time.Second, loopRouter1)
	ack.Flags = 0xffff
	r.exch.acquireAnswer = firstAckThenFail(ack)
	r.startWorker()
	r.runUntil(seconds(1005))

	assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), silentRenewTimes)
	assertTimes(t, "REBIND", r.extendTimes(v4DeliveryBroadcast), silentRebindTimes)
	assertTimes(t, "DORA", r.acquireTimes(), []time.Duration{0, seconds(1000)})
	assertTimes(t, "address removal", r.opTimes("remove-addr"), []time.Duration{seconds(1000)})

	want := []string{
		"replace eth7 192.0.2.50/24 1000 1000",
		"add-route eth7 0.0.0.0/0 192.0.2.254",
		"remove-addr eth7 192.0.2.50/24",
		"remove-route eth7 0.0.0.0/0 192.0.2.254",
	}
	if got := r.ops(); !slices.Equal(got, want) {
		t.Errorf("ops = %v, want %v", got, want)
	}
	if got := r.events(); !slices.Equal(got, []string{evAcquired, evExpired}) {
		t.Errorf("events = %v, want acquired then expired", got)
	}

	for _, c := range r.exch.snapshot() {
		if c.acquire {
			continue
		}
		req := c.attempt.request
		if !req.ClientIPAddr.Equal(loopLeased) {
			t.Errorf("%v: ciaddr %v, want the leased %v", c.at, req.ClientIPAddr, loopLeased)
		}
		if req.Options.Has(dhcpv4.OptionServerIdentifier) {
			t.Errorf("%v: extending REQUEST carries option 54", c.at)
		}
		if req.Options.Has(dhcpv4.OptionRequestedIPAddress) {
			t.Errorf("%v: extending REQUEST carries option 50", c.at)
		}
		if req.Flags&^flagsBroadcast != 0 {
			t.Errorf("%v: flags %#04x carry reserved bits", c.at, req.Flags)
		}
		if c.attempt.delivery == v4DeliveryUnicast {
			if !c.attempt.server.Equal(loopServerA) || !c.attempt.source.Equal(loopLeased) {
				t.Errorf("%v: RENEW %v -> %v, want %v -> %v", c.at, c.attempt.source, c.attempt.server, loopLeased, loopServerA)
			}
		}
	}
}

// TestDHCPv4NakRestartsConfiguration answers an extending REQUEST with a
// DHCPNAK, in each extending state.
//
// VALIDATES: AC-7, AC-8 -- teardown, dhcp-expired, a DORA at the same instant
// (no back-off), and one Info line naming the server and its message.
// PREVENTS: defect 4, a refused address kept until expiry.
func TestDHCPv4NakRestartsConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		lease    time.Duration
		nakOn    v4Delivery
		restarts time.Duration
	}{
		{name: "RENEWING", lease: 120 * time.Second, nakOn: v4DeliveryUnicast, restarts: seconds(60)},
		{name: "REBINDING", lease: 1000 * time.Second, nakOn: v4DeliveryBroadcast, restarts: seconds(875)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newV4Rig(t)
			r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, tc.lease, loopRouter1))
			r.exch.extendAnswer = func(_ int, a v4Attempt) v4Reply {
				if a.delivery == tc.nakOn {
					return nakReply(testNak(t, loopServerB, "address moved"))
				}
				return v4Reply{outcome: v4OutcomeNoAnswer}
			}
			r.startWorker()
			r.runUntil(tc.restarts + 5*time.Second)

			assertTimes(t, "DORA", r.acquireTimes(), []time.Duration{0, tc.restarts})
			assertTimes(t, "address removal", r.opTimes("remove-addr"), []time.Duration{tc.restarts})
			assertTimes(t, "route removal", r.opTimes("remove-route"), []time.Duration{tc.restarts})
			if got := r.events(); !slices.Equal(got, []string{evAcquired, evExpired}) {
				t.Errorf("events = %v, want acquired then expired", got)
			}
			if got := r.logs.count(slog.LevelInfo, "DHCPNAK"); got != 1 {
				t.Errorf("NAK logged at Info %d times, want 1", got)
			}
		})
	}
}

// TestDHCPv4RebindAcceptsAnyServer leases from server A, then only server B
// answers.
//
// VALIDATES: AC-13 -- RENEWING ignores B's ACK; AC-11 -- REBINDING accepts it,
// re-binds the lease to B (dhcp-renewed) and addresses the next RENEW to B,
// which B then answers.
// PREVENTS: defect 5, REBIND pinned to the original server.
func TestDHCPv4RebindAcceptsAnyServer(t *testing.T) {
	renewing := v4ExtendAccepts(false, loopServerA)
	if renewing(testAck(t, loopServerB, time.Hour, nil)) {
		t.Error("RENEWING matcher accepts another server's ACK")
	}
	if !renewing(testAck(t, loopServerA, time.Hour, nil)) {
		t.Error("RENEWING matcher refuses the leasing server's ACK")
	}

	r := newV4Rig(t)
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 1000*time.Second, nil))
	r.exch.extendAnswer = func(_ int, a v4Attempt) v4Reply {
		answer := testAck(t, loopServerB, 1000*time.Second, nil)
		if !a.accept(answer) {
			return v4Reply{outcome: v4OutcomeNoAnswer}
		}
		return ackReply(answer)
	}
	r.startWorker()
	r.runUntil(seconds(1380))

	assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), append(slices.Clone(silentRenewTimes), seconds(1375)))
	assertTimes(t, "REBIND", r.extendTimes(v4DeliveryBroadcast), []time.Duration{seconds(875)})
	calls := r.exch.snapshot()
	if last := calls[len(calls)-1]; !last.attempt.server.Equal(loopServerB) {
		t.Errorf("RENEW after the re-bind addressed to %v, want %v", last.attempt.server, loopServerB)
	}
	// B is the leasing server after the re-bind, so it answers the RENEW too.
	if got := r.events(); !slices.Equal(got, []string{evAcquired, evRenewed, evRenewed}) {
		t.Errorf("events = %v, want acquired, renewed by the REBIND, renewed by the RENEW to B", got)
	}
	if got := r.countOps("remove-addr"); got != 0 {
		t.Errorf("address removed during the re-bind; ops %v", r.ops())
	}
}

// TestDHCPv4InstallFailureIsNotBound makes the address install fail.
//
// VALIDATES: AC-21 -- no dhcp-acquired, no RENEW timer, the DORA retried after
// the 10 s back-off.
// PREVENTS: defect 12, a lease timed and renewed as held when it is not.
func TestDHCPv4InstallFailureIsNotBound(t *testing.T) {
	r := newV4Rig(t)
	r.backend.replaceErr = net.ErrClosed
	r.exch.acquireAnswer = func(int) v4Reply { return ackReply(testAck(t, loopServerA, 120*time.Second, nil)) }
	r.startWorker()
	r.runUntil(seconds(25))

	assertTimes(t, "DORA", r.acquireTimes(), []time.Duration{0, seconds(10), seconds(20)})
	assertTimes(t, "extending REQUEST", r.extendTimes(v4DeliveryUnicast), nil)
	if got := r.events(); len(got) != 0 {
		t.Errorf("events = %v, want none", got)
	}
}

// TestDHCPv4StopInEveryState stops the worker in each state.
//
// VALIDATES: AC-22 -- the worker returns; a held lease is torn down (address,
// then route); nothing is published at or after Stop.
// PREVENTS: a stopped client leaving its address installed or announcing an
// expiry nobody caused.
func TestDHCPv4StopInEveryState(t *testing.T) {
	cases := []struct {
		name  string
		lease time.Duration // 0: the DORA fails
		at    time.Duration
	}{
		{name: "INIT", at: seconds(5)},
		{name: "BOUND", lease: 1000 * time.Second, at: seconds(30)},
		{name: "RENEWING", lease: 1000 * time.Second, at: seconds(600)},
		{name: "REBINDING", lease: 1000 * time.Second, at: seconds(900)},
		{name: "BOUND infinite", lease: v4LeaseInfinite, at: seconds(10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newV4Rig(t)
			if tc.lease > 0 {
				r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, tc.lease, loopRouter1))
			}
			r.startWorker()
			r.runUntil(tc.at)
			before := r.events()
			r.stop()

			if got := r.events(); !slices.Equal(got, before) {
				t.Errorf("events after Stop = %v, before %v", got, before)
			}
			ops := r.ops()
			if tc.lease == 0 {
				if len(ops) != 0 {
					t.Errorf("ops = %v, want none", ops)
				}
				return
			}
			n := len(ops)
			if n < 2 || ops[n-2] != "remove-addr eth7 192.0.2.50/24" || ops[n-1] != "remove-route eth7 0.0.0.0/0 192.0.2.254" {
				t.Errorf("ops = %v, want address then route removed last", ops)
			}
		})
	}
}

// TestDHCPv4RefusedAckNotInstalled answers every DORA with an ACK that grants
// no lease time.
//
// VALIDATES: AC-15, AC-16 -- nothing installed, no dhcp-acquired, the DORA
// retried every 10 s; one Warn naming the problem and its fix, Debug for the
// repeats from the same server.
// PREVENTS: defect 7, an invented one-hour lease.
func TestDHCPv4RefusedAckNotInstalled(t *testing.T) {
	cases := []struct {
		name   string
		mods   []dhcpv4.Modifier
		reason *v4Refusal
	}{
		{name: "no option 51", reason: errV4LeaseTimeAbsent},
		{name: "option 51 is 0", mods: []dhcpv4.Modifier{
			dhcpv4.WithOption(dhcpv4.OptGeneric(dhcpv4.OptionIPAddressLeaseTime, []byte{0, 0, 0, 0})),
		}, reason: errV4LeaseTimeZero},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newV4Rig(t)
			r.exch.acquireAnswer = func(int) v4Reply { return ackReply(testAck(t, loopServerA, 0, nil, tc.mods...)) }
			r.startWorker()
			r.runUntil(seconds(35))

			assertTimes(t, "DORA", r.acquireTimes(), []time.Duration{0, seconds(10), seconds(20), seconds(30)})
			if got := r.countOps("replace"); got != 0 {
				t.Errorf("refused lease installed; ops %v", r.ops())
			}
			if got := r.events(); len(got) != 0 {
				t.Errorf("events = %v, want none", got)
			}
			if got := r.logs.count(slog.LevelWarn, "lease refused, nothing installed"); got != 1 {
				t.Errorf("refusal Warn logged %d times, want 1", got)
			}
			if got := r.logs.count(slog.LevelDebug, "lease refused again"); got != 3 {
				t.Errorf("repeat refusal Debug logged %d times, want 3", got)
			}
			if _, refusal := v4LeaseFromAck(testAck(t, loopServerA, 0, nil, tc.mods...), r.start, nil); refusal != tc.reason {
				t.Errorf("refusal = %v, want %v", refusal, tc.reason)
			}
		})
	}
}

// TestDHCPv4RenewalAckWithoutLeaseTimeIsNoAnswer answers every extending
// REQUEST with an ACK that carries no option 51.
//
// VALIDATES: AC-17 -- the held lease keeps its address and deadlines, the
// AC-9 schedule continues, and one Warn is logged for the lease.
func TestDHCPv4RenewalAckWithoutLeaseTimeIsNoAnswer(t *testing.T) {
	r := newV4Rig(t)
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 1000*time.Second, nil))
	r.exch.extendAnswer = func(int, v4Attempt) v4Reply { return ackReply(testAck(t, loopServerA, 0, nil)) }
	r.startWorker()
	r.runUntil(seconds(999))

	assertTimes(t, "RENEW", r.extendTimes(v4DeliveryUnicast), silentRenewTimes)
	assertTimes(t, "REBIND", r.extendTimes(v4DeliveryBroadcast), silentRebindTimes)
	if got := r.countOps("replace"); got != 1 {
		t.Errorf("address installed %d times, want once; ops %v", got, r.ops())
	}
	if got := r.logs.count(slog.LevelWarn, "renewal answer ignored"); got != 1 {
		t.Errorf("renewal refusal Warn logged %d times, want 1", got)
	}
}

// TestDHCPv4RouteInstallRetried fails the default-route install.
//
// VALIDATES: AC-3 -- with two failures, retries 5 s and 15 s after the first,
// the third call succeeds, one Warn and one Info, no further retry. AC-4 --
// with every call failing, retries settle at the 60 s cap through BOUND,
// RENEWING and REBINDING and stop at the teardown.
// PREVENTS: defect 3, a route failure logged once and never retried.
func TestDHCPv4RouteInstallRetried(t *testing.T) {
	t.Run("recovers", func(t *testing.T) {
		r := newV4Rig(t)
		r.backend.routeFailures = 2
		r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 120*time.Second, loopRouter1))
		r.exch.extendAnswer = func(int, v4Attempt) v4Reply {
			return ackReply(testAck(t, loopServerA, 120*time.Second, loopRouter1))
		}
		r.startWorker()
		r.runUntil(seconds(59))

		assertTimes(t, "failed route install", r.opTimes("add-route-failed"), []time.Duration{0, seconds(5)})
		assertTimes(t, "route install", r.opTimes("add-route"), []time.Duration{seconds(15)})
		if got := r.countEvents(evAcquired); got != 1 {
			t.Errorf("dhcp-acquired published %d times, want 1", got)
		}
		if got := r.logs.count(slog.LevelWarn, "default route install failed"); got != 1 {
			t.Errorf("route Warn logged %d times, want 1", got)
		}
		if got := r.logs.count(slog.LevelInfo, "default route installed after retry"); got != 1 {
			t.Errorf("route Info logged %d times, want 1", got)
		}
	})

	t.Run("keeps failing", func(t *testing.T) {
		r := newV4Rig(t)
		r.backend.routeFailures = 1 << 30
		r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 1000*time.Second, loopRouter1))
		r.startWorker()
		r.runUntil(seconds(1005))

		want := []time.Duration{0, seconds(5), seconds(15), seconds(35), seconds(75)}
		for at := seconds(135); at < seconds(1000); at += seconds(60) {
			want = append(want, at)
		}
		assertTimes(t, "failed route install", r.opTimes("add-route-failed"), want)
	})
}

// TestDHCPv4RouteFollowsRenewedRouter changes and then drops the router on
// renewal.
//
// VALIDATES: AC-5 -- the route via the old router is removed and the new one
// installed; AC-6 -- a renewal naming no router removes the old route.
// PREVENTS: defect 11, a stale default route left installed.
func TestDHCPv4RouteFollowsRenewedRouter(t *testing.T) {
	r := newV4Rig(t)
	r.exch.acquireAnswer = firstAckThenFail(testAck(t, loopServerA, 120*time.Second, loopRouter1))
	r.exch.extendAnswer = func(n int, _ v4Attempt) v4Reply {
		if n == 0 {
			return ackReply(testAck(t, loopServerA, 120*time.Second, loopRouter2))
		}
		return ackReply(testAck(t, loopServerA, 120*time.Second, nil))
	}
	r.startWorker()
	r.runUntil(seconds(150))

	var routes []string
	for _, op := range r.ops() {
		if len(op) > 5 && (op[:6] == "add-ro" || op[:6] == "remove") && op != "remove-addr eth7 192.0.2.50/24" {
			routes = append(routes, op)
		}
	}
	want := []string{
		"add-route eth7 0.0.0.0/0 192.0.2.254",
		"remove-route eth7 0.0.0.0/0 192.0.2.254",
		"add-route eth7 0.0.0.0/0 192.0.2.253",
		"remove-route eth7 0.0.0.0/0 192.0.2.253",
	}
	if !slices.Equal(routes, want) {
		t.Errorf("route ops = %v, want %v", routes, want)
	}
}

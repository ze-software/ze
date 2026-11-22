// Design: docs/research/l2tpv2-ze-integration.md -- session IPv6 carrier admission
// Related: session.go -- session.OpenIPv6Carrier, beginIPv6CarrierOpen

package kernel

import (
	"context"
	"errors"
	"io"
	"math"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// sessionTestCarrier models only the carrier's Close. The session never reads
// or writes through a carrier it opens, so the I/O methods refuse.
type sessionTestCarrier struct {
	mu       sync.Mutex
	closes   int
	closeErr error
}

var errSessionTestCarrierIO = errors.New("session test carrier carries no traffic")

func (c *sessionTestCarrier) ReadRS(context.Context) error { return errSessionTestCarrierIO }
func (c *sessionTestCarrier) WriteRA([]byte, int) error    { return errSessionTestCarrierIO }
func (c *sessionTestCarrier) ReadDHCPv6(context.Context, []byte) (int, netip.AddrPort, error) {
	return 0, netip.AddrPort{}, errSessionTestCarrierIO
}

func (c *sessionTestCarrier) WriteDHCPv6([]byte, netip.AddrPort) (int, error) {
	return 0, errSessionTestCarrierIO
}

func (c *sessionTestCarrier) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	return c.closeErr
}

func (c *sessionTestCarrier) closeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closes
}

var _ ppp.IPv6Carrier = (*sessionTestCarrier)(nil)

// carrierSessionOpener records what the session passed to the injected opener.
type carrierSessionOpener struct {
	calls int
	unit  *CreatedUnit
	index int
}

// ipv6CarrierTestSession holds a genuine original-unit lease over a regular
// file. The file grants no PPP authority: only the injected opener reaches it.
func ipv6CarrierTestSession(t *testing.T, index uint32) (*session, *CreatedUnit) {
	t.Helper()
	unit := retirementTestUnit(t)
	owner := &session{identity: ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: index},
		unitLease: &claimedCreatedUnit{unit: unit}}
	return owner, unit
}

// VALIDATES: a live session hands the opener its original unit and captured
// index, and returns the opened carrier without closing it.
// PREVENTS: the session closing or replacing a carrier its caller now owns.
func TestSessionIPv6CarrierLiveOpenReturnsCarrier(t *testing.T) {
	owner, unit := ipv6CarrierTestSession(t, 27)
	carrier := &sessionTestCarrier{}
	var opener carrierSessionOpener
	owner.ops.openIPv6Carrier = func(_ context.Context, got *CreatedUnit, index int) (ppp.IPv6Carrier, error) {
		opener.calls, opener.unit, opener.index = opener.calls+1, got, index
		return carrier, nil
	}
	opened, err := owner.OpenIPv6Carrier(t.Context())
	if err != nil {
		t.Fatalf("live open: %v", err)
	}
	if opened != carrier {
		t.Fatalf("live open returned %v, want the opened carrier", opened)
	}
	if opener.calls != 1 {
		t.Fatalf("opener calls = %d, want 1", opener.calls)
	}
	if opener.unit != unit {
		t.Fatal("opener did not receive the session's original unit")
	}
	if opener.index != 27 {
		t.Fatalf("opener index = %d, want 27", opener.index)
	}
	if got := carrier.closeCount(); got != 0 {
		t.Fatalf("live carrier closed %d times, want 0", got)
	}
}

// VALIDATES: a carrier whose session departed during the open is closed
// exactly once and refused with io.ErrClosedPipe, whether the session closed
// or its original unit changed.
// PREVENTS: a carrier bound to a departed session leaking to the caller.
func TestSessionIPv6CarrierDepartedDuringOpenClosesCarrier(t *testing.T) {
	for _, tc := range []struct {
		name   string
		depart func(t *testing.T, owner *session)
	}{
		{name: "session closed", depart: func(_ *testing.T, owner *session) { owner.closed = true }},
		{name: "unit changed", depart: func(t *testing.T, owner *session) {
			owner.unitLease = &claimedCreatedUnit{unit: retirementTestUnit(t)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, _ := ipv6CarrierTestSession(t, 27)
			carrier := &sessionTestCarrier{}
			owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
				owner.mu.Lock()
				tc.depart(t, owner)
				owner.mu.Unlock()
				return carrier, nil
			}
			opened, err := owner.OpenIPv6Carrier(t.Context())
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("departed open error = %v, want io.ErrClosedPipe", err)
			}
			if opened != nil {
				t.Fatalf("departed open returned carrier %v after its successful close", opened)
			}
			if got := carrier.closeCount(); got != 1 {
				t.Fatalf("departed carrier closed %d times, want exactly 1", got)
			}
		})
	}
}

// VALIDATES: a departed session's carrier whose Close fails is returned with
// io.ErrClosedPipe and the close failure, per OpenIPv6Carrier's contract.
// PREVENTS: losing the only cleanup owner of a carrier that did not close.
func TestSessionIPv6CarrierDepartedCloseFailureReturnsCarrier(t *testing.T) {
	owner, _ := ipv6CarrierTestSession(t, 27)
	closeErr := errors.New("carrier cleanup failed")
	carrier := &sessionTestCarrier{closeErr: closeErr}
	owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
		owner.mu.Lock()
		owner.closed = true
		owner.mu.Unlock()
		return carrier, nil
	}
	opened, err := owner.OpenIPv6Carrier(t.Context())
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v, want io.ErrClosedPipe", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want the carrier close failure", err)
	}
	if opened != carrier {
		t.Fatalf("returned %v, want the carrier whose cleanup failed", opened)
	}
	if got := carrier.closeCount(); got != 1 {
		t.Fatalf("carrier closed %d times, want exactly 1", got)
	}
}

// VALIDATES: an open that failed while the session departed answers both
// io.ErrClosedPipe and the open's cause, with no carrier to close.
// PREVENTS: a departed session hiding why the open itself failed.
func TestSessionIPv6CarrierDepartedOpenFailureKeepsCause(t *testing.T) {
	owner, _ := ipv6CarrierTestSession(t, 27)
	openErr := errors.New("control socket refused")
	owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
		owner.mu.Lock()
		owner.closed = true
		owner.mu.Unlock()
		return nil, openErr
	}
	opened, err := owner.OpenIPv6Carrier(t.Context())
	if opened != nil {
		t.Fatalf("returned carrier %v from a failed open", opened)
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v, want io.ErrClosedPipe", err)
	}
	if !errors.Is(err, openErr) {
		t.Fatalf("error = %v, want the open failure", err)
	}
}

// VALIDATES: each admission guard refuses before the opener runs: a closed
// session, no genuine original unit, index zero, an index past int32, and a
// session without an opener. MaxInt32 itself is admitted.
// PREVENTS: an open reaching the kernel with an unusable unit or index.
func TestSessionIPv6CarrierAdmissionGuards(t *testing.T) {
	for _, tc := range []struct {
		name      string
		index     uint32
		prepare   func(owner *session)
		closed    bool
		wantError string
	}{
		{name: "closed session", index: 27, prepare: func(owner *session) { owner.closed = true }, closed: true},
		{name: "no original unit", index: 27, prepare: func(owner *session) { owner.unitLease = nil }, wantError: "no original unit"},
		{name: "index zero", index: 0, wantError: "no captured native index"},
		{name: "index past int32", index: math.MaxInt32 + 1, wantError: "exceeds native width"},
		{name: "no opener", index: 27, prepare: func(owner *session) { owner.ops.openIPv6Carrier = nil }, wantError: "no IPv6 carrier opener"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, _ := ipv6CarrierTestSession(t, tc.index)
			calls := 0
			owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
				calls++
				return &sessionTestCarrier{}, nil
			}
			if tc.prepare != nil {
				tc.prepare(owner)
			}
			opened, err := owner.OpenIPv6Carrier(t.Context())
			if opened != nil {
				t.Fatalf("refused open returned carrier %v", opened)
			}
			if calls != 0 {
				t.Fatalf("opener ran %d times behind a refusing guard", calls)
			}
			if tc.closed {
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("error = %v, want io.ErrClosedPipe", err)
				}
				return
			}
			if err == nil {
				t.Fatal("guard admitted the open")
			}
			if errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("guard answered io.ErrClosedPipe for a live session: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want the %q guard", err, tc.wantError)
			}
			if owner.ipv6CarrierOpening {
				t.Fatal("a refused open left its admission token held")
			}
		})
	}
	owner, _ := ipv6CarrierTestSession(t, math.MaxInt32)
	owner.ops.openIPv6Carrier = func(_ context.Context, _ *CreatedUnit, index int) (ppp.IPv6Carrier, error) {
		if index != math.MaxInt32 {
			t.Errorf("opener index = %d, want MaxInt32", index)
		}
		return &sessionTestCarrier{}, nil
	}
	if _, err := owner.OpenIPv6Carrier(t.Context()); err != nil {
		t.Fatalf("MaxInt32 index refused: %v", err)
	}
}

// VALIDATES: a second open while one is in progress is refused without
// running the opener, and once the first open returned, a reopen after its
// carrier's Close is admitted, as is a retry after a failed open.
// PREVENTS: two carriers binding the DHCPv6 server port of one unit at once.
func TestSessionIPv6CarrierRefusesConcurrentOpen(t *testing.T) {
	owner, _ := ipv6CarrierTestSession(t, 27)
	entered, release := make(chan struct{}), make(chan struct{})
	first := &sessionTestCarrier{}
	var calls int
	var callsMu sync.Mutex
	owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
		callsMu.Lock()
		calls++
		callsMu.Unlock()
		close(entered)
		<-release
		return first, nil
	}
	type result struct {
		carrier ppp.IPv6Carrier
		err     error
	}
	done := make(chan result, 1)
	go func() {
		carrier, err := owner.OpenIPv6Carrier(context.Background())
		done <- result{carrier: carrier, err: err}
	}()
	<-entered
	second, err := owner.OpenIPv6Carrier(t.Context())
	if err == nil {
		close(release)
		<-done
		t.Fatal("a second open was admitted while the first was in progress")
	}
	if errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("concurrent refusal answered io.ErrClosedPipe for a live session: %v", err)
	}
	if second != nil {
		t.Errorf("refused open returned carrier %v", second)
	}
	close(release)
	got := <-done
	if got.err != nil {
		t.Fatalf("first open: %v", got.err)
	}
	if got.carrier != first {
		t.Fatalf("first open returned %v, want its carrier", got.carrier)
	}
	callsMu.Lock()
	if calls != 1 {
		t.Fatalf("opener ran %d times, want 1 while the second open was refused", calls)
	}
	callsMu.Unlock()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	openErr := errors.New("control socket refused")
	owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
		return nil, openErr
	}
	if _, err := owner.OpenIPv6Carrier(t.Context()); !errors.Is(err, openErr) {
		t.Fatalf("failed open error = %v, want its cause", err)
	}
	reopened := &sessionTestCarrier{}
	owner.ops.openIPv6Carrier = func(context.Context, *CreatedUnit, int) (ppp.IPv6Carrier, error) {
		return reopened, nil
	}
	carrier, err := owner.OpenIPv6Carrier(t.Context())
	if err != nil {
		t.Fatalf("reopen after Close and a failed retry: %v", err)
	}
	if carrier != reopened {
		t.Fatalf("reopen returned %v, want its carrier", carrier)
	}
}

// VALIDATES: the production session dependencies bind the carrier opener.
// PREVENTS: every production open refusing for a missing opener.
func TestProductionSessionOpsBindIPv6CarrierOpener(t *testing.T) {
	if productionSessionOps().openIPv6Carrier == nil {
		t.Fatal("production session ops do not bind openIPv6Carrier")
	}
}

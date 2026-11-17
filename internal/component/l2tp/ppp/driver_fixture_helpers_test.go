// Design: docs/architecture/l2tp/bng-5-pppoe.md -- protocol fixtures with per-session resources

package ppp

import (
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"
)

// driverConfig belongs only to the existing protocol fixtures. The live Driver
// receives none of these kernel dependencies; testStartForDriver places them on
// the same session payload the test queues through the actual admission path.
type driverConfig struct {
	Logger  *slog.Logger
	Backend testIfaceBackend
	Ops     pppOps
}

// Each protocol fixture creates a finite number of drivers. This test-process
// registry keeps their immutable recorder bindings out of the shipped Driver.
var driverFixtureBindings sync.Map // *Driver -> driverConfig

func newDriver(cfg driverConfig) *Driver {
	driver := NewProductionDriver(cfg.Logger)
	driverFixtureBindings.Store(driver, cfg)
	return driver
}

func testStartForDriver(driver *Driver, input *StartSession, channel, unit, number int) StartSession {
	start := *input
	value, found := driverFixtureBindings.Load(driver)
	if !found {
		panic("BUG: protocol fixture has no session dependency binding")
	}
	cfg, ok := value.(driverConfig)
	if !ok {
		panic("BUG: protocol fixture binding has an unexpected type")
	}
	return testSessionStart(&start, cfg.Backend, cfg.Ops, channel, unit, number)
}

func testStartPointerForDriver(driver *Driver, input *StartSession, channel, unit, number int) *StartSession {
	start := *input
	bound := testStartForDriver(driver, &start, channel, unit, number)
	return &bound
}

// testProtocolSession changes only ownership representation in direct FSM
// fixtures. The recorder and fake ioctl operations remain those supplied by the
// original fixture, including its intentional failure and ordering callbacks.
func testProtocolSession(session *pppSession, backend testIfaceBackend, ops pppOps,
	channel, unit, number int) *pppSession {
	owner := newTestSessionHandle(backend, "ppp"+strconv.Itoa(number))
	owner.ops, owner.chanFD, owner.unitFD, owner.unitNum = ops, channel, unit, number
	if closer, owns := session.chanFile.(io.Closer); owns {
		owner.owned = append(owner.owned, closer)
	}
	if closer, owns := session.unitFile.(io.Closer); owns {
		if unit != channel {
			owner.owned = append(owner.owned, closer)
		}
	}
	session.dataplane = owner
	session.frames = &testSessionFrames{control: session.chanFile, opened: session.unitFile}
	return session
}

func testHandleFor(session *pppSession) *testSessionHandle {
	owner, present := session.dataplane.(*testSessionHandle)
	if !present {
		panic("BUG: direct protocol fixture has no test session handle")
	}
	return owner
}

type pppOps struct {
	// setMRU sets the maximum receive unit on a /dev/ppp unit fd
	// via PPPIOCSMRU. Called once per session after LCP-Opened.
	setMRU func(unitFD int, mru uint16) error

	// connect links the PPP channel to its unit (PPPIOCCONNECT).
	// Deferred until after NCP: before connect, received frames go
	// to the channel fd; after, they go to the unit fd.
	connect func(chanFD, unitNum int) error

	// disconnect restores channel-fd delivery (PPPIOCDISCONN).
	// An attached channel MUST be disconnected before another NCP lifetime.
	disconnect func(chanFD int) error
}

// readInterrupter is a channel file whose blocked Read can be woken.
// Kernel session ownership supplies this capability for /dev/ppp.
type readInterrupter interface {
	InterruptRead() error
}

// readDeadliner is a channel file with read deadlines, as net.Conn has; a
// deadline in the past wakes a blocked Read.
type readDeadliner interface {
	SetReadDeadline(t time.Time) error
}

// interruptRead wakes a Read blocked on f. A file that offers neither form
// cannot be quiesced, and the caller refuses the handoff.
func interruptRead(f io.Reader) error {
	if v, ok := f.(readInterrupter); ok {
		return v.InterruptRead()
	}
	if v, ok := f.(readDeadliner); ok {
		return v.SetReadDeadline(time.Now())
	}
	return errChannelNotQuiescable
}

// testControlFromEvent retains the exact worker capability the assertion path
// already observed. It never resolves a numeric label or a current ID.
func testControlFromEvent(t *testing.T, event Event) SessionControl {
	t.Helper()
	if opened, ok := event.(EventLCPUp); ok {
		return opened.Control
	}
	if up, ok := event.(EventSessionUp); ok {
		return up.Control
	}
	t.Fatalf("event %T does not publish an admitted lifecycle control", event)
	return SessionControl{}
}

func testRebindSessionAttempt(t *testing.T, start *StartSession) {
	t.Helper()
	attempt, err := NewSessionAttempt(start.SubscriberID, start.Dataplane)
	if err != nil {
		t.Fatalf("fixture admission does not bind its actual owner: %v", err)
	}
	start.Attempt = attempt
}

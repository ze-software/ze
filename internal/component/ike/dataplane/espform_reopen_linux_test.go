//go:build linux

package dataplane

import (
	"bytes"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// errESPFormSocketFailed is the read error a scripted socket answers when the test fails
// it: a socket that failed on its own, not one the receiver closed.
var errESPFormSocketFailed = errors.New("test: the raw ESP socket failed")

// espFormReopenWait bounds every wait of these tests for the receiver's own worker. A
// test that drives the worker sets reopenMin to zero or a millisecond, so every attempt
// comes well inside it, and the bound only ends a test whose worker never acts.
const espFormReopenWait = 10 * time.Second

// espFormScriptedSocket is a reader socket a test drives through the receiver's openPaths
// seam: each ESP payload sent on reads is delivered as fakeESPFormConn delivers it, from
// the peer 198.51.100.7 to 203.0.113.9; closing fail ends the read with
// errESPFormSocketFailed; Close ends it with net.ErrClosed, as a closed socket does.
type espFormScriptedSocket struct {
	reads  chan []byte
	fail   chan struct{}
	closed chan struct{}
	once   sync.Once
}

func (s *espFormScriptedSocket) ReadMsgIP(p, oob []byte) (n, oobn, flags int, addr *net.IPAddr, err error) {
	select {
	case esp := <-s.reads:
		conn := fakeESPFormConn{from: &net.IPAddr{IP: net.IPv4(198, 51, 100, 7)}, reads: [][]byte{esp}}
		return conn.ReadMsgIP(p, oob)
	case <-s.fail:
		return 0, 0, 0, nil, errESPFormSocketFailed
	case <-s.closed:
		return 0, 0, 0, nil, net.ErrClosed
	}
}

func (s *espFormScriptedSocket) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

// espFormSentDevice stands in for ze-espform and hands each datagram the reader wrote to
// the test, copied, because the reader reuses its buffer.
type espFormSentDevice struct {
	sent chan []byte
}

func (d *espFormSentDevice) inject(pkt []byte) error {
	d.sent <- bytes.Clone(pkt)
	return nil
}

func (d *espFormSentDevice) Close() error { return nil }

// newIdleESPFormPaths answers paths whose socket delivers nothing until the test scripts
// it, so a reader on them runs until the receiver closes them.
func newIdleESPFormPaths() espFormPaths {
	return espFormPaths{
		conn: &espFormScriptedSocket{reads: make(chan []byte), fail: make(chan struct{}), closed: make(chan struct{})},
		tun:  &espFormSentDevice{sent: make(chan []byte, 16)},
	}
}

// espFormTestReceiver answers a receiver whose opens openPaths answers, closed when the
// test ends.
func espFormTestReceiver(t *testing.T, open func() (espFormPaths, error)) *espFormReceiver {
	t.Helper()
	r := newESPFormReceiver(slogutil.DiscardLogger())
	r.openPaths = open
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close the receiver: %v", err)
		}
	})
	return r
}

// failESPFormReader fails the socket of paths and waits until the reader on it stopped.
func failESPFormReader(t *testing.T, r *espFormReceiver, paths espFormPaths) {
	t.Helper()
	r.mu.Lock()
	life := r.life
	r.mu.Unlock()
	if life == nil {
		t.Fatal("no reader runs to fail")
	}
	socket, ok := paths.conn.(*espFormScriptedSocket)
	if !ok {
		t.Fatal("the paths carry no scripted socket")
	}
	close(socket.fail)
	select {
	case <-life.dead:
	case <-time.After(espFormReopenWait):
		t.Fatal("the reader did not stop after its socket failed")
	}
}

// requireESPFormServed sends one bare ESP datagram for spi on the socket of paths and
// fails the test unless the reader writes it, addressed to local, into their device.
func requireESPFormServed(t *testing.T, paths espFormPaths, spi uint32, local netip.Addr) {
	t.Helper()
	socket, ok := paths.conn.(*espFormScriptedSocket)
	if !ok {
		t.Fatal("the paths carry no scripted socket")
	}
	device, ok := paths.tun.(*espFormSentDevice)
	if !ok {
		t.Fatal("the paths carry no recording device")
	}
	select {
	case socket.reads <- bareESP(spi, 1, 0xAB):
	case <-time.After(espFormReopenWait):
		t.Fatal("no reader reads the reopened socket")
	}
	select {
	case pkt := <-device.sent:
		if len(pkt) < espFormHeaderLen {
			t.Fatalf("the reader wrote %d octets, shorter than the ESP-in-UDP header", len(pkt))
		}
		if dst := netip.AddrFrom4([4]byte(pkt[16:20])); dst != local {
			t.Errorf("the re-presented datagram is addressed to %s, want the SA's local address %s", dst, local)
		}
	case <-time.After(espFormReopenWait):
		t.Fatal("the bare ESP of the watched SA was not re-presented")
	}
}

// TestESPFormReceiverWatchReopensAStoppedReader proves a Watch that meets a reader whose
// socket failed opens the paths again and serves on them, rather than answer nil on a
// reader nothing runs. RFC 7296 Section 2.23: the SA's bare ESP is owed at any time.
//
// METHOD: the openPaths seam answers two scripted openings; the revive worker waits an
// hour, so the Watch meets the stopped reader first.
// MUTATION: in Watch, open the paths only when r.conn is nil rather than when the reader
// is not serving (servingLocked): the Watch answers nil on the stopped reader and makes
// no second open.
// MUTATION: in reopenReaderLocked, drop the closeReaderLocked call: the failed socket is
// never closed.
func TestESPFormReceiverWatchReopensAStoppedReader(t *testing.T) {
	const spi1, spi2 = 0x5E0A0001, 0x5E0A0002
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	first, second := newIdleESPFormPaths(), newIdleESPFormPaths()
	opens := 0
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		if opens == 1 {
			return first, nil
		}
		if opens == 2 {
			return second, nil
		}
		return espFormPaths{}, errors.New("test: an open beyond the two scripted")
	})
	r.reopenWait = time.Hour

	if err := r.Watch(spi1, natTarget(peer, local)); err != nil {
		t.Fatalf("watch the first SA: %v", err)
	}
	failESPFormReader(t, r, first)

	if err := r.Watch(spi2, natTarget(peer, local)); err != nil {
		t.Fatalf("watch after the reader stopped: %v", err)
	}
	if opens != 2 {
		t.Fatalf("the paths were opened %d times, want twice: the Watch after the reader stopped opened nothing", opens)
	}
	firstSocket, ok := first.conn.(*espFormScriptedSocket)
	if !ok {
		t.Fatal("the first paths carry no scripted socket")
	}
	select {
	case <-firstSocket.closed:
	default:
		t.Error("the failed socket is still open after the paths were opened again")
	}
	requireESPFormServed(t, second, spi1, local)
}

// TestESPFormReceiverWatchRefusesWithTheRecordedOpenFailure proves a Watch that meets a
// stopped reader and cannot open the paths again refuses with the open's operator error,
// records it for the doctor, and leaves the SA already installed watched.
//
// METHOD: the openPaths seam answers one scripted opening and then the raw socket's
// EPERM refusal; the revive worker waits an hour.
// MUTATION: in Watch, open the paths only when r.conn is nil: the Watch answers nil.
// MUTATION: in reopenReaderLocked, do not set r.failure on a failed open: the state the
// doctor reads carries no failure.
func TestESPFormReceiverWatchRefusesWithTheRecordedOpenFailure(t *testing.T) {
	const spi1, spi2 = 0x5E0B0001, 0x5E0B0002
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	refusal := espFormSocketRefusal(unix.EPERM)
	first := newIdleESPFormPaths()
	opens := 0
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		if opens == 1 {
			return first, nil
		}
		return espFormPaths{}, refusal
	})
	r.reopenWait = time.Hour

	if err := r.Watch(spi1, natTarget(peer, local)); err != nil {
		t.Fatalf("watch the first SA: %v", err)
	}
	failESPFormReader(t, r, first)

	err := r.Watch(spi2, natTarget(peer, local))
	if !errors.Is(err, refusal) {
		t.Fatalf("a Watch whose paths cannot open again answered %v, want the open's refusal", err)
	}
	if oe, ok := errors.AsType[*opererr.Error](err); !ok || oe.Fix() != privilege.DaemonCapabilitiesFix {
		t.Errorf("the refusal %v does not carry the raw socket's fix %q", err, privilege.DaemonCapabilitiesFix)
	}
	st, stErr := r.fastPathState()
	if stErr != nil {
		t.Fatalf("read the state the doctor reads: %v", stErr)
	}
	failure := st.CompletePathFailure
	if failure == nil {
		t.Fatal("the state the doctor reads carries no failure to open")
	}
	if !errors.Is(failure.Err, refusal) {
		t.Errorf("the recorded failure is %v, want the open's refusal", failure.Err)
	}
	if failure.At.IsZero() {
		t.Error("the recorded failure carries no time")
	}
	if _, ok := r.target(spi2, local); ok {
		t.Error("the refused SA is watched")
	}
	if _, ok := r.target(spi1, local); !ok {
		t.Error("the SA installed before the reader stopped is no longer watched")
	}
	if r.running() {
		t.Error("the stopped reader's socket is still open after the refused Watch")
	}
}

// TestESPFormReceiverReopensAStoppedReaderByItself proves the receiver opens its paths
// again by itself after the reader's socket failed, with no new Watch, so the SAs already
// in the SAD are served again; a failed attempt is recorded, and the attempt that opens
// clears the record. RFC 7296 Section 2.23: "at any time".
//
// METHOD: the openPaths seam answers a scripted opening, then the raw socket's EPERM
// refusal, then a second opening; reopenMin is zero, so the revive worker makes each
// attempt at once, and the test waits on the openings and reads the record, never the
// clock.
// MUTATION: in revive, return after the reader's death is logged instead of reopening:
// no second open comes, and the SA is never served again.
// MUTATION: in reopenReaderLocked, keep r.failure on an open that succeeds: the refused
// attempt stays recorded after the paths opened.
func TestESPFormReceiverReopensAStoppedReaderByItself(t *testing.T) {
	const spi = 0x5E0C0001
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	refusal := espFormSocketRefusal(unix.EPERM)
	first, third := newIdleESPFormPaths(), newIdleESPFormPaths()
	opened := make(chan int, 4)
	failureBeforeSuccess := make(chan *ESPFormCompletePathFailure, 1)
	opens := 0 // written only by openPaths, which runs under r.mu
	var r *espFormReceiver
	r = espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		opened <- opens
		if opens == 1 {
			return first, nil
		}
		if opens == 2 {
			return espFormPaths{}, refusal
		}
		if opens == 3 {
			// This seam holds r.mu, so the preceding refusal is observed before the
			// successful open clears it, whatever schedule the test goroutine gets.
			failureBeforeSuccess <- r.failure
			return third, nil
		}
		return espFormPaths{}, errors.New("test: an open beyond the three scripted")
	})
	r.reopenMin = 0
	waitOpen := func(want int) {
		t.Helper()
		select {
		case got := <-opened:
			if got != want {
				t.Fatalf("open %d came, want open %d", got, want)
			}
		case <-time.After(espFormReopenWait):
			t.Fatalf("open %d never came: the receiver did not open its paths again by itself", want)
		}
		// openPaths runs under r.mu, so taking it waits for the attempt to finish.
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	if err := r.Watch(spi, natTarget(peer, local)); err != nil {
		t.Fatalf("watch: %v", err)
	}
	waitOpen(1)
	failESPFormReader(t, r, first)

	waitOpen(2)
	var failure *ESPFormCompletePathFailure
	select {
	case failure = <-failureBeforeSuccess:
	case <-time.After(espFormReopenWait):
		t.Fatal("the receiver never tried the successful open after its refused attempt")
	}
	if failure == nil || !errors.Is(failure.Err, refusal) {
		t.Fatalf("after the refused attempt the recorded failure is %v, want the refusal", failure)
	}

	waitOpen(3)
	r.mu.Lock()
	failure = r.failure
	r.mu.Unlock()
	if failure != nil {
		t.Errorf("after the paths opened again the failure %v is still recorded", failure.Err)
	}
	requireESPFormServed(t, third, spi, local)
}

// TestESPFormReceiverHandsEachReaderStopOnOnce proves the stop of a reader whose socket
// failed is handed on once, with the read error and a time, before the receiver opens its
// paths again, so the engine raises it on the report bus even when the first reopen
// succeeds and no doctor record remains.
//
// METHOD: the readerStopped seam records each stop; openPaths answers two scripted
// openings; reopenMin is zero, so the test waits on channels, never on the clock.
// MUTATION: in revive, drop the reportReaderStopped call: no stop is handed on.
func TestESPFormReceiverHandsEachReaderStopOnOnce(t *testing.T) {
	const spi = 0x5E0E0001
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	first, second := newIdleESPFormPaths(), newIdleESPFormPaths()
	reopened := make(chan struct{})
	opens := 0 // written only by openPaths, which runs under r.mu
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		if opens == 1 {
			return first, nil
		}
		if opens == 2 {
			close(reopened)
			return second, nil
		}
		return espFormPaths{}, errors.New("test: an open beyond the two scripted")
	})
	r.reopenMin = 0
	stops := make(chan ESPFormReaderStopped, 4)
	r.readerStopped = func(stopped ESPFormReaderStopped) { stops <- stopped }

	if err := r.Watch(spi, natTarget(peer, local)); err != nil {
		t.Fatalf("watch: %v", err)
	}
	failESPFormReader(t, r, first)
	select {
	case <-reopened:
	case <-time.After(espFormReopenWait):
		t.Fatal("the receiver did not open its paths again after the reader stopped")
	}
	// The revive worker hands the stop on before it reopens, so it is here by now.
	select {
	case stopped := <-stops:
		if !errors.Is(stopped.Err, errESPFormSocketFailed) {
			t.Errorf("the stop carries %v, want the socket's read error", stopped.Err)
		}
		if stopped.At.IsZero() {
			t.Error("the stop carries no time")
		}
	default:
		t.Fatal("the reader's stop was not handed on")
	}
	select {
	case stopped := <-stops:
		t.Errorf("one stop was handed on twice, the second with %v", stopped.Err)
	default:
	}
	requireESPFormServed(t, second, spi, local)
}

// TestESPFormReceiverRefusesAfterClose proves a Watch or an acquire after Close refuses
// with errESPFormReceiverClosed and opens nothing: no reader, no hold, no SA watched.
//
// MUTATION: in Watch, drop the closed test: the Watch after Close opens the paths.
// MUTATION: in acquire, drop the closed test: the acquire after Close opens the paths and
// holds them.
func TestESPFormReceiverRefusesAfterClose(t *testing.T) {
	const spi = 0x5E0F0001
	target := natTarget(netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9"))
	opens := 0
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		return newIdleESPFormPaths(), nil
	})
	if err := r.Close(); err != nil {
		t.Fatalf("close the receiver: %v", err)
	}

	if err := r.Watch(spi, target); !errors.Is(err, errESPFormReceiverClosed) {
		t.Errorf("a Watch after Close answered %v, want errESPFormReceiverClosed", err)
	}
	if err := r.acquire(); !errors.Is(err, errESPFormReceiverClosed) {
		t.Errorf("an acquire after Close answered %v, want errESPFormReceiverClosed", err)
	}
	if opens != 0 {
		t.Errorf("the paths were opened %d times after Close, want none", opens)
	}
	if r.running() {
		t.Error("the paths are open after Close")
	}
	if _, ok := r.target(spi, target.local); ok {
		t.Error("the SA refused after Close is watched")
	}
	if r.holds != 0 {
		t.Errorf("the receiver holds its paths %d times after Close, want none", r.holds)
	}
}

// TestESPFormReceiverBacksOffAReaderThatDiesAtOnce proves the reopen backoff lives on the
// receiver, not on one reader's life: a reader that dies right after each open is reopened
// after a wait doubled each time, so a socket that fails at once is not reopened in a
// tight loop.
//
// METHOD: reopenMin is a millisecond; openPaths answers a new scripted opening each time
// and hands it to the test, which fails each reader at once and reads the receiver's next
// wait under its lock after each reopen, never the clock.
// MUTATION: in reopenReaderLocked, set r.reopenWait to zero on every open that succeeds:
// every stop is reopened at once, and the wait never grows.
func TestESPFormReceiverBacksOffAReaderThatDiesAtOnce(t *testing.T) {
	const spi = 0x5E100001
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	opened := make(chan espFormPaths, 8)
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		paths := newIdleESPFormPaths()
		opened <- paths
		return paths, nil
	})
	r.reopenMin = time.Millisecond
	next := func() espFormPaths {
		t.Helper()
		select {
		case paths := <-opened:
			return paths
		case <-time.After(espFormReopenWait):
			t.Fatal("the receiver did not open its paths again after the reader stopped")
		}
		return espFormPaths{}
	}

	if err := r.Watch(spi, natTarget(peer, local)); err != nil {
		t.Fatalf("watch: %v", err)
	}
	paths := next()
	for _, want := range []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond} {
		failESPFormReader(t, r, paths)
		paths = next()
		// openPaths runs under r.mu, so taking it waits for the attempt to finish.
		r.mu.Lock()
		got := r.reopenWait
		r.mu.Unlock()
		if got != want {
			t.Fatalf("after a reader that died at once, the receiver's next wait is %v, want %v", got, want)
		}
	}
}

// TestESPFormReceiverEndsOnlyAHoldItHolds proves watchHeld and release refuse to end a
// hold no acquire took, as the BUG panic a Ze defect alone reaches, rather than drive the
// count below zero, where the paths would stay open with no hold and no SA.
//
// MUTATION: in release, drop the holds <= 0 guard: release drives holds to -1 and returns.
// MUTATION: in watchHeld, drop the holds <= 0 guard: watchHeld registers the SA and returns.
func TestESPFormReceiverEndsOnlyAHoldItHolds(t *testing.T) {
	target := natTarget(netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9"))
	for name, end := range map[string]func(r *espFormReceiver){
		"release":   func(r *espFormReceiver) { r.release() },
		"watchHeld": func(r *espFormReceiver) { r.watchHeld(0x5E0D0001, target) },
	} {
		t.Run(name, func(t *testing.T) {
			r := espFormTestReceiver(t, func() (espFormPaths, error) { return newIdleESPFormPaths(), nil })
			defer func() {
				got, _ := recover().(string)
				if !strings.HasPrefix(got, "BUG: ") {
					t.Errorf("%s with no hold panicked with %q, want a BUG panic", name, got)
				}
				if r.holds != 0 {
					t.Errorf("%s with no hold left holds at %d", name, r.holds)
				}
			}()
			end(r)
		})
	}
}

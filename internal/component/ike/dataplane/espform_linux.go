// Design: plan/release/validation/spec-ipsec-esp-dual-form-receive.md -- one Child SA receives both ESP forms
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)
// Overview: espform.go -- the datagram builder, the watched-SPI registry and the rate bound
// Related: espform_tun_linux.go -- the ze-espform device the refused form is re-presented on
// Related: espform_fastpath_linux.go -- the fast path the receiver starts, writes and stops

//go:build linux

package dataplane

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The XFRM socket-policy values espFormBypassInboundPolicy sets, spelled here rather
// than inline so the numbers carry their kernel names (include/uapi/linux/xfrm.h).
const (
	xfrmPolicyDirIn       = 0 // XFRM_POLICY_IN
	xfrmPolicyActionAllow = 0 // XFRM_POLICY_ALLOW
	xfrmInfinity          = ^uint64(0)
)

// espFormListenNetwork is the raw-socket network the ESP form reader opens: IPv4,
// protocol 50 (ProtoESP). It names the protocol by number, never as "ip4:esp".
// Go resolves a protocol name through /etc/protocols and otherwise only through a
// built-in table of icmp, igmp, tcp, udp and ipv6-icmp (net/lookup.go `protocols`),
// so on Ze's gokrazy image, which ships no /etc/protocols, "ip4:esp" fails with
// "unknown IP protocol specified" and every SA that asks for both ESP forms refuses
// to install. A number needs no lookup on any system.
const espFormListenNetwork = "ip4:50"

// errNoESPFormReceiver reports that this backend was built without the receiver that
// serves the second ESP form, so it cannot honor SAParams.AcceptBothESPForms.
var errNoESPFormReceiver = errors.New("dataplane: no ESP form receiver, so this SA would receive one ESP wire form only")

// espFormReadBufLen bounds one inbound ESP read. The read holds the whole IPv4 datagram,
// header included (espFormBareIPv4), which cannot exceed the 16-bit total length, and each
// buffer is allocated once per reader rather than per datagram (ai/rules/performance.md).
const espFormReadBufLen = 0xFFFF

// espFormPacketReader is the read half of the reader socket that run uses: one datagram,
// IPv4 header included, and the address that sent it. *net.IPConn implements it, and
// unlike its ReadFrom, ReadMsgIP leaves the IPv4 header in place, which is where the
// received TOS is read from. It is the seam that lets a unit test script the socket,
// which needs CAP_NET_RAW, the way espFormInjector scripts the device.
type espFormPacketReader interface {
	ReadMsgIP(b, oob []byte) (n, oobn, flags int, addr *net.IPAddr, err error)
}

// espFormInjector hands one built datagram to the stack's receive path.
//
// It is the seam that makes the re-presentation observable. The real writer is the
// ze-espform TUN (espFormTUN), which needs CAP_NET_ADMIN and hands the datagram to the
// kernel, where a unit test cannot read it back. A test therefore substitutes a recorder
// and asserts on the datagram run actually built (the fakeOps seam of
// ai/rules/testing.md).
type espFormInjector interface {
	// inject delivers pkt, one complete IPv4 datagram addressed to the local endpoint
	// the SA was installed with. A TUN write needs no destination beyond the header's.
	//
	// The implementation MUST NOT retain pkt: the reader hands it the same buffer for
	// every datagram.
	inject(pkt []byte) error
}

// espFormReceiver re-presents inbound ESP that arrived in the form its XFRM state does
// not accept, so ONE Child SA receives both wire forms.
//
// RFC 7296 Section 2.23 requires a device that supports NAT-T to receive both
// UDP-encapsulated and bare ESP "at any time". Linux XFRM binds one inbound state to one
// form: net/xfrm/xfrm_input.c compares the datagram's encapsulation type against the
// state's template and raises XfrmInStateMismatch when they disagree. A second state on
// the same SPI is refused outright ("file exists"), and a live state's template cannot be
// changed (net/xfrm/xfrm_state.c returns -EINVAL), so the kernel alone cannot serve both
// forms for one SA.
//
// This receiver covers the gap for the states that carry a template. The kernel keeps
// serving the encapsulated form itself. A bare ESP datagram for the same SPI is refused
// by XFRM, and Linux delivers a copy to a raw IPPROTO_ESP socket first
// (ip_protocol_deliver_rcu calls raw_local_deliver before the protocol handler), so this
// receiver reads it and writes it, in the encapsulated form the template demands and
// addressed to the port-4500 socket, into the Ze TUN ze-espform (espFormTUN). The stack
// receives it on that device, the port-4500 socket carries UDP_ENCAP_ESPINUDP, so the
// kernel strips the header again and hands XFRM the encapsulation type the template wants.
//
// The policy check runs on what this path delivers. XFRM decapsulates the datagram with
// skb->dev still ze-espform, whose disable_policy is 0, so the decrypted packet goes
// through XFRM's inbound policy check and is matched against the SA's selectors (RFC 4301
// Section 5.2), with the SPD's catch-all behind them. The reader socket's own exemption
// (espFormBypassInboundPolicy) covers only the reading of the refused copy, never the
// packet this path re-presents. Loopback, which this receiver once wrote to, carries
// disable_policy=1 and skipped that check entirely.
//
// The cost of the device: an input-interface firewall rule sees ze-espform, not the link
// the datagram arrived on, for the traffic this path carries.
//
// MEASURED against a real kernel by TestEncapOneStateAcceptsBothForms and
// TestEncapBareESPVisibleToUserspaceWhenStateIsTemplated
// (encap_hybrid_integration_linux_test.go), for the mechanism, and by
// TestEncapEstablishedSAServesAPeerFormChange for this receiver.
//
// The receiver owns two paths, and every bare datagram for a watched SPI takes exactly
// one of them. The fast path (espFormFastPath, espform_fastpath_linux.go) is a tc ingress
// program that adds the UDP header in place to every whole bare datagram it can rewrite,
// on every link it is attached to, so the kernel receives the state's own form on the
// real link and the raw reader never sees it: it is UDP by then. The complete path is
// the raw reader above, and it carries what the fast path leaves alone: fragments
// reassembled by the kernel, IPv4 headers with options, a link the program is not
// attached to, and everything while the program cannot load. A datagram the fast path
// rewrote is no longer bare ESP and one it left alone is refused by XFRM, so no datagram
// is served twice. The fast path never fails a Watch: when it cannot load or cannot
// watch an SPI, the complete path serves that SPI alone.
//
// Only SPIs whose inbound state carries a template are watched, which is what keeps the
// bare traffic of a template-free SA off both paths entirely. A bare datagram for a
// watched SPI is one XFRM would refuse, so re-presenting it can never duplicate a packet
// the kernel accepted.
//
// The cryptography stays in XFRM. This receiver changes only how a datagram is PRESENTED,
// and never decrypts, so the kernel keeps owning the replay window and the integrity
// check.
//
// The outer TOS survives both paths, so XFRM's decapsulation copies a CE mark into the
// inner header as RFC 7296 Section 2.24 requires: the fast path rewrites the header in
// place and never touches its TOS octet (espFormFastPathIPv4), and the complete path reads
// the TOS from the received header and writes it into the one it builds (writeESPForm).
//
// Watch, acquire, watchHeld, release, Forget, retarget and Close are safe for concurrent
// use. Each one updates the
// registry and decides whether to start or stop the paths in one critical section under
// mu, so a Watch can never see the receiver running between the last Forget's registry
// update and its stop. The reader goroutine and the fast path are started by the first
// Watch or acquire, and stopped, and waited for, by the last Forget or release once no
// hold remains, or by Close.
//
// A reader whose socket fails ends, and the SAs already watched are in the SAD with
// nothing serving their bare ESP, so the receiver opens the complete path again by
// itself (revive), at most espFormReopenWaitMax apart, until an open succeeds or no SA is
// watched and no hold remains. The wait backs off across readers (reopenWait), so a reader
// that dies right after each open is not reopened in a tight loop, and starts at once again
// only after a reader served espFormReopenResetAfter. Each stop is logged and handed to
// the engine (reportReaderStopped), which raises it on the report bus. A Watch or acquire
// that meets the
// stopped reader first opens the paths itself, and refuses with the open's error when
// they do not open. Every failure to open is recorded (failure) for the doctor until
// an open succeeds. Each reader has one revive worker, started with it and ended by the
// stop of its paths, or by the open that replaces them; Close waits for every one.
type espFormReceiver struct {
	log  *slog.Logger
	reg  espFormRegistry
	fast *espFormFastPath

	mu sync.Mutex
	// conn and tun are the complete path's reader socket and device, nil while closed.
	conn espFormReaderSocket
	tun  espFormDevice
	// life is the current reader's life: nil while the paths are stopped, and set, with
	// conn and tun nil, while the reader stopped and its revive worker keeps reopening.
	life *espFormReaderLife
	// wg counts the reader, joined under mu by closeReaderLocked: read takes no lock.
	wg sync.WaitGroup
	// revivers counts the revive workers, joined by Close after it released mu: a
	// worker takes mu for each attempt, so a join under mu could wait for it forever.
	revivers sync.WaitGroup
	// holds counts the acquire calls whose watch is not yet registered (watchHeld) or
	// given up (release). While it is above zero the paths stay open with no SA
	// watched, so the SA a MOBIKE move is about to template is served the moment the
	// move lands.
	holds int
	// failure is the last failure to open the paths, nil once an open succeeds. The
	// doctor reads it through fastPathState.
	failure *ESPFormCompletePathFailure
	// reopenWait is the wait before a revive worker's next attempt to reopen the paths,
	// kept across readers rather than per reader life: zero, so the first reopen comes at
	// once, then doubled from reopenMin up to espFormReopenWaitMax at each attempt, and
	// back to zero only at the stop of a reader that served espFormReopenResetAfter
	// (reopenWaitLocked). A test sets it longer to meet the stopped reader from a Watch.
	reopenWait time.Duration
	// reopenMin is the smallest wait the backoff doubles from: espFormReopenWaitMin, set
	// by newESPFormReceiver. A test sets it to zero, so its reopens come at once and it
	// waits on channels, never on the clock.
	reopenMin time.Duration
	// closed is set by Close. After it, Watch and acquire refuse with
	// errESPFormReceiverClosed and open nothing.
	closed bool
	// readerStopped, when set, takes each reader stop in place of the registered
	// ESPFormReaderStoppedSink. It is the seam that lets a test count the stops the
	// receiver hands on. Nil outside tests.
	readerStopped func(ESPFormReaderStopped)

	// registryUpdated, when set, runs under mu right after Watch, watchHeld or Forget
	// updated the registry and before the start or stop decision. It is the seam that
	// lets a test check that the two are one critical section. Nil outside tests.
	registryUpdated func()
	// openPaths, when set, replaces openESPFormPaths where the receiver opens the paths,
	// and the fast path is then not started. It is the seam that lets a test refuse the
	// open, or answer one with a scripted socket and device, without the privileges a
	// raw socket, a TUN device and a tc program need. Nil outside tests.
	openPaths func() (espFormPaths, error)
}

// espFormReaderSocket is the complete path's reader socket: the read run makes, and the
// close that ends that read. *net.IPConn implements it; a test scripts one.
type espFormReaderSocket interface {
	espFormPacketReader
	Close() error
}

// espFormDevice is the device the complete path writes into, closed only once the
// reader has stopped writing. *espFormTUN implements it; a test records into one.
type espFormDevice interface {
	espFormInjector
	Close() error
}

// espFormPaths is what one open of the complete path holds: the reader socket and the
// ze-espform device.
type espFormPaths struct {
	conn espFormReaderSocket
	tun  espFormDevice
}

// espFormReaderLife is one reader's life. stop is closed when the receiver no longer
// wants the reader (retireLocked), and dead by read when the socket failed while stop was
// open, so the read error ended a reader the receiver still wanted. err is that error,
// written before dead is closed and read only after it.
type espFormReaderLife struct {
	stop chan struct{}
	dead chan struct{}
	err  error
	// opened is when the reader started, which tells a reader that served for a while
	// from one that died right after its open (reopenWaitLocked).
	opened time.Time
}

// The waits between two attempts of a revive worker to reopen the paths. Each attempt
// costs a raw socket and a TUN device, so one every thirty seconds costs a router
// nothing, and the doubling from one second keeps a failure that persists (a capability
// taken away) to a few attempts a minute. There is no count: while an SA is watched its
// bare ESP is owed at any time (RFC 7296 Section 2.23), so the worker tries until the
// paths open or nothing is watched. A reader that served espFormReopenResetAfter before
// it stopped makes the next reopen come at once again: its stop is a new failure, not the
// same one repeating.
const (
	espFormReopenWaitMin    = time.Second
	espFormReopenWaitMax    = ESPFormCompletePathReopenMax
	espFormReopenResetAfter = time.Minute
)

// errESPFormReceiverClosed reports a Watch or an acquire after Close, which only a Ze
// defect produces: the backend MUST NOT use the receiver once it closed it.
var errESPFormReceiverClosed = errors.New("dataplane: the ESP form receiver is closed, so it opens no reader for this SA")

func newESPFormReceiver(log *slog.Logger) *espFormReceiver {
	return &espFormReceiver{log: log, fast: newESPFormFastPath(log), reopenMin: espFormReopenWaitMin}
}

// Watch re-presents bare ESP for the SA that carries spi to target.local until Forget.
// target is the tuple the inbound state was installed with: its source address as the
// peer, its destination as the local address, and its template's source and destination
// ports as the peer and local ports, so the first datagram re-presented already carries
// the ports the state matches.
//
// It reports an error when the reader socket or the ze-espform device cannot be opened.
// The caller MUST surface that, because a receiver that is not running leaves one of the
// two ESP forms unserved and the tunnel then carries traffic in one form only
// (ai/rules/evidence.md). The fast path never fails it: a program that cannot load, or
// an SPI it cannot watch, leaves that SPI to the complete path, which serves it.
// A nil receiver FAILS CLOSED here, and that is the whole point of returning an error.
// xfrmBackend can be built as a bare literal, which leaves espForms nil. An SA that asked
// to receive both ESP forms and got a silent nil would install a state serving ONE form
// and drop the other, which is the quietest failure this subsystem has
// (ai/rules/evidence.md). Refusing the install surfaces it instead.
func (r *espFormReceiver) Watch(spi uint32, target espFormTarget) error {
	if r == nil {
		return errNoESPFormReceiver
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Before the registry: a closed receiver opens nothing again, and watches nothing.
	if r.closed {
		return errESPFormReceiverClosed
	}
	r.reg.watch(spi, target)
	r.afterRegistryUpdate()
	// The paths are not serving on the first SPI, and after the reader stopped: an SA
	// installed on a stopped reader would receive one ESP form only.
	if !r.servingLocked() {
		if err := r.openLocked(); err != nil {
			r.reg.forget(spi, target.local)
			r.stopIfUnusedLocked()
			return err
		}
	}
	r.serveLocked(spi, target.local)
	return nil
}

// acquire opens the reader socket and the ze-espform device when they are not serving, a
// reader that stopped included, and holds them open, with no SA watched, until the caller
// registers the SA with watchHeld or gives the hold up with release. It answers the open's
// error with nothing held, the error Watch would answer, and leaves open only what served
// the SAs already watched: a stopped reader's revive worker keeps trying for them. The
// failure is recorded for the doctor. MigrateTunnel calls it before it moves
// the SAD, so a move that would template an SA the receiver cannot serve is refused while
// nothing has changed. A caller whose acquire answered nil MUST end the hold with exactly
// one call to watchHeld or release. A nil receiver answers errNoESPFormReceiver.
//
// RFC 7296 Section 2.23: "all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."
func (r *espFormReceiver) acquire() error {
	if r == nil {
		return errNoESPFormReceiver
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errESPFormReceiverClosed
	}
	if !r.servingLocked() {
		if err := r.openLocked(); err != nil {
			return err
		}
	}
	r.holds++
	return nil
}

// watchHeld registers the SA that carries spi to target.local, as Watch does, on the
// paths an acquire holds open, and ends that hold. It cannot fail: the paths were open
// at the acquire, and a reader that stopped since is reopened by its revive worker,
// which serves every watched SA. MUST be called only after acquire answered nil, once
// per hold.
func (r *espFormReceiver) watchHeld(spi uint32, target espFormTarget) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.holds <= 0 {
		panic("BUG: espFormReceiver.watchHeld called with no hold an acquire took")
	}
	r.reg.watch(spi, target)
	r.afterRegistryUpdate()
	r.holds--
	r.serveLocked(spi, target.local)
}

// release ends a hold an acquire took, for an SA that will not be registered, and
// closes the paths when no other hold remains and no SA is watched. MUST be called only
// after acquire answered nil, once per hold.
func (r *espFormReceiver) release() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.holds <= 0 {
		panic("BUG: espFormReceiver.release called with no hold an acquire took")
	}
	r.holds--
	r.stopIfUnusedLocked()
}

// stopIfUnusedLocked stops the paths when no SA is watched and no hold remains, so a
// failed Watch or an ended hold leaves nothing open, and no revive worker reopening,
// for nothing. r.mu must be held.
func (r *espFormReceiver) stopIfUnusedLocked() {
	if r.holds != 0 {
		return
	}
	if !r.reg.empty() {
		return
	}
	r.stopLocked()
}

// servingLocked reports whether a reader is open and still reading: the paths are open
// and the reader's socket has not failed. r.mu must be held.
func (r *espFormReceiver) servingLocked() bool {
	if r.conn == nil {
		return false
	}
	select {
	case <-r.life.dead:
		return false
	default:
		return true
	}
}

// openLocked opens the paths, through the test seam when one is set, replacing a reader
// that stopped, and starts the fast path. The outcome is recorded: a failure for the
// doctor, with nothing left open but the stopped reader's revive worker, which keeps
// trying; a success clears the record. r.mu must be held.
func (r *espFormReceiver) openLocked() error {
	if err := r.reopenReaderLocked(); err != nil {
		return err
	}
	if r.openPaths != nil {
		return nil
	}
	// The fast path starts last and never fails the open: a program that cannot load
	// leaves the complete path, already running, to serve every bare datagram. One
	// already started is left as it is.
	r.fast.start()
	return nil
}

// reopenReaderLocked closes a reader that stopped, opens the reader socket and the
// device, and starts a new reader on them, which replaces the stopped one and ends its
// revive worker. When the open fails the stopped reader's life stays, so its revive
// worker keeps trying. The device is closed before the open because a second
// ze-espform cannot exist. r.mu must be held, and the paths MUST NOT be serving.
func (r *espFormReceiver) reopenReaderLocked() error {
	r.closeReaderLocked()
	open := openESPFormPaths
	if r.openPaths != nil {
		open = r.openPaths
	}
	paths, err := open()
	if err != nil {
		r.failure = &ESPFormCompletePathFailure{Err: err, At: time.Now()}
		return err
	}
	r.failure = nil
	r.retireLocked()
	r.startReaderLocked(paths)
	return nil
}

// startReaderLocked starts a reader and its revive worker on paths. r.mu must be held,
// and no reader life may be current.
func (r *espFormReceiver) startReaderLocked(paths espFormPaths) {
	life := &espFormReaderLife{stop: make(chan struct{}), dead: make(chan struct{}), opened: time.Now()}
	r.conn, r.tun, r.life = paths.conn, paths.tun, life

	r.wg.Add(1)
	go r.read(paths.conn, paths.tun, life)
	r.revivers.Add(1)
	go r.revive(life)
}

// retireLocked tells the current reader and its revive worker that the receiver no
// longer wants them, before its socket is closed, so the read error that close causes
// is not taken for a failed socket. r.mu must be held.
func (r *espFormReceiver) retireLocked() {
	if r.life == nil {
		return
	}
	close(r.life.stop)
	r.life = nil
}

// closeReaderLocked closes the reader socket, waits for the reader to finish, and only
// then closes the ze-espform device. Closing the socket ends the reader's ReadMsgIP, and
// read takes no lock of the receiver's, so waiting here under r.mu cannot deadlock. The
// reader writes into the device's fd, so closing that fd while it may still write would
// let the number be reused by an unrelated open and a late write land in it. r.mu must
// be held.
func (r *espFormReceiver) closeReaderLocked() {
	if r.conn == nil {
		return
	}
	// A failed close leaves nothing to act on. Linux releases the descriptor whatever
	// close(2) answers, and Go answers ErrClosed only for a second close, which the nil
	// guard above rules out, so no socket or device outlives this stop and no operator
	// step follows from the error.
	r.conn.Close() //nolint:errcheck // the descriptor is released whatever close answers; see above
	r.wg.Wait()
	r.tun.Close() //nolint:errcheck // the descriptor is released whatever close answers; see above
	r.conn, r.tun = nil, nil
}

// serveLocked writes the SA that carries spi to local into the fast path, and warns of
// a second SA on the SPI. r.mu must be held.
func (r *espFormReceiver) serveLocked(spi uint32, local netip.Addr) {
	// Both SAs are served: the complete path tells them apart by local address, and an
	// attached fast path keeps the first, so only this one's bare ESP is rate-bounded.
	// A fast path that is not attached serves no SA, so every SA's bare ESP already takes
	// the complete path, the collision costs nothing more, and that status has its own
	// warning: the collision is reported only while the fast path is attached, as the
	// doctor check reports it.
	served, ok := r.reg.fastTarget(spi)
	if ok && served.local != local && r.fast.currentStatus() == ESPFormFastPathAttached {
		r.logger().Warn("esp-form: two Child SAs carry one inbound SPI on two local addresses, so the "+
			"fast path keeps serving the first, and the bare ESP of the second is re-presented through "+
			espFormTUNName+" by the complete path, rate-bounded. Clearing either peer whose Child SA "+
			"carries the SPI, with "+ESPFormFixClearPeer+"<name>, gives its new Child SA a new SPI and "+
			"ends this; the listing shows each peer's inbound SPI",
			"spi", spi, "local", local, "fast-path-local", served.local,
			"datagrams-per-second-max", espFormRate, "fix", ESPFormFixListSAs,
			"code", diagnostic.CodeDoctorIPsecESPFormFastPath)
	}
	r.syncFastPath(spi)
}

// Forget stops re-presenting bare ESP for the SA that carries this SPI to local, and
// releases the reader socket, the ze-espform device and the fast path once no SA is
// watched and no acquire holds them. Another SA that carries the same SPI to another local address stays served,
// and takes the fast path over when this one held it. A deployment whose SAs all run
// without a template therefore holds no raw socket, no device and no program at all,
// and the kernel clones no bare ESP packet for it.
// A nil receiver holds nothing, so Forget is a no-op rather than a panic. Watch is where
// the absence is reported; a teardown path must never be the thing that crashes.
func (r *espFormReceiver) Forget(spi uint32, local netip.Addr) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	last := r.reg.forget(spi, local)
	r.afterRegistryUpdate()
	r.syncFastPath(spi)
	// A hold keeps the paths open for an SA about to be registered (acquire).
	if last && r.holds == 0 {
		r.stopLocked()
	}
}

// retarget updates the SA that carries spi to from after a MOBIKE address or port
// change, in the registry the complete path reads and in the fast path's map, and
// answers nil when it did. It does not create a watch or change the lifetime of the
// receiver's paths: an SA that is not watched is left alone (errESPFormNotWatched), and
// so is one whose new local address another SA on the same SPI already holds
// (errESPFormLocalTaken), which espFormRegistry.retarget alone decides. It logs
// nothing: its one caller, followMove, logs the refusal with both addresses, so one
// event makes one log line. A nil receiver watches nothing, so it answers
// errNoESPFormReceiver.
func (r *espFormReceiver) retarget(spi uint32, from netip.Addr, target espFormTarget) error {
	if r == nil {
		return errNoESPFormReceiver
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.reg.retarget(spi, from, target); err != nil {
		return err
	}
	r.syncFastPath(spi)
	return nil
}

// admitsMove answers errESPFormLocalTaken when a move of the SA on spi from one local
// address onto to would collide with another SA on that SPI, and changes nothing. For a
// watched SA that is the refusal retarget would give. For an SA not watched on from, the
// move registers it on to with watchHeld, which replaces whatever SA holds that SPI and
// address, so this check is the only thing that keeps the move from taking another SA's
// watch. MigrateTunnel asks it before it touches the SAD, so a move the receiver cannot
// follow is refused while nothing has changed. A nil receiver
// watches nothing, so it answers errNoESPFormReceiver.
func (r *espFormReceiver) admitsMove(spi uint32, from, to netip.Addr) error {
	if r == nil {
		return errNoESPFormReceiver
	}
	return r.reg.admitsMove(spi, from, to)
}

// target reports the address pair of the SA that carries spi to local, and whether it
// is watched at all. A nil receiver watches nothing.
func (r *espFormReceiver) target(spi uint32, local netip.Addr) (espFormTarget, bool) {
	if r == nil {
		return espFormTarget{}, false
	}
	return r.reg.target(spi, local)
}

// Close releases the reader socket, the ze-espform device and the fast path, and waits
// for the reader, every revive worker and the fast path's worker to finish. Callers MUST
// call it when the backend shuts down, and MUST NOT call Watch or acquire after it: both
// refuse with errESPFormReceiverClosed and open nothing.
// A nil receiver holds no socket and no goroutine, so Close succeeds trivially.
func (r *espFormReceiver) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.closed = true
	r.reg.forgetAll()
	r.stopLocked()
	r.mu.Unlock()

	// After the unlock: a revive worker waiting for mu then finds its reader retired and
	// returns.
	r.revivers.Wait()
	return nil
}

// afterRegistryUpdate runs the test seam, when one is set. r.mu must be held.
func (r *espFormReceiver) afterRegistryUpdate() {
	if r.registryUpdated != nil {
		r.registryUpdated()
	}
}

// syncFastPath writes into the fast path's map the target of the SA it serves for spi
// (espFormRegistry.fastTarget), or takes spi out of the map once no SA carries it.
// r.mu must be held.
func (r *espFormReceiver) syncFastPath(spi uint32) {
	target, ok := r.reg.fastTarget(spi)
	if ok {
		r.watchFastPath(spi, target)
		return
	}
	r.fast.remove(spi)
}

// watchFastPath writes one SPI's target into the fast path's map. A failure never fails
// the caller: the SPI stays watched and the complete path serves its bare ESP,
// rate-bounded, so the error is logged at Warn with the fast path's doctor code. r.mu
// must be held.
func (r *espFormReceiver) watchFastPath(spi uint32, target espFormTarget) {
	err := r.fast.set(spi, target.peer, target.local, target.peerPort, target.localPort)
	if err == nil {
		return
	}
	r.logger().Warn("esp-form: the fast path could not watch an SPI, so its bare ESP is re-presented through "+
		espFormTUNName+" by the complete path, rate-bounded",
		"spi", spi, "error", err, "datagrams-per-second-max", espFormRate,
		"code", diagnostic.CodeDoctorIPsecESPFormFastPath)
}

// fastPathState answers the fast path's state, with the SAs it does not serve because
// another SA holds their SPI in its map (espFormRegistry.shared), read from the registry
// the Watch and Forget calls change. A nil receiver watches nothing, so its fast path is
// idle.
//
// The receiver's last failure to open its complete path rides in the state too
// (CompletePathFailure), read under mu and whether or not the fast path's counters read.
func (r *espFormReceiver) fastPathState() (ESPFormFastPathState, error) {
	if r == nil {
		// refuse-by-default: guards Watch -- a nil receiver refuses every Watch with errNoESPFormReceiver, so no SPI is watched and nothing is loaded, which is what Idle states
		return ESPFormFastPathState{Status: ESPFormFastPathIdle, FirewallReading: ESPFormFirewallUnheld}, nil
	}
	r.mu.Lock()
	failure := r.failure
	r.mu.Unlock()

	st, err := r.fast.state()
	st.CompletePathFailure = failure
	if err != nil {
		return st, err
	}
	st.SharedSPIs = r.reg.shared()
	return st, nil
}

// holdFastPath shuts the fast path's gate for a firewall commit and answers its
// release. It takes no receiver lock: r.fast is set at construction and never
// replaced, and the firewall calls this while it holds its own reconcile lock. A nil
// receiver has no fast path, so its release does nothing.
func (r *espFormReceiver) holdFastPath() (release func()) {
	if r == nil {
		return func() {}
	}
	return r.fast.hold()
}

// logger reports the receiver's logger, and a usable one when the receiver is nil so a
// caller on an error path never has to guard the call.
func (r *espFormReceiver) logger() *slog.Logger {
	if r == nil || r.log == nil {
		return slogutil.Logger("ike.dataplane")
	}
	return r.log
}

// running reports whether the reader socket and the device are open. A nil receiver
// never has any.
func (r *espFormReceiver) running() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conn != nil
}

// empty reports whether no SA is watched, on any SPI. The receiver reads it when a hold
// on its paths ends or a Watch fails, so what registered no SA closes what it opened. It
// lives here, beside its only reader, because only the Linux receiver holds paths.
func (r *espFormRegistry) empty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.watched) == 0
}

// espFormBypassInboundPolicy exempts one socket from the inbound IPsec policy check.
//
// WHAT IT COVERS: the reading of the refused copy, and nothing else. The datagram the
// receiver then re-presents arrives on ze-espform, where disable_policy is 0, so after
// XFRM decrypts it the inbound policy check runs on the decrypted packet and matches it
// against the SA's selectors (RFC 4301 Section 5.2). No packet reaches a host or a
// forwarding path through this exemption.
//
// WHY THIS EXISTS, and it is MEASURED rather than reasoned. Linux runs
// xfrm4_policy_check on every packet it delivers to a raw socket: net/ipv4/raw.c
// raw_rcv drops the packet and raises XfrmInTmplMismatch when the check fails. The
// datagrams this receiver exists to read are exactly the ones that failed to arrive
// through an SA, and ze's own inbound Child SA policy (engine/child.go, dir in with
// an ESP template over the negotiated selectors) demands that they did. So the
// kernel refuses to hand ze the packet ze installed the reader to recover, and the
// one ESP form is lost with no log line anywhere.
//
// Measured in the strongSwan lab on 2026-08-03: with the reader open and the peer
// sending bare ESP to a templated state, /proc/net/raw showed the protocol-50
// socket with drops equal to the packet count, ze's inbound SA stayed at 0 packets,
// and XfrmInStateMismatch and XfrmInTmplMismatch each advanced once per datagram.
//
// A per-SOCKET policy is the right bound and a global one is not. Exempting
// "inbound ESP" globally would exempt every peer's ESP from the policy check,
// which is the check that proves a packet arrived through the SA the operator
// configured. This exemption reaches only datagrams delivered to this one raw
// socket, which reads the still-encrypted copy XFRM refused, and the cryptography is
// untouched: XFRM still decrypts and still owns the replay window. It is the same bound
// strongSwan takes for its own sockets (engine/bypass.go records the shape).
//
// RFC 4301 Section 4.4.1: BYPASS is an SPD disposition, and this is one, scoped to
// a socket ze owns.
//
// It takes the IP socket itself: only a raw IP socket can carry the exemption, and the
// type, not a check, keeps any other socket out.
func espFormBypassInboundPolicy(conn *net.IPConn) error {
	sc, err := conn.SyscallConn()
	if err != nil {
		return espFormBypassRefusal(err)
	}

	// XFRM_INF on every limit: a socket exemption has no lifetime, and a zero limit
	// is not the spelling for that.
	pol := nl.XfrmUserpolicyInfo{
		Sel: nl.XfrmSelector{Family: unix.AF_INET},
		Lft: nl.XfrmLifetimeCfg{
			SoftByteLimit:   xfrmInfinity,
			HardByteLimit:   xfrmInfinity,
			SoftPacketLimit: xfrmInfinity,
			HardPacketLimit: xfrmInfinity,
		},
		// The kernel copies a socket policy's priority and never matches on it, so
		// it carries Ze's mark: the listing names this row esp-form-socket.
		Priority: PriorityESPFormSocket,
		Dir:      xfrmPolicyDirIn,
		Action:   xfrmPolicyActionAllow,
	}

	var setErr error
	if ctlErr := sc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptString(int(fd), unix.IPPROTO_IP, unix.IP_XFRM_POLICY, string(pol.Serialize()))
	}); ctlErr != nil {
		return espFormBypassRefusal(ctlErr)
	}
	if setErr != nil {
		return espFormBypassRefusal(setErr)
	}
	return nil
}

// stopLocked stops the fast path, then the reader, and only then closes the ze-espform
// device (closeReaderLocked). It also ends the reader's revive worker. r.mu must be held.
//
// The order is the point. The fast path goes first, so from then on every bare datagram
// reaches the reader, which is still open to serve it. The fast path's worker takes only
// the fast path's own lock, so its stop under r.mu cannot deadlock. The reader is retired
// before its socket closes, so the read error the close causes is not taken for a failed
// socket. Holding r.mu across the whole stop also means a Watch that races the last
// Forget opens its device only after this one is gone: two holders of the name
// ze-espform cannot exist.
func (r *espFormReceiver) stopLocked() {
	if r.life == nil {
		return
	}
	r.fast.stop()
	r.retireLocked()
	r.closeReaderLocked()
}

// espFormStopped reports whether the reader has been asked to finish.
func espFormStopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// run reads bare ESP and re-presents every datagram whose SPI is watched and whose
// source is that SA's peer. It is one long-lived worker for the life of the reader
// socket and the device (ai/rules/goroutine-lifecycle.md), and both buffers are allocated
// once here rather than per datagram. It takes no lock of the receiver's, which is what
// lets closeReaderLocked wait for it while holding r.mu.
//
// It answers the read error that ended it: the close of a stop, or a socket that failed,
// which its caller (read) tells apart by the reader's stop. conn and inj are the seams: the production caller
// passes the reader socket and the ze-espform device, and a test passes a scripted
// socket and a recorder so the re-presented datagram can be read back.
func (r *espFormReceiver) run(conn espFormPacketReader, inj espFormInjector) error {
	read := make([]byte, espFormReadBufLen)
	out := make([]byte, espFormReadBufLen)
	limiter := newESPFormLimiter(time.Now())
	// failing reports that the last write into the device failed, so a device that
	// stays broken logs once rather than once per datagram.
	failing := false

	for {
		// No control message is asked for: the TOS travels in the header itself.
		n, _, _, from, err := conn.ReadMsgIP(read, nil)
		if err != nil {
			// The caller (read) logs nothing for a stop and wakes the revive worker for a
			// failed socket, which logs it with its doctor code.
			return err
		}

		tos, dst, esp, ok := espFormBareIPv4(read[:n])
		if !ok {
			continue
		}
		spi, ok := espFormSPI(esp)
		if !ok {
			continue
		}
		// The SA is the SPI and the datagram's destination, the pair XFRM keys an inbound
		// state on, so of two SAs that carry one SPI each datagram reaches its own. Only
		// the SA's local address is served, as the fast path serves it: the raw socket
		// receives bare ESP the peer sent to any address of this host, broadcast
		// included, and the datagram is rebuilt addressed to target.local, so a datagram
		// sent elsewhere finds no SA here rather than being re-addressed onto one.
		target, watched := r.reg.target(spi, dst)
		if !watched {
			// A template-free SA's own traffic, which XFRM has already accepted, or bare
			// ESP sent to an address no watched SA holds.
			continue
		}
		// The datagram keeps its real source, and only the SA's peer is served. The state
		// lookup ignores the source (__xfrm_state_lookup keys on destination, SPI,
		// protocol and family), so a datagram from any other address would still reach
		// the SA's crypto check, and one that passed it would be delivered with a changed
		// NAT mapping reported (esp_input_done2, net/ipv4/esp4.c). Neither is traffic the
		// SA was installed for: its peer is the address its keys were negotiated with, and
		// MOBIKE moves that address through retarget, never through a datagram. Checked
		// before the rate bound, so a flood from elsewhere spends none of the peer's share.
		source, ok := espFormSourceAddr(from)
		if !ok {
			continue
		}
		if source != target.peer {
			continue
		}
		if !limiter.allow(time.Now()) {
			continue
		}

		// RFC 7296 Section 2.24: the received TOS goes on the rebuilt outer header, so
		// XFRM's decapsulation sees the ECN field the peer's path set (writeESPForm).
		wrote := writeESPForm(out, source, target.local, tos, esp)
		if wrote == 0 {
			continue
		}
		// RFC 3948 Section 2.1: the ports MUST be those used by IKE traffic.
		// MOBIKE can change the translated peer port without changing its SPI.
		// The ports sit in the UDP header, outside the IPv4 header checksum, and the
		// UDP checksum is zero, so rewriting them leaves both checksums valid.
		binary.BigEndian.PutUint16(out[20:22], target.peerPort)
		binary.BigEndian.PutUint16(out[22:24], target.localPort)

		if err := inj.inject(out[:wrote]); err != nil {
			if !failing {
				// The TUN driver refuses a write with EIO while the interface is down
				// (tun_get_user, drivers/net/tun.c), the cause an operator can meet:
				// something outside Ze set ze-espform down.
				r.logger().Warn("esp-form: cannot re-present refused ESP into "+espFormTUNName+
					", the bare ESP form is not received until a write succeeds", "spi", spi, "error", err,
					"fix", "ip link set "+espFormTUNName+" up",
					"code", diagnostic.CodeDoctorIPsecESPFormTUN)
			}
			failing = true
			continue
		}
		failing = false
	}
}

// espFormSourceAddr reads the outer source address the raw ESP socket reported. It
// reports ok false when the socket reported none, or one that is not an IP address, so
// a datagram whose sender cannot be read is never re-presented under some other address.
func espFormSourceAddr(from *net.IPAddr) (netip.Addr, bool) {
	if from == nil {
		return netip.Addr{}, false
	}
	addr, ok := netip.AddrFromSlice(from.IP)
	if !ok {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// The reasons espFormRegistry.retarget refuses a move. A caller branches on them with
// errors.Is: only errESPFormLocalTaken is a problem to report, because an SA that is not
// watched is left alone by design.
var (
	// errESPFormNotWatched: no SA with the SPI is watched on the old local address.
	errESPFormNotWatched = errors.New("dataplane: no SA with this inbound SPI is watched on the old local address")
	// errESPFormLocalTaken: another SA with the SPI already holds the new local address.
	errESPFormLocalTaken = errors.New("dataplane: another Child SA with this inbound SPI holds the new local address")
)

// retarget updates the SA that carries spi to from after a MOBIKE address or port
// change, and answers nil when it did, so the receiver writes the fast path's map only
// for an SA it serves. It refuses with errESPFormLocalTaken a move onto a local address
// another SA on the same SPI holds: replacing that SA's target would leave it unserved.
// This is the one check of that collision; the receiver logs from its answer, and
// MigrateTunnel asks it first through admitsMove. It refuses with errESPFormNotWatched
// an SA that is not watched. It does not create a watch or change the lifetime of the
// receiver's paths. It lives here because only the Linux XFRM backend moves an SA.
// Callers outside the receiver go through espFormReceiver.retarget, which updates the
// fast path too.
func (r *espFormRegistry) retarget(spi uint32, from netip.Addr, target espFormTarget) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	at, err := r.moveSlotLocked(spi, from, target.local)
	if err != nil {
		return err
	}
	if at < 0 {
		return errESPFormNotWatched
	}
	r.watched[spi][at] = target
	return nil
}

// admitsMove answers the refusal retarget would give a move of the SA on spi from one
// local address to another, errESPFormLocalTaken or nil, and changes nothing. An SA not
// watched on from is no refusal here, and the same check judges it: the caller then
// registers it on to, and a watch REPLACES an SA already on that SPI and address rather
// than refusing it, so this answer is the only thing that keeps such a move from taking
// another SA's watch.
func (r *espFormRegistry) admitsMove(spi uint32, from, to netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := r.moveSlotLocked(spi, from, to)
	return err
}

// moveSlotLocked answers the index of the SA on spi that carries from, -1 when none
// does, and errESPFormLocalTaken when another SA on spi carries to. r.mu MUST be held.
func (r *espFormRegistry) moveSlotLocked(spi uint32, from, to netip.Addr) (int, error) {
	at := -1
	for i, sa := range r.watched[spi] {
		if sa.local == from {
			at = i
			continue
		}
		if sa.local == to {
			return -1, errESPFormLocalTaken
		}
	}
	return at, nil
}

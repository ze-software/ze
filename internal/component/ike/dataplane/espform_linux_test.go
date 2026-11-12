//go:build linux

package dataplane

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/test/userns"
)

// VALIDATES: the receiver opens its reader socket on the first watched SPI, keeps it while
// any SPI is watched, and releases it when the last one is forgotten.
// PREVENTS: holding a raw IPPROTO_ESP socket in a deployment whose SAs all run without an
// encapsulation template. That socket makes the kernel clone every bare ESP packet on the
// box, which taxes the kernel's own ESP receive for every tunnel, not only this one.
func TestESPFormReceiverHoldsSocketsOnlyWhileWatching(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())

	if r.running() {
		t.Fatal("a new receiver already holds sockets")
	}

	if err := r.Watch(0x1111, natTarget(peer, local)); err != nil {
		t.Fatalf("open the raw ESP sockets in the test's own namespace: %v", err)
	}
	if !r.running() {
		t.Fatal("the first watch did not start the receiver")
	}

	if err := r.Watch(0x2222, natTarget(peer, local)); err != nil {
		t.Fatalf("second watch: %v", err)
	}
	if !r.running() {
		t.Fatal("a second watch stopped the receiver")
	}

	r.Forget(0x1111, local)
	if !r.running() {
		t.Fatal("forgetting one of two SPIs released the sockets; the other SA loses its bare ESP form")
	}

	r.Forget(0x2222, local)
	if r.running() {
		t.Fatal("forgetting the last SPI left the sockets open")
	}

	// Close after a full release must not double-close, and must stay safe to call.
	if err := r.Close(); err != nil {
		t.Errorf("close after release: %v", err)
	}
}

// VALIDATES: the first Watch creates ze-espform, up, at MTU 65535, holding 192.0.0.8/32,
// with IPv4 disable_policy 0 and rp_filter 2; the device stays while any SPI is watched,
// and the last Forget and Close each remove it.
// PREVENTS: the bypass this device exists to close. The re-presented datagram is
// policy-checked only because it arrives on a device whose disable_policy is 0: a device
// that kept the value 1, as loopback carries, would deliver bare ESP with no RFC 4301
// Section 5.2 selector check again. rp_filter 1 would drop every re-presented datagram,
// because the peer's source routes back through the real link. A device left behind
// after the last SA would hold the name, so the next Watch could not create it.
func TestESPFormReceiverHoldsTUNOnlyWhileWatching(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())

	if err := r.Watch(0x1111, natTarget(peer, local)); err != nil {
		t.Fatalf("open the reader socket and %s in the test's own namespace: %v", espFormTUNName, err)
	}
	link, err := netlink.LinkByName(espFormTUNName)
	if err != nil {
		t.Fatalf("the first watch left no %s device: %v", espFormTUNName, err)
	}
	if attrs := link.Attrs(); attrs.Flags&net.FlagUp == 0 {
		t.Errorf("%s is down; the stack drops every datagram written into it", espFormTUNName)
	}
	if mtu := link.Attrs().MTU; mtu != espFormTUNMTU {
		t.Errorf("%s MTU %d, want %d; a reassembled datagram can be that large", espFormTUNName, mtu, espFormTUNMTU)
	}
	espFormWantTUNAddr(t, link)
	conf4 := "/proc/sys/net/ipv4/conf/" + espFormTUNName + "/"
	espFormWantSysctl(t, conf4+"disable_policy", "0")
	espFormWantSysctl(t, conf4+"rp_filter", "2")

	if err := r.Watch(0x2222, natTarget(peer, local)); err != nil {
		t.Fatalf("second watch: %v", err)
	}
	r.Forget(0x1111, local)
	if _, err := netlink.LinkByName(espFormTUNName); err != nil {
		t.Fatalf("forgetting one of two SPIs removed %s; the other SA loses its bare ESP form: %v", espFormTUNName, err)
	}
	r.Forget(0x2222, local)
	espFormWantNoTUN(t, "the last Forget")

	// The name is free again, so a later SA can create the device afresh, and Close
	// removes it as the last Forget did.
	if err := r.Watch(0x3333, natTarget(peer, local)); err != nil {
		t.Fatalf("watch after the device was removed: %v", err)
	}
	if _, err := netlink.LinkByName(espFormTUNName); err != nil {
		t.Fatalf("a watch after the last Forget left no %s device: %v", espFormTUNName, err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	espFormWantNoTUN(t, "Close")
}

// espFormWantTUNAddr fails the test unless the device holds exactly one IPv4 address,
// 192.0.0.8/32 of scope host. Without an address the loose reverse-path filter drops
// every re-presented datagram (__fib_validate_source, the no_addr branch), and an
// address of wider scope could be chosen as the source of outbound traffic.
func espFormWantTUNAddr(t *testing.T, link netlink.Link) {
	t.Helper()
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		t.Errorf("list the IPv4 addresses of %s: %v", espFormTUNName, err)
		return
	}
	if len(addrs) != 1 {
		t.Errorf("%s holds %d IPv4 addresses (%v), want exactly %s/32", espFormTUNName, len(addrs), addrs, espFormTUNAddr)
		return
	}
	ones, bits := addrs[0].Mask.Size()
	if !addrs[0].IP.Equal(espFormTUNAddr) || ones != 32 || bits != 32 {
		t.Errorf("%s holds %v, want %s/32", espFormTUNName, addrs[0].IPNet, espFormTUNAddr)
	}
	if addrs[0].Scope != unix.RT_SCOPE_HOST {
		t.Errorf("%s address scope %d, want host (%d), so it is never a source for outbound traffic",
			espFormTUNName, addrs[0].Scope, unix.RT_SCOPE_HOST)
	}
}

// espFormWantSysctl fails the test unless the /proc/sys file at path reads want.
func espFormWantSysctl(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", path, err)
		return
	}
	if got := strings.TrimSpace(string(raw)); got != want {
		t.Errorf("%s reads %s, want %s", path, got, want)
	}
}

// espFormWantNoTUN fails the test unless ze-espform is gone after the step named.
func espFormWantNoTUN(t *testing.T, step string) {
	t.Helper()
	_, err := netlink.LinkByName(espFormTUNName)
	if _, notFound := errors.AsType[netlink.LinkNotFoundError](err); !notFound {
		t.Errorf("after %s, looking up %s answered %v, want the device gone", step, espFormTUNName, err)
	}
}

// VALIDATES: a nil receiver has defined behavior on every method. Watch FAILS CLOSED, and
// the teardown methods are no-ops.
// PREVENTS: the panic this guard was written for. xfrmBackend is built as a bare
// &xfrmBackend{} literal in ten places, which leaves espForms nil, and RemoveSA and Close
// call into it unconditionally on a production path. Before the guard, tearing down any SA
// on such a backend crashed the process.
//
// Watch must not become a silent no-op. An SA that asked to receive both ESP forms and got
// one anyway drops the other form, and the tunnel then establishes and carries no traffic.
func TestESPFormReceiverNilIsFailClosed(t *testing.T) {
	var r *espFormReceiver

	if err := r.Watch(0x1111, natTarget(netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9"))); err == nil {
		t.Error("a nil receiver accepted a watch; the SA would silently receive one ESP form only")
	}
	if r.running() {
		t.Error("a nil receiver reported that it is running")
	}

	// The teardown path must never be the thing that crashes.
	r.Forget(0x1111, netip.MustParseAddr("203.0.113.9"))
	if err := r.Close(); err != nil {
		t.Errorf("close on a nil receiver: %v", err)
	}
}

// VALIDATES: Close releases the reader socket and the device, and is safe while SPIs are
// still watched.
// PREVENTS: a backend shutdown leaking the reader goroutine, its raw socket and ze-espform.
func TestESPFormReceiverCloseWhileWatching(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())

	if err := r.Watch(0x3333, natTarget(peer, local)); err != nil {
		t.Fatalf("open the raw ESP sockets in the test's own namespace: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if r.running() {
		t.Fatal("close left the sockets open")
	}
	if r.reg.watching(0x3333) {
		t.Error("close left an SPI watched, so a later Watch would not restart the receiver")
	}
}

// VALIDATES: the last Forget updates the registry and stops the receiver in one critical
// section, so a Watch that races it runs wholly after the stop and starts the receiver
// again: the receiver ends running, with the new SPI watched.
// PREVENTS: a stranded SPI. When Watch and Forget updated the registry before taking r.mu,
// Forget(A) could empty the set, Watch(B) then saw the receiver still running and
// returned, and Forget stopped it. B stayed watched with no reader, no ze-espform and no
// fast path, so its bare ESP form was lost with no error anywhere.
// METHOD: the registryUpdated seam runs inside the last Forget, after its registry update
// and before its stop decision. There the test checks that r.mu is held (TryLock fails),
// which decides the question with no timing at all, and starts Watch(B) on a goroutine.
// Because Forget holds r.mu, Watch(B) can only take it once the stop is done, so it meets
// a stopped receiver and starts it. The test waits for Watch(B) to answer, then asserts.
// No sleep: the order comes from the lock alone.
func TestESPFormReceiverWatchCannotRaceTheLastForget(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	if err := r.Watch(0x1111, natTarget(peer, local)); err != nil {
		t.Fatalf("open the reader socket and %s in the test's own namespace: %v", espFormTUNName, err)
	}

	watched := make(chan error, 1)
	// inLastForget is read and written under r.mu only: by the seam, which runs under it,
	// and before the Forget below takes it.
	inLastForget := true
	r.registryUpdated = func() {
		if !inLastForget {
			return
		}
		inLastForget = false
		if r.mu.TryLock() {
			r.mu.Unlock()
			t.Error("the last Forget updated the registry outside r.mu; a Watch there sees the receiver running, returns, and the stop that follows strands its SPI")
		}
		go func() { watched <- r.Watch(0x2222, natTarget(peer, local)) }()
	}
	r.Forget(0x1111, local)
	if err := <-watched; err != nil {
		t.Fatalf("the Watch that raced the last Forget: %v", err)
	}

	if !r.running() {
		t.Fatal("the receiver is stopped while an SPI is watched; the Watch that raced the last Forget was stranded")
	}
	if _, ok := r.reg.target(0x2222, local); !ok {
		t.Fatal("the Watch that raced the last Forget left its SPI unwatched")
	}
	if r.reg.watching(0x1111) {
		t.Error("the forgotten SPI is still watched")
	}
	if _, err := netlink.LinkByName(espFormTUNName); err != nil {
		t.Errorf("no %s device while an SPI is watched: %v", espFormTUNName, err)
	}
}

// VALIDATES: a fast path that cannot load never fails a Watch. In a user namespace the
// kernel refuses the tc program (loading one needs CAP_BPF and CAP_NET_ADMIN in the
// initial user namespace), so Watch still opens the complete path and answers nil, and
// the fast path reports Unavailable with a cause that names the failure and not the fix,
// which the doctor check writes itself; the last Forget returns it to Idle.
// PREVENTS: an SA refused, or a silent fast path, on a host without BPF. The complete path
// serves every bare datagram there, so refusing the SA would lose a working tunnel, and an
// Unavailable state with no cause would leave `ze doctor` nothing to tell the operator.
func TestESPFormReceiverServesWhenTheFastPathCannotLoad(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	peer := netip.MustParseAddr("198.51.100.7")
	local := netip.MustParseAddr("203.0.113.9")
	r := newESPFormReceiver(slogutil.DiscardLogger())
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	if err := r.Watch(0x1111, natTarget(peer, local)); err != nil {
		t.Fatalf("a fast path that cannot load failed the Watch: %v", err)
	}
	if !r.running() {
		t.Fatal("the complete path did not start")
	}
	st, err := r.fastPathState()
	if err != nil {
		t.Fatalf("read the fast path state: %v", err)
	}
	if st.Status != ESPFormFastPathUnavailable {
		t.Fatalf("fast path status %s in a user namespace, want %s: the tc program cannot load there", st.Status, ESPFormFastPathUnavailable)
	}
	if st.Cause == "" {
		t.Error("the Unavailable state carries no cause, so ze doctor cannot say why the fast path did not load")
	}
	if strings.Contains(st.Cause, privilege.DaemonCapabilitiesFix) {
		t.Errorf("the Unavailable cause %q carries the fix %q; the doctor check writes its own fix, so the operator would read it twice", st.Cause, privilege.DaemonCapabilitiesFix)
	}

	r.Forget(0x1111, local)
	st, err = r.fastPathState()
	if err != nil {
		t.Fatalf("read the fast path state after the last Forget: %v", err)
	}
	if st.Status != ESPFormFastPathIdle {
		t.Errorf("fast path status %s after the last Forget, want %s", st.Status, ESPFormFastPathIdle)
	}
}

// TestESPFormReceiverReportsSharedSPI proves the receiver's state carries the collision
// of two Child SAs on one inbound SPI from the Watch of the second SA to the Forget of
// either, through Watch, Forget and fastPathState, the calls the backend and the doctor
// check make.
//
// VALIDATES: one Watch reports no shared SPI; the second Watch of the SPI on another
// local address reports one, naming the first SA's address as served and the second's
// as unserved; forgetting the unserved SA clears it; watching it again and forgetting
// the served SA clears it too, the remaining SA now holding the fast path. It runs in a
// user namespace, where the fast path cannot load, because the record is read under
// every status.
// PREVENTS: a collision the dataplane logs once at Watch and the doctor never reports.
// MUTATION: in fastPathState, drop the SharedSPIs assignment: the second Watch records
// nothing, so the check after it is red.
func TestESPFormReceiverReportsSharedSPI(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	const spi = 0x2222
	peer := netip.MustParseAddr("198.51.100.7")
	localA := netip.MustParseAddr("203.0.113.9")
	localB := netip.MustParseAddr("203.0.113.10")
	r := newESPFormReceiver(slogutil.DiscardLogger())
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	check := func(when string, want []ESPFormSharedSPI) {
		t.Helper()
		st, err := r.fastPathState()
		if err != nil {
			t.Fatalf("%s: read the fast path state: %v", when, err)
		}
		if !slices.Equal(st.SharedSPIs, want) {
			t.Errorf("%s: SharedSPIs = %+v, want %+v", when, st.SharedSPIs, want)
		}
	}

	if err := r.Watch(spi, natTarget(peer, localA)); err != nil {
		t.Fatalf("watch the first SA: %v", err)
	}
	check("after the first Watch", nil)
	if err := r.Watch(spi, natTarget(peer, localB)); err != nil {
		t.Fatalf("watch the second SA on the SPI: %v", err)
	}
	check("after the second Watch", []ESPFormSharedSPI{{SPI: spi, Served: localA, Unserved: localB}})

	r.Forget(spi, localB)
	check("after the unserved SA was forgotten", nil)

	if err := r.Watch(spi, natTarget(peer, localB)); err != nil {
		t.Fatalf("watch the second SA again: %v", err)
	}
	check("after the second SA was watched again", []ESPFormSharedSPI{{SPI: spi, Served: localA, Unserved: localB}})
	r.Forget(spi, localA)
	check("after the served SA was forgotten", nil)
}

// espFormWarnings is a slog handler that records each record at Warn or above, for a
// test that reads what the receiver warned of. Safe for concurrent use: the fast path's
// worker logs from its own goroutine.
type espFormWarnings struct {
	mu      sync.Mutex
	records []slog.Record
}

func (w *espFormWarnings) Enabled(context.Context, slog.Level) bool { return true }

func (w *espFormWarnings) Handle(_ context.Context, record slog.Record) error { //nolint:gocritic // slog.Handler takes Record by value.
	if record.Level < slog.LevelWarn {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records = append(w.records, record.Clone())
	return nil
}

func (w *espFormWarnings) WithAttrs([]slog.Attr) slog.Handler { return w }

func (w *espFormWarnings) WithGroup(string) slog.Handler { return w }

// attrs answers the attributes of each recorded warning whose message starts with
// prefix, keyed by attribute name, one map per warning.
func (w *espFormWarnings) attrs(prefix string) []map[string]string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []map[string]string
	for i := range w.records {
		if !strings.HasPrefix(w.records[i].Message, prefix) {
			continue
		}
		attrs := make(map[string]string)
		w.records[i].Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value.String()
			return true
		})
		out = append(out, attrs)
	}
	return out
}

// espFormSharedSPIWarning opens the warning Watch logs for two SAs on one inbound SPI.
const espFormSharedSPIWarning = "esp-form: two Child SAs carry one inbound SPI"

// TestESPFormReceiverWarnsOfNoSharedSPIWhileUnavailable proves Watch does not warn of
// two SAs on one inbound SPI while the fast path is not attached, because no SA is then
// served by the fast path and the collision costs nothing more; the status has its own
// warning. TestESPFormFastPathWarnsOfASharedSPIWhileAttached is the positive control,
// under a fast path that loaded.
//
// METHOD: in a user namespace the fast path cannot load, so it is Unavailable; two SAs
// are watched on one SPI on two local addresses, and the receiver's warnings are read.
// MUTATION: in espFormReceiver.Watch, drop the test that the fast path is attached: the
// second Watch logs the collision warning and this test turns red.
func TestESPFormReceiverWarnsOfNoSharedSPIWhileUnavailable(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	warnings := &espFormWarnings{}
	r := newESPFormReceiver(slog.New(warnings))
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	const spi = 0x4444
	peer := netip.MustParseAddr("198.51.100.7")
	if err := r.Watch(spi, natTarget(peer, netip.MustParseAddr("203.0.113.9"))); err != nil {
		t.Fatalf("watch the first SA: %v", err)
	}
	if err := r.Watch(spi, natTarget(peer, netip.MustParseAddr("203.0.113.10"))); err != nil {
		t.Fatalf("watch the second SA on the SPI: %v", err)
	}
	if status := r.fast.currentStatus(); status != ESPFormFastPathUnavailable {
		t.Fatalf("fast path status %s in a user namespace, want %s", status, ESPFormFastPathUnavailable)
	}
	if got := warnings.attrs(espFormSharedSPIWarning); len(got) != 0 {
		t.Errorf("Watch warned of the shared SPI %d times under %s, want none: %v", len(got), ESPFormFastPathUnavailable, got)
	}
}

// TestESPFormRegistryRetargetRefusesATakenAddress drives the registry's one check of a
// MOBIKE move onto the local address another SA on the same inbound SPI holds, through
// retarget and admitsMove, with no receiver around it. It lives in a Linux file because
// retarget does: only the Linux XFRM backend moves an SA.
//
// VALIDATES: retarget refuses the move onto the other SA's address with
// errESPFormLocalTaken and changes neither SA; admitsMove gives the same answer, also for
// an SA that is not watched (the caller would watch it there), answers nil for a free
// address, and changes nothing; retarget of an SA not watched answers
// errESPFormNotWatched; a move onto a free address carries the SA there and leaves the
// other SA; a port change on the SA's own address is no collision.
// PREVENTS: a move that replaces another SA's target, which leaves that SA's bare ESP
// unserved while its tunnel is up.
// MUTATION: in espFormRegistry.moveSlotLocked, drop the test for another SA on the new
// local address: retarget answers nil for the move onto localB and the SA on localA
// replaces nothing but its own entry, so two entries hold localB and the first check is
// red.
// MUTATION: in espFormRegistry.retarget, drop the at < 0 test: the retarget of the SA
// that is not watched writes at index -1, which panics, and the test fails.
func TestESPFormRegistryRetargetRefusesATakenAddress(t *testing.T) {
	const spi = 0x3333
	peerA, localA := netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1")
	peerB, localB := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")
	free := netip.MustParseAddr("203.0.113.1")
	var reg espFormRegistry
	reg.watch(spi, natTarget(peerA, localA))
	reg.watch(spi, natTarget(peerB, localB))
	wantA := espFormTarget{peer: peerA, local: localA, peerPort: espFormUDPPort, localPort: espFormUDPPort}
	wantB := espFormTarget{peer: peerB, local: localB, peerPort: espFormUDPPort, localPort: espFormUDPPort}
	unchanged := func(when string) {
		t.Helper()
		if got := reg.watched[spi]; !slices.Equal(got, []espFormTarget{wantA, wantB}) {
			t.Errorf("%s: the SPI's SAs are %+v, want %+v unchanged", when, got, []espFormTarget{wantA, wantB})
		}
	}

	onto := espFormTarget{peer: netip.MustParseAddr("203.0.113.2"), local: localB, peerPort: 62001, localPort: 4500}
	if err := reg.retarget(spi, localA, onto); !errors.Is(err, errESPFormLocalTaken) {
		t.Errorf("retarget onto the other SA's address answered %v, want %v", err, errESPFormLocalTaken)
	}
	unchanged("after the refused retarget")

	for _, tc := range []struct {
		name     string
		from, to netip.Addr
		want     error
	}{
		{"watched SA onto the other SA's address", localA, localB, errESPFormLocalTaken},
		{"unwatched SA onto the other SA's address", free, localB, errESPFormLocalTaken},
		{"watched SA onto a free address", localA, free, nil},
	} {
		if err := reg.admitsMove(spi, tc.from, tc.to); !errors.Is(err, tc.want) {
			t.Errorf("admitsMove, %s, answered %v, want %v", tc.name, err, tc.want)
		}
	}
	unchanged("after admitsMove")

	if err := reg.retarget(spi, free, espFormTarget{peer: peerA, local: free}); !errors.Is(err, errESPFormNotWatched) {
		t.Errorf("retarget of an SA not watched answered %v, want %v", err, errESPFormNotWatched)
	}
	unchanged("after the retarget of an SA not watched")

	ports := espFormTarget{peer: peerB, local: localB, peerPort: 62002, localPort: 4500}
	if err := reg.retarget(spi, localB, ports); err != nil {
		t.Errorf("a port change on the SA's own address was refused: %v", err)
	}
	moved := espFormTarget{peer: netip.MustParseAddr("203.0.113.2"), local: free, peerPort: 62001, localPort: 4500}
	if err := reg.retarget(spi, localA, moved); err != nil {
		t.Fatalf("a move onto a free address was refused: %v", err)
	}
	if got := reg.watched[spi]; !slices.Equal(got, []espFormTarget{moved, ports}) {
		t.Errorf("after the moves the SPI's SAs are %+v, want %+v", got, []espFormTarget{moved, ports})
	}
}

// VALIDATES: espFormBypassInboundPolicy refuses a connection it cannot exempt, rather than
// reporting success.
// PREVENTS: the silent half of the defect this exemption fixes. A caller that reads a nil
// error believes the reader is exempt from the inbound IPsec policy check. If it is not,
// the kernel drops every datagram the reader was opened to recover and the SA carries one
// ESP wire form only, with no error anywhere (ai/rules/evidence.md). The exemption is
// therefore reported, and openESPFormPaths closes the socket and fails when it cannot be
// made. A socket that is not an IP socket can no longer reach it: it takes *net.IPConn, so
// the compiler refuses one. What remains reachable is an IP socket the kernel cannot set
// the policy on, here one that holds no descriptor, whose SyscallConn answers EINVAL.
//
// MUTATION: in espFormBypassInboundPolicy, answer nil when SyscallConn fails: the socket
// is reported exempt.
func TestESPFormBypassRefusesASocketItCannotExempt(t *testing.T) {
	var conn net.IPConn
	err := espFormBypassInboundPolicy(&conn)
	if err == nil {
		t.Fatal("an IP socket with no descriptor was reported as exempt from the inbound IPsec policy check")
	}
	if _, ok := errors.AsType[*opererr.Error](err); !ok {
		t.Errorf("the refusal %v is not an operator error", err)
	}
}

// VALIDATES: the ESP form reader names its raw-socket protocol by number, and that
// number is ProtoESP (50).
// PREVENTS: "ip4:esp" coming back. Go resolves a protocol name through /etc/protocols,
// falling back only to icmp, igmp, tcp, udp and ipv6-icmp, so on Ze's gokrazy image (no
// /etc/protocols) the name failed with "unknown IP protocol specified" and every Child SA
// asking for both ESP forms refused to install.
func TestESPFormListenNetworkIsNumeric(t *testing.T) {
	family, proto, found := strings.Cut(espFormListenNetwork, ":")
	if !found {
		t.Fatalf("espFormListenNetwork %q has no protocol part", espFormListenNetwork)
	}
	if family != "ip4" {
		t.Errorf("espFormListenNetwork family %q, want ip4", family)
	}
	number, err := strconv.Atoi(proto)
	if err != nil {
		t.Fatalf("espFormListenNetwork protocol %q is a name, which needs /etc/protocols: %v", proto, err)
	}
	if number != int(ProtoESP) {
		t.Errorf("espFormListenNetwork protocol %d, want ProtoESP (%d)", number, ProtoESP)
	}
}

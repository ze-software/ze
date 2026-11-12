// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Overview: espform_state.go -- the state the fast path reports, and its doctor codes
// Related: espform_program_linux.go -- the rewrite program and the maps this file loads and attaches
// Related: espform_tun_linux.go -- ze-espform, the device the fast path never attaches to
// Related: espform_fastpath_gate_linux.go -- the firewall gate this lifecycle opens and shuts
// RFC: rfc/short/rfc7296.md -- receive both ESP forms at any time (Section 2.23)

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The fast path's attach lifecycle. While any SPI is watched, the rewrite program
// (espform_program_linux.go) is attached at the head of every link's TCX ingress
// that has a variant for its L2 header, and a netlink link subscription attaches it
// to links created later. A link the fast path does not serve, and every link while
// the program cannot load, still has its bare ESP served by the complete path: the
// raw ESP reader re-presents it through ze-espform, at most espFormRate datagrams
// per second. No failure here ever refuses an SA.
//
// The program rewrites only while its gate is open, and the gate opens only while no
// nftables rule on the input path can drop ESP (espform_fastpath_gate_linux.go): a
// rewritten datagram passes every netfilter hook as UDP, so such a rule would never
// see it. With the gate shut the complete path serves the bare ESP, after the firewall
// decided on it.

// espFormLinkUpdatesDepth bounds the channel between the netlink subscription and the
// worker. When it is full the library's reader blocks and the kernel's socket buffer
// holds the next messages; an overrun of that buffer ends the subscription, which the
// worker records in the Cause it reports.
const espFormLinkUpdatesDepth = 64

// espFormTCXProbeIfindex is an interface index no device holds, so a kernel that
// attaches TCX programs answers ENODEV to the probe's attach and attaches nothing.
const espFormTCXProbeIfindex = math.MaxInt32

// espFormLinkChoice is what the fast path does with one link.
type espFormLinkChoice uint8

const (
	// espFormLinkUnspecified is the zero value: no choice was made.
	espFormLinkUnspecified espFormLinkChoice = iota
	// espFormLinkEthernet attaches the program built for a 14-octet Ethernet header.
	espFormLinkEthernet
	// espFormLinkL3 attaches the program built for a packet that starts with the IP
	// header.
	espFormLinkL3
	// espFormLinkSkipped leaves the link alone and reports nothing: lo, or ze-espform.
	espFormLinkSkipped
	// espFormLinkRefused leaves the link alone and reports it: a link type the
	// program has no variant for.
	espFormLinkRefused
)

// espFormAttachedLink is one link the program is attached to.
type espFormAttachedLink struct {
	name string
	tcx  link.Link
}

// espFormFastPath owns the fast path: the loaded collection, its TCX links, its
// firewall gate, and the one worker that follows link creation and judges the
// nftables ruleset.
//
// Lifecycle: newESPFormFastPath, then start on the first watched SPI, set and remove
// as SPIs come, move and go, and stop on the last forget and on close. start after
// stop loads afresh. start never fails its caller: a failure leaves the status
// Unavailable, holding nothing, and the complete path serves every bare datagram.
// hold may be called at any time, from any goroutine, and its release MUST be called
// once after it. Safe for concurrent use.
//
// Lock order: life, then mu. start and stop hold life for their whole run, so they
// never overlap. mu guards every other field, and set, remove, state, hold, its
// release and the worker each take it for one operation. hold and its release take mu
// alone, never life: the firewall calls them while it holds its own reconcile lock, so
// that lock comes before mu, and nothing that holds mu or life calls the firewall. The
// worker reads the ruleset holding no lock, so state never waits on a dump. stop
// closes done and waits for the worker while it holds life alone, never mu, so the
// worker can always finish the update or the judgment it holds and stop cannot
// deadlock.
type espFormFastPath struct {
	log *slog.Logger

	// kick asks the worker to judge the ruleset again. Its depth of one coalesces the
	// asks a release makes while a judgment is pending. Made by newESPFormFastPath and
	// never replaced, so it is read without mu.
	kick chan struct{}

	life sync.Mutex

	mu       sync.Mutex
	status   ESPFormFastPathStatus
	cause    string
	coll     *ebpf.Collection
	ethernet *ebpf.Program
	l3       *ebpf.Program
	gate     *ebpf.Map
	watched4 *ebpf.Map
	watched6 *ebpf.Map
	counter  *ebpf.Map
	links    map[int]espFormAttachedLink // by interface index
	refused  map[int]ESPFormLinkRefusal  // by interface index
	done     chan struct{}               // closed by stop to end the subscription
	exited   chan struct{}               // closed by the worker as it returns

	// The firewall gate (espform_fastpath_gate_linux.go). holds and epoch outlive
	// stop: a hold taken while the fast path is idle still keeps the gate of a start
	// during it shut.
	holds    int    // holds not yet released
	epoch    uint64 // steps on every hold, release and ruleset commit
	firewall espFormFirewallView
	// firewallLost is set while the ruleset cannot be read or watched: the gate then
	// stays shut. A reader or watch that start could not open leaves it set until
	// stop; a watch that failed later leaves it set until the worker watches again.
	firewallLost bool
	// gateOpen is the last value the gate took: set by a write of 1 the map
	// accepted, cleared by a write of 0 it accepted. A failed write leaves it.
	gateOpen        bool
	firewallBlocked bool
	firewallSite    string
	// firewallAcceptFix and firewallAcceptHow are what the operator changes at
	// firewallSite (espFormFirewallAccept), set with it, empty unless
	// firewallBlocked.
	firewallAcceptFix string
	firewallAcceptHow string
	firewallCause     string
	// firewallReading is how the ruleset is read, set at each transition of the gate's
	// judgment (espform_fastpath_gate_linux.go) and by start and stop: Unheld while no
	// program is loaded.
	firewallReading ESPFormFirewallReading
	// rulesetMoving is set from the first judgment of a stream of commits that ended
	// Moving (firewallMoving) until a judgment publishes or a shut records its own
	// reading (endMovingLocked); firewallMovingSince is when that first judgment ended,
	// zero while rulesetMoving is clear. state reports the pair as the Moving reading.
	// It overlays firewallReading and the reason fields, which keep the last judgment
	// that settled, so blockGateLocked and shutGateLocked log a site or a cause once,
	// however many streams of commits pass between two judgments that find it.
	rulesetMoving       bool
	firewallMovingSince time.Time

	// Seams, nil outside tests: judge replaces espFormFirewallBlocks, and writeGate
	// replaces the write into the gate map.
	judge     func(espFormNFTReader) (bool, espFormFirewallSite, error)
	writeGate func(value uint32) error
}

// newESPFormFastPath answers an idle fast path that logs to log.
func newESPFormFastPath(log *slog.Logger) *espFormFastPath {
	return &espFormFastPath{
		log: log, status: ESPFormFastPathIdle, firewallReading: ESPFormFirewallUnheld,
		kick: make(chan struct{}, 1),
	}
}

// logger reports the fast path's logger, and a usable one when none was given.
func (f *espFormFastPath) logger() *slog.Logger {
	if f.log == nil {
		return slogutil.Logger("ike.dataplane")
	}
	return f.log
}

// start loads the program, subscribes to link updates with the existing links
// listed first, so the initial dump and later links arrive through one channel,
// opens the nftables reader and watch the gate is judged through, and starts the
// worker that attaches to the links and judges the ruleset, first at once and then on
// every commit the watch signals. The gate stays shut until that first judgment
// finds the ruleset safe. A ruleset that cannot be read or watched leaves the gate
// shut and the links attached, which the state reports. A fast path already attached,
// unavailable or detached is left as it is: only stop returns it to Idle.
func (f *espFormFastPath) start() {
	f.life.Lock()
	defer f.life.Unlock()

	f.mu.Lock()
	status := f.status
	f.mu.Unlock()
	if status != ESPFormFastPathIdle {
		return
	}

	coll, err := loadESPFormFastPathCollection()
	if err != nil {
		f.unavailable(err)
		return
	}

	ns, err := netns.Get()
	if err != nil {
		coll.Close()
		f.unavailable(fmt.Errorf("read the network namespace to follow links in: %w", err))
		return
	}

	updates := make(chan netlink.LinkUpdate, espFormLinkUpdatesDepth)
	done := make(chan struct{})
	err = netlink.LinkSubscribeWithOptions(updates, done, netlink.LinkSubscribeOptions{
		ErrorCallback: f.subscriptionError(done),
		ListExisting:  true,
	})
	if err != nil {
		// The library may already wait on done to close its socket: release it.
		close(done)
		ns.Close() //nolint:errcheck // the subscription already failed; that error is the one reported
		coll.Close()
		f.unavailable(fmt.Errorf("subscribe to link updates: %w", err))
		return
	}
	exited := make(chan struct{})
	firewall, firewallErr := openESPFormFirewallView(ns)

	f.mu.Lock()
	f.status, f.cause = ESPFormFastPathAttached, ""
	f.coll = coll
	f.ethernet = coll.Programs[espFormFastPathEthernetProg]
	f.l3 = coll.Programs[espFormFastPathL3Prog]
	f.gate = coll.Maps[espFormFastPathGateMap]
	f.watched4 = coll.Maps[espFormFastPathWatched4Map]
	f.watched6 = coll.Maps[espFormFastPathWatched6Map]
	f.counter = coll.Maps[espFormFastPathCounterMap]
	f.links = make(map[int]espFormAttachedLink)
	f.refused = make(map[int]ESPFormLinkRefusal)
	f.done, f.exited = done, exited
	f.adoptFirewallLocked(firewall, firewallErr)
	f.mu.Unlock()

	go f.follow(updates, done, ns, firewall.events, exited)
}

// loadESPFormFastPathCollection removes the memlock limit, loads the collection and
// asks the kernel whether it attaches TCX programs, so a kernel without TCX is
// Unavailable at once rather than a Refused row for every link.
func loadESPFormFastPathCollection() (*ebpf.Collection, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("remove the memlock limit: %w", err)
	}
	coll, err := ebpf.NewCollection(espFormFastPathSpec())
	if err != nil {
		return nil, fmt.Errorf("load the rewrite program: %w", err)
	}
	if err := probeESPFormTCX(coll.Programs[espFormFastPathL3Prog]); err != nil {
		coll.Close()
		return nil, err
	}
	return coll, nil
}

// probeESPFormTCX attaches program to an interface index no device holds, the way
// trafficusage's probeTCX does. A kernel that parsed the TCX request answers ENODEV;
// one without TCX makes cilium/ebpf answer ebpf.ErrNotSupported (AttachTCX runs its
// haveTCX feature probe on a failed attach). Nothing stays attached.
func probeESPFormTCX(program *ebpf.Program) error {
	probe, err := link.AttachTCX(link.TCXOptions{
		Interface: espFormTCXProbeIfindex,
		Program:   program,
		Attach:    ebpf.AttachTCXIngress,
		Anchor:    link.Head(),
	})
	switch {
	case err == nil:
		// No device holds the index; release whatever a kernel attached.
		if closeErr := probe.Close(); closeErr != nil {
			return fmt.Errorf("release the TCX probe link: %w", closeErr)
		}
		return nil
	case errors.Is(err, ebpf.ErrNotSupported):
		return fmt.Errorf("this kernel does not attach TCX programs: %w", err)
	case errors.Is(err, unix.ENODEV):
		return nil
	default:
		return fmt.Errorf("probe the kernel's TCX attach: %w", err)
	}
}

// unavailable records a load or subscribe failure. Nothing is held, so the complete
// path serves every bare datagram. The recorded cause is the failure alone, so the
// doctor check's own fix does not reach the operator twice; the warning names the
// command that grants the capabilities beside it, as the doctor check does. A kernel
// without TCX (before Linux 6.6) says so in the cause.
func (f *espFormFastPath) unavailable(err error) {
	cause := err.Error()
	f.mu.Lock()
	f.status, f.cause = ESPFormFastPathUnavailable, cause
	f.mu.Unlock()
	f.logger().Warn("esp-form: the fast path did not load, so all bare ESP is re-presented through "+
		espFormTUNName+" by the complete path, rate-bounded",
		"cause", cause, "datagrams-per-second-max", espFormRate,
		"fix", privilege.DaemonCapabilitiesFix,
		"code", diagnostic.CodeDoctorIPsecESPFormFastPath)
}

// subscriptionError is the subscription's error callback. The library calls it from
// its own goroutine. Once stop closed done, the failed receive is the subscription
// ending as asked, and it is not logged.
func (f *espFormFastPath) subscriptionError(done <-chan struct{}) func(error) {
	return func(err error) {
		select {
		case <-done:
			return
		default:
		}
		f.logger().Warn("esp-form: the fast path's link subscription reported an error, "+
			"so a link created now may be served only by the complete path through "+espFormTUNName,
			"error", err, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
	}
}

// follow is the fast path's one worker. It runs in the network namespace start was
// called in: the kernel resolves a TCX attach's interface index in the calling
// thread's namespace (kernel/bpf/tcx.c, tcx_link_attach, v7.0: "struct net *net =
// current->nsproxy->net_ns;"), the indexes come from a subscription opened in start's,
// and the nftables reader's sockets were opened in start's too. The worker keeps ns
// open while it serves and watches the ruleset of ns again with it after a failure,
// naming the namespace rather than trusting the thread's. It serves until the library
// closes updates after stop closed done.
//
// Should the thread fail to enter the namespace, nothing is attached and the gate is
// never opened: the worker only drains updates until stop.
func (f *espFormFastPath) follow(updates <-chan netlink.LinkUpdate, done <-chan struct{}, ns netns.NsHandle, events <-chan struct{}, exited chan<- struct{}) {
	defer close(exited)
	defer ns.Close() //nolint:errcheck // the worker is done with the handle; the namespace goes with its last reference

	// The thread is never unlocked: when this goroutine returns the runtime retires
	// it, so a thread whose namespace this changed never runs another goroutine.
	runtime.LockOSThread()
	if err := netns.Set(ns); err != nil {
		f.followEnded(fmt.Errorf("enter the network namespace to attach in: %w", err))
		for range updates {
			// Drain until stop, so the library's reader never blocks.
		}
		return
	}
	f.serve(updates, events, done, espFormRewatcher{open: espFormRewatchIn(ns), after: time.After, now: time.Now})
}

// serve is the worker's loop: it judges the ruleset once, then applies each link
// update, judges again on each commit the watch signals and on each kick a release
// sends. Each commit the watch signals shuts the gate before anything else
// (firewallCommitted). A judgment that kept seeing the ruleset move arms the next one
// after a wait (judgeWatched); while it is armed, a commit only shuts the gate and a
// kick does nothing, because the armed judgment begins after both, and link updates
// are applied as ever. A watch that ends on a failure is opened again through rewatch,
// after a wait that doubles from espFormNFTRewatchFirst to espFormNFTRewatchMax while
// attempts fail, and each watch that opens is followed by a judgment that begins
// after it.
// It returns once the library has closed updates and stop has closed done:
// stop closes done, the library closes its netlink socket, its receive fails, and it
// closes the channel. Reading updates to the end keeps the library's reader from
// blocking on a send. A subscription that ends before stop leaves the worker judging
// the ruleset until stop, because the links already attached still rewrite.
//
// The loop has no other bound: it lives as long as the fast path is attached.
func (f *espFormFastPath) serve(updates <-chan netlink.LinkUpdate, events, done <-chan struct{}, rewatch espFormRewatcher) {
	watch := espFormWatchLoop{rewatch: rewatch, events: events}
	f.judgeWatched(&watch, done)
	var stopped <-chan struct{} // done, armed once updates closed before stop
	for {
		select {
		case update, ok := <-updates:
			if ok {
				f.apply(update)
				continue
			}
			updates = nil
			select {
			case <-done:
				return
			default:
			}
			f.followEnded(errors.New("the link subscription ended"))
			stopped = done
		case <-stopped:
			return
		case _, ok := <-watch.events:
			if !ok {
				f.watchLost(&watch)
				continue
			}
			f.firewallCommitted()
			if watch.rejudge != nil {
				// The ruleset keeps moving: the armed judgment begins after this commit.
				continue
			}
			f.judgeWatched(&watch, done)
		case <-watch.rejudge:
			f.judgeWatched(&watch, done)
		case <-watch.retry:
			f.watchAgain(&watch, done)
		case <-f.kick:
			if watch.rejudge != nil {
				// The armed judgment begins after the release that kicked.
				continue
			}
			f.judgeWatched(&watch, done)
		}
	}
}

// followEnded records that links created from now on are not attached. The links
// already attached stay attached; the next start, after stop, follows links again.
//
// The cause changes only while Attached. A fast path that detached after a refused
// gate shut attaches no link, new or old, so the subscription's end changes nothing it
// serves, and its cause keeps the refused write the doctor check reports for Detached.
func (f *espFormFastPath) followEnded(err error) {
	cause := err.Error() + "; a link created from now on is served by the complete path alone " +
		"until no Child SA is served in both ESP forms and one is installed again"
	f.mu.Lock()
	if f.status == ESPFormFastPathAttached {
		f.cause = cause
	}
	f.mu.Unlock()
	f.logger().Warn("esp-form: the fast path stopped following link creation",
		"cause", cause, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
}

// apply handles one link update under mu.
func (f *espFormFastPath) apply(update netlink.LinkUpdate) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// refuse-by-default: guards detachLocked -- a fast path whose gate could not shut detached from every link, and attaching a new one would rewrite past the firewall again
	if f.status != ESPFormFastPathAttached {
		return
	}
	ifindex := int(update.IfInfomsg.Index)
	if update.Header.Type == unix.RTM_DELLINK {
		f.linkRemoved(ifindex)
		return
	}
	if update.Header.Type != unix.RTM_NEWLINK {
		return
	}
	attrs := update.Link.Attrs()
	f.linkAdded(ifindex, attrs.Name, update.IfInfomsg.Type, attrs.EncapType, attrs.Flags)
}

// linkAdded attaches the program to a link the fast path has not met. A link it
// already serves or refused only takes its new name: a rename keeps the index and
// the TCX link, and a refusal was logged once, when the link appeared. MUST be
// called with mu held.
func (f *espFormFastPath) linkAdded(ifindex int, name string, linkType uint16, encapType string, flags net.Flags) {
	if attached, ok := f.links[ifindex]; ok {
		attached.name = name
		f.links[ifindex] = attached
		return
	}
	if refusal, ok := f.refused[ifindex]; ok {
		refusal.Link = name
		f.refused[ifindex] = refusal
		return
	}

	choice, reason := espFormLinkVariant(name, linkType, encapType, flags)
	var program *ebpf.Program
	switch choice {
	case espFormLinkEthernet:
		program = f.ethernet
	case espFormLinkL3:
		program = f.l3
	case espFormLinkSkipped:
		return
	case espFormLinkRefused:
		f.refuse(ifindex, name, reason)
		return
	case espFormLinkUnspecified:
		panic("BUG: espFormLinkVariant made no choice for link " + name)
	}

	tcx, err := link.AttachTCX(link.TCXOptions{
		Interface: ifindex,
		Program:   program,
		Attach:    ebpf.AttachTCXIngress,
		// tcx_run stops at the first program that answers anything but TCX_NEXT
		// (net/core/dev.c). trafficusage's counter answers TCX_NEXT
		// (internal/plugins/trafficusage/program_linux.go, tcxNext), but any other
		// TCX program ahead of the rewrite that answers TCX_PASS or a drop would keep
		// it from running. At the head the rewrite runs first, whatever else is
		// attached, and answers TCX_NEXT, so every program behind it still sees the
		// packet, rewritten, and so do the legacy tc filters after them.
		Anchor: link.Head(),
	})
	if err != nil {
		f.refuse(ifindex, name, "the kernel refused the attach: "+err.Error())
		return
	}
	f.links[ifindex] = espFormAttachedLink{name: name, tcx: tcx}
}

// refuse records a link the fast path does not serve and logs it once. MUST be
// called with mu held.
func (f *espFormFastPath) refuse(ifindex int, name, reason string) {
	f.refused[ifindex] = ESPFormLinkRefusal{Link: name, Reason: reason}
	f.logger().Warn("esp-form: the fast path does not serve link "+name+
		", so its bare ESP is re-presented through "+espFormTUNName+" by the complete path, rate-bounded",
		"link", name, "reason", reason, "code", diagnostic.CodeDoctorIPsecESPFormLink)
}

// linkRemoved forgets a deleted link. The kernel detached the program with the
// device; closing the TCX link releases the link object. MUST be called with mu held.
func (f *espFormFastPath) linkRemoved(ifindex int) {
	delete(f.refused, ifindex)
	attached, ok := f.links[ifindex]
	if !ok {
		return
	}
	delete(f.links, ifindex)
	if err := attached.tcx.Close(); err != nil {
		f.logger().Warn("esp-form: release the fast path's TCX link of a deleted interface",
			"link", attached.name, "error", err, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
	}
}

// espFormLinkVariant chooses the program for a link from its hardware type, the
// ifi_type the kernel reports (an ARPHRD_ value). The type is read as the number:
// the netlink library has no name for ARPHRD_RAWIP and renders it "unknown519", so
// encapType serves only the refusal's text.
//
// The program's L2 offset is skb->mac_len, which tcx_run pushes back before it runs
// the program ("__skb_push(skb, skb->mac_len);"). __netif_receive_skb_core sets the
// network header at skb->data and mac_len to the octets between the MAC header and
// it ("skb_reset_network_header(skb);", then "skb_reset_mac_len(skb);",
// net/core/dev.c, v7.0), so mac_len is what the driver pulled after it set the MAC
// header:
//
//   - ARPHRD_ETHER: eth_type_trans calls "skb_reset_mac_header(skb);" and then
//     eth_skb_pull_mac, which pulls the Ethernet header (net/ethernet/eth.c): 14.
//   - ARPHRD_NONE, a TUN device in IFF_TUN mode ("dev->type = ARPHRD_NONE;"):
//     tun_get_user calls "skb_reset_mac_header(skb);" on the IP header and pulls
//     nothing (drivers/net/tun.c): 0.
//   - ARPHRD_PPP: ppp_receive_nonmp_frame pulls the two-octet protocol field with
//     "skb_pull_rcsum(skb, 2);" and then calls "skb_reset_mac_header(skb);"
//     (drivers/net/ppp/ppp_generic.c): 0.
//   - ARPHRD_RAWIP, such as rmnet ("rmnet_dev->type = ARPHRD_RAWIP;"):
//     rmnet_deliver_skb calls "skb_reset_network_header(skb);" and
//     "skb_set_mac_header(skb, 0);" (drivers/net/ethernet/qualcomm/rmnet/
//     rmnet_handlers.c): 0.
//
// A wrong choice is safe: the program checks the version, the header length and the
// protocol at the offset it was built for, so on a link whose header sits elsewhere it
// finds no ESP and answers TCX_NEXT with the packet untouched.
//
// lo and ze-espform are skipped, never refused: a packet on lo is local, and one on
// ze-espform is already encapsulated, re-presented by the complete path.
func espFormLinkVariant(name string, linkType uint16, encapType string, flags net.Flags) (espFormLinkChoice, string) {
	if flags&net.FlagLoopback != 0 {
		return espFormLinkSkipped, ""
	}
	if linkType == unix.ARPHRD_LOOPBACK {
		return espFormLinkSkipped, ""
	}
	if name == espFormTUNName {
		return espFormLinkSkipped, ""
	}
	switch linkType {
	case unix.ARPHRD_ETHER:
		return espFormLinkEthernet, ""
	case unix.ARPHRD_NONE, unix.ARPHRD_PPP, unix.ARPHRD_RAWIP:
		return espFormLinkL3, ""
	}
	return espFormLinkRefused, "the fast path has no program for link type " + encapType +
		" (ARPHRD " + strconv.Itoa(int(linkType)) + ")"
}

// set watches spi in the map of its address family, and takes it out of the other
// family's map, so the two maps never hold one SPI twice. No production path moves an
// SPI across families today: InstallSA refuses an IPv6 SA that asks for both ESP
// forms, and validateXFRMMigration refuses a MOBIKE move to an IPv6 address, so every
// SPI set sees is IPv4 until spec-ike-ipv6-transport. It answers nil and
// writes nothing while the fast path is not attached: the complete path serves the
// SPI then, and the caller goes on watching it.
//
// A map update error is returned. The caller MUST log it and keep the SPI watched:
// the complete path still serves it, rate-bounded.
func (f *espFormFastPath) set(spi uint32, peer, local netip.Addr, peerPort, localPort uint16) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.status != ESPFormFastPathAttached {
		return nil
	}

	key := newESPFormFastPathKey(spi)
	if peer.Is4() {
		value, err := newESPFormFastPathValue4(peer, local, peerPort, localPort)
		if err != nil {
			return err
		}
		if err := f.watched4.Update(key, value, ebpf.UpdateAny); err != nil {
			return fmt.Errorf("ESP form fast path: watch SPI %#08x over IPv4: %w", spi, err)
		}
		return espFormFastPathForget(f.watched6, key, spi)
	}
	value, err := newESPFormFastPathValue6(peer, local, peerPort, localPort)
	if err != nil {
		return err
	}
	if err := f.watched6.Update(key, value, ebpf.UpdateAny); err != nil {
		return fmt.Errorf("ESP form fast path: watch SPI %#08x over IPv6: %w", spi, err)
	}
	return espFormFastPathForget(f.watched4, key, spi)
}

// remove takes spi out of both maps. A missing key is not an error. Another
// failure is logged: the program then still rewrites the SPI's bare ESP, which the
// kernel drops for want of a state, as it would have dropped the bare form.
func (f *espFormFastPath) remove(spi uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.status != ESPFormFastPathAttached {
		return
	}

	key := newESPFormFastPathKey(spi)
	for _, watched := range []*ebpf.Map{f.watched4, f.watched6} {
		if err := espFormFastPathForget(watched, key, spi); err != nil {
			f.logger().Warn("esp-form: the fast path could not forget an SPI",
				"error", err, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
		}
	}
}

// espFormFastPathForget deletes key from watched, a missing key included.
func espFormFastPathForget(watched *ebpf.Map, key espFormFastPathKey, spi uint32) error {
	err := watched.Delete(key)
	if err == nil {
		return nil
	}
	if errors.Is(err, ebpf.ErrKeyNotExist) {
		return nil
	}
	return fmt.Errorf("ESP form fast path: forget SPI %#08x in %s: %w", spi, watched.String(), err)
}

// stop shuts the gate, ends the subscription, waits for the worker, shuts the gate
// again, detaches every TCX link, closes the nftables watch and reader and then the
// collection, and returns the fast path to Idle. The program never rewrites while
// nothing watches the ruleset: the links detach before the watch closes, with the
// gate shut, and a rule committed after the watch closed finds no program attached.
// The watch and the reader close only after the worker returned, so no judgment
// reads through a closed reader. Open holds stay counted. Calling it again, or on a
// fast path that never started or was unavailable, does nothing but leave it Idle.
func (f *espFormFastPath) stop() {
	f.life.Lock()
	defer f.life.Unlock()

	f.mu.Lock()
	// The epoch steps with the shut, so a judgment the worker is reading now
	// publishes nothing (publishFirewall). One that reads the stepped epoch before
	// it sees done can still open the gate; the second shut below, once the worker
	// returned, undoes it.
	f.epoch++
	f.writeGateLocked(espFormFastPathGateClosed)
	done, exited := f.done, f.exited
	f.done, f.exited = nil, nil
	f.mu.Unlock()
	if done != nil {
		close(done)
		<-exited
	}

	f.mu.Lock()
	// The worker has returned, so nothing opens the gate from here on.
	f.writeGateLocked(espFormFastPathGateClosed)
	for _, attached := range f.links {
		if err := attached.tcx.Close(); err != nil {
			f.logger().Warn("esp-form: detach the fast path from a link",
				"link", attached.name, "error", err, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
		}
	}
	f.links = nil
	firewall := f.firewall
	f.firewall = espFormFirewallView{}
	f.mu.Unlock()
	f.closeFirewall(firewall)

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.coll != nil {
		f.coll.Close()
	}
	f.status, f.cause = ESPFormFastPathIdle, ""
	f.coll, f.ethernet, f.l3, f.gate = nil, nil, nil, nil
	f.watched4, f.watched6, f.counter = nil, nil, nil
	f.links, f.refused = nil, nil
	f.firewallLost, f.firewallBlocked, f.firewallSite, f.firewallCause = false, false, "", ""
	f.firewallAcceptHow, f.firewallAcceptFix = "", ""
	f.firewallReading = ESPFormFirewallUnheld
	f.endMovingLocked()
	f.gateOpen = false
}

// currentStatus answers the fast path's status alone, without the counters state reads,
// for a caller that only decides what to report from it.
func (f *espFormFastPath) currentStatus() ESPFormFastPathStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

// state answers the status, the cause, the links attached and refused, the firewall
// gate's verdict, and the rewrite counters summed over every CPU. It answers an error
// when the counters cannot be read.
func (f *espFormFastPath) state() (ESPFormFastPathState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	st := ESPFormFastPathState{
		Status:   f.status,
		Cause:    f.cause,
		GateOpen: f.gateOpen,
	}
	if f.rulesetMoving {
		// The reason fields hold the last judgment that settled, which no longer says
		// why the gate is shut: the stream of commits does, so they are left out.
		st.FirewallReading, st.FirewallMovingSince = ESPFormFirewallMoving, f.firewallMovingSince
	} else {
		st.FirewallReading = f.firewallReading
		st.FirewallBlocked, st.FirewallSite, st.FirewallCause = f.firewallBlocked, f.firewallSite, f.firewallCause
		st.FirewallAcceptFix, st.FirewallAcceptHow = f.firewallAcceptFix, f.firewallAcceptHow
	}
	if st.Status != ESPFormFastPathAttached {
		return st, nil
	}
	st.Attached = make([]string, 0, len(f.links))
	for _, attached := range f.links {
		st.Attached = append(st.Attached, attached.name)
	}
	slices.Sort(st.Attached)
	st.Refused = make([]ESPFormLinkRefusal, 0, len(f.refused))
	for _, refusal := range f.refused {
		st.Refused = append(st.Refused, refusal)
	}
	slices.SortFunc(st.Refused, func(a, b ESPFormLinkRefusal) int { return strings.Compare(a.Link, b.Link) })

	var err error
	if st.Rewritten4, err = espFormFastPathCount(f.counter, espFormFastPathSlotIPv4); err != nil {
		return ESPFormFastPathState{}, err
	}
	if st.Rewritten6, err = espFormFastPathCount(f.counter, espFormFastPathSlotIPv6); err != nil {
		return ESPFormFastPathState{}, err
	}
	return st, nil
}

// espFormFastPathCount sums one slot of the per-CPU rewrite counter.
func espFormFastPathCount(counter *ebpf.Map, slot uint32) (uint64, error) {
	var perCPU []uint64
	if err := counter.Lookup(slot, &perCPU); err != nil {
		return 0, fmt.Errorf("ESP form fast path: read rewrite counter slot %d: %w", slot, err)
	}
	var total uint64
	for _, n := range perCPU {
		total += n
	}
	return total, nil
}

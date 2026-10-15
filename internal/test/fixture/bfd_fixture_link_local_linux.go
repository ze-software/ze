// Design: docs/architecture/bfd.md -- "One session per neighbor": the zone reconciled on receive and on send
// RFC: rfc/short/rfc5881.md
// Overview: bfd_fixture_link_local.go -- the frame, the capture reading and the session reading this drives
// Related: register_bfd_linux.go -- registers the three fixtures below
//
// The fixtures behind test/bfd/bfd-first-packet-link-local.ci. They put a BFD
// session to an IPv6 link-local neighbor through a real kernel, on two links
// that carry the SAME neighbor address, which is the case the zone exists for.
//
// Receive: the neighbor is played by raw frames written on the far end of each
// veth, so the kernel hands Ze ordinary ingress on its own end, and recvmsg
// reports the source as fe80::b qualified by Ze's interface. Ze can select a
// session for a packet whose Your Discriminator is zero only when that zone is
// reconciled with the session's interface (reconcileZone in
// internal/component/bfd/engine/engine.go), and it proves the selection by
// reporting the injected My Discriminator as the session's
// remote-discriminator.
//
// Send: each far end is captured with AF_PACKET. Ze's frames for the session on
// one link carry that link's discriminator, so a frame on the wrong link names
// itself. The far end holds no address and a permanent neighbor entry replaces
// Neighbor Discovery, so nothing in either direction is answered through lo.

//go:build linux

package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// bfdLinkLocalAddrGenModeNone is IN6_ADDR_GEN_MODE_NONE from
	// linux/if_link.h, which x/sys/unix does not export. With it the kernel
	// gives neither veth end an EUI-64 link-local, so the only IPv6 address on
	// the links is the one the setup adds, and the far end has none.
	bfdLinkLocalAddrGenModeNone = 1

	// bfdLinkLocalSelectWindow bounds the wait for both sessions to report the
	// injected discriminator.
	bfdLinkLocalSelectWindow = 15 * time.Second

	// bfdLinkLocalCaptureWindow is how long each far end is read once both
	// sessions are selected. A session that is not Up transmits no faster
	// than once a second (RFC 5880 Section 6.8.3), so the window holds
	// several of Ze's packets on each link.
	bfdLinkLocalCaptureWindow = 4 * time.Second

	// bfdLinkLocalInjectInterval paces the injected packets. They repeat
	// because the daemon's sessions may not exist when the first one lands,
	// and because a session whose neighbor falls silent times out and forgets
	// the discriminator the capture is looking for.
	bfdLinkLocalInjectInterval = 200 * time.Millisecond

	// bfdLinkLocalCaptureQueueOctets is each capture's receive queue. About
	// six frames a second reach a far end (five written, one from Ze), each
	// costing a few kilobytes of queue, so 4 MiB holds minutes of a run that
	// lasts under thirty seconds.
	bfdLinkLocalCaptureQueueOctets = 4 << 20
)

// bfdFirstPacketLinkLocalSetup builds the two links before the daemon starts,
// removing any left by an earlier run first, so a run never meets another
// run's MACs or addresses. It is idempotent.
func bfdFirstPacketLinkLocalSetup(ctx context.Context, _ []string) error {
	if err := bfdFirstPacketLinkLocalTeardown(ctx, nil); err != nil {
		return err
	}
	links := bfdLinkLocalLinks()
	for i := range links {
		if err := bfdLinkLocalBuild(&links[i]); err != nil {
			return err
		}
	}
	return nil
}

// bfdFirstPacketLinkLocalTeardown removes both links. Deleting one end of a
// veth removes its peer, and the address and neighbor entry go with the link.
// A link that is already gone is not an error.
func bfdFirstPacketLinkLocalTeardown(context.Context, []string) error {
	for _, link := range bfdLinkLocalLinks() {
		existing, err := netlink.LinkByName(link.near)
		if err != nil {
			if _, notFound := errors.AsType[netlink.LinkNotFoundError](err); notFound {
				continue
			}
			return fmt.Errorf("find %s: %w", link.near, err)
		}
		if err := netlink.LinkDel(existing); err != nil {
			return fmt.Errorf("remove %s: %w", link.near, err)
		}
	}
	return nil
}

// bfdLinkLocalBuild creates one veth pair with the link's MACs, turns off
// kernel link-local generation on both ends, gives Ze's end fe80::a/64 with no
// duplicate address detection (a tentative address cannot source a packet),
// points a permanent neighbor entry for fe80::b at the far end's MAC, and
// brings both ends up.
func bfdLinkLocalBuild(link *bfdLinkLocalLink) error {
	veth := &netlink.Veth{
		Name:             link.near,
		HardwareAddr:     link.nearMAC,
		PeerName:         link.far,
		PeerHardwareAddr: link.farMAC,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("add veth %s/%s: %w", link.near, link.far, err)
	}
	near, err := netlink.LinkByName(link.near)
	if err != nil {
		return fmt.Errorf("find %s: %w", link.near, err)
	}
	far, err := netlink.LinkByName(link.far)
	if err != nil {
		return fmt.Errorf("find %s: %w", link.far, err)
	}
	for _, end := range []netlink.Link{near, far} {
		if err := netlink.LinkSetIP6AddrGenMode(end, bfdLinkLocalAddrGenModeNone); err != nil {
			return fmt.Errorf("addrgenmode none on %s: %w", end.Attrs().Name, err)
		}
	}
	local := &netlink.Addr{
		IPNet: &net.IPNet{IP: net.ParseIP(bfdLinkLocalLocal), Mask: net.CIDRMask(64, 128)},
		Flags: unix.IFA_F_NODAD,
	}
	if err := netlink.AddrAdd(near, local); err != nil && !errors.Is(err, syscall.EEXIST) {
		return fmt.Errorf("address %s/64 on %s: %w", bfdLinkLocalLocal, link.near, err)
	}
	for _, end := range []netlink.Link{near, far} {
		if err := netlink.LinkSetUp(end); err != nil {
			return fmt.Errorf("bring %s up: %w", end.Attrs().Name, err)
		}
	}
	neighbor := &netlink.Neigh{
		LinkIndex:    near.Attrs().Index,
		Family:       netlink.FAMILY_V6,
		State:        netlink.NUD_PERMANENT,
		IP:           net.ParseIP(bfdLinkLocalPeer),
		HardwareAddr: link.farMAC,
	}
	if err := netlink.NeighSet(neighbor); err != nil {
		return fmt.Errorf("neighbor %s on %s: %w", bfdLinkLocalPeer, link.near, err)
	}
	return nil
}

// bfdLinkLocalEnd is one far end while the scenario runs: a socket that writes
// the neighbor's frames and a socket that captures what reaches the far end.
// Both are released by close, which the scenario defers as soon as they open.
// Not safe for concurrent use, except that inject only reads the writer.
type bfdLinkLocalEnd struct {
	link    bfdLinkLocalLink
	writer  int
	capture int
	ifindex int
}

// bfdFirstPacketLinkLocalScenario is the observer driver. The captures open
// before the first frame is written, so every frame Ze sends from then on is
// read; the writers are stopped and waited for before the sockets close, so no
// goroutine outlives the run.
func bfdFirstPacketLinkLocalScenario(ctx context.Context, plugin *sdk.Plugin) error {
	links := bfdLinkLocalLinks()
	ends := make([]*bfdLinkLocalEnd, 0, len(links))
	defer func() {
		for _, end := range ends {
			end.close()
		}
	}()
	for i := range links {
		end, err := bfdLinkLocalOpen(&links[i])
		if err != nil {
			return err
		}
		ends = append(ends, end)
	}

	stop := make(chan struct{})
	writeErr := make(chan error, len(ends))
	var writers sync.WaitGroup
	defer func() {
		close(stop)
		writers.Wait()
	}()
	for _, end := range ends {
		writers.Add(1)
		go end.inject(stop, writeErr, &writers) // goroutine-lifecycle: one writer per link for the run, stopped and waited for by the defer above
	}

	if err := bfdLinkLocalAwaitSelection(ctx, plugin, ends, writeErr); err != nil {
		return err
	}
	for _, end := range ends {
		if err := end.expectOwnFrames(ends, writeErr); err != nil {
			return err
		}
	}
	return nil
}

// bfdLinkLocalOpen opens the far end's writer and capture. The writer's
// protocol is zero, so it receives nothing; the capture takes every frame on
// the far end, and a receive timeout keeps its reads bounded.
func bfdLinkLocalOpen(link *bfdLinkLocalLink) (*bfdLinkLocalEnd, error) {
	far, err := netlink.LinkByName(link.far)
	if err != nil {
		return nil, fmt.Errorf("find %s: %w", link.far, err)
	}
	end := &bfdLinkLocalEnd{link: *link, writer: -1, capture: -1, ifindex: far.Attrs().Index}

	end.writer, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open the writer on %s: %w", link.far, err)
	}
	all := vrrpTrackHtons(unix.ETH_P_ALL)
	end.capture, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, int(all))
	if err != nil {
		end.close()
		return nil, fmt.Errorf("open the capture on %s: %w", link.far, err)
	}
	if err := unix.Bind(end.capture, &unix.SockaddrLinklayer{Protocol: all, Ifindex: end.ifindex}); err != nil {
		end.close()
		return nil, fmt.Errorf("bind the capture to %s: %w", link.far, err)
	}
	timeout := unix.Timeval{Sec: 0, Usec: 200000}
	if err := unix.SetsockoptTimeval(end.capture, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &timeout); err != nil {
		end.close()
		return nil, fmt.Errorf("set the capture timeout on %s: %w", link.far, err)
	}
	// The capture queues from its open until the scenario reads it, up to
	// bfdLinkLocalSelectWindow plus a capture window later, and it queues the
	// writer's own frames too (skipped on read). A full queue drops the NEWEST
	// frames, which are the ones the send check counts, so the queue is forced
	// past rmem_max; the .ci declares net-admin, which SO_RCVBUFFORCE needs.
	if err := unix.SetsockoptInt(end.capture, unix.SOL_SOCKET, unix.SO_RCVBUFFORCE, bfdLinkLocalCaptureQueueOctets); err != nil {
		end.close()
		return nil, fmt.Errorf("size the capture queue on %s: %w", link.far, err)
	}
	return end, nil
}

// close releases both sockets. Safe to call on a half-opened end. It MUST be
// called only after inject has returned.
func (end *bfdLinkLocalEnd) close() {
	if end.writer >= 0 {
		unix.Close(end.writer) //nolint:errcheck // the run is over; nothing reads the result
		end.writer = -1
	}
	if end.capture >= 0 {
		unix.Close(end.capture) //nolint:errcheck // the run is over; nothing reads the result
		end.capture = -1
	}
}

// inject writes the neighbor's Down packet on the far end every
// bfdLinkLocalInjectInterval until stop closes. The caller MUST call
// writers.Add(1) before starting it and MUST wait for writers before close.
func (end *bfdLinkLocalEnd) inject(stop <-chan struct{}, fail chan<- error, writers *sync.WaitGroup) {
	defer writers.Done()

	frame := make([]byte, bfdLinkLocalFrameOctets)
	if _, err := bfdLinkLocalFrame(frame, &end.link); err != nil {
		fail <- err
		return
	}
	target := &unix.SockaddrLinklayer{Ifindex: end.ifindex, Halen: 6}
	copy(target.Addr[:], end.link.nearMAC)
	ticker := time.NewTicker(bfdLinkLocalInjectInterval)
	defer ticker.Stop()
	for {
		if err := unix.Sendto(end.writer, frame, 0, target); err != nil {
			fail <- fmt.Errorf("write the neighbor's frame on %s: %w", end.link.far, err)
			return
		}
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

// bfdLinkLocalAwaitSelection polls `show bfd sessions` until the session on
// each link reports that link's injected My Discriminator, which Ze learns only
// from a packet it selected that session for.
func bfdLinkLocalAwaitSelection(ctx context.Context, plugin *sdk.Plugin, ends []*bfdLinkLocalEnd, writeErr <-chan error) error {
	selected := make([]bool, len(ends))
	last := make([]uint32, len(ends))
	deadline := time.Now().Add(bfdLinkLocalSelectWindow)
	for time.Now().Before(deadline) {
		select {
		case err := <-writeErr:
			return err
		default:
		}
		sessions, err := bfdLinkLocalSessions(ctx, plugin)
		if err != nil {
			return err
		}
		done := true
		for i, end := range ends {
			if selected[i] {
				continue
			}
			seen, found, err := bfdLinkLocalRemoteDiscriminator(sessions, end.link.near)
			if err != nil {
				return err
			}
			last[i] = seen
			if found && seen == end.link.discriminator {
				selected[i] = true
				bfdLinkLocalSaySelected(end.link.near, seen)
				continue
			}
			if found && seen != 0 {
				return fmt.Errorf("the session to %s on %s took remote-discriminator %#x, not this link's %#x: a packet from one link selected the other link's session",
					bfdLinkLocalPeer, end.link.near, seen, end.link.discriminator)
			}
			done = false
		}
		if done {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	for i, end := range ends {
		if !selected[i] {
			return fmt.Errorf("no session was selected for the Your Discriminator 0 packets from %s on %s: remote-discriminator is %#x, want %#x, so the received zone was not reconciled with the session's interface",
				bfdLinkLocalPeer, end.link.near, last[i], end.link.discriminator)
		}
	}
	return nil
}

// bfdLinkLocalSessions answers `show bfd sessions` decoded.
func bfdLinkLocalSessions(ctx context.Context, plugin *sdk.Plugin) ([]map[string]any, error) {
	var raw json.RawMessage
	status, err := Dispatch(ctx, plugin, "show bfd sessions", &raw)
	if err != nil {
		return nil, fmt.Errorf("show bfd sessions: %w", err)
	}
	if status != statusDone {
		return nil, fmt.Errorf("show bfd sessions: status=%s data=%s", status, raw)
	}
	var sessions []map[string]any
	if err := json.Unmarshal(raw, &sessions); err != nil {
		return nil, fmt.Errorf("show bfd sessions: answer %s is not a list of sessions: %w", raw, err)
	}
	return sessions, nil
}

// expectOwnFrames reads the far end for bfdLinkLocalCaptureWindow and requires
// at least one of Ze's frames carrying this link's discriminator and none
// carrying another link's. Frames the far end itself wrote are skipped by
// packet type, and frames Ze sent before it learned the discriminator carry
// zero and prove nothing either way. A writer that failed during the window
// is reported as itself: with no injection the neighbor is gone, so a missing
// frame would otherwise read as a frame that left on the wrong link.
func (end *bfdLinkLocalEnd) expectOwnFrames(ends []*bfdLinkLocalEnd, writeErr <-chan error) error {
	buffer := make([]byte, 2048)
	own := 0
	deadline := time.Now().Add(bfdLinkLocalCaptureWindow)
	for time.Now().Before(deadline) {
		select {
		case err := <-writeErr:
			return err
		default:
		}
		n, from, err := unix.Recvfrom(end.capture, buffer, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return fmt.Errorf("read the capture on %s: %w", end.link.far, err)
		}
		if bfdLinkLocalWrittenHere(from) {
			continue
		}
		yours, err := bfdLinkLocalSentYourDiscriminator(buffer[:n])
		if errors.Is(err, errBfdLinkLocalNotControl) {
			continue
		}
		if err != nil {
			return fmt.Errorf("on %s: %w", end.link.far, err)
		}
		if yours == end.link.discriminator {
			own++
			continue
		}
		for _, other := range ends {
			if other != end && yours == other.link.discriminator {
				return fmt.Errorf("a frame Ze sent for the session on %s left on %s: it carries Your Discriminator %#x, the discriminator of %s's neighbor",
					other.link.near, end.link.near, yours, other.link.near)
			}
		}
	}
	if own == 0 {
		return fmt.Errorf("no frame Ze sent for the session on %s reached %s in %s: the session's packets did not leave on the link it names",
			end.link.near, end.link.far, bfdLinkLocalCaptureWindow)
	}
	bfdLinkLocalSayCaptured(end.link.near, own, end.link.discriminator)
	return nil
}

// bfdLinkLocalWrittenHere reports a frame the capture saw leave the far end,
// which is the fixture's own injected frame and not Ze's.
func bfdLinkLocalWrittenHere(from unix.Sockaddr) bool {
	linklayer, ok := from.(*unix.SockaddrLinklayer)
	if !ok {
		return false
	}
	return linklayer.Pkttype == unix.PACKET_OUTGOING
}

// bfdLinkLocalSaySelected prints the receive line the .ci matches for a link.
func bfdLinkLocalSaySelected(iface string, discriminator uint32) {
	var tb textbuf.Buffer
	tb.Str("OK: the first packet on ").Str(iface).Str(" selected the session to ").Str(bfdLinkLocalPeer).
		Str(" on ").Str(iface).Str(", remote-discriminator ").Uint32(discriminator).Byte('\n').
		StdErr() //nolint:errcheck // fixture output; the .ci reads it
}

// bfdLinkLocalSayCaptured prints the send line the .ci matches for a link.
func bfdLinkLocalSayCaptured(iface string, frames int, discriminator uint32) {
	var tb textbuf.Buffer
	tb.Str("OK: ").Int(int64(frames)).Str(" frames left on ").Str(iface).Str(" with Your Discriminator ").Uint32(discriminator).
		Str(" from ").Str(bfdLinkLocalLocal).Str(" to ").Str(bfdLinkLocalPeer).Str(" with hop limit 255, and none with another link's\n").
		StdErr() //nolint:errcheck // fixture output; the .ci reads it
}

// Design: docs/features/interfaces.md -- retained original native transactions
//go:build integration && linux

package ifacenetlink

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

const nativeVLANTransactionTestGuard = 5 * time.Second

// Goal: a queued kernel reply for another sequence cannot answer this request.
// Method: use one actual route socket, enqueue an old real link/ACK and error,
// then require current absent/healthy outcomes. There is no substituted IO.
// MUTATION: accept a kernel message without comparing its original sequence.
func TestNativeAccessTransactionIgnoresObsoleteNativeReplies(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		transport := newNativeTransactionTestTransport(t)
		loopback, err := netlink.LinkByName("lo")
		if err != nil {
			t.Fatal(err)
		}
		sendNativeTransactionTestRequest(t, transport, nativeVLANLinkRequest(unix.RTM_GETLINK, unix.NLM_F_ACK, loopback.Attrs().Index), 123)
		ctx, cancel := context.WithTimeout(t.Context(), nativeVLANTransactionTestGuard)
		defer cancel()
		if err := transport.bind(ctx); err != nil {
			t.Fatal(err)
		}
		_, err = transport.byIndex(nativeAccessIndexMax)
		if err == nil {
			t.Fatal("obsolete native link answered an absent original query")
		}
		if !errors.Is(err, unix.ENODEV) {
			t.Fatalf("original absent query cause: %v", err)
		}
		sendNativeTransactionTestRequest(t, transport, nativeVLANLinkRequest(unix.RTM_GETLINK, unix.NLM_F_ACK, nativeAccessIndexMax), 124)
		actual, err := transport.byIndex(loopback.Attrs().Index)
		if err != nil {
			t.Fatalf("obsolete native error rejected a healthy original query: %v", err)
		}
		if actual.Attrs().Index != loopback.Attrs().Index {
			t.Fatal("healthy original query returned another interface")
		}
		if transport.pending != nil {
			t.Fatal("completed original lookup retained a pending transaction")
		}
	})
}

// Goal: partial original replies and unrelated native traffic cannot renew the
// original absolute read deadline. The original accumulator survives expiry.
// Method: the real kernel returns a link, a socket filter drops only its ACK,
// and three causally consumed old-sequence responses arrive before expiry.
// MUTATION: give resume a fresh context detached from the original deadline.
func TestNativeAccessPartialReplyKeepsOriginalDeadline(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() { nativePartialTransactionTest(t, false) })
}

// Goal: cancellation joins the original poll callback and retains partial data.
// Method: cancel only after three real old-sequence messages were consumed;
// neither another query nor a cleanup retry may replace/resend the lost ACK.
// MUTATION: omit the original context's SetDeadline cancellation callback.
func TestNativeAccessPartialReplyCancellationKeepsOriginalAccumulator(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() { nativePartialTransactionTest(t, true) })
}

// Goal: an arbitrary ACK body cannot mint CREATE authority without its echo.
// Method: fuzz only the pure reply consumer with a synthetic pending owner;
// this supplies no socket and claims no native/kernel positive evidence.
func FuzzNativeAccessOriginalACKNeedsEcho(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, 4+unix.SizeofNlMsghdr))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > nativeVLANDatagramLimit {
			return
		}
		transport := &nativeVLANTransport{pid: 17}
		transaction := &nativeVLANTransaction{transport: transport, sequence: 1, kind: unix.RTM_NEWLINK, mode: nativeVLANReplyCreate, sendPhase: nativeVLANSendReturned, replies: nativeVLANReplyPending{}}
		transport.pending = transaction
		if _, err := transaction.consume(syscall.NetlinkMessage{Header: syscall.NlMsghdr{Type: unix.NLMSG_ERROR}, Data: payload}); err != nil {
			if transaction.nativeSuccess() {
				t.Fatal("rejected ACK body created native authority")
			}
			return
		}
		if transaction.nativeSuccess() {
			t.Fatal("ACK without an original echo created native authority")
		}
	})
}

func newNativeTransactionTestTransport(t *testing.T) *nativeVLANTransport {
	t.Helper()
	ops, err := newNativeVLANOps()
	if err != nil {
		t.Fatal(err)
	}
	transport := ops.transport
	if transport == nil {
		t.Fatal("native operations omitted their actual route socket")
	}
	t.Cleanup(func() {
		// These direct transport questions issue GETLINK only. The original send
		// and test goroutine are joined before disposal; no mutation is abandoned.
		if err := transport.file.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
	})
	return transport
}

func sendNativeTransactionTestRequest(t *testing.T, transport *nativeVLANTransport, request *nl.NetlinkRequest, sequence uint32) {
	t.Helper()
	request.Seq = sequence
	request.Pid = transport.pid
	raw, err := transport.file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var sendErr error
	if err := raw.Control(func(fd uintptr) {
		sendErr = unix.Sendto(int(fd), request.Serialize(), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK})
	}); err != nil {
		t.Fatal(err)
	}
	if sendErr != nil {
		t.Fatal(sendErr)
	}
}

func nativePartialTransactionTest(t *testing.T, cancelEarly bool) {
	t.Helper()
	ops, err := newNativeVLANOps()
	if err != nil {
		t.Fatal(err)
	}
	transport := ops.transport
	if transport == nil {
		t.Fatal("actual native operations omitted their original socket")
	}
	resources := &nativeVLANResources{ops: ops}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var answer chan error
	var originalRequest *nativeVLANRequest
	t.Cleanup(func() {
		cancel()
		// Every request in this fixture is GETLINK. The intentionally discarded
		// ACK is irrecoverable, so test disposal closes this read-only socket after
		// assertions, joins its actual reader, and stops that now-idle test worker.
		// This explicit fixture teardown is NOT a product mutation/retirement ACK.
		if err := transport.file.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
		if originalRequest != nil {
			select {
			case <-originalRequest.done:
			case <-time.After(nativeVLANTransactionTestGuard):
				t.Error("original native reader did not join")
				return
			}
		}
		if answer == nil {
			return
		} // No worker was admitted before an earlier fixture failure.
		select {
		case <-answer:
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Error("original native caller did not join")
			return
		}
		cleanup, cancelCleanup := context.WithTimeout(context.Background(), nativeVLANTransactionTestGuard)
		defer cancelCleanup()
		if _, err := resources.run(cleanup, true, func(context.Context) (any, error) { return struct{}{}, nil }, nil); err != nil {
			t.Error(err)
		}
	})
	loopback, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	target := transport.seq + 1
	dropNativeTransactionTestACK(t, transport, target)
	observed := make(chan uint32, 16)
	transport.observeReply = func(sequence uint32) { observed <- sequence }
	if err := transport.bind(ctx); err != nil {
		t.Fatal(err)
	}
	answer = make(chan error, 1)
	go func() {
		defer close(answer)
		_, err := resources.run(ctx, false, func(turn context.Context) (any, error) {
			if err := transport.bind(turn); err != nil {
				return nil, err
			}
			_, err := transport.request(nativeVLANLinkRequest(unix.RTM_GETLINK, unix.NLM_F_ACK, loopback.Attrs().Index), nativeVLANReplyCreate)
			return nil, err
		}, nil)
		answer <- err
	}()
	nextNativeTransactionTestReply(t, observed, target)
	resources.workerMu.Lock()
	originalRequest = resources.worker.current
	resources.workerMu.Unlock()
	if originalRequest == nil {
		t.Fatal("actual original native read completed without its required ACK")
	}
	for i := range 3 {
		obsolete := uint32(500 + i)
		sendNativeTransactionTestRequest(t, transport, nativeVLANLinkRequest(unix.RTM_GETLINK, unix.NLM_F_ACK, loopback.Attrs().Index), obsolete)
		nextNativeTransactionTestReply(t, observed, obsolete)
	}
	if cancelEarly {
		cancel()
	} else {
		<-ctx.Done()
	}
	select {
	case err := <-answer:
		if err == nil {
			t.Fatal("partial original reply was reported as complete")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("partial native receive renewed or ignored the original caller budget")
	}
	select {
	case <-originalRequest.done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("original native poll callback did not join within its own canceled budget")
	}
	original := transport.pending
	if original == nil {
		t.Fatal("partial native outcome lost its original transaction")
	}
	if original.sequence != target {
		t.Fatal("pending reply owner changed sequence")
	}
	if original.sendPhase != nativeVLANSendReturned {
		t.Fatal("real original GETLINK send did not physically finish")
	}
	if len(original.links) != 1 {
		t.Fatalf("original native link accumulator length %d", len(original.links))
	}
	if original.acknowledged {
		t.Fatal("dropped native ACK was invented")
	}
	before := transport.seq
	retry, retryCancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer retryCancel()
	if err := transport.bind(retry); !errors.Is(err, iface.ErrNativeAccessPending) {
		t.Fatalf("ordinary request replaced pending native transaction: %v", err)
	}
	detachNativeTransactionTestACK(t, transport)
	if err := resources.join(retry); err == nil {
		t.Fatal("physical join falsely settled an original lost ACK")
	}
	if transport.pending != original {
		t.Fatal("cleanup replaced its original native transaction")
	}
	if transport.seq != before {
		t.Fatal("cleanup resent or renumbered its original request")
	}
	if len(original.links) != 1 {
		t.Fatal("cleanup discarded its original partial echo")
	}
	// A separate real socket is healthy. The old socket is deliberately retained
	// as unknown: an ACK discarded by the kernel filter cannot be regenerated.
	healthy := newNativeTransactionTestTransport(t)
	healthyCtx, healthyCancel := context.WithTimeout(t.Context(), nativeVLANTransactionTestGuard)
	defer healthyCancel()
	if err := healthy.bind(healthyCtx); err != nil {
		t.Fatal(err)
	}
	actual, err := healthy.byIndex(loopback.Attrs().Index)
	if err != nil {
		t.Fatalf("healthy independent native query failed: %v", err)
	}
	if actual.Attrs().Index != loopback.Attrs().Index {
		t.Fatal("healthy control names another native interface")
	}
}

func nextNativeTransactionTestReply(t *testing.T, observed <-chan uint32, wanted uint32) {
	t.Helper()
	// At most seven real replies are generated by the bounded test population.
	for range 8 {
		select {
		case sequence := <-observed:
			if sequence == wanted {
				return
			}
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatalf("kernel message for sequence %d was not consumed", wanted)
		}
	}
	t.Fatalf("bounded native reply census omitted sequence %d", wanted)
}

func dropNativeTransactionTestACK(t *testing.T, transport *nativeVLANTransport, sequence uint32) {
	t.Helper()
	var encoded [4]byte
	nl.NativeEndian().PutUint32(encoded[:], sequence)
	sequenceOnWire := binary.BigEndian.Uint32(encoded[:])
	program := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_B | unix.BPF_ABS, K: 4},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: unix.NLMSG_ERROR, Jf: 3},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 8},
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: sequenceOnWire, Jf: 1},
		{Code: unix.BPF_RET | unix.BPF_K, K: 0},
		{Code: unix.BPF_RET | unix.BPF_K, K: nativeVLANDatagramLimit},
	}
	raw, err := transport.file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var attachErr error
	if err := raw.Control(func(fd uintptr) {
		attachErr = unix.SetsockoptSockFprog(int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, &unix.SockFprog{Len: uint16(len(program)), Filter: &program[0]})
	}); err != nil {
		t.Fatal(err)
	}
	if attachErr != nil {
		t.Fatal(attachErr)
	}
}

func detachNativeTransactionTestACK(t *testing.T, transport *nativeVLANTransport) {
	t.Helper()
	raw, err := transport.file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var detachErr error
	if err := raw.Control(func(fd uintptr) { detachErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_DETACH_FILTER, 0) }); err != nil {
		t.Fatal(err)
	}
	if detachErr != nil {
		t.Fatal(detachErr)
	}
}

// Goal: caller expiry cannot lose a successful original CREATE or stamp it late.
// Method: withhold processing immediately after the ACTUAL kernel CREATE send,
// let its caller expire, and keep traffic crossing a separate leased VLAN pair.
// On release, the exact queued ECHO+ACK roll back the unpublished original.
// MUTATION: discard a pending transaction before consuming its original replies.
func TestNativeAccessLateCreateKeepsOriginalOutcomeAndForwardingSurvivor(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		pair := &netlink.Veth{Name: "nativel0", PeerName: "nativer0"}
		if err := netlink.LinkAdd(pair); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"nativel0", "nativer0"} {
			link, err := netlink.LinkByName(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := netlink.LinkSetUp(link); err != nil {
				t.Fatal(err)
			}
		}
		backend := &netlinkBackend{}
		// Capture the original route socket on this namespace-bearing thread.
		ops, err := newNativeVLANOps()
		if err != nil {
			t.Fatal(err)
		}
		backend.nativeVLANs.ops = ops
		entered := make(chan *nativeVLANTransaction, 1)
		release := make(chan struct{})
		resume := sync.OnceFunc(func() { close(release) })
		var createAnswer chan error
		t.Cleanup(func() {
			resume()
			if createAnswer != nil {
				select {
				case <-createAnswer:
				case <-time.After(nativeVLANTransactionTestGuard):
					t.Error("bounded original creator caller did not join")
				}
			}
			joinCtx, cancel := context.WithTimeout(context.Background(), nativeVLANTransactionTestGuard)
			defer cancel()
			if err := backend.nativeVLANs.join(joinCtx); err != nil {
				t.Error(err)
			}
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		for _, parent := range []string{"nativel0", "nativer0"} {
			if err := backend.CreateVLAN(iface.VLANSpec{Parent: parent, VLANID: 400}); err != nil {
				t.Fatal(err)
			}
		}
		actual, err := backend.GetLink("nativel0.400")
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := backend.CaptureNativeVLAN(t.Context(), iface.Binding{OsName: actual.OsName, Ifindex: actual.Index, Alias: actual.Alias})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := prepared.Close(); err != nil {
				t.Error(err)
			}
		})
		writer, err := prepared.Borrow(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := writer.Close(); err != nil {
				t.Error(err)
			}
		})
		joiner, ok := writer.(iface.NativeVLANOperationJoiner)
		if !ok {
			t.Fatal("actual native writer omitted its physical join contract")
		}
		sender := nativeVLANForwardingTestSocket(t, "nativel0.400")
		receiver := nativeVLANForwardingTestSocket(t, "nativer0.400")
		nativeVLANForwardingTestFrame(t, sender, receiver, 1)
		addVLANQoSDummy(t, "latevlan0")
		ops.transport.observeSent = func(kind uint16, sequence uint32) {
			if kind != unix.RTM_NEWLINK {
				return
			}
			creator := ops.transport.creator
			if creator == nil {
				return
			}
			if creator.identity.Device != "latevlan0.317" {
				return
			}
			entered <- creator.creation
			<-release
		}
		createAnswer = make(chan error, 1)
		go func() {
			defer close(createAnswer)
			createAnswer <- backend.CreateVLAN(iface.VLANSpec{Parent: "latevlan0", VLANID: 317})
		}()
		var original *nativeVLANTransaction
		select {
		case original = <-entered:
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("original native CREATE did not reach the actual kernel send")
		}
		select {
		case err := <-createAnswer:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("original creator deadline: %v", err)
			}
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("original creator caller was not bounded")
		}
		if original == nil {
			t.Fatal("actual native CREATE lost its original transaction")
		}
		nativeVLANForwardingTestFrame(t, sender, receiver, 2)
		pendingCtx, pendingCancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		if err := joiner.JoinNativeVLAN(pendingCtx); err == nil {
			t.Fatal("withheld original native transaction falsely joined")
		}
		pendingCancel()
		resume()
		joinCtx, joinCancel := context.WithTimeout(t.Context(), nativeVLANTransactionTestGuard)
		defer joinCancel()
		if err := joiner.JoinNativeVLAN(joinCtx); err != nil {
			t.Fatal(err)
		}
		if !original.nativeSuccess() {
			t.Fatal("late original ECHO/ACK was discarded after the syscall returned")
		}
		if original.creator.creation != original {
			t.Fatal("late CREATE was rebound to another receipt")
		}
		if original.transport.pending != nil {
			t.Fatal("completed original outcome still blocked its transport")
		}
		if _, err := netlink.LinkByName("latevlan0.317"); err == nil {
			t.Fatal("expired unpublished original was stamped or leaked instead of rolled back")
		}
		nativeVLANForwardingTestFrame(t, sender, receiver, 3)
	})
}

type nativeVLANForwardingSocket struct {
	file  *os.File
	index int
	mac   [6]byte
}

// nativeVLANForwardingTestEtherType is the local experimental ethertype
// (IEEE 802 0x88B5) every forwarding test frame carries.
const nativeVLANForwardingTestEtherType = 0x88b5

func nativeVLANForwardingTestSocket(t *testing.T, name string) nativeVLANForwardingSocket {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(link.Attrs().HardwareAddr) != 6 {
		t.Fatal("native survivor has no Ethernet MAC")
	}
	// Bound to the test frames' own ethertype: with ETH_P_ALL the fresh VLAN
	// devices' IPv6 MLD/ND chatter used up the receive budget below.
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], nativeVLANForwardingTestEtherType)
	protocol := nl.NativeEndian().Uint16(encoded[:])
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(protocol))
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(fd), "native-vlan-survivor")
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: protocol, Ifindex: link.Attrs().Index}); err != nil {
		t.Fatal(err)
	}
	value := nativeVLANForwardingSocket{file: file, index: link.Attrs().Index}
	copy(value.mac[:], link.Attrs().HardwareAddr)
	return value
}

func nativeVLANForwardingTestFrame(t *testing.T, sender, receiver nativeVLANForwardingSocket, ordinal byte) {
	t.Helper()
	frame := make([]byte, 60)
	copy(frame[:6], receiver.mac[:])
	copy(frame[6:12], sender.mac[:])
	binary.BigEndian.PutUint16(frame[12:14], nativeVLANForwardingTestEtherType)
	frame[14] = ordinal
	raw, err := sender.file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var sendErr error
	if err := raw.Control(func(fd uintptr) {
		sendErr = unix.Sendto(int(fd), frame, 0, &unix.SockaddrLinklayer{Protocol: nl.NativeEndian().Uint16(frame[12:14]), Ifindex: sender.index, Halen: 6, Addr: [8]byte{receiver.mac[0], receiver.mac[1], receiver.mac[2], receiver.mac[3], receiver.mac[4], receiver.mac[5]}})
	}); err != nil {
		t.Fatal(err)
	}
	if sendErr != nil {
		t.Fatal(sendErr)
	}
	if err := receiver.file.SetReadDeadline(time.Now().Add(nativeVLANTransactionTestGuard)); err != nil {
		t.Fatal(err)
	}
	raw, err = receiver.file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	// The receiver is bound to the opposite VLAN device, not the sender's socket.
	// Ignore at most eight other frames of the test ethertype; exact payload
	// proves delivery.
	for range 8 {
		buffer := make([]byte, 2048)
		var count int
		var from unix.Sockaddr
		var readErr error
		if err := raw.Read(func(fd uintptr) bool {
			count, from, readErr = unix.Recvfrom(int(fd), buffer, 0)
			return readErr != unix.EAGAIN
		}); err != nil {
			t.Fatal(err)
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		source, ok := from.(*unix.SockaddrLinklayer)
		if !ok {
			t.Fatal("native Ethernet receive has another address family")
		}
		if source.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		if bytes.Equal(buffer[:count], frame) {
			return
		}
	}
	t.Fatal("unrelated healthy original VLAN stopped forwarding its exact frame")
}

// Goal: late rollback may delete only its exact original root and owned children.
// Method: after the real CREATE send, attach a foreign native VLAN child before
// releasing the expired original worker; require both links and its quarantine.
// MUTATION: bypass the shared descendant proof for unpublished rollback.
func TestNativeAccessLateRollbackPreservesForeignChild(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "latechild0")
		backend := &netlinkBackend{}
		ops, err := newNativeVLANOps()
		if err != nil {
			t.Fatal(err)
		}
		backend.nativeVLANs.ops = ops
		entered := make(chan *nativeVLANTransaction, 1)
		release := make(chan struct{})
		resume := sync.OnceFunc(func() { close(release) })
		var original *nativeVLANTransaction
		var answer chan error
		t.Cleanup(func() {
			resume()
			if answer != nil {
				select {
				case <-answer:
				case <-time.After(nativeVLANTransactionTestGuard):
					t.Error("original creator caller did not join")
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), nativeVLANTransactionTestGuard)
			defer cancel()
			if err := backend.nativeVLANs.join(ctx); err != nil {
				t.Error(err)
			}
			for _, name := range []string{"foreign317", "latechild0.317"} {
				link, err := netlink.LinkByName(name)
				if err == nil {
					if err := netlink.LinkDel(link); err != nil {
						t.Error(err)
					}
				}
			}
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		ops.transport.observeSent = func(kind uint16, sequence uint32) {
			if kind != unix.RTM_NEWLINK {
				return
			}
			creator := ops.transport.creator
			if creator == nil {
				return
			}
			if creator.identity.Device != "latechild0.317" {
				return
			}
			entered <- creator.creation
			<-release
		}
		answer = make(chan error, 1)
		go func() {
			defer close(answer)
			answer <- backend.CreateVLAN(iface.VLANSpec{Parent: "latechild0", VLANID: 317})
		}()
		select {
		case original = <-entered:
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("original CREATE did not reach its actual send")
		}
		root, err := netlink.LinkByName("latechild0.317")
		if err != nil {
			t.Fatal(err)
		}
		child := &netlink.Vlan{Name: "foreign317", ParentIndex: root.Attrs().Index, VlanId: 318}
		if err := netlink.LinkAdd(child); err != nil {
			t.Fatal(err)
		}
		foreign, err := netlink.LinkByName("foreign317")
		if err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-answer:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("original expiry: %v", err)
			}
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("original caller was not bounded")
		}
		resume()
		ctx, cancel := context.WithTimeout(t.Context(), nativeVLANTransactionTestGuard)
		defer cancel()
		if err := backend.nativeVLANs.join(ctx); err != nil {
			t.Fatal(err)
		}
		if !original.nativeSuccess() {
			t.Fatal("actual original ECHO and ACK did not settle")
		}
		actualRoot, err := netlink.LinkByIndex(root.Attrs().Index)
		if err != nil {
			t.Fatalf("rollback deleted quarantined root: %v", err)
		}
		if actualRoot.Attrs().Alias != "" {
			t.Fatal("expired original was stamped late")
		}
		actualChild, err := netlink.LinkByIndex(foreign.Attrs().Index)
		if err != nil {
			t.Fatalf("rollback cascaded through foreign child: %v", err)
		}
		if actualChild.Attrs().Name != "foreign317" {
			t.Fatal("foreign child identity changed")
		}
		if backend.nativeVLANs.records[original.creator.identity.Birth] != original.creator {
			t.Fatal("failed original rollback retired quarantine")
		}
		if _, ok := original.creator.state.(nativeVLANUnmarked); !ok {
			t.Fatal("failed original rollback became mutable")
		}
	})
}

// Goal: original late ECHO/ACK yields only an absence-reconciliation index.
// Method: hold the first actual CREATE reader until its caller expires, join
// that released worker, resume its original socket, and challenge a foreign
// alias at that index before explicit external removal permits Close.
// MUTATION: omit validated original ECHO index reconciliation after resumption.
func TestNativeAccessLateEchoAllowsOnlyOriginalAbsenceRetirement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withVLANQoSNetNS(t, func() {
		addVLANQoSDummy(t, "lateecho0")
		backend := &netlinkBackend{}
		ops, err := newNativeVLANOps()
		if err != nil {
			t.Fatal(err)
		}
		backend.nativeVLANs.ops = ops
		entered := make(chan *nativeVLANTransaction, 1)
		release := make(chan struct{})
		resume := sync.OnceFunc(func() { close(release) })
		var original *nativeVLANTransaction
		var issued *nativeVLANRequest
		var answer chan error
		t.Cleanup(func() {
			resume()
			if answer != nil {
				select {
				case <-answer:
				case <-time.After(nativeVLANTransactionTestGuard):
					t.Error("original caller did not join")
				}
			}
			if issued != nil {
				select {
				case <-issued.done:
				case <-time.After(nativeVLANTransactionTestGuard):
					t.Error("original reader did not join")
					return
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), nativeVLANTransactionTestGuard)
			defer cancel()
			if err := backend.nativeVLANs.join(ctx); err != nil {
				t.Error(err)
			}
			if link, err := netlink.LinkByName("lateecho0.319"); err == nil {
				if err := netlink.LinkDel(link); err != nil {
					t.Error(err)
				}
			}
			if err := backend.Close(); err != nil {
				t.Error(err)
			}
		})
		ops.transport.observeBeforeRead = func(transaction *nativeVLANTransaction) {
			if transaction.mode != nativeVLANReplyCreate {
				return
			}
			entered <- transaction
			<-release
		}
		// The first name validation in a process loads the reserved names from
		// the YANG schema; a daemon does it at config validation. Done here, it
		// keeps that load out of the 50 ms budget the CREATE turn is given.
		if err := iface.ValidateIfaceName("lateecho0"); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		answer = make(chan error, 1)
		go func() {
			defer close(answer)
			_, err := backend.nativeVLANs.run(ctx, false, func(turn context.Context) (any, error) {
				return nil, backend.nativeVLANs.createDirect(turn, iface.VLANSpec{Parent: "lateecho0", VLANID: 319})
			}, nil)
			answer <- err
		}()
		select {
		case original = <-entered:
		case err := <-answer:
			t.Fatalf("original CREATE ended before its reader: %v", err)
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("original CREATE did not reach its actual reader")
		}
		backend.nativeVLANs.workerMu.Lock()
		issued = backend.nativeVLANs.worker.current
		backend.nativeVLANs.workerMu.Unlock()
		if issued == nil {
			t.Fatal("original reader was not retained")
		}
		select {
		case err := <-answer:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("original expiry: %v", err)
			}
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("original caller was not bounded")
		}
		resume()
		select {
		case <-issued.done:
		case <-time.After(nativeVLANTransactionTestGuard):
			t.Fatal("expired original reader did not release its fence")
		}
		if original.creator.identity.Index != 0 {
			t.Fatal("fixture consumed an original ECHO before fence release")
		}
		ops.transport.observeBeforeRead = nil
		cleanup, cancelCleanup := context.WithTimeout(t.Context(), nativeVLANTransactionTestGuard)
		defer cancelCleanup()
		if err := backend.nativeVLANs.join(cleanup); err != nil {
			t.Fatal(err)
		}
		actual, err := netlink.LinkByName("lateecho0.319")
		if err != nil {
			t.Fatal(err)
		}
		if !original.nativeSuccess() {
			t.Fatal("original queued ECHO/ACK did not resume")
		}
		if original.creator.identity.Index != actual.Attrs().Index {
			t.Fatal("validated late original ECHO did not retain its absence index")
		}
		if actual.Attrs().Alias != "" {
			t.Fatal("cleanup stamped expired original")
		}
		if err := netlink.LinkSetAlias(actual, "foreign-index-owner"); err != nil {
			t.Fatal(err)
		}
		if err := backend.Close(); err == nil {
			t.Fatal("foreign occupant at original index retired quarantine")
		}
		foreign, err := netlink.LinkByIndex(actual.Attrs().Index)
		if err != nil {
			t.Fatalf("foreign occupant was removed: %v", err)
		}
		if foreign.Attrs().Alias != "foreign-index-owner" {
			t.Fatal("cleanup overwrote foreign index authority")
		}
		if err := netlink.LinkSetAlias(foreign, original.creator.alias); err != nil {
			t.Fatal(err)
		}
		if err := backend.Close(); err == nil {
			t.Fatal("later counterfeit original Birth adopted a released creator's mutable authority")
		}
		foreign, err = netlink.LinkByIndex(actual.Attrs().Index)
		if err != nil {
			t.Fatal(err)
		}
		if foreign.Attrs().Alias != original.creator.alias {
			t.Fatal("late matching marker was mutated during absence-only reconciliation")
		}
		if _, ok := original.creator.state.(nativeVLANUnmarked); !ok {
			t.Fatal("late reconciliation admitted mutable authority")
		}
		if backend.nativeVLANs.records[original.creator.identity.Birth] != original.creator {
			t.Fatal("foreign occupant released original quota")
		}
		if err := netlink.LinkDel(foreign); err != nil {
			t.Fatal(err)
		}
		if err := backend.Close(); err != nil {
			t.Fatalf("actual original-index absence could not retire quarantine: %v", err)
		}
		if _, ok := original.creator.state.(nativeVLANRetired); !ok {
			t.Fatal("confirmed absent original did not retire quota")
		}
	})
}

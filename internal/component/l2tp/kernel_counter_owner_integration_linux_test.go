//go:build integration && linux

// Design: docs/architecture/l2tp/subscriber-session-model.md -- original kernel counter ownership
// Related: ppp/kernel/created_unit.go -- the retained original counter reader
// These tests need the host's /dev/ppp grant. Each runs in its own user and
// network namespaces. DevPPPSetup creates a genuine original unit and attached
// channel over the fixture's native PPPoX carrier. These counter tests make no
// PPP negotiation or forwarding claim.

package l2tp

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// A live original remains measurable after rename, even when its old name
// resolves to a different device with distinguishable real counters.
func TestKernelCounterOwnerRenameKeepsOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	handle, backend, link, _ := kernelCounterOwner(t)
	kernelCounterOriginal(t, handle)
	if err := netlink.LinkSetName(link, "zc-original"); err != nil {
		t.Fatalf("rename original PPP unit: %v", err)
	}
	kernelCounterReplacement(t, backend, "zc-owner", 0)
	kernelCounterOriginal(t, handle)
}

// External deletion ends the original unit even while its file remains open.
// Reusing both its name and native index cannot authorize a replacement read.
func TestKernelCounterOwnerDeletedUnitRefusesReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	handle, backend, link, unitFD := kernelCounterOwner(t)
	kernelCounterOriginal(t, handle)
	index := link.Attrs().Index
	if err := netlink.LinkDel(link); err != nil {
		t.Fatalf("externally delete original PPP unit: %v", err)
	}
	kernelCounterDeadUnit(t, unitFD)
	kernelCounterReplacement(t, backend, "zc-owner", index)
	got, err := handle.Counters()
	if err == nil {
		t.Fatalf("dead original returned counters %+v from a reused name/index", got)
	}
}

// Calling from another network namespace still measures the live original.
// An equal name and index there identifies only that namespace's replacement.
func TestKernelCounterOwnerCallerNamespaceKeepsOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	handle, backend, link, _ := kernelCounterOwner(t)
	kernelCounterOriginal(t, handle)
	index := link.Attrs().Index
	kernelCounterOtherNamespace(t, func() {
		kernelCounterReplacement(t, backend, "zc-owner", index)
		kernelCounterOriginal(t, handle)
	})
	kernelCounterOriginal(t, handle)
}

// Moving the original device out of its creator namespace invalidates that
// namespace's ownership lookup. Reusing its old name and index must refuse,
// even if the original unit file remains live in the target namespace.
func TestKernelCounterOwnerMovedUnitRefusesReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	// Register both namespace lifetimes before constructing the PPP owner, so
	// its cleanup closes the unit before either namespace handle is released.
	original, target := kernelCounterMoveNamespaces(t)
	creator, err := netlink.NewHandleAt(original, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatalf("open creator-bound move socket: %v", err)
	}
	t.Cleanup(creator.Close)
	destination, err := netlink.NewHandleAt(target, unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatalf("open target-bound move socket: %v", err)
	}
	t.Cleanup(destination.Close)
	handle, backend, link, unitFD := kernelCounterOwner(t)
	kernelCounterOriginal(t, handle)
	index := link.Attrs().Index
	// Demand mode makes an unconnected unit's ordinary read-EOF distinct
	// from POLLHUP. Neither poll result is used to infer move support.
	if err := unix.IoctlSetPointerInt(unitFD, 0x40047459, 0x200); err != nil {
		t.Fatalf("set original SC_LOOP_TRAFFIC before move: %v", err)
	}
	poll := []unix.PollFd{{Fd: int32(unitFD), Events: unix.POLLIN}}
	if _, err := unix.Poll(poll, 0); err != nil {
		t.Fatalf("poll original unit before move: %v", err)
	}
	t.Logf("original unit poll before move = %#x", poll[0].Revents)
	if err := creator.LinkSetNsFd(link, int(target)); err != nil {
		t.Fatalf("real PPP device move setup failed: %v", err)
	}
	moved, err := destination.LinkByName("zc-owner")
	if err != nil {
		t.Fatalf("moved original is not present in target namespace: %v", err)
	}
	if moved.Type() != "ppp" {
		t.Fatalf("target device kind = %q, want original ppp", moved.Type())
	}
	_, err = creator.LinkByName("zc-owner")
	if err == nil {
		t.Fatal("moved original is still present in creator namespace")
	}
	if _, absent := errors.AsType[netlink.LinkNotFoundError](err); !absent {
		t.Fatalf("cannot verify original absence after move: %v", err)
	}
	poll[0].Revents = 0
	if _, err := unix.Poll(poll, 0); err != nil {
		t.Fatalf("poll original unit after verified move: %v", err)
	}
	t.Logf("original unit poll after verified move = %#x", poll[0].Revents)
	kernelCounterReplacement(t, backend, "zc-owner", index)
	got, err := handle.Counters()
	if err == nil {
		t.Fatalf("moved original returned counters %+v from creator-namespace replacement", got)
	}
}

// This is a Linux mechanism probe, separate from the product assertions above.
// A request-sequenced creator echo identifies the actual PPP device. The same
// retained socket keeps that namespace across caller changes, rename and delete.
// PPPIOCGUNIT remains a numeric label after deletion; original-file POLLHUP is
// the independently observed death signal. Passing this probe fixes no provider.
func TestKernelCounterCreatorEchoPrimitive(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	unitFD := kernelCounterOpenPPP(t)
	t.Cleanup(func() {
		if err := unix.Close(unitFD); err != nil {
			t.Error(err)
		}
	})
	socket, err := nl.Subscribe(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatalf("open creator namespace rtnetlink socket: %v", err)
	}
	t.Cleanup(socket.Close)
	timeout := unix.NsecToTimeval((3 * time.Second).Nanoseconds())
	if err := socket.SetSendTimeout(&timeout); err != nil {
		t.Fatal(err)
	}
	request := kernelCounterCreateRequest(unitFD, "")
	request.Flags |= unix.NLM_F_ECHO
	created, err := kernelCounterExchange(socket, request)
	if err != nil {
		t.Fatalf("creator echo primitive unavailable: %v", err)
	}
	if created == nil {
		t.Fatal("successful create ACK had no request-matched creator echo")
	}
	if created.Type() != "ppp" {
		t.Fatalf("creator echo kind = %q, want ppp", created.Type())
	}
	index := created.Attrs().Index
	if index <= 0 {
		t.Fatalf("creator echo index = %d, want a positive native index", index)
	}
	unit, err := unix.IoctlGetInt(unitFD, 0x80047456)
	if err != nil {
		t.Fatalf("read original PPPIOCGUNIT: %v", err)
	}
	// Demand mode prevents the unconnected live unit's ordinary read-EOF from
	// obscuring the independent POLLHUP caused by native device destruction.
	if err := unix.IoctlSetPointerInt(unitFD, 0x40047459, 0x200); err != nil {
		t.Fatalf("set original SC_LOOP_TRAFFIC: %v", err)
	}
	poll := []unix.PollFd{{Fd: int32(unitFD), Events: unix.POLLIN}}
	if _, err := unix.Poll(poll, 0); err != nil {
		t.Fatal(err)
	}
	if poll[0].Revents&unix.POLLHUP != 0 {
		t.Fatal("live original already reports POLLHUP")
	}
	kernelCounterOtherNamespace(t, func() {
		backend := kernelCounterNativeBackend(t)
		kernelCounterReplacement(t, backend, created.Attrs().Name, index)
		got, err := kernelCounterExchange(socket, kernelCounterIndexRequest(unix.RTM_GETLINK, index))
		if err != nil {
			t.Fatalf("query through retained creator socket: %v", err)
		}
		if got == nil {
			t.Fatal("creator socket query returned no device")
		}
		if got.Type() != "ppp" {
			t.Fatalf("creator socket retargeted to caller device kind %q", got.Type())
		}
		rename := kernelCounterIndexRequest(unix.RTM_NEWLINK, index)
		rename.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated("zc-renamed")))
		if _, err := kernelCounterExchange(socket, rename); err != nil {
			t.Fatalf("rename through retained creator socket: %v", err)
		}
		got, err = kernelCounterExchange(socket, kernelCounterIndexRequest(unix.RTM_GETLINK, index))
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("renamed original has no query response")
		}
		if got.Attrs().Name != "zc-renamed" {
			t.Fatalf("original name = %q after rename", got.Attrs().Name)
		}
		if _, err := kernelCounterExchange(socket, kernelCounterIndexRequest(unix.RTM_DELLINK, index)); err != nil {
			t.Fatalf("delete through retained creator socket: %v", err)
		}
		kernelCounterDeadUnit(t, unitFD)
		label, err := unix.IoctlGetInt(unitFD, 0x80047456)
		if err != nil {
			t.Fatalf("read retained unit label after delete: %v", err)
		}
		if label != unit {
			t.Fatalf("retained unit label changed from %d to %d", unit, label)
		}
		if _, err := kernelCounterExchange(socket, kernelCounterIndexRequest(unix.RTM_GETLINK, index)); !errors.Is(err, unix.ENODEV) {
			t.Fatalf("deleted original lookup = %v, want ENODEV", err)
		}
		caller, err := netlink.LinkByIndex(index)
		if err != nil {
			t.Fatalf("caller replacement was affected by creator delete: %v", err)
		}
		if caller.Type() != "dummy" {
			t.Fatalf("caller device kind = %q, want dummy", caller.Type())
		}
	})
}

func kernelCounterNativeBackend(t *testing.T) iface.Backend {
	t.Helper()
	backend, err := iface.OpenBackend("kernel")
	if err != nil {
		t.Fatalf("open real kernel counter backend: %v", err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	return backend
}

func kernelCounterOpenPPP(t *testing.T) int {
	t.Helper()
	fd, err := unix.Open("/dev/ppp", unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("counter fixture cannot open /dev/ppp: %v; host PPP grant is required", err)
	}
	return fd
}

func kernelCounterOwner(t *testing.T) (ppp.SessionHandle, iface.Backend, netlink.Link, int) {
	t.Helper()
	backend := kernelCounterNativeBackend(t)
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	transport := kernelCounterTransport(t)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "zc-owner")
	if err != nil {
		t.Fatalf("create genuine original PPP pair: %v", err)
	}
	transferred := false
	t.Cleanup(func() {
		if transferred {
			return
		}
		if err := unit.Close(); err != nil {
			t.Error(err)
		}
		if err := channel.Close(); err != nil {
			t.Error(err)
		}
	})
	config, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatalf("snapshot genuine original PPP pair: %v", err)
	}
	// NewSession consumes every fresh original on every outcome. The unit's
	// descriptor below remains only the old fixture's read-only POLLHUP label;
	// this fixture never closes it or reconstructs a file wrapper afterward.
	unitFD := config.UnitFD
	transferred = true
	_, handle, err := pppkernel.NewSession(config)
	if err != nil {
		t.Fatalf("construct actual PPP session owner: %v", err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	link, err := netlink.LinkByIndex(int(handle.Identity().NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	return handle, backend, link, unitFD
}

func kernelCounterOriginal(t *testing.T, handle ppp.SessionHandle) {
	t.Helper()
	got, err := handle.Counters()
	if err != nil {
		t.Fatalf("live original counter read failed: %v", err)
	}
	// These roots isolate original traffic ownership. Diagnostic availability
	// is checked separately, so a measured zero need not equal Unknown's bits.
	for _, value := range []uint64{got.RxBytes, got.TxBytes, got.RxPackets, got.TxPackets} {
		if value != 0 {
			t.Fatalf("idle original counters = %+v, want its real zero traffic", got)
		}
	}
}

func kernelCounterReplacement(t *testing.T, backend iface.Backend, name string, index int) {
	t.Helper()
	link := &netlink.Dummy{}
	link.Name = name
	link.Index = index
	// Bind cleanup before native creation so every later setup failure still
	// releases this task's device in its original namespace.
	cleanup, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	var owned netlink.Link
	t.Cleanup(func() {
		defer cleanup.Close()
		if owned == nil {
			return
		}
		if err := cleanup.LinkDel(owned); err != nil {
			t.Errorf("delete task-owned replacement: %v", err)
		}
	})
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatalf("replacement fixture cannot create name=%s requested-index=%d: %v", name, index, err)
	}
	owned = link
	actual, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	owned = actual
	if index > 0 {
		if actual.Attrs().Index != index {
			t.Fatalf("replacement native index = %d, requested %d", actual.Attrs().Index, index)
		}
	}
	if err := netlink.LinkSetUp(actual); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open replacement traffic socket: %v", err)
	}
	defer func() {
		if err := unix.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	var frame [60]byte
	frame[0], frame[6], frame[12], frame[13] = 2, 2, 0x88, 0xb5
	if err := unix.Sendto(fd, frame[:], 0, &unix.SockaddrLinklayer{Ifindex: actual.Attrs().Index, Protocol: 0xb588, Halen: 6}); err != nil {
		t.Fatalf("send distinguishable replacement traffic: %v", err)
	}
	stats, err := backend.GetStats(name)
	if err != nil {
		t.Fatalf("replacement counter positive control: %v", err)
	}
	if stats == nil {
		t.Fatal("replacement counter control returned no snapshot")
	}
	if stats.TxBytes == 0 {
		t.Fatal("replacement counter control did not observe transmitted bytes")
	}
	if stats.TxPackets == 0 {
		t.Fatal("replacement counter control did not observe a transmitted packet")
	}
}

func kernelCounterOtherNamespace(t *testing.T, work func()) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	}()
	other, err := netns.New()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("create caller namespace: %v", err)
	}
	defer func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	}()
	defer func() {
		if err := netns.Set(original); err != nil {
			// Keep the thread locked: Go terminates it when this test goroutine
			// exits, rather than lending a foreign namespace to another test.
			t.Fatalf("restore creator namespace: %v", err)
		}
		runtime.UnlockOSThread()
	}()
	work()
}

func kernelCounterMoveNamespaces(t *testing.T) (netns.NsHandle, netns.NsHandle) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("retain creator namespace for move: %v", err)
	}
	t.Cleanup(func() {
		if err := original.Close(); err != nil {
			t.Error(err)
		}
	})
	defer func() {
		if err := netns.Set(original); err != nil {
			// A failed restore must not lend this thread to another test.
			t.Fatalf("restore creator namespace after target creation: %v", err)
		}
		runtime.UnlockOSThread()
	}()
	target, err := netns.New()
	if err != nil {
		t.Fatalf("create target namespace for real PPP device move: %v", err)
	}
	t.Cleanup(func() {
		if err := target.Close(); err != nil {
			t.Error(err)
		}
	})
	return original, target
}

func kernelCounterDeadUnit(t *testing.T, unitFD int) {
	t.Helper()
	poll := []unix.PollFd{{Fd: int32(unitFD), Events: unix.POLLIN}}
	if _, err := unix.Poll(poll, 3000); err != nil {
		t.Fatalf("poll original unit after native deletion: %v", err)
	}
	if poll[0].Revents&unix.POLLHUP == 0 {
		t.Fatalf("deleted original unit poll events = %#x, want POLLHUP", poll[0].Revents)
	}
}

func kernelCounterCreateRequest(unitFD int, name string) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(nl.NewIfInfomsg(unix.AF_UNSPEC))
	if name != "" {
		request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name)))
	}
	info := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
	info.AddRtAttr(nl.IFLA_INFO_KIND, nl.NonZeroTerminated("ppp"))
	data := info.AddRtAttr(nl.IFLA_INFO_DATA, nil)
	data.AddRtAttr(unix.IFLA_PPP_DEV_FD, nl.Uint32Attr(uint32(unitFD)))
	request.AddData(info)
	return request
}

func kernelCounterIndexRequest(operation, index int) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(operation, unix.NLM_F_ACK)
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = int32(index)
	request.AddData(message)
	return request
}

// One outstanding request, at most eight receive batches and an absolute
// deadline bound the probe. Every reply must come from the kernel and match
// this socket's port and the request sequence; echoes never replace the ACK.
func kernelCounterExchange(socket *nl.NetlinkSocket, request *nl.NetlinkRequest) (netlink.Link, error) {
	port, err := socket.GetPid()
	if err != nil {
		return nil, err
	}
	if err := socket.Send(request); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(3 * time.Second)
	var link netlink.Link
	acknowledged := false
	for range 8 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, unix.ETIMEDOUT
		}
		timeout := unix.NsecToTimeval(remaining.Nanoseconds())
		if err := socket.SetReceiveTimeout(&timeout); err != nil {
			return nil, err
		}
		messages, from, err := socket.Receive()
		if err != nil {
			return nil, err
		}
		if from.Pid != 0 {
			return nil, fmt.Errorf("rtnetlink sender = %d, want kernel", from.Pid)
		}
		for _, message := range messages {
			if message.Header.Seq != request.Seq {
				return nil, fmt.Errorf("rtnetlink sequence = %d, want %d", message.Header.Seq, request.Seq)
			}
			if message.Header.Pid != port {
				return nil, fmt.Errorf("rtnetlink reply port = %d, want %d", message.Header.Pid, port)
			}
			switch message.Header.Type {
			case unix.NLMSG_ERROR:
				if len(message.Data) < 4 {
					return nil, errors.New("short rtnetlink acknowledgment")
				}
				if code := int32(nl.NativeEndian().Uint32(message.Data)); code != 0 {
					return nil, unix.Errno(-code)
				}
				acknowledged = true
			case unix.RTM_NEWLINK:
				if len(message.Data) < unix.SizeofIfInfomsg {
					return nil, errors.New("short creator link response")
				}
				header := unix.NlMsghdr(message.Header)
				link, err = netlink.LinkDeserialize(&header, message.Data)
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unexpected rtnetlink message type %d", message.Header.Type)
			}
		}
		if acknowledged {
			needsLink := request.Type == unix.RTM_GETLINK
			if request.Flags&unix.NLM_F_ECHO != 0 {
				needsLink = true
			}
			if needsLink {
				if link == nil {
					continue
				}
			}
			return link, nil
		}
	}
	return nil, errors.New("rtnetlink response exceeded eight receive batches")
}

// Design: docs/features/interfaces.md -- creator-bound raw counter readers
// Related: counter_create_protocol_linux_test.go -- the ordered reply decoder

//go:build integration && linux

package ifacenetlink

import (
	"errors"
	"runtime"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/userns"
)

// TestCounterCreateKernelTraffic requires an actual CREATE|EXCL|ACK request to
// produce its real name/index and a useful reader that advances with traffic.
func TestCounterCreateKernelTraffic(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-traffic")
	counterCreateUpTraffic(t, creator, link, 1)
	first := counterCreateRead(t, read)
	if first.TxBytes == 0 {
		t.Fatal("created interface has no measured transmitted bytes")
	}
	if first.TxPackets == 0 {
		t.Fatal("created interface has no measured transmitted packets")
	}
	counterSendFrames(t, link.Attrs().Index, 2)
	second := counterCreateRead(t, read)
	if second.TxBytes <= first.TxBytes {
		t.Fatal("bound raw bytes did not advance with new real traffic")
	}
	if second.TxPackets <= first.TxPackets {
		t.Fatal("bound raw packets did not advance with new real traffic")
	}
}

// TestCounterCreateKernelExplicitIndex preserves a requested free native index
// while still requiring the actual matching echo and useful bound statistics.
func TestCounterCreateKernelExplicitIndex(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	request := counterCreateDummyRequest("cc-index")
	header, present := request.Data[0].(*nl.IfInfomsg)
	if !present {
		t.Fatal("dummy fixture has no native link header")
	}
	header.Index = 129
	created, read, attempted, err := counterCreateCall(t, backend, request)
	if created != nil {
		counterCreateCleanupLink(t, creator, created)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !attempted {
		t.Fatal("explicit-index creation reported no attempt")
	}
	if created == nil {
		t.Fatal("explicit-index creation returned no actual link")
	}
	if created.Attrs().Index != 129 {
		t.Fatalf("actual explicit index=%d, want 129", created.Attrs().Index)
	}
	if read == nil {
		t.Fatal("explicit-index creation returned no counter reader")
	}
	counterCreateUpTraffic(t, creator, created, 1)
	counterCreateRead(t, read)
}

// TestCounterCreateKernelUnnamed keeps the actual automatic name from its echo,
// rather than requiring a requested name or guessing one from the native index.
func TestCounterCreateKernelUnnamed(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	info := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
	info.AddRtAttr(unix.IFLA_INFO_KIND, nl.ZeroTerminated("dummy"))
	request.AddData(info)
	created, read, attempted, err := counterCreateCall(t, backend, request)
	if created != nil {
		counterCreateCleanupLink(t, creator, created)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !attempted {
		t.Fatal("automatic-name creation reported no native attempt")
	}
	if created == nil {
		t.Fatal("automatic-name creation returned no actual link")
	}
	if created.Attrs().Name == "" {
		t.Fatal("automatic-name creation lost its actual echo name")
	}
	if created.Attrs().Index <= 0 {
		t.Fatal("automatic-name creation lost its actual echo index")
	}
	actual, err := creator.LinkByIndex(created.Attrs().Index)
	if err != nil {
		t.Fatal(err)
	}
	if created.Attrs().Name != actual.Attrs().Name {
		t.Fatal("automatic-name receipt differs from the actual created device")
	}
	if read == nil {
		t.Fatal("automatic-name creation returned no bound reader")
	}
	counterCreateUpTraffic(t, creator, created, 1)
	counterCreateRead(t, read)
}

// TestCounterCreateSerializesBeforeLock observes every caller Len/Serialize
// invocation during a real creation and checks that its request stays intact.
func TestCounterCreateSerializesBeforeLock(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	template := counterCreateDummyRequest("cc-serialize")
	payload := &counterCreatePayload{
		wire:   append([]byte(nil), template.Serialize()[unix.NLMSG_HDRLEN:]...),
		source: &backend.counters,
	}
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(payload)
	header := request.NlMsghdr
	created, read, attempted, err := counterCreateCall(t, backend, request)
	if created != nil {
		counterCreateCleanupLink(t, creator, created)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !attempted {
		t.Fatal("useful creation lost its attempted native owner")
	}
	if created == nil {
		t.Fatal("useful creation returned no actual link")
	}
	if read == nil {
		t.Fatal("useful creation returned no counter reader")
	}
	if payload.lengths == 0 {
		t.Fatal("request budget did not inspect caller payload length")
	}
	if payload.serializations == 0 {
		t.Fatal("creation never serialized caller payload")
	}
	if payload.locked {
		t.Fatal("caller Len or Serialize ran under the counter source lock")
	}
	if request.NlMsghdr != header {
		t.Fatal("creator mutated the caller's request header")
	}
}

// TestCounterBoundKernelReceiptMutation keeps caller changes to a returned Link
// from retargeting the original reader or the task-owned cleanup descriptor.
func TestCounterBoundKernelReceiptMutation(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	created, read := counterCreateOwned(t, backend, creator, "cc-immutable")
	// Mutating the returned descriptor cannot retarget the bound reader or
	// the task-owned cleanup copy retained before publication to this test.
	index := created.Attrs().Index
	counterCreateUpTraffic(t, creator, created, 1)
	created.Attrs().Index = 0
	created.Attrs().Name = "cc-mutated"
	stats := counterCreateRead(t, read)
	if stats.TxPackets == 0 {
		t.Fatalf("actual creator index %d lost useful traffic after descriptor mutation", index)
	}
}

// TestCounterCreateKernelSendUncertainty forces the real socket's atomic Send
// to fail before an echo, retaining the attempted flag and identifiable cause.
func TestCounterCreateKernelSendUncertainty(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, _, _ := counterCreateNative(t)
	backend.counters.mu.Lock()
	err := backend.counters.open()
	backend.counters.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	fd := backend.counters.socket.GetFd()
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_SNDBUF, 4096); err != nil {
		t.Fatal(err)
	}
	size, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_SNDBUF)
	if err != nil {
		t.Fatal(err)
	}
	if size > monitorReceiveBufferBytes/2 {
		t.Fatalf("send-buffer fixture exceeds the bounded request budget: %d", size)
	}
	if size+256 > int(^uint16(0))-unix.SizeofRtAttr {
		t.Fatalf("send-buffer fixture exceeds one well-framed attribute: %d", size)
	}
	request := counterCreateDummyRequest("cc-unsent")
	request.AddData(nl.NewRtAttr(unix.IFLA_IFALIAS, make([]byte, size+256)))
	created, read, attempted, err := counterCreateCall(t, backend, request)
	if !errors.Is(err, unix.EMSGSIZE) {
		t.Fatalf("real Send failure=%v, want EMSGSIZE", err)
	}
	if !attempted {
		t.Fatal("failed real Send discarded attempted ownership")
	}
	if created != nil {
		t.Fatal("failed real Send invented a created-link receipt")
	}
	if read != nil {
		t.Fatal("failed real Send invented a counter reader")
	}
}

// TestCounterBoundKernelRenameKeepsOriginal distinguishes a renamed live owner
// from a larger real counter sample attached to its old reusable name.
func TestCounterBoundKernelRenameKeepsOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-rename")
	counterCreateUpTraffic(t, creator, link, 1)
	before := counterCreateRead(t, read)
	if err := creator.LinkSetName(link, "cc-original"); err != nil {
		t.Fatal(err)
	}
	replacement := counterCreateReplacement(t, creator, "cc-rename", 0)
	counterCreateOriginalSample(t, read, before, replacement)
}

// TestCounterBoundKernelCallerNamespaceKeepsOriginal preserves a useful read
// when an equal name and index in the caller's namespace identify another link.
func TestCounterBoundKernelCallerNamespaceKeepsOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-caller")
	counterCreateUpTraffic(t, creator, link, 1)
	before := counterCreateRead(t, read)
	other, destination := counterCreateOtherNamespace(t)
	if err := netns.Set(other); err != nil {
		t.Fatal(err)
	}
	replacement := counterCreateReplacement(t, destination, "cc-caller", link.Attrs().Index)
	counterCreateOriginalSample(t, read, before, replacement)
}

// TestCounterBoundKernelDeleteRefusesReplacement deletes before the first
// bound read, so a later snapshot cannot supply the original creation receipt.
func TestCounterBoundKernelDeleteRefusesReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-delete")
	counterCreateUpTraffic(t, creator, link, 1)
	counterCreateNativeStats(t, creator, link.Attrs().Index)
	index := link.Attrs().Index
	if err := creator.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	counterCreateReplacement(t, creator, "cc-delete", index)
	counterCreateRefusedSample(t, read)
}

// TestCounterBoundKernelMoveRefusesReplacement keeps the caller in the creator
// namespace while moving its live link out and reusing the old name and index.
func TestCounterBoundKernelMoveRefusesReplacement(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, original := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-move")
	counterCreateUpTraffic(t, creator, link, 1)
	other, destination := counterCreateOtherNamespace(t)
	if err := netns.Set(original); err != nil {
		t.Fatal(err)
	}
	index := link.Attrs().Index
	if err := creator.LinkSetNsFd(link, int(other)); err != nil {
		t.Fatal(err)
	}
	moved, err := destination.LinkByName("cc-move")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Type() != "dummy" {
		t.Fatalf("moved link kind=%q, want original dummy", moved.Type())
	}
	counterCreateCleanupLink(t, destination, moved)
	counterCreateAbsent(t, creator, "cc-move")
	counterCreateReplacement(t, creator, "cc-move", index)
	counterCreateRefusedSample(t, read)
}

// TestCounterBoundKernelHistoryLossPermanent keeps a lost receipt refused before
// and after an ordinary consumer reopens the source with a fresh generation.
func TestCounterBoundKernelHistoryLossPermanent(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	cases := []struct {
		name        string
		readerFirst bool
	}{
		{"old-reader-before-reopen", true},
		{"ordinary-consumer-before-old-reader", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			backend, creator, _ := counterCreateNative(t)
			link, read := counterCreateOwned(t, backend, creator, "cc-history")
			counterCreateUpTraffic(t, creator, link, 1)
			before := counterCreateRead(t, read)
			backend.counters.mu.Lock()
			generation := backend.counters.states[link.Attrs().Index].generation
			backend.counters.invalidate()
			backend.counters.mu.Unlock()
			if generation == 0 {
				t.Fatal("useful original reader has no confirmed generation")
			}
			if test.readerFirst {
				counterCreateRefusedSample(t, read)
				if backend.counters.socket != nil {
					t.Fatal("lost original reader reopened its source")
				}
			}
			fresh, err := backend.GetInterface(link.Attrs().Name)
			if err != nil {
				t.Fatal(err)
			}
			if fresh.CounterGeneration == 0 {
				t.Fatal("ordinary consumer reopened no confirmed generation")
			}
			if fresh.CounterGeneration == generation {
				t.Fatal("ordinary consumer reused a generation after lost history")
			}
			if fresh.Stats == nil {
				t.Fatal("ordinary consumer returned no useful raw sample")
			}
			if fresh.Stats.TxBytes < before.TxBytes {
				t.Fatal("ordinary consumer lost the original useful byte totals")
			}
			if fresh.Stats.TxPackets < before.TxPackets {
				t.Fatal("ordinary consumer lost the original useful packet totals")
			}
			if backend.counters.socket == nil {
				t.Fatal("ordinary consumer did not reopen its counter source")
			}
			counterCreateRefusedSample(t, read)
		})
	}
}

// TestCounterCreateKernelForeignNamespaceRefuses rejects a new creation in a
// foreign caller namespace without mutating either namespace or the old reader.
func TestCounterCreateKernelForeignNamespaceRefuses(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, _ := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-survivor")
	counterCreateUpTraffic(t, creator, link, 1)
	before := counterCreateRead(t, read)
	other, destination := counterCreateOtherNamespace(t)
	if err := netns.Set(other); err != nil {
		t.Fatal(err)
	}
	counterCreateAbsent(t, creator, "cc-refused")
	counterCreateAbsent(t, destination, "cc-refused")
	created, refusedReader, attempted, err := counterCreateCall(t, backend, counterCreateDummyRequest("cc-refused"))
	if err == nil {
		t.Fatal("foreign-source creation reported success")
	}
	if attempted {
		t.Fatal("foreign-source creation reached Send")
	}
	if created != nil {
		t.Fatal("foreign-source creation published a link receipt")
	}
	if refusedReader != nil {
		t.Fatal("foreign-source creation published a reader")
	}
	counterCreateAbsent(t, creator, "cc-refused")
	counterCreateAbsent(t, destination, "cc-refused")
	stats := counterCreateRead(t, read)
	if stats.TxBytes < before.TxBytes {
		t.Fatal("refused creation invalidated the original useful bytes")
	}
	if stats.TxPackets < before.TxPackets {
		t.Fatal("refused creation invalidated the original useful packets")
	}
}

// TestCounterCreateKernelNamespaceOverrideRefuses rejects a valid target-ns FD
// even when the current caller matches the source; its old reader stays useful.
func TestCounterCreateKernelNamespaceOverrideRefuses(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, creator, original := counterCreateNative(t)
	link, read := counterCreateOwned(t, backend, creator, "cc-kept")
	counterCreateUpTraffic(t, creator, link, 1)
	before := counterCreateRead(t, read)
	other, destination := counterCreateOtherNamespace(t)
	if err := netns.Set(original); err != nil {
		t.Fatal(err)
	}
	counterCreateAbsent(t, creator, "cc-directed")
	counterCreateAbsent(t, destination, "cc-directed")
	request := counterCreateDummyRequest("cc-directed")
	request.AddData(nl.NewRtAttr(unix.IFLA_NET_NS_FD, nl.Uint32Attr(uint32(other))))
	created, refusedReader, attempted, err := counterCreateCall(t, backend, request)
	if err == nil {
		t.Fatal("directed primary-namespace creation reported success")
	}
	if attempted {
		t.Fatal("directed primary-namespace creation reached Send")
	}
	if created != nil {
		t.Fatal("directed primary-namespace creation published a link receipt")
	}
	if refusedReader != nil {
		t.Fatal("directed primary-namespace creation published a reader")
	}
	counterCreateAbsent(t, creator, "cc-directed")
	counterCreateAbsent(t, destination, "cc-directed")
	stats := counterCreateRead(t, read)
	if stats.TxBytes < before.TxBytes {
		t.Fatal("directed creation damaged the original useful byte totals")
	}
	if stats.TxPackets < before.TxPackets {
		t.Fatal("directed creation damaged the original useful packet totals")
	}
}

func counterCreateNative(t *testing.T) (*netlinkBackend, *netlink.Handle, netns.NsHandle) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	t.Cleanup(func() { counterCreatorCloseFD(t, int(original)) })
	t.Cleanup(func() {
		if err := netns.Set(original); err != nil {
			t.Errorf("restore native creator namespace: %v", err)
			return
		}
		runtime.UnlockOSThread()
	})
	backend := &netlinkBackend{}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	creator, err := netlink.NewHandleAt(original, unix.NETLINK_ROUTE)
	if err != nil {
		counterCreatorPrimitiveError(t, "open creator namespace handle", err)
	}
	t.Cleanup(creator.Close)
	return backend, creator, original
}

func counterCreateOtherNamespace(t *testing.T) (netns.NsHandle, *netlink.Handle) {
	t.Helper()
	other, err := netns.New()
	if err != nil {
		counterCreatorPrimitiveError(t, "create another native namespace", err)
	}
	t.Cleanup(func() { counterCreatorCloseFD(t, int(other)) })
	destination, err := netlink.NewHandleAt(other, unix.NETLINK_ROUTE)
	if err != nil {
		counterCreatorPrimitiveError(t, "open target namespace handle", err)
	}
	t.Cleanup(destination.Close)
	return other, destination
}

func counterCreateDummyRequest(name string) *nl.NetlinkRequest {
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name)))
	info := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
	info.AddRtAttr(unix.IFLA_INFO_KIND, nl.ZeroTerminated("dummy"))
	request.AddData(info)
	return request
}

func counterCreateOwned(t *testing.T, backend *netlinkBackend, creator *netlink.Handle, name string) (netlink.Link, func() (*netlink.LinkStatistics, error)) {
	t.Helper()
	created, read, attempted, err := counterCreateCall(t, backend, counterCreateDummyRequest(name))
	if created != nil {
		counterCreateCleanupLink(t, creator, created)
	}
	if err != nil {
		t.Fatal(err)
	}
	if !attempted {
		t.Fatal("successful creation reported no native attempt")
	}
	if created == nil {
		t.Fatal("successful creation returned no link")
	}
	if created.Attrs().Index <= 0 {
		t.Fatal("successful creation returned no native index")
	}
	if created.Attrs().Name != name {
		t.Fatalf("actual created name=%q, want %q", created.Attrs().Name, name)
	}
	if read == nil {
		t.Fatal("successful creation returned no bound reader")
	}
	return created, read
}

func counterCreateCleanupLink(t *testing.T, handle *netlink.Handle, link netlink.Link) {
	t.Helper()
	if link.Attrs().Index <= 0 {
		t.Fatal("created link has no actual index for task-owned cleanup")
	}
	// Own a descriptor copy, because a caller may mutate the returned Link.
	// Every index this helper deletes belongs to this test's private namespace.
	owned := &netlink.Device{}
	owned.Index = link.Attrs().Index
	owned.Name = link.Attrs().Name
	t.Cleanup(func() {
		err := handle.LinkDel(owned)
		if errors.Is(err, unix.ENODEV) {
			return
		}
		if errors.Is(err, unix.ENOENT) {
			return
		}
		if errors.Is(err, unix.ESRCH) {
			return
		}
		if err != nil {
			t.Errorf("release task-owned creator link: %v", err)
		}
	})
}

func counterCreateUpTraffic(t *testing.T, handle *netlink.Handle, link netlink.Link, frames int) {
	t.Helper()
	if err := handle.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	counterSendFrames(t, link.Attrs().Index, frames)
}

func counterCreateRead(t *testing.T, read func() (*netlink.LinkStatistics, error)) *netlink.LinkStatistics {
	t.Helper()
	stats, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil {
		t.Fatal("confirmed bound reader returned no sample")
	}
	if stats.TxBytes == 0 {
		t.Fatal("useful original reader returned no transmitted bytes")
	}
	if stats.TxPackets == 0 {
		t.Fatal("useful original reader returned no transmitted packets")
	}
	return stats
}

func counterCreateNativeStats(t *testing.T, handle *netlink.Handle, index int) *netlink.LinkStatistics {
	t.Helper()
	link, err := handle.LinkByIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	stats := link.Attrs().Statistics
	if stats == nil {
		t.Fatal("real native positive control returned no statistics")
	}
	if stats.TxPackets == 0 {
		t.Fatal("real native positive control sent no packets")
	}
	if stats.TxBytes == 0 {
		t.Fatal("real native positive control sent no bytes")
	}
	return stats
}

func counterCreateReplacement(t *testing.T, handle *netlink.Handle, name string, index int) *netlink.LinkStatistics {
	t.Helper()
	link := &netlink.Dummy{}
	link.Name = name
	link.Index = index
	if err := handle.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	counterCreateCleanupLink(t, handle, link)
	counterCreateUpTraffic(t, handle, link, 32)
	return counterCreateNativeStats(t, handle, link.Attrs().Index)
}

func counterCreateOriginalSample(t *testing.T, read func() (*netlink.LinkStatistics, error), before, replacement *netlink.LinkStatistics) {
	t.Helper()
	if replacement.TxBytes <= before.TxBytes {
		t.Fatal("replacement bytes do not distinguish it from the original")
	}
	if replacement.TxPackets <= before.TxPackets {
		t.Fatal("replacement packets do not distinguish it from the original")
	}
	stats := counterCreateRead(t, read)
	if stats.TxBytes < before.TxBytes {
		t.Fatal("original useful bytes went backwards")
	}
	if stats.TxPackets < before.TxPackets {
		t.Fatal("original useful packets went backwards")
	}
	if stats.TxBytes >= replacement.TxBytes {
		t.Fatal("bound reader returned the replacement's bytes")
	}
	if stats.TxPackets >= replacement.TxPackets {
		t.Fatal("bound reader returned the replacement's packets")
	}
}

func counterCreateRefusedSample(t *testing.T, read func() (*netlink.LinkStatistics, error)) {
	t.Helper()
	stats, err := read()
	if err == nil {
		t.Fatal("ended original reader accepted a replacement")
	}
	if stats != nil {
		t.Fatal("ended original reader published replacement counters")
	}
}

func counterCreateAbsent(t *testing.T, handle *netlink.Handle, name string) {
	t.Helper()
	_, err := handle.LinkByName(name)
	if err == nil {
		t.Fatalf("unexpected link %q exists", name)
	}
	if _, absent := errors.AsType[netlink.LinkNotFoundError](err); !absent {
		t.Fatalf("cannot establish %q absence: %v", name, err)
	}
}

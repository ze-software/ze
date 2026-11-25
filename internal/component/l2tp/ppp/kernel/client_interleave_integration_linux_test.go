//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- original-file attachment races.
// Related: session_pair_interleave_linux.go -- bounded AFTER-live-check native seam.
// Requires the proved original PPPoX supplier and final Root0010/0011 runtime.

package kernel

import (
	"errors"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/diagnostic"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// This fixture calls the actual registered unit and PPPoX channel producers;
// private fields and regular files never manufacture a native grant here.
func clientNativeInterleaveInputs(t *testing.T, name string, sid uint16) (*ClientSession,
	*CreatedUnit, *CreatedChannel, *UnitCreator, netlink.Link) {
	t.Helper()
	backend, err := iface.OpenBackend("kernel")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	creator, err := NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	unit, err := creator.Create(name)
	if unit != nil {
		t.Cleanup(func() {
			if err := unit.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if unit == nil {
		t.Fatal("actual unit producer returned no owner")
	}
	link, err := netlink.LinkByIndex(int(unit.Identity().NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	access := &netlink.Dummy{Name: name + "a"}
	if err := netlink.LinkAdd(access); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(access); err != nil {
			t.Error(err)
		}
	})
	if err := netlink.LinkSetUp(access); err != nil {
		t.Fatal(err)
	}
	transportFD, err := unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	// This is the single wrapper of the freshly allocated original socket.
	// It MUST remain owned through channel capture and client cleanup.
	transport := os.NewFile(uintptr(transportFD), "ppp.native.interleave.pppox")
	if transport == nil {
		closeErr := unix.Close(transportFD)
		t.Fatalf("retain fresh PPPoX socket: %v", errors.Join(errors.New("original socket has no file owner"), closeErr))
	}
	t.Cleanup(func() {
		if err := transport.Close(); err != nil {
			t.Error(err)
		}
	})
	transportConnection, err := transport.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var connectErr error
	if err := transportConnection.Control(func(fd uintptr) {
		connectErr = unix.Connect(int(fd), &unix.SockaddrPPPoE{SID: sid,
			Remote: []byte{2, 0, 0, 0, 0, 1}, Dev: access.Name})
	}); err != nil {
		t.Fatal(err)
	}
	if connectErr != nil {
		t.Fatal(connectErr)
	}
	channel, err := CreatePPPChannel(transport)
	if channel != nil {
		t.Cleanup(func() {
			if err := channel.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if channel == nil {
		t.Fatal("actual channel producer returned no owner")
	}
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClientSession(cfg)
	if client != nil {
		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("actual client constructor returned no owner")
	}
	return client, unit, channel, creator, link
}

func clientNativeAttachmentWitness(channel *CreatedChannel) error {
	// Observation-only legacy EINVAL distinguishes absent attachment. Exact
	// expected-peer cleanup is intentionally idempotent and cannot do that.
	return channel.WithDescriptor(func(fd int) error {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.PPPIOCDISCONN, 0)
		if errno != 0 {
			return errno
		}
		return nil
	})
}

// The opposite control proves the native seam still executes the actual ioctl
// and a useful attachment survives the check/operation boundary unchanged.
func TestClientPairNativeAfterLiveControlAttachesOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	client, _, channel, _, _ := clientNativeInterleaveInputs(t, "zpair-origin", 81)
	var checked int
	client.ops.connect = func(pair *claimedSessionPair) error {
		return connectOriginalPairAfterLive(pair, func() error { checked++; return nil })
	}
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if checked != 1 {
		t.Fatalf("between-operation control ran %d times", checked)
	}
	if err := clientNativeAttachmentWitness(channel); err != nil {
		t.Fatalf("no useful original attachment: %v", err)
	}
	if err := connectOriginalSessionPair(client.pair); err != nil {
		t.Fatalf("exact original reconnect: %v", err)
	}
	if err := disconnectOriginalSessionPair(client.pair); err != nil {
		t.Fatalf("exact original-pair cleanup: %v", err)
	}
	if err := clientNativeAttachmentWitness(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("cleanup left attachment: %v", err)
	}
}

// The actually registered check probes the running kernel before an actual
// pair attaches, changes MRU and applies independent family modes successfully.
// This is the genuine supported opposite to the injected runner refusals.
func TestClientPairNativeRegisteredDoctorSupportedPair(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	var checks int
	for _, check := range []diagnostic.DoctorCheck{originalAttachmentDoctorCheck(inspectOriginalAttachment, originalAttachmentKernel, func(diagnostic.DoctorCheckContext) (uint32, error) { return pppSupportSession, nil })} {
		if check.Name != originalAttachmentCheckName {
			continue
		}
		checks++
		if found := check.Check(diagnostic.DoctorCheckContext{}); len(found) != 0 {
			t.Fatalf("actual runtime prerequisite: %v", found)
		}
	}
	if checks != 1 {
		t.Fatalf("registered native dependency checks %d", checks)
	}
	client, _, channel, _, _ := clientNativeInterleaveInputs(t, "zpair-doctor", 83)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if err := client.SetIPv4Mode(NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := client.SetIPv6Mode(NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := clientNativeAttachmentWitness(channel); err != nil {
		t.Fatalf("supported probe did not attach: %v", err)
	}
	if err := connectOriginalSessionPair(client.pair); err != nil {
		t.Fatalf("supported original reconnect: %v", err)
	}
	if err := disconnectOriginalSessionPair(client.pair); err != nil {
		t.Fatalf("supported probe broke original pair: %v", err)
	}
	if err := clientNativeAttachmentWitness(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("supported cleanup left attachment: %v", err)
	}
}

// Deletion is synchronized AFTER userspace checked BOTH actual files and
// BEFORE the same production ioctl. Reusing the freed number cannot redirect
// attachment. The separate frozen BEFORE-Connect control remains unchanged.
func TestClientPairNativeDeleteBetweenLiveCheckAndAttachment(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	client, unit, channel, creator, link := clientNativeInterleaveInputs(t, "zpair-origin", 82)
	originalNumber := unit.UnitNumber()
	var replacement *CreatedUnit
	var checked int
	client.ops.connect = func(pair *claimedSessionPair) error {
		return connectOriginalPairAfterLive(pair, func() error {
			checked++
			if err := netlink.LinkDel(link); err != nil {
				return err
			}
			var err error
			replacement, err = creator.Create("zpair-reused")
			if replacement != nil {
				t.Cleanup(func() {
					if err := replacement.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			if err != nil {
				return err
			}
			if replacement == nil {
				return errors.New("native reuse prerequisite returned no original unit")
			}
			if replacement.UnitNumber() != originalNumber {
				return errors.New("native same-number reuse prerequisite was not established")
			}
			return nil
		})
	}
	if err := client.Connect(); !errors.Is(err, unix.ESTALE) {
		t.Fatalf("atomic dead-original refusal: %v", err)
	}
	if checked != 1 {
		t.Fatalf("delete did not reach the between-operation seam: %d", checked)
	}
	if replacement == nil {
		t.Fatal("native reuse prerequisite was not established")
	}
	if err := clientNativeAttachmentWitness(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("hidden replacement attachment: %v", err)
	}
	if _, err := replacement.Counters(); err != nil {
		t.Fatalf("refusal harmed replacement original: %v", err)
	}
}

// Query46's early native kind guard returns EBADF on this valid, retained
// unattached file. Query51 instead returns ESTALE for its valid sentinel tuple.
// Neither query may create/bind a unit/channel or accept the old int51 ABI.
func TestOriginalPairNativeFreshQueryEntryIsNonallocating(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	file, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	connection, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	queryChannel := func() error {
		var queryErr error
		err := connection.Control(func(fd uintptr) {
			owner := pppPairChannelOwner{abi: pppPairOwnerABI}
			queryErr = pppPairReadChannelOwner(int(fd), &owner)
		})
		if err != nil {
			return errors.Join(err, queryErr)
		}
		return queryErr
	}
	if err := queryChannel(); err != unix.EBADF { //nolint:errorlint // The bare kernel errno is the contract; a joined control error is a different failure.
		t.Fatalf("fresh query46 entry: %v", err)
	}
	var queryErr error
	if err := connection.Control(func(fd uintptr) {
		owner := pppPairUnitOwner{abi: pppPairOwnerABI, ifindex: 1, unit: 0}
		queryErr = pppPairReadUnitOwner(int(fd), &owner)
	}); err != nil {
		t.Fatal(err)
	}
	if queryErr != unix.ESTALE { //nolint:errorlint // The bare kernel errno from the unit-owner ioctl is the contract.
		t.Fatalf("fresh query51 kind guard: %v", queryErr)
	}
	if err := queryChannel(); err != unix.EBADF { //nolint:errorlint // The bare kernel errno is the contract; a joined control error is a different failure.
		t.Fatalf("query bound the fresh file: %v", err)
	}
}

// Retired native originals still permit SAME-pair cleanup. Fresh live-only
// polling/querying would mask the kernel's cleanup contract and leave custody.
func TestOriginalPairNativeDisconnectRetiredOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	client, _, channel, _, link := clientNativeInterleaveInputs(t, "zpair-retired", 84)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkDel(link); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := disconnectOriginalSessionPair(client.pair); err != nil {
			t.Fatalf("retired original cleanup: %v", err)
		}
	}
	if err := clientNativeAttachmentWitness(channel); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("retired cleanup left attachment: %v", err)
	}
}

// Wrong, unattached and attached-only files reach the SAME production peer
// serializer. Their native refusals must preserve the useful existing pair.
func TestOriginalPairNativePeerFileKindsPreserveAttachment(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	client, unit, channel, _, _ := clientNativeInterleaveInputs(t, "zpair-kind", 85)
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	_, other, _, _, _ := clientNativeInterleaveInputs(t, "zpair-other", 86)
	for _, path := range []string{"/dev/ppp", "/dev/null"} {
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		connection, err := file.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		var operationErr error
		err = connection.Control(func(peerFD uintptr) {
			operationErr = channel.WithDescriptor(func(channelFD int) error {
				return pppPairPeerOperation(channelFD, pppPairDisconnect, int(peerFD))
			})
		})
		if err != nil {
			t.Fatal(err)
		}
		if !errors.Is(operationErr, unix.EBADF) {
			t.Fatalf("wrong peer %s: %v", path, operationErr)
		}
	}
	err := other.WithDescriptor(func(peerFD int) error {
		return channel.WithDescriptor(func(channelFD int) error {
			return pppPairPeerOperation(channelFD, pppPairDisconnect, peerFD)
		})
	})
	if !errors.Is(err, unix.ESTALE) {
		t.Fatalf("wrong genuine expected unit: %v", err)
	}
	attached, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := attached.Close(); err != nil {
			t.Error(err)
		}
	})
	connection, err := attached.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var operationErr error
	if err := connection.Control(func(peerFD uintptr) {
		if err := unix.IoctlSetPointerInt(int(peerFD), 0x4004743d, unit.UnitNumber()); err != nil {
			operationErr = err
			return
		}
		operationErr = channel.WithDescriptor(func(channelFD int) error {
			return pppPairPeerOperation(channelFD, pppPairDisconnect, int(peerFD))
		})
	}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(operationErr, unix.ESTALE) {
		t.Fatalf("separately attached file gained creator authority: %v", operationErr)
	}
	if err := clientNativeAttachmentWitness(channel); err != nil {
		t.Fatalf("wrong-file refusal changed original attachment: %v", err)
	}
	if _, err := other.Counters(); err != nil {
		t.Fatalf("wrong-file refusal harmed other unit: %v", err)
	}
	if err := connectOriginalSessionPair(client.pair); err != nil {
		t.Fatal(err)
	}
	if err := disconnectOriginalSessionPair(client.pair); err != nil {
		t.Fatal(err)
	}
}

// Design: docs/research/l2tpv2-ze-integration.md -- original PPP creation custody
// Related: internal/plugins/iface/netlink/counter_create_linux.go -- ordered ECHO/ACK
// Linux: drivers/net/ppp/ppp_generic.c -- ppp_nl_newlink, ppp_poll

//go:build linux

package kernel

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// linkCounterCreator is deliberately a capability of the injected backend.
// This leaf does not import iface, select a backend, or reopen a global owner.
type linkCounterCreator interface {
	CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
}

// NewUnitCreator refuses an unsupported backend before any native allocation.
// The exact supplied backend owns the subscribed socket and must outlive reads.
func NewUnitCreator(backend InterfaceOperations) (*UnitCreator, error) {
	if nilSessionDependency(backend) {
		return nil, errors.New("ppp: missing original unit backend")
	}
	if !reflect.TypeOf(backend).Comparable() {
		return nil, errors.New("ppp: original unit backend has no stable owner identity")
	}
	source, ok := backend.(linkCounterCreator)
	if !ok {
		return nil, errors.New("ppp: backend cannot bind counters to original unit creation")
	}
	return &UnitCreator{create: func(name string) (*CreatedUnit, error) {
		unit, err := createOriginalPPPUnit(source, name)
		if unit != nil {
			unit.backend = backend
		}
		return unit, err
	}}, nil
}

func createOriginalPPPUnit(source linkCounterCreator, name string) (unit *CreatedUnit, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if len(name) >= unix.IFNAMSIZ || strings.IndexByte(name, 0) >= 0 {
		return nil, errors.New("ppp: original unit name is not a native interface name")
	}
	file, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("ppp: open original unit file: %w", err)
	}
	// Once open succeeds, the result retains the actual file on EVERY outcome.
	// A timeout or missing echo may still have created its unit. The original
	// file remains the cleanup authority even when no identity was received.
	unit = &CreatedUnit{file: file, number: -1, poll: pollOriginalPPPUnit}
	unit.self = unit
	defer func() {
		if failure := recover(); failure != nil {
			unit.creationRejection = nil
			err = fmt.Errorf("ppp: original unit creation panicked: %w", sessionPanicFailure(failure))
		}
	}()
	unit.namespace, err = captureSessionNamespace()
	if err != nil {
		return unit, err
	}
	connection, err := file.SyscallConn()
	if err != nil {
		return unit, err
	}
	var createErr error
	if err := connection.Control(func(fd uintptr) {
		// A provider panic cannot establish that Send was never attempted.
		// Normal return replaces this conservative fact with its real receipt.
		unit.attempted = true
		link, read, attempted, err := source.CreateLinkCounterReader(originalPPPUnitRequest(int(fd), name))
		unit.attempted = attempted
		if !nilSessionDependency(link) {
			if attrs := link.Attrs(); attrs != nil && attrs.Index > 0 && uint64(attrs.Index) <= uint64(^uint32(0)) {
				unit.identity = ppp.SessionIdentity{InterfaceName: attrs.Name, NativeIndex: uint32(attrs.Index)}
			}
		}
		if err != nil {
			unit.creationRejection = retainOriginalCreationRejection(link, attempted, err)
			createErr = fmt.Errorf("ppp: original unit creation: %w", err)
			return
		}
		if !attempted || nilSessionDependency(link) || read == nil {
			createErr = errors.New("ppp: original unit creation returned no complete receipt")
			return
		}
		if unit.identity.NativeIndex == 0 || unit.identity.InterfaceName == "" || link.Type() != "ppp" {
			createErr = errors.New("ppp: original creation echo does not identify a PPP unit")
			return
		}
		if name != "" && unit.identity.InterfaceName != name {
			createErr = errors.New("ppp: original creation echo has another unit name")
			return
		}
		number, err := ioctlGetInt(int(fd), pppiocGUnit)
		if err != nil {
			createErr = fmt.Errorf("ppp: original unit number: %w", err)
			return
		}
		if number < 0 {
			createErr = errors.New("ppp: original unit has an invalid number")
			return
		}
		unit.number = number
		unit.readCounters = func() (ppp.SessionCounters, error) {
			stats, err := read()
			if err != nil {
				return ppp.SessionCounters{}, err
			}
			if stats == nil {
				return ppp.SessionCounters{}, errors.New("ppp: original unit counter reply has no statistics")
			}
			return ppp.SessionCounters{
				RxBytes: stats.RxBytes, TxBytes: stats.TxBytes,
				RxPackets: stats.RxPackets, TxPackets: stats.TxPackets,
				RxErrors: ppp.MeasuredCounter(stats.RxErrors), TxErrors: ppp.MeasuredCounter(stats.TxErrors),
				RxDropped: ppp.MeasuredCounter(stats.RxDropped), TxDropped: ppp.MeasuredCounter(stats.TxDropped),
			}, nil
		}
	}); err != nil {
		return unit, err
	}
	if createErr != nil {
		return unit, createErr
	}
	if err := pollOriginalPPPUnit(file); err != nil {
		return unit, err
	}
	unit.ready = true
	return unit, nil
}

func originalPPPUnitRequest(unitFD int, name string) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK|unix.NLM_F_ECHO)
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

// ppp_poll's POLLHUP is pf->dead; an ordinary unattached live unit instead
// reports POLLIN for EOF. Do not consume that EOF or mutate SC_LOOP_TRAFFIC.
// Each Control lease ends before a native counter query can block.
func pollOriginalPPPUnit(file *os.File) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var pollErr error
	if err := connection.Control(func(fd uintptr) {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		for {
			_, pollErr = unix.Poll(fds, 0)
			if !errors.Is(pollErr, unix.EINTR) {
				break
			}
		}
		if pollErr == nil && fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			pollErr = fmt.Errorf("ppp: original unit file is no longer live (poll %#x)", fds[0].Revents)
		}
	}); err != nil {
		return err
	}
	return pollErr
}

// Append ONLY retainOriginalCreationRejection to created_unit_linux.go.
// errors and the netlink library already belong to that existing Linux file's imports.
func retainOriginalCreationRejection(link netlink.Link, attempted bool, cause error) nativeLinkCreationRejection {
	if !attempted {
		return nil
	}
	if !nilSessionDependency(link) {
		return nil
	}
	var rejection nativeLinkCreationRejection
	if !errors.As(cause, &rejection) {
		return nil
	}
	if !rejection.NativeLinkCreationRejected() {
		return nil
	}
	return rejection
}

//go:build linux

// Design: docs/labs/l2tp-interop.md -- syscall and ioctl boundary
// Related: l2tpdiag_linux.go -- operation order and report construction

package deployment

import (
	"os"
	"unsafe" //nolint:gosec // Linux PPP ioctls and sockaddr_pppol2tp have no typed Go wrappers

	"github.com/vishvananda/netlink"
	nl "github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const procPPPoL2TPPath = "/proc/net/pppol2tp"

type systemL2TPDiagnosticOps struct {
	calls []string
	files []*os.File
}

func newSystemL2TPDiagnosticOps() *systemL2TPDiagnosticOps {
	return &systemL2TPDiagnosticOps{calls: []string{}, files: []*os.File{}}
}

func (o *systemL2TPDiagnosticOps) record(call string) { o.calls = append(o.calls, call) }

func (o *systemL2TPDiagnosticOps) Operations() []string {
	return append([]string(nil), o.calls...)
}

func (o *systemL2TPDiagnosticOps) Socket(domain, kind, protocol int) (int, error) {
	operation := "socket inet"
	if domain == afPPPOX {
		operation = "socket pppox"
	}
	o.record(operation)
	return unix.Socket(domain, kind, protocol)
}

func (o *systemL2TPDiagnosticOps) setReusePort(fd int) error {
	o.record("setsockopt reuseport")
	return unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
}

func (o *systemL2TPDiagnosticOps) bindIPv4(fd int, address [4]byte, port uint16) error {
	o.record("bind udp")
	return unix.Bind(fd, &unix.SockaddrInet4{Port: int(port), Addr: address})
}

func (o *systemL2TPDiagnosticOps) Family(name string) (uint16, error) {
	o.record("resolve family")
	family, err := netlink.GenlFamilyGet(name)
	if err != nil {
		return 0, err
	}
	return family.ID, nil
}

func (o *systemL2TPDiagnosticOps) Execute(operation string, request *nl.NetlinkRequest) ([][]byte, error) {
	o.record(operation)
	return request.Execute(unix.NETLINK_GENERIC, 0)
}

func (o *systemL2TPDiagnosticOps) Connect(fd int, address []byte) error {
	o.record("connect pppox")
	//nolint:gosec // G103: address is the packed sockaddr owned by the caller and outlives connect(2).
	_, _, errno := unix.RawSyscall(unix.SYS_CONNECT, uintptr(fd), uintptr(unsafe.Pointer(&address[0])), uintptr(len(address)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (o *systemL2TPDiagnosticOps) ioctlGetInt(fd int, request uint) (int, error) {
	o.record("ioctl get channel")
	var value int32
	//nolint:gosec // G103: value is local and this PPP ioctl takes an int pointer.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(unsafe.Pointer(&value)))
	if errno != 0 {
		return 0, errno
	}
	return int(value), nil
}

func (o *systemL2TPDiagnosticOps) ioctlSetInt(fd int, request uint, value int) error {
	if request == pppiocAttChan {
		o.record("ioctl attach channel")
	} else {
		o.record("ioctl connect unit")
	}
	argument := int32(value)
	//nolint:gosec // G103: argument is local and this PPP ioctl takes an int pointer.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(unsafe.Pointer(&argument)))
	if errno != 0 {
		return errno
	}
	return nil
}

func (o *systemL2TPDiagnosticOps) ioctlGetSetInt(fd int, request uint, value int) (int, error) {
	o.record("ioctl new unit")
	argument := int32(value)
	//nolint:gosec // G103: argument is local and this PPP ioctl updates an int pointer.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(request), uintptr(unsafe.Pointer(&argument)))
	if errno != 0 {
		return 0, errno
	}
	return int(argument), nil
}

func (o *systemL2TPDiagnosticOps) openPPP() (int, error) {
	o.record("open /dev/ppp")
	file, err := os.OpenFile(devPPP, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	// Both files MUST remain open until the command exits. An early close
	// detaches the channel or destroys the transient PPP unit. Process exit
	// MUST close both descriptors, as it did for the standalone producer.
	o.files = append(o.files, file)
	return int(file.Fd()), nil
}

func (o *systemL2TPDiagnosticOps) procPPPoL2TP() string {
	var call textbuf.Buffer
	o.record(call.Str("read ").Str(procPPPoL2TPPath).String())
	data, err := os.ReadFile(procPPPoL2TPPath)
	return procPPPoL2TPDiagnosticText(data, err)
}

func procPPPoL2TPDiagnosticText(data []byte, err error) string {
	var tb textbuf.Buffer
	if err != nil {
		return tb.Str("  (read ").Str(procPPPoL2TPPath).Str(": ").Err(err).Str(")\n").String()
	}
	if len(data) == 0 {
		return tb.Str("  (").Str(procPPPoL2TPPath).Str(" is empty)\n").String()
	}
	return string(data)
}

func (o *systemL2TPDiagnosticOps) devPPPText() string {
	var call textbuf.Buffer
	o.record(call.Str("stat ").Str(devPPP).String())
	info, err := os.Stat(devPPP)
	if err != nil {
		return devPPPDiagnosticText(0, 0, unix.Stat_t{}, err, nil)
	}
	if info.Mode()&os.ModeDevice == 0 {
		return devPPPDiagnosticText(info.Mode(), info.Size(), unix.Stat_t{}, nil, nil)
	}
	var stat unix.Stat_t
	statErr := unix.Stat(devPPP, &stat)
	return devPPPDiagnosticText(info.Mode(), info.Size(), stat, nil, statErr)
}

func devPPPDiagnosticText(mode os.FileMode, size int64, stat unix.Stat_t, infoErr, statErr error) string {
	var tb textbuf.Buffer
	if infoErr != nil {
		return tb.Str("  (stat ").Str(devPPP).Str(": ").Err(infoErr).Str(")\n").String()
	}
	tb.Str("  ").Str(devPPP).Str(" mode=").Str(mode.String()).Str(" size=").Int(size).Byte('\n')
	if mode&os.ModeDevice == 0 {
		return tb.Str("  ").Str(devPPP).Str(" is not a device node, so the PPP channel ioctls have nothing to talk to\n").String()
	}
	if statErr != nil {
		return tb.Str("  (unix.Stat ").Str(devPPP).Str(": ").Err(statErr).Str(")\n").String()
	}
	return tb.Str("  ").Str(devPPP).Str(" device ").Uint32(unix.Major(stat.Rdev)).Byte(':').Uint32(unix.Minor(stat.Rdev)).
		Str(" uid=").Uint32(stat.Uid).Str(" gid=").Uint32(stat.Gid).Byte('\n').String()
}

func (o *systemL2TPDiagnosticOps) Link(name string) (diagnosticLink, error) {
	var call textbuf.Buffer
	o.record(call.Str("link ").Str(name).String())
	link, err := netlink.LinkByName(name)
	if err != nil {
		return diagnosticLink{}, err
	}
	attributes := link.Attrs()
	return diagnosticLink{
		Name: attributes.Name, Index: attributes.Index, Type: link.Type(), MTU: attributes.MTU,
		Flags: attributes.Flags, OperState: attributes.OperState.String(),
	}, nil
}

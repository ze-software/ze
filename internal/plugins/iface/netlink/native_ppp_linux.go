// Design: docs/features/interfaces.md -- original PPP file guarded native writers
// Related: native_ppp_request_linux.go -- guarded request serializers
// Related: native_ppp_transport_linux.go -- original send and complete ACK custody

//go:build linux

package ifacenetlink

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

const (
	nativePPPABI              = 1
	nativePPPOwnerQuery       = 0xc0147433
	nativePPPSetlink          = 1 << 0
	nativePPPAddr4            = 1 << 1
	nativePPPAddr6            = 1 << 2
	nativePPPRoute4           = 1 << 3
	nativePPPRoute6           = 1 << 4
	nativePPPAlias            = 1 << 5
	nativePPPExactAddressPair = 1 << 10
	nativePPPWriterLimit      = 1 << 20
)

type nativePPPOwnerReply struct {
	abi      uint32
	features uint32
	index    int32
	unit     int32
	reserved uint32
}

// nativePPPResources owns every writer produced by this exact backend. It
// creates no goroutine: Close synchronously joins each writer's bounded I/O.
// Its valid zero value admits bindings until close permanently retires it.
type nativePPPResources struct {
	mu       sync.Mutex
	closed   atomic.Bool
	writers  map[*nativePPPWriter]struct{}
	closeErr error
	// Observer sees the actual minted file before native Bind/name/probe. It
	// supplies no descriptor, I/O result, feature receipt or cleanup authority.
	afterSocket func(*os.File)
}

// nativePPPWriter retains one original-namespace socket. Successful binding
// alone grants features from its immutable positive original-file probe;
// failed construction can retain cleanup custody with no mutation authority.
// Its mutex serializes writes, settlement and close; it MUST NOT close the
// borrowed original unit. Nil/zero writers refuse use.
type nativePPPWriter struct {
	mu       sync.Mutex
	parent   *nativePPPResources
	owner    iface.NativePPPUnitOwner
	file     *os.File
	socket   int
	port     uint32
	index    int
	unit     int32
	features uint32
	sequence uint32
	pending  *nativePPPTransaction
	// closed prevents mutation and another os.File.Close attempt. Only a nil
	// socketCloseErr after that attempt proves native socket custody ended.
	closed         bool
	closeErr       error
	socketCloseErr error
	// Tests may pause an actual native boundary; observers never supply IO,
	// descriptors, feature receipts, ACKs or a cleanup authority.
	beforeSend func()
	beforeRead func()
}

// BindNativePPPUnit captures this backend's socket and a genuine positive
// query51 while Primary's existing joined helper holds the original namespace.
// The same thread creates the socket and probes the exact original creator.
// Later callers may be in another namespace; only the captured socket and
// retained original file reach the actual guarded native writer.
// A constructor failure with failed socket Close returns a refused partial
// writer; the caller MUST retain it for cleanup despite the returned error.
func (b *netlinkBackend) BindNativePPPUnit(owner iface.NativePPPUnitOwner, index int, unit int32) (iface.NativePPPWriter, error) {
	if owner == nil {
		return nil, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator owner is absent"))
	}
	value := reflect.ValueOf(owner)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator owner is typed nil"))
		}
	}
	if index <= 0 {
		return nil, errors.New("PPP original native index is not positive")
	}
	if int64(index) > int64(1<<31-1) {
		return nil, errors.New("PPP original native index exceeds signed native range")
	}
	if unit < 0 {
		return nil, errors.New("PPP original unit is negative")
	}
	resources := &b.nativePPPs
	resources.mu.Lock()
	defer resources.mu.Unlock()
	if resources.closed.Load() {
		return nil, errors.Join(iface.ErrNativePPPUnavailable, os.ErrClosed)
	}
	if len(resources.writers) >= nativePPPWriterLimit {
		return nil, errors.New("PPP original native writer limit reached")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	file, socket, port, err := newNativePPPSocket(resources.afterSocket)
	if file == nil {
		return nil, err
	}
	writer := &nativePPPWriter{parent: resources, owner: owner, file: file,
		socket: socket, port: port, index: index, unit: unit}
	var reply nativePPPOwnerReply
	if err == nil {
		reply, err = nativePPPProbe(owner, index, unit)
	}
	if err != nil {
		// Only this caller closes a post-open constructor failure. os.File's
		// logical retirement cannot prove the native socket was released.
		writer.closed = true
		writer.socketCloseErr = file.Close()
		writer.closeErr = errors.Join(err, writer.socketCloseErr)
		if writer.socketCloseErr == nil {
			return nil, writer.closeErr
		}
	} else {
		writer.features = reply.features
	}
	if resources.writers == nil {
		resources.writers = make(map[*nativePPPWriter]struct{})
	}
	resources.writers[writer] = struct{}{}
	return writer, writer.closeErr
}

func nativePPPProbe(owner iface.NativePPPUnitOwner, index int, unit int32) (nativePPPOwnerReply, error) {
	reply := nativePPPOwnerReply{abi: nativePPPABI, index: int32(index), unit: unit}
	called := false
	var probeErr error
	borrowErr := owner.WithDescriptor(func(fd int) error {
		if called {
			return errors.New("PPP original creator borrow invoked its operation twice")
		}
		called = true
		if fd < 0 {
			return errors.New("PPP original creator descriptor is negative")
		}
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), nativePPPOwnerQuery,
			uintptr(unsafe.Pointer(&reply))) //nolint:gosec // exact creator-only PPP owner UAPI pointer.
		if errno != 0 {
			probeErr = errno
			// These query51 outcomes name a missing kernel capability or a
			// missing/mismatched exact original. Other ioctl failures stay
			// ordinary errors; native write NACKs never use this branch.
			switch errno {
			case unix.ENOTTY, unix.EOPNOTSUPP, unix.ESTALE, unix.EBADF, unix.ENODEV, unix.ENXIO:
				probeErr = errors.Join(iface.ErrNativePPPUnavailable, errno)
			}
			return probeErr
		}
		return nil
	})
	if err := errors.Join(probeErr, borrowErr); err != nil {
		if errors.Is(borrowErr, os.ErrClosed) {
			return nativePPPOwnerReply{}, errors.Join(iface.ErrNativePPPUnavailable, err)
		}
		return nativePPPOwnerReply{}, err
	}
	if !called {
		return nativePPPOwnerReply{}, errors.New("PPP original creator probe did not borrow its file")
	}
	if reply.abi != nativePPPABI {
		return nativePPPOwnerReply{}, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator probe returned another ABI"))
	}
	if reply.index != int32(index) {
		return nativePPPOwnerReply{}, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator probe changed its captured index"))
	}
	if reply.unit != unit {
		return nativePPPOwnerReply{}, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator probe changed its captured unit"))
	}
	if reply.reserved != 0 {
		return nativePPPOwnerReply{}, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator probe returned nonzero reserved fields"))
	}
	if reply.features&nativePPPSetlink == 0 {
		return nativePPPOwnerReply{}, errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator probe lacks guarded SETLINK"))
	}
	return reply, nil
}

// newNativePPPSocket transfers every post-open file to the caller, including
// failures. The caller MUST attempt its sole Close and retain failed custody.
func newNativePPPSocket(afterSocket func(*os.File)) (*os.File, int, uint32, error) {
	socket, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_ROUTE)
	if err != nil {
		return nil, 0, 0, err
	}
	file := os.NewFile(uintptr(socket), "ze-original-ppp-netlink")
	if afterSocket != nil {
		afterSocket(file)
	}
	if err := unix.Bind(socket, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return file, socket, 0, err
	}
	address, err := unix.Getsockname(socket)
	if err != nil {
		return file, socket, 0, err
	}
	native, ok := address.(*unix.SockaddrNetlink)
	if !ok {
		return file, socket, 0, errors.New("PPP original socket has another address family")
	}
	return file, socket, native.Pid, nil
}

// mutate constructs and serializes before borrowing the original unit. The
// owner-fd payload is populated only during that borrow and erased on exit.
func (w *nativePPPWriter) mutate(ctx context.Context, required uint32, build func(int) (*nl.NetlinkRequest, error)) error {
	if w == nil {
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original writer is absent"))
	}
	if ctx == nil {
		return errors.New("PPP original write has no context")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.owner == nil {
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original writer is uninitialized"))
	}
	if w.pending != nil {
		return w.pending.unresolved(nil)
	}
	if w.closed {
		return errors.Join(iface.ErrNativePPPUnavailable, os.ErrClosed, w.closeErr)
	}
	if w.parent.closed.Load() {
		return errors.Join(iface.ErrNativePPPUnavailable, os.ErrClosed)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.features&required != required {
		return errors.Join(iface.ErrNativePPPUnavailable, fmt.Errorf("PPP original creator lacks native operation features %#x", required))
	}
	request, err := build(w.index)
	if err != nil {
		return err
	}
	if w.sequence == ^uint32(0) {
		return errors.New("PPP original request sequence is exhausted")
	}
	w.sequence++
	request.Seq = w.sequence
	request.Pid = w.port
	wire := request.Serialize()
	// Every serializer appends exactly one owner attribute last. Its four-byte
	// data is the only field populated while the original unit is borrowed.
	if len(wire) < unix.SizeofNlMsghdr+8 {
		return errors.New("PPP original request has no owner attribute")
	}
	transaction := &nativePPPTransaction{writer: w, wire: wire, header: request.NlMsghdr}
	w.pending = transaction
	called := false
	borrowErr := w.owner.WithDescriptor(func(fd int) error {
		if called {
			return errors.New("PPP original creator borrow invoked its send twice")
		}
		called = true
		if fd < 0 {
			return errors.New("PPP original creator descriptor is negative")
		}
		if uint64(fd) > uint64(1<<31-1) {
			return errors.New("PPP original creator descriptor exceeds signed native range")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		ownerPayload := wire[len(wire)-4:]
		nl.NativeEndian().PutUint32(ownerPayload, uint32(fd))
		defer clear(ownerPayload)
		if w.beforeSend != nil {
			w.beforeSend()
		}
		transaction.digest = sha256.Sum256(wire)
		transaction.entered = true
		transaction.sendErr = unix.Sendto(w.socket, wire, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK})
		return transaction.sendErr
	})
	transaction.cause = errors.Join(transaction.sendErr, borrowErr)
	if !transaction.entered {
		w.pending = nil
		if transaction.cause != nil {
			if errors.Is(borrowErr, os.ErrClosed) {
				return errors.Join(iface.ErrNativePPPUnavailable, transaction.cause)
			}
			return transaction.cause
		}
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original creator send did not borrow its file"))
	}
	// No unit descriptor borrow spans this receive, even when SEND's outcome
	// is uncertain. A matching complete ACK can settle that original attempt.
	return transaction.receive(ctx)
}

func (w *nativePPPWriter) Settle(ctx context.Context) error {
	if w == nil {
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original writer is absent"))
	}
	if ctx == nil {
		return errors.New("PPP original settlement has no context")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.owner == nil {
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original writer is uninitialized"))
	}
	if w.pending == nil {
		return w.closeErr
	}
	if w.observeUnitRetirementLocked() {
		return w.settleRetiredUnitLocked()
	}
	return w.pending.receive(ctx)
}

func (w *nativePPPWriter) closeOriginal() (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending != nil {
		if w.observeUnitRetirementLocked() {
			err := w.settleRetiredUnitLocked()
			return w.socketCloseErr == nil, err
		}
		return false, w.pending.unresolved(errors.New("PPP original socket cannot close before native settlement"))
	}
	if w.closed {
		return w.socketCloseErr == nil, w.closeErr
	}
	w.closed = true
	w.socketCloseErr = w.file.Close()
	w.closeErr = w.socketCloseErr
	return w.socketCloseErr == nil, w.closeErr
}

func (w *nativePPPWriter) Close() error {
	if w == nil {
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original writer is absent"))
	}
	if w.owner == nil {
		return errors.Join(iface.ErrNativePPPUnavailable, errors.New("PPP original writer is uninitialized"))
	}
	closed, err := w.closeOriginal()
	if closed {
		w.parent.mu.Lock()
		if _, retained := w.parent.writers[w]; retained {
			delete(w.parent.writers, w)
			w.parent.closeErr = errors.Join(w.parent.closeErr, err)
		}
		w.parent.mu.Unlock()
	}
	return err
}

// close returns settled only when no original PPP writer retains native
// custody. Historical unknown-write errors remain after successful socket Close
// and permit backend cleanup. A failed physical Close retains the exact writer;
// its retired os.File and numeric fd grant no authority for another attempt.
func (resources *nativePPPResources) close() (settled bool, result error) {
	resources.closed.Store(true)
	resources.mu.Lock()
	defer resources.mu.Unlock()
	settled = true
	result = resources.closeErr
	for writer := range resources.writers {
		closed, err := writer.closeOriginal()
		result = errors.Join(result, err)
		if !closed {
			settled = false
			continue
		}
		delete(resources.writers, writer)
		resources.closeErr = errors.Join(resources.closeErr, err)
	}
	return settled, result
}

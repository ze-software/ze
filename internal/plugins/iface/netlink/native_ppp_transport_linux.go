// Design: docs/features/interfaces.md -- original PPP native send and ACK custody
// Related: native_ppp_linux.go -- retained original unit, socket and backend

//go:build linux

package ifacenetlink

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

const (
	nativePPPWaitLimit       = 5 * time.Second
	nativePPPDatagramLimit   = 1 << 16
	nativePPPSettlementLimit = 64
)

// nativePPPTransaction is owned only by its writer under the writer mutex.
// Its exact header and original causes survive every uncertain wait. A poison
// means bytes were lost or invalid; another read cannot repair that evidence.
type nativePPPTransaction struct {
	writer    *nativePPPWriter
	wire      []byte
	header    unix.NlMsghdr
	digest    [sha256.Size]byte
	entered   bool
	sendErr   error
	cause     error
	nativeErr error
	waitErrs  []error
	poison    error
	turns     int
}

// nativePPPRejected can arise only after one complete validated datagram on
// the original socket supplies the matched kernel NACK. It proves a terminal
// kernel response, never absence of effects or rollback: a kernel operation can
// change native state before returning an errno. Domain cleanup/effect receipts
// remain with Primary even after transport pending clears. This type grants no
// safe retry, deletion, current-name, replacement or confirmed-absence authority.
type nativePPPRejected struct{ cause error }

func (rejected *nativePPPRejected) Error() string       { return rejected.cause.Error() }
func (rejected *nativePPPRejected) Unwrap() error       { return rejected.cause }
func (*nativePPPRejected) NativePPPWriteRejected() bool { return true }

func (transaction *nativePPPTransaction) unresolved(cause error) error {
	return errors.Join(iface.ErrNativePPPPending, transaction.cause,
		transaction.nativeErr, transaction.poison, errors.Join(transaction.waitErrs...), cause)
}

func (transaction *nativePPPTransaction) retainWait(cause error) error {
	transaction.waitErrs = append(transaction.waitErrs, cause)
	return transaction.unresolved(nil)
}

// receive waits only on the original socket and never re-enters Sendto or a
// unit descriptor borrow. An uncertain send may still yield its decisive ACK.
// The fixed turn limit bounds retained failure history; after it, only a future
// real original-unit terminal receipt may retire an unresolved transaction.
func (transaction *nativePPPTransaction) receive(ctx context.Context) error {
	if transaction.poison != nil {
		return transaction.unresolved(nil)
	}
	if transaction.turns >= nativePPPSettlementLimit {
		return transaction.unresolved(errors.New("PPP original settlement turn limit reached"))
	}
	transaction.turns++
	if err := ctx.Err(); err != nil {
		return transaction.retainWait(err)
	}
	deadline := time.Now().Add(nativePPPWaitLimit)
	if requested, ok := ctx.Deadline(); ok {
		if requested.Before(deadline) {
			deadline = requested
		}
	}
	writer := transaction.writer
	if writer.beforeRead != nil {
		writer.beforeRead()
	}
	var buffer [nativePPPDatagramLimit]byte
	// No producer other than the kernel can send to this unicast original
	// socket. The absolute deadline bounds even repeated EINTR/EAGAIN turns.
	for {
		if err := ctx.Err(); err != nil {
			return transaction.retainWait(err)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return transaction.retainWait(unix.ETIMEDOUT)
		}
		wait := min(remaining, 100*time.Millisecond)
		milliseconds := int((wait + time.Millisecond - 1) / time.Millisecond)
		poll := []unix.PollFd{{Fd: int32(writer.socket), Events: unix.POLLIN}}
		count, err := unix.Poll(poll, milliseconds)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return transaction.retainWait(err)
		}
		if count == 0 {
			continue
		}
		if poll[0].Revents&unix.POLLNVAL != 0 {
			return transaction.retainWait(unix.EBADF)
		}
		if poll[0].Revents&unix.POLLIN == 0 {
			return transaction.retainWait(fmt.Errorf("PPP original socket poll returned events %#x", poll[0].Revents))
		}
		length, _, flags, from, err := unix.Recvmsg(writer.socket, buffer[:], nil, unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if err != nil {
			if errors.Is(err, unix.ENOBUFS) {
				transaction.poison = err
				return transaction.unresolved(nil)
			}
			return transaction.retainWait(err)
		}
		if flags&unix.MSG_TRUNC != 0 {
			transaction.poison = errors.New("PPP original ACK datagram was truncated")
			return transaction.unresolved(nil)
		}
		sender, ok := from.(*unix.SockaddrNetlink)
		if !ok {
			transaction.poison = errors.New("PPP original ACK has another sender address family")
			return transaction.unresolved(nil)
		}
		if sender.Pid != 0 {
			transaction.poison = errors.New("PPP original ACK sender is not the kernel")
			return transaction.unresolved(nil)
		}
		nativeErr, parseErr := nativePPPParseACK(transaction.header, transaction.digest, buffer[:length])
		transaction.nativeErr = nativeErr
		if parseErr != nil {
			transaction.poison = parseErr
			return transaction.unresolved(nil)
		}
		writer.pending = nil
		if nativeErr != nil {
			return errors.Join(transaction.cause, errors.Join(transaction.waitErrs...), &nativePPPRejected{cause: nativeErr})
		}
		return errors.Join(transaction.cause, errors.Join(transaction.waitErrs...))
	}
}

// nativePPPParseACK validates the whole datagram, including trailing bytes and
// every message after a nominal ACK. Only a nil parse error establishes a final
// outcome; nativeErr is independently retained when a later message is invalid.
func nativePPPParseACK(original unix.NlMsghdr, digest [sha256.Size]byte, wire []byte) (nativeErr error, parseErr error) {
	acknowledgments := 0
	for offset := 0; offset < len(wire); {
		remaining := wire[offset:]
		if len(remaining) < unix.SizeofNlMsghdr {
			return nativeErr, errors.New("PPP original ACK has trailing truncated header bytes")
		}
		length := nl.NativeEndian().Uint32(remaining[:4])
		if length < unix.SizeofNlMsghdr {
			return nativeErr, errors.New("PPP original ACK header length is too short")
		}
		if uint64(length) > uint64(len(remaining)) {
			return nativeErr, errors.New("PPP original ACK message is truncated")
		}
		step := (uint64(length) + 3) &^ uint64(3)
		if step > uint64(len(remaining)) {
			return nativeErr, errors.New("PPP original ACK alignment is truncated")
		}
		kind := nl.NativeEndian().Uint16(remaining[4:6])
		if kind != unix.NLMSG_ERROR {
			return nativeErr, fmt.Errorf("PPP original ACK has unexpected message kind %d", kind)
		}
		flags := nl.NativeEndian().Uint16(remaining[6:8])
		if flags&unix.NLM_F_DUMP_INTR != 0 {
			return nativeErr, errors.New("PPP original ACK reports an interrupted request")
		}
		if nl.NativeEndian().Uint32(remaining[8:12]) != original.Seq {
			return nativeErr, errors.New("PPP original ACK names another sequence")
		}
		if nl.NativeEndian().Uint32(remaining[12:16]) != original.Pid {
			return nativeErr, errors.New("PPP original ACK names another port")
		}
		data := remaining[unix.SizeofNlMsghdr:length]
		if len(data) < 4+unix.SizeofNlMsghdr {
			return nativeErr, errors.New("PPP original ACK lacks the complete original header")
		}
		if nl.NativeEndian().Uint32(data[4:8]) != original.Len {
			return nativeErr, errors.New("PPP original ACK changed the original request length")
		}
		if nl.NativeEndian().Uint16(data[8:10]) != original.Type {
			return nativeErr, errors.New("PPP original ACK names another original request kind")
		}
		if nl.NativeEndian().Uint16(data[10:12]) != original.Flags {
			return nativeErr, errors.New("PPP original ACK changed the original request flags")
		}
		if nl.NativeEndian().Uint32(data[12:16]) != original.Seq {
			return nativeErr, errors.New("PPP original ACK names another original request sequence")
		}
		if nl.NativeEndian().Uint32(data[16:20]) != original.Pid {
			return nativeErr, errors.New("PPP original ACK names another original request port")
		}
		code := int32(nl.NativeEndian().Uint32(data[:4]))
		if code > 0 {
			return nativeErr, errors.New("PPP original ACK contains a positive errno")
		}
		if int64(code) < -4095 {
			return nativeErr, errors.New("PPP original ACK errno exceeds the native error range")
		}
		if code == 0 {
			// This original socket never enables EXT_ACK or CAP_ACK. The
			// pinned kernel produces only a capped, header-only success.
			if flags != unix.NLM_F_CAPPED {
				return nativeErr, errors.New("PPP original success ACK has unexpected flags")
			}
			if len(data) != 4+unix.SizeofNlMsghdr {
				return nativeErr, errors.New("PPP original success ACK has extra payload bytes")
			}
		}
		if code < 0 {
			// With neither socket option enabled, a NACK must echo exactly
			// the complete submitted request and carry no response flags.
			// Only its digest survived the original file borrow.
			if flags != 0 {
				return nativeErr, errors.New("PPP original NACK has unexpected flags")
			}
			if uint64(len(data)) != 4+uint64(original.Len) {
				return nativeErr, errors.New("PPP original NACK has another owner request payload length")
			}
			if sha256.Sum256(data[4:]) != digest {
				return nativeErr, errors.New("PPP original NACK echoed another original owner request")
			}
			// Preserve this independently validated native response even if
			// another message or malformed outer trailer follows it.
			nativeErr = errors.Join(nativeErr, unix.Errno(-int64(code)))
		}
		acknowledgments++
		if acknowledgments > 1 {
			return nativeErr, errors.New("PPP original ACK datagram contains duplicate acknowledgments")
		}
		offset += int(step)
	}
	if acknowledgments != 1 {
		return nativeErr, errors.New("PPP original ACK datagram is empty")
	}
	return nativeErr, nil
}

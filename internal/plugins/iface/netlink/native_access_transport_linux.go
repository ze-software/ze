// Design: docs/features/interfaces.md -- original native transaction custody
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

const nativeVLANReplyLimit = 65536
const nativeVLANDatagramLimit = 1 << 16

type nativeVLANReply uint8

const (
	nativeVLANReplyUnspecified nativeVLANReply = iota
	nativeVLANReplyACK
	nativeVLANReplyLink
	nativeVLANReplyDump
	nativeVLANReplyCreate
)

type nativeVLANSendPhase uint8

const (
	nativeVLANSendUnspecified nativeVLANSendPhase = iota
	nativeVLANSendPrepared
	nativeVLANSendEntered
	nativeVLANSendReturned
)

type nativeVLANReplyState interface{ nativeVLANReplyState() }
type nativeVLANReplyPending struct{ waitErr error }
type nativeVLANReplyComplete struct{ err error }

func (nativeVLANReplyPending) nativeVLANReplyState()  {}
func (nativeVLANReplyComplete) nativeVLANReplyState() {}

// nativeVLANTransaction owns the original request, socket and reply accumulator.
// It is constructed before send and retained by the transport and CREATE record.
// No caller expiry discards it, and no later request may drain its replies.
// Only the backend's one native worker may read or mutate this value.
type nativeVLANTransaction struct {
	transport    *nativeVLANTransport
	creator      *nativeVLANResource
	sequence     uint32
	kind         uint16
	mode         nativeVLANReply
	wire         []byte
	sendPhase    nativeVLANSendPhase
	replies      nativeVLANReplyState
	links        []netlink.Link
	acknowledged bool
	received     int
	poison       error
}

// nativeVLANTransport is confined to its original backend worker. Caller
// cancellation interrupts poll waits but MUST NOT close the socket or discard
// an original request still executing synchronously inside Linux RTNL sendto.
// An unresolved transaction allows only resumeOriginal, never another send.
type nativeVLANTransport struct {
	file              *os.File
	pid               uint32
	seq               uint32
	ctx               context.Context
	pending           *nativeVLANTransaction
	creator           *nativeVLANResource
	observeSent       func(uint16, uint32)         // A test may withhold an actual completed native send.
	observeReply      func(uint32)                 // Reports actual kernel messages; never supplies IO or authority.
	observeBeforeRead func(*nativeVLANTransaction) // A test may withhold the actual reader; never supplies IO or authority.
}

func newNativeVLANOps() (*nativeVLANOps, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_ROUTE)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "ze-native-vlan")
	fail := func(cause error) (*nativeVLANOps, error) { return nil, errors.Join(cause, file.Close()) }
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fail(err)
	}
	address, err := unix.Getsockname(fd)
	if err != nil {
		return fail(err)
	}
	local, ok := address.(*unix.SockaddrNetlink)
	if !ok {
		return fail(errors.New("native route socket has another address family"))
	}
	transport := &nativeVLANTransport{file: file, pid: local.Pid}
	return &nativeVLANOps{
		byName: transport.byName, byIndex: transport.byIndex, list: transport.list,
		add: transport.add, up: transport.up, modify: transport.modify, remove: transport.remove, alias: transport.alias,
		bind: transport.bind, bindCreate: transport.bindCreate, resumeOriginal: transport.resumeOriginal,
		timeout: func(time.Duration) error { return nil }, close: transport.close, transport: transport,
	}, nil
}

func (transport *nativeVLANTransport) bind(ctx context.Context) error {
	if ctx == nil {
		return errors.New("native request has no original context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if transport.pending != nil {
		return errors.Join(iface.ErrNativeAccessPending, errors.New("original native transaction has unconsumed replies"))
	}
	transport.ctx = ctx
	return nil
}

func (transport *nativeVLANTransport) bindCreate(record *nativeVLANResource) error {
	if record == nil {
		return errors.New("native CREATE has no original resource")
	}
	if record.owner == nil {
		return errors.New("native CREATE resource has no original backend")
	}
	if record.creation != nil {
		return errors.New("native CREATE resource already has its original transaction")
	}
	if transport.pending != nil {
		return iface.ErrNativeAccessPending
	}
	if transport.creator != nil {
		return errors.New("native CREATE transport already has a creator")
	}
	transport.creator = record
	return nil
}

func (transport *nativeVLANTransport) close() error {
	if transport.pending != nil {
		return errors.Join(iface.ErrNativeAccessPending, errors.New("original native transaction must settle before its socket closes"))
	}
	err := transport.file.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

// request is the sole send path. A cleanup read after a late successful send
// is still this exact transaction, not a new request or renewed caller budget.
func (transport *nativeVLANTransport) request(request *nl.NetlinkRequest, mode nativeVLANReply) ([]netlink.Link, error) {
	ctx := transport.ctx
	if ctx == nil {
		return nil, errors.New("native request has no original context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if transport.pending != nil {
		return nil, iface.ErrNativeAccessPending
	}
	switch mode {
	case nativeVLANReplyACK, nativeVLANReplyLink, nativeVLANReplyDump, nativeVLANReplyCreate:
	case nativeVLANReplyUnspecified:
		return nil, errors.New("native reply mode is unspecified")
	default:
		panic("BUG: original native caller supplied an unknown reply mode")
	}
	if transport.seq == ^uint32(0) {
		return nil, errors.New("original native request sequence is exhausted")
	}
	transport.seq++
	request.Seq = transport.seq
	request.Pid = transport.pid
	serialized := request.Serialize()
	if len(serialized) > nativeVLANDatagramLimit {
		return nil, errors.New("native VLAN request exceeds the datagram limit")
	}
	transaction := &nativeVLANTransaction{transport: transport, creator: transport.creator,
		sequence: request.Seq, kind: request.Type, mode: mode, wire: serialized,
		sendPhase: nativeVLANSendPrepared, replies: nativeVLANReplyPending{}}
	transport.pending = transaction
	if transaction.creator != nil {
		transaction.creator.creation = transaction
	}
	if err := transaction.send(ctx); err != nil {
		// A deadline before the syscall callback ran proves that no send occurred.
		// Once entered, an error alone cannot certify native absence.
		if transaction.sendPhase == nativeVLANSendPrepared {
			transaction.complete(err)
		}
		return transaction.links, errors.Join(err, ctx.Err())
	}
	if transport.observeSent != nil {
		transport.observeSent(transaction.kind, transaction.sequence)
	}
	if callerErr := ctx.Err(); callerErr != nil {
		// sendto may execute RTNL synchronously past the caller's expiry. Its file
		// and request are still owned. Consume only that original outcome while
		// the worker retains its uninterrupted creator fence; never stamp it late.
		cleanup, cancel := context.WithTimeout(context.Background(), nativeVLANOperationLimit)
		defer cancel()
		outcomeErr := transaction.resume(cleanup)
		return transaction.links, errors.Join(callerErr, outcomeErr)
	}
	outcomeErr := transaction.resume(ctx)
	return transaction.links, outcomeErr
}

func (transaction *nativeVLANTransaction) complete(err error) {
	if transaction.transport.pending != transaction {
		panic("BUG: completed native transaction lost its original owner")
	}
	transaction.replies = nativeVLANReplyComplete{err: err}
	transaction.transport.pending = nil
}

func (transaction *nativeVLANTransaction) nativeSuccess() bool {
	completed, ok := transaction.replies.(nativeVLANReplyComplete)
	if !ok {
		return false
	}
	return completed.err == nil
}

func (transaction *nativeVLANTransaction) waitError(err error) error {
	transaction.replies = nativeVLANReplyPending{waitErr: err}
	return errors.Join(iface.ErrNativeAccessPending, err)
}

// nativeVLANPollTurn MUST be stopped/joined before another turn changes the
// deadline. Its callback changes a poll deadline, never closes the socket.
func (transport *nativeVLANTransport) nativeVLANPollTurn(ctx context.Context) (syscall.RawConn, func() error, error) {
	if ctx == nil {
		return nil, nil, errors.New("native poll turn requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, nil, errors.New("native poll turn requires an absolute deadline")
	}
	if err := transport.file.SetDeadline(deadline); err != nil {
		return nil, nil, err
	}
	raw, err := transport.file.SyscallConn()
	if err != nil {
		return nil, nil, err
	}
	cancellation := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { cancellation <- transport.file.SetDeadline(time.Now()) })
	joined := func() error {
		if stop() {
			return nil
		}
		return <-cancellation
	}
	return raw, joined, nil
}

func (transaction *nativeVLANTransaction) send(ctx context.Context) (resultErr error) {
	raw, join, err := transaction.transport.nativeVLANPollTurn(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, join()) }()
	var syscallErr error
	err = raw.Write(func(fd uintptr) bool {
		transaction.sendPhase = nativeVLANSendEntered
		syscallErr = unix.Sendto(int(fd), transaction.wire, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK})
		retry := syscallErr == unix.EAGAIN || syscallErr == unix.EINTR
		if retry {
			return false
		}
		if syscallErr == nil {
			transaction.sendPhase = nativeVLANSendReturned
		}
		return true
	})
	return errors.Join(err, syscallErr)
}

// resumeOriginal is cleanup-only. It never increments a sequence, serializes
// another request, or invokes send. Pending replies keep their exact accumulator.
func (transport *nativeVLANTransport) resumeOriginal(ctx context.Context) error {
	transaction := transport.pending
	if transaction == nil {
		return nil
	}
	return transaction.resume(ctx)
}

func (transaction *nativeVLANTransaction) resume(ctx context.Context) (resultErr error) {
	if completed, ok := transaction.replies.(nativeVLANReplyComplete); ok {
		return completed.err
	}
	if transaction.poison != nil {
		return transaction.waitError(transaction.poison)
	}
	if transaction.sendPhase != nativeVLANSendReturned {
		return transaction.waitError(errors.New("original native send has no completed syscall outcome; it cannot be resent"))
	}
	raw, join, err := transaction.transport.nativeVLANPollTurn(ctx)
	if err != nil {
		return transaction.retainReadError(err)
	}
	defer func() { resultErr = errors.Join(resultErr, join(), ctx.Err()) }()
	if transaction.transport.observeBeforeRead != nil {
		transaction.transport.observeBeforeRead(transaction)
	}
	for transaction.received < nativeVLANReplyLimit {
		if err := ctx.Err(); err != nil {
			return transaction.retainReadError(err)
		}
		buffer := make([]byte, nativeVLANDatagramLimit)
		var count, flags int
		var from unix.Sockaddr
		var syscallErr error
		err = raw.Read(func(fd uintptr) bool {
			count, _, flags, from, syscallErr = unix.Recvmsg(int(fd), buffer, nil, 0)
			retry := syscallErr == unix.EAGAIN || syscallErr == unix.EINTR
			return !retry
		})
		if readErr := errors.Join(syscallErr, err); readErr != nil {
			return transaction.retainReadError(readErr)
		}
		if flags&unix.MSG_TRUNC != 0 {
			return transaction.reject(errors.New("native VLAN reply datagram was truncated"))
		}
		sender, ok := from.(*unix.SockaddrNetlink)
		if !ok {
			return transaction.reject(errors.New("native VLAN reply has another address family"))
		}
		if sender.Pid != 0 {
			return transaction.reject(errors.New("native VLAN reply did not come from the kernel"))
		}
		messages, err := syscall.ParseNetlinkMessage(buffer[:count])
		if err != nil {
			return transaction.reject(err)
		}
		if len(messages) == 0 {
			return transaction.reject(errors.New("native VLAN reply datagram is empty"))
		}
		// Process a datagram already received from the original kernel socket even
		// if cancellation raced it. Its bytes are retained outcome evidence, not IO
		// permission. Only the next blocking read uses the turn's original deadline.
		for _, message := range messages {
			transaction.received++
			if transaction.received > nativeVLANReplyLimit {
				return transaction.reject(errors.New("native VLAN reply message limit exceeded"))
			}
			if transaction.transport.observeReply != nil {
				transaction.transport.observeReply(message.Header.Seq)
			}
			if message.Header.Seq != transaction.sequence {
				continue
			}
			if message.Header.Pid != transaction.transport.pid {
				return transaction.reject(errors.New("native VLAN reply names another request port"))
			}
			if message.Header.Flags&unix.NLM_F_DUMP_INTR != 0 {
				return transaction.reject(nl.ErrDumpInterrupted)
			}
			done, err := transaction.consume(message)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
		}
	}
	return transaction.reject(errors.New("native VLAN request exhausted its reply limit"))
}

func (transaction *nativeVLANTransaction) reject(err error) error {
	transaction.poison = err
	return transaction.waitError(err)
}

func (transaction *nativeVLANTransaction) consume(message syscall.NetlinkMessage) (bool, error) {
	switch message.Header.Type {
	case unix.NLMSG_ERROR:
		if len(message.Data) < 4+unix.SizeofNlMsghdr {
			return false, transaction.reject(errors.New("native VLAN ACK is truncated"))
		}
		originalType := nl.NativeEndian().Uint16(message.Data[8:10])
		originalSequence := nl.NativeEndian().Uint32(message.Data[12:16])
		originalPID := nl.NativeEndian().Uint32(message.Data[16:20])
		if originalType != transaction.kind {
			return false, transaction.reject(errors.New("native VLAN ACK names another request kind"))
		}
		if originalSequence != transaction.sequence {
			return false, transaction.reject(errors.New("native VLAN ACK names another original sequence"))
		}
		if originalPID != transaction.transport.pid {
			return false, transaction.reject(errors.New("native VLAN ACK names another original port"))
		}
		code := int32(nl.NativeEndian().Uint32(message.Data[:4]))
		if code < 0 {
			err := unix.Errno(-code)
			transaction.complete(err)
			return false, err
		}
		if code > 0 {
			return false, transaction.reject(errors.New("native VLAN ACK contains a positive error code"))
		}
		transaction.acknowledged = true
	case unix.NLMSG_DONE:
		if transaction.mode != nativeVLANReplyDump {
			return false, transaction.reject(errors.New("unexpected native VLAN multipart completion"))
		}
		if len(message.Data) > 0 {
			if len(message.Data) < 4 {
				return false, transaction.reject(errors.New("native VLAN dump completion is truncated"))
			}
			code := int32(nl.NativeEndian().Uint32(message.Data[:4]))
			if code < 0 {
				err := unix.Errno(-code)
				transaction.complete(err)
				return false, err
			}
			if code > 0 {
				return false, transaction.reject(errors.New("native VLAN dump completion has a positive error code"))
			}
		}
		transaction.complete(nil)
		return true, nil
	case unix.RTM_NEWLINK:
		if len(message.Data) < unix.SizeofIfInfomsg {
			return false, transaction.reject(errors.New("native VLAN link reply is truncated"))
		}
		header := unix.NlMsghdr{Len: message.Header.Len, Type: message.Header.Type, Flags: message.Header.Flags, Seq: message.Header.Seq, Pid: message.Header.Pid}
		link, err := netlink.LinkDeserialize(&header, message.Data)
		if err != nil {
			return false, transaction.reject(err)
		}
		if link == nil {
			return false, transaction.reject(errors.New("native VLAN link reply is absent"))
		}
		if len(transaction.links) >= nativeVLANReplyLimit {
			return false, transaction.reject(errors.New("native VLAN link reply limit exceeded"))
		}
		transaction.links = append(transaction.links, link)
	case unix.NLMSG_NOOP:
		return false, transaction.reject(errors.New("unexpected native VLAN noop reply"))
	case unix.NLMSG_OVERRUN:
		return false, transaction.reject(errors.New("native VLAN original reply was overrun"))
	default:
		// Netlink message types are open kernel input, not Ze's closed mode enum.
		return false, transaction.reject(fmt.Errorf("unexpected native VLAN reply type %d", message.Header.Type))
	}
	switch transaction.mode {
	case nativeVLANReplyACK:
		if transaction.acknowledged {
			transaction.complete(nil)
			return true, nil
		}
	case nativeVLANReplyLink:
		if len(transaction.links) > 1 {
			return false, transaction.reject(errors.New("native lookup returned more than one original link"))
		}
		if len(transaction.links) == 1 {
			transaction.complete(nil)
			return true, nil
		}
	case nativeVLANReplyCreate:
		if len(transaction.links) > 1 {
			return false, transaction.reject(errors.New("native CREATE returned more than one original link"))
		}
		if transaction.acknowledged {
			if len(transaction.links) == 1 {
				if transaction.creator != nil {
					if err := transaction.creator.createdEcho(transaction.links[0]); err != nil {
						return false, transaction.reject(err)
					}
				}
				transaction.complete(nil)
				return true, nil
			}
		}
	case nativeVLANReplyDump:
	case nativeVLANReplyUnspecified:
		return false, transaction.reject(errors.New("native reply mode is unspecified"))
	}
	return false, nil
}

func nativeVLANLinkRequest(kind, flags, index int) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(kind, flags)
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = int32(index)
	request.AddData(message)
	return request
}
func (transport *nativeVLANTransport) byName(name string) (netlink.Link, error) {
	request := nativeVLANLinkRequest(unix.RTM_GETLINK, 0, 0)
	request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name)))
	return transport.one(request)
}
func (transport *nativeVLANTransport) byIndex(index int) (netlink.Link, error) {
	return transport.one(nativeVLANLinkRequest(unix.RTM_GETLINK, 0, index))
}
func (transport *nativeVLANTransport) one(request *nl.NetlinkRequest) (netlink.Link, error) {
	links, err := transport.request(request, nativeVLANReplyLink)
	if errors.Is(err, unix.ENODEV) {
		return nil, netlink.LinkNotFoundError{}
	}
	if err != nil {
		return nil, err
	}
	if len(links) != 1 {
		return nil, errors.New("native VLAN lookup returned no unique link")
	}
	return links[0], nil
}
func (transport *nativeVLANTransport) list() ([]netlink.Link, error) {
	return transport.request(nativeVLANLinkRequest(unix.RTM_GETLINK, unix.NLM_F_DUMP, 0), nativeVLANReplyDump)
}

func nativeVLANAttributes(request *nl.NetlinkRequest, link *netlink.Vlan) {
	if link.ParentIndex > 0 {
		request.AddData(nl.NewRtAttr(unix.IFLA_LINK, nl.Uint32Attr(uint32(link.ParentIndex))))
	}
	if link.Name != "" {
		request.AddData(nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(link.Name)))
	}
	info := nl.NewRtAttr(unix.IFLA_LINKINFO, nil)
	info.AddRtAttr(nl.IFLA_INFO_KIND, nl.ZeroTerminated("vlan"))
	data := info.AddRtAttr(nl.IFLA_INFO_DATA, nil)
	data.AddRtAttr(nl.IFLA_VLAN_ID, nl.Uint16Attr(uint16(link.VlanId)))
	appendMap := func(kind int, mapping map[uint32]uint32) {
		if mapping == nil {
			return
		}
		attribute := data.AddRtAttr(kind, nil)
		for source, target := range mapping {
			var encoded [8]byte
			nl.NativeEndian().PutUint32(encoded[:4], source)
			nl.NativeEndian().PutUint32(encoded[4:], target)
			attribute.AddRtAttr(nl.IFLA_VLAN_QOS_MAPPING, encoded[:])
		}
	}
	appendMap(nl.IFLA_VLAN_INGRESS_QOS, link.IngressQosMap)
	appendMap(nl.IFLA_VLAN_EGRESS_QOS, link.EgressQosMap)
	request.AddData(info)
}
func (transport *nativeVLANTransport) add(link netlink.Link) (netlink.Link, error) {
	creator := transport.creator
	if creator == nil {
		return nil, errors.New("native CREATE has no bound original resource")
	}
	defer func() { transport.creator = nil }()
	vlan, ok := link.(*netlink.Vlan)
	if !ok {
		return nil, errors.New("native creator requires a VLAN")
	}
	request := nativeVLANLinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK|unix.NLM_F_ECHO, 0)
	nativeVLANAttributes(request, vlan)
	links, err := transport.request(request, nativeVLANReplyCreate)
	if len(links) == 1 {
		return links[0], err
	}
	return nil, err
}
func (transport *nativeVLANTransport) up(link netlink.Link) error {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK)
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = int32(link.Attrs().Index)
	message.Flags = unix.IFF_UP
	message.Change = unix.IFF_UP
	request.AddData(message)
	_, err := transport.request(request, nativeVLANReplyACK)
	return err
}
func (transport *nativeVLANTransport) modify(link netlink.Link) error {
	vlan, ok := link.(*netlink.Vlan)
	if !ok {
		return errors.New("native map writer requires a VLAN")
	}
	request := nativeVLANLinkRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK, vlan.Index)
	nativeVLANAttributes(request, vlan)
	_, err := transport.request(request, nativeVLANReplyACK)
	return err
}
func (transport *nativeVLANTransport) remove(link netlink.Link) error {
	_, err := transport.request(nativeVLANLinkRequest(unix.RTM_DELLINK, unix.NLM_F_ACK, link.Attrs().Index), nativeVLANReplyACK)
	return err
}
func (transport *nativeVLANTransport) alias(link netlink.Link, alias string) error {
	request := nativeVLANLinkRequest(unix.RTM_NEWLINK, unix.NLM_F_ACK, link.Attrs().Index)
	request.AddData(nl.NewRtAttr(unix.IFLA_IFALIAS, nl.ZeroTerminated(alias)))
	_, err := transport.request(request, nativeVLANReplyACK)
	return err
}

func (transaction *nativeVLANTransaction) retainReadError(err error) error {
	return transaction.waitError(err)
}

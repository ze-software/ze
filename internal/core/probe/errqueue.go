// RFC: rfc/short/rfc792.md -- the ICMP errors the kernel queues for a DF probe
// Design: docs/architecture/diagnostics/active-probes.md -- the error queue behind a DF probe
// Related: errqueue_linux.go -- the reader, the parser and the kernel estimate
// Related: errqueue_other.go -- the stubs that report the capability absent
// Related: socket_linux.go -- applyDFOptions, which sets IP_RECVERR so the queue fills
//
// A probe sent with the Don't Fragment bit is refused in one of two places:
// by a router on the path, which answers with an ICMP Fragmentation Needed or
// Packet Too Big message, or by this host's own kernel, which refuses a send
// larger than the cached path MTU with EMSGSIZE. Linux runs the RFC 1191 and
// RFC 8201 state machines for both, and with IP_RECVERR set it hands each
// refusal to the socket's error queue as a SockExtendedErr whose ee_info is
// the reported next-hop MTU. Ze reads that queue rather than parsing the ICMP
// error itself (owner decision, 2026-09-11).
//
// This file holds the portable result types. What the kernel reports is
// never returned as a bare integer: a zero next-hop MTU is RFC 1191's signal
// from an unmodified router, an IPv6 value below 1280 is a message RFC 8201
// says to discard, and a caller that read either as a number would act on a
// value the RFCs forbid. So the reader answers with a named outcome, and the
// MTU field means something only under ErrQueueMTUReported.

package probe

import (
	"errors"
	"net/netip"
	"syscall"
)

// ErrQueueOutcome names what one read of the error queue found. The zero
// value is ErrQueueUnspecified so an unwritten result is never mistaken for
// an empty queue.
type ErrQueueOutcome uint8

const (
	// ErrQueueUnspecified is the zero value and is never a valid outcome.
	ErrQueueUnspecified ErrQueueOutcome = iota
	// ErrQueueEmpty says the queue held nothing: no refusal has reached the
	// socket since the last drain.
	ErrQueueEmpty
	// ErrQueueMTUReported says a refusal was queued and its reported next-hop
	// MTU is usable: QueuedError.MTU carries it.
	ErrQueueMTUReported
	// ErrQueueMTUUnreported says an error was queued and it carries no usable
	// next-hop MTU. QueuedError.Errno says which error it was: EMSGSIZE with a
	// zero next-hop MTU is RFC 1191's old-router signal, EMSGSIZE with an IPv6
	// value below 1280 is the message RFC 8201 Section 4 discards, and another
	// errno is an ICMP error that was never about size at all.
	ErrQueueMTUUnreported
)

// String names the outcome for a log line or a test failure.
func (o ErrQueueOutcome) String() string {
	switch o {
	case ErrQueueEmpty:
		return "empty"
	case ErrQueueMTUReported:
		return "mtu-reported"
	case ErrQueueMTUUnreported:
		return "mtu-unreported"
	default:
		return nameUnspecified
	}
}

// QuotedEcho is the ICMP echo header the kernel quotes with a refusal that
// came from the network. The kernel matches a queued ICMP error to a raw
// socket by protocol and bound address only, so a raw socket that is not
// connected receives every error quoting an ICMP datagram from this host.
// The caller matches ID and Seq against the probe it sent before it believes
// the value, which is the validation RFC 8201 Section 4 asks for: "Nodes
// should appropriately validate the payload of ICMPv6 PTB messages to ensure
// these are received in response to transmitted traffic". Present is false
// for a refusal raised by this host's own send, which quotes nothing.
type QuotedEcho struct {
	Present bool
	ID      uint16
	Seq     uint16
}

// QueuedError is one entry read off the socket's error queue.
type QueuedError struct {
	// Outcome says whether MTU holds a value. It is never ErrQueueEmpty on
	// an entry a drain hands to its visitor.
	Outcome ErrQueueOutcome
	// MTU is the reported next-hop MTU in octets, meaningful only under
	// ErrQueueMTUReported. On IPv4 a value below 68 arrives clamped to 68.
	MTU uint32
	// Errno is the kernel's translation of the error: EMSGSIZE for a size
	// refusal, the ICMP-derived errno for anything else.
	Errno syscall.Errno
	// Local is true when this host's own kernel refused the send against
	// its cached path MTU, so MTU is the cache's estimate rather than a value
	// a router put on the wire on this run. It is false for a refusal a
	// router answered.
	Local bool
	// Offender is the address of the router that answered, for a refusal
	// from the network. It is the zero Addr for a local refusal.
	Offender netip.Addr
	// Echo is the probe the refusal quotes, when it quotes one.
	Echo QuotedEcho
}

// Payload keys a refused probe carries, the same two on every prober. A
// refused row always carries FieldNextHopMTUReported, and FieldNextHopMTU
// only when it is true: a zero is never written in place of an absent value,
// because RFC 1191 Section 3 makes a zero next-hop MTU a distinct signal.
const (
	FieldNextHopMTU         = "next-hop-mtu"
	FieldNextHopMTUReported = "next-hop-mtu-reported"
)

// ErrQueueDrainMax bounds one drain of the error queue. A host flooding
// ICMP errors at a probe socket fills the queue faster than a drain empties
// it, and without a bound the goroutine that drains would never return to
// its select loop. After this many entries the drain returns, and the
// entries still queued wait for the next wake. The kernel bounds the queue
// itself by the socket's receive buffer, so nothing accumulates without
// limit on either side.
const ErrQueueDrainMax = 32

// ErrErrQueueUnsupported is what the reader answers on a platform whose
// socket layer has no error queue. The capability is absent by name, never
// an empty queue.
var ErrErrQueueUnsupported = errors.New("probe: the socket error queue is not supported on this platform")

// ErrPathMTUUnsupported is what KernelPathMTU answers on a platform with no
// IP_MTU socket option.
var ErrPathMTUUnsupported = errors.New("probe: the kernel path MTU estimate is not readable on this platform")

// ErrPathMTUUnknown is what KernelPathMTU answers when the kernel holds no
// estimate for the destination: there is no route, or the socket could not
// be connected to it. A zero is never returned in its place.
var ErrPathMTUUnknown = errors.New("probe: the kernel holds no path MTU estimate for this destination")

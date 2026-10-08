// Design: docs/architecture/mpls/mpls-kernel.md -- the path MTU floor on a push route
// Related: events.go -- Entry.PathMTU carries the value this floor bounds
// RFC: rfc/short/rfc3032.md
// RFC: rfc/short/rfc3209.md

package mplsfib

// MaxLabelStack is the deepest outgoing label stack the forwarding owner
// installs. fib-kernel refuses a deeper stack, so no labeled route Ze programs
// imposes more label headroom than this.
const MaxLabelStack = 16

const (
	// RFC 791 Section 3.2: "Every internet module must be able to forward a
	// datagram of 68 octets without further fragmentation.  This is because
	// an internet header may be up to 60 octets, and the minimum fragment is
	// 8 octets." No IPv4 path MTU is smaller.
	ipv4MinimumMTU = 68

	// RFC 3032 Section 2.1: "Each label stack entry is represented by 4
	// octets." N in RFC 3209's rule counts these.
	labelStackEntryOctets = 4
)

// PathMTUMinimum is the smallest non-zero path MTU Ze accepts, signals or
// installs on a labeled path. The path MTU includes the label stack (RFC 3209
// Section 2.6, "Let M be the smaller of the "Maximum Initially Labeled IP
// Datagram Size" or of (Path MTU - N)"), so under the deepest stack Ze installs
// the inner IPv4 datagram budget is still RFC 791's 68 octets: a full 60-octet
// header and one 8-octet fragment.
//
// Below it, Linux's IPv4 fragmentation on a push route can be left fewer than
// eight data octets per fragment, rounds each fragment to zero, and on a stock
// kernel emits empty fragments forever inside softirq (mpls-kernel.md).
//
// The floor is IPv4's and not RFC 8200's 1280: an IPv6 router never
// fragments, so an IPv6 datagram over the budget is dropped with Packet Too
// Big and cannot reach that loop, while a 1280 floor would refuse IPv4 paths
// RFC 791 makes legal.
const PathMTUMinimum uint32 = ipv4MinimumMTU + labelStackEntryOctets*MaxLabelStack

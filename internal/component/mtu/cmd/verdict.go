// Design: docs/architecture/diagnostics/path-mtu.md -- the tunnel verdicts, the run ladder and the underlay advice
// Related: arith.go -- the ceiling and the recommended value a tunnel is judged against
// Related: mtu.go -- runVerdict, the run-level enum this file's ladder produces

package cmd

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// tunnelVerdict classifies one tunnel interface against its ceiling and its
// recommended MTU. The zero value is Unspecified so an unset field can never
// pass for a verdict. The thresholds are the ported tool's (A-5).
type tunnelVerdict uint8

const (
	tunnelVerdictUnspecified tunnelVerdict = iota
	// tunnelVerdictDown: the interface is down, so it has no current MTU.
	tunnelVerdictDown
	// tunnelVerdictNoUsableMTU: no recommendation exists (errNoUsableMTU),
	// so the current size is beside the point and no command is emitted.
	tunnelVerdictNoUsableMTU
	// tunnelVerdictOversized: the current MTU is above the ceiling; the
	// figure is the excess in octets.
	tunnelVerdictOversized
	// tunnelVerdictTight: the current MTU is within the margin of the
	// ceiling; the figure is the spare octets below the ceiling.
	tunnelVerdictTight
	// tunnelVerdictUnderUtilized: the current MTU is below the recommended
	// value; the figure is the gain in octets.
	tunnelVerdictUnderUtilized
	// tunnelVerdictOK: the current MTU is the recommended value.
	tunnelVerdictOK
)

// String answers the wire spelling of the verdict. It is written into the
// payload and never compared.
func (v tunnelVerdict) String() string {
	switch v {
	case tunnelVerdictDown:
		return "down"
	case tunnelVerdictNoUsableMTU:
		return "no-usable-mtu"
	case tunnelVerdictOversized:
		return "oversized"
	case tunnelVerdictTight:
		return "tight"
	case tunnelVerdictUnderUtilized:
		return "under-utilized"
	case tunnelVerdictOK:
		return "ok"
	default:
		panic("BUG: tunnelVerdict written to the payload before it was set")
	}
}

// needsCommand reports whether a verdict earns a `set ... mtu <recommended>`
// command in the remediation list: the three verdicts where a recommended
// value exists and the current one differs from it.
func (v tunnelVerdict) needsCommand() bool {
	switch v {
	case tunnelVerdictOversized, tunnelVerdictTight, tunnelVerdictUnderUtilized:
		return true
	default:
		return false
	}
}

// tunnelSizing is what one tunnel is classified from.
type tunnelSizing struct {
	// up is false when the interface is down; current is then meaningless.
	up bool
	// current is the interface's MTU now.
	current uint16
	// ceiling is the largest packet the SA can carry over the measured path.
	ceiling uint16
	// recommended is the value arith.go recommended; hasRecommended is false
	// when it answered errNoUsableMTU.
	recommended    uint16
	hasRecommended bool
}

// classifyTunnel applies the five verdicts in the ported tool's order, first
// match wins, and answers the octets the verdict names: the excess above the
// ceiling, the spare below it, or the gain up to the recommended value. The
// figure is 0 for down, no-usable-mtu and ok, where no octets are named.
//
// "No usable MTU" is tested BEFORE "oversized" on purpose: when no safe value
// exists the current size is beside the point, and falling through would put
// the absent value into the remediation list as a command.
func classifyTunnel(s *tunnelSizing) (tunnelVerdict, int) {
	if !s.up {
		return tunnelVerdictDown, 0
	}
	if !s.hasRecommended {
		return tunnelVerdictNoUsableMTU, 0
	}
	current := int(s.current)
	ceil := int(s.ceiling)
	if current > ceil {
		return tunnelVerdictOversized, current - ceil
	}
	if current > ceil-recommendedMarginOctets {
		return tunnelVerdictTight, ceil - current
	}
	if current != int(s.recommended) {
		return tunnelVerdictUnderUtilized, int(s.recommended) - current
	}
	return tunnelVerdictOK, 0
}

// runVerdictOf folds the tunnel verdicts and the notes into the run-level
// ladder, first match wins: any oversized or no-usable-mtu tunnel is
// action-needed; any tight or down tunnel is check; a fault note found
// outside the tunnel table is action-needed, because a PMTU blackhole in the
// counters must not sit under an ok; no tunnels at all is no-tunnels; and
// the rest, under-utilized tunnels included, is ok.
//
// The ported tool's first rung, DO NOT APPLY for a non-standard ESP proposal,
// has no counterpart: the overhead is derived per negotiated transform, and a
// transform the arithmetic refuses (errOverheadRefused) is reported as a
// fault note for that one tunnel while every other tunnel's figures stay
// valid, so it reaches this ladder as faultOutsideTable.
func runVerdictOf(verdicts []tunnelVerdict, faultOutsideTable bool) runVerdict {
	var oversized, tight bool
	for _, v := range verdicts {
		switch v {
		case tunnelVerdictOversized, tunnelVerdictNoUsableMTU:
			oversized = true
		case tunnelVerdictTight, tunnelVerdictDown:
			tight = true
		case tunnelVerdictUnderUtilized, tunnelVerdictOK:
		default:
			panic("BUG: runVerdictOf given an unset tunnel verdict")
		}
	}
	if oversized {
		return runVerdictActionNeeded
	}
	if tight {
		return runVerdictCheck
	}
	if faultOutsideTable {
		return runVerdictActionNeeded
	}
	if len(verdicts) == 0 {
		return runVerdictNoTunnels
	}
	return runVerdictOK
}

// underlayOutcome is what the advice matrix decided about the underlay
// interface (AC-12, AC-13, AC-14). The zero value is Unspecified so an unset
// field can never pass for an outcome.
type underlayOutcome uint8

const (
	underlayUnspecified underlayOutcome = iota
	// underlayUnreadable: the interface MTU could not be read, so the run
	// cannot say whether the interface or the path is the constraint.
	underlayUnreadable
	// underlayCappedByInterface: the measurement equals the interface MTU,
	// which is below standard Ethernet, so the path may be wider than the
	// run can tell.
	underlayCappedByInterface
	// underlayAtStandard: the measurement equals the interface MTU at
	// standard Ethernet; there is nothing to say.
	underlayAtStandard
	// underlayUndecidable: the peers measure below the interface and no
	// reference answered, so a clamped circuit and a clamped peer path look
	// identical (AC-14).
	underlayUndecidable
	// underlayNotClamped: the reference reaches the full interface MTU, so
	// only the peer paths are clamped; lowering the interface would cost
	// octets on every other destination (AC-12).
	underlayNotClamped
	// underlayCircuitClamped: the reference measures the same as the
	// peers, so the whole access circuit is clamped and the interface
	// should match it (AC-13).
	underlayCircuitClamped
	// underlayTwoClamps: the reference lands between, so the circuit is
	// clamped and the peer paths tighter again; the interface should match
	// the lower of the two, confirmed with a third address.
	underlayTwoClamps
)

// String answers the wire spelling of the outcome. It is written into the
// payload and never compared.
func (o underlayOutcome) String() string {
	switch o {
	case underlayUnreadable:
		return "unreadable"
	case underlayCappedByInterface:
		return "capped-by-interface"
	case underlayAtStandard:
		return "at-standard"
	case underlayUndecidable:
		return "undecidable"
	case underlayNotClamped:
		return "not-clamped"
	case underlayCircuitClamped:
		return "circuit-clamped"
	case underlayTwoClamps:
		return "two-clamps"
	default:
		panic("BUG: underlayOutcome written to the payload before it was set")
	}
}

// severity is the note weight the ported tool gave each outcome.
func (o underlayOutcome) severity() noteSeverity {
	switch o {
	case underlayNotClamped, underlayAtStandard:
		return noteSeverityInfo
	case underlayUnreadable, underlayCappedByInterface, underlayUndecidable, underlayCircuitClamped, underlayTwoClamps:
		return noteSeverityCaution
	default:
		panic("BUG: severity of an unset underlayOutcome")
	}
}

// text is the sentence the note carries for an outcome, naming the figures
// the matrix decided from and, where a command was produced, the value it
// sets (AC-12, AC-13, AC-14).
func (o underlayOutcome) text(in *underlayInput, mtu uint16) string {
	var b textbuf.Buffer
	switch o {
	case underlayUnreadable:
		b.Str("the underlay interface MTU could not be read, so the run cannot tell the interface from the path as the constraint")
	case underlayCappedByInterface:
		b.Str("the measured path equals the ").Str(in.iface).Str(" MTU of ").Uint16(in.current)
		b.Str(", which is below standard Ethernet; the interface caps the measurement and the path may be wider")
	case underlayAtStandard:
		b.Str("the measured path equals the ").Str(in.iface).Str(" MTU of ").Uint16(in.current).Str(" at standard Ethernet; nothing clamps it")
	case underlayUndecidable:
		b.Str("the peers measure ").Uint16(in.tightest).Str(" below the ").Str(in.iface).Str(" MTU of ").Uint16(in.current)
		b.Str(" and no reference address answered, so a clamped circuit and a clamped peer path cannot be told apart; the underlay advice is undecidable")
	case underlayNotClamped:
		b.Str("the reference ").Addr(in.referenceHost).Str(" reaches ").Uint16(in.reference).Str(", the full ").Str(in.iface)
		b.Str(" MTU, so the access circuit is not clamped; only the peer paths are, and lowering ").Str(in.iface)
		b.Str(" would cost octets on every other destination")
	case underlayCircuitClamped:
		b.Str("the reference ").Addr(in.referenceHost).Str(" measures ").Uint16(in.reference).Str(", the same as the peers, so the whole access circuit is clamped; set ")
		b.Str(in.iface).Str(" to ").Uint16(mtu).Str(" so traffic sent outside a tunnel stops fragmenting too")
		noCommandClause(&b, in)
	case underlayTwoClamps:
		b.Str("the reference ").Addr(in.referenceHost).Str(" measures ").Uint16(in.reference).Str(" and the peers ").Uint16(in.tightest)
		b.Str(", so the circuit and the peer paths are clamped in series; set ").Str(in.iface).Str(" to ").Uint16(mtu)
		b.Str(" and confirm with a third address")
		noCommandClause(&b, in)
	default:
		panic("BUG: text of an unset underlayOutcome")
	}
	return b.String()
}

// noCommandClause says why the remediation list carries no command for an
// outcome that names a value to set: the underlay's link type has no list in
// the iface YANG, so the value is applied by whatever configures that link.
func noCommandClause(b *textbuf.Buffer, in *underlayInput) {
	if kindConfigurable(in.kind) {
		return
	}
	b.Str("; no configuration command is listed because ").Str(in.iface).Str(" is not an interface kind the configuration schema holds")
}

// ethernetStandardMTU is the MTU the interface is expected to sit at when
// nothing clamps it; an interface below it that caps the measurement earns a
// caution, one at it does not.
const ethernetStandardMTU = 1500

// underlayAdvice is one row of the advice matrix. Command is the underlay
// interface command the matrix produced, and is empty for every outcome that
// does not change the interface.
type underlayAdvice struct {
	outcome underlayOutcome
	// mtu is the value the interface should be set to, when command is set.
	mtu     uint16
	command string
}

// underlayInput is what the matrix decides from.
type underlayInput struct {
	// iface is the underlay interface name; empty when it could not be found.
	iface string
	// kind is the interface list of the iface YANG the underlay belongs to
	// (iface.CanonicalInterfaceType: ethernet, veth, bridge, dummy, tunnel,
	// wireguard, xfrm), and empty when the schema holds no list for its link
	// type. The command is spelled with it, and no command is produced without
	// it: `set interface ethernet br0` would create an ethernet block for a
	// bridge rather than size the bridge.
	kind string
	// current is the interface's MTU now; 0 when it could not be read.
	current uint16
	// tightest is the lowest path MTU measured to any peer.
	tightest uint16
	// reference is the path MTU measured to the reference host; hasReference
	// is false when no reference address answered.
	reference    uint16
	hasReference bool
	// referenceHost names the reference address for the note text.
	referenceHost netip.Addr
}

// adviseUnderlay is the advice matrix of the ported tool as a pure function.
// There is no current < tightest row: a probe larger than the interface MTU
// is refused by the local kernel and never leaves the box, so a measurement
// can never exceed the interface it was sent from.
func adviseUnderlay(in *underlayInput) underlayAdvice {
	if in.tightest == 0 {
		panic("BUG: adviseUnderlay called with no measured peer path; the caller skips the matrix then")
	}
	if in.iface == "" {
		return underlayAdvice{outcome: underlayUnreadable}
	}
	if in.current == 0 {
		return underlayAdvice{outcome: underlayUnreadable}
	}
	if in.current == in.tightest {
		if in.current == ethernetStandardMTU {
			return underlayAdvice{outcome: underlayAtStandard}
		}
		return underlayAdvice{outcome: underlayCappedByInterface}
	}
	if !in.hasReference {
		return underlayAdvice{outcome: underlayUndecidable}
	}
	if in.reference >= in.current {
		return underlayAdvice{outcome: underlayNotClamped}
	}
	if in.reference == in.tightest {
		return underlayAdvice{outcome: underlayCircuitClamped, mtu: in.reference, command: underlayCommand(in.kind, in.iface, in.reference)}
	}
	lowest := min(in.reference, in.tightest)
	return underlayAdvice{outcome: underlayTwoClamps, mtu: lowest, command: underlayCommand(in.kind, in.iface, lowest)}
}

// loopbackKind is the one canonical type iface answers for which the YANG holds
// no interface list, so no command can name it.
const loopbackKind = "loopback"

// underlayCommand is the Ze configuration command that sets the underlay
// interface's MTU: `interface <kind> <name> mtu <value>` in the iface YANG,
// where every interface list carries the interface-common mtu leaf and kind
// names the list the underlay belongs to. It is empty for a kind the schema
// does not configure: the note still names the value, and an operator is
// never handed a command that would create a block of the wrong kind.
func underlayCommand(kind, iface string, mtu uint16) string {
	if !kindConfigurable(kind) {
		return ""
	}
	return interfaceMTUCommand(kind, iface, mtu)
}

// kindConfigurable reports whether the iface YANG holds an interface list, and
// so an mtu leaf, for a canonical interface type: every type iface answers
// except the empty one (a link type outside the schema) and the loopback.
func kindConfigurable(kind string) bool {
	if kind == "" {
		return false
	}
	return kind != loopbackKind
}

// tunnelCommand is the Ze configuration command that sets a tunnel
// interface's MTU. A tunnel a Child SA is bound to is an xfrm interface, and
// the xfrm list carries the same interface-common mtu leaf.
func tunnelCommand(iface string, mtu uint16) string {
	return interfaceMTUCommand("xfrm", iface, mtu)
}

func interfaceMTUCommand(kind, iface string, mtu uint16) string {
	var b textbuf.Buffer
	b.Str("set interface ")
	b.Str(kind)
	b.Byte(' ')
	b.Str(iface)
	b.Str(" mtu ")
	b.Uint16(mtu)
	return b.String()
}

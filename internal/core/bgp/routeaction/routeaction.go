// Design: docs/architecture/api/json-format.md -- typed route action enum

// Package routeaction owns the typed route-action vocabulary shared by the BGP
// engine and the always-on route consumers (sysrib, the FIB backends, the
// best-change event contract).
//
// It lives in internal/core because those consumers must keep working when the
// BGP engine is compiled out (//go:build ze_bgp): a route verb is a
// forwarding-plane concept, not a BGP one, even though BGP is its busiest
// producer. The package has no dependencies beyond the standard library.
//
// Note the distinct internal/core/redistevents.RouteAction: that names a
// redistribution source event, this names what happens to a route.
package routeaction

import (
	"errors"
	"fmt"
)

var errUnspecifiedActionIsInvalidOnWire = errors.New("routeaction: unspecified Action is invalid on the wire")

// Action is the typed wire token describing what happened to a route.
// "add"/"del" are emitted by FamilyOperation (the wire-level command);
// "update"/"withdraw" are emitted by BestChangeEntry (best-path transitions).
// One typed enum covers both surfaces -- consumers compare on identity and
// MarshalText preserves the wire string for external plugins.
type Action uint8

const (
	Unspecified Action = 0
	Add         Action = 1
	Del         Action = 2
	Update      Action = 3
	Withdraw    Action = 4
	// Count is one past the highest valid Action. Use as the size of arrays
	// indexed by Action so per-action caches (pre-bound metric Counters,
	// dispatch tables, etc.) can be indexed directly with zero allocation.
	Count = 5
)

const (
	wireAdd         = "add"
	wireDel         = "del"
	wireUpdate      = "update"
	wireWithdraw    = "withdraw"
	wireUnspecified = "unspecified"
)

func (a Action) String() string {
	switch a {
	case Add:
		return wireAdd
	case Del:
		return wireDel
	case Update:
		return wireUpdate
	case Withdraw:
		return wireWithdraw
	case Unspecified:
		return wireUnspecified
	}
	return wireUnspecified
}

func (a Action) AppendTo(buf []byte) []byte { return append(buf, a.String()...) }

func (a Action) MarshalText() ([]byte, error) {
	if a == Unspecified {
		return nil, errUnspecifiedActionIsInvalidOnWire
	}
	return []byte(a.String()), nil
}

func (a *Action) UnmarshalText(data []byte) error {
	switch string(data) {
	case wireAdd:
		*a = Add
	case wireDel:
		*a = Del
	case wireUpdate:
		*a = Update
	case wireWithdraw:
		*a = Withdraw
	default:
		return fmt.Errorf("routeaction: unknown route action %q", string(data))
	}
	return nil
}

// Verb is the forwarding-plane operation an Action maps to. FIB backends
// (fib-kernel, fib-vpp, fib-p4) dispatch on it instead of each re-encoding that
// Withdraw and Del both mean remove and that Unspecified is a no-op. This
// package owns Action, so it is the single home for that mapping; every FIB
// backend already depends on this package for Action.
type Verb uint8

const (
	VerbSkip    Verb = iota // Unspecified or unknown action: no-op
	VerbInstall             // Add: program a new route
	VerbReplace             // Update: replace an existing route
	VerbRemove              // Withdraw or Del: remove a route
)

// Verb maps an Action to the forwarding-plane operation a FIB backend performs.
// Backends that do not distinguish install from replace (e.g. the MPLS and SRv6
// programming paths) handle VerbInstall and VerbReplace together. It is a pure
// value-enum lookup with no allocation, safe on the hot FIB install path.
func (a Action) Verb() Verb {
	switch a {
	case Add:
		return VerbInstall
	case Update:
		return VerbReplace
	case Withdraw, Del:
		return VerbRemove
	default: // Unspecified and any unknown value
		return VerbSkip
	}
}

// ProtocolType distinguishes iBGP from eBGP routes. This is a BGP-internal
// 2-value classification, not a cross-protocol identity (see
// redistevents.ProtocolID).
type ProtocolType uint8

const (
	ProtocolUnspecified ProtocolType = 0
	ProtocolEBGP        ProtocolType = 1
	ProtocolIBGP        ProtocolType = 2
	ProtocolCount       ProtocolType = 3
)

func (p ProtocolType) String() string {
	switch p {
	case ProtocolEBGP:
		return "ebgp"
	case ProtocolIBGP:
		return "ibgp"
	case ProtocolUnspecified, ProtocolCount:
		return wireUnspecified
	}
	return wireUnspecified
}

func (p ProtocolType) AppendTo(buf []byte) []byte { return append(buf, p.String()...) }

func (p ProtocolType) MarshalText() ([]byte, error) {
	if p == ProtocolUnspecified || p >= ProtocolCount {
		return nil, fmt.Errorf("routeaction: invalid ProtocolType %d on the wire", p)
	}
	return []byte(p.String()), nil
}

func (p *ProtocolType) UnmarshalText(data []byte) error {
	switch string(data) {
	case "ebgp":
		*p = ProtocolEBGP
	case "ibgp":
		*p = ProtocolIBGP
	default:
		return fmt.Errorf("routeaction: unknown BGP protocol type %q", string(data))
	}
	return nil
}

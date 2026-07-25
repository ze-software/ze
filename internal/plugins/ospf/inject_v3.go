// Design: plan/learned/1052-ospf-ext-14-debug-introspection.md -- guarded IPv6 native LSA inject.
// RFC: rfc/short/rfc5340.md (Section A.4.2.1: scope from S2/S1 bits; reserved=11 rejected),
// rfc/short/rfc2328.md (Section 14 MaxAge withdraw; Section 9.5 MinLSInterval pacing).
//
// `debug ipv6 ospf inject lsa scope <s> type <ls-type> id <link-state-id> [hex <body>]
// [withdraw]` originates a crafted OSPFv3 LSA into THIS router's LSDB through the base
// OriginateSelf (area/AS) / OriginateLinkSelf (link-local) seam. The flooding scope is
// DERIVED from the LS Type S2/S1 bits (a reserved scope, S2/S1 = 11, is rejected: AC-18);
// the optional scope keyword is cross-checked against it. Same double gate as IPv4 (AC-16/
// AC-17). Withdraw MaxAge-flushes via WithdrawSelf/WithdrawLinkSelf (AC-15).

package ospf

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v3InjectResult is the typed JSON payload of a v6 inject/withdraw action.
type v3InjectResult struct {
	Action      string `json:"action"`
	Scope       string `json:"scope"`
	LSType      string `json:"ls-type"`
	LSTypeHex   string `json:"ls-type-hex"`
	LinkStateID string `json:"link-state-id"`
	Installed   bool   `json:"installed"`
}

type v3InjectParams struct {
	scopeKw     string
	lsType      uint16
	linkStateID uint32
	body        []byte
	withdraw    bool
}

// debugInjectV3 validates and (if debug is enabled) originates or withdraws a crafted
// OSPFv3 LSA through the base origination seam.
func (e *engine) debugInjectV3(args []string) (v3InjectResult, error) {
	if !debugInjectIsEnabled() {
		return v3InjectResult{}, errDebugInjectDisabled
	}
	p, err := parseV3Inject(args)
	if err != nil {
		return v3InjectResult{}, err
	}
	if e.cfg.RouterID == (types.RouterID{}) {
		return v3InjectResult{}, errNoRouterID
	}
	wire := ospfv3types.LSType(p.lsType)
	neutral := v6NeutralLSType(wire)
	scope := v3ScopeName(neutral)
	if scope == "reserved" {
		return v3InjectResult{}, errReservedScope
	}
	if err := crossCheckV3Scope(p.scopeKw, scope); err != nil {
		return v3InjectResult{}, err
	}
	lsid := lsidBytes(p.linkStateID)
	router := e.cfg.RouterID
	key := types.LSAKey{Type: neutral, LinkStateID: types.LinkStateID(lsid), AdvertisingRouter: router}
	body := p.body
	enc := func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(wire, ospfv3types.LinkStateID(lsid), router, seq, purge),
			Body:   body,
		})
	}
	res := v3InjectResult{Scope: scope, LSType: neutral.String(), LSTypeHex: v3LSTypeHex(neutral)}
	var h packet.LSAHeader
	var ok bool
	switch {
	case p.withdraw && scope == scopeLinkLocal:
		res.Action = actionWithdraw
		h, ok = e.lsdb.WithdrawLinkSelf(e.firstInterfaceName(), key)
	case p.withdraw:
		res.Action = actionWithdraw
		h, ok = e.lsdb.WithdrawSelf(types.BackboneArea, key)
	case scope == scopeLinkLocal:
		res.Action = actionOriginate
		iface := e.firstInterfaceName()
		if iface == "" {
			return res, errors.New("no interface available for a link-local injection")
		}
		h, ok = e.lsdb.OriginateLinkSelf(iface, types.BackboneArea, key, body, enc)
	default:
		res.Action = actionOriginate
		h, ok = e.lsdb.OriginateSelf(types.BackboneArea, key, body, enc)
	}
	res.Installed = ok
	res.LinkStateID = h.LinkStateID.String()
	debugMetrics.Load().v6Inject.With(scope, res.Action).Inc()
	if ok {
		if p.withdraw {
			debugMetrics.Load().v6Injected.With(scope).Dec()
		} else {
			debugMetrics.Load().v6Injected.With(scope).Inc()
		}
	}
	return res, nil
}

// crossCheckV3Scope rejects a scope keyword that disagrees with the S2/S1-derived scope.
func crossCheckV3Scope(kw, derived string) error {
	if kw == "" {
		return nil
	}
	short := derived
	if derived == scopeLinkLocal {
		short = extRegistryLink
	}
	if kw != short && kw != derived {
		return fmt.Errorf("scope %q does not match the LS Type flooding scope %q", kw, derived)
	}
	return nil
}

// parseV3Inject parses the keyword-before-value v6 inject grammar.
func parseV3Inject(args []string) (v3InjectParams, error) {
	var p v3InjectParams
	haveType, haveID := false, false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "scope":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			p.scopeKw, i = v, i+2
		case "type":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			n, err := parseHexUint16(v)
			if err != nil {
				return p, err
			}
			p.lsType, haveType, i = n, true, i+2
		case "id":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			n, err := injectParseUint(v, 32)
			if err != nil {
				return p, err
			}
			p.linkStateID, haveID, i = uint32(n), true, i+2
		case "hex":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			b, err := decodeHexArg(v)
			if err != nil {
				return p, err
			}
			p.body, i = append(p.body, b...), i+2
		case actionWithdraw:
			p.withdraw, i = true, i+1
		default:
			return p, fmt.Errorf("unexpected inject argument: %s", args[i])
		}
	}
	if !haveType {
		return p, errors.New("inject requires an LS type (e.g. type 0x2009)")
	}
	if !haveID {
		return p, errors.New("inject requires a link-state id")
	}
	if len(p.body) > maxLSABodyLen {
		return p, errors.New("LSA body exceeds the maximum length (65515)")
	}
	return p, nil
}

// parseHexUint16 parses a 16-bit LS Type given as hex, with or without a 0x prefix.
func parseHexUint16(s string) (uint16, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	n, err := strconv.ParseUint(t, 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid LS type %q (want 16-bit hex like 0x2009): %w", s, err)
	}
	return uint16(n), nil
}

// lsidBytes renders a 32-bit Link State ID as its 4 big-endian octets.
func lsidBytes(v uint32) [4]byte {
	return [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

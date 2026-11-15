// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- guarded IPv6 native LSA inject.
// RFC: rfc/short/rfc5340.md (Section A.4.2.1: scope from S2/S1 bits; reserved=11 rejected),
// rfc/short/rfc2328.md (Section 14 MaxAge withdraw; Section 9.5 MinLSInterval pacing).
//
// `debug ipv6 ospf inject lsa type <ls-type> id <link-state-id> [scope <link|area|as>]
// [hex <body> ...] [withdraw] [address-family <af>]` originates a crafted OSPFv3 LSA into
// the LSDB of one of THIS router's OSPFv3 address-family instances (RFC 5838; ipv6-unicast
// when the keyword is absent) through the base OriginateSelf (area/AS) / OriginateLinkSelf
// (link-local) seam. The flooding scope is DERIVED from the LS Type S2/S1 bits (a reserved
// scope, S2/S1 = 11, is rejected: AC-18); the optional scope keyword is cross-checked
// against it. Same double gate as IPv4 (AC-16/AC-17), checked before the address family is
// read. Withdraw MaxAge-flushes via WithdrawSelf/WithdrawLinkSelf (AC-15).

package ospf

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
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

// debugInjectIntoAF serves `debug ipv6 ospf inject lsa`: it injects or withdraws a crafted
// OSPFv3 LSA in the LSDB of the address family cmdArgs names with `address-family <af>`,
// or of ipv6-unicast when it names none, by handing the other arguments to that engine's
// debugInjectV3. The enablement is checked first, so a disarmed router refuses the command
// whatever family it names, an unknown or idle one included: the double gate never depends
// on the family. Safe for concurrent use (it reads the set under its lock).
//
// It MUST hand debugInjectV3 the arguments splitInjectAddressFamily left, never cmdArgs:
// the inject parser (parseV3Inject) does not know the address-family keyword.
func (m *v6EngineSet) debugInjectIntoAF(cmdArgs []string) (v3InjectResult, error) {
	if !debugInjectIsEnabled() {
		return v3InjectResult{}, errDebugInjectDisabled
	}
	af, rest, err := splitInjectAddressFamily(cmdArgs)
	if err != nil {
		return v3InjectResult{}, err
	}
	// RFC 5838 Section 2: each address family's instance has its own link state database.
	inst, ok := m.instanceFor(af)
	if !ok {
		return v3InjectResult{}, afNotRunningError(af, m.runningAFs())
	}
	return inst.eng.debugInjectV3(rest)
}

// splitInjectAddressFamily takes the `address-family <af>` pair out of the inject arguments,
// wherever it stands among the other keywords, and answers the family it names
// (ipv6-unicast when none) with the arguments left for debugInjectV3. The registry does not
// validate the value of this modifier, so an unknown word, a missing value and a repeated
// keyword are refused here. No other inject value can spell the keyword: type and hex take
// hex digits, id a number and scope one of three words (ze-ospf-cmd.yang). The loop is
// bounded by the argument count.
func splitInjectAddressFamily(cmdArgs []string) (addressFamily, []string, error) {
	af := afIPv6Unicast
	named := false
	rest := make([]string, 0, len(cmdArgs))
	i := 0
	for i < len(cmdArgs) {
		if cmdArgs[i] != keywordAddressFamily {
			rest = append(rest, cmdArgs[i])
			i++
			continue
		}
		if named {
			return 0, nil, opererr.New(keywordAddressFamily+" is given more than once",
				"the LSA goes into the LSDB of one address family, so the keyword is given once",
				injectCommandNamingOnce(af, cmdArgs))
		}
		if i+1 == len(cmdArgs) {
			// Which families run is this router's state, so the fix lists them rather
			// than guess one for the operator.
			return 0, nil, opererr.New(keywordAddressFamily+" has no value",
				"the keyword is followed by one of "+afNameList(afRangeOrder)+
					", and the command below lists the ones running here",
				cmdShowIPv6Instance)
		}
		parsed, ok := afFromCanonicalName(cmdArgs[i+1])
		if !ok {
			return 0, nil, unknownAFError(cmdArgs[i+1])
		}
		af, named, i = parsed, true, i+2
	}
	return af, rest, nil
}

// injectCommandNamingOnce spells the inject command the operator typed with one
// address-family pair: the family the first pair named, then every other argument in the
// order given. Every address-family keyword is dropped, and so is the word after one
// unless that word is a keyword parseV3Inject takes (isV3InjectKeyword): a family name, an
// unknown word and a stray value all go, since the parser refuses any of them where they
// stand, while a word that starts another keyword's pair stays. It is the fix a repeated
// keyword's refusal names (splitInjectAddressFamily), a line the operator can run as it
// stands. The loop is bounded by the argument count.
func injectCommandNamingOnce(af addressFamily, cmdArgs []string) string {
	words := make([]string, 0, len(cmdArgs)+3)
	words = append(words, cmdDebugInjectLSA, keywordAddressFamily, af.String())
	i := 0
	for i < len(cmdArgs) {
		if cmdArgs[i] != keywordAddressFamily {
			words = append(words, cmdArgs[i])
			i++
			continue
		}
		i++
		if i == len(cmdArgs) {
			break
		}
		// A following address-family keyword is neither consumed nor kept here: the next
		// pass drops it as a keyword of its own.
		if cmdArgs[i] == keywordAddressFamily {
			continue
		}
		if !isV3InjectKeyword(cmdArgs[i]) {
			i++
		}
	}
	return strings.Join(words, " ")
}

// debugInjectV3 validates and (if debug is enabled) originates or withdraws a crafted
// OSPFv3 LSA in this engine's LSDB through the base origination seam. The arguments MUST
// carry no address-family keyword: it MUST be called with what debugInjectIntoAF left after
// splitInjectAddressFamily took the keyword out to choose this engine.
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

// isV3InjectKeyword reports whether word is one of the keywords parseV3Inject takes. It is
// the one list of them: parseV3Inject refuses any word it does not name before its switch
// dispatches, so a case added to that switch MUST be added here, or the keyword is refused.
// injectCommandNamingOnce reads it to keep a keyword the operator typed in its fix line.
func isV3InjectKeyword(word string) bool {
	switch word {
	case labelScope, injectKeywordType, injectKeywordID, injectKeywordHex, actionWithdraw:
		return true
	}
	return false
}

// parseV3Inject parses the keyword-before-value v6 inject grammar. Its keywords are the
// ones isV3InjectKeyword names, and any other word in a keyword position is refused.
func parseV3Inject(args []string) (v3InjectParams, error) {
	var p v3InjectParams
	haveType, haveID := false, false
	i := 0
	for i < len(args) {
		if !isV3InjectKeyword(args[i]) {
			return p, fmt.Errorf("unexpected inject argument: %s", args[i])
		}
		switch args[i] {
		case labelScope:
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			p.scopeKw, i = v, i+2
		case injectKeywordType:
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			n, err := parseHexUint16(v)
			if err != nil {
				return p, err
			}
			p.lsType, haveType, i = n, true, i+2
		case injectKeywordID:
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			n, err := injectParseUint(v, 32)
			if err != nil {
				return p, err
			}
			p.linkStateID, haveID, i = uint32(n), true, i+2
		case injectKeywordHex:
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

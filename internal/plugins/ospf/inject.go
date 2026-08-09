// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- guarded IPv4 opaque LSA inject.
// RFC: rfc/short/rfc5250.md (Section 3 LS-ID split; Section 9 Private-Use Opaque Type;
// Section 8 MinLSInterval pacing), rfc/short/rfc2328.md (Section 14 MaxAge withdraw).
//
// `debug ip ospf inject opaque scope <s> id <id> [type <t>] [hex <body> | tlv <t> <hex>...]
// [withdraw]` originates a crafted opaque LSA into THIS router's LSDB through the ext-1
// OriginateOpaque seam (which owns sequence/age/install/flood/pacing), then normal flooding
// carries it. It is DOUBLE-GATED: the read-only authz profile denies `debug`, AND the
// engine refuses unless `debug ospf inject enable` set the shared enablement (AC-16/AC-17).
// The default Opaque Type is Private-Use (128-255) so a test LSA never collides with a
// standards-track consumer (A-8). Withdraw MaxAge-flushes via the same seam (AC-15).

package ospf

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	// debugPrivateOpaqueType is the default injected Opaque Type: RFC 5250 Section 9
	// Private-Use (128-255), so a crafted test LSA never collides with TE(1)/grace(3)/RI(4).
	debugPrivateOpaqueType uint8 = 250
	// maxOpaqueID is the 24-bit ceiling of the Opaque ID (RFC 5250 App A.2).
	maxOpaqueID uint32 = 1<<24 - 1
	// maxLSABodyLen is the largest opaque body: the 16-bit LSA Length minus the 20-octet
	// header (RFC 2328 Section A.4.1).
	maxLSABodyLen = 65535 - types.LSAHeaderLen
	// privateOpaqueMin is the low bound of the RFC 5250 Section 9 Private-Use range.
	privateOpaqueMin = 128
)

// actionOriginate / actionWithdraw name the two inject actions (metric label + result).
const (
	actionOriginate = "originate"
	actionWithdraw  = "withdraw"
)

var (
	errDebugInjectDisabled = errors.New("debug injection not enabled (run `debug ospf inject enable`)")
	errNoRouterID          = errors.New("cannot inject: OSPF router-id is not set")
)

// injectResult is the typed JSON payload of an inject/withdraw action.
type injectResult struct {
	Action      string `json:"action"`
	Scope       string `json:"scope"`
	OpaqueType  uint8  `json:"opaque-type"`
	OpaqueID    uint32 `json:"opaque-id"`
	LinkStateID string `json:"link-state-id"`
	Installed   bool   `json:"installed"`
}

type opaqueInjectParams struct {
	scope      OpaqueScope
	opaqueType uint8
	opaqueID   uint32
	body       []byte
	withdraw   bool
}

// debugInjectOpaque validates and (if debug is enabled) originates or withdraws a crafted
// opaque LSA through the ext-1 seam.
func (e *engine) debugInjectOpaque(args []string) (injectResult, error) {
	if !debugInjectIsEnabled() {
		return injectResult{}, errDebugInjectDisabled
	}
	p, err := parseOpaqueInject(args)
	if err != nil {
		return injectResult{}, err
	}
	if e.cfg.RouterID == (types.RouterID{}) {
		return injectResult{}, errNoRouterID
	}
	res := injectResult{Scope: p.scope.String(), OpaqueType: p.opaqueType, OpaqueID: p.opaqueID}
	in := ospflsdb.OpaqueOriginateInput{
		Router:     e.cfg.RouterID,
		OpaqueType: p.opaqueType,
		OpaqueID:   p.opaqueID,
		Scope:      types.LSType(p.scope),
		Area:       types.BackboneArea,
		Interface:  e.firstInterfaceName(),
		Options:    types.OptionO,
		Body:       p.body,
		Withdraw:   p.withdraw,
	}
	h, ok := e.lsdb.OriginateOpaque(in)
	res.LinkStateID = h.LinkStateID.String()
	res.Installed = ok
	action := actionOriginate
	if p.withdraw {
		action = actionWithdraw
	}
	res.Action = action
	debugMetrics.Load().v4Inject.With(res.Scope, action).Inc()
	if ok {
		if p.withdraw {
			debugMetrics.Load().v4Injected.With(res.Scope).Dec()
		} else {
			debugMetrics.Load().v4Injected.With(res.Scope).Inc()
		}
	}
	return res, nil
}

// firstInterfaceName returns any current interface name (for a link-scope opaque LSA), or
// "" when none is configured.
func (e *engine) firstInterfaceName() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for name := range e.interfaces {
		return name
	}
	return ""
}

// parseOpaqueInject parses the keyword-before-value inject grammar. A malformed value or an
// out-of-range selector is rejected (never panics).
func parseOpaqueInject(args []string) (opaqueInjectParams, error) {
	p := opaqueInjectParams{opaqueType: debugPrivateOpaqueType}
	haveScope, haveID := false, false
	i := 0
	for i < len(args) {
		switch args[i] {
		case "scope":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			sc, err := parseOpaqueScopeKeyword(v)
			if err != nil {
				return p, err
			}
			p.scope, haveScope, i = sc, true, i+2
		case "id":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			n, err := injectParseUint(v, 32)
			if err != nil {
				return p, err
			}
			if n > uint64(maxOpaqueID) {
				return p, errors.New("opaque id exceeds 24 bits (max 16777215)")
			}
			p.opaqueID, haveID, i = uint32(n), true, i+2
		case "type":
			v, err := injectNextArg(args, i)
			if err != nil {
				return p, err
			}
			n, err := injectParseUint(v, 16)
			if err != nil {
				return p, err
			}
			if n < privateOpaqueMin || n > 255 {
				return p, errors.New("opaque type must be Private-Use (128-255)")
			}
			p.opaqueType, i = uint8(n), i+2
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
		case "tlv":
			if i+2 >= len(args) {
				return p, errors.New("tlv requires <type> <value-hex>")
			}
			tlv, err := buildInjectTLV(args[i+1], args[i+2])
			if err != nil {
				return p, err
			}
			p.body, i = append(p.body, tlv...), i+3
		case actionWithdraw:
			p.withdraw, i = true, i+1
		default:
			return p, fmt.Errorf("unexpected inject argument: %s", args[i])
		}
	}
	if !haveScope {
		return p, errors.New("inject requires a scope (link/area/as)")
	}
	if !haveID {
		return p, errors.New("inject requires an id")
	}
	if len(p.body) > maxLSABodyLen {
		return p, errors.New("opaque body exceeds the LSA maximum length")
	}
	return p, nil
}

// buildInjectTLV assembles one 4-byte-aligned opaque TLV from a decimal type and a hex value.
func buildInjectTLV(typeStr, valueHex string) ([]byte, error) {
	t, err := injectParseUint(typeStr, 16)
	if err != nil {
		return nil, err
	}
	val, err := decodeHexArg(valueHex)
	if err != nil {
		return nil, err
	}
	out := []byte{byte(t >> 8), byte(t), byte(len(val) >> 8), byte(len(val))}
	out = append(out, val...)
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out, nil
}

func parseOpaqueScopeKeyword(s string) (OpaqueScope, error) {
	switch s {
	case "link":
		return OpaqueScopeLink, nil
	case scopeAreaName:
		return OpaqueScopeArea, nil
	case "as":
		return OpaqueScopeAS, nil
	default:
		return 0, fmt.Errorf("unknown opaque scope (want link/area/as): %s", s)
	}
}

// injectNextArg returns the argument after position i, or an error when it is missing.
func injectNextArg(args []string, i int) (string, error) {
	if i+1 >= len(args) {
		return "", fmt.Errorf("%s requires a value", args[i])
	}
	return args[i+1], nil
}

// injectParseUint parses a base-10 unsigned integer of the given bit size.
func injectParseUint(s string, bits int) (uint64, error) {
	n, err := strconv.ParseUint(s, 10, bits)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", s, err)
	}
	return n, nil
}

// decodeHexArg decodes a hex body/value string, rejecting an odd-length or non-hex string.
func decodeHexArg(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex %q: %w", s, err)
	}
	return b, nil
}

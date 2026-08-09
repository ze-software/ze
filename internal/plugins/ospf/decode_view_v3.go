// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- IPv6 native LSA deep decode.
// RFC: rfc/short/rfc5340.md (Section A.4.2.1: scope-aware LS Type; Section A.4: base types),
// rfc/short/rfc5838.md (Section 2: address-family identity of the OSPFv3 instance).
//
// `show ospf ipv6 database <type> detail` / `... database scope <link|area|as>` render each
// native OSPFv3 LSA with its 20-octet header, the RFC 5340 Section A.4.2.1 scope decoded
// from the S2/S1 bits, and a DECODED body: a registered decoder (the base eight + extended +
// Grace, keyed by the neutral LS Type) renders named fields; else the generic scope-aware
// header + body-hex view. A malformed body renders as raw hex, bumps
// ze_ospfv3_debug_decode_errors_total, and never panics (AC-4/AC-24). The per-scope filter
// keys on the S2/S1 bits, NEVER a flat OSPFv2 type number.

package ospf

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v3ScopeBits is the RFC 5340 Section A.4.2.1 S2/S1 flooding-scope mask.
const v3ScopeBits types.LSType = 0x6000

// scopeLinkLocal / scopeAreaName / scopeASName are the rendered flooding-scope names.
const (
	scopeLinkLocal = "link-local"
	scopeAreaName  = "area"
	scopeASName    = "as"
)

var errReservedScope = errors.New("reserved LSA flooding scope (S2/S1 = 11)")

// v3DetailDecoderFn decodes a fully-parsed v3 LSA into a typed, JSON-renderable value. It
// returns an error on a malformed body; the view then falls back to generic + raw hex.
type v3DetailDecoderFn func(l *ospfv3packet.LSA) (any, error)

type v3DetailDecoder struct {
	name string
	fn   v3DetailDecoderFn
}

var (
	v3DetailMu       sync.RWMutex
	v3DetailDecoders = map[types.LSType]v3DetailDecoder{}
)

// registerV3DetailDecoder registers a typed decoder for one neutral OSPFv3 LS Type.
func registerV3DetailDecoder(t types.LSType, name string, fn v3DetailDecoderFn) {
	if fn == nil {
		return
	}
	v3DetailMu.Lock()
	defer v3DetailMu.Unlock()
	if _, exists := v3DetailDecoders[t]; exists {
		return
	}
	v3DetailDecoders[t] = v3DetailDecoder{name: name, fn: fn}
}

func lookupV3DetailDecoder(t types.LSType) (v3DetailDecoder, bool) {
	v3DetailMu.RLock()
	defer v3DetailMu.RUnlock()
	d, ok := v3DetailDecoders[t]
	return d, ok
}

// registerV3BaseDecoders registers the base eight (RFC 5340 Section A.4) plus the RFC 5187
// Grace-LSA and the RFC 8362 extended LSAs -- the OSPFv3 codec's own decoders -- as detail
// defaults. Called once from registerOSPF (registration lives in register*.go, not init()).
func registerV3BaseDecoders() {
	base := []struct {
		wire ospfv3types.LSType
		name string
		fn   v3DetailDecoderFn
	}{
		{ospfv3types.LSTypeRouter, "router", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeRouter() }},
		{ospfv3types.LSTypeNetwork, "network", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeNetwork() }},
		{ospfv3types.LSTypeInterAreaPrefix, "inter-area-prefix", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeInterAreaPrefix() }},
		{ospfv3types.LSTypeInterAreaRouter, "inter-area-router", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeInterAreaRouter() }},
		{ospfv3types.LSTypeASExternal, "as-external", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeExternal() }},
		{ospfv3types.LSTypeNSSA, "nssa", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeExternal() }},
		{ospfv3types.LSTypeLink, "link", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeLink() }},
		{ospfv3types.LSTypeIntraAreaPrefix, "intra-area-prefix", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeIntraAreaPrefix() }},
		{ospfv3types.LSTypeGrace, "grace", func(l *ospfv3packet.LSA) (any, error) { return l.DecodeGrace() }},
	}
	for _, b := range base {
		registerV3DetailDecoder(v6NeutralLSType(b.wire), b.name, b.fn)
	}
	extended := []struct {
		wire ospfv3types.LSType
		name string
	}{
		{ospfv3types.LSTypeERouter, "e-router"},
		{ospfv3types.LSTypeENetwork, "e-network"},
		{ospfv3types.LSTypeEInterAreaPrefix, "e-inter-area-prefix"},
		{ospfv3types.LSTypeEASExternal, "e-as-external"},
		{ospfv3types.LSTypeEType7, "e-nssa"},
		{ospfv3types.LSTypeELink, "e-link"},
		{ospfv3types.LSTypeEIntraAreaPrefix, "e-intra-area-prefix"},
	}
	for _, x := range extended {
		registerV3DetailDecoder(v6NeutralLSType(x.wire), x.name,
			func(l *ospfv3packet.LSA) (any, error) { return ospfv3packet.DecodeExtendedLSABody(l.Body) })
	}
}

// v3DetailLSA is one OSPFv3 LSA rendered with its scope-aware header and a decoded body.
type v3DetailLSA struct {
	LSType            string `json:"ls-type"`
	LSTypeHex         string `json:"ls-type-hex"`
	Scope             string `json:"scope"`
	Area              string `json:"area,omitempty"`
	Interface         string `json:"interface,omitempty"`
	AdvertisingRouter string `json:"advertising-router"`
	LinkStateID       string `json:"link-state-id"`
	Age               uint16 `json:"age"`
	Length            int    `json:"length"`
	Decoder           string `json:"decoder"`
	Decoded           any    `json:"decoded,omitempty"`
	BodyHex           string `json:"body-hex,omitempty"`
	Malformed         bool   `json:"malformed,omitempty"`
	LocalOriginated   bool   `json:"local-originated,omitempty"`
}

// v3DetailDatabase wraps the decoded OSPFv3 LSAs with their address-family identity.
type v3DetailDatabase struct {
	AddressFamily string        `json:"address-family"`
	InstanceID    uint8         `json:"instance-id"`
	LSAs          []v3DetailLSA `json:"lsas"`
}

// v3TypeMatches reports whether an LSA of type t (rendered name) passes the type filter. An
// empty filter matches all; "extended" matches the RFC 8362 extended LS types (function
// codes 0x21-0x29); otherwise the rendered name must equal the filter.
func v3TypeMatches(filter string, t types.LSType, name string) bool {
	switch filter {
	case "":
		return true
	case "extended":
		return isV3ExtendedType(t)
	default:
		return filter == name
	}
}

// isV3ExtendedType reports whether t is one of the RFC 8362 extended OSPFv3 LS types.
func isV3ExtendedType(t types.LSType) bool {
	fc := t & 0x1FFF
	return fc >= 0x21 && fc <= 0x29
}

// v3ScopeName renders the RFC 5340 Section A.4.2.1 flooding scope from the S2/S1 bits.
func v3ScopeName(t types.LSType) string {
	switch t & v3ScopeBits {
	case 0x0000:
		return scopeLinkLocal
	case 0x2000:
		return scopeAreaName
	case 0x4000:
		return scopeASName
	default:
		return "reserved"
	}
}

// v3ScopeSelector maps an operator scope keyword to the S2/S1 scope name it filters to. An
// unknown or reserved keyword is rejected (AC-8).
func v3ScopeSelector(sel string) (string, error) {
	switch sel {
	case "", "all":
		return "", nil
	case extRegistryLink, scopeLinkLocal:
		return scopeLinkLocal, nil
	case scopeAreaName:
		return scopeAreaName, nil
	case scopeASName:
		return scopeASName, nil
	default:
		return "", errReservedScope
	}
}

// v3DatabaseDetailSnapshot renders `show ospf ipv6 database [<type> detail | scope <s>]`:
// every native OSPFv3 LSA (optionally filtered by LS-type name and/or flooding scope),
// its scope-aware header, and a decoded body. Read-only over the v6 engine's LSDB.
func (e *engine) v3DatabaseDetailSnapshot(typeFilter, scopeFilter string) ([]any, error) {
	wantScope, err := v3ScopeSelector(scopeFilter)
	if err != nil {
		return nil, err
	}
	db := v3DetailDatabase{AddressFamily: e.af.String(), LSAs: []v3DetailLSA{}}
	if e.dispatch != nil {
		db.InstanceID = e.dispatch.currentInstanceID()
	}
	if e.lsdb == nil {
		return []any{db}, nil
	}
	self := e.cfg.RouterID
	for _, v := range e.lsdb.AllLSAViews() {
		scope := v3ScopeName(v.Type)
		if wantScope != "" && scope != wantScope {
			continue
		}
		name := v.Type.String()
		if !v3TypeMatches(typeFilter, v.Type, name) {
			continue
		}
		row := v3DetailLSA{
			LSType:            name,
			LSTypeHex:         v3LSTypeHex(v.Type),
			Scope:             scope,
			Interface:         v.Interface,
			AdvertisingRouter: v.AdvertisingRouter.String(),
			LinkStateID:       v.LinkStateID.String(),
			Age:               v.Age,
			Length:            len(v.Body),
			LocalOriginated:   v.AdvertisingRouter == self,
		}
		if scope != scopeLinkLocal {
			row.Area = v.Area.String()
		}
		e.decodeV3Body(&row, v)
		db.LSAs = append(db.LSAs, row)
	}
	return []any{db}, nil
}

// decodeV3Body fills a row's decode fields: a registered decoder first, else the generic
// header + body-hex view; a fault bumps the v6 decode-error metric (never panics).
func (e *engine) decodeV3Body(row *v3DetailLSA, v ospflsdb.NativeLSAView) {
	d, ok := lookupV3DetailDecoder(v.Type)
	if !ok {
		row.Decoder = decoderGeneric
		row.BodyHex = hex.EncodeToString(v.Body)
		return
	}
	decoded, err := safeV3Decode(d.fn, v.RawBytes)
	if err != nil {
		row.Decoder = decoderRaw
		row.Malformed = true
		row.BodyHex = hex.EncodeToString(v.Body)
		debugMetrics.Load().v6Decode.With(v3LSTypeHex(v.Type)).Inc()
		return
	}
	row.Decoder = d.name
	row.Decoded = decoded
}

// safeV3Decode re-parses the raw LSA through the v3 codec and runs the typed decoder under
// a recover wrapper (bound-checked codec + recover: a malformed body cannot crash).
func safeV3Decode(fn v3DetailDecoderFn, raw []byte) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			v, err = nil, errDecodePanicked
		}
	}()
	lsa, derr := ospfv3packet.DecodeLSA(raw)
	if derr != nil {
		return nil, derr
	}
	return fn(&lsa)
}

// v3LSTypeHex renders a 16-bit LS Type as a stable 0x-prefixed lowercase hex string.
func v3LSTypeHex(t types.LSType) string {
	var tb textbuf.Buffer
	return tb.Str("0x").Hex([]byte{byte(t >> 8), byte(t)}).String()
}

// Design: plan/learned/1052-ospf-ext-14-debug-introspection.md -- IPv4 opaque-LSA deep decode.
// RFC: rfc/short/rfc5250.md (Section 3 / App A.2: Opaque Type/ID split, generic TLVs),
// rfc/short/rfc2328.md (Section A.4.1: LSA header fields the detail view renders).
//
// `show ospf database opaque-<scope> detail` renders every opaque LSA of a scope with its
// header metadata and a DECODED body: a registered typed decoder (TE/RI/Extended/SR/Grace,
// keyed by Opaque Type) renders named content; otherwise the ext-1 generic TLV iterator
// yields (type, length, value-hex) rows. A malformed body falls back to raw hex and bumps
// ze_ospf_debug_decode_errors_total; it never panics (AC-3/AC-24). Generic code in this
// file spells NO consumer body format: each typed decoder is registered self-containedly by
// its owning consumer (ai/rules/plugin-self-containment.md).

package ospf

import (
	"encoding/hex"
	"errors"
	"sync"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// errDecodePanicked is returned when a typed decoder panics under the recover wrapper.
var errDecodePanicked = errors.New("opaque decoder panicked")

// decoderGeneric / decoderRaw name the fallback rendering modes.
const (
	decoderGeneric = "generic"
	decoderRaw     = "raw"
)

// opaqueDetailDecoderFn decodes an opaque LSA body (the bytes after the 20-octet header)
// into a typed, JSON-renderable value. It returns an error on a malformed body; the view
// then falls back to the generic TLV / raw-hex rendering. It MUST NOT retain the body.
type opaqueDetailDecoderFn func(body []byte) (any, error)

type opaqueDetailDecoder struct {
	name string
	fn   opaqueDetailDecoderFn
}

var (
	opaqueDetailMu       sync.RWMutex
	opaqueDetailDecoders = map[uint8]opaqueDetailDecoder{}
)

// registerOpaqueDetailDecoder registers a typed decoder for one Opaque Type (RFC 5250
// Section 9). Called from the owning consumer's package (ext-2 TE, ext-3 RI, ext-4
// Extended, ext-5 SR, ext-9 Grace) so removing the consumer removes its decoder and the
// generic fallback remains. A duplicate registration is ignored (idempotent across the
// per-instance engine factory).
func registerOpaqueDetailDecoder(opaqueType uint8, name string, fn opaqueDetailDecoderFn) {
	if fn == nil {
		return
	}
	opaqueDetailMu.Lock()
	defer opaqueDetailMu.Unlock()
	if _, exists := opaqueDetailDecoders[opaqueType]; exists {
		return
	}
	opaqueDetailDecoders[opaqueType] = opaqueDetailDecoder{name: name, fn: fn}
}

func lookupOpaqueDetailDecoder(opaqueType uint8) (opaqueDetailDecoder, bool) {
	opaqueDetailMu.RLock()
	defer opaqueDetailMu.RUnlock()
	d, ok := opaqueDetailDecoders[opaqueType]
	return d, ok
}

// opaqueTLVRow is one generic (type, length, value-hex) TLV row of the fallback rendering.
type opaqueTLVRow struct {
	Type     uint16 `json:"type"`
	Length   int    `json:"length"`
	ValueHex string `json:"value-hex"`
}

// opaqueDetailLSA is one opaque LSA rendered with its RFC 2328 Section A.4.1 header
// metadata and a decoded body (typed, generic TLVs, or raw hex).
type opaqueDetailLSA struct {
	Scope             string         `json:"scope"`
	Area              string         `json:"area,omitempty"`
	Interface         string         `json:"interface,omitempty"`
	AdvertisingRouter string         `json:"advertising-router"`
	LinkStateID       string         `json:"link-state-id"`
	OpaqueType        uint8          `json:"opaque-type"`
	OpaqueID          uint32         `json:"opaque-id"`
	Age               uint16         `json:"age"`
	Length            int            `json:"length"`
	Decoder           string         `json:"decoder"`
	Decoded           any            `json:"decoded,omitempty"`
	TLVs              []opaqueTLVRow `json:"tlvs,omitempty"`
	BodyHex           string         `json:"body-hex,omitempty"`
	Malformed         bool           `json:"malformed,omitempty"`
	LocalOriginated   bool           `json:"local-originated,omitempty"`
}

// opaqueDetailDatabase wraps the decoded opaque LSAs for one scope.
type opaqueDetailDatabase struct {
	OpaqueDetail []opaqueDetailLSA `json:"opaque-detail"`
}

// safeOpaqueDetailDecode runs a typed decoder under a recover wrapper so a panicking
// decoder cannot crash the engine (AC-24). A panic is reported as an error.
func safeOpaqueDetailDecode(fn opaqueDetailDecoderFn, body []byte) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			v, err = nil, errDecodePanicked
		}
	}()
	return fn(body)
}

// opaqueDetailSnapshot renders `show ospf database opaque-<scope> detail`: every opaque
// LSA of the requested scope, its header, and a decoded body. Read-only over the LSDB.
func (e *engine) opaqueDetailSnapshot(scope OpaqueScope) []any {
	base := e.databaseSnapshotByType(dbSubviewType[opaqueScopeCommand(scope)])
	rows := e.opaqueDetailRows(scope)
	return append(base, opaqueDetailDatabase{OpaqueDetail: rows})
}

// opaqueDetailRows builds the decoded rows for the opaque LSAs of one scope.
func (e *engine) opaqueDetailRows(scope OpaqueScope) []opaqueDetailLSA {
	if e.lsdb == nil {
		return nil
	}
	want := types.LSType(scope)
	self := e.cfg.RouterID
	var rows []opaqueDetailLSA
	for _, v := range e.lsdb.AllLSAViews() {
		if v.Type != want || !v.Type.IsOpaque() {
			continue
		}
		opaqueType := packet.OpaqueTypeOf(v.LinkStateID)
		row := opaqueDetailLSA{
			Scope:             scope.String(),
			Interface:         v.Interface,
			AdvertisingRouter: v.AdvertisingRouter.String(),
			LinkStateID:       v.LinkStateID.String(),
			OpaqueType:        opaqueType,
			OpaqueID:          packet.OpaqueIDOf(v.LinkStateID),
			Age:               v.Age,
			Length:            len(v.Body),
			LocalOriginated:   v.AdvertisingRouter == self,
		}
		if scope != OpaqueScopeLink {
			row.Area = v.Area.String()
		}
		e.decodeOpaqueBody(&row, opaqueType, v.Body)
		rows = append(rows, row)
	}
	return rows
}

// decodeOpaqueBody fills the decode fields of a row: a registered typed decoder first,
// else the generic TLV iterator, else raw hex; a fault bumps the decode-error metric.
func (e *engine) decodeOpaqueBody(row *opaqueDetailLSA, opaqueType uint8, body []byte) {
	if d, ok := lookupOpaqueDetailDecoder(opaqueType); ok {
		if decoded, err := safeOpaqueDetailDecode(d.fn, body); err == nil {
			row.Decoder = d.name
			row.Decoded = decoded
			return
		}
		// Typed decode failed: count it and fall through to the generic rendering.
		debugMetrics.Load().v4Decode.With(opaqueTypeLabel(opaqueType)).Inc()
	}
	tlvs, err := packet.DecodeOpaqueTLVs(body)
	for _, t := range tlvs {
		row.TLVs = append(row.TLVs, opaqueTLVRow{Type: t.Type, Length: t.Length, ValueHex: hex.EncodeToString(t.Value)})
	}
	if err != nil {
		row.Malformed = true
		row.BodyHex = hex.EncodeToString(body)
		debugMetrics.Load().v4Decode.With(opaqueTypeLabel(opaqueType)).Inc()
		if row.Decoder == "" {
			row.Decoder = decoderRaw
		}
		return
	}
	if row.Decoder == "" {
		row.Decoder = decoderGeneric
	}
}

// opaqueScopeCommand maps a scope to the base database subview command for reuse of
// databaseSnapshotByType's per-scope filter.
func opaqueScopeCommand(scope OpaqueScope) string {
	switch scope {
	case OpaqueScopeLink:
		return "show ospf database opaque-link"
	case OpaqueScopeAS:
		return "show ospf database opaque-as"
	default:
		return "show ospf database opaque-area"
	}
}

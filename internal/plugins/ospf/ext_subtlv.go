// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md -- the RFC 7684 sub-TLV registration hook.
// RFC: rfc/short/rfc7684.md -- sec 2 (Extended Prefix TLV sub-TLVs), sec 3.1 (Extended Link
// TLV sub-TLVs); the registries are seeded with only Reserved (0), so all sub-TLV VALUES are
// owned by downstream applications (RFC 8665, spec-ospf-ext-5).
//
// registerPrefixSubTLV / registerLinkSubTLV are the in-process hooks a downstream OSPF
// prefix/link-attribute application (spec-ospf-ext-5) uses to attach its own sub-TLVs to the
// Extended Prefix and Extended Link TLVs, without this carrier naming that application. They mirror the ext-1
// opaque consumer registry and the ext-3 registerRITLV hook one level down: the consumer
// registers from its own init(); the Extended TLV originator discovers the builders and the
// receive path dispatches matching sub-TLVs to the registered codec. Every codec callback is
// recover-wrapped (AC-16), so one bad consumer cannot crash OSPF. The hooks are UNEXPORTED:
// the only consumer is in-package (the wiring-completeness lesson from ext-1/ext-3).

package ospf

import (
	"errors"
	"net/netip"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// extSubTLVContext is the origination context passed to a registered sub-TLV builder so a
// downstream consumer can attach the right sub-TLV to the prefix or link the Extended TLV
// describes. Only the fields relevant to the registry (prefix vs link) are set.
type extSubTLVContext struct {
	Router types.RouterID
	// Prefix registry fields: the advertised prefix and its RFC 7684 sec 2.1 Route Type.
	Prefix    netip.Prefix
	RouteType uint8
	// Link registry fields: the RFC 2328 A.4.2 Link Type / Link ID / Link Data.
	LinkType uint8
	LinkID   [4]byte
	LinkData [4]byte
}

// extSubTLVCodec is a downstream consumer's encode/decode/render for one Extended Prefix or
// Extended Link sub-TLV type. Every callback is optional (nil = no contribution) and
// recover-wrapped (AC-16): a panicking codec contributes/dispatches nothing and cannot crash
// OSPF or wedge the LSDB.
type extSubTLVCodec struct {
	// Build contributes sub-TLVs to an originated Extended TLV (a downstream application
	// attaches its attribute sub-TLVs). nil contributes nothing (the empty-container carrier).
	Build func(ctx extSubTLVContext) []packet.ExtSubTLV
	// Receive is dispatched for each received sub-TLV of the registered type.
	Receive func(value []byte)
	// Render returns a display string for a received sub-TLV value (`show ospf database
	// opaque-area`); nil falls back to the generic type/length/hex rendering.
	Render func(value []byte) string
}

var (
	extSubTLVMu   sync.RWMutex
	prefixSubTLVs = map[uint16]extSubTLVCodec{}
	linkSubTLVs   = map[uint16]extSubTLVCodec{}
)

// ErrExtSubTLVRegistered is returned when a consumer registers a sub-TLV type another consumer
// already owns.
var ErrExtSubTLVRegistered = errors.New("ospf: extended sub-TLV type already registered")

// ErrExtSubTLVReserved is returned when a consumer registers sub-TLV type 0, reserved in the
// RFC 7684 sec 6.2 / sec 6.5 sub-TLV registries.
var ErrExtSubTLVReserved = errors.New("ospf: extended sub-TLV type 0 is reserved")

// registerPrefixSubTLV registers a downstream consumer's codec for one Extended Prefix TLV
// sub-TLV type (RFC 7684 sec 2). A downstream consumer (spec-ospf-ext-5) calls it from its own
// init(); the Extended Prefix originator invokes Build and the receive path dispatches a
// matching sub-TLV to Receive. Type 0 (Reserved) and duplicates are rejected.
func registerPrefixSubTLV(subType uint16, codec extSubTLVCodec) error {
	return registerExtSubTLV(prefixSubTLVs, subType, codec)
}

// registerLinkSubTLV registers a downstream consumer's codec for one Extended Link TLV sub-TLV
// type (RFC 7684 sec 3.1). Type 0 (Reserved) and duplicates are rejected.
func registerLinkSubTLV(subType uint16, codec extSubTLVCodec) error {
	return registerExtSubTLV(linkSubTLVs, subType, codec)
}

// registerExtSubTLV inserts codec for subType into reg, rejecting the Reserved type 0 and a
// duplicate. reg is a reference type shared under extSubTLVMu.
func registerExtSubTLV(reg map[uint16]extSubTLVCodec, subType uint16, codec extSubTLVCodec) error {
	if subType == 0 {
		return ErrExtSubTLVReserved
	}
	extSubTLVMu.Lock()
	defer extSubTLVMu.Unlock()
	if _, dup := reg[subType]; dup {
		return ErrExtSubTLVRegistered
	}
	reg[subType] = codec
	return nil
}

// resetExtSubTLVs clears both registries. Test-only helper; production never calls it.
func resetExtSubTLVs() {
	extSubTLVMu.Lock()
	prefixSubTLVs = map[uint16]extSubTLVCodec{}
	linkSubTLVs = map[uint16]extSubTLVCodec{}
	extSubTLVMu.Unlock()
}

// lookupExtSubTLV returns the codec registered for subType in reg.
func lookupExtSubTLV(reg map[uint16]extSubTLVCodec, subType uint16) (extSubTLVCodec, bool) {
	extSubTLVMu.RLock()
	defer extSubTLVMu.RUnlock()
	c, ok := reg[subType]
	return c, ok
}

// extSubTLVEntry pairs a sub-TLV type with its codec for ordered builder iteration.
type extSubTLVEntry struct {
	subType uint16
	codec   extSubTLVCodec
}

// snapshotExtSubTLVs returns the registered codecs of reg in ascending type order, for
// deterministic, idempotent origination.
func snapshotExtSubTLVs(reg map[uint16]extSubTLVCodec) []extSubTLVEntry {
	extSubTLVMu.RLock()
	out := make([]extSubTLVEntry, 0, len(reg))
	for typ, c := range reg {
		out = append(out, extSubTLVEntry{subType: typ, codec: c})
	}
	extSubTLVMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].subType < out[j].subType })
	return out
}

// dispatchPrefixSubTLV delivers a received Extended Prefix sub-TLV to its registered codec, if
// any. An unregistered type is skipped without error (RFC 7684 forward-compatibility). The
// codec's Receive runs recover-wrapped (AC-16); onPanic increments the sub-TLV-error metric.
func dispatchPrefixSubTLV(s packet.ExtSubTLV, onPanic func()) {
	dispatchExtSubTLV(prefixSubTLVs, s, onPanic)
}

// dispatchLinkSubTLV delivers a received Extended Link sub-TLV to its registered codec.
func dispatchLinkSubTLV(s packet.ExtSubTLV, onPanic func()) {
	dispatchExtSubTLV(linkSubTLVs, s, onPanic)
}

func dispatchExtSubTLV(reg map[uint16]extSubTLVCodec, s packet.ExtSubTLV, onPanic func()) {
	codec, ok := lookupExtSubTLV(reg, s.Type)
	if !ok || codec.Receive == nil {
		return
	}
	safeOpaqueCall(onPanic, func() { codec.Receive(s.Value) })
}

// buildPrefixSubTLVs returns the sub-TLVs every registered Extended Prefix consumer wants
// appended for ctx, in ascending sub-TLV-type order. Empty until a consumer (ext-5) registers.
func buildPrefixSubTLVs(ctx extSubTLVContext, onPanic func()) []packet.ExtSubTLV {
	return buildExtSubTLVs(prefixSubTLVs, ctx, onPanic)
}

// buildLinkSubTLVs returns the sub-TLVs every registered Extended Link consumer wants appended.
func buildLinkSubTLVs(ctx extSubTLVContext, onPanic func()) []packet.ExtSubTLV {
	return buildExtSubTLVs(linkSubTLVs, ctx, onPanic)
}

func buildExtSubTLVs(reg map[uint16]extSubTLVCodec, ctx extSubTLVContext, onPanic func()) []packet.ExtSubTLV {
	entries := snapshotExtSubTLVs(reg)
	out := make([]packet.ExtSubTLV, 0, len(entries))
	for _, e := range entries {
		out = append(out, invokeExtSubTLVBuild(e.codec, ctx, onPanic)...)
	}
	return out
}

// invokeExtSubTLVBuild runs a registered builder under panic recovery (AC-16/R-9): a panicking
// builder contributes no sub-TLVs and onPanic increments the sub-TLV-error metric, so a single
// bad consumer cannot wedge origination.
func invokeExtSubTLVBuild(codec extSubTLVCodec, ctx extSubTLVContext, onPanic func()) (out []packet.ExtSubTLV) {
	defer func() {
		if r := recover(); r != nil {
			out = nil
			if onPanic != nil {
				onPanic()
			}
		}
	}()
	if codec.Build == nil {
		return nil
	}
	return codec.Build(ctx)
}

// renderExtSubTLV returns a registered codec's display string for a received sub-TLV value, or
// "" when no codec (or Render) is registered so the caller falls back to type/length/hex.
func renderExtSubTLV(reg map[uint16]extSubTLVCodec, s packet.ExtSubTLV) string {
	codec, ok := lookupExtSubTLV(reg, s.Type)
	if !ok || codec.Render == nil {
		return ""
	}
	var out string
	safeOpaqueCall(nil, func() { out = codec.Render(s.Value) })
	return out
}

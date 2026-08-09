// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- the RFC 7770 RI-TLV registration hook.
// RFC: rfc/short/rfc7770.md -- sec 2.3 (RI LSA TLV stream), sec 2.4 (type-1 TLV first),
// sec 2.7 (per-TLV flooding scope).
//
// registerRITLV is the in-process hook a downstream OSPF extension (Segment Routing / ext-5)
// uses to inject its own TLVs (SR-Algorithm / SRGB / SRLB) into the SAME RI LSA, without this
// spec naming Segment Routing. It mirrors the ext-1 opaque consumer registry: the consumer
// registers from its own init(), the RI originator discovers the builders at origination time,
// and removing the consumer removes its registerRITLV call and all its TLVs
// (ai/rules/plugins.md). It is UNEXPORTED because the only consumer is
// in-package (the same wiring-completeness lesson as the ext-1 opaque registry).

package ospf

import (
	"errors"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// riTLVBuildFunc builds the RI TLVs a downstream consumer wants appended to the RI LSA for
// the originating router. The RI originator emits them AFTER the RFC 7770 sec 2.4 type-1
// Informational Capabilities TLV, in ascending registered-TLV-type order. A nil or empty
// return contributes nothing. The builder is invoked on the self-LSA origination pass and is
// recover-wrapped (AC-8): a panic contributes no TLVs and cannot crash the engine.
type riTLVBuildFunc func(router types.RouterID) []packet.RITLV

// riTLVEntry is one registered RI-TLV builder: its TLV type, the flooding scope it applies
// to (RFC 7770 sec 2.7 per-TLV scope), and the build function.
type riTLVEntry struct {
	tlvType uint16
	scope   OpaqueScope
	build   riTLVBuildFunc
}

var (
	riTLVMu       sync.RWMutex
	riTLVBuilders = map[uint16]riTLVEntry{}
)

// ErrRITLVRegistered is returned when a consumer registers a TLV type another consumer owns.
var ErrRITLVRegistered = errors.New("ospf: RI TLV type already registered")

// ErrRITLVScopeInvalid is returned when a consumer registers with a scope that is not one of
// the RFC 5250 opaque flooding scopes (link/area/AS).
var ErrRITLVScopeInvalid = errors.New("ospf: invalid RI TLV scope (must be link/area/AS)")

// ErrRITLVReserved is returned when a consumer registers TLV type 1 or 2, reserved for the RI
// consumer's own Informational (sec 2.4) / Functional (sec 2.6) Capabilities TLVs.
var ErrRITLVReserved = errors.New("ospf: RI TLV types 1 and 2 are reserved for the RI consumer")

// registerRITLV registers a downstream consumer's RI LSA TLV builder for one TLV type at one
// flooding scope. A consumer (Segment Routing / ext-5) calls it from its own init(); the RI
// originator invokes the builder during origination and appends its TLVs after the type-1
// Informational Capabilities TLV, in ascending TLV-type order. Registering a duplicate type,
// an invalid scope, or a reserved type (1/2) is rejected.
func registerRITLV(tlvType uint16, scope OpaqueScope, build riTLVBuildFunc) error {
	if !scope.valid() {
		return ErrRITLVScopeInvalid
	}
	if tlvType == packet.RITLVInformationalCapabilities || tlvType == packet.RITLVFunctionalCapabilities {
		return ErrRITLVReserved
	}
	riTLVMu.Lock()
	defer riTLVMu.Unlock()
	if _, dup := riTLVBuilders[tlvType]; dup {
		return ErrRITLVRegistered
	}
	riTLVBuilders[tlvType] = riTLVEntry{tlvType: tlvType, scope: scope, build: build}
	return nil
}

// riTLVBuildersForScope returns the registered builders whose flooding scope matches scope,
// in ascending TLV-type order (RFC 7770 sec 2.4: registered TLVs follow the type-1 TLV in a
// deterministic order, giving byte-identical, idempotent re-origination).
func riTLVBuildersForScope(scope OpaqueScope) []riTLVEntry {
	riTLVMu.RLock()
	defer riTLVMu.RUnlock()
	out := make([]riTLVEntry, 0, len(riTLVBuilders))
	for _, e := range riTLVBuilders {
		if e.scope == scope {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].tlvType < out[j].tlvType })
	return out
}

// resetRITLVs clears the registry. Test-only helper; production never calls it.
func resetRITLVs() {
	riTLVMu.Lock()
	riTLVBuilders = map[uint16]riTLVEntry{}
	riTLVMu.Unlock()
}

// invokeRITLVBuilder runs a registered builder under panic recovery (AC-8/R-6). onPanic
// increments ze_ospf_ri_tlv_builder_errors_total; a panicking builder contributes no TLVs and
// the RI LSA is emitted without them, so a single bad consumer cannot wedge origination.
func invokeRITLVBuilder(e riTLVEntry, router types.RouterID, onPanic func()) (tlvs []packet.RITLV) {
	defer func() {
		if r := recover(); r != nil {
			tlvs = nil
			if onPanic != nil {
				onPanic()
			}
		}
	}()
	if e.build == nil {
		return nil
	}
	return e.build(router)
}

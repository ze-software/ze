# 434 -- apply-mods

## Context

The reactor forward path could only accept or suppress routes per destination peer via egress filters. The `ModAccumulator` infrastructure existed but nothing applied accumulated modifications. RFC 9234 Section 5 requires OTC egress stamping (adding OTC = local ASN when forwarding to Customer/Peer/RS-Client), which was the first consumer. The mod framework also unblocks LLGR readvertisement which needs community addition, local-pref modification, and withdrawal conversion.

## Decisions

- Chose progressive single-pass build (Option B) over full-copy-then-patch (Option A) because most mod types change payload size (OTC add, community append), and the attribute walk is needed anyway to find targets. One pass vs two.
- Mods run after wire selection (EBGP/IBGP), not before. EBGP wires are pre-built once per ForwardUpdate; mods must apply to the peer-specific wire with AS-PATH already prepended.
- Flat `AttrOp` structure (code uint8, action uint8, buf []byte) over string-keyed mods. Integer code enables O(1) comparison during attribute walk. Multiple ops on same code accumulate naturally.
- Per-attribute-code `AttrModHandler` over generic callbacks. Each attribute type has its own semantics (scalar set, list add/remove, sequence prepend). Handler receives source bytes + all ops + output buffer + offset.
- Egress filters pre-build value bytes. The progressive build engine is generic; attribute knowledge lives in handlers.

## Consequences

- Future LLGR/community/AS-PATH mods register handlers by attr code and write `AttrOp` entries. The framework is ready.
- Zero-copy fast path preserved: `mods.Len() == 0` skips the build entirely.
- Pool buffer returned via defer on all exit paths including panic.
- Handler return offset is validated; invalid offsets cause fallback to source copy.

## Gotchas

- Buffer slack is 256 bytes. Handlers that expand attributes significantly (many communities) could exceed it. The build returns nil (abandon modification) rather than panic. Future: dynamic buffer growth if needed.
- `AttrOp.Buf` stores a slice reference (not a copy). Go escape analysis keeps the backing array alive, but this is implicit. If a filter reuses a buffer across calls, data corruption is possible.
- The `[256]bool` consumed array is stack-allocated but 256 bytes. Negligible at BGP scale.

## Files

- `internal/component/plugin/registry/registry.go` -- AttrOp, AttrModHandler, ModAccumulator.Op()
- `internal/component/bgp/reactor/forward_build.go` -- progressive build engine (NEW)
- `internal/component/bgp/reactor/forward_build_test.go` -- 13 tests (NEW)
- `internal/component/bgp/reactor/reactor_api_forward.go` -- wired buildModifiedPayload
- `internal/component/bgp/reactor/reactor.go` -- attrModHandlers field
- `internal/component/bgp/plugins/role/otc.go` -- otcAttrModHandler + egress filter migration
- `internal/component/bgp/plugins/role/register.go` -- RegisterAttrModHandler(35)
- `docs/architecture/core-design.md` -- documented progressive build

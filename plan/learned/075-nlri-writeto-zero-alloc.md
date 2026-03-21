# 075 — NLRI WriteTo Zero Alloc

## Objective

Eliminate allocations in NLRI `WriteTo()` methods that were internally calling `Pack()`/`Bytes()` and defeating the zero-alloc contract.

## Decisions

- Chose full zero-alloc WriteTo with cached-bytes fallback (not just copy-from-cache): user explicitly requested true zero-alloc; the fallback handles parsed NLRIs where components are not populated.
- EVPN ADD-PATH bug deferred to a separate spec: fixing it correctly was a larger scope than this spec warranted, and the bug pre-existed.
- LabeledUnicast was already zero-alloc when investigated — confirmed, no changes needed.

## Patterns

- Fallback pattern: if cached bytes exist but components are empty (parsed NLRI, not constructed), copy from cache; otherwise write directly to buffer. Avoids forcing callers to re-construct parsed NLRIs.
- `WriteTo` added to `FlowComponent` and descriptors to propagate zero-alloc through the component tree.

## Gotchas

- EVPN types have a pre-existing ADD-PATH bug: `Bytes()` excludes path ID, but `packEVPN()` assumes it is included when `hasPath=true`. EVPN WriteTo was intentionally left using `copy(buf, Pack(ctx))` to avoid making the bug worse.
- `RouteDistinguisher.WriteTo` helper added to `ipvpn.go` for reuse by VPN types.

## Files

- `internal/bgp/nlri/flowspec.go` — zero-alloc WriteTo + FlowComponent.WriteTo
- `internal/bgp/nlri/bgpls.go` — zero-alloc WriteTo + descriptor WriteTo
- `internal/bgp/nlri/other.go` — MVPN/VPLS/RTC/MUP zero-alloc WriteTo
- `internal/bgp/nlri/writeto_test.go` — new verification tests

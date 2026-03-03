# 322 — Allocation Reduction: Umbrella

## Objective

Coordinate three allocation reduction efforts on the BGP UPDATE hot path, identified from pprof analysis showing 17.9 GB total allocations dominated by text serialize→deserialize, per-batch slice allocations, and `fmt.Sprintf`/`netip.Addr.String()` calls.

## Decisions

- Dropped child 2 (AttributesWire pooling + bitset) — user judged complexity vs gain not justified
- Children 1 and 3 executed first (independent, lower risk); child 4 executed last (architectural, highest effort)
- No re-profiling ordered between children — proceeded in planned order

## Patterns

- Per-worker reusable fields are safe when workers are long-lived goroutines processing serially
- DirectBridge already bypassed socket transport but still received text — the next step was bypassing text formatting itself
- bgp-rs used text content only for withdrawal map tracking; actual forwarding uses cache IDs — structured delivery eliminates only the parsing, not the forwarding mechanism

## Gotchas

- Child 4 required three design iterations: eager StructuredEvent (violated lazy-first) → UpdateHandle wrapper (identity wrapper) → direct wire access. Third attempt was also the simplest.
- Profile was captured before UTP-2/3 Scanner work; Scanner had already reduced some parsing allocations. No re-profile was done, but child 4 proceeded anyway.
- The two identity-wrapper mistakes in child 4 prompted rule additions to `design-principles.md` and `before-writing-code.md`

## Files

- Umbrella only; implementation in children 319, 320, 321

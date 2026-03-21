# 055 — FlowSpec Params Type Inconsistency (Closed By Design)

## Objective

Investigate whether FlowSpecParams should use `Communities []uint32` instead of `CommunityBytes []byte` for consistency with UnicastParams/VPNParams. Closed as intentional.

## Decisions

- FlowSpec pre-packs communities at config load time (raw bytes) — no per-announcement repacking needed
- Chosen over unifying to `[]uint32`: would touch 4+ files for negligible benefit on a low-volume build path (~10–100 rules, set once at session establishment)
- The forward path for received routes uses `Route.wireBytes` (separate system), not FlowSpecParams at all
- Resolution: added a documentation comment explaining the design choice; no code change

## Patterns

- ZeBGP has two distinct paths: Build path (low volume, local origination via *Params structs) vs Forward path (high volume, zero-copy wire bytes)
- Optimising the Build path matters less; the Forward path already uses `Route.wireBytes`

## Gotchas

- None.

## Files

- `internal/bgp/message/update_build.go` — documentation comment added to FlowSpecParams

# 082 — Handler Integration

## Objective

Wire the update text parser (spec 081) to the reactor via `peer <addr> update text ...` command handler and `AnnounceNLRIBatch`/`WithdrawNLRIBatch` reactor methods.

## Decisions

- Handler fails fast on first group error; reactor uses `lastErr` pattern (tries all peers, collects errors): handler-level atomicity (parse-then-announce) vs reactor-level best-effort (all peers attempted).
- Reactor handles wire format and size splitting, not the handler: wire format requires peer-specific context (ADD-PATH, ASN4, max message size) that only the reactor has.
- Added `ErrNoPeersAcceptedFamily` when all peers skip due to family not negotiated: partial success (some peers accept, some skip) returns `done` with a `warnings` field.
- New `"warning"` response status alongside `"done"` and `"error"`: empty result (no routes to process) or all-peers-skipped-family deserves a distinct status.
- `NLRIBatch` is a new type (not existing per-route types): existing `RouteSpec`/`L3VPNRoute` are one-NLRI-each; `NLRIBatch` groups multiple NLRIs sharing attributes for efficient UPDATE building.

## Patterns

- Two split paths: `sendUpdateWithSplit` (build path, API-originated routes) vs `SplitWireUpdate` (forward path, received UPDATEs). Never mix.

## Gotchas

- Reactor methods use `*reactorAPIAdapter` receiver, not `*Reactor` — easy to add a method to the wrong type.

## Files

- `internal/plugin/types.go` — NLRIBatch, ReactorInterface additions
- `internal/plugin/update_text.go` — handleUpdateText, handler functions
- `internal/plugin/route.go` — "update" command registered
- `internal/reactor/reactor.go` — AnnounceNLRIBatch, WithdrawNLRIBatch

# 1081 -- unify-replay

## Context
Late-join replay (a new consumer needs the existing table re-sent) was implemented three
structurally different ways: `ribevents.ReplayRequest` and `sysribevents.ReplayRequest` were
payload-less `RegisterSignal` broadcasts (sysrib->rib, FIB->sysrib), while `redistevents.ReplayRequest`
was a payload-carrying token that lets the redistribute orchestrator target the ONE newly-established
BGP peer. DESIGN-REVIEW finding 2 + section 5 flagged this as one problem solved twice/thrice, with a
protocol-agnostic leaf (`redistevents`) carrying replay semantics that exist for one consumer. Goal:
one request payload type + one response marker convention, preserving all observable behavior, without
merging the three per-subsystem handlers.

## Decisions
- Canonical shape = the token-correlated request (broadcast is the special case where the token
  addresses everyone) over making everything a payload-less signal: a signal cannot target one
  consumer, but a reserved sentinel trivially expresses "broadcast".
- Extracted the shared vocabulary into a NEW neutral leaf `internal/core/replay`
  (`Request{ReplayID}`, `Broadcast = math.MaxUint64`, `IsReplay(token)`) over generalizing it inside
  `redistevents`, to answer DESIGN-REVIEW section 5 (redistevents was already over-loaded). `redistevents.ReplayRequest`
  is a `type = replay.Request` alias, so every producer/orchestrator compiled with zero churn.
- Retired the two write-only `Replay bool` fields to `ReplayID uint64 json:"-"` + `IsReplay()` +
  custom `MarshalJSON`/`UnmarshalJSON` mapping the token to/from the legacy `replay` bool wire, over
  keeping the bool: best-change round-trips through JSON in-tree (`fibvpp` `parseBatch`, forked path),
  so the wire tag must survive decode->encode while the token stays the single source of truth.
- Kept three distinct per-hop `(namespace,eventType)` handles all bound to `*replay.Request` over one
  shared handle: directions and subscriber sets differ, so one handle would cross-trigger unrelated hops.

## Consequences
- One mental model for all replay; deleted two identical `RegisterSignal("*","replay-request")` patterns
  and two write-only bools. Broadcast hops emit `replay.Broadcast`; the handler ignores the token.
- `RegisterSignal`/`SignalEvent`/`signalType` remain a supported events primitive with ZERO users after
  this change (no signal events remain in the tree); the dispatch signal branch is still exercised by tests.
- The `type alias` + embedded-pointer-struct marshaler is the pattern to add/override ONE JSON field
  without hand-writing a full marshaler and without recursion (`type alias T` strips the method set).

## Gotchas
- Changing a WIDELY-IMPORTED core leaf (`redistevents`) makes `scripts/dev/changed-pkgs.sh` return ~200
  reverse-dep packages, so `ze-lint-changed` and `ze-validate` lint/scan that whole closure and surface
  PRE-EXISTING debt in every file the change edits. Budget for this when touching a core leaf.
- `make ze-lint` was ALREADY red on `main` (a `goconst` on `intra-area`) independent of this change;
  goconst counts only NON-test occurrences (the `_test.go` exclusion suppresses findings in tests but the
  count is non-test), so removing one duplicate non-test literal (de-dup `routeTypeName`->`String()`) cleared it.
- `validate.py` (ze-validate) is diff-scoped and flags EVERY exported symbol in a changed file lacking a
  cross-package non-test caller. It cannot see registration func-values (`RunEngine: RunRIBPlugin`),
  same-package-only use, or map-VALUE type usage (`map[K]*PeerMeta`), so it false-positives on conventionally
  exported entry points/constructors/types. Editing a file for a one-line comment surfaces all of them.
- `encoding/json` never dispatches a pointer-receiver `MarshalJSON` on a nil pointer (returns `null`), so
  the custom best-change marshaler is nil-safe without a guard.
- Every other plugin already assigns an UNEXPORTED func to `RunEngine` (`runPlugin`/`runEngine`/...);
  `rib.RunRIBPlugin` was the lone exported one, so unexporting it was a consistency fix, not churn.

## Files
- Created: `internal/core/replay/replay.go` (+`replay_test.go`), `internal/core/bgp/ribevents/ribevents_test.go`.
- Replay vocabulary: `internal/core/redistevents/events.go` (alias + IsReplay delegate), `internal/core/events/typed.go` (doc + removed dead `IsSignal`).
- Broadcast hops: `rib/events/events.go`, `sysrib/events/events.go` (token marker + Marshal/Unmarshal), `rib.go`, `rib_bestchange.go`, `sysrib.go`, `fib/{kernel,vpp,p4}`.
- Tests: `rib_bestchange_test.go`, `sysrib_test.go`, `redistevents/replay_test.go`.
- Docs: `docs/architecture/core-design.md` (unified-vocabulary rewrite + source anchor).
- Pre-existing gate debt (user-approved): `ospf/spf/explain.go` (de-dup) + `route_delta_test.go`; rib unexports (`RunRIBPlugin`/`NewRIBManager`/`PeerMeta`) across rib `*_test.go`; `ospf/spf` `RunCount`->`runCount`.

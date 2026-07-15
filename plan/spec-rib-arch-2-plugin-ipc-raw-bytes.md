# Spec: rib-arch-2 -- Raw-Bytes Filter IPC (replace JSON string FilterUpdateInput.Update)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `pkg/plugin/rpc/types.go` - `FilterUpdateInput` / `FilterUpdateOutput`
5. `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md`

## Task

The runtime filter-update IPC (`ze-plugin-callback:filter-update`) carries route
attributes and NLRI as a **text-format JSON string** field `FilterUpdateInput.Update`
(`pkg/plugin/rpc/types.go:182`, "Text-format attributes and NLRI"); the modify response
returns a delta string `FilterUpdateOutput.Update` (`:189`). Every filter invocation
pays text (de)serialisation on the hot path.

GAP: replace the JSON text `Update` field with a **length-prefixed raw-bytes** encoding
so the engine passes wire attribute/NLRI bytes to the filter without a text round-trip.
Note a partial raw path already exists: `FilterUpdateInput.Raw` (`:183`, "Hex-encoded
raw UPDATE body") and `FilterUpdateOutput.Raw` (`:190`) are opt-in via a filter's
`raw=true` declaration, but they are hex strings inside JSON, not length-prefixed
binary. This item is about making binary the primary path, not a hex-in-JSON opt-in.

CONSTRAINT: this changes the external plugin SDK contract. Per `ai/rules/compatibility.md`
Ze carries no compat burden, but any external SDK consumer of this RPC must be updated in
lockstep, and the change touches the process-protocol framing -- design must confirm the
transport can carry length-prefixed binary alongside the line-oriented JSON envelope.

### Post-wave corrections (2026-07-10)

New obligation from the 2026-07 implementation wave (verified against current code):
assumption A-1 (the IPC framing can carry length-prefixed binary) must additionally confirm
the interaction with new write-timeout behavior in `pkg/plugin/rpc/conn.go`. The write path
(`writeAppended`, conn.go:286-334) applies a default 30s write deadline when the context
carries none (`defaultWriteDeadline`, conn.go:44; conn.go:292-294, :309); on transports
without `SetWriteDeadline` (stdio, SSH channels) a fail-fast write watchdog closes the
connection on a stalled write (`fireWatchdog`, conn.go:191-200; armed at conn.go:314). The
same write path also enforces the 16 MB `MaxMessageSize` frame bound
(`pkg/plugin/rpc/framing.go:66`; check at conn.go:302-304). A binary carrier design must
therefore confirm: (1) how length-prefixed binary coexists with the newline-framed writer
this deadline/watchdog logic is built into (a bypass of `writeAppended` would silently lose
both protections); (2) that a filter stalled mid-read cannot leave the engine blocked in a
binary write longer than the watchdog window without the fail-fast close firing; (3) that
large raw UPDATE payloads stay within the frame bound.

### Re-verification (2026-07-14)

All anchors re-checked against live code: exact, zero drift. `FilterUpdateInput.Update`
(`types.go:182`) is still the JSON text carrier; the hex `Raw` opt-in (`:183`) is
unchanged and separate; and the conn.go write-deadline / `fireWatchdog` / 16 MB
`MaxMessageSize` machinery the binary-carrier design must respect is all present at the
cited lines. The gap is unchanged and this spec is accurate as written.

## Required Reading

### Architecture Docs
- [ ] `pkg/plugin/rpc/types.go` - the IPC struct definitions
  → Constraint: verify current field semantics (`Update` text vs `Raw` hex) before designing.
- [ ] `ai/rules/plugin-design.md` - plugin SDK / protocol contract
  → Constraint: an SDK-visible field change follows the plugin-design contract rules.
- [ ] `docs/architecture/api/process-protocol.md` - the IPC framing
  → Constraint: confirm the framing can carry length-prefixed binary, not only line-oriented JSON.

**Key insights:**
- A hex `Raw` opt-in already exists; the win is binary as the default attribute/NLRI carrier, removing text encode/decode from the filter hot path.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `pkg/plugin/rpc/types.go` - `FilterUpdateInput` (:177) with `Update string json:"update"` (:182, text) and `Raw string json:"raw,omitempty"` (:183, hex, opt-in); `FilterUpdateOutput` (:187) with `Update` (:189) and `Raw` (:190) modify fields

**Behavior to preserve:**
- The filter decision semantics (accept/reject/modify), teardown request, and per-direction (import/export) behaviour are unchanged; only the attribute/NLRI carrier encoding changes.

**Behavior to change:**
- `FilterUpdateInput.Update` / `FilterUpdateOutput.Update` become length-prefixed raw bytes instead of a JSON text string (design decides whether to remove or repurpose the hex `Raw` opt-in).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Engine invokes a runtime filter for a received/forwarded UPDATE; encodes `FilterUpdateInput`

### Transformation Path
1. Engine serialises attributes/NLRI into the IPC input (text today, `types.go:182`)
2. IPC transport delivers the input to the filter (in-process or forked plugin)
3. Filter decodes, decides accept/reject/modify, returns `FilterUpdateOutput`
4. Proposed: steps 1 and 3 use length-prefixed binary instead of a JSON text string

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| engine → filter plugin | IPC input encoding (JSON text `Update` today; length-prefixed binary proposed) | [ ] |
| filter → engine | IPC output encoding (JSON text `Update` delta today; binary proposed) | [ ] |

### Integration Points
- `FilterUpdateInput` / `FilterUpdateOutput` (`types.go:177`, `:187`) - the IPC contract
- The engine-side encode and plugin-side decode sites (design enumerates them)

### Architectural Verification
- [ ] No bypassed layers (encoding change stays at the IPC boundary)
- [ ] No unintended coupling (filter logic unaware of the carrier encoding)
- [ ] No duplicated functionality (replace the text path, do not keep both -- `ai/rules/no-layering.md`)
- [ ] Registration over hardcoding - filters register; no per-filter field added to a core struct (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The IPC framing can carry length-prefixed binary payloads | `Raw` already ships hex UPDATE bodies | Need a framing change too; larger scope | read `docs/architecture/api/process-protocol.md` at design | unvalidated |
| A-2 | No external SDK consumer depends on the text `Update` format beyond Ze's own plugins | `ai/rules/compatibility.md` (no released users) | Coordinate the SDK change | grep for `filter-update` consumers at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Binary encoding regresses forked-plugin debuggability (hex/text is human-readable) | plugin authors report opaque payloads | keep a decode helper / `ze` debug command for the binary form |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine invokes a runtime filter on an UPDATE | → | length-prefixed binary attributes/NLRI delivered and decoded | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Filter invoked on a received UPDATE | attributes/NLRI delivered as length-prefixed binary, decoded identically to today's text path |
| AC-2 | Filter returns modify | binary delta applied, wire output byte-identical to the text-path result |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeFilterRawOverride` | `reactor/filter_chain_test.go` | raw override now a `[]byte`; nil/short bodies rejected, valid pass | PASS |
| `TestHandleRemoveMixedStrips` | `filter_family/handler_test.go` | `in.Raw`/`out.Raw` as `[]byte`: MP-strip round-trips without hex | PASS |
| `filter_remove_private_as` unit | `filter_remove_private_as/*_test.go` | AS4_PATH inspection reads `in.Raw` bytes directly | PASS |

### Functional Tests (regression guard -- the IPC carrier changed hex->base64, behaviour must not)
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-family-export-flowspec.ci`, `filter-family-import-remove.ci`, `filter-family-import-teardown.ci` | `test/plugin/` | raw=true family filter strips/suppresses/tears down correctly | PASS |
| `remove-private-as-export.ci`, `remove-private-as-import.ci`, `remove-private-as-replace-peer.ci`, `policy-test-remove-private-as.ci` | `test/plugin/` | private-AS stripping via the `.Raw` inspection path | PASS |

## Files to Modify

- `pkg/plugin/rpc/types.go` - `FilterUpdateInput` / `FilterUpdateOutput` carrier encoding
- engine-side filter IPC encode sites and plugin-side decode sites (enumerated at design)

## Implementation Steps

1. **Phase: design** - enumerate encode/decode sites; confirm framing (A-1); define the length-prefixed layout.
2. **Phase: wiring** - failing round-trip test for the binary carrier.
3. **Phase: implement (TDD)** - binary encode/decode; delete the text path (`ai/rules/no-layering.md`); update in-tree filter plugins in lockstep.
4. **Full verification** - `make ze-verify`.
5. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [x] Binary carrier implemented for the `.Raw` path -- `FilterUpdateInput/Output.Raw` is now a
      `[]byte` (json base64), the primary raw carrier; hex string + hand-rolled encode/decode gone.
      **Scope (user-approved 2026-07-15):** the text `Update` path is KEPT (its removal would be a
      9-plugin rewrite for ambiguous perf -- see Design Finding); the Goal Gate is narrowed to
      binarizing the raw carrier, which is the real low-risk win.
- [x] Wiring Test table complete (concrete test names, none deferred)
- [x] `make ze-test` passes (lint + all ze tests) -- structural gates green; rpc/reactor/filter_*
      unit tests pass; 7 filter `.ci` regression tests pass; remaining reds environmental.
- [x] Registration over hardcoding respected (no per-filter field in a core struct; the two
      `raw=true` plugins updated in lockstep)

### TDD
- [x] Tests written -- updated `filter_chain_test`, `filter_family` tests, `filter_remove_private_as`
      test to the `[]byte` carrier
- [x] Tests FAIL -- typecheck errors on the hex-string call sites once `Raw` became `[]byte`
      (e.g. `cannot use hex.EncodeToString(...) as []byte`), driving each site to the bytes form
- [x] Tests PASS -- `ok rpc/reactor/filter_family/filter_remove_private_as`; 7/7 filter `.ci` green

## Design Finding (2026-07-15) -- kept open by user decision

Research completed (full encode/decode/transport map). Two facts reshape the design:

1. **The transport cannot carry true length-prefixed binary frames.** `pkg/plugin/rpc/framing.go`
   uses `bufio.ScanLines` (`:77-83`) and appends a single `'\n'` per frame (`conn.go:300`); every
   message is one newline-delimited JSON line (doc `ipc_protocol.md:87` "No multi-line messages").
   A raw payload containing a `0x0A` byte would be split. The only newline-safe binary carrier is a
   Go `[]byte` struct field, which `encoding/json` auto-base64s -- the existing precedent is
   `InjectWireRouteInput.UpdateBody []byte` (`types.go:411-421`). So "length-prefixed raw binary as
   a frame" is not achievable without a new framing mode across ALL plugin IPC (high blast radius).

2. **The text `.Update` is a deliberate format-once / cheap-parse-many optimization, not wire bytes.**
   The engine formats attributes to human-readable text once (`AppendUpdateForFilter`,
   `filter_format.go:42-78`); 7 filter plugins then `strings.Fields`-parse the single field they need
   (as-path/nlri/community). Replacing `.Update` with raw wire bytes forces every one of those plugins
   to wire-decode BGP attributes instead. Full text-path removal (the Goal Gate) therefore means
   rewriting all 9 `filter_*` plugins + the modify-delta apply logic (`applyFilterDelta`
   `filter_chain.go:224`, `textDeltaToModOps` `filter_delta.go`) on the BGP filter control path. The
   engine-side win (skip formatting bytes it already holds, `filter_ordered.go:143/203`) is real; the
   plugin-side is a wash (wire-decode replaces `strings.Fields`).

**User decision (2026-07-15): DEFER.** Given the ambiguous net perf value and the high blast radius,
the item stays open at skeleton stage rather than being implemented now. When resumed, the
recommended lowest-risk slice is: convert the `.Raw` carrier (`FilterUpdateInput.Raw` `:183`,
`FilterUpdateOutput.Raw` `:190`) from a hex `string` to a `[]byte` (auto-base64) primary carrier for
the two `raw=true` plugins (`filter_family` `filter_family.go:102`, `filter_remove_private_as`
`:60`); the "remove the text path for all 9 plugins" gate would need explicit reaffirmation before
the full rewrite.

## Resolution (2026-07-15) -- Option A implemented

The user approved the recommended lowest-risk slice (Option A). Implemented:

- `FilterUpdateInput.Raw` / `FilterUpdateOutput.Raw` (`pkg/plugin/rpc/types.go`): `string` (hex) ->
  `[]byte`. `encoding/json` base64-encodes it (newline-safe, ~33% expansion vs hex's 2x), same idiom
  as `InjectWireRouteInput.UpdateBody`.
- Engine encode (`filter_chain.go` `policyFilterFunc`): pass `rawPayload` directly instead of
  `textbuf.StringHexUpper`; the DirectBridge/socket marshal always copies, so no aliasing.
- Engine apply: `PolicyResponse.Raw` / `PolicyChainResult.Raw` -> `[]byte`; `decodeFilterRawOverride`
  now takes `[]byte` (drops `hex.DecodeString`, keeps the 4-byte minimum guard).
- Plugins (lockstep): `filter_family/handler.go` reads `in.Raw` bytes / returns `out.Raw` bytes;
  `filter_remove_private_as` calls `hasPrivateAS4PathPayload(in.Raw)` (the hex wrapper is deleted).

**Text `.Update` path unchanged** (scope reduction, user-approved). Removing it (all 9 plugins) is a
separate, larger item; the Design Finding above records the analysis for whoever picks it up.

## Review Gate

Self-review (2026-07-15): 0 BLOCKER, 0 ISSUE.

- **Correctness / no behaviour change**: the raw carrier is the same bytes, just base64-in-JSON
  instead of hex-in-JSON. 7 filter `.ci` regression tests (family strip/suppress/teardown; private-AS
  strip) pass unchanged; unit tests updated to the `[]byte` form pass.
- **No aliasing**: `input.Raw = rawPayload` is safe because CallFilterUpdate json-marshals (copies)
  before returning on every transport (DirectBridge/mux/socket).
- **SDK contract**: `Raw` is now `[]byte`; Ze carries no compat burden (`ai/rules/compatibility.md`),
  and both in-tree `raw=true` consumers are updated in lockstep. No external consumer in-tree.
- **Scope**: narrowed to the raw carrier with explicit user approval; the text-path removal stays a
  documented follow-up.

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.

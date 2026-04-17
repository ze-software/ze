# Spec: fmt-2-json-append

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fmt-0-append (completed, see `plan/learned/614-fmt-0-append.md`) |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `.claude/rules/buffer-first.md` — Append vs WriteTo discipline
4. `plan/learned/614-fmt-0-append.md` — the pattern this spec extends
5. `internal/component/bgp/format/decode.go` — primary migration target
6. `internal/component/bgp/format/json.go` — one-line migration
7. `internal/component/bgp/format/format_buffer.go` — dead code being deleted
8. `.claude/hooks/block-encoding-alloc.sh` — model for the new hook

## Task

Close the residual cold-path allocation sites in the BGP message-format
package and install a PreToolUse hook that enforces fmt-0's banned-pattern
discipline across every file that fmt-0 migrated. Scope is deliberately the
"S2+S4" slice of the original fmt-2 proposal:

1. **Delete dead code.** `internal/component/bgp/format/format_buffer.go` and
   its test file contain seven `Format*JSON(io.Writer)` helpers plus
   `FormatPrefixFromBytes`. Repo-wide grep shows **zero production callers**
   anywhere outside the file itself and its test. fmt-0 rewrote the JSON
   emission path (`text_json.go appendFilterResultJSON`) without touching
   these helpers, and `text_json.go` has never called them. The file is
   dead; delete it.
2. **Migrate `decode.go` off `fmt.Sprintf`.** 14 sites in `formatCapability`,
   `notificationSubcodeString` family, `DecodeRouteRefresh`, `afiToString`,
   and `afiSafiToFamily`. Output bytes must stay identical — existing tests
   in `message_receiver_test.go` are the regression guard.
3. **Migrate `json.go:166` off `fmt.Sprintf`.** Single `fmt.Sprintf("%x",
   notify.Data)` site in the notification JSON writer. Replace with
   `hex.AppendEncode`. The remainder of `json.go` (`map[string]any` +
   `json.Marshal`) is intentionally out of scope — migrating the whole
   `JSONEncoder` to buffer-first is a separate effort (unfiled).
4. **Install `.claude/hooks/block-format-alloc.sh`.** A PreToolUse hook
   modeled on `block-encoding-alloc.sh` that rejects Write/Edit of a fixed
   allowlist of `format/*.go` and `reactor/filter_format.go` files
   containing `fmt.Sprintf`, `fmt.Fprintf`, `strings.Join`, `strings.Builder`,
   `strings.NewReplacer`, `strings.ReplaceAll`, `strconv.FormatUint`,
   `strconv.FormatInt`. fmt-0 left the guard as hand-maintained discipline
   on three files; this hook makes it mechanical and extends it to cover
   `decode.go` once migrated.

Out of scope:
- Rewriting `JSONEncoder` buffer-first (the S3 tier — large scope, cold path,
  deferred until a concrete perf need emerges).
- Any UPDATE-path change (closed by fmt-0).
- Plugin IPC raw-bytes transport (tracked separately as
  `spec-plugin-ipc-raw-bytes`).

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/buffer-first.md` — Append shape and banned-pattern discipline
  → Constraint: No `make` in wire-encoding helpers; caller owns buffer.
  → Constraint: banned primitives list (`fmt.Sprintf`, `strings.Builder`, `strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`, `strconv.FormatUint`, `strconv.FormatInt`) — `decode.go` becomes a newly-guarded file.
- [ ] `plan/learned/614-fmt-0-append.md` — idiom fmt-2 extends
  → Decision: Signature idiom is `AppendXxx(buf []byte, args...) []byte` for text generation; helpers use `strconv.AppendUint`, `netip.Addr.AppendTo`, `hex.AppendEncode`.
  → Decision: Hand-maintained banned-pattern list of three files (`reactor/filter_format.go`, `format/text.go`, `attribute/text.go`) — this spec graduates that list to a real hook.
  → Constraint: "Element types emit bare values; dispatchers emit names" — `DecodedCapability.Value` is a bare value, `json.go`/`text.go` prepend `name`.
- [ ] `.claude/rules/no-test-deletion.md` — permits deletion when testing removed functionality
  → Constraint: deleting `format_buffer_test.go` is legitimate because the functions it tests are being removed.
- [ ] `docs/architecture/api/json-format.md` — JSON key and value contract
  → Constraint: hex data in NOTIFICATION `"data"` field must stay lowercase, no `0x` prefix (matches `hex.AppendEncode` output exactly).
- [ ] `.claude/hooks/block-encoding-alloc.sh` — model implementation
  → Decision: shell script + `jq` + grep idiom; exit 2 to block; skip `_test.go`; file-path allowlist via `case` statement; skip empty/non-Go edits.

### RFC Summaries
None.

**Key insights:**
- fmt-0 closed every hot-path text allocation. What remains in `format/` are
  cold-path sites (OPEN / NOTIFICATION / ROUTE-REFRESH / capability decoding)
  that fire once per session or on rare events. Migrating them does not move
  a benchmark; it closes the banned-pattern surface area so future work
  (fmt-3-* etc.) starts from a clean `format/` tree.
- `fmt.Sprintf("%d", uint32)` and `strconv.AppendUint(buf, x, 10)` produce
  byte-identical decimal for all unsigned input sites in `decode.go`. Every
  int-formatting site in this migration is unsigned (`uint8` subcode,
  `uint16` AFI, `uint32` ASN). No sign-bit concerns.
- `fmt.Sprintf("%x", []byte)` and `hex.EncodeToString(src)` /
  `hex.AppendEncode(dst, src)` produce byte-identical lowercase hex. Safe
  swap.
- `DecodedCapability.Value` stays typed `string`. The migration cannot
  eliminate the heap allocation for the field itself; it eliminates the
  `fmt` reflection overhead (~2–4× CPU cost) and makes the file bannable
  under the new hook. Going fully zero-alloc on this path would require
  restructuring `DecodedCapability` (S3-scope, out of this spec).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/format/format_buffer.go` (225L) — contains
  `FormatPrefixFromBytes` plus 5 `Format*JSON(io.Writer)` helpers
  (`FormatASPathJSON`, `FormatCommunitiesJSON`, `FormatOriginJSON`,
  `FormatMEDJSON`, `FormatLocalPrefJSON`) and `wellKnownCommunityName`.
  7 `fmt.Fprintf(w, ...)` call sites inside the 5 writers. Repo-wide grep
  for each function name returns matches only in this file, its test file,
  `plan/deferrals.md`, this spec, and `docs/architecture/buffer-architecture.md`.
  → Constraint: preserve the doc reference (or update it when removing the
  helpers).
- [ ] `internal/component/bgp/format/format_buffer_test.go` (225L) — unit
  tests for the dead helpers plus `TestFormatPrefixFromBytes`. All tests
  exercise code that has zero production callers.
- [ ] `internal/component/bgp/format/decode.go` (463L) — produces API-friendly
  `DecodedOpen`, `DecodedNotification`, `DecodedRouteRefresh`,
  `DecodedNegotiated` structs. 14 `fmt.Sprintf` sites in `formatCapability`
  (5), `notificationSubcodeString` family (5 fallbacks), `DecodeRouteRefresh`
  (1), `afiToString` (1), `afiSafiToFamily` (2).
  → Constraint: every consumer reads string fields (`.Value`,
  `.ErrorSubcodeName`, `.Family`, `.Families[]`) — the struct shape cannot
  change without touching every caller. Keep the struct shape; only change
  how the strings are produced.
- [ ] `internal/component/bgp/format/json.go` (276L) — cold-path JSON
  encoder for STATE / OPEN / NOTIFICATION / KEEPALIVE / ROUTE-REFRESH /
  NEGOTIATED. Uses `map[string]any` + `json.Marshal`. One `fmt.Sprintf("%x",
  notify.Data)` at line 166.
  → Constraint: the `map[string]any` + `json.Marshal` idiom is preserved
  (out-of-scope for this spec). Only the hex-formatting site changes.
- [ ] `internal/component/bgp/format/message_receiver_test.go` (455L) —
  existing tests for `DecodeOpen`, `DecodeNotification`, `DecodeRouteRefresh`,
  `NegotiatedToDecoded`. Includes `DecodedCapability` equality assertions
  (`Value: "ipv4/unicast"`, `"0200"`, `"65536"`) that are the byte-level
  regression guard.
  → Constraint: do not weaken any existing assertion; add new tests for the
  fallback branches (`"Subcode(N)"`, `"unknown(N)"`, `"afi(N)"`,
  `"safi(N)"`).
- [ ] `internal/component/bgp/server/events.go` (lines 340-393) —
  `formatMessageForSubscription` dispatches on encoding mode. Text mode
  calls `format.AppendOpen(buf, peer, DecodedOpen, ...)` etc.; JSON mode
  calls `JSONEncoder.Open(peer, DecodedOpen, ...)`. Both consume
  `DecodedOpen.Capabilities[].Value` as a string.
  → Constraint: both consumers must see byte-identical output — both text
  mode and JSON mode read the migrated `Decoded*` fields.
- [ ] `.claude/hooks/block-encoding-alloc.sh` (162L) — model for the new
  hook. Uses `jq` to extract tool name, file path, and content; greps the
  content for banned patterns; exit 2 to block.
  → Constraint: new hook must (a) only process Write/Edit on `.go` files,
  (b) skip `_test.go`, (c) use a file allowlist via `case` matching,
  (d) grep the edit content (not the on-disk file).

**Behavior to preserve:**
- `DecodedCapability.Value` byte-level format for every capability:
  `"ipv4/unicast"` (multiprotocol), numeric-as-string for ASN4 (e.g.
  `"65536"`), `"ipv4/unicast receive"` (addpath receive),
  `"ipv4/unicast send-receive"` (addpath both), skip `none`-mode entries,
  `"ipv4/unicast ipv6"` (extended-nexthop), hex for unknown caps
  (lowercase, no `0x`), `"unknown-<code>"` name for unknown caps.
- `DecodedNotification.ErrorSubcodeName` byte-level format:
  `"Unspecific"` for subcode 0 with no specific mapping, `"Subcode(<n>)"`
  fallback for unmapped subcodes in Open/Update/Header error codes,
  specific RFC-text names for mapped subcodes.
- `DecodedRouteRefresh.SubtypeName` byte-level format: `"refresh"`,
  `"borr"`, `"eorr"`, or `"unknown(<n>)"` fallback.
- `afiSafiToFamily` output: `"ipv4/unicast"`, `"ipv6/multicast"`,
  `"ipv4/mpls-labels"`, `"ipv4/mpls-vpn"`, `"afi(<n>)/safi(<n>)"`
  fallback.
- `afiToString` output: `"ipv4"`, `"ipv6"`, `"afi(<n>)"` fallback.
- `JSONEncoder.Notification` `"data"` field: lowercase hex, empty string if
  `notify.Data` is empty.
- All existing tests in `message_receiver_test.go` pass unmodified.

**Behavior to change:**
- None. Pure refactor + dead-code removal + new guard.

## Data Flow (MANDATORY)

### Entry Point
- Production: `bgp/server/events.go formatMessageForSubscription` is the
  sole production consumer of `DecodeOpen` / `DecodeNotification` /
  `DecodeRouteRefresh` / `JSONEncoder.*`. Called per non-UPDATE BGP
  message received or sent on a peer subscription.
- The 5 `Format*JSON(io.Writer)` helpers in `format_buffer.go`: zero
  production callers.

### Transformation Path
1. Wire bytes arrive → message reactor classifies into OPEN / NOTIFICATION /
   ROUTE-REFRESH / KEEPALIVE.
2. `formatMessageForSubscription` calls `format.DecodeXxx(msg.RawBytes)` →
   produces `DecodedXxx` struct containing formatted strings (previously
   via `fmt.Sprintf`, now via `strconv.AppendUint` + `string(...)`).
3. Dispatch on encoding:
   - Text mode → `format.AppendXxx(stackScratch[:0], peer, decoded, ...)` →
     buffer-first, writes decoded strings by `append(buf, s...)`.
   - JSON mode → `encoder.Xxx(peer, decoded, ...)` → builds
     `map[string]any`, marshals with `json.Marshal`.
4. Result string flows to plugin subscribers via `Deliver`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire bytes → `DecodedXxx` struct | `DecodeXxx(body)` in `decode.go` | [ ] |
| `DecodedXxx` → Text | `AppendXxx(buf, peer, decoded, ...)` in `text.go` | [ ] |
| `DecodedXxx` → JSON | `JSONEncoder.Xxx` methods in `json.go` | [ ] |
| Hook → Write/Edit | `.claude/hooks/block-format-alloc.sh` rejects banned patterns in allowlisted files | [ ] |

### Integration Points
- `text.go` (`AppendOpen`, `AppendNotification`, `AppendRouteRefresh`) —
  zero-copy consumer of `DecodedXxx.*` string fields. Must see
  byte-identical strings after migration.
- `json.go` (`JSONEncoder.Open`, `.Notification`, `.RouteRefresh`) — reads
  the same string fields into `map[string]any`. Must see byte-identical
  strings after migration.
- `events.go formatMessageForSubscription` — the sole call-site glue,
  untouched by this spec.
- `message_receiver_test.go` — existing byte-level assertions act as the
  regression guard; new tests cover fallback paths that currently lack
  direct coverage.

### Architectural Verification
- [ ] No bypassed layers — data continues through `events.go` →
  `DecodeXxx` → `AppendXxx`/`JSONEncoder.Xxx`.
- [ ] No unintended coupling — `decode.go` gains no new imports beyond
  `strconv` and `encoding/hex`; drops `fmt`. `json.go` drops `fmt`,
  adds `encoding/hex`.
- [ ] No duplicated functionality — migration, not re-implementation.
- [ ] Zero-copy preserved — `DecodedXxx` struct still holds strings; no
  change in memory ownership.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP peer subscription, text mode, OPEN received | → | `DecodeOpen` → `AppendOpen` with migrated `formatCapability` | `TestDecodeOpenWithCapabilities` in `internal/component/bgp/format/message_receiver_test.go` |
| BGP peer subscription, JSON mode, NOTIFICATION received | → | `DecodeNotification` → `JSONEncoder.Notification` with migrated `notificationSubcodeString` + migrated hex on `notify.Data` | `TestDecodeNotification` + new `TestDecodeNotification_UnknownSubcode` in `message_receiver_test.go` |
| Edit attempt introducing `fmt.Sprintf` in `decode.go` | → | `.claude/hooks/block-format-alloc.sh` | `test/hooks/format-alloc-hook.sh` (new — smoke test of the hook via piped JSON stdin) |

The first two are the existing production wiring via
`formatMessageForSubscription`; the migration preserves their output
bytes, so the existing test coverage is the wiring test. The third row
is the wiring test for the new hook itself: a shell smoke-test that
pipes crafted Write-tool JSON through the hook and asserts exit 2 on
banned patterns, exit 0 on clean content.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `internal/component/bgp/format/format_buffer.go` and `format_buffer_test.go` are deleted from the tree | Both paths absent; `git log --diff-filter=D` shows the deletion commit |
| AC-2 | `decode.go` is grepped for `fmt\.(Sprintf\|Fprintf)`, `strings\.(Join\|Builder\|NewReplacer\|ReplaceAll)`, `strconv\.(FormatUint\|FormatInt)` | zero matches |
| AC-3 | `json.go` is grepped for `fmt\.(Sprintf\|Fprintf)` | zero matches |
| AC-4 | `TestDecodeOpenWithCapabilities` runs against the migrated `formatCapability` | passes unmodified; `DecodedCapability.Value` strings byte-identical (`"ipv4/unicast"`, `"0200"`, `"65536"`) |
| AC-5 | `TestDecodeNotification_UnknownSubcode` (new) feeds an OPEN-category NOTIFICATION with subcode 99 | `ErrorSubcodeName == "Subcode(99)"` |
| AC-6 | `TestDecodeRouteRefresh_UnknownSubtype` (new) feeds ROUTE-REFRESH with subtype 99 | `SubtypeName == "unknown(99)"` |
| AC-7 | `TestAfiSafiToFamily_Unknown` (new) calls `afiSafiToFamily(99, 99)` via `NegotiatedToDecoded` with a family whose AFI/SAFI are unknown | Family string `"afi(99)/safi(99)"` |
| AC-8 | Hook `block-format-alloc.sh` receives a Write PreToolUse event for `internal/component/bgp/format/decode.go` with new content containing `fmt.Sprintf("%d", x)` | Exit 2, stderr explains banned pattern + filename |
| AC-9 | Hook receives the same Write event but file path is `internal/component/bgp/format/json.go` and content contains `json.Marshal(...)` | Exit 0 (json.go is intentionally out of scope; hook does not block `json.Marshal`) |
| AC-10 | `make ze-verify-fast` runs with the full migration and hook registered | passes; `tmp/ze-verify.log` shows no new failures |
| AC-11 | `plan/deferrals.md` entry for `spec-fmt-2-json-append` is updated | Status column shows `done`, Destination column names `spec-fmt-2-json-append` (or its learned summary) |
| AC-12 | `.claude/settings.json` lists `block-format-alloc.sh` in the PreToolUse section for Write + Edit | grep confirms the hook registration entry |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeOpenWithCapabilities` (existing, preserved) | `internal/component/bgp/format/message_receiver_test.go` | AC-4 — `Multiprotocol` / `ASN4` / unknown-cap `Value` strings byte-identical | |
| `TestDecodeOpenAddPathReceive` (new) | `internal/component/bgp/format/message_receiver_test.go` | Addpath receive + send-receive emit `"ipv4/unicast receive"` / `"ipv4/unicast send-receive"`; `none` mode skipped | |
| `TestDecodeNotification` (existing, preserved) | `internal/component/bgp/format/message_receiver_test.go` | Known-subcode path unchanged | |
| `TestDecodeNotification_UnknownSubcode` (new) | `internal/component/bgp/format/message_receiver_test.go` | AC-5 — `"Subcode(99)"` fallback byte-identical across all 5 subcode functions (Open, Update, Header, FSM, default) | |
| `TestDecodeRouteRefresh_UnknownSubtype` (new) | `internal/component/bgp/format/message_receiver_test.go` | AC-6 — `"unknown(99)"` fallback byte-identical | |
| `TestNegotiatedToDecoded` (existing, preserved) | `internal/component/bgp/format/message_receiver_test.go` | Known AFI/SAFI family strings unchanged | |
| `TestAfiSafiToFamily_Unknown` (new) | `internal/component/bgp/format/message_receiver_test.go` | AC-7 — `"afi(99)/safi(99)"` and `"afi(99)/unicast"` fallbacks byte-identical | |
| `TestJSONEncoderNotification_HexData` (new) | `internal/component/bgp/format/text_test.go` or adjacent | AC-3 support — `JSONEncoder.Notification` with non-empty `notify.Data` emits lowercase hex in `"data"` field | |
| `TestFormatAllocHook_BlocksBannedInDecode` (new, shell) | `test/hooks/format-alloc-hook.sh` | AC-8 — piping crafted Write JSON with `fmt.Sprintf` for `decode.go` to the hook yields exit 2 | |
| `TestFormatAllocHook_AllowsInJsonGo` (new, shell) | `test/hooks/format-alloc-hook.sh` | AC-9 — same input but for `json.go` with `json.Marshal` yields exit 0 | |

### Boundary Tests
Not applicable — no numeric input ranges are introduced by this spec.

### Functional Tests
No new user-facing feature; existing `.ci` tests that exercise OPEN /
NOTIFICATION subscription in text and JSON modes act as the functional
regression guard.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing OPEN-subscribe coverage | `test/plugin/*.ci` exercising peer OPEN via subscription | Operator subscribes to peer events, receives OPEN with capabilities rendered byte-identical pre/post migration | |
| existing NOTIFICATION-subscribe coverage | `test/plugin/*.ci` exercising NOTIFICATION | Operator sees NOTIFICATION `"data"` field with lowercase hex identical pre/post migration | |

### Future
None planned.

## Files to Modify

- `internal/component/bgp/format/decode.go` — replace 14 `fmt.Sprintf` call sites with local scratch (`[32]byte` or `[64]byte`) + `strconv.AppendUint` / `append(buf, x.String()...)` / `hex.AppendEncode` + single `string(scratch[:n])` at each assignment. Drop `fmt` import. Add `strconv`, `encoding/hex` imports as needed.
- `internal/component/bgp/format/json.go` — replace line 166 `fmt.Sprintf("%x", notify.Data)` with `string(hex.AppendEncode(nil, notify.Data))`. Drop `fmt` import if no other site uses it (there is no other `fmt` usage in this file — safe drop). Add `encoding/hex` import.
- `internal/component/bgp/format/message_receiver_test.go` — add 5 new tests (addpath-receive, unknown-subcode-across-families, unknown-route-refresh-subtype, unknown-afi-safi, JSON-encoder hex data).
- `.claude/settings.json` — register `block-format-alloc.sh` under the PreToolUse Write + Edit sections (adjacent to `block-encoding-alloc.sh`).
- `plan/deferrals.md` — mark the fmt-2-json-append deferral entry `done`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | n/a |
| CLI commands/flags | [ ] No | n/a |
| Editor autocomplete | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No — existing `.ci` OPEN/NOTIFICATION coverage preserved | n/a |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | n/a |
| 2 | Config syntax changed? | [ ] No | n/a |
| 3 | CLI command added/changed? | [ ] No | n/a |
| 4 | API/RPC added/changed? | [ ] No | n/a |
| 5 | Plugin added/changed? | [ ] No | n/a |
| 6 | Has a user guide page? | [ ] No | n/a |
| 7 | Wire format changed? | [ ] No | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] No | n/a |
| 9 | RFC behavior implemented? | [ ] No | n/a |
| 10 | Test infrastructure changed? | [ ] Yes — hook reference list | `docs/functional-tests.md` (if the doc enumerates hooks) or `.claude/rules/buffer-first.md` (mention the new hook near the existing `block-encoding-alloc.sh` note) |
| 11 | Affects daemon comparison? | [ ] No | n/a |
| 12 | Internal architecture changed? | [ ] No | n/a |
| 13 | Route metadata? | [ ] No | n/a |

## Files to Create

- `.claude/hooks/block-format-alloc.sh` — new PreToolUse hook (~120 LOC); file-path allowlist + banned-pattern grep + exit-2 on match; skip `_test.go`.
- `test/hooks/format-alloc-hook.sh` — smoke test invoking the hook with crafted stdin JSON to verify AC-8 and AC-9. If `test/hooks/` does not exist, place alongside other hook tests or under `test/ci/hooks/`.
- `plan/learned/NNN-fmt-2-json-append.md` — learned summary (written at completion, not up front).

## Files to Delete

- `internal/component/bgp/format/format_buffer.go` — dead code (225L).
- `internal/component/bgp/format/format_buffer_test.go` — tests of dead code (225L); permitted by `rules/no-test-deletion.md` exception ("testing removed functionality") and explicitly approved by the user at the SCOPE gate.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, Files to Delete, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | loop |
| 8. Re-verify | re-run stage 5 |
| 9. Repeat 6-8 | max 2 passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | re-run stage 5 |
| 13. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: dead code removal** — delete `format_buffer.go` and `format_buffer_test.go`.
   - Tests: N/A — the only tests being removed are those that exercise dead code.
   - Files: the two files above.
   - Verify: `go build ./...` passes (no compile errors means no hidden caller existed); `make ze-verify-fast` runs; no test count regression beyond the removed `format_buffer_test.go` cases.

2. **Phase: add fallback-path tests to `message_receiver_test.go`** — write `TestDecodeOpenAddPathReceive`, `TestDecodeNotification_UnknownSubcode`, `TestDecodeRouteRefresh_UnknownSubtype`, `TestAfiSafiToFamily_Unknown`, `TestJSONEncoderNotification_HexData` against the unmigrated `decode.go` / `json.go`. All must PASS against current code (they assert existing behavior).
   - Tests: all 5 listed above.
   - Files: `message_receiver_test.go` plus one adjacent test file for the JSON encoder case.
   - Verify: `go test ./internal/component/bgp/format/...` passes. Tests now form the byte-level regression guard before migration.

3. **Phase: migrate `decode.go`** — replace each `fmt.Sprintf` site in order.
   - Tests from phase 2 + existing `TestDecodeOpenWithCapabilities`, `TestDecodeNotification`, `TestNegotiatedToDecoded`.
   - Files: `decode.go`.
   - Verify: all phase-2 and existing tests still pass with zero diff; `grep 'fmt\.Sprintf' decode.go` returns zero.

4. **Phase: migrate `json.go:166`** — swap `fmt.Sprintf("%x", …)` for `hex.AppendEncode`.
   - Tests: `TestJSONEncoderNotification_HexData` from phase 2.
   - Files: `json.go`.
   - Verify: test passes; `grep 'fmt\.Sprintf' json.go` returns zero; `grep 'encoding/hex' json.go` present.

5. **Phase: install `block-format-alloc.sh`** — write the hook, register it in `.claude/settings.json`, write the smoke test.
   - Tests: `test/hooks/format-alloc-hook.sh` covering AC-8 (blocks) + AC-9 (allows json.go).
   - Files: `.claude/hooks/block-format-alloc.sh`, `.claude/settings.json`, `test/hooks/format-alloc-hook.sh`.
   - Verify: smoke test passes; real Write of a test file under `decode.go` fails with exit 2; Write of the new migrated `decode.go` passes.

6. **Phase: close deferrals + docs** — mark the `plan/deferrals.md` fmt-2-json-append entry `done`; update `.claude/rules/buffer-first.md` or `docs/functional-tests.md` with a line naming the new hook.
   - Files: `plan/deferrals.md`, one doc file.
   - Verify: `grep -n fmt-2-json-append plan/deferrals.md` shows `done`.

7. **Full verification** — `make ze-verify-fast` from a cold state.

8. **Complete spec** — fill Implementation Summary, Implementation Audit, Pre-Commit Verification, Review Gate; write `plan/learned/NNN-fmt-2-json-append.md`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has implementation + test evidence; no deferrals. |
| Correctness | `DecodedCapability.Value`, `ErrorSubcodeName`, `SubtypeName`, `Family` string formats byte-identical (run `TestDecodeOpenWithCapabilities`, `TestDecodeNotification`, `TestNegotiatedToDecoded` unmodified and confirm pass). |
| Naming | Hook named `block-format-alloc.sh` (matches `block-encoding-alloc.sh` convention). Skip calling the hook `format-alloc-guard.sh` or other invented names. |
| Data flow | `decode.go` imports shrink (`fmt` out, `strconv` + `encoding/hex` in). No new dependency between `format/` and other packages. |
| Rule: no-layering | `format_buffer.go` fully deleted; no transitional "legacy" file. |
| Rule: no-test-deletion | Test deletion is `format_buffer_test.go` only; approval from SCOPE gate recorded here. |
| Rule: self-documenting | Hook script top comment names the reference rule (`rules/buffer-first.md`) and learned summary (`plan/learned/614-fmt-0-append.md`). |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `format_buffer.go` deleted | `test ! -e internal/component/bgp/format/format_buffer.go` |
| `format_buffer_test.go` deleted | `test ! -e internal/component/bgp/format/format_buffer_test.go` |
| `decode.go` fmt-clean | `! grep -E 'fmt\.(Sprintf\|Fprintf)\|strings\.(Join\|Builder\|NewReplacer\|ReplaceAll)\|strconv\.FormatUint\|strconv\.FormatInt' internal/component/bgp/format/decode.go` |
| `json.go` fmt-clean | `! grep -E 'fmt\.(Sprintf\|Fprintf)' internal/component/bgp/format/json.go` |
| Hook present + executable | `test -x .claude/hooks/block-format-alloc.sh` |
| Hook registered | `grep -q block-format-alloc .claude/settings.json` |
| Smoke test present | `test -x test/hooks/format-alloc-hook.sh` |
| Deferral closed | `grep -E 'fmt-2-json-append.*done' plan/deferrals.md` |
| `make ze-verify-fast` green | inspect `tmp/ze-verify.log` tail |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `hex.AppendEncode` handles zero-length input identically to `fmt.Sprintf("%x", nil)`; verify `TestJSONEncoderNotification_HexData` covers both empty and non-empty `notify.Data`. |
| Format-string injection | No dynamic format strings are introduced; `strconv.AppendUint` takes only a value + base. No risk. |
| Resource exhaustion | Scratch buffers are fixed-size stack arrays (`[32]byte` / `[64]byte`); no unbounded allocations. |
| Hook bypass | Hook matches on `tool_input.file_path`; ensure it handles absolute and relative paths equivalently (model after `block-encoding-alloc.sh` which already handles this). |
| Error leakage | No user-facing error paths changed. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error after migration | Fix in the phase that introduced it — likely a missing import |
| `TestDecodeOpenWithCapabilities` fails after decode.go migration | Byte drift in `formatCapability`; re-inspect the specific case that diverged |
| `TestJSONEncoderNotification_HexData` fails on empty data | `hex.AppendEncode(nil, nil)` returns `nil`; `string(nil) == ""` — verify the expected output is `""` not `"nil"` |
| Hook blocks legitimate edit | Review allowlist; the hook should only gate files it is meant to protect |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The 5 `Format*JSON(io.Writer)` helpers are called from `text_json.go` (per the original skeleton spec) | Zero production callers anywhere in the repo | Repo-wide grep at SCOPE gate | Spec premise was invalid; re-scoped to S2+S4 |
| fmt-0 installed a real hook guard for `fmt.Sprintf` in three files | The guard is hand-maintained; no hook exists | Grepping `.claude/hooks/` for the file names and the banned patterns | Added S4 (write the actual hook) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| "Migrate the 5 JSON writers in `format_buffer.go` to Append shape" (original spec) | No callers — migration target is dead code | Delete the file; migrate the actually-used cold-path sites in `decode.go` and `json.go` instead |
| "Full JSONEncoder buffer-first rewrite (S3)" | Large scope (~400 LOC), cold path, no measurable perf gain | Kept out of scope; recorded as a future-work candidate if perf need emerges |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Spec skeleton written from "what would be nice" rather than verified callers | 1 (this spec) | `rules/planning.md` already mandates current-behavior verification; consider a hook that requires spec Task references to exist in the codebase | Acknowledge; no new rule needed; fmt-0's practice of writing post-hoc learned summaries with bad claims is already addressed by `rules/quality.md` Learned Summary Verification |

## Design Insights

- `fmt.Sprintf` migration for `string`-typed struct fields does not reduce
  allocation count — the string allocation is unavoidable for a `string`
  field. The win is purely CPU (no reflection) and banned-pattern
  discipline. Future specs that promise "zero-alloc" should check whether
  the data flows through a `string` field before making the claim.
- `block-encoding-alloc.sh` is the better model than `block-panic-error.sh`
  for a format-alloc guard because the former already demonstrates file-path
  allowlisting; `block-panic-error.sh` uses a content-pattern allowlist
  instead.
- Per `plan/learned/614`, fmt-0's "hand-maintained" guard across three files
  survived because the surface area was small. Extending it to eight files
  without a mechanical hook would almost certainly regress.

## RFC Documentation

None — no protocol semantics changed.

## Implementation Summary

_(to be filled by /ze-implement)_

### What Was Implemented
- [list]

### Bugs Found/Fixed
- [list or "None"]

### Documentation Updates
- [list or "None"]

### Deviations from Plan
- [list or "None"]

## Implementation Audit

_(to be filled by /ze-implement)_

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

_(to be filled by /ze-implement)_

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [list]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

_(to be filled by /ze-implement)_

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-verify-fast` passes
- [ ] Migration code integrated (`decode.go`, `json.go`, new hook registered)
- [ ] Integration completeness proven end-to-end via existing `.ci` coverage
- [ ] Architecture docs updated (hook mentioned)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments — not applicable
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction — scope narrowed to demonstrated cold-path sites
- [ ] No speculative features — S3 (JSONEncoder buffer-first) explicitly out of scope
- [ ] Single responsibility — each phase covers one target
- [ ] Explicit > implicit — every banned pattern listed in the hook
- [ ] Minimal coupling — no new cross-package imports

### TDD
- [ ] Tests written BEFORE migration (phase 2)
- [ ] Tests FAIL on a bad migration (verified by the reference-case assertions)
- [ ] Tests PASS after migration
- [ ] Boundary tests — N/A
- [ ] Functional tests — existing `.ci` coverage

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary at `plan/learned/NNN-fmt-2-json-append.md`
- [ ] Summary included in commit

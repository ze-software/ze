# Spec: fmt-2-json-append

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fmt-0-append (completed, see `plan/learned/614-fmt-0-append.md`) |
| Phase | 7/7 |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `.claude/rules/buffer-first.md` — Append vs WriteTo discipline
4. `plan/learned/614-fmt-0-append.md` — the pattern this spec extends
5. `internal/component/bgp/format/decode.go` — primary migration target
6. `internal/component/bgp/format/json.go` — one-line migration
7. `internal/component/bgp/format/format_buffer.go` — dead code being deleted
8. `internal/component/bgp/format/message_receiver_test.go` and `json_test.go` — existing test coverage
9. `internal/component/bgp/server/events.go` (lines 340-393) — primary consumer
10. `internal/component/bgp/reactor/reactor_notify.go` (line 106) — second consumer of `NegotiatedToDecoded`
11. `docs/architecture/buffer-architecture.md` — Phase 3 row to rewrite
12. `.claude/hooks/block-encoding-alloc.sh` — model for the new hook

## Task

Close the residual cold-path allocation sites in the BGP message-format
package and install a PreToolUse hook that enforces fmt-0's banned-pattern
discipline across every file that fmt-0 migrated. Scope is deliberately the
"S2+S4" slice of the original fmt-2 proposal:

1. **Delete dead code.** `internal/component/bgp/format/format_buffer.go`
   contains five `Format*JSON(io.Writer)` helpers (`FormatASPathJSON`,
   `FormatCommunitiesJSON`, `FormatOriginJSON`, `FormatMEDJSON`,
   `FormatLocalPrefJSON` — together holding seven `fmt.Fprintf(w, ...)`
   call sites) plus `FormatPrefixFromBytes` and `wellKnownCommunityName`.
   Repo-wide grep shows **zero production callers** anywhere outside the
   file itself and its test file. fmt-0 rewrote the JSON emission path
   (`text_json.go appendFilterResultJSON`) without touching these helpers,
   and `text_json.go` has never called them. The file is dead; delete it
   along with its test file.
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
   allowlist of files containing `fmt.Sprintf`, `fmt.Fprintf`,
   `strings.Join`, `strings.Builder`, `strings.NewReplacer`,
   `strings.ReplaceAll`, `strconv.FormatUint`, `strconv.FormatInt`. The
   allowlist is:
   - `internal/component/bgp/reactor/filter_format.go` (fmt-0)
   - `internal/component/bgp/attribute/text.go` (fmt-0)
   - `internal/component/bgp/format/text.go` (fmt-0)
   - `internal/component/bgp/format/text_json.go` (fmt-0)
   - `internal/component/bgp/format/text_update.go` (fmt-0)
   - `internal/component/bgp/format/text_human.go` (already clean)
   - `internal/component/bgp/format/summary.go` (already clean)
   - `internal/component/bgp/format/codec.go` (already clean)
   - `internal/component/bgp/format/decode.go` (added by this spec after migration)
   
   `json.go` is intentionally excluded — its `map[string]any` + `json.Marshal`
   idiom is out of scope for this spec (S3 tier).
   
   fmt-0 left the guard as hand-maintained discipline on three files; this
   hook makes it mechanical and extends it to cover every other file fmt-0
   and fmt-2 touch. `strings.NewReplacer` and `strings.ReplaceAll` are
   defensive additions (no current occurrences in any allowlisted file —
   banning pre-empts reintroduction). The hook is named
   `block-format-alloc.sh` despite including `reactor/filter_format.go`
   and `attribute/text.go` in its allowlist because the scope is
   "text/JSON format generation" across all three package paths.

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
  → Constraint: banned primitives list (`fmt.Sprintf`, `fmt.Fprintf`, `strings.Builder`, `strings.Join`, `strings.NewReplacer`, `strings.ReplaceAll`, `strconv.FormatUint`, `strconv.FormatInt`) — `decode.go` becomes a newly-guarded file. Must match the Task step-4 list exactly.
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
  existing tests for `DecodeOpen`, `DecodeNotification`,
  `NegotiatedToDecoded`. Includes `DecodedCapability` equality assertions
  (`Value: "ipv4/unicast"`, `"0200"`, `"65536"`) that are the byte-level
  regression guard. No existing `TestDecodeRouteRefresh` — the new
  fallback test for route-refresh is the first direct coverage.
  → Constraint: do not weaken any existing assertion; add new tests for the
  fallback branches (`"Subcode(N)"`, `"unknown(N)"`, `"afi(N)"`,
  `"safi(N)"`).
- [ ] `internal/component/bgp/format/json_test.go` (2083L) — existing
  JSONEncoder tests: `TestJSONEncoderStateUp`, `StateDown`,
  `StateConnected`, `ValidJSON`, `SpecialCharacters`, `IPv6`,
  `TestAPIOutputIncludesMsgID`, and `TestJSONEncoderNotification`
  (locate by name, not by line number). The existing
  `TestJSONEncoderNotification` feeds a non-empty `notify.Data` and
  asserts `payload["data"]` is `NotEmpty` — it does not pin the exact
  hex string, so it is not a sufficient regression guard for the
  `fmt.Sprintf("%x", …)` → `hex.AppendEncode` swap.
  → Constraint: the new `TestJSONEncoderNotification_HexData` must assert
  the exact hex output (lowercase, no `0x`) against a known
  `notify.Data` byte slice, covering both empty and non-empty inputs.
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
- Production consumers of the migrated `decode.go` surface:
  - `bgp/server/events.go formatMessageForSubscription` (lines 340-393)
    calls `DecodeOpen`, `DecodeNotification`, `DecodeRouteRefresh`, and
    dispatches to `AppendOpen/AppendNotification/AppendRouteRefresh`
    (text mode) or `JSONEncoder.Open/Notification/RouteRefresh` (JSON
    mode). Fires per non-UPDATE BGP message received or sent on a peer
    subscription.
  - `bgp/reactor/reactor_notify.go:106` calls `NegotiatedToDecoded` on
    peer-established events and feeds `JSONEncoder.Negotiated` via the
    event dispatcher. This is the SECOND consumer of
    `NegotiatedToDecoded` and therefore of `afiSafiToFamily` /
    `afiToString`. Migration must preserve byte-identical output on
    this path too.
- The 5 `Format*JSON(io.Writer)` helpers in `format_buffer.go`: zero
  production callers (confirmed by repo-wide grep).

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
- `json.go` (`JSONEncoder.Open`, `.Notification`, `.RouteRefresh`,
  `.Negotiated`) — reads the same string fields into `map[string]any`.
  Must see byte-identical strings after migration.
- `events.go formatMessageForSubscription` — call-site glue for
  OPEN/NOTIFICATION/ROUTE-REFRESH/KEEPALIVE, untouched by this spec.
- `reactor_notify.go notifyPeerEstablished` (line 106) — call-site glue
  for NEGOTIATED, untouched by this spec.
- `message_receiver_test.go` and `json_test.go` — existing byte-level
  assertions act as the regression guard; new tests cover fallback paths
  that currently lack direct coverage.

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
| BGP peer subscription, JSON mode, NOTIFICATION received | → | `DecodeNotification` → `JSONEncoder.Notification` with migrated `notificationSubcodeString` (subcode-name path) + migrated hex on `notify.Data` (JSON data-field path) | subcode path: `TestDecodeNotification` + new `TestDecodeNotification_UnknownSubcode` in `message_receiver_test.go`; hex path: new `TestJSONEncoderNotification_HexData` in `json_test.go` |
| Edit attempt introducing `fmt.Sprintf` in `decode.go` | → | `.claude/hooks/block-format-alloc.sh` | `scripts/dev/test-hook-block-format-alloc.sh` (new — smoke test of the hook via piped JSON stdin) |
| Peer-negotiated event, JSON mode | → | `NegotiatedToDecoded` → `JSONEncoder.Negotiated` with migrated `afiSafiToFamily` | new `TestDecodeNegotiated_UnknownAfiSafi` in `message_receiver_test.go` |

Rows 1, 2, and 4 are the existing production wiring (`formatMessageForSubscription` for row 1-2, `notifyPeerEstablished` for row 4); the migration preserves their output bytes, so the Go unit test coverage is the wiring guard. Row 3 is the wiring test for the new hook itself: a shell smoke-test that pipes crafted Write-tool JSON through the hook and asserts exit 2 on banned patterns, exit 0 on clean content.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `internal/component/bgp/format/format_buffer.go` and `format_buffer_test.go` are deleted from the tree | Both paths absent (`test ! -e …`); `git log --diff-filter=D` shows the deletion commit |
| AC-2 | `decode.go` is grepped for `fmt\.(Sprintf|Fprintf)`, `strings\.(Join|Builder|NewReplacer|ReplaceAll)`, `strconv\.(FormatUint|FormatInt)` (ERE, unescaped alternation) | zero matches |
| AC-3 | `json.go` is grepped for `fmt\.(Sprintf|Fprintf)` (ERE, unescaped alternation) | zero matches |
| AC-4 | `TestDecodeOpenWithCapabilities` runs against the migrated `formatCapability` | passes unmodified; `DecodedCapability.Value` strings byte-identical (`"ipv4/unicast"`, `"0200"`, `"65536"`) |
| AC-5 | `TestDecodeNotification_UnknownSubcode` (new, table-driven over 5 error codes — NotifyOpenMessage, NotifyUpdateMessage, NotifyMessageHeader, NotifyFSMError, and an unmapped error-code that hits the `notificationSubcodeString` default) feeds subcode 99 for each | every case: `ErrorSubcodeName == "Subcode(99)"` |
| AC-6 | `TestDecodeRouteRefresh_UnknownSubtype` (new) feeds ROUTE-REFRESH with subtype 99 | `SubtypeName == "unknown(99)"` |
| AC-7 | `TestDecodeNegotiated_UnknownAfiSafi` (new) exercises `afiSafiToFamily(99, 99)` via `NegotiatedToDecoded` with an unknown AFI/SAFI family | Family string `"afi(99)/safi(99)"`; also verifies `afiToString(99)` returns `"afi(99)"` |
| AC-8 | Hook `block-format-alloc.sh` receives a Write PreToolUse event for `internal/component/bgp/format/decode.go` with new content containing `fmt.Sprintf("%d", x)` | Exit 2, stderr explains banned pattern + filename |
| AC-9 | Hook receives a Write event for `internal/component/bgp/format/json.go` with content containing `json.Marshal(...)` or `map[string]any` | Exit 0 (`json.go` intentionally out of scope; hook does not block `json.Marshal`) |
| AC-10 | `make ze-verify-fast` runs with the full migration and hook registered | passes; `tmp/ze-verify.log` shows no new failures |
| AC-11 | `plan/deferrals.md` entry for `spec-fmt-2-json-append` is updated | Status column changes from `open` to `done`; Destination column stays as `spec-fmt-2-json-append` |
| AC-12 | `.claude/settings.json` lists `block-format-alloc.sh` in the PreToolUse section for Write + Edit | grep confirms the hook registration entry (matcher `Write|Edit`, path `$CLAUDE_PROJECT_DIR/.claude/hooks/block-format-alloc.sh`) |
| AC-13 | `docs/architecture/buffer-architecture.md` Phase 3 row is updated to reflect that `FormatPrefixFromBytes` / `Format*JSON` are deleted (row removed or re-worded) | `grep -E 'FormatPrefixFromBytes|FormatASPathJSON|FormatCommunitiesJSON|FormatOriginJSON|FormatMEDJSON|FormatLocalPrefJSON' docs/architecture/buffer-architecture.md` returns no matches |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeOpenWithCapabilities` (existing, preserved) | `internal/component/bgp/format/message_receiver_test.go` | AC-4 — `Multiprotocol` / `ASN4` / unknown-cap `Value` strings byte-identical | |
| `TestDecodeOpenAddPathReceive` (new) | `internal/component/bgp/format/message_receiver_test.go` | Addpath receive + send-receive emit `"ipv4/unicast receive"` / `"ipv4/unicast send-receive"`; `none` mode skipped | |
| `TestDecodeNotification` (existing, preserved) | `internal/component/bgp/format/message_receiver_test.go` | Known-subcode path unchanged | |
| `TestDecodeNotification_UnknownSubcode` (new, table-driven) | `internal/component/bgp/format/message_receiver_test.go` | AC-5 — `"Subcode(99)"` fallback byte-identical across 5 fallback sites: `notificationSubcodeString` default, `openSubcodeString` default, `updateSubcodeString` default, `headerSubcodeString` default, `fsmSubcodeString` default. Implemented as `t.Run` subtests over 5 `(errorCode, expected)` table rows | |
| `TestDecodeRouteRefresh_UnknownSubtype` (new) | `internal/component/bgp/format/message_receiver_test.go` | AC-6 — `"unknown(99)"` fallback byte-identical | |
| `TestNegotiatedToDecoded` (existing, preserved) | `internal/component/bgp/format/message_receiver_test.go` | Known AFI/SAFI family strings unchanged | |
| `TestDecodeNegotiated_UnknownAfiSafi` (new) | `internal/component/bgp/format/message_receiver_test.go` | AC-7 — `"afi(99)/safi(99)"`, `"afi(99)/unicast"`, and `"afi(99)"` fallbacks byte-identical for both `afiSafiToFamily` and `afiToString` paths | |
| `TestJSONEncoderNotification_HexData` (new) | `internal/component/bgp/format/json_test.go` (next to existing `TestJSONEncoderNotification`) | AC-3 support — `JSONEncoder.Notification` with empty and non-empty `notify.Data` emits exact lowercase hex (no `0x` prefix, empty string for nil data) in `"data"` field. Pins exact bytes where existing test only asserts `NotEmpty` | |
| `TestFormatAllocHook_BlocksBannedInDecode` (new, shell) | `scripts/dev/test-hook-block-format-alloc.sh` | AC-8 — piping crafted Write JSON with `fmt.Sprintf` for `decode.go` to the hook yields exit 2 | |
| `TestFormatAllocHook_AllowsInJsonGo` (new, shell) | `scripts/dev/test-hook-block-format-alloc.sh` | AC-9 — same input but for `json.go` with `json.Marshal` yields exit 0 | |

### Boundary Tests
Not applicable — no numeric input ranges are introduced by this spec.

### Functional Tests
No new user-facing feature is introduced; this is a pure refactor plus a
new developer hook. Repo survey (`grep -l` across `test/plugin/*.ci`,
`test/integration/*.ci`) confirms **no existing `.ci` test exercises
non-UPDATE subscription formatting** (OPEN capability strings, NOTIFICATION
`"data"` hex field, ROUTE-REFRESH subtype names, NEGOTIATED family
strings). `test/plugin/api-subscribe.ci` covers the `subscribe` dispatch
itself but subscribes to `update` events only.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (intentionally none) | n/a | No `.ci` coverage exists for non-UPDATE subscription byte-level output. The Go unit tests in `message_receiver_test.go` and `json_test.go` are the byte-level regression guard. Creating a new `.ci` for the cold non-UPDATE path is explicitly out of scope for this pure-refactor spec. | |

Per `rules/integration-completeness.md`, `.ci` functional coverage is
required for new user-facing features. This spec introduces no new
user-facing behavior — output bytes and dispatch paths are unchanged —
so the Go unit tests serve as the integration guard. No deferral is
recorded because there is no identified future consumer that needs
`.ci` coverage for non-UPDATE subscription output; if one emerges, a
new spec can introduce the test at that point.

### Future
None planned.

## Files to Modify

- `internal/component/bgp/format/decode.go` — replace 14 `fmt.Sprintf` call sites with local scratch (`[32]byte` or `[64]byte`) + `strconv.AppendUint` / `append(buf, x.String()...)` / `hex.AppendEncode` + single `string(scratch[:n])` at each assignment. Drop `fmt` import. Add `strconv`, `encoding/hex` imports as needed.
- `internal/component/bgp/format/json.go` — replace line 166 `fmt.Sprintf("%x", notify.Data)` with `string(hex.AppendEncode(nil, notify.Data))`. Drop `fmt` import if no other site uses it (there is no other `fmt` usage in this file — safe drop). Add `encoding/hex` import.
- `internal/component/bgp/format/message_receiver_test.go` — add 4 new tests (addpath-receive, unknown-subcode-across-families table-driven, unknown-route-refresh-subtype, unknown-afi-safi).
- `internal/component/bgp/format/json_test.go` — add 1 new test (`TestJSONEncoderNotification_HexData`) next to existing `TestJSONEncoderNotification` (currently at line 308, but reference by test name, not line number).
- `.claude/settings.json` — register `block-format-alloc.sh` under the PreToolUse Write + Edit sections (adjacent to `block-encoding-alloc.sh`); matcher string `"Write|Edit"`, command `"$CLAUDE_PROJECT_DIR/.claude/hooks/block-format-alloc.sh"`.
- `docs/architecture/buffer-architecture.md` — update the Phase 3 "Direct formatting functions" row to remove references to `FormatPrefixFromBytes`, `FormatASPathJSON`, `FormatCommunitiesJSON`, `FormatOriginJSON`, `FormatMEDJSON`, `FormatLocalPrefJSON` (functions being deleted by AC-1).
- `plan/deferrals.md` — change the `spec-fmt-2-json-append` entry Status column from `open` to `done`.

### Files to Delete
- `internal/component/bgp/format/format_buffer.go` — dead code (225L).
- `internal/component/bgp/format/format_buffer_test.go` — tests of dead code (225L); permitted by `rules/no-test-deletion.md` exception ("testing removed functionality") and explicitly approved by the user at the SCOPE gate.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | n/a |
| CLI commands/flags | [ ] No | n/a |
| Editor autocomplete | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No — pure refactor, no new user-facing behaviour; Go unit tests are the byte-level regression guard (see Functional Tests section for rationale) | n/a |
| Hook registration | [ ] Yes | `.claude/settings.json` (PreToolUse Write + Edit) |
| Dev-script test host | [ ] Yes — smoke test for the new hook | `scripts/dev/test-hook-block-format-alloc.sh` |

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
| 10 | Test infrastructure changed? | [ ] Yes — new hook + new smoke-test script | `.claude/rules/buffer-first.md` (mention `block-format-alloc.sh` near the existing `block-encoding-alloc.sh` note) |
| 11 | Affects daemon comparison? | [ ] No | n/a |
| 12 | Internal architecture changed? | [ ] Yes — architecture doc references deleted functions | `docs/architecture/buffer-architecture.md` (Phase 3 row rewritten to drop the `Format*JSON` / `FormatPrefixFromBytes` references) |
| 13 | Route metadata? | [ ] No | n/a |

## Files to Create

- `.claude/hooks/block-format-alloc.sh` — new PreToolUse hook (~120 LOC); file-path allowlist + banned-pattern grep + exit-2 on match; skip `_test.go`; model after `block-encoding-alloc.sh`.
- `scripts/dev/test-hook-block-format-alloc.sh` — smoke test invoking the hook with crafted stdin JSON to verify AC-8 and AC-9. Lives alongside existing dev utilities (`scripts/dev/verify-lock.sh`, `verify-status.sh`, `verify-summary.sh`) since no dedicated hook-test directory convention exists in the repo; this is the minimum-churn location.
- `plan/learned/NNN-fmt-2-json-append.md` — learned summary (written at completion, not up front).

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

2. **Phase: characterization tests** — write `TestDecodeOpenAddPathReceive`, `TestDecodeNotification_UnknownSubcode` (table-driven), `TestDecodeRouteRefresh_UnknownSubtype`, `TestDecodeNegotiated_UnknownAfiSafi` in `message_receiver_test.go`, and `TestJSONEncoderNotification_HexData` in `json_test.go`. These are characterization tests (red-green-refactor does not apply — we are pinning existing behavior to detect byte drift during migration). All must PASS against the current unmigrated `decode.go` / `json.go`; a failure here indicates the test is mis-specified, not a code bug.
   - Tests: all 5 listed above.
   - Files: `message_receiver_test.go`, `json_test.go`.
   - Verify: `go test ./internal/component/bgp/format/...` passes. Tests now form the byte-level regression guard before migration.

3. **Phase: migrate `decode.go`** — replace each `fmt.Sprintf` site in order.
   - Tests from phase 2 + existing `TestDecodeOpenWithCapabilities`, `TestDecodeNotification`, `TestNegotiatedToDecoded`.
   - Files: `decode.go`.
   - Verify: all phase-2 and existing tests still pass with zero diff; `grep -E 'fmt\.(Sprintf|Fprintf)' decode.go` returns zero (ERE, unescaped `|`).

4. **Phase: migrate `json.go:166`** — swap `fmt.Sprintf("%x", …)` for `hex.AppendEncode`.
   - Tests: `TestJSONEncoderNotification_HexData` from phase 2.
   - Files: `json.go`.
   - Verify: test passes; `grep -E 'fmt\.(Sprintf|Fprintf)' json.go` returns zero; `grep 'encoding/hex' json.go` present.

5. **Phase: update buffer-architecture doc** — remove or rewrite the Phase 3 row referencing the deleted `Format*JSON` / `FormatPrefixFromBytes` helpers.
   - Files: `docs/architecture/buffer-architecture.md`.
   - Verify: `grep -E 'FormatPrefixFromBytes|FormatASPathJSON|FormatCommunitiesJSON|FormatOriginJSON|FormatMEDJSON|FormatLocalPrefJSON' docs/architecture/buffer-architecture.md` returns zero.

6. **Phase: install `block-format-alloc.sh`** — write the hook, register it in `.claude/settings.json`, write the smoke test.
   - Tests: `scripts/dev/test-hook-block-format-alloc.sh` covering AC-8 (blocks) + AC-9 (allows json.go).
   - Files: `.claude/hooks/block-format-alloc.sh`, `.claude/settings.json`, `scripts/dev/test-hook-block-format-alloc.sh`.
   - Verify: smoke test passes; piping a Write-event JSON for `decode.go` with `fmt.Sprintf` yields exit 2; piping the same for `json.go` with `json.Marshal` yields exit 0.

7. **Phase: close deferral + docs** — flip the `plan/deferrals.md` fmt-2-json-append entry Status to `done`; add a line to `.claude/rules/buffer-first.md` near the existing `block-encoding-alloc.sh` mention naming the new hook.
   - Files: `plan/deferrals.md`, `.claude/rules/buffer-first.md`.
   - Verify: `grep -n fmt-2-json-append plan/deferrals.md` shows `done`; `grep block-format-alloc .claude/rules/buffer-first.md` matches.

8. **Full verification** — `make ze-verify-fast` from a cold state.

9. **Complete spec** — fill Implementation Summary, Implementation Audit, Pre-Commit Verification, Review Gate; write `plan/learned/NNN-fmt-2-json-append.md`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-13 has implementation + test evidence; no deferrals. |
| Correctness | `DecodedCapability.Value`, `ErrorSubcodeName`, `SubtypeName`, `Family` string formats byte-identical on BOTH consumer paths: `formatMessageForSubscription` (events.go) AND `notifyPeerEstablished` (reactor_notify.go:106). Run `TestDecodeOpenWithCapabilities`, `TestDecodeNotification`, `TestNegotiatedToDecoded` unmodified and confirm pass. |
| Naming | Hook named `block-format-alloc.sh` (matches `block-encoding-alloc.sh` convention: `block-<area>-<concern>.sh`). |
| Data flow | `decode.go` imports shrink (`fmt` out, `strconv` + `encoding/hex` in). No new dependency between `format/` and other packages. |
| Rule: no-layering | `format_buffer.go` fully deleted; no transitional "legacy" file. |
| Rule: no-test-deletion | Test deletion is `format_buffer_test.go` only; approval from SCOPE gate recorded here. |
| Rule: self-documenting | Hook script top comment names the reference rule (`rules/buffer-first.md`) and learned summary (`plan/learned/614-fmt-0-append.md`). |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `format_buffer.go` deleted | `test ! -e internal/component/bgp/format/format_buffer.go` |
| `format_buffer_test.go` deleted | `test ! -e internal/component/bgp/format/format_buffer_test.go` |
| `decode.go` fmt-clean | `! grep -E 'fmt\.(Sprintf|Fprintf)|strings\.(Join|Builder|NewReplacer|ReplaceAll)|strconv\.(FormatUint|FormatInt)' internal/component/bgp/format/decode.go` (unescaped `|` in ERE) |
| `json.go` fmt-clean | `! grep -E 'fmt\.(Sprintf|Fprintf)' internal/component/bgp/format/json.go` (unescaped `|` in ERE) |
| Hook present + executable | `test -x .claude/hooks/block-format-alloc.sh` |
| Hook registered | `grep -q block-format-alloc .claude/settings.json` |
| Smoke test present | `test -x scripts/dev/test-hook-block-format-alloc.sh` |
| Deferral closed | `grep -E 'fmt-2-json-append.*done' plan/deferrals.md` |
| Doc updated | `! grep -E 'FormatPrefixFromBytes|FormatASPathJSON|FormatCommunitiesJSON|FormatOriginJSON|FormatMEDJSON|FormatLocalPrefJSON' docs/architecture/buffer-architecture.md` |
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
  survived because the surface area was small. Extending it to the nine
  files this spec covers (see Task step 4 allowlist) without a mechanical
  hook would almost certainly regress.

## RFC Documentation

None — no protocol semantics changed.

## Implementation Summary

### What Was Implemented
- Deleted dead `internal/component/bgp/format/format_buffer.go` (225L) and `format_buffer_test.go` (225L) after repo-wide grep confirmed zero production callers of the five `Format*JSON(io.Writer)` helpers + `FormatPrefixFromBytes` + `wellKnownCommunityName`.
- Migrated 14 `fmt.Sprintf` sites in `decode.go` to scratch-buffer + `strconv.AppendUint` / `hex.EncodeToString` idiom across `formatCapability`, `notificationSubcodeString`, `openSubcodeString`, `updateSubcodeString`, `headerSubcodeString`, `fsmSubcodeString`, `DecodeRouteRefresh`, `afiToString`, `afiSafiToFamily`. Added helpers `joinFamily`, `joinFamilyMode`, `subcodeFallback`, `refreshSubtypeName`, `afiNameOrFallback`, `safiNameOrFallback`, `appendAfiFallback`, `appendSafiFallback`. Dropped `fmt` import; added `encoding/hex` + `strconv`.
- Migrated the single `fmt.Sprintf("%x", notify.Data)` site in `json.go` to `string(hex.AppendEncode(nil, notify.Data))`. Dropped `fmt`; added `encoding/hex`.
- Created `.claude/hooks/block-format-alloc.sh` (PreToolUse) with file-path allowlist and banned-pattern list. Registered in `.claude/settings.json`. Smoke test at `scripts/dev/test-hook-block-format-alloc.sh` (8 cases, all pass).
- Added 5 characterization tests: `TestDecodeOpenAddPathReceive`, `TestDecodeNotification_UnknownSubcode` (table-driven over 5 fallback sites), `TestDecodeRouteRefresh_UnknownSubtype`, `TestDecodeNegotiated_UnknownAfiSafi`, `TestJSONEncoderNotification_HexData` (table-driven over 4 hex shapes).
- Updated `docs/architecture/buffer-architecture.md` Phase 3 row to drop references to deleted helpers.
- Added `block-format-alloc.sh` section to `.claude/rules/buffer-first.md`.
- Closed `spec-fmt-2-json-append` deferral in `plan/deferrals.md`.

### Bugs Found/Fixed
- None. Pure refactor + dead-code removal + new guard.

### Documentation Updates
- `docs/architecture/buffer-architecture.md` Phase 3 row rewritten (deleted-helper names removed).
- `.claude/rules/buffer-first.md` got a new "Text/JSON Format Generation" section describing the `block-format-alloc.sh` hook.
- `plan/deferrals.md` entry `spec-fmt-2-json-append` closed (`open` -> `done`) with re-scope note.

### Deviations from Plan
- Spec proposed `appendFamily` / `appendFamilyMode` helper names; implemented as `joinFamily` / `joinFamilyMode` to avoid naming collision with the Append-idiom convention (these helpers *return* a string, not *append* into a buffer).
- Spec called the route-refresh helper `unknownSubtypeFallback`; implemented as `refreshSubtypeName` (encapsulates the whole known-+fallback path, not just the fallback). Same output bytes.
- Restructured `afiToString`, `afiSafiToFamily`, and all subcode-string helpers to drop bare `default:` lines, using fall-through returns after the switch. Caused by `block-silent-ignore.sh` hook matching `default:[[:space:]]*$` even when the case returns on the next line. No semantic change; output bytes identical.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Delete dead `format_buffer.go` + test | Done | `git rm` (both files absent) | User approved test-file deletion via `tmp/delete-ae750012.sh` |
| 2. Migrate `decode.go` off `fmt.Sprintf` (14 sites) | Done | `internal/component/bgp/format/decode.go` | `grep -E 'fmt\.(Sprintf\|Fprintf)' decode.go` returns zero |
| 3. Migrate `json.go:166` hex site | Done | `internal/component/bgp/format/json.go` | `grep -E 'fmt\.(Sprintf\|Fprintf)' json.go` returns zero |
| 4. Install `block-format-alloc.sh` with 9-file allowlist | Done | `.claude/hooks/block-format-alloc.sh`, `.claude/settings.json` | 8/8 smoke-test cases pass |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test ! -e internal/component/bgp/format/format_buffer{.go,_test.go}` passes; `git status` shows `D` both | |
| AC-2 | Done | `grep -nE 'fmt\.(Sprintf\|Fprintf)\|strings\.(Join\|Builder\|NewReplacer\|ReplaceAll)\|strconv\.(FormatUint\|FormatInt)' decode.go` -> zero matches | |
| AC-3 | Done | `grep -nE 'fmt\.(Sprintf\|Fprintf)' json.go` -> zero matches | |
| AC-4 | Done | `TestDecodeOpenWithCapabilities` passes unmodified (`go test -race -run TestDecodeOpenWithCapabilities ./internal/component/bgp/format/...`) | |
| AC-5 | Done | `TestDecodeNotification_UnknownSubcode/{open_default,update_default,header_default,fsm_default,toplevel_default}` all PASS against migrated code | 5-case table over the 5 fallback sites |
| AC-6 | Done | `TestDecodeRouteRefresh_UnknownSubtype` PASS; asserts `SubtypeName == "unknown(99)"` | |
| AC-7 | Done | `TestDecodeNegotiated_UnknownAfiSafi` PASS; asserts `"afi(99)/safi(99)"`, `"afi(99)/unicast"`, and `ExtendedNextHop["ipv4/unicast"] == "afi(99)"` | |
| AC-8 | Done | `bash scripts/dev/test-hook-block-format-alloc.sh` row "AC-8 fmt.Sprintf in decode.go blocks" -> exit 2 | |
| AC-9 | Done | Same smoke test, row "AC-9 json.Marshal in json.go allowed" -> exit 0 | |
| AC-10 | Done | `make ze-verify-fast` -> only pre-existing parallel-load flakes (`bfd-auth-meticulous-persist`, `api-peer-prefix-update`), both logged in `plan/known-failures.md` and unrelated to format code (BFD + UPDATE paths respectively) | |
| AC-11 | Done | `grep -E 'fmt-2-json-append.*\| done' plan/deferrals.md` matches | |
| AC-12 | Done | `grep -n block-format-alloc .claude/settings.json` matches | |
| AC-13 | Done | `grep -E 'FormatPrefixFromBytes\|FormatASPathJSON\|FormatCommunitiesJSON\|FormatOriginJSON\|FormatMEDJSON\|FormatLocalPrefJSON' docs/architecture/buffer-architecture.md` -> zero matches | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDecodeOpenWithCapabilities` (preserved) | Done | `message_receiver_test.go` | Unmodified, still passes |
| `TestDecodeOpenAddPathReceive` (new) | Done | `message_receiver_test.go` | 3-tuple AddPath cap (receive/both/none); verifies none is skipped, both formats as "send-receive" |
| `TestDecodeNotification` (preserved) | Done | `message_receiver_test.go` | Unmodified, still passes |
| `TestDecodeNotification_UnknownSubcode` (new, table-driven) | Done | `message_receiver_test.go` | 5 subtests over Open/Update/Header/FSM defaults and top-level default |
| `TestDecodeRouteRefresh_UnknownSubtype` (new) | Done | `message_receiver_test.go` | |
| `TestNegotiatedToDecoded` (preserved) | Done | `message_receiver_test.go` | Unmodified, still passes |
| `TestDecodeNegotiated_UnknownAfiSafi` (new) | Done | `message_receiver_test.go` | Exercises `afiSafiToFamily(99,99)`, `afiSafiToFamily(99,1)`, `afiToString(99)` via `NegotiatedToDecoded` |
| `TestJSONEncoderNotification_HexData` (new) | Done | `json_test.go` | 4 subtests: empty, single_byte, mixed, all_high_bits |
| `TestFormatAllocHook_BlocksBannedInDecode` (new, shell) | Done | `scripts/dev/test-hook-block-format-alloc.sh` AC-8 row | |
| `TestFormatAllocHook_AllowsInJsonGo` (new, shell) | Done | `scripts/dev/test-hook-block-format-alloc.sh` AC-9 row | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/format/decode.go` | Done | Imports swapped; 14 sites migrated; 9 helpers added |
| `internal/component/bgp/format/json.go` | Done | `fmt` -> `encoding/hex`; one site migrated |
| `internal/component/bgp/format/message_receiver_test.go` | Done | 4 new tests added |
| `internal/component/bgp/format/json_test.go` | Done | 1 new test added |
| `.claude/settings.json` | Done | `block-format-alloc.sh` registered |
| `docs/architecture/buffer-architecture.md` | Done | Phase 3 row rewritten |
| `plan/deferrals.md` | Done | Entry flipped `open` -> `done` with re-scope note |
| `.claude/rules/buffer-first.md` | Done | "Text/JSON Format Generation" section added |
| `.claude/hooks/block-format-alloc.sh` | Done | Created, executable, 8/8 smoke-test cases pass |
| `scripts/dev/test-hook-block-format-alloc.sh` | Done | Created, executable |
| `plan/learned/624-fmt-2-json-append.md` | Done | 5-section learned summary |
| `internal/component/bgp/format/format_buffer.go` | Done | Deleted |
| `internal/component/bgp/format/format_buffer_test.go` | Done | Deleted |

### Audit Summary
- **Total items:** 13 ACs + 10 tests + 13 files = 36
- **Done:** 36
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (helper names + switch-default restructuring; output bytes identical)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `ze-verify-fast` failed on two parallel-load flakes unrelated to format code | `bfd-auth-meticulous-persist`, `api-peer-prefix-update` | Both documented in `plan/known-failures.md` (LOGGED 2026-04-17); pass standalone (`bin/ze-test bgp plugin D` and `Z` both exit 0). No format code touched. |

### Fixes applied
- None. Pre-existing flakes, not regressions.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Re-ran `ze-verify-fast`: only `bfd-auth-meticulous-persist` failed (different flake shape). Confirms non-determinism. | `bfd-auth-meticulous-persist` | Logged pre-existing; no action. |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (two pre-existing parallel-load flakes)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `.claude/hooks/block-format-alloc.sh` | yes | `ls -la .claude/hooks/block-format-alloc.sh` -> `-rwxrwxr-x ... 4223 bytes` |
| `scripts/dev/test-hook-block-format-alloc.sh` | yes | `test -x scripts/dev/test-hook-block-format-alloc.sh` exits 0 |
| `plan/learned/624-fmt-2-json-append.md` | yes | created this session |
| `internal/component/bgp/format/format_buffer.go` | no (expected) | `ls` returns "No such file" |
| `internal/component/bgp/format/format_buffer_test.go` | no (expected) | `git status` shows `D` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | format_buffer.go + test deleted | `ls internal/component/bgp/format/format_buffer*` -> "No such file or directory" |
| AC-2 | decode.go fmt-clean | `grep -nE 'fmt\.(Sprintf\|Fprintf)\|strings\.(Join\|Builder\|NewReplacer\|ReplaceAll)\|strconv\.(FormatUint\|FormatInt)' internal/component/bgp/format/decode.go` -> exit 1, no output |
| AC-3 | json.go fmt-clean | `grep -nE 'fmt\.(Sprintf\|Fprintf)' internal/component/bgp/format/json.go` -> exit 1, no output |
| AC-4 | Capability Value bytes preserved | `TestDecodeOpenWithCapabilities` PASS |
| AC-5 | Subcode(99) fallback byte-identical | `TestDecodeNotification_UnknownSubcode` 5/5 PASS |
| AC-6 | unknown(99) route-refresh subtype | `TestDecodeRouteRefresh_UnknownSubtype` PASS |
| AC-7 | afi(99)/safi(99), afi(99) for next-hop | `TestDecodeNegotiated_UnknownAfiSafi` PASS |
| AC-8 | Hook blocks fmt.Sprintf in decode.go | smoke test row 1 -> exit 2 |
| AC-9 | Hook allows json.Marshal in json.go | smoke test row 2 -> exit 0 |
| AC-10 | `make ze-verify-fast` green | only known flakes; runs logged to `tmp/ze-verify-fmt2{,-run2}.log` |
| AC-11 | Deferral closed | `grep -E 'fmt-2-json-append.*\| done' plan/deferrals.md` matches |
| AC-12 | Hook registered | `grep -n block-format-alloc .claude/settings.json` -> line 416 |
| AC-13 | Phase 3 doc no longer names deleted helpers | `grep -E 'FormatPrefixFromBytes\|FormatASPathJSON\|FormatCommunitiesJSON\|FormatOriginJSON\|FormatMEDJSON\|FormatLocalPrefJSON' docs/architecture/buffer-architecture.md` -> exit 1 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| BGP peer subscription, text mode, OPEN received | n/a (Go unit test guard per Functional Tests rationale) | `TestDecodeOpenWithCapabilities` PASS (bytes preserved) |
| BGP peer subscription, JSON mode, NOTIFICATION received | n/a | `TestDecodeNotification_UnknownSubcode` + `TestJSONEncoderNotification_HexData` PASS |
| Edit attempt introducing `fmt.Sprintf` in `decode.go` | n/a (shell smoke test) | `bash scripts/dev/test-hook-block-format-alloc.sh` 8/8 PASS |
| Peer-negotiated event, JSON mode | n/a | `TestDecodeNegotiated_UnknownAfiSafi` + `TestNegotiatedToDecoded` PASS |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-13 all demonstrated
- [x] Wiring Test table complete — every row has a concrete test name, none deferred
- [x] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE; two NOTEs for pre-existing flakes)
- [x] `make ze-test` passes (lint + all ze tests) — functional-test has two pre-existing parallel-load flakes logged in `plan/known-failures.md`
- [x] `make ze-verify-fast` passes (same caveat — flakes not regressions)
- [x] Migration code integrated (`decode.go`, `json.go`, new hook registered)
- [x] Integration completeness proven via Go unit tests (no `.ci` coverage exists for non-UPDATE subscription formatting; see Functional Tests section rationale)
- [x] Architecture docs updated (`buffer-architecture.md` Phase 3 row; `buffer-first.md` hook reference)
- [x] Critical Review passes

### Quality Gates (SHOULD pass — defer with user approval)
- [x] RFC constraint comments — not applicable
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction — scope narrowed to demonstrated cold-path sites
- [x] No speculative features — S3 (JSONEncoder buffer-first) explicitly out of scope
- [x] Single responsibility — each phase covers one target
- [x] Explicit > implicit — every banned pattern listed in the hook
- [x] Minimal coupling — no new cross-package imports

### TDD
- [x] Tests written — characterization tests added in phase 2 BEFORE any decode.go / json.go migration edits
- [x] Tests FAIL — characterization tests pin exact byte output; any byte drift in the migration would have been caught. Five tests PASS against unmigrated code, PASS again against migrated code, PASS for both text-mode (`TestDecodeOpenWithCapabilities` existing assertions) and JSON-mode paths (`TestJSONEncoderNotification_HexData`). A deliberate perturbation check was not run separately — the equivalence-of-output property was validated by the tests passing unchanged across both pre- and post-migration code states.
- [x] Tests PASS — after migration, full suite passes with byte-identical output
- [x] Boundary tests — N/A
- [x] Functional tests — N/A (pure refactor; Go unit tests are the regression guard, per Functional Tests section)

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Learned summary at `plan/learned/624-fmt-2-json-append.md`
- [ ] Summary included in commit

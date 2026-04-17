# Spec: fmt-0-append

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` -- workflow rules
3. `.claude/rules/buffer-first.md` -- existing `WriteTo(buf, off) int` pattern
4. `.claude/rules/design-principles.md` -- "Buffer-first encoding" and "No `make` where pools exist"
5. `internal/component/bgp/attribute/text.go` -- current `Format*` string APIs
6. `internal/component/bgp/reactor/filter_format.go` -- top Sprintf consumer
7. `internal/component/bgp/format/text.go` -- second Sprintf cluster
8. `internal/component/bgp/format/format_buffer.go` -- partial `io.Writer` precursor

## Task

Replace `fmt.Sprintf` / `strings.Join` / `strings.Builder` / `strconv.FormatUint` clusters in BGP message/attribute/filter text formatting with `AppendXxx(buf []byte, ...) []byte` helpers that mirror the wire `WriteTo(buf []byte, off int) int` pattern in spirit, but use the stdlib `Append*` shape (grow-via-`append`, return new slice) because output size is not pool-bounded.

The hot path that motivates this is the filter dispatch -- `formatSingleAttr` / `FormatAttrsForFilter` / `FormatUpdateForFilter` in `internal/component/bgp/reactor/filter_format.go` runs per attribute per UPDATE per filter plugin, followed by a whole-update send over IPC. Baseline grep across the three in-scope files shows 61 allocation sites for one formatted message. After this refactor each formatted message performs internal work on a stack-local `[]byte` scratch and allocates **exactly one string** at the IPC / subscription-cache boundary (see "Allocation Floor" below). The 10-15x reduction in per-message garbage is the deliverable.

**Allocation floor (not this spec's problem):** `pkg/plugin/rpc/types.go:95` declares `FilterUpdateInput.Update string` and the event cache at `server/events.go:146,249,487,592,663` stores `string`. Both are plugin-facing JSON/IPC contracts. Changing them to `[]byte` would (a) base64-encode the text over the wire, not shrink it; (b) break every external filter plugin via the SDK. A future spec (not created here, no deferral opened) could switch plugin IPC to a length-prefixed raw-bytes wire format. This spec stops at the string-boundary allocation.

Scope is BGP message-formatting allocation reduction inside the three files: attribute types (`attribute.*`), text render (`format/text.go` including its JSON branches that currently use `fmt.Sprintf("%x",...)` for hex), filter dispatch (`reactor/filter_format.go`). Out of scope: `format/format_buffer.go` migration off `io.Writer`+`fmt.Fprintf` (separate follow-up), the two boundary string conversions (Option 3), config text, CLI human output, migration tooling.

## DJB-Inspired Primitives

Daniel J. Bernstein's `fmt`/`buffer`/`stralloc` toolkit (qmail, daemontools, djbdns) solves this exact problem in C. The key ideas, mapped to Go equivalents already in the stdlib:

| DJB primitive | Purpose | Go equivalent |
|---------------|---------|---------------|
| `fmt_ulong(s, n)` / `fmt_xlong` | Write decimal / hex digits to fixed-size stack buffer, return length | `strconv.AppendUint(buf, n, 10)` / `strconv.AppendUint(buf, n, 16)` |
| `fmt_ip4` / `fmt_ip6` | Write IP text to stack buffer | `netip.Addr.AppendTo(buf)` |
| `stralloc_cats(&sa, "literal")` | Append literal to grown accumulator | `append(buf, "literal"...)` |
| `stralloc_catb(&sa, p, n)` | Append byte slice | `append(buf, p[:n]...)` |
| `stralloc_readyplus(&sa, n)` | Pre-grow accumulator | `slices.Grow(buf, n)` (or just let `append` handle it) |
| `buffer` with attached fd + flush | Zero-copy write to output | `scratch []byte` + `w.Write(scratch); scratch = scratch[:0]` |
| `FMT_ULONG` (compile-time max digits) | Fixed-size stack buffer | `make([]byte, 0, 4096)` preallocated once per dispatcher |

DJB invariants we will hold:
1. **No format strings.** Never `fmt.Sprintf("%d", n)` / `fmt.Fprintf(w, "%d", n)`. The reflection path allocates even when the stdlib docs say otherwise. Use named primitives only.
2. **No intermediate string lists.** Never build `[]string` and `strings.Join`. Write element, write separator byte, repeat; track "first" with a bool.
3. **Exactly one `string(buf)` per message, at a named boundary.** The two permitted edges are (a) assigning to `rpc.FilterUpdateInput.Update` just before `CallFilterUpdate` and (b) storing into `fmtCache` in `server/events.go`. Every other code path stays on `[]byte`. This invariant is auditable: grep for `string(` in the three in-scope files returns <= 2 matches, each matching one of the named edges.
4. **Scratch is stack-local to the outer caller.** A `var scratchArr [4096]byte; scratch := scratchArr[:0]` lives on the goroutine's stack inside the outer event / filter-dispatch function. Escape analysis keeps the backing array off the heap as long as the slice does not outlive the function. No struct field, no `sync.Pool`, no per-peer ownership. If the output exceeds 4096 bytes (pathological communities), `append` spills to the heap for that one call -- correct, just not zero alloc in the pathological case.
5. **Primitives are stateless.** `AppendText(buf []byte) []byte` never captures, never caches, never reads anything outside its receiver. No mutexes, no thread-local scratch inside the helper.
6. **Size the stack scratch to the realistic max.** BGP attribute text for one UPDATE is bounded (12 attributes, 50-500 bytes each, extreme communities list 2-4 KB). `[4096]byte` on the stack is cheap and covers the common case. Not `[65535]byte` (too big for a stack frame), not `[512]byte` (too small for communities).

The contrast with DJB: Go's runtime handles growing via `append`, so we do not implement our own `stralloc`. Go's runtime handles bounds checking, so we do not pre-compute sizes (DJB sometimes does a two-pass: compute length, then write; Go makes that unnecessary). We only borrow the philosophy -- fixed set of primitives, buffer-out not string-out, scratch on the consumer.

## Primitives Toolbox

The full set of zero-alloc `appendXxx` helpers used in this spec. Every other append operation is expressed in terms of these. No other formatting function is introduced.

| Helper | Signature | Source | Use |
|--------|-----------|--------|-----|
| stdlib uint | `strconv.AppendUint(buf, n, 10) []byte` | stdlib | ASN, MED, local-preference, cluster-octet, generic `%d` |
| stdlib int | `strconv.AppendInt(buf, n, 10) []byte` | stdlib | Signed integers (rare) |
| stdlib hex | `strconv.AppendUint(buf, n, 16) []byte` | stdlib | Single uint hex (community high/low, error codes) |
| stdlib hex-bytes | `hex.AppendEncode(buf, p) []byte` | stdlib (Go 1.22+) | Message raw-bytes hex, NOTIFICATION data, extended-community bytes |
| stdlib IP | `netip.Addr.AppendTo(buf) []byte` | stdlib | NEXT_HOP, router-id, peer address, local address |
| stdlib prefix | `netip.Prefix.AppendTo(buf) []byte` | stdlib | NLRI CIDR |
| local string-literal | `append(buf, "literal"...) []byte` | builtin | Keyword tokens (`"origin "`, `"as-path "`, `",\"type\":\""`) |
| local byte | `append(buf, ' ') []byte` | builtin | Separators, braces, commas |
| local JSON-escape | `appendJSONString(buf, s) []byte` | new helper in `format/` | Peer name, group, reason, any user-controlled string going into JSON |
| attribute method | `(attribute.*).AppendText(buf) []byte` | new per-type | One attribute rendered in filter-text form (`"origin igp"`, `"as-path [65001 65002]"`) |
| attribute method | `(attribute.*).AppendJSON(buf) []byte` | new per-type (Phase 5 only) | One attribute rendered in JSON value form |
| message helpers | `format.AppendOpen(buf, ...) []byte`, `AppendNotification`, `AppendKeepalive`, `AppendRouteRefresh`, `AppendEOR`, `AppendCongestion`, `AppendStateChange`, `AppendEmptyUpdate` | new in `format/text.go` | Whole-message text / JSON lines |
| dispatch helpers | `reactor.AppendAttrsForFilter(buf, ...) []byte`, `AppendUpdateForFilter(buf, ...) []byte` | replace `FormatAttrsForFilter` / `FormatUpdateForFilter` in `reactor/filter_format.go` | Filter-plugin text dispatch |

Banned from every file in scope:
- `fmt.Sprintf`, `fmt.Fprintf`, `fmt.Appendf` (reflection path allocates regardless of return type)
- `strings.Join`, `strings.Builder`, `strings.Replacer`
- `strconv.FormatUint`, `strconv.FormatInt`, `strconv.Itoa` (return `string`, allocate)
- `string(buf)` except at one named edge per function -- Phase 3/4 outer wrappers that still return `string` for third-party callers not yet migrated. Each such edge is listed in the Deviations table of the completed spec.

## Scratch Discipline

Scratch is a stack-local variable in each outer caller, not a struct field. This makes concurrency moot: every goroutine that formats a message has its own stack frame.

| Outer caller | Where the scratch lives | Source-file evidence |
|--------------|------------------------|---------------------|
| `(*Reactor).notifyMessageReceiver` | Local `var scratchArr [4096]byte` at the top of the function | `internal/component/bgp/reactor/reactor_notify.go:191` |
| `(*reactorAPIAdapter).ForwardUpdate` | Local `var scratchArr [4096]byte` at the top | `internal/component/bgp/reactor/reactor_api_forward.go:156` |
| Server event handlers (`events.go` functions that already declare `var fmtCache formatCache`) | Local `var scratchArr [4096]byte` next to the existing `fmtCache` | `internal/component/bgp/server/events.go:146, 249, 487, 592, 663` |

The pattern at every caller:
1. `var scratchArr [4096]byte`
2. `scratch := scratchArr[:0]`
3. `scratch = format.AppendOpen(scratch, ...)` / `scratch = reactor.AppendAttrsForFilter(scratch, ...)`
4. At the IPC boundary: `input.Update = string(scratch)` -- this is the one permitted allocation.
5. At the cache boundary: `fmtCache.set(cacheKey, string(scratch))` -- this is the other permitted allocation.
6. When the function returns, the stack frame is torn down. No reuse state to manage.

No `sync.Pool`, no per-struct field, no per-peer ownership. The `fmtCache` pattern already established at the five line numbers above is the precedent: scratch state is local to an event handler, not shared.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/buffer-first.md` -- wire encoding uses `WriteTo(buf []byte, off int) int` with pool-bounded buffers
  -> Decision: AppendText is the stdlib idiom for unbounded text; WriteTo is the ze idiom for bounded wire bytes. Both coexist.
  -> Constraint: Never call `make([]byte, N)` in an AppendText helper. Caller owns the buffer and grows it via `append`.
- [ ] `.claude/rules/design-principles.md` -- "Encapsulation onion", "Buffer-first encoding", "Lazy over eager"
  -> Decision: callers pass a scratch buffer reused across invocations; AppendText never stores or caches.
  -> Constraint: No intermediate string concatenation (`"a " + x + " b"`), no `strings.Join`, no `strings.Builder` inside AppendText.
- [ ] `docs/architecture/api/json-format.md` -- message text format contract (consumed by filter plugins, external tools)
  -> Constraint: wire-compatible output; byte-identical to current `Format*` output for every attribute type.
- [ ] `.claude/rules/no-layering.md` -- delete old string APIs after migration, no hybrid.
  -> Decision: existing `Format*(...) string` package functions are deleted (not wrapped) once callers are migrated. Method-level `String()` stays (stdlib `Stringer` interface).

### RFC Summaries
None -- output format is already RFC-compliant; this spec does not change semantics.

**Key insights:**
- Ze already uses AppendTo in two places (`INET.AppendKey`, `PrefixNLRI.AppendTo`) and stdlib AppendTo (`netip.Addr.AppendTo`). The pattern is established, not new.
- `fmt.Appendf` (Go 1.19+) is allowed but still uses reflection and escape-analysis defeats -- not zero-alloc. Use `strconv.AppendInt`, `strconv.AppendQuote`, `append` literals.
- A `[]byte` scratch buffer attached to the per-session or per-plugin-dispatch object is reused across every UPDATE; the `append` growth happens once at warm-up, then amortizes to zero.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/attribute/text.go` (350 lines) -- exports `FormatOrigin`, `FormatASPath`, `FormatCommunity`, `FormatCommunities`, `FormatLargeCommunities`, `FormatExtendedCommunities` plus the method `String()` on attribute types. Uses `fmt.Sprintf("%d:%d", ...)` and `strings.Join([...], " ")` to build `[65001:100 65001:200]` style lists.
  -> Constraint: output format (tokens, spacing, brackets, well-known names) is the contract. No changes.
- [ ] `internal/component/bgp/reactor/filter_format.go` (265 lines) -- `formatSingleAttr` switches on parsed attribute type and returns `fmt.Sprintf("name value", ...)` per attribute. Called from `FormatAttrsForFilter` which joins with space. Runs once per attribute per filter dispatch. This is the hot Sprintf cluster.
  -> Constraint: output text must match byte-for-byte; consumed by `text-mode` filter plugins that parse the tokens.
- [ ] `internal/component/bgp/format/text.go` (510 lines) -- `FormatOpen`, `FormatNotification`, `FormatKeepalive`, `FormatRouteRefresh`, `FormatEOR`, `FormatCongestion`, `formatStateChangeText`, `formatEmptyUpdate`. Each returns `string` built via `strings.Builder` + `fmt.Fprintf` or direct `fmt.Sprintf`. Called on every non-UPDATE message and on every state transition. Allocation floor but not as hot as filter dispatch.
  -> Constraint: text format consumed by CLI output, log subscribers, and external tools piping ze output. Byte-identical.
- [ ] `internal/component/bgp/format/format_buffer.go` (226 lines) -- existing `io.Writer`-based partial refactor. Uses `fmt.Fprintf(w, "%d", ...)` which still allocates via the interface dispatch. Only covers JSON outputs (`FormatASPathJSON`, `FormatCommunitiesJSON`, `FormatOriginJSON`, `FormatMEDJSON`, `FormatLocalPrefJSON`). Text side is untouched.
  -> Decision: do not extend this file; it is the wrong shape (`io.Writer` allocates on `fmt.Fprintf`). Add `AppendText` methods on the attribute types themselves.
- [ ] `internal/component/bgp/nlri/inet.go:190-198` -- reference implementation of `AppendKey`, `AppendString` on `*INET`. Delegates to `netip.Prefix.AppendTo`. Zero-alloc when the caller's buffer has capacity.
  -> Decision: copy this exact signature and delegation pattern for attribute types.

**Behavior to preserve:**
- Every byte of current `Format*` output. No token changes, no ordering changes, no spacing changes.
- `FormatOrigin`, `FormatASPath`, etc. keep their signatures while callers migrate (deleted at end of migration per no-layering).
- `String()` methods on attribute types stay (used by `fmt.Stringer`, logging, error messages).
- Well-known community names (`no-export`, `no-advertise`, ...), large-community format (`ASN:local:arg`), extended-community hex format.

**Behavior to change:**
- None. This is a pure allocation refactor with byte-identical output.

## Data Flow (MANDATORY)

### Entry Point
- Hot path 1: reactor dispatches filter text. `FilterDispatch -> FormatAttrsForFilter(attrs, declared) -> formatSingleAttr(attr)` per attribute. Output format: `"origin igp as-path 65001 next-hop 192.0.2.1 ..."`.
- Hot path 2: subscriber text render on non-UPDATE messages. `server.emit -> FormatOpen / FormatNotification / FormatKeepalive -> strings.Builder`. Output format: `"peer <ip> remote as <asn> <dir> open <id> router-id ..."`.
- Hot path 3: subscriber text render on state events. `FormatStateChange -> formatStateChangeText`. Output format: `"peer <ip> state <name> reason <reason>"`.

### Transformation Path
1. Parsed `attribute.Attribute` (interface) -> type switch -> per-type `AppendText(buf []byte) []byte` (new).
2. Per-type `AppendText` appends tokens directly: `strconv.AppendUint` for numbers, literal byte slices for constants, `netip.Addr.AppendTo` for addresses.
3. Caller concatenates by passing the same buffer through each call, with a single space byte between attributes (`buf = append(buf, ' ')`).
4. Buffer flushed to filter pipe / subscriber writer / log; then sliced to `buf[:0]` for reuse on next message.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor -> filter plugin | stdin bytes | [ ] |
| Reactor -> subscriber | text writer | [ ] |
| attribute pkg -> reactor pkg | method dispatch on `attribute.Attribute` interface | [ ] |
| attribute pkg -> format pkg | method dispatch on parsed attribute types | [ ] |

### Integration Points
- `reactor.dispatchFilter` -- holds a per-session scratch `[]byte`. Currently passes `string`; will pass `[]byte` from the scratch buffer, reset after write.
- `format.formatParsedFromResult` and friends -- currently return `string`; will accept a caller-provided buffer or keep returning `string` at the outermost boundary (at the subscriber-writer edge), with internal helpers appending.

### Architectural Verification
- [ ] No bypassed layers (attribute output still flows through the attribute package; format package keeps its boundary).
- [ ] No unintended coupling (AppendText is local to attribute types; no cross-package struct leaks).
- [ ] No duplicated functionality (`FormatCommunity` deleted once callers migrated; `wellKnownCommunityName` is the single source of truth for names and stays as-is).
- [ ] Zero-copy preserved (buffer reused across UPDATEs; only grows on first call per session).

## Wiring Test (MANDATORY)

**Pin before status `ready`:** Phase-1 research pass must identify the concrete `.ci` file names (currently unpinned because the spec author has not audited `test/plugin/` against the new token set yet). The rows below must be filled with actual file paths, not descriptions, before the spec transitions out of `design`.

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Text-mode filter plugin receives UPDATE | -> | `reactor.AppendAttrsForFilter` / `AppendUpdateForFilter` | `test/plugin/<TBD-filter-text>.ci` -- identify during Phase 1 audit |
| Subscriber receives OPEN text | -> | `format.AppendOpen` | `test/plugin/<TBD-open-text>.ci` -- identify during Phase 1 audit |
| Subscriber receives NOTIFICATION text | -> | `format.AppendNotification` | `test/plugin/<TBD-notif-text>.ci` -- identify during Phase 1 audit |
| Subscriber receives state change text | -> | `format.AppendStateChange` | `test/plugin/<TBD-state-text>.ci` -- identify during Phase 1 audit |

If any row ends up with no existing `.ci` coverage, a new minimal `.ci` test is written in that phase (not deferred).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Attribute types with a text form (`Origin`, `ASPath`, `NextHop`, `MED`, `LocalPref`, `AtomicAggregate`, `Aggregator`, `Communities`, `ClusterList`, `LargeCommunities`, `ExtendedCommunities`) | Each exposes `AppendText(buf []byte) []byte` that appends its filter-text rendering to buf and returns the extended slice. AtomicAggregate appends the bare token `atomic-aggregate` (no value pair); all others append `"<name> <value>"`. |
| AC-2 | Calling `x.AppendText(buf[:0])` twice on the same attribute value with `cap(buf)` pre-sized for the output | Returns byte-identical output both times. Benchmark confirms 0 allocs/op on the second and subsequent calls. |
| AC-3 | `reactor/filter_format.go` | Exports `AppendAttrsForFilter(buf []byte, ...) []byte` and `AppendUpdateForFilter(buf []byte, ...) []byte`. The old `FormatAttrsForFilter` / `FormatUpdateForFilter` / `formatSingleAttr` are deleted. Callers pass a stack-local scratch and convert to `string` exactly once when assigning to `rpc.FilterUpdateInput.Update`. |
| AC-4 | `AppendAttrsForFilter` output captured into a golden file at HEAD before the refactor begins | Post-refactor output byte-identical to the pre-refactor golden for every declared-attribute subset in the test matrix (all attributes, each single attribute, empty declared list, each pair). |
| AC-5 | `format/text.go` non-UPDATE formatters | Each `FormatXxx(...) string` is replaced by `AppendXxx(buf []byte, ...) []byte`. The 7 functions in scope: `AppendOpen`, `AppendNotification`, `AppendKeepalive`, `AppendRouteRefresh`, `AppendEOR`, `AppendCongestion`, `AppendStateChange`. The one-alloc boundary is at `fmtCache.set(cacheKey, string(scratch))` in each server-events handler -- not inside `Append*`. Hex message bytes use `hex.AppendEncode`, not `fmt.Sprintf("%x",...)`. JSON string values use `appendJSONString`, not `strings.Replacer`. |
| AC-6 | Benchmarks `BenchmarkAppendAttrsForFilter`, `BenchmarkAppendOpen`, `BenchmarkAppendNotification` measure the `Append*` call only (not the final `string(scratch)`) | Each reports 0 allocs/op. The one boundary allocation is measured separately in `BenchmarkFormat_Boundary_StringConvert` which is expected to report 1 alloc/op (the `string(scratch)` conversion). This decomposes the "before" total (10-15 allocs/op) into "after" = 0 + 1. |
| AC-7 | Package `internal/component/bgp/attribute` exports | `FormatOrigin`, `FormatASPath`, `FormatCommunity`, `FormatCommunities`, `FormatLargeCommunities`, `FormatExtendedCommunities` are deleted (no-layering). Their last callers migrated to AppendText. `String()` methods are preserved for `fmt.Stringer` and debug logging. |
| AC-8 | `grep -n 'fmt.Sprintf\|fmt.Fprintf\|fmt.Appendf\|strings.Join\|strings.Builder\|strings.Replacer\|strconv.FormatUint\|strconv.FormatInt\|strconv.Itoa' internal/component/bgp/reactor/filter_format.go internal/component/bgp/format/text.go internal/component/bgp/attribute/text.go` | Returns zero matches. (Note: `format/format_buffer.go` is out of scope and still contains `fmt.Fprintf` -- that is the separate follow-up.) |
| AC-9 | `grep -cn 'string(' internal/component/bgp/reactor/filter_format.go internal/component/bgp/format/text.go internal/component/bgp/attribute/text.go` plus the migrated callers of `rpc.FilterUpdateInput` and `fmtCache.set` | The count of `string(...)` conversions in the three in-scope files plus the two boundary call sites is <= 3 total: one for `rpc.FilterUpdateInput.Update` assignment (in `filter_chain.go:policyFilterFunc` or its migration target), one for each `fmtCache.set` pattern in `server/events.go` that takes `scratch`. Any additional `string(...)` is a bug. |
| AC-10 | `make ze-verify-fast` | Passes with no new failures. |
| AC-11 | `make ze-race-reactor` (reactor touched) | Passes with `-race -count=20`. |
| AC-12 | `appendJSONString(buf []byte, s string) []byte` helper | Produces byte-identical output to the old `jsonSafeReplacer.Replace(s)` + manual quote for every string in the test corpus (including empty, ASCII, control chars 0x00-0x1F, embedded quotes, embedded backslashes). Zero allocs measured via benchmark. |

## Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAppendText_Origin` | `internal/component/bgp/attribute/text_append_test.go` | `(*Origin).AppendText` matches `FormatOrigin` for `igp`, `egp`, `incomplete` and an unknown value. | |
| `TestAppendText_ASPath` | `internal/component/bgp/attribute/text_append_test.go` | Empty, single ASN (no brackets), multiple ASNs (with brackets), AS_SET segments. | |
| `TestAppendText_Community_WellKnown` | `internal/component/bgp/attribute/text_append_test.go` | All six well-known communities render to their names. | |
| `TestAppendText_Community_Plain` | `internal/component/bgp/attribute/text_append_test.go` | Boundary communities (16-bit ASN, 16-bit value): `0:0`, `0:65535`, `65535:0`, `65535:65535`. |
| `TestAppendText_Communities` | `internal/component/bgp/attribute/text_append_test.go` | Empty, single, multiple (brackets), mix of well-known and plain. | |
| `TestAppendText_LargeCommunities` | `internal/component/bgp/attribute/text_append_test.go` | Boundary LC values; single vs multi bracket handling. | |
| `TestAppendText_ExtendedCommunities` | `internal/component/bgp/attribute/text_append_test.go` | Hex format, single vs multi bracket handling. | |
| `TestAppendText_Aggregator` | `internal/component/bgp/attribute/text_append_test.go` | 2-byte and 4-byte ASN forms render identically. | |
| `TestAppendText_ClusterList` | `internal/component/bgp/attribute/text_append_test.go` | Empty returns unchanged buf; single vs multiple IDs. | |
| `TestAppendText_NextHop` | `internal/component/bgp/attribute/text_append_test.go` | IPv4 and IPv6 next-hop render via `netip.Addr.AppendTo`. | |
| `TestAppendText_MED_LocalPref` | `internal/component/bgp/attribute/text_append_test.go` | Zero and max uint32 boundary values. | |
| `TestAppendText_BufferReuse_NoGrow` | `internal/component/bgp/attribute/text_append_test.go` | After first call sizes the buffer, a second `AppendText(buf[:0])` returns the same underlying array (cap unchanged). | |
| `TestFormatAttrsForFilter_ByteIdenticalSnapshot` | `internal/component/bgp/reactor/filter_format_test.go` | Runs a golden-file comparison against the pre-refactor output for a fixed set of attribute combinations. | |
| `TestFormatOpen_AppendParity` | `internal/component/bgp/format/text_append_test.go` | New AppendText-based OPEN format == old Sprintf-based OPEN format. | |
| `TestFormatNotification_AppendParity` | `internal/component/bgp/format/text_append_test.go` | Same, for NOTIFICATION. Includes data-bytes hex branch. | |
| `TestFormatKeepalive_AppendParity` | `internal/component/bgp/format/text_append_test.go` | Same, for KEEPALIVE. | |
| `TestFormatStateChange_AppendParity` | `internal/component/bgp/format/text_append_test.go` | Both `up` (no reason) and `down` (with reason) branches. | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN (plain community) | 0-65535 | 65535 | N/A | N/A (upper bits mapped to 4-byte community) |
| Community value | 0-65535 | 65535 | N/A | N/A |
| Large-community global | 0-4294967295 | 4294967295 | N/A | N/A |
| MED / LocalPref | 0-4294967295 | 4294967295 | N/A | N/A |
| Cluster ID octet | 0-255 | 255 | N/A | N/A |

### Allocation Tests
| Benchmark | File | Validates | Status |
|-----------|------|-----------|--------|
| `BenchmarkFormatAttrsForFilter_Reused` | `internal/component/bgp/reactor/filter_format_bench_test.go` | `b.ReportAllocs()` shows 0 allocs/op on warm scratch buffer with representative 12-attribute UPDATE. | |
| `BenchmarkAppendTextCommunity` | `internal/component/bgp/attribute/text_append_bench_test.go` | Zero allocs/op for repeated appends into a reused buffer. | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Pick one existing filter-text `.ci` test | `test/plugin/` | Filter plugin receives expected text tokens -- byte-identical before/after refactor. | |
| Pick one existing notification-text `.ci` test | `test/plugin/` | Subscriber receives NOTIFICATION text -- byte-identical. | |

No new `.ci` tests are required because the output is byte-identical; the existing suite is the regression guard. The spec lists the two .ci files to be identified during research/implement and pinned here before marking the spec ready.

### Future (if deferring any tests)
- JSON output (`format/format_buffer.go`) migration from `io.Writer` + `fmt.Fprintf` to AppendText -- separate spec. Deferred.

## Files to Modify

- `internal/component/bgp/attribute/text.go` -- add `AppendText(buf []byte) []byte` methods on the 11 attribute types listed in AC-1. Delete `FormatOrigin`, `FormatASPath`, `FormatCommunity`, `FormatCommunities`, `FormatLargeCommunities`, `FormatExtendedCommunities` at end of migration.
- `internal/component/bgp/reactor/filter_format.go` -- replace `FormatUpdateForFilter`, `FormatAttrsForFilter`, `formatSingleAttr`, `formatAllAttrs`, `formatMPBlock`, `formatNLRIBlock` with `Append*` equivalents. No struct field -- scratch is stack-local at the outer callers.
- `internal/component/bgp/reactor/reactor_notify.go` -- at the `FormatUpdateForFilter` call site (line 382), declare stack scratch, call `AppendUpdateForFilter`, convert once for `PolicyFilterChain`.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- same treatment at line 419.
- `internal/component/bgp/reactor/peer_initial_sync.go` -- `policyFilterFunc(nil)` synthetic-update call at line 842; follow the same pattern.
- `internal/component/bgp/reactor/filter_chain.go` -- optionally change `PolicyFilterFunc` signature to take `[]byte`; convert to `string` only at the `rpc.FilterUpdateInput.Update` assignment. Decide in Phase 2 audit.
- `internal/component/bgp/format/text.go` -- replace the 7 public `FormatXxx(...) string` functions with `AppendXxx(buf []byte, ...) []byte`. Replace `fmt.Sprintf("%x",...)` with `hex.AppendEncode`. Replace `jsonSafeReplacer` + `writeJSONEscapedString` with `appendJSONString`. Remove `strings.Builder` from this file.
- `internal/component/bgp/server/events.go` -- migrate the 10 `format.FormatXxx` call sites (lines 358, 365, 371, 378, 494, 534, 596, 616, 733, 759). Each handler already declares `var fmtCache formatCache`; add `var scratchArr [4096]byte; scratch := scratchArr[:0]` next to it, format into `scratch`, then `fmtCache.set(cacheKey, string(scratch))`.
- `internal/component/bgp/format.go` -- if it re-exports any of the deleted `Format*` names, update.
- `internal/component/lg/render_test.go` -- migrate from deleted `Format*` symbols to AppendText equivalents.
- `internal/component/bgp/plugins/rib/rib_attr_format_test.go` -- same.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | n/a |
| CLI commands/flags | [ ] No | n/a |
| Editor autocomplete | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No | existing .ci regression coverage |

### Documentation Update Checklist
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
| 10 | Test infrastructure changed? | [ ] No | n/a |
| 11 | Affects daemon comparison? | [ ] No | n/a |
| 12 | Internal architecture changed? | [ ] Yes | `.claude/rules/buffer-first.md` -- add "AppendText (stdlib-style)" row next to the WriteTo table, describing when to use which. Also `.claude/rules/design-principles.md` "Buffer-first encoding" principle -- extend to call out the text-output counterpart. |

## Files to Create

- `internal/component/bgp/attribute/text_append_test.go` -- unit tests for each attribute AppendText method.
- `internal/component/bgp/attribute/text_append_bench_test.go` -- zero-alloc benchmarks.
- `internal/component/bgp/reactor/filter_format_bench_test.go` -- dispatcher benchmark with reused scratch.
- `internal/component/bgp/format/text_append_test.go` -- parity tests for non-UPDATE message text + `appendJSONString`.
- `internal/component/bgp/format/testdata/*.golden` -- frozen pre-refactor output samples captured at HEAD before Phase 3 begins. One file per message type.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` + `make ze-race-reactor` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Apply fixes |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase 1: Attribute AppendText methods (leaf layer)**
   - Write `TestAppendText_*` table tests with pre-computed golden strings derived from the existing `Format*` output. Tests fail (no methods yet).
   - Add `AppendText(buf []byte) []byte` on each attribute type by porting the existing `String()` / `Format*` logic to `strconv.AppendUint`, `netip.Addr.AppendTo`, and literal `append` calls. No allocations inside.
   - Tests pass.
   - Files: `attribute/text.go`, `attribute/text_append_test.go`.

2. **Phase 2: Filter dispatch migration (hot path, highest value)**
   - Capture a pre-refactor golden file for `FormatUpdateForFilter` output across a matrix of UPDATEs.
   - Rewrite `formatSingleAttr`, `formatAllAttrs`, `formatMPBlock`, `formatNLRIBlock`, `FormatAttrsForFilter`, `FormatUpdateForFilter` into `Append*` equivalents taking `buf []byte` and returning `[]byte`.
   - Update the two call sites in `reactor/reactor_notify.go:382` and `reactor/reactor_api_forward.go:419`: declare `var scratchArr [4096]byte; scratch := scratchArr[:0]` local to the function, call `scratch = AppendUpdateForFilter(scratch, ...)`, then pass `string(scratch)` to `PolicyFilterChain`.
   - Optional: change `PolicyFilterChain` and `PolicyFilterFunc` signatures to take `[]byte` internally, converting to `string` only at the final `rpc.FilterUpdateInput.Update` assignment in `policyFilterFunc`. This removes one `string(...)` between the scratch and the IPC adapter but is purely internal (no SDK impact). Decide during implement-audit; default yes.
   - Write `TestAppendUpdateForFilter_GoldenParity` and `BenchmarkAppendUpdateForFilter` (0 allocs/op) + `BenchmarkAppendUpdateForFilter_WithBoundary` (1 alloc/op for the final string conversion).
   - Files: `reactor/filter_format.go`, `reactor/filter_chain.go` (callback signature if we take the optional change), `reactor/reactor_notify.go`, `reactor/reactor_api_forward.go`, `reactor/peer_initial_sync.go` (also calls `policyFilterFunc`).

3. **Phase 3: Non-UPDATE message formatters migrated to Append signature**
   - Capture a golden file of the current `FormatOpen` / `FormatNotification` / etc. output for a fixed corpus of messages. Commit as `testdata/` (tests read the golden; before-refactor snapshot is frozen).
   - Rewrite each outer function to the new signature: `AppendOpen(buf []byte, ...) []byte`, `AppendNotification`, `AppendKeepalive`, `AppendRouteRefresh`, `AppendEOR`, `AppendCongestion`, `AppendStateChange`. `formatEmptyUpdate` is internal; fold into `AppendMessage` or leave as `appendEmptyUpdate`.
   - Replace `fmt.Sprintf("%x", msg.RawBytes)` with `hex.AppendEncode(buf, msg.RawBytes)`.
   - Replace `strings.Replacer`-based JSON escaping and `writeJSONEscapedString` (which uses `fmt.Fprintf`) with a new local helper `appendJSONString(buf []byte, s string) []byte` that handles `\`, `"`, `\n`, `\r`, `\t`, and control chars 0x00-0x1F via a direct loop.
   - Migrate each caller in `server/events.go` (lines 358, 365, 371, 378, 494, 534, 596, 616, 733, 759) to the new `Append` form. Each caller already declares `var fmtCache formatCache` locally -- add `var scratchArr [4096]byte; scratch := scratchArr[:0]` next to it. Format into `scratch`, then `fmtCache.set(cacheKey, string(scratch))`. That `string(scratch)` is the permitted boundary allocation.
   - Parity tests: `TestAppendOpen_GoldenParity` etc., compare the `Append*`-produced bytes to the frozen golden.
   - Benchmarks: `BenchmarkAppendOpen` / `BenchmarkAppendNotification` measure the `Append*` call only (expected 0 allocs/op). A separate `BenchmarkFormat_Boundary_StringConvert` measures the `string(scratch)` edge (expected 1 alloc/op).
   - Files: `format/text.go`, `format/text_append_test.go`, `format/testdata/*.golden`, `server/events.go`.

4. **Phase 4: Delete old string-returning helpers**
   - Grep for remaining callers of the six attribute `Format*` helpers and the eight `format.FormatXxx` message helpers from Phase 3.
   - Migrate any leftover call site to the `Append` form (caller passes in `buf[:0]` or a reusable scratch).
   - Delete all deprecated helpers. Per `rules/no-layering.md`, no wrapper, no transitional shim.
   - Update `internal/component/lg/render_test.go` and `internal/component/bgp/plugins/rib/rib_attr_format_test.go` to use `AppendText`.

5. **Phase 5: Full verification**
   - `make ze-verify-fast`
   - `make ze-race-reactor` (reactor touched)
   - Pick two existing `.ci` tests covering filter-text and notification-text paths; run them and confirm byte-identical output.

6. **Phase 6: Complete spec**
   - Fill audit tables. Fill pre-commit verification. Fill review gate. Write learned summary.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has implementation with file:line |
| Correctness | Byte-identical output verified against frozen golden for every attribute type and every message type |
| Naming | `AppendText` on attribute types. `AppendOpen` / `AppendNotification` etc. as free functions in `format/`. `AppendAttrsForFilter` / `AppendUpdateForFilter` in `reactor/`. |
| Data flow | Scratch is stack-local in each outer caller (no struct field, no sync.Pool). The two allowed `string(scratch)` sites are named in AC-9 and no others exist. |
| Rule: no-layering | All 14 deleted helpers confirmed absent from final tree; no transitional wrapper. |
| Rule: buffer-first / no-allocation | Banned-list grep (AC-8) returns zero matches. Benchmarks (AC-6) report 0 allocs/op for the `Append*` path and 1 alloc/op for the named boundary site. |
| Boundary discipline | Only two `string(scratch)` allocations per formatted message: one at `rpc.FilterUpdateInput.Update`, one at `fmtCache.set`. Every other consumer stays on `[]byte`. |
| File modularity | `attribute/text.go` line count after Phase 4 <= 600. If not, split into `text.go` (String methods + constants) + `text_append.go` (AppendText methods). |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| AppendText method on all 11 attribute types | `grep -n 'func (.*) AppendText(buf \[\]byte) \[\]byte' internal/component/bgp/attribute/text.go` returns 11 matches |
| Zero banned calls in targeted files (AC-8) | `grep -n 'fmt.Sprintf\|fmt.Fprintf\|fmt.Appendf\|strings.Join\|strings.Builder\|strings.Replacer\|strconv.FormatUint\|strconv.FormatInt\|strconv.Itoa' internal/component/bgp/reactor/filter_format.go internal/component/bgp/format/text.go internal/component/bgp/attribute/text.go` returns nothing |
| Old `Format*` attribute helpers deleted | `grep -n '^func FormatOrigin\|^func FormatASPath\|^func FormatCommunity\|^func FormatCommunities\|^func FormatLargeCommunities\|^func FormatExtendedCommunities' internal/component/bgp/attribute/text.go` returns nothing |
| Old `format.FormatXxx` message helpers deleted | `grep -n '^func FormatOpen\|^func FormatNotification\|^func FormatKeepalive\|^func FormatRouteRefresh\|^func FormatEOR\|^func FormatCongestion\|^func FormatStateChange' internal/component/bgp/format/text.go` returns nothing |
| New `AppendXxx` message helpers exported | `grep -n '^func AppendOpen\|^func AppendNotification\|^func AppendKeepalive\|^func AppendRouteRefresh\|^func AppendEOR\|^func AppendCongestion\|^func AppendStateChange' internal/component/bgp/format/text.go` returns 7 matches |
| `appendJSONString` helper present | `grep -n '^func appendJSONString' internal/component/bgp/format/text.go` returns 1 match |
| Boundary discipline: only the two allowed `string(scratch)` sites | Grep finds `string(scratch)` only (a) in the caller of `AppendUpdateForFilter` before `PolicyFilterChain` / `FilterUpdateInput.Update`, and (b) at each `fmtCache.set` in `server/events.go`. |
| Benchmarks report expected allocs/op | `go test -bench BenchmarkAppend -benchmem -run ^$ ./internal/component/bgp/...` -- every `BenchmarkAppend*` line ends `0 allocs/op`; `BenchmarkFormat_Boundary_StringConvert` ends `1 allocs/op`. |
| Race passes | `make ze-race-reactor` passes |
| `.ci` regression | Every `.ci` file pinned in Wiring Test table runs with exit 0 and unchanged stdout |
| Golden parity | `format/testdata/*.golden` diffs clean against new `Append*` output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | AppendText takes parsed types, not raw bytes -- no injection surface. Confirm no `%s`-style user input reaches the append path. |
| Buffer bounds | `append` grows via Go runtime; no manual indexing, no OOB risk. Confirm no `buf[off] = x` pattern used; only `append(buf, ...)`. |
| Output format contract | Any byte-level drift breaks filter plugins. Golden-file parity tests enforce byte-for-byte equality. |
| Denial via large input | Output size scales linearly with attribute count; no unbounded amplification. Existing message-size bounds already cap attribute count. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Parity test fails | Re-read the old `Format*` implementation; identify the token or separator the new AppendText drops |
| Benchmark shows allocs/op > 0 | Grep the new code for `fmt.`, `strings.Join`, `make`, `strconv.FormatUint` (returns string, allocs); replace with `strconv.AppendUint` |
| Reactor race test fails | Dispatcher scratch is shared across goroutines; confine it to one goroutine or use per-goroutine storage |
| 3 fix attempts fail | STOP, report approaches, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

## Implementation Audit

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

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

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

### Goal Gates
- [ ] AC-1..AC-12 all demonstrated (AC-9 pins the one-alloc-at-boundary invariant, AC-11 the reactor race)
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates
- [ ] Benchmarks report 0 allocs/op on reused scratch path
- [ ] Old `Format*` exported functions fully deleted
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per file
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests included
- [ ] Functional regression tests identified

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-fmt-0-append.md`
- [ ] Summary included in commit

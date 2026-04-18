# 624 -- fmt-2-json-append

## Context

`spec-fmt-0-append` closed the hot-path text/JSON allocations in
`internal/component/bgp/format/` but left residual `fmt.Sprintf` / `fmt.Fprintf`
call sites in three cold-path locations: `format_buffer.go` (5 `Format*JSON`
writers plus `FormatPrefixFromBytes` and `wellKnownCommunityName`), `decode.go`
(14 sites in `formatCapability`, `notificationSubcodeString` and friends,
`DecodeRouteRefresh`, `afiToString`, `afiSafiToFamily`), and one site in
`json.go` (`fmt.Sprintf("%x", notify.Data)`). fmt-0 also shipped a
hand-maintained banned-pattern guard across three files, relying on code-review
discipline rather than a mechanical hook. The goal was to close the residual
sites and graduate the guard to a real PreToolUse hook.

## Decisions

- **Delete `format_buffer.go` (225L) + its test file (225L) over migrating.**
  Repo-wide grep at the SCOPE gate showed zero production callers of the five
  `Format*JSON(io.Writer)` helpers (matches only in the file, its test, the
  deferral log, the spec, and one line in `docs/architecture/buffer-architecture.md`).
  The original fmt-0 deferral assumed these were live cold-path writers; the
  verification at SCOPE proved they were dead. Chose `git rm` over an
  `AppendXxx` rewrite because a rewrite would have preserved dead code.
  `rules/no-test-deletion.md` permits the test-file deletion under the
  "testing removed functionality" exception.
- **Preserve `json.go`'s `map[string]any` + `json.Marshal` idiom.** Migrating
  the whole JSONEncoder to buffer-first (spec-fmt S3) is ~400L of cold-path
  churn for no measurable perf win; deferred until a concrete need emerges.
  The hook therefore explicitly excludes `json.go` from its allowlist (AC-9).
- **Scratch-buffer idiom over simple string concat.** For each migrated site,
  used a stack-allocated `[16]byte` / `[32]byte` / `[64]byte` scratch with
  `append` + `strconv.AppendUint` + a single `string(out)` at return. Avoids
  `fmt` reflection (the main CPU cost) and matches the fmt-0 Append idiom.
- **Restructured switch statements to drop bare `default:` lines.** The
  existing `block-silent-ignore.sh` hook matches `default:[[:space:]]*$` as
  part of its "empty default" heuristic and blocks any bare `default:` even
  when the case returns on the next line. Rewrote `afiToString`,
  `afiSafiToFamily`, `openSubcodeString`, `updateSubcodeString`,
  `headerSubcodeString`, `fsmSubcodeString`, `notificationSubcodeString`,
  `refreshSubtypeName`, `formatCapability` to use fall-through returns after
  the switch. Net result is idiomatic Go and the hook passes; semantics
  unchanged.
- **`block-format-alloc.sh` PreToolUse hook over hand-maintained discipline.**
  The fmt-0 guard tracked only three files; fmt-2's allowlist covers nine.
  Graduating to a mechanical hook makes the surface enforceable across
  future work. Bans `fmt.Sprintf`, `fmt.Fprintf`, `strings.Join`,
  `strings.Builder`, `strings.NewReplacer`, `strings.ReplaceAll`,
  `strconv.FormatUint`, `strconv.FormatInt`. `strings.NewReplacer` /
  `strings.ReplaceAll` are defensive adds (no current occurrences).
- **Smoke test lives at `scripts/dev/test-hook-block-format-alloc.sh`.** No
  dedicated hook-test directory convention exists; placed alongside
  `verify-lock.sh`, `verify-status.sh`, etc. to minimise churn.

## Consequences

- `internal/component/bgp/format/` is now fully fmt-clean on the write side:
  `decode.go`, `text.go`, `text_json.go`, `text_update.go`, `text_human.go`,
  `summary.go`, `codec.go`, plus `reactor/filter_format.go` and
  `attribute/text.go`. `json.go` remains the only in-package file that uses
  `json.Marshal` (not `fmt.Sprintf`) and is intentionally out of the hook's
  allowlist.
- Future BGP text/JSON format authors get the banned list enforced
  automatically at Write/Edit time. The hook's allowlist must be extended
  when a new format-generation file is introduced (failure mode: new file
  bypasses the guard; check by grepping for `fmt.Sprintf` in
  `internal/component/bgp/format/`).
- `DecodedCapability.Value` / `ErrorSubcodeName` / `SubtypeName` / `Family`
  byte-level output unchanged. Both consumer paths verified:
  `formatMessageForSubscription` (events.go) and `notifyPeerEstablished`
  (reactor_notify.go:106).
- Running `fmt.Sprintf("%d", u32)` through `strconv.AppendUint` eliminates
  `fmt`'s reflection overhead (~2-4x CPU cost per call) on these cold
  paths. Does NOT eliminate the `string` allocation itself (the struct
  fields stay `string`-typed). Zero-alloc would require a struct rewrite,
  tracked as S3-scope for a future spec.

## Gotchas

- **Hook pattern surprise.** `block-silent-ignore.sh` matches bare
  `default:` lines as a proxy for "empty default case," which fires on
  every standard Go switch statement. Any future format-package edit with a
  `default:` must restructure or inline the return. The fmt-2 Write was
  blocked on first attempt; fixed by removing the `default:` keyword
  entirely and returning after the switch.
- **AFI/SAFI String() fallback differs from `afiSafiToFamily` fallback.**
  `capability.AFI.String()` returns `"afi-99"` (dash) via the family package;
  `afiSafiToFamily(99, 99)` returns `"afi(99)"` (parens). The spec/tests
  preserved both. Do NOT "simplify" by delegating to `family.Family.String()`.
- **`hex.AppendEncode(nil, nil)` returns `nil`; `string(nil) == ""`.** The
  migrated `json.go` emits `""` for empty `notify.Data`, matching the prior
  `fmt.Sprintf("%x", nil) == ""` behaviour. Pinned by
  `TestJSONEncoderNotification_HexData` (covers empty, single-byte, mixed,
  and all-high-bits inputs).
- **Parallel-suite flakes are not regressions.** `ze-verify-fast` surfaced
  `bfd-auth-meticulous-persist` and `api-peer-prefix-update` as
  intermittent failures; both are documented in
  `plan/known-failures.md` (LOGGED 2026-04-17) and pass standalone. No
  format code is on either test's path.
- **Test deletion hook still requires user approval.** `block-test-deletion.sh`
  cannot be bypassed by in-chat "yes"; must run a deletion script
  (`tmp/delete-<session>.sh`) from the harness shell. Not a fmt-2 issue --
  structural constraint for any test-file removal.

## Files

- Deleted: `internal/component/bgp/format/format_buffer.go`,
  `internal/component/bgp/format/format_buffer_test.go`.
- Modified: `internal/component/bgp/format/decode.go` (14 fmt.Sprintf sites
  migrated; scratch-buffer idiom helpers added),
  `internal/component/bgp/format/json.go` (hex site migrated),
  `internal/component/bgp/format/message_receiver_test.go` (4 new tests),
  `internal/component/bgp/format/json_test.go` (1 new hex test),
  `docs/architecture/buffer-architecture.md` (Phase 3 row rewritten),
  `.claude/rules/buffer-first.md` (new hook noted),
  `.claude/settings.json` (hook registered),
  `plan/deferrals.md` (fmt-2-json-append entry closed).
- Created: `.claude/hooks/block-format-alloc.sh`,
  `scripts/dev/test-hook-block-format-alloc.sh`.

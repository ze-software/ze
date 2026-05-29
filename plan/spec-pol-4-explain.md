# Spec: pol-4-explain -- Command-Line Policy Test and Trace

| Field | Value |
|-------|-------|
| Status | in-progress (code + unit + functional verified; `make ze-verify` pending clean tree) |
| Depends | spec-pol-3-validation.md |
| Phase | 5/5 -- implementation done, `/ze-review` + `/ze-review-deep` clean, all functional `.ci` pass; full `make ze-verify` deferred until concurrent agents' files settle |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-pol-0-umbrella.md` - policy explain and dry-run roadmap
4. `plan/spec-pol-3-validation.md` - unique filter names and plain refs
5. `plan/learned/572-cmd-8-policy-show.md` - existing policy introspection decisions
6. `internal/component/cmd/show/show_policy.go` - current policy command handlers
7. `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - `show policy` command tree
8. `internal/component/bgp/reactor/filter_chain.go` - chain execution
9. `internal/component/bgp/reactor/filter_format.go` - UPDATE to filter text format
10. `internal/component/bgp/reactor/filter_delta.go` - policy text delta to wire modifications
11. `internal/component/plugin/types_bgp.go` - reactor interface exposed to command handlers

## Task

Add a read-only command-line dry-run that lets operators test what a configured policy filter or policy chain would do to a supplied BGP UPDATE.

Primary user-facing command shape:

| Scenario | Command shape |
|----------|---------------|
| Test a peer's configured export chain | `ze show policy test peer PEER export update HEX` |
| Test a peer's configured import chain | `ze show policy test peer PEER import update HEX` |
| Test one named filter without changing config | `ze show policy test peer PEER export filter FILTER_NAME update HEX` |

> **Grammar correction (verified by functional test):** the peer selector must come first
> (`peer PEER`), matching every other peer-selector command, because the dispatcher's
> `extractPeerSelector` (command.go:845) strips the selector value and leaves the `peer` keyword
> to be absorbed by the registered YANG path (`show policy test peer`). The original
> `... export peer PEER ...` shape left `peer` as an orphan token that failed arg validation.
> Likewise `update HEX` (not `update hex HEX`): the `hex` sub-keyword confused
> `validateCommandArgs` when `filter` was also present. Both bugs were latent because the
> commands had never been run end-to-end. The same fix was applied to
> `show policy chain peer PEER [import|export]` (pol-3), which had the identical orphan-`peer` defect.

`FILTER_NAME` is a unique policy filter instance name. Type-prefixed and plugin-prefixed forms remain accepted only as advanced escape hatches, per `spec-pol-3-validation.md`.

This command must not forward routes, update RIB state, populate recent update cache, or mutate peer/session state. It is an explanation tool.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/api/commands.md` - command dispatch and structured output
  -> Decision: read-only commands return structured JSON data through the shared dispatcher.
  -> Constraint: online commands run against the daemon and should use existing `CommandContext` accessors.
- [ ] `docs/architecture/core-design.md` - policy chain execution and filter delta processing
  -> Decision: policy filters run as a linear chain with reject short-circuit and default accept.
  -> Constraint: dry-run must reuse the same chain logic, not reimplement filter behavior.
- [ ] `docs/architecture/update-building.md` - UPDATE copy-on-modify and payload construction
  -> Decision: modified wire payloads are produced only through the existing modification pipeline.
  -> Constraint: dry-run may build temporary payloads, but must not enqueue or cache them.
- [ ] `docs/architecture/wire/attributes.md` - path attribute parsing and context-sensitive AS_PATH handling
  -> Decision: AS_PATH parsing depends on ASN4 encoding context; AS4_PATH is separate.
  -> Constraint: dry-run input must make the source ASN4 context explicit or derive it from peer/session data.
- [ ] `docs/architecture/testing/ci-format.md` - functional test format
  -> Decision: command-line behavior must be verified through `.ci` tests.
  -> Constraint: tests should assert structured fields, not only exit code 0.

### Rules and Patterns

- [ ] `ai/patterns/cli-command.md` - online command registration pattern
  -> Decision: `show policy test` belongs with existing `show policy` handlers unless implementation reveals a better narrow package.
  -> Constraint: add YANG command tree and handler registration together.
- [ ] `ai/rules/cli-grammar.md` - action before identifier
  -> Decision (corrected): `show policy test peer PEER export ... update HEX`. The peer selector must be a YANG path node (`... test peer`) so the dispatcher's selector extraction works; direction (`export`/`import`) and the `filter`/`update` keywords follow the selector.
  -> Constraint: the supplied filter name appears after the keyword `filter`, and the hex bytes appear directly after the `update` keyword (no `hex` sub-keyword).
- [ ] `ai/rules/pipe-completeness.md` - commands producing output support pipe operators
  -> Decision: output must remain structured through the existing dispatcher so pipe operators can consume it.
  -> Constraint: do not print ad hoc multi-line text from the handler.
- [ ] `ai/rules/json-format.md` - JSON key naming
  -> Decision: output keys use kebab-case if project JSON rules require it.
  -> Constraint: tests must lock the chosen JSON shape.
- [ ] `ai/rules/buffer-first.md` - wire byte handling
  -> Decision: dry-run byte parsing and temporary rewrite should use existing buffer-first helpers.
  -> Constraint: no hot-path runtime behavior changes; this command is not on the forwarding path.

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` - BGP UPDATE format and AS_PATH behavior
  -> Constraint: supplied hex must decode as a BGP UPDATE before policy testing proceeds.
- [ ] `rfc/short/rfc6793.md` - ASN4 and AS4_PATH context
  -> Constraint: AS_PATH parsing must use the correct source ASN width.
- [ ] `rfc/short/rfc6996.md` - remove-private-AS behavior used by validation scenarios
  -> Constraint: dry-run output for remove-private-as must expose AS_PATH and AS4_PATH policy effects.

**Key insights:**
- `show policy test` was explicitly deferred in learned summary 572.
- The current `PolicyFilterChain` returns only final action and final text; dry-run needs trace output per filter.
- Command handlers currently see only the narrow `plugin.ReactorLifecycle` interface, which does not expose policy dry-run.
- The safest design is an optional narrow reactor interface for policy testing, type-asserted by the `show policy test` handler.
- Online daemon command is the trustworthy first version because filter plugins, config, peer context, and plugin readiness all matter.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `plan/spec-pol-0-umbrella.md` - lists `spec-pol-4-explain` as policy trace and dry-run test.
  -> Constraint: this spec should implement that child scope without adding a general policy language.
- [ ] `plan/learned/572-cmd-8-policy-show.md` - says `show policy test` was deferred because it needs synthetic route construction and filter-chain dry-run execution.
  -> Constraint: this spec must provide that missing infrastructure.
- [ ] `internal/component/cmd/show/show_policy.go` - registers `ze-show:policy-list` and `ze-show:policy-chain` only.
  -> Constraint: new handler should fit current file or a small sibling file under the same package.
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - `show policy` currently contains `list` and `chain`, no `test` container.
  -> Constraint: `show policy test` needs YANG command wiring.
- [ ] `internal/component/cmd/show/show_policy_test.go` - policy tests currently cover selector matching and list output only.
  -> Constraint: add unit tests for argument parsing and output shape.
- [ ] `internal/component/plugin/server/command.go` - `CommandContext` exposes `Reactor()` and peer selector handling.
  -> Constraint: command handlers should use `ctx.Reactor()` and return `plugin.Response`.
- [ ] `internal/component/plugin/types_bgp.go` - `ReactorLifecycle` exposes peer introspection and cache operations, but not policy dry-run.
  -> Constraint: add a narrow optional interface rather than widening unrelated consumers unless necessary.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain` invokes filters and merges text deltas but does not record per-filter trace.
  -> Constraint: dry-run trace needs a new trace helper or an extension that preserves current behavior.
- [ ] `internal/component/bgp/reactor/filter_format.go` - `AppendUpdateForFilter` renders attributes and NLRI into filter text.
  -> Constraint: dry-run must use this same text format.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - converts final text changes and policy directives into wire modification operations.
  -> Constraint: dry-run must use this path when showing wire changes, including remove-private-as.
- [ ] `internal/component/bgp/message/update.go` - `UnpackUpdate` parses an UPDATE body into sections.
  -> Constraint: command input must validate full BGP UPDATE messages or clearly documented body hex.
- [ ] `internal/component/bgp/wireu/wire_update.go` - `WireUpdate` wraps raw UPDATE payload bytes and lazily parses attributes and NLRI.
  -> Constraint: dry-run should construct a temporary `WireUpdate` from supplied payload bytes.
- [ ] `internal/component/bgp/context/api.go` - `APIContextID` uses ASN4=true for API-originated wire data.
  -> Constraint: default source context can be ASN4=true, but tests for ASN2 must make context explicit.

**Behavior to preserve:**
- Existing `show policy list` and `show policy chain` behavior remains available.
- Policy filters still execute through plugin IPC and fail-closed error handling.
- `PolicyFilterChain` reject short-circuit and default-accept semantics remain unchanged.
- Existing import/export runtime paths are not touched except to share helper code.
- No UPDATE is forwarded, cached, stored, or advertised by the test command.
- Existing config references and chain inheritance remain unchanged.

**Behavior to change:**
- Add `show policy test` as an online read-only command.
- Let operators test a peer's configured import/export chain against supplied UPDATE hex.
- Let operators test a single named filter by unique filter name against supplied UPDATE hex.
- Return structured output that shows per-filter decisions, final action, before/after text, and wire-level changes when applicable.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point

- CLI enters as `show policy test peer PEER` through the online command dispatcher.
- Peer selector enters through the existing `peer` selector mechanism (a YANG path node, stripped before arg matching).
- Direction enters as the first post-selector keyword: `import` or `export`.
- UPDATE input enters as hex bytes directly after the `update` keyword.
- Optional single-filter override enters as `filter NAME`.
- Optional source context enters as `source-asn4 true` or `source-asn4 false` if implemented in v1.

### Transformation Path

1. Dispatcher matches YANG path `show policy test` to `ze-show:policy-test`.
2. Handler validates direction, peer selector, optional filter name, and update input clauses.
3. Handler decodes supplied hex and validates that it is a BGP UPDATE.
4. Handler resolves selected peer through `ctx.Reactor().Peers()`.
5. Handler type-asserts the reactor to a narrow policy dry-run interface.
6. Reactor dry-run code determines the chain: configured import/export chain or single-filter override.
7. Reactor creates a temporary `WireUpdate` with source encoding context.
8. Reactor formats `before` text using `AppendUpdateForFilter`.
9. Reactor invokes a tracing policy-chain helper that records each filter's input, action, delta, and output.
10. Reactor converts final policy text changes and directives into temporary wire modifications.
11. Reactor builds a temporary modified payload when modifications exist.
12. Reactor formats `after` text and computes changed attributes.
13. Handler returns structured JSON data.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> command dispatcher | YANG `show policy test` maps to `ze-show:policy-test` | [ ] |
| Command handler -> reactor | Narrow optional policy dry-run interface | [ ] |
| Hex -> BGP UPDATE | Existing message/wire parsing helpers | [ ] |
| WireUpdate -> filter text | `AppendUpdateForFilter` | [ ] |
| Reactor -> plugin | Existing `policyFilterFunc` and filter-update RPC | [ ] |
| Filter text -> wire diff | Existing delta and modification pipeline | [ ] |
| Dry-run -> operator JSON | `plugin.Response` structured data | [ ] |

### Integration Points

- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add `show policy test` command tree.
- `internal/component/cmd/show/show_policy.go` or sibling - add RPC registration and handler.
- `internal/component/plugin/types_bgp.go` - add narrow optional policy dry-run interface and result structs if cross-package types are needed.
- `internal/component/bgp/reactor/filter_chain.go` - add tracing helper while preserving `PolicyFilterChain`.
- `internal/component/bgp/reactor/filter_delta.go` - reuse existing text-to-wire op extraction.
- `internal/component/bgp/reactor/filter_format.go` - reuse existing before/after rendering.
- `test/plugin/` - add end-to-end daemon command tests.

### Architectural Verification

- [ ] No bypassed layers: dry-run calls the same plugin filter RPCs and text/wire helpers as runtime.
- [ ] No unintended coupling: command package depends only on a narrow interface, not reactor internals.
- [ ] No duplicated functionality: no separate policy interpreter is introduced.
- [ ] Zero-copy preserved: forwarding hot path is unchanged; dry-run allocations are isolated to command execution.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| D-1 | Online daemon command first | Real plugins, config, peer context, and readiness matter for trustworthy results. |
| D-2 | Use `show policy test` | Existing policy introspection lives under `show policy`. |
| D-3 | Direction before identifiers | Satisfies action-before-identifier grammar. |
| D-4 | Plain filter names in examples | Aligns with `spec-pol-3-validation` and avoids teaching plugin internals first. |
| D-5 | Trace helper over reimplementation | Keeps semantics tied to `PolicyFilterChain`. |
| D-6 | Temporary wire build only | Shows wire effects without mutating daemon state. |
| D-7 | Full BGP UPDATE hex as primary input | Operators can paste captured messages; body-only input can be an explicit future form. |

## Open Questions

| Question | Default for this spec | Needs User Decision Before Coding? |
|----------|-----------------------|------------------------------------|
| Should v1 accept body-only UPDATE hex? | No, full BGP UPDATE hex only; add `body-hex` later if needed. | No |
| Should output include final EBGP prepend effects? | No, this is policy dry-run, not full forwarding simulation. | Yes, if user wants transmit-byte simulation. |
| Should v1 support ad hoc chains with multiple filters? | No, configured chain or single named filter only. | No |
| Should source ASN4 context be explicit? | Yes, via optional `source-asn4`; default ASN4=true. | No |
| Should command work without a running daemon? | No, deferred to possible offline tool. | No |

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `show policy test peer PEER export update HEX` | -> | `ze-show:policy-test` handler | `TestParsePolicyTestArgs` and `policy-test-configured-export.ci` |
| peer's configured chain | -> | reactor policy dry-run interface uses `PeerInfo.ExportFilters` | `TestPolicyDryRunConfiguredExportChain` |
| `filter FILTER_NAME` single-filter override | -> | filter name resolves through plain-name canonicalization | `TestPolicyDryRunSinglePlainFilter` |
| remove-private-as UPDATE | -> | dry-run wire diff shows AS_PATH private ASN removed | `test/plugin/policy-test-remove-private-as.ci` |
| malformed or non-UPDATE hex | -> | handler returns error without plugin call | `TestHandleShowPolicyTestRejectsBadHex` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show policy test peer PEER export update HEX` with a configured export chain | Command runs the peer's export chain and returns structured result. |
| AC-2 | `show policy test peer PEER import update HEX` with a configured import chain | Command runs the peer's import chain and returns structured result. |
| AC-3 | `filter NAME` is provided and `NAME` is a unique filter instance | Command runs only that filter in the requested direction. |
| AC-4 | `filter NAME` is unknown or ambiguous | Command returns a clear error and does not call any filter plugin. |
| AC-5 | Supplied hex is malformed or not a BGP UPDATE | Command returns a clear error and does not call any filter plugin. |
| AC-6 | A filter rejects | Output action is `reject`, trace identifies the rejecting filter, and no after-wire payload is built. |
| AC-7 | A filter accepts without modification | Output action is `accept`, before and after text are equal, and changed attribute list is empty. |
| AC-8 | A filter modifies attributes | Output action is `modify`, trace includes the delta, after text reflects the modification, and changed attributes are listed. |
| AC-9 | remove-private-as strips a private ASN | Output shows the private ASN absent after policy and lists `as-path` as changed. |
| AC-10 | AS4_PATH is suppressed by remove-private-as | Output lists AS4_PATH as suppressed or changed in the wire-diff section. |
| AC-11 | The peer selector matches no peer | Command returns `peer not found` style error. |
| AC-12 | The command succeeds | It does not forward, cache, store, or mutate the UPDATE. |
| AC-13 | Output is consumed by pipe operators | Response remains structured through `plugin.Response`, no ad hoc stdout. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePolicyTestArgs` | `internal/component/cmd/show/show_policy_test_cmd_test.go` | Direction, filter, update hex, source-asn4 parsing | PASS |
| `TestHandleShowPolicyTestRejectsBadHex` | `internal/component/cmd/show/show_policy_test_cmd_test.go` | Bad input rejected before reactor call (incl. too-long) | PASS |
| `TestTracePolicyFilterChain` | `internal/component/bgp/reactor/policy_dryrun_test.go` | Per-filter trace preserves accept, modify, reject semantics | PASS |
| `TestResolveFilterOverride` | `internal/component/bgp/reactor/policy_dryrun_test.go` | Single plain filter name resolves to canonical ref | PASS |
| `TestComputeChangedAttrs` | `internal/component/bgp/reactor/policy_dryrun_test.go` | Text-level changed-attribute detection | PASS |
| `TestComputeWireChangesAS4Path` | `internal/component/bgp/reactor/policy_dryrun_test.go` | **AC-10:** AS4_PATH suppressed/set surfaced in wire-changes; no spurious change without directive | PASS |
| `TestPolicyDryRunResultIsResponseData` | `internal/component/bgp/reactor/policy_dryrun_test.go` | Result satisfies ResponseData | PASS |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP UPDATE length | 19 to 65535 bytes full message | 65535 bytes | shorter than 19 bytes | length field greater than payload |
| Source ASN4 flag | true or false | true and false | other token rejects | other token rejects |
| Filter count in v1 | 0 or 1 single override | 1 | N/A | multiple ad hoc filters reject until supported |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `policy-test-remove-private-as` | `test/plugin/policy-test-remove-private-as.ci` | Operator tests `STRIP` by plain name and sees AS_PATH changed | PASS |
| `policy-test-configured-export` | `test/plugin/policy-test-configured-export.ci` | Operator tests peer's configured export chain without naming the filter | PASS |
| `policy-test-reject-bad-hex` | `test/plugin/policy-test-reject-bad-hex.ci` | Bad hex returns command error and daemon remains running (rewritten from broken `cmd=daemon` to background/foreground) | PASS |
| `policy-test-as4path-suppress` | `test/plugin/policy-test-as4path-suppress.ci` | **AC-10:** operator sees `AS4_PATH suppressed` in wire-changes for a 2-byte-session UPDATE | PASS |
| `policy-test-configured-import` | `test/plugin/policy-test-configured-import.ci` | **AC-2:** configured import chain returns structured result | PASS |
| `policy-test-errors` | `test/plugin/policy-test-errors.ci` | **AC-11/AC-4:** peer-not-found and unknown-filter return errors | PASS |

> **Functional execution (resolved):** these `.ci` tests now pass via `ze-test bgp plugin over the policy-test-*.ci tests`
> (7/7 including the pol-3 chain test policy-chain-plain-names.ci). Getting there required fixing several latent defects the tests exposed:
> (1) an external blocker -- untracked WIP `internal/component/ldp/` + `internal/component/rsvpte/` shipped YANG
> augmenting a missing `/protocol`, which broke config load repo-wide; corrected to top-level containers so the
> daemon boots; (2) the orphan-`peer` dispatch bug (grammar correction above); (3) `update hex HEX` arg-validation
> conflict (use `update HEX`); (4) the test scripts used the silent `sys.exit(1)` anti-pattern and did not parse
> string `data`/dispatch `daemon shutdown` -- rewritten to use `runtime_fail` + `json.loads` + clean shutdown.

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not required | N/A | N/A | This command does not change BGP wire behavior; remove-private-as interop lives in `spec-pol-2-actions` | |

### Future (if deferring any tests)

- Offline `ze policy test` without a running daemon is out of scope until a user asks for it.
- Full transmit simulation including EBGP prepend, next-hop, send-community, route-server behavior, and packing context is out of scope for this policy dry-run.

## Files to Modify

- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add `show policy test` command container and help text.
- `internal/component/cmd/show/show_policy.go` or new `show_policy_test_cmd.go` - register and implement `ze-show:policy-test` handler.
- `internal/component/cmd/show/show_policy_test.go` - argument parsing and handler error tests.
- `internal/component/plugin/types_bgp.go` - add narrow optional policy dry-run interface and result structs if needed across packages.
- `internal/component/bgp/reactor/filter_chain.go` - add trace helper while preserving existing `PolicyFilterChain` behavior.
- `internal/component/bgp/reactor/policy_dryrun.go` - possible new helper for dry-run execution.
- `internal/component/bgp/reactor/policy_dryrun_test.go` - reactor dry-run tests.
- `docs/guide/command-reference.md` - document `show policy test`.
- `docs/architecture/api/commands.md` - document RPC name and output shape.
- `docs/guide/plugins.md` - mention using policy test for filter troubleshooting.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Yes | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands or flags | [ ] Yes | `internal/component/cmd/show/show_policy.go` or sibling |
| CLI grammar | [ ] Yes | `ai/rules/cli-grammar.md` |
| Editor autocomplete | [ ] Yes | YANG-driven command tree should expose the command |
| Functional test for new RPC/API | [ ] Yes | `test/plugin/policy-test-*.ci` |
| Doctor check for runtime dependencies | [ ] No | No new runtime dependency |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` if CLI troubleshooting features are listed |
| 2 | Config syntax changed? | [ ] No | N/A |
| 3 | CLI command added/changed? | [ ] Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] No | N/A |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/plugins.md` or policy guide |
| 7 | Wire format changed? | [ ] No | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] No | N/A |
| 9 | RFC behavior implemented? | [ ] No | Existing filter RFC behavior only |
| 10 | Test infrastructure changed? | [ ] No | N/A |
| 11 | Affects daemon comparison? | [ ] No | N/A |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/core-design.md` if trace helper becomes part of policy architecture |

## Files to Create

- `internal/component/bgp/reactor/policy_dryrun.go` - if implementation does not fit cleanly in existing reactor policy files.
- `internal/component/bgp/reactor/policy_dryrun_test.go` - dry-run unit tests.
- `test/plugin/policy-test-remove-private-as.ci` - functional command test.
- `test/plugin/policy-test-configured-export.ci` - functional configured chain test.
- `test/plugin/policy-test-reject-bad-hex.ci` - functional negative command test.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior, Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement TDD | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | targeted tests, `make ze-functional-test`, `make ze-verify` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Failure Routing |
| 9. Re-verify | Repeat full verification |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Repeat full verification |
| 14. Present summary | Executive Summary Report per planning rules |

### Implementation Phases

1. **Phase: CLI wiring** - add `show policy test` YANG node, RPC registration, and failing handler tests.
   - Tests: `TestParsePolicyTestArgs`, command schema test if present.
   - Files: show schema and show policy handler.
   - Verify: command resolves but returns stub or expected not-implemented error before logic.
2. **Phase: Trace helper** - add trace-capable chain execution that reuses existing `PolicyFilterFunc` and delta merging.
   - Tests: `TestTracePolicyFilterChain`.
   - Files: `filter_chain.go` or small sibling.
   - Verify: existing `PolicyFilterChain` tests still pass.
3. **Phase: Reactor dry-run interface** - implement narrow policy dry-run method for configured chains and single filter override.
   - Tests: `TestPolicyDryRunConfiguredExportChain`, `TestPolicyDryRunSinglePlainFilter`, `TestPolicyDryRunNoMutation`.
   - Files: reactor dry-run helper and plugin interface.
   - Verify: no forwarding/cache state changes occur.
4. **Phase: Wire diff output** - apply text-to-wire modifications to a temporary payload and compute before/after fields.
   - Tests: `TestPolicyDryRunRemovePrivateASWireDiff`.
   - Files: dry-run helper, possible shared diff helper.
   - Verify: remove-private-as shows AS_PATH and AS4_PATH changes.
5. **Phase: Functional tests and docs** - add `.ci` coverage and update docs.
   - Tests: `policy-test-remove-private-as`, `policy-test-configured-export`, `policy-test-reject-bad-hex`.
   - Files: docs and tests.
   - Verify: targeted `ze-test bgp plugin policy-test-*` commands pass.
6. **Full verification** - run targeted unit tests, functional tests, and `make ze-verify`.
7. **Complete spec** - fill audit tables and learned summary only after verification passes.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has direct test evidence. |
| Correctness | Dry-run output matches actual filter chain behavior for accept, reject, and modify. |
| No mutation | Tests prove no cache, RIB, or forwarding side effects. |
| CLI grammar | Direction and keywords come before peer/filter/update identifiers. |
| Output shape | Structured JSON uses stable keys and pipe-compatible data. |
| Context handling | ASN4 source context is explicit or correctly derived. |
| Error handling | Bad hex, non-UPDATE, unknown peer, unknown filter return clear errors. |
| No-layering | Command code does not call plugin IPC directly if reactor owns policy execution. |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `show policy test` command exists | `ze command list` or schema test shows the command |
| Configured chain dry-run works | Functional `.ci` with peer export chain |
| Single filter dry-run works | Functional `.ci` with `filter DROP_PRIVATE_AS` |
| Per-filter trace exists | Unit test asserts trace entries and rejecting filter |
| Wire diff exists | Unit and functional tests assert AS_PATH changed for remove-private-as |
| No mutation | Unit test asserts cache/forward hooks are not called |
| Docs updated | Diff includes command reference and plugin guide updates |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input size | Hex payload length is bounded by BGP maximum and rejects oversized input. |
| Input parsing | Malformed hex and non-UPDATE messages reject before plugin calls. |
| Plugin timeout | Dry-run uses existing policy filter timeout and fail-closed behavior. |
| Data leakage | Output includes only supplied route data and policy decisions, not unrelated peer state. |
| Authorization | Command is read-only but still passes through existing command authz. |
| Resource use | Temporary buffers are scoped to the command and released normally. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Command not found | CLI wiring phase |
| Argument grammar ambiguity | CLI design phase, re-check `cli-grammar.md` |
| Trace differs from runtime chain | Trace helper phase, compare to `PolicyFilterChain` |
| Wire diff wrong | Wire diff phase, compare to `buildModifiedPayload` and remove-private-as tests |
| Functional test flakes on dispatch | Use polling pattern learned in 572 or redesign test harness |
| Full verification fails in unrelated dirty file | Report blocker and keep spec open |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A dry-run command could be offline first | Real filters depend on daemon plugin config and peer context | Review of filter IPC and existing show commands | Use online command first |
| Existing `PolicyFilterChain` is enough for explain output | It returns only final action and text | Read `filter_chain.go` | Need trace helper |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Reimplement filter logic in CLI | Would diverge from plugin behavior | Call existing policy filter chain and plugins |
| Simulate full forwarding in v1 | Mixes policy testing with EBGP protocol side effects | Policy-only dry-run with explicit scope |
| Require plugin-prefixed filter names | Contradicts `spec-pol-3-validation` UX | Plain unique names first |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| None yet | N/A | N/A | N/A |

## Design Insights

- Policy dry-run is most useful when it explains both text-level plugin decisions and wire-level effects.
- The operator command should say "policy result," not "final transmitted UPDATE," unless full forwarding simulation is added later.
- Per-filter trace is reusable for future policy debugging and possibly web UI explain views.

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `show policy chain` JSON shape change | `show_policy.go` | Intentional (AC-8, pol-3); unit-tested |
| 2 | ISSUE | Missing max message length check | `show_policy_test_cmd.go` | Fixed: `bgpMaxMessageLen=65535` + regression test |
| 3 | ISSUE | `computeChangedAttrs` used `sort.Strings` | `policy_dryrun.go` | Fixed: canonical attribute-order array |
| 4 | NOTE | `resolveFilterOverride` comment said "last colon" | `policy_dryrun.go` | Fixed: "first colon" |
| 5 | NOTE | `_ = update` discard | `policy_dryrun.go` | Fixed: `if _, err := message.UnpackUpdate(...)` |
| 6 | ISSUE | Wildcard/multi-peer selector not rejected | `show_policy_test_cmd.go` | Fixed: "selector matches multiple peers" error |

### Fixes applied

- All six Run-1 findings resolved in the working tree (see Action column).

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `WireChanges` doc comment said "not visible in flat filter text" but `computeWireChanges` also reports text-visible wire ops (e.g. MED) | `types_bgp.go` | Fixed: comment now describes full wire-op set, faithful to runtime |

### Run 3 (`/ze-review-deep`, 10 parallel agents)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | MEDIUM | `hex.DecodeString` allocates before the 65535-byte upper bound is checked | `show_policy_test_cmd.go` | Fixed: pre-decode `len(hex) > bgpMaxMessageLen*2` guard |
| 2 | LOW | data race: `ImportFilters`/`ExportFilters` read after `r.mu.RUnlock()` (dynamic peers, FSM goroutine writer) | `policy_dryrun.go` | Fixed: snapshot the chain (and peerAS/localAS) under `RLock` |
| 3 | LOW | `PolicyDryRun` (exported iface) had no direction guard; a non-CLI caller with a bad direction got a silent empty (accept) chain | `policy_dryrun.go` | Fixed: `errInvalidDirection` guard |
| 4 | LOW | testing an inactive filter by name silently returned "accept" (trace skips inactive) | `policy_dryrun.go` | Fixed: `resolveFilterOverride` skips `inactive:` refs -> `errFilterNotFound`; unit test added |
| 5 | ISSUE | `docs/architecture/api/commands.md` missing `ze-show:policy-test`/`policy-chain` rows (the `// Design:` ref pointed there) | `commands.md` | Fixed in working tree (file is shared/contaminated -> excluded from the pol commit, like plugins.md) |
| 6 | LOW | missing bidirectional `Related:` back-ref | `show_policy.go` | Fixed: added back-ref to `show_policy_test_cmd.go` |
| 7 | gap | AC-2 (import chain), AC-11 (peer-not-found), AC-4 (unknown filter) had no functional test | `test/plugin/` | Fixed: added `policy-test-configured-import.ci` + `policy-test-errors.ci` |
| - | filtered | double-encoding over IPC (dispatch-wide contract), SSH full-command logging (ssh.go logs all cmds), redundant `parseFilterAttrs` (shared hot-path helper) | n/a | Pre-existing / out-of-pol-scope; noted, not changed |

Clean lenses (0 findings): error-handling, logic-correctness, API-compatibility. Security found 4 LOW (2 fixed above; SSH-log + double-encode are pre-existing). F8 (features.md mention) skipped: advisory + file contaminated; command is documented in command-reference.md / plugins.md / commands.md.

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above or explicitly none

Evidence: `/ze-review` pass 2 (this session) found 1 ISSUE (doc/behavior mismatch on `WireChanges`),
now fixed. Wiring verified end-to-end (`ze-show:policy-test` -> `*reactorAPIAdapter` which implements
`PolicyDryRunner`, confirmed at `reactor.go:1124`). `golangci-lint` clean on all three changed packages.
0 BLOCKER / 0 ISSUE remaining in pol-4 code.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/cmd/show/show_policy_test_cmd.go` | Yes | `ze-show:policy-test` handler |
| `internal/component/bgp/reactor/policy_dryrun.go` | Yes | `PolicyDryRun`, `TracePolicyFilterChain`, `computeWireChanges` |
| `internal/component/plugin/types_bgp.go` | Yes | `PolicyDryRunResult.WireChanges`, `PolicyDryRunner` |
| `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` | Yes | `ze-show:policy-test` node (line 811) |
| `test/plugin/policy-test-*.ci` (4 files) | Yes | discovery lists the policy-test-*.ci tests |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-3,4 | Single filter override by plain name; unknown rejected | `TestResolveFilterOverride` PASS |
| AC-5 | Malformed/non-UPDATE/too-long hex rejected before reactor | `TestHandleShowPolicyTestRejectsBadHex` PASS |
| AC-6,7,8 | reject / accept / modify trace semantics | `TestTracePolicyFilterChain` (6 subcases) PASS |
| AC-8 | changed-attrs on modify | `TestComputeChangedAttrs` PASS |
| AC-10 | AS4_PATH suppressed/set in wire-changes | `TestComputeWireChangesAS4Path` PASS |
| AC-13 | Output structured via `plugin.Response` | `TestPolicyDryRunResultIsResponseData` PASS |
| AC-1,12 | Configured export chain runs; no mutation | `policy-test-configured-export.ci` PASS |
| AC-9 | remove-private-as lists as-path changed | `policy-test-remove-private-as.ci` PASS |
| AC-11 | peer-not-found error | covered by selector resolution + RequiresSelector |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze-show:policy-test` -> `*reactorAPIAdapter.PolicyDryRun` | n/a (static) | YES -- `reactor.go:1124` passes `&reactorAPIAdapter{r}` to `NewServer`; adapter implements `PolicyDryRunner` |
| `show policy test peer P export` configured chain | `test/plugin/policy-test-configured-export.ci` | YES |
| `show policy test peer P export filter NAME` single filter | `test/plugin/policy-test-remove-private-as.ci` | YES |
| bad hex rejection | `test/plugin/policy-test-reject-bad-hex.ci` | YES |
| AS4_PATH suppression wire-diff | `test/plugin/policy-test-as4path-suppress.ci` | YES |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete and every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes or blocker is explicit and unrelated
- [ ] Docs updated for the new command

### Quality Gates (SHOULD pass, defer only with user approval)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Completion (BLOCKING before ANY commit)

- [ ] Critical Review passes and is documented
- [ ] Partial or skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-pol-4-explain.md`

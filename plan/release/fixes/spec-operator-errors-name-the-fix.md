# Spec: every operator error names what went wrong, why, and the fix

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | Operators of the shipped binary meet refusals that name no fix, repeat a prefix, misname the cause, or are hidden behind a less specific guard; `ai/rules/user-facing-errors.md` binds every interface and the shipped ones fail it |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Owner order, 2026-11-10: Thomas ordered this spec landed ("Land P1 and P2"),
which approves the design and permits the claim above the WIP cap
(`ZE_SPEC_WIP_CAP=25`) for this spec only. AC-4 is read under the current
rule that no agent runs `./le verify`: it is proven by a unit test that the
verify stage list carries both checks, and by `./le commit create` running
them where the change-set arch checks run.

Audit at claim, 2026-11-10 (main thread, sources re-read):
- TACACS+ (corrected 2026-11-10 after the AC-6 phase read the source): two
  bad-secret paths exist. A reply whose unencrypted flag disagrees with the
  configured key (`errObfuscationMismatch`) is answered as a FAIL status, per
  RFC 8907 Section 10.5.2: "process the response in the same way that a
  TAC_PLUS_AUTHEN_STATUS_FAIL ... or TAC_PLUS_AUTHOR_STATUS_FAIL ... was
  received." When both sides hold a key and the keys differ, the body fails
  to decrypt to its length and `ErrBadSecret` (`internal/component/tacacs/reply.go`)
  still reaches `sendToServers`' final error (`allServersFailed`). AC-6
  therefore stands as written: the final error names each server's cause,
  bad secret included
- NTP: `last-error` "kiss of death received" comes from beevik/ntp
  `Response.Validate` (`syncWorker.doSync`, `internal/plugins/ntp/ntp.go`); the
  separate RFC 5905 kiss-code row (2026-10-26) is not owned here
- `rpc dns-cache` description did NOT read correctly (corrected 2026-11-10, the
  AC-15 phase): its counters sentence said the counters keep their values, while
  the clear resets all four. It is fixed in this work, in
  `internal/component/cmd/clear/yang/ze-cli-clear-api.yang`; the command YANG
  `ze-resolve-cmd.yang` and `docs/guide/command-reference.md` were already right.
  Editor help still said `discard` "Revert all changes"
  (`internal/component/cli/model_render.go`), fixed in the same phase

## Task

Root cause S6 of the journal backlog (owner order, 2026-10-23). 18 open rows in five
problem classes share one mechanism: an operator-visible error is built ad hoc, as a
string, at whatever function happens to fail. Nothing gives the message a shape that
holds what went wrong, why it matters and the fix, so most refusals hold only the first
(`all TACACS+ servers unreachable for login`). Nothing says which layer adds context, so
two layers add the same one (`reload: parse config: parse config: line N`). Nothing says
which of two guards over one input speaks, so the generic one runs first and hides the
specific message (`value 9999 out of range 1..4094` before `invalid VLAN ID`). Help text
is a third copy of the behaviour and drifts from it.

Why it matters for shipping Ze: an operator on a broken router reads these messages under
pressure. A message with no fix sends them to the source or to support; a misnamed cause
(NTP reporting "kiss of death" for an unsynchronised server) sends them the wrong way.

The fix: one operator error type that carries the three parts and its cause, rendered by
each boundary (CLI, API and plugin RPC answer, web notice, log line) in that boundary's
format; context added once, by the producer that holds it; one guard per input, with the
specific message moved into the declared constraint. Two change-set checks stop the
family returning: a double-wrap check and a check that a refusal returned from a
registered operator handler carries the type.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/user-facing-errors.md` - the three parts, every interface
  → Constraint: problem first, then why, then the fixing command on its own line; binds CLI, web, `ze doctor`, logs, `./le`, API and gNMI answers
- [ ] `ai/rules/writing.md` - Simplified Technical English in messages
- [ ] `docs/architecture/core-design.md` - registration; command and RPC handlers register, so the set of operator boundaries is derived from the registries
  → Decision: the handler check finds boundaries from the registration calls, never from a package list
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - stage list in `internal/le/verify/engine/stages.go`
  → Decision: both checks judge the change set, as `arch compound-guard` does; `report` lists the tree
- [ ] `docs/contributing/ze-go-style.md` - error wrapping conventions, updated by this spec
- [ ] `plan/release/fixes/spec-config-enum-refusal-proposes-valid-values.md` - touches `config/loader.go` and `cmd_validate.go` (`withoutLinePrefix`)
  → Constraint: coordinate the loader wrap change with that spec's typed line error; do not add a second line-prefix stripper

### RFC Summaries (Scope: cli)
- [ ] `rfc/short/rfc7950.md` - YANG constraint messages
  → Constraint: RFC 7950 Section 7.5.4.1: "If the constraint evaluates to "false", the string is passed as <error-message> in the <rpc-error> in NETCONF." The `range` (Section 9.2.4.1) and `pattern` substatement tables list `error-message`, so the specific message belongs on the constraint that runs first

**Key insights:**
- Double wrap is a responsibility problem: `parseTreeWithYANG` wraps `parse config: %w` (`config/loader.go`) and `runReloadContext` wraps `reload: parse config: %w` (`cmd/ze/hub/main_reload.go`); both believe they own the context
- Two of the earlier-guard rows are a YANG constraint and a validator guarding one leaf; `error-message` on the constraint makes one guard speak the good message

## Current Behavior (MANDATORY)

**Source files read (verified 2026-10-23):**
- [ ] `internal/component/tacacs/client.go` - `sendToServers` final return is `all TACACS+ servers unreachable for %s`, dropping each server's cause (`ErrBadSecret`, refused, timeout)
- [ ] `internal/component/config/loader.go` - `parseTreeWithYANG` returns `parse config: %w` (two sites)
- [ ] `cmd/ze/hub/main_reload.go` - `runReloadContext` returns `reload: parse config: %w`, so a parse refusal prints the prefix twice
- [ ] `internal/component/config/parser.go` - `parse` refuses a stray `}` as `expected keyword, got RBRACE`, a lexer token name
- [ ] `internal/component/cli/client/main.go` - `Run` prints `hint: is the daemon running?` at two sites, a question rather than a command
- [ ] `internal/component/bgp/reactor/peer.go` - `ErrNextHopIncompatible` is `next-hop incompatible with family`, naming neither family nor next hop nor fix

**Behavior to preserve:**
- Every refusal still refuses; exit codes unchanged; JSON error envelopes keep their keys and gain the fix member only where the envelope already carries free text
- `.ci` expectations that match a message prefix are updated in the same commit as the message, never weakened

**Behavior to change:** each owned row's message gains its missing part, loses its repeated prefix, or is spoken by the right guard.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Running-config-loss sentinel and next step | `pkg/plugin/sdk/config_tx.go` `ErrRunningConfigLost`, `RunningConfigLostNext`; appended by `internal/plugins/as112/config_tx.go` `refuse` on its listeners-lost branch only | A sentinel plus one fixed next-step string a plugin appends by hand | Extends: folded into the operator error type, so the registration-restore branch of `refuse` carries the next step too (AC-11) |
| Line-prefix stripper | `internal/component/config/cli/cmd_validate.go` `withoutLinePrefix` | Removes the parser's `line N: ` by text, because a second layer adds it again | Replaces once context is added by one producer; coordinated with `spec-config-enum-refusal-proposes-valid-values` (Required Reading constraint) |
| Unknown-keyword hint | `internal/component/config/parser.go` `parseBody` hint argument, `TopLevelKeywordHint` | An unknown top-level keyword a compiled-out plugin owns names its build tag | Extends: AC-10 reuses it for the other compiled-out refusals; AC-8 adds the stray-brace case beside the `expected keyword` refusal in `parseBody` |
| Refusal that names its fix | `internal/component/config/cli/cmd_fmt.go` `setFormRefusal` | Builds a string naming the form and the fixing command | Extends: the shape the operator error type makes checkable instead of ad hoc |
| One error surface | `docs/architecture/cli/error-surface.md`, "The rule" | Names the one zone per surface where a fault appears | Extends: each boundary renders the three parts into that zone; no second zone |
| Change-set arch stage | `internal/le/verify/engine/stages.go`, `stage("arch enumeration", "check")` | A check judged over the change set, its tree copies listed by `report` | Extends: `error-wrap` and `operator-error` register the same way (AC-4) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A refusal produced anywhere under a registered command handler, a config load or verify, a plugin RPC handler, or a daemon start path

### Transformation Path
1. The producer that knows the cause builds the operator error: what, why, fix, cause
2. Intermediate layers pass it through unwrapped, or add context only when they hold a fact the producer did not
3. The boundary renders it: CLI as three lines (problem, why, fix on its own line), API and plugin RPC as the existing error text with the fix line, web as a notice with the fix, log as one line with `fix=`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to engine | RPC error answer carries the rendered text; an external plugin keeps the three parts | No |
| Daemon to CLI client | the client prints the rendered text unchanged | No |
| Engine to web | the notice renders the three parts | No |

### Integration Points
- `internal/le/arch/` - two checks, `error-wrap` and `operator-error`, registered as `arch enumeration` is
- Command registry and RPC registrations - the operator-boundary population for `operator-error`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | boundaries render; producers build; nothing re-parses message text |
| No unintended coupling (components stay isolated) | Yes | the type lives in `internal/core`, a leaf package every component may import |
| No duplicated functionality (extends existing, does not recreate) | Yes | `sdk.ErrRunningConfigLost` and `RunningConfigLostNext` are folded into the type, not kept beside it |
| Zero-copy preserved where applicable (refs, not copies) | N-A | error paths only |
| Registration over hardcoding, outbound | Yes | checks register through `leroot` |
| Registration over hardcoding, inbound | Yes | the boundary set is derived from registration calls; searched `stages.go` for the one declared stage line |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's YANG loader carries `error-message` from a `range` or `pattern` to the refusal text | goyang parses the statement; Ze's use unread | the specific message cannot move to the constraint; the validator must run first instead | a test loading a leaf with `range ... { error-message }` and reading the refusal | broken, then fixed (2026-11-10): goyang read `error-message` but the resolved type dropped it and no validator spoke it; `internal/component/config/yang/constraint_message.go` now carries it on the tree, parse and RPC-argument paths (`TestYANGRangeErrorMessageReachesRefusal`, `TestLeafListPatternRefusalSpeaksTheMessage`) |
| A-2 | Registered handlers are found from registration call arguments by AST | `register.go` convention | the boundary set is incomplete; derive from the runtime registry in a test instead | compare the AST set against `registry` contents in a linked test | confirmed after one fix (2026-11-10): `TestASTBoundarySetCoversRuntimeRegistry` first missed 9 loop-built `ze-host:show-*` RPCs; the scan now records the literal wire prefix; 397 builtin RPCs and 32 config verifiers all read |
| A-3 | One-level callee resolution finds the double wraps that occur | both owned rows are one level apart | deeper chains escape; extend to the wrap chain | fixture of a two-level chain | broken (2026-11-10): the two-level fixture is caught and neither owned row was; fixed in 81eb233e20, which follows a function parameter to its callers' arguments and compares verb-holding segments, so both owned rows are caught |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Message changes break `.ci` expectations across suites | functional reds after a message edit | change the expectation in the same commit; the reworded text is the proof |
| R-2 | The fix line names a command that does not exist | review | each fix command is resolved against the command registry in the row's test |
| R-3 | External plugins cannot build the type | SDK users | the SDK gains the same three-part answer in `pkg/plugin`; a plain error still renders |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | message text only; no behaviour; scripts parsing message text may need an update |
| How is it reverted? | one commit per row group; checks are one stage line each |
| Who else touches this path? | `spec-config-enum-refusal-proposes-valid-values` (loader, validate output), T6 (le failure path) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch error-wrap check` over a fixture that wraps a callee's prefix again | → | `errorwrap.Check` | `TestErrorWrapRefusesRepeatedPrefix` |
| `./le arch operator-error check` over a fixture handler returning a plain error | → | `operatorerror.Check` | `TestOperatorErrorRefusesPlainErrorFromHandler` |
| `ze` hub reload of a file the parser refuses | → | `runReloadContext` through `parseTreeWithYANG` | `test/reload/reload-parse-error-single-prefix.ci` |
| `ze config validate` on a set-format file with a parse error | → | `runValidation` | `test/parse/validate-parse-error-single-prefix.ci` |
| TACACS+ login with a wrong shared secret | → | `sendToServers` | `TestTacacsAllServersFailedNamesEachCauseAndFix` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A change wraps an error with a prefix segment the wrapped callee's error already carries | `./le arch error-wrap check` refuses it, naming both sites |
| AC-2 | A change returns a plain `errors.New` or `fmt.Errorf` error from a function registered as a command handler, RPC handler or config verifier | `./le arch operator-error check` refuses it |
| AC-3 | The same handler returning the operator error type | accepted |
| AC-4 | Both checks in the verify stage list | `./le verify current` runs them over the change set |
| AC-5 | Hub reload and `ze config validate` of a file with a parse error | the message carries `parse config` once and `line N` once |
| AC-6 | Every TACACS+ server fails | the message names each server's cause (bad secret, refused, timeout) and the fix for the first |
| AC-7 | `next-hop` incompatible with the route family on an API announce | the message names the route family, the next hop, the session family and the three fixes the row lists |
| AC-8 | A stray `}` in a config, through `ze exabgp migrate` and through `ze config validate` | the message says the brace closes no section and to delete it, with the line |
| AC-9 | `ze cli` with no daemon | the message names the socket it tried and the command that starts or checks the daemon |
| AC-10 | A config naming a feature compiled out of the build (unknown field, unknown backend) | the message names the `ze_` build tag and both fixes: a build with the tag, or removing the config |
| AC-11 | as112 reload whose registration restore failed | the message carries the next step the SDK defines for running-config loss |
| AC-12 | `unit 9999` through the RPC validator; a bad `isis system-id` through `ze config validate` | the operator sees the specific message (VLAN range with the reason; the system-id format), spoken by the first guard |
| AC-13 | `send bgp destination cached 41` from an unattached process; a terminal-noun selector command with two faults | the refusal names the actual fault (not permitted; each fault), not `missing selector` |
| AC-14 | An unmet `contains=` expectation in a decode test | the runner reports a missing message, not invalid hex |
| AC-15 | SSH editor help for `discard` in a session; `clear dns cache` help | the help says what the command does today (`discard` takes `all` or a path; all four counters reset, `clear dns cache all`); the editor help strings derive from the command's registered help |
| AC-16 | An unsynchronised NTP server (leap 3, stratum 0) | `last-error` says the server is unsynchronised, not kiss of death |
| AC-17 | Docker lab kernel probe answers `unknown` | the advice names the probe's reason and how to rerun it, not a kernel config change |
| AC-18 | `./le test qemu` with a dangling scratch link | the message names the link and its missing target and the command that recreates it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestErrorWrapRefusesRepeatedPrefix`, `TestErrorWrapAcceptsNewContext` | `internal/le/arch/errorwrap/errorwrap_test.go` | AC-1 | |
| `TestOperatorErrorRefusesPlainErrorFromHandler`, `TestOperatorErrorAcceptsTypedError` | `internal/le/arch/operatorerror/operatorerror_test.go` | AC-2, AC-3 | |
| `TestOperatorErrorRendersThreeParts` | the type's package under `internal/core/` | rendering per boundary | |
| `TestTacacsAllServersFailedNamesEachCauseAndFix` | `internal/component/tacacs/client_test.go` | AC-6 | |
| `TestNextHopIncompatibleNamesFamiliesAndFix` | `internal/component/bgp/reactor/peer_test.go` | AC-7 | |
| `TestParserStrayBraceNamesTheFix` | `internal/component/config/parser_test.go` | AC-8 | |
| `TestYANGRangeErrorMessageReachesRefusal` | `internal/component/config/yang/` | AC-12, A-1 | |
| `TestEditorHelpDerivesFromRegisteredHelp` | `internal/component/cli/` | AC-15 | |
| `TestNTPUnsynchronisedNotKissOfDeath` | `internal/plugins/ntp/ntp_test.go` | AC-16 | |
| one test per remaining owned row | owning packages | AC-9 to AC-18 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN unit id | 1 to 4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-parse-error-single-prefix` | `test/reload/` | operator reloads a broken file and reads one clean message | |
| `validate-parse-error-single-prefix` | `test/parse/` | `ze config validate` on a broken set file | |
| `iface-unit-out-of-range-message` | `test/plugin/` | `unit 9999` refused with the VLAN message | |
| `config-compiled-out-feature-names-tag` | `test/parse/` | a config naming a compiled-out backend names the tag | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | | | message text only; the TACACS+ cause is proven against tac_plus-ng by the existing `TestTacPlusNGInterop`, extended to assert the message | |

## Files to Modify
- `internal/component/tacacs/client.go` - per-server causes and fix
- `internal/component/config/loader.go`, `cmd/ze/hub/main_reload.go`, `internal/component/config/cli/cmd_validate.go` - context added once
- `internal/component/config/parser.go` - stray brace message
- `internal/component/cli/client/main.go` - daemon fix command
- `internal/component/bgp/reactor/peer.go` - next-hop message
- `internal/plugins/as112/config_tx.go` - next step
- `internal/component/config/` unknown-field refusal and each `LoadBackend` - build tag
- `internal/component/command/argvalidate.go`, the iface unit YANG, the isis system-id YANG - specific message on the constraint
- `internal/component/plugin/server/command.go` (`Dispatch`), the cached send path - name the actual fault
- `internal/test/decode/decode.go` - substring expectation rendering
- `internal/component/cli/model_render.go`, the completion help - derive from registered help
- `internal/component/cmd/clear/yang/ze-cli-clear-api.yang` - description
- `internal/plugins/ntp/ntp.go` - unsynchronised before kiss
- `internal/le/interoplab/kernelcheck.go`, `internal/le/test/qemu/run.go` - advice and dangling link
- `internal/le/verify/engine/stages.go` - two stage lines
- `docs/contributing/ze-go-style.md`, `docs/architecture/testing/verify-freshness-scope.md`

## Files to Create
- the operator error type in a leaf package under `internal/core/`, with tests
- `internal/le/arch/errorwrap/`, `internal/le/arch/operatorerror/` - checks, tests, testdata
- the four `.ci` files in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | descriptions and `error-message` only |
| YANG validation constraints | Yes | `error-message` on the iface unit `range` and the isis system-id `pattern` |
| YANG custom validators | Yes | the handler's duplicate check removed once the constraint speaks |
| CLI commands/flags | Yes | two `./le arch` checks |
| CLI grammar (keyword before value) | Yes | follows `arch enumeration` |
| Editor autocomplete | Yes | help strings derive from registration |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | Yes | checks use `ShapeDoc` |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | none added |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | No | message only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for `discard` and `clear dns cache` help text |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` error answer gains the fix line |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` error examples |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` if the SDK error gains the fix part |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the decode `contains=` message |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/contributing/ze-go-style.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the files named here: `docs/architecture/config/syntax.md` (parser refusal text: affected, update the stray-brace example if shown), `docs/architecture/config/transaction-protocol.md` (reload error text: affected where it quotes the prefix), `docs/architecture/config/yang-config-design.md` (affected: `error-message` on constraints), `docs/architecture/testing/ci-format.md` (affected: decode `contains=` failure report), `docs/architecture/testing/interop.md` (kernel probe advice: affected), `docs/features/interfaces.md` (unit range message: affected if it quotes it). Re-run `./le spec citation anchors spec plan/release/fixes/spec-operator-errors-name-the-fix.md` per phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | error examples in the guide pages above |

## Implementation Steps

1. **Phase: Wiring** - the type, the two check skeletons, stage lines, fixtures red
2. **Phase: checks** - AC-1 to AC-4
3. **Phase: double wrap** - loader, reload, validate; AC-5
4. **Phase: missing fix** - TACACS+, next hop, parser, client, as112, compiled-out; AC-6 to AC-11
5. **Phase: one guard per input** - constraint messages, selector faults, decode Diff; AC-12 to AC-14
6. **Phase: help and causes** - editor help derived, YANG description, NTP, kernelcheck, qemu link; AC-15 to AC-18
7. **Phase: report** - `./le arch operator-error report` lists none of the owned producers

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every owned row's message read in a test |
| Correctness | each fix command exists in the command registry |
| Rule: user-facing-errors | problem, why, fix on its own line, plain English |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the two checks | each refuses its fixture |
| owned rows | each row's test asserts the full message |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | the TACACS+ cause names the failure class (bad secret), never the secret; no message echoes a credential |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the producer |
| Lint failure | Fix inline |
| Functional test fails | Check the AC |
| Audit finds a missing AC | Back to the relevant phase |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The earlier-guard rows are not message rows at heart: two declarations of one constraint. Moving the message onto the declared constraint removes the second declaration

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One type with what, why, fix and cause | a string convention checked by regex | a check can read a type; a convention is a claim |
| Boundary set derived from registration calls | a list of handler packages | `ai/rules/principles.md`: no hand list beside a registry |
| Specific message on the YANG constraint | run the validator before the constraint | RFC 7950 gives the constraint a message; one guard, one message |

## Known Limitations
- Log lines with no refusal (level misclassification rows such as `normal-state-logged-at-error.md`) are not this mechanism and are not owned here
- `./le arch operator-error check` reads a `plugin.Response{Status: StatusError, Error: ...}` answer only where the handler body builds it as a literal outside a nested function literal (605500cf11). An answer a helper function or a nested function literal builds, one whose `Status` is computed at run time, and one whose `Error` is assigned after the literal are not read (`docs/architecture/cli/error-surface.md`). On the tree it reports 0 typed, 187 plain and 43 pass-through answers; existing plain answers are reported, and only an added one is refused at commit
- On a model route (plugin process, subsystem, RPC wrapper, streaming, local data, local `ze <verb>`), a line missing only its selector reads `required argument missing: selector, as in 'selector <selector>'`, where the builtin dispatch route answers with the dispatcher guard's own refusal (37c3a2abb1). Both name the missing word; the wording differs
- `./le arch error-wrap check` still leaves wraps it cannot trace: 1737 of 8728 on the tree after 81eb233e20, which reports 178 findings (155 before). An untraced wrap is counted and reported, never passed as clean. A segment written more than `depthMax` (16) steps from its wrap is not seen (`internal/le/arch/errorwrap/errorwrap.go`)

## RFC Documentation (Scope: cli)

No protocol code changes. The `error-message` use cites RFC 7950 Section 7.5.4.1 in the YANG description comment.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `error-names-no-fix-command.md` | 2026-10-22 | `ErrNextHopIncompatible` names no family or fix |
| `error-names-no-fix-command.md` | 2026-10-22 | `./le test qemu` dangling scratch link |
| `error-names-no-fix-command.md` | 2026-10-15 | `ze cli` `hint: is the daemon running?` |
| `error-names-no-fix-command.md` | 2026-10-15 | as112 `refuse` without the next step |
| `error-names-no-fix-command.md` | 2026-10-19 | TACACS+ `sendToServers` drops causes |
| `error-names-no-fix-command.md` | 2026-10-20 | config parser `expected keyword, got RBRACE` |
| `error-names-no-fix-command.md` | 2026-10-20 | compiled-out feature refusals name no tag |
| `error-names-no-fix-command.md` | 2026-11-07 | iface `ErrNoBackendLoaded` names no cause or fix |
| `error-wrapped-twice-with-one-prefix.md` | 2026-10-13 | hub reload `parse config: parse config:` |
| `error-wrapped-twice-with-one-prefix.md` | 2026-10-13 | `ze config validate` `line N: parse config: line N:` |
| `earlier-guard-hides-the-better-error.md` | 2026-09-03 | `validateUint` ahead of `handleUnitAdd` |
| `earlier-guard-hides-the-better-error.md` | 2026-08-10 | YANG `pattern` ahead of `ze:validate` |
| `earlier-guard-hides-the-better-error.md` | 2026-09-05 | `send ... cached` answers `missing selector` |
| `earlier-guard-hides-the-better-error.md` | 2026-09-06 | `Dispatch` terminal-noun selector hides faults |
| `earlier-guard-hides-the-better-error.md` | 2026-10-18 | decode `Diff` reports `contains=` as invalid hex |
| `help-text-describes-superseded-behaviour.md` | 2026-10-11 | editor help says discard reverts all |
| `help-text-describes-superseded-behaviour.md` | 2026-10-17 | `rpc dns-cache` description |
| `failure-report-names-a-command-that-does-not-exist.md` | 2026-10-13 | `formatPeerError` missing-mandatory hint keeps the slash path |
| `guard-message-teaches-the-violation.md` | 2026-08-24 | `rowOperatorRefusal` tells the operator to select a list no command selects |

18 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| `error-answered-with-another-errors-code.md` | 2026-10-10 | AC-17: Docker lab kernel probe answers `unknown` with kernel-config advice | 2026-11-10: commit 4dbbcacf7b gave `unknown` its own advice in `checkDockerKernel` (`internal/le/interoplab/kernelcheck.go`); proven by the existing `TestDockerKernelUnknownIsProbeFailure`; `docs/architecture/testing/interop.md` L979 documents it. The row is not in "Owned journal rows": AC-17 maps to it |

On 2026-10-23 every owned producer read still produced the recorded message.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete
- [ ] Targeted tests pass; full gate once over the closing commits
- [ ] Every A-N confirmed or broken
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A with reason above)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows re-pointed
- [ ] **Commit B:** `remove plan/release/fixes/spec-operator-errors-name-the-fix.md` only

---

## Implementation Summary

### What Was Implemented
- `internal/core/opererr`: `New`, `Wrap`, `Error` (problem, why with cause, `fix:` line), `LogAttrs`; all three parts required (728fb3b57e)
- `le arch error-wrap` (`internal/le/arch/errorwrap`) and `le arch operator-error` (`internal/le/arch/operatorerror`, `Sites`, `Scan`), both in the verify stage list (`internal/le/verify/engine/stages.go`), and the change-set seam `internal/le/commit/changecheck` that `./le commit create` runs (`changeCheckRefusal`, `internal/le/commit/changechecks.go`) (728fb3b57e, 6c129c64a7)
- Every owned row's producer, phases 2 to 7 (3e94cb7c40): `config.LineError`, `parseConfigError`, `SplitLine`; reload wraps `reload: ` only; stray `}`; `registry.CompiledOutValueError`, `KeywordHint`; TACACS+ `allServersFailed`; `nextHopRefusal`; `daemonUnreachable`, `sshclient.CredentialsUnavailable`; `sdk.RunningConfigLost` in seven plugins; YANG `error-message` (`yang/constraint_message.go`); `ArgumentFaults`; `sendNotPermitted`; `decode.DiffPart`; `serverUnsynchronized`; `editorCommands`; `danglingScratchLink`; `rowOperatorRefusal`
- `MissingDependencyError` and `UnloadedDependencyError` (fdf90dd99e); every fault of an argument list named with its usage and the whole command (3af4462e82)
- Issues found along the way, fixed on Thomas's order: `withUnwind` keeps a refused reload's fix line last and `sdk.RunningConfigLost` no longer panics on an empty problem (d131605aa4); error-wrap follows function parameters and verb prefixes (81eb233e20); operator-error reads `StatusError` answers and `plugin.OperatorErrorResponse` is the typed route (605500cf11); `ArgumentFaults.RestatedForLine` restates a missing-only argument list alike on every route (37c3a2abb1)

### Bugs Found/Fixed
- No-session `discard <path>` discarded everything (`editor_discard_path.go`), fixed in 3e94cb7c40 with its test
- `le help` panicked at 728fb3b57e on a 72-character summary; 6c129c64a7 (another session) shortened it

### Documentation Updates
- In the implementation commits: `docs/architecture/cli/error-surface.md` ("The operator error"), `docs/contributing/ze-go-style.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/config/{syntax,transaction-protocol,yang-config-design}.md`, `docs/architecture/testing/{ci-format,qemu-integration,verify-freshness-scope}.md`, `docs/functional-tests.md`, `docs/guide/{configuration,config-editor,tacacs,plugins,ospf,config-reload}.md`, `docs/features/cli-commands.md`
- Closure grep for every old text (`is the daemon running`, `parse config: parse config`, `got RBRACE`, `all TACACS+ servers unreachable`, `kiss of death`, `next-hop incompatible`, `Revert all`) over `docs/`: no hit
- In the follow-up commits: `docs/architecture/api/process-protocol.md` (d131605aa4); `docs/architecture/cli/error-surface.md`, `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/{committing,ze-go-style}.md` (605500cf11); `docs/architecture/api/commands.md`, `docs/features/cli-commands.md` (37c3a2abb1)

### Deviations from Plan
- A-1 broken: goyang kept `error-message` but nothing spoke it; `yang/constraint_message.go` was added
- A-3 broken, then fixed: error-wrap did not see either owned double-wrap row; 81eb233e20 follows a function parameter to its callers' arguments and compares verb-holding segments, and both rows stay guarded by their `.ci` tests
- AC-4 read under the no-`./le verify` rule: `stages_operatorerrors_test.go` proves the stage lines, and the commit seam runs operator-error
- The isis system-id validator became completion only (`ValidateFn` nil): one guard, the YANG pattern

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: `error-message` on a `range` reaches the refusal | goyang parsed it, the resolved type dropped it | `TestYANGRangeErrorMessageReachesRefusal` red | `constraint_message.go` carries it on every path |
| assumption | A-3: one-level callee resolution finds the owned double wraps | the reload error arrives through a function value, the validate prefix holds a verb | fixture run against the owned rows | 81eb233e20 (`funcvalue.go`, verb-normalised segments) |
| approach | 728fb3b57e shipped a 72-character menu summary | the menu line holds 60, and `le help` panicked | the launcher set the builds aside | 6c129c64a7 |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One operator error type with what, why, fix, cause | Done | `internal/core/opererr/opererr.go` `New`, `Wrap` | |
| Context added once by the producer | Done | `config/line_error.go` `LineError`, `parseConfigError`; `cmd/ze/hub/main_reload.go` `runReloadContext` | |
| One guard per input, message on the constraint | Done | `config/yang/constraint_message.go`; `zt:vlan-id`; isis `system-id` pattern | |
| Double-wrap check | Done | `internal/le/arch/errorwrap` | sees both owned rows since 81eb233e20 |
| Handler check | Done | `internal/le/arch/operatorerror`, `internal/le/commit/changecheck` | returned errors and `StatusError` answers (605500cf11); unread shapes in Known Limitations |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestErrorWrapRefusesRepeatedPrefix` | |
| AC-2 | Done | `TestOperatorErrorRefusesPlainErrorFromHandler` | |
| AC-3 | Done | `TestOperatorErrorAcceptsTypedError` | |
| AC-4 | Done | `stages_operatorerrors_test.go`; `changeCheckRefusal` | see Deviations |
| AC-5 | Done | `reload-parse-error-single-prefix.ci`, `validate-parse-error-single-prefix.ci`, `TestSplitLineNamesTheLineOnce` | |
| AC-6 | Done | `TestTacacsAllServersFailedNamesEachCauseAndFix` | |
| AC-7 | Done | `TestNextHopIncompatibleNamesFamiliesAndFix` (resolves the fix line against the schema) | |
| AC-8 | Done | `TestParserStrayBraceNamesTheFix`, `TestMigrateStrayBraceNamesTheFix`, old-5.0 `conf-extended-attributes.ci` | |
| AC-9 | Done | `TestCLINoDaemonNamesEndpointAndFix` | |
| AC-10 | Done | `config-compiled-out-feature-names-tag.ci`, `TestCompiledOutRootNamesItsBuildTag`, `TestMissingDependencyErrorNamesTag`, `TestUnloadedDependencyErrorNamesWhatToConfigure` | |
| AC-11 | Done | `TestInstallRefusedReloadReturnsAFailedRegistrationRestore`, `TestRunningConfigLostCarriesWhyAndFix` | |
| AC-12 | Done | `iface-unit-out-of-range-message.ci`, `TestISISSystemIDPatternIsTheOneGuard`, `TestYANGRangeErrorMessageReachesRefusal`, `TestLeafListPatternRefusalSpeaksTheMessage` | |
| AC-13 | Done | `TestCacheForwardRefusalNamesTheAttachBlock`, `TestDispatchTerminalNounSelectorNamesTheRealFault`, `TestValidateArgsNamesEveryFault`, `cli-argument-refused-plugin-route.ci` | every route alike since 37c3a2abb1 (`TestRoutesRestateArgumentFaultsAlike`) |
| AC-14 | Done | `TestMismatchDiffReportsAMissingPartNotInvalidHex` | |
| AC-15 | Done | `TestEditorHelpDerivesFromRegisteredHelp`, `TestEditorDiscardBareRejected`; `ze-cli-clear-api.yang` `rpc dns-cache` | |
| AC-16 | Done | `TestNTPUnsynchronisedNotKissOfDeath`, `TestNTPClientRejectsUnsynchronisedChrony` | |
| AC-17 | Done | `TestDockerKernelUnknownIsProbeFailure` (4dbbcacf7b) | Rows found already fixed |
| AC-18 | Done | `TestDanglingScratchLinkNamesLinkTargetAndFix` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestErrorWrapRefusesRepeatedPrefix`, `TestErrorWrapAcceptsNewContext` | Done | `internal/le/arch/errorwrap/errorwrap_test.go` | |
| `TestOperatorErrorRefusesPlainErrorFromHandler`, `TestOperatorErrorAcceptsTypedError` | Done | `internal/le/arch/operatorerror/` | |
| `TestOperatorErrorRendersThreeParts` | Done | `internal/core/opererr/opererr_test.go` | |
| `TestTacacsAllServersFailedNamesEachCauseAndFix` | Done | `internal/component/tacacs/client_test.go` | |
| `TestNextHopIncompatibleNamesFamiliesAndFix` | Done | `internal/component/bgp/reactor/peer_test.go` | |
| `TestParserStrayBraceNamesTheFix` | Done | `internal/component/config/parser_test.go` | |
| `TestYANGRangeErrorMessageReachesRefusal` | Done | `internal/component/config/yang/constraint_message_test.go` | |
| `TestEditorHelpDerivesFromRegisteredHelp` | Done | `internal/component/cli/editor_help_test.go` | |
| `TestNTPUnsynchronisedNotKissOfDeath` | Done | `internal/plugins/ntp/ntp_test.go` | |
| Four functional tests | Done | `test/reload/`, `test/parse/`, `test/plugin/`, `test/ui/` | `config-compiled-out-feature-names-tag.ci` lives in `test/ui/`, not `test/parse/` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify | Done | `git show --stat` of 728fb3b57e, 3e94cb7c40, fdf90dd99e, 3af4462e82 |
| `internal/core/opererr/`, `internal/le/arch/errorwrap/`, `internal/le/arch/operatorerror/` | Done | |

### Audit Summary
- **Total items:** 18 ACs, 5 requirements, 10 test rows
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** AC-4 proof route, the isis validator, A-1 and A-3 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator reads what went wrong, why, and the fix | functional | `reload-parse-error-single-prefix.ci`, `validate-parse-error-single-prefix.ci`, `iface-unit-out-of-range-message.ci`, `config-compiled-out-feature-names-tag.ci`, `cli-argument-refused-plugin-route.ci` through the real `ze` entry points (3e94cb7c40, 3af4462e82 bodies: green) |
| Context added once | functional | the two single-prefix `.ci` tests assert `parse config` and `line N` once |
| The specific guard speaks | functional | `iface-unit-out-of-range-message.ci` (VLAN message through the RPC validator) |
| The family does not return | unit + commit seam | `TestOperatorErrorRefusesPlainErrorFromHandler`, `TestErrorWrapRefusesRepeatedPrefix`; `changeCheckRefusal` runs operator-error on every commit |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | the issues found along the way landed in d131605aa4, 81eb233e20, 605500cf11 and 37c3a2abb1; what remains is stated in Known Limitations | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/operator-errors-name-the-fix-ad9def1f-df41-4fcc-bc2d-9569a17e6b8c.md` (244 files: every file of the commits carrying `Spec-work: operator-errors-name-the-fix` and the closure files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 2: round 1 over the implementation (two NOTEs); round 2, by a second closer, over d131605aa4, 81eb233e20, 605500cf11, 37c3a2abb1 and 7c05adcdcf (`withUnwind` keeps the fix line last and `Unwrap` reaches every step; `RestatedForLine` writes each missing leaf once, the placed one excluded) and this closure text, no finding |
| Reviewer lenses used | logic and wiring (producers of every owned row), security (TACACS+ secret, panics on operator input), style (`opererr.New` panics are author-input only) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| N1 | NOTE | `runReloadContext`'s `(candidate cleanup failed: ...)` landed after the opererr fix line | `cmd/ze/hub/main_reload.go` | d131605aa4 (`withUnwind`, `cmd/ze/hub/reload_unwind.go`, `TestWithUnwindKeepsFixLineLast`) |
| N2 | NOTE | public `sdk.RunningConfigLost` panicked on an empty problem from a third-party caller | `pkg/plugin/sdk/config_tx.go` | d131605aa4 (`TestRunningConfigLostEmptyProblem`) |

No BLOCKER or ISSUE was found.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/opererr/opererr.go`, `internal/le/arch/errorwrap/`, `internal/le/arch/operatorerror/` | Yes | read at closure |
| `test/reload/reload-parse-error-single-prefix.ci`, `test/parse/validate-parse-error-single-prefix.ci`, `test/plugin/iface-unit-out-of-range-message.ci`, `test/ui/config-compiled-out-feature-names-tag.ci` | Yes | `git grep` at closure lists each with its VALIDATES line |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-4 | checks refuse their fixtures | 728fb3b57e body: `go test -race` green for opererr, errorwrap, operatorerror, verify/engine |
| AC-5 to AC-18 | owned producers fixed | 3e94cb7c40 body: 40 packages and 29 functional tests; fdf90dd99e and 3af4462e82 bodies: registry, server, command, peer, rib, mcp green |
| AC-12, AC-15 | `config/yang` and `cli` tests | main thread, 2026-11-10, green: `config/yang -run Constraint\|ErrorMessage\|TestYANGRange`, `cli -run TestEditor\|TestPeerMissingFieldHint` |
| AC-2, AC-3, AC-11, AC-13, A-3 | follow-up commits | main thread, 2026-11-10, green: `cmd/ze/hub` Confirm\|Revert\|TestWithUnwind\|TestReload; `pkg/plugin/sdk` and `internal/plugins/as112` TestRunningConfigLost\|TestConfigTx; full packages `internal/component/command`, `internal/component/plugin/server` (all feature tags), `internal/component/plugin`, `internal/le/arch/operatorerror`, `internal/le/arch/errorwrap`; functional `policy-test-errors`, `metrics-name-show`, `reload-parse-error-single-prefix`, ui `cli-argument-refused-plugin-route`, ui `debug-help`. Foreign red: `internal/le` root vocabulary tests on `reclaim worktrees` and `commit approval-check`, another session's |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| hub reload of a refused file | `test/reload/reload-parse-error-single-prefix.ci` | green, 3e94cb7c40 |
| `ze config validate` set file | `test/parse/validate-parse-error-single-prefix.ci` | green, 3e94cb7c40 |
| `./le arch error-wrap check`, `operator-error check`, TACACS+ | unit tests in the Wiring Test table | green, 728fb3b57e and 3e94cb7c40 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, then fixed | `constraint_message.go`; Mistake Log |
| A-2 | confirmed | `TestASTBoundarySetCoversRuntimeRegistry`: 397 RPCs, 32 verifiers |
| A-3 | broken, then fixed | 81eb233e20: `TestErrorWrapRefusesThroughAFunctionParameterChain`, `TestErrorWrapRefusesARepeatedVerbPrefix`; Mistake Log |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| error surface and operator error | `docs/architecture/cli/error-surface.md` against `opererr.Error.Error` | Yes |
| `clear dns cache`, `discard` | `docs/guide/command-reference.md` L1571-1577 against `ze-cli-clear-api.yang`; `editorCommands` | Yes |
| no page quotes an old message | closure grep over `docs/`, no hit | Yes |

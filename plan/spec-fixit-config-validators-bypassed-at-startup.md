# Spec: fixit-config-validators-bypassed-at-startup

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 2/2 |
| Deferral shard | `-` |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Every custom config validator is bypassed when the daemon starts and when it
reloads. A config the operator hand-edits reaches the wire unvalidated.**

`LoadConfig` (`internal/component/config/loader.go`) parses and returns. It calls
`ParseTreeWithYANG`, `ExtractPluginsFromTree`, `MergeCliPlugins` and
`ExpandDependencies`, and it never calls `ValidateTreeAllModules`. Verified
2026-08-10 by reading the function.

`ValidateTreeAllModules` (`internal/component/config/yang/validator.go`) has
**exactly one non-test caller**, `runValidation` in
`internal/component/config/cli/cmd_validate.go`. Verified by grep over every
tracked `.go` file, tests excluded.

So the only paths that apply a `ze:validate` rule are `ze config validate`, the
API commit hook (`configValidationHook`, `cmd/ze/hub/api.go`) and the two web
editor sites (`cmd/ze/hub/service_web.go`). The daemon's own startup
(`runYANGConfig`, `cmd/ze/hub/main.go`) and its SIGHUP reload (`readAndParse`
feeding `handleSIGHUPReload`) both go through `LoadConfig` and validate nothing.

**The blast radius is 22 registered validators over 29 YANG bindings**, measured
2026-08-10 from `internal/component/config/validators_register.go` and a grep of
`ze:validate` across every `.yang`:

| Validator | Bindings |
|-----------|----------|
| `ospf-router-id`, `ospf-area-id`, `mac-address` | 4 each |
| `registered-address-family` | 3 |
| `port-spec`, `nonzero-ipv4` | 2 each |
| `isis-hostname`, `isis-net`, `isis-system-id`, `ipv4-address`, `ipv6-address`, `community-range`, `send-message-type`, `redistribute-source`, `receive-event-type`, `internal-plugin-name` | 1 each |

**How it was found.** The independent review of
`plan/spec-fixit-isis-hostname-ascii.md` looked for the path that carries that
spec's RFC 5301 guarantee and could not find one. That spec makes the config
boundary the ONLY enforcement point, deliberately: `hostnameTLV` is unchanged so
the operator's octets reach the wire byte for byte, and Thomas ruled on
2026-07-30 to reject at config time rather than sanitise at emit. With startup
unvalidated, a hand-edited hostname plus a restart still puts 8-bit octets into
IS-IS TLV 137, violating RFC 5301 section 3.

**Owner ruling (Thomas, 2026-08-10).** Validate in `LoadConfig`. He chose this
over fixing the IS-IS plugin's own verify path, which does not obviously cover
startup and leaves the other 21 validators bypassed, and over recording an
owner-signed limitation, which `ai/rules/rfc-compliance.md` reserves to him for
an RFC MUST and which he declined.

**Owner ruling (Thomas, 2026-08-11, answering Q-1).** REFUSE the reload. On
SIGHUP with a config that fails validation, the reload fails and the daemon
keeps running its existing config. No override, no force flag.

## The upgrade cost, which is the whole risk

A config that boots today can stop booting. That is the same shape as the
management-listener guard (`spec-fixit-mgmt-listener-auth-guard`, closed 2026-08-11), and it
needs the same treatment: an entry under "Upgrading" in the operator guide,
naming what now refuses, why, and how to see it before the upgrade.

`ze config validate` already answers "would this config refuse", so an operator
has a way to check first. Say so where they will read it.

## Open questions

All three are answered. None is open.

| # | Question | Answer |
|---|----------|--------|
| Q-1 | Does the reload path refuse, or warn and keep the previous config? | **Thomas, 2026-08-11: REFUSE the reload.** On SIGHUP with a config that fails validation, the reload fails and the daemon keeps running its existing config. No override, no force flag |
| Q-2 | Is any currently-shipping example, test fixture or QEMU image config invalid under the 22 validators? | Measured: 3548 candidate config texts, 1387 that parse as ze config, checked against the 17 sections `runValidation` walks. 7 embedded configs fail, and all 7 are negative fixtures already asserting exit 1. Arming the gate over those 17 sections lands green |
| Q-3 | Does `ze config validate` cover every section `LoadConfig` parses? | No, and the gap is not closed here. See "Q-3, and why the list is not widened" below |

### Q-1 needs no new machinery

A `LoadConfig` error on the reload path lands in `runReload`
(`cmd/ze/hub/main_reload.go`) at its `loadErr` branch: it clears the staged
candidate and returns `reload: parse config: <err>` BEFORE `ReloadConfig`, the
provider refresh and `engine.Reload` run. `doReload` marks the reload processed
as failed, and `handleSIGHUPReload` prints `reload error: <err>` and loops. The
running config, the listeners and the open sessions are untouched. This is the
same outcome the `config reload: transaction failed` branch
(`internal/component/plugin/server/reload.go`) already produces, reached one
step earlier.

### Q-3, and why the list is not widened

`yangSectionsToValidate` is a hand-written list of 17 names against a schema
that derives 36 top-level sections. Six of its names -- `web`, `ssh`, `dns`,
`looking-glass`, `mcp`, `managed` -- are not top-level sections at all: they sit
under `environment`, which the list omits, so those six iterations have always
been dead. Twenty-five real sections are missing.

The list is implemented AS IT IS. Widening is gated on three separate defects,
each with its own blast radius, all recorded in
`plan/journal/gate-excludes-part-of-its-population.md`:

| Blocker | Measured |
|---------|----------|
| `AddressFamilyValidator` (`internal/component/config/validators.go`) reads `registry.FamilyMap()`, which excludes `builtinFamilies` | In a full-tag binary `FamilyMap` is 19 names WITHOUT `ipv4/unicast`; `AllFamilies` is 23 WITH it. Widening to `bgp` and `redistribute` would refuse 742 + 33 family sites across 687 shipped configs |
| `send-message-type` and `receive-event-type` read registries `RegisterPluginSendTypes` populates from `PeersFromTree` and `NewServer` | Both are strictly LATER than the point the new call sits, so they would fail closed on valid input at exactly that moment |
| The BGP exclusion is sound and stays | `PeersFromConfigTree` (`internal/component/bgp/config/peers.go`) re-checks all four bgp-bound names, and startup reaches it directly, from `CreateReactorFromTree` (`internal/component/bgp/config/loader_create.go`). `infra.ValidateBGPPeers` is the OFFLINE door onto the same walk, through `validatePeersFromTree` (`internal/component/bgp/config/register.go`); its two non-test callers are `checkBGPPeerConfig` (`ze doctor`) and `runValidation` (`ze config validate`), and neither is the startup path. Corrected 2026-08-12: the invariant holds, the chain first written here did not |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - how a `ze:validate` custom validator is declared, registered and reached
  → Constraint: a validator name used in YANG must be registered, or the startup integrity check in `internal/component/config/yang/validator_registry.go` fires. That check proves the NAME resolves; it does not prove the RULE runs.
  → Decision: the fix adds no validator and changes no rule. It gives the existing walk a second caller.

### RFC Summaries (Scope: config)
- [ ] `rfc/short/rfc5301.md` - the obligation that made this visible
  → Constraint: `RFC5301-3-7` is enforced at the config boundary and nowhere else, by owner ruling of 2026-07-30. An unvalidated startup path is therefore an unenforced MUST.

**Key insights:** (minimal context to resume after compaction)
- `LoadConfig` parses; it never validates. `ValidateTreeAllModules` has one non-test caller.
- The gap is not IS-IS. It is 22 validators over 29 bindings.
- The risk is not the fix, it is the upgrade: a config that boots today can stop booting.

## RFC Documentation

| RFC | Section | Requirement | How this spec touches it |
|-----|---------|-------------|--------------------------|
| RFC 5301 | 3 | `RFC5301-3-7`: "The Value field is encoded in 7-bit ASCII" | Enforcement is at the config boundary and nowhere else, by owner ruling of 2026-07-30: `hostnameTLV` is unchanged, so the operator's octets reach TLV 137 byte for byte. Until this spec the boundary was reached only by `ze config validate`, so a hand-edited hostname plus a restart still violated the MUST. `test/isis/isis-hostname-startup-refused.ci` adds a positive test on the daemon path. No requirement is added, retired or reclassified, and no `{gap}` changes |

## Current Behavior (MANDATORY)

**Source files read:** (2026-08-10, each at its producer)
- [ ] `internal/component/config/loader.go` - `LoadConfig` calls `ParseTreeWithYANG`, `ExtractPluginsFromTree`, `MergeCliPlugins`, `ExpandDependencies`. No validation call.
- [ ] `internal/component/config/yang/validator.go` - `ValidateTreeAllModules` is the tree walk that applies custom validators.
- [ ] `internal/component/config/cli/cmd_validate.go` - `runValidation` is its ONLY non-test caller, and it walks `yangSectionsToValidate` rather than the whole tree (Q-3).
- [ ] `cmd/ze/hub/main.go` - `runYANGConfig` (startup) and `readAndParse` feeding `handleSIGHUPReload` both reach `LoadConfig`.
- [ ] `internal/component/config/validators_register.go` - 22 registered names.

**Behavior to preserve:**
- Every config that is valid today boots unchanged (AC-4).
- `ze config validate`, the API commit hook and the web editor keep working as they do.

**Behavior to change:**
- Startup and reload apply the same rules the editor already applies.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config file on disk, at `ze start` or at SIGHUP.

### Transformation Path
1. `ParseTreeWithYANG` builds the tree and applies leaf-level YANG types and patterns.
2. **Missing today:** `ValidateTreeAllModules` applies the registered custom validators.
3. Plugin extraction and dependency expansion.
4. The tree reaches the subsystems, which encode from it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| File -> config tree | `ParseTreeWithYANG` | Yes, read |
| Config tree -> validators | absent on this path | Yes, that is the defect |
| Config tree -> protocol encoders | subsystem readers | Yes, IS-IS TLV 137 traced |

### Integration Points
- `runValidation` already performs the walk. Share it rather than writing a second one, or the two drift.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No, today | The validator layer is bypassed on the startup path; that is the subject |
| No unintended coupling | Yes | `LoadConfig` already depends on the yang package |
| No duplicated functionality | Must hold | Q-3: share `runValidation`'s walk, do not recreate it |
| Registration over hardcoding | Yes | Validators register; this adds no per-validator code |

## Risks & Assumptions

| # | Statement | Status |
|---|-----------|--------|
| R-1 | A shipping example, fixture or QEMU image config is invalid under the 22 validators, so arming the gate lands red | **retired.** Q-2 measured it: 7 of 1387 parsing configs fail, all 7 negative fixtures already asserting exit 1 |
| R-2 | A refusing reload is worse than an invalid value for a running daemon | **retired.** Thomas ruled on 2026-08-11: refuse. The daemon is not dropped -- it keeps its existing config, so the risk this row named does not arise |
| A-1 | `yangSectionsToValidate` covers every section `LoadConfig` parses | **broken.** Q-3 measured 17 names against 36 derived sections, six of the 17 dead. The list is shared unchanged rather than widened, so the two callers cannot disagree; closing the gap is separate work with its own blast radius |

## Blast Radius

Every daemon start and every SIGHUP, for every subsystem with a `ze:validate`
binding. That is the point of the change and the reason the upgrade note is an
acceptance criterion rather than a courtesy.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry point | | What runs | Test |
|-------------|---|-----------|------|
| `ze start` with an invalid config file | -> | `LoadConfig` -> `ValidateCustomSections` -> `ValidateTreeAllModules` -> refusal | `test/parse/config-startup-refuses-invalid-validator.ci` |
| SIGHUP with an invalid config file | -> | same walk, refused at `runReload`'s `loadErr` branch | `test/reload/config-reload-invalid-validator.ci` |
| Hand-edited IS-IS hostname, restart | -> | `isis-hostname` validator refuses | `test/isis/isis-hostname-startup-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config file carrying a value a registered validator refuses, and `ze start` | The daemon refuses to start and names the leaf, the value and the rule. It does not bind, and it does not put the value on any wire. The refusal is unconditional: it is NOT answered by `RecoverConfig`, which walks rollback history and rewrites the config file on disk. See "AC-1 and the rollback-recovery route" below |
| AC-2 | The same file and a SIGHUP reload of a running daemon | Q-1's answer, implemented and tested. Whatever it is, the invalid value never reaches a protocol encoder |
| AC-3 | An IS-IS hostname carrying an octet outside `0x20`..`0x7e`, hand-edited, plus a restart | Refused. This is the RFC 5301 section 3 path `plan/spec-fixit-isis-hostname-ascii.md` depends on, and it closes that spec's BLOCKER |
| AC-4 | Every config that boots today and is valid | Boots unchanged. Q-2 must be measured, not assumed |
| AC-5 | The operator guide after the change | Carries the upgrade entry, names `ze config validate` as the way to check first |

### AC-1 and the rollback-recovery route (found in review round 1, 2026-08-12)

`runYANGConfig` (`cmd/ze/hub/main.go`) answers a `LoadConfig` error with
`RecoverConfig` (`internal/component/config/stamp.go`). `RecoverConfig` acts
only when the config's schema stamp names a release NEWER than this binary's
(`version.IsNewerRelease`, checked first). It then walks rollback history, loads
the newest version this binary can read, WRITES it back over the config file,
and the daemon starts.

Adding the validation call to `LoadConfig` made that route reachable by a
validator refusal for the first time: before it, a refusal was never a
`LoadConfig` error. On a binary older than the config's stamp, an operator's
hand-edited bad value would have had their file replaced by a rollback and the
daemon started on a config they never wrote.

**Decided: a validator refusal declines recovery.** `RecoverConfig` answers ONE
failure, "this binary is older than the schema that wrote the file". A custom
validator refusal is a different fact, produced by rules that ship with THIS
binary: it says the operator wrote a value the rules refuse, and it names the
leaf, the value and the rule. Answering that by discarding their edit is the
opposite of AC-1, and it is destructive where the refusal is merely a stop.
Parse failures keep the recovery route, which is the failure version skew
really produces.

Implemented as `config.ErrCustomValidation` (a wrap, so the operator-facing
message is unchanged) plus `recoverableLoadError` in `cmd/ze/hub/main.go`.
`TestRecoverableLoadErrorDeclinesAValidationRefusal` and
`TestRecoverableLoadErrorAllowsAParseFailure`
(`cmd/ze/hub/main_recover_test.go`) drive the predicate with errors `LoadConfig`
really returns.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | hand-edits `config.conf` with a bad IS-IS hostname and restarts | file -> `LoadConfig` -> tree walk -> refusal, naming the leaf | AC-3 |
| 2 | runs `ze config validate` before upgrading, to see what will refuse | unchanged path, already works | AC-5 |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoadConfigRefusesAnInvalidCustomValidatorValue` | `internal/component/config/loader_test.go` | AC-1: `LoadConfig` returns an error naming the leaf and the value | |
| `TestLoadConfigAcceptsEveryShippingExample` | `internal/component/config/loader_test.go` | AC-4 and R-1: every example and fixture config still loads | |
| `TestLoadConfigWalksEverySectionCmdValidateWalks` | `internal/component/config/loader_test.go` | A-1 and Q-3: the two callers cover the same sections | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | this change adds no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-startup-refuses-invalid-validator` | `test/parse/config-startup-refuses-invalid-validator.ci` | the daemon refuses to start on a hand-edited invalid value and says which leaf | written, not executed (the run needs the functional suite this session was told not to run) |
| `config-reload-invalid-validator` | `test/reload/config-reload-invalid-validator.ci` | Q-1's answer, driven through a real SIGHUP: the reload fails and the session survives | written, not executed |
| `isis-hostname-startup-refused` | `test/isis/isis-hostname-startup-refused.ci` | AC-3: the RFC 5301 path `spec-fixit-isis-hostname-ascii` depends on | written, not executed |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | | | this is a config-boundary change; no wire format changes. The RFC 5301 emit path is already covered by `spec-fixit-isis-hostname-ascii` | N-A |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No leaf, container or RPC is added. The change gives an existing walk a second caller |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No validator is added and no rule changes. The 22 already registered in `validators_register.go` now run on one more path |
| CLI commands/flags | No | `ze config validate` keeps its command surface and its exit codes |
| CLI grammar (keyword before value) | N-A | No command added or changed |
| Editor autocomplete | N-A | No new leaf, so no `CompleteFn` |
| Functional test for new RPC/API | N-A | No RPC. The three `.ci` cover the daemon paths instead |
| Pipe completeness | N-A | No route output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, module, certificate or binary is added. `ze doctor` behaviour DOES change, because it loads through `LoadConfig`: a refused config now stops it at `doctor-config-parse`. That is documented in `docs/guide/configuration.md` rather than given a new check, because a check would need a second, unvalidated parse path -- the drift this spec exists to remove |
| Prometheus counters/metrics | No | The refusal is a startup or reload failure with a message, not ongoing state. `runReload` already marks the reload failed |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A behaviour change to an existing surface. It is documented as an upgrade entry, which is what an operator needs |
| 2 | Config syntax changed? | No (behaviour yes) | Syntax is untouched. `docs/guide/configuration.md` gains the upgrade entry under `## Validation` |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` and `docs/guide/config-reload.md` (its Error Handling section named only parse failures) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `test/isis/isis-hostname-startup-refused.ci` carries `RFC requirement: RFC5301-3-7 positive`. It ADDS a positive test for a requirement `spec-fixit-isis-hostname-ascii` already gates, so no `{gap}` changes and no `docs/features/rfc-status.md` row changes. `make ze-rfc-check` result recorded below |
| 10 | Test infrastructure changed? | No | Three `.ci` added; no runner change |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | One walk gained a caller. The reason it is one walk is in the `ValidateCustomSections` doc comment |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grepped `docs/` for anchors on `loader.go`, `cmd_validate.go`, `validate_sections.go` and `cmd/ze/hub/main.go`. `docs/guide/config-reload.md` was the one stale claim: its Error Handling listed only parse failures, and its Reload Workflow step 2 stopped at the YANG schema. Both updated. `docs/DESIGN.md` (config pipeline diagram), `docs/architecture/behavior/signals.md` (signal mapping, startup steps), `docs/architecture/core-design.md` and `docs/guide/command-reference.md` (`runValidation`) each state something the change leaves true |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `## Validation` section in `docs/guide/configuration.md`. Both error examples in it were fabricated and are now text `LoadConfig` produced |

## Implementation Steps

1. Answer Q-2 FIRST: run every shipping example, test fixture and QEMU image config through `ValidateTreeAllModules` and record what refuses. If anything does, fix the config or the validator before going further — arming the gate over a red population is what makes this land badly.
2. Answer Q-3: compare `yangSectionsToValidate` against the sections `LoadConfig` parses. Widen or record the gap.
3. Answer Q-1 with Thomas: does a reload refuse, or warn and keep the running config?
4. Share `runValidation`'s walk rather than writing a second one.
5. Call it from `LoadConfig`.
6. Write the upgrade entry in the operator guide, naming `ze config validate` as the pre-upgrade check.

## Design Insights

The startup integrity check in `validator_registry.go` proves every validator
NAME resolves. That is a different claim from every validator RUNNING, and the
gap between those two claims is this spec. A gate that verifies its own wiring
and not its own reach is the shape to look for elsewhere.

## Key Design Decisions

| Decision | Alternative rejected | Why |
|----------|---------------------|-----|
| Validate in `LoadConfig` | Fix the IS-IS plugin's verify path | Does not obviously cover startup, and leaves 21 validators bypassed (Thomas, 2026-08-10) |
| Validate in `LoadConfig` | Record an owner-signed limitation | `ai/rules/rfc-compliance.md` reserves that to the owner for an RFC MUST, and he declined it (2026-08-10) |
| Share `runValidation`'s walk | A second walk in the config package | Two walks drift, and the drift is invisible until a validator runs on one path only, which is this defect |

## Known Limitations

Q-1, Q-2 and Q-3 are open and are implementation steps 1 to 3, not deferrals.

## Files to Modify

- `internal/component/config/validate_sections.go` (new) - `validatedSections`, `SectionValidationError`, `ValidateCustomSections`, `refuseInvalidCustomSections`, `isSensitiveLeaf` (moved from `cli`), `ErrCustomValidation`
- `internal/component/config/loader.go` - `LoadConfig` calls `refuseInvalidCustomSections` after `ParseTreeWithYANG`
- `internal/component/config/cli/cmd_validate.go` - `runValidation` calls the shared walk; its own `yangSectionsToValidate` and `isSensitiveLeaf` are deleted
- `cmd/ze/hub/main.go` - `recoverableLoadError` keeps a validator refusal out of `RecoverConfig` (see "AC-1 and the rollback-recovery route")
- `docs/guide/configuration.md` - the upgrade entry under `## Validation`

## Checklist

Checkboxes stay `[ ]` (`.claude/rules/post-compaction.md`); the state is in the
annotation under each line.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
  -> AC-1, AC-2, AC-3: unit evidence only. The three `.ci` are written and UNRUN
     (no functional suite ran in the sessions that wrote them). AC-4, AC-5: done.
     See the Goal Validation table below.
- [ ] Every user story has a working path and a passing test
  -> Story 1 rests on `test/isis/isis-hostname-startup-refused.ci`, UNRUN. Story 2
     is the unchanged `ze config validate` path.
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
  -> All three rows name a `.ci` that exists on disk.
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
  -> NOT RUN. The implementation sessions were instructed not to run any suite.
     This is the main thread's gate to run, and it is what executes the three `.ci`.
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
  -> `LoadConfig` (`internal/component/config/loader.go`) and `runYANGConfig`
     (`cmd/ze/hub/main.go`) both call the new code.
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
  -> Documentation: `docs/guide/configuration.md`, "Upgrading from a release that
     validated only on demand". Every error string in it was produced by
     `LoadConfig` and pasted, not paraphrased.
- [ ] Architectural Verification table filled, including registration over hardcoding
  -> Filled above. One walk, two callers; no per-validator code added.
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
  -> Round 1 returned 1 BLOCKER and 3 ISSUEs, all fixed 2026-08-12. The Review
     Gate artifact is `/ze-close`'s, and it is not recorded here.
- [ ] Every A-N confirmed or broken, none `unvalidated`
  -> A-1 is `broken`, with the measurement and the reason the list is not widened.
- [ ] Deferral shard resolved: no live row without a destination
  -> Shard is `-`; there are no rows.

### TDD
- [ ] Tests written
  -> Unit: `internal/component/config/validate_sections_test.go` (5 tests),
     `internal/component/config/cli/cmd_validate_startup_agreement_test.go`,
     `cmd/ze/hub/main_recover_test.go` (2 tests). Functional: the three `.ci`.
- [ ] Tests FAIL (paste output)
  -> Two mutations were run on 2026-08-12 and both went red, sources restored
     byte-identical (sha256 compared before and after):
     `recoverableLoadError` returning true unconditionally (the pre-fix state):
     `--- FAIL: TestRecoverableLoadErrorDeclinesAValidationRefusal (0.34s)`
     `    main_recover_test.go:41: a validator refusal must not be answered by rewriting the config from rollback history`
     `isSensitiveLeaf` splitting the path on "." (the pre-fix separator):
     `--- FAIL: TestIsSensitiveLeafReadsTheProducersPathSeparator (0.00s)`
     `    validate_sections_test.go:144: isSensitiveLeaf("isis/authentication/key-chain[kc1]/key[1]/secret") = false, want true`
     `    validate_sections_test.go:144: isSensitiveLeaf("l2tp/tunnel[t1]/secret") = false, want true`
- [ ] Tests PASS (paste output)
  -> `ok  	github.com/ze-software/ze/internal/component/config	72.172s`
     `ok  	github.com/ze-software/ze/internal/component/config/cli	100.423s`
     `ok  	github.com/ze-software/ze/cmd/ze/hub	108.556s`
     `ok  	github.com/ze-software/ze/internal/component/doctor	39.942s`
     The three `.ci` are UNRUN.
- [ ] Boundary tests for all numeric inputs
  -> N-A. The change adds no numeric input; it gives an existing walk a caller.
- [ ] Functional `.ci` tests for end-to-end behavior
  -> Three written, all UNRUN.
- [ ] Interop tests for protocol features (or N-A with a reason)
  -> N-A. Config-boundary change, no wire format changes. The RFC 5301 emit path
     is covered by `spec-fixit-isis-hostname-ascii`.

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`

## Notes

`plan/spec-fixit-isis-hostname-ascii.md`'s Wiring Test row 2 currently asserts
that a daemon commit or reload "runs the same tree walk, so it reaches the same
validator". That is FALSE today and is corrected there. It becomes true when this
spec lands.

---

## Implementation Summary

### What Was Implemented
- `internal/component/config/validate_sections.go` (new): `validatedSections`
  (unexported; it has no cross-package consumer), `SectionValidationError` with
  `Blocking()` and `Message()`, `ValidateCustomSections` (the one walk),
  `refuseInvalidCustomSections` (LoadConfig's verdict), `isSensitiveLeaf` moved
  from `cli`, and `ErrCustomValidation`.
- `internal/component/config/loader.go`: `LoadConfig` calls
  `refuseInvalidCustomSections` after `ParseTreeWithYANG`. This is the whole
  behaviour change; the reload refusal needs no machinery of its own.
- `internal/component/config/cli/cmd_validate.go`: `runValidation` calls the
  shared walk. Its `yangSectionsToValidate` and its own `isSensitiveLeaf` are
  gone, so the two callers cannot drift.
- `cmd/ze/hub/main.go`: `recoverableLoadError` keeps a validator refusal out of
  `RecoverConfig`.
- `docs/guide/configuration.md`: the upgrade entry under `## Validation`.
- Tests: `validate_sections_test.go` (5), `cmd_validate_startup_agreement_test.go`
  (2), `main_recover_test.go` (2), and three `.ci` (`test/parse/`, `test/reload/`,
  `test/isis/`). `validators_register_test.go` ships with
  `spec-fixit-isis-hostname-ascii`, which owns the three IS-IS validator names it
  pins.

### Bugs Found/Fixed
- **`RecoverConfig` reachable by a validator refusal.** Found by the round-1
  independent review. An operator's hand-edited bad value, on a binary older
  than the config's schema stamp, would have had the config file on disk
  overwritten from rollback history and the daemon started. Covered by
  `TestRecoverableLoadErrorDeclinesAValidationRefusal` (`cmd/ze/hub/`).
- **`isSensitiveLeaf` split the path on `.`, and the walk emits `/`.** Found
  while re-reading the redaction for the review. `walkTree`
  (`internal/component/config/yang/validator.go`) joins with `Byte('/')`, and
  `SensitiveKeys` (`internal/component/config/schema.go`) keys on the bare leaf
  name, so the lookup matched nothing: `Message()` promised a redaction that
  could not fire, on a path that now prints into the startup log. The defect was
  moved verbatim from `cli/cmd_validate.go`, where it was equally live. Covered
  by `TestIsSensitiveLeafReadsTheProducersPathSeparator`.
- **Both error examples in the operator guide were fabrications.** Found by the
  round-1 review. The MAC example was not merely misworded: the `mac address`
  leaf carries a YANG `pattern`, which the config-file PARSE path applies and
  aborts on, so `MACAddressValidator` is unreachable from a config file and no
  such message exists. Both examples are now text `LoadConfig` produced.

### Documentation Updates
- `docs/guide/configuration.md`, `## Validation` -> "Upgrading from a release
  that validated only on demand". Source anchors added for
  `validate_sections.go`, `loader.go`, `main_reload.go` and `doctor.go`.
- `make ze-doc-test` NOT RUN (this session was instructed to run no suite).

### Deviations from Plan
- `ValidatedSections` was planned exported and ships unexported: no consumer
  outside the package uses it, and `cmd_validate.go` reaches the list only
  through `ValidateCustomSections`.
- The plan named no change in `cmd/ze/hub/main.go`. One was required: see
  "AC-1 and the rollback-recovery route".

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: `yangSectionsToValidate` covers every section `LoadConfig` parses | 17 names against 36 derived sections, six of the 17 dead | measured against the schema | list shared unchanged, gap recorded in `plan/journal/gate-excludes-part-of-its-population.md` |
| approach | The operator guide's two error examples were written from the shape of the code rather than from its output | Neither string exists; one of the two cannot exist at all, because a YANG `pattern` aborts the parse before the MAC validator runs | round-1 independent review | both replaced with text `LoadConfig` produced, pasted from a run |
| escalation | A doc claim about a sibling command (`ze doctor --json` reports the same refusals) was written without reading that command | doctor's config checks are `checkSemanticValidation` and `checkBGPPeerConfig`; neither reaches the custom validators | round-1 independent review | claim rewritten from `runChecks`. The general rule already exists (`ai/rules/evidence.md`, read the producer); no new rule is proposed |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Startup applies the registered custom validators | Done | `internal/component/config/loader.go`, `LoadConfig` | one call, after `ParseTreeWithYANG` |
| SIGHUP reload refuses and the daemon keeps its config | Done | `cmd/ze/hub/main_reload.go`, `runReload` `loadErr` branch | reached by the same `LoadConfig` error; no new machinery |
| One walk, not two | Done | `internal/component/config/validate_sections.go`, `ValidateCustomSections` | `runValidation` and `LoadConfig` are its only non-test callers |
| Upgrade entry in the operator guide | Done | `docs/guide/configuration.md` | error strings pasted from `LoadConfig` output |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done (unit), `.ci` unrun | `TestLoadConfigRefusesAnInvalidCustomValidatorValue`; `test/parse/config-startup-refuses-invalid-validator.ci` | the recovery route is closed, see the AC-1 section above |
| AC-2 | Done (unit), `.ci` unrun | `test/reload/config-reload-invalid-validator.ci` | refusal AND daemon liveness both fenced after review round 1 |
| AC-3 | Done (unit), `.ci` unrun | `TestLoadConfigRefusesAnISISHostnameOutside7BitASCII`; `test/isis/isis-hostname-startup-refused.ci` | closes the BLOCKER in `spec-fixit-isis-hostname-ascii` |
| AC-4 | Done | Q-2 measurement; `TestLoadConfigAcceptsAValidConfigUnchanged` | 7 of 1387 parsing configs fail, all 7 negative fixtures already asserting exit 1 |
| AC-5 | Done | `docs/guide/configuration.md` | every quoted string produced by the code |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLoadConfigRefusesAnInvalidCustomValidatorValue` | Done | `internal/component/config/validate_sections_test.go` | |
| `TestLoadConfigAcceptsEveryShippingExample` | Changed | Q-2 measurement + `TestLoadConfigAcceptsAValidConfigUnchanged` | the corpus walk was a one-time measurement, not a unit test over 1387 embedded texts |
| `TestLoadConfigWalksEverySectionCmdValidateWalks` | Changed | `TestLoadConfigAndConfigValidateAgree` (`.../config/cli/`) | the two callers now SHARE the list, so agreement is structural; the test asserts the same verdict on the same bytes |
| `config-startup-refuses-invalid-validator` | Written, UNRUN | `test/parse/` | |
| `config-reload-invalid-validator` | Written, UNRUN | `test/reload/` | |
| `isis-hostname-startup-refused` | Written, UNRUN | `test/isis/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/validate_sections.go` | Done | new |
| `internal/component/config/loader.go` | Done | |
| `internal/component/config/cli/cmd_validate.go` | Done | |
| `cmd/ze/hub/main.go` | Changed | not in the plan; added by review round 1 |
| `docs/guide/configuration.md` | Done | |

### Audit Summary
- **Total items:** 20
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations and in the tables above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A hand-edited config no longer reaches the wire unvalidated at daemon start | functional (`.ci`, UNRUN) + unit | `test/parse/config-startup-refuses-invalid-validator.ci`; `TestLoadConfigRefusesAnInvalidCustomValidatorValue` passes, and its input is a `type string { length "1..64" }` leaf, so only the custom validator can refuse it -- reverting the `LoadConfig` call turns it red |
| The SIGHUP reload refuses and the daemon keeps running its existing config (Thomas, Q-1) | functional (`.ci`, UNRUN) | `test/reload/config-reload-invalid-validator.ci`: the stderr expectations fence the refusal, and `action=sigterm` after the SIGHUP plus `expect=exit:code=0` fence the liveness. A daemon that died on the SIGHUP takes neither |
| The RFC 5301 section 3 path `spec-fixit-isis-hostname-ascii` depends on is enforced at startup | functional (`.ci`, UNRUN) + unit | `test/isis/isis-hostname-startup-refused.ci` (tagged `RFC requirement: RFC5301-3-7 positive`); `TestLoadConfigRefusesAnISISHostnameOutside7BitASCII` |
| Every config valid today still boots | measurement + unit | Q-2: 1387 parsing configs, 7 failures, all 7 negative fixtures already asserting exit 1; `TestLoadConfigAcceptsAValidConfigUnchanged` |
| An operator can see what will refuse BEFORE upgrading | doc + shared code path | `docs/guide/configuration.md`; `ze config validate` and `LoadConfig` call one function (`ValidateCustomSections`), so they cannot reach different verdicts. `TestLoadConfigAndConfigValidateAgree` asserts that on one input |
| A refusal never prints a secret into the startup log | unit | `TestSectionValidationErrorRedactsASensitiveLeaf` and `TestIsSensitiveLeafReadsTheProducersPathSeparator`; the second goes red on the pre-fix separator |

**Not proven yet, and it must be said plainly: the three `.ci` are UNRUN.** No
functional or QEMU suite ran in any session that wrote them. AC-1, AC-2 and AC-3
rest on unit evidence until the main thread runs the gate.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none -- the Deferral shard field is `-` | done | nothing was deferred; the section-list gap is a recorded journal class, not a deferral row |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-config-validators-bypassed-at-startup-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean rounds=2` |
| `review_gate.py check` | `review_gate: OK (clean, hashes match)` |
| Rounds | 2. Round 1 (2026-08-12): 1 BLOCKER, 3 ISSUEs, 2 NOTEs, all fixed. Round 2 is the independent pass over those fixes: 0 BLOCKER, 0 ISSUE, 2 NOTEs, both record defects, both fixed |
| Reviewer lenses used | doc-versus-producer, test discrimination, newly-reachable call paths, fail-closed guards, comment-versus-code drift |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The upgrade note told operators `ze doctor --json` reports the same refusals; doctor's config checks never reach the custom validators | `docs/guide/configuration.md` | rewritten from `runChecks` (`internal/component/doctor/doctor.go`): doctor loads through `LoadConfig`, so it now stops at `doctor-config-parse` and its other checks do not run. The guide says that and points the pre-upgrade check at `ze config validate` |
| 2 | ISSUE | Both quoted error examples were fabricated; the MAC one describes a message the code cannot produce | `docs/guide/configuration.md` | replaced with text pasted from `LoadConfig` output |
| 3 | ISSUE | The reload `.ci` proved the refusal but not that the daemon kept running | `test/reload/config-reload-invalid-validator.ci` | `action=sigterm` after the SIGHUP and `expect=exit:code=0`; header claim corrected |
| 4 | ISSUE | A validator refusal became reachable by `RecoverConfig`, which rewrites the config file and starts | `cmd/ze/hub/main.go` | `ErrCustomValidation` + `recoverableLoadError`, with two tests and a captured red |
| 5 | NOTE | The BGP-exclusion comment named a chain that is not the startup path | `internal/component/config/validate_sections.go`, spec Q-3 table | corrected in both |
| 6 | NOTE | `ValidatedSections` exported with no cross-package consumer | `internal/component/config/validate_sections.go` | unexported |

### Run 2 (independent, over the round-1 fixes)

0 BLOCKER, 0 ISSUE, 2 NOTE. Every round-1 fix was re-checked at its producing
function and all six hold. Two record defects were found and each was fixed in
one edit, so this is the last round (`ai/rules/planning.md`).

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 7 | NOTE | `TestValidatedSectionsExcludesBGP`'s comment still carried the chain finding 5 corrected: `infra.ValidateBGPPeers -> validatePeersFromTree -> PeersFromConfigTree ... at startup` | `internal/component/config/validate_sections_test.go` | comment names `PeersFromConfigTree` reached from `CreateReactorFromTree`, and calls `infra.ValidateBGPPeers` the offline door |
| 8 | NOTE | The Implementation Summary listed `validators_register_test.go` among this spec's tests; that file pins the three IS-IS validator names | this spec | line corrected: the file ships with `spec-fixit-isis-hostname-ascii` |

Checked and clean, with no finding: `parseTreeWithYANG` prunes inactive nodes
before returning, so the two callers cannot diverge on a deactivated subtree;
`ValidateTreeAllModules` never emits `ErrTypeUnknown`, so the deliberate
plugin-YANG asymmetry skips rather than rejects; `redactAll` starts true, so an
unbuildable schema redacts; `runValidation` reports
`config-validator-unavailable` with `Valid=false` where the old code waved the
config through; `go vet` over both changed packages exits 0.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/validate_sections.go` | Yes | `-rw-rw-r-- 7896 Aug 12 01:50` |
| `internal/component/config/validate_sections_test.go` | Yes | `-rw-rw-r-- 5741 Aug 12 01:49` |
| `cmd/ze/hub/main_recover_test.go` | Yes | `-rw-rw-r-- 2333 Aug 12 01:48` |
| `test/parse/config-startup-refuses-invalid-validator.ci` | Yes | `-rw-rw-r-- 2067 Aug 12 00:04` |
| `test/reload/config-reload-invalid-validator.ci` | Yes | `-rw-rw-r-- 4816 Aug 12 01:50` |
| `test/isis/isis-hostname-startup-refused.ci` | Yes | `-rw-rw-r-- 1800 Aug 12 00:04` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | startup refuses, and does not recover around the refusal | `ok github.com/ze-software/ze/internal/component/config 72.172s`; `ok github.com/ze-software/ze/cmd/ze/hub 108.556s`, carrying both `recoverableLoadError` tests |
| AC-2 | the reload refuses through the same error | `LoadConfig` error reaches `runReload`'s `loadErr` branch, read at `cmd/ze/hub/main_reload.go:193-195`; `.ci` UNRUN |
| AC-3 | an IS-IS hostname outside 7-bit ASCII is refused at load | `TestLoadConfigRefusesAnISISHostnameOutside7BitASCII` in `ok github.com/ze-software/ze/internal/component/config/cli 100.423s` |
| AC-4 | valid configs load unchanged | `TestLoadConfigAcceptsAValidConfigUnchanged`, in the config package run above |
| AC-5 | the guide carries the upgrade entry with real strings | the two quoted messages were produced by `LoadConfig` on 2026-08-12 and pasted verbatim |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze start` with an invalid config | `test/parse/config-startup-refuses-invalid-validator.ci` | Read: `exec=ze -` with the refused value, `expect=exit:code=1`, plus a second step proving `ze config validate` refuses identically |
| SIGHUP with an invalid config | `test/reload/config-reload-invalid-validator.ci` | Read: session established, rewrite, SIGHUP, SIGTERM; stderr fences the refusal and `expect=exit:code=0` fences liveness |
| Hand-edited IS-IS hostname, restart | `test/isis/isis-hostname-startup-refused.ci` | Read: UTF-8 hostname on stdin, `exit=1`, stderr names `0xc3` and `7-bit ASCII` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | 17 names against 36 derived sections, six of the 17 dead. The list is SHARED unchanged, so the two callers cannot disagree; widening is a separate problem with its own blast radius, recorded in `plan/journal/gate-excludes-part-of-its-population.md` |
| R-1 | retired | Q-2 measurement |
| R-2 | retired | Thomas's ruling of 2026-08-11 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "ze exits with status 1 ... and binds nothing" | `runYANGConfig` returns 1 on the load error before any listener is resolved (`cmd/ze/hub/main.go`) | Yes |
| the startup error string | produced by `LoadConfig` on the quoted config and pasted | Yes |
| the reload error string | `runReload` wraps the same error as `reload: parse config: %w`; `handleSIGHUPReload` prints `reload error: %v` | Yes |
| "the rules cover interface, sysctl, fib, plugin, telemetry, vpp, vpn, pki, l2tp, isis and ospf" | the 11 live names of `validatedSections`; the other six resolve to no top-level container | Yes |
| "a missing mandatory field stays a warning" | `SectionValidationError.Blocking()` returns false for `ErrTypeMissing` | Yes |
| the `ze doctor` behaviour | `runChecks` reports a `LoadConfig` failure as `doctor-config-parse` and RETURNS (`internal/component/doctor/doctor.go`) | Yes |

## Core Insight

A guard's reach and a guard's wiring are two different claims, and this repo had
a check for the second only: `validator_registry.go` proved every `ze:validate`
NAME resolves, and nothing proved any rule RUNS. The same gap has a second
instance inside this spec's own diff -- `isSensitiveLeaf` was wired, tested, and
looked for a separator the producer never writes. Both were invisible because
the test built its input instead of taking it from the function that makes it.

# Spec: improve-7-yang-handler-gate-deferred-strict-unknown-key

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-08-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/yang/validator.go` - both permissive skip sites
4. `internal/component/config/cli/cmd_validate.go` - the live config verify entry point

## Task

Deferred from `plan/spec-improve-7-yang-handler-gate.md` (Known Limitations, which
records: "Unknown-key permissiveness at verify (`validator.go`) is out of scope
(recorded as follow-up candidate in Design Insights)").

Config verify accepts keys that do not exist in the YANG schema. A misspelled leaf
inside an otherwise valid container is silently ignored rather than reported, so an
operator who types a leaf name wrong gets a clean verify and a setting that never
takes effect. This spec covers the opposite direction from improve-7's gate:
improve-7 asks "is every schema root claimed by a handler" (schema-not-in-handler),
this spec asks "is every config key present in the schema" (config-not-in-schema).

**Correction to the deferral record (verified 2026-07-16).** The permissiveness is
real and `validator.go` is still the exact line of the skip in
`validateContainerEntry`, but that function is NOT the producer for config verify:

| Path | Permissive site | Reached from | Live? |
|------|-----------------|--------------|-------|
| `validateContainerEntry` | `validator.go` (`if child, ok := entry.Dir[key]; ok` with no else) | `ValidateContainer` (`validator.go`) <- `reader.go`, `reader.go` | NO. `config.NewReader` (`reader.go`) has zero non-test callers; only `reader_test.go` constructs it |
| `walkTree` | `validator.go` (`if !ok { continue // unknown field }`) | `ValidateTreeAllModules` (`validator.go`) <- `cmd_validate.go`; `ValidateTree` (`validator.go`) <- `cli/validator.go`, `cli/validator.go` | YES. This is what `ze config validate` runs |

So the work to do is at `walkTree`, and `validator.go` is a second (currently
dead) copy of the same policy. Anyone picking this up must decide both sites
together or the policy forks. The improve-7 Design Insights already flagged the
`reader.go` block-dispatch machinery as a dead-code candidate that must be surfaced
to the user, never deleted unilaterally (`ai/rules/never-destroy-work.md`).

**Points to complete:**

1. Decide the policy: reject unknown keys, or report them as warnings. `walkTree`
   accumulates errors rather than stopping at the first, so an error type already
   fits the return shape.
2. Handle the multi-module case. `ValidateTreeAllModules` (`validator.go`)
   walks the SAME section data once per conf module and its own doc comment says
   "unknown fields from other modules are silently skipped". A per-module strict
   check would flag every other module's legitimate leaves. The check must be a
   union across modules, computed after all walks, not inside one.
3. Decide what to do with the dead `validateContainerEntry` site.
4. Decide the blast radius: an existing config with a stale or misspelled key
   starts failing verify after this lands.

**Known constraint:** the comment on the skip at `validator.go` says the unknown
field is handled elsewhere, by the config reader. That claim is untrue on the live
path (the reader is test-only per the table above).
Treat the comment as a belief, not a decision record (`ai/rules/evidence.md`),
and fix or remove it as part of this work.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - module load/resolve semantics, module categories
  → Constraint: (fill during design) the strict check must walk the RESOLVED entry tree, the same producer the daemon uses
- [ ] `ai/rules/evidence.md` - a guard that silently accepts is the failure mode being fixed
  → Constraint: (fill during design)
- [ ] `ai/rules/never-destroy-work.md` - governs the dead `reader.go` path
  → Constraint: surface the dead-code finding to the user; do not delete unilaterally
- [ ] `plan/spec-improve-7-yang-handler-gate.md` - the source spec, Design Insights and Known Limitations
  → Decision: this direction (config-not-in-schema) was recorded as a follow-up candidate, deliberately not folded into improve-7's gate

### RFC Summaries (MUST for protocol work)
- Not protocol work. YANG semantics come via goyang.

**Key insights:**
- Two permissive sites exist, only one is live. See the Task table.
- `ValidateTreeAllModules` walks one section once per module, so strictness cannot be decided inside a single walk.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/yang/validator.go` - `walkTree` (:616) skips unknown keys at :634-635 with a bare `continue`; `validateContainerEntry` (:509) skips them at :527 by only acting inside `if child, ok := entry.Dir[key]; ok`. Neither records an error. `ValidateTreeAllModules` (:599) walks a section once per conf module and its doc comment (:596-598) states other modules' fields are silently skipped
- [ ] `internal/component/config/cli/cmd_validate.go` - `ValidateContent` (:27) and `cmdValidate` (:110) are the verify entry points; :277 calls `ValidateTreeAllModules` per section in `yangSectionsToValidate` (:50)
- [ ] `internal/component/config/reader.go` - `NewReader` (:236) builds the block-dispatch reader that calls `ValidateContainer` (:329, :409); zero non-test callers, only `reader_test.go`
- [ ] `internal/component/cli/validator.go` - :236 and :293 call `ValidateTree` for `bgp` and `bgp/peer`, the CLI editor's deeper BGP path

**Behavior to preserve:**
- Mandatory-field checks keep reporting missing leaves (`walkTree` :619-629).
- Range, pattern, enum, cardinality, and `ze:validate` custom-validator errors keep their current paths and message shapes.
- `ValidateTree` returns all errors rather than stopping at the first.
- Multi-module sections (l2tp is the named case) keep validating: a leaf contributed by one module must not be reported as unknown by another module's walk.
- Sensitive-value redaction at `cmd_validate.go` keeps applying to any new error path.

**Behavior to change:**
- Config keys with no schema node become a reported error or warning at verify, instead of being silently skipped.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze config validate` (`cmd_validate.go` `cmdValidate`, `:27` `ValidateContent`), operating on the parsed config tree.
- The CLI editor's BGP validation (`internal/component/cli/validator.go`, `:293`).

### Transformation Path
1. Config file is parsed into a tree; `config.PruneInactive` drops inactive nodes (`cmd_validate.go`).
2. `cmd_validate.go` iterates `yangSectionsToValidate` and calls `ValidateTreeAllModules(section, container.ToMap())`.
3. `ValidateTreeAllModules` (`validator.go`) resolves each conf module's entry, finds the section child, and calls `walkTree` once per module.
4. `walkTree` (`validator.go`) checks mandatory children, then loops the provided data; keys absent from `entry.Dir` hit the `continue` at :634-635 and vanish.
5. Accumulated `ValidationError` values return to `cmd_validate.go`, which redacts sensitive leaves and renders text or JSON output.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree -> YANG validator | `container.ToMap()` producing `map[string]any` | [ ] |
| Validator -> multiple conf modules | `loader.ConfModuleNames()` / `GetEntry` per module | [ ] |
| Validator -> CLI output | `[]ValidationError` with Path/Type/Message | [ ] |
| Validator -> custom validators | `ValidatorRegistry` via `ze:validate` extension | [ ] |

### Integration Points
- `Validator.walkTree` and `Validator.ValidateTreeAllModules` (`internal/component/config/yang/validator.go`).
- `ValidationError` and `ErrorType` (`validator.go`, `:64`) for any new error kind.
- `cmd_validate.go` output and redaction path.
- `Validator.validateContainerEntry`, the dead second site.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding: no per-module or per-plugin key list is added to the validator; the schema stays the only source of known keys

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The union of all conf modules' entries covers every legitimate key in a section | `ValidateTreeAllModules` :599-613 doc comment and loop | Strict rejection produces false positives on valid config | Run the strict check over every config in `test/` and the demo configs | unvalidated |
| A-2 | No live path depends on unknown keys being accepted | `NewReader` has zero non-test callers (grep, 2026-07-16) | A hidden consumer breaks | Grep for non-test callers again at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Existing operator configs with stale keys start failing verify | Test configs fail on first run | Warning-first release, promote to error later |
| R-2 | The two permissive sites drift apart | Review finds one fixed, one not | Fix both, or delete the dead site with user approval |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config with a misspelled leaf | -> | strict unknown-key reporting in `walkTree` | `test/config/validate-unknown-key.ci` (fill during design) |
| CLI editor BGP validation | -> | same strict path via `ValidateTree` | `TestValidateTreeReportsUnknownKey` (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config contains a leaf name absent from every conf module's schema for that section | Verify reports it with the full path, and does not exit clean |
| AC-2 | Config contains a leaf contributed by a different conf module than the one being walked | No unknown-key report (multi-module union holds) |
| AC-3 | Config is fully valid | No new errors compared to today |
| AC-4 | An unknown key sits under a sensitive container | The report redacts the value per the existing redaction path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWalkTreeReportsUnknownKey` | `internal/component/config/yang/validator_test.go` | AC-1: an unknown leaf yields a ValidationError with the right path | |
| `TestValidateTreeAllModulesUnionOfModules` | `internal/component/config/yang/validator_test.go` | AC-2: a leaf from module B is not unknown during module A's walk | |
| `TestValidateTreeValidConfigNoNewErrors` | `internal/component/config/yang/validator_test.go` | AC-3: no regression on valid trees | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `validate-unknown-key` | `test/config/validate-unknown-key.ci` | Operator misspells a leaf and `ze config validate` names it | |

### Future (if deferring any tests)
- (fill during design)

## Files to Modify
- `internal/component/config/yang/validator.go` - strict unknown-key reporting in `walkTree` / `ValidateTreeAllModules`; decide the fate of `validateContainerEntry:527`; fix the untrue comment at :635
- `internal/component/config/cli/cmd_validate.go` - surface the new error kind if a new `ErrorType` is added
- `internal/component/config/reader.go` - only if the dead path is resolved (needs user approval)

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - add the failing unit test that asserts an unknown key is reported; confirm it fails today.
2. **Phase: Policy decision** - error vs warning, and the multi-module union point. (fill during design)
3. **Phase: Implement** - strict reporting on the live path. (fill during design)
4. **Phase: Second site** - resolve `validateContainerEntry:527` and the dead reader path with the user.
5. **Functional test** - `.ci` covering the operator-visible message.
6. **Full verification** - `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Fail-closed | An unknown key can never yield a clean verify (`ai/rules/evidence.md`) |
| Registration over hardcoding | No static key list added; the schema remains the only source of known keys |
| No drift | Both permissive sites resolved, or the dead one explicitly retired with user approval |

## Known Limitations
- (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete, every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Both permissive sites resolved or explicitly retired

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Learned summary written at closure

---

## CLOSED 2026-08-05: the premise is false. Config verify already rejects unknown keys.

**This spec is closed as no longer relevant, not implemented.** Its Task says an
operator who misspells a leaf "gets a clean verify and a setting that never takes
effect". That is not what happens. Both formats `ze config validate` accepts
reject a misspelled leaf with a precise, line-numbered error.

Measured against `bin/ze`, not read from source. Block format, `ruoter-id` for
`router-id` inside `session`:

```
$ ze config validate typo.conf
configuration invalid: typo.conf

Errors:
  line 9: line 9: unknown field in session: ruoter-id (line 9)
```

Set format, the same typo:

```
$ ze config validate typo.set
configuration invalid: typo.set

Errors:
  line 4: parse config: config contains fields unknown to this build (a feature
  compiled out of this binary, or a legacy field this schema no longer defines);
  refusing to load a config that would silently drop them: line 4: unknown
  field: ruoter-id (needs migration)
```

Both exit 1.

### Why the premise died

`runValidation` (`internal/component/config/cli/cmd_validate.go`) branches on
format and BOTH branches now fail closed before validation is ever reached:

| Branch | Producer | Behavior |
|--------|----------|----------|
| Block | `Parser.Parse` (`internal/component/config/parser.go`) | Errors with "unknown field in %s: %s". `runValidation` returns on the parse error, so `ValidateTreeAllModules` is never called |
| Set, set-meta | `ParseTreeForValidation` to `parseTreeWithYANG` (`internal/component/config/loader.go`) | Collects each unknown field as a warning, then refuses: "refusing to load a config that would silently drop them" |

The set-format guard is the decisive one and it was added for a different reason.
Its comment names the fail-open it closes: a build with a feature compiled out
would boot a committed set-meta config minus its gated blocks, and "tacacs/radius
authentication silently degrading to local auth was the concrete fail-open this
closes (feature-gate-12 review)". Fixing that closed this spec's gap as a side
effect, which is why nobody recorded it here.

### What is still true, and why it does not reopen this

The permissive skip this spec named IS still in the code. `walkTree`
(`internal/component/config/yang/validator.go`) still continues past a key the
schema does not know, and `validateContainerEntry` still has the same policy with
no `else`. Both are now UNREACHABLE with an unknown key from the validate path,
because the parse layer above them rejects it first.

That makes them dead policy rather than a live defect, and dead policy is not this
spec's subject. Do not reopen this file for them.

### The one thing worth carrying forward

Point 3 of the original work list asked what to do with the dead
`validateContainerEntry` site, and the improve-7 Design Insights flagged the
`config.NewReader` block-dispatch machinery as a dead-code candidate that "must be
surfaced to the user, never deleted unilaterally"
(`ai/rules/never-destroy-work.md`).

Re-checked 2026-08-05 and still true: `config.NewReader` has zero non-test
callers. That is a deletion question for Thomas, it is unchanged by this closure,
and it is NOT gated on anything here. It is recorded in this closure commit so it
survives the spec being removed.

Points 1, 2 and 4 are void: they asked which policy to adopt, how to make it work
across modules, and what the blast radius of adopting it would be. The policy is
already adopted, one layer up, where no multi-module union is needed because the
parser knows the whole schema at once.

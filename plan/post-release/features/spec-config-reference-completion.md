# Spec: config-reference-completion -- complete and validate every reference to a named config entry from one declaration

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P3 |
| Release reason | Editor capability. The cross-user race it removes is a release defect owned by `plan/release/fixes/spec-web-cli-config-one-off-defects.md` (journal `per-request-state-on-a-shared-object`); phase 1 here is that fix and lands first. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A config leaf that names an entry elsewhere in the config (a policy filter, a
BFD profile, an OSPF key-chain, an IKE group, a firewall chain) gets no editor
completion today, and most of them are plain strings checked by hand-written Go
in their component. The goal: every such reference is declared once in YANG,
and that one declaration drives both editor completion (CLI, web, `ze config
completion`) and commit refusal of a dangling reference.

Owner decisions (2026-10-30):

| Decision | Outcome |
|----------|---------|
| Spec | Separate from `spec-route-policy-0-umbrella`, which moved its completion row here |
| Scope | The mechanism and every string reference in the tree, in three ordered phases |
| Declaration | Standard YANG `leafref` with `require-instance` where a path expresses the reference; one ze extension for a reference no path can express (policy filter names span every `ze:filter` list under `policy`) |
| Tree access | The candidate tree is an argument of each completion call, never state on a shared completer |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/config-editor.md` - "YANG Completion"
  → Constraint: completion today offers schema children, list keys from the tree, enum/bool values, a type hint, and a `ze:validate` CompleteFn's fixed list. Nothing completes a value from config content.
- [ ] `ai/patterns/config-option.md` - steps 5 and 5b
  → Constraint: `RegisterSuggestion` and `RegisterCompleteFn` take no tree; they stay for registry-backed values (plugin names, interface names) and are not the reference mechanism.
- [ ] `docs/architecture/web-interface.md` - the web CLI and its completion handler
  → Constraint: the web server serves every logged-in user from one process; per-user editor state lives in `EditorManager` sessions, so nothing a request reads may be stored on an object other users share.
- [ ] `docs/architecture/config/yang-config-design.md` - the ze extension table
  → Decision: a new extension is added to that table with its contract.

**Key insights:**
- `internal/component/cli/completer.go`: `matchChildren` drops the data path and tree before `valueCompletions(entry, prefix)`; `validateCompletions` calls `CompleteFn()` with no argument; `c.tree` serves only list keys.
- The web server creates ONE completer (`cmd/ze/hub/service_web.go`) and `completeCLIInput` (`web/cli.go`) calls `SetTree` per request with no lock: a cross-user race.
- Ze already resolves leafref paths and refuses a dangling one at commit: `config/yang/reference_path.go` `leafrefTarget`, `validator_instance.go` `ValidateReferences`, `instancesAt`, `pathValues`, `stepNodes`; `absentLeafrefMessage` already lists the configured instances.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/completer.go` - `Complete`, `completeSetPath`, `matchChildren`, `valueCompletions`, `validateCompletions`, `listKeyCompletions`, `SetTree`
- [ ] `internal/component/cli/contract/contract.go` - `Completer` interface (`SetTree`, `Complete`)
- [ ] `internal/component/web/cli.go` - `completeCLIInput`; `web/editor.go` `EditorManager.Tree` releases the session lock before the caller reads the tree
- [ ] `cmd/ze/hub/service_web.go` - the one shared completer
- [ ] `internal/component/config/cli/cmd_completion.go` - `completerFor`
- [ ] `internal/component/config/yang/` - `loader_reference.go`, `reference_path.go`, `validator_reference.go`, `validator_instance.go`
- [ ] `internal/component/policy/registry.go` - `BuildRegistry`, `Names`; `chain.go` `CanonicalName`

**Behavior to preserve:** schema, key, enum and CompleteFn completion as today; leafref commit refusal and its message; every refusal a converted component gives today (wording may move to the leafref message, which names the configured choices).

**Behavior to change:** a reference leaf completes the configured entries it may name; each converted string reference becomes a leafref (or the extension) and its hand-written Go existence check is deleted (no layering).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A completion request: TUI keystroke, web CLI `HandleCLICompleteWithCommandCompleter`, `ze config completion`.

### Transformation Path
1. The caller passes its candidate tree to `Complete` (no `SetTree`).
2. `matchChildren` hands the data path and the tree to `valueCompletions`.
3. For a leaf whose schema carries a leafref, the yang validator answers `ReferenceInstances(root, leafPath)` by the same walk commit validation uses; for the extension, the registered reference source answers.
4. Commit validation reads the same declaration, so a value completion offers is exactly a value commit accepts.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web request to completer | tree passed per call; the completer holds no per-user state | No |
| Completer to yang validator | `ReferenceInstances(root map[string]any, leafPath []string) []string` over `Tree.ToPluginMap()` | No |

### Integration Points
- `config/yang` `referenceWalk` / `instancesAt` - reused, not duplicated.
- `internal/component/policy` - registers the `policy-filter` reference source (union of `ze:filter` instances, filtered by the attachment's field contract).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | completion and commit both read the YANG declaration |
| No unintended coupling | Yes | the completer asks the yang validator or a registered source; it imports no component |
| No duplicated functionality | Yes | the leafref walker is reused; per-component existence checks are deleted |
| Zero-copy preserved where applicable | Yes | the tree is lowered once per completion call |
| Registration over hardcoding, outbound | Yes | the extension names a registered reference source |
| Registration over hardcoding, inbound | Yes | the completer has no list of reference leaves; it reads the schema |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every listed string reference except policy filter names is expressible as a leafref path | research inventory | a reference needs the extension too | phase 3 converts each and its commit test proves the refusal | unvalidated |
| A-2 | Lowering the tree per completion call is fast enough for interactive Tab | `ToPluginMap` is already run per commit | Tab lags on large configs | a completion benchmark on a large config, run on the Mac by Thomas | unvalidated |
| A-3 | The web editor has no completion path other than the CLI completer | research grep of web/ | a second path keeps the race | grep in phase 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Converting a leaf to leafref changes a refusal message operators or tests rely on | a .ci expectation fails | the leafref message names the leaf and the configured choices; each changed expectation goes to the test judge |
| R-2 | A leafref across a list with templated or derived names (dynamic groups) refuses a valid config | commit test | keep that leaf on the extension with a source that knows the derivation |
| R-3 | Completion offers instances a later check refuses (e.g. a policy filter whose fields do not fit the attachment) | test | the policy source filters by the attachment's field contract, the same check verify runs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a config that loaded before is refused (phase 3), or completion offers wrong names |
| How is it reverted? | per phase, single commits |
| Who else touches this path? | `spec-route-policy-0-umbrella` (policy filters), components whose references convert |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| TUI Tab on a leafref value | → | `valueCompletions` → `ReferenceInstances` | `test/editor/completion/value-leafref-class.et` |
| TUI Tab on `filters [ ` | → | policy-filter reference source | `test/editor/completion/value-policy-filters.et` |
| web CLI completion, two users at once | → | per-call tree | `TestWebCompletionPerUserTree` (`web/cli_test.go`, -race) |
| `ze config completion` | → | same path | `test/parse/completion-reference.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two web users with different candidate configs complete concurrently | each sees only names from their own tree; no data race under -race |
| AC-2 | Tab on traffic-control `default-class` with classes a, b configured | offers a and b |
| AC-3 | Tab on `isis filter import filters [ ` with prefix-list P and community-match C defined | offers P (fields fit IS-IS), not C |
| AC-4 | Tab on BGP `filters [ ` | offers every filter instance whose fields fit a BGP attachment |
| AC-5 | Each converted reference (bfd profile, ospf key-chain, ipsec ike-group/esp-group, authz/tacacs profile, firewall jump/goto, geodns set, policy match prefix-list, pki certificate) with its target configured | Tab offers the configured names |
| AC-6 | Each converted reference naming an entry that is not configured | commit refused naming the leaf, the value and the configured choices; the old per-component Go check is gone |
| AC-7 | A config that named only existing entries before phase 3 | loads unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Tabs a filter name | editor → completer → policy source | `value-policy-filters.et` |
| 2 | Tabs a BFD profile on a peer | editor → leafref walk | `value-bfd-profile.et` |
| 3 | Commits a typo in a key-chain name | commit → leafref refusal | `test/parse/reference-keychain-missing.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCompleterTreePerCall` | `internal/component/cli/completer_test.go` | no tree state on the completer | |
| `TestReferenceInstances` | `internal/component/config/yang/rfc7950_reference_test.go` | instances at a data position, `current()` predicates | |
| `TestPolicyFilterReferenceSource` | `internal/component/policy/` | union of filter instances, filtered by attachment fields | |
| `TestWebCompletionPerUserTree` | `internal/component/web/cli_test.go` | AC-1 under -race | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `value-leafref-class` | `test/editor/completion/` | AC-2 | |
| `value-policy-filters` | `test/editor/completion/` | AC-3, AC-4 | |
| `value-bfd-profile` | `test/editor/completion/` | AC-5 | |
| `reference-*-missing` | `test/parse/` | AC-6, one per converted reference | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| none | N/A | N/A | config-only feature with no protocol impact | |

## Files to Modify
- `internal/component/cli/completer.go`, `cli/contract/contract.go`, `cli/model.go` - tree per call; reference completion
- `internal/component/web/cli.go`, `cmd/ze/hub/service_web.go` - no shared tree state
- `internal/component/config/cli/cmd_completion.go` - tree per call
- `internal/component/config/yang/validator_instance.go` - exported `ReferenceInstances`
- `internal/component/config/yang/modules/ze-extensions.yang` - the reference extension
- `internal/component/policy/` - policy-filter reference source; ze-policy-conf.yang and ze-bgp-conf.yang `filters` leaves declare it
- phase 3: ze-bfd-conf, ze-bgp-conf (bfd profile), ze-ospf-conf, ze-ipsec-conf, ze-authz-conf, ze-tacacs-conf, ze-firewall-conf, ze-geodns-conf, ze-policy-conf (match prefix-list), PKI certificate leaves in web, lg, ipsec, geodns, as112, rpki, plugin; and each component's hand-written existence check, deleted

## Files to Create
- `test/editor/completion/value-leafref-class.et`, `value-policy-filters.et`, `value-bfd-profile.et`
- `test/parse/completion-reference.ci`, `test/parse/reference-*-missing.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | leafref conversions; the extension in `ze-extensions.yang` |
| YANG validation constraints | Yes | `require-instance` on each leafref |
| YANG custom validators | Yes | the reference extension binds a registered source |
| CLI commands/flags | No | no command added |
| CLI grammar (keyword before value) | No | no command added |
| Editor autocomplete | Yes | the feature |
| Functional test for new RPC/API | No | no RPC |
| Pipe completeness | No | no output command |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the config editor feature entry |
| 2 | Config syntax changed? | No | values unchanged; refusal wording may change |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` "YANG Completion" |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7950 leafref semantics already implemented |
| 10 | Test infrastructure changed? | No | existing .et runner |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` extension table |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per phase by `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/patterns/config-option.md` steps 5 and 5b |

## Implementation Steps

1. **Phase 1: tree per call (fixes the web race)** - `Complete` takes the tree; `SetTree` and per-user state on the completer are deleted; web, TUI and `ze config completion` pass their tree. Tests: `TestCompleterTreePerCall`, `TestWebCompletionPerUserTree`. Closes the journal row owned by `spec-web-cli-config-one-off-defects`.
2. **Phase 2: reference completion** - `ReferenceInstances` on the yang validator; `valueCompletions` asks it for a leafref; the reference extension and the policy-filter source; policy and BGP `filters` declare it. Tests: `TestReferenceInstances`, `TestPolicyFilterReferenceSource`, the three `.et` files.
3. **Phase 3: convert every string reference** - one commit per component: leafref (or the extension), delete the component's existence check, `reference-*-missing.ci`. Changed refusal expectations go to the test judge.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | completion offers exactly what commit accepts, for every reference leaf |
| Data flow | no completer holds a tree between calls |
| Rule: no-layering | each converted component's own existence check is deleted in the same commit |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no tree state on the completer | `grep -n "SetTree" internal/component/cli` returns nothing |
| converted references | each `reference-*-missing.ci` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Cross-user isolation | a web user's completion reads only that user's tree (AC-1 under -race) |
| Input validation | a dangling reference is refused at commit for every converted leaf |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Commit validation already walks leafrefs and lists the configured choices; completion is that walk run at a cursor instead of over the whole tree.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Standard leafref first, one extension only where no path fits | a ze extension for every reference | leafref is standard, already validated at commit, and readable by any YANG tool |
| Tree as a per-call argument | a lock around `SetTree` | a lock serialises every user's Tab; an argument removes the shared state |
| Convert all string references (owner) | mechanism only | one declaration then serves every reference; the hand-written checks go |

## Known Limitations
- Registry-backed values (plugin names, interface names) keep their existing CompleteFn/suggestion sources: they name things outside the config.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.et` / `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only

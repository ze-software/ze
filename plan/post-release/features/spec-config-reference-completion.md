# Spec: config-reference-completion -- complete and validate every reference to a named config entry from one declaration

| Field | Value |
|-------|-------|
| Status | in-progress |
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
| `fixture-encodes-an-impossible-state` | 2026-10-30 | paths-limit-live names an undeclared admin profile |

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

## Implementation Summary

### What Was Implemented
- Phase 1 (cd02133374): `Completer.Complete`, `GhostText` and `Backends` take the candidate tree per call; per-call state lives in `completion` (`(*Completer).over`). `SetTree` and `contract.Completer` are deleted. The web completes inside `EditorManager.ReadTree`, so a user's completion reads only that user's tree.
- Phase 2 (c238155e95, 32015956fd): `Validator.ReferenceInstances` (`config/yang/reference_instances.go`) runs the commit reference walk at one position; `completion.referenceCompletions` asks it before CompleteFn, bool, enum and hint. The `ze:reference` extension (`ze-extensions.yang`) names a registered `yang.ReferenceSource` (`reference_source.go`), whose `ReferenceAnswer{Names, Check}` drives both completion and commit. `policy.filterReference` is the `policy-filter` source, filtered by each attachment's member check (`MustRegisterChainLeaf`, `ribMemberCheck`). Completion continues inside a leaf-list's `[`.
- Phase 3, one commit per component: OSPF (2ff4d9ae4b) and IS-IS (b8bbca16ad) key-chain leafrefs; BFD session profile leafref plus the `bfd-profile` source for BGP and static (3c8c2ae886); authz user and tacacs-profile leafrefs (d6c44d5fe0, with `childrenOf` so a path under a shared top-level container finds every module's copy); IPsec ike-group and esp-group leafrefs (c41ebbaf93); firewall jump/goto leafrefs (8de2916862); geodns host-set leafref (c3eaaad417); the policy match `prefix-list` source (bc9b54357b); `pki-certificate` and `pki-ca` sources in `pki/pkiref` on eighteen leaves (05646d85eb). Each hand-written existence check was deleted except where Deviations records it.
- Closure: `test/ui/cli-completion-value-reference.ci`, the wiring test for the `ze config completion` door, which the implementation had not written.

### Bugs Found/Fixed
- Cross-user completion race in the web CLI (journal `per-request-state-on-a-shared-object`): fixed in phase 1, covered by `TestWebCompletionPerUserTree` (-race; red before with 2 DATA RACE reports and leaked peers).
- A tacacs-profile leafref into the authz module answered `<leafref>` because `leafFrames` looked under the first module's `system` copy only: fixed by `childrenOf` (d6c44d5fe0), covered by `value-tacacs-profile.et`.
- Two IPsec PKI leaves were compared against nothing before; `ze:reference` now checks them (05646d85eb), covered by `reference-ipsec-second-round-*-missing.ci`.

### Documentation Updates
- `docs/guide/config-editor.md` "YANG Completion" (cd02133374, c238155e95, 32015956fd, 99da7360da, bc9b54357b, 05646d85eb), anchors `completer.go -- valueCompletions, referenceCompletions`, `reference_instances.go -- ReferenceInstances`, `policy/reference_source.go -- filterReference`, `web/cli.go -- completeCLIInput`.
- `docs/architecture/config/yang-config-design.md` extension table: the `ze:reference` row (c238155e95, 32015956fd).
- `docs/architecture/web-interface.md` "Per-User Editor" (cd02133374); `ai/patterns/config-option.md` steps 5 and 5b (c238155e95, 32015956fd); `features/backend-aware-cli-completion.md` (cd02133374).
- Per-component guides: `route-policy.md`, `bfd.md`, `static-routes.md`, `isis.md`, `authorization.md`, `ipsec.md`, `firewall.md`; architecture `ipsec-3-data-model.md` and `pki/pki-store.md`.
- `./le doc check all` at HEAD c13129b990 in a throwaway worktree fails on 5 source anchors, none from this spec: `interop.md` (assignLinkLocal, twice), `rfc-conformance-gates.md` (genericRoleWords, roleKey), `ipsec.md` (creationReplayOps, an anchor 75473971b5 added).

### Deviations from Plan
- The wiring test for `ze config completion` is `test/ui/cli-completion-value-reference.ci`, beside the other `ze config completion` tests, not `test/parse/completion-reference.ci`. User story 3's `reference-keychain-missing.ci` is `reference-ospf-key-chain-missing.ci`, and each converted reference has its own `reference-<component>-missing.ci`.
- BGP and static BFD profile, policy filters, policy match prefix-list and PKI names use `ze:reference`, not leafref: the target module can leave the build (R-2), or the names span several lists.
- Go checks kept, each judging more than existence: `firewall.validateAction` jump/goto (it also judges the programmatic tables copp `translate.go` and flowspec-firewall `rule_chains.go` build, which no YANG walk sees); `policy.parseMatchPrefixList` (the lookup that compiles the evaluator); `bgp/config.validateChainNames` (it is the `policy-filter` source's member check, and `peers.go` still calls it per peer); `bfdapi.CheckProfile` (the static and BGP mode and authentication check).
- `bfd.(*pluginConfig).validate` continues on an unknown profile instead of refusing it, annotated refuse-by-default: the leafref refuses that profile at commit first.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation shipped without the `ze config completion` wiring test the Wiring Test table names | the door worked, but nothing proved it | closure deliverables review | closure wrote `test/ui/cli-completion-value-reference.ci`, recorded red with `cmdCompletion` passing a nil tree |
| approach | Two RFC-tagged IPsec tests were edited before the judge approved them (c41ebbaf93 body says so) | the judge rules before the edit | c41ebbaf93 commit body | the approvals were recorded after the fact; no closure action |
| approach | Phase 3 changed tagged units of RFC4301-4.4.1-6 (c41ebbaf93), RFC5882-4.4-1 (3c8c2ae886) and RFC7296-2.16-11 (05646d85eb) and left their audit verdicts stale, and the RFC5882-4.4-1 discrimination record with them | the implementation phase re-takes the proof records of every tagged unit it changes, before closure starts | closure step 6, `./le rfc check` stale audit lines | the closure was handed back; an independent auditor re-judged the three verdicts (none changed) and `./le rfc record` re-recorded RFC5882-4.4-1; the four records ride in commit A |
| approach | Phase 3's `profile [ admin ]` fixture sweep fixed 18 of 19 fixtures and missed `test/plugin/paths-limit-live.ci` | every fixture naming a profile must declare it once the profile is a leafref | another session's red triage, journal `fixture-encodes-an-impossible-state` | closure added the `authorization profile admin` block, the row is `fixed:` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The tree is a per-call argument, never state on a shared completer | Done | `internal/component/cli/completer.go` `Complete`, `over`; `web/cli.go` `completeCLIInput` | `grep -rn SetTree internal/component/cli` is empty |
| Every reference declared once in YANG (leafref or `ze:reference`) | Done | the YANG modules under What Was Implemented | |
| One declaration drives completion | Done | `completion.referenceCompletions` -> `Validator.ReferenceInstances` | |
| One declaration drives commit refusal | Done | `config.referenceFailures` -> `Validator.ValidateReferences`, reached from `LoadConfig` (`refuseInvalidCustomSections`), the editor validator (`cli/validator.go`) and `ze config validate` (`cmd_validate.go`) | |
| Hand-written existence checks deleted | Changed | Deviations | four Go checks kept, each judging more than existence |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestWebCompletionPerUserTree` (-race) | |
| AC-2 | Done | `value-leafref-class.et`; `cli-completion-value-reference.ci` seq 1 | |
| AC-3 | Done | `value-policy-filters.et` (isis: contains P, excludes C) | |
| AC-4 | Done | `value-policy-filters.et` (bgp: contains C,P) | |
| AC-5 | Done | 17 `value-*.et` this spec added | |
| AC-6 | Done | 18 `reference-*-missing.ci` | Go checks kept per Deviations |
| AC-7 | Done | unchanged fixtures pass; 17 fixtures gained a `pki` block for names they already used (judged) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCompleterTreePerCall` | Done | `internal/component/cli/completer_test.go` | |
| `TestReferenceInstances` | Changed | `internal/component/config/yang/reference_instances_test.go` | the file name differs from the plan |
| `TestPolicyFilterReferenceSource` | Done | `internal/component/policy/reference_source_test.go` | |
| `TestWebCompletionPerUserTree` | Done | `internal/component/web/cli_test.go` | |
| `value-leafref-class`, `value-policy-filters`, `value-bfd-profile` | Done | `test/editor/completion/` | |
| `reference-*-missing` | Done | `test/parse/` (18 files) | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cli/completer.go`, `cli/model.go`, `cli/contract/contract.go` | Done | cd02133374, c238155e95, 32015956fd |
| `web/cli.go`, `cmd/ze/hub/service_web.go` | Done | `web/cli.go` and `web/editor.go` changed; `service_web.go` needed no change once the completer holds no tree |
| `config/cli/cmd_completion.go` | Done | cd02133374 |
| `config/yang/validator_instance.go`, `ze-extensions.yang` | Done | c238155e95 |
| `internal/component/policy/` | Done | 32015956fd, bc9b54357b |
| phase 3 modules | Done | the per-component commits |
| `test/parse/completion-reference.ci` | Changed | `test/ui/cli-completion-value-reference.ci` |

### Audit Summary
- **Total items:** 25
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A reference leaf completes the configured entries it may name, in the CLI, the web and `ze config completion` | functional | `value-leafref-class.et`, `value-policy-filters.et` and 17 `value-*.et` (TUI); `TestWebCompletionPerUserTree` (web); `test/ui/cli-completion-value-reference.ci` (CLI door, red with a nil tree: `output does not match regex "voice +configured default-class"`) |
| The same declaration refuses a dangling reference at commit | functional | 18 `test/parse/reference-*-missing.ci`, each recorded red with its declaration reverted (the phase 3 commit bodies) |
| No cross-user leak in web completion | unit, -race | `TestWebCompletionPerUserTree`: red before with DATA RACE at `setTreeInternal` and `bob missing own peer` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC and wiring row is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/config-reference-completion-b253ee28-af02-45d1-a862-79a55bdba37c.md` (the spec, `test/ui/cli-completion-value-reference.ci`, the three re-judged `rfc/audit/` verdicts,, the RFC5882-4.4-1 discrimination record, the `authorization profile admin` hunk of `test/plugin/paths-limit-live.ci` and its now-fixed journal row: the files commit A carries; the implementation is already in the thirteen commits named under What Was Implemented) |
| `./le spec review check` | clean |
| Rounds | 3 |
| Reviewer lenses used | wiring and entry points, no-layering, concurrency, security (cross-user isolation, input validation), docs against source, Go style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The Wiring Test row for `ze config completion` had no test: nothing proved `cmdCompletion` passes the loaded tree to `Complete`, so a nil tree there would answer only the type hint with every test green | `internal/component/config/cli/cmd_completion.go` `cmdCompletion` | `test/ui/cli-completion-value-reference.ci` (a leafref and a `ze:reference` leaf), red with `Complete(nil, ...)`, green restored |

### Run 1
- 1 ISSUE (above). NOTEs: `bfd.validate`'s continue on an unknown profile is unreachable after the leafref, and annotated; `validateChainNames` runs twice for a BGP peer (once as the source's member check, once in `peers.go` step 3b2), the same function, not a second implementation; the `writeChoices` comment reads "sorted names at most"; `referenceCompletions` builds a `yang.NewValidator` per reference completion. Style pass over the new Go: every `panic` is an init-time `BUG:` registration guard (`RegisterReferenceSource`, `MustRegisterChainLeaf`), none reachable from a peer or an operator value.

### Run 2
- 0 BLOCKER, 0 ISSUE over the union of the thirteen implementation commits plus the closure test.

### Run 3
- Over what changed since run 2: the spec's closure edits and the four `rfc/` records. 0 BLOCKER, 0 ISSUE. No `verdict` field changed; only the notes, the unit hashes, and the RFC5882-4.4-1 `unit-sha` and `break` moved. The changes are record-only, so this is the last round. The owned journal row (paths-limit-live names an undeclared admin profile) was a fixture phase 3's profile sweep missed; closure adds the same three-line `authorization` block the other 18 fixtures carry, assertions untouched, and `./le test bgp plugin --pattern paths-limit-live` passes 1/1.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/editor/completion/value-leafref-class.et`, `value-policy-filters.et`, `value-bfd-profile.et` | Yes | ls at closure |
| `test/ui/cli-completion-value-reference.ci` | Yes | written and run at closure |
| `test/parse/reference-*-missing.ci` | Yes | 18 files, ls at closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | per-user tree, no race | `go test -race -count=1` at c13129b990: config/yang, cli, web, policy ok (TestWebCompletionPerUserTree, TestCompleterTreePerCall, TestCompleterReference*, TestReferenceInstances, TestReferenceSource*, TestPolicyFilterReferenceSource) |
| AC-2 | default-class offers the classes | `./le test ui --pattern cli-completion-value-reference`: pass 1/1 |
| AC-3, AC-4, AC-5 | filters per attachment; each converted reference completes | `./le test editor --pattern value-` at c13129b990: pass 21/21 (value-policy-filters, value-leafref-class and every `value-*` reference .et) |
| AC-6, AC-7 | each dangling reference refused; existing configs load | `./le test bgp parse --pattern reference-` at c13129b990: pass 18/18; `--pattern ipsec-` (the PKI-fixture configs that load): pass 28/28. Owed on Linux, skipped on macOS and not run here: `test/reload/tx-ipsec-eap-mschapv2-requires-ca.ci`, `tx-ipsec-eap-tls-requires-ca.ci`, `tx-ipsec-intermediate-certificate-requires-eap-tls.ci`, `tx-ipsec-x509-requires-ca.ci` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| TUI Tab on a leafref value | `test/editor/completion/value-leafref-class.et` | read: edits the qdisc, Tab on `set default-class`, expects data,voice |
| TUI Tab on `filters [ ` | `test/editor/completion/value-policy-filters.et` | read: isis contains P, excludes C; bgp contains C,P |
| web CLI, two users | `TestWebCompletionPerUserTree` | read: two users, one handler, one completer, 50 rounds |
| `ze config completion` | `test/ui/cli-completion-value-reference.ci` | read and run: seq 1 leafref, seq 2 `ze:reference`; red with a nil tree |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | BGP and static bfd profile, policy match prefix-list and PKI names could not be leafrefs from every build (R-2) and took `ze:reference`; Deviations records it |
| A-2 | confirmed | closure, on the Mac: `ze config completion` over a 56261-line config (3000 pki certificates, 50 qdiscs of 200 classes), three runs each: `default-class` (200 names) 0.49-0.94s, `certificate` (3000 names) 0.56-0.70s, the non-reference `type` 0.50-0.60s, whole process including the parse. A process-level timing on a loaded machine, not a `./le perf` benchmark |
| A-3 | confirmed | phase 1: `completeCLIInput` was web's only `SetTree` caller; `cli_terminal.go` uses `mgr.Tree` for non-completion work |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| config-editor.md "YANG Completion": completion and commit run the same check | `completion.referenceCompletions` calls `Validator.ReferenceInstances`, which runs `newReferenceWalk` as `ValidateReferences` does | Yes |
| yang-config-design.md `ze:reference` row | `reference_source.go` `RegisterReferenceSource`, `ReferenceAnswer`, `checkSourceInstance`, `CheckAllReferenceSourcesRegistered` | Yes |
| pki-store.md Decisions paragraph | `pkiref/reference.go` `certificateReference`, `caReference`; `pki/register.go` imports pkiref | Yes |
| RFC status (No) | RFC 7950 leafref semantics were already implemented; no `rfc/short` support row changed; RFC4301-4.4.1-6 discrimination re-recorded in c41ebbaf93 | Yes |
| RFC proof records of the units this spec changed | closure re-judge: `rfc/audit/rfc4301/RFC4301-4.4.1-6.json`, `rfc/audit/rfc5882/RFC5882-4.4-1.json`, `rfc/audit/rfc7296/RFC7296-2.16-11.json` re-taken by an independent auditor against `rfc/full/` (no `verdict` changed, unit hashes refreshed); `rfc/discrimination/rfc5882/RFC5882-4.4-1.json` re-recorded by `./le rfc record` (`unit-sha` 06a9209e49622d8d) | Yes |
| `./le rfc check` after the re-judge | 2026-10-30 in main: exit 1 on 69 STALE lines, none naming RFC4301-4.4.1-6, RFC5882-4.4-1, RFC7296-2.16-11 or a unit any of this spec's thirteen commits changed. The two in IKE stems (RFC3948-5.1-1 over the `rfc3948_inner_selector_*` tests, RFC4301-5.1-7 over `engine/child.go` `installChildSA`) trace to other sessions' commits and uncommitted edits, the rest to BGP, LDP, VRRP and other stems this spec did not touch | Yes |

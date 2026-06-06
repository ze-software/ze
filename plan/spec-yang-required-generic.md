# Spec: Generic ze:required Enforcement

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | RESEARCH |
| Updated | 2026-06-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/510-yang-required.md` - original ze:required design + rationale
4. `internal/component/bgp/config/resolve.go` (`CheckRequiredFields`), `internal/component/cli/validator.go` (`validateWithYANG`/`validatePeer`), `internal/component/config/yang_schema.go` (extension parsing), `internal/component/web/handler_config.go` (web POST enforcement)

## Task

`ze:required` is documented (`ze-extensions.yang:186`, `docs/features/configuration.md:33`)
as a hard, post-inheritance requirement validated at `ze config validate`, editor commit,
and daemon startup. In practice enforcement is **BGP-specific**, and a second usage form is
**silently unenforced**:

- **Path form** (`ze:required "connection/remote/ip"`, list-level with a descendant path):
  enforced via `CheckRequiredFields` (BGP-only, `schema.Get("bgp")` → peer list) reached on
  daemon startup and `ze config validate` through peer construction, plus the editor
  (`cli/validator.go`, warning) and the web add form (`handler_config.go`, hard reject — the
  only generic path). No mechanism walks **non-BGP** lists at validate/startup/editor; only
  web covers them.
- **Bare form** (`ze:required;`, no argument, on a leaf — ipsec 10 sites, pki 2, l2tp 1):
  off-design (per `510`, `ze:required` is list-level-with-path by design) and a silent
  no-op — `yang_schema.go` splits the empty argument and the `fields[0] != ""` guard drops
  it, so it populates nothing and is enforced nowhere. Those leaves claim to be required but
  are not.

**Goal:** a generic, schema-driven `ze:required` enforcement that (1) covers path-form on
**any** list at `ze config validate`, editor commit, and daemon startup uniformly (not just
BGP), and (2) resolves the bare leaf-mandatory form deliberately (migrate to YANG-native
`mandatory true`, or implement leaf-level required), with a decided severity (keep the
warn/skip "allow partial config during editing" semantics vs hard error) and inheritance
handling (bgp→group→peer merge). Reconcile the doc to match reality.

**Out of scope:** interface `mac` is intentionally NOT required (removed in commit
`19fcae45b`). `unique "mac/address"` stays web-only; generic `unique` enforcement is a
separate concern (note for a follow-up spec, not this one).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `plan/learned/510-yang-required.md` - origin of `ze:required`/`ze:suggest`
  → Decision: `ze:required` is deliberately **list-level with a path argument**, NOT leaf-level — "a leaf cannot know if it's at group level (optional) or peer level (required post-resolve)". Bare leaf-level `ze:required;` is therefore off-design.
  → Decision: enforcement points are `CheckRequiredFields` (resolve.go, reached via `PeersFromConfigTree`), `cli/validator.go` (editor, warnings only — never blocks editing), and `handler_config.go` (web POST). Runtime guards in reactor are last-resort, not config validation.
  → Constraint: "New required fields can be added to any list by adding `ze:required "path/to/field"` in YANG — no Go code changes needed" was the design intent; reality only delivers this for the BGP peer list (CheckRequiredFields is BGP-hardcoded) + web. Generic delivery is the gap.
  → Constraint: `mergeGroupDefaults` is shallow at depth > 2; the bgpTree fallback masks this for current fields but risks false warnings for deeply-nested group-only required fields. A generic walk must preserve this behavior.
  → Constraint: `YANGSchema()` is not cached (re-parses each call) — cannot be called from inside `ResolveBGPTree`. A generic check must take the schema as a parameter.
- [ ] `docs/features/configuration.md` (lines 9-40) + `docs/architecture/config/syntax.md` (line 977) - documented ze:required contract
  → Constraint: doc claims "Validated at `ze config validate`, editor commit, and daemon startup" — accurate for BGP path-form, inaccurate as a general statement (non-BGP path-form + bare form are not). Doc must be reconciled to match the implemented generality.
- [ ] `ai/rules/config-design.md` - "Fail on unknown keys ... No silent ignore." + grouping/augment rules
  → Constraint: silent no-op of bare `ze:required;` violates the project's "no silent ignore" stance.
- [ ] `ai/rules/error-messages.md` - name subject + offending value + corrective action; one stable greppable phrase
  → Constraint: a missing-required message must name the list entry + the missing path + a `set ...` hint (the existing peer check already does this — preserve the format).

### RFC Summaries (MUST for protocol work)
- N/A — config-validation feature, no wire protocol.

**Key insights:**
- Path-form `ze:required` is enforced at validate + startup + editor + web, but only for BGP (CheckRequiredFields + cli/validator hard-coded to `bgp.peer`). Web is the only generic enforcer.
- After the iface-mac removal, BGP peer/group is the ONLY path-form `ze:required` — so the path-form genericity gap is currently future-proofing (no live unenforced path-form field) plus removing the special-casing.
- Bare `ze:required;` on ipsec/pki/l2tp leaves is the live bug: those fields are advertised as required but enforced nowhere. They are off-design for this extension.
- Severity is a deliberate product choice: today path-form is warn/skip ("incomplete", exit 0, partial-config editing allowed) at validate/editor and hard at web — inconsistent.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/resolve.go` - `CheckRequiredFields(schema, bgpTree)`: gets `schema.Get("bgp")` → peer list, loops `peerListNode.Required`, uses `hasNestedValue`. BGP-hardcoded. Returns error.
  → Constraint: signature takes `*config.Schema` + `map[string]any` (resolved bgp tree). A generic version needs the full schema + full resolved tree to walk every list.
- [ ] `internal/component/bgp/config/peers.go:58` - calls `CheckRequiredFields` in `PeersFromConfigTree` (daemon startup path).
  → Constraint: LSP findReferences shows `peers.go:58` is the ONLY direct production caller (rest are tests). `ze config validate` reaches the check transitively via peer construction, not a direct call — a generic check must hook a shared path or be added at each entry point explicitly.
- [ ] `internal/component/cli/validator.go` - `validateWithYANG` returns early without a `bgp` container; `validatePeer` loops `peerListNode().Required` and emits `severityWarning`. Editor-time, BGP-only.
- [ ] `internal/component/web/handler_config.go:344` - generic for any list: loops `listNode.Required`, hard-rejects on missing field+no inherited value. Uses `splitFieldPath` + `resolveInheritedValue`.
- [ ] `internal/component/web/fragment.go` - `collectRequiredFields`/`resolveNestedValue` (splits on `/`), generic.
- [ ] `internal/component/config/yang_schema.go:747-758` - parses `ze:required` as `strings.Split(arg, "/")`, guarded by `fields[0] != ""` → bare (empty-arg) form dropped. Populates `ListNode.Required` only.
- [ ] `internal/component/config/schema.go` - `ListNode.Required [][]string`.
- [ ] `internal/component/{ipsec,pki}/schema/*.yang`, `internal/plugins/l2tpauthradius/schema/*.yang` - bare `ze:required;` on leaves (encryption, etc.).

**Behavior to preserve:**
- Warn/skip "incomplete peer" semantics at `ze config validate` / startup (exit 0, peer skipped) so partial configs remain editable — unless DESIGN decides to change severity (user decision).
- Existing peer error/warning message format (names peer + missing path + `set` hint).
- Web add-form hard-reject behavior + required/suggested field rendering.
- `test/parse/required-field-{missing,inherited,all-present}.ci` expectations (BGP).

**Behavior to change:**
- Path-form enforcement must become list-generic (any list with `ze:required "a/b"`), not BGP-only, at validate/startup/editor.
- Bare `ze:required;` must stop being a silent no-op (design decision: migrate to `mandatory true` or implement leaf-level required).
- Doc reconciled to match.

## Data Flow (MANDATORY)

### Entry Point
- Config text → parsed `*config.Tree` (+ resolved `map[string]any`) at three points: `ze config validate` (cmd), editor commit (`cli/validator`), daemon startup (`PeersFromConfigTree` / config load), and web add (`handler_config`).

### Transformation Path
1. Parse → tree → (BGP) `ResolveBGPTree` inheritance merge → resolved map.
2. Required check walks list schema nodes with `.Required`, resolves each entry's effective value (with inheritance where applicable), reports missing.
3. Result surfaced as error/warning (severity TBD in DESIGN).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Schema ↔ resolved tree | walk `ListNode.Required` against resolved `map[string]any` | [ ] |
| Validate ↔ startup ↔ editor ↔ web | shared generic check vs per-entry-point wiring | [ ] |

### Integration Points
- `CheckRequiredFields` (generalize or replace), `cli/validator.validateWithYANG` (extend beyond bgp), `handler_config` (already generic), `yang_schema.go` parsing (bare-form decision).

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (generic check must not import component-specific packages)
- [ ] No duplicated functionality (one generic walker, not per-component copies)
- [ ] Zero-copy preserved where applicable (validation is not a hot path)

## Approach (selected)

Two coordinated changes, defaults chosen (overridable by the user):

- **Part A — bare form → native `mandatory true`.** Replace off-design `ze:required;` on
  ipsec/pki/l2tp leaves with YANG `mandatory true` (already enforced at `ze config validate`
  via the section loop). Severity: hard error (native). Add editor parity for non-BGP
  sections, and a parse-time rejection of bare `ze:required;` so it can never silently no-op
  again.
- **Part B — path form → generic anchor-scoped walker.** Add a generic `CheckRequired`
  in the `config` package that walks every schema node carrying `.Required` and, for each
  config instance of that anchor node, checks the descendant path resolves (on the resolved
  tree, so BGP inheritance still applies). Conditional: no anchor instance ⇒ no requirement.
  Replace the BGP-hardcoded `CheckRequiredFields` and the `bgp.peer`-only loop in
  `cli/validator.go` with the generic walker, preserving the existing peer message format.
  Severity: keep warn/skip (partial-config editing). Anchor: implicit (the node the
  extension is attached to) — no new YANG syntax.

## Wiring Test
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config whose anchor entry lacks a path-form required field | → | generic `config.CheckRequired` | `test/parse/required-field-missing.ci` (BGP, existing) |
| `ze config validate` on an ipsec proposal missing the (now `mandatory`) encryption leaf | → | native `mandatory` via `ValidateTree` section loop | `test/parse/ipsec-proposal-missing-mandatory.ci` (new) |
| editor commit of a config missing a path-form required field | → | `cli/validator` generic required loop | `TestValidatorRequiredGeneric` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config has a list entry whose schema node carries `ze:required "a/b"`, and that entry lacks `a/b` after inheritance | `ze config validate` reports the entry path + missing field + a `set ...` hint, at warn/skip severity (exit 0, entry treated as incomplete) |
| AC-2 | Same as AC-1 but on a NON-BGP list with `ze:required` | Enforced identically by the generic walker, with no component-specific Go added |
| AC-3 | The anchor node is absent (no list entry, or an optional parent container not configured) | No requirement is reported (conditional-on-presence) |
| AC-4 | An ipsec/pki/l2tp leaf migrated from bare `ze:required;` to `mandatory true` is omitted while its parent node is present | `ze config validate` reports "mandatory field X is missing" as a hard error |
| AC-5 | Editor commit of a config missing a path-form required field (any component) | Editor surfaces the same warning as `ze config validate` (parity, not bgp-only) |
| AC-6 | A YANG schema still containing bare `ze:required;` (no path argument) | Schema load / `ze config validate` rejects it (no silent no-op) |
| AC-7 | Existing valid BGP configs (peer with all required fields) | Validate clean; `required-field-{missing,inherited,all-present}.ci` keep passing (no regression) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckRequiredGenericPerAnchor` | `internal/component/config/required_test.go` | AC-1/AC-2: walker reports a missing required descendant for each present anchor instance, any component | |
| `TestCheckRequiredAnchorAbsentSkips` | `internal/component/config/required_test.go` | AC-3: no anchor instance ⇒ no requirement | |
| `TestCheckRequiredBGPInheritance` | `internal/component/bgp/config/resolve_test.go` | AC-7: bgp→group→peer inheritance still satisfies the requirement (no regression) | |
| `TestValidatorRequiredGeneric` | `internal/component/cli/validator_test.go` | AC-5: editor surfaces required warnings for non-bgp lists | |
| `TestBareRequiredRejectedAtLoad` | `internal/component/config/yang_schema_test.go` | AC-6: bare `ze:required;` is rejected, not silently dropped | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `required-field-missing` (existing) | `test/parse/required-field-missing.ci` | AC-1/AC-7: BGP peer missing required field reported as incomplete | |
| `ipsec-proposal-missing-mandatory` (new) | `test/parse/ipsec-proposal-missing-mandatory.ci` | AC-4: ipsec proposal without the mandatory leaf is rejected | |

## Files to Modify
- `internal/component/config/` (new `required.go`) - generic `CheckRequired(schema, tree)` anchor-scoped walker (no component imports)
- `internal/component/bgp/config/resolve.go` - replace `CheckRequiredFields` body with a call to the generic walker (or delegate), preserving the resolved-tree + message behavior
- `internal/component/cli/validator.go` - use the generic walker instead of the `bgp.peer`-only loop; extend non-bgp `ValidateTree` coverage for editor parity
- `internal/component/config/yang_schema.go` - reject bare `ze:required;` (no path) instead of dropping it
- `internal/component/ipsec/schema/ze-ipsec-conf.yang`, `internal/component/pki/schema/ze-pki-conf.yang`, `internal/plugins/l2tpauthradius/schema/ze-l2tp-auth-radius-conf.yang` - bare `ze:required;` → `mandatory true`
- `cmd/ze/config/cmd_validate.go` - wire the generic path-form walker (mandatory-true already covered by the section loop)
- `docs/features/configuration.md`, `docs/architecture/config/syntax.md` - reconcile the ze:required contract to match implemented generality

## Implementation Steps
1. **Phase: Wiring (MANDATORY FIRST)** — add `config.CheckRequired` skeleton + a failing `TestCheckRequiredGenericPerAnchor`; confirm it is reachable from `cmd_validate` and `cli/validator`.
2. **Phase: Generic path-form walker** — implement anchor-scoped, conditional check on the resolved tree; replace the BGP-hardcoded `CheckRequiredFields` call and the `bgp.peer`-only loop, preserving inheritance + the peer message format. Tests: `TestCheckRequiredGenericPerAnchor`, `TestCheckRequiredAnchorAbsentSkips`, `TestCheckRequiredBGPInheritance`.
3. **Phase: Editor parity** — extend `cli/validator` to validate non-bgp sections so required warnings surface in the editor. Test: `TestValidatorRequiredGeneric`.
4. **Phase: Bare-form migration** — switch ipsec/pki/l2tp bare `ze:required;` to `mandatory true`; find and fix configs/fixtures missing those leaves (blast radius). Test: `ipsec-proposal-missing-mandatory.ci`.
5. **Phase: No silent no-op** — reject bare `ze:required;` at schema load. Test: `TestBareRequiredRejectedAtLoad`.
6. **Phase: Docs** — reconcile `configuration.md` / `syntax.md`.
7. **Verify** — `make ze-verify`.

## Design Insights
<!-- LIVE -->
- The "generic" framing splits cleanly: (a) generalize the path-form walker beyond BGP (low live impact post-mac-removal, mostly future-proofing + de-special-casing), (b) fix the bare-form leaf-mandatory cases (the live unenforced bug).
- **KEY (research):** Ze's YANG validator ALREADY enforces native `mandatory true` (`internal/component/config/yang/validator.go:510-517, 555-562` → `ErrTypeMissing`), and `mandatory true` is already used in many schemas (bfd, sysctl, routingtable, static, l2tpshaper). So (b) becomes "migrate off-design `ze:required;` on leaves → `mandatory true`" — native, already-working, no new Go. (b) is NOT a ze:required problem; it is a misuse of the extension.
- **RESOLVED:** `cmd/ze/config/cmd_validate.go:272-277` loops `yangSectionsToValidate` and runs `ValidateTree(section, ...)` per section — comment: "check ... mandatory fields for all non-BGP config sections." So native `mandatory true` IS enforced at `ze config validate` for ipsec/pki/l2tp (verify those sections are in `yangSectionsToValidate`). The editor (`cli/validator.go:238,279`) only deep-validates the `bgp` / `bgp/peer` subtree, so native `mandatory` for non-BGP does NOT fire in the editor — that is the coverage gap to close for parity. Daemon-startup validation is component-specific (each component validates on build/apply); confirm during implementation.
- **Blast radius:** making those leaves `mandatory true` is a hard error (currently unenforced) — existing ipsec/pki/l2tp configs or test fixtures missing those leaves would start failing. Need to find/fix them.
- **CORE DESIGN (user direction):** every `ze:required` has an **anchor node** = the schema node it is declared on. Enforcement is per-anchor-instance and conditional: "for each X present, X must have Y; no X ⇒ no requirement." The anchor is currently implicit (the attachment point) and ignored by the Go check (which hardcodes `bgp.peer`); it must become the explicit, generic scope. The generic walker iterates schema nodes carrying `ze:required`, and for each config instance of that node validates the descendant path within that instance. This keeps enforcement inside each plugin's own YANG space.
- Mac removal is consistent with this model regardless: an `ethernet` entry exists whenever defined, so anchoring mac-required at `ethernet` would make it always-required, which is exactly the behavior we rejected. Mac simply should not be required at all.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Anchor-scoped requirement.** A `ze:required` is enforced relative to the schema node it is *declared on* (the anchor). For each config instance of that anchor node, the required descendant must resolve; if no anchor instance exists, the requirement does not apply. The anchor comes from the YANG attachment, discovered generically by walking the schema, not hardcoded per component. | (a) Current: hardcode the scope in Go (`CheckRequiredFields` → `bgp.peer`). (b) Global unconditional "config must contain Y". | A requirement is inherently conditional: "if you have an X, that X must have a Y." The scope (anchor) belongs to the plugin's YANG, so requirements are self-describing and plugin-local; one generic walker honors all of them. Conditional-on-presence is what the bare leaf form lacks today. |

## Known Limitations
- `unique` enforcement remains web-only (separate concern, separate spec).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>`

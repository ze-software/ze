# Spec: layout-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 (all children closed 2026-07-08: learned 1088, 1089, 1090, 1091; reactor-candidate destination recorded: `plan/spec-reactor-split.md`; umbrella closed 2026-07-08) |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/module-tiers.md` - the sibling tier effort this umbrella must NOT duplicate
4. `plan/spec-tiers-0-umbrella.md` - Path C gate philosophy + shrink-only baseline pattern
5. `scripts/dev/dep_audit.py` - existing placement gate (home for the new import-direction check)
6. `scripts/checks/plugin_process_boundary.go` - boundary guard with hardcoded scan roots
7. `scripts/codegen/plugin_imports.go` - `pluginDirs` / `nestedPluginDomains`, the source of truth for plugin namespaces

## Task

A comparative structure review (this session, 2026-07-08) measured Ze against
holo-routing/holo, osrg/gobgp, and bio-routing/bio-rd. Ze is ~1.14M lines of Go
across 610 packages. The review found that the placement question "which tier does
a package belong in" is already owned by `spec-tiers-0-umbrella.md` (engine gate
enforced, baseline empty), but four structural problems remain unowned by any
spec:

1. **The core tier leaks upward.** 5 files under `internal/core/` import
   `internal/component/` packages. Nothing prevents new upward imports; the tier
   rule's enforced gate covers engine placement, not import direction out of core.
2. **The plugin-boundary guard is blind to two plugin namespaces.**
   `scripts/checks/plugin_process_boundary.go` hardcodes two scan roots while
   `sdk.NewWithConn` engines also live under `internal/component/l2tp/plugins/`
   (4 engines) and `internal/component/firewall/plugins/` (1 engine). The bug
   class it guards (same-process-effect calls that silently no-op when the plugin
   runs external) is unchecked there.
3. **No shared protocol skeleton and no package-naming glossary.** Each protocol
   invents its own module vocabulary (BGP: `fsm`/`reactor`/`message`/`wireu`;
   BFD: `engine`/`packet`/`session`/`transport`; IKE: `engine`/`wire`/`crypto`;
   IS-IS: `adjacency`/`circuit`/`lsdb`/`packet`). Sibling packages
   `internal/component/cli`, `internal/component/cmd`,
   `internal/component/command` collide in meaning. Holo's strongest structural
   property is the inverse: one fixed module skeleton repeated across every
   protocol crate, so learning one protocol teaches all of them.
4. **Repo-root clutter and a stale architecture claim.** Committed at root:
   `screenlog.0` (empty screen log, not gitignored), `qos-map.md` (vendor QoS
   research), `AI-NAVIGATION-AUDIT.md` (audit report; `plan/audits/` exists),
   `test-web` (dev script), `parked/` (dead code, still linted by the Makefile).
   `docs/architecture/overview.md` still lists OSPF and IS-IS as not implemented
   while `internal/plugins/isis` + `internal/plugins/ospf` hold 339 non-test Go
   files.

**Goal:** close these four gaps with the same philosophy spec-tiers chose (Path
C): crisp mechanical gates where a rule is unambiguous, shrink-only baselines for
grandfathered violations, documented conventions where judgment is needed, and no
package moves between tiers (tiers-5 owns those).

This is an **umbrella spec**. Each gap becomes a sequenced child spec (below).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `ai/rules/module-tiers.md` - the tier taxonomy and what the existing gate enforces
  → Decision: tier = dependency direction; the enforced gate covers ONLY engine placement (Path C); core/composition classification is advisory.
  → Constraint: this umbrella must not move packages between tiers and must not duplicate tiers-5 work (B-2 library extraction, B-3 host tier, config split).
- [ ] `plan/spec-tiers-0-umbrella.md` - the sibling umbrella (Phase 4/6, in-progress)
  → Decision: Path C (crisp mechanical subset enforced, no allowlist) was chosen over full-trichotomy enforcement; follow the same philosophy here.
  → Constraint: `scripts/dev/tier_migration_baseline.txt` is the house pattern for shrink-only grandfathered violations; reuse the pattern, not the file.
- [ ] `ai/rules/naming.md` - existing naming rule
  → Constraint: the package-naming glossary extends this file; do not fork a second naming doc.
- [ ] `ai/rules/plugin-design.md` - registration patterns, Proximity Principle
  → Constraint: skeleton conventions must not contradict the registration/proximity rules.
- [ ] `docs/architecture/overview.md` - the stale Non-Goals claim (lines 23-26)
  → Constraint: factual doc changes need source-anchor comments (`ai/rules/documentation.md`).
- [ ] `ai/rules/canonical-sources.md` - where rules live and how generated mirrors sync
  → Constraint: new rule docs go under `ai/rules/` with an `ai/rules/INDEX.md` row (regenerated, not hand-edited).

### RFC Summaries (MUST for protocol work)
- N/A - no wire-protocol behavior changes anywhere in this set.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The tiers effort owns placement BETWEEN tiers; this umbrella owns import
  direction OUT of core, conventions WITHIN tiers, and repo hygiene.
- Every gate added here must be additive, mechanical, and baseline-protected,
  matching the Path C precedent.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/core/diagnostic/doctor_registry.go` - lines 12-14 import `internal/component/config/storage`, `internal/component/host`, `internal/component/plugin`; `DoctorCheckContext` (lines 33-39) embeds the component types `zeplugin.PluginConfig`, `storage.Storage`, `host.PlatformInfo` as struct fields
  → Constraint: this is a TYPE dependency, not just a registration shim; removing it means interface extraction or type relocation, so it stays baselined until designed (its `Tree any` field, lines 31-33, shows the cycle-avoidance pattern already in use for config).
- [ ] `internal/core/diagnostic/types.go` - line 7 imports `internal/component/plugin`
- [ ] `internal/core/resolve/resolve.go` - line 11 imports `internal/component/config/storage`; `Storage()` (line 25) constructs `storage.Storage` backends
  → Constraint: a core package manufacturing component-owned backends is the inverted-direction case the gate exists to stop recurring.
- [ ] `internal/core/ipc/yang/register.go` - line 6 imports `internal/component/config/yang`; the file is GENERATED by `scripts/codegen/yang_glue.go` (line 1), so its fix routes through the generator or the glue file's placement, not a hand edit
  → Constraint: exactly 5 files under `internal/core/` import `internal/component/` today (repo-wide grep, this session; the fifth is `internal/core/ipc/yang_test.go`). These are the "registry shims" already noted in `plan/spec-tiers-0-umbrella.md` Current Behavior. They seed the shrink-only baseline, each annotated with its fix route.
- [ ] `scripts/checks/plugin_process_boundary.go` - header (lines 1-40) states it scans plugin packages "under internal/plugins/ or internal/component/bgp/plugins/"; the scan-root list near line 117 hardcodes those two roots
  → Constraint: `sdk.NewWithConn(` engines exist outside its scan roots: `internal/component/l2tp/plugins/{pool,shaper,authlocal,authradius}/register.go` and `internal/component/firewall/plugins/irr/irr.go` (grep, this session).
- [ ] `scripts/codegen/plugin_imports.go` - `pluginDirs` (lines 143-156) lists all plugin search roots including `internal/component/firewall/plugins`; `nestedPluginDomains` (line 158) covers component-owned `plugins/` dirs
  → Decision: the generator is the single source of truth for plugin namespaces; the boundary check must derive its scan roots from it (same lesson as tiers blocker B-1: never re-derive "is a plugin" with a second heuristic).
- [ ] `.golangci.yml` - no `depguard`/`gomodguard` import-boundary linter configured (grep, this session)
  → Decision: house precedent is a bespoke gate in `scripts/dev/dep_audit.py` wired into `make ze-verify` (target `ze-tier-check`), not a golangci depguard; extend that gate rather than adding a second enforcement mechanism.
- [ ] `internal/component/bgp/wireu/doc.go` - "Package wireu implements lazy-parsed BGP UPDATE messages with zero-copy iterators over wire bytes"
  → Constraint: "u" means UPDATE; the name is opaque to every reader (naming-glossary child decides rename vs glossary-entry-only).
- [ ] `docs/architecture/overview.md` - lines 23-26 ("Non-Goals: OSPF and IS-IS are not implemented in the current tree"; Last Updated 2026-05-29)
  → Constraint: contradicted by `internal/plugins/isis` + `internal/plugins/ospf` (339 non-test `.go` files, ~118k lines, this session's count).
- [ ] `Makefile` - line 209 lints `./parked/...`; `prod.json` consumed by `Makefile` + `mk/appliance.mk` and documented in `docs/guide/appliance.md` (grep, this session)
  → Constraint: `prod.json` is NOT junk; it is appliance-build input carrying a real device address. Moving or renaming it is a user decision recorded in the hygiene child.
- [ ] `.gitignore` - no rule for `screenlog.*`; `screenlog.0` is committed (git ls-files, this session)
- [ ] ~~Root-file consumers (grep, this session): `qos-map.md` referenced by `docs/guide/configuration.md`, `internal/plugins/cos/yang/ze-cos-conf.yang`, `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go`; `test-web` referenced by `docs/architecture/testing/runner-architecture.md`; `AI-NAVIGATION-AUDIT.md` referenced by `plan/learned/1067-generated-discovery-indexes.md`~~
  → Correction (2026-07-08, child 1 audit, `plan/learned/1088-layout-1-hygiene.md`): WRONG -- these were bare-token grep hits, not references to the moved files. Every `qos-map` hit is the config keyword (`parseQoSMap`, `internal/component/iface/config.go:928,932`); the `test-web` and `AI-NAVIGATION-AUDIT` hits are learned-summary filenames/records. The relocations were clean renames with zero referrer edits; editing the listed files would have corrupted live config parsing.
  → Constraint: every root-file move must update its non-`plan/` referrers in the same change; `plan/learned/` references stay as historical records (precedent: tiers Phase 2 results). Disambiguate keyword-vs-filename (grep the literal path, e.g. `qos-map\.md`) BEFORE trusting a referrer list.

**Behavior to preserve:** (unless user explicitly said to change)
- All runtime behavior. No package moves, no API changes, no wire changes.
- `make ze-verify` stays green on a compliant tree; new gates fail only on NEW violations (baseline covers the 5 known files).
- `bin/ze --plugins` inventory unchanged (nothing here touches registration).
- The 5 grandfathered `internal/core/` upward imports keep compiling until their baseline entries are individually removed.

**Behavior to change:** (only if user explicitly requested)
- `make ze-verify` gains a core import-direction check (fails on new upward imports from `internal/core/`).
- `make ze-plugin-boundary-check` covers all plugin namespaces, not two.
- Root-level stray files relocated/deleted; stale overview claims corrected.
- New convention docs (`ai/rules/`) constrain FUTURE package layout and naming; existing packages are not mass-renamed.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- An author creates or edits a Go package (placement, name, imports), or adds a
  plugin engine, or edits repo-root files.

### Transformation Path
1. Import graph: `scripts/dev/dep_audit.py --check` parses package imports (it already does this for the engine gate).
2. New check: any `internal/core/**` file importing `internal/component/**` or `internal/plugins/**` is a violation unless listed in the shrink-only baseline.
3. Plugin namespaces: `plugin_process_boundary.go` reads `pluginDirs` + `nestedPluginDomains` from `scripts/codegen/plugin_imports.go` (or a shared derivation) instead of its private hardcoded list, then scans as today.
4. Conventions: `ai/rules/naming.md` (glossary) and the new protocol-skeleton rule steer new package creation; an advisory report lists per-protocol divergence without failing.
5. `make ze-verify` runs the gates; violations name the file, the rule doc, and the required fix.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | unchanged (no registration or IPC changes) | [ ] |
| Wire ↔ Storage | unchanged | [ ] |
| core ↔ component import direction | new dep_audit check + baseline | [ ] gate selftest |
| checker ↔ generator namespace list | boundary check derives roots from `plugin_imports.go` | [ ] parity test |

### Integration Points
- `scripts/dev/dep_audit.py --check` - existing gate in `make ze-verify` (`ze-tier-check`); gains the core-direction check.
- `scripts/checks/plugin_process_boundary.go` + its `_test.go` selftest - gains derived scan roots.
- `ai/rules/INDEX.md` - regenerated to list the new/extended rules.
- `scripts/dev/check_doc_links.py` - proves relocated docs leave no broken references.

### Architectural Verification
- [ ] No bypassed layers (gates read the same import graph dep_audit already parses)
- [ ] No unintended coupling (no new runtime dependencies; checks are build-time only)
- [ ] No duplicated functionality (extend dep_audit + existing checker; no second graph walker, no depguard duplicate)
- [ ] Zero-copy preserved (N/A - no runtime code changes)
- [ ] Registration over hardcoding — the boundary checker derives plugin namespaces from the generator's registry instead of a second hardcoded list (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->
<!-- Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis) land HERE, not just in conversation. -->

### Assumptions
<!-- Things believed true that the design depends on. Every row needs a validation method. -->
<!-- Status: unvalidated → confirmed | broken. A broken assumption also gets a Mistake Log "Wrong Assumptions" row. -->
<!-- No assumption may still be `unvalidated` at Pre-Commit Verification. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Exactly 5 files under `internal/core/` import `internal/component/`, and none import `internal/plugins/` | repo-wide grep this session (2026-07-08) | baseline seeded wrong; gate fails on day one | re-run the grep during the child's audit step, paste output into the child spec | **confirmed** (2026-07-08, child 2) -- audit re-grep found exactly 5 files / 10 import pairs, 0 core->plugins; seeded as `scripts/dev/core_import_baseline.txt` (learned 1089) |
| A-2 | `dep_audit.py` already holds a usable import graph and is the correct home for the direction check | `ai/rules/module-tiers.md` (gate description); `plan/spec-tiers-0-umbrella.md` tiers-5 B-1 results | a separate script is needed; more wiring | read `dep_audit.py` structure during child design | **confirmed** (2026-07-08, child 2) -- `core_direction_gate` landed inside `dep_audit.py` reusing its existing import graph (commit `095c91fc9`); no second parser |
| A-3 | The l2tp/firewall nested plugins use the same SDK/DirectBridge mechanics the boundary checker assumes | `sdk.NewWithConn` grep hits in their `register.go` files | scan-root extension flags false positives | run extended checker, triage every new finding before enabling the gate | **confirmed** (2026-07-08, child 2) -- extended checker over the 13 derived roots (`--print-roots`) reported ZERO new findings; nothing to triage |
| A-4 | Each root-file move has only the referrers found this session | grep across repo for each filename | broken link or build break after move | re-grep per file at move time; `check_doc_links.py` + `make ze-doc-test` after | **broken** (2026-07-08) -- the umbrella's referrer list was bare-token hits; the child-1 re-grep found ZERO real referrers (see Mistake Log + `plan/learned/1088-layout-1-hygiene.md`) |
| A-5 | `prod.json` disposition (keep at root, move, or strip the device address) is a decision the user will make at hygiene-child approval | Makefile + `mk/appliance.mk` consumers; content read this session | hygiene child blocked on one file | explicit user decision at hygiene child approval | **confirmed** (2026-07-08) -- user chose keep-at-root unchanged; address is private RFC1918 |
| A-6 | A protocol skeleton can be defined that fits BGP, BFD, IKE, IS-IS, OSPF, LDP without forcing renames of stable packages | holo precedent (uniform skeleton across 8 protocol crates); Ze's existing `yang//cmd//cli/` convention already uniform | skeleton rule stays advisory forever or fragments into per-protocol exceptions | design probe in child 4: table mapping every existing protocol module to the proposed skeleton | **confirmed** (2026-07-08, child 4) -- probe table maps all 7 protocols cleanly; only documented exceptions are the BGP historical vocabulary and `ike/wire`; zero renames forced (`ai/rules/protocol-skeleton.md`; learned 1091) |
| A-7 | The reactor god package (69 non-test files, one package) is NOT already covered by the rib-arch set | grep for "reactor" in `plan/spec-rib-arch-0-umbrella.md` returned no decomposition scope this session | duplicate scope with rib-arch children | re-check the rib-arch umbrella + children when scheduling the candidate child | **confirmed** (2026-07-08, umbrella closure) -- grep for decomposition scope across all nine `plan/spec-rib-arch-*.md`: zero hits; rib-arch-8 edits two reactor files' internals (`filter_delta.go`, `reactor_api_forward.go`) without touching package structure |

### Risks
<!-- Things that could go wrong even if all assumptions hold. From /ze-spec Failure Mode Analysis. -->
<!-- Surviving risks copy forward to the Executive Summary "Risks & observations" and the learned summary. -->
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The core-direction gate flags a legitimate future shim and blocks unrelated work | a PR/spec stalls on the gate with a defensible upward import | baseline accepts a new row ONLY with a spec reference in the rationale (same policy as `tier_non_engine_categories.txt` planned-violation rows) |
| R-2 | Naming/skeleton rules trigger mass-rename churn across a 610-package tree | child spec's Files to Modify balloons | glossary-first: rules constrain NEW packages; renames limited to an explicit, user-approved shortlist (at most `wireu`), executed with the `migrate_module.py`-style deterministic tool |
| R-3 | Collision with in-flight rib-arch specs touching `internal/component/bgp/` | merge conflicts in bgp trees | children 1-3 touch no bgp source; child 4 is a rule doc + advisory report only; the reactor-split candidate stays unscheduled until rib-arch lands |
| R-4 | ~~Relocating `qos-map.md` breaks YANG/test references that embed the path~~ DISSOLVED (2026-07-08): no YANG/test file embeds the path; the hits were the `qos-map` config keyword (A-4 broken) | `ze-cos-conf.yang` or the vlan-qos lab test fails | risk never existed; the move landed as a clean rename with `check_doc_links.py` showing no new breakage |
| R-5 | Extending the boundary checker's roots surfaces pre-existing findings in l2tp/firewall plugins | extended checker exits non-zero on first run | triage each finding per its own rules (guard present vs real bug); real bugs get fixed or spec'd, never silenced |
| R-6 | Umbrella scope creep into tiers-5 territory (core split, host tier) | a child starts proposing package moves | hard scope rule: any move between tiers is out of scope here; hand it to the tiers umbrella |
| R-7 | The generated `internal/core/ipc/yang/register.go` upward import cannot be removed without a codegen change | yang_glue generator emits component imports into core | its baseline row names the generator as the fix route; if unfixable cheaply it stays baselined with a spec reference, never silently dropped from the gate |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
Umbrella-level wiring is the gates themselves; per-change wiring lives in child specs.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` | → | core import-direction check in `dep_audit.py --check` | `TestCoreImportDirection` (dep_audit selftest fixture, child 2) |
| `scripts/dev/dep_audit.py --check` on a tree with a new `internal/core/` upward import | → | exit 2 naming file, imported package, rule doc | dep_audit selftest fixture (child 2) |
| `make ze-plugin-boundary-check` | → | scan roots derived from generator plugin namespaces | `plugin_process_boundary_test.go` parity case: derived roots cover l2tp/firewall plugin dirs (child 2) |
| new rule docs discoverable | → | `ai/rules/INDEX.md` rows for glossary + skeleton rules | `make ze-rules-index` check clean (children 3-4) |
| relocated docs still linked | → | no dangling references after root cleanup | `scripts/dev/check_doc_links.py` clean (child 1) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A NEW `.go` file under `internal/core/` imports `internal/component/` or `internal/plugins/` | `dep_audit.py --check` exits non-zero naming the file and pointing at the rule doc; `make ze-verify` fails |
| AC-2 | The 5 grandfathered upward-import files, unchanged | Gate passes; the baseline lists exactly those files with a fix-route annotation each; a fixed file left in the baseline is a stale-entry failure (same semantics as the tier baseline) |
| AC-3 | `plugin_process_boundary.go` runs | Scan roots are derived from the generator's plugin-namespace source of truth; packages under `internal/component/l2tp/plugins/` and `internal/component/firewall/plugins/` are scanned; selftest proves it |
| AC-4 | An author looks up what `packet`/`message`/`wire`/`session`/`engine`/`cli`/`cmd`/`command` mean as package names | `ai/rules/naming.md` carries a package-naming glossary defining each term, the `internal/component/{cli,cmd,command}` trio, and the four rib-named packages (`internal/core/rib`, `internal/core/routingtable`, `internal/component/bgp/rib`, `internal/plugins/routingtable`); `ai/rules/INDEX.md` row updated |
| AC-5 | A new protocol package set is created | `ai/rules/protocol-skeleton.md` exists defining the standard protocol subpackage skeleton and when each module is required; an advisory conformance report lists per-protocol divergence for existing protocols without failing the build |
| AC-6 | Repo root after hygiene child | `screenlog.0` deleted and `screenlog.*` gitignored; `qos-map.md`, `AI-NAVIGATION-AUDIT.md`, `test-web` relocated with all non-`plan/` referrers updated; `parked/` removed and the Makefile lint invocation no longer names it; `prod.json` disposition recorded as an explicit user decision |
| AC-7 | `docs/architecture/overview.md` after hygiene child | Non-Goals no longer claims OSPF/IS-IS are unimplemented; corrected claims carry source anchors; repo-wide grep finds no other doc making the stale claim |
| AC-8 | A future session asks "which spec owns package moves between tiers" | This umbrella's scope statements and the child specs all point to the tiers umbrella; no child performs a tier move |

## End-to-End User Stories (MANDATORY for new features)

<!-- For each user-facing operation the feature enables, trace the full path.
     This section catches missing code that narrow ACs miss. ACs verify individual
     components work; user stories verify the full chain is connected.
     Every story must have a corresponding functional or wiring test. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Author adds `internal/core/foo` importing `internal/component/bar`, runs `make ze-verify` | dep_audit import parse -> direction check -> baseline miss -> exit 2 with file + rule pointer | dep_audit selftest fixture (child 2) |
| 2 | Author fixes `internal/core/resolve/resolve.go` upward import but forgets the baseline row | stale-baseline detection -> gate fails demanding baseline shrink | dep_audit selftest stale-entry fixture (child 2) |
| 3 | Author adds a same-process-effect call in an l2tp nested plugin | boundary checker scans derived roots -> finding reported | `plugin_process_boundary_test.go` new-root case (child 2) |
| 4 | New contributor reads the naming glossary before creating a protocol package | `ai/rules/INDEX.md` row -> `naming.md` glossary -> `protocol-skeleton.md` | `make ze-rules-index` clean; doc-link check (children 3-4) |
| 5 | User browses repo root | no stray logs/scripts/reports; architecture overview matches the tree | `git ls-files` root listing + `check_doc_links.py` (child 1) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCoreImportDirection` (name per child design) | dep_audit selftest (`--selftest` fixture set) | new upward import from core -> exit 2; baselined file -> pass; stale baseline row -> fail | |
| boundary-checker derived-roots parity | `scripts/checks/plugin_process_boundary_test.go` | derived scan roots include every generator plugin namespace; l2tp/firewall dirs scanned; five historical fixture cases stay green | |
| rules-index freshness | existing `make ze-rules-index` check | INDEX rows for extended/new rules regenerate cleanly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no numeric inputs | - | - | - | - |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-verify` end-to-end | existing verify pipeline | gates run inside verify; compliant tree green, planted violation red (proven in child 2 via fixture, not by dirtying the tree) | |
| vlan-qos parse suite stays green after the qos-map move | `test/parse/iface-vlan-qos.ci`, `test/parse/iface-vlan-qos-invalid.ci`, `test/parse/cos-profile-conflict.ci`, `test/parse/iface-vpp-rejects-nonidentity-qos.ci` | ~~these `.ci` tests reference `qos-map`; child 1 updates their references~~ Correction (2026-07-08): they use the `qos-map` config KEYWORD (`parseQoSMap`, `internal/component/iface/config.go:928,932`), not the moved file; child 1 correctly left them untouched and the suite's greenness proves the keyword surface was not disturbed | |
| doc integrity after moves | `make ze-doc-test` + `scripts/dev/check_doc_links.py` | relocated files leave no dangling references | |

### Interop Tests (MANDATORY for protocol features)
N/A - no wire-protocol behavior changes. (Justification: gates, docs, and file
relocations only; no protocol code path is modified.)

### Future (if deferring any tests)
- None planned; each child defines its own additions within the rows above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `scripts/dev/dep_audit.py` - core import-direction check + baseline handling (child 2)
- `scripts/checks/plugin_process_boundary.go` + `_test.go` - derive scan roots from the generator's plugin namespaces (child 2)
- `ai/rules/naming.md` - package-naming glossary section (child 3)
- `ai/rules/INDEX.md` - regenerated rows (children 3-4)
- `ai/rules/module-tiers.md` - one cross-reference line pointing at the new import-direction gate (child 2)
- `.gitignore` - `screenlog.*` (child 1)
- `Makefile` - drop `./parked/...` from the lint invocation, line 209 (child 1)
- `docs/architecture/overview.md` - correct Non-Goals; refresh directory-structure table if touched rows are stale (child 1)
- ~~`docs/guide/configuration.md`, `internal/plugins/cos/yang/ze-cos-conf.yang`, `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` - `qos-map.md` path updates (child 1)~~ NOT MODIFIED (2026-07-08): phantom referrers -- they carry the `qos-map` config keyword, not the file path (A-4 broken)
- ~~`test/parse/iface-vlan-qos.ci`, `test/parse/iface-vlan-qos-invalid.ci`, `test/parse/cos-profile-conflict.ci`, `test/parse/iface-vpp-rejects-nonidentity-qos.ci`, `test/vlan-qos-lab/run.sh` - `qos-map` references (child 1; re-grep at move time per A-4)~~ NOT MODIFIED: the A-4 re-grep found only keyword uses; editing them would have corrupted config parsing
- ~~`docs/architecture/testing/runner-architecture.md` - `test-web` path update (child 1)~~ NOT MODIFIED: its `test-web` hit is the learned-summary filename `868-test-web-parallel.md`, not the moved script
- `Makefile` / `mk/appliance.mk` / `docs/guide/appliance.md` - only if the user decides to move `prod.json` (child 1)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | no config surface changes |
| YANG validation constraints | [ ] no | - |
| YANG custom validators | [ ] no | - |
| CLI commands/flags | [ ] no | no CLI changes |
| CLI grammar (action before identifier) | [ ] no | - |
| Editor autocomplete | [ ] no | - |
| Functional test for new RPC/API | [ ] no | no new RPC/API |
| Pipe completeness | [ ] no | no command output |
| Env var registration | [ ] no | no env vars |
| Doctor check for runtime dependencies | [ ] no | no runtime dependencies added |
| Prometheus counters/metrics | [ ] no | no observable runtime state added |
| Verification gate | [ ] yes | `make ze-verify` wiring of the extended `dep_audit.py` check (child 2) |
| Discovery updates | [ ] yes | `ai/rules/INDEX.md` + `ai/INDEX.md` keyword rows for the new rules and gate (`ai/rules/discovery-updates.md`) |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- Every No MUST be backed by a source-aware check, not a guess. At minimum, grep docs for source anchors pointing at changed files. -->
<!-- Any factual doc change MUST include or update a source-anchor HTML comment after the claim. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] no | gates and conventions are contributor-facing |
| 2 | Config syntax changed? | [ ] no | - |
| 3 | CLI command added/changed? | [ ] no | - |
| 4 | API/RPC added/changed? | [ ] no | - |
| 5 | Plugin added/changed? | [ ] no | scan-root derivation changes no plugin behavior |
| 6 | Has a user guide page? | [ ] no (resolved 2026-07-08) | ~~`docs/guide/configuration.md` (qos-map path)~~ keyword not path (A-4 broken); `docs/guide/appliance.md` untouched (prod.json stayed at root) |
| 7 | Wire format changed? | [ ] no | - |
| 8 | Plugin SDK/protocol changed? | [ ] no | - |
| 9 | RFC behavior implemented/changed? | [ ] no | - |
| 10 | Test infrastructure changed? | [ ] yes (child 2) | `docs/functional-tests.md` only if the verify stage list it documents changes |
| 11 | Affects daemon comparison? | [ ] no | - |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/overview.md` (stale claims, child 1); rule docs under `ai/rules/` (children 2-4) |
| 13 | Route metadata keys added/changed? | [ ] no | - |
| 14 | Prometheus counters added/changed? | [ ] no | - |
| 15 | Registered inventory changed? | [ ] no | - |
| 16 | Changed files referenced by doc source anchors? | [ ] verify per child | grep `docs/` for `source:` anchors naming each touched file |
| 17 | Docs show examples for this area? | [ ] verify per child | grep for moved filenames across `docs/` |

## Files to Create
- `ai/rules/protocol-skeleton.md` - the standard protocol subpackage skeleton (child 4)
- shrink-only baseline for the 5 grandfathered core upward imports, each row annotated with its fix route (file name per child 2 design; pattern of `scripts/dev/tier_migration_baseline.txt`)
- advisory protocol-skeleton conformance report (script or dep_audit section, child 4 design)
- child specs `spec-layout-1..4` as sequenced below (from `plan/TEMPLATE.md`)

## Child Spec Plan (sequenced, lowest risk first)

| # | Child spec | Scope | Risk | Gated by |
|---|-----------|-------|------|----------|
| 1 | `spec-layout-1-hygiene.md` -- **COMPLETE** (closed 2026-07-08; `plan/learned/1088-layout-1-hygiene.md`) | Root cleanup: delete `screenlog.0` + gitignore; relocate `qos-map.md` (docs/research/), `AI-NAVIGATION-AUDIT.md` (plan/audits/), `test-web` (scripts/dev/) ~~with referrer updates~~ (zero real referrers, A-4 broken); remove `parked/` + Makefile lint edit; record the `prod.json` decision (kept at root, user decision); fix `docs/architecture/overview.md` Non-Goals + stale rows. No Go changes. | low | doc-link check + `make ze-doc-test` |
| 2 | `spec-layout-2-core-import-gate.md` -- **COMPLETE** (closed 2026-07-08; `plan/learned/1089-layout-2-core-import-gate.md`; commits `095c91fc9`/`87043ee82`) | Extend `dep_audit.py --check` with the core import-direction rule (`internal/core/` MUST NOT import `internal/component/` or `internal/plugins/`), seeded shrink-only baseline (5 files / 10 pairs, fix-route annotations), selftest fixtures, `make ze-verify` wiring; derive `plugin_process_boundary.go` scan roots from the generator's plugin namespaces (13 roots; zero new findings to triage). | low | child 2 selftests green; `make ze-verify` green |
| 3 | `spec-layout-3-naming-glossary.md` -- **COMPLETE** (closed 2026-07-08; `plan/learned/1090-layout-3-naming-glossary.md`; commits `4b81e4654`/`4d179ede2`) | Package-naming glossary in `ai/rules/naming.md` (7 terms, the `cli`/`cmd`/`command` trio, the four rib-named packages); `wireu` KEPT + documented per user decision (rename shortlist closed empty). | med | user approval on the rename shortlist (given 2026-07-08) |
| 4 | `spec-layout-4-protocol-skeleton.md` -- **COMPLETE** (closed 2026-07-08; `plan/learned/1091-layout-4-protocol-skeleton.md`) | `ai/rules/protocol-skeleton.md` (required-vs-optional modules, RFC-term per-peer state naming, probe mapping for 7 protocols, exceptions = BGP vocabulary + `ike/wire`); advisory `scripts/dev/protocol_skeleton_report.py` wired as the non-enforcing last line of `ze-tier-check`. No moves, no renames. | med (design-heavy) | design probe table (A-6) confirmed in the child |
| candidate (unscheduled) | reactor decomposition | `internal/component/bgp/reactor` is one package of 69 non-test files (~64k lines with tests). Splitting it is code re-architecture, not layout convention; it must be sequenced against the in-flight `spec-rib-arch-*` set (A-7). Recorded here so the finding has a destination; scheduling is a user decision after rib-arch. -> DESTINATION (2026-07-08, user decision at umbrella closure): `plan/spec-reactor-split.md` (skeleton, Depends `spec-rib-arch-0-umbrella.md`). Standalone spec chosen over folding into rib-arch (no decomposition scope there; another session's in-flight work) and over renaming `reactor` now (would churn 154 package clauses + 331 doc anchors twice; see `spec-rename-0-umbrella.md` exclusion). | high | rib-arch outcome + user decision (given) |

Cross-references: children reference this umbrella and their predecessors by
filename. Scope rule for every child: package moves between tiers belong to
`plan/spec-tiers-0-umbrella.md`, never here.

## Child 1 (hygiene) -- COMPLETE (2026-07-08)

Implemented in commit `b652e176c`, closed in `5424c170f`, learned summary
`plan/learned/1088-layout-1-hygiene.md` (added retroactively in `4db00c3bb`;
the spec had planned number 1087, taken meanwhile by the linux-lint spec).

Results (post-closure `/ze-review-spec` verified, 2026-07-08):
- AC-6 satisfied: root clean (`git ls-files` shows only `prod.json` of the watched
  names), `.gitignore:79` covers `screenlog.*`, `Makefile:209` no longer lints
  `./parked/...`, all three relocations landed as clean renames (executable bit
  preserved on `scripts/dev/test-web`), `prod.json` untouched.
- AC-7 satisfied: both docs corrected with dated source anchors (OSPF 245 / IS-IS 94
  non-test `.go` files); repo-wide grep for the stale claim finds zero hits.
- A-4 broke (see Mistake Log): the umbrella's referrer list was bare-token grep hits;
  the relocations needed zero referrer edits.

Process gaps found by the post-closure review (recorded, not repairable in the
deleted child spec): the closure ran without a `/ze-review` Review Gate and with
child assumptions A-3/A-4 still marked unvalidated (evidence existed: A-3 via the
clean lint after `parked/` removal, A-4 via the counts in the overview.md source
anchor). Children 2-4 MUST fill their gate/assumption sections before commit B.

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against — they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the active child |
| 2. Audit | Current Behavior - re-verify every grep-based claim (A-1, A-3, A-4) before coding |
| 3. Wiring phase | Wiring Test table - gates and selftests first |
| 4. Implement (TDD) | Implementation Phases below (per child) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section - per child |
| 14. Present summary + close | Executive Summary Report; two-commit closure per child |

### Implementation Phases

<!-- List concrete phases of work. Each phase follows TDD: write test → fail → implement → pass.
     Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test.
     Remaining phases fill in feature logic behind the wired skeleton. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Hygiene (child 1, MANDATORY FIRST - cheapest, zero Go risk)** - root cleanup + stale-doc fixes per the child table.
   - Tests: `check_doc_links.py`, `make ze-doc-test`, cos/iface tests touched by the qos-map path
   - Files: root files + referrers listed in Files to Modify
   - Verify: `git ls-files` root listing clean; doc checks green
2. **Phase: Core import-direction gate (child 2)** - selftest fixtures first (failing), then the dep_audit extension + baseline, then boundary-checker root derivation.
   - Tests: dep_audit selftest fixtures; `plugin_process_boundary_test.go` parity case
   - Files: `scripts/dev/dep_audit.py`, `scripts/checks/plugin_process_boundary.go`, baseline file
   - Verify: fixtures fail -> implement -> pass; `make ze-verify` green on the real tree
3. **Phase: Naming glossary (child 3)** - glossary + trio/rib documentation + rename decision.
   - Tests: `make ze-rules-index` regeneration clean
   - Files: `ai/rules/naming.md`, `ai/rules/INDEX.md`, (shortlist renames only if approved)
   - Verify: glossary present; any rename builds green via the deterministic tool
4. **Phase: Protocol skeleton (child 4)** - design probe table, rule doc, advisory report.
   - Tests: advisory report runs in CI without failing; rules-index clean
   - Files: `ai/rules/protocol-skeleton.md`, report script, `ai/rules/INDEX.md`
   - Verify: report lists current divergences; no build-time enforcement
5. **Full verification** - `make ze-verify` after each child.
6. **Complete spec** - per child: audit tables, learned summary, two-commit closure per `ai/rules/planning.md`.

### Critical Review Checklist (/implement stage 6)

<!-- MANDATORY: Fill with feature-specific checks. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line (per child) |
| Feature completeness | Every End-to-End User Story path connected; gate messages name file + rule doc |
| Correctness | Baseline is shrink-only: fixed file left in baseline -> gate fails (stale-entry semantics identical to the tier baseline) |
| Naming | New rule docs named per `ai/rules/` conventions; glossary terms match actual package doc comments |
| Data flow | Gates read the existing dep_audit import graph; no second parser introduced |
| CLI grammar | N/A - no CLI commands added |
| Registration over hardcoding | Boundary-checker roots derived from the generator, not a second hardcoded list |
| Doctor checks | N/A - no runtime dependencies |
| YANG validation | N/A - no YANG leaves |
| Prometheus counters | N/A - no runtime state |
| Rule: no-layering | Old hardcoded scan-root list fully deleted once derivation lands |
| Rule: comparison-honesty | Any doc text citing holo/gobgp/bio-rd comparisons follows `ai/rules/comparison-honesty.md` |

### Deliverables Checklist (/implement stage 10)

<!-- MANDATORY: Every deliverable with a concrete verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Core import-direction gate wired into verify | run `make ze-verify` on a fixture with a planted upward import: non-zero; on the real tree: green |
| Shrink-only baseline with exactly the 5 known files | read the baseline file; grep `internal/core` for upward imports and diff against it |
| Boundary checker covers all plugin namespaces | selftest output lists derived roots; includes l2tp/firewall plugin dirs |
| Glossary + skeleton rule docs indexed | `ls ai/rules/protocol-skeleton.md`; grep `ai/rules/INDEX.md` for both rows |
| Clean repo root | `git ls-files` shows none of the removed/relocated names at root |
| Corrected overview | grep `docs/` for the stale OSPF/IS-IS claim: no hits |

### Security Review Checklist (/implement stage 11)

<!-- MANDATORY: Feature-specific security concerns. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Gate scripts parse Go files from the repo only; no external input |
| Secrets/topology exposure | `prod.json` carries a real device address and update port in a public repo; the hygiene child must surface this to the user explicitly (A-5) rather than silently keep it |
| Boundary-check regressions | Extending scan roots must not weaken existing findings (selftest keeps the five historical fixture cases green) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Gate false positive on the real tree | Re-derive the violation list (A-1/A-3); adjust baseline seed, never the rule |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The umbrella's Current Behavior referrer list for `qos-map.md`/`test-web`/`AI-NAVIGATION-AUDIT.md` named real references needing updates on move (A-4) | Zero real referrers existed: every hit was the `qos-map` config keyword (`parseQoSMap`, `internal/component/iface/config.go:928,932`) or a learned-summary filename | Child 1 `/ze-implement` assumption audit disambiguated literal path (`qos-map\.md`) from bare token | Child 1 Files to Modify shrank to 4 files; following the umbrella as written would have corrupted live config keywords and broken the vlan-qos parse suite. Lesson in `plan/learned/1088-layout-1-hygiene.md` |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- The comparative review found Ze already has the strengths gobgp is praised for
  (public SDK surface in `pkg/` with private implementations) and avoids
  bio-rd's everything-public sprawl. The two portable lessons are holo's:
  uniform per-protocol skeleton, and dependency-direction rules enforced by the
  build rather than by prose.
- The component/plugins trees are not layers: imports run both ways (381 plugin
  files import component packages; 20 component files import plugins, e.g.
  `internal/component/bfd/bfd.go` line 38). The tiers effort makes placement
  correct; direction WITHIN the pair is intentional (plugins consume platform
  components). The only enforceable direction rule today is "nothing imports
  upward out of `internal/core/`", which is exactly what child 2 gates.
- One of the five upward imports is generated (`internal/core/ipc/yang/register.go`,
  emitted by `scripts/codegen/yang_glue.go`), so baseline rows need a fix-route
  column: hand-fixable vs generator-fixable vs needs-design (the
  `DoctorCheckContext` type dependency).

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
Conventions that survive a 610-package tree are the ones a machine checks. Every
finding in the review traces back to a rule that existed in prose but not in a
gate; this umbrella converts the checkable subset into gates and writes down the
rest as glossary/skeleton rules that new code can follow.

## Key Design Decisions
<!-- Record each significant design choice as it is made. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Complement spec-tiers, never move packages between tiers | fold everything into one structure mega-effort | tiers is in-flight (Phase 4/6) with its own sequencing; duplicate scope produces merge conflicts and contradictory rules |
| Extend `dep_audit.py` for the direction gate | golangci `depguard` | one enforcement home already wired into `make ze-verify`; depguard would be a second mechanism with its own config drift (house precedent: tiers B-1, one discovery source) |
| Shrink-only baseline for the 5 known upward imports | fix all 5 first, then gate | gating first stops NEW violations immediately; fixes land incrementally with the baseline enforcing progress (pattern proven by `tier_migration_baseline.txt`) |
| Glossary-first naming, renames only via approved shortlist | mass renames to a clean vocabulary | 610 packages; churn cost exceeds benefit except where a name is actively misleading (`wireu`) |
| Protocol skeleton advisory before any enforcement | enforced skeleton gate | existing protocols diverge widely (A-6); an enforced gate today would need a huge allowlist, repeating the tiers Path B lesson |
| Reactor split recorded as unscheduled candidate | include as a child now | it is re-architecture, not layout; sequencing belongs after `spec-rib-arch-*` lands (A-7) |

## Known Limitations
<!-- Deliberate scope boundaries and constraints accepted. -->
- Does not move any package between `internal/core/`, `internal/component/`,
  `internal/plugins/` (tiers umbrella owns that, including tiers-5 core split).
- Does not split `internal/component/bgp/reactor` (unscheduled candidate; see
  Child Spec Plan).
- Does not restructure `test/` or the `internal/plugins/*-cmd` YANG-shim dirs;
  the shim layout follows the plugin self-containment mechanics and touching it
  needs its own investigation (raise at child 3 design if the glossary work
  surfaces a cheap improvement).
- Does not decide the host-tier question (tiers-5 B-3).
- Skeleton rule constrains new/touched code; existing protocols are not
  force-migrated.

## RFC Documentation

N/A - no protocol behavior is implemented or changed by this set.

## Implementation Summary

### What Was Implemented
- Child 1 (hygiene): root cleanup + stale-doc fixes (commits `b652e176c`/`5424c170f`; learned 1088).
- Child 2 (core import gate): `core_direction_gate` in `dep_audit.py` + shrink-only `scripts/dev/core_import_baseline.txt` (5 files / 10 pairs, fix routes) + boundary-checker scan roots derived from the generator (13 roots, zero new findings) (commits `095c91fc9`/`87043ee82`; learned 1089).
- Child 3 (naming glossary): package-naming glossary in `ai/rules/naming.md`; `wireu` kept + documented per user decision (commits `4b81e4654`/`4d179ede2`; learned 1090).
- Child 4 (protocol skeleton): `ai/rules/protocol-skeleton.md` + advisory `scripts/dev/protocol_skeleton_report.py` as the non-enforcing last line of `ze-tier-check` (commits `83faa5796`/`8f5f2ff4b`; learned 1091).
- Umbrella closure: reactor-candidate destination created (`plan/spec-reactor-split.md`, skeleton, blocked on rib-arch), all assumptions resolved, this record.

### Bugs Found/Fixed
- Child 4 in-pass review ISSUE: a stale PROTOCOLS manifest row rendered silently as single-package; fixed with a `missing` flag + regression selftest cases (learned 1091).
- No runtime bugs: the set changed no runtime behavior.

### Documentation Updates
- `docs/architecture/overview.md` Non-Goals corrected with dated source anchors (child 1).
- New/extended rule docs: `ai/rules/naming.md` glossary, `ai/rules/protocol-skeleton.md`; `ai/rules/INDEX.md` regenerated (rows 61, 78); `ai/rules/module-tiers.md` + `docs/plugin-overview.md` gate descriptions extended (child 2).

### Deviations from Plan
- A-4 broke: the umbrella's root-file referrer list was bare-token grep hits; the relocations needed zero referrer edits (Mistake Log; learned 1088).
- Child 4 shipped a standalone report script instead of a dep_audit section (umbrella suggested either; the report shares no logic with the import graph -- recorded in the child).
- Child 3's "wireu KEPT" decision was superseded later the same day: the user approved folding `wireu` into the future `bgp/packet` (`plan/spec-rename-0-umbrella.md`, `spec-rename-3-wireu-fold.md`). The glossary row updates land with that spec set, not here.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Gap 1: core tier leaks upward, unguarded | done | `core_direction_gate` in `scripts/dev/dep_audit.py` + `scripts/dev/core_import_baseline.txt` | shrink-only; child 2 |
| Gap 2: boundary guard blind to two namespaces | done | derived scan roots in `scripts/checks/plugin_process_boundary.go` (from `scripts/codegen/plugin_imports.go`) | 13 roots; child 2 |
| Gap 3: no skeleton / no naming glossary | done | `ai/rules/naming.md` glossary (child 3); `ai/rules/protocol-skeleton.md` + advisory report (child 4) | |
| Gap 4: root clutter + stale overview claim | done | child 1 (`b652e176c`) | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | dep_audit selftest fixture: new upward pair -> gate fails | child 2 |
| AC-2 | done | baseline lists exactly the 5 files / 10 pairs with fix routes; stale-row selftest fixture | child 2 |
| AC-3 | done | `--print-roots` lists 13 roots incl. l2tp/firewall plugin dirs; `TestBoundaryScanRootsDerivedFromGenerator` | child 2 |
| AC-4 | done | `ai/rules/naming.md` "Package-Naming Glossary"; `ai/rules/INDEX.md:61` | child 3 |
| AC-5 | done | `ai/rules/protocol-skeleton.md`; report exit 0 (advisory); `ai/rules/INDEX.md:78` | child 4 |
| AC-6 | done | `git ls-files` root check: only `prod.json` of the watched names remains (user decision) | child 1 |
| AC-7 | done | repo grep for the stale OSPF/IS-IS claim: 0 hits | child 1 |
| AC-8 | done | scope statements here + in children point at the tiers umbrella; no tier moves occurred | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| dep_audit core-direction fixtures | done | `scripts/dev/dep_audit.py --selftest` | OK 2026-07-08 |
| boundary derived-roots parity | done | `scripts/checks/plugin_process_boundary_test.go` | `TestBoundaryScanRootsDerivedFromGenerator` |
| rules-index freshness | done | `make ze-rules-index` | regenerates clean (80 rules) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/dep_audit.py` | done | child 2 |
| `scripts/checks/plugin_process_boundary.go` + `_test.go` | done | child 2 |
| `ai/rules/naming.md`, `ai/rules/INDEX.md`, `ai/rules/module-tiers.md` | done | children 2-3 |
| `.gitignore`, `Makefile`, `docs/architecture/overview.md` | done | child 1 |
| struck-through referrer rows (qos-map/test-web docs, .ci files) | not modified | A-4 broken: phantom referrers |
| `ai/rules/protocol-skeleton.md`, baseline file, report script | done | children 2, 4 |
| child specs `spec-layout-1..4` | created + closed | two-commit flow per child; learned 1088-1091 |

### Audit Summary
- **Total items:** 4 requirements, 8 ACs, 3 tests, file plan above
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** A-4 phantom-referrer rows (documented in Deviations + Mistake Log)

## Goal Validation (BLOCKING)

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Core tier cannot grow new upward imports | gate test | `dep_audit.py --selftest` OK (2026-07-08; fixtures: new pair fails, baselined passes, stale row fails); gate wired in `make ze-tier-check`, green at children 2-4 closures |
| Boundary guard covers every plugin namespace | gate selftest | `--print-roots` (2026-07-08): 13 roots including `internal/component/l2tp/plugins` and `internal/component/firewall/plugins`; parity test green |
| Naming/skeleton conventions discoverable and followed | rule docs + index | `ai/rules/INDEX.md:61` (Naming) + `:78` (Protocol Subpackage Skeleton); skeleton already consumed by the follow-on rename set (`spec-rename-*`), which is the "first new work cites them" proof |
| Root and overview match reality | repo listing + grep | `git ls-files` watched-name check: only `prod.json` (2026-07-08); stale OSPF/IS-IS claim grep: 0 hits |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)

Each child ran its own `/ze-review` gate before closure (recorded in the child
specs, preserved in their commit-A history; child 1 predates the practice --
that gap is recorded in the Child 1 section and `plan/learned/1088`). The
umbrella-closure diff itself (this spec's closure edits +
`plan/spec-reactor-split.md` skeleton) was reviewed 2026-07-08:
`make ze-validate` -> "all checks passed";
`scripts/dev/audit-test-relaxation.py` -> "clean (no tests deleted or weakened)".

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | - | none -- 0 BLOCKER, 0 ISSUE, 0 NOTE on the closure diff (docs/spec-only) | - | - |

### Fixes applied
- None required (clean run).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `plan/learned/1088..1091-layout-*.md` | yes | `ls plan/learned/1088* 1089* 1090* 1091*` (2026-07-08) listed all four |
| `scripts/dev/core_import_baseline.txt` | yes | `wc -l` -> 18 lines (10 pair rows + annotations/header) |
| `scripts/dev/protocol_skeleton_report.py` | yes | ran 2026-07-08: exit 0, "7 protocols; canonical 27, rfc-state 4, version 1, domain 29, legacy 4" |
| `ai/rules/protocol-skeleton.md` | yes | `ai/rules/INDEX.md:78` row (regenerated index) |
| `plan/spec-reactor-split.md` | yes | created at this closure (skeleton, Depends `spec-rib-arch-0-umbrella.md`) |
| child specs `spec-layout-1..4` | closed | deleted per two-commit flow; final states in commits `5424c170f`/`87043ee82`/`4d179ede2`/`8f5f2ff4b` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/AC-2 | core gate fails new pairs, enforces shrink-only baseline | `dep_audit.py --selftest` exit 0 (2026-07-08); baseline 10 pair rows |
| AC-3 | all plugin namespaces scanned | `--print-roots` (2026-07-08): 13 roots incl. l2tp/firewall plugin dirs |
| AC-4/AC-5 | glossary + skeleton discoverable | `ai/rules/INDEX.md:61` and `:78` (fresh grep 2026-07-08); report exit 0 |
| AC-6 | root clean | `git ls-files` watched-name check (2026-07-08): only `prod.json` |
| AC-7 | stale claim gone | grep "OSPF and IS-IS are not implemented" in docs/: 0 hits (2026-07-08) |
| AC-8 | no tier moves | no package moved between core/component/plugins in any child commit; scope statements present |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-verify` -> `ze-tier-check` -> core gate + advisory report | N/A (gate selftests, not .ci) | dep_audit selftest OK; report exit 0; ze-tier-check green at children's closures |
| boundary checker scan | N/A (Go selftest) | `TestBoundaryScanRootsDerivedFromGenerator` green (child 2) |
| vlan-qos parse suite (child 1 keyword surface) | `test/parse/iface-vlan-qos.ci` + siblings | green at child 1 closure; `make ze-validate` clean today |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | child 2 audit re-grep: 5 files / 10 pairs, 0 core->plugins (learned 1089) |
| A-2 | confirmed | gate landed inside `dep_audit.py` (commit `095c91fc9`) |
| A-3 | confirmed | extended checker over 13 roots: zero new findings |
| A-4 | broken | phantom referrers; Mistake Log row + learned 1088 |
| A-5 | confirmed | user decision: `prod.json` kept at root unchanged |
| A-6 | confirmed | child 4 probe table: all 7 protocols map; no renames forced (learned 1091) |
| A-7 | confirmed | grep across all nine rib-arch specs: no decomposition scope |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/overview.md` Non-Goals corrected | stale-claim grep 0 hits; dated source anchors added in child 1 | yes |
| rule docs indexed and discoverable | `ai/rules/INDEX.md:61`, `:78` (regenerated, not hand-edited) | yes |
| doc-link + anchor integrity | `make ze-validate` "all checks passed" (2026-07-08, closure diff) | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`scripts/*`, `ai/rules/*`, docs)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)

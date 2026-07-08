# Spec: layout-1 -- Repo-root hygiene + stale architecture-doc fix

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-layout-0-umbrella.md` - set context (this is child 1 of 4)
4. `docs/architecture/overview.md` lines 23-25 - the stale Non-Goals claim
5. `Makefile` line 209 - the `./parked/...` lint invocation

## Task

Umbrella gap 4 (repo-root clutter + stale architecture claim). Low-risk child.
Mechanical repo hygiene plus one factual doc correction:

- Delete `screenlog.0` (tracked empty screen log) and add `screenlog.*` to `.gitignore`.
- Relocate `qos-map.md` → `docs/research/`, `AI-NAVIGATION-AUDIT.md` → `plan/audits/`,
  `test-web` → `scripts/dev/`. Audit (2026-07-08) proved these three files have NO
  referrers outside `plan/` (see Current Behavior + A-1), so the moves are clean
  git-tracked renames with no source edits.
- Remove `parked/` (8 tracked dead-code Go files) and drop `./parked/...` from the
  `Makefile` golangci-lint line (209).
- `prod.json`: KEEP AT ROOT UNCHANGED (user decision 2026-07-08). It is live
  appliance-build input consumed by `Makefile` + `mk/appliance.mk`; its device
  address `10.12.104.10` is private RFC1918 (topology hint, not a routable secret).
- Fix the stale "OSPF and IS-IS are not implemented" claim in BOTH
  `docs/architecture/overview.md` (line 24) and `docs/research/bgp-implementations-analysis.md`
  (line 71), honestly reflecting the actual `internal/plugins/{ospf,isis}` state, with
  source anchors (`ai/rules/documentation.md`, `ai/rules/comparison-honesty.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/overview.md` - lines 23-25 ("Non-Goals: OSPF and IS-IS are not implemented in the current tree")
  → Constraint: contradicted by `internal/plugins/isis` + `internal/plugins/ospf`; the correction must be honest about maturity (`ai/rules/comparison-honesty.md`) and carry a source anchor (`ai/rules/documentation.md`).
- [ ] `docs/research/bgp-implementations-analysis.md` - line 71 comparison-table row repeating the same claim
  → Constraint: AC-2 requires this second occurrence corrected too; it is a comparison doc, so `ai/rules/comparison-honesty.md` governs the wording.
- [ ] `plan/spec-layout-0-umbrella.md` - the umbrella (A-1, A-2, R-1)
  → Decision: `plan/learned/{908,1067}` reference the moved files and stay as historical records; only non-`plan/` referrers are updated (there are none).

**Key insights:**
- The audit dissolved the spec's original risk: `qos-map` in Go/YAML/`.ci` is the
  config keyword (`ingress-qos-map`/`egress-qos-map`), NOT the research file
  `qos-map.md`; `test-web` hits are the learned summary `868-test-web-parallel.md`
  and `test-web-rbac-deny.sh`. The relocated files have zero real referrers.

## Current Behavior (MANDATORY)

**Source files read:** (anchors verified this session, 2026-07-08)
- [ ] `internal/component/iface/config.go` - `parseQoSMap(um, "ingress-qos-map", "priority")` (:928) and `"egress-qos-map"` (:932) prove `qos-map` is a CONFIG KEYWORD, unrelated to the root research file `qos-map.md`
  → Constraint: the `qos-map.md` relocation MUST NOT edit any file that uses this keyword (`iface.go`, `config.go`, `manage_linux.go`, `ze-cos-conf.yang`, `docs/guide/configuration.md`, the vlan-qos `.ci` suites); editing them would corrupt live config parsing and break tests.
- [ ] `scripts/dev/check_doc_links.py` - the doc-link integrity check that must stay clean after relocations
  → Constraint: run it after the moves; a dangling reference fails it.
- [ ] `Makefile` line 209 - `@golangci-lint run ./cmd/ze/... ./internal/... ./pkg/... ./parked/... ./test/...`; `prod.json` consumed by `Makefile` + `mk/appliance.mk`
  → Constraint: removing `parked/` requires dropping `./parked/...` here; `prod.json` stays (user decision).

**Behavior to preserve:**
- All runtime behavior (no code logic changes). The config keyword `qos-map` /
  `ingress-qos-map` / `egress-qos-map` and its parser stay byte-identical.
- `prod.json` stays valid appliance-build input at root, unchanged.
- `plan/learned/` historical references to moved files are left untouched.
- `make ze-lint` stays green after `parked/` and its lint entry are removed together.

**Behavior to change:**
- Root-level stray files deleted/relocated; `screenlog.*` gitignored; `parked/` removed
  and no longer linted; the stale OSPF/IS-IS Non-Goals claim corrected in two docs.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A contributor browses the repo root, or `make ze-doc-test` / `check_doc_links.py`
  walks doc references, or `make ze-lint` lints `./parked/...`.

### Transformation Path
1. Delete `screenlog.0`; add `screenlog.*` to `.gitignore`.
2. Git-rename each of the three root files to its target dir (no referrer edits: audit proved none exist).
3. Remove `parked/` (8 files); delete `./parked/...` from `Makefile` line 209.
4. Correct the OSPF/IS-IS Non-Goals claim in `overview.md` and `bgp-implementations-analysis.md` with source anchors.
5. Record the `prod.json` keep-at-root decision (done in Task).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Doc ↔ source reference | relocated files have no non-`plan/` referrers; moves change no reference | [ ] `check_doc_links.py` clean |
| Makefile ↔ filesystem | `./parked/...` lint entry removed with the directory | [ ] `make ze-lint` green |
| Doc claim ↔ tree reality | OSPF/IS-IS claim corrected to match `internal/plugins/{ospf,isis}` | [ ] grep finds no stale claim |

### Integration Points
- `scripts/dev/check_doc_links.py` + `make ze-doc-test` - prove no dangling references after moves.
- `make ze-lint` - proves the `parked/` removal + Makefile edit leave lint green.

### Architectural Verification
- [ ] No bypassed layers (relocation + doc edits only; no code path changes)
- [ ] No unintended coupling (no new imports; the config-keyword files are untouched)
- [ ] No duplicated functionality (files are renamed, not copied)
- [ ] Registration over hardcoding - N/A (no runtime registration surface touched)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The relocated files (`qos-map.md`, `AI-NAVIGATION-AUDIT.md`, `test-web`) have referrers to update | umbrella Current Behavior list (2026-07-08) | edits to phantom referrers corrupt config keywords / tests | re-grep per file, disambiguating file-path refs from the `qos-map` keyword | **broken** — zero real referrers; all listed hits were the `qos-map` config keyword or the learned summaries `882-vlan-qos-map.md` / `868-test-web-parallel.md` (see Mistake Log) |
| A-2 | `prod.json` disposition is a user decision | Makefile + `mk/appliance.mk` consumers | child blocked on one file | explicit user decision | **confirmed** — user chose keep-at-root unchanged (2026-07-08); device address is private RFC1918 |
| A-3 | `parked/` is dead code with no importers | umbrella ("dead code, still linted") | removing it breaks the build | grep for imports of `parked/` before removal; `make ze-lint`/`ze-verify` after | unvalidated |
| A-4 | OSPF and IS-IS genuinely have implementations (not empty stubs) so "not implemented" is factually wrong | umbrella (339 non-test `.go` files under `internal/plugins/{ospf,isis}`) | the doc correction would overstate support | count non-test `.go` + read a top-level file per protocol before writing the correction | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The doc correction overstates OSPF/IS-IS maturity (comparison-honesty violation) | reviewer disputes the wording | word the correction to state what exists (`internal/plugins/{ospf,isis}`) without claiming production parity; anchor to the tree (`ai/rules/comparison-honesty.md`) |
| R-2 | `git rm -r parked/` removes a file something still imports | `make ze-lint`/build fails after removal | A-3 grep first; if imported, STOP and raise scope with the user |

## Wiring Test (MANDATORY)

Wiring is the doc-link gate, the lint gate, and the repo listing; no runtime feature
(N/A for a `.ci` — this is repo hygiene, not user-facing behaviour).

| Entry Point | → | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-doc-test` | → | no dangling reference after relocations | `scripts/dev/check_doc_links.py` clean |
| `make ze-lint` | → | `parked/` gone + Makefile lint entry removed | `make ze-lint` exit 0 |
| `git ls-files` at repo root | → | none of the removed/relocated names present | root-listing assertion (Deliverables) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Repo root after this child | `screenlog.0` deleted and `screenlog.*` gitignored; `qos-map.md`, `AI-NAVIGATION-AUDIT.md`, `test-web` relocated (git detects renames); `parked/` removed and the Makefile lint line no longer names it; `prod.json` kept at root unchanged (decision recorded here) |
| AC-2 | OSPF/IS-IS stale claim | Corrected in BOTH `docs/architecture/overview.md` (line 24) and `docs/research/bgp-implementations-analysis.md` (line 71) with source anchors; a repo-wide grep for the "not implemented" claim finds no remaining occurrence |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| doc-link integrity | `scripts/dev/check_doc_links.py` | no dangling reference after relocations | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A - repo hygiene + doc fixes, no user-facing behaviour; verified by `check_doc_links.py`, `make ze-lint`, and `git ls-files`, not a `.ci`. The vlan-qos `.ci` suites are unaffected (they use the `qos-map` config keyword, not the moved research file) | doc/lint gates | root is clean; lint green; no dangling doc links | |

## Files to Modify

- `.gitignore` - add `screenlog.*`
- `Makefile` - drop `./parked/...` from the golangci-lint invocation (line 209)
- `docs/architecture/overview.md` - correct Non-Goals OSPF/IS-IS claim (line 24) with a source anchor
- `docs/research/bgp-implementations-analysis.md` - correct the OSPF/IS-IS comparison-table claim (line 71) with a source anchor

Relocations (git-tracked renames, no content or referrer edits):
- `qos-map.md` → `docs/research/qos-map.md`
- `AI-NAVIGATION-AUDIT.md` → `plan/audits/AI-NAVIGATION-AUDIT.md`
- `test-web` → `scripts/dev/test-web`

Removals (tracked):
- `screenlog.0`
- `parked/cbor/{base64,cbor,hex}.go` + their `_test.go`, `parked/reader/reader.go` + `reader_test.go` (8 files)

No change: `prod.json` (kept at root per user decision).

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | no | hygiene only |
| 2 | Config syntax changed? | no | the `qos-map` keyword is untouched |
| 3 | CLI command added/changed? | no | - |
| 4 | API/RPC added/changed? | no | - |
| 5 | Plugin added/changed? | no | - |
| 6 | Has a user guide page? | no | - |
| 7 | Wire format changed? | no | - |
| 8 | Plugin SDK/protocol changed? | no | - |
| 9 | RFC behavior implemented/changed? | no | - |
| 10 | Test infrastructure changed? | no | `test-web` moves but keeps its role |
| 11 | Affects daemon comparison? | yes | `docs/research/bgp-implementations-analysis.md` (OSPF/IS-IS row, AC-2) |
| 12 | Internal architecture changed? | yes | `docs/architecture/overview.md` Non-Goals (AC-2) |
| 13-17 | (metadata/counters/inventory/anchors/examples) | verify | grep `docs/` for source anchors naming moved files; none point at the root research file |

## Files to Create
- `plan/learned/1087-layout-1-hygiene.md` - learned summary at closure
- (relocation targets `docs/research/qos-map.md`, `plan/audits/AI-NAVIGATION-AUDIT.md`, `scripts/dev/test-web` are moves, not new content)

## Implementation Steps

1. **Phase: audit (done + finish)** - A-1 broken (recorded); validate A-3 (grep parked importers) and A-4 (count OSPF/IS-IS `.go`, read one file each) before editing.
2. **Phase: hygiene** - delete `screenlog.0`, gitignore `screenlog.*`, relocate the 3 files, remove `parked/`, drop the Makefile lint entry.
3. **Phase: doc fix** - correct the OSPF/IS-IS claim in both docs with honest wording + source anchors.
4. **Full verification** - `check_doc_links.py`, `make ze-doc-test`, `make ze-lint` (Makefile + Go removal touched the build), `make ze-verify` per the git-safety table.
5. **Complete spec** - learned summary (1087), two-commit closure per `ai/rules/planning.md`.

## Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | No file using the `qos-map` config keyword was edited; only the research file moved |
| Completeness | Every AC has file:line evidence; root listing clean; both stale-claim docs corrected |
| Comparison honesty | The OSPF/IS-IS correction states what exists without claiming production parity (`ai/rules/comparison-honesty.md`) |
| Data flow | Relocations are renames (git shows rename), not copies; no dangling references |
| No-layering | `parked/` fully removed AND its lint entry removed in the same change (`ai/rules/no-layering.md`) |
| Registration over hardcoding | N/A - no runtime registration surface |

## Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Clean repo root | `git ls-files` shows none of `screenlog.0`, `qos-map.md`, `AI-NAVIGATION-AUDIT.md`, `test-web`, `parked/` at root |
| Relocated files present | `ls docs/research/qos-map.md plan/audits/AI-NAVIGATION-AUDIT.md scripts/dev/test-web` |
| Corrected docs | grep the OSPF/IS-IS "not implemented" claim across `docs/`: no hits; source anchors present |
| No dangling links | `scripts/dev/check_doc_links.py` + `make ze-doc-test` clean |
| Lint intact | `make ze-lint` exit 0 after `parked/` + Makefile edit |
| prod.json unchanged | `git diff prod.json` empty |

## Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Topology exposure | `prod.json` device address surfaced to the user (A-2); user chose keep-at-root; address is private RFC1918, not a routable secret |
| No secret movement | none of the relocated files carry credentials; `prod.json` is not moved |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `qos-map.md`, `test-web` have many referrers to rewrite on move (umbrella Files to Modify) | They have zero real referrers; every listed hit was the `qos-map` config keyword or the learned summaries `882-vlan-qos-map.md` / `868-test-web-parallel.md` | `/ze-implement` step-3 assumption audit: disambiguated `qos-map\.md` (file) from `qos-map` (keyword) and path-`test-web` from token | Files to Modify shrank drastically; editing the phantom referrers would have corrupted live config keywords and broken the vlan-qos tests |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Spec conflated a config keyword with a like-named doc file | 1 (this spec) | when a spec lists referrers, disambiguate keyword vs filename before trusting the list | note in learned 1087 |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 demonstrated with evidence
- [ ] `git ls-files` root listing shows none of the removed/relocated names
- [ ] `check_doc_links.py` + `make ze-doc-test` + `make ze-lint` clean
- [ ] `prod.json` unchanged; decision recorded
- [ ] A-1..A-4 all resolved (confirmed/broken) with evidence
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (N/A here; recorded)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Umbrella / siblings: `plan/spec-layout-0-umbrella.md`, `plan/spec-layout-2-core-import-gate.md`, `plan/spec-layout-3-naming-glossary.md`, `plan/spec-layout-4-protocol-skeleton.md`.

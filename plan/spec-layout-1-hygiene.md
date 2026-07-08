# Spec: layout-1 -- Repo-root hygiene + stale architecture-doc fix

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-layout-0-umbrella.md` - set context (this is child 1 of 4)
4. `docs/architecture/overview.md` lines 23-26 - the stale Non-Goals claim
5. `Makefile` line 209 - the `./parked/...` lint invocation

## Task

Umbrella gap 4 (repo-root clutter + stale architecture claim). Lowest-risk child,
zero Go changes. Mechanical repo hygiene plus one factual doc correction:

- Delete `screenlog.0` (empty screen log) and add `screenlog.*` to `.gitignore`.
- Relocate `qos-map.md` -> `docs/research/`, `AI-NAVIGATION-AUDIT.md` -> `plan/audits/`,
  `test-web` -> `scripts/dev/`, updating every non-`plan/` referrer in the same change.
- Remove `parked/` (dead code) and drop `./parked/...` from the `Makefile` lint line.
- Record the `prod.json` disposition as an explicit user decision (keep at root, move,
  or strip the device address) -- `prod.json` is appliance-build input, NOT junk.
- Fix `docs/architecture/overview.md` Non-Goals (lines 23-26): OSPF and IS-IS ARE
  implemented; correct the claim with a source anchor.

This is a skeleton (captured intent). Moves to `design` when picked up.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/overview.md` - lines 23-26 ("Non-Goals: OSPF and IS-IS are not implemented")
  → Constraint: contradicted by `internal/plugins/isis` + `internal/plugins/ospf` (339 non-test `.go` files this session); factual doc changes need a source-anchor comment (`ai/rules/documentation.md`).
- [ ] `ai/rules/documentation.md` - source-anchor convention for factual doc claims
  → Constraint: every corrected claim carries a `source:` anchor naming the tree that proves it.
- [ ] `plan/spec-layout-0-umbrella.md` - the umbrella (A-1, A-2, R-1)
  → Decision: `plan/learned/` references to moved files stay as historical records; only non-`plan/` referrers are updated.

**Key insights:**
- The only decision item is `prod.json`; everything else is mechanical relocation
  or deletion. Each root-file move must re-grep its referrers at move time (A-1).

## Current Behavior (MANDATORY)

**Source files read:** (re-grep referrers at move time per A-1)
- [ ] `scripts/dev/check_doc_links.py` - the doc-link integrity check that must stay clean after every relocation
  → Constraint: run it after each move; a dangling reference fails it.
- [ ] `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` - references `qos-map` by path
  → Constraint: its path reference must be updated in the same change as the `qos-map.md` move.
- [ ] `internal/plugins/cos/yang/ze-cos-conf.yang` - embeds the `qos-map` path (non-`.go` referrer)
- [ ] `docs/guide/configuration.md` - references `qos-map` (user guide)
- [ ] `docs/architecture/testing/runner-architecture.md` - references `test-web`
- [ ] `Makefile` line 209 - `./parked/...` in the lint invocation; `prod.json` consumed by `Makefile` + `mk/appliance.mk`
  → Constraint: `prod.json` carries a real device address and update port in a public repo; surface this to the user (A-2), do not silently keep it.

**Behavior to preserve:**
- All runtime behavior (no Go changes). `prod.json` stays valid appliance-build input.
- Every `qos-map` reference in YANG/tests/docs keeps resolving after the move.
- `plan/learned/` historical references to moved files are left untouched.

**Behavior to change:**
- Root-level stray files deleted/relocated; `screenlog.*` gitignored; `parked/` removed
  and no longer linted; `docs/architecture/overview.md` Non-Goals corrected.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A contributor browses the repo root, or `make ze-doc-test` / `check_doc_links.py`
  walks doc references, or the Makefile lints `./parked/...`.

### Transformation Path
1. Delete `screenlog.0`; add `screenlog.*` to `.gitignore`.
2. Move each root file to its target dir; re-grep its referrers; rewrite every non-`plan/` reference.
3. Remove `parked/`; delete `./parked/...` from the `Makefile` lint line.
4. Correct `docs/architecture/overview.md` Non-Goals with a source anchor.
5. Record the `prod.json` decision in this spec once the user answers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Doc <-> source reference | relocated path rewritten in every non-`plan/` referrer | [ ] `check_doc_links.py` clean |
| Makefile <-> filesystem | `./parked/...` lint target removed with the directory | [ ] lint runs without `parked` |
| Test <-> data file | `qos-map` path updated in the vlan-qos suite | [ ] parse `.ci` suites green |

### Integration Points
- `scripts/dev/check_doc_links.py` - proves no dangling references after moves.
- `make ze-doc-test` - doc integrity gate.
- `test/parse/iface-vlan-qos*.ci` - the user-visible suites that embed `qos-map`.

### Architectural Verification
- [ ] No bypassed layers (relocation only; no code path changes)
- [ ] No unintended coupling (no new imports; docs and data files only)
- [ ] No duplicated functionality (files move, they are not copied)
- [ ] Registration over hardcoding - N/A (no runtime registration surface touched)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each root-file move has only the referrers found this session | grep across repo per filename (2026-07-08) | broken link or build break after move | re-grep per file at move time; `check_doc_links.py` + `make ze-doc-test` after | unvalidated |
| A-2 | `prod.json` disposition is a decision the user makes at approval | Makefile + `mk/appliance.mk` consumers; content read this session | child blocked on one file | explicit user decision recorded here | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Relocating `qos-map.md` breaks YANG/test references that embed the path | `ze-cos-conf.yang` or the vlan-qos lab test fails | update all referrers in the same change; run cos/iface tests + `make ze-doc-test` before presenting |
| R-2 | Silently keeping `prod.json`'s device address in a public repo | reviewer notices the address post-merge | surface it to the user (A-2); record the explicit decision here |

## Wiring Test (MANDATORY)

Wiring is the doc-link gate and the repo listing; no runtime feature (N/A for a new
`.ci` beyond the pre-existing parse suites the relocation must not break).

| Entry Point | → | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-doc-test` | → | no dangling reference after relocations | `scripts/dev/check_doc_links.py` clean |
| `git ls-files` at repo root | → | none of the removed/relocated names present | root-listing assertion (this spec's Deliverables) |
| parse the vlan-qos config | → | updated `qos-map` path still resolves | `test/parse/iface-vlan-qos.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Repo root after this child | `screenlog.0` deleted and `screenlog.*` gitignored; `qos-map.md`, `AI-NAVIGATION-AUDIT.md`, `test-web` relocated with all non-`plan/` referrers updated; `parked/` removed and the Makefile lint invocation no longer names it; `prod.json` disposition recorded as an explicit user decision |
| AC-2 | `docs/architecture/overview.md` after this child | Non-Goals no longer claims OSPF/IS-IS are unimplemented; corrected claims carry source anchors; repo-wide grep finds no other doc making the stale claim |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| doc-link integrity | `scripts/dev/check_doc_links.py` | no dangling reference after relocations | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| vlan-qos parse suite stays green after the `qos-map` move | `test/parse/iface-vlan-qos.ci`, `test/parse/iface-vlan-qos-invalid.ci`, `test/parse/cos-profile-conflict.ci`, `test/parse/iface-vpp-rejects-nonidentity-qos.ci` | these `.ci` tests reference `qos-map`; updating their references must not change user-visible parse behaviour | |

## Files to Modify

- `.gitignore` - add `screenlog.*`
- `Makefile` - drop `./parked/...` from the lint invocation (line 209)
- `docs/architecture/overview.md` - correct Non-Goals (lines 23-26) with a source anchor
- `docs/guide/configuration.md` - `qos-map.md` path update
- `internal/plugins/cos/yang/ze-cos-conf.yang` - `qos-map` path update
- `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` - `qos-map` path update
- `test/parse/iface-vlan-qos.ci`, `test/parse/iface-vlan-qos-invalid.ci`, `test/parse/cos-profile-conflict.ci`, `test/parse/iface-vpp-rejects-nonidentity-qos.ci`, `test/vlan-qos-lab/run.sh` - `qos-map` references (re-grep at move time per A-1)
- `docs/architecture/testing/runner-architecture.md` - `test-web` path update
- `Makefile` / `mk/appliance.mk` / `docs/guide/appliance.md` - only if the user decides to move `prod.json`
- root files removed: `screenlog.0`, `qos-map.md`, `AI-NAVIGATION-AUDIT.md`, `test-web`, `parked/`

## Implementation Steps

1. **Phase: audit** - re-grep every root-file referrer (A-1); surface the `prod.json` decision to the user (A-2).
2. **Phase: relocate** - delete/move files, rewrite referrers, gitignore `screenlog.*`, drop the `parked` lint target.
3. **Phase: doc fix** - correct the overview Non-Goals with a source anchor.
4. **Full verification** - `check_doc_links.py`, `make ze-doc-test`, the vlan-qos parse `.ci` suites.
5. **Complete spec** - learned summary, two-commit closure per `ai/rules/planning.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 demonstrated
- [ ] `git ls-files` root listing shows none of the removed/relocated names
- [ ] `check_doc_links.py` + `make ze-doc-test` clean
- [ ] `prod.json` decision recorded as an explicit user choice
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected (N/A here; recorded for completeness)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `plan/spec-layout-0-umbrella.md`, `plan/spec-layout-2-core-import-gate.md`, `plan/spec-layout-3-naming-glossary.md`, `plan/spec-layout-4-protocol-skeleton.md`.

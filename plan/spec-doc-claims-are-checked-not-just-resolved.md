# Spec: doc-claims-are-checked-not-just-resolved

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `spec-fixit-dead-design-pointers-in-tests`, closed 2026-08-10 (`90082fb08`) |
| Phase | 5/5 (implementation green; closure is `/ze-close` on the review model) |
| Deferral shard | `plan/deferrals/doc-claims-are-checked-not-just-resolved.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every documentation-freshness gate in this repository verifies REFERENCE
INTEGRITY and none verifies CLAIM TRUTH. A path that exists, a line inside a
file, a pointer that resolves: all checked. Whether the sentence above the
anchor describes what the named function does: never checked.

Measured while closing `spec-problem-journal`, which moved 1155
`// Design:` pointers and wrote about 130 new architecture documents:

| Evidence | Number |
|---|---|
| Go-targeted `<!-- source: -->` anchors swept | 1611 |
| Anchors naming a symbol ABSENT from the file they point at | 82 |
| Of those, found by an independent reviewer sampling by hand | 7 |
| False prose claims in one IKE page, each above a resolving anchor | 3 |
| Dead citations hidden behind a `doc-links: ignore` marker no gate reads | 98 |

Every one of the 82 passed a green gate. A resolving anchor lends a false
sentence credibility, which is the mechanism this spec exists to break.

Three changes. The `_test.go` blindness in `check_design_refs` is the fourth and
it has its own spec, named in Depends.

1. **The source-anchor gate verifies the symbols it already carries.** An anchor
   is `<!-- source: <path> -- Sym1, Sym2 -->`. Both anchor walkers verify
   `<path>` and discard everything after the `--`: `check_source_anchor_stale_paths`
   in `scripts/dev/validate.py`, and `extract_paths` in `scripts/dev/code_to_docs.py`.
   Verify the symbols too, in `code_to_docs.py`, because that is the walker
   `make ze-verify` reaches (see Integration Points).
2. **A suppression carries a reason a gate reads.** `doc-links: ignore` is
   honoured by `check_doc_links.py` and read by nothing else, so 98 dead
   citations sat behind it while `digest_check.py` was hard red on the same
   lines. A marker with no audited reason is a silent allowlist.
3. **An independent reader is mandatory where prose makes claims.** Only a
   reader can falsify a sentence. `/ze-review-docs` exists; nothing requires it
   when a spec touches a subsystem document.

**The research phase carries one more item, and it is an owner request rather
than a derivation from the three above.** Thomas asked on 2026-08-09 for a full
audit of whether the rule corpus keeps design documentation current as code and
features are added, to run at the end of the problem-journal work rather than
during it. It is the second row of this spec's deferral shard. The three changes
above came from the gates that were READ while moving 1155 pointers; the audit
asks the wider question that reading never covered, so answering the three does
not answer it. Research reports it as its own finding set, and every change it
drives is either named here or homed in its own spec before this one closes.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - which gate owns which surface
  → Constraint: a new gate is registered in the gate list, or nobody discovers it
- [ ] `ai/rules/evidence.md` - why a claim needs its producer read
  → Constraint: this spec mechanises the cheap half; the reader is still owed for the rest

**Key insights:**
- The symbol list already exists in every anchor. This is a gate that stopped
  short, not a convention that needs inventing.
- `gopls symbols <file>` resolves a file's declarations for about 1.3 KB against
  44 KB for the file itself (`ai/rules/context-economy.md`), so the check is cheap.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/code_to_docs.py` - `main()` check mode walks every anchor under `docs/`; `ANCHOR_RE` captures the whole body and `extract_paths()` splits segments on `;`, drops the tail at `DESC_SEP`, and keeps paths under `PATH_PREFIX` only. This is the walk `make ze-doc-test` runs
- [ ] `scripts/dev/validate.py` - `check_source_anchor_stale_paths()` verifies the path only; `SOURCE_ANCHOR_RE` captures it. `run_checks` calls it, and `stagesForMode` in `scripts/status/verify_run.go` reaches neither
- [ ] `scripts/dev/check_doc_links.py` - `check_design_refs()`, `path_resolves()`, and the `doc-links: ignore` handling
- [ ] `scripts/dev/digest_check.py` - `check_digest()` verifies `file:line` exists and is in range, and does NOT read `doc-links: ignore`
- [ ] `ai/skills/ze-review-docs.md` - the reader that can falsify a claim

**Behavior to preserve:**
- Every current check keeps its coverage. This adds, it never relaxes.
- An anchor whose target is outside the repo stays exempt: `check_source_anchor_stale_paths` already documents why a `~/` or `/` path cannot be resolved here.

**Behavior to change:**
- A symbol named in an anchor must exist in the anchored file.
- A `doc-links: ignore` marker must carry a reason, and an unreasoned marker fails.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-doc-test` and `make ze-validate` run the gates over `docs/` and `ai/digests/`.

### Transformation Path
1. `SOURCE_ANCHOR_RE` captures the path; the symbol list after `--` is currently discarded.
2. The new check resolves the file's declarations and compares.
3. A symbol that names a call, a parameter, or an env key rather than a declaration is the known false-positive shape: 18 of the 82 were exactly that.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ Go type information | a declaration scan of the anchored file's own text, never `gopls` | Yes: `code_to_docs.py` shells out to nothing, so the gate needs no language server and no build context |

### Integration Points
- `scripts/dev/code_to_docs.py` - extend the anchor walk in `main()`: `ANCHOR_RE` already
  captures the whole anchor body and `extract_paths()` already segments it on `;` and
  discards the tail after `--`. The symbols are in hand there and are thrown away.
- The design phase named `scripts/dev/validate.py` `check_source_anchor_stale_paths`.
  Rejected on evidence: that walk is a SECOND walk over `docs/`, it reads only the first
  path of a multi-path anchor, and no gate reaches it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The false-positive rate is manageable, because a token after `--` may legitimately name a call or a field rather than a declaration | 18 of 82 flagged tokens were accurate on inspection | the gate cries wolf and gets ignored | run the check over the whole tree before arming it, and count | **broken as stated**: 372 unresolved of 4779 claims checked over 5315 anchor segments, and 250 (67%) are the legitimate non-declaration shape (prose noun 105, string key 63, in-file text 34, call 26, receiver member 24). R-1's classification is what makes it carryable: drop all-lowercase single-word tokens, and report only a claim the anchored file's text does not hold anywhere. That leaves 122 over 43 doc files (`tmp/anchor-claims-worklist.md`), of which 81 name a symbol declared nowhere in the repo |
| A-2 | Symbols behind a build tag resolve | `gopls` uses one build context, which already cost this work an anchor error | linux-only declarations read as absent | run the sweep under `GOOS=linux` too and diff the finding sets | **confirmed**: `go_declarations` reads file text and takes no `GOOS` input, so no sweep can differ. Measured on darwin, it extracts 27, 15, 14 and 8 declarations from `manage_linux.go`, `netlink_linux.go`, `attach_linux.go` and `tuning_linux.go`; over 56 linux-only anchored files it resolves 124 of 145 claims and none of the 21 remaining is a hidden declaration (`IP_TTL`, `SO_BINDTODEVICE`, `ebpf.NewCollection`). The row's `gopls` basis did not reproduce: `gopls symbols` returned the same sets on darwin for the same four files |
| A-3 | Every `doc-links: ignore` marker in the tree today can be given a reason or removed | at HEAD, 46 hits over 19 files: 37 markers in 18 files, of which 31 already state a reason, plus 9 prose mentions of the words, all inside this spec. The 6 unreasoned markers are 3, each written twice (a point file and the rule rendered from it), and each hid a dead `plan/learned/<id>-*.md` citation | arming the check reds the tree | counted twice, by the checker's own grammar over `git ls-files` and over `git grep HEAD`; the 3 were repaired by deleting the dead citation with its marker, and `make ze-doc-links` is green with no unreasoned marker left | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The symbol check floods on the non-declaration shape and is switched off | more findings than a reviewer will read | classify a token as a declaration, a member, or free text, and gate only on declarations first |
| R-2 | Mandating `/ze-review-docs` makes every doc-touching spec slower and gets waived | the waiver appears in specs | scope the requirement to specs that CREATE a doc or change a claim, not to a typo fix |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A doc gate reds on correct documentation. Nothing user-visible |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every session that writes a doc |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-doc-test` | → | `code_to_docs.py` anchor symbol check | `test_anchor_naming_an_absent_symbol_fails` |
| `make ze-doc-links` | → | `check_doc_links.py` marker-reason check | `test_ignore_marker_without_a_reason_fails` |

Both entry points are stages of `make ze-verify`: `stagesForMode` in
`scripts/status/verify_run.go` lists `ze-doc-test` and `ze-doc-links` in the
`ze-verify-changed` and full-verify branches. `make ze-validate`, which the
design phase named, is in NEITHER branch and has no other caller, so a check
placed there is enforced by nothing. That is why the symbol check moved.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An anchor names a symbol not declared in the anchored file | the gate reports the file, the anchor, and the symbol |
| AC-2 | An anchor names a symbol that IS declared there | no finding |
| AC-3 | An anchor names a call, a field, or an env key rather than a declaration | no finding, or a finding at a severity the tree can carry, per A-1's measurement |
| AC-4 | A `doc-links: ignore` marker carries no reason | the gate fails and names the line |
| AC-5 | A spec creates a `docs/architecture/` file or changes a claim in one | closure requires a recorded `/ze-review-docs` pass |
| AC-6 | The whole tree, after the work | `make ze-validate` and `make ze-doc-test` are green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_anchor_naming_an_absent_symbol_fails` | `scripts/dev/code_to_docs_test.py` | AC-1 | |
| `test_anchor_naming_a_declared_symbol_passes` | `scripts/dev/code_to_docs_test.py` | AC-2 | |
| `test_member_token_is_not_flagged` | `scripts/dev/code_to_docs_test.py` | AC-3 | |
| `test_ignore_marker_without_a_reason_fails` | `scripts/dev/check_doc_links_test.py` | AC-4 | |

### Functional Tests

The subject is a documentation gate, so the end-to-end proof is the gate running
inside its make target over the real tree, plus the existing suites staying green.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-doc-test` | `Makefile` | an agent cannot land a doc claiming a symbol that does not exist | |
| `make ze-plugin-test` | `test/plugin/*.ci` | the gate change breaks no daemon behaviour | |

## Files to Modify
- `scripts/dev/code_to_docs.py` - the symbol check, inside the anchor walk `make ze-doc-test` runs
- `scripts/dev/code_to_docs_test.py` - its tests
- `scripts/dev/check_doc_links.py` - the marker-reason requirement
- `scripts/dev/check_doc_links_test.py` - its test
- `ai/rules/planning.md` (via its point files) - when `/ze-review-docs` is owed
- `ai/rules/repo-maintenance.md` (via its point files) - register the new gate

## Files to Create
- none expected

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no config surface |
| YANG custom validators | N-A | no config surface |
| CLI commands/flags | N-A | a make target |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no config surface |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no route output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | the check scans the anchored file's text and shells out to nothing, so it adds no runtime dependency |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface | N-A | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | agent tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` and the `ai/rules/repo-maintenance.md` gate list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on the changed scripts |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the check exists and is reachable, reporting nothing
   - Tests: `test_anchor_naming_a_declared_symbol_passes`
   - Files: `scripts/dev/code_to_docs.py`, `code_to_docs_test.py`
   - Verify: `make ze-doc-test` runs the new check and the tree is unchanged
2. **Phase: Measure before arming** -- run the check over the whole tree, under both `GOOS` values, and count
   - Verify: A-1 and A-2 answered with numbers, and the severity chosen from them rather than guessed
3. **Phase: Arm the symbol check** -- fix or reclassify what it finds, then let it fail the gate
   - Tests: `test_anchor_naming_an_absent_symbol_fails`, `test_member_token_is_not_flagged`
4. **Phase: The suppression reason** -- `doc-links: ignore` requires a reason
   - Tests: `test_ignore_marker_without_a_reason_fails`
   - Verify: count the surviving markers first, per A-3
5. **Phase: The reader** -- `/ze-review-docs` owed when a spec creates a doc or changes a claim
   - Files: the point files under `ai/rules/points/planning/`, then `make ze-rules-condensed`
   - Also: register both new checks in the gate list, and correct the false sentence
     the audit found. `ai/rules/repo-maintenance.md` and its point file
     `ai/rules/points/repo-maintenance/discovery-updates/the-discovery-surface-that-answers-each-need.md`
     both state that `make ze-doc-links` is "folded into `make ze-doc-test`". The
     `ze-doc-test` recipe in `mk/inventory.mk` runs no `check_doc_links.py`, and
     `ze-doc-links` is its own verify stage. A rule asserting a gate that does not
     fire is this spec's own subject matter, so it is corrected here rather than deferred

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A declaration behind a build tag is found, not reported absent |
| Naming | The finding names the anchor and the symbol, not only the file |
| Data flow | One check, extending the existing anchor walk, not a second walk over `docs/` |
| Rule: `ai/rules/evidence.md` | The gate must fail CLOSED: an unreadable file is a finding, never a pass |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The symbol check exists and fails on an absent symbol | `python3 scripts/dev/code_to_docs_test.py` |
| The tree is clean under it | `make ze-doc-test` and `make ze-doc-links` |
| No unreasoned suppression survives | `git grep -n 'doc-links: ignore'` reviewed against the gate |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Repo-authored input only. An unreadable or unparseable file must be a finding, not a skip |

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

- A resolving anchor makes a false sentence MORE credible, not less. That is why
  the reviewer who found the first false claims had to be told to hunt claims
  rather than links.
- The rule text is documentation too. Two rule files asserted that a closure gate
  fired after it had silently stopped firing, and no gate can check a rule's
  claim about a gate. Only a reader catches that.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Check the symbols already in the anchors | Invent a new annotation | The convention exists and is populated. This is a gate that stopped short |
| Put the check in `code_to_docs.py`, not `validate.py` | Keep the design-phase home; or wire `ze-validate` into `make ze-verify` | `stagesForMode` in `scripts/status/verify_run.go` runs neither `ze-validate` nor anything that depends on it, so the design-phase home is unenforced. Wiring `ze-validate` in would arm five unrelated checks at once, which is a separate defect and gets its own spec |
| Resolve symbols by scanning the anchored file's text | `gopls symbols` | A text scan has no build context, so a `//go:build linux` declaration is found rather than reported absent. It answers A-2 by construction and removes the `gopls` runtime dependency the Integration Checklist flagged |
| Measure before arming | Arm and fix the fallout | 82 findings were real, but 18 flagged tokens were accurate. Arming first would teach everyone to ignore the gate |
| Keep the human reader for claim truth | Try to verify prose mechanically | A sentence is falsifiable only by reading the producer. The gate buys the cheap half |
| Rule 1 drops a token only when it carries no capital AND no `_` and no `.` | Drop every token with no capital | The narrow form is what phase 2 measured: it removes 105 prose nouns. The wide form removes 118, and the 13 extra include `sa_count`, `tunnel_up` and `ze.storage.blob`, which are true findings the worklist keeps. A separator says identifier even with no capital |
| Rule 2 searches the anchored file's text with a word boundary | Search for a substring; or keep reporting every undeclared claim | `\b` treats `.` as a boundary, so `Run` is found in `p.Run()` while `Runner` does not satisfy a claim on `Run`. Reporting all 372 floods the gate, which is R-1 |

## Known Limitations
- A symbol that exists and a sentence that is wrong about it still passes. This
  closes the anchor half, not the prose half.
- **The two severity rules demote 250 of the 372 unresolved claims, and 70 of
  those are a real defect the gate now stays silent about.** Rule 1 drops 105
  single lowercase words. Rule 2 drops 230 claims the anchored file names
  without declaring. They overlap on 85. Of the 230, 70 point at the WRONG
  FILE: 46 name a declaration in a sibling file of the same package, and 24
  name one in another package (`tmp/anchor-claims-worklist.md`, shape table).
  Both shapes send the reader to a file that mentions the symbol but does not
  hold it. R-1's mitigation is "gate on declarations first", so 70 is the
  population a later tightening starts from, not zero.
- **Rule 2 passes a rename whose old name survives in a comment of the same
  file.** The check cannot separate a stale claim from an accurate one about a
  call or a comment, because both read as text in the file. Pinned by
  `test_rule_2_costs_the_claim_a_file_only_mentions`.
- **Rule 1 gives up the all-lowercase Go declaration.** An anchor can no longer
  claim `run` or `main`, because nothing separates them from an English word in
  a prose list. Pinned by `test_rule_1_costs_the_all_lowercase_go_declaration`.
- AC-5 arms the reader when a spec CREATES a `docs/architecture/` page or CHANGES
  a claim in one. A spec that adds a package with NO page does neither, so AC-5
  stays silent on the blindest case. Widening it here would order a review of a
  document that does not exist. The case is homed in
  `spec-code-can-land-with-no-design-doc`, which makes the page owed in
  the first place.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional coverage: the gate runs inside its make target
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal row written for anything this teaches
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-doc-claims-are-checked-not-just-resolved.md` only

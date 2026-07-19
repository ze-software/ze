# Spec: fixit-pppoe-orphaned-tests

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/functional-test-gate.md`, `ai/rules/no-workarounds-for-missing-behavior.md`
4. `internal/test/runner/record_parse.go` (`parseOption`), `internal/test/cli/register.go`

## Task

**`test/pppoe/` is orphaned AND now redundant dead code.** It cannot parse, nothing
runs it -- but PPPoE Access is NOT uncovered. The 3 `.ci` are stale artifacts of an
abandoned netns approach that two real functional labs (landed 2026-06-20) superseded.
Both labs exercise RFC 2516 discovery against a real accel-ppp peer:

| Coverage that already exists | Entry point | What it drives |
|------------------------------|-------------|----------------|
| accel-ppp Docker lab | `make ze-deployment-pppoe-accel-docker-test` -> `test/pppoe-interop/run.py` (`lab.py`, scenario `test/pppoe-interop/scenarios/01-pppoe-chap-ipv4/`) | Ze's `pppoe-client` interface vs a real accel-ppp AC over a Docker bridge; PPPoE discovery (EtherType 0x8863) |
| QEMU accel-ppp lab | `ze-qemu-pppoe-accel-test` (`mk/test-integration.mk:423`) -> `scripts/evidence/effective-pppoe-accel.py` | Ze's PPPoE client vs a real accel-ppp AC on a CONFIG_PPPOE kernel in QEMU |

The `.ci` (`pppoe-basic.ci`, `pppoe-vlan.ci` both 2026-05-28) predate that lab
(2026-06-20); the netns primitive they assume was never built and the labs made it
unnecessary. This is therefore **low-priority housekeeping (delete stale files), NOT a
coverage gap** -- sequence it below active feature work.

The narrow defect is nonetheless real. Two independent reasons keep the 3 files silent,
both verified at the producer on 2026-07-16:

| # | Claim | Verification |
|---|-------|--------------|
| 1 | The `.ci` files use an option type the parser rejects | `parseOption` (`internal/test/runner/record_parse.go:295-448`) switches on `optType`. Its 12 cases are: `file`, `asn`, `bind`, `timeout`, `tcp_connections`, `open`, `update`, `env`, `skip-os`, `needs-linux`, `skip-env`, `require-tag`. There is **no `netns` case**, so `option=netns:veth=...` falls to the `default:` branch at `:444-445`, which returns `fmt.Errorf("unknown option type %q", optType)`. |
| 2 | No suite registers pppoe | `internal/test/cli/register.go` calls `registerCIRoot` 20 times at `:17-36` (appliance, firewall, flow-export, install, ipsec, isis, isis-wire, ospf-wire, ospf, ospfv3, l2tp, l2tp-wire, ldp, managed, policy, rsvpte, static, traffic, ui, vrrp). **pppoe is not among them**; `grep -rni pppoe internal/test/cli/` returns nothing. |

All **3** files are affected, and all 3 carry the rejected directive:

| File | `option=netns` line |
|------|---------------------|
| `test/pppoe/pppoe-basic.ci` | `:149` — `option=netns:veth=veth-bng,veth-sub` |
| `test/pppoe/pppoe-vlan.ci` | `:110` — `option=netns:veth=veth-bng,veth-sub:vlan=100` |
| `test/pppoe/pppoe-concurrent-l2tp.ci` | `:145` — `option=netns:veth=veth-bng,veth-sub` |

→ Constraint: `test/pppoe/` are the **only** consumers of `option=netns` in the whole
tree (`grep -rln "option=netns" test/` returns exactly these three). Nothing else
depends on the directive, so repairing it serves only these tests.

### Options to consider

**Recommended: Option D + the `TestCIRootsRegistered` guard.** Because the QEMU and
Docker accel-ppp labs already cover RFC 2516 discovery, repairing the orphaned `.ci`
(Options A/B/C) buys no coverage that does not already exist, while Option A alone is a
large netns/veth/root test-infrastructure project. Delete the 3 stale files and keep
only the generalizable guard so the next orphaned suite is caught automatically.

| Option | What it means | Notes already known |
|--------|---------------|---------------------|
| D. Delete them (recommended) | Remove `test/pppoe/` (the 3 stale `.ci`) | Requires user approval per `ai/rules/never-destroy-work.md`. Removes NO coverage: PPPoE RFC 2516 discovery is exercised by `test/pppoe-interop/` and `ze-qemu-pppoe-accel-test`. The `.ci` remain in git history. |
| C'. Guard against recurrence (recommended alongside D) | Add `TestCIRootsRegistered` (assert every `test/` subdir with `.ci` files is rooted) | Independent of D and worth keeping regardless: it is what would have caught this orphan in May |
| A. Repair the directive | Implement a `netns` case in `parseOption` plus the veth/netns setup it implies | Hard to justify now: needs root/CAP_NET_ADMIN, is Linux-only (`ai/rules/qemu-testing.md`), the runner has no netns primitive, AND the labs already cover the path. Not recommended. |
| B. Re-mark the tests | Rewrite the 3 `.ci` onto directives that already exist | All 3 already carry `option=skip-os:value=darwin` (`pppoe-basic.ci:9`, `pppoe-vlan.ci:7`, `pppoe-concurrent-l2tp.ci:7`); re-marking them to parse-and-skip is a false green (`ai/rules/no-workarounds-for-missing-behavior.md`). Not recommended. |
| C. Register a suite | Add `registerCIRoot("pppoe", ...)` to `internal/test/cli/register.go` | Only meaningful as a prerequisite for A/B (both not recommended); alone it turns silent death into a loud parse failure |

→ Decision for the user: confirm deletion (Option D) plus landing `TestCIRootsRegistered`.
Options A/B/C are recorded for completeness but are not recommended, because the coverage
they would build already exists.

→ AUTONOMOUS DEFAULT (2026-07-17): **Adopt Option D (delete the 3 stale `test/pppoe/*.ci`)
+ the `TestCIRootsRegistered` guard (C') as the plan of record.** Rationale: the spec's own
RECOMMEND, and the conservative/self-contained choice (brief decision protocol: scope
question -> smaller, self-contained option). Repairing (A) is a net-new netns/veth/root
test-infrastructure project — git history confirms **no `netns` parser case ever existed**
(`git log -S 'case "netns"'` and `-S "netns:veth" -- internal/test/` both empty on
2026-07-17), so A is new construction, not restoration; B (re-mark to parse-and-skip) is a
false green (`ai/rules/no-workarounds-for-missing-behavior.md`); C alone turns silent death
into a loud CI failure (R-1). Deletion removes NO coverage: RFC 2516 discovery is proven by
`test/pppoe-interop/scenarios/01-pppoe-chap-ipv4/` and `ze-qemu-pppoe-accel-test`
(`mk/test-integration.mk:423` -> `scripts/evidence/effective-pppoe-accel.py`), both verified
present 2026-07-17. Thomas: override if you want the netns lab built instead.

→ [STAKES: scope] DELETION-NEEDS-APPROVAL GATE (2026-07-17): **Readiness records the
decision + plan; it does NOT delete any file.** The actual removal of `test/pppoe/` is an
implementation-time action GATED on Thomas's EXPLICIT approval (`ai/rules/never-destroy-work.md`,
R-2) — the implementer MUST ask before deleting and MUST NOT proceed on inference. This spec
is `ready` because the decision and sequencing are fixed, not because files are gone.

→ Constraint (guard/deletion coupling, 2026-07-17): `TestCIRootsRegistered` is RED while
`test/pppoe/` exists unrooted and only goes GREEN once the directory is deleted (Option D) —
there is no clean way to land the guard green without resolving `test/pppoe/`, and the only
non-workaround resolution is deletion (C alone would make the netns `.ci` fail loudly). So
although C' is conceptually independent of D (worth keeping under any option), at
implementation time the guard and the deletion land together in the same change, both under
the approval gate above. Implementer sequence: write the guard RED (Phase 2, TDD), obtain
deletion approval, delete (Phase 3), guard turns GREEN.

→ Constraint (`ai/rules/no-workarounds-for-missing-behavior.md`): if the tests are
repaired, they must actually exercise the PPPoE discovery path. Making them parse and
skip is a false green, which is what the current state already is in effect.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the `.ci` directive contract and how suites are registered
  → Constraint: a `.ci` that no suite roots is never discovered, so it cannot fail visibly.
- [ ] `docs/guide/pppoe.md` - the feature these tests were meant to cover
  → Decision: `docs/features.md:88` marks PPPoE Access `Partial` to reflect incomplete PPPoE *features*, NOT missing tests -- RFC 2516 discovery is covered by `test/pppoe-interop/` and `ze-qemu-pppoe-accel-test`.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2516.md` - PPPoE discovery (PADI/PADO/PADR/PADS/PADT), if the tests are repaired to assert wire behavior
  → Constraint: verify this summary exists before citing it; create via `/ze-rfc` if missing.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- Two independent failures (unparseable directive AND unregistered suite) mean neither alone explains the silence; fixing one leaves the tests still dead.
- The tests were never running, so there is no regression to fear and no baseline to preserve.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/test/runner/record_parse.go` - `parseOption` (`:295-448`) has 12 cases (`file`, `asn`, `bind`, `timeout`, `tcp_connections`, `open`, `update`, `env`, `skip-os`, `needs-linux`, `skip-env`, `require-tag`), none named `netns`; `default:` at `:444-445` returns `unknown option type %q`. `parseLine` (`:243`) dispatches `option` at `:273`
- [ ] `internal/test/cli/register.go` - `registerCIRoot` called for 20 suites at `:17-36`; no pppoe entry
- [ ] `test/pppoe/pppoe-basic.ci` - `option=skip-os:value=darwin` (`:9`), `option=env:var=TEST_IFACE:value=veth-sub` (`:148`), `option=netns:veth=veth-bng,veth-sub` (`:149`)
- [ ] `test/pppoe/pppoe-vlan.ci` - `option=netns:veth=veth-bng,veth-sub:vlan=100` (`:110`)
- [ ] `test/pppoe/pppoe-concurrent-l2tp.ci` - `option=netns:veth=veth-bng,veth-sub` (`:145`), plus `ze.l2tp.skip-kernel-probe` (`:143`)

**Behavior to preserve:** (unless user explicitly said to change)
- The 20 registered suites in `register.go:17-36` and their behavior — this spec adds at most one row, changes none.
- `parseOption`'s fail-closed `default:` branch. An unknown option type MUST stay an error; do not relax it to a warning to make these files parse (`ai/rules/fail-closed-guards.md`).
- The existing 12 option types and their semantics.

**Behavior to change:** (only if user explicitly requested)
- Depends entirely on which of options A-D the user picks. Under D, `test/pppoe/` is deleted and nothing in `internal/` changes.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A `.ci` file on disk under `test/<suite>/`, discovered only if a `registerCIRoot` call names its directory.
- The `option=<type>:<k>=<v>` directive line inside that file.

### Transformation Path
1. `registerCIRoot` (`internal/test/cli/register.go:17-36`) roots a suite name to a `test/` subdirectory — pppoe is absent, so `test/pppoe/` is never walked
2. `EncodingTests.Discover` (`record_parse.go:68`) walks a rooted directory for `.ci` files
3. `parseAndAdd` (`:111`) reads a file and calls `parseLine` per line
4. `parseLine` (`:243`) dispatches the `option` keyword at `:273` to `parseOption`
5. `parseOption` (`:295`) switches on option type; `netns` matches no case and hits `default:` at `:444`, returning `unknown option type "netns"` at `:445`
6. The parse error aborts the record; the test never becomes runnable

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Suite registry ↔ filesystem | `registerCIRoot` name → `test/<dir>/` walk | [ ] |
| `.ci` text ↔ runner Record | `parseLine` / `parseOption` keyword dispatch | [ ] |
| Runner ↔ kernel netns/veth | **does not exist today** — this is the missing primitive under option A | [ ] |

### Integration Points
- `registerCIRoot` (`internal/test/cli/register.go`) - the registration seam a pppoe suite would use; registration over hardcoding already holds here.
- `option=skip-os` / `option=needs-linux` - existing gating primitives the repaired tests would reuse.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — a pppoe suite registers via `registerCIRoot`; no per-suite switch case is added to the runner (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `option=netns` was never implemented, rather than removed later | `parseOption` has no `netns` case and no dead handler; the 3 `.ci` are its only users | The primitive exists elsewhere and the tests should be pointed at it | `git log -S "netns:veth" -- internal/test/` | unvalidated |
| A-2 | The 3 `.ci` describe PPPoE behavior that the current code still has | `docs/features.md:88` describes a live PPPoE subsystem (`internal/component/l2tp/pppoe/`) | The tests assert a design that changed; repairing them means rewriting them | Read the 3 `.ci` against `internal/component/l2tp/pppoe/` | unvalidated |
| A-3 | Nothing else waits on a `netns` runner primitive | `grep -rln "option=netns" test/` returns only `test/pppoe/` | Option A has more value than this spec credits | Re-grep at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Registering the suite (option C) without repairing the directive turns silent death into a loud parse failure across CI | `make ze-functional-test` newly red on 3 files | Land C only together with A or B; never C alone |
| R-2 | Option D deletes the only written record of intended PPPoE test topology | — | `ai/rules/never-destroy-work.md`: ask the user first; the `.ci` remain in git history either way |
| R-3 | Option A quietly becomes a large test-infrastructure project (netns + veth + VLAN + root) hiding behind a one-word directive | Design keeps growing past a parser case | Scope A explicitly before starting; `ai/rules/qemu-testing.md` may make QEMU the right host |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-test` runs the guard | → | `TestCIRootsRegistered` asserts every `test/` subdir with `.ci` files is rooted (or absent) | `TestCIRootsRegistered` (RED while `test/pppoe/` exists unrooted, GREEN once deleted) |
| A `.ci` declares an unknown option (`netns`) | → | `parseOption`'s fail-closed `default:` still errors | `TestParseOptionUnknownStillErrors` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-functional-test` | `test/pppoe/*.ci` are either discovered and RUN, or absent from the tree — never present-and-ignored |
| AC-2 | Each of the 3 `.ci` parses | No `unknown option type` error (or the file no longer exists, under option D) |
| AC-3 | `docs/features.md:88` PPPoE Access row | Left as-is: RFC 2516 discovery already has functional coverage (`test/pppoe-interop/`, `ze-qemu-pppoe-accel-test`); deleting the stale `.ci` removes no coverage, so the `Partial` marking (a feature-completeness statement, not a test claim) needs no change |
| AC-4 | An unknown option type in any `.ci` | Still an error. `parseOption`'s fail-closed default is not weakened |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the PPPoE accel-ppp labs and sees RFC 2516 discovery pass; the tree carries no orphaned `.ci` | `make ze-deployment-pppoe-accel-docker-test` / `ze-qemu-pppoe-accel-test` for coverage; `TestCIRootsRegistered` for the no-orphan guard | `test/pppoe-interop/scenarios/01-pppoe-chap-ipv4/check.py`, `TestCIRootsRegistered` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseOptionNetns` | `internal/test/runner/record_parse_test.go` | AC-2: the topology directive parses (option A only) | |
| `TestParseOptionUnknownStillErrors` | `internal/test/runner/record_parse_test.go` | AC-4: the fail-closed default survives | |
| `TestCIRootsRegistered` | `internal/test/cli/register_test.go` | AC-1: every `test/` subdirectory holding `.ci` files has a registered root — the general guard that would have caught this | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `vlan=` in `option=netns` (option A only) | 1-4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| accel-ppp Docker lab | `test/pppoe-interop/scenarios/01-pppoe-chap-ipv4/check.py` | Ze's PPPoE client completes RFC 2516 discovery + CHAP against a real accel-ppp AC | exists |
| QEMU accel-ppp lab | `scripts/evidence/effective-pppoe-accel.py` (`ze-qemu-pppoe-accel-test`) | Same, on a CONFIG_PPPOE kernel in QEMU | exists |
| `TestCIRootsRegistered` guard | `internal/test/cli/register_test.go` | The tree carries no orphaned `.ci` directory | |

### Interop Tests (MANDATORY for protocol features)

Already exist -- do NOT create new scenarios. PPPoE interop against a real accel-ppp
peer is covered by the lab below; this spec adds none.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| PPPoE CHAP IPv4 | `test/pppoe-interop/scenarios/01-pppoe-chap-ipv4/` | `accel-ppp` (Ze `pppoe-client`) | Real RFC 2516 discovery + PPP/CHAP session between Ze and accel-ppp | exists |

### Future (if deferring any tests)
- ~~(fill during design)~~
- Resolved (2026-07-17): None deferred. Under the plan of record (Option D + C'), no PPPoE
  `.ci` tests are kept or deferred — the 3 stale files are deleted (approval-gated) and RFC
  2516 discovery coverage remains with the existing accel-ppp labs. The only new test is
  `TestCIRootsRegistered`, which lands in-scope, not deferred. Should Thomas later choose to
  build the netns lab (Option A), that is a separate future spec, not a deferral of this one.

## Files to Modify
- `internal/test/cli/register_test.go` - add `TestCIRootsRegistered` (recommended; the recurrence guard)
  → Correction (2026-07-17): this file does NOT exist today (`ls` -> "No such file or directory"); it is a **new file to CREATE** (see Files to Create), not modify. Plain `package cli`, no build tags.
- `test/pppoe/pppoe-basic.ci`, `test/pppoe/pppoe-vlan.ci`, `test/pppoe/pppoe-concurrent-l2tp.ci` - DELETE (recommended Option D)
- `internal/test/cli/register.go` - add a `registerCIRoot("pppoe", ...)` row (ONLY under non-recommended options A/B/C)
- `internal/test/runner/record_parse.go` - add a `netns` case to `parseOption` (ONLY under non-recommended option A)
- `docs/features.md` - `:88` PPPoE Access row: no change expected (coverage already exists; `Partial` reflects feature completeness, not tests)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI commands/flags | [ ] | A new `ze-test pppoe` verb comes free from `registerCIRoot` |
| Functional test for new RPC/API | [ ] | `test/pppoe/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md:88` PPPoE Access row: no change expected — coverage already exists, `Partial` reflects feature completeness |
| 6 | Has a user guide page? | [ ] | `docs/guide/pppoe.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` — a new option type or suite must be documented |

## Files to Create
- ~~(fill during design — depends on the chosen option)~~
- Resolved (2026-07-17): **`internal/test/cli/register_test.go` — NEW FILE.** Correction to
  "Files to Modify": that section lists `register_test.go` as if it existed, but
  `ls internal/test/cli/register_test.go` returns "No such file or directory" on 2026-07-17
  — the guard's home file does not yet exist and must be CREATED. It is a plain
  `package cli` test file (no build-tag header; matches the sibling `*_test.go` in that
  directory, e.g. `build_test.go`, `ci_runner_test.go`), holding `TestCIRootsRegistered`.
  No other file is created: the 3 `.ci` are deleted, `register.go` is unchanged under Option
  D. (Options A/B/C, not recommended, would instead edit existing files — a parser case in
  `record_parse.go` / a suite row in `register.go` / repaired `.ci` — creating no new files.)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; confirm the 3 `.ci` still match today's PPPoE code (A-2) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Decide** — put options A-D to the user, recommending D + `TestCIRootsRegistered`; record the ruling here before any code
   - Tests: none
   - Files: this spec
   - Verify: the user has chosen; scope is agreed (`ai/rules/no-partial-completion.md` — no unilateral scope reduction)
2. **Phase: Wiring (MANDATORY FIRST)** — write `TestCIRootsRegistered`, the guard that makes an unrooted `.ci` directory impossible in future
   - Tests: `TestCIRootsRegistered`
   - Files: `internal/test/cli/register_test.go`
   - Verify: RED against today's tree (pppoe unrooted)
3. **Phase: Execute the chosen option** — recommended: delete `test/pppoe/` (D); otherwise A, B, C+A, or C+B
   - Tests: per the TDD table
   - Files: per Files to Modify
   - Verify: under D the 3 `.ci` are gone and `TestCIRootsRegistered` is GREEN; under A/B they run and assert real PPPoE behavior
4. **Functional tests** → PPPoE coverage stays green via the accel-ppp labs; no orphaned `.ci` remain
5. **Full verification** → `make ze-verify`
6. **Complete spec** → learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The tests exercise PPPoE discovery, not just parse-and-skip |
| Rule: no-workarounds | The `.ci` were not weakened to go green (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| Rule: fail-closed | `parseOption`'s `default:` still errors on unknown types |
| Registration over hardcoding | The pppoe suite registers via `registerCIRoot`; no per-suite branch in the runner (`ai/rules/plugin-self-containment.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| No orphaned `.ci` directory | `TestCIRootsRegistered` passes |
| No unparseable directive | `grep -rn "option=netns" test/` matches a `parseOption` case, or returns nothing |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Privilege | Option A needs netns/veth creation (root or CAP_NET_ADMIN) in the test runner; confirm it cannot escape the test host or leak namespaces on failure |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Two independent defects (unparseable directive, unregistered suite) produced one silence. Either alone would have been noticed: an unrooted suite fails no build, and an unknown option type errors only when something parses the file. Together they cancel out into "no signal at all", which is why this survived from May to July.
- The generalisable guard is `TestCIRootsRegistered`: assert that every `test/` subdirectory containing `.ci` files is rooted. That catches the next orphaned suite without anyone noticing it.

## Core Insight
~~(fill during design)~~

Resolved (2026-07-17): An orphaned test suite is invisible precisely because two independent
failures cancel: an unrooted `test/` subdir fails no build (nothing walks it), and an unknown
option type errors only when something parses the file — neither alone raises a signal, so
`test/pppoe/` went silent from May to July. The durable fix is not to repair these specific
files (their coverage already exists in the accel-ppp labs) but to install a structural guard
— `TestCIRootsRegistered` — that makes "a `.ci` directory no suite roots" a hard test
failure. That converts a silent whole class of future orphans into a loud one. Deleting the 3
stale files is the one-off cleanup; the guard is the reusable insight.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- The `Partial` marking on `docs/features.md:88` PPPoE Access reflects incomplete PPPoE *features*, not missing tests: RFC 2516 discovery is proven by `test/pppoe-interop/` and `ze-qemu-pppoe-accel-test`. The orphaned `.ci` never contributed to that coverage.

## RFC Documentation

Add `// RFC 2516 Section X.Y: "<quoted requirement>"` above enforcing code, if the repaired tests pin discovery behavior.
MUST document: validation rules, error conditions, state transitions.

## Implementation Summary

### What Was Implemented
- `internal/test/cli/register_test.go` (NEW): `TestCIRootsRegistered` guard — asserts
  every top-level `test/<dir>` holding `.ci` files is reachable by a ze-test runner
  (`registry.HasRootHandler(dir)` covers the 20 `registerCIRoot` suites + `vpp`; an
  explicit `bigRunnerCIDirs` allowlist covers the 8 big-runner subcommand dirs). Also
  fails on a stale allowlist entry. Verified RED (flags only `[pppoe]`) —
  `tmp/fixit-pppoe/guard-red.log`.
- `internal/test/runner/record_parse_test.go` (CHANGED): added
  `TestParseOptionUnknownStillErrors` — `option=netns:veth=...` through `parseAndAdd`
  must error with "unknown option type"/"netns" (AC-4 fail-closed default). Verified
  GREEN — `tmp/fixit-pppoe/ac4.log`.
- `test/pppoe/*.ci` deletion (Option D): PARKED, approval-gated (see Deviations).

### Bugs Found/Fixed
- None. The narrow defect (unparseable `option=netns` + unregistered suite) is resolved
  by deletion + guard, not by a code fix to `parseOption` (its fail-closed default is
  intentionally preserved and now pinned by a test).

### Documentation Updates
- `docs/features.md:88` PPPoE Access row: left AS-IS (AC-3 — `Partial` reflects feature
  completeness, not tests; RFC 2516 discovery coverage already exists in the accel-ppp
  labs). Optional one-line note to `docs/functional-tests.md` about the guard is staged
  in the drain recipe.

### Deviations from Plan
- **Deletion of `test/pppoe/*.ci` NOT executed.** These are git-tracked user-visible
  files; the spec's own DELETION-NEEDS-APPROVAL gate (`ai/rules/never-destroy-work.md`)
  requires Thomas's EXPLICIT approval, and a parent-agent task instruction is not the
  user's consent. Deletion is staged in `tmp/drain-fixit-pppoe-orphaned-tests.md` as the
  sole remaining step; the guard is intentionally left RED until it lands.
- Refined assumption A-1: a per-test netns LAUNCH mode DOES exist
  (`internal/test/runner/netns_linux.go`, `enterTestNetns`, `netnsModeActive()`), but it
  is a global env-driven runner mode, NOT the `option=netns:veth=` `.ci` directive — that
  directive and its veth topology were never built. Spec's conclusion (repair = net-new
  construction) stands.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | BLOCKED (approval) | `TestCIRootsRegistered` (RED now, GREEN on deletion) | Guard written + verified RED flagging only `[pppoe]`; goes GREEN only when `test/pppoe/` is deleted — deletion approval-gated |
| AC-2 | BLOCKED (approval) | deletion of the 3 `.ci` | Under Option D the files no longer exist; parked on approval |
| AC-3 | DONE | `docs/features.md:88` unchanged | `Partial` reflects feature completeness, not tests; coverage exists in accel-ppp labs |
| AC-4 | DONE | `TestParseOptionUnknownStillErrors` GREEN (`tmp/fixit-pppoe/ac4.log`) | `parseOption` fail-closed default preserved and now pinned by a test |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParseOptionNetns` | SKIPPED (Option A only) | — | Not implemented: Option D chosen, no `netns` parser case added |
| `TestParseOptionUnknownStillErrors` | DONE (GREEN) | `internal/test/runner/record_parse_test.go` | AC-4 |
| `TestCIRootsRegistered` | DONE (RED, by design) | `internal/test/cli/register_test.go` | AC-1; GREEN on deletion |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/cli/register_test.go` | CREATED | `TestCIRootsRegistered` guard |
| `internal/test/runner/record_parse_test.go` | MODIFIED | added `TestParseOptionUnknownStillErrors` |
| `test/pppoe/pppoe-basic.ci` | DELETE PENDING (approval) | staged in drain recipe |
| `test/pppoe/pppoe-vlan.ci` | DELETE PENDING (approval) | staged in drain recipe |
| `test/pppoe/pppoe-concurrent-l2tp.ci` | DELETE PENDING (approval) | staged in drain recipe |
| `internal/test/cli/register.go` | UNCHANGED | Option D adds no `registerCIRoot` row |
| `internal/test/runner/record_parse.go` | UNCHANGED | Option D adds no `netns` parser case (fail-closed default preserved) |
| `docs/features.md` | UNCHANGED | AC-3 |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| The tree carries no orphaned `.ci`; PPPoE RFC 2516 discovery coverage is preserved | functional test + guard | `TestCIRootsRegistered` green; `test/pppoe-interop/` and `ze-qemu-pppoe-accel-test` unchanged and passing |

## Review Gate

### Run 1 (initial — independent reviewer subagent over the diff)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | (no BLOCKER) | — | — | — |
| 2 | ISSUE | `bigRunnerCIDirs` re-hardcodes names that have canonical in-package sources (validCommands, predecessorTestDir); a new `ze-test bgp <sub>` suite would false-positive if the copy is not updated | `register_test.go` allowlist | FIXED — see Fixes applied |
| 3 | NOTE | Allowlist membership trusted, not verified against real dispatch (residual false-green surface) | `register_test.go` stale-check | Mitigated by fix (allowlist now IS the dispatch source `bgpCIRunnerDirs`); doc-comment records the trust boundary |
| 4 | NOTE | Guard keys on `dir == root-name` for registerCIRoot suites (true for all 20 today) | `register_test.go` | Documented in guard comment; accepted |
| 5 | NOTE | Confirm `internal/test/cli` is in the unit-test gate (else dead test) | `mk/test-unit.mk` | VERIFIED: `ZE_PACKAGES = go list ./... (minus root)` includes it → runs in `make ze-unit-test`/`ze-test-rest` |

### Fixes applied
- ISSUE (#2): promoted the 7 bgp-subcommand dirs to a package-level single source of
  truth `var bgpCIRunnerDirs` in `cmd_bgp.go`; the dispatch arg-check
  (`if !bgpCIRunnerDirs[command]`) and the guard's `coveredByBigRunner` both consume it,
  and `exabgp-compat` is referenced via the existing `predecessorTestDir` const. The
  duplicated literal in the test is gone. Dispatch behavior is identical (same 7
  subcommands). Verified: `go vet ./internal/test/cli/` clean; `TestCIRootsRegistered`
  still RED flagging only `[pppoe]` (`tmp/fixit-pppoe/guard2.log`).

### Run 2 (confirmation review of the ISSUE fix — independent subagent)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | (no BLOCKER, no ISSUE) | Dispatch behavior identical (same 7 subcommands via `bgpCIRunnerDirs`); no coverage regression — exactly one orphan `pppoe`; stale-check honest, no false-green | `cmd_bgp.go`, `register_test.go` | — |
| 2 | NOTE | Stale identifier in doc-comment: referenced removed `validCommands` | `cmd_bgp.go:34` | FIXED — comment now points to the `if !bgpCIRunnerDirs[command]` gate |

### Final status
- [x] Independent review re-run shows 0 BLOCKER, 0 ISSUE (Run 1: 1 ISSUE fixed; Run 2 confirmation: clean). Artifact: `tmp/review/fixit-pppoe-orphaned-tests-58c51aab-79d8-400d-b779-2c0cf322a274.md`
- [x] All NOTEs recorded: #3 (trust boundary — mitigated, allowlist is the dispatch source), #4 (dir==root-name — documented), #5 (unit-test gate — verified in ZE_PACKAGES), Run-2 #2 (stale comment — fixed)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-fixit-pppoe-orphaned-tests.md` only

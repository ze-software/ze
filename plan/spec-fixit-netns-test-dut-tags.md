# Spec: fixit-netns-test-dut-tags

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | — |
| Phase | 0/2 (research) |
| Updated | 2026-07-28 |

## Task

`make ze-netns-test` is the only vehicle that runs the `firewall`, `policy`,
`ospf` and `ospfv3` suites natively on a Linux dev host (`ZE_NETNS_SUITES`,
`mk/test-integration.mk:136`). It has two independent defects that make it
report failures no test is responsible for.

**Defect 1 — it runs a production DUT.** The target's prerequisites name
`$(ZEBIN_ZE)` (`mk/test-integration.mk:138`), and the recipe hands the suites
`ZE_TEST_NO_BUILD=1` (`mk/test-integration.mk:148`), so the daemon under test is
the production binary. But
`mk/test-integration.mk:532` states plainly that "the real `$(ZEBIN_ZE)` has
neither zetest nor ze_test", while the functional-test DUT is built with
`-tags 'ze_core ze_distro ze_setup zetest $(ZE_FEATURES) $(ZE_TAGS)'`
(`mk/test-functional.mk:140`). Any test whose config touches a `zetest`-only
YANG augment therefore cannot start its daemon.

Observed: `test/firewall/ddos-local-withdraw.ci` configures `ddos { fake { ... } }`,
a node contributed only by `internal/test/plugins/fakeddos/yang/ze-fakeddos-conf.yang`
("Compiled only under the zetest build tag, so production never sees it"). Under
`ze-netns-test` the daemon exits with
`parse config: line 5: unknown field in ddos: fake`, the driver's readiness poll
burns its full 10s budget, and the runner reports a 15.1s TEST failure.

**Defect 2 — it cannot run on-session at all.** `mk/session.mk:117` makes the
built binary `bin/ze-<session-id>`, but the runner resolves the DUT by BARE name
through `sessionpath.FindPrebuiltDir` (`internal/test/sessionpath/sessionpath.go:107-132`),
which probes only `tmp/s/<id>/bin` and `bin` for a file literally called `ze`.
`.ci` tests also exec `ze` by bare name. Under any Claude session the target
fails immediately with
`ZE_TEST_NO_BUILD set but .../bin/ze is missing (cross-compile it first)`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint: annotations. -->
- [ ] `ai/rules/qemu-testing.md` — which vehicle is authoritative for linux-only tests, and why a skip is not evidence.
- [ ] `docs/architecture/testing/ci-format.md` — `option=needs-linux:caps=` gating, which decides whether a suite is reachable at all.
- [ ] `ai/rules/functional-test-gate.md` — every behavior keeps its required functional test; this spec must not "fix" the vehicle by dropping a test from it.

### RFC Summaries (MUST for protocol work)
- [ ] N/A — build/test vehicle; no wire-protocol behavior changes.

## Data Flow (MANDATORY)

### Entry Point
- A developer runs `make ze-netns-test` on a Linux host.

### Transformation Path
1. make builds/refreshes the DUT binaries named by the target's prerequisites.
2. `sudo setcap` grants the DUT ambient `cap_net_admin,cap_net_raw,cap_net_bind_service`.
3. For each suite, `sudo env ... $(ZEBIN_TEST) <suite> --all -p 1` runs with `ZE_TEST_NETNS=1`, so the runner puts each test in its own netns and launches `ze` by bare name from one PATH directory.
4. The daemon parses the test's config. A node contributed only by a `zetest`-tagged YANG augment resolves ONLY if step 1 produced a zetest DUT — today it does not.
5. After the suites, host nft tables are compared against the pre-run snapshot and caps are dropped.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| make -> runner | binary paths as prerequisites + `ZE_TEST_NO_BUILD=1` | [ ] |
| make -> runner (session identity) | `sudo env` allowlist; `ZE_SESSION_ID` is NOT forwarded | [ ] |
| runner -> daemon | bare-name exec from one PATH dir (`sessionpath.FindPrebuiltDir`) | [ ] |
| daemon -> YANG schema | build tags decide which augments exist | [ ] |

### Integration Points
- `mk/test-integration.mk` (the target), `mk/test-functional.mk` (the reference DUT tag set), `internal/test/sessionpath` (bare-name resolution). No production code.

### Architectural Verification
- [ ] No bypassed layers (the fix changes which binary is built, not how tests find it).
- [ ] No unintended coupling (no `.ci` test learns about build tags).
- [ ] No duplicated functionality (reuse `ZE_ALT_BUILD`'s tag list rather than restating it).
- [ ] Registration over hardcoding — no per-test allowlist of "needs zetest".

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `mk/test-integration.mk` — `ze-netns-test` (:138-163): prerequisites
  `$(ZEBIN_ZE) $(ZEBIN_STRIPPED) $(ZEBIN_TEST)`, `sudo setcap` on the first two,
  then `sudo env ... ZE_TEST_NO_BUILD=1 ZE_TEST_NETNS=1 $(ZEBIN_TEST) $$suite`.
  -> Constraint: the `sudo env` allowlist does not forward `ZE_SESSION_ID`, so
  even a suffix-aware runner would not see the session under sudo.
- [ ] `mk/test-functional.mk:140` — `ZE_ALT_BUILD`, the DUT tag set that
  `ze-functional-test` uses. This is the reference spelling to converge on.
- [ ] `internal/test/sessionpath/sessionpath.go:107-132` — `FindPrebuiltDir`
  resolves a DIRECTORY holding every bare name, deliberately (`.ci` tests exec
  `ze` and `ze-stripped` by bare name and the runner puts one directory on their
  PATH). A suffix cannot simply be taught to this function without breaking that.
- [ ] `internal/test/plugins/fakeddos/yang/ze-fakeddos-conf.yang` — augments
  `/dd:ddos` with the `fake` presence container, zetest-only.

**Behavior to preserve:**
- The host-safety check (nft table set compared before/after) and the
  `setcap -r` teardown.
- One netns per test, `-p 1` (the `test/policy` suite shares global kernel
  objects; `test/policy/001-boot-apply.ci:9-21` records why raising parallelism
  broke all six on 2026-07-25).

**Behavior to change:**
- The netns run must use a DUT built with the functional-test tag set.
- The target must work on-session, or refuse with a message that says how to
  run it (rather than an opaque missing-binary error).

## Acceptance Criteria

| AC ID | Piece | Expected Behavior |
|-------|-------|-------------------|
| AC-1 | DUT tags | `make ze-netns-test` runs the suites against a daemon built with the same tag set as `ZE_ALT_BUILD` (`mk/test-functional.mk:140`), so `zetest`-only YANG augments resolve |
| AC-2 | ddos-local-withdraw | passes under `make ze-netns-test` with no change to the test; it already passes in 653ms against a correctly-tagged DUT |
| AC-3 | On-session | `make ze-netns-test` works with `CLAUDE_CODE_SESSION_ID` set, OR fails fast with a message naming the off-session invocation; never the current opaque `bin/ze is missing` |
| AC-4 | No new production surface | the fix is confined to the make graph and, if needed, the netns launch path; no `.ci` test and no production binary changes |
| AC-5 | Regression | `firewall`, `policy`, `ospf`, `ospfv3` all green under the fixed target, host nft tables unchanged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Status |
|----|-----------|-------|----------|--------|
| A-1 | A zetest DUT is safe for every netns suite, not just the ddos test | `ze-functional-test` already runs all of them against exactly this tag set | the target needs a per-suite DUT choice | pending |
| A-2 | No netns test depends on production-only behavior that `zetest` changes | zetest only ADDS test plugins and command surface | a test that asserts absence of a test plugin would flip | pending |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | Building a second DUT doubles the target's build time | wall-clock on a cold cache | reuse the `ZE_ALT_BIN` artifacts `ze-functional-test` already builds rather than adding a third binary |
| R-2 | A suffix fix that teaches bare-name lookup about sessions breaks `.ci` exec-by-name | functional suites go red everywhere | prefer a PATH-directory fix (what `FindPrebuiltDir` already models) over renaming |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-netns-test` | -> | netns recipe picks the zetest DUT | `test/firewall/ddos-local-withdraw.ci` green under the target |
| `make ze-netns-test` on-session | -> | session-id handling in the recipe | run with `CLAUDE_CODE_SESSION_ID` set |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| DUT tag set matches the functional one | `internal/test/runner/*_test.go` (existing `TestBuildTags` is the precedent, `mk/test-integration.mk:534`) | the netns DUT carries `zetest` | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `test/firewall/ddos-local-withdraw.ci` | `test/firewall/` | the zetest-gated case: passes under `make ze-netns-test` with NO change to the .ci file (it already passes in 653ms against a correctly-tagged DUT) | |
| `test/firewall/001-boot-apply.ci` | `test/firewall/` | a production-path control: must stay green under the retagged DUT, proving A-2 (zetest only adds surface) | |
| `test/policy/001-boot-apply.ci` | `test/policy/` | second suite, second netns, same DUT: the target's other suites are unaffected by the retag | |
| `test/ospf/*.ci` (full suite) | `test/ospf/` | 97 tests, the largest netns suite: no regression from the DUT change | |

## Files to Modify
- `mk/test-integration.mk` — `ze-netns-test` prerequisites and the `sudo env` line.
- Possibly `mk/test-functional.mk` — to share the DUT build rather than duplicate the tag list.

## Implementation Steps

### Implementation Phases
1. Point the netns run at a correctly-tagged DUT; prove AC-2 and AC-5.
2. Decide the on-session behaviour (support or fail-fast) and implement AC-3.

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-2 false (a suite needs the production DUT) | per-suite DUT selection, recorded in the target |
| The session fix would need `FindPrebuiltDir` to learn suffixes | stop; that breaks exec-by-bare-name (R-2) — take the fail-fast half of AC-3 instead |

## Key Design Decisions
| Decision | Alternatives | Rationale |
|----------|-------------|-----------|
| Reuse the functional-test DUT tag set verbatim | hand-pick tags for netns | one spelling of "the DUT", so it cannot drift; `mk/test-integration.mk:358` records that this exact list already drifted three times (learned 1258, 1269) |

## Core Insight

The netns target's failures are not test failures, and reading them as such
costs real time: `ddos-local-withdraw` presents as a 15s red test with a plain
`exit code 1`, and the actual cause is one line of daemon stderr about a config
field that a differently-tagged binary would have accepted. A vehicle that can
report a failure no test can cause is worse than a vehicle that refuses to run.

## Known Limitations
- Does not address QEMU targets; `ZE_QEMU_DUT_TAGS` (`mk/test-integration.mk:362`)
  already includes `zetest` and is unaffected.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] make ze-test

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

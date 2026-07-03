# Spec: redistribute source registration + list-key validation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/redistribute/` - source/consumer registries, evaluator
4. `internal/component/config/yang/validator.go` - the YANG validation walker (Bug B lives here)
5. `internal/plugins/connected/connected.go` - the reference source-registration pattern
6. `internal/plugins/static/` - the plugin missing its source registration (Bug A)

## Task

Fix two defects that together make `redistribute { destination bgp { import static } }`
a config that passes validation but silently does nothing (or is rejected) at runtime,
and harden the redistribute surface so this class of bug cannot recur.

- **Bug A — static is not a registered redistribute source.** The static plugin emits
  route-change events (a redistribute *producer*) but never registers the config *source*
  name `static`. Any config importing `static` is rejected at runtime with
  `unknown source "static"`.
- **Bug B — `ze:validate` on a list key never runs.** The YANG validation walker recurses
  into a list entry's children but never validates the list *key* itself. So the
  `redistribute-source` validator on the `import` key leaf is dead code: even
  `import totalgarbage123` passes `ze config validate`, then fails/no-ops at runtime.
  **Coupled sub-bug (B2): source registration timing.** `ze config validate` does NOT run
  plugins; it only imports them (via the `all.go` composition root, so `init()` runs). BGP
  registers its sources in `init()` (`internal/component/bgp/redistribute/bgp.go:14-15`), but
  connected/kernel/ospf/isis/l2tp/ipsec register theirs at plugin-*run* time (e.g.
  `internal/plugins/connected/connected.go:158` inside `runConnectedPlugin`). So at
  validate time only the BGP sources exist. Fixing B1 alone (validate the key) would then
  FALSELY reject every non-BGP `import` at validate time. B1 MUST be shipped with B2: move
  all source registrations to `init()` so the validate-time registry is complete.
- **Hardening — producer↔source parity.** Add an invariant so a protocol that registers a
  redistribute producer without a matching config source (exactly the static bug) is caught
  by a test, not by an operator in production.
- **Docs — quickstart.** Rewrite the quickstart's first example to lead with the
  declarative, non-"magical" path (`import static` + a direct `update {}` announce) instead
  of the `process rib` plugin-binding incantation. Demote `process` to an advanced
  "bring-your-own-handler" note.

Out of scope decision needed from user: whether to also do the larger "RIB-backed peer by
default, `process` only for external handlers" redesign (tracked separately, not here).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` - redistribute orchestrator + producer/consumer model
  → Constraint: producers emit `redistevents.RouteChangeBatch`; the orchestrator dispatches
    to registered `RedistConsumer`s; config source names gate via the evaluator.
- [ ] `docs/guide/configuration.md` (redistribute section, ~L580-608) - the config surface
  → Constraint: `redistribute { destination <proto> { import <source> [ { family ... } ] } }`
    is the current syntax; the bare top-level `import` form is legacy (pre-701).
- [ ] `ai/rules/config-surface.md`, `ai/rules/plugin-self-containment.md`
  → Constraint: a plugin owns its own registrations; the source name belongs in the static
    plugin, not a central table.

**Key insights:**
- The `redistribute-source` validator already exists and is correct
  (`internal/component/config/validators.go:483-497`); it is simply never invoked on the
  `import` list key. Bug B is a *walker* fix, not a validator fix.
- Static already emits events (`internal/plugins/static/inject.go:346-373`), so Bug A needs
  only the source registration, no new producer plumbing.

## Current Behavior (MANDATORY)

**Source files read (BEFORE writing this spec):**
- [ ] `internal/component/config/redistribute/registry.go` - `RegisterSource` / `LookupSource`
  (source-name registry; keys the evaluator + the `redistribute-source` validator).
  → Constraint: sources carry `{Name, Protocol, Description}`; `LookupSource(name)` is the
    gate ExtractRedistributeRules and the validator both consult.
- [ ] `internal/component/config/loader_redistribute.go:21-77` - `ExtractRedistributeRules`
  calls `LookupSource(source)` and returns `unknown source %q` for misses (`:40`, `:50`).
  → Constraint: this is the RUNTIME rejection path; it currently fires for `static`.
- [ ] `internal/component/config/validators.go:483-497` - `RedistributeSourceValidator`
  (`LookupSource` → `%q is not a registered redistribute source`). Registered as
  `redistribute-source` in `validators_register.go:22`.
  → Constraint: correct logic, never reached for list keys.
- [ ] `internal/component/config/yang/validator.go:615-707` - `walkTree`. At `:642-651` it
  iterates list entries and recurses into each entry's child map, but never validates the
  `listKey` value against the list's key-leaf schema.
  → Constraint: THIS is Bug B. The fix adds key-leaf validation for list entries.
- [ ] `internal/plugins/static/register.go` - static plugin registration. No `RegisterSource`.
  → Constraint: THIS is Bug A. Contrast `internal/plugins/connected/connected.go:25-34`.
- [ ] `internal/plugins/static/events/events.go:12-19` + `inject.go:346-373` - static IS a
  producer (registers `redistevents` producer + emits `RouteChange` on add/remove).
  → Constraint: producer half works; only the config source name is missing.
- [ ] `internal/plugins/connected/connected.go:25-34` + `:158` - `registerConnectedSources()`
  (`sync.Once` → `redistribute.RegisterSource{...}`), but CALLED from `runConnectedPlugin`
  (run time), so absent during `config validate`. Same pattern in kernel (`register.go:69`),
  ospf/isis/l2tp/ike (run/OnStarted paths).
  → Constraint: this run-time timing is the B2 defect. The CORRECT reference is
    `internal/component/bgp/redistribute/bgp.go:14-15` which registers in `init()`.
- [ ] `internal/component/config/cli/cmd_validate.go` - `ze config validate` path; runs YANG
  validation + `InProcessConfigVerifier`s, but does NOT start plugin engines (`RunEngine`).
  → Constraint: only `init()`-registered sources are visible during validate.
- [ ] `internal/component/plugin/all/all.go:207,236,243` - blank-imports connected/kernel/static
  so their `init()` runs in the `ze` binary (but their run-time source registration does not).
- [ ] `docs/guide/quickstart.md:55-105` - current first example uses `plugin { internal rib }`
  + `process rib { receive [state] send [update] }` + `update {}`.

**Behavior to preserve:**
- `import connected|kernel|l2tp|ipsec|ospf|isis|ibgp|ebgp` continue to validate and run.
- Every other list-key custom validator that DID work keeps working (Bug B fix must not
  regress existing keyed-list validation; verify no keyed list currently relies on the key
  being unvalidated).
- The direct per-peer `update {}` static announce path is unchanged
  (`internal/component/bgp/reactor/peer_initial_sync.go:65`).

**Behavior to change (user-requested):**
- `import static` becomes a first-class, functional source.
- `ze config validate` rejects unregistered/misspelled redistribute sources at validate time
  (currently only rejected/dropped at runtime).
- Quickstart first example no longer uses `process rib`.

## Data Flow (MANDATORY)

### Entry Point
- Config file `redistribute { destination bgp { import static } }` → config tree.

### Transformation Path
1. `ze config validate` → YANG walker `validateTree`/`walkTree`
   (`internal/component/config/yang/validator.go`). **Bug B:** list key `static` not validated.
2. Runtime load → `ExtractRedistributeRules` (`loader_redistribute.go:21`) → `LookupSource`.
   **Bug A:** `static` absent → `unknown source "static"` (WARN, rule dropped → silent no-op).
3. When registered: static route apply → `emitRouteChange` (`inject.go:346-373`) →
   EventBus → `redistribute-orchestrator` (`bgp/plugins/redistribute_egress/redistribute.go:156`)
   → BGP consumer `InjectRoute` (`internal/component/bgp/redistribute/consumer.go:38`) →
   reactor `UpdateRoute("*")` → UPDATE to peers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Validator | YANG walker applies `ze:validate` to leaves + (new) list keys | [ ] |
| Static plugin ↔ redistribute registry | `RegisterSource` at init | [ ] |
| Static plugin ↔ EventBus | `RouteChange.Emit` (already present) | [ ] |
| Orchestrator ↔ BGP reactor | consumer `InjectRoute` → `UpdateRoute("*")` | [ ] |

### Integration Points
- `redistribute.RegisterSource` (`internal/component/config/redistribute/registry.go:36`) - static registers here (Bug A fix).
- `RedistributeSourceValidator` / `redistribute-source` (`internal/component/config/validators.go:483`, `validators_register.go:22`) - reached once the walker validates list keys (Bug B fix).
- `walkTree` list-entry loop (`internal/component/config/yang/validator.go:642-651`) - the extension point for list-key validation.
- `redistevents.Producers()` (`internal/core/redistevents/registry.go`) - enumerated by the parity test.

### Architectural Verification
- [ ] Registration over hardcoding — `static` source registered by the static plugin itself
      (plugin self-containment), not added to a central list.
- [ ] No duplicated functionality — reuse existing `RegisterSource`; no new registry.
- [ ] Bug B fix is in the shared walker and benefits ALL keyed lists with validated keys.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Registering source `static` is sufficient for end-to-end (producer already emits) | `inject.go:346-373`, `events/events.go:12-19` | Need producer work too | interop scenario shows static prefix on peer | unvalidated |
| A-2 | No existing keyed list depends on its key NOT being validated by `ze:validate` | grep all `ze:validate` on list `key` leaves | Bug B fix breaks a config | enumerate list-key `ze:validate` uses + run full config test suite | unvalidated |
| A-3 | `unknown source` at runtime is non-fatal (WARN, rule dropped), so `import static` is a silent no-op today | observed WARN in local run; `loader_redistribute.go` returns error to caller | Behavior differs | read the ExtractRedistributeRules caller's error handling | unvalidated |
| A-4 | The `redistribute-source` validator name maps to a `key` leaf, and the walker has access to the list's key-leaf entry (`entry.Key` / `entry.Dir[key]`) | `validator.go:642-651`, goyang `Entry.Key` | Fix needs different plumbing | implement + unit test | unvalidated |
| A-5 | Moving source registration to `init()` makes sources visible during `config validate` (the `ze` binary imports all plugins via `all.go`, so their `init()` runs; validate does not run engines) | `all.go:207,236,243`; `cmd_validate.go` has no `RunEngine` | B2 fix does not close the validate gap; need validate to run a registration phase | test: standalone `LookupSource("connected")` true after import; AC-9 configs validate | unvalidated |
| A-6 | Registering sources at `init()` has no init-order hazard (redistribute registry map exists before any plugin init runs) | `registry.go:25` package-var map init | init panic / lost registration | build + run all unit tests; parity test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Bug B fix surfaces previously-hidden invalid keys in EXISTING shipped configs (e.g. stale `test/l2tp-interop/.../ze.conf` bare `import`, `test/interop/scenarios/isis-redist-frr/ze.conf` `import static`) | full `.conf` validation sweep fails | fix the stale configs in the same spec; they are already broken at runtime |
| R-2 | Other producers also missing source registration (same class as static) | parity test fails for a second protocol | parity test lists all offenders; fix each |
| R-3 | Validating list keys changes error output/positions relied on by tests | config validation golden tests differ | update goldens; confirm messages are strictly better |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `redistribute { destination bgp { import static } }` in config | → | `LookupSource("static")` returns registered | `TestStaticRegistersRedistributeSource` (unit) |
| `ze config validate` with `import <unregistered>` | → | `walkTree` validates list key → `redistribute-source` rejects | `TestValidateRejectsUnknownRedistributeSource` (unit) |
| static route applied while `import static` configured | → | `emitRouteChange` → orchestrator → BGP consumer → peer UPDATE | `NN-static-redist-bgp-<peer>` interop (or functional if interop infeasible) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Static plugin loaded | `redistribute.LookupSource("static")` returns a `RouteSource{Name:"static", Protocol:"static"}` |
| AC-2 | `redistribute { destination bgp { import static } }` at runtime | Orchestrator accepts the source; no `unknown source "static"` warning; static routes are advertised to BGP peers |
| AC-3 | `ze config validate` with `import totalgarbage123` (scalar OR list form) | Validation FAILS with `"totalgarbage123" is not a registered redistribute source` |
| AC-4 | `ze config validate` with `import connected` / `import static` (registered) | Validation PASSES |
| AC-5 | Any registered `redistevents` producer protocol | Has ≥1 registered config source with matching `Protocol`; the parity test asserts this for all producers |
| AC-6 | `ze config validate` on every shipped `*.conf` under `test/` | Passes (stale configs using invalid sources / legacy `import` syntax fixed) |
| AC-7 | Quickstart first example (`docs/guide/quickstart.md`) | Uses `static { table default { route ... } }` + `redistribute { destination bgp { import static } }` + a direct `update {}` announce; no `process rib`; validates AND runs |
| AC-8 | Bug B fix applied | No previously-valid keyed-list config regresses (full config test suite green) |
| AC-9 | `ze config validate` (standalone, no daemon) with `import connected\|kernel\|ospf\|isis\|l2tp\|ipsec\|static\|ibgp\|ebgp` | PASSES for every registered source — i.e. all sources are `init()`-registered and visible at validate time (B2) |

## End-to-End User Stories (MANDATORY)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `import static`, starts ze | config → LookupSource(ok) → orchestrator subscribes → static emits → consumer → peer UPDATE | interop/functional `static-redist-bgp` |
| 2 | Typos `import staic`, runs `ze config validate` | walker validates list key → rejected before start | `TestValidateRejectsUnknownRedistributeSource` |
| 3 | Follows the quickstart verbatim | validate passes → daemon announces `172.16.0.0/24` (redistributed) + `192.168.1.0/24` (direct) | quickstart functional check / interop |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStaticRegistersRedistributeSource` | `internal/plugins/static/register_test.go` | AC-1 | |
| `TestValidateRejectsUnknownRedistributeSource` | `internal/component/config/yang/validator_test.go` | AC-3, Bug B | |
| `TestValidateAcceptsRegisteredRedistributeSource` | `internal/component/config/yang/validator_test.go` | AC-4 | |
| `TestListKeyCustomValidatorRuns` | `internal/component/config/yang/validator_test.go` | Bug B general (list-key validators fire) | |
| `TestEveryProducerHasRegisteredSource` | `internal/component/config/redistribute/parity_test.go` | AC-5 | |

### Boundary Tests
N/A (no new numeric inputs).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `static-redist-bgp` | `test/.../*.ci` or interop | static route redistributed into BGP reaches a peer | |
| config-validate reject | `test/.../*.ci` | `ze config validate` fails on unknown source | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `static-redist-frr` (or extend `isis-redist-frr`) | `test/interop/scenarios/` | FRR/GoBGP | `import static` into BGP actually advertises; FRR installs the prefix | |

<!-- Note: `import static` is protocol-adjacent (drives BGP UPDATE emission). An interop or
     functional test is REQUIRED per ai/rules/interop-and-goal-validation.md; unit tests alone
     are insufficient because Bug A is precisely a "unit-tested-but-not-wired" failure. -->

## Files to Modify
- `internal/plugins/static/register.go` - add `registerStaticSources()` and call it from `init()` (NOT run — mirror `bgp.go:14-15`, not `connected.go:158`). (Bug A)
- `internal/component/config/yang/validator.go` - `walkTree`: validate list-entry keys against the key-leaf schema (type + `ze:validate`). (Bug B1)
- `internal/plugins/connected/connected.go` - call `registerConnectedSources()` from `init()` instead of (or in addition to) `runConnectedPlugin`. (Bug B2)
- `internal/plugins/kernel/register.go` - call `registerKernelSources()` from `init()`. (Bug B2)
- `internal/plugins/ospf/redistribute/source.go`, `internal/plugins/isis/redistribute/source.go` - register source at `init()`. (Bug B2)
- `internal/component/l2tp/redistribute.go`, `internal/component/ike/engine/redistribute.go` - register source at `init()`. (Bug B2)
  <!-- All source registration is pure metadata (sync.Once, no I/O); safe at init. Verify no
       init-order dependency on the redistribute registry existing first. -->

<!-- B1 and B2 MUST land together: B1 without B2 regresses AC-9 (validate falsely rejects
     run-time-registered sources); B2 without B1 leaves the validator dead. -->

- `test/l2tp-interop/scenarios/02-ppp-bgp-redistribute-frr/ze.conf` - migrate legacy `redistribute { import l2tp; }` to `redistribute { destination bgp { import l2tp } }`. (AC-6, R-1)
- `test/interop/scenarios/isis-redist-frr/ze.conf` - already uses `import static`; confirm it now works (was a runtime no-op). (R-1)
- `docs/guide/quickstart.md` - new first example. (AC-7)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | source name is a runtime registration, not schema |
| YANG custom validators | Reuse | `redistribute-source` already exists; Bug B makes it reachable for list keys |
| Functional test for behavior | Yes | interop/functional `static-redist-bgp` |
| Doctor check | No | no new runtime dependency |
| Prometheus counters | No (reuse) | orchestrator already counts announcements/withdrawals |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | Yes | `docs/guide/quickstart.md` (new example), verify `docs/guide/configuration.md` redistribute section lists `static` as a source |
| 5 | Plugin changed? | Yes | `docs/guide/plugins.md` / `docs/plugin-overview.md` if it enumerates redistribute sources |
| 15 | Registered source inventory changed? | Yes | any doc listing redistribute sources must include `static` |
| 17 | Existing docs show examples for this area? | Yes | verify quickstart/config examples validate AND run |

## Files to Create
- `internal/component/config/redistribute/parity_test.go` - producer↔source parity invariant. (AC-5)
- `test/interop/scenarios/static-redist-frr/` (or extend existing) - end-to-end proof. (Wiring)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, verify current behavior citations still hold |
| 3. Wiring | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review | Review Gate |
| 6. Verify | `make ze-verify` (lint + unit + functional) |
| 7-10 | Critical review loop |
| 11-14 | Deliverables, security, re-verify, summary |

### Implementation Phases

1. **Phase: Wiring (FIRST)** — write failing unit tests
   - Tests: `TestStaticRegistersRedistributeSource`, `TestValidateRejectsUnknownRedistributeSource`, `TestEveryProducerHasRegisteredSource`
   - Verify: all three FAIL against current code (proves the bugs).
2. **Phase: Bug A — static source registration**
   - Files: `internal/plugins/static/register.go`
   - Add `registerStaticSources()` (`sync.Once` → `redistribute.RegisterSource{Name:"static", Protocol:"static", Description:"static routes"}`), call from `init()`.
   - Verify: `TestStaticRegistersRedistributeSource` passes; AC-2 orchestrator no longer warns.
3. **Phase: Bug B2 — sources register at `init()` (do FIRST, before B1)**
   - Files: connected, kernel, ospf, isis, l2tp, ike (see Files to Modify).
   - Move each `register*Sources()` call into the plugin's `init()` (keep `sync.Once` so the run-time call, if kept, is a no-op).
   - Verify: a test importing `all.go` sees `LookupSource("connected"/"kernel"/...)` true WITHOUT running engines (AC-9 precondition). Ship this BEFORE B1 so B1 doesn't regress validate.
4. **Phase: Bug B1 — validate list keys**
   - Files: `internal/component/config/yang/validator.go`
   - In `walkTree`, for each list entry, resolve the key leaf (`entry.Dir[entry.Key]` on the list `child`) and run `validateEntry` + `applyCustomValidators` on the `listKey` value.
   - Guard: only when the list has a single key leaf that carries a type/`ze:validate`; skip composite keys.
   - Verify: `TestValidateRejectsUnknownRedistributeSource`, `TestListKeyCustomValidatorRuns` pass; AC-8 (no regression) via full config suite; AC-9 (all real sources still validate) green.
5. **Phase: Parity invariant + fix offenders**
   - Files: `internal/component/config/redistribute/parity_test.go`
   - Enumerate `redistevents.Producers()`; assert each has ≥1 `RegisterSource` with matching `Protocol`. Fix any additional offenders R-2 surfaces.
6. **Phase: Config sweep (AC-6)**
   - Migrate legacy/broken `.conf` files under `test/` so `ze config validate` passes tree-wide.
7. **Phase: Docs — quickstart (AC-7)** — see "Proposed quickstart" below.
8. **Phase: Interop/functional** — prove static→BGP advertisement end to end.
9. **Full verify** → `make ze-verify`.
10. **Complete spec** → audit tables + `plan/learned/NNN-redist-source-registration.md`, two-commit close.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-8 each have code + test with file:line |
| Correctness | Bug B validates ONLY the key leaf, not the whole entry map twice; no double error |
| Plugin self-containment | `static` source registered inside the static plugin; nothing central spells "static" |
| No regression | full config validation suite green (A-2, AC-8) |
| Registration over hardcoding | parity test enumerates the registry, no hardcoded protocol list |
| Docs | quickstart example both validates and runs (not another validate-only artifact) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| static source registered | `grep -n RegisterSource internal/plugins/static/register.go` |
| unknown source rejected at validate | `bin/ze config validate` on a garbage-source conf → non-zero + message |
| parity test | `go test ./internal/component/config/redistribute/ -run Parity` |
| quickstart works | `bin/ze config validate` on the doc's config → valid; interop/functional shows advertisement |
| tree-wide configs valid | script/loop `ze config validate` over `test/**/*.conf` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Bug B strengthens validation (fail-closed on unknown source); ensure it does not panic on non-string keys or lists without a key leaf |
| Resource | no unbounded growth; registration is one-time |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Bug B breaks an existing config | Inspect whether that config was ever valid at runtime; if legacy → fix config; if regression → narrow the walker fix |
| Interop can't run on host | Fall back to a functional `.ci` driving `ze-test peer --mode sink --decode`; note env limitation |
| 3 fix attempts fail | STOP, report, ask user |

## Proposed quickstart first example (for AC-7)

```
static {
    table default {
        route 172.16.0.0/24 {
            next {
                hop 10.0.0.1 {
                }
            }
        }
    }
}

redistribute {
    destination bgp {
        import static
    }
}

bgp {
    router-id 10.0.0.1

    peer test-peer {
        connection {
            remote { ip 10.0.0.2 }
            local  { ip 10.0.0.1 }
        }
        session {
            asn { local 65000; remote 65001 }
            family { ipv4/unicast { prefix { maximum 1000000 } } }
        }
        update {
            attribute { origin igp; next-hop 10.0.0.1 }
            nlri  { ipv4/unicast add 192.168.1.0/24 }
        }
    }
}
```

- `172.16.0.0/24` is a static route redistributed into BGP (declarative, any-source→any-dest).
- `192.168.1.0/24` is a direct per-peer announcement (no plugin wiring).
- `process { run <cmd> }` and `process rib` move to an advanced "attach your own handler /
  make a peer RIB-backed" section, not the first thing a newcomer sees.

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
| Producer registered without config source (static) | 2nd+ time this class appears? verify via parity test | "Every redistribute producer MUST register a source; enforced by parity test" | Add parity test (AC-5); if recurs, add rule |
| Custom validator on a list key silently dead | new | "List-key `ze:validate` must be exercised by a test" | Bug B fix + `TestListKeyCustomValidatorRuns` |

## Design Insights
- Two parallel registries exist: `redistevents` (producer, ProtocolID-keyed, for the bus) and
  `config/redistribute` (source, Name-keyed, for config). A protocol must appear in BOTH. The
  parity test binds them so future protocols can't register one and forget the other.
- Bug B is not redistribute-specific: it is a general hole in list-key validation. Fixing it in
  the walker fixes every keyed list whose key has `ze:validate`.
- **Registration belongs in `init()`, not plugin-run.** Source registration is pure metadata
  and must be visible to `ze config validate`, which imports plugins (via `all.go`) but never
  starts their engines. BGP already does this correctly; the rest register at run and are thus
  invisible to validation. This is why B1 (validate the key) is inseparable from B2 (register
  at init). It also generalizes: anything the config validator must check against a registry
  has to be populated at import time, not at engine start.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix Bug B in the walker (validate list keys) | Add a bespoke redistribute config-load validator that re-runs LookupSource during `validate` | Walker fix closes the whole class; a bespoke check would leave every other keyed list still unguarded |
| Register `static` source in the static plugin | Central source table | Plugin self-containment (`ai/rules/plugin-self-containment.md`) |
| Parity test enumerating the registry | Manual audit | Prevents recurrence mechanically |

## Known Limitations
- Full daemon runtime could not be exercised on the macOS dev host during research (BGP plugin
  tier fails to start locally, unrelated to this change); end-to-end proof relies on the
  Linux interop/functional harness.
- The larger "RIB-backed-by-default, `process` for handlers only" redesign is deliberately NOT
  in scope; this spec only removes `process rib` from the quickstart's first example.

## Implementation Summary

### What Was Implemented
- [filled during /implement]

### Bugs Found/Fixed
- Bug A: static not a registered redistribute source.
- Bug B: `ze:validate` on list keys never executed.

### Documentation Updates
- [filled during /implement]

### Deviations from Plan
- [filled during /implement]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `import static` works end to end | interop/functional | [test name + peer sees prefix] |
| bad source rejected at validate | functional | [`ze config validate` non-zero + message] |
| quickstart correct | functional | [config validates + advertises] |
| class can't recur | unit | [parity test enumerates producers] |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | | |

### Fixes applied
-

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional/interop tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary + Audit filled
- [ ] Learned summary written to `plan/learned/NNN-redist-source-registration.md`
- [ ] Commit A: code + tests + docs + spec + learned summary
- [ ] Commit B: `git rm plan/spec-redist-source-registration.md`

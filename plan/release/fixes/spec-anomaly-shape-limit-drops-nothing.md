# Spec: anomaly-shape-limit-drops-nothing

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | A shipped plugin's default response does nothing while reporting the source as armed |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`anomaly-shape { mode armed }` is meant to throttle each anomalous source to a configured packet rate (`limit-rate` per `limit-unit`, with optional `limit-burst`). Its default action, `limit`, currently returns `[firewall.Counter{}, firewall.Limit{Rate, Unit, Dimension: RateDimensionPackets, Burst}]` in `buildActions` (`internal/plugins/anomaly/shape/match.go`), leaving `Over` false, placing `Counter` ahead of `Limit`, and emitting no `firewall.Drop` verdict.

The nftables lowering (`lowerLimit`, `internal/plugins/firewall/nft/lower_linux.go`) lowers `firewall.Limit` to `expr.Limit` alone, carrying `Over` as given and emitting no verdict of its own. Because `mkTable` (`internal/plugins/anomaly/shape/match.go`) attaches the `ingress` filter chain to `HookInput` with `PolicyAccept`, a packet under the rate matches `limit` and continues to `policy accept`, and a packet over the rate fails `limit` and also continues to `policy accept`. Meanwhile the leading `counter` increments on every packet from that source rather than on dropped excess packets, `show anomaly shape` lists the source as armed, and `ze_anomaly_shape_armed` counts it as mitigated while zero packets are dropped.

Evidence: verified at HEAD by reading `internal/plugins/anomaly/shape/match.go`, `internal/plugins/anomaly/shape/match_test.go`, `internal/plugins/firewall/nft/lower_linux.go`, and `internal/plugins/firewall/nft/backend_linux.go` (owning the 2026-11-12 row in `plan/journal/silent-fall-through.md`).

The goal: a source armed under `action limit` has its packets above the configured rate (plus burst) counted by the term's counter and dropped by the kernel, while its packets at or below the rate (and traffic from unarmed sources) pass unaffected, using the exact excess-and-drop action order `[firewall.Limit{..., Over: true}, firewall.Counter{}, firewall.Drop{}]`.

## Required Reading

### Subsystem Doors
- [ ] `ai/doors/system.md` - platform boundaries, reuse of existing patterns, updating affected pages in the same change
  → Constraint: reuse Ze's existing excess-and-drop firewall model pattern rather than adding plugin-specific lowering or bypassing the firewall component
- [ ] `ai/doors/system-plugins.md` - edge plugin placement, process-boundary rules, firewall lowering as a backend
  → Constraint: `anomaly-shape` builds abstract `firewall.Table` values and hands them to `firewall.RegisterTables` / `firewall.ApplyAll`; lowering belongs solely to the firewall backend
- [ ] `ai/doors/forwarding.md` - firewall table ownership, `ze_` naming prefix, backend verification
  → Constraint: keep reserved `ze_anomaly-shape` and `ze_anomaly-shape6` names under owner `anomaly-shape`

### Architecture Docs
- [ ] `docs/architecture/anomaly/anomaly-2-shape.md` - the responder and the term it installs
  → Constraint: the responder re-registers the whole owner term set on every change under key `anomaly-shape`, one table per IP family
  → Decision: update the `Surface` section in the same change to record the excess-and-drop term shape (`Limit{Over: true}`, `Counter`, `Drop`)
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - table registry, `ze_` prefix, table reservation, apply-time verification
  → Constraint: tables keep their reserved `ze_` names (`ze_anomaly-shape`, `ze_anomaly-shape6`); `ApplyAll` verifies runtime-built tables against the active backend
- [ ] `docs/architecture/testing/qemu-integration.md` - build tags and `userns.Enter` for Linux kernel firewall tests
  → Constraint: a kernel-touching nftables test served by a user namespace MUST use `//go:build linux` and call `userns.Enter(t)` first (not `//go:build integration && linux`) so every Linux unit run and QEMU unit phase executes it

**Key insights:**
- nftables evaluates expressions inside a rule left to right and stops evaluating at the first non-matching expression. Putting `firewall.Counter{}` between `firewall.Limit{Over: true}` and `firewall.Drop{}` makes the counter count dropped excess packets (and not accepted within-rate packets), while `addChainRules` (`internal/plugins/firewall/nft/backend_linux.go`, `hasCounterExpr`) sees the explicit `*expr.Counter` and does not prepend a duplicate leading counter.
- `lowerLimit` (`internal/plugins/firewall/nft/lower_linux.go`), `formatLimit` (`internal/component/firewall/cmd/show.go`), and `page_firewall.go` (`internal/component/web/page_firewall.go`) already support `firewall.Limit.Over = true` and render `over` explicitly.
- On VPP, `firewallvpp.Verify` (`internal/plugins/firewall/vpp/verify.go`) already refuses `firewall.Counter`, so `ApplyAll` rejects `anomaly-shape` tables cleanly on VPP without silent approximation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/anomaly/shape/match.go` - `buildActions` (lines 28-38) returns `[firewall.Counter{}, firewall.Drop{}]` for `ActionDrop` and `[firewall.Counter{}, firewall.Limit{Rate, Unit, Dimension: RateDimensionPackets, Burst}]` for `ActionLimit`, with `Over` left false, `Counter` ahead of `Limit`, and no `firewall.Drop`; `mkTable` (lines 75-89) creates a base `ChainFilter` chain `ingress` at `HookInput`, priority `-200`, with `PolicyAccept`
- [ ] `internal/plugins/anomaly/shape/match_test.go` - `TestBuildSourceTerm` (lines 16-50) checks only that some `firewall.Limit` appears in `term.Actions` for `ActionLimit` and some `firewall.Drop` for `ActionDrop`; it never inspects `Over`, a trailing `firewall.Drop`, or action order
- [ ] `internal/plugins/anomaly/shape/responder.go` - `reinstall` registers `buildTables(prefixes, r.cfg)` under owner `anomaly-shape` via `registerTables` and reconciles via `applyAll`; `onDetected`, `onOngoing`, `onCleared`, `autoRevertFire`, `killSwitch`, and `Stop` manage armed state and timers
- [ ] `internal/plugins/anomaly/shape/config.go` - `DefaultConfig` defaults `Action` to `ActionLimit` (`"limit"`), `LimitRate` to `1000`, `LimitUnit` to `"second"`, `LimitBurst` to `0`; `Validate` validates rate >= 1 and rate unit via `firewall.RateUnitSeconds`
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerLimit` (lines 1279-1301) emits `&expr.Limit{Type, Rate, Unit, Burst, Over: l.Over}` with no verdict; `lowerCounter` emits `&expr.Counter{}`; `lowerAction` emits `&expr.Verdict{Kind: expr.VerdictDrop}` for `firewall.Drop`
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `addChainRules` (lines 288-291) checks `hasCounterExpr(exprs)` and skips `withCounter()` when the rule already carries an explicit `*expr.Counter`; `readRuleCounter` (lines 443-455) extracts the first `*expr.Counter` for `GetCounters`
- [ ] `internal/plugins/flowspec-firewall/rule_chains.go` - the `-excess` term (lines 52-58) uses `[firewall.Limit{..., Over: true}, firewall.Drop{}]` so excess packets drop while conforming packets fall through
- [ ] `internal/plugins/copp/translate.go` - `neighborChainTerms` (lines 184-202) uses `limit.Over = true` with `firewall.Drop{}` for `over-limit-policy drop`
- [ ] `internal/component/firewall/cmd/show.go` - `formatLimit` (lines 268-279) already renders `limit rate over ...` when `Over` is true
- [ ] `internal/component/web/page_firewall.go` - rule formatter (lines 460-471) already renders `limit over ...` when `Over` is true
- [ ] `internal/plugins/firewall/vpp/verify.go` - `verifyAction` (lines 374-375) already refuses `firewall.Counter` on the VPP backend

**Behavior to preserve:**
- `ActionDrop` fallback continues to emit `[firewall.Counter{}, firewall.Drop{}]`
- Table names (`ze_anomaly-shape`, `ze_anomaly-shape6`), owner key (`anomaly-shape`), chain metadata (`ingress`, `ChainFilter`, `HookInput`, priority `-200`, `PolicyAccept`), and per-family split in `buildTables` / `mkTable` stay unchanged
- Responder state machine (`shadow` default, allowlist, blast-radius cap, TTL auto-revert, early clear, kill-switch, `show anomaly shape`, metrics, doctor check) stays unchanged
- `firewall.Limit` lowering (`lowerLimit`), `addChainRules` counter handling, and CLI/web rendering stay unchanged

**Behavior to change:**
- `buildActions` in `internal/plugins/anomaly/shape/match.go` for `ActionLimit` (default) changes from `[firewall.Counter{}, firewall.Limit{..., Over: false}]` to `[firewall.Limit{Rate: cfg.LimitRate, Unit: cfg.LimitUnit, Dimension: firewall.RateDimensionPackets, Burst: cfg.LimitBurst, Over: true}, firewall.Counter{}, firewall.Drop{}]`
- `docs/architecture/anomaly/anomaly-2-shape.md` documents the excess-and-drop term shape (`Limit` with `Over: true`, `Counter`, `Drop`) and cleans up the duplicate `match.go` source anchor

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Excess-and-drop limiter idiom | `internal/plugins/flowspec-firewall/rule_chains.go` (`ruleChains`, `-excess` term) and `internal/plugins/copp/translate.go` (`neighborChainTerms`, `overPolicyDrop`) | Uses `firewall.Limit{..., Over: true}` followed by `firewall.Drop{}` in an accept-policy chain so excess packets are dropped and within-rate packets fall through to accept | Extends: `anomaly-shape` `buildActions` adopts the exact same `Limit{Over: true}` + `Drop` idiom (with `Counter` between them) so lowering needs zero changes |
| `firewall.Limit` model and nftables lowering | `internal/component/firewall/model.go` (`Limit`), `internal/plugins/firewall/nft/lower_linux.go` (`lowerLimit`) | Lowers `Over: true` directly to `expr.Limit{Over: true}` with packet/byte dimension, rate, unit, and burst | Reused unchanged: `Over` is already modeled and lowered to `limit rate over` |
| Explicit `Counter` placement preservation and readback | `internal/plugins/firewall/nft/backend_linux.go` (`addChainRules`, `hasCounterExpr`, `readRuleCounter`, `GetCounters`) | Keeps an explicit `*expr.Counter` where the term places it without prepending a second leading counter, and reads that counter back for `show firewall ruleset` | Reused unchanged: placing `firewall.Counter{}` after `firewall.Limit{Over: true}` and before `firewall.Drop{}` makes the term counter count dropped excess packets |
| `Over` display in CLI and web UI | `internal/component/firewall/cmd/show.go` (`formatLimit`), `internal/component/web/page_firewall.go` (action formatter) | Renders `Over: true` explicitly as `limit rate over ...` (CLI) and `limit over ...` (web) | Reused unchanged: operator inspection already surfaces the inverted limiter accurately |
| `ActionDrop` fallback | `internal/plugins/anomaly/shape/match.go` (`buildActions`), `internal/plugins/anomaly/shape/config.go` (`ActionDrop`) | Emits `[firewall.Counter{}, firewall.Drop{}]` when `action drop` is configured | Preserved unchanged |
| Unprivileged Linux namespace test harness | `internal/test/userns/userns_linux.go` (`userns.Enter`), pattern in `internal/plugins/copp/copp_integration_linux_test.go` | Re-executes a `//go:build linux` test in a private user, network, and mount namespace with `lo` up and full `CAP_NET_ADMIN` over its own nftables | Reused unchanged: drives `TestAnomalyShapeLimitDropsTheExcess` on every Linux unit run and QEMU unit phase without requiring host root |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Before you write code in a subsystem | the door every change reads | "Grep for the pattern that already exists before you design one" | agrees | Reuses the existing `firewall.Limit{Over: true}` + `firewall.Drop{}` pattern from `flowspec-firewall` and `copp`, touches no `pkg/` API or central switch, and updates `docs/architecture/anomaly/anomaly-2-shape.md` in the same change |
| `docs/architecture/anomaly/anomaly-2-shape.md` | Surface | the `// Design:` page of `internal/plugins/anomaly/shape/match.go`; anchored to `internal/plugins/anomaly/shape/match.go`; a page this spec modifies | "Each incident source gets its own rate limit (`firewall.MatchSourceAddress` plus `Limit`, with a drop fallback) and a TTL." | agrees | Makes the `limit` action actually enforce the per-source rate limit the page describes by setting `Over: true` and appending `Counter` and `Drop`, preserving whole-set replace, per-family tables, and the `drop` fallback; the doc edit clarifies that exact term shape |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 2: a producer's table name carries the `ze_` prefix | anchored to `internal/plugins/anomaly/shape/match.go` | "The kernel firewall backend uses `firewall.Table.Name` verbatim and adds nothing. It reconciles and deletes only kernel tables whose name starts with `ze_`." | agrees | Preserves `ze_anomaly-shape`, `ze_anomaly-shape6`, owner key `anomaly-shape`, and table reservations untouched; only the `ActionLimit` action sequence inside each term changes |
| `ai/doors/system-plugins.md` | Where it sits | the door covering `internal/plugins/anomaly/shape/match.go`, `match_test.go`, and `limit_integration_linux_test.go` | "The component owns the model, and the plugin turns it into kernel or VPP state (`ai/digests/firewall.md`)." | agrees | Keeps `anomaly-shape` strictly on the `firewall` component's abstract action types (`Limit`, `Counter`, `Drop`) via `RegisterTables` / `ApplyAll`, with zero backend-specific code added to the plugin |

## Data Flow

### Entry Point
- An `anomalyevent.AnomalyDetected` event (`EntityKind: EntityKindSource`) delivered on the event bus to `responder.onDetected` while configured with `anomaly { shape { mode armed; action limit; limit-rate R; limit-unit U; limit-burst B; } }`.

### Transformation Path
1. `responder.onDetected` (`internal/plugins/anomaly/shape/responder.go`) checks `actsOn`, `killed`/`ModeShadow`, allowlist, and `BlastRadiusCap`, inserts the source prefix into `r.armed`, and calls `r.reinstall()`.
2. `r.reinstall()` builds the full per-family table slice via `buildTables(prefixes, r.cfg)` -> `buildSourceTerm(entity, cfg)` -> `buildActions(cfg)` (`internal/plugins/anomaly/shape/match.go`).
3. For `ActionLimit`, `buildActions(cfg)` returns `[firewall.Limit{Rate, Unit, Dimension: RateDimensionPackets, Burst, Over: true}, firewall.Counter{}, firewall.Drop{}]`.
4. `r.reinstall()` calls `firewall.RegisterTables("anomaly-shape", tables)` and `firewall.ApplyAll()` (`internal/component/firewall/registry.go`).
5. `firewall.ApplyAll()` merges tables, loads the OS-default `kernel` backend on demand, runs the backend verifier (if any) and commit observers, and calls `backend.Apply(all)`.
6. `firewallnft` (`internal/plugins/firewall/nft/backend_linux.go`, `lower_linux.go`) lowers each term to `MatchSourceAddress` (`Payload` + `Bitwise` + `Cmp`), `&expr.Limit{Over: true}`, `&expr.Counter{}`, `&expr.Verdict{Kind: expr.VerdictDrop}` in the `ingress` input-hook chain (`policy accept`), skipping auto-counter insertion because `hasCounterExpr` is true.
7. On incoming packets at the kernel input hook: packets from the armed source within rate+burst fail `limit rate over` and fall through to `policy accept` without touching `counter`; packets exceeding rate+burst match `limit rate over`, increment `counter`, and are discarded by `drop`; packets from other sources fail `MatchSourceAddress` and pass to `policy accept`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `anomaly-shape` plugin ↔ `firewall` component | In-process `firewall.RegisterTables("anomaly-shape", tables)` and `firewall.ApplyAll()` (process-local firewall table registry; `table-ownership-and-shutdown-flush.md`) | No |
| `firewall` component ↔ Linux kernel (`firewallnft`) | Netlink `nf_tables` message batch (`AddTable`, `AddChain`, `AddRule`, `Flush`) via `google/nftables` | No |
| Kernel rule counters → CLI/API readback | `firewall.GetBackend().GetCounters("ze_anomaly-shape")` (`readRuleCounter`, `show firewall ruleset`) | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/` for `firewall.Limit`, `Over:`, and `anomaly-shape` / `anomaly_shape` (`grep -rn "firewall\.Limit\|Over:" internal/` and `grep -rn "anomaly-shape\|anomaly_shape\|anomaly shape" docs/ internal/ test/`). Positive controls found: `internal/plugins/flowspec-firewall/rule_chains.go` (`Over: true` writer), `internal/plugins/copp/translate.go` (`limit.Over = true` writer), `internal/plugins/firewall/nft/lower_linux.go` (`lowerLimit` netlink encoder reading `l.Over`), `internal/component/firewall/cmd/show.go` (`formatLimit` CLI reader of `l.Over`), `internal/component/web/page_firewall.go` (web reader of `v.Over`), and `internal/plugins/anomaly/shape/match.go` (the defective writer).

No wire format, RPC message, JSON event envelope, YANG schema leaf, CLI output contract, or netlink expression schema changes in this spec. `anomalyevent.AnomalyDetected/Ongoing/Cleared`, `ze-anomaly-shape-conf.yang`, `show anomaly shape` (`shapeStatus`), `firewall.Limit` (including `Over`), `firewall.Counter`, `firewall.Drop`, `lowerLimit`, `formatLimit`, and `page_firewall.go` are all unchanged. The only behavioral change is the in-process `[]firewall.Action` slice returned by `buildActions` in `internal/plugins/anomaly/shape/match.go`, which populates the existing `firewall.Limit.Over` boolean and appends existing `firewall.Counter{}` and `firewall.Drop{}` actions.

### Integration Points
- `internal/plugins/anomaly/shape/match.go` `buildActions` - emits `[Limit{Over: true}, Counter, Drop]` for `ActionLimit`
- `internal/plugins/firewall/nft/lower_linux.go` `lowerLimit` - unchanged; already lowers `Over: true`
- `internal/plugins/firewall/nft/backend_linux.go` `addChainRules` / `hasCounterExpr` / `readRuleCounter` - unchanged; preserves explicit `Counter` placement after `Limit{Over: true}` and reports its count via `GetCounters`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `responder.onDetected` -> `buildTables` -> `buildSourceTerm` -> `buildActions` -> `firewall.RegisterTables` -> `firewall.ApplyAll` -> `firewallnft` |
| No unintended coupling (components stay isolated) | Yes | `anomaly-shape` imports only `internal/component/firewall` model types; test file blank-imports `firewall/nft` only in `_test.go` (same pattern as `internal/plugins/copp/copp_integration_linux_test.go`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `firewall.Limit{Over: true}` + `firewall.Drop{}` idiom from `flowspec-firewall` and `copp` and `userns.Enter` from `internal/test/userns` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | N-A to data-plane packet copying; `buildActions` allocates one 3-element `[]firewall.Action` slice on low-rate control-plane incident arming |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them | Yes | No new commands, views, families, or handlers; existing `anomaly-shape` plugin and table reservations are unchanged |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name | Yes | Verified via `grep -rn "anomaly-shape"` across `internal/` and `docs/`; no switch, parser, validator, or runner list changes |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux nftables evaluates `ip saddr X limit rate over R burst B counter drop` left to right so within-rate packets skip `counter` and `drop` while excess packets increment `counter` and drop | `internal/plugins/firewall/nft/lower_linux.go` (`loweredRule`, `lowerLimit`) and `backend_linux.go` (`addChainRules`, `hasCounterExpr`, `readRuleCounter`) | Counter would count all packets instead of drops, or packets would not be throttled | `TestAnomalyShapeLimitDropsTheExcess` on a real Linux kernel inside `userns.Enter` | unvalidated |
| A-2 | `userns.Enter(t)` grants sufficient privilege in a private user+network namespace to apply `ze_anomaly-shape` (`ip` and `ip6` filter input chains with `MatchSourceAddress`, `Limit`, `Counter`, `Drop`) without requiring host root or `//go:build integration` | `docs/architecture/testing/qemu-integration.md` ("Build Tags") and `internal/plugins/copp/copp_integration_linux_test.go` | Test would fail namespace setup or `ApplyAll` with `EPERM` and require investigation before touching build tags | Running `./le test unit pkg path ./internal/plugins/anomaly/shape/` on Linux | unvalidated |
| A-3 | Existing unit tests in `internal/plugins/anomaly/shape/` (`TestBuildSourceTerm`, `TestBuildTablesPerFamily`, `responder_test.go`) remain green with the 3-action slice `[Limit{Over: true}, Counter, Drop]` | Read `match_test.go` (iterates `term.Actions` looking for `firewall.Limit`) and `responder_test.go` (checks term counts, not action slices) | An existing test asserting exact action slice length would need review | `./le test unit pkg path ./internal/plugins/anomaly/shape/` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Kernel token bucket timing variance in `TestAnomalyShapeLimitDropsTheExcess` makes a fixed packet count assertion flaky under host load | Intermittent test failures when a slow send loop refills extra tokens | Measure the actual send window duration around the tight UDP burst and bound `received` by `burst + ceil(rate * window) + slack` (mirroring `copp_integration_linux_test.go`), send a burst much larger than `burst + 1s * rate`, and check `counter.Packets == uint64(sent - received)` |
| R-2 | `LimitBurst: 0` in Linux nftables `expr.Limit` uses a kernel-default burst (5 packets when burst is omitted/0 in `nft_limit_init`) rather than strictly 0 | Observed `received` exceeds `0 + refill` when testing with `LimitBurst: 0` | Configure an explicit non-zero `LimitBurst` (e.g. `5`, `LimitRate: 5/second`) and send a large burst (e.g. `60` packets) so both the within-rate floor and the excess-drop ceiling are unambiguous, and note kernel burst behavior rather than asserting zero burst |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong `Over` polarity (leaving `Over: false` with `Drop`) would invert the throttle and drop conforming traffic while accepting excess traffic; a wrong counter placement would misreport dropped packets as total packets |
| How is it reverted? | Single commit revert of `internal/plugins/anomaly/shape/match.go`, tests, and doc |
| Who else touches this path? | Only `internal/plugins/anomaly/shape`; `firewall/nft`, `flowspec-firewall`, and `copp` are untouched |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `responder.onDetected` for an armed source under `action limit` (`ModeArmed`, `ActionLimit`) | → | `reinstall` → `buildTables` → `buildSourceTerm` → `buildActions` → `firewall.RegisterTables` → `firewall.ApplyAll` → `firewallnft` kernel rule | `TestAnomalyShapeLimitDropsTheExcess` (`internal/plugins/anomaly/shape/limit_integration_linux_test.go`, `//go:build linux` with `userns.Enter`: excess packets from the armed source are dropped and counted, within-rate packets pass, unarmed source passes whole, and `onCleared` withdraws the rule) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A source armed via `responder.onDetected` under `mode armed` and `action limit` at rate R and burst B sends a burst of N packets exceeding B + R*window over loopback to a local listener, alongside an identical burst from an unarmed control source | The kernel accepts the within-budget packets from the armed source (received > 0), drops the excess packets above B + R*window (received < N), records the dropped excess packet count in the term's kernel counter via `GetCounters(tableNameV4)` (counter.Packets == N - received, strictly between 0 and N), delivers all N packets from the unarmed control source, and delivers all N packets from the previously armed source after `responder.onCleared` withdraws the rule | Within-budget packets (up to burst B plus token refill over the send window) pass because `action limit` is a surgical throttle, not a block; traffic from unarmed sources passes whole because the rule matches only the armed `MatchSourceAddress` prefix |
| AC-2 | `buildActions` and `buildSourceTerm` called with `ActionLimit` (default config) and with `ActionDrop` | For `ActionLimit`, `term.Actions` is the exact 3-element sequence `[firewall.Limit{Rate, Unit, Dimension: RateDimensionPackets, Burst, Over: true}, firewall.Counter{}, firewall.Drop{}]`; for `ActionDrop`, `term.Actions` is `[firewall.Counter{}, firewall.Drop{}]` | `firewall.Counter{}` stays in both actions (after `Limit{Over: true}` and before `Drop` for `ActionLimit`; before `Drop` for `ActionDrop`) so `show firewall ruleset` and `GetCounters` read the drop count without `addChainRules` inserting a leading counter |
| AC-3 | Architecture doc accuracy check on `docs/architecture/anomaly/anomaly-2-shape.md` | The `Surface` section documents the exact `limit` action term shape (`MatchSourceAddress`, `Limit` with `Over: true`, `Counter`, `Drop`) and `./le doc check all` passes with no duplicate or broken source anchors | Existing `anomaly-shape` safety capabilities, metrics, and table names remain documented unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `anomaly { shape { mode armed; action limit; limit-rate R; limit-unit second; limit-burst B; } }` and an anomalous source is detected | `AnomalyDetected` -> `responder.onDetected` -> `reinstall` -> `buildTables` -> `buildSourceTerm` -> `buildActions` -> `firewall.RegisterTables` -> `firewall.ApplyAll` -> `firewallnft` -> kernel drops excess packets and counts them while letting conforming packets and other sources pass | `TestAnomalyShapeLimitDropsTheExcess` |
| 2 | Clears the anomaly incident (or TTL expires) | `AnomalyCleared` -> `responder.onCleared` -> `withdraw` -> `reinstall` -> `firewall.RegisterTables(owner, nil)` -> `firewall.ApplyAll` -> kernel removes `ze_anomaly-shape` and all packets from that source pass again | `TestAnomalyShapeLimitDropsTheExcess` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildActionsLimitDropsTheExcess` | `internal/plugins/anomaly/shape/match_test.go` | AC-2: exact 3-action order `[Limit{Over: true, Rate, Unit, Dimension: RateDimensionPackets, Burst}, Counter, Drop]` for `ActionLimit` (IPv4 and IPv6 terms), and preserved `[Counter, Drop]` for `ActionDrop` | |

### Kernel Integration Tests (`//go:build linux` with `userns.Enter`)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAnomalyShapeLimitDropsTheExcess` | `internal/plugins/anomaly/shape/limit_integration_linux_test.go` | AC-1 and Wiring Test: real `responder.onDetected` -> `firewall.ApplyAll` -> kernel drops excess UDP packets from armed source, passes within-rate packets, term counter equals dropped excess count (`N - received`), unarmed control source passes 100%, and `responder.onCleared` restores full delivery; MUST fail RED against HEAD before `match.go` is changed | |

### Boundary Tests (numeric inputs)
N-A: no numeric config ranges, parsers, or validators change; `limit-rate` (>= 1), `limit-unit` (`firewall.RateUnitSeconds`), and `limit-burst` (`uint32`) validation in `config.go` and `limit_unit_test.go` are unchanged.

### Functional Tests
N-A: `anomaly-shape` is driven by in-process `anomalyevent.Detected` bus events (no operator CLI command triggers arming directly, and `test/plugin/anomaly-shape-shadow.ci` already covers plugin startup and `show anomaly shape`). Kernel packet enforcement and counter readback are proven directly against a real Linux kernel in `TestAnomalyShapeLimitDropsTheExcess` via `userns.Enter(t)`, which runs on every Linux unit pass and in QEMU `unitPhase`.

## Files to Modify
- `internal/plugins/anomaly/shape/match.go` - `buildActions` emits `[firewall.Limit{..., Over: true}, firewall.Counter{}, firewall.Drop{}]` for `ActionLimit`
- `internal/plugins/anomaly/shape/match_test.go` - add `TestBuildActionsLimitDropsTheExcess` validating AC-2 while preserving existing tests
- `docs/architecture/anomaly/anomaly-2-shape.md` - document the excess-and-drop term shape (`Limit{Over: true}`, `Counter`, `Drop`) in `Surface` and remove the duplicate `match.go` source anchor on line 16

## Files to Create
- `internal/plugins/anomaly/shape/limit_integration_linux_test.go` - `//go:build linux` kernel test using `userns.Enter(t)` implementing `TestAnomalyShapeLimitDropsTheExcess` (AC-1 + Wiring Test)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Existing `internal/plugins/anomaly/shape/yang/ze-anomaly-shape-conf.yang` already declares `mode`, `action`, `limit-rate`, `limit-unit`, and `limit-burst` |
| YANG validation constraints | No | Existing `range "1..max"`, `enumeration`, and `uint32` constraints are unchanged |
| YANG custom validators | No | No new config leaves or cross-leaf rules |
| CLI commands/flags | No | `show anomaly shape` and `show firewall ruleset` are unchanged (`formatLimit` already prints `over`) |
| CLI grammar (keyword before value) | No | No CLI command added or changed |
| Editor autocomplete | No | No YANG leaves added or changed |
| Functional test for new RPC/API | N-A | No new RPC or API; existing `test/plugin/anomaly-shape-shadow.ci` covers `show anomaly shape` and kernel enforcement is proven by `TestAnomalyShapeLimitDropsTheExcess` |
| Pipe completeness | N-A | No command output changed |
| Env var registration | No | No env var added or changed |
| Doctor check for runtime dependencies | No | Existing `anomaly-shape-firewall` (`doctor-anomaly-shape-armed-no-firewall`) in `internal/plugins/anomaly/shape/doctor.go` is unchanged |
| Doctor code per problem class | No | No new refusal, warning, or error class introduced |
| Prometheus counters/metrics | No | Existing four metrics in `internal/plugins/anomaly/shape/metrics.go` are unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | Scope is `plugin` (anomaly firewall responder), not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `docs/features.md` line 60 already describes per-source rate-limit via `MatchSourceAddress`+`Limit` and drop fallback accurately |
| 2 | Config syntax changed? | No | No config syntax change |
| 3 | CLI command added/changed? | No | No CLI command change |
| 4 | API/RPC added/changed? | No | No API/RPC change |
| 5 | Plugin added/changed? | No | Existing plugin behavior repaired to match documented contract |
| 6 | Has a user guide page? | No | Checked `docs/guide/anomaly.md` lines 91-162: already states `action limit` rate-limits the armed source without claiming the broken term shape |
| 7 | Wire format changed? | No | No wire format change |
| 8 | Plugin SDK/protocol changed? | No | No SDK/protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is `plugin`, no RFC requirement |
| 10 | Test infrastructure changed? | No | Reuses existing `internal/test/userns` (`//go:build linux`) without adding a new test harness or QEMU list entry |
| 11 | Affects daemon comparison? | No | Not a comparison change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/anomaly/anomaly-2-shape.md` (`Surface` section: specify `Limit{Over: true}`, `Counter`, `Drop` order) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors spec plan/release/fixes/spec-anomaly-shape-limit-drops-nothing.md` and `grep -rn "anomaly/shape/match.go" docs/`: `docs/architecture/anomaly/anomaly-2-shape.md` (declared by `// Design:` in `match.go`, updated by this spec), `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (anchors `tableNameV4`, unaffected because table names are unchanged), `docs/features.md` (anchors source term + per-family tables, unaffected) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Checked `docs/guide/anomaly.md` lines 95-107 and `docs/guide/command-reference.md` lines 1819-1828: examples remain accurate |

## Design presentation

### What changes
When `anomaly-shape` is armed with its default `action limit`, each armed source's firewall term now emits `firewall.Limit{..., Over: true}`, followed by `firewall.Counter{}`, followed by `firewall.Drop{}` instead of `[firewall.Counter{}, firewall.Limit{..., Over: false}]`. On the Linux nftables backend, packets from the armed source within the configured rate and burst fall through to the chain's accept policy, while excess packets above the rate match `limit rate over`, increment the term's drop counter, and are dropped.

### Before and after
1. **Armed source sending above `limit-rate` (e.g. 60 packets in a tight burst under `limit-rate 5; limit-unit second; limit-burst 5;`)**
   - **Before:** `buildActions` emits `[Counter, Limit{Over: false}]`, lowering to `ip saddr <src> counter limit rate 5/second burst 5` with no verdict in an `accept`-policy input chain. All 60 packets are accepted, the rule counter reports 60 (all traffic, not drops), and `show anomaly shape` reports the source as armed.
   - **After:** `buildActions` emits `[Limit{Over: true}, Counter, Drop]`, lowering to `ip saddr <src> limit rate over 5/second burst 5 counter drop`. Only the burst plus refill packets are accepted, the excess packets are dropped by the kernel, and the term's counter in `show firewall ruleset` equals the dropped excess packet count.
2. **Firewall rule display (`show firewall ruleset`)**
   - **Before:** renders `counter limit rate 1000/second burst 0` with no `over` and no `drop`.
   - **After:** renders `limit rate over 1000/second burst 0 counter drop`.

### Alternatives set aside
| Alternative | Why set aside |
|-------------|---------------|
| Keep `firewall.Counter{}` before `firewall.Limit{Over: true}` (`[Counter, Limit{Over: true}, Drop]`) | In nftables left-to-right evaluation, a counter ahead of `limit rate over` increments on every packet from the armed source, including conforming packets that pass, so `show firewall ruleset` cannot report dropped excess packets (violating AC-1). Placing `Counter` after `Limit{Over: true}` and before `Drop` counts exact drops while still preventing `addChainRules` from prepending a duplicate leading counter |
| Use `limit rate R accept` followed by a second `drop` term for the same source | Splits one armed source across two rules in the base chain, complicates per-term naming and counter readback (`anomaly-<prefix>`), and departs from the single-term `Limit{Over: true}` + `Drop` idiom already used in `flowspec-firewall` (`rule_chains.go`) and `copp` (`translate.go`) |
| Add special-case verdict synthesis in `firewallnft` `lowerLimit` | `firewall.Limit` is a reusable conditional expression (used with `Accept` in `copp` `over-limit-policy accept` and with `Drop` in `flowspec-firewall` and `copp` `over-limit-policy drop`). Making `lowerLimit` implicitly drop would break other callers and violate separation between model and backend |
| Tag the kernel test `//go:build integration && linux` and require QEMU guest-root | `docs/architecture/testing/qemu-integration.md` ("Build Tags") requires kernel-touching nftables tests that a user namespace serves to use `//go:build linux` + `userns.Enter(t)` so they run on every Linux unit run as well as in QEMU `unitPhase` |

### Documents departed from
none

### Choices for Thomas
none

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- create `internal/plugins/anomaly/shape/limit_integration_linux_test.go` (`//go:build linux`, calling `userns.Enter(t)`) with `TestAnomalyShapeLimitDropsTheExcess` and add `TestBuildActionsLimitDropsTheExcess` in `internal/plugins/anomaly/shape/match_test.go`.
   - Tests: `TestAnomalyShapeLimitDropsTheExcess`, `TestBuildActionsLimitDropsTheExcess`
   - Files: `internal/plugins/anomaly/shape/limit_integration_linux_test.go`, `internal/plugins/anomaly/shape/match_test.go`
   - Verify: both tests FAIL RED against unmodified `match.go` (unit test fails on missing `Over: true` / `Drop` / action order; kernel test fails because all excess packets pass instead of dropping).
2. **Phase 2: Fix and Docs** -- update `buildActions` in `internal/plugins/anomaly/shape/match.go` to return `[firewall.Limit{Rate: cfg.LimitRate, Unit: cfg.LimitUnit, Dimension: firewall.RateDimensionPackets, Burst: cfg.LimitBurst, Over: true}, firewall.Counter{}, firewall.Drop{}]` for `ActionLimit`, and update `docs/architecture/anomaly/anomaly-2-shape.md` (`Surface` section + remove duplicate `match.go` anchor).
   - Tests: `TestBuildActionsLimitDropsTheExcess`, `TestAnomalyShapeLimitDropsTheExcess`, plus all existing tests in `./internal/plugins/anomaly/shape/`
   - Files: `internal/plugins/anomaly/shape/match.go`, `docs/architecture/anomaly/anomaly-2-shape.md`
   - Verify: main thread runs `./le test unit pkg path ./internal/plugins/anomaly/shape/`, `./le go lint run scope "./internal/plugins/anomaly/shape"`, `./le go vet-platforms darwin`, and `./le doc check all`; all pass green.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 (`TestAnomalyShapeLimitDropsTheExcess`), AC-2 (`TestBuildActionsLimitDropsTheExcess`), and AC-3 (`anomaly-2-shape.md`) all implemented and verified |
| Feature completeness | Full path from `responder.onDetected` through `buildTables` -> `buildSourceTerm` -> `buildActions` -> `firewall.RegisterTables` -> `firewall.ApplyAll` -> real kernel packet drop & counter readback |
| Correctness | Exact action order is `[Limit{Over: true}, Counter, Drop]` (NOT `[Counter, Limit{Over: true}, Drop]` and NOT `[Limit{Over: false}, Drop]`); `ActionDrop` fallback still emits `[Counter, Drop]` |
| Kernel test hygiene | `limit_integration_linux_test.go` uses `//go:build linux`, calls `userns.Enter(t)` as the first statement, does not call `t.Parallel()`, locks/cleans up namespace and closes firewall backend in `t.Cleanup`, and fails (never skips) on namespace privilege errors (`docs/architecture/testing/qemu-integration.md`) |
| Data flow | Zero changes to `lower_linux.go`, `backend_linux.go`, or `firewall` model; `hasCounterExpr` prevents duplicate counters |
| Platform compatibility | Cross-platform compilation holds on darwin (`./le go vet-platforms darwin`) because the kernel test is gated with `//go:build linux` |
| Journal ownership | Owned row in `plan/journal/silent-fall-through.md` (2026-11-12) updated from `open:` to `fixed:` with exact test evidence in Commit A |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `buildActions` emits `[Limit{Over: true}, Counter, Drop]` for `ActionLimit` | `TestBuildActionsLimitDropsTheExcess` passes via `./le test unit pkg path ./internal/plugins/anomaly/shape/` |
| Kernel drops excess packets, passes within-rate packets, counts drops, leaves unarmed sources untouched, and restores delivery on `onCleared` | `TestAnomalyShapeLimitDropsTheExcess` observed RED before fix and GREEN after fix via `./le test unit pkg path ./internal/plugins/anomaly/shape/ run TestAnomalyShapeLimitDropsTheExcess` |
| Architecture doc updated and anchors clean | `./le doc check all` passes |
| Lint and darwin cross-vet clean | `./le go lint run scope "./internal/plugins/anomaly/shape"` and `./le go vet-platforms darwin` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open vs fail-closed polarity | `Over: true` MUST be set alongside `firewall.Drop{}`; omitting `Over` would drop conforming traffic and pass excess floods, while omitting `Drop` leaves armed sources completely unthrottled |
| Blast-radius and source isolation | Term still matches `firewall.MatchSourceAddress{Prefix: entity}` only; unarmed sources and allowlisted sources are never throttled (`TestAnomalyShapeLimitDropsTheExcess` control source + `TestAllowlistNeverInstalls`) |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-12 | anomaly-shape default `limit` action has no verdict and drops nothing |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not run (populated at `/ze-close`) |
| `./le spec review check` | not run |
| Rounds | 0 |
| Reviewer lenses used | pending implementation |

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present with valid Status, Depends, Phase, and Updated
- [ ] All doors and producer files read in full before spec edit
- [ ] No code snippets; all checkboxes `[ ]`
- [ ] `## Existing Mechanisms`, `## Design documents`, `### Changed-Wire Census`, `## Acceptance Criteria`, and `## Design presentation` complete
- [ ] `./le spec design check spec plan/release/fixes/spec-anomaly-shape-limit-drops-nothing.md` clean except `no judge has judged this design`

### TDD
- [ ] Tests written (`TestBuildActionsLimitDropsTheExcess`, `TestAnomalyShapeLimitDropsTheExcess`)
- [ ] Tests FAIL RED before fix (paste output)
- [ ] Tests PASS GREEN after fix (paste output)
- [ ] Scoped verification run by main thread: `./le test unit pkg path ./internal/plugins/anomaly/shape/`, `./le go lint run scope "./internal/plugins/anomaly/shape"`, `./le go vet-platforms darwin`, `./le doc check all`
- [ ] Closure: append `plan/templates/TEMPLATE-CLOSURE.md`, complete `/ze-review` gate, update `plan/journal/silent-fall-through.md` row to `fixed:`, land Commit A + Commit B (`remove plan/release/fixes/spec-anomaly-shape-limit-drops-nothing.md`)

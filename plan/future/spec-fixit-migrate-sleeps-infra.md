# Spec: migrate-sleeps-infra

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | migrate-plugin-sleeps (committed edfe4c0e1), payload-predicate-waits (committed) |
| Phase | 2/9 |
| Updated | 2026-08-14 |

## Provenance

Reclassified as an improvement on 2026-08-14 at Thomas's instruction and moved
from `plan/` to `plan/future/`. Reason: this is test speed and determinism
only. Its one production-bug premise (P0) was carved out to
`spec-fixit-redistribute-establishment-stall`, which root-caused it to two test
harness defects and no engine stall. Read the correction blocks below before
you quote any number or premise in this spec.

## Correction 2026-08-03 (bookkeeping audit): the counts and the fib premise

**Two things in this spec are stale. Read this before you quote any number in it, and
before you build P4.**

**1. The sleep count. The tree holds 79, not 246 or 223.**

```
$ grep -rn "time\.sleep(" test --include=*.ci | wc -l
79
```

`test/.ci-sleep-baseline` sums to 79 as well (origin 125, then `-11 -12 -1 -1 -12 -8 -1`),
so the ratchet and the tree agree. Every "246" and "223" below is a 2026-07-14 reading and
is kept only as history. 167 of those sleeps have gone since, most of them in the
`test/policy` and `test/firewall` suites the baseline file documents. Re-derive the number
before you quote it. Do not carry 246 forward.

**2. AC-4's premise is false: a queryable fib signal EXISTS.**

The Current Behavior entry below says of `fib/kernel`: "no end-to-end 'FIB in sync' signal
exists (P4)". Producers read on 2026-08-03, not inferred:

| Producer | Where | What it does |
|----------|-------|--------------|
| `fibKernel.showInstalled` | `internal/plugins/fib/kernel/backend.go` | returns the `installed` map as `{prefix, next-hop}` JSON |
| the `OnExecuteCommand` handler | `internal/plugins/fib/kernel/register.go` | serves `show fib kernel` from `showInstalled`, and the plugin declares that command in its `sdk.Registration` |
| `fibKernel.applyBatch` install and replace branches | `internal/plugins/fib/kernel/fibkernel.go` | writes `f.installed[pfx]` only AFTER `addChange` / `replaceChange` returns nil, so a prefix appears in the map only once the netlink program succeeded |

So a bounded poll on `show fib kernel` until the prefix appears is a real signal that the
kernel route was programmed, and it is the P4 surface AC-4 asks for. The spec's own Revised
Approach already said so ("fib-kernel (uses existing `show fib kernel`)"); the Current
Behavior entry contradicts it and is the wrong half.

**Two bounds that stay true, and they are why the poll must be bounded rather than
one-shot.** Delivery from sysrib to fib-kernel is still fire-and-forget, so the prefix
arrives at a time the test cannot predict. And the map is fib-kernel's own record, so it
does not detect a route deleted out from under Ze by something else; `monitor.go` owns that
direction. Neither bound blocks the conversion.

Phase note (was in the Phase cell; moved 2026-07-22): the P0 establishment
investigation is RESOLVED -- it was carved out to
`spec-fixit-redistribute-establishment-stall`, which root-caused it (no engine
stall; two test-harness defects) and has landed F1-F4. This spec's
host-verifiable buckets are converted (baseline 246 -> 223 per the
Implementation Summary); the remaining work is QEMU-gated (P2/P3/P4/P8) plus
the infra-gated cases. Revised Approach signed off 2026-07-16.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `plan/deferrals.md` (the two 2026-07-14 `spec-migrate-plugin-sleeps` rows: bgp-redistribute group + DEFER/KEEP buckets). <!-- doc-links: ignore (the single deferrals file was retired for per-source shards) -->
3. `spec-migrate-plugin-sleeps` (closed 2026-08-12; the completed primitive-migration, whose Design Insights hold the conversion recipes -- read it from git history).
4. `test/scripts/ze_api.py` (existing primitives) and `internal/test/runner/` (the runner / engine-step executor).

## Task

> **REVISED 2026-07-14. SIGNED OFF 2026-07-16.** A first `/ze-implement` pass audited the
> codebase and attempted the flagship redistribute conversion. It found the spec's "build infra,
> conversion is mechanical" premise is wrong (see Core Insight + Mistake Log). The piece-by-piece
> framing (P1..P9) below is superseded by the revised plan in "Revised Approach" and "Revised
> Implementation Phases"; the original P1..P9 text is kept for history but is NOT the plan of
> record.
>
> -> Decision (user, 2026-07-16): Revised Approach approved as the plan of record. P0 is
> additionally promoted to a **production-bug investigation in its own right**, not merely the
> test-conversion blocker it was framed as. Rationale: if an external observer's engine RPCs
> during establishment can stall a single-peer session, a monitoring plugin polling during
> convergence can stall production peering. The test unblock is a side effect of that fix, not
> its purpose. P0 outcome (a) is therefore the expected one and (b) needs positive evidence.
>
> -> Constraint (2026-07-16, UNVERIFIED hypothesis, must be checked not assumed): P0 may share a
> root cause with the head-of-line blocking chain characterised in
> `plan/spec-fixit-firewall-concurrency-deadlock.md` (a synchronous dispatch path contending with
> a long-running operation that holds a lock). Nobody has read a producer linking them. Check it
> during P0 before spending the investigation twice; do not record it as a finding until cited.

The primitive-migration spec (`migrate-plugin-sleeps`, committed) converted 214 of the 305
`test/plugin` sleeps with the existing Layer 1/2 wait primitives. ~~**246 real `time.sleep()`
calls remain across `test/**/*.ci`** (91 in `test/plugin`, 155 in non-plugin dirs).~~
**Corrected 2026-08-03: 79 remain, 33 of them in `test/plugin`. See the Correction block.**

The original framing assumed each was left for lack of **new test-synchronization infrastructure**.
The audit disproved that: the signals mostly already exist. Each remaining sleep was left for a
**per-test reason** that only surfaces on attempting + running the conversion. Verification splits
hard by platform: 83/91 plugin sleeps are host-runnable on darwin; the rest (8 plugin + ~155
non-plugin) are `needs-linux` and verifiable ONLY via `./le qemu run command "./le qemu all-tests"` (which this
dev host does not run).

Scope confirmed by user (2026-07-14): "write a spec for the ones you deferred/kept so we can
also convert them." Genuinely-intentional sleeps (deliberate timers that ARE the test) stay,
documented.

**Added 2026-08-03 (bookkeeping audit): `test/web/commit-flow.wb`. CONVERTED 2026-08-07,
see the session note in the Implementation Summary.** Homed here from the chaos-reconnect
spec, which recorded it and could not own it. That spec is closed and gone; its row survives
in `plan/deferrals/fixit-chaos-reconnect-load-sensitive.md`, which now carries the closing
evidence. The test carried `option=timeout:value=45s` and two blind `action=wait:ms=1000`
steps. It took 36.8 seconds under a full run against 14.1 seconds standalone on 2026-07-30, a
slowdown of 2.6 times it did not survive. It was the same defect this spec exists to remove: a
wait on elapsed time where a wait on state belongs. The ratchet in `test/.ci-sleep-baseline`
counts `test/**/*.ci` only, so a `.wb` conversion moves no number, and the predecessor spec
already converted `rbac-web`, so the web suite was not new ground here.

## Revised Approach (plan of record, 2026-07-14)

1. ~~**P0 -- Establishment-blocking investigation (BLOCKS all establishment-observing conversions).**
   Root-cause why an external observer's engine RPCs during BGP establishment prevent a single-peer
   session from establishing (`bgp-redistribute-announce.ci`) while a 2-peer config
   (`redistribute-as112-announce.ci`) is unaffected. Producing code to read: the reactor connect /
   FSM goroutine vs the plugin-engine synchronous command-dispatch path (`ze-plugin-engine:*` RPC
   handling). Outcome is one of: (a) a real concurrency bug -> fix at source (high value; a
   monitoring plugin polling during convergence could stall production peering); (b) a documented
   config constraint + a conversion recipe that avoids it. Until P0 resolves, redistribute/rs/
   teardown-style conversions that must observe establishment are blocked.~~
   **P0 IS RESOLVED and blocks nothing. Superseded 2026-08-14, restating the phase note above
   where a reader meets the plan of record.** It was carved out to
   `spec-fixit-redistribute-establishment-stall`, which root-caused it: there is NO engine stall,
   and the symptom came from two test-harness defects. Outcome (b) is what landed, so the
   "monitoring plugin could stall production peering" premise is withdrawn and this spec carries
   no production-bug half. Every "P0-gated" and "P0-blocked" marker below is history. Do not read
   one as an open blocker.
2. **Bucket-by-bucket conversion, host-first.** For each host-runnable bucket, discover the recipe
   on ONE test, run it 3x to prove determinism (guard against false-positive passes -- verify the
   observer `main()` actually runs and the gating assertion actually fires), then apply to the rest
   of the bucket. Buckets, in tractability order: reject-fence (asserts after convergence, may not
   hit P0), bmp-receiver, teardown, show-l2tp, rs, redistribute (P0-gated).
3. **QEMU buckets are convert-but-cannot-verify-here.** fib-kernel (uses existing `show fib kernel`),
   firewall/traffic/policy/static/vpp/l2tp, external-warns. Convert mechanically where the signal is
   clear, but mark each done ONLY after a Linux/QEMU verification run -- never on darwin-skip alone.
4. **Ratchet per bucket**, deliberate timers documented + kept.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint: annotations. -->
- [ ] `docs/architecture/testing/ci-format.md` — `.ci` directive catalog; the engine-step executor is the Go-side extension target for a runner-level "wait for daemon stderr pattern" primitive (P2).
- [ ] `ai/rules/platform-linux.md` — QEMU integration is mandatory for linux-only code; the linux-only converted tests (firewall/traffic/policy/vpp/fib-kernel) are verified via `./le qemu run command "./le qemu all-tests"`, not the darwin host.
- [ ] `ai/rules/testing.md` — every converted behavior keeps its required functional test.

### RFC Summaries (MUST for protocol work)
- [ ] N/A — test infrastructure; no wire-protocol behavior changes.

**Key insights:**
- Three sleep shapes remain: (a) standalone driver scripts (no `ze_api` import) polling `daemon.ready` or shelling `nft`/`tc`/`ip route`; (b) waits for effects with no queryable state (~~fib-kernel async delivery,~~ **fib-kernel struck 2026-08-03: `show fib kernel` is queryable state; only the ARRIVAL TIME is unpredictable, which a bounded poll handles. See the Correction block.** external-plugin WARNs on the daemon's own stderr, rejected routes that never land); (c) genuine deliberate timers.
- The fix is per-shape infrastructure, not per-test cleverness. Each infra piece (P1..P8) unblocks a named bucket; P9 re-examines KEEP.

## Current Behavior (MANDATORY)

**Source files read (this + prior session, agent + direct):**
- [ ] `internal/component/bgp/reactor/reactor_api.go` — `FlushForwardPool` (:891) / `DrainPeerSync` (:922): the quiesce barrier is OUTBOUND-only and forward-pool-based.
  -> Constraint: redistribute UPDATEs and control messages (KEEPALIVE/ROUTE-REFRESH) bypass the updates-sent counter via the forward-pool send path (`reactor_notify.go`), so `quiesce()` blocks ~10s on the forward-pool barrier for them (P6).
- [ ] `internal/component/bgp/reactor/peer.go` — `DefaultReconnectMin = 5s` (:81): the reconnect backoff that makes establishment slow/variable in bgp-redistribute-* (P6).
- [ ] `internal/component/plugin/server/startup.go` — `WaitForStartupComplete` (:291), called before the `ze.ready.file` write (`cmd/ze/hub/main.go`): the daemon-readiness barrier a `wait_for_daemon_ready()` helper (P1) and the dataplane-programmed question (P3) build on.
  -> Decision (2026-07-28): A-1 is RESOLVED, and the answer is "yes for the plugin's own apply, no for what the plugin programs afterwards". `daemon.ready` is written only after every plugin startup phase settled (`startup.go` -> `signalStartupComplete`), and the policy plugin's `OnConfigure` (`internal/plugins/policyroute/register.go`) calls `applyPolicies` SYNCHRONOUSLY. But `applyPolicies` programs nftables FIRST (`firewall.ApplyAll`, `:200`) and the ip rules/auto routes SECOND (`rm.applyAll`, `:204`), so a wait must gate on whichever object the test actually reads -- gating on the nft table proves nothing about an `ip rule show` assertion. P3 therefore did NOT delete the lead-ins; it replaced each with a bounded poll on that test's own readback (`ze_api.wait_for_output`).
- [ ] `internal/plugins/fib/kernel/fibkernel.go` — `installed` map (the authoritative programmed set), fire-and-forget sysrib->fib-kernel delivery: ~~no end-to-end "FIB in sync" signal exists (P4)~~ **corrected 2026-08-03, see the Correction block: `show fib kernel` serves that map through `showInstalled` (`fib/kernel/backend.go`, `fib/kernel/register.go`), and the map is written only after the netlink program succeeds, so it IS the P4 signal.**
- [ ] `internal/plugins/traffic/netlink/ops_linux.go` / `backend_linux.go` — tc `Apply` is synchronous, run in-band in `OnConfigure`/`OnConfigApply`; `ListQdiscs` is a live readback (P3).
- [ ] `test/scripts/ze_api.py` — existing primitives; the home for `wait_for_daemon_ready` (P1) and a reject-fence helper (P5).
- [ ] `internal/test/runner/engine_steps.go` / `runner_exec.go` — the runner + engine-step executor; the home for a daemon-stderr-wait primitive (P2).

**Behavior to preserve:**
- Every converted test keeps its exact `expect=`/`reject=`/fatal assertions; only the WAIT mechanism changes.
- Deliberate-timer sleeps stay (concurrent-config-commit verify-hold, ddos rate pacing, keep-alive loops) — documented, not converted.
- No production behavior change beyond additive test-support surfaces (a helper, a runner primitive, an optional reflecting-`show`).

**Behavior to change:**
- Add the P1..P8 infrastructure and convert the sleeps each unblocks — user requested.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` observer or standalone driver calls a new deterministic-wait surface (P1 `wait_for_daemon_ready`, P2 daemon-stderr wait, P4 fib reflecting-`show`, P5 reject-fence helper) instead of `time.sleep`.

### Transformation Path
1. The new surface polls a real signal (the ready file, a reflecting `show`, a runner-observed stderr pattern, a sentinel route landing) until satisfied or a bounded timeout.
2. The test's assertion proceeds unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| driver/observer <-> daemon | existing ready file / `dispatch-command` / new reflecting-`show` | [ ] |
| runner <-> daemon stderr | new runner-level "wait for stderr pattern" primitive (P2) | [ ] |
| engine <-> fib-kernel (cross-process) | new reflecting-`show` or FIB quiescer fence (P4) | [ ] |

### Integration Points
- `test/scripts/ze_api.py` (P1, P5), `internal/test/runner/*` (P2), `internal/plugins/fib/kernel/*` (P4), `internal/component/bgp/reactor/*` (P6/P7). Additive; no existing behavior removed.

### Architectural Verification
- [ ] No bypassed layers (each wait polls the real effect it asserts).
- [ ] No unintended coupling (test-support surfaces are additive).
- [ ] No duplicated functionality (reuse `WaitForStartupComplete`/`ListQdiscs`/`installed` where they exist).
- [ ] Registration over hardcoding — reflecting-`show` commands register like any command.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `daemon.ready` (WaitForStartupComplete) already implies nft/tc `Apply` completed | tc `Apply` is in-band in OnConfigure (`ops_linux.go`) | P3 lead-ins can't just be deleted | QEMU: assert nft/tc state immediately after ready with no sleep | pending |
| A-2 | fib-kernel installs are observable via a `show fib`/`show rib` that can be added | `installed` map exists (`fibkernel.go`) | P4 needs the full cross-process quiescer instead | prototype a `show fib kernel installed <prefix>` reflecting query | pending |
| A-3 | A sentinel "always-accepted" route announced after a rejected one lands only after the reject decision is made (ordering) | opQueue/adj-rib-in FIFO per peer | reject-fence gives false green | P5 unit + a converted reject test 3x | pending |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | A reflecting-`show` or helper adds production surface area for test-only needs | review pushback | keep additive + test-scoped; reuse existing state (`installed`, `ListQdiscs`); gate behind existing commands where possible |
| R-2 | Linux-only conversions unverifiable on the dev host give false confidence | — | MANDATORY QEMU run per `platform-linux.md`; never mark a linux-only conversion done on darwin-skip alone |
| R-3 | reject-fence / RS-anchor patterns are subtly vacuous | converted negative test passes when the route wrongly landed | require a positive transition (sentinel lands / inbound route lands) before the negative assertion; per-test 3x |
| R-4 | FIB quiescer (P4 full form) is a large cross-process feature | scope creep | prefer the reflecting-`show` (A-2) first; escalate to the quiescer only if the show is insufficient; it may become its own sub-spec |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| driver calls `wait_for_daemon_ready()` | -> | `ze_api.wait_for_daemon_ready` (P1) | static/*.ci, policy-routes-show.ci (3x / QEMU) |
| `.ci` waits on a daemon stderr pattern | -> | runner stderr-wait primitive (P2) | as112/cos/ddos-external-warns/trafficusage/flowexport (QEMU) |
| observer waits for nft/tc programmed | -> | P3 (ready-implies-programmed or reflecting show) | firewall/policy/traffic (QEMU) |
| observer waits for a kernel FIB route | -> | fib reflecting-`show` / quiescer (P4) | fib-mpls-kernel/blackhole/srv6-kernel/table (QEMU) |
| observer asserts a rejected route | -> | reject-fence helper (P5) | prefix-filter-reject, aspath-*-reject, ... (3x) |

## Acceptance Criteria

| AC ID | Piece | Expected Behavior |
|-------|-------|-------------------|
| AC-1 | **P1 wait_for_daemon_ready** | importable helper (works in standalone drivers with no `API()` and in observers); converts the `for..: if exists('daemon.ready'): break; sleep(0.05)` polls + lead-ins in static/firewall/policy/traffic drivers, policy-routes-show:18, ddos-detect-*:46/:43; each converted test green (host or QEMU) |
| AC-2 | **P2 daemon-stderr wait** | a runner-level primitive to wait until the spawned daemon's stderr matches a pattern (bounded); converts the external-warn hold-open sleeps (as112, cos, ddos-detect-external-warns, trafficusage, flowexport-external-refuses); QEMU-verified |
| AC-3 | **P3 dataplane-programmed** | resolve A-1: if `daemon.ready` implies nft/tc programmed, delete the lead-ins; else add a reflecting readiness. Converts firewall (48), policy (13), traffic (16 netlink); QEMU-verified |
| AC-4 | **P4 fib reflecting-show / quiescer** | a queryable signal that a kernel-FIB route is installed; converts fib-mpls-kernel, fib-blackhole, fib-srv6-kernel(3), fib-table + non-plugin static/vpp kernel waits; QEMU-verified. **Corrected 2026-08-03: the signal already exists (`show fib kernel`), so this AC is a conversion of the named tests to a bounded poll on it, NOT the construction of a new surface. See the Correction block** |
| AC-5 | **P5 reject-fence** | a documented sentinel-route pattern (+ helper) proving a reject decision completed without a positive edge; converts prefix-filter-reject, prefix-filter-chain-order, community-match-reject, aspath-filter-reject, aspath-length-reject, role-otc-ingress-reject, rpki-validate-reject, rpki-maxlength, rpki-as-set, rfc7606-withdraw; each 3x |
| AC-6 | **P6 redistribute/control-message signal** | a fast outbound-sent signal that redistribute UPDATEs and KEEPALIVE/ROUTE-REFRESH increment (or a forward-pool barrier that returns promptly); converts bgp-redistribute-* (5), api-raw, api-route-refresh; each 3x |
| AC-7 | **P7 RS inbound-anchor** | anchor on the RS's inbound view before quiescing outbound; converts bgp-rs-* (6), remove-private-as-export, remove-private-as-replace-peer; each 3x |
| AC-8 | **P8 QEMU verification** | every linux-only conversion (P2/P3/P4 + ospf/l2tp linux tests) passes `./le qemu run command "./le qemu all-tests"` |
| AC-9 | **P9 KEEP re-examination + ratchet** | peer-driver pacing (l2tp handshake) assessed for determinism; standalone stderr backoffs moved to `select` where clean; genuine deliberate timers documented and kept; `test/.ci-sleep-baseline` lowered as each piece lands (target: only documented deliberate timers remain) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path | Test |
|---|-----------|------|------|
| 1 | writes a standalone driver test that waits for boot | `wait_for_daemon_ready()` (no `API()` needed) | static/static-boot-apply.ci |
| 2 | writes a test asserting an external plugin logged a WARN | runner waits for the daemon stderr pattern | as112-external-refuses.ci (QEMU) |
| 3 | writes a test asserting a kernel FIB route | reflecting `show fib` poll | fib-blackhole.ci (QEMU) |
| 4 | writes a test asserting a route was rejected | announce a sentinel; wait for it to land; then assert the rejected prefix absent | prefix-filter-reject.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| P2 stderr-wait parse/exec | `internal/test/runner/engine_steps_test.go` | the new directive parses + matches a stderr pattern with a bounded timeout | |
| P4 reflecting-show | `internal/plugins/fib/kernel/*_test.go` | the query reports an installed prefix and absence after withdraw | |
| P6 outbound-sent signal | `internal/component/bgp/reactor/*_test.go` | redistribute/control-message increments the signal | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Notes |
|-------|-------|-------|
| stderr-wait timeout | bounded | timeout error names the pattern; no unbounded loop |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| each converted `.ci` | `test/**` | its own scenario, deterministic wait | per-piece, 3x host or QEMU |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Notes |
|----------|-------|
| N/A | test infrastructure |

### Future (if deferring any tests)
- Each piece P1..P8 is independently shippable and MAY be split into its own sub-spec (P4 fib quiescer especially — it is the original "Layer 3" cross-process feature).

## Files to Modify

Infra (production/test-support) files, each additive:
- `test/scripts/ze_api.py` — P1 `wait_for_daemon_ready`, P5 reject-fence helper.
- `internal/test/runner/engine_steps.go`, `internal/test/runner/runner_exec.go` — P2 daemon-stderr-wait primitive.
- `internal/plugins/fib/kernel/` (backend + register) — P4 reflecting-`show` (or the FIB quiescer).
- `internal/component/bgp/reactor/` — P6 outbound-sent signal, P7 RS inbound-anchor support.
- `docs/architecture/testing/ci-format.md`, `ai/rules/testing.md` — document the new surfaces (P2/P4/P5) and the reject-fence pattern.
- The converted `.ci` files across `test/plugin`, `test/firewall`, `test/l2tp`, `test/traffic`, `test/static`, `test/policy`, `test/vpp`, `test/flow-export`, `test/ospf`, `test/encode`, `test/pppoe`, `test/install`, `test/reload`, `test/ui`.
- `test/.ci-sleep-baseline` — lowered as each piece lands.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infra docs | yes | `docs/architecture/testing/ci-format.md`, `ai/rules/testing.md` |
| Discovery updates | yes (new primitives/gates) | `ai/INDEX.md` per `ai/rules/repo-maintenance.md` |
| QEMU verification | yes | `./le qemu run command "./le qemu all-tests"` for linux-only conversions |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File |
|---|----------|----------|------|
| 8 | Plugin SDK/test-support changed? | Yes (new observer/runner surfaces) | `ai/rules/testing.md`, `docs/functional-tests.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` |

## Files to Create
- Possibly `internal/test/runner/engine_predicate.go`-style helpers if a file exceeds size limits; per-piece unit-test files as needed. <!-- doc-links: ignore (hypothetical helper name, never created) -->

## Implementation Steps

### /implement Stage Mapping
| Stage | Section |
|-------|---------|
| Audit | deferrals.md rows + this spec's AC table |
| Implement | P1..P9, each piece as a phase |
| Verify | 3x host per converted test + `./le qemu run command "./le qemu all-tests"` for linux-only |
| Close | ratchet lowered per piece; two-commit closure |

### Implementation Phases
1. **P1 wait_for_daemon_ready** (smallest, host-verifiable in part) — helper + convert static/policy-routes-show/ddos-ready polls.
2. **P5 reject-fence** — host-verifiable; pattern + helper + convert reject/negative tests.
3. **P6 redistribute/control-message signal** — host-verifiable; convert bgp-redistribute-*, api-raw, api-route-refresh.
4. **P7 RS inbound-anchor** — host-verifiable; convert bgp-rs-*, remove-private-as-*.
5. **P3 dataplane-programmed** — resolve A-1; convert firewall/policy/traffic; QEMU.
6. **P4 fib reflecting-show / quiescer** — convert fib-kernel + static/vpp kernel; QEMU (may split to sub-spec).
7. **P2 daemon-stderr-wait** — runner primitive; convert external-warn; QEMU.
8. **P8 QEMU verification** — full linux-only pass.
9. **P9 KEEP re-examination + final ratchet.**

### Critical Review Checklist (/implement stage 6)
| Check | For this spec |
|-------|---------------|
| Assertions preserved | every converted test keeps its `expect=`/`reject=`/fatal checks |
| Non-vacuous | reject-fence + RS-anchor require a positive transition first (R-3) |
| QEMU-verified | no linux-only conversion claimed done on darwin-skip alone (R-2) |
| Additive-only | infra surfaces reuse existing state; no production behavior removed (R-1) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification |
|-------------|--------------|
| Each piece's tests converted | `grep` sleep gone; 3x green host or QEMU |
| Ratchet lowered | `cat test/.ci-sleep-baseline`; `verify_wiring_docs.py` green |
| No regressions | `bin/ze-test bgp plugin --all` + affected suites + QEMU |

### Security Review Checklist (/implement stage 11)
| Check | Notes |
|-------|-------|
| Input validation | new predicates/patterns bounded (timeout/attempts); test-only |
| Resource exhaustion | no unbounded loop; runner stderr-wait bounded |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 false (ready != programmed) | P3: add reflecting readiness |
| A-2 false (no usable show) | P4: escalate to the FIB quiescer sub-spec |
| Converted test flakes | investigate race (do not re-add sleep) |
| 3 fix attempts fail | mark DEFER in deferrals.md, move on, report |

## Mistake Log
### Wrong Assumptions
| Assumed | True | Discovered | Impact |
|---------|------|------------|--------|
| P6: redistribute UPDATEs bypass the updates-sent counter (per the deferral comment on bgp-redistribute-announce.ci) | `reactor_notify.go` calls `peer.IncrUpdatesSent()` for EVERY sent UPDATE incl. redistribute/forward-pool sends; `show bgp peer <n> detail` exposes `updates-sent`/`eor-sent`/`state` (`internal/component/bgp/plugins/cmd/peer/peer.go`) | 2026-07-14 audit (read producer + `redistribute-as112-announce.ci` uses exactly this signal) | P6 needs NO new outbound signal; the counter already exists |
| P4: a fib reflecting-`show` must be built | `show fib kernel` already exists and returns the `installed` map as `[{prefix,next-hop}]` JSON (`fib/kernel/register.go`, `backend.go`) | 2026-07-14 audit | P4 needs NO new production code; fib-kernel conversion is a `dispatch_until('show fib kernel', ...)` poll (still QEMU-gated) |
| "Build the signal, then conversion is mechanical" (Core Insight) | The signals mostly already exist; the real blocker is an engine BEHAVIOR (below), so conversion is NOT mechanical | 2026-07-14 redistribute investigation | Spec framing (P1..P8 = build infra) is largely wrong; the work is per-test root-causing, not infra-building |
### Failed Approaches
| Approach | Why | Replacement |
|----------|-----|-------------|
| Convert bgp-redistribute-announce with the proven `redistribute-as112-announce` poll (established -> eor-sent -> updates-sent) | The single-peer session NEVER establishes (`connections-established: 0`) when the observer is ACTIVE (dispatch/quiesce/show-poll OR even pure `wait_for_event` callback reads). Only the original blind `time.sleep` (observer fully idle, not reading its connection) lets it establish. `quiesce()` after emit blocks the full 10s ze-system bound ok=False (forward pool can't drain to an unestablished peer). Bisected: NOT peer-count (single-peer `redistribute-as112-announce` works), NOT peer-name, NOT explicit-vs-auto orchestrator, NOT plain-BGP (`nexthop-self.ci` polls + works). The differentiator is the `import fakeredist` redistribute config specifically vs `import as112` | **P0 CONCLUSION: redistribute bucket is BLOCKED, needs its own engine-investigation sub-spec.** The stall is a real, reproducible, config-specific interaction between observer activity and the redistribute late-join-replay-on-establish path (`redistribute_egress/replay.go` fires ReplayRequest on peer establishment). Root-causing it is an engine concurrency investigation, not a test conversion. Deferred to a dedicated sub-spec; other buckets proceed |
### Escalation Candidates
| Mistake | Frequency | Rule |
|---------|-----------|------|

## Design Insights
<!-- LIVE -->
- The remaining sleeps are an infrastructure gap, not a per-test one. Grouping by infra piece (P1..P8) makes each independently shippable and lets host-verifiable pieces (P1/P5/P6/P7) land before the QEMU-gated ones (P2/P3/P4).
- **AUDIT REVISION (2026-07-14):** The spec's core premise ("build the signal, conversion is mechanical") is largely wrong. The signals mostly ALREADY exist (`updates-sent` counter for P6, `show fib kernel` for P4). The real remaining difficulty is a reproducible engine BEHAVIOR: an external observer that issues engine RPCs (dispatch/quiesce/show-poll) during BGP establishment can prevent a single-peer session from coming up (`connections-established` stays 0), so deterministic waits that must observe establishment cannot be added without either (a) root-causing that behavior or (b) proving it absent per-config. `redistribute-as112-announce.ci` (2-peer) is unaffected, so it is config-dependent and subtle.
- **Host/QEMU split (verified 2026-07-14):** Of 91 remaining `test/plugin` sleeps, 83 are NOT `option=needs-linux` (host-runnable on darwin); only 8 (ddos/flowexport/trafficusage external-warns) need linux. Of 155 non-plugin sleeps, most are linux-only (firewall 48, l2tp 24, static 17, traffic 16, vpp 14, policy 13) and can only be verified via retired `ze-qemu-needs-linux-test` (current: `./le qemu run command "./le qemu all-tests"`), which this darwin host does not run.
- **Reject-fence via injected counted message (PROVEN 2026-07-14, rfc7606-withdraw):** to prove an UNCOUNTED engine action was processed (a malformed treat-as-withdraw UPDATE returns in `session_read.go` `processMessage` at 163-171 BEFORE the `onMessageReceived` callback that drives `IncrUpdatesReceived` at `reactor_notify.go`), inject a well-formed COUNTED message (an empty EOR) right after it in the SAME `seq` group on the peer connection. ze processes a connection in TCP order on one session-read goroutine, so the counted message's `updates-received` increment is a deterministic fence proving the uncounted action already ran. The observer waits on `updates-received` instead of sleeping. This realizes the "reject-fence sentinel" decision (Key Design Decisions) concretely, WITHOUT any production change. It applies to any shape where the peer sends the uncounted message on a connection the test drives.
- **Infra-gated shape (no pollable signal, injected-message technique does NOT apply):** reload-listener-rejected, as112-external-refuses, cos-external-warns prove a rejection/warning observable ONLY as a relayed-stderr line (checked runner-side via `expect=stderr:contains=`), driven by a standalone `wait.py` with no ze_api connection to poll and no peer connection to inject a fence on. Their correctness is ALREADY timing-independent (the runner's stderr expect waits up to the test timeout); only the secondary observer-side check keeps a sleep, and making it deterministic needs a NEW production signal (a plugin-visible reload-processed / external-plugin-exit event). These are the genuine P5 reject-fence INFRA cases.
- **Session conversions landed 2026-07-14 (baseline 226 -> 204):** rfc7606-withdraw (EOR-fence, 9e2bb1821); remove-private-as x2 (receiver `updates-sent` over the RS fast path per `reactor_notify.go`, 16cd29d00); l2tp teardown/show x10 (raw-L2TP driver: pre-ICRQ `sleep(0.05)` -> ICRQ retransmit-until-ICRP(AVP 14) retry loop mirroring the file's SCCRQ loop, plus dropped redundant pre-SCCRQ settle sleeps; 15 sleeps; 8cf13f1c1); cli-commit x2 + rbac-web (hand-rolled readiness poll loops -> `ze_api.wait_until`, ratchet-exempt; 4 sleeps; de83367f0). 22 sleeps removed, each verified 3x+ standalone and concurrently against the prebuilt binary (the tree does not build: a concurrent session broke `internal/component/iface`, so commits used `--unverified`/`--stale-index-ok`).
- **Remaining 204, by category + owner:** QEMU-gated needs-linux bulk (~150; requires `./le qemu run command "./le qemu all-tests"`, unavailable on darwin); P0-blocked redistribute (7 tests + api-raw/route-refresh) -> `plan/spec-fixit-redistribute-establishment-stall.md`; infra-gated reject-fence cases (reload-listener-rejected, as112/cos-external) -> `plan/spec-fixit-reject-fence-observability.md`; a deliberate-window timer (concurrent-config-commit `sleep(3)` IS the concurrency race window, keep); raw-protocol pacing left as-is (bfd-* 0.05 BFD control loop, exabgp-bridge-sdk startup with no clear readiness signal). Blind-converting the QEMU bulk without a Linux env is explicitly NOT done: unverified sleep conversions would violate this spec's own verification principle. <!-- doc-links: ignore (spec closed and removed) -->

## Core Insight

**REVISED 2026-07-14 (post-audit).** The original insight below was wrong; the corrected one supersedes it.

- **ORIGINAL (wrong):** "what remains needs new observability (a fib reflecting-show, a fast
  outbound signal, ...). Build the signal, then the conversion is mechanical."
- **CORRECTED:** The observability signals **mostly already exist** -- `updates-sent`/`eor-sent`/
  `state` via `show bgp peer <n> detail`, `show fib kernel` for installed FIB routes, adj-rib-in
  queries for inbound routes. The 246 remaining sleeps were NOT left unconverted for lack of a
  signal. Each was left because of a **per-test reason** that only surfaces when you actually
  attempt the conversion and run it. The dominant reason (found in the redistribute bucket) is an **engine
  behavior**: an external observer that issues engine RPCs during BGP establishment can prevent a
  single-peer session from coming up. So the real work is **per-test investigation + a small
  number of shared root-causes**, not infra-building. Conversion is NOT mechanical.

## Key Design Decisions
| Decision | Alternatives | Rationale |
|----------|-------------|-----------|
| Umbrella spec, pieces independently shippable | one giant spec / many tiny specs | each piece unblocks a named bucket and can ship + verify alone; P4 may split out |
| Prefer reflecting-`show` over a full FIB quiescer (P4) | build the cross-process quiescer first | a read-only query reusing `installed` is far smaller; escalate only if insufficient |
| Reject-fence sentinel over an event subscription | subscribe to rejected-route events | a sentinel needs no new event plumbing and proves ordering deterministically |

## Known Limitations
- Genuine deliberate-timer sleeps (the delay IS the test) are kept and documented, not converted.
- Linux-only conversions are only verified in QEMU, not on the darwin dev host.

## RFC Documentation
N/A — no protocol behavior.

## Implementation Summary
### What Was Implemented
Host-verifiable blind-sleep conversions (each verified 3x+ standalone and concurrently, no flake; baseline ratcheted down per commit):
- Reject bucket (6): prefix-filter-reject, community-match-reject, aspath-filter-reject, aspath-length-reject, role-otc-ingress-reject, prefix-filter-chain-order — wait `updates-received >= 1` then `quiesce()`.
- RS bucket (6): bgp-rs-{mod-copy,reactor-fastpath,reactor-fastpath-fallback,fastpath-ibgp-identity,fastpath-ebgp-shared,replaying-gate} — wait all-peers `eor-sent` via `show bgp`.
- BMP (1 file/4 sleeps): bmp-receiver-messages — poll `show bmp peers` / `show bmp sessions` state.
- RPKI (4): rpki-{validate-reject,maxlength,as-set} wait VRP sync + `updates-received`; rpki-aspa-disabled dropped a redundant lead-in sleep (test-relax token).
- rfc7606-withdraw (1, commit 9e2bb1821): EOR-fence — peer injects a well-formed EOR after the malformed UPDATE; observer waits `updates-received` (the malformed is uncounted). See Design Insights.
- remove-private-as-export + remove-private-as-replace-peer (2, commit 16cd29d00): observer waits receiver-peer `updates-sent` (route forwarded over RS fast path) before shutdown.

Baseline: 246 -> 223. Remaining 223 are categorized in Design Insights (QEMU-gated bulk, P0-blocked redistribute, infra-gated reject-fence cases, deliberate-window timers, raw-protocol pacing, already-deterministic non-api poll loops). No further CLEAN host-verifiable blind sleeps remain to convert on this darwin host.

### Session 2026-07-28 (dd843d81) -- P1 shipped, P3 done for policy + firewall

**The blocker was the HOST, and it is gone.** Every "cannot verify here" note above
says *darwin*. This session ran on Linux with passwordless sudo, `nft`, `tc`,
`setcap` and QEMU present, so `ZE_NETNS_SUITES` (`internal/le/integration/gates.go` --
firewall, policy, ospf, ospfv3) runs NATIVELY, each test in its own netns, with a
host-safety check on the nft table set. That is a real verification vehicle for
exactly the buckets P1 and P3 name. Nothing in the analysis below rests on a skip.

**P1 (AC-1) -- `wait_for_daemon_ready`, shipped.** `test/scripts/ze_api.py`.
Returns the pid rather than just a bool, because every caller reads `daemon.pid`
immediately afterwards to SIGTERM the daemon, and doing that as a separate step
re-opens the race the helper exists to close: ze creates `daemon.pid` before
writing it, so `os.path.exists` could be followed by `int("")`. The pid parse is
therefore part of the wait predicate.

**P3 (AC-3) -- `wait_for_output`, shipped.** Same file. The kernel-readback
counterpart to `dispatch_until`, for standalone drivers with no API connection:
run a readback command until a predicate holds, treating a non-zero exit as "not
yet" and still consulting the predicate with `""` so ABSENCE waits work. On
exhaustion it returns the LAST output instead of raising, so each driver's own
per-phase assertion still produces the failure message.

**A-1 is RESOLVED and the answer is subtler than the assumption.** `daemon.ready`
IS a real barrier -- it is written only after every plugin startup phase settled
(`startup.go`, `cmd/ze/hub/main.go`) and the policy plugin's
`OnConfigure` applies synchronously. But `applyPolicies`
(`internal/plugins/policyroute/register.go`) programs nftables FIRST
(`:200`) and ip rules/auto routes SECOND. So the lead-ins could NOT
simply be deleted, and a wait on the nft table would not have covered an
`ip rule show` assertion. Each converted driver waits on the object IT reads.

**Converted, 20 sleeps removed, baseline 100 -> 80:**
- `test/policy/*.ci` -- all six, 12 -> 0. 3x 6/6, and faster (7.6s -> ~4.6s).
- `test/firewall/*.ci` -- 8 -> 0 across firewall-cli-show, firewall-set-element-timeout,
  ddos-local-withdraw. 3x 23/23.
- `firewall-cli-show` could not be deleted either: the SSH CLI server binds separately
  from `OnConfigure`, so `daemon.ready` does not cover it. It now waits on
  127.0.0.1:2222 actually accepting.
- `policy-reload`'s `wait_rules` now delegates to the shared helper while keeping its
  own 12s deadline, derived per call from the budget still remaining.

**Non-vacuity is demonstrated, not asserted.** Three source mutations, each
built and run:
| Mutation | Expected | Observed |
|----------|----------|----------|
| skip `firewall.ApplyAll` (register.go) | every policy test fails | 6/6 fail |
| skip `rm.applyAll` (register.go) -- ip rules only | ONLY the two reading `ip rule show` fail | exactly 002, 005 |
| drop set elements (nft/lower_linux.go) | only the set-element test fails | exactly firewall 009 |
Plus three mutations of the helpers themselves against their new unit tests in
`test/scripts/ze_api_test.py` (wired into `go test` via
`internal/le/`), each red.

**Found while verifying, deferred:** `./le qemu netns-test` reports
`ddos-local-withdraw` as failing, and that is the TARGET's defect. It depends on
`$(ZEBIN_ZE)`, the production binary, which `internal/le/integration/gates.go` says has
"neither zetest nor ze_test" -- so the `ddos/fake` node the test configures does
not exist and the daemon dies with `unknown field in ddos: fake`. Against a
zetest-tagged DUT the same test passes in 653ms. The target also cannot run
on-session at all (`bin/ze-<sid>` vs bare-name lookup). Both ->
`plan/future/spec-fixit-netns-test-dut-tags.md`.

**Scope boundary observed:** `test/traffic` and `test/vpp` blind holds are the
"backgrounded ze has no ZE_READY_FILE to poll" shape, which is
`plan/future/spec-fixit-sleeps-qemu-bulk.md`'s AC-2/AC-3/AC-6, not this spec's. Left
alone deliberately rather than done under the wrong spec.

**Still open here:** P2 (runner stderr-wait), P4 (fib), P6/P7 (already reported
done above), P8, P9, and the `test/plugin` remainder.

### Session 2026-08-07 -- `test/web/commit-flow.wb` converted

The deferral row homed here on 2026-08-03 is closed
(`plan/deferrals/fixit-chaos-reconnect-load-sensitive.md`). Both
`action=wait:ms=1000` steps are gone and `option=timeout` is 30s.

**The second wait hid a vacuous assertion, and that is the finding.** Discard
answers with an HX-Redirect (`handleConfigDiscard`,
`internal/component/web/handler_config_commit.go`), so the browser leaves the
page. The pending change then leaves its DOM whether the server discarded it or
not, and `expect=html:not-contains=` was true on the page it landed on. Stubbing
`EditorManager.Discard` (`internal/component/web/editor.go`) to `return nil` left
the old test PASSING. The migrated test polls the server's own readback and FAILS
against the same stub. This is R-3 (a subtly vacuous negative) caught in the web
suite rather than in the reject bucket.

**New primitive: `action=wait-until:path=<p>:contains=<text>`**
(`Browser.WaitUntil`, `internal/component/web/testing/runner.go`). It re-opens a
path until the served HTML carries the text, bounded by the `expectDeadline`
already used by every browser retry. It is the `.wb` counterpart of
`ze_api.wait_for_output` from P3: a readback poll for a standalone driver with no
push signal. The first wait needed no primitive at all, because a positive
`expect=` already polls the DOM (`retryPositive`,
`internal/component/web/testing/expect.go`).

**Two defects found while verifying, neither blocking this conversion, both
unhomed:**
- `option=timeout` is INERT in `.wb`. `WBTestCase.Timeout` is written by
  `parseWBOption` and read by nothing (`gopls references`: two hits, both inside
  `parser.go`), and `zeTestRunWebTest` applies no wall-clock bound. A `.wb` test's
  only real bounds are `agentTimeout` (30s per browser command) and
  `expectDeadline` (15s per retried assertion). Wire it with
  `ParallelTimeoutHeadroom`, or delete the option: the doc now states the truth
  either way.
- Concurrent `ze-test web` PROCESSES share browser sessions. The session name is
  `test.Nick` (`zeTestRunWebTest`), which is derived from the file path, so two
  runs of the same test in one checkout drive one browser page and stomp each
  other. This makes `internal/le/stressrepro/run.go` unusable against the web suite,
  and it is a live hazard for two agents in a shared checkout.
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

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

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| remaining convertible sleeps eliminated | ratchet + functional | (fill per piece) |
| linux-only conversions verified | QEMU | (fill: ./le qemu run command "./le qemu all-tests") |
| only deliberate timers remain | audit | (fill) |

## Checklist

Migration-adapted TDD (each converted `.ci` IS its own functional test; infra pieces have unit tests):
- [ ] Tests written — infra pieces (P2/P4/P6) get unit tests; each converted `.ci` keeps its exact assertions.
- [ ] Tests FAIL — infra unit tests are red-first (TDD); a converted `.ci` that FAILS surfaces a real race, fixed at the source (never a re-added sleep).
- [ ] Tests PASS — each converted test 3x green (host) or via `./le qemu run command "./le qemu all-tests"` (linux-only).
- [ ] `./le verify current mode full`, the affected native functional suites, and QEMU are green before each piece's commit.

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

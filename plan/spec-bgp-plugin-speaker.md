# Spec: bgp-plugin-speaker

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-20 |

## Task

A minimal, independent Python BGP speaker for interop-style testing, built like ExaBGP's
process model: a **fixed engine** (the plumbing) plus a **per-test plugin** (the only code
that changes). It exists to catch the class of bug that `ze-test peer` cannot -- a wire
output that a strict, INDEPENDENT peer rejects -- without needing a full Docker daemon.

Motivating case: ze's route-server replay emitted NEXT_HOP twice (RFC 7606 Section 3(g)); the
entire `.ci` suite stayed green because `ze-test peer` asserts only the bytes it was told to
expect. FRR caught it; nothing in-tree did. This speaker is the in-tree, independent check.

### Owner-specified design (verbatim intent)
- **Minimal BGP speaker.** iBGP session, per-instance router-id (ExaBGP-style), so multiple
  engines never collide.
- **One engine per test.** Each test runs its own engine instance, no shared state.
- **Minimal parsing.** Decode only what a check needs.
- **No validation except what the test names.** The engine validates nothing on its own; each
  test plugin runs ONLY its own check and NAMES itself in any failure.
- **Tests are plugins, dynamically loaded.** `importlib` loads a per-test module. The plumbing
  is identical for every test; only the plugin changes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the interop harness and its sidecars
  → Constraint: the speaker is a new sidecar, started like the injector (by file presence).
- [ ] `internal/test/peer/message.go` - `ze-test peer`'s OPEN/KEEPALIVE wire encoding
  → Constraint: the engine mirrors this framing; it is not novel wire, just independent code.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - OPEN/KEEPALIVE/UPDATE framing (Section 4)
- [ ] `rfc/short/rfc7606.md` - Section 3(g) duplicate attributes (the first plugin's subject)

**Key insight:** the engine reimplements only enough wire to receive; independence from ze's
Go code is the point, so it must NOT import or mirror ze's validators.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/interop.py` - `Scenario.setup` starts sidecars by config-file presence
  (`inject.msg` -> injector, `rpki-server` -> RPKI, `frr.conf` -> FRR). The speaker is added
  the same way.
- [ ] `internal/test/peer/message.go:94` - `KeepaliveMsg`; `:104` `DefaultRouteMsg` shows the
  UPDATE framing the engine decodes.

**Behavior to preserve:**
- Existing sidecars (injector, RPKI, BMP) and daemon classes (FRR/BIRD/GoBGP) unchanged.

**Behavior to add:**
- A `Speaker` sidecar + helper class; a fixed engine that loads a per-test plugin.

## Data Flow (MANDATORY)

### Entry Point
- The engine process, started per scenario, dialing ze on port 179.

### Transformation Path
1. Engine dials ze, sends OPEN (iBGP, router-id), exchanges KEEPALIVE -> Established.
2. Engine reads each message; UPDATE bodies are decoded minimally into an `Update`.
3. Engine calls the dynamically-loaded plugin's `on_update(update, ctx)`.
4. On session end, `on_end(ctx)`; results written to a file; exit code reflects `ctx.fail`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze ↔ engine | TCP BGP session | [ ] |
| engine ↔ plugin | `importlib` dynamic load; `on_update`/`on_end` hooks | [ ] |
| engine ↔ check.py | results file (pass/fail + messages) | [ ] |

### Integration Points
- `test/interop/interop.py` `Scenario.setup` (new sidecar), a `Speaker` helper class.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| engine loads a plugin and receives an UPDATE | → | `engine.dispatch` -> `plugin.on_update` | `test_engine.py::test_dynamic_load_optional_on_end` (PASS) |
| ze replays a duplicate-NEXT_HOP UPDATE | → | engine + `no_duplicate_attribute` plugin | scenario 48: reverted -> RED, fixed -> GREEN (PROVEN) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-1 | engine loads a plugin by path via importlib | the plugin's `on_update` is called for each received UPDATE | MET (`test_dynamic_load_optional_on_end`) |
| AC-2 | `no-duplicate-attribute` plugin, an UPDATE with NEXT_HOP twice | `ctx.fail` fires, naming the test; engine exits non-zero | MET (unit `test_plugin_flags_duplicate_next_hop` + LIVE scenario 48 reverted = RED) |
| AC-3 | same plugin, a well-formed UPDATE | no failure; engine exits zero | MET (unit `test_plugin_passes_clean_update` + LIVE scenario 48 fixed = GREEN) |
| AC-4 | engine against a real ze whose replay emits a duplicate NEXT_HOP (dedup fix reverted) | the scenario is RED; with the fix, GREEN | MET (scenario 48: reverted `type 3 appears more than once` RED; fixed one NEXT_HOP GREEN) |
| AC-5 | two engines in one run, different router-ids | both establish, neither collides | MET (unit `test_open_message_carries_per_instance_router_id` + LIVE scenario 49: two speakers at 172.30.0.10/172.30.0.11 both reach Established, GREEN) |
| AC-6 | a plugin defines only `NAME` + `on_update` (no `on_end`) | engine runs it without error | MET (`test_dynamic_load_optional_on_end`) |

**All acceptance criteria are MET.** interop.py now starts one or two speaker sidecars
(`speaker-args` / `speaker2-args`, at `SPEAKER_IP` / `SPEAKER2_IP`); scenario 49
(`49-speaker-two-instance`) runs two engines with distinct router-ids and asserts both establish.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | ze establishes an iBGP session with a minimal OPEN (MP IPv4-unicast + 4-octet-ASN) | `ze-test peer` does the same (message.go) | no session, no test | the interop scenario establishing | **confirmed** -- scenarios `48-rfc7606-speaker-dup-attr` and `49-speaker-two-instance` exist and establish |
| A-2 | ze's replay reaches the speaker as a peer | route server forwards to all clients | no UPDATE to check | the scenario receiving a route | **confirmed** -- by the same two scenarios; `test_plugin_flags_duplicate_next_hop` is what reads the received UPDATE |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|-----------|
| R-1 | the hand-rolled engine has its own decode bug -> false result | unit fixtures disagree with hand-computed bytes | red/green unit fixtures per plugin; keep decode minimal |
| R-2 | a plugin over-reaches into a broad buggy validator | review flags scope creep | one check per plugin, each unit-tested in isolation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_decode_update_sections` | `test/interop/speaker/test_engine.py` | minimal UPDATE decode | |
| `test_plugin_flags_duplicate_next_hop` | same | AC-2 | |
| `test_plugin_passes_clean_update` | same | AC-3 | |
| `test_dynamic_load` | same | AC-1/AC-6 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| ~~`bgp-speaker-no-dup-attr.ci`~~ | ~~`test/plugin/bgp-speaker-no-dup-attr.ci`~~ | INFEASIBLE for this bug -- see finding below. The dup-NEXT_HOP lives in the wire-mode re-encode path (`reactor_api_batch.go` `buildWireModeUpdate`), which only fires when Ze FORWARDS/REPLAYS a route to a DISTINCT receiving peer. A `.ci` test cannot use multiple peer IPs (all 127.0.0.1) -- this is the documented limitation at `test/plugin/adj-rib-in-replay-on-peerup.ci:4-5`, and is exactly why the bug escaped the `.ci` suite originally. The interop scenario (distinct IPs) is the correct home; it provides STRONGER coverage than this `.ci` could. | infeasible (finding) |
| speaker interop scenario (AC-4) | `test/interop/scenarios/48-rfc7606-speaker-dup-attr/` | the engine dials ze in the Docker harness, is replayed the injector's route through the route-server re-encode path, and its `no-duplicate-attribute` plugin fails the scenario when ze emits NEXT_HOP twice. Proven RED when the `buildWireModeUpdate` de-duplication is reverted. | |

> **Finding (code-verified):** a `.ci` proof for this specific bug is infeasible, not skipped.
> The wire-mode dup-NEXT_HOP is produced by `buildWireModeUpdate` (reached via
> `AnnounceNLRIBatch`/`SendRoutes` -> `buildBatchAnnounceUpdate` -> `buildWireModeUpdate`,
> `internal/component/bgp/reactor/reactor_api_batch.go:341,405`), which re-encodes a
> received attribute block. That path only runs when Ze forwards/replays a route to a
> SECOND peer. The `.ci` harness is single-IP (`adj-rib-in-replay-on-peerup.ci:4-5`:
> "`.ci` tests cannot use multiple peer IPs (all 127.0.0.1)"), so no `.ci` peer can be
> the distinct receiver the bug needs. The Docker interop harness gives every peer its own
> IP, so scenario 48 (this AC-4) is the only place the live bug manifests -- and it is a
> stronger proof than the planned `.ci`, since the receiver is a real independent peer.
> The engine's decode + dup-attribute discrimination are covered by the unit tests
> (`test/interop/speaker/test_engine.py`, red/green fixtures).

## RFC Documentation

Add `// RFC 4271 Section 4.2/4.3` references in the engine's framing; the first plugin cites
`RFC 7606 Section 3(g)` for the duplicate-attribute rule it enforces.

## Files to Modify
- `test/interop/interop.py` - speaker-sidecar support + `Speaker` helper class
- `docs/architecture/testing/interop.md`, `ai/INDEX.md` - discovery

## Files to Create
- `test/interop/speaker/engine.py`
- `test/interop/speaker/plugins/no_duplicate_attribute.py`
- `test/interop/speaker/test_engine.py`
- an interop scenario using the speaker sidecar

## Implementation Steps

1. Engine wire helpers (OPEN/KEEPALIVE/UPDATE encode+decode), TDD with `test_engine.py`.
2. Dynamic plugin load + dispatch; the `Update`/`ctx` contracts.
3. First plugin `no_duplicate_attribute`; red/green unit fixtures.
4. Interop sidecar wiring + scenario; prove RED when the reactor dedup is reverted.
5. Docs + discovery.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 demonstrated
- [ ] Wiring Test table complete
- [ ] Unit fixtures red/green per plugin
- [ ] Interop scenario proven to discriminate (RED on revert)
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Completion (BLOCKING — before ANY commit)
- [ ] Independent review
- [ ] Implementation Summary filled
- [ ] Learned summary
- [ ] Commit A: code + tests + docs + spec + learned; Commit B: `git rm` spec

## Findings During Implementation (2026-07-20)

**AC-4 is NOT yet met.** The engine, plugin, sidecar, and a live GREEN happy-path run all
work (the speaker establishes with a real ze, receives the relayed route, and its plugin
renders a verdict). But the scenario as built does NOT discriminate: reverting the
`buildWireModeUpdate` NEXT_HOP de-duplication (three separate reverts: strip-only, unconditional
insert, and a rebuilt image) left the bytes the speaker received BYTE-IDENTICAL
(`000000144001010040020602010000fdec400304ac1e0009180a0000`, one NEXT_HOP). So the scenario
never reaches the buggy code.

**Root cause (code-verified).** The dup-NEXT_HOP bug lives in `buildWireModeUpdate`
(`reactor_api_batch.go:405`), which is the **API-announce / adj-rib-in replay** path
(`AnnounceNLRIBatch`/`SendRoutes`). A **route server** does NOT use that path: RS forwarding is
`bgp-rs` `ForwardCached` -> `buildFwdBody` (`server_forward.go:195`, `forward_body.go:34`), a
**separate** builder with its own zero-copy and re-encode branches. My scenario 48 (ze as RS,
like scenario 47) therefore forwards the injector's route VERBATIM and never calls
`buildWireModeUpdate`. The adj-rib-in auto replay-on-peer-up path DOES reach it: `handleState`
on peer-up dispatches `update hex`/`update text` commands (`adj_rib_in/rib.go:562-567`) that go
through the API announce path.

**Correct AC-4 design (next step):** model the scenario on `adj-rib-in-replay-on-peerup.ci`
(config with `internal adj-rib-in { use bgp-adj-rib-in }`, injector bound to it), NOT on the RS
scenario 47. The speaker connects as a distinct-IP peer; auto replay-on-peer-up re-encodes the
stored route to it via `buildWireModeUpdate`. This is exactly the coverage the `.ci` suite
cannot provide (a real second peer at a distinct IP), so it is non-redundant.

**Cross-cutting concern for scenario 47 (relay-shape, Thread A -- uncommitted, "reviewed"):**
scenario 47 is ALSO RS-based (`rib`+`rs`, no adj-rib-in), so by the same analysis it does NOT
exercise `buildWireModeUpdate`. Its claimed "RED when the NEXT_HOP de-duplication is reverted"
(Path 1, REPLAY) is therefore SUSPECT and must be re-verified before that work is committed --
its true discriminator may be the `buildFwdBody` split fix (Path 2), with the NEXT_HOP-dedup
discrimination resting only on the unit test
(`TestBuildBatchAnnounce_WireMode_IPv4_NoDuplicateNextHop`). Flagged to the user.

### Deeper conclusion after the adj-rib-in redesign (11 Docker runs)

The user approved redesigning 48 on the adj-rib-in path. I did, and it also does NOT
discriminate. Full evidence:

1. `buildWireModeUpdate` is reached ONLY by the wire-mode structured announce
   (`update ... attr set <block> nhop set <addr>`, parsed in `update_wire.go:60-101`, which
   sets `NLRIGroup.Wire`). NO automatic ze forwarding path uses it:
   - route server: `bgp-rs` `ForwardCached` -> `buildFwdBody` (verbatim/zero-copy);
   - normal best-path forward: `buildFwdBody`;
   - `redistribute_ingress`/`redistribute_egress`: do not call the announce builder at all;
   - adj-rib-in replay: emits `update hex attr set ... nhop set ...`, which SHOULD reach
     `buildWireModeUpdate`, BUT the auto replay-on-peer-up RACES the reactor's established-peers
     set: it fires on the peer's state=up event before the peer is registered as sendable, and
     fails with "no established peers to send to" (`reactor_api_forward.go:71`, logged
     `subsystem=bgp.rs` at `rs/server.go:393`). The route the speaker actually receives comes
     from the always-on forwarding layer, verbatim.
2. Empirical: across RS, RS+adj-rib-in, and adj-rib-in-only configs, on a ze image with the fix
   reverted (both strip-only and the full original unconditional insert), the bytes the speaker
   received were BYTE-IDENTICAL every time:
   `000000144001010040020602010000fdec400304ac1e0009180a0000` (one NEXT_HOP). The insert branch
   of `buildWireModeUpdate` never executed on any delivered route.

**Net:** the dup-NEXT_HOP bug is not on any reliable LIVE path, so no interop scenario (47 or
48) discriminates on it; the correct coverage for `buildWireModeUpdate`'s de-dup is the unit
test. Two real defects surfaced and are NOT yet fixed: (a) scenario 47 is vacuous for its
NEXT_HOP claim; (b) adj-rib-in auto replay-on-peer-up races the established-peers set and fails
to deliver ("no established peers to send to"). AC-4 remains OPEN pending a user decision on
direction (fix the replay race, drive the announce API from a plugin, or accept unit coverage).

**State note:** the local `ze-interop:latest` image is currently a fix-REVERTED build (from the
RED experiments); the working-tree source is fully restored (relay-shape fix intact). Any
`run.py` without `NO_BUILD=1` rebuilds it from correct source.

### Resolution (corrects the two findings above)

The "vacuous" and "replay race" conclusions above were BOTH wrong -- artifacts of a test knob,
not real properties of ze. Root cause: the speaker ran with `--stop-after-updates 1`, so it
disconnected after the FIRST route-bearing UPDATE. Debug logging showed that first UPDATE is
the initial-sync forward (verbatim, one NEXT_HOP); the dup arrives LATER, on Ze's delta-replay
re-announce, which DOES go through `buildWireModeUpdate`. The speaker was quitting before it
arrived, and the "no established peers to send to" EoR error was simply Ze trying to finish the
replay to the already-disconnected speaker -- not a pre-establishment race.

With `--stop-after-updates 0` (stay connected) the scenario DISCRIMINATES against a real ze:

| ze image | replayed UPDATE bytes | speaker |
|----------|----------------------|---------|
| fix in place | `...400304ac1e0009 180a0000` (one NEXT_HOP) | PASS (GREEN) |
| fix reverted | `...400304ac1e0009 400304ac1e0009 180a0000` (NEXT_HOP twice) | FAIL "type 3 appears more than once" (RED) |

**AC-4 is MET.** No reactor/ze code change was needed. The dup-NEXT_HOP bug IS live-reachable
(adj-rib-in delta-replay -> `buildWireModeUpdate` with a valid next-hop and a stored block that
already carries NEXT_HOP), and the independent speaker catches it, which `ze-test peer` cannot.

**Correction for scenario 47 (Thread A) -- re-verified empirically:** the "scenario 47 is
vacuous" claim above is WRONG. Reverting the `buildWireModeUpdate` NEXT_HOP strip DOES make
scenario 47 RED. The result table (rebuild reverted, run 47):

| Route | Path | fix in place | strip reverted |
|-------|------|--------------|----------------|
| 10.0.0.0/24 | Path 1 (replay-on-peer-up to FRR) | present | present (VACUOUS -- verbatim buildFwdBody, not affected by the strip) |
| 203.0.113.0/24 | Path 2 (live relay after FRR is up) | present | ABSENT -> scenario RED |

So scenario 47 correctly gates the strip fix -- but via Path 2 (the live-relayed announce goes
through the wire-mode announce builder and dups when reverted), NOT via Path 1 as its `check.py`
and `inject.msg` comments claim. The static "RS -> buildFwdBody, never buildWireModeUpdate"
analysis was incomplete: the live-relay announce path DOES reach `buildWireModeUpdate`.

**Net for scenario 47:** it is NON-vacuous and gates the fix; only its internal attribution is
inaccurate (Path 1 is vacuous; Path 2 is the discriminator). That comment inaccuracy should be
fixed in the scenario-47 files as part of the relay-shape (Thread A) work, not here.

## Independent Review (round 1)

An independent reviewer (separate context) read the speaker files against the empirically-verified
RED/GREEN behavior. Findings and dispositions:

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| 1 | ISSUE | `engine._recv_exact` returned `None` for BOTH a socket timeout and EOF, so the session loop `break`s at the first sub-second idle gap. The stay-connected design was inoperative; discrimination held only because Ze bursts both UPDATEs within 1s. A delayed duplicate would give a false GREEN on the reverted build. | FIXED: `_recv_exact`/`_read_message` now return distinct `_TIMEOUT` vs `_CLOSED` sentinels; the loop `continue`s on idle and stops only on a real close. Unit-tested (`test_recv_exact_idle_timeout_is_distinct_from_close`, `..._waits_through_midmessage_timeout`, `test_read_message_timeout_then_message`). |
| 2 | ISSUE | An uncaught `struct.error` in `decode_update` (malformed body) or an exception in `run()` would abort with no `result:` line, mis-reported as a relay bug. | FIXED: `decode_update` is bounds-guarded; `main()` wraps `run()` and emits `result: FAIL engine crashed: ...` on any exception. Unit-tested (`test_decode_update_truncated_does_not_crash`). |
| 3 | NOTE | `sendall` in the message branches was unguarded (peer close between recv and reply). | FIXED: the branch body is wrapped in `try/except OSError` and stops cleanly with a note. |
| 4 | NOTE | `run()` docstring was stale vs the ISSUE-1 behavior. | Resolved by the ISSUE-1 fix (the loop now genuinely stays connected, matching the docstring). |
| 5 | NOTE | The peer's OPEN is not parsed (negotiated hold-time ignored; KA cadence uses the engine's own `hold_time/3`). | Left as-is: harmless here (both sides 90s). Recorded as a known limitation. |

After the fixes: 10 unit tests pass; discrimination re-verified with the hardened engine
(reverted build -> RED, fixed build -> GREEN).

## Known Limitations
- The engine is not a mature BGP stack; it decodes only what plugins need. It complements the
  Docker interop daemons (FRR/BIRD/GoBGP), it does not replace them.
- A plugin can have its own bugs; keeping each check tiny and per-test bounds the risk, and
  every plugin ships with a red/green unit fixture.

## Core Insight
The plumbing is identical for every test; only the per-test plugin changes. That is what keeps
an independent, hand-rolled speaker from accreting its own broad (and buggy) validator: a check
exists only because a test wrote it, and it is unit-tested red/green in isolation.

## Implementation Audit

Filled at closure, 2026-08-03, from an INDEPENDENT audit subagent that checked each
claim against the producing file rather than against this spec's own text
(`ai/rules/critical-review.md`, `ai/rules/no-fabrication.md`). The main thread
re-verified the file and test names below before recording them.

| Item | Status | Evidence |
|------|--------|----------|
| The speaker engine and its plugin surface | Done | `test/interop/speaker/engine.py`, `test/interop/speaker/plugins/no_duplicate_attribute.py` |
| AC-1, AC-6 optional plugin loaded on demand | Done | `test_dynamic_load_optional_on_end` (`test/interop/speaker/test_engine.py`) |
| AC-2 duplicate attribute flagged | Done | `test_plugin_flags_duplicate_next_hop` |
| AC-3 a clean UPDATE passes | Done | `test_plugin_passes_clean_update` |
| AC-5 per-instance router id in the OPEN | Done | `test_open_message_carries_per_instance_router_id` |
| Unit coverage | Done | 11 `def test_` in `test/interop/speaker/test_engine.py` |
| Interop scenarios | Done | `test/interop/scenarios/48-rfc7606-speaker-dup-attr`, `49-speaker-two-instance` |
| Sidecar wiring | Done | `speaker-args` / `speaker2-args` and `SPEAKER_IP` / `SPEAKER2_IP` in `test/interop/interop.py` |
| Documentation | Done | `docs/functional-tests/interop.md` speaker section, and the `ai/INDEX.md` keyword row |
| Scenario 47's Path-1/Path-2 attribution correction | Done | `test/interop/scenarios/47-*/check.py` and its `inject.msg` now record Path 1 as verbatim and vacuous, Path 2 as the discriminator |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A scriptable BGP speaker exists for interop scenarios ze cannot drive with `ze-test peer` | data + interop | `test/interop/speaker/engine.py` plus two scenarios using it, 48 and 49 |
| Its plugin surface can assert on a received UPDATE | unit | `test_plugin_flags_duplicate_next_hop` reads the UPDATE and flags the duplicate NEXT_HOP |
| Two instances can run side by side | interop | `49-speaker-two-instance`, with per-instance router id proven by `test_open_message_carries_per_instance_router_id` |

## Pre-Commit Verification

Re-verified independently at closure rather than copied from the audit above.

| Table | Fresh Evidence |
|-------|----------------|
| Files Exist | `ls` returns all three of `test/interop/speaker/engine.py`, `test_engine.py`, `plugins/no_duplicate_attribute.py`; `ls -d test/interop/scenarios/4[89]-*` returns both scenario directories |
| AC Verified | `grep -c 'def test_'` returns 11; each of the four named tests returns present |
| Assumptions Resolved | A-1 and A-2 both `confirmed` above; neither remains `unvalidated` |
| Deferrals | This spec has NO deferral shard: `plan/deferrals/bgp-plugin-speaker.md` does not exist, so closure deletes only the spec |
| Documentation | The interop guide's speaker section and the `ai/INDEX.md` keyword row both exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-plugin-speaker-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| Reviewer lenses used | One independent audit subagent under `/ze-audit`, remit: verify each AC against the producing file, check the spec's own text for open work, check every deferral row's destination, and name what closure would delete |
| Findings | None open. The spec's own "Independent Review (round 1)" findings about AC-4 and a replay race are superseded in-file by its Resolution section, byte table included |

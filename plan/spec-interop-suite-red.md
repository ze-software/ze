# Spec: interop-suite-red -- five interop scenarios are red at HEAD, and one harness helper hides why

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Five scenarios in `test/interop/scenarios/` fail at HEAD, and nothing in `plan/`
records them. `.github/workflows/evidence-nightly.yml` is the ONLY automated
caller of the interop suite, and it is the prerequisite that lets an interop
scenario carry an `RFC requirement:` tag at all (`ai/rules/rfc-compliance.md`).
A suite with five unexplained reds trains its readers to discard it.

**Measured 2026-08-05**, each run twice, once with the working tree's
`test/interop/interop.py` and once with HEAD's, with identical results:

| Scenario | Symptom |
|----------|---------|
| `05-routes-from-frr` | `Ze RIB has 0 received routes (expected >= 3)` |
| `06-routes-from-bird` | `Ze RIB has 0 received routes (expected >= 3)` |
| `09-route-withdrawal-frr` | route absent |
| `10-ipv6-ebgp-frr` | session stuck Active for the whole 90s budget |
| `11-addpath-frr` | plugin dies ~6s in on a rejected token; **root cause found, see below** |

`06` was re-run directly by the main thread and reproduced: BIRD established the
session and exported 3 routes, and ze reported 0.

### 2026-08-07: the RIB reds are fixed, and the fail-open helper with them

`05` and `06` shared one cause after all (A-2 **confirmed**), and it was three
faults stacked behind the single `0` this spec predicted would hide them:

| Fault | Producer | Fix |
|-------|----------|-----|
| The verb form resolved nothing, for ANY verb. 56 of the 63 `ze show` subcommands answered `unknown command` on the HOST, and `clear` / `monitor` / `request` / `set` / `delete` with them | `RunCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) walked the verb-RELATIVE tree from `cli.BuildVerbCommandTree` with the verb-INCLUSIVE argv | `ResolveCommand` aligns the words; `cli.AbsoluteVerbPath` (`internal/component/cli/client/verb_tree.go`) rebuilds every absolute form |
| The interop daemon started no SSH listener, so no CLI client could reach it whatever the verb | `infraSetup` (`cmd/ze/hub/infra_setup.go`) starts one only when the config asks, and no scenario `ze.conf` asked | `ZE_CLI_CONFIG` appended to every RENDERED `ze.conf`, queried through `ze cli -c`, as `test/ipsec-interop/lab.py` already did |
| `Ze.rib_count` returned 0 on failure, which is what made the two above indistinguishable | `Ze.rib_count` (`test/interop/interop.py`) | raises, naming the command and the container (AC-1 **met**) |

It was NOT the `use bgp-rib` producer this spec named as the first thing to read.
The plugin loads; nothing could ask it anything.

`05` PASS (3 routes), `06` PASS (3), `13-graceful-restart-frr` PASS (1). `13` was
not on the list above and was red for its own reason: its `frr.conf` advertised
nothing at all, so `ze.rib_received(1)` could not pass on any run. It now
advertises one prefix.

R-1 fired exactly as written: making the helper fail closed revealed reds that a
masked zero had been carrying. Their rows are below, and none was silenced.

| Scenario | Symptom | Verified |
|----------|---------|----------|
| `35-srv6-frr` | Session never leaves Active over the 90s budget, so the RIB is never queried. Ze LISTENS (TCP connect to `172.30.0.2:179` from the FRR container succeeds) and logs nothing about the attempt, while FRR reports `remote router ID 0.0.0.0`. FRR also reports `Configuration file[/etc/frr/frr.conf] processing failure: 11`, so whether the fault is Ze's or the scenario's SRv6 config is OPEN | Reproduced with the config append disabled, so it predates this work and is not caused by it. The producer that should log an accepted connection has NOT been read |
| `19-routes-gobgp` | `route_json returned no data`: `GoBGP.route_json` (`test/interop/interop.py`) answers None and the check raises BEFORE any RIB query | The failure is in the GoBGP-side helper, not the Ze-side one. `rib_count` is never reached |
| `ospf-gr-frr` | **Ze never starts: its own scenario config does not parse.** `ze validate config test/interop/scenarios/ospf-gr-frr/ze.conf` answers `line 11: expected ';' after support value, got WORD`. Line 11 is `restarter { support planned restart-interval 120 }`: two leaves on one line with no separator, which `p.errorf` (`internal/component/config/parser.go`) refuses. The same line is in `ospf-gr-fib-retention`, `ospf-v6-gr-frr` and `ospf-v6-gr-fib-retention` | Reproduced against the UNMODIFIED file with a host `ze` build, so it is neither the container nor this work. **Which side is wrong is a real question, not bookkeeping**: either the four configs are missing `;` or the parser should accept the compact form. Answer it before editing either |

**A trap waiting behind the `ospf-gr-frr` config fix.** Its check calls
`Ze().rib_received(0)`, and that scenario configures no BGP at all. The call was
a tautology (`count >= 0`) over a helper that answered 0 for a failed query, so
it passed without asking anything. `rib_count` raises now, so whoever fixes the
config meets a red on a BGP surface an OSPF scenario never configures. The
replacement is an OSPF route-retention assertion, not a deleted line: the
scenario's own comment already points at `ospf-gr-fib-retention` for the real
property.

## The reporting defect is separable from the reds, and it is the reason they are hard to read

`Ze.rib_count` (`test/interop/interop.py`) ends `return 0` when its command
produces no parseable output. So "the daemon does not answer this verb" and "the
daemon received no routes" are the same number, and every caller asserting a
lower bound reports the second.

**Its own docstring records that this already happened once**: the helper read
`show rib status` until 2026-08-04, the daemon answered `unknown command`, and it
returned 0 for every caller. The verb was corrected; the fail-open was not.
Measured again on 2026-08-05: `ze show bgp rib status` answers `unknown command`
in the scenario container, so the helper is masking a second fault the same way.

That shape is the same one that produced a BLOCKER in
`spec-wire-edit-4-api-origin-deferred-bird-interop` (closed 2026-08-07 in `2cc75ab5f`) round 3, where
`API.peer_counter` returned its default of 0 on an unreadable lookup and a guard
read 0 as permission to proceed. `ai/rules/evidence.md`: a zero value must never
be a valid-looking answer.

**`05` and `06` declare the plugin.** Both carry `plugin { internal rib { use
bgp-rib; } }`, so a missing declaration is not the explanation. Whether the
plugin fails to load, or the command fails to register, is UNVERIFIED: nobody has
read the producer that resolves `use bgp-rib`. Do that first.

## `11-addpath-frr`: root cause, verified at the producer 2026-08-05

The scenario sends `path-information` as a TOP-LEVEL token
(`test/interop/scenarios/11-addpath-frr/announce-addpath.py`). `ParseUpdateText`
(`internal/component/bgp/plugins/cmd/update/update_text.go`) rejects it there, and
the plugin dies on the `RuntimeError`.

**The error message advertises the keyword it is rejecting.** Its text lists
`path-information (info)` among the valid tokens, so an operator who reads it and
retries at the same position fails again. The keyword is real but belongs
elsewhere: `kwPathInfo` is consumed by the NLRI-section parser
(`internal/component/bgp/plugins/cmd/update/update_text_nlri.go`), whose own
comment states "info/path-information is per-NLRI-section, not top-level".

So there are two defects, and they need separating before either is fixed:

| Defect | Where |
|--------|-------|
| The top-level error lists a keyword that is not valid at top level, pointing the reader at the mistake that produced it | the message in `ParseUpdateText` (`ai/rules/cli.md` governs error text) |
| The scenario places the token at top level | `11-addpath-frr/announce-addpath.py` |

**Fix the message first, then the scenario.** Correcting only the scenario leaves
the message misleading for every future caller, and the message is what taught the
mistake. Decide deliberately whether `path-information` SHOULD be accepted at top
level (ADD-PATH ids are per-NLRI, so probably not) rather than inferring the
answer from whichever fix is smaller.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/evidence.md` - a guard or reader that fails open must say something
- [ ] `test/interop/interop.py` - `Ze.rib_count`, `docker_exec_quiet`
- [ ] `ai/rules/completion.md` - a red is fixed, not recorded; this spec is the home, not the resolution

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `test/interop/interop.py` (`Ze.rib_count` returns 0 on failure; `docker_exec_quiet` returns "" on non-zero exit)
- [ ] `test/interop/scenarios/06-routes-from-bird/ze.conf` (declares `internal rib { use bgp-rib; }`)
- [ ] the producer that resolves `use bgp-rib` into a loaded plugin -- NOT YET READ, and it is the first thing this spec owes

**Behavior to preserve:** every scenario that passes today keeps passing. This
spec makes failures legible and fixes the reds; it does not relax an assertion to
reach green (`ai/rules/completion.md`).

## Data Flow (MANDATORY)

### Entry Point
`python3 test/interop/run.py <scenario>`, or the nightly workflow.

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The five reds are product or config faults, not harness faults. | Each fails identically with HEAD's `interop.py` and with the working tree's. | The harness is implicated and the blast radius grows. | The both-harness runs above. | **confirmed** |
| A-2 | `05` and `06` share one cause. | Identical symptom, identical helper, both declare the rib plugin. | Two fixes needed, not one. | Read the `use bgp-rib` producer. | unvalidated |
| A-3 | The reds are not caused by another session's uncommitted work in this shared checkout. | Unverified. The tree carried 18 modified files on 2026-08-05, four on the BGP path including `received_update.go`. | The reds may vanish on a clean tree, and this spec is chasing a ghost. | Re-run all five from a clean `git archive HEAD` export. | unvalidated -- DO THIS FIRST |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing `rib_count` to fail closed turns other currently-green scenarios red, because they were passing on a masked zero. | More scenarios red after the helper change than before. | That is the helper working. Each newly visible red is a real fault and gets its own row here, never a revert of the fix. |
| R-2 | The reds are environmental on one machine. | They pass elsewhere. | A-3's clean-export run settles it before any fix is designed. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A scenario asks ze for its received-route count and the verb does not resolve | -> | `Ze.rib_count` | a test asserting the helper RAISES rather than returning 0, named at design time |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze show bgp rib status` fails or answers unparseably | `Ze.rib_count` raises and names the failure. It never returns 0 (`ai/rules/evidence.md`) |
| AC-2 | The five scenarios above | Each passes, or has a row here naming its root cause in a producing function |
| AC-3 | The helper is made to fail closed | Any scenario newly revealed as red gets a row, and none is silenced by relaxing an assertion |
| AC-4 | The full suite runs from a clean HEAD export | The red set is exactly the set this spec names, so the working tree is excluded as a cause (A-3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | AC-1 | |

### Functional Tests
<!-- Tooling scope: no daemon Go changes are known yet, so the driving surface is
     the runner. If the rib fault turns out to be daemon-side, this spec grows a
     .ci and this row is revisited. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/interop/run.py` over the five named scenarios | `test/interop/` | the interop suite reports zero unexplained reds | |

## Files to Modify
- `test/interop/interop.py` - `Ze.rib_count` fail-closed
- `docs/architecture/testing/interop.md` - the harness contract, if helper behaviour changes

## Files to Create
- (fill during design)

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | **Yes** | `docs/architecture/testing/interop.md` |
| 1-9, 11-17 | - | Decided at design | The rib fault may prove to be daemon-side, which would reopen rows 4 and 12 |

## Implementation Steps

1. (fill during design -- but A-3 first: re-run all five from a clean HEAD export)

## Known Limitations

- Scope is the five named reds and the one helper that hides them. Other
  fail-open readers in the harness are likely (`ai/rules/evidence.md`), and a
  sweep for them is separate work.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `make ze-precommit-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated, not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

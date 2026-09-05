# Spec: finish-l2tp

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-08-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Close the remaining L2TP test-coverage and documentation gaps. Core L2TP subsystem shipped (7b/7c done); these are proof-run and unit-level residuals.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Release-proof run (L44)** - interop harness complete (`./le deployment l2tp-ppp-test`, xl2tpd/pppd LAC + FRR lab). Open item is the release-proof RUN on a host with `/dev/ppp` + PPPoL2TP kernel support.
- **accel-ppp LCP-Opened+MTU `.ci` (L162)** - needs `/dev/ppp` + root + accel-ppp peer.
- **offline-show-tunnels `.ci` (L194)** - `ze l2tp show tunnels` SSH-creds round-trip; needs ci-harness SSH-cred plumbing.
- **NCP unit-test gaps (L41,L42,L43)** - backend-error injection (L41, mock `addAddrP2PErr` never set), renegotiation-after-Opened behavioural (L42), IPCP DNS Configure-Reject absorb (L43, `ncp.go` unexercised).
- **LCP restart-counter (L163)** - restart-timer landed; IRC/ZRC restart-counter/backoff + AckRcvd coverage still deferred.
- **ppp component-imports doc row (L164)** - `docs/architecture/core-design.md` component-imports table has no `ppp` row.
- **DONE (2026-08-07): `--user` / `-u` flag for `ze l2tp show`/`tunnel`/`session` (from spec-fixit-cli-credential-resolution R-5, 2026-07-16).** The flag, the arg split and the unit tests shipped in commit `b6086ee7d`. The shell-completion half of the row's own scope was still missing and landed on 2026-08-07: `registry.RegisterCommandFlags` for the three paths plus a non-empty `Meta.Subs` (`internal/component/l2tp/cli/register.go`), covered by `register_test.go`. `clientFlags` owns `--user`/`-u` only; every other token stays the daemon's grammar and forwards unchanged, and a flag left after the subcommand is refused before any round trip. Deferral row set to `done` in the retired deferral shard "fixit-cli-credential-resolution". Original record kept below for context: `internal/component/l2tp/cli/show.go` calls `sshclient.LoadCredentialsWithFlags("")` with a hardcoded empty user and exposes no flag, unlike the other 9 client CLIs (`ze cli`, `ze signal`, `ze config set/edit/deactivate/archive`, `ze interface migrate`, `ze bgp plugin cli`). Since spec-fixit-cli-credential-resolution made the zefs store optional, an operator who cannot read the store (every user who did not install ze: the store is one shared `0600` file under `/etc/ze`) can name themselves with `--user` on every command EXCEPT this one, so `ze l2tp show` alone fails with the "no credentials ... pass --user" error unless `ze.ssh.username` is exported. **This is bigger than adding a flag -- read before estimating.** `internal/component/l2tp/cli/` has NO flag parsing outside `decode.go`: `cmdShow` (`show.go`), `cmdTunnelTeardown` and `cmdSessionTeardown` each take a raw `args []string` and append it VERBATIM to the daemon command string (`append([]string{"show","l2tp"}, args...)`), then call the shared `forwardToDaemon(command string)`, which is where the hardcoded `LoadCredentialsWithFlags("")` lives. So there is no FlagSet to hang `--user` on, and `ze l2tp show --user alice tunnels` today would ship `show l2tp --user alice tunnels` to the daemon as a command string (unverified whether the daemon rejects it or misparses -- check before writing the fix). The work is: introduce flag parsing in the three commands without breaking arg pass-through, then thread a user through `forwardToDaemon` into `LoadCredentialsWithFlags`. All three commands need it, not just `show`, or `ze l2tp tunnel`/`session` stay unreachable for the same operators. Reference for the long+short pair: `cmd_set.go` / `cmd_deactivate.go` / `cmd_edit.go` / `signal/main.go` (`fs.String("user", ...)` + `fs.StringVar(user, "u", "", "Short alias for --user")`); `decode.go` shows the local `flag.NewFlagSet` idiom already used in this package. NOT `cmd_archive.go` -- it defines only the long form and has no `-u` alias, so it is the wrong template despite also calling `LoadCredentialsWithFlags`. Note the existing **offline-show-tunnels `.ci` (L194)** item above covers the same command's SSH-creds round-trip, so do both together.
- **Four RADIUS subscriber accounting attributes (from spec-radius-subscriber-attributes, homed here 2026-09-03, re-homed the same day)** - Calling-Station-Id (31), Event-Timestamp (55), Acct-Delay-Time (41) and Acct-Terminate-Cause (49). **Settled, built and closed: `spec-radius-acct-session-attributes` implemented all four and closed on 2026-09-04, and `docs/architecture/l2tp/bng-1-radius-attributes.md` describes what each record now carries.** All four are RFC 2119 MAY for a NAS, so the RFCs decide nothing and `ai/rules/rfc-compliance.md` reserved the choice for the owner. Thomas ruled on 2026-09-03: copy Juniper, emit all four, no config knob. The one place ze does not follow Juniper is Acct-Terminate-Cause on Interim, which RFC 2866 Section 5.10 confines to Stop. Nothing is owed here any more; the costs this row used to carry are in that spec's Task section. Row: the retired deferral shard "radius-subscriber-attributes".
- **AVP reserved bits 2-5 on RECEIVE (from spec-rfcgate-2-deferred-nonunit-evidence-backfill, 2026-08-03)** - establish what ze does with a received AVP whose reserved bits 2-5 are set, which is the receive half of `RFC2661-4.1-1`. A probe on 2026-08-02 produced neither a ze log line nor a reply, and the cause was not isolated, so **nothing is claimed in either direction**: this is not a finding that ze is lenient, and not a finding that it is strict. It needs a deliberate reproduction before it can be called conformant or not. Related and separate: the ledger's `RFC2661-x-1` row records `{single-polarity}` for the header reserved bits on the same grounds, that ze emits them zero by construction and never rejects non-zero on receive (`internal/component/l2tp/header.go`). The send half of both requirements IS proven, by `test/l2tp/rfc2661-emitted-control-shape.ci`. Copy that file's shape: a Python peer that hand-packs the datagram and parses ze's reply with its own decoder. Knowledge from the source spec: `plan/pre-release/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md`. If the reproduction shows ze accepts a reserved-bit-set AVP where the RFC requires otherwise, that is a conformance question for Thomas, not a `{gap}` to annotate (`ai/rules/rfc-compliance.md`).
- **DONE (leaf 2026-07-16, `.ci` 2026-08-23): authradius coa-port YANG leaf + CoA end-to-end `.ci` (from spec-startup-resilience 2026-07-10).** The leaf landed in commit `64d160ace`, with `TestParseConfigCoAPortProductionPath` pinning the File -> Tree -> `Config.CoAPort` path, so the row's claim that no leaf exists has been false since that date. The end-to-end half landed on 2026-08-23 as `test/l2tp/radius-coa-listener.ci`: it starts the daemon from a config naming `coa-port` and asserts the listener answers an RFC 5176 Disconnect-Request with a Disconnect-NAK carrying Error-Cause 503, which proves source filtering accepted the configured RADIUS address and both authenticators verified. The apply-path DNS lookup on that branch is bounded (spec-startup-resilience FIX 2: one shared 750ms deadline < the plugin's 1s ApplyBudget), pinned by `TestServerIPs*`.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - the L2TPv2 integration design, companion to the protocol spec guide

### Source files / docs

- [ ] `internal/component/l2tp/ppp/ncp.go` (absorb/error paths at :444,:696)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/l2tp/ppp/session_run.go` (LCP restart timer / IRC-ZRC at :206,:306,:896)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/interop-l2tp/`, ~~`internal/le/integration/gates.go`~~ internal/le/integration/gates.go :129-131 (interop harness; :112 is stale, see Post-wave corrections 2026-07-10)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/component/l2tp/ppp/ncp.go`
- [ ] `internal/component/l2tp/ppp/session_run.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- L2TP/PPP session establishment against a LAC; `ze l2tp show` CLI

### Transformation Path
1. A LAC establishes an L2TP tunnel + PPP session
2. NCP negotiates addresses/DNS; LCP restart timer governs retransmit
3. Operator queries state via `ze l2tp show tunnels`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LAC -> LNS | L2TP control + PPP over the wire | [ ] |
| CLI -> daemon | `ze l2tp show` over SSH | [ ] |

### Integration Points
- `internal/component/l2tp/ppp/`
- `test/interop-l2tp/`
- `docs/architecture/core-design.md`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Real LAC establishes PPP against ze LNS | → | LCP reaches Opened + MTU set | (fill during design) |
| `ze l2tp show tunnels` against a live daemon | → | tunnel state rendered | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| accel-ppp-lcp, offline-show-tunnels (new) (`.ci`) | test/interop-l2tp, test/plugin | LNS behaviour vs a real LAC / live daemon | |

## Files to Modify

- `internal/component/l2tp/ppp/ncp.go` - see Task work items
- `internal/component/l2tp/ppp/session_run.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `./le verify current mode full`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/planning.md`). Moves to `design` when someone picks it up.

### Post-wave corrections (2026-07-10)

- Stale line ref fixed: `internal/le/integration/gates.go` no longer points at the l2tp interop harness (line 112 now falls inside the deployment-evidence VPP block, between the `./le deployment vpp-test` recipe at :109-111 and `./le deployment vpp-iface-test` at :113). Verified current l2tp locations: the l2tp `.PHONY` declarations are at internal/le/integration/gates.go (plus `ze-qemu-l2tp-ppp-test` in the QEMU line :27); the target recipes are at :121-139 (`./le deployment l2tp-test` :121, `./le deployment l2tp-ppp-test` :125, `./le deployment docker-l2tp-ppp-test` :129, which invokes the `test/interop-l2tp/run.py` (retired; now `internal/le/interoplab/l2tp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> harness at :131, `./le deployment gokrazy-l2tp-ppp-test` :137) and `ze-qemu-l2tp-ppp-test` at :337. Core NCP/LCP evidence (`ncp.go`, `session_run.go` refs) is untouched by the wave.
- Coordination note: this spec is DISTINCT from the in-flight `plan/spec-followup-l2tp-call.md` (designed, in-progress as of 2026-07-10). Whoever picks this skeleton up must check that spec's state at design time and coordinate scope so neither duplicates nor contradicts the other's l2tp test work.

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `rfcgate-2-deferred-nonunit-evidence-backfill.md`, 2026-08-02

Deferred by spec-rfcgate-2-deferred-nonunit-evidence-backfill.

Establish what ze does with a received AVP whose reserved bits 2-5 are set (RFC2661-4.1-1 receive side)

### From `rfcgate-2-deferred-nonunit-evidence-backfill.md`, 2026-08-03

Deferred by spec-rfcgate-2-deferred-nonunit-evidence-backfill (closure review).

Fence `test/l2tp/rfc2661-emitted-control-shape.ci` on the daemon's acceptance line: the peer script sends the SCCCN and exits without reading a reply, so the runner can reach teardown before ze verifies the digest and writes `tunnel now established`

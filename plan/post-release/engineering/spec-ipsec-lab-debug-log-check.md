# Spec: ipsec-lab-debug-log-check

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | A proof gap in the IPsec interop lab (`le` tooling): the lab's debug logging key is proven by a unit test only, and no lab step depends on it. It changes no shipped behaviour, so it does not block release correctness |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The IPsec interop lab gives the Ze container `ZE_LOG=debug`, the registered
base logger key `ze.log` (`zeEnvironment`, `internal/le/interoplab/ipsec/ipsec.go`,
commit `163a5d1b88`). The former key, `ZE_LOG_LEVEL`, resolved to a logger
subsystem named "level" and set no base level at all, so the lab ran Ze's
`ike` logger at its WARN default while every reader assumed debug.

The repair is proven today by one unit test,
`TestPreparedIPsecPeerEnablesDebugLogging`
(`internal/le/interoplab/ipsec/environment_consumer_test.go`), which hands the
prepared Ze container environment to the real logger consumer and asks whether
the `ike` logger is enabled at debug. Nothing in the lab itself would go red if
the key regressed: every Ze-peer needle a checker waits on today is logged at
Info or Warn (`plan/post-release/engineering/tooling-one-off-evidence.md`,
section "P-12 (c)").

This spec adds the lab half: an IPsec interop checker step that waits, with
`lab.waitLog` on `zePeer`, for a record Ze emits only at debug level, in a
scenario that carries no log override of its own (no `ze-env` log key, no
`environment { log { ... } }` block in its `ze.conf`). With `ZE_LOG` restored
to `ZE_LOG_LEVEL` the step must fail, and that red is recorded.

### Owner decision

Thomas, 2026-11-09: P-12 (c) of
`plan/post-release/engineering/spec-tooling-one-off-defects.md` is satisfied by
its unit carrier, `TestPreparedIPsecPeerEnablesDebugLogging`, and the lab check
becomes this separate spec. His words: "unit test and add a spec for a lab
check". This spec therefore owns only the lab step; it does not reopen P-12 (c).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md`, "IPsec diagnostic environment" - the page that states the lab's logging environment
  → Constraint: scenario `ze-env` files are additive; a more specific key such as `ZE_LOG_IKE=info` selects that subsystem's level over the shared base
- [ ] `docs/architecture/testing/interop.md`, vacuity traps - the red walk this step owes
  → Constraint: a lab step is a proof only once the regression is reintroduced, the Ze image rebuilt, and the step seen red
- [ ] `ai/doors/le-tooling.md`, `ai/doors/access-security.md` - the lab and IKE doors
  → Constraint: in IPsec scenarios the scenario name does not say which role Ze plays; read both config files before choosing the needle

**Key insights:**
- The logger precedence is most specific key first, then parent keys, then `ze.log`, then WARN (`internal/core/slogutil/slogutil.go`, package comment). A scenario `ze-env` with `ZE_LOG_IKE=info` therefore lowers `ike` from the lab's debug to info, and cannot carry a debug needle.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/ipsec/ipsec.go` - `zeEnvironment` starts every Ze peer with `ZE_LOG=debug`, then appends the scenario's `ze-env` lines
- [ ] `internal/le/interoplab/ipsec/helpers.go` - `scenarioLab.waitLog` polls the peer's last `logLinesMax` (10000) log lines every 2 s for a substring, until the timeout
- [ ] `internal/le/interoplab/ipsec/checkers.go` - per-scenario checker table; `checkPSKSiteToSite` (scenario `psk-site-to-site`) establishes and checks XFRM state and traffic, with no Ze log wait
- [ ] `internal/le/interoplab/ipsec/environment_consumer_test.go` - the unit carrier of the key
- [ ] `internal/component/ike/engine/fsm.go` - the initiator logs `"ike: sent IKE_SA_INIT"` at Debug after sending the request
- [ ] `internal/component/ike/engine/responder.go` - the responder logs `"ike: sent IKE_SA_INIT response (responder)"` at Debug
- [ ] `internal/component/ike/engine/register.go` - `ConfigureEngineLogger` installs `slogutil.Logger(loggerName)` as the engine logger

**Behavior to preserve:**
- Every existing checker's needles and timeouts; the additive `ze-env` contract.

**Behavior to change:**
- One checker gains a Ze-peer wait on a debug-only record.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lab log wait | `internal/le/interoplab/ipsec/helpers.go` `scenarioLab.waitLog` | Waits until a peer's log holds a substring, failing with the peer and needle named | Extends: the new step is one `waitLog` call on `zePeer`; no new wait primitive |
| Ze peer environment | `internal/le/interoplab/ipsec/ipsec.go` `zeEnvironment` | Sets `ZE_LOG=debug` for every Ze peer, then appends `ze-env` lines | Extends: the step is the lab-level consumer of this base key; the attributed red edits this function back to `ZE_LOG_LEVEL` |
| Scenario `ze-env` files | `test/interop-ipsec/scenarios/*/ze-env` | Per-scenario env additions; seven scenarios carry one, five of them set `ZE_LOG_IKE=info` | Does not apply to the carrying scenario: it must carry none, or the more specific key decides the level instead of the base |
| Config log level | `environment { log { level ...; } }` in `ze.conf` (`invalid-ke-retry`, `ike-rekey-retries-in-the-named-group`) | Sets a daemon log level from config | Does not apply: a scenario with this block is excluded, because the block, not the lab key, could decide the level |
| Unit carrier | `internal/le/interoplab/ipsec/environment_consumer_test.go` `TestPreparedIPsecPeerEnablesDebugLogging` | Proves the prepared environment enables `ike` debug through the real logger consumer | Kept: it is the owner-accepted proof of P-12 (c); the lab step adds the end-to-end half through the container and the daemon's own logger wiring |
| Per-scenario checker table | `internal/le/interoplab/ipsec/checkers.go` checker map | Maps a scenario name to its checker | Extends: the step goes into an existing checker (candidate `checkPSKSiteToSite`) rather than a new scenario, unless research finds that one cannot carry it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The lab's Ze container environment, built by `zeEnvironment` and passed as `PeerConfig.Environment`

### Transformation Path
1. The container starts `ze start /etc/ze/ze.conf` with `ZE_LOG=debug` in its environment
2. The plugin host calls the IKE engine's `ConfigureEngineLogger`, which builds the engine logger from `slogutil.Logger`; the level resolves from the env keys (most specific first, then `ze.log`)
3. The engine sends or answers IKE_SA_INIT and writes the debug record to stderr
4. The checker reads the Ze peer's container log through `scenarioLab.logs` and `waitLog` matches the needle

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Lab ↔ Ze container | Environment variables, container log | No |

### Changed-Wire Census

No wire changes: the step reads an existing log record and adds no format, field or producer. The log line text is the only shared surface, and its producer (`fsm.go` or `responder.go`) is unchanged.

### Integration Points
- `scenarioLab.waitLog` - the step's only primitive
- The carrying checker in the `checkers.go` checker map - where the step runs, after `establish`

## Open Questions

| # | Question | What research must establish |
|---|----------|------------------------------|
| Q-1 | Which debug-level IKE record does the step wait on? | Candidates: `"ike: sent IKE_SA_INIT"` (`fsm.go`, Ze initiator, every exchange) and `"ike: sent IKE_SA_INIT response (responder)"` (`responder.go`, Ze responder). The initiator needle is a prefix of the responder one, so it is safe only where Ze initiates; the choice must quote the producing function and confirm no Info or Warn record carries the same text |
| Q-2 | Which scenario carries it? | Candidate `psk-site-to-site` (Ze initiates, no `ze-env`, no config `log` block). The alternative is `responder-psk` with the responder needle. Read both config files to confirm Ze's role |
| Q-3 | Does the engine's logger name resolve to `ike`? | `ConfigureEngineLogger` receives `CanonicalSubsystemName(name)` from the plugin host. Confirm that name, so `ze.log` (and not a subsystem key the lab never sets) decides its level |
| Q-4 | Does a config `environment { log { level } }` override the env base? | Read the config-side log level consumer. If config wins, the carrying scenario's `ze.conf` must stay free of it, and the step's comment says so |
| Q-5 | The attributed red | Restore `ZE_LOG_LEVEL` in `zeEnvironment`, rebuild the Ze image the lab runs, run the carrying scenario, and record the step failing with the needle named; restore and record green (`ai/rules/interop-and-goal-validation.md`) |
| Q-6 | Stale `ze-env` comments | Five `ze-env` files say "the ike subsystem runs at WARN by default" and set `ZE_LOG_IKE=info` for an Info needle. Under the lab's `ZE_LOG=debug` that premise no longer holds, and the key lowers `ike` to info. Research decides whether each file is still needed, and its comment is corrected in this work (`ai/rules/stale-comments.md`) |

## Acceptance Criteria

Draft, to be fixed at design.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The carrying scenario runs in the IPsec interop lab with the lab's `ZE_LOG=debug` and no scenario log override | The checker's step finds the chosen debug-only IKE record in the Ze peer's log, and the scenario passes | Every existing assertion of the scenario stays; the step adds to them |
| AC-2 | `zeEnvironment` sets `ZE_LOG_LEVEL=debug` instead of `ZE_LOG=debug`, the Ze image is rebuilt, the scenario runs | The step fails, naming the Ze peer and the needle, and the red is recorded in the spec | None: an unchanged pass would prove the step does not depend on the key |
| AC-3 | The step's source | A comment above the wait names the producing function and its Debug level, and why the scenario carries no override | `TestPreparedIPsecPeerEnablesDebugLogging` stays as the unit proof |
| AC-4 | `docs/architecture/testing/interop.md`, "IPsec diagnostic environment" | The section names the lab step that proves the base key end to end | The existing text on additive `ze-env` keys stays |
| AC-5 | The five `ze-env` files that set `ZE_LOG_IKE=info` | Each comment states the level the lab actually runs at; a file is kept only where research shows it still decides an outcome | A file kept for a reason research names |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPreparedIPsecPeerEnablesDebugLogging` (existing) | `internal/le/interoplab/ipsec/environment_consumer_test.go` | The prepared environment enables `ike` debug | exists |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `psk-site-to-site` (candidate, Q-2) | `test/interop-ipsec/scenarios/` | strongSwan | The lab's base log key reaches Ze's IKE logger in the running daemon | |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-ipsec` with the carrying scenario selected | → | the carrying checker's `lab.waitLog(ctx, zePeer, <debug needle>, ...)` | the carrying scenario's checker (candidate `checkPSKSiteToSite`) |

## Files to Modify
- `internal/le/interoplab/ipsec/checkers.go` - the debug-log step in the carrying checker
- `docs/architecture/testing/interop.md` - "IPsec diagnostic environment"
- `test/interop-ipsec/scenarios/*/ze-env` - the stale comments of Q-6

## Files to Create
- None expected.

## Implementation Steps

1. **Phase: Research** -- answer Q-1 to Q-6 with the producing functions quoted, and fix the ACs
2. **Phase: Checker step** -- add the `waitLog` step to the carrying checker
   - Tests: the carrying scenario in `./le test integration interop-ipsec`
   - Files: `internal/le/interoplab/ipsec/checkers.go`
   - Verify: green with `ZE_LOG=debug`; red with `ZE_LOG_LEVEL` after an image rebuild; green restored
3. **Phase: Docs and comments** -- the interop page section and the `ze-env` comments of Q-6

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extend an existing checker | A dedicated new scenario | One `waitLog` in a scenario that already establishes the SA proves the key with no new lab topology |

## Known Limitations
- None yet.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-ipsec-lab-debug-log-check.md` only, in the same `./le commit create` script

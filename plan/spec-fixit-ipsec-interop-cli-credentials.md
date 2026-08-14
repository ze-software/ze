# Spec: fixit-ipsec-interop-cli-credentials -- the lab provisions the credential per scenario, not once

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (none was ever created; `ls plan/deferrals/fixit-ipsec-interop-cli-credentials.md` on 2026-08-14 reports no such file) |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Deferral holder created on 2026-08-02 while the rfcgate-1b RFC 7296 pilot spec was
closing. No spec owned this.

## 2026-08-14: AC-1, AC-2 and AC-4 are met by work that landed elsewhere. AC-3 is NOT

This spec was triaged as "already fixed, close it". A closure review refused
that: the scenario is green, but AC-3 is unmet and the evidence first offered
for it was a mis-grep. The spec stays OPEN on one narrow deliverable, stated
under "Remaining work" below.

Both facts this spec measured are false now. The IPsec sessions that fixed the
Delete path provisioned the credential on the way, so the scenario reaches its
assertions and passes.

| The spec's measured fact | What the tree holds on 2026-08-14 |
|--------------------------|-----------------------------------|
| "The lab provisions no SSH credentials. No `ze init`, no `ze.ssh.username`, no `ze.ssh.password`" | `ze_cli` (`test/ipsec-interop/lab.py`) seeds the client store once with `ZE_CONFIG_DIR=/tmp/ze-cli-store ze init`, fed by a `printf` of user, password, host and port, then runs the command with `ZE_SSH_PASSWORD` in the environment. Its docstring records this spec's exact failure as the reason. Introduced by `7cef0689c` |
| "Scenario 10's `ze.conf` has no `system` block at all" | It carries `system { authentication { user interop { password "$2a$04$..." } } }` and an `environment { ssh { ... server main { ip 127.0.0.1; port 2222 } } }` listener. The bcrypt hash is `lab.py`'s published `ZE_CLI_PASSWORD_HASH`, the same cost-4 hash `test/plugin/authz-default.ci` uses, so it is no new secret. Introduced by `c36ad1627`, refined by `7cef0689c` |

The fix took the route this spec wanted: it provisioned the credential the guard
asks for and it weakened nothing. `readCredentials`
(`internal/core/ssh/client/client.go`) is untouched, and `check.py` still calls
`ze cli -c "clear vpn ipsec sa"`, which is what the spec's R-1 protects.

**It did NOT take the shared route, which is what R-2 asked for and what AC-3
requires.** The daemon-side half is hand-copied per scenario. Only two of the
sixteen scenario configs carry the `system { authentication { user interop } }`
account, `10-clear-reestablish` and `24-delete-while-window-held`, and only
three carry an `environment { ssh }` listener at all. A `grep -l authentication`
over the scenario configs returns all sixteen and means nothing: every scenario
carries the IKE peer's own `authentication { mode pre-shared-secret }` block.
That mis-grep was the first evidence offered for AC-3, and it is why this spec
did not close.

The route AC-3 asks for already exists in the sibling lab. `test/interop/interop.py`
appends `ZE_CLI_CONFIG`, the same account and listener, to the rendered copy of
every scenario `ze.conf` in `_render_scenario_dir`, so no BGP scenario carries
the boilerplate. Two places already claim the IPsec lab does the same, and both
are false today: the comment on `ZE_CLI_CONFIG`, and the interop-lab paragraph
in `docs/architecture/testing/interop.md`.

**Live evidence, 2026-08-14.** `python3 test/ipsec-interop/run.py
10-clear-reestablish` against Docker 29.5.2:

```
  ✓ Ze initiator established a PSK tunnel with strongSwan
  ESP SPIs before clear: ['0x6e3710dd', '0xc293b83a']
  ran `clear vpn ipsec sa` on Ze
  ✓ strongSwan re-established a NEW ESP SA after Ze's clear
    (SPIs ['0x6e3710dd', '0xc293b83a'] -> ['0xb3b57657', '0xc767a33c'])
  ✓ strongSwan SA 'ze' is ESTABLISHED
  ✓ Ze clear re-established the tunnel against strongSwan within 30s
  ✓ PASS
PASS  1 scenario(s)
```

The SPI change is what makes this evidence rather than an exit code: the
strongSwan peer tore its SA down and built a new one because Ze's clear reached
the engine. A run that stopped at the credential wall could not produce it.

The closure review confirmed the discrimination holds for THAT run and found one
way a future run could pass vacuously. `swan_esp_spis` reads through
`StrongSwan.xfrm_state`, which calls `docker_exec_quiet` (`test/ipsec-interop/lab.py`),
and that helper returns an empty string on a non-zero exit or any exception. A
failed `before` snapshot yields an empty set, `now - before` is then non-empty on
the first poll from the SA that already existed, and `check()` passes with the
clear having done nothing. `check()` never asserts `before` is non-empty; it logs
`sorted(before) or "none"`. The recorded run printed two SPIs, so it is sound.
The test does not guarantee that.

## Remaining work

| # | Deliverable | Why |
|---|-------------|-----|
| W-1 | Append the account and the SSH listener to the rendered `ze.conf` in `Lab._prepare_ze_conf` (`test/ipsec-interop/lab.py`), the way `_render_scenario_dir` does in `test/interop/interop.py`, then delete the two hand-written copies from `10-clear-reestablish/ze.conf` and `24-delete-while-window-held/ze.conf` | AC-3, and R-2. Until this lands, the next `ze cli` caller meets a daemon with no listener |
| W-2 | Make the `before` snapshot fail closed: `check()` asserts it is non-empty before the clear, or `xfrm_state` distinguishes "no SAs" from "the command failed" | The scenario's discrimination rests on `before` being real |
| W-3 | Correct the two claims that the IPsec lab already does W-1: the `ZE_CLI_CONFIG` comment in `test/interop/interop.py`, and the interop-lab paragraph in `docs/architecture/testing/interop.md` | Both are false today. W-1 makes them true, so land them together |

Scope note: W-1 and W-3 are one change. W-2 is a test-integrity fix in the same
file and rides with it. No daemon code changes, which is what this spec said from
the start.

## Task

**IPsec interop scenario `10-clear-reestablish` is red. `ze cli -c "clear vpn ipsec sa"`
exits 1 before any SSH connection is opened, because nothing ever provisioned an SSH
username for the lab's ze container.**

The scenario aborts at `test/ipsec-interop/scenarios/10-clear-reestablish/check.py`. The
lab helper raises on any non-zero exit, so the SPI assertions that follow never run. The
IKE code under test is never reached.

### The earlier diagnosis was wrong at its root, and is superseded

A previous diagnosis named `internal/component/ssh/client/client.go` `readCredentials`
failing against the lab's `ZE_STORAGE_BLOB=false`. Both halves are wrong, verified
2026-08-02:

| Claim | Reality |
|-------|---------|
| The file is `internal/component/ssh/client/client.go` | That path does not exist. `internal/component/ssh/` is the SSH **server**. The function is `readCredentials` at `internal/core/ssh/client/client.go` |
| `ZE_STORAGE_BLOB=false` is the cause | `readCredentials` never reads that key. It opens the zefs database directly through `ResolveDBPath`. The only producing consumer of `ze.storage.blob` is `resolve.Storage` (`internal/core/resolve/resolve.go`), which selects the daemon's **config** storage backend |
| Fixing the flag would fix the scenario | It would not. Even with blob storage on, `ze start` creates an empty database and `storedUsername` still returns the empty string. The credential keys are written only by `ze init`, `ze connect`, appliance assemble, and the image server. The lab runs none of them |

`readCredentials` does not fail on the store at all: `openStoreIfReadable` returns a
sentinel for a missing store and resolution continues with a nil store. It fails one step
later, fail-closed and correctly, because no source named a user.

### The actual defect

Two facts, both measured:

1. **The lab provisions no SSH credentials.** No `ze init`, no `ze.ssh.username`, no
   `ze.ssh.password`. `test/ipsec-interop/lab.py` sets only `ZE_STORAGE_BLOB=false` and
   a log level. The per-scenario `ze-env` seam the lab already supports is unused by
   scenario 10, which carries only `check.py`, `swanctl.conf` and `ze.conf`.
2. **Scenario 10's `ze.conf` has no `system` block at all.** It is a bare
   `vpn { ipsec { ... } }` config, so no login user exists for the SSH server to accept even
   if a client credential resolved.

Every green `ze cli` test in the tree provisions first. `test/ipsec/ipsec-clear-reestablish.ci`
does not hit this because it drives the engine-steps executor and never opens SSH, and every
other interop scenario asserts through `swanctl` on the strongSwan side. Scenario 10 is the
only `ze cli` caller in `test/ipsec-interop/` and the only one with neither step.

**This is not an IKE defect.** The handler `handleClearIPsecSA`
(`internal/component/ike/cmd/ipsec.go`) is never reached: the client returns before an SSH
connection is constructed.

### Why this is a defect and not a lab-configuration detail

The scenario exists to prove that an operator `clear vpn ipsec sa` tears down and
re-establishes an SA against a real peer. It has never once proven that. It was authored and
deferred to CI (`plan/deferrals/fixit-ipsec-clear-reestablish.md`, 2026-07-19), and when CI
finally ran it, it aborted before its first assertion. A scenario that cannot reach its
assertions is not coverage (`ai/rules/completion.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - `readCredentials` is a correct fail-closed guard
  → Constraint: the guard is right. Do not weaken it to make the lab pass. Provision the
    credential the guard is asking for.
- [ ] `ai/rules/completion.md` - the user goal is the operator clear
  → Constraint: the fix must make the goal work, not route around the assertion.
- [ ] The `fixit-cli-credential-resolution` record (retired with the learned corpus) - owns `readCredentials`
  → Decision: the resolver's precedence (flag, then env, then store) and its injectable
    seams are settled. This spec consumes them and does not redesign them.
- [ ] The `fixit-ipsec-clear-reestablish` record (retired with the learned corpus) - the IKE half, already landed
  → Constraint: its own notes park scenarios 10 and 11 as statically validated only. That
    park is what this spec closes.

## Current Behavior (MANDATORY)

**Source files read on 2026-08-02:**

- [ ] `internal/core/ssh/client/client.go` - `readCredentials`; resolves username from
  flag, then `ze.ssh.username`, then the store; fails closed naming the host and port when
  none supplies one
- [ ] `internal/core/resolve/resolve.go` - `Storage`; the only producing consumer of
  `ze.storage.blob`, selecting the daemon config backend and nothing to do with credentials

**Behavior to preserve:** `readCredentials` must keep failing closed when no source names a
user. Its error text is the diagnostic that made this finding tractable. The other twelve
interop scenarios must keep passing unchanged.

**Behavior to change:** scenario 10 reaches its SPI assertions. Nothing in the credential
resolver changes.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`docker_exec(ZE_CONTAINER, ["ze", "cli", "-c", "clear vpn ipsec sa"])`, at
`test/ipsec-interop/scenarios/10-clear-reestablish/check.py`.

### Transformation Path

1. `ze cli` calls `LoadCredentialsWithFlags` with no `--user`.
2. `readCredentials` opens the zefs database, which the lab never populated, and continues
   with a nil store.
3. No flag, no `ze.ssh.username`, and no stored username, so it returns the no-credentials
   error naming host and port.
4. The client tries an offline fallback for `clear vpn ipsec sa`. Only two fallbacks exist
   in the tree, neither matching, so there is none.
5. `ze cli` prints the error and returns 1. No SSH connection is opened.
6. The lab's `docker_exec` raises on the non-zero exit, aborting `check()` before the SPI
   comparison.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| lab harness ↔ ze container | `docker exec`, which raises on any non-zero exit | Yes |
| `ze cli` ↔ ze daemon | SSH on 127.0.0.1:2222 | Never reached |
| daemon command dispatch ↔ IKE engine | `handleClearIPsecSA` | Never reached |

### Integration Points

- `test/ipsec-interop/lab.py` - the shared container env, and the per-scenario `ze-env` seam
  that scenario 10 does not use.
- `test/plugin/ssh-user-login-yang.ci` - the working precedent: it runs `ze init` with piped
  answers, then invokes `ze cli` with the password in the environment.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The daemon binds its SSH listener with a config that has no `system` block | NOT verified. The default listen address is a package constant, but whether the listener starts absent a `system` section was not read | The fix needs a `system login user` block in `ze.conf`, not only an env credential | Read the SSH component's startup path and confirm | unvalidated |
| A-2 | Providing a username and password through the per-scenario `ze-env` seam is enough, with no `ze init` | `readCredentials` prefers `ze.ssh.username` and `ze.ssh.password` over the store | The lab must run `ze init` in the container, which is a larger change | Try the env route first; it is the smaller diff | unvalidated |
| A-3 | Once the command runs, the scenario's SPI assertions pass | The IKE work landed and is recorded in learned 1215 | A second, real IKE defect is hiding behind the credential failure. That would be a separate finding, not a scope change | Run the scenario once the command succeeds | unvalidated |
| A-4 | Scenario 11 (`11-responder-accepts-reinit`) is unaffected | It uses no `ze cli`; the whole tree has exactly one such call site | Scenario 11 needs the same fix | Grep the tree for `ze cli` under `test/ipsec-interop/` | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The temptation to drop the `ze cli` call and drive the clear the way the `.ci` does. That removes the only CLI-path coverage in the lab, which is part of what the scenario is for | A proposed diff that deletes line 54 | Reducing coverage to reach green is banned (`ai/rules/completion.md`). Provision the credential instead |
| R-2 | A credential provisioned only for scenario 10 leaves the next `ze cli` caller with the same wall | A second scenario is written and fails identically | Prefer provisioning in the shared `lab.py` over a per-scenario override, if A-1 allows |
| R-3 | A-3 is broken and a real IKE defect surfaces. The scenario then stays red for a new reason | The command succeeds and the SPI poll times out | That is a new finding with its own home. It does not reopen this spec's scope |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The interop lab only. No daemon behavior changes |
| How is it reverted? | Single commit, test-tree only |
| Who else touches this path? | `plan/spec-finish-ci-coverage.md` holds the 2026-07-19 row that deferred scenarios 10 and 11 to CI. This spec answers that row with the measured reason |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze cli -c "clear vpn ipsec sa"` inside the lab container | → | SSH client credential resolution, then `handleClearIPsecSA` | `test/ipsec-interop/scenarios/10-clear-reestablish/check.py`, which must reach its SPI comparison |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze cli -c "clear vpn ipsec sa"` in the lab container | Exits 0 and reaches the daemon |
| AC-2 | Scenario 10 run end to end | Reaches its SPI comparison and its `wait_sa_established`, and passes |
| AC-3 | A future `ze cli` caller added to any interop scenario | Resolves credentials without a per-scenario copy of the setup |
| AC-4 | `readCredentials` with no user from any source | Still fails closed with the same error naming host and port |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| existing coverage | `internal/core/ssh/client/` | AC-4. The resolver is correct and is not changed by this spec | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-clear-reestablish` | `test/ipsec/ipsec-clear-reestablish.ci` | The engine-level clear, already green, must stay green | passing |
| `10-clear-reestablish` | `test/ipsec-interop/scenarios/10-clear-reestablish/check.py` | An operator clear tears down and re-establishes against strongSwan | RED, this spec |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `10-clear-reestablish` | `test/ipsec-interop/scenarios/` | strongSwan | The clear reaches the engine and the SA re-establishes with a new SPI | RED |

## Files to Modify

Written as a table: no daemon code is expected to change.

| File | Change |
|------|--------|
| `test/ipsec-interop/lab.py` | Provision SSH credentials for the ze container, preferring the shared path over a per-scenario override (R-2) |
| `test/ipsec-interop/scenarios/10-clear-reestablish/ze.conf` | Add a `system login user` block, if A-1 shows the listener needs one |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- resolve A-1 by reading the SSH component's startup
   path. Record whether the listener binds with no `system` block.
2. **Phase: Provision** -- add the credential by the smallest route A-1 and A-2 allow.
   - Verify: `ze cli -c "clear vpn ipsec sa"` exits 0 in the container.
3. **Phase: Run** -- run scenario 10 end to end.
   - Verify: AC-2. If the SPI poll now times out, A-3 is broken: report it as a new finding
     and keep this spec open.
4. **Phase: Generalize** -- confirm AC-3 by adding no per-scenario duplication, and note the
   pattern where the next author will find it.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| No weakened guard | `readCredentials` still fails closed. The diff must not touch it |
| No dropped coverage | Line 54 still calls `ze cli`. The scenario still exercises the CLI path |
| Root cause, not symptom | The fix provisions a credential; it does not suppress the non-zero exit or catch the raise |

## Known Limitations

- The lab runs on Docker. Whether the ze container has XFRM at all is host-dependent, and
  scenario 10's assertions read strongSwan's SPIs rather than Ze's for that reason.

## Deliverables Checklist

Added 2026-08-14 by the attempted closure. The spec was written as a skeleton
and never carried this table, so `/ze-close` step 1 had nothing to verify
against.

| Deliverable | Verification method |
|-------------|---------------------|
| The lab provisions an SSH credential for the ze container | Read `test/ipsec-interop/lab.py` for `ze_cli`, and check it seeds the store and passes the password |
| Scenario 10's config accepts that login | Read `test/ipsec-interop/scenarios/10-clear-reestablish/ze.conf` for the `system authentication` account and the SSH listener |
| The provisioning is shared, not per-scenario | `grep -l "user interop" test/ipsec-interop/scenarios/*/ze.conf` returns NOTHING once W-1 lands, because the account is appended at render time. Never grep the bare word `authentication`: every scenario carries an IKE `authentication` block and the answer is always sixteen |
| Scenario 10 passes end to end | `python3 test/ipsec-interop/run.py 10-clear-reestablish` |
| The credential guard is untouched | `git log -- internal/core/ssh/client/client.go` |

## Security Review Checklist

Added 2026-08-14, for the same reason.

| # | Concern | Check |
|---|---------|-------|
| S-1 | A test password becomes a real credential | `ZE_CLI_PASSWORD` is `testpass` and its bcrypt form is the published cost-4 hash `test/plugin/authz-default.ci` already uses. It is no new secret, and the comment above it says so |
| S-2 | The credential escapes the container | `ZE_CLI_STORE` is `/tmp/ze-cli-store` inside the container, seeded by `docker exec`. Nothing writes to an operator machine, and the comment above the constants states that boundary |
| S-3 | The SSH listener is exposed | The scenario binds `127.0.0.1:2222` inside the container. It is not published to the host |
| S-4 | The guard is weakened to make the lab pass | `readCredentials` is unchanged. The fix supplied the credential the guard asked for |
| S-5 | A password on a command line leaks to other container processes | It is visible: `ze_cli` builds one `sh -c` string that carries `ZE_SSH_PASSWORD=testpass` and a `printf` of the same password, so both sit in the argv of a `docker exec` and in the container's process table. The lab container is single-tenant, holds no other workload, and is torn down per scenario, and the password is the published test constant. Accepted for a test lab. It is not a daemon path and no production surface copies it |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

## Implementation Summary (partial, 2026-08-14)

This section records what landed elsewhere. It is not a closure: W-1 in
"Remaining work" is still owed.

### What Was Implemented
Nothing by this spec. Part of its scope landed in the IPsec Delete-path sessions:

- `test/ipsec-interop/lab.py` gained `ze_cli`, which seeds the client zefs store
  with `ze init` once per container and runs the command with `ZE_SSH_PASSWORD`
  in the environment. It also gained `ZE_CLI_USER`, `ZE_CLI_PASSWORD`,
  `ZE_CLI_PORT` and `ZE_CLI_PASSWORD_HASH`, so the daemon-side account is one
  constant rather than a per-scenario secret (`7cef0689c`).
- TWO scenario configs gained the `system { authentication { user interop } }`
  account and an SSH listener, by hand: `10-clear-reestablish` (added by
  `c36ad1627`, refined by `7cef0689c`) and `24-delete-while-window-held`. The
  other fourteen have neither, which is what leaves AC-3 unmet.

### Bugs Found/Fixed
None by this spec. Closing it fixes the record, not the product.

### Documentation Updates
None. The pattern a future author needs is in `ze_cli`'s own docstring, which
names the failure, the seed, and what the scenario's `ze.conf` must carry.

### Deviations from Plan
A-2 predicted the env seam alone would be enough with no `ze init`. The landed
fix runs `ze init` for the username and uses the environment for the password.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: the per-scenario `ze-env` seam would supply both halves, so no `ze init` was needed | The username comes from the seeded store. Only the password rides the environment (`ze_cli`, `test/ipsec-interop/lab.py`) | Reading the landed helper during the 2026-08-14 triage | Recorded; nothing to change |
| approach | This spec planned the work as its own diff | Another session met the same wall while fixing the IKE Delete path and provisioned the credential there, per scenario. The spec then described a tree that no longer existed, so a triage pass read it as fully fixed | Triage pass 2026-08-14, refused by the closure review | Spec kept open on AC-3. The lesson is the class row in `plan/journal/stale-spec-claims-done.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Scenario 10 reaches its SPI assertions | Done | `test/ipsec-interop/scenarios/10-clear-reestablish/check.py` | Proven by the 2026-08-14 lab run |
| Nothing in the credential resolver changes | Done | `internal/core/ssh/client/client.go` `readCredentials` | Untouched; `git log` shows no change from the IPsec commits |
| The `ze cli` call site survives | Done | `check.py` `ze_cli("clear vpn ipsec sa")` | R-1 held: the coverage was not dropped |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | The lab run prints "ran `clear vpn ipsec sa` on Ze" and continues | `docker_exec` raises on a non-zero exit, so reaching the next line IS exit 0 |
| AC-2 | Done | SPIs moved from `['0x6e3710dd', '0xc293b83a']` to `['0xb3b57657', '0xc767a33c']`, then `wait_sa_established` passed | `PASS 1 scenario(s)` |
| AC-3 | **NOT met** | `ze_cli` is shared, but only 2 of 16 scenario configs carry the account and only 3 carry a listener | A new `ze cli` caller in any other scenario is refused by a daemon with no listener until its author hand-copies both blocks. `ze_cli`'s docstring mandates that copy. W-1 is the fix |
| AC-4 | Done | `readCredentials` is unchanged, so its fail-closed error is unchanged | The spec's own Required Reading constraint |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `10-clear-reestablish` | Done, green | `test/ipsec-interop/scenarios/10-clear-reestablish/check.py` | Run live on 2026-08-14 |
| `ipsec-clear-reestablish` | Unchanged, green | `test/ipsec/ipsec-clear-reestablish.ci` | The engine-level clear. Untouched by this work |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/ipsec-interop/lab.py` | Done, by another session | `ze_cli` plus the four constants |
| `test/ipsec-interop/scenarios/10-clear-reestablish/ze.conf` | Done, by another session | `system authentication` account and the SSH listener |

### Audit Summary
- **Total items:** 9
- **Done:** 8
- **Partial:** 1 (AC-3, owed by W-1; no user approval sought or given, and the spec stays open rather than closing over it)
- **Skipped:** 0
- **Changed:** 1 (A-2, recorded in Deviations and the Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator `clear vpn ipsec sa` tears an SA down and re-establishes it against a real peer | interop, strongSwan | `python3 test/ipsec-interop/run.py 10-clear-reestablish` on 2026-08-14: `PASS 1 scenario(s)`, with the ESP SPI pair changing across the clear |
| The scenario is coverage rather than a wall | interop, discrimination | The assertion that fires is the SPI COMPARISON, not an exit code. A credential failure aborts `check()` before it, which is the state this spec was written in. The SPI change can only be produced by the clear reaching `handleClearIPsecSA` and the peer rebuilding. One caveat, owned by W-2: the `before` snapshot fails open, so the guarantee holds for the recorded run and not for every future one |
| The credential guard stays fail-closed | source | `readCredentials` (`internal/core/ssh/client/client.go`) is unchanged. The fix provisioned the credential the guard asks for |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard exists | n/a | `ls plan/deferrals/fixit-ipsec-interop-cli-credentials.md` on 2026-08-14: no such file. The metadata row said one was named but never created, and 2026-08-14 confirms it |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | none. No artifact is recorded, because the spec is NOT closing |
| `review_gate.py check` | not run |
| Rounds | 1, verdict FINDINGS |
| Reviewer lenses used | Refute the closure claim: is the scenario green for the reason the spec says, and does it still assert what it claims to assert |

### Findings
| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| 1 | BLOCKER | AC-3 is unmet and its evidence was a mis-grep. Only 2 of 16 scenario configs carry the account; a `grep -l authentication` returns 16 because every scenario carries an IKE `authentication` block | `test/ipsec-interop/scenarios/*/ze.conf`, `test/ipsec-interop/lab.py` `ze_cli` | Spec stays OPEN. W-1 owns the fix. Every closure statement that rested on the mis-grep is corrected above |
| 2 | ISSUE | The shared route already exists in `test/interop/interop.py` (`ZE_CLI_CONFIG`, `_render_scenario_dir`), and two places claim the IPsec lab does the same. Both are false | `test/interop/interop.py`, `docs/architecture/testing/interop.md` | W-3 |
| 3 | ISSUE | The `before` SPI snapshot fails open, so a future run could pass with the clear doing nothing | `test/ipsec-interop/lab.py` `docker_exec_quiet`, `StrongSwan.xfrm_state`; `10-clear-reestablish/check.py` `check` | W-2 |
| 4 | NOTE | The Documentation Verified row understated its own grep | this spec | Corrected below |

### Refutations that failed
The claim survives on these, each checked at the producer: the SPI comparison is
a real assertion, not a log line (`check` polls, then calls `log_fail` and raises
`AssertionError`); `docker_exec` raises on any non-zero exit; no timer can
produce the SPI change inside the 30 second bound (ESP lifetime 3600, IKE 28800,
strongSwan on its defaults, no `dpd_delay`); no offline fallback can serve
`clear vpn ipsec sa`, because only `show host` and `show crashes` register one
(`MustRegisterOfflineFallback` in `internal/plugins/host/register.go` and
`internal/plugins/crashes/register.go`) and `runOfflineFallback` is consulted
only after a credential or connection failure; `readCredentials` still fails
closed with the host-and-port error, so AC-4 holds.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ipsec-interop/lab.py` | Yes | Read at closure; `ze_cli` at the "client-side store" block |
| `test/ipsec-interop/scenarios/10-clear-reestablish/ze.conf` | Yes | Read at closure; carries `system authentication user interop` and the SSH listener |
| `test/ipsec-interop/scenarios/10-clear-reestablish/check.py` | Yes | Read at closure; `ze_cli("clear vpn ipsec sa")` |
| `plan/deferrals/fixit-ipsec-interop-cli-credentials.md` | No | `ls`: no such file. This is the expected answer |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The command exits 0 and reaches the daemon | Lab run 2026-08-14: "ran `clear vpn ipsec sa` on Ze" is printed AFTER `ze_cli` returns, and `docker_exec` raises on any non-zero exit |
| AC-2 | The scenario reaches its SPI comparison and passes | ESP SPIs `['0x6e3710dd', '0xc293b83a']` -> `['0xb3b57657', '0xc767a33c']`, then `strongSwan SA 'ze' is ESTABLISHED`, then `PASS` |
| AC-3 | A future caller needs no per-scenario setup | **REFUTED.** `grep -l "user interop" test/ipsec-interop/scenarios/*/ze.conf` returns 2 files, and `grep -l environment` returns 3. The earlier "all 16" reading came from grepping the bare word `authentication`, which every scenario carries for its IKE peer |
| AC-4 | `readCredentials` still fails closed | `git log` records no change to `internal/core/ssh/client/client.go` from the IPsec commits; the function is as this spec described it |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze cli -c "clear vpn ipsec sa"` in the lab container -> SSH credential resolution -> `handleClearIPsecSA` | `test/ipsec-interop/scenarios/10-clear-reestablish/check.py` | Yes. Read at closure and RUN at closure. The path is proven by the SPI change on the strongSwan side, which nothing short of the clear reaching the engine produces |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | **broken** | The listener does need config. `ze.conf` carries `environment { ssh { ... } }` and the account, and `ze_cli`'s docstring states "Nothing else in the lab starts a listener, so a config without it refuses the connection" |
| A-2 | **broken** | `ze init` was needed for the username. Only the password rides `ZE_SSH_PASSWORD` (`ze_cli`, `lab.py`) |
| A-3 | **confirmed** | The SPI assertions pass. No second IKE defect was hiding behind the credential wall |
| A-4 | **confirmed** | `grep -rn ze_cli test/ipsec-interop/` outside `lab.py` returns two lines, both in scenario 10's `check.py`. Scenario 11 makes no `ze cli` call |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| One doc category applies | Test-tree only for the daemon: no behavior, no config syntax, no CLI surface, no RFC row. `grep -rln "ipsec-interop" docs/` returns EIGHT files, not the five first recorded here. Seven name the lab, the runner target or a scenario, and go nowhere near credentials. The eighth, `docs/architecture/testing/interop.md`, states that `test/ipsec-interop/lab.py` appends the CLI account the way the BGP lab does. That is false today and W-3 owns it | Partly. The false claim is recorded, not yet fixed |
| The pattern a future author needs | `ze_cli`'s docstring in `test/ipsec-interop/lab.py` states the failure, the seed and the `ze.conf` requirement | Yes, in place |

# Spec: fixit-ipsec-interop-cli-credentials -- scenario 10 is the only `ze cli` caller in the lab, and the lab provisions no credentials

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the Known Limitation is a lab constraint, not postponed work. Create `plan/deferrals/fixit-ipsec-interop-cli-credentials.md` on the first deferral) |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Deferral holder created on 2026-08-02 while the rfcgate-1b RFC 7296 pilot spec was
closing. No spec owned this.

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

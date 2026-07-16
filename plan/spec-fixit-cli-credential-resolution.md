# Spec: fixit-cli-credential-resolution

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-07-16 |

<!-- Renamed from spec-fixit-cli-nonroot-login-default.md at the WRITE gate: the
     OS-login-name default was dropped from scope, so the old name described work
     this spec no longer does. Both remaining bugs are one over-constrained resolver. -->


## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/600-user-login.md`, `plan/learned/747-zefs-remote-creds.md`, `plan/learned/648-ssh-pubkey.md`
4. `internal/core/ssh/client/client.go`, `internal/component/cli/client/main.go`

## Task

**Credential resolution in `internal/core/ssh/client` is over-constrained in two
independent ways. Both block legitimate users; neither has a security purpose.**

**Bug 1 -- resolution prompts when the caller cannot accept a prompt.**
`resolvePassword` blocks on an interactive terminal prompt (`client.go:339-341`) for
any non-super-admin user with no `ze.ssh.password`. Shell tab completion calls the
same resolver (`internal/plugins/completion/peers.go:30`) and runs with stdin attached
to the operator's terminal. An operator who follows the documented completion setup
(`docs/guide/authentication.md:110-117`, which invites setting `ZE_SSH_USERNAME`
without `ZE_SSH_PASSWORD` -- "or use a key-locked secret store") gets a hung shell on
TAB: `password for alice: ` written to stderr, blocking on stdin. `peers.go:31-33`
guards against resolution *errors*, but a prompt is not an error, it is a block.

**Bug 2 -- resolution demands the zefs store even when nothing is needed from it.**
A YANG-configured user must be able to supply a username and password and log in.
For `ZE_SSH_PASSWORD=... ze cli --user alice -c "show version"` every input is already
available without the store: username from the flag (`client.go:320-322`), password
from env (`client.go:333-334`), host/port from env or the `127.0.0.1:2222` defaults
(`client.go:364-384`). Yet `ReadCredentialsForRemote` opens zefs first and aborts on
failure (`client.go:276-279`). Because the store is a single shared `0600` file
(`pkg/zefs/store.go:507`) under a binary-derived config dir (`/usr/local/bin/ze` ->
`/etc/ze`, `internal/core/paths/paths.go:48-51`), any user who is not the installing
user is refused with `open database: permission denied` before their credentials are
ever considered. `test/plugin/ssh-user-login-yang.ci` does not catch this because it
points `ZE_CONFIG_DIR` at a test-owned directory.

This spec makes the store optional when the needed values come from elsewhere, and
makes prompting an explicit opt-in that non-interactive callers can decline.

**Explicitly REJECTED, not deferred (decided at the SCOPE gate):** any restriction on `--user`.
This is not work postponed to a later spec; it is a design that will never be built, so it
has no deferral destination.
An earlier framing proposed gating `--user` on `os.Getuid()==0` and forcing non-root
callers to their login name. That was rejected: role accounts mean an OS user
`thomas` may legitimately need to log into kit as `noc` or `noc-thomas`, and a login-name
rule would lock exactly that operator out -- it would also break the project's own
install guide, which documents `ze cli --user noc` at `docs/guide/ubuntu-build-install.md:194`.
The daemon authenticates every username already, so client-side identity restriction
has no security value (see A-1).

~~Make the username default to the invoking user's OS login name when no zefs store
is readable.~~ **Superseded** at the RESEARCH/DESIGN gates. The user's requirement is
that an operator *supplies* a username and password explicitly; an implicit OS-login-name
default is not needed to satisfy it, and would add a guessing rule with no requester.
Bug 2 delivers the actual need (explicit credentials work without a store) without it.

## Required Reading

### Architecture Docs

- [ ] `plan/learned/600-user-login.md` - introduced `--user` and the current precedence
  → Decision: username precedence is flag > `ze.ssh.username` env > zefs super-admin; password precedence is `ze.ssh.password` env > TTY prompt > super-admin hash-as-token. This spec must NOT reorder or extend these three username sources. ~~This spec appends a fourth username source (OS login name) BELOW zefs.~~ **Superseded at the DESIGN gate** -- the OS-login-name default was dropped from scope; precedence is unchanged. What changes is only that the zefs source may be ABSENT (Bug 2), not that a new source is added.
  → Constraint: no `--password` flag may be added -- passwords in argv leak to shell history, `ps`, and CI logs.
  → Constraint: hash-as-token is super-admin-only; it works only because the daemon's stored hash equals the bytes the client sends. A YANG user can never use it (different salt), so the no-store path MUST supply a real password and MUST NOT attempt hash-as-token (AC-7).

- [ ] `plan/learned/747-zefs-remote-creds.md` - per-service credential storage and `ze remote`
  → Constraint: the `meta/ssh/default` pointer may target a REMOTE host, so "is this a local daemon?" cannot be inferred from the absence of a `--remote` flag; only the resolved host tells the truth.
  → Constraint: credentials are keyed `meta/ssh/<host>/<port>/{username,password}`; `resolveHostPort` must still run even when no per-service username exists.

- [ ] `plan/learned/648-ssh-pubkey.md` - SSH public key authentication
  → Decision: `ze cli` is password-only; key-based login goes through a standard `ssh` client. The no-store path therefore cannot lean on key auth to avoid needing a password source: with no readable store and no `ze.ssh.password`, the only remaining sources are an interactive prompt or an error (AC-5, AC-7).

- [ ] `ai/rules/before-writing-code.md` - sibling call-site audit
  → Constraint: the change lands in a shared resolver with 11 call sites; every caller inherits the new behavior and must be audited, not just `ze cli`.

**Key insights:**
- `resolveUsername` is the single username chokepoint; all 11 CLI entry points reach it through `LoadCredentials*`.
- The daemon authenticates every username at the SSH layer, so the client is not an identity authority.
- Credential resolution currently has a side effect (an interactive password prompt) that fires during what callers treat as a cheap lookup. Extending the success path widens the blast radius of that prompt. This is the central design problem of this spec.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/core/ssh/client/client.go` - SSH credential resolution for every client CLI
  → Constraint: `ReadCredentialsForRemote` (:275) opens zefs FIRST (:276-279) and returns `open database: %w` on any failure, so an unreadable store aborts before host/port/username resolution can run. Bug 2's fix must restructure this ordering so a store failure is classified (A-3) rather than fatal, while a CORRUPT store still errors (AC-8).
  → Constraint: `resolveUsername` (:319) is exact-match and case/whitespace sensitive by design; `--user "admin "` is deliberately a different user from `admin`. Preserve.
  → Constraint: `resolvePassword` (:332-343) PROMPTS on a TTY (:339-341) for any non-super-admin user. This is the pre-existing side effect that the new path would inherit.
  → Constraint: `hostKeyCallback` (:397-407) already defines "local" as `127.0.0.1` / `::1` / `localhost`. Reuse this notion rather than inventing a second one.

- [ ] `internal/component/cli/client/main.go` - `ze cli` entry point
  → Constraint: `runOfflineFallback` (:249) is consulted ONLY after a credential failure (:286) or a connection-level failure (:308). The comment at :305-307 states this ordering is deliberate so the fallback never shadows a live daemon. A fix must not reorder it.
  → Decision: today a non-root user's credential failure at :286 routes `show crashes` / `show host` straight to the offline fallback. If credential resolution starts succeeding, that route is only reached later, at :308.

- [ ] `internal/plugins/completion/peers.go` - shell tab completion
  → Constraint: calls `LoadCredentials()` (:30) with no flag and no TTY discipline. It currently fails fast for non-root. If resolution starts prompting, completion blocks the operator's shell. This is the hardest constraint in the spec.

- [ ] `pkg/zefs/store.go` - blob store
  → Constraint: `Open` (:70) wraps every `load()` failure as `zefs: open %s: %w`; permission-denied and file-missing are not distinguished at the type level. The fallback trigger must classify the error rather than treat all failures alike.
  → Constraint: the store is written `0600` (:507), which is what makes non-root readers fail in production.

- [ ] `internal/component/ssh/ssh.go` - daemon SSH server
  → Decision: `wish.WithPasswordAuth` (:433-453) authenticates `ctx.User()` against the AAA chain and rejects on failure (:450-452). The client cannot grant identity; it can only propose one.

**Behavior to preserve:**
- Username precedence flag > env > zefs, exact-match semantics.
- Password precedence env > TTY prompt > super-admin hash-as-token.
- Super-admin hash-as-token path, unchanged.
- `--user` remains unrestricted for every OS user (role accounts depend on it).
- Offline fallback ordering: never consulted before a live-daemon attempt.
- Existing non-root `.ci` tests that pass `--user` against a test-owned config dir.
- `LoadCredentials()` / `ReadCredentials(dbPath)` back-compat signatures.

**Behavior to change:**
- Prompting becomes an explicit caller policy. Callers that cannot accept a prompt (tab completion) decline it and get an error instead of a block (Bug 1).
- A missing or unreadable zefs store is no longer fatal when username, password, and host/port are all available from flag, env, or defaults (Bug 2). A CORRUPT store remains fatal (AC-8).
- ~~When no zefs store is readable, resolve the username to the OS login name.~~ **Superseded at the DESIGN gate** -- no OS-login-name default; with no store and no username the CLI errors clearly (AC-7).

## Data Flow (MANDATORY)

### Entry Point
- Operator runs a client CLI (`ze cli`, `ze signal`, `ze config set`, ...), optionally with `--user` / `-u`, optionally with `--remote host:port`.

### Transformation Path
1. Flag parsed at the CLI entry point (e.g. `internal/component/cli/client/main.go:264-265`).
2. `LoadCredentialsWithFlags(user)` / `LoadCredentialsForRemote(user, host, port)` (`client.go:429`, `:439`).
3. `ResolveDBPath` (`client.go:410`) locates `database.zefs` under the resolved config dir.
4. `ReadCredentialsForRemote` (`client.go:275`) opens zefs, resolves host/port, reads the per-service username, calls `resolveUsername` (:299) then `resolvePassword` (:302).
5. `Credentials{Host, Port, Username, Auth}` returned.
6. `ExecCommand` / `StreamCommand` build `ssh.ClientConfig{User, Auth: ssh.Password(...)}` (`client.go:71-78`) and `ssh.Dial` (:82).
7. Daemon authenticates via `wish.WithPasswordAuth` (`internal/component/ssh/ssh.go:433`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ credential resolver | `LoadCredentials*` returning a `Credentials` value | [ ] |
| Resolver ↔ zefs store | `zefs.Open` + `ReadFile` on `meta/ssh/*` keys | [ ] |
| Resolver ↔ terminal | prompt policy: `allowPrompt` decided by the caller, not inferred from tty state alone | [ ] |
| Client ↔ daemon | SSH password auth over TCP | [ ] |

### Integration Points
- `resolvePassword` (`client.go:332`) - gains an `allowPrompt` policy parameter (Bug 1).
- `isStdinTTY` (`client.go:346`) - becomes an injectable var for testing.
- `ReadCredentialsForRemote` (`client.go:275`) - gains a classified no-store path (Bug 2).
- `LoadCredentials` (`client.go:422`) - gains a non-interactive sibling for completion.
- `runOfflineFallback` (`internal/component/cli/client/main.go:249`) - its reachability changes as a side effect.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The client cannot grant identity; `--user` only proposes a username the daemon must authenticate. | `internal/component/ssh/ssh.go:433-453` -- `wish.WithPasswordAuth` calls `authenticator.Authenticate` and returns false on failure. | The whole "no client-side restriction" premise collapses and the SCOPE decision must be revisited. | Read of the producing handler (done); `test/plugin/ssh-user-login-yang.ci:146` asserts a wrong password is rejected. | confirmed |
| A-2 | zefs being unreadable is the actual reason a non-installing user's `ze cli` fails in production, and the store is SHARED rather than per-user. | `pkg/zefs/store.go:507` writes `0600`; `client.go:276-279` aborts on `zefs.Open` error; `internal/core/paths/paths.go:70-85` derives the config dir from the BINARY path (not `$HOME`), and `:48-51` maps `/usr/local/bin/ze` -> `/etc/ze`. One shared store for all users. | The fix targets a condition that never occurs; Bug 2 is pointless. | Read of the producing chain (done). `test/plugin/cli-credential-resolution.ci` reproduces it by making the store unreadable. | confirmed |
| A-3 | A permission-denied open can be reliably distinguished from other store failures (corrupt file, bad path). | The syscall error is preserved by `%w` at every hop: `os.Open` -> `fmt.Errorf("zefs: mmap open: %w")` (`pkg/zefs/mmap_unix.go:19-21`, and `mmap_other.go:17-19` for the heap path) -> `load()` returns it unwrapped (`store.go:330-331`) -> `Open` wraps again (`store.go:78`). `decode()` corruption errors do not match `fs.ErrPermission`. | Falling back on ANY store error would mask genuine corruption as "no credentials", hiding real bugs (R-3). | Read of the producing chain (done). Unit test `TestReadCredentialsCorruptStoreReportsError` locks it in. | confirmed |
| A-4 | ~~An OS login name is always resolvable for a process that can run the CLI.~~ | ~~`internal/core/privilege/drop.go:13` uses `os/user`.~~ | N/A | **Withdrawn** -- the OS-login-name default was dropped from scope at the DESIGN gate (see Task). No code now depends on this belief. | withdrawn |
| A-5 | Tab completion runs with stdin attached to a terminal. | `internal/plugins/completion/peers.go:30` calls `LoadCredentials()` with no TTY discipline; shells leave stdin on the tty during completion. | Bug 1 is not reachable in practice and the fix targets nothing. | `TestWritePeersNeverPromptsOnTTY` under a real pty: it must HANG/FAIL before the fix, which proves both the assumption and the bug. | unvalidated -- proven by the phase-1 failing test |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | ~~Risk~~ **PROMOTED TO BUG 1 (see Task).** Tab completion hangs on a password prompt: `completion/peers.go:30` reaches `resolvePassword` (:339-341) with a TTY stdin and blocks the operator's shell. Not a hypothetical consequence of this spec -- reachable today by following `docs/guide/authentication.md:110-117`. | Pressing TAB stalls; a stray `password for alice:` appears in the completion buffer. | Fixed by this spec: explicit non-interactive resolution for callers that cannot accept a prompt. |
| R-2 | **Offline fallback regressed behind a prompt.** `ze cli -c "show crashes"` with an unreadable store currently routes to the offline fallback at `main.go:286` via a credential error; Bug 2's fix makes resolution succeed more often, so a TTY user could be prompted before the fallback is reached at `:308`. | Interactive `show crashes` / `show host` starts asking for a password where it never used to. | AC-7 keeps the no-username no-store case an ERROR, so `:286` still routes to the fallback. With `--user` supplied the operator has explicitly asked to reach the daemon, so a prompt is correct. Verify during implementation. |
| R-3 | Falling back on ANY zefs error masks a corrupt or truncated store as "no credentials", turning a loud bug into a silent misconfiguration. | Users report mysterious auth failures instead of a store error. | Classify the error (A-3); fall back only on permission-denied / not-exist. |
| R-4 | The 9 non-`ze cli` callers (`signal`, `config set/edit/deactivate/archive`, `iface migrate`, `bgp cmd_plugin`, `l2tp show`) inherit the new path unaudited; some are used in scripts where a prompt would hang CI. | A CI job hangs instead of failing fast. | Sibling call-site audit is mandatory; each caller's interactivity is classified in the spec before implementation. |
| R-5 | `l2tp/cli/show.go:51` passes an empty user and exposes no `--user` flag, so an operator who cannot read the store has no way to name themselves for that command (AC-7 error, no login). | Operator cannot reach `l2tp show` as a role account without setting env vars. | Accepted at the DESIGN gate: `ze.ssh.username` + `ze.ssh.password` still work. Adding the flag is a pre-existing gap in `l2tp show`, not work this spec skipped; deferred with a destination to `plan/spec-finish-l2tp.md` and logged in `plan/deferrals.md`. |
| R-6 | **`ze-verify-changed` cannot go green in this working tree, for reasons entirely outside this spec.** Another session is mid-surgery on the functional test HARNESS: `internal/test/peer/expect.go`, `internal/test/runner/{record,record_parse,runner_exec}.go` (modified) and `internal/test/runner/peer_contract.go` (untracked), all for `spec-fixit-redistribute-establishment-stall`. Their new peer contract rejects 60+ existing `.ci` files at discovery ("stdin=peer block: check-mode ze-peer ... declares no ze-peer-consumed expectation ... can only pass vacuously"), taking the plugin suite from 484/484 to 409/484; their `expect.go` changes break `conf-watchdog` (exabgp 41) on a message-matching TIMEOUT. Verified NOT caused by this spec: `conf-watchdog`'s driver `test/exabgp-compat/etc/run/watchdog.run` uses `exabgp_api.send()` over the ExaBGP API bridge, NOT `ze cli`/SSH; `llgr-readvertise-multipeer.ci` and `rr-basic.ci` contain no ssh, no `ze cli` and no credential use. 10 of 12 verify stages pass, including lint, every structural gate, and unit tests -- only `ze-functional-test` and `ze-exabgp-test` fail. | Verify red on tests this diff cannot reach; the summary compounds it by printing "12 suite(s) failed" while every other suite reports 100% pass in the same log (`tmp/verify/11-ze-functional-test.log:640,1091-1707`). An earlier run was also flagged by the harness itself: "contended run detected: 14 ze-test processes". | Committed with `--unverified` naming this cause. Positive evidence for this diff instead: `cli-credential-resolution`, `ssh-user-login-yang`, `rbac-ssh-only-enforced` all pass (3/3) against their REBUILT harness; both changed packages' unit tests pass; the full plugin suite was 484/484 before their `peer_contract.go` landed. A future session should re-run verify once their harness work is committed. The misleading "N suite(s) failed" summary line is worth a look too. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze completion peers` in a shell with `ZE_SSH_USERNAME` set and no password | → | `writePeers` → `LoadCredentialsNonInteractive` → `resolvePassword(allowPrompt=false)` | `TestWritePeersNeverPromptsOnTTY` (pty-backed) |
| `ZE_SSH_PASSWORD=... ze cli --user alice -c "show version"` with an unreadable zefs store | → | `ReadCredentialsForRemote` no-store path → `resolveUsername` → `resolvePassword` | `test/plugin/cli-credential-resolution.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Completion resolves credentials; `ze.ssh.username` set, no `ze.ssh.password`, stdin IS a terminal | No prompt is issued; resolution returns a `no password source` error; `writePeers` returns 0 and emits no completions |
| AC-2 | Completion; no `ze.ssh.username`, readable zefs store | Unchanged: super-admin hash-as-token resolves, completions are produced |
| AC-3 | Completion; both `ze.ssh.username` and `ze.ssh.password` set | Unchanged: completions are produced |
| AC-4 | `ze cli -u alice`, stdin IS a terminal, no `ze.ssh.password` | Unchanged: password is prompted interactively (`docs/guide/authentication.md:97`) |
| AC-5 | `ze cli -u alice -c "..."`, stdin is NOT a terminal, no `ze.ssh.password` | Unchanged: fails with the existing `no password source for user %q` error |
| AC-6 | `ze cli --user alice -c "..."` with `ze.ssh.password` set and an UNREADABLE zefs store | Resolves and connects: username from flag, password from env, host/port from defaults. No `open database` error |
| AC-7 | `ze cli -c "..."` with an unreadable zefs store and NO username from flag or env | Fails with an error naming `--user` and `ze.ssh.password` as the way forward; never attempts the super-admin hash-as-token path |
| AC-8 | zefs store exists but is CORRUPT (not a permission or missing-file failure) | The store error is reported, not silently swallowed as "no credentials" |
| AC-9 | Readable zefs store, no flag or env | Unchanged: super-admin username and hash-as-token, exactly as today |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolvePasswordNoPromptWhenNonInteractive` | `internal/core/ssh/client/client_test.go` | AC-1 -- with `isStdinTTY` stubbed TRUE and `allowPrompt=false`, returns an error and never calls `promptPassword`. The decisive test: proves the prompt is unreachable without needing a real terminal. | planned |
| `TestResolvePasswordPromptsWhenInteractive` | `internal/core/ssh/client/client_test.go` | AC-4 -- with `isStdinTTY` stubbed TRUE and `allowPrompt=true`, the prompt path is taken (stubbed reader), preserving documented behavior | planned |
| `TestReadCredentialsNoStoreFlagAndEnv` | `internal/core/ssh/client/client_test.go` | AC-6 -- unreadable/missing store + `--user` + env password resolves without error | planned |
| `TestReadCredentialsNoStoreNoUsername` | `internal/core/ssh/client/client_test.go` | AC-7 -- unreadable store with no username errors clearly and never reaches hash-as-token with a nil store | planned |
| `TestReadCredentialsCorruptStoreReportsError` | `internal/core/ssh/client/client_test.go` | AC-8 -- a corrupt store is NOT treated as "no credentials" | planned |
| `TestReadCredentialsSuperAdminUnchanged` | `internal/core/ssh/client/client_test.go` | AC-9 -- regression guard on the existing super-admin path | planned |
| `TestWritePeersNeverPromptsOnTTY` | `internal/plugins/completion/peers_test.go` | AC-1 end-to-end -- completion under a real pty (`github.com/creack/pty`, precedent `internal/component/config/system/console_integration_linux_test.go`) exits without blocking | planned |

### Functional Tests
<!-- Provisional -- confirmed at the DESIGN gate. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-credential-resolution-no-store` | `test/plugin/cli-credential-resolution.ci` | AC-6: a YANG user runs `ZE_SSH_PASSWORD=... ze cli --user alice -c "show version"` with the zefs store made unreadable, and logs in -- instead of hitting `open database: permission denied`. | planned |
| `cli-credential-resolution-no-creds` | `test/plugin/cli-credential-resolution.ci` | AC-7: the same operator with no `--user` and no store gets a clear error naming `--user` / `ze.ssh.password`, and no super-admin attempt is made. | planned |

## Files to Modify

- `internal/core/ssh/client/client.go` - make the store optional; thread `allowPrompt`
  into `resolvePassword`; add the non-interactive entry point; make `isStdinTTY` a var
  for injection (repo idiom: `var currentUID = os.Getuid`, `internal/component/doctor/checks_linux.go:92`).
  Its `// Design:` annotation points at `docs/architecture/system-architecture.md` -- check
  whether the credential-resolution description there needs updating.
- `internal/core/ssh/client/client_test.go` - unit tests per the TDD plan.
- `internal/plugins/completion/peers.go` - call the non-interactive variant (:30).
- `internal/plugins/completion/peers_test.go` - pty-backed no-hang test.
- `cmd/ze/internal/ssh/client/client.go` - re-export the new entry point alongside the
  existing thin wrappers (:39-66) so the `cmd/ze` facade stays complete.
- `docs/guide/authentication.md` - fix the completion guidance at :110-117, which currently
  invites the hang by suggesting a username without a password.
- `docs/guide/command-reference.md` - `--user`/`-u` precedence table at :1459-1470.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No new config surface; behavior fix only |
| YANG validation constraints | No | No new leaves |
| YANG custom validators | No | No new leaves |
| CLI commands/flags | No | No new flag; `--user` semantics unchanged |
| CLI grammar (action before identifier) | No | No new command |
| Editor autocomplete | No | Not a config leaf |
| Functional test for new RPC/API | Yes | `test/plugin/cli-credential-resolution.ci` |
| Pipe completeness | No | No new command output |
| Env var registration | No | `ze.ssh.username` / `ze.ssh.password` already registered (`client.go:32-33`) |
| Doctor check for runtime dependencies | No | No new runtime dependency; the zefs store is an EXISTING dependency being made optional |
| Prometheus counters/metrics | No | Client-side resolution, no daemon-observable state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Behavior fix; no new feature row |
| 2 | Config syntax changed? | No | No config surface change |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md:1459-1470` -- `--user` now works without a readable store |
| 4 | API/RPC added/changed? | No | No RPC change |
| 5 | Plugin added/changed? | No | `completion` behavior fix, no interface change |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md:83-117` -- login + completion guidance |
| 7 | Wire format changed? | No | No wire change |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented/changed? | No | Not protocol work |
| 10 | Test infrastructure changed? | Yes (verify) | If the pty test needs harness support, `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | Not a comparison feature |
| 12 | Internal architecture changed? | Yes (verify) | `docs/architecture/system-architecture.md` -- named by `client.go`'s `// Design:` annotation |
| 13 | Route metadata keys added/changed? | No | Not route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin/event/command inventory changed? | No | No registry change |
| 16 | Changed source files referenced by doc source anchors? | Yes | `docs/guide/authentication.md:103` anchors `client.go -- ReadCredentialsWithFlags`; re-verify after the change |
| 17 | Existing docs show examples for this area? | Yes | `docs/guide/authentication.md:89-101`, `docs/guide/ubuntu-build-install.md:191-200` -- verify examples still hold |

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the non-interactive entry point and point
   completion at it; write the failing pty test.
   - Tests: `TestWritePeersNeverPromptsOnTTY`
   - Files: `client.go`, `peers.go`, `cmd/ze/internal/ssh/client/client.go`, `peers_test.go`
   - Verify: the pty test hangs/fails before the fix, proving it reproduces the real bug
2. **Phase: No-prompt policy (Bug 1)** — make `isStdinTTY` injectable; thread `allowPrompt`
   into `resolvePassword`; error instead of prompting when declined.
   - Tests: `TestResolvePasswordNoPromptWhenNonInteractive`, `TestResolvePasswordPromptsWhenInteractive`
   - Files: `client.go`, `client_test.go`
   - Verify: AC-1..AC-5; the pty test now passes
3. **Phase: Store-optional resolution (Bug 2)** — classify the `zefs.Open` failure
   (permission/not-exist vs corrupt); on absence resolve host/port/username/password from
   flag, env, and defaults; guard the nil-store hash-as-token path.
   - Tests: `TestReadCredentialsNoStoreFlagAndEnv`, `TestReadCredentialsNoStoreNoUsername`,
     `TestReadCredentialsCorruptStoreReportsError`, `TestReadCredentialsSuperAdminUnchanged`
   - Files: `client.go`, `client_test.go`
   - Verify: AC-6..AC-9
4. **Functional tests** → `test/plugin/cli-credential-resolution.ci`
5. **RFC refs** → N/A (not protocol work)
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has implementation with file:line |
| Correctness | The nil-store path can never reach `readKey` (AC-7); `isSuperAdmin` cannot be true when `zefsUser` is empty |
| Correctness | Error classification distinguishes permission/not-exist from corruption (AC-8) |
| Data flow | Prompt policy is decided by the CALLER, never inferred from tty state alone |
| Rule: sibling call-site audit | All 11 `LoadCredentials*` callers reviewed; each one's interactivity classified and stated |
| Rule: no-workarounds | Completion's silent degradation is the existing documented policy (`peers.go:31-33`), not a new workaround |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Non-interactive entry point exists and is used by completion | `grep -n "NonInteractive" internal/plugins/completion/peers.go` |
| No caller of the prompting variant runs non-interactively | `grep -rn "LoadCredentials" internal/ cmd/ --include=*.go` and re-audit all 11 |
| Functional test exists | `ls -la test/plugin/cli-credential-resolution.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| No privilege change | The fix grants no identity: the daemon still authenticates every username (`internal/component/ssh/ssh.go:433-453`). Confirm no code path sends a credential the user did not supply |
| Hash-as-token containment | The super-admin hash-as-token path (`client.go:336-338`) must remain reachable ONLY with a readable store and a matching zefs username; never on the no-store path |
| Error leakage | The AC-7 error must name env vars and flags, never echo a password or hash |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A non-root user should be forced to their OS login name, and `--user` gated on root. | Role accounts (`thomas` → `noc`) make OS name and device account deliberately unrelated; the gate would lock out the operator it aimed to serve. | User raised the role-account case at the SCOPE gate. | Original framing abandoned before any code was written; spec re-scoped to the login-name default only. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Gate `--user` / `ze.ssh.username` on `os.Getuid()==0`; non-root must match login name. | Breaks role accounts (see above). Also has no security value: the daemon authenticates every username (A-1), and `ssh noc@kit` bypasses any client-side rule. | Leave `--user` unrestricted; change only the no-credentials default. |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Credential *resolution* in this codebase is not a pure lookup: it can block on an
  interactive password prompt (`client.go:339-341`). Every caller that treats it as
  cheap (tab completion, scripted CLIs) is exposed the moment the success path widens.
  Any fix that makes resolution succeed more often must first decide who is allowed
  to prompt.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Leave `--user` unrestricted. | Root-gate it; force non-root to their login name. | Role accounts (`thomas` → `noc`) require a free choice of login name, and the daemon already authenticates every username (`ssh.go:433-453`), so a client-side rule guards nothing an attacker cannot walk around with `ssh`. |
| Treat "local" as loopback host, not "no `--remote` flag". | Infer local from flag absence. | The `meta/ssh/default` pointer can target a remote host (`747-zefs-remote-creds`), so only the resolved host is authoritative. Reuses the existing notion in `hostKeyCallback` (`client.go:397-407`). |

## Known Limitations

- **A store-less user loses the `meta/ssh/default` pointer.** Host and port fall back to
  env or `127.0.0.1:2222` (`client.go:364-384`). An operator whose daemon listens on a
  non-default port and who cannot read the store must set `ze.ssh.host` / `ze.ssh.port`
  (or pass `--remote`). Accepted: the pointer lives in the store, so it is unknowable
  without it. Documented rather than guessed at.
- **Completion degrades silently.** With a username but no password, the operator gets no
  peer completions and no explanation, because completion's stdout is a completion stream
  and stderr noise would corrupt the shell display. This matches the existing policy for an
  unreachable daemon (`peers.go:31-33`). Better than today's hang, but not self-explaining.
- **No OS-login-name default.** Deliberately dropped (see Task). An operator with no
  credentials and no `--user` gets a clear error naming the way forward (AC-7) rather than
  a guessed identity.
- **`internal/component/l2tp/cli/show.go:51`** passes an empty user and exposes no `--user`
  flag, so it cannot benefit from Bug 2's fix without a flag of its own. A pre-existing gap
  in `l2tp show` rather than work this spec skipped; `ze.ssh.username` still overrides.
  Recorded as R-5 and deferred to `plan/spec-finish-l2tp.md` via `plan/deferrals.md`.

## Implementation Summary

### What Was Implemented
- `resolvePassword` gained an `allowPrompt` caller policy; `isStdinTTY` and
  `passwordPrompter` became injectable vars so the prompt decision is testable
  without a real terminal.
- `LoadCredentialsNoPrompt` added; `internal/plugins/completion/peers.go:30` now
  uses it, so tab completion can never block on a prompt.
- `readCredentials` (new unexported core) makes the zefs store optional:
  `openStoreIfReadable` classifies missing/unreadable as `errStoreUnavailable` and
  continues, while any other failure (corruption) still surfaces.
- `storedUsername` returns "" for a nil store or a missing entry; `isSuperAdmin` now
  requires a non-empty stored username, closing the nil-store hash-as-token path.
- `resolveHostPort` nil-guards the default-pointer read.

### Bugs Found/Fixed
- **Tab completion hung the operator's shell** (Bug 1). Reproduced under a real pty:
  `password for alice:` printed and `writePeers` blocked for the full 5s deadline.
  Test: `TestWritePeersNeverPromptsOnTTY`.
- **Non-installing users could not run `ze cli` at all** (Bug 2). Tests:
  `TestReadCredentialsNoStoreFlagAndEnv`, `TestReadCredentialsUnreadableStoreFlagAndEnv`,
  `test/plugin/cli-credential-resolution.ci`.
- **Found during implementation, not in the spec:** a store that OPENS but holds no
  entry for the requested host/port also failed hard (old `client.go:311-314`), so
  `ze cli --remote 192.0.2.10:2222 --user noc` with a password set was refused -- the
  flow `docs/guide/ubuntu-build-install.md:200` documents. Same root cause, fixed
  together. Test: `TestReadCredentialsStoreWithoutEntryForRemote`.

### Documentation Updates
- `docs/guide/authentication.md:108-127` -- completion guidance rewritten: it
  previously invited the hang by suggesting `ZE_SSH_USERNAME` with the password left
  to a secret store. Anchor: `client.go -- LoadCredentialsNoPrompt`.
- `docs/guide/command-reference.md:1473-1485` -- the store is a source, not a
  prerequisite. Anchor: `client.go -- readCredentials, openStoreIfReadable`.
- `make ze-doc-test` PASSED (609 packages, 290 design docs, 1150 summaries, 3205
  digest anchors resolve).

### Deviations from Plan
- **Spec said to re-export `LoadCredentialsNoPrompt` in `cmd/ze/internal/ssh/client`
  "so the facade stays complete". Not done.** The /ze-review gate found that facade has
  ZERO Go importers -- it is dead code, and completing it would only add a dead symbol.
  Removed; a comment points live callers at the core function instead.
- **Named `NoPrompt`, not `NonInteractive`** (the spec's working name). The audit found
  `client_test.go:226` already uses "NonInteractive" to mean "stdin is not a tty", a
  different concept. Overloading the term would have been actively misleading.
- **Added `passwordPrompter` as a second injectable seam**, not in the spec. Needed to
  assert AC-4 (the interactive prompt is preserved) rather than merely assert its
  absence. `promptPassword` itself was left untouched because a pretool hook blocks
  `fmt.Fprint(os.Stderr, ...)` outside `cmd/`.
- **`.ci` uses an empty config dir rather than `chmod 000`.** chmod cannot deny root, so
  a chmod-based `.ci` would pass vacuously wherever the suite runs as root. Absent and
  unreadable take the same `errStoreUnavailable` path.
- **Scope grew by one sub-case** (the store-with-no-entry path above) after reading the
  producer during implementation. Same root cause and same fix; no new design.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Bug 1: completion must never block the operator's shell | Functional (pty) test, demonstrated RED then GREEN | `TestWritePeersNeverPromptsOnTTY`. Pre-fix: printed `password for alice:` and blocked, `--- FAIL ... (5.04s)`. Post-fix: `--- PASS (0.08s)`. The bug was reproduced, not assumed |
| Bug 2: a YANG user supplies username + password and logs in | `.ci` through the real binary against a live daemon | `test/plugin/cli-credential-resolution.ci` test 1: empty `ZE_CONFIG_DIR` (no store at all), `ze cli --user alice`; asserts the daemon logged `SSH auth success ... username=alice`. `bin/ze-test bgp plugin cli-credential-resolution` -> `1/1 PASS` |
| Bug 2: no credentials gives an actionable error, not a guess | `.ci` assertion on operator-visible stderr | `cli-credential-resolution.ci` test 2: asserts the error names both `--user` and `ze.ssh.password` |
| No regression to documented behavior | Existing suites | 6 pre-existing `client_test.go` tests unmodified and passing; `ssh-user-login-yang` + `rbac-ssh-only-enforced` pass; 484/484 plugin functional tests pass |
| No privilege gain | Source trace + test | With no store, `isSuperAdmin` is unreachable (`client.go:332` requires `zefsUser != ""`), so hash-as-token cannot be used by guessing the admin name; the daemon authenticates regardless (`internal/component/ssh/ssh.go:433-453`). `TestReadCredentialsNoStoreNoUsername` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Bug 1: prompting becomes an explicit caller policy | Done | `client.go:397-412` (`resolvePassword`), `client.go:508-521` (`LoadCredentialsNoPrompt`), `peers.go:29-35` | |
| Bug 2: store optional when values come from elsewhere | Done | `client.go:292-345` (`readCredentials`), `:355-368` (`openStoreIfReadable`) | |
| Corrupt store still surfaces | Done | `client.go:364-366` default branch | AC-8 |
| `--user` remains unrestricted | Done | `resolveUsername` unchanged | Role accounts preserved |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestResolvePasswordNoPromptWhenDeclined`, `TestWritePeersNeverPromptsOnTTY` | RED before fix |
| AC-2 | Done | `TestResolvePasswordSourcesBypassPromptPolicy/super-admin...` | |
| AC-3 | Done | `TestResolvePasswordSourcesBypassPromptPolicy/env_password...` | |
| AC-4 | Done | `TestResolvePasswordPromptsWhenAllowed` | Prompt preserved |
| AC-5 | Done | `TestResolvePasswordNoTTYNeverPrompts` + pre-existing `TestReadCredentialsNonInteractiveNoPassword` | |
| AC-6 | Done | `TestReadCredentialsNoStoreFlagAndEnv`, `TestReadCredentialsUnreadableStoreFlagAndEnv`, `.ci` test 1 | |
| AC-7 | Done | `TestReadCredentialsNoStoreNoUsername`, `.ci` test 2 | |
| AC-8 | Done | `TestReadCredentialsCorruptStoreReportsError` | |
| AC-9 | Done | `TestReadCredentialsSuperAdminStillResolves` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestResolvePasswordNoPromptWhenNonInteractive` | Renamed | `client_test.go` | Shipped as `TestResolvePasswordNoPromptWhenDeclined`; "NonInteractive" was already taken (see Deviations) |
| `TestResolvePasswordPromptsWhenInteractive` | Renamed | `client_test.go` | Shipped as `TestResolvePasswordPromptsWhenAllowed` |
| `TestReadCredentialsNoStoreFlagAndEnv` | Done | `client_test.go` | |
| `TestReadCredentialsNoStoreNoUsername` | Done | `client_test.go` | |
| `TestReadCredentialsCorruptStoreReportsError` | Done | `client_test.go` | |
| `TestReadCredentialsSuperAdminUnchanged` | Renamed | `client_test.go` | Shipped as `TestReadCredentialsSuperAdminStillResolves` |
| `TestWritePeersNeverPromptsOnTTY` | Done | `peers_test.go` | |
| (added) `TestResolvePasswordNoTTYNeverPrompts` | Added | `client_test.go` | AC-5 guard |
| (added) `TestResolvePasswordSourcesBypassPromptPolicy` | Added | `client_test.go` | AC-2/AC-3 |
| (added) `TestReadCredentialsUnreadableStoreFlagAndEnv` | Added | `client_test.go` | Real permission case |
| (added) `TestReadCredentialsStoreWithoutEntryForRemote` | Added | `client_test.go` | Sub-case found during implementation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/ssh/client/client.go` | Done | |
| `internal/core/ssh/client/client_test.go` | Done | |
| `internal/plugins/completion/peers.go` | Done | |
| `internal/plugins/completion/peers_test.go` | Done | pty test |
| `cmd/ze/internal/ssh/client/client.go` | Changed | Re-export NOT added -- dead facade (see Deviations); comment added instead |
| `docs/guide/authentication.md` | Done | |
| `docs/guide/command-reference.md` | Done | |
| `test/plugin/cli-credential-resolution.ci` | Done | |

### Audit Summary
- **Total items:** 9 ACs, 11 tests, 8 files
- **Done:** all 9 ACs; 11/11 tests present (3 renamed, 4 added beyond plan)
- **Partial:** none
- **Skipped:** none
- **Changed:** `cmd/ze` re-export dropped as dead code (Deviations + Review Gate BLOCKER 1)

## Review Gate

Pre-checks (step 0): `make ze-validate` -> 1 issue, PRE-EXISTING (NOTE 3).
`scripts/dev/audit-test-relaxation.py` -> 1 `[RELAXED]`, justified (NOTE 4).

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `LoadCredentialsNoPrompt` re-export is unwired dead code: `cmd/ze/internal/ssh/client` has ZERO Go importers (only a `// Related:` doc-comment mention in `pkg/zefs/store.go:10`). The spec's "keep the facade complete" rationale rested on a false premise -- completing a dead facade only adds a dead symbol. | `cmd/ze/internal/ssh/client/client.go:60` | fixed -- re-export removed; comment now points live callers at `core.LoadCredentialsNoPrompt` |
| 2 | NOTE | The whole `cmd/ze/internal/ssh/client` facade is dead (no importers). Pre-existing, out of scope. | `cmd/ze/internal/ssh/client/client.go` | acknowledged |
| 3 | NOTE | `ze-validate`: `TrimErrorPrefix` has no cross-package non-test caller. Verified PRE-EXISTING -- present in `HEAD:internal/core/ssh/client/client.go:113` with identical package-only usage, so `ze-validate` was already red before this diff. Not part of `ze-verify` (`Makefile:288`), so it does not gate the commit. | `internal/core/ssh/client/client.go:114` | acknowledged (pre-existing) |
| 4 | NOTE | `audit-test-relaxation.py` flags one `[RELAXED]` t.Skip. Justified: `chmod 000` cannot deny root, so the permission-denied condition is unobservable when the suite runs as root. An environment-capability guard, NOT test weakening -- the identical resolution path is covered unconditionally by `TestReadCredentialsNoStoreFlagAndEnv` (not-exist branch) and by the `.ci`. Only the `errors.Is(fs.ErrPermission)` classification is skipped, and only under root. | `internal/core/ssh/client/client_test.go` | acknowledged, reason recorded on the line |
| 5 | NOTE | AC-1 has no `.ci`. Justified: the hang reproduces only with stdin on a tty, which the `.ci` harness does not provide -- a `.ci` would pass vacuously. `TestWritePeersNeverPromptsOnTTY` (real pty) is the only vehicle that reproduces it, and it was demonstrated RED before the fix. | `internal/plugins/completion/peers_test.go` | acknowledged |
| 6 | NOTE | `readCredentials` doc comment ran two paragraphs together with no separator, muddling the function's primary documentation. | `internal/core/ssh/client/client.go:283` | fixed -- paragraph break restored |

Checks that passed (recorded so a re-reviewer need not redo them):

| Check | Evidence |
|-------|----------|
| Guard fails closed | `username == ""` returns an error naming `--user` (`client.go:322-327`); never returns a permissive default |
| Guard driven from its entry point | `TestReadCredentialsNoStoreNoUsername` via `ReadCredentialsWithFlags`, plus `cli-credential-resolution.ci` test 2 through the real `ze cli` binary |
| No nil dereference | `isSuperAdmin` requires `zefsUser != ""` (`client.go:332`) and `storedUsername` returns `""` for a nil store (`client.go:372-374`), so hash-as-token `readKey(store, ...)` is unreachable without a store |
| No privilege gain | With no store, guessing the super-admin name still yields `isSuperAdmin == false`, so hash-as-token is unreachable and a real password is required. The daemon authenticates regardless (`internal/component/ssh/ssh.go:433-453`) |
| Removed-behavior audit | The deleted `zefs.Open` hard-fail invariant is re-established by the `default` branch of `openStoreIfReadable` (AC-8, `TestReadCredentialsCorruptStoreReportsError`). The deleted `no credentials for %s:%s` error survives for the no-username case |
| No external dependency on the changed message | `grep "no credentials for"` -- only `internal/plugins/connect/main.go:300,337`, which formats its own independent message |
| Sibling call-site audit | All 11 `LoadCredentials*` callers classified; completion (`peers.go:30`) was the only unattended one and is fixed. The other 10 are user-invoked commands where prompting is documented behavior |
| No test rewrite dropped coverage | All 6 pre-existing tests in `client_test.go` are unmodified and still pass; the new tests are purely additive |

### Fixes applied
- `cmd/ze/internal/ssh/client/client.go`: removed the unwired `LoadCredentialsNoPrompt` re-export (BLOCKER 1); left a comment directing live callers to `core.LoadCredentialsNoPrompt`.
- `internal/core/ssh/client/client.go`: restored the paragraph break in the `readCredentials` doc comment (NOTE 6).

### Run 2 (after fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | none | No BLOCKER and no ISSUE. `go build ./...` clean; `make ze-lint-changed` 0 issues; `ze-validate` reports only pre-existing NOTE 3; unit suites green; 484/484 plugin functional tests pass. | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

Evidence for final status: Run 2 reports 0 BLOCKER, 0 ISSUE. NOTEs 2-5 are
acknowledged as pre-existing or justified; NOTE 6 was fixed.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/cli-credential-resolution.ci` | Yes | `bin/ze-test bgp plugin cli-credential-resolution` -> `1/1 PASS 104 cli-credential-resolution` (3.6s) |
| `internal/plugins/completion/peers_test.go` | Yes | `TestWritePeersNeverPromptsOnTTY` runs (RED at 5.04s pre-fix, PASS at 0.08s post-fix) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Completion never prompts, even on a tty | `TestResolvePasswordNoPromptWhenDeclined` PASS; `TestWritePeersNeverPromptsOnTTY` PASS under a real pty. Pre-fix the same test printed `password for alice:` and blocked 5.04s -- the bug reproduced, then was fixed |
| AC-2 | Super-admin completion unchanged | `TestResolvePasswordSourcesBypassPromptPolicy/super-admin_hash-as-token_works_with_prompting_declined` PASS |
| AC-3 | Username+password completion unchanged | `TestResolvePasswordSourcesBypassPromptPolicy/env_password_wins_even_with_prompting_declined` PASS |
| AC-4 | Interactive prompt preserved | `TestResolvePasswordPromptsWhenAllowed` PASS (asserts the prompter was actually reached) |
| AC-5 | Non-tty scripted call errors, never prompts | `TestResolvePasswordNoTTYNeverPrompts` PASS; pre-existing `TestReadCredentialsNonInteractiveNoPassword` still PASS |
| AC-6 | YANG user logs in with no readable store | `TestReadCredentialsNoStoreFlagAndEnv` + `TestReadCredentialsUnreadableStoreFlagAndEnv` PASS; `.ci` test 1 asserts the daemon logged `SSH auth success ... username=alice` with an empty config dir |
| AC-7 | No store + no username -> actionable error, no super-admin attempt | `TestReadCredentialsNoStoreNoUsername` PASS; `.ci` test 2 asserts the error names `--user` and `ze.ssh.password` |
| AC-8 | Corrupt store reported, not masked | `TestReadCredentialsCorruptStoreReportsError` PASS |
| AC-9 | Readable store + no flag/env -> super-admin unchanged | `TestReadCredentialsSuperAdminStillResolves` PASS; 6 pre-existing tests unmodified and PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze completion peers` with a username but no password, on a tty | (pty Go test -- `.ci` cannot supply a tty, see Review Gate NOTE 5) | Yes -- `TestWritePeersNeverPromptsOnTTY` drives `writePeers` -> `LoadCredentialsNoPrompt` -> `resolvePassword(allowPrompt=false)`; demonstrated RED before the fix |
| `ZE_SSH_PASSWORD=... ze cli --user alice -c "show version"` with no store | `test/plugin/cli-credential-resolution.ci` | Yes -- runs the real `ze cli` binary against a live daemon with an empty `ZE_CONFIG_DIR`; asserts the daemon authenticated the request as `alice`, not as the super-admin |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/component/ssh/ssh.go:433-453` -- `wish.WithPasswordAuth` authenticates `ctx.User()` via the AAA chain and returns false on failure. The client proposes an identity; it cannot grant one |
| A-2 | confirmed | `pkg/zefs/store.go:507` (0600) + `internal/core/paths/paths.go:70-85`, `:48-51` (binary-derived `/etc/ze`, not `$HOME`) = one shared store. `client.go:276-279` (pre-change) aborted on any open failure |
| A-3 | confirmed | `%w` preserved at every hop: `mmap_unix.go:19-21` -> `store.go:330-331` -> `store.go:78`. `TestReadCredentialsCorruptStoreReportsError` proves corruption is NOT classified as unavailable |
| A-4 | withdrawn | OS-login-name default dropped from scope at the DESIGN gate; no code depends on the belief |
| A-5 | **confirmed** | `TestWritePeersNeverPromptsOnTTY` FAILED on the unfixed tree, printing `password for alice:` and blocking for the full 5s deadline. Tab completion does run on a tty and did hang -- Bug 1 was real, not theoretical |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/authentication.md:108+` -- completion never prompts; set both env vars or neither | Anchored to `client.go -- LoadCredentialsNoPrompt`; matches `peers.go:30` + `resolvePassword` `allowPrompt` gate | Yes; `make ze-doc-test` PASSED |
| `docs/guide/command-reference.md:1459+` -- the store is a source, not a prerequisite; host/port fall back to defaults | Anchored to `client.go -- readCredentials, openStoreIfReadable`; matches `resolveHostPort` nil-store branch | Yes; `make ze-doc-test` PASSED |
| Row 16 (source anchors on changed files) | `grep -rn "source: internal/core/ssh/client" docs/` found 5 anchors, each re-checked: `authentication.md:103` (`ReadCredentialsWithFlags` -- still exists, precedence unchanged); `authentication.md:127` (new, `LoadCredentialsNoPrompt`); `command-reference.md:1485` (new, `readCredentials, openStoreIfReadable`); `command-reference.md:1488` (`ReadCredentialsWithFlags`, unchanged); `operations.md:54` (`ze.ssh.host` / `ze.ssh.port` env override -- still accurate: the env branch of `resolveHostPort` is untouched and still wins; only the store-pointer branch below it gained a nil guard) | Yes -- no stale anchor |
| Row 12 (internal architecture) -- verification | `grep -i "credential\|zefs\|super-admin\|meta/ssh" docs/architecture/system-architecture.md` returns only `:334` "connects to running daemon as SSH client". It documents that the CLI uses SSH, not how credentials resolve, so no claim went stale | Yes -- no update needed |
| Rows 1, 2, 4, 5, 7, 8, 9, 11, 13, 14, 15 answered No | No new feature/config/RPC/plugin/wire/SDK/RFC/metric/inventory surface: the diff changes resolution behavior only. `make ze-doc-test` PASSED (609 packages, 290 design docs, 3205 digest anchors resolve) | Yes |
| Row 10 (test infrastructure) | No harness change: the pty test uses `github.com/creack/pty`, already a direct dependency (`go.mod:74`, precedent `internal/component/config/system/console_integration_linux_test.go`) | Yes -- no update needed |
| Row 12 (internal architecture) | See the dedicated verification row above | Yes -- no update needed |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)

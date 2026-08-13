# Spec: resolve the API per-user gate independently of the ssh block

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md` |
| Handoff | verify |
| Updated | 2026-08-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The REST and gRPC API decide WHETHER to authenticate per user from the
`environment { ssh { } }` config block. `ExtractSSHConfig`
(`internal/component/config/infra/ssh.go`) returns a zero struct when the tree
carries no `environment` container or no `environment/ssh` container, so its
`Users` field is empty. Two hub sites merge that empty field into the API user
list: `runYANGConfig` (`cmd/ze/hub/main.go`) at boot, and `apiAuthReloader`
(`cmd/ze/hub/mgmt_auth_reload.go`) on a reload.

`buildUserAuthenticator` (`cmd/ze/hub/api.go`) returns nil for an empty list, so
the REST and gRPC listeners then gate nothing per user, and `apiAuthed`
(`cmd/ze/hub/main.go`) is false.

The goal is one reader for the API user list, the same reader the login path
already uses: `ExtractAuthUsers` over the `system` root. The ssh block then
decides only whether an SSH server runs, which is what it names.

## Open Question for the Owner

- **Today:** a config that declares `system.authentication.user`, declares an `environment/api-server` block, declares no `environment/ssh` block, sets no API token, and runs where the zefs power user is unreadable resolves to zero API users. `apiAuthed` is false, and `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) makes the daemon exit 1 when the API listener is not loopback.
- **After the correction:** the same config resolves its users from `system/authentication/user`, `apiAuthed` is true, and the daemon serves that listener with per-user authentication actually enforced.
- **Configs whose boot outcome changes:** exactly that set. A readable zefs power user, an API token, an `environment/ssh` block, or a loopback-only API listener each keep the outcome the daemon gives today.
- **Way 1:** correct the resolution and let the guard pass. A false refusal ends, and a daemon that refuses to start today starts tomorrow on an authenticated non-loopback listener.
- **Way 2:** correct the resolution and keep the guard refusing for that set until the operator opts in with a new leaf or a `ze.api-server.*` variable the guard reads. No config changes its boot outcome without an operator edit, at the cost of one more knob.
- **Which way do you want this fixed?** The resolution is corrected either way. Only the guard's verdict for that one set is in question.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the rule this change is governed by: the resolution is a guard
  → Constraint: an empty user list MUST NOT read as "nobody configured" when the reader failed. A reader that cannot answer MUST say so, and the caller MUST deny.
  → Constraint: the test MUST drive the guard from its entry point, a daemon reading a config tree, not from `buildUserAuthenticator` alone.
- [ ] `docs/architecture/config/syntax.md` - the `// Design:` anchor of `internal/component/config/infra/ssh.go`
  → Decision: `environment` holds daemon-process settings and `system` holds identity. The user list lives under `system`, so an API reader that consults `environment/ssh` is reading the wrong root.
- [ ] `docs/guide/authentication.md` - the operator-facing claim about who authenticates on each surface
  → Constraint: its remedy table gives the API remedy as "Configure an API token or initialize zefs users". That row becomes false once config users gate the API, so correcting it is part of this change.

### RFC Summaries (Scope: protocol)
- N-A. Scope is config. No wire format and no RFC obligation is touched.

**Key insights:** (minimal context to resume after compaction)
- The WHO half is already live and MUST NOT be re-fixed: `ExtractAuthUsers` reads `system/authentication/user`, `liveConfigUsers` calls it per request through the live `zeconfig.Provider`, `liveLocalUsers` merges the zefs power users into it, and the API boot path already passes that closure as `apiBuildInputs.UsersLive`.
- The WHETHER half is what remains: the `len(users) == 0` test in `buildUserAuthenticator` and the `len(apiUsers) > 0` test in `apiAuthed` both read a list built from `sshCfg.Users`.
- The failing population needs BOTH halves empty. `apiUsers` starts as the zefs power users, so the defect appears only when `loadZefsUsers` fails or reports `admin-disabled`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `cmd/ze/hub/main.go` - `runYANGConfig` sets `sshCfg` from `infra.ExtractSSHConfig`, then merges `sshCfg.Users` into `apiUsers` with `mergeAuthUsers`, derives `apiAuthed` from `len(apiUsers) > 0 || apiCfg.Token != ""`, feeds `apiAuthed` to the API `mgmtListener` declaration and to `markMgmtAuth` for `svcREST` and `svcGRPC`, and passes `Users: apiUsers` beside `UsersLive: localUsers` in `apiBuildInputs`.
- [ ] `cmd/ze/hub/mgmt_auth_reload.go` - `apiAuthReloader` repeats the same merge over the reloaded tree and derives its own `authenticated` verdict from the merged list and the token.
- [ ] `cmd/ze/hub/api.go` - `buildUserAuthenticator` returns nil for an empty list; for a non-empty list it builds a `LocalAuthenticator` that prefers `usersLive` over the snapshot.
- [ ] `cmd/ze/hub/main_servers.go` - `mergeAuthUsers`, `liveConfigUsers`, `liveLocalUsers`, `loadZefsUsers`, `usersFromZefsDB`.
- [ ] `cmd/ze/hub/mgmt_guard.go` - `checkMgmtListeners` skips an authenticated surface, refuses an unauthenticated non-loopback one, and refuses an unauthenticated surface with no resolved address.
- [ ] `cmd/ze/hub/api_infra.go` - `resolveAPIListeners` and `apiGuardAddrs` produce the addresses the guard judges.
- [ ] `internal/component/config/infra/ssh.go` - `ExtractSSHConfig` returns an empty `SSHExtractedConfig` on a missing `environment` or `environment/ssh` container, before it reaches its own `ExtractAuthUsers` call. `ExtractAuthUsers` reads `system/authentication/user` and is the one producer of that shape.
- [ ] `cmd/ze/hub/service_web.go` - `startWebServer` takes `localUsersLive` and asks it once at construction. The web surface is already independent of the ssh block and is out of scope here.

**Behavior to preserve:**
- The zefs power user keeps authenticating on REST and gRPC exactly as it does now, including the reserved recovery profile `usersFromZefsDB` attaches.
- `mergeAuthUsers` precedence stays: a config user with the same name as a zefs power user replaces it.
- A shared bearer token keeps authenticating when it is set, and `apiAuthed` stays true for a token with no users.
- `apiAuthReloader` keeps its two fail-closed answers: `ExtractAPIConfig` answering not-ok leaves the running credentials alone, and power-user credentials that were readable at boot and are no longer readable fail the reload.
- `buildUserAuthenticator` keeps preferring `usersLive` over the boot snapshot, which is what makes a deleted user stop authenticating without a restart.
- Every message on stderr keeps its exact text: the three `API auth mode:` lines and the guard's refusal line are asserted by `.ci` tests.
- The guard keeps failing closed on an unauthenticated surface with no resolved address.

**Behavior to change:**
- The API boot user list and the reload user list are resolved from the `system` root through `ExtractAuthUsers`, not from `ExtractSSHConfig(tree).Users`.
- Under Way 1 only: `apiAuthed` becomes true, and the daemon boots, for the config set named in the Open Question.
- Under Way 2 only: `apiAuthed` becomes true for the authenticator, and the guard keeps refusing that set until an explicit operator opt-in is present.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config file or a stdin config read by `zeconfig.LoadConfig` in `runYANGConfig` (`cmd/ze/hub/main.go`), producing the boot tree.
- A SIGHUP reload delivering a new config tree to the reloaders registered by `registerMgmtAuthReloaders` (`cmd/ze/hub/mgmt_auth_reload.go`).

### Transformation Path
1. `ExtractSSHConfig` reads the boot tree. With no `environment/ssh` container it returns the zero struct and never reaches its own `ExtractAuthUsers` call.
2. `loadZefsUsers` supplies the power users, or an error that only prints a warning.
3. `mergeAuthUsers` combines the power users with `sshCfg.Users` to produce `apiUsers`.
4. `apiAuthed` is derived from the length of `apiUsers` and the API token.
5. `apiAuthed` reaches the API `mgmtListener` declaration, then `checkMgmtListeners`, which exits the daemon with 1 on refusal.
6. `apiAuthed` also reaches `markMgmtAuth`, which records the boot classification the reload exposure guard reads.
7. `buildAPIShared` calls `buildUserAuthenticator` with `apiUsers` and `localUsers`, and gets nil when `apiUsers` is empty.
8. On reload, `apiAuthReloader` repeats stages 1, 2, 3 and 7 against the new tree.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → hub resolution | `ExtractSSHConfig` and `ExtractAuthUsers` read the config tree and return plain `authz.UserConfig`; the hub holds no YANG path knowledge | Yes: both live in `internal/component/config/infra/ssh.go` |
| Hub → boot exposure guard | `mgmtListener.authenticated` carries the classification; the guard function names no service | Yes: `cmd/ze/hub/mgmt_guard.go` |
| Hub → reload exposure guard | `markMgmtAuth` writes the boot answer, `authReloader` re-answers it per tree | Yes: `cmd/ze/hub/mgmt_auth_reload.go` |
| Hub → REST and gRPC transports | `apiShared.Authenticator`, a header-to-username function; no transport type crosses back | Yes: `cmd/ze/hub/api.go`, `cmd/ze/hub/api_infra.go` |

### Integration Points
- `ExtractAuthUsers` (`internal/component/config/infra/ssh.go`) - the one producer of the config user shape. The corrected boot resolution calls it, so the boot answer and the per-request answer come from the same reader.
- `liveLocalUsers` (`cmd/ze/hub/main_servers.go`) - already the live merged source. The corrected boot list must agree with it rather than duplicate its merge.
- `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) - consumes `apiAuthed` unchanged. The guard itself is not edited.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The hub keeps reading the tree through `internal/component/config/infra`; no YANG path is spelled in `cmd/ze/hub` |
| No unintended coupling (components stay isolated) | Yes | The change REMOVES a coupling: the API stops depending on the SSH config extractor |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ExtractAuthUsers` already exists and is already the live path's reader; no second reader is written |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Startup and reload path; no wire buffers |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them (`ai/rules/plugins.md`) | Yes | No new field, switch case, or factory in a core package. The `mgmtListener` declaration site is unchanged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The WHO half is already live for the API, so only the WHETHER half needs correcting | `runYANGConfig` passes `UsersLive: localUsers` in `apiBuildInputs`; `buildUserAuthenticator` (`cmd/ze/hub/api.go`) replaces the snapshot with `usersLive` when it is non-nil; `apiAuthReloader` passes `apiUsersLive` | Implementation re-does landed work and risks reintroducing a snapshot beside the live source | Read both call sites before editing; `TestAPILoginAdmitsPowerAndConfigUsers` (`cmd/ze/hub/auth_e2e_test.go`) keeps proving the live source is honored | unvalidated |
| A-2 | The failing population needs the zefs power user to be absent as well as the ssh block | `apiUsers` starts as the `loadZefsUsers` result in `runYANGConfig`; only merging two empty lists yields an empty one | The spec overstates the blast radius, and the functional test does not reproduce the case | `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` asserts `API auth mode: single-token (shared bearer)`, which is reached only when the user list is empty, so that suite already runs with no readable zefs power user | unvalidated |
| A-3 | No test or `.ci` asserts that a config with users and no ssh block gets NO API authenticator | Nothing in `cmd/ze/hub` tests names that combination | A green suite hides the change, or the change breaks a test that encodes the defect | Grep `cmd/ze/hub` and `test/` for `API auth mode: NONE` and for the guard refusal text before editing | unvalidated |
| A-4 | `system/authentication/user` is readable from the boot tree at the point `apiUsers` is built | `ExtractSSHConfig` calls `ExtractAuthUsers` on the `system` container of the same tree at the same phase | The boot list stays empty for a different reason and the fix does not work | A unit test that builds a tree with users and no `environment` block and asserts the resolved list | unvalidated |
| A-5 | The owner's answer decides only the guard's verdict, not the resolution | The Open Question above; `apiAuthed` and `buildUserAuthenticator` read the same list today | Way 2 needs a second list, and the design needs reworking | The owner's answer recorded in this spec before implementation starts | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A daemon that refuses to boot today starts tomorrow and serves a non-loopback API listener the operator never re-reviewed | The `.ci` that asserts the refusal changes verdict | This is the Open Question. Way 2 exists to keep the refusal until an operator opts in |
| R-2 | The boot list and the live list drift apart again because the implementation adds a second reader instead of calling `ExtractAuthUsers` | A review finds two places parsing `system/authentication` | AC-1 names the reader. The critical review checks for exactly one producer |
| R-3 | A read failure is turned into an empty list, so the daemon serves unauthenticated where it used to refuse | An authenticator built from a list that came back empty after an error | AC-4: a resolution failure MUST leave `apiAuthed` false and MUST print the reason. Fail closed |
| R-4 | Only one call site is corrected, so a SIGHUP re-derives the old answer and strips per-user authentication from a running listener | The reload test passes at boot and fails after SIGHUP | AC-3 covers `apiAuthReloader`, and the functional test drives a reload, not only a boot |
| R-5 | The reload exposure guard's boot record changes, so a SIGHUP can now migrate the API listener to a non-loopback address for the affected set | `markMgmtAuth` receives a different `apiAuthed` | Intended under Way 1 and stated here so it is a decision, not a side effect. Way 2 must decide whether the opt-in gates this too |
| R-6 | The guard's no-resolved-address refusal is disturbed by an edit near the declaration site | The `checkMgmtListeners` refusal text changes | The declaration site is not edited. Only the value of `apiAuthed` changes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A management API bound to a non-loopback address serves with no per-user gate, or with a gate built from a list a failed read emptied. That is remote unauthenticated command dispatch through the same dispatcher the CLI uses, config commit paths included. The opposite error is milder and visible: a daemon that refuses to boot and prints the reason. A third error is a SIGHUP that strips per-user authentication off a listener that stays up, which is silent |
| How is it reverted? | Single commit revert. No config migration, no persisted state, no wire compatibility. A reverted daemon refuses to boot again for the affected set, which is the behavior operators have today |
| Who else touches this path? | spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild owned the reload-time AAA rebuild seam beside `ListenerMigrator.ReloadListeners`. It CLOSED on 2026-08-12, so the coordination it needed is over and its result is in `cmd/ze/hub/mgmt_auth_reload.go`, the same file as AC-3. Read that file rather than the spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config tree with `system/authentication/user` and no `environment` block, read at boot | → | the API user resolution in `runYANGConfig` (`cmd/ze/hub/main.go`) | `TestAPIUsersResolveWithoutSSHBlock` (`cmd/ze/hub/mgmt_auth_reload_test.go`) |
| The same tree delivered by a SIGHUP reload | → | `apiAuthReloader` (`cmd/ze/hub/mgmt_auth_reload.go`) | `TestAPIAuthReloaderResolvesUsersWithoutSSHBlock` (`cmd/ze/hub/mgmt_auth_reload_test.go`) |
| `ze -` started on that config with a non-loopback REST listener | → | `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) reading `apiAuthed` | `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` |
| A REST request carrying a config user's bearer credential against that daemon | → | the authenticator from `buildUserAuthenticator` (`cmd/ze/hub/api.go`) | `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config tree declaring one user under `system/authentication/user` and no `environment` container, resolved at boot with no readable zefs power user and no API token | The resolved API user list holds that user, read through `ExtractAuthUsers` over the `system` root. Exactly one reader of that root exists in the resolution path |
| AC-2 | The same tree, at boot | `buildUserAuthenticator` receives a non-empty list and returns a non-nil authenticator, and `apiAuthed` is true |
| AC-3 | The same tree delivered to `apiAuthReloader` on a reload | The reloader resolves the same user list and the same authenticated verdict as the boot path, for the same tree |
| AC-4 | A resolution that cannot read the `system` root, as distinct from a `system` root that declares no users | `apiAuthed` stays false, no authenticator is built, and the daemon prints the reason. The empty list is never treated as "the operator configured nobody" |
| AC-5 | A config declaring `system/authentication/user` and `environment/ssh` together | The resolved list is unchanged from today: the same users, once each, with `mergeAuthUsers` precedence intact |
| AC-6 | A config with a readable zefs power user and a config user, no ssh block | Both authenticate over REST, the power user keeps its reserved recovery profile, and a name collision resolves to the config entry |
| AC-7 | An API token set and no users at all | `apiAuthed` stays true and stderr still reads `API auth mode: single-token (shared bearer)` |
| AC-8 | No users, no token, an API block present | stderr still carries the `API auth mode: NONE` warning, and the guard still refuses a non-loopback listener |
| AC-9 (Way 1 only) | The Open Question config with a non-loopback REST listener | The daemon starts, stderr reads `API auth mode: per-user (1 users)`, and a request with a wrong password is refused |
| AC-10 (Way 2 only) | The same config with no opt-in present | The daemon still exits 1 with the guard refusal, and the message names the opt-in as the remedy |
| AC-11 (Way 2 only) | The same config with the opt-in present | The daemon starts and enforces per-user authentication, exactly as AC-9 describes |
| AC-12 | A user deleted from the config, then a reload, on a daemon whose API gate came from AC-2 | That user stops authenticating over REST without a restart, which is the live-source behavior this change must not regress |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | declares an operator under `system.authentication.user`, enables the REST API, writes no `environment/ssh` block, and dispatches a command with that credential | config tree → API user resolution in `runYANGConfig` → `buildUserAuthenticator` → REST transport → dispatcher | `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` |
| 2 | removes that operator and sends SIGHUP | reloaded tree → `apiAuthReloader` → `UpdateAuth` on the running REST server | `test/ui/api-user-removed-by-reload.ci` (existing, must stay green) |
| 3 | starts the daemon with users configured and a non-loopback API listener | config tree → `apiAuthed` → `checkMgmtListeners` | `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAPIUsersResolveWithoutSSHBlock` | `cmd/ze/hub/mgmt_auth_reload_test.go` | AC-1, AC-2: a tree with users and no `environment` container resolves a non-empty API user list and a true authenticated verdict | |
| `TestAPIAuthReloaderResolvesUsersWithoutSSHBlock` | `cmd/ze/hub/mgmt_auth_reload_test.go` | AC-3: the reloader agrees with the boot path on the same tree | |
| `TestAPIUserResolutionFailsClosedWhenTheSystemRootIsUnreadable` | `cmd/ze/hub/mgmt_auth_reload_test.go` | AC-4: a read failure denies and says so; it never becomes an empty "nobody configured" list | |
| `TestAPIUsersUnchangedWithSSHBlockPresent` | `cmd/ze/hub/mgmt_auth_reload_test.go` | AC-5: no duplicate entries, `mergeAuthUsers` precedence intact | |
| `TestAPIUsersMergePowerAndConfigWithoutSSHBlock` | `cmd/ze/hub/auth_e2e_test.go` | AC-6: both credentials authenticate through the real authenticator, and the name collision resolves to the config entry | |
| `TestAPIAuthedTokenOnlyUnchanged` | `cmd/ze/hub/mgmt_guard_test.go` | AC-7, AC-8: the token-only and nothing-configured verdicts are unchanged, and the guard's refusal for the nothing-configured case still fires | |
| `TestAPIAuthReloaderFailsClosedWhenCredentialsBecomeUnreadable` | `cmd/ze/hub/mgmt_auth_reload_test.go` | existing test, must stay green: the boot-had-credentials reload still fails closed | |
| `TestAPILoginAdmitsPowerAndConfigUsers` | `cmd/ze/hub/auth_e2e_test.go` | existing test, must stay green: the authenticator still answers from the live source, not a snapshot | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| resolved API user count | 0 to N | 1, the smallest count that builds an authenticator | 0, which builds none and leaves `apiAuthed` false | N-A: no upper bound. A larger list costs a longer linear scan and nothing else |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mgmt-guard-api-users-without-ssh-block` | `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` | An operator declares a user under `system.authentication.user`, enables REST on a non-loopback address, writes no ssh block and no token, and expects the daemon to serve that user and refuse a wrong password. Under Way 1 the daemon starts and stderr reads the per-user auth mode. Under Way 2 the same config still exits 1 and a second run carrying the opt-in starts. Follows the sibling `test/plugin/mgmt-guard-api-env-started-settings-survive.ci`, including its exclusive `mgmt-guard` group | |
| `api-user-removed-by-reload` | `test/ui/api-user-removed-by-reload.ci` | Existing test. A deleted user loses REST access at the next reload. AC-12: it must stay green, because it is what proves the live source survived this change | |

### Interop Tests (Scope: protocol)
N-A. Scope is config. No wire format, no peer daemon, and no RFC obligation is
touched. The only protocol on the path is the HTTP bearer credential, which this
change does not alter.

## Files to Modify
- `cmd/ze/hub/main.go` - resolve `apiUsers` from the `system` root rather than from `sshCfg.Users`. Under Way 2, also feed the opt-in into the API `mgmtListener` declaration
- `cmd/ze/hub/mgmt_auth_reload.go` - the same correction in `apiAuthReloader`, so a reload cannot re-derive the old answer
- `cmd/ze/hub/mgmt_auth_reload_test.go` - the resolution and fail-closed unit tests
- `cmd/ze/hub/auth_e2e_test.go` - the merged power-user and config-user case
- `cmd/ze/hub/mgmt_guard_test.go` - the unchanged token-only and nothing-configured verdicts
- `docs/guide/authentication.md` - its remedy table gives the API remedy as a token or zefs users; config users now gate the API too
- `internal/component/config/infra/ssh.go` - only if the implementation extracts a shared helper. `ExtractAuthUsers` already exists, so no change is expected here

## Files to Create
- `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` - the functional test for the whole chain, driven from `ze -` with a config on stdin

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Way 2 only | Way 1 adds no leaf. Way 2 adds one opt-in leaf under `environment/api-server`, beside the existing api-server leaves in the owning component's `yang/` |
| YANG validation constraints | Way 2 only | The opt-in is a boolean leaf; `type boolean` is the whole constraint |
| YANG custom validators | No | No cross-node rule. The guard reads the value at boot |
| CLI commands/flags | No | No new verb. The daemon boot path is the only surface |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | Way 2 only | Automatic for a YANG boolean leaf; no `CompleteFn` needed |
| Functional test for new RPC/API | Yes | `test/plugin/mgmt-guard-api-users-without-ssh-block.ci` |
| Pipe completeness | No | No new command output |
| Env var registration | Way 2 only | An `environment/` leaf needs a matching `ze.api-server.<leaf>` through `env.MustRegister()`, as `ze.api-server.token` has |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary. The listen addresses and their doctor coverage are unchanged |
| Prometheus counters/metrics | No | No new observable state. The auth mode is already reported on stderr at boot |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Way 2 only | `docs/features.md`, for the opt-in leaf. Way 1 corrects a defect and adds no feature |
| 2 | Config syntax changed? | Way 2 only | `docs/guide/configuration.md`, `docs/architecture/config/environment.md` for the opt-in leaf |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | The REST and gRPC surfaces are unchanged. Only who may reach them changes |
| 5 | Plugin added/changed? | No | The hub is not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md`, the page that states which source authenticates which surface |
| 7 | Wire format changed? | No | No wire format on this path |
| 8 | Plugin SDK/protocol changed? | No | No SDK type crosses this seam |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs local credential resolution |
| 10 | Test infrastructure changed? | No | The new `.ci` uses the existing `test/plugin` suite and `make ze-plugin-test` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about this |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md` if it states where the API user list comes from; verify before editing |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for source anchors naming `cmd/ze/hub/main.go`, `cmd/ze/hub/mgmt_auth_reload.go`, and `cmd/ze/hub/api.go`, and correct each stale claim. `docs/guide/authentication.md` already anchors `cmd/ze/hub/main_servers.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The remedy table in `docs/guide/authentication.md` gives the API remedy as a token or zefs users. That remedy is incomplete once config users gate the API |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point reaches the resolution before changing it
   - Tests: `TestAPIUsersResolveWithoutSSHBlock`, `TestAPIAuthReloaderResolvesUsersWithoutSSHBlock`
   - Files: `cmd/ze/hub/mgmt_auth_reload_test.go`
   - Verify: both tests fail today, and they fail on the resolved user list being empty rather than on a compile error or a nil map
2. **Phase: Boot resolution** -- read the API user list from the `system` root
   - Tests: `TestAPIUsersResolveWithoutSSHBlock`, `TestAPIUsersUnchangedWithSSHBlockPresent`, `TestAPIUsersMergePowerAndConfigWithoutSSHBlock`
   - Files: `cmd/ze/hub/main.go`
   - Verify: AC-1, AC-2, AC-5 and AC-6 hold, and the three `API auth mode:` stderr lines keep their exact text
3. **Phase: Reload parity** -- the same correction in `apiAuthReloader`
   - Tests: `TestAPIAuthReloaderResolvesUsersWithoutSSHBlock`, `TestAPIAuthReloaderFailsClosedWhenCredentialsBecomeUnreadable`
   - Files: `cmd/ze/hub/mgmt_auth_reload.go`
   - Verify: AC-3 holds, and the two existing fail-closed reload answers are unchanged. spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild edited the same file and closed on 2026-08-12, so read `cmd/ze/hub/mgmt_auth_reload.go` at HEAD before starting
4. **Phase: Fail closed** -- a resolution that cannot answer denies and says so
   - Tests: `TestAPIUserResolutionFailsClosedWhenTheSystemRootIsUnreadable`
   - Files: `cmd/ze/hub/main.go`, `cmd/ze/hub/mgmt_auth_reload.go`
   - Verify: AC-4 holds. An error never becomes an empty list, and the message names the read that failed
5. **Phase: The owner's answer** -- Way 1 or Way 2, whichever this spec records
   - Tests: AC-9 under Way 1, or AC-10 and AC-11 under Way 2
   - Files: `cmd/ze/hub/main.go`, plus the YANG leaf and its env registration under Way 2
   - Verify: the guard's verdict for the affected config set matches the recorded answer, and nothing else changed
6. **Phase: Functional proof and docs**
   - Tests: `test/plugin/mgmt-guard-api-users-without-ssh-block.ci`, and `test/ui/api-user-removed-by-reload.ci` kept green
   - Files: the new `.ci`, `docs/guide/authentication.md`
   - Verify: `make ze-plugin-test` passes, and reverting the resolution change alone turns the new `.ci` red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol, and the two Way-specific AC groups are resolved to whichever the owner chose |
| Feature completeness | Both call sites corrected. A boot-only fix that a SIGHUP undoes is a half fix |
| Correctness | The corrected list holds each user once, and `mergeAuthUsers` precedence is unchanged. The reserved recovery profile still reaches the power user |
| Naming | A new resolution helper, if any, names the `system` root it reads, not the ssh block it replaces |
| Data flow | The boot answer and the per-request answer both come from `ExtractAuthUsers`. No second parser of `system/authentication` exists after the change |
| Rule: `ai/rules/evidence.md` | An empty list from a FAILED read never reaches `buildUserAuthenticator` or `apiAuthed`. The test drives the guard from a config tree, not from `buildUserAuthenticator` alone |
| Rule: `ai/rules/simplicity.md` | No new abstraction. `ExtractAuthUsers` already exists, so the change removes a call rather than adding a layer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The API user resolution no longer reads the SSH extractor | `grep -n 'ExtractSSHConfig' cmd/ze/hub/main.go cmd/ze/hub/mgmt_auth_reload.go` leaves only the SSH server's own use |
| Exactly one reader of `system/authentication` in the resolution path | `grep -rn 'ExtractAuthUsers' cmd/ze/hub/ internal/component/config/infra/`, reviewed against the call sites |
| Both call sites corrected | `make ze-test-pkg PKG=./cmd/ze/hub` with the four new unit tests present |
| The functional chain works | `make ze-plugin-test` with the new `.ci` present |
| The new `.ci` discriminates | Revert only the resolution change and re-run `make ze-plugin-test`; the new `.ci` must go red |
| The live source survived | `make ze-ui-test` keeps `test/ui/api-user-removed-by-reload.ci` green |
| Lint | `make ze-lint-changed` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | A REST or gRPC listener on a non-loopback address that accepts any request, or accepts a request carrying no per-user identity, reaching the same command dispatcher the CLI uses. That is remote command execution against the daemon, config commit paths included |
| What proves it did not | The functional test asserts BOTH halves on the same running daemon: a correct credential is accepted and a wrong password is refused. A test that only proves the daemon starts proves nothing about the gate |
| Fail closed | A resolution error MUST leave `apiAuthed` false and MUST NOT build an authenticator. Trace every error return in the new path and confirm none yields an empty slice with a nil error |
| Empty is not "nobody configured" | Distinguish "the `system` root declares no users" from "the `system` root could not be read", as `liveConfigUsers` already does with its two named errors |
| Guard driven from its entry point | The test starts a daemon from a config tree and lets `checkMgmtListeners` run. Calling `buildUserAuthenticator` with a hand-made list proves the helper, never the wiring |
| Privilege | A config user must not gain the reserved recovery profile, which belongs to the zefs power user alone. Check that the merge does not copy it |
| Authorization unchanged | Authentication says who; `liveAAABundleAuthorizer` still says what they may run. Confirm no profile check is bypassed for a user admitted by the new path |
| No secret in output | The auth-mode line prints a count, never a name or a hash. The guard's refusal prints a remedy, never a token |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| The new `.ci` passes with the fix reverted | The test is vacuous. Rewrite it to assert the credential decision, not the daemon's exit code alone |
| `cmd/ze/hub/mgmt_auth_reload.go` conflicts with the concurrent reload-rebuild spec | Stop and coordinate. Do not merge two designs into that file |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The coupling is one field, not one design. `ExtractSSHConfig` computes its
  `Users` field from the `system` root, which has nothing to do with SSH, and it
  returns before that line when the `environment/ssh` container is absent. Every
  consumer of that field inherits an early return written for the SSH server's
  own settings.
- The web surface reached the same answer by a different route: deleting the
  `ConfigUsers` field left one live source, so no second list remained for a
  serve-or-not check to consult. The API still holds that second list, as
  `apiBuildInputs.Users` beside `apiBuildInputs.UsersLive`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Call `ExtractAuthUsers` over the `system` root at both API resolution sites | Make `ExtractSSHConfig` fill its `Users` field before its early returns | That changes every consumer of `SSHExtractedConfig`, the SSH server's own boot path included, to fix a caller that should not be reading an SSH struct for identity at all |
| Keep `checkMgmtListeners` unedited | Add an API-specific branch to the guard | The guard names no service by design. An API-specific branch would be the first per-service case in a function built to have none |
| Leave the WHO half alone | Rebuild the live source alongside the boot list | The live source already answers per request at both API sites. A second source is the drift this change exists to remove |

## Known Limitations
- The gNMI surface resolves its own token separately in `resolveGNMIListeners`
  and is untouched. It has no per-user mode to couple.
- The looking glass is deliberately outside the guard and stays outside it.
- The reload-time AAA bundle rebuild for remote backends stayed with
  spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild, which closed
  on 2026-08-12. It is done, and `cmd/ze/hub/mgmt_auth_reload.go` holds it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated, with the Way-specific rows resolved to the owner's answer
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`cmd/ze/hub/main.go`, `cmd/ze/hub/mgmt_auth_reload.go`), not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: scope is config, no wire format)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

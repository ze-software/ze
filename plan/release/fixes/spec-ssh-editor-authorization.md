# Spec: ssh-editor-authorization -- The SSH Session Editor Honours the Authorization Profile

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P0 |
| Release reason | A fail-open boundary: a user whose profile denies configuration changes can commit one through the SSH session editor. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

On a daemon with `system.authorization` configured, a user whose profile
denies the edit section can open the SSH session editor, enter config mode,
change the configuration and commit. The profile is never consulted on that
path. For the daemon's own configuration the commit reloads the daemon. For a
session that names another stored configuration, the commit writes that file
in the store, a hub's client configurations included.

Every other surface that writes configuration does check the profile: the web
editor and web terminal, the REST and gRPC config sessions, and the SSH config
upload. The SSH session editor is the one that does not.

The goal: a config-mode action in the SSH session editor is authorized against
the user's profile, verb by verb, with the same permission strings the web
terminal, the web editor and the API use, and a refused user is told so in the
access-control wording the dispatcher uses. A session that names another stored
configuration is authorized with the same strings followed by the
configuration's name, so an existing entry still covers it and an operator can
write an entry for one file.

Found on 2026-11-19 while tracing the hub's session editor for
`plan/release/fixes/spec-fleet-6-config-freeze.md`. It exists with or without
fleets. Read at source, not run: the first implementation step is the
reproduction below, red before any fix.

The sample `operator` profile in `docs/guide/operator-access-rbac.md` allows
`configure` and `commit` in the edit section. Neither prefix matches a string
any surface authorizes (`config set`, `config commit`, ...), so that operator
can change nothing on the web, the API or the fixed SSH editor. The sample is
corrected in this work (a small defect found in passing, `ai/META-DECISIONS.md`
"How agents work").

A second small defect, found while running the tests, is fixed here too
(AC-15). Loading a profile whose edit entry is `config`, `config set`, `config
commit` or `config commit client-edge.conf` logged `authz match entry does not
match any known command`, because `validateMatchEntries`
(`internal/component/config/infra/authz.go`) compared entries only with the
command registry, and the config permission strings are not commands. The
corrected guide sample, and every profile written as the guide documents it,
warned at each load. Now `unknownMatchEntries` also accepts an entry that
reaches a string of `plugin.ConfigAuthStrings` at a word boundary.

## Owner Decisions

| # | Question | Answer | Source |
|---|----------|--------|--------|
| Q-1 | One gate at session open, or a check per config-mode verb? | Per verb. Navigation and display (`edit`, `up`, `top`, `show`, `who`, `option`, `help`) stay ungated, as in the web terminal | Settled by the existing web terminal mapping (`internal/component/web/cli_terminal.go` `terminalAuthCommand`), which authorizes the same command language verb by verb; `docs/architecture/web-interface.md` |
| Q-2 | What a missing authorizer means | Fail closed: every config-mode mutation is refused | Settled by the SSH transport's own policy: `cmd/ze/hub/service_ssh.go` `sshSessionPolicy` replaces a missing session authorizer with one that denies everything for every operational executor, and `cmd/ze/hub/config_upload_ssh.go` refuses an upload without one. A daemon with no `system.authorization` still gets an authorizer that allows, so permissive mode is unchanged |
| Q-3 | Whether naming another stored configuration needs its own permission | No separate permission. A session on a named configuration authorizes each verb with the shared string followed by the configuration name (`config commit client-edge.conf`). Prefix matching keeps every existing entry covering it, and an entry naming the file restricts that file alone | Thomas, 2026-11-19, at the design gate |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/authorization.md` - what each profile section promises
  → Constraint: the edit section judges every command whose verb is not a read verb, and the built-in `read-only` profile denies the whole edit section, so every config permission string is judged with `isReadOnly=false`
  → Constraint: prefix matching is case-insensitive with a word boundary, so `config commit` covers `config commit client-edge.conf` and `config` covers every config permission. The name suffix of Q-3 relies on this
  → Decision: the section "What string gets matched" says CLI-entered strings are what the user typed, `configure` included. That is wrong for config mode: `configure` is handled locally and never authorized. The page gains the config-mode permission table and loses the `configure` example
- [ ] `docs/guide/operator-access-rbac.md` - the sample operator profile
  → Constraint: its `operator` profile allows `configure` and `commit`, which match no authorized string. It is rewritten to allow `config` in the edit section and keep the rest denied, and its test section gains a config-mode check
- [ ] `docs/architecture/api/process-protocol.md` - the `// Design:` page of `internal/component/plugin/types.go`, where the nine strings now live
  → Constraint: the strings are Go constants of the daemon's authorizer calls, not plugin protocol wire; the page describes no config permission string and gains none
- [ ] `docs/architecture/config/syntax.md` - the `// Design:` page of `internal/component/config/infra/authz.go` (daemon-startup authorization extraction)
  → Constraint: the profile syntax is unchanged; AC-15 changes only which prefix entries the load-time check reports as unknown, a warning, never a refusal
- [ ] `ai/doors/commands-cli.md` - editor and SSH boundaries
  → Constraint: trap "One surface fixed, the other not" (the web CLI and SSH `set` diverged once). The verb-to-permission mapping is declared once and both surfaces read it
- [ ] `ai/doors/access-security.md` - who can run what
  → Constraint: the session authorizer is the accepted generation's policy bound at authentication (`aaa.AuthorizerForResult`); the editor uses that bound authorizer, never the daemon's live fallback
- [ ] `ai/doors/api-surfaces.md` - web and API config sessions
  → Constraint: the web and API surfaces keep their own strings and refusals unchanged; only the declaration of the strings moves so the CLI can read them
- [ ] `ai/doors/hub-engine.md` - session factory wiring
  → Constraint: `cmd/ze/hub/session_editor.go` is untagged and shared with the attached console, which authenticates nobody; the authorization hook belongs to the SSH factory, never to `newSessionEditor`
- [ ] `ai/doors/plugin-infra.md` - the dispatcher the refusal mirrors
  → Constraint: `pluginserver.ErrUnauthorized` carries `PermissionDenied()` so adapters classify a refusal without comparing text; the editor refusal wraps it rather than inventing a second refusal type

**Key insights:**
- The authorizer reaches the session factory and is handed to the operational command executor only. The editor package holds no authorizer.
- Every config-mode command, including `top <cmd>` and the operational-mode shortcut that switches to config mode on `set`, reaches one function, `dispatchCommandWords`. Pipes accept `show` only. One check there covers every path.
- `configure` itself never reaches an authorizer; it only switches mode.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` - `setSessionAuthorizer`, `getSessionAuthorizer`, `teaHandler`: authentication binds one authorizer to the connection (nothing when the server has none) and passes it to the session model factory
- [ ] `internal/component/ssh/session.go` - `parseSessionRequest`: accepts a config name that is one plain filename and a mode of `edit` or `command`; authorizes nothing
- [ ] `cmd/ze/hub/session_factory.go` - `buildSessionModelFactory`: uses the authorizer once, in `srv.ExecutorForUser`; it is not given to the editor or the model. For a named configuration, fleet-6's declared-client refusal (`undeclaredClientSessionRefusal`, in flight in the working tree) runs, then an existence check; the commit is store-only
- [ ] `cmd/ze/hub/session_editor.go` - `newSessionEditor`: `cli.ValidateUser` checks the characters of the user name; shared with `attachedConsoleEditor`
- [ ] `cmd/ze/hub/service_ssh.go` - `sshSessionPolicy`: a nil session authorizer becomes `sshMissingSessionPolicy`, which denies every command
- [ ] `internal/component/cli/model_commands.go` - `dispatchCommandWords`: routes `set`, `delete`, `commit`, `rollback`, `load`, `save`, `discard`, `activate`, `deactivate`, `rename`, `copy`, `insert` and `disconnect` to their handlers, guarded only by "an editor exists"
- [ ] `internal/component/cli/model_keys.go` - `handleEnter`: `configure` and a config verb typed in operational mode switch to config mode and fall through to the same dispatch
- [ ] `internal/component/cli/model.go` - `Model` holds the operational `commandExecutor` and no authorizer; `SetCommandExecutor` is the pattern for a setter the hub calls
- [ ] `internal/component/cli/model_commands_commit.go` - `cmdCommitRequest`, `runCommitSession`: validates, commits, notifies the reload; no profile check
- [ ] `internal/component/web/cli_terminal.go` - `terminalAuthCommand`: the web terminal's verb-to-permission mapping, the model to share
- [ ] `internal/component/web/handler_config.go` - `authorizeWebConfigMutation` and the `webCommandConfig*` strings, four of which (`config add`, `config rename`, `config rollback`, `config save`) are web-local literals
- [ ] `internal/component/web/rbac.go` - `canEdit`: the web route gate authorizes `config commit` as the representative edit permission
- [ ] `cmd/ze/hub/config_upload_ssh.go` - `registerSSHConfigUpload`: authorizes `config commit` and fails closed on a missing authorizer
- [ ] `internal/component/api/types.go` - `ConfigAuthEdit`, `ConfigAuthSet`, `ConfigAuthDelete`, `ConfigAuthCommit`, `ConfigAuthDiscard`
- [ ] `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go` - `requireWriteAccess`: the API's check
- [ ] `internal/component/plugin/server/command.go` - `ErrUnauthorized`, `unauthorizedError`: the dispatcher's refusal is `plugin.UnauthorizedMessage` followed by the command

**Behavior to preserve:**
- A daemon with no `system.authorization` configured stays permissive, as documented
- The attached console (`ze start --cli`) and the offline `ze config edit` authenticate nobody and keep every action
- A user whose profile allows editing keeps every config-mode action, on the daemon's own configuration and on a named one
- A read-only user keeps config-mode navigation and display: `edit`, `up`, `top`, `show`, `who`, `option`, `help` and `run <operational command>` (the last still authorized by the executor)
- The web, REST, gRPC and SSH upload refusals and the strings they authorize are unchanged
- fleet-6's declared-client refusal for a named configuration stays and runs before any profile check

**Behavior to change:**
- A config-mode mutation in the SSH session editor is refused for a user whose profile denies its permission string, with no change to the candidate, the store or the running configuration
- A config-mode mutation in an SSH session with no session authorizer is refused
- The sample `operator` profile in the RBAC guide allows `config`

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Web terminal verb mapping | `internal/component/web/cli_terminal.go` `terminalAuthCommand` | Maps each config-mode verb of the CLI language to a permission string; navigation is ungated; `top <cmd>` is judged as its command | Extends: the mapping moves to the CLI package that owns the verbs, and the web terminal reads it from there. One declaration for both surfaces |
| Config permission strings | `internal/component/api/types.go` `ConfigAuth*`; `internal/component/web/handler_config.go` `webCommandConfig*` | Five shared strings in the API package, four more as web-local literals | Extends: all nine are declared once, in one package both the CLI and the web read; the web literals are deleted |
| Web config authorization | `internal/component/web/handler_config.go` `authorizeWebConfigMutation` | Authorizes each web config action by its string | Does not change: the web keeps its own refusal path |
| API config authorization | `requireWriteAccess` in `rest/auth.go` and `grpc/server.go` | Authorizes config sessions | Does not change |
| SSH session policy | `cmd/ze/hub/service_ssh.go` `sshSessionPolicy` | Turns a missing session authorizer into deny-all for the operational executors | Extends: the editor gets the same policy, so the two SSH paths share one fail-closed rule |
| SSH upload authorization | `cmd/ze/hub/config_upload_ssh.go` `registerSSHConfigUpload` | Authorizes a commit and fails closed with no authorizer | Precedent on the same transport; unchanged |
| Session authorizer | `internal/component/ssh/ssh.go` `getSessionAuthorizer` | Carries the bound profile to the session factory | Extends: the factory passes it on to the editor model |
| Dispatcher refusal | `internal/component/plugin/server/command.go` `ErrUnauthorized`, `plugin.UnauthorizedMessage` | "command restricted by access control" plus the command, classified by `PermissionDenied()` | Extends: the editor refusal wraps `ErrUnauthorized`, names the permission string, and adds a fix line |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An authenticated SSH connection opens an interactive session (`teaHandler`), optionally naming a stored configuration and a mode
- Each line the operator enters in the editor

### Transformation Path
1. Authentication binds the profile's authorizer to the connection (`setSessionAuthorizer`)
2. `buildSessionModelFactory` receives it, wraps it with `sshSessionPolicy`, and installs on the editor model a config authorizer that knows the user, the remote address and, for a named configuration, its name
3. `handleEnter` switches mode where needed and queues the line
4. `dispatchCommandWords` looks up the verb's permission string; a verb with none (navigation, display) goes on unchanged
5. For a verb with a permission string and a config authorizer installed, the model asks the authorizer with the string (plus the name for a named configuration) in the edit section; a refusal returns before the handler runs
6. Otherwise the handler runs as today

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SSH server ↔ hub session factory | `SessionModelFactory` already carries the authorizer | No |
| Hub ↔ CLI editor model | A setter on the model taking a function of the permission string; the CLI package imports no AAA package | No |
| CLI ↔ web | The web terminal reads the CLI's mapping | No |

### Changed-Wire Census

The changed contract is the set of permission strings handed to an authorizer
for configuration changes: operators' profile entries and a TACACS+ server's
command authorization read them.

**Search evidence:** scope `internal/`, `cmd/`, `test/`, `docs/`, Go and `.ci`/`.md`; terms `ConfigAuth`, `webCommandConfig`, `"config add"`, `"config rename"`, `"config rollback"`, `"config save"`, `match config`, `match commit`, `match configure`; positive control: `cmd/ze/hub/config_upload_ssh.go` authorizes `api.ConfigAuthCommit` and was found by the `ConfigAuth` term.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Config permission string | writer | `internal/component/web/handler_config.go` `webCommandConfig*`, read by `cli_terminal.go` `terminalAuthCommand` and the web config handlers | grep `webCommandConfig` | Preserve every string byte for byte; only the declaration moves |
| Config permission string | writer | `internal/component/api/rest/server.go` and `grpc/server.go` `requireWriteAccess` callers | grep `ConfigAuth` | Preserve |
| Config permission string | writer | `cmd/ze/hub/config_upload_ssh.go` `registerSSHConfigUpload` | grep `ConfigAuth` | Preserve |
| Config permission string | writer (new) | the SSH editor model's dispatch gate, bound by `cmd/ze/hub/session_factory.go` | new | New writer: same strings, plus the configuration name for a named configuration |
| Config permission string | reader | `internal/component/authz` `Store.authorize`, prefix and regex matching | `docs/guide/authorization.md` "Matching rules" | Preserve; the name suffix is matched by the existing word-boundary prefix rule |
| Config permission string | reader | TACACS+ command authorization (`internal/component/tacacs`), sent to the operator's server | `docs/architecture/aaa-tacacs.md` | Preserve; the server receives the same strings it receives from the web today |
| Config permission string | reader | Operator profile entries in `docs/guide/operator-access-rbac.md` | grep `match commit` | Migrate: the sample becomes `match config` |
| Config permission string | reader | RBAC fixtures `internal/test/fixture/plugin_fixture_12_rbac.go` and `test/plugin/*rbac*.ci` | grep `"config add"` | Preserve; they assert web strings that do not change |

### Integration Points
- `cli.Model` gains a config authorizer setter; nil means no check (attached console, offline editor)
- `buildSessionModelFactory` always installs one for an SSH session
- `terminalAuthCommand` in the web package derives from the CLI mapping

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The check sits in the one dispatch function every config-mode line reaches |
| No unintended coupling (components stay isolated) | Yes | The CLI receives a function, not an AAA type; the hub binds it |
| No duplicated functionality (extends existing, does not recreate) | Yes | The web mapping and strings move; nothing is copied |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding |
| Registration over hardcoding, outbound | Yes | No new feature registry; the mapping is keyed by the editor's own verbs, which the CLI package owns |
| Registration over hardcoding, inbound | Yes | Searched `dispatchCommandWords`, `terminalAuthCommand`, `handleEnter`; the new verb-to-permission table lives beside the verb constants in the CLI, so a new editor verb is added in one package. A verb with no entry is ungated, which a unit test over every mutating handler refuses (TDD plan) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The defect is real at runtime, not only in the reading | Source read on 2026-11-19: the authorizer stops at the session factory | The spec is obsolete | `TestSSHEditorAuthorizesConfigVerbsOverPTY`, run red before the fix | confirmed: with the factory's `SetConfigAuthorizer` call removed, `TestSSHEditorAuthorizesConfigVerbsOverPTY`, `TestSSHEditorAuthorizesNamedConfigurationOverPTY`, `TestSessionFactoryFailsClosedWithoutAuthorizer` and `test/plugin/session-editor-authorization.ci` go red; restored, green |
| A-2 | No other layer refuses the session before the editor is built | No profile check between authentication and the editor was found | The defect is narrower than stated | Same test, red on the denied user's commit | confirmed: the same red proof as A-1; nothing between authentication and the editor refused the denied user |
| A-3 | Every config-mode mutation reaches `dispatchCommandWords` | `handleEnter` falls through to `executeCommand`; `top` recurses; pipes accept `show` only | A path bypasses the gate | `TestEveryMutatingEditorVerbHasAPermission` plus the `top` and operational-shortcut rows of the PTY test | confirmed: `TestEditorGateRefusesBeforeHandler` covers `top set`, `top top set` and the operational shortcut; `TestEveryMutatingEditorVerbHasAPermission` reads the dispatch switch |
| A-4 | The CLI package can read the permission strings without an import cycle or a tier refusal | `internal/component/cli` already imports `internal/component/plugin` and `plugin/server`; `internal/component/api` imports neither `cli` nor `web` | The strings move to `internal/component/plugin` instead of staying in `api` | `./le arch tier check` after the move | broken, fallback taken: `docs/architecture/core-design.md` section 19 does not allow `cli` to import `api`, so the nine strings moved to `internal/component/plugin/types.go` and `api.ConfigAuth*` was deleted |
| A-5 | A nil authorizer reaches the factory only when the SSH server was built without one | `setSessionAuthorizer` stores nothing for nil; the hub passes the AAA bundle's authorizer | A permissive daemon would lose editing | AC-10 test with no `system.authorization` | confirmed: the AC-10 subtest of the PTY test is green with no `system.authorization` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fix that gates navigation locks a read-only user out of `show` in config mode | AC-1 survivor rows fail | Only verbs with a permission string are gated; AC-1 asserts the survivors |
| R-2 | Existing session factory tests pass a nil authorizer and mutate, so fail closed turns them red | `cmd/ze/hub/session_factory_test.go` red on set or commit | Those tests pass an allowing authorizer, which keeps what they prove (their subject is not authorization); judged by `ze-test-judge` if the weakening detector flags it |
| R-3 | fleet-6 is changing `cmd/ze/hub/session_factory.go` concurrently (its declared-client refusal is already in the working tree) | A conflicting hunk in the same function | The authorizer installation is one statement after the model is built; commit whole files per the shared-checkout rule and keep fleet-6's refusal first |
| R-4 | A verb added later without a permission entry is silently ungated | New handler in `dispatchCommandWords` | `TestEveryMutatingEditorVerbHasAPermission` enumerates the dispatch's mutating verbs and refuses one with no entry |
| R-5 | The refusal is printed as a generic error and the operator cannot tell it from a parse error | PTY test output | AC-1 asserts the access-control message and the permission string |
| R-6 | The confirmed-commit timer's automatic rollback is refused for a user who lost the commit permission mid-window | Window expiry leaves the candidate committed | The automatic rollback is the daemon's own action, not a typed verb, and does not pass through `dispatchCommandWords`; AC-13 asserts it still runs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator whose profile should allow editing over SSH is refused; or a denied operator still commits |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-fleet-6-config-freeze.md` (session factory, named configurations); the web terminal (shared mapping) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH PTY session, edit mode, denying profile, typed `set` and `commit` | → | `buildSessionModelFactory` installs the authorizer; `dispatchCommandWords` refuses | `TestSSHEditorAuthorizesConfigVerbsOverPTY` |
| SSH PTY session naming a stored configuration | → | the same gate with the name suffix | `TestSSHEditorAuthorizesNamedConfigurationOverPTY` |
| Web CLI bar `set` | → | `terminalAuthCommand` reading the shared mapping | `TestTerminalAuthCommandReadsSharedMapping`, with the existing web terminal authorization tests unchanged and green |
| Running daemon, `le test fixture plugin/session-editor` driving `ze config edit` over a PTY | → | the SSH session factory and the dispatch gate | `test/plugin/session-editor-authorization.ci` |
| Profile load (`extractAuthzConfig`, `internal/component/config/infra/authz.go`) | → | `validateMatchEntries` reading `unknownMatchEntries` | `TestUnknownMatchEntriesKnowsConfigPermissions` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | User with a `read-only` profile (as the built-in describes: the edit section denied), SSH session in edit mode on the daemon's configuration, types `set system host-name x` | The line answers with an error that opens with "command restricted by access control", names `config set`, and ends with a fix line; the candidate shows no change | `edit`, `up`, `top`, `show`, `who`, `option`, `help` and `run show version` answer as before, because reading the configuration is not an edit and the web terminal leaves them ungated |
| AC-2 | Same user types `commit` | Refused with the same message naming `config commit`; the running configuration and the store are unchanged | Navigation as in AC-1 |
| AC-3 | Same user in command mode types `set system host-name x` (the shortcut into config mode) | The session moves to config mode and the line is refused as in AC-1 | The mode switch itself, because `configure` changes nothing |
| AC-4 | Same user types `top set system host-name x` | Refused as in AC-1 | `top show` answers, judged as `show` |
| AC-5 | Each mutating editor verb: `set`, `activate`, `delete`, `deactivate`, `commit`, `discard`, `disconnect`, `rollback`, `rename`, `copy`, `insert`, `save`, `load` | Each maps to the permission string the web terminal used for it before this change (`config set`, `config delete`, `config commit`, `config discard`, `config rollback`, `config rename`, `config add`, `config save`), and the web terminal and the SSH editor read the same table | Every other verb maps to no permission, because none of them changes the candidate or the store |
| AC-6 | User with an `admin` profile (as the built-in describes: every section allowed), same session, `set system host-name x` then `commit` | Both succeed and `show config running` over SSH exec reports host-name `x` | None refused, because the profile allows the edit section |
| AC-7 | Profile whose edit section is `default-action deny` with allow entries `config set` and `config commit` | `set` and `commit` succeed and the running configuration changes; `delete system host-name` is refused naming `config delete` | `set` and `commit` survive, because the profile allows exactly them: per-verb granularity |
| AC-8 | Session naming stored configuration `client-edge.conf` (declared to fleet-6's check), user whose profile allows the edit section by default with a deny entry `config commit client-edge.conf` | `set` succeeds in the candidate; `commit` is refused naming `config commit client-edge.conf`; the stored bytes of `client-edge.conf` are unchanged; in a second session on the daemon's own configuration the same user's `commit` succeeds | The daemon's own configuration and `set` on the named one survive, because the deny entry names the file and the commit verb only |
| AC-9 | `admin` user, session naming `client-edge.conf`, `set` then `commit` | The stored bytes of `client-edge.conf` change to include the edit | None refused |
| AC-10 | Daemon with no `system.authorization`, any authenticated user, `set` then `commit` | Both succeed and the running configuration changes | Every verb, because an unconfigured daemon is documented as permissive |
| AC-11 | SSH session whose connection carries no session authorizer | Every mutating verb is refused with the access-control message | Navigation and display, as in AC-1, because they change nothing |
| AC-12 | Attached console (`ze start --cli`), and `ze config edit` in file mode (`-f`) or backup mode (`--backup`), `set` then `commit` | Both succeed as today | Every verb, because those surfaces authenticate nobody and run as the local user. Plain `ze config edit` is not among them: it opens an SSH session (`runStoredEditor`) and is authorized like one |
| AC-13 | `admin` user runs `commit confirmed 1`, then the profile is reloaded to deny the edit section, then the window expires | The daemon rolls back to the previous configuration | The automatic rollback, because it is the daemon's action and no verb was typed |
| AC-14 | The RBAC guide's corrected `operator` profile, applied as written | Over SSH that user can `set` and `commit` a change in config mode, and is refused `clear interface counters all` | The `run` section's allows, as the sample states |
| AC-15 | A profile whose edit entries are `config`, `config set`, `config commit` and `config commit client-edge.conf` is loaded | The profile loads, the four entries are reported known by `unknownMatchEntries`, and they authorize as written (a session's `config commit client-edge.conf` is matched by the `config commit client-edge.conf` entry) | A real unknown entry (`confg set`, `config frobnicate`, `config commits`) still warns, because the check exists to catch typos |

The built-in `read-only` and `admin` profiles (`builtinReadOnlyProfile`,
`builtinAdminProfile` in `internal/component/authz/authz.go`) are never
installed outside tests, so the live store holds only the profiles the
configuration declares. The tests of AC-1 and AC-6, the Go PTY tests and the
`.ci` alike, therefore declare `read-only` and `admin` in their configuration.
That no shipped path installs the built-ins is journal `unwired-feature`
2026-09-05 (the `builtinReadOnlyProfile` row), owned by
`plan/release/fixes/spec-wire-or-delete-unreached-code.md`.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A read-only NOC user connects with `ze cli`, browses config mode and tries to change the host name | SSH auth → session factory → editor model → dispatch gate → refusal | `TestSSHEditorAuthorizesConfigVerbsOverPTY` |
| 2 | An administrator edits a hub's client configuration by name and commits | SSH auth → factory with name → gate with suffix → store write | `TestSSHEditorAuthorizesNamedConfigurationOverPTY` |
| 3 | An operator with the guide's sample profile changes and commits a value | SSH auth → gate with `config` allowed → commit → reload | `TestSSHEditorAuthorizesConfigVerbsOverPTY` (operator case) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryMutatingEditorVerbHasAPermission` | `internal/component/cli/config_permission_test.go` | AC-5: every verb `dispatchCommandWords` routes to a mutating handler has a permission string, and the strings equal the web terminal's former ones | |
| `TestEditorGateRefusesBeforeHandler` | `internal/component/cli/config_permission_test.go` | AC-1, AC-3, AC-4: a denying authorizer refuses `set`, the operational shortcut and `top set`; the editor's working content is unchanged; the refusal carries the message and the string and classifies as permission denied | |
| `TestEditorGateAppendsConfigurationName` | `internal/component/cli/config_permission_test.go` | AC-8: with a named configuration the authorizer receives the string followed by the name | |
| `TestEditorGateLeavesNavigationUngated` | `internal/component/cli/config_permission_test.go` | AC-1 survivors: `show`, `edit`, `up`, `top show`, `who` never call the authorizer | |
| `TestEditorWithoutAuthorizerKeepsEveryVerb` | `internal/component/cli/config_permission_test.go` | AC-12: a model with no config authorizer runs `set` and `commit` | |
| `TestSessionFactoryFailsClosedWithoutAuthorizer` | `cmd/ze/hub/session_factory_test.go` | AC-11: a nil session authorizer refuses `set` | |
| `TestTerminalAuthCommandReadsSharedMapping` | `internal/component/web/cli_terminal_test.go` | AC-5: the web terminal's answer for every verb equals the CLI table | |
| `TestUnknownMatchEntriesKnowsConfigPermissions` | `internal/component/config/infra/authz_match_test.go` | AC-15: `config`, `config set`, `config commit` and `config commit client-edge.conf` are known entries | |
| `TestUnknownMatchEntriesReportsUnknownEntry` | `internal/component/config/infra/authz_match_test.go` | AC-15 survivors: a misspelt or invented entry is still reported | |
| `TestUnknownMatchEntriesNeedsWordBoundary` | `internal/component/config/infra/authz_match_test.go` | AC-15 survivors: `config commits` does not reach `config commit` at a word boundary and is reported | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestSSHEditorAuthorizesConfigVerbsOverPTY` | `cmd/ze/hub/session_authz_test.go` (`ze_ssh`) | Real SSH server, real local authz store with `admin`, `read-only`, the AC-7 profile and the guide's corrected `operator`; PTY sessions type the AC-1 to AC-7, AC-10, AC-14 lines and assert the screen and the store | |
| `TestSSHEditorAuthorizesNamedConfigurationOverPTY` | `cmd/ze/hub/session_authz_test.go` (`ze_ssh`) | AC-8 and AC-9 through a PTY session that sets the configuration name | |
| `TestConfirmedRollbackSurvivesLostPermission` | `cmd/ze/hub/session_authz_test.go` (`ze_ssh`) | AC-13 | |
| `test/plugin/rbac-ssh-only-enforced.ci` | existing | Header updated: it no longer says interactive config mode is untested, and points at the Go PTY tests | |
| `test/plugin/session-editor-authorization.ci` | new, promoted from `test/draft/plugin/` after its draft and stress runs | AC-1, AC-2, AC-6 against a running daemon: `viewer` (profile `read-only`, edit default deny) types `set bgp router-id 9.9.9.9` and `commit now` in the SSH editor and both are refused naming `config set` and `config commit`, while `show bgp` still answers and `ze cli -c "show bgp"` keeps 1.2.3.4; `admin` (profile `admin`) sets and commits 5.6.7.8 and `show bgp` reports it; the saved file holds 5.6.7.8 and never 9.9.9.9 | |

A `.ci` reaches the interactive editor through
`le test fixture plugin/session-editor` (`docs/functional-tests.md`, "Driving
the SSH session editor from a script"), which opens `ze config edit` over a PTY
against the running daemon, so the new `.ci` proves AC-1, AC-2 and AC-6 end to
end. The Go PTY tests stay for what the fixture cannot express: the named
stored configuration (AC-8, AC-9) and the confirmed-commit rollback after the
profile is reloaded (AC-13). They run the real SSH server, the real session
factory and the real authorization store over a real SSH connection (the
harness: `cmd/ze/hub/infra_history_owner_test.go` opens a PTY session against
a built SSH server). The profiles AC-1 and AC-6 use are declared in each
test's configuration (see the note under the Acceptance Criteria).

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Not a protocol change; the TACACS+ server receives the strings the web already sends | |

## Files to Modify
- `internal/component/cli/model_commands.go` - the gate in `dispatchCommandWords`, before the verb switch and after the `top` recursion
- `internal/component/cli/model.go` - the config authorizer field and setter; the configuration name it appends
- `internal/component/web/cli_terminal.go` - `terminalAuthCommand` derives from the CLI table
- `internal/component/web/handler_config.go` - the `webCommandConfig*` literals give way to the shared declaration
- `internal/component/plugin/types.go` - the nine config permission strings (`ConfigAuth*`) and `ConfigAuthStrings`, beside `UnauthorizedMessage` (A-4 broken, fallback taken)
- `internal/component/api/types.go` - the `ConfigAuth*` block deleted
- `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go` - read `plugin.ConfigAuth*`
- `cmd/ze/hub/config_upload_ssh.go`, `cmd/ze/hub/config_upload_ssh_test.go`, `cmd/ze/hub/auth_e2e_test.go` - read `plugin.ConfigAuth*`
- `internal/component/cli/model_test.go` - `knownModelFields` names the model's two new fields, `configAuthorizer` and `configName`
- `internal/component/config/infra/authz.go` - `unknownMatchEntries` knows the config permission strings (AC-15)
- `cmd/ze/hub/session_factory.go` - install the authorizer (wrapped by `sshSessionPolicy`) with the user, the remote address and the configuration name
- `cmd/ze/hub/session_factory_test.go` - mutating cases pass an allowing authorizer (R-2)
- `test/plugin/rbac-ssh-only-enforced.ci` - header only
- `docs/guide/authorization.md` - config-mode permission table, the name suffix, the `configure` correction
- `docs/guide/operator-access-rbac.md` - the `operator` sample and its test step
- `docs/guide/config-editor.md` - a refused config-mode verb and what to do
- `docs/architecture/hub-architecture.md` - the session factory's `// Design:` page: the editor authorizer
- `ai/digests/cli-editor.md` - the dispatch flow gains the gate
- `docs/architecture/config/yang-config-design.md` - the `// Design:` page of `internal/component/cli/model_commands.go`: its config editor section gains the per-verb authorization of an SSH session
- `docs/architecture/api/architecture.md` - the `// Design:` page of `internal/component/api/types.go` and `internal/component/api/rest/auth.go`: the config session permission strings are now shared with the editor and the web terminal, and the four former web literals join them
- `docs/architecture/cli/error-surface.md` - the editor's access-control refusal
- `docs/architecture/web-interface.md` - the web terminal reads the editor's verb table
- `docs/architecture/appliance/remote-operations.md` - the SSH upload's permission string now comes from `plugin`
- `ai/digests/api-ipc.md` - the strings moved out of `api`

## Files to Create
- `internal/component/cli/config_permission.go` - the verb-to-permission table, the name suffix, the refusal
- `internal/component/cli/config_permission_test.go` - unit tests above
- `cmd/ze/hub/session_authz_test.go` - the PTY tests
- `internal/component/config/infra/authz_match_test.go` - the AC-15 tests
- `test/plugin/session-editor-authorization.ci` - AC-1, AC-2 and AC-6 against a running daemon

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added; profiles already exist |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added; existing editor verbs gain a check |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | Completion is unchanged; a denied verb still completes and is refused on Enter, as the web terminal does |
| Functional test for new RPC/API | N-A | No RPC; the PTY tests are the end-to-end proof |
| Pipe completeness | N-A | No output added |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Doctor code per problem class | N-A | A refusal is the profile's verdict, not a fault on the box; it adds no log line at `Warn` or `Error` |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the `features/` entry for authorization, if it names the surfaces enforced; `./le feature report` decides |
| 2 | Config syntax changed? | No | Profiles unchanged |
| 3 | CLI command added/changed? | No | Verbs unchanged |
| 4 | API/RPC added/changed? | No | Strings preserved |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | Yes | `docs/guide/authorization.md`, `docs/guide/operator-access-rbac.md`, `docs/guide/config-editor.md` |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | Uses the existing PTY harness |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/hub-architecture.md`, `ai/digests/cli-editor.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/release/fixes/spec-ssh-editor-authorization.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `operator` sample in `docs/guide/operator-access-rbac.md` |

## Implementation Steps

1. **Phase: Reproduction and wiring (first)** -- write `TestSSHEditorAuthorizesConfigVerbsOverPTY` and `TestSSHEditorAuthorizesNamedConfigurationOverPTY` against today's code; they must go red on the denied user's `set` and `commit` (validates A-1, A-2). Add the empty config authorizer setter so the factory compiles against it
   - Tests: the two PTY tests
   - Files: `cmd/ze/hub/session_authz_test.go`, `internal/component/cli/model.go`
   - Verify: red for the denied rows, green for admin rows
2. **Phase: Shared permission table** -- declare the nine strings once, the verb table in the CLI package, the web terminal reading it
   - Tests: `TestEveryMutatingEditorVerbHasAPermission`, `TestTerminalAuthCommandReadsSharedMapping`, existing web terminal tests
   - Files: `config_permission.go`, `plugin/types.go` (strings), `api/types.go` (block deleted), `api/rest/server.go`, `api/grpc/server.go`, `cmd/ze/hub/config_upload_ssh.go` and its test, `cmd/ze/hub/auth_e2e_test.go`, `web/cli_terminal.go`, `web/handler_config.go`
   - Verify: web tests unchanged and green; `./le arch tier check`
3. **Phase: The gate** -- check in `dispatchCommandWords`, refusal text, name suffix
   - Tests: the unit tests of `config_permission_test.go`
   - Files: `model_commands.go`, `config_permission.go`
4. **Phase: Factory binding** -- install the wrapped authorizer and the configuration name in `buildSessionModelFactory`; adjust R-2 tests
   - Tests: PTY tests now green, `TestSessionFactoryFailsClosedWithoutAuthorizer`, `TestConfirmedRollbackSurvivesLostPermission`
   - Files: `session_factory.go`, `session_factory_test.go`
5. **Phase: Docs** -- the guide pages, the architecture page, the digest and the `.ci` header, each edited as the phase that makes it wrong lands
6. **Phase: Profile entry check (AC-15)** -- `unknownMatchEntries` accepts an entry that reaches a config permission string at a word boundary
   - Tests: `TestUnknownMatchEntriesKnowsConfigPermissions`, `TestUnknownMatchEntriesReportsUnknownEntry`, `TestUnknownMatchEntriesNeedsWordBoundary`
   - Files: `internal/component/config/infra/authz.go`, `authz_match_test.go`
7. **Phase: Functional test** -- `test/plugin/session-editor-authorization.ci`, drafted under `test/draft/plugin/`, promoted after its draft and stress runs

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | The gate runs before any handler touches the editor, and after `top` resolves its inner command; a refused line leaves the candidate byte-identical |
| Naming | The nine strings are byte-identical to today's web and API strings |
| Data flow | The CLI package imports no AAA package; the hub binds the authorizer |
| Operator errors and doctor codes | The refusal is an `opererr` error wrapping `pluginserver.ErrUnauthorized`, opening with `plugin.UnauthorizedMessage`, naming the string, with a fix on the last line |
| Rule: no-layering | The web literals are deleted, not kept beside the shared table |
| Rule: principles, declare once | One verb table, read by both surfaces |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| SSH editor refuses denied verbs | `./le test unit pkg path ./cmd/ze/hub/ run TestSSHEditorAuthorizes` |
| Shared mapping | `./le test unit pkg path ./internal/component/cli/ run TestEveryMutatingEditorVerbHasAPermission` and `./le test unit pkg path ./internal/component/web/` |
| No web literal left | grep for `"config add"` in `internal/component/web` returns only the shared reference |
| Corrected sample | grep `match configure` in `docs/` returns nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail open | A nil session authorizer on SSH denies; a nil config authorizer on the model is reachable only from the console and the offline editor |
| Name injection | The configuration name is the one `parseSessionRequest` already restricts to a single plain filename, so it adds exactly one token to the permission string |
| Bypass paths | `top`, the operational shortcut, pipes, pasted multi-line input (the dispatch queue runs each line through the same function) |
| Bound policy | The editor uses the authorizer bound at login, never the live fallback (`ai/doors/access-security.md`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The editor's Update goroutine reads the editor while a dispatched command mutates it: the race detector reports it on any commit typed over. `dispatchQueue` orders commands, not Update's reads. Recorded in `plan/journal/per-request-state-on-a-shared-object.md`, 2026-11-19, owned by `plan/release/fixes/spec-web-cli-config-one-off-defects.md`. The PTY tests wait for each answer before typing on, so they do not trip it.
- `plugin.ConfigAuthStrings()` lists the nine constants a second time in the same file, because Go cannot enumerate constants. A constant added without its slice entry would be unknown to the profile entry check only: a false warning at load, never a wrong authorization verdict.
- The permission strings are a contract with operators' profiles and with TACACS+ servers, which match on them. Renaming one breaks every profile and every server rule that names it.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Check per verb in the editor's dispatch | One gate in the session factory, refusing the session | The gate cannot express a profile that allows `set` and `commit` but not `delete`, and it would remove config-mode `show` from a read-only user; the web terminal already checks per verb |
| Fail closed with no session authorizer | Allow, as the web and API do with a nil authorizer | The SSH transport already denies in that case for every operational command and for upload; one rule per transport |
| Name appended for a named configuration | Same strings with no name; a separate open permission | Owner decision Q-3: existing entries keep covering it, and a profile can single out one file |
| Mapping owned by the CLI package | Keep it in the web package and import it from the hub | The CLI owns the verbs, and the web already imports the CLI |

## Known Limitations
- Completion still offers a verb the profile denies; it is refused on Enter, as in the web terminal.
- The built-in `read-only` and `admin` profiles are never installed outside tests, so an operator declares them in the configuration. That is owned by `plan/release/fixes/spec-wire-or-delete-unreached-code.md` (journal `unwired-feature` 2026-09-05).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ssh-editor-authorization.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

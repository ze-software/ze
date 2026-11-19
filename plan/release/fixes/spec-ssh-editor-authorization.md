# Spec: ssh-editor-authorization -- The SSH Session Editor Honours the Authorization Profile

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P0 |
| Release reason | A fail-open boundary: a user whose profile denies configuration changes can commit one through the SSH session editor. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Implementation tier | development tier |
| Updated | 2026-11-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

This is a skeleton. It records a finding, its evidence and the questions its
design owes. Nothing here is designed: run `/ze-spec` on it, and put the design
to the owner before it moves to `ready`.

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
the user's profile, the same way the same action is authorized on the web and
through the API, and a refused user is told so in the access-control wording
the other surfaces use.

Found on 2026-11-19 while tracing the hub's session editor for
`plan/release/fixes/spec-fleet-6-config-freeze.md`. It exists with or without
fleets. Read at source, not run: no test drives a denying profile into the
editor, so the first step of this spec is the reproduction below.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/authorization.md` - what each profile section promises
  → Constraint: [to be annotated: the wording of the `read-only` profile and of the edit section]
- [ ] `docs/guide/operator-access-rbac.md` - the sample operator profile
  → Constraint: [to be annotated: the sample allows `configure` and `commit` in the edit section, which an all-or-nothing gate cannot express]

**Key insights:**
- The authorizer reaches the session factory and is handed to the operational command executor only.
- The editor package holds no authorizer. Config-mode lines never reach the executor, so the comment that "the selected executor still authorizes the command" holds for operational lines alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` - `setSessionAuthorizer`, `getSessionAuthorizer`, `teaHandler`: authentication binds one authorizer to the connection and passes it to the session model factory
- [ ] `internal/component/ssh/session.go` - `parseSessionRequest`: accepts a config name that is one plain filename and a mode of `edit` or `command`; authorizes nothing
- [ ] `cmd/ze/hub/session_factory.go` - `buildSessionModelFactory`: uses the authorizer once, in `srv.ExecutorForUser`; it is not given to the editor or the model. A session that names another stored configuration is checked for existence only, and its commit is store-only
- [ ] `cmd/ze/hub/session_editor.go` - `newSessionEditor`: `cli.ValidateUser` checks the characters of the user name
- [ ] `internal/component/cli/model_commands.go` - `dispatchCommandWords`: routes `set`, `delete`, `commit`, `rollback`, `load`, `save`, `discard`, `activate` and `deactivate` to their handlers, guarded only by "an editor exists"
- [ ] `internal/component/cli/model_commands_commit.go` - `runCommitSession`: validates, commits, notifies the reload; no profile check
- [ ] `internal/component/web/handler_config.go` - `authorizeWebConfigMutation`: the web's per-action check, the model to match
- [ ] `cmd/ze/hub/config_upload_ssh.go` - `registerSSHConfigUpload`: authorizes the commit and fails closed on a missing authorizer
- [ ] `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go` - `requireWriteAccess`: the API's check

**Behavior to preserve:**
- A daemon with no `system.authorization` configured stays permissive, as documented
- The attached console authenticates nobody and runs as the local user
- A user whose profile allows editing keeps every config-mode action

**Behavior to change:**
- A config-mode action in the SSH session editor is refused for a user whose profile denies it

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Web config authorization | `internal/component/web/handler_config.go` `authorizeWebConfigMutation` | Authorizes each config action by a command string against the user's profile | Extends: the SSH editor asks the same question with the same strings |
| API config authorization | `internal/component/api/types.go`, the `ConfigAuth` strings; `requireWriteAccess` in the REST and gRPC servers | Authorizes config sessions | Extends: one set of strings for every surface |
| SSH upload authorization | `cmd/ze/hub/config_upload_ssh.go` `registerSSHConfigUpload` | Authorizes a commit and fails closed with no authorizer | Extends: the precedent on the same transport |
| Session authorizer | `internal/component/ssh/ssh.go` `getSessionAuthorizer` | Carries the profile to the session factory | Extends: it has to reach the editor model, where it stops today |

## Design Questions the Owner Decides

| # | Question | Why it is open |
|---|----------|----------------|
| Q-1 | One gate at session open, or a check per config-mode verb? | A gate in `buildSessionModelFactory` is one site and covers the daemon's own and every named configuration, but it is all or nothing. The documented operator profile allows `configure` and `commit` and denies the rest, which needs a per-verb check in the editor model, and a read-only user would lose config-mode `show` under the single gate |
| Q-2 | What a missing authorizer means | The SSH upload fails closed; the web and the API allow. The editor needs one of the two, stated |
| Q-3 | Whether naming another stored configuration needs its own permission | Today any authenticated user can name any stored file. A profile that may edit the daemon's configuration may not be meant to edit a hub's client configurations |

## Reproduction Owed (first step)

No existing test covers this. The scenario, in words:

- A daemon with an admin user and a user whose profile denies the edit section, as `test/plugin/data-authz-read-only.ci` configures them, on disk storage, with a second stored configuration.
- As the denied user, open an interactive session in edit mode: today it opens. It must be refused in the access-control wording.
- As the denied user, open a session naming the second stored configuration: today it opens and a commit changes the stored bytes. It must be refused and the bytes unchanged.
- As the denied user in command mode, enter config mode, change a value and commit: today the active configuration changes. It must be refused and the configuration unchanged.
- As the admin user, all three succeed, each proven by its effect.

Existing tests near it: `test/plugin/rbac-ssh-only-enforced.ci` proves dispatcher refusals over SSH exec and says in its header that interactive `configure` is not tested. `cmd/ze/hub/session_factory_test.go` passes no authorizer in any case.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The defect is real at runtime, not only in the reading | Source read on 2026-11-19: the authorizer stops at the session factory | The spec is obsolete | The reproduction above, run before any design | unvalidated |
| A-2 | No other layer refuses the session before the editor is built | No profile check was found between authentication and the editor; a check elsewhere was not excluded | The defect is narrower than stated | The reproduction above | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fix that gates the whole editor locks out a profile that is meant to commit | An operator profile refused config mode | Q-1 is decided before any code |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [to be written at design, from the reproduction and Q-1 to Q-3] | [to be written] | [to be written] |

## Files to Modify
- [to be decided at design. Known sites: `cmd/ze/hub/session_factory.go`, `internal/component/cli/model_commands.go`]

## Files to Create
- [to be decided at design. A functional test for the reproduction is owed]

## Known Limitations
- None recorded. This is a skeleton.

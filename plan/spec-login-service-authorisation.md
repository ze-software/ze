# Spec: login-service authorisation, a named set of doors an account may use

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

Ze authenticates a user, then authorises each command that user runs. Nothing
between those two steps asks whether the user may use the surface they arrived
on. `(*Store).Authorize` (`internal/component/authz/authz.go`) is the one
authorisation decision point, and it takes a command path, never a surface name.

The owner's design, given 2026-08-11: a new authorisation section, sibling to
`profile`, naming the login services an account may use. A service that offers a
login passes its own name into the authentication request. When the account's
login set has no match for that name, authentication FAILS: the username does
not work on that surface even though the account exists and the password is
right.

This is an AUTHENTICATION gate, not an authorisation one. A refused login never
reaches command authorisation.

Service names are REGISTERED, not spelled in the schema. A plugin that offers a
new login registers its name, and config validation checks a configured value
against the live registry.

**Scope of this spec: the four surfaces that already know who the user is.**
SSH, web, REST and gRPC each carry a username into `(*Store).Authorize` today.
The looking glass and gNMI authenticate with a shared token and learn no
username at all, so they cannot answer the question yet; giving them identity is
`plan/spec-login-identity-for-looking-glass-and-gnmi.md`.

## Assumptions the owner has not ruled on

- **A user naming no login set reaches every login service.** Denying by
  default locks every existing deployment out at upgrade, because no config in
  the field carries the new section. Stated here so it is a decision rather than
  an accident; the owner can invert it and only AC-7 changes.
- **MCP stays outside this work.** `(*Streamable).authenticate`
  (`internal/component/mcp/bearer.go`) resolves an `Identity` with its own
  `HasScope` model that never reaches `authz.Store`. Folding two scope models
  into one is a separate design, not a line in this spec.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the rule this change is governed by: the login set is a guard
  → Constraint: a login set that could not be READ MUST NOT read as "no restriction". A reader that cannot answer MUST deny.
  → Constraint: the test MUST drive the gate from a real login on a running daemon, never from the matching helper alone.
- [ ] `ai/patterns/registration.md` - the registration pattern this copies
  → Decision: the core discovers names through a registry; no central file lists the services.
- [ ] `docs/guide/authentication.md` - the operator-facing claim about who authenticates on each surface
  → Constraint: its table becomes incomplete the moment a correct password can be refused for the surface it arrived on.

### RFC Summaries (Scope: protocol)
- N-A. Scope is config and local credential policy. No wire format and no RFC
  obligation is touched. The bearer and password credentials on the path are
  unchanged.

**Key insights:** (minimal context to resume after compaction)
- The registry-backed name check already exists three times over and needs no invention: `RegisterSource` (`internal/component/config/redistribute/registry.go`) is written at `init()`, the leaf is `type string` carrying `ze:validate "redistribute-source"`, and `RedistributeSourceValidator` (`internal/component/config/validators.go`) calls `redistribute.LookupSource` at validate time and hands `redistribute.SourceNames` back as its `CompleteFn`. `CheckAllValidatorsRegistered` (`internal/component/config/yang/validator_registry.go`) fails a `ze:validate` name nobody registered.
- There is ONE user list, not two. `ExtractAuthUsers` (`internal/component/config/infra/ssh.go`) reads `system/authentication/user` and is the only producer of `authz.UserConfig`, which is `aaa.UserCredential` (`internal/component/aaa/types.go`).
- `aaa.UserCredential` carries `Name`, `Hash`, `Profiles` and `PublicKeys`. It carries NO surface field, so the surface name has to be threaded in rather than read off the credential.
- The user list already carries `leaf-list profile` (`internal/component/ssh/yang/ze-ssh-conf.yang`). The login-set reference is its sibling, and the ssh module owns that node even though the authz module owns the profile it names.
- No surface has a per-user surface gate today. Four carry a username into `authz`; gNMI and the looking glass carry none.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/authz/authz.go` - `Store` holds `profiles` and `assignments`; `(*Store).Authorize` is the single decision point and fails closed on an empty identity; `(*Profile).Authorize` picks a run or edit section and `(*Section).evaluate` walks numbered entries against a command path. Nothing on this path names a surface.
- [ ] `internal/component/aaa/types.go` - `UserCredential{Name, Hash, Profiles, PublicKeys}` is the credential shape every backend consumes. `BuildParams` carries `LocalUsers` and `LocalUsersFunc`, which is what makes a deleted user stop authenticating without a restart.
- [ ] `internal/component/config/infra/ssh.go` - `ExtractAuthUsers` reads `system/authentication/user` from the resolved `system` map and is the ONE producer of the credential shape. `ExtractSSHConfig` derives its own `Users` field from it, after two early returns that fire when `environment` or `environment/ssh` is absent.
- [ ] `internal/component/config/infra/authz.go` - `extractAuthzConfig` populates the store and `ValidateAuthzConfig` rejects a user naming an undefined profile, by string compare against the configured profile list. This is the existing shape a login-set reference check must match.
- [ ] `internal/component/authz/yang/ze-authz-conf.yang` - `system/authorization/profile`, a named list whose `run` and `edit` containers each hold a `default-action` and an ordered `entry` list. The new section is its sibling.
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - owns `system/authentication/user`, with `name`, `password`, `plaintext-password`, `leaf-list profile` and a `public-keys` list. The user side of the new reference belongs here, beside `profile`.
- [ ] `internal/component/config/redistribute/registry.go` - `RegisterSource`, `SourceNames`: the registry shape to copy, including `ErrSourceConflict` on a re-registration that disagrees.
- [ ] `internal/component/config/validators.go` - `RedistributeSourceValidator`, the validator shape to copy, including `CompleteFn` for CLI completion.
- [ ] `internal/component/config/validators_register.go` - `RegisterValidators` wires a validator to the `ze:validate` name.
- [ ] `internal/component/ssh/passwordauth.go`, `internal/component/ssh/pubkey.go` - `(*Server).authenticatePassword` and `.authenticatePublicKey`, the SSH login decision.
- [ ] `internal/component/web/auth.go` - `(*SessionStore).CreateSession` and `.ValidateToken`, the web login decision.
- [ ] `internal/component/api/rest/auth.go` - `(*RESTServer).withAuth`, the REST login decision.
- [ ] `internal/component/api/grpc/server.go` - `(*GRPCServer).checkAuth`, the gRPC login decision.

**Behavior to preserve:**
- `(*Store).Authorize` keeps failing closed on an empty identity, and command authorisation keeps its current verdicts for every user who is admitted.
- The zefs power user keeps authenticating everywhere it does today, including its reserved recovery profile. It is the recovery path and MUST NOT be lockable out of a surface by a config edit.
- The live user source survives: a user deleted from the config still stops authenticating at the next reload without a restart.
- Public-key SSH login keeps working, and the login set gates it identically to a password login.
- Every existing config keeps working with no edit, per the default in Assumptions.

**Behavior to change:**
- A new named list under `system/authorization/` names a set of login services.
- `system/authentication/user` gains a reference to that list, validated the way the profile reference already is.
- Each login service passes its registered name into the authentication request.
- A login whose service name is not in the account's set is REFUSED, with the same response the surface gives a wrong password.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator presents a credential to SSH, web, REST or gRPC.
- A config tree read at boot or at reload, carrying `system/authorization/` and `system/authentication/user`.
- A plugin's `init()` registering a login-service name.

### Transformation Path
1. A component that offers a login registers its service name at `init()`, before any config is read.
2. Config validation resolves each configured service name against the registry, and each user's login-set reference against the configured sets.
3. `ExtractAuthUsers` produces the credential list, now carrying the login-set reference.
4. A login arrives on one surface. That surface names itself in the authentication request.
5. The authenticator matches the credential first, then the service name against the account's resolved login set.
6. No match refuses the login. The surface returns its existing unauthorised response, unchanged in shape.
7. A match admits the login, and `(*Store).Authorize` decides commands exactly as it does today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → name registry | A registration call at `init()`, no import of the plugin by the core | Yes: the shape `RegisterSource` (`internal/component/config/redistribute/registry.go`) already uses |
| Config tree → credential | `ExtractAuthUsers` (`internal/component/config/infra/ssh.go`) stays the one producer; the login-set reference joins the existing fields | Yes: one reader, no second parser of `system/authentication` |
| Surface → authentication request | The surface passes its registered name; no transport type crosses into `aaa` or `authz` | To be established by the design phase: `aaa.UserCredential` has no surface field today |
| Authentication → authorisation | Unchanged. A refused login never reaches `(*Store).Authorize` | Yes: the gate sits before the authorisation call |

### Integration Points
- `ExtractAuthUsers` (`internal/component/config/infra/ssh.go`) - the one credential producer. The login-set reference is read here or nowhere.
- `ValidateAuthzConfig` (`internal/component/config/infra/authz.go`) - already rejects a user naming an undefined profile. The login-set reference gets the same treatment in the same function.
- `RegisterValidators` (`internal/component/config/validators_register.go`) - where the new `ze:validate` name is wired.
- `(*Store).Authorize` (`internal/component/authz/authz.go`) - NOT edited. Command authorisation is a separate question and stays one.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The gate sits in the authentication path every surface already calls; no surface grows its own copy of the rule |
| No unintended coupling (components stay isolated) | Yes | The core imports no login provider. Names arrive through the registry |
| No duplicated functionality (extends existing, does not recreate) | Yes | The registry, the validator wiring and the reference check all copy shapes that exist |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Login path, no wire buffers |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them (`ai/rules/plugins.md`) | Yes | This is the point of the design. No central file may list the service names, and a new plugin adds one registration call |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The registry-backed name check needs no new mechanism | `RegisterSource` / `RedistributeSourceValidator` / `RegisterValidators` are read and are exactly this shape | The spec invents a mechanism the repo already has, against `ai/rules/simplicity.md` | The implementation's validator is a copy of the redistribute one, reviewed side by side | unvalidated |
| A-2 | Four surfaces can name themselves at login without a transport type crossing into `aaa` | Each has a single login decision function already isolated from its transport | The threading grows a parameter through unrelated layers | A design phase that names the exact signature before any surface is edited | unvalidated |
| A-3 | The default in Assumptions keeps every field config working | No config in the field carries a section that does not exist yet | An upgrade locks operators out of their own daemon, including out of the surface they would use to fix it | AC-7, and a functional test on a config carrying no login section at all | unvalidated |
| A-4 | The zefs power user cannot be locked out by a config edit | `usersFromZefsDB` (`cmd/ze/hub/main_servers.go`) attaches a reserved recovery profile that config users do not get | The recovery path becomes lockable, which is the failure this whole area exists to prevent | AC-8, driven from a running daemon whose config denies every service | unvalidated |
| A-5 | Registration completes before the first config validation reads the registry | Every existing registry of this shape is written at `init()`, and the composition root is generated | An external plugin registering later leaves a valid name rejected at validate time | The design phase states the ordering explicitly, as `RegisterListenerProtocols` already does for its own readers | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator locks themselves out of every surface, including the one they would use to undo it | A config that names a login set excluding every service | The zefs power user is exempt (A-4), and the serial console path does not go through this gate. AC-8 proves it |
| R-2 | The gate is added to three surfaces and forgotten on the fourth, so one door silently stays open | A surface whose login function does not name itself | AC-2 covers all four, and the wiring test drives each surface separately rather than testing the matcher once |
| R-3 | A read failure resolves to an empty login set, which the matcher reads as "no restriction" | An empty set reaching the matcher with a nil error | AC-6: a set that could not be read denies and says so. Empty from a FAILED read is never the permissive answer |
| R-4 | The gate is placed after command authorisation, so a refused user still runs a command | A test that asserts the response code but not the side effect | The gate sits in the authentication path. AC-3 asserts that a refused login performs no command |
| R-5 | The login set is checked at boot only, so a reload cannot tighten it | The gate passes after a config edit that should have closed it | AC-5 drives a reload, matching the live-source behaviour the credential path already has |
| R-6 | A name registered by two components with different meanings | The registry accepts a silent overwrite | Copy the conflict error `RegisterSource` already returns on a re-registration that disagrees |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two directions, both bad and not symmetric. Too permissive: a surface that should refuse an account admits it, which is the status quo and is at least no worse. Too strict: an operator is locked out of every management surface of a running router, with no way in except physical access. The second is the one to design against |
| How is it reverted? | Single commit revert. The new config section becomes unknown, so a config carrying it must be edited before the older daemon accepts it. That is a one-way door for the config file and MUST be stated in the release note |
| Who else touches this path? | `plan/spec-hub-deferred-api-auth-independent-of-ssh-block.md` corrects where the API reads its user list from, in the same area. It is separable and lands first. `plan/spec-login-identity-for-looking-glass-and-gnmi.md` extends the same design to two more surfaces |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config declaring a login set and a user referencing it, validated at load | → | the new `ze:validate` validator wired in `RegisterValidators` (`internal/component/config/validators_register.go`) | `TestLoginServiceValidatorRejectsUnregisteredName` (`internal/component/config/validators_test.go`) |
| An SSH password login by a user whose set excludes ssh | → | `(*Server).authenticatePassword` (`internal/component/ssh/passwordauth.go`) | `test/plugin/login-service-scope-ssh-refused.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A REST request by a user whose set excludes rest | → | `(*RESTServer).withAuth` (`internal/component/api/rest/auth.go`) | `test/plugin/login-service-scope-rest-refused.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web login by a user whose set excludes web | → | `(*SessionStore).CreateSession` (`internal/component/web/auth.go`) | `test/ui/login-service-scope-web-refused.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gRPC call by a user whose set excludes grpc | → | `(*GRPCServer).checkAuth` (`internal/component/api/grpc/server.go`) | `test/plugin/login-service-scope-grpc-refused.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A component offering a login registers its service name at startup | The name is in the registry before any config validation runs, and a second registration of the same name with a different meaning is an error, not a silent overwrite |
| AC-2 | An account whose login set excludes a given service presents a CORRECT credential to that service, once for each of SSH, web, REST and gRPC | Every one refuses, with the same response it gives a wrong credential. No surface leaks that the account exists |
| AC-3 | The same refused login on a surface that would have dispatched a command | No command is dispatched, no session is created, and no accounting record claims a successful login |
| AC-4 | The same account presents the same credential to a service its set DOES include | The login succeeds and command authorisation behaves exactly as it does today |
| AC-5 | A running daemon, a config edit removing a service from an account's set, then a reload | The next login on that service is refused, with no restart |
| AC-6 | A login set that cannot be resolved, as distinct from a user that names no set | The login is refused and the reason is logged. An unreadable set is never treated as "no restriction" |
| AC-7 | A user naming no login set at all | Every login service admits it, so an untouched config keeps working after an upgrade |
| AC-8 | The zefs power user, on a config whose login sets exclude every service | It still authenticates on every surface. The recovery path cannot be closed by a config edit |
| AC-9 | A config naming a login service that no component registered | Config validation rejects it at load, naming the value, and CLI completion offers only registered names |
| AC-10 | A user referencing a login set that is not defined | Config validation rejects it, the way an undefined profile reference is already rejected |
| AC-11 | A public-key SSH login by an account whose set excludes ssh | Refused, identically to the password case. The gate is not password-specific |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | creates a monitoring account allowed on the API only, and tries to SSH in with it | config → credential with a login set → SSH login decision | `test/plugin/login-service-scope-ssh-refused.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | uses that same account against the REST API | config → credential → REST login decision → dispatcher | `test/plugin/login-service-scope-rest-refused.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | mistypes a service name in the config | config load → the new validator → rejection naming the value | `TestLoginServiceValidatorRejectsUnregisteredName` |
| 4 | removes a service from an account's set on a running router | reload → live credential source → next login refused | `test/plugin/login-service-scope-reload-tightens.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoginServiceRegistryRejectsConflictingRegistration` | `internal/component/config/loginservice/registry_test.go` | AC-1: a second registration that disagrees is an error, not an overwrite | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestLoginServiceValidatorRejectsUnregisteredName` | `internal/component/config/validators_test.go` | AC-9: an unregistered name fails validation and completion lists only registered names | |
| `TestLoginSetReferenceMustExist` | `internal/component/config/infra/authz_test.go` | AC-10: an undefined login-set reference is rejected beside the existing undefined-profile check | |
| `TestLoginSetAbsentAdmitsEveryService` | `internal/component/authz/authz_test.go` | AC-7: the default in Assumptions | |
| `TestLoginSetUnreadableDenies` | `internal/component/authz/authz_test.go` | AC-6: an unresolvable set denies and never reads as permissive | |
| `TestPowerUserIgnoresLoginSets` | `cmd/ze/hub/auth_e2e_test.go` | AC-8: the recovery path cannot be closed by config | |
| `TestSSHPublicKeyLoginHonoursLoginSet` | `internal/component/ssh/pubkey_test.go` | AC-11: the gate is not password-specific | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| services named in one login set | 0 to the registry size | 1, the smallest set that admits anything | 0, an empty set, which admits nothing and MUST be distinguishable from an absent reference | N-A: naming every registered service is the same as naming none, by the default in Assumptions |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `login-service-scope-ssh-refused` | `test/plugin/login-service-scope-ssh-refused.ci` | A correct password on a surface the account is not allowed on is refused, and the same password succeeds on a surface it is allowed on | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `login-service-scope-rest-refused` | `test/plugin/login-service-scope-rest-refused.ci` | The REST half of AC-2 and AC-4 on one running daemon | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `login-service-scope-grpc-refused` | `test/plugin/login-service-scope-grpc-refused.ci` | The gRPC half of AC-2 and AC-4 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `login-service-scope-web-refused` | `test/ui/login-service-scope-web-refused.ci` | The web half, driven through the editor suite that already exercises web login | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `login-service-scope-reload-tightens` | `test/plugin/login-service-scope-reload-tightens.ci` | AC-5: a reload closes a door on a running daemon | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

### Interop Tests (Scope: protocol)
N-A. Scope is config and local credential policy. No wire format, no peer
daemon, and no RFC obligation is touched.

## Files to Modify
- `internal/component/authz/yang/ze-authz-conf.yang` - the new named list under `system/authorization/`, sibling to `profile`
- `internal/component/ssh/yang/ze-ssh-conf.yang` - the user's reference to a login set, beside the existing `leaf-list profile`
- `internal/component/config/infra/ssh.go` - `ExtractAuthUsers` carries the reference on the credential it already produces
- `internal/component/config/infra/authz.go` - `ValidateAuthzConfig` rejects an undefined login-set reference, beside the undefined-profile check
- `internal/component/config/validators.go` - the new validator, copying `RedistributeSourceValidator`
- `internal/component/config/validators_register.go` - wire the new `ze:validate` name
- `internal/component/aaa/types.go` - the login set on the credential, and the surface name on the authentication request
- `internal/component/ssh/passwordauth.go`, `internal/component/ssh/pubkey.go` - name the surface at login
- `internal/component/web/auth.go` - name the surface at login
- `internal/component/api/rest/auth.go` - name the surface at login
- `internal/component/api/grpc/server.go` - name the surface at login
- `docs/guide/authentication.md` - the table stating who authenticates on which surface
- `docs/guide/configuration.md` - the new config section

## Files to Create
- `internal/component/config/loginservice/registry.go` - the name registry, copying `internal/component/config/redistribute/registry.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- the five `.ci` files named in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The new list in the authz module, the reference in the ssh module |
| YANG validation constraints | Yes | The reference is `type string` with `ze:validate`; the registry holds the value set, not the schema |
| YANG custom validators | Yes | The new validator, wired by name in `RegisterValidators` |
| CLI commands/flags | No | No new verb. The config editor reaches the new section through the schema |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | Yes | Automatic once the validator supplies `CompleteFn`, as the redistribute source leaf does |
| Functional test for new RPC/API | Yes | The five `.ci` files |
| Pipe completeness | No | No new command output |
| Env var registration | No | No `environment/` leaf. The section lives under `system` |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary |
| Prometheus counters/metrics | Yes | A refused-by-login-set counter, so an operator can see a misconfigured set rather than hunt for it in logs |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, and the authz section of `docs/architecture/config/` |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | The surfaces are unchanged. Only who may reach them changes |
| 5 | Plugin added/changed? | No | No plugin is added; a plugin CAN now register a login name |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md` |
| 7 | Wire format changed? | No | No wire format on this path |
| 8 | Plugin SDK/protocol changed? | Yes | A plugin offering a login registers a name; the SDK page must say how |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs local credential policy |
| 10 | Test infrastructure changed? | No | Existing `test/plugin` and `test/ui` suites |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it claims parity on per-user access control |
| 12 | Internal architecture changed? | Yes | The authentication path gains a decision. Document it beside the AAA design |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The refused-by-login-set counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A new registry. Add it to the registration inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming the four login functions and `internal/component/config/infra/ssh.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/authentication.md` shows user config examples that now have a further dimension |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove a login reaches a decision point that can refuse it
   - Tests: the four surface `.ci` files, written to fail
   - Files: the five `.ci` files
   - Verify: each fails because the login SUCCEEDS today, not on a setup error
2. **Phase: The registry** -- names register, and nothing central lists them
   - Tests: `TestLoginServiceRegistryRejectsConflictingRegistration`
   - Files: `internal/component/config/loginservice/registry.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
   - Verify: AC-1. No file outside the registry enumerates the service names
3. **Phase: Config surface** -- the section, the reference, and their validation
   - Tests: `TestLoginServiceValidatorRejectsUnregisteredName`, `TestLoginSetReferenceMustExist`
   - Files: the two YANG modules, `validators.go`, `validators_register.go`, `infra/authz.go`
   - Verify: AC-9, AC-10, and CLI completion offers registered names only
4. **Phase: The credential carries the set** -- one producer, no second reader
   - Tests: `TestLoginSetAbsentAdmitsEveryService`, `TestLoginSetUnreadableDenies`
   - Files: `internal/component/config/infra/ssh.go`, `internal/component/aaa/types.go`
   - Verify: AC-6, AC-7. An unreadable set denies
5. **Phase: The gate, on all four surfaces at once** -- never one surface at a time
   - Tests: the four surface `.ci` files, plus `TestSSHPublicKeyLoginHonoursLoginSet`
   - Files: the four login functions
   - Verify: AC-2, AC-3, AC-4, AC-11. A surface left ungated is the failure this phase exists to prevent
6. **Phase: Reload and recovery**
   - Tests: `login-service-scope-reload-tightens`, `TestPowerUserIgnoresLoginSets`
   - Files: whichever reload path the credential source already uses
   - Verify: AC-5, AC-8
7. **Phase: Docs, metric and discrimination**
   - Tests: every `.ci` above, each re-run with the gate reverted
   - Files: the documentation set, the counter
   - Verify: reverting the gate turns every new `.ci` red. A test that stays green proves nothing

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All four surfaces gated, proven separately. Three out of four is a hole, not progress |
| Feature completeness | Boot AND reload. A gate a reload cannot tighten is half a gate |
| Correctness | A refused login is indistinguishable from a wrong password to the caller, and distinguishable in the log to the operator |
| Naming | The section names LOGIN, not permission. It decides which door, never which command |
| Data flow | One credential producer, one registry, no central list of service names anywhere |
| Rule: `ai/rules/evidence.md` | An unreadable login set denies. An empty set from a failed read never reaches the matcher as permissive |
| Rule: `ai/rules/simplicity.md` | The registry and the validator are copies of shapes that exist. A new mechanism here is the smell |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No central list of service names | Grep the tree for any file enumerating the names outside the registry and its registrants |
| All four surfaces gated | The four `.ci` files pass, and each fails with the gate reverted |
| The recovery path survives | `TestPowerUserIgnoresLoginSets`, driven from a running daemon |
| An untouched config still works | A `.ci` on a config with no login section at all |
| Lint | `make ze-lint-changed` |
| Schema | `make ze-doc-test`, `make ze-cli-grammar-check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | Permissive: the status quo, an account reaching a surface it should not. Strict: an operator locked out of a running router with no remote way back. Design against the second |
| What proves it did not | Each `.ci` asserts BOTH halves on one daemon: refused where excluded, admitted where included. A refusal-only test cannot tell a working gate from a broken login |
| Fail closed | An unresolvable login set denies. Trace every error return and confirm none yields an empty set with a nil error |
| Empty is not "no restriction" | A user naming an EMPTY set is denied everywhere. A user naming NO set is admitted everywhere. These must not collapse into one value |
| Guard driven from its entry point | Real logins on a running daemon. Calling the matcher with a hand-made set proves the helper, never the wiring |
| Privilege | The zefs power user is exempt. Confirm no config edit can reach it |
| No user enumeration | A refused login MUST NOT reveal that the account exists. Same status, same body, same timing shape as a wrong credential |
| Accounting | A refused login is recorded as a failed login, never as a successful one |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A new `.ci` passes with the gate reverted | The test is vacuous. Assert the credential decision, not the response code alone |
| The surface name cannot be threaded without a transport type crossing into `aaa` | STOP. That is A-2 broken. Report the signature you reached and ask which way |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The mechanism the owner asked for exists three times over. Route redistribution
  registers source names at `init()`, tags the leaf `ze:validate
  "redistribute-source"`, and lets one validator answer both "is this real" and
  "what may I offer in completion". Address families and internal plugin names
  use the same shape. The schema-enum-versus-validator question is therefore
  already settled in this repository, in favour of the validator.
- The hard half is not the registry. It is that no surface asks the question at
  all today, so this is new enforcement rather than a new check on existing
  enforcement.
- The two halves of "who are you" and "what may you run" are cleanly separated
  today, and this gate belongs in the first. Putting it in the second would let a
  refused user hold a session.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A separate named section, not a field on `profile` | Add a service list to the existing profile | The owner's instruction of 2026-08-11. The two answer different questions, and a profile is about commands. Keeping them separate lets one account share a command policy with another while differing on doors |
| Refuse at authentication, not at authorisation | Admit the login and deny every command | The owner's instruction: the username does not work. An admitted session that can do nothing still holds state, still appears in accounting, and still tells an attacker the account exists |
| Names come from a registry, never from the schema | A YANG enum listing the services | A schema enum makes every new login-offering plugin a schema edit. The registry is what the owner asked for and what the repo already does three times |
| A user naming no set reaches every service | Deny by default | Denying breaks every config in the field at upgrade, including locking the operator out of the surface they would use to fix it. Recorded in Assumptions for the owner to invert |

## Known Limitations
- The looking glass and gNMI are not covered. Both authenticate with a shared
  token and learn no username, so they cannot answer the question until they
  have identity. That is
  `plan/spec-login-identity-for-looking-glass-and-gnmi.md`.
- MCP keeps its own `Identity` scope model. Unifying the two is not attempted
  here.
- Three lists of service names already exist and disagree: the audit constants
  (`internal/core/audit/audit.go`), `registerService`
  (`cmd/ze/hub/service_registry.go`), and `DiscoverListenerServices`
  (`internal/component/config/listener.go`). This spec adds a fourth that is
  narrower on purpose, holding LOGIN services only. Reconciling the other three
  is not in scope and MUST NOT be attempted here.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated on all four surfaces, not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] The owner has ruled on both items under "Assumptions the owner has not ruled on"

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

# Spec: make SSH an optional build component

| Field | Value |
|-------|-------|
| Status | closure-ready |
| Scope | plugin, config |
| Depends | - |
| Phase | closure-ready |
| Deferral shard | - |
| Handoff | verify |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

SSH already compiles out through the `ze_ssh` feature gate. The hub keeps an
always-on nil seam, and `register_ssh.go` installs the SSH implementation only
when the tag is present. However, the gated SSH YANG module also owns the shared
`system.authentication.user` list. `ExtractSSHConfig` carries these users in
`SSHExtractedConfig.Users`, so always-on AAA and API startup still discover local
identities through an SSH-named result.

Make SSH independently removable like web. A build without `ze_ssh` must omit
the SSH component and its schema while retaining password and authorization
profile users for non-SSH management surfaces. SSH public keys must remain
available only when SSH is compiled in. A build with SSH must preserve current
config syntax, login behavior, host-key behavior, and defaults.

The dependency spec removes the REST and gRPC consumers of
`SSHExtractedConfig.Users` first. This spec then removes that obsolete field and
finishes the schema, AAA, and build-composition separation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component boundaries and authentication ownership
  → Decision: `authz` owns shared authentication data; SSH consumes it as one removable transport.
  → Constraint: every package under `internal/component/` is independently removable. Always-on code must not import the optional SSH package.
- [ ] `docs/architecture/config/yang-config-design.md` - YANG ownership and registration
  → Decision: the always-on authz module owns the base user list. The gated SSH module augments that list with public keys.
  → Constraint: removing a gated module registration must remove only that component's nodes.
- [ ] `ai/patterns/registration.md` - feature-gated registration
  → Decision: preserve `feature-gates.txt`, the generated `all_ze_ssh.go` import, and the existing `ssh_infra.go` seam.
  → Constraint: no new registry or generic service layer. The current seam already provides compile-time omission.
- [ ] `ai/patterns/config-option.md` - YANG and parser proof
  → Decision: preserve the operator path `system.authentication.user`. Only module ownership changes.
  → Constraint: the no-SSH composition must prove shared nodes are accepted and SSH-only nodes are rejected.
- [ ] `docs/guide/operator-access-rbac.md` - documented user, SSH, and RBAC syntax
  → Decision: preserve every documented config path.
  → Constraint: password and profile fields remain without SSH; `public-keys` and `environment.ssh` do not.
- [x] `spec-hub-deferred-api-auth-independent-of-ssh-block` - boot user, no-BGP AAA, REST, and gRPC migration (closed)
  → Decision: that spec owns all `cmd/ze/hub/main.go` edits, API reload, API transport tests, and no-BGP AAA proof.
  → Constraint: it lands first, removes every `main.go` consumer of `SSHExtractedConfig.Users`, and installs AAA when no BGP reactor exists.
- [ ] `plan/spec-login-service-authorisation.md` and `plan/spec-ssh-fido2-keys.md` - later shared-user and SSH-key work
  → Decision: both specs depend on this ownership migration.
  → Constraint: login-service edits authz YANG and `infra/authz.go`; FIDO2 edits the gated SSH public-key augment.

### RFC Summaries (Scope: protocol)
- [ ] N-A - build composition and config ownership do not change a wire protocol.

**Key insights:**
- `ze_ssh` already gates the component package and its YANG registration. Replacing that seam would add a second pattern.
- The base user list is shared identity, but the current gated SSH module declares it.
- `infraSetup` currently builds AAA and SSH inputs from the SSH-owned snapshot even though the shared live user source is already authoritative.
- Web's omission proof checks schema rejection and linked symbols. The SSH omission test checks only nil seam functions.
- `TestReloadHashesPlaintextPassword` is shared authentication coverage. Its `ze_bgp && ze_ssh` tag masks the bare-core schema regression this work can introduce.
- `ze-stripped` intentionally includes `ze_ssh`; bare `ze_core` is the existing no-SSH composition.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `feature-gates.txt` - declares `ze_ssh internal/component/ssh` as the compile-out source of truth.
- [ ] `scripts/codegen/plugin_imports.go` - derives gated imports from the manifest.
- [ ] `internal/component/plugin/all/all_ze_ssh.go` - imports the SSH YANG package only under `ze_ssh`.
- [ ] `cmd/ze/hub/ssh_infra.go` - defines the always-on nil seam without importing SSH.
- [ ] `cmd/ze/hub/register_ssh.go` - installs the SSH seam functions under `ze_ssh`.
- [ ] `cmd/ze/hub/build_tag_ssh_absent_test.go` - checks only that the seam remains nil.
- [ ] `cmd/ze/hub/build_tag_web_absent_test.go` - precedent for config rejection and symbol omission.
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - owns shared users and SSH transport settings.
- [ ] `internal/component/authz/yang/ze-authz-conf.yang` - always-on owner of authorization profiles.
- [ ] `internal/component/config/infra/ssh.go` - extracts SSH settings and shared users in one result.
- [ ] `internal/component/config/infra/hook.go` - declares the obsolete `SSHExtractedConfig.Users` field.
- [ ] `cmd/ze/hub/infra_setup.go` - builds BGP-path AAA and SSH inputs from that field while also carrying the shared live source.
- [ ] `cmd/ze/hub/main_reload_ssh_test.go` - tests shared plaintext-password reload only when BGP and SSH are compiled.
- [ ] `cmd/ze/hub/main_reload_test.go` - provides the untagged reload test reactor used by that test.

**Behavior to preserve:**
- Default Ze and appliance builds include SSH through `ze_ssh`.
- Existing SSH, user, password, profile, and public-key config text remains valid with `ze_ssh`.
- SSH host-key storage, authentication, sessions, listener defaults, and startup behavior do not change.
- Zefs power-user and config-user merge precedence does not change.
- Plaintext passwords are hashed on boot and reload, then removed from the running tree.

**Behavior to change:**
- A build without `ze_ssh` accepts password/profile users while rejecting `environment.ssh` and `public-keys`.
- `ExtractAuthUsers` has shared authz ownership. It reads base credentials in every build and the registered SSH public-key augment only when that schema is present.
- `SSHExtractedConfig.Users` is removed. `ExtractSSHConfig` returns transport settings only.
- BGP-path `infraSetup` resolves one current list from the shared live source for AAA and SSH construction.
- Plaintext-password reload coverage runs without `ze_ssh`.
- The no-SSH test proves schema behavior and the absence of gated SSH implementation symbols while retaining the always-on nil seams.
- A `ze_core,ze_rest` binary without `ze_ssh` starts a loopback REST listener,
  authenticates a config user, and enforces that user's authorization profile.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build composition enters through feature tags derived from `feature-gates.txt`.
- Operator configuration enters through YANG-backed config parsing at boot and reload.

### Transformation Path
1. The generated `all_ze_ssh.go` import registers the SSH module only with `ze_ssh`.
2. Authz YANG registers in both compositions and defines the base user list.
3. With `ze_ssh`, SSH YANG augments each user with `public-keys` and defines `environment.ssh`.
4. Parsing accepts only nodes supplied by the registered modules and preserves the unprefixed user path.
5. `ExtractAuthUsers` reads base fields and any registered credential augments from `system`. `ExtractSSHConfig` reads only `environment.ssh`.
6. The dependency installs no-BGP AAA and supplies standalone SSH from its shared boot snapshot.
7. On the BGP path, `infraSetup` resolves the shared live source once and uses that list for AAA and optional SSH construction.
8. `diskConfigLoaders` applies plaintext-password hashing independently of SSH at every reload.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Feature manifest → generated composition | `ze_ssh` selects the SSH package and schema registration | Existing generator plus binary-symbol test |
| YANG registry → config parser | Registered modules define accepted paths | Positive shared-user and negative SSH-only parser tests |
| Config tree → always-on AAA | `liveConfigUsers` calls `ExtractAuthUsers`; `liveLocalUsers` merges config and zefs users; `UsersFunc` is authoritative | Config-infra and hub wiring tests |
| Config file → reload tree | `diskConfigLoaders` calls the normal loader and password transform | Ungated `TestReloadHashesPlaintextPassword` |
| No-SSH REST binary → authenticated command | spawned `ze_core,ze_rest` binary uses authz YANG, no-BGP AAA, and REST | Correct, incorrect, allowed, and denied requests |
| Hub → optional SSH component | Existing nil function seam installed by `register_ssh.go` | Present and absent build-tag tests |

### Integration Points
- `yang.RegisterModule` - keeps shared identity always-on and the SSH augment gated.
- `ExtractAuthUsers` - remains the single user parser; schema registration controls optional SSH public-key values without an SSH package import.
- `ExtractSSHConfig` - becomes transport-only.
- `infraSetup` - resolves the shared live source once before the optional SSH branch.
- `sshBuild` - remains the only hub construction path into SSH.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Config stays in YANG and extraction stays in `config/infra`; the hub does not parse YANG nodes |
| No unintended coupling | Yes | `SSHExtractedConfig` carries no shared identity; always-on code imports no SSH package |
| No duplicated functionality | Yes | Existing live extractor, manifest, generated import, and nil seam are reused |
| Zero-copy preserved where applicable | N-A | Startup config data, not a wire or per-event path |
| Registration over hardcoding | Yes | Existing generated registration remains the composition switch; no new core switch is added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Authz YANG is present in bare `ze_core` | The untagged aggregator imports authz YANG | Shared users still disappear | Absent-tag shared-user parse test | confirmed by `internal/component/plugin/all/all.go` import |
| A-2 | Moving the base list preserves config text and tree paths | Parser and extractors use unprefixed container names | Existing config needs migration | Present/absent parse tests and functional SSH workflows | confirmed by `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig`, `TestBuildTag_SSH_PresentAcceptsSSHConfig`, and the supplied 605/605 plugin result |
| A-3 | The SSH augment resolves after registered modules load | Existing plugin schemas use cross-module augments | Tagged builds reject public keys | Present-tag SSH config test | confirmed by `TestBuildTag_SSH_PresentAcceptsSSHConfig` |
| A-4 | Every production `infraSetup` call receives the shared live user source | `setupInfraHook` threads `localUsers` into the hook | BGP-path AAA has no current users | Wiring test and LSP references | confirmed by source |
| A-5 | The dependency removes every `main.go` field consumer and installs no-BGP AAA | Its ownership table and ACs name both paths | Field removal breaks main or bare-core authorization stays absent | Dependency verification evidence | confirmed by `resolveBootUsers` and no-BGP `buildAAABundle` source |
| A-6 | The plaintext reload test has no SSH-only dependency | Its producer is `diskConfigLoaders`; its reactor helper is untagged | Moving it causes a compile failure | Bare-core package test | confirmed by source |
| A-7 | REST can run without the SSH feature tag | REST and SSH have separate manifest entries and seams | The binary does not link or start | Spawned `ze_core,ze_rest` integration test | confirmed by the supplied no-SSH hub package pass for `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Base user nodes remain gated | Bare-core shared-user parse reports unknown field | Move the whole base list to authz YANG |
| R-2 | Public keys leak into no-SSH schema | Bare-core parser accepts `public-keys` | Keep public keys only in the SSH augment |
| R-3 | Tagged config stops parsing | Present-tag test rejects the augment path | Import authz YANG from SSH YANG; preserve syntax |
| R-4 | BGP-path AAA still depends on the SSH field or treats a failed live read as empty | Wiring test receives no user or authenticates after an injected source error | Resolve `liveUsers` once, log the error, keep the live function authoritative, and reject local auth |
| R-5 | An untagged import links SSH | Binary test finds `internal/component/ssh/`, `sshBuildImpl`, `sshWireImpl`, or `sshBuildStandaloneImpl` | Gate the importing producer; retain the always-on nil seam |
| R-6 | API and optionality specs edit the same coupling | This diff touches dependency-owned default-composition API tests | Land the dependency first and enforce ownership below |
| R-7 | Reload hashing stays SSH-gated | Bare-core package run omits the test | Rename the file and remove its SSH/BGP build constraint |
| R-8 | Source anchors still attribute users to SSH YANG | Documentation check finds old anchors | Update every named anchor |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Bare-core rejects all local users, tagged SSH rejects existing config, AAA starts without configured identities, or reload leaves plaintext in the running tree |
| How is it reverted? | Single commit revert. Config text and persisted data do not change |
| Who else touches this path? | The dependency owns API boot/reload consumers. This spec owns schema, config-infra, AAA startup, reload test placement, composition proof, and schema ownership docs |

## Cross-Specification Ownership

| Surface | Owning spec | Contract |
|---------|-------------|----------|
| REST/gRPC boot and reload resolution, default-composition tests, and API functional proof | `spec-hub-deferred-api-auth-independent-of-ssh-block` | Reuse the existing live merged user source; do not edit YANG or `infraSetup` |
| Spawned `ze_core,ze_rest,!ze_ssh` composition proof | This spec | Build the actual no-SSH binary and prove config-user authentication and authorization |
| Shared user and SSH YANG ownership | This spec | Move the base list and add the gated public-key augment |
| `ExtractAuthUsers`, `SSHExtractedConfig`, and BGP-path AAA | This spec | Move shared extraction, remove `Users`, and make `infraSetup` consume the live source |
| Plaintext reload test placement | This spec | Rename to `main_reload_auth_test.go` and remove SSH/BGP tags |
| Build composition tests | This spec | Prove shared acceptance, SSH rejection, installed seam, and omitted symbols |
| `docs/guide/authentication.md` | This spec | Update both the shared-schema source anchor and the API remedy after the dependency lands |
| Login-service user fields | `spec-login-service-authorisation` after this spec | Add shared user fields only in authz YANG and `infra/authz.go` |
| FIDO2 public-key fields | `spec-ssh-fido2-keys` after this spec | Extend only the gated SSH public-key augment and SSH runtime |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Bare `ze_core` parses a shared user | → | authz YANG and `ExtractAuthUsers` | `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig` |
| Bare `ze_core` parses SSH transport config | → | absent SSH registration | `TestBuildTag_SSH_AbsentRejectsSSHConfig` |
| Bare `ze_core` parses user public keys | → | absent SSH augment | `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys` |
| BGP-path infrastructure has no SSH snapshot | → | shared live users → `infraSetup` → AAA and optional SSH | `TestInfraSetupUsesLiveUsersWithoutSSHSnapshot` |
| Reload reads plaintext-password in bare core | → | `diskConfigLoaders` and password hashing | `TestReloadHashesPlaintextPassword` |
| Tagged build parses current SSH config | → | SSH registration and authz-user augment | `TestBuildTag_SSH_PresentAcceptsSSHConfig` |
| Bare-core binary is linked | → | feature-gated SSH omission | `TestBuildTag_SSH_AbsentBinaryDropsSSHSymbols` checks the gated package and `*Impl` producers, not the always-on nil seams |
| Default daemon accepts password and public-key SSH login | → | shared authz user → SSH augment → SSH server | `test/plugin/ssh-user-login-yang.ci` and `test/plugin/ssh-pubkey-auth.ci` |
| No-BGP/no-SSH management authorizes a config user | → | dependency boot snapshot → AAA → API | `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` |
| BGP-path live user read fails | → | `infraSetup` → local AAA backend | `TestInfraSetupLiveUsersFailureFailsClosed` |
| No-SSH REST binary receives a command | → | authz YANG → no-BGP AAA → REST authenticator/authorizer | `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Status and evidence |
|-------|-------------------|-------------------|---------------------|
| AC-1 | Build and test hub with bare `ze_core` | SSH seam functions remain nil and the binary contains none of `internal/component/ssh/`, `sshBuildImpl`, `sshWireImpl`, or `sshBuildStandaloneImpl`; always-on `sshBuild`, `sshWirePostStart`, and `sshBuildStandalone` seams remain | Done. Supplied no-SSH hub package pass covers `TestBuildTag_SSH_Absent` and `TestBuildTag_SSH_AbsentBinaryDropsSSHSymbols`; `assertNoSSHImplementationSymbols` checks all five implementation needles. |
| AC-2 | Bare-core config declares a password/profile user and no SSH block | Config parses and `ExtractAuthUsers` returns the user | Done. Supplied no-SSH hub package pass covers `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig`. |
| AC-3 | Bare-core config declares `environment.ssh` | Parsing fails with an unknown-field error | Done. Supplied no-SSH hub package pass covers `TestBuildTag_SSH_AbsentRejectsSSHConfig`. |
| AC-4 | Bare-core config declares `public-keys` under a user | Parsing fails with an unknown-field error | Done. Supplied no-SSH hub package pass covers `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys`. |
| AC-5 | Tagged config uses current user, public-key, and SSH listener syntax | Parsing succeeds with the same values and defaults; `ExtractAuthUsers` returns the same public-key names, types, and bytes | Done. Supplied SSH-tag hub/config/SSH package passes cover `TestBuildTag_SSH_PresentAcceptsSSHConfig` and the schema/extractor tests. |
| AC-6 | `infraSetup` receives live users and no `SSHExtractedConfig.Users` field | It installs AAA from a successful live list and passes that list to SSH only when the seam and config exist; a failed live read is logged and local authentication rejects | Done. Supplied full hub race pass covers `TestInfraSetupUsesLiveUsersWithoutSSHSnapshot` and `TestInfraSetupLiveUsersFailureFailsClosed`, including the diagnostic and source error assertions. |
| AC-7 | Bare-core reload reads `plaintext-password` | Reload hashes it into `password`, removes plaintext from the tree, and installs the hashed tree | Done. Supplied no-SSH hub package and full hub race passes cover `TestReloadHashesPlaintextPassword`. |
| AC-8 | Default tagged daemon authenticates a configured SSH user | Correct password and public-key credentials succeed; incorrect credentials fail | Done. Supplied plugin suite passed 605/605, including `ssh-user-login-yang.ci` and `ssh-pubkey-auth.ci`. |
| AC-9 | Documentation and source anchors are checked | Shared users point to authz YANG; SSH listener and public-key claims point to SSH YANG | Done. Source audit found the split anchors in `core-design.md`, `configuration.md`, `operator-access-rbac.md`, `authentication.md`, and `ubuntu-build-install.md`; `aaa-auth.md` states the same ownership. |
| AC-10 | A `ze_core,ze_rest` binary with no `ze_ssh` receives config user credentials | It starts without BGP or SSH, accepts the correct password for an allowed command, rejects a wrong password, denies a command outside the profile, and contains none of the AC-1 gated implementation needles | Done. Supplied no-SSH hub package pass covers the spawned binary, 200/401/403 outcomes, and the symbol scan on that exact REST artifact. |
| AC-11 | Dependency evidence is present | No-BGP/no-SSH-block API startup installs AAA and enforces an authorization profile before this spec removes the field | Done. Dependency spec is closed in `b72f23279` and `82271e95d`; the supplied no-SSH hub and UI 169/169 results cover its composition contract. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds REST without SSH and configures a local management user | `ze_core,ze_rest` → authz YANG → no-BGP AAA → REST | `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` |
| 2 | Builds without SSH and supplies SSH-only config | bare core → schema set → parser refusal | Both absent-tag rejection tests |
| 3 | Runs BGP-path infrastructure without an SSH snapshot | live user source → `infraSetup` → AAA | `TestInfraSetupUsesLiveUsersWithoutSSHSnapshot` |
| 4 | Reloads a bare-core config containing plaintext-password | file → normal loader → bcrypt transform → installed tree | `TestReloadHashesPlaintextPassword` |
| 5 | Uses the default build and logs in with password or public-key SSH config | SSH augment → shared extraction → AAA → SSH server | `test/plugin/ssh-user-login-yang.ci` and `test/plugin/ssh-pubkey-auth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig` | `cmd/ze/hub/build_tag_ssh_absent_test.go` | AC-2 | pass in supplied no-SSH hub package run |
| `TestBuildTag_SSH_AbsentRejectsSSHConfig` and `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys` | same | AC-3 and AC-4 | pass in supplied no-SSH hub package run |
| `TestBuildTag_SSH_AbsentBinaryDropsSSHSymbols` | same | AC-1 | pass in supplied no-SSH hub package run; exact gated package and three `*Impl` producers checked |
| `TestBuildTag_SSH_PresentAcceptsSSHConfig` | `cmd/ze/hub/build_tag_ssh_present_test.go` | AC-5 | pass in supplied SSH-tag hub run; current syntax, defaults, profile, and public key are preserved |
| `TestInfraSetupUsesLiveUsersWithoutSSHSnapshot` | `cmd/ze/hub/infra_setup_auth_test.go` | AC-6 | pass in supplied full hub race run |
| `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` | `cmd/ze/hub/build_tag_ssh_absent_test.go` | AC-10 through a spawned no-SSH binary | pass in supplied no-SSH hub package run |
| `TestInfraSetupLiveUsersFailureFailsClosed` | `cmd/ze/hub/infra_setup_auth_test.go` | AC-6 failed-source behavior | pass in supplied full hub race run; log text, source error, rejected authentication, and skipped SSH construction asserted |
| `TestExtractAuthUsersAvailableWithoutSSHFeature` and `TestSSHExtractedConfigIsTransportOnly` | `internal/component/config/infra/authz_no_ssh_test.go` | AC-1, AC-2, and AC-6 package seam | pass in supplied config/SSH package runs |
| Existing `TestExtractAuthUsers*` and `TestExtractSSHConfig*` | `internal/component/config/infra/authz_test.go` and `ssh_test.go` | Base and optional-key extraction plus unchanged transport values | pass in supplied config/SSH package runs |
| `TestSchema_ZeAuthzOwnsSharedAuthenticationUsers` and `TestSchema_ZeSSHOwnsPublicKeyAugmentOnly` | authz and SSH `yang/schema_test.go` | AC-2, AC-4, and AC-5 ownership | pass in supplied SSH-tag config/SSH package runs |
| `TestSSHFeatureGateFeedsDependencyAudit` plus existing `TestEnginePlacement` | `scripts/dev/*_test.go` | AC-1 dependency boundary | source-audited; the main thread will run the tracked-build gate after the remaining producers are committed |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Feature presence | absent or present | `ze_core,ze_ssh` | N-A | N-A |
| User count | 0 to N | 1, smallest configured identity | 0 denies | N-A: no new bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ssh-user-login-yang` | `test/plugin/ssh-user-login-yang.ci` | Tagged password login and wrong-password refusal remain unchanged | pass in supplied plugin suite, 605/605 |
| `ssh-pubkey-auth` | `test/plugin/ssh-pubkey-auth.ci` | Tagged YANG public-key augment reaches SSH accept/reject behavior | pass in supplied plugin suite, 605/605 |
| `ssh-config-valid` | `test/parse/ssh-config-valid.ci` | Current SSH config syntax still parses | source-audited and covered at the same schema boundary by the supplied tagged hub parser pass; the broad parse target was not rerun |
Bare-core behavior is a build-composition contract. Build-tag tests build the
actual binary and use the actual registered parser, so no new `.ci` runner mode
is required.

### Interop Tests
N-A. No SSH wire behavior changes.

## Files to Modify
- `internal/component/authz/yang/ze-authz-conf.yang` - own the base user list.
- `internal/component/ssh/yang/ze-ssh-conf.yang` - retain transport settings and augment users with public keys.
- `internal/component/config/infra/ssh.go` - remove shared extraction helpers and return transport settings only.
- `internal/component/config/infra/authz.go` - receive `ExtractAuthUsers` and its shared helpers.
- `internal/component/config/infra/hook.go` - remove `SSHExtractedConfig.Users`.
- `internal/component/config/infra/ssh_test.go` and `authz_test.go` - move shared extractor tests without dropping assertions; add no-SSH extraction coverage.
- `cmd/ze/hub/infra_setup.go` - resolve the shared live source once and use it for BGP-path AAA and optional SSH inputs.
- `cmd/ze/hub/build_tag_ssh_absent_test.go` - add schema, binary omission, and spawned no-SSH REST authorization proof.
- `cmd/ze/hub/build_tag_ssh_present_test.go` - add positive schema proof.
- `internal/component/ssh/yang/schema_test.go` - prove the SSH module no longer owns the base user list.
- `cmd/ze/hub/main_reload_auth_test.go` - receive `TestReloadHashesPlaintextPassword` and its helper without changing existing auth reload tests.
- `cmd/ze/hub/ssh_pubkey_live_test.go` - migrate the tagged running-config fixture from removed `SSHExtractedConfig.Users` to the shared live user source without changing assertions.
- `docs/architecture/core-design.md` and `docs/architecture/config/yang-config-design.md` - record schema ownership.
- `docs/guide/operator-access-rbac.md`, `docs/guide/configuration.md`, `docs/guide/authentication.md`, and `docs/guide/ubuntu-build-install.md` - correct source anchors and API remedy.
- `ai/digests/aaa-auth.md` - update the config-to-AAA flow.


## Files to Create
- `internal/component/authz/yang/schema_test.go` - prove the authz module owns the shared user fields.
- `cmd/ze/hub/infra_setup_auth_test.go` - prove live AAA installation with no SSH config or seam.

## Files to Remove
- `cmd/ze/hub/main_reload_ssh_test.go` - remove after its shared test and helper move into the existing untagged auth file.

## Files Explicitly Not Modified
- `feature-gates.txt` and the generator - the existing gate is correct.
- `cmd/ze/hub/ssh_infra.go`, `register_ssh.go`, and `service_ssh.go` - the seam and implementation remain.
- `cmd/ze/hub/main.go`, `cmd/ze/hub/mgmt_auth_reload.go`, and default-composition API tests - dependency-owned; the no-SSH build-composition REST test remains owned here.
- Appliance config and QEMU forwarding - build selection is the requested surface.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | Both authz and SSH YANG modules |
| YANG validation constraints | Yes | Preserve password annotations, public-key enumeration, listener types, and existing constraints |
| YANG custom validators | No | None added |
| CLI commands/flags | No | Config paths remain unchanged |
| CLI grammar | No | No command change |
| Editor autocomplete | Yes | YANG registration automatically adds or removes matching nodes |
| Functional test for new RPC/API | N-A | Dependency owns API behavior |
| Pipe completeness | N-A | No output command |
| Env var registration | No | Existing SSH variables remain unchanged when tagged |
| Doctor check for runtime dependencies | No | No new dependency; existing SSH checks are inert when SSH config is rejected |
| Prometheus counters | No | None |
| BGP family surface | N-A | None |

### Documentation Update Checklist
| # | Question | Applies? | File to update | Closure audit |
|---|----------|----------|----------------|---------------|
| 1 | New user-facing feature? | No | Existing composition is completed | Verified: no new runtime option or config path. |
| 2 | Config syntax changed? | No | Syntax preserved; ownership docs and anchors change | Verified against both YANG producers and tagged/untagged parser tests. |
| 3 | CLI changed? | No | None | Verified: no SSH composition CLI was added. |
| 4 | API/RPC changed? | No | Dependency-owned | Verified against the closed dependency and no-SSH REST proof. |
| 5 | Plugin changed? | No | Existing component gate remains | Verified: `ze_ssh` remains the only registration tag. |
| 6 | User guide page? | Yes | `operator-access-rbac.md`, `configuration.md`, `authentication.md`, `ubuntu-build-install.md` | Verified: base users anchor to authz YANG; listeners and public keys anchor to SSH YANG. |
| 7 | Wire format changed? | No | None | Verified: no protocol encoding changed. |
| 8 | SDK/protocol changed? | No | None | Verified: no public SDK or protocol type changed. |
| 9 | RFC behavior changed? | N-A | None | Verified: build composition and config ownership do not change SSH wire behavior. |
| 10 | Test infrastructure changed? | No | Existing targets used | Verified: build-tag and existing functional runners provide the proof. |
| 11 | Daemon comparison affected? | No | No claim changes | Verified: default build behavior is preserved. |
| 12 | Internal architecture changed? | Yes | `core-design.md`, `yang-config-design.md`, `ai/digests/aaa-auth.md` | Verified: all three describe authz base ownership, the SSH augment, and accepted live-user flow. |
| 13 | Route metadata changed? | No | None | Verified: no route metadata surface is involved. |
| 14 | Metrics changed? | No | None | Verified: no metric producer changed. |
| 15 | Registered inventory changed? | No | Same tagged module set | Verified from `feature-gates.txt`, `all_ze_ssh.go`, and `register_ssh.go`. |
| 16 | Changed files have source anchors? | Yes | Both YANG files and extraction flow | Verified by source-anchor audit across the named architecture and guide files. |
| 17 | Existing examples cover this area? | Yes | User, public-key, and SSH examples | Verified by tagged/untagged parser tests and the supplied plugin 605/605 result. |

## Implementation Steps

1. **Phase: Wiring** - add failing composition, AAA, and ungated reload tests.
   - Tests: all rows in the Wiring Test table.
   - Verify: failures name missing schema ownership or wiring, not setup.
   - Status: complete; all named composition tests passed in the supplied no-SSH and SSH-tag hub runs.
   - Source evidence: authz YANG owns `authentication`; SSH YANG owns only the public-key augment and `environment.ssh`; the manifest and generated import retain the single `ze_ssh` gate.
2. **Phase: Schema ownership** - move the base list and add the gated public-key augment.
   - Tests: tagged and untagged parser tests, existing parse functional test.
   - Verify: operator config text is unchanged; bare core accepts only shared fields.
   - Status: complete; focused schema and tagged/untagged parser tests passed, and default SSH workflows passed in the supplied plugin suite.
   - Source evidence: `ze-authz-conf.yang` is the sole base `system.authentication.user` owner; `ze-ssh-conf.yang` imports authz, augments that exact list with `public-keys`, and retains `environment.ssh`.
3. **Phase: Shared extraction and BGP-path AAA** - move `ExtractAuthUsers` into `infra/authz.go`, remove `SSHExtractedConfig.Users`, and resolve `liveUsers` once in `infraSetup`.
   - Tests: config-infra tests and the no-snapshot AAA wiring test.
   - Verify: source references show no removed field use and dependency tests prove no-BGP AAA.
   - Status: complete; supplied config/SSH package and full hub race runs passed.
   - Source evidence: `ExtractAuthUsers` and its helpers live in `infra/authz.go`; `SSHExtractedConfig` is transport-only; `infraSetup` resolves the accepted source once and fails closed on read errors.
4. **Phase: Reload test ownership** - rename the test file and remove feature tags.
   - Tests: `TestReloadHashesPlaintextPassword` in default and bare-core package runs.
   - Verify: test passes through `diskConfigLoaders` and `doReload` without SSH registration.
   - Status: complete in the supplied no-SSH hub and full hub race runs.
5. **Phase: Artifact and functional preservation** - prove binary omission and default SSH behavior.
   - Tests: symbol test and existing functional tests.
   - Verify: bare core omits component symbols; tagged login is unchanged.
   - Status: complete in the supplied no-SSH hub pass and plugin suite, 605/605.
6. **Phase: Documentation** - update ownership tables, anchors, and digest.
   - Verify: source audit found no named ownership anchor attributing shared users to SSH YANG.

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| Completeness | AC-1 through AC-11 map to named tests and files |
| Feature completeness | Bare core and tagged composition are both tested |
| Correctness | Base users survive, SSH nodes vanish, tagged values/defaults remain identical |
| Naming | Shared extraction uses auth/user names; transport remains SSH-named |
| Data flow | No identity crosses `SSHExtractedConfig`; no-BGP main and BGP `infraSetup` consume the same shared source |
| Test ownership | Plaintext reload coverage has no SSH or BGP build gate |
| Simplicity | No new registry, runtime option, alias, or compatibility field |

### Deliverables Checklist
| Deliverable | Verification method | Closure status and evidence |
|-------------|---------------------|-----------------------------|
| Bare-core schema and artifact proof | No-SSH hub package run | Done. Supplied pass includes shared-user parsing, SSH-only rejection, binary symbol omission, and spawned REST authentication/authorization. |
| Shared config extraction | Config/SSH package runs | Done. Supplied SSH-tag config/SSH package tests passed; source shows `ExtractAuthUsers` in authz and transport-only `SSHExtractedConfig`. |
| YANG ownership | Authz and SSH YANG package tests | Done. Supplied package passes cover both ownership tests. |
| Default SSH workflow | Plugin and parser coverage | Done. Supplied plugin suite passed 605/605; tagged hub parser proof preserved syntax and defaults. |
| Generated composition | Manifest, generator, generated import, and tracked-build check | Source-complete. `ze_ssh` is still the sole gate; the main thread will run the tracked-build check after committing the remaining producers. |
| Documentation | Source-anchor audit | Done. Every named ownership anchor and architecture statement was checked. Broad doc targets were not rerun under this closure constraint. |
| Go quality | Focused package tests and race run | Done. Supplied no-SSH and SSH-tag package runs passed, and the full hub race passed. No lint command was rerun. |

### Security Review Checklist
| Check | What to look for | Closure status and evidence |
|-------|------------------|-----------------------------|
| Fail closed | Missing or malformed users produce no authenticator, never anonymous access | Clean. `infraSetup` logs and withholds local users on source failure; accepted API staging requires authentication. |
| SSH-only credentials | Bare core rejects public keys instead of ignoring them | Clean. `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys` passed. |
| Secret handling | Bcrypt, ephemeral, and sensitive annotations move intact; no secret is logged | Clean. Authz YANG retains all annotations; reload hashing passed and diagnostics contain no credential material. |
| Authorization | Profile lists reach the same authorizer | Clean. Accepted identity publication binds users and the authz store to one generation; allowed and denied REST outcomes passed. |
| Recovery identity | Zefs precedence and reserved recovery profile remain unchanged | Clean after review fixes bound recovery grants to the accepted generation and covered same-name replacement. |
| Artifact omission | No untagged import links `internal/component/ssh` | Clean. Both bare-core and `ze_core,ze_rest` artifacts run the exact implementation-symbol scan. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Bare-core user unknown | Fix authz schema ownership |
| Bare core accepts SSH nodes | Fix gated SSH module or augment |
| Tagged config rejected | Fix augment import/path, not operator syntax |
| API or no-BGP AAA still reads the SSH field | Finish the dependency; do not add a compatibility path |
| BGP-path AAA has no config user | Fix `liveUsers` resolution or wiring; do not restore the field |
| Reload test fails without tags | Fix a real shared dependency; do not restore the SSH tag |
| Binary finds a gated SSH implementation needle | Gate the importing producer; retain the always-on nil seam and do not weaken the exact needles |
| Functional behavior changes | Restore preserved behavior in the responsible phase |

## Design Insights

- Compile-time omission already works at the package boundary. The unfinished
  part is shared data ownership and artifact proof.
- The schema needs a base-plus-augment shape. Moving all fields to authz leaks
  SSH public keys. Keeping the base in SSH preserves the defect.
- API migration must precede shared ownership cleanup. It removes the `main.go`
  field consumers and installs AAA on the no-BGP path.
- The plaintext reload test belongs to shared authentication. Its current file
  name and tags encode the coupling this work removes.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| Authz base user plus SSH public-key augment | Move all fields to authz; keep an always-on SSH user module | Shared identity survives while SSH-only syntax disappears |
| Reuse `ze_ssh` seam | Convert SSH to generic service registry | Existing seam already omits the package |
| Keep optional public-key parsing in `ExtractAuthUsers` | Retain the whole user snapshot in `SSHExtractedConfig`; add a second user parser | One parser preserves base and registered augment values; schema gating controls accepted input and no SSH package is imported |
| Resolve `liveUsers` once in `infraSetup` | Parse `ConfigTree` again; add an `AuthUsers` field | One live source already crosses the hook and supplies AAA plus SSH |
| Remove `SSHExtractedConfig.Users` after the dependency | Retain an SSH-only compatibility field | All construction paths then consume shared users directly |
| Ungate and rename plaintext reload test | Add a second bare-core copy | One test should prove one shared behavior in every composition |
| Match web's symbol proof | Purge every lexical SSH string | Exact package and gated `*Impl` symbols prove component linkage while allowing the always-on nil seam |
| Update existing composition tests and move the plaintext reload test | Preserve SSH-gated coverage or add duplicate tests | Owner approved the test-contract updates; each shared behavior has one composition-independent test |

## Known Limitations
- `ze-stripped` continues to include `ze_ssh` by design. Bare `ze_core` is the no-SSH proof lane.
- Runtime enable/disable behavior is unchanged. This spec concerns build composition.
- Appliance recovery and QEMU forwarding continue to use an SSH-enabled build.
- API boot/reload logic belongs to the dependency spec. This spec owns only the no-SSH binary composition proof.

## Checklist

### Goal Gates
- [x] AC-1 through AC-11 demonstrated
- [x] Every user story has a named test
- [x] Wiring table complete
- [x] Both integration and documentation checklists answered
- [x] Architectural and critical review complete
- [x] Every assumption validated during implementation
- [x] Dependency implemented before field removal
- [x] No live deferral row

### TDD
- [x] Tests written first
- [ ] Initial red-run output was not retained in the supplied evidence; final contract tests pass
- [x] Tests PASS after implementation
- [x] Both feature-tag boundaries tested
- [x] Functional SSH preservation tests pass
- [x] Interop N-A: no wire behavior change

### Closure
- [x] Append and complete `plan/TEMPLATE-CLOSURE.md`
- [ ] Independent review gate clean and recorded
- [x] Learned outcome routed to architecture documentation
- [ ] Commit A contains code, tests, docs, and spec
- [ ] `make ze-precommit-verify`
- [ ] Commit B removes the spec only after closure

---

## Implementation Summary

### What Was Implemented
- Moved the base `system.authentication.user` schema, including password, plaintext-password, and profile fields, to always-on authz ownership.
- Kept `environment.ssh` and the `public-keys` user augment in the `ze_ssh`-gated SSH YANG module.
- Made `SSHExtractedConfig` transport-only. `ExtractAuthUsers` is the shared identity parser, and `infraSetup` gives AAA and optional SSH one accepted user generation.
- Preserved the existing feature manifest, generated `all_ze_ssh.go` import, and always-on nil seam as the only composition mechanism.
- Added bare-core and `ze_core,ze_rest` schema, authentication, authorization, and linked-symbol omission proof while preserving tagged SSH password and public-key workflows.

### Bugs Found/Fixed
- The first no-SSH REST proof did not scan its own artifact for SSH symbols. `assertNoSSHImplementationSymbols` now checks both the bare-core and REST binaries.
- A failed `infraSetup` live-user read denied authentication but its diagnostic was not defended. `TestInfraSetupLiveUsersFailureFailsClosed` now asserts the warning and original error.
- Reload initially exposed candidate credentials separately from policy. `acceptedLocalIdentityState`, `stageAPIAuthentication`, and `publishAcceptedLocalIdentity` now publish accepted users, policy, and API authentication as one generation.
- An absent API block could change credentials on a listener that remained live. `apiAuthReloader` and `candidateAPIToken` now retain the accepted token while still evaluating candidate users.
- Username-global recovery profile state could survive a same-name identity replacement. Accepted-generation profile binding now invalidates stale recovery grants.
- BGP startup could reach a nil AAA authorizer before infrastructure wiring completed. Boot bundle ownership now installs or claims the fail-closed bundle before management dispatch.
- Text command transports could run shutdown completion before delivering the response. Transport-specific completion now follows accepted-action delivery across SSH, web, CLI, and plugin paths.

### Documentation Updates
- `docs/architecture/core-design.md` and `docs/architecture/config/yang-config-design.md` now anchor shared users to authz YANG and the optional listener/public-key nodes to SSH YANG.
- `docs/guide/operator-access-rbac.md`, `docs/guide/configuration.md`, `docs/guide/authentication.md`, and `docs/guide/ubuntu-build-install.md` carry the same source split.
- `ai/digests/aaa-auth.md` records the config-to-accepted-generation flow.
- Broad documentation commands were not rerun in this pass. The closure audit read the source anchors directly, as required by the no-broad-command constraint.

### Deviations from Plan
- Final adversarial review widened the dependency-owned authentication work into atomic accepted-generation publication, recovery-profile lifecycle, fail-closed boot authorization, and post-delivery lifecycle completion. These fixes were required to preserve the SSH and no-SSH management contracts under reload and shutdown.
- The API dependency landed first and is closed in `b72f23279` and `82271e95d`, as required by AC-11.
- The final tracked-build check and machine review artifact remain with the main thread because several reviewed producers are still uncommitted. Recording before their final content is present would create a stale hash-pinned artifact.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spawned no-SSH REST proof was treated as sufficient without scanning that exact binary. | Every independently composed artifact needs its own exact package and `*Impl` symbol scan. | Independent composition review. | Reused `assertNoSSHImplementationSymbols` on the REST binary. |
| approach | Candidate users, policy, and API credentials were updated at different reload boundaries. | Authentication and authorization must come from one accepted generation, with fail-closed staging while listeners move. | Lifecycle and security reviews. | Added atomic accepted identity publication and candidate staging. |
| approach | Recovery profile state was keyed by username without an accepted-generation lifetime. | A same-name replacement is a different identity and must not inherit recovery authority. | Authentication security review. | Bound local profile records to the accepted generation and invalidated stale state. |
| approach | Lifecycle completion was attached to JSON conversion rather than actual response delivery. | Shutdown and restart actions can run only after each transport has delivered the accepted response. | Lifecycle review. | Moved completion to transport delivery boundaries. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Omit SSH implementation and schema without `ze_ssh` | Done | `feature-gates.txt`; `internal/component/plugin/all/all_ze_ssh.go`; `cmd/ze/hub/register_ssh.go` | One existing feature gate controls registration and implementation seams. |
| Retain shared password/profile users without SSH | Done | `internal/component/authz/yang/ze-authz-conf.yang`; `internal/component/config/infra/authz.go:ExtractAuthUsers` | Bare-core parser and REST authentication proof passed. |
| Keep SSH public keys and transport config gated | Done | `internal/component/ssh/yang/ze-ssh-conf.yang` | The module owns only the user augment and `environment.ssh`. |
| Remove shared identity from SSH transport extraction | Done | `internal/component/config/infra/hook.go:SSHExtractedConfig`; `internal/component/config/infra/ssh.go:ExtractSSHConfig` | The extracted structure contains transport fields only. |
| Feed AAA and optional SSH from one accepted source | Done | `cmd/ze/hub/aaa_lifecycle.go:liveAcceptedLocalUsers`; `cmd/ze/hub/infra_setup.go:infraSetup` | One resolved snapshot constructs both paths; read errors fail closed. |
| Preserve tagged syntax, login, host-key, session, and default behavior | Done | `cmd/ze/hub/build_tag_ssh_present_test.go`; `test/plugin/ssh-user-login-yang.ci`; `test/plugin/ssh-pubkey-auth.ci` | Tagged package passes and plugin suite 605/605 supplied. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBuildTag_SSH_Absent`; `TestBuildTag_SSH_AbsentBinaryDropsSSHSymbols` | Supplied no-SSH hub pass. |
| AC-2 | Done | `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig` | Supplied no-SSH hub pass. |
| AC-3 | Done | `TestBuildTag_SSH_AbsentRejectsSSHConfig` | Supplied no-SSH hub pass. |
| AC-4 | Done | `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys` | Supplied no-SSH hub pass. |
| AC-5 | Done | `TestBuildTag_SSH_PresentAcceptsSSHConfig` | Supplied SSH-tag hub/config/SSH package passes. |
| AC-6 | Done | `TestInfraSetupUsesLiveUsersWithoutSSHSnapshot`; `TestInfraSetupLiveUsersFailureFailsClosed` | Supplied full hub race pass. |
| AC-7 | Done | `TestReloadHashesPlaintextPassword` | Supplied no-SSH hub and full hub race passes. |
| AC-8 | Done | `ssh-user-login-yang.ci`; `ssh-pubkey-auth.ci` | Supplied plugin suite passed 605/605. |
| AC-9 | Done | Source-anchor audit recorded below | Shared and SSH-only ownership is accurate. |
| AC-10 | Done | `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` | Supplied no-SSH hub pass covers the binary, symbols, 200, 401, and 403 outcomes. |
| AC-11 | Done | Closed dependency plus no-SSH REST and UI evidence | Dependency commits are `b72f23279` and `82271e95d`; UI suite passed 169/169. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Bare-core shared-user parsing | Done | `cmd/ze/hub/build_tag_ssh_absent_test.go` | Supplied no-SSH hub pass. |
| Bare-core SSH transport and public-key rejection | Done | `cmd/ze/hub/build_tag_ssh_absent_test.go` | Both negative parser tests passed. |
| Bare-core and REST artifact symbol omission | Done | `cmd/ze/hub/build_tag_ssh_absent_test.go` | Both artifacts use the same exact needle scan. |
| Tagged schema and extractor preservation | Done | `cmd/ze/hub/build_tag_ssh_present_test.go` | Supplied tagged hub pass. |
| Shared user extraction and transport-only SSH result | Done | `internal/component/config/infra/*_test.go` | Supplied config/SSH package passes. |
| Authz base and SSH augment ownership | Done | `internal/component/authz/yang/schema_test.go`; `internal/component/ssh/yang/schema_test.go` | Supplied schema package passes. |
| BGP-path accepted users and failure handling | Done | `cmd/ze/hub/infra_setup_auth_test.go` | Supplied full hub race pass. |
| Bare-core plaintext reload hashing | Done | `cmd/ze/hub/main_reload_auth_test.go` | Supplied no-SSH hub and race passes. |
| No-SSH REST authentication and authorization | Done | `cmd/ze/hub/build_tag_ssh_absent_test.go` | Spawned composition passed. |
| Tagged password SSH workflow | Done | `test/plugin/ssh-user-login-yang.ci` | Supplied plugin suite, 605/605. |
| Tagged public-key SSH workflow | Done | `test/plugin/ssh-pubkey-auth.ci` | Supplied plugin suite, 605/605. |
| Feature registration and generated composition | Done | `scripts/codegen/plugin_imports.go`; `internal/component/plugin/all/all_ze_ssh.go` | Source audit complete; tracked-build gate remains for the main thread. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/authz/yang/ze-authz-conf.yang` | Done | Owns base users and authorization profiles. |
| `internal/component/ssh/yang/ze-ssh-conf.yang` | Done | Owns the public-key augment and SSH transport only. |
| `internal/component/config/infra/authz.go` | Done | Owns shared user extraction. |
| `internal/component/config/infra/ssh.go` | Done | Transport extraction only. |
| `internal/component/config/infra/hook.go` | Done | `SSHExtractedConfig` has no users. |
| `internal/component/config/infra/authz_test.go`, `ssh_test.go`, and `authz_no_ssh_test.go` | Done | Extraction ownership and both compositions covered. |
| `cmd/ze/hub/infra_setup.go` | Done | Uses one accepted user snapshot and fails closed. |
| `cmd/ze/hub/build_tag_ssh_absent_test.go` | Done | Covers schema, binaries, REST auth, and omission. |
| `cmd/ze/hub/build_tag_ssh_present_test.go` | Done | Covers seam installation and tagged syntax/defaults. |
| `internal/component/ssh/yang/schema_test.go` | Done | Covers augment-only ownership. |
| `internal/component/authz/yang/schema_test.go` | Done | Covers always-on base ownership. |
| `cmd/ze/hub/infra_setup_auth_test.go` | Done | Covers common snapshot and read failure. |
| `cmd/ze/hub/main_reload_auth_test.go` | Done | Contains the ungated plaintext reload proof. |
| `cmd/ze/hub/main_reload_ssh_test.go` | Done | Removed after its shared test moved. |
| `cmd/ze/hub/ssh_pubkey_live_test.go` | Done | Uses the accepted live-user source. |
| Named architecture and guide documents | Done | Ownership and source anchors audited. |
| `ai/digests/aaa-auth.md` | Done | Accepted identity flow recorded. |
| `feature-gates.txt`, `register_ssh.go`, and the existing seam | Done | Intentionally preserved as the single composition mechanism. |

### Audit Summary
- **Total items:** 47
- **Done:** 47
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0. Review-driven scope expansion is recorded under Deviations from Plan.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A build without `ze_ssh` omits the SSH component and schema. | Build-composition integration | Supplied no-SSH hub pass covers nil seams, schema rejection, and `go tool nm` omission for both `ze_core` and `ze_core,ze_rest`. |
| Non-SSH management surfaces retain password/profile users. | Spawned functional composition | `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` passed with correct password allow, wrong password rejection, and out-of-profile denial. |
| SSH public keys remain available only with SSH compiled in. | Positive and negative parser boundary | The absent public-key rejection and tagged value-preservation tests passed. |
| Default builds retain SSH behavior and existing syntax. | Functional and parser preservation | The supplied plugin suite passed 605/605, including password and public-key SSH workflows; the tagged hub parser test passed. |
| Composition continues to use the existing registration pattern. | Source and artifact audit | `feature-gates.txt`, generated `all_ze_ssh.go`, `register_ssh.go`, and `ssh_infra.go` retain the single `ze_ssh` path; no second registry or runtime toggle was added. |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| None. The spec metadata names no deferral shard. | done | No live row exists to move, cancel, or remove. |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ssh-optional-composition-95ead384-f7b2-4a4a-9286-268f9021bd63.md` |
| `review_gate.py check` | `OK`, 28 code files, clean, hashes match. |
| Rounds | 5. Round 4 found product defects in rejected-candidate credential rollback, recovery-profile carryover, BGP startup authorization, and response completion ordering; round 5 was clean. |
| Reviewer lenses used | Build composition and schema; lifecycle and concurrency; authentication and authorization security; documentation and source anchors. |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The no-SSH REST artifact was not scanned for SSH implementation symbols. | `cmd/ze/hub/build_tag_ssh_absent_test.go` | `assertNoSSHImplementationSymbols` now checks that exact binary. |
| 2 | ISSUE | Shared-user documentation still pointed at SSH YANG. | Named architecture and guide source anchors | Anchors now point base fields at authz YANG and optional nodes at SSH YANG. |
| 3 | ISSUE | Failed live-user reads were not defended by a diagnostic assertion. | `cmd/ze/hub/infra_setup_auth_test.go` | `TestInfraSetupLiveUsersFailureFailsClosed` asserts the warning and original error. |
| 4 | BLOCKER | Candidate credentials and accepted authorization policy could be observed from different reload generations. | `cmd/ze/hub/aaa_lifecycle.go`; `cmd/ze/hub/main_reload.go` | `acceptedLocalIdentityState`, `stageAPIAuthentication`, and final atomic publication. |
| 5 | BLOCKER | Removing `environment.api-server` could change credentials on a listener that stayed live. | `cmd/ze/hub/mgmt_auth_reload.go` | `apiAuthReloader` and `candidateAPIToken` preserve accepted absent-block credentials. |
| 6 | BLOCKER | A same-name replacement could inherit a stale recovery profile. | AAA profile recording and accepted identity publication | Accepted-generation binding invalidates stale recovery authority. |
| 7 | BLOCKER | BGP startup could authorize through a nil bundle before infrastructure startup completed. | `cmd/ze/hub/aaa_lifecycle.go`; `cmd/ze/hub/infra_setup.go`; `cmd/ze/hub/main.go` | Single boot-bundle ownership and fail-closed startup wiring. |
| 8 | ISSUE | Text transports could trigger lifecycle completion before delivering the response. | Plugin dispatch, SSH, web, and CLI transport paths | Accepted actions now complete at each transport delivery boundary. |

Final independent source review after these fixes: **0 BLOCKER, 0 ISSUE**.

The recorded artifact is hash-pinned to the final 28-file implementation set:

```sh
python3 scripts/dev/review_gate.py record \
  --spec plan/spec-ssh-optional-composition.md \
  --verdict clean \
  --rounds 5 \
  --rounds-reason "Round 4 found rejected-candidate credential rollback, stale recovery-profile carryover, BGP startup authorization fail-open, and lifecycle completion before response delivery; round 5 verified the fixes clean." \
  --reviewers "composition+schema,lifecycle+concurrency,authentication+security,documentation+anchors" \
  --files \
  cmd/ze/hub/aaa_authenticator_web.go \
  cmd/ze/hub/build_tag_ssh_absent_test.go \
  cmd/ze/hub/build_tag_ssh_present_test.go \
  cmd/ze/hub/build_tag_ssh_probe_test.go \
  cmd/ze/hub/main.go \
  cmd/ze/hub/main_reload_auth_test.go \
  cmd/ze/hub/service_registry.go \
  cmd/ze/hub/ssh_infra.go \
  cmd/ze/hub/ssh_pubkey_live_test.go \
  internal/component/config/infra/ssh.go \
  internal/component/config/infra/ssh_test.go \
  internal/component/ssh/passwordauth.go \
  internal/component/ssh/passwordauth_test.go \
  internal/component/ssh/pubkey.go \
  internal/component/ssh/session.go \
  internal/component/ssh/yang/schema_test.go \
  internal/component/ssh/yang/ze-ssh-conf.yang \
  internal/component/plugin/all/all_ze_ssh.go \
  scripts/codegen/plugin_imports.go \
  internal/component/web/sse_snapshot.go \
  internal/component/cli/all_import_test.go \
  internal/component/cli/contract/contract.go \
  internal/component/cli/model.go \
  internal/component/cli/model_dashboard.go \
  internal/component/cli/model_mode.go \
  internal/component/cli/model_mode_test.go \
  internal/component/cli/transcript.go \
  internal/component/cli/transcript_test.go

python3 scripts/dev/review_gate.py check \
  --spec plan/spec-ssh-optional-composition.md \
  --files \
  cmd/ze/hub/aaa_authenticator_web.go \
  cmd/ze/hub/build_tag_ssh_absent_test.go \
  cmd/ze/hub/build_tag_ssh_present_test.go \
  cmd/ze/hub/build_tag_ssh_probe_test.go \
  cmd/ze/hub/main.go \
  cmd/ze/hub/main_reload_auth_test.go \
  cmd/ze/hub/service_registry.go \
  cmd/ze/hub/ssh_infra.go \
  cmd/ze/hub/ssh_pubkey_live_test.go \
  internal/component/config/infra/ssh.go \
  internal/component/config/infra/ssh_test.go \
  internal/component/ssh/passwordauth.go \
  internal/component/ssh/passwordauth_test.go \
  internal/component/ssh/pubkey.go \
  internal/component/ssh/session.go \
  internal/component/ssh/yang/schema_test.go \
  internal/component/ssh/yang/ze-ssh-conf.yang \
  internal/component/plugin/all/all_ze_ssh.go \
  scripts/codegen/plugin_imports.go \
  internal/component/web/sse_snapshot.go \
  internal/component/cli/all_import_test.go \
  internal/component/cli/contract/contract.go \
  internal/component/cli/model.go \
  internal/component/cli/model_dashboard.go \
  internal/component/cli/model_mode.go \
  internal/component/cli/model_mode_test.go \
  internal/component/cli/transcript.go \
  internal/component/cli/transcript_test.go
```

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/authz/yang/schema_test.go` | Yes | Closure path audit found the created ownership test. |
| `cmd/ze/hub/infra_setup_auth_test.go` | Yes | Closure path audit found the created wiring test. |
| `cmd/ze/hub/main_reload_auth_test.go` | Yes | Closure path audit found the ungated replacement file. |
| `cmd/ze/hub/main_reload_ssh_test.go` | Removed | Closure path audit confirmed the obsolete SSH-gated path is absent. |
| Named YANG, config-infra, hub, SSH, documentation, and digest files | Yes | Every planned producer and directly cited source anchor was read during the closure audit. |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No-SSH seams remain nil and implementation symbols are absent. | Supplied no-SSH hub pass; source audit of `TestBuildTag_SSH_Absent` and `assertNoSSHImplementationSymbols`. |
| AC-2 | Shared users parse and extract without SSH. | Supplied pass for `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig`. |
| AC-3 | No-SSH rejects `environment.ssh`. | Supplied pass for `TestBuildTag_SSH_AbsentRejectsSSHConfig`. |
| AC-4 | No-SSH rejects `public-keys`. | Supplied pass for `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys`. |
| AC-5 | Tagged syntax, values, and defaults remain. | Supplied tagged hub/config/SSH package passes. |
| AC-6 | BGP-path AAA and optional SSH share accepted users and fail closed. | Supplied full hub race pass for both `infraSetup` tests. |
| AC-7 | Reload hashes and removes plaintext independently of SSH. | Supplied no-SSH hub and full hub race passes. |
| AC-8 | Tagged SSH password and public-key behavior remains. | Supplied plugin result, 605/605. |
| AC-9 | Source anchors describe the ownership split. | Closure source-anchor audit of all named documents. |
| AC-10 | No-SSH REST authenticates and authorizes a config user with no SSH symbols. | Supplied no-SSH hub pass for the spawned REST composition. |
| AC-11 | Dependency behavior landed first. | Closed commits `b72f23279` and `82271e95d`; UI suite passed 169/169. |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Bare `ze_core` shared user | Build-tag Go integration | Yes, supplied no-SSH hub pass. |
| Bare `ze_core` SSH transport config | Build-tag Go integration | Yes, unknown-field rejection passed. |
| Bare `ze_core` SSH public key | Build-tag Go integration | Yes, unknown-field rejection passed. |
| BGP-path infrastructure without SSH snapshot | Hub wiring Go test | Yes, supplied full hub race pass. |
| Bare-core plaintext reload | Hub reload Go test | Yes, supplied no-SSH hub and race passes. |
| Tagged current SSH config | Tagged hub Go test | Yes, supplied tagged hub pass. |
| Bare-core linked artifact | Build-tag binary test | Yes, exact implementation needles checked. |
| Tagged password login | `test/plugin/ssh-user-login-yang.ci` | Yes, supplied plugin suite 605/605. |
| Tagged public-key login | `test/plugin/ssh-pubkey-auth.ci` | Yes, supplied plugin suite 605/605. |
| No-SSH REST config user | Spawned build-tag Go integration | Yes, supplied no-SSH hub pass. |
| Failed live-user read | Hub wiring Go test | Yes, supplied full hub race pass, including log and denial assertions. |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Always-on aggregator imports authz YANG; absent-tag shared-user test passed. |
| A-2 | confirmed | Tagged and untagged parser proofs preserve the path and values. |
| A-3 | confirmed | Tagged public-key augment test passed. |
| A-4 | confirmed | `setupInfraHook` and `infraSetup` source plus wiring tests use the accepted live source. |
| A-5 | confirmed | Dependency is closed and no-SSH REST authorization passed. |
| A-6 | confirmed | Ungated reload test passed in no-SSH and full race runs. |
| A-7 | confirmed | Spawned `ze_core,ze_rest` binary started and served authenticated requests without SSH. |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Base user ownership | Authz YANG lines defining `system.authentication.user` and matching guide anchors | Yes |
| SSH-only ownership | SSH YANG public-key augment and `environment.ssh`, with matching guide anchors | Yes |
| Architecture and data flow | `core-design.md`, `yang-config-design.md`, and `ai/digests/aaa-auth.md` | Yes |
| Config syntax examples | Tagged and untagged parser tests against the registered YANG modules | Yes |
| RFC, doctor, CLI, API, SDK, wire, metric, and comparison categories | No new runtime dependency, command, RPC, wire behavior, metric, or support-level claim was introduced | Yes |

The supplied verification record is: no-SSH hub package pass; SSH-tag hub/config/SSH package passes; full hub race pass; UI suite 169/169; plugin suite 605/605. No build, test, lint, documentation suite, pre-commit, git, or commit command was run during this closure edit.

## Core Insight

An optional transport cannot own identities shared by other management surfaces. The stable boundary is an always-on identity and authorization generation, with transport-specific schema and construction attached only through the existing feature-gated registration seam.

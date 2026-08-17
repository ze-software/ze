# Spec: make SSH an optional build component

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin, config |
| Depends | - |
| Phase | 3/6 |
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
| A-2 | Moving the base list preserves config text and tree paths | Parser and extractors use unprefixed container names | Existing config needs migration | Present/absent parse tests and `make ze-functional-parse-test` | confirmed by focused present/absent parser tests; functional parse suite pending |
| A-3 | The SSH augment resolves after registered modules load | Existing plugin schemas use cross-module augments | Tagged builds reject public keys | Present-tag SSH config test | confirmed by `TestBuildTag_SSH_PresentAcceptsSSHConfig` |
| A-4 | Every production `infraSetup` call receives the shared live user source | `setupInfraHook` threads `localUsers` into the hook | BGP-path AAA has no current users | Wiring test and LSP references | confirmed by source |
| A-5 | The dependency removes every `main.go` field consumer and installs no-BGP AAA | Its ownership table and ACs name both paths | Field removal breaks main or bare-core authorization stays absent | Dependency verification evidence | confirmed by `resolveBootUsers` and no-BGP `buildAAABundle` source |
| A-6 | The plaintext reload test has no SSH-only dependency | Its producer is `diskConfigLoaders`; its reactor helper is untagged | Moving it causes a compile failure | Bare-core package test | confirmed by source |
| A-7 | REST can run without the SSH feature tag | REST and SSH have separate manifest entries and seams | The binary does not link or start | Spawned `ze_core,ze_rest` integration test | unvalidated |

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

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Build and test hub with bare `ze_core` | SSH seam functions remain nil and the binary contains none of `internal/component/ssh/`, `sshBuildImpl`, `sshWireImpl`, or `sshBuildStandaloneImpl`; always-on `sshBuild`, `sshWirePostStart`, and `sshBuildStandalone` seams remain |
| AC-2 | Bare-core config declares a password/profile user and no SSH block | Config parses and `ExtractAuthUsers` returns the user |
| AC-3 | Bare-core config declares `environment.ssh` | Parsing fails with an unknown-field error |
| AC-4 | Bare-core config declares `public-keys` under a user | Parsing fails with an unknown-field error |
| AC-5 | Tagged config uses current user, public-key, and SSH listener syntax | Parsing succeeds with the same values and defaults; `ExtractAuthUsers` returns the same public-key names, types, and bytes |
| AC-6 | `infraSetup` receives live users and no `SSHExtractedConfig.Users` field | It installs AAA from a successful live list and passes that list to SSH only when the seam and config exist; a failed live read is logged and local authentication rejects |
| AC-7 | Bare-core reload reads `plaintext-password` | Reload hashes it into `password`, removes plaintext from the tree, and installs the hashed tree |
| AC-8 | Default tagged daemon authenticates a configured SSH user | Correct password and public-key credentials succeed; incorrect credentials fail |
| AC-9 | Documentation and source anchors are checked | Shared users point to authz YANG; SSH listener and public-key claims point to SSH YANG |
| AC-10 | A `ze_core,ze_rest` binary with no `ze_ssh` receives config user credentials | It starts without BGP or SSH, accepts the correct password for an allowed command, rejects a wrong password, denies a command outside the profile, and contains none of the AC-1 gated implementation needles |
| AC-11 | Dependency evidence is present | No-BGP/no-SSH-block API startup installs AAA and enforces an authorization profile before this spec removes the field |

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
| `TestBuildTag_SSH_AbsentAcceptsSharedUserConfig` | `cmd/ze/hub/build_tag_ssh_absent_test.go` | AC-2 | pass in the main-owned bare-core focused run |
| `TestBuildTag_SSH_AbsentRejectsSSHConfig` and `TestBuildTag_SSH_AbsentRejectsSSHPublicKeys` | same | AC-3 and AC-4 | pass in the main-owned bare-core focused run |
| `TestBuildTag_SSH_AbsentBinaryDropsSSHSymbols` | same | AC-1 | written; exact gated package and three `*Impl` producers checked |
| `TestBuildTag_SSH_PresentAcceptsSSHConfig` | `cmd/ze/hub/build_tag_ssh_present_test.go` | AC-5 | pass in the main-owned tagged focused run; current syntax, defaults, profile, and public key are preserved |
| `TestInfraSetupUsesLiveUsersWithoutSSHSnapshot` | `cmd/ze/hub/infra_setup_auth_test.go` | AC-6 | production source now resolves the shared live list once and feeds the same snapshot to AAA and optional SSH; validation intentionally not run in this phase assignment |
| `TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser` | `cmd/ze/hub/build_tag_ssh_absent_test.go` | AC-10 through a spawned no-SSH binary | written; Phase 2 removed the shared-schema blocker in source; end-to-end validation remains with the main suite owner |
| `TestInfraSetupLiveUsersFailureFailsClosed` | `cmd/ze/hub/infra_setup_auth_test.go` | AC-6 failed-source behavior | production source logs the read error, withholds local users and the live callback from AAA, and skips optional SSH construction; validation intentionally not run in this phase assignment |
| `TestExtractAuthUsersAvailableWithoutSSHFeature` and `TestSSHExtractedConfigIsTransportOnly` | `internal/component/config/infra/authz_no_ssh_test.go` | AC-1, AC-2, and AC-6 package seam | production source now keeps `ExtractAuthUsers` in authz extraction and makes `SSHExtractedConfig` transport-only; validation intentionally not run in this phase assignment |
| Existing `TestExtractAuthUsers*` and `TestExtractSSHConfig*` | `internal/component/config/infra/authz_test.go` and `ssh_test.go` | Base and optional-key extraction plus unchanged transport values | shared extractor tests moved to authz ownership and SSH tests now consume the shared extractor without dropping their field assertions; validation intentionally not run in this phase assignment |
| `TestSchema_ZeAuthzOwnsSharedAuthenticationUsers` and `TestSchema_ZeSSHOwnsPublicKeyAugmentOnly` | authz and SSH `yang/schema_test.go` | AC-2, AC-4, and AC-5 ownership | pass in the main-owned focused package runs |
| `TestSSHFeatureGateFeedsDependencyAudit` plus existing `TestEnginePlacement` | `scripts/dev/*_test.go` | AC-1 dependency boundary | written/existing; manifest ownership and real-tree audit are separate evidence |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Feature presence | absent or present | `ze_core,ze_ssh` | N-A | N-A |
| User count | 0 to N | 1, smallest configured identity | 0 denies | N-A: no new bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ssh-user-login-yang` | `test/plugin/ssh-user-login-yang.ci` | Tagged password login and wrong-password refusal remain unchanged | existing, must stay green |
| `ssh-pubkey-auth` | `test/plugin/ssh-pubkey-auth.ci` | Tagged YANG public-key augment reaches SSH accept/reject behavior | existing, must stay green |
| `ssh-config-valid` | `test/parse/ssh-config-valid.ci` | Current SSH config syntax still parses | existing, must stay green |
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
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | No | Existing composition is completed |
| 2 | Config syntax changed? | No | Syntax preserved; ownership docs and anchors change |
| 3 | CLI changed? | No | None |
| 4 | API/RPC changed? | No | Dependency-owned |
| 5 | Plugin changed? | No | Existing component gate remains |
| 6 | User guide page? | Yes | Four guide anchors listed above |
| 7 | Wire format changed? | No | None |
| 8 | SDK/protocol changed? | No | None |
| 9 | RFC behavior changed? | N-A | None |
| 10 | Test infrastructure changed? | No | Existing targets used |
| 11 | Daemon comparison affected? | No | No claim changes |
| 12 | Internal architecture changed? | Yes | Core design, YANG design, AAA digest |
| 13 | Route metadata changed? | No | None |
| 14 | Metrics changed? | No | None |
| 15 | Registered inventory changed? | No | Same tagged module set |
| 16 | Changed files have source anchors? | Yes | Update all anchors for both YANG files and extraction flow |
| 17 | Existing examples cover this area? | Yes | Verify user, public-key, and SSH examples in both compositions |

## Implementation Steps

1. **Phase: Wiring** - add failing composition, AAA, and ungated reload tests.
   - Tests: all rows in the Wiring Test table.
   - Verify: failures name missing schema ownership or wiring, not setup.
   - Status: test sources written first; validation intentionally not executed in this phase assignment.
   - Source evidence: authz YANG lacks `authentication`; SSH YANG still owns the base list; `SSHExtractedConfig.Users` and `infraSetup`'s `sshCfg.Users` read remain; the manifest and generated import already gate SSH.
2. **Phase: Schema ownership** - move the base list and add the gated public-key augment.
   - Tests: tagged and untagged parser tests, existing parse functional test.
   - Verify: operator config text is unchanged; bare core accepts only shared fields.
   - Status: focused authz schema, SSH schema, tagged parser, and bare-core parser tests pass in main-owned verification; the existing functional parse suite remains pending.
   - Source evidence: `ze-authz-conf.yang` is the sole base `system.authentication.user` owner; `ze-ssh-conf.yang` imports authz, augments that exact list with `public-keys`, and retains `environment.ssh`.
3. **Phase: Shared extraction and BGP-path AAA** - move `ExtractAuthUsers` into `infra/authz.go`, remove `SSHExtractedConfig.Users`, and resolve `liveUsers` once in `infraSetup`.
   - Tests: config-infra tests and the no-snapshot AAA wiring test.
   - Verify: LSP shows no field reference and dependency tests prove no-BGP AAA.
   - Status: production source implemented; validation intentionally not run in this phase assignment.
   - Source evidence: `ExtractAuthUsers` and its helpers live only in `infra/authz.go`; `SSHExtractedConfig` is transport-only; `infraSetup` resolves the live source once, feeds the successful list to AAA and optional SSH, and withholds local credentials on source failure.
4. **Phase: Reload test ownership** - rename the test file and remove feature tags.
   - Tests: `TestReloadHashesPlaintextPassword` in default and bare-core package runs.
   - Verify: test passes through `diskConfigLoaders` and `doReload` without SSH registration.
5. **Phase: Artifact and functional preservation** - prove binary omission and default SSH behavior.
   - Tests: symbol test and existing functional tests.
   - Verify: bare core omits component symbols; tagged login is unchanged.
6. **Phase: Documentation** - update ownership tables, anchors, and digest.
   - Verify: no anchor attributes shared users to SSH YANG.

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
| Deliverable | Verification method |
|-------------|---------------------|
| Bare-core schema and artifact proof | `make ze-unit-test-cached` (includes the `GO_TEST_CORE` hub pass) |
| Shared config extraction | `make ze-unit-pkg-test PKG=./internal/component/config/infra` |
| YANG ownership | `make ze-unit-pkg-test PKG=./internal/component/authz/yang` and `make ze-unit-pkg-test PKG=./internal/component/ssh/yang` |
| Default SSH workflow | `make ze-functional-plugin-test` and `make ze-functional-parse-test` |
| Generated composition | `make generate`, `make ze-plugin-imports-check`, and `make ze-feature-tags-check` |
| Documentation | `make ze-doc-verify` and `make ze-doc-wiring-check` |
| Go quality | `make ze-lint-changed` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail closed | Missing or malformed users produce no authenticator, never anonymous access |
| SSH-only credentials | Bare core rejects public keys instead of ignoring them |
| Secret handling | Bcrypt, ephemeral, and sensitive annotations move intact; no secret is logged |
| Authorization | Profile lists reach the same authorizer |
| Recovery identity | Zefs precedence and reserved recovery profile remain unchanged |
| Artifact omission | No untagged import links `internal/component/ssh` |

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
- [ ] AC-1 through AC-11 demonstrated
- [ ] Every user story has a named test
- [ ] Wiring table complete
- [ ] Both integration and documentation checklists answered
- [ ] Architectural and critical review complete
- [ ] Every assumption validated during implementation
- [ ] Dependency implemented before field removal
- [ ] No live deferral row

### TDD
- [ ] Tests written first
- [ ] Tests FAIL for intended reasons
- [ ] Tests PASS after implementation
- [ ] Both feature-tag boundaries tested
- [ ] Functional SSH preservation tests pass
- [ ] Interop N-A: no wire behavior change

### Closure
- [ ] Append and complete `plan/TEMPLATE-CLOSURE.md`
- [ ] Independent review gate clean and recorded
- [ ] Learned outcome routed to architecture documentation
- [ ] Commit A contains code, tests, docs, and spec
- [ ] `make ze-precommit-verify`
- [ ] Commit B removes the spec only after closure

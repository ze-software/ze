# Spec: give the looking glass and gNMI a per-user login

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | `plan/spec-login-service-authorisation.md` |
| Phase | - |
| Deferral shard | - |
| Handoff | verify |
| Updated | 2026-08-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two management surfaces authenticate with a shared secret and never learn who
the caller is.

`bearerAuth` (`internal/component/lg/auth.go`) returns the handler unchanged
when the configured token is empty, so a looking glass with no token is fully
public. With a token set, every caller presents the same string. There is no
user, no session and no identity.

`(*Server).checkAuth` (`internal/component/gnmi/server.go`) returns nil when the
configured token is empty, so a gNMI server with no token authenticates nobody
at all. With a token set, every caller presents the same string.

The owner's requirements, given 2026-08-11:

- The looking glass MUST carry an explicit choice between open and requiring a
  user. Today "open" is what an operator gets by leaving a leaf empty, which is
  a default rather than a decision.
- gNMI MUST behave like SSH, which means a per-user login against the same
  credentials every other surface uses.

Once each surface knows who the caller is, it registers its login-service name
and takes part in the login-set gate that
`plan/spec-login-service-authorisation.md` builds. That spec deliberately covers
only the four surfaces that already carry a username; this one supplies the
identity the other two lack.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the rule this change is governed by: both auth paths are guards
  → Constraint: an unset auth mode MUST NOT silently mean "open". A surface that cannot resolve its mode MUST refuse to serve rather than serve unauthenticated.
- [ ] `ai/rules/config.md` - the YANG-versus-env-var decision for the new looking-glass mode leaf
  → Constraint: an `environment/` leaf needs a matching `ze.<service>.<leaf>` registration, as the existing tokens have.
- [ ] `docs/guide/authentication.md` - the operator-facing table of who authenticates where
  → Constraint: both rows are wrong the moment these surfaces gain users.

### RFC Summaries (Scope: protocol)
- N-A for the looking glass: an HTTP bearer credential, unchanged in form.
- gNMI is specified by OpenConfig rather than an RFC. Its transport is gRPC over
  TLS, and per-user credentials travel in call metadata exactly as the shared
  token does today. No wire format changes.

**Key insights:** (minimal context to resume after compaction)
- Both surfaces treat an EMPTY token as "no gate". The looking glass is deliberately public by design; gNMI is not, and its empty-token case is the more surprising of the two.
- `ExtractAuthUsers` (`internal/component/config/infra/ssh.go`) is the one producer of the credential shape, and the live source that makes a deleted user stop authenticating. Both surfaces must consume it rather than build a second list.
- The looking glass YANG (`internal/component/lg/yang/ze-lg-conf.yang`) already carries `enabled`, a `server` list, `tls` and `token` under `environment/looking-glass`. The mode leaf is its sibling.
- `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) refuses an unauthenticated non-loopback management listener. The looking glass is deliberately outside that guard today, and an explicit "open" mode is what lets it stay outside on purpose rather than by omission.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/lg/auth.go` - `bearerAuth` returns `next` unchanged for an empty token; otherwise it compares sha256 digests with `subtle.ConstantTimeCompare`. `bearerTokenMatches` fails closed on a missing header, a wrong scheme and a bare token. No username is parsed anywhere.
- [ ] `internal/component/gnmi/server.go` - `(*Server).checkAuth` returns nil for an empty token, then requires `authorization: Bearer <token>` from call metadata and compares digests. No username, and no `authz.Store` on the path.
- [ ] `internal/component/lg/yang/ze-lg-conf.yang` - `environment/looking-glass` with `enabled`, a `server` list keyed by name, `tls` and `token`.
- [ ] `internal/component/config/infra/ssh.go` - `ExtractAuthUsers`, the one credential producer, and the live source both surfaces must adopt.
- [ ] `internal/component/api/grpc/server.go` - `(*GRPCServer).checkAuth`, the gRPC surface that ALREADY carries a username. It is the model for gNMI, which is the same transport with a different service.
- [ ] `cmd/ze/hub/mgmt_guard.go` - `checkMgmtListeners`, which refuses an unauthenticated non-loopback management listener and which the looking glass currently sits outside.
- [ ] `internal/component/authz/authz.go` - `(*Store).Authorize`, which neither surface reaches today. gNMI gaining a username means gNMI operations become authorisable for the first time.

**Behavior to preserve:**
- The looking glass stays able to be genuinely public, because that is what it is for. The change makes that a stated choice, never a removed capability.
- The shared-token mode keeps working on both surfaces for operators who use it. This spec ADDS a per-user mode; it does not delete the token.
- The constant-time digest comparison stays on both paths. A per-user password check must not reintroduce a timing leak.
- `checkMgmtListeners` keeps its current verdicts for every surface it already judges.

**Behavior to change:**
- The looking glass gains an explicit auth mode: open, token, or per-user. An unset mode is resolved to a stated default rather than to whatever the token leaf happens to be.
- gNMI gains a per-user mode reading the same credentials as SSH, web, REST and gRPC.
- Both register a login-service name and take part in the login-set gate.
- gNMI with neither a token nor users configured stops serving unauthenticated. It refuses, the way every other management surface does.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An HTTP request to a looking-glass route.
- A gRPC call to a gNMI service method.
- A config tree read at boot or at reload, carrying the new mode leaf.

### Transformation Path
1. Config resolves each surface's auth mode: open, token, or per-user.
2. A per-user surface obtains credentials from the live source every other surface uses.
3. A request arrives. The surface resolves an identity from the credential it carries.
4. The surface names itself in the authentication request, so the login-set gate applies.
5. An admitted gNMI caller reaches command authorisation for the first time; an admitted looking-glass caller reaches its read-only routes.
6. An unresolvable mode refuses the listener at boot rather than serving open.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → surface mode | The mode leaf is read where each surface's config is already extracted | Yes: both have an existing extractor for their token |
| Surface → credential source | The live source from `ExtractAuthUsers`, never a second list | Yes: the shape every other surface already consumes |
| Surface → login-set gate | The registered service name, per `plan/spec-login-service-authorisation.md` | Depends on that spec landing first |
| gNMI → authorisation | New. A username reaching `(*Store).Authorize` for gNMI operations | To be established by the design phase |

### Integration Points
- `ExtractAuthUsers` (`internal/component/config/infra/ssh.go`) - the credential source both surfaces adopt.
- `(*GRPCServer).checkAuth` (`internal/component/api/grpc/server.go`) - the working per-user example on the same transport. gNMI copies its shape.
- `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) - gNMI's refusal for an unauthenticated non-loopback listener becomes reachable once "no token" stops meaning "no gate".

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both surfaces adopt the existing credential source rather than reading config themselves |
| No unintended coupling (components stay isolated) | Yes | Neither imports another surface. The shared piece is the credential producer they all already depend on |
| No duplicated functionality (extends existing, does not recreate) | Yes | gNMI copies the gRPC surface's per-user shape on the same transport |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Login path, no wire buffers |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them (`ai/rules/plugins.md`) | Yes | Each surface registers its login-service name; no central file lists them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | gNMI can carry a per-user credential in call metadata the way the gRPC API does | Both are gRPC; `(*GRPCServer).checkAuth` already resolves a username from metadata | gNMI needs a different credential channel and the design changes | Read both metadata paths side by side before implementing | unvalidated |
| A-2 | gNMI clients in the field can present a per-user credential | Per-user is a NEW mode; the token mode is preserved | An operator upgrading finds their collector locked out | AC-6: the default mode preserves today's behaviour for an untouched config | unvalidated |
| A-3 | The looking glass has somewhere to put a session for a per-user mode | It is an HTTP surface, as web is, and web has `SessionStore` | Per-user looking glass needs session machinery it does not have | The design phase decides between reusing the web session store and a per-request credential | unvalidated |
| A-4 | Making gNMI refuse when nothing is configured breaks no existing deployment that was actually secure | An unauthenticated gNMI listener is either loopback-only or already exposed | A daemon that boots today refuses tomorrow | AC-7, and the same reasoning the management-listener guard already applies to other surfaces | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An upgrade locks a monitoring system out of gNMI | The gNMI functional suite fails on a config that worked before | A-2 and AC-6: an untouched config keeps its behaviour, and the per-user mode is opt-in |
| R-2 | The looking glass's public mode is lost, and a public route viewer starts refusing | The looking glass suite fails on a config with no token | AC-1 pins the open mode as a first-class, explicitly selectable state |
| R-3 | A per-user password check on an HTTP path reintroduces a timing leak the token path avoided | A comparison that is not constant-time, or an early return on an unknown username | AC-8: the refusal for an unknown user and for a wrong password must be indistinguishable |
| R-4 | gNMI gains a username but no authorisation, so any admitted user may do anything | An admitted gNMI caller running an operation their profile denies | AC-4: an admitted gNMI user reaches `(*Store).Authorize` like every other surface |
| R-5 | The two surfaces are done separately and only one lands, leaving the login-set gate with a permanent hole | A merged change covering one surface | Both are in this spec on purpose. A one-surface landing is not a partial success |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A monitoring system loses gNMI access, which is visible and quickly diagnosed. The worse direction is quieter: a looking glass that was meant to be public starts refusing, or one that was meant to be gated stays open because an unset mode resolved to open |
| How is it reverted? | Single commit revert. A config carrying the new mode leaf must be edited before an older daemon accepts it |
| Who else touches this path? | `plan/spec-login-service-authorisation.md` builds the gate these surfaces join, and must land first. The closed `spec-hub-deferred-api-auth-independent-of-ssh-block` work is in the same credential area |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A looking-glass request against a server configured for per-user auth | → | `bearerAuth` and its per-user successor (`internal/component/lg/auth.go`) | `test/plugin/lg-per-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A looking-glass request against a server explicitly configured open | → | the resolved mode in the looking-glass config extractor | `test/plugin/lg-open-mode-stays-public.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gNMI call carrying a per-user credential | → | `(*Server).checkAuth` (`internal/component/gnmi/server.go`) | `test/plugin/gnmi-per-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gNMI listener configured with neither token nor users | → | `checkMgmtListeners` (`cmd/ze/hub/mgmt_guard.go`) | `test/plugin/gnmi-unconfigured-refuses.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A looking glass explicitly configured open | Every route serves with no credential, exactly as an untokened looking glass does today. Open is a selectable state, not the absence of a setting |
| AC-2 | A looking glass configured for per-user auth, and a correct user credential | The request is served, and the user is identifiable in the log and in accounting |
| AC-3 | The same looking glass, and a credential for a user whose login set excludes it | Refused, indistinguishably from a wrong password |
| AC-4 | A gNMI call by a correctly authenticated user | The call is served, and the operation passes through `(*Store).Authorize` like every other surface's |
| AC-5 | A gNMI call by a user whose login set excludes gnmi | Refused, indistinguishably from a wrong credential |
| AC-6 | A config carrying a gNMI token and no user configuration, unchanged from before the upgrade | The token keeps working exactly as it does today |
| AC-7 | A gNMI listener with neither a token nor users, on a non-loopback address | The daemon refuses to start and names the remedy. Today it serves unauthenticated |
| AC-8 | An unknown username and a known username with a wrong password, on both surfaces | The same response, the same status, and no timing signal separating them |
| AC-9 | Both surfaces, at startup | Each has registered its login-service name, so a config naming it validates and CLI completion offers it |
| AC-10 | A user deleted from the config, then a reload | That user stops authenticating on both surfaces without a restart, matching every other surface |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs a public route viewer and says so in the config | config → resolved open mode → unauthenticated routes | `test/plugin/lg-open-mode-stays-public.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | puts the looking glass behind named accounts | config → per-user mode → credential source → served request | `test/plugin/lg-per-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | points a gNMI collector at the router with a named account | config → per-user mode → metadata credential → authorised operation | `test/plugin/gnmi-per-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 4 | forgets to configure gNMI auth at all | config → no mode resolvable → listener refused at boot | `test/plugin/gnmi-unconfigured-refuses.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLGOpenModeIsExplicit` | `internal/component/lg/auth_test.go` | AC-1: open is a resolved mode, not an empty token | |
| `TestLGPerUserLoginRefusesUnknownUser` | `internal/component/lg/auth_test.go` | AC-2, AC-8: admitted and refused are both correct, and indistinguishable from each other on failure | |
| `TestGNMIPerUserLogin` | `internal/component/gnmi/server_test.go` | AC-4: a username is resolved from metadata and reaches authorisation | |
| `TestGNMITokenModeUnchanged` | `internal/component/gnmi/server_test.go` | AC-6: the existing token path is untouched | |
| `TestGNMIUnconfiguredDoesNotAuthenticateNobody` | `internal/component/gnmi/server_test.go` | AC-7: an empty configuration stops meaning "no gate" | |
| `TestBothSurfacesRegisterTheirLoginName` | `internal/component/config/loginservice/registry_test.go` | AC-9 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestDeletedUserLosesBothSurfaces` | `cmd/ze/hub/auth_e2e_test.go` | AC-10: the live credential source reaches both | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| looking-glass auth mode | one of open, token, per-user | any of the three, each explicitly selectable | an unset leaf, which MUST resolve to the documented default rather than to "whatever the token leaf is" | an unrecognised mode string, which MUST fail config validation rather than fall through to open |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-open-mode-stays-public` | `test/plugin/lg-open-mode-stays-public.ci` | A deliberately public looking glass serves with no credential | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-per-user-login` | `test/plugin/lg-per-user-login.ci` | A named account is served and a wrong credential is refused, on one running daemon | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-per-user-login` | `test/plugin/gnmi-per-user-login.ci` | A named account drives a gNMI operation, and an excluded account is refused | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-unconfigured-refuses` | `test/plugin/gnmi-unconfigured-refuses.ci` | An unconfigured non-loopback gNMI listener stops the daemon at boot with a named remedy | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

### Interop Tests (Scope: protocol)
gNMI is exercised against a real client in the existing gNMI suite. A per-user
credential MUST be proven with that client rather than with a hand-built
metadata map, because the credential travels in call metadata and a hand-built
map cannot prove a real client can produce it.

## Files to Modify
- `internal/component/lg/auth.go` - the resolved mode, and the per-user path beside the token path
- `internal/component/lg/yang/ze-lg-conf.yang` - the mode leaf beside `token`
- `internal/component/gnmi/server.go` - `(*Server).checkAuth` resolves a username; an unconfigured surface stops authenticating nobody
- the gNMI config extractor and its YANG module - the mode leaf, matching the looking glass's
- `cmd/ze/hub/mgmt_guard.go` - only if gNMI's authenticated verdict is computed there; the guard function itself gains no per-service branch
- `docs/guide/authentication.md` - both rows
- `docs/guide/configuration.md` - the two new mode leaves

## Files to Create
- the four `.ci` files named in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | A mode leaf on each of the two surfaces |
| YANG validation constraints | Yes | The mode is a bounded set; an unrecognised value fails validation rather than falling through |
| YANG custom validators | No | The mode set is fixed and small, unlike the login-service names |
| CLI commands/flags | No | No new verb |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | Yes | Automatic for an enumeration leaf |
| Functional test for new RPC/API | Yes | The four `.ci` files |
| Pipe completeness | No | No new command output |
| Env var registration | Yes | Each `environment/` leaf needs its `ze.<service>.<leaf>` registration, as the tokens have |
| Doctor check for runtime dependencies | Yes | An unconfigured management surface is what a doctor check should name before boot refuses |
| Prometheus counters/metrics | Yes | A refused-login counter per surface, matching what the other four will carry |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: two surfaces gain per-user authentication |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and the environment architecture page |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | Yes | gNMI's authentication contract changes for callers |
| 5 | Plugin added/changed? | No | Both are components |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md` |
| 7 | Wire format changed? | No | The credential travels in existing headers and metadata |
| 8 | Plugin SDK/protocol changed? | No | No SDK type crosses this seam |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | gNMI is an OpenConfig specification, not an RFC |
| 10 | Test infrastructure changed? | No | Existing suites |
| 11 | Affects daemon comparison? | Yes | If `docs/comparison.md` claims gNMI parity, per-user auth is part of that claim |
| 12 | Internal architecture changed? | Yes | gNMI reaches authorisation for the first time |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The refused-login counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Two login-service registrations |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `internal/component/lg/auth.go` and `internal/component/gnmi/server.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every looking-glass and gNMI config example gains a mode |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove neither surface knows a username today
   - Tests: the four `.ci` files, written to fail
   - Files: the four `.ci` files
   - Verify: each fails because no identity exists, not on a setup error
2. **Phase: The looking glass mode** -- open becomes a decision
   - Tests: `TestLGOpenModeIsExplicit`
   - Files: `internal/component/lg/auth.go`, the looking-glass YANG
   - Verify: AC-1. An unset mode resolves to a documented default, and an unrecognised one fails validation
3. **Phase: Looking-glass per-user login**
   - Tests: `TestLGPerUserLoginRefusesUnknownUser`, `lg-per-user-login`
   - Files: `internal/component/lg/auth.go`
   - Verify: AC-2, AC-3, AC-8
4. **Phase: gNMI per-user login** -- copy the gRPC surface on the same transport
   - Tests: `TestGNMIPerUserLogin`, `TestGNMITokenModeUnchanged`
   - Files: `internal/component/gnmi/server.go` and its config extractor
   - Verify: AC-4, AC-5, AC-6
5. **Phase: gNMI stops serving unconfigured**
   - Tests: `TestGNMIUnconfiguredDoesNotAuthenticateNobody`, `gnmi-unconfigured-refuses`
   - Files: `internal/component/gnmi/server.go`, the hub's gNMI verdict
   - Verify: AC-7. The refusal names a remedy
6. **Phase: Join the login-set gate**
   - Tests: `TestBothSurfacesRegisterTheirLoginName`, `TestDeletedUserLosesBothSurfaces`
   - Files: the two registration sites
   - Verify: AC-9, AC-10
7. **Phase: Docs and discrimination**
   - Tests: every `.ci` above, each re-run with the identity change reverted
   - Files: the documentation set, the counters
   - Verify: every new `.ci` goes red without the change

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both surfaces. One is a permanent hole in the login-set gate, not partial progress |
| Feature completeness | Per-user AND the preserved token mode AND the explicit open mode. Dropping any one changes what an operator can express |
| Correctness | An unknown user and a wrong password are indistinguishable to the caller on both surfaces |
| Naming | The mode names what it selects, and "open" says open rather than "none" |
| Data flow | One credential source. Neither surface builds its own user list |
| Rule: `ai/rules/evidence.md` | An unresolvable mode refuses. It never falls through to open |
| Rule: `ai/rules/simplicity.md` | gNMI copies the gRPC surface rather than inventing a second per-user mechanism on the same transport |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Both surfaces resolve an identity | The four `.ci` files pass and fail with the change reverted |
| The public looking glass survives | `lg-open-mode-stays-public` on a config that selects open |
| No second user list | `grep -rn 'ExtractAuthUsers' internal/component/lg internal/component/gnmi` shows the shared source and nothing else parses users |
| Lint | `make ze-lint-changed` |
| Schema | `make ze-doc-verify`, `make ze-cli-grammar-check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | A gNMI listener that authenticates nobody, which is today's behaviour for an unconfigured surface, or a looking glass that resolves to open when the operator asked for a gate |
| What proves it did not | Each `.ci` asserts both halves on one daemon: admitted with the right credential, refused with the wrong one |
| Fail closed | An unresolvable mode refuses to serve. Trace every error return and confirm none yields "open" |
| Empty is not "open" | An unset mode is a distinct state from an explicitly open one. They must not collapse into the same value |
| No user enumeration | AC-8, on both surfaces, including timing |
| Constant time | The existing digest comparison is constant-time. A per-user path must not compare a password with an early-returning helper |
| Privilege | An admitted gNMI user reaches authorisation. Confirm gNMI operations are not exempt from the profile that governs the equivalent CLI command |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A new `.ci` passes with the change reverted | The test is vacuous. Assert the identity decision, not the response code alone |
| gNMI cannot carry a per-user credential in metadata | STOP. That is A-1 broken. Report what the transport allows and ask which way |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The two surfaces look alike and are not. A looking glass with no token is
  public ON PURPOSE, and the owner asked to keep that while making it a stated
  choice. A gNMI server with no token authenticates nobody by ACCIDENT, and that
  is a defect rather than a mode.
- gNMI and the gRPC API share a transport, and the gRPC API already resolves a
  username from call metadata. So the per-user half of this work has a working
  example inside the same process, which is why gNMI is the cheaper of the two
  despite being the more surprising.
- Splitting this from `plan/spec-login-service-authorisation.md` is deliberate.
  That spec checks a fact the daemon already knows; this one teaches it a fact it
  has never known. They fail differently and test differently.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An explicit mode leaf on each surface | Infer the mode from whether a token or users are configured | Inference is what produces today's surprise, where an empty leaf silently disables a gate. The owner asked for an option, which means a stated choice |
| Keep the shared-token mode on both | Replace it with per-user | A token is the right credential for an unattended collector. Removing it would break working deployments to no end |
| gNMI copies the gRPC surface | A gNMI-specific credential scheme | Same transport, same metadata, an existing working example in the same binary |
| Both surfaces in one spec | One spec each | Either alone leaves the login-set gate with a hole, and the hole is invisible from the surface that did land |

## Known Limitations
- MCP keeps its own `Identity` scope model and is not covered here.
- This spec depends on `plan/spec-login-service-authorisation.md` for the gate
  itself. Landing this one first would give two surfaces an identity with
  nothing to check it against.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated on both surfaces, not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features: the gNMI per-user credential proven with a real client

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

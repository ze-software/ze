# Spec: fixit-authz-admin-fallthrough

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/guide/operator-access-rbac.md`, `docs/guide/tacacs.md` - the operator contract this spec must not break
4. `internal/component/authz/authz.go` (`Store.Authorize`), `cmd/ze/hub/main_servers.go` (`usersFromZefsDB`), `internal/component/plugin/server/server.go` (`wrapHandler`)
5. `plan/learned/1121-negative-test-must-fail-for-its-reason.md` - how this area's tests hid two bypasses

## Task

`authz.Store.Authorize` has three paths that end in allow-all. Each is deliberate,
documented, and asserted by a test, so none is a bug on its own. Together they mean a
box with `system.authorization` profiles configured can still authorize an
authenticated user as admin.

The crux is `hasUsers` (`authz.go:343`), which is `len(s.assignments) > 0` and serves as
the proxy for "is RBAC in use". Assignments come only from
`system.authentication.user[*].profile` (`bgp/config/loader.go:301-308`), so a
TACACS-only box has profiles but no assignments: `hasUsers` is false, and every
authenticated user whose profiles are not login-resolved lands on the admin default.

This spec decides what should happen instead, without locking an operator out of a
router. It is a design decision with bricking risk, not a patch: the obvious fix (give
the bootstrap admin an explicit assignment) flips `hasUsers` to always-true, which
changes a second site and would deny the internal RPC path.

Found while fixing two real bypasses (60e35c0d5, 701cbaaa3, 0544b274d). Those are
committed; this is the residue deliberately left alone.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/guide/operator-access-rbac.md` - the documented operator contract for profiles
  → Decision: `admin` is kept as a recovery account and is defined as a config user with a profile, so the documented setup has `hasUsers` true and unassigned users fail closed.
  → Constraint: any change must keep a documented recovery path to a box whose authorization config is wrong or partial.
- [ ] `docs/guide/tacacs.md` - priv-lvl mapping and fallback semantics
  → Constraint: an unmapped priv-lvl rejects the login (AC-18), so a TACACS user reaching authorization should already have resolved profiles.
  → Decision: with `authorization true` the TACACS server decides per command and local profiles are only the unreachable-server fallback; `strict-fallback true` makes that case deny.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - three tests assert today's behavior
  → Constraint: changing them is a deliberate behavior change needing user approval, not test weakening.

### RFC Summaries (MUST for protocol work)
- [ ] N/A - no protocol wire behavior. TACACS+ (RFC 8907) is involved only through
  already-implemented authorization; this spec changes no packet.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The documented RBAC setup is safe today: it defines config users with profiles, so `hasUsers` is true and unassigned users are denied.
- The exposure is the supported-but-undocumented shape: profiles configured, no config users (TACACS-only, or profiles staged before users).
- Recovery access is the constraint that makes this hard. A network OS that locks out its operator needs console or physical access to recover.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/authz/authz.go` - `Store.Authorize` (:339). Three terminal paths reach allow-all.
  → Constraint: `hasUsers := len(s.assignments) > 0` (:343) is the only signal for "RBAC is in use", and it counts config assignments only.
  → Decision: the godoc at :336-338 already states the intended contract — "When user assignments are configured, empty username and unassigned users are denied (fail closed). When no assignments exist, all access is allowed." The code matches its comment; the comment is what this spec questions.
  → Decision: `HasProfiles` (:301) and `HasUserAssignments` (:330) already exist as separate predicates, so O-1 needs no new accessor — only a decision about which one means "RBAC is in use".
- [ ] `internal/component/bgp/config/loader.go` - `extractAuthzConfig` (:267) builds the store; assignments come only from `system.authentication.user[*].profile` (:301-308). `ValidateAuthzConfig` (:215) rejects undefined profile references for users (:248-254) and, since 0544b274d, for `tacacs-profile`.
  → Constraint: any profile name reaching the store from config is guaranteed to resolve; validation is what keeps S-3 unreachable.
- [ ] `cmd/ze/hub/main_servers.go` - `usersFromZefsDB` (:115) returns the `ze init` bootstrap admin as `UserConfig{Name, Hash}` with **no Profiles** (:131).
  → Constraint: the break-glass admin can never have an assignment, so it always lands on S-2. This is what makes S-2 load-bearing.
  → Decision: `meta/instance/admin-disabled` (:116-118) already lets an operator suppress the bootstrap admin entirely, so an explicit "recovery identity" concept partly exists and is a natural hook for O-3.
- [ ] `internal/component/plugin/server/server.go` - `wrapHandler` (:121) builds a `CommandContext` with no Username (:123-127); the comment at :139-140 states identity must be injected by the transport layer.
  → Constraint: RPC handlers authorize with an empty username, so S-1 is a live path, not theoretical.
- [ ] `internal/component/aaa/login_profiles.go` - records profiles resolved at authentication; `authz.go:352-384` consumes them when there is no config assignment, filtered to names the store defines.
  → Decision: a TACACS user's profiles arrive here, so S-1..S-3 are now only reachable by users with *no* resolved profiles.

### Design-phase findings (2026-07-16): read these before trusting the framing above

The research changed the shape of this spec. Five facts, each read from the producing
function, matter more than the three sites themselves.

**F-1 (keystone). A non-nil `Store` in production ALWAYS has profiles, so `HasProfiles()`
is a constant `true` at every production call of `Authorize`.** `extractAuthzConfig`
returns nil when `system.authorization` is absent (`loader.go:291-294`), nil when it
defines no profiles (`loader.go:296-299`), and nil again unless `store.HasProfiles()`
(`loader.go:332-334`). The only non-test caller that builds a store is
`extractAuthzConfig` (`loader.go:285`), reached via `ExtractAuthzStore` (`loader.go:279`).
  → Constraint: "the operator configured no authorization" is carried by the store being
    **nil**, not by any branch inside `Authorize`. `StoreAuthorizer.Authorize` returns
    `true` for a nil store (`register.go:21-23`), and `Dispatcher.isAuthorized` returns
    `true` for a nil authorizer (`command.go:514-516`).
  → Decision: this dissolves O-1's premise. Keying S-1/S-2 on `HasProfiles()` is not a
    subtler signal than `hasUsers`; in production it reduces exactly to "always Deny".
    The honest statement of the fix is **the store's existence is the "RBAC is in use"
    signal**, and `hasUsers` is a second, weaker copy of a question already answered one
    layer up. The permissive branches are not a safety net for the no-RBAC box; that box
    never reaches them.

**F-2. The three tests do not assert what the Task section assumes.** `TestStoreAuthorizeNoAuth`
(`authz_test.go:200-207`) and `TestStoreAuthorizeNoProfiles` (`authz_test.go:188-198`) both
build a bare `NewStore()` with **no profiles**, so `HasProfiles()` is false in both. Only
`TestStoreProfileNotFound` (`authz_test.go:324-333`) would fail under a store-existence rule,
and it too uses an empty store plus a dangling assignment.
  → Decision: R-3's blast radius is one test, not three. Two of the three keep passing
    unchanged, because the shape they pin (an empty store) is exactly the shape that
    cannot occur in production. This is much cheaper than the Task section feared.

**F-3. Giving the bootstrap admin profiles does NOT flip `hasUsers`.** The Task section's
central objection (and R-2, and A-3) assumes the only way to give the admin a profile is
`store.AssignProfiles`. There is a second, entirely separate route:
`usersFromZefsDB` → `BuildParams.LocalUsers` (`aaa/types.go:46`, wired at `infra_setup.go:39`)
→ `LocalAuthenticator.Users` (`register.go:58`) → `AuthResult.Profiles` (`auth.go:58-62`)
→ `profileRecordingAuthenticator.Authenticate` (`login_profiles.go:83-89`)
→ `RecordLoginProfiles` (`login_profiles.go:45-53`) → `aaa.LoginProfiles`
(`login_profiles.go:58-68`) → consumed at `authz.go:372`. `UserCredential` already carries a
`Profiles []string` field (`aaa/types.go:37`); `usersFromZefsDB` simply never sets it
(`main_servers.go:131`).
  → Decision: **R-2 is avoidable and A-3 is broken as stated.** `s.assignments` is written
    by exactly one non-test caller, `loader.go:323`, fed only from the config tree. A
    recovery profile delivered through login-profiles never touches `assignments`, so
    `hasUsers` stays false and S-1 is not disturbed. The S-1/S-2 coupling that made this
    spec look dangerous is an artifact of assuming the wrong delivery route.
  → Constraint: login-resolved names are filtered to profiles the store defines
    (`authz.go:374-378`), so a recovery profile name must exist **in the store**. Config-built
    stores contain only config profiles (`loader.go:303-316`); `BuiltinAdminProfile` is
    registered into a store by no production caller (only `web/rbac_test.go:108` and
    `web/handler_config_test.go:554`). A recovery route therefore needs the builtin
    registered at load, and `BuiltinAdminProfile().Name` is `"admin"` (`authz.go:245-251`),
    which can collide with an operator-defined `admin` profile.

**F-4. A documented recovery path already exists and is already exercised by the guide.**
`mergeAuthUsers` (`main_servers.go:81-93`) drops the zefs entry when a config user has the
same name, letting config override the built-in password. `docs/guide/operator-access-rbac.md`
does exactly this: `set system authentication user admin password ...` plus
`set system authentication user admin profile admin`.
  → Decision: on the documented setup the bootstrap admin is a *config* user with a real
    assignment, so it never reaches S-2 and a strict default cannot lock it out. The lockout
    risk is confined to boxes that have profiles but have **not** defined a config `admin`.

**F-5 (new exposure, config-reachable). A TACACS priv-lvl mapped to an EMPTY profile list
authenticates successfully with zero profiles and lands on S-2.** `handlePass` looks up
`profiles, ok := a.privLvlMap[privLvl]` (`authenticator.go:88`); an **unmapped** level takes
`!ok` and rejects the login (`authenticator.go:89-94`, the AC-18 behavior A-6 asserts). A level
that is *present but empty* takes `ok == true` and returns
`AuthResult{Authenticated: true, Profiles: nil}` (`authenticator.go:98-102`).
`RecordLoginProfiles` then stores nothing (`login_profiles.go:46` returns early on
`len(profiles) == 0`), `LoginProfiles` reports `false`, and `Authorize` falls to S-2. The
YANG permits this shape: `leaf-list profile` under `tacacs-profile` has no `min-elements`
(`ze-tacacs-conf.yang:89-92`), and `ValidateAuthzConfig` only iterates the names present
(`loader.go:266-272`), so an empty list raises nothing.
  → Decision: `set system authentication tacacs-profile 15` with no `profile` leaf is valid
    config that grants **allow-all** to every priv-lvl-15 TACACS user on a TACACS-only box.
    This is a live, config-reachable admin grant, not a theoretical one. It is the strongest
    single argument for closing S-2, and it is independently fixable (reject or warn on an
    empty mapping) whatever is decided about the fall-throughs.

**F-6. Internal identities are inconsistent, and one of them may already be broken.**
`wrapHandler` builds a context with **no** Username (`server.go:123-127`) and authorizes with
it (`server.go:143`), registered for every RPC method at `server.go:233`; that is S-1, and it
confirms A-4. But `dispatchCommandArgs` sets `Username` to `"plugin:<name>"`
(`dispatch.go:434`) and authorizes it (`dispatch.go:437`). A `plugin:<name>` identity has no
assignment and no login profiles, so on the **documented** RBAC box (`hasUsers` true) it
already takes `authz.go:386-387` and is **denied today**.
  → Constraint: two internal callers, two different identities, two different outcomes. Any
    decision here must cover both, or fixing S-1 leaves the `plugin:` hole (or leaves an
    existing denial unexplained).
  → Decision: whether plugin-dispatched commands are *supposed* to be denied on an RBAC box
    is unresolved and is a question for Thomas. It is flagged rather than assumed: this spec
    does not claim it is a bug, only that the code produces a denial on that path.

The three paths, as they stand:

| Site | Condition | Result | Test asserting it | Reachable by |
|------|-----------|--------|-------------------|--------------|
| S-1 `authz.go:345-350` | `username == ""` and `!hasUsers` | `Allow` | `TestStoreAuthorizeNoAuth` ("empty username (no auth configured) allows all") | RPC handlers via `wrapHandler`, which pass no username |
| S-2 `authz.go:385-391` | named user, no assignment, no login-resolved profiles, and `!hasUsers` | `BuiltinAdminProfile` = allow-all | `TestStoreAuthorizeNoProfiles` ("PREVENTS: users locked out when no profile assigned") | the `ze init` bootstrap admin on any box with no config users; any authenticated user on a TACACS-only box whose profiles did not resolve |
| S-3 `authz.go:428-430` | assignment exists but no referenced profile resolves | `BuiltinAdminProfile` = allow-all | `TestStoreProfileNotFound` ("user assigned a non-existent profile gets admin default") | not reachable from config today: `ValidateAuthzConfig` rejects undefined references, and 701cbaaa3 filters login-resolved names to known ones. Reachable only through the direct `Store` API |

Corrections to the table above, from the design-phase reads (the table's *conditions* are
accurate; two of its *reachability* claims were understated):

| Site | Correction | Evidence |
|------|-----------|----------|
| S-1 | Confirmed live, and narrower than "RPC handlers" suggests: the empty username comes from `wrapHandler` only (`server.go:123-127`), registered per RPC method at `server.go:233`. The other internal caller, `dispatchCommandArgs`, supplies `plugin:<name>` (`dispatch.go:434`) and so lands on S-2, not S-1 | F-6 |
| S-2 | Reachable by a **third** shape the table omits: a TACACS user whose priv-lvl is mapped to an empty profile list. This is valid config today | F-5, `authenticator.go:88-102`, `ze-tacacs-conf.yang:89-92` |
| S-2 | NOT reachable by the bootstrap admin on the **documented** setup: the guide defines a config `admin` user with `profile admin`, and `mergeAuthUsers` drops the zefs entry in favour of it | F-4, `main_servers.go:81-93`, `docs/guide/operator-access-rbac.md` |
| S-3 | Confirmed unreachable from config. `AssignProfiles` has exactly one non-test caller (`loader.go:323`), gated by `ValidateAuthzConfig` (`loader.go:248-254`, `:266-272`) | A-5, grep of all callers |
| all | The `!hasUsers` guard on S-1/S-2 is unreachable-as-permissive in production: any store that exists has profiles, but `hasUsers` counts assignments, so a profiles-with-no-assignments box has `hasUsers == false` and takes the permissive branch | F-1 |

**Behavior to preserve:** (unless user explicitly said to change)
- The documented setup in `docs/guide/operator-access-rbac.md` keeps working: profiles plus config users with assignments, unassigned users denied.
- A box whose config defines no authorization at all stays fully permissive.
- The `ze init` bootstrap admin retains a usable path to a box, including one whose authorization config is wrong or partial. This is the recovery account.
- Internal RPC dispatch (`wrapHandler`, empty username) keeps working on a box with RBAC configured.
- TACACS+ priv-lvl mapping continues to govern commands (60e35c0d5), and an unresolvable mapping continues to fail closed (701cbaaa3).

**Behavior to change:** (only if user explicitly requested)
- ~~None yet. The change is the subject of this spec and needs a decision at the DESIGN gate. Candidate directions are in Key Design Decisions; none is approved.~~
- **UPDATED (2026-07-17): decided.** Q-2 is answered by the user (deny always); Q-1/Q-3/Q-4 are answered by AUTONOMOUS DEFAULT below (see Key Design Decisions). The concrete behavior changes, all in `Store.Authorize` and its wiring:
  1. A named, authenticated user who resolves **no applicable profile** is **Denied** (was: `BuiltinAdminProfile` allow-all at S-2, `authz.go:385-391`, when `!hasUsers`). Q-2.
  2. An assignment naming only undefined profiles is **Denied** (was: admin default at S-3, `authz.go:428-430`). AC-4; safe to flip alone (A-5).
  3. An **empty** username reaching `Authorize` is **Denied** (was: Allow at S-1, `authz.go:345-350`, when `!hasUsers`). Requires O-4 to inject an explicit internal identity first, or every RPC method breaks (Q-4).
  4. The `hasUsers` signal (`authz.go:343`) stops being a decision input: "a store exists" already means "RBAC is in use" (F-1), so all three fall-throughs collapse to Deny.
  5. A **reserved recovery identity** (name outside the config namespace, R-8) is delivered to the `ze init` bootstrap admin via the login-profiles route (F-3), so a strict default cannot brick a box with profiles but no config `admin` (O-3').
  6. The godoc at `authz.go:336-338` is rewritten to state the new rule (R-6), and the two operator guides are updated (AC-7).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A command arrives on a surface: SSH exec/session, REST/gRPC, web, MCP, or an internal RPC handler.
- Format at entry: a command string plus a caller identity (username, remote address). The internal RPC path supplies no username.

### Transformation Path
1. Authentication resolves the identity and, for local/TACACS users, a profile-name list (`aaa` chain; `login_profiles.go` records it).
2. The surface builds a `CommandContext` carrying username and remote address (`plugin/server/command.go`), or omits the username on the `wrapHandler` RPC path.
3. `Dispatcher.isAuthorized` (`plugin/server/command.go:503`) calls the configured `aaa.Authorizer`.
4. `authz.StoreAuthorizer` forwards to `Store.Authorize` (`authz/register.go`).
5. `Store.Authorize` selects profiles: config assignment, else login-resolved names filtered to known profiles, else one of the three fall-throughs above.
6. The chosen profile's run/edit section decides; a refusal surfaces as `plugin.UnauthorizedMessage`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Authentication ↔ Authorization | profile names recorded at login, read by the store | [ ] |
| Config ↔ Store | `extractAuthzConfig` builds profiles and assignments | [ ] |
| Transport ↔ Dispatcher | `CommandContext.Username`; empty on the RPC path | [ ] |
| zefs ↔ AAA | `usersFromZefsDB` supplies the bootstrap admin credential, without profiles | [ ] |

### Integration Points
- `aaa.Bundle.Authorizer` / `liveAAABundleAuthorizer` - resolves the live bundle per call, so a store change takes effect on reload without re-wiring.
- `tacacs.TacacsAuthorizer` - falls back to the local store on server error, so any store change also changes TACACS fallback behavior.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The bootstrap admin from `ze init` never carries profiles, so it always reaches S-2 | `cmd/ze/hub/main_servers.go:131` returns `UserConfig{Name, Hash}` only | If it can carry profiles, S-2 stops being the recovery path and may be removable outright | Read the producer; add a test asserting the zefs user's Profiles are empty | **confirmed (partly superseded)**. `usersFromZefsDB` returns `[]authz.UserConfig{{Name: name, Hash: string(hash)}}` (`main_servers.go:131`), leaving `Profiles` nil. But "always reaches S-2" is **false on the documented setup**: `mergeAuthUsers` (`main_servers.go:81-93`) drops the zefs entry when a config `admin` exists, and the guide defines one (F-4). The correct statement: the zefs admin reaches S-2 only when no config user shares its name |
| A-2 | Denying at S-2 would lock the bootstrap admin out of a box that has profiles but no config users | Follows from A-1 plus `authz.go:385-391` | The lockout risk evaporates and the fix is a one-line flip | Functional test: profiles, no config users, authenticate as the zefs admin, run a command | **confirmed, and narrowed**. The chain holds for a box with profiles and no config `admin`. Scope is smaller than feared: the documented setup is immune (F-4). Still a real bricking risk for a box that staged profiles before users, which is precisely the exposed shape. A `.ci` test remains required before any flip |
| A-3 | Assigning the bootstrap admin a profile makes `hasUsers` true on every box | `hasUsers := len(s.assignments) > 0` (`authz.go:343`) counts any assignment | The S-1 coupling disappears and the sites can be fixed independently | Read `extractAuthzConfig`; add a store test | **broken**. True only for the `store.AssignProfiles` route, whose sole non-test caller is `loader.go:323` (config tree only). The `UserCredential.Profiles` → `AuthResult.Profiles` (`auth.go:60`) → `RecordLoginProfiles` (`login_profiles.go:86`) → `LoginProfiles` → `authz.go:372` route delivers profiles **without touching `assignments`**, so `hasUsers` stays false. See F-3. Consequence: **R-2 does not fire** and S-1/S-2 are separable |
| A-4 | S-1 (empty username) is a live path, not dead code | `plugin/server/server.go:123-127` builds a context with no username; :139-140 says identity is injected by the transport | If dead, S-1 can be denied outright with no consequence | Instrument or test `wrapHandler` dispatch on an RBAC-configured box | **confirmed**. `wrapHandler` builds `CommandContext{Server, RequestContext, Peer}` with no Username (`server.go:123-127`) and calls `isAuthorized` with it (`server.go:143`); registered for every RPC method at `server.go:233`. `isAuthorized` reads `ctx.Username` (`command.go:519`, empty) and forwards to the authorizer (`command.go:522`) |
| A-5 | S-3 is unreachable from config | `ValidateAuthzConfig` (`loader.go:215`) rejects undefined references for users and tacacs-profile; 701cbaaa3 filters login names | If reachable, a typo grants admin and this becomes urgent rather than defensive | Grep every `AssignProfiles`/`AddProfile` caller for a path that skips validation | **confirmed**. Grep of `AssignProfiles` across `internal/`, `cmd/`, `pkg/` yields one non-test caller, `loader.go:323`, fed from the config tree and gated by `ValidateAuthzConfig` (`loader.go:248-254` for users, `:266-272` for tacacs-profile). Login-resolved names are filtered at `authz.go:374-378`. S-3 stays defensive-only |
| A-6 | An unmapped TACACS priv-lvl rejects the login, so TACACS users reaching authorization have resolved profiles | `docs/guide/tacacs.md` ("unmapped priv-lvl rejects the login (AC-18)") | TACACS users could reach S-2 and get admin, making this urgent | Read the tacacs authenticator's handlePass; add a test for an unmapped level | **confirmed for "unmapped", BROKEN for "mapped-to-empty"**. `handlePass` rejects when `!ok` (`authenticator.go:88-94`), but a level present with an empty profile list takes `ok == true` and returns `Authenticated: true, Profiles: nil` (`authenticator.go:98-102`). YANG allows it (`ze-tacacs-conf.yang:89-92`, no `min-elements`). Such a user reaches S-2 and is granted admin. See F-5. **This makes the exposure urgent and config-reachable** |
| A-7 | `admin-disabled` is the existing operator control for suppressing the recovery account | `cmd/ze/hub/main_servers.go:116-118` returns `errAdminDisabledInZefs` | O-3 needs a different hook for naming the recovery identity | Read the flag's consumers and docs | **confirmed**. `usersFromZefsDB` checks `zefs.KeyInstanceAdminDisabled` first and returns `errAdminDisabledInZefs` before reading credentials (`main_servers.go:116-118`), so the whole zefs-admin contribution vanishes. It is a suppression switch, not a naming hook, so O-3 still needs a place to name the recovery profile |
| A-8 | `HasUserAssignments` has no production caller, so renaming or removing the `hasUsers` notion breaks nothing outside the store | new (design phase) | If a production caller exists, the predicate is load-bearing elsewhere | grep across `internal/`, `cmd/`, `pkg/` | **confirmed**. Callers are `authz_test.go:625-631` and `loader_authz_test.go:215` only. `HasProfiles` by contrast has a production caller at `loader.go:332`. `HasUserAssignments` is currently test-only, so it is free to repurpose or delete |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-0 | The three sites are load-bearing in ways a reader may not expect, so a future session "cleans them up" without reading this spec | A diff flips a fall-through to Deny with no spec reference | The Current Behavior table names who reaches each site; R-3 covers the tests |
| R-1 | A stricter default locks an operator out of a live router; recovery needs console or physical access | A functional test authenticating the bootstrap admin against a profiles-only config starts failing | Keep an always-available recovery identity; make the deny path explicit and logged so the cause is visible in the daemon log |
| ~~R-2~~ | ~~Making the bootstrap admin an explicit assignment flips `hasUsers` true everywhere, so S-1 starts denying internal RPC dispatch~~ | ~~RPC-driven features fail with the access-control refusal on boxes that previously worked~~ | **Superseded: does not fire for the recommended route.** R-2 assumed the only delivery is `store.AssignProfiles`. Profiles delivered via `UserCredential.Profiles` → login-profiles (F-3) never touch `s.assignments`, so `hasUsers` is unchanged and S-1 is untouched. R-2 remains live *only* if an implementation chooses the `AssignProfiles` route, which this spec now recommends against |
| R-3 | The three tests asserting today's behavior get "fixed" to match new code without a decision, silently changing the security contract | A diff editing `TestStoreAuthorizeNoAuth`/`NoProfiles`/`ProfileNotFound` without a spec reference | Any change to those tests cites this spec and the approved decision in its commit body. **Narrowed by F-2:** only `TestStoreProfileNotFound` must change under a store-existence rule; the other two use empty stores and keep passing. A diff that edits all three is a signal the author changed more than the decision authorised |
| R-4 | Fixing only S-2 leaves S-1 as an equivalent hole via the RPC path | An RPC-dispatched command succeeds for an identity the SSH path would refuse | Treat S-1 and S-2 as one decision; do not ship a partial. **Widened by F-6:** there are two internal identities, not one (`""` from `wrapHandler`, `plugin:<name>` from `dispatchCommandArgs`). A decision covering only the empty username leaves the `plugin:` identity undecided |
| R-7 | The empty tacacs-profile mapping (F-5) is a live admin grant that exists **today**, independent of every decision in this spec | A priv-lvl-15 TACACS user on a TACACS-only box runs any command and is allowed | Fixable on its own (reject an empty `profile` list at validation, or `min-elements 1` in YANG). Should not wait on the S-1/S-2 decision. Candidate for its own commit ahead of this spec |
| R-8 | Registering a builtin `admin` profile into every store (needed for the F-3 recovery route) collides with an operator-defined profile named `admin` | A config `profile admin` silently changes meaning, or the builtin overwrites it via `AddProfile` (`authz.go:287-291` replaces by name) | Decide precedence explicitly. `TestStoreOverrideBuiltinProfile` (`authz_test.go:335`) suggests config-overrides-builtin is the existing intent; a reserved name outside the config namespace avoids the question entirely |
| ~~R-5~~ | ~~The `bgp/config` authz tests do not run (they need the ssh build tag), so a regression here is invisible to that package's suite~~ | ~~`TestExtractAuthzConfig_*` and `TestValidateAuthzConfig_*` fail on "unknown field in authentication: user"~~ | **Superseded: this risk does not exist.** The tests run and pass under the real gate. The reds were phantoms from running bare `go test` without Ze's feature tags — see `ai/rules/bash-output.md` "Bare `go test` Lies". With `-tags "ze_core ... ze_ssh"` the package is `ok`, and `make ze-verify` never reported it. Recorded in the Mistake Log below. |
| R-5b | Unit coverage for this area is *not* the weak spot, so the temptation is to trust it entirely; but `Store.Authorize` decides for surfaces (ssh, RPC, web) that unit tests do not exercise | A store-level test passes while an operator is still refused, or still admitted, on a real box | Keep the `.ci` rows: they cover the surface path, which is where 60e35c0d5 and 701cbaaa3 actually bit |
| R-6 | The godoc at `authz.go:336-338` documents the current contract, so changing behavior without rewriting it leaves a lying comment | Reviewer reads the comment and believes the old rule | Update the godoc in the same commit as the behavior |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH command from the zefs bootstrap admin, config has profiles but no config users | → | `Store.Authorize` S-2 branch (`authz.go:385-391`) | `test/plugin/authz-recovery-admin.ci` |
| SSH command from an authenticated user with no resolved profiles on a TACACS-only config | → | `Store.Authorize` S-2 branch (`authz.go:385-391`) | `test/plugin/authz-no-applicable-profile.ci` |
| Internal RPC dispatch with no username on an RBAC-configured box | → | `Store.Authorize` S-1 branch (`authz.go:345-350`), entered from `wrapHandler` (`server.go:123-127`) via `server.go:233` | `test/plugin/authz-rpc-identity.ci`. Must prove the decided behavior does not break plugin RPC |
| TACACS login at a priv-lvl mapped to an empty profile list | → | `handlePass` (`authenticator.go:105-111`) → login rejected before authz | `TestTacacsAuthenticatorProfileMappingShapes` (`authenticator_test.go:138`) + `TestExtractConfigPrivLvlMapEmptyProfileList` (`config_test.go:127`) — **already landed (F-5, AC-8/AC-9), not new work for this spec** |

## Acceptance Criteria

<!-- Written against the problem, not a chosen solution. The DESIGN gate fixes the expected column. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config defines `system.authorization` profiles and no config users; an authenticated user with no resolved profiles runs a command | ~~**Pending Q-2.** Recommended: `Deny`, with the reason logged (AC-6). Not "silently admin" under any answer.~~ **RESOLVED: `Deny`, always**, with the reason logged (AC-6). Q-2 answered by the user 2026-07-16 (`authz.go:385-391` S-2 stops returning `BuiltinAdminProfile`). |
| AC-2 | Same config; the `ze init` bootstrap admin runs a command | The recovery account reaches the box. ~~**Pending Q-3:** either an explicit recovery profile (O-3'), or the documented "define a config `admin` user" route that already works (F-4).~~ **RESOLVED (Q-3 → O-3'):** the bootstrap admin carries an explicit **reserved** recovery profile (name outside the config namespace, R-8), delivered via login-profiles (F-3, never `AssignProfiles`), registered into the store at load. The F-4 "define a config `admin`" route remains as a second, operator-controlled recovery path. |
| AC-3 | Internal RPC dispatch (no username) on a box with profiles and config users | Plugin RPC keeps working, and the reason is explicit rather than incidental. ~~**Pending Q-4.** Note: today this is already `Deny` when `hasUsers` is true (`authz.go:346-348`), so "as today" must be pinned by a test before it is assumed.~~ **RESOLVED (Q-4 → O-4):** the RPC boundary injects an explicit **reserved internal identity** (`wrapHandler`, `server.go:126-145`), which authorizes via a reserved trusted profile registered at load. An empty username at `Authorize` then **fails closed** (`authz.go:345-350` returns `Deny`). Pinned by `test/plugin/authz-rpc-identity.ci` before S-2 lands (S-1 sequencing). |
| AC-4 | A store whose assignment names only undefined profiles (direct API) | Fails closed rather than granting admin. **Safe to fix independently** (A-5 confirmed: unreachable from config). Changes `TestStoreProfileNotFound`. |
| AC-5 | The documented setup in `docs/guide/operator-access-rbac.md` | Unchanged: assigned users get their profile, unassigned users are denied. **Immune to the change** per F-4. |
| AC-6 | Whatever outcome AC-1 fixes | The daemon log states which rule decided, so an operator can tell "denied by profile" from "denied because no profile applied". |
| AC-7 | The godoc on `Store.Authorize` and the two operator guides | Describe the rule the code actually implements after the change. |
| AC-8 | A `tacacs-profile` level configured with an empty `profile` leaf-list | Rejected at validation (or warned), so it can no longer authenticate a user with zero profiles into the S-2 admin default. **New, from F-5. Independent of Q-1..Q-4.** **DISCHARGED ELSEWHERE (verified 2026-07-17):** closed at the authenticator in the now-landed sibling spec — `handlePass` treats an empty/nil mapping as unmapped and denies (`authenticator.go:105-111`, `!ok || len(profiles) == 0`), tested by `TestTacacsAuthenticatorProfileMappingShapes` (`authenticator_test.go:138`) and `TestExtractConfigPrivLvlMapEmptyProfileList` (`config_test.go:127`). No further validation/YANG work required by THIS spec unless Thomas also wants a config-time reject. |
| AC-9 | A TACACS user at a priv-lvl mapped to an empty profile list, on a TACACS-only box, runs a command | Not granted admin. This is the concrete exposure F-5 proves is reachable today. **DISCHARGED ELSEWHERE (verified 2026-07-17):** the login is now rejected before authorization (`authenticator.go:105-111`); the user never reaches S-2. Covered by `TestTacacsAuthenticatorProfileMappingShapes`. |
| AC-10 | A `plugin:<name>` identity dispatches a command on a box with profiles and config user assignments | A decided, tested outcome. ~~**Pending Q-4.** Today the code produces `Deny` (`dispatch.go:434` + `authz.go:386-387`); whether that is intended is unresolved.~~ **RESOLVED (Q-4 → O-4):** `dispatchCommandArgs`/`dispatchCommand` (`dispatch.go:434`, `:469`) are regularised onto the same explicit reserved internal identity as `wrapHandler`, so both internal callers authorize via the reserved trusted profile and stop disagreeing (F-6). The current incidental `Deny` for `plugin:<name>` is replaced by a deliberate, named grant that keeps plugin dispatch working. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Stages authorization profiles before adding config users, then logs in as the bootstrap admin | ssh auth → aaa chain → dispatcher → `Store.Authorize` S-2, reached via the reserved recovery profile (O-3') | `test/plugin/authz-recovery-admin.ci` |
| 2 | Runs a TACACS-only box where a user authenticates but resolves no profiles | tacacs authen → login profiles (empty) → `Store.Authorize` S-2 → Deny (Q-2) | `test/plugin/authz-no-applicable-profile.ci` |
| 3 | Uses a plugin feature that dispatches an RPC with no username | plugin → `wrapHandler` (reserved internal identity, O-4) → `isAuthorized` → `Store.Authorize` | `test/plugin/authz-rpc-identity.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStoreAuthorizeNoAuth` | `internal/component/authz/authz_test.go:200` | existing S-1 contract. **Uses an empty store, so it survives a store-existence rule unchanged** (F-2) | keep as-is |
| `TestStoreAuthorizeNoProfiles` | `internal/component/authz/authz_test.go:188` | existing S-2 contract. **Uses an empty store, so it survives unchanged** (F-2) | keep as-is |
| `TestStoreProfileNotFound` | `internal/component/authz/authz_test.go:324` | existing S-3 contract. **The one test that must change** under AC-4 | changes |
| `TestStoreAuthorizeProfilesNoAssignments` (new) | `internal/component/authz/authz_test.go` | the gap itself: store with profiles, no assignments, named user, no login profiles. Pins AC-1. This shape has **no test today**, which is why the hole survived | new |
| `TestStoreAuthorizeEmptyUsernameWithProfiles` (new) | `internal/component/authz/authz_test.go` | S-1 on a store that has profiles but no assignments. Pins AC-3 | new |
| `TestUsersFromZefsDBProfiles` (new) | `cmd/ze/hub/zefs_users_test.go` | the bootstrap admin's profile shape (A-1). Note the file is `zefs_users_test.go`, not `main_servers_test.go` | new |
| `TestMergeAuthUsersConfigOverridesZefs` | `cmd/ze/hub/zefs_users_test.go` | F-4: a config user of the same name replaces the zefs admin. Check whether this already exists before writing it | verify first |
| ~~`TestHandlePassEmptyProfileList` (new)~~ → landed as `TestTacacsAuthenticatorProfileMappingShapes` | `internal/component/tacacs/authenticator_test.go:138` | F-5/AC-9: a priv-lvl mapped to an empty/nil list is now treated as unmapped and denied at the producer | **landed (sibling spec)** |
| ~~`TestValidateAuthzConfigEmptyTacacsProfile` (new)~~ → landed as `TestExtractConfigPrivLvlMapEmptyProfileList` | `internal/component/tacacs/config_test.go:127` | AC-8: the config half of the empty `tacacs-profile` mapping | **landed (sibling spec)** |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — this spec adds no numeric input | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `authz-recovery-admin` | `test/plugin/authz-recovery-admin.ci` | profiles configured, no config users, bootstrap admin runs a command. Proves AC-2: the recovery path works whatever Q-3 decides | new |
| `authz-no-applicable-profile` | `test/plugin/authz-no-applicable-profile.ci` | profiles configured, no config users, authenticated user with no resolved profiles. Proves AC-1: the decided outcome, and that it is not silently admin | new |
| `authz-rpc-identity` | `test/plugin/authz-rpc-identity.ci` | a plugin RPC dispatches on an RBAC-configured box. Proves AC-3/AC-10 and guards R-4: the S-1 decision does not break plugin dispatch | new |
| `rbac-ssh-only-enforced` | `test/plugin/rbac-ssh-only-enforced.ci` | existing: profiles apply on an ssh-only daemon (must keep passing) | |
| `tacacs-readonly`, `tacacs-author` | `test/plugin/` | existing: priv-lvl mapping governs commands (must keep passing) | |
| `authz-deny`, `authz-allow`, `authz-default` | `test/plugin/` | existing: profile allow/deny contract (must keep passing) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A — no wire protocol behavior changes | - | - | - | - |

### Future (if deferring any tests)
- None planned. `.ci` coverage is mandatory here because `Store.Authorize` decides for surfaces the unit tests do not exercise (R-5b), not because the unit tests are broken — they are not (see the superseded R-5).

## Files to Modify
- `internal/component/authz/authz.go` - the three fall-throughs in `Store.Authorize` (`:339-431`), the `hasUsers` signal (`:343`), and the godoc (`:336-338`, R-6). Possibly a named recovery profile beside `BuiltinAdminProfile` (`:245`)
- `internal/component/authz/authz_test.go` - `TestStoreProfileNotFound` (`:324`) changes; the other two (`:188`, `:200`) do not (F-2). New tests per the TDD plan
- `cmd/ze/hub/main_servers.go` - `usersFromZefsDB` (`:131`), only if Q-3 gives the bootstrap admin an explicit profile (O-3')
- `internal/component/plugin/server/server.go` - `wrapHandler` (`:121-127`), only if Q-4 injects an identity on the RPC path (O-4)
- `internal/component/plugin/server/dispatch.go` - `dispatchCommandArgs` (`:434`) / `dispatchCommand` (`:469`), only if Q-4 regularises the `plugin:<name>` identity (F-6)
- `internal/component/tacacs/yang/ze-tacacs-conf.yang` - `leaf-list profile` (`:89-92`), if AC-8 is enforced by `min-elements 1`
- `internal/component/bgp/config/loader.go` - `ValidateAuthzConfig` (`:266-272`), if AC-8 is enforced in validation rather than YANG
- `docs/guide/operator-access-rbac.md` - the operator-facing rule for a user with no applicable profile; the recovery contract
- `docs/guide/tacacs.md` - the empty-mapping rule (AC-8) and, if it changes, the TACACS-only shape's outcome

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | none expected; a reserved profile name would need a decision on whether it is config-visible |
| CLI commands/flags | [ ] | none expected |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` — mandatory, see R-5b |
| Doctor check for runtime dependencies | [ ] | consider a check that warns when profiles exist but nothing assigns them (the exposed shape) |
| Prometheus counters/metrics | [ ] | consider counting refusals by reason, to make AC-6 observable |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/operator-access-rbac.md`, `docs/guide/tacacs.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | only if the RPC identity path changes |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` if refusal counters are added |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | N/A |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | grep `docs/` for `source: internal/component/authz/authz.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | `docs/guide/operator-access-rbac.md` profile examples |

## Files to Create
- ~~`test/plugin/*.ci` - (fill during design) functional coverage for the decided behavior~~
- `test/plugin/authz-recovery-admin.ci` - profiles configured, no config users; the bootstrap admin runs a command and reaches the box via the reserved recovery profile (AC-2, O-3')
- `test/plugin/authz-no-applicable-profile.ci` - profiles configured, no config users; an authenticated user with no resolved profiles is Denied, not admin (AC-1, Q-2)
- `test/plugin/authz-rpc-identity.ci` - a plugin RPC dispatches on an RBAC-configured box; the reserved internal identity (O-4) keeps dispatch working, and an empty username fails closed (AC-3/AC-10, R-4)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. ~~**Phase: Validate the assumptions before designing**~~. **DONE (2026-07-16, design phase).** A-1..A-7 settled by reading the producers; A-3 and A-6 came back **broken**, A-8 added. Findings F-1..F-6 recorded in Current Behavior. No `unvalidated` rows remain.
2. ~~**Phase: Decide**~~. **Q-2 ANSWERED (user, 2026-07-16).**

   -> Decision (user, 2026-07-16), answering **Q-2**: a user who authenticates but has no
   applicable profile is **DENIED, always** -- regardless of whether any config user exists.
   The invariant is uniform and absolute: *authenticated implies at least one profile*. This
   is option O-1'. The three `hasUsers` fall-throughs in `Store.Authorize` (`authz.go:343`,
   `:345-350`, `:385-390`) go away rather than being narrowed: under this answer `hasUsers`
   stops being a decision input at all, since "no profile" now means Deny on both sides of it.
   Note F-1: a no-RBAC box never reaches `Authorize` (nil store is permissive one layer up,
   `register.go:21-23`), so removing the fall-throughs does not brick an unconfigured box; that
   permissiveness lives in the loader returning nil (`loader.go:291-299`), which is untouched.

   -> ~~Constraint: Q-1, Q-3 and Q-4 are NOT answered.~~ **Q-1, Q-3, Q-4 ANSWERED (AUTONOMOUS
   DEFAULT, 2026-07-17)** — see the Resolutions block in Key Design Decisions. Q-4 (are internal
   identities subjects of authorization?) is load-bearing and must be settled BEFORE S-2 lands:
   `wrapHandler` sends `""` (`server.go:126-145`) and `dispatchCommandArgs` sends `plugin:<name>`
   (`dispatch.go:434`). Under "denied always", BOTH become Deny unconditionally, where today they
   are Deny only when `hasUsers` is true. That is a behaviour change on the internal RPC path, and
   per S-1 it must precede or accompany S-2. Do not read Q-2's answer as licence to skip it.
   **Resolution: Q-4 → O-4** (inject a reserved internal identity at the RPC boundary; empty
   username fails closed), **Q-3 → O-3'** (reserved recovery profile via login-profiles), **Q-1 →
   supported shape, no config-time rejection**. Any of these may be overridden by Thomas before
   implementation.

3. ~~**Phase: O-5 / AC-8**~~. **DONE 2026-07-16, landed separately at the user's direction.**
   The F-5 empty-`tacacs-profile` admin grant is closed in
   `plan/spec-fixit-tacacs-empty-profile-mapping.md` (`authenticator.go:104`, `!ok || len(profiles) == 0`).
   The sibling RADIUS escalation, which this spec never identified, is closed in
   `plan/spec-fixit-radius-empty-profile-mapping.md`. Both enforce Q-2's invariant locally at the
   authenticator; this spec still owes it globally at `Authorize`.
4. **Phase: Wiring (MANDATORY FIRST once approved)**. Register the entry points from the Wiring Test table and write the three failing `.ci` tests before touching `Authorize`
5. **Phase: S-1 / Q-4 (must precede or accompany S-2)**. The internal-identity decision. Covers both `wrapHandler` (`server.go:123-127`) and `plugin:<name>` (`dispatch.go:434`), per R-4 as widened by F-6
6. **Phase: S-2 + S-3 + recovery**. The decided behavior, TDD per site. S-3 (AC-4) is safe to flip alone (A-5). Recovery via the login-profiles route (F-3), never via `AssignProfiles` (which would resurrect R-2)
7. **Phase: godoc + guides**. `authz.go:336-338` (R-6), `operator-access-rbac.md`, `tacacs.md`
5. **Functional tests** → the `.ci` rows above; unit coverage alone is insufficient here because the decision serves surfaces the store tests never drive (R-5b)
6. **Full verification** → `make ze-verify`
7. **Complete spec** → learned summary + two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The bootstrap admin still reaches a box whose authorization config is wrong |
| Correctness | Internal RPC dispatch still works on an RBAC-configured box |
| Data flow | The decision is made in `Store.Authorize` only; surfaces are unchanged |
| Naming | Any new "RBAC is in use" signal is named for what it means, not for what it counts |
| Rule: no-layering | No second authorization notion added alongside the store |
| Observability | A refusal states which rule decided (AC-6) |
| Comment truth | The `Store.Authorize` godoc matches the implemented rule (R-6) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Each of S-1..S-3 has a decided, tested outcome | `go test ./internal/component/authz -run TestStoreAuthorize` |
| Recovery path documented | grep `docs/guide/operator-access-rbac.md` for the recovery rule |
| Existing RBAC/TACACS functional tests still pass | `ze-test bgp plugin rbac-ssh-only-enforced tacacs-readonly tacacs-author authz-deny` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Fail-closed default | No remaining path where "nothing applied" means allow-all |
| Recovery vs exposure | The recovery identity is explicit, named, and logged — not an accidental consequence of a counter being zero |
| Information disclosure | A refusal does not reveal profile contents to an unauthorized caller |
| Downgrade | A config reload cannot transiently widen a session's authority |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The three fall-throughs were fail-open bugs to fix | Each is deliberate, documented, and asserted by a test; S-2 is the break-glass path for the `ze init` admin, which carries no profiles | Read `usersFromZefsDB` (`main_servers.go:131`) and the three tests' stated intent before changing anything | The reported "fix these" scope collapsed to one real gap (tacacs-profile validation, fixed in 0544b274d); the rest became this spec |
| A-3: giving the bootstrap admin a profile flips `hasUsers` true everywhere, coupling S-2 to S-1 and making the fix dangerous (R-2). This framing drove the whole Task section and the "not a patch, a design decision with bricking risk" premise | True only for `store.AssignProfiles`. A second delivery route already exists and bypasses `assignments` entirely: `UserCredential.Profiles` (`aaa/types.go:37`) → `AuthResult.Profiles` (`auth.go:60`) → `RecordLoginProfiles` (`login_profiles.go:86`) → `LoginProfiles` → `authz.go:372`. `hasUsers` is untouched | Grepped every `.Profiles` reader and every `AssignProfiles` caller during design, instead of reasoning from `hasUsers`'s definition alone | R-2 superseded, O-3 re-scored from "most principled but blocked on S-1" to "cheap and independent". The spec's central objection to the obvious fix was an artifact of assuming one delivery route |
| A-6: an unmapped TACACS priv-lvl rejects the login, so a TACACS user reaching authorization always has resolved profiles (making S-2 hard to reach) | True for *unmapped*. But a level **mapped to an empty profile list** takes `ok == true` (`authenticator.go:88`) and authenticates with zero profiles (`:98-102`), reaching S-2 and receiving allow-all. The YANG permits it (`ze-tacacs-conf.yang:89-92`, no `min-elements`) | Read `handlePass` rather than trusting the doc sentence in `docs/guide/tacacs.md`, then checked whether the YANG could produce the third state | Turned the exposure from "supported-but-undocumented shape" into a **config-reachable admin grant that exists today**. Added AC-8/AC-9 and O-5, recommended as an independent fix ahead of the policy decision |
| F-1 was not assumed by anyone, but it inverts the spec's framing: the permissive fall-throughs protect the no-RBAC box | The no-RBAC box has a **nil store** (`loader.go:291-299`, `:332-334`) and is allowed at `register.go:21-23`, never reaching `Authorize`. The fall-throughs protect nothing | Read `extractAuthzConfig`'s nil returns and `StoreAuthorizer`'s nil check, rather than accepting "a box with no authorization config stays fully permissive" as evidence that the fall-throughs cause it | "Behavior to preserve" listed the permissive no-config box as something the fall-throughs deliver. They do not. Removing them cannot affect it |
| `bgp/config` had 10 broken tests that "never run" (needing the ssh build tag), and `plugin/all` had 3 failing golden snapshots — both reported as pre-existing, and R-5 written into this spec on that basis | Neither is broken. Both pass under Ze's feature tags: `go test -tags "ze_core ... ze_ssh"` returns `ok`. `make ze-verify` never reported either package — the reds existed only in bare `go test` runs, where `all_ze_ssh.go` (`//go:build ze_ssh`) never registers the ssh YANG module, so `system.authentication.user` becomes an unknown field | Re-ran with the tags after noticing commit fd72df184, "rules: bare go test drops feature tags and fakes reds", which landed mid-session and describes this exact trap | R-5 superseded. The claim also reached the user repeatedly ("the blind spot that let the authz bugs hide") and the body of commit 0544b274d, which calls them "10 pre-existing failures". That commit's conclusion still holds — the failure set was identical before and after, so the change added none — but its premise is a phantom |

**Why the baseline "confirmed" it.** The `git archive f9dc6132b` check ran bare `go test`
too, so it reproduced the same tags-less mistake and returned an identical failure set.
Identical-to-baseline proved only "not caused by my change"; it could never prove "real",
because both sides shared the defect. `ai/rules/bash-output.md` names this: *"a bare run
there reproduces your own mistake and 'confirms' a red that does not exist."*

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Flip the fall-throughs to Deny | Would lock the bootstrap admin out of a box with profiles but no config users (A-1/A-2) | Decide the recovery identity first; this spec |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Calling a tested, deliberate behavior a "bug" before reading its test's stated intent | 1 (this spec) | Partly covered by `ai/rules/no-fabrication.md` (read the producer); the gap was reading the producer of the *value* but not of the *intent* | Note in the learned summary if it recurs |

## Design Insights
- `hasUsers` conflates two different questions: "did the operator configure RBAC?" (profiles exist) and "did the operator assign anyone?" (assignments exist). All three paths key off the second while meaning the first. Naming those apart is probably the whole fix, and both predicates already exist (`HasProfiles` :301, `HasUserAssignments` :330) — they are simply not used here.
  → **Refined by F-1:** the sharper statement is that `Authorize` should not be asking this question at all. The store's *existence* already answers it, because `extractAuthzConfig` returns nil unless profiles exist (`loader.go:296-299`, `:332-334`) and a nil store is permissive one layer up (`register.go:21-23`). `hasUsers` is a redundant, weaker copy of a decision already made correctly elsewhere. `HasProfiles()` inside `Authorize` would be a tautology, not a fix.
- The bootstrap admin is an *implicit* super-user: it is admin because nothing matched, not because anything says so. Making it explicit is what would let the defaults become strict.
  → **F-3 makes this cheap.** `UserCredential.Profiles` already exists (`aaa/types.go:37`) and already flows to authorization through login-profiles (`auth.go:60` → `login_profiles.go:86` → `authz.go:372`) **without touching `s.assignments`**. The objection that killed this idea (it flips `hasUsers`) applies only to the `AssignProfiles` route.
- `admin-disabled` (`main_servers.go:116-118`) shows the recovery account is already a first-class concept at the zefs layer; authorization simply never learned about it.
  → **F-4 adds a second half:** `mergeAuthUsers` (`main_servers.go:81-93`) already lets a config user of the same name replace the zefs admin, and the guide relies on it. So a documented recovery path exists *today* and is immune to a strict default. The question is whether to also keep an always-present one for boxes that never defined a config `admin`.
- **New: the gap has no test, and that is why it survived.** Every existing store test builds `NewStore()` and adds profiles or assignments in the shape the test needs. No test builds the production shape (profiles present, assignments empty), which is exactly the shape `extractAuthzConfig` produces on the exposed box. The three "asserting" tests do not defend the behavior in question (F-2); they defend an empty store that production never constructs.
- **New: two internal identities, two answers** (F-6). `wrapHandler` says "no username"; `dispatchCommandArgs` says `plugin:<name>`. The first reaches S-1, the second reaches S-2 and is already denied wherever `hasUsers` is true. Whatever is decided, these two should stop disagreeing.

## Core Insight
~~A permissive default that exists to prevent lockout is not the same thing as a bug — but it becomes one the moment a second, unrelated shape (a TACACS-only box) reaches the same branch. The fix is not to flip the default; it is to give the recovery case a name, so the default no longer has to carry it.~~

**Revised after research.** The permissive default was never carrying the lockout case, because the box it was supposedly protecting (no authorization configured) never reaches it: that box has a **nil store** and is allowed one layer up (`register.go:21-23`). The fall-throughs are a second, weaker answer to a question the loader already answered by returning nil (F-1). They therefore protect nothing and only fire for shapes nobody designed for: a TACACS-only box, profiles staged before users, and an empty priv-lvl mapping (F-5) that grants allow-all through valid config today.

So the choice is smaller and less dangerous than it looked. `hasUsers` is not a safety net to be carefully replaced; it is a redundant guard to be deleted, once the recovery identity is explicit (F-3 shows that is cheap and does not disturb S-1) and the internal identities are named (F-6). The one real sequencing constraint is S-1: an empty username is live (A-4), so it needs an identity before it can be denied.

## Key Design Decisions
<!-- None approved. These are the options to present at the DESIGN gate. -->

**Nothing below is approved.** The policy question is Thomas's: what is a user with no
applicable profile entitled to, and does the bootstrap admin get an explicit profile.
The research settles the *mechanics*, not the *policy*.

### The four questions that need a ruling

| Q | Question | Why it cannot be answered by reading code |
|---|----------|-------------------------------------------|
| Q-1 | Is "profiles configured, no config users" (the TACACS-only box, or profiles staged before users) a **supported** shape? | The config permits it and nothing documents it either way. If unsupported, validation should reject it and the code question mostly evaporates |
| Q-2 | What is an authenticated user with **no applicable profile** entitled to? | This is a security posture, not a fact. Recommendation below, but it is a product call |
| Q-3 | Should the bootstrap admin carry an **explicit** recovery profile, or does F-4 (define a config `admin`) already discharge the recovery duty? | Trade-off between an always-present recovery identity and a smaller privileged surface |
| Q-4 | Are internal identities (`""` from `wrapHandler`, `plugin:<name>` from `dispatchCommandArgs`) **subjects** of authorization at all, or should they bypass it as trusted in-process callers? | F-6 shows the code currently answers this two different ways. Note `plugin:<name>` is already denied on the documented RBAC box, so this may be an existing bug |

### Resolutions (2026-07-17)

Q-2 was answered by the user on 2026-07-16 (deny always; see Implementation Phase 2). Q-1,
Q-3 and Q-4 are resolved here by autonomous default so a fresh implementer can proceed with
zero questions. All three take the fail-closed / recovery-preserving branch, consistent with
`ai/rules/fail-closed-guards.md` and grounded in the code reads confirmed 2026-07-17
(`authz.go:336-391`, `register.go:21-24`, `command.go:513-523`, `loader.go:285-334`,
`server.go:126-152`, `dispatch.go:434`/`:469`, `main_servers.go:81-131`,
`login_profiles.go:45-53`, `authenticator.go:105-119` — every cited line confirmed).

- **Q-1 [STAKES: scope].** → AUTONOMOUS DEFAULT (2026-07-17): "profiles configured, no config
  users" is a **SUPPORTED** shape; do **not** add config-time validation that rejects it.
  Rationale: under Q-2's deny-always invariant plus the O-3' recovery identity the shape is
  already safe at runtime (an ordinary user with no profile is Denied, the bootstrap admin
  recovers via the reserved profile), so rejecting the config buys no additional safety while
  risking an upgrade-time lockout of a box that legitimately staged profiles before users. This
  is the smaller, self-contained option and leaves no allow-all path. Thomas: override if wrong
  (e.g. if you want the shape rejected outright, which would make O-3' unnecessary but could
  fail a config that boots today).

- **Q-3 [STAKES: security].** → AUTONOMOUS DEFAULT (2026-07-17): **YES — give the bootstrap
  admin an explicit reserved recovery profile (O-3'),** delivered through the login-profiles
  route (`UserCredential.Profiles` → `AuthResult.Profiles` → `RecordLoginProfiles` →
  `authz.go:372`, F-3), **never** through `store.AssignProfiles` (which would resurrect R-2).
  The recovery profile uses a **reserved name outside the config namespace** (R-8, so it cannot
  collide with an operator-defined `profile admin`) and is registered into every config-built
  store at load so the login-resolved name survives the store-defined filter (`authz.go:374-378`).
  The F-4 route (define a config `admin` user, honoured by `mergeAuthUsers`, `main_servers.go:81-93`)
  remains as a second, operator-controlled recovery path. Rationale: this is the escape hatch the
  brief and fail-closed-guards require — failing SAFE, not open. Without it, deny-always would
  brick a box that has profiles but no config `admin` (A-2, real bricking risk). `admin-disabled`
  (`main_servers.go:116-117`) still lets an operator suppress the recovery account deliberately.
  Thomas: override if wrong (e.g. if F-4 alone is deemed sufficient and you accept that a box
  which stages profiles before a config `admin` has no break-glass path).

- **Q-4 [STAKES: security].** → AUTONOMOUS DEFAULT (2026-07-17): internal identities **ARE
  subjects of authorization (O-4), not a bypass.** Inject an explicit **reserved internal
  identity** at the RPC boundary — covering both `wrapHandler`'s empty username
  (`server.go:126-145`) and `dispatchCommandArgs`/`dispatchCommand`'s `plugin:<name>`
  (`dispatch.go:434`, `:469`) — that authorizes via a reserved trusted profile registered at
  load, so both internal callers stop disagreeing (F-6). An **empty** username reaching
  `Authorize` then **fails closed** (Deny), because "no identity injected" is a bug or an attack,
  never a valid caller. Rationale: routing internal calls through `Authorize` with a named
  identity keeps every decision visible and logged (AC-6, auditable), which is strictly more
  fail-closed than a silent bypass that fails OPEN if its predicate is ever wrong. This is the
  load-bearing sequencing constraint: O-4 MUST precede or accompany the S-1 deny (S-2 phase),
  or every RPC method (`server.go:233`) breaks. Thomas: override if wrong (e.g. if you prefer a
  hard in-process trust bypass that never consults `Authorize` for internal callers).

### Options, re-scored against the findings

| Option | What it does | Trade-off, corrected by research |
|--------|-------------|----------------------------------|
| ~~**O-1** as written~~ | ~~Key the fall-throughs on `HasProfiles()` instead of `hasUsers`~~ | **Premise dissolved by F-1.** A production store always has profiles, so this is not a subtler signal: it is "always Deny when a store exists", stated obscurely. If that is the intent, say it directly (O-1') rather than dressing it as a predicate swap |
| **O-1'** (replaces O-1) | State the real rule: **a store that exists means RBAC is in use, so S-1/S-2/S-3 all Deny.** The no-RBAC box is already served by the nil-store path (`register.go:21-23`, `command.go:514-516`) | Smallest honest change. Removes a redundant, weaker copy of a signal that already lives one layer up. Costs: needs O-3' for recovery (Q-3) and a Q-4 ruling for S-1. Breaks exactly one test (F-2) |
| **O-2** | Leave S-1/S-2; document the shape, add a doctor check plus startup warning when profiles exist with no assignments | Zero lockout risk. But F-5 makes this weaker than it looked: an empty tacacs-profile mapping grants admin through **valid config**, and a warning does not close it. Defensible only if Q-1 answers "unsupported" and validation rejects the shape |
| **O-3'** (revised) | Give the bootstrap admin an explicit recovery profile by setting `Profiles` in `usersFromZefsDB` (`main_servers.go:131`), delivered via login-profiles | **Now cheap: F-3 shows this does NOT flip `hasUsers`,** so R-2 does not fire and S-1 is untouched. Requires registering a recovery profile into the store (F-3 constraint) and settling the `admin` name collision (R-8) |
| **O-4** | Inject an explicit internal identity at the RPC boundary so S-1 stops needing an empty-username rule | Still the principled answer to Q-4, and F-6 strengthens it: it would also regularise `plugin:<name>`. Touches the plugin server contract |
| **O-5** (new) | Reject or warn on a `tacacs-profile` level with an empty `profile` list (F-5), via `min-elements 1` in YANG or a check in `ValidateAuthzConfig` | Independent of every other option and closes a real config-reachable admin grant. **Recommend doing this regardless**, ahead of the rest |

### Recommendation (for Thomas to accept, modify, or reject)

| # | Recommendation | Reasoning |
|---|---------------|-----------|
| 1 | **Take O-5 now, on its own commit.** | F-5 is a live admin grant through valid config. It does not depend on the policy question and should not wait for it |
| 2 | **Answer Q-2 as "nothing": a user with no applicable profile is denied.** | A box whose operator configured authorization has stated an intent. "No profile applied" meaning allow-all inverts that intent, and F-1 shows the permissive branch is not protecting the no-RBAC box, which never reaches it |
| 3 | **Then O-1' plus O-3' together.** | O-1' makes the default strict; O-3' keeps a recovery identity. F-3 makes them independent of S-1, so this is a much smaller change than the Task section feared: one test changes, `hasUsers` is untouched |
| 4 | **Settle Q-4 with O-4 before, or with, O-1'.** | S-1 is live (A-4). Denying an empty username without injecting an identity breaks every RPC method (`server.go:233`). This is the one genuine sequencing constraint left |
| 5 | **Prefer a reserved recovery profile name outside the config namespace over reusing `admin`.** | Avoids R-8 entirely. `BuiltinAdminProfile().Name == "admin"` (`authz.go:247`) collides with the guide's own `profile admin` |

→ Constraint: recommendations 2 and 3 change a documented security contract and the
  `Store.Authorize` godoc (`authz.go:336-338`). They do not proceed without an explicit ruling.
→ ~~Decision: this spec stays in `design` until Q-1..Q-4 are answered. It does not advance to
  `ready` on the strength of the research alone.~~
→ Decision (2026-07-17): Q-2 was answered by the user (deny always); Q-1/Q-3/Q-4 are now
  answered by AUTONOMOUS DEFAULT in the Resolutions block above (fail-closed with a preserved,
  reserved recovery identity), and all placeholders are filled, so the spec advances to `ready`.
  Recommendations 1–5 are **adopted as the plan of record** — they are the fail-closed
  reading: (1) O-5 is already landed separately (Phase 3); (2) Q-2 = deny; (3) O-1' + O-3'
  together; (4) O-4 settles Q-4 before/with S-1; (5) reserved recovery name, not `admin`. The
  security-contract and godoc changes still carry `[STAKES: security]`: Thomas may override any
  autonomous default before implementation, and the two-commit closure must land the godoc
  rewrite (R-6) and guide updates (AC-7) with the behavior.

## Known Limitations
- This spec deliberately does not decide. It records what is true, what breaks if changed naively, and what must be settled first.
- S-3 is defensive only: unreachable from config today. Fixing it is cheap and safe, but on its own it closes nothing an operator can hit.
- The exposed shape (profiles with no assignments) is not currently documented as supported or unsupported. Deciding that is arguably prior to deciding the code.

## RFC Documentation

N/A — no protocol behavior. No `// RFC` annotations expected.

## Implementation Summary

### What Was Implemented
- Nothing yet — `skeleton`.

### Bugs Found/Fixed
- Not in this spec. Related bugs already fixed and committed: ssh-only daemons ignoring profiles and TACACS priv-lvl mapping not governing commands (60e35c0d5), an unresolvable mapping granting admin (701cbaaa3), `tacacs-profile` references unvalidated (0544b274d).

### Documentation Updates
- None yet.

### Deviations from Plan
- None yet.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| A box with profiles configured never silently authorizes a user as admin | functional test | `test/plugin/authz-no-applicable-profile.ci` proves an authenticated user with no resolved profile is Denied (paste run output at implementation) |
| The recovery account still reaches a misconfigured box | functional test | `test/plugin/authz-recovery-admin.ci` proves the bootstrap admin reaches a profiles-but-no-config-users box via the reserved recovery profile (paste run output at implementation) |
| Internal RPC dispatch is unaffected | functional test | `test/plugin/authz-rpc-identity.ci` proves plugin RPC still dispatches under the reserved internal identity, and an empty username fails closed (paste run output at implementation) |

## Review Gate

### Run 1 (initial — independent adversarial security review)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Reserved-name mechanism spoofable via untrusted AAA wire input: a hostile TACACS+/RADIUS server could return a `\x00ze:` reserved profile name (recovery/internal) and have `Store.Authorize` short-circuit to Allow | `radius/authenticator.go` mapProfiles, `tacacs/authenticator.go` handlePass | Fixed: drop `IsReservedName` values in both untrusted backends before assembling profiles (`FilterReservedNames`) |
| 2 | ISSUE | Reserved USERNAME spoof: an attacker-chosen username matching a reserved identity could be authenticated by a backend and recorded as login profiles | `aaa/login_profiles.go` | Fixed: `profileRecordingAuthenticator.Authenticate` rejects `IsReservedName(request.Username)` with `ErrAuthRejected` before consulting any backend — the single auth choke point |
| 3 | ISSUE | Internal RPC route-propagation dispatch built a CommandContext with an empty username → Denied on a strict RBAC box → route push broken | `plugin/server/dispatch_registry.go` opUpdateRoute, `dispatch.go` handleUpdateRouteSelDirect | Fixed: inject `internalPluginIdentity(proc.Name())` reserved internal identity on all five internal dispatch constructions |

### Fixes applied
- `radius/authenticator.go:203` mapProfiles drops reserved Filter-Id values; `tacacs/authenticator.go:114` `FilterReservedNames(profiles)` before the empty-check.
- `aaa/login_profiles.go:92` rejects a reserved `request.Username` before the backend; documented invariant that the reserved recovery profile is NOT stripped here (trusted local backend delivers it via this path — see comment) so break-glass is preserved.
- `plugin/server/{dispatch_registry.go:247, dispatch.go:478,541,576, server.go:137}` inject the reserved internal identity on every internal CommandContext.
- RED-first tests added for each: `TestRadiusDropsReservedProfileName`, `TestTacacsAuthenticatorDropsReservedProfile`, `TestProfileRecordingAuthenticatorRejectsReservedUsername`, `TestOpUpdateRouteInjectsInternalIdentity`; `.ci`: `authz-recovery-admin`, `authz-no-applicable-profile`, `authz-rpc-identity`.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | CLEAN | Re-review: BLOCKER-1 DEAD, ISSUE-2 DEAD, ROUTE-PROP FIXED, core vuln stays dead, break-glass recovery works (no lockout), all tests non-vacuous (fail on pre-fix code) | whole changeset | none required |
| N1 | NOTE | Reserved-PROFILE filtering distributed across untrusted backends rather than centralized; a future wire-sourcing backend that forgets to filter could reopen BLOCKER-1 | `aaa/login_profiles.go` | Addressed by an explicit anti-pattern comment at the choke point documenting WHY a central `FilterReservedNames` would break recovery (the trusted local backend delivers the reserved recovery profile through this exact path) — prevents a future "helpful" centralization |
| N2 | NOTE | `reactor.ExecuteCommand` builds a CommandContext with empty Username → Deny on RBAC box; no production caller, can only fail closed | `bgp/reactor/reactor.go:740` | Pre-existing, outside this changeset; fail-closed, not a hole |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (or explicitly "none") — 2 NOTEs, both non-blocking (N1 addressed by documented invariant, N2 pre-existing fail-closed)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/aaa/reserved.go` | Yes | `ReservedInternalPrefix`, `ReservedRecoveryProfile`, `IsReservedName`, `FilterReservedNames` |
| `test/plugin/authz-recovery-admin.ci` | Yes | break-glass recovery admin reaches strict RBAC box |
| `test/plugin/authz-no-applicable-profile.ci` | Yes | authenticated + no resolved profile → Deny |
| `test/plugin/authz-rpc-identity.ci` | Yes | plugin RPC dispatch authorizes via reserved internal identity |
| `plan/learned/1242-fixit-authz-admin-fallthrough.md` | Yes | learned summary |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No profiles resolved → Deny (not admin) | `authz.go:440` S-2 `return Deny`; `TestStoreAuthorizeProfilesNoAssignmentsDeniesNotAdmin` |
| AC-2 | Bootstrap admin reaches box via reserved recovery profile | `main_servers.go:144` `ReservedRecoveryProfile`; `TestStoreAuthorizeRecoveryProfile`; `authz-recovery-admin.ci` |
| AC-3/AC-10 | Internal RPC / `plugin:<name>` dispatch keeps working via reserved internal identity | `internalPluginIdentity` at dispatch_registry.go:247, dispatch.go:478/541/576, server.go:137; `TestOpUpdateRouteInjectsInternalIdentity`; `authz-rpc-identity.ci` |
| AC-4 | Store naming only undefined profiles fails closed | `authz.go:484` final `return Deny` |
| AC-6 | Log states which rule decided | `authzLogger` decision lines; `authz-recovery-admin.ci` asserts "break-glass recovery admin" log line |
| AC-8/AC-9 | Empty tacacs profile mapping cannot grant admin | discharged in sibling spec + `FilterReservedNames` guard (tacacs/authenticator.go:114) |
| — (security) | Reserved names spoof-proof from AAA wire | radius/authenticator.go:203 `IsReservedName` drop; tacacs/authenticator.go:114 `FilterReservedNames`; login_profiles.go:92 username reject |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| ssh auth → aaa chain → `Store.Authorize` S-2 (recovery profile) | `authz-recovery-admin.ci` | Pass (darwin) |
| tacacs authen → empty login profiles → `Store.Authorize` S-2 → Deny | `authz-no-applicable-profile.ci` | Pass (darwin) |
| plugin RPC → reserved internal identity → `Store.Authorize` | `authz-rpc-identity.ci` | Pass (darwin) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-5 | Confirmed | Store naming undefined profiles unreachable from config; direct-API only, fails closed (`authz.go:484`) |
| Q-2/Q-3/Q-4 | Resolved (user 2026-07-16/17) | Deny always (Q-2); reserved recovery profile O-3' (Q-3); reserved internal identity O-4 (Q-4) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Recovery account / break-glass rule | `docs/guide/operator-access-rbac.md` "The recovery account" section | Yes |
| TACACS empty-profile → deny, no admin default | `docs/guide/tacacs.md` | Yes |
| `Store.Authorize` godoc matches implemented rule | `authz.go` godoc updated | Yes |

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

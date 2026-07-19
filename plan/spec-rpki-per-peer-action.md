# Spec: rpki-per-peer-action

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/10 |
| Updated | 2026-07-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rpki/rpki_config.go`, `rpki.go` (buildDecisions), `yang/ze-rpki.yang`
4. `internal/component/bgp/plugins/role/config.go` (resolution SHAPE only), `internal/component/bgp/plugins/watchdog/config.go` (CORRECT remote-IP path: connection>remote>ip), `configjson/traverse.go`

## Task

Make the RPKI validation action settable per-peer and per-group while keeping the
global option. Rename the global origin container `rpki { policy { invalid-action;
not-found-action } }` to `rpki { action { invalid; not-found } }`, and the ASPA
container `rpki { aspa { policy { invalid-action; unknown-action } } }` to
`rpki { aspa { action { invalid; unknown } } }`. Add the same `action` block under
peers and groups. Precedence: peer > group > global, resolved per leaf. Global-only
(never per-peer): `cache-server`, `validation-timeout`, `aspa/validation` enable.
Extend `show bgp rpki status` to display the effective global actions and a per-peer
breakdown (resolved actions with their source: peer / group / global).

Also in scope (surfaced during research, user approved folding in): introduce a shared
`configjson.PeerRemoteIP(peerMap, groupMap)` helper that reads `connection>remote>ip`
(peer wins over group), have RPKI use it for per-peer keying, and migrate `role` and
`watchdog` onto it. This fixes a latent `role` bug: `role.extractRemoteIP` reads a stale
flat `remote/ip` path, so RFC 9234 OTC config-role filtering is silently disabled on
real configs (details in Design Insights). Includes fixing role's unrealistic unit
fixtures and hardening the vacuous `role-otc-ingress-reject` assertion.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/config-option.md` - structural template for a new YANG config leaf/container
  → Constraint: every leaf gets maximum native validation; enums are natively completed; kebab-case names.
- [ ] `ai/rules/config-surface.md` - YANG-vs-env-var decision
  → Decision: this is per-peer routing policy, so YANG config (not env var) is correct; it must be schema-visible and completable.
- [ ] `ai/rules/plugin-self-containment.md` - remove the plugin, all its features vanish
  → Constraint: per-peer RPKI action lives entirely in the rpki plugin (YANG augment + parser + decision); no rpki spelling in generic/central packages. The shared traversal helper `configjson` is the only generic touch and already exists.
- [ ] `docs/guide/rpki.md` - user-facing config reference for RPKI (tables at lines 60-64, 214-227)
  → Constraint: config tables document `policy/invalid-action` etc.; the rename must update these rows and add per-peer/group documentation.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6811.md` - Origin Validation: the action per validation state is operator-configurable
  → Constraint: RFC 6811 Section 2/3 make excluding Invalid an explicit operator policy choice, not an automatic side effect; per-peer granularity is a policy refinement, still RFC-conformant (a peer's unset action falls back to a configured global action).

**Key insights:**
- The "action" is applied in `buildDecisions` (rpki.go:640-685), which already holds `req.peerAddr` (the route's source peer remote IP). Per-peer dispatch is a map lookup at that point.
- The `role` plugin is a complete template for per-peer/group plugin config: augment 3 levels, resolve peer > group in a `configjson.ForEachPeer` visitor, key the result map by remote IP.
- Rename is a hard cutover (no dual-support): the old `policy` leaves stop existing; existing test fixtures/configs/docs using old syntax must be migrated in the same change.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - defines `grouping rpki-config`, container `rpki` (presence), with `policy` (origin: `invalid-action` default reject, `not-found-action` default accept) and `aspa/policy` (`invalid-action` default log-only, `unknown-action` default accept). Augmented only at `/bgp:bgp` (global).
  → Constraint: container is `presence` (enables validation + starts RTR). Per-peer must NOT reuse the full grouping (would drag `cache-server`/`validation-timeout` onto every peer). Needs a separate policy-only grouping.
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` - `parseRPKIConfig(jsonStr)` reads only the `bgp/rpki` subtree; parses `policy/invalid-action` -> `OriginInvalidAction`, `policy/not-found-action` -> `OriginNotFoundAction`, `aspa/policy/*` -> `ASPA*Action`. Defaults set in the struct literal (rpki_config.go:68-74). Does NOT parse any per-peer config today.
  → Constraint: action strings map to uint8 via `aspaActionFromString` (reject=0, log-only=1, accept=2). Reuse this mapping for per-peer leaves.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - `startSessions` stores actions into lock-free atomics `originInvalidAction`/`aspaInvalidAction`/`aspaUnknownAction` (rpki.go:272-274). `buildDecisions` (rpki.go:640-685) loads those atomics once per batch and, per route, computes `reject` from `req.state`, `req.aspaState`, and the actions. Each decision carries `req.peerAddr` (rpki.go:677).
  → Constraint: `buildDecisions` runs in the single `validationWorker` goroutine (rpki.go:553). Config reload writes actions from the OnConfigure goroutine. Current sync is per-field atomics; a per-peer map needs an equivalent lock-free swap (atomic.Pointer to an immutable map).
  → Constraint: `req.peerAddr` originates from `se.PeerAddress` (rpki.go:313) / `event.GetPeerAddress()` (rpki.go:452) = the peer's remote IP. The per-peer map must be keyed by remote IP to match.
- [ ] `internal/component/bgp/plugins/role/config.go` - `extractPeerRoleConfigs` walks `configjson.ForEachPeer(peerAddr, peerMap, groupMap)`, resolves peer > group (`parseRoleFromMap`), and keys the result map by `extractRemoteIP(peerMap, groupMap)` (role/config.go:151-165), falling back to the config key.
  → Constraint: reuse the RESOLUTION shape (ForEachPeer + peer>group), but NOT role's `extractRemoteIP`: it reads the flat `m["remote"]["ip"]` path, which is stale post connection-reorganization and returns "" on real config. See the role-bug finding in Design Insights. RPKI must read `connection>remote>ip` like `watchdog`.
- [ ] `internal/component/bgp/plugins/watchdog/config.go` - `extractRemoteIP` (watchdog/config.go:110-123) reads `connection > remote > ip` (the correct path) and keys its pools by that IP. This is the CORRECT reference for RPKI's per-peer keying.
  → Constraint: RPKI's remote-IP extraction must mirror watchdog (connection>remote>ip), peer map wins over group. This spec lifts a shared `configjson.PeerRemoteIP` helper and migrates role + watchdog onto it, fixing role's bug against one correct implementation.
- [ ] `internal/component/bgp/plugins/role/role.go` - stores the per-peer map under a `sync.RWMutex` (role.go:47-93) because filters read it on the data path from multiple goroutines.
  → Decision: RPKI reads the per-peer map from ONE goroutine (validationWorker), so an atomic.Pointer swap is sufficient and lock-free; no RWMutex needed.
- [ ] `internal/component/bgp/configjson/traverse.go` - `ForEachPeer` visits standalone (`bgp.peer`) and grouped (`bgp.group.<name>.peer`) peers, passing both `peerMap` and enclosing `groupMap`. Inheritance is the plugin's responsibility (config JSON is the raw nested tree, not flattened).
  → Constraint: group->peer merge is done in-plugin, not upstream.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - `statusCommand` (rpki.go:928-964) emits JSON via the buffer-first `textbuf` pattern (`b.Str`/`b.Int`/`b.Bool`, no fmt/alloc). It currently reports `running`, VRP counts, `sessions`, `aspa-enabled`, `aspa-records`, and a `cache-servers` array. It does NOT currently emit ANY configured actions (not even global).
  → Constraint: the command contract is JSON; `.ci` tests assert on JSON fields via `result_json_data` (e.g. test/plugin/rpki-cache-connect.ci:69). Extend by adding new JSON fields (`actions` global + `peer-actions` array); keep buffer-first encoding, no fmt.Sprintf.
  → Constraint: `statusCommand` must read the per-peer map and global actions from the same atomic sources the validation path uses, so the display never disagrees with enforcement.

**Behavior to preserve:**
- Global RPKI behavior for peers with no override is unchanged (same effective actions, same RTR/cache handling).
- `aspaOverridesAccept` semantics (ASPA can override an origin "accept" to reject) at rpki.go:667-669.
- The RFC 6811 default: Invalid=reject, NotFound=accept; ASPA Invalid=log-only, Unknown=accept.
- Lock-free hot path in `buildDecisions` (no per-route allocation, no lock).

**Behavior to change:**
- YANG container `policy` -> `action`; leaves `invalid-action` -> `invalid`, `not-found-action` -> `not-found`, ASPA `unknown-action` -> `unknown` (global block). Breaking rename, no dual-support.
- Add `action` block (origin + ASPA) under peer and group nodes.
- `buildDecisions` selects per-route actions by `req.peerAddr`, falling back to global.
- NEW ENFORCEMENT (user-approved during implement audit): wire origin `not-found` enforcement, which is currently parsed but inert. Add `originNotFoundAction atomic.Uint32`, store in `startSessions`, and in `buildDecisions` reject NotFound routes when action==reject and log when log-only (mirroring the existing Invalid handling). This makes `not-found` meaningful globally AND per-peer. Behavior change: existing configs with `not-found-action reject`/`log-only` (previously no-ops) will start acting.
- `show bgp rpki status` gains an `actions` object (effective global actions) and a `peer-actions` array (per-peer resolved actions with the source of each leaf: peer/group/global).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- BGP config JSON delivered to the rpki plugin via `OnConfigure` with root `bgp` (rpki.go:196). Contains global `rpki` block plus the full `peer`/`group` tree.

### Transformation Path
1. `parseRPKIConfig` (rpki_config.go) parses the renamed global `rpki/action` and `rpki/aspa/action` into `rpkiConfig` (unchanged fields, new source keys).
2. NEW: `parseRPKIConfig` also walks `configjson.ForEachPeer`, resolving per leaf `invalid`/`not-found`/aspa `invalid`/`unknown` with precedence peer > group > (global), keyed by remote IP, into a per-peer action map.
3. `startSessions` stores the global actions (existing atomics) AND swaps the per-peer map into an `atomic.Pointer[map[string]peerActions]`.
4. `buildDecisions` (validationWorker) loads global actions once, then per route looks up `req.peerAddr` in the per-peer map; a hit supplies leaf overrides, misses fall through to global.
5. `statusCommand` reads the same global atomics + per-peer map and serializes them into the `actions` / `peer-actions` JSON fields (display uses the identical resolution as enforcement).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin | config JSON via OnConfigure (root=bgp) | [ ] |
| Config parse -> validation worker | atomic.Pointer swap of immutable per-peer map | [ ] |
| Route peer identity | `req.peerAddr` (remote IP) matched to config remote IP key | [ ] |

### Integration Points
- `parseRPKIConfig` (extend), `startSessions` (rpki.go:263, add map swap), `buildDecisions` (rpki.go:640, add per-peer lookup), `RPKIPlugin` struct (add atomic.Pointer field).
- `configjson.ForEachPeer` + a `connection>remote>ip` extraction (mirror `watchdog.extractRemoteIP`, NOT role's flat helper; see A-4 and the role-bug finding).

### Architectural Verification
- [ ] No bypassed layers (per-peer resolution stays inside the rpki plugin)
- [ ] No unintended coupling (uses generic `configjson`, no cross-plugin import)
- [ ] No duplicated functionality (extends parse + decision; no second validation path; consolidates 2 divergent `extractRemoteIP` copies into 1 shared `configjson.PeerRemoteIP`)
- [ ] Zero-copy preserved where applicable (map lookup by string key; no per-route alloc)
- [ ] Registration over hardcoding (YANG augment via the plugin's own module; no central switch touched)
- [ ] Shared helper is generic (lives in `configjson` next to `ForEachPeer`; no plugin-specific spelling); role/watchdog bug fixes stay within their own packages

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `req.peerAddr` at decision time equals the peer's configured remote IP string | rpki.go:313/452 set it from `se.PeerAddress`/`event.GetPeerAddress()`; role keys its map the same way (role/config.go:209-217) | per-peer lookup always misses -> silently falls back to global | functional test: per-peer override changes outcome for a route from that peer | confirmed (rpki.go:313 `peerAddr := se.PeerAddress`, :452 `event.GetPeerAddress()`; string format equivalence still proven by functional test) |
| A-2 | Config framework delivers YANG leaf values as JSON strings ("reject"), same as global parse | rpki_config.go:93 reads `policyMap["invalid-action"].(string)`; role notes same (role/config.go:250) | parser reads wrong type -> override ignored | unit test on `parseRPKIConfig` with per-peer JSON | confirmed (existing global parse at rpki_config.go:93 already relies on `.(string)`) |
| A-3 | Leaf-level inheritance (unset leaf falls through) is the desired semantic, not container-level replace | user requirement "keeping the current global option"; Junos-like inheritance | partial peer override wipes global not-found | user confirmation at design gate + AC-5 | confirmed (user selected "Per-leaf fallback" at design gate) |
| A-4 | The delivered peer config keys peers by NAME with remote IP nested at `connection/remote/ip`; RPKI must key its per-peer map on that IP (like `watchdog`), NOT the flat `remote/ip` path `role` reads | RESOLVED: `Tree.ToMap()` emits a keyed YANG list keyed by entry name (authradius/config.go:142-145); `watchdog.extractRemoteIP` reads connection>remote>ip (watchdog/config.go:110-123); real config `peer frr-rpki { connection { remote { ip } } }` (test/interop/scenarios/43-rpki-frr/ze.conf:27-33) | if RPKI copied role's flat path, every per-peer lookup would miss and silently fall back to global | producer confirmed + `role`-bug finding below | confirmed |
| A-5 | Renaming the global leaves does not silently accept old configs (old leaf becomes a hard YANG error) | YANG schema validation rejects unknown leaves | old configs pass silently with default actions | parse test: old `policy { invalid-action }` config is rejected | confirmed (`ze config validate` on `rpki { policy { invalid-action reject; } }` -> exit 1 "unknown field in rpki: policy"; new `action { invalid reject; }` -> valid) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Dynamic/range peers (no static remote IP, keyed by name) can't match a remote-IP-keyed override | functional test with a dynamic peer shows global action only | Document as known limitation; per-peer action applies to statically-addressed peers (same limit as role plugin) |
| R-2 | Rename breaks existing fixtures/tests/demos using old syntax | `make ze-functional-test` reds on rpki-aspa-policy-* and interop 43-rpki-frr | Migrate all 5 test files + 2 config fixtures + demo in the same change (no deferral) |
| R-3 | Config reload while validationWorker reads the map -> torn read | data race under `-race` | atomic.Pointer swap of an immutable (never-mutated-after-publish) map |
| R-4 | ASPA `action` block present on a peer while ASPA `validation` is globally disabled | per-peer aspa override has no effect | Expected: ASPA enable stays global; document that per-peer aspa action only applies when ASPA validation is globally on |
| R-5 | Hardening `role-otc-ingress-reject.ci` blocked by the "validation gate makes positive assertions unreliable" race the file notes | the reworked .ci flakes on adj-rib-in count | Assert via the dest-peer WIRE (route not re-advertised), the pattern `role-otc-egress-filter.ci` already uses; the deterministic regression guarantee is the UNIT test `TestOTCIngressFilter_RejectsLeakRealConfig`, which never touches the gate. If even the wire-negative is flaky, keep the unit test as the guard and record the functional limitation (do not weaken to a vacuous check). |
| R-6 | Migrating `watchdog`/`role` to the shared helper changes remote-IP semantics (role previously fell back to group; watchdog never did) | existing watchdog/role tests red | `PeerRemoteIP` keeps peer-wins-over-group; watchdog passes its `groupMap` (harmless, group has no concrete member IP); run full watchdog + role suites before/after |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `peer X { rpki { action { invalid accept } } }` | → | `parseRPKIConfig` builds per-peer map; `buildDecisions` applies it for routes from X | `TestParseRPKIConfig_PerPeerOverride` (unit) + `rpki-per-peer-action.ci` (functional) |
| Config `group G { rpki { action { invalid reject } } peer Y {} }` | → | group override resolved for member peer Y | `TestParseRPKIConfig_GroupInheritance` (unit) |
| Config `peer P { connection { remote { ip } } role { import ... } }` | → | `configjson.PeerRemoteIP` -> role map keyed by IP -> `OTCIngressFilter` hits | `TestExtractPeerRoleConfigs_RealShape` + hardened `role-otc-ingress-reject.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Global `rpki { action { invalid log-only } }`, no per-peer block | All peers use log-only for Invalid origin (rename works; global still authoritative) |
| AC-2 | Global default reject; peer X sets `rpki { action { invalid accept } }` | Invalid route from X is accepted; Invalid route from another peer is rejected |
| AC-3 | Group G sets `rpki { action { invalid reject } }`; member peer Y sets nothing | Invalid route from Y is rejected (group applies) |
| AC-4 | Group G sets `invalid reject`; member peer Y sets `rpki { action { invalid accept } }` | Invalid route from Y is accepted (peer beats group) |
| AC-5 | Peer X sets only `action { invalid accept }`; global `not-found reject` | Invalid from X accepted; NotFound from X still reject (per-leaf fallback to global) |
| AC-6 | Global ASPA on; peer X sets `rpki { aspa { action { invalid accept } } }` | ASPA-Invalid route from X accepted; from another peer, global ASPA action applies |
| AC-7 | Config uses old syntax `rpki { policy { invalid-action reject } }` | Config rejected by YANG schema validation (hard rename, no dual-support) |
| AC-8 | No `action` anywhere (global or peer) | RFC 6811 defaults: Invalid=reject, NotFound=accept, ASPA Invalid=log-only, Unknown=accept |
| AC-9 | `show bgp rpki status` with only global actions configured | JSON includes an `actions` object with the effective global origin + ASPA actions |
| AC-10 | `show bgp rpki status` with peer X overriding `invalid` (not `not-found`) | JSON `peer-actions` array has an entry for X showing `invalid` sourced from `peer` and `not-found` sourced from `global`, matching what enforcement applies |
| AC-11 | `configjson.PeerRemoteIP` given a peer map with `connection>remote>ip` set | Returns that IP; returns group's when peer lacks it; returns "" when neither has it |
| AC-12 | `role` config parsed from realistic delivered JSON (peer keyed by name, `connection>remote>ip` nested) | role's per-peer config map is keyed by the remote IP, so `OTCIngressFilter` lookup by IP hits and a route leak from a Customer is rejected (was silently accepted before) |
| AC-13 | `watchdog` config parsed after migration to `configjson.PeerRemoteIP` | watchdog pools remain keyed by remote IP; existing watchdog behavior unchanged |
| AC-14 | Global `rpki { action { not-found reject } }`, route with NotFound origin state | Route is rejected (NotFound enforcement now wired; was a no-op before). `log-only` logs and keeps the route; `accept` keeps it silently |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a per-peer `rpki { action { invalid accept } }` and receives an Invalid route from that peer | config JSON -> parseRPKIConfig per-peer map -> buildDecisions lookup by peerAddr -> Accept | `rpki-per-peer-action.ci` |
| 2 | Sets a group-level RPKI action and adds peers to the group | config JSON -> ForEachPeer group resolution -> member peers inherit | `rpki-group-action.ci` |
| 3 | Keeps only a global `rpki { action { ... } }` (existing behavior, new syntax) | config JSON -> global atomics -> buildDecisions fallback | `rpki-global-action.ci` (migrated from existing) |
| 4 | Runs `show bgp rpki status` to see which action applies to each peer | command -> statusCommand reads global atomics + per-peer map -> `actions` + `peer-actions` JSON | `rpki-per-peer-action.ci` asserts on `peer-actions` |
| 5 | Configures an RFC 9234 role on a named peer and a Customer leaks a route | delivered JSON (name-keyed, connection>remote>ip) -> `PeerRemoteIP` -> role map keyed by IP -> `OTCIngressFilter` lookup by IP hits -> reject | hardened `role-otc-ingress-reject.ci` + `TestExtractPeerRoleConfigs_RealShape` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRPKIConfig_RenamedGlobalKeys` | `rpki_config_test.go` | global `action { invalid/not-found }` parsed into OriginInvalidAction/OriginNotFoundAction | |
| `TestParseRPKIConfig_PerPeerOverride` | `rpki_config_test.go` | per-peer `action` builds map keyed by remote IP | |
| `TestParseRPKIConfig_GroupInheritance` | `rpki_config_test.go` | group action resolved for member peer lacking its own | |
| `TestParseRPKIConfig_PeerBeatsGroup` | `rpki_config_test.go` | peer override wins over group | |
| `TestParseRPKIConfig_PerLeafFallback` | `rpki_config_test.go` | unset leaf falls through to global (AC-5) | |
| `TestParseRPKIConfig_ASPAPerPeer` | `rpki_config_test.go` | per-peer ASPA action parsed | |
| `TestBuildDecisions_PerPeerAction` | `rpki_test.go` | buildDecisions applies per-peer override; miss -> global | |
| `TestBuildDecisions_NotFoundEnforced` | `rpki_test.go` | NotFound route rejected when not-found action==reject; kept on log-only/accept (AC-14) | |
| `TestStatusCommand_GlobalActions` | `rpki_commands_test.go` | status JSON includes effective global `actions` object | |
| `TestStatusCommand_PerPeerActions` | `rpki_commands_test.go` | status JSON `peer-actions` shows resolved actions + per-leaf source | |
| `TestPeerRemoteIP` | `configjson/traverse_test.go` | reads connection>remote>ip; peer wins over group; "" when absent (AC-11) | |
| `TestExtractPeerRoleConfigs_RealShape` | `role/config_test.go` | role config map keyed by remote IP for a name-keyed peer with connection>remote>ip (AC-12; fails before fix) | |
| `TestOTCIngressFilter_RejectsLeakRealConfig` | `role/otc_test.go` | OTCIngressFilter returns reject for a Customer leak when config parsed from realistic JSON (AC-12) | |
| `TestWatchdogPeerRemoteIP_AfterMigration` | `watchdog/config_test.go` | watchdog pools still keyed by remote IP via shared helper (AC-13) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (all fields are enums, not numeric) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-per-peer-action` | `test/plugin/rpki-per-peer-action.ci` | peer-level action override changes Invalid outcome for that peer only; `show bgp rpki status` reports the peer's resolved actions (AC-10) | |
| `rpki-group-action` | `test/plugin/rpki-group-action.ci` | group-level action applies to member peers | |
| `rpki-global-action` | `test/plugin/*.ci` (migrate existing) | global action with new `action` syntax still enforces | |
| `role-otc-ingress-reject` (harden) | `test/plugin/role-otc-ingress-reject.ci` | Customer route leak is actually rejected on real config (was vacuously passing) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `43-rpki-frr` (migrate syntax) | `test/interop/scenarios/43-rpki-frr/` | FRR | RPKI enforcement still interoperates after rename | |

### Future (if deferring any tests)
- None. All ACs tested in this change.

## Files to Modify
- `internal/component/bgp/configjson/traverse.go` - add `PeerRemoteIP(peerMap, groupMap map[string]any) string` reading `connection>remote>ip`, peer wins over group. Shared correct implementation. Design: `docs/architecture/config/syntax.md`.
- `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - rename global `policy`->`action` + leaves; add policy-only grouping + 3-level peer/group augments.
- `internal/component/bgp/plugins/rpki/rpki_config.go` - parse renamed global keys; add per-peer/group resolution keyed by remote IP via `configjson.PeerRemoteIP`.
- `internal/component/bgp/plugins/rpki/rpki.go` - add `atomic.Pointer` per-peer map field; swap in `startSessions`; per-peer lookup in `buildDecisions`; extend `statusCommand` (rpki.go:928) with `actions` + `peer-actions` JSON (buffer-first).
- `internal/component/bgp/plugins/role/config.go` - replace the buggy local `extractRemoteIP` (flat `remote/ip`) with `configjson.PeerRemoteIP`; delete the stale helper.
- `internal/component/bgp/plugins/role/config_test.go` - fix unrealistic fixtures (`{"peer":{"10.0.0.1":{...}}}`) to realistic delivered shape (peer keyed by name, `connection>remote>ip` nested); add a test asserting the config map is keyed by IP for a named peer.
- `internal/component/bgp/plugins/watchdog/config.go` - replace watchdog's local `extractRemoteIP` with `configjson.PeerRemoteIP` (verify group handling; watchdog currently takes a single peerTree arg).
- `test/plugin/role-otc-ingress-reject.ci` - harden the assertion (currently only checks `total-routes >= 0`); assert the leak is actually rejected via dest-peer wire (pattern from `role-otc-egress-filter.ci`) and/or adj-rib-in absence.
- `test/plugin/rpki-aspa-policy-logonly.ci`, `rpki-aspa-policy-reject.ci`, `rpki-aspa-policy-unknown-reject.ci` - migrate to `aspa { action { ... } }`.
- `test/parse/coverage-rpki.ci` - migrate to new syntax.
- `test/interop/scenarios/43-rpki-frr/ze.conf`, `demos/terminal/rpki/ze.conf` - migrate to new syntax.
- `docs/guide/rpki.md`, `docs/config-reference.md`, `docs/features.md` - config tables (rename) + per-peer/group documentation.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` (rename + peer/group augments) |
| YANG validation constraints | Yes | enum leaves (`invalid`/`not-found`/`unknown`) are natively constrained; no bare strings |
| YANG custom validators | N/A | native enum completion suffices |
| CLI commands/flags | Yes | extend existing `show bgp rpki status` output (`rpki.go:928`); no new verb |
| CLI grammar | N/A | no new command verb (extends an existing one) |
| Editor autocomplete | Yes | automatic for the enum leaves (YANG-driven) |
| Functional test for new RPC/API | Yes | `test/plugin/rpki-per-peer-action.ci`, `rpki-group-action.ci` |
| Pipe completeness | Yes | `show bgp rpki status` already routes through the command dispatcher; the new fields are part of the same JSON output, no separate pipe path |
| Env var registration | N/A | routing policy is peer config, not an env var (config-surface.md) |
| Doctor check for runtime dependencies | N/A | no new file/socket/port/binary; reuses existing RTR machinery |
| Prometheus counters/metrics | No | existing `validationOutcomes` counter already records per-state outcomes (rpki.go:655); per-peer label out of scope (see Known Limitations) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (per-peer RPKI action) |
| 2 | Config syntax changed? | Yes | `docs/guide/rpki.md`, `docs/config-reference.md` (rename + per-peer/group) |
| 3 | CLI command added/changed? | Yes | `docs/guide/rpki.md` / `docs/guide/command-reference.md`: `show bgp rpki status` now reports `actions` + `peer-actions` |
| 4 | API/RPC added/changed? | No | no new plugin RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/rpki.md` (behavior section) |
| 6 | Has a user guide page? | Yes | `docs/guide/rpki.md` |
| 7 | Wire format changed? | No | no wire change |
| 8 | Plugin SDK/protocol changed? | No | uses existing OnConfigure |
| 9 | RFC behavior implemented/changed? | Yes | `docs/features/rfc-status.md` RFC 6811 row (note per-peer action granularity + source anchor) |
| 10 | Test infrastructure changed? | No | reuses existing rpki functional harness |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` if per-peer RPKI policy is a compared row |
| 12 | Internal architecture changed? | No | no subsystem restructure |
| 13 | Route metadata keys added/changed? | No | no new meta key |
| 14 | Prometheus counters added/changed? | No | none added |
| 15 | Registered plugin/event/command/capability changed? | No | no new registration surface |
| 16 | Changed source files referenced by doc source anchors? | Yes | grep `docs/` for anchors on `ze-rpki.yang`/`rpki_config.go`/`rpki.go` and update |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rpki.md` examples use old `policy` syntax — migrate |

## Files to Create
- `test/plugin/rpki-per-peer-action.ci` - functional test: peer-level override.
- `test/plugin/rpki-group-action.ci` - functional test: group-level inheritance.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-verify-changed` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | - |
| 8. Re-verify | - |
| 9. Repeat 6-8 | - |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | two-commit closure |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add the YANG peer/group augments (policy-only grouping) and the `atomic.Pointer` field; write failing `TestParseRPKIConfig_PerPeerOverride` and `rpki-per-peer-action.ci`.
   - Tests: `TestParseRPKIConfig_PerPeerOverride`, `rpki-per-peer-action.ci`
   - Files: `yang/ze-rpki.yang`, `rpki.go` (field), `rpki_config.go` (stub resolver)
   - Verify: schema loads; tests fail because resolver returns empty / buildDecisions ignores map
2. **Phase: Rename global** — rename `policy`->`action` and leaves in YANG + parser; migrate fixtures/tests/docs.
   - Tests: `TestParseRPKIConfig_RenamedGlobalKeys`, migrated `rpki-aspa-policy-*.ci`, AC-7 rejection test
   - Files: `yang/ze-rpki.yang`, `rpki_config.go`, all fixtures/docs in Files to Modify
   - Verify: old syntax rejected; new global syntax enforces as before
3. **Phase: Shared helper** — add `configjson.PeerRemoteIP(peerMap, groupMap)` reading connection>remote>ip (peer wins over group).
   - Tests: `TestPeerRemoteIP`
   - Files: `configjson/traverse.go`, `configjson/traverse_test.go`
   - Verify: AC-11 passes
4. **Phase: Per-peer/group resolution + NotFound enforcement** — implement ForEachPeer walk, peer>group>global per-leaf resolution keyed by `configjson.PeerRemoteIP`; swap map in startSessions; lookup in buildDecisions. Wire origin NotFound enforcement (add `originNotFoundAction` atomic, store in startSessions, reject/log in buildDecisions).
   - Tests: `TestParseRPKIConfig_GroupInheritance`, `_PeerBeatsGroup`, `_PerLeafFallback`, `_ASPAPerPeer`, `TestBuildDecisions_PerPeerAction`, `TestBuildDecisions_NotFoundEnforced`
   - Files: `rpki_config.go`, `rpki.go`
   - Verify: AC-2..AC-6, AC-14 pass; `-race` clean
5. **Phase: role bug fix + watchdog migration** — replace role's + watchdog's local `extractRemoteIP` with `configjson.PeerRemoteIP`; fix role's unrealistic unit fixtures; harden `role-otc-ingress-reject.ci`.
   - Tests: `TestExtractPeerRoleConfigs_RealShape` (fails before, passes after), `TestOTCIngressFilter_RejectsLeakRealConfig`, `TestWatchdogPeerRemoteIP_AfterMigration`, hardened `role-otc-ingress-reject.ci`
   - Files: `role/config.go`, `role/config_test.go`, `role/otc_test.go`, `watchdog/config.go`, `test/plugin/role-otc-ingress-reject.ci`
   - Verify: AC-12, AC-13 pass; full role + watchdog suites green before/after (R-6)
6. **Phase: Status display** — extend `statusCommand` with `actions` (global effective) + `peer-actions` (resolved, per-leaf source) JSON, buffer-first.
   - Tests: `TestStatusCommand_GlobalActions`, `TestStatusCommand_PerPeerActions`
   - Files: `rpki.go` (`statusCommand`)
   - Verify: AC-9, AC-10 pass; JSON asserted from `rpki-per-peer-action.ci`
7. **Functional tests** → `rpki-per-peer-action.ci` (incl. status assertion), `rpki-group-action.ci`; migrate interop `43-rpki-frr`.
8. **RFC refs** → confirm RFC 6811 comment still accurate at buildDecisions; update rfc-status ledger row.
9. **Full verification** → `make ze-verify-changed`.
10. **Complete spec** → audit tables + learned summary; two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every user story path connected end-to-end |
| Correctness | peer>group>global precedence correct; per-leaf fallback (not container replace) |
| Naming | YANG kebab-case `action`/`invalid`/`not-found`/`unknown`; string->uint8 via `aspaActionFromString` |
| Data flow | per-peer resolution in rpki plugin only; reactor unaware; keyed by remote IP matching req.peerAddr |
| Status consistency | `statusCommand` derives `actions`/`peer-actions` from the SAME atomics/map buildDecisions uses; display can never disagree with enforcement |
| Buffer-first | `statusCommand` additions use `textbuf` (`b.Str`/`b.Int`), no fmt.Sprintf, no per-call alloc |
| Registration over hardcoding | augment via plugin's own YANG module; no central switch touched |
| YANG validation | enum leaves fully constrained; no bare `type string` |
| Rule: no-layering | old `policy` leaves fully removed (no dual-support); role's + watchdog's local `extractRemoteIP` fully deleted (one shared helper) |
| Rule: fail-closed | unknown action string -> keep configured/global default, never silently "accept" |
| Shared helper genericity | `configjson.PeerRemoteIP` has no rpki/role/watchdog-specific spelling; it is generic peer traversal (belongs in configjson alongside ForEachPeer/GetCapability) |
| role fix is non-vacuous | hardened `role-otc-ingress-reject.ci` asserts the leak is actually rejected (wire or absence), NOT just `total-routes >= 0`; unit test guards regardless of gate race |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Renamed global YANG | `grep -n "action" yang/ze-rpki.yang`; `grep -c "policy" yang/ze-rpki.yang` == 0 |
| Per-peer/group augments | `grep -n "augment.*peer\|augment.*group" yang/ze-rpki.yang` (3 rows) |
| Per-peer map + lookup | `grep -n "atomic.Pointer" rpki.go`; `grep -n "peerAddr" rpki.go` in buildDecisions |
| Migrated fixtures | `grep -rL "invalid-action\|not-found-action\|unknown-action" test/ demos/` shows none left |
| Functional tests exist | `ls test/plugin/rpki-per-peer-action.ci test/plugin/rpki-group-action.ci` |
| Status shows actions | `grep -n "peer-actions\|\"actions\"" rpki.go` in `statusCommand`; `rpki-per-peer-action.ci` asserts on `peer-actions` |
| Shared helper exists | `grep -n "func PeerRemoteIP" configjson/traverse.go`; `TestPeerRemoteIP` passes |
| role uses helper, stale fn gone | `grep -c "func extractRemoteIP" role/config.go` == 0; `grep -n "PeerRemoteIP" role/config.go` |
| watchdog uses helper | `grep -c "func extractRemoteIP" watchdog/config.go` == 0; `grep -n "PeerRemoteIP" watchdog/config.go` |
| role fix proven | `TestExtractPeerRoleConfigs_RealShape` fails on pre-fix code, passes after; `TestOTCIngressFilter_RejectsLeakRealConfig` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Unrecognized action string in per-peer config must not silently weaken policy (fail to configured default, log warn) |
| Fail-closed | A peer whose override fails to parse must fall back to the (stricter) global action, never to "accept" |
| Resource | Per-peer map is bounded by peer count; built once per config reload, no unbounded growth |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read Current Behavior sources |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `not-found-action` (OriginNotFoundAction) is enforced globally today (implicit in AC-5) | It is parsed (rpki_config.go:100) but NEVER enforced: no atomic field, not stored in startSessions, never read in buildDecisions. `not-found reject` is a global no-op. Only Invalid + ASPA Invalid/Unknown are enforced. | `/ze-implement` audit (grep for OriginNotFoundAction consumers; buildDecisions rpki.go:640-685 rejects only on ValidationInvalid) | AC-5's "NotFound still reject" cannot be demonstrated without first implementing NotFound enforcement. Scope decision required (see below). |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

### Adjacent bug found: `role` plugin keys per-peer config by the wrong path (fixed in this spec)
While validating how to key RPKI's per-peer map, found that `role.extractRemoteIP`
(role/config.go:153-164) reads the flat `m["remote"]["ip"]`, but the delivered config
nests it at `connection>remote>ip` (producer: `Tree.ToMap()` keys a YANG list by entry
NAME, authradius/config.go:142-145; correct reader: watchdog/config.go:110-123; real
config: 43-rpki-frr/ze.conf:27-33). Consequence: on any normally-configured peer,
`extractRemoteIP` returns "", so `extractPeerRoleConfigs` keys its config map by peer
NAME (role/config.go:211). At runtime `OTCIngressFilter` looks up by IP
(`getFilterConfig(src.Address.String())`, otc.go:312) -> miss -> `cfg == nil` -> RFC 9234
OTC config-role filtering and `src-role` metadata are silently disabled for that peer.
Masked by (a) unit tests using an unrealistic shape (`{"peer":{"10.0.0.1":{...}}}` -
keyed by IP, no `connection` wrapper), and (b) `role-otc-ingress-reject.ci` asserting
only `total-routes >= 0`, not `== 0` (the file itself notes the positive assertion is
unreliable). Fixed IN THIS SPEC (user approved folding it in): introduce shared
`configjson.PeerRemoteIP`, migrate role + watchdog onto it, fix role's unrealistic
unit fixtures, and harden `role-otc-ingress-reject.ci` to assert the leak is actually
rejected. See AC-11..AC-13, phase 5, and R-5/R-6.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-peer resolution + lookup in the plugin, keyed by `connection>remote>ip` | Resolve upstream in config layer and deliver flattened peers | Config JSON is delivered as the nested tree keyed by peer NAME; in-plugin peer>group resolution via `configjson.ForEachPeer` is the established shape. Key on the remote IP (matching `req.peerAddr`) by reading `connection>remote>ip` like `watchdog` (role's flat helper is buggy - see Design Insights). |
| Store per-peer actions in `atomic.Pointer[map]`, swapped on reload | Mirror role's `sync.RWMutex` map | RPKI reads from ONE goroutine (validationWorker); lock-free pointer swap matches RPKI's existing lock-free action atomics and keeps buildDecisions allocation- and lock-free. |
| Per-leaf inheritance (unset leaf falls to group then global) | Container-level replace (peer `action` block fully overrides) | "keep the global option" implies partial overrides fall back; Junos-like semantics. Confirmed at design gate (A-3). |
| Hard rename, no dual-support for old `policy` syntax | Accept both old and new for a deprecation window | Nothing is shipped as stable config; dual-support adds parser branches and hides the migration. Old syntax becomes a YANG error (fail-closed). |
| Separate policy-only grouping for peer/group | Reuse `rpki-config` grouping at peer level | Reusing it would drag `cache-server`/`validation-timeout`/`aspa validation` onto every peer, which are global-only transport/enable settings. |
| `show bgp rpki status` reports resolved actions from the enforcement atomics/map | Compute a separate display-time resolution | A second resolution path could drift from enforcement; deriving display from the same source guarantees the operator sees what actually applies. |

## Known Limitations
- Per-peer action keys on remote IP, so dynamic/range peers (no static remote IP) receive only the global action (same constraint as the role plugin). Documented; not addressed here. `show bgp rpki status` `peer-actions` therefore lists statically-addressed peers only.
- ASPA per-peer action only takes effect when ASPA `validation` is enabled globally; the enable stays global.
- No per-peer Prometheus label for validation outcomes (existing counter is per-state only); out of scope.
- The per-peer map is keyed by the config `connection>remote>ip` string and looked up by the runtime `req.peerAddr` (`se.PeerAddress`) using raw string equality. For IPv4 the forms match. For IPv6, if the two sides ever diverge in canonical form the lookup would miss and fall back to the (permissive-direction) global action -- the same string-key convention role and watchdog already use. Unverified for IPv6; recommended follow-up: an IPv6 per-peer functional test. Flagged by the /ze-review gate (NOTE).

## RFC Documentation

Confirm the existing RFC 6811 Section 2/3 comment at `buildDecisions` (rpki.go:646-650) stays accurate: excluding Invalid remains an explicit operator policy choice; per-peer granularity refines *which* configured action applies but never makes exclusion automatic.

## Implementation Summary

### What Was Implemented
- YANG (`yang/ze-rpki.yang`): renamed global `policy`->`action`, leaves `invalid-action`->`invalid`, `not-found-action`->`not-found`, ASPA `unknown-action`->`unknown`; added `validation-action` typedef; added policy-only grouping `rpki-peer-policy` (no defaults, per-leaf inheritance) augmented at `/bgp:bgp/bgp:peer`, `/bgp:bgp/bgp:group/bgp:peer`, `/bgp:bgp/bgp:group`.
- `configjson.PeerRemoteIP(peerMap, groupMap)`: shared helper reading `connection>remote>ip`, peer wins over group.
- `rpki_config.go`: parse renamed global keys; `parsePeerActions` resolves peer>group>global per leaf into `map[string]peerActionSet` keyed by remote IP (with per-leaf `actionSource` for display); dynamic/no-IP peers skipped.
- `rpki.go`: added `originNotFoundAction atomic.Uint32` (NotFound now ENFORCED) and `perPeerActions atomic.Pointer[map]`; `startSessions` stores both; `buildDecisions` resolves per-route effective actions (per-peer override else global) and rejects Invalid and NotFound per action.
- `rpki_status.go` (new): `statusCommand` emits `actions` (effective global) + `peer-actions` (resolved, per-leaf source), buffer-first.
- `role` fix: `extractRemoteIP` (stale flat path) replaced with `configjson.PeerRemoteIP`; `TestExtractPeerRoleConfigs_RealShape` regression guard.
- Migrated all old-syntax fixtures/docs; added `rpki-per-peer-action.ci`, `rpki-group-action.ci`; hardened `role-otc-ingress-reject.ci` to assert the leak is actually rejected (adj-rib-in empty), verified non-flaky (5/5).

### Bugs Found/Fixed
- Origin `not-found-action` was parsed but never enforced (no atomic, never read in `buildDecisions`) -- a global no-op. Now wired (user-approved). See Mistake Log.
- `role.extractRemoteIP` read the stale flat `remote/ip` path (pre connection-container reorg), silently disabling RFC 9234 OTC config-role filtering on real configs. Fixed via `configjson.PeerRemoteIP`; the previously vacuous `role-otc-ingress-reject.ci` (asserted only `total-routes >= 0`) now asserts `== 0`.

### Documentation Updates
- `docs/guide/rpki.md`: config table renamed to `action`; added per-peer/group section + example + status note (source anchors added).
- `docs/config-reference.md`: ASPA example migrated to nested `aspa { action { invalid } }`.
- `docs/features.md`: ASPA row updated to `rpki/aspa/action/invalid`; added "RPKI Per-Peer Action" row (source anchors).
- `make ze-doc-test`: [recorded in Pre-Commit Verification].

### Deviations from Plan
- NotFound enforcement ADDED to scope (spec originally assumed it was enforced; audit found it inert; user chose "fix it too"). New AC-14, `TestBuildDecisions_NotFoundEnforced`.
- The `role` fix was committed BUNDLED with a parallel session's uncommitted local-ASN refactor (`spec-fixit-local-asn-config-key`: `role.go`/`otc.go`/`otc_test.go` + `config.go` `extractLocalASN` removal), because the files are entangled and would not compile separately. User explicitly approved bundling ("commit it part of your fix") after stopping the parallel session.
- `watchdog` migration to `configjson.PeerRemoteIP` was DROPPED (was in scope) -- watchdog already reads the correct path and is uncontested; leaving it avoids touching an unrelated plugin. `PeerRemoteIP` still has a non-test caller (rpki + role).
- buildDecisions tests placed in a new `rpki_action_test.go` (not `rpki_test.go`) and status tests in `rpki_status_test.go` -- cleaner separation; same coverage.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | migrated ASPA/coverage `.ci`; global `action` parsed (`TestParseRPKIConfigOriginAction`) | global still authoritative |
| AC-2 | Done | `TestBuildDecisions_PerPeerAction`, `rpki-per-peer-action.ci` | per-peer override accepts Invalid; other peer rejects |
| AC-3 | Done | `TestParseRPKIConfig_GroupInheritance`, `rpki-group-action.ci` | group applies to member |
| AC-4 | Done | `TestParseRPKIConfig_PeerBeatsGroup` | peer beats group |
| AC-5 | Done | `TestParseRPKIConfig_PerLeafFallback` | unset leaf inherits global |
| AC-6 | Done | `TestParseRPKIConfig_ASPAPerPeer` | per-peer ASPA action |
| AC-7 | Done | `ze config validate` rejects old `policy` (Goal Validation) | hard cutover |
| AC-8 | Done | `TestParseRPKIConfigDefaults` | RFC 6811 defaults preserved |
| AC-9 | Done | `TestStatusCommand_GlobalActions` | status `actions` object |
| AC-10 | Done | `TestStatusCommand_PerPeerActions`, `rpki-per-peer-action.ci` | per-leaf source in `peer-actions` |
| AC-11 | Done | `TestPeerRemoteIP` | connection>remote>ip, peer>group |
| AC-12 | Done | `TestExtractPeerRoleConfigs_RealShape`, hardened `role-otc-ingress-reject.ci` | role keyed by IP; leak rejected |
| AC-13 | Changed | N/A | watchdog migration dropped (see Deviations); watchdog already correct |
| AC-14 | Done | `TestBuildDecisions_NotFoundEnforced` | NotFound now enforced |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Parser unit tests (rename + per-peer) | Done | `rpki_config_test.go` | all pass |
| `TestBuildDecisions_*` | Done | `rpki_action_test.go` | per-peer + NotFound |
| `TestStatusCommand_*`, `TestActionString` | Done | `rpki_status_test.go` | pass |
| `TestPeerRemoteIP` | Done | `configjson/traverse_test.go` | pass |
| `TestExtractPeerRoleConfigs_RealShape` | Done | `role/config_test.go` | pass; fails on pre-fix code |
| Functional `.ci` (per-peer, group, aspa, parse, role-otc) | Done | `test/plugin/`, `test/parse/` | all PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| yang/ze-rpki.yang, rpki_config.go, rpki.go | Done | implemented |
| rpki_status.go, rpki_action_test.go, rpki_status_test.go | Done | created |
| configjson/traverse.go(+test) | Done | PeerRemoteIP |
| role/config.go(+test) | Done | migrated to helper |
| watchdog/config.go | Changed | migration dropped (Deviations) |
| fixtures + docs | Done | migrated |

### Audit Summary
- **Total items:** 14 ACs + tests + files
- **Done:** all ACs except AC-13
- **Partial:** none
- **Skipped:** none
- **Changed:** AC-13 (watchdog migration dropped, user-scoped to "rpki+role"); documented in Deviations

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Per-peer RPKI action overrides global | functional test | `rpki-per-peer-action.ci` PASS (Invalid route accepted via per-peer override; `peer-actions` shows invalid=accept source=peer) |
| Group-level RPKI action inherited by members | functional test | `rpki-group-action.ci` PASS (Invalid route accepted via group override; source=group) |
| Global option preserved (renamed) | functional test | `coverage-rpki.ci` PASS (new `action` syntax parses); migrated ASPA policy `.ci` PASS; interop 43-rpki-frr config migrated |
| NotFound now enforced (was inert) | unit test | `TestBuildDecisions_NotFoundEnforced` PASS (reject/log-only/accept) |
| Operator can see per-peer effective actions | unit + functional | `TestStatusCommand_GlobalActions`/`_PerPeerActions` + `rpki-per-peer-action.ci` asserts `show bgp rpki status` `peer-actions` |
| role OTC config-role filtering works on real configs | unit + functional test | `TestExtractPeerRoleConfigs_RealShape` (fails on pre-fix code) + hardened `role-otc-ingress-reject.ci` (asserts adj-rib-in empty; 5/5 stable) |
| Old syntax is a hard error (fail-closed cutover) | manual validate | `ze config validate` rejects `rpki { policy { invalid-action } }` with "unknown field in rpki: policy" |

## Review Gate

### Run 1 (initial)
Two independent reviewer agents (rpki per-peer logic; status serialization + role fix) plus mechanical pre-checks (`make ze-validate`, `audit-test-relaxation.py`). All findings NOTE-level; no BLOCKER, no ISSUE.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `actionString` renders out-of-range (>=3) as "accept" (defensive only; atomics only store validated 0/1/2, unreachable) | rpki_status.go:11-20 | acknowledged (not reachable) |
| 2 | NOTE | `statusCommand` issues independent atomic loads; a reload mid-serialization could briefly mix generations in the OUTPUT (display-only); the "never disagree" comment overstated | rpki.go:1004-1007 | fixed (comment corrected to describe the display-only window) |
| 3 | NOTE | Dynamic/no-IP peer override is dropped toward the permissive global (logged Warn); matches documented Known Limitation | rpki_config.go:283-287 | acknowledged (documented limitation) |
| 4 | NOTE | Per-peer map keyed by config `connection>remote>ip` vs runtime `req.peerAddr` via raw string equality; IPv6 canonical-form divergence would miss -> permissive fallback (unverified) | rpki.go:686 | recorded as Known Limitation + follow-up IPv6 test |
| 5 | NOTE | Reload removing cache servers leaves prior RTR sessions running (pre-existing lifecycle gap; new fields inherit, do not introduce) | rpki.go:295 | acknowledged (pre-existing, out of scope) |
| - | (mechanical) | `ze-validate` unwired-export ISSUES are all in a concurrent session's `cli/`, `web/`, `ssh/` files, none in this diff | (other session) | not this diff |
| - | (mechanical) | 1 documented `test-relax` (role config_test): `extractRemoteIP` + `extractLocalASN` removals -- removed features / replaced coverage | role/config_test.go | valid (confirm reasons) |

### Fixes applied
- NOTE 2: corrected the `statusCommand` comment (rpki.go:1004-1007) to state the display reflects enforced policy but independent loads mean a reload can briefly mix generations in output only.

### Run 2+ (re-runs until clean)
No BLOCKER/ISSUE found in Run 1, so no fix cycle was required. The only code change (comment wording) does not alter behavior; no re-review of logic needed.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

Result: 0 BLOCKER, 0 ISSUE. 5 NOTEs recorded (1 fixed, 4 acknowledged/documented). Gate satisfied.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| rpki_status.go, rpki_action_test.go, rpki_status_test.go | Yes | created under internal/component/bgp/plugins/rpki/ |
| test/plugin/rpki-per-peer-action.ci, rpki-group-action.ci | Yes | created; both PASS via `bin/ze-test bgp plugin` |
| configjson/traverse.go PeerRemoteIP | Yes | `grep -n "func PeerRemoteIP"` |
| plan/learned/1180-rpki-per-peer-action.md | Yes | created; .counter bumped to 1181 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2 | per-peer override changes outcome | `rpki-per-peer-action.ci` PASS (Invalid accepted for overriding peer) |
| AC-3 | group inheritance | `rpki-group-action.ci` PASS (source=group) |
| AC-5 | per-leaf fallback | `TestParseRPKIConfig_PerLeafFallback` PASS |
| AC-7 | old syntax rejected | `ze config validate` -> "unknown field in rpki: policy" |
| AC-10 | status peer-actions | `TestStatusCommand_PerPeerActions` + `rpki-per-peer-action.ci` assert peer-actions |
| AC-12 | role keyed by IP | `TestExtractPeerRoleConfigs_RealShape` PASS (fails on pre-fix); hardened `role-otc-ingress-reject.ci` PASS 5/5 |
| AC-14 | NotFound enforced | `TestBuildDecisions_NotFoundEnforced` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `peer X { rpki { action { invalid accept } } }` | rpki-per-peer-action.ci | Yes -- route accepted; peer-actions shows invalid=accept source=peer |
| `group G { rpki { action { invalid reject } } peer Y {} }` | rpki-group-action.ci | Yes -- route accepted via group; source=group |
| `peer P { connection { remote { ip } } role { import ... } }` | role-otc-ingress-reject.ci | Yes -- leak rejected (adj-rib-in empty), 5/5 stable |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | req.peerAddr = se.PeerAddress (rpki.go:313); functional test matches config IP |
| A-2 | confirmed | parseRPKIConfig reads `.(string)`; per-peer JSON parses |
| A-3 | confirmed | user selected per-leaf fallback |
| A-4 | confirmed | producer (Tree.ToMap keyed-by-name) + real config + watchdog reference |
| A-5 | confirmed | `ze config validate` rejects old `policy` container |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| docs/guide/rpki.md config table uses `action` syntax | validated via `ze config validate` on new syntax | Yes |
| docs/guide/rpki.md per-peer section + status note | matches parsePeerActions + statusCommand (source anchors added) | Yes |
| docs/config-reference.md ASPA example nested `aspa { action }` | matches YANG ze-rpki.yang | Yes |
| docs/features.md ASPA row + new Per-Peer Action row | source anchors to rpki_config.go / rpki.go | Yes |
| No stale source anchors on changed files | `make ze-validate` "all references valid" (doc-anchor subcheck passed; the ze-doc-test failure was .counter numbering, now fixed) | Yes |
| `make ze-doc-test` | .counter bumped to 1181 (was the only doc-test blocker attributable to this diff); LEARNED-FULL-INDEX regen deferred (whole-tree, blocked by concurrent uncommitted summaries) | Partial (see Deviations) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Architecture docs and guides updated
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments verified
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A — enums only)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rpki-per-peer-action.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rpki-per-peer-action.md`

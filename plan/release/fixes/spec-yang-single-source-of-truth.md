# Spec: yang-single-source-of-truth

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped config leaves, RPCs, and notifications validate, commit, and show while nothing reads or emits them: a config leaf commits and changes nothing (`label`/`labels` on plugin families, stale `ze-bgp-api` RPC names, missing `environment/reactor` YANG env aliases, missing `vxlan` `ttl` leaf, inert config leaves), and no build-time or `ze doctor` gate catches unconsumed leaves or producer-less notifications. These are defects in shipped config and API surfaces, so they belong before the first release. |
| RFC gaps | - |
| Depends | Split on 2026-11-14 (A, B, C moved to `spec-review-typed-config-decode.md`, `spec-validation-walks-every-schema-section.md`, `spec-yang-defaults-applied-once.md`) and on 2026-11-26 per owner decision T-1 (2026-11-24: Piece D moved to `plan/release/fixes/spec-yang-command-arguments-by-leaf-name.md`, Piece F moved to `plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md`, this file retains Piece E so existing cross-references stay valid). Companion owners for T-4 rows: `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md` (`software-version/mode`), `plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md` (`bgp/rib/adj-rib-*`), `plan/post-release/features/spec-bgp-api-notification-producers.md` (the 7 `ze-bgp-api` notifications), `plan/post-release/features/spec-ipsec-remote-access.md` (`remote-access` subtree). Related: `plan/release/fixes/spec-peer-leaves-the-peer-parser-never-reads.md`, `plan/release/fixes/spec-plugin-external-yang-at-startup.md`. |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| ID | Decision |
|----|----------|
| T-1 (split vs. single spec) | Split into **three separate specs**: Piece D (`plan/release/fixes/spec-yang-command-arguments-by-leaf-name.md`), Piece E (this spec, unused config leaf/RPC/notification gate), and Piece F (`plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md`) so smaller pieces D and F land independently of E. |
| T-2 & T-3 (leaf-read tracking & typed SDK config accessors) | Choose T-2 option 3 + T-3 add typed accessors: add schema-bound typed config accessors (`sdk.ConfigReader`) to `pkg/plugin/sdk` so plugins stop hand-parsing raw JSON maps and read leaf paths are recorded accurately without maintaining manual leaf lists. Keep the Stage 2 JSON-string wire payload backward-compatible, and add an optional Stage 1 `read-leaf-paths` field so external plugins can report read paths to `ze doctor`. |
| T-5 (temporary allowlist on leaf gate) | Allow a temporary recorded allowlist (like `claims/allowlist.json`, each entry naming `path`, `reason`, and `owner` spec/symbol) so the build gate turns on immediately and blocks new unused declarations while deferred T-4 rows are resolved. |
| T-4 (per-row disposition, verified against current tree) | `rpki/cache-server/preference` and `update/watchdog/withdraw` are already implemented and tested (retired into Existing Mechanisms); `remote-access` is owned by `plan/post-release/features/spec-ipsec-remote-access.md`; `bgp/rib/adj-rib-*` is owned by `plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md`; `software-version/mode` (`require` and `refuse`) is owned solely by `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`; the 7 `ze-bgp-api` notifications sit on the temporary allowlist citing `plan/post-release/features/spec-bgp-api-notification-producers.md`; this spec fixes the 3 stale `ze-bgp-api` RPC names (`peer-show`, `peer-show-capabilities`, `peer-show-statistics`), the `label`/`labels` plugin-family bug, the `environment/reactor` YANG env aliases, the missing `vxlan` `ttl` leaf (YANG + netlink wiring), and the remaining owned inert leaves. |

## Task

**Problem.** Ze's existing config claim gate (`internal/component/config/claims/claims.go`) judges delivery only at config-root granularity (`bgp`, `interface`, `ddos`), while `./le config unread-leaves report` (`internal/le/config/unreadleaves/yangleafmentions.go`) is an advisory string-literal heuristic that exits 0. Because nothing enforces leaf-level consumption or YANG notification production:

1. Config leaves validate, commit, and display while no consumer reads them (`label`/`labels` dropped in `bgp_routes_plugin.go`, 7 `environment/reactor` leaves lacking YANG-named env aliases, inert leaves across plugins, and `vxlan` `ttl` missing from `ze-iface-conf.yang` and unwired in `buildVxlan`).
2. `ze-bgp-api.yang` still declares 3 stale RPC names (`peer-show`, `peer-show-capabilities`, `peer-show-statistics`) that no handler serves under those names, and 7 notifications (`peer-state-change`, `route-received`, `route-update-sent`, `eor-received`, `graceful-restart-state`, `session-established`, `session-closed`) with no producer, while `./le doc yang-contract command-contract` (`internal/le/doc/yangcontract/contract.go`) checks RPCs only and ignores notifications.
3. Plugins hand-parse raw `map[string]any` JSON strings from Stage 2 `rpc.ConfigSection` with no shared typed config accessor, so read leaf paths cannot be measured accurately without hand-maintained lists.

**Goal.** Implement Piece E per Thomas's T-1..T-5 decisions:

1. Add schema-bound typed config accessors (`sdk.ConfigReader` in `pkg/plugin/sdk/config_reader.go`) over the existing Stage 2 `rpc.ConfigSection` JSON-string wire payload so parsers read typed leaves with automatic path recording (and optional Stage 1 `read-leaf-paths` on `rpc.DeclareRegistrationInput` validated in `stage1Checks` for external plugins).
2. Extend `internal/component/config/claims` (`AuditLeaves`, `leaf_allowlist.json`) and `internal/component/plugin/all/config_claims_test.go` (`TestConfigSchemaLeavesConsumed`, `TestConfigLeafPhantomReads`) to refuse any writable config leaf with no recorded consumer and any phantom/stale allowlist or read-path entry; repoint `./le config unread-leaves report` onto `claims.AuditLeaves`.
3. Extend `ze doctor` (`internal/component/doctor/checks_config_claims.go`, code `doctor-config-leaf-unread`) to report configured leaves that no running consumer reads (compiled out, failed to load, or external plugin declaring none).
4. Extend `./le doc yang-contract command-contract` (`internal/le/doc/yangcontract/contract.go`) with a notification producer check plus temporary recorded allowlist (`notification_allowlist.json`) pointing the 7 `ze-bgp-api` notifications to `plan/post-release/features/spec-bgp-api-notification-producers.md`.
5. Fix the owned T-4 surface defects: rename the 3 stale `ze-bgp-api.yang` RPCs (`peer-show` → `peer-detail`, `peer-show-capabilities` → `peer-capabilities`, `peer-show-statistics` → `peer-statistics`), refuse configured `attribute { label / labels }` on all 6 `InProcessConfigRouteParser` plugin families in `internal/component/bgp/config/bgp_routes_plugin.go` with `opererr` (none of the 6 plugin families accept attribute labels; labeled unicast and MPLS VPN use the standard prefix path, and EVPN/VPLS take labels inline in `nlri`), register and plumb the 7 YANG-named `environment/reactor` env aliases in `internal/component/config/environment.go` and `internal/component/config/apply_env.go`, add `leaf ttl` (`ze:backend "kernel";`, `default 64;`) to `vxlan` in `internal/component/iface/yang/ze-iface-conf.yang` and wire `spec.TTLSet`/`spec.TTL` into `buildVxlan` (`internal/plugins/iface/netlink/tunnel_linux.go`), and resolve or allowlist every remaining owned inert leaf with its reason and owner spec.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md`, "Claim Completeness Gate"
  → Decision: delivery is judged at build time by `TestConfigSchemaRootsClaimed` and `TestConfigRootsPhantomClaims`, and at run time by `ze doctor` (`doctor-config-root-unclaimed`) for plugins compiled out, failed to load, or external
  → Constraint: promoting leaf coverage to a build gate requires exact path recording through typed accessors rather than string-literal scanning (`unreadleaves`), with explicit allowlist entries carrying `path`, `reason`, and `owner`
- [ ] `docs/architecture/api/process-protocol.md`, "5-Stage Startup Protocol", `docs/plugin-development/protocol.md`, "Stage 2", and `docs/architecture/plugin/plugin-system.md`, "One table of Stage 1 checks, and the conformance checker"
  → Decision: `pkg/plugin/rpc` `ConfigSection` keeps every Stage 2 leaf value as a JSON string for cross-language external plugin compatibility; `DeclareRegistrationInput` (`sdk.Registration`) gains an optional `read-leaf-paths` field (`omitempty`) validated in `stage1Checks` (`internal/component/plugin/server/stage1_checks.go`)
  → Constraint: changing `pkg/plugin/rpc` or `pkg/plugin/sdk` is a published contract change that must keep non-Go external plugins working and update `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`, and `docs/architecture/plugin-manager-wiring.md` in the same change
- [ ] `docs/architecture/doctor-and-health-checks.md`, "The runtime phase"
  → Constraint: `doctor-config-leaf-unread` reports configured leaves with no running reader (including external plugins that declare no `read-leaf-paths`, never assuming an undeclared external plugin reads a leaf)
- [ ] `docs/architecture/update-building.md` and `internal/component/bgp/config/bgp_routes_plugin.go`
  → Constraint: `ConfigRouteRequest` carries only NLRI tokens, next hop, and extended communities; none of the 6 `InProcessConfigRouteParser` families (`flowspec`, `vpls`, `mvpn`, `evpn`, `srpolicy`, `mup`) accept `attribute { label / labels }` (`ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, `ipv6/mpls-vpn` use the standard prefix path in `bgp_routes.go`, while EVPN/VPLS read inline NLRI label tokens), so `buildConfigRouteRequest` refuses `attribute { label / labels }` with an `opererr` naming the family and fix rather than silently dropping it
- [ ] `docs/architecture/config/environment-block.md` and `docs/architecture/config/environment.md`
  → Constraint: reactor tuning lives under YANG `environment/reactor/<leaf>` (`environment { reactor { ... } }`); all 7 promoted leaves register canonical `ze.bgp.reactor.<leaf>` env vars in `internal/component/config/environment.go` beside legacy `ze.fwd.*`/`ze.buf.*` keys and plumb cleanly in `internal/component/config/apply_env.go`
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/forwarding.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md`
  → Constraint: `internal/component/config/claims` holds claim semantics only and imports no plugin registry or BGP packages; `internal/component/doctor` and `internal/component/plugin/all` supply the live inventories; VXLAN `ttl` carries `ze:backend "kernel";` and `default 64;` so VPP (whose `VxlanAddDelTunnelV3` has no TTL field) refuses it exact-or-reject at verify

**Key insights:**
- Keeping Stage 2 `rpc.ConfigSection.Data` as JSON strings while layering `sdk.ConfigReader` on top in `pkg/plugin/sdk` delivers T-2 option 3 + T-3 without breaking third-party non-Go plugins or changing the Stage 2 JSON wire format.
- Verified across all 6 `InProcessConfigRouteParser` families (`flowspec`, `vpls`, `mvpn`, `evpn`, `srpolicy`, `mup`): none consumes `attribute { label / labels }` (`flowspec`, `mvpn`, `mup`, `srpolicy` have no NLRI label field; `vpls` uses inline `label-base` and `evpn` uses per-route-type inline `label`/`label1`/`label2` tokens in `req.Content`; `ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, `ipv6/mpls-vpn` take the standard prefix path in `bgp_routes.go:178-220`). Refusing `attribute { label / labels }` in `buildConfigRouteRequest` keeps `registry.ConfigRouteRequest` unchanged and agrees with `docs/architecture/update-building.md` and `docs/architecture/plugin/plugin-system.md`.
- Several 2026-09-03/04/05 journal items listed in the original 2026-10-23 spec are already implemented and tested in HEAD (`rpki/cache-server/preference`, `update/watchdog/withdraw`, `ze-plugin:session-ping`, `ddos/flowtriq/api-key`, `bfd/enabled`, `static/table/route/tag`, `fib/kernel/{flush-on-stop,sweep-delay}`, `ospf/reference-bandwidth`, `bmp/route-action`, `isis` per-level timers); retiring them into `## Existing Mechanisms` keeps the implementation focused on genuine remaining defects.

## Current Behavior (MANDATORY)

**Source files read (verified at HEAD):**
- [ ] `internal/component/config/claims/claims.go` and `allowlist.json` - `Audit` and `AuditConfigured` judge config delivery at subtree/root granularity only; no leaf-level `AuditLeaves` or `leaf_allowlist.json` exists
- [ ] `internal/component/plugin/all/config_claims_test.go` - `TestConfigSchemaRootsClaimed` and `TestConfigRootsPhantomClaims` check roots only, not leaves
- [ ] `internal/le/config/unreadleaves/yangleafmentions.go` - scans Go source for string literals matching kebab leaf names and always exits 0
- [ ] `internal/component/doctor/checks_config_claims.go` - `checkConfigClaims` emits `doctor-config-root-unclaimed` for unclaimed roots only; no leaf-level check exists
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` (line 340) carries `WantsConfig` (root names) and no `ReadLeafPaths`; `ConfigSection` (line 994) carries `Root` and JSON string `Data`
- [ ] `internal/component/plugin/server/stage1_checks.go` - `stage1Checks` holds the ordered Stage 1 validation table shared by the daemon and `JudgeDeclaration`, with no check yet for `ReadLeafPaths`
- [ ] `pkg/plugin/sdk/config_tx.go` - provides transaction lifecycle helpers (`ConfigTx`), with no schema-bound typed `ConfigReader` accessor that records read leaf paths
- [ ] `internal/le/doc/yangcontract/contract.go` - `contractSatisfied` checks orphan YANG command nodes, handlers, local handlers, and RPCs, but performs no check on YANG `notification` declarations
- [ ] `internal/component/bgp/yang/ze-bgp-api.yang` and `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - `ze-bgp-api.yang` still declares 3 stale RPC names (`peer-show` line 249, `peer-show-capabilities` line 306, `peer-show-statistics` line 340, referenced by `ze-peer-cmd.yang:97,116,133`, `schema_test.go:475`, `yang_test.go:131-132`) and 7 notifications whose descriptions state no producer emits them
- [ ] `internal/component/bgp/config/bgp_routes_plugin.go`, `internal/component/bgp/config/bgp_routes.go`, and all 6 `internal/component/bgp/plugins/nlri/*/config.go` (`flowspec`, `vpls`, `mvpn`, `evpn`, `srpolicy`, `mup`) - `buildConfigRouteRequest` (lines 46-80) calls `applyAttributesFromTree` and `ParseRouteAttributes` (parsing `sr.Label`/`sr.Labels` into `parsed.Labels`) and silently discards them; none of the 6 plugin-registered families accepts `attribute { label / labels }` (`ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, `ipv6/mpls-vpn` use the standard prefix path in `bgp_routes.go:178-220`)
- [ ] `internal/component/config/environment.go` and `internal/component/config/apply_env.go` - `environment.go:108-120` registers the 7 promoted `environment/reactor` env vars under legacy keys only (`ze.fwd.chan.size`, `ze.fwd.batch.limit`, `ze.fwd.pool.maxbytes`, `ze.fwd.pool.headroom`, `ze.fwd.teardown.grace`, `ze.buf.read.size`, `ze.buf.write.size`) with no `ze.bgp.reactor.<leaf>` aliases, and `envPlumbingTable` (`apply_env.go:46-52`) plumbs `environment/reactor/<leaf>` only to those legacy keys
- [ ] `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/config.go`, `internal/plugins/iface/netlink/tunnel_linux.go`, and `internal/plugins/iface/vpp/vxlan.go` - `parseTunnelLeaves` (`config.go:863-870`) already parses `ttl` into `spec.TTL`/`spec.TTLSet`, but `container vxlan` (`ze-iface-conf.yang:1653`) omits `leaf ttl`, `buildVxlan` (`tunnel_linux.go:280-301`) does not copy `spec.TTLSet`/`spec.TTL` onto `netlink.Vxlan.TTL`, and VPP `VxlanAddDelTunnelV3` (`vxlan.go:46-53`) carries no TTL field (so VPP requires `ze:backend "kernel";` exact-or-refuse)

**Behavior to preserve:**
- Existing Stage 2 `rpc.ConfigSection` JSON-string payload format stays backward-compatible for all external plugins in Go and other languages.
- All valid existing configs continue loading with identical effect, except any genuinely inert leaf deleted under T-4 (which fails validation with an operator-facing fix) or `attribute { label / labels }` configured on one of the 6 plugin-registered families (which is now refused with an `opererr` instead of silently dropped). Standard labeled families (`ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, `ipv6/mpls-vpn`) on the prefix path in `bgp_routes.go` stay untouched.
- Root-level claim checks (`TestConfigSchemaRootsClaimed`, `TestConfigRootsPhantomClaims`, `doctor-config-root-unclaimed`) continue running unchanged beside the new leaf-level checks.

**Behavior to change:**
- `pkg/plugin/sdk` gains `ConfigReader` typed accessors recording every schema-relative leaf path read, `rpc.DeclareRegistrationInput` gains optional `ReadLeafPaths` (`json:"read-leaf-paths,omitempty"`), and `stage1Checks` validates `ReadLeafPaths` bounds and control characters on both the daemon and `JudgeDeclaration`.
- `TestConfigSchemaLeavesConsumed` and `TestConfigLeafPhantomReads` refuse any writable config leaf with no recorded reader (unless explicitly listed in `leaf_allowlist.json` with `path`, `reason`, and `owner`) and any phantom read path or stale allowlist entry.
- `ze doctor` reports configured unread leaves under `doctor-config-leaf-unread` (covering compiled-out plugins, failed plugins, and external plugins declaring no `read-leaf-paths`).
- `./le doc yang-contract command-contract` refuses any YANG `notification` with no registered producer (unless listed in `notification_allowlist.json` with `reason` and `owner` spec).
- The 3 stale `ze-bgp-api` RPC names, `attribute { label / labels }` refusal on plugin-family config routes, the 7 `environment/reactor` env aliases, the `vxlan` `ttl` leaf (`ze:backend "kernel";`, `default 64;`) + `buildVxlan` netlink wiring, and remaining owned inert leaves are repaired.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Config root claims gate | `internal/component/config/claims/claims.go` `Audit`, `AuditConfigured`; `allowlist.json`; `internal/component/plugin/all/config_claims_test.go` | Verifies every writable config root reaches a plugin or hub handler, refuses phantom claims and stale/reasonless allowlist entries | Extends: adds leaf-granularity `AuditLeaves`, `AuditConfiguredLeaves`, `leaf_allowlist.json`, `TestConfigSchemaLeavesConsumed`, and `TestConfigLeafPhantomReads` |
| Unread-leaves report | `internal/le/config/unreadleaves/yangleafmentions.go` (`./le config unread-leaves report`) | Advisory string-literal grep heuristic that always exits 0 | Replaces: repoints onto `claims.AuditLeaves` so the CLI report and the build gate share one exact signal |
| Doctor config claim check | `internal/component/doctor/checks_config_claims.go` `checkConfigClaims` (`doctor-config-root-unclaimed`) | Reports configured roots delivered to no running plugin or handler | Extends: adds `doctor-config-leaf-unread` for configured leaves read by no running consumer (compiled-out, failed, or external) |
| Plugin config delivery, transactions & Stage 1 checks | `pkg/plugin/rpc/types.go` `DeclareRegistrationInput`, `ConfigSection`; `pkg/plugin/sdk/config_tx.go` `ConfigTx`; `internal/component/plugin/server/stage1_checks.go` `stage1Checks` | Delivers root-scoped JSON-string sections at Stage 2, validates Stage 1 declarations in one table shared by the daemon and `JudgeDeclaration`, and manages verify/apply/rollback transactions | Extends: adds `sdk.ConfigReader` typed leaf accessors with path recording over `ConfigSection`, optional `ReadLeafPaths` on `DeclareRegistrationInput`, and a `read-leaf-paths` entry in `stage1Checks` |
| VXLAN tunnel parsing, netlink builder & VPP backend | `internal/component/iface/config.go` `parseTunnelLeaves`; `internal/plugins/iface/netlink/tunnel_linux.go` `buildVxlan`; `internal/plugins/iface/vpp/vxlan.go` `createVxlanTunnel` | `parseTunnelLeaves` already parses `ttl` into `spec.TTL`/`spec.TTLSet`; `buildVxlan` builds `netlink.Vxlan` without setting `TTL`; VPP `VxlanAddDelTunnelV3` has no TTL field | Extends: adds `leaf ttl` (`ze:backend "kernel";`, `default 64;`) to `container vxlan` in `ze-iface-conf.yang` and wires `spec.TTLSet`/`spec.TTL` into `buildVxlan` so kernel sets `netlink.Vxlan.TTL` while VPP refuses `vxlan/ttl` at verify |
| Plugin-family config route dispatcher | `internal/component/bgp/config/bgp_routes_plugin.go` `buildConfigRouteRequest`; `internal/component/plugin/registry/registry.go` `ConfigRouteRequest` | Encodes generic attributes centrally and passes only NLRI tokens, next hop, and extended communities to the 6 `InProcessConfigRouteParser` families; currently drops `sr.Label`/`sr.Labels` | Extends: refuses configured `attribute { label / labels }` in `buildConfigRouteRequest` with `opererr` naming the family and fix while preserving `ConfigRouteRequest` unchanged |
| Reactor environment variable registration & plumbing | `internal/component/config/environment.go`; `internal/component/config/apply_env.go` `envPlumbingTable` | Registers and plumbs the 7 `environment/reactor` forward/buffer leaves under legacy `ze.fwd.*`/`ze.buf.*` keys only | Extends: registers the 7 canonical `ze.bgp.reactor.<leaf>` keys with legacy `Aliases` in `environment.go` and plumbs them in `apply_env.go` with OS-env precedence over either spelling |
| YANG command contract gate | `internal/le/doc/yangcontract/contract.go` `contractSatisfied` (`./le doc yang-contract command-contract`) | Refuses orphan YANG command nodes, handlers, local handlers, and RPCs | Extends: adds YANG `notification` producer checking with temporary recorded `notification_allowlist.json` |
| Already-landed T-4 fixes in HEAD | `rpki/cache-server/preference` (`rpki/rpki.go` `startSessions`), `update/watchdog/withdraw` (`eb084a4b99`), `ze-plugin:session-ping` (fixed 2026-09-15), `ddos/flowtriq/api-key` + `bfd/enabled` + `static/table/route/tag` + 3 ddos timers, `fib/kernel/{flush-on-stop,sweep-delay}`, `ospf/reference-bandwidth`, `bmp/route-action` (`fa74dc4c00`), `isis` timers | Already wired and tested at HEAD (`update/watchdog/withdraw` row 2026-09-03 already marked `fixed:` in journal) | Does not apply (already fixed): retired from open work; remaining open journal rows for already-fixed items (`rpki/cache-server/preference`, `ze-plugin:session-ping`, `ddos/flowtriq/api-key` row) verified and flipped to `fixed:` at closure |
| Sibling pieces D, F, and companion T-4 specs | `spec-yang-command-arguments-by-leaf-name.md` (D), `spec-yang-enum-vocabulary-agreement-gate.md` (F), `spec-software-version-mode-implements-two-of-four.md` (`software-version/mode`), `spec-bgp-rib-state-subtree-has-no-producer.md` (`adj-rib-*`), `spec-bgp-api-notification-producers.md` (7 notifications), `spec-ipsec-remote-access.md` (`remote-access`) | Own command argument binding, enum agreement, `software-version/mode`, `adj-rib-*`, notification producers, and IPsec `remote-access` | Does not apply: owned by those specs and cited as owners in temporary allowlist entries where deferred |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads; the door covering `internal/component/doctor/checks_config_claims.go` | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | Enforces that every writable YANG leaf has a real consumer or explicit allowlisted owner, every YANG RPC/notification has a handler/producer, and `vxlan` `ttl` carries `ze:backend "kernel";` so VPP refuses it at verify instead of silently ignoring it |
| `docs/architecture/api/ipc_protocol.md` | Overview | the // Design: page of `pkg/plugin/rpc/types.go`; anchored to `types.go`, `startup.go`, `ze-bgp-api.yang` | - | silent | Covers newline-framed IPC wire framing; adding optional `read-leaf-paths` (`omitempty`) to Stage 1 JSON keeps framing unchanged |
| `docs/architecture/api/commands.md` | Overview | the // Design: page of `types.go`, `startup.go`; anchored to `ze-bgp-api.yang`, `ze-peer-cmd.yang` | - | silent | Covers verb-first command dispatch; renaming the 3 stale `ze-bgp-api.yang` RPCs aligns the API module with existing live command paths |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers bus participant wiring; unaffected by optional Stage 1 `read-leaf-paths` and `sdk.ConfigReader` |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `types.go`; the // Design: page of `startup.go`; a page this spec modifies | "The protocol is the same for all invocation modes (internal goroutine, external subprocess)." | agrees | Optional Stage 1 `read-leaf-paths` and `sdk.ConfigReader` over unchanged Stage 2 JSON strings work identically for internal and forked plugins, and this page is updated |
| `docs/architecture/api/wire-format.md` | Transport | anchored to `types.go`, `contract.go`, `ze-bgp-api.yang` | - | silent | Covers transport wire format; unchanged by optional JSON `read-leaf-paths` or notification contract checking |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers config apply ordering across plugins; unaffected by `sdk.ConfigReader` leaf recording |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `pkg/plugin/rpc/types.go`, `internal/component/plugin/server/startup.go` | - | silent | Covers verify/apply/rollback config transactions; `sdk.ConfigReader` reads inside existing callbacks without changing transaction phases |
| `docs/architecture/core-design.md` | Command engine and composition roots | anchored to `types.go`; the // Design: page of `internal/le/doc/yangcontract/contract.go` | - | silent | Covers core component boundaries and le/ze personality split; `yangcontract` notification checking stays inside `internal/le` |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `types.go`, `startup.go`; a page this spec modifies | "Some readiness facts exist only in the running daemon" | agrees | `doctor-config-leaf-unread` reports configured leaves unread by running/external plugins and is documented in this page |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers firewall IRR prefix sets; untouched by Stage 1 `read-leaf-paths` |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers forked route installation RPC; untouched by Stage 1 `read-leaf-paths` |
| `docs/architecture/plugin-manager-wiring.md` | Stage 1 Registration: Config Transaction Fields | anchored to `pkg/plugin/rpc/types.go`; a page this spec modifies | "Stage 1 (`declare-registration`) includes fields for the config transaction protocol:" | agrees | Optional Stage 1 `read-leaf-paths` (`[]string`, `omitempty`) sits beside `wants-config` on `DeclareRegistrationInput` and is documented in this section's table |
| `docs/architecture/plugin/plugin-system.md` | One table of Stage 1 checks, and the conformance checker | anchored to `pkg/plugin/rpc/types.go`, `internal/component/plugin/server/startup.go`; the // Design: page of `internal/component/plugin/server/stage1_checks.go` | "The checks a Stage 1 declaration must pass are one ordered table, `stage1Checks` in `internal/component/plugin/server/stage1_checks.go`." | agrees | Adds the `read-leaf-paths` count/length/control-character check to `stage1Checks` (so daemon and `JudgeDeclaration` agree), keeps `InProcessConfigRouteParser` unchanged (NLRI tokens, next hop, extended communities), and keeps `sdk.ConfigReader` generic |
| `docs/architecture/system-architecture.md` | Overview | anchored to `pkg/plugin/rpc/types.go` | - | silent | High-level system architecture; untouched by leaf-level config claim checks |
| `docs/architecture/update-cache.md` | Purpose | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers BGP UPDATE cache consumers; untouched by Stage 1 `read-leaf-paths` |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers NLRI wire encoding; untouched by Stage 1 `read-leaf-paths` |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `pkg/plugin/rpc/types.go`, `startup.go`, `config_claims_test.go`, `schema_test.go`, `pkg/plugin/sdk/config_reader.go` | "The SDK stays generic." | agrees | `sdk.ConfigReader` is a generic schema-bound reader over Stage 2 JSON-string sections with zero plugin-specific types or imports |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers CLI tab completion; unaffected by storing `ExternalReadLeafPaths` at Stage 1 |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers nft table ownership; untouched by Stage 1 `read-leaf-paths` |
| `docs/architecture/hub-architecture.md` | Overview | anchored to `startup.go`, `checks_config_claims.go`, `apply_env.go` | - | silent | Covers daemon boot orchestration; untouched by leaf claim diagnostics or env aliases |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers IKEv2 engine; `remote-access` stays allowlisted under `spec-ipsec-remote-access.md` per T-4/T-5 |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers PPPoE client; untouched by Stage 1 `read-leaf-paths` |
| `docs/architecture/config/yang-config-design.md` | Claim Completeness Gate | the // Design: page of `claims.go`, `yangleafmentions.go`, `checks_config_claims.go`; anchored to `config_claims_test.go`, `ze-bgp-api.yang`, `ze-iface-conf.yang`; a page this spec modifies | "Validation says a config value is well formed." | agrees | Extends this section to document leaf-granularity `claims.AuditLeaves`, `leaf_allowlist.json`, `sdk.ConfigReader`, and `doctor-config-leaf-unread` |
| `ai/doors/config-yang.md` | Who depends on it from outside | the door covering `claims.go`, `apply_env.go`, `leaf_allowlist.json`; a page this spec modifies | "Its config at Stage 2 as `rpc.ConfigSection` (a root plus a JSON body), every value a JSON string, a removed root as `{}`" | agrees | Preserves the Stage 2 JSON-string wire contract while layering `sdk.ConfigReader` on top, adds leaf-level claims, and updates this door's Gates/Traps |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `yangleafmentions.go`, `contract.go`, `notification_allowlist.json`, `test/plugin/doctor-config-leaf-unread.ci` | "`le` is allowed to link the product root `internal/component/plugin/all`" | agrees | `unreadleaves` and `yangcontract` stay inside `internal/le` reading product registries and YANG modules in the allowed direction |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `internal/component/bgp/yang/ze-bgp-api.yang` | - | silent | Covers UPDATE text syntax; renaming 3 peer inspection RPCs in `ze-bgp-api.yang` does not touch UPDATE syntax |
| `ai/doors/bgp-engine.md` | Where it sits | the door covering `ze-bgp-api.yang`, `bgp_routes_plugin.go`, `bgp_routes_plugin_test.go` | "The engine speaks the protocol and holds no policy." | agrees | Refuses configured `attribute { label / labels }` on all 6 `InProcessConfigRouteParser` plugin families with `opererr` without adding plugin imports to the engine or changing `ConfigRouteRequest` |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` | - | silent | Updating `ze:rpc` targets on `show bgp peer detail/capabilities/statistics` preserves all CLI verbs and paths |
| `ai/doors/bgp-plugins.md` | Where it sits | the door covering `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` | "Every one can run in another process." | agrees | Updating `ze:rpc` pointers on `ze-peer-cmd.yang` aligns `-cmd` with `ze-bgp-api.yang` without changing process boundaries |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/ipc/yang_test.go` | "Never import upward." | agrees | Updating expected RPC names in `internal/core/ipc/yang_test.go` adds zero upward imports |
| `docs/architecture/update-building.md` | Path 1: Build Path (Local Origination) | the // Design: page of `internal/component/bgp/config/bgp_routes_plugin.go`; anchored to `bgp_routes_plugin.go`, `bgp_routes_plugin_test.go`; a page this spec modifies | "The plugin receives only the NLRI tokens, the next hop and the extended communities, which a family reads as input (FlowSpec actions, EVPN's merged set). The load refuses a route whose plugin does not emit configured extended communities, an attribute both halves wrote (RFC 4271 Section 6.3), and a generic AS_PATH holding a set (RFC 9774 Section 3), so no configured attribute is lost in silence." | agrees | Preserves `ConfigRouteRequest` (only NLRI tokens, next hop, extended communities) because none of the 6 plugin families accepts `attribute { label / labels }`, adds an `opererr` refusal in `buildConfigRouteRequest` so configured attribute labels are never lost in silence, and updates this paragraph |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | the // Design: page of `internal/component/config/apply_env.go` | - | silent | Covers config file grammar; adding canonical `ze.bgp.reactor.<leaf>` env aliases for `environment/reactor` in `environment.go` and `apply_env.go` keeps config file syntax unchanged |
| `docs/architecture/config/environment-block.md` | Sections | anchored to `internal/component/config/apply_env.go`; a page this spec modifies | "`ze.bgp.reactor.*` / `ze.fwd.*` / `ze.buf.*`" | agrees | Registers the 7 missing canonical `ze.bgp.reactor.<leaf>` env aliases in `environment.go` and plumbs `environment/reactor/<leaf>` (`environment { reactor { ... } }`) in `apply_env.go`, updating this table to reflect that all 11 `reactor` leaves have canonical `ze.bgp.reactor.*` keys |
| `docs/architecture/config/environment.md` | Overview | anchored to `internal/component/config/apply_env.go`, `internal/component/config/environment.go`; a page this spec modifies | "Each YANG `environment/<section>/<option>` leaf also has a matching env var so the operator can override it at runtime." | agrees | Registers and honors the 7 missing canonical `ze.bgp.reactor.<leaf>` env aliases beside legacy short keys for `environment/reactor/<leaf>` and documents them on this page |
| `docs/architecture/diagnostics/path-mtu.md` | The module is removable | anchored to `internal/component/config/apply_env.go` | - | silent | Covers `show mtu`; untouched by `environment/reactor` env aliases |
| `docs/architecture/iface/management.md` | Two layers, not one flat list | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | - | silent | Covers interface management layering; adding `leaf ttl` (`ze:backend "kernel";`, `default 64;`) to `vxlan` in `ze-iface-conf.yang` and wiring `buildVxlan` stays within the existing tunnel layer |
| `docs/architecture/iface/offload.md` | Absence means "keep the OS default" | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | - | silent | Covers interface offload flags; untouched by adding `vxlan` `ttl` |
| `docs/architecture/iface/vlan-qos-map.md` | Decision: two maps, because the kernel has two | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | - | silent | Covers VLAN QoS maps; untouched by adding `vxlan` `ttl` |
| `ai/doors/forwarding.md` | Who depends on it from outside | the door covering `internal/component/iface/yang/ze-iface-conf.yang`, `vxlan_ttl_test.go`, `internal/plugins/iface/netlink/tunnel_linux.go`, `tunnel_vxlan_build_linux_test.go` | "The YANG schema. A node a backend cannot implement carries `ze:backend`, and the commit refuses it with the YANG path (`core-design.md`, section 14a)" | agrees | Exposes `vxlan` `leaf ttl` with `ze:backend "kernel";` and `default 64;` (matching sibling IPv4-underlay tunnels) so VPP (whose `VxlanAddDelTunnelV3` has no TTL field) refuses it exact-or-reject at verify, and wires `spec.TTLSet`/`spec.TTL` into `buildVxlan` (`netlink.Vxlan.TTL`) |
| `docs/plugin-development/protocol.md` | Wire Format | a page this spec modifies | "Every message is a single newline-terminated line:" | agrees | Preserves newline-framed Stage 1/Stage 2 JSON wire format, adds optional `read-leaf-paths` (`omitempty`) to Stage 1, and documents `sdk.ConfigReader` on this page |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Does not add scope, presentation, or format command arguments |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not modify pipe operation overrides |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not alter pipe behavior |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` config rendering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Does not touch bus participant registration |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not touch `send > to` routing |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not touch `receive > from` routing |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not touch bus allow-lists |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not touch bus filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Does not touch bus arbitration |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not touch bus defaults display |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Optional `read-leaf-paths` stays on existing Stage 1 `declare-registration` without adding an unauthorized engine operation |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not touch event delivery queues |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not alter kernel/VPP feature gating |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not alter data-plane mismatch verdicts |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Unrelated to VPP firewall verification |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch data-plane selector naming |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch egress next-hop policy |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch export chain next-hop authoring |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch `policychain.Chain` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch export chain folding |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch `policychain.Sets.NextHop` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch route origination next-hop handling |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied filter publication |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Unread leaves are judged at build time and reported by `ze doctor` (`doctor-config-leaf-unread`), never refusing `ze start` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Consistent with startup availability: `ze doctor` reports unread configured leaves at runtime without preventing boot |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Optional `read-leaf-paths` is carried inside the same pure Stage 1 declaration emitted in query mode |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not touch FlowSpec redirect next-hop emission |
| `docs/features/interfaces.md` | Outer-header TTL | the // Design: page of `internal/plugins/iface/netlink/tunnel_linux.go`; a page this spec modifies | "64 is the default because inherit blackholes a multi-hop underlay." | agrees | Brings `vxlan` into exact agreement with this section (`ze:backend "kernel";`, `default 64;`, wired into `buildVxlan`, refused on VPP at verify) and updates the `vxlan` row and anchors on this page |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | Does not touch `ze init` device discovery |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | Does not touch `ze init --confirm` prompting |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build-time gate: `TestConfigSchemaLeavesConsumed` and `TestConfigLeafPhantomReads` in `internal/component/plugin/all/config_claims_test.go`, `./le config unread-leaves report`, and `./le doc yang-contract command-contract`.
- Runtime inspection: `ze doctor` (`internal/component/doctor/checks_config_claims.go` `checkConfigClaims`).
- Plugin config parsing & startup: Stage 1 `declare-registration` (`ReadLeafPaths`, validated in `internal/component/plugin/server/stage1_checks.go`) and Stage 2 `ConfigSection` parsed via `sdk.ConfigReader`.
- Config loading & interface backend: `internal/component/bgp/config/bgp_routes_plugin.go` (`buildConfigRouteRequest`), `internal/component/config/environment.go` + `internal/component/config/apply_env.go` (`ApplyEnvConfig`), and `ze-iface-conf.yang` (`vxlan` `ttl`) → `parseTunnelLeaves` → `internal/plugins/iface/netlink/tunnel_linux.go` (`buildVxlan`).

### Transformation Path
1. Each plugin's config parser reads its `rpc.ConfigSection` through `sdk.ConfigReader` (`String`, `Bool`, `Uint*`, `Int64`, `StringSlice`, `Child`, `List`, `Has`), which coerces the existing Stage 2 JSON-string wire representation and records every accessed schema-relative leaf path.
2. In the composition-root test binary (`internal/component/plugin/all`), the leaf claim collector gathers every compiled-in plugin's recorded leaf reads (by running each plugin's `ConfigReader`-based parser/probe plus hub/engine config readers) and hands them with `yang.DefaultLoader()` and `leaf_allowlist.json` to `claims.AuditLeaves`.
3. `claims.AuditLeaves` refuses any writable YANG leaf with no consumer and no valid allowlist entry, any phantom read path naming no YANG leaf, and any stale or reasonless allowlist entry.
4. At runtime, external plugins may report their read leaf paths in Stage 1 `DeclareRegistrationInput.ReadLeafPaths`; `stage1Checks` validates count/length/control-character bounds on both the daemon and `JudgeDeclaration`, `pluginserver` records them beside `ExternalConfigRoots`, and `ze doctor` runs `claims.AuditConfiguredLeaves` over the active config tree, reporting unread leaves under `doctor-config-leaf-unread` (never assuming an external plugin with empty `ReadLeafPaths` reads a leaf).
5. `./le doc yang-contract command-contract` extracts all YANG `notification` nodes, compares them against registered notification producers and `notification_allowlist.json`, and refuses any unproduced, unallowlisted notification.
6. For `vxlan` `ttl`: `ze-iface-conf.yang` declares `leaf ttl` (`ze:backend "kernel";`, `type uint8;`, `default 64;`), `ze:backend` refuses `vxlan/ttl` at verify on the VPP backend (whose `VxlanAddDelTunnelV3` carries no TTL field), `parseTunnelLeaves` (`internal/component/iface/config.go`) populates `spec.TTLSet`/`spec.TTL`, and `buildVxlan` (`internal/plugins/iface/netlink/tunnel_linux.go`) sets `netlink.Vxlan.TTL = int(spec.TTL)`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ external/internal plugin (Stage 1) | Optional `read-leaf-paths` (`[]string`, `omitempty`) on `rpc.DeclareRegistrationInput` (`sdk.Registration`), checked in `stage1Checks` | No |
| Engine → plugin (Stage 2) | Existing `rpc.ConfigSection` JSON-string payload read through `pkg/plugin/sdk` `ConfigReader` (zero Stage 2 wire change) | No |
| Composition root / `ze doctor` → `internal/component/config/claims` | Inventories passed in as pure `[]claims.LeafClaim` values so `claims` holds zero plugin/BGP imports | No |
| BGP config → plugin family route builder | `buildConfigRouteRequest` refuses configured `attribute { label / labels }` with `opererr` before invoking `InProcessConfigRouteParser` (keeping `registry.ConfigRouteRequest` unchanged: NLRI tokens, next hop, extended communities) | No |
| Interface config → kernel / VPP tunnel backend | `ze-iface-conf.yang` `vxlan/ttl` carries `ze:backend "kernel";` (exact-or-refuse on VPP at verify) and reaches `buildVxlan` (`netlink.Vxlan.TTL`) via `iface.TunnelSpec.TTLSet`/`TTL` | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/`; terms `DeclareRegistrationInput`, `ConfigSection`, `ExternalConfigRoots`, `ConfigRouteRequest`, `peer-show`, `ze-bgp-api`; positive controls: `pkg/plugin/rpc/types.go:340`, `pkg/plugin/rpc/types.go:994`, `internal/component/plugin/server/startup.go`, `internal/component/bgp/config/bgp_routes_plugin.go:65`, `internal/component/bgp/yang/ze-bgp-api.yang:249`. Stage 2 `ConfigSection` wire format does NOT change (every value stays a JSON string; `sdk.ConfigReader` reads it in-process inside the plugin). Two wires change cleanly: (1) Stage 1 `declare-registration` adds optional `read-leaf-paths` (`omitempty`, backward-compatible for third-party plugins); (2) `ze-bgp-api.yang` renames the 3 stale RPC names (`peer-show` → `peer-detail`, `peer-show-capabilities` → `peer-capabilities`, `peer-show-statistics` → `peer-statistics`) to match the existing registered handlers.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Stage 1 `declare-registration` (`rpc.DeclareRegistrationInput.ReadLeafPaths`) | writer | `pkg/plugin/sdk` `Plugin.Run` / `RunOrDeclare` sending `sdk.Registration` (`DeclareRegistrationInput` with optional `read-leaf-paths` JSON tag, `omitempty`) | `pkg/plugin/rpc/types.go:340`; `ai/doors/plugin-infra.md` | Additive optional field (`omitempty`): external plugins that omit it continue starting normally and are reported by `ze doctor` (`doctor-config-leaf-unread`) rather than assumed to read; documented in `docs/plugin-development/protocol.md` and `docs/architecture/api/process-protocol.md` |
| Stage 1 `declare-registration` (`rpc.DeclareRegistrationInput.ReadLeafPaths`) | reader | `internal/component/plugin/server/stage1_checks.go` (`stage1Checks` `read-leaf-paths` check) and `internal/component/plugin/server/startup.go` Stage 1 handler storing `ExternalReadLeafPaths` for `internal/component/doctor/checks_config_claims.go` `checkConfigClaims` | `ai/doors/plugin-infra.md`; `stage1_checks.go`; `checks_config_claims.go:47` | Validates optional `ReadLeafPaths` bounds/control characters in `stage1Checks` (shared by daemon and `JudgeDeclaration`), stores non-empty paths, and feeds `claims.AuditConfiguredLeaves` in `ze doctor` |
| `ze-bgp-api` peer inspection RPC names (`peer-detail`, `peer-capabilities`, `peer-statistics`) | writer | `internal/component/bgp/yang/ze-bgp-api.yang` RPC declarations and `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` `ze:rpc` pointers (`lines 97, 116, 133`) | `ze-bgp-api.yang:249,306,340`; `ze-peer-cmd.yang:97,116,133` | Renames the 3 dead RPC names (`peer-show`, `peer-show-capabilities`, `peer-show-statistics`) to match the live registered wire methods (`ze-bgp:peer-detail`, `ze-bgp:peer-capabilities`, `ze-bgp:peer-statistics`); updates `schema_test.go` and `yang_test.go` |
| `ze-bgp-api` peer inspection RPC names (`peer-detail`, `peer-capabilities`, `peer-statistics`) | reader | `internal/component/bgp/plugins/cmd/peer/peer.go` registered handlers (`ze-bgp:peer-detail`, `ze-bgp:peer-capabilities`, `ze-bgp:peer-statistics`) and `internal/le/doc/yangcontract/contract.go` | `ze-peer-cmd.yang:96-133`; `contract.go` | Preserved: handlers already serve `peer-detail`, `peer-capabilities`, and `peer-statistics`; aligning `ze-bgp-api.yang` eliminates the 3 orphan RPC names |

### Integration Points
- `pkg/plugin/sdk/config_reader.go`, `pkg/plugin/rpc/types.go`, & `internal/component/plugin/server/stage1_checks.go` - schema-bound typed config accessors, optional Stage 1 `read-leaf-paths`, and Stage 1 table validation.
- `internal/component/config/claims` (`AuditLeaves`, `AuditConfiguredLeaves`, `leaf_allowlist.json`) & `internal/component/plugin/all/config_claims_test.go` - build-time leaf consumption gate.
- `internal/component/doctor/checks_config_claims.go` (`doctor-config-leaf-unread`) - runtime unread-leaf report.
- `internal/le/config/unreadleaves/yangleafmentions.go` - repointed onto `claims.AuditLeaves`.
- `internal/le/doc/yangcontract/contract.go` - YANG notification producer check + `notification_allowlist.json`.
- `internal/component/bgp/yang/ze-bgp-api.yang`, `internal/component/bgp/config/bgp_routes_plugin.go`, `internal/component/config/environment.go`, `internal/component/config/apply_env.go`, `internal/component/iface/yang/ze-iface-conf.yang`, & `internal/plugins/iface/netlink/tunnel_linux.go` - T-4 surface fixes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Plugins read Stage 2 config through `sdk.ConfigReader`; gates and `ze doctor` call `claims.AuditLeaves`/`AuditConfiguredLeaves` |
| No unintended coupling (components stay isolated) | No | `claims` takes leaf claims as pure data and imports no plugin registry or BGP packages; `pkg/plugin/sdk` stays generic |
| No duplicated functionality (extends existing, does not recreate) | No | Extends `claims`, `doctor/checks_config_claims.go`, and `yangcontract`; retires the ad-hoc grep heuristic in `unreadleaves` |
| Zero-copy preserved where applicable (refs, not copies) | No | Config/control plane only; no BGP forwarding hot path touched |
| Registration over hardcoding, outbound | No | `ConfigReader` records read paths automatically from live parser execution; notification producers register rather than hardcoding a central list |
| Registration over hardcoding, inbound | No | Searched `yangleafmentions.go`, `claims.go`, `contract.go`, `apply_env.go`; replaces grep heuristic with recorded reads and keeps temporary allowlists strictly validated for `reason`, `owner`, and staleness |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Running each compiled-in plugin's `ConfigReader`-based parser against a schema-populated probe tree exercises every conditional branch that reads a supported leaf (or explicit reader-probe fixtures cover choice/case branches) | Owner decision T-2 (option 3) + T-3; YANG schemas declare every choice/case deterministically | A leaf read only when another leaf has a specific enum value could be missed by a single-pass probe | `TestConfigSchemaLeavesConsumed` + multi-branch probe over YANG `choice`/`case` nodes | unvalidated |
| A-2 | External plugins written in Go or other languages do not break when `DeclareRegistrationInput` adds an optional `read-leaf-paths` JSON field (`omitempty`) | `pkg/plugin/rpc/types.go` uses standard `encoding/json` with `omitempty` for all optional Stage 1 fields (`WantsConfig`, `Claims`, `DoctorChecks`, `FilterInsertions`) | Strict unknown-field decoding on an older binary | `pkg/plugin/rpc` round-trip tests + `./le mock plugin-external` | unvalidated |
| A-3 | Every remaining inert leaf uncovered by the first run of `TestConfigSchemaLeavesConsumed` can be either wired, deleted with a fix, or recorded in `leaf_allowlist.json` with a concrete owner spec per T-5 | Owner decision T-5 (2026-11-24) | Unowned inert leaves would fail the gate | First gate run audit against all `-conf.yang` modules | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Migrating all ~70+ plugin config parsers to `sdk.ConfigReader` in one commit could create an oversized, hard-to-review diff | Diff size exceeding one reviewable phase | Phase the migration cleanly: Phase 1 lands `sdk.ConfigReader` + `claims.AuditLeaves` + `leaf_allowlist.json` + doctor + yangcontract + T-4 surface fixes; subsequent phases migrate plugin parsers in bounded batches while shrinking `leaf_allowlist.json` until every shipped plugin uses `sdk.ConfigReader` |
| R-2 | Deleting an inert leaf that an existing `.ci` config writes breaks that test | `.ci` parse failure naming the deleted leaf | Check `git grep` across `test/` before deleting any leaf; wire leaves that belong to shipped features and delete only truly dead leaves with a clear operator fix |
| R-3 | An operator authoring an `l2vpn/evpn` or `l2vpn/vpls` config route writes `label` in `attribute { }` instead of inline in `nlri { }` (or a future labeled plugin family is registered later); note `ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, and `ipv6/mpls-vpn` use the standard prefix path in `bgp_routes.go:178-220`, not `InProcessConfigRouteParser` | Config load refusal when `attribute { label / labels }` appears with a plugin-registered family | `buildConfigRouteRequest` refuses `attribute { label / labels }` with an `opererr` naming the family and directing the operator to remove `label`/`labels` from `attribute { }` (or use inline `label`/`label-base` tokens in `nlri { }` for EVPN/VPLS); any future plugin family that genuinely needs attribute labels explicitly extends `ConfigRouteRequest` instead of silently dropping them |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Build gate (`TestConfigSchemaLeavesConsumed`) refuses the tree; `ze doctor` emits false-positive unread-leaf warnings; external plugin Stage 1 handshake fails; `ze-bgp-api` RPC callers or `vxlan`/`bgp` config parsers break |
| How is it reverted? | Single commit revert (or per-phase revert); `read-leaf-paths` is additive `omitempty` |
| Who else touches this path? | Siblings D (`spec-yang-command-arguments-by-leaf-name.md`) and F (`spec-yang-enum-vocabulary-agreement-gate.md`); `spec-software-version-mode-implements-two-of-four.md`; `spec-bgp-rib-state-subtree-has-no-producer.md`; `spec-bgp-api-notification-producers.md`; `spec-ipsec-remote-access.md`; `spec-peer-leaves-the-peer-parser-never-reads.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A test YANG module declaring a writable config leaf no consumer reads, inside `internal/component/plugin/all` | → | `claims.AuditLeaves` over `sdk.ConfigReader`-recorded paths + `leaf_allowlist.json` | `TestConfigSchemaLeavesConsumed` |
| `ze doctor` on a config setting a leaf whose plugin is compiled out or external with no `read-leaf-paths` | → | `checkConfigClaims` (`doctor-config-leaf-unread`) | `test/plugin/doctor-config-leaf-unread.ci` |
| A YANG module declaring a `notification` with no registered producer and no allowlist entry | → | `yangcontract.CheckNotifications` (`./le doc yang-contract command-contract`) | `TestNotificationWithoutProducerFailsContract` |
| Config route with `attribute { label 100; }` on any of the 6 `InProcessConfigRouteParser` plugin families | → | `bgpconfig.buildConfigRouteRequest` (`internal/component/bgp/config/bgp_routes_plugin.go`) | `TestPluginRouteRefusesAttributeLabels` |
| `vxlan` tunnel with `ttl 64` (kernel) and `ttl` on VPP | → | `ze-iface-conf.yang` (`ze:backend "kernel";`, `default 64;`), `parseTunnelLeaves`, and `buildVxlan` (`internal/plugins/iface/netlink/tunnel_linux.go`) | `TestVxlanTTLLeafAccepted` & `TestBuildVxlanTTL` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A plugin parses Stage 2 `rpc.ConfigSection` through `sdk.ConfigReader` (`pkg/plugin/sdk/config_reader.go`) or declares `read-leaf-paths` at Stage 1 | `ConfigReader` decodes typed values (`String`, `Bool`, `Uint8/16/32/64`, `Int64`, `StringSlice`, `Child`, `List`, `Has`) from the existing Stage 2 JSON-string wire payload without changing the Stage 2 wire format, and returns the exact set of schema-relative leaf paths read (`ReadPaths()`); `stage1Checks` (`internal/component/plugin/server/stage1_checks.go`) validates `ReadLeafPaths` count/length/control-character bounds identically for the daemon and `JudgeDeclaration` | Legacy parsers not yet migrated in external third-party plugins continue decoding `rpc.ConfigSection.Data` JSON directly without wire breakage |
| AC-2 | Any writable YANG config leaf in `internal/component/plugin/all` has no consumer recording a read of that leaf path (reproduced with a synthetic test module and against every owned inert leaf) | `TestConfigSchemaLeavesConsumed` (`claims.AuditLeaves`) fails naming the exact leaf path and fix; `TestConfigLeafPhantomReads` fails on any declared read path naming no schema leaf and on any `leaf_allowlist.json` entry missing `path`/`reason`/`owner` or gone stale; `./le config unread-leaves report` uses `claims.AuditLeaves` | Temporary entries in `internal/component/config/claims/leaf_allowlist.json` per T-5, each carrying a concrete `reason` and owning spec/symbol (such as `remote-access` → `spec-ipsec-remote-access.md`, `bgp/rib/adj-rib-*` → `spec-bgp-rib-state-subtree-has-no-producer.md`) |
| AC-3 | `ze doctor` runs on a config setting a leaf no running consumer declares reading (plugin compiled out, failed to load, or external plugin omitting `read-leaf-paths`) | `checkConfigClaims` emits a diagnostic under `doctor-config-leaf-unread` naming the leaf path, why it has no reader, and the exact fix | External plugins that declare `read-leaf-paths` in Stage 1 satisfy their declared leaves; external plugins declaring none are reported, never assumed to read |
| AC-4 | A YANG module declares an RPC with no handler or a `notification` with no registered producer | `./le doc yang-contract command-contract` refuses both naming the node; the 3 stale `ze-bgp-api.yang` RPCs are renamed to `peer-detail`, `peer-capabilities`, `peer-statistics` matching their live handlers; the 7 `ze-bgp-api` notifications sit in `notification_allowlist.json` naming `plan/post-release/features/spec-bgp-api-notification-producers.md` | Only entries in `notification_allowlist.json` naming a concrete post-release owner spec and reason (stale entries refused) |
| AC-5 | T-4 owned surface defects: `attribute { label / labels }` on plugin-family config routes, 7 `environment/reactor` YANG env aliases, `vxlan` `ttl` leaf in `ze-iface-conf.yang` + `buildVxlan`, and remaining owned inert leaves | `buildConfigRouteRequest` refuses configured `attribute { label / labels }` with `opererr` naming the family and fix for all 6 `InProcessConfigRouteParser` families (leaving `ConfigRouteRequest` and standard prefix-path labeled families untouched); all 7 `environment/reactor` leaves register canonical `ze.bgp.reactor.<leaf>` keys with legacy `Aliases` in `environment.go` and plumb in `apply_env.go`; `ze-iface-conf.yang` `vxlan` declares `leaf ttl` with `ze:backend "kernel";` and `default 64;` (refused on VPP at verify) and `buildVxlan` writes `spec.TTLSet`/`spec.TTL` to `netlink.Vxlan.TTL`; each remaining owned inert leaf is wired with a test, deleted with a fix, or recorded in `leaf_allowlist.json` under its owner spec per T-4/T-5 | Deferred subtrees explicitly ruled by Thomas in T-4 (`remote-access`, `bgp/rib/adj-rib-*`, 7 `ze-bgp-api` notifications) and `software-version/mode` (owned solely by `spec-software-version-mode-implements-two-of-four.md`) |
| AC-6 | Every owned Piece E journal row's reproduction (including verifying already-landed `rpki/cache-server/preference`, `ze-plugin:session-ping`, `ddos/flowtriq/api-key` row, and `TestDispatchBGPPeerBoRR`/`EoRR`) | None of the owned Piece E defect rows reproduce; each resolved row turns `fixed:` at closure with its verifying test, and any row whose deferred subtree stays allowlisted names its owning spec | The 3 "other cause:" rows held in Owned journal rows remain flagged for main-thread transfer to their fitting specs rather than being dropped or falsely closed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze doctor` on a config containing a leaf whose owning plugin is compiled out or external without `read-leaf-paths` | `ze doctor` → `checkConfigClaims` → `claims.AuditConfiguredLeaves` → `doctor-config-leaf-unread` diagnostic naming leaf and fix | `test/plugin/doctor-config-leaf-unread.ci` |
| 2 | Configures `attribute { label 100; }` on a plugin-registered BGP family (e.g., `ipv4/flow`, `l2vpn/evpn`, `l2vpn/vpls`) | Config tree → `buildConfigRouteRequest` → refused at config load with `opererr` naming the family and fix (while `ipv4/nlri-mpls`/`ipv4/mpls-vpn` continue through the standard prefix path) | `TestPluginRouteRefusesAttributeLabels` |
| 3 | Overrides `ze.bgp.reactor.forward-queue-size` via OS env or configures `vxlan { ttl 64; }` on kernel vs. VPP | Env registration (`environment.go`) + `ApplyEnvConfig` / `ze-iface-conf.yang` (`ze:backend "kernel";`, `default 64;`) → `parseTunnelLeaves` → `buildVxlan` (`netlink.Vxlan.TTL`) | `TestReactorEnvYANGAliases`, `TestVxlanTTLLeafAccepted`, `TestBuildVxlanTTL` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigReaderTypedAccessAndPathRecording` | `pkg/plugin/sdk/config_reader_test.go` | AC-1 (typed string-coerced reads for all YANG scalar/list/container shapes + exact recorded `ReadPaths()`) | |
| `TestStage1ChecksReadLeafPathsBounds` | `internal/component/plugin/server/stage1_checks_test.go` | AC-1 (`stage1Checks` refuses oversize/control-character `ReadLeafPaths` on both daemon and `JudgeDeclaration`) | |
| `TestConfigSchemaLeavesConsumed` | `internal/component/plugin/all/config_claims_test.go` | AC-2 (synthetic unread leaf fails; shipped schema passes with validated `leaf_allowlist.json`) | |
| `TestConfigLeafPhantomReads` | `internal/component/plugin/all/config_claims_test.go` | AC-2 (phantom read path, stale allowlist entry, missing reason/owner all fail) | |
| `TestDoctorReportsUnreadConfigLeaf` | `internal/component/doctor/checks_config_claims_test.go` | AC-3 (`doctor-config-leaf-unread` for compiled-out plugin and external plugin declaring no `read-leaf-paths`) | |
| `TestNotificationWithoutProducerFailsContract` | `internal/le/doc/yangcontract/contract_test.go` | AC-4 (unallowlisted notification without producer fails; allowlisted `ze-bgp-api` notifications pass; stale entry fails) | |
| `TestPluginRouteRefusesAttributeLabels` | `internal/component/bgp/config/bgp_routes_plugin_test.go` | AC-5 (`attribute { label / labels }` refused with `opererr` on all 6 `InProcessConfigRouteParser` families; standard prefix-path labeled families unaffected) | |
| `TestReactorEnvYANGAliases` | `internal/component/config/apply_env_test.go` | AC-5 (all 7 `ze.bgp.reactor.<leaf>` env aliases registered in `environment.go`, honored via OS env and `environment { reactor { ... } }`, legacy keys still accepted) | |
| `TestVxlanTTLLeafAccepted` | `internal/component/iface/yang/vxlan_ttl_test.go` | AC-5 (`vxlan` `ttl` uint8 leaf validates with `default 64` and `ze:backend "kernel"` in `ze-iface-conf.yang`, and VPP backend refuses `vxlan/ttl` at verify) | |
| `TestBuildVxlanTTL` | `internal/plugins/iface/netlink/tunnel_vxlan_build_linux_test.go` | AC-5 (`buildVxlan` wires `spec.TTLSet`/`spec.TTL` onto `netlink.Vxlan.TTL`, including boundary values 0 and 255) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `vxlan` `ttl` | 0..255 (`uint8`) | 0, 255 | -1 | 256 |
| `attribute` `label` | 0..1048575 (20-bit MPLS label) | 0, 1048575 | -1 | 1048576 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-config-leaf-unread` | `test/plugin/doctor-config-leaf-unread.ci` | `ze doctor` reports `doctor-config-leaf-unread` with leaf path and fix for an unread configured leaf | |

### Interop Tests (Scope: protocol)
N-A: Config claims, SDK config reader, doctor diagnostics, and YANG API alignment; no BGP protocol wire change.

## Files to Modify
- `pkg/plugin/rpc/types.go` - add optional `ReadLeafPaths []string` (`json:"read-leaf-paths,omitempty"`) to `DeclareRegistrationInput`
- `internal/component/plugin/server/stage1_checks.go` and `internal/component/plugin/server/stage1_checks_test.go` - add `read-leaf-paths` count/length/control-character check to `stage1Checks` (shared by daemon and `JudgeDeclaration`) and unit test
- `internal/component/plugin/server/startup.go` - accept and store optional `ReadLeafPaths` from Stage 1 declarations (`ExternalReadLeafPaths`)
- `internal/component/config/claims/claims.go` - add `LeafClaim`, `AuditLeaves`, `AuditConfiguredLeaves`, and `LeafAllowlist` loading `leaf_allowlist.json`
- `internal/component/plugin/all/config_claims_test.go` - add `TestConfigSchemaLeavesConsumed` and `TestConfigLeafPhantomReads`
- `internal/le/config/unreadleaves/yangleafmentions.go` - repoint `./le config unread-leaves report` onto `claims.AuditLeaves`
- `internal/component/doctor/checks_config_claims.go` - add `doctor-config-leaf-unread` check calling `claims.AuditConfiguredLeaves`
- `internal/le/doc/yangcontract/contract.go` - add YANG notification producer verification and `notification_allowlist.json` enforcement
- `internal/component/bgp/yang/ze-bgp-api.yang` - rename `peer-show` → `peer-detail`, `peer-show-capabilities` → `peer-capabilities`, `peer-show-statistics` → `peer-statistics`
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - update `ze:rpc` pointers on lines 97, 116, 133 to `peer-detail`, `peer-capabilities`, `peer-statistics`
- `internal/component/plugin/server/schema_test.go` and `internal/core/ipc/yang_test.go` - update RPC name assertions to `peer-detail`, `peer-capabilities`, `peer-statistics`
- `internal/component/bgp/config/bgp_routes_plugin.go` and `internal/component/bgp/config/bgp_routes_plugin_test.go` - refuse configured `attribute { label / labels }` with `opererr` on all 6 `InProcessConfigRouteParser` plugin families and add `TestPluginRouteRefusesAttributeLabels`
- `internal/component/config/environment.go` and `internal/component/config/apply_env.go` - register the 7 canonical `ze.bgp.reactor.<leaf>` env keys with legacy `Aliases` in `environment.go` and plumb `environment/reactor/<leaf>` in `apply_env.go`
- `internal/component/iface/yang/ze-iface-conf.yang` - add `leaf ttl` (`ze:backend "kernel";`, `type uint8;`, `default 64;`) to `container vxlan`
- `internal/plugins/iface/netlink/tunnel_linux.go` and `internal/plugins/iface/netlink/tunnel_vxlan_build_linux_test.go` - wire `spec.TTLSet`/`spec.TTL` into `buildVxlan` (`netlink.Vxlan.TTL`) and add `TestBuildVxlanTTL`
- `docs/architecture/config/yang-config-design.md` - document leaf-level claim gate, `leaf_allowlist.json`, and `sdk.ConfigReader` path recording under "Claim Completeness Gate"
- `docs/architecture/doctor-and-health-checks.md` - document `doctor-config-leaf-unread`
- `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`, and `docs/architecture/plugin-manager-wiring.md` - document optional Stage 1 `read-leaf-paths` and `sdk.ConfigReader`
- `docs/architecture/update-building.md` - document the `attribute { label / labels }` refusal in `buildConfigRouteRequest` alongside the existing plugin-route refusal list
- `docs/architecture/config/environment.md` and `docs/architecture/config/environment-block.md` - document canonical `ze.bgp.reactor.<leaf>` env aliases for all 11 `environment/reactor` leaves
- `docs/features/interfaces.md` - update the `vxlan` table row and "Outer-header TTL" anchors for `vxlan` `ttl` (`default 64`, `ze:backend "kernel"`)
- `docs/guide/configuration.md` - document `vxlan` `ttl` (`default 64`, kernel backend) and any deleted/newly wired leaves
- `ai/doors/config-yang.md` - update Gates and Traps for leaf-level claims and `sdk.ConfigReader`

## Files to Create
- `pkg/plugin/sdk/config_reader.go` - schema-bound typed `ConfigReader` over `rpc.ConfigSection` with automatic read-leaf path recording
- `pkg/plugin/sdk/config_reader_test.go` - unit tests for `ConfigReader` coercion and path recording
- `internal/component/config/claims/leaf_allowlist.json` - temporary recorded leaf-gate allowlist (`path`, `reason`, `owner`) per T-5
- `internal/le/doc/yangcontract/notification_allowlist.json` - temporary recorded notification allowlist (citing `plan/post-release/features/spec-bgp-api-notification-producers.md` for the 7 `ze-bgp-api` notifications)
- `internal/component/iface/yang/vxlan_ttl_test.go` - unit test for `vxlan` `ttl` YANG leaf (`ze:backend "kernel"`, `default 64`, VPP verify refusal)
- `test/plugin/doctor-config-leaf-unread.ci` - functional test verifying `ze doctor` emits `doctor-config-leaf-unread`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-api.yang` (3 RPC renames), `ze-peer-cmd.yang`, `ze-iface-conf.yang` (`vxlan` `ttl` with `ze:backend "kernel";` and `default 64;`), plus deleted/wired inert leaves |
| YANG validation constraints | Yes | `vxlan` `ttl` `uint8` + `ze:backend "kernel"`; `attribute { label / labels }` refusal on plugin families |
| YANG custom validators | No | None |
| CLI commands/flags | No | No new CLI verb or path |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` verified over modified `ze-peer-cmd.yang` |
| Editor autocomplete | Yes | Follows updated `ze-iface-conf.yang` (`vxlan` `ttl`) and deleted inert leaves |
| Functional test for new RPC/API | Yes | `test/plugin/doctor-config-leaf-unread.ci` |
| Pipe completeness | No | No command answer shape changes |
| Env var registration | Yes | 7 canonical `ze.bgp.reactor.<leaf>` env keys with legacy `Aliases` in `environment.go` and `apply_env.go` |
| Doctor check for runtime dependencies | No | None |
| Doctor code per problem class | Yes | `doctor-config-leaf-unread` in `internal/component/doctor/checks_config_claims.go` |
| Prometheus counters/metrics | No | None |
| BGP family surface | No | No new BGP family added; `ConfigRouteRequest` unchanged and `buildConfigRouteRequest` refuses `attribute { label / labels }` on all 6 plugin families |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Closes inert config/RPC/notification defects and adds `doctor-config-leaf-unread` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (`vxlan` `ttl` and any deleted/wired inert leaves) |
| 3 | CLI command added/changed? | No | Existing peer inspection CLI paths unchanged |
| 4 | API/RPC added/changed? | Yes | `ze-bgp-api.yang` aligns 3 stale RPC names to live wire methods |
| 5 | Plugin added/changed? | Yes | `pkg/plugin/sdk/config_reader.go` typed config reader adopted by plugins |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | Yes | Optional Stage 1 `read-leaf-paths` field (`omitempty`) on `DeclareRegistrationInput` |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/plugin-manager-wiring.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/config/yang-config-design.md` ("Claim Completeness Gate") |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/update-building.md`, `docs/architecture/plugin-manager-wiring.md`, `docs/architecture/config/environment-block.md`, `ai/doors/config-yang.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Doctor code `doctor-config-leaf-unread` and 7 `ze.bgp.reactor.*` env vars |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors spec plan/release/fixes/spec-yang-single-source-of-truth.md`. Updated in this spec: `docs/architecture/config/yang-config-design.md` (leaf claim gate, `leaf_allowlist.json`, `sdk.ConfigReader`), `docs/architecture/api/process-protocol.md` (optional Stage 1 `read-leaf-paths`), `docs/architecture/plugin-manager-wiring.md` (Stage 1 `read-leaf-paths` row in `DeclareRegistrationInput` table), `docs/architecture/doctor-and-health-checks.md` (`doctor-config-leaf-unread`), `docs/architecture/update-building.md` (`attribute { label / labels }` refusal in `buildConfigRouteRequest`), `docs/architecture/config/environment.md` and `docs/architecture/config/environment-block.md` (canonical `ze.bgp.reactor.<leaf>` env aliases for `environment/reactor`), `docs/features/interfaces.md` (`vxlan` `ttl` row and "Outer-header TTL" anchors). Checked and unaffected: `docs/architecture/api/commands.md` (renaming 3 stale `ze-bgp-api.yang` RPCs aligns with existing command paths), `docs/architecture/api/ipc_protocol.md` (optional `omitempty` JSON field leaves line framing unchanged), `docs/architecture/config/syntax.md` (`environment/reactor` env aliases do not change config file syntax), `docs/architecture/core-design.md` (`yangcontract` notification check stays in `internal/le`), `docs/architecture/plugin/plugin-system.md` (`stage1Checks` adds one check without changing table architecture; `InProcessConfigRouteParser` stays unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md`, `docs/plugin-development/protocol.md`, `ai/doors/config-yang.md` |

## Design presentation

### What changes

This spec implements Piece E alone (after splitting Pieces D and F into their own specs per T-1). It introduces schema-bound typed config accessors (`sdk.ConfigReader`) over Stage 2 `rpc.ConfigSection` JSON strings with automatic read-leaf path recording (T-2 option 3 + T-3), adds optional Stage 1 `read-leaf-paths` validated in `stage1Checks`, extends the root claim gate (`claims.Audit`) and `ze doctor` (`doctor-config-leaf-unread`) to leaf granularity backed by a strictly validated temporary allowlist (`leaf_allowlist.json`, T-5), adds notification producer enforcement to `./le doc yang-contract command-contract`, and repairs the verified T-4 surface defects (`ze-bgp-api.yang` 3 stale RPC names, `attribute { label / labels }` refusal on the 6 `InProcessConfigRouteParser` plugin families, 7 `environment/reactor` env aliases in `environment.go` and `apply_env.go`, `vxlan` `ttl` leaf with `ze:backend "kernel";` and `default 64;` wired into `buildVxlan`, and remaining owned inert leaves).

### Before and after

1. **Adding a YANG config leaf that no Go code reads.**
   Before: `TestConfigSchemaRootsClaimed` passes because the parent root is claimed; `./le config unread-leaves report` exits 0 as an advisory text grep; the operator configures the leaf and it silently does nothing.
   After: `TestConfigSchemaLeavesConsumed` fails at build time naming the unread leaf path and fix (unless explicitly recorded in `leaf_allowlist.json` with `reason` and `owner`), and `ze doctor` reports configured unread leaves at runtime under `doctor-config-leaf-unread`.
2. **Parsing Stage 2 config in a plugin.**
   Before: Plugins hand-parse `map[string]any` JSON strings via ad-hoc `configvalue` helpers, leaving no accurate record of which leaf paths are read.
   After: Plugins read through `sdk.ConfigReader` typed accessors over the unchanged Stage 2 JSON-string wire format, automatically recording read leaf paths; external plugins may also publish `read-leaf-paths` in Stage 1 (`omitempty`, validated by `stage1Checks`).
3. **Configuring `attribute { label 100; }` on a plugin family, setting `vxlan { ttl 64; }`, or setting `ze.bgp.reactor.forward-queue-size`.**
   Before: `buildConfigRouteRequest` silently drops `parsed.Labels` on all 6 plugin families; `vxlan` `ttl` fails YANG validation and `buildVxlan` ignores `spec.TTLSet`/`spec.TTL`; `ze.bgp.reactor.forward-queue-size` is unregistered.
   After: `buildConfigRouteRequest` refuses `attribute { label / labels }` with an `opererr` naming the family and fix (standard prefix-path labeled families stay unaffected); `vxlan` `ttl` validates with `ze:backend "kernel";` and `default 64;` (refused exact-or-reject on VPP) and reaches `netlink.Vxlan.TTL` in `buildVxlan`; all 7 `ze.bgp.reactor.<leaf>` env aliases are registered in `environment.go` and honored via OS env or `environment { reactor { ... } }`.
4. **Declaring a YANG notification with no producer or stale `ze-bgp-api` RPC names.**
   Before: `./le doc yang-contract command-contract` ignores notifications and `ze-bgp-api.yang` keeps 3 stale RPC names (`peer-show`, `peer-show-capabilities`, `peer-show-statistics`).
   After: The 3 RPCs match their live handlers (`peer-detail`, `peer-capabilities`, `peer-statistics`), and `./le doc yang-contract command-contract` enforces notification producers with the 7 `ze-bgp-api` notifications allowlisted to `plan/post-release/features/spec-bgp-api-notification-producers.md`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep D, E, and F in one combined spec (T-1) | Rejected by Thomas on 2026-11-24: splitting lets small, self-contained Pieces D and F land immediately without waiting on E's SDK accessor and leaf-gate rollout |
| Refuse unconsumed leaves at daemon startup (2026-10-23 design) | Plugins load on demand, compile out by feature tag, or run external and declare after config load; `yang-config-design.md` already assigns this to build-time gate + `ze doctor` |
| Promote `unreadleaves` string-literal grep heuristic to a build gate (T-2 option 2) | Rejected by Thomas on 2026-11-24: text matching produces both false positives and false negatives |
| Change Stage 2 `ConfigSection.Data` wire encoding from JSON strings to typed JSON numbers/booleans | Would break external non-Go plugins relying on the documented Stage 2 string contract; layering `sdk.ConfigReader` over the existing Stage 2 JSON-string payload satisfies T-2 option 3 + T-3 cleanly with zero Stage 2 wire break |
| Extend `registry.ConfigRouteRequest` with `Labels` for plugin families | Verified all 6 `InProcessConfigRouteParser` families (`flowspec`, `vpls`, `mvpn`, `evpn`, `srpolicy`, `mup`): none consumes `attribute { label / labels }` (`ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, `ipv6/mpls-vpn` use the standard prefix path, and EVPN/VPLS read inline NLRI tokens). Refusing `attribute { label / labels }` in `buildConfigRouteRequest` keeps `ConfigRouteRequest` minimal and agrees with `update-building.md` |
| Default `vxlan` `ttl` to `0` (inherit) or omit `ze:backend "kernel"` | Sibling IPv4-underlay tunnels (`gre`, `gretap`, `ipip`, `sit`) all declare `ze:backend "kernel";` and `default 64;` (matching VyOS `interfaces_vxlan.xml.in` overriding `0` with `64` so outer packets carry their own underlay hop budget rather than dropping early across multi-hop underlays; `plan/journal/schema-omits-a-leaf-the-backend-supports.md:21`), and VPP's `VxlanAddDelTunnelV3` carries no TTL field so omitting `ze:backend "kernel"` would violate exact-or-refuse |
| Zero allowlist on leaf gate (T-5) | Rejected by Thomas on 2026-11-24: a temporary allowlist (`path`, `reason`, `owner`) lets the gate turn on immediately to block new inert leaves while deferred subtrees (`remote-access`, `adj-rib-*`, 7 notifications) land in their owning specs |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add failing tests: `TestConfigReaderTypedAccessAndPathRecording`, `TestStage1ChecksReadLeafPathsBounds`, `TestConfigSchemaLeavesConsumed`, `TestConfigLeafPhantomReads`, `TestDoctorReportsUnreadConfigLeaf`, `TestNotificationWithoutProducerFailsContract`, `TestPluginRouteRefusesAttributeLabels`, `TestReactorEnvYANGAliases`, `TestVxlanTTLLeafAccepted`, `TestBuildVxlanTTL`, and `test/plugin/doctor-config-leaf-unread.ci`.
2. **Phase: SDK typed config reader & Stage 1 field (AC-1)** -- implement `pkg/plugin/sdk/config_reader.go`, add optional `ReadLeafPaths` (`omitempty`) to `pkg/plugin/rpc/types.go`, add the `read-leaf-paths` count/length/control-character check to `stage1Checks` in `internal/component/plugin/server/stage1_checks.go`, and store `ExternalReadLeafPaths` in `internal/component/plugin/server/startup.go`.
3. **Phase: Leaf claim gate, doctor check, and unreadleaves repoint (AC-2, AC-3)** -- implement `claims.AuditLeaves`, `claims.AuditConfiguredLeaves`, and `leaf_allowlist.json`; wire `TestConfigSchemaLeavesConsumed` / `TestConfigLeafPhantomReads` in `internal/component/plugin/all/config_claims_test.go`; add `doctor-config-leaf-unread` in `internal/component/doctor/checks_config_claims.go`; repoint `internal/le/config/unreadleaves/yangleafmentions.go` onto `claims.AuditLeaves`.
4. **Phase: Notification contract & stale RPC repair (AC-4)** -- add notification producer verification and `notification_allowlist.json` in `internal/le/doc/yangcontract/contract.go`; rename `peer-show`, `peer-show-capabilities`, `peer-show-statistics` in `ze-bgp-api.yang`, `ze-peer-cmd.yang`, `schema_test.go`, and `yang_test.go`.
5. **Phase: T-4 surface fixes & parser adoption (AC-5)** -- refuse `attribute { label / labels }` with `opererr` in `internal/component/bgp/config/bgp_routes_plugin.go`, register the 7 canonical `ze.bgp.reactor.<leaf>` env keys with legacy `Aliases` in `internal/component/config/environment.go` and plumb `environment/reactor/<leaf>` in `internal/component/config/apply_env.go`, add `leaf ttl` (`ze:backend "kernel";`, `default 64;`) to `vxlan` in `internal/component/iface/yang/ze-iface-conf.yang` and wire `spec.TTLSet`/`spec.TTL` into `buildVxlan` (`internal/plugins/iface/netlink/tunnel_linux.go`), migrate in-tree plugin parsers onto `sdk.ConfigReader`, resolve remaining owned inert leaves, and shrink `leaf_allowlist.json` to strictly T-4/T-5 deferred owners.
6. **Phase: Docs & journal closure (AC-6)** -- update all listed architecture, protocol, guide, and door pages; verify every owned Piece E journal row no longer reproduces and mark resolved rows `fixed:` at closure.

## Review Gate

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has concrete code, unit test, and functional verification at file + symbol |
| Correctness | `sdk.ConfigReader` preserves Stage 2 JSON-string wire compatibility; `stage1Checks` validates `ReadLeafPaths` identically on daemon and `JudgeDeclaration`; external plugins omitting `read-leaf-paths` are reported by `ze doctor`, never assumed to read; `attribute { label / labels }` on plugin families refused with `opererr`; `vxlan` `ttl` wired to `netlink.Vxlan.TTL` and refused on VPP via `ze:backend "kernel"` |
| Data flow | `claims` remains decoupled from plugin/BGP registries; `unreadleaves` shares `claims.AuditLeaves` with zero duplicated logic |
| Rule: no-layering | Advisory string-literal grep heuristic retired in favor of `claims.AuditLeaves`; every allowlist entry carries `path`, `reason`, and `owner` and fails if stale |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `sdk.ConfigReader`, Stage 1 `read-leaf-paths`, & `stage1Checks` | `./le test unit pkg path ./pkg/plugin/sdk/`, `./le test unit pkg path ./pkg/plugin/rpc/`, and `./le test unit pkg path ./internal/component/plugin/server/` |
| Leaf claim gate & allowlist validation | `./le test unit pkg path ./internal/component/plugin/all/` and `./le config unread-leaves report` |
| Doctor unread-leaf check | `./le test unit pkg path ./internal/component/doctor/` and `test/plugin/doctor-config-leaf-unread.ci` |
| Notification contract & RPC alignment | `./le doc yang-contract command-contract` and `./le test unit pkg path ./internal/le/doc/yangcontract/` |
| T-4 surface fixes (`label`, `environment/reactor` env aliases, `vxlan` `ttl` + netlink) | Unit tests in `bgp/config`, `config`, `iface/yang`, `plugins/iface/netlink`, `plugin/server`, `core/ipc` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| External plugin Stage 1 validation | `stage1Checks` (`internal/component/plugin/server/stage1_checks.go`) validates `ReadLeafPaths` entries are bounded in count/length and control-character free, matching all other Stage 1 declaration checks (`ai/rules/plugins.md`) |
| Exact-or-reject on `label`/`labels` and `vxlan` `ttl` | Plugin families refuse configured `attribute { label / labels }` at config load with `opererr`; VPP refuses `vxlan/ttl` at verify via `ze:backend "kernel"` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Gate reports a leaf that has a real reader | Migrate that reader onto `sdk.ConfigReader` (or add its schema probe branch) so the read path is recorded automatically |
| Gate reports an inert leaf with no reader | Wire it with a test, delete it with a fix, or file/cite an owning spec in `leaf_allowlist.json` per T-5 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

Rows moved out of this spec by the 2026-11-26 T-1 split and T-4 deduplication:
- Piece D (6 rows) → `plan/release/fixes/spec-yang-command-arguments-by-leaf-name.md`.
- Piece F (3 rows) → `plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md`.
- `unwired-feature.md` 2026-09-04 (`software-version/mode` half) → `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md` (its `adj-rib-*` half is owned by `plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md`).
- `unwired-feature.md` 2026-09-09 (`ze-bgp-api` notifications) → `plan/post-release/features/spec-bgp-api-notification-producers.md`.
- `declared-format-contradicts-payload.md` 2026-09-03 (`update/watchdog/withdraw`) → already marked `fixed:` in the journal (`eb084a4b99`).

Three rows of another cause, marked `other cause:` below, remain held here until the main thread moves them to their fitting specs (`spec-config-yang-loader-structural-checks`, `spec-web-cli-config-one-off-defects`, `spec-prose-cites-symbols-not-facts`).

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer.md` | 2026-10-10 | other cause: `goyang-enum-negative.patch` has no owner to drop it |
| `zero-value-as-valid-answer.md` | 2026-10-10 | other cause: `TestGoyangNumbersAnEnumAfterANegativeValue` follow-up |
| `hardcoded-count-in-test.md` | 2026-10-23 | other cause: `request bgp watchdog *` moved to YANG (`dcc0ab9dc5`): the wire-method snapshot half is fixed by `741d4382b7`; the `published-cli-commands.json` site catalog half is still open |
| `unwired-feature.md` | 2026-10-23 | `label`/`labels` in a config `attribute` block never reach plugin-family routes |
| `unwired-feature.md` | 2026-09-04 | `request peer <sel> borr` declared in `ze-refresh-cmd.yang`, dispatcher answers unknown command (verify `rfc7313_dispatch_test.go` and flip to `fixed:`) |
| `unwired-feature.md` | 2026-09-04 | fifteen leaves incl. `l2tp/shaper/upload-rate`, `image-server/listen-interface` (`bmp/route-action` already fixed by `fa74dc4c00`) |
| `unwired-feature.md` | 2026-09-04 | `ddos/flowtriq/api-key`, `bfd/enabled` and four more leaves (all 6 already fixed in HEAD; verify and flip to `fixed:`) |
| `unwired-feature.md` | 2026-09-04 | `fib/p4/*`, `fib/vpp/*` (`fib/kernel/flush-on-stop`, `sweep-delay`, `policy/route/interface` already fixed in HEAD) |
| `unwired-feature.md` | 2026-09-04 | IPsec `remote-access` subtree and seventeen more leaves inert (`remote-access` allowlisted citing `spec-ipsec-remote-access.md` + siblings) |
| `unwired-feature.md` | 2026-09-03 | `ospf/max-metric/router-lsa/*` (`ospf/reference-bandwidth` and `accept-lifetime` already fixed in HEAD) |
| `unwired-feature.md` | 2026-09-03 | `unit/ipv6/dhcpv6/pd`, `interface/monitor` and two more iface leaves inert |
| `unwired-feature.md` | 2026-09-03 | three remaining stale `ze-bgp-api` RPCs (`peer-show`, `peer-show-capabilities`, `peer-show-statistics`; `peer-add`/`peer-remove` already renamed) |
| `unwired-feature.md` | 2026-08-30 | `environment/reactor` 7 leaves lack the canonical YANG-named env alias (`ze.bgp.reactor.<leaf>`) |
| `declared-format-contradicts-payload.md` | 2026-09-05 | `rpki/cache-server/preference` (already implemented and tested in HEAD; verify and flip `open:` row to `fixed:`) |
| `declared-format-contradicts-payload.md` | 2026-08-31 | `ze-plugin:session-ping` reply leaf (already fixed 2026-09-15; verify and flip `open:` row to `fixed:`) |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-05 | `vxlan` case in `ze-iface-conf.yang` has no `ttl` leaf |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build-time leaf gate + `ze doctor` (`doctor-config-leaf-unread`) | Refuse unconsumed leaves at daemon startup | Load cannot decide: plugins load on demand, compile out, or run external and declare after config load; `yang-config-design.md` already assigns this to build gate + doctor |
| Layer `sdk.ConfigReader` over unchanged Stage 2 JSON-string wire payload + optional Stage 1 `read-leaf-paths` validated in `stage1Checks` | Rewrite Stage 2 wire format to typed JSON; maintain manual leaf lists; grep source literals | Satisfies Thomas's T-2 option 3 + T-3 while keeping non-Go external plugins working, eliminating manual leaf lists, and keeping daemon and `JudgeDeclaration` Stage 1 validation identical |
| Temporary recorded allowlist (`leaf_allowlist.json` + `notification_allowlist.json`) with required `path`, `reason`, `owner` and staleness check | Block gate until every deferred post-release subtree is built | Per Thomas's T-5: turns on the build gate immediately to block new inert declarations while keeping deferred items visible and owned |
| Refuse `attribute { label / labels }` in `buildConfigRouteRequest` without modifying `ConfigRouteRequest` | Add `Labels` to `ConfigRouteRequest` | None of the 6 `InProcessConfigRouteParser` families (`flowspec`, `vpls`, `mvpn`, `evpn`, `srpolicy`, `mup`) accepts attribute labels (`ipv4/nlri-mpls`, `ipv6/nlri-mpls`, `ipv4/mpls-vpn`, `ipv6/mpls-vpn` take the standard prefix path in `bgp_routes.go`; EVPN/VPLS read inline NLRI label tokens); refusal prevents silent drops while preserving `update-building.md`'s contract |
| `vxlan` `leaf ttl` carries `ze:backend "kernel";` and `default 64;`, wired into `buildVxlan` (`netlink.Vxlan.TTL`) | `default 0` (inherit) or omitting `ze:backend "kernel"` | Matches sibling IPv4-underlay tunnels (`gre`, `gretap`, `ipip`, `sit`) and VyOS (`interfaces_vxlan.xml.in` overriding `0` with `64` so outer packets get their own underlay hop budget; `plan/journal/schema-omits-a-leaf-the-backend-supports.md:21`), while `ze:backend "kernel";` enforces exact-or-refuse on VPP whose `VxlanAddDelTunnelV3` has no TTL field |

## Known Limitations
- External plugin YANG compiled at startup is owned by `plan/release/fixes/spec-plugin-external-yang-at-startup.md`.
- Wiring producers for the 7 `ze-bgp-api` notifications is post-release in `plan/post-release/features/spec-bgp-api-notification-producers.md`.
- IPsec `remote-access` is post-release in `plan/post-release/features/spec-ipsec-remote-access.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" for every changed command

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass, run by the main thread: `./le test unit pkg path ./pkg/plugin/sdk/`, `./le test unit pkg path ./pkg/plugin/rpc/`, `./le test unit pkg path ./internal/component/config/claims/`, `./le test unit pkg path ./internal/component/plugin/all/`, `./le test unit pkg path ./internal/component/doctor/`, `./le test unit pkg path ./internal/component/bgp/config/`, `./le test unit pkg path ./internal/component/config/`, `./le test unit pkg path ./internal/component/iface/yang/`, `./le test unit pkg path ./internal/plugins/iface/netlink/`, `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./internal/core/ipc/`, `./le test unit pkg path ./internal/le/doc/yangcontract/`, `./le config claims`, `./le config unread-leaves report`, `./le cli grammar`, `./le doc yang-contract command-contract`, `./le test functional plugin`, `./le mock plugin-external`, and `./le doc check all`
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` test passes (`test/plugin/doctor-config-leaf-unread.ci`)
- [ ] Interop tests N-A: no protocol wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + resolved owned journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-yang-single-source-of-truth.md` only

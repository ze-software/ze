# Spec: bgp-import-prepend-guards

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Existing import as-path-prepend must not silently modify route-server paths or contradict accepted no-prepend configuration. |
| RFC gaps | - |
| Depends | - (spec-bgp-local-as-options closed 2026-10-09; no dependency on the deferred migration recipe) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Inherited Sept 6 `deferred` pause | Lift the pause on these two existing-policy config-load guards (`rs-client` + import `as-path-prepend`, and `local-options no-prepend` + import `as-path-prepend`); move to `design` and proceed for release 1. The automatic inbound append migration recipe remains post-release in `plan/post-release/features/spec-bgp-local-as-inbound-append.md`. |

## Task

Own AC-3, AC-4, and the existing-policy safeguard work split from
`plan/post-release/features/spec-bgp-local-as-inbound-append.md` for release 1.
Ze's existing BGP import policy chain already executes `policy { modify <name> { set { as-path-prepend <1..32>; } } }`
at ingress (`FilterStagePeerChain`), rewriting the received UPDATE's `AS_PATH`
before cache insertion, RIB/plugin dispatch, and both forwarding rails. Today,
no config-load guard prevents attaching such a modifier to the import chain of:

1. a route-server client (`session { rs-client true; }`), where rewriting
   `AS_PATH` on ingress propagates a modified path to every other route-server
   client in violation of RFC 7947 Section 2.2.2.1; or
2. a peer declaring `session { asn { local-options [ no-prepend ]; } }`, where
   `no-prepend` and import `as-path-prepend` state opposite inbound `AS_PATH`
   intents on the same session.

Refuse both combinations fail-closed at every config validation and load entry
point (`ze config validate`, editor/API commit, daemon Stage 2 configure, and
live reload verify), using each owning plugin's single shared config parser
(`bgp-rs` for AC-3, `bgp-filter-modify` for AC-4), with actionable operator
errors (`internal/core/opererr`). Neither guard introduces automatic inbound
append or publishes the deferred ASN-migration recipe, which stays in
`plan/post-release/features/spec-bgp-local-as-inbound-append.md`.

### Reciprocal ownership

| Original obligation | Owner |
|---------------------|-------|
| AC-3 and AC-4 config-load refusals, unit and wiring tests, route-server functional proof (User Story 3 / A-3), guard-specific YANG help/descriptions and guide updates | This spec |
| AC-1, AC-2, AC-5, AC-6, AC-7, AC-8, ASN-migration recipe publication, width regression tests, other user stories, FRR interop, and recipe documentation | `plan/post-release/features/spec-bgp-local-as-inbound-append.md` |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, registration over hardcoding, operator errors
  → Constraint: every plugin works internal and forked over RPC; config checks run identically across offline validation, Stage 2 boot, and reload.
- [ ] `ai/doors/bgp-plugins.md` - BGP plugin ownership, boundaries, config transactions, gates
  → Constraint: `bgp-rs` owns `rs-client` and `bgp-filter-modify` owns `as-path-prepend`; neither imports another plugin's implementation.
- [ ] `docs/architecture/core-design.md` - plugin model, config transactions, policy filter chains
  → Decision: keep both guards inside their owning plugins' config transaction parsers (`sdk.ConfigTx`) and `InProcessConfigVerifier` registrations, matching `bgp-role` and `bgp-filter-family`.
- [ ] `docs/architecture/config/transaction-protocol.md` - `ConfigRoots`, `ConfigReads`, `WantsConfig`, `sdk.ConfigTx`
  → Constraint: declaring `ConfigReads: ["policy"]` beside `ConfigRoots: ["bgp"]` (and `WantsConfig: ["bgp", "policy"]` at Stage 1) delivers both roots whole whenever either changes, without auto-loading the plugin on a `policy`-only config.
- [ ] `docs/architecture/plugin/plugin-system.md` - `InProcessConfigVerifier`, `FatalOnConfigError`, `ConfigReads`
  → Constraint: registering `InProcessConfigVerifier` ensures `ze config validate`, editor commit, and API commit run the same parser as Stage 2 `OnConfigure` and reload `OnConfigVerify`; `FatalOnConfigError: true` makes a Stage 2 config refusal fail boot rather than run silently without the plugin.
- [ ] `docs/architecture/bgp/structural-forwarding.md` - received-wire forwarding rail
  → Constraint: ingress rewrites replace the cached payload before `reactorForwardRS` and plugin dispatch; stripping edits later on the RS rail would silently disagree with configuration and is rejected.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7947.md` - Internet Exchange BGP Route Server transparency
  → Constraint: Section 2.2.2.1 (`RFC7947-x-1`) specifies that a route server SHOULD NOT prepend its own AS number nor modify `AS_PATH` in any other way; refusing import `as-path-prepend` on `rs-client` prevents silent path corruption across clients.
- [ ] `rfc/short/rfc7705.md` - Autonomous System Migration Mechanisms
  → Constraint: Section 3.3 defines "No Prepend Inbound" (`local-options no-prepend`) as suppressing inbound prepending; combining it with an active import `as-path-prepend` is self-contradictory. This spec adds no automatic inbound append and changes no `rfc/short/` conformance verdict.

**Key insights:**
- On HEAD, `policy { modify }` lives under the top-level `policy` root (not under `bgp`), while peer `session` and `filter { import }` chains live under the `bgp` root.
- `policy.SectionsParser` (`internal/component/policy/section.go`) and `configjson.ForEachPeer` (`internal/component/bgp/configjson/traverse.go`) already exist for cross-root BGP filter validation (proven by `bgp-filter-family` and `bgp-role`).
- `bgp-filter-modify` already declares `ConfigRoots: ["bgp"]`, `ConfigReads: ["policy"]`, and `WantsConfig: ["bgp", "policy"]`; `bgp-rs` declares `ConfigRoots: ["bgp"]` and `WantsConfig: ["bgp"]` and only needs `ConfigReads: ["policy"]` and `policy` in `WantsConfig` added.
- Neither plugin currently registers `InProcessConfigVerifier` or `FatalOnConfigError`, so wiring both alongside `sdk.ConfigTx` gives one verdict across `ze config validate`, commit, reload, and boot.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/register.go` - registers `bgp-rs` with `ConfigRoots: ["bgp"]`, `Dependencies: ["bgp-rib"]`, `OptionalDependencies: ["bgp-adj-rib-in"]`; has no `ConfigReads`, no `InProcessConfigVerifier`, and no `FatalOnConfigError`.
- [ ] `internal/component/bgp/plugins/rs/server.go` - `RunRouteServer` binds `newQueueSizeConfigTx(...).Register(p)`; `routeServerRegistration` sets `WantsConfig: ["bgp"]`.
- [ ] `internal/component/bgp/plugins/rs/config_tx.go` - `parseQueueSizeSections` reads only `bgp > route-server > worker-queue-size` via `sdk.ConfigTx[int]`; never inspects `policy > modify`, `rs-client`, or import filter chains.
- [ ] `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang` - augments `/bgp:bgp/bgp:peer/bgp:session`, `/bgp:bgp/bgp:group/bgp:peer/bgp:session`, and `/bgp:bgp/bgp:group/bgp:session` with `leaf rs-client` (boolean, default false).
- [ ] `internal/component/bgp/plugins/filter_modify/register.go` - registers `bgp-filter-modify` with `ConfigRoots: ["bgp"]`, `ConfigReads: ["policy"]`, `Dependencies: ["bgp", "policy"]`; has no `InProcessConfigVerifier` and no `FatalOnConfigError`.
- [ ] `internal/component/bgp/plugins/filter_modify/filter_modify.go` - `runFilterModify` binds `newModifyConfigTx().Register(p)` and declares `WantsConfig: ["bgp", "policy"]`.
- [ ] `internal/component/bgp/plugins/filter_modify/config_tx.go` - `newModifyConfigTx` uses `policy.SectionParser` (reads `policy` only, ignores `bgp`).
- [ ] `internal/component/bgp/plugins/filter_modify/config.go` - `parseModifyDefs` and `buildSets` parse `policy > modify > <name> > set > as-path-prepend` (validated 1..32) into `rpc.FilterSet` with `FieldASPath` / `FilterSetActionPrepend`, without checking `bgp` peer `local-options no-prepend`.
- [ ] `internal/component/bgp/plugins/filter_modify/modify.go` - `modifyDef` tracks `egressOnlyNextHop`, `medRemove`, and compiled `sets`, but does not record a boolean for `as-path-prepend`.
- [ ] `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` - `as-path-prepend` (`uint8` range `1..32`) states that Ze prepends the peer's `local-as` but does not mention the `rs-client` or `local-options no-prepend` config-load refusals.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `local-options` enum `no-prepend` (group and peer `session > asn`) describes inbound non-prepending but does not state that pairing it with an import `as-path-prepend` modifier is refused at config load.
- [ ] `internal/component/bgp/reactor/reactor_notify.go`, `filter_ordered.go`, `aspath_edit.go`, `forward_rs.go` - `runIngressPolicyChainMeta` runs at stage 300 (after `LoopIngress` at stage 0), `prependASPathOps` (`aspath_edit.go`, replacing the older `filter_delta.go`) encodes `as-path-prepend` at source ASN width (`LenWithASN4`/`WriteToWithASN4` and `wireu.AS4PathForRewrite`), and the rewritten payload replaces `msg.WireUpdate` before caching, `reactorForwardRS`, and plugin dispatch.
- [ ] `internal/component/bgp/configjson/traverse.go` - `ForEachPeer` visits standalone peers, grouped peers, and dynamic-group templates (`origin.Template == true`).
- [ ] `internal/component/policy/section.go` - `SectionsParser` delivers both `policy` and `bgp` subtrees to `sdk.ConfigTx` (used by `internal/component/bgp/plugins/filter_family/config_tx.go`).
- [ ] `internal/component/bgp/plugins/role/config_tx.go` and `register.go` - precedent for a BGP plugin reading `bgp` + `policy` via `ConfigReads: [policy.RootName]`, `sdk.ConfigTx`, and `InProcessConfigVerifier`.
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` extracts `reg.ConfigSectionRoots()` (`ConfigRoots` + `ConfigReads`) from `ActiveConfigMap` (inactive members already excluded) and invokes `reg.InProcessConfigVerifier`.

**Behavior to preserve:**
- Width-aware `as-path-prepend` encoding in `prependASPathOps` (`aspath_edit.go`) and outbound `secondaryPrependAS` behavior (`peer_forward_facts.go`, `test/plugin/bgp-local-as-options.ci`, `test/plugin/bgp-local-as-inbound-untouched.ci`).
- General import `as-path-prepend` on non-`rs-client` peers that do not set `local-options no-prepend` (including peers without a `local-as` override and peers with `replace-as` alone).
- Export `as-path-prepend` on any peer, including `rs-client` peers or peers with `local-options no-prepend` (since `no-prepend` governs inbound only).
- Deactivated (`inactive`) import filter references are excluded from `ActiveConfigMap` / `ToPluginMap` and are not refused.
- `bgp-rs` `worker-queue-size` reload transaction, env-var precedence (`ze.bgp.route-server.worker-queue-size`), unchanged-subtree no-op, and `bgp-rs` dependency list (`bgp-adj-rib-in` stays in `OptionalDependencies`, not `Dependencies`).
- Ingress loop detection at stage 0 (`LoopIngress`) ahead of stage 300 (`policyChainStepName`), and existing `as-path-prepend` YANG range `1..32`.

**Behavior to change:**
- `bgp-rs` refuses at config validation, Stage 2 configure, and reload verify any active import chain (global, group, peer, or dynamic-group template) referencing a `policy { modify }` definition with `as-path-prepend` on any peer or dynamic group whose effective `session > rs-client` is `true` (AC-3).
- `bgp-filter-modify` refuses at config validation, Stage 2 configure, and reload verify any active import chain (global, group, peer, or dynamic-group template) referencing a `policy { modify }` definition with `as-path-prepend` on any peer or dynamic group whose effective `session > asn > local-options` includes `no-prepend` (AC-4).
- Guard-specific YANG descriptions/help (`ze-filter-modify.yang`, `ze-bgp-conf.yang`), generated `docs/config-reference.md`, and user docs (`docs/guide/configuration.md`, `docs/guide/plugins.md`) document both refusals.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Route-server config transaction | `internal/component/bgp/plugins/rs/config_tx.go`, `newQueueSizeConfigTx` / `parseQueueSizeSections`; `server.go`, `routeServerRegistration` | Parses `bgp > route-server > worker-queue-size` via `sdk.ConfigTx[int]` for Stage 2 and reload, reading only the `bgp` root | Extends: adds `policy` root (`ConfigReads` + `WantsConfig`), checks AC-3 inside the same parser, and exposes `verifyRSConfig` for `InProcessConfigVerifier` |
| Modify filter config transaction | `internal/component/bgp/plugins/filter_modify/config_tx.go`, `newModifyConfigTx`; `config.go`, `parseModifyDefs`; `modify.go`, `modifyDef` | Parses `policy > modify` definitions via `policy.SectionParser` and swaps `defsByName` via `sdk.ConfigTx` | Extends: switches to `policy.SectionsParser` over `bgp`, tracks `hasASPathPrepend` on `modifyDef`, checks AC-4 inside `parseModifyDefsWithBGP`, and exposes `verifyModifyConfig` for `InProcessConfigVerifier` |
| Cross-root policy section parser | `internal/component/policy/section.go`, `SectionsParser` | Unwraps both `policy` and a second root (`bgp`) from `[]sdk.ConfigSection` for filter plugins (`filter_family` precedent) | Extends/uses directly: `filter_modify/config_tx.go` calls `policy.SectionsParser` instead of duplicating section unwrapping |
| BGP peer and dynamic-group JSON traversal | `internal/component/bgp/configjson/traverse.go`, `ForEachPeer` and `ParseBGPSubtree` | Walks standalone peers, grouped peers, and dynamic-group templates (`origin.Template == true`) over delivered `bgp` JSON | Uses directly: both `rs` and `filter_modify` walk every peer and dynamic-group template through `configjson.ForEachPeer` |
| Static/offline plugin config verification and fatal startup refusal | `internal/component/config/plugin_verify.go`, `VerifyPluginConfigMapTransition`; `internal/component/plugin/registry/registry.go`, `ConfigReads`, `InProcessConfigVerifier`, `FatalOnConfigError` | Delivers `ConfigRoots` + `ConfigReads` to `InProcessConfigVerifier` during `ze config validate` and commit, and stops `ze` at Stage 2 on `*rpc.RPCCallError` when `FatalOnConfigError` is true (`bgp-gr`, `bgp-rpki`, `bgp-role` precedent) | Uses directly: both `bgp-rs` and `bgp-filter-modify` register `InProcessConfigVerifier` and `FatalOnConfigError: true` so validate, commit, reload, and boot enforce one verdict |
| Operator error formatting | `internal/core/opererr` (`ai/rules/user-facing-errors.md`) | Formats refusals with problem, why it matters, and the exact fix on its own last line | Uses directly: both AC-3 and AC-4 refusals are returned as `opererr` errors |
| Width-aware ingress AS_PATH prepend | `internal/component/bgp/reactor/aspath_edit.go`, `prependASPathOps`; `filter_ordered.go`, `runIngressPolicyChainMeta` | Applies `as-path-prepend` at stage 300 using the source payload's negotiated ASN width and `peer.settings.LocalAS` | Does not apply (preserved untouched): already width-aware on HEAD; this spec guards invalid config combinations rather than altering wire rewriting |
| BGP peer-tree import modify check | `internal/component/bgp/config/peers.go`, `checkImportModifyNextHop` | Refuses egress-only `next-hop` modes (`self`/`unchanged`/`auto`) on BGP import chains inside core `bgp/config` | Does not apply: `rs-client` is owned by `bgp-rs` (`ze-rs-conf.yang`, `TestRSConfigOwnedByPlugin`) and `as-path-prepend` is owned by `bgp-filter-modify`; placing the refusals in their owning plugins preserves plugin ownership ("delete the folder and the feature vanishes") |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | Both `bgp-rs` and `bgp-filter-modify` refuse conflicting import `as-path-prepend` configs fail-closed at verify, validate, and Stage 2 using `opererr`, never approximating or silently picking a winner |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `rs/register.go` and `rs/server.go` | "Its config verifier and its schema receive them, and they never auto-load it." | agrees | `bgp-rs` and `bgp-filter-modify` declare `ConfigReads: ["policy"]`, `InProcessConfigVerifier`, and `FatalOnConfigError: true`, so `policy`-only configs never auto-load them while BGP configs deliver both roots to offline validate, boot, and reload |
| `ai/doors/bgp-plugins.md` | Boundaries | door covering `rs/*` and `filter_modify/*` | "A plugin never imports another plugin's implementation" | agrees | `bgp-rs` reads `policy` JSON via `ConfigReads`/`WantsConfig` and `bgp-filter-modify` reads `bgp` JSON via `ConfigRoots`; neither plugin imports the other |
| `docs/architecture/core-design.md` | 18. Config Transaction Protocol | `// Design:` page of `rs/server.go`, `rs/config_tx.go`, `filter_modify/config_tx.go`, `config.go`, `modify.go`; anchored to `filter_modify/config.go` and `ze-bgp-conf.yang` | "Config changes (SIGHUP, CLI commit, API) use a bus-based transaction protocol" | agrees | Both guards execute inside each plugin's `sdk.ConfigTx` verify phase during reloads (aborting before apply) and inside the matching `InProcessConfigVerifier` during offline validate and commit |
| `docs/architecture/api/commands.md` | Overview | anchored to `rs/server.go` | - | silent | Anchored via `rs/server.go` (`show bgp rs status` / `peers`); this spec adds or modifies no CLI or API commands |
| `docs/architecture/api/process-protocol.md` | 5-Stage Startup Protocol | anchored to `rs/server.go` | "A Stage 2 error response is a REFUSAL, and a dead transport is not." | agrees | Both plugins set `FatalOnConfigError: true` so a Stage 2 config refusal (`*rpc.RPCCallError`) stops boot while transport/timeout errors remain non-fatal |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to `rs/server.go` | - | silent | Anchored via `rs/server.go` (`dispatchText`); this spec touches only config validation and `WantsConfig`, never text event parsing |
| `docs/architecture/buffer-architecture.md` | Executive Summary | anchored to `rs/server.go` | - | silent | Anchored via `rs/server.go`; this spec changes no wire buffer, iterator, or pooled encoder |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `rs/server.go` | - | silent | Anchored via `rs/server.go` (`lifecycleDoctorCheck`); this spec adds no doctor check, doctor code, or `Warn`/`Error` log line |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `rs/server.go` | - | silent | Anchored via `rs/server.go`; this spec changes no RIB storage, attribute pool, or route selection code |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Anchored via `ze-bgp-conf.yang`; this spec edits only `no-prepend` description prose and changes no UPDATE syntax |
| `docs/architecture/bgp/filter-path-asn.md` | The config | anchored to `ze-bgp-conf.yang` | - | silent | Anchored via `ze-bgp-conf.yang`; this spec does not touch `bgp-filter-path-asn` (`reject-asn`) |
| `docs/architecture/config/environment-block.md` | TL;DR | anchored to `ze-bgp-conf.yang` | - | silent | Anchored via `ze-bgp-conf.yang`; this spec adds or modifies no `environment { }` leaf |
| `docs/architecture/config/environment.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Anchored via `ze-bgp-conf.yang`; this spec adds or modifies no env var and preserves `ze.bgp.route-server.worker-queue-size` unchanged |
| `docs/architecture/config/syntax.md` | Overview | anchored to `ze-bgp-conf.yang` | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | Preserves all YANG node types and config syntax unchanged; updates only `as-path-prepend` and `no-prepend` description/help prose |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `ze-bgp-conf.yang` | "YANG defines format. Extensions declare behavior. Implementation executes behavior." | agrees | YANG defines `as-path-prepend`, `rs-client`, and `no-prepend`, while each owning plugin's Go verifier enforces the cross-root config-load guards |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `ze-bgp-conf.yang` | "**API = Policy** - RIB storage, best-path selection, route refresh, GR state" | agrees | Both policy guards stay in their owning plugins (`bgp-rs` and `bgp-filter-modify`), keeping policy validation out of the core BGP engine |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `ze-bgp-conf.yang` | "Never name a plugin package from the reactor." | agrees | Neither the reactor nor `bgp/config` imports `bgp-rs` or `bgp-filter-modify`; only `ze-bgp-conf.yang` `no-prepend` description prose changes under `internal/component/bgp/` |
| `ai/doors/le-tooling.md` | Where it sits | door covering `test/plugin/bgp-local-as-inbound-route-server.ci` | "Functional `.ci` files live under `test/<suite>/` and run through" | agrees | Adds `test/plugin/bgp-local-as-inbound-route-server.ci` in the standard `test/plugin/` suite following `ai/patterns/functional-test.md` |
| `docs/config-reference.md` | Route Modification Policy | a page this spec modifies | "The `set { }` container holds `local-preference`, `med`, `origin`, `next-hop`," | agrees | Updates `## Route Modification Policy` to document the two import `as-path-prepend` config-load refusals beside the existing `del { med; }` import-only rule |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to `rs/register.go` and `rs/server.go` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline external-plugin query validation is untouched; `InProcessConfigVerifier` runs in-process for compiled-in `bgp-rs` and `bgp-filter-modify` without starting any run loop |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | BGP policy guards are data-plane-neutral and leave all kernel-vs-VPP and `fib`/`sysctl` validation untouched |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Leaves kernel/VPP mismatch handling untouched and follows the same single-verdict discipline across validate, commit, reload, and start for both BGP policy guards |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Firewall backend verification and `ze:backend` annotations are untouched |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Introduces no data-plane selector, backend name, or `ze:backend` annotation |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Leaves BGP egress next-hop rewriting and automatic trailing `next-hop:auto` untouched |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Leaves `policy { modify }` `set { next-hop }` parsing and built-in `next-hop:*` chain references completely untouched |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not alter `policychain.Chain`, `FilterInsertion`, or export-chain compilation |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Leaves export-chain next-hop folding and raw-override provenance completely untouched |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Preserves `validateNextHop` and `egressOnlyNextHop` in `filter_modify` untouched and extends config-load import-chain refusal to `as-path-prepend` conflicts |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not modify forwarded or locally originated next-hop rewriting behavior |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Adds no automatic filter insertions and leaves `bgp-rs`'s existing `rsInsertions()` and `Policy.Implied()` untouched |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | That rule scopes non-networking/auxiliary services (registered service blocks and management listeners) so networking comes up, while unscoped BGP routing policy errors (`bgp-gr`, `bgp-llnh`, `PeersFromConfigTree`, `TestLoadConfigForStartRefusesUnscopedFailure`) remain fail-closed at load and reload stays refused as a whole without approximating conflicting routing policy |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` page of `rs/*` and `filter_modify/*`; anchored to `ze-bgp-conf.yang` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Kernel-on-VPP start and validate handling is untouched by these BGP import-policy config-load guards |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `rs/server.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds or modifies no CLI or API commands or pipe syntax |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `rs/server.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Adds or modifies no command or pipe override |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `rs/server.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Adds or modifies no pipe operator |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `rs/server.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation and preserves `bgp-rs` `Sends`/`Receives` and compiled bus authorization unchanged |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `rs/server.go` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not modify event queues or engine-to-plugin event delivery |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Candidate or startup configuration text parsed into a `config.Tree`, lowered through `Tree.ToPluginMap()` (active nodes only, inactive members pruned) into `[]rpc.ConfigSection` carrying both `bgp` and `policy` roots.
- Reached through four entry paths sharing one parser per plugin:
  1. `ze config validate` / CLI editor commit / API commit -> `config.VerifyPluginConfig` / `VerifyPluginConfigTransition` -> `reg.InProcessConfigVerifier(sections)`.
  2. Daemon boot (Stage 2) -> `plugin/server` `deliverConfigRPC` -> `sdk.ConfigTx.Configure(sections)`.
  3. Live reload / SIGHUP / commit on running daemon -> `TxCoordinator` verify phase -> `sdk.ConfigTx.Verify(sections)`.
  4. Runtime route forwarding (existing RS path verified by functional test) -> UPDATE from client 1 -> `reactor_notify.go` -> `reactorForwardRS` / `bgp-rs` -> client 2.

### Transformation Path
1. `config.BuildPluginConfigSections` / `VerifyPluginConfigMapTransition` / `plugin/server` (`joinConfigReads` and `reloadConfigSections`) collects `bgp` and `policy` JSON sections for both `bgp-rs` (`ConfigRoots: ["bgp"]`, `ConfigReads: ["policy"]`) and `bgp-filter-modify` (`ConfigRoots: ["bgp"]`, `ConfigReads: ["policy"]`).
2. **AC-3 (`bgp-rs`)**: `parseRSConfigSections` (invoked by `verifyRSConfig`, `OnConfigure`, and `OnConfigVerify`) parses `worker-queue-size` from `bgp`, extracts all `policy > modify > <name>` definitions declaring `set > as-path-prepend`, reads global `bgp > filter > import > filters`, and walks every peer and dynamic-group template via `configjson.ForEachPeer`. For each peer/template whose effective `session > rs-client` is `true` (peer overrides group, default `false`), it checks global + group + peer active import filter references (matching bare `NAME`, `modify:NAME`, and `bgp-filter-modify:NAME`). Any match returns an `opererr` refusal naming the peer/group, modify definition, `as-path-prepend`, RFC 7947 Section 2.2.2.1, and fix.
3. **AC-4 (`bgp-filter-modify`)**: `parseModifyDefsWithBGP` (invoked via `policy.SectionsParser` by `verifyModifyConfig`, `OnConfigure`, and `OnConfigVerify`) parses all `policy > modify` definitions (marking `modifyDef.hasASPathPrepend` in `buildSets`), reads global `bgp > filter > import > filters`, and walks every peer and dynamic-group template via `configjson.ForEachPeer`. For each peer/template whose effective `session > asn > local-options` contains `"no-prepend"` (peer overrides group when set), it checks global + group + peer active import filter references (matching bare `NAME`, `modify:NAME`, and `bgp-filter-modify:NAME`). Any match against a definition with `hasASPathPrepend` returns an `opererr` refusal naming the peer/group, modify definition, `as-path-prepend`, `local-options no-prepend`, and fix.
4. On refusal:
   - Offline `ze config validate` and commit fail before writing or activating the candidate.
   - Live reload fails during Phase 1 Verify; `apply` never runs, and running `bgp-rs` / `bgp-filter-modify` state stays intact.
   - Daemon boot fails Stage 2 with `*rpc.RPCCallError`; because both plugins set `FatalOnConfigError: true`, `configRefusalIsFatal` fails startup rather than running without the plugin.
5. On accepted valid route-server config (without import `as-path-prepend`), received UPDATEs flow through `reactor_notify.go` -> `reactorForwardRS` / `bgp-rs` to other clients with the original `AS_PATH` byte-identical.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ `bgp-rs` (offline) | `registry.Registration.InProcessConfigVerifier` handed `[]rpc.ConfigSection` for `bgp` + `policy` (`plugin_verify.go`) | Yes — `VerifyPluginConfigMapTransition` in `internal/component/config/plugin_verify.go` |
| Engine ↔ `bgp-rs` (runtime, internal & forked) | Stage 2 `configure` and reload `config-verify` RPCs carrying `[]sdk.ConfigSection` for `bgp` + `policy` via `WantsConfig` + `ConfigReads` | Yes — `rs/server.go` and `rs/config_tx.go` |
| Config ↔ `bgp-filter-modify` (offline) | `registry.Registration.InProcessConfigVerifier` handed `[]rpc.ConfigSection` for `bgp` + `policy` (`plugin_verify.go`) | Yes — `VerifyPluginConfigMapTransition` in `internal/component/config/plugin_verify.go` |
| Engine ↔ `bgp-filter-modify` (runtime, internal & forked) | Stage 2 `configure` and reload `config-verify` RPCs carrying `[]sdk.ConfigSection` for `bgp` + `policy` via `WantsConfig` + `ConfigReads` | Yes — `filter_modify/filter_modify.go` and `filter_modify/config_tx.go` |
| Engine ↔ `bgp-rs` (data plane, unchanged) | `reactorForwardRS` / cached forwarding relays post-import-filter `WireUpdate.Payload()` | Yes — `reactor/forward_rs.go` and `reactor/reactor_notify.go` |

### Changed-Wire Census

No wire format, RPC message schema, event format, or BGP wire attribute encoding changes in this spec.

**Search evidence:** Inspected `internal/component/bgp/plugins/rs/server.go`, `internal/component/bgp/plugins/rs/config_tx.go`, `internal/component/bgp/plugins/filter_modify/filter_modify.go`, `internal/component/bgp/plugins/filter_modify/config.go`, `internal/component/bgp/plugins/filter_modify/config_tx.go`, `internal/component/bgp/reactor/aspath_edit.go`, and `internal/component/bgp/reactor/forward_rs.go` (positive control: existing `WantsConfig` / `rpc.ConfigSection` delivery in `internal/component/policy/section.go` and `internal/component/bgp/plugins/role/config_tx.go`). All changes reuse the existing `rpc.ConfigSection` JSON delivery (`bgp` and `policy` roots) and existing `as-path-prepend` wire path without altering any wire bytes, RPC fields, or YANG leaf types.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| None (no wire changes) | writer | `internal/component/config/plugin_verify.go`, `BuildPluginConfigSections`; `internal/component/plugin/server/startup.go`, `joinConfigReads` | Existing `rpc.ConfigSection` JSON wire carrying `bgp` and `policy` roots is already produced unchanged | Preserve unchanged; no wire schema or field changes |
| None (no wire changes) | reader | `internal/component/bgp/plugins/rs/config_tx.go`, `parseQueueSizeSections`; `internal/component/bgp/plugins/filter_modify/config_tx.go`, `newModifyConfigTx` | Both plugins read the existing `bgp` and `policy` `ConfigSection` JSON payload without adding any new RPC or field | Preserve unchanged; reuse existing `configjson.ParseBGPSubtree` and `policy.SectionsParser` |

### Integration Points
- `internal/component/policy.SectionsParser` - unwraps `policy` and `bgp` config sections in `filter_modify/config_tx.go`.
- `internal/component/bgp/configjson.ForEachPeer` and `ParseBGPSubtree` - iterates standalone peers, grouped peers, and dynamic-group templates consistently with the reactor.
- `internal/core/configvalue.LeafList` - reads `filters` and `local-options` leaf-lists cleanly regardless of scalar vs slice JSON representation (`ai/rules/config.md`).
- `internal/core/opererr` - produces structured problem/why/fix refusals.
- `registry.Registration` (`ConfigReads`, `InProcessConfigVerifier`, `FatalOnConfigError`) - connects both plugins to `ze config validate`, commit, reload, and boot without touching core BGP or config packages.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both guards execute inside the existing `sdk.ConfigTx` (Stage 2 + reload verify) and `InProcessConfigVerifier` (`ze config validate` + commit) paths, never bypassing config transaction verification or adding runtime RS stripping |
| No unintended coupling (components stay isolated) | Yes | `bgp-rs` owns the `rs-client` refusal and reads `policy` JSON via `ConfigReads`; `bgp-filter-modify` owns the `no-prepend` refusal and reads `bgp` JSON via its existing `ConfigRoots`. Neither plugin imports the other (`./le arch tier check`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `sdk.ConfigTx`, `policy.SectionsParser`, `configjson.ForEachPeer`, `configvalue.LeafList`, `InProcessConfigVerifier`, and `opererr`; adds no new YANG leaf or custom validator layer |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Config-load validation runs off the packet hot path; zero changes to UPDATE forwarding or buffer pools |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Both plugins register `InProcessConfigVerifier`, `ConfigReads`, and `FatalOnConfigError` on their own `registry.Registration` in `register.go`; core `config` and `bgp/config` remain unaware of either guard, and deleting either plugin removes its guard |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No new plugin, command, leaf, or filter name is introduced. Searched `internal/component/config/plugin_verify.go`, `internal/component/plugin/server/startup.go`, `internal/component/bgp/config/register.go`, and `internal/component/bgp/plugins/all/` — `VerifyPluginConfigMapTransition` and `joinConfigReads` iterate `registry.All()` / `reg.ConfigSectionRoots()` dynamically |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Both `bgp` and `policy` sections are delivered together to `bgp-rs` and `bgp-filter-modify` across `ze config validate`, commit, Stage 2 boot, and reload when `ConfigRoots: ["bgp"]`, `ConfigReads: ["policy"]`, and `WantsConfig: ["bgp", "policy"]` are declared. | Verified in `internal/component/config/plugin_verify.go` (`reg.ConfigSectionRoots()`), `internal/component/policy/section.go` (`SectionsParser`), `internal/component/bgp/plugins/role/register.go`, and `docs/architecture/config/transaction-protocol.md` (`joinConfigReads`, `reloadConfigSections`). | One entry point would miss cross-root conflicts (e.g., editing only `policy { modify }` would not re-check `bgp` peers). | `TestRouteServerRefusesInboundASPathPrepend` and `TestNoPrependRefusesInboundASPathPrepend` driving both `config.VerifyPluginConfig` and `sdk.ConfigTx` (`Configure` + `Verify`). | confirmed by source; runtime test owed |
| A-2 | In delivered `bgp` JSON (`ToPluginMap`), inactive filter references are already pruned, `filter > import > filters` is cumulative (`bgp` + `group` + `peer`), and peer-level `session > rs-client` and `session > asn > local-options` override group-level values when set. | Verified in `internal/component/config/plugin_verify.go` (`ActiveConfigMap`), `internal/component/bgp/config/peers.go` lines 245-310, and `internal/component/bgp/config/resolve.go` (`cumulativePaths` excludes `local-options`). | A deactivated ref could be falsely refused, or a peer overriding `rs-client false` / `local-options [ replace-as ]` could inherit a stale group conflict. | Unit test subcases covering inactive refs, global/group/peer import inheritance, dynamic groups, and peer-overrides-group precedence. | confirmed by source; runtime test owed |
| A-3 | On a route server, an ingress import `as-path-prepend` rewrites `msg.WireUpdate` before caching, `reactorForwardRS`, and `bgp-rs` dispatch, so a second client would receive the modified path unless config load refuses the combination. | Verified in `internal/component/bgp/reactor/reactor_notify.go` lines 714-1009 and `forward_rs.go` (`update.WireUpdate.Payload()`). | Even if the RS rail were unaffected, the refusal stays as defense in depth and explicit config integrity (original design ruling). | `test/plugin/bgp-local-as-inbound-route-server.ci` proving client 2 receives client 1's AS_PATH byte-identically on valid RS config while the conflicting config is refused. | confirmed by source; functional proof owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding `ConfigReads: ["policy"]` or `WantsConfig` to `bgp-rs` breaks an existing `rs` test (such as `config_tx_test.go` or `all_test.go`) that delivers only a `bgp` section or inspects `bgp-rs` registration metadata. | Failure in `internal/component/bgp/plugins/rs` or `internal/component/plugin/all` unit tests. | Handle absent `policy` section cleanly (no `modify` definitions -> no conflict, `worker-queue-size` still parses and preserves unchanged-subtree no-op); keep `bgp-rs` `Dependencies` unchanged (`bgp-adj-rib-in` stays in `OptionalDependencies`). |
| R-2 | Adding `FatalOnConfigError: true` causes a transport failure (closed conn / timeout) to kill `ze`, or breaks a test that intentionally sends malformed JSON to `rs` or `filter_modify`. | Failure in `plugin/server` or functional tests. | `configRefusalIsFatal` (`internal/component/plugin/server/startup.go`) is already fatal only on `*rpc.RPCCallError` (plugin config refusal), never on transport/timeout errors; verify all existing `rs` and `filter_modify` tests pass under `-race`. |
| R-3 | Coercion mismatch: JSON config values for `rs-client` arrive as string `"true"`/`"false"` via `ToPluginMap` or Go `bool` in hand-built test maps, and single-element `local-options` / `filters` can arrive as a string or `[]any` / `[]string`. | False accept on real YANG-parsed config where `rs-client` is `"true"` (string) instead of `true` (bool), or `./le config coercion check` failure. | Use a shared bool-coercion helper accepting both `bool` and `"true"`/`"false"` strings (matching `filter_modify/modify.go` `readBool` with explicit presence detection for peer-override-group) and `configvalue.LeafList` for `filters` and `local-options`; pass `./le config coercion check`. |
| R-4 | Refusal errors omit required identifiers or violate `opererr` format (problem, why, fix on last line). | `./le arch operator-error check` failure or missing peer/definition/leaf/RFC text in unit tests. | Build both refusals with `opererr` naming peer or `group <name>`, modify definition, `as-path-prepend`, conflicting setting (`rs-client` + RFC 7947 Section 2.2.2.1, or `local-options no-prepend`), and exact removal fix on the last line. |
| R-5 | Ambiguous or foreign filter references: a peer import chain names `prefix-list:FOO` when a `policy { modify FOO { set { as-path-prepend 1; } } }` also exists, or names a `modify` filter in three spellings (`FOO`, `modify:FOO`, `bgp-filter-modify:FOO`). | False refusal on another plugin's filter, or missed refusal on a qualified reference. | Match only bare `NAME`, `modify:NAME`, and `bgp-filter-modify:NAME` (matching `filter_family/config.go` `exportRefInstanceName` and `bgp/config/peers.go` `namesFilterType`); ignore any reference qualified with another prefix. |
| R-6 | Contradictory settings accepted with a warning or silent precedence winner instead of failing closed. | Config loads despite both settings present. | Both checks return hard errors in `InProcessConfigVerifier`, `OnConfigure`, and `OnConfigVerify`; never warn-and-continue and never pick a silent winner. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A too-permissive check lets an RS client silently rewrite AS_PATH for every other IXP client (RFC 7947 Section 2.2.2.1 violation) or accepts contradictory `no-prepend` + import `as-path-prepend`. A too-broad check rejects valid configs (e.g., export `as-path-prepend`, non-RS peers, `replace-as`-only peers, or deactivated import refs). |
| How is it reverted? | Single commit revert; no persisted state, wire format, or schema structure changes (only YANG descriptions/help and config-load validation). |
| Who else touches this path? | `plan/post-release/features/spec-bgp-local-as-inbound-append.md` (deferred sibling depending on these guards); `internal/component/bgp/plugins/rs/config_tx.go` (`worker-queue-size` reload); `internal/component/bgp/plugins/filter_modify/config.go` (modify definitions). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator configures `rs-client true` (peer, group, or dynamic group) and an active import `as-path-prepend` modifier, validated via `config.VerifyPluginConfig` and delivered to `bgp-rs` `ConfigTx` (`Configure` / `Verify`) | → | `rs` `verifyRSConfig` and `parseQueueSizeSections` (`internal/component/bgp/plugins/rs/config_tx.go`) | `TestRouteServerRefusesInboundASPathPrepend` |
| Operator configures `local-options [ no-prepend ]` (peer, group, or dynamic group) and an active import `as-path-prepend` modifier, validated via `config.VerifyPluginConfig` and delivered to `bgp-filter-modify` `ConfigTx` (`Configure` / `Verify`) | → | `filter_modify` `verifyModifyConfig` and `parseModifyDefsWithBGP` (`internal/component/bgp/plugins/filter_modify/config_tx.go`, `config.go`) | `TestNoPrependRefusesInboundASPathPrepend` |
| Route server runs with two `rs-client` peers (client 1 announces a route, client 2 receives it) and rejects the conflicting `rs-client` + import `as-path-prepend` config at config load | → | `bgp-rs` config refusal + `reactorForwardRS` / `bgp-rs` transparent relay | `test/plugin/bgp-local-as-inbound-route-server.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-3 | A peer (standalone, grouped with inherited `rs-client`, or dynamic-group template) whose effective `session > rs-client` is `true` has an active import chain (global, group, or peer level) referencing a `policy { modify <name> }` carrying `set { as-path-prepend <n>; }` in any valid spelling (`<name>`, `modify:<name>`, or `bgp-filter-modify:<name>`) | Config validation (`ze config validate`, commit), daemon Stage 2 configure, and live reload verify all fail with an `opererr` error naming the peer (or `group <name>`), the modify definition `<name>`, the `as-path-prepend` leaf, RFC 7947 Section 2.2.2.1, and the removal fix; on a valid route server without that combination, a second client receives the first client's `AS_PATH` byte-identically | Peers with effective `rs-client false` (including grouped peers overriding group `rs-client true` to `false`), export-chain `as-path-prepend` references, deactivated (`inactive`) import references, and import references qualified for another filter type (`prefix-list:<name>`) stay accepted, because none of them rewrites an RS client's received `AS_PATH` |
| AC-4 | A peer (standalone, grouped with inherited `local-options`, or dynamic-group template) whose effective `session > asn > local-options` contains `no-prepend` has an active import chain (global, group, or peer level) referencing a `policy { modify <name> }` carrying `set { as-path-prepend <n>; }` in any valid spelling (`<name>`, `modify:<name>`, or `bgp-filter-modify:<name>`) | Config validation (`ze config validate`, commit), daemon Stage 2 configure, and live reload verify all fail with an `opererr` error naming the peer (or `group <name>`), the modify definition `<name>`, `as-path-prepend`, and `local-options no-prepend`, stating why they conflict and that one of the two must be removed | Peers without `no-prepend` (including peers with `local-options [ replace-as ]` alone, peers overriding a group's `no-prepend` with a peer `local-options` lacking `no-prepend`, or peers with no `local-as` override), export-chain `as-path-prepend` references on a `no-prepend` peer, deactivated (`inactive`) import references, and foreign-prefixed filter references stay accepted, because `no-prepend` governs inbound prepending only and general import `as-path-prepend` remains a valid policy action |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a route server and attaches an import `as-path-prepend` modifier to a route-server client (standalone, grouped, or dynamic group), or reloads into that state | config text → `ParseTreeForValidation` / `ToPluginMap` → `bgp-rs` `InProcessConfigVerifier` / Stage 2 `OnConfigure` / reload `OnConfigVerify` → `opererr` refusal naming peer, definition, `as-path-prepend`, RFC 7947 Section 2.2.2.1, and fix; valid RS config relays client 1's AS_PATH byte-identically to client 2 | `TestRouteServerRefusesInboundASPathPrepend` and `test/plugin/bgp-local-as-inbound-route-server.ci` |
| 2 | Configures a peer with `session { asn { local-options [ no-prepend ]; } }` and attaches an import `as-path-prepend` modifier (standalone, grouped, or dynamic group), or reloads into that state | config text → `ParseTreeForValidation` / `ToPluginMap` → `bgp-filter-modify` `InProcessConfigVerifier` / Stage 2 `OnConfigure` / reload `OnConfigVerify` → `opererr` refusal naming peer, definition, `as-path-prepend`, `local-options no-prepend`, and fix | `TestNoPrependRefusesInboundASPathPrepend` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteServerRefusesInboundASPathPrepend` | `internal/component/bgp/plugins/rs/config_refusal_test.go` | AC-3 across `config.VerifyPluginConfigContent` (full YANG parse + `InProcessConfigVerifier`), `ConfigTx.Configure` (Stage 2), and `ConfigTx.Verify` (reload leaving running `worker-queue-size` unchanged): refuses standalone peer, group-inherited `rs-client`, dynamic-group template, global/group/peer import chains, and bare/`modify:`/`bgp-filter-modify:` spellings; asserts `opererr` fields naming peer/group, definition, `as-path-prepend`, RFC 7947 Section 2.2.2.1, and fix; confirms permitted survivors (non-`rs-client` peer, peer overriding `rs-client false`, export-only `as-path-prepend`, deactivated import ref, foreign-prefixed ref, modify without `as-path-prepend`) load cleanly; confirms `FatalOnConfigError` and `ConfigReads` on `bgp-rs` registration | |
| `TestNoPrependRefusesInboundASPathPrepend` | `internal/component/bgp/plugins/filter_modify/config_refusal_test.go` | AC-4 across `config.VerifyPluginConfigContent` (full YANG parse + `InProcessConfigVerifier`), `ConfigTx.Configure` (Stage 2), and `ConfigTx.Verify` (reload leaving running `defsByName` unchanged): refuses standalone peer, group-inherited `no-prepend`, dynamic-group template, global/group/peer import chains, and bare/`modify:`/`bgp-filter-modify:` spellings; asserts `opererr` fields naming peer/group, definition, `as-path-prepend`, `local-options no-prepend`, and fix; confirms permitted survivors (`replace-as` alone, no `local-options`, peer overriding group `no-prepend`, export-only `as-path-prepend` on `no-prepend` peer, deactivated import ref, foreign-prefixed ref, modify without `as-path-prepend`) load cleanly; confirms `FatalOnConfigError` on `bgp-filter-modify` registration | |

### Boundary Tests (numeric inputs)
N-A — this spec introduces no numeric inputs and leaves `as-path-prepend` (`uint8` range `1..32`, already tested in `internal/component/bgp/plugins/filter_modify/rfc4271_modify_test.go`) and `worker-queue-size` (`1..1000000`, already tested in `internal/component/bgp/plugins/rs/config_tx_test.go`) unchanged.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-local-as-inbound-route-server` | `test/plugin/bgp-local-as-inbound-route-server.ci` | Proves A-3 and User Story 1 end to end: two `rs-client` peers exchange a route through `bgp-rs` with client 2 receiving client 1's `AS_PATH` byte-identically, and running config validation / load with `rs-client true` + import `as-path-prepend` is refused naming the peer, modify definition, `as-path-prepend`, and RFC 7947 Section 2.2.2.1 | |

### Interop Tests (Scope: protocol)
N-A — this spec enforces local config-load refusals and verifies unchanged byte-identical route-server forwarding; it introduces no new wire encoding, capability, or peer-facing protocol state machine behavior. FRR interop for the deferred ASN-migration inbound recipe remains owned by `plan/post-release/features/spec-bgp-local-as-inbound-append.md`.

## Files to Modify
- `internal/component/bgp/plugins/rs/register.go` - add `ConfigReads: []string{policy.RootName}`, `InProcessConfigVerifier: verifyRSConfig`, and `FatalOnConfigError: true`.
- `internal/component/bgp/plugins/rs/server.go` - add `policy.RootName` to `routeServerRegistration().WantsConfig` so internal and forked `bgp-rs` receive `policy` alongside `bgp`.
- `internal/component/bgp/plugins/rs/config_tx.go` - parse `bgp` + `policy` sections in `parseQueueSizeSections`, enforce the AC-3 `rs-client` + import `as-path-prepend` refusal, and expose side-effect-free `verifyRSConfig`.
- `internal/component/bgp/plugins/filter_modify/register.go` - add `InProcessConfigVerifier: verifyModifyConfig` and `FatalOnConfigError: true`.
- `internal/component/bgp/plugins/filter_modify/config_tx.go` - switch `newModifyConfigTx` to `policy.SectionsParser` over `configRootBGP` calling `parseModifyDefsWithBGP`, and expose side-effect-free `verifyModifyConfig`.
- `internal/component/bgp/plugins/filter_modify/config.go` - add `parseModifyDefsWithBGP` and `validateNoPrependImportConflict` enforcing the AC-4 `local-options no-prepend` + import `as-path-prepend` refusal.
- `internal/component/bgp/plugins/filter_modify/modify.go` - record `hasASPathPrepend bool` on `modifyDef` in `buildSets`.
- `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` - update `as-path-prepend` description/`ze:help` to note the config-load refusals on `rs-client` peers (RFC 7947 Section 2.2.2.1) and peers with `local-options no-prepend`.
- `internal/component/bgp/yang/ze-bgp-conf.yang` - update `local-options` / `no-prepend` description to note the config-load refusal alongside an active import `as-path-prepend` modifier.
- `docs/guide/configuration.md` - document both config-load refusals beside `no-prepend` and `as-path-prepend` (without publishing the deferred ASN-migration recipe).
- `docs/guide/plugins.md` - document `bgp-rs` refusing import `as-path-prepend` on `rs-client` peers and `bgp-filter-modify` refusing import `as-path-prepend` on `local-options no-prepend` peers.
- `docs/config-reference.md` - update `## Route Modification Policy` to document the two import `as-path-prepend` config-load refusals beside `del { med; }`.

## Files to Create
- `internal/component/bgp/plugins/rs/config_refusal_test.go` - `TestRouteServerRefusesInboundASPathPrepend` (AC-3 unit and config-entry wiring proof).
- `internal/component/bgp/plugins/filter_modify/config_refusal_test.go` - `TestNoPrependRefusesInboundASPathPrepend` (AC-4 unit and config-entry wiring proof).
- `test/plugin/bgp-local-as-inbound-route-server.ci` - route-server transparency and config-refusal functional proof (A-3 / User Story 1).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf, container, or RPC; only prose updates to existing `as-path-prepend` and `no-prepend` descriptions/help |
| YANG validation constraints | No | Existing `uint8` range `1..32` on `as-path-prepend` and `local-options` enum are unchanged |
| YANG custom validators | No | Both checks compare cross-root/cross-leaf settings (`policy > modify` vs `bgp` peer `rs-client` / `local-options`), which leaf `ValidateFn` cannot see; enforced in owning plugins' `InProcessConfigVerifier` + `sdk.ConfigTx` |
| CLI commands/flags | No | No CLI command or flag added |
| CLI grammar (keyword before value) | N-A | No CLI command added |
| Editor autocomplete | No | Existing leaves and enums already complete |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-local-as-inbound-route-server.ci` proves end-to-end config refusal and byte-identical RS relay |
| Pipe completeness | N-A | No command output added |
| Env var registration | N-A | No `environment/` leaf or env key added; existing `ze.bgp.route-server.worker-queue-size` preserved |
| Doctor check for runtime dependencies | No | No new host/file/socket/service/kernel runtime dependency introduced |
| Doctor code per problem class | No | Neither guard emits a `Warn`/`Error` log line or report-bus raise; both return synchronous `opererr` config-validation errors through `InProcessConfigVerifier`, Stage 2 `OnConfigure`, and reload `OnConfigVerify` |
| Prometheus counters/metrics | No | No new runtime state or counter needed for static config-load refusals |
| BGP family surface (new SAFI / capability / attribute) | N-A | No address family, capability, or path attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Config-load safety guards on existing `as-path-prepend`, `rs-client`, and `no-prepend` leaves; no new feature card or level change |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/config-reference.md` (document the two config-load refusals; syntax itself unchanged) |
| 3 | CLI command added/changed? | No | No CLI command changes |
| 4 | API/RPC added/changed? | No | No RPC or command answer changes |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (note `bgp-rs` and `bgp-filter-modify` config-load refusals) |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` and `docs/guide/plugins.md` |
| 7 | Wire format changed? | No | No BGP or RPC wire format changes |
| 8 | Plugin SDK/protocol changed? | No | Reuses existing `sdk.ConfigTx`, `ConfigReads`, `WantsConfig`, `InProcessConfigVerifier`, and `FatalOnConfigError` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7947 and RFC 7705 are already enrolled; no requirement row or conformance classification changes |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` plugin suite and Go unit test harness |
| 11 | Affects daemon comparison? | No | Daemon comparison note for the deferred ASN-migration recipe remains in `plan/post-release/features/spec-bgp-local-as-inbound-append.md` |
| 12 | Internal architecture changed? | No | Extends existing `sdk.ConfigTx` + `InProcessConfigVerifier` + `ConfigReads` architecture without altering structure |
| 13 | Route metadata keys added/changed? | No | No metadata key touched |
| 14 | Prometheus counters added/changed? | No | No metrics touched |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Existing `bgp-rs` and `bgp-filter-modify` keep their names, commands, events, and capabilities |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors spec plan/release/fixes/spec-bgp-import-prepend-guards.md`: zero unaddressed blocking `// Design:` docs (`docs/architecture/core-design.md` is unaffected because plugin/transaction structures are unchanged). The 37 advisory `<!-- source: -->` docs mentioning `rs/register.go`, `rs/server.go`, `filter_modify/config.go`, or `ze-bgp-conf.yang` (`docs/DESIGN.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/text-parser.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/bgp/filter-path-asn.md`, `docs/architecture/buffer-architecture.md`, `docs/architecture/config/environment-block.md`, `docs/architecture/config/environment.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/rib-transition.md`, `docs/comparison.md`, `docs/features/bgp-protocol.md`, `docs/features/configuration.md`, `docs/features/plugins.md`, `docs/features/srv6.md`, `docs/guide/add-path.md`, `docs/guide/bfd.md`, `docs/guide/bgp-peering.md`, `docs/guide/bgp-policy.md`, `docs/guide/command-reference.md`, `docs/guide/config-deactivate.md`, `docs/guide/docker.md`, `docs/guide/flowspec-route-reflector.md`, `docs/guide/graceful-restart.md`, `docs/guide/healthcheck.md`, `docs/guide/irr-filtering.md`, `docs/guide/looking-glass-howto.md`, `docs/guide/quickstart.md`, `docs/guide/redistribution.md`, `docs/guide/route-reflection.md`, `docs/guide/status.md`, `docs/guide/ubuntu-build-install.md`, `docs/why-ze.md`) are unaffected because no command, wire format, schema structure, or unrelated feature changes; guard prose lands in `docs/guide/configuration.md`, `docs/guide/plugins.md`, and `docs/config-reference.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `local-as` / `no-prepend` and `as-path-prepend` examples in `docs/guide/configuration.md`, `docs/guide/plugins.md`, and regenerated `docs/config-reference.md` against the YANG schema |

## Design presentation

### What changes

Ze now refuses two invalid BGP import-policy combinations fail-closed at every config entry point (`ze config validate`, editor/API commit, daemon boot Stage 2, and live reload) instead of silently accepting them: attaching an active `policy { modify }` with `as-path-prepend` to the import chain of a route-server client (`rs-client true`, owned by `bgp-rs`, citing RFC 7947 Section 2.2.2.1), or to the import chain of a peer declaring `local-options no-prepend` (owned by `bgp-filter-modify`). Both checks inspect standalone peers, grouped peers (with inheritance and peer-level overrides), dynamic-group templates, and global/group/peer cumulative import chains across all three reference spellings (`NAME`, `modify:NAME`, `bgp-filter-modify:NAME`), while keeping non-RS peers, `replace-as`-only peers, export-chain `as-path-prepend`, and deactivated references accepted.

### Before and after

1. **Route-server client with import `as-path-prepend` (`bgp-rs`, AC-3)**
   - **Before:** `ze config validate`, `ze start`, and reload silently accept a peer with `session { rs-client true; }` and `filter { import { filters [ PREPEND ]; final accept; } }` where `policy { modify PREPEND { set { as-path-prepend 1; } } }`. At runtime, ingress rewrites `AS_PATH` before caching and forwarding, so other route-server clients receive a modified `AS_PATH` in violation of RFC 7947 Section 2.2.2.1.
   - **After:** Config validation, daemon boot, and reload refuse the configuration with an `opererr` error naming the peer (or `group <name>`), modifier `PREPEND`, `as-path-prepend`, RFC 7947 Section 2.2.2.1, and the concrete removal fix on the last line.

2. **Peer with `local-options no-prepend` and import `as-path-prepend` (`bgp-filter-modify`, AC-4)**
   - **Before:** `ze config validate`, `ze start`, and reload silently accept a peer declaring both `session { asn { local-options [ no-prepend ]; } }` and an active import `as-path-prepend` modifier, leaving two contradictory statements in force with no diagnostic.
   - **After:** Config validation, daemon boot, and reload refuse the configuration with an `opererr` error naming the peer (or `group <name>`), modifier `PREPEND`, `as-path-prepend`, and `local-options no-prepend`, explaining the contradiction and stating on the last line that one of the two must be removed.

3. **Unchanged valid configurations**
   - **Before and after:** Non-RS peers without `no-prepend`, peers with `local-options [ replace-as ]` alone, peers using `as-path-prepend` on export chains, peers with deactivated (`inactive`) import references, and route-server clients without import `as-path-prepend` continue to load and forward identically.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Strip ingress `as-path-prepend` edits later on the RS forwarding rail | Silently disagrees with the operator's accepted configuration and leaves RIB/Adj-RIB-In holding a rewritten path that differs from what clients receive. |
| Place both checks in core `internal/component/bgp/config/peers.go` | Breaks plugin ownership: `rs-client` is owned by `bgp-rs` (`ze-rs-conf.yang`, `TestRSConfigOwnedByPlugin`) and `as-path-prepend` is owned by `bgp-filter-modify`. Keeping each guard in its owning plugin via `ConfigReads` + `InProcessConfigVerifier` + `sdk.ConfigTx` honors "delete the folder and the feature vanishes". |
| Use a YANG leaf `ze:validate` (`ValidateFn`) | `ValidateFn` receives one path and one value and cannot compare sibling/cross-root settings (`policy > modify` vs `bgp` peer `session`). |
| Warn-only acceptance or pick a silent winner between `no-prepend` and import `as-path-prepend` | Leaves one of the operator's two explicit statements silently false; rejected in favor of fail-closed refusal (R-6). |
| Refuse `as-path-prepend` on all import chains or on peers lacking a `local-as` override | `as-path-prepend` on import is a legitimate general policy action; the original design explicitly preserves it outside these two invalid combinations. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- connect cross-root config delivery, `InProcessConfigVerifier`, and `FatalOnConfigError` for both plugins, and add failing refusal/wiring tests
   - Tests: `TestRouteServerRefusesInboundASPathPrepend`, `TestNoPrependRefusesInboundASPathPrepend`
   - Files: `internal/component/bgp/plugins/rs/register.go`, `server.go`, `config_tx.go`, `config_refusal_test.go`; `internal/component/bgp/plugins/filter_modify/register.go`, `config_tx.go`, `config_refusal_test.go`
   - Verify: both unit tests reach the real `config.VerifyPluginConfigContent`, `ConfigTx.Configure`, and `ConfigTx.Verify` entry points and fail before the guards are implemented.
2. **Phase: Refusal Guards** -- implement AC-3 in `bgp-rs` and AC-4 in `bgp-filter-modify`
   - Tests: `TestRouteServerRefusesInboundASPathPrepend`, `TestNoPrependRefusesInboundASPathPrepend`, plus existing `rs` and `filter_modify` package tests
   - Files: `internal/component/bgp/plugins/rs/config_tx.go`; `internal/component/bgp/plugins/filter_modify/modify.go`, `config.go`, `config_tx.go`
   - Verify: both refusal tests pass across all config entry points, inheritance levels, dynamic groups, and filter reference spellings; all permitted survivor cases pass; existing `rs` and `filter_modify` tests remain green.
3. **Phase: Functional Route-Server Proof** -- add `test/plugin/bgp-local-as-inbound-route-server.ci`
   - Tests: `test/plugin/bgp-local-as-inbound-route-server.ci` (plus existing `test/plugin/bgp-local-as-options.ci` and `test/plugin/bgp-local-as-inbound-untouched.ci`)
   - Files: `test/plugin/bgp-local-as-inbound-route-server.ci`
   - Verify: client 2 receives client 1's `AS_PATH` byte-identically on valid RS config, and the conflicting `rs-client` + import `as-path-prepend` config is refused at config load (proves A-3 and User Story 1).
4. **Phase: Guard Documentation and YANG Texts** -- update YANG descriptions/help and guard reference/guide docs
   - Tests: `./le doc check all`, `./le config coercion check`, `./le arch operator-error check`, `./le arch tier check`, `./le plugin boundary check`
   - Files: `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang`, `internal/component/bgp/yang/ze-bgp-conf.yang`, `docs/config-reference.md`, `docs/guide/configuration.md`, `docs/guide/plugins.md`
   - Verify: schema parses cleanly, `docs/config-reference.md` and guides match YANG texts, and all doc/coercion/error/tier gates pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Both AC-3 and AC-4 have implementations at file:line and concrete tests covering offline validate, Stage 2 configure, and reload verify |
| Feature completeness | Both user stories work end to end, including dynamic-group templates, inherited group settings, and peer-level overrides |
| Correctness | Global + group + peer import chains accumulate; peer-level `rs-client` and `local-options` override group-level values; deactivated refs are ignored; foreign-prefixed refs (`prefix-list:NAME`) are not falsely matched |
| Naming | Refusal messages name the peer (or `group <name>`), modify definition, `as-path-prepend`, and conflicting setting (`rs-client` + RFC 7947 Section 2.2.2.1, or `local-options no-prepend`) |
| Data flow | Neither guard touches `secondaryPrependAS`, `prependASPathOps`, or the forwarding hot path; `worker-queue-size` unchanged-subtree no-op and env override stay intact |
| Operator errors and doctor codes | Both refusals are built with `opererr` with the problem, why it matters, and the exact fix on the last line (`ai/rules/user-facing-errors.md`); no new `Warn`/`Error` log lines are added |
| Rule: `ai/rules/config.md` | Config values coerce string and native JSON forms (`configvalue.LeafList`, string/bool helper), passing `./le config coercion check` |
| Rule: `ai/rules/rfc-compliance.md` | Enforcing code in `rs/config_tx.go` carries the `// RFC 7947 Section 2.2.2.1:` quote; no `rfc/short/` row or conformance verdict changes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| AC-3 refusal wired and passing in `bgp-rs` | Main thread runs `./le test unit pkg path ./internal/component/bgp/plugins/rs/ run TestRouteServerRefusesInboundASPathPrepend` |
| AC-4 refusal wired and passing in `bgp-filter-modify` | Main thread runs `./le test unit pkg path ./internal/component/bgp/plugins/filter_modify/ run TestNoPrependRefusesInboundASPathPrepend` |
| Existing `rs`, `filter_modify`, `config`, and `plugin/all` tests stay green | Main thread runs `./le test unit pkg path ./internal/component/bgp/plugins/rs/ path ./internal/component/bgp/plugins/filter_modify/ path ./internal/component/config/ path ./internal/component/plugin/all/` |
| Route-server end-to-end proof and untouched local-as suites pass | Main thread runs `./le test bgp plugin bgp-local-as-inbound-route-server`, `./le test bgp plugin bgp-local-as-options`, and `./le test bgp plugin bgp-local-as-inbound-untouched` |
| Coercion, operator-error, tier, boundary, and doc gates pass | Main thread runs `./le config coercion check`, `./le arch operator-error check`, `./le arch tier check`, `./le plugin boundary check`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Malformed `bgp` or `policy` JSON sections fail closed with an error rather than returning empty maps and silently skipping the guards |
| Fail open | Both guards reject the config via `InProcessConfigVerifier`, Stage 2 `OnConfigure` (with `FatalOnConfigError: true`), and reload `OnConfigVerify`; never warn-and-continue and never silently drop the conflicting setting |
| Process-boundary parity | Both `bgp-rs` and `bgp-filter-modify` declare `policy` in Stage 1 `WantsConfig` as well as `registry.Registration.ConfigReads`, so forked plugins receive both roots identically to in-process plugins |

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
- On HEAD, `policy { modify }` lives under the top-level `policy` root while peers live under `bgp`. Both `bgp-filter-family` and `bgp-role` already solve cross-root validation through `ConfigRoots: ["bgp"]`, `ConfigReads: ["policy"]`, `WantsConfig: ["bgp", "policy"]`, `policy.SectionsParser`, `configjson.ForEachPeer`, and `InProcessConfigVerifier`.
- Registering `InProcessConfigVerifier` alongside `sdk.ConfigTx` guarantees that `ze config validate`, CLI/API commit, daemon Stage 2 configure, and live reload verify enforce the exact same refusal function with zero drift.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep AC-3 refusal in `bgp-rs` (`config_tx.go` + `InProcessConfigVerifier`) and AC-4 refusal in `bgp-filter-modify` (`config.go`/`config_tx.go` + `InProcessConfigVerifier`) | Put both checks in core `bgp/config/peers.go` or in a YANG `ValidateFn` | Preserves plugin ownership (`rs-client` belongs to `bgp-rs`, `as-path-prepend` belongs to `bgp-filter-modify`); `ValidateFn` cannot compare cross-root siblings. |
| Deliver `policy` to `bgp-rs` via `ConfigReads: ["policy"]` and `WantsConfig: ["bgp", "policy"]` without adding `policy` to `bgp-rs` `Dependencies` | Require `policy` as a hard dependency of `bgp-rs` | Matches `bgp-rpki` (`ConfigReads: ["pki"]` without depending on `pki`); a `policy`-only config never auto-loads `bgp-rs`, while any config with `bgp` delivers both roots whole across validate, boot, and reload. |
| Set `FatalOnConfigError: true` on `bgp-rs` and `bgp-filter-modify` | Leave `FatalOnConfigError: false` so boot logs an error and runs without the plugin | Matches `bgp-gr`, `bgp-rpki`, and `bgp-llnh`: a Stage 2 config refusal fails boot (`*rpc.RPCCallError` only, never transport errors) rather than running silently without the route server or attribute modifier. |
| Refuse fail-closed with `opererr` at config load rather than stripping edits on the RS rail or picking a winner | Runtime RS strip, warn-only acceptance, or silent precedence | Avoids silent divergence between config and wire behavior; names exact offending settings and removal fix. |

## Known Limitations
- The deferred RFC 7705 Section 3.3 inbound append configuration recipe, its width regression proof, and FRR interop remain post-release in `plan/post-release/features/spec-bgp-local-as-inbound-append.md` (owner decision of 2026-09-06, reaffirmed 2026-11-24).
- `RFC7705-3.3-9` remains an unimplemented SHOULD; `local-options no-prepend` continues to select no automatic engine suppression and gains only the AC-4 contradiction guard here.
- General import `as-path-prepend` on peers without `rs-client` and without `local-options no-prepend` (including peers with no `local-as` override) remains permitted by design.

## RFC Documentation (Scope: protocol)

Add `// RFC 7947 Section 2.2.2.1: "..."` above the AC-3 enforcing code in `internal/component/bgp/plugins/rs/config_tx.go`.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 7947 Section 2.2.2.1 | A route server (`session { rs-client true; }`) SHOULD NOT prepend its own AS number to `AS_PATH` nor modify `AS_PATH` in any other way; combining `rs-client true` with an active import `as-path-prepend` modifier is refused at config load, and valid RS peers relay `AS_PATH` byte-identically | Export-chain `as-path-prepend`, non-`rs-client` import `as-path-prepend`, deactivated import refs, and existing route-requested RS large-community prepends (`wireu.CommunityPolicy.RSPrepend`) remain preserved | `TestRouteServerRefusesInboundASPathPrepend`, `test/plugin/bgp-local-as-inbound-route-server.ci` | `TestRouteServerRefusesInboundASPathPrepend` (permitted survivor subcases) |
| RFC 7705 Section 3.3 | A peer declaring `session { asn { local-options [ no-prepend ]; } }` states that inbound prepending is suppressed; pairing it with an active import `as-path-prepend` modifier is refused at config load as contradictory | `local-options [ replace-as ]` alone, peers without `no-prepend`, export-chain `as-path-prepend` on `no-prepend` peers, and existing outbound `secondaryPrependAS` behavior remain preserved | `TestNoPrependRefusesInboundASPathPrepend`, `test/plugin/bgp-local-as-inbound-untouched.ci` | `TestNoPrependRefusesInboundASPathPrepend` (permitted survivor subcases), `test/plugin/bgp-local-as-options.ci` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions and Permitted Survivors and Why
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks every row Yes / No / N-A with reasons

### Goal Gates (MUST pass)
- [ ] AC-3 and AC-4 both demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches; GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here (`plan/post-release/features/spec-bgp-local-as-inbound-append.md`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A — no new numeric inputs)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A — local config-load refusals and unchanged RS forwarding; FRR recipe interop stays in sibling spec)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-import-prepend-guards.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

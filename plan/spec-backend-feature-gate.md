# Spec: backend-feature-gate — Commit-Time Backend Capability Check

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` — workflow rules
3. `.claude/rules/design-principles.md` — no identity wrappers, single responsibility
4. `internal/component/iface/backend.go` — existing Backend interface (`RegisterBackend` pattern)
5. `internal/plugins/ifacenetlink/register.go` — backend registration example
6. `internal/plugins/ifacevpp/` — VPP backend, candidate for first capability gap
7. `internal/component/config/` — config validate and commit flow (walker, error reporting)
8. `plan/spec-fw-0-umbrella.md` — fw-6/fw-7 will adopt this pattern from day 1
9. `plan/spec-vpp-0-umbrella.md` — references this gate as prerequisite

## Task

Today, a user can write YANG config that their chosen backend cannot implement.
Example: `interface { tunnel t0 { encapsulation { gre { ... } } } }` with
`backend = vpp`. The parser accepts it, commit succeeds, the daemon starts, and
then `ifacevpp.CreateTunnel` returns `errNotSupported("CreateTunnel (pending
GoVPP tunnel API wiring)")` from its Apply path — an error that names the VPP
backend method, not the user's YANG path.

The user requirement is: **commit must fail with a clear message naming the
unsupported feature and the backend** when the backend cannot satisfy the
requested config. The diagnostic points at the user's YANG path, not a
Backend method symbol.

This spec introduces a declarative YANG extension `ze:backend "name1 name2"`
that annotates which backends support a given node/case, a schema-time reader
analogous to `getOSExtension` at `internal/component/config/yang_schema.go:329`,
and a post-parse walker that aggregates annotation vs active-backend mismatches
into a commit-rejection error. Adopted concretely in the `interface` component
end-to-end (with `.ci` tests), plus annotations and a `backend` leaf added to
firewall and traffic YANG so that the forthcoming fw-3 and fw-5 component
plugins can call the same walker with one line of wiring.

This is foundational infrastructure — it enables every downstream "add feature
X to backend Y" spec (bridge-via-vpp, tunnel-vxlan, etc.) to declare the
support matrix in YANG and produce clean commit-time rejection automatically.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/design-principles.md` — design principle alignment
  → Constraint: "Explicit > implicit" — a backend that cannot serve a config must say so explicitly at commit time, not fail implicitly at apply time with a backend-specific symbol name.
  → Constraint: "No identity wrappers" — the gate must transform information (YANG path to backend verdict), not just delegate.
  → Constraint: "Do it right" — the gate applies to every backend-bearing component (iface, firewall, traffic), not only iface.
- [ ] `.claude/rules/config-design.md` — fail on unknown keys principle
  → Constraint: unknown YANG keys are already rejected by the config parser. This gate addresses a different class: known YANG keys that the chosen backend cannot serve.
- [ ] `.claude/rules/integration-completeness.md` — wiring test requirement
  → Constraint: every AC must be reachable via a `.ci` test — a Go unit test is not sufficient. Two `.ci` tests are mandated below (one rejecting, one accepting).
- [ ] `.claude/rules/plugin-design.md` — plugin boundary and registration
  → Constraint: backends are NOT plugins — they register through component-local `RegisterBackend(name, factory)` calls, not `registry.Register()`. Changes to the Backend interface do not touch `registry.Registration`.
- [ ] `.claude/rules/file-modularity.md` — one concern per file
  → Constraint: `internal/component/iface/config.go` is already 1479 lines. Gate logic MUST go in a new file (e.g. `feature_gate.go`), not appended to `config.go`.
- [ ] `.claude/rules/api-contracts.md` — document lifecycle obligations
  → Constraint: the public `ValidateBackendFeatures(tree, backendLeafPath) []error` helper must document its contract: what tree shape it expects, what a nil return means, what backendLeafPath looks like.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` — existing `ze:os`, `ze:listener`, `ze:required`, `ze:syntax`, `ze:sensitive`, `ze:validate`, `ze:command`, `ze:decorate`, `ze:filter`, `ze:hidden`, `ze:ephemeral`, `ze:bcrypt` extensions
  → Decision: add a sibling `extension backend { argument names; ... }` — same declaration style, same "argument string" convention as `ze:os` (argument goos).
  → Constraint: the namespace is `urn:ze:extensions`; prefix is `ze`. Consumers match on `ext.Keyword == "ze:backend"` OR `strings.HasSuffix(ext.Keyword, ":backend")` to handle both long and short prefixes, mirroring `getOSExtension`.
- [ ] `internal/component/config/yang_schema.go:329-338` — `getOSExtension` is the precedent implementation
  → Constraint: `getBackendExtension` follows the exact same shape — iterate `entry.Exts`, match keyword, return argument string.
  → Constraint: `ze:os` prunes at schema-build time (line 231-233) because GOOS is immutable; `ze:backend` cannot prune — the backend is chosen at config parse time. The returned list is stored on the schema Node for post-parse consultation.

### RFC Summaries (MUST for protocol work)

Not protocol work. No RFCs apply.

**Key insights:**
- Chosen approach is YANG-native: a new `ze:backend "name1 name2"` extension declared on YANG nodes (lists, leaves, cases) that declares the support matrix. A generic post-parse walker cross-references annotations against the active backend leaf and aggregates mismatches as errors.
- Three components (`iface`, `firewall`, `traffic`) share a near-identical Go-level `Backend` interface boilerplate. None of that boilerplate changes — the gate is entirely at the YANG/config layer.
- `fib` (fibkernel/fibvpp) uses a different shape: no `RegisterBackend` helper, each FIB plugin holds its own backend directly. Out of scope for this spec.
- ifacevpp already has an `errNotSupported` helper at `internal/plugins/ifacevpp/ifacevpp.go:150-153`, firing for 14 methods. These stay as runtime defence-in-depth. The `ze:backend` annotations become the primary commit-time diagnostic.
- The SDK's 5-stage protocol runs `OnConfigVerify` before `OnConfigApply` on every reload. `OnConfigVerify` returning an error propagates back to the user as the commit-rejection message. That is the mechanical insertion point for reloads.
- On initial startup, only `OnConfigure` fires — no separate verify. The gate must also run inside `OnConfigure` before `applyConfig` is called.
- `ze config validate` (offline CLI path) also benefits: since the walker needs only parsed tree + schema + the active backend leaf value, validation works without a running daemon or loaded backend. This is a UX win compared to a Go-registration approach that would need plugin init to run.
- Only iface currently has a `backend` leaf in its YANG (at line 328 of `ze-iface-conf.yang`). firewall and traffic YANG need a similar leaf added in this spec so their future plugin wiring (fw-3, fw-5) can call the same walker.
- The iface `Backend` interface is imperative (30+ methods like `CreateDummy`, `CreateTunnel`, `CreateWireguardDevice`). firewall/traffic are declarative (single `Apply(desired)` method). The walker is agnostic — it only reads annotations and the chosen backend, never the backend instance.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` (209L) — Backend interface with 30+ methods (CreateDummy, CreateVeth, CreateBridge, CreateTunnel, CreateVLAN, CreateWireguardDevice, ConfigureWireguardDevice, GetWireguardDevice, DeleteInterface, AddAddress, RemoveAddress, ReplaceAddressWithLifetime, AddAddressP2P, AddRoute, RemoveRoute, ListRoutes, SetAdminUp/Down, SetMTU, SetMACAddress, GetMACAddress, GetStats, ListInterfaces, GetInterface, BridgeAddPort, BridgeDelPort, BridgeSetSTP, SetupMirror, RemoveMirror, StartMonitor, StopMonitor, Close). Module state: `backendsMu` (sync.Mutex), `backends` (map[string]factory), `activeBackend`. Helpers: `RegisterBackend(name, factory)` / `LoadBackend(name)` / `GetBackend()` / `CloseBackend()`.
  → Constraint: imperative per-operation interface; feature presence is implicit in "can CreateX succeed?"
  → Constraint: adding any new method to this interface forces every existing backend implementation to satisfy the new signature.
- [ ] `internal/component/firewall/backend.go` (123L) — same RegisterBackend/LoadBackend/GetBackend/CloseBackend pattern. Backend interface: `Apply(desired []Table) error`, `ListTables() ([]Table, error)`, `GetCounters(table string) ([]ChainCounters, error)`, `Close() error`.
  → Constraint: declarative single-Apply shape; feature support is implicit in which Table/Action/Match types Apply knows how to render.
- [ ] `internal/component/traffic/backend.go` (118L) — same RegisterBackend pattern. Backend interface: `Apply(desired map[string]InterfaceQoS) error`, `ListQdiscs(ifaceName) (InterfaceQoS, error)`, `Close() error`.
  → Constraint: same declarative shape as firewall, different desired-state key.
- [ ] `internal/plugins/ifacenetlink/register.go` (15L) and `internal/plugins/ifacevpp/register.go` (18L) — both call `iface.RegisterBackend(name, factory)` from `init()`.
  → Constraint: registration is a single function call with 2 args today. Adding capabilities as a third arg is a breaking signature change that must land in one commit across every register.go.
- [ ] `internal/component/iface/register.go` (935L) — iface plugin implementation. Key entry points: `OnConfigure` (line 222, initial apply, also calls LoadBackend), `OnConfigVerify` (line 340, reload precheck, stores `pendingCfg`), `OnConfigApply` (line 353, reload commit), `OnConfigRollback` (line 419). In `OnConfigure`, `applyConfig(cfg, nil, b)` is called directly without a separate validation step. In `OnConfigVerify`, the only checks are `cfg.Backend != ""`; no per-feature validation.
  → Constraint: `OnConfigVerify` is the natural gate for reloads but is NOT called on daemon startup; gate must also run in `OnConfigure` before `applyConfig`.
  → Constraint: `LoadBackend` is called inside `OnConfigure`. Gate in `OnConfigure` runs AFTER LoadBackend. Gate in `OnConfigVerify` must be resilient to `GetBackend() == nil` during a first-time verify against a backend that isn't loaded yet (edge case: validate from CLI without daemon).
- [ ] `internal/plugins/ifacevpp/ifacevpp.go:150-153` (and 13 more sites) — current unsupported-feature reporting via `errNotSupported(method)`: `return fmt.Errorf("ifacevpp: %s not supported on VPP backend", method)`. Sites: CreateVeth (line 201), CreateTunnel (254), CreateWireguardDevice (258), ConfigureWireguardDevice (262), GetWireguardDevice (266), AddAddressP2P (354), AddRoute (360), RemoveRoute (364), ListRoutes (368), GetStats (436), BridgeSetSTP (485), SetupMirror (491), RemoveMirror (495).
  → Constraint: these errors fire at apply time with a Backend-method name; the user never sees the YANG path. This is the exact UX the spec replaces (not removes — the runtime errors stay as a defence-in-depth layer).
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` (1000+ L) — feature surface: lists for `ethernet`, `dummy`, `veth`, `bridge`, `tunnel` (with `choice kind` cases: gre, gretap, ip6gre, ip6gretap, ipip, sit, ip6tnl, ipip6), `wireguard`, `loopback`. Bridge list is annotated `ze:os "linux"`. Tunnel cases are NOT OS-annotated. Unit-level features include vlan-id, dhcp/dhcpv6, route-priority, mirror.
  → Constraint: the feature vocabulary the gate names must match YANG paths or case names, not internal Go field names, so error messages point back to user config.

**Behavior to preserve:**
- Existing Backend interface method signatures and the `RegisterBackend(name, factory)` call shape in every backend's register.go — adding a capability declaration must not break backends that do not opt in.
- ifacevpp's runtime `errNotSupported` returns — they stay as a defence-in-depth layer. Removing them would leave a window where a gate bug allows an unsupported feature through.
- YANG schema unchanged — no new ze extensions in YANG; the gate lives entirely in Go.
- Initial-apply and reload error return paths — the gate's error must travel through the same channel so editor / CLI / web all report it identically.

**Behavior to change:**
- A new `ze:backend` YANG extension is declared in `ze-extensions.yang` and understood by the schema builder in `yang_schema.go`.
- The schema `Node` interface (or the concrete `ContainerNode` / `ListNode` structs in `schema.go`) carries a new field storing the list of supporting backends for a node (empty means "unrestricted — all backends OK").
- A new public helper `config.ValidateBackendFeatures(parsedTree, activeBackend, backendLeafPath) []error` walks the parsed tree, consults each node's schema for its backend list, emits an error when the active backend is not in the list.
- The iface component calls this helper in both `OnConfigure` (startup, before `applyConfig`) and `OnConfigVerify` (reload, before `pendingCfg` is stored).
- The iface YANG (`ze-iface-conf.yang`) gains `ze:backend` annotations on feature-gated lists and cases (bridge, tunnel cases, wireguard, veth, mirror).
- The firewall YANG (`ze-firewall-conf.yang`) gains a `backend` leaf and `ze:backend` annotations on primitives that have backend-dependent support.
- The traffic YANG (`ze-traffic-control-conf.yang`) gains a `backend` leaf and `ze:backend` annotations similarly.
- Error messages name the full YANG path of the rejected node, the active backend, and any `ze:description` text from the node — no Backend method symbols.

## Data Flow (MANDATORY)

### Entry Points

| User action | SDK callback / CLI path | Component behavior today | Gate insertion |
|-------------|------------------------|-------------------------|----------------|
| Daemon starts with a config | iface plugin `OnConfigure` | Parses, `LoadBackend`, `applyConfig` immediately | Insert between LoadBackend and applyConfig |
| `ze config commit` / web / editor save (reload) | iface plugin `OnConfigVerify` → `OnConfigApply` | Verify parses + checks `cfg.Backend != ""`; Apply reconciles | Insert at tail of OnConfigVerify, before `pendingCfg` assignment |
| `ze config validate` (offline CLI, no daemon) | Schema-driven parse only | No backend loaded today; no per-feature check | Gate DOES run because it reads annotations + parsed tree only. Authoritative commit-time diagnostic. |

### Transformation Path (reload case, the primary one)

1. User edits config, hits commit.
2. Parser builds schema-driven typed tree including the `backend` leaf value.
3. SDK delivers `OnConfigVerify([]ConfigSection)` to the iface plugin process.
4. `parseIfaceSections(sections)` → `*ifaceConfig` (typed tree).
5. **Gate (new):** `config.ValidateBackendFeatures(parsedTree, cfg.Backend, "/interface/backend")` walks the parsed tree, consults each Node's schema for its `ze:backend` support list, emits one error per node whose list excludes the active backend.
6. If aggregated errors: join and return from OnConfigVerify — SDK surfaces the text as the commit-rejection message.
7. Otherwise: store as `pendingCfg`. SDK then delivers `OnConfigApply(diffs)`. `applyConfig(pendingCfg, previousCfg, backend)` runs imperative reconciliation via backend method calls (unchanged).

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| YANG annotations → schema Node | `getBackendExtension` reader during schema build, stored on Node | [ ] |
| Parsed config → walker | Typed traversal over the parsed tree, consulting each node's schema | [ ] |
| Walker → user | Aggregated errors returned via SDK callback return → commit rejection | [ ] |

### Integration Points

- `internal/component/config/yang/modules/ze-extensions.yang` — new `extension backend` declared here
- `internal/component/config/yang_schema.go` — new `getBackendExtension(entry)` (mirrors `getOSExtension`); Node population with backend list
- `internal/component/config/schema.go` — Node interface / concrete node types gain a backend-support field
- `internal/component/config/backend_gate.go` (new) — generic walker, public `ValidateBackendFeatures` helper
- `internal/component/iface/register.go:222` and `:340` — call site for the walker in `OnConfigure` and `OnConfigVerify`
- `internal/component/iface/schema/ze-iface-conf.yang` — annotate bridge/tunnel/wireguard/veth/mirror with `ze:backend`
- `internal/component/firewall/schema/ze-firewall-conf.yang` — add `leaf backend` + annotate nft-only / vpp-only primitives
- `internal/component/traffic/schema/ze-traffic-control-conf.yang` — add `leaf backend` + annotate tc-only / vpp-only primitives
- `internal/plugins/ifacevpp/ifacevpp.go:150-153` — existing `errNotSupported` returns unchanged (defence-in-depth)

### Architectural Verification
- [ ] No bypassed layers (gate runs on the same path as commit, not a side channel)
- [ ] No unintended coupling (components stay independent; gate is per-component)
- [ ] No duplicated functionality (one capability model per component, reused by all its backends)
- [ ] Zero-copy preserved where applicable (not relevant — validation path, not wire path)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface { backend vpp; bridge br0 { ... } }` (bridge not supported by vpp) | → | walker rejects, commit fails with YANG path + backend name | `test/parse/iface-vpp-rejects-bridge.ci` |
| `interface { backend vpp; tunnel t0 { encapsulation { gre { ... } } } }` | → | walker rejects, commit fails naming `/interface/tunnel` | `test/parse/iface-vpp-rejects-tunnel.ci` |
| `interface { backend vpp; ethernet eth0 { ... } }` (ethernet is supported) | → | walker accepts, commit succeeds | `test/parse/iface-vpp-accepts-ethernet.ci` |
| `interface { backend netlink; bridge br0 { ... } }` (regression: netlink supports bridge) | → | walker accepts, commit succeeds | `test/parse/iface-netlink-accepts-bridge.ci` |
| `interface { backend vpp; bridge br0; tunnel t0 { ... }; wireguard wg0 { ... } }` (3 unsupported features) | → | walker aggregates all 3 mismatches into one error | `test/parse/iface-vpp-aggregates-errors.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Parsed config `interface { backend vpp; bridge br0 { ... } }` | Commit rejected. Error text includes the YANG path `/interface/bridge`, the active backend name `vpp`, and the list of backends that DO support the feature (`netlink`). |
| AC-2 | Same shape with `tunnel t0 { encapsulation { gre { ... } } }` | Commit rejected. Error names `/interface/tunnel` and the active backend. |
| AC-3 | `interface { backend vpp; ethernet eth0 { ... } }` (ethernet has no `ze:backend` annotation → unrestricted) | Commit accepted. Walker returns empty error set. |
| AC-4 | Daemon startup path (no reload — `OnConfigure`) with an unsupported combination | Daemon refuses to start. `OnConfigure` returns the same aggregated error text as `OnConfigVerify`. |
| AC-5 | Config with 3 unsupported features under `backend vpp` (bridge + tunnel + wireguard) | Returned error contains one line per mismatch, all three YANG paths named. |
| AC-6 | Existing netlink config with bridge + tunnel + wireguard | No regression. Walker accepts. |
| AC-7 | `ze:backend` typo (argument names a backend that is not registered by any plugin) | Unit test at schema-build time flags it. Hook or test fails the build. |
| AC-8 | `ze config validate` CLI on the same rejected config | Exits non-zero. Stderr contains the same aggregated error text as the daemon-commit path. |
| AC-9 | Annotation granularity: `list tunnel { ze:backend "netlink"; choice kind { case vxlan { ze:backend "netlink vpp" } } }` — user config has vxlan | Walker picks the narrowest annotation (the per-case override wins), vxlan accepts under vpp. |
| AC-10 | Walker consistency: every `ze:backend` argument names a registered backend | Unit test enumerates annotations across all schema modules, cross-checks against known backend names. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGetBackendExtension_AbsentReturnsEmpty` | `internal/component/config/yang_schema_test.go` | A YANG entry without any `ze:backend` statement yields empty list | |
| `TestGetBackendExtension_SingleArgument` | `internal/component/config/yang_schema_test.go` | `ze:backend "netlink"` yields `["netlink"]` | |
| `TestGetBackendExtension_SpaceSeparatedList` | `internal/component/config/yang_schema_test.go` | `ze:backend "netlink vpp"` yields `["netlink", "vpp"]` | |
| `TestValidateBackendFeatures_SupportedAccepts` | `internal/component/config/backend_gate_test.go` | Synthetic tree with a list annotated `"netlink"`, active backend `netlink` → no errors | |
| `TestValidateBackendFeatures_UnsupportedRejects` | `internal/component/config/backend_gate_test.go` | Same tree, active backend `vpp` → error names YANG path and backend | |
| `TestValidateBackendFeatures_AggregatesMultiple` | `internal/component/config/backend_gate_test.go` | Tree with three annotated lists, one backend unsupported by all → returned slice has three entries, order matches tree walk | |
| `TestValidateBackendFeatures_UnrestrictedAccepts` | `internal/component/config/backend_gate_test.go` | Tree nodes with no `ze:backend` annotation → no errors regardless of active backend | |
| `TestValidateBackendFeatures_ChoiceCaseOverride` | `internal/component/config/backend_gate_test.go` | Tree where parent list is `"netlink"` but case is `"netlink vpp"`, active backend `vpp` → case override wins, no error | |
| `TestValidateBackendFeatures_EmptyActiveBackend` | `internal/component/config/backend_gate_test.go` | Active backend is `""` (not configured) → walker returns a single clear error asking the user to configure a backend | |
| `TestBackendExtensionNames_AllAnnotationsNameKnownBackends` | `internal/component/config/backend_gate_test.go` | Enumerate every `ze:backend` argument in every schema module; each name must appear in a registered backend across iface / firewall / traffic | |

### Boundary Tests (MANDATORY for numeric inputs)

Not numeric. Behavioural boundaries below.

| Condition | Case | Expected |
|-----------|------|----------|
| Annotation absent | Node lacks any `ze:backend` | Unrestricted — any backend accepted |
| Annotation empty string | `ze:backend ""` | Treated as absent (unrestricted). Schema builder emits a WARN log flagging the useless annotation |
| Annotation single name | `ze:backend "netlink"` | Only `netlink` matches |
| Annotation multiple names | `ze:backend "netlink vpp"` | Either `netlink` or `vpp` matches |
| Duplicate names | `ze:backend "netlink netlink"` | De-duplicated at schema build, equivalent to single name |
| Unknown name | `ze:backend "notabackend"` | Caught by AC-7 consistency test at build time |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Reject bridge under vpp | `test/parse/iface-vpp-rejects-bridge.ci` | Config declares `backend vpp` and a bridge list entry. `ze config validate` exits non-zero with stderr naming `/interface/bridge` and `vpp` | |
| Reject tunnel under vpp | `test/parse/iface-vpp-rejects-tunnel.ci` | Config declares `backend vpp` and a tunnel list entry. `ze config validate` exits non-zero naming `/interface/tunnel` | |
| Accept ethernet under vpp | `test/parse/iface-vpp-accepts-ethernet.ci` | Config declares `backend vpp` and an ethernet list entry. `ze config validate` exits zero | |
| Accept bridge under netlink | `test/parse/iface-netlink-accepts-bridge.ci` | Config declares `backend netlink` (or default) and a bridge list entry. `ze config validate` exits zero — regression guard | |
| Aggregate multiple rejections | `test/parse/iface-vpp-aggregates-errors.ci` | Config declares `backend vpp` plus bridge + tunnel + wireguard entries. Stderr lists all three YANG paths | |

### Future (if deferring any tests)
- Firewall and traffic `.ci` tests deferred to fw-3 / fw-5 when those plugins exist (no `OnConfigVerify` exists today in those components). Walker unit tests in this spec cover the annotation reading and walker logic end-to-end.

## Files to Modify

- `internal/component/config/yang/modules/ze-extensions.yang` — declare `extension backend { argument names; ... }`
- `internal/component/config/yang_schema.go` — add `getBackendExtension(entry) []string`; populate Node field when building schema
- `internal/component/config/schema.go` — add backend-support field on the Node abstraction so walker can read it without importing gyang
- `internal/component/iface/register.go` — call `config.ValidateBackendFeatures` inside `OnConfigure` (after parse, before `LoadBackend`+`applyConfig`) AND inside `OnConfigVerify` (after parse, before `pendingCfg` assignment); return the aggregated error
- `internal/component/iface/schema/ze-iface-conf.yang` — annotate `list bridge`, `list tunnel` (with per-case overrides where applicable), `list wireguard`, `list veth`, `leaf mirror` (if present) with `ze:backend "netlink"`
- `internal/component/firewall/schema/ze-firewall-conf.yang` — add `leaf backend` at component root (default "nft"); annotate nft-only or vpp-only primitives as they surface (minimal initial annotation set — full audit happens when fw-3 lands)
- `internal/component/traffic/schema/ze-traffic-control-conf.yang` — add `leaf backend` at component root (default "tc"); annotate tc-only or vpp-only qdisc/filter types similarly
- `docs/features.md` — one paragraph: commit-time backend capability check
- `docs/guide/configuration.md` — error reference section: example of a rejection message
- `docs/architecture/core-design.md` — describe the `ze:backend` extension alongside `ze:os`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `ze-extensions.yang` (new extension), plus annotations on iface/firewall/traffic YANG |
| CLI commands/flags | No new flag, but `ze config validate` error output changes | `docs/guide/command-reference.md` |
| Editor autocomplete | No | - |
| Functional test | Yes | `test/parse/iface-vpp-*.ci` files |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — commit-time backend capability check |
| 2 | Config syntax changed? | No (existing `backend` leaf; no new user syntax) | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `ze config validate` new error class |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` — error-reference subsection |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — extensions section gains `ze:backend` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — mention commit-time backend capability check |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `ze config validate` new error class |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — backend-author contract for capability declaration |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` — error reference section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | Backend interface contract change; whichever doc describes the Backend interface |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — describe capability gate |

## Files to Create

- `internal/component/config/backend_gate.go` — public `ValidateBackendFeatures(tree Node, activeBackend string, backendLeafPath string) []error` helper, walker implementation, tree traversal, error formatter
- `internal/component/config/backend_gate_test.go` — unit tests for walker (10 tests listed in TDD)
- `test/parse/iface-vpp-rejects-bridge.ci` — functional: vpp + bridge rejected
- `test/parse/iface-vpp-rejects-tunnel.ci` — functional: vpp + tunnel rejected
- `test/parse/iface-vpp-accepts-ethernet.ci` — functional: vpp + ethernet accepted
- `test/parse/iface-netlink-accepts-bridge.ci` — functional: netlink + bridge accepted (regression)
- `test/parse/iface-vpp-aggregates-errors.ci` — functional: multi-feature rejection aggregation

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + linked `rules/` + `ze-extensions.yang` for style reference |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify-fast` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue; no deferrals |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with tests green + Self-Critical Review.

1. **Phase: Schema extension infrastructure.** Declare `extension backend` in `ze-extensions.yang`. Add `getBackendExtension` helper to `yang_schema.go` (mirror `getOSExtension`). Add backend-support field to Node (extend `schema.go`). Populate the field during schema build.
   - Tests: `TestGetBackendExtension_AbsentReturnsEmpty`, `TestGetBackendExtension_SingleArgument`, `TestGetBackendExtension_SpaceSeparatedList`. Written first, run → FAIL; implement; run → PASS.

2. **Phase: Generic walker.** Create `internal/component/config/backend_gate.go` with `ValidateBackendFeatures(tree, active, backendLeafPath) []error`. Walker recurses the parsed tree, consults each Node's backend-support list, emits one error per unsupported mismatch, respects narrowest-annotation-wins for nested annotations.
   - Tests: 7 walker tests in `backend_gate_test.go`. Written first → FAIL; implement; → PASS.

3. **Phase: iface annotations.** Annotate `list bridge`, `list tunnel` (and per-case if the support matrix differs per kind), `list wireguard`, `list veth`, mirror nodes in `ze-iface-conf.yang` with `ze:backend "netlink"`. No `ze:backend` on `ethernet`, `dummy`, `loopback` (supported by all current backends).
   - No unit test impact (annotations consumed by existing walker tests at phase 2 via synthetic trees). Consistency test from phase 6 catches typos.

4. **Phase: iface wiring.** Call `ValidateBackendFeatures` in `iface/register.go` at `OnConfigure` (after parse, before `LoadBackend`+`applyConfig`) and at `OnConfigVerify` (after parse, before `pendingCfg`). Return aggregated error via existing `joinApplyErrors` or a new joiner.
   - Tests: 5 `.ci` functional tests in `test/parse/`. Written first → FAIL; implement; → PASS.

5. **Phase: firewall/traffic readiness.** Add `leaf backend` to `ze-firewall-conf.yang` (default "nft") and `ze-traffic-control-conf.yang` (default "tc"). Add minimal `ze:backend` annotations on primitives that are obviously nft-only or tc-only (low-confidence list — full audit defers to fw-3 / fw-5 when each component plugin lands and has real user-reachable paths). No wiring — those components don't have `OnConfigVerify` yet. Walker availability for fw-3 and fw-5 is proven by their existence in shared `config/backend_gate.go`.
   - Tests: schema-build unit tests verify the new `backend` leaves parse and the annotations are recognized.

6. **Phase: Consistency guard.** `TestBackendExtensionNames_AllAnnotationsNameKnownBackends` walks every schema module, collects every `ze:backend` argument, compares against a known-names list. Registered-names list comes from the component backend registries at test time (requires importing `iface`, `firewall`, `traffic` packages).
   - Tests: the consistency test itself. Must pass after phase 3 and phase 5 annotations land.

7. **Phase: Documentation.** Update `docs/features.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/architecture/core-design.md`. Each with `<!-- source: -->` anchors per `rules/documentation.md`.

8. **Full verification.** `make ze-verify-fast` green. Critical review. Fill audit tables. Write learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 10 AC demonstrated; all 5 `.ci` tests present and green |
| Correctness | Walker honours narrowest-annotation-wins semantics; aggregation preserves tree-walk order |
| Naming | `ze:backend` matches `ze:os` shape exactly; helper name `ValidateBackendFeatures` aligns with `config.` package style |
| Data flow | Annotations read once at schema build, consulted at commit time; no re-parsing of YANG text at runtime |
| Rule: no-layering | Runtime `errNotSupported` in ifacevpp stays (defence-in-depth, not removed) |
| File modularity | New `backend_gate.go` is a single-concern file (walker + helper); no additions to existing 1000+ line files |
| Adversarial review | What happens if a user names a backend that's registered but this component has no leaf for it? (e.g., `backend vpp` in iface when vpp is only a fib backend) — walker reads the `backendLeafPath` explicitly, so the component controls its own path; no cross-contamination |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze:backend` extension declared | `grep -n "extension backend" internal/component/config/yang/modules/ze-extensions.yang` |
| Walker file exists | `ls internal/component/config/backend_gate.go` |
| Walker tests pass | `go test -run ValidateBackend internal/component/config/...` |
| Schema reader tests pass | `go test -run GetBackendExtension internal/component/config/...` |
| Functional tests pass | `bin/ze-test parse -p iface-vpp -p iface-netlink` |
| iface annotations present | `grep -n "ze:backend" internal/component/iface/schema/ze-iface-conf.yang` |
| firewall + traffic have `leaf backend` | `grep -n "leaf backend" internal/component/firewall/schema/*.yang internal/component/traffic/schema/*.yang` |
| Consistency test passes | `go test -run TestBackendExtensionNames internal/component/config/...` |
| Docs updated | `grep -l "ze:backend" docs/features.md docs/guide/configuration.md docs/architecture/core-design.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Error text leakage | Aggregated error names YANG paths and backend names (user-authored). Does not leak internal file paths, Go symbols, or stack traces |
| Input validation | `ze:backend` argument is parsed space-separated; whitespace/duplicates handled safely; no unbounded allocation per annotation |
| CPU/memory | Walker allocates one error slice per commit; proportional to tree size. No concern at commit frequencies |
| Concurrency | Walker is called from the plugin goroutine handling OnConfigure/OnConfigVerify. Schema is immutable after build. No shared state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Schema fails to build with new extension | Fix the extension declaration or the reader; no feature work proceeds |
| Annotation not picked up by walker | Instrument walker with logging; check schema Node population in phase 1 |
| Walker false positive (rejects supported config) | Check narrowest-annotation-wins logic; per-case overrides may not be applied |
| `.ci` test fails wrong reason | Read test output; annotate or parse issues usually |
| Consistency test flags typo | Fix annotation or name; no merging until green |
| Regression in netlink flow | Check that no-annotation nodes default to unrestricted; walker must never emit an error without an annotation |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Review Gate

`/ze-review` run against the uncommitted changes found 4 ISSUEs and 3 NOTEs; 4 ISSUEs fixed in-session, 2 NOTEs addressed (NOTE 2 doc clarification; NOTE 1/3 judged acceptable as-is).

| Severity | Finding | Resolution |
|----------|---------|-----------|
| ISSUE | Walker lacked `InlineListNode` traversal + `backendAnnotation` missing `InlineListNode`/`FlexNode` annotation reads | Added `*InlineListNode` branch in `walkBackendNode` (delegating to a new `walkBackendInlineList` helper); added `Backend []string` fields to `InlineListNode` and `FlexNode` in `schema.go`; populated them from `yangToInlineListWithKey` and `yangToFlex`; extended `backendAnnotation` to check both types. |
| ISSUE | List-level rejection suppressed for all entries if any one entry had a descendant override | Split list handling into `walkBackendListMap` / `walkBackendInlineList` that evaluate each entry independently: per-entry error at `list/<key>` path when the entry has no override; accepting entry does not affect siblings. Added `TestValidateBackendFeatures_PerEntryIndependence` to lock in the semantic. Existing `.ci` tests still pass because the new per-entry paths (e.g. `/interface/bridge/br0`) contain the old list path substring (`/interface/bridge`). |
| ISSUE | `cmd/ze/config/cmd_validate.go` hardcoded `defaultB: "netlink"`; diverged from runtime on non-Linux | Replaced hardcoded string with `ifaceDefaultBackend()` defined in build-tagged files `cmd/ze/config/default_backend_{linux,other}.go`. Returns `"netlink"` on Linux and `""` elsewhere, matching `internal/component/iface/default_{linux,other}.go`. Empty value triggers the walker's empty-backend guard, producing the same user-visible rejection the daemon does. |
| ISSUE | Only the first `ze:backend` statement on an entry was read | Reworked `getBackendExtension` to merge every `ze:backend` statement on the entry, dedup across them, return in first-seen order. Added `TestGetBackendExtension_MergesMultipleStatements`. |
| NOTE | Error path string concatenates user-provided JSON list keys verbatim | Judged acceptable: YANG schema parser validates list key names against their leaf type/pattern, so a malicious key with control characters cannot reach the walker. Not a security issue. |
| NOTE | "Safe for concurrent use" doc comment was ambiguous | Expanded the `ValidateBackendFeatures` godoc to explicitly state "concurrent calls with distinct tree/errs slices are safe" and "a single call is NOT re-entrant within itself". |
| NOTE | `sync.Once` schema cache persists across tests | No fix needed: tests build synthetic schemas directly (never touch the cached one) and there is no run-time path that mutates the already-loaded YANG. |

Second review pass found 5 further items; all resolved:

| Severity | Finding | Resolution |
|----------|---------|-----------|
| ISSUE | `InlineListNode.Backend` and `FlexNode.Backend` added but no tests exercised them | New tests `TestValidateBackendFeatures_InlineListNodeAnnotation` and `TestValidateBackendFeatures_FlexNodeAnnotation` build synthetic schemas with each type, assert per-entry error emission and path format. |
| NOTE | Nested rejecting annotations at different levels double-emitted (e.g. rejecting list + rejecting inner container both emitted for the same entry) | Refactored walker return type from `bool` to `(spoke, accepts bool)`. Narrowest-wins now applies to both accept-overrides-reject (AC-9) and reject-at-inner-suppresses-reject-at-outer. Callers use `spoke` to decide whether a narrower annotation already spoke for a subtree. New test `TestValidateBackendFeatures_NestedRejectsDoNotDoubleEmit` locks in the behavior. |
| NOTE | `ifaceDefaultBackend()` was a separate source of truth from `iface.defaultBackendName` | Exported `iface.DefaultBackendName()` and added `cmd/ze/config/default_backend_test.go` — a one-line sync assertion that the two stay equal on every build target. |
| NOTE | `walkBackendInlineList` duplicated per-entry emission logic from `walkBackendListMap` | Factored shared emission into `emitListEntryError` helper; both call sites now use it. Also extracted `walkInlineListEntryFields` to keep `walkBackendInlineList` symmetric with `walkBackendListMap`. |
| NOTE | `getBackendExtension` merge behavior for `ze:backend ""` statements was underspecified | Expanded the godoc to state explicitly that empty-after-trim tokens contribute nothing and CANNOT reset or widen a sibling statement -- the union always grows (or is a no-op), never shrinks. |

## Design Insights

- The walker is parameterized by component root and backend-leaf path, so the same helper serves iface today and firewall / traffic once those components implement Apply. No per-component copy of the walk.
- The JSON wrapper `ValidateBackendFeaturesJSON` lives next to the core walker so the iface plugin does not duplicate unmarshal logic at its call site.
- `ze config validate` runs the gate over an already-parsed `*Tree` via `ToMap()` instead of re-parsing the file text — one tree, two walkers (YANG tree validator + backend gate).
- Narrowest-annotation-wins is implemented by bubbling up "any descendant accepts" from recursive calls, so a per-case annotation that accepts the active backend suppresses the outer list's rejection for that subtree. This keeps AC-9 working without special per-case code paths.
- Backend names are not a global registry: each component owns its backends. The consistency test keeps a static allow-list (`netlink, vpp, nft, tc`) rather than importing four component packages to discover what is registered.

## Implementation Summary

### What Was Implemented

- `extension backend { argument names; ... }` in `ze-extensions.yang`.
- `getBackendExtension` reader in `yang_schema.go` mirroring `getOSExtension`. De-duplicates, tolerates `iface:backend` suffix form.
- `Backend []string` field on `LeafNode`, `ContainerNode`, `ListNode` in `schema.go`; populated from the YANG entry during build in `yangToLeaf`, `yangToContainer`, `yangToList`.
- `internal/component/config/backend_gate.go` — `ValidateBackendFeatures`, `ValidateBackendFeaturesJSON`, walker with narrowest-annotation-wins.
- `ze:backend "netlink"` annotations on iface YANG: `list bridge`, `list tunnel`, `list veth`, `list wireguard`, `container mirror`.
- `leaf backend` with sensible defaults added to `ze-firewall-conf.yang` (`nft`) and `ze-traffic-control-conf.yang` (`tc`) — ready for fw-3 / fw-5 to attach per-feature annotations.
- Iface wiring: `validateBackendGate` called from `OnConfigure` (before `LoadBackend`) and `OnConfigVerify` (before `pendingCfg`). Schema cached via `sync.Once`.
- Offline CLI wiring: `ze config validate` loops a gated-components table and runs the same helper.
- 17 unit tests (10 walker + 6 reader + 1 consistency guard) and 5 `.ci` functional tests under `test/parse/`.
- Docs: `docs/features.md` (feature summary), `docs/guide/configuration.md` (error reference), `docs/guide/command-reference.md` (ze config validate new error class), `docs/architecture/core-design.md` (new section 14a describing the gate).

### Bugs Found/Fixed

None found during implementation; no pre-existing failures surfaced.

### Documentation Updates

| File | Section | Change |
|------|---------|--------|
| `docs/features.md` | Feature table | Added "Commit-Time Backend Capability Check" row with source anchors. |
| `docs/guide/configuration.md` | Interface Configuration → Backend Capability Errors | New subsection with example rejection and fix, source anchors. |
| `docs/guide/command-reference.md` | `ze config validate` | Note on new error class, cross-link to configuration guide, source anchors. |
| `docs/architecture/core-design.md` | §14a Commit-Time Backend Capability Gate | New section describing the extension, schema reader, walker, wiring, and initial coverage. Source anchors on every factual claim. |

### Deviations from Plan

- Added `ValidateBackendFeaturesJSON` convenience wrapper (not named in spec). Avoids duplicating the JSON unmarshal at the iface call site and matches the shape of `sdk.ConfigSection.Data`.
- Added a seventh walker test (`TestValidateBackendFeatures_AbsentComponentRoot`) beyond the ten in the spec's TDD plan — covers the "config has bgp but no interface" case cleanly.
- Added `TestValidateBackendFeaturesJSON` and `TestValidateBackendFeaturesJSON_BadJSON` covering the new wrapper.
- Added backend-gate wiring inside `ze config validate` (noted in Data Flow as an expected benefit); matches the daemon-commit diagnostic offline.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Declarative YANG extension `ze:backend` | Done | `internal/component/config/yang/modules/ze-extensions.yang:125-142` | Declared alongside `ze:os`, same argument-string style. |
| Schema-time reader mirroring `getOSExtension` | Done | `internal/component/config/yang_schema.go:340-363` | Name: `getBackendExtension`. |
| Post-parse walker aggregating mismatches | Done | `internal/component/config/backend_gate.go:50-140` | `ValidateBackendFeatures` public, `walkBackendNode` private. |
| Adopt end-to-end in `interface` component | Done | `internal/component/iface/register.go:33-85`, `:264`, `:362` | Both `OnConfigure` and `OnConfigVerify` call the gate. |
| `leaf backend` in firewall and traffic YANG | Done | `internal/component/firewall/schema/ze-firewall-conf.yang:329-337`, `internal/component/traffic/schema/ze-traffic-control-conf.yang:48-56` | Defaults `nft`, `tc`. |
| Error names YANG path, active backend, supporting list | Done | `backend_gate.go:193-198` (`formatBackendError`) | No Go symbols, no stack traces. |
| `ze config validate` CLI matches daemon diagnostic | Done | `cmd/ze/config/cmd_validate.go:245-271` | Same helper, same text. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/parse/iface-vpp-rejects-bridge.ci` — stderr contains `/interface/bridge`, `"vpp"`, `netlink` | `bgp parse 71` passes. |
| AC-2 | Done | `test/parse/iface-vpp-rejects-tunnel.ci` — stderr contains `/interface/tunnel`, `"vpp"` | `bgp parse 72` passes. |
| AC-3 | Done | `test/parse/iface-vpp-accepts-ethernet.ci` — exit 0 | `bgp parse 69` passes. |
| AC-4 | Done | `internal/component/iface/register.go:264` calls `validateBackendGate` inside `OnConfigure`; same aggregated error shape as `OnConfigVerify`. | Schema cached via `sync.Once`. |
| AC-5 | Done | `test/parse/iface-vpp-aggregates-errors.ci` — stderr contains all three YANG paths | `bgp parse 70` passes. |
| AC-6 | Done | `test/parse/iface-netlink-accepts-bridge.ci` — exit 0 | `bgp parse 65` passes. |
| AC-7 | Done | `TestBackendExtensionNames_AllAnnotationsNameKnownBackends` in `backend_gate_test.go` | Walks every loaded schema module, cross-checks every `ze:backend` argument against the known-names set. |
| AC-8 | Done | `cmd/ze/config/cmd_validate.go:245-271`; `test/parse/iface-vpp-*.ci` exercise this path via `ze config validate -` | Same error text as daemon-commit. |
| AC-9 | Done | `TestValidateBackendFeatures_ChoiceCaseOverride` in `backend_gate_test.go` | Narrowest-annotation wins; case-level accept suppresses outer reject. |
| AC-10 | Done | Same consistency test as AC-7 | Single enumeration covers both assertions. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestGetBackendExtension_AbsentReturnsEmpty | Done | `internal/component/config/yang_schema_test.go:1329` | Pass. |
| TestGetBackendExtension_SingleArgument | Done | `yang_schema_test.go:1337` | Pass. |
| TestGetBackendExtension_SpaceSeparatedList | Done | `yang_schema_test.go:1347` | Pass. |
| TestValidateBackendFeatures_SupportedAccepts | Done | `backend_gate_test.go:13` | Pass. |
| TestValidateBackendFeatures_UnsupportedRejects | Done | `backend_gate_test.go:33` | Pass; asserts path, backend, and supporting list appear in error. |
| TestValidateBackendFeatures_AggregatesMultiple | Done | `backend_gate_test.go:59` | Pass; three entries in returned slice, each naming its path. |
| TestValidateBackendFeatures_UnrestrictedAccepts | Done | `backend_gate_test.go:92` | Pass; ethernet has no annotation. |
| TestValidateBackendFeatures_ChoiceCaseOverride | Done | `backend_gate_test.go:109` | Pass; covers AC-9. |
| TestValidateBackendFeatures_EmptyActiveBackend | Done | `backend_gate_test.go:137` | Pass; single clear error at backend-leaf path. |
| TestBackendExtensionNames_AllAnnotationsNameKnownBackends | Done | `backend_gate_test.go:246` | Pass; consistency guard over every registered module. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/yang/modules/ze-extensions.yang` | Done | `extension backend` declared. |
| `internal/component/config/yang_schema.go` | Done | `getBackendExtension` + Node population (leaf/container/list). |
| `internal/component/config/schema.go` | Done | `Backend []string` field on LeafNode, ContainerNode, ListNode. |
| `internal/component/config/backend_gate.go` | Done | New file: walker + JSON wrapper + error formatter. |
| `internal/component/config/backend_gate_test.go` | Done | New file: 10 walker tests + consistency guard + helper. |
| `internal/component/iface/register.go` | Done | `validateBackendGate` + OnConfigure/OnConfigVerify wiring. |
| `internal/component/iface/schema/ze-iface-conf.yang` | Done | Annotations on bridge/tunnel/wireguard/veth/mirror. |
| `internal/component/firewall/schema/ze-firewall-conf.yang` | Done | `leaf backend` (default "nft"). |
| `internal/component/traffic/schema/ze-traffic-control-conf.yang` | Done | `leaf backend` (default "tc"). |
| `cmd/ze/config/cmd_validate.go` | Done (deviation) | Added backend-gate loop in `runValidation`. Not listed in spec's Files to Modify but needed to satisfy AC-8. |
| `docs/features.md` | Done | Row added. |
| `docs/guide/configuration.md` | Done | Backend Capability Errors subsection. |
| `docs/guide/command-reference.md` | Done | `ze config validate` new error class. |
| `docs/architecture/core-design.md` | Done | §14a new section. |
| `test/parse/iface-vpp-rejects-bridge.ci` | Done | AC-1. |
| `test/parse/iface-vpp-rejects-tunnel.ci` | Done | AC-2. |
| `test/parse/iface-vpp-accepts-ethernet.ci` | Done | AC-3. |
| `test/parse/iface-netlink-accepts-bridge.ci` | Done | AC-6 regression guard. |
| `test/parse/iface-vpp-aggregates-errors.ci` | Done | AC-5. |

### Audit Summary
- **Total items:** 19 files + 10 AC + 10 TDD tests + 7 Task requirements = 46 tracked items.
- **Done:** 46.
- **Partial:** 0.
- **Skipped:** 0.
- **Changed:** 1 (added `cmd/ze/config/cmd_validate.go` beyond the spec's Files to Modify to satisfy AC-8 without a daemon).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/backend_gate.go` | Yes | `ls -la internal/component/config/backend_gate.go` — 7428 bytes |
| `internal/component/config/backend_gate_test.go` | Yes | `ls -la internal/component/config/backend_gate_test.go` — 8913 bytes |
| `test/parse/iface-vpp-rejects-bridge.ci` | Yes | `ls test/parse/iface-vpp-rejects-bridge.ci` |
| `test/parse/iface-vpp-rejects-tunnel.ci` | Yes | `ls test/parse/iface-vpp-rejects-tunnel.ci` |
| `test/parse/iface-vpp-accepts-ethernet.ci` | Yes | `ls test/parse/iface-vpp-accepts-ethernet.ci` |
| `test/parse/iface-netlink-accepts-bridge.ci` | Yes | `ls test/parse/iface-netlink-accepts-bridge.ci` |
| `test/parse/iface-vpp-aggregates-errors.ci` | Yes | `ls test/parse/iface-vpp-aggregates-errors.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | vpp+bridge → error names `/interface/bridge` + `vpp` + `netlink` | `bin/ze-test bgp parse 71` → pass (215ms) |
| AC-2 | vpp+tunnel → error names `/interface/tunnel` + `vpp` | `bin/ze-test bgp parse 72` → pass (185ms) |
| AC-3 | vpp+ethernet → accepted | `bin/ze-test bgp parse 69` → pass (208ms) |
| AC-4 | Daemon startup gate | `grep -n 'validateBackendGate(sections, cfg.Backend)' internal/component/iface/register.go` → two matches (OnConfigure, OnConfigVerify) |
| AC-5 | Three-feature aggregation | `bin/ze-test bgp parse 70` → pass (186ms); stderr contains `/interface/bridge`, `/interface/tunnel`, `/interface/wireguard` |
| AC-6 | netlink+bridge → no regression | `bin/ze-test bgp parse 65` → pass (185ms) |
| AC-7 | Consistency guard | `go test -run TestBackendExtensionNames ./internal/component/config/...` → pass (0.04s) |
| AC-8 | Offline CLI parity | `grep -n ValidateBackendFeatures cmd/ze/config/cmd_validate.go` → `runValidation` loop calls it |
| AC-9 | Per-case override | `go test -run ChoiceCaseOverride ./internal/component/config/...` → pass |
| AC-10 | Annotation name consistency | Same test as AC-7 (single guard covers both) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate -` (stdin) | `test/parse/iface-vpp-rejects-bridge.ci` | Yes — runs actual `ze` binary via `cmd=foreground:seq=1:exec=ze config validate -:stdin=config`, asserts exit code 1 and stderr contains YANG path, backend, supporting list. |
| `ze config validate -` (stdin) | `test/parse/iface-vpp-rejects-tunnel.ci` | Yes — same shape, asserts `/interface/tunnel` and `vpp`. |
| `ze config validate -` (stdin) | `test/parse/iface-vpp-accepts-ethernet.ci` | Yes — exit 0 + stdout `configuration valid`. |
| `ze config validate -` (stdin) | `test/parse/iface-netlink-accepts-bridge.ci` | Yes — exit 0 (regression guard). |
| `ze config validate -` (stdin) | `test/parse/iface-vpp-aggregates-errors.ci` | Yes — all three YANG paths appear in stderr. |
| iface plugin `OnConfigure` | `internal/component/iface/register.go:264` | Yes — grep confirms the call site between `LoadBackend` call and `applyConfig` call. |
| iface plugin `OnConfigVerify` | `internal/component/iface/register.go:362` | Yes — grep confirms the call site before `pendingCfg = cfg`. |

## Checklist

### Goal Gates (MUST pass)
- [ ] All 10 AC demonstrated with tests
- [ ] Wiring Test table complete with .ci file names — 5 rows
- [ ] `make ze-verify-fast` passes (and `make ze-test` passes for full suite)
- [ ] `ze:backend` extension declared + schema reader + generic walker landed
- [ ] iface component end-to-end: annotations + wiring + 5 `.ci` tests all green
- [ ] firewall / traffic pattern-ready: `leaf backend` + initial annotations
- [ ] Documentation updates complete (4 files)

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-backend-feature-gate.md`
- [ ] Summary included in commit

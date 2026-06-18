# Spec: isis-4-component-config

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-3-l2-transport.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` - authoritative scope (this child is row isis-4)
4. `internal/component/ldp/register.go` - closest registration + SDK lifecycle template
5. `internal/component/ldp/yang/` (embed.go, register.go, ze-ldp-conf.yang) - generated YANG glue pattern
6. `internal/component/bgp/config/resolve.go` (`ResolveBGPTree`), `internal/component/bgp/config/peers.go` (`PeersFromConfigTree`) - config tree to typed struct resolution
7. `internal/component/config/yang_schema.go` lines 203-231 - `ze-*-conf.yang` discovery/merge; `make generate` updates `internal/component/plugin/all/all.go`
8. `docs/research/isis-implementation-guide.md` section 9 (Configuration Shape, lines 586-712), sections 10/15 (plugin model)

## Task

Create the `internal/component/isis/` component and register it so a top-level
`isis { ... }` config block reaches a running IS-IS engine. This is the **wiring
backbone** of the IS-IS spec set: the integration skeleton that every runtime
child (adjacency, LSDB, flooding, DIS, SPF, auth, redistribution, IPv6, CLI)
depends on. After this spec, Ze has an IS-IS component that registers, accepts
and validates the `isis` config subtree, resolves it to typed Go structs, applies
YANG defaults, and starts an engine that opens circuits via the spec-isis-3 L2
transport and then does nothing else yet. The runtime behaviour (forming
adjacencies, building the LSDB, flooding, SPF, route install) is delivered by the
sibling specs (`spec-isis-5-adjacency` and later); here the goroutines those
specs fill are stubs.

Concretely this spec delivers, modelled on `internal/component/ldp/register.go`:

- A `registry.Registration` in `init()` with `Name "isis"`, a description,
  `Features "yang"`, `YANG` set to the embedded `ze-isis-conf.yang`,
  `ConfigRoots ["isis"]`, `Dependencies ["fib-kernel", "sysctl"]`, the matching
  `RFCs`, `RunEngine runISISEngine`, plus `ConfigureEngineLogger`,
  `ConfigureMetrics`, `ConfigureEventBus`, and a `CLIHandler`.
- The SDK lifecycle `runISISEngine(conn net.Conn) int`: `sdk.NewWithConn("isis",
  conn)`, then `OnConfigVerify` (parse-check only), `OnConfigure` (parse to typed
  config), `OnConfigApply` (reconcile against the running engine via a journal so
  reloads are incremental, not restart-everything), `OnStarted` (open circuits via
  spec-isis-3 transport and launch the per-circuit goroutines as stubs that later
  specs fill), and `OnExecuteCommand` (dispatch the `show isis` commands, stubbed
  here), then `p.Run(ctx, sdk.Registration{WantsConfig:["isis"], Commands:[...]})`
  with a clean shutdown that stops circuits and goroutines.
- `internal/component/isis/yang/ze-isis-conf.yang`: a top-level `container isis`
  with `ze:config-root "isis"` and maximally-validated leaves per
  `ai/patterns/config-option.md` (the leaf table is in the Acceptance Criteria
  and TDD sections below).
- Config resolution that parses the `isis` JSON subtree to typed Go structs,
  applies YANG defaults, and validates required fields (at least one NET, a
  system-id derivable from the NET).
- `events.RegisterNamespace` for IS-IS event types (session up/down, LSP change)
  consumed by later specs.
- `make generate` discovering the new `internal/component/isis` and
  `internal/component/isis/yang` packages and adding them to
  `internal/component/plugin/all/all.go` (generated, never hand-edited),
  including the generated `yang/register.go` and `yang/embed.go` glue.

The wiring test `TestISISComponentStart` proves the chain: config `isis { ... }`
present, component registers, engine runs, circuits open. A functional `.ci`
under `test/isis/` loads an IS-IS config and confirms the component is up.

## Required Reading

### Architecture Docs
- [ ] `internal/component/ldp/register.go` - closest registration + SDK lifecycle template
  -> Decision: model `init()` registration and `runISISEngine` directly on the LDP `registerLDP`/`runLDPEngine` pair (registry.Registration fields, OnConfigVerify/OnConfigure/OnStarted/OnExecuteCommand, `p.Run`, clean shutdown)
  -> Constraint: registration not switch dispatch (`ai/rules/registration-dispatch.md`): IS-IS registers; core discovers it through the registry, never imports it directly
- [ ] `internal/component/ldp/yang/` (embed.go, register.go, ze-ldp-conf.yang) - generated YANG glue pattern
  -> Constraint: `yang/register.go` and `yang/embed.go` are `// Code generated ... DO NOT EDIT.`; they are produced by `scripts/codegen/yang_glue.go` via `make generate`, never hand-written
- [ ] `internal/component/bgp/config/resolve.go` (`ResolveBGPTree`), `peers.go` (`PeersFromConfigTree`) - config tree to typed struct resolution
  -> Decision: parse the `isis` subtree (JSON in the SDK ConfigSection, like LDP `parseLDPConfig`) into typed Go structs; container fields merge at key level, leaf-lists accumulate where marked
- [ ] `internal/component/config/yang_schema.go` lines 203-231 - `ze-*-conf.yang` discovery/merge
  -> Constraint: any module whose name ends `-conf` has its top-level data nodes merged into the schema; `ze-isis-conf.yang` is auto-discovered at init once the generated glue imports the package
- [ ] `ai/patterns/config-option.md` - YANG leaf + module registration + validator pattern
  -> Constraint: native validation maximised (`range`/`length`/`pattern`/`enumeration`); custom `ze:validate` + `ValidateFn` + `CompleteFn` only where native is insufficient (NET, system-id)
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md` - YANG vs env var, kebab-case naming
  -> Constraint: IS-IS config is YANG (no env vars), top-level `isis` container, kebab-case leaves, Go struct fields are PascalCase of the leaf
- [ ] `ai/rules/wiring-completeness.md`, `ai/rules/plugin-self-containment.md` - reachability and self-containment
  -> Constraint: every exported symbol has a non-test caller; all IS-IS schema/help/commands live under `internal/component/isis/`; no `isis` spelling in generic/central packages except generated `all.go`

### RFC Summaries (MUST for protocol work)
- [ ] `iso/short/iso10589.md` - IS-IS base (created by isis-2; referenced in `RFCs`)
  -> Constraint: NET / system-id / area-id structure constrains the config validators
- [ ] `rfc/short/rfc5305.md` - wide metrics (TLV 22 / TLV 135)
  -> Constraint: wide metric range 0..16777215 bounds the per-interface `metric` leaf
- [ ] `rfc/short/rfc5301.md` - dynamic hostname (TLV 137)
  -> Constraint: `hostname` leaf advertises the dynamic hostname (origination is isis-6)

**Key insights:**
- IS-IS is a component (`internal/component/isis/`), not a plugin dir; engine-owning protocols (BGP, LDP, RSVP-TE) live in `component/`
- The component is registered exactly like LDP; the only IS-IS-specific config concern is the NET / system-id custom validators with `CompleteFn`
- `make generate` auto-wires `all.go` and the `yang/` glue; hand-editing generated files is forbidden
- This spec wires a skeleton: config in, engine up, circuits opened, nothing else; runtime lives in sibling specs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] No `internal/component/isis/` directory and no `isis` registry entry exist today
  -> Constraint: this spec creates the directory, the registration, and the YANG module from scratch
- [ ] No top-level `isis` config container exists; `internal/component/config/yang_schema.go` only merges `-conf` modules that are registered, and none is `ze-isis-conf`
  -> Constraint: the `isis` container only appears once `ze-isis-conf.yang` is registered via generated glue and `make generate` runs
- [ ] `internal/component/plugin/all/all.go` is generated and does not import an `isis` package
  -> Constraint: never hand-edit `all.go`; `make generate` adds the import after the package exists
- [ ] LDP demonstrates the full pattern: `registerLDP` builds `registry.Registration`, `runLDPEngine` wires the SDK callbacks and `p.Run`
  -> Constraint: IS-IS mirrors this pattern; differences are config shape and circuit opening (via isis-3) instead of UDP/TCP discovery

**Behavior to preserve:**
- BGP, LDP, RSVP-TE, static, connected route sources remain independent and functional
- The YANG schema discovery/merge in `yang_schema.go` is unchanged (IS-IS adds a module, does not change the loader)
- `internal/component/plugin/all/all.go` remains generated only; no manual edits

**Behavior to change:**
- New top-level `isis` config container and `internal/component/isis/` component
- A new IS-IS event namespace registered for session up/down and LSP change
- `server.go` owns a new PDU-type receive dispatcher (keyed by the 5-bit PDU type) that isis-5/6/7 register handlers against
- `make generate` adds `internal/component/isis` and `internal/component/isis/yang` to `all.go`
- A new `test/isis` functional-test suite registered in `internal/test/cli/register.go` and `mk/test-functional.mk`

## Data Flow (MANDATORY)

### Entry Point
- Config arrives as the `isis` subtree of the YANG-validated config tree, delivered to the engine as a JSON `sdk.ConfigSection` (Root `isis`), exactly as LDP receives its `ldp` section
- Interface up/down events (from the iface EventBus, used to open or close circuits) are consumed by the engine started in `OnStarted` via the spec-isis-3 transport

### Transformation Path
1. **Schema:** `ze-isis-conf.yang` is registered at init (generated glue) and its top-level `isis` container is merged into the config schema by `yang_schema.go`
2. **Validate (parse-check):** the SDK invokes `OnConfigVerify`; the engine parses the JSON subtree and rejects malformed config (bad NET, no NET, undefiable system-id) without mutating state
3. **Configure (typed structs):** the SDK invokes `OnConfigure`; the engine parses the subtree into typed Go structs and applies YANG defaults, producing the staged config
4. **Apply (reconcile):** `OnConfigApply` diffs the staged config against the running config via a journal and reconciles circuits/levels incrementally
5. **Start (circuits):** `OnStarted` opens a circuit per enabled, non-passive interface via the spec-isis-3 transport and launches per-circuit goroutines (stubs in this spec; later specs fill RX/TX/timers/adjacency)
6. **Receive dispatch:** the spec-isis-3 transport delivers `(ifindex, pdu []byte)` after stripping 802.3+LLC and does NOT switch on PDU type. `server.go` owns the PDU-type receive dispatcher, keyed by the 5-bit PDU type, that routes the PDU: IIH (0x0f L1 LAN, 0x10 L2 LAN, 0x11 P2P) to adjacency (isis-5); LSP and CSNP/PSNP (0x12/0x14 LSP, 0x18/0x19 CSNP, 0x1a/0x1b PSNP) to lsdb/flooding (isis-6/isis-7). Handlers register with the dispatcher at startup; this spec installs the dispatcher and stub handlers, later specs fill the real handlers (per Shared Contracts "PDU receive dispatcher", owner isis-4)
7. **Commands:** `OnExecuteCommand` dispatches `show isis ...` (stubbed here; later specs return real snapshots)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> engine | SDK `OnConfigVerify`/`OnConfigure`/`OnConfigApply` (JSON `isis` subtree) | [ ] |
| Engine <-> L2 transport | spec-isis-3 `Open(circuit)` per enabled interface | [ ] |
| Transport <-> PDU dispatcher | spec-isis-3 delivers `(ifindex, pdu)`; `server.go` dispatcher routes by 5-bit PDU type (no PDU switch in transport) | [ ] |
| Engine <-> EventBus | `events.RegisterNamespace` + emit (session up/down, LSP change) | [ ] |
| CLI <-> engine | `OnExecuteCommand` dispatch keyed by `show isis <noun>` | [ ] |
| Schema <-> loader | `ze-isis-conf.yang` `-conf` suffix auto-merge in `yang_schema.go` | [ ] |

### Integration Points
- New component `internal/component/isis/` (register.go, config.go, events.go, server.go) plus `yang/`
- `server.go` PDU-type receive dispatcher (keyed by the 5-bit PDU type) that isis-5/6/7 register handlers against; spec-isis-3 delivers `(ifindex, pdu)` to it and holds no protocol switch
- `ze-isis-conf.yang` registered through generated `yang/register.go`
- `internal/component/plugin/all/all.go` regenerated by `make generate` to import the new packages
- spec-isis-3 L2 transport (circuit open/close) consumed by `OnStarted`
- iface EventBus link up/down drives circuit enable/disable (wired here, acted on by isis-5)

### Architectural Verification
- [ ] No bypassed layers (config -> schema -> SDK callbacks -> engine -> transport)
- [ ] No unintended coupling (IS-IS independent of BGP-LS; transport behind the isis-3 interface)
- [ ] No duplicated functionality (registration reuses the registry; no second config path)
- [ ] Generated files untouched (`all.go`, `yang/register.go`, `yang/embed.go` produced by `make generate`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `make generate` discovers a new `component/isis` + `yang` package automatically | LDP/RSVP-TE precedent; `yang_schema.go` 203-231; umbrella A-5 | Manual `all.go` edit needed (forbidden) | build + `grep isis all.go` after `make generate` | unvalidated |
| A-2 | The SDK ConfigSection JSON for the `isis` subtree carries every leaf shape the typed parse expects | LDP `parseLDPConfig` reads its subtree the same way | Parse misses nested containers (level-1/level-2, interfaces list) | `TestISISConfigResolve` against a full config | unvalidated |
| A-3 | spec-isis-3 exposes a circuit `Open`/`Close` API the engine can call from `OnStarted` | umbrella architecture (transport behind an interface) | Wiring stub cannot open a circuit; isis-4 blocked on isis-3 API shape | `TestISISComponentStart` opening a circuit on a STUB transport backend (no real veth -- the real veth round-trip is isis-3's QEMU integration test); this unit test depends only on the isis-3 transport interface | unvalidated |
| A-4 | A top-level `isis` container with `ze:config-root "isis"` and `ConfigRoots:["isis"]` routes the subtree to the engine | LDP uses top-level `ldp` + `ConfigRoots:["ldp"]`; `test/parse/ldp-config-valid.ci` | Config never reaches `OnConfigVerify`; validate passes vacuously | `test/isis/isis-config.ci` exercising validate + start | unvalidated |
| A-5 | system-id can always be derived from a valid NET (the 6 bytes before the NSEL) | ISO/IEC 10589 NET structure; research doc sec 9 | Need a separate explicit system-id leaf as the source of truth | `TestISISConfigValidate` NET-only case | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Bare `type string` leaves slip in (NET, system-id, hostname) instead of validated types | YANG validation review flags it | Custom `ze:validate` + pattern; NET/system-id validators with `CompleteFn` |
| R-2 | Reload restarts every circuit instead of reconciling | circuit flap on any config change | `OnConfigApply` journal-diff reconcile (isis-4), not restart-all |
| R-3 | `make generate` not run, so `all.go` lacks the import and the component never loads | component absent from `ze plugin` inventory | Wiring test asserts the component is in the inventory; run `make generate` in the wiring phase |
| R-4 | Circuit-open stub couples to isis-3 internals instead of its interface | isis-3 API churn breaks isis-4 build | Depend only on the isis-3 transport interface; keep the stub behind it |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `isis { ... }` present | -> | `registerISIS` registration + `runISISEngine` start, circuits opened | `TestISISComponentStart` |
| config `isis { ... }` present | -> | component appears in the plugin inventory after `make generate` wires `all.go` | `TestISISComponentStart` (inventory assertion) |
| `isis { ... }` loaded via `ze config validate` | -> | `OnConfigVerify` accepts a valid config, rejects an invalid NET | `test/isis/isis-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `isis { net 49.0001.0000.0000.0001.00 }` present | Component registers, `runISISEngine` starts, engine reaches `OnStarted`, circuits opened for enabled interfaces |
| AC-2 | Config with a full `isis` block (net, level, lsp-lifetime, interfaces list with metric/hello/priority and level-1/level-2 sub-containers) | `OnConfigure` resolves it to typed structs with YANG defaults applied; `TestISISConfigResolve` matches expected struct |
| AC-3 | Config with no `net` leaf | `OnConfigVerify` rejects with a clear error (at least one NET required) |
| AC-4 | Config with an invalid NET (wrong length, bad hex, missing NSEL `00`) | `OnConfigVerify` rejects via the NET custom validator before the engine mutates state |
| AC-5 | Config with `level l3` (not in the enum) | YANG native enum validation rejects it at schema validation, before the engine |
| AC-6 | Config with interface `metric 16777216` (above wide-metric max) | YANG native range validation rejects it |
| AC-7 | `ze plugin` inventory queried after build | `isis` appears as a registered component (proves `make generate` wired `all.go`) |
| AC-8 | Config reload changing one interface metric | `OnConfigApply` reconciles only the affected circuit via the journal; other circuits are not torn down |
| AC-9 | Valid NET only, no explicit system-id | A system-id is derived from the NET (6 bytes before the NSEL); engine starts |
| AC-10 | Tab-completion on `net` / `system-id` in the editor | The custom validator `CompleteFn` returns guidance values for the field |
| AC-11 | `bin/ze-test isis --all` invoked (suite registered in `internal/test/cli/register.go` and `mk/test-functional.mk`) | The `test/isis` suite resolves and `test/isis/isis-config.ci` runs under the functional-test runner (`make ze-isis-test`), passing |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `isis { net ... }` and runs `ze config validate` | config file -> YANG schema -> validate -> `OnConfigVerify` accept | `test/isis/isis-config.ci` |
| 2 | Mistypes the NET and runs `ze config validate` | config file -> YANG schema -> validate -> NET validator reject | `test/isis/isis-config.ci` (invalid case) |
| 3 | Starts Ze with an `isis` block | config -> component -> `runISISEngine` -> `OnStarted` -> circuits opened | `TestISISComponentStart` |
| 4 | Lists plugins to confirm IS-IS is present | `ze plugin` -> registry inventory | `TestISISComponentStart` (inventory assertion) |
| 5 | Reloads config with a changed interface metric | reload -> `OnConfigApply` -> journal diff -> reconcile one circuit | `TestISISConfigApplyReconcile` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISConfigResolve` | `internal/component/isis/config_test.go` | Full `isis` subtree parses to typed structs with YANG defaults applied | |
| `TestISISConfigDefaults` | `internal/component/isis/config_test.go` | Omitted leaves resolve to YANG defaults (level `l1-l2`, lsp-lifetime 1200, metric 10, hello-interval 10, hold-multiplier 3, priority 64) | |
| `TestISISConfigValidate` | `internal/component/isis/config_test.go` | NET-only config validates; system-id derived from NET; missing NET rejected | |
| `TestISISNETValidator` | `internal/component/isis/config_test.go` | NET custom validator accepts valid NETs, rejects bad length/hex/NSEL; `CompleteFn` returns guidance | |
| `TestISISSystemIDValidator` | `internal/component/isis/config_test.go` | system-id pattern accepts `xxxx.xxxx.xxxx`, rejects malformed | |
| `TestISISComponentStart` | `internal/component/isis/server_test.go` | Registration present; `runISISEngine` reaches `OnStarted`; circuit opened on a stub; component in inventory | |
| `TestISISConfigApplyReconcile` | `internal/component/isis/server_test.go` | `OnConfigApply` reconciles only the changed circuit (journal diff), no full restart | |
| `TestISISEventNamespace` | `internal/component/isis/events_test.go` | `events.RegisterNamespace` registers session up/down and LSP-change types | |
| `TestISISPDUDispatch` | `internal/component/isis/server_test.go` | The PDU-type dispatcher routes each 5-bit PDU type (IIH 0x0f/0x10/0x11, LSP 0x12/0x14, CSNP 0x18/0x19, PSNP 0x1a/0x1b) to its registered handler; unknown types are dropped, not panicked | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interface metric (wide) | 1..16777215 | 16777215 | 0 | 16777216 |
| DIS priority | 0..127 | 127 | N/A | 128 |
| Hold multiplier | 1..255 | 255 | 0 | 256 |
| Hello interval | 1..65535 | 65535 | 0 | 65536 |
| LSP lifetime | 1..65535 | 65535 | 0 | 65536 |
| LSP refresh interval | 1..65535 | 65535 | 0 | 65536 |

### YANG Leaf Table (ze-isis-conf.yang)

Top-level `container isis` with `ze:config-root "isis"`. Leaves (described, not pasted as a YANG block):

| Leaf / node | Type | Constraints | Default | Description |
|-------------|------|-------------|---------|-------------|
| `net` | leaf-list string | `ze:validate "isis-net"` (custom NET validator + `CompleteFn`) | - | Network Entity Title(s); at least one required |
| `system-id` | leaf string | `pattern "[0-9a-fA-F]{4}\\.[0-9a-fA-F]{4}\\.[0-9a-fA-F]{4}"` | derived from NET | 6-byte system identifier (xxxx.xxxx.xxxx) |
| `level` | leaf enumeration | enum `l1` / `l2` / `l1-l2` | `l1-l2` | Routing level of this IS |
| `lsp-lifetime` | leaf uint16 | `range "1..65535"`, units seconds | `1200` | Maximum LSP remaining lifetime |
| `lsp-refresh-interval` | leaf uint16 | `range "1..65535"`, units seconds | `900` | LSP refresh interval |
| `overload` | leaf boolean | - | `false` | Set the overload bit |
| `hostname` | leaf string | `length "1..255"` | - | Dynamic hostname to advertise (RFC 5301) |
| `interfaces` | list (key `name`) | - | - | Per-interface IS-IS configuration |
| `interfaces/name` | leaf string | key | - | Interface name |
| `interfaces/enabled` | leaf boolean | - | `true` | IS-IS enabled on this interface |
| `interfaces/passive` | leaf boolean | - | `false` | Advertise the interface but form no adjacencies |
| `interfaces/circuit-type` | leaf enumeration | enum `broadcast` / `point-to-point` | `broadcast` | Circuit type |
| `interfaces/level` | leaf enumeration | enum `l1` / `l2` / `l1-l2` | `l1-l2` | Per-interface level override |
| `interfaces/metric` | leaf uint32 | `range "1..16777215"` | `10` | Wide metric (RFC 5305) |
| `interfaces/hello-interval` | leaf uint16 | `range "1..65535"`, units seconds | `10` | Hello interval |
| `interfaces/hold-multiplier` | leaf uint8 | `range "1..255"` | `3` | Hold time = hello-interval * hold-multiplier |
| `interfaces/priority` | leaf uint8 | `range "0..127"` | `64` | DIS election priority (broadcast) |
| `interfaces/level-1` | container | mirrors metric/hello-interval/hold-multiplier/priority | per-leaf | Level-1 per-interface overrides |
| `interfaces/level-2` | container | mirrors metric/hello-interval/hold-multiplier/priority | per-leaf | Level-2 per-interface overrides |
| `interfaces/address-family` | list (key `af`) | - | - | Per-interface address families enabled on this circuit (single-topology) |
| `interfaces/address-family/af` | leaf enumeration | enum `ipv4-unicast` / `ipv6-unicast` | - | Address family key; the exact path spec-isis-12-ipv6 and spec-isis-13 reference |
| `key-chains` | list (key `name`) | - | - | Named authentication key chains for hitless key rotation |
| `key-chains/name` | leaf string | key, `length "1..63"` | - | Key-chain name; referenced by per-interface (IIH) and per-level (LSP/SNP) auth chain leaves |
| `key-chains/key` | list (key `key-id`) | - | - | Keys in this chain |
| `key-chains/key/key-id` | leaf uint16 | key, `range "0..65535"` | - | Key identifier carried in TLV 10 (RFC 5310) |
| `key-chains/key/algorithm` | leaf enumeration | enum `cleartext` / `hmac-md5` / `hmac-sha-256` / `hmac-sha-1` / `hmac-sha-384` / `hmac-sha-512` | `hmac-md5` | Authentication algorithm; auth type codes cleartext 1, HMAC-MD5 54 (RFC 5304), generic crypto / HMAC-SHA 3 (RFC 5310) |
| `key-chains/key/secret` | leaf string | `length "1..255"`, `ze:sensitive` ($9$-encoded) | - | The shared secret, masked and `$9$`-encoded at rest |
| `key-chains/key/send-lifetime` | container | optional start/end timestamps | - | When this key may be used to sign (hitless rotation) |
| `key-chains/key/accept-lifetime` | container | optional start/end timestamps | - | When this key is accepted on receive (hitless rotation) |
| `interfaces/level-1/auth-key-chain` | leaf string | leafref/name into `key-chains` | - | L1 per-interface (IIH) key chain reference |
| `interfaces/level-2/auth-key-chain` | leaf string | leafref/name into `key-chains` | - | L2 per-interface (IIH) key chain reference |
| `level-1/auth-key-chain` | leaf string | leafref/name into `key-chains` | - | L1 per-level (LSP/SNP, area key) key chain reference |
| `level-2/auth-key-chain` | leaf string | leafref/name into `key-chains` | - | L2 per-level (LSP/SNP, domain key) key chain reference |

Authentication is modelled as named KEY CHAINS, not bare auth strings: a
`key-chains` list of keys, each carrying a `key-id`, an `algorithm` enum
(cleartext / hmac-md5 / hmac-sha-256 / ...), a `$9$`-encoded length-constrained
`secret`, and optional send/accept lifetimes for hitless rotation. Per-interface
chains apply to IIH PDUs; per-level chains apply to LSP/SNP PDUs (the L1 chain is
the area key, the L2 chain the domain key). These leaves are declared here for
completeness and this spec only parses and stores them; the per-PDU verify/sign
semantics live in `spec-isis-10-auth` per Shared Contracts "Authentication config
model".

Address-family configuration follows Shared Contracts "Address-family config
path": per-interface families under `interfaces/interface/address-family` with
the `af` enum `ipv4-unicast` / `ipv6-unicast` (single-topology; both ride the
shared SPF tree). This is the exact path spec-isis-12-ipv6 (IPv6 enablement) and
spec-isis-13 (display) reference.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-config` | `test/isis/isis-config.ci` | A valid `isis { net ... }` block validates and the component starts; an invalid NET is rejected | |

### Interop Tests (MANDATORY for protocol features)
No wire-protocol behaviour is introduced by this spec (it is component/config
wiring only). Protocol interop is covered by sibling specs:
`spec-isis-5-adjacency` (adjacency), `spec-isis-9-spf-rib` (convergence),
`spec-isis-13-cli-diag-interop` (full FRR scenarios). Interop is therefore N/A
for isis-4.

### Future (if deferring any tests)
- Runtime behaviour tests (adjacency up, LSDB sync, SPF) belong to siblings isis-5/6/7/9 and are not deferred here; this spec only proves the skeleton starts.

## Files to Modify
- `internal/component/plugin/all/all.go` - regenerated by `make generate` to import `internal/component/isis` and `internal/component/isis/yang` (generated, never hand-edited)
- `internal/test/cli/register.go` - register the new `test/isis` functional-test suite via `registerCIRoot("isis", "isis", "isis", "<description>", 0)` so `bin/ze-test isis --all` resolves; isis-4 establishes the suite, later specs add `.ci` cases
- `mk/test-functional.mk` - add a `ze-isis-test` individual target (`@$(SUITE_RUN) bin/ze-test isis --all`), add `isis` to the `all_suites` / `run_suite` list and the `.PHONY` declarations so the suite runs under `make ze-functional-test`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/component/isis/yang/ze-isis-conf.yang` (top-level `isis`, `ze:config-root`) |
| YANG validation constraints | Yes | `range`/`pattern`/`enumeration` on every numeric/enum/id leaf (metric, priority, hold-multiplier, level, system-id, lifetimes) |
| YANG custom validators | Yes | NET validator `isis-net` and system-id validation with `CompleteFn`; registered in the config validator registry |
| CLI commands/flags | No | `show isis neighbor/database/route/interface` dispatched in `OnExecuteCommand` but stubbed; real grammar in spec-isis-13-cli-diag-interop |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md`; command names declared in `sdk.Registration.Commands` as `show isis <noun>` |
| Editor autocomplete | Yes | YANG enum/type driven, plus NET/system-id `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/isis/isis-config.ci` |
| Pipe completeness | No | Deferred to spec-isis-13 (no real show output yet) |
| Env var registration | No | None; IS-IS config is YANG-only, nothing under `environment/` |
| Doctor check for runtime dependencies | No | `CAP_NET_RAW` / raw-socket doctor check is owned by spec-isis-3-l2-transport |
| Prometheus counters/metrics | Yes | `ConfigureMetrics` wired (registry stored); the actual `ze_isis_*` series are registered per-owner by the runtime siblings (isis-3/5/6/7/8/9/10/11 per the umbrella "Metrics (canonical)" table), NOT by isis-13 (which only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Deferred to spec-isis-13 (feature is not usable until adjacency/SPF land) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` (new `isis` block) |
| 3 | CLI command added/changed? | No | Commands stubbed; documented in spec-isis-13 |
| 4 | API/RPC added/changed? | No | Stubbed RPC; documented in spec-isis-13 |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md` (new `isis` component) |
| 6 | Has a user guide page? | No | `docs/guide/isis.md` is created by spec-isis-13 when the feature is usable |
| 7 | Wire format changed? | No | No wire format in this spec |
| 8 | Plugin SDK/protocol changed? | No | Uses the existing SDK lifecycle unchanged |
| 9 | RFC behavior implemented? | No | Config wiring only; RFC behaviour in siblings |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/isis/` directory) |
| 11 | Affects daemon comparison? | No | Deferred to spec-isis-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new `isis` component) |
| 13 | Route metadata keys added/changed? | No | None in this spec |
| 14 | Prometheus counters added/changed? | No | Registry wired here; the `ze_isis_*` series are registered per-owner by runtime siblings (isis-3/5/6/7/8/9/10/11), not centrally and not by isis-13 |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` (new component, new event namespace) |
| 16 | Any changed source file referenced by doc source anchors? | No | Grep `docs/` for source anchors at completion |
| 17 | Existing docs show examples for this area? | No | Grep at completion; verify `isis` block example against the YANG |

## Files to Create
- `internal/component/isis/register.go` - `init()` registration (`registry.Registration`) + `runISISEngine` SDK lifecycle
- `internal/component/isis/config.go` - typed config structs, parse from the `isis` JSON subtree, YANG-default application, NET/system-id validation and derivation
- `internal/component/isis/events.go` - `events.RegisterNamespace` + IS-IS event types (session up/down, LSP change)
- `internal/component/isis/server.go` - top-level orchestration: open circuits via spec-isis-3 transport, launch per-circuit goroutine stubs, the PDU-type receive dispatcher (keyed by the 5-bit PDU type, routing IIH to adjacency / LSP / CSNP / PSNP to lsdb/flooding; handlers register at startup, stubs here), clean shutdown, journal-based reconcile
- `internal/component/isis/yang/ze-isis-conf.yang` - config schema (top-level `isis`, `ze:config-root`, validated leaves per the leaf table). The `yang/` directory is the home for all IS-IS YANG modules: this spec creates only `ze-isis-conf.yang`; the command module `ze-isis-cmd.yang` (CLI command ownership; show binds `ze-show:isis-*`, clear binds `ze-clear:isis-*`, modelled on `internal/plugins/ldp-cmd/yang/ze-ldp-cmd.yang`; there is NO `ze-isis-api.yang`) lives in the same directory but is owned and authored by spec-isis-13-cli-diag-interop per Shared Contracts "Command + API YANG"; this spec does NOT define the cmd schema
- `internal/component/isis/yang/register.go` - generated glue (`configyang.RegisterModule`), produced by `make generate`
- `internal/component/isis/yang/embed.go` - generated `//go:embed ze-isis-conf.yang` var, produced by `make generate`
- `test/isis/isis-config.ci` - functional test: valid `isis` config validates and the component starts; invalid NET rejected

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-isis-0-umbrella.md` |
| 2. Audit | Files to Create, Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register the IS-IS component, write the failing wiring test
   - Tests: `TestISISComponentStart`
   - Files: `internal/component/isis/register.go` (stub `runISISEngine` with empty `OnStarted`), `internal/component/isis/yang/ze-isis-conf.yang` (minimal: `isis` container + `net` leaf-list), run `make generate` for `yang/register.go`, `yang/embed.go`, and `all.go`
   - Verify: registration present, component in inventory, `runISISEngine` reachable; the wiring test fails because circuit opening is still a stub
2. **Phase: YANG schema + validators** - full leaf table, NET and system-id validators
   - Tests: `TestISISNETValidator`, `TestISISSystemIDValidator`, boundary tests for metric/priority/hold-multiplier/hello-interval/lifetimes
   - Files: `internal/component/isis/yang/ze-isis-conf.yang`, validator registration
   - Verify: native validation rejects out-of-range/enum/pattern violations; custom NET validator rejects bad NETs and offers `CompleteFn`
3. **Phase: Config resolution** - typed structs, defaults, validation, system-id derivation
   - Tests: `TestISISConfigResolve`, `TestISISConfigDefaults`, `TestISISConfigValidate`
   - Files: `internal/component/isis/config.go`
   - Verify: full subtree resolves with defaults; missing NET rejected; system-id derived from NET
4. **Phase: Events** - IS-IS event namespace
   - Tests: `TestISISEventNamespace`
   - Files: `internal/component/isis/events.go`
   - Verify: namespace + session up/down + LSP-change types register without collision
5. **Phase: Lifecycle + circuits + PDU dispatcher** - `OnConfigVerify`/`OnConfigure`/`OnConfigApply`/`OnStarted`/`OnExecuteCommand`, circuit open via isis-3, the PDU-type receive dispatcher (keyed by the 5-bit PDU type, with stub handlers; isis-3 delivers `(ifindex, pdu)` to it and holds no protocol switch), journal reconcile, clean shutdown
   - Tests: `TestISISComponentStart` (now passes), `TestISISConfigApplyReconcile`
   - Files: `internal/component/isis/server.go`, `internal/component/isis/register.go`
   - Verify: engine opens circuits for enabled interfaces; the dispatcher routes PDU types to the right (stub) handler; reload reconciles one circuit; shutdown stops circuits
6. **Functional test + suite registration** - register the `test/isis` suite in `internal/test/cli/register.go` and `mk/test-functional.mk`; write `test/isis/isis-config.ci`: valid config validates and starts; invalid NET rejected; confirm `make ze-isis-test` runs it
7. **Full verification** - `make ze-verify`
8. **Complete spec** - fill audit tables, write learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; the wiring test passes end-to-end |
| Feature completeness | The skeleton matches the LDP reference shape (registration fields, all SDK callbacks present); circuits open via isis-3 |
| Correctness | Defaults match the leaf table (level `l1-l2`, lsp-lifetime 1200, metric 10, hello 10, hold-multiplier 3, priority 64); system-id derived correctly from NET |
| Naming | YANG kebab-case; Go fields PascalCase of leaves; CLI `show isis <noun>`; module `ze-isis-conf` |
| Data flow | Config flows tree -> schema -> SDK callbacks -> engine -> transport; no second config path |
| CLI grammar | Command names are action-before-identifier (`show isis <noun>`) |
| YANG validation | No bare `type string` on numeric/enum/id leaves; NET/system-id have custom validator + `CompleteFn` |
| Rule: registration-dispatch | IS-IS registers; core discovers via registry, no direct import outside generated `all.go` |
| Rule: no hand-edit of generated files | `all.go`, `yang/register.go`, `yang/embed.go` produced only by `make generate` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| IS-IS component directory | `ls internal/component/isis/` |
| YANG module | `ls internal/component/isis/yang/ze-isis-conf.yang` |
| Generated glue | `ls internal/component/isis/yang/register.go internal/component/isis/yang/embed.go` |
| `all.go` wired | `grep isis internal/component/plugin/all/all.go` |
| Functional test | `ls test/isis/isis-config.ci` |
| Test suite registered + runs | `grep isis internal/test/cli/register.go mk/test-functional.mk`; `make ze-isis-test` runs `test/isis/isis-config.ci` under the functional-test runner and passes |
| Component registered | `ze plugin` inventory shows `isis` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | NET and system-id validated before use; every numeric leaf range-bounded; reject malformed config in `OnConfigVerify` before mutating state |
| Resource handling | Circuit goroutines and the transport are stopped cleanly on shutdown; no leaked goroutines on reload |
| Privilege | Raw-socket privilege (`CAP_NET_RAW`) handling is owned by spec-isis-3; this spec must not silently swallow an EPERM at circuit open |
| Error leakage | Config errors are descriptive but do not echo secrets (auth keys parsed here are masked per `ze:sensitive` if applicable) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior (LDP register.go) |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| `all.go` missing the import | Run `make generate`; never hand-edit |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## Design Insights

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- This spec wires a skeleton only: the component starts, ingests config, and opens circuits, but forms no adjacencies and computes no routes (siblings isis-5+ deliver runtime behaviour)
- Authentication leaves are parsed and stored but not verified/signed (spec-isis-10-auth)
- `show isis` commands are declared and dispatched but return stub data (spec-isis-13-cli-diag-interop)

## RFC Documentation

Add `// ISO/IEC 10589 Section X.Y: "<quoted requirement>"` (and 5305 for wide metric
range, 5301 for the dynamic hostname leaf) above the enforcing config-validation
code where a NET/system-id/metric constraint maps to a normative requirement.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `isis { ... }` reaches a running engine | wiring test | `TestISISComponentStart` |
| Config accepted and validated, invalid NET rejected | functional test | `test/isis/isis-config.ci` |
| Component in plugin inventory; `make generate` wires `all.go` | inventory + grep | `ze plugin`, `grep isis internal/component/plugin/all/all.go` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/isis/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (runtime stays in siblings)
- [ ] Single responsibility (component + config wiring only)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (transport behind the isis-3 interface)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests (N/A: no wire protocol in this spec; covered by siblings)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-4-component-config.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-isis-4-component-config.md`

# Spec: ospf-4-component-config

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-2-wire.md, spec-ospf-3-ip-transport.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - authoritative scope (this child is row ospf-4); Shared Contracts "Area + interface config model", "Authentication config model", "Packet receive dispatcher", "Command + API YANG"
4. `internal/plugins/ldp/register.go` and `internal/plugins/isis/register.go` - closest registration + SDK lifecycle templates
5. `internal/plugins/isis/yang/` (embed.go, register.go, ze-isis-conf.yang) - generated YANG glue pattern OSPF copies
6. `internal/component/config/validators_register.go` (`RegisterValidators`) - where the central ValidateFns live (cycle-break; CompleteFns stay component-owned)
7. `internal/component/config/yang_schema.go` lines 203-231 - `ze-*-conf.yang` discovery/merge; `make generate` updates `internal/component/plugin/all/all.go`
8. `docs/research/ospf-implementation-guide.md` section 10 (Configuration Shape, lines 591-769) and section 11 (Plugin Model and Code Organisation, lines 773-856)

## Task

Create the `internal/plugins/ospf/` edge plugin and register it so a top-level
`ospf { ... }` config block reaches a running OSPFv2 engine. This is the **wiring
backbone** of the OSPF spec set (the mandatory first runtime spec to implement):
the integration skeleton that every runtime child (ISM, NSM, LSDB/flooding, SPF,
inter-area ABR, AS-external ASBR, stub/NSSA, auth, CLI) depends on. After this
spec, Ze has an OSPF edge plugin that registers, accepts and validates the `ospf`
config subtree, resolves it to typed Go structs, applies YANG defaults, and
starts an engine that opens the raw IP socket via the spec-ospf-3 transport,
enrols its configured interfaces, and then does nothing else yet. The runtime
behaviour (sending Hellos, forming adjacencies, building per-area LSDBs,
flooding, SPF, route install) is delivered by the sibling specs
(`spec-ospf-5-interface-ism` and later); here the goroutines those specs fill are
stubs and the packet receive dispatcher routes to stub handlers.

Concretely this spec delivers, modelled on `internal/plugins/ldp/register.go`
and `internal/plugins/isis/register.go`:

- A `registry.Registration` in `init()` with `Name "ospf"`, a description,
  `Features "yang"`, `YANG` set to the embedded `ze-ospf-conf.yang`,
  `ConfigRoots ["ospf"]`, `Dependencies ["fib-kernel", "sysctl"]`, the matching
  `RFCs`, `RunEngine runOSPFEngine`, plus `ConfigureEngineLogger`,
  `ConfigureMetrics`, `ConfigureEventBus`, and a `CLIHandler`.
- The SDK lifecycle `runOSPFEngine(conn net.Conn) int`: `sdk.NewWithConn("ospf",
  conn)`, then `OnConfigVerify` (parse-check only), `OnConfigure` (parse to typed
  config), `OnConfigApply` (reconcile against the running engine via a journal so
  reloads add/remove areas and interfaces incrementally, not restart-everything),
  `OnStarted` (open the raw IP socket via spec-ospf-3, enrol each enabled
  interface, join `224.0.0.5` per enabled interface, launch the per-interface
  goroutines as stubs that later specs fill), and `OnExecuteCommand` (dispatch the
  `show ip ospf` commands, stubbed here), then `p.Run(ctx,
  sdk.Registration{WantsConfig:["ospf"], Commands:[...]})` with a clean shutdown
  that leaves multicast groups, closes the socket, and stops goroutines.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang`: a top-level `container ospf`
  with `ze:config-root "ospf"` and maximally-validated leaves per
  `ai/patterns/config-option.md` (the leaf table is in the TDD section below):
  router-id, reference-bandwidth, maximum-paths (ECMP cap 8), SPF timers,
  `default-information originate`, `redistribute` hooks; `areas/area` with
  area-id, area-type, no-summary, default-cost, ranges; per-interface enrolment
  binding an interface to an area (per-interface, NOT FRR `network <prefix>
  area`) with network-type, cost, hello/dead intervals, priority, passive,
  mtu-ignore, retransmit-interval, transmit-delay, and authentication references.
- Config resolution that parses the `ospf` JSON subtree to typed Go structs,
  applies YANG defaults, and validates required fields (a resolvable router-id;
  every interface binds a declared area).
- Two custom validators registered in `validators_register.go` (central, to break
  the config import cycle; CompleteFns stay component-owned): `ospf-router-id`
  (dotted-quad) and `ospf-area-id` (dotted-quad or integer).
- `events.RegisterNamespace` for OSPF event types (neighbor up/down, SPF run,
  LSDB change) consumed by later specs.
- The packet receive dispatcher in `instance.go`, keyed by the common-header
  `Type` field, that ospf-5/6/7 register handlers against; spec-ospf-3 delivers
  `(ifindex, src, payload)` to it and holds no protocol switch (per Shared
  Contracts "Packet receive dispatcher", owner ospf-4).
- `make generate` discovering the new `internal/plugins/ospf` and
  `internal/plugins/ospf/yang` packages and adding them to
  `internal/component/plugin/all/all.go` (generated, never hand-edited),
  including the generated `yang/register.go` and `yang/embed.go` glue.

The wiring test `TestOSPFComponentStart` proves the chain: config `ospf { ... }`
present, component registers, engine runs, the raw socket opens, interfaces are
enrolled. A functional `.ci` under `test/ospf/` loads an OSPF config and confirms
the component is up.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/ldp/register.go`, `internal/plugins/isis/register.go` - closest registration + SDK lifecycle templates
  -> Decision: model `init()` registration and `runOSPFEngine` directly on the LDP/IS-IS `register*`/`run*Engine` pairs (registry.Registration fields, OnConfigVerify/OnConfigure/OnConfigApply/OnStarted/OnExecuteCommand, `p.Run`, clean shutdown)
  -> Constraint: registration not switch dispatch (`ai/rules/registration-dispatch.md`): OSPF registers; core discovers it through the registry, never imports it directly
- [ ] `internal/plugins/isis/yang/` (embed.go, register.go, ze-isis-conf.yang) - generated YANG glue pattern
  -> Constraint: `yang/register.go` and `yang/embed.go` are `// Code generated ... DO NOT EDIT.`; they are produced by `scripts/codegen/yang_glue.go` via `make generate`, never hand-written
- [ ] `docs/research/ospf-implementation-guide.md` section 10 (Configuration Shape, lines 591-769) - the first-pass YANG shape and the configuration decisions
  -> Decision: per-interface area binding (not FRR `network <prefix> area`); ECMP default-on cap 8; per-interface auth with `inherit`; no TOS routing; no virtual links in v1
  -> Constraint: the leaf set and defaults (hello 10, dead 40, priority 1, reference-bandwidth, SPF timers) come from §10; this spec validates them maximally and resolves them to typed structs
- [ ] `docs/research/ospf-implementation-guide.md` section 11 (Plugin Model and Code Organisation, lines 773-856) - the `internal/plugins/ospf/` file layout and integration points
  -> Constraint: this spec creates `register.go` / `config.go` / `instance.go` / `area.go` / `events.go` / `yang/`; the per-packet-type codec, ISM, NSM, LSDB, SPF directories are stubs or created by siblings
- [ ] `plan/learned/926-isis-0-umbrella.md`, `plan/learned/930-isis-4-component-config.md` - the sibling IS-IS umbrella + wiring-backbone child; OSPF copies the component/config/sysrib/redistribution conventions verbatim
  -> Constraint: copy the component registration, the central-ValidateFn cycle-break, the test-suite registration, and the metrics-table conventions; do NOT couple OSPF to IS-IS code
- [ ] `ai/patterns/config-option.md` - YANG leaf + module registration + validator pattern
  -> Constraint: native validation maximised (`range`/`length`/`pattern`/`enumeration`); custom `ze:validate` + `ValidateFn` + `CompleteFn` only where native is insufficient (router-id, area-id)
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md` - YANG vs env var, kebab-case naming
  -> Constraint: OSPF config is YANG (no env vars), top-level `ospf` container, kebab-case leaves, Go struct fields are PascalCase of the leaf
- [ ] `ai/rules/wiring-completeness.md`, `ai/rules/plugin-self-containment.md` - reachability and self-containment
  -> Constraint: every exported symbol has a non-test caller; all OSPF schema/help/commands live under `internal/plugins/ospf/`; no `ospf` spelling in generic/central packages except generated `all.go` and the central ValidateFn cycle-break

### RFC Summaries (MUST for protocol work; existing, read before implementation)
- [ ] `rfc/short/rfc2328.md` - OSPF Version 2 base standard (created during ospf-2/5/6/7/8; referenced in `RFCs`)
  -> Constraint: the interface-parameter set (Hello/Router-Dead/Rxmt/InfTransDelay intervals, Router Priority, interface Cost) and area structure constrain the config leaves and defaults
- [ ] `rfc/short/rfc9129.md` - YANG Data Model for OSPF (created by ospf-4; informs the schema shape)
  -> Constraint: per-interface area binding and the area/interface container hierarchy follow RFC 9129, not FRR's `network` matching

**Key insights:**
- OSPF is a component (`internal/plugins/ospf/`), not a plugin dir; engine-owning protocols (BGP, LDP, RSVP-TE, IS-IS) live in `component/`
- The component is registered exactly like LDP/IS-IS; the only OSPF-specific config concern is the router-id / area-id custom validators with `CompleteFn`
- `make generate` auto-wires `all.go` and the `yang/` glue; hand-editing generated files is forbidden
- This spec wires a skeleton: config in, engine up, raw socket open, interfaces enrolled, nothing else; runtime lives in sibling specs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] No `internal/plugins/ospf/` directory and no `ospf` registry entry exist today
  -> Constraint: this spec creates the directory, the registration, and the YANG module from scratch
- [ ] No top-level `ospf` config container exists; `internal/component/config/yang_schema.go` only merges `-conf` modules that are registered, and none is `ze-ospf-conf`
  -> Constraint: the `ospf` container only appears once `ze-ospf-conf.yang` is registered via generated glue and `make generate` runs
- [ ] `internal/component/plugin/all/all.go` is generated and does not import an `ospf` package
  -> Constraint: never hand-edit `all.go`; `make generate` adds the import after the package exists
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` already carries an `ospf` admin-distance leaf (default 110); `sysrib.go` `adminDist` map already has `"ospf": 110`
  -> Constraint: this spec adds NO sysrib leaf; the admin distance already exists (umbrella Existing Foundation). FIB install (ospf-8) reuses it
- [ ] LDP / IS-IS demonstrate the full pattern: `register*` builds `registry.Registration`, `run*Engine` wires the SDK callbacks and `p.Run`; the IS-IS sibling put its NET/system-id ValidateFns in the central config package (cycle-break) while keeping CompleteFns component-owned
  -> Constraint: OSPF mirrors this pattern; differences are config shape (router-id/area/per-interface enrolment) and raw-socket open + multicast join (via ospf-3) instead of UDP/TCP discovery

**Behavior to preserve:**
- BGP, LDP, RSVP-TE, IS-IS, static, connected route sources remain independent and functional
- The YANG schema discovery/merge in `yang_schema.go` is unchanged (OSPF adds a module, does not change the loader)
- `internal/component/plugin/all/all.go` remains generated only; no manual edits
- The existing `ospf` sysrib admin-distance leaf and `adminDist` entry are untouched

**Behavior to change:**
- New top-level `ospf` config container and `internal/plugins/ospf/` component
- A new OSPF event namespace registered for neighbor up/down, SPF run, LSDB change
- `instance.go` owns a new packet-type receive dispatcher (keyed by the common-header Type) that ospf-5/6/7 register handlers against
- `make generate` adds `internal/plugins/ospf` and `internal/plugins/ospf/yang` to `all.go`
- A new `test/ospf` functional-test suite registered in `internal/test/cli/register.go` and `mk/test-functional.mk`

## Data Flow (MANDATORY)

### Entry Point
- Config arrives as the `ospf` subtree of the YANG-validated config tree, delivered to the engine as a JSON `sdk.ConfigSection` (Root `ospf`), exactly as LDP/IS-IS receive their sections
- Interface up/down + address add/remove events (from the iface EventBus, used to enrol/withdraw interfaces and (re)join multicast) are consumed by the engine started in `OnStarted` via the spec-ospf-3 transport

### Transformation Path
1. **Schema:** `ze-ospf-conf.yang` is registered at init (generated glue) and its top-level `ospf` container is merged into the config schema by `yang_schema.go`
2. **Validate (parse-check):** the SDK invokes `OnConfigVerify`; the engine parses the JSON subtree and rejects malformed config (bad router-id, bad area-id, an interface bound to an undeclared area) without mutating state
3. **Configure (typed structs):** the SDK invokes `OnConfigure`; the engine parses the subtree into typed Go structs (instance + per-area + per-interface) and applies YANG defaults, producing the staged config
4. **Apply (reconcile):** `OnConfigApply` diffs the staged config against the running config via a journal and reconciles areas/interfaces incrementally (add/remove an enrolled interface or area without restarting the engine)
5. **Start (socket + enrol):** `OnStarted` opens the raw IP socket (proto 89) via the spec-ospf-3 transport, enrols each enabled, non-passive interface (join `224.0.0.5`, bind source, TTL 1), and launches per-interface goroutines (stubs in this spec; later specs fill RX/TX/timers/ISM/NSM)
6. **Receive dispatch:** the spec-ospf-3 transport delivers `(ifindex, src netip.Addr, payload []byte)` after stripping the IP header and does NOT switch on packet type. `instance.go` owns the packet-type receive dispatcher, keyed by the common-header `Type` (1 Hello, 2 DD, 3 LS Request, 4 LS Update, 5 LS Ack), that validates version==2 / area match / checksum / auth then routes: Hello -> ISM/neighbour discovery (ospf-5); DD / LS Request -> NSM exchange (ospf-6); LS Update / LS Ack -> LSDB/flooding (ospf-7). Handlers register at startup; this spec installs the dispatcher and stub handlers, later specs fill the real handlers (per Shared Contracts "Packet receive dispatcher", owner ospf-4)
7. **Commands:** `OnExecuteCommand` dispatches `show ip ospf ...` (stubbed here; later specs return real snapshots)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> engine | SDK `OnConfigVerify`/`OnConfigure`/`OnConfigApply` (JSON `ospf` subtree) | [ ] |
| Engine <-> IP transport | spec-ospf-3 `Open()` / per-interface enrol (multicast join, source bind) | [ ] |
| Transport <-> packet dispatcher | spec-ospf-3 delivers `(ifindex, src, payload)`; `instance.go` dispatcher routes by common-header Type (no packet switch in transport) | [ ] |
| Engine <-> EventBus | `events.RegisterNamespace` + emit (neighbor up/down, SPF run, LSDB change) | [ ] |
| CLI <-> engine | `OnExecuteCommand` dispatch keyed by `show ip ospf <noun>` | [ ] |
| Schema <-> loader | `ze-ospf-conf.yang` `-conf` suffix auto-merge in `yang_schema.go` | [ ] |

### Integration Points
- New component `internal/plugins/ospf/` (register.go, config.go, events.go, instance.go, area.go) plus `yang/`
- `instance.go` packet-type receive dispatcher (keyed by the common-header Type) that ospf-5/6/7 register handlers against; spec-ospf-3 delivers `(ifindex, src, payload)` to it and holds no protocol switch
- `ze-ospf-conf.yang` registered through generated `yang/register.go`
- `internal/component/plugin/all/all.go` regenerated by `make generate` to import the new packages
- spec-ospf-3 IP transport (raw socket open, per-interface multicast enrol/withdraw) consumed by `OnStarted`
- iface EventBus link up/down + address change drives interface enrol/withdraw and multicast (re)join (wired here, acted on by ospf-5)
- `internal/component/config/validators_register.go` central ValidateFns for router-id/area-id (cycle-break; CompleteFns registered from the component)

### Architectural Verification
- [ ] No bypassed layers (config -> schema -> SDK callbacks -> engine -> transport)
- [ ] No unintended coupling (OSPF independent of IS-IS; transport behind the ospf-3 interface)
- [ ] No duplicated functionality (registration reuses the registry; no second config path; no new sysrib leaf)
- [ ] Generated files untouched (`all.go`, `yang/register.go`, `yang/embed.go` produced by `make generate`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `make generate` discovers a new `component/ospf` + `yang` package automatically | LDP/IS-IS precedent; `yang_schema.go` 203-231; umbrella A-5 | Manual `all.go` edit needed (forbidden) | build + `grep ospf all.go` after `make generate` | unvalidated |
| A-2 | The SDK ConfigSection JSON for the `ospf` subtree carries every leaf shape the typed parse expects (nested `areas/area`, `interfaces/interface`, `ranges/range`) | LDP/IS-IS read their subtrees the same way | Parse misses nested containers (area list, interface list, range list) | `TestOSPFConfigResolve` against a full config | unvalidated |
| A-3 | spec-ospf-3 exposes a transport `Open`/per-interface enrol/`Close` API the engine can call from `OnStarted` | umbrella architecture (transport behind an interface; RSVP-TE raw-IP precedent) | Wiring stub cannot open the socket/enrol; ospf-4 blocked on ospf-3 API shape | `TestOSPFComponentStart` opening over a STUB transport backend (no real raw socket -- the real socket round-trip is ospf-3's QEMU integration test); this unit test depends only on the ospf-3 transport interface | unvalidated |
| A-4 | A top-level `ospf` container with `ze:config-root "ospf"` and `ConfigRoots:["ospf"]` routes the subtree to the engine | LDP/IS-IS use top-level `ldp`/`isis` + matching `ConfigRoots` | Config never reaches `OnConfigVerify`; validate passes vacuously | `test/ospf/ospf-config.ci` exercising validate + start | unvalidated |
| A-5 | The router-id can always be resolved (configured dotted-quad, else derived from a loopback/highest interface address per RFC 2328 §C.1) | guide §10 ("if omitted, derive from a loopback"); RFC 2328 | Need an explicit required router-id leaf with no derivation | `TestOSPFConfigValidate` router-id-omitted derivation case | unvalidated |
| A-6 | The central config package can host the router-id/area-id ValidateFns without importing `ospf` (IS-IS cycle-break precedent) | IS-IS put its NET/system-id ValidateFns in `validators_register.go`; component keeps CompleteFns | ValidateFn must live in the component, forcing a different registration path | build with no import cycle; `TestOSPFRouterIDValidator` in the config package | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Bare `type string` leaves slip in (router-id, area-id, prefix) instead of validated types | YANG validation review flags it | Custom `ze:validate` + pattern; router-id/area-id validators with `CompleteFn`; prefix uses an `ip-prefix` type |
| R-2 | Reload restarts every interface instead of reconciling | interface flap on any config change | `OnConfigApply` journal-diff reconcile (ospf-4), not restart-all |
| R-3 | `make generate` not run, so `all.go` lacks the import and the component never loads | component absent from `ze plugin` inventory | Wiring test asserts the component is in the inventory; run `make generate` in the wiring phase |
| R-4 | Interface-enrol stub couples to ospf-3 internals instead of its interface | ospf-3 API churn breaks ospf-4 build | Depend only on the ospf-3 transport interface; keep the stub behind it |
| R-5 | Per-interface area binding and area-type interactions (stub `no-summary`, `default-cost`) are mis-modelled, leaking semantics into the schema | review against RFC 9129 / guide §10 flags it | Keep ospf-4 to parse + store; the area-type semantics (Type 5 suppression, default injection) are ospf-11, not here |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config `ospf { ... }` present | -> | `registerOSPF` registration + `runOSPFEngine` start, raw socket opened, interfaces enrolled | `TestOSPFComponentStart` |
| config `ospf { ... }` present | -> | component appears in the plugin inventory after `make generate` wires `all.go` | `TestOSPFComponentStart` (inventory assertion) |
| `ospf { ... }` loaded via `ze config validate` | -> | `OnConfigVerify` accepts a valid config, rejects a bad router-id and an interface bound to an undeclared area | `test/ospf/ospf-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `ospf { router-id 10.0.0.1; areas { area 0.0.0.0 { } } interfaces { interface eth0 { area 0.0.0.0 } } }` present | Component registers, `runOSPFEngine` starts, engine reaches `OnStarted`, the raw IP socket opens and the enabled interface is enrolled (multicast `224.0.0.5` joined on a stub backend) |
| AC-2 | Config with a full `ospf` block (router-id, reference-bandwidth, maximum-paths, SPF timers, default-information originate, multiple areas with area-type/default-cost/ranges, multiple interfaces with network-type/cost/hello/dead/priority/passive/auth refs) | `OnConfigure` resolves it to typed structs with YANG defaults applied; `TestOSPFConfigResolve` matches expected struct |
| AC-3 | Config with no `router-id` leaf and no derivable address | `OnConfigVerify` rejects with a clear error (router-id required or underivable); with a loopback present, the router-id derives (AC-9) |
| AC-4 | Config with an invalid `router-id` (not a dotted-quad) | `OnConfigVerify` rejects via the `ospf-router-id` custom validator before the engine mutates state |
| AC-5 | Config with `area-type stub-and-a-half` (not in the enum) | YANG native enum validation rejects it at schema validation, before the engine |
| AC-6 | Config with interface `cost 0` or `cost 65536` (below/above the 1..65535 range) | YANG native range validation rejects it |
| AC-7 | `ze plugin` inventory queried after build | `ospf` appears as a registered component (proves `make generate` wired `all.go`) |
| AC-8 | Config reload changing one interface cost | `OnConfigApply` reconciles only the affected interface via the journal; other interfaces are not withdrawn/re-enrolled |
| AC-9 | Config with no `router-id` but a loopback / highest interface address present | A router-id is derived (RFC 2328 §C.1 highest-loopback-then-highest-interface rule); engine starts |
| AC-10 | Tab-completion on `router-id` / area `area-id` in the editor | The custom validator `CompleteFn` returns guidance values for the field |
| AC-11 | Config binding an interface to an `area` not declared under `areas/area` | `OnConfigVerify` rejects (every interface must bind a declared area) |
| AC-12 | `bin/ze-test ospf --all` invoked (suite registered in `internal/test/cli/register.go` and `mk/test-functional.mk`) | The `test/ospf` suite resolves and `test/ospf/ospf-config.ci` runs under the functional-test runner (`make ze-ospf-test`), passing |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes `ospf { router-id ...; areas { area 0.0.0.0 {} } interfaces { interface eth0 { area 0.0.0.0 } } }` and runs `ze config validate` | config file -> YANG schema -> validate -> `OnConfigVerify` accept | `test/ospf/ospf-config.ci` |
| 2 | Mistypes the router-id and runs `ze config validate` | config file -> YANG schema -> validate -> router-id validator reject | `test/ospf/ospf-config.ci` (invalid case) |
| 3 | Binds an interface to an undeclared area and runs `ze config validate` | config file -> YANG schema -> validate -> `OnConfigVerify` cross-check reject | `test/ospf/ospf-config.ci` (undeclared-area case) |
| 4 | Starts Ze with an `ospf` block | config -> component -> `runOSPFEngine` -> `OnStarted` -> raw socket open + interface enrol | `TestOSPFComponentStart` |
| 5 | Lists plugins to confirm OSPF is present | `ze plugin` -> registry inventory | `TestOSPFComponentStart` (inventory assertion) |
| 6 | Reloads config with a changed interface cost | reload -> `OnConfigApply` -> journal diff -> reconcile one interface | `TestOSPFConfigApplyReconcile` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFConfigResolve` | `internal/plugins/ospf/config_test.go` | Full `ospf` subtree (areas/area, interfaces/interface, ranges/range) parses to typed structs with YANG defaults applied | |
| `TestOSPFConfigDefaults` | `internal/plugins/ospf/config_test.go` | Omitted leaves resolve to YANG defaults (reference-bandwidth, maximum-paths 8, hello-interval 10, dead-interval 40, priority 1, retransmit-interval 5, transmit-delay 1, area-type normal, SPF delay/hold defaults) | |
| `TestOSPFConfigValidate` | `internal/plugins/ospf/config_test.go` | Router-id resolves (configured or derived); interface bound to an undeclared area rejected; engine-required fields present | |
| `TestOSPFRouterIDValidator` | `internal/component/config/validators_ospf_test.go` | `ospf-router-id` validator accepts dotted-quads, rejects non-dotted-quad/out-of-range octets; `CompleteFn` returns guidance | |
| `TestOSPFAreaIDValidator` | `internal/component/config/validators_ospf_test.go` | `ospf-area-id` validator accepts dotted-quad and integer forms, rejects malformed; `CompleteFn` returns guidance (`0.0.0.0`) | |
| `TestOSPFComponentStart` | `internal/plugins/ospf/instance_test.go` | Registration present; `runOSPFEngine` reaches `OnStarted`; raw socket opens on a stub backend; an enabled interface is enrolled (multicast join recorded); component in inventory | |
| `TestOSPFConfigApplyReconcile` | `internal/plugins/ospf/instance_test.go` | `OnConfigApply` reconciles only the changed interface (journal diff), no full restart, no spurious multicast leave/join on unrelated interfaces | |
| `TestOSPFEventNamespace` | `internal/plugins/ospf/events_test.go` | `events.RegisterNamespace` registers neighbor up/down, SPF-run, and LSDB-change types without collision | |
| `TestOSPFPacketDispatch` | `internal/plugins/ospf/instance_test.go` | The packet-type dispatcher routes each common-header Type (1 Hello, 2 DD, 3 LS Request, 4 LS Update, 5 LS Ack) to its registered handler; unknown/short/wrong-version packets are dropped, not panicked | |
| `TestOSPFInterfaceEnrolment` | `internal/plugins/ospf/config_test.go` | Only enabled interfaces bound to a declared area enrol; passive interfaces are enrolled-but-silent (originate, no Hellos); disabled interfaces are skipped | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interface cost | 1..65535 | 65535 | 0 | 65536 |
| Router priority | 0..255 | 255 | N/A | 256 |
| Hello interval | 1..65535 | 65535 | 0 | 65536 |
| Dead interval | 1..65535 | 65535 | 0 | 65536 |
| Retransmit interval | 1..65535 | 65535 | 0 | 65536 |
| Transmit delay | 0..3600 | 3600 | N/A | 3601 |
| Maximum paths (ECMP) | 1..32 | 32 | 0 | 33 |
| Range cost | 0..16777215 | 16777215 | N/A | 16777216 |

### YANG Leaf Table (ze-ospf-conf.yang)

Top-level `container ospf` with `ze:config-root "ospf"`. Leaves (described, not
pasted as a YANG block). Per the guide §10 configuration decisions: per-interface
area binding (NOT FRR `network <prefix> area`), ECMP default-on cap 8,
per-interface auth with `inherit`, no TOS, no virtual links.

| Leaf / node | Type | Constraints | Default | Description |
|-------------|------|-------------|---------|-------------|
| `router-id` | leaf string | `ze:validate "ospf-router-id"` (custom dotted-quad validator + `CompleteFn`) | derived (RFC 2328 §C.1) | Router ID; if omitted, derived from the highest loopback then highest interface address |
| `reference-bandwidth` | leaf uint32 | `range "1..4294967"`, units Mbps | `100000` | Auto-cost reference bandwidth (cost = ref-bw / link-bw) |
| `maximum-paths` | leaf uint8 | `range "1..32"` | `8` | Maximum ECMP paths per prefix (cap 8 default, guide §10) |
| `default-information` | container | - | - | Originate a default route as an AS-External LSA |
| `default-information/originate` | leaf boolean | - | `false` | Originate a default Type 5 (semantics ospf-10) |
| `default-information/always` | leaf boolean | - | `false` | Originate even with no default in the RIB |
| `default-information/metric` | leaf uint32 | `range "0..16777215"` | `1` | Metric of the originated default |
| `default-information/metric-type` | leaf enumeration | enum `type-1` / `type-2` | `type-2` | External metric type of the default |
| `timers` | container | - | - | SPF / LSA throttle timers |
| `timers/spf-delay-ms` | leaf uint32 | `range "0..600000"`, units ms | `50` | Initial SPF delay |
| `timers/spf-hold-ms` | leaf uint32 | `range "0..600000"`, units ms | `200` | SPF hold (exponential back-off floor) |
| `timers/spf-max-hold-ms` | leaf uint32 | `range "0..600000"`, units ms | `5000` | SPF max hold |
| `timers/min-ls-interval-ms` | leaf uint32 | `range "0..600000"`, units ms | `5000` | MinLSInterval between re-originations of an LSA |
| `timers/min-ls-arrival-ms` | leaf uint32 | `range "0..600000"`, units ms | `1000` | MinLSArrival before accepting a new instance |
| `redistribute` | list (key `source`) | - | - | Redistribution hooks (semantics ospf-10) |
| `redistribute/source` | leaf enumeration | enum `connected` / `static` / `kernel` / `bgp` / `isis` | - | Route source to import as Type 5 |
| `redistribute/metric` | leaf uint32 | `range "0..16777215"` | `20` | Injected metric |
| `redistribute/metric-type` | leaf enumeration | enum `type-1` / `type-2` | `type-2` | External metric type |
| `redistribute/tag` | leaf uint32 | `range "0..4294967295"` | `0` | External route tag |
| `areas` | container | - | - | OSPF areas |
| `areas/area` | list (key `area-id`) | - | - | Per-area configuration |
| `areas/area/area-id` | leaf string | key, `ze:validate "ospf-area-id"` (dotted-quad or integer + `CompleteFn`) | - | Area identifier (`0.0.0.0` = backbone) |
| `areas/area/area-type` | leaf enumeration | enum `normal` / `stub` / `nssa` | `normal` | Area type (semantics ospf-11) |
| `areas/area/no-summary` | leaf boolean | - | `false` | Totally-stubby / totally-NSSA: suppress Type 3 summaries (ospf-11) |
| `areas/area/default-cost` | leaf uint32 | `range "0..16777215"` | `1` | Cost of the default the ABR injects into a stub/NSSA area (ospf-11) |
| `areas/area/authentication` | container | - | - | Area-level authentication default used by interfaces whose auth mode is `inherit` (ospf-12) |
| `areas/area/authentication/key-chain` | leaf string | leafref/name into `key-chains` | - | Area key chain inherited by interfaces in this area unless overridden |
| `areas/area/ranges` | container | - | - | Address ranges for inter-area summarisation |
| `areas/area/ranges/range` | list (key `prefix`) | - | - | A summarised range (semantics ospf-9) |
| `areas/area/ranges/range/prefix` | leaf string | key, `type ze-types:ip-prefix` (CIDR) | - | The aggregate prefix, e.g. `10.0.0.0/16` |
| `areas/area/ranges/range/advertise` | leaf enumeration | enum `advertise` / `not-advertise` | `advertise` | Advertise the aggregate or suppress it |
| `areas/area/ranges/range/cost` | leaf uint32 | `range "0..16777215"` | - | Override cost for the aggregate Type 3 |
| `interfaces` | container | - | - | OSPF-enabled interfaces |
| `interfaces/interface` | list (key `name`) | - | - | Per-interface OSPF configuration |
| `interfaces/interface/name` | leaf string | key | - | OS interface name |
| `interfaces/interface/area` | leaf string | `ze:validate "ospf-area-id"` (per-interface area binding) | - | Area this interface belongs to; MUST reference a declared area |
| `interfaces/interface/enabled` | leaf boolean | - | `true` | OSPF enabled on this interface |
| `interfaces/interface/network-type` | leaf enumeration | enum `broadcast` / `point-to-point` | `broadcast` | Network type (NBMA/P2MP out of scope v1) |
| `interfaces/interface/cost` | leaf uint16 | `range "1..65535"` | derived | Interface output cost (else reference-bandwidth / link-bw) |
| `interfaces/interface/hello-interval` | leaf uint16 | `range "1..65535"`, units seconds | `10` | Hello interval |
| `interfaces/interface/dead-interval` | leaf uint16 | `range "1..65535"`, units seconds | `40` | Router-dead interval (commonly 4x hello) |
| `interfaces/interface/priority` | leaf uint8 | `range "0..255"` | `1` | DR/BDR election priority (0 = ineligible) |
| `interfaces/interface/passive` | leaf boolean | - | `false` | Originate the interface route but send no Hellos / form no adjacency |
| `interfaces/interface/mtu-ignore` | leaf boolean | - | `false` | Skip the DD MTU mismatch check (ospf-6) |
| `interfaces/interface/retransmit-interval` | leaf uint16 | `range "1..65535"`, units seconds | `5` | LSA retransmit interval |
| `interfaces/interface/transmit-delay` | leaf uint16 | `range "0..3600"`, units seconds | `1` | Estimated LSA transmission delay (InfTransDelay) |
| `interfaces/interface/authentication` | container | - | - | Per-interface authentication (semantics ospf-12) |
| `interfaces/interface/authentication/mode` | leaf enumeration | enum `inherit` / `none` / `simple` / `md5` / `hmac-sha-1` / `hmac-sha-256` / `hmac-sha-384` / `hmac-sha-512` | `inherit` | Auth mode; `inherit` picks up the area key (guide §10) |
| `interfaces/interface/authentication/key-chain` | leaf string | leafref/name into `key-chains` | - | Reference into the keychain list |
| `key-chains` | list (key `name`) | - | - | Named authentication key chains for hitless rotation (semantics ospf-12) |
| `key-chains/name` | leaf string | key, `length "1..63"` | - | Key-chain name referenced by per-interface auth |
| `key-chains/key` | list (key `key-id`) | - | - | Keys in this chain |
| `key-chains/key/key-id` | leaf uint32 | key, `range "0..4294967295"` | - | Key identifier; AuType 2 send path rejects values above 255, AuType 3 uses the full 32-bit field |
| `key-chains/key/algorithm` | leaf enumeration | enum `simple` / `md5` / `hmac-sha-1` / `hmac-sha-256` / `hmac-sha-384` / `hmac-sha-512` | `md5` | Authentication algorithm (AuType 1 simple, AuType 2 crypto, or AuType 3 RFC 7474 trailer) |
| `key-chains/key/secret` | leaf string | `length "1..255"`, `ze:sensitive` ($9$-encoded) | - | The shared secret, masked and `$9$`-encoded at rest |
| `key-chains/key/send-lifetime` | container | optional start/end timestamps | - | When this key may be used to sign (hitless rotation) |
| `key-chains/key/accept-lifetime` | container | optional start/end timestamps | - | When this key is accepted on receive (hitless rotation) |

Per-interface area binding (Shared Contracts "Area + interface config model"):
each `interfaces/interface/area` references a declared `areas/area`. This is the
guide §10 decision (per-interface binding, NOT FRR `network <prefix> area`
matching); the cross-reference check ("every interface binds a declared area")
runs in `OnConfigVerify` because YANG cannot express it natively without a
leafref into the area list, which this spec may add as a `leafref` if the loader
supports cross-container references; otherwise the Go cross-check is authoritative.

Authentication is modelled as named KEY CHAINS, not bare auth strings, mirroring
the IS-IS sibling: a `key-chains` list of keys, each carrying a `key-id`, an
`algorithm` enum, a `$9$`-encoded length-constrained `secret`, and optional
send/accept lifetimes for hitless rotation. Per-interface `authentication`
references a chain (with `inherit` falling back to the area key). These leaves are
declared here for completeness; this spec only parses and stores them. The
AuType-on-the-wire codes and the per-packet verify/sign semantics (incl. the RFC
7474 HMAC-SHA trailer) live in `spec-ospf-12-auth` per Shared Contracts
"Authentication config model".

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-config` | `test/ospf/ospf-config.ci` | A valid `ospf { router-id ...; areas{...} interfaces{...} }` block validates and the component starts; a bad router-id and an interface bound to an undeclared area are rejected | |

### Interop Tests (MANDATORY for protocol features)
No wire-protocol behaviour is introduced by this spec (it is component/config
wiring only). Protocol interop is covered by sibling specs:
`spec-ospf-5-interface-ism` (Hello/DR election), `spec-ospf-8-spf-rib`
(convergence), and `spec-ospf-13-cli-diag-interop` (full FRR `ospfd` scenarios:
P2P, broadcast/DR, multi-area, stub, NSSA, redistribution, auth, convergence).
Interop is therefore N/A for ospf-4.

### Future (if deferring any tests)
- Runtime behaviour tests (Hello exchange, adjacency Full, LSDB sync, SPF, FIB install) belong to siblings ospf-5/6/7/8 and are not deferred here; this spec only proves the skeleton starts, opens the socket, and enrols interfaces.

## Files to Modify
- `internal/component/plugin/all/all.go` - regenerated by `make generate` to import `internal/plugins/ospf` and `internal/plugins/ospf/yang` (generated, never hand-edited)
- `internal/component/config/validators_register.go` - register the central `ospf-router-id` and `ospf-area-id` ValidateFns (cycle-break; the component cannot be imported by `config`). The component owns the matching CompleteFns via `configyang.RegisterCompleteFn`
- `internal/test/cli/register.go` - register the new `test/ospf` functional-test suite via `registerCIRoot("ospf", "ospf", "ospf", "<description>", 0)` so `bin/ze-test ospf --all` resolves; ospf-4 establishes the suite, later specs add `.ci` cases
- `mk/test-functional.mk` - add a `ze-ospf-test` individual target (`@$(SUITE_RUN) bin/ze-test ospf --all`), add `ospf` to the `all_suites` / `run_suite` list and the `.PHONY` declarations so the suite runs under `make ze-functional-test`
- `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - document the new `ospf` config block, per-interface area binding, area auth inheritance, and validation failures
- `docs/guide/plugins.md`, `docs/plugin-overview.md`, `docs/guide/status.md` - document the new OSPF edge plugin, lifecycle/status row, and removal/self-containment surface
- `docs/functional-tests.md` - document the new `test/ospf/` suite and `ze-ospf-test` target
- `docs/architecture/core-design.md` - add OSPF to the edge-plugin protocol list, not the component/platform list

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` (top-level `ospf`, `ze:config-root`) |
| YANG validation constraints | Yes | `range`/`pattern`/`enumeration` on every numeric/enum/id leaf (cost, priority, intervals, maximum-paths, area-type, metric-type) per the leaf table |
| YANG custom validators | Yes | `ospf-router-id` and `ospf-area-id` validators with `CompleteFn`; ValidateFns registered centrally in `validators_register.go` (cycle-break), CompleteFns registered from the component |
| CLI commands/flags | No | `show ip ospf neighbor/interface/database/route/border-routers/spf` dispatched in `OnExecuteCommand` but stubbed; real grammar in spec-ospf-13-cli-diag-interop |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md`; command names declared in `sdk.Registration.Commands` as `show ip ospf <noun>` |
| Editor autocomplete | Yes | YANG enum/type driven, plus router-id/area-id `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-config.ci` |
| Pipe completeness | No | Deferred to spec-ospf-13 (no real show output yet) |
| Env var registration | No | None; OSPF config is YANG-only, nothing under `environment/` |
| Doctor check for runtime dependencies | No | `CAP_NET_RAW` / raw-socket doctor check (`doctor-ospf-raw-socket`) is owned by spec-ospf-3-ip-transport; ospf-4 defers all doctor detail to ospf-3 |
| Prometheus counters/metrics | Yes | `ConfigureMetrics` wired (registry stored); the actual `ze_ospf_*` series are registered per-owner by the runtime siblings (ospf-3/5/6/7/8/9/10/11/12 per the umbrella "Metrics (canonical)" table), NOT by ospf-13 (which only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Deferred to spec-ospf-13 (feature is not usable until adjacency/SPF land) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` (new `ospf` block) |
| 3 | CLI command added/changed? | No | Commands stubbed; documented in spec-ospf-13 |
| 4 | API/RPC added/changed? | No | Stubbed RPC; documented in spec-ospf-13 |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md` (new `ospf` edge plugin) |
| 6 | Has a user guide page? | No | `docs/guide/ospf.md` is created by spec-ospf-13 when the feature is usable |
| 7 | Wire format changed? | No | No wire format in this spec |
| 8 | Plugin SDK/protocol changed? | No | Uses the existing SDK lifecycle unchanged |
| 9 | RFC behavior implemented? | No | Config wiring only; RFC behaviour in siblings |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/` directory) |
| 11 | Affects daemon comparison? | No | Deferred to spec-ospf-13 |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (new `ospf` edge plugin) |
| 13 | Route metadata keys added/changed? | No | None in this spec |
| 14 | Prometheus counters added/changed? | No | Registry wired here; the `ze_ospf_*` series are registered per-owner by runtime siblings, not centrally and not by ospf-13 |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` (new component, new event namespace) |
| 16 | Any changed source file referenced by doc source anchors? | No | Grep `docs/` for source anchors at completion |
| 17 | Existing docs show examples for this area? | No | Grep at completion; verify the `ospf` block example against the YANG |

## Files to Create
- `internal/plugins/ospf/register.go` - `init()` registration (`registry.Registration`) + `runOSPFEngine` SDK lifecycle; registers the event namespace, the router-id/area-id CompleteFns, and (if added) the config-sanity diagnostic codes
- `internal/plugins/ospf/config.go` - typed config structs (instance + per-area + per-interface + ranges + key-chains), parse from the `ospf` JSON subtree, YANG-default application, router-id resolution/derivation, and the cross-check that every interface binds a declared area
- `internal/plugins/ospf/events.go` - `events.RegisterNamespace` + OSPF event types (neighbor up/down, SPF run, LSDB change)
- `internal/plugins/ospf/instance.go` - top-level OSPF instance: router-id, area map, AS-external store placeholder, timers, goroutine lifecycle, the packet-type receive dispatcher (keyed by the common-header Type, routing Hello to ISM / DD,LS Request to NSM / LS Update,LS Ack to LSDB/flooding; handlers register at startup, stubs here), raw-socket open + per-interface enrol via spec-ospf-3, clean shutdown (multicast leave, socket close), journal-based reconcile
- `internal/plugins/ospf/area.go` - per-area state scaffolding: area-id, area-type, the interface set, an LSDB placeholder, and the SPF-trigger handle (filled by ospf-7/8); instance holds the `map[AreaID]*area`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - config schema (top-level `ospf`, `ze:config-root`, validated leaves per the leaf table). The `yang/` directory is the home for all OSPF YANG modules: this spec creates only `internal/plugins/ospf/yang/ze-ospf-conf.yang`. The command module `internal/plugins/ospf/yang/ze-ospf-cmd.yang` (CLI command ownership; show binds `ze-show:ospf-*`, clear binds `ze-clear:ospf-*`, modelled on `internal/plugins/ldp-cmd/yang/ze-ldp-cmd.yang` and the IS-IS `ze-isis-cmd.yang`) lives in the same directory but is owned and authored by spec-ospf-13-cli-diag-interop per Shared Contracts "Command + API YANG"; this spec does NOT define the cmd schema. No per-component api yang file is created: by design the show/clear RPCs register in Go via the central `ze-show` / `ze-clear` namespaces (LDP/IS-IS style)
- `internal/plugins/ospf/yang/register.go` - generated glue (`configyang.RegisterModule`), produced by `make generate`
- `internal/plugins/ospf/yang/embed.go` - generated `//go:embed ze-ospf-conf.yang` var, produced by `make generate`
- `internal/component/config/validators_ospf_test.go` - tests for the central `ospf-router-id` / `ospf-area-id` ValidateFns (cycle-break, IS-IS precedent)
- `test/ospf/ospf-config.ci` - functional test: valid `ospf` config validates and the component starts; bad router-id and undeclared-area binding rejected

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-ospf-0-umbrella.md` |
| 2. Audit | Files to Create, Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register the OSPF edge plugin, write the failing wiring test
   - Tests: `TestOSPFPluginStart`
   - Files: `internal/plugins/ospf/register.go` (stub `runOSPFEngine` with empty `OnStarted`), `internal/plugins/ospf/yang/ze-ospf-conf.yang` (minimal: `ospf` container + `router-id` + `areas/area` + `interfaces/interface`), run `make generate` for `yang/register.go`, `yang/embed.go`, and `all.go`
   - Verify: registration present, plugin inventory includes OSPF, `runOSPFEngine` reachable; the wiring test fails because socket open / interface enrol is still a stub
2. **Phase: YANG schema + validators** - full leaf table, router-id and area-id validators
   - Tests: `TestOSPFRouterIDValidator`, `TestOSPFAreaIDValidator`, boundary tests for cost/priority/intervals/maximum-paths/range-cost
   - Files: `internal/plugins/ospf/yang/ze-ospf-conf.yang`, `internal/component/config/validators_register.go` (central ValidateFns), CompleteFn registration in `register.go`
   - Verify: native validation rejects out-of-range/enum/pattern violations; custom validators reject bad router-id/area-id and offer `CompleteFn`
3. **Phase: Config resolution** - typed structs, defaults, router-id derivation, undeclared-area cross-check
   - Tests: `TestOSPFConfigResolve`, `TestOSPFConfigDefaults`, `TestOSPFConfigValidate`, `TestOSPFInterfaceEnrolment`
   - Files: `internal/plugins/ospf/config.go`, `internal/plugins/ospf/area.go`
   - Verify: full subtree resolves with defaults; router-id resolves or derives; interface bound to an undeclared area rejected; only enabled interfaces bound to a declared area enrol
4. **Phase: Events** - OSPF event namespace
   - Tests: `TestOSPFEventNamespace`
   - Files: `internal/plugins/ospf/events.go`
   - Verify: namespace + neighbor up/down + SPF-run + LSDB-change types register without collision
5. **Phase: Lifecycle + socket + enrol + packet dispatcher** - `OnConfigVerify`/`OnConfigure`/`OnConfigApply`/`OnStarted`/`OnExecuteCommand`, raw socket open + per-interface enrol via ospf-3, the packet-type receive dispatcher (keyed by the common-header Type, with stub handlers; ospf-3 delivers `(ifindex, src, payload)` to it and holds no protocol switch), journal reconcile, clean shutdown
   - Tests: `TestOSPFComponentStart` (now passes), `TestOSPFConfigApplyReconcile`, `TestOSPFPacketDispatch`
   - Files: `internal/plugins/ospf/instance.go`, `internal/plugins/ospf/register.go`
   - Verify: engine opens the socket and enrols enabled interfaces; the dispatcher routes packet types to the right (stub) handler and drops unknown/short/wrong-version packets; reload reconciles one interface; shutdown leaves multicast groups and closes the socket
6. **Functional test + suite registration** - register the `test/ospf` suite in `internal/test/cli/register.go` and `mk/test-functional.mk`; write `test/ospf/ospf-config.ci`: valid config validates and starts; bad router-id and undeclared-area binding rejected; confirm `make ze-ospf-test` runs it
7. **Full verification** - `make ze-verify`
8. **Complete spec** - fill audit tables, write learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; the wiring test passes end-to-end |
| Feature completeness | The skeleton matches the LDP/IS-IS reference shape (registration fields, all SDK callbacks present); the raw socket opens and interfaces enrol via ospf-3 |
| Correctness | Defaults match the leaf table (maximum-paths 8, hello 10, dead 40, priority 1, retransmit 5, transmit-delay 1, area-type normal, SPF delay/hold defaults); router-id derives per RFC 2328 §C.1; every interface binds a declared area |
| Naming | YANG kebab-case; Go fields PascalCase of leaves; CLI `show ip ospf <noun>`; module `ze-ospf-conf` |
| Data flow | Config flows tree -> schema -> SDK callbacks -> engine -> transport; no second config path; no new sysrib leaf |
| CLI grammar | Command names are action-before-identifier (`show ip ospf <noun>`) |
| YANG validation | No bare `type string` on numeric/enum/id leaves; router-id/area-id have custom validator + `CompleteFn` |
| Rule: registration-dispatch | OSPF registers; core discovers via registry, no direct import outside generated `all.go` and the central ValidateFn cycle-break |
| Rule: no hand-edit of generated files | `all.go`, `yang/register.go`, `yang/embed.go` produced only by `make generate` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OSPF edge plugin directory | `ls internal/plugins/ospf/` |
| YANG module | `ls internal/plugins/ospf/yang/ze-ospf-conf.yang` |
| Generated glue | `ls internal/plugins/ospf/yang/register.go internal/plugins/ospf/yang/embed.go` |
| `all.go` wired | `grep ospf internal/component/plugin/all/all.go` |
| Central ValidateFns | `grep ospf-router-id internal/component/config/validators_register.go` |
| Functional test | `ls test/ospf/ospf-config.ci` |
| Test suite registered + runs | `grep ospf internal/test/cli/register.go mk/test-functional.mk`; `make ze-ospf-test` runs `test/ospf/ospf-config.ci` under the functional-test runner and passes |
| Component registered | `ze plugin` inventory shows `ospf` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | router-id and area-id validated before use; every numeric leaf range-bounded; the undeclared-area cross-check runs in `OnConfigVerify`; reject malformed config before mutating state |
| Resource handling | Interface goroutines, the raw socket, and multicast group memberships are released cleanly on shutdown; no leaked goroutines or lingering group joins on reload |
| Privilege | Raw-socket privilege (`CAP_NET_RAW`) handling is owned by spec-ospf-3; this spec must not silently swallow an EPERM at socket open |
| Error leakage | Config errors are descriptive but do not echo secrets (auth keychain secrets parsed here are masked per `ze:sensitive`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior (LDP/IS-IS register.go) |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| `all.go` missing the import | Run `make generate`; never hand-edit |
| Import cycle (config <-> ospf) | Move the ValidateFn to `validators_register.go`; keep CompleteFn in the component (IS-IS precedent) |
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
- This spec wires a skeleton only: the component starts, ingests config, opens the raw IP socket, and enrols interfaces (multicast join), but sends no Hellos, forms no adjacencies, builds no LSDB, and computes no routes (siblings ospf-5+ deliver runtime behaviour)
- Area-type semantics (Type 5 suppression in stub, default injection, NSSA Type 7) are parsed and stored but not enforced (spec-ospf-9 / spec-ospf-11)
- Authentication leaves are parsed and stored but not verified/signed (spec-ospf-12-auth)
- Redistribute and default-information leaves are parsed and stored but not acted on (spec-ospf-10)
- `show ip ospf` commands are declared and dispatched but return stub data (spec-ospf-13-cli-diag-interop)

## RFC Documentation

Add `// RFC 2328 Section X.Y: "<quoted requirement>"` (and RFC 9129 for the YANG
schema shape) above the enforcing config-validation/derivation code where a
router-id / area-id / interface-parameter constraint maps to a normative
requirement (e.g., the §C.1 router-id derivation rule, the §C.3 interface default
intervals).

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered - add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

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
| `ospf { ... }` reaches a running engine that opens the socket and enrols interfaces | wiring test (unit, -race) | `TestOSPFComponentStart` |
| Config accepted and validated; bad router-id and undeclared-area binding rejected | functional test | `test/ospf/ospf-config.ci` |
| Component in plugin inventory; `make generate` wires `all.go` | inventory + grep | `ze plugin` lists `ospf`; `grep ospf internal/component/plugin/all/all.go` |
| Typed config resolution with YANG defaults | unit test (-race) | `TestOSPFConfigResolve` + `TestOSPFConfigDefaults` |
| Interop / on-the-wire validation | interop | N/A for ospf-4 by design (no wire protocol); covered by ospf-5/8/13 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` not run yet for this design spec | this spec | run during implementation; record concrete findings here |

### Fixes applied
- Pending: record concrete fixes after `/ze-review` reports BLOCKER or ISSUE findings.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled - 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/`)
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
- [ ] Minimal coupling (transport behind the ospf-3 interface)

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
- [ ] Write learned summary to `plan/learned/NNN-ospf-4-component-config.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-ospf-4-component-config.md`

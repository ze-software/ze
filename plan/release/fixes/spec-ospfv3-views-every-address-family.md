# Spec: ospfv3-views-every-address-family

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped RFC 5838 multi-AF: an operator, or an interop checker, cannot see the neighbors, LSDB or routes of an OSPFv3 instance other than IPv6 unicast, so `show ospf ipv6 neighbor` answers an empty list while an IPv4-unicast adjacency is Full |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

OSPFv3 runs one engine per configured address family (RFC 5838):
`v6EngineSet` (`internal/plugins/ospf/register_multiaf.go`) holds an engine for
each of `ipv6-unicast`, `ipv6-multicast`, `ipv4-unicast` and `ipv4-multicast`
that the config names. Every `show ospf ipv6 ...` view except the AF listings
reads only one of them: the `OnExecuteCommand` switch in
`internal/plugins/ospf/register.go` calls `v6set.engineFor(afIPv6Unicast)` for
the neighbor, neighbor detail, interface, interface detail, SPF, SPF detail,
graceful-restart and segment-routing views, `v6DatabaseDetail`
(`internal/plugins/ospf/debug_wiring.go`) does the same for every database
view (detail, router detail, the three scope views, extended), and both RI
views (`show ospf database router-information`, `show ospf ipv6 database
router-information`) pass that one engine to `riDatabaseSnapshot`
(`ri_show.go`). There is no `show ospf ipv6 route` at all: `show ospf route`
reads the OSPFv2 engine, so no OSPFv3 instance's computed routes are visible.
`debug ipv6 ospf inject lsa` likewise injects only into the IPv6-unicast
engine.

So an RFC 5838 instance such as `ipv4-unicast` with Instance ID 64 has no view
of its neighbors' state, its LSDB or its routes. With only that AF configured,
`show ospf ipv6 neighbor` answers `[]` while its adjacency is Full;
`show ospf ipv6` and `show ospf ipv6 instance` count its neighbors but cannot
show them. This was found fixing `ospf-multiaf-v4-frr` (b5fdf9a87f, 8c9423c5d0,
1c7c181359): the checker could not read Ze's adjacency or LSDB for the
IPv4-unicast instance and had to fall back to the peer's kernel route
(`internal/le/interoplab/bgp/checkers.go`, the `ospf-multiaf-v4-frr` entry).
RFC 5838's debugging consequence, as `rfc/short/rfc5838.md` records it, is that
show commands must identify the address-family instance when multi-AF is
enabled.

The goal: every OSPFv3 view answers for every running address family, each row
naming its `address-family` and `instance-id`, and takes an optional typed
`address-family <af>` keyword that narrows it to one; `show ospf ipv6 route`
exists; `debug ipv6 ospf inject lsa` takes the same keyword. Each is a daemon
command, so it answers on every surface (`ai/META-DECISIONS.md`, "Every
operator command runs through the daemon on every command surface").

## Required Reading

### Architecture Docs
- [ ] `ai/doors/routing-protocols.md` - OSPF runs one engine per OSPFv2 instance and per OSPFv3 address family over shared codecs
  → Constraint: OSPFv2 and OSPFv3 share code and logic written for one was wrong in the other (Traps); the fix lives in the one OSPF plugin, never in a second engine
- [ ] `ai/doors/commands-cli.md` - the command model, `command.AnswerSurface`, the R1-R9 grammar
  → Constraint: a handler answers data, never text; the argument is a leaf the model declares, never a positional token the handler reads on its own
  → Constraint: help, usage and completion are derived from the owner's registration and schema
- [ ] `ai/rules/cli.md` - "CLI Grammar: Keywords Before Values", output and JSON keys
  → Constraint: the selector is a closed keyword (`address-family`) whose value is typed; JSON keys are kebab-case (`address-family`, `instance-id`)
  → Decision: a `show` verb only reports, so an omitted selector may answer every member; the explicit-`all` rule binds set-acting verbs only
- [ ] `docs/architecture/ospf/ospf-ext-15-multi-af.md` and `ospf-ext-14-debug-introspection.md` - multi-AF engines, the IPv6 deep-introspection views
  → Constraint: `debug` stays double-gated (authz and the runtime enablement); adding the keyword changes neither gate
- [ ] `docs/architecture/api/commands.md` - "Which surfaces offer a command", plugin-proxied builtin RPCs
  → Constraint: the builtin RPC forwards to the plugin by `ForwardToPlugin`, never by re-dispatching the command string
- [ ] `ai/doors/le-tooling.md` - `.ci` tests, the fixture package, the interop lab
  → Constraint: a `.ci` that needs raw IPv6 sockets carries `option=needs-linux` and runs in the QEMU VM

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5838.md` - Processing Consequences, "Debugging"
  → Constraint: neighbor state and the LSDB are per instance; show commands must identify the address-family instance

**Key insights:**
- Only `show ospf ipv6` (`afSummary`) and `show ospf ipv6 instance` (`instanceListing`) iterate `runningAFs()`; every other view hardcodes `afIPv6Unicast`.
- `forwardToOSPF` (`cmd_show.go`) refuses any argument; `forwardToOSPFArgs` already forwards validated tokens.
- `afFromName` (`multiaf.go`) already maps the four AF spellings the config uses.
- BIRD answers an ambiguous protocol show by refusing: `proto_get_named` (`nest/proto.c`) raises "There are multiple %s protocols running" when no name is given and several run. Ze's AF rows carry their identity instead, so no refusal is needed for a read.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ospf/register.go` - `OnExecuteCommand`: the `show ospf ipv6 ...` arms call `v6set.engineFor(afIPv6Unicast)`; `commandDecls` names every command
- [ ] `internal/plugins/ospf/debug_wiring.go` - `v6DatabaseDetail`, `v6DatabaseExtended`: the IPv6-unicast engine only, an AF-tagged empty record when idle
- [ ] `internal/plugins/ospf/register_multiaf.go` - `v6EngineSet`, `engineFor`, `runningAFs`, `afSummary`
- [ ] `internal/plugins/ospf/instance_view.go` - `instanceListing`: the per-AF iteration shape to reuse
- [ ] `internal/plugins/ospf/instance_snapshots.go` - `neighborSnapshot`, `routeSnapshot`: per-engine, no AF field
- [ ] `internal/plugins/ospf/ri_show.go` - `riDatabaseSnapshot(v4, v6)`: one v6 engine
- [ ] `internal/plugins/ospf/cmd_show.go` - builtin RPCs, `forwardToOSPF` refuses arguments, `forwardToOSPFArgs` forwards them
- [ ] `internal/plugins/ospf/multiaf.go` - `afFromName`, the AF name constants
- [ ] `internal/plugins/ospf/yang/ze-ospf-cmd.yang` - the `show ospf ipv6` command tree; no leaf, no `route` container
- [ ] `internal/le/interoplab/bgp/checkers.go` - the `ospf-multiaf-v4-frr` checker reads `show ospf ipv6 instance` and the peer's kernel route
- [ ] `test/ospf/ospf-multiaf-show.ci` - the AF listing `.ci` over two configured AFs

**Behavior to preserve:**
- `show ospf ipv6` and `show ospf ipv6 instance` payloads.
- With only IPv6 unicast configured, each list view (`interface`, `neighbor`, `neighbor detail`, `interface detail`, `spf`) answers the rows it gives today, each led by `address-family` and `instance-id`; a row key of either name is replaced by the engine's own value, so `interface detail`'s `instance-id`, 0 on every v6 engine before (`instanceIDForV4`), becomes the engine's Instance ID. The `database` views and `spf detail` keep their per-AF records, which already carried both keys.
- The design answers every running AF (Key Design Decisions), so a view that answered one object cannot keep that shape: `graceful-restart` and `segment-routing` answer a list of one record per AF, tagged; with no OSPFv3 AF the GR list is empty and the SR list holds one untagged record, the IPv6 SR state of the OSPFv2 Router ID (`v3SegmentRoutingView`). The two RI views keep their record and put the two keys after each OSPFv3 entry's `af` (`riRouterEntry`).
- The `debug` double gate and the OSPFv2 views.

**Behavior to change:**
- Each OSPFv3 view answers for every running AF, rows tagged `address-family` and `instance-id`.
- An optional `address-family <af>` keyword narrows a view, or the inject target, to one AF; an AF that is not running is refused with a message naming the running ones and `show ospf ipv6 instance`.
- A new `show ospf ipv6 route` (and the RI views over every AF).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-AF engine set | `internal/plugins/ospf/register_multiaf.go` `v6EngineSet`, `engineFor`, `runningAFs` | Spawns and holds one engine per configured AF; `runningAFs` orders them by RFC 5838 range | Extends: one helper over `runningAFs` (optionally narrowed) feeds every view, replacing the hardcoded `afIPv6Unicast` lookups |
| AF-aware listings | `instance_view.go` `instanceListing`, `register_multiaf.go` `afSummary` | Iterate every running AF and tag each row with `address-family` and `instance-id` | Extends: the same iteration and the same two keys for every other view |
| Per-engine snapshots | `instance_snapshots.go` (`neighborSnapshot`, `routeSnapshot`), `v3NeighborDetailSnapshot`, `v3InterfaceDetailSnapshot`, `spfSnapshot`, `spfExplainSnapshot`, `v3DatabaseDetailSnapshot`, `grSnapshot`, `srSnapshot`, `ipsecInterfaceSnapshot` | Answer one engine's state | Extends: called once per AF; unchanged inside |
| Plugin-proxied builtin RPCs | `cmd_show.go` `RegisterRPCs`, `forwardToOSPF`, `forwardToOSPFArgs` | The daemon serves each `ze-ospf:*` method and forwards to the plugin | Extends: the views that gain the keyword forward their validated tokens; `show ospf ipv6 route` is one more registration of the same shape |
| AF name mapping | `multiaf.go` `afFromName`, `afName*` | Maps the four config spellings (and bare `ipv6`) to an AF | Extends: split into `afFromCanonicalName` (the four names, which the keyword parsers `selectInstances` and `splitInjectAddressFamily` use) and `afFromName` (those plus bare `ipv6`, the config's); the YANG enumeration lists the four canonical names |
| Typed selector leaf on a show command | `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` `leaf family`, `ze-cli-announce-cmd.yang` `leaf family` | A typed optional filter on a `show` command | Extends the pattern: an `address-family` leaf, enumerated, on each OSPFv3 view |
| Surface parity | `internal/component/command/surface.go` `command.AnswerSurface`, `TestEveryCommandOnEverySurface` | One predicate decides which surfaces offer a command | Applies unchanged: a daemon RPC is offered on every surface |
| Interop lab checker | `internal/le/interoplab/bgp/checkers.go`, `ospf-multiaf-v4-frr` | Reads Ze's AF listing and the peer kernel route | Extends: also waits on Ze's own view of the IPv4-unicast adjacency |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator or program sends `show ospf ipv6 <view> [address-family <af>]` on any surface (SSH, `ze cli`, REST, gRPC, MCP, web); `debug ipv6 ospf inject lsa ... [address-family <af>]` likewise.

### Transformation Path
1. The command registry validates the path and the enumerated `address-family` value against `ze-ospf-cmd.yang`.
2. The `ze-ospf:*` builtin RPC (`cmd_show.go`) forwards the plugin command with its validated tokens through `ForwardToPlugin`.
3. The OSPF plugin's `OnExecuteCommand` parses the optional keyword with `afFromCanonicalName` (the four names; `afFromName` also takes the config's bare `ipv6`), refuses an AF that is not running, and gathers the view from each selected engine, tagging every row.
4. The answer returns as structured data through `ApplyPipes`, so `| json`, `| yaml` and `| table` render the same rows.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Surface ↔ daemon dispatcher | Command registry, YANG path and leaf | No |
| Daemon ↔ OSPF plugin | `ForwardToPlugin`, plugin command plus tokens (works internal and forked) | No |

### Integration Points
- `commandDecls` (`register.go`) - gains `show ospf ipv6 route`; it has two readers and stays one function.
- `ze-ospf-cmd.yang` - the leaf on each view, the `route` container.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Views go through the registry, the builtin RPC and the plugin, as today |
| No unintended coupling (components stay isolated) | Yes | Everything stays in `internal/plugins/ospf` and its YANG |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `runningAFs`, the existing snapshots and `afFromName` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Read-only operator views |
| Registration over hardcoding, outbound: new commands register, the core discovers them | Yes | `show ospf ipv6 route` is a `RegisterRPCs` entry and a `commandDecls` entry in the plugin |
| Registration over hardcoding, inbound: no core list learns the feature's name | Yes | Searched the command catalog, surface predicate and grammar feeders: they derive from the YANG and the registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every per-engine snapshot is safe to call on a non-default AF engine | All AF engines are the same `engine` type over the v6 codec (`newEngineWithCodecAF`) | A view panics or misreads on an IPv4 AF | A unit test per view over an `ipv4-unicast` engine, as `instance_view_test.go` builds one | unvalidated |
| A-2 | A YANG grouping (or a repeated leaf) carrying `address-family` validates on every view container and keeps `./le cli grammar` green | The rib and announce command modules carry typed `family` leaves | The grammar feeders refuse the shape | `./le cli grammar` and `./le doc yang-contract command-contract` after the YANG edit | unvalidated |
| A-3 | No consumer depends on `show ospf ipv6 neighbor` and its siblings answering only the IPv6-unicast AF | Grep of `test/`, `internal/test`, `internal/le/interoplab` finds the users named in Files to Modify | A checker reads the wrong AF's rows | Each named user re-run, or converted to pass `address-family ipv6-unicast` | unvalidated |
| A-4 | `riDatabaseSnapshot` can take several v6 engines and tag each observation with its AF | `riObservation` already carries `af` (`v2`/`v3`) | The RI view needs a reshaped record | Unit test with two v6 engines | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Rows of two AFs become indistinguishable to a consumer that ignores the new keys | A checker matches a row from the wrong AF | Every row carries `address-family`; the interop and `.ci` users pass `address-family` where they mean one |
| R-2 | The selector and `detail` subcontainers collide in the grammar | `./le cli grammar` refusal or completion offering the wrong token | Put the leaf on the leaf-most container of each view, `detail` included, and test both orders the grammar accepts |
| R-3 | An idle AF (configured, no interfaces up) answers differently from an unconfigured one | Operator confusion | Running AFs come from `runningAFs`; the refusal names exactly those and `show ospf ipv6 instance` |
| R-4 | The debug inject keyword weakens the double gate | A `debug` command reachable without the enablement | The gate is checked before the AF is resolved; a test with the enablement off refuses whatever AF is named |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Operator views and checkers read wrong or empty rows; no routing state changes |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Other OSPF specs adding `show ospf ipv6 <noun>` views (`plan/post-release/features/spec-ospf-ext-0-umbrella.md`), the drain spec reading `show ospf ipv6 database router detail` (`plan/release/fixes/spec-daemon-drain-before-stop.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show ospf ipv6 database address-family ipv4-unicast` from an observer plugin | → | `OnExecuteCommand` AF selection over `v6EngineSet` | `test/ospf/ospf-multiaf-views.ci` |
| `show ospf ipv6 neighbor address-family ipv4-unicast` against a second Ze | → | per-AF `neighborSnapshot` | interop `ospf-multiaf-v4-frr` checker |
| `show ospf ipv6 route address-family ipv4-unicast` on a second Ze that learned an AS-External route | → | per-AF `routeSnapshot` | interop `ospf-multiaf-v4-frr` checker, the route step on `peerZePeer` (`10.99.0.0/24` tagged `ipv4-unicast`, 64); `ospf-multiaf-views.ci` proves only the tag and the keyword, one router computing no route |
| `ze cli -c` with `show ospf ipv6 database address-family ipv4-unicast`, rendered as JSON and as a table | → | SSH session, builtin proxy, plugin, pipe renderer | `test/ospf/ospf-multiaf-views-cli.ci` (runner `ze cli -c` steps, after the `ospf/ospf-multiaf-views-cli` readiness barrier), promoted from `test/draft/ospf/` after its draft run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ipv6-unicast` and `ipv4-unicast` configured; `show ospf ipv6 database` | Records for both AFs, each with its `address-family` and `instance-id`; the IPv4-unicast record holds that instance's self-originated LSAs |
| AC-2 | Same config; `show ospf ipv6 database address-family ipv4-unicast` | Only the IPv4-unicast record |
| AC-3 | `show ospf ipv6 neighbor`, `neighbor detail`, `interface`, `interface detail`, `spf`, `spf detail`, `graceful-restart`, `segment-routing`, every `database` subview, and both RI views, each with and without `address-family` | Rows from every running AF (or the one named), each tagged. With only `ipv6-unicast` configured: a list view's rows equal today's rows led by `address-family` and `instance-id`, a row key of either name carrying the engine's own value (`interface detail`'s `instance-id` was 0); the database views and `spf detail` answer today's per-AF records; `graceful-restart` and `segment-routing` answer a list of one tagged record per AF where they answered one object, because one object cannot answer several AFs; each RI entry of an OSPFv3 instance carries the two keys after `af` |
| AC-4 | `show ospf ipv6 route` | The computed routes of every running OSPFv3 AF, each tagged; `address-family` narrows |
| AC-5 | `address-family ipv4-multicast` when that AF is not running | Refused with a message naming the running AFs and `show ospf ipv6 instance` |
| AC-6 | `address-family bogus` | Refused by the registry before the plugin, naming the four valid values |
| AC-7 | `ospf-multiaf-v4-frr` interop run | The checker sees Ze's IPv4-unicast neighbor Full through `show ospf ipv6 neighbor address-family ipv4-unicast`, beside the existing kernel-route check; with the AF selection reverted the step goes red |
| AC-8 | `debug ipv6 ospf inject lsa ... address-family ipv4-unicast` with the enablement on | The LSA lands in the IPv4-unicast engine's LSDB; with the enablement off it is refused whatever AF is named |
| AC-9 | The same views over REST, MCP and `ze cli` | The same rows (`TestEveryCommandOnEverySurface` green; `ospf-multiaf-views-cli.ci` runs `ze cli -c` as runner steps, as the daemon's user) |
| AC-10 | `| table` and `| json` on a multi-AF answer | The same rows; the table leads with the `address-family` and `instance-id` columns, which each per-AF view declares (`commandDecls`, `v3ViewColumns`; `TestOSPFv3ViewDeclsMatchV3Views`) |
| AC-11 | `docs/guide/ospf.md`, multi-AF section | States the IPv4-unicast next hop as the neighbor's Link-LSA IPv4 address (RFC 5838 §2.5), as `v6NextHop` computes it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3ViewsAnswerEveryAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-1, AC-3, A-1: two engines, every view tags rows from both | |
| `TestOSPFv3ViewsNarrowToOneAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-2, AC-5: narrowing and the refusal text | |
| `TestOSPFv3ViewsKeepTheRowsOfOneAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-3 single-AF half: today's row byte for byte behind the two keys | |
| `TestOSPFv3RouteViewPerAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-4 | |
| `TestRIViewOverEveryV6Engine` | `internal/plugins/ospf/ri_show_test.go` | A-4 and the RI half of AC-3 | |
| `TestDebugInjectLSATargetsTheNamedAddressFamily` | `internal/plugins/ospf/inject_v3_test.go` | AC-8 and R-4 | |
| `TestDebugInjectLSARefusesAnAddressFamilyItCannotUse` | `internal/plugins/ospf/inject_v3_test.go` | AC-5 for inject: unknown word, AF not running, repeated keyword, no value; nothing injected | |
| `TestOSPFCommandSchemaCarriesAddressFamily` | `internal/plugins/ospf/yang/cmd_schema_test.go` | AC-6: the leaf on every view, enumerated | |
| `TestOSPFv3ViewTableLeadsWithTheInstance` | `internal/plugins/ospf/v3_view_shape_test.go` | AC-10: under the plugin's own declarations, `show ospf ipv6 route | table` over two AFs leads with `address-family`, `instance-id`, then `area`, and its rows match `| json` row for row | |
| `TestOSPFv3ViewDeclsMatchV3Views` | `internal/plugins/ospf/v3_view_shape_test.go` | AC-10: `commandDecls` declares every `v3Views` key exactly once as a table led by `address-family`, `instance-id`, both RI views as `af`, `address-family`, `instance-id`, no other command with both identity columns, and no shared columns slice | |
| `TestRIViewTableLeadsWithTheVersion` | `internal/plugins/ospf/v3_view_shape_test.go` | AC-10 for RI: under the plugin's own declarations, `show ospf database router-information | table` over an OSPFv2 and two OSPFv3 engines heads its entries `af`, `address-family`, `instance-id` | |
| `TestRIGroupOrderBreaksAnInstanceIDTieByAddressFamily` | `internal/plugins/ospf/ri_show_test.go` | The address-family step of `riGroupKeyLess`: two OSPFv3 RI groups at one Instance ID render ipv4-unicast before ipv6-unicast in either input order, before scope decides | |
| `TestPluginShapesFromDeclsConvertsEachShapedDeclaration` | `internal/component/command/answer_shape_test.go` | The one `CommandDecl`-to-`PluginShape` conversion Stage 1 and the OSPF shape tests call: a shapeless declaration skipped, each shaped one carried in order, a misspelled shape refused by name | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `address-family` | enumeration of four names | `ipv4-multicast` | N/A | any other token (AC-6) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-multiaf-views` | `test/ospf/ospf-multiaf-views.ci` | Two AFs configured; an observer reads the database, route and neighbor views with and without `address-family` and logs the tagged rows (`needs-linux`) | |
| `ospf-multiaf-show` | `test/ospf/ospf-multiaf-show.ci` | Unchanged listing still passes | |
| `ospf-multiaf-views-cli` | `test/ospf/ospf-multiaf-views-cli.ci`, promoted from `test/draft/ospf/` | Two AFs configured; `ze cli -c` over SSH reads the database view narrowed and whole as JSON, and as a table (`needs-linux`) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-multiaf-v4-frr` | `test/interop/scenarios/` | a second Ze (FRR ospf6d has no IPv4-unicast AF) | Ze's own view shows the IPv4-unicast adjacency Full (AC-7) | |

## Files to Modify
- `internal/plugins/ospf/register.go` - the view arms iterate the selected AFs; `show ospf ipv6 route`; the inject target; `commandDecls` declares each per-AF view's leading columns (`v3ViewColumns`) and both RI views' (`riViewColumns`)
- `internal/plugins/ospf/inject_v3.go` - `splitInjectAddressFamily` refuses a repeated keyword with the operator's own command naming the family once (`injectCommandNamingOnce`), and a keyword with no value with `show ospf ipv6 instance`
- `internal/component/command/answer_shape.go` - `PluginShapesFromDecls`, the one conversion from a `CommandDecl` to a `PluginShape`
- `internal/component/plugin/server/startup.go` - `registerPluginShapes` calls `PluginShapesFromDecls`
- `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md` - name the conversion
- `internal/plugins/ospf/register_multiaf.go` - the one helper that answers the selected running engines from an optional AF name
- `internal/plugins/ospf/debug_wiring.go` - `v6DatabaseDetail` over the selected engines
- `internal/plugins/ospf/ri_show.go` - `riDatabaseSnapshot` over several v6 engines
- `internal/plugins/ospf/instance_snapshots.go` - unchanged: the snapshots stay per engine, and the tag is added when a row is marshaled (`afTaggedRow`)
- `internal/plugins/ospf/instance_view.go` - `instanceListing` reads the identity through `instanceFor`
- `internal/plugins/ospf/cmd_show.go` - forward tokens for the views with the keyword; the `show-ospfv3-route` RPC
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` - the `address-family` leaf on each view; the `route` container
- `internal/le/interoplab/bgp/checkers.go` - the `ospf-multiaf-v4-frr` steps reading Ze's IPv4-unicast neighbor and route
- `internal/test/fixture/routing_fixture.go` - the `ospf/ospf-multiaf-views` scenario
- `internal/test/fixture/register_ospf_multiaf_views_cli.go` - the `ospf/ospf-multiaf-views-cli` barrier: waits over in-process SSH until the database view holds both tagged records and this router's ipv4-unicast LSA, so the `.ci`'s runner `ze cli -c` steps judge a built database (AC-9)
- `internal/test/fixture/testdata/ze-help-ai-*.golden` - none exists in this tree (Phase A), so nothing is regenerated
- `docs/guide/ospf.md`, `docs/guide/command-reference.md`, `docs/architecture/ospf/ospf-ext-15-multi-af.md` - the keyword, the route view, the AF tagging

## Files to Create
- `test/ospf/ospf-multiaf-views.ci` - the functional proof (AC-1, AC-2, AC-4)
- `test/ospf/ospf-multiaf-views-cli.ci` - the `ze cli -c` proof (AC-9, AC-10), drafted at `test/draft/ospf/`
- `internal/plugins/ospf/v3_view_shape_test.go` - `TestOSPFv3ViewTableLeadsWithTheInstance` (AC-10)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/ospf/yang/ze-ospf-cmd.yang`: the leaf and `route` |
| YANG validation constraints | Yes | `address-family` is an enumeration of the four AF names |
| YANG custom validators | No | The enumeration is enough; running-AF refusal is the plugin's answer |
| CLI commands/flags | Yes | `show ospf ipv6 route`; the keyword on existing views |
| CLI grammar (keyword before value) | Yes | `address-family <af>`; checked by `./le cli grammar` |
| Editor autocomplete | Yes | Automatic from the enumeration |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-multiaf-views.ci` |
| Pipe completeness | Yes | Structured rows through `ApplyPipes`; AC-10 |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | Views only |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | The multi-AF feature exists; its views are completed |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` only if it lists OSPF RPCs; otherwise N-A after the grep |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5838.md` debugging row, with a source anchor |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-ext-15-multi-af.md`, `docs/architecture/ospf/ospf-ext-14-debug-introspection.md`, `docs/architecture/testing/interop.md` and `docs/functional-tests.md` updated. Unaffected, read against the change: `docs/architecture/ospf/ospf-13-cli-diag-interop.md` (describes the OSPFv2 `show ospf` forwarding, names no OSPFv3 view), `docs/architecture/ospf/ospf-4-component-config.md` (config and the engine lifecycle, no command declaration), `docs/architecture/ospf/ospf-ext-3-router-information.md` (the RI LSA wire format and reception, not the show view) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` if it lists OSPF commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-ospfv3-views-every-address-family.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `show ospf ipv6` examples in `docs/guide/ospf.md` |

## Implementation Steps

1. **Phase: Wiring** -- the YANG leaf and `route` container, the RPC registration, the `.ci` skeleton reading `show ospf ipv6 route address-family ipv4-unicast`.
   - Tests: `ospf-multiaf-views` red (no rows for the IPv4 AF)
   - Files: `ze-ospf-cmd.yang`, `cmd_show.go`, `register.go` (`commandDecls`), `routing_fixture.go`
2. **Phase: AF selection** -- the one selection helper; every view arm and `v6DatabaseDetail` use it; rows tagged.
   - Tests: the unit tests above red, then green; the `.ci` green
3. **Phase: RI and inject** -- `riDatabaseSnapshot` over the selected engines; the inject target.
4. **Phase: Users** -- convert every user the grep finds (A-3), the interop checker step (AC-7), the help goldens through the judge, and the docs.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | No `engineFor(afIPv6Unicast)` remains in a view arm; each view has a test over two AFs |
| Correctness | Each row's `instance-id` is the engine's own, from `currentInstanceID` |
| Naming | `address-family` and `instance-id` keys, the four AF names as the config spells them |
| Data flow | The AF is chosen in the plugin; the daemon only validates and forwards |
| Rule: cli.md | Structured data, keyword before value, refusals name the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Views answer every AF | `./le job run label ospf command go test -tags ze_ospf ./internal/plugins/ospf/ -run 'OSPFv3|RIView|DebugInject|CommandSchema'` |
| Functional proof | `./le test ospf ospf-multiaf-views` in the QEMU guest (`needs-linux`) |
| Interop proof | `./le test integration interop` with `INTEROP_SCENARIO=ospf-multiaf-v4-frr` |
| Grammar | `./le cli grammar`, `./le doc yang-contract command-contract` |
| Change set judged | GitHub CI's `verify` workflow on the pushed commit: no agent runs any `./le verify` command (`ai/META-DECISIONS.md`, "Ze is pre-release"); the scoped runs in Pre-Commit Verification are the closure's evidence |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The AF value is an enumeration checked by the registry; the plugin re-checks with `afFromCanonicalName` and refuses an unknown or idle AF; the inject modifier, which the registry does not validate, is checked by `splitInjectAddressFamily` with the same mapper |
| Authorization | The inject keyword sits behind the existing `debug` authz and enablement |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Phase B: one registry, `v3Views` (`debug_wiring.go`), maps each per-AF OSPFv3 view to its producer; `OnExecuteCommand`'s default arm serves it, so the views are declared once. Selection is `v6EngineSet.selectInstances`, tagging `afTaggedRow.MarshalJSON` (`register_multiaf.go`).
- Phase B: `graceful-restart` and `segment-routing` (a single object each before) now answer a list, one record per AF; with no OSPFv3 AF the GR view answers `[]` and the SR view keeps its OSPFv2-Router-ID IPv6 record (one untagged list entry). Naming an AF when none runs is refused.
- Phase B: a row key that repeats `address-family` or `instance-id` is dropped for the engine's own value: `show ospf ipv6 interface detail` carried `instance-id` 0 on every v6 engine (`instanceIDForV4`).
- Phase C: the RI views keep their record shape; each OSPFv3 entry carries `address-family` and `instance-id` after `af` (`riOSPFv3Instance`, embedded pointer, nil for OSPFv2), and the group key holds the OSPFv3 identity so the same router's RI LSA in two AF instances stays two entries. They are not in `v3Views`: `TestOSPFv3ViewsAnswerEveryAddressFamily` requires every `v3Views` row to lead with the identity, which an RI record does not; their arms call `routerInformationView` / `v3RouterInformationView` (`ri_show.go`).
- Phase C: `debug ipv6 ospf inject lsa` checks the enablement first, then takes the `address-family` pair out (`debugInjectIntoAF`, `splitInjectAddressFamily`, `inject_v3.go`); `errNoV6Engine` is gone, an idle default AF is refused by `afNotRunningError`. `errDebugInjectDisabled` became an `opererr` (fix: `debug ospf inject enable`), and the two AF refusals Phase B wrote no longer say "view", since inject shares them.
- Phase D: the keyword takes the four canonical names only. `afFromName` also maps the config's bare `ipv6`, which the views' YANG enumeration refuses but the inject modifier (not validated by the registry) let through; `afFromCanonicalName` (`multiaf.go`) is the keyword's mapper for `selectInstances` and `splitInjectAddressFamily`, and `afFromName` stays the config parser's.
- Phase D: R-1 is met by narrowing every interop step that means one AF (`check_extras.go`, `check_virtual_link.go`: `address-family ipv6-unicast`). `server.TestUsageRendersTheDeclaredValues` changed its expected inject usage line in place to carry Phase A's `[address-family <...>]` group, a judge unit.
- Review fix: `| table` sorts keys by name unless a command declares columns (`pipe_table.go` `declaredKeys`), and `| json` sorts every key, so the order `afTaggedRow.MarshalJSON` writes reaches no rendering. Each `v3Views` command is now declared in `commandDecls` with shape `tab` and columns `address-family`, `instance-id` (`v3ViewColumns`), and both RI views declare `af`, `address-family`, `instance-id` (`riViewColumns`); without its own declaration the IPv6 RI view would inherit `show ospf ipv6 database`'s. How those entries are written is the next point.
- Review fix: `./le cli dispatch check` reads every `CommandDecl` `Name:` from the source (`pluginCommandDecls`, `internal/le/cli/dispatch/resolver.go`) and refused the loop in `v3ViewDecls` and the parameter of `riViewDecl`. Both helpers are gone: `commandDecls` lists each `v3Views` key and both RI views as literal entries whose columns come from `v3ViewColumns()` / `riViewColumns()`, each a fresh slice. `v3Views` stays the source, and `TestOSPFv3ViewDeclsMatchV3Views` (`v3_view_shape_test.go`) compares the copy against it in both directions.
- Review fix (round 2): the OSPF shape tests registered the plugin's declarations through a copy of Stage 1's `CommandDecl`-to-`PluginShape` loop. That loop is now one exported function, `command.PluginShapesFromDecls` (`internal/component/command/answer_shape.go`), which `registerPluginShapes` (`internal/component/plugin/server/startup.go`) and `declareOSPFShapes` both call (`ai/rules/testing.md`, one function per rule). `internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go` still carries its own copy: BGP plugins were outside this round by the owner's constraint.
- Review fix (round 2): the inject refusals for a repeated `address-family` and for one with no value named a fix line of placeholders. A repeated keyword now names the operator's own command with the keyword once (`injectCommandNamingOnce`, `inject_v3.go`), and a missing value names `show ospf ipv6 instance`, which lists the running families (`ai/rules/user-facing-errors.md`).
- Review fix: the route Wiring Test row moved to the interop `ospf-multiaf-v4-frr` route step: `routeSnapshot` reads `spf.Computer`'s `c.last`, the set `Run` handed the installer, so the AS-External `10.99.0.0/24` the peer's kernel holds is a row of the peer's IPv4-unicast view.
- Phase B: `docs/guide/ospf.md` said the IPv4-AF next hop came from the IPv6 link-local adjacency; the code (`v6NextHop`, `TestRFC5838IPv4NextHopFromNeighborLinkLSA`) takes the Link-LSA's IPv4 address (RFC 5838 §2.5). Corrected (AC-11).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `view-reads-one-instance-of-several.md` | 2026-11-02 | OSPFv3 views read only the IPv6-unicast engine |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An omitted `address-family` answers every running AF, each row tagged | Refuse when several AFs run and none is named (BIRD's `proto_get_named`) | `show` only reports; RFC 5838 asks that show output identify the instance, which tagged rows do, and the single-AF answer stays what it is today |
| One typed `address-family` leaf, enumerated | A positional AF token, or one command tree per AF | `ai/rules/cli.md`: keyword before value, a closed set; one tree keeps help and completion derived |
| Flat rows with two identity keys | One nested record per AF | Rows keep `| table` and the row operators working; the database views already answer per-AF records and keep that shape |
| `debug ipv6 ospf inject lsa` takes the same keyword | Leave inject on IPv6 unicast | The same hardcoded lookup; an operator testing an IPv4 instance has no other way to inject |

## Known Limitations
- OSPFv2 multi-instance (RFC 6549) views show the same defect: every OSPFv2 view but `show ospf instance` reads the base engine. That work is its own spec, `plan/release/fixes/spec-ospfv3-views-every-address-family-ospfv2-instances.md`; this spec changes only OSPFv3 address families.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the OSPF package tests, `./le test ospf` for the changed `.ci` in QEMU, the interop scenario; the whole-tree check is GitHub CI's on the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ospfv3-views-every-address-family.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- One selection over the RFC 5838 engine set: `v6EngineSet.selectInstances` and `instanceFor` (`internal/plugins/ospf/register_multiaf.go`) answer every running address family in range order, or the one `address-family <af>` names, refusing an idle, unknown or stray argument with an `opererr` (`unexpectedViewArgsError`, `unknownAFError`, `afNotRunningError`).
- One registry of per-AF views, `v3Views` (`internal/plugins/ospf/debug_wiring.go`), served by the default arm of `OnExecuteCommand` (`register.go`); rows are led by `address-family` and `instance-id` through `afTaggedRow.MarshalJSON`. `show ospf ipv6 route` is new (YANG `container route`, `ze-ospf:show-ospfv3-route`, `cmdShowIPv6Route`).
- Both RI views read every selected engine (`routerInformationView`, `v3RouterInformationView`, `riOSPFv3Instance`, `riGroupKeyOf`, `riGroupKeyLess` in `ri_show.go`).
- `debug ipv6 ospf inject lsa ... address-family <af>`: `debugInjectIntoAF`, `splitInjectAddressFamily`, `injectCommandNamingOnce` (`inject_v3.go`), enablement checked first; `errDebugInjectDisabled` is an `opererr` (`inject.go`).
- The daemon forwards the keyword in one shape (`afViewForwarder`, `addressFamilyArgs`, `cmd_show.go`); the YANG grouping `ospfv3-address-family` puts the enumerated leaf on 18 view containers (`ze-ospf-cmd.yang`).
- `commandDecls` declares each per-AF view as a table led by `address-family`, `instance-id` (`v3ViewColumns`) and both RI views by `af`, `address-family`, `instance-id` (`riViewColumns`); `command.PluginShapesFromDecls` is the one declaration-to-shape conversion (`answer_shape.go`, called by `registerPluginShapes` in `plugin/server/startup.go`).
- Users converted: interop checkers narrowed to `ipv6-unicast` where they mean one family (`check_extras.go`, `check_virtual_link.go`), the `ospf-multiaf-v4-frr` neighbor and route steps added (`checkers.go`), two `.ci` tests and their fixtures, the netns selection, the wire-method snapshot.

### Bugs Found/Fixed
- The IPv4-unicast next hop was documented as coming from the IPv6 link-local adjacency; `v6NextHop` takes the Link-LSA IPv4 address (RFC 5838 Section 2.5). `docs/guide/ospf.md` corrected (AC-11), pinned by the existing `TestRFC5838IPv4NextHopFromNeighborLinkLSA`.
- The inject modifier admitted the config's bare `ipv6`, which the views' enumeration refuses: `afFromCanonicalName` now serves both keyword parsers; `TestDebugInjectLSARefusesAnAddressFamilyItCannotUse` and `TestOSPFv3ViewsNarrowToOneAddressFamily` carry the case.
- `instanceListing` read a zero Instance ID when an engine had no dispatcher; every engine gets one from `newEngineWithCodecAF` (`instance.go`), and the guard was replaced by `instanceFor`.

### Documentation Updates
- `docs/guide/ospf.md`, `docs/guide/command-reference.md`, `docs/architecture/ospf/ospf-ext-15-multi-af.md`, `docs/architecture/ospf/ospf-ext-14-debug-introspection.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/testing/interop.md`, `docs/functional-tests.md`, `rfc/short/rfc5838.md` (Debugging row), each with `<!-- source: -->` anchors to the producers above.
- `./le doc check all` (main thread, this worktree): green except three pre-existing IRR source anchors in `resolve.md` and `irr-filtering.md`. `./le spec citation anchors spec <this spec>`: green. Closure re-ran `./le doc check all` after its own edits: the same three IRR anchors and nothing else. `./le rfc update` at closure left the tree unchanged: the RFC 5838 edit is a Processing Consequences row, and no requirement row or `RFC requirement:` tag changed.
- Closure: news fragment `website/changes/fragments/ospfv3-views-every-address-family.md`; the OSPFv3 row of `plan/journal/view-reads-one-instance-of-several.md` marked `fixed:`; the citation in `spec-ospfv3-views-every-address-family-ospfv2-instances.md` restated as the bare stem; `spec-bgp-one-off-defects.md` amended with AC-63.

### Deviations from Plan
- `graceful-restart` and `segment-routing` answer a list of one record per AF where they answered one object (Design Insights, Phase B); AC-3 was rewritten to state it.
- The route Wiring Test row proves per-AF content through the interop route step, because one router in `ospf-multiaf-views.ci` computes no route.
- `internal/test/fixture/testdata/ze-help-ai-*.golden` named in Files to Modify does not exist in this tree; nothing was regenerated.
- The BGP test `adj_rib_in/rib_shape_test.go` keeps its own copy of the shape conversion by owner constraint; homed in Work Not Done.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `commandDecls` built the per-AF entries in a loop (`v3ViewDecls`, `riViewDecl`) | `./le cli dispatch check` reads every `Name:` literal from the source and refused names it could not read | round-1 review fix, `pluginCommandDecls` (`internal/le/cli/dispatch/resolver.go`) | Literal entries plus `TestOSPFv3ViewDeclsMatchV3Views` comparing them with `v3Views` |
| assumption | The order `afTaggedRow.MarshalJSON` writes would reach `| table` and `| json` | `| json` sorts every key, `| table` sorts unless a command declares columns (`declaredKeys`, `pipe_table.go`) | round-1 review | Each view declares its leading columns (`v3ViewColumns`, `riViewColumns`) |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every OSPFv3 view answers for every running AF, rows tagged | Done | `debug_wiring.go` `v3Views`; `register_multiaf.go` `selectInstances`, `afTaggedRow` | No `engineFor(afIPv6Unicast)` remains outside tests (grep) |
| Optional typed `address-family <af>` keyword narrows | Done | `ze-ospf-cmd.yang` grouping `ospfv3-address-family`; `cmd_show.go` `addressFamilyArgs` | |
| `show ospf ipv6 route` exists | Done | `ze-ospf-cmd.yang` `container route`; `cmd_show.go` `ze-ospf:show-ospfv3-route` | |
| `debug ipv6 ospf inject lsa` takes the keyword | Done | `inject_v3.go` `debugInjectIntoAF` | |
| Every surface answers | Done | daemon RPCs via `ForwardToPlugin` | `TestEveryCommandOnEverySurface` green |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestOSPFv3ViewsAnswerEveryAddressFamily`; `ospf-multiaf-views.ci` | |
| AC-2 | Done | `TestOSPFv3ViewsNarrowToOneAddressFamily`; `ospf-multiaf-views.ci`; `ospf-multiaf-views-cli.ci` step 3 | |
| AC-3 | Done | `TestOSPFv3ViewsAnswerEveryAddressFamily`, `TestOSPFv3ViewsKeepTheRowsOfOneAddressFamily`, `TestRIViewOverEveryV6Engine`, `TestOSPFCommandSchemaCarriesAddressFamily` | |
| AC-4 | Done | `TestOSPFv3RouteViewPerAddressFamily`; interop route step | |
| AC-5 | Done | `TestOSPFv3ViewsNarrowToOneAddressFamily`, `TestDebugInjectLSARefusesAnAddressFamilyItCannotUse`; `ospf-multiaf-views.ci` | |
| AC-6 | Done | `TestOSPFCommandSchemaCarriesAddressFamily`; `ospf-multiaf-views.ci` (`address-family bogus`) | |
| AC-7 | Done | interop `ospf-multiaf-v4-frr` step 2, red with `selectInstances` reverted | |
| AC-8 | Done | `TestDebugInjectLSATargetsTheNamedAddressFamily` | |
| AC-9 | Done | `TestEveryCommandOnEverySurface`; `ospf-multiaf-views-cli.ci` | |
| AC-10 | Done | `TestOSPFv3ViewTableLeadsWithTheInstance`, `TestOSPFv3ViewDeclsMatchV3Views`, `TestRIViewTableLeadsWithTheVersion` | |
| AC-11 | Done | `docs/guide/ospf.md` multi-AF section | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOSPFv3ViewsAnswerEveryAddressFamily`, `TestOSPFv3ViewsNarrowToOneAddressFamily`, `TestOSPFv3ViewsKeepTheRowsOfOneAddressFamily`, `TestOSPFv3RouteViewPerAddressFamily` | Done | `internal/plugins/ospf/instance_view_test.go` | |
| `TestRIViewOverEveryV6Engine`, `TestRIGroupOrderBreaksAnInstanceIDTieByAddressFamily` | Done | `internal/plugins/ospf/ri_show_test.go` | |
| `TestDebugInjectLSATargetsTheNamedAddressFamily`, `TestDebugInjectLSARefusesAnAddressFamilyItCannotUse` | Done | `internal/plugins/ospf/inject_v3_test.go` | |
| `TestOSPFCommandSchemaCarriesAddressFamily` | Done | `internal/plugins/ospf/yang/cmd_schema_test.go` | |
| `TestOSPFv3ViewTableLeadsWithTheInstance`, `TestOSPFv3ViewDeclsMatchV3Views`, `TestRIViewTableLeadsWithTheVersion` | Done | `internal/plugins/ospf/v3_view_shape_test.go` | |
| `TestPluginShapesFromDeclsConvertsEachShapedDeclaration` | Done | `internal/component/command/answer_shape_test.go` | |
| `ospf-multiaf-views`, `ospf-multiaf-views-cli`, `ospf-multiaf-show` | Done | `test/ospf/` | QEMU PASS |
| interop `ospf-multiaf-v4-frr` | Done | `test/interop/scenarios/ospf-multiaf-v4-frr` | PASS, discrimination recorded below |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `register.go`, `inject_v3.go`, `register_multiaf.go`, `debug_wiring.go`, `ri_show.go`, `instance_view.go`, `cmd_show.go`, `multiaf.go`, `inject.go`, `yang/ze-ospf-cmd.yang` | Done | |
| `instance_snapshots.go` | Done | Left unchanged, as planned: the tag is added at marshal time |
| `answer_shape.go`, `startup.go`, `docs/architecture/api/*` | Done | Outside the OSPF area, named in the c2a548c944 to 6eba0e53a9 bodies |
| `checkers.go`, `routing_fixture.go`, `register_ospf_multiaf_views_cli.go` | Done | |
| `ze-help-ai-*.golden` | Changed | Absent from this tree |
| `test/ospf/ospf-multiaf-views.ci`, `test/ospf/ospf-multiaf-views-cli.ci`, `v3_view_shape_test.go` | Done | |

### Audit Summary
- **Total items:** 30 (5 requirements, 11 ACs, 8 test rows, 6 file rows)
- **Done:** 29
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1, the absent `ze-help-ai-*.golden` (recorded in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An IPv4-unicast OSPFv3 instance's neighbors are visible, tagged | interop | `ospf-multiaf-v4-frr` PASS (image sha256:b4f6fc1ee49a): `show ospf ipv6 neighbor address-family ipv4-unicast` on the second Ze holds `"address-family": "ipv4-unicast"`, `"instance-id": 64`, router-id 172.30.0.2, state full. With `selectInstances` looking up `afIPv6Unicast`, image sha256:8fe10c1d0a8b, the run failed at that step ("OSPFv3 address family ipv4-unicast is not running") while step 1 stayed green; restored, PASS (sha256:fd207b4712a0), 2026-11-15 |
| Its LSDB is visible and narrowable | functional | `test/ospf/ospf-multiaf-views.ci` PASS in QEMU: two tagged records, the ipv4-unicast one holding LSAs of 10.0.0.1, one record when narrowed |
| `show ospf ipv6 route` shows an OSPFv3 instance's computed routes | interop | `ospf-multiaf-v4-frr` route step: `show ospf ipv6 route address-family ipv4-unicast` holds `10.99.0.0/24` tagged ipv4-unicast, 64 |
| Every view takes the keyword and answers every surface | functional + unit | `ospf-multiaf-views-cli.ci` PASS (`ze cli -c` over SSH, `| json` and `| table`); `TestEveryCommandOnEverySurface` green; `TestOSPFCommandSchemaCarriesAddressFamily` green |
| `debug ipv6 ospf inject lsa` reaches a named family behind the double gate | unit (negative included) | `TestDebugInjectLSATargetsTheNamedAddressFamily`: the LSA lands in the ipv4-unicast LSDB; disarmed, every family is refused |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| `declareShapes` in `internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go` still copies the Stage 1 shape conversion instead of calling `command.PluginShapesFromDecls` | BGP area, kept out by owner constraint | `plan/release/fixes/spec-bgp-one-off-defects.md` (AC-63) |
| The same per-instance views for OSPFv2 (RFC 6549) | Known Limitation, separate mechanism (`instanceManager`) | `plan/release/fixes/spec-ospfv3-views-every-address-family-ospfv2-instances.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ospfv3-views-every-address-family-5a9a4bd9-5348-4116-8dd3-8815135bbc31.md` (32 files, verdict clean) |
| `./le spec review check` | clean: "OK (31 code files, clean, hashes match)" |
| Rounds | 4: rounds 1-3 by implementation-phase reviewer agents, round 4 by this closure agent over the whole diff `git diff 7a35bc133d..HEAD` (the working tree is clean, so the pinned files are the committed content) |
| Reviewer lenses used | round 4: logic+wiring, security+edge-cases, RFC/protocol+style; pre-checks `./le test weakened check base 7a35bc133d` clean (15 test files), `./le repo check` 4 issues, all in IRR, firewall and `internal/le/test/deployment`, none in this diff |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `| table` and `| json` did not keep the identity keys first | `register.go` `commandDecls` | 8626726527 (`v3ViewColumns`, `riViewColumns`, `TestOSPFv3ViewTableLeadsWithTheInstance`) |
| 2 | ISSUE | No `ze cli` proof for AC-9 | `test/ospf/ospf-multiaf-views-cli.ci` | 5a90b47393 |
| 3 | ISSUE | Route wiring row proved only tags on one router | `checkers.go` route step | 8626726527 |
| 4 | ISSUE | Silent zero Instance ID guard; placeholder fix lines | `instance_view.go`, `cmd_show.go` | 8626726527 |
| 5 | ISSUE | A copy of the Stage 1 shape conversion in the OSPF test | `answer_shape.go` `PluginShapesFromDecls` | bd29e82654 |
| 6 | ISSUE | Inject refusals named placeholder fix lines; RI tie order untested | `inject_v3.go`, `ri_show_test.go` | bd29e82654 |
| 7 | ISSUE | A repeated `address-family` refusal named a fix line that kept the repeated value | `inject_v3.go` `injectCommandNamingOnce` | 6eba0e53a9 |

Round 4 (this closure, whole diff): 0 BLOCKER, 0 ISSUE. NOTEs, none blocking:
- N1: `ospf-multiaf-views.ci` judges the route and neighbor rows of one router, which computes no route and forms no adjacency, so those two tag checks hold over empty lists; the test says so, and the unit tests and the interop steps prove per-family content.
- N2: `unexpectedViewArgsError` (`register_multiaf.go`) echoes the arguments unclamped; only a caller that bypasses the registry reaches it, because `addressFamilyArgs` refuses spare tokens in the daemon first.
- N3: The segment-routing views declare `v3ViewColumns` while their no-OSPFv3 fallback record carries neither key; `docs/guide/ospf.md` states the exception.
- Verified at the producer: `instanceFor`'s never-nil dispatcher (`spawn` calls `newEngineWithCodecAF`, which sets `dispatch: newDispatcher(codec)` in `instance.go`); both RFC 5838 quotes byte-exact against `rfc/full/rfc5838.txt` (Sections 1.1 and 2); the guide's route table header matches `spf.RouteSnapshotEntry` sorted by name; no `RFC requirement:` tag added or changed.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ospf/ospf-multiaf-views.ci` | Yes | `wc -l`: 74 lines |
| `test/ospf/ospf-multiaf-views-cli.ci` | Yes | 138 lines |
| `internal/plugins/ospf/v3_view_shape_test.go` | Yes | 307 lines |
| `internal/test/fixture/register_ospf_multiaf_views_cli.go` | Yes | 85 lines |
| `test/ospf/ospf-multiaf-show.ci` | Yes | 63 lines |
| `test/interop/scenarios/ospf-multiaf-v4-frr` | Yes | `ls test/interop/scenarios/` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5, AC-8, AC-10 | Unit tests green | `./le test unit pkg path ./internal/plugins/ospf/` green after 6eba0e53a9 (main thread) |
| AC-6 | Enumeration on every view | `./le test unit pkg path ./internal/plugins/ospf/yang/` green; `ospf-multiaf-views.ci` PASS |
| AC-7 | Neighbor step red without the selection | interop discrimination above |
| AC-9 | Every surface | `./le test unit pkg path ./cmd/ze/hub/ run TestEveryCommandOnEverySurface` green; `ospf-multiaf-views-cli.ci` PASS |
| AC-11 | Next hop text | `docs/guide/ospf.md`: "takes its next hop from the IPv4 address in the leading 32 bits of the neighbor's Link-LSA (§2.5)" |
| Completeness | No hardcoded lookup left | `grep -n "engineFor(afIPv6Unicast)" internal/plugins/ospf/*.go` outside tests: no hit |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show ospf ipv6 database address-family ipv4-unicast` via the plugin dispatch | `test/ospf/ospf-multiaf-views.ci` (fixture `ospfMultiAFViewsScenario`) | Yes, read: the narrowed answer must be the one ipv4-unicast record |
| `show ospf ipv6 neighbor address-family ipv4-unicast` on a second Ze | interop `ospf-multiaf-v4-frr` (`checkers.go`) | Yes, PASS and red when reverted |
| `show ospf ipv6 route address-family ipv4-unicast` on a second Ze | interop `ospf-multiaf-v4-frr` | Yes, PASS with 10.99.0.0/24 |
| `ze cli -c` with the database view, `| json` and `| table` | `test/ospf/ospf-multiaf-views-cli.ci` | Yes, read: steps 3-5, with reject lines for a second record and for ipv6-unicast when narrowed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestOSPFv3ViewsAnswerEveryAddressFamily` runs every `v3Views` producer over an ipv4-unicast engine, green |
| A-2 | confirmed | `./le cli grammar` (only the pre-existing R9 on `le arch error-inventory`/`error-wrap`), `./le doc yang-contract command-contract`, `usage-contract`, `help-shape` green |
| A-3 | confirmed | Users narrowed (`check_extras.go`, `check_virtual_link.go`); `ospfv3-debug-decode-frr`, `ospfv3-vlink-frr` PASS; `ospfv3-sr-frr` step 2 (the narrowed SR view, 16000) PASS, and its step 3 red is the open 2026-11-14 row of `plan/journal/interop-check-expects-what-the-peer-cannot-do.md`, owned by `spec-rfc-verdict-fix-ospf` (MPLS label-space sysctl) |
| A-4 | confirmed | `TestRIViewOverEveryV6Engine` green |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| CLI reference: per-AF views, keyword, route view, inject keyword | `docs/guide/command-reference.md` against `ze-ospf-cmd.yang` and `v3Views` | Yes |
| User guide: route `| table` header | `spf.RouteSnapshotEntry` keys area, prefix, metric, type, origin, next-hops, sorted after the two declared columns | Yes |
| RFC 5838 Debugging row | `rfc/short/rfc5838.md` anchors `selectInstances`, `afTaggedRow`, `v3Views`, `riOSPFv3Instance` | Yes |
| Feature level (category 1) | The `ospf` feature's scope is unchanged; the views complete it | No update |
| `docs/guide/status.md` (category 15) | `grep -c "ospf ipv6" docs/guide/status.md`: 0, no OSPF command list | No update |
| `./le doc check all` | Green except three pre-existing IRR anchors | Yes |

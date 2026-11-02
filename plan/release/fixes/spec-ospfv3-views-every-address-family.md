# Spec: ospfv3-views-every-address-family

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped RFC 5838 multi-AF: an operator, or an interop checker, cannot see the neighbors, LSDB or routes of an OSPFv3 instance other than IPv6 unicast, so `show ospf ipv6 neighbor` answers an empty list while an IPv4-unicast adjacency is Full |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

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
- With only IPv6 unicast configured, every view's rows are the rows it gives today, plus the two identity fields.
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
| AF name mapping | `multiaf.go` `afFromName`, `afName*` | Maps the four config spellings (and bare `ipv6`) to an AF | Extends: the plugin parses the keyword's value through it; the YANG enumeration lists the four canonical names |
| Typed selector leaf on a show command | `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` `leaf family`, `ze-cli-announce-cmd.yang` `leaf family` | A typed optional filter on a `show` command | Extends the pattern: an `address-family` leaf, enumerated, on each OSPFv3 view |
| Surface parity | `internal/component/command/surface.go` `command.AnswerSurface`, `TestEveryCommandOnEverySurface` | One predicate decides which surfaces offer a command | Applies unchanged: a daemon RPC is offered on every surface |
| Interop lab checker | `internal/le/interoplab/bgp/checkers.go`, `ospf-multiaf-v4-frr` | Reads Ze's AF listing and the peer kernel route | Extends: also waits on Ze's own view of the IPv4-unicast adjacency |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator or program sends `show ospf ipv6 <view> [address-family <af>]` on any surface (SSH, `ze cli`, REST, gRPC, MCP, web); `debug ipv6 ospf inject lsa ... [address-family <af>]` likewise.

### Transformation Path
1. The command registry validates the path and the enumerated `address-family` value against `ze-ospf-cmd.yang`.
2. The `ze-ospf:*` builtin RPC (`cmd_show.go`) forwards the plugin command with its validated tokens through `ForwardToPlugin`.
3. The OSPF plugin's `OnExecuteCommand` parses the optional keyword with `afFromName`, refuses an AF that is not running, and gathers the view from each selected engine, tagging every row.
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
| `show ospf ipv6 route` | → | per-AF `routeSnapshot` | `test/ospf/ospf-multiaf-views.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ipv6-unicast` and `ipv4-unicast` configured; `show ospf ipv6 database` | Records for both AFs, each with its `address-family` and `instance-id`; the IPv4-unicast record holds that instance's self-originated LSAs |
| AC-2 | Same config; `show ospf ipv6 database address-family ipv4-unicast` | Only the IPv4-unicast record |
| AC-3 | `show ospf ipv6 neighbor`, `neighbor detail`, `interface`, `interface detail`, `spf`, `spf detail`, `graceful-restart`, `segment-routing`, every `database` subview, and both RI views, each with and without `address-family` | Rows from every running AF (or the one named), each tagged; with only `ipv6-unicast` configured, the rows equal today's rows plus the two keys |
| AC-4 | `show ospf ipv6 route` | The computed routes of every running OSPFv3 AF, each tagged; `address-family` narrows |
| AC-5 | `address-family ipv4-multicast` when that AF is not running | Refused with a message naming the running AFs and `show ospf ipv6 instance` |
| AC-6 | `address-family bogus` | Refused by the registry before the plugin, naming the four valid values |
| AC-7 | `ospf-multiaf-v4-frr` interop run | The checker sees Ze's IPv4-unicast neighbor Full through `show ospf ipv6 neighbor address-family ipv4-unicast`, beside the existing kernel-route check; with the AF selection reverted the step goes red |
| AC-8 | `debug ipv6 ospf inject lsa ... address-family ipv4-unicast` with the enablement on | The LSA lands in the IPv4-unicast engine's LSDB; with the enablement off it is refused whatever AF is named |
| AC-9 | The same views over REST, MCP and `ze cli` | The same rows (`TestEveryCommandOnEverySurface` green; one `.ci` drives `ze cli`) |
| AC-10 | `| table` and `| json` on a multi-AF answer | The same rows; the table shows the `address-family` column |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3ViewsAnswerEveryAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-1, AC-3, A-1: two engines, every view tags rows from both | |
| `TestOSPFv3ViewsNarrowToOneAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-2, AC-5: narrowing and the refusal text | |
| `TestOSPFv3RouteViewPerAddressFamily` | `internal/plugins/ospf/instance_view_test.go` | AC-4 | |
| `TestRIViewOverEveryV6Engine` | `internal/plugins/ospf/ri_show_test.go` | A-4 and the RI half of AC-3 | |
| `TestDebugInjectLSATargetsTheNamedAddressFamily` | `internal/plugins/ospf/inject_v3_test.go` | AC-8 and R-4 | |
| `TestOSPFCommandSchemaCarriesAddressFamily` | `internal/plugins/ospf/yang/cmd_schema_test.go` | AC-6: the leaf on every view, enumerated | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `address-family` | enumeration of four names | `ipv4-multicast` | N/A | any other token (AC-6) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-multiaf-views` | `test/ospf/ospf-multiaf-views.ci` | Two AFs configured; an observer reads the database, route and neighbor views with and without `address-family` and logs the tagged rows (`needs-linux`) | |
| `ospf-multiaf-show` | `test/ospf/ospf-multiaf-show.ci` | Unchanged listing still passes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-multiaf-v4-frr` | `test/interop/scenarios/` | a second Ze (FRR ospf6d has no IPv4-unicast AF) | Ze's own view shows the IPv4-unicast adjacency Full (AC-7) | |

## Files to Modify
- `internal/plugins/ospf/register.go` - the view arms iterate the selected AFs; `show ospf ipv6 route`; the inject target
- `internal/plugins/ospf/register_multiaf.go` - the one helper that answers the selected running engines from an optional AF name
- `internal/plugins/ospf/debug_wiring.go` - `v6DatabaseDetail` over the selected engines
- `internal/plugins/ospf/ri_show.go` - `riDatabaseSnapshot` over several v6 engines
- `internal/plugins/ospf/instance_snapshots.go` - the row tagging for the per-engine snapshots
- `internal/plugins/ospf/cmd_show.go` - forward tokens for the views with the keyword; the `show-ospfv3-route` RPC
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` - the `address-family` leaf on each view; the `route` container
- `internal/le/interoplab/bgp/checkers.go` - the `ospf-multiaf-v4-frr` step reading Ze's IPv4-unicast neighbor
- `internal/test/fixture/routing_fixture.go` - the `ospf/ospf-multiaf-views` scenario
- `internal/test/fixture/testdata/ze-help-ai-*.golden` - regenerated for the new leaf and command (a refresh, judged by `ze-test-judge`)
- `docs/guide/ospf.md`, `docs/guide/command-reference.md`, `docs/architecture/ospf/ospf-ext-15-multi-af.md` - the keyword, the route view, the AF tagging

## Files to Create
- `test/ospf/ospf-multiaf-views.ci` - the functional proof (AC-1, AC-2, AC-4)

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
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-ext-15-multi-af.md` |
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
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The AF value is an enumeration checked by the registry; the plugin re-checks with `afFromName` and refuses an unknown or idle AF |
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
- [ ] The targeted tests for what changed pass: the OSPF package tests, `./le test ospf` for the changed `.ci` in QEMU, the interop scenario, and `./le verify changed` over the closing commits
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

# Spec: home-cpe-18-interoplab-multi-link-topology

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation tooling. The access chain (child 19) and the home-CPE lab (child 11) are the only route to an end-to-end proof of the shipped PPPoE-to-L2TP relay data path, and they need several links and several Ze nodes in one scenario, which the harness cannot express today. |
| RFC gaps | - |
| Depends | - |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 18).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The interop harness runs one suite at a time, one Docker network per scenario,
one Ze container per scenario, and only the images its own runner builds. A
scenario that keeps two daemons apart declares VLAN links, but that mechanism
lives inside the BGP package (`labLink`, `scenarioLinks`) and accepts only the
three containers the BGP start wrapper knows (`ze`, `frr`, `frr-sink`).

The home-CPE lab needs, in ONE scenario: a LAN segment, an access segment that
carries PPPoE and nothing else, a core segment and an internet segment; three
Ze nodes (CPE, LAC, LNS); and peers whose images belong to other suites
(FreeRADIUS from `interop-radius-subscriber`, strongSwan from `interop-ipsec`,
accel-ppp from `interop-pppoe`, dhcpcd and the DNS servers from `interop`, the
WireGuard peer from the VPN deployment proof).

This spec makes links and image selection a property of the shared core:

1. A scenario declares named links. Each link is a VLAN over the scenario's
   network with an optional IPv4 and IPv6 prefix; a link with no prefix is a
   pure layer-2 segment (PPPoE, a LAN that DHCP numbers).
2. Any planned peer can be a link member, whatever image it runs.
3. A scenario may run several Ze nodes, each with its own name and its own
   configuration file.
4. Every suite registers its images by name in one core registry, so a scenario
   names an image another suite declared, and each suite's own runner derives
   its image list from the same registry.
5. A new suite, `topology`, hosts the multi-node scenarios; children 19 and 11
   add theirs to it.

This replaces the BGP-only link mechanism; it does not add a second one beside
it (`ai/rules/no-layering.md`). `labLink`, `scenarioLinks`, `renderLinks`,
`validateLink` and the link half of the BGP start wrapper are deleted, and the
one scenario that uses them, `isis-import-policy-frr`, moves to the core
declaration and stays green.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` "The suites, one per protocol area", "Separate links between daemons", "Prove a scenario discriminates", "The Docker host kernel check"
  → Decision: a scenario directory carries declarative inputs only; assertions are typed Go checkers joined by `Discover` against the suite's checker registry, and a scenario with no checker is an error.
  → Constraint: a link the harness cannot build as declared is refused before any container starts (VLAN out of range or reused, fewer than two members, a member the scenario does not start). The generalisation keeps every refusal.
  → Constraint: a scenario directory is named, never numbered.
- [ ] `ai/rules/no-layering.md`
  → Constraint: the core link mechanism replaces the BGP one in the same change; no scenario keeps the old path.
- [ ] `ai/rules/commands.md`
  → Constraint: the new suite runs through a native `./le test integration` verb, never a hand-written loop.
- [ ] `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` (in-progress)
  → Constraint: every Docker run that runs Ze refuses a host whose kernel lacks a feature Ze enrolls (owner D-4); the topology suite inherits that check through `Suite.Run` and adds no exemption.

**Key insights:**
- The link mechanism works today (`isis-import-policy-frr` proves it); what is BGP-specific is only who may be a member and how the member's start is wrapped.
- Docker networks per link were the alternative; the VLAN-over-one-network shape keeps the core's single pre-clean, orphan sweep and subnet selection unchanged.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/lab.go` - `ScenarioPlan` (l.40) holds one `NetworkSpec`, `Peers`, `Containers` and a `Preparer`; `Suite` (l.73) holds `Images []ImageBuild` built by the suite's own runner; `Suite.Run` probes the Docker kernel with the staged ze before any image build (`DockerKernel(s.StagedZe)`, l.257); `runScenario` (l.370) pre-cleans, creates the one network (`CreateNetwork`), runs `Prepare`, starts each peer on that network and hands the checker a `CheckContext` with that `Network`. `CheckerLab` (l.172) addresses every peer by name.
- [ ] `internal/le/interoplab/docker.go` - `NetworkSpec` (l.156: name, subnet candidates), `Network` (selected IPv4 and IPv6), `PeerConfig` (l.195: name, container, image, host number, mounts, capabilities, environment, arguments, command, ready probe, AppArmor profile), `ImageBuild` (l.109).
- [ ] `internal/le/interoplab/bgp/prepare_links.go` - `labLink` (VLAN, IPv4 /24 at most, members as indexes into `linkLocalOwners`), `scenarioLinks` map filled by register files, `renderLinks` and `validateLink` (refusals), `writeLinkScript` (adds a VLAN subinterface of `eth0` named `eth<N>` in declaration order, its address, link up).
- [ ] `internal/le/interoplab/bgp/prepare.go` - `linkLocalOwners` (l.755) lists three owners with their entrypoint and image command; `wrapStart` (l.827) replaces a member's entrypoint with a shell script that adds the eth0 link-local, the links and the prestart, then execs the original entrypoint.
- [ ] `internal/le/interoplab/bgp/register_import_policy.go` - the one `scenarioLinks` entry (`isis-import-policy-frr`, VLANs 101 and 102).
- [ ] `internal/le/interoplab/catalog.go`, `discover.go` - `RegisterCatalog` and `Discover`, shared by every suite.
- [ ] `internal/le/interoplab/l2tp/l2tp.go`, `radiussubscriber/radiussubscriber.go`, `pppoe/pppoe.go`, `ipsec/ipsec.go`, `internal/le/test/deployment/wireguardproof.go` - each builds its own `interoplab.Suite` with its own `imageBuilds`; `l2tp` and `radiussubscriber` name their single Ze peer `peerZe = "ze"`. 113 non-test call sites in `internal/le/interoplab` address the Ze peer by that literal.
- [ ] `internal/le/interoplab/pppdevice.go` - `PPPDeviceArguments`, the `/dev/ppp` grant every PPP container needs.
- [ ] `internal/le/test/integration/gates.go` - the `interop*` verbs (l.141, l.167).

**Behavior to preserve:**
- Every existing suite, scenario name, catalog entry, selector variable and native action.
- `isis-import-policy-frr` devices and addresses: FRR-A on VLAN 101 (`10.101.0.0/24`, Ze `eth1`), FRR-B on VLAN 102 (`10.102.0.0/24`, Ze `eth2`).
- Every link refusal and its message's named fix.
- The BGP eth0 link-local placement and its placeholders.

**Behavior to change:**
- Links move from the BGP package to the core, open to any peer.
- Images become registered by name in the core; suites derive their lists from it.
- A scenario may plan several Ze peers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration interop-topology [scenario]`, and every existing `interop*` verb.
- Format: a scenario directory of configuration files, plus a Go register file that declares the scenario's nodes, links and checker.

### Transformation Path
1. Register files run at init: each suite registers its images by name; the topology suite registers each scenario's node and link declaration and its checker.
2. `Discover` joins scenario directories with checkers (unchanged).
3. The suite's plan builder turns a declaration into a `ScenarioPlan` whose `Links` name planned peers.
4. `Suite.Run` probes the kernel (unchanged), prepares the images the plans name from the registry, and probes each link member's image once for a shell and `ip`.
5. `runScenario` validates the links against the planned peers, creates the one network, and composes ONE start wrapper per peer: links first, then the peer's own prestart (the BGP link-local is such a prestart), then the image's own entrypoint and command, read from the pinned image.
6. The checker receives the links in its `CheckContext`, so it reads each member's device and address from the declaration rather than from a literal.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ Docker | `docker create/start` with a wrapper entrypoint; `docker image inspect` for the original entrypoint and command | No |
| Harness ↔ container | the wrapper script creates VLAN subinterfaces of eth0 inside the container's namespace | No |
| Suite ↔ core | image and topology registration at init | No |

### Integration Points
- `ScenarioPlan` gains a `Links` field; `PeerConfig` gains a `Prestart` field; `CheckContext` gains the links.
- `Suite.Images` is derived from the image registry for the names the plans use.
- `Catalog` and `Discover` unchanged; the topology suite registers one catalog.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | links built inside `runScenario`, the one place containers start |
| No unintended coupling | Yes | suites register images; no suite imports another suite's package |
| No duplicated functionality | Yes | BGP link code deleted, one wrapper |
| Zero-copy preserved where applicable | N-A | tooling |
| Registration over hardcoding, outbound | Yes | images and topology scenarios register; the core holds no per-suite list |
| Registration over hardcoding, inbound | Yes | `linkLocalOwners`' role as the link member list is removed; members are any planned peer; `gates.go` gains one verb row as every suite has |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A VLAN subinterface of the bridge-attached eth0 carries PPPoE discovery and session frames between two members and not to other containers | 802.1Q tagging over a Linux bridge; `isis-import-policy-frr` already isolates multicast IS-IS this way | the access segment leaks PADI to every container | `two-ze-pppoe-link` scenario: a third container on eth0 captures no PADI | unvalidated |
| A-2 | The original entrypoint and command of any image can be read from the pinned image ID | `docker image inspect` reports Config.Entrypoint and Config.Cmd | the wrapper cannot exec the daemon | unit test over an inspect fixture, and the smoke scenario | unvalidated |
| A-3 | Every image a link member runs carries `sh` and `ip`, or can be given them in its Dockerfile | Alpine and Debian images in `test/` install iproute2 for other reasons | a member cannot build its link | the per-image probe; refusal names the Dockerfile | unvalidated |
| A-4 | One staged ze binary serves every Ze node of a scenario | `Suite.StagedZe` is per suite, configuration is per peer | per-node binaries needed | the smoke scenario runs two Ze nodes | unvalidated |
| A-5 | MTU on a VLAN over the lab bridge is 1500 less nothing that PPPoE (1492) cares about | VLAN subinterface inherits eth0's 1500 | PPPoE MRU negotiation differs from the field | child 19 asserts the negotiated MRU | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving the BGP scenario breaks its addresses | `isis-import-policy-frr` red | AC-7 runs it; addresses are kept identical |
| R-2 | An image's entrypoint is a shell form that the wrapper re-quotes wrongly | daemon exits at start | inspect gives the exec form; unit test covers both forms |
| R-3 | Three Ze nodes plus peers exceed the Linux host's capacity | slow starts | scenarios run one at a time; the full lab runs on the hosts that run full suites (`ai/META-DECISIONS.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | interop scenarios fail to start; nothing user-visible in Ze |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | every interop suite; `spec-docker-hosts-run-the-ze-kernel` (kernel check in `Suite.Run`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-topology two-ze-static-link` | → | topology runner, core `Links`, start wrapper | scenario `two-ze-static-link` |
| `./le test integration interop-topology two-ze-pppoe-link` | → | layer-2 link with no prefix | scenario `two-ze-pppoe-link` |
| `./le test integration interop isis-import-policy-frr` | → | BGP scenario on core links | scenario `isis-import-policy-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A scenario declares a link with an IPv4 prefix and two members | each member holds `eth<N>` (N its ordinal among the links it joins) on the link's VLAN at its host number, before its daemon starts; the two members ping each other on it |
| AC-2 | A link declares no prefix | members get the VLAN device up with no address; a PPPoE session forms across it between two Ze nodes, and a container on eth0 alone captures no PADI |
| AC-3 | A link declares an IPv6 prefix | each member gets the /64 address with its host number as the low octet, `nodad` |
| AC-4 | A link has a VLAN outside 1 to 4094 or reused, an IPv4 prefix longer than /24, an IPv6 prefix not /64, fewer than two members, a member twice, or a member the scenario does not plan | the scenario fails before any container starts, and the message names the link, the fault and the fix |
| AC-5 | A link member's image has no `sh` or no `ip` | the suite refuses before the scenario starts, naming the image and its Dockerfile |
| AC-6 | A scenario plans three Ze peers with distinct names and configuration files | each starts with its own file; the checker queries each by name |
| AC-7 | `isis-import-policy-frr` after the move | green, same devices and addresses; `labLink`, `scenarioLinks` and `renderLinks` exist nowhere |
| AC-8 | A topology scenario names an image another suite registered (FreeRADIUS, strongSwan, accel-ppp) | the image is built once by the registry's declaration and the container runs |
| AC-9 | A scenario names an unregistered image | refused at plan time, naming the registered images |
| AC-10 | Two suites register the same image name | the registration panics at init with both owners named |
| AC-11 | Each existing suite runs its full scenario list | same verdicts as before; its image list comes from the registry |
| AC-12 | A topology scenario on a host whose kernel lacks an enrolled feature | `Suite.Run` refuses, as every suite does |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkValidateRefusals` | `internal/le/interoplab/link_test.go` | AC-4, one case per refusal | |
| `TestLinkRenderDevicesAndAddresses` | `internal/le/interoplab/link_test.go` | AC-1, AC-2, AC-3 ordinals and addresses | |
| `TestStartWrapperComposesLinksPrestartEntrypoint` | `internal/le/interoplab/link_test.go` | order: links, prestart, original exec form and shell form | |
| `TestImageRegistryDuplicatePanics` | `internal/le/interoplab/imageregistry_test.go` | AC-10 | |
| `TestPlanRefusesUnknownImage` | `internal/le/interoplab/imageregistry_test.go` | AC-9 | |
| `TestBGPImportPolicyLinksUnchanged` | `internal/le/interoplab/bgp/register_import_policy_test.go` | AC-7 devices and addresses | |
| `TestTopologyCatalogDiscovers` | `internal/le/interoplab/topology/topology_test.go` | every scenario directory has a declaration and a checker | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| VLAN | 1-4094 | 1 and 4094 | 0 | 4095 |
| IPv4 prefix length | 0-24 | 24 | N/A | 25 |
| IPv6 prefix length | 64 | 64 | 63 | 65 |
| host number | 1-254 | 254 | 0 | 255 |
| members per link | 2+ | 2 | 1 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `two-ze-static-link` | `test/interop-topology/scenarios/` | two Ze nodes on an IPv4 link ping each other; a third container on eth0 cannot reach the link address | |
| `two-ze-pppoe-link` | `test/interop-topology/scenarios/` | Ze PPPoE client and Ze PPPoE server on a prefix-less link reach IPCP Opened | |

### Interop Tests (Scope: protocol)
N-A: tooling. Children 19 and 11 carry the protocol scenarios.

## Files to Modify
- `internal/le/interoplab/lab.go` - `ScenarioPlan.Links`, link validation and wrapper composition in `runScenario`, links in `CheckContext`, image list from the registry
- `internal/le/interoplab/docker.go` - `PeerConfig.Prestart`; image inspect for entrypoint and command
- `internal/le/interoplab/bgp/prepare.go` - `wrapStart` deleted; link-local and owner prestarts become `Prestart`
- `internal/le/interoplab/bgp/register_import_policy.go` - core link declaration
- `internal/le/interoplab/{bgp,ipsec,l2tp,pppoe,radius,radiussubscriber,rsvpte}/` image builders - register by name
- `internal/le/test/deployment/wireguardproof.go` - WireGuard image registered by name
- `internal/le/test/integration/gates.go` - `interop-topology` verb
- `docs/architecture/testing/interop.md` - "Separate links between daemons" rewritten for the core; new section for the topology suite; suites table row
- `docs/functional-tests.md` - the new suite

## Files to Create
- `internal/le/interoplab/link.go`, `link_test.go` - link type, validation, rendering, wrapper
- `internal/le/interoplab/imageregistry.go`, `imageregistry_test.go` - registered images
- `internal/le/interoplab/topology/` - runner, `register_catalog.go`, one register file per scenario, checkers
- `test/interop-topology/scenarios/two-ze-static-link/` - `ze-a.conf`, `ze-b.conf`
- `test/interop-topology/scenarios/two-ze-pppoe-link/` - `ze-client.conf`, `ze-server.conf`

Deleted: `internal/le/interoplab/bgp/prepare_links.go`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | tooling, no Ze config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `gates.go` verb `interop-topology`, and its `le help` entry |
| CLI grammar | Yes | verb then optional scenario selector, as the sibling verbs |
| Editor autocomplete | N-A | not the ze editor |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no ze output |
| Env var registration | Yes | the suite's selector and suffix variables, declared through `interoplab.ReadEnvironment` like the others |
| Doctor check | N-A | no Ze runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | ze unchanged |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/interop.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived with `./le spec citation anchors`; `interop.md` anchors `bgp/prepare_links.go` and `bgp/prepare.go` |
| 17 | Existing docs show examples for this area? | Yes | the `isis-import-policy-frr` paragraph |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- topology suite, catalog, verb, the two smoke scenario directories, checkers that fail because `Links` is ignored
   - Tests: `TestTopologyCatalogDiscovers`, `two-ze-static-link`
   - Files: `topology/`, `gates.go`, scenario directories
   - Verify: the verb runs the scenario and it fails for want of `eth1`
2. **Phase: Core links** -- `Link`, validation, rendering, one wrapper with `Prestart`, image inspect
   - Tests: `TestLinkValidateRefusals`, `TestLinkRenderDevicesAndAddresses`, `TestStartWrapperComposesLinksPrestartEntrypoint`
   - Files: `link.go`, `lab.go`, `docker.go`
   - Verify: both smoke scenarios green
3. **Phase: BGP migration** -- delete `prepare_links.go` and `wrapStart`; BGP uses `Prestart` and core links
   - Tests: `TestBGPImportPolicyLinksUnchanged`, scenario `isis-import-policy-frr`
   - Verify: grep finds no `labLink`; scenario green
4. **Phase: Image registry** -- every suite registers; runners derive their lists
   - Tests: `TestImageRegistryDuplicatePanics`, `TestPlanRefusesUnknownImage`; one scenario per suite on the host that runs full suites
   - Verify: AC-8, AC-11

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | link-local eth0 placement in BGP unchanged; wrapper execs the original entrypoint with the original arguments |
| Rule: no-layering | no BGP link code and no second wrapper survives |
| Rule: principles | a refused link names its fix; no silent skip of a member |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Core links | `TestLink*` pass |
| BGP code removed | grep `labLink\|scenarioLinks\|wrapStart` in `internal/le` returns nothing |
| Topology suite | `./le test integration interop-topology` green on both smoke scenarios |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | scenario declarations are Go, compiled; the wrapper quotes every interpolated value |
| Privilege | only NET_ADMIN and `/dev/ppp` as each member declares, no privileged containers |

## Owned journal rows

None: `grep -rn 'labLink\|scenarioLinks\|one network per\|two Ze' plan/journal/` finds no open row.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| VLAN links over the scenario's one network, in the core | one Docker network per link, connected before start | one network keeps the single pre-clean, orphan sweep and subnet choice; Docker's per-endpoint interface naming is not declared in order on every daemon version the labs run; the VLAN mechanism is already proven |
| Read the member's entrypoint and command from the pinned image | keep a per-owner table like `linkLocalOwners` | the table is a central enumeration that refuses every image it does not name (`ai/rules/principles.md`) |
| Topology declared in Go register files | a text topology file in the scenario directory | a second schema and parser; the BGP links are already Go declarations; the compiler checks the names |
| A new `topology` suite | add multi-node scenarios to an existing suite | no existing suite owns a chain that crosses PPPoE, L2TP, RADIUS, IPsec and WireGuard |
| Image registry derived lists | each runner keeps its own list and the topology suite a copy | a second copy of the same fact drifts |

## Known Limitations

- Existing suites keep addressing their single Ze peer as `ze`; only shared helpers the topology checkers call take a peer name. Renaming the 113 call sites is not needed by any scenario.
- Links are layer-2 segments with optional static addresses; routing between segments is a node's job (the LNS routes the internet segment), not the harness's.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: tooling)
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass; no agent runs the full verification
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

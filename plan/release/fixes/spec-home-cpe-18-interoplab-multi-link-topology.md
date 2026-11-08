# Spec: home-cpe-18-interoplab-multi-link-topology

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Validation tooling. The access chain (child 19) and the home-CPE lab (child 11) are the only route to an end-to-end proof of the shipped PPPoE-to-L2TP relay data path, and they need several links and several Ze nodes in one scenario, which the harness cannot express today. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 18).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The interop harness runs one suite at a time, one Docker network per scenario,
and only the images its own runner builds. Several Ze nodes per scenario exist
already, each inside one suite: the BGP suite starts a second Ze, `ze-peer`,
from `ze-peer.conf` (`zeDaemonPeer`), and the RSVP-TE suite fills up to four
roles with Ze nodes from `<role>.conf`, each with VLANs from a typed
`<role>-net.txt` (`zeNodeScript`). A BGP scenario that keeps two daemons apart
declares VLAN links, but that mechanism lives inside the BGP package
(`labLink`, `scenarioLinks`) and accepts only the three containers the BGP
start wrapper knows (`ze`, `frr`, `frr-sink`). Existing Mechanisms says what
of each is reused and what is replaced.

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
- [ ] `internal/le/interoplab/lab.go` - `ScenarioPlan` holds one `NetworkSpec`, `Peers`, `Containers` and a `Preparer`; `Suite` holds `Images []ImageBuild` built by the suite's own runner; `Suite.Run` probes the Docker kernel with the staged ze before any image build (`DockerKernel(s.StagedZe)`); `Suite.runScenario` pre-cleans, creates the one network (`CreateNetwork`), runs `Prepare`, starts each peer on that network and hands the checker a `CheckContext` with that `Network`. The `CheckerLab` interface addresses every peer by name.
- [ ] `internal/le/interoplab/docker.go` - `NetworkSpec` (name, subnet candidates), `Network` (selected IPv4 and IPv6), `PeerConfig` (name, container, image, host number, mounts, capabilities, environment, arguments, command, ready probe, AppArmor profile), `ImageBuild`.
- [ ] `internal/le/interoplab/bgp/prepare_links.go` - `labLink` (VLAN, IPv4 /24 at most, members as indexes into `linkLocalOwners`), `scenarioLinks` map filled by register files, `renderLinks` and `validateLink` (refusals), `writeLinkScript` (adds a VLAN subinterface of `eth0` named `eth<N>` in declaration order, its address, link up).
- [ ] `internal/le/interoplab/bgp/prepare.go` - `linkLocalOwners` lists three owners (`ze`, FRR, the FRR sink, one per `labconfig.LinkLocals` entry) with their entrypoint and image command; `wrapStart` replaces a member's entrypoint with a shell script that adds the eth0 link-local, the links and the prestart, then execs the original entrypoint.
- [ ] `internal/le/interoplab/bgp/register_import_policy.go` - the one `scenarioLinks` entry (`isis-import-policy-frr`, VLANs 101 and 102).
- [ ] `internal/le/interoplab/catalog.go`, `discover.go` - `RegisterCatalog` and `Discover`, shared by every suite.
- [ ] `internal/le/interoplab/l2tp/l2tp.go`, `radiussubscriber/radiussubscriber.go`, `pppoe/pppoe.go`, `ipsec/ipsec.go`, `internal/le/test/deployment/wireguardproof.go` - each builds its own `interoplab.Suite` with its own `imageBuilds`; `l2tp` and `radiussubscriber` name their single Ze peer `peerZe = "ze"`. 113 non-test call sites in `internal/le/interoplab` address the Ze peer by that literal.
- [ ] `internal/le/interoplab/pppdevice.go` - `PPPDeviceArguments`, the `/dev/ppp` grant every PPP container needs.
- [ ] `internal/le/test/integration/gates.go` - the `interop*` verbs in the action list `Table` returns (`interop`, `interop-ipsec`, `interop-radius`, `interop-radius-subscriber` and the rest).

**Behavior to preserve:**
- Every existing suite, scenario name, catalog entry, selector variable and native action.
- `isis-import-policy-frr` devices and addresses: FRR-A on VLAN 101 (`10.101.0.0/24`, Ze `eth1`), FRR-B on VLAN 102 (`10.102.0.0/24`, Ze `eth2`).
- Every link refusal and its message's named fix.
- The BGP eth0 link-local placement and its placeholders.

**Behavior to change:**
- Links move from the BGP package to the core, open to any peer.
- Images become registered by name in the core; suites derive their lists from it.
- A scenario may plan several Ze peers.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| BGP-suite links between daemons | `internal/le/interoplab/bgp/prepare_links.go` `labLink`, `scenarioLinks`, `renderLinks`, `validateLink`, `writeLinkScript`; `docs/architecture/testing/interop.md` "Separate links between daemons" | declares VLAN links between members of `linkLocalOwners`, refuses bad links, and writes the `eth<N>` subinterface script, for the BGP suite only | replaces: the link type, its refusals and its script move to the `interoplab` core, open to any peer; the BGP copy is deleted, not kept beside it |
| Entrypoint wrapper | `internal/le/interoplab/bgp/prepare.go` `wrapStart`, `linkLocalOwners` | replaces a member's entrypoint with a script that adds the eth0 link-local, the links and a prestart, then execs the original entrypoint | replaces: `wrapStart` is deleted and its work becomes `PeerConfig.Prestart` composed in the core |
| Scenario run | `internal/le/interoplab/lab.go` `ScenarioPlan`, `Suite.Run`, `Suite.runScenario`, `CheckerLab` | one network per scenario, one `Preparer`, peers addressed by name | extends: `ScenarioPlan` gains links and may plan several Ze peers |
| Docker network and peer | `internal/le/interoplab/docker.go` `NetworkSpec`, `Docker.CreateNetwork`, `PeerConfig`, `ImageBuild` | creates the scenario network and starts each peer from its config | extends: `PeerConfig` gains `Prestart`; image inspect reads entrypoint and command |
| Per-suite image lists | `Suite.Images` in each suite builder (`l2tp/l2tp.go`, `radiussubscriber/radiussubscriber.go`, `pppoe/pppoe.go`, `ipsec/ipsec.go`, `internal/le/test/deployment/wireguardproof.go`) | each suite builds its own `[]ImageBuild` | replaces: images register by name in the core and suites derive their lists from that registry |
| Scenario catalog and discovery | `internal/le/interoplab/catalog.go` `RegisterCatalog`, `discover.go` `Discover` | shared by every suite | extends: the new `topology` suite registers through it unchanged |
| `/dev/ppp` grant | `internal/le/interoplab/pppdevice.go` `PPPDeviceArguments` | the device grant every PPP container needs | extends: reused by the `two-ze-pppoe-link` scenario |
| Second Ze daemon in the BGP suite | `internal/le/interoplab/bgp/prepare.go` `zeDaemonPeer`, `scenarioPeers`; `bgp/names.go` `peerZePeer`, `zePeerConfigFile`; `docs/architecture/testing/interop.md` "Scenario Structure" (`ze-peer.conf`) | a `ze-peer.conf` starts a second Ze, `ze-peer`, on FRR's host 3 (`ospf-multiaf-v4-frr`); a scenario carrying both `frr.conf` and `ze-peer.conf` is refused | reuses: `zeDaemonPeer` is how a Ze node starts, and the core's several-Ze-nodes plan calls it for each node; replaces: the fixed `ze-peer` name and fixed host become one planned Ze node of the core declaration, so the BGP suite keeps `ze-peer.conf` as its file name and its refusal, and builds the node through the core like every other Ze node. So "no scenario runs two Ze nodes" in the Task is wrong: the BGP suite runs two, and the RSVP-TE suite up to four |
| Role topology of the RSVP-TE suite | `internal/le/interoplab/rsvpte/zenode.go` `zeNodeScript`, `netLineVLAN`; `docs/architecture/testing/interop.md` "The freeRouter RSVP-TE suite" | up to four nodes in fixed roles on one segment; `<role>.conf` makes a role a Ze node; each Ze node's `<role>-net.txt` declares `vlan <name> id <n> address <a/len>` and `route` lines that `zeNodeScript` turns into its start command | reuses: the role idea (a named node, a configuration file per node, typed declarations built into commands in Go, no shell script in a scenario) is the shape of the core declaration; replaces: the `vlan` lines are links by another name, so they move to the core link declaration in this change (`ai/rules/no-layering.md`), and `zeNodeScript` keeps only the `route` lines and the MPLS sysctls as the node's prestart. The RSVP-TE scenarios stay green with the same devices and addresses (added to Behavior to preserve). Its freeRouter nodes join the segment through `rawInt.bin`, not VLANs, and are not link members |
| Integration verbs | `internal/le/test/integration/gates.go` | the `interop*` native actions | extends: one `interop-topology` verb |

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
| AC-6 | A scenario plans three Ze peers with distinct names and configuration files | each starts with its own file; the checker queries each by name. Evidence: the smoke scenario `two-ze-static-link` is replaced by `ze-static-link-chain`, three Ze nodes A, B and C on two IPv4 links (A-B and B-C, B forwarding), whose checker runs `show interface brief` on each node by name and pings C from A through B; `two-ze-pppoe-link` stays the two-node layer-2 smoke |
| AC-7 | `isis-import-policy-frr` after the move | green, same devices and addresses; `labLink`, `scenarioLinks` and `renderLinks` exist nowhere |
| AC-7b | `ospf-multiaf-v4-frr` (BGP suite `ze-peer`) and every RSVP-TE scenario after the move | green, same devices and addresses; no `vlan` line parser remains in `rsvpte/zenode.go` |
| AC-8 | A topology scenario names an image another suite registered (FreeRADIUS, strongSwan, accel-ppp) | the image is built once by the registry's declaration and the container runs |
| AC-9 | A scenario names an unregistered image | refused at plan time, naming the registered images |
| AC-10 | Two suites register the same image name | the registration panics at init with both owners named |
| AC-11 | Each existing suite's image list comes from the registry | development evidence, owed by this spec's closure: `TestSuiteImageListsUnchanged` compares each suite's registry-derived image list with the list it built before the change, and each suite runs one named scenario green (`./le test integration <verb> <scenario>`), plus in full the suites whose start path this spec moves (`isis-import-policy-frr`, `ospf-multiaf-v4-frr`, the RSVP-TE scenarios) |
| AC-11b | Each existing suite runs its full scenario list | same verdicts as before. This is the release check, not development evidence: it is cited from the CI `verify` workflow run on the pushed commit (`ai/META-DECISIONS.md`, "Ze is pre-release"), and the spec's closure does not wait on a local run of every suite |
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
| `ze-static-link-chain` | `test/interop-topology/scenarios/` | three Ze nodes A, B, C on two IPv4 links (A-B, B-C); each answers `show interface brief` by name; A pings C through B; a container on eth0 alone cannot reach a link address (AC-1, AC-6) | |
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
- `docs/architecture/testing/qemu-integration.md` - "VPN facilities without module loading" (the design page `wireguardproof.go` declares): read, and nothing there becomes wrong; it describes the WireGuard proof's checks and module guards, not how its image list is built, so registering the image by name leaves every sentence true

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
| 16 | Changed source referenced by doc anchors? | Yes | derived with `./le spec citation anchors`; `interop.md` anchored the BGP link file (deleted) and `internal/le/interoplab/bgp/prepare.go` |
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
   - Tests: `TestImageRegistryDuplicatePanics`, `TestPlanRefusesUnknownImage`, `TestSuiteImageListsUnchanged`; one named scenario per suite, and in full the moved start paths (AC-11). The full list of every suite is AC-11b, the CI release run
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

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output): the scoped package tests of `internal/le/interoplab` and `internal/le/interoplab/bgp` under `./le job run`, then `./le test integration interop-topology` and `./le test integration interop isis-import-policy-frr`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

## Implementation Summary

### What Was Implemented
- Core links: `internal/le/interoplab/link.go` (`Link`, `LinkMembership`, `ValidateLinks`, `LinkMemberships`, `linkScript`, `ImageStart`, `parseImageStart`, `ComposeStart`, `wrapStart`, `linkToolRefusal`), with external ends (`Link.External`) and named devices (`Link.Devices`) for the RSVP-TE freeRouter VLAN and files that already name a device. `lab.go` `ScenarioPlan.Links`, `CheckContext.Links`/`Memberships`, `runScenario` validates links and `wrapStarts` composes one wrapper per peer; `docker.go` `PeerConfig.Prestart`, `Docker.ImageStart`, `probeLinkTools`. Commit 60c615ead1.
- Image registry: `internal/le/interoplab/imageregistry.go` (`RegisterImage`, `SuiteImages`, `ResolveImages`); every suite (bgp, l2tp, radius, radiussubscriber, pppoe, ipsec, rsvpte, deployment WireGuard, core FreeRADIUS) registers by name. 60c615ead1.
- BGP migration: `bgp/prepare_links.go` and the BGP `wrapStart` deleted; link-local and scenario prestarts become `PeerConfig.Prestart`; `register_import_policy.go` declares `interoplab.Link`s through `linkDeclarations`. 60c615ead1.
- RSVP-TE: the `vlan` line of `<role>-net.txt` removed; `rsvpte.go` `labLinks` declares the protected VLAN as a core link with freeRouter as its external end (AC-7b). 75e454a300.
- Topology suite `internal/le/interoplab/topology/`, verb `interop-topology` (`gates.go`, keyword `scenario`), scenarios `ze-static-link-chain` and `two-ze-pppoe-link`. 60c615ead1; lab fixes 0204244dd1, 8e2c5c8dca.
- Closure fix: `lab.go` `imageStarts` keeps the link probe cache apart from the start cache (Review Gate finding 1); `internal/le/interoplab/wrapstarts_test.go`.

### Bugs Found/Fixed
- An image first wrapped for a Prestart alone was never probed when a later scenario of the same run made it a link member, because the start cache answered first (`lab.go` `wrapStarts`). Covered by `TestWrapStartsProbesAnImageFirstSeenWithoutLinks`.

### Documentation Updates
- `docs/architecture/testing/interop.md` "Separate links between daemons" rewritten for the core (anchors `link.go`, `docker.go` `PeerConfig, ImageStart, probeLinkTools`), "The topology suite" section, suites row; `docs/functional-tests.md` topology row (anchor `gates.go`). Landed in 60c615ead1. The closure fix makes the existing sentence "A member's image is probed once for `sh` and `ip`" true in every scenario order; no page edit needed.
- `internal/le/interoplab/bgp/prepare_bfd_link_local.go` header: the stale `wrapStart` mention repointed at `interoplab.ComposeStart`.

### Deviations from Plan
- AC-6: the smoke scenario `two-ze-static-link` became `ze-static-link-chain` (three Ze nodes), as AC-6 itself records.
- `Link` gained `External` and `Devices`, not in the design: the RSVP-TE protected VLAN has one container member (freeRouter terminates it in its jar over rawInt) and its checker downs a device named `prot0`.
- Implementation commits carry `Spec-work: home-cpe-1-logical-interface-name`, the claim of the session that wrote them, not this spec's stem.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The probe-once cache was the start cache, keyed on the first role an image appeared in | Probing and inspecting are separate facts; a Prestart-only peer is inspected, never probed | Review Gate, reading `wrapStarts` | Split into `imageStarts`, regression test |
| approach | Implementation commits were made under another spec's claim | `Spec-work:` comes from the session's claim, so the commits name `home-cpe-1-logical-interface-name` | closure, `git log` trailers | Recorded here; this spec sat in `design` with no claim while it was implemented |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Named links, optional IPv4/IPv6, prefix-less layer 2 | Done | `internal/le/interoplab/link.go` `Link`, `linkScript` | |
| 2. Any planned peer can be a member | Done | `link.go` `validateLink` (members are planned peer names) | `linkLocalOwners` no longer gates membership |
| 3. Several Ze nodes, own name and config | Done | `topology/topology.go`, scenario `ze-static-link-chain` | |
| 4. Images registered by name, suites derive lists | Done | `imageregistry.go`; each suite's `register_images.go` | `TestSuiteImageListsUnchanged` |
| 5. `topology` suite | Done | `internal/le/interoplab/topology/`, `gates.go` `interop-topology` | |
| BGP link mechanism deleted | Done | `bgp/prepare_links.go` gone; `git grep -w -E 'labLink\|scenarioLinks\|renderLinks'` empty | the `wrapStart` hits are the core's own wrapper in `link.go` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLinkRenderDevicesAndAddresses`; `ze-static-link-chain` pass, red walk `c18-chain-links-broken` | |
| AC-2 | Done | `two-ze-pppoe-link` pass; red walks `c18-pppoe-down`, `c18-pppoe-leak` (the eth0-only observer captures untagged PADI when the link goes untagged) | |
| AC-3 | Done | `TestLinkRenderDevicesAndAddresses` (IPv6 /64, host octet, nodad) | |
| AC-4 | Done | `TestLinkValidateRefusals`, `TestLinkDeviceNameRefusals`, `TestLinkExternalEnd` | |
| AC-5 | Done | `TestLinkToolRefusalNamesTheDockerfile`, `TestWrapStartsProbesAnImageFirstSeenWithoutLinks` | |
| AC-6 | Done | `ze-static-link-chain` pass (three nodes, `show interface brief` by name, A pings C through B) | |
| AC-7 | Done | `isis-import-policy-frr` pass, red walk `c18-isis-links-broken`; `TestBGPImportPolicyLinksUnchanged` | |
| AC-7b | Done | `ospf-multiaf-v4-frr` pass; RSVP-TE 10 passed; `TestBackupPathProtectedLinkFromCore`; `zenode.go` `zeNodeNet.add` refuses any line but `route` | |
| AC-8 | Done | topology scenarios of children 11 and 19 run FreeRADIUS, strongSwan and accel-ppp images (`register_home_lab.go`, `register_wholesale_accel.go`) | |
| AC-9 | Done | `TestPlanRefusesUnknownImage` | |
| AC-10 | Done | `TestImageRegistryDuplicatePanics` | |
| AC-11 | Done | `TestSuiteImageListsUnchanged`; one scenario per suite green (`rwinv/out-c18/*.log`: bgp, rsvpte, radius, radius-subscriber, topology, ipsec, pppoe, l2tp-ppp 13 passed); WireGuard `./le test qemu vpn-kernel` PASS | |
| AC-11b | Changed | release check, cited from the CI `verify` run on the pushed commit, as the AC states | closure does not wait on it (AC text) |
| AC-12 | Done | `lab.go` `Suite.Run` calls `DockerKernel(s.StagedZe)` before any image build; topology builds a `Suite` like every suite | no exemption |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLinkValidateRefusals` | Done | `internal/le/interoplab/link_test.go` | |
| `TestLinkRenderDevicesAndAddresses` | Done | `link_test.go` | |
| `TestStartWrapperComposesLinksPrestartEntrypoint` | Done | `link_test.go` | |
| `TestImageRegistryDuplicatePanics` | Done | `imageregistry_test.go` | |
| `TestPlanRefusesUnknownImage` | Done | `imageregistry_test.go` | |
| `TestBGPImportPolicyLinksUnchanged` | Done | `bgp/prepare_links_test.go` | file name kept, refusal cases moved to core |
| `TestTopologyCatalogDiscovers` | Done | `topology/topology_test.go` | |
| `TestSuiteImageListsUnchanged` | Done | `topology/image_lists_test.go` | |
| `ze-static-link-chain` | Done | `test/interop-topology/scenarios/ze-static-link-chain/` | |
| `two-ze-pppoe-link` | Done | `test/interop-topology/scenarios/two-ze-pppoe-link/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/interoplab/lab.go`, `docker.go` | Done | |
| `internal/le/interoplab/bgp/prepare.go`, `register_import_policy.go` | Done | |
| suite image builders, `wireguardproof.go` | Done | `register_images.go` per suite |
| `internal/le/test/integration/gates.go` | Done | |
| `docs/architecture/testing/interop.md`, `docs/functional-tests.md` | Done | |
| `docs/architecture/testing/qemu-integration.md` | Done | read, nothing became wrong |
| `link.go`, `imageregistry.go`, `topology/`, scenario dirs | Done | |
| `bgp/prepare_links.go` deleted | Done | |

### Audit Summary
- **Total items:** 37
- **Done:** 36
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-11b, by its own text a CI release check)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Links are a property of the shared core, open to any peer | interop | `isis-import-policy-frr` green on core links and red with links broken (`rwinv/out-interop/c18-isis-links-*.log`) |
| A prefix-less layer-2 link isolates PPPoE | interop | `two-ze-pppoe-link` green; red when the link goes untagged (`c18-pppoe-leak-broken.log`: "observer, on eth0 alone, received untagged PPPoE discovery") |
| Several Ze nodes per scenario | interop | `ze-static-link-chain` green, red with links broken (`c18-chain-links-*.log`) |
| Images registered once, suites derive lists | unit + interop | `TestSuiteImageListsUnchanged`; every suite one scenario green (`rwinv/run-c18-suites.log`), WireGuard proof PASS (`rwinv/out-gb5/vpn-kernel.log`) |
| A `topology` suite for children 19 and 11 | interop | `./le test integration interop-topology`; children 19 and 11 scenarios register in it |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is done; AC-11b is the CI release run by its own text | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-18-interoplab-multi-link-topology-7e292f27-ced0-427f-a7b6-3a931db211f9.md` |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | logic + wiring (`wrapStarts`, `wrapStart`, `ValidateLinks`, registry), security (script quoting, device names, capabilities), removed behavior (BGP refusals moved to core), style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A link member whose image was first wrapped for a Prestart alone skipped the `sh`/`ip` probe (the start cache answered first), so AC-5's named refusal was lost in that order, and the `runScenario` comment claimed a probe record it did not keep | `internal/le/interoplab/lab.go` `wrapStarts` | `imageStarts` with a separate `probed` map; `TestWrapStartsProbesAnImageFirstSeenWithoutLinks` |

### Run 2
0 BLOCKER, 0 ISSUE. NOTEs: `wrapStart` keeps a trailing `--entrypoint` with no value as an argument (Docker refuses it, no wrapper can be built); child 11's CPE node adds `BPF` beside `NET_ADMIN`, declared by that member as the Security checklist allows. Style pass over `lab.go` and `wrapstarts_test.go`: no peer-reachable panic (the `linkVLAN` panics in `link.go` fire only on a membership `LinkMemberships` built from a validated declaration), one fact per guard, `textbuf` for strings.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/link.go`, `link_test.go`, `imageregistry.go`, `imageregistry_test.go` | Yes | read at closure |
| `internal/le/interoplab/topology/` | Yes | `topology.go`, `register_catalog.go`, `topology_test.go`, `image_lists_test.go` |
| `test/interop-topology/scenarios/ze-static-link-chain/`, `two-ze-pppoe-link/` | Yes | `ls test/interop-topology/scenarios/` |
| `internal/le/interoplab/bgp/prepare_links.go` | Deleted | `ls`: No such file or directory |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5, AC-9, AC-10 | unit tests pass | `./le job run label unit-pkg command go test -count=1 ./internal/le/interoplab/...`: every package ok (pppoe under `ze_l2tp`, where its wire checkers are gated) |
| AC-7 | BGP link code gone | `git grep -w -E 'labLink\|scenarioLinks\|renderLinks' -- internal/le`: no hit |
| AC-11 | suites green | `rwinv/run-c18-suites.log` all exit=0; `rwinv/out-gb5/vpn-kernel.log` "WireGuard kernel ping and no /sbin/modprobe: PASS" |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test integration interop-topology` | scenario `ze-static-link-chain` (interop scenario, no .ci) | pass, `rwinv/out-c18/topology.log` |
| `./le test integration interop-topology` | scenario `two-ze-pppoe-link` | pass, `c18-pppoe-*-clean.log` |
| `./le test integration interop isis-import-policy-frr` | scenario | pass, `c18-isis-links-clean.log` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `two-ze-pppoe-link`: the eth0-only observer captures no untagged PADI, and red when untagged. A packet socket on eth0 still sees the tagged frame (`link.go` `Link` comment), which no protocol socket receives |
| A-2 | confirmed | `TestParseImageStart`; every wrapped scenario starts its daemon |
| A-3 | confirmed | every suite's member images pass `probeLinkTools`; the refusal names the Dockerfile (`TestLinkToolRefusalNamesTheDockerfile`) |
| A-4 | confirmed | `ze-static-link-chain` runs three Ze nodes from one `StagedZe` |
| A-5 | confirmed | child 19's `lcpMRUVerdict` (`topology/check_access_capture.go`) asserts the negotiated MRU over the core link |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| interop.md "Separate links between daemons" | `link.go` `wrapStart`, `ValidateLinks`; `docker.go` `probeLinkTools`; "probed once" now holds in every order (`lab.go` `imageStarts`) | Yes |
| interop.md "The topology suite", functional-tests.md row | `gates.go` `interop-topology`, `topologyScenarioKeyword` | Yes |
| prepare_bfd_link_local.go header | `link.go` `ComposeStart` | Yes |

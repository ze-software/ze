# Spec: dataplane-seams-4 -- Shared Control-Packet Receive Path: Decide Whether One Should Exist (Skeleton)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/spec-dataplane-seams-0-umbrella.md` (finding F-4) |
| Phase | - |
| Deferral shard | `plan/deferrals/dataplane-seams.md` (create on the first deferral) |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**This spec's deliverable is a decision and a written design, not an
implementation.** "No shared path, and here is why" is a legitimate and complete
outcome. Do not let it turn into a subscriber-feature spec.

Ze has no shared control-packet receive path. Ten protocols each open their own
socket in their own transport package, and each defines its own per-packet
metadata type. No transport package imports another's.

Three properties follow, and each is a real constraint on anything built later.

| Property | Evidence | Verified |
|----------|----------|----------|
| No receive path anywhere carries a VLAN tag. `PACKET_AUXDATA`, `TP_STATUS_VLAN` and `VlanTCI` appear nowhere in the tree, so the kernel strips the tag and ze never sees it. VLAN is expressed only by binding to a sub-interface | tree-wide search, 2026-08-07 | Read |
| DHCP learns neither its source nor its ingress interface. `serveMulti` discards the address returned by `ReadFromUDP`, then tries each handler in content order until one replies | `serveMulti` in `internal/plugins/dhcpserver/register.go` | Read |
| There is no ARP receive path at all. Only a gratuitous-ARP send. Neighbor state comes from netlink, which means from the kernel having already resolved it | `sendGARPLocked` in `internal/plugins/vrrp/transport/garp_linux.go`; `internal/component/iface/netlink/neighbor_linux.go` | Unverified |

### The receive paths as they stand

**Rows marked unverified were reported by a research agent and not read
directly. Confirm each against its producing function before designing
(`ai/rules/evidence.md`).**

| Protocol | Producer | Transport | Verified |
|----------|----------|-----------|----------|
| DHCP | `serveMulti`, `listenDHCP` in `internal/plugins/dhcpserver/` | UDP, one socket per configured interface | Read |
| BFD | `internal/component/bfd/transport/udp.go` | UDP 3784 / 4784 | Unverified |
| PPPoE discovery | `internal/component/l2tp/pppoe/kernel_linux.go` | packet socket, PPPoE discovery ethertype | Unverified |
| IS-IS | `internal/plugins/isis/transport/backend_linux.go` | packet socket, bound per circuit | Unverified |
| OSPFv2 | `internal/plugins/ospf/transport/backend_linux.go` | raw IP protocol 89 | Unverified |
| OSPFv3 | `internal/plugins/ospf/v3/transport/backend_linux.go` | raw IP protocol 89 | Unverified |
| VRRP | `internal/plugins/vrrp/transport/backend_linux.go` | raw IP protocol 112 | Unverified |
| RSVP-TE | `internal/plugins/rsvpte/transport_linux.go` | raw IP protocol 46 | Unverified |
| IKE | `internal/component/ike/transport/udp.go` | UDP 500 / 4500 | Unverified |
| IPv6 RA / RS | `internal/plugins/iface/ra/sender_linux.go` | ICMPv6, filtered to router solicitation | Unverified |

Four unrelated per-packet metadata types were reported: `transport.Inbound`
(bfd), `wire.RawPacket` (ospf), `transport.RawFrame` (isis), `packet.RxMeta`
(vrrp). Unverified.

### What the decision has to weigh

| For a shared path | Against |
|-------------------|---------|
| Per-packet metadata is inherited by everything built on top. Ten sockets with four metadata shapes cannot be merged once features depend on them | Ten working implementations exist. Replacing them is risk with no user-visible gain |
| VLAN information is unobtainable today on every path. Any feature keyed on a tag needs it, and retrofitting it into ten sockets is ten changes | Nothing today needs a VLAN tag. Adding the capability before a consumer exists is speculation |
| Control-plane policing cannot see what it does not own (child 5) | Policing can be solved in the firewall layer without touching the receive paths |
| A second dataplane would otherwise need ten separate ports | No second dataplane consumes control packets today |

### Rules that bind this decision

- `ai/rules/no-layering.md`: if a shared path replaces the per-protocol sockets, the old ones are deleted, not left beside it. A shared path that coexists with ten private ones is worse than either.
- `ai/rules/completion.md`: partial migration is not completion. Deciding to migrate means migrating all of them.
- `ai/rules/goroutine-lifecycle.md`: a receive path is a long-lived worker reading from a channel, never a goroutine per packet.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation and where a shared transport would belong
- [ ] `ai/rules/architecture.md` - tier rules. A shared receive path is a candidate `internal/core` leaf, and its placement decides whether `make ze-tier-check` passes
- [ ] `ai/rules/no-layering.md` - delete X before implementing Y
- [ ] `ai/rules/goroutine-lifecycle.md` - long-lived workers, never per-packet goroutines
- [ ] `ai/rules/platform-linux.md` - a packet-socket path is Linux-only and needs QEMU integration tests

### Related Specs
- [ ] `plan/spec-dataplane-seams-0-umbrella.md` - the parent, finding F-4
- [ ] `plan/spec-dataplane-seams-5-copp-non-tcp.md` - the policing side of the same question
- [ ] `plan/spec-cp-survival-0-umbrella.md` - in-progress, owns control-plane survivability

**Key insights:** (minimal context to resume after compaction)
- The deliverable is a decision plus a written design. "No shared path" is a complete answer.
- If the answer is yes, `no-layering.md` means the ten existing sockets are deleted, not left in place.
- Nothing today needs a VLAN tag. That absence is the strongest argument against acting now, and the strongest argument for deciding the metadata shape before something does.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-07)
- [ ] `internal/plugins/dhcpserver/register.go` - `serveMulti` reads with `ReadFromUDP`, discards the address, tries each handler in order
- [ ] tree-wide search for `PACKET_AUXDATA`, `TP_STATUS_VLAN`, `VlanTCI` - no match

**Source files to read before design:**
- [ ] every transport named in the receive-path table, to replace `Unverified` with evidence
- [ ] `internal/core/capture/` - the existing pcap-style debug capture, to confirm it is not a punt path and to see whether it already solves part of the problem

**Behavior to preserve:**
- Every protocol keeps working. This spec proposes no behavior change on its own.
- `ze diag capture` and the debug capture path are untouched.

**Behavior to change:**
- None. The output is a decision and a design document.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Ten independently opened sockets, each owned by the protocol that reads it.

### Transformation Path
1. Each transport package opens its own socket, with its own filter and its own binding.
2. Each reads into its own buffer and builds its own metadata value.
3. Each hands that value to its own protocol logic. No type is shared.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| kernel ↔ protocol plugin | one socket per protocol, no shared type | No |
| protocol plugin ↔ protocol logic | a per-protocol metadata struct | No |
| (proposed) kernel ↔ shared receive path ↔ protocol plugin | to be designed | No |

### Integration Points
- each protocol's `transport` package - what a shared path would replace
- `internal/plugins/copp` - what would gain visibility if a shared path existed
- `internal/core/` - where a shared path would live if the tier rules allow it

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A shared receive path is wanted at all | Not stated by the user. This spec exists to answer it | The spec closes as "no change", which is a valid outcome | The design gate | unvalidated |
| A-2 | The ten transports are genuinely independent and none already shares a type | No transport package imports another's, per a research agent | The unification is smaller than assumed | Read each transport package | unvalidated |
| A-3 | A packet socket can supply ingress interface and VLAN metadata that the current paths discard | Standard Linux capability, not verified in this tree | The shared path cannot deliver its main benefit | A QEMU probe before designing (`ai/rules/platform-linux.md`) | unvalidated |
| A-4 | No current feature needs a VLAN tag on receive | Tree-wide search found no VLAN metadata handling | The absence is already a live defect, not a future constraint | Search for any feature keyed on a received tag | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The spec expands into subscriber or access work it was not scoped for | The design discusses session state rather than packet delivery | The deliverable is a decision and a design. Stop at that boundary |
| R-2 | A shared path lands beside the ten existing sockets rather than replacing them | Both exist after the change | `ai/rules/no-layering.md`. Migrating means migrating all ten, or not starting |
| R-3 | Ten working protocol implementations are destabilised for no user-visible gain | Protocol functional tests regress | Each protocol migrates behind its own passing functional test, or the migration does not happen |
| R-4 | A shared path becomes a single point of failure for every protocol at once | One bug stops BGP, OSPF, IS-IS and BFD together | Weigh this explicitly in the decision. Ten independent failures may be preferable to one shared one |
| R-5 | The design is written and never revisited, so the findings rot | The spec sits at skeleton for a year | That is acceptable. The findings table is the value, and it is recorded |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing, while the spec produces only a decision. If a migration follows, every routing and access protocol at once |
| How is it reverted? | The decision is free to revisit. A migration is not, once ten transports are deleted |
| Who else touches this path? | `spec-cp-survival-0-umbrella` (control-plane survivability), child 5 (policing), every protocol spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| This spec produces no code. The wiring table is filled only if the decision is to build a shared path, and then it names one row per migrated protocol | → | (fill only if the decision is to build) | (fill only if the decision is to build) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The research phase completes | Every `Unverified` row in both tables above is replaced by evidence naming the producing function |
| AC-2 | The design gate is reached | A written decision exists, with the weighing recorded, and it is either "no shared path, because X" or a design naming the metadata shape and the migration order |
| AC-3 | The decision is to build | The design names every one of the ten transports and states that all are migrated, per `ai/rules/no-layering.md`. A partial migration is not an option on the table |
| AC-4 | The decision is not to build | The reason is recorded in the umbrella's findings table so the finding is not re-derived later |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (none while the deliverable is a decision) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none while the deliverable is a decision) | | Filled only if the decision is to build | |

### Functional Tests
<!-- The regression baseline any shared path must preserve. These already pass;
     if a migration happens, every one of them must still pass unchanged. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-route-install` (existing baseline) | `test/isis/isis-route-install.ci` | IS-IS still forms adjacency and installs routes | |
| `ospf-route-install`, `ospf-route-daemon` (existing baseline) | `test/ospf/ospf-route-install.ci`, `test/ospf/ospf-route-daemon.ci` | OSPF still forms adjacency and installs routes | |
| `dhcp-pxe-config`, `dhcp-zero-listener` (existing baseline) | `test/install/dhcp-pxe-config.ci`, `test/install/dhcp-zero-listener.ci` | The DHCP server still answers and still handles a zero-listener config | |
| `doctor-dhcp-iface` (existing baseline) | `test/ui/doctor-dhcp-iface.ci` | The DHCP interface doctor check still reports correctly | |
| new: shared path delivers ingress and VLAN metadata | `test/*/*.ci` (only if the decision is to build) | A tagged frame reaches its protocol handler with the ingress interface and both tags intact | |

## Files to Modify
- (none while the deliverable is a decision)

## Files to Create
- `docs/architecture/` - the written design or the recorded decision (exact path at design)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A decision adds no config. Revisit if the decision is to build |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | N-A | No RPC while the deliverable is a decision |
| Pipe completeness | N-A | No CLI output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | A decision opens no socket. **A shared path would open one, and would then need a doctor check and a diagnostic code** (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | N-A | A shared path would want received and dropped counters per protocol. Revisit if the decision is to build |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | A decision ships no feature |
| 2 | Config syntax changed? | N-A | No syntax change |
| 3 | CLI command added/changed? | N-A | No command change |
| 4 | API/RPC added/changed? | N-A | No API change |
| 5 | Plugin added/changed? | N-A | No plugin added |
| 6 | Has a user guide page? | N-A | Not user-facing |
| 7 | Wire format changed? | N-A | No wire format change |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC obligation touched by a decision. A migration would need each protocol's RFC tests to still pass |
| 10 | Test infrastructure changed? | N-A | No test infrastructure change |
| 11 | Affects daemon comparison? | N-A | No externally comparable behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - the decision belongs there whichever way it goes |
| 13 | Route metadata keys added/changed? | N-A | No route payload change |
| 14 | Prometheus counters added/changed? | N-A | No counters while the deliverable is a decision |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | N-A | No source file changes |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | No syntax change |

## Implementation Steps

1. **Phase: verify** -- replace every `Unverified` row in both tables with evidence from the producing function.
2. **Phase: probe** -- establish in QEMU whether a packet socket can supply the ingress interface and VLAN metadata the current paths discard (`ai/rules/platform-linux.md`). Without this, the main benefit is unproven.
3. **Phase: weigh and decide** -- write the decision, with the weighing recorded. If it is "no shared path", record the reason in the umbrella and close.
4. **Phase: design (only if the decision is to build)** -- name the metadata shape, the placement and its tier, the migration order, and how all ten transports are deleted rather than duplicated. Stop here. Implementation is a separate spec.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every `Unverified` row replaced with evidence |
| Correctness | The decision weighs the single-point-of-failure risk explicitly, not only the benefits |
| Data flow | Any proposed design deletes the paths it replaces (`ai/rules/no-layering.md`) |
| Rule: `ai/rules/goroutine-lifecycle.md` | Any proposed design uses long-lived workers, never a goroutine per packet |
| Rule: `ai/rules/platform-linux.md` | Any proposed design has a QEMU test plan, not a "needs hardware" exemption |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The decision exists in writing | The file named in Files to Create |
| Every table row verified | No `Unverified` remains |
| The umbrella records the outcome | The findings table row for F-4 names the decision |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A shared receive path is a single queue for every protocol. Design the backpressure before the path, or one protocol's flood starves the rest |
| Input validation | A shared path parses untrusted frames before any protocol sees them, which moves the first parse into shared code. That parser is attack surface for every protocol at once |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- This spec deliberately stops at a design. Implementing a shared receive path, if that is the decision, is separate work with its own spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

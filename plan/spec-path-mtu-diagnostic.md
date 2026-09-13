# Spec: path-mtu-diagnostic

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Depends | spec-probe-do-not-fragment |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An IPsec tunnel sized larger than the path it rides on fragments or blackholes,
and neither symptom names its cause. The operator sees slow transfers, stalled
TLS handshakes and working pings, and has no way to ask the box what the path
actually carries.

`show mtu` answers that. It measures the real underlay path MTU to every
configured site-to-site peer and to an arbitrary host, derives the ESP ceiling
for each tunnel from the transform that tunnel actually negotiated, and reports
whether the interface is oversized, tight, under-utilised or correct, with the
configuration commands to fix it.

This ports a tool Exa runs in production on VyOS CPE (`faros_mtu.py`, version
2.2, from the FarOS package set), as part of migrating FarOS to Ze. The port is
not a translation. The Python shells out to `ping`, `ip`, `nstat` and `sysctl`
because VyOS operational mode is a shell, and it hardcodes AES-GCM-128 with an
IPv4 underlay and IPv6 inner because it cannot see what a tunnel negotiated. Ze
is the IKE daemon, so it derives the transform per child SA and refuses nothing.
The distilled arithmetic, probe budget, candidate ladder and verdict thresholds
are recorded in the session scratch note named under Required Reading.

The module is its own feature, removable by dropping a blank import (owner
requirement, 2026-09-11). It therefore takes IPsec state through a registered
snapshot rather than importing the IKE engine.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - cross-boundary value types and the communication patterns
  → Constraint: a payload crossing a boundary carries no pointer fields, so the tunnel snapshot is a self-contained value type
  → Decision: the EventBus is asynchronous one-to-many with no replay, so an event subscription cannot answer "what tunnels exist right now"; a module started after a tunnel came up would report none
- [ ] `ai/patterns/registration.md` - every registry and its Register and Query symbols
  → Constraint: a feature registers itself and is discovered; no central list learns its name
  → Decision: `internal/core/diagnostic/doctor_registry.go` is the shape to copy for a registered synchronous query, and the IKE component is already a registrant there
- [ ] `ai/patterns/cli-command.md` and `ai/rules/cli.md` - the command contract
  → Constraint: keyword before value, and the handler returns structured data so `| json`, `| yaml` and `| table` each render one payload
  → Constraint: a severity marker is never glued to a name; the verdict is its own field
- [ ] `docs/architecture/diagnostics/active-probes.md` - the probe layer this depends on
  → Constraint: probes are in-daemon sockets; `plan/spec-probe-do-not-fragment.md` adds the DF mode and the error-queue read this module consumes
- [ ] `docs/contributing/ze-go-style.md` - the working standard
  → Constraint: `netip.Addr` for an address, never a string; a typed enum whose zero means `Unspecified` for the verdict
  → Constraint: state the limit. The probe budget, the follow count and the candidate ladder are bounded and the bound is in the code

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc4301.txt` - Security Architecture for IP, Section 8
  → Constraint: Section 8.2.2, "In all IPsec implementations, the PMTU associated with an SA MUST be 'aged'". `plan/immediate/spec-rfc4301-architecture-gaps.md` phase 7 owns that value; this module READS it and never writes it (owner decision, 2026-09-11)
- [ ] `rfc/full/rfc1191.txt` - Path MTU Discovery
  → Constraint: Section 7 warns that a plateau table with too many entries wastes the search, and calls its own eleven values "an implementation suggestion, NOT a specification or requirement". The 31-value ladder is a deliberate departure, justified in Key Design Decisions (owner decision, 2026-09-11)
  → Constraint: Section 3, a reported MTU of zero is the old-router signal to start searching, not a value to discard. The ported Python discards it, which is a defect this port does not carry across
- [ ] `rfc/full/rfc4821.txt` - Packetization Layer Path MTU Discovery
  → Constraint: Section 7.6.4, "The presence of other losses near the loss of the probe may indicate that the probe was lost due to congestion rather than due to an MTU limitation. In this case, the state variables ... SHOULD NOT be updated". A bisection that moves its bounds on every silent probe walks the estimate down under congestion
- [ ] `rfc/full/rfc8899.txt` - Datagram PLPMTUD
  → Constraint: Section 5.1.3, "The default value of MAX_PROBES is 3". The ported Python retries once; three consecutive attempts of one size is the figure with an authority behind it
- [ ] `rfc/full/rfc8200.txt` - IPv6
  → Constraint: Section 5, the 1280 minimum link MTU is the floor below which no IPv6-carrying tunnel may be sized
- [ ] `rfc/full/rfc791.txt` - IPv4
  → Decision: 576 is a reassembly minimum, not a path-MTU floor, and is used here as a documented default rather than published as conformance

### Other
- [ ] `tmp/session/2026-09-11-fb4cab68-8dd5-42e7-955c-62baf7405f11/scratch/faros-mtu-tuning.md` - the distillation of `faros_mtu.py` 2.2
  → Constraint: the ESP arithmetic, the four verdict thresholds and the underlay advice matrix are reproduced from a tool in production; changing a threshold changes a verdict an operator already relies on
  → Decision: the colour-on-stderr handling, the subprocess prober and the refusal to advise on a non-standard proposal do not port

**Key insights:**
- The ESP ceiling is `alignDown(underlay - overhead, block) - trailer`, where the overhead is the outer IP header, 8 more octets when UDP encapsulation is in use, the 8-octet ESP header, the cipher IV and the cipher ICV. Every one of those except the outer header comes from the negotiated transform, which is why Ze can compute what the Python had to assume.
- The recommended MTU backs off 32 octets from the ceiling and lands on the cipher's alignment boundary, and is *absent* rather than small when that lands below the inner family's minimum link MTU.
- The reference measurement against an address unrelated to the tunnels is what makes the underlay advice decidable: a clamped access circuit is worth matching, a clamped peer path is not.
- A verdict is a field, not a prefix on a string, so the table renderer and `| json` agree by construction.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/reconcile.go` - `PeerInfo` and `(*PeerSession).Info()`; the snapshot taken under `ps.mu`, child fields filled only when the child SA exists
- [ ] `internal/component/ike/engine/child.go` - `ChildSA` holds `UDPEncap`, `Mode` and `RemoteAddr`; none of the three reaches `PeerInfo`
- [ ] `internal/component/ike/engine/responder.go` - `selectResponderESP` narrows `sa.ESPGroup.Proposals` to the single accepted proposal, and `matchOfferedESPProposal` is what selects it
- [ ] `internal/component/ike/engine/register.go` - `ActiveTable` and `ActivePeers` are process globals, reachable only from inside the IKE component
- [ ] `internal/component/ike/cmd/show_ipsec.go` - the existing read-only IPsec command and its row builder, the payload template to follow
- [ ] `internal/component/ping/cmd/register.go` and `internal/plugins/ping-cmd/yang/register.go` - the component-plus-YANG-plugin pair this module copies
- [ ] `internal/component/iface/dispatch.go` - `ListInterfaces`, `GetInterface` and `GetXFRMInfo`; the name-to-if_id mapping lives here, not in IKE
- [ ] `internal/component/iface/config.go` - the configured interface MTU, separate from the kernel one; no symbol reports both
- [ ] `internal/component/sysctl/backend_linux.go` - `read` is unexported and owns the key-to-path mapping including the interface-name case
- [ ] `internal/component/telemetry/collector/snmp_linux.go` - the vendored `procfs.ProcSnmp` is already read here, as rates rather than absolute values

**Behavior to preserve:**
- Every existing `show vpn ipsec sa` payload key. The negotiated-transform fix changes a value, not a key.
- The IKE engine's locking: the snapshot is taken the way `Info()` already takes it.
- `internal/component/sysctl`'s key-to-path mapping stays the single declaration of that mapping.

**Behavior to change:**
- `PeerInfo` gains the UDP-encapsulation flag, the mode, the installed remote address, and reports the negotiated transform rather than the configured one.
- The sysctl component gains an exported read, so no second copy of the key mapping is written.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show mtu`, `show mtu host <address>`, with `force` and `detail` available under either.
- No value at all in the default form: the peers come from the running IKE state.

### Transformation Path
1. The command handler asks the registered IPsec inventory for a snapshot of live tunnels.
2. For each distinct peer address, and for the reference address, the probe layer measures the path MTU.
3. Per tunnel, the ESP overhead is derived from that tunnel's negotiated transform, mode and encapsulation; the ceiling and the recommended value follow.
4. Each tunnel is classified against its interface's current MTU.
5. Local state is read: the kernel's cached PMTU, the fragmentation counters, and `tcp_mtu_probing`.
6. One payload is returned, carrying the measurements, the tunnel rows with their verdicts, the remediation commands and the notes. The pipe operators render it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| mtu ↔ ike | a registered snapshot query in a core leaf, value types only | No |
| mtu ↔ iface | `GetXFRMInfo` and `GetInterface` for the if_id-to-name resolution and the interface MTU | No |
| mtu ↔ sysctl | an exported read for `tcp_mtu_probing` | No |
| mtu ↔ probe layer | the DF mode and the error-queue read from `plan/spec-probe-do-not-fragment.md` | No |
| Component ↔ CLI | `ze-mtu-cmd.yang` and the registered RPC | No |

### Integration Points
- `internal/core/ipsecinventory` is new: the IKE engine registers a snapshot function at `init()`, the mtu module calls it. Neither imports the other.
- `internal/component/mtu/cmd/` registers the RPC and the local meta, copying `internal/component/ping/cmd/register.go`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | After negotiation, the single proposal left in the SA's ESP group is the one actually installed in the kernel | `selectResponderESP` narrows `sa.ESPGroup.Proposals` to the accepted proposal, and `installChildSA` reads index zero | the derived overhead is wrong for every tunnel and the tool repeats the failure it exists to remove | a QEMU test negotiating the SECOND of two configured proposals and asserting the reported transform and the derived overhead both follow it | unvalidated |
| A-2 | The cipher's IV and ICV lengths, and its alignment block, are derivable from the negotiated transform for every transform Ze offers | the transform identifies the algorithm, and the arithmetic is a property of the algorithm | the overhead is wrong for some ciphers and right for others, which is worse than being wrong for all | a table-driven test over every ESP transform Ze can negotiate, asserting the derived overhead against hand-computed values |
| A-3 | `ChildSA.RemoteAddr` is the address the SA is installed on and differs from the configured address behind NAT | the peer endpoint is adopted after authentication | probes go to the configured address and measure a path the ESP traffic does not ride | a NAT interop scenario asserting the probed address equals the installed one | unvalidated |
| A-4 | A registered snapshot query is reachable synchronously from the mtu module in every deployment shape | the doctor registry works this way today and IKE is already a registrant | the module reports no tunnels on a box that has them, which is a silently-wrong zero | the snapshot returns an explicit "not registered" outcome, and a functional test asserts the message an operator sees when IKE is absent | unvalidated |
| A-5 | The verdict thresholds carried over from `faros_mtu.py` are right for Ze's operators too | the tool is in production on Exa CPE | verdicts disagree with the tool operators already trust, during a migration where both run | the thresholds are stated in the spec and reviewed by the owner before implementation; a table-driven test pins each boundary | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `plan/immediate/spec-rfc4301-architecture-gaps.md` phase 7 introduces a per-SA PMTU while this module also holds one, and the two disagree | both specs edit the IKE engine's notion of a path MTU | this module READS and never writes (owner decision). Where phase 7 has landed, disagreement between the engine's value and the measured one is reported as a finding rather than silently resolved |
| R-2 | The measurement takes 30 to 60 seconds on a degraded path, and a CLI caller times out or an operator thinks the box has hung | a functional test with an unreachable peer | progress is emitted per peer as the run proceeds, and the probe budget is bounded so the worst case is computable rather than open |
| R-3 | Probing every configured peer from a production CPE emits traffic an operator did not expect | a customer asks why their firewall logged probes | the command is operator-invoked and never scheduled; the payload states what was sent and to where |
| R-4 | The reference measurement sends probes to an address outside the operator's control | a policy objection to pinging a public resolver by default | the reference target is a config leaf, defaulting to 1.1.1.1, and a run with it unset simply reports that the underlay advice is undecidable |
| R-5 | Deriving overhead per transform is more surface than hardcoding it, and a transform Ze can negotiate but whose arithmetic is unwritten produces a wrong number | a new ESP transform is added and nobody updates the overhead table | the overhead derivation is driven from the transform registry, so an unknown transform is an explicit refusal to advise rather than a default |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator applies an MTU derived from a wrong figure and clamps a working tunnel, or leaves an oversized one in place believing it checked. The command changes nothing itself, so the damage is through the advice it prints, which is why a wrong figure must be impossible rather than unlikely |
| How is it reverted? | Single commit revert for the module. The `PeerInfo` changes and the negotiated-transform fix are separable and worth keeping regardless |
| Who else touches this path? | `plan/immediate/spec-rfc4301-architecture-gaps.md` phase 7 (the per-SA PMTU), `plan/spec-probe-do-not-fragment.md` (the probe layer this depends on), `plan/spec-ike-padded-path-probe.md` (the later, more accurate prober) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show mtu` from the CLI | → | the registered RPC handler | `TestShowMTUReachesTheHandler` |
| `show mtu host <address>` | → | the same handler with one target | `TestShowMTUHostMeasuresOneAddress` |
| The IKE engine's `init()` | → | the registered inventory snapshot | `TestIPsecInventoryRegisteredByIKE` |
| `show mtu \| json` | → | the payload, rendered | `TestShowMTUPayloadRendersAsJSON` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show mtu host <address>` against a path clamped below the interface MTU | the measured path MTU equals the clamp, and the payload names how it was found |
| AC-2 | A peer whose child SA negotiated AES-GCM-128 without UDP encapsulation | the derived ESP overhead is the outer IP header plus 8 ESP plus 8 IV plus 16 ICV, and the ceiling aligns down to the 4-octet AEAD boundary less the 2-octet trailer |
| AC-3 | The same peer with UDP encapsulation in use | the overhead is 8 octets greater and the ceiling 8 lower |
| AC-4 | A tunnel whose interface MTU exceeds its ceiling | the verdict is oversized, the payload names the excess in octets, and a command setting the recommended value appears in the remediation list |
| AC-5 | A tunnel whose interface MTU is within the safety margin of the ceiling | the verdict distinguishes "tight" from "oversized", and names the spare octets |
| AC-6 | A tunnel whose ceiling leaves no value at or above the inner family's minimum link MTU | no MTU is recommended, the payload says so explicitly, and no command for that interface appears in the remediation list |
| AC-7 | A peer that answers no probe at any size | that peer is reported unmeasurable and every other peer is still measured |
| AC-8 | A path where a router reports an MTU | the reported value is confirmed on the wire, meaning the size passes and one octet more fails, before it is reported as the answer |
| AC-9 | A path where ICMP is filtered entirely | the candidate ladder and then a bisection find the value, and the payload says the answer came from a search rather than from a report |
| AC-10 | A probe is lost but the path is not limited at that size | the bounds are not moved on a single silence; the size is retried to the configured probe count before it is believed |
| AC-11 | `force` is given | the kernel's cached PMTU is bypassed, and a deliberately poisoned cache value does not appear in the answer |
| AC-12 | The reference address answers with the full interface MTU while the peers measure lower | the payload says the access circuit is not clamped and that lowering the underlay interface would cost octets on every other destination |
| AC-13 | The reference address measures the same as the peers | the payload says the whole circuit is clamped and recommends the underlay change, naming that it matters for traffic sent outside a tunnel |
| AC-14 | No reference address answers | the underlay advice is reported as undecidable rather than guessed |
| AC-15 | A peer negotiated the second of two configured ESP proposals | `show vpn ipsec sa` reports the negotiated transform, not the first configured one, and the MTU arithmetic uses the negotiated one |
| AC-16 | A child SA in transport mode rather than tunnel mode | the overhead reflects the mode, and a verdict is never computed with tunnel-mode arithmetic for a transport-mode SA |
| AC-17 | The peer is behind NAT, so the installed endpoint differs from the configured address | probes are sent to the installed address |
| AC-18 | The IKE component is not present in the build | `show mtu` reports that no IPsec inventory is registered, and `show mtu host <address>` still works |
| AC-19 | `show mtu \| json`, `\| yaml` and `\| table` | all three render the same payload, and no verdict is carried as a marker glued to another field's text |
| AC-20 | The box has non-zero outbound fragmentation counters | the payload reports them as a finding, with the absolute values, not as a rate |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show mtu` on a CPE with two oversized tunnels | CLI → inventory snapshot → probes → per-tunnel arithmetic → payload with verdicts and commands | `test-show-mtu-oversized-tunnels` |
| 2 | runs `show mtu host 1.1.1.1` to tell a clamped circuit from a clamped peer path | CLI → probe layer → single measurement | `test-show-mtu-host` |
| 3 | runs `show mtu force` after a stale cached PMTU misled them | CLI → bypass mode → wire measurement | `test-show-mtu-force-bypasses-cache` |
| 4 | pipes the output into a ticket as JSON | CLI → payload → pipe operators | `test-show-mtu-json` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestESPOverheadPerTransform` | `internal/component/mtu/cmd/overhead_test.go` | a table over every negotiable ESP transform, against hand-computed overheads (A-2) |  |
| `TestESPOverheadUnknownTransformRefuses` | `internal/component/mtu/cmd/overhead_test.go` | an unrecognised transform yields a refusal to advise, never a default | |
| `TestCeilingAlignsAndSubtractsTrailer` | `internal/component/mtu/cmd/arith_test.go` | the ceiling lands on the cipher boundary | |
| `TestRecommendedAbsentBelowMinimumLinkMTU` | `internal/component/mtu/cmd/arith_test.go` | no value is recommended below 1280 inner IPv6 | |
| `TestVerdictThresholds` | `internal/component/mtu/cmd/verdict_test.go` | each of the five verdicts at its exact boundary (A-5) | |
| `TestVerdictOrderNoUsableMTUBeforeOversized` | `internal/component/mtu/cmd/verdict_test.go` | a tunnel with no usable value is never classified as merely oversized | |
| `TestSearchDoesNotMoveBoundsOnSingleSilence` | `internal/component/mtu/cmd/search_test.go` | RFC 4821 Section 7.6.4 (AC-10) | |
| `TestSearchLadderThenBisect` | `internal/component/mtu/cmd/search_test.go` | candidates inside the bracket are tried before the number line | |
| `TestReportedMTUConfirmedOnTheWire` | `internal/component/mtu/cmd/search_test.go` | a reported value is accepted only when the size passes and one more fails | |
| `TestInventorySnapshotUnregisteredIsNotEmpty` | `internal/core/ipsecinventory/registry_test.go` | an unregistered inventory is distinguishable from zero tunnels | |
| `TestPeerInfoReportsNegotiatedTransform` | `internal/component/ike/engine/reconcile_test.go` | AC-15, the defect fix | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| measured path MTU | 68-65535 | 68 | 67 | 65536 |
| recommended tunnel MTU, inner IPv6 | 1280-65535 | 1280 | 1279 | N/A |
| recommended tunnel MTU, inner IPv4 | 576-65535 | 576 | 575 | N/A |
| probes per size | 1-3 | 3 | 0 | 4 |
| reported-MTU follows | 0-6 | 6 | N/A | 7 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-mtu-oversized-tunnels` | `test/plugin/*.ci` | an operator is told which tunnels are oversized and by how much | |
| `test-show-mtu-host` | `test/plugin/*.ci` | a single address is measured | |
| `test-show-mtu-force-bypasses-cache` | `test/plugin/*.ci` | a poisoned cache does not reach the answer | |
| `test-show-mtu-json` | `test/plugin/*.ci` | the payload renders through every pipe operator | |
| `test-show-mtu-no-ipsec-component` | `test/plugin/*.ci` | the absence of IKE is stated, not reported as zero tunnels | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `mtu-tunnel-sizing-strongswan` | `test/interop/scenarios/` | strongSwan | a tunnel to a real peer over a clamped path is measured, and the recommended MTU carries traffic where the current one fragments | |
| `mtu-negotiated-transform` | `test/interop/scenarios/` | strongSwan | a peer that negotiates the second configured proposal is reported as running it, and the derived overhead follows (A-1, AC-15) | |
| `mtu-nat-installed-endpoint` | `test/interop/scenarios/` | strongSwan behind NAT | probes go to the installed endpoint, not the configured one (A-3, AC-17) | |

## Files to Modify
- `internal/component/ike/engine/reconcile.go` - `PeerInfo` gains the encapsulation flag, the mode and the installed remote address, and reads the negotiated proposal rather than the configured group
- `internal/component/ike/engine/register.go` - registers the inventory snapshot at `init()`
- `internal/component/ike/cmd/show_ipsec.go` - the payload carries the corrected transform and the new fields
- `internal/component/sysctl/backend_linux.go` - an exported read, so the key-to-path mapping keeps one declaration
- `internal/component/plugin/all/all.go` - generated; regenerate with `./le repository generate`
- `docs/architecture/ike/ipsec-3-data-model.md` - the child SA's published state gains three fields
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by the `// Design:` header of `internal/component/ike/engine/reconcile.go` and `register.go`. The engine gains a registered inventory, and the snapshot it publishes changes what it reports about a negotiated proposal
- `docs/architecture/ike/ipsec-10-cli-diag.md` - declared by the `// Design:` header of `internal/component/ike/cmd/show_ipsec.go`. The `show vpn ipsec sa` payload gains three fields and corrects the transform it reports
- `docs/guide/command-catalogue.md` - the `MTU discover / path MTU` row: fill the Ze cell, and drop the `shell` backend and the `ping -M` note, both of which this spec makes false
- `docs/guide/command-reference.md` - the new command
- `docs/features.md` - a new user-facing feature
- `plan/journal/constant-reported-as-measured-state.md` - the 2026-09-11 row is resolved by AC-15

## Files to Create
- `internal/core/ipsecinventory/registry.go` - `Register` and a snapshot query returning an explicit not-registered outcome
- `internal/component/mtu/cmd/register.go` - the RPC and local-meta registration, copying `internal/component/ping/cmd/register.go`
- `internal/component/mtu/cmd/mtu.go` - the handler and the run
- `internal/component/mtu/cmd/overhead.go` - the per-transform ESP overhead derivation
- `internal/component/mtu/cmd/arith.go` - the ceiling, the recommended value and the MSS
- `internal/component/mtu/cmd/search.go` - the confirm-follow-ladder-bisect search
- `internal/component/mtu/cmd/verdict.go` - the tunnel classification
- `internal/component/mtu/cmd/state.go` - the cached PMTU, the fragmentation counters and `tcp_mtu_probing`
- `internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang` - the command grammar
- `internal/plugins/mtu-cmd/yang/register.go` and `embed.go` - generated
- `docs/architecture/diagnostics/path-mtu.md` - the owning page; no page covers MTU, PMTU or fragmentation today
- `test/interop/scenarios/mtu-tunnel-sizing-strongswan/`, `mtu-negotiated-transform/`, `mtu-nat-installed-endpoint/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang` for the command; a config leaf for the reference address |
| YANG validation constraints | Yes | the host argument is an `inet:ip-address` from `ze-types.yang`, so a malformed address is refused by the grammar |
| YANG custom validators | N-A | native types cover the address and the two bare keywords |
| CLI commands/flags | Yes | `internal/component/mtu/cmd/mtu.go`, registered, not added to a central list |
| CLI grammar (keyword before value) | Yes | `show mtu host <address>`; `force` and `detail` are bare keywords |
| Editor autocomplete | Yes | automatic for the typed leaf; the host argument takes no dynamic completion because an arbitrary address is legal |
| Functional test for new RPC/API | Yes | five `.ci` scenarios listed above |
| Pipe completeness | Yes | one payload through `ApplyPipes`, proven by AC-19 |
| Env var registration | Yes | the reference-address leaf needs its `ze.mtu.<leaf>` registration via `env.MustRegister()` |
| Doctor check for runtime dependencies | Yes | this module reads `/proc/sys` and the SNMP counters, which are runtime dependencies: a check in the owning package plus a code in `internal/core/diagnostic/codes.go`. The ICMP socket dependency is covered by `plan/spec-probe-do-not-fragment.md` and is not duplicated here |
| Prometheus counters/metrics | N-A | the command is operator-invoked and holds no continuous state; the fragmentation counters it reports are already collected by telemetry |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the reference-address leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, and the `docs/guide/command-catalogue.md` row that currently says `planned` with a `shell` backend |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, a new `mtu-cmd` YANG plugin |
| 6 | Has a user guide page? | Yes | `docs/architecture/diagnostics/path-mtu.md` is created; `docs/guide/ipsec.md` links to it from the tunnel sizing discussion |
| 7 | Wire format changed? | N-A | no message format changes; the probes are ICMP echo built by the existing primitive |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` if the phase 7 PMTU row moves; RFC 1191, 8201, 4821 and 8899 are cited for the algorithm's shape but not enrolled, per the owner decision recorded in `plan/spec-probe-do-not-fragment.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, three new interop scenarios |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`; no other daemon offers tunnel sizing advice from a live measurement |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` gains the new core leaf, and its Component Boundaries table today covers 15 of 44 component directories, so the mtu row is added rather than assumed |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md`: a new command, a new YANG plugin, a new registry and a doctor check |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-path-mtu-diagnostic.md` and name every result before implementation closes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` discusses tunnel sizing; verify its examples against what this command now reports |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `show mtu` reaches a handler
   - Tests: `TestShowMTUReachesTheHandler`, `TestShowMTUPayloadRendersAsJSON`, `TestIPsecInventoryRegisteredByIKE`
   - Files: `ze-mtu-cmd.yang`, `internal/component/mtu/cmd/register.go`, a stub handler, `internal/core/ipsecinventory/registry.go`, and the regenerated composition root
   - Verify: the command exists and returns a stub payload; the wiring test fails on content, not on reachability
2. **Phase: the IKE boundary and the negotiated-transform fix**
   - Tests: `TestPeerInfoReportsNegotiatedTransform`, `TestInventorySnapshotUnregisteredIsNotEmpty`, the `mtu-negotiated-transform` scenario
   - Files: `reconcile.go`, `register.go`, `show_ipsec.go`
   - Verify: A-1 is validated by negotiating the second of two proposals, not asserted. This phase stands alone and is committed on its own: it repairs a defect `show vpn ipsec sa` has today
3. **Phase: the arithmetic**
   - Tests: `TestESPOverheadPerTransform`, `TestESPOverheadUnknownTransformRefuses`, `TestCeilingAlignsAndSubtractsTrailer`, `TestRecommendedAbsentBelowMinimumLinkMTU`
   - Files: `overhead.go`, `arith.go`
   - Verify: A-2 is validated by the table over every negotiable transform
4. **Phase: the search**
   - Tests: `TestSearchLadderThenBisect`, `TestReportedMTUConfirmedOnTheWire`, `TestSearchDoesNotMoveBoundsOnSingleSilence`
   - Files: `search.go`
   - Verify: the RFC 4821 congestion rule and the RFC 8899 probe count are each pinned by a test that fails without them
5. **Phase: verdicts and local state**
   - Tests: `TestVerdictThresholds`, `TestVerdictOrderNoUsableMTUBeforeOversized`
   - Files: `verdict.go`, `state.go`, the exported sysctl read
   - Verify: each threshold is pinned at its exact boundary
6. **Phase: the payload, the output and the pages**
   - Tests: the five `.ci` scenarios, `mtu-tunnel-sizing-strongswan`, `mtu-nat-installed-endpoint`
   - Files: `mtu.go`, `docs/architecture/diagnostics/path-mtu.md`, and every row named at Documentation row 16
   - Verify: A-3 and A-4 are validated; the command-catalogue row no longer says `shell`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The overhead is derived from the negotiated transform in every path, including the rekeyed one. No arithmetic reads a configured value where an installed one exists |
| Naming | Payload keys are kebab-case. The verdict is a field with a typed value, never a marker glued to a name |
| Data flow | The mtu module imports neither the IKE engine nor the iface backend. Every cross-component read goes through a registry or a dispatch call |
| Rule: `ai/rules/principles.md` | An unregistered inventory, a peer with no tunnels, and a peer whose tunnel is down are three outcomes with three names. No zero is an answer |
| Rule: `ai/rules/evidence.md` | Every figure the payload calls measured was measured. A value assumed from another peer's path is marked as assumed wherever it appears |
| Rule: `ai/rules/cli.md` | `| json`, `| yaml` and `| table` render the same payload, and the remediation commands are a field rather than pre-formatted text |

## Review Gate

<!-- Filled by /ze-review at implementation time, per .claude/rules/planning.md. -->

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The module is removable | dropping its blank import from the composition root still builds and `show mtu` disappears |
| No import of the IKE engine | `grep -rn "ike/engine" internal/component/mtu/` returns nothing |
| The transform is negotiated, not configured | the `mtu-negotiated-transform` interop scenario goes red when the fix is reverted |
| The catalogue row is true | `grep -n "path MTU" docs/guide/command-catalogue.md` shows the Ze cell filled and no `shell` backend |
| Every verdict boundary is pinned | `go test -run TestVerdictThresholds ./internal/component/mtu/...` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The host argument is operator-supplied and reaches a socket; it is a typed address from the grammar, never a string passed onward |
| Untrusted input | A reported next-hop MTU comes from the network and drives printed advice. It is bounded by family and confirmed on the wire before it reaches a recommendation |
| Resource exhaustion | The probe budget, the follow count and the ladder are each bounded, so a hostile path cannot extend a run indefinitely |
| Error leakage | The payload names peer addresses and interface names, which an operator already sees; it carries no key material and no SPI beyond what `show vpn ipsec sa` already publishes |
| Authorization failing open | The command is read-only by construction. It must hold no code path that writes configuration, and the review checks that no write helper is reachable from the handler |

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
| Its own component plus YANG plugin, taking IPsec state through a registered snapshot | extend `internal/component/ike/cmd/`, which is where the state already is | the owner requires a removable module (2026-09-11). The IKE placement was simpler and would have made the feature inseparable from the IKE component |
| A registered synchronous snapshot in a core leaf | subscribe to the existing `child-up` and `child-down` events | the bus has no replay, so a module started after a tunnel came up would report no tunnels: a silently-wrong zero, which `ai/rules/principles.md` bans outright |
| The overhead is derived from the negotiated transform | hardcode AES-GCM-128 as the Python does, and refuse to advise otherwise | Ze is the IKE daemon and knows what was negotiated. The refusal path exists in the Python only because it cannot see it, and removing that limitation is most of the value of porting rather than copying |
| The 31-value candidate ladder is kept | RFC 1191's eleven plateaus, or ladder-then-bisect | the ladder was tuned on real degraded customer paths, and RFC 1191 Section 7 calls its own table "an implementation suggestion, NOT a specification or requirement". The RFC's eleven values step from 1492 straight to 1006 and would land far below the truth on the clamped paths this tool exists for. Owner decision, 2026-09-11 |
| The module reads the engine's per-SA PMTU and never writes it | keep an independent value, or write measurements back | writing back breaks the read-only promise that makes the command safe on live customer equipment. Keeping a second value means the daemon and the diagnostic can disagree with nothing reconciling them. Reading it makes disagreement itself a reportable finding. Owner decision, 2026-09-11 |
| ICMP echo now, the IKE-padded probe later | build both, or build only the IKE-padded probe | `show mtu host <address>` and the reference measurement both need ICMP regardless, so the IKE probe removes no prober. It is homed in `plan/spec-ike-padded-path-probe.md` so it is scheduled rather than forgotten. Owner decision, 2026-09-11 |

## Known Limitations

- The measurement is ICMP, so a path that treats UDP or ESP differently is not measured and the figures are optimistic. The payload says so. `plan/spec-ike-padded-path-probe.md` removes this for a peer with a live IKE SA.
- The configured interface MTU and the kernel one are reported separately because no symbol today reports both sides; reconciling them is `internal/component/iface`'s work, not this module's.
- A policy-based SA with no bound interface has nothing to size and is reported as such rather than measured.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The enforcing code here is the search and the floors: RFC 4821 Section 7.6.4 above
the rule that a single silent probe does not move the bounds, RFC 8899 Section 5.1.3
above the probe count, RFC 8200 Section 5 above the 1280 floor, and RFC 4301
Section 8.2.2 above the read of the engine's aged per-SA PMTU.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

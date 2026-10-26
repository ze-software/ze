# Spec: rfc4861-ze-owned-must-gaps

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Ze ships Router Advertisement origination, Router Solicitation answering and VRRP unsolicited Neighbor Advertisements, so shipped code must meet the RFC 4861 MUSTs of those capabilities. The rows below are `{gap}` in `rfc/short/rfc4861.md` |
| RFC gaps | RFC4861-6.1.1-2, RFC4861-4.6-1, RFC4861-4.4-3, RFC4861-6.2.1-1, RFC4861-6.2.1-8, RFC4861-6.2.4-1, RFC4861-6.2.5-2, RFC4861-7.2.6-1, RFC4861-9-7 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Close the Ze-owned RFC 4861 MUST gaps that the summary walk of 2026-10-26
recorded. Each is a defect in a capability Ze implements, so each is owed work.

| Row | Producer | What is missing |
|-----|----------|-----------------|
| RFC4861-6.1.1-2, RFC4861-4.6-1 | `readSolicitations` (`internal/plugins/iface/ra/sender_linux.go`), `rsReaderLoop` (`internal/component/l2tp/ppp/ra_linux.go`) | No validity check on a received Router Solicitation: IP Hop Limit 255, ICMP Code 0, ICMP length of 8 or more octets, every option length above zero, and no Source Link-layer Address option from the unspecified source. Every packet the ICMPv6 type filter passes is counted and answered |
| RFC4861-4.4-3 | VRRP group verify (`internal/plugins/vrrp/groups.go`), `BuildNA` | A multicast IPv6 virtual-address after the first is accepted and becomes an NA Target Address |
| RFC4861-7.2.6-1 | `announcer.runBurst` (`internal/plugins/vrrp/transport/announce.go`) | Three unsolicited NAs 100 ms apart, below RetransTimer (1,000 ms) |
| RFC4861-6.2.1-1 | router-advertisement YANG and `parseRAConfig` | AdvLinkMTU cannot be configured, so no MTU option is sent |
| RFC4861-6.2.1-8 | RA sender configuration | The advertised reachable-time, retransmit-timer and hop-limit are not applied to the router's own interface |
| RFC4861-6.2.4-1, RFC4861-6.2.5-2 | `reconcileRA`, `Sender.run` | Advertising does not follow the interface forwarding state: a host still advertises, and a router that stops forwarding keeps a non-zero Router Lifetime. Reconcile with design decision D-6 of the RA spec, which made forwarding a doctor warning |
| RFC4861-9-7 | prefix list YANG, `Sender.run` | No bound keeps one advertisement inside the link MTU, and Ze never splits one |

Read FRR (`zebra/rtadv.c`) and BIRD (`proto/radv/`) for each before design,
and quote the deciding function.

## Required Reading

- `rfc/short/rfc4861.md` and `rfc/full/rfc4861.txt`, Sections 4.4, 6.1.1, 6.2.1 to 6.2.5, 7.2.6 and 9
- `docs/features/interfaces.md` (Router Advertisement sender), `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`

## Current Behavior

Skeleton: the Task table states each producer's behavior as read on
2026-10-26. The design phase re-reads each producer before any decision.

**Source files read:** (by the 2026-10-26 summary walk; the design phase re-reads each)
- [ ] `internal/plugins/iface/ra/sender_linux.go` - `readSolicitations` reports every packet the ICMPv6 type 133 filter passes; `Sender.run` sends the configured advertisement on its schedule with no link MTU bound
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - `rsReaderLoop`, the subscriber-session reader, has the same shape as `readSolicitations`
- [ ] `internal/component/iface/config_ra.go` - `parseRAConfig` and `raValidate` read and bound the router configuration variables; no AdvLinkMTU leaf
- [ ] `internal/component/iface/reconcile_ra.go` - `reconcileRA` starts a sender for every enabled unit, whatever its forwarding state
- [ ] `internal/plugins/vrrp/groups.go` - requires only the first IPv6 virtual-address to be link-local
- [ ] `internal/plugins/vrrp/transport/announce.go` - `announcer.runBurst` repeats each unsolicited NA three times, 100 ms apart

**Behavior to preserve:**
- The Router Advertisement wire format and schedule the tagged RFC 4861 tests prove, and the recorded discrimination of each

**Behavior to change:**
- Each row of the Task table, as that row states

## Data Flow

Skeleton: config (`ze-iface-conf.yang`, VRRP YANG) to `reconcileRA` and the
VRRP group verify, then to the senders and readers named in the Task table.

### Entry Point
- Operator config: the `router-advertisement` container of an interface unit, and a VRRP group's IPv6 virtual-addresses
- The wire: a Router Solicitation (ICMPv6 type 133) received on an advertising interface or an L2TP subscriber session

### Transformation Path
1. `parseRAConfig` and `raValidate` turn the container into `raUnitConfig`; `reconcileRA` starts one sender per advertising unit
2. `Sender.run` builds each advertisement with `BuildRA` and sends it; `readSolicitations` or `rsReaderLoop` signals a solicitation to the sender
3. The VRRP group verify admits the virtual-addresses; `announcer.runBurst` sends the unsolicited NAs that `BuildNA` encodes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to iface component | config transaction, `reconcileRA` | [ ] |
| iface component to RA sender plugin | the sender factory `reconcileRA` calls | [ ] |
| Ze to the kernel | raw ICMPv6 socket (`openRASocket`), sysctls for the router's own ND variables | [ ] |

### Integration Points
- `reconcileRA` - where forwarding state and AdvLinkMTU must be read
- `readSolicitations`, `rsReaderLoop` - where the validity checks go

## Wiring Test

Skeleton: each AC is proven through the daemon entry point, the RA rows by
the QEMU integration tests in `internal/plugins/iface/ra/`.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Router Solicitation on an advertising interface | → | `readSolicitations` validity checks | `TestRFC4861SolicitationReaderDiscardsInvalid` (planned) |
| `router-advertisement` config with forwarding off | → | `reconcileRA` | `TestRFC4861AdvertisingFollowsForwarding` (planned) |
| VRRP group with an IPv6 virtual-address | → | VRRP group verify, `announcer.runBurst` | `TestRFC4861VRRPRefusesMulticastTarget` (planned) |

## 🧪 TDD Test Plan

Skeleton: one positive and one negative tagged test per row, each with a
recorded discrimination. The names below are planned, not yet written.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4861SolicitationReaderDiscardsInvalid` | `internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go` | RFC4861-6.1.1-2, RFC4861-4.6-1: a solicitation with Hop Limit below 255, Code other than 0, length below 8, a zero-length option, or a Source Link-layer Address option from `::` is not reported; a valid one is | |
| `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid` | `internal/component/l2tp/ppp/rfc4861_solicitation_reader_linux_test.go` | The same five refusals and the valid case on the subscriber-session reader | |
| `TestRFC4861VRRPRefusesMulticastTarget` | `internal/plugins/vrrp/rfc4861_virtual_address_test.go` | RFC4861-4.4-3: a multicast IPv6 virtual-address is refused at verify; a unicast one is accepted | |
| `TestRFC4861UnsolicitedNASpacedByRetransTimer` | `internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go` | RFC4861-7.2.6-1: repeated unsolicited NAs are at least RetransTimer apart | |
| `TestRFC4861AdvLinkMTUConfigured` | `internal/component/iface/rfc4861_router_configuration_test.go` | RFC4861-6.2.1-1: AdvLinkMTU is configurable and reaches the MTU option | |
| `TestRFC4861RouterUsesItsAdvertisedVariables` | `internal/component/iface/rfc4861_router_configuration_test.go` | RFC4861-6.2.1-8: the advertised reachable-time, retransmit-timer and hop-limit are applied to the interface | |
| `TestRFC4861AdvertisingFollowsForwarding` | `internal/component/iface/rfc4861_router_configuration_test.go` | RFC4861-6.2.4-1, RFC4861-6.2.5-2: a non-forwarding interface sends no advertisement with a non-zero Router Lifetime | |
| `TestRFC4861AdvertisementInsideLinkMTU` | `internal/core/ndp/rfc4861_router_advertisement_test.go` | RFC4861-9-7: no advertisement exceeds the link MTU | |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `counter-counts-the-wrong-packets.md` | 2026-10-26 | RS readers count every ICMPv6 type 133 packet as a valid solicitation |
| `unwired-feature.md` | 2026-10-26 | RA sender: AdvLinkMTU, own ND variables, forwarding state, link MTU bound |
| `unwired-feature.md` | 2026-10-26 | VRRP unsolicited NA spacing and multicast target |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | Each row in the Task table is implemented, its `{gap}` removed, and it carries positive and negative tagged tests with discrimination records |

## Files to Modify

Skeleton: the producers the Task table names, as read on 2026-10-26.

- `internal/plugins/iface/ra/sender_linux.go` - Router Solicitation validity checks in `readSolicitations`; link MTU bound in `Sender.run`
- `internal/component/l2tp/ppp/ra_linux.go` - the same validity checks in `rsReaderLoop`
- `internal/component/iface/config_ra.go`, `internal/component/iface/yang/ze-iface-conf.yang` - AdvLinkMTU leaf and its parse
- `internal/component/iface/reconcile_ra.go` - advertising follows the forwarding state
- `internal/core/ndp/ra.go` - MTU option in `BuildRA`
- `internal/plugins/vrrp/groups.go` - refuse a multicast IPv6 virtual-address
- `internal/plugins/vrrp/transport/announce.go` - space repeated unsolicited NAs by RetransTimer
- `rfc/short/rfc4861.md` - remove each `{gap}` the work closes
- `docs/features/interfaces.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - the changed behavior

## Implementation Steps

Skeleton: the design phase orders and confirms these.

1. **Phase: Wiring** -- the AdvLinkMTU leaf through `parseRAConfig` to the sender, and the forwarding state into `reconcileRA`
   - Tests: `TestRFC4861AdvLinkMTUConfigured`, `TestRFC4861AdvertisingFollowsForwarding`
   - Verify: the leaf reaches the sender, and a non-forwarding unit is seen by the reconciler
2. **Phase: Router Solicitation validity** -- both readers
   - Tests: `TestRFC4861SolicitationReaderDiscardsInvalid`, `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid`
3. **Phase: VRRP** -- multicast target refusal and NA spacing
   - Tests: `TestRFC4861VRRPRefusesMulticastTarget`, `TestRFC4861UnsolicitedNASpacedByRetransTimer`
4. **Phase: Router variables and MTU** -- own ND variables, MTU option, link MTU bound
   - Tests: `TestRFC4861RouterUsesItsAdvertisedVariables`, `TestRFC4861AdvertisementInsideLinkMTU`
5. **Phase: Records** -- `./le rfc record` for each tagged unit, the guest ones with `kernel tmp/kernel/build/vmlinuz`

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] FRR `zebra/rtadv.c` and BIRD `proto/radv/` read for each row, deciding function quoted
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1 demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: `./le test unit plugins`, `./le test qemu all` for the RA integration tests, and `./le verify changed` over the closing commits
- [ ] Every owned journal row resolved

### Verification
- [ ] `./le test unit plugins` green
- [ ] `./le verify changed` green over the closing commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** remove the spec

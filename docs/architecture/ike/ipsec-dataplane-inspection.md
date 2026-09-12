# IPsec Dataplane Inspection

Every other IPsec surface reports what the IKE engine believes at install time.
This one reads the kernel back: the Security Association Database, the Security
Policy Database, and the difference between the two beliefs.

The read path uses the `Dataplane` backend that installs Child SAs. Linux XFRM
implements inspection. VPP and noop return `ErrNotSupported` from the public
SAD and SPD reads. The engine-belief siblings are documented in
[`ipsec-10-cli-diag.md`](ipsec-10-cli-diag.md); the install side is
[`ipsec-8-ikev2-child-xfrm.md`](ipsec-8-ikev2-child-xfrm.md).
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- ListSAs, ListPolicies -->
<!-- source: internal/component/ike/dataplane/vpp.go -- ListSAs -->
<!-- source: internal/component/ike/dataplane/vpp_policy.go -- ListPolicies -->
<!-- source: internal/component/ike/dataplane/noop.go -- ListSAs, ListPolicies -->

| Concern | File |
|---------|------|
| `show vpn ipsec dataplane sa / policy / drift` | `internal/component/ike/cmd/show_dataplane.go` |
| Kernel record to report mapping | `internal/component/ike/dataplane/xfrm_linux.go` |
| Reported shapes | `internal/component/ike/dataplane/dataplane.go` |
| Health signal | `internal/component/ike/engine/health_drift.go` |

<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- ListSAs, saInfoFromState, ListPolicies, policyInfoFromKernel -->
`PolicyInfo.Action` reports all three dispositions of RFC 4301 Section 4.4.1.
A kernel `block` policy is read back as DISCARD on its own action rather than
on an empty template list, because the kernel accepts and ignores a template
beside a block policy: reading such a policy as a protect entry would tell the
operator their traffic is encrypted while it is being dropped.

<!-- source: internal/component/ike/dataplane/dataplane.go -- SAInfo, PolicyInfo -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- policyInfoFromKernel -->

## Decision: a failed read is an error, never an empty table

An empty list answers "is my tunnel programmed?" with "no". That answer is wrong
whenever the truth is "nobody asked the kernel". Three causes are distinct and an
operator acts differently on each, so each one gets its own message and none of
them collapses into an empty result.

| Cause | What the operator sees |
|-------|------------------------|
| No backend is loaded | the ike component never started, or no backend registered |
| `ErrNotSupported` | the active backend cannot enumerate this table |
| `EPERM` or `EACCES` | the read needs CAP_NET_ADMIN and this process does not have it |

<!-- source: internal/component/ike/cmd/show_dataplane.go -- activeDataplane, dataplaneReadError -->

The same rule shapes the counter columns of `show vpn ipsec sa`. That command
reports engine belief and must keep working when the kernel cannot be read, so a
failed dump is recorded as "not known" and every counter renders as null. A zero
is a measurement; a null is the absence of one.

Counter and drift lookups use the SPI, destination, protocol, and XFRM interface
identifier. Another SA with the same SPI does not satisfy the lookup.
`SAIdentity` stores the destination as a normalized `netip.Addr`, so the join
accepts both Go IPv4 representations without allocating address strings.
`PeerSession.Info` derives the expected keys from the Child SA endpoints used by
`installChildSA`, including its ESP protocol and interface identifier. Configured
peer addresses can differ from those negotiated endpoints and cannot supply this
identity.

<!-- source: internal/component/ike/dataplane/dataplane.go -- SAIdentity, IdentityOf -->
<!-- source: internal/component/ike/engine/reconcile.go -- PeerInfo, Info, setChildSA -->
<!-- source: internal/component/ike/engine/child.go -- installChildSA -->

<!-- source: internal/component/ike/cmd/show_ipsec.go -- sadCounters, readSADCounters -->

## Decision: drift is compared in one direction only

Drift is an SA the engine counts as installed that the kernel SAD does not
hold. The opposite case is not drift: RFC 7296 Section 2.8 keeps the old and the
new Child SA alive together until the old one is deleted, so a rekey window
legitimately holds an SA the engine no longer names.

An unreadable SAD makes drift unknown, not clean. Health must report that it
cannot determine dataplane state rather than report healthy.

`ObserveDataplane` records a generation before reading peer belief and checks it
after the SAD dump. Child publication and peer-roster changes advance that
generation. Installation and removal also advance it and hold a writer count
until the backend operation returns, including failure and rollback. The count
permits concurrent peer actors and nested pair/half removals; parity alone would
mistake two writers for an idle dataplane.
A Child SA whose removal has started also remains inconclusive while it is still
published as the live pair. Publication of its replacement or removal closes that
interval; a successful reinstallation clears the flag on the same Child SA.

A changed generation or any outstanding writer makes the comparison unknown.
The drift CLI returns an error, health reports degraded with an unknown-state
reason, and the counter display keeps engine belief with null counters and
`counters-known: false`. Health and both dataplane gauges compare the peer map
from this same observation. Raw SA and policy dumps still read the backend
directly, so an operator can inspect old/new-SA coexistence during rekey.

<!-- source: internal/component/ike/engine/health_drift.go -- ObserveDataplane, beginDataplaneWrite, endDataplaneWrite, driftingPeers, driftingPeersFrom, driftDetail -->
<!-- source: internal/component/ike/engine/health.go -- checkIPsecHealth -->

## Decision: a gauge with no readable kernel publishes no series

`ze_ipsec_dataplane_sa_count{if_id}` reports how many SAs the kernel SAD holds
under each if_id. `ze_ipsec_dataplane_drift{peer}` reports 1 for a peer whose
believed Child SA the kernel does not hold, and 0 when its SAs are present.
Both are published by `IPsecMetrics.Update`, which runs on a 5 second
ticker.

A dataplane nobody could read publishes NO series, and every series an earlier
pass published is deleted. A drift of 0 there says "no drift" on the strength of
a question nobody asked, which is the false green this whole surface exists to
remove. Prometheus spells "unknown" as the absence of a series, so both metrics
are a `GaugeVec` even where one label value would do: a `Gauge` cannot be
deleted.
An inconclusive generation follows the same rule and clears both gauge families.

The same rule covers a label that stops appearing. A peer that leaves
`PeerInfoMap`, and an if_id the kernel no longer holds, each lose their series
rather than keep the value the last pass left there.

The dump is skipped when `ActiveTable()` is nil. A daemon that runs no IKE engine
has no belief to contradict, and it must not issue a netlink dump every 5 seconds
to learn that.

The label is `if_id`, not the `if-id` the JSON keys use. A Prometheus label name
matches `[a-zA-Z_][a-zA-Z0-9_]*`, so a hyphen is refused at registration and
takes the daemon down at startup rather than at a scrape.

<!-- source: internal/component/ike/engine/metrics.go -- publishDataplaneGauges, clearDataplaneGauges, setGaugeSeries -->

## Decision: the counters come from the kernel

The IKE engine never sees ESP payload, so a userspace count reports zero
forever. `BytesCurrent` and `PacketsCurrent` are what the kernel recorded on the
SA. `BytesHard` and `PacketsHard` are the lifetime ceilings, and zero means no
limit.
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- saInfoFromState, hardLimitFromKernel -->

## Decision: a policy says whether ze owns it

The kernel SPD holds every policy on the node, including ones another daemon or
the operator installed. `PolicyInfo` carries `Owner` and a separate `OwnerKnown`
flag. An unresolved owner renders as `unknown` with `owner-known: false`, which
also distinguishes it from a peer named `unknown`.
<!-- source: internal/component/ike/cmd/show_dataplane.go -- policyInfoToMap, policyOwnerName -->

Readback preserves kernel port masks, including policies Ze did not install.
The policy command reports `src-port` and `dst-port` as `any` for a zero mask,
or as a decimal port for mask `0xffff`, including `0` for exact port zero.
Partial masks use `<decimal-port>/0x<four-lowercase-hex-digits>`, such as
`4608/0xff00`. Both the structured response and its public renderings keep this
spelling, so a masked selector cannot appear as one exact port.
<!-- source: internal/component/ike/cmd/show_dataplane.go -- portString, policyInfoToMap -->

Wildcard prefixes use the same normalized identity at installation and lookup.
<!-- source: internal/component/ike/dataplane/policy_owner.go -- policyKey, cidrKey -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- policyInfoFromKernel -->

## Trap: SPI zero is refused as a selector

RFC 4303 Section 2.1 reserves SPI value zero for local use and forbids sending
it on the wire, so zero is never a real SA. Accepting it as "every SPI" would
make a typo look like a successful full dump.

<!-- source: internal/component/ike/cmd/show_dataplane.go -- dataplaneSPISelector -->

## Trap: an empty kernel makes a read-only test vacuous

A dump command passes on an empty kernel with its body deleted. Every kernel
assertion names an SPI or an address the test itself installed, and asserts a
transition (present, then absent) rather than a state.

## Trap: the records are sorted, not the rendered rows

The kernel dump order is not stable between calls. The SAD handler sorts records
by SPI before mapping them. The SPD handler retains kernel order. Each handler
returns `plugin.Map`; `command.ApplyPipes` renders the table and the `| json` form.
<!-- source: internal/component/ike/cmd/show_dataplane.go -- handleShowVPNIPsecDataplaneSA, handleShowVPNIPsecDataplanePolicy -->

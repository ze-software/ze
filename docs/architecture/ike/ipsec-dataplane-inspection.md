# IPsec Dataplane Inspection

Every other IPsec surface reports what the IKE engine believes at install time.
This one reads the kernel back: the Security Association Database, the Security
Policy Database, and the difference between the two beliefs.

The read path is the same `Dataplane` backend that installs Child SAs, so the
Linux XFRM backend and the VPP backend answer the same three questions. The
engine-belief siblings are documented in
[`ipsec-10-cli-diag.md`](ipsec-10-cli-diag.md); the install side is
[`ipsec-8-ikev2-child-xfrm.md`](ipsec-8-ikev2-child-xfrm.md).

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

<!-- source: internal/component/ike/cmd/show_ipsec.go -- sadCounters, readSADCounters -->

## Decision: drift is compared in one direction only

Drift is an SPI the engine counts as installed that the kernel SAD does not
hold. The opposite case is not drift: RFC 7296 Section 2.8 keeps the old and the
new Child SA alive together until the old one is deleted, so a rekey window
legitimately holds an SPI the engine no longer names.

`driftingPeers` returns the peer names AND a `known` flag. When the kernel could
not be read the name list is empty and means nothing. A caller that read that as
"no drift" would report healthy on the strength of a question nobody asked.

`driftingPeersFrom` is the comparison over an SA list the caller already holds,
and `driftingPeers` is the wrapper that reads the SAD for it. The split exists so
one kernel dump answers both readers: the health check reaches it through the
wrapper, and the metrics pass hands the same slice it published the SA count
from. Two dumps for one pass are two answers to one question, and they disagree
across a rekey.

<!-- source: internal/component/ike/engine/health_drift.go -- driftingPeers, driftingPeersFrom, driftDetail -->

## Decision: a gauge with no readable kernel publishes no series

`ze_ipsec_dataplane_sa_count{if_id}` reports how many SAs the kernel SAD holds
under each if_id. `ze_ipsec_dataplane_drift{peer}` reports 1 for a peer whose
believed Child SA SPI the kernel does not hold, and 0 for a peer whose SPIs are
present. Both are published by `IPsecMetrics.Update`, which runs on a 5 second
ticker.

A dataplane nobody could read publishes NO series, and every series an earlier
pass published is deleted. A drift of 0 there says "no drift" on the strength of
a question nobody asked, which is the false green this whole surface exists to
remove. Prometheus spells "unknown" as the absence of a series, so both metrics
are a `GaugeVec` even where one label value would do: a `Gauge` cannot be
deleted.

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

## Decision: a policy says whether ze owns it

The kernel SPD holds every policy on the node, including ones another daemon or
the operator installed. `PolicyInfo` carries `Owner` and a separate `OwnerKnown`
flag, and a renderer must print "not ours" rather than an empty cell. A blank
owner reads as "unowned" when the truth is "not ze".

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

The kernel dump order is not stable between calls. The handlers sort the records
by SPI before mapping them, so the order is defined by the SPI value and not by
a rendered string. Each handler returns `plugin.Map` and formats nothing;
`command.ApplyPipes` renders the table and the `| json` form.

# DDoS Detection and Auto-Mitigation

Ze can automatically detect volumetric DDoS attacks on its interfaces, mitigate
them locally or upstream, and report incidents to the Flowtriq cloud.

## Overview

The system has four independent components, each enabled separately:

| Component | Plugin | What it does |
|-----------|--------|-------------|
| **Detector** | `ddos-detect` | Watches interface rates, learns a baseline, triggers when traffic exceeds the dynamic threshold |
| **Local responder** | `ddos-local` | Installs an nftables drop rule on the host when an attack is detected |
| **FlowSpec responder** | `ddos-flowspec` | Announces a surgical FlowSpec (or RTBH) rule upstream via BGP |
| **Flowtriq reporter** | `ddos-flowtriq` | Reports incidents to the Flowtriq cloud API for dashboarding and server-driven mitigation |

Each component subscribes to the detector's events independently. You can run
any combination: detector alone (monitoring only), detector + local (on-host
protection), detector + flowspec (upstream mitigation), or all four.

## Quick Start

### Alert-only mode (monitoring, no mitigation)

```
ddos {
    detect {
        enabled true
    }
    local {
        response-level alert
    }
}
```

The detector watches interface rates and emits events. The local responder logs
what it would do but installs no rules. Check the logs for
`ddos-local: alert mode, would mitigate`.

### Local mitigation mode

```
ddos {
    detect {
        enabled true
        policy {
            default-action deny
            rule 10.0.0.0/8 { action allow; match destination; scope mitigation }
            rule 192.168.0.0/16 { action allow; match destination; scope mitigation }
        }
    }
    local {
        response-level enforce
    }
}
traffic {
    usage {
        enabled true
        track-ip true
        interfaces {
            interface eth0 { enabled true }
        }
    }
}
```

On attack detection the detector identifies the victim by querying on-box flow
data, then emits the target so the local responder installs an nftables drop
rule for it. The `ddos detect policy` `allow` rules prevent auto-mitigation from
ever blocking management, DNS, or other critical prefixes.

**Target identification needs a flow source.** The detector reads the attacked
destination from `traffic-usage` (with `track-ip` enabled on the exposed
interfaces). Without a reachable flow source the detector still fires but emits a
generic signal with no target, and the local responder cannot install a targeted
rule. The coarse rule matches the destination prefix; characterization (below)
then narrows it in place to the attack's protocol, ports, and TCP flags.
<!-- source: internal/plugins/ddos/detect/characterize.go -- characterizeTarget fills the victim DstPrefix from traffic-usage -->

The drop rule is removed automatically when the attack stops (the detector
observes RxPps falling below threshold, which works because nftables drops occur
after the kernel NIC RX counter).

### Attack characterization (Stage 2)

With `flow-export` conntrack enabled, the detector runs a second, finer pass on
the trigger: it queries the flow-export recent-flow ring for flows to the victim,
classifies the attack, and emits a characterized signal so responders install a
surgical rule and then narrow the local drop in place:

- **reflection** -- UDP with a dominant amplifier source port (DNS 53, NTP 123,
  SNMP 161, CLDAP 389, SSDP 1900, Memcached 11211, ...): matches proto + source port.
- **syn-flood** -- TCP dominated by half-open conntrack states: matches proto + SYN flag.
- **icmp-flood** -- ICMP dominant: matches proto.
- **udp-flood** -- UDP dominant without a reflector: matches proto (and dominant dst port).
- **generic-flood** -- no clear signature (includes fragment floods, which have no
  on-box conntrack signal): coarse destination-prefix drop.

The recent-flow ring is IPv4 and IPv6 capable, so IPv6 victims (invisible to the
IPv4-only `traffic-usage` map) are resolved from the ring. Characterization runs
off the detection hot path with a bounded timeout and degrades to the coarse
target when no flow source is reachable.
<!-- source: internal/plugins/ddos/detect/characterize.go -- classifyFlows: family + narrowest VectorTuple from the recent-flow ring -->

**Characterization refreshes the ring on demand.** The recent-flow ring is
normally refreshed only at each conntrack dump (every `flow-export`
`active-timeout` seconds, default 60s). To avoid classifying against a pre-attack
ring, detection forces the refresh: `ddos-detect` emits `AttackDetected`,
`flow-export` responds with an immediate out-of-band conntrack dump, and the
classifier then polls the ring for up to `characterize-timeout` until it reflects
the attack. So a just-started flood is characterized promptly regardless of
`active-timeout`; if the ring still never yields discriminating flows within the
budget, characterization falls back to the coarse destination drop (still
protective, just not narrowed). Lowering `active-timeout` is no longer required
for responsive characterization (it still governs NetFlow/IPFIX export cadence).
<!-- source: internal/plugins/flowexport/register.go -- AttackDetected subscription forces conntrackWorker.Refresh -->
<!-- source: internal/plugins/ddos/detect/characterize.go -- awaitClassifiableFlows polls the ring within characterize-timeout -->

Inspect the ring directly:

```
show flow recent                     # all recent conntrack flows (bounded to recent-flow-ring)
show flow recent dst 203.0.113.42    # flows to a destination prefix or address
```
<!-- source: internal/plugins/flowexport/cmd_show.go -- handleShowFlowRecent -->

Each signal carries a graded **severity** from the peak-to-threshold ratio:
`medium` (>=1x), `high` (>=2x), `critical` (>=5x). The FlowSpec responder can use
`critical` to engage an immediate blackhole (below).
<!-- source: internal/core/ddosevent/event.go -- GradeSeverity -->

### Upstream FlowSpec mitigation

```
ddos {
    detect {
        enabled true
        policy {
            default-action deny
            rule 10.0.0.0/8 { action allow; match destination; scope mitigation }
        }
    }
    flowspec {
        response-level enforce
        action rate-limit
        rate-limit-bytes 1000000
        hold-down 300
        probe-interval 60
    }
}
```

Once the attack is characterized, a surgical BGP FlowSpec rule is announced to the
configured upstream peer, matching the attack's protocol, ports, and flags. The
FlowSpec responder waits for characterization by default rather than acting on the
fast signal: announcing upstream blinds the box behind the filter, so the rule
must be right the first time. The `action` is mandatory (no default, because
neither choice is universally safe): `rate-limit` announces an RFC 8955
traffic-rate of `rate-limit-bytes` bytes/sec (preserving legitimate traffic up to
that rate), while `discard` drops the whole characterized flow. A `rate-limit-bytes`
of `0` is valid and is equivalent to `discard`; `action rate-limit` without a
`rate-limit-bytes` is a configuration error (no rate is fabricated).
<!-- source: internal/plugins/ddos/flowspec/config.go -- Validate: action mandatory; rate-limit requires rate-limit-bytes; 0 == discard -->
<!-- source: internal/plugins/ddos/flowspec/responder.go -- renderFlowspecCommand emits update text extended-community [rate-limit:<bps>] nlri <fam> add <components> -->


**Blackhole fallback.** With `blackhole-fallback true`, a `critical`-severity
attack (peak >= 5x threshold) engages an immediate upstream `discard` on the fast
signal without waiting for characterization -- the escape hatch when local
filtering cannot hold the flood.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- onCharacterized announces; onDetected blackholes only on critical + policy -->

**Clearing under FlowSpec mitigation:** once the upstream drops the attack
traffic, Ze's local sensors go blind (the traffic never arrives). The responder
uses a *leak-probe*: after the hold-down period, it periodically narrows the
FlowSpec rule to a small non-zero rate (`probe-rate`, default 1 Mbps) and
observes whether the leaked traffic saturates that rate. If yes, the flood is
still arriving and the rule is re-tightened with exponential backoff. If no, the
attack is over and the rule is withdrawn.

The probe reads the detector's `AttackOngoing` events, which carry the bandwidth
measured behind the filter, so the probe compares a real observation against
`probe-rate` rather than a synthetic tick. The responder ignores the detector's
clear while it mitigates, because the probe holds the trustworthy signal.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- onOngoing feeds probeTick with AttackOngoing.CurrentBps -->

**Withdraw paths.** A FlowSpec announce is withdrawn when any of these happens:

| Trigger | What it means |
|---------|---------------|
| The leak-probe sees no leak at `probe-rate` | The flood has stopped |
| `max-mitigation-duration` expires | The wall-clock cap fires even when the attack went quiet and `AttackOngoing` stopped arriving. Checked once a second; `0` means no cap |
| Characterization carries the exemption decision | The traffic policy exempts this attack from the mitigation action, so a blackhole fallback installed on the fast signal is taken back |
| Characterization reclassifies the victim as local | The box is mitigating it on-host, so the upstream rule must not stand for it |

The last two follow the detector's contract: the characterized event is
authoritative, so a responder withdraws whatever the fast `AttackDetected` path
installed. The reclassification case is real rather than theoretical, because
characterization re-derives direction from the narrowed victim: a /24 that looked
remote can narrow to a box-owned /32 after the blackhole fallback is already
announced.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- onCharacterized, enforceMaxDuration -->
<!-- source: internal/plugins/ddos/detect/characterize.go -- direction re-classified from the narrowed victim -->

### Flowtriq cloud reporting

```
ddos {
    flowtriq {
        enabled true
        api-key YOUR_API_KEY
        node-uuid YOUR_NODE_UUID
    }
}
```

Incidents are reported to the Flowtriq cloud API in real time: open on detection,
update every few seconds with current rates, resolve when the attack ends. The
Flowtriq dashboard provides historical analysis, alerting, and optional
server-driven mitigation commands.

## Configuration Reference

### ddos detect (detector)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `enabled` | `false` | bool | Enable the detector |
| `check-interval` | `1` | 1-3600 s | Seconds between detection evaluations |
| `confirm-duration` | `3` | 0-3600 | Consecutive ticks above threshold before triggering (0 = immediate) |
| `clear-consecutive-checks` | `10` | 1-100 | Consecutive ticks below threshold before clearing |
| `baseline-window` | `300` | 10-86400 | Rolling baseline window in samples (~seconds at 1 Hz) |
| `threshold-multiplier` | `3.00` | 1.00-100.00 | Baseline p99 multiplier for the dynamic PPS threshold |
| `absolute-floor` | `5000` | 1+ PPS | Minimum PPS threshold regardless of baseline |
| `startup-grace` | `90` | 0-3600 s | Seconds after startup where only an extreme spike (>5x floor) or an armed bandwidth trigger fires |
| `bps-trigger-enable` | `true` | bool | Enable the bandwidth (BPS) trigger alongside the PPS threshold |
| `bps-threshold-multiplier` | `3.00` | 1.00-100.00 | Baseline p99 multiplier for the bandwidth trigger |
| `bps-floor` | `50000000` | 1+ bits/s | Minimum bandwidth (bits/s) below which the BPS trigger is inert (default 50 Mbps) |
| `characterize-enable` | `true` | bool | Run Stage-2 flow characterization (family + narrowest vector, `AttackCharacterized`) |
| `top-n-sources` | `10` | 1-100 | Max attacker source addresses ranked into TopSources |
| `characterize-window` | `10` | 1-60 s | Seconds of recent flows to consider (timestamp-less flows always kept) |
| `characterize-timeout` | `2000` | 50-5000 ms | Budget for the on-trigger traffic-usage / flow-recent queries |
| `entropy-threshold` | `2.00` | 0.00-16.00 bits | Source-entropy at/above which an attack is logged as distributed/spoofed |
| `policy` | (default-action `deny`) | container | Allow/deny traffic policy (see [Traffic policy](#traffic-policy)) that exempts or defends prefixes; replaces the old per-responder allowlists |

**How the threshold works:**
```
threshold = max(baseline_p99 * threshold-multiplier, absolute-floor)
```

The baseline is a rolling window of the last `baseline-window` non-attack
samples. The p99 is recalculated every 10 samples. Samples collected during an
active attack or above the current threshold are excluded from the baseline to
prevent poisoning.

That exclusion has one damped exception, and without it the detector latches: a
permanent rise in offered load (a new customer, a migrated service) would hold
the threshold under the new level for good. One sample in every 3600 consecutive
above-threshold samples enters the window, which is one per hour at the default
`check-interval`. A 300-sample window then follows a sustained new level after 4
to 13 hours, and any flood shorter than that leaves the threshold where it was.

**Bandwidth (BPS) trigger:** amplification floods (NTP, memcached, CLDAP) are
low-PPS but very high bandwidth, so a packet-rate threshold alone misses them. A
parallel bandwidth baseline (same rolling window and poisoning guard, in bytes/s)
fires when the observed bandwidth exceeds `max(bps_p99 * bps-threshold-multiplier,
bps-floor)`. It only engages once the bandwidth baseline is warmed, and `bps-floor`
(expressed in bits/s for operator convenience) keeps it inert below that bandwidth
so legitimate high-throughput bursts are not flagged. Set `bps-trigger-enable false`
to disable just the bandwidth path.
<!-- source: internal/plugins/ddos/detect/detector.go -- applyTick: bpsAbove = enable && baselineBps.Ready() && maxBps > baselineBps.Threshold() -->

**Baseline persistence:** the detector saves both baselines to
`<config-dir>/state/ddos-detect-baseline.json` on shutdown/reconfigure and
periodically, and restores them on startup, so a restart or config change resumes
detection without re-warming over `baseline-window` (a stale, too-few-sample, or
corrupt file is rejected and the baseline warms fresh).
<!-- source: internal/plugins/ddos/detect/persist.go -- saveBaselines/loadBaselines; baseline.restore version + min-sample + sanity guards -->

**Incident confidence:** on characterization the detector computes a 0-100
confidence from the peak/threshold ratio, family specificity, and source spread.
It is stored on the incident (shown in `show ddos incidents`), reported to the
Flowtriq dashboard, and can gate the responders (see `confidence-min` below).
<!-- source: internal/core/ddosevent/event.go -- GradeConfidence; set in internal/plugins/ddos/detect/characterize.go -->

Confidence is computed at attack start (ze characterizes once, early), so attack
duration is not a factor.

### ddos local (local responder)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `response-level` | `alert` | alert, enforce | `alert` logs only; `enforce` installs nft drop rules |
| `max-mitigation-duration` | `3600` | 0-86400 s | Parsed and range-checked, but the local responder does not act on it yet. An on-host drop is removed when the attack clears, not on a timer. The FlowSpec responder does enforce its own cap |
| `confidence-min` | `0` | 0-100 | Minimum incident confidence to mitigate from a characterized attack (`0` = no gate). Note: the coarse drop on the fast `AttackDetected` carries no confidence, so this only gates the in-place narrowing on the characterized path |
| `forward-mitigation` | `false` | bool | Also drop a remote (transit) victim's traffic on the netfilter FORWARD hook to protect a downstream host. Default guards only local (box-owned) victims on INPUT and leaves remote victims to flowspec (see [Direction](#direction-local-vs-remote)) |

### ddos flowspec (FlowSpec/RTBH responder)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `response-level` | `alert` | alert, enforce | `alert` logs only; `enforce` announces FlowSpec |
| `action` | (required) | rate-limit, discard | Mandatory FlowSpec traffic-action (no default). `rate-limit` needs `rate-limit-bytes` and preserves legitimate traffic up to that rate; `discard` drops the flow |
| `rate-limit-bytes` | (none) | 0..max bytes/s | Required when `action` is `rate-limit`; the RFC 8955 traffic-rate. `0` is valid and equals `discard`. Ignored for `discard` |
| `hold-down` | `300` | 1-86400 s | Minimum seconds before the first leak-probe |
| `probe-interval` | `60` | 1-3600 s | Seconds between leak-probe attempts |
| `probe-window` | `10` | 1-300 s | Seconds to observe leaked traffic during a probe |
| `probe-rate` | `1000000` | 1+ bps | Bits per second to allow during a leak-probe |
| `announce-rate-limit` | `10` | 1-600 /min | Maximum FlowSpec announcements per minute |
| `max-mitigation-duration` | `3600` | 0-604800 s | Force-withdraw the announce after this many seconds. Checked once a second; `0` means no cap |
| `backoff-cap` | `3600` | 1-604800 s | Maximum hold-down after exponential backoff |
| `blackhole-fallback` | `false` | bool | Engage an immediate upstream `discard` on a `critical` fast signal without waiting for characterization |
| `confidence-min` | `0` | 0-100 | Minimum incident confidence to announce an upstream rule from a characterized attack (`0` = no gate). The blackhole-fallback fast path is never gated (it carries no confidence) |

### ddos flowtriq (Flowtriq cloud reporter)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `enabled` | `false` | bool | Enable Flowtriq reporting |
| `api-key` | (required) | 1-512 chars | Flowtriq API bearer token |
| `node-uuid` | (required) | 1-128 chars | Node UUID from Flowtriq setup |
| `api-base` | `https://flowtriq.com/api/v1` | URL | API base URL |

## Operational Notes

### Recommended rollout

1. Start with `ddos detect` enabled and `ddos local` in `alert` mode. Monitor
   logs for false positives. Tune `threshold-multiplier` and `absolute-floor` if
   the detector triggers on legitimate traffic spikes.

2. Once confident in detection accuracy, switch `ddos local` to `enforce` mode.
   Always configure the `ddos detect policy` with `allow` rules for management,
   DNS, and control-plane prefixes so mitigation never blocks them.

3. For upstream mitigation, add `ddos flowspec` in `alert` mode first, then
   `enforce`. The hold-down and probe parameters control how aggressively the
   responder probes for attack end.

### Traffic policy

The `ddos detect policy` is a single allow/deny policy, indexed by prefix, that
governs how detected attacks are handled. It replaces the old per-responder
`allowlist` leaves (which are removed: a config still using them fails validation
with `unknown field ... allowlist`). The detector is the single enforcement point and
encodes the decision on the emitted event, so every responder honors one policy
without duplicating it.
<!-- source: internal/plugins/ddos/detect/policy.go -- Policy.evaluate; characterize.go sets SuppressMitigation on the event -->

Each `rule` is keyed by a prefix and carries:

| Leaf | Values | Meaning |
|------|--------|---------|
| `action` | `allow`, `deny` | `allow` exempts matching traffic; `deny` subjects it to DDoS handling |
| `match` | `source`, `destination`, `any` (default) | Match the prefix against the attack source, the victim, or either |
| `scope` | `detection`, `mitigation` (default) | For `allow`: `detection` suppresses the incident entirely; `mitigation` still records it but never blocks |

`default-action` (default `deny`) applies when no rule matches. Rules are evaluated
**longest-prefix-match**: the most specific rule wins, so a `/24 deny` inside a
`/16 allow` is decided by the `/24` with no ordering needed. Ties resolve to `deny`.
Source rules are evaluated once the attack sources are characterized; that
characterized decision is authoritative and withdraws a fast-path drop if it flips to
exempt.

Example: defend everything, exempt a management block from blocking (but keep seeing
incidents), and never even flag a trusted scanner source:

```
ddos {
    detect {
        policy {
            default-action deny;
            rule 198.51.100.0/24 { action allow; match destination; scope mitigation; }
            rule 203.0.113.7/32   { action allow; match source;      scope detection;  }
        }
    }
}
```

Configure `allow` rules at minimum for: management/SSH prefixes, DNS servers, BGP
session endpoints, and any prefix where dropping traffic would cause a control-plane
outage.

**Migration from `allowlist`:** move each `ddos local` / `ddos flowspec` `allowlist X`
entry to a `ddos detect policy` `rule X { action allow; match destination; scope
mitigation; }`. For entries you want fully invisible (no incident logged), use
`scope detection`.

### Direction (local vs remote)

The detector classifies each attack's victim as **local** (an address the box
terminates: control-plane traffic on the netfilter INPUT hook) or **remote** (a
downstream host it forwards: FORWARD hook), shown in `show ddos incidents` as
`direction`. Mitigation is routed by direction: the local responder installs an
INPUT-hook drop for local victims, flowspec announces upstream for remote victims,
and the local responder additionally installs a FORWARD-hook drop for remote victims
only when `ddos local forward-mitigation` is enabled. An unresolved victim is treated
as remote (a local INPUT drop cannot protect an address the box does not own).
<!-- source: internal/component/iface/dispatch.go -- AddressIsLocal; detector.go classifyDirection; local/responder.go hookForDirection -->

### Local mode clear signal

Local-mode clear works because nftables drops occur *after* the kernel NIC RX
counter increments. The detector's RxPps signal continues to reflect the
arriving flood even while it is dropped. The attack is "over" only when RxPps
actually falls below threshold.

**Caveat:** an XDP drop backend would break this (XDP_DROP precedes the RX
counter). Local mode is nft-only for v1.

### FlowSpec mode sensor blindness

Once a FlowSpec rule takes effect upstream, the attack traffic never reaches
Ze's interfaces. The detector goes blind: RxPps drops to baseline immediately.
The detector's `AttackCleared` event is therefore not trustworthy while the
FlowSpec responder is mitigating.

The responder handles this by ignoring the detector's clear signal and running
its own leak-probe cycle. Each probe lets a bounded trickle of traffic through
(`probe-rate` bps) to test whether the flood is still arriving. This is the only
passive signal available until an inbound flow collector exists.

### VPP dataplane

Detection works on both the Linux netlink dataplane and the VPP DPDK dataplane.
The VPP iface backend populates `InterfaceInfo.Stats` from VPP's stats segment,
so `rate.go` computes RxPps/RxBps identically for both dataplanes. No
VPP-specific detector code is needed.

### Viewing DDoS state

`show ddos` is a namespace; the subcommands are each owned by the plugin that
holds the data, so a subcommand appears only while its plugin is loaded:

- `show ddos status` (`ddos-observe`): one-line status showing whether observation is
  running, how many attacks are currently active, and how many incidents are held
  in the ring.
- `show ddos incidents` (`ddos-observe`): the incident ring, newest first. Per
  incident the target vector (prefix / proto / port), attack family, top source
  addresses, peak pps/bps, start/end time, and whether it is still active. The
  ring is bounded by `incident-ring-size` and fed by the detector's
  `AttackDetected` / `AttackCleared` events.
- `show ddos local` (`ddos-local`): whether an on-host nft drop is installed and
  the target vector it covers.
- `show ddos flowspec` (`ddos-flowspec`): whether an upstream FlowSpec rule is
  announced, its target vector, and whether the leak-probe is running.

`show ddos local` and `show ddos flowspec` read an immutable snapshot that the
writer publishes under its own lock, so neither command waits on a netlink
reconcile or on an RPC round trip to the BGP engine. A status command answers
while mitigation is being applied.

<!-- source: internal/plugins/ddos/observe/show.go -- handleShowDdos, handleShowDdosIncidents -->
<!-- source: internal/plugins/ddos/local/show.go -- handleShowDdosLocal -->
<!-- source: internal/plugins/ddos/flowspec/show.go -- handleShowDdosFlowspec -->

### Flow source and observability

Characterization needs an on-box flow source: `traffic usage` (`track-ip`) for the
fast IPv4 target and/or `flow-export` (`conntrack`) for the classifier (proto,
ports, TCP flags, IPv6, top sources). If characterization is enabled with neither
configured, `ze doctor` reports `doctor-ddos-detect-no-flow-source` and mitigation
degrades to a coarse generic-flood drop.
<!-- source: internal/plugins/ddos/detect/doctor.go -- checkFlowSource -->

Prometheus metrics:
- `ze_ddos_detect_characterize_total{family}` -- characterization outcomes per family.
- `ze_ddos_detect_characterize_fallback_total` -- characterizations with no usable flow data.
- `ze_flowexport_recent_ring_drops` -- recent-flow ring entries overwritten before being read.
<!-- source: internal/plugins/ddos/detect/metrics.go -- ze_ddos_detect_characterize_* -->

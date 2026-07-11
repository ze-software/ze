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
    }
    local {
        response-level enforce
        allowlist 10.0.0.0/8
        allowlist 192.168.0.0/16
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
rule for it. The allowlist prevents auto-mitigation from ever blocking
management, DNS, or other critical prefixes.

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

**Tune `active-timeout` for responsive characterization.** Characterization is
one-shot: it queries the recent-flow ring once, when the attack is confirmed. The
ring is refreshed only at each conntrack dump (every `flow-export` `active-timeout`
seconds), so with the default 60s a just-started flood's flows may not be in the
ring yet, and characterization falls back to the coarse destination drop (still
protective, just not narrowed). Set `active-timeout` to a few seconds when you
want the classifier to catch attacks promptly.
<!-- source: internal/plugins/flowexport/conntrack_worker.go -- run() dumps every active-timeout, feeding the ring -->

Inspect the ring directly:

```
show flow-recent                     # all recent conntrack flows (bounded to recent-flow-ring)
show flow-recent dst 203.0.113.42    # flows to a destination prefix or address
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
    }
    flowspec {
        response-level enforce
        action rate-limit
        rate-limit-bytes 1000000
        hold-down 300
        probe-interval 60
        allowlist 10.0.0.0/8
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
| `startup-grace` | `90` | 0-3600 s | Seconds after startup where only extreme spikes (>5x floor) trigger |
| `bps-trigger-enable` | `true` | bool | Enable the bandwidth (BPS) trigger alongside the PPS threshold |
| `bps-threshold-multiplier` | `3.00` | 1.00-100.00 | Baseline p99 multiplier for the bandwidth trigger |
| `bps-floor` | `50000000` | 1+ bits/s | Minimum bandwidth (bits/s) below which the BPS trigger is inert (default 50 Mbps) |
| `characterize-enable` | `true` | bool | Run Stage-2 flow characterization (family + narrowest vector, `AttackCharacterized`) |
| `top-n-sources` | `10` | 1-100 | Max attacker source addresses ranked into TopSources |
| `characterize-window` | `10` | 1-60 s | Seconds of recent flows to consider (timestamp-less flows always kept) |
| `characterize-timeout` | `2000` | 50-5000 ms | Budget for the on-trigger traffic-usage / flow-recent queries |
| `entropy-threshold` | `2.00` | 0.00-16.00 bits | Source-entropy at/above which an attack is logged as distributed/spoofed |

**How the threshold works:**
```
threshold = max(baseline_p99 * threshold-multiplier, absolute-floor)
```

The baseline is a rolling window of the last `baseline-window` non-attack
samples. The p99 is recalculated every 10 samples. Samples collected during an
active attack or above the current threshold are excluded from the baseline to
prevent poisoning.

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
| `max-mitigation-duration` | `3600` | 0-86400 s | Safety valve: force-remove rule after this many seconds (0 = no cap) |
| `confidence-min` | `0` | 0-100 | Minimum incident confidence to mitigate from a characterized attack (`0` = no gate). Note: the coarse drop on the fast `AttackDetected` carries no confidence, so this only gates the in-place narrowing on the characterized path |
| `allowlist` | (empty) | prefix list | Prefixes that must never be blocked (e.g. management, DNS) |

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
| `max-mitigation-duration` | `3600` | 0-604800 s | Safety valve: force-withdraw after this many seconds |
| `backoff-cap` | `3600` | 1-604800 s | Maximum hold-down after exponential backoff |
| `blackhole-fallback` | `false` | bool | Engage an immediate upstream `discard` on a `critical` fast signal without waiting for characterization |
| `confidence-min` | `0` | 0-100 | Minimum incident confidence to announce an upstream rule from a characterized attack (`0` = no gate). The blackhole-fallback fast path is never gated (it carries no confidence) |
| `allowlist` | (empty) | prefix list | Prefixes that must never be announced for mitigation |

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
   Always configure the `allowlist` with management, DNS, and control plane
   prefixes.

3. For upstream mitigation, add `ddos flowspec` in `alert` mode first, then
   `enforce`. The hold-down and probe parameters control how aggressively the
   responder probes for attack end.

### Allowlist

The allowlist is critical. Every responder subtracts allowlisted prefixes from
the mitigation match before installing or announcing. If the target is fully
covered by the allowlist, no action is taken and a log message explains why.

Configure at minimum:
- Management/SSH prefix
- DNS server addresses
- BGP session endpoints
- Any prefix where dropping traffic would cause a control plane outage

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

- `show ddos status` (`ddos-observe`) — one-line status: whether observation is
  running, how many attacks are currently active, and how many incidents are held
  in the ring.
- `show ddos incidents` (`ddos-observe`) — the incident ring, newest first: per
  incident the target vector (prefix / proto / port), attack family, top source
  addresses, peak pps/bps, start/end time, and whether it is still active. The
  ring is bounded by `incident-ring-size` and fed by the detector's
  `AttackDetected` / `AttackCleared` events.
- `show ddos local` (`ddos-local`) — whether an on-host nft drop is installed and
  the target vector it covers.
- `show ddos flowspec` (`ddos-flowspec`) — whether an upstream FlowSpec rule is
  announced, its target vector, and whether the leak-probe is running.

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

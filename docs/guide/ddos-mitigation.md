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
ddos-detect {
    enabled true
}
ddos-local {
    response-level alert
}
```

The detector watches interface rates and emits events. The local responder logs
what it would do but installs no rules. Check the logs for
`ddos-local: alert mode, would mitigate`.

### Local mitigation mode

```
ddos-detect {
    enabled true
}
ddos-local {
    response-level enforce
    allowlist 10.0.0.0/8
    allowlist 192.168.0.0/16
}
```

On attack detection, an nftables drop rule is installed matching the attack
vector (destination prefix + protocol + port). The allowlist prevents
auto-mitigation from ever blocking management, DNS, or other critical prefixes.

The drop rule is removed automatically when the attack stops (the detector
observes RxPps falling below threshold, which works because nftables drops occur
after the kernel NIC RX counter).

### Upstream FlowSpec mitigation

```
ddos-detect {
    enabled true
}
ddos-flowspec {
    response-level enforce
    action rate-limit
    hold-down 300
    probe-interval 60
    allowlist 10.0.0.0/8
}
```

On attack detection, a surgical BGP FlowSpec rule is announced to the configured
upstream peer. The default action is `rate-limit` (non-zero rate) rather than
`discard`, preserving legitimate traffic.

**Clearing under FlowSpec mitigation:** once the upstream drops the attack
traffic, Ze's local sensors go blind (the traffic never arrives). The responder
uses a *leak-probe*: after the hold-down period, it periodically narrows the
FlowSpec rule to a small non-zero rate (`probe-rate`, default 1 Mbps) and
observes whether the leaked traffic saturates that rate. If yes, the flood is
still arriving and the rule is re-tightened with exponential backoff. If no, the
attack is over and the rule is withdrawn.

### Flowtriq cloud reporting

```
ddos-flowtriq {
    enabled true
    api-key YOUR_API_KEY
    node-uuid YOUR_NODE_UUID
}
```

Incidents are reported to the Flowtriq cloud API in real time: open on detection,
update every few seconds with current rates, resolve when the attack ends. The
Flowtriq dashboard provides historical analysis, alerting, and optional
server-driven mitigation commands.

## Configuration Reference

### ddos-detect (detector)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `enabled` | `false` | bool | Enable the detector |
| `check-interval` | `1` | 1-3600 s | Seconds between detection evaluations |
| `confirm-duration` | `3` | 0-3600 | Consecutive ticks above threshold before triggering (0 = immediate) |
| `clear-consecutive-checks` | `10` | 1-100 | Consecutive ticks below threshold before clearing |
| `baseline-window` | `300` | 10-86400 | Rolling baseline window in samples (~seconds at 1 Hz) |
| `threshold-multiplier` | `3.00` | 1.00-100.00 | Baseline p99 multiplier for the dynamic threshold |
| `absolute-floor` | `5000` | 1+ PPS | Minimum threshold regardless of baseline |
| `startup-grace` | `90` | 0-3600 s | Seconds after startup where only extreme spikes (>5x floor) trigger |

**How the threshold works:**
```
threshold = max(baseline_p99 * threshold-multiplier, absolute-floor)
```

The baseline is a rolling window of the last `baseline-window` non-attack
samples. The p99 is recalculated every 10 samples. Samples collected during an
active attack or above the current threshold are excluded from the baseline to
prevent poisoning.

### ddos-local (local responder)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `response-level` | `alert` | alert, enforce | `alert` logs only; `enforce` installs nft drop rules |
| `max-mitigation-duration` | `3600` | 0-86400 s | Safety valve: force-remove rule after this many seconds (0 = no cap) |
| `allowlist` | (empty) | prefix list | Prefixes that must never be blocked (e.g. management, DNS) |

### ddos-flowspec (FlowSpec/RTBH responder)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `response-level` | `alert` | alert, enforce | `alert` logs only; `enforce` announces FlowSpec |
| `action` | `rate-limit` | rate-limit, discard | FlowSpec traffic-action: rate-limit preserves legitimate traffic |
| `hold-down` | `300` | 1-86400 s | Minimum seconds before the first leak-probe |
| `probe-interval` | `60` | 1-3600 s | Seconds between leak-probe attempts |
| `probe-window` | `10` | 1-300 s | Seconds to observe leaked traffic during a probe |
| `probe-rate` | `1000000` | 1+ bps | Bits per second to allow during a leak-probe |
| `announce-rate-limit` | `10` | 1-600 /min | Maximum FlowSpec announcements per minute |
| `max-mitigation-duration` | `3600` | 0-604800 s | Safety valve: force-withdraw after this many seconds |
| `backoff-cap` | `3600` | 1-604800 s | Maximum hold-down after exponential backoff |
| `allowlist` | (empty) | prefix list | Prefixes that must never be announced for mitigation |

### ddos-flowtriq (Flowtriq cloud reporter)

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `enabled` | `false` | bool | Enable Flowtriq reporting |
| `api-key` | (required) | 1-512 chars | Flowtriq API bearer token |
| `node-uuid` | (required) | 1-128 chars | Node UUID from Flowtriq setup |
| `api-base` | `https://flowtriq.com/api/v1` | URL | API base URL |

## Operational Notes

### Recommended rollout

1. Start with `ddos-detect` enabled and `ddos-local` in `alert` mode. Monitor
   logs for false positives. Tune `threshold-multiplier` and `absolute-floor` if
   the detector triggers on legitimate traffic spikes.

2. Once confident in detection accuracy, switch `ddos-local` to `enforce` mode.
   Always configure the `allowlist` with management, DNS, and control plane
   prefixes.

3. For upstream mitigation, add `ddos-flowspec` in `alert` mode first, then
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

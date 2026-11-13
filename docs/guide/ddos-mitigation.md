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
interfaces). Without a reachable flow source-ipv4 the detector still fires but emits a
generic signal with no target, and the local responder cannot install a targeted
rule. The coarse rule matches the destination prefix; characterization (below)
then narrows it in place to the attack's protocol, ports, and TCP flags.
<!-- source: internal/plugins/ddos/detect/characterize.go -- characterizeTarget fills the victim DstPrefix from traffic-usage -->

The flow sources are the `traffic-usage` plugin (build tag `ze_trafficusage`)
and `flow-export` (`ze_flowexport`). A build that carries neither refuses
`ddos detect enabled true` at `ze config validate`, at commit, at daemon start
and at SIGHUP reload, naming both tags as alternatives in its fix line. A build
that carries either accepts it, and whether that source is configured stays the
`doctor-ddos-detect-no-flow-source` warning of `ze doctor`. Enabling detection
starts no flow source: the source you configure (`flow-export` or
`traffic-usage`) starts from its own config block, and one you do not
configure does not run.
`anomaly detect enabled true` gets the same treatment, for the same two sources.

`ddos flowspec response-level enforce` announces through the BGP subsystem, so a
build without `ze_bgp` refuses it the same way, naming `ze_bgp`;
`response-level alert` announces nothing and is accepted in every build.
<!-- source: internal/plugins/ddos/detect/register.go -- ConfigNeeds -->
<!-- source: internal/plugins/anomaly/detect/register.go -- ConfigNeeds -->
<!-- source: internal/plugins/ddos/flowspec/register.go -- ConfigNeeds, bgpPlugin -->

The drop rule is removed automatically when the attack stops (the detector
observes RxPps falling below threshold, which works because nftables drops occur
after the kernel NIC RX counter), when it reaches `max-mitigation-duration`, and
when a `commit` turns mitigation off, and when a `commit` deletes the `ddos
local` block or the `ddos` block that holds it. The table under "When the clear
never comes" gives each case.

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
target when no flow source-ipv4 is reachable.
<!-- source: internal/plugins/ddos/detect/characterize.go -- classifyFlows: family + narrowest VectorTuple from the recent-flow ring -->

**Characterization refreshes the ring on demand.** The recent-flow ring is
normally refreshed only at each conntrack dump (every `flow-export`
`active-timeout` seconds, default 60s). To avoid classifying against a pre-attack
ring, detection forces the refresh: `ddos-detect` emits `AttackDetected`,
`flow-export` responds with an immediate out-of-band conntrack dump, and the
classifier then polls the ring for up to `characterize-timeout` until it reflects
the attack. So a just-started flood is characterized promptly regardless of
`active-timeout`; if the ring still never yields discriminating flows within the
budget, ddos local keeps the coarse destination drop it installed on the fast
signal (still protective, just not narrowed). ddos-flowspec has no such
fallback: see "When characterization does not happen" below. Lowering `active-timeout` is no longer required
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
`critical` to engage an immediate blackhole (below). The severity is graded once,
when the detector triggers, from the highest rate seen above the threshold up to
that moment. It is not revised while the attack continues: the `AttackOngoing`
events that follow carry the current rates and no severity.
<!-- source: internal/core/ddosevent/event.go -- GradeSeverity -->
<!-- source: internal/plugins/ddos/detect/detector.go -- applyTick (peakRxPps), onAttackStart (one grade at trigger) -->
<!-- source: internal/plugins/ddos/detect/characterize.go -- characterizeAndEmit calls GradeSeverity -->
<!-- source: internal/core/ddosevent/event.go -- AttackOngoing has no severity -->

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
must be right the first time. Once a `flowspec` block is present, its `action` is
mandatory (no default, because neither choice is universally safe); a `ddos` block
without `flowspec` needs none: `rate-limit` announces an RFC 8955
traffic-rate of `rate-limit-bytes` bytes/sec (preserving legitimate traffic up to
that rate), while `discard` drops the whole characterized flow. A `rate-limit-bytes`
of `0` is valid and is equivalent to `discard`; `action rate-limit` without a
`rate-limit-bytes` is a configuration error (no rate is fabricated).
<!-- source: internal/plugins/ddos/flowspec/config.go -- Validate: action mandatory; rate-limit requires rate-limit-bytes; 0 == discard -->
<!-- source: internal/plugins/ddos/flowspec/responder.go -- renderFlowspecCommand emits update text extended-community [rate-limit:<bps>] mp <fam> add <components> -->


**Blackhole fallback.** With `blackhole-fallback true`, a `critical`-severity
attack (peak >= 5x threshold) engages an immediate upstream `discard` on the fast
signal without waiting for characterization -- the escape hatch when local
filtering cannot hold the flood. Because the grade is fixed when the detector
triggers, an attack that triggers below five times the threshold and then grows
is not treated as `critical`, and blackhole-fallback does not fire for it. With
characterization enabled, the characterized announcement still follows.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- onCharacterized announces; onDetected blackholes only on critical + policy; onOngoing feeds only probeTick -->
<!-- source: internal/plugins/ddos/detect/detector.go -- onAttackStart -->

**When characterization does not happen.** ddos-flowspec announces on the fast
signal only when `blackhole-fallback` is `true` and the attack is `critical`.
Every other detection waits for the characterized event. With
`characterize-enable false`, or when characterization finds no classifiable
flows within `characterize-timeout`, the detector emits no characterized event,
so a `medium` or `high` attack is never announced upstream. The only trace is
the INFO line `ddos-flowspec: awaiting characterization before announcing`; a
run with no flows also counts in
`ze_ddos_detect_characterize_fallback_total`. If you rely on FlowSpec for
attacks below `critical`, keep `characterize-enable true` with a flow source
configured. ddos local does not wait: it installs its coarse drop on the fast
signal, so it still protects a victim it covers when characterization does not
happen.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- onDetected, onCharacterized -->
<!-- source: internal/plugins/ddos/detect/characterize.go -- characterizeFromFlows returns on !CharacterizeEnable and on no flows (incFallback) -->
<!-- source: internal/plugins/ddos/local/responder.go -- onDetected applies the coarse drop -->

**A detection while the plugin starts.** ddos-flowspec acts on detections only
after its start has withdrawn any rule a previous run left recorded. A
detection that fires before that, during the plugin's startup handshake, is not
seen. The detector reports each attack once, so that attack is announced only
if its characterized event arrives after the start, or once the attack clears
and triggers again.
<!-- source: internal/plugins/ddos/flowspec/register.go -- runEngine: OnStarted withdraws the recorded rule, then subscribes -->

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
| `confirm-duration` | `3` | 0-3600 | Consecutive evaluations above threshold before triggering (0 = immediate) |
| `clear-consecutive-checks` | `10` | 1-100 | Consecutive evaluations below threshold before clearing |
| `baseline-window` | `300` | 10-86400 | Rolling baseline window in samples, one sample per evaluation (~seconds at the default `check-interval`) |
| `threshold-multiplier` | `3.00` | 1.00-100.00 | Baseline p99 multiplier for the dynamic PPS threshold |
| `absolute-floor` | `5000` | 1+ PPS | Minimum PPS threshold regardless of baseline |
| `startup-grace` | `90` | 0-3600 s | Seconds after startup where only a packet rate at least 5x the floor or an armed bandwidth trigger escapes grace |
| `bps-trigger-enable` | `true` | bool | Enable the bandwidth (BPS) trigger alongside the PPS threshold |
| `bps-threshold-multiplier` | `3.00` | 1.00-100.00 | Baseline p99 multiplier for the bandwidth trigger |
| `bps-floor` | `50000000` | 1+ bits/s | Minimum bandwidth (bits/s) below which the BPS trigger is inert (default 50 Mbps) |
| `characterize-enable` | `true` | bool | Run Stage-2 flow characterization (family + narrowest vector, `AttackCharacterized`) |
| `top-n-sources` | `10` | 1-100 | Max attacker source addresses ranked into TopSources |
| `characterize-window` | `10` | 1-60 s | Seconds of recent flows to consider (timestamp-less flows always kept) |
| `characterize-timeout` | `2000` | 50-5000 ms | Budget for the on-trigger traffic-usage / flow-recent queries |
| `entropy-threshold` | `2.00` | 0.00-16.00 bits | Source-entropy at/above which an attack is logged as distributed/spoofed |
| `policy` | (default-action `deny`) | container | Allow/deny traffic policy (see [Traffic policy](#traffic-policy)) that exempts or defends prefixes; replaces the old per-responder allowlists |

**How `check-interval` works:** the trafficstat rate feed publishes once a
second, and the detector folds `check-interval` of those samples into one evaluation. The
evaluation reads the PEAK of the interval, on the interface that carried it, so a
one-second flood inside a 10-second interval still reaches the threshold.
`confirm-duration`, `clear-consecutive-checks` and `baseline-window` all count
evaluations, so raising `check-interval` multiplies their wall-clock meaning by
the same factor. `startup-grace` counts feed samples and stays in seconds.
<!-- source: internal/plugins/ddos/detect/detector.go -- tick, intervalPeak -->

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

**Baseline persistence:** the detector saves both baselines in the shared zefs
state store on shutdown/reconfigure and periodically, then restores them on startup.
A full restored bandwidth window arms the BPS trigger at the first evaluation.
A valid partial snapshot can restore with 50 samples (or the window size when
smaller), but BPS stays unarmed until the window is full. Missing, malformed, or
incompatible state starts cold. PPS always has its absolute-floor threshold.
<!-- source: internal/plugins/ddos/detect/persist.go -- saveBaselines, loadBaselines -->
<!-- source: internal/plugins/ddos/detect/baseline.go -- restore, Ready, Threshold -->

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
| `max-mitigation-duration` | `3600` | 0-86400 s | Remove the on-host drop after this many seconds. Checked once a second; `0` means no cap. The clock starts when the rule goes in, and a narrowing of the same rule does not restart it |
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
| `announce-rate-limit` | `10` | 1-600 /min | Maximum FlowSpec announcements in any 60-second window. One over the limit is refused and logged, and the responder stays idle so the next characterization inside the window announces nothing |
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

**When the clear never comes.** A detector that is reconfigured or stopped
mid-attack emits no `AttackCleared`, and an attack that never falls below the
threshold produces none either. `max-mitigation-duration` is what bounds the
drop rule in both cases: a worker in the plugin checks the age of the live rule
once a second and removes a rule that has reached the cap, logging
`max-mitigation-duration reached`. `show ddos local` then reports no mitigation.

**What a commit does to a live drop rule.** The plugin builds a new responder for
the new config, and what happens to the rule depends on what the commit says.

| The commit | The live drop rule |
|------------|--------------------|
| Any `ddos local` leaf, `response-level` still `enforce` | KEPT, with its cap. The new responder is handed the rule, the victim it covers and the instant it went in, so the cap keeps counting from the FIRST install and an `AttackCleared` still removes it. The new config governs from the commit, so a `commit` that shortens `max-mitigation-duration` applies the shorter cap to the rule already installed |
| `response-level alert` | REMOVED at once. `alert` is detect-and-report, and un-blackholing a victim is what an operator commits it for. The log line is `response-level left enforce, removing the drop rule` |
| `forward-mitigation false`, with the live rule on the FORWARD hook | REMOVED at once. The leaf's whole subject is whether this box drops a transit victim's traffic on-host, so turning it off is the commit that says stop. The log line is `forward-mitigation was disabled, removing the drop rule`. A drop on the INPUT hook, for a victim the box owns, is KEPT: this leaf says nothing about it |
| The `ddos local` block deleted | REMOVED at once. The plugin is stopped as soon as the reload lands, so a rule carried past this point would stay in the kernel with no responder and no cap worker left to remove it. The log line is `the ddos local section was removed, removing the drop rule` |
| The parent `ddos` block deleted | REMOVED at once, as for the `ddos local` block. The reload records one removed key, `ddos`, and matches it as an ancestor of `ddos/local`, so the plugin is told that its section is gone. The log line is `the ddos local section was removed, removing the drop rule` |
<!-- source: internal/plugins/ddos/local/responder.go -- enforceMaxDuration, adoptMitigation, withdrawMitigation; internal/plugins/ddos/local/register.go -- startMaxDurationWorker, replaceResponder, stopResponder -->
<!-- source: internal/component/plugin/server/reload.go -- rootHasChanges -->

**A daemon stop makes no promise about the drop.** ddos local does try to
withdraw the rule as it exits. That withdrawal covers a stop that delivers no
config, such as a plugin crash, while the daemon and the firewall engine both
live on.

At a DAEMON stop nothing orders it. `ProcessManager.Stop` cancels every plugin at
once and waits on them together. So the firewall engine CAN close its backend
while ddos local is still joining its cap worker. A withdrawal that lands after
that close writes nothing to the kernel. A crash, a power loss and
`firewall { flush-on-shutdown false; }` each leave the rule as well.

**ddos local clears its previous drop at startup.** A ddos drop is an attack
response, and a reboot is one way an operator clears state. The plugin sweeps
`ze_ddos-local` at its initial configure callback, after the firewall dependency
has selected its configured backend and before a responder subscribes to events.
Reload uses the verify/apply callbacks and does not repeat this sweep.
<!-- source: internal/plugins/ddos/local/register.go -- runEngine, clearStaleDropRule -->
<!-- source: internal/component/plugin/server/startup.go -- runPluginPhase -->
<!-- source: internal/component/firewall/engine.go -- runEngine -->

On nft, the sweep first claims the name to reach a table this backend instance
did not install, then withdraws the empty table. Other owners keep their desired
rules. Cleanup is best-effort: a failure logs
`could not clear a drop rule left by a previous process`, with the failed stage
in `error`. A failed final reconcile can leave only an empty table after the
first reconcile removed the drop. Detection and reporting continue.
<!-- source: internal/plugins/ddos/local/register.go -- clearStaleDropRule, runEngine -->
<!-- source: internal/plugins/firewall/nft/backend_linux.go -- shouldDeleteTable -->

Protection after a restart depends on fresh detection. The timing below assumes
an uninterrupted one-second rate feed and the default `check-interval` of 1.

| What | Default | What it costs after a restart |
|------|---------|-------------------------------|
| `startup-grace` | 90 seconds | Quiet evaluations return before either baseline admits a sample. A packet rate at least five times `absolute-floor`, including exactly 5x, escapes grace. An enabled BPS trigger also escapes when its baseline is full and traffic exceeds its threshold. Escaping grace still requires the detection threshold and confirmation |
| `confirm-duration` | 3 evaluations | Three consecutive above-threshold evaluations before `AttackDetected`, about 3 seconds at the default cadence |
| `baseline-window` | 300 admitted samples | PPS has no extra window delay: a cold cache gives the absolute-floor threshold. BPS requires a full window. With quiet cold startup, 90 seconds of grace precede 300 admitted evaluations, so BPS first evaluates armed at about 391 seconds, then needs confirmation. A full restored window is armed immediately. A partial restored window still needs its missing samples |

<!-- source: internal/plugins/ddos/detect/detector.go -- applyTick, the startup-grace escape -->
<!-- source: internal/plugins/ddos/detect/persist.go -- saveBaselines, loadBaselines -->
<!-- source: internal/plugins/ddos/detect/baseline.go -- Threshold, Ready, restore -->

The cold BPS estimate is about 390 seconds plus confirmation because readiness
is read before the final sample is admitted. At default settings, an attack
first above threshold on evaluation 391 confirms on evaluation 393. Attack-marked
samples are normally excluded, so a sustained PPS attack can extend BPS warm-up.
Larger `check-interval` values multiply the sample and confirmation periods,
while startup grace remains in feed seconds.
<!-- source: internal/plugins/ddos/detect/detector.go -- tick, applyTick -->
<!-- source: internal/plugins/ddos/detect/baseline.go -- Add, admit -->
<!-- source: internal/plugins/ddos/local/register.go -- clearStaleDropRule -->

**Going back to `enforce`.** A commit of `response-level alert` removes the rule,
and a commit of `enforce` after it does NOT put the rule back for the attack that
is already running. The detector signals an attack once for each generation, so
no new signal arrives until the flood clears and a fresh one starts. Leave the
box in `enforce` while an attack is live.

### FlowSpec mode sensor blindness

Once a FlowSpec rule takes effect upstream, the attack traffic never reaches
Ze's interfaces. The detector goes blind: RxPps drops to baseline immediately.
The detector's `AttackCleared` event is therefore not trustworthy while the
FlowSpec responder is mitigating.

The responder handles this by ignoring the detector's clear signal and running
its own leak-probe cycle. Each probe lets a bounded trickle of traffic through
(`probe-rate` bps) to test whether the flood is still arriving. This is the only
passive signal available until an inbound flow collector exists.

**What a commit does to a live FlowSpec announcement.** The plugin builds a new
responder for the new config, and what happens to the announcement depends on
what the commit says. The NLRI lives in the BGP engine's RIB and in every peer's,
neither of which belongs to this plugin, so an announcement the plugin forgets is
one no operator can lift.

| The commit | The live announcement |
|------------|-----------------------|
| Any `ddos flowspec` leaf, `response-level` still `enforce` | KEPT, with its cap and its running leak-probe. The new responder is handed the rule, the victim it covers, the instant it went out and the probe's position in its cycle, so the cap keeps counting from the FIRST announce and the probe still decides the withdrawal. The new config governs from the commit, so a `commit` that shortens `max-mitigation-duration` applies the shorter cap to the rule already announced. `announce-rate-limit` keeps the budget already spent, so committing is not a way to announce past the limit |
| `response-level alert` | WITHDRAWN at once. `alert` is detect-and-report, and lifting an upstream discard is what an operator commits it for. The log line is `response-level left enforce, withdrawing the announcement` |
| The `ddos flowspec` block deleted | WITHDRAWN at once. The plugin is stopped as soon as the reload lands, so an announcement carried past this point would sit in every peer's RIB with no responder and no cap worker left to withdraw it. The log line is `the ddos flowspec section was removed, withdrawing the announcement` |
| `action` or `rate-limit-bytes` changed | The rule already on the wire keeps the action it was announced with. Nothing re-announces, so the new values govern the NEXT announcement |
| The parent `ddos` block deleted | WITHDRAWN at once, as for the `ddos flowspec` block. The reload records one removed key, `ddos`, and matches it as an ancestor of `ddos/flowspec`, so the plugin is told that its section is gone. The log line is `the ddos flowspec section was removed, withdrawing the announcement` |
<!-- source: internal/plugins/ddos/flowspec/register.go -- replaceResponder; internal/plugins/ddos/flowspec/responder.go -- adoptAnnouncement, withdrawForConfig, enforceMaxDuration -->
<!-- source: internal/component/plugin/server/reload.go -- rootHasChanges -->

**What each end of the plugin does to a live FlowSpec announcement.** A commit
is not the only way the plugin ends. Where the plugin can still reach the BGP
engine, it withdraws the rule on its way out. Where it cannot, the rule stays
recorded and the next start of the plugin withdraws it. The plugin runs in the
daemon by default, and then it cannot reach the engine on its way out on any
end other than the `bye`: the plugin's calls are refused once its run ends.

| The end | The live announcement |
|---------|-----------------------|
| The plugin removed while the daemon runs (the engine's `bye`) | WITHDRAWN before the plugin answers the `bye`. The engine waits 500 ms for that answer, and the withdrawal has 400 ms of its own. A withdrawal that does not reach the engine in that time, or that the engine refuses, leaves the record. The plugin then logs one WARN line with the code `doctor-ddos-flowspec-orphan-announcement` and, under `fix`, the exact withdrawal to send by hand, because no start follows a removal; with the section gone, `ze doctor` reports the recorded rule. When the rule has no record (its record write failed), the line says it is not recorded instead, with the same `fix` |
| A daemon stop | NOT withdrawn. The engine sends no `bye`. The plugin still tries the withdrawal on its way out, but the daemon's stop refuses the plugin's calls before the plugin's run ends, so the withdrawal cannot leave. When the rule is recorded, the record stays, the next start withdraws the recorded rule, and the plugin logs one INFO line that says `because the plugin's connection to the engine is closed; the rule stays recorded`, with the plugin restart under `fix`. When it is not recorded (its record write failed), no start withdraws it, and the plugin logs one WARN line that says `and it is not recorded`, with the exact withdrawal to send by hand under `fix`. A plugin that runs as its own process can also log the WARN of a failed run at a daemon stop, when the stop closes its connection while it is writing an answer to the engine. The BGP sessions end with the daemon: a peer drops the rule with the session, or keeps it as stale under graceful restart until its restart timer runs out. The one exception is a plugin that runs as its own process (`external`) and receives SIGINT or SIGTERM from outside before the daemon's stop reaches it, for example a signal sent to its process group: its run then ends while the engine still answers, the withdrawal reaches the engine, and the plugin removes the record |
| The plugin stops on its own (a panic, a failed run) while the daemon runs | In the daemon (the default placement): NOT withdrawn, because the plugin's calls are refused once its run ends. The rule stays at the peers. When it is recorded, the plugin logs the same line as at a daemon stop, at WARN with the code `doctor-ddos-flowspec-orphan-announcement`, and what follows in this row applies. When it is not recorded (its record write failed), the plugin logs the WARN line that says `and it is not recorded`, with the exact withdrawal to send by hand under `fix`: no start withdraws it. The plugin declares no failure policy, so the daemon does not start it again: `ze doctor` reports it as stopped (`doctor-plugin-stopped`), and its fix, `ze request plugin restart ddos-flowspec`, starts it, and that start withdraws the recorded rule. As its own process (`external`): WITHDRAWN on the way out when its run ends while its connection to the engine is still open; a stop that closed the connection first leaves the rule recorded, as in the daemon |
| The plugin is killed | NOT withdrawn: a killed process runs nothing on its way out. The rule stays recorded, and the next start withdraws it |
<!-- source: internal/plugins/ddos/flowspec/register.go -- stopResponder, closedTransportMessage, unrecordedExitMessage, removedExitMessage, exitKind, exitWithdrawBudget, runEngine (the deferred exit calls stopResponder with its exitKind; OnBye passes exitRemoved), transportClosed -->
<!-- source: internal/plugins/ddos/flowspec/responder.go -- announce and adoptAnnouncement set recorded, endAnnouncement clears it; withdrawalCommand -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop returns the send error of a callback answer, so Run fails -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn.closedErr wraps the reader's terminal error (io.EOF when the engine closed its end) -->
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run: its deferred bridge.CloseCallbacks closes dispatch before the plugin's deferred exit runs -->
<!-- source: internal/component/plugin/server/failure_policy.go -- applyFailurePolicy: no declared policy records the plugin stopped -->
<!-- source: internal/component/plugin/process/stopped.go -- RecordStopped raises doctor-plugin-stopped with RestartCommand -->
<!-- source: internal/component/plugin/process/manager.go -- ProcessManager.Stop closes each plugin's connection -->
<!-- source: internal/component/plugin/process/process.go -- Process.Stop: StopDispatch, then CloseCallbacks, then closeConns -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge.DispatchRPC refuses through beginDispatch with ErrBridgeClosed -->
<!-- source: pkg/plugin/sdk/signal.go -- SignalContext under HostOwnsSignals -->

**The live-announcement record.** Each announcement is recorded in the runtime
state key `meta/ddos-flowspec/announcement` before the plugin goes on, and every
withdrawal the BGP engine accepts removes the record. A withdrawal the engine
refuses (for example, when no peer gives the plugin permission to send) leaves
the record. When the record removal after an accepted withdrawal fails, the
plugin logs a WARN line with the code `doctor-ddos-flowspec-orphan-announcement`
and the record stays until the next start, which withdraws the rule again (a
second withdrawal changes nothing at the peers) and removes it. When the record cannot
be written, the plugin logs a WARN line with the code
`doctor-ddos-flowspec-orphan-announcement`, and the announcement still goes out:
refusing to mitigate because of a failed record would be worse. Such a rule is
live with no record, so no start withdraws it and `ze doctor` does not report
it; if the plugin then ends without withdrawing it, its exit line says so at
WARN and carries the withdrawal by hand under `fix`. A record of the same rule
that an earlier failed withdrawal left still counts: it names the live rule, so
the next start withdraws it. When the plugin
starts and finds a record, it withdraws that rule, logs `a previous run left a
FlowSpec rule announced, withdrawing it` at WARN with the victim, and removes the
record. It adopts nothing: protection after a restart comes from fresh detection,
as for ddos local. This withdrawal runs when the plugin starts, which can be
before any BGP session is established. It then reaches no peer, and that is
correct, because a session that comes up later never held the rule. When the
withdrawal fails, the rule stays recorded, and the WARN line names the error
and, under `fix`, the plugin restart that tries again.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- announce, withdrawThrough, withdrawRecordedAnnouncement, readRecordedAnnouncement, withdrawRecordedRule, encodeAnnouncementRecord -->

One record holds one rule. So when a rule is still recorded because its
withdrawal failed, a new announcement first withdraws that rule, and only then
announces and records its own. When that withdrawal fails again, nothing new is
announced: the plugin logs `not announcing, because an earlier FlowSpec rule is
still recorded` at WARN, and the next attack event tries both again. A record of
the same rule that is about to be announced is not withdrawn first, because the
announcement replaces it at the peers.
<!-- source: internal/plugins/ddos/flowspec/responder.go -- announce, withdrawEarlierRecord -->

`ze doctor` reports a recorded rule that no running responder holds, under
`doctor-ddos-flowspec-orphan-announcement`. It names the victim and says why the
rule may still be at the peers. While `ddos flowspec` is configured and the
plugin is stopped, no check reads the record: doctor reports the stopped plugin
under `doctor-plugin-stopped`, and its fix, the plugin restart, withdraws the
recorded rule when the plugin starts. What it prints as the fix depends on
whether the plugin runs:

| `ddos flowspec` | Who answers | Fix | What clears the report |
|-----------------|-------------|-----|------------------------|
| Configured | The plugin, from its own runtime check | `ze request plugin restart ddos-flowspec` | The restarted plugin withdraws the recorded rule when it starts, and removes the record. When the cause of the failed withdrawal is still there (for example, no peer gives the plugin permission to send), the restart fails the same way, the WARN line names the cause, and the report stays until you fix that cause and restart again |
| Not configured | Doctor, from the store | The exact withdrawal, `ze cli -c "send bgp * update text mp ipv4/flow del ..."` | The withdrawal removes the rule from the peers, not the record, so the report stays. The plugin removes the record the next time it starts, when you configure `ddos flowspec` again. `ze data rm meta/ddos-flowspec/announcement` removes it only while ze is stopped, because the store refuses a second writer while the daemon owns it |
<!-- source: internal/plugins/ddos/flowspec/doctor.go -- orphanRuntimeCheckDecl, orphanRecordCheck, ruleFindings, unownedOrphanDiagnostic, unconfiguredOrphanDiagnostic, recordRemovalToday -->
<!-- source: internal/component/plugin/process/stopped.go -- RestartCommand -->

A record that does not decode is reported as corrupt and is never sent to the
peers, and no start of the plugin withdraws anything from it, so a restart does
not clear it. Its fix is `ze data cat meta/ddos-flowspec/announcement`, which
prints the stored record: it can still show the victim prefix, so you can
withdraw that rule at the peers by hand. The record stays until the plugin
records a new announcement over it, or until `ze data rm` removes it while ze is
stopped. That announcement still goes out, over a corrupt record or one the
plugin cannot read, and its WARN line says that the earlier rule, if it is still
at the peers, can no longer be withdrawn from its record: it carries the stored
bytes of a corrupt record under `record`, and the withdrawal by hand under `fix`.
A doctor run that opened no store, or holds no parsed configuration,
cannot judge the record: it says so, and its fix is the same `ze data cat`
command, which prints the record if one is stored.
<!-- source: internal/plugins/ddos/flowspec/doctor.go -- corruptRecordDiagnostic, recordReadCommand, manualWithdrawalCommand, checkOrphanRecord, storedRecordReading -->
<!-- source: internal/plugins/ddos/flowspec/responder.go -- withdrawEarlierRecord, classifyRecord, recordReading -->

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
  `AttackDetected` / `AttackCleared` events. The clear names the victim the
  detection named, so it closes that incident as soon as the attack ends; only
  an incident whose clear never comes waits for `stale-incident-timeout`.
  <!-- source: internal/plugins/ddos/detect/detector.go -- emitCleared -->
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
configured, `ze doctor` reports `doctor-ddos-detect-no-flow-source`. ddos local
then degrades to a coarse generic-flood drop, and ddos-flowspec announces only a
`critical` attack under `blackhole-fallback` (see "When characterization does
not happen").
<!-- source: internal/plugins/ddos/detect/doctor.go -- checkFlowSource -->

Detection itself does not read the flow sources. Its input is the per-interface
receive rate from the trafficstat rate feed. A rate feed that delivers nothing, or reports zero
traffic, fills the baseline with zeros. The baseline then reads as ready, the
threshold sits at `absolute-floor`, and the detector cannot trigger. When this
lasts for one baseline window (`baseline-window` times `check-interval`
seconds, 300 seconds by default), `ze doctor` reports `doctor-ddos-detect-blind`
as a warning, and the log carries one WARN line at the onset and one INFO line
when traffic returns. The warning names which case it saw:

| Case | What to check |
|------|---------------|
| ddos-detect runs as an external plugin process | Remove the `plugin { external ddos-detect { ... } }` block. The feed is a same-process subscription, so an external ddos-detect never receives it. Today an external ddos-detect never arms: it fails startup with `ddos-detect: event bus not configured` in the log, because a forked plugin is handed no event bus. Once that is fixed, this case is reported as soon as the detector is armed, with no window |
| No sample from the feed for a whole window | `ze show interface rate` |
| The feed reports no interface | `ze show interface` |
| Zero received packets and bytes on every interface | `ze show interface rate`. An idle lab or standby router shows this legitimately |

The warning clears on the first sample with traffic. `ddos detect enabled false`
silences it, because a disabled detector subscribes to no feed.
<!-- source: internal/plugins/ddos/detect/blind.go -- observeFeed, rateFeedFinding, blindMessage -->
<!-- source: internal/plugins/ddos/detect/doctor.go -- checkRateFeed -->

Prometheus metrics:
- `ze_ddos_detect_characterize_total{family}` -- characterization outcomes per family.
- `ze_ddos_detect_characterize_fallback_total` -- characterizations with no usable flow data.
- `ze_flowexport_recent_ring_drops` -- recent-flow ring entries overwritten before being read.
<!-- source: internal/plugins/ddos/detect/metrics.go -- ze_ddos_detect_characterize_* -->

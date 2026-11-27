---
spec: firewall-dynamic-set-config-fidelity
topic: Firewall
feature: firewall
found-by: review
---

## Before

A firewall named set configured with `flags-interval` and a CIDR element such as
`element 192.0.2.0/24 {}` silently discarded the prefix length during nftables
lowering and programmed only the single host address `192.0.2.0`, leaving the
rest of the prefix unmatched. Sets with invalid flag or element combinations
(a CIDR prefix without `flags-interval`, an element `timeout` without
`flags-timeout`, host bits set in a CIDR prefix, wrong-family IP elements,
overlapping intervals, or `flags-constant` combined with `flags-dynamic` or
`flags-timeout`) slipped past `ze config validate` and either dropped timeouts,
truncated prefixes, or failed later at kernel apply; an invalid firewall
configuration at `ze start` also exited the daemon instead of keeping the rest
of the router running. In addition, firewall terms using `from { tcp-flags ...; }`
were rejected by YANG schema validation even though the firewall parser, kernel
and VPP backends, and user guide already supported them, and kernel lowering of
`tcp-flags` did not emit a `meta l4proto tcp` guard when `protocol tcp;` was
omitted.

## After

Operator-configured IPv4 and IPv6 `flags-interval` sets now lower canonical CIDR
prefixes and bare host addresses into numerically sorted half-open intervals in
nftables (attaching per-element timeouts to interval start elements and handling
adjacent and top-of-space prefixes cleanly), so `network-group` sets match full
prefixes directly without a one-term-per-prefix workaround. `ze config validate`,
commits, and reloads now refuse contradictory set flags (`flags-constant` with
`flags-dynamic` or `flags-timeout`), per-element timeouts without
`flags-timeout`, CIDR prefixes without `flags-interval`, non-canonical prefixes
with host bits set, wrong-family or zoned IP elements, and overlapping intervals
with clear remediation steps; when a startup configuration file contains an
invalid firewall set, `ze start` leaves the firewall unstarted, logs an error
under `config-plugin-verify` with the fix, and keeps the daemon and its other
services running while `ze doctor` reports the same code and fix. Firewall terms
now accept `from { tcp-flags ...; }` (`fin`, `syn`, `rst`, `psh`, `ack`, `urg`)
through the YANG schema on both `kernel` and `vpp` data planes, and the kernel
backend prepends `meta l4proto tcp` so terms omitting `protocol tcp;` inspect
only TCP headers.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none

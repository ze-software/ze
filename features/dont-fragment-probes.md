# Don't Fragment probes

## Meta

| Field | Value |
|-------|-------|
| Name | Don't Fragment probes |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/probe, internal/component/ping, internal/component/traceroute |
| Real-path tests | test/plugin/ping-do-not-fragment-reports-mtu.ci, test/plugin/ping-do-not-fragment-unprivileged.ci, test/plugin/doctor-icmp-probe-missing.ci |
| Docs | docs/guide/command-reference.md |
| Doc review | 2026-10-07: IP_RECVERR set in internal/core/probe socket options and the error queue read in errqueue_linux.go DrainErrorQueue; the IPv6 1280 floor read in errqueue_linux.go |
| Defect review | 2026-10-07: journal 2026-09-15 probe rows fixed; no open row or immediate spec names internal/core/probe |
| Extra criteria | supported: show traceroute do-not-fragment over a clamped path reports next-hop-mtu = test/plugin/traceroute-do-not-fragment-reports-mtu.ci; supported: IPv6 do-not-fragment probe reports next-hop-mtu = test/plugin/ping-do-not-fragment-ipv6.ci |

## Description

`show ping`, `resolve ping`, `show traceroute` and `resolve traceroute` take `do-not-fragment honor-cache` or `do-not-fragment bypass-cache`. The probe socket installs `IP_MTU_DISCOVER` and `IP_RECVERR`, the Linux kernel runs RFC 1191 and RFC 8201 path-MTU discovery, and the reply that a router refused carries `status: too-big`, `next-hop-mtu-reported` and `next-hop-mtu` read off the socket error queue, never parsed from ICMP by ze. A zero or an IPv6 value below 1280 is reported as no usable value. `bypass-cache` ignores the kernel's cached estimate so the wire answers. Without `CAP_NET_RAW`, ping runs on the unprivileged datagram ICMP socket when the daemon's group is inside `net.ipv4.ping_group_range`; `ze doctor` reports `doctor-icmp-probe` when neither socket opens and `doctor-icmp-probe-unprivileged` when only the datagram one does (traceroute needs the raw socket). Linux only.

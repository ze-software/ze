# Connection Tracking Management

## Meta

| Field | Value |
|-------|-------|
| Name | Connection Tracking Management |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/system/conntrack.go, internal/component/config/system/conntrack_helpers.go, internal/component/firewall/conntrack_helpers.go, internal/core/sysctl/known.go |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: anchors conntrack.go and known.go exist; no functional test was found, so none is listed |
| Defect review | 2026-10-07: no open spec or journal row found naming conntrack |
| Extra criteria | supported: a parse test and a privileged apply test for system conntrack = none yet |

## Description

Declarative conntrack configuration under `system { conntrack {} }`. Conntrack helpers (ALGs) assigned only when listed: each `module` (amanda, ftp, h323, irc, netbios-ns, pptp, sane, sip, snmp, tftp) gets a ze-owned nft table with a `ct helper` object per kernel helper and transport, assigned by destination port in prerouting and output at priority -5, in an `inet` table for a helper the kernel registers for both families and an `ip` table for an IPv4-only one; an empty list assigns nothing. Ze's kernel builds every helper in; on a distribution kernel the module is loaded with modprobe. `show system conntrack` lists the assigned helpers. User-friendly config for table sizing (table-size, hash-size, expect-max), per-protocol timeouts (TCP, UDP, ICMP, ICMPv6, GRE, SCTP, DCCP), TCP behavior flags (be-liberal, loose, max-retrans, ignore-invalid-rst), and global flags (accounting, timestamp, checksum, log-invalid). All sysctl values routed through the sysctl plugin for three-layer precedence. Dual-setting prevention rejects keys in `sysctl {}` that conntrack manages. On gokrazy (modules built-in), module loading is skipped gracefully. CLI: `show system conntrack`. Telemetry: configured-max gauge alongside existing per-CPU counters. <!-- source: internal/component/config/system/conntrack.go -- ConntrackConfig, sysctl key mapping --> <!-- source: internal/core/sysctl/known.go -- KnownSysctl registry, MustRegister, Validate -->

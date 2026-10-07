# Connection Tracking Management

## Meta

| Field | Value |
|-------|-------|
| Name | Connection Tracking Management |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/system/conntrack.go, internal/core/sysctl/known.go |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: anchors conntrack.go and known.go exist; no functional test was found, so none is listed |
| Defect review | 2026-10-07: no open spec or journal row found naming conntrack |
| Extra criteria | supported: a parse test and a privileged apply test for system conntrack = none yet |

## Description

Declarative conntrack configuration under `system { conntrack {} }`. Helper module loading (ftp, sip, h323, pptp, tftp, sane, irc, amanda, netbios-ns, snmp, nfs, sqlnet) via modprobe on Linux (load-only, never unload). User-friendly config for table sizing (table-size, hash-size, expect-max), per-protocol timeouts (TCP, UDP, ICMP, ICMPv6, GRE, SCTP, DCCP), TCP behavior flags (be-liberal, loose, max-retrans, ignore-invalid-rst), and global flags (accounting, timestamp, checksum, log-invalid). All sysctl values routed through the sysctl plugin for three-layer precedence. Dual-setting prevention rejects keys in `sysctl {}` that conntrack manages. On gokrazy (modules built-in), module loading is skipped gracefully. CLI: `show system conntrack`. Telemetry: configured-max gauge alongside existing per-CPU counters. <!-- source: internal/component/config/system/conntrack.go -- ConntrackConfig, sysctl key mapping --> <!-- source: internal/core/sysctl/known.go -- KnownSysctl registry, MustRegister, Validate -->

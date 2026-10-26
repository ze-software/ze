| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-26 | lan-dns-resolver | dnsserver | The shipped authoritative listeners (as112, geodns) build `dns.Server` with `UDPSize` left at zero, so miekg/dns reads at most 512 octets of a UDP query, and no OPT record is ever answered: RFC 4035 Section 3 rows RFC4035-3-1 and RFC4035-3-2 (EDNS0 support) stay open `{gap}` rows with no owner (`internal/core/dnsserver/manager.go`, `startUDP`) | open: not fixed; owned by spec-network-services-one-off-defects |

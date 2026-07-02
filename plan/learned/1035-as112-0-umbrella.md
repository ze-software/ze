# 1035 - as112-0-umbrella

## Context

Umbrella spec tying together the AS112 anycast DNS feature: spec-as112-1
(generic plugin-owned address registry, [1028](1028-as112-1-iface-address-registry.md)),
spec-as112-2 (the DNS-serving plugin, [1033](1033-as112-2-dns-server.md)),
spec-as112-3 (BGP watchdog integration, [1034](1034-as112-3-bgp-integration.md)),
plus a later cross-cutting review-hardening pass ([1032](1032-as112-review-hardening.md)).
This entry captures the umbrella-level decisions that don't belong in any one
child.

## Decisions

- **Host addresses and covering prefixes are distinct object types; do not
  conflate them.** The four `/32`/`/128` host addresses (192.175.48.1,
  192.31.196.1, 2620:4f:8000::1, 2001:4:112::1) are what the DNS server binds
  on `lo` (child 1/2's registry). The four `/24`/`/48` covering prefixes
  (192.175.48.0/24, 192.31.196.0/24, 2620:4f:8000::/48, 2001:4:112::/48) are
  what BGP actually announces (child 3, RFC 7534 §3.4) -- announcing the host
  addresses instead would be wrong (widely filtered, not what §3.4 requires).
  Getting this distinction backwards is an easy mistake since both are "the
  as112 addresses" in casual conversation.
- Nearly the entire feature is composition of existing, already-tested ze
  mechanisms (BGP communities, `local-as` override, watchdog/healthcheck) --
  the only genuinely new code is the DNS plugin and a small generic iface
  registry, both independently useful beyond this feature. This is why
  spec-as112-3 has zero new as112-specific Go code: its "implementation" is a
  correctly-composed worked example plus tests proving the composition
  actually behaves as claimed.
- `asn.local 112 replace-as` is a global-routing foot-gun: applied to a
  publicly-peered group, it injects the node into the GLOBAL AS112 anycast
  system with no RFC 7534 §3.2/§5 community coordination (a requirement that
  is not software-enforceable). Default behavior uses the operator's own
  ASN (local-use mirror); the override is per-group opt-in with a hard
  warning in the docs, not a silent default.
- Only one address per service per family (not the secondary IANA
  BLACKHOLE-1/2 addresses) and no NSID (RFC 5001) support -- both explicit,
  user-confirmed scope cuts, not oversights. The HOSTNAME.AS112.NET/ARPA TXT
  zone (operator-configured `hostname` leaf) is the v1 node-identification
  mechanism; NSID is a possible future addition, not a gap in this pass.

## Consequences

- A single `docs/guide/as112.md` page (written by spec-as112-3, since none of
  the three children had created it yet when each reached its own
  documentation phase) covers the whole feature -- each section still traces
  to the spec that owns its content (config reference -> as112-2, BGP worked
  example -> as112-3, RFC Compliance Mapping -> this umbrella).
- The generic address-ownership registry (spec-as112-1) and the shared DNS
  harness (`internal/core/dnsserver`, from the already-closed
  spec-dns-server-harness) both outlive this feature as reusable
  infrastructure -- `cos` became the registry's second (partial) consumer
  during the later review-hardening pass ([1032](1032-as112-review-hardening.md)).
- Every one of the ~15 real bugs found across the three children's adversarial
  review rounds was found by DEDICATED review passes, not the original TDD
  cycles that produced the code -- the tests that existed at each point were
  internally consistent with the (buggy) design, not with the real
  requirement. This is not specific to AS112; it's a recurring pattern
  worth remembering for any spec with non-trivial concurrency or shared
  state (see 1028's own "General lesson" gotcha for the sharpest version of
  this observation).

## Files

See each child's own Files section:
[1028](1028-as112-1-iface-address-registry.md),
[1033](1033-as112-2-dns-server.md),
[1034](1034-as112-3-bgp-integration.md),
[1032](1032-as112-review-hardening.md).
Umbrella-only: `docs/guide/as112.md` (RFC Compliance Mapping section),
`docs/features.md` (AS112 row).

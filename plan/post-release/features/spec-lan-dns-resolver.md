# Spec: lan-dns-resolver

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release (the fttx-edge profile's LAN DNS resolver); existing supported behavior and its obligations remain required. |
| RFC gaps | RFC4035-3-5, RFC4035-3.1.6-6, RFC4035-3.2.1-1, RFC4035-3.2.1-2, RFC4035-3.2.1-3, RFC4035-3.2.2-1, RFC4035-3.2.2-4, RFC4035-3.2.3-2, RFC4035-4.2-1, RFC4035-4.2-3, RFC4035-4.2-5, RFC4035-4.2-6, RFC4035-4.3-1, RFC4035-4.4-1, RFC4035-4.7-2, RFC4035-4.7-3, RFC4035-4.7-7, RFC4035-4.8-1, RFC4035-5-1, RFC4035-5-2, RFC4035-5-3, RFC4035-5.2-1, RFC4035-5.2-3, RFC4035-5.3.1-1, RFC4035-5.3.1-2, RFC4035-5.3.1-3, RFC4035-5.3.1-4, RFC4035-5.3.1-5, RFC4035-5.3.1-6, RFC4035-5.3.1-7, RFC4035-5.3.1-8, RFC4035-5.3.1-9, RFC4035-5.3.1-10, RFC4035-5.3.2-1, RFC4035-5.3.2-2, RFC4035-5.3.2-3, RFC4035-5.3.3-1, RFC4035-5.3.3-2, RFC4035-5.4-1, RFC4035-5.4-2, RFC4035-5.4-3, RFC4035-5.4-4, RFC4035-5.5-2, RFC4035-5.5-3, RFC2181-5.3.1-2, RFC2181-5.3.1-3, RFC2181-5.3.1-4, RFC2181-5.4.1-3, RFC2181-5.4.1-8, RFC2181-5.4.1-9 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The `fttx-edge` use-case profile (`rfc/profiles/fttx-edge.md`, "FTTx edge:
customers to the internet over L2TP, local DNS for the LAN") reaches RFC 1035,
RFC 2181 and RFC 4035 as roots, but Ze has no DNS server that answers a
customer's recursive query. Today Ze answers DNS only as an authority (the
`as112` and `geodns` plugins over `internal/core/dnsserver`, which clears RA on
every reply by design), and resolves names only for itself (the stub in
`internal/component/resolve/dns`). The owner decision of 2026-10-25 recorded in
`plan/release/delivery/spec-rfc-priority-tree.md` reports this gap at 0% until
he decides on it.

Goal: Ze serves customer LANs (subscriber sessions over L2TP and PPPoE, and
DHCP-addressed LAN segments) as a caching, DNSSEC-validating resolver, in a
forwarding mode (to operator-named upstreams) and a full-recursion mode (from
the root), answering only on the addresses and to the source prefixes the
operator names. The address it serves on is what Ze already advertises through
DHCPv4 option 6, the RA RDNSS option, and IPCP Primary/Secondary DNS.

No existing spec covers a resolver: `./le spec status` and a search of every
work-item folder for "resolver", "recursive" and "unbound" find only the stub
resolver's own work and the paused DNS specs listed below.

## Owner questions

Each question carries the reading the design proceeds under. None changes the
AC set except Q-2, which adds one row if the answer is yes.

| # | Question | Evidence | Reading the design takes |
|---|----------|----------|--------------------------|
| Q-1 | DNS work is owner-paused (`plan/proposals/spec-fixit-dns-rfc1035-conformance.md`, `plan/release/validation/spec-dns-existing-support-proof.md`: "The DNS pause still applies"), and `plan/release/fixes/spec-firewall-dnssec-membership-validation.md` (P0, blocked) owns WP-DNSSEC: "local DNSSEC signature validation ... Trust-anchor policy and the treatment of unsigned answers need design when the owner reopens DNS work". This spec designs exactly that validator (`internal/core/dnssec`, shared by the resolver plugin and the stub that feeds the firewall). Does this spec run before the pause lifts, and does WP-DNSSEC consume this validator (making this spec, or its Phase 2, a dependency of that P0 spec)? | The three spec files above; `internal/component/resolve/dns/resolver.go` `dnssecDecision` trusts the upstream's SERVFAIL, which is the reliance RFC4035-4.9.3-2 forbids | The spec stays post-release P3 and paused with DNS until the owner says otherwise. Phase 2 builds the validator as a core package with no plugin vocabulary, so WP-DNSSEC can consume it without a redesign |
| Q-2 | Trust-anchor rollover. RFC 4035 Section 4.4: "the resolver SHOULD have some reasonably robust mechanism for obtaining such keys when it boots". The robust mechanism in practice is RFC 5011 automated rollover, which no summary enrols, and RFC enrolment is frozen (owner, 2026-10-08). Ship static anchors only, or implement RFC 5011 without a summary? | `rfc/short/` holds no rfc5011; `rfc/full/rfc4035.txt` Section 4.4 | Static anchors: the two current root KSK DS records built in, replaceable by `dnssec trust-anchor` config. A root KSK roll then needs a Ze release or a config change; recorded under Known Limitations |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/dns/server-harness.md` - the shared listener, the enforced answer shape, bind policy
  → Constraint: `Authoritative` owns the single `WriteMsg` and re-asserts `shapeAuthoritative` (RA=0, AD=0, AA by RCODE) after the answer function; a resolver cannot use it. A second wrapper `Recursive` in the same package owns the recursive shape the same way (answer function gets `Peer`, never the `ResponseWriter`)
  → Constraint: every consumer names its `BindPolicy` (lenient at boot, strict on reload); a reload that changes the listener set pauses serving; a reload that changes only data must not touch listeners (`reconcile` short-circuits on an unchanged endpoint signature)
  → Decision: the harness stays vocabulary-free; ACL prefixes go through the existing `Matcher` (`Entry{Prefix, Label}`)
- [ ] `ai/digests/dns-services.md` - traced flow of as112 and geodns over the harness
  → Constraint: plugin shape to copy: `register.go` init registration with `RunEngine`, `OnConfigure` parses and publishes an atomic state snapshot the query path reads lock-free, then `ApplyWithSecure`; `DoctorChecks` probe bind capability; metrics default to a no-op registry before `ConfigureMetrics`
  → Constraint: as112's `allow-from` empty means allow everyone; that default is wrong for a resolver (open resolver), so the resolver refuses a commit with an empty `allow-from`
- [ ] `docs/features/dns-resolver.md` - the client-side stub (`system dns`), its cache and `show/clear dns cache`
  → Constraint: the stub is a different subsystem with its own cache and commands; the LAN resolver shares neither the cache nor the `dns cache` command words, and it must not change what `system name-server` does
- [ ] `docs/architecture/testing/interop.md` - suites, scenario naming, checkers, discrimination
  → Constraint: scenario directories are named, no numeric prefix; checkers are typed Go under `internal/le/interoplab/`; the dnsmasq DHCP scenarios already live in `test/interop/scenarios/` with checkers in `internal/le/interoplab/bgp/check_dhcp.go`, so DNS resolver scenarios join that suite
  → Constraint: a scenario counts only after a recorded red: revert the producing change, rebuild, observe red, restore
- [ ] `rfc/profiles/fttx-edge.md` - the profile this closes
  → Decision: roots rfc1035, rfc2181, rfc4035 are in the profile; RFC 8106 is listed Absent, and the RA RDNSS producer already exists (`internal/core/ndp/ra.go` `OptRDNSS`), so no new summary is written (enrolment frozen)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4035.md` - DNSSEC protocol modifications
  → Constraint: every recursive-server and validator row is `{not-applicable: ze's DNS servers never recurse ...}` or `{not-applicable: ze runs no local validator ...}` today; this spec turns each listed in `RFC gaps` into a tagged, discriminated test and rewrites its annotation
  → Constraint: RFC4035-3-6 is `{single-polarity: positive}` today because no path consults DO; the resolver's strip path gives it a negative polarity, so its test gains a resolver case, but it is not listed as a gap this spec closes
  → Constraint: RFC4035-3-1 and RFC4035-3-2 (EDNS0 on the name server, at least 1220 octets) are open `{gap}` rows on the shipped authoritative servers; they are not claimed here (they belong to the shipped harness), but the resolver listener must meet both from its first commit
- [ ] `rfc/short/rfc2181.md` - DNS clarifications
  → Constraint: 5.3.1-2/3/4, 5.4.1-3, 5.4.1-8 and 5.4.1-9 are `{not-applicable}` because Ze is "authoritative plus a stub resolver only"; a caching resolver makes each applicable
- [ ] `rfc/short/rfc1035.md` - DNS
  → Constraint: no RFC 1035 row is closed by this spec (27 rows, all met or the paused zone transfer); the resolver's new producer must still meet 2.3.3-1, 3.1-5 (case-insensitive compare, applies to the cache key), 4.1.1-1 (Z zero), 4.1.1-2 (AA meaning), 4.1.4-5 (pointers understood), 4.2.1-1/2 (UDP bound and TC), 4.2.1-3/4.2.2-1 (port 53, TCP length prefix), 6.4-1 (Not Implemented), and each tagged test for those rows gains a resolver-side case
- [ ] `rfc/short/rfc2132.md`, `rfc/short/rfc1877.md` - how the address is advertised
  → Constraint: advertisement already exists (DHCPv4 option 6 `dns-server`, IPCP 129/131 via the L2TP pool's `dns-primary`/`dns-secondary`); this spec adds no advertisement code

**Key insights:**
- Ze has three producers that advertise a resolver address and no resolver behind any of them.
- The harness forbids recursion by construction; the resolver needs a second shaping wrapper, not a flag on the first.
- `github.com/miekg/dns` v1.1.73 (vendored) supplies the DNSSEC primitives (`RRSIG.Verify`, `RRSIG.ValidityPeriod`, `DNSKEY.ToDS`, `NSEC3.Cover`, `NSEC3.Match`) but no validator, no chain building, no iterative resolver.
- Unbound and BIND agree on the two silent points the RFCs leave open (AD on a reply, ACL default); their deciding code is quoted in Key Design Decisions.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/dnsserver/handler.go` - `Authoritative(log, fn, onPanic)` wraps an `AnswerFunc(msg, r, p Peer) (send bool)`; `shapeAuthoritative` sets AA unless RCODE 5, clears RA, Z, AD and compression; `send` truncates datagram replies with `udpReplyLimit` (512 floor, raised by an EDNS OPT); non-QUERY opcodes draw Not Implemented
- [ ] `internal/core/dnsserver/manager.go` - `New(log, handler dns.Handler, opts Options)`; `Options{Freebind, OnListenerChange, TLSMaterialResolver}`; UDP and TCP `dns.Server` per endpoint with `UDPSize` left zero (the RFC4035-3-2 gap)
- [ ] `internal/core/dnsserver/matcher.go`, `client.go` - longest-prefix `Matcher`, `RemoteAddr`
- [ ] `internal/component/resolve/dns/resolver.go` - stub: `Resolver{client, server, cache, dnssec}`; `Resolve` caches string renderings of answer records; `dnssecDecision` treats upstream SERVFAIL as the DNSSEC failure signal (permissive/strict); no local validation
- [ ] `internal/plugins/geodns/yang/ze-geodns-conf.yang` - `service { geodns { enabled; listener <name> { uses zt:listener } ... } }`
- [ ] `internal/component/config/listener.go` - `DiscoverListenerServices` walks every YANG `zt:listener` user so the cross-service port-conflict check covers new services by registration
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `dns-server` leaf-list, IPv4, option 6, at most 63
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - RA `rdnss { server [..] (max 8); lifetime }`
- [ ] `internal/component/l2tp/plugins/pool/register.go`, `internal/component/l2tp/ppp/ncp.go` - pool `dns-primary`/`dns-secondary` carried into IPCP NAK/ACK
- [ ] `internal/le/mock/dns/register.go` - `le mock dns`, the functional-test upstream stub

**Behavior to preserve:**
- `Authoritative` and `shapeAuthoritative` unchanged; as112 and geodns replies byte-identical.
- The stub resolver, `system name-server`, resolv.conf writing, `show dns cache` and `clear dns cache` unchanged.
- The listener conflict check keeps refusing two services on one address and port.
- DHCP, RA and IPCP advertisement code and YANG unchanged.

**Behavior to change:**
- New `service dns-resolver` answers recursive queries from allowed sources on configured listeners.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `service { dns-resolver { ... } }` committed through the standard transaction.
- Wire: a DNS query (UDP or TCP, port 53 by default) from a LAN host or subscriber to a configured listener address.

### Transformation Path
1. `OnConfigure` parses the section, validates, builds a state snapshot (allow matcher, mode, upstreams or root hints, anchors, cache limits) and publishes it atomically; then reconciles listeners through `Manager.ApplyWithSecure` (lenient at boot, strict on reload).
2. A query reaches `dnsserver.Recursive`, which applies the recursive shape (QR, RA=1, AA=0, CD copied, Z clear) and calls the resolver's answer function with `Peer`.
3. Answer function: opcode check (harness), ACL by packet source through `Matcher` (miss: REFUSED, counted, no upstream query), then cache lookup keyed by lower-cased name, type, class, and the CD bit.
4. Cache miss: the resolver side runs one resolution (forward: query the upstream list; recursive: iterate from root hints), always with DO=1 and CD=1 upstream, coalescing identical in-flight resolutions, inside a per-query work budget.
5. Validation (`internal/core/dnssec`): build the chain from the configured anchor through DS and DNSKEY to the answer's signer; classify secure, insecure, bogus or indeterminate; verify NSEC or NSEC3 denial for negative answers.
6. Cache insert: whole RRsets replaced, never merged; bogus results to a BAD cache with a fixed 60 s TTL; TTLs capped by `cache max-ttl`.
7. Reply: bogus with CD=0 draws SERVFAIL; otherwise answer; AD set iff secure and the query had DO or AD; DNSSEC RRs stripped when DO=0 unless the query asked for that type; harness bounds UDP size and sets TC; single write.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | YANG `service dns-resolver`, SDK `OnConfigure` JSON section | No |
| Plugin to harness | `dnsserver.New` + `dnsserver.Recursive` answer function, `ApplyWithSecure` | No |
| Plugin to validator | `internal/core/dnssec` Go API (core package, no plugin import) | No |
| Ze to upstream / authorities | UDP and TCP DNS from an ephemeral, per-query source port with a random ID | No |
| CLI to plugin | `show dns-resolver`, `clear dns-resolver cache` registered RPCs | No |

### Integration Points
- `internal/core/dnsserver` - new `Recursive` wrapper and an `Options.UDPSize` the resolver sets to 1232 (the listener reads at least 1220 octets, RFC4035-3-2 for this listener)
- `internal/component/config/listener.go` - automatic via `zt:listener` registration; no list edit
- `internal/component/plugin/all/all.go` - regenerated by `./le repo update`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to prove: query path only through `Recursive`; config only through `OnConfigure` |
| No unintended coupling (components stay isolated) | No | to prove: plugin imports `internal/core/dnsserver` and `internal/core/dnssec` only; no import of `geodns`, `as112` or `component/resolve` |
| No duplicated functionality (extends existing, does not recreate) | No | to prove: listener lifecycle, ACL matcher, opcode check, UDP bound reused from the harness |
| Zero-copy preserved where applicable (refs, not copies) | No | cache stores packed RRsets once; reply assembly copies RR pointers, not wire bytes |
| Registration over hardcoding, outbound | No | plugin registers via `init()`; commands via YANG `ze:command`; doctor checks via `DoctorChecks` |
| Registration over hardcoding, inbound | No | to search at implementation: listener conflict list (`DiscoverListenerServices`), doctor code table (`internal/core/diagnostic/codes.go`), plugin inventory docs, completion; each must derive from registration |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | miekg/dns `RRSIG.Verify` covers RSASHA256, RSASHA512, ECDSAP256SHA256, ECDSAP384SHA384, ED25519 (the algorithms the root and TLDs use) | `vendor/github.com/miekg/dns/dnssec.go` `Verify` | a missing algorithm makes a zone unverifiable; RFC4035-5.2-4 then treats it as insecure, which must be implemented, not a bogus | unit test per algorithm with keys generated by miekg | unvalidated |
| A-2 | The listener conflict check refuses the resolver on an address and port geodns or as112 already binds, with no edit to `listener.go` | `DiscoverListenerServices` walks every `zt:listener` user | two services race for port 53 | functional test `dns-resolver-listener-conflict.ci` | unvalidated |
| A-3 | An L2TP subscriber reaches a resolver bound on an LNS loopback or gateway address; binding per ppp interface is not needed | subscriber sessions are dynamic interfaces; IPCP hands out an address, not an interface | an operator needs per-session binding; ACL by source prefix still holds | interop scenario `dns-resolver-l2tp-subscriber` | unvalidated |
| A-4 | `dns.Client` uses a fresh ephemeral UDP port and a random 16-bit ID per exchange and drops a reply whose ID does not match | `vendor/github.com/miekg/dns/client.go` | cache poisoning by off-path spoofing | unit test that injects a mismatched ID and a reply from another source | unvalidated |
| A-5 | The host clock is close enough for RRSIG validity windows at boot | NTP client plugin persists time (`internal/plugins/ntp`) | every signed answer bogus until sync | R-3 mitigation test | unvalidated |
| A-6 | NSEC3 (RFC 5155) denial proofs are needed for correct operation (most TLDs, `.com` included, use NSEC3), and implementing them needs no summary | real zones; enrolment frozen applies to summaries, not code | without NSEC3 every unsigned delegation under an NSEC3 parent is bogus, which is a logic defect | interop zone signed with NSEC3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Open resolver on the WAN (amplification, abuse) | queries from unexpected sources in `ze_dns_resolver_refused_total` | wildcard listen address refused at commit; empty `allow-from` refused at commit; REFUSED (not answer) outside the ACL |
| R-2 | Cache poisoning | answers that disagree with Unbound in the cross-check scenario | random ID and port (A-4), bailiwick check on every RR (records outside the zone being asked are dropped), validation |
| R-3 | Clock unsynced at boot makes signed answers bogus | SERVFAIL burst after boot, `validation_total{outcome="bogus"}` with reason `signature-expired` | log names the clock as the reason; doctor check `doctor-dns-resolver-clock-unsynced`; never downgrade bogus to insecure |
| R-4 | Validation work amplification (deep chains, many keys, NSEC3 iterations) | CPU per query, `upstream_queries_total` per client query | per-query budget (upstream queries, CNAME restarts, signatures tried), NSEC3 iteration cap treated as insecure above the cap (RFC 9276 practice, Unbound `val-nsec3-keysize-iterations`), coalescing |
| R-5 | Cache memory growth | `ze_dns_resolver_cache_entries` at limit | entry-count bound with LRU eviction; BAD cache bounded too |
| R-6 | Reload pauses serving | dropped queries during commit | data-only changes never touch listeners (signature unchanged); listener changes follow harness strict policy |
| R-7 | Upstream unreachable read as "no such name" | NOERROR empty or NXDOMAIN on timeout | timeout and SERVFAIL are distinct outcomes; never cache a timeout as negative |
| R-8 | TCP fallback missing breaks large signed answers | TC replies loop | resolver side retries over TCP on TC; client side gets TC and the TCP listener |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Name resolution for every customer behind the box; a validation defect fails resolution (SERVFAIL) rather than spoofing it, by design. Default off, so nothing changes until configured |
| How is it reverted? | Single commit revert; config section removed |
| Who else touches this path? | `internal/core/dnsserver` (as112, geodns), the stub resolver, WP-DNSSEC (`spec-firewall-dnssec-membership-validation`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service dns-resolver` forward config + query from allowed source | → | `dnsresolver` answer function via `dnsserver.Recursive` | `test/plugin/dns-resolver-forward.ci` |
| query from a source outside `allow-from` | → | ACL in the answer function | `test/plugin/dns-resolver-acl-refused.ci` |
| commit with wildcard listener or empty `allow-from` | → | `parseConfig` refusal | `test/plugin/dns-resolver-config-refused.ci` |
| signed zone from `le mock dns` signed mode | → | `internal/core/dnssec` chain validation | `test/plugin/dns-resolver-dnssec-secure.ci` |
| tampered signature | → | bogus path, SERVFAIL / CD=1 passthrough | `test/plugin/dns-resolver-dnssec-bogus.ci` |
| `show dns-resolver`, `clear dns-resolver cache all` | → | `show.go` handlers | `test/plugin/dns-resolver-show.ci` |
| recursive mode with lab root hints | → | iterative resolution | interop `dns-resolver-recursive-knot` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `dns-resolver` enabled, `mode forward`, upstream answers A for a name; query from an allowed source | NOERROR with the upstream's records; RA=1, AA=0, Z=0; answer TTL no greater than the upstream's |
| AC-2 | Query from a source outside every `allow-from` prefix | REFUSED, `ze_dns_resolver_refused_total` +1, no upstream query sent (upstream query counter unchanged) |
| AC-3 | Commit with a listener `ip` of `0.0.0.0` or `::` | Commit refused; message names the listener, says a wildcard would answer on the WAN, and names the fix (a LAN or loopback address) |
| AC-4 | Commit with `enabled true` and no `allow-from` entry | Commit refused; message says the resolver would refuse every client and names the leaf to set |
| AC-5 | Same query repeated within the TTL | Second answer from cache: no upstream query, TTL reduced by the elapsed seconds |
| AC-6 | Upstream returns a new RRset for a cached name and type | Cached RRset replaced whole, never merged with the old records (RFC2181-5.4-1) |
| AC-7 | Signed zone with a valid chain from the configured anchor; query DO=1 | NOERROR, AD=1, RRSIGs in the answer section |
| AC-8 | Same zone; query DO=0 AD=0 | AD=0, no RRSIG, NSEC or NSEC3 in any section |
| AC-9 | Same zone; query DO=0 AD=1 | AD=1, DNSSEC records stripped |
| AC-10 | Query DO=0 for type RRSIG or DNSKEY explicitly | Those records returned, not stripped (RFC4035-3.2.1-3) |
| AC-11 | Signed zone with a tampered RRSIG; query CD=0 | SERVFAIL (RFC4035-5.5-2), `validation_total{outcome="bogus"}` +1 |
| AC-12 | Same; query CD=1 | Full response, AD=0 (RFC4035-5.5-3) |
| AC-13 | Signed zone whose upstream strips RRSIGs | SERVFAIL; absence of signatures is not taken as insecure (RFC4035-5-3) |
| AC-14 | Unsigned child of a signed parent, proven by an authenticated NSEC and, separately, NSEC3 record that no DS exists | NOERROR, AD=0 (insecure, not bogus) |
| AC-15 | Name absent from a signed zone, NSEC and NSEC3 variants | NXDOMAIN with AD=1 when DO=1, denial records validated; a forged NXDOMAIN without valid proof draws SERVFAIL |
| AC-16 | Bogus answer queried again within 60 s | SERVFAIL from the BAD cache with no upstream query; after 60 s, refetched; a BAD entry is never returned to a CD=0 query as data (RFC4035-4.7-3, 4.7-7) |
| AC-17 | `dnssec trust-anchor` set to a lab DS record | The lab hierarchy validates; the built-in root anchors are not used; a malformed DS refuses the commit naming the value |
| AC-18 | `dnssec validation off` | No validation, AD never set, no SERVFAIL for a bogus zone; the resolver side still sends DO=1 (RFC4035-3.2.1-1) |
| AC-19 | `mode recursive` with root hints pointing at a lab root | Resolution follows delegations from the root; DS for a child is asked of the parent's servers (RFC4035-4.2-6, 5.2-1) |
| AC-20 | CNAME loop, or a chain longer than the budget | SERVFAIL within the configured timeout; upstream queries for that client query never exceed the budget (RFC4035-5.4-3) |
| AC-21 | Query with RD=0 | Answered from cache when present; otherwise resolved as with RD=1 (Unbound behavior, quoted below) |
| AC-22 | UDP query with EDNS buffer 1232 and an answer larger than 1232 octets | Reply carries OPT, is at most 1232 octets and has TC=1; the same query over TCP returns the whole answer |
| AC-23 | UDP query of 1220 octets or more (EDNS) | Read whole and answered (listener `UDPSize` at least 1220) |
| AC-24 | Upstream unreachable | SERVFAIL after the timeout; nothing cached as negative |
| AC-25 | Reply from upstream with a mismatched ID, or from a different source address or port | Ignored; the resolver keeps waiting or times out |
| AC-26 | Upstream answer carrying records outside the queried zone's bailiwick | Those records are neither cached nor returned |
| AC-27 | Resolver listener on an address and port another DNS service binds | Commit refused by the listener conflict check, naming both services |
| AC-28 | `dns-resolver` absent or `enabled false` | No listener bound; `system name-server` and the stub unchanged |
| AC-29 | Reload changing only `allow-from` or upstreams | New policy on the next query; listeners not rebound (no serving pause) |
| AC-30 | `show dns-resolver` | Listeners and their state, mode, upstreams or root-hint source, anchor key tags, cache entries and hit rate, validation outcome counts; `clear dns-resolver cache all` empties both caches |
| AC-31 | LAN host gets a DHCPv4 lease from Ze with `dns-server` set to the resolver address, then resolves a name | The query reaches the resolver and is answered |
| AC-32 | L2TP subscriber gets the resolver address via IPCP from the pool's `dns-primary` and resolves a name | Answered; the subscriber's pool prefix is in `allow-from` |
| AC-33 | Inverse query, or any opcode other than QUERY | Not Implemented, as the authoritative services (RFC1035-6.4-1) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator enables a forwarding resolver for a LAN and points DHCP at it | config → listener bound → DHCP option 6 → host query → ACL → cache/forward → reply | `test/plugin/dns-resolver-dhcp-lan.ci`, interop `dns-resolver-forward-unbound` |
| 2 | FTTx subscriber over L2TP resolves names with DNSSEC protection | IPCP DNS → query to LNS address → validate → AD reply | interop `dns-resolver-l2tp-subscriber` |
| 3 | Attacker zone serves a bad signature | upstream → validator bogus → SERVFAIL to the customer | interop `dns-resolver-recursive-knot` (bogus zone), `test/plugin/dns-resolver-dnssec-bogus.ci` |
| 4 | Operator runs full recursion with no upstream | root hints → iterative → reply | interop `dns-resolver-recursive-knot` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecursiveShape` | `internal/core/dnsserver/recursive_test.go` | RA=1, AA=0, Z=0, CD copied, answer function cannot set AA or clear RA | |
| `TestRecursiveADRule` | `internal/core/dnsserver/recursive_test.go` | AD set iff secure and (DO or AD in query) | |
| `TestRecursiveUDPSize` | `internal/core/dnsserver/recursive_test.go` | listener reads 1232-octet query; OPT on reply | |
| `TestChainSecure`, `TestChainBogusSignature`, `TestChainExpired`, `TestChainMissingSignatures` | `internal/core/dnssec/chain_test.go` | RFC4035-5-1/5-2/5-3, 5.3.1-1..10 | |
| `TestNSECDenial`, `TestNSEC3Denial`, `TestNSEC3IterationCap` | `internal/core/dnssec/denial_test.go` | RFC4035-5.4-1..4, A-6, R-4 | |
| `TestWildcardExpansion` | `internal/core/dnssec/wildcard_test.go` | RFC4035-4.2-3, 5.3.2-1, 5.3.3-1 | |
| `TestDNAMESynthesizedCNAME` | `internal/core/dnssec/dname_test.go` | RFC4035-4.8-1 | |
| `TestTTLClampToRRSIG` | `internal/core/dnssec/ttl_test.go` | RFC4035-5.3.3-2 | |
| `TestTrustAnchorParse` | `internal/core/dnssec/anchor_test.go` | RFC4035-4.4-1, built-in root DS, malformed refused | |
| `TestUnsupportedAlgorithmInsecure` | `internal/core/dnssec/chain_test.go` | RFC 4035 Section 5.2 unsupported algorithms are insecure, not bogus | |
| `TestCacheReplaceNotMerge`, `TestCacheCaseFold`, `TestCacheTTLDecrement` | `internal/plugins/dnsresolver/cache_test.go` | RFC2181-5.4-1, RFC1035-3.1-5, AC-5 | |
| `TestBadCacheTTL`, `TestBadCacheNotServedCD0` | `internal/plugins/dnsresolver/cache_test.go` | RFC4035-4.7-2/3/7, 3.2.2-4 | |
| `TestStripDNSSECWhenDOClear`, `TestKeepRequestedDNSSECType` | `internal/plugins/dnsresolver/answer_test.go` | RFC4035-3.2.1-2/3, 3-5, 3-6 | |
| `TestUpstreamDOAndCD` | `internal/plugins/dnsresolver/forward_test.go` | RFC4035-3.2.1-1, 3.2.2-1 | |
| `TestBailiwick`, `TestIDMismatchIgnored` | `internal/plugins/dnsresolver/forward_test.go` | AC-25, AC-26, A-4 | |
| `TestWorkBudget` | `internal/plugins/dnsresolver/iterate_test.go` | RFC4035-5.4-3, AC-20 | |
| `TestDSFromParent` | `internal/plugins/dnsresolver/iterate_test.go` | RFC4035-4.2-5, 4.2-6, 5.2-1, 5.2-3 | |
| `TestTrustRanking` | `internal/plugins/dnsresolver/cache_test.go` | RFC2181-5.4.1-3, 5.4.1-8 (authenticated beats unauthenticated, answer beats glue) | |
| `TestSIGRRSetHandling` | `internal/plugins/dnsresolver/cache_test.go` | RFC2181-5.3.1-2/3/4 | |
| `TestParseConfigRefusals` | `internal/plugins/dnsresolver/config_test.go` | AC-3, AC-4, AC-17 malformed | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| listener `port` | 1-65535 | 65535 | 0 | 65536 |
| `cache size` (entries) | 0-10000000 | 10000000 | N/A (0 disables) | 10000001 |
| `cache max-ttl` (s) | 0-2147483647 | 2147483647 | N/A | 2147483648 |
| `timeout` (s) | 1-60 | 60 | 0 | 61 |
| `forward server` count | 1-8 | 8 | 0 (with `mode forward`) | 9 |
| `dnssec trust-anchor` count | 1-16 | 16 | N/A | 17 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-resolver-forward` | `test/plugin/dns-resolver-forward.ci` | allowed client resolves through `le mock dns` upstream; second query cached | |
| `dns-resolver-acl-refused` | `test/plugin/dns-resolver-acl-refused.ci` | outside client refused, upstream untouched | |
| `dns-resolver-config-refused` | `test/plugin/dns-resolver-config-refused.ci` | wildcard listener, empty allow-from, malformed anchor refused with fix | |
| `dns-resolver-listener-conflict` | `test/plugin/dns-resolver-listener-conflict.ci` | same address and port as geodns refused | |
| `dns-resolver-dnssec-secure` | `test/plugin/dns-resolver-dnssec-secure.ci` | signed mock zone, DO and AD variants | |
| `dns-resolver-dnssec-bogus` | `test/plugin/dns-resolver-dnssec-bogus.ci` | tampered RRSIG: SERVFAIL; CD=1 passthrough; BAD cache | |
| `dns-resolver-show` | `test/plugin/dns-resolver-show.ci` | `show dns-resolver` and `clear dns-resolver cache all` | |
| `dns-resolver-reload` | `test/plugin/dns-resolver-reload.ci` | allow-from change applies without rebinding | |
| `dns-resolver-dhcp-lan` | `test/plugin/dns-resolver-dhcp-lan.ci` | DHCPv4 lease option 6 names the resolver and the client resolves | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dns-resolver-forward-unbound` | `test/interop/scenarios/dns-resolver-forward-unbound/` | Unbound (latest stable) as upstream, Knot DNS as signed lab root and zones, `kdig`/`delv` client | Ze forwards and validates against a real upstream; AD and RCODE per query equal Unbound's own answers to the same queries with the same anchor | |
| `dns-resolver-recursive-knot` | `test/interop/scenarios/dns-resolver-recursive-knot/` | Knot DNS authoritative hierarchy (lab root, `test.`, signed NSEC zone, signed NSEC3 zone, unsigned child, bogus zone) | full recursion from lab root hints; secure, insecure and bogus outcomes; DS from parent; cross-check against Unbound in recursive mode | |
| `dns-resolver-l2tp-subscriber` | `test/interop/scenarios/dns-resolver-l2tp-subscriber/` | xl2tpd + pppd LAC (existing L2TP lab image) | subscriber gets the resolver address through IPCP and resolves through it | |

### Future (if deferring any tests)
- None deferred.

## Files to Modify
- `internal/core/dnsserver/handler.go` - add `Recursive` wrapper and `shapeRecursive` (owns RA, AA, CD, AD, Z); `Authoritative` untouched
- `internal/core/dnsserver/manager.go` - `Options.UDPSize` passed to `dns.Server.UDPSize`
- `internal/le/mock/dns/register.go` (and its handler files) - signed-zone mode: generate a key at start, sign a fixture zone, optional tampered signature and stripped-signature modes, print the DS for the test's trust anchor
- `internal/le/interoplab/bgp/` checker registry - add the three scenarios
- `rfc/short/rfc4035.md`, `rfc/short/rfc2181.md` - rewrite the annotations of every `RFC gaps` row to the tagged tests; Meta `Support coverage` and `Support remaining` updated (in-scope summaries, no new enrolment)
- `docs/architecture/dns/server-harness.md` - the second wrapper and why
- `docs/features/dns-resolver.md` - point at the LAN resolver as the server side
- `ai/digests/dns-services.md` - add the resolver flow
- `ai/INDEX.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/guide/plugins.md`, `docs/features/plugins.md`, `docs/plugin-overview.md`, `docs/comparison.md`, `docs/plugin-development/metrics.md`
- `internal/component/plugin/all/all.go` - regenerated

## Files to Create
- `internal/core/dnssec/` - `anchor.go` (built-in root DS for key tags 20326 and 38696, parse), `chain.go`, `rrsig.go`, `denial.go` (NSEC, NSEC3), `wildcard.go`, tests above
- `internal/plugins/dnsresolver/` - `register.go`, `config.go`, `config_tx.go`, `state.go`, `server.go` (answer function, ACL), `forward.go`, `iterate.go`, `roothints.go`, `cache.go`, `budget.go`, `show.go`, `doctor.go`, `metrics.go`, `yang/ze-dns-resolver-conf.yang`, `yang/ze-dns-resolver-cmd.yang`, tests
- `test/plugin/dns-resolver-*.ci` - the nine functional tests above
- `test/interop/scenarios/dns-resolver-*/` - three scenarios; `test/interop/Dockerfile.unbound`, `test/interop/Dockerfile.knot`
- `internal/le/interoplab/bgp/check_dns_resolver.go` - typed checkers
- `features/lan-dns-resolver.md` - feature record
- `docs/guide/dns-resolver.md` - operator guide (config, advertisement via DHCP, RA, IPCP; ACL; DNSSEC; trust anchors)
- `docs/architecture/dns/lan-resolver.md` - design (shape, cache, validation, budget)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang`, `ze-dns-resolver-cmd.yang` |
| YANG validation constraints | Yes | `zt:listener` ports, `range` on cache, timeout, counts; `max-elements` on forward servers (8) and anchors (16); prefix type on `allow-from` |
| YANG custom validators | Yes | wildcard listener refusal, empty `allow-from` refusal, DS record syntax (`ze:validate`) |
| CLI commands/flags | Yes | `show dns-resolver`, `clear dns-resolver cache all` via `ze:command` |
| CLI grammar (keyword before value) | Yes | `clear dns-resolver cache all`; bare `clear dns-resolver cache` refused as `clear dns cache` is |
| Editor autocomplete | Yes | automatic for enums (`mode`, `validation`); listener names by YANG |
| Functional test for new RPC/API | Yes | `test/plugin/dns-resolver-show.ci` |
| Pipe completeness | Yes | `show dns-resolver` output through `ApplyPipes` |
| Env var registration | No | no `environment/` leaf; all config is YANG under `service` |
| Doctor check for runtime dependencies | Yes | listen port 53: `doctor-dns-resolver-port-unavailable`; clock: `doctor-dns-resolver-clock-unsynced`; codes in `internal/core/diagnostic/codes.go`, unit and functional tests |
| Prometheus counters/metrics | Yes | `ze_dns_resolver_queries_total{transport,rcode}`, `ze_dns_resolver_refused_total`, `ze_dns_resolver_cache_entries{cache=answer|bad}`, `ze_dns_resolver_cache_hits_total`, `ze_dns_resolver_upstream_queries_total{result}`, `ze_dns_resolver_validation_total{outcome}`, listener gauge via `OnListenerChange` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/lan-dns-resolver.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`; `docs/architecture/config/syntax.md` N-A (no syntax change, a new section only) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (two RPCs) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/dns-resolver.md` (new) |
| 7 | Wire format changed? | No | DNS wire format unchanged; miekg codec |
| 8 | Plugin SDK/protocol changed? | No | uses existing SDK surfaces |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4035.md`, `rfc/short/rfc2181.md`, `rfc/short/rfc1035.md` (resolver-side cases on existing rows), `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/ci-format.md` (`le mock dns` signed mode; the latter is the `// Design:` doc of `internal/le/mock/dns`), `docs/architecture/testing/interop.md` (DNS scenarios, Unbound and Knot images) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/dns/server-harness.md`, `docs/architecture/dns/lan-resolver.md` |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/post-release/features/spec-lan-dns-resolver.md` at implementation; known: `docs/architecture/dns/server-harness.md`, `docs/guide/as112.md` (handler.go anchors) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/dns-resolver.md` examples stay valid; DHCP and RA guide examples gain a pointer to the resolver |

## Implementation Steps

1. **Phase: Wiring** -- plugin registers, YANG loads, listener binds, `Recursive` wrapper answers SERVFAIL for everything
   - Tests: `dns-resolver-forward.ci` (fails on SERVFAIL), `dns-resolver-config-refused.ci`, `TestRecursiveShape`
   - Files: `register.go`, `config.go`, `config_tx.go`, `server.go`, YANG, `handler.go`, `manager.go`
   - Verify: listener reachable; refusals pass; forward test red for the right reason
2. **Phase: Forwarding and cache** -- ACL, forward, cache, BAD cache skeleton, budget, show and clear
   - Tests: AC-1..6, AC-20..30 unit and functional tests
   - Files: `forward.go`, `cache.go`, `budget.go`, `show.go`, `metrics.go`, `doctor.go`
3. **Phase: Validator** -- `internal/core/dnssec`, anchors, chain, NSEC, NSEC3, wildcard, DNAME; resolver applies it; `le mock dns` signed mode
   - Tests: validator unit tests, AC-7..18, `dns-resolver-dnssec-*.ci`
4. **Phase: Recursion** -- root hints, iteration, DS from parent, glue and bailiwick
   - Tests: AC-19, `TestDSFromParent`, interop `dns-resolver-recursive-knot`
5. **Phase: Interop and advertisement stories** -- the three scenarios, cross-check against Unbound, DHCP and L2TP stories; red runs recorded with `./le rfc record`
6. **Phase: Summaries and docs** -- every `RFC gaps` row annotated to its tagged test with a discrimination record; pages listed above (each written in the phase whose code changes it, per `ai/rules/documentation.md`; this phase only closes the RFC rows)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each user story path runs end to end in a test |
| Correctness | AD never set on insecure or indeterminate data; bogus never downgraded to insecure; timeout never cached as negative; RRsets replaced, never merged |
| Naming | YANG `dns-resolver`, JSON keys kebab-case, metric prefix `ze_dns_resolver_` |
| Data flow | Validator package holds no plugin or config vocabulary; only `Recursive` writes replies |
| Rule: principles (no silent zero) | An indeterminate validation answers SERVFAIL, never NOERROR with data |
| Rule: rfc-compliance | Every implementing function quotes its RFC 4035 or RFC 2181 sentence; every caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | `ze plugin list` shows `dnsresolver`; `./le repo update` leaves `all.go` clean |
| Validator package | `./le job run` scoped test of `internal/core/dnssec` green |
| Functional tests | the nine `.ci` files pass under their scoped functional run |
| Interop scenarios | three scenarios green, each with a recorded red |
| RFC rows | `./le rfc check` green with every `RFC gaps` row tagged and discriminated |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | malformed queries, oversized names, label pointers looping (miekg), queries with QDCOUNT not 1 |
| Open resolver | wildcard refused, empty ACL refused, REFUSED outside ACL, no answer amplification to refused clients |
| Poisoning | random ID and port, bailiwick, validation, no merge of RRsets |
| Resource exhaustion | cache bound, BAD cache bound, in-flight coalescing, per-query budget, NSEC3 iteration cap, TCP connection limit from the harness |
| Fail-open | validator error paths return bogus or indeterminate (SERVFAIL), never secure |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

RFC sentences the design rests on, read from `rfc/full/`:

| RFC section | Sentence | Design consequence |
|-------------|----------|-------------------|
| RFC 1035 Section 4.1.1 | "RA Recursion Available - this be is set or cleared in a response, and denotes whether recursive query support is available in the name server." | `Recursive` sets RA on every reply; `Authoritative` keeps it clear |
| RFC 2181 Section 5.4 | "Servers must never merge RRs from a response with RRs in their cache to form an RRSet." | cache replaces whole RRsets |
| RFC 4035 Section 3.2.1 | "The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests, regardless of the state of the DO bit in the initiating request received by the name server side." | DO=1 upstream always, also with `validation off` |
| RFC 4035 Section 3.2.1 | "If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response but MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested." | strip on DO=0 unless the qtype is a DNSSEC type |
| RFC 4035 Section 3.2.2 | "The name server side MUST copy the setting of the CD bit from a query to the corresponding response." | `shapeRecursive` copies CD |
| RFC 4035 Section 3.2.3 | "The name server side of a security-aware recursive name server MUST NOT set the AD bit in a response unless the name server considers all RRsets in the Answer and Authority sections of the response to be authentic." | AD only on secure; mixed secure and insecure sections clear it |
| RFC 4035 Section 4.4 | "A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR and SHOULD be capable of being configured with multiple trusted public keys or DS RRs." | `dnssec trust-anchor` list, built-in root DS defaults |
| RFC 4035 Section 4.7 | "Since RRsets that fail to validate do not have trustworthy TTLs, the implementation MUST assign a TTL." | fixed 60 s BAD-cache TTL |
| RFC 4035 Section 5 | "The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists." | stripped signatures from a signed zone are bogus |
| RFC 4035 Section 5.4 | "As with all DNS operations, however, the resolver MUST bound the work it puts into answering any particular query." | per-query budget |
| RFC 4035 Section 5.5 | "If the validation was being done to service a recursive query, the name server MUST return RCODE 2 to the originating client.  However, it MUST return the full response if and only if the original query had the CD bit set." | SERVFAIL on bogus with CD=0; full response with CD=1 |
| RFC 2132 Section 3.8 | "The domain name server option specifies a list of Domain Name System (STD 13, RFC 1035 [8]) name servers available to the client." | the existing `dns-server` leaf is the DHCPv4 advertisement |
| RFC 1877 Section 1.1 | "This Configuration Option defines a method for negotiating with the remote peer the address of the primary DNS server to be used on the local end of the link." | the existing L2TP pool `dns-primary` is the subscriber advertisement |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A new system plugin `internal/plugins/dnsresolver` over the shared harness | extend geodns; a component | the harness exists for exactly this (`server-harness.md`, two consumers); a plugin cannot import a sibling; the resolver has a config-driven lifecycle, so it is a plugin, not core |
| Second harness wrapper `Recursive` beside `Authoritative` | a flag on `Authoritative`; give the resolver the `ResponseWriter` | the harness doc records that a convention-based guard was replaced by an enforced one after two reviews; a flag reopens that; one wrapper per shape keeps both enforced |
| Ze's own Go validator in `internal/core/dnssec` | ship Unbound in the appliance and proxy to it; a third-party Go validator | RFC 4035's summary declares Implementation `ze`; WP-DNSSEC needs a validator at the Go resolver boundary consumed by the firewall, so a separate daemon would leave that owed; core placement lets both consumers share it without a plugin import |
| Both forward and recursive modes, forward first (Phase 2), recursion after (Phase 4) | forward only; recursion only | FTTx operators commonly forward to their central resolvers; recursion removes the upstream dependency; the iterative-mode MUSTs (RFC4035-4.2-5, 4.2-6, 5.2-1) bind only recursion, so phasing keeps each phase's proof self-contained |
| Validate locally in forward mode, querying upstream with CD=1 | trust the upstream AD bit | RFC4035-4.6-3 makes a resolver disregard AD from an insecure channel; Unbound does the same: validator.c "User queries are validated with BIT_CD to make our cache clean" |
| AD on a reply iff secure and the query had DO or AD | AD only with DO | RFC 4035 is silent on DO=0 queries. Unbound `reply_info_answer_encode` (util/data/msgencode.c): `if(secure && (dnssec || (qflags&BIT_AD))) flags \|= BIT_AD;`. BIND `ns_query_start` (lib/ns/query.c): "Set wantad if the client has set AD in the query. This allows AD to be returned on queries without DO set." and `if (client->inner.wantdnssec \|\| client->inner.wantad) { message->flags \|= DNS_MESSAGEFLAG_AD; }`. Both agree, so Ze does the same |
| Outside the ACL: REFUSED, not silent drop | drop | RFCs silent. Unbound `deny_refuse` (daemon/worker.c) offers both, and its default for unlisted sources is refuse ("the 'refuse' defaults for /0 are now done per interface instead", `acl_list_apply_cfg`, daemon/acl_list.c). REFUSED lets a misconfigured customer see the cause; the reply is no larger than the query |
| Empty `allow-from` refuses the commit | allow-all (as112 default); allow loopback only (Unbound default allows 127.0.0.0/8 and ::1) | an LAN resolver that answers no LAN is a misconfiguration, and allow-all is an open resolver; refusing names the fix |
| Wildcard listener refused | allow wildcard with ACL | keeps the WAN side closed even if the ACL is wrong |
| RD=0 answered from cache, else resolved | REFUSED on RD=0 cache miss | Unbound `worker_handle_request` tries `answer_norec_from_cache` and on failure falls through to the normal resolution path ("answer norec from cache -- need to validate or not primed") |
| BAD-cache TTL fixed at 60 s | a config leaf | Unbound's default `cfg->bogus_ttl = 60` (util/config_file.c); RFC asks only that a TTL be assigned and be small; no operator need shown for a knob |
| NSEC3 validation included | NSEC only | A-6: without it most real TLDs fail; RFC 5155 has no summary and none is written (enrolment frozen), so its tests carry no RFC tag |
| Advertisement left to the existing leaves | a `self` keyword that resolves to the resolver address in DHCP, RA and the L2TP pool | the three producers live in other plugins and components; a cross-plugin lookup is an import the plugin rules forbid; the guide shows the three leaves |

## Known Limitations
- Static trust anchors only (Q-2): a root KSK roll needs a Ze release or a config change, until RFC 5011 is admitted.
- Upstream transport in forward mode is plain UDP and TCP; DNS over TLS to upstreams is not in this spec.
- DHCPv6 option 23 advertisement waits for `plan/post-release/features/spec-dhcpv6-server.md`; IPv6 hosts are served through RA RDNSS today.
- No RFC 1035, RFC 5155, RFC 5011 or RFC 8106 summary is written or enrolled (enrolment frozen); RFC 4035 and RFC 2181 rows carry the proof.
- RFC4035-3-1 and RFC4035-3-2 stay open for the authoritative services; the harness change here makes them closable there by setting the same option, which is that work's to do.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-33 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

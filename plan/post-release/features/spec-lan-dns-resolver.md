# Spec: lan-dns-resolver

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release (the fttx-edge profile's LAN DNS resolver); existing supported behavior and its obligations remain required. |
| RFC gaps | RFC2181-5.1-1, RFC2181-5.3.1-2, RFC2181-5.3.1-3, RFC2181-5.3.1-4, RFC2181-5.4.1-3, RFC2181-5.4.1-9 |
| Depends | - |
| Phase | 1/4 |
| Handoff | - |
| Updated | 2026-10-27 |
| Approved | Session f3ae7c00, 2026-10-27: Thomas chose to run this spec; the design is settled by the RFC 1034, 1035, 2181, 2308 and 5452 text and the Unbound behaviour quoted below |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions

One decision is open. It does not block this spec: every phase below is the
same under both answers, so this spec is implemented now and only the
recursion spec waits on it.

| # | Decision | Options | Evidence | Recommended default | Blocks this spec? |
|---|----------|---------|----------|---------------------|-------------------|
| D-2 | Does Ze also ship full recursion from the root, beside forwarding? | (a) Yes: `plan/post-release/features/spec-dns-resolver-recursion.md` runs after this spec. (b) No: that spec is withdrawn and forwarding is the only mode | Neither RFC 1035, RFC 2181 nor the profile title ("local DNS for the LAN") requires either mode. Unbound recurses by default and forwards when a `forward-zone` is set; no rule in `ai/META-DECISIONS.md` decides it | (a), ship it: recursion removes the dependency on an operator upstream | No |

Questions the previous draft asked that a rule or the owner's brief now answers (not put to Thomas):

| Former | Answer | Source |
|--------|--------|--------|
| Q-1: does this spec run before the DNS pause lifts | It runs: Thomas chose this spec to run for the fttx-edge profile on 2026-10-27. Whether the firewall spec consumes a validator is `plan/post-release/features/spec-dns-resolver-dnssec-validation.md`'s concern | The main session's brief for this design pass |
| D-1: does local DNSSEC validation belong to this spec | No, option (b): this resolver is not security-aware (upstream queries carry EDNS0 with DO clear, DNSSEC record types are ordinary RRsets, AD is never set). The validator, criteria V-1..V-15, the RFC 4035 rows, RFC2181-5.4.1-8 and the static-anchor answer to the former Q-2 moved to `plan/post-release/features/spec-dns-resolver-dnssec-validation.md`, which runs only once Thomas brings DNSSEC into scope | The fttx-edge profile lists RFC 4033, 4034, 4035 and 8106 under `Optional use cases` only. `rfc/short/rfc4034.md` Enrolment reason: "the owner ruled that DNSSEC is not urgent and lists RFC 4033 and RFC 4034 as optional only, never required, for the fttx-edge use case". Commit 5ff4834b67: "Owner decision: DNSSEC is not urgent." `ai/META-DECISIONS.md`: an optional RFC "counts toward the profile's percentage only once Thomas brings it into scope" |

## Corrections made in the 2026-10-27 design pass

Each corrects a statement of the previous draft against source.

| Previous statement | Correct | Evidence |
|--------------------|---------|----------|
| RFC 4035 is a root of the fttx-edge profile | Optional only; required DNS RFCs are 1035 and 2181 | `rfc/short/rfc4035.md` `Use cases` is `-`, `Optional use cases` is `fttx-edge` |
| RFC 8106 is listed Absent, no summary | `rfc/short/rfc8106.md` exists, Implementation `ze`, optional for fttx-edge | its Meta rows |
| "No RFC 1035 ... summary is written" (Known Limitations) | `rfc/short/rfc1035.md` exists, required for fttx-edge | its Meta rows |
| RD=0 cache miss is resolved as RD=1, citing Unbound | Unbound's default `allow` action REFUSES every RD=0 query; only `allow_snoop` answers one from cache. Ze refuses RD=0 (AC-12) | `daemon/worker.c` `worker_handle_request`: "If this request does not have the recursion bit set, verify ACLs allow the snooping." then `acl != acl_allow_snoop` logs "refused nonrec (cache snoop) query from"; unbound.conf `allow`: "Non-recursive queries are refused."; RFC 1034 Section 4.3.1: "the name server should never perform recursive service unless asked via RD" |
| `le mock dns` signed mode lives in `internal/le/mock/dns/register.go` and its handler files | That file only registers the command; the server is `internal/test/mock/dns/` (`server.go`, `zone.go`) | `gopls symbols` on both |
| The L2TP subscriber scenario uses "the existing L2TP lab image" in the main interop suite | The L2TP lab is its own suite: `test/interop-l2tp/scenarios/`, checkers in `internal/le/interoplab/l2tp/l2tp.go` `scenarioCheckerMap`, run by `./le test deployment l2tp-ppp` | `docs/architecture/testing/interop.md` "The suites"; `internal/le/interoplab/l2tp/register_catalog.go` |
| `test/plugin/dns-resolver-dhcp-lan.ci` proves the DHCP story | No `.ci` harness acts as a DHCP client against Ze's DHCP server, and no interop scenario does either (no `udhcpc`, `dhclient` or `dhcpcd` under `internal/le` or `test/interop*`). The story moves to interop scenario `dns-resolver-dhcp-lan` with a client container | grep over `internal/le/interoplab`, `internal/le/test`, `test/interop*/scenarios` |
| The `.ci` tests can send queries with chosen header bits | No query client exists in `le` or `ze` (no `dns.Client` outside the stub resolver); the stub prints records and RCODE, not header bits. A harness tool `le dns query` is created | grep for `dns.Client` and `Exchange(` under `internal/le` and `cmd` |
| `dns.Client` drops a reply that does not match the query | It drops a mismatched ID on UDP and the connected socket filters the source; it never compares the question. RFC 5452 Section 9.1 also requires matching name, class and type, so the resolver checks the question itself (AC-18) | `vendor/github.com/miekg/dns/client.go` `exchange`, the `r.Id == m.Id` loop |
| RFC2181-5.1-1 is not this spec's | The resolver is a name server that must mark a reply whose RRset does not fit; the harness `send` already calls `Msg.Truncate` on datagram replies, so the resolver's test proves the row for the resolver and the row's annotation (which still says the harness never truncates) is rewritten with it | `internal/core/dnsserver/handler.go` `send`; `rfc/short/rfc2181.md` RFC2181-5.1-1 |
| Data flow: `OnConfigure` parses and applies | The DNS plugins run the config transaction: `OnConfigure` parses, `OnConfigVerify` verifies, `OnConfigApply` applies, `OnConfigRollback` restores | `internal/plugins/as112/config_tx.go` |

## Task

The `fttx-edge` use-case profile (`rfc/profiles/fttx-edge.md`, "FTTx edge:
customers to the internet over L2TP, local DNS for the LAN") requires RFC 1035
and RFC 2181, and Ze has no DNS server that answers a customer's recursive
query. Today Ze answers DNS only as an authority (the `as112` and `geodns`
plugins over `internal/core/dnsserver`, which clears RA on every reply by
design), and resolves names only for itself (the stub in
`internal/component/resolve/dns`). `plan/learned/036-rfc-priority-tree.md`
item 5 names the LAN DNS resolver as one of the fttx-edge gaps.

Goal: Ze serves customer LANs (subscriber sessions over L2TP, and DHCP-addressed
LAN segments) as a caching resolver, answering only on the addresses and to the
source prefixes the operator names, by forwarding to the upstreams the
operator names. It is not security-aware: DNSSEC validation is
`plan/post-release/features/spec-dns-resolver-dnssec-validation.md`, and full
recursion is `plan/post-release/features/spec-dns-resolver-recursion.md` (D-2).
The address it serves on is what Ze already advertises through DHCPv4 option 6,
the RA RDNSS option, and IPCP Primary/Secondary DNS.

No existing spec covers a resolver: `./le spec status` and a search of every
work-item folder for "resolver", "recursive" and "unbound" find only the stub
resolver's own work and the paused DNS specs named below.

### RFCs the profile lists for this feature

| RFC | Tier for fttx-edge | Summary | Implementation kind | Role here |
|-----|--------------------|---------|---------------------|-----------|
| RFC 1035 | required | `rfc/short/rfc1035.md` (exists) | ze | message format, header bits, UDP and TCP transport; the resolver is a new producer of existing rows |
| RFC 2181 | required | `rfc/short/rfc2181.md` (exists) | ze | RRset handling, data ranking, TTL; rows in `RFC gaps` become applicable |
| RFC 2132 | required | `rfc/short/rfc2132.md` (exists) | ze | option 6 advertisement, existing code |
| RFC 1877 | required | `rfc/short/rfc1877.md` (exists) | ze | IPCP DNS advertisement, existing code |
| RFC 4035 | optional | `rfc/short/rfc4035.md` (exists) | ze | not this spec: its Support coverage text records the resolver's non-security-aware posture; validation is `plan/post-release/features/spec-dns-resolver-dnssec-validation.md` |
| RFC 4034 | optional | `rfc/short/rfc4034.md` (exists, not enrolled) | ze | not this spec: the record formats the validator parses (`plan/post-release/features/spec-dns-resolver-dnssec-validation.md`) |
| RFC 4033 | optional | `rfc/short/rfc4033.md` (exists) | foundation | terms only |
| RFC 8106 | optional | `rfc/short/rfc8106.md` (exists) | ze | RDNSS advertisement, existing code; no row changes |

Every listed RFC has a summary, so no summary is written. RFCs the design reads
but no profile lists, so no summary is written for them and their tests carry
no RFC tag: RFC 1034 (resolver algorithm, RD semantics), RFC 2308 (negative
caching), RFC 5452 (forgery countermeasures) and RFC 6891 (EDNS0). Their texts
are under `rfc/full/` (1034 and 5452 added in the 2026-10-27 pass).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/system-plugins.md`, `ai/doors/core.md` - the doors for a new system plugin and a new `internal/core` package
  → Constraint: every plugin here can run in another process; the resolver reaches nothing but its own state and `internal/core/dnsserver`. It owns its commands, YANG, doctor checks and help in its own folder
  → Constraint: a plugin that owns config and registers no `OnConfigApply` refuses every commit (`pkg/plugin/sdk/sdk_callbacks.go` `refuseConfigApply`); a nil parsed config must not be read as "nothing to apply"
- [ ] `docs/architecture/dns/server-harness.md` - the shared listener, the enforced answer shape, bind policy
  → Constraint: `Authoritative` owns the single `WriteMsg` and re-asserts `shapeAuthoritative` (AA unless RCODE 5, RA=0, Z=0, AD=0, Compress=false) after the answer function; a resolver cannot use it. A second wrapper `Recursive` in the same package owns the recursive shape the same way (answer function gets `Peer`, never the `ResponseWriter`)
  → Constraint: every consumer names its `BindPolicy` (lenient at boot, strict on reload); a reload that changes the listener set pauses serving; a reload that changes only data must not touch listeners (`reconcile` short-circuits on an unchanged endpoint signature)
  → Decision: the harness stays vocabulary-free; ACL prefixes go through the existing `Matcher` (`Entry{Prefix, Label}`)
- [ ] `ai/digests/dns-services.md` - traced flow of as112 and geodns over the harness
  → Constraint: plugin shape to copy is as112: `register.go` init registration (`Name`, `Features "yang"`, `ConfigRoots` = service root, `DoctorChecks` list), `config_tx.go` wiring `OnConfigure` / `OnConfigVerify` / `OnConfigApply` / `OnConfigRollback`, an atomic state snapshot the query path reads lock-free, then `ApplyWithSecure`; metrics default to a no-op registry before `ConfigureMetrics`
  → Constraint: as112's `allow-from` empty means allow everyone; that default is wrong for a resolver (open resolver), so the resolver refuses a commit with an empty `allow-from`
- [ ] `docs/features/dns-resolver.md`, `features/dns-resolver.md` - the client-side stub (`system dns`), its cache and `show/clear dns cache`
  → Constraint: the stub is a different subsystem with its own cache, commands and feature record; the LAN resolver shares neither the cache nor the `dns cache` command words, gets its own feature record `features/lan-dns-resolver.md`, and must not change what `system name-server` does
- [ ] `docs/architecture/testing/interop.md` - suites, scenario naming, checkers, discrimination
  → Constraint: scenario directories are named, no numeric prefix; checkers are typed Go under `internal/le/interoplab/<suite>/`; dnsmasq sidecars already exist in the main suite (`test/interop/Dockerfile.dnsmasq`), so DNS scenarios with Unbound and Knot join `test/interop/scenarios/` with checkers in `internal/le/interoplab/bgp/` (registry `scenarioOperations`, `checkers.go`); the L2TP story joins `test/interop-l2tp/scenarios/` (`internal/le/interoplab/l2tp/l2tp.go` `scenarioCheckerMap`)
  → Constraint: a scenario counts only after a recorded red: revert the producing change, rebuild the image the test drives, observe red, restore; a carrier with an `RFC requirement:` tag is walked by `./le rfc record`
  → Constraint: third-party lab software runs its latest stable upstream release (`ai/META-DECISIONS.md`): Unbound and Knot DNS images pin the current stable release
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` format and the `le mock dns` harness command
  → Constraint: the mock (`internal/test/mock/dns`, registered by `internal/le/mock/dns/register.go`) serves a fixed A/AAAA zone with per-name RCODE and TTL and counts served queries (`Server.served`); it signs nothing and logs no query. A harness tool is registered the same way: one package per command under `internal/le/`, implementation under `internal/test/`
- [ ] `ai/rules/goroutine-lifecycle.md`
  → Constraint: no goroutine per query or per upstream exchange. The resolution runs on the request goroutine miekg's server already gives each query; upstream exchanges are sequential within one resolution; in-flight coalescing parks waiters on a channel owned by the first resolution; cache expiry and rate-limit buckets are swept by one long-lived worker started at plugin start
- [ ] `rfc/profiles/fttx-edge.md` and the profile rows of each summary
  → Decision: required: rfc1035, rfc2181 (resolver behavior), rfc2132, rfc1877 (advertisement, existing); optional: rfc4033, rfc4034, rfc4035, rfc8106. The optional DNSSEC set stays out of this spec (D-1, answered (b))

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1035.md` - DNS
  → Constraint: no RFC 1035 row is closed by this spec (all met or the paused zone transfer); the resolver's new producer must still meet 2.3.3-1, 3.1-5 (case-insensitive compare, applies to the cache key), 4.1.1-1 (Z zero), 4.1.1-2 (AA meaning: AA=0 on every resolver reply), 4.1.4-5 (pointers understood), 4.2.1-1/2 (UDP bound and TC), 4.2.1-3/4.2.2-1 (port 53, TCP length prefix), 6.4-1 (Not Implemented), and each tagged test for those rows gains a resolver-side case with a discrimination record
- [ ] `rfc/short/rfc2181.md` - DNS clarifications
  → Constraint: 5.3.1-2/3/4, 5.4.1-3 and 5.4.1-9 are `{not-applicable}` because Ze is "authoritative plus a stub resolver only"; a caching resolver makes each applicable. This resolver is not security-aware, so RRSIG is an ordinary type it caches and returns when asked, and 5.3.1-2/3/4 still bind. 5.4.1-8 ("When DNS security ... is in use") is conditional on validation: this spec rewrites its annotation `excluded-kind: feature-out-of-scope`, and `plan/post-release/features/spec-dns-resolver-dnssec-validation.md` closes it
  → Constraint: 5.1-1 (TC when the RRset does not fit) gains the resolver case and its stale annotation is rewritten; 5.4-1/5.4-2 (replace RRsets, never merge) and section 8 (TTL 0..2147483647) gain resolver cases
- [ ] `rfc/short/rfc4035.md` - DNSSEC protocol modifications
  → Constraint: every recursive-server and validator row is `{not-applicable: ze's DNS servers never recurse ...}` or `{not-applicable: ze runs no local validator ...}` today; those rows stay as they are, the resolver's non-security-aware posture is added to the summary's Support coverage text, and the rows the validator closes are `plan/post-release/features/spec-dns-resolver-dnssec-validation.md`'s `RFC gaps`
  → Constraint: RFC4035-3-1 and RFC4035-3-2 (EDNS0 on the name server, at least 1220 octets) are open `{gap}` rows on the shipped authoritative servers; they are not claimed here, but the resolver listener meets both from its first commit
- [ ] `rfc/short/rfc2132.md`, `rfc/short/rfc1877.md`, `rfc/short/rfc8106.md` - how the address is advertised
  → Constraint: advertisement already exists (DHCPv4 option 6 `dns-server` leaf-list in `ze-dhcp-server-conf.yang`; RA `rdnss` container in `ze-iface-conf.yang`; IPCP 129/131 via the L2TP pool's `dns-primary` / `dns-secondary` in `ze-l2tp-pool-conf.yang`); this spec adds no advertisement code

**Key insights:**
- Ze has three producers that advertise a resolver address and no resolver behind any of them.
- The harness forbids recursion by construction; the resolver needs a second shaping wrapper, not a flag on the first.
- Only RFC 1035 and 2181 are required; all DNSSEC RFCs are optional for fttx-edge, so the validator has its own spec.
- `github.com/miekg/dns` v1.1.73 (vendored) supplies the codec and the client, but no iterative resolver and no validator.
- Unbound is the reference for every point the RFCs leave open; each deciding function or documented default is quoted in Key Design Decisions.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/dnsserver/handler.go` - `Authoritative(log, fn, onPanic)` wraps an `AnswerFunc(msg, r, p Peer) (send bool)`; non-QUERY opcodes draw Not Implemented before `fn`; `shapeAuthoritative` sets AA unless RCODE 5, clears RA, Z and AD and compression, before and after `fn`; `send` calls `Msg.Truncate(udpReplyLimit(r))` on datagram replies (512 floor, raised by an EDNS OPT) and writes once, counting write failures
- [ ] `internal/core/dnsserver/manager.go` - `New(log, handler dns.Handler, opts Options)`; `Options{Freebind, OnListenerChange, TLSMaterialResolver}`; no UDP size option, so `dns.Server.UDPSize` stays zero (the RFC4035-3-2 gap); `ApplyWithSecure(enabled, plain, sc, policy, log)` in `secure.go`
- [ ] `internal/core/dnsserver/matcher.go`, `client.go` - longest-prefix `Matcher`, `RemoteAddr`
- [ ] `internal/component/resolve/dns/resolver.go` - stub: `Resolver{client, server, cache, dnssec}`; caches string renderings of answer records; `dnssecDecision` treats upstream SERVFAIL as the DNSSEC failure signal; no local validation
- [ ] `internal/plugins/as112/register.go`, `config_tx.go` - the registration and transaction shape the resolver copies
- [ ] `internal/plugins/geodns/yang/ze-geodns-conf.yang` - `service { geodns { enabled; listener <name> { uses zt:listener } ... } }`
- [ ] `internal/component/config/listener.go` - `DiscoverListenerServices` walks every YANG `zt:listener` user so the cross-service port-conflict check covers new services by registration
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `dns-server` leaf-list, IPv4, option 6
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - RA `rdnss` container
- [ ] `internal/component/l2tp/plugins/pool/register.go`, `yang/ze-l2tp-pool-conf.yang` - pool `dns-primary` / `dns-secondary` carried into IPCP
- [ ] `internal/le/mock/dns/register.go`, `internal/test/mock/dns/server.go`, `zone.go` - `le mock dns`, the functional-test upstream: fixed zone, `Set` changes an answer, `served` counts queries
- [ ] `internal/le/interoplab/l2tp/l2tp.go` - L2TP lab peers (Ze, LAC, FRR, RADIUS) and `scenarioCheckerMap`
- [ ] `vendor/github.com/miekg/dns/client.go` - `exchange`: on a packet conn, reads until `r.Id == m.Id`; on a stream, a mismatched ID is `ErrId`; the question is never compared

**Behavior to preserve:**
- `Authoritative` and `shapeAuthoritative` unchanged; as112 and geodns replies byte-identical.
- The stub resolver, `system name-server`, resolv.conf writing, `show dns cache` and `clear dns cache` unchanged.
- The listener conflict check keeps refusing two services on one address and port.
- DHCP, RA and IPCP advertisement code and YANG unchanged.
- `le mock dns` keeps its current zone and output; new behavior is behind new flags.

**Behavior to change:**
- New `service dns-resolver` answers recursive queries from allowed sources on configured listeners.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `service { dns-resolver { ... } }` committed through the standard transaction.
- Wire: a DNS query (UDP or TCP, port 53 by default) from a LAN host or subscriber to a configured listener address.

### Transformation Path
1. Config transaction: `OnConfigure` parses the section into a candidate; `OnConfigVerify` refuses wildcard listeners, an empty `allow-from` and an empty forward list; `OnConfigApply` publishes the candidate as the atomic state snapshot (allow matcher, mode, upstreams, limits, cache bounds) and reconciles listeners through `Manager.ApplyWithSecure` (lenient at boot, strict on reload); `OnConfigRollback` restores the previous snapshot.
2. A query reaches `dnsserver.Recursive`: non-QUERY opcode draws Not Implemented; otherwise the recursive shape (QR, RA=1, AA=0, Z=0, CD copied) is applied and the resolver's answer function is called with `Peer`.
3. Answer function: ACL by packet source through `Matcher` (miss: REFUSED, counted, no upstream query); per-source rate limit (over the limit: dropped, 1 in 10 passed); RD=0: REFUSED; QDCOUNT not 1: FORMERR; then cache lookup keyed by lower-cased name, type and class.
4. Cache miss: join an identical in-flight resolution if one exists, otherwise start one if the outstanding-resolution bound allows (else drop); query the upstream list in order with failover. Every upstream exchange uses a fresh ephemeral source port and a random ID, and an EDNS0 OPT advertising 1232 with DO clear; a reply is accepted only if ID, question name (case-insensitive), class and type match; TC=1 retries over TCP. The whole resolution runs inside the per-query budget (upstream queries, CNAME restarts, deadline).
5. Ranking and bailiwick: records outside the zone being asked are dropped; answer-section data outranks authority and additional data (RFC 2181 Section 5.4.1); additional-section data is never returned as an answer.
6. Cache insert: whole RRsets replaced, never merged; positive TTL capped at `cache max-ttl`; NXDOMAIN cached by name and class, NODATA by name, type and class, for min(SOA TTL, SOA MINIMUM) capped at `cache max-negative-ttl`; timeouts and SERVFAIL never cached as negative.
7. Reply: answer with TTLs decremented by time in cache; AD never set; the harness bounds UDP size, sets TC, and writes once.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin | YANG `service dns-resolver`, SDK config transaction callbacks | No |
| Plugin to harness | `dnsserver.New` + `dnsserver.Recursive` answer function, `Options.UDPSize`, `ApplyWithSecure` | No |
| Ze to upstreams | UDP and TCP DNS from an ephemeral, per-exchange source port with a random ID | No |
| CLI to plugin | `show dns-resolver`, `clear dns-resolver cache all` registered RPCs | No |

### Integration Points
- `internal/core/dnsserver` - new `Recursive` wrapper and `Options.UDPSize`, which the resolver sets to 1232 (the listener reads at least 1220 octets)
- `internal/component/config/listener.go` - automatic via `zt:listener` registration; no list edit
- `internal/core/diagnostic/codes.go` - doctor codes the resolver registers
- `internal/component/plugin/all/all.go` - regenerated by `./le repo update`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to prove: query path only through `Recursive`; config only through the transaction callbacks |
| No unintended coupling (components stay isolated) | No | to prove: plugin imports `internal/core/dnsserver` only; no import of `geodns`, `as112` or `component/resolve` |
| No duplicated functionality (extends existing, does not recreate) | No | to prove: listener lifecycle, ACL matcher, opcode check, UDP bound reused from the harness; codec and client from miekg |
| Zero-copy preserved where applicable (refs, not copies) | No | cache stores parsed RRsets once; reply assembly copies RR pointers with a TTL rewrite on a per-reply copy of the header fields only |
| Registration over hardcoding, outbound | No | plugin registers via `init()`; commands via YANG `ze:command`; doctor checks via `DoctorChecks` |
| Registration over hardcoding, inbound | No | to search at implementation: listener conflict list (`DiscoverListenerServices`), doctor code table, plugin inventory docs, completion, `le` command registry; each must derive from registration |

## Config and CLI surface

### YANG `ze-dns-resolver-conf.yang` (under `service`)

| Path | Type | Default | Constraint | Purpose |
|------|------|---------|------------|---------|
| `dns-resolver enabled` | boolean | false | - | serve or not |
| `dns-resolver listener <name>` | list, `uses zt:listener` (ip, port) | port 53 | at least one when enabled; ip not `0.0.0.0` or `::` | where to answer |
| `dns-resolver allow-from` | leaf-list of IPv4 or IPv6 prefix | none | at least one when enabled | source prefixes answered |
| `dns-resolver mode` | enum `forward` (`plan/post-release/features/spec-dns-resolver-recursion.md` adds `recursive`) | `forward` | - | resolution mode |
| `dns-resolver forward server <ip>` | list keyed by IP, leaf `port` | port 53 | 1..8 entries, required in forward mode | upstreams, tried in configured order |
| `dns-resolver cache size` | uint32 entries | 100000 | 0..10000000; 0 disables caching | answer-cache bound, LRU eviction |
| `dns-resolver cache max-ttl` | uint32 s | 86400 | 0..2147483647 | positive TTL cap (Unbound `cache-max-ttl` default) |
| `dns-resolver cache max-negative-ttl` | uint32 s | 3600 | 0..2147483647 | negative TTL cap (Unbound `cache-max-negative-ttl` default) |
| `dns-resolver timeout` | uint8 s | 10 | 1..60 | whole-resolution deadline; SERVFAIL after it |
| `dns-resolver limit outstanding` | uint32 | 2048 | 1..65535 | concurrent resolutions; beyond it a new query is dropped (Unbound `num-queries-per-thread` libevent default and behavior) |
| `dns-resolver limit per-client-rate` | uint32 qps | 0 (off) | 0..1000000 | per-source query rate; excess dropped, 1 in 10 let through (Unbound `ip-ratelimit` default 0, `ip-ratelimit-factor` default 10) |

Fixed, not configurable: per-resolution limits of 32 upstream queries (Unbound
`max-sent-count` default 32) and 11 CNAME restarts (Unbound
`max-query-restarts` default 11); upstream EDNS buffer 1232.

### Commands `ze-dns-resolver-cmd.yang`

| Command | Output |
|---------|--------|
| `show dns-resolver` | listeners and their state, mode, upstreams, limits, cache entries (answer, negative), hit rate, refused, rate-limited and dropped counts, upstream query outcomes. Through `ApplyPipes`, JSON keys kebab-case |
| `clear dns-resolver cache all` | empties every resolver cache; bare `clear dns-resolver cache` is refused with the keyword to add, as `clear dns cache` is |

### Harness tool `le dns query` (new, test infrastructure)

| Flag | Meaning |
|------|---------|
| `--server <ip>[:port]` | destination |
| `--source <ip>` | bind the query's source address (ACL tests) |
| `--tcp` | TCP instead of UDP |
| `--name`, `--type` | question |
| `--rd`, `--cd`, `--ad`, `--do`, `--edns <size>`, `--no-edns`, `--opcode <n>` | header and OPT |
| output | one line per field: `rcode`, `flags` (qr aa tc rd ra ad cd), `size`, then each section's records in presentation form with TTL; exit 0 on any reply, non-zero with the reason on timeout |

`le mock dns` gains `--log-queries` (one stderr line per query received, so a
`.ci` can assert that no upstream query was sent) and `--delay <ms>` /
`--drop` per name (timeout and slow-upstream tests).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-2 | The listener conflict check refuses the resolver on an address and port geodns or as112 already binds, with no edit to `listener.go` | `DiscoverListenerServices` walks every `zt:listener` user | two services race for port 53 | functional test `dns-resolver-listener-conflict.ci` | unvalidated |
| A-3 | An L2TP subscriber reaches a resolver bound on an LNS loopback or gateway address; binding per ppp interface is not needed | subscriber sessions are dynamic interfaces; IPCP hands out an address, not an interface | per-session binding needed; ACL by source prefix still holds | interop `dns-resolver-l2tp-subscriber` | unvalidated |
| A-4 | Spoofed replies are rejected: fresh ephemeral port and random ID per exchange, source filtered by the connected UDP socket, mismatched ID skipped; the question is NOT checked by miekg | `vendor/github.com/miekg/dns/client.go` `exchange` (read) | cache poisoning by off-path spoofing | unit test injecting a mismatched ID, a mismatched question, and a reply from another source | read at source; question check is resolver code (AC-18) |
| A-7 | Ze's DHCP server runs in the interop Ze container and leases to a client container on the lab bridge | `internal/plugins/dhcpserver` serves on a configured interface; the lab Ze container owns `eth0` on the bridge | the DHCP story needs a QEMU guest instead | interop `dns-resolver-dhcp-lan` | unvalidated |
| A-8 | miekg `dns.Server` handles each UDP and TCP query on its own goroutine, so a blocking answer function does not stall other queries | miekg server design; `docs/architecture/dns/server-harness.md` | one slow resolution blocks the listener | unit test: a resolution blocked on a slow upstream while a cached query is answered | unvalidated |

A-1, A-5 and A-6 were validator assumptions and moved with the validator to
`plan/post-release/features/spec-dns-resolver-dnssec-validation.md`; the ids are not reused.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Open resolver on the WAN (amplification, abuse) | queries from unexpected sources in `ze_dns_resolver_refused_total` | wildcard listen address refused at commit; empty `allow-from` refused at commit; REFUSED outside the ACL, never an answer; the REFUSED reply is no larger than the query |
| R-2 | Cache poisoning | answers that disagree with Unbound in the cross-check scenario | random ID and port (A-4), question match, bailiwick check on every RR, RFC 2181 ranking, no RRset merge |
| R-4 | Work amplification (long CNAME chains, CNAME loops) | `upstream_queries_total` per client query | per-resolution budget (32 upstream queries, 11 restarts, deadline), coalescing, outstanding bound |
| R-5 | Cache memory growth | `ze_dns_resolver_cache_entries` at limit | entry-count bound with LRU eviction on every cache |
| R-6 | Reload pauses serving | dropped queries during commit | data-only changes never touch listeners (signature unchanged); listener changes follow harness strict policy |
| R-7 | Upstream unreachable read as "no such name" | NOERROR empty or NXDOMAIN on timeout | timeout and SERVFAIL are distinct outcomes; never cached as negative |
| R-8 | TCP fallback missing breaks large answers | TC replies loop | resolver side retries over TCP on TC; client side gets TC and the TCP listener |
| R-9 | A LAN host floods the resolver | `outstanding` at its bound, `rate_limited_total` rising | outstanding bound drops new work; optional per-client rate limit; a flood from one source cannot evict another's cached data faster than LRU allows |
| R-10 | Negative TTL taken from the wrong field | NXDOMAIN cached for the SOA's own TTL when MINIMUM is lower | RFC 2308 Section 5 min(SOA TTL, SOA MINIMUM), capped; unit test with each field the smaller |

R-3 (unsynced clock at boot) was a validator risk and moved to
`plan/post-release/features/spec-dns-resolver-dnssec-validation.md`.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Name resolution for every customer behind the box. Default off, so nothing changes until configured |
| How is it reverted? | Single commit revert; config section removed |
| Who else touches this path? | `internal/core/dnsserver` (as112, geodns), the stub resolver |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service dns-resolver` forward config + `le dns query` from an allowed source | → | `dnsresolver` answer function via `dnsserver.Recursive` | `test/plugin/dns-resolver-forward.ci` |
| query from a source outside `allow-from` | → | ACL in the answer function | `test/plugin/dns-resolver-acl-refused.ci` |
| commit with wildcard listener or empty `allow-from` | → | `OnConfigVerify` refusal | `test/plugin/dns-resolver-config-refused.ci` |
| `show dns-resolver`, `clear dns-resolver cache all` | → | `show.go` handlers | `test/plugin/dns-resolver-show.ci` |
| LAN host leased by Ze's DHCP server queries the advertised address | → | answer function, forward path | interop `dns-resolver-dhcp-lan` |
| L2TP subscriber queries the IPCP-advertised address | → | answer function, forward path | interop `dns-resolver-l2tp-subscriber` |

## Acceptance Criteria

AC ids are stable: AC-34 and AC-35 (recursion) moved with their ids to
`plan/post-release/features/spec-dns-resolver-recursion.md`, and V-1..V-15
(validation) to `plan/post-release/features/spec-dns-resolver-dnssec-validation.md`.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `dns-resolver` enabled, `mode forward`, upstream answers A for a name; query RD=1 from an allowed source | NOERROR with the upstream's records; RA=1, AA=0, Z=0, AD=0; answer TTL no greater than the upstream's. → Test: `test/plugin/dns-resolver-forward.ci` |
| AC-2 | Query from a source outside every `allow-from` prefix | REFUSED, AA=0, `ze_dns_resolver_refused_total` +1, and `le mock dns --log-queries` logs no query. → Test: `test/plugin/dns-resolver-acl-refused.ci` (seq=3), `TestACLRefused` |
| AC-3 | Commit with a listener `ip` of `0.0.0.0` or `::` | Commit refused; message names the listener, says a wildcard would answer on the WAN, and names the fix (a LAN or loopback address) |
| AC-4 | Commit with `enabled true` and no `allow-from` entry | Commit refused; message says the resolver would refuse every client and names the leaf to set |
| AC-5 | Same query repeated within the TTL | Second answer from cache: no upstream query logged, TTL reduced by the elapsed whole seconds. → Test: `test/plugin/dns-resolver-forward.ci` (seq=4, seq=5: one upstream question), `TestCacheTTLDecrement` (the TTL) |
| AC-6 | Upstream returns a new RRset for a cached name and type after expiry | Cached RRset replaced whole, never merged with the old records (RFC2181-5.4-1) |
| AC-7 | Upstream answers NXDOMAIN with an SOA whose TTL is 300 and MINIMUM 60 | NXDOMAIN cached 60 s for every type of that name; NODATA cached per type; `max-negative-ttl` caps a larger value |
| AC-8 | Query DO=1 | The upstream query carries an OPT with DO=0; the reply carries an OPT, AD=0, and no record the upstream did not return for the question |
| AC-9 | Query for type RRSIG, DNSKEY or DS | Returned as an ordinary RRset, the entire RRset (RFC2181-5.3.1-2); an RRSIG seen only in an authority or additional section is never returned for an RRSIG query (RFC2181-5.3.1-3/4) |
| AC-10 | Upstream reply sets AA=1 or AD=1 | Ze's reply has AA=0 and AD=0 |
| AC-11 | CNAME loop, or a chain longer than 11 restarts, or more than 32 upstream queries needed | SERVFAIL; upstream queries for that client query never exceed 32 |
| AC-12 | Query with RD=0 from an allowed source | REFUSED, whether or not the name is cached, and no upstream query. → Test: `test/plugin/dns-resolver-acl-refused.ci` (seq=4), `TestRDZeroRefused` |
| AC-13 | UDP query with EDNS buffer 1232 and an answer larger than 1232 octets | Reply carries OPT, is at most 1232 octets and has TC=1 (RFC2181-5.1-1, RFC1035-4.2.1-2); the same query over TCP returns the whole answer |
| AC-14 | UDP query of 1220 octets or more (EDNS) | Read whole and answered (listener `UDPSize` 1232) |
| AC-15 | Upstream answer carries TC=1 | The resolver repeats the query over TCP and answers with the whole RRset |
| AC-16 | All upstreams unreachable or silent | SERVFAIL within `timeout`; nothing cached as negative; the next query retries upstream. → Test: `test/plugin/dns-resolver-timeout.ci`, `TestFailoverAndTimeout` |
| AC-17 | First upstream silent, second answers | Answer from the second within `timeout`; `upstream_queries_total{result="timeout"}` +1 |
| AC-18 | Reply from upstream with a mismatched ID, a mismatched question name, class or type, or from another source address or port | Ignored; the resolver keeps waiting or times out (RFC 5452 Section 9.1 matching) |
| AC-19 | Upstream answer carrying records outside the queried zone's bailiwick | Those records are neither cached nor returned |
| AC-20 | Answer with an A record in the answer section and a different A for the same name in an earlier reply's additional section | The answer-section RRset wins and is what a later query returns; additional-section data is never returned as an answer (RFC2181-5.4.1-3) |
| AC-21 | Upstream answer with a TTL above 2147483647 (high bit set) | Treated as 0 (RFC 2181 Section 8) and not cached |
| AC-22 | Two clients ask the same uncached question at once | One upstream query; both get the answer |
| AC-23 | `limit outstanding 1` and a second distinct uncached query while the first is in flight | The second is dropped (no reply); `dropped_total` +1; a cached query still answers |
| AC-24 | `limit per-client-rate 5` and one source sending 50 queries in a second | No more than 5 plus one in ten of the excess answered; `rate_limited_total` counts the rest; another source unaffected |
| AC-25 | Query with QDCOUNT 0 or 2 | FORMERR, no upstream query |
| AC-26 | Inverse query, or any opcode other than QUERY | Not Implemented (RFC1035-6.4-1) |
| AC-27 | Name compared in another case (`WWW.Example.TEST`) | Same cache entry as the lower-case name; the reply's question echoes the query's case (RFC1035-3.1-5) |
| AC-28 | Resolver listener on an address and port another DNS service binds | Commit refused by the listener conflict check, naming both services |
| AC-29 | `dns-resolver` absent or `enabled false` | No listener bound; `system name-server` and the stub unchanged |
| AC-30 | Reload changing only `allow-from`, upstreams or limits | New policy on the next query; listeners not rebound (no serving pause). → Test: `TestReloadKeepsListeners` (one socket across the reload), `test/reload/dns-resolver-reload.ci` (allow-from reloaded, next query refused or answered by the new list) |
| AC-31 | `show dns-resolver` | The fields in the Commands table; `clear dns-resolver cache all` empties every cache and the next query goes upstream. → Test: `test/plugin/dns-resolver-show.ci` |
| AC-32 | LAN host gets a DHCPv4 lease from Ze with `dns-server` set to the resolver address, then resolves a name with `kdig` against the leased server | Answered |
| AC-33 | L2TP subscriber gets the resolver address via IPCP from the pool's `dns-primary` and resolves a name | Answered; the subscriber's pool prefix is in `allow-from` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator enables a forwarding resolver for a LAN and points DHCP at it | config → listener bound → DHCP option 6 → host query → ACL → cache/forward → reply | interop `dns-resolver-dhcp-lan` |
| 2 | FTTx subscriber over L2TP resolves names | IPCP DNS → query to LNS address → ACL → forward → reply | interop `dns-resolver-l2tp-subscriber` |
| 3 | Operator forwards to the ISP's central resolver | config → forward → Unbound upstream → reply, answers equal Unbound's own | interop `dns-resolver-forward-unbound` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecursiveShape` | `internal/core/dnsserver/recursive_test.go` | RA=1, AA=0, Z=0, AD=0, CD copied; an answer function cannot set AA, clear RA, or set AD (RFC1035-4.1.1-1, 4.1.1-2) | |
| `TestRecursiveOpcode` | `internal/core/dnsserver/recursive_test.go` | AC-26, RFC1035-6.4-1 resolver case | |
| `TestRecursiveUDPSize` | `internal/core/dnsserver/recursive_test.go` | AC-13, AC-14: 1232-octet query read; OPT on reply; TC on oversize (RFC2181-5.1-1, RFC1035-4.2.1-2) | |
| `TestAuthoritativeUnchanged` | `internal/core/dnsserver/handler_test.go` | as112/geodns shape byte-identical after the change | |
| `TestACLRefused`, `TestRDZeroRefused`, `TestQDCount` | `internal/plugins/dnsresolver/server_test.go` | AC-2, AC-12, AC-25 | |
| `TestRateLimit`, `TestOutstandingBound` | `internal/plugins/dnsresolver/limit_test.go` | AC-23, AC-24 | |
| `TestCacheReplaceNotMerge`, `TestCacheCaseFold`, `TestCacheTTLDecrement`, `TestTTLHighBit` | `internal/plugins/dnsresolver/cache_test.go` | AC-5, AC-6, AC-21, AC-27; RFC2181-5.4-1, RFC1035-3.1-5, RFC 2181 Section 8 | |
| `TestNegativeCacheTTL` | `internal/plugins/dnsresolver/cache_test.go` | AC-7, R-10 | |
| `TestTrustRanking` | `internal/plugins/dnsresolver/cache_test.go` | AC-20, RFC2181-5.4.1-3 | |
| `TestSIGRRSetHandling` | `internal/plugins/dnsresolver/cache_test.go` | AC-9, RFC2181-5.3.1-2/3/4 | |
| `TestUpstreamQueryShape` | `internal/plugins/dnsresolver/forward_test.go` | AC-8 (OPT 1232, DO clear), AC-10, RFC2181-5.4.1-9 (AA=0 on every resolver reply) | |
| `TestReplyMatching`, `TestBailiwick` | `internal/plugins/dnsresolver/forward_test.go` | AC-18, AC-19, A-4 | |
| `TestFailoverAndTimeout`, `TestTCPRetryOnTC` | `internal/plugins/dnsresolver/forward_test.go` | AC-15, AC-16, AC-17, R-7 | |
| `TestCoalescing`, `TestSlowUpstreamDoesNotBlockCache` | `internal/plugins/dnsresolver/server_test.go` | AC-22, A-8 | |
| `TestWorkBudget` | `internal/plugins/dnsresolver/budget_test.go` | AC-11 | |
| `TestParseConfigRefusals` | `internal/plugins/dnsresolver/config_test.go` | AC-3, AC-4, forward list empty, boundary table | |
| `TestReloadKeepsListeners` | `internal/plugins/dnsresolver/config_tx_test.go` | AC-30 | |
| `TestDoctorCodes` | `internal/plugins/dnsresolver/doctor_test.go` | every registered code in `internal/core/diagnostic/codes.go` | |
| `TestQueryToolFlags` | `internal/test/dnsquery/query_test.go` | the harness tool sets each header bit and prints each field | |
| `TestMockLogAndDelay` | `internal/test/mock/dns/server_test.go` | `--log-queries`, `--delay`, `--drop` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| listener `port` | 1-65535 | 65535 | 0 | 65536 |
| `cache size` (entries) | 0-10000000 | 10000000 | N/A (0 disables) | 10000001 |
| `cache max-ttl`, `cache max-negative-ttl` (s) | 0-2147483647 | 2147483647 | N/A | 2147483648 |
| `timeout` (s) | 1-60 | 60 | 0 | 61 |
| `limit outstanding` | 1-65535 | 65535 | 0 | 65536 |
| `limit per-client-rate` (qps) | 0-1000000 | 1000000 | N/A (0 off) | 1000001 |
| `forward server` count | 1-8 | 8 | 0 (with `mode forward`) | 9 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-resolver-forward` | `test/plugin/dns-resolver-forward.ci` | allowed client resolves through `le mock dns` upstream with `le dns query`; header bits asserted; second query cached (mock logs one query) | |
| `dns-resolver-acl-refused` | `test/plugin/dns-resolver-acl-refused.ci` | query with `--source` outside `allow-from` refused; RD=0 refused; mock logs nothing | |
| `dns-resolver-config-refused` | `test/plugin/dns-resolver-config-refused.ci` | wildcard listener, empty allow-from, empty forward list refused, each naming the fix | |
| `dns-resolver-listener-conflict` | `test/plugin/dns-resolver-listener-conflict.ci` | same address and port as geodns refused | |
| `dns-resolver-timeout` | `test/plugin/dns-resolver-timeout.ci` | mock `--drop` on one name: SERVFAIL within timeout, then answered once the mock serves it | |
| `dns-resolver-show` | `test/plugin/dns-resolver-show.ci` | `show dns-resolver` and `clear dns-resolver cache all` | |
| `dns-resolver-reload` | `test/reload/dns-resolver-reload.ci` | allow-from change applies without rebinding | |

Every `.ci` uses `option=exclusive:group=dns-stub-port-53` where it binds port
53, as `dns-stub-lookup.ci` does, or an unprivileged port otherwise.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dns-resolver-forward-unbound` | `test/interop/scenarios/dns-resolver-forward-unbound/` | Unbound (latest stable) as upstream over a Knot DNS authoritative lab hierarchy; `kdig` client container | Ze forwards to a real resolver; per query, RCODE and answer RRsets equal Unbound's own answers to the same queries; TC and TCP retry with a large RRset | |
| `dns-resolver-dhcp-lan` | `test/interop/scenarios/dns-resolver-dhcp-lan/` | client container (Debian stable: `dhcpcd`, `knot-dnsutils`) leased by Ze's DHCP server; Knot DNS authoritative | the host learns the resolver from option 6 and resolves through it; a host outside `allow-from` is refused | |
| `dns-resolver-l2tp-subscriber` | `test/interop-l2tp/scenarios/dns-resolver-l2tp-subscriber/` | the lab's xl2tpd + pppd LAC | subscriber gets the resolver address through IPCP and resolves through it | |

### Future (if deferring any tests)
- None deferred.

## Files to Modify
- `internal/core/dnsserver/handler.go` - add `Recursive` wrapper and `shapeRecursive` (owns RA, AA, CD, AD, Z); `Authoritative` untouched
- `internal/core/dnsserver/manager.go` - `Options.UDPSize` passed to `dns.Server.UDPSize`
- `internal/test/mock/dns/server.go`, `zone.go`, `dns.go` - `--log-queries`, `--delay`, `--drop`
- `internal/le/interoplab/bgp/checkers.go` (registry) and `internal/le/interoplab/l2tp/l2tp.go` (`scenarioCheckerMap`, peers) - the new scenarios
- `internal/core/diagnostic/codes.go` - resolver doctor codes; its design page `docs/features/ai-first.md` (built-in diagnostic codes) gains the new codes
- `rfc/short/rfc2181.md` - rewrite the annotations of every `RFC gaps` RFC 2181 row to its tagged test, resolver cases on 5.4-1/5.4-2 and section 8; RFC2181-5.4.1-8 rewritten `excluded-kind: feature-out-of-scope`; Meta `Support coverage` and `Support remaining`
- `rfc/short/rfc1035.md` - resolver-side cases on the rows listed in Required Reading; Meta `Support coverage`
- `rfc/short/rfc4035.md` - the Support coverage text describing the resolver's non-security-aware posture; no row changes
- `docs/architecture/dns/server-harness.md` - the second wrapper and why
- `docs/features/dns-resolver.md` - point at the LAN resolver as the server side
- `ai/digests/dns-services.md` - add the resolver flow
- `docs/architecture/testing/ci-format.md` - `le dns query` and the new mock flags
- `docs/architecture/testing/interop.md` - DNS scenarios, Unbound, Knot and client images, the L2TP scenario
- `ai/INDEX.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/guide/plugins.md`, `docs/features/plugins.md`, `docs/plugin-overview.md`, `docs/comparison.md`, `docs/plugin-development/metrics.md`, `docs/architecture/api/commands.md`, `docs/functional-tests.md`, `docs/guide/status.md`
- `internal/component/plugin/all/all.go` - regenerated

## Files to Create
- `internal/plugins/dnsresolver/` - `register.go`, `config.go`, `config_tx.go`, `state.go`, `server.go` (answer function, ACL, RD and QDCOUNT checks), `limit.go` (outstanding bound, per-client rate, one sweeper worker), `forward.go`, `cache.go` (answer and negative caches, ranking, LRU), `budget.go`, `show.go`, `doctor.go`, `metrics.go`, `yang/ze-dns-resolver-conf.yang`, `yang/ze-dns-resolver-cmd.yang`, `yang/register.go`, tests above
- `internal/le/dns/query/register.go` - registers `le dns query`; implementation `internal/test/dnsquery/` with tests
- `test/plugin/dns-resolver-*.ci` - the functional tests above
- `test/interop/scenarios/dns-resolver-forward-unbound/`, `dns-resolver-dhcp-lan/`; `test/interop-l2tp/scenarios/dns-resolver-l2tp-subscriber/`
- `test/interop/Dockerfile.unbound`, `test/interop/Dockerfile.knot`, `test/interop/Dockerfile.dnsclient`
- `internal/le/interoplab/bgp/check_dns_resolver.go`, `check_dns_resolver_predicate.go` (with predicate tests, as `check_dhcp_predicate.go`), and the L2TP checker in `internal/le/interoplab/l2tp/`
- `features/lan-dns-resolver.md` - feature record
- `docs/guide/dns-resolver.md` - operator guide (config, advertisement via DHCP, RA, IPCP; ACL; limits; DNSSEC posture: not validating)
- `docs/architecture/dns/lan-resolver.md` - design (shape, cache, ranking, limits, budget)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang`, `ze-dns-resolver-cmd.yang` |
| YANG validation constraints | Yes | `zt:listener` ports, `range` on cache, TTLs, timeout, limits; `max-elements` on forward servers (8); prefix type on `allow-from` |
| YANG custom validators | Yes | wildcard listener refusal, empty `allow-from` refusal, forward list required in forward mode (`ze:validate`) |
| CLI commands/flags | Yes | `show dns-resolver`, `clear dns-resolver cache all` via `ze:command`; `le dns query` harness tool |
| CLI grammar (keyword before value) | Yes | `clear dns-resolver cache all`; bare `clear dns-resolver cache` refused naming `all` |
| Editor autocomplete | Yes | automatic for enums (`mode`); listener names by YANG |
| Functional test for new RPC/API | Yes | `test/plugin/dns-resolver-show.ci` |
| Pipe completeness | Yes | `show dns-resolver` output through `ApplyPipes` |
| Env var registration | No | no `environment/` leaf; all config is YANG under `service` |
| Doctor check for runtime dependencies | Yes | listen port 53: `doctor-dns-resolver-port-unavailable`; no reachable upstream: `doctor-dns-resolver-upstream-unreachable`; codes in `internal/core/diagnostic/codes.go`, unit and functional tests. → Test: `TestDoctorCodes`, `TestDoctorListenerPortUnavailable`, `TestDoctorUpstreamUnreachable` (`internal/plugins/dnsresolver/doctor_test.go`) |
| Prometheus counters/metrics | Yes | `ze_dns_resolver_queries_total{transport,rcode}`, `ze_dns_resolver_refused_total{reason=acl|rd-clear}`, `ze_dns_resolver_rate_limited_total`, `ze_dns_resolver_dropped_total`, `ze_dns_resolver_outstanding`, `ze_dns_resolver_cache_entries{cache=answer|negative}`, `ze_dns_resolver_cache_hits_total`, `ze_dns_resolver_upstream_queries_total{result}`, listener gauge via `OnListenerChange` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/lan-dns-resolver.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`; `docs/architecture/config/syntax.md` N-A (a new section only) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (two RPCs) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/dns-resolver.md` (new) |
| 7 | Wire format changed? | No | DNS wire format unchanged; miekg codec |
| 8 | Plugin SDK/protocol changed? | No | uses existing SDK surfaces |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2181.md`, `rfc/short/rfc1035.md`, `rfc/short/rfc4035.md`; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/ci-format.md` (`le dns query`, mock flags), `docs/architecture/testing/interop.md` (DNS scenarios and images) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/dns/server-harness.md`, `docs/architecture/dns/lan-resolver.md` |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/post-release/features/spec-lan-dns-resolver.md` at implementation; known: `docs/architecture/dns/server-harness.md`, `docs/guide/as112.md` (handler.go anchors) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/dns-resolver.md` examples stay valid; DHCP, RA and L2TP pool guide examples gain a pointer to the resolver |

### Discovery (`ai/rules/repo-maintenance.md` mechanical checklist)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` keyword row "LAN DNS resolver" → `docs/architecture/dns/lan-resolver.md`, `docs/guide/dns-resolver.md`; `ai/digests/dns-services.md` |
| Rule that prevents regression | `rfc/short/rfc2181.md` and `rfc1035.md` tagged rows with discrimination records, checked by `./le rfc check` |
| Registry or inventory that prevents drift | plugin registration (`all.go` regenerated), `DiscoverListenerServices`, doctor code table, interop catalogs |
| Verification that proves it | the functional tests and interop scenarios above, each interop with a recorded red |

## Implementation Steps

1. **Phase: Wiring** -- plugin registers, YANG loads, listener binds, `Recursive` wrapper answers SERVFAIL for everything; `le dns query` exists
   - Tests: `dns-resolver-forward.ci` (red on SERVFAIL), `dns-resolver-config-refused.ci`, `dns-resolver-listener-conflict.ci`, `TestRecursiveShape`, `TestRecursiveOpcode`, `TestRecursiveUDPSize`, `TestAuthoritativeUnchanged`, `TestQueryToolFlags`
   - Files: `register.go`, `config.go`, `config_tx.go`, `server.go`, YANG, `handler.go`, `manager.go`, `internal/le/dns/query`, `internal/test/dnsquery`
   - Docs in this phase: `server-harness.md`, `ci-format.md`
2. **Phase: Forwarding, cache and limits** -- ACL, RD and QDCOUNT checks, forward with failover and TCP retry, reply matching, bailiwick, ranking, answer and negative caches, coalescing, budget, outstanding bound, per-client rate, show and clear, metrics, doctor
   - Tests: AC-1..AC-31 unit and functional tests; mock flags
   - Docs in this phase: guide, command reference, metrics, plugin inventory, feature record, `lan-resolver.md`
3. **Phase: Interop and advertisement stories** -- the scenarios `dns-resolver-forward-unbound`, `dns-resolver-dhcp-lan` and `dns-resolver-l2tp-subscriber`, cross-check against Unbound, DHCP and L2TP stories (AC-32, AC-33); each red recorded (`./le rfc record` for tagged carriers)
4. **Phase: RFC rows** -- every `RFC gaps` row annotated to its tagged test with a discrimination record, in the commit of the code it proves; RFC2181-5.4.1-8 rewritten `excluded-kind: feature-out-of-scope`; the RFC 4035 Support coverage text (each page above is written in the phase whose code changes it, per `ai/rules/documentation.md`; this phase only closes the RFC rows that span phases)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each user story path runs end to end in a test |
| Correctness | AA never set; AD never set; timeout never cached as negative; RRsets replaced, never merged; RD=0 refused |
| Naming | YANG `dns-resolver`, JSON keys kebab-case, metric prefix `ze_dns_resolver_` |
| Data flow | Only `Recursive` writes replies |
| Rule: principles (no silent zero) | An upstream failure answers SERVFAIL, never NOERROR empty |
| Rule: goroutine-lifecycle | No goroutine per query, per exchange, or per cache entry |
| Rule: rfc-compliance | Every implementing function quotes its RFC 1035 or 2181 sentence; every caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | `ze plugin list` shows `dnsresolver`; `./le repo update` leaves `all.go` clean |
| Harness tool | `le help` lists `dns query`; `TestQueryToolFlags` |
| Functional tests | the `.ci` files pass under their scoped functional run |
| Interop scenarios | each green, each with a recorded red |
| RFC rows | `./le rfc check` green with every `RFC gaps` row tagged and discriminated |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | malformed queries, oversized names, label pointers looping (miekg), QDCOUNT not 1 |
| Open resolver | wildcard refused, empty ACL refused, REFUSED outside ACL, RD=0 refused (no cache snooping), no answer amplification to refused clients |
| Poisoning | random ID and port, question match, bailiwick, ranking, no RRset merge |
| Resource exhaustion | cache bounds, outstanding bound, per-client rate, coalescing, per-resolution budget, TCP connection limit from the harness |
| Fail-open | every error path answers SERVFAIL or drops, never stale or fabricated data |

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
| RFC 1034 Section 4.3.1 | "Note that the name server should never perform recursive service unless asked via RD, since this interferes with trouble shooting of name servers and their databases." | RD=0 never triggers resolution; with Unbound's default it is refused |
| RFC 2181 Section 5.4 | "Servers must never merge RRs from a response with RRs in their cache to form an RRSet." | cache replaces whole RRsets |
| RFC 2308 Section 5 | "When the authoritative server creates this record its TTL is taken from the minimum of the SOA.MINIMUM field and SOA's TTL." | negative TTL is min(SOA TTL, SOA MINIMUM) |
| RFC 2308 Section 5 | "upon reaching zero (0) indicates the cached negative answer MUST NOT be used again." | negative entries expire like positive ones |
| RFC 5452 Section 9.1 | "A resolver implementation MUST match responses to all of the following attributes of the query:" (source and destination address, port, ID, name, class and type) | question compared by the resolver; miekg checks only the ID |
| RFC 5452 Section 9.2 | "Use an unpredictable source port for outgoing queries from the range of available ports (53, or 1024 and above) that is as large as possible and practicable;" | fresh ephemeral port per exchange, never a fixed source port |
| RFC 4035 Section 4.2 | "A security-aware resolver MUST support the signature verification mechanisms described in Section 5" | no DNSSEC-transparent middle ground: this resolver sets DO clear upstream and is not security-aware until `plan/post-release/features/spec-dns-resolver-dnssec-validation.md` lands |
| RFC 4035 Section 3.2.2 | "The name server side MUST copy the setting of the CD bit from a query to the corresponding response." | `shapeRecursive` copies CD |
| RFC 4035 Section 5.4 | "As with all DNS operations, however, the resolver MUST bound the work it puts into answering any particular query." | per-resolution budget |
| RFC 2132 Section 3.8 | "The domain name server option specifies a list of Domain Name System (STD 13, RFC 1035 [8]) name servers available to the client." | the existing `dns-server` leaf is the DHCPv4 advertisement |
| RFC 1877 Section 1.1 | "This Configuration Option defines a method for negotiating with the remote peer the address of the primary DNS server to be used on the local end of the link." | the existing L2TP pool `dns-primary` is the subscriber advertisement |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/help-text-describes-superseded-behaviour.md` | 2026-10-27 | `le mock dns help` starts the mock instead of printing help |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A new system plugin `internal/plugins/dnsresolver` over the shared harness | extend geodns; a component | the harness exists for exactly this (`server-harness.md`, two consumers); a plugin cannot import a sibling; the resolver has a config-driven lifecycle, so it is a plugin, not core |
| Second harness wrapper `Recursive` beside `Authoritative` | a flag on `Authoritative`; give the resolver the `ResponseWriter` | the harness doc records that a convention-based guard was replaced by an enforced one after two reviews; a flag reopens that; one wrapper per shape keeps both enforced |
| Forwarding in this spec, recursion in `plan/post-release/features/spec-dns-resolver-recursion.md` | one spec carrying both modes | D-2 is open and forwarding is complete without recursion, so it ships while Thomas decides; the `mode` leaf exists from the start so recursion adds an enum value, not a new syntax |
| Not security-aware: upstream EDNS0 with DO clear, DNSSEC types as ordinary RRsets, AD never set | DNSSEC-transparent forwarding (DO passed, RRSIGs relayed, no validation) | RFC 4035 Section 4.2 makes a resolver that asks for DNSSEC data owe validation, so the transparent option would claim RFC 4035 conformance it does not have; D-1 answered (b) |
| Outside the ACL: REFUSED, not silent drop | drop | RFCs silent. unbound.conf `access-control`: "The most specific netblock match is used, if none match refuse is used." and `refuse`: "Stops queries too, but sends a DNS rcode REFUSED error message back." REFUSED lets a misconfigured customer see the cause; the reply is no larger than the query |
| RD=0 refused | answer from cache (Unbound `allow_snoop`); resolve as RD=1 (the previous draft) | Unbound default `allow`: "Non-recursive queries are refused."; worker.c refuses unless `acl_allow_snoop`; RFC 1034 Section 4.3.1 forbids recursing without RD; refusing also closes cache snooping |
| Empty `allow-from` refuses the commit | allow-all (as112 default); loopback only (Unbound default allows 127.0.0.0/8 and ::1) | a LAN resolver that answers no LAN is a misconfiguration, and allow-all is an open resolver; refusing names the fix |
| Wildcard listener refused | allow wildcard with ACL | keeps the WAN side closed even if the ACL is wrong |
| Outstanding bound 2048, excess dropped | SERVFAIL the excess | unbound.conf `num-queries-per-thread`: "If more queries arrive that need servicing, and no queries can be jostled out ... then the queries are dropped. This forces the client to resend after a timeout"; default 2048 (libevent) |
| Per-client rate limit off by default, 1 in 10 of the excess passed | always on; drop all excess | unbound.conf `ip-ratelimit` "Default: 0 (disabled)", "More queries are completely dropped and will not receive a reply, SERVFAIL or otherwise.", and `ip-ratelimit-factor` "Default is 10, allowing 1/10 traffic to flow normally" |
| Per-resolution limits 32 upstream queries, 11 CNAME restarts | config leaves | unbound.conf `max-sent-count` "Default: 32", "Results in SERVFAIL when reached."; `max-query-restarts` "Default: 11"; RFC 4035 Section 5.4 bound; no operator need shown for a knob |
| Positive TTL cap 86400, negative cap 3600, configurable | fixed | unbound.conf `cache-max-ttl` "Default: 86400 (1 day)", `cache-max-negative-ttl` "Default: 3600"; operators tune these |
| Advertisement left to the existing leaves | a `self` keyword resolving to the resolver address in DHCP, RA and the L2TP pool | the three producers live in other plugins and components; a cross-plugin lookup is an import the plugin rules forbid; the guide shows the three leaves |
| A new `le dns query` tool for `.ci` | assert through the stub resolver's output | the stub prints records and RCODE only; RA, AA, AD, TC and size are the behavior under test |

## Known Limitations
- No DNSSEC validation (D-1 answered (b)): a validating stub on the LAN receives no RRSIGs through Ze, and no release claim may say Ze validates signatures (commit 5ff4834b67). The validator is `plan/post-release/features/spec-dns-resolver-dnssec-validation.md`.
- No full recursion: the resolver needs an operator-named upstream until `plan/post-release/features/spec-dns-resolver-recursion.md` lands (D-2).
- Upstream transport in forward mode is plain UDP and TCP; DNS over TLS to upstreams is not in this spec. The harness's DoT and DoH listeners are not offered on the resolver.
- DHCPv6 option 23 advertisement waits for `plan/post-release/features/spec-dhcpv6-server.md`; IPv6 hosts are served through RA RDNSS today.
- No summary is written for RFC 1034, 2308, 5452 or 6891 (no profile lists them); RFC 1035 and 2181 rows carry the proof.
- RFC4035-3-1 and RFC4035-3-2 stay open for the authoritative services; `Options.UDPSize` makes them closable there by setting the same option, which is that work's to do.

## Work Not Done
| Item | Spec |
|------|------|
| DNSSEC validation (D-1 answered (b)); runs once Thomas brings DNSSEC into scope | `plan/post-release/features/spec-dns-resolver-dnssec-validation.md` |
| Full recursion from the root; waits on Thomas's D-2 answer | `plan/post-release/features/spec-dns-resolver-recursion.md` |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

Every row in `RFC gaps` becomes a tagged unit (`RFC requirement:`) whose
discrimination record is written by `./le rfc record` in the commit of the code
it proves; every RFC 1035 and 2181 row that gains a resolver-side case has its
record re-recorded, and any audit verdict the change stales is re-judged in the
same commit (`ai/rules/planning.md`).

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

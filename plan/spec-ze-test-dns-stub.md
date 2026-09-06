# Spec: ze-test-dns-stub

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-06 |

<!-- Bucket: plan/ (this level). ze-test is a host developer binary, absent from
     the appliance image and from every operator surface, so no operator meets
     this gap. It is not pre-release either: the release binary builds and ships
     without it. The work it unblocks, plan/spec-firewall-domain-group.md, sits
     at this level too, and an enabler does not outrank the work it enables. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze-test` can serve a fake IRR whois server, a fake RPKI cache, a fake RADIUS
server, a fake TACACS+ server and a fake PeeringDB. It cannot serve DNS for a
name a test chooses. `ze-test cymru` is a DNS server, but it synthesizes one
answer shape (a Team Cymru TXT record derived from the queried ASN) and no test
consumes it.

The consequence is measured, not predicted. `plan/spec-firewall-domain-group.md`
names three `.ci` files that exist and have never run:
`test/plugin/firewall-domain-group-update.ci`,
`test/plugin/firewall-domain-group-clear.ci` and
`test/firewall/firewall-cli-domain-group-show.ci`. Each needs a Go fixture, and
each fixture's first step is "serve DNS on 127.0.0.1:53". None can be written.
A domain group's whole behavior is resolving a name, so with no server to
resolve against, the feature is proven at unit level only and no assertion is
made from outside the daemon.

The goal is one deterministic DNS server in `ze-test`, in the shape
`ze-test irr` already established, reachable two ways: as a background process a
`.ci` starts, and as an in-process server a compiled fixture starts and
reprograms while the test runs. The second way is what
`firewall-domain-group-update.ci` needs and what `ze-test irr` has no equivalent
of: its step 3 changes the answer for a name and dispatches a second update.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the `ze-test` surface inventory, including the
  `### ze-test irr` and `### ze-test rpki` sections this command must sit beside,
  and the "Netns launch mode" section that says where a privileged `.ci` runs.
  → Decision: a new mock server is documented as its own `### ze-test <name>`
  section carrying a `<!-- source: -->` anchor at the implementation file.
  → Constraint: a test that declares a capability it does not hold is SKIPPED
  with a reason, so an over-strict gate deletes coverage on hosts that could run
  the test.
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` grammar and the
  `// Design:` annotation every mock server file carries.
  → Constraint: `option=exclusive:group=<name>` is how two tests that contend
  for one node-wide resource are serialized; a fixed port is such a resource.
  → Decision: the per-test netns launch mode locks one OS thread, so the daemon,
  `ze-peer` and compiled fixtures fork-inherit the namespace and reach each other
  over 127.0.0.1.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1035.md` - the RCODE values the stub must be able to answer.
  → Constraint: RFC 1035 Section 4.1.1 defines rcode 3 as "Name Error -
  Meaningful only for responses from an authoritative name server, this code
  signifies that the domain name referenced in the query does not exist."
- [ ] `rfc/full/rfc2308.txt` - NODATA, the case a name that exists in one family
  and not the other produces. No `rfc/short/rfc2308.md` summary exists; the full
  text was read for this spec.
  → Constraint: RFC 2308 Section 2.2 states "NODATA is indicated by an answer
  with the RCODE set to NOERROR and no relevant answers in the answer section."
  A stub that answered NXDOMAIN for the AAAA of an IPv4-only name would tell the
  firewall plugin the name is GONE rather than that it holds no IPv6.

**Key insights:** (minimal context to resume after compaction)
- The seam a test points at the stub is the `system name-server` config leaf.
  `newResolvers` (`cmd/ze/hub/main_system.go`) reads `NameServers[0]` into
  `ResolverConfig.Server`, and `NewResolver`
  (`internal/component/resolve/dns/resolver.go`) appends port 53 when the value
  carries none.
- That leaf is declared `type zt:ip-address` in
  `internal/component/config/system/yang/ze-system-conf.yang`, so it cannot
  carry a port. The stub is therefore bound to port 53 for any test that drives
  the daemon's own resolver.
- A fixture CAN bind port 53: the netns launch mode drops only the `ze` daemon
  to an ordinary uid, because the `Credential` assignment in
  `runOrchestrated` (`internal/test/runner/runner_exec.go`) is guarded by
  `binName == binNameZe`, and its comment says peers and fixture helpers stay
  root.
- A stub answer with TTL 0 is not cached: `cache.put`
  (`internal/component/resolve/dns/cache.go`) returns before storing when the
  TTL is zero, on the reasoning that the server said not to cache.
- A TTL of 0 does not disarm the refresh schedule: `refreshInterval`
  (`internal/component/firewall/plugins/domain/schedule.go`) takes the maximum
  of the TTL and the group's floor.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/mock/irr/irr.go` - the model. One `Run(args []string) int`,
  a `flag.NewFlagSet` with `--port`, a listener on 127.0.0.1, a readiness line
  on stderr naming the bound port, an accept loop, and a package-level map from
  query to answer. `--empty-after-first` is its one behavior flag.
- [ ] `internal/test/mock/cymru/cymru.go` - the only DNS server in the tree. It
  serves UDP through `miekg/dns` `Server.ActivateAndServe` over a `net.PacketConn`
  and answers TXT for `*.asn.cymru.com` only, REFUSED for anything else. No `.ci`
  file references it.
- [ ] `internal/test/cli/register.go` - `registerRoot("irr", irr.Run, ...)` and
  the rest of the mock-server block; `registerRoot("fixture", fixture.Run, ...)`.
- [ ] `internal/test/fixture/fixture.go` - `Register(name, driver)` and the
  driver map. A fixture is compiled Go inside `ze-test`, so it can hold a server
  object and call methods on it between dispatches.
- [ ] `internal/component/resolve/dns/resolver.go` - `NewResolver`, `query`,
  `ResolveWithTTL`, `Status`, `statusFromRcode`. The client is built with the
  network set to UDP: Ze never queries over TCP, and a truncated answer is
  logged as a warning rather than retried.
- [ ] `internal/component/resolve/dns/cache.go` - `put` skips a zero TTL, and
  `maxTTL` caps rather than raises a response TTL.
- [ ] `internal/component/firewall/plugins/domain/domain.go` - `resolveAndRecord`
  branches on `Status`: a transport error and a non-authoritative status keep the
  last good addresses, NXDOMAIN empties the name's contribution, NOERROR is the
  group's content.
- [ ] `internal/component/firewall/plugins/domain/schedule.go` - `families` says
  both families are asked for every name, always; `refreshInterval` clamps a TTL
  up to the group's floor and down to one day.
- [ ] `internal/component/resolve/cmd/show_dns.go` - `handleDNSLookup` uses
  `resolvers.DNS.ResolveWithTTL` when the hub published a resolver, and
  `dnsLookupStdlib` only when it did not. The answer carries `records`, `count`,
  `ttl` and `status`.
- [ ] `internal/test/runner/caps.go` - `capsRequired` maps an accepted
  `option=needs-linux:caps=` token to the capability bits `probeCaps` tests.
  Tokens today: `net-admin`, `net-raw`, `bpf`. No token names
  CAP_NET_BIND_SERVICE.
- [ ] `internal/test/runner/runner_exec.go` - `runOrchestrated`, the netns launch
  mode, and the credential drop that applies to the `ze` binary alone.
- [ ] `internal/le/qemu/netns.go` - `netnsCapabilities` is set on the copies of
  `ze` and `ze-stripped` only; `ze-test` gets no file capability, which is why a
  fixture binding a privileged port depends on running as root rather than on a
  capability of its own.

**Behavior to preserve:**
- `ze-test irr`, `ze-test cymru` and every other registered mock keep their
  current flags and answers. This spec adds a sibling; it replaces nothing.
- The `system name-server` leaf keeps its `zt:ip-address` type. Nothing in the
  daemon's resolve path changes.
- Existing `option=needs-linux:caps=` tokens keep their meaning and their bits.

**Behavior to change:**
- None. Every change is additive: one new `ze-test` subcommand, one new mock
  package, one new caps token, two new `.ci` files, one new documentation
  section.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze-test dns --port <N>` started by a `.ci` `cmd=background` line. Entry format:
  argv.
- A compiled fixture in `internal/test/fixture/` starting the same server
  in-process. Entry format: a Go call carrying a listen address.
- Either way the wire entry is a DNS query datagram on UDP 127.0.0.1 at the
  bound port.

### Transformation Path
1. The daemon's `system name-server` value reaches `ResolverConfig.Server`
   through `newResolvers` (`cmd/ze/hub/main_system.go`).
2. `NewResolver` appends port 53 when the value carries no port, so the query
   leaves the daemon addressed to the stub.
3. The stub's handler reads the question name and type, looks the name up in its
   zone, and writes a reply carrying the rcode, the records and their TTL.
4. `Resolver.query` maps the reply's rcode to a `Status` and returns the records
   and the minimum answer TTL.
5. The caller branches: `handleDNSLookup` renders them, `resolveAndRecord`
   programs an nftables set from them.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test process ↔ daemon | DNS over UDP on 127.0.0.1, inside the test's network namespace when the netns launch mode is active | No |
| `.ci` runner ↔ stub process | `cmd=background` argv, plus the readiness line on stderr | No |
| Fixture ↔ stub | a Go method call inside `ze-test`, no transport | No |

### Integration Points
- `internal/test/cli/register.go` - one `registerRoot` line, the same way every
  other mock server is discovered.
- `internal/test/fixture/` - the two fixtures this spec adds register through
  `fixture.Register`, in the existing driver table for their suite.
- `internal/test/runner/caps.go` - one row in `capsRequired`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The stub speaks DNS on the wire. The daemon reaches it through its configured resolver, not through a test hook inside the resolver |
| No unintended coupling (components stay isolated) | Yes | `internal/test/mock/dns` imports `miekg/dns` and nothing from `internal/component/` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ze-test cymru` answers TXT it synthesizes from the queried ASN and has no `.ci` consumer; this stub answers a zone a test supplies. They share no answer and no consumer |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test tooling off every hot path; a query per test, not per packet |
| Registration over hardcoding | Yes | `registerRoot` for the subcommand, `fixture.Register` for the fixtures, a `capsRequired` row for the token. No switch, factory or central enumeration is edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A compiled fixture can bind 127.0.0.1:53 in the runs that matter | `runOrchestrated` (`internal/test/runner/runner_exec.go`): the credential drop is guarded by `binName == binNameZe`, and its comment says peers and fixture helpers stay root | Every DNS test must run outside the netns mode, or the port seam of A-4 becomes required work | AC-8's `.ci` running under `./le functional plugin` as root and under `./le qemu all-tests` | unvalidated |
| A-2 | A zero TTL in the stub's answer stops the daemon's resolver caching it, so a changed answer is seen on the next lookup | `cache.put` (`internal/component/resolve/dns/cache.go`) returns before storing when the TTL is zero | The answer-change test asserts a stale address; the fixture would have to dispatch `clear dns cache` between lookups | AC-7, which changes an answer and asserts the new address arrives with no cache-clearing command in between | unvalidated |
| A-3 | Ze's resolver never queries over TCP | `NewResolver` (`internal/component/resolve/dns/resolver.go`) builds its client with the network set to UDP; `query` logs a truncated response rather than retrying | A test using a large answer hangs or reads a truncated one; the stub then owes a TCP listener | The unit tests over the zone, which keep every answer inside one datagram, plus the limitation naming the producer | unvalidated |
| A-4 | No test in this spec needs the daemon pointed at a non-53 port | The leaf is `zt:ip-address` in `internal/component/config/system/yang/ze-system-conf.yang`; both new `.ci` files gate on the capability instead | A test that must run unprivileged cannot use the stub, and the leaf must learn to carry a port | The two `.ci` files running to green in the privileged population | unvalidated |
| A-5 | `CAP_NET_BIND_SERVICE` is capability bit 10 | Linux `capability.h`; the sibling constants at the head of `internal/test/runner/caps.go` follow the same numbering | The new token gates on the wrong bit, so the test skips on a host that could run it, or runs on one that cannot bind | The unit test that drives both polarities through the `hasCaps` seam, as `net-raw` already does | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Something on the host already listens on 127.0.0.1:53 (dnsmasq, a container resolver) | The stub's listen fails with "address already in use" | The failure is loud and names the port: `Run` returns 1 and the fixture returns the error. It must never be turned into a skip, which would delete the coverage this spec exists to add |
| R-2 | Two DNS tests run at once and contend for port 53 | Intermittent bind failures in the plugin suite | Both `.ci` files declare `option=exclusive:group=dns-stub-port-53`, the mechanism `firewall-domain-group-*.ci` already uses for the node-wide nftables ruleset |
| R-3 | The stub answers NXDOMAIN for the AAAA of an IPv4-only name, and a firewall test then reads a name as deleted | A domain-group test whose IPv6 set empties for a name that is alive | AC-3 makes existence a property of the NAME and the record set a property of the type, which is what RFC 2308 Section 2.2 requires |
| R-4 | The new caps token gates tests off every ordinary developer host, and the coverage is never seen | The two `.ci` files report SKIP in every local run | They run in the privileged population (`./le qemu all-tests`, and any root `./le functional plugin`), which is where every netlink `.ci` already runs. This is the same trade `caps=net-raw` makes for the ping tests |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator can reach. Every file is inside `ze-test`, the test runner, `test/`, and one documentation page. A wrong stub makes a test lie, which is why the red phase in step 5 is mandatory |
| How is it reverted? | Single commit revert. No config migration, no persisted state, no wire compatibility |
| Who else touches this path? | `plan/spec-firewall-domain-group.md` owns the three `.ci` files this unblocks and will write their fixtures against the API this spec defines. Nothing else in `plan/` names a DNS stub |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` line `cmd=background:exec=ze-test dns --port 53` | → | `dns.Run` in `internal/test/mock/dns/dns.go` | `test/plugin/dns-stub-lookup.ci` |
| A compiled fixture starting the server in-process | → | `dns.Start` and `dns.(*Server).Set` in `internal/test/mock/dns/server.go` | `test/plugin/dns-stub-answer-change.ci` |
| Operator command `show dns lookup <name> type A` against a daemon whose `system name-server` names the stub | → | `Resolver.ResolveWithTTL` reading the stub's reply | `test/plugin/dns-stub-lookup.ci` |
| `option=needs-linux:caps=net-bind` in a `.ci` header | → | `capsRequired` in `internal/test/runner/caps.go` | `TestCapsNetBindGateBothPolarities` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-test dns --port <N>` is started | It binds UDP on 127.0.0.1 at that port, writes one readiness line to stderr naming the bound port, and serves until killed. `--port 0` binds a port the OS chooses and the readiness line names it |
| AC-2 | A query arrives for a name in the stub's zone, of a type that name holds | The reply carries rcode NOERROR, one record per configured address, and the TTL configured for that answer |
| AC-3 | A query arrives for a name in the zone, of a type that name does NOT hold | The reply carries rcode NOERROR and an empty answer section (RFC 2308 Section 2.2), never NXDOMAIN |
| AC-4 | A query arrives for a name absent from the zone | The reply carries rcode NXDOMAIN (RFC 1035 Section 4.1.1) |
| AC-5 | The zone declares a name whose answer is SERVFAIL, and another whose answer is REFUSED | Each query for those names is answered with that rcode and no records, so a caller that branches on `Status.Authoritative()` takes its non-authoritative path |
| AC-6 | A fixture starts the server in-process, then changes the answer for one name and type while the server runs | Every query after the change is answered with the new value, and no query is dropped or delayed while the change is applied |
| AC-7 | A daemon resolves the same name twice, and the stub's answer changed in between, with the answer carrying TTL 0 | The daemon's second answer carries the new address: the zero TTL kept the first answer out of the resolver cache, and no cache-clearing command was needed |
| AC-8 | A daemon whose `system name-server` names the stub runs `show dns lookup <name> type A` and `type AAAA` | The rendered answer carries the stub's addresses for both families, and a lookup of a name absent from the zone reports the NXDOMAIN status |
| AC-9 | A `.ci` file declares `option=needs-linux:caps=net-bind` and the runner does not hold CAP_NET_BIND_SERVICE | The test is SKIPPED with a reason naming the capability. Holding it, the test runs |
| AC-10 | `ze-test dns --help` is run | The usage text lists every name in the zone with the type, addresses, TTL and rcode it answers, so an author picks a name without reading the source |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A test author starts the stub from a `.ci` line and asserts on a daemon's DNS answer | `cmd=background` -> `dns.Run` -> UDP listener -> daemon resolver -> `show dns lookup` -> `.ci` assertion | `test/plugin/dns-stub-lookup.ci` |
| 2 | A fixture author serves a name, changes its address, and asserts the daemon sees the change | fixture -> `dns.Start` -> dispatch -> `Set` -> dispatch -> assertion | `test/plugin/dns-stub-answer-change.ci` |
| 3 | A test author needs a name that fails to resolve, and one that no longer exists | zone entries `broken.example.test` (SERVFAIL) and any absent name (NXDOMAIN) -> `Status` -> the caller's non-authoritative and authoritative-empty branches | `TestZoneAnswersEveryRcode`, and `test/plugin/dns-stub-lookup.ci` for the NXDOMAIN path over the operator command |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZoneAnswersEveryRcode` | `internal/test/mock/dns/zone_test.go` | Each declared name answers the rcode it declares, and an absent name answers NXDOMAIN | |
| `TestNameWithoutThatTypeAnswersNoData` | `internal/test/mock/dns/zone_test.go` | A name holding only A answers NOERROR with an empty answer section for AAAA (AC-3) | |
| `TestAnswerCarriesTheDeclaredTTL` | `internal/test/mock/dns/server_test.go` | The TTL on every returned record is the one the zone entry declares, including 0 | |
| `TestSetChangesTheNextAnswer` | `internal/test/mock/dns/server_test.go` | A query after `Set` returns the new value; a query before it returns the old one (AC-6) | |
| `TestStartBindsChosenAndEphemeralPort` | `internal/test/mock/dns/server_test.go` | Port 0 binds and reports a real port; a busy port returns an error rather than a silently dead server | |
| `TestRunReportsListenFailure` | `internal/test/mock/dns/dns_test.go` | `Run` returns 1 and writes a message naming the port when the bind fails (R-1) | |
| `TestUsageListsEveryZoneName` | `internal/test/mock/dns/dns_test.go` | The usage text is derived from the zone table, so a name added without a usage line fails (AC-10) | |
| `TestMalformedQueryIsAnsweredNotDropped` | `internal/test/mock/dns/server_test.go` | A message with no question section is answered REFUSED and the server keeps serving | |
| `TestCapsNetBindGateBothPolarities` | `internal/test/runner/caps_test.go` | The `net-bind` token maps to CAP_NET_BIND_SERVICE, skips without it and runs with it, driven through the `hasCaps` seam as `net-raw` is | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--port` | 0-65535 | 65535 | -1 | 65536 |
| answer TTL (seconds) | 0-4294967295 | 4294967295 | N/A (unsigned 32-bit) | N/A (unsigned 32-bit) |
| addresses per answer | 0-64 | 64 | N/A | 65 refused by the zone declaration, matching `maxAddressesPerName` in `internal/component/firewall/plugins/domain/sets.go` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dns-stub-lookup` | `test/plugin/dns-stub-lookup.ci` | An operator asks a daemon to resolve a name and the answer is the stub's, for A, for AAAA, and for a name that does not exist | |
| `dns-stub-answer-change` | `test/plugin/dns-stub-answer-change.ci` | A name's address changes while the daemon runs, and the next lookup reports the new one | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | The stub is test tooling, not a protocol surface Ze ships. It speaks DNS to Ze's own resolver, and `test/plugin/dns-stub-lookup.ci` is the assertion that the two agree on the wire | N-A |

## Files to Modify
- `internal/test/cli/register.go` - one `registerRoot("dns", ...)` line in the
  mock-server block.
- `internal/test/runner/caps.go` - one `capsRequired` row for `net-bind`, and its
  entry in the token documentation block above the table.
- `internal/test/fixture/plugin_fixture_06.go` - register the two new fixture
  drivers in the existing plugin driver table (the file that already holds
  `plugin/dns-lookup-show`).
- `docs/functional-tests.md` - a `### ze-test dns` section beside `### ze-test irr`,
  with the zone table and a `<!-- source: -->` anchor, and the `net-bind` token
  in the capability list.

## Files to Create
- `internal/test/mock/dns/dns.go` - `Run(args []string) int`: the flag set, the
  usage text derived from the zone, the listen, the readiness line.
- `internal/test/mock/dns/server.go` - the `Server` type and its operations.
- `internal/test/mock/dns/zone.go` - the answer record type and the default zone.
- `internal/test/mock/dns/doc.go` - the package comment, as `cymru` and `irr` have.
- `internal/test/mock/dns/dns_test.go`, `server_test.go`, `zone_test.go` - the
  unit tests above.
- `internal/test/fixture/plugin_fixture_dns_stub.go` - the two fixture drivers.
- `test/plugin/dns-stub-lookup.ci`, `test/plugin/dns-stub-answer-change.ci`.

### The server's operations

No code in a spec, so the surface is a table. Every operation is on one type.

| Operation | Input | Effect |
|-----------|-------|--------|
| Start | listen address, host and port; port 0 asks the OS to choose | Binds UDP, serves the default zone, returns the running server or an error |
| Addr | none | The bound address, so a caller can print it or hand it to a daemon |
| Set | name, record type, one answer record | Replaces what the stub answers for that name and type. In effect for the next query. A name the zone does not carry is added and answers NOERROR, so a fixture can serve a name the package never declared |
| Close | none | Stops serving and releases the port |
| Wait | none | Blocks until the server stops and reports why. ADDED at implementation: the process form has no step after Start and needs something to block on, and Close is what makes it return. A fixture calls Close instead |

### The answer record

CORRECTED at closure to match the code. The design above put `Rcode` on the
per-type answer, and the implementation moved it to the NAME. A response code is
a property of the name, not of the record type: a server that cannot answer
cannot answer for either family, so a SERVFAIL name would have had to declare
that fact twice, once per family, with nothing stopping the two from
disagreeing. Splitting the two also states RFC 2308 Section 2.2 in the type
system, because a name that exists holds one code and a type it does not carry
simply has no entry. The default zone table below already carried one Rcode
column per NAME, so the spec disagreed with itself before the code was written.
Producers: `Name` and `Answer` (`internal/test/mock/dns/zone.go`), `answerFor`
(`internal/test/mock/dns/server.go`).

The zone entry, one per name:

| Field | Type | Description |
|-------|------|-------------|
| Rcode | one of NOERROR, SERVFAIL, REFUSED | The response code every query for this name is answered with. NXDOMAIN never appears here: a name absent from the zone answers it and needs no entry |
| Answers | record type to answer record | The records this name carries for each type. A type absent from it answers NODATA |

The answer record, one per name and record type:

| Field | Type | Description |
|-------|------|-------------|
| Addresses | list of IP addresses | The records the reply carries when the name's Rcode is NOERROR. Empty is legal and means NODATA |
| TTL | seconds, unsigned 32-bit | The TTL every record in this answer carries. 0 tells the resolver not to cache it |

### The default zone

| Name | A | AAAA | TTL | Rcode | Why it exists |
|------|---|------|-----|-------|---------------|
| `web.example.test` | 203.0.113.10 | 2001:db8::10 | 0 | NOERROR | The dual-family name the three firewall `.ci` files already name |
| `v4only.example.test` | 198.51.100.10 | none | 0 | NOERROR | NODATA for AAAA: the case a stub that answers NXDOMAIN gets wrong (AC-3) |
| `cached.example.test` | 203.0.113.20 | none | 300 | NOERROR | A non-zero TTL, for a test of the resolver cache and of a TTL-driven schedule |
| `broken.example.test` | none | none | 0 | SERVFAIL | The server-side failure that must keep a caller's last good answer |
| `refused.example.test` | none | none | 0 | REFUSED | The second non-authoritative rcode, which classifies through a different branch of `statusFromRcode` |
| any other name | none | none | 0 | NXDOMAIN | The authoritative "this name is gone" |

Addresses come from the documentation ranges of RFC 5737 (203.0.113.0/24,
198.51.100.0/24) and RFC 3849 (2001:db8::/32). Names sit under `.test`, which
RFC 6761 reserves for testing.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Nothing the daemon reads changes. The stub is pointed at through the existing `system name-server` leaf |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | Yes | `ze-test dns --port`. `ai/rules/cli.md` places the `--flag` form in the offline host tooling, which is where every `ze-test` mock server already declares its flags. The flag-register feeder does not judge them: `ze-test` flag sets are counted out of scope by `internal/le/cligrammar/report.go` |
| CLI grammar (keyword before value) | Yes | `--port <N>` is keyword before value; the subcommand noun `dns` comes first, as `irr` and `rpki` do |
| Editor autocomplete | No | Not a daemon command |
| Functional test for new RPC/API | Yes | `test/plugin/dns-stub-lookup.ci`, `test/plugin/dns-stub-answer-change.ci` |
| Pipe completeness | N-A | The stub is a server, not a command with an answer. Its only output is one readiness line on stderr, the shape `ze-test irr`, `ze-test cymru` and `ze-test rpki` all use. There is no response payload for `ApplyPipes` to render |
| Env var registration | No | No new environment key |
| Doctor check for runtime dependencies | No | The listen port belongs to a test process, not to a daemon runtime dependency |
| Prometheus counters/metrics | No | Test tooling |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The feature is a developer tool; `docs/features.md` describes the product |
| 2 | Config syntax changed? | No | No config change |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` documents the operator's `ze` commands, not `ze-test` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Developer tooling |
| 7 | Wire format changed? | No | The stub speaks standard DNS; nothing Ze encodes changes |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The stub is not a Ze protocol surface. It cites RFC 1035 and RFC 2308 for the answers it produces; no `rfc/short/` support row changes |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: a `### ze-test dns` section, and the `net-bind` token in the capability list |
| 11 | Affects daemon comparison? | No | No product capability changes |
| 12 | Internal architecture changed? | No | A new leaf package under `internal/test/mock/` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | A `ze-test` subcommand is not in the daemon inventory those pages list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, not from memory: run `./le spec citation anchors spec plan/spec-ze-test-dns-stub.md` during implementation. `internal/test/cli/register.go` and `internal/test/runner/caps.go` are already anchored from `docs/functional-tests.md`, which row 10 updates |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `### ze-test irr` section is the model to match; check its usage line format and reuse it |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the entry points exist and fail.
   - Tests: `test/plugin/dns-stub-lookup.ci` written first and failing because
     `ze-test dns` is not a subcommand.
   - Files: `internal/test/mock/dns/dns.go` with `Run` returning an error,
     `internal/test/cli/register.go`.
   - Verify: `ze-test dns --help` prints usage; the `.ci` fails at the assertion
     rather than at "unknown subcommand".
2. **Phase: Zone and answers** -- the record type, the default zone, the handler.
   - Tests: `TestZoneAnswersEveryRcode`, `TestNameWithoutThatTypeAnswersNoData`,
     `TestAnswerCarriesTheDeclaredTTL`, `TestUsageListsEveryZoneName`,
     `TestMalformedQueryIsAnsweredNotDropped`.
   - Files: `internal/test/mock/dns/zone.go`, `server.go`.
   - Verify: each test fails, then passes. AC-2 to AC-5 and AC-10 hold.
3. **Phase: The in-process server** -- `Start`, `Addr`, `Set`, `Close`.
   - Tests: `TestSetChangesTheNextAnswer`, `TestStartBindsChosenAndEphemeralPort`,
     `TestRunReportsListenFailure`.
   - Files: `internal/test/mock/dns/server.go`, `dns.go`.
   - Verify: AC-1 and AC-6 hold.
4. **Phase: The capability token** -- `net-bind`.
   - Tests: `TestCapsNetBindGateBothPolarities`.
   - Files: `internal/test/runner/caps.go`.
   - Verify: AC-9 holds, with both polarities driven through the `hasCaps` seam.
5. **Phase: The two functional tests and their fixtures.**
   - Tests: `test/plugin/dns-stub-lookup.ci`, `test/plugin/dns-stub-answer-change.ci`.
   - Files: `internal/test/fixture/plugin_fixture_dns_stub.go`,
     `internal/test/fixture/plugin_fixture_06.go`.
   - Verify: AC-7 and AC-8 hold. Then force the red phase required by
     `ai/rules/interop-and-goal-validation.md`: make the stub answer a fixed
     address whatever `Set` was given, rebuild `ze-test`, confirm
     `dns-stub-answer-change` goes RED, restore, confirm GREEN, and record the
     red output.
6. **Phase: Documentation.**
   - Files: `docs/functional-tests.md`.
   - Verify: the `### ze-test dns` section carries the zone table, the usage
     line, and the source anchor; the capability list names `net-bind`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and the two `.ci` files ran rather than being written and left |
| Feature completeness | Both entry points have a caller: the subcommand from a `.ci` line, the `Server` API from a fixture. No exported symbol is reachable only from a `_test.go` file |
| Correctness | A name that exists in one family answers NODATA, never NXDOMAIN, for the other. A zero TTL is served as zero rather than replaced by a default |
| Naming | The zone's names sit under `.test`; the addresses come from the documentation ranges; the caps token is `net-bind`, matching the `net-admin` and `net-raw` spelling |
| Data flow | The daemon reaches the stub over UDP through its configured resolver. No test hook is added inside `internal/component/resolve/` |
| Rule: `ai/rules/simplicity.md` | The stub serves UDP only, because UDP is what Ze's client queries with. No TCP listener, no zone file parser, no control socket is added before a test needs one |
| Rule: `ai/rules/principles.md` | A bind failure returns an error naming the port. It is never converted into a skip or into a server that accepts nothing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `ze-test dns` is a registered subcommand | `./le build` then `./bin/ze-test dns --help` |
| The stub answers every rcode in the zone | `./le test unit package ./internal/test/mock/dns/...` |
| A daemon resolves through it over the operator command | `./le functional plugin` run privileged, tests `dns-stub-lookup` and `dns-stub-answer-change` |
| The `net-bind` token gates on CAP_NET_BIND_SERVICE | `./le test unit package ./internal/test/runner/` |
| The red phase was observed | The recorded RED output pasted into the closure section |
| The documentation section exists | `grep -n "ze-test dns" docs/functional-tests.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A malformed or question-less message is answered REFUSED and never panics the server, the way `handleCymruDNS` answers a question-less message |
| Listen scope | The listener binds 127.0.0.1 only. A stub answering on every address would serve a developer's whole network a fake zone |
| Resource exhaustion | One UDP socket, no per-query allocation of unbounded size. The zone is fixed and small |
| Privilege | The stub needs CAP_NET_BIND_SERVICE only because port 53 is privileged, and it never asks for more. It drops nothing and elevates nothing |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| `.ci` fails to bind port 53 | R-1: report the listener holding the port. Never convert it to a skip |
| `.ci` reads a stale address after `Set` | A-2 is broken: the answer's TTL is not reaching the resolver's cache decision. Re-read `cache.put` |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The IRR stub and the DNS stub differ in one structural way, and it decides the
  design: an IRR answer is fixed for the life of the process, and a DNS answer is
  the thing a test changes. `--empty-after-first` is the IRR mock's whole answer
  to "the server behaves differently the second time", and it is a flag because
  nothing can call into that process. A fixture is compiled into `ze-test`, so it
  can hold the server and change one entry. That is why this spec ships an
  in-process type as well as a subcommand, and why no second behavior flag is
  needed.
- A TTL is not only what the feature under test schedules on: it is also what
  decides whether the daemon asks the stub again at all. A stub that answered
  with a comfortable 300-second TTL would be invisible to a test that changes an
  answer, and the test would report the daemon's cache back to itself. Zero is
  the default in the zone for that reason.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A subcommand AND an in-process type | Subcommand only, as `ze-test irr` | `firewall-domain-group-update.ci` changes the answer mid-test, which a process with no control channel cannot do. Both surfaces get a consumer in this spec, so neither is speculative |
| The zone decides the rcode, per name | A `--rcode` flag for the whole server | One server then serves a group holding both a healthy name and a failing one, which is what the firewall's keep-last-good behavior needs, and no restart is required to change the mix. It is also the shape `irrResponses` already uses |
| Serve UDP only | Serve UDP and TCP | The client `NewResolver` builds is UDP-only and a truncated answer is logged rather than retried over TCP, so a TCP listener would have no caller. The trigger that adds one is named in Known Limitations |
| Keep `ze-test cymru` | Fold it into this stub as a TXT zone | Replacing it would mean deleting it first (`ai/rules/no-layering.md`), and its answers are synthesized from the queried ASN rather than read from a zone. No `.ci` consumes it, so nothing is served by moving it now |
| Bind port 53, gated by a new caps token | Teach `system name-server` to carry a port | The leaf is an IP address and its value is also written to `resolv.conf`, which cannot express a port; changing it touches the leaf type, the `resolv.conf` writer, and `dnsServerResponds` in `internal/component/doctor/checks_reach.go`, which joins port 53 onto every configured value. That is a product change with an operator-visible surface, and it is not what unblocks the three files, whose own tests already run privileged |
| A `net-bind` token rather than reusing `net-admin` | Declare `caps=net-admin` on the DNS tests | A token whose name says one capability while the test needs another is a guard that cannot evaluate what it claims (`ai/rules/evidence.md`, and the comment on `capsRequired` says exactly this). It would also skip on a host that can bind port 53 but cannot program netlink |

## Known Limitations
- The stub answers over UDP only. A consumer that needs a reply larger than the
  requester's UDP size, or one that queries TCP first, is not served. Nothing in
  Ze does either today: the resolver is UDP-only and the largest zone answer is
  two records.
- A test can point the daemon at the stub only on port 53, because
  `system name-server` is an IP address with no port. That confines the stub's
  daemon-driven tests to runs that hold CAP_NET_BIND_SERVICE, which is the
  privileged population `./le qemu all-tests` and a root `./le functional`
  provide. A test that must run unprivileged needs the leaf to accept a port,
  which is a product change with an operator-visible surface (the `resolv.conf`
  writer and `checkDNSResolvers` both assume no port), and it belongs to a spec
  of its own. That spec does not exist and is not written here, because no test
  in this spec or in `plan/spec-firewall-domain-group.md` needs it: write it when
  a test does, in `plan/` at this level.
- The zone is fixed in Go, not loaded from a file. A test needing a name the zone
  does not carry uses the in-process server and `Set`, or adds a name to the zone
  with its usage line.
- `ze-test cymru` stays a separate DNS server. Two DNS mocks now exist, serving
  different answer kinds to different callers.

## RFC Documentation (Scope: protocol)

The stub is tooling, not a Ze protocol surface, so it claims no conformance and
changes no `rfc/short/` support row. Two requirements govern the answers it
produces, and each is quoted above its handling code:

- RFC 1035 Section 4.1.1: "Name Error - Meaningful only for responses from an
  authoritative name server, this code signifies that the domain name referenced
  in the query does not exist." A name absent from the zone answers NXDOMAIN.
- RFC 2308 Section 2.2: "NODATA is indicated by an answer with the RCODE set to
  NOERROR and no relevant answers in the answer section." A name in the zone that
  holds no record of the queried type answers NOERROR with an empty answer
  section.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `internal/test/mock/dns/` (`dns.go`, `server.go`, `zone.go`, `doc.go`): the
  stub. `Run` is the `ze-test dns` subcommand, `Start`/`Addr`/`Set`/`Close`/`Wait`
  are the in-process form, `defaultZone` is the six-name zone and `handle` is the
  answer path.
- `internal/test/cli/register.go`: `registerRoot("dns", dnsmock.Run, ...)`.
- `internal/test/runner/caps.go`: the `net-bind` token, `capNetBindService`, and
  the `capsRequired` row that binds them.
- `internal/test/fixture/plugin_fixture_dns_stub.go`: `dnsStubLookup` and
  `dnsStubAnswerChange`, registered in the existing plugin driver table
  (`internal/test/fixture/plugin_fixture_06.go`).
- `test/plugin/dns-stub-lookup.ci` and `test/plugin/dns-stub-answer-change.ci`.
- `ai/PACKAGE-MAP.md`: the generated row for the new package
  (`./le discovery-index check` reports it up to date at closure).

### Bugs Found/Fixed
- `ze-test <mock> --help` printed nothing at all. `crashlog.Init`
  (`internal/core/crashlog/crashlog.go`) replaces `os.Stderr` with a pipe and
  `flushCrashlog` (`cmd/ze/dispatch.go`, called from `main`, `cmd/ze/main.go`)
  is its only drain, so `flag.ExitOnError` exits past the flush and discards the
  usage. Fixed for `dns` only, with `flag.ContinueOnError` and a `Run` that
  returns a code; `TestHelpPrintsTheZone` covers it. Measured the same for `irr`
  and `rpki` and NOT fixed there. Class row:
  `plan/journal/output-lost-to-an-exit-past-the-flush.md`.

### Documentation Updates
- `docs/functional-tests.md` carries a `### ze-test dns` section (zone table,
  the NODATA paragraph, the TTL paragraph, the two forms, the port-53 reason,
  three `<!-- source: -->` anchors) and a `caps=` token table naming `net-bind`.
  **The text is written and NOT committed.** Another session holds an
  uncommitted ExaBGP hunk in the same file (`exaBGPPythonPackage`, still absent
  from HEAD at closure), and staging the file would carry that session's work.
  Recorded as debt: `plan/journal/documentation-stranded-by-a-siblings-hunk.md`.
- `ai/PACKAGE-MAP.md` is generated; regenerated rather than hand-edited.

### Deviations from Plan
- The response code moved from the per-type answer record to the NAME. The
  design table in "The answer record" has been CORRECTED above rather than left
  disagreeing with `Name` and `Answer` (`internal/test/mock/dns/zone.go`). The
  spec's own default-zone table already carried one Rcode per name, so the
  design contradicted itself before any code was written.
- `Wait` was added to the server's operations. The process form has nothing to
  block on after `Start`, and `Close` is what makes `Wait` return. Recorded in
  the operations table above.
- `TestSetAddsANameTheZoneDoesNotCarry` and `TestEveryZoneNameResolves` were
  added beyond the TDD plan; `TestRunRefusesAPortOutsideTheRange` is the plan's
  `--port` boundary row as a named test.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The design put the response code on the answer record, one per name AND type | A response code is a property of the name: a server that cannot answer cannot answer for either family, and a SERVFAIL name would have declared the fact twice with nothing keeping the two consistent | Writing `defaultZone`: `broken.example.test` needed a SERVFAIL entry for A and another for AAAA, both carrying no addresses | Spec's "The answer record" section corrected at closure to the shape the code took |
| escalation | `ze-test dns --help` printed nothing, and so does every other `ze-test` mock's `--help` | `flag.ExitOnError` calls `os.Exit` from inside the flag package, past the only drain of the pipe `crashlog.Init` put over stderr | AC-10 could not be demonstrated: the built binary exited 0 and wrote 0 bytes | Fixed for `dns`; class row written for the rest at `plan/journal/output-lost-to-an-exit-past-the-flush.md`, which names the source repair as a decision about `crashlog` rather than an edit |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One deterministic DNS server in `ze-test`, in the `ze-test irr` shape | Done | `Run`, `internal/test/mock/dns/dns.go` | `registerRoot("dns", ...)`, `internal/test/cli/register.go` |
| Reachable as a background process a `.ci` starts | Done | `test/plugin/dns-stub-lookup.ci`, line `cmd=background:seq=1:exec=ze-test dns --port 53` | |
| Reachable as an in-process server a fixture reprograms while the test runs | Done | `dnsStubAnswerChange`, `internal/test/fixture/plugin_fixture_dns_stub.go` | Holds the `*Server` and calls `Set` between two dispatches |
| A name that exists in one family answers NODATA for the other | Done | `handle` and `answerFor`, `internal/test/mock/dns/server.go` | `carries` is false for an absent type, so no record is appended and the code stays NOERROR |
| Unblock the three firewall domain-group `.ci` files | Changed | not attempted | Those files run the daemon with no external-plugin block and launch their fixture as a separate process, so it has no channel to dispatch on. That is a design decision inside `plan/spec-firewall-domain-group.md`, which owns them |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestStartBindsChosenAndEphemeralPort`, `TestRunReportsListenFailure` | `Start` returns once `NotifyStartedFunc` fires; `Run` writes `ze-test dns: listening on port %d` |
| AC-2 | Done | `TestAnswerCarriesTheDeclaredTTL`, `TestZoneAnswersEveryRcode` | |
| AC-3 | Done | `TestNameWithoutThatTypeAnswersNoData`, and `expect=stdout:contains=v4only.example.test type AAAA -> records none status NOERROR` in `test/plugin/dns-stub-lookup.ci` | `handle` appends records only when `rcode == RcodeSuccess && carries` |
| AC-4 | Done | `TestZoneAnswersEveryRcode` (`absent.example.test`); `answerFor` returns `RcodeNameError` for an unheld key | |
| AC-5 | Done | `TestZoneAnswersEveryRcode` rows for `broken` and `refused` | `statusFromRcode` (`internal/component/resolve/dns/resolver.go`) maps both to a status `Status.Authoritative` rejects, which is the branch `resolveAndRecord` (`internal/component/firewall/plugins/domain/domain.go`) uses to keep the last good answer |
| AC-6 | Done | `TestSetChangesTheNextAnswer`, `test/plugin/dns-stub-answer-change.ci` | `Set` takes the write lock and replaces the whole `Answer`, so a reader never observes it half applied |
| AC-7 | Done | `test/plugin/dns-stub-answer-change.ci`, both `expect=` lines | `cache.put` (`internal/component/resolve/dns/cache.go`) returns before storing a zero TTL |
| AC-8 | Done | `test/plugin/dns-stub-lookup.ci`, five `expect=stdout:contains=` lines | |
| AC-9 | Done | `TestCapsNetBindGateBothPolarities` | Both polarities driven through the `hasCaps` seam; `SkipReason` becomes `StateSkip` in `runTest` (`internal/test/runner/runner_exec.go`) and in the worker of `internal/test/runner/parallel.go` |
| AC-10 | Done | `TestHelpPrintsTheZone`, `TestUsageListsEveryZoneName` | `zoneUsage` is derived from the zone table, so a name added without a usage line cannot exist |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestZoneAnswersEveryRcode` | Done | `internal/test/mock/dns/zone_test.go` | |
| `TestNameWithoutThatTypeAnswersNoData` | Done | `internal/test/mock/dns/zone_test.go` | |
| `TestAnswerCarriesTheDeclaredTTL` | Done | `internal/test/mock/dns/server_test.go` | |
| `TestSetChangesTheNextAnswer` | Done | `internal/test/mock/dns/server_test.go` | |
| `TestStartBindsChosenAndEphemeralPort` | Done | `internal/test/mock/dns/server_test.go` | |
| `TestRunReportsListenFailure` | Done | `internal/test/mock/dns/dns_test.go` | |
| `TestUsageListsEveryZoneName` | Done | `internal/test/mock/dns/dns_test.go` | |
| `TestMalformedQueryIsAnsweredNotDropped` | Done | `internal/test/mock/dns/server_test.go` | |
| `TestCapsNetBindGateBothPolarities` | Changed | `internal/test/runner/caps_option_test.go` | The plan named `caps_test.go`; the package's existing gate tests live in `caps_option_test.go` and the new one joined them |
| `--port` boundary row (-1, 65536) | Changed | `TestRunRefusesAPortOutsideTheRange`, `internal/test/mock/dns/dns_test.go` | The plan carried it as a table row rather than a named test |
| `TestHelpPrintsTheZone` | Changed | `internal/test/mock/dns/dns_test.go` | Added: `zoneUsage` being right does not prove `--help` reaches a reader, which is the defect the crashlog flush produced |
| `TestSetAddsANameTheZoneDoesNotCarry`, `TestEveryZoneNameResolves` | Changed | `internal/test/mock/dns/server_test.go`, `zone_test.go` | Added beyond the plan |
| `dns-stub-lookup`, `dns-stub-answer-change` | Done | `test/plugin/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/mock/dns/dns.go`, `server.go`, `zone.go`, `doc.go` | Done | |
| `internal/test/mock/dns/dns_test.go`, `server_test.go`, `zone_test.go` | Done | |
| `internal/test/fixture/plugin_fixture_dns_stub.go` | Done | |
| `internal/test/fixture/plugin_fixture_06.go` | Done | Two driver-table rows |
| `internal/test/cli/register.go` | Done | One `registerRoot` line |
| `internal/test/runner/caps.go` | Done | One const, one `capsRequired` row, the token documentation block |
| `test/plugin/dns-stub-lookup.ci`, `dns-stub-answer-change.ci` | Done | |
| `docs/functional-tests.md` | Partial | Written, not committed. See Documentation Updates and Work Not Done |
| `ai/PACKAGE-MAP.md` | Changed | Not in the plan; generated, and the generator's check refuses a stale one |

### Audit Summary
- **Total items:** 39
- **Done:** 31
- **Partial:** 1 (`docs/functional-tests.md`, blocked by another session's hunk)
- **Skipped:** 0
- **Changed:** 7 (recorded in Deviations and in the tables above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A test can point a real daemon at a DNS server whose answers it chose, and read them back through an operator command | functional | `test/plugin/dns-stub-lookup.ci`: five `expect=stdout:contains=` lines over `show dns lookup` for A, AAAA, the NODATA family and an absent name. Red observed with NODATA turned into NXDOMAIN: the AAAA line read `status=NXDOMAIN` |
| A test can change one answer while the daemon runs and see the change with no cache-clearing command | functional | `test/plugin/dns-stub-answer-change.ci`: `before the change -> records 203.0.113.10` then `after the change -> records 203.0.113.11`. Red observed with `Set` made a no-op: the second line read `203.0.113.10` |
| A name that holds one family answers the other NODATA, so a caller does not read it as deleted | functional + unit | `TestNameWithoutThatTypeAnswersNoData` queries a real UDP server and asserts NOERROR with zero records; the producer is `handle` (`internal/test/mock/dns/server.go`), which appends a record only when `rcode == RcodeSuccess && carries`. The consequence is at `resolveAndRecord` (`internal/component/firewall/plugins/domain/domain.go`): a non-authoritative status keeps the last good addresses, and a NOERROR with no records contributes none |
| The capability gate names the capability the test actually needs | unit, both polarities | `TestCapsNetBindGateBothPolarities` asserts `capsRequired["net-bind"]` holds `capNetBindService` and not `capNetAdmin`, that the probe is asked about `net-bind`, that the skip reason names it, and that the test RUNS when the probe answers yes. The bit number was read from the kernel's `include/linux/capability.h`, where `CAP_NET_BIND_SERVICE` is 10 |
| An author can pick a zone name without reading the source | unit | `TestUsageListsEveryZoneName` derives its expectations from `defaultZone()`, so a name added without a usage line fails; `TestHelpPrintsTheZone` proves the text reaches a reader through `Run` rather than only through `zoneUsage` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The `### ze-test dns` section and the `caps=` token table in `docs/functional-tests.md` are written and uncommitted | Another session holds an uncommitted ExaBGP hunk in the same file; staging the file would carry that session's work, which `ai/rules/git-safety.md` forbids | No spec: the text exists in the working tree and lands with the next commit that legitimately carries this file. The risk that it does not is recorded at `plan/journal/documentation-stranded-by-a-siblings-hunk.md` |
| The three firewall domain-group `.ci` files still have no fixture | They run the daemon with no external-plugin block and launch their fixture as a separate process, so it holds no channel to dispatch on. Giving them one is a design decision about those files | `plan/spec-firewall-domain-group.md` |
| `flag.ExitOnError` still discards the usage of `irr`, `cymru`, `rpki`, `peeringdb`, `radius-mock`, `rtr-mock` and `tacacs-mock` | The source repair is a decision about `crashlog` owning an exit path, which touches the shipped daemon's crash capture and is out of scope for a `ze-test` mock | Journal class `plan/journal/output-lost-to-an-exit-past-the-flush.md`, which carries the measurement and the two candidate repairs |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ze-test-dns-stub-zeclose-dnsstub-7742.md` (14 code files, verdict=clean) |
| `./le spec session review check` | `review_gate: OK (14 code files, clean, hashes match tmp/review/ze-test-dns-stub-zeclose-dnsstub-7742.md)`. The command also NOTEs that it could not determine the running model, so the review-model boundary is unchecked by the tool; the phase boundary holds regardless, because this closure did not author the diff |
| Rounds | 2. Round 1 found three ISSUEs, all in the record rather than in the product: a design table the code had rejected, two spec citations commit B would dangle, and an unrecorded documentation debt. Round 2 re-read each fix against its producer and found nothing further |
| Reviewer lenses used | wiring, functional-test coverage, documentation drift, removed-behaviour, logic correctness, guard audit, style pass over `docs/contributing/ze-go-style.md`, simplicity, security, RFC (1035 Section 4.1.1 and 2308 Section 2.2) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The spec's "The answer record" table declares `Rcode` per name AND type; the code declares it per NAME. A design the code rejected, left standing as if it were current | `plan/spec-ze-test-dns-stub.md`, "The answer record" | Table corrected and split into a zone entry and an answer record, with the reason and the producers named |
| 2 | ISSUE | Commit B deletes the spec, and both `.ci` files carried `# Spec: plan/spec-ze-test-dns-stub.md`, which `./le doc check links` resolves | `test/plugin/dns-stub-lookup.ci`, `test/plugin/dns-stub-answer-change.ci` | Restated as the bare stem `spec-ze-test-dns-stub`, the form `test/ui/doctor-config-claims.ci` already uses |
| 3 | ISSUE | `docs/functional-tests.md` cannot be committed, and nothing durable would have recorded that the text exists but never landed | `docs/functional-tests.md` | Journal class `plan/journal/documentation-stranded-by-a-siblings-hunk.md` plus the Work Not Done row above |
| 4 | NOTE | A fixture forked by a credential-dropped `ze` inherits its uid without `cap_net_bind_service`, which only the `ze` and `ze-stripped` copies are given (`prepareNetnsBinaries`, `internal/le/qemu/netns_linux.go`), so `dnsStubAnswerChange`'s bind on 53 would fail EACCES under the per-test netns launch mode. Unreached today because `vmSuites` gives `plugin` `Namespace: guestRoot` | `internal/test/fixture/plugin_fixture_dns_stub.go` | The constraint written onto `dnsStubAddress`, which is where the next author reads why the port is 53 |
| 5 | NOTE | `answerFor`'s third return value cannot change the reply: an `Answer` present with zero addresses produces the same message as an absent entry, so `&& carries` in `handle` is a condition with no outcome | `internal/test/mock/dns/server.go`, `answerFor` and `handle` | Left as written. The flag is what states RFC 2308 Section 2.2 at the branch, and removing it would trade a named distinction for one dimension of return type in test-only code |
| 6 | NOTE | `reply.Authoritative` is set for SERVFAIL and REFUSED as well as for NOERROR and NXDOMAIN | `internal/test/mock/dns/server.go`, `handle` | Left as written. Ze's resolver reads the rcode (`statusFromRcode`) and the AD bit (`dnssecDecision`), never AA |
| 7 | NOTE | `maxAddressesPerAnswer` copies `maxAddressesPerName` (`internal/component/firewall/plugins/domain/sets.go`) with a comment naming the source and no check comparing them | `internal/test/mock/dns/zone.go` | Left as written. Both read 64 at closure, and a stub bound that drifts from the product's cap changes what a test may declare, never what the product does |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/mock/dns/dns.go` | Yes | `ls -l` 3034 bytes |
| `internal/test/mock/dns/server.go` | Yes | `ls -l` 7846 bytes |
| `internal/test/mock/dns/zone.go` | Yes | `ls -l` 7254 bytes |
| `internal/test/mock/dns/doc.go` | Yes | `ls -l` 92 bytes |
| `internal/test/mock/dns/dns_test.go` | Yes | `ls -l` 4125 bytes |
| `internal/test/mock/dns/server_test.go` | Yes | `ls -l` 7108 bytes |
| `internal/test/mock/dns/zone_test.go` | Yes | `ls -l` 3655 bytes |
| `internal/test/fixture/plugin_fixture_dns_stub.go` | Yes | `ls -l` 7582 bytes |
| `internal/test/runner/caps_option_test.go` | Yes | `ls -l` 9196 bytes |
| `test/plugin/dns-stub-lookup.ci` | Yes | `ls -l` 3665 bytes |
| `test/plugin/dns-stub-answer-change.ci` | Yes | `ls -l` 2140 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The stub binds, reports the bound port, and a failed bind is an error | `go test -count=1 -v ./internal/test/mock/dns/...`: `--- PASS: TestStartBindsChosenAndEphemeralPort`, `--- PASS: TestRunReportsListenFailure` |
| AC-2 | Records and TTL are what the zone declares | `--- PASS: TestAnswerCarriesTheDeclaredTTL`, `--- PASS: TestZoneAnswersEveryRcode` |
| AC-3 | NODATA, never NXDOMAIN, for a type a held name does not carry | `--- PASS: TestNameWithoutThatTypeAnswersNoData`; producer read at `handle` (`internal/test/mock/dns/server.go`): the record loop is guarded by `rcode == mdns.RcodeSuccess && carries`, and `reply.Rcode` is already the name's own code |
| AC-4 | An absent name answers NXDOMAIN | `--- PASS: TestZoneAnswersEveryRcode`; `answerFor` returns `mdns.RcodeNameError` when the key is not held |
| AC-5 | SERVFAIL and REFUSED names answer their code with no records | `--- PASS: TestZoneAnswersEveryRcode`; `Status.Authoritative` (`internal/component/resolve/dns/resolver.go`) is true only for `StatusSuccess` and `StatusNameError`, so both take the non-authoritative branch of `resolveAndRecord` |
| AC-6 | `Set` is in effect for the next query and touches nothing else | `--- PASS: TestSetChangesTheNextAnswer`, `--- PASS: TestSetAddsANameTheZoneDoesNotCarry` |
| AC-7 | A zero TTL keeps the answer out of the resolver cache | `cache.put` (`internal/component/resolve/dns/cache.go`) returns before storing when the TTL is zero; `test/plugin/dns-stub-answer-change.ci` asserts `203.0.113.11` after `Set` with no cache command |
| AC-8 | `show dns lookup` renders the stub's answers for both families and the NXDOMAIN status | `test/plugin/dns-stub-lookup.ci` five `expect=` lines; `handleDNSLookup` (`internal/component/resolve/cmd/show_dns.go`) writes `status` only when the status is neither Unspecified nor Success, which is what makes the ABSENT field prove NODATA |
| AC-9 | The `net-bind` gate skips without the capability and runs with it | `go test -count=1 -run 'TestCapsNetBind\|TestNeedsLinux\|TestBPFToken' ./internal/test/runner/`: 7 PASS, `ok 0.023s` |
| AC-10 | `--help` lists every zone name with type, addresses, TTL and rcode | `--- PASS: TestHelpPrintsTheZone`, `--- PASS: TestUsageListsEveryZoneName` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `.ci` line `cmd=background:exec=ze-test dns --port 53` -> `dns.Run` | `test/plugin/dns-stub-lookup.ci` | Yes. Read the file: line `cmd=background:seq=1:exec=ze-test dns --port 53`, and `registerRoot("dns", dnsmock.Run, ...)` in `internal/test/cli/register.go` |
| A compiled fixture starting the server in-process -> `dns.Start`, `Server.Set` | `test/plugin/dns-stub-answer-change.ci` | Yes. `dnsStubAnswerChange` calls `dnsmock.Start(dnsStubAddress)` then `server.Set("web.example.test", mdns.TypeA, ...)`; registered as `plugin/dns-stub-answer-change` in `internal/test/fixture/plugin_fixture_06.go` and named by the `.ci` `plugin { external ... }` block |
| `show dns lookup <name> type A` against a daemon whose `system name-server` names the stub -> `Resolver.ResolveWithTTL` | `test/plugin/dns-stub-lookup.ci` | Yes. The `.ci` sets `name-server 127.0.0.1`; `newResolvers` (`cmd/ze/hub/main_system.go`) reads `NameServers[0]` into `cfg.Server`, and `NewResolver` joins port 53 onto a value with none |
| `option=needs-linux:caps=net-bind` -> `capsRequired` | both `.ci` files, `TestCapsNetBindGateBothPolarities` | Yes. Both files carry the option; an unknown token is rejected at parse time on every host (`TestNeedsLinuxRejectsUnknownCaps`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The credential drop in `runOrchestrated` (`internal/test/runner/runner_exec.go`) is guarded by `binName == binNameZe`, and the `plugin` suite runs `Namespace: guestRoot` (`vmSuites`, `internal/le/qemu/alltests.go`), where nothing is dropped at all. The narrower reading is now written onto `dnsStubAddress`: under the per-test netns mode a fixture WOULD lose the capability, because `prepareNetnsBinaries` (`internal/le/qemu/netns_linux.go`) setcaps only `ze` and `ze-stripped` |
| A-2 | confirmed | `cache.put` (`internal/component/resolve/dns/cache.go`) returns before storing a zero TTL, and `Resolver.query` returns TTL 0 for every non-success rcode, so no negative answer is cached either |
| A-3 | confirmed | `NewResolver` builds its client with `Net: "udp"` (`internal/component/resolve/dns/resolver.go`) and `Resolver.query` logs `truncated DNS response` rather than retrying |
| A-4 | confirmed | The leaf is `type zt:ip-address`; both `.ci` files bind 53 and gate on `caps=net-bind`, and `WriteResolvConf` (`internal/component/config/system/resolv_linux.go`) writes nothing for the empty path the files set, so no host file is touched |
| A-5 | confirmed | `CAP_NET_BIND_SERVICE` is 10 in the kernel's `include/linux/capability.h`; `capNetBindService` carries that number in `internal/test/runner/caps.go`; `TestCapsNetBindGateBothPolarities` asserts the mapping and refuses `capNetAdmin` in the same set |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 10, test infrastructure: `docs/functional-tests.md` needs a `### ze-test dns` section and the `net-bind` token | The text is written; the zone table matches `defaultZone()`, the NODATA paragraph matches `handle`, and the three anchors name `dns.go`, `zone.go` and `server.go` | Written, NOT COMMITTED. See Work Not Done |
| Row 16, changed files under existing doc anchors | `docs/functional-tests.md` anchors `internal/test/runner/record_parse.go` and `internal/test/cli/cmd_exabgp.go`; the `caps=` table adds an anchor at `internal/test/runner/caps.go -- capsRequired`. `git grep -n "source: internal/test/mock" docs/` returns nothing before this change | Yes |
| Rows 1-9 and 11-15, No | `ze-test` is a host developer binary: `git grep -n "ze-test dns" docs/` outside `docs/functional-tests.md` returns nothing, and no daemon surface, RPC, YANG leaf, plugin, wire format or RFC support row changed | Yes |
| Doctor check for a runtime dependency | The listen port belongs to a test process. `git grep -n "ze-test" internal/component/doctor/` returns nothing, and no daemon dependency was added | Yes |
| `ai/PACKAGE-MAP.md` freshness | `./le discovery-index check`: `checked 763 packages, ai/PACKAGE-MAP.md up to date` | Yes |

## Core Insight

A stub's job is to be believed, so the shape of its zone is a correctness
question rather than a convenience one. Putting the response code on the name
and the addresses on the type is not tidier: it is the only shape in which
RFC 2308 Section 2.2 cannot be violated by a zone entry, because a name that
exists has exactly one code, and a type it does not hold has no entry to
contradict it. The alternative shape would have let a zone declare
`v4only.example.test` NOERROR for A and NXDOMAIN for AAAA, and the test reading
that would have concluded the firewall was right to empty its set.

The same reasoning picks the default TTL. Zero is not "unset": it is what stops
the daemon answering a second lookup from its own cache, and a stub that quietly
substituted a comfortable TTL would make every answer-change test pass on the
address it was meant to prove had moved.

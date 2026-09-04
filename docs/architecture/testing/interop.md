---
title: Interoperability Testing (Testing Architecture)
---
# Interoperability Testing

Ze validates protocol correctness against production BGP daemons in two complementary ways:
live session interop tests (Docker containers running real daemons) and byte-level wire format
validation against ExaBGP (Ze's predecessor, a BGP implementation in Python).

For BGP terminology used in this document, see [docs/features.md](../../features.md).

`ai/rules/interop-and-goal-validation.md` states when an interop test is owed.
This page is the infrastructure it is owed against.

## The suites, one per protocol area

| Protocol area | Peer implementation | Scenario directory | Native action |
|---------------|---------------------|--------------------|---------------|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP, StayRTR | `test/interop/scenarios/` | `./le integration interop` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/interop-ipsec/` | `./le integration interop-ipsec` |
| L2TP | Docker | `test/interop-l2tp/` | `./le deployment l2tp-test`, and `./le deployment l2tp-ppp-test` for the full PPP and NCP path |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/interop-pppoe/` | `./le deployment docker-pppoe-accel-test` |
| RADIUS (admin login: PAP, CHAP, EAP, Filter-Id) | Docker: FreeRADIUS | `test/interop-radius/scenarios/` | `./le integration interop-radius` |

<!-- source: internal/le/integration/gates.go -- interop, interop-ipsec and interop-radius verbs -->
<!-- source: internal/le/deployment/actions.go -- l2tp-test, l2tp-ppp-test, docker-pppoe-accel-test verbs -->

Every suite discovers its scenarios the same way. `Discover`
(`internal/le/interoplab/discover.go`) reads the scenario directory, keeps the
subdirectories, sorts the names lexically, and joins each one against the
owning checker registry. A scenario with no checker, or a nil checker, is an
ERROR rather than a skipped test, so a fixture and its registry cannot silently
disagree. Nothing depends on the order: each scenario gets its own setup, check
and teardown.

A scenario directory carries only declarative inputs its runner reads: `ze.conf`
plus the peer configuration and argument files that topology needs. Assertions
never live there. They are typed Go checkers under `internal/le/interoplab/`:
BGP builds its registry from `scenarioOperations` in
`internal/le/interoplab/bgp/checkers.go`, and IPsec declares `scenarioCheckers`
in `internal/le/interoplab/ipsec/checkers.go`. A checker waits for readiness,
asserts the protocol behaviour, verifies stability where the scenario needs it,
and returns an error on failure.

A scenario directory is NAMED and carries no numeric prefix. The name is the
scenario's identity: `Discover` matches it exactly, `./le integration` takes it
as a scenario selector, and specs, journal rows and code comments cite it.

## Tested Daemons

| Daemon | Version | Image | Query Method | What It Validates |
|--------|---------|-------|--------------|-------------------|
| FRR | 10.3.1 | `quay.io/frrouting/frr:10.3.1` | vtysh | eBGP, iBGP, route exchange, GR, communities, MD5, route server |
| BIRD | 2.x (Alpine 3.21) | Alpine build | birdc | eBGP, route exchange, triangle topologies |
| GoBGP | 3.31.0 | Go builder | gobgp CLI | eBGP, route injection and verification |
| StayRTR | 0.6.4 | Go builder | HTTP `/rpki.json` export | RTR (RFC 8210) as the CACHE, so Ze is the client of an implementation that is not its own. Origin validation answers (RFC 6811) against VRPs a third party encoded |
| ExaBGP | API 6.0.0 contract fixtures | Compiled Go wire server | Wire byte comparison | Byte-for-byte encoding across all address families |
<!-- source: internal/le/interoplab/bgp/prepare.go -- peer helpers and scenario preparation -->
<!-- source: internal/le/interoplab/bgp/run.go -- scenario orchestrator -->

## Prerequisites

| Requirement | Used By | Notes |
|-------------|---------|-------|
| Docker | Interop tests | Containers for FRR, BIRD, GoBGP, Ze |
| ~1.5 GB disk | Interop tests | Docker images (Go builder, FRR, Alpine) |

The interop test network uses `172.30.0.0/24`. MD5 authentication scenarios require
`NET_ADMIN` capability (granted automatically by the orchestrator).

## Live Interop Tests (`test/interop/`)

Each scenario runs Ze and one or more peer daemons in Docker containers on a shared
network (`172.30.0.0/24`), establishes real BGP sessions, and asserts correct behavior
via each daemon's native CLI.

### How It Works

The native `internal/le/interoplab/bgp` package discovers scenario directories in
`test/interop/scenarios/`. For each scenario, the shared `interoplab.Suite`
engine:

1. Creates an isolated Docker network.
2. Starts Ze and the peer daemons declared by the scenario's config files.
3. Waits for every declared readiness probe.
4. Runs the scenario's typed checker from the package-local BGP registry.
5. Tears down every container and network, including after setup or checker failure.
<!-- source: internal/le/interoplab/lab.go -- suite lifecycle -->

Daemons start conditionally: FRR if `frr.conf` exists, BIRD if `bird.conf` exists,
GoBGP if `gobgp.toml` exists. This means each scenario only runs the daemons it needs.

**A scenario directory is NAMED, never numbered** (owner directive, 2026-08-24).
Run order is lexical by scenario directory name and no scenario depends on it.
The rule and its reasoning live in `ai/rules/interop-and-goal-validation.md`,
which is always-on, so a spec author planning a scenario meets it without
opening this page.

### Container Addresses

| Daemon | IP | Container |
|--------|----|-----------|
| Ze | 172.30.0.2 | `ze-iop-ze-<pid>` |
| FRR | 172.30.0.3 | `ze-iop-frr-<pid>` |
| BIRD | 172.30.0.4 | `ze-iop-bird-<pid>` |
| GoBGP | 172.30.0.5 | `ze-iop-gobgp-<pid>` |
| Raw injector | 172.30.0.9 | `ze-iop-inject-<pid>` |
| Compiled strict speaker | 172.30.0.10 | `ze-iop-speaker-<pid>` |
| Compiled strict speaker (2nd) | 172.30.0.11 | `ze-iop-speaker2-<pid>` |
| StayRTR | 172.30.0.12 | `ze-iop-stayrtr-<pid>` |

Container names include the runner PID as suffix, so concurrent runs do not conflict.
<!-- source: internal/le/interoplab/bgp/prepare.go -- container naming, IP addresses -->

### Scenario Structure

Each scenario is a directory under `test/interop/scenarios/`:

```
scenarios/bgp-ebgp-ipv4-frr/
  ze.conf        # Ze configuration (required)
  frr.conf       # FRR configuration (starts FRR container)
```

Every directory has one typed checker in `internal/le/interoplab/bgp`. The
catalogue uses explicit operations for ordinary session, route, adjacency, log,
and negative assertions, plus bespoke checkers for scenarios whose control flow
cannot be represented as an ordered operation list.

A bespoke checker is written in two halves. The body in `check_rfc.go` does the
lab I/O and numbers each assertion, so a failure names the assertion that found
it. The pure predicate in `check_rfc_predicate.go` takes the text or the decoded
JSON a peer daemon produced, holds no lab handle, and decides. The split is what
lets `TestBespokeCheckerBranches` drive both polarities of every predicate in
seconds with no container.
<!-- source: internal/le/interoplab/bgp/check_rfc_predicate.go -- pure predicates -->
<!-- source: internal/le/interoplab/bgp/check_rfc.go -- tagged checker bodies -->

A body MUST NOT call `checkScenario` (`check_engine.go`). That function answers
`scenario %s has no typed assertions` for every name absent from
`scenarioOperations`, and a bespoke name is absent from that table by design, so
the call is an error that always fires and every line under it is unreachable.

### Optional sidecars

A scenario directory may also carry files that start extra containers before Ze:

| File | Sidecar | Purpose |
|------|---------|---------|
| `inject.msg` | `ze-test peer` (raw injector, 172.30.0.9) | Drive Ze with wire bytes no conforming daemon would emit. An optional `inject-args` file adds flags. Because the injector and Ze start before the peer daemons, an early route exercises Ze's replay-on-peer-up path. |
| `speaker-args` (and optional `speaker2-args`) | `ze-test interop-bgp speaker` (172.30.0.10; second at 172.30.0.11) | Dial Ze with an independent strict peer. The compiled speaker negotiates the requested families and ADD-PATH mode, frames BGP itself, applies the named native oracle, and writes a structured verdict to container logs. It catches wire output that Ze's own lenient decoder could accept. |
| `vrps.json` | StayRTR (172.30.0.12:8282) | Serve RPKI VRPs from a real third-party cache, so Ze is the RTR client of an implementation that is not its own. The typed checker asserts each per-prefix validation answer, not merely the RTR session. |

BMP scenarios start `ze-test interop-bgp bmp-collector`. Announcement and
observer process plugins use `ze-test interop-bgp process <scenario> <plugin>`.
These personalities are compiled into `ze-test`; no interpreter or source mount
is present in the Ze image.
<!-- source: internal/le/interoplab/bgp/prepare.go -- sidecar startup -->
<!-- source: internal/le/interoplab/bgp/helper.go -- compiled process and BMP helpers -->
<!-- source: internal/le/interoplab/bgp/speaker.go -- compiled strict speaker -->
<!-- source: test/interop/scenarios/bgp-rfc7606-relay-shape-frr/ -- injector worked example -->
<!-- source: test/interop/scenarios/bgp-rfc7606-speaker-dup-attr/ -- speaker worked example -->
<!-- source: test/interop/scenarios/rtr-stayrtr/ -- StayRTR worked example -->

### Prove a scenario discriminates

An interop scenario is evidence only if it goes RED when the behaviour it tests is
broken. Before you rely on a new scenario, revert the fix, run the scenario, and
confirm that it fails. Then restore the fix and confirm that it passes.

**Let the harness build. Do NOT `docker build -t ze-interop` by hand and then run
with `NO_BUILD=1`.** A tag is shared by every run on the host. A build in another
session rebinds that tag between yours and your container start. Your mutation run
then measures a daemon you did not build, and that inverted a proof twice in one
review on 2026-08-05. `Docker.Build` reads the image ID from `docker build -q`,
and the suite pins every container of the run to that immutable ID.
Quote that line beside the result, because it names the binary the run measured.

A scenario that passes either way (common when the peer must accept both the old
and the new wire form) proves acceptance, not correctness. Say which one it
proves in the spec's Goal Validation, and move the discrimination to a unit or
mutation test that CAN fail.

A scenario added to ALREADY-WORKING code never had a red phase, so its
discrimination is unproven until you force one. That is not TDD's red-then-green:
a regression test and a scenario for existing behaviour both start green.

Five traps make a scenario pass whatever the code does. Check each by its tell
before you call the scenario evidence:

| Vacuity trap | Why it passes anyway | The tell |
|--------------|----------------------|----------|
| A scenario for a sender-side wire change whose receiver is obliged to accept any form (RFC 7606 Section 5.1: receivers accept any field combination) | A conforming peer accepts the old and the new wire equally | Reverting the sender change leaves the peer's routing table identical |
| A test asserting the ABSENCE of something (no log line, no allocation, no route) | Deleting the mechanism leaves the same absence | Ask what would still be absent if the code were removed |
| A test whose fixture is at an extreme (all fields set, maximum value) | An off-by-one or a partial break still handles the extreme | Boundary the fixture: test one below and one above |
| A test whose data reaches the peer by a DIFFERENT path than the one changed | The unchanged path still delivers | Trace which code path actually produces the asserted bytes |
| An assertion whose clauses are all satisfied by ONE stimulus | Each clause reads as an independent observation, and they are one observation written twice | Name the single event that satisfies every clause. Then ask which clause a peer that did nothing would still satisfy |

The fifth trap is what the IPsec suite carried until 2026-09-04, and it is worth
reading in full because the shape recurs. `verifyTunnelTraffic`
(`internal/le/interoplab/ipsec/helpers.go`) pinged from Ze to strongSwan and
passed when each peer's ESP byte counters advanced. Both clauses were satisfied by
that ONE ping: Ze encrypting the echo request advances Ze's outbound SA, and
strongSwan decrypting the same request advances strongSwan's inbound SA. Nothing
required strongSwan to have encrypted anything toward Ze.

RFC 4301 Section 4.1 says why the aggregate could not discriminate:

> An SA is a simplex "connection" that affords security services to the traffic
> carried by it.

A protected bidirectional flow is two SAs, and the RECEIVER chooses the SPI, so
both peers name one direction by the same SPI value. Measured in
`psk-site-to-site`: Ze holds `src 172.28.0.2 dst 172.28.0.3 spi 0xc12fa7e3` and
`src 172.28.0.3 dst 172.28.0.2 spi 0xf008af63`, and strongSwan holds those same
two SPIs. A counter map keyed by SPI alone therefore folds the two peers' views of
one direction into one entry. `verifyESPDirections` now reads each simplex SA by
its own `src`/`dst` header and takes the set of directions its caller can claim,
and it also refuses a ping whose `% packet loss` summary is missing or non-zero.
<!-- source: internal/le/interoplab/ipsec/helpers.go -- directed ESP counters and the lossless-ping clause -->

### The strongSwan lab drop-in

Every scenario that starts a strongSwan peer mounts
`test/interop-ipsec/strongswan-lab.conf` read-only at
`/etc/strongswan.d/98-lab.conf`. It carries the charon settings the whole lab
needs, and today that is one: `charon.plugins.bypass-lan.load = no`.

charon loads `bypass-lan` by default, and that plugin installs a PASS shunt for
every locally attached subnet. This lab puts both containers on 172.28.0.0/24, so
the shunt covers the peer. Measured on 2026-08-30 in `psk-site-to-site`, the shunt
sits at priority 175423 against the Child SA policy's 399999, the lower number
wins, and every packet strongSwan sends to Ze leaves in the clear. A ping still
succeeds under that shunt, which is exactly why a lossless ping is necessary and
not sufficient evidence that a tunnel carried anything.

A scenario's own `strongswan.conf` still mounts at
`/etc/strongswan.d/99-interop.conf` and composes with the lab file. A scenario
MUST NOT set `bypass-lan` itself: `TestNoScenarioCarriesItsOwnBypassLanOverride`
(`test/interop-ipsec/parity_test.go`) refuses a second copy, because two files
setting one value is a disagreement with nothing to arbitrate it.
<!-- source: internal/le/interoplab/ipsec/ipsec.go -- prepareScenario mounts the lab drop-in -->
<!-- source: test/interop-ipsec/strongswan-lab.conf -- the lab-wide charon settings -->

### The FreeRADIUS admin-login suite

`internal/le/interoplab/radius/` runs ze's operator login against a real
FreeRADIUS server at a pinned tag, pulled through `ImageBuild{Pull: true}`. It
exists because every other RADIUS proof ze holds runs against a mock ze wrote:
`test/plugin/aaa-radius-admin.ci` drives `internal/test/mock/radius/radius.go`,
and the L2TP lab's peer is `internal/le/interoplab/l2tp/radiusmock/`, which is
ze's own Go program in a container. A mock built beside ze's encoder agrees with
ze by construction, and ze now computes a CHAP digest a server must reproduce
from its own stored password. Only a server ze did not write can disagree.

It is its own suite rather than four more L2TP scenarios. The L2TP lab probes
for the `l2tp_ppp` or `pppol2tp` kernel module and refuses to run without it,
which is correct for a suite that carries PPP sessions. Admin login is ze's SSH
listener, a UDP socket and a RADIUS server, so this lab declares no preflight
beyond its own ze cross-compile, mounts no module tree, asks for no capability
and runs nothing privileged. `TestSuiteNeedsNoKernelModule` holds that.

Every checker reads BOTH sides. Ze's log saying `source=radius` is not enough on
its own, because a login the local bcrypt backend satisfied produces a line of
the same shape and no server traffic at all. The lab therefore mounts a
`linelog` module at `/etc/raddb/mods-enabled/ze_request_log` that writes one
line per answered request to `/var/log/freeradius/ze-request.log`, carrying the
verdict, the User-Name, the PRESENCE of a User-Password, of a CHAP-Password and
of an EAP-Message, and the NAS-Identifier. Presence and not value: a fixture
must not put a password or a digest in a log file. `parseServerRecord` refuses a
line missing any of the six fields, so a truncated or reformatted line is never
read as a partial verdict.

An EAP login is several requests and FreeRADIUS runs no `post-auth` section for
an Access-Challenge, so the reply that asked the question records nothing. A
second module, `ze_state_echo_log`, is called from `authorize` on a request
carrying both an EAP-Message and a State, and writes `verdict=state-echo`. That
is the server's own evidence that ze returned the State unmodified, which
RFC 2865 Section 5.24 requires, and that the login was a conversation rather
than one request.

| Scenario | Ze's side | The server's side |
|----------|-----------|-------------------|
| `radius-admin-pap-freeradius` | An operator logs in over ze's real SSH listener, ze's log says `source=radius`, the Filter-Id profile denies `show bgp`, and the local account's own password is refused | `verdict=accept` with a User-Password present, a CHAP-Password absent and ze's NAS-Identifier, then `verdict=reject` for the wrong password |
| `radius-admin-chap-freeradius` | The same, with `auth-method chap` against a `Cleartext-Password` entry | `verdict=accept` with a CHAP-Password present and NO User-Password beside it, which is what RFC 2865 Section 4.1 demands of an Access-Request |
| `radius-admin-chap-hashed-freeradius` | The CHAP login is REFUSED, and ze authenticates the user through no backend at all | A `radclient` probe first proves the same entry accepts the same password over PAP, then `verdict=reject` for the CHAP request |
| `radius-admin-eap-freeradius` | The same, with `auth-method eap-mschapv2`. Ze answers the EAP conversation itself from the operator's password, Naks the server's MD5-Challenge toward MSCHAPv2, and returns the State on every later round | `verdict=state-echo` for at least one round carrying the State the server issued, then `verdict=accept`, both with an EAP-Message present and NEITHER password attribute beside it |

The third scenario is the one that proves `docs/guide/radius.md` is telling the
truth. RFC 2865 Section 2.2:

> For example, CHAP requires that the user's password be available in cleartext
> to the server so that it can encrypt the CHAP challenge and compare that to
> the CHAP response.  If the password is not available in cleartext to the
> RADIUS server then the server MUST send an Access-Reject to the client.

A rejection on its own would also follow from a typo in the user file, which is
the fourth vacuity trap wearing another face: the asserted result would be
produced by a different cause than the one under test. The `radclient` PAP probe
removes it, because the storage form is then the only thing left to explain the
CHAP rejection.

Two credentials share one username on purpose. `radiusop` exists in the server's
user file AND in ze's local account list with a DIFFERENT password, so the
scenario can send the local password while RADIUS rejects it: a chain that fell
through to local bcrypt would accept that login. `localop` exists only locally
and the server answers nothing for it, which is the positive control that proves
the SSH listener and the local backend are both live before any refusal is read
as evidence.
<!-- source: internal/le/interoplab/radius/radius.go -- the suite, its pinned image, its peers and its probes -->
<!-- source: internal/le/interoplab/radius/checkers.go -- every observation, on ze's side and on the server's -->
<!-- source: test/interop-radius/mods-ze-request-log -- the linelog module the server's record comes from -->

### Typed checker operations

`checkers.go` is the complete scenario catalogue. Each operation identifies the
peer, exact command, required or forbidden evidence, proof for negative
assertions, and a bound. `check_engine.go` executes those operations through the
shared `CheckerLab` interface. FRR, BIRD, GoBGP, Ze, speaker logs, and kernel
state are all queried through explicit typed branches.

An absent value never proves a negative assertion by itself. The operation must
also name positive evidence that the query mechanism ran. Failed and empty
queries remain errors rather than becoming plausible empty protocol state.

That rule decides which command a negative assertion sends. `opBIRDRouteAbsent`
reads the whole table with `birdc show route`, and never `show route for
<prefix>`: BIRD answers a lookup for a network it does not hold with "Network
not found", and birdc exits 1. A lookup therefore fails in the exact state the
assertion exists to observe, and the failure is not absence. Ask a question the
peer answers in both states, then read the absence out of the answer.

Scenarios with non-linear behavior register a bespoke checker in
`specialCheckers` (`check_special.go`). Each one owes a named subtest in
`TestBespokeCheckerBranches` that drives its predicate in BOTH polarities: the
true case, and a false case written against one stated wrong reading, such as
two tokens matched across two log lines, a peer-originated event passing as a
received one, or an absence with no proof that the query ran.
<!-- source: internal/le/interoplab/bgp/bgp_test.go -- TestBespokeCheckerBranches -->

### Querying Ze

`Ze.cli(command)` is the only way to ask the Ze daemon anything. It runs
`ze cli -c <command> --user ... --format json` inside the container.

Two properties of that line are load-bearing and neither is obvious.

`ze cli -c` rather than the verb form `ze show bgp rib status`: `--user` and
`--format` are flags of `ze cli`, and the verb form has no slot for either.

The daemon starts an SSH listener only when its config asks for one
(`infraSetup`, `cmd/ze/hub/infra_setup.go`), and `ze cli` reaches the daemon over
SSH. No scenario `ze.conf` asks. The harness appends `ZE_CLI_CONFIG` -- the
listener plus the account it authenticates against -- to the RENDERED copy of
every `ze.conf` (`renderScenario`), so no scenario carries the boilerplate and
none can forget it. The native IPsec plan appends the same blocks in
`renderZeConfig` (`internal/le/interoplab/ipsec/ipsec.go`).

**A Ze helper never converts a failed query into a plausible number.**
`Ze.rib_count` raises when the command fails or answers without a `routes-in`
field, because 0 is a legitimate RIB size and a failed query is not
(`ai/rules/evidence.md`). It returned 0 on failure until 2026-08-07 and three
separate faults hid behind that one number for three days
(`spec-fixit-test-harness-fail-open-guards`, guard 3). Write new Ze
helpers the same way.

All session waiters use explicit bounds (default 90 seconds, override via
`SESSION_TIMEOUT`). The harness passes that value into the Ze container so a
compiled process helper can size its barriers against the same budget.
<!-- source: internal/le/interoplab/wait.go -- bounded wait -->
<!-- source: internal/le/interoplab/bgp/prepare.go -- container environment -->

### A compiled process helper that fails

Scenario process personalities use the Go plugin SDK. A helper failure writes
the `ZE-OBSERVER-FAIL` sentinel to stderr and requests daemon shutdown. The
checker failure path reads the last 2,000 Ze log lines and appends that measured
cause when the sentinel is present.

An unreadable log is not a plugin verdict. `checkerFailure` retains the original
scenario assertion when the log read fails, so a Docker diagnostic cannot
replace the protocol failure that triggered it.

<!-- source: internal/le/interoplab/bgp/helper.go -- runtimeFailure -->
<!-- source: internal/le/interoplab/bgp/check_engine.go -- checkerFailure -->
<!-- source: internal/le/interoplab/bgp/bgp_test.go -- TestCheckerFailureKeepsPrimaryCauseWhenLogsFail -->

### Scenario Inventory

The suite has grown to over 100 scenario directories in `test/interop/scenarios/`. The table
below lists the core BGP scenarios (01-37); beyond these, the suite also covers route
reflection, policy import/export, RPKI origin validation, BMP monitoring, PATHS-LIMIT,
max-prefix cease, GTSM, AS112, the RFC 7454 Section 9 transit leak
(`bgp-path-asn-leak-frr` gives FRR two prefixes that differ only in their AS_PATH, and requires
ze to drop the one reached through a listed transit ASN, keep the other, and keep the session),
ADD-PATH re-advertisement (`bgp-addpath-readvertise-collision-frr`
proves a receiver keeps two paths whose sources both chose one Path Identifier, and
`bgp-addpath-rail-agreement-speaker` proves the live forward and the peer-up replay emit the same
bytes for one path), and full IS-IS (auth, convergence, dual-stack, LAN DIS,
P2P, redistribution) and OSPFv2/OSPFv3 (auth, BFD, TE, LFA/TI-LFA, graceful restart,
segment routing, opaque LSAs, stub/NSSA, virtual links, and more) interop families.

| # | Scenario | Daemons | What It Tests |
|---|----------|---------|---------------|
| 01 | ebgp-ipv4-frr | Ze, FRR | Basic eBGP session establishment |
| 02 | ebgp-ipv4-bird | Ze, BIRD | Basic eBGP session with BIRD |
| 03 | ibgp-frr | Ze, FRR | iBGP session (same AS) |
| 04 | 4byte-asn-frr | Ze, FRR | 4-byte ASN negotiation (RFC 6793) |
| 05 | routes-from-frr | Ze, FRR | Ze receives routes originated by FRR |
| 06 | routes-from-bird | Ze, BIRD | Ze receives routes originated by BIRD |
| 07 | routes-to-frr | Ze, FRR | FRR receives routes originated by Ze |
| 08 | triangle | Ze, FRR, BIRD | Three-way topology, multi-peer stability |
| 09 | route-withdrawal-frr | Ze, FRR | Route withdrawal propagation |
| 10 | ipv6-ebgp-frr | Ze, FRR | IPv6 eBGP session and route exchange |
| 11 | addpath-frr | Ze, FRR | ADD-PATH capability (RFC 7911) |
| 12 | route-refresh-frr | Ze, FRR | Route Refresh (RFC 2918) |
| 13 | graceful-restart-frr | Ze, FRR | Graceful Restart negotiation (RFC 4724) |
| 14 | route-server-frr | Ze, FRR, BIRD | Route server: forwards without inserting own ASN |
| 15 | community-frr | Ze, FRR | Standard community propagation |
| 16 | extended-community-frr | Ze, FRR | Extended community propagation |
| 17 | md5-auth-frr | Ze, FRR | TCP MD5 authentication (RFC 2385) |
| 18 | ebgp-gobgp | Ze, GoBGP | eBGP session with GoBGP |
| 19 | routes-gobgp | Ze, GoBGP | Route exchange with GoBGP |
| 20 | role-frr | Ze, FRR | RFC 9234 Role capability negotiation |
| 21 | role-gobgp | Ze, GoBGP | RFC 9234 Role capability negotiation |
| 22 | evpn-frr | Ze, FRR | EVPN Type-2 route exchange |
| 23 | vpn-frr | Ze, FRR | VPN (L3VPN) route exchange |
| 24 | flowspec-frr | Ze, FRR | FlowSpec rule exchange |
| 25 | ipv6-ebgp-bird | Ze, BIRD | IPv6 eBGP route exchange |
| 26 | ipv6-ebgp-gobgp | Ze, GoBGP | IPv6 eBGP route exchange |
| 27 | multihop-ebgp-frr | Ze, FRR | Multi-hop eBGP with outgoing-ttl |
| 28 | evpn-gobgp | Ze, GoBGP | EVPN Type-2 route exchange |
| 29 | vpn-gobgp | Ze, GoBGP | VPN (L3VPN) route exchange |
| 30 | flowspec-gobgp | Ze, GoBGP | FlowSpec rule exchange |
| 31 | multihop-ebgp-bird | Ze, BIRD | Multi-hop eBGP with outgoing-ttl |
| 32 | multihop-ebgp-gobgp | Ze, GoBGP | Multi-hop eBGP with outgoing-ttl |
| 33 | bfd-frr | Ze, FRR | BFD opt-in and BFD-triggered BGP teardown |
| 34 | ecmp-frr | Ze, FRR, GoBGP | FRR ECMP selection for the same prefix from Ze and GoBGP |
| 35 | srv6-frr | Ze, FRR | SRv6 VPNv6 route exchange and Prefix-SID handling |
| 36 | remove-private-as-frr | Ze, FRR, GoBGP | remove-private-as export policy to FRR |
| 37 | remove-private-as-as4path-frr | Ze, FRR, BIRD | remove-private-as handling for AS4_PATH private ASNs |
<!-- source: test/interop/scenarios/ -- scenario directories -->

### Running

```bash
./le integration interop
INTEROP_SCENARIO=bgp-ebgp-ipv4-frr ./le integration interop
VERBOSE=1 ./le integration interop
NO_BUILD=1 ./le integration interop
FRR_IMAGE=quay.io/frrouting/frr:10.3 ./le integration interop
BUILD_TIMEOUT=7200 ./le integration interop
```

Interop tests require Docker and are not part of the offline precommit gate.
They are a separate protocol-validation action.

The first run builds the Docker images. How long that takes is a property of the
build host, and the spread is wide. `Dockerfile.ze` copies the whole tree and
compiles ze twice with no cache mount. One colima VM of 2 CPUs and 2 GB built it
in 2m48s on 2026-09-04, and the same VM took 40m39s for it earlier that day, when
the host disk was full and the guest was thrashing.

Each build is bounded at 90 minutes, and `BUILD_TIMEOUT` sets that bound in whole
seconds for a machine slower or faster than that one. The bound stops a wedged
Docker daemon and is not a budget for the build, so a build that finishes returns
at once and a generous bound costs nothing. A value that does not parse, or that
is not positive, keeps the 90 minutes.

An image that needs more than the machine bound declares its own
`ImageBuild.Timeout`, and that field only ever LENGTHENS a bound. A number below
the machine bound shortens it, which kills a build the machine would finish, so
no suite declares one. The PPPoE suite did until 2026-09-04: 10 minutes for its
ze image, 15 for accel-ppp and 10 for the client, each written when the shipped
default was 10 minutes and each a cap once the default became 90.

Subsequent runs with `NO_BUILD=1` skip rebuilds. Once the images exist, the full
suite takes roughly 5-10 minutes depending on session establishment times.
<!-- source: internal/le/interoplab/docker.go -- dockerBuildTimeoutDefault, buildTimeout -->


### Debugging Failures

On failure, the orchestrator automatically dumps the last 20 lines of container logs.
For more detail:

- `VERBOSE=1` enables debug output (polling status, container commands, raw CLI output)
- `SESSION_TIMEOUT=120` increases the session establishment timeout (default 90s)
- Single-scenario runs isolate the problem: `INTEROP_SCENARIO=bgp-graceful-restart-frr ./le integration interop`

### Writing a New Scenario

1. Create a descriptively named directory under `test/interop/scenarios/`.
2. Add `ze.conf` and whichever peer configs the scenario needs.
3. Add the scenario's ordered assertions to `scenarioOperations` and
   `scenarioExtras`, or register a bespoke checker in `specialCheckers` when the
   control flow is non-linear.
4. For a bespoke checker, put each decision in a pure predicate in
   `check_rfc_predicate.go` and add its both-polarity subtest to
   `TestBespokeCheckerBranches`.
5. Run `INTEROP_SCENARIO=<name> ./le integration interop`.

`TestCheckerPopulationMatchesProducer` compares every scenario directory with
the package-local registry. `TestEveryCheckerFailsClosedWithoutPeerEvidence`
rejects a checker that can pass without reading a peer. Each negative assertion
must carry positive proof that its query mechanism ran.
<!-- source: internal/le/interoplab/bgp/checkers.go -- checkers -->

#### A scenario that reads the RIB attaches the RIB plugin

`plugin { internal rib { use bgp-rib; } }` loads the plugin. It does NOT feed it.
A peer delivers an event to a process only where both halves agree, and the
peer's half is its attach block (`Server.PeerScopedProcs`,
`internal/component/plugin/server/delivery_graph.go`). A peer with no
`attach process rib` block grants nothing, so the plugin sees no peer at all and
every RIB question answers empty:

```
	attach process rib {
		receive [ update state refresh ];
	}
```

The tell is `"peers": 0` from `show bgp rib status` while `show bgp peer list`
reports both sessions Established. Ze also logs it at startup: *"the plugin
declared events and no peer attaches it"*.

#### Editing a config means recreating the container, not restarting it

Ze persists its configuration, so `docker restart` on a scenario container runs
the peers of the FIRST boot and ignores the edited file the mount now carries.
`docker exec ... cat /etc/ze/bgp.conf` shows the new text while `show bgp peer
list` shows the old peers, which reads as a config that had no effect. Remove
the container and start a new one instead. Every restart-based config
experiment in a hand-built lab is void, and the harness is unaffected because it
creates each container once.

For Ze's configuration syntax, see [docs/architecture/config/syntax.md](../config/syntax.md).
Copy an existing scenario's `ze.conf` as a starting point.

## ExaBGP Wire Compatibility (`test/exabgp-compat/`)

A separate test suite validates that Ze's wire encoding matches the reviewed
ExaBGP API 6.0.0 contract fixtures. The compiled Go server negotiates each BGP
session and compares every received frame byte-for-byte.

### What It Tests

The harness migrates each ExaBGP-derived configuration, runs Ze, and compares
its wire bytes with the known-good fixture. The 42 `.ci` cases in
`test/exabgp-compat/encoding/` use `option=file:`, `option=serial`, `1:cmd:`,
`1:raw:`, and `1:json:` records rather than the standard `.ci` format.
`option=serial` marks process-driven fixtures that must not overlap other ExaBGP
harness instances; the runner executes those after the parallel batch.

Coverage includes:

| Category | Examples |
|----------|----------|
| Address families | IPv4/IPv6 unicast, VPN, FlowSpec, FlowSpec VPN, EVPN, VPLS, MPLS labeled, MUP, MVPN |
| Path attributes | ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, communities (standard, extended, large), AGGREGATOR, ORIGINATOR_ID, PREFIX_SID, SRv6 |
| Capabilities | 4-byte ASN, ADD-PATH, link-local next-hop, software version, hostname |
| Edge cases | Generic/unknown attributes, self-referencing routes, group limits, IPv4+IPv6 mixed configs, deferred announcement (watchdog) |

### Running

```bash
./le functional exabgp-test
```

ExaBGP compatibility is part of the offline precommit gate.
<!-- source: test/exabgp-compat/encoding/ -- .ci test files for wire compatibility -->

## Test Hierarchy

| Workflow | Includes Interop? | Includes ExaBGP? | Requires Docker? |
|----------|-------------------|-------------------|-------------------|
| Offline precommit gate | No | Yes | No |
| Standard functional sweep | No | Yes | No |
| `./le integration interop` | Yes | No | Yes |
| `./le functional exabgp-test` | No | Yes | No |

Interop tests are intentionally separate from the pre-commit gate because they require
Docker and take longer to run. ExaBGP wire compatibility tests run as part of the
standard verification suite.

## Current Scope

Interop scenarios cover core BGP: session establishment, route exchange, withdrawal,
capabilities (4-byte ASN, ADD-PATH, GR, route refresh, PATHS-LIMIT), communities, MD5
auth, route server behavior, route reflection, policy import/export, RPKI origin
validation, BMP monitoring, BFD failover, ECMP, SRv6 VPNv6, remove-private-as export
policy, GTSM, AS112, and non-unicast address families (EVPN, VPN, FlowSpec). The suite
also includes full IS-IS and OSPFv2/OSPFv3 interop families (adjacency, flooding, SPF,
dual-stack, authentication, TE, LFA/TI-LFA, graceful restart, and segment routing).
ExaBGP compat covers wire encoding for all supported address families.
<!-- source: test/interop/scenarios/ -- scenario directories -->

Not yet covered by interop tests:

- Long-Lived Graceful Restart with live peers

## Known Vendor Limitations

| Vendor | Limitation | Affected Scenario | Workaround |
|--------|-----------|-------------------|------------|
| GoBGP 3.31 | Deduplicates Multiprotocol capabilities by AFI. When two families share the same AFI (e.g., ipv4-unicast + l3vpn-ipv4-unicast, both AFI=1), GoBGP keeps only one. | bgp-vpn-gobgp | None from Ze side. Ze's OPEN is correct per RFC 4760. Families with different AFIs (e.g., ipv4-unicast + l2vpn-evpn) work fine. |
| BIRD 2.15 | Enforces next-hop reachability for IPv6 routes. On IPv4-only Docker networks, IPv6 next-hops are unreachable and BIRD rejects routes as invalid (RFC 7606 treat-as-withdraw). | bgp-ipv6-ebgp-bird | Add `multihop;` to BIRD config to disable the directly-connected next-hop check. |

<!-- source: test/interop/scenarios/bgp-vpn-gobgp -- GoBGP same-AFI dedup -->
<!-- source: test/interop/scenarios/bgp-ipv6-ebgp-bird -- BIRD next-hop reachability -->

## Related Documents

- [`.ci` test format](ci-format.md) -- Ze's standard functional test file format
- [Functional test system](../../functional-tests.md) -- complete guide to the functional test system
- [BGP implementation comparison](../../comparison.md) -- feature matrix comparing Ze with FRR, BIRD, GoBGP, ExaBGP, and others
- [ExaBGP comparison report](../../exabgp/exabgp-comparison-report.md) -- detailed implementation differences between Ze and ExaBGP

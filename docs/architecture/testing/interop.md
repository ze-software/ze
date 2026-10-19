---
title: Interoperability Testing (Testing Architecture)
---
# Interoperability Testing

Ze validates protocol correctness against production BGP daemons in two complementary ways:
live session interop tests (Docker containers running real daemons) and byte-level wire format
validation against ExaBGP (Ze's predecessor, a BGP implementation in Python).

For BGP terminology used in this document, see [BGP protocol](../../features/bgp-protocol.md).

`ai/rules/interop-and-goal-validation.md` states when an interop test is owed.
This page is the infrastructure it is owed against.

## The suites, one per protocol area

| Protocol area | Peer implementation | Scenario directory | Native action |
|---------------|---------------------|--------------------|---------------|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP, StayRTR | `test/interop/scenarios/` | `./le test integration interop` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/interop-ipsec/` | `./le test integration interop-ipsec` |
| L2TP | Docker | `test/interop-l2tp/` | `./le test deployment l2tp-test`, and `./le test deployment l2tp-ppp-test` for the full PPP and NCP path |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/interop-pppoe/` | `./le test deployment docker-pppoe-accel-test` |
| RADIUS (admin login: PAP, CHAP, EAP, Filter-Id) | Docker: FreeRADIUS | `test/interop-radius/scenarios/` | `./le test integration interop-radius` |
| RADIUS (subscriber: accounting, CoA and Disconnect) | Docker: FreeRADIUS, an xl2tpd + pppd LAC | `test/interop-radius-subscriber/scenarios/` | `./le test integration interop-radius-subscriber` |
| RSVP-TE (Ze as transit) | Docker: freeRouter, ingress and egress | `test/interop-rsvpte/scenarios/` | `./le test integration interop-rsvpte` |

<!-- source: internal/le/test/integration/gates.go -- interop, interop-ipsec, interop-radius, interop-radius-subscriber and interop-rsvpte verbs -->
<!-- source: internal/le/test/deployment/actions.go -- l2tp-test, l2tp-ppp-test, docker-pppoe-accel-test verbs -->

Every suite discovers its scenarios the same way. `Discover`
(`internal/le/interoplab/discover.go`) reads the scenario directory, keeps the
subdirectories, sorts the names lexically, and joins each one against the
owning checker registry. A scenario with no checker, or a nil checker, is an
ERROR rather than a skipped test, so a fixture and its registry cannot silently
disagree. Nothing depends on the order: each scenario gets its own setup, check
and teardown.

A reader outside a suite asks for its scenarios through the suite's catalog.
Each lab registers one in its `register.go` (`interoplab.RegisterCatalog`,
`internal/le/interoplab/catalog.go`), built on the same `Discover` call and
checker map its runner uses, so a name cited elsewhere is a name the runner
runs. `./le feature check` resolves a feature's Interop entry
`<suite>/<scenario>` this way, and names the registered suites when it refuses
one it does not know. The catalog also declares `RunScenario`, which runs ONE
scenario through the lab's own `RunAt` with the name as its selector, winning
over the lab's selector variable. `./le feature record-run` records an interop
green run through it, against the scenario directory's git tree id, so a
feature's Supported claim needs a current green run of each scenario it counts
(`docs/contributing/feature-maturity.md`, "Recorded runs").
<!-- source: internal/le/interoplab/catalog.go -- RegisterCatalog, CatalogNamed, Catalog.RunScenario -->

A scenario directory carries only declarative inputs its runner reads: `ze.conf`
plus the peer configuration and argument files that topology needs. Assertions
never live there. They are typed Go checkers under `internal/le/interoplab/`:
BGP builds its registry from `scenarioOperations` in
`internal/le/interoplab/bgp/checkers.go`, and IPsec declares `scenarioCheckers`
in `internal/le/interoplab/ipsec/checkers.go`. A checker waits for readiness,
asserts the protocol behaviour, verifies stability where the scenario needs it,
and returns an error on failure.

A scenario directory is NAMED and carries no numeric prefix. The name is the
scenario's identity: `Discover` matches it exactly, `./le test integration` takes it
as a scenario selector, and specs, journal rows and code comments cite it.

## Tested Daemons

| Daemon | Version | Image | Query Method | What It Validates |
|--------|---------|-------|--------------|-------------------|
| FRR | 10.3.1 | `quay.io/frrouting/frr:10.3.1` | vtysh | eBGP, iBGP, route exchange, GR, communities, MD5, route server |
| BIRD | 2.x (Alpine 3.21) | Alpine build | birdc | eBGP, route exchange, triangle topologies |
| GoBGP | 3.31.0 | Go builder | gobgp CLI | eBGP, route injection and verification |
| StayRTR | 0.6.4 | Go builder | HTTP `/rpki.json` export | RTR (RFC 8210) as the CACHE, so Ze is the client of an implementation that is not its own. Origin validation answers (RFC 6811) against VRPs a third party encoded |
| pmacct `pmbmpd` | latest | `pmacct/pmbmpd:latest` | JSON msglog file | BMP (RFC 7854, RFC 9069) as the COLLECTOR. pmacct decodes the per-peer header, the Peer Up Information TLVs and the Peer Down reason itself, so a scenario reads another implementation's reading of Ze's bytes rather than a string Ze emitted |
| ExaBGP | API 6.0.0 contract fixtures | Compiled Go wire server | Wire byte comparison | Byte-for-byte encoding across all address families |
<!-- source: internal/le/interoplab/bgp/prepare.go -- peer helpers and scenario preparation -->
<!-- source: internal/le/interoplab/bgp/run.go -- scenario orchestrator -->

The conditional-MED collector check requires a single AS number in the received
AS_PATH. It uses the shared AS-number parser, so decimal and dotted spellings
identify the same source without accepting a multi-AS path.
<!-- source: internal/le/interoplab/bgp/check_med_pmacct_epoch.go -- medWholeSetInputCriteria -->

The Extended Message FRR scenarios use a bounded, single-session relay that
changes capability 6 in OPEN only. It records Ze's original OPEN separately,
leaves UPDATE and KEEPALIVE bytes unchanged, caps each direction at 256 frames,
and joins both socket workers before returning. FRR 10.3.1 gates extension
bilaterally, so its facing OPEN always advertises capability 6 while Ze's facing
advertisement varies independently. This isolates Ze's directional behavior
without replacing FRR's UPDATE producer or consumer.
<!-- source: internal/le/interoplab/bgp/extended_relay.go -- runExtendedRelaySession, captureExtendedRelay -->

The producer is `frr` at host `.3`; the consumer is a separate `frr-sink` at
host `.15`, both in AS 65002, with distinct native router IDs matching those
addresses. Relays `.10` and `.11` connect them to Ze independently. The consumer
originates no test routes: its decoded path must come from `.11` with AS_PATH
`65001 65002`, and `allowas-in 1` admits the producer's AS once.
The old one-daemon topology presented the same `(AS, router ID)` on two configured
Ze peers. Ze's default identifier-claim policy rejected the second OPEN with 2/3
through `Peer.validateOpen`, `routerIDClaims.claim`, and
`Session.runOpenValidator`. This was not RFC 4271 Section 6.8 TCP collision
resolution, which closes the losing connection with Cease.
[RFC 6286 Section 2.1](https://www.rfc-editor.org/rfc/rfc6286#section-2.1)
says an identifier "should be unique within an AS"; Section 2.2 mandates Bad BGP
Identifier for zero or a self-identifier from an internal peer, not this case.
Section 3 explicitly mentions parallel sessions between the same speakers.
The fixture therefore uses distinct native speakers compatible with Ze's default
policy; it does not disable that policy or rewrite identifiers in transit.
FRR's [`bgp_open_make`](https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_packet.c)
writes each peer's native `local_id` into its OPEN.
<!-- source: internal/le/interoplab/bgp/register_extended_message.go -- independent producer and consumer -->
<!-- source: internal/le/interoplab/bgp/check_extended_message.go -- consumer route queries and per-daemon session generations -->

No test route is originated from the producer's startup configuration. The checker
first requires both native FRR sessions to be `Established` and both Ze peers
to report `established` through `show bgp peer list | json`. These two-sided
observations share one 60-second readiness deadline. Only then does it originate
the baseline once through FRR's existing `vtysh` path and require its decoded path
at the consumer. Large and later control routes follow through the same origin
path. This is a first-origination barrier, not a route retry or session replacement.
The source relay starts before the sink relay. A startup `network` announcement
could therefore reach `reactorForwardRSSection` before the sink has live
`forwardFacts`, when that destination is correctly skipped. These live fast-path
fixtures do not attach the route-server/Adj-RIB-In event consumers for retained
peer-up replay, so waiting for the sink afterward cannot recover that earlier
announcement. FRR's Established state alone is insufficient: FRR can receive Ze's
initial KEEPALIVE before Ze processes FRR's KEEPALIVE. Ze's peer-list producer
`handleBgpPeerList` serializes `reactorAPIAdapter.Peers`'s `Peer.State()`, not the
raw session FSM state. `Peer.runOnce` publishes negotiated encoding contexts and
forwarding facts before publishing `PeerStateEstablished`, so that peer state
is the local admission barrier. The checker reuses `waitZePeerState` for each
original relay peer and retains the independent FRR observations. It changes
neither AS-loop policy nor capability permissions.
<!-- source: internal/le/interoplab/bgp/check_extended_message.go -- first baseline origination after two-sided peer readiness -->
<!-- source: internal/le/interoplab/bgp/check_special.go -- waitZePeerState, zePeerState -->

The FRR-facing Extended Message relays start only after the native configuration probes pass.
Ze's CLI must list both relay peers. Each destination FRR daemon's last passive
neighbor must reach `Active (passive)`, after `peer_unshut_after_cfg` clears the configuration hold.
A listening TCP socket or successful `show version` is not sufficient: FRR
`bgp_accept` can close a connection while configuration is still loading.
These barriers use the existing ordered `ReadyProbe` hooks, with probe-specific
environment and required stdout strings. Probes with no required strings retain
exit-only readiness; relays to other daemons and other speaker personalities retain their startup order.
No readiness probe opens a BGP connection, and the relay never reconnects a
connected socket.
<!-- source: internal/le/interoplab/bgp/extended_relay.go -- prepareExtendedRelayPeers, dialExtendedRelay -->
<!-- source: internal/le/interoplab/bgp/prepare.go -- scenarioPeers -->
<!-- source: internal/le/interoplab/lab.go -- waitPeer -->

Bounded readiness waits check cancellation before each probe, including the
first. A simultaneously ready poll tick cannot admit another probe after
cancellation is observed. Failure retains the last measured state or probe
error and does not start dependent peers or the scenario checker.
<!-- source: internal/le/interoplab/wait.go -- Wait -->

A failed check names what the peer printed, not only what the checker
expected. A wait that measured the peer but timed out ends its error with
`last output:` and the last value its probe returned: text as the peer printed
it, any other value in Go's `%v` form. A wait whose value is a verdict, such as
a bool or a count, also ends with `last answer:` and the text its probe last
read. A one-shot assertion over an answer it read ends with `searched output:`
and that answer. Every quoted answer is trimmed and cut to its last 40 lines and
its last 4096 bytes, with a note on its first line saying how much was cut, so
a full routing table cannot flood the report. An empty answer is named as
empty.
<!-- source: internal/le/interoplab/output.go -- OutputExcerpt, WithSearchedOutput, WithLastAnswer -->
<!-- source: internal/le/interoplab/wait.go -- waitNotReadyError, waitValueExcerpt -->

The FRR readiness producers are
[`peer_unshut_after_cfg`](https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgpd.c),
[`bgp_start`](https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_fsm.c), and
[`bgp_accept`](https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_network.c).

All five `bgp-extended-message-*-frr` fixtures build the native FRR attribute
through four `EXTENDED` route-map sequences of 100 distinct large communities.
The first sets the attribute; the next three use `additive`, with `on-match next`
on the first three sequences. FRR therefore accumulates all 400 values (4800
attribute-value octets) on one route without exceeding its 4096-byte VTY input
buffer. Repeating `set large-community` within one sequence is not equivalent:
FRR replaces the previous set rule of that kind.
The FRR producers are [`config_from_file`](https://github.com/FRRouting/frr/blob/frr-10.3.1/lib/command.c),
[`route_map_add_set` and `route_map_apply_ext`](https://github.com/FRRouting/frr/blob/frr-10.3.1/lib/routemap.c),
and [`route_set_lcommunity`](https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_routemap.c).

Before teardown, a failed Extended Message checker reads both relays' result
JSON, original/delivered frame captures, TCP socket tables, and last 80 log lines.
The diagnostic queries share a 15-second deadline and preserve read errors and
the original assertion failure. They do not retry or replace the BGP session.
<!-- source: internal/le/interoplab/bgp/check_extended_message.go -- checkExtendedMessages, extendedFailureDiagnostics -->

`bgp-parsed-empty-mp-unreach-frr` drives the parsed forwarding path from an
ASN4 source to a two-octet recipient, with real FRR behind the OPEN-only relay.
Both sessions negotiate IPv4 and IPv6 without ADD-PATH. The FRR fixture supplies
a local IPv6 address: IPv6-family negotiation still needs it on IPv4 transport.
After initial synchronization, the source seeds a route, withdraws it in an
UPDATE carrying an empty IPv6 MP_UNREACH, then sends a genuine standalone IPv6
EOR in a separate epoch. Distinct-MED announcements fence each epoch.
<!-- source: internal/le/interoplab/bgp/speaker_parsed_empty_mp.go -- runParsedEmptyMPSource, parsedEmptyMPPhase -->

The checker requires FRR installation and removal, complete source and recipient
wire histories, the negotiated AS_PATH widths, and unchanged FRR daemon/session
identity. A mixed withdrawal must not manufacture an EOR; the later genuine
marker must survive. Before teardown, pass and failure both retain `proof.json`
under the native session scratch directory of the checkout the suite runs from,
including fixture inputs, captures, FRR table, PID, version and logs. The checker
resolves that directory against `CheckContext.Checkout`, never the working
directory, and refuses when no checkout is given. Required capture or write
failures fail the proof. Diagnostic collection has a cancellation-independent
fifteen-second bound.
<!-- source: internal/le/interoplab/bgp/check_parsed_empty_mp.go -- checkParsedEmptyMPFRR, parsedEmptyMPDiagnostics -->
<!-- source: internal/le/interoplab/bgp/proof_directory.go -- retainedProofDirectory -->
<!-- source: internal/le/interoplab/bgp/check_parsed_empty_mp_wire.go -- parsedEmptyMPCapture, parsedEmptyMPHistory -->

## Prerequisites

| Requirement | Used By | Notes |
|-------------|---------|-------|
| Docker | Interop tests | Containers for FRR, BIRD, GoBGP, Ze |
| A Docker daemon kernel with every feature Ze enrolls | Interop tests, `docker-*` deployment proofs, the `l2tp-test`, `vpp-test` and `vpp-iface-test` proofs, `TestVPPSRv6ServiceRoute`, the terminal-demo render and validate mode, the `./le perf` ze DUT, `./le verify evidence release-candidate` | Checked before any image build or container start; see "The Docker host kernel check" |
| ~1.5 GB disk | Interop tests | Docker images (Go builder, FRR, Alpine) |
| Docker | `./le test integration cleanup`, `./le test integration docker-peers` | No Ze kernel: neither runs Ze in a container |

The interop test network uses `172.30.0.0/24`. MD5 authentication scenarios require
`NET_ADMIN` capability (granted automatically by the orchestrator).

## Real VPP SRv6 service-route proof

Run the focused service-route proof through native admission:

```bash
./le job run label vpp-srv6 command go test -tags integration -count=1 \
  -timeout 30m ./internal/le/test/deployment \
  -run '^TestVPPSRv6ServiceRoute$' -v
```

This test reuses the deployment VPP image, its container (`NET_ADMIN`,
`/dev/net/tun` and `/dev/vhost-net`, never `--privileged`), build recipes,
sockets, and staged Ze configuration. It does not replace the legacy
`./le test deployment vpp-test` population. Docker, Go, a VPP image with the
SRv6 API and AF_PACKET plugin, and a Docker kernel with veth support are required.
The managed phase also needs `/usr/bin/vpp`, writable `/etc/vpp`, and sufficient
already-reserved hugepage resources for its configured 64M heap, 1024 buffers,
and 16M stats segment. The harness never changes global hugepage reservations.
`ZE_VPP_DOCKER_IMAGE`, `ZE_VPP_DOCKER_PLATFORM`, and `ZE_VPP_DOCKER_GOARCH` select
the image and matching target architecture through the existing VPP settings.
The test prints the selected image, VPP version, and retained scratch directory.
<!-- source: internal/le/test/deployment/vpp_srv6_integration_test.go -- TestVPPSRv6ServiceRoute -->
These Docker-host integration tests are excluded from the QEMU guest package
population: the guest has no Docker daemon. The native admission command above
runs their real container and packet probes rather than substituting guest mocks.
<!-- source: internal/le/test/qemu/alltests.go -- excludedIntegrationPackages -->
<!-- source: internal/le/test/deployment/vppiface.go -- newVPPIface, containerSpec -->

The compiled peer sends IPv4/unicast MP_REACH with an IPv6 next hop and an
RFC 9252 L3 Service TLV through a real BGP session. Only Ze installs service
policies and steering. The probe reads VPP's policy and steering dumps and CLI
output. It requires an encapsulation policy whose single segment is the received
SID, a distinct local BSID, and matching prefix steering in table zero. Two
prefixes share a policy. A normal Ze restart against the still-running external
VPP must retain the confirmed BSID identity without duplicates and resume packet
forwarding. Withdrawal of one, replacement of one SID, last-user withdrawal,
and reinstallation each have state and packet assertions.
<!-- source: internal/le/test/deployment/vpp_srv6_probe_integration_linux_test.go -- TestVPPSRv6ServiceRouteProbe, vppSRv6StateMatches -->

The same harness then switches to Ze-managed VPP. A test-only external plugin
uses the existing `sdk.Plugin.EmitEvent` ingress to publish best-change entries
for the same prefix in tenant tables 10 and 20. This is a separate producer
contract from the BGP/sysrib path above, not a claim of BGP VRF configuration.
Steering must use the tenant table while policy/outer lookup remains in backend
table zero. The probe kills the real managed VPP child, requires a different PID
and a production replay request, restores only interface/underlay prerequisites,
then answers that request through the ordinary event ingress. Restored forwarding,
repeated replay without duplicate BSIDs, replacement in one table, independent
withdrawal, shared-policy retention, and final removal are checked.
The runtime publisher also decodes the exact bytes it sends and requires their
replay marker to match the requested replay, rather than treating an ordinary
incremental event as a replay proof.
Before querying VPP, the probe waits for the binary API handshake's `Connected`
event using the production connector's bounded connection mechanism.
`AllPluginsReady` starts the plugin population; it does not prove the managed
VPP child has opened its API socket. The harness retains the complete managed
daemon/child stdout and stderr in `srv6-managed-daemon.log` and copies the
generated configuration and VPP logfile before container cleanup, including
failures before API readiness.
<!-- source: internal/le/test/deployment/vpp_srv6_managed_probe_integration_linux_test.go -- TestVPPSRv6ManagedProbe, vppSRv6Publish -->
<!-- source: internal/le/test/deployment/vpp_srv6_managed_integration_test.go -- vppSRv6Managed, vppSRv6ManagedConfig -->

The packet oracle injects a unique Ethernet/IPv4 datagram through a veth and
captures VPP's emitted frame. It checks the IPv6 source and received-SID
destination, reduced encapsulation without an SRH, and preserved inner payload
with one TTL decrement. Withdrawal must stop forwarding the corresponding
datagram; adjacent positive probes prevent a dead capture from passing.
The managed phase uses VPP's built-in packet generator for real graph ingress
and Ethernet egress capture, with the same independent byte oracle. It requires
the generated-packet counter to reach one before a negative assertion. Input and
output pcaps and the external plugin's complete test transcript remain in the
scratch directory.
Prerequisite mutations must return their successful CLI response: `vppctl` exit
status zero alone is not acceptance. A failed prerequisite or positive packet
assertion records bounded VPP logging, interface, error, IPv6 FIB, and trace
diagnostics. Packet observations log at most eight frame prefixes per injection;
these diagnostics do not replace the forwarding assertion or extend its deadline.
<!-- source: internal/le/test/deployment/vpp_srv6_probe_integration_linux_test.go -- vppSRv6CLIOutput, vppSRv6Diagnostics -->

The external-VPP configuration activates the real `connected` producer and
netlink interface monitor. A test-only SDK plugin waits for all plugins to be
ready, adds the Linux covering addresses through real netlink events, and queries
`show rib` until both next-hop and SID covering prefixes have protocol `connected`.
The BGP probe waits for that result before sending service announcements. After
Ze restarts, the SDK probe re-adds the addresses to the new monitor and publishes
a new process-generation readiness record; the old record cannot satisfy the
barrier. Its actual RIB result remains in `srv6-connected.log`.
<!-- source: internal/le/test/deployment/vpp_srv6_connected_integration_linux_test.go -- TestVPPSRv6ConnectedProbe, vppSRv6AwaitConnected -->

The fixture additionally supplies the VPP IPv6 underlay route, neighbor, tenant
tables, and encapsulation source. These
prerequisites are not evidence that Ze added underlay provisioning, SID
reachability validation, remote decapsulation, or external-VPP reconnect support.
<!-- source: internal/le/test/deployment/vpp_srv6_wire_integration_linux_test.go -- vppSRv6Packet, vppSRv6PacketMatches -->
<!-- source: internal/le/test/deployment/vpp_srv6_probe_integration_linux_test.go -- vppSRv6Underlay -->
<!-- source: internal/le/test/deployment/vpp_srv6_pg_integration_linux_test.go -- vppSRv6PGUnderlay, vppSRv6PGPacket, vppSRv6PGGenerated -->

Synthetic oracle tests check rejection of wrong policy bindings, tables, SIDs,
and packet fields. They are not dataplane evidence. A real proof requires an
executed, non-skipped `TestVPPSRv6ServiceRoute` result. For discrimination, omit
policy creation in `(*govppSRv6Backend).acquirePolicy`, rerun the command above
to rebuild the daemon, and require failure at the first installed-state assertion. Restore the source
and require the packet proof to pass before claiming closure.
<!-- source: internal/le/test/deployment/vpp_srv6_oracle_integration_linux_test.go -- TestVPPSRv6StateOracle, TestVPPSRv6PacketOracle -->
<!-- source: internal/le/test/deployment/vpp_srv6_integration_test.go -- TestVPPSRv6ServiceRoute -->

## Live Interop Tests (`test/interop/`)

Each scenario runs Ze and one or more peer daemons in Docker containers on a shared
network (`172.30.0.0/24`), establishes real BGP sessions, and asserts correct behavior
via each daemon's native CLI.

### How It Works

The native `internal/le/interoplab/bgp` package discovers scenario directories in
`test/interop/scenarios/`. Before any scenario, the shared `interoplab.Suite`
engine removes what a killed earlier run left behind ("Crash-safe teardown"
below), runs the lab's preflight (which stages ze), checks the Docker daemon's
kernel with that ze ("The Docker host kernel check" below), and builds the
images. For each scenario it then:

1. Creates an isolated Docker network, after refusing an existing container or
   network of the same name that it cannot prove abandoned.
2. Starts Ze and the peer daemons declared by the scenario's config files.
3. Waits for every declared readiness probe.
4. Runs the scenario's typed checker from the package-local BGP registry.
5. Tears down the containers and the network this scenario created, including
   after setup or checker failure and after a Ctrl-C or SIGTERM, which cancel
   the run rather than end the process. A resource is recorded for teardown
   only once its create succeeds (a failed `docker run` counts unless the
   daemon refused the name as taken), so a name the pre-clean refused, or one
   another run took first, is never removed. `Docker.CreateNetwork` refuses a
   network name that already exists rather than join it. A SIGKILL, a lost
   machine or a panic runs no teardown; the next lab start, or
   `./le test integration cleanup`, removes what it left.
<!-- source: internal/le/interoplab/lab.go -- Suite.Run, Lab.preClean, Lab.cleanup, createdResources -->
<!-- source: internal/le/interoplab/docker.go -- Docker.CreateNetwork, containerNameTaken -->
<!-- source: internal/le/test/integration/actions.go -- nativeRunner, signalContext -->

### Crash-safe teardown

Every container, one-shot container (the kernel probe included), network and
built image the wrapper (`interoplab.Docker`) creates carries owner labels, all
from one helper, `ownerIdentity.labelArguments`:

| Label | Value |
|-------|-------|
| `ze.lab` | `1`, the marker. The cleanup lists only resources carrying it |
| `ze.lab.run` | a random id for the run |
| `ze.lab.pid` | the creating process's pid |
| `ze.lab.pid-start` | that pid's start token, which changes when the pid is reused (`procscan`) |
| `ze.lab.boot-id` | the client machine's boot id |
| `ze.lab.machine-id` | the client machine's id (`/etc/machine-id`, or the Mac's IOPlatformUUID) |
| `ze.lab.started` | when the run started, RFC 3339 |
| `ze.lab.purpose` | the creating command line, for the operator |
| `ze.lab.kind` | the object the create made: `container`, `network` or `image` |
<!-- source: internal/le/interoplab/orphans.go -- labelMarker, ownerIdentity.labelArguments -->

A container inherits the labels of its image, so one a person starts by hand
from a lab-built image carries the builder's owner labels with `ze.lab.kind`
`image`. The wrapper sets the kind on every create, overriding the inherited
one, and the cleanup judges a resource only when its kind label names the
object it is: such a container is kept and reported as carrying image labels
only, not created by a lab run.
<!-- source: internal/le/interoplab/orphans.go -- ownerJudge.judge, resourceKind.labelValue -->

Every owner fact is read on the client, the machine running `le`, never from
the daemon: on a Mac the daemon runs in a VM whose pids are not the Mac's.

`Docker.CleanupOrphans` removes a labeled resource only when its owner is
provably gone, on one of three paths: the same machine and a different boot;
the same machine and boot with the pid absent; or the pid held by a process
with another start token. It removes containers, then networks, then the
run-owned `ze-interop-retained:` tag, each by the id the labeled listing gave.
Everything else is kept and reported with its reason and the command that
resolves it: an owner still running, an owner on another machine (run the
cleanup there), labels it cannot read, or a network another run's container is
still attached to. An unlabeled resource, a volume and the shared cache tag
that `NO_BUILD=1` reuses are never candidates.
<!-- source: internal/le/interoplab/orphans.go -- Docker.CleanupOrphans, ownerJudge.judge -->

A removal call that fails, or that outlasts the 30 s client bound while the
daemon completes it, is not reported failed on its own say. The wrapper
inspects that exact resource again under the same bound
(`Docker.confirmRemoval`): gone is "removed", still present is "failed" with
the removal's error, and an inspect that fails too is "unknown", naming both
errors.
<!-- source: internal/le/interoplab/docker.go -- Docker.confirmRemoval -->

A one-shot container (`Docker.RunOneShot`: the kernel probe, a host
preflight, a terminal-demo render) runs in the foreground with `--rm` and a
`ze-oneshot-<random>` name. Killing the docker client stops nothing the
daemon runs, so when the caller's context is canceled or the one-shot's bound
ends, the wrapper removes the container by that name on a context the
cancellation does not reach, and reports the removal's outcome with the
run's error.
<!-- source: internal/le/interoplab/docker.go -- Docker.RunOneShot -->

A caller outside a lab Suite that needs a long-running container, such as the
deployment proofs (`./le test deployment l2tp-test`, `vpp-iface-test`,
`vpp-test`), starts it with `Docker.RunDetached`. Its typed spec
(`DetachedContainer`: platform, network and address, capabilities, devices,
mounts, environment, working directory, entrypoint, command) carries no raw
docker flag, and the container carries the same owner labels, so a run killed
before its own removal is reaped like a lab's. Commands inside it run through
`Docker.ExecAttached`, with attached streams and their own bound. Those verbs
root on SIGINT and SIGTERM, remove their container on a context the signal
does not reach, and name a failed removal in the report's
`container-removal-error`, exiting 1.
<!-- source: internal/le/interoplab/container.go -- Docker.RunDetached, Docker.ExecAttached -->
<!-- source: internal/le/test/deployment/deployment.go -- removeContainer -->

The multi-DUT benchmark (`./le perf run`, `./le perf evidence-record`) is the
other such caller. Its network goes through `Docker.CreateNetwork` and
`Docker.RemoveNetwork` with the owner labels, each DUT and the sender through
`Docker.RunDetached`, the readiness wait through `Docker.ContainerRunning`,
and each image through `Docker.Build` (a named build context, such as
FreeRtr's source checkout, is a typed `NamedBuildContext`) followed by
`Docker.ReleaseImage`, which drops the run-owned tag and keeps the DUT's own.
`Docker.ContainerRunning` answers an inspect it cannot read as an error, never
as "not running".
<!-- source: internal/test/perfrunner/run.go -- Runner.measure, Runner.buildImages -->
<!-- source: internal/le/interoplab/container.go -- Docker.ContainerRunning -->

`Suite.Run` runs the cleanup after the Docker probe and before Preflight, and
prints each removal or skip as a progress line. A listing the daemon refuses is
a setup error. Before a scenario creates its network and containers, its
pre-clean removes an existing resource of the same name only on the same
verdict, and otherwise fails the scenario naming the owner and the fix: set
the lab's own suffix variable, which the message names from
`Suite.SuffixVariable` (`ZE_L2TP_INTEROP_SUFFIX` for the L2TP lab), or the
verdict's own fix: stop the owner pid it names, or run the cleanup. The
scenario's teardown then removes nothing of that name: it removes only what
the scenario itself created.
<!-- source: internal/le/interoplab/orphans.go -- Docker.claimName -->

`./le test integration cleanup` runs the cleanup by hand and exits 1 when a
removal failed, naming the failing command. `./le test integration cleanup
dry` lists what would go and what stays, removes nothing, and also lists
`ze-interop-retained:` tags left from before the labels existed, which no mode
removes. Both render through `| json`.
<!-- source: internal/le/test/integration/cleanup.go -- labCleanup -->

### The Docker host kernel check

A container runs on the Docker daemon's kernel, so a kernel feature Ze needs and
that kernel lacks fails scenarios for a fact about the host. `mobike-initiator`
and `mobike-responder` showed the cost: on a 6.8 kernel they spent fifteen
minutes and reported two protocol failures that were the missing
`XFRM_MSG_MIGRATE_STATE`. Every Docker run that runs Ze therefore checks the
daemon's kernel first, and refuses a kernel that lacks any feature. The owner's
ruling is that a wrong kernel "should not be possible - fail": there is no skip,
no warning mode and no exemption.

The required set is not listed in le. `Suite.Run` starts the ze the lab staged
for the daemon's architecture (`Suite.StagedZe`, from `StagedZePath`) in a
throwaway container and reads its answer:

```bash
docker run --rm --cap-add NET_ADMIN --cap-add SYS_ADMIN --network none \
  -v <staged ze>:/ze:ro alpine:3.21 /ze doctor --json kernel-capabilities
```

`ze doctor kernel-capabilities` probes every capability a package enrolls in
`internal/component/kernelcap`, with configuration ignored
(`docs/architecture/doctor-and-health-checks.md`). A capability enrolled later
is required of every Docker host with no le change. `NET_ADMIN` lets the
netlink probes ask; `SYS_ADMIN` lets the xfrm-interface probe unshare a network
namespace. Without it that row reads `unknown`, which also refuses.

On a daemon whose `docker info` security options list `name=apparmor`, the
probe container runs under Ze's AppArmor profile instead of `docker-default`
(`--security-opt apparmor=ze-kernel-probe`): docker-default denies the MPLS
transit MTU probe's mount and its `/proc/sys` write, and that row would read
`denied` (owner D-7: the host is set up for Ze, never unconfined). On a Linux
client whose `/sys/kernel/security/apparmor/profiles` lacks the profile, the
check refuses before any container starts and names
`./le setup docker-kernel apparmor confirm ze-kernel-probe`. An unreadable list
(apparmorfs refuses it to a reader without policy-view privilege, such as a
non-root user) is never read as "loaded": the probe runs, and
when runc answers "unable to apply apparmor profile" the check refuses with the
same command rather than reporting an unreadable answer. Both refusals also name
the daemon's kernel release and the kernel route below, because the profile lets
the probe answer and adds no kernel feature: a stock kernel may still lack some
(MOBIKE's xfrm migrate, for example), and the operator reads the whole path in
one run. A security
options query that fails or prints no JSON refuses, because the argv depends on
it. Ze's own kernel carries no AppArmor, so the QEMU guest's daemon lists none
and the guest route is unchanged.

<!-- source: internal/le/interoplab/kernelcheck.go -- dockerAppArmor, appArmorProfileMissing, appArmorProfileUnapplied, appArmorRefusal, kernelProbeContainer -->

The probe is the first container to run under a Ze profile, and the same rule
holds for every lab container: one that needs more than docker-default grants
runs under a Ze profile that grants exactly that, never `apparmor=unconfined`.
Each profile registers in `interoplab` (`RegisterAppArmorProfile`, from a lab's
`register_apparmor.go`) with its name, what runs under it, what docker-default
denies it, its text and the command to run once it is loaded. A peer names its
profile in `PeerConfig.AppArmorProfile`. On a daemon that applies AppArmor the
container starts with `--security-opt apparmor=<name>`; on a Linux client whose
profile list lacks it, the lab refuses before the container starts, naming
`./le setup docker-kernel apparmor confirm <name>`; where that list is
unreadable the container runs, and when runc answers "unable to apply apparmor
profile" the lab refuses naming the same command (`labAppArmorUnapplied`); on a
daemon that applies no AppArmor the option is left out. `ze-kernel-probe` parses with `apparmor_parser -Q`
4.0.1 (Ubuntu 24.04).

A lab peer runs with no more than its daemon needs, never `--privileged`
(`plan/pre-release/spec-lab-containers-least-privilege.md`). A sysctl set once
before the daemon starts is a `--sysctl` argument, written by Docker into the
peer's own network namespace. A sysctl written while the peer runs takes
`interoplab.NetSysctlWriteArguments` (`systempaths=unconfined`) together with
the generic profile `ze-lab-net-sysctl`, which is docker-default with every
`/proc/sys` write denied outside `net/` and every mount denied; any lab names it,
none defines its own copy (owner, 2026-10-10: one shared profile granting only
what the labs together need). The VRRP scenarios' ze (any scenario carrying
`keepalived.conf`) is such a peer: once it knows its interface VRRP writes the
per-device knobs under `net/ipv4/conf/` and `net/ipv6/conf/` and the
namespace-wide `net/ipv4/conf/all/rp_filter` and
`net/ipv4/icmp_errors_use_inbound_ifaddr` (`applyDataplaneSysctls`,
`internal/plugins/vrrp/dataplane_linux.go`), and
`TestVRRPWritesOnlyLabGrantedSysctls` records every write it makes and checks
the profile grants each. In the IPsec lab the NAT box, strongSwan and ze hold
`NET_ADMIN`; the NAT box sets its three sysctls through `--sysctl`; strongSwan
runs under `ze-lab-net-sysctl` only in a scenario whose `scenarioCheckers`
entry sets `writesPeerSysctl` (`internal/le/interoplab/ipsec/checkers.go`),
because its checker cuts strongSwan's reassembly marks at run time;
`TestOnlyCheckersThatWritePeerSysctlsAreFlagged` reads the source and refuses
a checker that reaches `dropFragmentsAtPeer` without the flag, or the flag
without the write. In the L2TP lab ze and the xl2tpd LAC hold `NET_ADMIN` and
`--device /dev/ppp`, and the preflight probes with the same grants and loads no
module, so it passes only where the peers can run. The PPPoE lab grants ze,
accel-ppp and the pppd client the same two, its preflight probes the same way,
and no PPPoE image installs `kmod` or runs `modprobe`. The RADIUS subscriber lab
grants ze and its LAC the same two and FreeRADIUS neither.

The `/dev/ppp` half of those preflights is declared once, in
`internal/le/interoplab/pppdevice.go`, because the Docker host kernel check
below cannot see a character device. Each lab still runs one preflight
container: it puts `PPPDeviceProbe` (the line printing `DEV_PPP=ok` or
`DEV_PPP=missing`) first in its `sh -c` command and adds its own probes, parses
the output with `ParsePreflight`, takes the device entry of its refusal from
`PPPDeviceMissing`, and refuses a container that did not run through
`PPPPreflightRunRefusal`. That refusal always wraps Docker's error, keeping the
operation, exit code, stderr and cause, and adds `sudo modprobe ppp_generic`
only when Docker's stderr names `/dev/ppp`.

<!-- source: internal/le/interoplab/pppdevice.go -- PPPDeviceArguments, PPPDeviceProbe, ParsePreflight, PPPDeviceMissing, PPPPreflightRunRefusal -->

<!-- source: internal/le/interoplab/apparmor.go -- RegisterAppArmorProfile, appArmorSecurityOption, labAppArmorProfileMissing, labAppArmorUnapplied -->
<!-- source: internal/le/interoplab/apparmor_netsysctl.go -- NetSysctlAppArmorProfileName, NetSysctlGranted -->
<!-- source: internal/plugins/vrrp/dataplane_linux.go -- applyDataplaneSysctls -->

The check runs after the preflight, because it needs the staged ze, and before
the first image build, so a refused host costs no build and counts no scenario.
It refuses when:

| Answer | Refusal names |
|--------|---------------|
| a row is `absent`, `unknown` or `denied` | every such row by subsystem, `CONFIG_` symbol, state and reason, the daemon's kernel release, and the next step for the platform (`DockerKernelRoute`, owner D-6), its command on a line of its own: on Linux a kernel with every feature or `./le setup docker-kernel install` and a reboot; inside the Docker lab's QEMU guest, which exports `ZE_DOCKER_LAB_GUEST=1` before its check, the appliance kernel's own fix instead (add the options to `gokrazy/kernel/runtime.config` and `runtime.require`, then rebuild the kernel cache entry on the host with `./le build host-driver` and `./ze-host appliance kernel --target runtime --arch <arch>`); elsewhere the refused run's own le words inside the Ze-kernel QEMU guest, `./le test qemu docker-lab lab '<the words>'` printed with the words filled in, followed by `env '<NAME=value>'` with each scenario selector variable the suite declared to `ReadEnvironment` and the run set (`IPSEC_INTEROP_SCENARIO=mobike-responder`, for example), so the guest reruns the one scenario rather than the whole suite; or `./le test qemu docker-lab` (boot and check only) when the process is not le or a word or an assignment holds white space or a quote. A `denied` row adds, on a daemon applying AppArmor, `./le setup docker-kernel apparmor confirm ze-kernel-probe`; on any other daemon it says the policy the row names must grant the probe its steps, and under SELinux names `sudo ausearch -m avc -ts recent` to read the denial |
| the container fails, or prints no JSON | the command it ran, its exit code, stdout and stderr |
| the answer holds no row | the command it ran |
| every row is `present` but the probe exits non-zero or answers `ready` false | the command it ran, its exit code and the `ready` it read: the producer's verdict and its rows disagree, so the answer is unread, never a pass |
| the suite names no staged ze | `Suite.StagedZe` |

The refusal is the suite's setup error: `interop: setup: <reason>`, exit 1. The
`docker-l2tp-ppp-test` and `docker-pppoe-accel-test` deployment proofs run the
L2TP and PPPoE suites, so they reach the same check. A suite with no kernel
module of its own, such as the FreeRADIUS admin-login suite, is checked too:
the owner's rule is about the host, not about the scenario.

The `l2tp-test`, `vpp-test` and `vpp-iface-test` deployment proofs run no suite,
and neither does `TestVPPSRv6ServiceRoute` (build tag `integration`), which
prepares its daemon through the step `vpp-iface-test` uses. Each cross-compiles its own daemon to `tmp/evidence/bin/ze-linux-<arch>`, and
`buildCheckedDaemon` runs the same check with that binary right after the build,
before the proof inspects or pulls its peer image and before its first
container, so a refused host downloads and starts nothing. With
`NO_BUILD=1` the proof skips the build and probes with the daemon already at
that path, and refuses by name when there is none.

The terminal-demo render (`le site terminal-demo render` and `render-all`) and
the renderer's `validate` mode run Ze in validator and recorder containers, so
both make the same check, through one shared preflight, with the demo binary
(`tmp/terminal-demos/bin/ze`) after confirming that binary exists and before the
renderer image is inspected or any container starts.

`./le perf run` and `./le perf evidence-record` run the ze DUT in a container,
so a run that tests the ze DUT (every DUT, or `ze` named) first cross-compiles
the ze the DUT image carries, `test/interop/ze-linux` (the path the BGP lab
declares, through the BGP lab's own staging), then makes the same check with it
before the benchmark starts. A build alone, or a run of other DUTs only, starts
no ze, stages nothing and does not check.

Every route refuses a staged ze path that holds no file, or holds a directory,
naming the path, before any docker command runs. Docker answers a bind mount of
a missing path by creating a root-owned directory there, which a later staging
then cannot replace. A missing file names, on a line of its own, the run's own
le words again: `env -u NO_BUILD ./le <words>` when the run skipped its build,
`./le <words>` otherwise (perf stages without `NO_BUILD`).
<!-- source: internal/le/perf/bench.go -- Bench.Run, runsZe, stageZe, stagedZe -->
<!-- source: internal/le/interoplab/kernelcheck.go -- checkDockerKernel, stagedZeMissing, stageZeAgain -->

`./le verify evidence release-candidate` runs the whole verify gate, Ze's
functional tests included, inside a container on the daemon's kernel. After the
clean-tree check and before the container starts, it cross-compiles a linux ze
for the daemon's architecture into `tmp/verify-evidence/ze-linux`, which git
ignores so the tree stays clean, and makes the same check with it. The
container never runs `--privileged`: it holds one `--cap-add` for each
capability the `option=needs-linux:caps=` vocabulary names and one `--device`
for each node a production path reached by a test opens (`/dev/ppp`, opened by
`ppp.DevPPPSetup`), read from the runner's own tables
(`runner.ContainerCapsDockerNames`, `runner.ContainerDevicePaths`), and keeps
Docker's default seccomp and AppArmor profiles. A test declares no device:
`TestContainerDevicesCoverWhatPPPOpens` reads the L2TP sources for every
`/dev` node they open and checks the table holds exactly those. A token added
to either table reaches the container with no other edit. `sys-time` is left out:
a container shares the Docker host's clock, so a verify run must not set it,
and the NTP restore test skips there through its own caps gate. A container
never loads a kernel module, so a Docker host without one of those nodes makes
Docker refuse the start; the run then answers an error naming the node and the
host command that creates it (`sudo modprobe ppp_generic` for `/dev/ppp`)
rather than a bare exit code.

The container is a `Docker.RunOneShot`, so it carries the owner labels and a
name, and runs under a four-hour bound (owner D-1, 2026-10-19): the gate takes
25 to 53 minutes, so the bound never ends a normal run and only stops a wedged
daemon. SIGINT and SIGTERM cancel the action's root context; the wrapper then
removes the container by name, and the run answers the interruption as an
error, never as a failed gate. The three named cache volumes
(`ze-gomod-cache`, `ze-gobuild-cache`, `ze-apt-cache`) are mounted, never
removed: they are what keeps a second run minutes long.
<!-- source: internal/le/verify/evidence/evidence.go -- Runner.Run, Runner.container, evidenceContainerTimeout, deviceRefusal, dockerKernel, kernelZeRel -->
<!-- source: internal/le/verify/evidence/actions.go -- signalRooted -->
<!-- source: internal/test/runner/caps.go -- capsRequired, capDockerName, capsAltersHost, ContainerCapsDockerNames -->
<!-- source: internal/test/runner/devices.go -- containerDevices, ContainerDevicePaths, DeviceHostModule -->

`./le setup docker-kernel check` asks the daemon in hand the same question. With
no `ze` it cross-builds `tmp/qemu/linux-<arch>/ze` for the daemon's architecture
(`docker version`'s server arch) through `BuildLinuxZe`, the producer
`./le test qemu docker-lab` uses; `ze <path>` names another. Either way the file
must be an ELF executable for that architecture, or the check refuses before any
container runs, naming the path, what it is instead (a directory, not ELF,
another machine), and the build route. `./le setup docker-kernel apparmor` is Linux-only:
it refuses where `/sys/module/apparmor/parameters/enabled` does not read `Y` or
`apparmor_parser` is absent, installs the registered profile named after
`confirm` as `/etc/apparmor.d/<name>` (so it loads again at boot) and loads it
with `apparmor_parser -r -W`; without `confirm` naming a registered profile it
prints the two `sudo` steps of every registered profile, runs none, and names
the command that loads each. `./le setup docker-kernel install` is the Linux
route: it installs the cached runtime kernel for the host's architecture under
`/boot` and `/lib/modules`, rebuilds the initramfs and the GRUB menu with the
Debian tools, and saves the new entry as GRUB's default by its title. It
refuses before any step unless `GRUB_DEFAULT=saved`, and unless the cached
release ends with the `CONFIG_LOCALVERSION` suffix its config declares (`-ze`,
so never a bare upstream release another kernel could own), and unless the
cached kernel's config builds in, or as a module, every option this host
relies on its running kernel for: the PCI hotplug drivers when an interface in
`/sys/class/net`, or one netplan's generated
`/run/systemd/network/10-netplan-<id>.network` names, is a slot name (`ens18`,
which the kernel without them renames `enp0s18`); the option behind each
iptables table, match or target module in `/proc/modules`; and UFW's stock
matches and targets when `/etc/ufw/ufw.conf` exists. One table maps each
module to its option, and a loaded netfilter module it does not map is
refused by name. The refusal names each dependency, what the kernel would
break, the missing options, and the rebuild (`hostdeps.go`). After the steps
it reads `/boot/grub/grub.cfg`, through `sudo cat` when the host keeps it
root-only, to find the entry's title. Without
`confirm <release>` it prints the steps and runs none, because passwordless
sudo asks nothing: `./le setup docker-kernel install confirm 7.2.9-ze`. The
cache entry is keyed by the backend that built the kernel, and ze-host answers
docker's entry whenever the docker binary is on PATH, so a kernel built with
QEMU on such a host is named with `builder qemu`
(`./le setup docker-kernel install builder qemu confirm 7.2.9-ze`); the dry
run's confirming command and every rebuild it prints keep that backend. A
reinstall replaces: it removes `/lib/modules/<release>` and
`/boot/initrd.img-<release>` before writing them again, so `cp -a` never nests
the new modules inside the old tree. It prints each `sudo` step before it runs
it, and never reboots. Off Linux it refuses and names the guest
route, `./le test qemu docker-lab` (`docs/architecture/testing/qemu-integration.md`).
<!-- source: internal/le/setup/dockerkernel/dockerkernel.go -- runDockerKernelCheck, dockerKernelProbeZe, linuxExecutableFor, runDockerKernelInstall, cachedKernelServesHost, readGrubConfig -->
<!-- source: internal/le/setup/dockerkernel/hostdeps.go -- netfilterModuleOptions, ufwModules, slotNameOptions, readHostFacts, hostDependencies, zeKernelServesHost -->
<!-- source: internal/le/test/qemu/dockerlab.go -- BuildLinuxZe, GuestZeRel -->
<!-- source: internal/le/interoplab/kernelcheck.go -- DockerKernelRoute -->

To ask a host by hand, stage ze with any lab once and run the command above.
`TestDockerKernelCheckOnThisHost` (build tag `integration`) does that against
the daemon this machine uses: it stages ze, starts the probe container and
nothing else, and judges the verdict against the rows it read. On colima's
`6.8.0-117-generic` (2026-10-10) the check refuses `ipsec-mobike`, `l2tp`,
`l2tp-ppp` and `mpls` as absent and `mpls-transit-mtu` as unknown.

```bash
./le job run label kernel-host command go test -tags integration -count=1 \
  -run '^TestDockerKernelCheckOnThisHost$' ./internal/le/interoplab/
```
<!-- source: internal/le/interoplab/kernelcheck.go -- DockerKernel, checkDockerKernel, StagedZePath -->
<!-- source: internal/le/interoplab/lab.go -- Suite.Run, Suite.StagedZe -->
<!-- source: internal/le/test/deployment/daemonkernel.go -- buildCheckedDaemon -->
<!-- source: internal/le/test/deployment/vppiface.go -- vppIface.prepareDaemon -->
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.preflightContainers, Engine.validationCheckAll, Engine.validateAndRender, Engine.checkKernel -->

The report prints only after the last scenario, so `Suite.Run` also writes
progress lines while it runs: one as image preparation starts, one when the
images are ready with the time it took, and two for each scenario, `interop: [i/n] name:
running` and then `pass` or `FAIL` with its elapsed time and error. They go to
`Suite.Progress`, or to stderr when that is nil, which is the job log in CI. A
suite that hangs then names the scenario that holds it. A failed progress write
does not fail the suite.
<!-- source: internal/le/interoplab/lab.go -- Suite.Run, Suite.progress, scenarioLine -->

Each local image build also creates a unique, run-owned tag. This keeps its
image ID available when another build replaces the shared cache tag.
The suite removes its tags after the scenarios, including after setup failure.
Image cleanup errors appear in `cleanup-errors` and make the suite fail.
<!-- source: internal/le/interoplab/docker.go -- Docker.Build, Docker.ReleaseImage -->

One `./le test integration <action>` invocation returns that action's full report.
Multiple action names return a combined summary.

The image-retention regression, `TestDockerBuildRetainsImageAcrossRetag`,
carries the build tag `docker`, so `./le test integration docker-peers` runs it
against the host Docker daemon with `ZE_REQUIRE_DOCKER=1`. Run it alone through
native admission:

```bash
ZE_REQUIRE_DOCKER=1 CGO_ENABLED=0 ./le --name image-retention job run label image-retention command \
  go test -tags docker -count=1 ./internal/le/interoplab \
  -run '^TestDockerBuildRetainsImageAcrossRetag$'
```

Neither it nor `TestKilledLabRunIsCleanedByNextStart` sets a bound of its own.
Each build takes the wrapper's machine bound, and the test's context ends ten
seconds before the deadline that `go test -timeout` sets. A one-minute build
bound once failed the retention test under `docker-peers`, which runs every
docker package in parallel.
<!-- source: internal/le/interoplab/image_retention_docker_test.go -- TestDockerBuildRetainsImageAcrossRetag -->
<!-- source: internal/le/interoplab/deadline_docker_test.go -- testDeadlineContext -->
<!-- source: internal/le/test/integration/dockerpeers.go -- dockerPeersArgv, requireDockerEnvironment -->

`internal/le/interoplab` stays out of the QEMU guest population because of
`TestDockerKernelCheckOnThisHost` alone: it judges the kernel of the daemon the
host uses, which a guest cannot stand in for.
<!-- source: internal/le/test/qemu/alltests.go -- excludedIntegrationPackages -->

Daemons start conditionally: FRR if `frr.conf` exists, BIRD if `bird.conf` exists,
GoBGP if `gobgp.toml` exists. This means each scenario only runs the daemons it needs.

**A scenario directory is NAMED, never numbered** (owner directive, 2026-08-24).
Run order is lexical by scenario directory name and no scenario depends on it.
The rule and its reasoning live in `ai/rules/interop-and-goal-validation.md`,
which is always-on, so a spec author planning a scenario meets it without
opening this page.

### The exact grant-set check

Each lab's privilege test (`TestL2TPLabPeersRunWithoutPrivilege` and its
PPPoE, IPsec and RSVP-TE siblings, the deployment proofs' container tests and
the release-candidate container test) compares a container's whole grant set
with the set its daemon needs, through `internal/le/interoplab/grantcheck`. A
check that only looked for the wanted grants would pass a container that also
held `--cap-add ALL` or `--privileged`; comparing the whole set fails on any
extra.

`grantcheck.Parse` reads a `docker run` argv, or a lab peer's extra arguments,
in every spelling Docker reads (`--flag value`, `--flag=value`, a short flag
with its value joined or separate, combined short booleans). A flag that can
widen what the container may do fills one list: capabilities, devices,
security options, sysctls, host namespaces, networks, device cgroup rules,
mounts, or other grants (`--user`, `--volumes-from`, `--runtime` and the like).
`--privileged` counts in any spelling Docker reads as true. A flag classified
as no grant (`--name`, `--env`, `--rm`...) is skipped. Every other flag is
unclassified, and `grantcheck.Problems` reports each one whatever the wanted
set, so a flag nobody classified fails the check rather than passing unread:
classifying it is an edit to `grantFlags` or `nonGrantFlags`. A leading
`docker` is skipped, reading stops at `--` or at the image, and an argv that
opens with any other word is refused, because reading would stop there before
a single flag.

`grantcheck.ParsePeer` adds a lab peer's declared capabilities, its AppArmor
profile and its mounts. A read-only mount of a file whose source, symlinks
resolved, lies strictly inside the checkout or the system temporary directory
(`grantcheck.StagedRoots`) is the lab staging its daemon's own config and is no
grant. A writable mount, a directory, a unix socket, a root itself, a source
whose symlinks do not resolve, or a source anywhere else is a mount grant the
want must name. Every lab mounts files only, and a directory is refused because
what it holds, a socket included, reaches the container unread.

<!-- source: internal/le/interoplab/grantcheck/grantcheck.go -- Grants, Parse, ParsePeer, Problems, StagedRoots, stagedInside, resolvePath -->

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
| pmacct `pmbmpd` | 172.30.0.13 | `ze-iop-pmacct-<pid>` |

Container names include the runner PID, which prevents name collisions.
Runs that share a fixed subnet or staged binary paths must run serially.

<!-- source: internal/le/interoplab/bgp/prepare.go -- container naming, IP addresses -->

The table gives the fixture addresses, which the runner rewrites onto its selected
network. IPv6 scenarios also receive host addresses on a selected /64.

A scenario file may carry a link-local placeholder. Each one renders to an
`fe80::` address whose interface identifier holds the first six bytes of the
selected IPv6 lab prefix and, in its last byte, the host number of the container
that owns it:

| Placeholder | Owner | Host |
|-------------|-------|------|
| `@ZE_LINK_LOCAL@` | `ze` | 2 |
| `@FRR_LINK_LOCAL@` | `frr` | 3 |

When any file of the scenario carries a placeholder, its owner's start is
wrapped: `/bin/sh` assigns the address to `eth0` with `nodad`, then execs the
start it replaced (Ze's `ze`, or FRR's image entrypoint `/sbin/tini` with its
command `-- /usr/lib/frr/docker-start`). The daemon's first interface snapshot
therefore already holds the address. A placeholder also selects an IPv6 lab
network, because the address is built from its prefix. The scenario's name
decides nothing.

A scenario's prestart is Go, not a scenario file: its register file adds an
`ownerPrestarts` entry, one shell fragment per link-local owner, to
`scenarioPrestarts` under the scenario's name. The interop tree carries no
interpreted helper (`TestInteropTreeUsesOnlyCompiledHelpers`), so the harness
compiled into `le` holds it. The fragment's link-local placeholders render as
they do in the scenario's files, and it runs inside that owner's wrapper, under
`sh -e`, after the `eth0` assignment and before the exec. It is where a
scenario builds what must exist before its daemon sends a packet: a second
link, a link-local on it, an nft table. A prestart for an owner whose
placeholder no scenario file carries would never run, so preparation refuses
it by name rather than skipping it.

`bgp-rfc2545-linklocal-nexthop-frr` uses IPv6 host 2 for Ze and host 3 for FRR:
the speaker, its on-link global next hop and the recipient share ONE IPv6 subnet,
not separate IPv4 and IPv6 connected prefixes. Its `ze.conf` carries
`@ZE_LINK_LOCAL@`, so the advertised link-local next hop belongs to the same
router as the global address.
The checker requires FRR to decode the global-plus-link-local pair and install
the on-link route via that owned link-local address. A second prefix uses an
off-link global next hop and must decode as global-only; the IPv6 session must
remain established after both assertions. These are two routes on one session,
not identical UPDATEs differing only in their next-hop length.
<!-- source: internal/le/interoplab/bgp/prepare.go -- prepareScenario, renderScenario, needsIPv6, linkLocalOwners, linkLocalAt, assignLinkLocal, ownerPrestarts, scenarioPrestarts, renderPrestarts -->
<!-- source: internal/le/interoplab/bgp/check_rfc.go -- checkRFC2545NextHops -->

Typed checker queries and their expected JSON field values are rendered onto the
same selected network. Rendering clones the expected-field map so concurrent or
later runs cannot change the registered scenario's addresses. In `inject.msg`,
literal UPDATE sends also receive the selected lab address in IPv4 NEXT_HOP
and MP_REACH next-hop fields, including the address after a VPN RD. The
renderer validates the envelope and attribute sequence before changing either
field. RDs, IPv6 addresses, NLRI and unrelated attributes remain intact.
Malformed framing and duplicate attribute codes remain unchanged,
preserving negative protocol fixtures rather than repairing them.
<!-- source: internal/le/interoplab/bgp/check_engine.go -- rewriteOperation -->
<!-- source: internal/le/interoplab/bgp/prepare_inject.go -- renderInjectedNextHops, renderInjectedNextHop -->

The IPsec, L2TP, PPPoE and RADIUS labs mount only Ze's input configuration file
read-only. Its parent `/etc/ze` belongs to the container and remains writable,
so `start /etc/ze/ze.conf` creates its database tree there without writing to
the scenario source directory. Each new container gets its own tree.
<!-- source: internal/le/interoplab/ipsec/ipsec.go -- prepareScenario -->
<!-- source: internal/le/interoplab/l2tp/l2tp.go -- zePeer -->
<!-- source: internal/le/interoplab/pppoe/scenarios.go -- prepareZeClient, prepareZeAccessConcentrator -->
<!-- source: internal/le/interoplab/radius/radius.go -- zePeerConfig -->

The deployment L2TP PPP proof uses a private `/tmp` directory owned by its
invoking user and writes Ze's explicit configuration beneath `ze/`. VPP stages
each input from the host scratch mount into `/run/ze/<scenario-config>/` inside
the container, where its explicit `ze.conf` and database tree live together.
The root daemon cannot use a database directory beneath a mount owned by another
user. A scenario's disable-and-restart check recopies the new input into the
same private directory before each start.
<!-- source: internal/le/test/deployment/l2tppppinputs.go -- writeInputs -->
<!-- source: internal/le/test/deployment/vppiface.go -- writeScratch, stageConfig, daemonExec -->
<!-- source: internal/le/test/deployment/vppevidence.go -- writeConfig, evidenceDaemonExec -->

After the native L2TP PPP proof has negotiated IPCP and carried traffic, it
injects an LCP Configure-Request from the peer namespace into the existing
kernel L2TP session. The packet uses that session's live tunnel/session IDs and
UDP endpoints. [RFC 2661 section 3.1](https://www.rfc-editor.org/rfc/rfc2661#section-3.1)
says, “Tunnel ID in each message is that of the intended recipient, not the
sender,” and gives the same rule for Session ID.
[RFC 1661 section 3.4](https://www.rfc-editor.org/rfc/rfc1661#section-3.4)
says, “The receipt of the LCP Configure-Request causes a return to the Link
Establishment phase from the Network-Layer Protocol phase or Authentication
phase.” Its section 4.1 specifies `tld,scr,sca/8` for an acceptable request in
Opened. The independent xl2tpd/pppd peer then renegotiates LCP and IPCP; a
credentialed input must also complete a fresh authentication exchange.
`ZE_L2TP_PPP_SCENARIO=no-auth` is the default. Select `chap-md5` to use the
existing local credential handler and require CHAP-MD5 on both negotiations.

Each namespace must have exactly one kernel tunnel and one nonzero-ID data
session. xl2tpd's kernel path also creates a tunnel-management PPPoL2TP socket:
[Linux `pppol2tp_connect`](https://github.com/torvalds/linux/blob/v6.8/net/l2tp/l2tp_ppp.c)
registers its local/peer session IDs as zero, without a PPP channel, and
[`l2tp_nl_cmd_session_dump`](https://github.com/torvalds/linux/blob/v6.8/net/l2tp/l2tp_netlink.c)
enumerates it alongside the data session. The proof validates this optional
management record separately: both session IDs must be zero, both tunnel IDs
must match the live tunnel, and no duplicate management record or second data
session is accepted. Management presence is part of the before/after identity;
its zero IDs never become injection targets.

The proof records the pppd log offset, Ze observation counts and both kernel
transport identities before injection. Only later request/Ack exchanges,
authentication when selected, route withdrawal/reinjection and address
assignment can satisfy the restart assertion. Both namespaces must retain
their original transport identities, recover their negotiated kernel addresses
and deliver all three interface-bound ping replies in each direction. Finally,
the peer leaves first and the proof requires another route withdrawal and
ordinary kernel cleanup. Injection uses Python 3's standard-library raw IPv4
socket and requires `CAP_NET_RAW` in addition to the namespace/PPP privileges.
The boundary JSON and peer log remain in the scratch directory on failure.
Raw tunnel/session listings are also printed before the restart identity check
returns, while both peers are still running. Later general diagnostics run
after deferred peer/daemon cleanup; their empty kernel state or pppd SIGTERM
must not be read as the state that caused an earlier assertion to fail.
<!-- source: internal/le/test/deployment/l2tpppprestart.go -- assertLCPRestart, awaitL2TPPPPWithdrawal -->

The credentialed carrier then starts another xl2tpd/pppd peer with a wrong
secret against the same daemon. It requires a matching CHAP
Challenge/Response/Failure exchange, a local rejection, and session teardown.
After the rejection boundary, neither the rejected peer's log nor new Ze
observations can show network admission. PPP units, L2TP data sessions and
subscriber routes must disappear before the proof stops the rejected peer.
The control tunnel and its validated zero-ID management context may remain
until that peer exits; they cannot stand in for a surviving PPP data session.
<!-- source: internal/le/test/deployment/l2tppppauth.go -- assertWrongSecret, l2tpPPPCHAPRejected, l2tpPPPRejectedState -->

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

Scenario-local `register_*.go` files can register bespoke checkers. The
conditional-MED checker runs its four original baseline operations unchanged,
then correlates collector events without adding an operation kind to the engine.
<!-- source: internal/le/interoplab/bgp/register_med_pmacct.go -- init -->

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
| `inject.msg` | `le test peer` (raw injector, 172.30.0.9) | Drive Ze with wire bytes no conforming daemon would emit. An optional `inject-args` file adds flags. Because the injector and Ze start before the peer daemons, an early route exercises Ze's replay-on-peer-up path. |
| `speaker-args` (and optional `speaker2-args`) | `le test interop-bgp speaker` (172.30.0.10; second at 172.30.0.11) | Dial Ze with an independent strict peer. The compiled speaker negotiates the requested families and ADD-PATH mode, frames BGP itself, applies the named native oracle, and writes a structured verdict to container logs. It catches wire output that Ze's own lenient decoder could accept. |
| `vrps.json` | StayRTR (172.30.0.12:8282) | Serve RPKI VRPs from a real third-party cache, so Ze is the RTR client of an implementation that is not its own. The typed checker asserts each per-prefix validation answer, not merely the RTR session. |
| `dnsmasq.conf` (and optional `dnsmasq-b.conf`) | dnsmasq DHCPv4 server (172.30.0.20; second at 172.30.0.21), `test/interop/Dockerfile.dnsmasq` | Serve Ze's DHCPv4 client from an implementation Ze did not write. The checkers of `dhcpv4-renew-nak-dnsmasq` and `dhcpv4-rebind-two-servers-dnsmasq` count the DHCP messages in dnsmasq's `log-dhcp` output and read Ze's eth0 addresses. The renew-NAK checker writes a reservation into the container's own `/run/dhcp-hosts` and reloads it with SIGHUP; the rebind checker stops the server that granted the lease. |
| `kea-dhcp6.conf` | Kea DHCPv6 server `kea-dhcp6` (IPv6 host 22 of the lab /64; the file's `fd00:1e:0::` selects an IPv6 lab network), `test/interop/Dockerfile.kea` | Serve Ze's DHCPv6 client from an implementation Ze did not write. The checker of `dhcpv6-client-renew-kea` reads Kea's INFO log, one exchange per distinct transaction id (`DHCP6_PACKET_RECEIVED`, `DHCP6_LEASE_ALLOC`, `DHCP6_LEASE_RENEW`, and `DHCP6_USING_SERVERID` at each start), and samples Ze's eth0 every 2 s: the leased /128 must be present with a finite `valid_lft` of at most the 40 s lease at every sample. Over three lease periods Kea must see one Solicit and one Request, between 10 and 16 RENEW exchanges that each extended the lease, and no Rebind: a client that never renews, renews without extending its deadlines (it loses the address after one lease and solicits again) or renews at every chance fails one of those bounds. The pool is one address (host `0x50`), so the lease is known before the run. The image deletes Kea's persisted server DUID on every start, while the memfile lease database stays in the container: the checker stops and starts Kea to change the server's identity under a live lease, so Ze's next RENEW names a server that no longer exists and only its REBIND is answered. Ze must hold the address for two lease periods after the stop, with no Solicit, and then renew with the restarted server. |
| `pmbmpd.conf` | pmacct `pmbmpd` (172.30.0.13:1790) | Read Ze's BMP stream with a collector Ze did not write. The file is also the selector: a scenario that carries it starts pmacct INSTEAD of Ze's own collector, because two collectors are two readings of one stream and only the third-party one is interop evidence. The typed checker greps pmacct's JSON msglog, so every needle is a field pmacct printed after decoding. |

The `ospf-te-frr` and `ospf-te-interas-frr` scenarios keep their FRR adjacency,
TED and flooded-LSA assertions and add a BGP-LS speaker sidecar. Ze exports its
native OSPF database through `bgp-ls-export`; the collector records actual
negotiated SAFI 71 UPDATEs, not injected substitutes. The checker reconstructs
the final received inventory, including withdrawals, with the native UPDATE
and BGP-LS decoders: the wire types and TLV codes come from the core codec
`internal/core/bgp/nlri/bgpls`, and the attribute decode runs through the
plugin's registered `bgp-nlri-ls` decoder. It requires exact Node and bidirectional Link auxiliary
IPv4 Router-ID TLVs for ordinary TE, and local IPv4 plus remote IPv4/IPv6 IDs
for the configured inter-AS link. It rejects missing, swapped, duplicate or
invented local IPv6 IDs. The bounded capture must finish successfully and
establish a BGP session; a historical matching UPDATE alone cannot pass.
<!-- source: internal/le/interoplab/bgp/check_ospf_bgpls.go -- checkOSPFRouterIDCollector, ospfBGPLSRouterIDVerdict, ospfBGPLSApplyUpdate -->

The inter-AS source assertion reads structured `show ospf database opaque-as`
output. It requires the configured remote AS and both ASBR IDs on one decoded
link, no Link ID, and a matching live Type-11, opaque-type-6 header. FRR's
AS-wide database must contain the same originator and LSA ID with matching
checksum and length. Neither a display label nor an address elsewhere in the
output proves that link. These source and flooding checks supplement, not
replace, the collector's final BGP-LS link and auxiliary Router-ID assertions.
<!-- source: internal/le/interoplab/bgp/check_ospf_interas.go -- checkOSPFInterASDatabase, ospfInterASSourceVerdict, ospfInterASFloodedVerdict -->

Both FRR fixtures explicitly configure `capability opaque`, separately from
TE origination, so their DDs can negotiate the opaque LSAs needed by this
proof. The native neighbor regression also keeps an O-clear peer: the
production summary filter must still establish Full without advertising
opaque headers to that peer. A capable interop fixture cannot replace that
negative control or excuse an ExStart/SeqNumberMismatch loop.
<!-- source: internal/plugins/ospf/neighbor/rfc5250_dd_capability_test.go -- TestDDNegotiatedOpaqueCapabilityControlsAdvertisedSummary -->
The capture deadline bounds reads as well as the outer loop. Expiry while
waiting for a new header is quiet completion; an unfinished header or body,
EOF, notification or send failure cannot certify a complete capture.
<!-- source: internal/le/interoplab/bgp/speaker.go -- runSpeakerSession, readSpeakerMessage, readSpeakerExact -->
If either OSPF scenario fails, its checker captures both peers' interface and
neighbor detail, Ze's packet/drop and NSM counters, and FRR's DD packet debug
log before container teardown. The original Full-adjacency failure remains
the result; diagnostic query failures are retained alongside it.
<!-- source: internal/le/interoplab/bgp/check_ospf_bgpls.go -- ospfBGPLSFailureDiagnostics -->

A scenario carrying `frr.conf` may also carry its own `daemons` file. That file
names which FRR daemons run and what each one is started with, so a scenario
needing a `bgpd` module (`-M bmp` for one that drives Ze's BMP receiver) carries
its own copy instead of adding the module to every scenario in the suite. Without
one, the shared `test/interop/daemons` is mounted.

It may also carry an `frr-image` file holding one image reference. Its FRR
containers then start from that release instead of the suite's FRR (and
`FRR_IMAGE` does not replace it), and the suite pulls each pinned release once.
A scenario needs this when the behavior under test exists only in a later FRR:
`bgp-linklocal-only-multihop-withdraw-frr` pins FRR 10.4.1, the first release
here that negotiates the Link-Local Next Hop capability (77), because without it
a different gate withholds the route and the scenario cannot go red. A file
naming no image, or two, is an error.
<!-- source: internal/le/interoplab/bgp/prepare.go -- scenarioFRRImage, scenarioFRRReference -->
<!-- source: internal/le/interoplab/bgp/run.go -- pinnedFRRImages -->

`bgp-linklocal-only-multihop-withdraw-frr` exercises the off-connected-subnet
gate on one lab network: FRR's session address is on its loopback (10.255.0.3),
outside every subnet Ze is attached to, and the checker gives Ze a host route
via FRR's adjacent interface. There is no intervening router. This proves the
gate and FRR's received withdrawal, not a physically multihop path.
`test/plugin/linklocal-only-multihop-withdraw.ci` supplies the separate routed
IPv6-hop proof through a transit namespace. Ze's link scope reads the interface
table, not the configured BGP TTL.
<!-- source: internal/le/interoplab/bgp/check_linklocal_multihop.go -- checkLinkLocalOnlyMultihopWithdraw -->

A BMP scenario with no `pmbmpd.conf` starts `le test interop-bgp bmp-collector`. Announcement and
observer process plugins use `le test interop-bgp process <scenario> <plugin>`.
These personalities are compiled into `le`; no interpreter or source mount
is present in the Ze image.
<!-- source: internal/le/interoplab/bgp/prepare.go -- sidecar startup -->
<!-- source: internal/le/interoplab/bgp/helper.go -- compiled process and BMP helpers -->
<!-- source: internal/le/interoplab/bgp/speaker.go -- compiled strict speaker -->
<!-- source: test/interop/scenarios/bgp-rfc7606-relay-shape-frr/ -- injector worked example -->
<!-- source: test/interop/scenarios/bgp-rfc7606-speaker-dup-attr/ -- speaker worked example -->
<!-- source: test/interop/scenarios/rtr-stayrtr/ -- StayRTR worked example -->
<!-- source: test/interop/scenarios/bmp-locrib-pmacct/ -- pmacct collector worked example -->

The pinned StayRTR v0.6.4 image runs with `-protocol 1 -enforce.version=true`.
Its default connection state advertises version 0 when rejecting Ze's initial
version-2 query. Enforcement sets the connection version to 1 before that
error, so Ze can reconnect at the advertised supported version.
The `rtr-stayrtr` checker then requires two IPv4 and two IPv6 VRPs and checks
the per-prefix validation answers. HTTP readiness alone proves no RTR transfer.
A changed cache command requires an image rebuild; `NO_BUILD=1` retains the
old image. For a current result, rebuild the image and run
`INTEROP_SCENARIO=rtr-stayrtr ./le test integration interop`. Read that run's VRP
and validation checks; an archived result does not establish the current outcome.
<!-- source: test/interop/Dockerfile.stayrtr -- pinned protocol and enforcement flags -->
<!-- source: internal/le/interoplab/bgp/check_rpki_reload.go -- StayRTR VRP, validation, and reload assertions -->

### Live RPKI policy reload

The `rtr-stayrtr` scenario also runs FRR as a source and BIRD as an export sink.
It changes Invalid policy from reject to accept, then restores reject through
SIGHUP. Each reload must advance the applied configuration generation once.
The same Invalid route must change eligibility, selection, and BIRD export
without a source UPDATE or route refresh. A Valid control route remains usable.
All phases require the same complete four-VRP set from StayRTR.

Continuity uses FRR's `connectionsEstablished` counter while its peer is
Established, plus Ze's connection, OPEN, UPDATE, and refresh counters.
The FRR counter identifies a session within the configured peer object's
lifetime, not across daemon restart or peer recreation. The checker does not
use FRR's derived wall-clock epoch, which can change without a reconnect.
BIRD's session identity, TCP endpoints, and Ze-side lifetime counters must also
remain unchanged. RTR reconnection is permitted; route reannouncement is not.
This carrier does not test disabling validation or rejecting a reload transaction.
<!-- source: internal/le/interoplab/bgp/check_rpki_reload.go -- checkRPKIPolicyReload -->
<!-- source: internal/le/interoplab/bgp/check_rfc.go -- queryFRRSessionGeneration, waitFRRNewSession -->

### Live BFD profile edit

The `bgp-bfd-profile-edit-live-frr` scenario is the `bfd-frr` lab plus a
reload. Ze's BGP peer opts into BFD under profile `fast` at 300ms and detect
multiplier 3; `ze-reload.conf` edits that profile to 200ms and multiplier 5 and
changes nothing else. Once FRR's BFD session is up at 300ms, the checker reloads
Ze through SIGHUP (`applyZeReload`, the same reload `rtr-stayrtr` drives) while
that session is Up. FRR must then report `remote-transmit-interval` 200,
`remote-receive-interval` 200 and `remote-detect-multiplier` 5 in
`show bfd peer <ze> json`, which it reads from Ze's packets. RFC 5880 Section
6.8.3 makes that change a Poll Sequence on an Up session.

Continuity compares four values read before and after the reload. Both BFD
discriminators (`id`, and `remote-id`, Ze's discriminator) must stay the same,
so the session was not rebuilt. The `session-down` counter of
`show bfd peer <ze> counters json` must not move, so no flap went unseen
between two samples. FRR's BGP `connectionsEstablished` must not move either.
FRR's own intervals stay at the bfdd defaults throughout, so the remote fields
are the only ones the reload can change.
<!-- source: internal/le/interoplab/bgp/check_bfd_profile_edit.go -- checkBFDProfileEditLive, requireBFDProfileEditContinuity -->
<!-- source: test/interop/scenarios/bgp-bfd-profile-edit-live-frr/ -- ze.conf, ze-reload.conf, frr.conf -->

### BFD over link-local

`bfd-link-local-frr` runs no client protocol. Ze's `bfd` root pins one
single-hop session to FRR's link-local on `eth0`, with Ze's own link-local as
the source; FRR's `bfd` block names
`peer <Ze's link-local> local-address <FRR's link-local> interface eth0`.
Without `local-address`, bfdd 10.3.1 sources its packets from `eth0`'s global
address, so neither side matches the other's packets to its session and FRR
stays down with `remote-id` 0.
Both files carry both placeholders, so both containers own their address
before the daemons start. The checker waits for FRR's
`show bfd peer <Ze's link-local> interface eth0 json` to read `up`, then for
Ze's `show bfd sessions` to hold an `up` session to FRR's address on `eth0`,
and requires the discriminators to have crossed: FRR's `remote-id` equals Ze's
`local-discriminator`, and Ze's `remote-discriminator` equals FRR's `id`.
Each value exists only on the side that chose it until a packet carries it
over, so neither clause holds on one direction of traffic.

The scenario is acceptance evidence. RFC 5881 Section 3 makes both sides
Active, so once FRR answers, the session also converges through Your
Discriminator, and the zone Ze reconciles on a first packet is not what the
scenario can tell apart. The send side cannot fail here either: every session
of the loop names `eth0`, so the loop's socket is bound to that device
(`resolveLoopDevices`) and the kernel puts the packet on `eth0` with no zone
and no ifindex. Both reverts stayed Up in a smoke run. The discrimination is
`test/bfd/bfd-first-packet-link-local.ci` in the QEMU guest, and
`bfd-link-local-two-links-frr` against FRR.

`bfd-link-local-two-links-frr` puts two links between Ze and FRR and ONE
address pair on both. Each container's prestart builds `eth1` (VLAN 101) and
`eth2` (VLAN 102) over `eth0`, and gives the container's link-local to both, so
the address cannot tell the two sessions apart and only the link can. Ze names
one session per link; FRR one peer per link. Two things make it discriminate:

- **Send.** The sessions name two interfaces, so the loop's socket is bound to
  no device, and a packet reaches its link only through the zone `sendZone`
  puts on the destination or the ifindex `pinsToLink` asks for. Without either,
  the kernel refuses the write (`sendmsg: invalid argument`) and FRR hears
  nothing.
- **Receive.** Ze's prestart installs an nft output rule in table
  `inet bfdfirst` that drops every BFD Control packet Ze sends with Your
  Discriminator zero. FRR then learns Ze's discriminator only from a packet Ze
  sends after selecting FRR's first packet by address and link, so without the
  zone reconciliation (`reconcileZone`) neither side leaves Down. Ze logs each
  dropped send as `bfd send failed ... operation not permitted`, which is the
  rule working.

The checker reads the links from Ze's own `show bfd sessions` (the interfaces of
its sessions to FRR's link-local, at least two), then runs the one-link check
on each: FRR's peer on that link Up, Ze's session on that link Up, and the
discriminators crossed. A smoke on colima (not evidence) held both Up with the
code as it is, and both Down with `reconcileZone` returning its key unchanged
and with `sendZone` returning `""` and `pinsToLink` returning false.
<!-- source: internal/le/interoplab/bgp/check_bfd_link_local.go -- checkBFDLinkLocal, checkBFDLinkLocalTwoLinks, checkBFDLinkLocalOn, zeLinkLocalLinks, zeLinkLocalSession, requireCrossedDiscriminators -->
<!-- source: internal/le/interoplab/bgp/prepare.go -- scenarioPrestarts, renderPrestarts, assignLinkLocal -->
<!-- source: test/interop/scenarios/bfd-link-local-frr/ -- ze.conf, frr.conf -->
<!-- source: test/interop/scenarios/bfd-link-local-two-links-frr/ -- ze.conf, frr.conf -->
<!-- source: internal/le/interoplab/bgp/prepare_bfd_link_local.go -- bfdLinkLocalTwoLinksZePrestart, bfdLinkLocalTwoLinksFRRPrestart -->

### Live attach and BFD opt-in without a session reset

Three scenarios change a running FRR peer's Ze config by SIGHUP and require
FRR's BGP `connectionsEstablished` to stay where it was before the first reload,
through every reload except the BFD opt-in removal.

| Scenario | Reloads | What FRR must show |
|----------|---------|--------------------|
| `bgp-attach-receive-only-live-frr` | `ze-reload.conf` attaches `adj-rib-in` to the peer, receive-only | exactly one more `messageStats.routeRefreshRecv` (one negotiated family, RFC 2918 Section 4), and Ze's `show bgp adj-rib-in` then holds the route FRR announced before the reload |
| `bgp-bfd-enable-live-frr` | `ze-reload.conf` adds the peer's non-strict BFD opt-in, `ze-reload-edit.conf` edits its profile to 200ms and multiplier 5, `ze.conf` removes the opt-in | BFD Up at 300ms after the add; the same discriminators and no `session-down` at 200ms after the edit; BFD Down with `remote-diagnostic` `administratively down` after the removal |
| `bgp-bfd-dynamic-enable-live-frr` | `ze-reload.conf` adds the opt-in to the template of the group FRR joins as a dynamic peer, `ze.conf` removes it | BFD Up at 300ms after the add, Down with `remote-diagnostic` `administratively down` after the removal |

The attach checker first requires that `adj-rib-in` holds nothing for FRR, so
the route it finds afterwards can only have come from the catch-up refresh.
The BFD configs keep the `bfd` root in every step, because an opt-in names a
profile of the bfd plugin.

The removal step asserts what Ze sends and stops there. Ze releases the BFD
session with AdminDown, diagnostic 7 (RFC 5880 Section 6.8.16), and FRR 10.3.1
then resets BGP with Cease/BFD Down, although RFC 5882 Section 4.2 says
"clients SHOULD NOT take any control protocol action" on a remote AdminDown.
That reset is a recorded peer deviation, not a Ze failure, so the checker does
not require BGP to survive the removal.

A scenario that reloads more than once names each further config
`ze-reload-<step>.conf`. The harness mounts every such file in the ze container
at `/etc/ze/<file name>`, beside `ze-reload.conf` at `/etc/ze/reload.conf`, and
`ze.conf` stays at `/etc/ze/bgp.conf`, so a checker reloads back to the start
with `applyZeReload` on that path.
<!-- source: internal/le/interoplab/bgp/check_live_keep_session.go -- checkAttachReceiveOnlyLive, checkBFDEnableLive, checkBFDDynamicEnableLive, requireOneRefreshSameSession, removeAndSignalAdminDown -->
<!-- source: internal/le/interoplab/bgp/prepare.go -- scenarioPeers -->
<!-- source: internal/le/interoplab/bgp/names.go -- zeStepReloadConfigPattern, zeMountedStepReloadConfig -->

### RPKI route retention

`INTEROP_SCENARIO=rpki-frr ./le test integration interop` runs FRR as the BGP
source for `9.43.0.0/24`, `10.43.0.0/24`, and `11.43.0.0/24`. The checker
requires an established session and all source routes in FRR. Ze must retain
all received routes in Adj-RIB-In: Valid (`1`) and NotFound (`2`) are eligible;
Invalid (`3`) has `ineligible: true`. Missing routes or missing eligibility
fields fail the observation. The process observer and native checker use the
same structural predicate and save the observed route objects in
`/tmp/rpki-check.json` inside the Ze container.

This scenario exercises FRR-to-Ze BGP reception with RPKI eligibility decisions.
It uses the same-repository `le test rpki` RTR cache mock and does not establish
independent RTR interoperability. The `rtr-stayrtr` scenario uses StayRTR for
that purpose. The retention observation does not measure downstream export:
FRR originates these routes, and there is no separate receiving peer.
<!-- source: internal/le/interoplab/bgp/helper.go -- requireRPKIObservation, requireRPKIResult -->
<!-- source: internal/le/interoplab/bgp/check_extras.go -- scenarioRPKIFRR -->

The functional `rpki-revalidate-late-sync` carrier uses an observer-controlled
RTR barrier. The mock waits for `rpki-sync.release` in the test's private working
directory. Only after the observer reads a retained, eligible NotFound route and
an empty VRP set does it create that file. The observer then requires the same
received attribute, next-hop, and NLRI bytes with Invalid/ineligible state.
Scheduling delays cannot let the cache synchronize before the initial observation.
<!-- source: internal/test/mock/rtr/rtr.go -- rtrMockWaitRelease -->
<!-- source: internal/test/fixture/plugin_fixture_15_rpki.go -- plugin15RPKILateSync -->

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
| A test whose data reaches the peer by a DIFFERENT path than the one changed | The unchanged path still delivers | Trace which code path actually produces the asserted bytes. A scenario proving two peers are NOT merged is this trap when they are already kept apart by another fact: `local-as-replace-as-partition` passed with the group key's prepend zeroed until BIRD was made to advertise the RFC 8654 capability FRR advertises by default, because the two sessions differed in `announceFacts.extended`. Log each peer's key in the red build |
| An assertion whose clauses are all satisfied by ONE stimulus | Each clause reads as an independent observation, and they are one observation written twice | Name the single event that satisfies every clause. Then ask which clause a peer that did nothing would still satisfy |
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate (ExtendedMessageSend follows the peer's advertisement) -->
<!-- source: test/interop/scenarios/local-as-replace-as-partition/bird.conf -- enable extended messages -->

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

### IPsec address movement

`mobike-initiator` moves Ze; `mobike-responder` moves strongSwan. Each native
checker adds a new outer address, selects it as the route's source, then removes
the old address without reloading either daemon or initiating another IKE SA.

The checker requires the same IKE and Child SPIs before and after movement, the
new UDP 4500 endpoints, unchanged inner selectors, and exactly the original two
directed XFRM states on each peer. It sends lossless pings in both directions and
requires all four directed ESP byte and packet counters to advance. A replacement
tunnel, a stale direction, or a one-way success does not satisfy the scenario.

The Docker host must support `XFRM_MSG_MIGRATE_STATE`. A kernel without atomic live
ESP migration cannot run this scenario: Ze does not negotiate MOBIKE there. The
checker does not convert that missing prerequisite into a pass, and the suite
does not reach it on such a kernel: the Docker host kernel check refuses the
run first, naming `ipsec-mobike (CONFIG_XFRM_MIGRATE)`.

<!-- source: internal/le/interoplab/ipsec/mobike.go -- checkMOBIKEInitiator, checkMOBIKEResponder -->

### The IPsec NAT box

A scenario that carries `nat.conf` starts a third container, `nat`, at
172.28.0.5. It is keyed on that file exactly as strongSwan is keyed on
`swanctl.conf` and FRR on `frr.conf`, and it is started FIRST so its addresses
answer ARP before either daemon sends a datagram.

It exists because RFC 7296 Section 2.23.1 is only reachable across a REAL
translation. `natt-transport-inner-checksum` and `natt-tunnel-inner-checksum`
reach the UDP-encapsulated path through strongSwan's `encap = yes`, which fakes
the NAT_DETECTION_SOURCE_IP hash with no middlebox on the path. The pre-NAT
address and the observed address are therefore equal in those two, every
Section 2.23.1 substitution is the identity, and its absence cannot show. That is
the fourth vacuity trap above: the asserted bytes reach the peer by a path the
mechanism under test does not touch.

`nat.conf` is one translation per line, `<real> <public>`. The box gives itself
each public address as a SECONDARY address on `eth0`, then installs one DNAT rule
in PREROUTING and one SNAT rule in POSTROUTING per line. Three consequences are
load-bearing:

- **No peer needs a route.** Both public addresses sit inside the lab bridge's own
  prefix, so a peer resolves them by ARP and the box answers. A NAT on its own
  segment would need a second Docker network, and `interoplab.ScenarioPlan` carries
  one `NetworkSpec` that the BGP suite shares.
- **Both addresses of a crossing datagram are rewritten,** which is the two-NAT
  figure of Section 2.23.1 drawn with one box, and it is what makes all four
  NAT_DETECTION comparisons mismatch.
- **The rules carry no port and no protocol.** IKE floats from UDP 500 to UDP 4500
  mid exchange, so a port-scoped rule would translate the handshake and drop the
  ESP that follows.

Three scenarios use it, and they differ in one variable each. The two transport
scenarios differ in ROLE, because the substitution has two producers and a single
scenario cannot fail on one of them. `real-nat-tunnel-control` differs from them in
MODE, and it does two jobs: it proves the substitution stays out of tunnel mode, and
it makes a red transport scenario readable, because a broken topology reds it too.
Its selectors are INNER addresses the box never sees, which is the one selector pair
in this lab a translation cannot move.

#### The port column and the rebind step

A row may carry a third column, `<real> <public> <port>`: the public port the
box maps that peer's UDP 4500 to. 0, 500 and 4500 are refused, because a mapping
to any of them could not tell an encapsulation port taken from the IKE SA from a
constant (RFC 3948 Section 2.1: "the Source Port and Destination Port MUST be the
same as that used by IKE traffic,"). For such a row `natSetupScript` emits a port
SNAT for UDP source port 4500 AHEAD of the row's address SNAT, and no DNAT: the
peer initiates and conntrack reverses the replies. The other peer keeps its own
address, so it is not behind a NAT. That is the precondition of the endpoint
adoption the rebind step drives, and it is why the peer behind the port
translation reaches the other one by a host route via its own public address,
which the checker installs before it initiates (`natPortEstablish`).

The rebind step (`natRebind`) replaces the port rule with one to a second public
port and deletes the box's UDP conntrack entries with `conntrack -D -p udp`
(`conntrack-tools` in `Dockerfile.nat`). The flush is needed because a NAT rule
applies to a flow's first packet only, and the peer's keepalives and DPD keep the
old entry alive. The step fails when conntrack prints no deleted count, and when
`conntrack -L` afterwards still holds a flow from strongSwan's UDP 4500 mapped to
the first public port (`conntrackDeletedCount`, `conntrackStaleMapping`). A flush
that did nothing then fails at once, rather than as a 90 s wait for a move that
cannot come. The peer's next DPD then reaches Ze from the new port, Ze adopts it,
and the checker waits for exactly one of Ze's four rebind log lines.

The two rebind scenarios set `mobike = no` in `swanctl.conf`. strongSwan offers
MOBIKE by default. When MOBIKE is negotiated, Ze correctly refuses to adopt the new
source of an authenticated IKE packet, because RFC 4555 Section 3.8 says "The host
not behind a NAT MUST NOT use these dynamic updates for IKEv2 packets". The peer
must send UPDATE_SA_ADDRESSES instead. Ze logs that refusal at Debug, so a
scenario that loses the setting shows the reason in Ze's log.

| Scenario | Proves |
|----------|--------|
| `real-nat-port-translated-ze-responder` | tunnel mode: outbound encap dport 14500, traffic both ways; after the remap the row 1 "migrated" line, dport 14501 on the SAME SPI, traffic both ways |
| `real-nat-port-translated-transport-ze-responder` | transport mode: after the remap the row 2 "rekeyed" line with `reason="transport mode"`, dport 14501 on a NEW SPI, traffic both ways |
| `real-nat-port-translated-eap-ze-responder` | the EAP-TLS responder's first Child SA encapsulates to dport 14500 |
| `responder-eap-tls12` (no NAT) | a strongSwan EAP-TLS client pinned to TLS 1.2 authenticates and traffic crosses |

All four have run in the nightly ipsec-interop suite. In the 2026-10-17 run the
two rebind scenarios and `responder-eap-tls12` were red, and
`real-nat-port-translated-eap-ze-responder` passed, possibly vacuously (see the
next sentence). On 2026-10-17 the two rebind scenarios got `mobike = no`. In the same change,
`waitZeOutboundEncap` stopped accepting an outbound state at any port: it had
read a drained `textbuf.Buffer` inside its poll and so matched "". No run since
these fixes is recorded, and `responder-eap-tls12` waits on an owner decision.
Each scenario still owes a green run and its revert, rebuild and red walk.

<!-- source: internal/le/interoplab/ipsec/nat.go -- readNATConfig, natSetupScript, natPortRule -->
<!-- source: internal/le/interoplab/ipsec/natport.go -- natPortEstablish, natRebind, conntrackDeletedCount, conntrackStaleMapping, waitRebindLine, waitZeOutboundEncap -->
<!-- source: internal/component/ike/engine/sa.go -- adoptAuthenticatedEndpoint, the MOBIKE refusal -->

Two more scenarios reuse the tunnel-control topology to drive `show mtu` across a
real NAT. `mtu-tunnel-sizing-strongswan` binds the tunnel to an xfrm interface,
clamps the route the box forwards over to 1400 (`ip route replace ... mtu 1400`,
which Linux honors when it forwards), and requires the measured path, the derived
ceiling over the negotiated transform behind UDP encapsulation, the recommended
interface MTU and its command; an iputils `ping -M do` at the old size MUST lose
every packet and one at the recommended size MUST lose none. `mtu-nat-installed-endpoint`
requires the peer measurement and the tunnel row to target the box's face, the
address the Child SA carries ESP to, and never the peer's real address, where no
ESP arrives. The ze lab image installs `iputils` for `-M do`, so every IPsec
scenario's ping is the iputils one.

`ike-padded-probe-strongswan` reuses the sizing topology to drive the padded IKE
path probe (`docs/architecture/diagnostics/path-mtu.md`, "The IKE prober") where
ICMP cannot see the clamp. The checker keeps the 1400 clamp for UDP and ESP, routes
ICMP past it through a policy-routing table (`exemptICMPFromClamp`, so an echo
crosses at 1500), and drops every Fragmentation Needed the box would send Ze
(`dropTooBigToward`). The ICMP search then measures 1500, the padded INFORMATIONAL
asks 1500 on UDP/4500, its DF copy vanishes, its DF-clear retransmission crosses
the box fragmented and strongSwan answers it, and the descent settles on 1400 with
`prober: ike`. Each `show mtu` runs DETACHED (`zeShowMTUDetached`): a probe the
peer never answers holds the request window for 30 s before the SA is deemed
failed, longer than one docker exec is given, so the shell writes the document to
a file and the checker polls for the marker. A second run races `swanctl --rekey`,
and a third cuts strongSwan's reassembly memory (`dropFragmentsAtPeer`,
`net.ipv4.ipfrag_high_thresh`) so no fragmented datagram is ever reassembled: an
`iptables -f` rule matches nothing on either container, because conntrack
defragments before any chain runs. The SA then fails exactly once and the row
names `sa-failed` and the size.

<!-- source: internal/le/interoplab/ipsec/checkers.go -- checkMTUTunnelSizingStrongSwan, checkMTUNATInstalledEndpoint, checkIKEPaddedProbeStrongSwan, mtuClampedPath -->
<!-- source: internal/le/interoplab/ipsec/helpers.go -- exemptICMPFromClamp, dropTooBigToward, dropFragmentsAtPeer, zeShowMTUDetached -->
<!-- source: test/interop-ipsec/Dockerfile.ze -- iputils -->

<!-- source: internal/le/interoplab/ipsec/nat.go -- readNATConfig, natSetupScript -->
<!-- source: internal/le/interoplab/ipsec/checkers.go -- checkRealNATTransport, checkRealNATTunnelControl -->

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

### AES CCM on the strongSwan peer

`ike-aes-ccm16` gives the IKE SA's Encrypted payload the transform RFC 5282
Section 7.2 numbers 16, AES CCM with a 16-octet ICV, and takes the Child SA to
AES GCM because Ze refuses AES CCM for ESP at config parse
(`ipsec.EncryptionImplementedESP`). The checker reads charon's own
`selected proposal: IKE:AES_CCM_16_256/` line, then Ze's `encryption` field, then
ESP in both directions, so the agreement on the Transform ID and the verification
of the encrypted ICV are separate observations.

The lab image carries that transform although `/usr/lib/ipsec/plugins` holds no
`libstrongswan-ccm.so`. Alpine 3.21 builds strongSwan 5.9.14 with no
`--enable-ccm`, so the standalone ccm plugin is absent, and charon's openssl
plugin registers the algorithm instead: `swanctl --list-algs` in
`ze-ipsec-strongswan` answers `AES_CCM_16[openssl]`, `AES_CCM_12[openssl]` and
`AES_CCM_8[openssl]` (measured 2026-09-14). The absent FILE is not an absent
capability, and a source build of the ccm plugin would add a second provider of
one algorithm and nothing else.
<!-- source: test/interop-ipsec/scenarios/ike-aes-ccm16/ -- the AES CCM fixtures -->
<!-- source: internal/le/interoplab/ipsec/checkers.go -- checkIKEAESCCM16 -->

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
and runs nothing privileged. `TestSuiteNeedsNoKernelModule` holds that. The
Docker host kernel check still applies: it is a rule about the host, so a
kernel without PPPoL2TP refuses this suite as it refuses every other.

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

### The FreeRADIUS subscriber suite

`./le test integration interop-radius-subscriber` puts FreeRADIUS 3.2.7, the
release the admin-login suite pins, in front of ze's subscriber RADIUS surface:
the accountant and the CoA and Disconnect listener of
`internal/component/l2tp/plugins/authradius`. Both act only on a kernel L2TP
session, which no fixture can stand in for, so the lab carries its own LAC
(xl2tpd and pppd), and ze and the LAC hold `NET_ADMIN` and `/dev/ppp`. That is
why it is a suite of its own: the admin-login suite promises no capability at
all, and the L2TP lab's RADIUS peer is a mock ze wrote. FreeRADIUS takes ze's
Access-Requests and Accounting-Requests on one `auth+acct` listener, because a
ze server entry carries one port, writes every accounting record to its
`ze_detail` file, and its `radclient` is the Dynamic Authorization Client.

| Scenario | What it proves |
|----------|----------------|
| `radius-coa-duplicate-freeradius` | RFC 5176 Section 2.3 duplicate detection. With ze paused, `radclient` sends one Disconnect-Request and retransmits it on its own socket: ze answers the copies from the cache, `radclient` gets a Disconnect-ACK, and ze logs one teardown. On a second session, two `radclient` processes send one request file with the same Identifier from two ports: the second is processed afresh and answered Disconnect-NAK. A tcpdump capture in ze's container confirms the retransmitted copies share port, Identifier and Authenticator, and the two clients' requests differ in the port alone |
| `radius-acct-restart-freeradius` | RFC 2866 Section 5.5 across a real restart. The Accounting-Start before ze restarts and the one after carry ids of the form tunnel-session-tag-counter, the two boot tags differ, and each Stop carries its Start's id, all read from FreeRADIUS's detail file |

Both scenarios were added on 2026-10-14 and have not run yet: this machine's
Docker kernel refuses the lab, so neither has a recorded pass or a recorded red.

<!-- source: internal/le/interoplab/radiussubscriber/radiussubscriber.go -- the suite, its peers, its grants and its preflight -->
<!-- source: internal/le/interoplab/radiussubscriber/checkers.go -- checkCoADuplicate, checkAcctRestart, judgeCapture -->
<!-- source: test/interop-radius-subscriber/mods-ze-detail -- the detail module the server's record comes from -->

### The freeRouter RSVP-TE suite

`internal/le/interoplab/rsvpte/` puts up to four nodes on one Docker segment,
`172.29.81.0/24`, in four roles: `ingress` (host 2), `transit` (3), `egress`
(4) and `relay` (5). A scenario directory decides which implementation fills
each role by the files it carries: `<role>.conf` makes the role a Ze node
answering on the container address `.<host>`, and
`<role>-hw.txt` with `<role>-sw.txt` makes it a freeRouter node answering on
its own address `.<10+host>`. A role with neither file is absent. Nodes start
downstream first, so the first PATH meets nodes that already listen. The
addresses are fixed because the configurations in each scenario directory name
them. Two shapes are used: freeRouter, Ze, freeRouter, where freeRouter
originates and Ze relays; and Ze, freeRouter, Ze (and Ze, freeRouter, Ze, Ze),
where Ze originates PATH, ResvErr, ResvTear, PathErr and strict hops and
freeRouter is the independent implementation that parses, relays and
re-encodes them. A Ze head-end tunnel in this suite requests `fast-reroute`,
because freeRouter's `packRsvp.parseDatPatReq` refuses a PATH without
SESSION_ATTRIBUTE and Ze emits that object only for a protected tunnel and a
bypass. A third shape puts Ze at the point of local repair: Ze ingress, Ze
PLR, freeRouter relay on the bypass, freeRouter egress as the merge point. Each freeRouter owns its own IPv4 stack
and MAC: its container command (`freeRtrScript`) makes the container's `eth0`
promiscuous and joins it to the jar through the upstream `rawInt.bin`, so the
suite needs Docker alone, never host root, a TAP device or a network namespace,
and no container runs privileged: a freeRouter holds `NET_ADMIN` and `NET_RAW`.
A Ze node holds `NET_ADMIN` to program MPLS labels and VLANs. Its container
command (`zeNodeScript`) creates the VLANs and adds the routes the role's
optional `<role>-net.txt` declares, one `vlan <name> id <1-4094> address
<address/length>` or `route <prefix> via <gateway>` per line, and a line of any
other shape refuses the run. The command writes its MPLS sysctls at run time
(one names a VLAN it creates first), so it runs under `ze-lab-net-sysctl` with
`/proc/sys` writable. No scenario carries a shell script: the commands are
built in Go from those typed lines. The preflight
reads `/proc/sys/net/mpls/platform_labels` with no grant, loads no module, and
refuses a host kernel without MPLS routing with the host command that loads it.

Every assertion reads what a peer received. Every container, Ze or freeRouter,
runs `tcpdump -vvv` on its `eth0` into `/run/fr/rsvp.txt`, RSVP and every
MPLS-labelled frame, because a PATH carried through a bypass LSP is labelled.
The checker parses that text: a message Ze
logs as sent counts for nothing until the peer's capture holds it.

| Scenario | What the peers observe |
|----------|------------------------|
| `transit-loose-ero-expansion` | freeRouter ingress, Ze transit, freeRouter egress. The ingress names Ze and the egress loopback as loose hops and prepends its own next hop, Ze, as a strict subobject (`ipFwdTab.fillRsvpFrst`), so it signals `[Ze strict, Ze loose, egress loopback loose]`. Ze's native route to the loopback runs through `.14`, which no subobject names, so the PATH the egress captures carries `.14` ahead of the still-loose loopback (RFC 3209 Section 4.3.4.1 steps 5 and 6). The ingress captures Ze's RESV with a label, and Ze's MPLS table holds a swap via `.14` |
| `plr-backup-path-to-egress-merge-point` | Ze ingress, Ze PLR, freeRouter relay `.15`, freeRouter egress `.14`. The ingress asks facility protection for a tunnel whose protected hop is the PLR's `prot0`, VLAN 100 to the egress at `10.0.14.14`; the PLR's configured bypass runs untagged through the relay and merges at that egress. Before the fault the egress holds no backup PATH. The checker takes `prot0` down, and the egress captures, through the bypass, a PATH whose sender and RSVP_HOP are the PLR and whose ERO is `10.0.14.14` alone; the PLR captures at least two labelled RESVs from the egress naming its own sender and answers none with a ResvErr (RFC 4090 Sections 6.1 and 6.4). The egress answers from its `eth0` address `.14`, not from `10.0.14.14`, and Ze accepts an alternate merge-point source only when a live native IGP database attributes it to the same node, so the PLR and the egress run OSPF on `eth0` (freeRouter router-id `10.0.14.14`) and the checker waits for a Full adjacency, read through `show ospf neighbor` on the PLR, and then for the egress's router-LSA to carry its point-to-point link (48 octets or more in `show ospf database router`), before it takes `prot0` down: the egress re-originates that LSA after the adjacency forms, and until it arrives nothing ties `.14` to `10.0.14.14`. After the cut the ingress keeps refreshing its PATH, whose ERO still names `10.0.14.14` strict across the failed link; the PLR relays it through the bypass, and the checker fails on a Routing Problem PathErr for the protected LSP-ID 1 within three refreshes (RFC 4090 Section 6.4.3). The Notify `Tunnel locally repaired` for LSP-ID 1 and a Routing Problem for a new LSP-ID the ingress signals over the same ERO are correct and do not count. Because a PLR that dropped the refresh in silence would send no PathErr either, the checker then waits until the protected PSB's lifetime has passed since the cut (10.5 seconds, RFC 2205 Section 3.7 with K = 3 and the ingress's 2-second refresh) and requires a further backup PATH at the egress and a further labelled RESV from the merge point at the PLR. freeRouter is the merge point only as an egress: it keys the backup PATH by sender and LSP-ID as a new LSP, and has no transit merge |
| `transit-strict-hop-forwarded` | Ze ingress, freeRouter relay `.15`, Ze transit, freeRouter egress `.14`. The Ze ingress names every hop strict. The PATH the egress captures from Ze carries the ERO shortened to strict `.14` alone, the Ze ingress captures a labelled RESV relayed by freeRouter, and the Ze transit holds a swap via `.14` |
| `transit-strict-hop-outside-refused` | Ze ingress, freeRouter relay, Ze transit. The last strict hop's native route at the transit runs through a node outside both abstract nodes, so the transit sends PathErr Routing Problem / Bad strict node (24/2) and never forwards the PATH (RFC 3209 Section 4.3.3.1). The Ze ingress captures that PathErr as freeRouter relays it, naming the transit as the error node |
| `ingress-resv-error-relayed` | Ze ingress, freeRouter relay, Ze egress. The ingress interface reserves less than the tunnel asks, so the ingress refuses the RESV freeRouter relays and sends a ResvErr naming itself, Error Code 1 Admission Control failure, value 2. The Ze egress captures that ResvErr from freeRouter with the error node, code and value intact (RFC 2205 Sections 2.5 and 3.1.8) |
| `transit-resv-tear-relayed` | Ze ingress, freeRouter relay, Ze transit, Ze egress. Once the LSP is up the egress is frozen with `SIGSTOP`, so the transit's reservation times out while its path state is refreshed; the transit sends a ResvTear upstream, and the Ze ingress captures it as freeRouter relays it (RFC 2205 Section 3.1.6) |
| `transit-resv-increase-refused-in-place` | freeRouter ingress, Ze transit, patched freeRouter egress. The ingress tunnel signals 10 Mbit/s (`bandwidth 10000`) and the transit's `eth0` reserves 100 Mbit/s. The egress's second RESV, answering the ingress's first PATH refresh (freeRouter refreshes every 120 s), raises its reservation in place to 1 Gbit/s, same session, sender and LSP-ID. The transit captures that RESV, and the egress captures Ze's ResvErr naming the transit, Error Code 1 value 2, ERROR_SPEC flags `0x01` (InPlace). The transit's swap stays installed and the ingress never receives the raised rate (RFC 2205 Section 3.1.8) |
| `transit-ff-resv-unknown-sender` | freeRouter ingress, Ze transit, patched freeRouter egress. Every RESV the egress sends is fixed-filter, with a second flow descriptor naming LSP-ID+1 of the same sender, which never signalled a PATH. The egress captures one ResvErr from the transit naming only that LSP-ID, Error Code 4 No sender information, and no ResvErr naming the real sender. The ingress receives a labelled RESV naming only the real sender, and the transit holds its swap (RFC 2205 Section 3.1.8) |
| `egress-implicit-null-penultimate-pop` | freeRouter ingress, Ze transit, patched freeRouter egress. Every RESV the egress sends distributes Implicit NULL, label 3, so Ze is the penultimate hop. The transit captures that RESV, the ingress receives Ze's RESV carrying a label Ze allocated, not 3, and Ze's MPLS table pops that label toward `.14` with no outgoing label: no `as to` (RFC 3032 Section 2.1) |
| `transit-resv-state-expires` | freeRouter ingress, Ze transit, freeRouter egress. Once the LSP is up and Ze holds its swap via `.14`, the checker freezes the egress's Java process with `SIGSTOP`, so no RESV refreshes Ze's reservation while the ingress keeps refreshing the path. The ingress captures a ResvTear from Ze, none of which existed before the freeze, and Ze's MPLS table drops the swap via `.14` (RFC 2205 Sections 2.3 and 3.1.6). The freeRouter egress advertises its fixed 120-second refresh period in the RESV's TIME_VALUES, so Ze, with refresh-multiplier 3, times the reservation out about six minutes after the freeze, and the checker waits that long |

The checker compares the ERROR_SPEC code and value as the numbers tcpdump
prints in parentheses, because tcpdump names no code for every value: it prints
Admission Control failure as `unknown (1)`.

The pinned freeRouter originates only PATH, PathTear and RESV. It relays
PathErr, ResvErr and ResvTear but never originates them. It encodes its
configured ERO hops as loose (`clntMplsTeP2p.workDoer`,
`ipFwdTab.fillRsvpPack`) and prepends one strict subobject for its own next
hop unless the first hop is already strict (`ipFwdTab.fillRsvpFrst`), and it
routes each hop without reading the strict bit (`rtrRsvpIface.getHop`), so it
does not enforce strict hops. It signals one fixed bandwidth for the life of
an LSP, and sends only shared-explicit RESVs naming its own sender. In every scenario
where Ze originates a message, freeRouter is the independent implementation
that parses it, keeps the state it needs to relay it, and re-encodes it toward
the next Ze node: the evidence is that freeRouter accepts and relays what Ze
originates, not that freeRouter enforces strict hops or originates ResvErr,
ResvTear or PathErr itself.

The image is built with one patch, `test/interop-rsvpte/freertr/ze-interop-resv.patch`,
because the pinned jar can neither raise a reservation in place, name a
sender it has no path for, nor distribute Implicit NULL as an egress
(`rtrRsvpIface` always sends the label it allocated). The patch adds three knobs to the RESV a freeRouter
egress originates, each off unless its environment variable is set, so every
scenario that sets neither runs the upstream behavior. A scenario sets them in
`<role>-env.txt`, one `NAME=value` per line, which the lab passes to that
freeRouter container and refuses when a line is not `NAME=value`.
`FREERTR_ZE_RESV_RATE_AT=n:rate` sends only the n-th RESV with its FLOWSPEC
rate (bytes per second) raised; `FREERTR_ZE_RESV_FF_EXTRA_SENDER=1` makes every
RESV fixed-filter and appends a second flow descriptor (FLOWSPEC, FILTER_SPEC
with LSP-ID+1, LABEL); `FREERTR_ZE_RESV_IMPLICIT_NULL=1` puts label 3 in the
LABEL object of every RESV in place of the allocated label.

Two scenarios depend on objects RFC 2205 makes optional and freeRouter's parser
requires. `packRsvp.parseDatPatErr` refuses a PathErr without an ADSPEC, which
`transit-strict-hop-outside-refused` relies on, and `packRsvp.parseDatResTer`
refuses a ResvTear without a FLOWSPEC, which `transit-resv-tear-relayed` relies
on. The PathErr sender descriptor is "<SENDER_TEMPLATE> <SENDER_TSPEC>
[ <ADSPEC> ]" (Section 3.1.3), and "FLOWSPEC objects in the flow descriptor list
of a ResvTear message will be ignored and may be omitted" (Section 3.1.6). Ze
sends both objects (`buildPathErr`, `buildReservationControl`), so both
scenarios pass against the unpatched image. Before Ze carried them, freeRouter's
capture held Ze's message and freeRouter never relayed it.

<!-- source: internal/le/interoplab/rsvpte/rsvpte.go -- the suite, its topology and its MPLS preflight -->
<!-- source: internal/le/interoplab/rsvpte/checkers.go -- every observation, read from a peer's capture -->
<!-- source: internal/le/interoplab/rsvpte/zenode.go -- freeRtrScript, zeNodeScript, readZeNodeNet -->
<!-- source: test/interop-rsvpte/freertr/ze-interop-resv.patch -- the three test-only RESV knobs -->
<!-- source: internal/le/interoplab/rsvpte/rsvpte.go -- freeRtrEnvironment reads <role>-env.txt -->

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

### Raw frame injection from inside a peer container

A checker that must put a hand-built Ethernet frame on a peer's wire, as that
peer, sends it with `interoplab.SendFrameInContainer`. The helper writes the
frame as a one-record pcap inside the named container and runs `tcpreplay`
there through `docker exec`, so the image MUST carry `tcpreplay`. Ze's BGP lab
image does, for the `isis-purge-reorig-frr` own-LSP purge, and so does the
PPPoE client image, for `pppoe-padr-replay`, `pppoe-pap-ze-ac` and the IPv6CP
replays.

The send runs in the container, never from the checker process. On a rootful
Docker daemon a container's network namespace belongs to root: an unprivileged
checker cannot open `/proc/<pid>/ns/net`, because the kernel's ptrace access
check refuses another user's process, and could not `setns` into it without
`CAP_SYS_ADMIN`. `docker exec` already runs inside the namespace and needs
neither.
<!-- source: internal/le/interoplab/sendframe.go -- SendFrameInContainer -->

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

Rendered configurations stay under `tmp/interop-rendered` in the checkout.
Docker must share the checkout with its Linux VM. An unshared system-temp path
gives the container a directory instead of the configuration file.
IPsec and L2TP create private directories there even without an AI session ID.
<!-- source: internal/le/interoplab/lab.go -- RenderedConfigDirectory -->
<!-- source: internal/le/interoplab/ipsec/ipsec.go -- prepareScenario -->
<!-- source: internal/le/interoplab/l2tp/l2tp.go -- renderInitiatorConfig -->

**A Ze helper never converts a failed query into a plausible number.**
`Ze.rib_count` raises when the command fails or answers without a `routes-in`
field, because 0 is a legitimate RIB size and a failed query is not
(`ai/rules/evidence.md`). It returned 0 on failure until 2026-08-07 and three
separate faults hid behind that one number for three days
(`spec-fixit-test-harness-fail-open-guards`, guard 3). Write new Ze
helpers the same way.

**A checker asserts on the STRUCTURED answer, never on the rendered text.** The
IPsec suite asks with `show vpn ipsec sa | json` and decodes the list of SA
records (`zeIKESAs`, `internal/le/interoplab/ipsec/helpers.go`). The text
rendering is a table, and its column order follows the field names, so a
line-anchored `<field> <value>` regex over that table matches by accident. On
2026-09-06 one new field, `behind-nat`, took first place from `child-sa` in that
order. Every nested Child SA key moved off the line start, and
`child-rekey-narrowing`, `peer-reload-narrowing` and
`initiator-rekey-answer-narrows` went red with no daemon behavior changed.
<!-- source: internal/le/interoplab/ipsec/helpers.go -- zeIKESAs, assertNATVerdict, assertZeSelectors -->

The VPN withdrawal checker receives its phase from private named constants,
not from FRR output. An unknown internal phase is a `BUG` assertion; malformed
or unsupported external table data remains an ordinary rejected observation.
<!-- source: internal/le/interoplab/bgp/check_vpn_withdraw.go -- requireVPNWithdrawTable -->

**A number two readers print in two bases is normalized to one TYPE, and the
decode is typed.** The `dataplane-readback` scenario joins the SPI set
`show vpn ipsec dataplane sa | json` answers against the set `ip xfrm state`
prints in the same container. iproute2 writes an SPI as `0xc1a2b3c4` and Ze
answers a JSON number, so a string comparison is false for every SPI. The
checker decodes the Ze answer into a struct whose `spi` field is a `uint32`, and
parses the printed form to the same type. Decoding into `map[string]any` would
route the number through `float64`, which holds a uint32 today and says nothing
about the uint64 byte counter beside it.

**An agreement between two readers asserts NON-EMPTY before it asserts EQUAL.**
Two empty sets are equal, so a read-only comparison passes over a kernel that
holds nothing, which is what the dump answers with its whole body deleted.
`requireSameSPISet` refuses an empty side first and names which side was empty.
`requireSPISetChanged` refuses any old SPI that remains after rekey. The checker
polls through the bounded old-SA cleanup window until Ze and iproute2 agree on a
nonempty replacement set, and checks strongSwan's kernel against that set too.
RFC 7296 Section 2.8 permits overlap during rekey; overlap after cleanup fails.
<!-- source: internal/le/interoplab/ipsec/helpers.go -- decodeZeDataplaneSPIs, spiValues, requireSameSPISet, requireSPISetChanged -->
<!-- source: internal/le/interoplab/ipsec/checkers.go -- checkDataplaneReadback -->

The same scenario reads all four Child SA byte and packet counters as `uint64`
values and compares them with the directed lifetime-current records from
`ip -s xfrm state`. It samples before and after bidirectional traffic and requires
each counter to advance. A bounded one-way probe makes the directions differ,
so swapping inbound and outbound counters cannot pass on a symmetric ping.

<!-- source: internal/le/interoplab/ipsec/checkers.go -- requireZeCountersAdvanced, readbackAsymmetricTraffic, requireCounterAdvancement -->
<!-- source: internal/le/interoplab/ipsec/helpers.go -- readbackCounters, requireDirectedCounters -->
Dataplane observation also crosses the live operator surfaces. With the
Prometheus exporter enabled, the checker scrapes the installed-SA count and
clean drift gauge, deletes one kernel SA, and requires CLI drift, degraded
`show health`, and the corresponding gauge transition. It restores the kernel
state and waits for recovery. Removing the configured peer then removes both
the peer and interface-label series.
<!-- source: internal/le/interoplab/ipsec/checkers.go -- checkDataplaneMonitoring, requireLiveDataplaneHealth -->

An unreadable observation is induced inside the Ze scenario container by
temporarily setting only the daemon's soft `RLIMIT_NOFILE` to zero. The netlink
package's default handle opens a socket for each XFRM dump. A Python helper
finds the daemon as the one `ze` process whose parent pid in `/proc/<pid>/stat`
is tini, because `/proc/<pid>/task/<tid>/children` exists only with
`CONFIG_PROC_CHILDREN`, which Ze's runtime kernel does not set. The helper
opens and warms an HTTP connection before changing the limit, scrapes that same
connection until both dataplane metric families have no series, and restores
the original limit in `finally`. Reconnection is refused during the fault.
During the fault, live health must identify an unknown or unreadable dataplane;
the preceding known-drift diagnosis cannot satisfy that assertion. Restoring
the limit must restore both the drift diagnosis and the published drift gauges.
The checker requires published series before the fault and their return after
restoration, so an exporter that never publishes cannot satisfy the test.
This helper needs Python in the lab image; it changes no production backend.
<!-- source: internal/le/interoplab/ipsec/helpers.go -- unreadableDataplaneProbe, requireUnreadableDataplane -->

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

Ordered-checker failures include the selected IPv4 and IPv6 networks. A JSON
wait retains its last answer, and a pmacct failure captures the collector log
plus the last 80 msglog rows before container teardown. These diagnostics retain
the original assertion failure; they do not substitute a guessed route state.

<!-- source: internal/le/interoplab/bgp/helper.go -- runtimeFailure -->
<!-- source: internal/le/interoplab/bgp/check_engine.go -- checkerFailure -->
<!-- source: internal/le/interoplab/bgp/bgp_test.go -- TestCheckerFailureKeepsPrimaryCauseWhenLogsFail -->

The external SDK carrier `srv6-service-export-control` announces two VPN
services with different route targets and SRv6 SIDs. A destination's export
policy withholds RT 10 while permitting RT 20; the unfiltered control peer must
receive both SIDs. Both peers remain connected while the observer checks the
socket-write counters, and the filtered peer continues rejecting RT 10 after
its initial End-of-RIB. These are BGP export observations, not SRv6 dataplane
reachability evidence.
<!-- source: internal/test/fixture/srv6_service_export.go -- srv6ServiceExportControl -->

The BMP Route Monitoring collector also waits past initial End-of-RIB messages.
It completes only after a received-direction UPDATE carries the test prefix in
its announced IPv4 NLRI. A matching byte sequence in attributes, a withdrawal,
or an Adj-RIB-Out report cannot establish reception of that route.
<!-- source: internal/test/fixture/plugin_fixture_04_bmp.go -- monitoringPrefix04, bmpCollector04 -->

### Conditional MED observed by a foreign collector

`bgp-addpath-best-path-pmacct` retains its two-identifier baseline and adds a
separate prefix, `10.99.77.0/24`, with three sources. FRR sends A from AS65004
with MED0 and Router ID198.51.100.3. GoBGP sends B from AS65005 with MED50 and
Router ID198.51.100.2. The existing injector sends C from AS65004 with MED100
and Router ID198.51.100.1. All three are external, have one AS in their path,
and carry ORIGIN IGP. The checker reads the exact received attributes from
pmacct's latest Adj-RIB-In route, including the absence of AIGP and received
LOCAL_PREF; Ze explicitly configures their common preference base. Router ID is
carried by Peer Up, not Route Monitoring. The checker joins the route to its
preceding Peer Up using the BMP router, connection port, peer address, RIB view
and increasing collector sequence, and requires the expected ASN in both rows.
A Peer Down, a new Peer Up or a BMP stream restart invalidates the old route.
An earlier correct identifier cannot satisfy a route from a different epoch.

A removes C at MED, then B defeats A at Router ID. The checker requires B,
withdraws non-best A through FRR, requires C, restores A, and requires B again.
Before each change it checkpoints the collector's append-only log. The selection
query keeps pre-checkpoint Peer Up context and every epoch boundary, rejecting
snapshots beyond its 256-line bound rather than silently truncating them.
Only the newest route in the current Loc-RIB epoch may satisfy the assertion,
and its absolute file position must follow the checkpoint. A matching Peer Down,
stream reset or replacement Peer Up invalidates an earlier winner. AS_PATH,
NEXT_HOP and ORIGIN identify the source because Loc-RIB reports omit MED.
All four original baseline assertions run before and after the cycle.
<!-- source: internal/le/interoplab/bgp/check_med_pmacct.go -- checkMEDWholeSet, medWholeSetBaselineOperations -->
<!-- source: internal/le/interoplab/bgp/check_med_pmacct_selected.go -- waitMEDWholeSetSelected, readMEDWholeSetLocEpoch, medWholeSetSelectedCriteria -->
<!-- source: internal/le/interoplab/bgp/check_med_pmacct_epoch.go -- requireMEDWholeSetInput, medWholeSetSameEpoch, medWholeSetInputCriteria -->
<!-- source: test/interop/scenarios/bgp-addpath-best-path-pmacct/ze.conf -- peer inputs -->

### Scenario Inventory

The suite has grown to over 100 scenario directories in `test/interop/scenarios/`. The table
below lists the core BGP scenarios (01-37); beyond these, the suite also covers route
reflection, policy import/export, RPKI origin validation, BMP monitoring
(`bmp-locrib-pmacct` puts Ze's RFC 9069 Loc-RIB feed in front of pmacct and requires
pmacct to print the configured Peer AS, the configured Peer BGP ID, the `global`
VRF/Table Name TLV and reason code 6 on the Peer Down; `bmp-locrib-receiver-frr`
turns the direction around, so FRR's `bmpd` drives Ze's BMP receiver and
`show bmp peers` must report the third party's Loc-RIB peer and its address
family; `bgp-addpath-best-path-pmacct` has the raw injector announce one prefix
under two ADD-PATH identifiers, MED 10 then MED 50, and requires pmacct's reading
of the Loc-RIB stream to carry the MED 10 best and never the MED 50 path, told
apart by their AS_PATHs since Ze's Loc-RIB Route Monitoring carries no MED, because
Ze advertises no best path to a BGP peer and the Loc-RIB feed is where a foreign
implementation can observe its election), an RFC 8277 labelled withdrawal
(`bgp-labeled-withdraw-compatibility-frr` has the raw injector announce 10.10.0.0/24 under
label 100 and 10.11.0.0/24 under label 101 in ipv4/mpls-label, then withdraw 10.10.0.0/24 alone
with the Compatibility value 0x800000 in the label field. FRR, fed through bgp-rs, must hold both
routes, then lose 10.10.0.0/24 with exactly one logged withdrawal while the injector session stays
Established. The checker then stops the injector, FRR must lose 10.11.0.0/24 through bgp-rs's
peer-down withdrawals, and its log must still hold one withdrawal of 10.10.0.0/24. bgp-rs relays
the withdrawal's bytes unchanged, so FRR losing the route alone cannot tell whether Ze read it:
Ze's reading decides what bgp-rs keeps in its route inventory, and a misread withdrawal leaves
10.10.0.0/24 there to be withdrawn a second time at peer down), PATHS-LIMIT,
max-prefix cease, a reload of the global router-id (`bgp-reload-global-router-id`
starts Ze with 10.255.0.1, reloads it to 10.255.0.2, and requires BIRD, a static
peer, and FRR, which Ze adds after the reload with `create bgp peer`, each to
report 10.255.0.2 as Ze's BGP Identifier), GTSM (`bgp-gtsm-frr` for the session, and `gtsm-related-icmp-ttl`
for RFC 5082 Section 3's related ICMP messages: FRR's kernel reads the hop limit of an
ICMPv6 error ze's kernel generates about the session, and its TCPMinTTLDrop counter is
what goes red when ze's host-route metric is absent), AS112, the RFC 7454 Section 9 transit leak
(`bgp-path-asn-leak-frr` gives FRR two prefixes that differ only in their AS_PATH, and requires
ze to drop the one reached through a listed transit ASN, keep the other, and keep the session),
an import prefix-list over multiprotocol UPDATEs (`bgp-policy-modify-mp-frr`: the raw injector
sends each case as one UPDATE, an IPv6 MP_REACH_NLRI with one prefix kept and one refused, IPv4
unicast inside MP_REACH_NLRI, an all-refused MP_REACH_NLRI, and thirty kept /64s with one refused
so the rebuilt attribute is 291 octets, then an MP_UNREACH_NLRI for the kept /48; FRR, behind
the OPEN-only relay at .11, must hold exactly the kept prefixes and lose the withdrawn one, and
the relay's capture of Ze's frames must show the extended-length MP_REACH_NLRI and no
MP_REACH_NLRI without NLRI),
next-hop self under `local ip auto` (`bgp-nexthop-self-local-auto-frr` configures no local
address toward FRR, and requires FRR to hold both ze's own route and a route relayed from a raw
injector with NEXT_HOP 172.30.0.2, ze's connected endpoint, never the injector's 172.30.0.9),
ADD-PATH re-advertisement (`bgp-addpath-readvertise-collision-frr`
proves a receiver keeps two paths whose sources both chose one Path Identifier, and
`bgp-addpath-rail-agreement-speaker` proves the live forward and the peer-up replay emit the same
bytes for one path), the RFC 6793 mixed-width relay
(`as-path-mixed-width-relay-frr` gives ze a route from a two-octet injector whose AS_PATH carries
AS_TRANS and whose AS4_PATH carries the real four-octet AS number, and requires FRR to report that
AS number and never 23456; `as-path-prepend-two-octet-peer` turns the direction around, so ze's own
non-mappable AS is prepended toward an FRR that refused the four-octet AS capability), the
RFC 8669 Section 6 repeated Prefix-SID TLV (`bgp-prefix-sid-duplicate-tlv-frr` gives ze an SRv6
L3VPN route whose Prefix-SID carries the SRv6 L3 Service TLV twice, and requires FRR, whose own
parser refuses a repeated type-5 TLV, to hold the route with the first SID only), the
Software Version capability (`frr-software-version` holds two sessions to one FRR: peer `legacy`
sends the length-prefixed form over IPv4 and must reach Established with FRR showing ze's version,
while peer `draft` sends the draft's bare form over IPv6 and must be refused, with ze recording
FRR's NOTIFICATION OPEN Message Error/Unspecific, because FRR 10.3.1 reads the first octet as a
length, and with ze's log carrying the one WARN `notification received` line that names it), the
RFC 9072 Section 2 boundary (`bgp-open-extended-params-boundary-frr` pads ze's capabilities to 254
octets with a 200-octet hostname, so the Optional Parameters field is 256 octets and must take the
extended form, and requires FRR to reach Established and report a prefix of that hostname), and full
IS-IS (auth, convergence, dual-stack, LAN DIS,
P2P, redistribution) and OSPFv2/OSPFv3 (auth, BFD, TE, LFA/TI-LFA, graceful restart,
segment routing, opaque LSAs, stub/NSSA, virtual links, and more) interop families.

`ospf-virtual-link-frr` and `ospfv3-vlink-frr` put a transit FRR between Ze and
the far virtual-link endpoint. VLAN 100 connects Ze `eth1` to transit `eth1`;
VLAN 200 connects transit `eth2` to the far endpoint's `eth2`. Docker `eth0`
remains the management link and carries no OSPF adjacency. The routed data
links use `10.200.0.0/30` and `10.200.0.4/30`, with IPv6
`2001:db8:100::/64` and `2001:db8:200::/64`.
The OSPFv2 endpoint is FRR. OSPFv3 uses the existing BIRD peer image because
FRR 10.3.1 does not implement the OSPFv3 virtual-link configuration command.
Ze advertises `192.0.2.1/32` and `2001:db8:10::1/128` from the dedicated
passive interface `backbone0`, so the IPv4 host prefix is the interface's primary
address rather than an unadvertised secondary beside `127.0.0.1` on `lo`.

The checker requires the far endpoint's address in Ze's Full backbone neighbor
record and an OSPF route at the independent endpoint for Ze's backbone host prefix. Changing the transit egress
cost from 10 to 30 must change the virtual cost from 20 to 40 without changing
the database-exchange identity or persistent adjacency-reset counters. Taking
that data interface down must withdraw the route and virtual reachability;
bringing it up must restore both through a new database exchange.

Ze's existing RIB and kernel FIB plugins install the transit route. The checker
reads the actual kernel route to the far data subnet, requires the intermediate
router as next hop on `eth1`, and requires withdrawal and restoration across the
same down/up cycle. IPv6 uses the transit router's observed link-local address.
Neither endpoint receives a preinstalled transit or backbone route; FRR uses
its normal zebra forwarding and BIRD exports OSPF routes through its kernel protocol.
<!-- source: internal/le/interoplab/bgp/prepare_virtual_link.go -- prepareVirtualLinkPeers -->
<!-- source: internal/le/interoplab/bgp/check_virtual_link.go -- checkOSPFVirtualLink -->
<!-- source: test/interop/scenarios/ospf-virtual-link-frr/ -- OSPFv2 configurations -->
<!-- source: test/interop/scenarios/ospfv3-vlink-frr/ -- OSPFv3 configurations -->

`bgp-graceful-restart-frr` retains its original session, route, GR and EOR
checks and adds two strict input speakers with FRR as the independent receiver.
One source omits IPv4 from GR while advertising a 40-second IPv4 LLST; its
IPv6 family has a 20-second GR period followed by the same LLST. The other
source advertises zero LLST. After fencing initial receipt and bilateral FRR
LLGR negotiation, the checker sends each native source `USR1` through
`Lab.Signal`. Only this oracle handles that signal: it closes its BGP socket
without NOTIFICATION, stops KEEPALIVEs, and keeps PID 1, the container and its
next-hop interfaces alive until its existing bounded lifetime ends or lab cleanup.
Unexpected peer closure, NOTIFICATION, or lifetime expiry before control remains
a failure. The main loss timestamp precedes the signal; the DOWN poll and
three-second observation margin are unchanged. The checker requires immediate
removal for zero LLST, immediate IPv4 LLGR_STALE, and IPv6's conventional-to-LLGR
transition.
IPv4 must expire at its original DOWN+40 deadline while IPv6 survives until
DOWN+60. FRR's established/dropped connection counters must remain unchanged;
its recomputed wall-clock epoch is not a session identity. Complete observation
cycles, including final expiry probes, must stay within the sampling bound.
An explicit FRR-origin received-route check preserves the original reverse
direction proof despite the added native sources. This does not cover
NOTIFICATION, reconnect F-bit or received-stale-policy behavior.

The receipt query is `show bgp rib received | json`; its CLI output is a
top-level array of route rows, not an object containing a `routes` field.
An empty array is a measured RIB with no matching route; null or an object
is not a valid CLI observation.

The received-route fence retains the last completed CLI result (stdout, stderr,
exit status and parse error), the last successfully measured RIB, and probe
counts when it fails. A final cancelled Docker exec cannot overwrite this
earlier evidence. Capturing it adds no queries or time to the original
30-second receipt fence and changes none of the 40/60-second expiry windows.
FRR route-observation errors retain the queried prefix and raw JSON response,
so an invalid route can be distinguished from an unexpected response shape.
<!-- source: internal/le/interoplab/bgp/check_llgr.go -- llgrReceivedFence -->

`bgp-nexthop-self-local-auto-frr` retains both original self-next-hop checks
and adds AIGP on the general forwarding path. The received metric is 100;
source-link cost 7 and destination-link cost 43 must produce 107 and Ze's
next hop at FRR. GoBGP is the unchanged-next-hop control and must retain 100.
The fixture's ASes are in one administrative domain. FRR 10.3.1 requires both
`neighbor ... aigp` and `neighbor ... oad` to retain received AIGP on eBGP;
`aigp` alone leaves the route installed but discards its AIGP attribute.
`oad` keeps the session eBGP, including its AS-path and next-hop assertions.
See the pinned [FRR receive gate](https://github.com/FRRouting/frr/blob/frr-10.3.1/bgpd/bgp_attr.c#L3421-L3427).
<!-- source: test/interop/scenarios/bgp-nexthop-self-local-auto-frr/frr.conf -- AIGP-enabled OAD neighbor -->
A third-party next hop initially has no distance: the rewritten route must
be absent while the unchanged control holds it. An SDK route installation
with metric 11 restores 111, then metric 7 produces 107; replacing that
distance with zero withdraws the whole rewritten route, and restoring 7
recovers it again. Source UPDATE, EOR and session counters fence the recovery
against a new advertisement.
FRR's established/dropped counters and local/foreign socket ports are also
captured before the initial withheld inventory and checked after every metric
transition and the final baseline checks. Recovery through a replacement
recipient session cannot satisfy that fence.

A separate iBGP wire recipient uses the existing `speaker-args` container.
Its passive Ze peer binds the explicit lab address; `local ip auto` supplies
no address for a passive listener. The original FRR local-auto checks remain
unchanged. The wire recipient records one connection's original received frames,
never reconnects, and
must observe exact AIGP TLVs 107 and 111, Ze's next-hop bytes, and a complete
withdrawal of `10.10.3.0/24`. Its history also checks the subsequent
107/withdrawal/107 transitions. Both the FRR readiness prefix and a checker
release route precede injection; the release follows all recipients' initial
EORs. This carries the former `aigp-source-cost-recovery` draft's wire contract
on a legal source topology: 127/8 NEXT_HOPs are rejected before AIGP, even when
unassigned to a host interface. Run it with
`INTEROP_SCENARIO=bgp-nexthop-self-local-auto-frr ./le test integration interop`.
<!-- source: internal/le/interoplab/bgp/speaker_aigp.go -- runAIGPWireRecipient -->
<!-- source: internal/le/interoplab/bgp/check_aigp_wire.go -- releaseSynchronizedAIGPSource, requireAIGPWire -->
<!-- source: test/interop/scenarios/bgp-nexthop-self-local-auto-frr/ -- source, wire and foreign recipients -->

`bgp-labeled-withdraw-compatibility-frr` also exercises VPNv4 and VPNv6,
with each prefix announced under three distinct RDs. Compatibility values
`800000` and `123456` withdraw two RDs while the third survives. FRR's
RD-keyed table and exact withdrawal log identify each route. After the
injector goes down, only the surviving RD may receive another withdrawal;
the previously removed routes must not be withdrawn twice. Receiver session
generation and a subsequent received KEEPALIVE fence the final observation.

`bgp-flowspec-sctp-gobgp` supplies both a covering unicast route and a FlowSpec
from GoBGP, checks the installed kernel rule, then withdraws and restores only
the unicast route. Its checker requires the existing FlowSpec filter to
disappear and return without another FlowSpec announcement. Finally it
withdraws the FlowSpec itself. Empty-ruleset checks require nft's JSON response,
not a table that should have been removed.

`bgp-flowspec-gobgp` retains its originated-rule and OR-of-AND checks and adds
a received rule with a nonzero next hop. A byte-preserving relay captures
the UPDATE delivered to GoBGP: its MP_REACH next-hop length must be zero,
and GoBGP must decode the rule and traffic-rate action. Withdrawing and
restoring only the covering route drives stored replay without a new source
FlowSpec UPDATE. The checker requires the same rule and action after recovery.
This independent-peer scenario covers IPv4 FlowSpec; the four-family wire
matrix remains a separate internal proof.

The Linux `integration`-tagged
`TestSelectedFlowSpecKernelPacketSemantics` uses an isolated network namespace
and actual UDP packets to distinguish ordered terminal and continuing actions,
deferred DSCP marking, withdrawal, and excess-rate drops. It requires
`CAP_SYS_ADMIN`, `CAP_NET_ADMIN`, and nftables kernel support. A capability skip
is not packet-forwarding evidence.

<!-- source: internal/le/interoplab/bgp/checkers.go -- bgp-flowspec-sctp-gobgp -->
<!-- source: internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go -- TestSelectedFlowSpecKernelPacketSemantics -->

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
| 38 | vpn-addpath-frr | Ze, FRR | ipv4/mpls-vpn over ADD-PATH with no path-information: FRR holds the exact RD, prefix and label, so ze wrote Path Identifier 0 (RFC 7911 Section 3) |
| 39 | ldp-keepalive-expiry-frr | Ze, FRR ldpd | ze's LDP KeepAlive Timer expires after FRR's TCP toward ze is dropped in ze's container (an `ldplab` nft table); FRR logs a received Notification naming KeepAlive Timer Expired (RFC 5036 Section 3.5.1.2.3) and leaves OPERATIONAL. Ships its own `daemons` (`ldpd=yes`) and gives ze transport address 172.30.0.250 so ze is the active LSR |
| 40 | bgp-attr-flags-low-nibble-gobgp | Ze, GoBGP, raw injector | RFC 4271 Section 4.3: attribute 241 sent to Ze at flags 0xCF reaches GoBGP at 0xE0 (low nibble cleared, Partial stamped by Section 5), never 0xEF |
| 41 | bgp-large-community-no-repeat-gobgp | Ze, GoBGP, raw injector | RFC 8092 Section 3: an export policy adding values the route already carries leaves GoBGP with each Large Community once |
| 42 | bgp-mup-malformed-t1st-withdraw-gobgp | Ze, GoBGP, raw injector | draft-ietf-bess-mup-safi Section 3.1.3.1: a malformed T1ST withdraws the earlier generation at GoBGP, a well-formed sibling stays, the session stays up |
| 43 | bgp-role-rs-client-learned-not-upward-frr | Ze, FRR 10.4.1, raw injector | RFC 9234 Section 3.1: a route learned on a session where Ze is the Route Server never reaches FRR, Ze's Provider; Ze's own control route does |
| 44 | bgp-rfc7999-blackhole-readvertise-frr | Ze, BIRD, FRR 10.4.1 | RFC 7999 Section 3.1: a BLACKHOLE-tagged route from BIRD is never re-advertised to FRR, an eBGP neighbor with no agreement; the untagged control is |
| 45 | bgp-sr-policy-gobgp | Ze, GoBGP | RFC 9830: GoBGP's text RIB holds Ze's configured IPv4 and IPv6 SR Policies with their distinguisher, color and endpoint, Binding SID label 1048575 and Type A segment labels 16 and 1048575, and a third IPv4 policy (color 300) whose SRv6 BSID 2001:db8::b5 rides the type-13 Binding SID sub-TLV at Length 18 (RFC 9830 Section 2.4.2), which GoBGP prints through its type-13 decoder with no B-Flag. Goal validation for origination: in-range labels never reach the 20-bit range check, so no revert turns the label checks red |
| 46 | bgp-sr-policy-malformed-receive-gobgp | Ze, GoBGP, raw injector | RFC 9830 Section 5: after policies A, B and control C reach GoBGP, A beside a 13-octet NLRI and then B without its Tunnel Encapsulation attribute are each treat-as-withdraw, so GoBGP loses A, then B, keeps C, and its session with Ze stays up (GoBGP resets a session that carries an SR Policy NLRI whose length is neither 96 nor 192 bits) |
| 47 | bfd-link-local-frr | Ze, FRR bfdd | RFC 5881 Section 3: a BFD-plugin-pinned single-hop session to FRR's IPv6 link-local on `eth0` reaches Up, each side holding the other's discriminator. Acceptance, not discrimination (see "BFD over link-local") |
| 48 | bfd-link-local-two-links-frr | Ze, FRR bfdd | RFC 5881 Section 3: two sessions to ONE link-local address, one each on `eth1` and `eth2`, reach Up on their own link with their own discriminators crossed, while Ze's Your-Discriminator-zero sends are dropped. Discriminates the receive-side zone reconciliation and the send-side link pinning (see "BFD over link-local") |
<!-- source: test/interop/scenarios/ -- scenario directories -->

### Running

```bash
./le test integration interop
INTEROP_SCENARIO=bgp-ebgp-ipv4-frr ./le test integration interop
VERBOSE=1 ./le test integration interop
NO_BUILD=1 ./le test integration interop
FRR_IMAGE=quay.io/frrouting/frr:10.3 ./le test integration interop
BUILD_TIMEOUT=7200 ./le test integration interop
```

Interop tests require Docker and are not part of the offline precommit gate.
They are a separate protocol-validation action.

The first run cross-compiles the lab binaries on the host, then builds the Docker
images. No `Dockerfile.ze` carries a Go compiler: each one is an `alpine:3.21`
base, one `apk add`, and a `COPY` of a binary the suite's preflight has already
written into the build context (`internal/le/interoplab/zebuild.go`,
`StageBinaries`). Each lab declares the binaries it needs beside the images it
needs, and the bgp lab declares two, because its scenarios also run the harness
from inside the container: a linux `le` built by `internal/le/linuxle`
(`linuxle.Base`), staged at `test/interop/le-linux` and copied to
`/usr/local/bin/le`. Harness peers start with entrypoint `le` and a command that
begins `test` (`le test interop-bgp speaker ...`). The image holds no program
under a retired harness name, so a scenario `ze.conf` that still runs one fails
to start it.

Measured on 2026-09-06 on a 32-core workstation: 6.1s and 4.8s for the two
cross-compiles against a warm `cache/go-cache`, at a peak resident set of 1.03
GiB and 0.98 GiB, then 54.7s for the `docker build`, which is now context
transfer rather than compilation.

Measured again on 2026-10-09 on the same workstation, from a `git archive`
export of 1cfc44afd7 with an empty `cache/go-cache`, while other sessions ran
labs: `INTEROP_SCENARIO=as-path-prepend-two-octet-peer ./le test integration interop`
reported `1 passed, 0 failed` in 9m27s of wall time, which includes building
the launcher itself from cold. The staged binaries were written about 18s and
9s apart. The largest process of the whole run peaked at 1524736 KiB
(1.45 GiB) under `/usr/bin/time -v`. A separate `docker build` of
`test/interop/Dockerfile.ze` over those staged binaries took 199.4s at a load
average of 65, with the client peaking at 48.5 MiB, so the time is the
contended daemon rather than any compile. Inside that image `ze --version` and
`le test` both ran, and `/etc/alpine-release` read `3.21.7`.

The shape before 2026-09-06 is what those numbers are read against. `Dockerfile.ze`
copied the whole tree and compiled ze twice with no cache mount. One colima VM of
2 CPUs and 2 GB built it in 2m48s on 2026-09-04, and the same VM took 40m39s for
it earlier that day, when the host disk was full and the guest was thrashing. On
2026-09-06 the kernel killed that build three times on an idle 31 GiB
workstation, once with 23 GiB free and nothing else running, so the compiler in
the container was the thing that did not fit rather than the machine being busy.

A consequence: `docker build -f test/interop/Dockerfile.ze .` on a clean checkout
now fails at the `COPY` until `./le test integration interop` has run its preflight.
Each converted Dockerfile's header names the action that writes its binary.

Each build is bounded at 90 minutes, and `BUILD_TIMEOUT` sets that bound in whole
seconds for a machine slower or faster than that one. The bound stops a wedged
Docker daemon and is not a budget for the build, so a build that finishes returns
at once and a generous bound costs nothing. A value that does not parse, or that
is not positive, keeps the 90 minutes.

An image that needs more than the machine bound declares its own
`ImageBuild.Timeout`. The build runs under the larger of that field and the
machine bound, so the field only ever lengthens the bound and a value below it
changes nothing; `BUILD_TIMEOUT` stays the one knob that lowers it. No suite
declares one. The PPPoE suite did until 2026-09-04: 10 minutes for its ze image,
15 for accel-ppp and 10 for the client, each written when the shipped default was
10 minutes and each a cap once the default became 90, because the field then
replaced the machine bound instead of raising it.

Subsequent runs with `NO_BUILD=1` skip rebuilds. Once the images exist, the full
suite takes roughly 5-10 minutes depending on session establishment times.
<!-- source: internal/le/interoplab/docker.go -- dockerBuildTimeoutDefault, buildTimeout -->
<!-- source: internal/le/interoplab/zebuild.go -- StageBinaries, LabBinary -->


### A package test's own server container (labtest)

A package `_test.go` that needs one real server, such as stayrtr for the RPKI
client, starts it with `labtest.Start(t, spec)` from
`internal/le/interoplab/labtest`, never with its own `exec.Command("docker")`.
`Start` pulls the image, starts one labeled container through
`Docker.RunDetached` (so a sweep removes it when the test binary is killed),
answers the host address the server listens on, and returns only once
`Spec.Ready` accepts that address. `Spec.Port` names the container port with
its transport, `interoplab.TCPPort(3323)` or `interoplab.UDPPort(123)`, and a
port with no transport is refused before any docker command runs. The
container either publishes that port on the host loopback at a port the daemon
picks (`Docker.PublishedPort` reads which, under the same transport), or runs
on the host network (`Spec.HostNetwork`) where it listens on `127.0.0.1:Port`
and reaches a listener the test opened. `Spec.Entrypoint` replaces the image's
entry point, for an image whose entry point writes its own configuration and
ignores the command. `t.Cleanup` writes the log tail of a failed test, then
removes the container.

| Machine state | What the test does |
|---------------|--------------------|
| No `docker` binary | skips, naming the install route; fails under `ZE_REQUIRE_DOCKER=1` |
| `docker info` fails | fails, naming the probe error |
| The image cannot be pulled | fails; under the `live` tag it skips an ad hoc run and fails under `ZE_REQUIRE_DOCKER=1` (owner decision Q-6, `pull_live.go`) |
| A `Spec.Dockerfile` image does not build | fails, naming the image, the Dockerfile and the `docker build` command that shows the failing step |
| The container does not start, or `Ready` never accepts | fails, then the log tail follows |

Two package tests start their server this way. The RPKI live tests start
stayrtr over TCP (`internal/component/bgp/plugins/rpki/rpki_live_test.go`, tag
`live`). The NTP client tests start chrony (`dockurr/chrony:4.9`) over UDP
(`internal/plugins/ntp/ntp_chrony_docker_test.go`, tag `docker`) and run
`syncWorker.doSync` with the real query: a server on its local reference at
stratum 7 is accepted and reported at stratum 7 with a near-zero offset, and a
server with no synchronised source (leap indicator 3, stratum 0 on the wire)
is rejected with no clock step. The container reads the host's own kernel
clock, so the offset is bounded, never a configured value.

A server with no published image is built from its Dockerfile:
`Spec.Dockerfile` names it from the module root, and `Start` then builds
`Spec.Image` through `Docker.Build`, with the Dockerfile's directory as the
context, instead of pulling it, and releases the tag the run owns after the
container is removed; a build that fails fails the test. A server no probe from
the host can observe, such as a UDP listener behind a published port that
accepts datagrams before the server binds, names `Spec.ReadyInContainer`
instead of `Spec.Ready`: a command run in the container whose exit 0 is ready.
A test that reads what the server wrote calls `labtest.StartContainer`, which
answers the `Container` with its address and `Container.Exec`. The flow-export
collector tests run this way (`internal/plugins/flowexport/collectors_docker_test.go`,
tag `docker`): nfcapd/nfdump and sflowtool built from
`test/interop-flowexport/Dockerfile.collectors`, ready once the collector's
socket is in the container's UDP table, their decoded records read with
`docker exec` (`docs/architecture/flowexport/flow-export-1-counter-export.md`,
"Independent collector tests").

A package test whose real peer is a client, against a server the test runs
in-process, runs it with `labtest.Run(t, client)`. `Run` pulls the image, runs
one labeled container to its exit through `Docker.RunOneShot`, and answers the
client's stdout, stderr and exit code, a non-zero exit included, so a test can
assert a refused request. `Client.HostNetwork` puts the client in the host's
network namespace (`OneShotContainer.Network`), where it reaches the test's
listener on `127.0.0.1`, and `Client.Mounts` bind-mounts files the test wrote,
such as a CA certificate (`OneShotContainer.Mounts`). The machine states in the
table below are handled as `Start` handles them; a run the docker client did
not finish (`interoplab.OneShotDidNotExit`), or an exit code `docker run` keeps
for its own failure (125 to 127), fails the test. The gNMI server tests run
gnmic this way (`internal/component/gnmi/gnmic_docker_test.go`, tag `docker`):
Capabilities, Get, Subscribe ONCE and Set over TLS with a bearer token,
decoded from gnmic's protojson output, and a request with no token or a wrong
one refused `Unauthenticated`.

`labtest` runs no Docker host kernel check: Ze runs in the test process on the
host, and that check guards a containerised Ze. A peer that needs a Ze-kernel
feature belongs in an `interoplab.Suite` lab. Only a `_test.go` file, or tooling
under `internal/le`, imports `labtest` (`./le arch tier check`,
`docs/architecture/module-tiers.md`).
<!-- source: internal/le/interoplab/labtest/labtest.go -- Start, start, host.release -->
<!-- source: internal/le/interoplab/labtest/container.go -- Container, host.build, host.readyInContainer -->
<!-- source: internal/plugins/flowexport/collectors_docker_test.go -- TestFlowExportNFDumpInterop, TestFlowExportSFlowToolInterop -->
<!-- source: internal/le/interoplab/labtest/labtest.go -- Run, run, Client -->
<!-- source: internal/le/interoplab/docker.go -- OneShotContainer, OneShotDidNotExit -->
<!-- source: internal/component/gnmi/gnmic_docker_test.go -- TestGnmicInterop -->
<!-- source: internal/plugins/ntp/ntp_chrony_docker_test.go -- TestNTPClientSyncsFromChrony, TestNTPClientRejectsUnsynchronisedChrony -->
<!-- source: internal/le/interoplab/container.go -- DetachedContainer.PublishedPorts, ContainerPort, Docker.PublishedPort -->

### Debugging Failures

The shared orchestrator does not read container logs. Each suite's checker adds
them to the failure it returns:

| Suite | What a failed checker carries |
|-------|-------------------------------|
| BGP (`interop`) | the last 80 log lines of each peer the scenario started (`checkerFailure`, `internal/le/interoplab/bgp/check_engine.go`), and pmacct's message log |
| IPsec (`interop-ipsec`) | the last 60 log lines of Ze, strongSwan, the NAT box and FRR, for each one the scenario started, read after the checker's own deadline has passed, all of them within one 20 s bound (`checkerFailure`, `internal/le/interoplab/ipsec/failure.go`) |
| RSVP-TE (`interop-rsvpte`) | each peer's capture tail and the last 60 log lines (`diagnosticError`, `internal/le/interoplab/rsvpte/checkers.go`) |
| A package test's own server (`labtest.Start`) | the last `Spec.LogLines` lines (1 to 1000) of its container, written to the test log by `t.Cleanup` when the test failed, before the container is removed (`host.release`, `internal/le/interoplab/labtest/labtest.go`) |

A `docker exec` that runs past its 30 s bound names the container, the command it
ran there (cut at its first line and at 160 bytes), and the bound
(`execOperation`, `internal/le/interoplab/docker.go`).
<!-- source: internal/le/interoplab/ipsec/failure.go -- checkerFailure, checkerFailureReport -->
<!-- source: internal/le/interoplab/docker.go -- execOperation, commandError -->

For more detail:

- `VERBOSE=1` enables debug output (polling status, container commands, raw CLI output)
- `SESSION_TIMEOUT=120` increases the session establishment timeout (default 90s)
- Single-scenario runs isolate the problem: `INTEROP_SCENARIO=bgp-graceful-restart-frr ./le test integration interop`

### Writing a New Scenario

1. Create a descriptively named directory under `test/interop/scenarios/`.
2. Add `ze.conf` and whichever peer configs the scenario needs.
3. Add the scenario's ordered assertions to `scenarioOperations` and
   `scenarioExtras`, or register a bespoke checker in `specialCheckers` when the
   control flow is non-linear.
4. For a bespoke checker, put each decision in a pure predicate in
   `check_rfc_predicate.go` and add its both-polarity subtest to
   `TestBespokeCheckerBranches`.
5. Run `INTEROP_SCENARIO=<name> ./le test integration interop`.

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

#### Explicit configuration is authoritative at every start

`ze start <file>` reads that file on every start, including a container restart.
Persisted configuration from a previous run does not replace the explicit input.
The labs mount individual configuration files, so replacing a source file by
rename can leave an existing container's bind mount on the old inode. Recreating
the container refreshes the mount; the harness creates each container once.

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
`1:raw:`, `1:signal:`, and `1:json:` records rather than the standard `.ci`
format. `option=serial` marks process-driven fixtures that must not overlap
other ExaBGP harness instances; the runner executes those after the parallel
batch.

The mock replies with the client's OPEN capabilities, changing only its AS
number and router ID; it does not infer capabilities from expected UPDATEs.
Migration expands an omitted or empty ExaBGP family block to every family
registered in the current binary. An explicit family block stays restricted
to its entries. `conf-vpn` explicitly declares `ipv4 mpls-vpn`, which migrates
to `session > family ipv4/mpls-vpn`, so both OPENs advertise AFI 1, SAFI 128.
The negotiated-family guard remains active: RFC 4760 Section 8 requires both
speakers to advertise the family for bidirectional exchange.

`<prefix>:signal:<NAME>` marks the point in a connection's script where the
runner reloads Ze. It divides the script: every `raw` frame written before it
must match before the reload happens, and the frames after it are matched only
once it has. Each signal step consumes the NEXT `option=file:` config the case
names, so a case owes one config more than it has signals, and the runner
refuses a case where those two counts disagree. `test/exabgp-compat/api/api-reload.ci`
is the case that drives this: its withdrawal of `2.0.0.0/24` is produced BY the
reload.

The fixtures name ExaBGP's reload signal, `SIGUSR1`. Ze reloads on SIGHUP, so
the runner translates the name (`deliverExaBGPReloads`), writes the next
migrated config over the path Ze reads, and signals Ze's own pid rather than the
process group, which also holds the bridge's scripts.

Coverage includes:

| Category | Examples |
|----------|----------|
| Address families | IPv4/IPv6 unicast, VPN, FlowSpec, FlowSpec VPN, EVPN, VPLS, MPLS labeled, MUP, MVPN |
| Path attributes | ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, communities (standard, extended, large), AGGREGATOR, ORIGINATOR_ID, PREFIX_SID, SRv6 |
| Capabilities | 4-byte ASN, ADD-PATH, link-local next-hop, software version, hostname |
| Edge cases | Generic/unknown attributes, self-referencing routes, group limits, IPv4+IPv6 mixed configs, deferred announcement (watchdog) |

### Running

```bash
./le test functional exabgp-test
```

ExaBGP compatibility is part of the offline precommit gate.
<!-- source: test/exabgp-compat/encoding/ -- .ci test files for wire compatibility -->

## Test Hierarchy

| Workflow | Includes Interop? | Includes ExaBGP? | Requires Docker? |
|----------|-------------------|-------------------|-------------------|
| Offline precommit gate | No | Yes | No |
| Standard functional sweep | No | Yes | No |
| `./le test integration interop` | Yes | No | Yes |
| `./le test functional exabgp-test` | No | Yes | No |

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

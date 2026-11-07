# PPPoE Docker Interop Lab

This peer-isolated Docker lab proves both Ze PPPoE roles against independent
implementations. Ze acts as a client against
[accel-ppp](https://accel-ppp.org/), then as an access concentrator against
`pppd` with the `rp-pppoe` plugin.

## Overview

Each scenario runs its containers on an isolated bridge because PPPoE uses
Ethernet discovery and session frames (EtherTypes 0x8863 and 0x8864). No
container runs privileged: Ze, accel-ppp and the pppd client each hold
`NET_ADMIN` and the host's `/dev/ppp` device, and the discovery sockets use the
`NET_RAW` Docker grants by default. The host kernel must provide `/dev/ppp` and
the `pppoe` pppox module, built in or loaded by the host: no container loads a
kernel module.

The client scenario requires PADI/PADO/PADR/PADS discovery, LCP, CHAP-MD5,
IPCP, one kernel device named after the `pppoe-client` key (`pppoe0`),
traffic to the access concentrator, the
accel-ppp session view, and clean teardown. The access-concentrator scenario
requires the same discovery and NCP path from an independent pppd trace. It
also dials a wrong credential and requires CHAP refusal before proving that all
session state disappeared.

## Layout

```
test/interop-pppoe/
  Dockerfile.ze        Ze image used in both roles (copies the ze-linux the
                       suite preflight cross-compiles)
  ze-linux             the staged daemon, git-ignored and rewritten each run
  Dockerfile.accel     accel-ppp access-concentrator image
  Dockerfile.client    pppd and rp-pppoe client image
  scenarios/
    01-pppoe-chap-ipv4/         Ze client, accel-ppp concentrator
    02-ze-ac-pppd-client/       Ze concentrator, pppd client
    pppoe-omitted-mru-ze-ac/    Ze concentrator, peer MRU omitted, measured Ze transmit cap
    pppoe-chap-rechallenge-ze-client/  Ze client, native accel-ppp reauthentication
    pppoe-named-device-reconnect-ze-client/  Ze client as a CPE: pppoe0, static default, masquerade, reconnect
    pppoe-peer-terminate-ze-ac/        Ze concentrator, pppd LCP termination without incoming PADT
    pppoe-chap-only-pap-client/       Ze concentrator, CHAP-only default, PAP-only pppd client
    pppoe-empty-service-name/  Ze concentrator, pppd client, no service-name configured
    pppoe-padr-replay/         Ze concentrator, pppd client, max-sessions-per-mac 1
    ipv6cp-zero-identifier/    Ze concentrator, pppd session, zero IPv6CP identifier replayed
    ipv6cp-missing-option/     Ze concentrator, pppd client whose IPv6CP request carries no identifier option
internal/le/interoplab/pppoe/
  pppoe.go             Native images, preflight, selection, and lifecycle
  scenarios.go         Role-selected container plans and mounts
  check_client.go      Ze client assertions
  check_ac.go          Ze access-concentrator assertions
  check_named_device.go  pppoe-named-device-reconnect-ze-client: device name, route and NAT across a reconnect
  check_service_name.go  pppoe-empty-service-name: wire-level Service-Name proof
  check_auth_allow.go    pppoe-chap-only-pap-client: session-correlated policy refusal and resource cleanup
  check_padr_replay.go   pppoe-padr-replay: wire-level replay and per-MAC cap proof
  check_ipv6cp.go        ipv6cp-zero-identifier, ipv6cp-missing-option: wire-level
                         IPv6CP proof, BLOCKED until plan/post-release/features/spec-l2tp-ipv6-subscriber.md
                         lands (see the file's own header comment)
```

The Dockerfiles keep their small amount of peer initialisation in the image
entrypoint command. There are no separate runner or entrypoint scripts.

## Prerequisites

The native preflight probes `/dev/ppp` and the `pppoe` module from a temporary
container that holds the peers' grants and nothing more. It exits non-zero when
either is absent, names what is missing, and prints the host command that loads
it:

```
sudo modprobe -a ppp_generic pppoe
```

Ze's kernel builds both in, so `./le test qemu docker-lab` needs neither. Setting
`ZE_PPPOE_SKIP_KERNEL_PROBE` or `ze.pppoe.skip-kernel-probe` causes an immediate
refusal.

The probe decides whether Docker can carry the proof. Docker Desktop on macOS
was measured on 2026-08-01 and its Linux VM supplied both requirements on that
machine. The QEMU action remains available for a host where the probe refuses.

<!-- source: internal/le/interoplab/pppoe/pppoe.go -- preflight -->

The client image pins pppd **2.5.5** from the official GitHub tag archive with
a SHA256 checksum. The archive contains autotools inputs, not `configure`.
Upstream `autogen.sh` generates the build system and runs `configure` with
the image's existing installation paths. Build-only autotools dependencies
do not enter the runtime image. Native runs against pppd 2.5.5 passed
`pppoe-chap-only-pap-client`, `pppoe-pap-ze-ac` and
`02-ze-ac-pppd-client` on 2026-11-04. The CHAP-only scenario also failed
against a rebuilt producer with the old broad authentication default and
passed again after restoration.
<!-- source: test/interop-pppoe/Dockerfile.client -- PPP_VERSION, PPP_SHA256, build stage -->

## Running

```
./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=01-pppoe-chap-ipv4 ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=02-ze-ac-pppd-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-empty-service-name ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-padr-replay ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=ipv6cp-zero-identifier ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=ipv6cp-missing-option ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-pap-ze-ac ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-pap-ze-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-chap-rechallenge-ze-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-named-device-reconnect-ze-client ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-peer-terminate-ze-ac ./le test deployment docker-pppoe-accel
ZE_PPPOE_INTEROP_SCENARIO=pppoe-chap-only-pap-client ./le --name session test deployment docker-pppoe-accel
```

`01-pppoe-chap-ipv4` and `02-ze-ac-pppd-client` predate the naming rule and keep
their numeric prefixes. A scenario added since carries none:
`interoplab.Discover` and `ZE_PPPOE_INTEROP_SCENARIO` both match a scenario by
its directory name, and a number is a reservation a later scenario can take, or
a hole nothing tells apart from one (`ai/rules/interop-and-goal-validation.md`).

`NO_BUILD` skips image builds, `SESSION_TIMEOUT` changes the default 90-second
scenario bound, and `ZE_PPPOE_INTEROP_SUFFIX` provides parallel-run isolation.

The QEMU proof for Ze as a client against accel-ppp is:

```
./le test qemu run kernel tmp/kernel/build/vmlinuz \
  packages "iproute2 iputils accel-ppp ppp kmod" \
  command "./le test qemu pppoe-accel"
```

The host `qemu run` action boots the runtime kernel and installs the guest
packages. The guest `pppoe-accel` action then runs Ze and accel-ppp in
two network namespaces joined by a veth. Invoking the guest action on the
host does not boot a VM. This proof covers the client role only. The Docker
suite carries both roles.

<!-- source: internal/le/test/qemu/pppoe_accel_linux.go -- runPPPoEAccelGuest -->

## Scenarios

### 01-pppoe-chap-ipv4

Ze authenticates as `alice` with CHAP-MD5. IPCP assigns `10.11.0.2` with
`10.11.0.1` as its peer. The checker requires one Ze PPP device, `pppoe0`, with that
point-to-point address, a route through it, ICMP to the peer, an accel-ppp
session for `alice`, and removal of that session after Ze stops.

The checker also reads accel-ppp's verbose trace and requires, in order, its
LCP Configure-Request with `<auth CHAP-md5>`, Ze's Configure-Ack carrying that
option, a CHAP Challenge, Ze's Response naming `alice`, and a CHAP Success.
A PAP exchange is refused even if IPCP and ping succeed. The shared session
checks remain the same for the PAP scenario; each scenario checks its own
authentication evidence.

These spellings come from accel-ppp 1.14.0's
[`ppp_lcp.c`](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/ppp/ppp_lcp.c),
[`auth_print`](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/ppp/ppp_auth.c),
and [CHAP-MD5 handler](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/auth/auth_chap_md5.c),
not from Ze's configured method. Both client fixtures set `[log] copy=1`:
accel-ppp's
[`general_log`](https://github.com/accel-ppp/accel-ppp/blob/1.14.0/accel-pppd/logs/log_file.c)
otherwise discards session messages from the general log exposed by Docker.
<!-- source: internal/le/interoplab/pppoe/check_client.go -- checkZeClient, checkZeClientSession, accelCHAPEvidence -->

### 02-ze-ac-pppd-client

The independent pppd client requests service `internet`, authenticates as
`alice`, negotiates IPCP, and pings Ze through its PPP interface. The checker
reads pppd's discovery, bidirectional LCP, CHAP, and IPCP trace and compares it
with Ze's REST session table. It then requires PADT and empty state before
repeating the dial with `wrong-secret`; the second trace must reach CHAP and
receive a refusal without creating a session.

### pppoe-omitted-mru-ze-ac

The real pppd client omits its MRU option. The checker correlates its decoded
Configure-Request with Ze's matching Configure-Ack and requires reciprocal
LCP acknowledgment, CHAP success and assigned IPv4 service. It measures Ze's
own `pppN` interface with `ip -j link show` and requires MTU 1492. A client
interface capped at 1492 cannot substitute for this Ze-side measurement.
Both peers must send traffic through the session, followed by PADT, empty
session tables and release of Ze's kernel PPP resources.
<!-- source: internal/le/interoplab/pppoe/check_mru_omitted.go -- checkOmittedPeerMRU, omittedMRULCPEvidence, omittedMRUZeLink -->

The checker appends pppd's `default-mru` option to the normal typed dial.
In pinned pppd 2.5.5, `lcp_addci` omits MRU when `neg_mru` is false.
The PPPoE plugin otherwise lowers the requested MRU to 1492 for a normal
Ethernet link, so leaving out a command-line `mru` value alone does not
exercise omission. `default-mru` also refuses Ze's initial MRU option;
successful replacement negotiation remains part of this proof. The test
retains pppd's full trace and Ze's measured link JSON in the native job log.
<!-- source: internal/le/interoplab/pppoe/check_mru_omitted.go -- checkOmittedPeerMRU -->
<!-- source: test/interop-pppoe/Dockerfile.client -- PPP_VERSION -->

Run `ZE_PPPOE_INTEROP_SCENARIO=pppoe-omitted-mru-ze-ac ./le --name session test deployment docker-pppoe-accel`.
The scenario is authored; a passing run and rebuilt defective-artifact
discrimination must be recorded before its behavior is claimed as proved.
<!-- source: internal/le/interoplab/pppoe/check_mru_omitted.go -- init -->

### pppoe-chap-rechallenge-ze-client

After IPCP traffic succeeds, the checker observes two native accel-ppp CHAP
reauthentication exchanges. It checks each Response's Identifier, CHAP-MD5
digest and username, and each corresponding Success. The kernel PPPoE session,
PPP interface, address and route must remain unchanged, and traffic must still
cross the session. This separates network-phase CHAP from initial login.

<!-- source: internal/le/interoplab/pppoe/check_lifecycle.go -- checkCHAPRechallenge -->
<!-- source: internal/le/interoplab/pppoe/check_lifecycle_wire.go -- rechallengeEvidence -->
<!-- source: test/interop-pppoe/scenarios/pppoe-chap-rechallenge-ze-client/accel-ppp.conf -- native auth interval -->

### pppoe-named-device-reconnect-ze-client

Ze runs the home CPE configuration the guides show. The `pppoe-client pppoe0`
entry sets `no-default-route`, a static route `0.0.0.0/0` has
`next { interface pppoe0 }`, and a NAT postrouting term masquerades
`output-interface "pppoe0"`. A dummy `lan0` at `198.51.100.1/24` stands in for
the LAN. The checker first deletes the default route Docker gives the
container through `eth0`. A CPE has no such route, and at metric 0 it would
collide with fib-kernel's add of the static default. If Ze already holds a PPP
device at that point, the checker stops and asks for a rerun. It also adds
`192.0.2.1` to accel-ppp's loopback: only the static default covers that
address on Ze's side.

On the first session the checker requires:

- `pppoe0` as the only PPP device in Ze's container, so no `pppN` comes from
  the client
- its IPCP address
- the default route at protocol 250 (fib-kernel) through `pppoe0`
- `ip route get 192.0.2.1` through `pppoe0`
- a ping from `198.51.100.1` to `192.0.2.1` that is answered, which needs the
  masquerade, because accel-ppp holds no route back to `198.51.100.0/24`
- a higher packet count on the masquerade rule's counter, read with
  `nft list chain ip ze_cpenat postrouting`

The checker then runs `accel-cmd terminate username alice`. In accel-ppp,
`terminate_exec2` (`accel-pppd/cli/std_cmd.c`) matches the session by
username and calls `ap_session_terminate`, and the PPPoE `disconnect` sends
PADT. The checker waits for a new `pppoe0` with another ifindex. It then
requires the same evidence again: the default route back through the new
device with no reload, the masqueraded ping, and a higher counter. The second
session's address is `10.11.0.3`, because accel-ppp's pool hands out the head
of its free list (`get_ip` in `accel-pppd/extra/ippool.c`) and `put_ip` returns
`10.11.0.2` to the tail.

The accel-ppp functions were read at `06f64b1` of the upstream tree. The lab
runs Alpine's 1.14.0 package.
<!-- source: internal/le/interoplab/pppoe/check_named_device.go -- checkNamedDeviceReconnect, removeDockerDefaultRoute, namedMasqueradeThroughSession -->
<!-- source: test/interop-pppoe/scenarios/pppoe-named-device-reconnect-ze-client/ze.conf -- CPE configuration -->

The scenario is authored. A passing run and a red run against a rebuilt
defective producer must be recorded before this page claims the behavior is
proved.

### pppoe-peer-terminate-ze-ac

The checker first establishes authenticated traffic with pppd. It then filters
only pppd's outgoing PADT and asks pppd to terminate. Capture at Ze must show the
peer's LCP Terminate-Request, the matching Ze Terminate-Ack, and Ze's own PADT
after the Restart timer. No client PADT may reach Ze. The checker records Ze's
live PPP interface before termination, then requires the REST session and both
peers' PPP interfaces to disappear. Ze's `/proc/net/pppoe` must retain only its
header, independently proving kernel session cleanup. The filter requires the
host's `clsact`, `u32`, `mirred` and `dummy` support and fails closed if
installation fails. It redirects only PADT into a dummy interface owned by the
client container; removing that container removes the sink too. Redirect consumes
the matched frame rather than mirroring it, and the dummy driver frees it without
transmission. These facilities already ship for Ze's interface and mirroring
features, so this fault injection needs no test-only `gact` kernel requirement.

The action semantics come from iproute2's
[`parse_direction`](https://github.com/iproute2/iproute2/blob/main/tc/m_mirred.c)
and Linux's
[`tcf_mirred_to_dev`](https://github.com/torvalds/linux/blob/master/net/sched/act_mirred.c)
and [`dummy_xmit`](https://github.com/torvalds/linux/blob/master/drivers/net/dummy.c).

<!-- source: internal/le/interoplab/pppoe/check_lifecycle.go -- checkPeerTerminate -->
<!-- source: internal/le/interoplab/pppoe/check_lifecycle_wire.go -- terminateEvidence -->

### pppoe-chap-only-pap-client

Ze's AC has neither `auth-method` nor `authentication-allow` configured, so
its effective policy is CHAP-MD5 only. The real pppd client refuses CHAP,
EAP and both MS-CHAP variants, leaving PAP as its only acceptable method.
The fixture supplies valid credentials and an address pool: an accidental
PAP downgrade cannot hide behind a credential or address-allocation failure.

Before dialing, the checker waits for tcpdump's capture-activation marker at
Ze. It requires Ze's successful PADS, its CHAP-MD5 Configure-Request, the
client's matching-Identifier PAP Configure-Nak, Ze's LCP Terminate-Request,
and exactly one **Ze-originated** PADT, all on the PADS session ID and the
same Ethernet endpoints. A client PADT cannot substitute for Ze's PADT.
No PAP or other authentication/NCP traffic may appear, and a client
Configure-Ack that could let Ze's LCP reach Opened is refused. The checker
also reads pppd's own Configure-Nak and Terminate-Request trace, and requires
Ze's refusal record to name the PAP suggestion and the CHAP-only allow-list.

The checker does not kill pppd or sleep for a guessed teardown interval.
It waits for the dial to exit, Ze's REST session table to empty, both peers'
PPP interfaces to disappear, and Ze's `/proc/net/pppoe` to retain only its
header. Capture remains active through those observations; the final flushed
capture is judged again so duplicate Ze PADTs count.

The independent peer behavior comes from pppd 2.5.5's
[`lcp_reqci`](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/lcp.c),
which Naks refused CHAP with PAP when EAP is refused, and
[`fsm_rtermreq`](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/fsm.c),
which acknowledges termination before Opened but keeps negotiating until
Ze's lower-layer teardown. This negative scenario complements, rather than
changes, the explicit `auth-method pap` success proof below.

<!-- source: internal/le/interoplab/pppoe/check_auth_allow.go -- checkCHAPOnlyPAPClient, chapOnlyRefusalEvidence -->

### pppoe-empty-service-name

Ze's AC carries no `service-name` leaf. The independent pppd client dials with
no requested service (RFC 2516 Section 5.1's "any service is acceptable"). The
checker captures the discovery exchange on the wire (tcpdump inside the client
container, decoded with `internal/core/pcap` and
`internal/component/l2tp/pppoe.ParseDiscovery`) and requires exactly one
Service-Name tag on the PADO and exactly one on the PADS, alongside the same
LCP/CHAP/IPCP proof `02-ze-ac-pppd-client` performs. A session coming up is not
enough evidence here: the unit tests already pin `BuildPADO`/`BuildPADS` always
writing one tag, so this scenario's job is proving a real peer reads the same
bytes off Ze's own wire, not proving the encoder again.

### pppoe-padr-replay

Ze's AC carries `max-sessions-per-mac 1`. The checker dials one session with the
independent pppd client, waits until it is live in PPP, then replays the exact
captured PADR frame from inside the client container with `tcpreplay`
(`interoplab.SendFrameInContainer`, the one frame sender every lab uses, the BGP
lab's IS-IS purge injector included). The send runs in the container because an
unprivileged checker cannot enter a rootful Docker container's network
namespace. It requires the PADS that replay provokes to
carry the SAME session id, never a second one. It then dials a second,
genuinely independent session from the same MAC and requires the PADS it
provokes to carry session id `0x0000` and an AC-System-Error tag, read off the
captured frame -- a session count that stayed at one is not enough evidence on
its own for either claim (`ai/rules/interop-and-goal-validation.md`, "Prove a
scenario discriminates"). The first dial's pppd is still running during the
second dial and owns the session the cap counts, so the second pppd carries
`ipparam ze-over-cap-dial` and the checker stops only that one (`pkill -f`).
Ze's table must still list the first session afterwards.
`TestOverCapDialCleanupSparesTheLiveSession` reproduces the failure a stop of
every pppd caused. `spec-pppoe-padr-replay-allocates-unbounded-sessions`.

### pppoe-pap-ze-ac

Ze's AC carries `auth-method pap`, and pppd dials with `refuse-chap`, so PAP is
the only method both ends accept. The checker requires pppd's trace to show
Ze's Configure-Request demanding `<auth pap>`, the client's named
Authenticate-Request and Ze's Authenticate-Ack, with no CHAP anywhere, then
IPCP, ICMP to Ze, and PADT teardown. It captures the session-stage frames
(0x8864) on the client's wire and requires exactly one Authenticate-Request and
one Authenticate-Ack carrying the same Identifier, both on the PPPoE SESSION_ID
Ze's REST table reports.

It then resends that captured Authenticate-Request from inside the client
container with `tcpreplay`, after IPCP has opened, with its Identifier advanced
by one, because a peer retransmitting MUST change it (RFC 1334 Section 2.2.1:
"The Identifier field MUST be changed each time an Authenticate-Request packet
is issued"). It requires Ze to put exactly one Authenticate-Ack and no Nak on
the wire, on the same SESSION_ID, carrying the new Identifier (Section 2.2.2:
"The Identifier field MUST be copied from the Identifier field of the
Authenticate-Request which caused this reply"), so a cached Ack resent with the
old Identifier fails. Afterwards Ze's REST table must still list that one
session, under the same SESSION_ID, and the client must still hold one PPP
interface. That is RFC 1334 Section 2.2.1: "the authenticator MUST allow
repeated Authenticate-Request packets after completing the Authentication
phase", answered with the same Code (`internal/component/l2tp/ppp/pap.go`,
`reanswerPAP`). `TestCheckPAPReanswerJudgesTheReply` drives the same judgement
over scripted replies.
`spec-ppp-pap-reanswer-after-auth`.

### pppoe-pap-ze-client

Ze dials accel-ppp as in `01-pppoe-chap-ipv4`, but accel-ppp loads `auth_pap`
and no CHAP module, so the session comes up only if Ze's client authenticates
with PAP (`internal/component/l2tp/pppoeclient/session.go`, `runClientAuth`).
The checker runs the shared session proof (link, address, route, accel-ppp
session, ICMP, removal after Ze stops), then reads accel-ppp's own
trace and requires `recv [PAP AuthReq` and `send [PAP AuthAck`, with no CHAP.
The lab cannot lose a frame on demand, so the retry itself stays proven by the
discriminated unit tests in `pppoeclient`; this scenario proves the PAP path
interoperates. `spec-pppoe-client-pap-retry`.

### pppoe-ipv6cp-ze-client

Ze dials accel-ppp with `ipv6 enabled true`, `accept-ra 2`, `autoconf true`
and `forwarding true` on its `pppoe-client`. accel-ppp runs `ipv6=prefer`,
so the session starts only once IPv6CP opened. Its own identifier is
`0:0:0:1`, and `ipv6-peer-intf-id=0:0:0:5a` with
`ipv6-accept-peer-intf-id=0` makes it Nak any other identifier the client
offers with `0:0:0:5a` (`ipaddr_recv_conf_req` in accel-ppp's
`ppp/ipv6cp_opt_intfid.c`). Ze starts from its source MAC's EUI-64, so the
Nak is always exercised. `ipv6pool` hands the session the first /64 of
`2001:db8:100::/56`, and `ipv6_nd` advertises it with the autonomous flag.

The checker reads accel-ppp's own trace for, in order: Ze's Configure-Request
with one non-zero Interface-Identifier and no other option, accel-ppp's
Configure-Nak of that request suggesting `0:0:0:5a`, Ze's next request
carrying exactly `0:0:0:5a` (RFC 5072 Section 4.1), and accel-ppp's
Configure-Ack of it, plus Ze's Configure-Ack of accel-ppp's own request.
In Ze's container it then requires the device to be `pppoe0`, `fe80::5a/64`
on it, `accept_ra` 2 and `forwarding` 1 on it, a default route via `fe80::1`
(accel-ppp's link-local) through it, a global address SLAAC formed in
`2001:db8:100::/64`, and an IPv6 ping to `fe80::1` across the session. The
IPv4 session proof of `01-pppoe-chap-ipv4` follows.

### pppoe-ipv6cp-rejected-ze-client

The same Ze client against an accel-ppp with `ipv6=deny`, which answers every
IPv6CP packet with an LCP Protocol-Reject (`ipv6cp_recv` in accel-ppp's
`ppp/ppp_ipv6cp.c`). The checker waits for that reject, pings the IPv4
gateway five times (longer than Ze's 3-second IPv6CP Restart timer), then
requires from accel-ppp's trace exactly one Protocol-Reject of IPv6CP, so the
client stopped sending it (RFC 1661 Section 5.7), no LCP Terminate-Request
either way, and exactly one CHAP success, so there was no second session.
`pppoe0` must hold no link-local address. The IPv4 session proof follows
(RFC 7084 WLL-3).

Both scenarios carry an `ipv6-session` file. Its presence makes the lab set
`net.ipv6.conf.default.disable_ipv6=0` in both containers, so the PPP devices
they create at run time keep IPv6, and run Ze under the lab's net-sysctl
grant, because the client writes its session device's sysctls each time a
session creates it (`scenarios.go`, `ipv6SessionScenario`). accel-ppp has no
command that closes IPv6CP alone, and once IPv6CP has opened accel-ppp ends
the whole session when it closes (`ipv6cp_layer_finished`), so an IPv6CP
close under a live IPCP stays proven by the `pppoeclient` unit tests.

### ipv6cp-zero-identifier and ipv6cp-missing-option

Written and registered against RFC 5072 Section 4.1's zero-identifier Nak
and its missing-option one-shot Nak, driven by a real pppd client and
read off the wire (`check_ipv6cp.go`), the same shape `pppoe-padr-replay`
established. pppd cannot be made to send either request: its `ipv6`
option refuses the zero identifier ("Illegal interface identifier (local)",
`setifaceid` in pppd's `ipv6cp.c`), and it always sends the option. So both
checkers capture pppd's own IPv6CP Configure-Request and replay rebuilt
copies of it: the zero-identifier checker sends the zero value, then the
value Ze's Configure-Nak suggested, and wants a Configure-Nak and then a
Configure-Ack from Ze's MAC. **Neither can pass today.** `poolPlugin.handle`
(`internal/component/l2tp/plugins/pool/register.go`) answers every
`EventIPRequest` whose family is not IPv4 with `Accept: false`, and
`runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads that decline
before the session reads a single client frame, so Ze's IPv6CP FSM never
starts, on any configuration. Both scenarios use the pinned pppd peer: this
lab's client image carries pppd and rp-pppoe, and accel-ppp-as-client is not a
role its images support. Both checkers return a citing error at the first
missing wire evidence rather than passing vacuously.
`plan/post-release/features/spec-l2tp-ipv6-subscriber.md` is what switches them on.

## Relationship to other evidence

| Evidence | Ze role | Independent peer | Kernel PPPoE |
|----------|---------|------------------|--------------|
| `test/pppoe/pppoe-basic.ci` | Access concentrator | Functional fixture | No |
| `test/pppoe/pppoe-vlan.ci` | Access concentrator on VLAN | Functional fixture | No |
| `test/pppoe/pppoe-service-name.ci` | Access concentrator | Functional fixture | No (`option=netns-link`, `./le test qemu pppoe`) |
| `./le test deployment docker-pppoe-accel` | Client and access concentrator | accel-ppp and pppd | Host kernel |
| `./le test qemu pppoe-accel` | Client | accel-ppp | Runtime kernel |

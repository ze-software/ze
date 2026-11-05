# L2TP PPP/NCP Docker Interop Lab

Peer-isolated Docker lab for full L2TP PPP/NCP/kernel dataplane evidence.

## Overview

The lab runs Ze as an LNS, a real `xl2tpd`/`pppd` LAC, and optionally FRR
as a BGP peer in separate Docker containers on an isolated bridge
network. No container runs privileged: Ze and the LAC hold `NET_ADMIN` and the
host's `/dev/ppp` device, and neither loads a kernel module. It proves the complete path from L2TP control tunnel through PPP
LCP/IPCP, kernel `pppN` interface creation, dataplane connectivity, and BGP
route redistribution from a live PPP session.

## Layout

```
test/interop-l2tp/
  Dockerfile.ze        Ze LNS image (Alpine + iproute2 + kmod + ppp; copies the
                       ze-linux the suite preflight cross-compiles)
  ze-linux             the staged daemon, git-ignored and rewritten each run
  Dockerfile.lac       LAC image (Alpine + xl2tpd + ppp + iproute2)
  daemons              FRR daemons config (zebra + bgpd)
  vtysh.conf           FRR vtysh config
  scenarios/
    01-ppp-ipv4/       PPP IPv4 dataplane proof
    02-ppp-bgp-redistribute-frr/   BGP route redistribution proof
    03-ze-lac-xl2tpd-lns/   ze as initiator (LAC) vs real xl2tpd LNS
    04-radius-acct-attrs/   subscriber attributes in RADIUS auth and accounting
    dns-resolver-l2tp-subscriber/   IPCP DNS to Ze's LAN resolver, then a lookup
    l2tp-unknown-mandatory-avp-xl2tpd/   unknown AVP, M=0 then M=1, through the relay
    ppp-code-reject-xl2tpd/   unknown LCP and IPCP Codes injected through the relay
    l2tp-lns-mru-xl2tpd/   configured MRU, CHAP, live daemon output, and traffic
    l2tp-lns-omitted-mru-xl2tpd/   default peer MRU and Ze-side transmit cap
    l2tp-lns-negotiated-mru-xl2tpd/   unequal configured/negotiated session output
    l2tp-lns-chap-only-xl2tpd/   PAP Nak refused before authentication
    l2tp-proxy-mru-transmit-cap/   accepted proxy AVPs, kernel MTU, and ICMP
    l2tp-radius-acct-wire-fixture/ original accounting fixture regression
internal/le/interoplab/l2tp/
  l2tp.go              Native topology, images, selection, and lifecycle
  checkers.go          Typed protocol assertions for the scenarios
  dns_resolver.go      The dns-resolver-l2tp-subscriber assertions
  mandatory_avp.go     The l2tp-unknown-mandatory-avp-xl2tpd assertions
  ppp_code_reject.go   The ppp-code-reject-xl2tpd assertions
  lns_policy.go        MRU and CHAP-only typed assertions and daemon queries
  avprelay/            UDP relay between the LAC and Ze, on 172.29.0.4
  radiusmock/          Independent Go RADIUS peer
```

Each scenario contains the protocol configuration mounted by its typed plan.
The plans choose the Ze, xl2tpd, FRR, and RADIUS peers for that scenario, while
the checker map fixes the complete scenario population. The
`dns-resolver-l2tp-subscriber` plan adds Knot DNS on `172.29.0.7`, and the
`l2tp-unknown-mandatory-avp-xl2tpd` and `ppp-code-reject-xl2tpd` plans put the
relay on `172.29.0.4`, the address their xl2tpd dials as its LNS.
The `l2tp-lns-*` plans also start the independent RADIUS backend on
`172.29.0.5` before Ze and the LAC. The positive plan requires a credential
request there; the refusal plan requires none. Both disable xl2tpd redial and
pppd persistence, so the evidence belongs to one attempt.

## Prerequisites

The lab requires Docker and a host kernel with PPPoL2TP support, built in
or loaded by the host: a lab container never loads a kernel module. The
preflight check probes for `/dev/ppp`, `ip l2tp`, and the `l2tp_ppp` or
`pppol2tp` kernel module from inside a temporary container that holds the
peers' grants (`NET_ADMIN`, `--device /dev/ppp`) and nothing more. If any
requirement is missing, the runner exits non-zero, names what is missing, and
prints the host command that loads it:

```
sudo modprobe -a ppp_generic l2tp_ppp l2tp_netlink
```

Ze's kernel builds all three in, so `./le test qemu docker-lab` needs none.

Docker Desktop on macOS typically cannot pass this check because its Linux
VM lacks PPPoL2TP kernel modules. The runner does not skip or downgrade;
it fails strictly.

Setting `ZE_L2TP_SKIP_KERNEL_PROBE` or `ze.l2tp.skip-kernel-probe` in the
environment causes an immediate refusal.

## Running

```
./le test deployment docker-l2tp-ppp
ZE_L2TP_INTEROP_SCENARIO=01-ppp-ipv4 ./le test deployment docker-l2tp-ppp
```

To exercise working-tree changes, use a named build:

```
ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-mru-xl2tpd ./le --name session test deployment docker-l2tp-ppp
ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-chap-only-xl2tpd ./le --name session test deployment docker-l2tp-ppp
```

Environment variables: `FRR_IMAGE` (default
`quay.io/frrouting/frr:10.3.1`), `NO_BUILD`, `SESSION_TIMEOUT` (default 90s),
`ZE_L2TP_INTEROP_SCENARIO`, and `ZE_L2TP_INTEROP_SUFFIX` (default PID, for
parallel-run isolation).

## Scenarios

### 01-ppp-ipv4

Proves: L2TP tunnel establishment, PPP LCP/IPCP completion, kernel `pppN`
with correct local/peer addresses, dataplane ping from LAC to Ze through the
PPP tunnel, route inject/withdraw log presence, and clean L2TP/PPP teardown
(both containers return to empty `ip l2tp show tunnel` and `ip link show
type ppp`).

### 02-ppp-bgp-redistribute-frr

Proves: FRR establishes BGP with Ze, a PPP-assigned subscriber /32 appears
in FRR's BGP table via Ze's `redistribute destination bgp { import l2tp }` (real RouteObserver
and `redistribute-orchestrator` path), and the route is withdrawn from FRR
after LAC session teardown. BGP session stability is verified after
withdrawal.

### 03-ze-lac-xl2tpd-lns

The inverse topology: **ze is the L2TP initiator (LAC/dialer)** and a real
`xl2tpd` runs as the LNS answerer. This proves ze's initiator half of the
tunnel FSM (SCCRQ initiation → SCCRP handling → SCCCN → established)
interoperates with an independent RFC 2661 implementation. Both sides confirm
it: ze logs `tunnel now established (initiator)`, and xl2tpd logs
`Connection established ... LNS session is 'default'`. ze is triggered to dial
by the `request l2tp outgoing-call` RPC over its token-guarded REST API.

The typed plan starts `xl2tpd` and Ze in containers on the isolated lab bridge.
Ze's L2TP listener binds `0.0.0.0`, because Ze dials from its listener socket
and a loopback listener cannot reach the LNS on the bridge;
`TestZeL2TPListenerReachesTheLabBridge` refuses a fixture that binds one.
The checker triggers Ze through its REST API and requires both peers to report
the established tunnel. The PPPoL2TP preflight is skipped when this
control-plane-only scenario is selected alone. `xl2tpd` cannot answer the OCRQ
that follows because it has no outgoing-call answerer (it logs `Unimplemented
message 7`), so the RPC returns an error by design and the established control
connection is the interop proof. The full OCRQ→OCRP→OCCN call flow is proven
functionally by `test/l2tp/lns-outgoing-call.ci`. The LAC incoming-call PPP data
plane is covered by `./le test deployment gokrazy-l2tp-ppp`.

### 04-radius-acct-attrs

Proves the subscriber attributes ze sends to a RADIUS server for a real PPP
session: NAS-Port-Id (RFC 2869 Section 5.17) resolved from the operator's
`nas-port-id-format` in the Access-Request and in the accounting records, and
Framed-IP-Address (RFC 2865 Section 5.8) in the Accounting-Start carrying the
address pppd actually negotiated (RFC 2866 Section 4.1). The assertion reads
what the server decoded, not what ze logged.

The Start record also proves the session attributes ze adds to every
Accounting-Request: Event-Timestamp within an hour of the lab clock, and
Acct-Delay-Time present. It proves two absences as well. Acct-Terminate-Cause
is absent, which RFC 2866 Section 5.10 requires of every record that is not a
Stop. Calling-Station-Id is absent because xl2tpd's ICRQ carries Message Type,
Assigned Call ID, Call Serial Number and Bearer Type alone, so no Calling
Number AVP reaches ze, and RFC 2865 Section 5 forbids sending a text attribute
of length zero.

The checker then POSTs `clear l2tp session all` to ze's REST surface on
`127.0.0.1:17012` and reads the Stop record that follows. That teardown is the
operator one, so the Stop reports Admin Reset, RFC 2866 Section 5.10 value 6.

The typed plan starts the Go RADIUS peer on `172.29.0.5` before Ze and xl2tpd.
The peer answers Access-Request and Accounting-Request packets and writes the
decoded attributes that the checker compares. Its Access-Accept carries a
Message-Authenticator (RFC 2869 Section 5.14), because ze discards an unsigned
answer to an Access-Request unless the server's
`require-message-authenticator` is false (BlastRADIUS, CVE-2024-3596).
This scenario uses the same PPPoL2TP preflight as 01 and 02.
<!-- source: internal/le/interoplab/l2tp/radiusmock/main.go -- appendMessageAuthenticator -->

### dns-resolver-l2tp-subscriber

Proves that a subscriber learns Ze's LAN DNS resolver through IPCP and
resolves through it. The pool's `dns-primary` and `dns-secondary` are the
pool gateway 10.100.0.1, where `service dns-resolver` listens with
`allow-from 10.100.0.0/24` and forwards to Knot DNS on `172.29.0.7`, the only
server that holds the lab zone `lab.example.` (the BGP lab's
`test/interop/Dockerfile.knot`). pppd runs with `usepeerdns`, so it asks for
both RFC 1877 options and writes what Ze gave it to `/etc/ppp/resolv.conf`.

The checker waits for `PPP session up`, requires every `nameserver` line of
that file to be 10.100.0.1, requires the LAC's route to 10.100.0.1 to leave
through its `ppp` interface, and requires busybox
`nslookup -type=a www.lab.example 10.100.0.1` to answer from that server with
exactly 192.0.2.10 and 192.0.2.11. The gateway exists only while a session is
up. The resolver sets `IP_FREEBIND` on its listener sockets, so the listener
binds at start, before the first session, and Ze's container needs no sysctl.

`auth-method chap-md5` in this scenario's `ze.conf` is load-bearing, unlike the
same leaf in `test/l2tp/radius-acct-wire.ci`. xl2tpd sends no RFC 2661 Section
4.4.5 proxy LCP AVPs, so ze negotiates LCP itself and the leaf picks the
method. Read in xl2tpd v1.3.18 and v1.3.19: `avpsend.c` has no builder for AVP
26, 27 or 28, and the ICRQ and ICCN branches of `control.c` carry none. The
authentication has to reach a credential, because RFC 2865 Section 4.1 admits
no Access-Request without one.

### ppp-code-reject-xl2tpd

Proves Ze's PPP Code-Reject against a real xl2tpd and pppd session (RFC 1661
Section 5.6). pppd requests an MRU of 128. Once a ping has crossed the
session, the relay sends Ze, as the LAC, two LCP packets with the unknown Code
99 and two IPCP packets with Code 12, all with Identifier 0x42 and 204 octets
long. The checker reads Ze's answers off the wire. The two LCP and the two
IPCP Code-Rejects carry different Identifiers within each protocol, each fits
the 128-octet MRU with a copy of the start of the probe it rejects, and the
session stays up. The IPCP answers come from the PPP unit descriptor, where
Linux queues IPCP once the channel is connected. Last, the relay sends one
IPCP Terminate-Request and Ze must answer with a Terminate-Ack carrying its
Identifier. The full
account is in `docs/architecture/testing/interop.md`, "L2TP and PPPoE
lifecycle fault scenarios".

### l2tp-lns-mru-xl2tpd

Ze is the LNS with `mru 1492`, `auth-method chap-md5`, and
`authentication { allow [ chap-md5 ]; }`. The checker requires pppd's decoded
trace to show Ze's LCP Configure-Request containing both MRU 1492 and
CHAP-MD5, and pppd's Configure-Ack with the same Identifier and options.
The client explicitly requests MRU 1493. The trace must show Ze's Nak to
1492 with that request's Identifier, pppd's retry at 1492, and Ze's Ack with
the retry's Identifier. Only after both LCP directions converge does the
checker accept CHAP Challenge, Response, and Success in order. It requires no PAP
protocol packets, a measured client `pppN` MTU of 1492, an independent
RADIUS Access-Request, and a ping across the live PPP session.

The client does not set an MTU cap. In pppd 2.5.5, `lcp_addci` omits the MRU
option at the default 1500, but emits an explicit 1493. `lcp_nakci` accepts
Ze's suggestion of 1492. This input makes the promised Nak observable.
The client's receive MRU and Ze's receive MRU remain distinct facts. In particular,
`show l2tp session id <id>` reports the **peer receive MRU**, which is
separate from the local transmit cap and configured local receive request.
The checker reaches the running daemon through the token-guarded REST
command dispatcher and requires:

- `show l2tp config`: `mru` 1492 and `authentication-allow` `[chap-md5]`.
- `show l2tp sessions`: exactly one live session with a nonzero ID.
- `show l2tp session id <id>`: the same session, username `alice`, assigned
  address `10.100.0.2`, actual negotiated `mru` 1492 and `auth-method`
  `chap-md5`. Missing or unknown negotiation state fails the assertion.

The wire request is load-bearing: the client's own MRU can limit its MTU
even if Ze ignores the new leaf, so an MTU-only assertion would be vacuous.
The checker retains the decoded negotiation, measured link JSON and daemon
answers in the job log, then stops the LAC and requires both peers' kernel
PPP links and L2TP tunnels to disappear.
<!-- source: internal/le/interoplab/l2tp/lns_policy.go -- checkLNSMRU, lnsMRUEvidence, lnsClientMTU, checkLNSPolicyOutput, lnsNegotiatedSession -->

### l2tp-lns-omitted-mru-xl2tpd

Ze again has a 1492 session cap, while pppd uses its default receive MRU
and omits the MRU option. The checker requires actual peer Configure-Request
records without MRU, a matching Configure-Ack, Ze's separate 1492/CHAP
request and its Ack, and a successful CHAP exchange. A configured option
or a diagnostic line alone cannot prove omission on the wire.

The checker reads Ze's own `pppN` interface and requires MTU 1492. A
client-only measurement cannot establish Ze's transmit limit. The daemon's
effective config remains 1492, while the actual session's peer receive MRU
is 1500. The scenario also requires an independent RADIUS credential request
and useful traffic across the PPP session before complete kernel teardown.
These measurements distinguish peer negotiation from local send policy.
<!-- source: internal/le/interoplab/l2tp/lns_omitted_mru.go -- checkLNSOmittedMRU, lnsOmittedMRUEvidence -->
<!-- source: test/interop-l2tp/scenarios/l2tp-lns-omitted-mru-xl2tpd/ppp-options -- default MRU omission -->

### l2tp-lns-chap-only-xl2tpd

Ze is the LNS with `auth-method chap-md5` and no explicit allow-list, exercising
the derived CHAP-only default. pppd refuses CHAP-MD5, MS-CHAP, MS-CHAPv2 and EAP
but allows PAP. The checker requires one pppd connection attempt and the
following decoded packets in order:

1. Ze's CHAP-MD5 Configure-Request.
2. pppd's Configure-Nak suggesting PAP, with that request's Identifier.
3. Ze's Terminate-Request.
4. pppd's Terminate-Ack with that termination's Identifier.

Before Terminate-Request, the peer must not acknowledge a Ze LCP request:
in pppd's trace this is a `sent [LCP ConfAck ...]` with an Identifier from
a received Ze Configure-Request in the same connection attempt. Such an Ack
can let LCP reach Opened before a later refusal. Ze's independent Ack of the
peer's request is permitted: that alone leaves Ze in Ack-Sent. Diagnostic
mentions, pre-connection packets and unmatched Identifiers do not establish
that the peer acknowledged Ze's request.
<!-- source: internal/le/interoplab/l2tp/lns_policy.go -- lnsCHAPOnlyEvidence -->

Ze must never offer PAP after the Nak. No PAP protocol packet, CHAP
credential exchange, IPCP packet or assigned address may appear. The
checker then waits for pppd to exit without stopping the LAC, requires
the daemon's session list to become empty, and requires no `PPP session up`
event and no request of any kind at the independent RADIUS backend.
Only after these refusal assertions does it stop xl2tpd to remove the idle
control tunnel and require both peers' kernel PPP/L2TP state to be empty.
An absent session alone cannot pass this scenario.

The complete pppd trace is written to `/run/lns-policy-pppd.log` in the
isolated LAC container and copied to the native job log before teardown.
pppd's process exit is measured separately: its `Exit.` message goes to
syslog, not that trace file.
<!-- source: internal/le/interoplab/l2tp/lns_policy.go -- checkLNSCHAPOnly, lnsCHAPOnlyEvidence, lnsNoCredentialRequest, retainLNSPolicyEvidence -->

### l2tp-lns-negotiated-mru-xl2tpd

This separate scenario keeps Ze's configured MRU at 1492 while pppd requests
1400. The checker requires both directional requests and matching Acks in
pppd's decoded trace, followed by CHAP success, an independent RADIUS
credential request, and traffic through the session. It measures Ze's own
`pppN` transmit MTU as 1400.
<!-- source: internal/le/interoplab/l2tp/lns_negotiated_mru.go -- checkLNSNegotiatedMRU, lnsDistinctMRUEvidence -->

The daemon's effective configuration must still report 1492. Both
`show l2tp sessions` and the selected `show l2tp session id <id>` must report
the negotiated peer receive MRU of 1400 and `chap-md5`. Returning the
configured 1492 in either session view fails this proof, even when traffic
succeeds. The different values make a configuration copy distinguishable
from actual negotiation. The existing snapshot and command tests retain the
null-before-Opened/null-after-down behavior and distinct negotiated `none`.
<!-- source: internal/le/interoplab/l2tp/lns_policy.go -- checkLNSPolicyOutputForMRU, lnsNegotiatedSessionMRU -->
<!-- source: internal/le/interoplab/l2tp/lns_negotiated_mru.go -- checkLNSNegotiatedMRU -->
<!-- source: internal/component/l2tp/snapshot_test.go -- TestSessionSnapshotNegotiatedPPP -->
<!-- source: internal/component/l2tp/cmd/l2tp_test.go -- TestSessionJSONMRUAndAuthMethod -->

The MRU Nak and omitted-option scenarios remain separate attempts. Their
inputs do not replace this smaller-peer case. New checker tests include
positive negotiation plus wrong aggregate output, wrong detail output, and
an interface MTU copied from configuration. No live result is claimed by
the scenario definition alone.
<!-- source: test/interop-l2tp/scenarios/l2tp-lns-negotiated-mru-xl2tpd/ppp-options -- explicit peer MRU -->
<!-- source: internal/le/interoplab/l2tp/lns_negotiated_mru_test.go -- TestLNSMeasuredOutputUnequalMRUs -->

### Peer-source basis for the policy scenarios

The LAC image pins pppd **2.5.5** from the official GitHub tag archive and
xl2tpd **1.3.20** from its release archive, each with a SHA256 checksum.
The ppp archive contains autotools inputs, not `configure`. Upstream
`autogen.sh` generates the build system and runs `configure` with the
image's existing installation paths. Build-only autotools dependencies do
not enter the runtime image. The subscriber RADIUS LAC uses the same release
pins, tag archive and autotools build procedure. Both images derive their
runtime library dependencies with `scanelf`; each keeps its suite-specific
runtime tools.
<!-- source: test/interop-l2tp/Dockerfile.lac -- release pins and build stage -->
<!-- source: test/interop-radius-subscriber/Dockerfile.lac -- release pins and build stage -->

Upstream lists [pppd 2.5.5 as its latest stable release](https://github.com/ppp-project/ppp/releases/tag/v2.5.5),
checked on 2026-11-04. `TestLACPeerReleasePinsAgree` compares both images'
pppd and xl2tpd release/hash declarations and rejects a missing or malformed
hash. This check catches paired-image drift; image builds and subscriber
scenarios still prove that the selected release operates correctly.
<!-- source: internal/le/interoplab/lac_images_test.go -- TestLACPeerReleasePinsAgree -->

After both native suites build their LAC images, the integration-tagged
`TestLACPeerRuntimeVersions` probes those images through the lab's owned
one-shot container lifecycle. It checks `pppd --version`, including stderr,
against each image's own Dockerfile pin and requires the PPPoL2TP plugin at
`/usr/lib/pppd/<version>/pppol2tp.so`. Missing Docker access, images or plugin
files fail the test. The real subscriber scenarios prove plugin loading and
PPP service separately.
<!-- source: internal/le/interoplab/lac_images_runtime_integration_test.go -- TestLACPeerRuntimeVersions -->

Historical native runs against pppd 2.5.5 passed both L2TP policy
scenarios on 2026-11-04. Each scenario failed under its rebuilt producer break
and passed again after restoration: ignored MRU configuration for the MRU
scenario, and the old broad authentication default for CHAP-only refusal.
Those runs preceded the subscriber RADIUS image alignment and are not proof
of that image's build or behavior.
<!-- source: test/interop-l2tp/Dockerfile.lac -- PPP_VERSION, PPP_SHA256, XL2TPD_VERSION, XL2TPD_SHA256, build stage -->

In pppd 2.5.5's
[`lcp_reqci`](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/lcp.c),
a refused CHAP request is Nak'd toward PAP when EAP is also refused.
`lcp_printpkt` prints the MRU and authentication options the checkers read;
`lcp_up` sets the client MTU from the peer's receive MRU, bounded by the
client's own settings. Its
[`fsm_rtermreq`](https://github.com/ppp-project/ppp/blob/v2.5.5/pppd/fsm.c)
acknowledges Terminate-Request in Req-Sent/Ack-Sent but keeps negotiating.
The refusal checker therefore waits for Ze's lower-layer teardown, not an
assumed immediate exit on Terminate-Request.
[`start_pppd`](https://github.com/xelerance/xl2tpd/blob/v1.3.20/xl2tpd.c)
creates the PPPoL2TP socket and launches the pinned pppd plugin with it.
These scenarios do not substitute a synthetic PPP peer.
<!-- source: test/interop-l2tp/Dockerfile.lac -- PPP_VERSION, PPP_SHA256, XL2TPD_VERSION, XL2TPD_SHA256 -->

### l2tp-proxy-mru-transmit-cap

This scenario runs the existing compiled `l2tp/proxy-mru-transmit-cap`
fixture inside Ze's container. The fixture shares that container's PPP
namespace, `NET_ADMIN` grant and `/dev/ppp` device. Its native LAC sends the
real ICCN proxy AVPs with Last-Sent MRU 1492 and Last-Received MRU 1500,
9000, or no MRU option. Each attempt completes local CHAP and IPCP, reads
the kernel interface MTU as 1492, and receives a full-size IPv4 ICMP
Echo-Reply through the PPPoL2TP data path. StopCCN must remove the PPP link
before the next attempt. This is a native kernel functional proof carried
by the interop suite; it does not substitute a synthetic peer for the
separate xl2tpd interoperability scenarios.
<!-- source: internal/test/fixture/tunnel_fixture_l2tp_proxy_mru.go -- tunnelL2TPProxyMRUTransmitCap -->
<!-- source: internal/le/interoplab/l2tp/proxy_mru.go -- checkProxyMRU, proxyMRUFixtureEvidence -->
<!-- source: test/interop-l2tp/scenarios/l2tp-proxy-mru-transmit-cap/ze.conf -- proxy policy and local credentials -->

The shared `StageBinaries` preflight builds a separate static Linux `le`
personality from the current checkout, with the same feature gates as the
daemon. The existing Ze image copies it to `/usr/local/bin/le`; its name
selects the registered fixture commands. The image carries no compiler,
and its daemon remains the normal `ze_core` build. The checker rejects a
missing helper, nonzero exit, usage output, or absent MTU, ICMP, CHAP,
IPCP or cleanup evidence.
<!-- source: internal/le/interoplab/l2tp/l2tp.go -- LabBinaries, prepareScenario -->
<!-- source: internal/le/interoplab/zebuild.go -- StageBinaries, stageBinaries -->
<!-- source: test/interop-l2tp/Dockerfile.ze -- separate static helper copy -->
<!-- source: internal/le/interoplab/l2tp/proxy_mru.go -- checkProxyMRU, proxyMRUFixtureEvidence -->

Select it with `ZE_L2TP_INTEROP_SCENARIO=l2tp-proxy-mru-transmit-cap`
when running `./le --name session test deployment docker-l2tp-ppp`.
The suite retains its normal Docker-host Ze-kernel prerequisite, image
staging, peer readiness and cleanup.
<!-- source: internal/le/interoplab/l2tp/l2tp.go -- runAt, scenarioCheckerMap -->

### l2tp-radius-acct-wire-fixture

This regression scenario runs the original registered fixtures from
`test/l2tp/radius-acct-wire.ci` inside the same Ze container and kernel
namespace. The existing shared proxy-handshake extraction also serves that
accounting peer, so its original CHAP, IPCP and decoded RADIUS assertions
remain part of the required proof. Its Last-Sent and Last-Received MRUs stay
1500; the separate scenario retains the default local cap and the original
NAS identifier, template, 60-second accounting interval, pool and link
monitoring settings.
<!-- source: internal/test/fixture/tunnel_fixture_l2tp_ppp.go -- tunnelL2TPRadiusAccountingPeer, checkRadius -->
<!-- source: test/l2tp/radius-acct-wire.ci -- original fixture invocations and config contracts -->
<!-- source: test/interop-l2tp/scenarios/l2tp-radius-acct-wire-fixture/ze.conf -- native carrier configuration -->

The checker uses the existing detached-exec operation because a complete
Interim-Update cannot arrive within the ordinary 30-second exec deadline.
It starts both original fixtures in a private shared directory, waits for
the mock's socket readiness, and polls the worker's published exit status.
The worker stops its mock before publishing that status. The checker
requires both fixtures' output, Access-Request, Start, Interim-Update and
Lost-Carrier Stop evidence, then observes removal of the kernel PPP link.
It preserves the original functional carrier's 240-second bound.
<!-- source: internal/le/interoplab/l2tp/radius_fixture.go -- radiusFixtureWorker, checkRadiusAccountingFixture, radiusAccountingFixtureEvidence -->
<!-- source: internal/le/interoplab/docker.go -- dockerCommandTimeout, Exec, ExecDetached -->

Select it with `ZE_L2TP_INTEROP_SCENARIO=l2tp-radius-acct-wire-fixture`
when running `./le --name session test deployment docker-l2tp-ppp`.
It reuses the proxy-cap scenario's staged helper and the suite's Ze image,
readiness, kernel prerequisite and cleanup.
<!-- source: internal/le/interoplab/l2tp/l2tp.go -- LabBinaries, scenarioCheckerMap, prepareScenario -->

## Relationship to Other Evidence

| Action | What it proves | PPPoL2TP required |
|--------|----------------|-------------------|
| `./le test deployment l2tp` | Control tunnel and incoming-call session | No |
| `./le test deployment l2tp-ppp` | Native Linux full PPP/NCP/kernel proof in peer-isolated netns | Yes |
| `./le test deployment docker-l2tp-ppp` | Peer-isolated Docker lab (this) | Yes for PPP scenarios |
| `./le test deployment gokrazy-l2tp-ppp` | QEMU gokrazy appliance LNS with a netns LAC | Yes |
| `test/plugin/redistribute-l2tp-*.ci` | Synthetic BGP UPDATE rendering | No |

The native proof and Docker lab catch different failure shapes. The native
proof isolates Ze and the LAC in Linux network namespaces joined by a veth
underlay; the Docker lab isolates them across a Docker bridge and adds the FRR
BGP redistribution scenario.

The gokrazy appliance proof reuses the native LAC shape but puts Ze behind the
same gokrazy/QEMU image used for appliance deployment. The appliance attaches
to a host bridge by TAP (user-mode slirp cannot deliver the LAC's inbound UDP
1701), so the LAC namespace still exercises a real host PPPoL2TP kernel path
while the appliance kernel provides Ze's LNS-side PPPoL2TP support. The image
build resolves ze's runtime kernel, the only kernel an appliance image carries.
Before it starts, the proof checks the runtime kernel cache entry for its
architecture and refuses, before boot, an entry that is absent or cannot carry
PPPoL2TP, naming the command that rebuilds it.
<!-- source: internal/le/test/deployment/actions.go -- Answer -->

## Design Pattern

The lab follows the repository's native interop pattern: checked-in scenario
configuration, a per-run Docker network with a unique suffix, fixed peer
addresses, typed lifecycle plans, and typed assertions. L2TP keeps its own
package because its peer roles, image mounts, RADIUS server, and PPP checks are
specific to this protocol.

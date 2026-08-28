# L2TP PPP/NCP Docker Interop Lab

Peer-isolated Docker lab for full L2TP PPP/NCP/kernel dataplane evidence.

## Overview

The lab runs Ze as an LNS, a real `xl2tpd`/`pppd` LAC, and optionally FRR
as a BGP peer in separate privileged Docker containers on an isolated bridge
network. It proves the complete path from L2TP control tunnel through PPP
LCP/IPCP, kernel `pppN` interface creation, dataplane connectivity, and BGP
route redistribution from a live PPP session.

## Layout

```
test/interop-l2tp/
  Dockerfile.ze        Ze LNS image (Alpine + ze + iproute2 + kmod + ppp)
  Dockerfile.lac       LAC image (Alpine + xl2tpd + ppp + iproute2)
  daemons              FRR daemons config (zebra + bgpd)
  vtysh.conf           FRR vtysh config
  scenarios/
    01-ppp-ipv4/       PPP IPv4 dataplane proof
    02-ppp-bgp-redistribute-frr/   BGP route redistribution proof
    03-ze-lac-xl2tpd-lns/   ze as initiator (LAC) vs real xl2tpd LNS
    04-radius-acct-attrs/   subscriber attributes in RADIUS auth and accounting
internal/le/interoplab/l2tp/
  l2tp.go              Native topology, images, selection, and lifecycle
  checkers.go          Typed protocol assertions for all four scenarios
  radiusmock/          Independent Go RADIUS peer
```

Each scenario contains the protocol configuration mounted by its typed plan.
The plans choose the Ze, xl2tpd, FRR, and RADIUS peers for that scenario, while
the checker map fixes the complete four-scenario population.

## Prerequisites

The lab requires Docker and a host kernel with PPPoL2TP support. The
preflight check probes for `/dev/ppp`, `ip l2tp`, and the `l2tp_ppp` or
`pppol2tp` kernel module from inside a temporary privileged container. If
any requirement is missing, the runner exits non-zero with a clear message.

Docker Desktop on macOS typically cannot pass this check because its Linux
VM lacks PPPoL2TP kernel modules. The runner does not skip or downgrade;
it fails strictly.

Setting `ZE_L2TP_SKIP_KERNEL_PROBE` or `ze.l2tp.skip-kernel-probe` in the
environment causes an immediate refusal.

## Running

```
./le deployment docker-l2tp-ppp-test
ZE_L2TP_INTEROP_SCENARIO=01-ppp-ipv4 ./le deployment docker-l2tp-ppp-test
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
The checker triggers Ze through its REST API and requires both peers to report
the established tunnel. The PPPoL2TP preflight is skipped when this
control-plane-only scenario is selected alone. `xl2tpd` cannot answer the OCRQ
that follows because it has no outgoing-call answerer (it logs `Unimplemented
message 7`), so the RPC returns an error by design and the established control
connection is the interop proof. The full OCRQ→OCRP→OCCN call flow is proven
functionally by `test/l2tp/lns-outgoing-call.ci`. The LAC incoming-call PPP data
plane is covered by `./le deployment gokrazy-l2tp-ppp-test`.

### 04-radius-acct-attrs

Proves the subscriber attributes ze sends to a RADIUS server for a real PPP
session: NAS-Port-Id (RFC 2869 Section 5.17) resolved from the operator's
`nas-port-id-format` in the Access-Request and in the accounting records, and
Framed-IP-Address (RFC 2865 Section 5.8) in the Accounting-Start carrying the
address pppd actually negotiated (RFC 2866 Section 4.1). The assertion reads
what the server decoded, not what ze logged.

The typed plan starts the Go RADIUS peer on `172.29.0.5` before Ze and xl2tpd.
The peer answers Access-Request and Accounting-Request packets and writes the
decoded attributes that the checker compares. This scenario uses the same
PPPoL2TP preflight as 01 and 02.

## Relationship to Other Evidence

| Action | What it proves | PPPoL2TP required |
|--------|----------------|-------------------|
| `./le deployment l2tp-test` | Control tunnel and incoming-call session | No |
| `./le deployment l2tp-ppp-test` | Native Linux full PPP/NCP/kernel proof in peer-isolated netns | Yes |
| `./le deployment docker-l2tp-ppp-test` | Peer-isolated Docker lab (this) | Yes for PPP scenarios |
| `./le deployment gokrazy-l2tp-ppp-test` | QEMU gokrazy appliance LNS with a netns LAC | Yes |
| `test/plugin/redistribute-l2tp-*.ci` | Synthetic BGP UPDATE rendering | No |

The native proof and Docker lab catch different failure shapes. The native
proof isolates Ze and the LAC in Linux network namespaces joined by a veth
underlay; the Docker lab isolates them across a Docker bridge and adds the FRR
BGP redistribution scenario.

The gokrazy appliance proof reuses the native LAC shape but puts Ze behind the
same gokrazy/QEMU image used for appliance deployment. The appliance attaches
to a host bridge by TAP (user-mode slirp cannot deliver the LAC's inbound UDP
1701), so the LAC namespace still exercises a real host PPPoL2TP kernel path
while the appliance kernel provides Ze's LNS-side PPPoL2TP support. The proof
resolves that kernel itself because the pinned rtr7 kernel has no L2TP support.
It validates an operator-supplied kernel package or materialises the runtime
kernel from the durable cache and fails before boot when neither can carry
PPPoL2TP.
<!-- source: internal/le/deployment/actions.go -- Answer -->

## Design Pattern

The lab follows the repository's native interop pattern: checked-in scenario
configuration, a per-run Docker network with a unique suffix, fixed peer
addresses, typed lifecycle plans, and typed assertions. L2TP keeps its own
package because its peer roles, image mounts, RADIUS server, and PPP checks are
specific to this protocol.

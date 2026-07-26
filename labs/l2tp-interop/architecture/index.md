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
test/l2tp-interop/
  run.py               Runner: preflight, image build, scenario selection
  lab.py               Docker lifecycle, helpers, FRR/PPP verification
  Dockerfile.ze        Ze LNS image (Alpine + ze + iproute2 + kmod + ppp)
  Dockerfile.lac       LAC image (Alpine + xl2tpd + ppp + iproute2)
  daemons              FRR daemons config (zebra + bgpd)
  vtysh.conf           FRR vtysh config
  scenarios/
    01-ppp-ipv4/       PPP IPv4 dataplane proof
    02-ppp-bgp-redistribute-frr/   BGP route redistribution proof
    03-ze-lac-xl2tpd-lns/   ze as INITIATOR (LAC) vs real xl2tpd LNS
```

Scenarios 01/02 (ze = LNS) contain `ze.conf`, `xl2tpd.conf`, `ppp-options`,
`l2tp-secrets`, and a `check.py` with a `check()` function. A scenario that
instead ships its own `run.py` (like 03) is self-contained: it manages its own
containers and system-under-test, and the runner delegates to it (skipping the
ze=LNS image build and the PPPoL2TP preflight).

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
make ze-deployment-l2tp-ppp-docker-test          # all scenarios
python3 test/l2tp-interop/run.py 01-ppp-ipv4     # single scenario
VERBOSE=1 python3 test/l2tp-interop/run.py       # debug output
```

Environment variables: `FRR_IMAGE` (default `quay.io/frrouting/frr:10.3.1`),
`VERBOSE`, `NO_BUILD`, `SESSION_TIMEOUT` (default 90s),
`ZE_L2TP_INTEROP_SUFFIX` (default PID, for parallel-run isolation).

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
`xl2tpd` runs as the LNS answerer. Proves ze's initiator half of the tunnel FSM
(SCCRQ initiation → SCCRP handling → SCCCN → established) interoperates with an
independent RFC 2661 implementation — confirmed on both sides (ze logs `tunnel
now established (initiator)`; xl2tpd logs `Connection established ... LNS session
is 'default'`). ze is triggered to dial by the `request l2tp outgoing-call` RPC
over its token-guarded REST API.

Self-contained (`run.py`): `xl2tpd` runs in Docker (`--network host`); `ze` runs
from `bin/ze` with isolated filesystem storage. Control-plane only, so it needs
no PPPoL2TP modules and runs unprivileged. `xl2tpd` cannot answer the OCRQ that
follows (it has no outgoing-call answerer — logs `Unimplemented message 7`), so
the RPC returns an error by design; the interop proof is the established control
connection. The full OCRQ→OCRP→OCCN call flow is proven functionally by
`test/l2tp/lns-outgoing-call.ci`. The LAC incoming-call PPP data plane (kernel
channel bridge, A-4) is env-blocked — see the scenario README and
`make ze-qemu-l2tp-ppp-test`.

## Relationship to Other Evidence

| Target | What it proves | PPPoL2TP required |
|--------|---------------|-------------------|
| `make ze-deployment-l2tp-test` | Control tunnel + incoming-call session (skip-kernel-probe) | No |
| `make ze-deployment-l2tp-ppp-test` | Native Linux full PPP/NCP/kernel proof in peer-isolated netns | Yes |
| `make ze-deployment-l2tp-ppp-docker-test` | Peer-isolated Docker lab (this) | Yes (host kernel) |
| `make ze-deployment-gokrazy-l2tp-ppp-test` | QEMU gokrazy appliance LNS with real netns LAC | Yes (host LAC side and appliance kernel) |
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
resolves that kernel itself: the pinned rtr7 kernel has no l2tp support, so
the script validates `KERNEL_PKG` or materializes the runtime kernel from the
durable cache, and fails fast with the `make ze-kernel KERNEL_ARCH=<arch>`
command when it cannot.
<!-- source: scripts/evidence/effective-gokrazy-l2tp-ppp.py -- resolve_kernel_pkg, qemu_command -->

## Design Pattern

Follows the `test/interop/` BGP interop pattern: scenario directory with
daemon configs, per-run Docker network with PID suffix, fixed container IPs,
`atexit` global cleanup, and `check.py` assertion scripts imported by the
runner. The L2TP lab is a separate module because the BGP interop has
domain-specific names, images, and daemon helpers.

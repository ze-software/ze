# PPPoE Docker Interop Lab (Ze client vs accel-ppp)

Peer-isolated Docker lab proving Ze's PPPoE **client** interoperates with a
real-world access concentrator: [accel-ppp](https://accel-ppp.org/), the
dominant open-source BRAS/AC.

## Why this exists (and why not L2TP)

The original request was an L2TP interop test against accel-ppp. That is not
buildable: an L2TP tunnel needs exactly one LAC (initiator) and one LNS
(responder), and **both Ze and accel-ppp are LNS-only**. Ze defers LAC mode
(`internal/component/l2tp/session_fsm.go`: "LAC-initiated incoming calls are
deferred to a later spec"), and accel-ppp's L2TP module is a documented LNS with
no supported client/LAC initiation. Two LNSes cannot form a tunnel.

PPPoE is the protocol where Ze and accel-ppp have *complementary* roles:
accel-ppp's first-class role is the **PPPoE server (access concentrator)**, and
Ze has a full RFC 2516 **PPPoE client** (`internal/component/l2tp/pppoeclient/`,
`pppoe-client` interface kind). This lab exercises exactly that pairing, which
no other test covers: the existing `test/pppoe/*.ci` tests run Ze as the
*server* against a synthetic client.

## Overview

Ze runs as a PPPoE client and accel-ppp as the AC in two privileged Docker
containers on an isolated user-defined bridge. Because PPPoE is an L2 protocol
(EtherType 0x8863/0x8864), the shared bridge gives the broadcast PADI a path to
the AC. The lab proves the complete client path: PADI/PADO/PADR/PADS discovery,
LCP, CHAP-MD5 authentication, IPCP address assignment, kernel `pppN` interface
creation with the server-assigned P2P address, dataplane ping to the AC
gateway, the AC's own session view, and clean teardown when the client stops.

## Layout

```
test/pppoe-interop/
  run.py               Runner: preflight, image build, scenario selection
  lab.py               Docker lifecycle, helpers, Ze/accel verification
  Dockerfile.ze        Ze PPPoE-client image (Alpine + ze + iproute2 + ppp + kmod)
  Dockerfile.accel     accel-ppp AC image (Alpine + apk add accel-ppp)
  entrypoint-accel.sh  modprobe ppp/pppoe, run accel-pppd in foreground
  scenarios/
    01-pppoe-chap-ipv4/  CHAP + IPv4 pool session proof
```

Each scenario contains `ze.conf` (Ze `pppoe-client` config), `accel-ppp.conf`
(AC config), `chap-secrets` (credentials), and a `check.py` with a `check()`
function imported by the runner.

## Prerequisites

Docker and a host kernel with PPPoE support. The preflight probes for `/dev/ppp`
and the `pppoe` (pppox `PX_PROTO_OE`) kernel module from inside a temporary
privileged container. If either is missing, the runner exits non-zero with a
clear message; it never skips or downgrades. Setting `ZE_PPPOE_SKIP_KERNEL_PROBE`
or `ze.pppoe.skip-kernel-probe` causes an immediate refusal.

Let the probe decide, rather than the host operating system. Docker Desktop on
macOS was measured on 2026-08-01 and its Linux VM DID supply both `pppoe` and
`/dev/ppp`, so an earlier claim here that it "typically cannot pass this check"
was wrong. Run the Docker path first. If the probe refuses, use the QEMU path
below. The accel-ppp image installs the Alpine `accel-ppp` package (the same
build the QEMU runner uses), so the image build is fast.

<!-- source: test/pppoe-interop/lab.py -- preflight_strict -->

Note that `l2tp_ppp` is a SEPARATE module and was absent on that same host, so
the sibling L2TP lab (`docs/labs/l2tp-interop.md`) can still refuse where this
one runs.

## Running

Two paths exercise the same Ze-client-vs-accel-ppp pairing. Prefer the QEMU path
on macOS or any host without PPPoE kernel support.

### Docker (host kernel)

```
make ze-deployment-pppoe-accel-docker-test               # all scenarios
python3 test/pppoe-interop/run.py 01-pppoe-chap-ipv4     # single scenario
VERBOSE=1 python3 test/pppoe-interop/run.py              # debug output
```

Environment variables: `VERBOSE`, `NO_BUILD`, `SESSION_TIMEOUT` (default 90s),
`ZE_PPPOE_INTEROP_SUFFIX` (default PID, for parallel-run isolation).

### QEMU (macOS-friendly, no Docker)

```
make ze-kernel GOKRAZY_ARCH=arm64      # once: build runtime kernel with CONFIG_PPPOE
make ze-qemu-pppoe-accel-test
```

This boots the runtime kernel in a QEMU Alpine VM, installs `accel-ppp` from
Alpine community, and runs `scripts/evidence/effective-pppoe-accel.py` -- the
netns sibling of this lab (Ze client + accel-ppp server in two namespaces joined
by a veth, no Docker). It is the canonical way to run this proof on a dev
machine. See `ai/rules/platform-linux.md` ("Interop Labs ... Need a QEMU Runner Too").

## Scenarios

### 01-pppoe-chap-ipv4

Proves: PPPoE discovery completes, LCP opens, Ze authenticates to accel-ppp with
CHAP-MD5, IPCP assigns the pool address (`10.11.0.2`) with the AC gateway
(`10.11.0.1`) as peer, exactly one kernel `pppN` interface appears in Ze with
that P2P address, accel-ppp's `show sessions` lists the subscriber, Ze pings the
AC gateway through the session, and accel-ppp drops the session after the Ze
client stops.

## Relationship to Other Evidence

| Target | What it proves | Ze role | Kernel PPPoE required |
|--------|---------------|---------|-----------------------|
| `test/pppoe/pppoe-basic.ci` | Server-side discovery (PADI/PADO/PADR/PADS) in a netns | Server (AC) | No (raw socket only) |
| `test/pppoe/pppoe-vlan.ci` | Server-side discovery over VLAN | Server (AC) | No |
| `make ze-deployment-pppoe-accel-docker-test` | Full client path vs a real AC, Docker (this) | Client | Yes (host kernel) |
| `make ze-qemu-pppoe-accel-test` | Same proof in QEMU netns (`effective-pppoe-accel.py`) | Client | Yes (runtime kernel) |

The Docker lab and the QEMU runner prove the same thing with the same peer
(accel-ppp from Alpine community); the QEMU runner is the one that works on a
macOS dev machine. Per `ai/rules/platform-linux.md`, a Linux-only interop lab must
ship both.

The `.ci` tests exercise Ze as the access concentrator with a synthetic Python
client; this lab is the only test where Ze is the *client* and the peer is a
real, independent AC implementation.

## Design Pattern

Follows the `test/l2tp-interop/` lab pattern (see `l2tp-interop.md`): scenario
directory with daemon configs, per-run Docker network with PID suffix, fixed
container IPs, `atexit` global cleanup, strict kernel preflight, and `check.py`
assertion scripts imported by the runner. It is a separate module because the
roles are inverted (Ze is the client here) and the peer daemon, image, and
helpers are PPPoE/accel-ppp specific.

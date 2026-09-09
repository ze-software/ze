# PPPoE Docker Interop Lab

This peer-isolated Docker lab proves both Ze PPPoE roles against independent
implementations. Ze acts as a client against
[accel-ppp](https://accel-ppp.org/), then as an access concentrator against
`pppd` with the `rp-pppoe` plugin.

## Overview

Each scenario runs privileged containers on an isolated bridge because PPPoE
uses Ethernet discovery and session frames (EtherTypes 0x8863 and 0x8864). The
host kernel must provide `/dev/ppp` and the `pppoe` pppox module.

The client scenario requires PADI/PADO/PADR/PADS discovery, LCP, CHAP-MD5,
IPCP, one kernel `pppN` interface, traffic to the access concentrator, the
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
    pppoe-empty-service-name/  Ze concentrator, pppd client, no service-name configured
    pppoe-padr-replay/         Ze concentrator, pppd client, max-sessions-per-mac 1
internal/le/interoplab/pppoe/
  pppoe.go             Native images, preflight, selection, and lifecycle
  scenarios.go         Role-selected container plans and mounts
  check_client.go      Ze client assertions
  check_ac.go          Ze access-concentrator assertions
  check_service_name.go  pppoe-empty-service-name: wire-level Service-Name proof
  check_padr_replay.go   pppoe-padr-replay: wire-level replay and per-MAC cap proof
```

The Dockerfiles keep their small amount of peer initialisation in the image
entrypoint command. There are no separate runner or entrypoint scripts.

## Prerequisites

The native preflight probes `/dev/ppp` and the `pppoe` module from a temporary
privileged container. It exits non-zero when either is absent, and setting
`ZE_PPPOE_SKIP_KERNEL_PROBE` or `ze.pppoe.skip-kernel-probe` causes an immediate
refusal.

The probe decides whether Docker can carry the proof. Docker Desktop on macOS
was measured on 2026-08-01 and its Linux VM supplied both requirements on that
machine. The QEMU action remains available for a host where the probe refuses.

<!-- source: internal/le/interoplab/pppoe/pppoe.go -- preflight -->

## Running

```
./le deployment docker-pppoe-accel-test
ZE_PPPOE_INTEROP_SCENARIO=01-pppoe-chap-ipv4 ./le deployment docker-pppoe-accel-test
ZE_PPPOE_INTEROP_SCENARIO=02-ze-ac-pppd-client ./le deployment docker-pppoe-accel-test
ZE_PPPOE_INTEROP_SCENARIO=pppoe-empty-service-name ./le deployment docker-pppoe-accel-test
ZE_PPPOE_INTEROP_SCENARIO=pppoe-padr-replay ./le deployment docker-pppoe-accel-test
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
./le qemu pppoe-accel-test
```

It boots the runtime kernel in QEMU and runs Ze and accel-ppp in two network
namespaces joined by a veth. That proof covers the client role only. The Docker
suite carries both roles.

## Scenarios

### 01-pppoe-chap-ipv4

Ze authenticates as `alice` with CHAP-MD5. IPCP assigns `10.11.0.2` with
`10.11.0.1` as its peer. The checker requires one Ze `pppN` interface with that
point-to-point address, a route through it, ICMP to the peer, an accel-ppp
session for `alice`, and removal of that session after Ze stops.

### 02-ze-ac-pppd-client

The independent pppd client requests service `internet`, authenticates as
`alice`, negotiates IPCP, and pings Ze through its PPP interface. The checker
reads pppd's discovery, bidirectional LCP, CHAP, and IPCP trace and compares it
with Ze's REST session table. It then requires PADT and empty state before
repeating the dial with `wrong-secret`; the second trace must reach CHAP and
receive a refusal without creating a session.

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
captured PADR frame from inside the client container's own network namespace
(`interoplab.SendFrameInNamespace`, the same AF_PACKET/netns mechanism the BGP
lab's IS-IS purge injector uses). It requires the PADS that replay provokes to
carry the SAME session id, never a second one. It then dials a second,
genuinely independent session from the same MAC and requires the PADS it
provokes to carry session id `0x0000` and an AC-System-Error tag, read off the
captured frame -- a session count that stayed at one is not enough evidence on
its own for either claim (`ai/rules/interop-and-goal-validation.md`, "Prove a
scenario discriminates"). `spec-pppoe-padr-replay-allocates-unbounded-sessions`.

## Relationship to other evidence

| Evidence | Ze role | Independent peer | Kernel PPPoE |
|----------|---------|------------------|--------------|
| `test/pppoe/pppoe-basic.ci` | Access concentrator | Functional fixture | No |
| `test/pppoe/pppoe-vlan.ci` | Access concentrator on VLAN | Functional fixture | No |
| `test/pppoe/pppoe-service-name.ci` | Access concentrator | Functional fixture | No (`option=netns-link`, `./le qemu pppoe-test`) |
| `./le deployment docker-pppoe-accel-test` | Client and access concentrator | accel-ppp and pppd | Host kernel |
| `./le qemu pppoe-accel-test` | Client | accel-ppp | Runtime kernel |

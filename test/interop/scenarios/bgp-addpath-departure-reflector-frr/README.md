# Scenario: bgp-addpath-departure-reflector-frr

A source departure through the route reflector, with FRR as an ADD-PATH
receiving client. RFC 7911 Section 2: "A BGP speaker that re-advertises a route
MUST generate its own Path Identifier to be associated with the re-advertised
route."

## What it proves

The raw injector announces 10.0.0.0/8 under Path Identifiers 11 and 12
(`inject.msg`). Ze reflects both paths through `bgp-rr` to FRR, which must hold
two paths: that proves ADD-PATH was negotiated and that Ze sent each path under
an identifier of its own. The checker then stops the injector. FRR must drop
every path of 10.0.0.0/8, which happens only when the departure withdraws the
identifiers Ze advertised to FRR rather than the source's. The last read
requires FRR's one session with Ze to be still up and never reset
(`Connections established 1; dropped 0`), because a reset would also clear the
paths.

With the source-DOWN recovery removed from `bgp-rr`, FRR still held
10.0.0.0/8 sixty seconds after the departure (assertion 4 red).

## Files

| File | Role |
|------|------|
| `ze.conf` | Ze as the route reflector, the injector and FRR both clients |
| `frr.conf` | FRR, which advertises ADD-PATH receive for IPv4 unicast by default, and one prefix of its own as the injector's readiness barrier |
| `inject.msg`, `inject-args` | The injector's UPDATE and its session options |

The checker is the `bgp-addpath-departure-reflector-frr` entry of
`scenarioOperations` in `internal/le/interoplab/bgp/checkers.go`.

Run it: `INTEROP_SCENARIO=bgp-addpath-departure-reflector-frr ./le test
integration interop`, or on a host without Ze's kernel, inside the Docker lab
guest: `./le test qemu docker-lab lab 'test integration interop' env
'INTEROP_SCENARIO=bgp-addpath-departure-reflector-frr'`.

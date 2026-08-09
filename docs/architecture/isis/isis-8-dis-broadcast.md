# IS-IS Designated IS and Pseudo-Node LSPs

Broadcast-LAN behavior (ISO/IEC 10589 clause 8.4.5): per-level designated IS
election, pseudo-node LSP origination by the elected DIS, the own-LSP star
encoding, re-election on DIS loss, and the LAN CSNP cadence.

The **protocol** behavior, including the election comparison, the pseudo-node LSP
contents and the star topology, is documented in
[`../wire/isis.md`](../wire/isis.md) under "DIS election and pseudo-node LSPs".
This page carries the structural decisions.

| Concern | File |
|---------|------|
| Election state machine and damping | `circuit/dis.go` |
| Pseudo-node origination and purge | `lsdb/pseudonode.go` |
| Engine glue | `dis_wiring.go` |

## Decision: election is pure

`DISState.Elect(candidates, damp, now)` compares candidates and returns the
role-transition flags. It reads no live table. The circuit gathers candidates
(the local node plus each up LAN neighbor) and the engine reacts to the flags.
State is per level, so level 1 and level 2 elect independently on the same
circuit.

<!-- source: internal/plugins/isis/circuit/dis.go -- DISState, Elect, the candidate gather and damping -->

## Decision: the pseudo-node reuses the ordinary originator

`OriginatePseudonode(level, info)` differs from ordinary origination only by a
non-zero pseudonode source ID and a metric-0 member stream. It uses the same
fragmenter, the same sequence and suspension state, and the same store insert.
The purge mirrors the ordinary stale-fragment purge. There is no second store and
no side channel.

<!-- source: internal/plugins/isis/lsdb/pseudonode.go -- OriginatePseudonode, PurgePseudonode -->

## Decision: the pseudonode ID is derived deterministically

The pseudonode octet is derived per `(circuit, level)` and folded to a non-zero
value, so a re-election reuses the same ID rather than churning distinct
pseudo-node LSP IDs, and level 1 and level 2 get distinct LAN IDs.

## Decision: election runs on transitions and on a one-second tick

Engine glue lives in `dis_wiring.go`, a sibling of `lsdb_wiring.go` and
`flooding_wiring.go`, rather than being threaded through the circuit or LSDB
packages. Election runs on the adjacency transition hooks **and** on a periodic
tick, because a DIS lost through the hold-timer sweep fires no hello and would
otherwise not trigger a re-election promptly.

<!-- source: internal/plugins/isis/dis_wiring.go -- the election trigger, pseudo-node originate and purge, the LAN CSNP cadence -->

## Decision: the star encoding is a small change in the level state

A broadcast circuit with a recorded pseudo-node and at least one up neighbor at
the level emits **one** TLV 22 entry pointing at the pseudo-node and skips the
per-peer entries. A LAN of N nodes then appears as one pseudo-node with N spokes
rather than an N-by-N mesh, in every node's database and SPF graph.

## Consequence: per-neighbor priority had to be threaded through

The DIS priority exists on the wire in the LAN hello, and it was being dropped
before the adjacency record. It is now carried through the adjacency, the hello
input, the state machine and the LAN receive path.

## Owned metrics

`ze_isis_dis_elections_total{level}` and `ze_isis_pseudonode_lsps{level}`.

## Trap: a runtime priority change does not reach a live circuit

Circuit reconcile marks the priority parameter changed, but the running circuit
keeps its build-time priority. So a priority-driven role transfer is not
reachable through reconcile. The election **logic** handles a priority change and
is unit-tested. The engine-level triggers that do work are DIS loss and a
higher-priority node joining.

## Trap: an abruptly-departed DIS cannot purge its pseudo-node

It ages out over max lifetime through the ordinary aging path. A DIS-loss test
must assert the **new** DIS comes up, not that the old pseudo-node is already
gone.

The purge-before-yielding path applies only to a node that loses the role while
still present, so test it with a higher-priority join rather than with a node
shutdown.

## Limit: a foreign DIS's pseudonode octet is not learned

The adjacency table does not surface a neighbor's advertised LAN ID, so a non-DIS
Ze node uses a deterministic placeholder pseudonode ID for the star edge. This
affects cross-vendor pseudonode-ID matching when the DIS is not a Ze node. It
does not affect Ze-to-Ze convergence, because the DIS originates the
authoritative pseudo-node LSP.

## Test harness

The shared lossless in-memory Layer-2 segment harness supports several engines on
one wire, which is what makes a three-node LAN DIS test runnable on a host with
no raw socket. Reuse it rather than building a second harness.

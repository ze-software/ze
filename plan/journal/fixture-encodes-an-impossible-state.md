# fixture-encodes-an-impossible-state

A functional fixture gives two roles the same identity because one host can hold
both: the same address on both ends of a session, one port for two listeners, one
identifier for two speakers. The protocol forbids the state, so the fixture asserts
behaviour about a deployment that cannot exist.

Nothing is wrong until a guard reasons about that identity. The guard is correct,
the fixture reddens, and the session that meets the red has to prove the guard
right before it can call the fixture wrong. The cheap move at that moment is to
weaken the guard, which is the move that must not happen.

The rule this class produced is
`ai/rules/points/testing/functional-test-gate/a-fixture-must-encode-a-topology-that-can-exist.md`.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-15 | - | bgp reactor egress, test/plugin | `test/plugin/relay-withdraw-nexthop-self.ci` gave its dest peer `remote ip 127.0.0.2` and `local ip 127.0.0.2`, so both ends of one BGP session held one address. `precomputeNextHop` (`internal/component/bgp/reactor/peer_forward_facts.go`) resolves `next-hop self` to `PeerSettings.LocalAddress`, and `buildForwardFacts` in the same file sets the guard's comparand from `PeerSettings.Address`, so the address on the wire WAS the peer's own address. `egressNextHopIsPeerOwn` (`internal/component/bgp/reactor/forward_next_hop.go`), added by commit 480897faf for RFC 4271 Section 5.1.3, therefore withheld the advertisement and the fixture's observer timed out waiting for it. A real session cannot reach that state: two ends of a TCP connection between two hosts hold different addresses, and loopback is the only reason the fixture could be written | fixed in the fixture, not in the guard: Ze's local address on the dest session became 127.0.0.3, distinct from both peers, and the asserted next-hop hex moved with it. Every assertion the file makes survives, including the withdrawal's exact-hex negative. The guard was left alone deliberately -- an exemption for "an address that is also Ze's own local address" could only ever fire on a fixture, because the RFC forbids the peer's address on the wire whoever else holds it. The suite that would have caught this at commit time is `make ze-functional-plugin-test`, which the surface table did not name for a Go change; it does now |
| 2026-08-15 | fixit-peer-process-event-filter | test/plugin | `test/plugin/attach-process-delivery-graph.ci` asserted its peer's initial-sync End-of-RIB (`expect=bgp:...00170200000000`) from a peer block that configures no `session { family { } }`. Ze then advertises no MP capability, negotiates no family, and `sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`) loops over `nc.Families()`, which is empty, so no marker can ever be written. `show bgp peer detail` reads `families: []` and `eor-sent: 0` for an established session. The fixture passed only while its observer shut the daemon down before the peer noticed the frame was missing | fixed in the fixture: peer1 configures `ipv4/unicast`, and the observer holds `api.wait_peer_eor_sent`, the barrier ze_api documents for any test whose peer asserts that frame. Both halves were needed -- the family makes the marker possible and the barrier makes the assertion honest |
| 2026-08-16 | - | hook phase-gate fixture | The human-model fixture inherited `CLAUDE_CODE_FORK_SUBAGENT` from the invoking harness, which changed its verdict. | The fixture clears the fork flag and adds payload-ID acknowledgement coverage. |
| 2026-08-16 | - | SubagentStart hook fixture | The hook fixture called SubagentStart with no event payload, a state Claude cannot produce. | The fixture passes JSON with the intended session ID. |

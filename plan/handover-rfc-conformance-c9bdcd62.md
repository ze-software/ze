# Handover: RFC conformance drive, session c9bdcd62

Stopped at 99% of the week's usage budget, not at a natural boundary. Everything
below is the state a successor needs; nothing here is a plan, only what is true.

## Where the gate stands

`./le rfc check` went from **80 violations to 2 that belong to this work**.

| Violation | Owner decision |
|---|---|
| `RFC4302-5-1`, `RFC4302-5-2` | Deferred by the owner until the AH kernel answer landed. It has now landed, so these are ready to be ruled on |
| `internal/plugins/ospf/iface`, `internal/plugins/ospf/lsdb` | Another session's packages do not type-check, which invalidates 26 tags there. Not this work's to fix |

`./le doc check links` is **green**, 32 dead references to 0. It is stage 24 of the
49, and this session wrongly called it non-gating early on.

## What is committed

23 commits, `af10938607` through `a8c9f9adfb`. Six of them are product fixes
rather than test or ledger work:

- **VRRP** sent advertisements with source `0.0.0.0` whenever no primary IPv4 had
  resolved, which is non-conformant and loses every sender-address tie-break in
  the election, so a ze router could lose a Master election it should win.
- **The software-version decoder** handed invalid UTF-8 to the renderer, because a
  Go string conversion never fails, so a peer could put arbitrary bytes into what
  an operator reads.
- **Ze advertised BGP capability 77** and had no code that could produce the
  link-local-only next hop it promises.
- **`ValidNextHopLens` admitted only `{4,16}`** for IPv4 unicast, so ze's own
  RFC 7606 validation judged the 32-octet next hop RFC 8950 Section 3 defines to
  be malformed and reset the session. A conformant peer doing IPv4-over-IPv6 with
  a link-local was dropped.
- **The shipped appliance kernel could not do AH at all** while ze negotiated
  `protoAH` and installed AH SAs. Measured red-to-green on ze's own image.
- **GTSM** sent its related ICMP errors at TTL 64 and accepted spoofed off-link
  ICMP errors against IPv4 sessions.

AES CCM for IKEv2 was implemented from scratch, since neither the vendored tree
nor `golang.org/x/crypto` carries a CCM mode, and is driven by all 24 RFC 3610
packet vectors byte for byte plus an interop scenario against strongSwan.

## The finding that outranks the rest

`buildIPsecSA` (`internal/plugins/ospf/ipsec_install.go`) installed ONE XFRM state
with `Src` and `Dst` both `net.IPv6zero`, under a comment claiming the kernel
resolves it for every OSPF destination. It does not: `xfrm_state_find` matches a
transport state on exact destination equality and `__xfrm6_state_addr_check`
wildcards the source only. **OSPFv3 with `ipsec` configured black-holes OSPF
entirely, and always has.** Measured: `XfrmOutNoStates` advanced by exactly 1 for
one datagram to ff02::5.

Every test passed because each asserted the params ze BUILDS or read the state
back with `ip xfrm`, both true of a state no packet can reach. A review then found
six more test groups of the same shape, ranked below.

The owner ruled: fix it as **multicast now, unicast per adjacency**, and fix all
six audit rows. Both were in flight when the budget ran out.

## What is uncommitted, and the one thing that is broken

An agent changed the signature of `buildIPsecSA` to take a destination and a
direction, and was stopped before updating its callers. `internal/plugins/ospf`
therefore does NOT compile: `ipsec_install_test.go` and `ipsec_rfc4302_test.go`
call it with the old two-argument form. That is half-finished work, not a defect
to diagnose.

Two other agents were stopped mid-edit in `internal/plugins/fib/kernel/`,
`internal/plugins/policyroute/`, `internal/plugins/firewall/` and
`internal/core/network/ttl_integration_linux_test.go`.

`internal/plugins/ospf/ipsec_pmtu_integration_linux_test.go` is committed and RED
for the product reason above. It carries the `RFC4302-3.3.4-1` tag, and the owner
chose to keep the tag and fix the product rather than untag it.

## The six audit rows, ranked

1. **FIB, 9 tests.** `zeRoutes` (`internal/plugins/fib/kernel/integration_linux_test.go`)
   keeps routes whose `Protocol == rtprotZE` and reads no metric, table or selected
   nexthop. The IP FIB is the product's output and no test in the tree observes a
   packet moving. `netlink.RouteGet` is already used in production by
   `routeForDestination` (`internal/plugins/iface/netlink/route_linux.go`).
2. **MPLS, 4 tests.** No interface has `net.mpls.conf.<iface>.input` set, and `emit`
   (`internal/component/iface/config_sysctl.go`) is its only producer, so no labeled
   frame could be accepted however the entries read back.
3. **Policy routing.** `applyIPRules` and `applyAutoRoutes`
   (`internal/plugins/policyroute/rules_linux.go`) have no test of any kind.
4. **CoPP and rate limits.** Assert the rendered ruleset; a term matching no packet
   renders identically.
5. **OSPF AH, 4 tests.** The case above.
6. **IPv6 minimum hop count.** `TestSetIPv6TTLAndMinHopReadback` asserts only the
   getsockopt value; its IPv4 twin forces a drop.

Cleared as already correct: the whole `internal/component/gtsm` suite, the XFRM
bypass and reqid tests, and `internal/component/sysctl`. They drive real traffic
and read kernel counters. `gtsm` was written around the trap that ze places a
rule's counter BEFORE its matches, so a counter read proves evaluation and not a
match (`plan/journal/counter-counts-the-wrong-packets.md`).

## Owner decisions on record

- AH kernel: build it with AH, rather than correcting the 20 `{lower-layer}` rows.
  Done and proven. The rows are still false in practice until the SA keying is
  fixed, because the layer performs nothing for OSPF traffic.
- `RFC4302-3.3.4-1`: keep the tag, fix the product.
- MS-CHAPv2 EMSK: derive one. Stopping the key-derivation claim was rejected,
  because `eapAuthSecret` (`internal/component/ike/engine/eap_auth.go`) keys the
  IKEv2 AUTH payload with the EAP MSK per RFC 7296 Section 2.16, so the change
  would have been wire-visible.
- GTSM transmit: per-peer route metric, conditional on the kernel library taking
  it. It does: `vishvananda/netlink` v1.3.1 carries `Route.Hoplimit`.

## Blockers on the push that are not conformance

- **927 verification-debt rows.** `./le commit debt-clear all of <m>` clears them
  after one green run.
- **`./le verify worktree` has never run green** this session. It was started once
  and killed by a terminal memory incident.
- **`fd00::2` is missing from lo0**, which reddens two reactor tests correctly.
  The scratch directory of this session holds `loopback-setup.sh`, which adds it;
  it needs a password and does not survive a reboot.

## Cautions for whoever picks this up

- Several sessions share this checkout. `internal/core/bgp/attribute/origin.go`,
  `internal/component/cli/model_keys.go` and
  `internal/component/config/system/system.go` were mid-edit elsewhere and broke
  lint typecheck repeatedly. They are not yours.
- A `test/rfc-changed/` row records the OWNER's decision and an author may not
  write one. A `test/weakened/` row is the author's own. `./le commit create`
  prunes the rfc shard to the rows of the commit it is preparing, so a row written
  ahead of time disappears when an earlier commit runs.
- Writing any of the five annotations (`{gap}`, `{not-applicable}`,
  `{lower-layer}`, `{feature-declined}`, `{single-polarity}`) is the owner's call,
  not an agent's.
- This host is darwin. A `//go:build linux` discrimination record cannot be
  observed here; a privileged `golang:1.27` container over an exported copy of
  HEAD works and was used successfully several times.

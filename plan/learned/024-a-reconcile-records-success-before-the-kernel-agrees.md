# 024 - A reconcile records success before the kernel agrees

**Spec:** spec-gtsm-related-icmp-ttl, closed 2026-09-15
**Class:** `plan/journal/record-written-before-the-operation-succeeds.md`,
`plan/journal/counter-counts-the-wrong-packets.md`

## What the work built

RFC 5082 Section 3 extends the TTL 255 rule to "the related ICMP error
handling messages", in both directions. `internal/component/gtsm` installs the
two pieces of kernel state the socket options cannot reach: a host route to
each GTSM peer carrying RTAX_HOPLIMIT 255 (`installHopLimitRoute`,
`route_linux.go`), so an ICMP error ze's kernel generates leaves at 255, and
the `ze_gtsm` nftables input table (`filterTables`, `peerTerms`, `gtsm.go`)
that drops an ICMPv4 error from the peer quoting a TCP header on the peer's
BGP port when its outer TTL is below the peer's floor. The BGP reactor
publishes the peer set at start and at every peer reconcile
(`publishGTSMKernelState`, `internal/component/bgp/reactor/gtsm.go`). The
IPv6 receive half was already the kernel's, on IPV6_MINHOPCOUNT. A doctor
check (`checkKernelState`, `doctor.go`) names a peer short of either half.

## Decisions

- Transmit is a route metric, never a sysctl: `net.ipv4.ip_default_ttl` would
  raise the TTL of every locally generated packet on the box.
- The IPv4 receive half is a filter, because `tcp_v4_err` compares IP_MINTTL
  against the TTL QUOTED inside the error, which the sender chooses.
- Every drop term requires the quoted BGP port, so an error about any other
  flow is Unknown and is never dropped (RFC5082-3-4).

## What the closure found

`SetPeers` wrote `current = wanted` after a route install FAILED, and its
unchanged-set short-circuit then skipped the retry that three comments and
the doctor message promised. A reconcile now records `routeMissing` and
re-runs an unchanged set while it is set. The tagged receive proofs read
`/proc/net/*`, which answers for the thread-group leader's namespace, not
the locked thread that unshared one: they passed or failed by thread
placement. They now read `/proc/thread-self/net/*`, and all four records in
`rfc/discrimination/rfc5082.json` were re-observed red on this host.

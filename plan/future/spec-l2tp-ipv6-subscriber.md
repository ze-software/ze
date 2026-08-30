# Spec: l2tp-ipv6-subscriber

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze accepts no IPv6 address request from an L2TP or PPPoE subscriber, so
IPv6CP never opens and no IPv6 service starts on a subscriber session.**

### Why this spec is in `plan/future/`

`plan/future/README.md` refuses defects. This is not one: it is a feature slice
that was never wired, and Ze does not claim to carry it.

**Owner ruling, 2026-08-30: the first release ships L2TP and PPPoE subscribers
IPv4-only.** The question put to him was whether the first release carries L2TP
IPv6 subscribers at all, given that wiring the path is a feature slice rather
than a fix. Deferring it is a scope decision, and it is his.

## Current behavior

Each link read at its producer:

- `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`)
  opens by refusing every address family that is not IPv4, answering
  `Accept: false` with the reason `IPv6 not supported by static pool`.
- That handler is the only one ever registered: `l2tp.RegisterPoolHandler` has
  exactly one caller, the `init()` in the same file. No config reaches it, and
  PPPoE shares the registry.
- `runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads the refusal as
  `declined`, sets `disableIPv6CP` and does not start the IPv6 NCP, so
  `ipv6cpState` stays at Initial. The comment is right that an independent NCP
  must not tear down a good session.
- `startIPv6Service` (`internal/component/l2tp/ppp/session_run.go`) runs only
  when `ipv6cpState` is `LCPStateOpened`.

Corroboration from outside the code: `plan/journal/silent-fall-through.md`
records pppd 2.5.1 giving up after nine IPv6CP Configure-Requests against Ze.

The sibling half is already recorded as limitation L-1 of
`plan/spec-radius-subscriber-attributes.md`: `session_run.go` passes a nil
prefix allocator and `l2tp.GetPrefixHandler` has no caller, so the DHCPv6-PD
server that the RA's M and O flags point subscribers at can delegate nothing.

## What this spec owes when it runs

- Accept an IPv6 address request only when the operator configured a pool that
  carries IPv6 prefix delegation. An unconfigured deployment must keep
  declining, so IPv4-only sessions behave exactly as they do today.
- Wire `GetPrefixHandler` and `GetPrefixReleaser` into `startIPv6Service`, so
  DHCPv6-PD answers with a prefix rather than NoPrefixAvail. Shipping the accept
  without the delegation was considered and rejected: it gives a subscriber an
  IPv6 service that hands out nothing.
- Discharge the two test rows that `spec-ppp-ra-refinements` could not (that
  spec closed on 2026-08-30 and its file is gone; `git log --diff-filter=D
  -- plan/spec-ppp-ra-refinements.md` finds its final state):
  a functional `.ci` proving a teardown emits the final zero-lifetime Router
  Advertisement, and an interop scenario proving a real client kernel drops its
  default route on it.

## What already exists and must not be rebuilt

The RFC 4861 send schedule and the cease advertisement are implemented, reviewed
and mutation-proven: `raSchedule` (`internal/component/l2tp/ppp/ra_schedule.go`),
`stopRASender` (`internal/component/l2tp/ppp/ra_send.go`) and the shared
arithmetic in `internal/core/ndp/schedule.go`. That code is correct and
unreachable. This spec makes it reachable; it does not rewrite it.

## Constraints on the test work

`test/l2tp/radius-acct-wire.ci` is the shape to follow for the functional row.
It carries `option=needs-linux:caps=net-admin` and drives a real kernel PPP
session to an IPCP-negotiated address inside the QEMU guest, which loads
`ppp_generic`, `l2tp_ppp` and `l2tp_netlink` at boot. Its peer fixture
`tunnelL2TPRadiusAccountingPeer`
(`internal/test/fixture/tunnel_fixture_l2tp_ppp.go`) is what extends to IPv6CP.
The cease advertisement returns on that same UDP socket as a PPP frame of
protocol 0x0057, so no packet capture is needed.

The interop row needs a host whose kernel carries `l2tp_ppp`. The development
machine's Docker VM has `ppp_generic` built in and no L2TP module anywhere in
`/lib/modules`, so the four existing L2TP scenarios cannot start there either.
That is a machine constraint, not a code one.

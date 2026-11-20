# Spec: l2tp-ipv6-subscriber

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Subscriber IPv6 is an explicit post-release extension; the first release is IPv4-only. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze accepts no IPv6 address request from an L2TP or PPPoE subscriber, so
IPv6CP never opens and no IPv6 service starts on a subscriber session.**

### Why this spec is in `plan/post-release/features/`

`plan/README.md`, "Start here", puts incorrect shipped behavior in
`release/fixes/` and additional capabilities in `post-release/features/`. This
is not a defect: it is a feature slice that was never wired, and Ze does not
claim to carry it.

**Owner ruling, 2026-08-30: the first release ships L2TP and PPPoE subscribers
IPv4-only.** The question put to him was whether the first release carries L2TP
IPv6 subscribers at all, given that wiring the path is a feature slice rather
than a fix. Deferring it is a scope decision, and it is his.

## Required Reading

- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md`, "IPv6CP" - the client-side IPv6CP automaton the server side does not have
- [ ] `docs/architecture/testing/interop.md` - the L2TP and PPPoE suites the scenarios below join

## Current Behavior

**Source files read:**
- [ ] `internal/component/l2tp/plugins/pool/register.go` - `poolPlugin.handle`, the only registered pool handler
- [ ] `internal/component/l2tp/ppp/ncp.go` - `runNCPPhase`, which reads the refusal as declined
- [ ] `internal/component/l2tp/ppp/session_run.go` - `startIPv6Service`, gated on IPv6CP Opened
- [ ] `internal/component/l2tp/pppoeclient/ipv6cp_client.go` - `clientIPv6CP`, the client side that works today

Each link read at its producer:

- `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`)
  opens by refusing every address family that is not IPv4, answering
  `Accept: false` with the reason `IPv6 not supported by static pool`.
- That handler is the only one ever registered: `l2tp.RegisterPoolHandler` has
  exactly one non-test caller, the `init()` in the same file. No config reaches
  it, and PPPoE shares the registry.
- `runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads the refusal as
  `declined`, sets `disableIPv6CP` and does not start the IPv6 NCP, so
  `ipv6cpState` stays at Initial. The comment is right that an independent NCP
  must not tear down a good session.
- `startIPv6Service` (`internal/component/l2tp/ppp/session_run.go`) runs only
  when `ipv6cpState` is `LCPStateOpened`.
- The gap is on the server side only, where Ze is the LNS or the PPPoE access
  concentrator. Ze's PPPoE client runs its own IPv6CP automaton (`clientIPv6CP`,
  `internal/component/l2tp/pppoeclient/ipv6cp_client.go`, design page
  `docs/architecture/l2tp/cpe-1-pppoe-client.md`, "IPv6CP") and negotiates it
  against accel-ppp today, in the interop scenarios `pppoe-ipv6cp-ze-client`
  and `pppoe-ipv6cp-rejected-ze-client` (`test/interop-pppoe/scenarios/`).

Corroboration from outside the code: `plan/journal/silent-fall-through.md`
records pppd 2.5.1 giving up after nine IPv6CP Configure-Requests against Ze.

The sibling half is already recorded as limitation L-1 of
spec-radius-subscriber-attributes (closed 2026-09-03). The pool plugin's
`init()` now registers a prefix handler and a prefix releaser
(`l2tp.RegisterPrefixHandler`, `l2tp.RegisterPrefixReleaser`), but no
session path consumes them: `session_run.go` still passes a nil prefix
allocator to `startIPv6Service`, `l2tp.GetPrefixHandler` is called only by the
`fakel2tp` test plugin, and `l2tp.GetPrefixReleaser` has no caller. The
DHCPv6-PD server that the RA's M and O flags point subscribers at therefore
delegates nothing.

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
- Write the `ppp-ipv6cp-negotiation` functional test, handed here at the closure
  of `spec-ipv6cp-accepts-and-proposes-a-zero-interface-identifier` on
  2026-09-09. It drives a subscriber session to IPv6CP Opened inside the QEMU
  guest and asserts the kernel holds no route whose next hop is `fe80::`. It
  belongs here rather than there because a subscriber session cannot negotiate
  IPv6CP at all while `poolPlugin.handle` declines the family, so the test can
  neither pass nor fail meaningfully until this spec runs.
- Run the two interop scenarios that spec left registered and blocked,
  `ipv6cp-zero-identifier` and `ipv6cp-missing-option`
  (`test/interop-pppoe/scenarios/`, checkers in
  `internal/le/interoplab/pppoe/check_ipv6cp.go`). They are written against the
  real target wire behavior and fail today with `errIPv6CPNeverEngaged`; this
  spec is what turns them from a documented block into a proof.
- Run the two L2TP interop scenarios moved here from
  `spec-ppp-ncp-independent-lifetime` (closed) on 2026-11-20
  (Thomas's decision, its C-3): they need a pool that accepts IPv6, so they
  cannot run before this spec. Both go in `test/interop-l2tp/scenarios/`, with
  xl2tpd and pppd 2.5.5 dialling Ze's dual-stack LNS through the relay, and both
  need IPv6 enabled on pppN at creation (`net.ipv6.conf.default.disable_ipv6=0`
  on the Ze and LAC containers, as `ipv6SessionArguments` does in the PPPoE lab).
  - `l2tp-lns-ipv4-only-pppd-rejects-ipv6cp`: pppd with `noipv6` Protocol-Rejects
    the IPv6CP Ze offers (0x8057); the wire log shows no IPv6CP from Ze after it,
    and the session stays up on IPv4 past a short `ncp timeout`.
  - `l2tp-lns-pppd-ipv6cp-terminate-after-up`: after up, the relay injects an
    IPv6CP Terminate-Request; Ze Terminate-Acks it, the IPv6 address and the
    Framed-IPv6-Route prefix leave pppN and `show subscriber id <id> detail`
    loses `ipv6-interface-id` while IPv4 stays; pppd renegotiates IPv6CP and all
    three return. It needs a relay marker for the IPv6CP Terminate-Request and a
    lab RADIUS peer that returns a Framed-IPv6-Route.

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

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A subscriber's PPP client, behind an L2TP LAC or as a PPPoE host, reaching the NCP phase of a session where Ze is the LNS or the access concentrator

### Transformation Path
1. `runNCPPhase` asks for an IPv6 interface identifier (`requestIPv6CPInterfaceID`), which the registered pool handler answers
2. `poolPlugin.handle` declines every non-IPv4 family today; this spec makes it accept when the operator configured a pool that carries IPv6 prefix delegation
3. On accept the IPv6 NCP starts and, once `ipv6cpState` is Opened, `startIPv6Service` runs with a prefix allocator from `l2tp.GetPrefixHandler` rather than nil
4. At teardown the prefix returns through `l2tp.GetPrefixReleaser`, and `stopRASender` emits the final zero-lifetime Router Advertisement

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP session to the pool plugin | The handler registries `l2tp.RegisterPoolHandler`, `RegisterPrefixHandler`, `RegisterPrefixReleaser`, shared by L2TP and PPPoE | No |

### Integration Points
- `poolPlugin.handle`, `poolPlugin.handlePrefix` and `releasePrefixBySubscriberID` (`internal/component/l2tp/plugins/pool/register.go`)
- The `startIPv6Service` call in `internal/component/l2tp/ppp/session_run.go`, which passes a nil prefix allocator today

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A subscriber session in the QEMU guest | → | `poolPlugin.handle` accepts IPv6, IPv6CP Opened | `ppp-ipv6cp-negotiation` functional test |
| A subscriber session teardown | → | `stopRASender` | the zero-lifetime Router Advertisement `.ci`, named at design |
| xl2tpd and pppd 2.5.5 dialling Ze's dual-stack LNS | → | IPv6CP on the LNS | `l2tp-lns-ipv4-only-pppd-rejects-ipv6cp`, `l2tp-lns-pppd-ipv6cp-terminate-after-up` |
| A PPPoE host | → | IPv6CP on the access concentrator | `ipv6cp-zero-identifier`, `ipv6cp-missing-option` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named at design | `internal/component/l2tp/plugins/pool/` | IPv6 accepted with a configured prefix-delegation pool, declined without one | |

## Files to Modify
- `internal/component/l2tp/plugins/pool/register.go` - accept IPv6 when a prefix-delegation pool is configured
- `internal/component/l2tp/ppp/session_run.go` - pass the prefix allocator to `startIPv6Service`
- The new functional tests and the two `test/interop-l2tp/scenarios/` directories, named at design

## Implementation Steps
1. **Phase: accept** -- the pool accepts IPv6 only with a configured prefix-delegation pool
2. **Phase: delegation** -- `GetPrefixHandler` and `GetPrefixReleaser` reach the session path
3. **Phase: functional** -- the zero-lifetime RA `.ci` and `ppp-ipv6cp-negotiation`
4. **Phase: interop** -- the two PPPoE scenarios pass, and the two L2TP scenarios are written and pass

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test unit pkg path ./internal/component/l2tp/plugins/pool/` and `./le test unit pkg path ./internal/component/l2tp/ppp/` green
- [ ] `./le test functional l2tp` green, with the two new functional tests
- [ ] The four interop scenarios above pass, each with a recorded red

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

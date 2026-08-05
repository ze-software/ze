# Deferrals — spec-fixit-otc-src-role-meta-fallback

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-04 | spec-fixit-otc-src-role-meta-fallback | **Ze puts an incomplete well-known attribute set on a relayed withdraw-only UPDATE, and FRR rejects it.** The forward rail synthesises an AS_PATH attribute for a payload that carries none: `(*ASPathEdit).recordPrepend` (`internal/component/bgp/wireu/aspath_slot.go`) takes its `else` branch when `spans.Find(AttrASPath)` misses, builds `&attribute.ASPath{}` from nothing and records one `OpGen`, driven from `forwardUpdateCore` (`internal/component/bgp/reactor/reactor_api_forward.go`) whose only predicate on that branch is `facts.isEBGP`. A source withdrawal of `attrLen=0000` therefore reaches an eBGP peer as `attrLen=0009`, AS_PATH present, ORIGIN and NEXT_HOP absent. RFC 4271 Section 4.3 says a withdraw-only UPDATE "will not include path attributes", and Section 6.3 exempts only "correct path attributes", which an incomplete well-known set is not. Observed against FRR 10.3.1 on 2026-08-04: it logs `Missing well-known attribute NEXT_HOP` and `rcvd UPDATE with errors in attr(s)!! Withdrawing route` for every relayed withdrawal. The route is still removed and the session survives, but RFC 7606 Section 5.2 permits a receiver to answer this shape with a session reset. The route-server rail is unaffected: `rsClient` leaves `Prepend` empty, so `recordTranscode` returns without an op and the withdrawal is relayed byte-identical. | Separable from this spec, and a different producer. This spec's subject is the OTC gate in `role/otc.go` (`payloadAdvertisesNLRI`), which is landed, proven by nine unit tests, two `.ci` tests and interop scenario 51, and does not depend on the AS_PATH shape. Found while building that interop scenario: on the forward rail a stamped and an unstamped withdrawal both produce the same FRR error, so the negative could not discriminate there; the scenario uses the route-server rail, where the withdrawal is byte-clean and an added OTC Attribute is the only possible difference. Fixing the AS_PATH shape means changing what `recordPrepend` does for an attribute-free payload, which changes the wire output of every eBGP forward rail and re-baselines `test/plugin/role-otc-fwd-withdraw.ci`. That is its own AC set. | plan/spec-rfc7606-5-1-2-relay-shape.md | done |

## Detail

`test/plugin/role-otc-fwd-withdraw.ci` pins the CURRENT shape (`attrLen=0009`,
AS_PATH `[65000]`, no attribute of code 35) because that is what ze emits today.
Its comment records the AS_PATH as a defect rather than as intended behaviour, and
says what the expectation becomes once the defect is fixed: `attrLen=0000`. The
assertion the test exists for -- no OTC attribute on a message that advertises no
route -- holds under either shape.

No other spec owned this. `plan/spec-rfc7606-5-1-2-relay-shape.md` (Status
`in-progress`) already owns withdraw-only UPDATE shape on a relay rail and pins the
attribute-free form in its own `.ci`, which makes it the destination rather than a
new spec.

## Resolution (2026-08-04)

Fixed at the source, in the destination spec's "Follow-On: withdraw-only relay
shape" section. `ASPathEdit.Record` (`internal/component/bgp/wireu/aspath_slot.go`)
resolves a payload with no reachable NLRI as transcode-only, so `recordPrepend` --
the only frame that can create an AS_PATH -- is unreachable for a withdraw-only
UPDATE, on both the plain forward rail and the route-server rail.

`test/plugin/role-otc-fwd-withdraw.ci` was re-baselined to `attrLen=0000` as its
own header said it must be, which is strictly stronger: it now refuses ANY path
attribute rather than one specific extra one.
`test/interop/scenarios/52-relay-withdraw-shape-frr` proves FRR 10.3.1 accepts the
relayed withdrawal, and reddens with FRR's own `Missing well-known attribute
NEXT_HOP` when the guard is narrowed to the End-of-RIB alone.

The Status cell above read `**landed 2026-08-04**` until 2026-08-05. That is a
state written as a sigil on a value, which no gate can read: the vocabulary
`deferral_shard_removal_problems` (`scripts/dev/commit_helper.py`) accepts is
`done`, `cancelled` or `resolved`, so the row counted as LIVE and the shard could
not be removed at closure. It now reads `done`. Re-verified against source on
2026-08-05: `ASPathEdit.Record` tests the empty-Prepend case FIRST, so a
route-server intent still lands on `recordTranscode`, and a NON-EMPTY prepend over
a payload with no reachable NLRI lands on `recordWithdrawOnly`. Either way
`recordPrepend` is unreachable, which is the property this row asked for.
`role-otc-fwd-withdraw.ci` passes in the plugin suite and scenario 52 passes.

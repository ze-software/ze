# Handover 06 -- wire-edit set: what is in flight, and how to converge it

Written 2026-08-01. **The working tree does not compile.** Read this before
running anything. Nothing here is lost; it needs reconciling, not redoing.

## What is committed and green

| SHA | Content |
|-----|---------|
| `02b74bf44` `3bb30c87b` | Tier 1 item T1-1, observability then behaviour, plus `test/plugin/modify-oversize-suppress.ci` |
| `8e67a9b03` | T1-2 pooled transcode buffer (adopt-handle), T1-4 single receive walk |
| `1d48f2edd` | T1-5 RFC 8669 multi-PrefixSID fix, T1-3 accumulator hoist, RFC ledger + audit repair |
| `94f6c579e` | The RFC 7606 Section 5.4 spec |
| `bbd53bf22` | wire-edit child 1 (immutable base + eager span index) and a second fail-open leak |

Every one of those was verified green at the time it landed: `make ze-test-bgp`
81/81, `golangci-lint` clean, `make ze-rfc-check` exit 0, `make ze-race-reactor`
0 data races.

## Why the tree is red now: I over-parallelised

I ran three implementation agents concurrently against ONE Go package. That was
my error. Their file reservations did not cover the seams they actually shared,
so the tree is mid-migration in three directions at once:

| Workstream | State | What it broke |
|-----------|-------|---------------|
| wire-edit child 2 (`plan/spec-wire-edit-2-edit-apply.md`) | **still running** | Migrating `filterapi.AttrModHandler` from `func(src, ops, buf, off) int` to `func(*AttrPlan)`. Until it finishes, `plugins/role/register.go`, `plugins/filter_community/register.go` and their tests do not compile. This is EXPECTED mid-migration, not damage. |
| RFC 7606 Section 5.4 (`plan/spec-fixit-rfc7606-5-4-...md`) | **still running** | Changed `locateMPNLRI`'s signature in `message/rfc7606.go`. |
| Review findings F1-F6 (`plan/spec-hotpath-alloc-round-4.md` AC-12..AC-15) | **finished** | Added `Reactor.policyFilterSeam` and changed `recordModifyFailure` to take a peer string. Its callers in `reactor_api_forward.go`, `forward_rs.go` and `reactor_api_batch.go` still pass `netip.Addr`, and `forward_modify_failure.go` references `netip` with no import. |

The three-way collision is concentrated in `recordModifyFailure` and
`AttrModHandler`. I reserved `forward_build.go` and `filterapi/` from the review
agent but NOT `forward_modify_failure.go`, which is where `recordModifyFailure`
lives. That gap is the direct cause.

## Converge in this order

1. **Let child 2 finish.** It owns the `AttrModHandler` migration; nothing else
   can compile until its signature change is complete across every registered
   handler. Do not start a fourth agent in this package.
2. **Reconcile `recordModifyFailure`.** Pick ONE signature. The review agent's
   version takes a peer string so it can rate-limit per site and log through
   `fwdLogger()`; that is the better shape (finding F3). Update the three callers
   to pass `facts.addr.String()` / `dest.Address.String()`, and add the `netip`
   import or drop the parameter type back.
3. **Reconcile `locateMPNLRI`** with whatever the Section 5.4 agent landed.
4. Then `make ze-test-bgp`, `golangci-lint`, `make ze-rfc-check`,
   `make ze-race-reactor`. Only then commit.

## Do not lose these, they are real work

- **F2 is a genuine bug fix**, not a test change: `rebuildWithAttrDiscard` derived
  merged-marker transitivity from the FIRST occurrence via `AttrFind`. A peer
  sending Prefix-SID twice with different Transitive bits got `0xC0` where
  draft-mangin-idr-attr-tombstone-00 Section 5.7 requires `0x80`. T1-5 is what
  made that path always reachable.
- **F1 closed a real coverage hole**: the ingress and egress modify-failure
  guards had no test from their own entry point, and the ingress one converts a
  modify failure into a route drop on the RECEIVE path.
- **F3 is only half done and says so**: the second log line lives inside
  `buildModifiedPayload`, which was reserved all session. Residual is N lines per
  UPDATE, not 2N.

## Still not started

wire-edit children 3, 4 and 5. Child 5 additionally needs a **fan-out benchmark**
before its A-5 can be answered at all: `ze-perf-bench` is single-peer and
provably cannot see the per-destination loop (proved 2026-08-01, which is why the
T1-3 frame was invisible).

## Two open items that are nobody's yet

- `TestFwdPool_BackpressureBehavior` fails under `make ze-race-reactor`
  (`-count=20`), passes 5/5 isolated, 0 data races. Per `ai/rules/completion.md`
  that is a BROKEN TEST waiting on a duration instead of a condition. It must be
  fixed, not recorded in `plan/known-failures/`.
- Tier 1 closure: the Review Gate artifact, the audit tables, A-1..A-6, the
  Deliverables contradiction (`grep make([]byte` versus the sanctioned
  above-pool-class fallback), the learned summary, and the two closure commits.

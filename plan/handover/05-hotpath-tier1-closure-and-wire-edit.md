# Handover 05 -- hotpath-alloc-round-4 closure, and the wire-edit set

Written 2026-08-01 by the session that implemented Tier 1. Receiving session:
follow `.claude/rules/session-start.md`, then read this in full before planning.

## Why this is a handover rather than a finished spec

`plan/spec-hotpath-alloc-round-4.md` is CODE-COMPLETE and committed, but NOT
closed. Closure needs an independent review, and the session that wrote the code
cannot give one (`ai/rules/planning.md`). Thomas chose a fresh session
over an in-session review pass for exactly that reason.

That session also ran its implementation on Opus 5 by explicit operator override
(`tmp/session/.model-ack-2546e79c-8d57-4803-b856-593a4da12c55`), which makes the
independence of the review matter more, not less.

## What landed

Four commits, all on `main`. Every one carries `--structural-red-ok` because two
OTHER concurrent sessions had the tree structurally red throughout.

| SHA | Content |
|-----|---------|
| `02b74bf44` | T1-1 observability: typed `modifyFailure`, `ze_bgp_update_modify_failed_total{reason}`, golden byte harness |
| `3bb30c87b` | T1-1 behaviour: five call sites fail closed, `test/plugin/modify-oversize-suppress.ci` |
| `8e67a9b03` | T1-2 pooled transcode buffer (adopt-handle), T1-4 single receive walk |
| `1d48f2edd` | T1-5 RFC 8669 fix, T1-3 accumulator hoist, RFC ledger + audit repair |

State at handover: `make ze-test-bgp` 81/81, `make ze-rfc-check` exits 0,
`make ze-race-reactor` clean at `-count=20`, `make ze-plugin-test` 528/528,
Tier 2 boundary untouched.

## Verify these overrides before closing (BLOCKING)

Every commit above asserts the red gates belong to other sessions:
`internal/component/bgp/plugins/rib/rib_rfc4271_mixed_update_test.go` (untracked,
does not compile) and the IKE session's `ai/RFC-REQUIREMENTS.md` work. **Re-check
that both have cleared before you close.** If they have, run a clean
`make ze-verify` and record it. If a red turns out to be ours, it is ours.

## What the spec got WRONG -- carry these forward

The spec was written 2026-07-28 without re-checking. Four claims were proved
false against producing functions. All are recorded in the spec, but a reviewer
should confirm rather than trust:

| Claim | Reality |
|-------|---------|
| A-2: "the three call sites read nil identically" | **FIVE** call sites. `forward_rs.go` and `reactor_api_batch.go` were never named. Found because making the return value mandatory turned the compiler into the enumerator. |
| A-3: the transcode buffer fits `acquireModBuf` | **BROKEN.** It is aliased into an ASYNCHRONOUS TCP write, so a `defer Put` recycles it under a pending write. Implemented with `adoptFwdHandle` instead. |
| "three exit paths return nil after work has begun" | TEN. |
| T1-4 saves a walk | Only on eBGP sessions with PrefixSID acceptance off. The gate `!isIBGP && !AcceptSRv6PrefixSID` was always there; the umbrella states the saving unconditionally. |

## Known gaps, stated rather than hidden

1. **T1-3 has no RS-rail `.ci`.** `test/plugin/modify-accumulator-per-peer-isolation.ci`
   rides the GENERAL forward rail only. The route-server rail's hoist is covered
   by a unit test and a shared-code argument, but there is no RS-fast-path twin
   (compare `bgp-rs-community-strip-multi-fastpath.ci`). Add one before claiming
   the item fully proven.
2. **T1-3 makes no throughput claim, and must not be read as one.** The profile
   captured 2026-08-01 (`tmp/perf-run/pprof/100000/`) does not contain the
   accumulator frame at all. It lands as a precondition of wire-edit child 2.
3. **`ze-perf-bench` cannot answer questions about the per-destination loop.**
   It is single-peer with almost no fan-out, so `forwardUpdateCore` and
   everything under it never appears. This blocks justifying OR refuting several
   umbrella children. Belongs to `plan/spec-perf-next-0-umbrella.md`, which owns
   methodology. This is the most reusable finding in the whole effort.
4. **`wireu.TranscodeASPath` has no destination bounds check.** `copy` truncates
   silently, `PutUint32` panics. The caller carries an undocumented obligation
   (`ai/rules/go-standards.md`). Documenting it belongs to wire-edit child 3,
   which rewrites that function.
5. **The Deliverables row `grep -n "make(\[\]byte" forward_body.go returns
   nothing`** now contradicts the Boundary Tests row that sanctions the
   above-pool-class fallback. Reconcile at closure. The check was deliberately
   NOT edited to match the code.

## New work Thomas authorized today

- **`plan/spec-fixit-rfc7606-5-4-discard-unrecognized-nlri.md`** (Status
  `skeleton`, created today). He REVERSED the 2026-07-20 divergence ruling and
  chose full RFC 7606 Section 5.4 compliance. Read that spec's "Why this is not
  a small change": the retention is EMERGENT, nothing decides it, and a blanket
  discard would break BGP-LS where RFC 9552 Section 5.1 requires the opposite.
  The `{gap}` annotation in `rfc/short/rfc7606.md` is deliberately still there,
  because it accurately describes today's behaviour. Delete it in the same commit
  as the fix, not before.

## The wire-edit set, and what is now known about it

`plan/spec-wire-edit-0-umbrella.md` plus five children, all Status `design`.
Thomas chose to drive the children through subagents with a main thread
supervising.

**Child 1's central premise is BROKEN, and it is recorded in its own spec as
A-6.** The base that gets published is not always the array the RFC 7606 walk
indexed: the Section 3.g keep-first strip rebuilds the body, and the discard path
tombstones a type-code byte in place without building a new `WireUpdate`. So the
eager span index cannot simply be a by-product of that walk, and
`StripAttrRanges` / `ApplyAttrDiscard` are missing from child 1's Files to
Modify. **T1-5 changed both mechanisms today**, so re-read them rather than
trusting either spec's description.

Child 2 consumes T1-1 and T1-3 as preconditions. Both are now in, so it is
unblocked.

## Suggested order

1. Verify the other sessions' reds cleared; clean `make ze-verify`.
2. Independent `/ze-review` over the four commits' diff. Loop to zero. Record
   with `scripts/dev/review_gate.py`.
3. Fill the spec's Implementation Audit and Pre-Commit Verification tables,
   resolve A-1..A-6, reconcile the Deliverables contradiction.
4. Learned summary, then the two closure commits.
5. Then either the RFC 7606 Section 5.4 spec or wire-edit child 1. Child 1 needs
   design work first, because of A-6.

## Do not re-read these

The four commits' diffs are self-describing and their messages carry the
reasoning. The producing functions behind every claim above were read this
session and cited by symbol. What is NOT settled is anything in "Known gaps".

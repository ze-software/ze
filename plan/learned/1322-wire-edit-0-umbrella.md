# 1322 -- wire-edit-0-umbrella

## Context

The forward path carried a representation problem with five symptoms: the
attribute TLV sequence was walked by five separate scanners, every touched
attribute was rebuilt in an intermediate buffer, the AS-path family ran as a
second pass costing a second payload copy, the two announce rails had two
encoders that could disagree, and fan-out sharing keyed on the materialised
pointer -- downstream of the copy it was meant to avoid. This umbrella split the
fix into five independently landable children.

## Decisions

- **Five children, each independently landable and revertible**, over one large
  change. Child 1 is pure addition and changes no output byte, which made the
  substrate safe to land before anything depended on it.
- **Merge-insert new attributes at their ascending type-code position**, which is
  a deliberate WIRE CHANGE. Thomas approved it on 2026-08-01. AC-1's "byte
  identical output for every existing transform" therefore CANNOT hold as
  written: two goldens moved, both for that reason.
- **Land the oversize counter before the behaviour flip**, so the rate was known
  before a silent unmodified forward became a visible suppression.
- **Fingerprint the edit set, not the materialised pointer**, which is what moves
  fan-out dedup upstream of the copy.
- **Keep the package name `wireu`**, honouring the 2026-07-08 user decision that
  declined the rename over roughly 47 importers.

## Consequences

- One attribute index built once and read without a lock; one payload copy per
  destination instead of two; one encoder for both origins; one materialisation
  per policy group.
- Four live wire defects closed as a side effect of translating the code: the
  fail-open overflow forward, AGGREGATOR destroyed on a same-width prepend, the
  tombstone transitive clear firing on one path of three, and LOCAL_PREF reaching
  eBGP peers (RFC 4271 Section 5.1.5).
- The filter IPC representation is unchanged, so `plan/spec-filter-wire-0-umbrella.md`
  now has this as its substrate rather than an open axis.
- Four items survive closure, each homed in its own spec: the unreachable eBGP
  wire cache, the unfinished fan-out `.ci`, the small-fan-out regression, and the
  BIRD interop scenario plus the oversize counter.

## Gotchas

- **`ze-perf-bench` provably cannot measure this work.** It is a single-peer
  100k-route convergence run with almost no fan-out, so it cannot see a
  per-destination loop. A flat baseline is not evidence either way. This was
  stated up front as child 5's R-6, and it is why child 5 needed its own fan-out
  benchmark rather than a baseline delta.
- **"Merge-insert applies only when a new attribute is added" was read as
  denying a byte change.** It bounds the blast radius; it does not make the
  output identical. Any FUTURE golden that moves for a reason other than
  merge-insert ordering is a stop-and-report.
- **A representation redesign is a bug-finding exercise.** Every defect above was
  found by enumerating branches in order to translate them, not by looking for
  bugs. Budget for that.
- **One missing vocabulary word cost four structural workarounds.**
  `mpReachNextHopHandler` wrote base bytes, new bytes, base bytes, by hand,
  because the accumulator had no word for a fragment list.

## Files

- Children: `plan/learned/1317-wire-edit-1-base-index.md` through
  `plan/learned/1321-wire-edit-5-fanout-dedup.md`
- Commits: `bbd53bf22`, `a1aec5e6c`, `ddf04953a`, `e2037e598`, `b1fa7ab1e`,
  `bd1f3d873`, `f1f746fb6`, `ea6a4bbda`
- Survivors: `plan/spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal.md`,
  `plan/spec-wire-edit-4-api-origin-deferred-bird-interop.md`,
  `plan/spec-wire-edit-4-api-origin-deferred-oversize-metric.md`,
  `plan/spec-wire-edit-5-fanout-dedup-deferred-fanout-ci.md`,
  `plan/spec-wire-edit-5-fanout-dedup-deferred-small-fanout-regression.md`,
  `plan/spec-fixit-ci-peer-block-silent-directives.md`

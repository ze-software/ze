---
kind: directive
level: MUST NOT
stage:
---
- **SHOULD prefer a `.ci` over an interop binding** when a behavior is reachable from both: a `.ci` runs inside `./le verify current mode full` on every push, interop does not (owner decision, umbrella D3).
- A requirement whose ONLY evidence is nightly-tier is marked `**nightly-only**` on its ledger row and counted in its own rollup column: it is not merge-gate-proven, and the rollup deliberately never sums the two.
- **An interop tier is DERIVED; it MUST NOT be declared.** A native Go test under `internal/le/interoplab/` earns `interop/nightly` when a scheduled workflow names its registered `./le` runner. `internal/le/rfc.Carriers` derives that relation, so adding the job is the whole fix and deleting it removes the tier.
- **A scenario configuration directory is not an evidence carrier.** RFC tags belong in the native Go checker test that executes the assertion. A fixture name or configuration file cannot claim a tier.
- **A QEMU sibling is not that pipeline.** The registered QEMU actions execute their own Go packages and cannot justify an interop tier for a checker they never call.
- **Non-unit evidence is monotonic, per requirement and per tier.** Replacing a `.ci` binding with a unit tag, or with a nightly interop tag, fails `./le rfc check`, and no annotation satisfies it.

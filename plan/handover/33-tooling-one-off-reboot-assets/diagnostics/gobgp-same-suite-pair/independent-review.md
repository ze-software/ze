# Independent same-Suite diagnostic review

## Current private-input revision

The revised four-file source is APPROVED for MAIN's formatting and scoped
execution. See `independent-review-private-inputs.md` for the actual-delta
judgment. It removes all canonical patch writes and preserves the original
provenance and semantic assertions. No execution result is claimed.

## Historical shared-patch draft

Verdict: **REFUSE native execution of this draft.** Source-only review; no
formatter, compiler, Go tool, test, Docker operation, or runtime probe ran.

## Blocking finding

P1 — `goBGPSameSuitePair.replacePatch` does not preserve an unknown concurrent
edit across publication. It reads the live bytes into `latest`, compares them
with `current`, then separately calls `os.Rename(path, pair.patchPath)`. Another
writer can change or replace the live file after that read. Rename then removes
the displaced path entry without retaining that inode or its new bytes. The
following read observes only the diagnostic file, so it cannot detect or recover
the lost edit. The same window exists during deferred restoration.

The same-filesystem precondition MAIN confirmed avoids cross-device rename
failure. It does not turn the read and rename into an atomic compare-and-swap.
The draft's comment that an unknown concurrent edit is retained is therefore
stronger than its implementation. The owner's work-preservation ban applies to
this automatic diagnostic mutation too.

`RENAME_EXCHANGE` would retain the displaced inode, unlike replacement rename.
That alone is not approval: it still changes the public path before proving
ownership, and a check/exchange-back sequence has its own concurrent writers.
A repair using exchange would need an independently reviewed preservation and
recovery contract for every displaced inode, not a claim that exchange is CAS.
This review does not grant permission to move an unknown foreign edit.

## Safe repair direction

Remove shared-patch mutation from the diagnostic. Bind each plan's exact retained
patch artifact into both its private image-build input and the strict expected
patch-provenance input. Keep actual in-image bytes, the named runtime path, SHA256
validation, and every semantic/wire assertion. The ordinary native route must
retain its existing canonical-file default. No source-tree copy, expected-byte
bypass, hardcoded success, or weakened provenance check is acceptable.

The current `gobgpReloadProof.identifyPatch` reads the fixed path below
`CheckContext.Checkout`; the current API has no per-plan patch-input binding.
An explicit narrow input seam is therefore a source change to review, not a
capability this draft already has. Parameterizing an input while preserving its
checks is a possible implementation repair within this proof's purpose; it is
not an owner authorization to overwrite shared work. Review the actual revised
delta before executing it.

The proposed bounded correction can use a selected-file Go overlay that factors
the existing private checker into the same assertion body plus an immutable
expected-patch input. Keep its normal native input unchanged. Each private
GoBGP build context may contain only the exact canonical Dockerfile and its
selected small patch artifact; it is not another first-party source tree.
Retain one shared Suite for the nonproducer images. This direction removes all
canonical patch writes and preserves the byte/SHA algorithm; its actual code
still requires independent review before execution.

## Proof-strength checks completed

- The original `TestBGPSuiteSelectedImagesUsePreparedDependencies` body remains
  unchanged from HEAD and the exact canonical source. It still covers explicit
  selection, environment pinning, explicit precedence, eager full-suite
  behavior, exact selected names and image dependencies, prepared cleanup
  errors, and refusal of an unknown partial selector.
- `Suite.Run` has one Docker owner and one image-reference table across the two
  plans. `runScenario` calls dynamic preparation before `preparePeerImages`;
  that helper prepares successful logical image names once. The shared Ze,
  BIRD and FRR declarations can therefore remain held across both plans, while
  the two GoBGP logical names trigger separate builds.
- Both plans retain the original registered checker. Only their preparers are
  wrapped, and only the GoBGP peer's logical image name changes. Source and
  scenario configuration oracles are not replaced.
- The outer root separately rejects scenario cleanup and retained-image cleanup
  errors. It requires the first scenario's intended stale-prefix error, the
  second scenario's clean pass, and the native report's one-fail/one-pass exit.
- The negative proof reconstructs one complete probe. Existing parsers and
  `compareGoBGPReloadSnapshot` retain source attributes, capabilities, session
  counters, generations and daemon identity fences. Only secondary prefix 6
  is required to retain its prior useful path; every other receiver contract
  and the independent sink's absence assertion remain enforced.
- Both retained captures are hashed and passed through the existing complete
  capture and wire checks. Consumed nonproducer image identities must match;
  the GoBGP producer images and observed executables must differ.
- Read bounds are explicit, no worker or retry loop is added, and raw proofs
  and suite reports stay private. A Go timeout can bypass defers, so MAIN's
  post-run canonical-patch check remains necessary for this rejected draft.

## Judgment rows

| Unit | Verdict | Reason |
|------|---------|--------|
| `bgp.TestBGPSuiteSelectedImagesUsePreparedDependencies` | APPROVE unchanged preservation | Its original inputs and assertions are retained byte-for-byte; this is not a weakening. |
| `bgp.TestNativeGoBGPSecondarySameSuitePair` | REFUSE execution | The automatic patch publication/restoration can overwrite a concurrent foreign edit after its last ownership check. |

No weakening or RFC approval command was run. The delta adds a private root and
does not change an RFC-tagged unit or weaken the original test. This is a safety
refusal of the diagnostic implementation, not an approval-row requirement.

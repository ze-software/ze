# Independent review: private expected-patch inputs

**APPROVE this revised source for MAIN's formatting and scoped execution.**
This is a source verdict, not an executed or accepted paired proof. The refused
shared-patch draft remains under `rejected-shared-patch/`. I ran no formatter,
compiler, Go tool, test, Docker operation, approval command, or runtime probe.

## Actual delta

Reviewed all four entries in `overlay.json`, their before/after changes, the
earlier rejected caller, and the reached Suite/image/checker implementations.
All four before snapshots still match the canonical files. The three original
test bodies are unchanged; only imports and additive roots/helpers change their
test files. No RFC-tagged unit or expected output is weakened.

The prior P1 is removed. The caller has no `selectPatch`, `replacePatch`,
`restoreFull`, rename, or deferred restoration. Canonical patch/source paths
are read-only. Its writes are exclusive creates below its newly allocated
private directory, plus permissions on those newly created private contexts.
A timeout cannot strand a diagnostic patch in the shared checkout.

## Input and assertion checks

- `checkGoBGPExportReload` still selects the canonical input. Its body now
  delegates to the same common assertion body with a nil explicit input. The
  native observation order and finish/capture defers remain unchanged.
- `expectedPatch` retains the default missing-checkout, file-read, and empty-file
  refusal order. It runs at the original `identifyPatch` point, after actual
  image/version observations and before runtime patch/config reads.
- The selected constructor copies caller bytes into an immutable string.
  `goBGPPairChecker` captures a value; the selected checker helper passes its
  own value's address. Later changes to the constructor slice or the pair's
  stored input cannot mutate that captured string. Each expected-byte query
  returns a fresh slice. An explicit zero input refuses instead of choosing
  the canonical fallback.
- Both sources enter the unchanged applied-byte comparison, named runtime path,
  digest shape, hex decoding, and computed SHA256 comparison. No version-only
  identity or success bypass appears. The new tests retain the original
  before-config refusal requirement and include mutable-source ownership,
  empty/zero inputs, runtime byte mismatch, malformed/wrong/duplicate digests,
  wrong runtime path, and missing checkout.
- Both plans use the same common native assertion body and the original
  `specialChecker` error/log wrapper. Their only checker dependency difference
  is their owned expected patch. Route, source attribute, capability, session,
  daemon, repeated-read, capture and wire assertions are unchanged.
- The two contexts contain only the captured canonical Dockerfile bytes and
  their selected reviewed patch. The Dockerfile's only host-file COPY is that
  patch; its other COPY inputs are inline or builder-stage artifacts. These
  are explicit third-party build inputs, not another first-party source tree.
  The context files use exclusive creation/read-only modes and exact-content
  checks before each plan and after the suite.
- One Suite owner and image-reference table remain shared. Distinct GoBGP
  logical names prepare their private contexts separately. The root still
  requires identical actual consumed Ze/BIRD/FRR/sink image IDs, different
  GoBGP images/executables, the coherent targeted negative observation, a
  clean full positive result, and clean scenario/image retention teardown.
- Proof reads remain bounded and raw credential-bearing maps/reports stay
  private. The safe report does not replace the final named Go test verdict.

## Judgment rows

| Unit | Verdict | Reason |
|------|---------|--------|
| `bgp.TestBGPSuiteSelectedImagesUsePreparedDependencies` | APPROVE preserved | Its original inputs, selector precedence, exact dependency maps, cleanup handling and unknown-selector refusal are unchanged. |
| `bgp.TestGoBGPReloadRetainsAppliedPatch` | APPROVE preserved | Its complete original default provenance assertions remain unchanged. |
| `bgp.TestGoBGPReloadRefusesUnknownAppliedPatch` | APPROVE preserved | Its complete original refusal inputs and before-config assertions remain unchanged. |
| `bgp.TestGoBGPReloadSelectedPatchInputOwnsBytes` | APPROVE additive | It proves the selected binding owns its bytes while retaining runtime byte/digest checks and subsequent config observations. |
| `bgp.TestGoBGPReloadSelectedPatchInputRefusesEmpty` | APPROVE additive | Empty construction and a zero explicit input cannot become a canonical fallback or reach runtime patch/config observations. |
| `bgp.TestGoBGPReloadSelectedPatchInputKeepsRuntimeChecks` | APPROVE additive | Selected inputs retain the required runtime byte/path/digest and checkout refusals before configuration reads. |
| `bgp.TestNativeGoBGPSecondarySameSuitePair` | APPROVE revised source | It composes the same assertion body with private immutable inputs, preserves all paired oracles, and removes shared-file mutation. |

No weakening/RFC approval row is needed: original units are unchanged and the
new units are additive, untagged private diagnostics. No approval command ran.

MAIN still owes formatting only the selected after files, source identities,
the unchanged/default plus selected-input cohort, then the exact native root.
Read each named PASS/FAIL/SKIP result and retain actual source, image and proof
identities. Historical four-peer evidence, other checker obligations, the old
cleanup stall and whole Phase 9 remain outside this source verdict.

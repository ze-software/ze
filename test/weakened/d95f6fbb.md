| Test | Reason |
|---|---|
| TestRaceSelectionWithoutGreenBaselineCoversTheTree | It asserted that an absent, red or unresolvable verify certificate widens the race groups to `./...`. That widening is the defect spec-verify-release-check-only deletes (D-1): the change set is now anchored on git (`ResolveAnchor`), and a checkout with no anchor is refused, which `TestRaceSelectionRefusesWithNoAnchor` asserts with `errors.Is(err, ErrNoUpstream)`. |
| TestRaceSelectionWithGreenDocsOnlyChangeIsEmpty | Renamed to `TestRaceSelectionAtAnchorDocsOnlyChangeIsEmpty`: the same empty-selection and query-count assertions, with the anchor's two queries in place of the certificate's `cat-file` query. |
| TestRaceSelectionRealCommittedClosure | The three removed assertions covered the deleted certificate cases: a green certificate narrowing, and a removed certificate widening to `./...`. The test now drives a real upstream: the unpushed Go commit behind a docs-only commit is raced, and once the upstream holds both nothing is. |
| selectionBaseline | Helper that wrote a green certificate; replaced by `isolateSelection`, which clears the published scope files, because no certificate takes part in selection any more. |
| writeScopeStatus | Helper that wrote a certificate for the two deleted selector tests below; nothing else used it. |
| TestSelectorWidensWithNoTrustedGreenBaseline | It asserted the deleted widening on a missing, red or `git_sha=unknown` certificate. The anchor tests replace it: `TestScopeAnchoredOnUpstreamWithNoCertificate` asserts a red certificate no longer widens, and `TestAnchorRefusesWithNoUpstream` asserts the refusal with exit 2 and the fixing command. |
| TestSelectorReadsAnAbsoluteStatusFileOverride | It pinned how the deleted `greenBaseline` joined an absolute `ZE_VERIFY_STATUS_FILE`. The selector reads no status file now, so there is no path to pin. |
| selector_test | The file-level count drops with the two deleted tests above. |

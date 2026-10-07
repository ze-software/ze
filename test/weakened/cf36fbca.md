# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| drift_test | TestDriftReadsTheFeatureInventory left with checkFeaturesMD: docs/features.md is now rendered from features/*.md, so a Status cell outside the vocabulary cannot be written. The refusal moved to the producer: Parse refuses an unknown Level or Scope (declaration_test AC-2) and TestFeaturesPageRendersEveryDeclaration asserts every row carries the vocabulary label for its pair |
| TestDriftReadsTheFeatureInventory | Deleted with checkFeaturesMD, the drift check of hand-typed Status cells in docs/features.md. That page is now rendered from features/*.md (internal/le/feature RenderPage), so no hand cell exists to check; Parse refuses a value outside the vocabulary and TestFeaturesPageRendersEveryDeclaration asserts each row carries the label for its scope and level |

# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| TestTheFeaturesPageUsesTheSharedShell | Renamed back to `TestTheFeaturesPageReadsAsThePublishedPage`, which keeps every chrome check this test held and restores the body golden and the byte-for-byte mirror golden it had dropped. Nothing left the suite. |

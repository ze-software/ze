# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| TestStressHarnessBuildsZeFromCheckoutWhenMissing | It pinned the defect: the stress runner built the DUT only when bin/ze was missing, so a stale bin/ze was profiled as the tree under test. TestStressHarnessBuildsZeFromCheckoutEveryRun replaces it with bin/ze present and asserts the exact build argv with the feature-gates.txt tags, the directory, CGO_ENABLED=0, the reported binary, and that bin/ze is never started |

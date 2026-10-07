# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| snapshot_test | New test TestAnUnreadableCopyIsKeptAndReported, nothing removed: it skips only as root, which reads a file with no permission bits, so the unreadable copy AC-5 needs cannot be built there (testing.md permits t.Skip with a reason for a test that cannot run everywhere). The unstageable half of AC-5 is TestAnUnstageableCopyIsKeptAndReported, which runs everywhere |

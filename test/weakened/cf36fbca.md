# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| recordedTree | Extracted, not a weakening: its body moved into `recordedTreeOf(t, edit)` (recordrun_test.go), which applies a declaration edit first; recordedTree now calls it with the identity edit, so the os.Remove error check still runs for every caller. |

# Release roadmap closure: restored assertions

The closure of spec-release-roadmap restores the assertions that its handoff
commit removed (recorded in `test/weakened/be999491.md`). One test name leaves
the suite only because it takes its original name back.

| Test | Reason |
|------|--------|
| judgeOne | Nothing left the suite: its three assertions (Check error, parse problems, widget verdict found) moved unchanged into judgeOn, which judgeOne now calls with fixtureToday so every fixture run is judged on a fixed day instead of the wall clock |

# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| TestDispatchCapturesStdoutAndPreservesIdentityCodeArgsAndRoot | Verify stages now run as child processes, so the parent never overrides its own repository root. The "root restored to before-dispatch" assertion checked the undo of an override that no longer exists. Output, identity, code, args and the root the stage judged are still asserted, now as the child reports them. |
| TestDispatchRefusesUnownedMissingAndMismatchedRoots | Renamed TestDispatchRefusesUnownedMissingAndAbsentRoots. The "mismatched" case tested the root-mismatch branch, deleted with the in-process override; that the stage judges the requested root is asserted end to end by the child reporting `lepath.Root()`. |
| TestDispatchSerializesRootOverrides | Replaced by TestDispatchGivesOverlappingStagesTheirOwnRoot. It guarded the mutex around the shared in-process root override, which no longer exists; the replacement covers the same risk, two stages at once, and goes red if a child does not receive its own root. |
| dispatch_test | The file-level count drop is the three rows above; the file gains eight tests (killed, interrupted, spawn, crashed, relayed panic text, leftover process, SIGTERM on cancel, real re-exec). |

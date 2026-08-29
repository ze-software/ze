| Test | Reason |
|------|--------|
| TestHandlewithdrawTagKeyValue | Renamed to `TestWithdrawByTagKeyValue` in the same file. `withdraw` became three sibling commands, so `handlewithdrawTag` became `handleWithdrawTag` and its tests took the name of the command rather than the function. Every assertion is unchanged |
| TestHandlewithdrawTagKeyWildcard | Renamed to `TestWithdrawByTagKeyWildcard`. Same rename, assertions unchanged |
| TestHandlewithdrawTagKeyOnly | Renamed to `TestWithdrawByTagKeyOnly`. Same rename, assertions unchanged. This is the case the authored prose got wrong: `withdraw tag <key>` alone is valid, because the value defaults to `*` |
| TestHandlewithdrawTagStar | Renamed to `TestWithdrawByTagStar`. Same rename, assertions unchanged |
| TestHandlewithdrawTagMissingArgs | Renamed to `TestWithdrawByTagMissingArgs`. Same rename, assertions unchanged |
| TestHandleWithdrawIDValid | Renamed to `TestWithdrawByIDValid`. Same rename, assertions unchanged |
| TestHandleWithdrawIDNotFound | Renamed to `TestWithdrawByIDNotFound`. Same rename, assertions unchanged |
| TestHandleWithdrawIDInvalid | Renamed to `TestWithdrawByIDInvalid`. Same rename, assertions unchanged. It still proves a non-numeric id is refused with a useful error, which is why the `id` leaf is a length-bounded string rather than a uint64: a numeric type turns `withdraw id abc` into "unknown command" |
| TestHandleWithdrawIDMissing | Renamed to `TestWithdrawByIDMissing`. Same rename, assertions unchanged |
| TestHandlewithdrawAllEmpty | Renamed to `TestWithdrawEveryEmpty`. Same rename, assertions unchanged |
| TestHandlewithdrawAllWithEntries | Renamed to `TestWithdrawEveryWithEntries`. Same rename, assertions unchanged |
| TestHandlewithdrawAllWithSelector | Renamed to `TestWithdrawEveryWithSelector`. Same rename, assertions unchanged. It covers the `selector <pattern>` filter that `handlewithdrawAll` has always accepted and the authored prose documented nowhere; this commit declares that leaf |
| TestActionTableDeclaresThreeNativeVerbs | Renamed to `TestActionTableDeclaresFourNativeVerbs`, because the docvalid action table gained `usage-contract`. The assertion is strengthened rather than weakened: it names four verbs where it named three, so a fifth added without a test still fails it |

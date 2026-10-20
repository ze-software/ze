| Test | Reason |
|---|---|
| attributeCodes | Helper only TestWithheldFlowWithdrawWritesItsAttributesWithNoRoute called; that test's message no longer exists, so nothing reads attribute codes |
| TestWithheldFlowWithdrawWritesItsAttributesWithNoRoute | The attributes-only UPDATE it pinned is removed: ExaBGP at the pin 685f772e4 and 5.0.13 send none, no RFC asks for it. Replaced by TestWithheldFlowWithdrawWritesNothing, which asserts no UPDATE reaches the peer and the route is counted withheld |
| TestWithheldFlowWithdrawArmsNothing | The per-body NLRI check went with the body: the test now asserts no UPDATE at all after two withheld withdrawals, which is stronger than "the second body names no route" |
| pinBuilderSignatures | buildWithheldWithdrawUpdate is deleted, so its nil check and parameter leave; the two remaining group builders are still pinned |

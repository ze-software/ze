# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| awaitChainEvaluated | Replaced by awaitTermMatched. It waited for rule 1's counter as "the chain ran", which a counter placed behind the term's matches (7278126c3f) no longer means. The drop test now waits on the counter of the term that must drop the message, which is a narrower and true signal. |
| assertWholeChainEvaluated | Replaced by assertNoTermMatched after the delivery wait on IcmpMsg InType3. Lost: proof that the chain evaluated a packet no term matched, which no match-scoped counter can observe. Thomas approved losing it on 2026-10-17 ("approve fix"). Delivery is still proven by InType3, a match by any term still fails, a rule with no counter fails (rulePackets), and TestGTSMDropsADangerousQuotedICMPError proves the same chain sees type-3 messages on the same path. |
| lastTermReached | Replaced by termsMatched. "The last non-zero rule" named the matching rule only while the counter sat ahead of the matches; termsMatched reads each counter as its term's matches, and the drop test asserts exactly the quoted-destination term matched, which is stricter. |

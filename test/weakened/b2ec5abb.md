# Test weakenings this commit accepts

The BFD two-link prestarts moved from `*-prestart.sh` files in the scenario
directory to Go constants indexed by link-local owner. The removed case tested a
refusal for a state the new type cannot hold, so nothing it guarded is left
unguarded.

| Test | Reason |
|------|--------|
| awaitChainEvaluated | Replaced by awaitTermMatched (owner-approved GTSM fix): commit 7278126c3f moved each nft rule counter after its matches, so the first rule's counter no longer moves for a packet that rule does not match; the helper now waits until any rule's counter moves. |
| lastTermReached | Replaced by termsMatched (owner-approved): with counters after the matches, every rule whose counter moved is a rule that matched, so the drop test asserts exactly the quoted-destination term matched. |
| assertWholeChainEvaluated | Replaced by assertNoTermMatched (owner-approved): a non-matching packet leaves no counter trace now, so the proof that the chain evaluated it is gone; the ICMP receive counter and the installed-table check remain, and the delivery tests assert no term matched. |

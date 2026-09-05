# Membership answered by a scan, once per item

Asking "is this value in that list" by walking the list is correct, and it is
free while one of the two counts is small. It stops being free when both counts
come from the same untrusted message, because the sender then chooses their
product.

The cost hides well. The nested walk usually sits inside a branch a conforming
sender never takes, so every test, every capture and every benchmark shows it
running over an empty list. Its own comment often says so, and reads as a bound
when it is only an observation about well-behaved senders.

Ask two questions of every nested walk. Who chooses the outer count, and who
chooses the inner one. When the answer to both is "the peer", key the inner
collection once per message and answer with a lookup. Say what one message can
hold, in the comment, as a number.

The rule that governs this already exists. `docs/contributing/ze-go-style.md`
refuses "an algorithm that grows with the square of the peer count" under Zero
technical debt, and asks for a stated limit on every loop over external input.
This file adds only the TELL, which that page does not carry: the nested walk
hides inside a branch a conforming sender never takes, so the tests, the
captures and the benchmarks all show it running over an empty list.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | spec-rfc7911-generate-own-path-id | bgp reactor, RFC 7911 Path Identifier release | `fwdReleaseSection` (`internal/component/bgp/reactor/forward_path_id.go`) answered "does this same UPDATE also announce the pair I am about to free" by walking the announced section once per withdrawn NLRI (`fwdSectionCarries`). Both counts are chosen by one peer inside one message, and the walk runs with `RecentUpdateCache.mu` held (`recent_cache.go` `evictLocked`), which is the lock every peer's UPDATE ingest takes. Two 32000-octet ADD-PATH sections of an RFC 8654 extended UPDATE (`message.ExtMsgLen`) cost 41 million comparisons, measured at 137ms, repeatable at line rate. The exception itself is correct and had to stay: freeing a pair the same UPDATE announces strands it at the destination forever. Its comment named RFC 7606 Section 5.1, which forbids a conforming sender to write both fields, and read as a bound when it only said that a well-behaved peer never pays | `fwdSectionCarries` is deleted. `fwdAnnouncedPaths` keys the announced section once into a `map[fwdPathKey]struct{}` and the release answers with a lookup, so each section is walked once and a conforming withdraw builds no map at all. `TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt` (`forward_path_id_churn_test.go`) pins the exception, which nothing had asserted: deleting the branch reddened no test before it existed |

---
kind: directive
level: MUST
stage:
---
- **A discrimination proof MUST state whether its re-run actually ran.** `go test` keys a cached verdict on the files the TEST BINARY OPENED, which is narrower than a source hash: a producer the test reaches through `exec`, a compiler it invokes, or an interpreter it shells out to is not one of those files, so mutating it changes no cache key and the tool answers `ok (cached)` for a run that never happened. The tell in the output is a bare `ok` with no duration.
- **A mutation to PACKAGE SOURCE owes nothing further; a mutation to an exec-reached producer MUST defeat the cache with `-count=1`, or drive the producer through a runner that keeps no Go cache, and say which was done.** A `.ci`, `.et`, `.wb` or Docker run has no Go result cache at all, so the caveat MUST NOT be applied where it cannot apply.
- **Applying `-count=1` everywhere MUST NOT be treated as the answer.** It spends the cache of a gate that already costs tens of minutes; the obligation is to know which category the proof is in.

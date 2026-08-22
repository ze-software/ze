---
kind: directive
level: MUST
stage:
---
**A discrimination proof MUST state whether its re-run actually ran.** "It went
red when I mutated it" is not a sufficient claim. The report says which mutation
was applied, and whether the re-run was a real execution or a cached verdict.

The reason is mechanical rather than a matter of care. `go test` keys a cached
verdict on the files the TEST BINARY OPENED. A producer the test reaches through
`exec`, a compiler it invokes, or an interpreter it shells out to is not one of
those files, so mutating it changes no cache key and the tool answers `ok
(cached)` for a run that never happened. The standard proof, mutate the producer
then re-run and observe red, degrades silently into mutate the producer, re-run
nothing, observe the old green.

Which category a proof falls into decides what it owes:

- A mutation to PACKAGE SOURCE changes the cache key. Nothing further is owed.
- A mutation to a producer the test EXECS does not. Defeat the cache with
  `-count=1`, or drive the producer through a runner that keeps no Go cache, and
  say which was done.
- A `.ci`, `.et`, `.wb` or Docker run has no Go result cache at all. Say so rather
  than applying the caveat where it cannot apply.

The tell in the output is a bare `ok` with no duration. A real run reports one.

**Applying `-count=1` everywhere is not the answer and MUST NOT be treated as
one.** It disables the cache for every test in the run, and a gate that already
costs tens of minutes pays that in full. The obligation is to know which category
a proof is in, not to spend the cache to avoid thinking about it.

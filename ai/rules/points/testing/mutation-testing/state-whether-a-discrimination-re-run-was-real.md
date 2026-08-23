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

**A real re-run is only half of it. You MUST also verify that the MUTATION
APPLIED, between the patch and the run, with a diff that MUST come back non-empty
or a grep for the mutated text.** A patch that fails to apply leaves the test
running against unmodified source, so it passes, and the artifact of that attempt
is byte-identical to a successful proof. This is the worse half: a stale cached
verdict at least ran once against real code, while an unapplied mutation means
nothing was ever tested.

**Restore the file by copying back a pristine copy you saved first. You MUST NOT
reach for `git checkout --`, `git restore` or `git stash`: they are banned
outright, and a mutation proof is the moment the reflex to use one is strongest.**
Save the copy before the patch, restore with `cp`, and confirm the file is
byte-identical afterwards. In a shared checkout those verbs would discard another
session's uncommitted work in the same file, and the ban does not soften inside a
throwaway worktree, where the habit is formed and carried back out.

It defeats the habit that catches every other shape. Break it and watch it go red
fails silently when the break never landed, and the report then says truthfully
that the re-run was real while the proof is worth nothing. Everybody already
saves a copy of the file and restores it by hash, because the interesting moment
feels like the run. Confirming the change landed costs one command, and it is the
only thing that makes the run mean anything.

**Applying `-count=1` everywhere is not the answer and MUST NOT be treated as
one.** It disables the cache for every test in the run, and a gate that already
costs tens of minutes pays that in full. The obligation is to know which category
a proof is in, not to spend the cache to avoid thinking about it.

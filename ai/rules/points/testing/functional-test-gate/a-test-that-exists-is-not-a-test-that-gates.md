---
kind: note
level:
stage:
rationale: plan/learned/1062-redistribute-late-join-replay.md
---
A functional test that EXISTS is not the same as one that GATES. A `.ci`/`.et` can
pass whether or not the feature works (a **false-pass**) when the observed effect
reaches the assertion by a path OTHER than the one under test. Real example: three
`redistribute-late-join*.ci` tests kept passing with the late-join replay
(`handleReplayBatch`) disabled, so the route reached the peer by some path other than
the replay: they guarded nothing and shipped green
(`plan/learned/1062-redistribute-late-join-replay.md`).

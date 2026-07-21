### `l2tp` functional suite `session-stopccn-cascade` -- pre-existing, predates the initiator work

Confirmed pre-existing 2026-07-10 by building `bin/ze` + `bin/ze-test` at commit
`fe6aa242f` (the parent of `b68e7e9c9`, the first `spec-followup-l2tp-call`
commit) via `git archive` and running the test: it fails there **3/3**
deterministically, identically to `HEAD` (5/5). So `spec-followup-l2tp-call`
did not cause it. Symptom: `test/l2tp/session-stopccn-cascade.ci` step 2
(`expect=stderr:contains=StopCCN clearing sessions`) does not match. Root cause
lies in the answering-side reliable-receive path: after the tunnel + session 1
establish, ze's receive window does not advance past the second session's
rapid-fire ICRQ, so the peer's later StopCCN (a higher Ns) is never delivered to
`handleStopCCN` (`tunnel_fsm.go:582`), and the `StopCCN clearing sessions` log at
`tunnel_fsm.go:597` never fires. It belongs to the answering-side
reliable-receive path, not this initiator spec. Fix owner: whichever session
next works the L2TP reliable-transport receive path. `.ci` unchanged since
before this spec.

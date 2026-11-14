# Learned: one declaration of heavy work, read by the hook and by admission

`spec-native-action-job-admission` made every heavy `./le` line take one job
slot, so `./le job report` shows the heavy work on the machine. The dispatcher
reads the slot a line takes from the declarations its own package makes
(`leroot.SlotOfLine`, `internal/le/le/root/heavy.go`). By default it asks the
admitter that `internal/le/job` registers (`leroot.RegisterAdmitter`,
`admitCentral` in `internal/le/job/central.go`) for one unshared slot before
the body runs. An action that admits itself (`leaction.Action.AdmitsItself`,
`leroot.RegisterAdmitted`) or takes no slot (`Action.NoSlot`,
`leroot.RegisterNoSlot`, each with its reason on the help page and in the
manifest) says so on its own row. `le exec` takes a slot by the program it
carries (`job.HoldTool`), judged by the one heavy-tool definition the hook
also reads (`internal/le/heavytool`). The design is in
`docs/architecture/core-design.md` and `docs/contributing/running-commands.md`,
"One verify at a time" and "Waiting for another session's job". The proof is
`test/ui/le-job-admits-heavy-action.ci`. It went red with the dispatcher's
central branch disabled.

## What outlives the spec

1. **A population that must grow by itself is a default, never a list.** The
   earlier rollout wrapped 106 make targets, two of which admitted after the
   port, and nothing went red (`plan/journal/refactor-removes-feature.md`,
   2026-09-05). Admission is now what a heavy line gets unless its own row
   says otherwise, with a reason. The census (`TestCensusHeavyActionsAreAdmitted`)
   pins only the exceptions.
2. **The declaration that hands a line back to the main thread is the same
   fact that admits it.** `HeavyCommand` and `SlotOfLine` read one resolution
   (`resolveLine`), so the hook and the dispatcher cannot disagree on what is
   heavy.
3. **A process that replaces itself has no "after" to release in.** `le exec`
   execs its program, so a slot taken by the dispatcher could never be freed.
   The heavy-tool case runs the program as a child, forwards every signal and
   answers its code. Every other case still replaces the process.
4. **A waiter never removes a holder it did not create, and an unknown
   liveness is not a dead one.** At 61e01bb12a two waiter paths could remove
   a live holder's entry, and the verify slot lost mid-run
   (`plan/journal/lifetime-ends-before-its-last-reader.md`, 2026-10-24) fits
   the first better: `scanAndClaim` reaped any entry `alive` read as dead,
   and `alive` answered dead on every `ps` error, a timeout under load
   included, with no signal sent, so verify kept running; and `breakStalled`
   killed and reaped a holder whose own log was quiet while its stages wrote
   theirs. ee1deac76a closed both: `alive` returns an error beside its
   answer, an unknown liveness keeps the slot, and `breakStalled` is gone.

## Limits left as stated

The hook does not read launcher words (`nohup`, `xargs`, `find -exec`,
`watch`, `parallel`), and it reads no shell except bash and sh. A SIGKILL of
`le` orphans its tool. A central holder shows no stage. The main thread's
post-edit lint stays outside the slots by the main thread's choice. The
report's closing notice (`slotsNotice`, `internal/le/job/overview.go`) is a
hand-written list of what the count leaves out: the raw-admission escape,
light `exec` programs, slot-free actions and the post-edit lint. It is a
second copy of declarations made elsewhere and can drift from them. Deriving
it needs a registration for work that takes no slot. Weighing a
job by memory and disk is
`plan/post-release/engineering/spec-test-verdicts-independent-of-load.md`.

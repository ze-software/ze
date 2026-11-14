# Learned: the code that reacts to an end must be proven through the entry point that reaches it

`spec-single-owner-lifecycle-5-session-end-locals` fixed three places where a
session or a plugin ends and the code that reacts reads the wrong thing or acts
in the wrong order. An LDP session that reached operational now clears its
setup backoff and publishes one SessionDown, and an attempt that never got
there publishes nothing (`internal/plugins/ldp/register.go`,
`startSessionForAdj`). The events of one L2TP session now reach their readers
in the order its state changed, through a per-tunnel order ticket taken under
`tunnelsMu` and delivered after it is released (`internal/component/l2tp/session.go`
`issueNoticeTicket`, `docs/architecture/l2tp/subscriber-session-model.md`).
The ddos-flowspec plugin withdraws on `bye`, records its live rule, withdraws a
recorded rule when it starts, and `ze doctor` reports a rule nobody owns
(`docs/guide/ddos-mitigation.md`). The proof is five daemon-level `.ci` tests
and the FRR interop scenario `ldp-backoff-clear-frr`, each with an observed red.

## What outlives the spec

1. **A unit test that calls a step directly proves nothing about the entry
   point that guards it.** The section-removed withdrawal was proven by calling
   `replaceResponder`. The first daemon-level run showed a real reload refused
   the removal at config verify, so the withdrawal could never run (fixed in
   3e347a86e6). Before calling a reaction proven, drive it from what the
   operator does (a reload, a `clear`, a peer closing), not from the function
   the reaction lives in.
2. **The state at an end comes from the code that saw it happen, not from the
   object afterwards.** LDP read `sess.State()` after `Stop` had reset it, and
   `Stop` has three callers, so reordering two defers fixed one of them. The
   on-operational callback already ran once on the right goroutine; carrying
   its fact to the end callback fixed all three.
3. **An absence at a reader is not a signal; the writer gives the order.** At
   every L2TP reader "no entry for this session" is the ordinary first event,
   so no reader can tell a late event from an early one. A second lock held
   across the publication would have ordered it and made every state change
   wait for the slowest reader (RADIUS Accounting-Stop, 10 s). A ticket issued
   inside the critical section and redeemed outside it orders the publication
   and holds nothing.
4. **A red is a claim until it is observed in the artifact the test drives.**
   An interop red applied through a host build overlay passed: the ze image is
   built from the tree, so the mutation never reached it, and only the missing
   log line showed it. In review, removing the exit withdrawal reddened
   nothing, because no test reached it. Read the red's own output for the
   mutated behavior before recording it.
5. **What an exit can still reach depends on how the process is placed, and the
   SDK decides it.** In process, `(*Plugin).Run` stops bridge dispatch before a
   plugin's deferred exit runs, so a withdrawal there is answered
   `ErrBridgeClosed` on every end; only a plugin running as its own process
   reaches the engine on its way out. A declared runtime doctor check also
   vanishes with its plugin's process, because doctor asks only the processes
   the manager still holds. Anything that must survive a plugin's end is
   recorded while it is live and settled by the next start or by a check the
   daemon owns. Read the plugin's failure policy before writing who starts it
   next: ddos-flowspec declares none, so the next start is the operator's
   `ze request plugin restart ddos-flowspec`. A check the daemon owns must then
   report what the stopped plugin left: the first closure recorded this as
   done on the unconfigured-section case alone, and with the section
   configured doctor named the stopped plugin, not the record. A registered
   runtime-phase check now reports the record with the victim and the restart.
6. **A doctor check over live daemon state takes the runtime phase, and the
   phase it lacks data in is not a reason to move it.** The first fix asked
   the daemon's plugin snapshot from the post-config check, which also runs
   when no daemon answers: every local run after a daemon stop warned, with a
   fix that needed the daemon. The runtime phase had no configuration to tell
   "section removed" from "daemon loaded another file", so the owner agreed
   to hand it the saved configuration the same answer parsed
   (`docs/architecture/doctor-and-health-checks.md`). That tree is the saved
   file, never the running one: use it only to agree with the config-phase
   check that read it, and decide which check owns each state before writing
   either, or one record gets two rows whose fixes disagree.
7. **A red an agent reports is a claim until the main thread observes it.** The
   first SDK write-error test was reported red and passed against the unfixed
   code: on a pipe the write fails when the deadline is set, before the
   misreported path. Run the test against the unfixed code yourself before
   recording its red.
8. **A sibling path with the same shape belongs inside the fix.** The
   write-error fix covered the single-message write and left the batch write,
   which carries every event delivery, on the old decision for one more review
   round. Before closing a fix, search for every function that makes the same
   decision.
9. **Text corrected in one round is new text the next round must read.** Rounds
   three to six each found a misstatement in an exit line, a report or a guide
   row the previous round had rewritten. Check each rewritten message against
   the code that produces its condition, as you would new code.
10. **A change to a function an RFC record is tied to stales the record in the
    same commit.** Three code commits here left 27 records and 2 audit verdicts
    stale until closure. Run `./le rfc check` for the stems a change touches
    before landing it, and re-take what it names in the same piece of work.
11. **A setup that depends on timing fails only on the run that loses the
   race.** The interop checker added ze's transport address after ze could
   already dial, and a later run lost a backoff step to `bind: cannot assign
   requested address`. Put a test's preconditions in place before the daemon
   starts (the lab's prestart), and stress every `.ci` under load before
   trusting a first green.

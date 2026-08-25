---
kind: directive
level: MUST NOT
stage:
rationale: plan/journal/false-synchronization-claim.md
---
**A `time.sleep(` MUST NOT be written until a synchronisation primitive has been
tried and found not to fit.** The primitives are in `test/scripts/ze_api.py`:
`wait_until`, `wait_for_output`, `wait_for_stderr_lines`, `wait_for_event`,
`wait_for_events`, `dispatch_until`, `dispatch_until_done`, `wait_for_daemon_ready`,
`wait_for_shutdown`, `wait_peer_counter`, `wait_peer_eor_sent`,
`wait_peers_established`, `wait_rs_replayed`, `wait_for_config`,
`wait_for_registry`, `quiesce`. A duration is what a test writes when it cannot
name the thing it is waiting for, so naming that thing is the work, and the
sleep is what remains when the name does not exist.

**The comment MUST declare which kind the sleep is, in the marker form
`# sleep(<kind>): <reason>`.** The kinds are the closed set the table above
names: `poll-interval`, `timer`, `timeout-under-test`, `needs-linux`,
`no-signal`. The marker is what makes "I tried" checkable. A free-text comment
is not: `# settle` satisfied every gate this repository had, and a reader
learned nothing from it.

**A `timer`, `timeout-under-test` or `no-signal` reason MUST name the mechanism
and where its period is set.** "The tracker pushes live carrier once a second"
is a reason a later reader can check and overturn. "Needs a moment" is not, and
it is the shape that turns a deliberate timer and a guessed duration into the
same line of code.

`wait_until` deserves its own sentence, because it is the answer more often than
it looks. It takes a predicate, so it converts any readback the test can already
perform, including one that shells out. Its own internal sleep is EXEMPT from
the ratchet, so moving a poll interval into it removes a counted sleep without
removing a wait.

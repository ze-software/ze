---
kind: directive
level: MUST NOT
stage:
---
**Separate from the API-sync wait.** `apiSync` counts plugins that SEND routes
and carries a 500 ms IPC grace for external ones. Barrier plugins only register,
so they MUST NOT drag that sleep in, and the two counters MUST NOT merge: a
route sender's signal satisfying a registrar's obligation is a fail-open
(`ai/rules/evidence.md`). A peer whose only barrier plugin is
in-process does not block on the wait: state-event delivery is synchronous, so
the acknowledgement has already landed on the FSM callback goroutine by the time
`sendInitialRoutes` is spawned, and the wait finds the channel closed. The
barrier is therefore an enforced invariant rather than a delay on that path:
it blocks only where delivery is genuinely asynchronous or slow.

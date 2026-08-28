---
kind: directive
level: MUST NOT
stage:
rationale: plan/journal/false-synchronization-claim.md
---
**A `time.Sleep` MUST NOT be written until a synchronization primitive has been
tried and found not to fit.** Compiled fixtures use `fixture.Poll`,
`fixture.Dispatch`, SDK readiness callbacks, contexts, and runner engine-step
predicates. Peer-specific helpers poll counters such as `eor-sent` when a
scenario depends on bytes reaching the wire. A duration is what a test writes
when it cannot name the condition, so naming that condition is the work.

**The comment MUST declare which kind the sleep is, in the marker form
`// sleep(<kind>): <reason>`.** The kinds are the closed set the table above
names: `poll-interval`, `timer`, `timeout-under-test`, `needs-linux`,
`no-signal`.
A free-text `// settle` comment is insufficient because it names no mechanism a
reader can check.

**A `timer`, `timeout-under-test` or `no-signal` reason MUST name the mechanism
and where its period is set.** "The tracker pushes live carrier once a second"
is a reason a later reader can check and overturn. "Needs a moment" is not, and
it is the shape that turns a deliberate timer and a guessed duration into the
same line of code.

`fixture.Poll` takes a predicate, so it converts any state readback the fixture
can perform into a bounded wait. Its own timer is the synchronization helper,
not a delay before an unrelated assertion.

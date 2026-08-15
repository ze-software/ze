---
kind: directive
level: MUST
stage:
---
**A guard is real only on the path that carries the traffic. Before you record
one as met, you MUST ask what the LIVE path reads, never what the template, the
config or the surrounding code says.**

A check on a surface the traffic does not use looks correct in review, because
the check is genuinely there and it genuinely rejects. It simply never runs. A
page template hides a control from a read-only session, and a script re-fetches
that control from a handler which consults no authorizer. A config states which
program receives an event, and delivery never reads that config.

**The tell is a second surface answering the same question.** When a fragment, an
OOB swap, a refresh endpoint or a cache can produce what a guarded surface
produces, each one is a path and each one needs the guard. Name them, then check
the one the browser or the peer actually calls.

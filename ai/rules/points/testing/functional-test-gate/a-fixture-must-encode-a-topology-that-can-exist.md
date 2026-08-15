---
kind: directive
level: MUST
stage:
---
**A functional fixture MUST give distinct roles distinct identities, even where the
host lets them share one.** Loopback, one container and one process make it cheap
to hand two ends of a protocol session the same address, the same identifier or the
same port. The protocol forbids that, so a fixture built on it encodes a state no
deployment can reach, and every assertion it makes is about a machine that cannot
exist.

**The cost is paid later, by somebody else.** A guard that reasons about identity is
correct and still reddens such a fixture. The session that meets that red has to
prove the guard right before it can call the fixture wrong, and the cheap move at
that moment is to weaken the guard. That is the one move this point exists to stop.

**Give each role its own identity when you WRITE the fixture.** It costs one line
there and an investigation anywhere else.

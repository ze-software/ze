---
kind: directive
level: MUST
stage:
---
**When a second gate reuses a check another gate already runs, it MUST supply
that check the same INPUT SHAPE the first gate supplies, and the shared shape
MUST be stated where the check is defined.** Sharing one implementation is what
keeps two gates from disagreeing about a rule. It does not keep them from
disagreeing about the SUBJECT, because a check reads its subject through the
values its caller passes. A caller that builds those values differently gets
different answers out of identical code, and the sharing hides it: both gates
cite the same function, so the difference looks impossible.

**The failure MUST be treated as blocking rather than cosmetic, because a
later gate that refuses what an earlier gate allowed leaves no way forward.**
The earlier gate has already passed, its verdict cannot be revisited, and the
later refusal names a rule the author did not break. An exemption a check grants
is part of its contract exactly as much as the violation it reports, so an input
shape that silently voids an exemption converts a permitted subject into a
blocked one.

**A shared check MUST be exercised through the NEW caller, with the values that
caller really constructs.** A test that calls the check directly, or that
rebuilds its input by hand, proves the check and not the wiring. Reusing a
subject the check is known to exempt is what makes the test discriminate: an
input shape that agrees with the original caller reports the exemption, and one
that does not reports the violation.

**Where the shape is derived rather than passed through, the derivation MUST NOT
import context from outside the artifact under test.** Widening a value until
the check accepts it can make the check depend on the environment the tool runs
in, which turns one gate's verdict into a property of the machine.

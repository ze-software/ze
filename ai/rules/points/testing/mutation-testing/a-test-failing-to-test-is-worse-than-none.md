---
kind: directive
level: MUST
stage:
---
**A test that cannot fail is worse than no test, because it also spends the
attention that would have found the gap.** So before a test is offered as
evidence for a requirement, MUST establish that it would go red if the behavior
it names stopped happening. Reading the assertion is not that establishment: an
assertion can be true for a reason unrelated to the code under test.

**Five shapes recur, and each is invisible to a gate that checks only whether a
test EXISTS.** MUST check for them by name:

- A value asserted into memory that already held it. Any assertion that
  something is zero, empty, or absent is suspect when the buffer or structure
  was freshly allocated: deleting the code that writes the value changes
  nothing.
- The happy branch alone, where the justification for a missing case cites the
  very branch no test enters. When an annotation explains why one polarity is
  absent, that explanation names the case most worth writing.
- A property strictly weaker than the one the requirement states. "Not a
  constant" is not "unpredictable"; "present" is not "correct"; "non-empty" is
  not "complete". A test can only assert what it can observe, so where the
  requirement's own word is unobservable, MUST assert the structural fact the
  requirement depends on and say that is what is proven.
- One clause of a requirement that states two, with the tag claiming both. A
  requirement joined by "and" needs both halves asserted or the tag overstates.
- A negative confounded by a guard that fires first. When the input crafted to
  violate one rule also violates an earlier one, the earlier rule does the
  failing and the rule under test is never reached.

**Mutation is the cheap answer and MUST be preferred to argument.** Revert the
behavior in a throwaway copy and run the test. A verdict reached that way costs
one edit and one run, and it is the only kind that survives someone reading the
test differently later. Where a test and the code it checks share an
implementation, the mutation MUST change them together: a test that recomputes
the answer the same way agrees with any error the implementation makes.

**When a test is repaired this way, its coverage MUST rise rather than move.**
A rewrite that pins the new contract is worth more than a deletion, and it keeps
the requirement proven while the contract changes underneath it.

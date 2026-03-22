---
name: No deferral on hard work
description: User expects hard phases to be implemented, not deferred. Deferring is not an acceptable response to difficulty.
type: feedback
---

Do not defer implementation phases because they are difficult or require deeper design work. The user delegates hard work to Claude specifically because it is hard. Deferring defeats the purpose.

**Why:** User explicitly said "stop deferring when it is hard." Trust and delegation means doing the hard thing, not punting it.

**How to apply:** When a phase seems complex (e.g., cross-goroutine coordination, synthetic events, protocol changes), implement it rather than marking it deferred. If genuinely blocked (missing information, architectural question only the user can answer), ask -- don't defer.

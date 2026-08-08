---
kind: note
level:
stage:
---
Two failures were already named in this repository and neither had an owning rule. `ai/rules/architecture.md` carried YAGNI and Simplicity as two rows in a 22-row design-principles table, and `ai/rules/quality.md` asked "Simplest solution? Over-engineered?" as one review question. Both are read after the design exists. Neither says what to do at the moment the shape of a fix is chosen, and neither is reachable from a trigger.

The standard is stated as a pair because each half fails alone. "Simplest" alone reads as permission to solve less of the problem, which rung 2 and rung 3 of `ai/rules/rule-precedence.md` already forbid. "Fully correct" alone is satisfied by any amount of machinery, which is how an interface with one implementation and an option nobody asked for enter the tree and stay there.

The cost this removes is paid by the next reader. Somebody who did not write the machinery cannot delete it safely, because the absence of a reason is not proof there was none. That is why `name-the-simpler-design-you-rejected` asks for one line. It lets the next reader delete the machinery, or keep it, on evidence.

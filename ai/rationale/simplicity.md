# Why the standard is "simplest AND fully correct"

The standard is stated as a pair because each half fails alone. "Simplest" alone
reads as permission to solve less of the problem, which rung 2 and rung 3 of
`ai/rules/rule-precedence.md` already forbid. "Fully correct" alone is satisfied
by any amount of machinery, which is how an interface with one implementation
and an option nobody asked for enter the tree and stay there.

The cost this removes is paid by the next reader. Somebody who did not write the
machinery cannot delete it safely, because the absence of a reason is not proof
there was none. That is why the rule asks for one line naming the simpler design
that was rejected. It lets the next reader delete the machinery, or keep it, on
evidence.

Rule: `ai/rules/simplicity.md`.

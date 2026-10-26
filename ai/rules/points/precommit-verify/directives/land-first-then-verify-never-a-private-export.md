---
kind: directive
level: MUST
stage:
rationale: ai/rationale/precommit-verify.md
---
**A change MUST land through `./le commit create` and its script as soon as its own package tests pass, and the commit MUST then be judged by `./le repo compiles check` and `./le verify changed`; an agent MUST NOT build its own export, archive or copy of HEAD to test a commit set before it lands, and a brief MUST NOT ask for one.** Those two checks already compile what git holds in a throwaway worktree, so a hand-built export repeats them at the cost of tens of tool calls, and an agent that spends its budget there hands a finished change to a successor only to commit it. A red they report is fixed forward in a new commit. A red that the change's own diff did not cause MUST NOT be investigated on the way to landing: the commit names it in its body. `./le verify changed` runs in the main thread, whose prompt cache outlives it.

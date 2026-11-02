---
kind: directive
level: MUST
stage:
rationale: ai/rationale/precommit-verify.md
---
**A change MUST land through `./le commit create` and its script as soon as its own scoped tests pass, and a commit that carried Go MUST then be judged by `./le repo compiles check`; an agent MUST NOT build its own export, archive or copy of HEAD to test a commit set, before or after it lands, and a brief MUST NOT ask for one.** The compile check already judges what git holds, so a hand-built export repeats it at the cost of tens of tool calls, and an agent that spends its budget there hands a finished change to a successor only to commit it. A red it reports is fixed forward in a new commit. A red that the change's own diff did not cause MUST NOT be investigated on the way to landing: the commit names it in its body.

---
kind: directive
level: MUST
stage:
rationale: ai/rationale/precommit-verify.md
---
**The main thread MUST land a code change through `./le commit create` and its script as soon as the change's owed commands pass, and MUST then judge a commit that carried Go with `./le repo compiles check`; an agent MUST NOT build its own export, archive or copy of HEAD to test a commit set, before or after it lands, and a brief MUST NOT ask for one.** The compile check already judges what git holds, so a hand-built export repeats it at the cost of tens of tool calls. The main thread makes the commit, so no successor agent is spent on it. A red the compile check reports is fixed forward in a new commit. A red that the change's own diff did not cause MUST NOT be investigated on the way to landing: the commit names it in its body.

---
kind: directive
level: MUST NOT
stage:
---
- **Review. 144 review agents were 15.4% of measured subagent context, and the fix/debug phase they prevent was 24.5%.** Cutting lenses, passes, or the model a reviewer runs on to save tokens MUST NOT happen: it costs more than it saves, and it is banned by `ai/rules/planning.md` independently of this measurement. `./le token-economy` prints both figures, and labels its phase split a keyword heuristic over the spawn description: nothing in the transcript store records the phase an agent ran.
- **Gates. A check, test, or verification target MUST NOT be skipped to save a round trip** (`ai/rules/completion.md`, `ai/rules/git-safety.md`). A gate not run is not a saving; it is an unmeasured risk.
- **The rules themselves. A rule you did not read is a rule you did not follow.** `ai/rules/TRIGGERS.md` names every rule in one line each precisely so the read is targeted, not skipped.

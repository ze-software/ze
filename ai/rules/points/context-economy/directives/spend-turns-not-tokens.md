---
kind: directive
level: MUST
stage:
rationale: ai/rationale/context-economy.md
---
- **Independent commands MUST go in ONE Bash call, and independent tool calls in ONE turn.** Every API call re-feeds the whole context, so at the 250k a session carries, one extra turn costs 25k tokens: the price of reading a 100KB file, paid for a result that is 600 tokens at the median. A `git status`, a `git diff --stat` and a `gopls symbols` that do not depend on one another are one call; three edits to three files are one turn.
- **A turn MUST NOT be spent to learn what a previous result already told you.** A `wc -l` before the read, a second `ls` of a directory listed a turn ago, a `cat` of a file the handoff digests: each re-feeds the context for nothing new.

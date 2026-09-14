---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/context-economy.md
---
- **A `<persisted-output>` file MUST NOT be read whole: read it with `grep` or `sed -n` for the lines the decision needs.** The harness writes that file when a command's output passes 33KB, and a whole read puts every one of those bytes into a context that is then re-fed on each later call.
- **A file MUST be read in ONE turn at the range the question needs, and MUST NOT be read in consecutive slices.** Each slice is a turn that re-feeds the whole context, so three `sed -n` slices of one file cost three contexts for one read. `gopls symbols` gives the range first; then one read of that range.
- **A rule, a page, or the spec that a brief names MUST be read once.** A phase agent re-reading a file the handoff already digests pays twice for one answer, and the digest tells it which lines to read when it needs more.

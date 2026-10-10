---
kind: directive
level: MUST
stage:
rationale: ai/rationale/user-facing-errors.md
---
- **A user-visible message MUST be easy to read and to scan: the problem first, then why it matters, then the command that fixes it on a line of its own, in plain English with no jargon.** The Simplified Technical English rule in `ai/rules/writing.md` applies to every such message, because its readers include non-native English speakers and people under pressure on a broken host. Color MAY highlight the important parts, the problem and the command to run, and SHOULD be used sparingly, in the roles `docs/architecture/cli/color-system.md` defines. Color MUST NOT carry meaning alone, so the words say the same thing with it off, and it MUST be off wherever `slogutil.UseColor` (`internal/core/slogutil/color.go`) turns it off: `NO_COLOR`, `TERM=dumb`, and any writer that is not a terminal, which covers a log written to a file. JSON output MUST NOT carry color codes.

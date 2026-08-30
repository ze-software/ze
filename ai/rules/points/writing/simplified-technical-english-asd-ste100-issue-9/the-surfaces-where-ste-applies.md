---
kind: directive
level: MUST
stage:
---
**STE MUST be applied exactly where this table says, and MUST NOT be applied where it says No:**

| Surface | STE applies |
|---------|-------------|
| `docs/` guides, references, comparisons, architecture pages | Yes |
| Code comments and godoc | Yes |
| Error messages, log lines, diagnostic remediation text | Yes, together with `ai/rules/cli.md` |
| CLI output, help text, completions, TUI labels | Yes |
| YANG `description` strings | Yes |
| `ai/` rules, patterns and digests, plus the durable half of `plan/`: journal rows, learned summaries, the template | Yes |
| Commit messages and PR text | Yes |
| A `plan/` document deleted at closure: `plan/spec-*.md`, a deferral shard, a known-failure shard | No. It is removed when the work closes, so nobody reads the edit |
| Chat replies, reports, and analysis for the user | No. Answer the person who asked |
| Thomas's authored prose: blog posts, articles, emails, the weekly update | No. That prose is his voice and it stays UK English |
| Identifiers, keys, tokens, quoted external text, and fixture data | No. `docs/contributing/writing-style.md`, "Words that never change" |

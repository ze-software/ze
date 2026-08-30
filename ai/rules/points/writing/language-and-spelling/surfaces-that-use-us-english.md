---
kind: directive
level: MUST
stage:
---
**Every surface below MUST be US English:**

| Surface | Examples |
|---------|----------|
| Go identifiers, types, functions, fields | `color`, `Normalize`, `Analyzer`, `licenseKey` |
| Code comments and godoc | "normalize the value", "this behavior" |
| Error messages, log lines, diagnostic codes | see `ai/rules/cli.md` |
| CLI output, help text, completions, TUI labels | `ze` command help, dashboards |
| YANG leaf descriptions and config help text | schema `description` strings |
| `docs/` user and architecture documentation | guides, references, comparisons |
| `ai/` rules, patterns, digests, indexes | this file included |
| `plan/` specs and journal rows | spec bodies, acceptance criteria |
| Commit messages and PR text | subject and body |

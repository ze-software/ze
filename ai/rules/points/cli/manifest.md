---
title: CLI and Output
when: adding or changing any CLI command, flag, exit code, output format, error message, JSON envelope, or agent-facing contract
severity: blocking
related: evidence, performance, protocol, repo-maintenance, git-safety
---
directives ## Directives
  answer-with-structured-data-never-pre-rendered-text
  name-json-keys-in-lowercase-kebab-case
cli-grammar-keywords-before-values ## CLI Grammar: Keywords Before Values
  the-first-token-after-the-noun-must-be-a-keyword
  the-verb-is-chosen-by-the-command-s-effect-on-live-state
  flags-belong-to-the-offline-tooling-only
  the-r1-to-r9-ruleset-and-where-it-is-implemented
cli-patterns ## CLI Patterns
  return-exit-codes-and-write-errors-to-stderr
agent-tooling-contract ## Agent Tooling Contract
  use-the-skill-instead-of-a-raw-agent

---
title: Go Standards
when: writing Go in Ze: naming, env access, logging, imports, typed-vs-string choices, external commands, or a compatibility shim
severity: blocking
related: config, cli, performance, repo-maintenance, architecture
---
directives ## Directives
  read-the-ze-style-guide-before-go-design-or-review
  guard-with-early-returns-one-fact-per-guard
  never-write-these-forbidden-go-patterns
  log-through-slog-never-printf
  access-env-vars-only-through-internal-core-env
  use-typed-numeric-identity-on-hot-paths
  ze-code-must-not-fork-a-system-tool
  ze-is-unreleased-so-write-no-compat-code

---
title: Pre-Commit Verification
when: before running tests to judge a change, before any `./le verify` command, or before committing Go
severity: blocking
---
directives ## Directives
  no-agent-runs-verify-and-tests-run-in-the-main-checkout
  only-one-check-compiles-what-git-holds
  land-first-never-a-private-export

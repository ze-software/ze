---
name: Codeberg CLI (tea)
description: tea CLI is available at /opt/homebrew/bin/tea for interacting with the Codeberg repository (PRs, issues, comments, API)
type: reference
---

`tea` (Gitea CLI) is installed and configured for the ze Codeberg repository.

Use `tea` instead of `gh` for all repository operations: PRs, issues, comments, labels, milestones, releases.

Key commands:
- `tea pr list`, `tea pr create` -- pull requests
- `tea issue list`, `tea issue create` -- issues
- `tea comment` -- add comments to issues/PRs
- `tea api` -- raw Gitea API calls (for anything not covered by subcommands)

Also documented in `.claude/rules/git-safety.md`.

---
spec: ze-agent-skill-setup
topic: CLI
feature: ai-first-design
---

## Before

An AI agent with a shell and the `ze` binary was told nothing about Ze. It
found the guides the binary carries only when you told it to run
`ze show skills`.

## After

`ze create agent skill`, run at a shell, sets your agent up. It installs a
skill named `ze` for Claude Code, Codex, OpenCode and Pi, whichever it finds
in your home directory, and the skill tells the agent to run
`ze show skill ze`, so the guide always matches the installed binary. For
Claude Code and Codex it also adds a session-start hook that runs
`ze show agent context`, so each session starts knowing the ze version,
whether the daemon answers and where the config lives. Codex runs the hook
only after you trust it through `/hooks`.

Ze edits a harness's settings file only where it understands the whole file,
leaves every other byte as it was, and never touches a skill or a hook it did
not write. `ze show agent skill` reports what is installed, and
`ze delete agent skill` removes it.

Through the daemon (MCP, SSH, REST, gRPC, the web and `ze cli`), the same
commands write nothing: they answer the skill a remote agent should place on
its own machine, which tells it to ask the router for `show skill ze`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none

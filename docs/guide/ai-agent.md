# Use Ze with an AI agent

If you test Ze with Claude Code or Codex, one command sets the agent up. It
installs a skill named `ze` that sends the agent to the guide your installed
`ze` serves, so the agent works from the version you run. For Claude Code and
Codex it also adds a hook, so every new session starts knowing the Ze version,
whether the daemon answers, and where the configuration is.
<!-- source: internal/plugins/skills/agent_skill.go -- harnesses, stubText -->
<!-- source: internal/plugins/skills/agent_hook.go -- hookCommand -->

Ze is pre-release software. Give the agent a lab, not a live network.

## Set the agent up

Run the command as the user who runs the agent, on the machine the agent runs
on:

```
$ ze create agent skill
Ze set the ze skill up for each harness below. A refused row names its fix.

claude skill: created (harness found)
  /home/you/.claude/skills/ze/SKILL.md

claude hook: created (harness found)
  /home/you/.claude/settings.json

codex skill: created (harness found)
  /home/you/.agents/skills/ze/SKILL.md
  Codex, OpenCode and Pi all read this one file.

codex hook: created (harness found)
  /home/you/.codex/hooks.json
  Codex skips a new or changed hook until you trust it. To trust it, open Codex, run /hooks, and trust the hook that runs ze show agent context.
```

Ze sets up each harness whose configuration directory exists: `~/.claude` for
Claude Code, `~/.codex` for Codex, `~/.config/opencode` for OpenCode and `~/.pi`
for Pi. To set up one harness before its directory exists, name it:
`ze create agent skill agent codex`. When Ze finds no harness, it writes only
the shared skill and says so. The command exits 1 when a row is refused, and
the row names the fix.
<!-- source: internal/plugins/skills/agent_skill.go -- harnesses, placeOf, localRun.note, exitCode -->

## What changed on your machine

| File | What Ze put there |
|------|-------------------|
| `~/.claude/skills/ze/SKILL.md` | The `ze` skill, for Claude Code |
| `~/.claude/settings.json` | A session-start hook that runs `ze show agent context` |
| `~/.agents/skills/ze/SKILL.md` | The `ze` skill that Codex, OpenCode and Pi read |
| `~/.codex/hooks.json` | The same session-start hook, for Codex |
<!-- source: internal/plugins/skills/agent_skill.go -- harnesses, harness.stubPath, harness.hookPath -->

When `CLAUDE_CONFIG_DIR` or `CODEX_HOME` is set, the Claude Code or Codex files
are under that directory. Ze adds its hook at the end of the session-start
list and keeps every other byte of the file, the hooks other tools wrote
included. It refuses a file it does not fully understand, leaves it as it is,
and says what it found. OpenCode and Pi get the skill only, because Ze knows no
session-start hook format for them.
<!-- source: internal/plugins/skills/agent_hook.go -- createHook, readLayout, settingsLayout.withGroup -->

The skill is short. It tells the agent to run `ze show skill ze` and follow
the guide it prints, and to run `ze help` if that command fails. Its first line
after the frontmatter is Ze's marker, which is how Ze knows the file is its
own.
<!-- source: internal/plugins/skills/agent_skill.go -- stubText, stubMarker -->

At each session start, the hook adds a few lines like these to the agent's
context:

```
ze dev (built unknown)
daemon: not answering at 127.0.0.1:2222 (dial tcp 127.0.0.1:2222: connect: connection refused)
config directory: /etc/ze
guide: the ze skill, read it with ze show skill ze
```

The hook waits at most one second for the daemon and always succeeds. When
`ze` is not on the path the harness sees, the hook prints nothing and the
session starts as usual.
<!-- source: internal/plugins/skills/agent_context.go -- dialBound, showAgentContextLocal, drawAgentContext -->
<!-- source: internal/plugins/skills/agent_hook.go -- hookCommand, hookTimeoutSeconds -->

## Trust the hook in Codex

Codex does not run a new or changed hook until you trust it. Open Codex, run
`/hooks`, and trust the hook that runs `ze show agent context`. Do this again
when a later `ze create agent skill` updates the hook. Ze never writes Codex's
trust store, so `ze show agent skill` can say the hook is installed, but it can
not say whether Codex trusts it.
<!-- source: internal/plugins/skills/agent_hook.go -- hookEditor.create, inspectHook -->

## Check and remove

```
ze show agent skill
ze delete agent skill
```

`ze show agent skill` reports, for each harness Ze knows, whether Ze found it
and the state of its skill and hook. It changes nothing. Run
`ze create agent skill` again after you upgrade Ze: it rewrites what an older
version wrote and leaves a current setup as it is.
<!-- source: internal/plugins/skills/agent_stub.go -- createStub, inspectStub -->
<!-- source: internal/plugins/skills/agent_hook.go -- inspectHook -->

`ze delete agent skill` removes only what Ze wrote. A `SKILL.md` without Ze's
marker and every hook Ze did not add stay as they are. Without `agent` it acts
on every harness Ze finds, as `agent all` does, and `agent <name>` limits it to
one harness. When you name one harness, the shared skill
under `~/.agents` stays while another harness on the machine still reads it.
<!-- source: internal/plugins/skills/agent_stub.go -- removeStub, keptSharedStub -->
<!-- source: internal/plugins/skills/agent_hook.go -- deleteHook -->

## What to ask the agent first

Ask it to use the ze skill, for example "use the ze skill and check my
configuration". The agent reads the guide itself: the skill sends it to
`ze show skill ze`, the guide built into your `ze`, which names the entry
points (configuration validation, diagnostic codes, repair plans and the
command reference from `ze help ai --json`). `ze show skills` lists the other
guides the binary carries, and the agent loads the one its task needs with
`ze show skill <name>`. The guides come from the binary, so they change when
you upgrade it.
<!-- source: internal/plugins/skills/data/ze.md -- Agent Entry Points -->
<!-- source: internal/plugins/skills/main.go -- inventory, showSkills, showSkill -->

## An agent that reaches a remote router

When the agent runs on another machine and reaches Ze over MCP, REST, gRPC,
SSH or the web, send the same commands over that connection, for example
`create agent skill` through the `ze_execute` MCP tool. The router reads and
writes nothing. It answers, for each harness, the path relative to your home
directory and the skill to write there, on the agent's machine. That skill
tells the agent to ask the Ze system it is connected to for `show skill ze`
over the connection it already uses. There is no hook, because the hook needs
a local `ze`. To connect an agent over MCP, see [MCP Integration](mcp/overview.md).
<!-- source: internal/plugins/skills/agent_skill.go -- remoteAnswer, remoteNote, stubText -->

## Details

The full behavior of each command (every state a row can answer, how Ze edits
the settings files, what it refuses) is in
[Setting an agent up](../features/ai-first.md#setting-an-agent-up) and
[Session context](../features/ai-first.md#session-context). The command
reference has one entry per command:
[create agent skill](command-reference.md#create-agent-skill),
[show agent skill](command-reference.md#show-agent-skill),
[delete agent skill](command-reference.md#delete-agent-skill) and
[show agent context](command-reference.md#show-agent-context).

---
name: ze
description: Ze network OS overview and agent entry points (expanded).
---

# Ze

Ze is a network OS in Go with its own BGP implementation, YANG-modeled config,
SSH-accessible CLI, HTMX web UI, and agent-facing tooling.

## Agent Entry Points

```sh
ze cli -c "show system triage | json"
ze cli -c "show config validation <file> | json"
ze show diagnostic [--json] <diagnostic-code>
ze config fix --plan <file>
ze help ai --json
ze show skills [--json]
ze show skill <name> [--full] [--json]
```

## Diagnostic Loop

1. Run `ze cli -c "show config validation <file> | json"` to get structured diagnostics.
2. For each diagnostic, check `code` and `help` fields.
3. Run `ze show diagnostic <code>` for codes you do not recognize.
4. Run `ze config fix --plan <file>` for repair candidates.
5. Apply the smallest fix, then re-validate.

## Live Runtime Triage Loop

1. Run `ze cli -c "show system triage | json"` (or MCP `ze_show_system` action `triage`) to capture build provenance, `health`, `doctor`, `errors`, `warnings`, `crashes`, `plugins`, `reload`, `recent-logs`, and `runtime` in one call.
2. Inspect `doctor`, `errors`, `crashes`, and `recent-logs` first (including any `doctor-internal-defect` raised by a recovered panic).
3. Drill down only into the sections triage flags: `show crashes name <file>`, `show log recent level err`, `show event recent`, `show system goroutines mode blocked`, `show system profile`, `show reload-status`, or `show plugin list`.
4. Load `ze show skill ze-diagnostics` for the full runtime triage playbook and bug report checklist.

## Skills

Use `ze show skills` to see all bundled skills.
Use `ze show skill ze-diagnostics` for config diagnostics and live runtime triage.
Use `ze show skill ze-config` for config syntax and common errors.
Use `ze show skill ze-commands` for CLI and dispatch key reference.
Use `ze show skill ze-agent` for the agent edit loop.

## Agent Setup

At a shell, `ze create agent skill` sets an agent up. It writes a stub skill named `ze`
for each agent harness it finds (Claude Code, Codex, OpenCode, Pi). The stub tells the
agent to run `ze show skill ze`, so the guide always matches the installed binary. For
Claude Code and Codex it also installs a session-start hook that runs
`ze show agent context`, which prints the Ze version, whether the daemon answered, and
the config directory. `agent <name>` sets up one harness only.

- `ze show agent skill` reports each harness, the state of its stub and hook, and the
  path it read. It changes nothing.
- `ze delete agent skill` removes what Ze installed. `agent <name>` removes
  it from one harness, and keeps the shared stub while another harness found reads it.
- Codex runs a new hook only after you trust it: open Codex, run `/hooks`, and trust
  the hook that runs `ze show agent context`.

Through the daemon (MCP, REST, gRPC, SSH, web, `ze cli`), the three `agent skill`
commands write no file. They answer what an agent on another machine places in its own
home.

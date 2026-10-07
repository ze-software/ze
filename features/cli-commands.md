# CLI Commands

## Meta

| Field | Value |
|-------|-------|
| Name | CLI Commands |
| Page | docs/features/cli-commands.md |
| Kind | daemon |
| Scope | partial |
| Scope gaps | YANG RPC declarations with no handler (plan/immediate/spec-yang-rpc-declarations-with-no-handler.md), published RPC names that do not reach their handler (plan/immediate/spec-rpc-published-name-does-not-reach-its-handler.md), declared commands without leaves (plan/immediate/spec-declared-commands-without-leaves.md) |
| Level | experimental |
| Components | internal/plugins/signal/main.go, internal/component/ssh/ssh.go, internal/component/command |
| Real-path tests | test/ui/cli-verb-daemon-dispatch.ci, test/ui/send-old-paths-are-refused.ci, test/ui/help-parent-node.ci |
| Docs | docs/features/cli-commands.md |
| Doc review | 2026-10-07: ssh.go intercepts stop, restart and reboot (commandReboot) in the exec middleware as the row says |
| Defect review | 2026-10-07: open: the three Scope gaps specs plus plan/immediate/spec-cli-question-mark-cannot-be-typed.md |
| Extra criteria | supported: a gate that every declared command path has a handler = none yet |

## Description

Protocol tools, config management, schema discovery, daemon control, AS topology graph, policy dry-run testing (`show policy test`). Verb-first is now the only spelling: every command starts with `show`, `clear`, `monitor`, `request`, `set` or `delete`, and the words after it are the YANG path. The bare forms are gone from the command tree, not aliased, so the dispatcher answers `unknown command` for `daemon reload`, `daemon status`, `daemon quit`, `bgp summary`, `system memory`, `interface rate` and the rest; the verb-first spellings are `request reload`, `show status`, `request halt`, `show bgp`, `show system memory`, `show interface rate`. Three exec verbs stay bare on purpose (`stop`, `restart`, `reboot`): the SSH exec middleware intercepts them before the dispatcher, which registers no key for them. `show interface` reshaped its keywords too: `name <n> detail` and `name <n> counters` replace `detail <n>` and `counters <n>`, and `brief`, `scan`, `type`, `errors` and `rate` each answer a distinct question. <!-- source: internal/plugins/signal/main.go -- ExecCommand mapping and the removed spellings --> <!-- source: internal/component/ssh/ssh.go -- execMiddleware -->

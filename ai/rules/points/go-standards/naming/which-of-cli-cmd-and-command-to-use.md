---
kind: table
level:
stage:
---
| Package | Is | Use it when |
|---------|----|-------------|
| `component/cli` | The unified interactive TUI: config editing, CLI, and SSH sessions. | Adding an interactive surface or TUI behavior |
| `component/cmd` | A namespace of top-level CLI VERB implementations -- one subpackage per verb (`clear`, `delete`, `log`, `meta`, `metrics`, `monitor`, `set`, `show`, `subscribe`, `update`). | Adding or extending a top-level verb: `component/cmd/<verb>` |
| `component/command` | Shared types and logic for operational command execution (grammar, registry) consumed by the other two. | Adding command plumbing that more than one verb or surface needs |

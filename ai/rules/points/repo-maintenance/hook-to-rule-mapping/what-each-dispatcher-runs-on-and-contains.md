---
kind: table
level:
stage:
---
| Go source | Runs on | Contains |
|---|---|---|
| `internal/le/hookruntime/bash.go` | PreToolUse `Bash` | every registered Bash check below |
| `internal/le/hookruntime/writeedit.go` | PreToolUse `Write\|Edit\|MultiEdit\|NotebookEdit` | every registered Write/Edit check below |
| `internal/le/hookruntime/agent.go` | PreToolUse `Task\|Agent` | skill routing, review-model enforcement, and the Go style-guide reminder |
| `internal/le/hookruntime/postwrite.go` | PostToolUse `Write\|Edit` | formatting and post-edit advisory checks |

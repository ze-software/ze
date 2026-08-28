# Claude Hooks

All registered hooks execute in the compiled root `le` command. The canonical
configuration is `.claude/settings.json`; each entry invokes:

```text
$CLAUDE_PROJECT_DIR/le hook-check <hook-name>
```

The runtime lives in `internal/le/hookruntime`. `internal/le/hookcheck` owns the
registered command surface, the 208 typed dispatcher fixtures, and the 607 typed
behavior fixtures. Hook payloads are read from standard input as JSON. Hook
protocol output is written directly by the Go runtime, and exit codes retain the
Claude severity contract: 0 permits, 1 warns, and 2 blocks.

## Native runtime families

| Hook family | Native owner |
|---|---|
| Bash command guards | `internal/le/hookruntime/bash.go` |
| Write/Edit guards | `internal/le/hookruntime/writeedit.go` |
| Post-write formatting and advisories | `internal/le/hookruntime/postwrite.go` |
| Agent skill and review-model gates | `internal/le/hookruntime/agent.go` |
| Session identity and parent propagation | `internal/le/hookruntime/session.go` |
| Session, marker, compaction, stop, and validation hooks | `internal/le/hookruntime/lifecycle.go` |
| JSON dispatch and shared scratch identity | `internal/le/hookruntime/runtime.go` |

Session identity resolution and dated session paths are canonical in
`internal/le/lepath/session.go`. Test weakening is judged by
`internal/le/testweakened`, journal rows by `internal/le/journal`, and running review
models by `internal/le/specsession`. The hook runtime calls those packages
in-process rather than launching a second implementation.

Run the focused hook proof with:

```text
./le hook-check unit
```

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-10 | spec-session-editor-file-mode-parity | `checkLiveConflicts` (`internal/component/cli/editor_commit.go`) | No production caller: commit's LIVE conflicts come from `Editor.detectConflicts` (`editor_draft.go`), yet two `editor_test.go` tests hold `checkLiveConflicts`, so they pass over code no commit runs. Found when a mutation of it left `session-editor-load-conflict.ci` green | open: delete the function and its two tests, or route commit through it |

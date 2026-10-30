# Per-request state is written into an object every request shares

One instance serves all callers, and each call stores its own input on it before
using it, so concurrent calls race and one caller can act on another's data.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | web CLI completion: one shared `cli.Completer` (`cmd/ze/hub/service_web.go`), `completeCLIInput` (`web/cli.go`) | Each request called `SetTree` then `Complete` on the shared completer with no lock, so two users completing at once raced and could be offered names from the other's candidate config | fixed: `Complete` and `GhostText` take the tree per call, `SetTree` and `contract.Completer` deleted, the web completes under `EditorManager.ReadTree`; `TestWebCompletionPerUserTree` (-race), `TestCompleterTreePerCall` |
| 2026-10-30 | config-reference-completion (found, not caused) | `EditorManager.Tree` (`web/editor.go`) and its non-completion web callers | `Tree` returns the session's live tree after releasing the session lock, so a handler walking it can race that same user's concurrent edit from another tab; only completion moved to `ReadTree` | open: move every remaining reader to `ReadTree` (or a snapshot) and delete `Tree`, test two concurrent requests of one user under -race |

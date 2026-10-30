# Per-request state is written into an object every request shares

One instance serves all callers, and each call stores its own input on it before
using it, so concurrent calls race and one caller can act on another's data.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | web CLI completion: one `cli.Completer` from `cli.NewCompleter` in `cmd/ze/hub/service_web.go`, `completeCLIInput` (`web/cli.go`) | Every request calls `SetTree(mgr.Tree(username))` then `Complete` on the shared completer with no lock, and `EditorManager.Tree` releases the session lock before the walk, so two users completing at once race and can be offered names from the other user's candidate config | open: pass the tree into `Complete` per call instead of storing it, test two users under -race |

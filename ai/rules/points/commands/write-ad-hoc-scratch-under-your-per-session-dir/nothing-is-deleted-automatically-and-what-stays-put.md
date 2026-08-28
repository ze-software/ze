---
kind: note
level:
stage:
---
**Nothing under `tmp/session/` is ever deleted automatically**: not at session
end, not on an age timer, not by a hook. Your directory outlives your session,
so a log you wrote is still there tomorrow. `./le session reap` removes only
session directories whose owners are provably gone. Do NOT relocate artifacts
that are already session-keyed (commit scripts under the session directory) or
shared by design (`tmp/ze-verify.*` and the durable cache); those stay put.
`internal/le/gotoolchain` assigns the repository Go build cache to native test
and verification actions.

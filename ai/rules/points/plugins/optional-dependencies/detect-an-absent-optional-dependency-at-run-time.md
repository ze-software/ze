---
kind: directive
level:
stage:
---
1. Dispatch the command targeting the optional dep normally.
2. If the response returns the engine's `ErrUnknownCommand` (propagated as a
   string across the plugin IPC boundary), treat it as the "plugin absent"
   signal.
3. Use `sync.Once` to log one `WARN` per process lifetime; skip the feature
   (e.g. replay convergence loop) and continue with the rest of the flow.

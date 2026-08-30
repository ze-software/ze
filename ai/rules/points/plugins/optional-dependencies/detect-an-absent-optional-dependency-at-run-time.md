---
kind: directive
level: MUST
stage:
---
- **The owner of an optional dependency MUST detect its absence at run time and fall back cleanly.** It MUST treat the engine's `ErrUnknownCommand`, propagated as a string across the plugin IPC boundary, as the plugin-absent signal, and it MUST use `sync.Once` to log one `WARN` per process lifetime before skipping the feature and continuing. `bgp-rs` disabling replay when `bgp-adj-rib-in` is absent is the worked example.

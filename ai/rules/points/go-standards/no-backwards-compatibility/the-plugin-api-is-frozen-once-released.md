---
kind: directive
level: MUST NOT
stage:
---
**The only exception is the plugin API**, the surface that external plugin authors compile against (`pkg/plugin/` SDK types, the JSON event / text command protocol between core and plugins, and anything re-exported for plugin consumption). Once released, that surface MUST NOT break. Everything else under `internal/` remains free to change.

---
kind: directive
level: MUST NOT
stage:
---
- **A plugin's declaration MUST NOT be able to panic the daemon.** The shape, column and address-field registries keep their panic on `declare`, which only in-tree Go reaches. A plugin's declaration goes through `declareFor` instead: the same cases with the panic replaced by an error, so a conflicting plugin is refused and the daemon keeps running. `RegisterPluginShapes` is the only caller-facing write, and `UnregisterPluginShapes` takes the whole declaration back when the plugin stops.
<!-- source: internal/component/command/column_order.go -- declare, declareFor, withdraw -->

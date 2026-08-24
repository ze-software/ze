---
kind: note
level: MUST NOT
stage:
---
**A plugin declaration MUST NOT be able to panic the daemon.** The three
registries keep their panic on `declare`, which only in-tree Go reaches. A
plugin's declaration goes through `declareFor` instead. That is the same four
cases with the panic replaced by an error, so a conflicting plugin is refused
and the daemon keeps running. `RegisterPluginShapes`
(`internal/component/command/answer_shape.go`) is the only caller-facing write.
`UnregisterPluginShapes` takes the whole declaration back when the plugin stops.
<!-- source: internal/component/command/column_order.go -- declare, declareFor, withdraw -->

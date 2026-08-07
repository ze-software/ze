---
kind: note
level:
stage:
---
Internal plugins that register `OnStructuredEvent` receive `*rpc.StructuredEvent` instead of formatted text. The engine delivers pre-extracted peer metadata + `RawMessage` pointer, eliminating JSON formatting on the engine side and `ParseEvent` on the plugin side.

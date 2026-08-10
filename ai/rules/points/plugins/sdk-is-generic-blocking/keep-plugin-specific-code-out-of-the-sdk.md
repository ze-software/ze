---
kind: directive
level: MUST NOT
stage:
---
**The SDK (`pkg/plugin/sdk/`) MUST NOT contain plugin-specific code.** Adding or removing
a callback type requires only one `On*` method in `sdk_callbacks.go` that registers a
handler in the callback map. The event loops, dispatch logic, and transport layers are
callback-agnostic: they dispatch through `map[string]callbackHandler` without knowing
what callbacks exist.

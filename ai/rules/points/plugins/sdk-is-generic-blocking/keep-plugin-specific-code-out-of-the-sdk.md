---
kind: directive
level: MUST NOT
stage:
---
- **The SDK (`pkg/plugin/sdk/`) MUST NOT contain plugin-specific code.** Adding or removing a callback type is one `On*` method in `sdk_callbacks.go` and nothing else: the event loops, the dispatch logic and the transport layers dispatch through `map[string]callbackHandler` without knowing which callbacks exist. What that property requires is `docs/architecture/plugin/plugin-system.md`, "The SDK stays generic".

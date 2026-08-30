---
kind: directive
level: MUST
stage:
---
- **The mechanism behind every directive here is documented, and the page MUST be read before the plugin work it covers:** `docs/architecture/plugin/plugin-system.md` for registration, the engine boundary, the communication patterns, `OnStarted` against `OnAllPluginsReady`, role claims, the peer-up barrier, answer-shape and pipe-alias declaration; `docs/architecture/plugin/feature-gates.md` for compile-out; `docs/architecture/command-ownership.md` for command placement; `docs/architecture/api/process-protocol.md` for the wire protocol and the accumulator arity; `ai/patterns/plugin.md` for the file template and the new-plugin checklist; and `pkg/plugin/rpc/bridge.go` before any new core-to-plugin plumbing, because DirectBridge carries request and response where the EventBus MUST NOT.

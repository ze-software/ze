---
kind: directive
level: MUST
stage:
---
- **Every test file that defines a private mock of `ze.EventBus` MUST carry a compile-time check in that same file: `var _ ze.EventBus = (*<stubName>)(nil)`.** Without it, an interface change compiles the stub against an outdated signature and fails only when a test actually constructs the stub.

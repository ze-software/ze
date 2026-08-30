---
kind: directive
level: MUST
stage:
---
**Each trigger below MUST be answered by the audit named beside it, in the same commit.**

| Trigger | Action |
|---------|--------|
| A `nil` check on a result | Check every other caller for the same nil case |
| A fallback when an external system is unavailable | Check every other caller of that dependency |
| A retry or backoff | Check every other caller doing the same I/O |
| A new error-wrapping context | Check every other caller wrapping the same error |
| Replacing a direct call with a helper | Check every other caller that SHOULD use the helper |
| Changing or removing how a binary is invoked | Search EVERY invocation of the bare token: `.ci` directives, embedded `tmpfs=` bodies, compiled drivers under `internal/test/fixture`, runner launch code, native actions, and docs. The complete affected suite MUST be proven, never a sample |

---
kind: table
level:
stage:
---
| Trigger | Action |
|---------|--------|
| `nil` check on a result | Check every other caller for the same nil case |
| Fallback when external system unavailable | Check every other caller of the dependency |
| Retry / backoff | Check every other caller doing the same I/O |
| New error-wrapping context | Check every other caller wrapping the same error |
| Replace direct call with helper | Check every other caller that should use the helper |
| Change or remove how a binary is invoked | Search EVERY invocation of the bare token, including `.ci` directives, embedded `tmpfs=` bodies, compiled drivers under `internal/test/fixture`, runner launch code, native actions, and docs. Prove the complete affected suite, never a sample (learned 1248) |

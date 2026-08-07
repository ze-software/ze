---
kind: table
level:
stage:
---
| Situation | Why |
|-----------|-----|
| Config tree (`map[string]any`) | YANG-parsed, accessed once at load |
| CLI dispatch table (built at init) | Looked up once per command, not per-UPDATE |
| JSON marshal/unmarshal | External format requires string keys |
| User-facing display | One-shot, cold path |
| Map built and discarded in a test | Not production code |

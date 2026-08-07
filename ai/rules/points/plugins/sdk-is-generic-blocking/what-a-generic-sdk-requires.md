---
kind: table
level:
stage:
---
| Rule | Meaning |
|------|---------|
| No switch/case on method names in event loops | Dispatch is map lookup, not enumeration |
| No transport-specific handler methods | One handler per callback, used by both pipe and bridge |
| Bye is the only special case | It terminates the loop -- checked by method name, not by handler signature |
| Adding a callback = one On* method | Zero changes to sdk_dispatch.go or event loop code |

---
kind: directive
level:
stage:
---
**Never use switch/case to dispatch subcommands.** All command dispatch must use the registration pattern: register handlers into a dispatcher (or sub-dispatcher), then call `Dispatch(args)`. This applies at every level of nesting.

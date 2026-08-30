---
kind: directive
level: MUST NOT
stage:
---
- **A client MUST NOT build a flag into a command string it sends to the daemon.** `(*Dispatcher).Dispatch` (`internal/component/plugin/server/command.go`) refuses any flag-shaped token before the handler runs, so such a command fails on every invocation while its client half and its daemon-side parser both read as finished code.

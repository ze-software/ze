---
spec: ai-diagnostics-3-external-plugin-introspection
topic: Diagnostics
---

## Before

`show system goroutines`, `show system profile`, `show runtime memory`, `show system memory`, and `show log recent` inspected only the daemon process itself. When a forked external plugin deadlocked, leaked memory, spun CPU, or logged errors inside its own process, operators and AI assistants over CLI or MCP had no built-in way to inspect its goroutines, profiles, Go heap, OS memory map, or structured log ring. Separate-process compiled-in plugins also started without mutex and block profile sampling enabled.

## After

All five diagnostic commands now accept a command-owned `| plugin <name>` pipe filter (`pipe: [{"name": "plugin", "args": ["<name>"]}]` over MCP, composable with `| summary` on `show system profile`), routing identically to in-process plugins over `DirectBridge` and forked external plugins over TLS via an auto-started independent `ze-plugin-callback:runtime-inspect` SDK worker that keeps answering even when a plugin's ordinary event loop is deadlocked. Responses tag `plugin`, `process-mode` (`internal` or `external`), and `pid`, and separate-process Ze plugins (`ze plugin <reg>`) enable the same low-overhead mutex and block sampling defaults as the daemon so `mutex` and `block` profiles report `sampling-rate`. When a forked external plugin cannot answer (wedged runtime, timeout, disconnected transport, or non-Go binary), the daemon never sends `SIGQUIT` or any terminating signal; it returns a non-destructive `fallback: true` snapshot carrying `fallback-reason`, Linux `/proc/<pid>` metrics (`state`, `wchan`, `threads`, `vm-rss-kb`, `fd-count`), and the plugin's last 32 `stderr-tail` lines. `show skill ze-diagnostics` documents the `| plugin <name>` drill-down workflow and fallback interpretation.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none

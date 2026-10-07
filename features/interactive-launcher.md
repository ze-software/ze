# Interactive Launcher

## Meta

| Field | Value |
|-------|-------|
| Name | Interactive Launcher |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | cmd/ze/tui_menu.go |
| Real-path tests | test/ui/tui-noargs-nontty-fallback.ci |
| Docs | docs/guide/command-reference.md |
| Doc review | 2026-10-07: command-reference.md documents the no-argument launcher and cites runTUILauncher; only the non-TTY fallback has a functional test |
| Defect review | 2026-10-07: no open spec or journal row found naming the launcher |
| Extra criteria | supported: the TTY menu driven under a pty = none yet |

## Description

Running `ze` with no arguments in a terminal shows a BubbleTea menu of all commands grouped by section, with type-ahead filtering, scrolling, and drill-down into YANG verb sub-commands. Non-TTY invocations fall back to static help text. <!-- source: cmd/ze/tui_menu.go -- runTUILauncher, buildTopLevel -->

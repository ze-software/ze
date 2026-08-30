---
kind: directive
level: MUST NOT
stage:
---
**Shipped Ze code MUST NOT run an external binary: no `exec.Command`, no `exec.CommandContext`, no shell.** Ze runs on an appliance image carrying no shell utilities, so a fork depends on a binary the environment lacks and answers with a second implementation's opinion where Ze holds its own; take the native path (`vishvananda/netlink`, a hand-built netlink request, `os.ReadFile` over `/proc` and `/sys`, `x/sys/unix`). Thomas authorises every exception and it MUST carry a row in `ai/allowed-system-commands.md` before the code lands, so an agent that believes a fork is unavoidable MUST state the case and STOP. `_test.go`, `test/`, `internal/test/` and `internal/le/` drive a developer machine and are outside this rule; everything Ze ships under `cmd/`, `internal/` and `pkg/` is inside it.

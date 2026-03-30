---
name: CLI dispatch discovery gaps
description: Three missing features that block debugging a running ze daemon - non-interactive CLI, dispatch string listing, help showing dispatch keys not RPC names
type: project
---

Three gaps discovered 2026-03-30 during live debugging:

1. **No non-interactive one-shot command from the terminal.** `ze show`/`ze run` already use SSH (`sshclient.ExecCommand`) internally, but there is no simple `ze cli -c "summary"` that a user can type at a shell prompt. SSH required sshpass (not installed), web UI rejected curl. User was stuck.

2. **`ze help --ai --api` shows YANG RPC names, not dispatch strings.** It shows `ze-bgp:summary` but the user has no way to know if the dispatcher matches on `ze-bgp:summary`, `summary`, `peer summary`, or `show summary`. Help should show the string you actually type to execute a command.

3. **No command to list dispatcher match keys.** Something like `ze help --ai --dispatch` that prints every key in the Dispatcher's sorted lookup table would let users discover valid command strings in seconds.

**Why:** `reactor.ExecuteCommand()` is the entry point but its accepted strings are undiscoverable without reading source code. This blocks anyone trying to debug or integrate.

**How to apply:** These are cli-dispatch spec items. The non-interactive CLI command is the highest priority since it unblocks debugging workflows entirely. Connection is via SSH (port 2222, credentials from zefs database), not TLS or Unix socket.

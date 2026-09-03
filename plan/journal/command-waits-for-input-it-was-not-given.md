# Command waits for input it was not given

A binary reads its payload from stdin because a hook runner feeds it there. The
same binary is reachable as an ordinary command, and then nothing feeds it. It
does not refuse. It blocks in the read, prints nothing, and outlives the command
that started it, so the cost lands on a later reader who finds a process with no
output and no explanation.

The refusal already exists and is well written. What is missing is the moment it
fires: the tool knows what an absent payload means, and never gets to say so
because it is still waiting for one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-03 | - | `./le hook-check validate-spec` | The action is a PostToolUse hook: it reads a JSON payload on stdin and takes no arguments. Fed one, it refuses correctly with "no tool name in the hook payload -- NOTHING WAS CHECKED". Given a stdin that never reaches EOF, which is what an agent's Bash call inherits, it blocks in that read with no output and no bound. Three were alive on this machine at once, aged 59 minutes, 7 hours and 21 hours, from three sessions, each having outlived the command that started it | FIXED. `payloadReader` (`internal/le/hookcheck/actions.go`) bounds the FIRST byte at 10 seconds with a timer over a goroutine, and `reportNoPayload` says the verb is a hook rather than a command. Only the first byte is bounded, because a payload that has started arriving is a real invocation whose checks take as long as they take. The first attempt used `os.File.SetReadDeadline` and shipped a no-op: the Go runtime never registers the standard streams with its poller, so a deadline on `os.Stdin` reports `ErrNoDeadline` and bounds nothing. Three unit tests passed over an `os.Pipe`, which is pollable, and the end-to-end run still waited the full 120 seconds with nothing on stderr. The tests now model stdin with a plain reader that never returns |

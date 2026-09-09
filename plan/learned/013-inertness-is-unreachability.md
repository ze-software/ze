# Learned: a mode a plugin READS is a promise, a mode it never REACHES is a guarantee

The owner asked for a way to start a plugin and ask it what it declares, with
the plugin aware that it is being interrogated and doing none of a live start's
work: no data, no connection, no bind, no listener, no timer.

The obvious shape is a mode the plugin reads at the top of its own runner. That
shape cannot deliver the guarantee, and the measurement says why. Of the 92
registered runners, 19 do work that reaches outside their own locals BEFORE the
declaration is sent: `flowspec-firewall` and `ike` program nftables and XFRM,
`trafficusage` calls `setrlimit` on the calling process, `fib/kernel` opens a
netlink handle, four allocate the process-wide default Loc-RIB, three start
goroutines, and nine leave a process global pointing at a plugin that is about
to die. A mode read inside the runner body is read AFTER all of it.

**So the answer is written by the process that already holds the declaration,
and the runner is never entered.** `cli.Run`
(`internal/component/plugin/cli/main.go`) looks the plugin up and holds the
whole `*registry.Registration` before it calls `reg.CLIHandler`. Query mode is
answered there, from that value. For a plugin the ze binary carries, no plugin
code runs at all, so inertness is a property of unreachability rather than of a
promise anyone has to keep.

## Where Ze does not own the binary, say CONVENTION and mean it

A third-party plugin is another program. Nothing Ze writes can stop its `main`
from binding a socket. The SDK entry point takes the declaration and the
activation function as SEPARATE arguments (`sdk.RunOrDeclare`,
`pkg/plugin/sdk/sdk_query.go`), so an adopter's side effects are unreachable
under the mode for the same reason: they live inside an argument that is never
called. Work the author does ABOVE that call still runs, and the docs say so.

A plugin that adopts neither route is reported `no-answer`. It is never guessed
at, and the reader passes it no hub host, no port, no token, no CA and no plugin
name, so a plugin that ignores the mode and starts for real fails to connect
rather than joining a live daemon's hub.

The general shape: when a guarantee cannot be enforced for one population,
enforce it for the population you own, name the other one, and give it an entry
point rather than a rule. Do not write one sentence that reads as a guarantee
for both.

## "Declared nothing" and "sent nothing" are different answers

Roughly two thirds of the runners declare an empty command list, so "nothing" is
the NORMAL answer and never a failure signal. A reader that folded an empty
declaration together with silence would report every plugin without query mode
as a plugin with nothing to say. The five states keep them apart, and a row is
never dropped: `declared`, `declared-none`, `no-answer`, `unstartable`,
`timeout` (`internal/component/plugin/declarations.go`).

`unstartable` needed the child's exit status, which is why the reader forks the
run string itself rather than calling `(*Process).startExternal`: that path
discards the status, and under `/bin/sh -c` a missing binary always starts the
shell, so only the 126/127 exit separates "never ran" from "ran and said
nothing".

## Two defects the proof found, and both were older than the spec

The spec's AC-7 compares one declaration through the live SDK startup and
through `sdk.RunOrDeclare`. It could not be made to pass, and the code was
wrong rather than the test.

**`Run` declared `WantsValidateOpen` for every plugin ever built.** It read
`p.callbacks[callbackValidateOpen] != nil`, and `initCallbackDefaults` installs
a default accept-everything handler for that callback, so the entry is present
for every plugin. Presence answered "how do I reply", and the code read it as
"ask me". The engine therefore asked every plugin to validate every OPEN. The
field is now `p.wantsValidateOpen`, set only by `OnValidateOpen`. Only the
`role` plugin registers that handler.

**A row whose alphabetically first key held a list rendered YAML no parser
accepts.** `writeKeyValue` (`internal/component/command/format.go`) took ONE
string for two jobs: what opens the key's own line, and what opens every line
under it. For a sequence item the first is `- `, and a child line that inherited
it started a new sequence entry. Any command whose rows carry a list under a key
that sorts first was affected; `show plugin declarations` is one, with
`commands` sorting before `kind`, `name` and `state`.

Both are in `plan/journal/field-carries-two-meanings.md`. The transferable part
is the same in each: one value carried two meanings, and the second meaning had
no test because it had no name.

## A stop must reach the process, not the wrapper in front of it

A `run` string is given to a shell, and a shell that does not exec-optimize it
keeps the plugin as its own child. Stopping the direct child stops the shell and
leaves the plugin running, which for a query is the one outcome the whole
feature exists to prevent. Two halves are needed, `Setpgid` at the fork and a
kill aimed at the group, and each half alone stops nothing extra: a group kill
with no `Setpgid` signals a group no process heads, and the kernel answers ESRCH
silently. `KillGroupOnCancel` (`internal/component/plugin/sysproc.go`) sets both
and is the only caller of `NewSysProcAttr`, so the pair cannot be taken apart.
The daemon's live start took the same fix, and it is daemon-visible: a stopped
external plugin now really stops.

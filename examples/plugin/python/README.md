# A Ze plugin in Python

This directory holds an external Ze plugin written in Python, and the small
library it is built on. It shows that a plugin in any language brings its own
YANG modules to Ze: a config section, commands with checked arguments, and the
parameters the API surfaces publish for those commands. Ze treats them as it
treats the modules compiled into `ze`.

| File | What it is |
|------|------------|
| `ze_plugin.py` | The library. One file, standard library only, Python 3.9 or later |
| `acme_monitor.py` | The example plugin, its three YANG modules written inline |

The wire contract is `docs/plugin-development/protocol.md`, and defining a
command in YANG is `docs/plugin-development/commands.md`. The library speaks
plugin protocol version 1.

## What the example declares

| Module | What it adds |
|--------|--------------|
| `acme-monitor-conf` | The config section `acme-monitor`: a mandatory `target`, the host the monitor probes, of 1 to 64 characters, and an `interval` from 1 to 10 seconds with no default. The leaves are those of the module Ze's tests declare under the same name (`internal/test/externalyang/externalyang.go`), so a config for this example is valid in Ze's repository config sweep too |
| `acme-monitor-cmd` | The commands `show acme-monitor status` and `show acme-monitor target <name>`, with their help, and the argument `name`, a string of 1 to 16 characters |
| `acme-monitor-api` | The rpc `show-target`, whose input is the parameter list MCP, REST and gRPC publish for `show acme-monitor target` |

`plugin.schema()` is called once for each module. Each `@plugin.command`
lists its command in the Stage 1 declaration, which is what makes Ze ask the
plugin for it, and serves it when Ze asks.

What the plugin does with them:

- `verify` refuses a target written as a URL (`https://web.example.net/`),
  and the refusal names the `set` line that fixes it.
- `apply` stores the target and the interval, and logs them with the changed
  config paths. `rollback` puts the previous values back.
- `show acme-monitor status` answers the target and the interval, or `not set`
  for an interval the config does not state. With no `acme-monitor` section it
  answers an error whose fix sets the target.
- `show acme-monitor target <name>` answers the target when `<name>` is the
  configured one. Any other name is answered with an error whose fix is
  `show acme-monitor status`.

## Check it without a daemon

The conformance checker starts the program in declare mode and judges the
declaration it writes. Run it from the root of the Ze repository:

```
ze show plugin conformance run "python3 examples/plugin/python/acme_monitor.py"
```

No row should fail. The rows marked `not-judged` are the parts a daemon must
exercise (`docs/plugin-development/testing.md`, "Checking That a Program Is a
Valid Plugin").

To see the declaration itself:

```
ZE_PLUGIN_MODE=declare python3 examples/plugin/python/acme_monitor.py
```

It prints one line that starts with `#1 ze-plugin-engine:declare-registration`
and exits 0. The line lists the three modules under `schemas` and the two
commands under `commands`.

## Run it under Ze

Declare the plugin in the config. Give the full path of the file, because Ze
may start the program from another directory:

```
plugin {
    external acme-monitor {
        run "python3 /path/to/ze/examples/plugin/python/acme_monitor.py"
        encoder json
    }
}
```

Then write the plugin's own section:

```
acme-monitor {
    target web.example.net
    interval 3
}
```

Start Ze with that config. Ze runs the plugin first, joins its three modules,
and only then parses the config, so the `acme-monitor` section loads and the
two commands exist. Ze relays what the plugin writes to stderr into its own
log: each accepted config writes a line there that starts with
`acme-monitor: applied`.

## What an operator types

`ze cli -c` sends the line to the running Ze:

```
ze cli -c "show acme-monitor status"
ze cli -c "show acme-monitor target web.example.net"
```

Both answer the target `web.example.net` and the interval `3`.

Ze judges the argument against the leaf `name` of `acme-monitor-cmd` before it
asks the plugin. A name of 25 characters is refused, the plugin is never asked,
and the command exits 1:

```
$ ze cli -c "show acme-monitor target this-name-is-far-too-long"
error: show acme-monitor target name: "this-name-is-far-too-long" is outside the accepted length (25 characters)
name accepts 1..16 characters
fix: show acme-monitor target <1..16 characters>
```

A line with no name is refused before the plugin is asked:

```
error: required argument missing: name
This command needs name: The target name, from 1 to 16 characters.
fix: show acme-monitor target name <name>
```

A name within the bound reaches the plugin as the operator typed it: the
handler gets it as `args[0]`.

## What is not built

- `ze show acme-monitor ...` typed in the shell, with no `cli -c`, is not
  built: send the line to the running Ze with `ze cli -c`, or type it in a
  `ze cli` session.
- Ze asks a plugin only for the commands its Stage 1 declaration lists. It does
  not compare that list with the commands the plugin's `-cmd` module defines,
  so a command the module defines and the declaration leaves out is not routed
  to the plugin, and nothing says so at start.
- `ze config validate <file>` checks the `acme-monitor` section only while Ze
  runs with this plugin: it sends the file to the running Ze, which holds the
  plugin's modules. With no Ze running, Ze starts no plugin, so the section is
  refused as `unknown top-level keyword: acme-monitor`, and the message says
  why and gives the fix (`ze start`, then the same command). `ze config fmt`,
  `dump`, `diff` and `graph` behave the same way.
- `rollback` has not run against a live Ze: no test makes another plugin fail
  after this one applied.

## What the library leaves out

- No event subscription and no call to the engine after startup.
- No address family, capability, route filter, doctor check or metric.
- One request at a time, on one thread. A handler answers from what it holds
  and does not wait.
- A command answer is one document. It is not streamed row by row, so it must
  fit one wire message of 16 MiB.

## Writing your own plugin

Copy `ze_plugin.py` beside your program and start from `acme_monitor.py`.

- Write nothing to stdout. In declare mode stdout carries the declaration
  line only. The library sends `print()` to stderr once the
  `Plugin` exists, and `plugin.log(text)` writes there too.
- Name each module after your plugin, with the suffix that says what it adds:
  `-conf` for config, `-cmd` for commands. A module whose definitions Ze reads
  nowhere refuses the start, and so does a command path or a module name Ze already holds.
- The name you give `@plugin.command` is the path of the container that holds
  the command's `ze:command`. It starts with a verb such as `show`, `request`
  or `clear`, and holds lowercase words. The library refuses another name when
  the program loads.
- Raise `CommandError(message, fix=...)` when a command cannot answer, with the
  command that clears it.
- A class that handles a config section must define `apply`. Ze refuses every
  commit for a plugin that cannot apply it.
- Every config value arrives as a string. Use `new.int(key, default=...)` and
  `new.bool(key, default=...)` to convert one. A list arrives as one object
  keyed by its entry names.
- An empty `new` in `apply` means the operator removed the section.
- Your modules change only when Ze restarts. A plugin Ze starts again must
  declare the same modules, or Ze refuses it.

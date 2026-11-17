# A Ze plugin in Python

This directory holds an external Ze plugin written in Python, and the small
library it is built on. It shows that a plugin in any language can bring its
own YANG config module to Ze, be told its config, and serve a command.

| File | What it is |
|------|------------|
| `ze_plugin.py` | The library. One file, standard library only, Python 3.9 or later |
| `acme_monitor.py` | The example plugin, about 80 lines |

The wire contract is `docs/plugin-development/protocol.md`. The library speaks
plugin protocol version 1.

## What the example does

- It declares the YANG module `acme-monitor-conf`. The module adds the config
  section `acme-monitor`, with a mandatory `endpoint` and an `interval` from 1
  to 10 seconds that defaults to 5.
- `verify` refuses an `http://` endpoint, and the refusal names the fix.
- `apply` stores the values and logs them with the changed config paths.
- `rollback` puts the previous values back.
- The command `show acme-monitor status` answers one row with the stored
  values.

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
and exits 0.

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
    endpoint https://probe.example.net/health
    interval 3
}
```

Start Ze with that config, then ask the plugin for its state:

```
ze cli -c "show acme-monitor status"
```

Ze relays what the plugin writes to stderr into its own log. Each accepted
config writes a line there that starts with `acme-monitor: applied`.

## What works today

Against a Ze built from the branch that carries this example:

- Declare mode and the conformance checker.
- Registration: Ze starts the plugin, the plugin connects back over TLS,
  authenticates, and completes the five startup stages.
- The command `show acme-monitor status`.
- The plugin's own config section at start. Ze starts the plugin, takes its
  YANG module from the Stage 1 declaration, and only then parses the config,
  so the `acme-monitor` section loads. The plugin's `verify` and `apply` run,
  and the command answers the configured endpoint and interval.
- Ze enforces the module: an `interval` of 11 is refused when the config
  loads, with the accepted range.
- A config change in a running Ze. `verify` then `apply` run with the changed
  paths, and a `verify` refusal fails the reload with the plugin's message and
  fix while the plugin keeps its previous state.
- A `verify` refusal at start. Ze stays up, reports the plugin as stopped with
  the plugin's own message and fix, and the plugin's command is not there.
- In a running Ze, the schema surfaces hold the plugin's module: `show config
  running` answers the `acme-monitor` section, and typed in a `ze cli`
  session `show schema list` lists `acme-monitor-conf` with the root the
  plugin wants and `show yang tree config` shows the `acme-monitor` nodes.
  `ze show doctor` does not report the section as delivered to no plugin.
- Doctor codes for a schema Ze did not get: `doctor-plugin-schema-undeclared`
  when the plugin cannot be run or declares no module, and
  `doctor-plugin-schema-refused` when Ze refuses the module at start. Both are
  on the refusal Ze prints; `ze show plugin conformance run "<command>"` shows
  what the program declares.

## What does not work yet

- `ze config validate <file>` checks the `acme-monitor` section only while Ze
  runs with this plugin: it sends the file to the running Ze, which holds the
  plugin's module. With no Ze running, Ze starts no plugin, so the section is
  refused as `unknown top-level keyword: acme-monitor`, and the message says
  that the config declares external plugin `acme-monitor`, that Ze reads its
  schema when `ze start` runs it, why no running Ze answered, and the fix
  (`ze start`, then the same command). A config with the `plugin` block and no
  `acme-monitor` section validates either way.

- `rollback` has not run against a live Ze: no test yet makes another plugin
  fail after this one applied.

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
- A command name starts with a verb such as `show`, `request` or `clear`, and
  holds lowercase words. The library refuses another name when the program
  loads.
- A class that handles a config section must define `apply`. Ze refuses every
  commit for a plugin that cannot apply it.
- Every config value arrives as a string. Use `new.int(key, default=...)` and
  `new.bool(key, default=...)` to convert one.
- An empty `new` in `apply` means the operator removed the section.

# YANG Schema Authoring

Plugins declare their configuration schema using YANG (RFC 7950). This guide covers the basics for plugin authors.

## What works today

| Plugin kind | Its own YANG module | Config it receives |
|-------------|---------------------|--------------------|
| Internal plugin, compiled into `ze` (package added to `internal/component/plugin/all`) | Registered at build, parsed and validated; the operator writes config under its roots | Its `WantsConfig` roots |
| External plugin (a separate process, SDK or any language) | Joined to the schema at `ze start`, from the modules its Stage 1 declaration lists under `schemas` (config, command and rpc modules); the operator writes config under its roots. With no daemon running Ze starts no plugin, so `ze config validate` and the other commands that parse a config then cannot check a section under those roots | The roots it declares, its own included |

At `ze start` the daemon reads the plugin declarations first (the `plugin`
block and the plugins named on the command line), runs each declared external
plugin, reads its Stage 1 declaration, joins every module it lists and only
then parses the full config. The plugin waits for its Stage 1 answer in the
meantime, so a plugin MUST NOT time out between sending its declaration and
receiving the answer. A module that does not compile, or that clashes with a
module Ze holds, refuses the start and names the plugin. A plugin and its
modules change only when `ze` restarts.

Ze enforces that rule while it runs:

- A reload whose config adds or removes an external plugin, or changes the
  `run`, `encoder`, `respawn` or `timeout` of one, is refused whole. The
  message names each plugin and what changed, and its fix is `restart ze`.
  The rest of the config reloads without a restart, your own section
  included.
- When Ze starts your plugin again (it ended and declared `failure-policy
  restart`, or the operator ran `request plugin restart <name>`), the plugin
  MUST declare the same modules: each with the same name, namespace and text,
  none missing and none added, or none if it declared none. This holds for a
  command or rpc module as for a config module. Another schema refuses the
  plugin at Stage 1. Ze does not run it, keeps the config under its root, and
  raises the warning `doctor-plugin-schema-changed`, which `ze show doctor`
  reports with the first module that differs. To ship a new schema, install
  the new program and restart `ze`.
<!-- source: internal/component/plugin/server/stage1_schema.go -- refuseChangedSchema, schemaDifference -->

A plugin that cannot be run, or that sends no declaration within its stage
timeout, declares nothing: a config section only its module defines is then
refused as `unknown top-level keyword: <root>`, and the refusal names the
plugin, why it did not declare and the command that checks it
(`ze show plugin conformance run "<command>"`). The same holds for a plugin
that sends its declaration with no YANG module: it defines no config section,
and a section written for it is refused with a message that says so.

Each of these outcomes has a doctor code:

| Code | When | Where you read it |
|------|------|-------------------|
| `doctor-plugin-schema-undeclared` | Ze holds no config schema of your plugin: it could not be run, it sent no declaration in time, or it declared no module and the config holds a section no schema defines | On the refusal and in the log when the start is refused, and then in `ze doctor`, from the record Ze wrote before it exited, until Ze next starts cleanly. In `ze show doctor` when the config holds no section of your plugin, so Ze starts |
| `doctor-plugin-schema-refused` | Ze refused your module at start: it does not compile, it clashes with a module Ze holds, it redefines a top-level node, it names a `ze:validate` validator or a `ze:reference` source Ze does not hold, it declares a command or a wire method another module declares, a `ze:rpc` names an rpc no module defines, or it adds nothing | On the refusal and in the log. Ze does not start, and `ze doctor` reports the refusal from the record Ze wrote before it exited, until Ze next starts cleanly |
| `doctor-plugin-schema-changed` | Your plugin was started again with other YANG modules (a config, command or rpc module) | In `ze show doctor` |

In a running Ze your config module is part of the schema every surface reads: the
session editor parses and completes under your root, `show config running`
answers your section, `show yang tree config` shows your nodes with their
types and descriptions, and `show schema list` lists your module with the
roots your plugin wants. Type them in a `ze cli` session or send them over
SSH: both reach the running Ze, while `ze cli -c` answers these two in its own
process, from the modules compiled into `ze`. A running Ze has no command that
prints the YANG text of one module; `ze schema show <module>` prints it for a
module compiled into `ze` only. The doctor check for config that reaches no plugin counts the roots
your running plugin declared, so your section is not reported as having no
effect.

A change to your section that an operator commits in the session editor
reaches your plugin as verify then apply, and Ze saves it: the next `ze start`
on the same config directory runs your plugin, reads your modules again and
configures your plugin with the committed value.
<!-- source: test/ui/external-yang-session-save.ci -- a commit under an external root survives a restart -->

Your plugin's process reads the settings of the config's `environment` block
in its own environment, although Ze starts it before it has parsed the whole
config and applied that block to itself. A variable set in the environment Ze
was started from wins over the block, as it does for Ze. A plugin Ze starts
again later inherits Ze's environment, which holds the block by then.
<!-- source: internal/component/config/apply_env.go -- EnvironmentEntries -->
<!-- source: internal/component/plugin/process/process.go -- SetConfigEnvironment -->
<!-- source: internal/core/env/env.go -- WithDefaults -->

With no daemon, `ze schema` shows the modules compiled into `ze` only. A
`--plugin` that names your program does not start it: the answer is followed
by a warning that your module is not available with no running Ze, whose fix
is `ze cli`: typed in that session, `show schema list` lists your module and
`show yang tree config` shows its nodes.

With no daemon running, Ze never starts an external plugin's program. Your
module reaches Ze only when `ze start` runs your plugin, so a command that
parses a config with no daemon holds the schemas compiled into the `ze` binary
and nothing else. It accepts a config that declares your plugin and holds no
section under your roots. It refuses a section under your roots as
`unknown top-level keyword: <root>`, and the message says why and how to check
the section.

`ze config validate <file>` asks the running Ze first when the config declares
an external plugin: it sends the file's content, so the file may be anywhere,
and the running Ze, which read your module when it started your plugin, judges
the section in full. The output and the exit codes are those of a local
verdict. With no Ze running it answers:

```
config ze.conf does not parse: it holds a name that no config schema compiled into ze defines
the config declares external plugin "acme-monitor"; the config schema of an external plugin is not compiled into ze: ze reads it from the plugin when `ze start` runs it. This command starts no plugin, so only a running ze that started with these plugins checks a section that such a plugin defines, and none answered: no ze listens at 127.0.0.1:2222 (...). Run the command below, then `ze config validate ze.conf` again. What the parser refused: line 8: unknown top-level keyword: acme-monitor
fix: ze start
```

`ze config fmt`, `dump`, `diff` and `graph`, and their `ze show config`
spellings, ask the running Ze the same way: the answer is printed as a local
answer is, `fmt -w` rewrites your file with the text the running Ze formatted,
and with no Ze running the message above names the command you ran in place of
`ze config validate`. `ze config diff` compares the resolved `bgp` block and
the order of ordered lists, so a leaf that differs in your plugin's own section
is not reported as a difference.

The other commands ask no running Ze. Their message says so, and its fix is
`ze config validate <file>`.

| Command | A section under an external plugin's root |
|---------|-------------------------------------------|
| `ze config validate` | Judged by the running Ze; with none running, refused with the message above |
| `ze config fmt`, `dump`, `diff`, `graph` | Answered by the running Ze; with none running, refused with the message above, naming the command to run again |
| `ze config migrate`, `completion`, with no daemon | Refused, naming `ze config validate <file>` as the fix |
| `ze config edit` | The file under edit is refused when it is opened |
| `ze doctor`, the support report, `show crashes` readiness, `ze tacacs` probe, `show plugin declarations config`, `ze plugin test`, with no daemon | The config they read is refused, naming `ze config validate <file>` as the fix |

The message names the plugins the config declares. It does not say that one of
them owns the refused name: Ze holds no schema that could tell a section of
your plugin from a mistyped keyword.

`show config validation <file>` typed in a `ze cli` session, or sent over SSH,
the web, MCP, REST or gRPC, reaches the running Ze too, but the daemon reads
the file on its own host and only from its config directories. A caller that
holds the config elsewhere sends its bytes: `show config supplied validation
content <base64>`, which is what `ze config validate` does. `ze cli -c "show
config validation <file>"` answers in its own process and asks no daemon.

Inside a running daemon the same commands start no program either. The daemon
joined the modules of its plugins when it started, and a command it serves (a
web upload, an editor commit, a served `show`) is answered from that set, so a
section under your roots is checked against your module there, ranges
included. A config that declares a plugin the daemon did not start with is
refused there with the restart of `ze` as the fix.

Declare mode (`ZE_PLUGIN_MODE=declare`) is still worth answering.
`ze show plugin conformance run "<command>"` starts the program that way with
no daemon, because running the program you name is what it is for. No other
command with no daemon starts it: `ze show plugin declarations config <file>`
lists your plugin with the state `not-available` and names that checker. A
running Ze starts your program in declare mode when `show plugin declarations
config <file>` is typed in a session it serves.
<!-- source: internal/component/config/declared_schemas.go -- SchemaWithDeclaredPlugins, PublishDeclaredSchemas, DeclaredSchemas.ParseRefusal -->
<!-- source: internal/component/command/local_data.go -- ServeLocal -->
<!-- source: internal/component/plugin/declarations.go -- QueryDeclaration -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation -->
<!-- source: cmd/ze/hub/declared_plugins.go -- joinDeclaredPlugins, undeclaredPluginLines -->
<!-- source: internal/component/plugin/server/early_stage1.go -- HoldDeclaredPlugins -->
<!-- source: internal/component/config/first_pass.go -- FirstPass -->
<!-- source: internal/component/config/plugin_verify.go -- BuildPluginConfigSections -->

## Registering a Schema

Schemas are declared via the `Schemas` field of `sdk.Registration`, which is passed to
`p.Run()`. It lists one `sdk.SchemaDecl` for each module the plugin brings: a
module whose name ends in `-conf` adds config roots, one that augments another
module extends it, one whose name ends in `-cmd` adds the commands its
`ze:command` nodes declare, and the rpc statements of a module (an `-api`
module, as compiled-in plugins pair them) supply the parameter metadata for a
command without typed CLI leaves. A command path with typed CLI leaves publishes
those names and types instead, including for each alias. Its `ze:rpc` still
names the RPC contract. Ze judges each module by the rules the
modules compiled into `ze` obey; it also refuses a command path or a wire
method that another module already declares, and a `ze:rpc` that names an rpc
no module defines. A plugin that answers a command also declares it in
`Commands`, and Ze refuses the start when a `ze:command` node of its modules
is reached by no command it declares there. Defining a command in a `-cmd` module, with a worked example and
the refusals, is [commands.md](commands.md), "Defining a Command in YANG".
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, SchemaDecl -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, SchemaDecl -->

```go
p := sdk.NewWithConn("my-plugin", conn)

p.Run(ctx, &sdk.Registration{
    Schemas: []sdk.SchemaDecl{{
        Module:    "ze-my-plugin-conf",
        Namespace: "urn:ze:my-plugin",
        YANGText:  myPluginYANG,        // YANG module text (string)
        Handlers:  []string{"my-plugin"},
    }},
    WantsConfig: []string{"my-plugin"},
})
```
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

### SchemaDecl Fields

| Field | Type | Purpose |
|-------|------|---------|
| `Module` | `string` | YANG module name, as the text states it |
| `Namespace` | `string` | YANG namespace URI |
| `YANGText` | `string` | Full YANG module text |
| `Handlers` | `[]string` | Config path prefixes; read only by the subsystem handler |

<!-- source: pkg/plugin/rpc/types.go -- SchemaDecl -->

### Embedding YANG Files

Internal plugins use `//go:embed` to embed YANG files at compile time:
<!-- source: internal/component/bgp/plugins/rib/yang/embed.go -- ZeRibYANG (go:embed) -->
<!-- source: internal/component/bgp/plugins/rib/register.go -- YANG field set to ribschema.ZeRibYANG -->

```go
import _ "embed"

//go:embed my-plugin.yang
var myPluginYANG string
```

The embedded string is then passed as `YANGText` in the `SchemaDecl`.

For external plugins, the YANG text can come from any source (file read, constant, etc.).

## Minimal Schema

```yang
module my-plugin {
    namespace "urn:my-company:my-plugin";
    prefix mp;

    container my-plugin {
        leaf enabled {
            type boolean;
            default true;
        }
    }
}
```

## Module Structure

```yang
module name {
    namespace "urn:...";  // Unique namespace URI
    prefix prefix;        // Short prefix for references

    // Content: containers, lists, leafs
}
```

## Common Types

| YANG Type | Go Type | Example |
|-----------|---------|---------|
| `string` | `string` | `"hello"` |
| `boolean` | `bool` | `true`/`false` |
| `uint8/16/32/64` | `uintN` | `42` |
| `int8/16/32/64` | `intN` | `-42` |

## Constraints

### Range (Numbers)

```yang
leaf port {
    type uint16 {
        range "1..65535";
    }
}

leaf hold-time {
    type uint16 {
        range "0 | 3..65535";  // 0 or 3+
    }
}
```

### Length (Strings)

```yang
leaf name {
    type string {
        length "1..64";
    }
}
```

### Pattern (Regex)

```yang
leaf ipv4-address {
    type string {
        pattern '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+';
    }
}
```

### Enumeration

```yang
leaf action {
    type enumeration {
        enum allow;
        enum deny;
        enum log;
    }
}
```

## Containers and Lists

### Container (Single Instance)

```yang
container settings {
    leaf timeout { type uint32; }
    leaf retries { type uint8; }
}
```

Config:
```
settings {
    timeout 30;
    retries 3;
}
```

### List (Multiple Instances)

```yang
list endpoint {
    key "name";

    leaf name { type string; }
    leaf url { type string; }
    leaf enabled { type boolean; default true; }
}
```

Config:
```
endpoint api {
    url "https://api.example.com";
}
endpoint backup {
    url "https://backup.example.com";
    enabled false;
}
```

## Mandatory and Default

```yang
leaf required-field {
    type string;
    mandatory true;  // Must be provided
}

leaf optional-field {
    type uint32;
    default 100;     // Used if not provided
}
```

## References

Ze checks the three YANG reference types. A `leafref` value must have the
type of the leaf its path names and, unless the type says
`require-instance false`, equal an instance of that leaf somewhere in the
config, or its default where the default is in use. An `identityref` value
must name an identity derived from the base, unprefixed or with the defining
module's prefix. An `instance-identifier` value is a path with every node
prefixed by its module's prefix, and must name a node the config holds. The
loader refuses a leafref whose path names no leaf, a config leafref pointing
at state, and a loop of leafrefs (`ErrLeafrefPath`).

```yang
// In your module: a reference to a class of the same qdisc
leaf default-class {
    type leafref { path "../class/name"; }
}
```

A reference no YANG path can express, such as one into state a plugin holds,
stays a string checked by a `ze:validate` function.
<!-- source: internal/component/config/yang/loader_reference.go -- checkReferences -->
<!-- source: internal/component/config/yang/validator_reference.go -- validateLeafref, validateIdentityref, validateInstanceIdentifier -->
<!-- source: internal/component/config/yang/validator_instance.go -- ValidateReferences -->

## Extensions that name a registration

Four `ze:` extensions name something registered inside Ze rather than
something your module defines. A name is held only when the component that
registers it is compiled into the `ze` you run. When `ze start` joins your
module, Ze checks the names of two of them:

| Extension | What it names | A name Ze does not hold |
|-----------|---------------|-------------------------|
| `ze:validate` | A validator function | Ze refuses the start, naming your plugin, your module, the extension, the name and the schema path, with the code `doctor-plugin-schema-refused` |
| `ze:reference` | A source of names that a component registered | The same refusal |
| `ze:decorate` | A display decorator of the web UI | Ze accepts the module. The web UI shows the value with no decoration and no message |
| `ze:backend` | The backends that support a node | Ze accepts the module. At commit, the components of `interface`, `firewall` and `traffic control` compare the list with the backend the config selects, and a name no backend answers to never matches: a node under those roots that lists only such names is refused whenever a config uses it, and completion does not offer it. Under any other root Ze does not read the extension |

To check a value with logic Ze does not hold, check it in the config verify
step of your plugin: the refusal names that fix.

For `ze:validate` and `ze:reference` the rule is the same for a module
compiled into `ze`: a name that `ze` does not hold refuses the start. Your
module's refusal names your plugin. A compiled-in module's refusal says that
this `ze` was built wrong and that the config is not at fault. It names the
module, the extension, the name and the schema path, with the code
`doctor-build-registration-unheld`. A test over the shipped module set keeps
such a build from shipping.
<!-- source: internal/component/config/yang/join.go -- refuseUnheldRegistrations, unheldRegistration, unheldRegistrationRefusal -->
<!-- source: internal/component/config/yang/registration_held.go -- unheldName, RefuseUnheldCompiledRegistrations -->
<!-- source: cmd/ze/hub/build_registrations.go -- refuseWronglyBuilt -->
<!-- source: internal/component/web/decorator.go -- DecoratorRegistry.Get -->
<!-- source: internal/component/config/backend_gate.go -- validateBackendFeatures -->
<!-- source: internal/component/cli/completer.go -- backendAllowed -->

## Best Practices

1. **Use meaningful namespaces** -- include your organization
2. **Add both help texts** -- a `ze:help` one-line summary for the completion row, and a `description` paragraph for the `?` box
3. **Set sensible defaults** -- reduce required config
4. **Validate early** -- use YANG constraints, not just code
5. **Use the `Handlers` field** -- list the config path prefixes your plugin manages

## Example: Complete Plugin with Schema

This plugin runs as an external process and defines its own root,
`acme-monitor`. Ze joins its module when `ze start` runs the plugin; a command
that parses the config with no daemon starts no plugin and cannot check the
section (see "What works today" above). `sdk.RunOrDeclare` is the entry that
also answers declare mode, which the conformance check uses.

A complete example showing YANG schema registration with the SDK. The plugin
reads leaves of its root, so it registers a config transaction: Stage 2
installs the parsed config, and a reload verifies, applies and, when the commit
fails, rolls back the same value. A plugin that declares `WantsConfig` and
registers only `OnConfigure` refuses every reload that changes its root (see
`handlers.md`, "A plugin with no apply handler").

```go
package main

import (
    "context"
    _ "embed"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/signal"
    "sync"

    "github.com/ze-software/ze/pkg/plugin/sdk"
)

//go:embed acme-monitor.yang
var acmeMonitorYANG string

// monitorConfig is the part of the acme-monitor container the plugin acts on.
type monitorConfig struct {
    Endpoint string      `json:"endpoint"`
    Interval json.Number `json:"interval"`
}

// parseMonitor reads the plugin's config from one delivery. It changes no
// state, because a refused verify and a CLI validate run it too.
func parseMonitor(sections []sdk.ConfigSection) (monitorConfig, bool, error) {
    for _, s := range sections {
        if s.Root != "acme-monitor" {
            continue
        }
        var data struct {
            Monitor monitorConfig `json:"acme-monitor"`
        }
        if err := json.Unmarshal([]byte(s.Data), &data); err != nil {
            return monitorConfig{}, true, fmt.Errorf("acme-monitor: %w", err)
        }
        return data.Monitor, true, nil
    }
    return monitorConfig{}, false, nil
}

// monitor holds the running config the poller reads.
type monitor struct {
    mu  sync.Mutex
    cfg monitorConfig
}

// install makes cfg the running config. A failed install must leave the
// running config as it was.
func (m *monitor) install(cfg monitorConfig) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.cfg = cfg
    log.Printf("monitoring %s every %ss", cfg.Endpoint, cfg.Interval)
    return nil
}

func main() {
    p, err := sdk.NewFromEnv("acme-monitor")
    if err != nil {
        log.Fatal(err)
    }

    m := &monitor{}
    sdk.NewConfigTx(monitorConfig{}, parseMonitor, m.install).Register(p)

    p.OnEvent(func(event string) error {
        log.Printf("event: %s", event)
        return nil
    })

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    if err := p.Run(ctx, &sdk.Registration{
        Schemas: []sdk.SchemaDecl{{
            Module:    "acme-monitor-conf",
            Namespace: "urn:acme:monitor",
            YANGText:  acmeMonitorYANG,
            Handlers:  []string{"acme-monitor"},
        }},
        WantsConfig: []string{"acme-monitor"},
    }); err != nil {
        log.Fatal(err)
    }
}
```
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromEnv, Run -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent -->
<!-- source: pkg/plugin/sdk/config_tx.go -- NewConfigTx, ConfigTx.Register, ConfigParser, ConfigInstaller -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, SchemaDecl, ConfigSection -->

### Example YANG Schema

```yang
module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    import ze-extensions { prefix ze; }

    description "ACME endpoint monitoring plugin";

    container acme-monitor {
        ze:help "Monitor configuration.";
        description "Every endpoint the plugin polls, and where it sends an alert.";

        leaf endpoint {
            type string;
            mandatory true;
            ze:help "HTTPS endpoint to monitor.";
            description "The plugin issues one GET to this URL at every interval.";
        }

        leaf interval {
            type uint32 {
                range "10..3600";
            }
            default 60;
            ze:help "Check interval in seconds.";
            description "The pause between two polls of the endpoint, from 10 seconds to one hour.";
        }

        list alert {
            key "name";
            ze:help "Alert destinations.";
            description "One entry for each address that receives a failure notice.";

            leaf name {
                type string {
                    length "1..64";
                }
            }

            leaf email {
                type string;
            }
        }
    }
}
```

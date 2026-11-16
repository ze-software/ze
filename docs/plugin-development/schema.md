# YANG Schema Authoring

Plugins declare their configuration schema using YANG (RFC 7950). This guide covers the basics for plugin authors.

## What works today

| Plugin kind | Its own YANG module | Config it receives |
|-------------|---------------------|--------------------|
| Internal plugin, compiled into `ze` (package added to `internal/component/plugin/all`) | Registered at build, parsed and validated; the operator writes config under its roots | Its `WantsConfig` roots |
| External plugin (a separate process, SDK or any language) | Joined to the schema at `ze start`, from the `YANGText` of its Stage 1 declaration; the operator writes config under its roots. With no daemon running Ze starts no plugin, so `ze config validate` and the other commands that parse a config then cannot check a section under those roots | The roots it declares, its own included |

At `ze start` the daemon reads the plugin declarations first (the `plugin`
block and the plugins named on the command line), runs each declared external
plugin, reads its Stage 1 declaration, joins the module it carries and only
then parses the full config. The plugin waits for its Stage 1 answer in the
meantime, so a plugin MUST NOT time out between sending its declaration and
receiving the answer. A module that does not compile, or that clashes with a
module Ze holds, refuses the start and names the plugin. A plugin and its
module change only when `ze` restarts.

A plugin that cannot be run, or that sends no declaration within its stage
timeout, declares nothing: a config section only its module defines is then
refused as `unknown top-level keyword: <root>`, and the refusal names the
plugin, why it did not declare and the command that checks it
(`ze show plugin conformance run "<command>"`).

With no daemon running, Ze never starts an external plugin's program. Your
module reaches Ze only when `ze start` runs your plugin, so a command that
parses a config with no daemon holds the schemas compiled into the `ze` binary
and nothing else. It accepts a config that declares your plugin and holds no
section under your roots. It refuses a section under your roots as
`unknown top-level keyword: <root>`, and the message says why and how to check
the section:

```
config ze.conf does not parse: it holds a name that no config schema compiled into ze defines
the config declares external plugin "acme-monitor"; the config schema of an external plugin is not compiled into ze: ze reads it from the plugin when `ze start` runs it. This command does not ask a running ze and starts no plugin, so it cannot check a section that such a plugin defines. A running ze that started with these plugins can: type `show config validation ze.conf` in the session the command below opens. What the parser refused: line 8: unknown top-level keyword: acme-monitor
fix: ze cli
```

| Command, with no daemon | A section under an external plugin's root |
|-------------------------|-------------------------------------------|
| `ze config validate`, `fmt`, `dump`, `diff`, `migrate`, `completion`, `show config graph` | Refused with the message above |
| `ze config edit` | The file under edit is refused when it is opened |
| `ze doctor`, the support report, `show crashes` readiness, `ze tacacs` probe, `show plugin declarations config`, `ze plugin test` | The config they read is refused with the message above |

The message names the plugins the config declares. It does not say that one of
them owns the refused name: Ze holds no schema that could tell a section of
your plugin from a mistyped keyword.

`ze cli -c "show config validation <file>"` is not the daemon form: `ze cli -c`
answers that command in its own process, with the same refusal. An interactive
`ze cli` session, SSH, the web, MCP, REST and gRPC reach the running Ze.

Inside a running daemon the same commands start no program either. The daemon
joined the modules of its plugins when it started, and a command it serves (a
web upload, an editor commit, a served `show`) is answered from that set, so a
section under your roots is checked against your module there, ranges
included. A config that declares a plugin the daemon did not start with is
refused there with the restart of `ze` as the fix.

Declare mode (`ZE_PLUGIN_MODE=declare`) is still worth answering:
`ze show plugin conformance run "<command>"` and `show plugin declarations`
start the program that way, because running the program you name is what they
are for. No other command with no daemon starts it.
<!-- source: internal/component/config/declared_schemas.go -- SchemaWithDeclaredPlugins, PublishDeclaredSchemas, DeclaredSchemas.ParseRefusal -->
<!-- source: internal/component/command/local_data.go -- ServeLocal -->
<!-- source: internal/component/plugin/declarations.go -- QueryDeclaration -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation -->
<!-- source: cmd/ze/hub/declared_plugins.go -- joinDeclaredPlugins, undeclaredPluginLines -->
<!-- source: internal/component/plugin/server/early_stage1.go -- HoldDeclaredPlugins -->
<!-- source: internal/component/config/first_pass.go -- FirstPass -->
<!-- source: internal/component/config/plugin_verify.go -- BuildPluginConfigSections -->

## Registering a Schema

Schemas are declared via the `Schema` field of `sdk.Registration`, which is passed to
`p.Run()`. The `Schema` field is a pointer to `sdk.SchemaDecl`.
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, SchemaDecl -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, SchemaDecl -->

```go
p := sdk.NewWithConn("my-plugin", conn)

p.Run(ctx, &sdk.Registration{
    Schema: &sdk.SchemaDecl{
        Module:    "ze-my-plugin",
        Namespace: "urn:ze:my-plugin",
        YANGText:  myPluginYANG,        // YANG module text (string)
        Handlers:  []string{"my-plugin"},
    },
    WantsConfig: []string{"my-plugin"},
})
```
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

### SchemaDecl Fields

| Field | Type | Purpose |
|-------|------|---------|
| `Module` | `string` | YANG module name |
| `Namespace` | `string` | YANG namespace URI |
| `YANGText` | `string` | Full YANG module text |
| `Handlers` | `[]string` | Config path prefixes this plugin handles |

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
        Schema: &sdk.SchemaDecl{
            Module:    "acme-monitor",
            Namespace: "urn:acme:monitor",
            YANGText:  acmeMonitorYANG,
            Handlers:  []string{"acme-monitor"},
        },
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
module acme-monitor {
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

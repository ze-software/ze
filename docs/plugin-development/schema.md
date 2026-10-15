# YANG Schema Authoring

Plugins declare their configuration schema using YANG (RFC 7950). This guide covers the basics for plugin authors.

## Registering a Schema

Schemas are declared via the `Schema` field of `sdk.Registration`, which is passed to
`p.Run()`. The `Schema` field is a pointer to `sdk.SchemaDecl`.
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, SchemaDecl -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, SchemaDecl -->

```go
p := sdk.NewWithConn("my-plugin", conn)

p.Run(ctx, sdk.Registration{
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

## References (leafref)

Reference values from other parts of config:

```yang
// In ze-bgp-conf module
list peer-group {
    key "name";
    leaf name { type string; }
}

// In your module - reference peer-group
leaf group {
    type leafref {
        path "/bgp/peer-group/name";
    }
}
```

Ze validates that referenced values exist.

## Best Practices

1. **Use meaningful namespaces** -- include your organization
2. **Add both help texts** -- a `ze:help` one-line summary for the completion row, and a `description` paragraph for the `?` box
3. **Set sensible defaults** -- reduce required config
4. **Validate early** -- use YANG constraints, not just code
5. **Use the `Handlers` field** -- list the config path prefixes your plugin manages

## Example: Complete Plugin with Schema

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

    if err := p.Run(ctx, sdk.Registration{
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

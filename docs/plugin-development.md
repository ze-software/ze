# Plugin Development

Ze plugins receive events, add configuration and commands, validate or apply configuration changes, implement address families and capabilities, and call the shared command dispatcher. External and in-process plugins use the same five-stage registration model.

## Start here

The public Go SDK is `pkg/plugin/sdk`. An external plugin normally:

1. Creates an SDK client from the environment supplied by Ze.
2. Registers callbacks.
3. Declares subscriptions, commands, schemas, families, or capabilities.
4. Runs the five-stage startup protocol.
5. Processes runtime events and commands until shutdown.

```go
package main

import (
    "context"
    "log"

    "github.com/ze-software/ze/pkg/plugin/sdk"
)

func main() {
    plugin, err := sdk.NewFromEnv("example")
    if err != nil {
        log.Fatal(err)
    }
    defer plugin.Close()

    plugin.OnEvent(func(event string) error {
        log.Print(event)
        return nil
    })
    plugin.SetStartupSubscriptions([]string{"update", "state"}, nil, "")

    if err := plugin.Run(context.Background(), sdk.Registration{}); err != nil {
        log.Fatal(err)
    }
}
```

Ze starts an external plugin with a per-plugin token in `ZE_PLUGIN_HUB_TOKEN` and the certificate authority root that issued the engine certificate in `ZE_PLUGIN_CA_PEM`. `sdk.NewFromEnv` consumes these values and connects to the engine.

## Guide set

| Guide | Purpose |
|-------|---------|
| [Protocol](plugin-development/protocol.md) | Five-stage registration, runtime framing, requests, responses, and shutdown |
| [Schemas](plugin-development/schema.md) | Register YANG configuration and constraints |
| [Handlers](plugin-development/handlers.md) | Configure, verify, apply, event, and codec callbacks |
| [Commands](plugin-development/commands.md) | Declare commands and return structured results |
| [Testing](plugin-development/testing.md) | SDK harness, functional scenarios, and failure cases |
| [Metrics](plugin-development/metrics.md) | Register and expose plugin metrics without a second telemetry path |

The [plugin operator guide](guide/plugins.md) covers loading, dependencies, process bindings, subscriptions, filters, and the built-in catalogue.

## Security model

- Each external process receives a token bound to its configured plugin name.
- The SDK validates the engine certificate chain against the supplied authority root, and refuses to dial without one.
- Configuration verification must fail closed on invalid candidate state.
- Commands remain subject to the shared dispatcher and authorisation path.
- Plugin output must be structured and bounded because it crosses process and operator boundaries.

## Development workflow

1. Implement one registration or callback surface.
2. Test the callback with the SDK harness.
3. Add a functional scenario through the normal plugin process manager.
4. Verify malformed requests and unavailable dependencies.
5. Run the plugin against a minimal Ze configuration.
6. Check command discovery, YANG registration, events, shutdown, and reload.

Do not invent a private configuration parser, command transport, or metrics server. Register with the shared engine so the same surface remains available through CLI, web, REST, gRPC, and MCP.

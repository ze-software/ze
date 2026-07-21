# Testing Your Plugin

This guide covers testing strategies for ze plugins using the SDK (`pkg/plugin/sdk`).

## Unit Testing with `net.Pipe`

The SDK is designed for testability. Use `net.Pipe()` to create a connected pair of
connections, then use `sdk.NewWithConn()` on one end and simulate the engine on the other.
<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn -->

```go
import (
    "net"
    "testing"

    "codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
    "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func newTestPair(t *testing.T) (*sdk.Plugin, *rpc.MuxConn) {
    t.Helper()

    pluginEnd, engineEnd := net.Pipe()
    t.Cleanup(func() {
        pluginEnd.Close()
        engineEnd.Close()
    })

    p := sdk.NewWithConn("test-plugin", pluginEnd)

    engineConn := rpc.NewConn(engineEnd, engineEnd)
    engineMux := rpc.NewMuxConn(engineConn)
    t.Cleanup(func() { engineMux.Close() })

    return p, engineMux
}
```
<!-- source: pkg/plugin/sdk/sdk_test.go -- newTestPair -->

The engine side uses `rpc.MuxConn` for bidirectional RPCs: it reads plugin requests
via `Requests()` and sends engine callbacks via `CallRPC`.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn, NewMuxConn, Requests, CallRPC -->
<!-- source: pkg/plugin/rpc/conn.go -- NewConn, Conn -->

## Testing Event Handlers

Register callbacks with `OnEvent`, `OnConfigure`, or `OnExecuteCommand`, then
run the plugin in a goroutine and simulate engine messages from the other end of the pipe.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent, OnConfigure, OnExecuteCommand -->

```go
func TestEventHandler(t *testing.T) {
    p, engineMux := newTestPair(t)

    eventReceived := make(chan string, 1)
    p.OnEvent(func(event string) error {
        eventReceived <- event
        return nil
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    errCh := make(chan error, 1)
    go func() {
        errCh <- p.Run(ctx, sdk.Registration{})
    }()

    // Complete the 5-stage startup from the engine side
    completeStartup(t, ctx, engineMux)

    // Deliver an event
    eventInput := struct {
        Event string `json:"event"`
    }{Event: `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`}

    _, err := engineMux.CallRPC(ctx, "ze-plugin-callback:deliver-event", eventInput)
    require.NoError(t, err)

    select {
    case got := <-eventReceived:
        assert.Contains(t, got, "10.0.0.1")
    case <-time.After(time.Second):
        t.Fatal("event callback not called")
    }

    // Shutdown
    byeInput := struct {
        Reason string `json:"reason"`
    }{Reason: "test-done"}
    _, _ = engineMux.CallRPC(ctx, "ze-plugin-callback:bye", byeInput)

    require.NoError(t, <-errCh)
}
```
<!-- source: pkg/plugin/sdk/sdk_test.go -- TestSDKEventDelivery -->

## Simulating the 5-Stage Startup

The `Run()` method performs the 5-stage startup protocol before entering the event
loop. Tests must simulate all five stages from the engine side.
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

```go
func completeStartup(t *testing.T, ctx context.Context, engineMux *rpc.MuxConn) {
    t.Helper()

    // Stage 1: read declare-registration, respond OK
    req := <-engineMux.Requests()
    assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
    require.NoError(t, engineMux.SendOK(ctx, req.ID))

    // Stage 2: send configure
    configInput := struct {
        Sections []sdk.ConfigSection `json:"sections"`
    }{}
    _, err := engineMux.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
    require.NoError(t, err)

    // Stage 3: read declare-capabilities, respond OK
    req = <-engineMux.Requests()
    assert.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method)
    require.NoError(t, engineMux.SendOK(ctx, req.ID))

    // Stage 4: send share-registry
    registryInput := struct {
        Commands []sdk.RegistryCommand `json:"commands"`
    }{}
    _, err = engineMux.CallRPC(ctx, "ze-plugin-callback:share-registry", registryInput)
    require.NoError(t, err)

    // Stage 5: read ready, respond OK
    req = <-engineMux.Requests()
    assert.Equal(t, "ze-plugin-engine:ready", req.Method)
    require.NoError(t, engineMux.SendOK(ctx, req.ID))
}
```
<!-- source: pkg/plugin/sdk/sdk_test.go -- completeStartup -->

The five stages are:

| Stage | Direction | RPC Method |
|-------|-----------|------------|
| 1 | Plugin to Engine | `ze-plugin-engine:declare-registration` |
| 2 | Engine to Plugin | `ze-plugin-callback:configure` |
| 3 | Plugin to Engine | `ze-plugin-engine:declare-capabilities` |
| 4 | Engine to Plugin | `ze-plugin-callback:share-registry` |
| 5 | Plugin to Engine | `ze-plugin-engine:ready` |

<!-- source: pkg/plugin/sdk/sdk.go -- Run (stages 1-5) -->

## Testing Command Execution

Plugins register command handlers with `OnExecuteCommand`. The handler receives
a serial, command name, args, and peer address, and returns status, data, and error.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->

```go
func TestCommandHandler(t *testing.T) {
    p, engineMux := newTestPair(t)

    p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
        if command == "show-status" {
            return "done", `{"status":"healthy"}`, nil
        }
        return "error", "unknown command", nil
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    errCh := make(chan error, 1)
    go func() {
        errCh <- p.Run(ctx, sdk.Registration{
            Commands: []sdk.CommandDecl{
                {Name: "show-status", Description: "Show health status"},
            },
        })
    }()

    completeStartup(t, ctx, engineMux)

    // Send execute-command
    cmdInput := struct {
        Serial  string   `json:"serial"`
        Command string   `json:"command"`
        Args    []string `json:"args,omitempty"`
        Peer    string   `json:"peer,omitempty"`
    }{Serial: "1", Command: "show-status"}

    result, err := engineMux.CallRPC(ctx, "ze-plugin-callback:execute-command", cmdInput)
    require.NoError(t, err)

    var out sdk.ExecuteCommandOutput
    require.NoError(t, json.Unmarshal(result, &out))
    assert.Equal(t, "done", out.Status)
    assert.Contains(t, out.Data, "healthy")
}
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

## Testing Configuration Handling

The `OnConfigure` callback receives config sections during Stage 2.
The `OnConfigVerify` and `OnConfigApply` callbacks handle config reload.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigure, OnConfigVerify, OnConfigApply -->

```go
func TestConfigHandler(t *testing.T) {
    p, engineMux := newTestPair(t)

    var receivedConfig []sdk.ConfigSection
    p.OnConfigure(func(sections []sdk.ConfigSection) error {
        receivedConfig = sections
        return nil
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    errCh := make(chan error, 1)
    go func() {
        errCh <- p.Run(ctx, sdk.Registration{
            WantsConfig: []string{"bgp"},
        })
    }()

    // Stage 1
    req := <-engineMux.Requests()
    require.NoError(t, engineMux.SendOK(ctx, req.ID))

    // Stage 2: send config
    configInput := struct {
        Sections []sdk.ConfigSection `json:"sections"`
    }{
        Sections: []sdk.ConfigSection{
            {Root: "bgp", Data: `{"router-id":"1.2.3.4"}`},
        },
    }
    _, err := engineMux.CallRPC(ctx, "ze-plugin-callback:configure", configInput)
    require.NoError(t, err)

    assert.Equal(t, 1, len(receivedConfig))
    assert.Equal(t, "bgp", receivedConfig[0].Root)
}
```
<!-- source: pkg/plugin/rpc/types.go -- ConfigSection -->

## Table-Driven Tests

Test multiple scenarios efficiently using Go table-driven test patterns:

```go
func TestCommandDispatch(t *testing.T) {
    tests := []struct {
        name       string
        command    string
        wantStatus string
        wantData   string
    }{
        {"known command", "show-routes", "done", `{"count":42}`},
        {"unknown command", "invalid", "error", "unknown command"},
        {"empty args", "show-routes", "done", `{"count":42}`},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p, engineMux := newTestPair(t)

            p.OnExecuteCommand(func(serial, cmd string, args []string, peer string) (string, string, error) {
                if cmd == "show-routes" {
                    return "done", `{"count":42}`, nil
                }
                return "error", "unknown command", nil
            })

            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            go func() { _ = p.Run(ctx, sdk.Registration{}) }()
            completeStartup(t, ctx, engineMux)

            cmdInput := struct {
                Serial  string `json:"serial"`
                Command string `json:"command"`
            }{Serial: "1", Command: tt.command}

            result, err := engineMux.CallRPC(ctx, "ze-plugin-callback:execute-command", cmdInput)
            require.NoError(t, err)

            var out sdk.ExecuteCommandOutput
            require.NoError(t, json.Unmarshal(result, &out))
            assert.Equal(t, tt.wantStatus, out.Status)
            assert.Equal(t, tt.wantData, out.Data)
        })
    }
}
```
<!-- source: pkg/plugin/sdk/sdk_types.go -- ExecuteCommandOutput -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn, CallRPC -->

## CI Integration

```yaml
# .github/workflows/test.yml (or Forgejo/Woodpecker equivalent)
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Test
        run: go test -race -v ./...

      - name: Build plugin
        run: go build -o my-plugin
```

## Coverage

Run with coverage:

```bash
go test -race -cover -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Target at least 80% coverage on handlers.

## Debugging

Enable debug logging with ze's hierarchical log system:
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromEnv -->

```bash
# Set log level for your plugin
export ZE_LOG_LEVEL=debug
./my-plugin
```

Or add structured logging in your plugin:

```go
import "log/slog"

p.OnEvent(func(event string) error {
    slog.Debug("event received", "event", event)
    // ...
    return nil
})
```

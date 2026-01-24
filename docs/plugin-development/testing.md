# Testing Your Plugin

This guide covers testing strategies for ZeBGP plugins.

## Unit Testing Handlers

Test handlers in isolation:

```go
func TestVerifyHandler_Valid(t *testing.T) {
    ctx := &plugin.VerifyContext{
        Action: "create",
        Path:   "my-plugin.item",
        Data:   `{"endpoint":"https://example.com"}`,
    }

    err := myVerifyHandler(ctx)
    require.NoError(t, err)
}

func TestVerifyHandler_Invalid(t *testing.T) {
    ctx := &plugin.VerifyContext{
        Action: "create",
        Path:   "my-plugin.item",
        Data:   `{"endpoint":"http://example.com"}`,  // Not HTTPS
    }

    err := myVerifyHandler(ctx)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "HTTPS")
}
```

## Testing Commands

```go
func TestStatusCommand(t *testing.T) {
    ctx := &plugin.CommandContext{
        Command: "status",
        Args:    nil,
    }

    result, err := myStatusHandler(ctx)
    require.NoError(t, err)

    m, ok := result.(map[string]any)
    require.True(t, ok)
    assert.Contains(t, m, "status")
}
```

## Integration Testing

Test the full plugin with mock I/O:

```go
func TestPlugin_Protocol(t *testing.T) {
    p := plugin.New("test-plugin")
    p.SetSchema(testSchema, "test")
    p.OnVerify("test", myVerifyHandler)
    p.OnCommand("status", myStatusHandler)

    var out bytes.Buffer
    p.SetOutput(&out)

    input := strings.Join([]string{
        "config done",
        "registry done",
        "#a status",
        `{"shutdown":true}`,
    }, "\n") + "\n"
    p.SetInput(strings.NewReader(input))

    err := p.Run()
    require.NoError(t, err)

    output := out.String()
    assert.Contains(t, output, "declare done")
    assert.Contains(t, output, "ready")
    assert.Contains(t, output, "@a ok")
}
```

## Table-Driven Tests

Test multiple scenarios efficiently:

```go
func TestVerifyHandler(t *testing.T) {
    tests := []struct {
        name    string
        data    string
        wantErr bool
        errMsg  string
    }{
        {"valid", `{"endpoint":"https://x.com"}`, false, ""},
        {"no_endpoint", `{}`, true, "required"},
        {"http", `{"endpoint":"http://x.com"}`, true, "HTTPS"},
        {"empty", `{"endpoint":""}`, true, "required"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := &plugin.VerifyContext{
                Action: "create",
                Path:   "test",
                Data:   tt.data,
            }

            err := myVerifyHandler(ctx)

            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## Testing State Changes

```go
func TestApplyHandler_Create(t *testing.T) {
    // Reset state
    monitors = make(map[string]*Monitor)

    ctx := &plugin.ApplyContext{
        Action: "create",
        Path:   "monitor.api",
    }

    err := myApplyHandler(ctx)
    require.NoError(t, err)

    // Verify state changed
    assert.Len(t, monitors, 1)
    assert.Contains(t, monitors, "monitor.api")
}

func TestApplyHandler_Delete(t *testing.T) {
    // Setup initial state
    monitors = map[string]*Monitor{
        "monitor.api": &Monitor{},
    }

    ctx := &plugin.ApplyContext{
        Action: "delete",
        Path:   "monitor.api",
    }

    err := myApplyHandler(ctx)
    require.NoError(t, err)

    // Verify state cleared
    assert.Len(t, monitors, 0)
}
```

## Validation Tool

Use `ze plugin validate` to check your plugin:

```bash
# Validate YANG schema
ze plugin validate schema ./my-plugin.yang

# Validate binary follows protocol
ze plugin validate binary ./my-plugin

# Test with sample config
ze plugin test ./my-plugin --config test.conf
```

## CI Integration

```yaml
# .github/workflows/test.yml
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
        run: go test -v ./...

      - name: Build plugin
        run: go build -o my-plugin

      - name: Validate
        run: |
          ze plugin validate schema schema.yang
          ze plugin validate binary ./my-plugin
```

## Coverage

Run with coverage:

```bash
go test -cover -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Target at least 80% coverage on handlers.

## Debugging

Enable debug logging:

```bash
export DEBUG=1
./my-plugin
```

Or add logging in your plugin:

```go
import "log/slog"

p.OnVerify("test", func(ctx *plugin.VerifyContext) error {
    slog.Debug("verify called",
        "action", ctx.Action,
        "path", ctx.Path,
    )
    // ...
})
```

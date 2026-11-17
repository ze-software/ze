# Testing Your Plugin

This guide covers testing strategies for ze plugins using the SDK (`pkg/plugin/sdk`).

## Checking That a Program Is a Valid Plugin

The conformance checker judges a plugin program in any language. It starts the
program in declare mode (`ZE_PLUGIN_MODE=declare`, described in
[protocol.md](protocol.md), "Query Mode: Answering Without Starting") and judges
what the program wrote. It needs no daemon and no config file.

With no daemon running, the checker is the only command that starts your
program. `ze show plugin declarations config <file>` starts no external plugin
then: it lists your plugin with the state `not-available`, and its reason names
this checker and the form a running Ze answers. A running Ze starts your
program in declare mode when you type `show plugin declarations config <file>`
in the session `ze cli` opens, and the row then carries what your plugin
declares.
<!-- source: internal/component/plugin/declarations.go -- configuredDeclarationRow, unavailableDeclarationRow, queriedDeclarationRow -->

Two commands run the same checker and write the same rows:

```bash
# The shipped binary, for a plugin author or an operator.
ze show plugin conformance run "./my-plugin --flag"
ze show plugin conformance run "./my-plugin --flag" name acme-monitor

# The development launcher, inside the Ze repository.
./le plugin conformance check run "./my-plugin --flag"
./le plugin conformance check run "./my-plugin --flag" name acme-monitor
```

The value after `run` is the run string you would configure under
`plugin { external <name> { run ...; } }`, as one quoted token. A shell starts
it, and a relative path is read from the directory you typed the command in.
`name` is the plugin name the checks use in their refusals. Without it, the
name is the file name of the program the run string starts. The program has the
budget `ze.plugin.query.timeout` sets, 5 seconds by default.
<!-- source: internal/component/plugin/conformance/register.go -- dataConformance -->
<!-- source: internal/le/plugin/conformance/actions.go -- runCheck -->
<!-- source: internal/component/plugin/conformance/conformance.go -- Run, DefaultName -->

### The report

The report holds one row for each check, in the order the protocol reaches
them. Each row carries these fields:

| Field | Value |
|-------|-------|
| `check` | The name of the check |
| `stage` | The part of the protocol the check belongs to: `declare` for query mode, a stage number, or `transport`, `runtime`, `shutdown`, `schema` |
| `verdict` | `pass`, `fail` or `not-judged` |
| `reason` | What the check found. A passed row carries none |
| `fix` | The change that clears the row. A passed row carries none |
<!-- source: internal/component/plugin/conformance/conformance.go -- Row, Verdict -->

A `not-judged` row is a check that nobody ran, and its reason says why. A report
with no failed row is therefore not full conformance: read the `not-judged`
rows to learn what is still unproven.

The first four rows judge the declare-mode run itself:

| Check | It passes when |
|-------|----------------|
| `program-starts` | The run string starts a program. A shell exit of 126 or 127 means that no program started |
| `exits-clean` | The program exits 0 inside the budget and leaves no process holding its standard output |
| `declaration-line` | Standard output holds exactly one line: `#1 ze-plugin-engine:declare-registration` and the declaration as compact JSON, ended by one line feed |
| `declaration-decodes` | The JSON of that line decodes as a Stage 1 declaration |
<!-- source: internal/component/plugin/conformance/checks.go -- programStarts, exitsClean, declarationLine, strictLine, declarationDecodes -->
<!-- source: internal/component/plugin/declarations.go -- queryExitOf -->

The next rows are the daemon's own Stage 1 checks, run over the declaration the
program wrote. The checker and the daemon read one table, so a row that fails
here is a declaration the daemon refuses, with the daemon's own reason. These
rows carry stage `1`: `protocol-version`, `filter-declarations`,
`filter-insertions`, `filter-insertion-paths`, `drain`, `resolver-caches`,
`doctor-checks`, `enrichers`, `pipe-aliases`, `answer-shapes` and `help-texts`.
The row `command-names` carries stage `5`. The daemon applies that rule at
Stage 5, where a bad name refuses the command and keeps the plugin.

When the program did not start, every later row is `not-judged`. When no
declaration was read, every Stage 1 row is `not-judged`. A row is never passed
for a declaration that nobody read.
<!-- source: internal/component/plugin/server/stage1_checks.go -- stage1Checks, JudgeDeclaration -->
<!-- source: internal/component/plugin/conformance/checks.go -- stage1Rows, judgedRow, notStartedRow -->

### What this version does not judge

These rows are `not-judged` in every report. Test each behavior under a
daemon: configure the plugin with `plugin { external <name> { run ...; } }` and
start ze.

| Check | Stage | Why the checker does not judge it |
|-------|-------|-----------------------------------|
| `ambiguous-registration` | `1` | The check compares the declaration with the other plugins the daemon runs |
| `doctor-check-declared-twice` | `1` | The check compares the declared doctor checks with the registry row of the operator's block |
| `respawn-agreement` | `1` | The check compares the declared failure policy with the `respawn` leaf of the operator's config |
| `schema-unchanged` | `1` | The check compares the declared config schema with the one the daemon read from the plugin when it started |
| `dependencies` | `1` | The check looks for each declared dependency among the plugins the daemon configured or loaded |
| `registration-conflicts` | `1` | The check writes the plugin's names into the daemon's registries, where another plugin can hold the same name |
| `tls-connect-back` | `transport` | The checker starts no hub, so the TLS connect-back and the `#0 auth` exchange do not run |
| `stage-2-config` | `2` | The checker sends no configure message |
| `stage-3-capabilities` | `3` | The checker reads no declare-capabilities message |
| `stage-4-registry` | `4` | The checker sends no share-registry message |
| `stage-5-ready` | `5` | The checker reads no ready message, so the subscriptions are not judged |
| `config-callbacks` | `runtime` | The checker opens no config transaction, so it calls no verify, apply, rollback or committed callback |
| `execute-command-framing` | `runtime` | The checker sends no execute-command, so it reads no answer line |
| `bye-and-exit` | `shutdown` | The checker sends no bye |
| `schema` | `schema` | The checker does not compile the YANG module in the `schema` field of the declaration |
<!-- source: internal/component/plugin/conformance/register.go -- init, the notJudged registrations -->
<!-- source: internal/component/plugin/server/stage1_checks.go -- stage1Checks, the entries with daemonState -->
<!-- source: internal/component/plugin/conformance/checks.go -- daemonStateRows, notJudged -->

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | No row failed. Some rows are `not-judged` |
| 1 | One row or more failed |
| 2 | The run did not happen, so there is no report: no run string, no shell on this host, an invalid budget, or a caller that is not local |
<!-- source: internal/component/plugin/conformance/conformance.go -- ExitConforms, ExitViolation, ExitNoRun, Run -->

### A sample report

This is the report of a conforming program. The `reason` and `fix` columns are
left out here to keep the lines short.

```
check                        stage      verdict
program-starts               declare    pass
exits-clean                  declare    pass
declaration-line             declare    pass
declaration-decodes          declare    pass
protocol-version             1          pass
filter-declarations          1          pass
filter-insertions            1          pass
filter-insertion-paths       1          pass
drain                        1          pass
resolver-caches              1          pass
doctor-checks                1          pass
enrichers                    1          pass
pipe-aliases                 1          pass
answer-shapes                1          pass
help-texts                   1          pass
command-names                5          pass
ambiguous-registration       1          not-judged
doctor-check-declared-twice  1          not-judged
respawn-agreement            1          not-judged
schema-unchanged             1          not-judged
dependencies                 1          not-judged
registration-conflicts       1          not-judged
tls-connect-back             transport  not-judged
stage-2-config               2          not-judged
stage-3-capabilities         3          not-judged
stage-4-registry             4          not-judged
stage-5-ready                5          not-judged
config-callbacks             runtime    not-judged
execute-command-framing      runtime    not-judged
bye-and-exit                 shutdown   not-judged
schema                       schema     not-judged
```
<!-- source: internal/component/plugin/conformance/register.go -- init, the check order and the column order -->

`| json` gives the same rows under the key `checks`. The `le` command answers
one document that holds `plugin`, `run`, `verdict` and `checks`.
<!-- source: internal/component/plugin/conformance/register.go -- keyChecks -->
<!-- source: internal/component/plugin/conformance/conformance.go -- Report -->

### The `ze` command runs locally only

`ze show plugin conformance run` starts the program that the command line
names, so it runs in your own `ze` process, under your own identity. The daemon
starts no program that a remote caller names. Through the daemon (SSH, the web
UI, REST, gRPC, MCP), the command starts nothing and answers a refusal with
code 2. The refusal gives the reason and the command to run in your own shell
on the host that holds the program.
<!-- source: internal/component/plugin/conformance/register.go -- refuseRemoteCaller -->

## Unit Testing with `net.Pipe`

The SDK is designed for testability. Use `net.Pipe()` to create a connected pair of
connections, then use `sdk.NewWithConn()` on one end and simulate the engine on the other.
<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn -->

```go
import (
    "net"
    "testing"

    "github.com/ze-software/ze/pkg/plugin/rpc"
    "github.com/ze-software/ze/pkg/plugin/sdk"
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
        errCh <- p.Run(ctx, &sdk.Registration{})
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

Stage 1 must answer the negotiated protocol version, not an empty `ok`: the SDK
refuses a missing or different version before continuing to Stage 2.
<!-- source: pkg/plugin/sdk/sdk_version.go -- acceptProtocolVersion -->

```go
func completeStartup(t *testing.T, ctx context.Context, engineMux *rpc.MuxConn) {
    t.Helper()

    // Stage 1: read declare-registration, answer the protocol version
    req := <-engineMux.Requests()
    assert.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
    require.NoError(t, engineMux.SendResult(ctx, req.ID,
        &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))

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

    p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
        if command == "show-status" {
            return "done", `{"status":"healthy"}`, nil
        }
        return "error", "unknown command", nil
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    errCh := make(chan error, 1)
    go func() {
        errCh <- p.Run(ctx, &sdk.Registration{
            Commands: []sdk.CommandDecl{
                {Name: "show-status", ShortHelp: "Show health status"},
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

    out := executeCommand(t, ctx, engineMux, cmdInput)
    assert.Equal(t, "done", out.Status)
    assert.Contains(t, string(out.Data), "healthy")
}
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->

`CallRPC` cannot read this answer. A plugin answers `execute-command` with a
head, its records and a terminator, so a test that reads one JSON result takes
the head line's tail for its payload. Read the answer with `CallAnswer` and
collapse it, which is the engine's own three steps:

```go
func executeCommand(t *testing.T, ctx context.Context, mux *rpc.MuxConn, input any) *rpc.ExecuteCommandOutput {
    t.Helper()

    answer, err := mux.CallAnswer(ctx, "ze-plugin-callback:execute-command", input)
    require.NoError(t, err)

    document, collapseErr := rpc.CollapseAnswer(answer)
    require.NoError(t, answer.Err(), "the answer must reach its terminator")
    require.NoError(t, collapseErr)
    return &rpc.ExecuteCommandOutput{Status: answer.Status, Data: document}
}
```

`answer.Err()` is read AFTER the collapse, because the walk is what fills it.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn.CallAnswer -->
<!-- source: pkg/plugin/rpc/collapse.go -- CollapseAnswer -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer, ExecuteCommandValue -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

## Testing Configuration Handling

The `OnConfigure` callback receives config sections during Stage 2.
The `OnConfigVerify` and `OnConfigApply` callbacks handle config reload. A test
pair that wants config and exercises config-verify or config-apply must register
`OnConfigApply` or call `DeclareAppliesNothing`: without either the SDK refuses
the reload, and `TestConfigApplyWithoutHandlerIsRefused` in
`pkg/plugin/sdk/sdk_callbacks_test.go` asserts that refusal.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigure, OnConfigVerify, OnConfigApply, DeclareAppliesNothing -->

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
        errCh <- p.Run(ctx, &sdk.Registration{
            WantsConfig: []string{"bgp"},
        })
    }()

    // Stage 1: answer the protocol version
    req := <-engineMux.Requests()
    require.NoError(t, engineMux.SendResult(ctx, req.ID,
        &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}))

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
<!-- source: pkg/plugin/sdk/sdk_version.go -- acceptProtocolVersion -->

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

            p.OnExecuteCommand(func(serial, cmd string, args []string, peer string) (string, any, error) {
                if cmd == "show-routes" {
                    return "done", `{"count":42}`, nil
                }
                return "error", "unknown command", nil
            })

            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            go func() { _ = p.Run(ctx, &sdk.Registration{}) }()
            completeStartup(t, ctx, engineMux)

            cmdInput := struct {
                Serial  string `json:"serial"`
                Command string `json:"command"`
            }{Serial: "1", Command: tt.command}

            out := executeCommand(t, ctx, engineMux, cmdInput)
            assert.Equal(t, tt.wantStatus, out.Status)
            assert.JSONEq(t, tt.wantData, string(out.Data))
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
# Set the base log level (ze.log); ze.log.<subsystem> sets one subsystem
export ZE_LOG=debug
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

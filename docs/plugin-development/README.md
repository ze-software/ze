# ZeBGP Plugin Development Guide

This guide explains how to create third-party plugins for ZeBGP. Plugins extend ZeBGP's configuration schema and add custom functionality.

## Quick Start (5 minutes)

### 1. Create a Plugin

```go
package main

import "codeberg.org/thomas-mangin/ze/pkg/plugin"

func main() {
    p := plugin.New("my-plugin")
    p.SetSchema(mySchema, "my-prefix")
    p.OnVerify("my-prefix", myVerifyHandler)
    p.OnCommand("status", myStatusHandler)
    p.Run()
}
```

### 2. Build and Test

```bash
go build -o my-plugin
ze plugin validate binary ./my-plugin
```

### 3. Configure ZeBGP

```
process my-plugin {
    run "./my-plugin";
}

my-prefix {
    # Your config here
}
```

## What Can Plugins Do?

- **Extend configuration** - Add new config sections with YANG schemas
- **Validate changes** - Reject invalid config before it's applied
- **React to config** - Take action when config is created/modified/deleted
- **Add commands** - Provide new API commands

## Architecture

```
ZeBGP Engine                Plugin
      │                        │
      │── config verify ──────>│  "Is this config valid?"
      │<── done/error ─────────│
      │                        │
      │── config apply ───────>│  "Apply this config"
      │<── done/error ─────────│
      │                        │
      │── command ────────────>│  "Execute command"
      │<── response ───────────│
```

Plugins communicate via stdin/stdout using a text protocol.

## Documentation

| Guide | Description |
|-------|-------------|
| [protocol.md](protocol.md) | 5-stage protocol details |
| [schema.md](schema.md) | YANG schema authoring |
| [handlers.md](handlers.md) | Verify/Apply handlers |
| [commands.md](commands.md) | Adding commands |
| [testing.md](testing.md) | Testing your plugin |

## Example Plugins

- [Go Example](../examples/plugin/go/) - Complete Go plugin
- Shell Example (coming soon)
- Python Example (coming soon)

## SDK Reference

The Go SDK (`pkg/plugin`) provides:

| Type | Purpose |
|------|---------|
| `Plugin` | Main plugin struct |
| `VerifyContext` | Context for verify handlers |
| `ApplyContext` | Context for apply handlers |
| `CommandContext` | Context for command handlers |

```go
// Create plugin
p := plugin.New("name")

// Set schema
p.SetSchema(yangText, "handler1", "handler2")

// Register handlers
p.OnVerify("prefix", func(ctx *VerifyContext) error { ... })
p.OnApply("prefix", func(ctx *ApplyContext) error { ... })
p.OnCommand("cmd", func(ctx *CommandContext) (any, error) { ... })

// Run protocol loop
p.Run()
```

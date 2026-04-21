# Compatibility Rationale

Why: `ai/rules/compatibility.md`

## Why No Backwards Compat
Ze has never been released. No users to break. Applies to: API protocols, config syntax, wire formats, CLI commands -- everything.

## ExaBGP Bridge Architecture

ExaBGP compatibility is provided via external tools, not in-engine code:

| Tool | Purpose |
|------|---------|
| `ze exabgp plugin` | Run ExaBGP plugins with Ze (bidirectional JSON/command translation) |
| `ze config migrate` | Convert ExaBGP configs to Ze format |

### File Layout
- `internal/exabgp/bridge/bridge.go` -- Core translation: `ZebgpToExabgpJSON()`, `ExabgpToZebgpCommand()`, `Bridge`
- `internal/exabgp/bridge/bridge_test.go` -- Unit tests
- `cmd/ze/exabgp/main.go` -- CLI wrapper: `ze exabgp plugin <cmd>`

### Design Rationale
- Programmatic use of translation in other Go tools (library)
- CLI for direct use in Ze `run` commands
- Testing the library independently from the CLI
- Engine code: zero ExaBGP format awareness
- Bridge tool: handles all format translation externally
- Config migration: one-time conversion, not runtime compatibility

### Usage
```
# Command line
ze exabgp plugin /path/to/exabgp-plugin.py

# Ze config
process exabgp-compat {
    run "ze exabgp plugin /path/to/exabgp-plugin.py";
}
```

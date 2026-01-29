# No Backwards Compatibility

**Ze has never been released. There are no users.**

## Ze-to-Ze Compatibility

Ze will never have backwards compatibility with itself:
- NO backwards compatibility code
- NO backwards compatibility comments
- NO legacy shims or fallbacks
- NO "for compatibility with older versions" logic

If something needs to change, just change it. Delete the old code. There is no one to break.

This applies to:
- API protocols
- Config syntax
- Wire formats
- CLI commands
- Everything

## ExaBGP Compatibility

ExaBGP compatibility is provided via **external tools**, not in-engine code:

| Tool | Purpose |
|------|---------|
| `ze exabgp plugin` | Run ExaBGP plugins with Ze (bidirectional JSON/command translation) |
| `ze bgp config migrate` | Convert ExaBGP configs to Ze format |

### Architecture

```
internal/exabgp/           # Go library - core translation logic
├── bridge.go         # ZebgpToExabgpJSON(), ExabgpToZebgpCommand(), Bridge
└── bridge_test.go    # Unit tests

cmd/ze/bgp/exabgp.go   # CLI wrapper - ze exabgp plugin <cmd>
```

This allows:
- Programmatic use of translation in other Go tools
- CLI for direct use in Ze `run` commands
- Testing the library independently

### Usage

```
# Command line
ze exabgp plugin /path/to/exabgp-plugin.py

# Ze config
process exabgp-compat {
    run "ze exabgp plugin /path/to/exabgp-plugin.py";
}
```

### What This Means

- **Engine code:** No ExaBGP format awareness, no compatibility shims
- **Bridge tool:** Handles all format translation externally
- **Config migration:** One-time conversion, not runtime compatibility

# No Backwards Compatibility

**ZeBGP has never been released. There are no users.**

## ZeBGP-to-ZeBGP Compatibility

ZeBGP will never have backwards compatibility with itself:
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
| `zebgp exabgp plugin` | Run ExaBGP plugins with ZeBGP (bidirectional JSON/command translation) |
| `zebgp config migrate` | Convert ExaBGP configs to ZeBGP format |

### Architecture

```
internal/exabgp/           # Go library - core translation logic
├── bridge.go         # ZebgpToExabgpJSON(), ExabgpToZebgpCommand(), Bridge
└── bridge_test.go    # Unit tests

cmd/zebgp/exabgp.go   # CLI wrapper - zebgp exabgp plugin <cmd>
```

This allows:
- Programmatic use of translation in other Go tools
- CLI for direct use in ZeBGP `run` commands
- Testing the library independently

### Usage

```
# Command line
zebgp exabgp plugin /path/to/exabgp-plugin.py

# ZeBGP config
process exabgp-compat {
    run "zebgp exabgp plugin /path/to/exabgp-plugin.py";
}
```

### What This Means

- **Engine code:** No ExaBGP format awareness, no compatibility shims
- **Bridge tool:** Handles all format translation externally
- **Config migration:** One-time conversion, not runtime compatibility

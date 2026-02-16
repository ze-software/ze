# Naming Convention

**"Ze" = "The" with a French accent.** It's a pun.

## Usage Rules

**Rule:** Use "ze" where "the" would work grammatically.

| Context | Use | Example |
|---------|-----|---------|
| Application name | `ze` | "Start ze BGP daemon" |
| CLI binary | `ze` | `ze bgp server config.conf` |
| BGP config YANG | `ze-bgp-conf` | `module ze-bgp-conf { ... }` |
| BGP JSON format | `ze-bgp` | `"format": "ze-bgp"` |
| Go variables for BGP | `ZeBGPConf*` | `ZeBGPConfYANG` |
| Prose/docs | `Ze` or `ze` | "Ze BGP running" |

## YANG Module Naming

- Config modules use `-conf` suffix: `ze-bgp-conf`, `ze-hub-conf`, `ze-plugin-conf`
- API modules use `-api` suffix: `ze-bgp-api` (future)
- Wire format identifier `"format": "ze-bgp"` is separate from module names
- Socket names and binary names are also separate

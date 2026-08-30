---
name: ze-config
description: Ze config syntax, validation, and common errors.
---

# Ze Config

Ze uses a YANG-modeled configuration with section-based syntax.

## Validate

```sh
ze config validate <file>
ze cli -c "validate config <file> | json"
ze config validate -q <file>
```

Exit codes: 0 = valid, 1 = invalid, 2 = file not found.

## Common Diagnostic Codes

- `config-parse`: syntax error (missing token, unknown keyword)
- `config-yang-missing`: mandatory field not present
- `config-yang-type`: wrong value type
- `config-yang-enum`: value not in allowed set
- `config-yang-range`: numeric value outside range
- `config-bgp-resolve`: BGP template resolution failure
- `config-bgp-peer`: peer settings or capability failure
- `config-listener-conflict`: two listeners on same address/port
- `config-mcp-invalid`: MCP auth/TLS consistency failure
- `config-gnmi-invalid`: gNMI listens non-loopback with no token

## Config Sections

Top-level: bgp, interface, sysctl, fib, plugin, web, ssh, dns,
telemetry, looking-glass, mcp, managed, vpp, environment.

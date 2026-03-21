# Route Injection

Ze supports injecting routes at runtime through text, hex, or base64 encoded UPDATE commands. Routes can be sent from the CLI, from external plugins, or from process scripts.

## Text Format

Human-readable format with named attributes:

```bash
ze run peer upstream1 update text \
    origin set igp \
    nhop set 192.168.1.1 \
    local-preference set 200 \
    as-path set [ 65001 65002 ] \
    community set [ 65000:100 no-export ] \
    nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24
```

### Attribute Keywords

| Attribute | Set | Add | Delete |
|-----------|-----|-----|--------|
| `origin` | `origin set igp` / `egp` / `incomplete` | -- | `origin del` |
| `nhop` | `nhop set 192.168.1.1` or `nhop set self` | -- | `nhop del` |
| `med` | `med set 100` | -- | `med del` |
| `local-preference` | `local-preference set 200` | -- | `local-preference del` |
| `as-path` | `as-path set [ 65000 65001 ]` | `as-path add [ 65000 ]` (prepend) | `as-path del` |
| `community` | `community set [ 65000:100 ]` | `community add [ 65000:200 ]` | `community del [ 65000:100 ]` |
| `large-community` | `large-community set [ 65000:1:1 ]` | `large-community add [ ... ]` | `large-community del [ ... ]` |
| `extended-community` | `extended-community set [ rt:65000:100 ]` | `extended-community add [ ... ]` | `extended-community del [ ... ]` |

### Well-Known Communities

`no-export`, `no-advertise`, `no-export-subconfed`, `nopeer`

### Next-Hop Self

`nhop set self` resolves to the local address of each destination peer at wire time.

### NLRI Operations

```bash
nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24    # Announce prefixes
nlri ipv4/unicast del 10.0.2.0/24                  # Withdraw prefix
nlri ipv4/unicast eor                               # End-of-RIB marker
```

Multiple families in one command:

```bash
nlri ipv4/unicast add 10.0.0.0/24 \
nlri ipv6/unicast add 2001:db8::/32
```

## Hex Format

Wire-encoded bytes for debugging or replay:

```bash
ze run peer upstream1 update hex \
    attr set 40010100400200400304c0a80101 \
    nhop set c0a80101 \
    nlri ipv4/unicast add 180a0000
```

## Base64 Format

Compact encoding for scripts:

```bash
ze run peer upstream1 update b64 \
    attr set QAEBAAQDAsCoBQE= \
    nlri ipv4/unicast add GAoAAA==
```

## Peer Selector

Routes are sent to peers matching the selector:

| Selector | Example | Description |
|----------|---------|-------------|
| `*` | `peer *` | All peers |
| Name | `peer upstream1` | By configured peer name |
| IP address | `peer 10.0.0.1` | Exact peer IP |
| Glob | `peer 192.168.*.*` | Pattern match |
| Exclusion | `peer !10.0.0.1` | All except this peer |

## Commit Workflow

For atomic multi-route updates:

```bash
ze run commit start my-batch
ze run peer * update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24
ze run peer * update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.1.0/24
ze run commit end my-batch         # All routes sent together
```

## From Plugins

External plugins send routes through the SDK:

```python
from ze_api import API

api = API()
# ... 5-stage startup ...
api.send("peer * update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
```

Go plugins use the SDK method:

```go
p.UpdateRoute(ctx, "*", "update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
```

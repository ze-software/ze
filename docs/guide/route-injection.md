# Route Injection

Ze supports injecting routes at runtime through text, hex, or base64 encoded UPDATE commands. Routes can be sent from the CLI, from external plugins, or from process scripts.
<!-- source: internal/component/bgp/plugins/cmd/update/ -- update text/hex/b64 command parsing -->

## Text Format

Human-readable format with flat attribute declarations:

```bash
ze cli -c "peer upstream1 update text \
    origin igp \
    nhop 192.168.1.1 \
    local-preference 200 \
    as-path [ 65001 65002 ] \
    community [ 65000:100 no-export ] \
    nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24"
```

### Attribute Keywords

Attributes are flat: keyword followed by value. No `set`/`add`/`del` on attributes.
`add` and `del` are NLRI-only operations (announce vs. withdraw).

| Attribute | Syntax | Delete |
|-----------|--------|--------|
| `origin` | `origin igp` / `egp` / `incomplete` | -- |
| `nhop` | `nhop 192.168.1.1` or `nhop self` | -- |
| `med` | `med 100` | -- |
| `local-preference` | `local-preference 200` | -- |
| `as-path` | `as-path [ 65000 65001 ]` | -- |
| `community` | `community [ 65000:100 no-export ]` | -- |
| `large-community` | `large-community [ 65000:1:1 ]` | -- |
| `extended-community` | `extended-community [ rt:65000:100 ]` | -- |

### Well-Known Communities

`no-export`, `no-advertise`, `no-export-subconfed`, `nopeer`
<!-- source: internal/component/bgp/attribute/ -- community name constants -->

### Next-Hop Self

`nhop self` resolves to the local address of each destination peer at wire time.

### NLRI Operations

`add` and `del` are NLRI operations (MP_REACH and MP_UNREACH):

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
ze cli -c "peer upstream1 update hex \
    attr set 40010100400200400304c0a80101 \
    nhop set c0a80101 \
    nlri ipv4/unicast add 180a0000"
```

## Base64 Format

Compact encoding for scripts:

```bash
ze cli -c "peer upstream1 update b64 \
    attr set QAEBAAQDAsCoBQE= \
    nlri ipv4/unicast add GAoAAA=="
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

<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw message injection -->

## Commit Workflow

For atomic multi-route updates:

```bash
ze cli -c "commit start my-batch"
ze cli -c "peer * update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24"
ze cli -c "peer * update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.1.0/24"
ze cli -c "commit end my-batch"    # All routes sent together
```
<!-- source: internal/component/cmd/commit/ -- commit command RPCs; internal/component/bgp/transaction/ -- commit manager -->

## From Plugins

External plugins send routes through the SDK:

```python
from ze_api import API

api = API()
# ... 5-stage startup ...
api.send("peer * update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
```

Go plugins use the SDK method:

```go
p.UpdateRoute(ctx, "*", "update text nhop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24")
```
<!-- source: pkg/plugin/sdk/ -- SDK DispatchCommand; test/scripts/ze_api.py -- Python SDK -->

# Monitoring

Ze provides real-time BGP event monitoring through the CLI. Events are streamed as they happen, with filtering by peer, event type, and direction.
<!-- source: internal/component/bgp/plugins/cmd/monitor/ -- monitor streaming RPCs -->

## CLI Usage

```
ze cli bgp monitor
```

### Filters

| Filter | Example | Description |
|--------|---------|-------------|
| `peer` | `peer upstream1` | Show events for one peer |
| `event` | `event update,state` | Filter by event type (comma-separated) |
| `direction` | `direction received` | Only received or sent events |

Combine filters:

```
ze cli bgp monitor peer upstream1 event update direction received
```

### Event Types

| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | Yes | Route announcements and withdrawals |
| `open` | Yes | OPEN message exchange |
| `notification` | Yes | Session error notifications |
| `keepalive` | Yes | Keepalive exchanges |
| `refresh` | Yes | Route refresh requests |
| `state` | No | Peer state changes (up/down) |
| `negotiated` | No | Capability negotiation results |
| `eor` | Yes | End-of-RIB markers |
| `rpki` | Yes | RPKI validation results |
<!-- source: internal/component/bgp/event.go -- event type definitions -->

### Output Formats

Pipe the output through format operators:

```
ze cli bgp monitor | json      # Full JSON envelope
ze cli bgp monitor | table     # Tabular format
ze cli bgp monitor | match rx  # Regex filter on output
```
<!-- source: internal/component/command/ -- ApplyJSON, ApplyTable pipe operators -->

## JSON Event Format

All events follow the ze-bgp JSON envelope:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {
      "address": "10.0.0.1",
      "asn": 65001
    },
    "message": {
      "id": 42,
      "direction": "received",
      "type": "update"
    }
  }
}
```

### UPDATE Event

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "message": {"id": 1, "direction": "received", "type": "update"},
    "update": {
      "ipv4/unicast": [
        {
          "next-hop": "10.0.0.1",
          "action": "add",
          "nlri": ["10.0.0.0/24", "10.0.1.0/24"]
        }
      ]
    },
    "origin": "igp",
    "as-path": [65001, 65002],
    "local-preference": 100
  }
}
```

### State Event

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "message": {"type": "state"},
    "state": "up"
  }
}
```

## Programmatic Access

Plugins can subscribe to events via the SDK:

```
process my-plugin {
    receive [ update state ]
}
```

The plugin receives events through its `OnEvent` callback. See [Plugins guide](plugins.md) for details.
<!-- source: internal/component/plugin/server/ -- event dispatch to plugins -->

## Single Command

For scripting, use `--run` to execute a single command and exit:

```
ze cli --run "bgp summary"
ze cli --run "rib routes received"
ze cli --run "rpki status"
```
<!-- source: cmd/ze/cli/main.go -- Run, Execute, StreamMonitor -->

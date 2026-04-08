# RPKI Origin Validation

Ze validates received BGP routes against RPKI ROA data. Invalid routes are rejected before entering the RIB. The feature connects to RTR cache servers (RFC 8210), downloads Validated ROA Payloads (VRPs), and applies the RFC 6811 origin validation algorithm to each received prefix.
<!-- source: internal/component/bgp/plugins/rpki/register.go -- bgp-rpki registration, RFCs 6811/8210 -->

## Configuration

Add the `bgp-rpki` and `bgp-adj-rib-in` plugins, then configure one or more RTR cache servers under `bgp { rpki { ... } }`.

```
plugin {
    external rpki {
        use bgp-rpki
        encoder json
    }
    external adj-rib-in {
        use bgp-adj-rib-in
        encoder json
    }
}

bgp {
    rpki {
        cache-server 192.0.2.1 {
            port 323
        }
    }

    peer peer1 {
        remote {
            ip 10.0.0.1
            as 65001
        }
        local {
            as 65000
            ip 10.0.0.2
        }
        router-id 10.0.0.2

        family {
            ipv4/unicast
        }

        process rpki {
            receive [ update ]
        }
        process adj-rib-in {
            receive [ update state ]
        }
    }
}
```

### Config Reference

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `rpki / cache-server <addr>` | list | -- | RTR cache server (keyed by IP/hostname) |
| `rpki / cache-server / port` | uint16 | 323 | RTR TCP port |
| `rpki / cache-server / preference` | uint8 | 100 | Server preference (lower preferred) |
| `rpki / validation-timeout` | uint16 | 30 | Seconds before fail-open on pending routes |
| `rpki / policy / invalid-action` | enum | reject | Action for Invalid routes: reject, log-only, accept |
| `rpki / policy / not-found-action` | enum | accept | Action for NotFound routes: accept, reject, log-only |

Multiple cache servers are supported for redundancy. VRP tables from all servers are merged (union).
<!-- source: internal/component/bgp/plugins/rpki/schema/ -- ze-rpki YANG schema -->

### Plugin Bindings

The rpki plugin must be bound to peers with `process rpki { receive [ update ] }`. The adj-rib-in plugin must also be bound with `process adj-rib-in { receive [ update state ] }` -- it provides the validation gate that holds routes pending validation.
<!-- source: internal/component/bgp/plugins/rpki/register.go -- Dependencies: bgp-adj-rib-in -->

## How It Works

### Validation States

Each received route gets one of three states (RFC 6811):

| State | Meaning | Default Action |
|-------|---------|----------------|
| Valid | Origin AS and prefix length match a VRP | Accept |
| Invalid | A VRP covers the prefix but origin AS or length doesn't match | Reject |
| NotFound | No VRP covers the prefix | Accept |

### Validation Flow

1. Ze connects to configured RTR cache servers and downloads VRPs
2. A BGP UPDATE arrives from a peer
3. The adj-rib-in plugin stores the route as "pending"
4. The rpki plugin extracts the origin AS (rightmost AS in final AS_SEQUENCE segment)
5. For each NLRI prefix, the rpki plugin looks up covering VRPs and computes the validation state
6. Valid/NotFound routes are promoted to installed; Invalid routes are discarded

### Fail-Open Safety

If the rpki plugin does not respond within `validation-timeout` seconds (default: 30), pending routes are automatically promoted to installed. This prevents route black-holing if the RPKI infrastructure is unavailable.

If all RTR cache servers disconnect, the existing VRP cache is retained until the connection is re-established. Routes continue to be validated against the last known good cache.
<!-- source: internal/component/bgp/plugins/rpki/ -- RPKI validation logic, RTR client, fail-open -->

### AS_PATH Edge Cases

| AS_PATH | Origin AS | Result |
|---------|-----------|--------|
| Normal sequence `[65000 65001]` | 65001 (rightmost) | Normal validation |
| Ends with AS_SET `{65001 65002}` | None | Always Invalid if covered by VRP |
| Empty (iBGP, no AS prepend) | None | NotFound (no origin to match) |

## CLI Commands

Query RPKI status through the ze CLI:

| Command | Description |
|---------|-------------|
| `rpki status` | Show RTR session count and VRP counts |
| `rpki cache` | Show cache server connection details |
| `rpki roa` | Show ROA table summary |
| `rpki summary` | Show validation statistics |

Example:

```
$ ze cli -c "rpki status"
{"running":true,"vrp-count-ipv4":3,"vrp-count-ipv6":0,"sessions":1}
```
<!-- source: internal/component/bgp/plugins/rpki/ -- RPKI CLI commands (status, cache, roa, summary) -->

## RPKI Validation Events

When the rpki plugin is loaded, it emits validation events that other plugins can subscribe to. A plugin subscribing to `rpki direction received` receives a JSON event for each validated UPDATE:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
    "message": {"id": 42, "type": "rpki"},
    "rpki": {
      "ipv4/unicast": {
        "10.0.1.0/24": "valid",
        "10.0.2.0/24": "invalid"
      }
    }
  }
}
```

When the ROA cache is empty: `"rpki": {"status": "unavailable"}`.
<!-- source: internal/component/bgp/plugins/rpki/ -- RPKI event emission -->

## Merged Events (bgp-rpki-decorator)

Instead of receiving separate UPDATE and rpki events, you can use the `bgp-rpki-decorator` plugin to get a single `update-rpki` event containing both the UPDATE data and the RPKI validation state:

```
plugin {
    external rpki-decorator {
        use bgp-rpki-decorator
        encoder json
    }
}

bgp {
    peer peer1 {
        process my-consumer {
            receive [ update-rpki ]
        }
        process rpki {
            receive [ update ]
        }
        process rpki-decorator {
            receive [ update rpki ]
        }
        process adj-rib-in {
            receive [ update state ]
        }
    }
}
```

The merged event contains the full UPDATE JSON with an `rpki` section injected:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "remote": {"as": 65001}},
    "message": {"id": 42, "type": "update-rpki"},
    "update": {"attr": {"origin": "igp"}, ...},
    "rpki": {"ipv4/unicast": {"10.0.1.0/24": "valid"}}
  }
}
```

If the RPKI validation does not arrive within the timeout (2 seconds), the event is emitted without the `rpki` section (graceful degradation).
<!-- source: internal/component/bgp/plugins/rpki_decorator/register.go -- bgp-rpki-decorator registration -->

## Testing RPKI Locally

The `ze-test rpki` command starts a deterministic mock RTR server that auto-generates VRPs based on the first octet of each /8 prefix:

```
ze-test rpki --port 3323
```

Validation states are predictable (for routes from AS 65001 with default flags):

| First octet | Modulo | State | Example |
|-------------|--------|-------|---------|
| 0, 3, 6, 9... | %3 == 0 | Valid | 9.0.1.0/24 |
| 1, 4, 7, 10... | %3 == 1 | Invalid | 10.0.1.0/24 |
| 2, 5, 8, 11... | %3 == 2 | NotFound | 11.0.1.0/24 |

## Without RPKI

When the rpki plugin is not loaded, routes flow directly into the adj-rib-in with zero overhead. No pending state, no validation delay. The validation gate is only activated when the rpki plugin sends `adj-rib-in enable-validation` during startup.
<!-- source: internal/component/bgp/plugins/adj_rib_in/ -- adj-rib-in validation gate -->

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Routes delayed 30s then accepted | RTR cache server unreachable | Check connectivity to cache server, verify port |
| All routes Invalid | Wrong cache server data, or origin AS mismatch | Check `rpki roa` output, verify VRP coverage |
| No VRPs loaded | RTR session not established | Check `rpki status`, verify cache server is running |
| Routes accepted without validation | rpki plugin not bound to peer | Add `process rpki { receive [ update ] }` to peer config |

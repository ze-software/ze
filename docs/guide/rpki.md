# RPKI Origin Validation

Ze validates received BGP routes against RPKI ROA data. Invalid routes are rejected before entering the RIB. The feature connects to RTR cache servers (RFC 8210), downloads Validated ROA Payloads (VRPs), and applies the RFC 6811 origin validation algorithm to each received prefix.
<!-- source: internal/component/bgp/plugins/rpki/register.go -- bgp-rpki registration, RFCs 6811/8210 -->

## Configuration

Add the `bgp-rpki` and `bgp-adj-rib-in` plugins, then configure one or more RTR cache servers under `bgp { rpki { ... } }`.

```
plugin {
    internal rpki {
        use bgp-rpki
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
}

bgp {
    rpki {
        cache-server 192.0.2.1 {
            port 323
        }
    }

    peer peer1 {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip 10.0.0.2
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
            router-id 10.0.0.2
            family {
                ipv4/unicast {
                    prefix {
                        maximum 1000000
                    }
                }
            }
        }

        attach process rpki {
            receive [ update-received ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
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
| `rpki / action / invalid` | enum | reject | Action for Invalid routes: reject, log-only, accept |
| `rpki / action / not-found` | enum | accept | Action for NotFound routes: accept, reject, log-only |
| `rpki / aspa / validation` | boolean | false | Enable ASPA path verification using RTR v2 ASPA records |
| `rpki / aspa / action / invalid` | enum | log-only | Action for ASPA Invalid routes: reject, log-only, accept |
| `rpki / aspa / action / unknown` | enum | accept | Action for ASPA Unknown routes: accept, reject, log-only |

Multiple cache servers are supported for redundancy. VRP tables from all servers are merged (union).
<!-- source: internal/component/bgp/plugins/rpki/yang/ -- ze-rpki YANG schema -->

#### Per-peer and per-group actions

The `action` block (both the origin `action` and the ASPA `action`) can also be set under a
`peer` or a `group`, overriding the global `rpki / action` for routes learned from that peer.
Only the action blocks are per-peer; `cache-server`, `validation-timeout`, and `aspa / validation`
remain global. Resolution is **peer > group > global, per leaf**: a leaf left unset on the peer
inherits the group's value, then the global value.

A listen-range group states the actions for every session it accepts. Such a session is
created from the group's template when the connection arrives, so it has no `peer` block of
its own, and it inherits what the group states. A NAMED peer that gives no
`connection / remote / ip` of its own is different: ze never builds it from the template, so
it has no address at all and it uses the global actions. It is reported at startup with
`rpki: per-peer action override ignored: no static remote ip`.

```
bgp {
    rpki {                                  /* global: caches + baseline actions */
        cache-server 192.0.2.1 { port 323; }
        action { invalid reject; not-found accept; }
    }
    group transit {
        rpki { action { invalid reject; } }        /* group default for members */
        peer customer-a {
            rpki { action { invalid log-only; } }  /* per-peer override; not-found inherits global */
        }
    }
    group ix {                                     /* listen range: no peer blocks */
        connection { remote { ip dynamic; range 192.0.2.0/24; } }
        rpki { action { invalid reject; } }         /* every session this group accepts */
    }
}
```

`show bgp rpki status` reports the effective global actions (`actions`) and the resolved per-peer
overrides with the source of each leaf (`peer-actions`). An entry names what it is: `"peer"`
carries a remote address, and `"group"` carries a listen-range group's name and states what
every session that group accepts inherits.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- buildDecisions per-peer resolution, statusCommand -->

#### Blackhole exemption

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `peer / rpki / blackhole-exempt` | boolean | false | Keep a BLACKHOLE-tagged route whose only origin-validation fault is prefix length |

The leaf resolves peer then group, and it has NO global level. RFC 7999 Section
3.3 binds the blackhole agreement to one BGP session, so a daemon-wide exemption
would reach sessions that agreed to nothing.

A blackhole prefix is as long as possible, usually a /32 or a /128, while a ROA
for the covering block carries its maxLength at the aggregate. RFC 6811 then
makes the announcement Invalid on length alone, and a session running
`action { invalid reject; }` drops it before anything can honor it. RFC 7999
Section 3.3 states that an operator must make sure origin validation does not
block a legitimate announcement carrying BLACKHOLE, and this leaf is that
mechanism.

```
bgp {
    peer transit-a {
        rpki {
            action { invalid reject; }
            blackhole-exempt true;
        }
        blackhole {
            communities blackhole;
            prefixes    192.0.2.0/24;
        }
    }
}
```

The exemption is narrow. It applies only when a covering VRP names the route's
own origin AS and disagrees on nothing but length. A wrong origin AS stays
Invalid, which is the hijack RFC 6811 exists to catch. A prefix with no covering
VRP is NotFound rather than Invalid, so the exemption never reaches it.

Set it on the same session that names a blackhole community. The exemption reads
the communities THAT session agreed to, so a peer running RTBH on `65001:666`
gets it for `65001:666`. On a session that names no community the leaf does
nothing: it would accept a route it would have rejected and discard nothing. See
[Blackhole Honoring](configuration.md#blackhole-honoring-rfc-7999) for the
`blackhole` container itself.
<!-- source: internal/component/bgp/plugins/rpki/yang/ze-rpki.yang -- blackhole-exempt; internal/component/bgp/plugins/rpki/blackhole.go -- invalidByLengthOnly, carriesAgreedBlackhole -->

### Plugin Bindings

The rpki plugin must be bound to peers with `attach process rpki { receive [ update-received ]; }`. It validates what the peer announces, so it asks for one direction. The adj-rib-in plugin must also be bound with `attach process adj-rib-in { receive [ update-received state ]; }` -- it provides the validation gate that holds routes pending validation.
<!-- source: internal/component/bgp/plugins/rpki/register.go -- Dependencies: bgp-adj-rib-in -->

## How It Works

### Validation States

Each received route gets one of three states (RFC 6811):

| State | Meaning | Default Action |
|-------|---------|----------------|
| Valid | Origin AS and prefix length match a VRP | Accept |
| Invalid | A VRP covers the prefix but origin AS or length doesn't match | Reject |
| NotFound | No VRP covers the prefix | Accept |

A prefix ze cannot parse gets Invalid and a warning, never NotFound. NotFound
states that the VRP set was consulted and covers nothing, and the default
`not-found accept` action accepts a route on that reading. A prefix that was
never validated fails closed instead.
<!-- source: internal/component/bgp/plugins/rpki/validate.go -- Validate -->

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

### Re-validation on VRP Change

When the VRP set changes, ze re-validates every tracked route and applies the
current action to each route whose state changed (RFC 6811 Section 4). The
decision reaches routes that are already installed, not only routes still
pending: an accept rewrites the state in place and keeps the route's sequence
number, and a reject removes the route from the Adj-RIB-In. No session clear and
no route refresh is needed to pick up a new ROA.

This matters most when UPDATEs arrive before the first sync completes. Those
routes validate NotFound against an empty VRP set, and the default
`not-found accept` installs them. The re-validation after the sync is what turns
an RPKI-Invalid one into a reject.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_validation.go -- applyToInstalled -->

### RTR Poll Timing

RFC 8210 Section 6 splits two intervals, and ze uses each for its own event. The
Refresh Interval from the End Of Data PDU is the wait before the next poll, and
its countdown starts when that PDU arrives. The Retry Interval is the wait after
a query FAILED, and it also covers a session that has never completed a
successful query.
<!-- source: internal/component/bgp/plugins/rpki/rtr_session.go -- pollDelay -->

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
| `show bgp rpki status` | Show RTR session count, sync state, and VRP counts |
| `show bgp rpki cache` | Show cache server connection details |
| `show bgp rpki roa` | Show ROA table summary |
| `show bgp rpki summary` | Show validation statistics |

Example:

```
$ ze cli -c "show bgp rpki status"
{"running":true,"vrp-count-ipv4":3,"vrp-count-ipv6":0,"sessions":1,"sessions-synced":1,"synced":true,"aspa-enabled":false,"aspa-records":0,"cache-servers":[{"address":"192.0.2.1","port":3323,"state":"idle","synced":true,"version":2}],"actions":{"invalid":"reject","not-found":"accept","aspa-invalid":"log-only","aspa-unknown":"accept"},"peer-actions":[]}
```

`running` says that a cache server is configured. `synced` says that a cache
server completed a sync and gave ze a VRP set. The two are different states:
while `synced` is false, ze holds no VRP set, so every prefix reads `not-found`
and the default `not-found accept` action accepts it. `sessions-synced` counts
the cache servers that delivered data, and each entry in `cache-servers` carries
its own `synced`. `state` is the RTR connection state, which returns to `idle`
between polls even after a successful sync.
<!-- source: internal/component/bgp/plugins/rpki/ -- RPKI CLI commands (status, cache, roa, summary) -->

## RPKI Validation Events

When the rpki plugin is loaded, it emits validation events that other plugins can subscribe to. A plugin subscribing to `rpki direction received` receives a JSON event for each validated UPDATE:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
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
    internal rpki-decorator {
        use bgp-rpki-decorator
    }
}

bgp {
    peer peer1 {
        attach process my-consumer {
            receive [ update-rpki ]
        }
        attach process rpki {
            receive [ update-received ]
        }
        attach process rpki-decorator {
            receive [ update-received rpki ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```

The merged event contains the full UPDATE JSON with an `rpki` section injected:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
    "message": {"id": 42, "type": "update-rpki"},
    "update": {"attr": {"origin": "igp"}, ...},
    "rpki": {"ipv4/unicast": {"10.0.1.0/24": "valid"}}
  }
}
```

If the RPKI validation does not arrive within the timeout (2 seconds), the event is emitted without the `rpki` section (graceful degradation).
<!-- source: internal/component/bgp/plugins/rpki_decorator/register.go -- bgp-rpki-decorator registration -->

## ASPA Path Verification

ASPA (Autonomous System Provider Authorization) verifies that AS_PATH hops are authorized by provider-customer relationships. ASPA records are distributed via RTR v2 (RFC 9582) alongside VRPs. Ze implements the verification algorithm from draft-ietf-sidrops-aspa-verification Section 6.

ASPA is opt-in. Enable it under the `rpki { aspa { ... } }` block. By default ASPA results are informational (included in the RPKI event JSON as `"aspa-state"`). Configure policy actions in the same block to enforce ASPA verification by rejecting routes with Invalid or Unknown paths.

### Configuration

```
bgp {
    rpki {
        cache-server 192.0.2.1 {
            port 323;
        }
        aspa {
            validation true;
            action {
                invalid reject;
            }
        }
    }
}
```

### ASPA Policy Actions

| Setting | Values | Default | Effect |
|---------|--------|---------|--------|
| `aspa / action / invalid` | reject, log-only, accept | log-only | Action when a route's AS_PATH fails ASPA verification |
| `aspa / action / unknown` | accept, reject, log-only | accept | Action when ASPA records are missing for some ASes in the path |

The default for `invalid` is `log-only` (conservative) rather than `reject` because ASPA deployment is incomplete and missing ASPA records can cause false Invalid results. Set to `reject` once your upstream providers have published ASPA records.

ASPA policy overrides origin validation: a route that is ROA Valid but ASPA Invalid will be rejected when `invalid` is set to `reject`.

### ASPA Validation States

Each route receives one of three ASPA states:

| State | Meaning |
|-------|---------|
| Valid | Every hop pair in the AS_PATH is authorized by an ASPA record |
| Invalid | At least one hop pair has an ASPA record that does not list the provider candidate |
| Unknown | No ASPA records exist for one or more customer ASNs in the path |

### How It Works

1. The RTR session negotiates v2 (with v1 fallback on error code 4)
2. The cache server sends ASPA PDUs (type 11) alongside VRPs
3. For each received UPDATE, the plugin normalizes the AS_PATH: removes consecutive duplicate ASNs (prepend artifacts), strips AS_CONFED_SEQUENCE segments, and flags AS_SET or AS_CONFED_SET as unverifiable
4. Each adjacent pair (provider candidate, customer) is checked against the ASPA cache
5. The result is included as `"aspa-state"` in the RPKI event JSON

### ASPA Event Format

The `"aspa-state"` field is included alongside per-prefix origin validation results:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "name": "upstream", "remote": {"address": "10.0.0.1", "as": 65001}},
    "message": {"id": 42, "type": "rpki"},
    "rpki": {
      "ipv4/unicast": {
        "10.0.1.0/24": "valid"
      },
      "aspa-state": "valid"
    }
  }
}
```

When ASPA validation is disabled or the cache has no ASPA records, the `"aspa-state"` field is omitted.

### Re-validation on Cache Change

Routes are tracked with their normalized AS_PATH. When ASPA cache data changes (new records from the RTR server), affected routes are automatically re-validated and updated events are emitted for any route whose ASPA state changed. If policy is set to `reject` and a route's state changes to Invalid or Unknown, the route is withdrawn from the RIB.

### Testing ASPA

The `ze-test rtr-mock` command supports ASPA records with the `--aspa` flag:

```
ze-test rtr-mock --port 3323 \
    --vrp 10.0.0.0/8,24,65001 \
    --aspa 64502:64501 \
    --aspa 64501:64500
```

The format is `customer:provider1,provider2,...` (repeatable). When ASPA records are present, the mock server uses RTR v2. Seven functional tests cover ASPA: `rpki-aspa-valid.ci`, `rpki-aspa-invalid.ci`, `rpki-aspa-unknown.ci`, `rpki-aspa-disabled.ci` for verification states, and `rpki-aspa-policy-reject.ci`, `rpki-aspa-policy-logonly.ci`, `rpki-aspa-policy-unknown-reject.ci` for policy enforcement.

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

<!-- terminal-demo: rpki -->

## Without RPKI

When the rpki plugin is not loaded, routes flow directly into the adj-rib-in with zero overhead. No pending state, no validation delay. The validation gate is only activated when the rpki plugin sends `request bgp adj-rib-in enable-validation` during startup.
<!-- source: internal/component/bgp/plugins/adj_rib_in/ -- adj-rib-in validation gate -->

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Routes delayed 30s then accepted | RTR cache server unreachable | Check connectivity to cache server, verify port |
| All routes Invalid | Wrong cache server data, or origin AS mismatch | Check `show bgp rpki roa` output, verify VRP coverage |
| No VRPs loaded | RTR session not established | Check `show bgp rpki status`, verify cache server is running |
| Routes accepted without validation | rpki plugin not bound to peer | Add `attach process rpki { receive [ update-received ]; }` to peer config |

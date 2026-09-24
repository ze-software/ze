# RPKI Origin Validation

Ze validates received BGP routes against RPKI ROA data. By default, Invalid routes remain in Adj-RIB-In with their received attributes but are ineligible for selection and export. The feature connects to RTR cache servers (RFC 8210), downloads Validated ROA Payloads (VRPs), and applies the RFC 6811 origin validation algorithm to each received prefix.
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
            trusted-network true
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
            receive [ update-received state ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```

### RTR Protocol Versions

Ze starts RTR negotiation at version 2 and can reconnect at an advertised
version 1. It does not implement version 0. Ze rejects an Unsupported Protocol
Version reply advertising version 0 rather than guessing another version.

StayRTR v0.6.4 needs `-protocol 1 -enforce.version=true` for this exchange.
Without enforcement, its unsupported-version reply can carry the new client's
initial version 0 even though the server supports version 1. Enforcement
initializes the client at version 1 before it rejects Ze's version-2 query.
This cache setting leaves Ze's version negotiation unchanged.
<!-- source: internal/component/bgp/plugins/rpki/rtr_session.go -- newRTRSession, handlePDU -->
<!-- source: test/interop/Dockerfile.stayrtr -- pinned v1 cache configuration -->

### Trusted RTR Transport

Choose TLS unless the cache is on a trusted, controlled network. Unprotected TCP
requires an explicit `trusted-network true`; that setting records the operator's
choice, not a measurement of the network's security.

For native mutual TLS, name a CA and a client identity from the
[PKI certificate store](configuration.md#pki-certificate-store):

```
bgp {
    rpki {
        cache-server 192.0.2.1 {
            source-address 192.0.2.2
            tls {
                ca-certificate rtr-ca
                certificate rtr-router
                server-name cache.example.net
            }
        }
    }
}
```

The client certificate needs its private key, its intermediate chain, and one
or more `iPAddress` subjectAltNames. The cache checks those against the client's
address as the cache sees it, including any NAT translation, and must request
and authenticate this identity. Ze validates the cache
against the named CA and the DNS name in `server-name`, using a `dNSName`
subjectAltName, never the Common Name. A DNS cache address supplies the reference
name when `server-name` is omitted; an IP cache address requires an explicit
DNS name. TLS defaults to port 324, unprotected TCP to port 323.
An authentication failure sends no RTR query. Ze never downgrades that cache
connection to plaintext, even if `trusted-network true` is also present.
Candidate configuration validates
the named credentials without changing live PKI. A committed PKI-only rotation
restarts the cache transport with the candidate identity; rollback restores the
identity from before that transaction's apply. If the transaction fails before
RPKI applies its candidate, rollback leaves the committed policy and credentials
unchanged. Transport changes retain the last complete payload set only until
its existing expiration deadline.
Reload replaces only roots named in the delivery. An omitted `bgp` or `pki`
root keeps its committed value; an explicitly removed root does not.
<!-- source: internal/component/bgp/plugins/rpki/rtr_tls.go -- buildRTRTLSConfig, startTLS; internal/component/bgp/plugins/rpki/rpki_config_verify.go -- parseRPKISections; internal/component/bgp/plugins/rpki/rpki_reload.go -- replaceConfig -->

### Config Reference

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `rpki / cache-server <addr>` | list | -- | RTR cache server (keyed by IP/hostname) |
| `rpki / cache-server / port` | 1..65535 | 323 TCP, 324 TLS | RTR transport port |
| `rpki / cache-server / trusted-network` | boolean | false | Explicitly permit unprotected TCP on a trusted, controlled network |
| `rpki / cache-server / tls / ca-certificate` | string | required with TLS | Named PKI CA for cache authentication |
| `rpki / cache-server / tls / certificate` | string | required with TLS | Named PKI client certificate and private key |
| `rpki / cache-server / tls / server-name` | string | cache DNS address | DNS reference identity; required for an IP cache address |
| `rpki / cache-server / preference` | uint8 | 100 | Server preference (lower preferred) |
| `rpki / validation-timeout` | 1..65535 | 30 | Seconds before fail-open on pending routes; a reload applies the new deadline |
| `rpki / action / invalid` | enum | reject | Action for Invalid routes: reject, log-only, accept |
| `rpki / action / not-found` | enum | accept | Action for NotFound routes: accept, reject, log-only |
| `rpki / aspa / validation` | boolean | false | Enable ASPA path verification using RTR v2 ASPA records |
| `rpki / aspa / action / invalid` | enum | reject | Action for ASPA Invalid routes: reject, log-only, accept |
| `rpki / aspa / action / unknown` | enum | accept | Action for ASPA Unknown routes: accept, reject, log-only |

Multiple cache servers are supported for redundancy. Ze tries them in preference order and uses the most preferred server that answers. A completed full synchronization atomically replaces the previous server's data; partial transfers do not replace the working set.
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
        cache-server 192.0.2.1 { port 323; trusted-network true; }
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

`running` is the flag that controls per-prefix validation. `synced` reports a
completed synchronization for a configured session whose payload set is still
current and unexpired. After a transport change, those sessions can be unsynced
while Ze still uses the previous generation's unexpired data. The summary's
`validation-enabled` reports whether validation has a usable payload set.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- buildDecisions per-peer resolution, statusCommand, validationEnabled -->

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

Cache updates also reconsider this exemption when the origin state remains
Invalid. Replacing a length-only authorization with one for a different origin
makes the retained route ineligible; restoring the correct origin can admit it
again without another UPDATE.

### Plugin Bindings

Bind the rpki plugin with `attach process rpki { receive [ update-received state ]; }`. UPDATEs supply the received routes; state events remove a disconnected peer's tracked paths. Bind adj-rib-in with `attach process adj-rib-in { receive [ update-received state ]; }` too: it owns the validation gate and retained received routes.
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
6. The configured actions determine eligibility. Rejected routes retain their received data and validation state, but cannot be selected or exported.

### Fail-Open Safety

If the rpki plugin does not respond within `validation-timeout` seconds (default: 30), pending routes are automatically promoted to installed. This prevents route black-holing if the RPKI infrastructure is unavailable.

If all RTR cache servers disconnect or fail authentication, the last complete VRP and ASPA sets remain usable only until the Expire Interval from their last completed End of Data. Expiry clears both sets and triggers re-validation. Partial responses, reconnects, and credential changes do not extend that deadline.
<!-- source: internal/component/bgp/plugins/rpki/ -- RPKI validation logic, RTR client, fail-open -->

### Re-validation on VRP Change

When the VRP set changes, Ze re-validates every tracked route and applies the
current action (RFC 6811 Section 4). A rejection marks the received route
ineligible rather than deleting it. A later valid authorization can make that
same received path eligible again, without another UPDATE or a route refresh.

RPKI events also report cache availability changes. A route received before a
nonempty ROA load initially reports `unavailable`; synchronization publishes its
current origin and ASPA verdicts even if both verdicts are unchanged. Cache
expiry publishes `unavailable` again. No second UPDATE is required.

Changing validation policy re-evaluates retained received attributes while
Adj-RIB-In holds the affected routes pending. Disabling RPKI removes only
RPKI's denial, not another validator's decision or a newer UPDATE's generation
fence. Rolling back the configuration re-applies the previous policy.
The retained-route snapshot also publishes current RPKI verdicts. This covers
an UPDATE that arrived before RPKI's asynchronous startup callback, without
waiting for another UPDATE or a later cache change. Each event groups all
retained prefixes and families from the same peer and received UPDATE, so the
RPKI decorator can correlate the complete verdict set with that UPDATE.

This matters most when UPDATEs arrive before the first sync completes. Those
routes validate NotFound against an empty VRP set, and the default
`not-found accept` installs them. The re-validation after the sync is what turns
an RPKI-Invalid one into a reject.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_validation.go -- applyToInstalled -->

### RTR Poll Timing

The Refresh Interval from End of Data controls the next successful-cache poll;
the Retry Interval controls attempts after a failed query, including a cache
that has never answered. The Expire Interval is a separate data lease starting
at End of Data. Its timer runs independently of blocked network reads and
route re-validation. A successful End of Data renews the lease; expiration
invalidates the serial base, so the next query requests a complete reset.
<!-- source: internal/component/bgp/plugins/rpki/rtr_session.go -- pollDelay; internal/component/bgp/plugins/rpki/rtr_expire.go -- prepareQuery, rtrDataLease -->

A cache answering No Data Available ends the current attempt. Ze tries the next
cache in preference order; if none answers, it retries with Reset Queries after
the Retry Interval. It does not wait for the cache to close its connection.

### AS_PATH Edge Cases

| AS_PATH | Origin AS | Result |
|---------|-----------|--------|
| Normal sequence `[65000 65001]` | 65001 (rightmost) | Normal validation |
| Ends with AS_SET `{65001 65002}` | None | Always Invalid if covered by VRP |
| Empty (iBGP, no AS prepend) | Local speaker AS | Normal validation |
| Ends with AS_CONFED_SEQUENCE or AS_CONFED_SET | Local speaker AS | Normal validation |

## CLI Commands

Query RPKI status through the ze CLI:

| Command | Description |
|---------|-------------|
| `show bgp rpki` | Show the validation counters with one row for each cache server |
| `show bgp rpki status` | Show RTR session count, sync state, VRP counts, and the effective actions |
| `show bgp rpki cache` | Show cache server connection details |
| `show bgp rpki roa` | Show ROA table summary, or the covering VRPs for a prefix |
| `show bgp rpki summary` | Show validation statistics |
| `show bgp rpki aspa` | Show the ASPA cache, or the providers for a customer AS |
| `request bgp rpki validate <prefix> <origin-asn>` | Validate one prefix against the ROA cache |
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- handleCommand, overviewCommand, statusCommand, cacheCommand, roaCommand, summaryCommand, aspaCommand, validateCommand -->

`show bgp rpki aspa <customer-asn>` answers its one result under `entries`, in
the same row shape the no-argument spelling writes, so `| count` and `| display`
act on either. `found` stays beside the rows: it separates a customer with no
ASPA record from an empty cache, which the row count alone cannot say.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- aspaCommand -->

`show bgp rpki` answers the counters and the cache server rows as siblings, so
`show bgp rpki | summary` cuts it down to the counters alone. That name is a pipe
alias the plugin declares over its own command, and it answers the same record
`show bgp rpki summary` answers. The seven counters are `vrp-count`,
`validation-enabled`, `sessions-total`, `sessions-established`,
`sessions-synced`, `aspa-enabled` and `aspa-records`.

`validation-enabled` is true when validation is active and an unexpired payload
set is available, including an authenticated empty set. It remains true while
that set is retained across a transport change, and becomes false on expiration
or when RPKI is disabled. It is not a claim that every prefix has a covering ROA.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- summaryFieldNames, summaryAliasExpansion, appendSummaryFields -->

A plugin's pipe alias lives in the daemon's registry. `ze cli` with no command
argument expands the chain in the client process instead, so `| summary` comes
back there as `pipe error: unknown pipe operator: summary`. Use
`ze cli -c "..."` as above, or the interactive session a plain ssh client
reaches. `show bgp rpki summary` works in every client.
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->

Example:

```
$ ze cli -c "show bgp rpki status | json compact"
{"running":true,"vrp-count-ipv4":3,"vrp-count-ipv6":0,"sessions":1,"sessions-synced":1,"synced":true,"aspa-enabled":false,"aspa-records":0,"cache-servers":[{"address":"192.0.2.1","port":3323,"state":"idle","synced":true,"version":2}],"actions":{"invalid":"reject","not-found":"accept","aspa-invalid":"reject","aspa-unknown":"accept"},"peer-actions":[]}
```

The `| json compact` pipe asks for that shape. Without it the answer is rendered
in the format `environment cli format default` names, whose registered value is
`text`.

`running` says that a cache server is configured. `sessions-synced` counts the
configured sessions supplying a current, unexpired set; each cache row has its
own `synced` flag. During transport rotation, a retained set may still be usable
even though none of the new sessions has synchronized. See `validation-enabled`
in the summary for data availability. `state` is the RTR connection state,
which returns to `idle`
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
            receive [ update-received state ]
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

ASPA (Autonomous System Provider Authorization) checks AS paths against published provider authorizations. Ze uses the RTR v2 wire format from draft-ietf-sidrops-8210bis-27 Section 5.12 and the verification procedures from draft-ietf-sidrops-aspa-verification-28 Section 5. RFC 9582 specifies the ROA profile, not RTR v2.

Verification runs on IPv4 unicast and IPv6 unicast routes only, as draft-ietf-sidrops-aspa-verification Section 6.2 requires. A route of any other address family carries no ASPA state, is not tracked for re-validation, and no ASPA action excludes it.
<!-- source: internal/component/bgp/plugins/rpki/aspa_verify.go -- aspaAppliesTo -->

ASPA is opt-in. Enable it under `rpki / aspa / validation` and configure `role / import` on each participating peer or group. Once enabled, the default policy keeps Invalid routes in the Adj-RIB-In but excludes them from route selection and advertisement. Valid and Unknown routes are accepted unless another configured policy rejects them.

### Configuration

```
bgp {
    rpki {
        cache-server 192.0.2.1 {
            port 323;
            trusted-network true;
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

The configured role describes **Ze's local role**. It selects the procedure for routes received on that session:

| Local `role / import` | Route received from | Procedure |
|---|---|---|
| `provider` | Customer | Upstream |
| `peer` | Peer | Upstream |
| `rs` | Route-server client | Upstream |
| `rs-client` | Transparent route server | Upstream |
| `customer` | Provider | Downstream |

Without a configured role, an ordered nonempty path is Unknown: Ze cannot infer whether the neighbor is a provider or a customer from the AS numbers. ASPA verification is not applied to iBGP UPDATEs.

### ASPA Policy Actions

| Setting | Values | Default | Effect |
|---------|--------|---------|--------|
| `aspa / action / invalid` | reject, log-only, accept | reject | Action when a route's AS_PATH fails ASPA verification |
| `aspa / action / unknown` | accept, reject, log-only | accept | Action when ASPA records are missing for some ASes in the path |

Missing ASPA records produce Unknown, not a false Invalid. Invalid means the procedure found an unauthorized relationship or a structurally invalid path. An ASPA that omits a real provider can therefore make a legitimate path Invalid. Operators can choose `log-only` or `accept` instead of the default `reject`; `log-only` records a warning without making ASPA itself a reason to exclude the route.

Both origin and path policies apply. An ASPA rejection excludes an otherwise ROA-Valid route. Repairing its ASPA state does not override a remaining origin-validation rejection.

### ASPA Validation States

Each route receives one of three ASPA states:

| State | Meaning |
|-------|---------|
| Valid | The applicable upstream path or downstream ramps satisfy the authorization procedure |
| Invalid | The procedure proves an unauthorized path, or the path is empty or contains an AS_SET |
| Unknown | Available authorizations or the configured relationship do not establish Valid or Invalid |

### How It Works

1. The RTR session starts at v2. If the cache names supported v1 in an Unsupported Version response, Ze reconnects at v1; ASPA records are unavailable at that version.
2. The cache sends ASPA PDUs alongside VRPs. An announcement replaces the provider list for one customer AS; a 12-byte withdrawal removes that customer's complete record. An AS0-only list means no authorized providers. AS0 mixed with other providers is an RTR error.
3. The BGP receive path checks the first AS against the neighbor after AS4 reconstruction. A mismatch is treated as withdrawal, except on a local transparent route-server-client session. AS_SET handling follows the receive path's RFC 9774 policy; when an AS_SET reaches ASPA verification, its result is Invalid.
4. ASPA removes consecutive duplicate ASNs. Upstream verification checks authorization from origin toward neighbor; downstream verification compares the authorized and possible ramps from both ends of the path.
5. The result appears as `"aspa-state"` in the RPKI event JSON.

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

The `"aspa-state"` field is omitted when ASPA verification does not apply, including disabled ASPA and UPDATEs containing only non-unicast families. In a mixed-family UPDATE, it describes the IPv4 and IPv6 unicast routes, not the other families sharing that event. An enabled verifier with an empty ASPA cache reports Unknown for a multi-AS path, not an omitted state.

### Re-validation on Cache Change

Routes retain their received attributes and normalized AS_PATH. When ASPA data changes, Ze re-evaluates affected routes and emits updated states. A configured rejection makes the retained route ineligible and withdraws any advertisement; it does not delete the Adj-RIB-In copy. If later cache data makes both configured validation policies accept the route, it becomes eligible and can be advertised again without a new UPDATE from the neighbor. Withdrawals, replacement UPDATEs, and session teardown prevent old cache decisions from restoring obsolete routes.

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

When the rpki plugin is not loaded, routes flow directly into Adj-RIB-In without an RPKI pending state or validation delay. The plugin enables the validation gate when startup or a configuration change adds a cache server. Removing the last cache server disables the gate and releases retained routes from RPKI policy.
<!-- source: internal/component/bgp/plugins/adj_rib_in/ -- adj-rib-in validation gate -->

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Routes delayed 30s then accepted | RPKI validation callback missing or unresponsive | Check process bindings and plugin logs; an empty or expired cache returns NotFound without this delay |
| All routes Invalid | Wrong cache server data, or origin AS mismatch | Check `show bgp rpki roa` output, verify VRP coverage |
| No VRPs loaded | Cache empty or RTR synchronization failed | Check `show bgp rpki status` and plugin logs; for TLS, check the named credentials, certificate chains, and DNS reference name |
| Routes accepted without validation | rpki plugin not bound to peer | Add `attach process rpki { receive [ update-received state ]; }` to peer config |

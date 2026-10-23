# Filter BGP Imports with IRR

Ze can build a BGP import prefix-list from an ASN or IRR AS-SET. The `bgp-filter-irr` plugin resolves the AS-SET through an IRR whois server, keeps the resulting IPv4 and IPv6 prefixes in memory, and rejects received routes that are not in that list.

Use this for a customer or peer that should only announce prefixes registered to its network. It replaces a separate `bgpq4` job and static prefix-list deployment.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- IRR resolution, refresh, and filter decisions -->
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- handleConfigure, the in-memory prefix store -->

## Complete example

This example accepts registered routes from customer AS 65001. Replace the addresses, ASN, and `AS-CUSTOMER` with values for your network.

```
plugin {
    internal bgp-adj-rib-in {
        use bgp-adj-rib-in
    }
    internal bgp-filter-irr {
        use bgp-filter-irr
    }
}

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }

    policy {
        irr {
            server whois.radb.net
            peeringdb-url https://www.peeringdb.com
            refresh-interval 3600
        }
    }

    peer customer-a {
        connection {
            remote {
                ip 198.51.100.2
            }
            local {
                ip 198.51.100.1
            }
        }
        session {
            asn {
                local 65000
                remote 65001
            }
            family {
                ipv4/unicast {
                    prefix {
                        maximum 1000
                    }
                }
                ipv6/unicast {
                    prefix {
                        maximum 1000
                    }
                }
            }
            irr {
                as-set AS-CUSTOMER
            }
        }
        filter {
            import {
                filters [ bgp-filter-irr:65001 ];
                final accept;
            }
        }
        attach process bgp-adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- filter-policy -->

The two plugin blocks load the filter and Adj-RIB-In. The Adj-RIB-In plugin is not required to make the filter decision, but it provides `show bgp adj-rib-in` so you can verify which routes passed.

The suffix in `bgp-filter-irr:65001` is the peer's remote ASN. It selects the dynamic list associated with AS 65001. Use the corresponding ASN in each peer's filter reference. Prefixes that do not match that list are rejected by the implicit deny.

No `attach process bgp-filter-irr` block is needed. A filter reference invokes the plugin directly through the BGP import filter chain.

<!-- source: internal/component/bgp/reactor/filter_chain.go -- named import filter execution -->
<!-- source: internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang -- peer AS-SET configuration -->

## Add it to an existing stored configuration

If the stored `ze.conf` already contains the BGP peer but no IRR filter, stop Ze and add these settings to the stored configuration. An offline `ze config set` writes the file path it is given and never resolves a stored name, so the stored configuration travels through stdin: `ze config cat` reads it, one `ze config set -` stage applies each setting, and `ze config import` stores the result as a new version.

```console
$ sudo systemctl stop ze.service
$ ze config cat ze.conf |
> ze config set - plugin internal bgp-filter-irr use bgp-filter-irr |
> ze config set - bgp policy irr server whois.radb.net |
> ze config set - bgp policy irr refresh-interval 3600 |
> ze config set - bgp peer customer-a session irr as-set AS-CUSTOMER |
> ze config set - bgp peer customer-a filter import filters bgp-filter-irr:65001 |
> ze config set - bgp peer customer-a filter import final accept |
> ze config import --yes --name ze.conf -
$ sudo systemctl start ze.service
```

The import replaces the stored `ze.conf` only because `--yes` confirms it; the previous version stays available to `ze config rollback`. Nothing here contacts the daemon, which is stopped anyway. Starting Ze loads the plugin and resolves the AS-SET in the background. An UPDATE that arrives before the list waits up to 4 seconds for it, and is then rejected and kept for another evaluation, as [After a restart](#after-a-restart) describes. Replace `ze.conf`, `customer-a`, `AS-CUSTOMER`, and `65001` with the stored configuration key, peer name, AS-SET, and remote ASN for your deployment.
<!-- source: internal/component/config/cli/editor_stdin.go -- openEditableConfig -->
<!-- source: internal/component/config/cli/cmd_import.go -- cmdImportWithStorage -->

Run `ze config cat ze.conf` before and after these commands when you want to review the exact stored changes. The terminal demonstration below starts from a configuration without any IRR settings and executes this workflow.

## Choose the AS-SET

Set `session { irr { as-set AS-CUSTOMER; } }` when you know the correct AS-SET. This is the most predictable production configuration.

If you omit `as-set`, Ze queries PeeringDB with the remote ASN. It uses the registered `irr_as_set` value when one exists, then falls back to the origin object `AS<number>` if PeeringDB does not return an AS-SET.

For a peer that must bypass this policy, set:

```
session {
    irr {
        enable disable
    }
}
```

## Changing IRR settings on a running router

A commit that changes a peer's `as-set`, the IRR `server`, or a peer's
`filter import filters` reference takes effect on the running filter. A changed
as-set does not inherit the list of the old one: the next UPDATE from that peer
waits a few seconds for the new set to resolve, then is filtered against it.
Deleting the `as-set` counts as a change: Ze asks PeeringDB again, as it would
for a peer that never named one, and filters against the set PeeringDB returns.
If PeeringDB does not answer, Ze queries `AS<asn>` itself until the next
refresh after PeeringDB answers. With no `as-set`, every refresh asks PeeringDB,
and Ze caches each PeeringDB answer for an hour, so a change to the registered
AS-SET is followed at the first refresh after the cached answer expires: up to
about two hours at the default `refresh-interval` of 3600 seconds, and up to a
day at 86400. A configuration reload starts a new PeeringDB cache, so the next
refresh after a reload sees the change. `clear peeringdb cache asn <asn>`
empties the cached answer without a reload, so the next refresh asks PeeringDB
again.
<!-- source: internal/component/bgp/plugins/filter_irr/config_tx.go -- newIRRConfigTx -->
<!-- source: pkg/plugin/sdk/config_tx.go -- ConfigTx.Apply -->
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- handleConfigure -->
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- runFilterIRR, clearResolverCache -->

## Verify before carrying traffic

After committing the configuration and establishing the BGP session, check the generated list:

```console
$ ze cli -c 'show bgp irr | no-more'
$ ze cli -c 'show bgp irr prefix customer-a | no-more'
```

These commands use the CLI's default `text` format. Its indented key/value layout can resemble YAML, but it is not YAML. `text` is the default unless `environment { cli { format { default ... } } }` changes it. Select another format explicitly with `| yaml`, `| json`, or `| table`.

The `prefix` command takes the configured peer name. Confirm that the status is `ok`, the resolved AS-SET is correct, and the prefix counts are plausible.

Test both a registered and an unregistered route without changing policy:

```console
$ ze cli -c 'show bgp irr check customer-a 203.0.113.0/24 | no-more'
$ ze cli -c 'show bgp irr check customer-a 192.0.2.0/24 | no-more'
```

Each result reports `accepted: true` or `accepted: false`. Use `show bgp adj-rib-in` to confirm that only accepted routes entered the inbound RIB.

## Refresh and failure behavior

Ze refreshes each list at `refresh-interval`. You can also refresh all enrolled peers, one ASN, or every peer using one AS-SET:

```console
$ ze cli -c 'update bgp irr all'
$ ze cli -c 'update bgp irr asn 65001'
$ ze cli -c 'update bgp irr as-set AS-CUSTOMER'
```

A refresh that returns prefixes atomically replaces the in-memory list. The firewall IRR plugin also writes its lists to ZeFS; the BGP filter does not (see "After a restart" below). Two other outcomes keep the last known good list instead, and both record the reason in `show bgp irr`:

- The refresh failed: the server was unreachable, refused the query, or sent a truncated reply.
- The refresh succeeded and returned no prefixes at all. Ze treats "I have nothing" the same as a failure, because an empty allow-list rejects every route the peer sends. A server that answers `D` (key not found), answers with no records, or reports an error can therefore not empty a live list.

The prefixes an empty answer left in place are still enforced. `show bgp irr` and `show firewall irr` mark the entry `stale` and give the age of the data, so enforcing data nobody has confirmed is visible rather than silent.

The two address families are kept separately. Ze queries IPv4 and IPv6 with two commands, and a server that returns prefixes for one and nothing for the other keeps the last known good list of the family that returned nothing. The entry is marked `stale` and reports the age of the older list. This matters most for an interface binding, where the rules accept the registered prefixes of each family and drop the rest: a family whose list was emptied would have every packet dropped.

Removing prefixes is always deliberate. When an AS-SET is deregistered upstream for good, clear its cached data:

```console
$ ze cli -c 'clear firewall irr as-set AS-CUSTOMER'
$ ze cli -c 'clear firewall irr asn 65001'
```

The entry disappears from memory and from ZeFS, and the firewall tables are re-applied. Firewall configuration that still references the name fails to verify until it is fetched again with `update firewall irr`.

### After a restart

The BGP filter keeps its prefix-lists in memory only and writes nothing to disk, so a restarted Ze starts with no list for any ASN and resolves every enrolled ASN again from the IRR server. A configuration reload keeps the lists already resolved, except for an ASN whose AS-SET changed, which starts from no list.

Until an ASN's first resolution completes, an UPDATE from its peer waits for it, for at most 4 seconds. An UPDATE still without a list after that wait has its announced routes rejected, and they are kept for another evaluation. When the list arrives, the filter asks the peer for them again by ROUTE-REFRESH, as described in the next section, so the peer needs Route Refresh negotiated and the `refresh` permission. A peer without them keeps those routes rejected until it sends them again.

When the IRR server cannot be reached at startup, no last known good list exists to fall back to: the peer's announcements are rejected, and the filter retries the lookup every minute while it holds a rejected route, until the server answers.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- firstResolveWait, handleConfigure, forgetUnconfigured -->

At first enrollment, do not place the session into service until `show bgp irr` reports `status: ok`. A peer without loaded prefix data has no list from which to make a useful allow decision.

### A rejected route is evaluated again when the list changes

A route rejected because its ASN had no list yet, or because the list did not hold it, is kept for another evaluation. When a list arrives or a refresh changes it, the filter checks what it kept, and for each peer with a route the new list authorizes it sends a ROUTE-REFRESH for that family. The peer sends its routes again and the filter decides them against the new list. While an ASN that holds a rejected route has no list, or its last lookup failed, the filter retries the lookup every minute.

The ROUTE-REFRESH needs two things. Route Refresh must be negotiated with the peer: Ze advertises it only when the peer's `capability` block, or its group's, says `route-refresh enable`, and the peer must advertise it too. The peer must also give the filter the permission to send it, with `refresh` or `*` in the `send` list:

```text
capability {
    route-refresh enable
}
attach process bgp-filter-irr {
    send [ refresh ]
}
```

A `send` list on the peer replaces the group's, so a peer that states its own `send` list for the filter needs `refresh` in that list. Without either piece, the route stays rejected until the peer sends it again, and the filter logs one warning for the peer that names the missing piece. When a configuration is applied, the filter also logs one warning for each IRR-filtered peer that lacks the permission, and one for each to which Ze does not advertise Route Refresh. If the ROUTE-REFRESH fails for another reason, for example a timeout, the filter asks again on its next pass. At most 100 000 rejected prefixes are kept; past that a rejected prefix is evaluated again only when its peer sends it again, and the filter logs one warning. A withdrawal, an accepted announcement and a session that goes down each drop what was kept for that route or peer.

An UPDATE that announces some routes the list authorizes and some it does not keeps the authorized ones and drops the rest, in the IPv4 NLRI field and in a multiprotocol block of IPv4 or IPv6 unicast, multicast or labeled unicast alike. An UPDATE that withdraws routes keeps its withdrawals even when the filter rejects every route it announces, and even before the ASN has a list. The exception is a rejected route in a multiprotocol block of another family, a FlowSpec rule for example, which Ze cannot remove from the UPDATE: that UPDATE is rejected whole, its authorized routes and its withdrawals included. The filter then asks the peer for the authorized routes again every minute, and the peer answers with the same UPDATE, so those routes stay rejected until the peer stops announcing the refused one.

<!-- source: internal/component/bgp/plugins/filter_irr/readmit.go -- readmitter, heldPrefixMax, readmitRetryInterval -->

<!-- source: internal/component/bgp/plugins/filter_irr/command.go -- show, check, and manual refresh commands -->
<!-- source: internal/component/resolve/irr/store/store.go -- atomic refresh, last-known-good, and the firewall plugin's persistence -->
<!-- source: internal/component/resolve/irr/client.go -- RPSL reply classification (parseReply) -->
<!-- source: internal/component/firewall/plugins/irr/command.go -- firewall show and clear commands -->

## Local demonstration

The recording starts with a stored BGP peer that has no IRR plugin, server, AS-SET, or import filter. It adds all five settings through a `ze config set -` pipeline that it imports back into ZeFS, then starts Ze against `le test irr`, a deterministic local whois server. Its `AS-TEST` object contains `10.0.0.0/24` but not `192.168.0.0/24`. A local BGP peer announces both routes. Ze constructs the list, accepts the registered route, rejects the other route, and shows only the accepted route in Adj-RIB-In.

<!-- source: internal/test/mock/irr/irr.go -- deterministic AS-TEST responses -->
<!-- source: demos/terminal/irr-filter/ze.conf -- baseline BGP configuration without IRR filtering -->
### Demo: Add IRR filtering to an existing BGP peer

Populate a stored configuration with the IRR plugin, server, AS-SET, and import filter, then prove registered routes pass and unregistered routes do not.

[Download the asciicast recording](../../assets/demos/irr-filter.cast?v=603992bcbd) · [Plain-text transcript](../../assets/demos/irr-filter.txt?v=4f5412294e)

Recorded with Ze 26.10.09 on macOS and Linux using Ze recorder. Duration: 3 minutes 31 seconds.

```console
$ ze config cat ze.conf | grep -q bgp-filter-irr || echo 'IRR filtering is not configured'
IRR filtering is not configured
$ ze config cat ze.conf |
> ze config set - plugin internal bgp-filter-irr use bgp-filter-irr |
> ze config set - bgp policy irr server 127.0.0.1:4343 |
> ze config set - bgp policy irr refresh-interval 3600 |
> ze config set - bgp peer customer-a session irr as-set AS-TEST |
> ze config set - bgp peer customer-a filter import bgp-filter-irr:65001 |
> ze config import --yes --name ze.conf -
set plugin internal bgp-filter-irr use bgp-filter-irr
set bgp policy irr server 127.0.0.1:4343
set bgp policy irr refresh-interval 3600
set bgp peer customer-a session irr as-set AS-TEST
set bgp peer customer-a filter import bgp-filter-irr:65001
replaced ze.conf (... bytes)
1 file(s) imported
$ ze config cat ze.conf | grep -E 'bgp-filter-irr|AS-TEST|127.0.0.1:4343'

$ ze cli -c 'show bgp irr | no-more'
asn: 65001
as-set: AS-TEST
status: ok
ipv4-count: 3
ipv6-count: 1
$ ze cli -c 'show bgp irr prefix customer-a | no-more'
10.0.0.0/24
10.0.1.0/24
172.16.0.0/16
2001:db8::/32
$ ze cli -c 'show bgp irr check customer-a 10.0.0.0/24 | no-more | yaml'
accepted: true
matched-entry: 10.0.0.0/24
$ ze cli -c 'show bgp irr check customer-a 192.168.0.0/24 | no-more | yaml'
accepted: false
$ ze cli -c 'show bgp adj-rib-in | no-more'
10.0.0.0/24

The deterministic AS-TEST registry contains 10.0.0.0/24 but not 192.168.0.0/24. Ze builds the import prefix-list, accepts the registered route, rejects the unregistered route, and installs only 10.0.0.0/24 in Adj-RIB-In.

The operational commands use Ze's default text format. Its key/value layout resembles YAML, but YAML is only selected explicitly with `| yaml`.
```


## Troubleshooting

| Symptom | Check | Resolution |
|---------|-------|------------|
| `show bgp irr` has no entry for the ASN | Plugin and filter reference | Load `bgp-filter-irr` and configure `filter { import { filters [ bgp-filter-irr:<remote-asn> ]; final accept; } }` on the peer. |
| Status stays `pending` | IRR connectivity | Permit outbound TCP to the configured whois server and run `update bgp irr asn <asn>`. |
| Status is `error` | `error` field in `show bgp irr` | Correct the AS-SET or server, then refresh. The last known good list remains active. |
| Status is `stale` | `stale-since` and `data-age-seconds` in `show firewall irr` | The last refresh returned no prefixes. Confirm the AS-SET still exists and the server is reachable, then refresh. The previous prefixes stay enforced meanwhile. |
| Status is `oversized`, and the rules naming the entry filter nothing | `ipv4-count` and `ipv6-count` in `show firewall irr` | One family holds more than the 500000 prefixes a firewall set takes. Ze programs none of the list, because a set holding part of it drops traffic an `accept` rule was written to pass. Narrow the AS-SET to the members you filter on, or run `clear firewall irr as-set <name>`. |
| An AS-SET no longer exists and its prefixes are still enforced | `show firewall irr` | Run `clear firewall irr as-set <name>`. An empty answer never removes prefixes, so removal is an operator action. |
| The wrong AS-SET is selected | `as-set` in `show bgp irr` | Configure the expected AS-SET explicitly under the peer session. |
| A valid customer route is rejected | `show bgp irr check <peer-name> <prefix>` | Confirm the exact prefix is registered in the selected IRR sources and refresh the list. |
| An unregistered route appears in Adj-RIB-In | Active import chain | Confirm the peer uses the IRR filter and has not set `irr { enable disable; }`. |

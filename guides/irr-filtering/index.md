# Filter BGP Imports with IRR

Ze can build a BGP import prefix-list from an ASN or IRR AS-SET. The `irr` plugin resolves the AS-SET through an IRR whois server and stores the resulting IPv4 and IPv6 prefixes, and the `bgp-filter-irr` plugin rejects received routes that are not in that stored list.

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

system {
    irr {
        server whois.radb.net
        refresh-interval 3600
    }
    peeringdb {
        url https://www.peeringdb.com
    }
}

bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
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
> ze config set - system irr server whois.radb.net |
> ze config set - system irr refresh-interval 3600 |
> ze config set - bgp peer customer-a session irr as-set AS-CUSTOMER |
> ze config set - bgp peer customer-a filter import filters bgp-filter-irr:65001 |
> ze config set - bgp peer customer-a filter import final accept |
> ze config import --yes --name ze.conf -
$ sudo systemctl start ze.service
```

The import replaces the stored `ze.conf` only because `--yes` confirms it; the previous version stays available to `ze config rollback`. Nothing here contacts the daemon, which is stopped anyway. Starting Ze loads the plugin; the `irr` plugin fetches the AS-SET in the background the first time and stores it. An UPDATE that arrives before the list is rejected at once and kept for another evaluation, as [After a restart](#after-a-restart) describes. Replace `ze.conf`, `customer-a`, `AS-CUSTOMER`, and `65001` with the stored configuration key, peer name, AS-SET, and remote ASN for your deployment.
<!-- source: internal/component/config/cli/editor_stdin.go -- openEditableConfig -->
<!-- source: internal/component/config/cli/cmd_import.go -- cmdImportWithStorage -->

Run `ze config cat ze.conf` before and after these commands to review the stored changes. This offline workflow is separate from the live SSH workflow in the demonstration below.
<!-- source: internal/component/config/cli/cmd_import.go -- cmdImportWithStorage -->

## Choose the AS-SET

Set `session { irr { as-set AS-CUSTOMER; } }` when you know the correct AS-SET. This is the most predictable production configuration.

Tab completes the name. On the peer's `session irr as-set`, on the firewall's AS-SET leaves, and after `update irr as-set` and `clear irr as-set`, Ze offers every AS-SET the `irr` plugin knows: each name a consumer references and each name it has stored, in upper case, marked "AS-SET with a stored IRR dataset" or "AS-SET a consumer references, nothing stored yet". The suggestion refuses nothing, so an AS-SET nothing references yet stays valid: writing it is how you ask for it.
<!-- source: internal/plugins/irr/completion.go -- knownASSets -->
<!-- source: internal/plugins/irr/register_completion.go -- asSetHelp -->
<!-- source: internal/plugins/irr/register_commands.go -- completeASSetArgument -->

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

A commit that changes a peer's `as-set` or a peer's `filter import filters`
reference takes effect on the running filter. A changed as-set does not inherit
the list of the old one: when the new set is stored the filter enforces it at
once; when it is not, the `irr` plugin fetches it right after the commit, and an
UPDATE from that peer before it arrives has its announced routes rejected and
kept, and the filter asks the peer for them again once the new list arrives.
Deleting the `as-set` counts as a change: the filter then enforces the set the
`irr` plugin's PeeringDB answer for the remote ASN names, as for a peer that
never named one. The `irr` plugin stores that answer, so a restart queries the
same set without asking PeeringDB, and asks PeeringDB again when the set is due
for a refresh; if PeeringDB does not answer, the stored set is kept. Where and
when the `irr` plugin fetches is `system irr` and `system peeringdb url`
([command reference](../command-reference/index.md), "show, update and clear irr").
<!-- source: internal/component/bgp/plugins/filter_irr/config_tx.go -- newIRRConfigTx -->
<!-- source: pkg/plugin/sdk/config_tx.go -- ConfigTx.Apply -->
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- handleConfigure -->
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- runFilterIRR -->
<!-- source: internal/plugins/irr/irr.go -- clearResolverCache -->

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

The `irr` plugin refreshes each stored list when `system irr refresh-interval` says it is due, counted from the last time the server answered. You can also fetch every set a consumer names, the set of one ASN, or one AS-SET at once; the filter enforces each stored list when the `irr` plugin announces it:

```console
$ ze cli -c 'update irr all'
$ ze cli -c 'update irr asn 65001'
$ ze cli -c 'update irr as-set AS-CUSTOMER'
```

A refresh that returns prefixes is stored by the `irr` plugin, and the filter replaces the list it enforces atomically when it reads the stored copy (see "After a restart" below). Three other outcomes keep the last known good list instead, and each records the reason in `show bgp irr`:

- The refresh failed: the server was unreachable, refused the query, or sent a truncated reply.
- The refresh returned more than 500000 prefixes across both families. The BGP filter refuses the list whole rather than cutting it, because a cut kept the IPv4 half and lost IPv6; the error gives the IPv4 and IPv6 counts. Narrow the AS-SET.
<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- prefixListFromIRR -->

- The refresh succeeded and returned no prefixes at all. Ze treats "I have nothing" the same as a failure, because an empty allow-list rejects every route the peer sends. A server that answers `D` (key not found), answers with no records, or reports an error can therefore not empty a live list.

The prefixes an empty answer left in place are still enforced. `show bgp irr` and `show firewall irr` mark the entry `stale` and give the age of the data, so enforcing data nobody has confirmed is visible rather than silent.

The two address families are kept separately. Ze queries IPv4 and IPv6 with two commands, and a server that returns prefixes for one and nothing for the other keeps the last known good list of the family that returned nothing. The entry is marked `stale` and reports the age of the older list. This matters most for an interface binding, where the rules accept the registered prefixes of each family and drop the rest: a family whose list was emptied would have every packet dropped.

Removing prefixes is always deliberate. When an AS-SET is deregistered upstream for good, clear its cached data:

```console
$ ze cli -c 'clear irr as-set AS-CUSTOMER'
$ ze cli -c 'clear irr asn 65001'
```

The `irr` plugin removes the stored copy, and BGP and the firewall stop enforcing it on its notice; the answer names every consumer that still references it. A cleared set is not fetched again until `update irr` or a reference that newly names it. Firewall configuration that still references the name fails to verify until it is fetched again with `update irr as-set <name>` or `update irr asn <asn>`.

### After a restart

The prefix-lists survive a restart. The `irr` plugin fetches every AS-SET a filter enforces, stores each list in Ze's state store, and the BGP filter reads the stored lists when it starts. A restarted Ze sends no IRR or PeeringDB query at start for a set that is stored: `show bgp irr` reports the same AS-SETs, counts, `generation` and `refreshed-at` as before the restart, and an UPDATE is judged against the stored list at once. A refresh happens only when `system irr refresh-interval` says it is due, counted from the last time the server answered (`show irr` gives the next refresh), or when you run `update irr all`, `update irr as-set <name>` or `update irr asn <asn>`. When the IRR server cannot be reached, the stored list stays enforced.

A configuration reload keeps the lists already enforced, except for an ASN whose AS-SET changed, which starts from no list until the new set's stored list is read. A set that is not stored yet, on the first start or after a commit that names a new one, is fetched by the `irr` plugin at once. Until it arrives, an UPDATE from its peer has its announced routes rejected at once, with no wait, and they are kept for another evaluation; its withdrawals still apply. `show bgp irr` reports the ASN `pending` meanwhile. When the list arrives, the filter asks the peer for them again by ROUTE-REFRESH, as described in the next section, so the peer needs Route Refresh negotiated and the `refresh` permission. A peer without them keeps those routes rejected until it sends them again.

`clear irr as-set <name>` removes the stored list: the filter then rejects and holds that set's announcements, as for a set not stored yet, until `update irr as-set <name>` stores it again. A stored list the filter cannot read, for example one that failed its integrity check, is reported by `ze show doctor` under `doctor-bgp-irr-stored-copy-unreadable`, and the filter keeps the list it read before.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- handleFilterUpdate, handleConfigure, apply -->
<!-- source: internal/component/bgp/plugins/filter_irr/stored.go -- storeReader -->

At first enrollment, do not place the session into service until `show bgp irr` reports `status: ok`. A peer without loaded prefix data has no list from which to make a useful allow decision.

### A rejected route is evaluated again when the list changes

A route rejected because its ASN had no list yet, or because the list did not hold it, is kept for another evaluation. When a list arrives or a refresh changes it, the filter checks what it kept, and for each peer with a route the new list authorizes it sends a ROUTE-REFRESH for that family. The peer sends its routes again and the filter decides them against the new list. The filter itself never fetches: a fetch that failed is retried every minute by the `irr` plugin, and the list it then stores reaches the filter.

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

<!-- source: internal/component/bgp/plugins/filter_irr/readmit.go -- readmitter, heldPrefixMax, readmitInterval -->

<!-- source: internal/component/bgp/plugins/filter_irr/command.go -- show, check, and manual refresh commands -->
<!-- source: internal/component/resolve/irr/store/store.go -- atomic refresh, last-known-good, and the firewall plugin's persistence -->
<!-- source: internal/component/resolve/irr/client.go -- RPSL reply classification (parseReply) -->
<!-- source: internal/component/firewall/plugins/irr/command.go -- firewall show and clear commands -->

## Local demonstration

The demonstration starts Ze with a customer peer but no IRR policy. It loads the server, AS-SET and import filter sections into one SSH candidate, compares them, then commits. The IRR service is a deterministic local test server, not a public registry. Its `AS-TEST` object contains `10.0.0.0/24` but not `192.168.0.0/24`.
<!-- source: demos/terminal/topics/irr/configure.tape -- sectioned live configuration -->
<!-- source: internal/test/mock/irr/irr.go -- irrResponses -->

The customer announces both routes and two control prefixes. The check commands explain decisions; they do not prove an UPDATE arrived. Before each reannouncement, the lab waits for the previous customer session to depart and its Adj-RIB-In to clear. It puts the current policy's accepted control last in the fixture's actual send order: `10.0.1.0/24` for `AS-TEST`, or `192.0.2.0/24` for `AS-V4ONLY`. Seeing that control in Adj-RIB-In proves the earlier announcements were processed. The outside prefix and the other policy's control must be absent.

The failure changes the peer to `AS-V4ONLY`, which lacks the customer prefix. After the new session's final control arrives, `10.0.0.0/24` must be absent. The repair restores `AS-TEST` through SSH, refreshes it with `update irr as-set AS-TEST`, then announces again and checks the customer route and its final control. Customer establishment has a separate deadline covering Ze's existing maximum 60-second reconnect backoff plus handshake time; policy convergence still uses its shorter bound. Ze stays running throughout.
<!-- source: demos/terminal/topics/irr/show.tape -- failure, diagnosis and live repair -->
<!-- source: internal/le/site/terminaldemo/scenarios_routing.go -- checkTopicIRR, requireIRRState -->
<!-- source: internal/le/site/terminaldemo/validate_policy_evidence.go -- orderIRRAnnouncements, requireIRRCustomerCleared, awaitIRRCustomer, requireIRRRoutes -->
### Demo: Add IRR filtering to an existing BGP peer

Load the local test IRR server, AS-SET and import policy over SSH, prove route acceptance and refusal, then diagnose and repair a wrong AS-SET without restarting Ze.

[Download the asciicast recording](../../assets/demos/irr-filter.cast?v=20eb104aee) · [Plain-text transcript](../../assets/demos/irr-filter.txt?v=cc64033bb9)

Recorded with Ze 26.11.20 in a Linux namespace lab using Ze recorder. Duration: 3 minutes 36 seconds.

```console
IRR: live import filtering and repair

The running base router has a customer in AS 65001 and no IRR policy.
The IRR service at 127.0.0.1:4343 is a local test server, not a public registry.
AS-TEST contains 10.0.0.0/24 but not 192.168.0.0/24.

$ ze-demo card topic-irr lab
$ ze cli -c 'show bgp irr | yaml | no-more'

Load three sections in one SSH candidate:
$ ze-demo card topic-irr overview
$ sshpass -e ssh ze-demo
ze# load file absolute merge topics/irr/1-server.conf
ze# load file absolute merge topics/irr/2-as-set.conf
ze# load file absolute merge topics/irr/3-import.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit

The customer announces both disputed routes and two controls. Inspect show bgp
irr, show bgp irr prefix customer and show bgp irr check customer for each prefix.
Adj-RIB-In must contain 10.0.0.0/24 and the final accepted 10.0.1.0/24 control,
but omit both 192.168.0.0/24 and the wrong-set 192.0.2.0/24 control.

$ ze-demo card topic-irr proof
$ ze cli -c 'show bgp irr | yaml | no-more'
$ ze cli -c 'show bgp irr prefix customer | yaml | no-more'
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp irr check customer 192.168.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

Load 4-wrong-as-set.conf in the SSH editor and commit. The local AS-V4ONLY
object is valid, but its 192.0.2.0/24 prefix does not authorize this customer.
Before reannouncing, wait for the old session and its routes to clear. The new
fixture sends AS-V4ONLY's accepted 192.0.2.0/24 control last. Only after that
control arrives can the absence of both 10.0 routes prove refusal, not delay.

$ ze-demo card topic-irr failure
$ sshpass -e ssh ze-demo
ze# load file absolute merge topics/irr/4-wrong-as-set.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp irr prefix customer | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

Load 2-as-set.conf again and commit the repair. Run update irr as-set
AS-TEST, then ze-demo run showcase-lab control customer reannounce. Inspect
the accepted decision, 10.0.0.0/24 and its final 10.0.1.0/24 control again.
The wrong-set control and outside route must be absent. Ze stays running.

$ ze-demo card topic-irr recovery
$ sshpass -e ssh ze-demo
ze# load file absolute merge topics/irr/2-as-set.conf
ze# show | compare
ze# commit now
ze# exit
ze> exit
$ ze cli -c 'update irr as-set AS-TEST | yaml | no-more'
$ ze-demo run showcase-lab control customer reannounce
$ ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'
$ ze cli -c 'show bgp adj-rib-in | no-more'

These are the authored walkthrough steps. Runtime validation and recording
must succeed before this source is published as a recording.
```


## Troubleshooting

| Symptom | Check | Resolution |
|---------|-------|------------|
| `show bgp irr` has no entry for the ASN | Plugin and filter reference | Load `bgp-filter-irr` and configure `filter { import { filters [ bgp-filter-irr:<remote-asn> ]; final accept; } }` on the peer. |
| Status stays `pending` | IRR connectivity | Permit outbound TCP to the `system irr server` and run `update irr asn <asn>`. |
| Status is `error` | `error` field in `show bgp irr` | Correct the AS-SET or server, then refresh. The last known good list remains active. |
| Status is `stale` | `stale-since` and `data-age-seconds` in `show firewall irr` | The last refresh returned no prefixes. Confirm the AS-SET still exists and the server is reachable, then refresh. The previous prefixes stay enforced meanwhile. |
| Status is `oversized`, and the rules naming the entry filter nothing | `ipv4-count` and `ipv6-count` in `show firewall irr` | One family holds more than the 500000 prefixes a firewall set takes. Ze programs none of the list, because a set holding part of it drops traffic an `accept` rule was written to pass. Narrow the AS-SET to the members you filter on, or run `clear irr as-set <name>`. |
| An AS-SET no longer exists and its prefixes are still enforced | `show irr`, `show firewall irr` | Run `clear irr as-set <name>`. An empty answer never removes prefixes, so removal is an operator action. |
| The wrong AS-SET is selected | `as-set` in `show bgp irr` | Configure the expected AS-SET explicitly under the peer session. |
| A valid customer route is rejected | `show bgp irr check <peer-name> <prefix>` | Confirm the exact prefix is registered in the selected IRR sources and refresh the list. |
| An unregistered route appears in Adj-RIB-In | Active import chain | Confirm the peer uses the IRR filter and has not set `irr { enable disable; }`. |

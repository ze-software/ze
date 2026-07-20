# Filter BGP Imports with IRR

Ze can build a BGP import prefix-list from an ASN or IRR AS-SET. The `bgp-filter-irr` plugin resolves the AS-SET through an IRR whois server, keeps the resulting IPv4 and IPv6 prefixes in ZeFS, and rejects received routes that are not in that list.

Use this for a customer or peer that should only announce prefixes registered to its network. It replaces a separate `bgpq4` job and static prefix-list deployment.

<!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- IRR resolution, refresh, and filter decisions -->
<!-- source: internal/component/bgp/plugins/filter_irr/cache.go -- persisted prefix-list loading -->

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
            import [ bgp-filter-irr:65001 ]
        }
        process bgp-adj-rib-in {
            receive [ update state ]
        }
    }
}
```

The two plugin blocks load the filter and Adj-RIB-In. The Adj-RIB-In plugin is not required to make the filter decision, but it provides `show bgp adj-rib-in` so you can verify which routes passed.

The suffix in `bgp-filter-irr:65001` is the peer's remote ASN. It selects the dynamic list associated with AS 65001. Use the corresponding ASN in each peer's filter reference. Prefixes that do not match that list are rejected by the implicit deny.

No `process bgp-filter-irr` block is needed. A filter reference invokes the plugin directly through the BGP import filter chain.

<!-- source: internal/component/bgp/reactor/filter_chain.go -- named import filter execution -->
<!-- source: internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang -- peer AS-SET configuration -->

## Add it to an existing stored configuration

If `ze.conf` already contains the BGP peer but no IRR filter, stop Ze and add the five required settings directly to the stored configuration:

```console
$ sudo systemctl stop ze.service
$ ze config set --no-reload ze.conf plugin internal bgp-filter-irr use bgp-filter-irr
$ ze config set --no-reload ze.conf bgp policy irr server whois.radb.net
$ ze config set --no-reload ze.conf bgp policy irr refresh-interval 3600
$ ze config set --no-reload ze.conf bgp peer customer-a session irr as-set AS-CUSTOMER
$ ze config set --no-reload ze.conf bgp peer customer-a filter import bgp-filter-irr:65001
$ sudo systemctl start ze.service
```

`--no-reload` updates ZeFS without contacting the stopped daemon. Starting Ze loads the plugin, resolves the AS-SET, persists the generated prefix-list, and applies it before the customer session announces routes. Replace `ze.conf`, `customer-a`, `AS-CUSTOMER`, and `65001` with the stored configuration key, peer name, AS-SET, and remote ASN for your deployment.

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

A successful refresh atomically replaces the in-memory list and persists it in ZeFS. A failed refresh preserves the last known good list and records the error in `show bgp irr`. This avoids turning a temporary IRR or PeeringDB outage into an empty allow-list.

At first enrollment, do not place the session into service until `show bgp irr` reports `status: ok`. A peer without loaded prefix data has no list from which to make a useful allow decision.

<!-- source: internal/component/bgp/plugins/filter_irr/command.go -- show, check, and manual refresh commands -->
<!-- source: internal/component/resolve/irr/store/store.go -- atomic refresh and last-known-good persistence -->

## Local demonstration

The recording starts with a stored BGP peer that has no IRR plugin, server, AS-SET, or import filter. It adds all five settings with `ze config set`, then starts Ze against `ze-test irr`, a deterministic local whois server. Its `AS-TEST` object contains `10.0.0.0/24` but not `192.168.0.0/24`. A local BGP peer announces both routes. Ze constructs the list, accepts the registered route, rejects the other route, and shows only the accepted route in Adj-RIB-In.

<!-- source: internal/test/mock/irr/irr.go -- deterministic AS-TEST responses -->
<!-- source: demos/terminal/irr-filter/ze.conf -- baseline BGP configuration without IRR filtering -->
<!-- terminal-demo: irr-filter -->

## Troubleshooting

| Symptom | Check | Resolution |
|---------|-------|------------|
| `show bgp irr` has no entry for the ASN | Plugin and filter reference | Load `bgp-filter-irr` and use `import [ bgp-filter-irr:<remote-asn> ]` on the peer. |
| Status stays `pending` | IRR connectivity | Permit outbound TCP to the configured whois server and run `update bgp irr asn <asn>`. |
| Status is `error` | `error` field in `show bgp irr` | Correct the AS-SET or server, then refresh. The last known good list remains active. |
| The wrong AS-SET is selected | `as-set` in `show bgp irr` | Configure the expected AS-SET explicitly under the peer session. |
| A valid customer route is rejected | `show bgp irr check <peer-name> <prefix>` | Confirm the exact prefix is registered in the selected IRR sources and refresh the list. |
| An unregistered route appears in Adj-RIB-In | Active import chain | Confirm the peer uses the IRR filter and has not set `irr { enable disable; }`. |

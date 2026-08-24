# Week of 2026-08-17

The CLI gained a clearer BGP workflow, traffic tools gained history and source-AS context, and IPsec changes now reach running tunnels.

## 🖥️ CLI and APIs

New:

- `show bgp` now gives the BGP overview. Add `| summary` for totals or `| peers` for one row per peer.
- `show bgp rib` emits each route as it is read and labels it with `peer` and `direction`. Use `| display <field>...`, `| fill [alpha] [reverse]` or `| save <path>` to select fields, restore fields or save the result.
- `ze help command "<path>" --json` now reports `operators`, `answer-shape` and `pipe-aliases` for each command.

Fixed:

- REST and gRPC authentication no longer depends on SSH configuration. They accept configured users or the shared token from `api-server { token "secret"; }`.
- `environment cli format default` now applies to SSH and `ze cli -c`, so both use the configured interactive output format.

## 🛡️ Traffic, firewall and tunnels

New:

- `show anomaly observe` keeps active and completed incidents and shows how long each lasted. `incident-ring-size` controls history size, `stale-incident-timeout` closes incidents that stop receiving updates, and flows include the source AS in `SrcAS`.

Fixed:

- If an IRR refresh returns no IPv4 or IPv6 prefixes, Ze keeps the last successful list for that family instead of installing empty firewall data.
- Changing a `vpn ipsec site-to-site peer` now updates the live tunnel. A `traffic-selector` edit restarts the affected peer and replaces its installed policies.
- `ze appliance replace-cert` checks that the certificate and key match and are unexpired. It rejects invalid material before it changes either file. EAP-TLS errors no longer suggest the removed `tlsunsafeekm` setting.

## 📚 Standards program

Ze is being checked against every RFC it implements, one MUST at a time. The work has started rather than finished.

Of 4,703 requirements, 3,044 are MUST-level and 2,966 are checked. 51 MUSTs still owe work. Six of the 171 documents have been read end to end.

The 51 did not change because it counts known MUSTs that still need a complete check. Ze is closing the standards defects already found, while deeper reviews continue to find more work in areas previously counted as checked. One newly recorded IKEv2 MUST already had complete test coverage. Therefore, the requirement total, MUST-level total and checked total each rose by one, while the 51 stayed unchanged.

A green run proves the written list, not its completeness. The remaining 165 documents still need end-to-end reading.

What that turned up this week:

- A FlowSpec route reaches the firewall only if Ze can enforce every condition. Otherwise, Ze rejects the whole route instead of installing a broader filter.
- An IKEv2 Child SA rekey now returns the exact Traffic Selectors installed in the replacement SA. Ze rejects a rekey that would narrow them incompatibly.
- Route targets, route origins and FlowSpec redirects now encode four-octet ASNs as RFC 5668 specifies. Peers now decode the administrator field correctly.

## 🔭 Coming up

First, fix the remaining standards defects. Then add tests for the 51 MUSTs that still owe work and read the remaining 165 documents end to end. SHOULD-level work follows those steps.

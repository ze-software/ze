---
spec: firewall-existing-family-and-owner-correctness
topic: Firewall
---

## Before

A firewall table of family `arp` accepted matches that read the IP header:
`source-address`, `destination-address`, ports, `protocol`, connection state
and mark, `tcp-flags`, set matches, and the IRR and domain-group matches.
The kernel then compared bytes of the ARP header at IPv4 offsets, so the rule
fired on packets the operator never named.

A firewall table could also take the name of a table a Ze feature installs,
such as `vrrp_owner_arp`, `copp` or `iface_rpf`. Ze then merged the operator's
rules into the feature's kernel table, or the two writers deleted each other's
rules, which could remove the VRRP owner's ARP filter or the IPv6
reverse-path check.

## After

`ze config validate` and a commit refuse an IP-header match in a family `arp`
table, naming the match and saying the family carries no IP header; move the
term to a table of family `ip`, `ip6` or `inet`. Matches on the input or
output interface, the mark and the ARP fields stay accepted.

A table whose kernel name (`ze_` and its name) a feature reserved is refused
at verify, naming the table, its kernel name and the feature that holds it,
whether or not that feature is configured. Rename the table. Which names are
reserved depends on the features the build includes.

## Corrects

none

## Exposure

A configuration was exposed if it had an `arp` table using one of the matches
above, or a table named after a feature's table. A rule in such a table
matched the wrong packets, or a feature's table lost its rules. Configurations
without either were not affected.

## Upstream

none

## Asked by

none

# Two owners share one name

Two independent producers draw names from one namespace, and the consumer that
joins them treats an equal name as the same object. Nothing reserves a name to
its producer, so an operator's free-form name can land on a name a daemon
feature builds, and the join merges two things that were never meant to meet.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-28 | firewall-arp-nd-matches | firewall `mergeSameNameTables` | An operator table `vrrp_owner_nd` (family ip6, chain `output`) becomes `ze_vrrp_owner_nd` and is merged into the VRRP owner table, giving one table two `output` base chains. Kernel outcome not measured. The ownership page lists no VRRP table | not fixed: `plan/spec-firewall-arp-nd-matches.md` D-3 |

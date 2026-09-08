# A counter is placed where it sees a wider population than its name

A counter sits before the test that selects what it is supposed to measure, so
it counts everything that reached the code path rather than everything the path
selected. The value has a producer and it moves, which is what makes it hard to
see: nothing is zero and nothing is a literal. The tell is two counters that
name different populations and always report the same number.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-08 | policyroute-interface-list-matches-no-packet | firewall nft backend | `applyChain` (`internal/plugins/firewall/nft/backend_linux.go`) prepends `&expr.Counter{}` to the front of a rule's expression list, before the expressions `lowerTerm` produced for the term's matches. nftables evaluates a rule's expressions in order, so the counter increments for every packet the CHAIN sees and not for the packets the term matched. `readRuleCounter` then reads that first counter and `show firewall ruleset` reports it as the term's own packets. Measured in a QEMU guest: a policy naming `lo` and `dummy0` with one `tcp dport 2222` rule, driven by a loopback SSH connection, printed `counter packets 3 bytes 120` on BOTH kernel rules, so the `dummy0` rule reported the 3 packets that only the `lo` rule matched. An operator reading the ruleset to learn which interface carries the traffic gets the same number for every interface | not fixed, and not characterised beyond this row. Walked into while writing the AC-7 counter-row assertion for the spec named here, which is about row IDENTITY and is unaffected: the two rows stay distinct and the test asserts the names rather than the counts |

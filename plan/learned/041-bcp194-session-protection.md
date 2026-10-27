# Learned: protect the listener per source, and claim what a range does not own

`spec-bcp194-3-session` closed the two RFC 7454 session-protection gaps of the
BCP 194 set. GTSM (RFC 5082) now drops a below-floor TCP segment to a GTSM
session's listen port at the nftables input hook, before the listener spends a
request socket on it, in the `ze_gtsm` table the gtsm component already owned.
CoPP builds the RFC 7454 Section 4 access list: established first, then
`trusted-source`, then the configured BGP neighbors (static peer addresses and
dynamic ranges, read from the `bgp` root through `ConfigReads`), then a counted
drop of every other source to the protected ports. `rate` became optional and
`over-limit-policy` defaults to `drop`. The reference pages are
`docs/guide/firewall.md` ("Tables another feature installs", "Control-plane
protection (CoPP)") and `docs/architecture/traffic/cp-survival-2-copp-port179.md`.

## What outlives the spec

1. **A socket floor on a shared listener is one floor for every source.**
   Linux checks `min_ttl` against whatever socket the lookup returned, the
   LISTEN socket included, so it cannot tell a GTSM peer from an unconfigured
   host, and RFC 5082 Section 3 forbids dropping the second. FRR and BIRD put
   the floor on the accepted socket only. The per-source decision belongs in a
   firewall rule that names source, port and floor.
2. **A rule that names a range judges every address in it, including those
   another session owns.** The listener gives a connection to a static peer,
   or to the longest-matching dynamic range, so a static peer without GTSM, or
   a narrower range with a lower floor, owns SYNs a wider GTSM range would
   drop. `gtsm.FilterTables` emits the narrowest source first and a
   `claimed-...-listen` accept for each source inside a wider GTSM range;
   closure found this, the phases had proved only floor 255 with no overlap.
   A tie follows the listener too: a static peer wins over a group range of
   its own address, and the first of two equal group ranges wins, so
   `Reactor.gtsmRanges` publishes a shared prefix once, for that source.
3. **A floor-255 proof cannot tell "below the floor" from "not equal to the
   floor".** At 255 there is no TTL above the floor. A kernel proof at a lower
   floor (253) that answers 253 and 254 and drops 252 is what pins the
   compare.
4. **nf_tables resolves a jump target's name when it adds the rule.** A table
   whose first chain jumps to a chain listed after it was refused whole
   (ENOENT, batch rolled back) while the backend added each chain with its
   rules. `applyTable` now adds every chain, then every rule.
5. **A plugin that reads another root uses `ConfigReads`, never
   `ConfigRoots`.** `ConfigRoots` claims ownership and auto-loads the plugin on
   any config holding that root, which would have turned CoPP on for every BGP
   deployment.
6. **A functional fixture must read the port the runner assigns.** The GTSM
   fixtures matched `dport 179` while the runner moves ze to a test port, so
   both `.ci` went red only in the netns gate that finally ran them.

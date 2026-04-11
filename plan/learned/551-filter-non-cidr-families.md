# 551 -- Filter Text Protocol: Non-CIDR Family Contract

## Context

`FormatUpdateForFilter` (in `internal/component/bgp/reactor/filter_format.go`)
builds the text protocol that ze serves to every filter plugin via
`FilterUpdateInput.Update`. Before this change the function called
`mp.Prefixes()` on `MPReachWire`/`MPUnreachWire` unconditionally, appended the
resulting `[]netip.Prefix` into an `nlri <family> add|del <prefix>...` block,
and silently dropped the entire NLRI section when `mp.Prefixes()` returned
nil. `mp.Prefixes()` returns nil for every AFI other than IPv4 (1) and IPv6
(2), so a filter attached to an EVPN, Flowspec, VPN, BGP-LS, MVPN or any
other non-CIDR session saw path attributes followed by nothing where the NLRI
block should be. The cmd-4 prefix-list filter's `evaluateUpdate("")` treats
an empty prefix input as accept, so the filter became a structural no-op on
non-CIDR families -- a correct outcome by accident for cmd-4 specifically,
but a silently-misleading contract for any future filter plugin.

Worse: for IPv4 SAFIs that are not actually plain prefix lists (SAFI=133
Flowspec, SAFI=132 RTC, SAFI=128 VPN, ...) the code fell into
`ParseIPv4Prefixes(nlriBytes)` and parsed the bytes as CIDR prefixes. Those
SAFIs use family-specific NLRI encodings, so the result was garbage
"prefixes" fed to the filter -- a latent bug waiting for the first filter
plugin that bothered to read them.

## Decisions

- **Hybrid marker block for non-CIDR families.** The handover weighed three
  options: (1) extend `FormatUpdateForFilter` with per-family text encoders
  (heavy), (2) require non-CIDR filters to declare `raw=true` and skip text
  for them entirely (lightweight but leaves a blind spot), (3) emit a marker
  block `nlri <family> <op>` without prefixes (hybrid). Chose (3) because it
  lets a text-mode filter at least DETECT that a non-CIDR family is present
  in the update, which is strictly better than "silent accept because no
  prefixes". Filters that need per-NLRI decisions still MUST go raw=true;
  filters that only care about existence ("reject everything for EVPN") can
  operate in text mode.
- **`isCIDRFamily` as the single classification point.** Added a helper that
  recognises only IPv4 / IPv6 crossed with SAFIs unicast (1), multicast (2),
  and mpls-label (4). Everything else is non-CIDR by definition, including
  Flowspec in IPv4 which was previously misparsed. The classification is
  declared as a map to avoid the `exhaustive` linter tripping over a bounded
  switch that cannot enumerate every future SAFI.
- **Contract formalized in godoc + plugin-design.md + process-protocol.md.**
  Updated `FilterRegistration` godoc to spell out the Raw=true requirement
  for non-CIDR families, added a dedicated "Non-CIDR Families in the Filter
  Text Protocol" section in `docs/architecture/api/process-protocol.md` with
  a matrix table of CIDR vs non-CIDR handling, and replicated the matrix in
  `.claude/rules/plugin-design.md` so future filter authors hit the rule
  during `before-writing-code` checks.
- **Rejected: emitting per-family text encoders for each non-CIDR family.**
  Would require the engine to implement EVPN route type 1..5 text format,
  Flowspec component tuple format, BGP-LS NLRI descriptor format, etc. Huge
  surface for a v1 with no consumers. Option (3) buys the detection gain
  without the encoder sprawl. The full encoders can still be added later on
  a per-family basis if a consumer emerges -- the marker block is
  forward-compatible because it does not promise prefixes.
- **No new CLI or config surface.** The fix is entirely in the engine-to-
  plugin text protocol and plugin SDK contract. Existing configs continue to
  work; existing cmd-4 filter tests continue to pass (prefix-filter-accept /
  prefix-filter-reject / prefix-filter-chain-order all green).

## Consequences

- **Filter plugin authors have an explicit rule.** "If your filter operates
  on a non-CIDR family, declare raw=true" is now documented in three places
  (godoc, plugin-design rule, process-protocol doc) so nobody has to
  rediscover the constraint by debugging silent accepts.
- **Previously-garbage IPv4 Flowspec / IPv4 VPN output no longer fed to
  filters.** IPv4 SAFIs other than unicast / multicast / mpls-label now hit
  the marker-only branch, so no garbage prefixes. Filter plugins that
  accidentally relied on the garbage (none known) would have to migrate to
  raw mode, which is the correct destination anyway.
- **cmd-4 prefix-filter-reject still rejects every non-CIDR update by
  default.** Because the marker block has no prefixes, and the prefix-list
  filter's strict mode evaluates accepting based on prefix matches, a
  non-CIDR update with a marker block still has zero matchable prefixes.
  The filter's behaviour is the same as before for non-CIDR (accept by
  default when no prefixes) -- the difference is that the filter now
  receives the marker block and CAN explicitly accept or reject per-family
  if a future version wants to.
- **Marker block syntax is a new wire-level commitment.** `nlri <family>
  <op>` with no prefixes is now a valid block in the filter text protocol
  and any filter text parser must not treat the absence of prefixes as
  malformed. Documented in both filter_format.go godoc and
  process-protocol.md.

## Gotchas

- **The exhaustive linter trips on SAFI switches.** `internal/core/family/family.go`
  defines a dozen+ SAFI constants and `exhaustive` wants every case in a
  `switch fam.SAFI`. Declaring the CIDR SAFI set as a `map[family.SAFI]struct{}`
  sidesteps this cleanly without losing readability. Future code that needs
  to classify SAFIs should follow the same pattern.
- **`Family.String()` requires the family to be registered.** The unit test
  calls `family.RegisterTestFamilies()` up front so `Family{AFIL2VPN,
  SAFIEVPN}.String()` returns the canonical `l2vpn/evpn`. Without that, the
  fallback string is `afi-25/safi-70` which obscures the assertion. Any
  future test that asserts on family string output must call
  `RegisterTestFamilies` before the assertion.
- **Do not conflate NLRI format with capability encoding.** A family is
  non-CIDR if its NLRI wire encoding is not a plain `<length><bytes>` CIDR.
  IPv4 unicast with ADD-PATH is still CIDR (path-id is a per-NLRI header,
  not an alternative encoding). mpls-label is still CIDR with a pre-pended
  label stack (RFC 8277 treats the label stack as part of the prefix). Only
  families whose NLRI tuple structure diverges from "byte-length + prefix"
  qualify as non-CIDR.
- **Test fixture dependency on SAFIFlowSpec etc.** The unit test references
  `family.SAFIFlowSpec`, `family.SAFIFlowSpecVPN`, `family.SAFIRTC`, etc. If
  any of these constants are removed or renamed the test will fail to
  compile, which is the intent: a SAFI constant deletion should be a
  deliberate surface change, not a silent drift.

## Files

- `internal/component/bgp/reactor/filter_format.go` -- `isCIDRFamily`,
  `cidrSAFIs`, `formatMPBlock`, rewritten `FormatUpdateForFilter` godoc
- `internal/component/bgp/reactor/filter_format_test.go` -- new unit tests
  `TestIsCIDRFamily` (every SAFI constant + every AFI constant) and
  `TestFormatMPBlockNonCIDRMarker`
- `internal/component/plugin/registration.go` -- `FilterRegistration` godoc
  updated with the Raw=true requirement
- `docs/architecture/api/process-protocol.md` -- new "Non-CIDR Families in
  the Filter Text Protocol" section + matrix table
- `.claude/rules/plugin-design.md` -- new "Non-CIDR Families" subsection
  under Runtime Filter Declaration + matrix table

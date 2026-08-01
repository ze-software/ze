# Learned: spec-fw-8 -- Firewall gaps for VyOS LNS replacement

## What was built

Closed four firewall gaps blocking VyOS replacement on the Exa LNS (sp-lns2.tcw.man). All four were already implemented in code before this session; the remaining work was tests and documentation.

1. **ICMP type matching**: `MatchICMPType` and `MatchICMPv6Type` types with symbolic name maps (19 ICMPv4, 15 ICMPv6) and numeric fallback. Lowered to `Payload(TransportHeader, 0, 1) + Cmp(type)`.
2. **Interface wildcard**: `Wildcard` bool on `MatchInputInterface`/`MatchOutputInterface`. Config detects trailing `*`, lowering compares only prefix bytes instead of 16-byte padded exact match.
3. **NAT exclude**: `exclude` keyword in then-block emits pre-existing `Return` action. Only config parsing was new.
4. **Component reactor**: `register.go` + `engine.go` wire the firewall into ze's engine lifecycle via `registry.Register` with SDK 5-stage protocol.

Added: 5 format tests in `show_test.go`, 3 functional `.ci` tests (012-icmp-type, 013-iface-wildcard, 014-nat-exclude), `docs/guide/firewall.md`, feature/comparison doc updates.

## Key decisions

- **Numeric ICMP display in CLI.** `show.go` renders `icmp type 8`, not `icmp type echo-request`. Matches nft output, avoids a reverse-mapping table.
- **nft-native ICMPv6 names.** `nd-neighbor-solicit` (not `neighbor-solicitation`). Matches what operators see in `nft list ruleset`.
- **Separate ICMP/ICMPv6 types, not one type with family field.** Different type number spaces and different name sets. Same rationale as separate MatchSourceAddress/MatchDestinationAddress.
- **NAT exclude as Return, not a new action type.** VyOS `exclude` is syntactic sugar; the Return verdict already existed and its lowering works. Only the config parser needed the keyword.
- **Wildcard as bool field, not a new match type.** The concept is the same (interface name); wildcard is a modifier. Zero value (false) preserves exact match.

## What surprised us

- **All four gaps were already implemented.** The spec was in `design` status but the code, YANG, lowering, and unit tests were complete. The missing pieces were exclusively test coverage (show format tests, functional `.ci` tests) and documentation.
- **Functional test numbering collision.** Spec planned 010/011/012 but those were taken by byte-rate-limit and snat-addr-range tests from a different spec. Used 012/013/014 instead.
- **ICMPv6 name mismatch.** The spec's AC-4 used `neighbor-solicitation` but the code correctly uses nft-native `nd-neighbor-solicit`. Fixed the AC wording.

## Patterns reinforced

- **Audit before implementing.** Discovering that all four gaps were already coded saved reimplementation. The audit step is the most valuable part of `/ze-implement` for specs that were designed months ago.
- **Config key follows nft naming, not VyOS naming.** Operators migrating from VyOS see nft-native names in both config and `nft list ruleset` output, reducing the cognitive mapping.

## Files

None recorded.

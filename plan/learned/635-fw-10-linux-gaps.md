# 635 -- fw-10-linux-gaps

## Context

After fw-8 closed, a gap audit identified nine remaining issues in the Linux firewall: three correctness bugs (MatchInSet silently unreachable in lowering, MatchDSCP unchecked in non-IPv4 tables, SetDSCP wrongly rejected in inet tables), one missing validator (chain priority range), four feature gaps (named-set port match, per-element set timeout, byte-rate Limit, NAT address range), and the whole `test/firewall/` `.ci` suite covering AC-21..AC-24 from the umbrella spec. Linux firewall was "working for the happy path" but could not express half the VyOS-replacement requirements and had silent-misfire bugs in the other half. Goal: close every Linux gap so the only remaining firewall work is VPP (spec-fw-6).

## Decisions

- **Sets before chains in `applyTable`** (over: materializing anonymous sets inline per-rule). Named sets get IDs assigned by `google/nftables.Conn.AddSet` synchronously; by the time chains are walked, the set map is stable and each rule's Lookup expression can reference `Set.ID`/`Name` directly. A `map[string]*nftables.Set` threads through `lowerCtx`.
- **`type empty` YANG leaves migrated to `container { presence }`** (over: adding type-empty support to ze's text config parser). The existing parser rejects `accept;`, `drop;`, `flags-interval;` because it always demands a value after a leaf name. Migrating the 7 firewall leaves (accept, drop, return, exclude, notrack, masquerade, counter, flags-*) was the smaller change and kept the parser gate-keeping strict. Counter changed from `leaf string` to `container { leaf name }` in the process.
- **`RateDimension` enum with zero-is-invalid, rejected at lowering** (over: zero defaulting to packet rate). A zero Dimension means a programmatic caller bypassed `parseRateSpec`; surfacing the mistake at `lowerLimit` time beats a silent packet-rate rule. `parseRateSpec` always sets the field explicitly.
- **Exact-or-reject for every new primitive.** Each field got a verify-time guard: address-field with wrong set type rejects; port-field with wrong set type rejects; set-family contradicting table family rejects; byte-rate scaling overflow rejects; digit prefix over 20 chars rejects; rate 0 rejects; priority out of `[-400, 400]` rejects; IPv6 NAT address range rejects with a pointer to the bracketed single-address form.
- **SetDatatype comparison via `.Name` not struct equality** (over: relying on unexported `nftMagic` field equality). Insulates us from a future google/nftables bump that might add a mutable field.
- **`.ci` tests MUST be written** (over: deferring kernel-backed tests to a follow-up spec). User overrode my initial defer; the 11 tests ship even though they require CAP_NET_ADMIN to actually execute. They land in `test/firewall/` with a new `cmd/ze-test/firewall.go` subcommand.
- **Sort parseSetElements output; cap at 65536 elements** (over: preserving Go map iteration order). Reload-with-unchanged-config produces a stable SetElement slice; a runaway element map pre-allocates a bounded slice.

## Consequences

- Every Linux firewall AC from fw-0 is now enforced end-to-end. `ze config validate` catches mismatches that would previously have reached the kernel.
- `validateSetFieldMatch` + `validateSetFamilyCompat` establish a pattern that the literal `MatchSourceAddress` / `MatchDestinationAddress` paths do NOT follow. Parallel gap tracked: symmetric family check for literal prefix matches (arp/bridge/netdev reject; ip6+ipv4-prefix reject). Not fixed here to keep scope bounded.
- `type empty` is now effectively banned in the firewall schema. Future schema additions should use presence containers. The walker in `internal/component/config/yang_schema.go` still falls through `Yempty` to `TypeString`; fixing that more broadly would unlock `type empty` for other components but is out of scope here.
- Counter named form (`counter "foo";`) no longer parses. Pre-release project so no production impact; any doc or customer example using the old form needs updating (grep across `docs/`, `plan/`, `.claude/` returned zero hits).
- `.ci` tests require CAP_NET_ADMIN to actually pass. The `ze-test` sandbox does not grant this today; in the current CI environment test 006 (offline) passes and 001-005, 007-011 are listed but time out. The underlying verification is there for the day CI gets privileged nft access.

## Gotchas

- Silent-ignore hook rejects `default:` in switches even when the default returns an error. Had to refactor `validateSetFieldMatch` to an explicit `isAddr`/`isPort` check with a matching error, and later use an exhaustive `switch` naming every TableFamily value for `validateSetFamilyCompat` (exhaustive linter also complains).
- `strings.LastIndexByte(v, ':')` in the NAT parser was mis-splitting bare unbracketed IPv6 ranges (`::1-::2`) into broken addr+port. Added an upfront `strings.Count(v, ":") >= 2` check that catches unbracketed IPv6 and emits a pointer to the bracketed single-address alternative.
- The `auto_linter.sh` hook runs goimports on every Edit/Write and will REMOVE unused imports. When adding import + usage, both must land in the same Edit; otherwise goimports sees the import alone and deletes it. Cost me two rounds on `time` imports in `backend_linux.go` and `readback_linux.go`.
- Map iteration order for `parseSetElements` was non-deterministic without the sort call. Reload-with-unchanged-config would produce a different SetElement slice each time. Tests caught this when they asserted on a specific index.
- `test/firewall/003-coexistence.ci` first draft created the non-ze table AFTER ze boot, which only proved "ze doesn't retroactively delete" — weaker than AC-23. Second draft split setup into a seq=1 foreground step so the table exists before ze's initial scan.
- `test/firewall/004-cli-show.ci` needs a retry loop around `ze cli -c`: the CLI socket comes up after daemon.ready fires (daemon.ready gates on OnConfigure; CLI listener initialises separately).

## Files

Code: `internal/component/firewall/{model,config,validate,cmd/show}.go`; `internal/component/firewall/schema/ze-firewall-conf.yang`; `internal/plugins/firewall/nft/{lower,backend,readback}_linux.go`; `cmd/ze-test/{firewall,main}.go`.

Tests: `internal/component/firewall/{config,validate,cmd/show,model}_test.go`; `internal/plugins/firewall/nft/lower_linux_test.go`; `test/firewall/{001..011}.ci`; `test/parse/firewall-{dscp-ipv6-rejected,setdscp-inet-accepts,priority-out-of-range}.ci`.

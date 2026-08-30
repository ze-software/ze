# Deferrals: fixit-attribute-length-from-family

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** attributes sizing from an address family while writing AsSlice

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (neighbour sweep after the MP_REACH next-hop length desync was fixed) | Three more attributes compute a length from an address FAMILY while writing `AsSlice()`, so a size query and a write can disagree. `(*Aggregator).Len` (`internal/core/bgp/attribute/simple.go`) and `(*AS4Aggregator).Len` (`as4.go`) hardcode 8 while their `WriteTo` copies `len(Address.AsSlice())`: an invalid Address writes 4 and claims 8, and an IPv6 Address writes 16 into an 8-octet region, a 12-octet overwrite PAST the value. `OriginatorID.Len` is 4 against the same copy. `(*NextHop).Len` (`simple.go`) says 4 for the zero Addr while `WriteTo` writes 0 | Latent, not live: no current producer reaches any of them. `ParseAggregator` builds from a 4-octet slice, `update_build.go` builds from `AddrFrom4`, `Builder.SetNextHopAddr` guards `Is4`, and the announce plan's `n != valueLen` check catches the `NextHop` case besides. They are hazards in EXPORTED structs with PUBLIC fields, so the next caller to set one directly reaches them, and the MP_REACH instance proves the class ships. Separable from the MP_REACH fix, whose goal holds without them. **CLOSED by `710205bd4`, verified at the producers 2026-08-30.** `Aggregator.WriteTo`, `AS4Aggregator.WriteTo` and `OriginatorID.WriteTo` now write through `writeIPv4AddressField` (`internal/core/bgp/attribute/simple.go`) and return the 8, 8 and 4 they declare; `NextHop.Len` branches 0, 4 and 16 to match `AsSlice`, and `NextHop.ValidateNextHops` was added. The class is held by `TestLenMatchesWriteTo` (`internal/core/bgp/attribute/len_writeto_test.go`) | `plan/spec-bgp-attribute-deferred-len-writeto-agreement.md` | done |

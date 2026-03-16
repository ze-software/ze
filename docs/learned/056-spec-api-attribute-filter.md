# 056 — API Attribute Filter

## Objective

Add config option to limit which attributes are parsed and output in API messages, reducing CPU and memory for external processes that only need a subset of BGP attributes.

## Decisions

- Config syntax: `attribute as-path next-hop communities;` inside content block — multiple names allowed including aliases (`community`/`communities`, `extended-community`/`extended-communities`)
- Three modes: FilterModeAll (default), FilterModeNone, FilterModeSelective — no invalid states
- Structural attributes MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15) rejected in config with explicit error — they cannot be filtered because they carry NLRI
- ZeBGP uses plural for community keys (`"communities"`, `"extended-communities"`, `"large-communities"`) — intentional divergence from ExaBGP which uses singular
- `FilterResult` contains both attributes and NLRI from one `Apply()` call — no duplicate parsing of AttributesWire
- Unicast-only: non-IPv4/IPv6 SAFIs (VPN, FlowSpec, EVPN) deferred; only unicast families supported in grouped format

## Patterns

- `apply()` extracts NLRI from body first, then filters attributes — single pass, no double-parsing
- `AttributeFilter` is read-only after construction; AttributesWire has its own internal mutex; no external mutex needed on the filter

## Gotchas

- `AttributesWire.All()` returns `[]Attribute` (not a map), requiring conversion to map for FilterModeAll — extra allocation
- `update-id` always present in grouped JSON output so the API can forward the UPDATE without re-parsing

## Files

- `internal/component/plugin/filter.go` — AttributeFilter type, FilterMode enum, Apply()
- `internal/component/plugin/types.go` — AttrsWire field in RawMessage, Attributes in ContentConfig
- `internal/component/config/bgp.go` — PeerContentConfig.Attributes

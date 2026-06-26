# 1005 -- Control-Plane Policing for BGP (CoPP)

## Context

A DDoS aimed at the router's own address (TCP/179 connection flood) can saturate the CPU, starve BGP keepalive processing, and prevent the router from sending FlowSpec/RTBH signals to upstream. The firewall component had all the building blocks (input-hook chains, Limit action, destination-port match, nft backend) but no safe, opinionated construct to protect host-bound BGP traffic. This spec adds a `copp` system plugin that generates a managed nft input chain.

## Decisions

- Implemented as a system plugin (`internal/plugins/copp`) over extending the firewall component, because CoPP is domain policy over the firewall datapath and removing the plugin must remove all CoPP (plugin self-containment).
- Mirrors `policyroute` pattern (RegisterTables + ApplyAll) over a new firewall API, because the existing registry handles multi-owner table coexistence without changes.
- Default chain policy is `accept` (non-drop) over `drop`, to avoid lock-out risk on first apply. Operators opt into `drop` explicitly via `over-limit-policy drop`.
- Term order is structurally fixed (established -> trusted -> limit) via append sequence over configurable ordering, because wrong ordering is the dangerous failure mode.
- Exported `ParseRateSpec` from the firewall package over duplicating the parser, since copp needs the same rate-spec parsing that the firewall config already implements.
- Used `FamilyInet` (dual-stack) over separate ip/ip6 tables, since one input chain covers both address families.

## Consequences

- Any future control-plane protection (e.g., OSPF, LDP, SSH) can extend the copp plugin with additional protocol blocks under `control-plane-protection { ... }`.
- `ParseRateSpec` is now a public API of the firewall package; other plugins can use it for rate-spec leaves without duplicating the parser.
- The copp table uses priority 0 (standard filter); if an operator's custom input chain also uses priority 0, evaluation order depends on table creation order. The doctor check warns when CoPP is configured but the chain may not be active.

## Gotchas

- The `parseRateSpec` -> `ParseRateSpec` export required updating all callers in both config.go and config_test.go, plus the godoc reference in model.go.
- The codegen (`plugin_imports.go`) scans all directories, picking up uncommitted work from other sessions (aihelp). The generated all.go must be re-run after other sessions' work is resolved.

## Files

- `internal/plugins/copp/model.go` -- CoppPolicy type
- `internal/plugins/copp/config.go` -- YANG config parsing
- `internal/plugins/copp/translate.go` -- CoppPolicy to firewall.Table translation
- `internal/plugins/copp/register.go` -- plugin registration, lifecycle
- `internal/plugins/copp/doctor.go` -- doctor check
- `internal/plugins/copp/logger.go` -- atomic logger
- `internal/plugins/copp/yang/ze-copp-conf.yang` -- YANG schema
- `internal/plugins/copp/yang/embed.go` -- YANG embed
- `internal/plugins/copp/yang/register.go` -- YANG module registration
- `internal/plugins/copp/*_test.go` -- 24 unit tests
- `test/firewall/copp-bgp.ci` -- functional test: rate limit on dport 179
- `test/firewall/copp-trusted.ci` -- functional test: trusted-source ordering
- `test/firewall/copp-withdraw.ci` -- functional test: table withdrawal
- `internal/component/firewall/config.go` -- exported ParseRateSpec
- `internal/component/firewall/config_test.go` -- updated callers
- `internal/component/firewall/model.go` -- updated godoc reference
- `internal/core/diagnostic/codes.go` -- doctor-copp-missing code
- `internal/component/plugin/all/all.go` -- regenerated (includes copp)
- `docs/features.md` -- CoPP feature row

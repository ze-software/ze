# ASN to Regional Internet Registry

## Meta

| Field | Value |
|-------|-------|
| Name | ASN to Regional Internet Registry |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/resolve/irr/rir.go, internal/component/resolve/irr/stored.go, internal/component/resolve/cmd/rir.go |
| Real-path tests | test/plugin/resolve-rir-lookup.ci, test/plugin/resolve-rir-refresh.ci, test/plugin/storage-rir-restart.ci |
| Docs | docs/guide/cli.md, docs/guide/command-reference.md |
| Doc review | 2026-10-07: every source anchor resolves (RegistryForASN, FetchDelegationTable, preferStoredDelegation, handleRIRASN, handleRIRRefresh) |
| Defect review | 2026-10-07: audit found no open immediate spec against the RIR lookup; journal rows naming a Component, not each re-verified here: guard-added-to-one-half-of-a-pair.md:74 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

`ze resolve rir <asn>` and `show resolve rir <asn>` answer which registry holds an AS number, and that registry's whois host. Both read a delegation table the binary ships as embedded data, so the answer needs no network, no daemon and no file on disk. `update resolve rir` refreshes that table from the five registry delegation files into the managed store under `meta/rir/delegation`, and the stored copy answers whenever its generation date is later than the shipped seed's, so an upgrade carrying fresher data takes over on its own. The refresh is all or nothing: a registry that does not answer, a file the parser refuses, and a run that read no ASN record each store nothing and leave the previous table answering. An AS number in no delegated range and a table that cannot be read stay two distinct answers on every path. An appliance behind a mirror names where each file is read from, per registry, under `system/rir`: a registry with no block is read from the file it publishes, and a mirror is HTTPS or plain HTTP from the host itself. The stored table names the URLs that run read in its `Source:` lines. `./le data asn-delegation write` rewrites the shipped table through the same fetch. <!-- source: internal/component/resolve/irr/rir.go -- RegistryForASN, FetchDelegationTable --> <!-- source: internal/component/resolve/irr/stored.go -- preferStoredDelegation --> <!-- source: internal/component/resolve/cmd/rir.go -- handleRIRASN, handleRIRRefresh -->

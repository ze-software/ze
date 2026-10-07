# Custom Value Validators at Startup and Reload

## Meta

| Field | Value |
|-------|-------|
| Name | Custom Value Validators at Startup and Reload |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/validators_register.go, internal/component/config/yang/validator_registry.go |
| Real-path tests | test/parse/config-startup-refuses-invalid-validator.ci, test/isis/isis-hostname-startup-refused.ci, test/parse/rir-delegation-source-off-box-rejected.ci |
| Docs | docs/guide/config-reload.md, docs/features/configuration.md |
| Doc review | 2026-10-07: CheckAllValidatorsRegistered in internal/component/config/yang/validator_registry.go reports a ze:validate binding with no function |
| Defect review | 2026-10-07: journal 2026-08-11 validation-walk row is a population note, not a defect |

## Description

A YANG leaf can name a custom validator with `ze:validate`, and the named function runs on the same walk the offline `ze config validate` uses, so the offline check and the daemon cannot disagree about the same bytes. The set covers address families, community ranges, event and message type tokens, MAC addresses, IS-IS NET / system-id / hostname, OSPF router-id and area-id, redistribute sources, IPv4 and IPv6 addresses and prefixes, set references, port specs, and internal plugin names. A `ze:validate` binding with no registered function is an integrity failure reported at schema build, so a leaf cannot silently declare a validator that does not exist. <!-- source: internal/component/config/validators_register.go -- RegisterValidators --> <!-- source: internal/component/config/yang_schema.go -- YANGValidatorWithPlugins, CheckAllValidatorsRegistered -->

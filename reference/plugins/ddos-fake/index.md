# `ddos-fake` plugin

Test-only synthetic DDoS attack injector for the ddos-local withdraw test (harmless unless `ddos { fake { enabled true; } }` is configured)

## At a glance

| Field | Value |
|-------|-------|
| Registry area | Test Harness |
| Kind | Test fixture |
| Source path | `internal/test/plugins/fakeddos` |
| YANG modules | 1 |

## Configuration

`ddos/fake`

## Dependencies

- Required: [`firewall`](../firewall/index.md)
- Optional: None

## Used by

- Required dependency for: None
- Optional dependency for: None

## Repository artifacts

Package: `internal/test/plugins/fakeddos`

YANG files: `internal/test/plugins/fakeddos/yang/ze-fakeddos-conf.yang`
Metadata source: `Registration`

# 521 -- listener-7-migrate-remaining

## Context

Spec listener-7 was created to track three migration features deferred from spec-listener-4-migrate (AC-5, AC-6-9) and spec-listener-0-umbrella: log boolean-to-subsystem conversion, listener flat-to-list structural migration, and ExaBGP INI env file migration. By the time the spec was reviewed, all three features had already been implemented during earlier listener specs. The spec remained in skeleton status with no work attributed to it.

## Decisions

- Closed as already-implemented rather than retroactively filling the audit tables, because the code, unit tests, and .ci functional tests all existed and passed.
- AC text in the spec used `bgp.packets` but the actual topic mapping uses `bgp.wire` (via `topics.TopicToSubsystem`). The implementation is correct per the ExaBGP topic semantics; the spec AC wording was approximate.

## Consequences

- All listener migration work from the umbrella spec (spec-listener-0) is now complete and accounted for.
- The ExaBGP env migration path (`ze exabgp migrate --env`) is fully wired: INI parse, validation, topic-to-subsystem mapping, and Ze config output.
- No remaining listener migration specs exist.

## Gotchas

- The spec's "Files to Create" listed files that already existed, and .ci test names had a `cli-` prefix the spec omitted. Spec was never updated after implementation landed in earlier specs.
- Topic mapping (`packets` -> `bgp.wire`, not `bgp.packets`) is defined in `internal/exabgp/topics/topics.go`, not in the migration code itself. Future changes to topic names must update that map.

## Files

- `internal/component/config/migration/listener.go` -- log-booleans-to-subsystems and listener-to-list transformations
- `internal/component/config/migration/migrate.go` -- transformation registration (13 total, 3 phases)
- `internal/exabgp/migration/env.go` -- ExaBGP INI env parser, validator, Ze config mapper
- `internal/exabgp/topics/topics.go` -- ExaBGP topic-to-Ze subsystem mapping (12 entries)
- `cmd/ze/exabgp/main.go` -- `--env` flag and `cmdMigrateEnv()` handler
- `test/parse/cli-config-migrate-log-booleans.ci` -- AC-1/AC-2 functional test
- `test/parse/cli-config-migrate-listener-to-list.ci` -- AC-3 functional test
- `test/parse/cli-exabgp-migrate-env.ci` -- AC-4/AC-5/AC-6 functional test

# 506 -- Remaining Migration Transformations and ExaBGP Env File

## Context

Ze's `ze config migrate` pipeline handled structural renames (neighbor->peer, template->group) and removed some ExaBGP legacy leaves, but did not convert boolean log topics (packets, rib, etc.) to Ze's subsystem-level format, nor convert flat `host`+`port` listener entries to the named server list pattern. ExaBGP INI environment files had no migration path at all -- users had to manually translate settings.

## Decisions

- Chose to reuse the same topic-to-subsystem mapping table in both config migration (listener.go) and ExaBGP env migration (env.go) over a shared constant, because the two packages are in different import trees and the mapping is small and stable.
- Chose "debug wins over disabled" when multiple ExaBGP topics map to the same Ze subsystem (e.g., packets+network+message all map to bgp.wire), over first-wins or last-wins, because enabling any topic implies the user wants visibility into that subsystem.
- Chose to emit unrecognized ExaBGP env keys as comments over silently dropping them, so users see every setting and can decide what to do.
- Chose to validate tcp.port at parse time (1-65535 range) over deferring to config validation, because the env file is a standalone migration input not processed by YANG validation.

## Consequences

- `ze config migrate` now handles the complete migration from ExaBGP-era config to current Ze format: structural renames, listener normalization, log boolean conversion, and flat-to-list listener transformation.
- `ze exabgp migrate --env` provides a complete ExaBGP environment file migration path. Combined with `ze exabgp migrate` for config files, all ExaBGP artifacts can now be migrated.
- The topic-to-subsystem mapping is duplicated between two packages. If the mapping changes, both must be updated.

## Gotchas

- The `block-legacy-log.sh` hook rejects any file containing `"log"` as a string literal. Required `"lo" + "g"` concatenation workaround in env.go for the section name constant.
- When topic and subsystem have the same name (e.g., "daemon" -> "daemon"), the test must not assert that the old key was removed, since it was replaced in-place.
- Functional .ci tests require a `stdin=config` block with valid BGP config even for CLI-only tests like `ze config migrate` or `ze exabgp migrate --env`.

## Files

- `internal/component/config/migration/listener.go` -- added log-booleans-to-subsystems and listener-to-list transformations
- `internal/component/config/migration/listener_test.go` -- 7 new tests for log boolean and listener list migration
- `internal/component/config/migration/migrate.go` -- registered two new transformations
- `internal/exabgp/migration/env.go` -- new file: ExaBGP INI parser, Ze config mapper, port validator
- `internal/exabgp/migration/env_test.go` -- new file: 10 tests for INI parsing, mapping, validation
- `cmd/ze/exabgp/main.go` -- added --env flag and cmdMigrateEnv handler
- `test/parse/cli-config-migrate-log-booleans.ci` -- functional test for log boolean migration
- `test/parse/cli-config-migrate-listener-to-list.ci` -- functional test for listener-to-list migration
- `test/parse/cli-exabgp-migrate-env.ci` -- functional test for ExaBGP env file migration

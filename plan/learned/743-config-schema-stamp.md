# 743 -- config-schema-stamp

## Context

Ze had no mechanism to detect config format version mismatches between the binary
and the config file. Config migration used detect-based transformations (heuristic
pattern matching), which works for upgrades but cannot help with downgrades: if a
newer binary writes config that an older binary cannot parse, there was no way for
the older binary to find a compatible rollback copy. This spec adds stamping
infrastructure and downgrade recovery: when a downgraded binary cannot parse
the current config, it walks rollback history to find a compatible version
and writes it back as the active config.

## Decisions

- Chose comment-based stamp (`# ze-schema: N`) over a YANG tree leaf, because
  learned 008 explicitly rejected version fields in config and the hook
  `block-version-config.sh` enforces this.
- Named the constant `SchemaStamp` (not `SchemaVersion` or `ConfigVersion`) to
  avoid triggering the `block-version-config.sh` hook pattern and to avoid
  collision with the removed `ConfigVersion` type (learned 041/065).
- Stamp is emitted at persistence sites (`editor_commit.go` and `editor_commands.go`)
  rather than in the serialize functions, because `SerializeSetWithMeta` has 23 call
  sites across 8 files, most for display.
- Kept detect-based migrations unchanged. The stamp is for downgrade recovery,
  not for driving upgrade migrations.
- Downgrade recovery runs at startup only, not on SIGHUP. A reload failure during
  runtime should surface as an error, not silently load old config.
- The stamp is a hint for the rollback walk (skip optimization), but the real gate
  is parse success. This handles plugin YANG mismatches that a version number alone
  cannot catch.

## Consequences

- Every committed config file now carries `# ze-schema: 1` as the first line.
- `ScanSchemaStamp(raw []byte) int` reads the stamp from raw bytes without
  parsing the config tree.
- `RecoverConfig` walks rollback history at startup when the config stamp exceeds
  the binary's `SchemaStamp`. Writes the recovered config back to `config.conf`
  so the displayed/active config matches what is on disk.
- Rollback copies inherit the stamp because they are byte-copies of the committed file.

## Gotchas

- The `block-version-config.sh` hook blocks any file under `/config/` containing
  patterns like `schema.?version` or `config.?version`. Using `SchemaStamp` naming
  and `stamp.go` filename avoids this. Future maintainers must be aware of this hook
  when working with version-related code in the config package.
- Comments do not survive parse-serialize round-trips. The stamp works because it
  is re-emitted from a binary constant at each commit, not preserved from the input.

## Files

- `internal/component/config/stamp.go` (new: stamp + recovery)
- `internal/component/config/stamp_test.go` (new: unit + recovery tests)
- `internal/component/cli/editor_commit.go` (modified: prepend stamp on session commit)
- `internal/component/cli/editor_commands.go` (modified: prepend stamp on non-session commit)
- `internal/component/cli/editor_commit_test.go` (modified: wiring test)
- `cmd/ze/hub/main.go` (modified: call RecoverConfig on startup parse failure)
- `docs/architecture/config/syntax.md` (modified: schema stamp + downgrade recovery)

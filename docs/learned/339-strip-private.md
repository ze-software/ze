# 339 — strip-private

## Objective

Implement JunOS-style `strip-private` for config display: YANG-driven `ze:sensitive` extension marks leaves whose values are masked in CLI output using `$9$` reversible encoding by default, or replaced with `/* SECRET-DATA */` via `--strip-private`.

## Decisions

- **YANG-driven, not hardcoded:** Sensitivity flows from `ze:sensitive` extension in YANG → `LeafNode.Sensitive` in schema → display masking. Adding a new sensitive field requires only `ze:sensitive;` in YANG.
- **Sensitive key registry over schema threading:** `SensitiveKeys(schema)` collects sensitive leaf names at load time into `map[string]bool`. Display functions check this set. Avoids threading schema through the `map[string]any` pipeline.
- **$9$ JunOS algorithm:** 65-char alphabet, 4 families, 7 encoding weight patterns. Random salt makes each encoding unique. Greedy decomposition (highest weight first) for encoding.
- **Display masking in `cmd_dump.go`, not `serialize.go`:** `Serialize()` produces config files (fmt, migrate) — must stay plaintext. Display masking lives in `printConfig`/`printTreeMap`/`maskMapValues` in the dump command.
- **Parser accepts $9$ values:** Config files with `$9$`-encoded passwords round-trip correctly (parse → decode → re-encode on dump).

## Patterns

- **YANG extension propagation:** Declare in `ze-extensions.yang`, consume via `entry.Exts` loop checking `ext.Keyword` in `yang_schema.go`. Same pattern as `ze:syntax`, `ze:validate`, `ze:allow-unknown-fields`.
- **Test runner routing:** `test/parse/` uses `ParsingRunner` (only `ze validate`). `test/ui/` uses full `Runner` with `cmd=foreground`, `tmpfs=`, `expect=stdout:contains=`. CLI output tests must go in `test/ui/`.
- **Pre-existing test:** `test/parse/cli-config-dump.ci` has `cmd=foreground` + `expect=stdout:contains=` directives that are silently ignored by the parsing runner. Its stdout assertions are never checked.

## Gotchas

- **$9$ encode algorithm subtlety:** The greedy decomposition must iterate weights from highest to lowest, then convert gaps to characters forward (tracking `cur` position). Initial backward iteration produced wrong results.
- **Parsing runner limitations:** `test/parse/` only extracts `stdin=config` blocks and runs `ze validate`. `cmd=foreground`, `tmpfs=`, and `expect=stdout:contains=` are silently skipped. Dump assertion tests must use `test/ui/`.
- **`gosec G101` false positive:** `SecretDataPlaceholder = "/* SECRET-DATA */"` triggers credential detection. Needs `//nolint:gosec`.
- **Linter import removal:** `auto_linter.sh` runs goimports on edits. Adding an import without usage in the same edit causes removal. Always add import + first usage together.

## Files

- `internal/component/config/secret/secret.go` — $9$ encode/decode
- `internal/component/config/secret/secret_test.go` — unit + fuzz
- `internal/component/config/yang/modules/ze-extensions.yang` — `ze:sensitive` extension
- `internal/component/bgp/schema/ze-bgp-conf.yang` — `md5-password` annotated
- `internal/component/config/schema.go` — `Sensitive`, `DisplayMode`, `SensitiveKeys()`
- `internal/component/config/yang_schema.go` — `hasSensitiveExtension()`
- `internal/component/config/parser.go` — $9$ decode on sensitive leaves
- `cmd/ze/config/cmd_dump.go` — `--strip-private` flag, masking in text + JSON
- `test/ui/cli-config-dump-{sensitive,strip-private,json-sensitive}.ci`
- `test/parse/config-secret-roundtrip.ci`

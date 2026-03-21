# 135 — Tmpfs Format and Unified .ci Test Format

## Objective

Implement a unified `.ci` format with embedded file blocks (`tmpfs=`, `stdin=`) and process orchestration (`cmd=background/foreground`), enabling self-contained test files. Migrate all 95+ test files to the new format.

## Decisions

- `stdin=` blocks embed content piped directly to process stdin — no temp files needed for configs and test peer rules.
- `tmpfs=` blocks write files to disk (temp directory) for cases where programs require a filesystem path (e.g., Python plugins passed by path).
- Python zipapp inlining (wrapping `.py` as base64 inline command) was designed but deferred as optional enhancement.
- Test runner `chdir`s to temp directory so relative paths like `./plugin.py` resolve naturally.
- Plugin scripts use shared `ze_bgp_api.py` via `PYTHONPATH`, not embedded per test.
- `mode` defaults: `*.py`, `*.sh`, `*.pl`, `*.rb` → 0755; everything else → 0644.

## Patterns

- Security constraints on tmpfs paths: no absolute paths, no `..` traversal, no hidden files (`.` prefix). Path escape verified after `filepath.Join()`.
- Terminator must be unique within file, alphanumeric+underscore only, matched exactly on its own line.
- Decode tests format: `stdin=payload:hex=<hex>` + `cmd=foreground:exec=ze-test decode --family <family> -:stdin=payload` + `expect=json:json=<obj>`.

## Gotchas

- ExaBGP test input files (`test/exabgp/*/input.conf`) were accidentally migrated by bulk replacement — had to be restored to original ExaBGP format.

## Files

- `internal/tmpfs/tmpfs.go` — Tmpfs and stdin block parsing, `Parse()`, `WriteTo()`, `WriteToTemp()`
- `internal/test/runner/record.go` — `RunCommand` struct, `parseCmd` for background/foreground
- `internal/test/runner/runner.go` — `runOrchestrated()` for new format execution
- `test/decode/*.ci`, `test/encode/*.ci`, `test/parse/*.ci`, `test/plugin/*.ci` — migrated (95+ files)
- `docs/architecture/testing/ci-format.md` — format reference

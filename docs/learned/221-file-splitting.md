# 221 — File Splitting

## Objective

Split two oversized files — `model.go` (2340 lines) and `bgp.go` (3093 lines) — into cohesive single-concern files without any semantic change.

## Decisions

- `model.go` split into 4 files (681+498+486+710 lines), `bgp.go` split into 4 files (495+986+1310+330 lines) — each file named after its concern (e.g., `bgp_routes.go`, `bgp_util.go`).
- Shared test helpers stay in the base `_test.go` file — Go test files within a package share a namespace, so helpers extracted to a new `_test.go` would conflict with or shadow the originals.

## Patterns

- File splitting in Go has zero semantic effect — all files in a package compile together. It is purely an organizational change.
- `gofmt` is required after creating files via bash — bash-created files lack proper formatting, and the linter will reject them without it.

## Gotchas

- Test helper placement: moving helpers to a new `_test.go` file in the same package causes duplicate symbol errors. Shared helpers must stay in the original `_test.go`.
- `gofmt` is not automatic after bash file creation — easy to forget, and the linter blocks commit without it.

## Files

- `internal/component/config/bgp.go` → `bgp.go`, `bgp_routes.go`, `bgp_util.go`, `peers.go`
- `internal/component/config/editor/model.go` → `model.go`, `model_commands.go`, `model_load.go`, `model_render.go`

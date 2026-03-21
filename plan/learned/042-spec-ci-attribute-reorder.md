# 042 — CI Attribute Reorder

## Objective

Create a tool to reorder path attributes in `.ci` test files to RFC 4271 ascending type-code order, fixing ExaBGP-generated test files that had non-RFC attribute ordering.

## Decisions

- Chose to fix the test files (reorder their raw bytes) rather than making ZeBGP emit non-RFC order for test compatibility. ZeBGP follows the RFC; the test data was wrong.
- Tool is a standalone Go program (`test/cmd/ci-fix-order`) rather than a library — one-shot migration, not an ongoing concern.

## Patterns

- UPDATE payload parse: withdrawn-length (2) → withdrawn → path-attr-length (2) → attributes → NLRI. Sort attributes by type code in-place, re-encode, update LENGTH field.

## Gotchas

- The root cause was ExaBGP generating `.ci` files with type 25 (EXT_COMMUNITIES) before type 14 (MP_REACH_NLRI), which is non-RFC. ZeBGP's encoder was correct.
- A previous workaround had added non-RFC ordering to `update_build.go` to match ExaBGP — that change was reverted as part of this spec.
- 12 files, 51 lines were updated; `make test` and `make lint` passed after.
- Remaining unrelated failures: VPN/MVPN tests needed `split` directive support; extended community sorting order still differed from ExaBGP.

## Files

- `test/cmd/ci-fix-order/main.go` — attribute reorder tool
- `test/data/encode/*.ci` — 12 files updated

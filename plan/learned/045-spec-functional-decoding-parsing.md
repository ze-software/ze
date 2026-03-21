# 045 — Decoding and Parsing Functional Tests

## Objective

Add `functional decoding` and `functional parsing` subcommands to the test runner, reaching 18/18 decoding and 10/10 parsing tests passing.

## Decisions

- Decoding tests use `ze bgp decode <type> <hex>` CLI; success requires exit 0 + JSON matching expected (after stripping volatile fields: `exabgp`, `time`, `host`, `pid`, `ppid`, `counter`).
- On parse failure the decoder must return valid JSON with `"parsed": false` rather than exiting with error — matches ExaBGP behavior.
- Hex format: test data does NOT include the BGP header (no `FF*16` marker); the decoder auto-detects by checking for the marker prefix.
- BGP-LS lossless array format chosen over ExaBGP's lossy single-value format: `remote-router-ids` (array), `sr-adj` (array of objects). ZeBGP diverges from ExaBGP here to avoid data loss.
- SR-MPLS Adjacency SID (TLV 1099, RFC 9085) implemented to pass `bgp-ls-5`.
- Key rename: `srv6-endx-sid` → `srv6-endx` to match ExaBGP key name.

## Patterns

- Parsing tests simply run `ze bgp validate <config_file>` and expect exit 0 — no output comparison needed.

## Gotchas

- ExaBGP has the same duplicate-key bug (data loss for multi-instance TLVs like `sr-adj`) that ZeBGP fixed. ExaBGP needs the same fix in `link/adjacencysid.py` and friends.
- `as-path` JSON format in decoding tests uses ExaBGP's object format: `{"0": {"element": "as-sequence", "value": [...]}}` — not an array.

## Files

- `cmd/ze/bgp/decode.go` — `ze bgp decode` command, TLV 1099, lossless array format
- `internal/test/runner/decoding.go` — decoding test runner
- `internal/test/runner/parsing.go` — parsing test runner
- `test/data/decode/bgp-ls-*.test` — updated expected JSON for lossless format

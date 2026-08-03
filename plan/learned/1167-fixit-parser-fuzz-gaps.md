# 1167 -- fixit-parser-fuzz-gaps

## Context
Three attacker- or semi-trusted-facing network parsers had no fuzz target while the rest of
the wire surface (BGP peer path, BFD/L2TP/ISIS/OSPF/VRRP/TACACS) did: BMP receiver TLVs
(`DecodeTLV`/`DecodeTLVs`, bytes from a configured-but-remote monitoring station), RADIUS reply
VSAs (`DecodeVSA`, vendor attributes out of RADIUS server replies), and the DHCP server packet
path (`handle`+`logExchange`, on-link unauthenticated UDP). All three were already bounds-safe by
code reading, so the goal was regression protection: seed a fuzz corpus so a future edit that
drops a bound is caught, not a live crash fix.

## Decisions
- Drove the real receive-path entry points, not the sub-parsers: BMP fuzzes `DecodeTLVs(data,0,len)`
  (the framing loop `msg.go` uses) plus a `DecodeTLV` boundary call; DHCP fuzzes `handle`+`logExchange`
  (the two `serveMulti` consumers) over `newDHCPHandler`, chosen over calling `parseMsgType`/
  `parseOptionAddr` directly because those are only bounds-safe behind the caller's `len>=244` guard.
- Fresh `*dhcpHandler` per fuzz iteration with `defer leases.stop()` over reusing one handler,
  because `handle` mutates pool/lease state and reuse would exhaust the pool and shrink coverage (R-2).
- Registered each target with an EXACT package path in `mk/test-fuzz.mk` over `/...`, because every
  target package has a `yang/` sibling and `go test -fuzz` refuses to run when the pattern matches
  more than one package.
- Bumped the doc count 54→57 and added a table row over reconciling the whole count, because the
  doc's "54" is internally the sum of its own Fuzz Target Areas table; the makefile's larger count
  (now 63) is a pre-existing 6-target drift unrelated to this spec, which does not reconcile it.

## Consequences
- `make ze-fuzz-test` now runs 63 targets; `make ze-fuzz-one FUZZ=<Name> PKG=<exact-path>` drives one.
- The BMP/RADIUS/DHCP corpora are permanent regression guards: an edit that drops a bound turns a
  future fuzz run red with a reproducer under `testdata/fuzz/<Name>/`.
- `docs/functional-tests.md` Fuzz Target Areas table (57) and `mk/test-fuzz.mk` (63) still differ by
  the pre-existing 6-target gap; a follow-up doc-sync fixit would close it.

## Gotchas
- A "regression protection" fuzz spec has no red-first state: the parsers are already safe, so TDD's
  "tests FAIL first" is genuinely N/A. Do not manufacture a failure; record the empirical clean run
  (0.9M–1.8M execs/target, zero crashes) as the evidence instead of a fake red.
- `go test -fuzz=X ./pkg/...` fails with "matches more than one package" whenever the tree under
  `./pkg` has any second package (here a `yang/` subpackage) — always register fuzz targets with an
  exact single-package path.
- `gocritic weakCond` rejects `resp != nil && resp[0] != x`; guard the index with an explicit
  `len(resp)==0` check between the nil test and the index (also makes the empty-reply case explicit).
- The doc's headline fuzz count was the SUM of its own category table, not the makefile's enumeration
  — bumping it by 3 keeps the table self-consistent without pretending to fix the separate drift.
- Adding a test file to a package runs `ze-verify-changed`, whose `ze-verify-wiring-docs` stage scans
  the WHOLE tree for `// Design:` refs — so a pre-existing broken ref anywhere (here
  `vrrp/packet/checksum.go`, the only `// Design: RFC` in the tree) becomes a deterministic
  STRUCTURAL gate red that `commit_helper --unverified` cannot bypass and that blocks EVERY commit.
  Fix: wrap the note in parentheses — `check_doc_links.py` skips `// Design: (…)` as a
  parenthetical (88 existing uses like `// Design: (none -- predates documentation)`). The
  `doc-links: ignore` marker only works for markdown, not Go Design refs.
- `tmp/ze-verify-failures.json` (the file `commit_helper` reads for structural-gate reds) is rewritten
  ONLY by `verify_run.go` (a full `make ze-verify[-changed]`), never by the bare
  `make ze-verify-wiring-docs` target — so after fixing a structural red you must re-run the full
  verify to clear the commit gate.

## Files
- Created: `internal/component/bgp/plugins/bmp/fuzz_test.go` (`FuzzDecodeBMPTLV`)
- Created: `internal/component/radius/fuzz_test.go` (`FuzzDecodeRADIUSVSA`)
- Created: `internal/plugins/dhcpserver/fuzz_test.go` (`FuzzDHCPHandle`)
- Modified: `mk/test-fuzz.mk` (three exact-path registrations)
- Modified: `docs/functional-tests.md` (count 54→57, table row, three source anchors)

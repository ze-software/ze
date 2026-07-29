# 1294 -- Closing the shared-store contention, and the residuals a fix leaves behind

## Context

`plan/learned/1293` removed the SIGBUS a truncating write inflicted on other
processes' mappings of `database.zefs`, and explicitly recorded that it removed
the crash and NOT the contention that produced it: OSPF opened a second
`*BlobStore` over the shared store from its own process, where zefs's in-process
`sync.RWMutex` cannot exclude anyone. That summary also listed a handful of
residual defects found by review and deliberately not acted on.

Leaving a diagnosed root cause and a list of known defects in a summary is not
finishing the work. This closes all of them.

## Decisions

- **Gate the OSPF store openers on an explicitly pinned config dir, exactly as
  the daemon already gates runtime-state persistence.** `cmd/ze/hub/main.go`
  opens a state store only when `ze.config.dir` is set, and its comment spells
  out why: unpinned, the path is binary-relative and shared by every `ze`
  invocation on the host, "so a one-off `ze -` (and the whole functional test
  suite) must not create or contend on a database.zefs there". Both OSPF openers
  called `paths.DefaultConfigDir()` unconditionally and bypassed that. They now
  go through `pinnedStateDir`, which returns "" unless the operator pinned one.
  Unpinned they degrade exactly as they already did with no store: the RFC 7474
  boot count falls back to the hashed clock seed, GR restart facts are not
  persisted.
- **Take the gate's resolver as a parameter.** Under `go test` the binary lives
  in a build temp dir, so `paths.DefaultConfigDir()` returns "" on its own and a
  gate that did nothing is indistinguishable from one that works. The first
  version of the test proved this by passing with the gate defeated. Injecting
  the resolver makes the gate observable, and the mutation now fails the test.
- **Replace a running binary by rename, not by truncation.**
  `internal/plugins/local/cmd_install.go` copied over an existing `ze` with
  `O_TRUNC` while `replacing` was known true. The kernel maps a running
  executable, so that is the same hazard 1293 fixed in zefs; Linux usually
  refuses with ETXTBSY, macOS does not protect it. Now temp + chmod + rename,
  following `internal/component/config/system/selfupdate.go`'s `stageBinary`.
- **Make the discovery-index gate's stale signal an exit code, not a phrase.**
  The commit gate decided "this index drifted" by matching the substring
  `"is stale"` in a generator's human-facing warning. A wording change would
  silently degrade a BLOCKING gate to warn-only, because a nonzero exit with no
  match reads as "the generator failed" and an unjudgeable index does not block.
  `STALE_EXIT = 3` now carries that meaning, declared once in
  `discovery_sources.py` where the generator/output pairing already lives.

## Consequences

- The functional suite no longer has 64 daemons opening one `database.zefs`. The
  contention that produced the 1293 crash is gone, not merely survivable.
- OSPF's durable boot-count monotonicity (RFC 7474 sec 3) now requires a pinned
  config dir. Unpinned, the high word still advances per restart via the clock
  seed, which is the documented fallback -- but it is a real reduction in
  guarantee for anyone who was relying on the binary-relative default. The
  appliance pins its config dir, so the deployment that needs the guarantee keeps
  it.
- Replacing the installed `ze` now sets exactly the requested mode rather than
  inheriting the previous inode's, so a stale non-executable mode cannot survive
  an install.

## Gotchas

- **A test for a gate is worthless if the ungated path returns the same answer in
  a test environment.** Both the OSPF gate test and, earlier, two zefs tests
  passed with their fix reverted for exactly this reason. Mutation-testing is the
  only thing that caught it each time. If a mutation cannot be made to fail the
  test, the test is not testing the fix -- change the seam until it can.
- **Fixing the in-place write found a second defect for free.** The old
  `copyFile` used `O_CREATE`, so a failed copy left a zero-length `ze` behind: a
  failed install could replace a working binary with an empty file. The
  temp+rename shape makes that impossible, and the test that proves the inode
  changes also proves the temp is cleaned up.
- **Documenting a residual is not the same as leaving it safe.** 1293's own
  Consequences section named the OSPF bypass, the truncating install, and the
  string-coupled gate. All three were real defects with owners; recording them
  bought a tidy summary and shipped the bugs.

## Files

- `internal/plugins/ospf/auth_keystore.go` -- `pinnedStateDir`, boot-count opener gated
- `internal/plugins/ospf/gr_nvs.go` -- GR store opener gated
- `internal/plugins/ospf/state_store_gate_test.go` -- the gate, mutation-verified
- `internal/plugins/local/cmd_install.go` -- temp + chmod + rename
- `internal/plugins/local/cmd_install_atomic_test.go` -- inode replacement + temp cleanup
- `scripts/dev/discovery_sources.py` -- `STALE_EXIT`
- `scripts/dev/{learned_index,package_map,docs_to_code}.py`, `scripts/dev/commit_helper.py` -- the exit-code contract
- `.claude/hooks/pretool-bash.py` -- `timeout 240s` / `nice -n 5` resolve to the real command

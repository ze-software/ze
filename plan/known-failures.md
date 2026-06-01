# Known Failures

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" → pre-existing failures >10 min): logged, not blocking unrelated commits.

### 2026-05-31 — pppoe-client `no-default-route` rejected by config parser

**Resolved 2026-05-31** (commit pending). Fixed with a dedicated `TypeEmpty`
value type wired end-to-end: `yangTypeToValueType` maps `gyang.Yempty → TypeEmpty`
(`yang_schema.go`); `parseLeaf` accepts a bare presence flag, tolerating the
explicit `name true;` form (`parser.go`); `ValidateValue` / `ValueType.String`
cover it (`schema.go`, `valueTypeEmpty` constant in `constants.go`); the set-style
parser accepts the bare flag (`setparser.go`); the serializer emits a bare flag
that round-trips (`serialize.go`). Tests: `parser_type_empty_test.go` (9 cases:
parse bare/value/absent/ASI, serialize + nested-container round-trip, set-parser,
YANG-load `Yempty→TypeEmpty`, `ValidateValue`). Original diagnosis kept below.

**Was Open.** `test/parse/iface-netlink-accepts-pppoe-client.ci` failed. It is
`option=skip-os:value=darwin`, so it never ran on macOS and was first observed in
the QEMU Linux VM run. `ze config validate` rejects the bare `no-default-route`
flag:

```
configuration invalid: -
Errors:
  line 13: line 13: expected value for no-default-route, got SEMICOLON
```

**Root cause (confirmed):** `no-default-route` is `leaf no-default-route { type
empty; }` (`internal/component/iface/schema/ze-iface-conf.yang:1042`) — a valueless
flag. `Parser.parseLeaf` (`internal/component/config/parser.go:179`) is schema-aware
(it receives `node *LeafNode`) but unconditionally requires a `TokenWord`/`TokenString`
value (line 183-188), never checking the leaf's YANG type. So a bare `type empty`
leaf, where the next token is the statement terminator, hits "expected value for ...,
got SEMICOLON". Fix: in `parseLeaf`, when `node` is `type empty`, accept the leaf with
no value (store presence) instead of erroring. Any `type empty` leaf used as a bare
flag hits this, so add a unit test for the parser path plus the existing functional
test.

**Reproduce (linux-only; needs the cross-compiled linux bin/ze):**
```
make ze-qemu-debug NOBUILD=1 RUN='bin/ze-test bgp parse 91 -v'   # 91 = iface-netlink-accepts-pppoe-client (from --list)
# interactive: make ze-qemu-shell  then in VM: ./bin/ze config validate tmp/pppoe-test.conf
```

### 2026-05-31 — QEMU full-run suite failures pending clean triage

**Open (triage).** The first `make ze-qemu-all-test` run (commit dea8336c8, while
the host was under concurrent load) showed failures in `plugin` (16 failed + 54
timeout), `reload` (8 failed + 6 timeout), `l2tp` (2 failed: indices 13,15), and
the VM crashed during `firewall` (`exit 255`), so `policy`/`install`/`unit`/
`integration` never ran. The high timeout counts and the crash are likely host CPU
contention (8 emulated vCPUs starved by concurrent editing), not real failures, but
this needs a CLEAN re-run (quiet host) to separate real from artifact. `encode`,
`decode`, `ui`, `editor`, `managed` passed clean as root. Use `make ze-qemu-debug`
(see below) to drill into specific indices once a clean baseline exists.

> Debug tooling added 2026-05-31: `make ze-qemu-debug RUN='bin/ze-test bgp <suite>
> <idx> -v'` runs targeted tests verbosely in the VM; `make ze-qemu-shell` boots a
> persistent VM for interactive inspection (`qemu-run.py --keep-alive`). Get the
> right `<idx>` from `bin/ze-test bgp <suite> --list` — the suite-summary `[N, M]`
> nicks are run-scoped and do NOT round-trip as selectors.


### 2026-05-31 — host `make ze-verify` still has open BGP functional failures

**Open (triage).** A host `make ze-verify` run after the verb-first command
cutover got past lint, unit, race, and builds, but `ze-test bgp` is still not
clean end-to-end. The command-migration fallout was real and partially fixed:
`bin/ze-test bgp encode 50` (`watchdog`) is green again, and the focused plugin
rerun for the command-touched cases is green for ids `1, 21, 22, 108, 170, 172,
305, 308, 309, 310, 314, 322, 323, 324, 325, 326, 327, 328, 396, 398, 400`.
One touched case still times out: plugin test `2` (`adj-rib-in-replay-on-peerup`)
receives its expected UPDATE but never exits cleanly. The full `tmp/ze-verify-full.log`
still shows many additional pre-existing plugin failures and timeouts (`39, 42,
44, 50, 51, 52, 53, 79, 99, 103, 104, 105, 116, 121, 136, 137, 139, 140, 154,
155, 174, 189, 190, 201, 202, 203, 206, 207, 208, 241, 248, 252, 256, 274, 275,
276, 280, 286, 288, 291, 295, 297, 298, 299, 300, 301, 329, 330, 346, 347, 348,
349`), so unrelated commits should not be blocked on this baseline until the
suite is triaged.

### 2026-06-01 — host `make ze-verify` protocol run still fails functional and ExaBGP baselines

**Open (triage).** The verify debugging protocol run completed all seven top-level
stages and wrote `tmp/ze-verify-failures.log`. Lint, wiring/docs, evidence vet,
cached unit tests, and changed-group race tests passed. Remaining failures are:

- `ze-functional-test` plugin groups: `bfd` (`52`), `check` (`79`), `dispatch`
  (`121`), `fib` (`139`), `iface` (`174`), `loop` (`189,190`), `mpls`
  (`201,202`), `rib` (`288,291,289,290`), `show` (`347,356`), and `teardown`
  (`393,395`).
- `ze-exabgp-test` encoding group: `a` (`conf-watchdog`), reproduced by
  `uv run --with psutil --with paramiko ./test/exabgp-compat/bin/functional encoding --timeout 180 a`.

Evidence paths from that run: `tmp/ze-verify-failures.log`,
`tmp/verify/06-ze-functional-test.log`, and `tmp/verify/07-ze-exabgp-test.log`.

## Resolved

### 2026-05-31 — dispatch single-marshal + stale plugin lists (15 packages)

**Resolved 2026-05-31.** The 15 packages that began failing once `make ze-verify`
was runnable again (after the `tmp/go.mod` sentinel landed) are all green. Fixes,
by class:

1. **`single-marshal OnExecuteCommand` (commit 30b025270).** Command handlers now
   return structured `any`; the SDK marshals once. Tests that did
   `assert.Contains(t, data, "substring")` were comparing against a `map`/`[]byte`/
   typed slice (key/element match, never substring). Fixed by asserting on the
   marshaled JSON string: `adj_rib_in`, `healthcheck`, `rs`, `fib/kernel`,
   `fib/p4`, `fakeredist`.
2. **Stale registration / section lists.** Added the new plugins
   (`bgp-filter-aspath-length`, `flow-export`, `ldp`, `rsvp-te`) to the expected
   sets in `cmd/ze` and `internal/component/plugin/all`, and `platform` to the
   `cmd/ze/host` section list.
3. **Migration serializer keyword gap (commit 3da416d31).** The `internal` plugin
   keyword landed with updated goldens but `migrate_serialize.go` still emitted
   `external`. It now emits `internal` for built-in (`use`) plugins, `external`
   for `run` processes.
4. **Multi-line YANG descriptions.** `cmd/ze/completion/words.go` now collapses
   descriptions to their first line so shell completion stays one row per
   candidate; `internal/component/config/yang` description-propagation assertions
   updated to the enriched strings.
5. **CLI grammar catch-up (committed refactors 336cb2472 modes, 72d268c77 view
   consolidation).** `summary` doc lookup → `show summary` (canonical verb-first
   path); `option changes` is a display column, not a pipe redirect (only `blame`
   redirects); 7 `.et` files updated to the shipped grammar (`exit` switches
   config→operational, `show | blame` / `show | changes [all]` for views,
   `disconnect` requires an active session in completions).

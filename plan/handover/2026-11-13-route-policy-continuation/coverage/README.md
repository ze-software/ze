# Phase 6 scoped measured coverage preparation

SOURCE ONLY. No command in commands.txt was executed by this author. No product,
test, documentation, approval, index, build or native state was changed. Root runs
commands serially in the main checkout after the active backend source handoff.
Existing unit/race/fuzz/RFC results are not repeated as pending here. Daemon
discrimination cuts, the final 28-carrier matrix and stress80 remain separate.

## Existing route, with the subprocess detail resolved

`ai/rules/testing.md:77` requires count mode and reading each new outcome, so the
old `rp1-rp2-coverage.out` and `rp3-forwarding-output-coverage.out`, which both say
`mode: atomic`, do not discharge this explicit debt. Neither a passing test list
nor a package percentage substitutes for the block counts.

The scoped unit action deliberately lacks arbitrary coverage flags. Use its
documented fallback, `./le job run ... command go test`, with canonical tags
(`docs/contributing/running-commands.md:479`). The native job owner detects race
and selects CGO0 for a nonrace Go test (`internal/le/job/process.go:97`). Keep
count mode separate from the already completed race runs; `-race` forces atomic.

There is already an established native subprocess coverage route; no new harness
API is needed. `internal/le/rfc/discriminate_guest.go:271` explains and constructs
it: compile a count-instrumented test ELF on the host, copy that ELF guest-local,
pass BOTH `-test.coverprofile=<shared absolute path>` and
`-test.gocoverdir=<fresh guest-local counter directory>`. Current
`internal/test/userns/userns_linux.go:132` withholds only the text-profile flag and
forwards `test.gocoverdir` to the userns child. The parent builds its text profile
from that directory after joining the child, so the parent cannot overwrite the
child's useful counts with a parent-only profile. Environment-only GOCOVERDIR
is unnecessary for this route. The explicit test flag is the established input.

Do not hand-run a covered ELF with just `-test.coverprofile`; the testing runtime
can choose a private directory unknown to the reexecuted child. Do not share one
raw directory between different coverage binaries or old runs. Ordinary host
`go test -coverprofile` supplies its own shared `-test.gocoverdir` automatically.

The guest command reuses the own-system kernel already named by root. Source
preparation does not reverify its hash. Runtime output must again show the named
kernel, actual command-exit 0 and interrupted false, alongside every expected
top-level PASS and zero SKIP. The guest never needs a Go toolchain.

## Reading the results

The command file gives a small set of package profiles, rather than a whole-tree
run. Sysrib runs all its Test functions because changed old forwarding paths are
part of the affected population. RPC/SDK/server/startup selections are narrowed
to this spec. Fuzz campaigns are already done; `^Test` avoids counting fuzz seed
skips as ordinary-test skips.

For every profile first check `mode: count`; retain all nonzero AND zero rows in
the new producer files, using the supplied awk. Join source ranges to the
outcome checklist and record the actual third-column count for each selected
body. For a one-line body Go can combine source spans: use the full location,
not merely the start-line. A positive function entry does not prove every
return below it. When a short-circuit predicate shares a single counter, the
named subtest results identify which input outcome ran; the count alone does
not separate operands. Do not invent branch percentages from statement data.

Retain the host native job log and refuse `(cached)` as a claim of a fresh run.
The first change from atomic to count instrumentation has a different test
binary; do not blanket-repeat all already completed race populations. If a
prepared coverage invocation is cached, rerun only that invocation with
`-count=1`. The guest ELF is always directly executed with `-test.count=1`.

For new rejection guards that stay zero, report the exact source range and
missing outcome. The source census below flags likely gaps; only the actual
count profile can establish zero. No new tests are proposed or changed by this
read-only task.

## Source census cautions

* Native paired controls are still owed measured positive counts at current
  `pkg/plugin/rpc/bridge_native.go:21` (callback failure) and `:31` (before Ready).
  The exact tests each contain a useful healthy control. The existing old
  profile recorded both guards zero; that is historical data, not a claim about
  a final current count run. Their existing cut/restoration obligation is not
  replaced by coverage.
* The five identity probe tests call live, nonnil handles/routes/destinations.
  No current internal test call targets RouteProbeIPv4's three EINVAL guards
  (nil handle, nil route, nil Dst). Its IPv6 EAFNOSUPPORT and live request path
  have explicit tests. Instrument `github.com/vishvananda/netlink` or none of
  these new vendor-producer outcomes will appear at all.
* `forwardingScope` has defensive non-unicast, missing direct-device and missing
  member-device outcomes. The 20-case matrix covers discard via buildRichRoute's
  earlier return, so that does not itself exercise forwardingScope's non-unicast
  branch. No matrix case explicitly supplies a targetless direct/member route.
  Inspect these rows before claiming every new outcome executed.
* backend_replay_linux.go was initially old-behavior stubs, then this author
  reread its final 68-line implementation after the stable handoff. The four
  pure and eight own-kernel roots remain the selectors below. Its return bodies
  are rendering refusal31, destination/nonIPv4 fallback42/45, probe miss49,
  unexpected ACK52, other probe failure58, change-disappearance62/65, and final
  handled change result67. Root retains final-source execution and counts.
* TestBuildRichRouteScope's 20 cases are a loop, not t.Run subtests. The native
  log names the top-level test; a PASS means all 20 assertions completed, but
  do not print an invented 20-test execution count.

## Boundaries

This is preparation, not coverage evidence. Package totals, successful test
counts, skips and source block counts remain unmeasured by this author. The
eight RFC discrimination records and five fuzz campaigns are already completed
per root; none is rerun here. Coverage adds no daemon matrix/stress80/interop,
doc, Review Gate or closure claim. Additional selected-QEMU/stressrepro/fixture
tooling branches also owe their existing count-mode obligation if root has no
current measured profile for them; they are outside the routing producer
selections below and must not silently be marked covered by these profiles.

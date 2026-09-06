# Learned: ze-test-dns-stub

`ze-test` could fake an IRR, an RPKI cache, RADIUS, TACACS+ and PeeringDB, and
no DNS a test controls. Every feature that resolves a name was therefore proven
at unit level only. The stub is small: one package, two entry points, two `.ci`
files. What it taught is about the shape of a fake, not about DNS.

## What the design turned on

**A fake's data model is a correctness surface, and the wrong shape lets a test
lie.** The spec put the response code on the answer record, one per name AND
record type. Writing the zone found the problem immediately: `broken.example.test`
answers SERVFAIL, and SERVFAIL is what a server says when it cannot answer at
all, so the entry had to declare it twice with nothing keeping the two halves
consistent. The code moved the code to the NAME and left the addresses on the
TYPE.

That is not tidiness. RFC 2308 Section 2.2 says NODATA is NOERROR with an empty
answer section, and in the corrected shape a zone entry cannot say anything
else: a name that exists carries exactly one code, and a type it does not hold
has no entry to contradict it. The rejected shape would have permitted
`v4only.example.test` NOERROR for A and NXDOMAIN for AAAA. A firewall test
reading that would have watched `resolveAndRecord`
(`internal/component/firewall/plugins/domain/domain.go`) empty a live set and
called it correct, because on NXDOMAIN emptying the set IS correct.

**Zero is the useful TTL, and that is a claim about the consumer, not about the
stub.** A comfortable 300-second TTL would have made the stub invisible: the
second lookup would have been answered by `cache.put`
(`internal/component/resolve/dns/cache.go`), which returns before storing only
when the TTL is zero, and the answer-change test would have reported the
daemon's own cache back to itself and passed. The zone defaults to 0 for that
reason, and `cached.example.test` carries 300 for the test that wants the
opposite.

**A capability token must name the capability the test needs, not the one that
happens to be nearby.** The two `.ci` files bind port 53, and `net-admin` was
the only token that would have covered it without new code. Declaring it would
have skipped every host that can bind a low port and cannot program nftables:
an over-strict gate deletes coverage, which is what the gate exists to prevent.
`net-bind` was added instead, and `TestCapsNetBindGateBothPolarities` drives
both polarities through the `hasCaps` seam so the "skips without it" branch is
proven on a host that has it.

## What the work walked into

**A process that replaces its own stderr has exactly one drain, and a library
that exits for you walks past it.** `ze-test dns --help` printed nothing.
`crashlog.Init` (`internal/core/crashlog/crashlog.go`) puts a pipe over fd 2,
`flushCrashlog` (`cmd/ze/dispatch.go`) is the only reader, and `main`
(`cmd/ze/main.go`) calls it after dispatch returns. `flag.ExitOnError` calls
`os.Exit` from inside the flag package, so the usage text reached a pipe nobody
read and the command exited 0 having written nothing. Measured the same for
`irr` and `rpki`.

The stub uses `flag.ContinueOnError` and returns a code. The other seven mocks
still have it, and the source repair is a decision rather than an edit: either
`crashlog` owns an exit function every caller uses, or nothing reachable from
the dispatcher calls `os.Exit`. Both touch the shipped daemon's crash capture.
Class: `plan/journal/output-lost-to-an-exit-past-the-flush.md`.

**A test that proves the helper does not prove the reader gets the text.**
`TestUsageListsEveryZoneName` passed throughout, over `zoneUsage`, while
`--help` printed nothing at all. `TestHelpPrintsTheZone` was added to drive
`Run` with stderr captured, which is the entry point an author actually types.

## What is left open

**The documentation is written and did not land.** `docs/functional-tests.md`
holds a `### ze-test dns` section and a `caps=` token table in the working tree,
and another session holds an uncommitted ExaBGP hunk in the same file, so
staging it would carry their work. New class:
`plan/journal/documentation-stranded-by-a-siblings-hunk.md`.

**A fixture that binds a privileged port depends on running as root, and the
per-test netns launch mode would take that away.** `prepareNetnsBinaries`
(`internal/le/qemu/netns_linux.go`) setcaps `ze` and `ze-stripped` only, so a
fixture forked by a credential-dropped `ze` inherits the uid without
`cap_net_bind_service`. Unreached today because the `plugin` suite declares
`Namespace: guestRoot` (`vmSuites`, `internal/le/qemu/alltests.go`). The
constraint is written onto `dnsStubAddress`
(`internal/test/fixture/plugin_fixture_dns_stub.go`), where the next author
reads why the port is 53. The real repair is the port seam:
`system name-server` is `type zt:ip-address` and cannot carry one, and changing
that touches the leaf type, the `resolv.conf` writer and `dnsServerResponds`.
That is a product change with an operator-visible surface, and no test needs it
yet.

**The three firewall domain-group `.ci` files are still unwritten.** They were
the reason this spec existed, and the stub is not what blocks them: they run the
daemon with no external-plugin block and launch their fixture as a separate
process, so it holds no channel to dispatch on. That is a design decision inside
`plan/spec-firewall-domain-group.md`, which owns them.

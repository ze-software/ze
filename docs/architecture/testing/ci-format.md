# .ci Test File Format

The `.ci` format is used by Ze's test runner to define functional tests. It supports embedded files (Tmpfs), test options, expectations, and commands.

> For the execution architecture (how tests are scheduled and run concurrently) and the web `.wb` format, see [`runner-architecture.md`](runner-architecture.md).

## Syntax Overview

All lines use key=value format with `:` separators:

```
action=type:key=value:key=value:...
```

| Action | Purpose |
|--------|---------|
| `stdin=` | Embed stdin content for processes |
| `tmpfs=` | Embed file content inline |
| `option=` | Test configuration |
| `cmd=` | Commands (API, shell, foreground/background) |
| `expect=` | Expectations to validate |
| `await=` | Block until the daemon's stderr carries a line, then tear down (deterministic fence) |
| `reject=` | Negative expectations (fail if matched) |
| `action=` | Actions (send notification, raw bytes) |
| `http=` | HTTP endpoint checks and readiness polls |
<!-- source: internal/test/runner/record_parse.go -- parseAndAdd, CI file parsing -->
<!-- source: internal/test/tmpfs/tmpfs.go -- Tmpfs, File, Parse -->

## Key Concepts

### An unparseable test file fails; it never hides or vanishes

A file that does not parse is recorded as a **permanent failure** and discovery
continues. It is never dropped, and it never aborts the rest of the directory.
All three discoverers behave identically.

| Discoverer | Format | Marker |
|------------|--------|--------|
| `EncodingTests.Discover` | `.ci`, suites rooted by `registerCIRoot` (encode, plugin, ui, ...) | `Record.ParseFailed` + `State=StateFail` + `FailureType=parse_error` |
| `ParsingTests.Discover` | `.ci` (parse suite) | `parsingTest.ParseError` |
| `ParsingTests.Discover` | legacy `valid/*.conf` + `invalid/*.conf` with a companion `.expect` | `parsingTest.ParseError` |
| `DecodingTests.Discover` | `.ci` and `.test` (decode suite) | `decodingTest.ParseError` |

The legacy `.conf` layout has no instances in the tree today, and it is held to
the same contract anyway: a missing, empty, or bad-regex `.expect` file records
that one fixture as a failure rather than abandoning the directory. An
unreachable abort becomes reachable the moment someone adds the directory, and
the shape is the bug.

Both alternatives are silent-coverage-loss bugs this project has already paid
for, which is why neither is permitted:

| Anti-pattern | What it costs |
|--------------|---------------|
| Return the parse error out of `Discover` | The whole directory is abandoned. One bad `.ci` made the `test/ui` suite discover and run ZERO tests, and a suite that runs nothing reads as green. |
| `continue` past the bad file | The file leaves the suite with no warning and no failure record. Its coverage disappears and nothing says so. |

A guard that neither denies nor speaks does not exist
(`ai/rules/evidence.md`). The runner short-circuits a parse-failed
test before executing anything, so the reported error is the parse error rather
than a confusing downstream symptom.

**Unparseable outranks skipped.** A file that both fails to parse and carries
`option=needs-linux` / `option=skip-os` is reported FAIL, not SKIP. Its skip
marker was parsed from the same broken file, so it is not trustworthy evidence
that the file need not run: a contradicting directive may sit past the break, or
the marker itself may be what was mis-parsed. Honoring it would mean trusting a
broken file's own claim that it can be ignored.

The consequences are asymmetric, which is what settles the ordering. 158 `.ci`
files carry one of those markers (12 in `test/ui`), and on a non-Linux host they
never run. A wrongly-SKIPPED malformed file is invisible indefinitely, which
is exactly how `test/ui` rotted. A wrongly-FAILED one is loud and costs a single
commit. The check therefore lives in `parallel.go`'s per-test goroutine, ahead of
the skip short-circuit: that is the real entry point, and both
`Runner.runTest` and `parsingRunner.runTest` are reached through it.
<!-- source: internal/test/runner/parallel.go -- per-test goroutine, ParseFailed ahead of SkipReason -->
<!-- source: internal/test/runner/parsing.go -- parsingRunner.Run, ParseError suppresses the SkipReason copy -->
<!-- test: internal/test/runner/discover_malformed_test.go TestSkipMarkedMalformedCIStillFailsThroughParallelRunner, TestSkipMarkedMalformedParseTestFailsThroughParallelRunner -->
<!-- source: internal/test/runner/record_parse.go -- EncodingTests.Discover, the reference shape -->
<!-- source: internal/test/runner/parsing.go -- ParsingTests.Discover, parsingRunner.runTest -->
<!-- source: internal/test/runner/decoding.go -- DecodingTests.Discover, decodingRunner.runTest -->
<!-- test: internal/test/runner/discover_malformed_test.go -- parse and decode discoverers -->
<!-- test: internal/test/runner/record_parse_test.go TestDiscoverSkipsUnparseableFile -->

### Suite label, test id, and failure identity

The verify debugging protocol identifies a functional failure with:

| Field | Source | Purpose |
|-------|--------|---------|
| Suite label | `ze-test` runner label such as `plugin`, `ui`, or `managed` | First routing boundary inside `./le functional` |
| Test id | One-based decimal id printed by `--list` and per-test result lines | Exact single-test rerun scope |
| Run number | `N/TOTAL` printed by `--list` and per-test result lines | Human progress marker for long suites |
| CI file path | Parsed `.ci` source path | Full test definition and embedded fixtures |
| Failure kind | Runner failure type, timeout state, or mismatch class | Conservative grouping key |
| Expected / received evidence | `TEST FAILURE` block detail | Full debugging evidence in the stage log |

In `ZE_VERIFY_MODE=1`, failed suites emit native failure-group metadata before
the full failure blocks. The compact verify index uses that metadata for group
routing and keeps the full `TEST FAILURE` blocks in the stage log.
<!-- source: internal/test/runner/failure_group.go -- suite-local failure groups -->
<!-- source: internal/test/runner/report.go -- TEST FAILURE blocks -->
<!-- source: internal/test/runner/display.go -- per-test result lines -->

### Per-step trace output

All three runner families (`.ci`, `.wb`, `.et`) record per-step outcomes during
execution and emit dual-format trace output:

- **Human:** colored checkmark/cross glyph per step with kind, assert, and detail.
- **Machine:** `VERIFY STEP: {"file":"...","step":N,"kind":"...","status":"pass|fail",...}` -- one JSON line per step, matching the `VERIFY FAILURE GROUP:` prefix convention.

Trace is emitted automatically for failed tests. Under `-v`, passing tests also
show their trace. The `.ci` runner includes the trace in its `TEST FAILURE` report
block when `StepTrace` is non-empty.
<!-- source: internal/test/trace/trace.go -- StepResult, PrintTrace, writeHuman, writeMachine -->
<!-- source: internal/test/runner/report.go -- step trace in failure reports -->

### conn and seq

Most directives use `conn=N` and `seq=N` to identify message ordering:

- **conn** (connection): 1-based TCP connection index. Each `ze-peer` instance
  manages one TCP connection. Multi-peer tests use `conn=1` for the first peer,
  `conn=2` for the second, etc. The maximum is set by `option=tcp_connections:value=N`.
- **seq** (sequence): 1-based message sequence within a connection. `seq=1` is the
  first BGP message after OPEN/KEEPALIVE, `seq=2` is the second, etc.

A test with two peers and one UPDATE each uses `conn=1:seq=1` and `conn=2:seq=1`,
not `conn=1:seq=1` and `conn=1:seq=2`.

### Port Substitution

Each test owns a pair of ports, exposed as variables in commands, `tmpfs=`
content, `option=env` values, and URLs:

| Variable | Meaning |
|----------|---------|
| `$PORT` | BGP peer port (leased by the runner, used by `ze-peer --port $PORT`) |
| `$PORT2` | Secondary port, `$PORT`+1 (web UI, looking glass, plugin acceptor) |

Never hardcode port numbers. Use `$PORT` in `cmd=` exec values and `$PORT2` in `http=` URLs.

The pair is LEASED when the test starts, not when the suite discovers it
(`runner.LeaseTestPorts`, `internal/test/runner/ports.go`). Discovery numbers the
Nth test of every suite from the same base, so the preference alone collides
whenever two ze-test processes run at once; the lease takes a machine-wide
advisory lock in `$TMPDIR/ze-test-port-locks` and probes the pair, and a test
whose preferred pair is locked or occupied gets one from 25000-32759 instead. A
hardcoded number in a `.ci` file takes part in neither step, which is why the
rule above is a rule.

<!-- source: internal/test/runner/ports.go -- LeaseTestPorts -->
<!-- source: internal/test/runner/runner_exec.go -- runTest leases before any $PORT expansion -->


## Stdin Blocks

A stdin block embeds content the runner pipes to a process's standard input.
Two commands take it as a FILE instead, and the table under "Where the block
goes" says which, why, and what the `.ci` writes to select each route.

### Syntax

**Multi-line (with terminator):**
```
stdin=<name>:terminator=<TERM>
<content>
<TERM>
```

**Single-line hex:**
```
stdin=<name>:hex=<hex-value>
```

**Single-line text:**
```
stdin=<name>:text=<text-value>
```

### Parameters

| Parameter | Description |
|-----------|-------------|
| `name` | Identifier referenced by `cmd=...:stdin=<name>` |
| `terminator` | End marker for multi-line content |
| `hex` | Hex-encoded content (single-line) |
| `text` | Plain text content (single-line, newline appended) |
<!-- source: internal/test/tmpfs/tmpfs.go -- StdinBlocks map, parseStdinBlock -->

### Examples

**Multi-line (config):**
```
stdin=ze-bgp:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    local-as 65533;
}
EOF_CONF

cmd=foreground:seq=1:exec=ze -:stdin=ze-bgp
```

The block goes to a file and the daemon runs as `ze start <file>`. The example
said `exec=ze bgp server -:stdin=ze` until 2026-09-07, naming a command the CLI
does not have and a block the file does not declare.

**Single-line hex (decode test):**
```
stdin=payload:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C020000001C...
cmd=foreground:seq=1:exec=ze bgp decode --json --family ipv4/unicast -:stdin=payload
expect=json:json={ "type": "update", ... }
```

The block is PIPED: the `-` belongs to `decode`, not to the daemon.

**Single-line text:**
```
stdin=cmd:text=update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24
```

### Where the block goes

The runner pipes the block, with two exceptions. Each one exists because the
child needs a path it can open, and each is selected by what the `exec=` line
already writes: no new directive chooses between them.

| The `exec=` line | Where the block goes | Why |
|------------------|----------------------|-----|
| `ze -`, and its flagged forms `ze -d -`, `ze --plugin <p> -`, `ze --mcp <port> -`, `ze --web <port> --insecure-web -` | a FILE in the work directory, and argv becomes `ze [flags] start <file>` | the daemon re-reads its config on SIGHUP, a restart reuses it, `action=rewrite:dest=ze-bgp.conf` addresses it by bare name, and a rollback assertion reads the directory beside it |
| `ze-peer ...` with NO `-` in argv | a temporary FILE appended to argv | `ze-test peer` takes its expect script as a path argument |
| `ze-peer ... -` | PIPED | `LoadExpectFile` opens its argument through `cliio`, so `-` is standard input there |
| every other line, `ze bgp decode -`, `ze config validate -`, `ze-test replay -`, `sh -c ...` included | PIPED | `-` is the `cliio` stdin token (`ai/rules/cli.md`), and the command reads standard input |

The daemon `-` is recognized by POSITION, not by a list of verbs: the runner
asks `zeDaemonConfigArgIndex` which argument is the config, and substitutes only
when that argument is the `-`. A `-` belonging to a verb is left in argv and the
block is piped, so `ze config validate -` and `ze bgp decode pcap -` test the
form the operator types.
<!-- source: internal/test/runner/runner_exec_util.go -- routeStdinBlock -->

Until 2026-09-07 the runner took the FIRST `-` in argv whatever it meant, so
every verb form ran against a file path the author never wrote. A test that
named a stdin block on a verb was testing the path form under a `-` that said
otherwise, and `test/ui/bgp-decode-stdin-hex.ci` failed with
`invalid hex: encoding/hex: invalid byte: U+002F '/'` for that reason.

### What a ze-peer block may carry

A block handed to `ze-peer` (named on a `cmd=...:exec=ze-peer ...:stdin=<name>`
line) is read first by ze-peer and then by the test runner. The block named
`peer` is validated by the same rules even with no such line, because a `.ci`
with no `cmd=` at all feeds its `expect=` lines to ze-peer by another route. **A line neither of them
acts on fails the file at parse time**, naming the block, the line number and the
directive.

| Line | Read by |
|------|---------|
| `expect=bgp`, `reject=bgp`, `action=*`, and the peer's own `option=` (`asn`, `bind`, `tcp_connections`, `conn_map`, `open`, `update`, `linger`, `silent`, `await_eor`) | ze-peer |
| `expect=json`, `expect=stderr`, `expect=syslog`, `reject=stderr`, `reject=stdout`, `reject=syslog` | the test runner, where the line stands |
| `cmd=api` | nobody. It documents the command that produced the expected bytes |
| `option=timeout` | the test runner parses it, and **adopts it only when the file declares none.** Its scope is the whole test, not this peer, so a file-level value always wins. 450 tracked peer blocks carry one. Write the one that governs outside the block |
| `option=env` | **refused.** It sets the environment of every process the test starts, not of this peer, so it must be written outside the block |
| a line neither parser accepts | **refused** |

Any OTHER directive the runner parses is accepted where it stands and applies to
the whole test. `option=file`, `expect=exit`, `await=` and `http=` all work
inside a peer block, and none of them is peer-scoped. Only `option=env` and
`option=timeout` are singled out. Those two read as peer scope and are not.

A value the named option does not have is refused as well, not just an unknown
option name. `option=update:value=inspect-update-message` and
`option=asn:value=abc` each used to parse into nothing, which is the same silent
drop one level down.

The accepted set is derived from the two parsers rather than written out a third
time (`peer.ClaimLine` reports what ze-peer did with the line, and the runner's
own line parser answers for the rest), so a directive added to either one cannot
start being dropped here. Before the guard existed the block forwarded only
`expect=` and `action=` lines and discarded everything else in silence: eleven
`reject=bgp` directives across nine RFC-behaviour tests, and the `reject=stderr`
of a tenth, asserted nothing while reading as the negative half of the proof.

<!-- source: internal/test/runner/peer_contract.go -- validatePeerBlockDirectives, the guard -->
<!-- source: internal/test/peer/expect.go -- ClaimLine, ze-peer's own answer -->

## Tmpfs (Virtual File System)

Tmpfs allows embedding multiple files within a single `.ci` file. Files are extracted to a temp directory at runtime.

### Syntax

```
tmpfs=<path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
<content>
<TERM>
```

### Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `path` | Yes | - | Relative path (no `..`, no absolute) |
| `mode` | No | Auto | File permissions (octal: 644, 755) |
| `encoding` | No | `text` | Content encoding: `text` or `base64` |
| `terminator` | Yes | - | End marker (alone on line) |
<!-- source: internal/test/tmpfs/tmpfs.go -- File struct, Tmpfs.AddFile -->

### Mode Defaults

Files default to `0644`. Use an explicit `mode=` only when the test needs a
different permission.

### Terminator Rules

- Must be non-empty
- Must be unique within file (no two Tmpfs blocks can share terminator)
- Alphanumeric and underscore only: `[A-Za-z0-9_]+`
- Matched exactly (no whitespace trimming)
- Recommended: `EOF_<PURPOSE>` (for example, `EOF_CONF`)

### Example

```
tmpfs=peer.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    local-as 65533;
}
EOF_CONF


option=file:path=peer.conf
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF...
```

### Security Constraints

1. **No absolute paths** - must be relative
2. **No parent traversal** - no `..` components
3. **No hidden files** - no `.` prefix in path components
4. **Path length limit** - max 256 characters
5. **Path depth limit** - max 10 levels
<!-- source: internal/test/tmpfs/security.go -- validatePath, Validate -->

### Limits

Configurable via environment variables:

| Limit | Default | Environment Variable |
|-------|---------|---------------------|
| Max file size | 1 MB | `ze.bgp.ci.max_file_size` |
| Max total size | 1 MB | `ze.bgp.ci.max_total_size` |
| Max files | 100 | `ze.bgp.ci.max_files` |
| Max path length | 256 | `ze.bgp.ci.max_path_length` |
| Max path depth | 10 | `ze.bgp.ci.max_path_depth` |

## Options

```
option=<type>:key=value[:key=value...]
```

| Type | Keys | Description |
|------|------|-------------|
| `file` | `path=<name>` | Config file to use |
| `asn` | `value=<N>[:peer=<ip>]` | The AS ze-peer opens with. It reaches BOTH carriers RFC 6793 defines: the two-octet My Autonomous System field, narrowed to AS_TRANS (23456) above 65535, and the Capability Value of capability 65. The range is 1 to 4294967295 and a value outside it fails the file when it is read, naming the option and the value. `peer=<ip>` binds the declaration to one of ze's endpoint addresses, for a peer process that serves several of ze's peers at once (`option=conn_map`). With no `option=asn` line at all the runner derives one from the `session { asn { remote N } }` leaf of the ze configuration the `.ci` names. It reads a `tmpfs=` block, a `stdin=` block and the file an `option=file:path=` points at, and it follows the inheritance chain: the router's own `bgp { session { asn { ... } } }`, then a `group` or `template`, then the peer, each level overriding the one above. Three things fail the file at read time rather than being guessed: two peers ze dials at ONE address expecting different ASNs, a declared AS the reader cannot read, and an eBGP peer the derivation reached with nothing. **That last refusal knows only what the reader knows.** A peer is judged eBGP by comparing the local and remote AS, so a peer for which NO local AS is declared at any level of the chain is not judged eBGP and is not refused; it inherits ze's own AS from the mirror, which is right for an iBGP session and wrong for an eBGP one. The derivation reaches no peer at all when a compiled fixture under `internal/test/fixture` writes the configuration and launches `ze-test peer` itself, because no `.ci` block is involved; such a fixture passes `--asn` (`cliWirePeerAS`, `internal/test/fixture/ui_fixture_send_bgp.go`). |
| `bind` | `value=ipv6` | Bind to IPv6 |
| `timeout` | `value=<duration>` | Test timeout (e.g., `30s`). Overrides auto-timeout. |
| `tcp_connections` | `value=<N>` | The number of TCP connections the peer serves. It is a BOUND, never a witness: the count says how many connections happened and not who caused them. A peer that closes when its expectations are met makes ze dial again on its retry timer, so `value=2` is reached by a daemon that did nothing. To assert that the DAEMON dropped and restarted a session, add `option=linger`, which makes the peer incapable of causing the second connection. |
| `linger` | `value=true` | Peer-block only: the check peer never closes a connection itself. After ALL expectations complete it prints its success token and holds the session open (answering KEEPALIVEs) until test teardown. BETWEEN connections, where one connection's expectations are met and a later connection is still owed, it holds that connection until the REMOTE closes it, and fails the test if teardown comes first. Without it a completed peer closes, which ze correctly treats as session-down; that withdraws the peer's routes and races any forwarding still in flight toward other peers, and it also makes the daemon's OWN close unobservable, because ze dials again on its retry timer whoever closed (`test/reload/config-apply-ordering-address-swap.ci` asserts a stop and a restart, and needs the peer to be incapable of causing one). A `conn_map` peer is excluded from the between-connections hold: it serves every connection of one batch in turn and then waits for the daemon to close them all, so holding the first would starve the batch. A `reject=bgp` that fires during either hold RETRACTS the success token already printed, so the peer still fails the test. |
| `silent` | `value=true` | Peer-block only, check mode only: the peer stops sending the automatic KEEPALIVE reply it otherwise writes for every message it receives. It holds the TCP connection open and keeps reading and matching expectations. Needed to reach ze's receive hold timer: ze sends its own KEEPALIVE every hold/3 seconds, each automatic reply resets ze's hold timer, and "the peer went quiet" is otherwise unexpressible. A closed connection is a different event on a different code path, so `action=close` does not substitute. **Explicit writes still happen**: `action=send`, `action=notification`, the OPEN handshake itself, and `option=linger`'s post-completion KEEPALIVE loop are unaffected, so `silent` with `linger` is not silent. Sink and echo modes ignore it. See `test/plugin/deadpeer-holddown.ci`. |
| `open` | `value=<behavior>` | OPEN message behavior |
| `update` | `value=<behavior>` | UPDATE message behavior |
| `env` | `var=<KEY>:value=<V>` | Set environment variable |
| `skip-os` | `value=<os>[,<os>]` | Skip test on listed GOOS values (e.g., `darwin`, `linux`) |
| `needs-linux` | `[caps=<tok>[,<tok>]]` | Linux-only test. It skips on non-Linux hosts and runs in the QEMU guest through `./le qemu all-tests`. `caps=` declares required capabilities such as `net-admin`, `net-raw`, and `bpf`; an unavailable capability produces a visible skip. |
| `needs-path` | `value=<repo-rel-path>[:hint=<cmd>]` | Declares an optional heavyweight artifact. The runner resolves the path against the repository root and prints the native `hint` when the artifact is absent. A malformed or escaping path is a parse error. |
| `netns-link` | `name=<if>[:address=<cidr>]` | Provisions a dummy interface inside the per-test namespace. The test skips outside the `./le qemu netns-test` path because the named link must never be created on the host. |
| `exclusive` | `group=<name>` | Never run concurrently with another test carrying the same group name. Tests outside the group are unaffected and keep running alongside, so this costs far less wall-clock than dropping a whole suite to `-p 1`. Use it when tests contend for a kernel-global observation surface that unique names or addresses cannot partition: the ddos tests (`group=ddos-flood`) all flood the same loopback interface, and each daemon's detector picks its victim by top-destination-bytes over that interface's counters, so a sibling's concurrent flood is indistinguishable from the test's own. Applies on every platform and in every runner mode, because the contention is a property of the tests rather than of the host. |
<!-- source: internal/test/runner/record_parse.go -- parseAndAdd, option parsing -->
<!-- source: internal/test/runner/caps.go -- capsRequired, the caps= token table -->
<!-- source: internal/test/runner/needs_path.go -- repoRootFrom, the needs-path lookup -->
<!-- source: internal/test/runner/parallel.go -- per-group lock, taken before the concurrency semaphore -->

#### Choosing between `needs-linux`, `caps=`, and `skip-os`

| The `.ci` test ... | Use |
|--------------------|-----|
| Only validates config (`ze config validate -`), parses, or runs an offline `ze show` / `ze env` | Nothing. It runs natively on every OS |
| Boots a daemon that APPLIES Linux-only config (interface, VLAN, firewall, L2TP kernel) | `option=needs-linux` |
| The same, and needs privileged network configuration (creates interfaces, brings links up, programs netlink) | `option=needs-linux:caps=net-admin` |
| The same, and opens a raw or packet socket (`resolve ping`, traceroute) | `option=needs-linux:caps=net-raw` |
| The same, and loads eBPF | `option=needs-linux:caps=bpf` |
| Skips on one non-Linux OS for a reason unrelated to the kernel | `option=skip-os:value=darwin` |
| Needs an optional heavyweight artifact the checkout does not carry | `option=needs-path:value=<repo-rel>:hint=<cmd>` |

`caps=` takes a comma-separated list, so a test that programs netlink and loads
eBPF declares `caps=net-admin,bpf` and is gated on both. An unknown token is a
parse error on every host, macOS included, so a typo cannot silently disable the
gate.

`caps=net-admin` exists because Linux alone is not the requirement. On an
unprivileged Linux host, a CI runner or a rootless container, a test that
applies interface config does not fail cleanly: the interface plugin fails its
configure handshake with `operation not permitted` and the DAEMON exits 1, then
the TEST hangs because its check peer waits for a session the exited daemon will
never open. The gate reads `CapEff` from `/proc/self/status`, not uid 0: a
setcap'd binary holds the capability without being root, and a restricted
container can be root without it.

A `caps=` test does not run in the merge gate. `./le verify worktree` runs
unprivileged, so the marker turns an opaque hang into an honest skip there, and
the coverage relocates to `.github/workflows/qemu-nightly.yml`.
`TestCapabilityGatedTestsHaveANativeVMHome` fails when that link is broken:
marking a test with a capability nobody's CI has would be a coverage deletion
wearing a skip's clothing.

<!-- source: internal/test/runner/caps.go -- capsRequired, capsAccepted -->
<!-- source: internal/test/runner/caps_linux.go -- probeCaps, the CapEff read -->
<!-- source: internal/le/workflowcheck/workflowcheck_test.go -- TestCapabilityGatedTestsHaveANativeVMHome -->

### OPEN Behaviors

| Value | Description |
|-------|-------------|
| `send-unknown-capability` | Add unknown capability (code 66) to OPEN |
| `inspect-open-message` | Validate received OPEN against expectations |
| `send-unknown-message` | Send unknown message type (255) after OPEN |
| `drop-capability` | Remove a capability from ze-peer's OPEN response |
| `add-capability` | Add a capability to ze-peer's OPEN response |
| `router-id` | Send an explicit BGP Identifier instead of the mirrored one |
| `hold-time` | State the Hold Time of the OPEN body (`seconds=<N>`) |
| `graceful-restart` | State ze-peer's own Graceful Restart capability (`restart-time=<N>`, `family=`, `forward-state=`) |
| `llgr` | State ze-peer's own Long-Lived Graceful Restart capability (`stale-time=<N>`, `family=`, `forward-state=`) |
| `paths-limit` | State ze-peer's own PATHS-LIMIT entries (`family=`, `limit=<N>`) |
<!-- source: internal/test/peer/expect.go -- parseOptionConfig; internal/test/peer/open.go -- buildOpen -->

### Sender Facts (hold-time, graceful-restart, llgr, paths-limit)

Each of these four values states a fact about ZE-PEER that a receiver acts on.
They exist because a mirror asserts sameness: mirrored, each one made ze read its
own configuration back out of the peer's OPEN and believe the peer had said it.

```
option=open:value=hold-time:seconds=<N>
option=open:value=graceful-restart:restart-time=<N>[:family=<f>[,<f>]][:forward-state=true|false]
option=open:value=llgr:stale-time=<N>[:family=<f>[,<f>]][:forward-state=true|false]
option=open:value=paths-limit:family=<f>[,<f>]:limit=<N>
```

| Key | Range | What reads it |
|-----|-------|---------------|
| `seconds` | 0, or 3 to 65535 | `session_negotiate` takes the smaller of this and ze's `receive-hold-time`, then calls `timers.SetHoldTime`. RFC 4271 Section 4.2. 1 and 2 fail the file |
| `restart-time` | 0 to 4095 | `grStateManager.onSessionDown` arms the restart timer on it, `runPeer` gives it to `startEORTimer`, and `show bgp peer` prints it. RFC 4724 Section 3 gives the field 12 bits |
| `stale-time` | 0 to 16777215 | `enterLLGRLocked` arms one timer per family on it. RFC 9494 Section 3 gives the field 24 bits |
| `limit` | 0 to 65535 | The Max Paths field of the code 76 entry this peer advertises. `Session.filterPathsLimit` drops paths past it, so ze sends this peer no more than `limit` paths for one prefix |
| `family` | a family name, or a comma-separated list | The `<AFI, SAFI, Flags>` tuples of code 64, the 7-octet tuples of code 71, and the entries of code 76 |
| `forward-state` | `true` (default) or `false` | The F bit of every tuple. `onSessionReestablished` purges the stale routes of a family whose F bit is clear |

**The families are what make graceful restart act at all.** RFC 4724 Section 3
pairs the Restart Time with a tuple list, and `onSessionDown` builds its stale
family set from that list and returns without dispatching anything when the set is
empty. Ze's own code 64 carries the time and no tuples (`parseGRCapValue`), so
until ze-peer owned the value, `retain-routes`, `mark-stale` and `purge-stale`
were never dispatched in any test.

**A file that states none of them gets the harness's own defaults, never ze's.**

| Fact | Default |
|------|---------|
| Hold Time | 65535, the largest the two-octet field states. RFC 4271 Section 4.2 takes the smaller of the two, so the negotiated value stays ze's own and no existing test changes cadence. Only a `.ci` stating a smaller one opts in |
| Restart Time | 300 seconds, longer than any functional test runs |
| Long-Lived Stale Time | 600 seconds |
| Max Paths | 65535, which bounds nothing |
| Software version (code 75) | `ze-peer`. It has no option, because no session decision turns on it: `capability.Parse` holds no code-75 arm and the only reader is the offline `ze bgp decode` |
| Families, for all three capabilities | the families ze-peer's own OPEN advertises, read from its Multiprotocol capabilities. An OPEN carrying none is an `ipv4/unicast` speaker (RFC 4760 Section 8) |

Two refusals fail the file rather than dropping a line in silence:

- Stating a fact AND `add-capability` or `drop-capability` for the same code. The
  stated octets would win, and the typed line would be read, validated and then
  lost.
- Stating a fact for a capability **ze does not offer**. The capability SET
  ze-peer sends mirrors ze's, so the value would have no place to go. Configure
  ze to offer the capability, or drop the option.

<!-- source: internal/test/peer/expect.go -- parseOpenHoldTime, parseGracefulRestartDecl, parseLLGRDecl, parsePathsLimitDecl -->
<!-- source: internal/test/peer/open_capability.go -- ownedCapabilities, refuseUnofferedDeclarations -->

### BGP Identifier Control (router-id)

```
option=open:value=router-id:id=<a.b.c.d>
```

Ze-peer's default OPEN carries ze's own BGP Identifier with the last octet incremented, which is always a distinct, valid identifier. This option replaces it outright, so a test can present an identifier the default can never produce: `0.0.0.0`, or ze's own router-id (RFC 6286 Section 2.2 rejects both, the second only from an internal peer). A malformed or IPv6 value fails the file when it is read. It was ignored until 2026-09-08, which sent the DEFAULT identifier from a test that asked for an invalid one: the file then tested the valid identifier and passed.

<!-- source: internal/test/peer/expect.go -- parseOptionConfig "router-id"; internal/test/peer/open.go -- openIdentity -->

### Capability Control (drop-capability / add-capability)

Ze-peer mirrors the capability SET of ze's OPEN, so a `.ci` that says nothing about capabilities still negotiates whatever ze offers. It does NOT mirror the values that describe the SENDER. The AS, the BGP Identifier, the Hold Time, the Role (code 9), the Graceful Restart time, families and flags (code 64), the ADD-PATH directions (code 69), the Long-Lived Graceful Restart stale time and flags (code 71), the FQDN (code 73), the software version (code 75) and the PATHS-LIMIT entries (code 76) are each resolved from the test's own configuration and written into ze-peer's OPEN, because a mirror asserts sameness and every one of those facts is about the speaker rather than about the session. `ownedCapabilities` (`internal/test/peer/open_capability.go`) is the list: a code absent from it is mirrored by construction.

The `drop-capability` and `add-capability` options act on that reconciled OPEN at wire level, allowing tests to control exactly which capabilities ze-peer advertises.

**A capability the `.ci` states REPLACES the one ze-peer would have resolved.** An `add-capability` naming any resolved code -- 9, 64, 65, 69, 71, 73, 75 or 76 -- is sent as written and ze-peer adds no second capability of that code, so a file that drops a code and adds it back gets exactly what it asked for. Dropping code 65 removes the four-octet AS capability, which leaves the two-octet My Autonomous System field as the only carrier of the peer's AS: above 65535 that field can carry AS_TRANS alone, which is what RFC 6793 Section 3 defines for a speaker with no two-octet AS.

**An `add-capability:code=65` carrying four octets IS the AS declaration, and the My Autonomous System field follows it.** It outranks `option=asn` and the derivation, because it names the octets that reach the wire and RFC 6793 Section 4.1 makes those the octets a receiver reads. Letting the header field come from anywhere else would put two ASNs in one OPEN, which is the disagreement ze answers with NOTIFICATION 2/2 Bad Peer AS. A stated code 65 of any OTHER length declares no AS: it is a malformed capability the test is driving on purpose, so it is sent as written and the header field keeps the AS the rest of the configuration resolved. That is the one case where the two carriers differ, and they differ because the `.ci` asked.

**Drop a capability:**

```
option=open:value=drop-capability:code=<N>
```

Removes the capability with the given code from ze-peer's OPEN response. The peer will not see this capability in the mirrored OPEN.

**Add a capability:**

```
option=open:value=add-capability:code=<N>:hex=<value-bytes>
```

Adds a capability with the given code and hex-encoded value bytes to ze-peer's OPEN response.

| Key | Description |
|-----|-------------|
| `code` | Capability code (1-255), e.g., 65 for ASN4, 2 for route-refresh |
| `hex` | Hex-encoded capability value bytes (only for add-capability) |

**Use case: testing capability mode enforcement:**

When Ze is configured with `require` mode for a capability, it sends a NOTIFICATION if the peer lacks that capability. To test this, use `drop-capability` to make ze-peer omit the capability from its response:

```
# Test: Ze requires ASN4, ze-peer drops it → Ze should send NOTIFICATION
option=open:value=drop-capability:code=65
```

When Ze is configured with `refuse` mode, it sends a NOTIFICATION if the peer has a capability. To test this, the default mirror behavior already includes the capability, but `add-capability` can add capabilities not in the original OPEN:

```
# Test: Add a custom capability for refuse testing
option=open:value=add-capability:code=73:hex=067A652D626770
```

**Multiple overrides** can be combined:

```
option=open:value=drop-capability:code=65
option=open:value=drop-capability:code=2
option=open:value=add-capability:code=73:hex=067A652D626770
```

### UPDATE Behaviors

Routes ze-peer sends after the OPEN handshake, before it starts matching
expectations.

| Value | Keys | Description |
|-------|------|-------------|
| `send-default-route` | none | Send one UPDATE for `0.0.0.0/0` |
| `send-route` | `prefix`, `origin-as`, `next-hop`, and optionally `as-path`, `as-set`, `originator-id`, `cluster-list`, `label` | Send one UPDATE for one prefix. Repeat the line for more |
| `send-bulk` | `prefix`, `count`, `next-hop`, `origin-as`, and optionally `max-msg`, `eor` | Generate `count` sequential prefixes from `prefix` and send them as whole BGP messages |
<!-- source: internal/test/peer/expect.go -- parseOptionConfig "update"; parseBulkSpec -->

**Generating one oversize UPDATE (`max-msg`):**

```
option=update:value=send-bulk:prefix=10.0.0.0/24:count=16373:next-hop=10.0.0.1:origin-as=65001:max-msg=65535
```

`max-msg` caps one generated message, header included. It defaults to the RFC
4271 limit of 4096, and accepts 23 to 65535. Raise it to 65535 when the session
negotiates the Extended Message capability (RFC 8654), which ze advertises when
the peer carries `capability { extended-message enable }`. Prefixes that do not
fit one message spill into the next, so the example above is exactly one 65535
byte message: a 65516 byte body, the largest BGP permits.

Use it whenever a test needs an UPDATE too large to write as a hex literal. A
max-size message is 131070 hex characters, which is past the 64 KiB line limit
of both `.ci` scanners and past what a reviewer can check. It is also the only
way to hand the daemon a single oversize body: splitting the same prefixes
across several standard messages is a different input, because the daemon
decides per message.

A malformed key fails the test load rather than sending nothing. That is
deliberate: a spec that silently degraded to `count=0` would let a test asserting
a route was NOT forwarded pass because no route was ever offered.
<!-- source: internal/test/peer/inject.go -- InjectSpec.MaxMsgLen, buildV4Unicast -->

## Commands

```
cmd=<type>:key=value[:key=value...]
```

### API Commands

```
cmd=api:conn=<N>:seq=<N>:text=<command>
```

| Key | Description |
|-----|-------------|
| `conn` | Connection number (1-4) |
| `seq` | Sequence number within connection |
| `text` | API command text |

### Example

```
cmd=api:conn=1:seq=1:text=update text origin set igp nhop set 10.0.1.1 nlri ipv4/unicast add 10.0.0.0/24
```

### Process Commands (Foreground/Background)

For orchestrating multiple processes:

```
cmd=background:seq=<N>:exec=<command>[:stdin=<name>][:name=<handle>][:timeout=<dur>]
cmd=foreground:seq=<N>:exec=<command>[:stdin=<name>][:timeout=<dur>][:exit=<N>]
cmd=stop:seq=<N>:name=<handle>[:signal=kill|term]
```

| Key | Description |
|-----|-------------|
| `seq` | Execution order (lower first) |
| `exec` | Command to execute |
| `stdin` | Stdin block name to pipe |
| `timeout` | Foreground test budget, or background process lifetime (e.g., `10s`). |
| `exit` | Exit code asserted for **this** command (0..255). See below. |
| `name` | Handle for a background process, so a later `cmd=stop` can target it. |
| `signal` | `cmd=stop` only: `kill` (SIGKILL, default) or `term` (SIGTERM). |

Markers may appear in any order; each value runs to the next key in the table
above, whichever key that is.

**The table is the whole vocabulary. Any other `:<word>=` on a `cmd=` line fails
the file**, naming the key, the accepted set and the line. The scan reads the
whole line, `exec=` included, because that is exactly where a key the parser
does not read ends up: a value runs to the next KNOWN key, so an unknown one is
swallowed into the value before it rather than dropped.
`cmd=foreground:seq=2:exec=ze -:stdin=ze-bgp:timeout=15s:env=ZE_FWD_WRITE_DEADLINE=10s`
parsed with `timeout="15s:env=ZE_FWD_WRITE_DEADLINE=10s"`, so the line got
neither the timeout it declared nor the variable, and every assertion still
passed. Set an environment variable with `option=env:var=<name>:value=<value>`.

One consequence: a command carrying a `:<word>=` span of its own cannot be
written on a `cmd=` line. Put it in a `tmpfs=` script and run the script.

`timeout=` must be a Go duration (`10s`, `1m30s`). Both readers of the value
keep their own default when it does not parse, so it is refused here instead.
<!-- source: internal/test/runner/record_parse_cmd.go -- cmdExecKeys, cmdStopKeys, parseCmdExec -->
<!-- source: internal/test/runner/record_parse_keys.go -- checkMarkerKeys -->
<!-- test: internal/test/runner/record_parse_cmd_test.go TestCmdUnknownKeyRefused, TestCmdUnknownKeyNotSwallowedIntoExec -->

**Background:** Starts and keeps running until its timeout, an explicit stop, or test completion.
**Foreground:** Setup commands finish before the next step. A `ze` daemon starts
without blocking later steps. Its peers or observer determine when teardown starts.
**Stop:** Terminates a named background process mid-test (see below).

When an embedded observer sends `request shutdown`, teardown gives that daemon
its bounded self-stop grace before stopping the other background processes.

<!-- source: internal/test/runner/runner_exec.go -- runOrchestrated, startBackgroundLifetime -->
<!-- source: internal/test/runner/runner_exec_util.go -- tmpfsRequestsDaemonShutdown, terminateAfterSelfExit -->

#### How an `exec=` value becomes argv

The runner splits the value on spaces and tabs, and a span inside `"` or `'`
stays ONE argument. Quote an argument that carries a space or a pipe, exactly as
you would in a shell:

```
cmd=foreground:seq=1:exec=ze cli -c "show config dump - | json":stdin=config
```

`-c` receives `show config dump - | json`, and the quotes do not reach the
process. Three limits apply:

- A backslash escape is not handled. `\"` is a backslash and a quote character,
  never a literal quote inside an argument.
- An unbalanced quote fails the test with `unclosed quote in ...`.
- No shell runs, so `|`, `>` and `$HOME` are ordinary characters inside an
  argument. The runner expands `$PORT` and `$PORT2` and nothing else, and
  `ze-test fixture` expands the environment in its own arguments.

One splitter serves every suite, so a `cmd=` line produces the same argv
wherever it runs.

**Provenance:** until 2026-09-02 the `.ci` runner split the value on whitespace
alone while the parse suite honored quotes. The example above was already
published here, so four `test/ui` tests were written against it. `-c` received
only the first word of the quoted command, `ze cli` fell through to its SSH
client, and each test failed with `no credentials for 127.0.0.1:2222`.

<!-- source: internal/test/runner/runner_exec_util.go -- splitCommand -->
<!-- source: internal/test/runner/runner_exec.go -- runExecCommands argv construction -->
<!-- source: internal/test/runner/parsing.go -- runOneCommand, the parse suite's own execution -->
<!-- source: internal/test/fixture/fixture.go -- Run, os.ExpandEnv over the fixture's own arguments -->
<!-- test: test/runner/exec-quoted-argument.ci -- a quoted argument carrying a pipe reaches the callee as one argv element -->

#### `cmd=stop` -- terminate a background process mid-test

A background process started with `name=<handle>` can be stopped at a chosen step
by `cmd=stop:seq=<N>:name=<handle>`. The runner looks the process up, signals it,
and **waits for it to exit before the next step runs**, so a later step can
deterministically observe what happens after the process dies (e.g. `show vpn
ipsec sa` emptying once an IKE responder is killed and DPD fires).

- `signal=kill` (default) sends **SIGKILL**: the process gets no chance to flush or
  send a protocol teardown. This is the choice for liveness/dead-peer tests where
  the peer must go **silent** (a clean shutdown would take a different code path).
- `signal=term` sends **SIGTERM**, escalating to SIGKILL if the process does not
  exit within the teardown grace period -- a graceful stop.

Fail-closed: a `cmd=stop` naming a process that was never started (no matching
`name=`) **fails the test** with a clear error; it never silently no-ops, and it
can only ever signal a process the runner itself started (never an arbitrary PID).
Teardown still kills every remaining background process, and tolerates one the stop
step already reaped.

<!-- source: internal/test/runner/record_parse_cmd.go -- parseCmdExec (name=), parseCmdStop -->
<!-- source: internal/test/runner/runner_exec_util.go -- stopNamedBackground, stopBackgroundProcess -->
<!-- source: internal/test/runner/runner_exec.go -- modeStop step + namedBg registration -->
<!-- test: internal/test/runner/record_parse_cmd_test.go TestParseStopBackgroundDirective, TestParseAndAdd_StopDirective -->
<!-- test: internal/test/runner/runner_stop_test.go TestStopBackgroundKillsNamedProcess, TestTeardownToleratesStoppedProcess -->
<!-- test: test/runner/stop-background.ci -- full parse->start->stop->assert wiring proof -->

**Provenance:** added by spec-fixit-runner-kill-background to unblock end-to-end
peer-death observation (the deleted `test/ipsec/ipsec-dpd-timeout.ci` needed it).

#### `exit=` vs `expect=exit:code=` (per-command vs file-level)

`expect=exit:code=` is **file-level**: `Record.ExpectExitCode` is a single value
(a later `expect=exit:code=` silently overwrites an earlier one) and the runner
compares it against `lastQuickZeErr` -- the exit status of the **last** quick-exit
`ze` command in the file. A file that runs several `ze config validate` commands
therefore asserts only the final one; every earlier command can exit with any code
and the test still passes.

Use `exit=` on the `cmd=` line to assert a specific command's own exit code. It is
checked the moment that command finishes, and names the offending `seq` on failure:

```
cmd seq=2 (ze config validate -): expected exit code 1, got 0
```

Prefer `exit=` whenever a file runs more than one quick-exit `ze` command. A
"quick-exit `ze` command" is any foreground `ze` whose verb is not a daemon verb
(`hub`, `start`, `cli`, `monitor`) and which has no config-file argument or
`--web` flag.

#### Every stream assertion is FILE-level, over ONE buffer

This section describes the **generic** runner, which is every suite except
`test/parse`. The parse suite scopes each assertion to one command; see "The
parse suite reads its own dialect" below.

`expect=stdout:`, `expect=stderr:`, `reject=stdout:` and `reject=stderr:contains=`
all read `Record.ClientOutput`, which the runner builds ONCE at the end of the
test as the concatenated stdout AND stderr of every command in the file. Two
consequences, and an author who misses either writes an assertion that cannot
mean what it says:

- **There is no per-command scope.** `expect=stdout:contains=` can be satisfied
  by a different command than the one it sits under, and a reject trips on any
  command's output. A file that asserts a needle PRESENT for one command and
  ABSENT for another asserts two contradictory things about one string.
- **The stream name selects nothing** for `contains=`. Only `pattern=` on
  `expect=stderr:` / `reject=stderr:` reads stderr alone, through
  `validateLogging`.

So a negative assertion belongs in a file whose every command may satisfy it.
Split the file otherwise: `test/plugin/vpp-doctor-hugepages.ci` and
`vpp-doctor-hugepages-quiet.ci` are one scenario in two files for exactly this
reason, as are `test/appliance/no-install-appliance.ci` and
`appliance-help-not-deprecated.ci`, and each says so at the top. Also see
`test/vrrp/vrrp-doctor-quiet.ci`, a single-command file so its reject is
meaningful.
<!-- source: internal/test/runner/runner_exec.go -- rec.ClientOutput = clientStdout.String() + clientStderr.String() -->
<!-- source: internal/test/runner/runner_output_assert.go -- checkOutputAssertions -->
<!-- source: internal/test/runner/runner_exec.go -- quickZe branch, per-command exit assertion -->
<!-- source: internal/test/runner/record_parse_cmd.go -- parseCmdExec, markerExit -->
<!-- source: internal/test/runner/record.go -- RunCommand.ExitCode -->

<!-- test: internal/test/runner/record_newformat_test.go TestParseCmdExec -- exit= parsing, marker order, 0..255 bounds -->
<!-- test: test/vrrp/vrrp-config-invalid.ci -- 11 rejections, each asserted via exit=1 -->
<!-- test: test/vrrp/vrrp-doctor-quiet.ci -- single-command file so reject=stdout is meaningful -->

**Known gap:** 108 quick-exit `ze` commands across 50 `.ci` files predate `exit=`
and are still unasserted (their `expect=exit:code=` never reaches them). Arming
them may surface real defects; tracked in `plan/known-failures/`.

#### The parse suite reads its own dialect

`test/parse` runs under `ParsingTests`, a second parser with its own execution
model. Two differences are load-bearing for an author.

**Every assertion is scoped to one command.** It is checked against the stdout
and stderr of the `cmd=` line directly above it, not against a file-level
buffer. So a needle asserted present under one command and absent under another
means what it says here, and does not need the file split the section above
describes. An assertion that appears before the first `cmd=` has nothing to
assert against and **fails the file**; `expect=stderr:contains=` is the one
exception, because a `.ci` holding an inline config and no command at all is a
legacy negative test whose expected error it carries.

**The dialect is a subset of the generic vocabulary, and nothing else parses.**
A directive no arm reads **fails the file at discovery**, naming the directive,
the file, and every directive the suite does read. It is the same rule as
"Unknown Keys Are a Parse Error" above, applied to the whole line rather than to
a key inside it.

| Read by the parse suite |
|---|
| `cmd=`, `expect=exit:code=` |
| `expect=stdout:contains=`, `expect=stdout:pattern=` |
| `expect=stderr:contains=`, `expect=stderr:pattern=` |
| `reject=stdout:contains=`, `reject=stdout:pattern=`, `reject=stderr:pattern=` |
| `option=skip-os:value=`, `option=env:` |

Two spellings this suite once had are **deleted**, not aliased. Both existed
only here, and both are what two parsers over one corpus costs:

- `expect=stdout:regex=` is now `expect=stdout:pattern=`. The generic parser
  reads `pattern=` and refuses `regex=`, so the gates that walk the whole corpus
  with it could not read three `test/parse` files at all.
- `expect=stdout:not:contains=` is now `reject=stdout:contains=`, which this
  suite already read with the identical meaning. This is the one that mattered:
  the generic parser splits `not:contains=` at the `:contains=` key boundary and
  drops the bare `not`, so one written line meant "must be absent" to the suite
  that runs `test/parse` and "must be present" to every gate that reads it.

**`cmd=...:timeout=<duration>` is honored**, and a bare `ze -` runs as
`ze start <file>`, the same translation the generic runner makes. Substituting
the path in place produced `ze <path>`, which is not a command: the daemon
answered `unknown command: <path>` with its usage and exit 1 before reading a
line of config, so a file could assert `expect=exit:code=1` and be satisfied by
the usage error.

<!-- source: internal/test/runner/parsing.go -- ciDirectives, ciFileParser.line, runOneCommand -->
<!-- test: internal/test/runner/parsing_test.go -- TestParseCIRefusesUnknownDirective, TestParseCIAssertionsDiscriminate, TestParseCICorpusReadsUnderTheGenericParser -->

**Daemon readiness (`ze` only):** a `ze` daemon launched **either** foreground or
background is told (via `ZE_READY_FILE`) to write `daemon.ready` once startup
completes, and the runner publishes its PID to `daemon.pid` in the tmpfs directory.
Tests poll both files directly or through a compiled driver in
`internal/test/fixture` before signaling the daemon or asserting on it. The
readiness handshake is armed only for Ze daemons.
<!-- source: internal/test/runner/runner_exec.go -- process orchestration -->

### Example (Decode Test)

```
stdin=payload:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C...
cmd=foreground:seq=1:exec=ze-test decode --family ipv4/unicast -:stdin=payload
expect=json:json={ "type": "update", ... }
```

### Example (Multi-Process)

```
stdin=peer:terminator=EOF_PEER
option=asn:value=65000
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER

stdin=ze-bgp:terminator=EOF_CONF
peer test-peer { remote { ip 127.0.0.1; } ... }
EOF_CONF

cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze bgp server -:stdin=ze-bgp:timeout=10s
```

### Example (Multi-Peer)

Tests needing two or more BGP peers use the same `$PORT` on different loopback
addresses. The `ze_bgp_tcp_port` override sets all peer ports uniformly, which
is correct because every peer listens on `$PORT`.

```
# Source peer on 127.0.0.1 (default)
stdin=source:terminator=EOF_SOURCE
option=tcp_connections:value=1
action=send:conn=1:seq=1:hex=FFFF...
EOF_SOURCE

# Dest peer on 127.0.0.2 -- sink mode absorbs any UPDATE
stdin=dest:terminator=EOF_DEST
option=tcp_connections:value=1
EOF_DEST

cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=source
cmd=background:seq=2:exec=ze-peer --bind 127.0.0.2 --mode sink --port $PORT:stdin=dest
cmd=foreground:seq=3:exec=ze -:stdin=ze-bgp:timeout=20s
```

Each ze-peer gets independent output capture and `WaitFor` synchronization.
The runner waits for each peer's "listening on" message before starting the
next command.

On Linux, 127.0.0.2 works automatically (127.0.0.0/8 routes to lo). On macOS
and FreeBSD, the test runner adds loopback aliases via the `SIOCAIFADDR` ioctl.

IPv6 works differently, because a host carries exactly one IPv6 loopback
address. A fixture that needs a second one uses `fd00::2`, which is unique-local
(RFC 4193) and never globally routable. `./le setup install` adds it, and
`./le setup check` reports whether it is there. The runner never adds it:
the ioctl returns EPERM to an unprivileged process, and `./le verify current mode full` runs as
an ordinary user. A test that binds an address this host does not carry fails at
once with `loopback_address_missing` and the command to run, rather than timing
out on a bind that could not succeed.

The check reads the three places a fixture names an address it binds. One is
`ze-peer --bind <ip>` on a `cmd=` line. The second is `connection { local { ip
<addr> } }` in the config the fixture embeds: Ze sends from that address and
listens on it when `accept` is true, so the host must carry it too. The third is
`local-address <addr>;` in an ExaBGP-syntax config the exabgp-compat suite keeps
beside its `.ci` and names with `option=file:`. That suite has a parser and a
runner of its own, so it calls the same scan through
`EnsureConfigFileBindAddresses` rather than through the embedded path.

A local address outside 127.0.0.0/8 and fc00::/7 is left alone. A
config-validation fixture names a routable one (`local { ip 192.0.2.1 }`), the
daemon exits before it binds anything, and `./le setup install` adds no such
address. The `local-link-local fe80::1` leaf that sits beside `local-address` in
an exabgp-compat config is left alone for the same reason.
<!-- source: internal/test/runner/loopback.go -- probe, error text, --bind, config-local and local-address scan -->
<!-- source: internal/test/cli/cmd_exabgp.go -- runOneExaBGPTest, the exabgp-compat preflight call -->
<!-- source: internal/test/runner/loopback_linux.go -- no-op on Linux for IPv4 -->
<!-- source: internal/test/runner/loopback_darwin.go -- SIOCAIFADDR on BSD -->
<!-- source: internal/le/setup/actions.go -- Answer -->

## Expectations

```
expect=<type>:key=value[:key=value...]
```

### BGP Wire Expectations

```
expect=bgp:conn=<N>:seq=<N>:hex=<hex-bytes>
expect=bgp:conn=<N>:seq=<N>:prefix=<hex-bytes>
expect=bgp:conn=<N>:seq=<N>:contains=<hex-bytes>
expect=bgp:conn=<N>:seq=<N>:ordered=<hex-bytes>
```

Validates the BGP wire message received: `hex=` matches the exact message,
`prefix=` the message start, `contains=` a substring anywhere in one message.
Within one `seq` group, `hex`/`prefix`/`contains` checks match in any order and
each consumes exactly one received message.

`ordered=` checks in a `seq` group form a strict FIFO subqueue for asserting
in-order delivery across message boundaries: the front needle must appear in
the received message, and one message may consume several consecutive needles,
each matched at an advancing offset (so order inside a packed message is
enforced too). Use `ordered=` instead of per-message `contains=` when the
sender may legally pack several NLRIs into one UPDATE (the forward rail's
bucket merge): per-message framing is not a property ze owes, but delivery
order is. A message whose content matches only a non-front needle consumes
nothing and is reported as a mismatch.
<!-- source: internal/test/peer/checker.go -- parseExpectRule, consumeMatches, consumeOrdered -->

These forms are peer-block directives (inside a `stdin=<name>:` block);
top-level `expect=bgp` lines support `hex=` only.

### JSON Expectations

```
expect=json:conn=<N>:seq=<N>:json=<json-object>
expect=json:json=<json-object>
```

Validates the decoded message matches expected JSON.

**Validation rules:**
- Parsed and compared field-by-field (key order independent)
- Volatile fields removed before comparison: `exabgp`, `ze-bgp`, `time`, `host`, `pid`, `ppid`, `counter`
- Neighbor normalization: `peer` ↔ `neighbor` treated as equivalent, `direction` field ignored
- All non-volatile fields must match exactly
<!-- source: internal/test/runner/runner_validate.go -- JSON comparison, volatile field removal -->

### Exit Code Expectations

```
expect=exit:code=<N>
```

Validates the foreground process exit code. A test whose ONLY assertion is
`expect=exit:code=0` is **accept-only** (weak) and is gated by a lint; see
[Assertion Strength](#assertion-strength-accept-only-tests-and-readback).

### An Unknown Directive or Key Is a Parse Error

Every directive the runner reads is declared: the `action=` word, the type after
it, and the keys inside it. A word or a key outside its declared set fails the
file at discovery, naming what was written, the accepted set, and the line.
Nothing is dropped in silence, and nothing is guessed.

```
line 12: unknown action "exepct" (accepts action, await, cmd, command, expect, http, option, reject, stream)
line 12: unknown expect type "stdoutt" (accepts bgp, command-error, event, exit, file, json, output, stderr, stdout, stream, syslog)
line 12: cmd=foreground: unknown key "env" (accepts exec, exit, name, seq, stdin, timeout)
```

The accepted set in each message is the list the parser gates on, so it cannot
describe a vocabulary the runner does not have. The line number is the line in
the FILE: comments, blank lines and whole `stdin=` and `tmpfs=` blocks are
consumed before the directives are parsed, and the refusal used to count only
the directives that survived, which named line 2 for a line that was number 69.
<!-- source: internal/test/runner/record_parse_vocabulary.go -- recordActions and the type lists each switch gates on -->
<!-- source: internal/test/tmpfs/tmpfs.go -- Line, the directive text with its file line number -->
<!-- test: internal/test/runner/record_parse_test.go TestUnknownDirectiveNamesLineAndAccepted, TestDirectiveVocabularyIsLive -->

The rule exists because a dropped key takes the whole assertion with it. An
`expect=stdout:not-contains=X` line recorded NO assertion and then passed
whatever the command printed. Ten such lines were live across seven `.ci` files
when the check was added, and one of them was the only assertion its test made.
<!-- source: internal/test/runner/record_parse_keys.go -- checkKeys, retiredKeys -->

Two spellings get a named message, because both are the right key somewhere
else: `not-contains=` belongs to `expect=file:`, and `!contains=` was the
`expect=stdout` spelling until stream non-containment moved to `reject=`.

### Stdout Expectations

```
expect=stdout:contains=<text>
expect=stdout:pattern=<regex>
```

Two modes:
- `contains=` -- substring match against stdout (multiple allowed, all must match)
- `pattern=` -- regex match against stdout (uses Go `regexp` syntax)

Stdout must NOT contain text is `reject=stdout:contains=`, below. A stream has
ONE negation spelling and it lives on `reject=`.

### Stderr Expectations

```
expect=stderr:pattern=<regex>
expect=stderr:contains=<text>
```

Two modes:
- `pattern=`: regex match against stderr (uses Go `regexp` syntax)
- `contains=`: substring match against stderr

`pattern=` reads the daemon's stderr alone. `contains=` reads the same combined
buffer every stdout assertion reads (see the scope note below), so it matches a
needle the command wrote to stdout. Reach for `pattern=` when the stream matters.

Stderr must NOT contain text is `reject=stderr:contains=`, below.

### Await (deterministic stderr fence)

```
await=stderr:contains=<text>[:timeout=<dur>]
```

Blocks the runner until the daemon's relayed stderr contains `<text>`, then tears
the daemon down. This is a deterministic replacement for a blind `time.sleep` that only
held the daemon open long enough for a line to appear. `timeout=` is an optional
Go duration (default 10s); on timeout the test fails with a precise message.
`<text>` follows the same rule as `expect=stderr:contains=` (the needle must not
contain a literal `:`).

Pair it with a matching `expect=stderr:contains=` so the line is both the fence
and the assertion. Use it for the reject-fence bucket: an external plugin whose
refusal aborts the daemon's plugin-startup coordinator
(`StartupCoordinator.PluginFailed`) leaves no in-daemon observer able to poll it,
so the relayed stderr line is the only non-plugin signal.

Choose a needle that is SPECIFIC to the plugin under test, not just a shared
phrase. Example (`test/plugin/as112-external-refuses.ci`): the bare "refusing to
start as an external plugin process" is emitted verbatim by three plugins, so the
needle includes the as112-only tail `-- the address-ownership registry`:

```
cmd=foreground:seq=1:exec=ze -:stdin=ze-bgp:timeout=15s
await=stderr:contains=refusing to start as an external plugin process -- the address-ownership registry
expect=stderr:contains=refusing to start as an external plugin process -- the address-ownership registry
```

<!-- source: internal/test/runner/await_stderr.go -- parseAwait / awaitDaemonStderr -->

### Syslog Expectations

```
expect=syslog:pattern=<regex>
```

Validates that captured syslog output matches the regex pattern. When any `expect=syslog:` line is present, the test runner automatically starts a UDP syslog server and injects `ze.log.backend=syslog` and `ze.log.destination=127.0.0.1:<port>` into the test environment.
<!-- source: internal/test/syslog/testsyslog.go -- UDP syslog server for tests -->

### File Expectations

```
expect=file:path=<rel>:exists=true
expect=file:path=<rel>:absent=true
expect=file:path=<rel>:contains=<text>
expect=file:path=<rel>:not-contains=<text>
expect=file:glob=<rel-pattern>:count=<N>
expect=file:glob=<rel-pattern>:contains=<text>
expect=file:glob=<rel-pattern>:not-contains=<text>
```

Validates files after the test process or peer sequence has completed. Paths and
glob patterns are relative to the test's own work directory, which is where every
child ran and where a daemon writes its artifacts. A test that declares `tmpfs=`
files gets them in that same directory, so the two spellings name one place. For
glob `contains`, at least one matched file must contain the text. For glob
`not-contains`, no matched file may contain it.

Use file expectations for post-run artifacts such as generated configs, pointer
files, and logs. Do not write shell just to inspect files.
<!-- source: internal/test/runner/runner_validate.go -- validateFileChecks -->

### Negative Expectations (reject)

```
reject=stderr:contains=<text>
reject=stderr:pattern=<regex>
reject=stdout:contains=<text>
reject=stdout:pattern=<regex>
reject=syslog:pattern=<regex>
reject=bgp:conn=<N>:pattern=<hex>
```

Inverse of `expect=` -- the test **fails** if the pattern matches. Used to verify that unwanted output (e.g., deprecated warnings, ERROR-level messages) does NOT appear.

| Type | Description |
|------|-------------|
| `reject=stderr:contains=<text>` | Fail if the combined output contains substring (the mirror of `expect=stderr:contains=`) |
| `reject=stderr:pattern=<regex>` | Fail if stderr matches regex |
| `reject=stdout:contains=<text>` | Fail if stdout contains substring |
| `reject=stdout:pattern=<regex>` | Fail if stdout matches regex |
| `reject=syslog:pattern=<regex>` | Fail if syslog output matches regex |
| `reject=bgp:conn=<N>:pattern=<hex>` | Fail if connection N of a ze-peer receives a frame carrying these wire bytes |
<!-- source: internal/test/runner/runner_exec.go -- stdout reject handling -->
<!-- source: internal/test/runner/runner_validate.go -- stderr/syslog reject handling -->
<!-- source: internal/test/peer/reject.go -- reject=bgp, the wire rejection -->

#### `reject=bgp` -- bytes a peer must never receive

`reject=bgp` is the only reject type ze-peer reads, because ze-peer is what sees
the wire. It goes inside the peer's stdin block, beside the `expect=bgp` lines.
The hex needle is matched on wire bytes at a byte BOUNDARY, which
`expect=bgp:contains=` is not: that one is a plain substring match over the hex
text, so it can also match at an odd nibble offset.

Five properties make it an assertion rather than a hope:

- It is never consumed. Every frame the peer's message loop reads is checked
  against it. The `option=linger` loop keeps checking after completion. Pair the
  rejection with `option=linger:value=true` to hold it open for the whole test.
- **A rejection found during linger RETRACTS the success token.** Under
  `option=linger` the peer prints its success token BEFORE the loop, because
  teardown is a kill and a post-run print can be lost. The verdict reads that
  token, so a rejection arriving afterwards has to withdraw it: the peer prints
  `ZE-PEER-REJECTED` on its own output, and the verdict reads the retraction
  after the token and lets it win. Without that channel a linger rejection was
  detected, returned, and then discarded, which made every negative assertion
  held open by linger vacuous.
  <!-- source: internal/test/peer/reject.go -- RejectionMarker, (*Peer).rejected -->
  <!-- source: internal/test/runner/peer_contract.go -- failedCheckPeers, peerRejectionMarker -->
- **Check mode only.** Sink and echo peers read every accepted connection
  concurrently against one checker, so a `conn=` could not select the session the
  frame arrived on. The runner refuses the file, and ze-peer refuses to start.
- The block MUST also carry an `expect=bgp:conn=<N>` on the same connection, and
  the runner refuses the file otherwise. That rule is necessary and it is not
  sufficient. It bounds how long the rejection is checked. It cannot say whether
  the forbidden bytes would have arrived inside that window.
  **The author owes the second half.** Send the delivery LAST on that connection,
  so a leak of the governed route arrives ahead of it. Or hold the session open
  with `option=linger`. A connection stops being read once its expectations
  complete, and its rejection then passes for the one reason a rejection must not.
- An odd-length or non-hexadecimal needle is a parse error, because a needle
  that can never match would pass for that same reason. The runner reads the
  line with ze-peer's own parser (`peer.ParseRejectRule`), so a typo fails the
  FILE rather than the peer: a rejection rejected inside ze-peer would stop it
  binding, and the runner would report that as a bind timeout.

```
stdin=external:terminator=EOF_EXTERNAL
option=tcp_connections:value=1
option=linger:value=true
# The delivery that makes the rejection an assertion rather than an empty session.
expect=bgp:conn=1:seq=1:contains=18C00002
# 180A0100 is 10.1.0.0/24, which NO_ADVERTISE forbids on any peer.
reject=bgp:conn=1:pattern=180A0100
EOF_EXTERNAL
```

## Assertion Strength: accept-only tests and readback

A test whose ONLY assertion is `expect=exit:code=0` is **accept-only** (weak): it
proves a config or command was ACCEPTED, never that it parsed to the CORRECT tree.
A parser that accepts `interval 300` but stores `0`, or silently drops a `source
0.0.0.0/0` block, still passes such a test green. This is the functional-suite
analog of the "count-only assertion" mistake class (`ai/rules/testing.md`).

A lint enforces that the class cannot GROW: `TestCIAcceptOnlyLint` walks every
`test/**/*.ci`, classifies each with the single accept-only predicate, and FAILS on
a NEW accept-only test that is neither strengthened nor annotated. Existing
accept-only tests are grandfathered in `test/.accept-only-baseline` (a sorted
allow-list that only shrinks; strengthening or annotating a test removes its line).
Correctly EXCLUDED (never weak): a test whose real check lives in a tmpfs `set -e`
script (e.g. `test/managed/auth-reject.ci`), and any `reject=` test.
<!-- source: internal/test/runner/accept_only.go -- isAcceptOnly, acceptOnlyAnnotation, the accept-only predicate + baseline -->
<!-- source: internal/test/runner/accept_only_lint_test.go -- TestCIAcceptOnlyLint, TestCIAcceptOnlyLintFlags/Allows -->

### Strengthen with a readback

Add a second `cmd=` that dumps the parsed tree and assert a representative value
with `expect=stdout:contains=` / `pattern=`. `show config dump -` reads a config
from stdin and answers the stored tree, and `| json` renders it, so the assertion
observes the parsed VALUE, not just that parsing did not error.

```
cmd=foreground:seq=1:exec=ze config validate -:stdin=config:exit=0
cmd=foreground:seq=2:exec=ze cli -c "show config dump - | json":stdin=config-dump
expect=exit:code=0
expect=stdout:pattern="interval": "300"
```

Two gotchas, both load-bearing:

- **`ze config dump` requires a `bgp { }` block** (it resolves the full BGP tree)
  where `ze config validate` does not. To keep proving that a subsystem-only config
  validates, keep the original `validate` step on the subsystem-only stdin block and
  run the `dump` readback against a second stdin block that prepends a minimal `bgp
  { router-id ... }`.
- **A needle containing `:` followed by something shaped like `key=` must use
  `pattern=`.** ~~Only `json=`/`text=`/`hex=`/`pattern=` preserve colons, and
  `contains=` truncates at the first colon.~~
  - **Corrected 2026-07-30.** That claim was true until `ParseKVPairs` became
    boundary-aware. `contains=` now keeps an ordinary colon, so
    `contains=error: no such peer` asserts the whole sentence.
  - What still splits is a colon that introduces a real key token: a letter,
    then letters, digits, `-` or `_`, then `=`. That split is deliberate,
    because it is how the engine-step form `contains=aes-cbc:timeout=25` keeps
    working. So `contains=note:level=high` still splits at `:level=`, and it is
    the one shape that needs `pattern=`.
  - The old behavior was not harmless while it lasted. A sweep on 2026-07-29
    found **203 assertions across 15 suites** silently reduced to the text
    before their first colon. The re-armed assertions then exposed a security
    test (`test/appliance/appliance-push-image-escape.ci`). That test had never
    once executed the path-traversal guard it was named for.
- **Keep a `pattern=` needle free of the substrings `json=`, `text=`, and `hex=`.**
  `ParseKVPairs` extracts a complex-key value by `strings.Index` of the first such
  marker, so a needle that itself contains one of them is mis-split. None of the
  readback needles above contain these markers. This note is forward guidance for
  new needles.
<!-- source: internal/test/ci/ciformat.go -- ParseKVPairs, complexKeys (json/text/hex/pattern consumed whole), splitOnKeyBoundary (an ordinary colon stays in the value; only a colon introducing a key= token splits) -->
<!-- source: internal/component/config/cli/cmd_dump.go -- cmdDump reads stdin and prints the parsed tree -->

### Annotate when a unit test already covers the value

When a unit test already asserts the parsed value, a readback would duplicate it.
Mark the test accept-only instead, with a comment naming the covering test:

```
# accept-only: md5 { password; ip } value-parsing is unit-covered by
# TestParsePeerMD5FieldsParsed (internal/component/bgp/reactor/config_test.go).
```

The marker is a comment line whose content is `accept-only:` followed by a
non-empty reason. A file carrying it is allowlisted by the lint without a baseline
entry. Keep the reason greppable and truthful (name the covering test).
<!-- source: internal/test/runner/accept_only.go -- acceptOnlyAnnotation, hasAcceptOnlyAnnotation -->

## Actions

```
action=<type>:key=value[:key=value...]
```

### Notification

```
action=notification:conn=<N>:seq=<N>:text=<message>
```

Sends NOTIFICATION with shutdown message.

### Send Raw

```
action=send:conn=<N>:seq=<N>:hex=<hex-bytes>
```

Sends raw bytes to peer.

### Rewrite Config File

```
action=rewrite:conn=<N>:seq=<N>:source=<tmpfs-file>:dest=<config-file>
```

Copies a tmpfs file over the daemon's config file. Used with `action=sighup` to test config reload.

It also writes a marker a second process can wait on, which is its other use:
`test/reload/config-apply-ordering-address-swap.ci` copies one file to
`session-returned.txt` once the restarted BGP session has re-announced its
routes, and the test's own driver waits for that file before it signals the
daemon. When the rewrite is the LAST item the peer block queues, the exchange
is over and the peer completes on it, exactly as `action=sighup` and
`action=sigterm` do. Without that the peer kept matching, and the daemon's
shutdown NOTIFICATION arrived against an empty expectation list and failed the
test as a message mismatch.

| Key | Description |
|-----|-------------|
| `conn` | Connection number triggering the rewrite |
| `seq` | Sequence number (after matching messages) |
| `source` | Source file name in tmpfs |
| `dest` | Destination file name in tmpfs (usually `ze-bgp.conf`) |

### Send SIGHUP

```
action=sighup:conn=<N>:seq=<N>
```

Sends SIGHUP to the daemon process. Reads PID from `daemon.pid` in the tmpfs directory (written automatically by the test runner).

| Key | Description |
|-----|-------------|
| `conn` | Connection number triggering the signal |
| `seq` | Sequence number (after matching messages) |

### Send SIGTERM

```
action=sigterm:conn=<N>:seq=<N>
```

Sends SIGTERM to the daemon process. Reads PID from `daemon.pid` in the tmpfs directory (written automatically by the test runner). After sending SIGTERM, the connection is expected to close (daemon shuts down gracefully).

| Key | Description |
|-----|-------------|
| `conn` | Connection number triggering the signal |
| `seq` | Sequence number (after matching messages) |

## HTTP Checks

HTTP checks validate web endpoint responses after all `cmd=` processes have started.
Executed in `seq` order with automatic retry on connection errors (server starting up).

### Assertion Checks (get/post)

```
http=get:seq=N:url=URL:status=CODE[:contains=TEXT][:bodyfile=PATH][:header=NAME: VALUE]
http=post:seq=N:url=URL:status=CODE[:contains=TEXT][:bodyfile=PATH][:sendfile=PATH][:content-type=TYPE][:header=NAME: VALUE][:insecure-tls=true]
```

| Key | Required | Description |
|-----|----------|-------------|
| `seq` | Yes | Execution order (>= 1, lower first) |
| `url` | Yes | Request URL (supports `$PORT` and `$PORT2` substitution) |
| `status` | Yes | Expected HTTP status code |
| `contains` | No | Expected body substring |
| `bodyfile` | No | Path to file with expected body (exact match, resolved relative to `.ci` file) |
| `sendfile` | No | Path to file sent as POST request body, resolved from tmpfs first |
| `content-type` | No | Request body content type for `sendfile`, defaults to `application/json` |
| `header` | No | Request header in `Name: Value` wire form. **Repeatable** -- the only key that can appear more than once on one line |
| `insecure-tls` | No | Set `true` for self-signed local HTTPS endpoints |
<!-- source: internal/test/runner/runner_validate.go -- executeOneHTTPCheck -->

#### Request Headers (`header=`)

`header=` takes the header exactly as it appears on the wire, `Name: Value`.
Repeat `header=` to set several headers on one check. It works on `get`, `post`,
and `wait`.

```
http=post:seq=1:url=http://127.0.0.1:$PORT/mcp:status=200:sendfile=call.json:header=MCP-Protocol-Version: 2026-07-28:header=Mcp-Method: tools/call:header=Mcp-Name: ze
```

| Behavior | Detail |
|----------|--------|
| Splitting | On the **first** colon only, so a value can contain colons (`header=Referer: http://127.0.0.1:8080/page`) |
| Whitespace | Trimmed around both name and value, so `header=Foo: bar` and `header=Foo:bar` are equivalent |
| Precedence | Applied **after** the `sendfile` default `Content-Type`, so an explicit `header=Content-Type: ...` wins |
| Repeats of one name | The first occurrence replaces the value. Each later occurrence adds one more value to that field |
| `Host` | Routed to the request's Host field, because `net/http` ignores a `Host` entry in the header map |
| Malformed | A `header=` value with no colon is a **parse error** naming the offending value, never a silent drop |
| Value stops at | The next known key marker (`:status=`, `:contains=`, the next `:header=`, ...), like every other key |
<!-- source: internal/test/runner/record_parse.go -- parseHTTP header scan loop -->
<!-- source: internal/test/runner/runner_validate.go -- applyCheckHeaders -->

Retries up to 20 times at 200ms intervals on transient connection errors (ECONNREFUSED, ECONNRESET, EOF).
Non-connection errors (wrong status, missing content) fail immediately.

### Readiness Polls (wait)

```
http=wait:seq=N:url=URL:status=CODE[:contains=TEXT][:header=NAME: VALUE][:timeout=DUR]
```

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `seq` | Yes | - | Execution order (>= 1) |
| `url` | Yes | - | Request URL |
| `status` | Yes | - | Expected HTTP status code |
| `contains` | No | - | Expected body substring |
| `header` | No | - | Request header in `Name: Value` form, repeatable (see above) |
| `timeout` | No | `15s` | Poll timeout duration |
<!-- source: internal/test/runner/runner_validate.go -- executeOneHTTPWait -->

Unlike assertion checks, `wait` retries on **all** failures: connection errors, wrong status codes,
and content mismatches. Polls at 500ms intervals until the condition is met or the timeout expires.
Wait checks run **before** assertion checks, making them suitable for waiting until a server has
populated data (e.g., routes injected by an async plugin).

### Example

```
cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=background:seq=2:exec=ze -:stdin=ze-bgp

# Wait until routes are available before checking graph output
http=wait:seq=1:url=http://127.0.0.1:$PORT2/lg/graph?prefix=10.10.1.0/24&mode=aspath&format=text:status=200:contains=AS2914:timeout=15s

# Exact SVG match against reference file
http=get:seq=1:url=http://127.0.0.1:$PORT2/lg/graph?prefix=10.10.1.0/24&mode=aspath:status=200:bodyfile=expect/graph.svg

# Substring check
http=get:seq=2:url=http://127.0.0.1:$PORT2/lg/graph?prefix=10.10.1.0/24&mode=nexthop&format=text:status=200:contains=egress
```

## Sleeps and their justification markers

<!-- source: internal/le/doc/wiring -- checkSleepJustification -->
<!-- source: internal/le/hookruntime/writeedit.go -- writeCISleep -->

Every `time.sleep(` in a live `.ci` carries a marker comment in the form
`// sleep(<kind>): <reason>`, on one `#` comment line directly above the sleep,
indented to match it exactly. The embedded `.ci` observer body is indentation
sensitive. Two producers enforce the marker: `checkSleepJustification` in
`internal/le/doc/wiring` (run by `./le doc wiring`, scoped to changed `.ci`
files, listing every unjustified `file:line` and exiting 1), and `writeCISleep`
in `internal/le/hookruntime/writeedit.go`, which blocks a Write or Edit that
introduces an unmarked sleep.

The kinds are a closed set, and each one owes a different reason:

| Kind | What the reason states |
|------|------------------------|
| `poll-interval` | The real condition the enclosing loop breaks or returns on. This is already a deterministic wait; the sleep is only its granularity |
| `timer` | The delay itself IS the behavior under test, and the mechanism plus where its period is set |
| `timeout-under-test` | The fixed internal timeout the sleep waits out, which the test asserts on |
| `needs-linux` | A dataplane effect (tc, qdisc, nft, kernel FIB) with no readback in the driver, convertible only after a QEMU run |
| `no-signal` | The awaited effect exposes no queryable state to this driver, and what is held until instead |

A free-text `// settle` comment is insufficient: it names no mechanism a reader
can check. "The tracker pushes live carrier once a second" is a reason a later
reader can overturn; "needs a moment" is the shape that makes a deliberate timer
and a guessed duration the same line of code.

A separate ratchet caps how MANY sleeps exist: the total `time.sleep(` count
across `test/**/*.ci` may not exceed the committed baseline in
`test/.ci-sleep-baseline`, and `./le doc wiring` fails when it does. The markers
cap how many are unexplained.

## The compiled observer API

<!-- source: internal/test/fixture/fixture.go -- Register, Run, Observe, observeConfigured, Dispatch, Poll, ReportFailure -->

Compiled `.ci` observers live under `internal/test/fixture`. They use
`pkg/plugin/sdk` for the five-stage plugin protocol (`Plugin.Run` owns it) and
the local `fixture` package for registration, dispatch, polling and failure
reporting.

| Function | Purpose |
|----------|---------|
| `fixture.Register(name, driver)` | Register one compiled fixture command |
| `fixture.Run(args)` | Dispatch `ze-test fixture <name> [args...]` |
| `fixture.Observe(...)` | Connect through the SDK, complete startup, run the scenario after all plugins are ready, then request shutdown |
| `observeConfigured(...)` | Install callbacks before startup, then run the same observer lifecycle. It is unexported, so only a fixture in this package calls it |
| `fixture.Dispatch(...)` | Send one command and decode its JSON answer into a Go value |
| `fixture.Poll(...)` | Retry a predicate until success, exhaustion, or context cancellation |
| `fixture.ReportFailure(err)` | Emit the `ZE-OBSERVER-FAIL` sentinel `checkObserverSentinel` (`internal/test/runner/runner_validate.go`) detects |
| `sdk.Plugin.DispatchCommand(...)` | Send a typed command request through the plugin connection |

`fixture.Poll` around `fixture.Dispatch` is the payload-predicate wait: it blocks
until the observed payload matches, within a bounded attempt count. It is what
replaces a `time.Sleep` followed by a one-shot assertion.

`fixture.Observe` can request a clean daemon shutdown even after an assertion
failed, so the daemon's exit code does not prove the observer's assertion. A
failing observer returns an error, which `fixture.Run` hands to
`fixture.ReportFailure`.

**A fixture-driven `.ci` needs no `bgp` block and no `ze-peer` to get a daemon
that stops.** `request shutdown` reaches a reactorless daemon through the
shutdown callback the daemon wires beside its plugin server, before any plugin
can dispatch, so a BFD-only or DHCP-only configuration stops on the fixture's
request like a BGP one. Adding a BGP peer only to make the daemon stoppable adds
a second protocol to the test's failure surface: `test/bfd/bfd-detection-interval.ci`
was red for exactly that reason until 2026-09-07.
<!-- source: cmd/ze/hub/main.go -- apiServer.SetShutdownFunc; internal/component/plugin/server/system.go -- handleDaemonShutdown -->

**`option=asn` moves both AS numbers.** The AS is resolved once and written into
the two-octet My Autonomous System field AND the four-octet AS capability, so the
two disagree only where a `.ci` states a malformed capability 65 itself (see
"Capability Control" above). RFC 6793 Section 4.1 states the precedence a receiver
applies: it "MUST use the AS number encoded in the Capability Value field of the
'support for four-octet AS number capability' in lieu of the 'My Autonomous
System' field of the OPEN message". So a half-applied option leaves the
capability carrying ze's own AS, ze reads THAT, finds an AS the session is not
configured for, and answers OPEN Message Error / Bad Peer AS under RFC 4271
Section 6.2. The refusal is RFC 4271's; RFC 6793 decides which field ze read.
<!-- source: internal/test/peer/open.go -- buildOpen; internal/component/bgp/reactor/peer.go -- openAdvertisedAS -->

**The sentinel is written where the scenario fails, BEFORE `request shutdown`.**
The runner reads it out of the DAEMON's stderr, and the daemon relays a plugin's
stderr only while both processes live (`internal/component/plugin/process`,
`relayStderrFrom`). `fixture.Run` reports the same error after `Plugin.Run`
returns, which is after the daemon was asked to stop, so that line reaches the
runner only when it wins the race with the shutdown. Measured 2026-09-02: an
observer asserting a route that can never arrive still passed
`test/plugin/rpki-group-action.ci`, and three plugin cases (`fib-table`,
`metrics-name-show`, `modify-increment-localpref`) passed with a failing
observer. `fixture.ReportFailure` emits the FIRST failure only, so the report at
the failure site and the one on the way out are one line.
<!-- source: internal/test/fixture/fixture.go -- observeConfigured, ReportFailure -->
<!-- source: internal/component/plugin/process/process.go -- relayStderrFrom -->

**A driver's working directory is the per-test directory, so a driver names its
files relatively and `fixture.Run` refuses to start in the checkout.** The
runner creates one directory for each test, gives it to every child of that test
and removes it at the end (`Record.WorkDir`, `internal/test/runner/runner_exec.go`),
so `daemon.ready`, `testkey` and `<case>.conf` are written where the test can
find them and nothing survives the run. The same names in the checkout land
beside tracked source: on 2026-09-05 a run left two ed25519 private keys and
five config files at the repository root, none of them ignored, so a `git add -A`
from any session would have committed a private key. `fixture.Run` therefore
reads its own working directory first and exits 1 with the `ZE-OBSERVER-FAIL`
sentinel when that directory holds the ze `go.mod`
(`sessionpath.IsRepoRoot`). A driver that needs the checkout reads
`$ZE_REPO_ROOT`, which the runner exports; it never reads the working directory
for one.
<!-- source: internal/test/fixture/fixture.go -- Run, refuseRepoRoot -->
<!-- test: internal/test/fixture/fixture_test.go -- TestRunRefusesADriverStartedInTheCheckoutRoot, TestRunDispatchesADriverStartedOutsideTheCheckoutRoot -->

An observer assertion is therefore worth only what its wait is worth: a value
that a plugin fills asynchronously (the Adj-RIB-In after RPKI validation, the
SPF table after the first run, a session that drops when the peer completes)
needs `fixture.Poll` around the read, never a fixed settle before it.

## Engine Steps

Engine steps drive a live daemon through CLI dispatch, first-class in `.ci`
instead of an embedded Python observer. The runner serializes the parsed steps
to `engine-steps.json` in the test tmpfs; the `.ci` declares the executor as an
external plugin (`run "ze-test engine-steps ./engine-steps.json"`), which runs
the steps from `OnAllPluginsReady` and reports failures via the
`ZE-OBSERVER-FAIL` sentinel the runner gates on.

```
command=<cli command text>
stream=<monitor command text>
expect=output:<predicate>[:timeout=<dur>]
expect=event:namespace=<ns>:name=<name>[:timeout=<dur>]
expect=stream:<predicate>[:timeout=<dur>]
expect=command-error:contains=<text>
```

`command=`/`stream=` keep their full raw text (colons included). `expect=output`
re-dispatches the most recent `command=` until its predicate holds or the
timeout expires; `expect=stream` matches delivered `stream=` events;
`expect=event` matches a delivered event by its `(namespace, name)` identity.

### expect=event

The executor declares ONE startup event subscription, derived from the
`expect=event` steps of the file itself, and asks for enveloped delivery. A `.ci`
therefore names each event once, in the step that waits for it, and nothing else
declares the subscription.

A startup subscription rides the `ready` RPC, so it is registered before any
plugin's `OnAllPluginsReady` runs. That is what lets a step observe the FIRST
delivery of an event the daemon emits while it starts. `test/ipsec/ipsec-sa-installed.ci`
is the case that forced it: the IKE engine emits `vpn-ipsec`/`sa-up` from its
startup peer reconciliation, so a subscription dispatched from the step could
only ever see a SECOND establishment, and a test with no re-negotiation has none.

Enveloped delivery wraps each event with its `(namespace, event)` identity
(`rpc.EventEnvelope`), which is what tells one subscribed event from another once
several share the one subscription. A bare payload decodes with an empty
namespace and event, so a delivery the executor did not ask to be enveloped can
never satisfy a step.

Two consequences follow, and both are load-bearing when writing a test:

- **A step list names ONE namespace.** `rpc.SubscribeEventsInput` carries one, so
  steps naming two are REFUSED before the executor dials, with a message naming
  both. Split the test rather than subscribing to one and dropping the rest.
- **A step matches ANY delivery of that identity, scrollback included.** It cannot
  assert "another one, after this point". A second `sa-up` after an operator
  `clear` re-matches the first, so re-establishment is asserted by a separate
  test (`test/ipsec/ipsec-clear-reestablish.ci`), not by a second `expect=event`.

<!-- source: internal/test/runner/engine_steps.go -- EngineStepSubscriptionFor, RunEngineSteps -->
<!-- source: internal/test/cli/cmd_engine_steps.go -- cmdEngineSteps -->
<!-- source: internal/component/plugin/server/dispatch.go -- buildEventEnvelope, resolveSubscriptionNamespace -->

### expect=command-error

`expect=command-error:contains=<text>` asserts that the PRECEDING `command=`
FAILED, and that its message contains the text.

Use it for a command that must refuse. The plugin SDK turns a `StatusError`
response into a Go error, so without this directive the command step aborts the
run before any `expect=` is reached, and no `.ci` can assert an operational
error at all. That left one class untestable end to end, and it is the class
where a wrong answer costs most: a command that must refuse is exactly the one
whose failure mode is answering confidently instead. `test/ipsec/ipsec-dataplane-show.ci`
uses it to prove that a dataplane which cannot be read SAYS so rather than
rendering an empty table.

It takes `contains=` only, and no timeout. An error is the result of one
dispatch, so re-dispatching until one appears would wait for a state change no
`expect=` can cause.

A command failure that NO `expect=command-error` consumes still fails the run,
whether the next step is of another kind or the command is the last step in the
file. Every `.ci` written before this directive existed relies on that.

<!-- source: internal/test/runner/engine_steps.go -- parseEngineExpectCommandError, RunEngineSteps -->

**It must not be vacuous.** A `command=` that SUCCEEDS does not satisfy
`expect=command-error`: the step fails with "the preceding command succeeded".
Without that rule the directive would pass for the very regression it guards, a
refusal quietly becoming a successful empty answer
(`TestRunEngineStepsCommandErrorRequiresAFailure`).

### expect=output / expect=stream predicates

The optional trailing `:timeout=<dur>` is split off the END first, so a predicate
operand may itself contain `:` (a compact-JSON fragment, an IPv6 address). The
remainder is one predicate:

| Predicate | Surfaces | Holds when |
|-----------|----------|-----------|
| `contains=<text>` | output, stream | the output contains the substring (the default) |
| `matches=<regexp>` | output, stream | the Go `regexp` matches the output (compiled at parse time, so a bad regexp fails the test immediately, not at timeout) |
| `absent=<text>` | output only | the output does NOT contain the substring |
| `json=<dotted.path>=<value>` | output only | the dotted path into the JSON `data` field stringifies to `<value>` |

`json=` walks the raw `data` field (not `status data`): each `.`-segment indexes
a JSON object by key or a JSON array by integer index (0..len-1; out-of-range or
missing is "not yet", named at timeout). The leaf is compared as a string
(numbers/bools stringified via JSON). `absent=`/`json=` are `expect=output` only:
they re-dispatch a query, whereas `expect=stream` is an append-only event stream
with no "absent" and no single-event JSON path.

**`absent=` must be non-vacuous.** An `absent=` on output that was never populated
passes instantly (false green). Precede it with a step that makes the substring
present (a `contains=`/`json=` after an inject), then the transition (e.g. a
withdraw) the `absent=` proves. See `test/plugin/engine-steps-predicates.ci`.

<!-- source: internal/test/runner/engine_steps.go -- parseEngineExpectContains, engineOutputSatisfied, engineJSONPathValue -->

### Example

```
command=request bgp rib inject 10.0.0.1 ipv4/unicast 172.16.0.0/16 origin igp nexthop 10.0.0.2
command=show rib
expect=output:matches=172\.16\.[0-9.]+/16:timeout=10
expect=output:json=0.prefix=172.16.0.0/16:timeout=10
command=request bgp rib withdraw 10.0.0.1 ipv4/unicast 172.16.0.0/16
command=show rib
expect=output:absent=172.16.0.0/16:timeout=10
```

## Complete Example

```
# Embed config using Tmpfs
tmpfs=test.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65000;
    }
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    hold-time 180;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 10.0.1.254;
        }
    }
}
EOF_CONF

# Test configuration
option=file:path=test.conf
option=asn:value=65000

# Expected API command and wire output
cmd=api:conn=1:seq=1:text=update text origin set igp nhop set 10.0.1.254 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FFFD4003040A0001FE180A0000

# EOR
cmd=api:conn=1:seq=1:text=announce eor ipv4/unicast
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
```

## Consumers

Different components consume different line types:

| Line Type | Consumer |
|-----------|----------|
| `stdin=` | Test runner (pipes to processes) |
| `tmpfs=` | Test runner (writes to temp) |
| `option=` | Test runner + ze-peer |
| `cmd=api:` | nobody. `parseCmd` stores the text on the message and only the failure reporter prints it, so the line documents the command that produced the expected bytes and executes nothing. A test that expects it to inject routes asserts against an empty RIB |
| `cmd=foreground:`, `cmd=background:` | Test runner (process orchestration) |
| `expect=exit:`, `stdout:`, `stderr:`, `json:`, `syslog:`, `file:` | Test runner |
| `reject=stderr:`, `reject=syslog:` | Test runner (negative expectations) |
| `http=get:`, `http=post:` | Test runner (HTTP assertion checks) |
| `http=wait:` | Test runner (HTTP readiness polls) |
| `expect=bgp:` | ze-peer |
| `action=notification:`, `action=send:` | ze-peer |
| `action=rewrite:`, `action=sighup:`, `action=sigterm:` | ze-peer (reload/signal tests) |
<!-- source: internal/test/peer/expect.go -- ConsumesLine, the ze-peer-consumed set -->
<!-- source: internal/test/runner/record.go -- Record, State -->

Lines not recognized by a consumer are ignored.

### A check-mode peer block MUST declare a ze-peer-consumed expectation

**The consumer split above is load-bearing, not trivia.** Only the four
ze-peer rows reach ze-peer. Everything else -- including `expect=json` --
is validated by the test runner from its own copy of the messages.

A check-mode `ze-peer` with no consumed directive has nothing to check, so it
prints `no test data available to test against` and exits 1 **before binding a
listening socket**. ze then dials a dead port, gets connection refused, and backs
off 5->10->20->40s. That looks exactly like a BGP establishment stall and cost a
multi-day investigation before the cause was found in the harness.

So a peer block whose only expectation is `expect=json` runs no BGP at all. The
parser now rejects that at discovery time, naming the file and the remedy:

```
stdin=peer block: check-mode ze-peer (cmd seq=1) declares no ze-peer-consumed
expectation, so it exits with "no test data available to test against" before
binding a listening socket and the test can only pass vacuously.
```

| Want | Do |
|------|----|
| Assert the wire exchange | Add `expect=bgp:conn=N:seq=N:hex=...` (or an `action=send/notification/rewrite/close/sighup/sigterm`) to the peer block |
| A peer that is only a dial target for ze (routes injected through an external process plugin, assertions made by that plugin or `http=`) | Run it as `ze-peer --mode sink` -- sink/echo/inject peers legitimately declare nothing |

`expect=json` still works, but only **in addition to** a consumed directive: it
cannot make the peer listen.
<!-- source: internal/test/runner/peer_contract.go -- validatePeerBlocks, the parse-time guard -->
<!-- test: internal/test/runner/peer_contract_test.go TestParseAndAdd_CheckPeerWithOnlyJSONExpectRejected -->

### A ze-peer always governs its test's result

A test with a **check-mode** ze-peer is never self-validated: the peer must print
`successful` and its `expect=json` expectations must match, whatever else the file
asserts. An `expect=exit:code=0` is additive and does NOT disable the BGP checks.

Until 2026-07-16 it did, which is how a test could pass on ze's exit code while its
peer never listened and its JSON assertion never ran. sink/echo/inject peers do not
govern (they never report completion), so a test whose only peers are scaffolding
is still governed by its own exit/output/file assertions.
<!-- source: internal/test/runner/peer_contract.go -- isSelfValidated, hasCheckPeer -->
<!-- test: internal/test/runner/peer_contract_test.go TestIsSelfValidated, TestHasCheckPeer -->

### A scaffolding ze-peer is signaled at teardown

A sink, echo or inject `ze-peer` never ends itself: its accept loop runs until its
context is cancelled, and `ze-test peer` maps SIGTERM to that cancel. The runner
sends that SIGTERM at teardown, after the last step and before the barrier that
collects peer output. The peer exits with status 0 and its capture is complete, so
a `.ci` author needs no teardown directive and must not add one.

A **check-mode** peer is never signaled. It exits by itself when its expectations
are met, and the runner reads its verdict from the capture a signal would truncate.

Before 2026-08-10 no code signaled a scaffolding peer, so the drain barrier waited
out its full 10s grace on every `--mode sink` or `--mode echo` test. That cost was
not only latency: `test/plugin/event-predicate-wait.ci` failed at its 15s budget
with `TYPE: timeout` while the daemon itself completed correctly.
<!-- source: internal/test/runner/runner_exec_util.go -- terminateScaffoldPeers, drainPeers -->
<!-- source: internal/test/runner/runner_exec.go -- runOrchestrated teardown, terminateScaffoldPeers call -->
<!-- source: internal/test/cli/cmd_peer.go -- cmdPeer, SIGTERM mapped to the peer's context cancel -->
<!-- test: internal/test/runner/peer_teardown_test.go TestTerminateScaffoldPeersReapsSinkPeer, TestTerminateScaffoldPeersLeavesCheckPeer -->

## Migration from Old Format

Old format (deprecated):
```
option:file:test.conf
option:asn:65000
1:raw:FFFF...
1:json:{...}
```

New format:
```
option=file:path=test.conf
option=asn:value=65000
expect=bgp:conn=1:seq=1:hex=FFFF...
expect=json:conn=1:seq=1:json={...}
```

Key changes:
- `=` instead of `:` after action
- Explicit `conn=` and `seq=` for message ordering
- `hex=` prefix for wire bytes
- `json=` prefix for JSON data

## Editor Test Format (.et)

The `.et` format extends `.ci` for interactive editor testing. Tests are located in `test/editor/`.
<!-- source: internal/component/cli/testing/parser.go -- editor test parsing -->

### Overview

Editor tests simulate user input sequences against the headless configuration editor and verify state changes.

### Input Actions

| Action | Purpose | Example |
|--------|---------|---------|
| `input=type:text=<text>` | Type text | `input=type:text=edit bgp` |
| `input=key:name=<key>` | Send special key | `input=key:name=tab` |
| `input=tab` | Tab key (shorthand) | `input=tab` |
| `input=enter` | Enter key (shorthand) | `input=enter` |
| `input=ctrl:key=<c>` | Ctrl+key | `input=ctrl:key=u` |
| `input=space` | Space key | `input=space` |

Named keys `input=key:name=<key>` accepts: `tab`, `enter`, `esc`, `escape`,
`up`, `down`, `left`, `right`, `backspace`, `delete`, `home`, `end`, `pgup`,
`pgdn`, `pgdown`, `space`. `shift+tab` is handled separately as a modifier.
<!-- source: internal/component/cli/testing/input.go -- keyNameToCode, toKeyMessages -->


### Expectations

| Expectation | Purpose | Example |
|-------------|---------|---------|
| `expect=context:path=<p>` | Context equals path | `expect=context:path=bgp.peer.1.1.1.1` |
| `expect=context:root` | Context is root | `expect=context:root` |
| `expect=completion:contains=<list>` | Completions include all items | `expect=completion:contains=set,delete,edit` |
| `expect=completion:excludes=<list>` | Completions must NOT include items | `expect=completion:excludes=vpp,kernel` |
| `expect=completion:count=<N>` | Number of completions | `expect=completion:count=5` |
| `expect=ghost:text=<suffix>` | Ghost text suggestion | `expect=ghost:text=-id` |
| `expect=dirty:true` | Has unsaved changes | `expect=dirty:true` |
| `expect=content:contains=<text>` | Config content includes text | `expect=content:contains=router-id` |
| `expect=content:not-contains=<text>` | Config content must NOT include | `expect=content:not-contains=old-value` |
| `expect=content:lines=<N>` | Config content line count | `expect=content:lines=5` |
| `expect=viewport:contains=<text>` | Displayed output includes text | `expect=viewport:contains=10.0.0.1` |
| `expect=viewport:not-contains=<text>` | Displayed output must NOT include | `expect=viewport:not-contains=error` |
| `expect=errors:count=<N>` | Validation error count | `expect=errors:count=0` |
| `expect=status:contains=<text>` | Status message | `expect=status:contains=committed` |
| `expect=hint:contains=<text>` | Second message line includes text | `expect=hint:contains=show the peers` |
| `expect=hint:empty` | Second message line is blank | `expect=hint:empty` |
| `expect=explanation:contains=<text>` | Revealed long explanation includes text | `expect=explanation:contains=established` |
| `expect=explanation:empty` | No explanation is revealed | `expect=explanation:empty` |
| `expect=error:none` | No command error | `expect=error:none` |
| `expect=timer:active` | Confirm timer running | `expect=timer:active` |
| `expect=file:path=<rel>:contains=<text>` | On-disk file content | `expect=file:path=test.conf:contains=bgp` |
| `expect=file:path=<rel>:not-contains=<text>` | File must NOT contain | `expect=file:path=test.conf:not-contains=old` |
| `expect=file:path=<rel>:absent=true` | File does not exist | `expect=file:path=test.conf:absent=true` |
<!-- source: internal/component/cli/testing/expect.go -- editor expectation types -->

`hint` reads the second message line with every style stripped. That row carries
the completion hint, and the summary of the candidate the operator selected.
`explanation` reads the long explanation Tab reveals, without the box that frames
it on screen. Both accept `empty` and `contains=`. Both refuse an expectation
that names neither key.
<!-- source: internal/component/cli/testing/expect.go -- checkHint, checkExplanation -->

A test that runs `option=mode:value=command` or `option=mode:value=operational`
completes against the real command tree, built from the registered `-cmd` YANG
modules. The tree carries no value hints and no plugin commands. A hint provider
reads the live RIB, and a plugin command comes from the live dispatcher. A
headless test has neither, so a completion over a VALUE stays empty there.
<!-- source: internal/component/cli/testing/headless.go -- headlessCommandTree -->

### Wait Actions

| Action | Purpose | Example |
|--------|---------|---------|
| `wait=ms:<N>` | Wait N milliseconds | `wait=ms:200` |
| `wait=validation` | Wait for validation | `wait=validation` |
| `wait=timer:expire` | Wait for timer expiry | `wait=timer:expire` |

### Example

```
# Test: Edit navigation
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  router-id 1.2.3.4;
  peer upstream1 {
    remote {
      ip 1.1.1.1;
      as 65001;
    }
  }
}
EOF_CONF

option=file:path=test.conf

expect=context:root
input=type:text=edit bgp
input=enter
expect=context:path=bgp
expect=error:none

input=type:text=set
input=space
expect=completion:contains=router-id,local-as,peer
```

### Test Categories

| Category | Location | Tests |
|----------|----------|-------|
| Navigation | `test/editor/navigation/` | edit, up, top, context |
| Completion | `test/editor/completion/` | commands, YANG paths, values |
| Commands | `test/editor/commands/` | set, delete, show, compare |
| Lifecycle | `test/editor/lifecycle/` | commit, rollback, load, history |
| Validation | `test/editor/validation/` | hold-time, peer-as |
| Pipe | `test/editor/pipe/` | grep, head, tail |
<!-- source: internal/component/cli/testing/parser.go -- editor test file parsing -->
<!-- source: internal/component/cli/testing/session_test.go -- editor session tests -->

Full format specification: `plan/spec-editor-testing-framework.md`

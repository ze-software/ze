# Spec: tftp-server-timeout-and-windowsize

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | Absent options of RFCs the TFTP server implements (RFC 2349 timeout, RFC 7440 windowsize as the sender). A missing RFC feature may wait until after the first release under the owner's release timing, so post-release; Thomas ordered every open item run now |
| RFC gaps | RFC2349-x-1, RFC2349-x-2, RFC7440-3-3, RFC7440-4-1, RFC7440-4-4, RFC7440-4-5 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Ownership: designed by a subagent of session `bc566aa5-3f69-415d-8c8b-c6f8cdf8adae`
on 2026-10-31, at that session's order, without `./le spec claim` (the parent
holds the session marker). The design stops at the approval gate: Thomas has
not approved it, and the decisions only he can make are in "Owner decisions
pending" below.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's TFTP server (`internal/plugins/tftpserver`, `ze_pxe`) negotiates blksize
and tsize but not the RFC 2349 timeout option, and it ignores windowsize, so it
always sends in RFC 1350 lockstep. The mandatory rows that bind the server for
those two options are gaps: commit `e624893a7e` (2026-10-30) already replaced
their `{not-applicable}` annotations with `{gap: ...}` annotations, so this
spec's work is the implementation and the proof, not the reclassification.
rfc2349 stands at 1 of 4 mandatory proven and rfc7440 at 5 of 9.

Implement both on the server: accept a requested timeout of 1 to 255 seconds and
echo it in the OACK, using it as the retransmission interval; accept a requested
windowsize, answer it in the OACK no larger than requested, and send as the
RFC 7440 DSND (send the window, wait for the ACK of its last block, restart the
window from the last ACK received on a timeout or an out-of-sequence ACK). The
server's opcodes, option names and ranges already live in `internal/core/tftp`,
shared with Ze's client, which is the DRCV side and already proves the
receiver rows.

The rows this spec owns:

| Row | Level | Today's annotation | Owed |
|-----|-------|--------------------|------|
| RFC2349-x-1 | MUST | `{gap}`: no timeout option | Acknowledge a valid timeout with the client's value, use it |
| RFC2349-x-2 | MUST | `{gap}`: no range check | Refuse (do not acknowledge) a value outside 1..255 |
| RFC7440-3-3 | MUST | `{gap}`: gap in proof | Acknowledged windowsize at most the requested value |
| RFC7440-4-1 | MUST | `{gap}`: gap in proof | Send the agreed window, then wait for the ACK of its last block |
| RFC7440-4-4 | SHALL | `{gap}`: gap in proof | On timeout, restart the window after the last ACK received |
| RFC7440-4-5 | MUST | `{gap}`: gap in proof | On an in-window ACK, start the next window after that block |
| RFC7440-4-3 | MUST | proven by the client only | Add server-side tags: window of one is RFC 1350 lockstep |
| RFC2349-x-3 | MUST | server tags carry no record | Record the server's `TestRFC2349TsizeRRQReturnsActualSize` in both polarities |

The skeleton listed neither RFC7440-3-3 nor the server side of RFC7440-4-3;
both bind the sender this spec builds, so they are added.

Split from spec-tftp-client (closed 2026-10-30), where these server rows were a
Known Limitation outside that spec's scope.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/system-plugins.md` - design facts every change respects
  → Constraint: the TFTP client during PXE is boot firmware Ze cannot change: "Any wire change" breaks it, so a request carrying neither option must stay byte-identical on the wire and in timing (5 s ACK timeout, 4 sends)
  → Constraint: `tftpserver` is gated by `ze_pxe`; the shared wire facts belong in the core leaf `internal/core/tftp`, which always-on code (the client in `internal/core/fetch`) may import, never in the plugin
- [ ] `docs/architecture/provisioning/tftp-server.md` - the server, its negotiation and its lockstep send
  → Constraint: "Only the ACK timeout causes a retransmission. A duplicate or stale ACK gets no DATA in answer" (Sorcerer's Apprentice fix) must survive for a window of one, and the windowed sender keeps the same rule for an ACK of the block before the window
  → Constraint: "After four sends without the expected ACK the transfer ends and its slot is released" -- maxRetransmit 3 stays the bound, counted per window without progress
  → Constraint: Traps names the 65535 block wrap; the windowed sender must compare wire block numbers modulo 2^16 relative to the window start, never as plain integers
  → Decision: the Traps line "Making the timeout configurable to shorten it would add an option nothing else needs" is superseded by the client-requested RFC 2349 timeout, which lets the retransmission tests request `timeout 1`; the page is edited in the same phase
- [ ] `docs/architecture/appliance/on-device-installer.md` - Ze's client, the test peer
  → Constraint: the client asks for blksize 1468, tsize 0 and windowsize 4, and never sends a timeout option, so a timeout test needs a raw-socket client or curl
- [ ] `docs/contributing/rfc-conformance-gates.md` - "The discrimination record", "Producing a record", "Test file names"
  → Constraint: every tagged unit a commit adds owes a `./le rfc record` record in the same commit; a record's `unit-sha`, `claim-sha` and `producer-sha` are judged against HEAD, so a reworded claim or a changed producer stales the record and owes a re-record
  → Constraint: a test file whose tags all cite one stem is named `rfc<NNNN>_<topic>_test.go`
- [ ] `docs/architecture/testing/interop.md` - "Independent TFTP duplicate-ACK recovery", "Ze's TFTP client against dnsmasq"
  → Constraint: TFTP interop runs as Linux, Docker-tagged package tests through `labtest`, with images pinned by digest or built from `test/interop-tftp/Dockerfile.*`; on a Mac the test binary is cross-built and run in a `docker:cli` container on the VM's host network

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2349.md`, `rfc/full/rfc2349.txt` - timeout option
  → Constraint: RFC 2349 Timeout Interval Option Specification: "If the server is willing to accept the timeout option, it sends an Option Acknowledgment (OACK) to the client.  The specified timeout value must match the value specified by the client." -- the server may decline, never counter-propose
  → Constraint: same section: "Valid values range between "1" and "255" seconds, inclusive." -- 0, 256 and any non-decimal value are declined (not acknowledged), and the transfer keeps the default timeout
- [ ] `rfc/short/rfc7440.md`, `rfc/full/rfc7440.txt` - windowsize, the DSND rows
  → Constraint: RFC 7440 Section 3: "The specified value MUST be less than or equal to the value specified by the client." -- the server may lower the window to its own maximum
  → Constraint: RFC 7440 Section 4: "The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window."
  → Constraint: RFC 7440 Section 4: "In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent."
  → Constraint: RFC 7440 Section 4: "The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence." The figure in Section 4 shows ACK n+05 answered at once with a full window n+06..n+09
  → Constraint: RFC 7440 Section 4: "Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]."
  → Constraint: RFC 7440 Section 3: "The reception of a data window with a number of blocks less than the negotiated windowsize is the final window." -- the window holding the short (or empty) block ends the transfer once its ACK arrives
  → Constraint: RFC 7440 Section 6: "Implementations SHOULD always set a maximum number of retries for datagram retransmissions" -- the existing maxRetransmit bound applies to windows

### Reference implementations (read 2026-10-31)

Sources under `tmp/session/2026-10-30-bc566aa5-3f69-415d-8c8b-c6f8cdf8adae/scratch/ref-tftpd-src/`:
tftp-hpa at `f2d2b5d88ca3` (upstream HEAD 2026-10-12, version file 7.3; latest
release tag `tftp-hpa-7.2`), dnsmasq at `2463798e0322` (2026-10-26), curl
`lib/tftp.c` at tag `curl-8_23_0`.

- [ ] tftp-hpa `tftpd/tftpd.c` `negotiate_timeout`
  → Decision: tftp-hpa declines an out-of-range timeout (`if (tos->uint < MIN_TIMEOUT_SEC || tos->uint > MAX_TIMEOUT_SEC) tos = NULL;`, `MIN_TIMEOUT_SEC 1`, `MAX_TIMEOUT_SEC 255` in `common/tftp.h`) and acknowledges a valid one unchanged; Ze follows it. dnsmasq `src/tftp.c` clamps instead (`if (val > 255) val = 255;` then acknowledges), which breaks "must match"; Ze does not follow dnsmasq here
- [ ] tftp-hpa `tftpd/tftpd.c` `negotiate_windowsize`, `configure.ac`
  → Decision: tftp-hpa acknowledges a windowsize only above 1 (`if (ws && ws->uint > 1)` ... `if (window > 1) ws->flags |= POF_ACK;`), capped by `MAX_WINDOWSIZE` (configure default 256) and `MAX_WINDOWBYTES` (default 4194304). Ze acknowledges only a granted window above 1, so a request for windowsize 1 alone keeps today's bytes (no OACK)
  → Decision: dnsmasq caps at `TFTP_MAX_WINDOW 32` (`src/config.h`); tftp-hpa's client requests 64 by default (CHANGES: "defaults to "--blocksize mtu --windowsize 64""); the server maximum is an owner decision (D-1)
- [ ] tftp-hpa `common/tftp-xfer.c` `tftp_xfer_send`
  → Decision: an ACK of the window's last block ends the window; an ACK of an earlier block in the window restarts sending after it ("An ACK within this window cumulatively confirms its prefix"); any other ACK, including the ACK of the block before the window, is ignored and the wait continues; a timeout resends the window. Ze does the same, except that the restart after an in-window ACK sends a full new window of the agreed size (the RFC 7440 Section 4 figure, and dnsmasq's `transfer->block = transfer->lastack` loop of `winsize` blocks) rather than tftp-hpa's suffix of the old one
- [ ] dnsmasq `src/tftp.c` ACK handling
  → Decision: dnsmasq ignores "duplicate ACKs and ACKs for blocks we've not yet sent" (`block >= transfer->lastack && block <= transfer->block`); Ze ignores both too
- [ ] curl 8.23.0 `lib/tftp.c` `tftp_set_timeouts`, `tftp_send_first`
  → Constraint: curl sends `timeout <retry_time>` in every RRQ unless `--tftp-no-options` (`retry_time = timeout / retry_max`, `retry_max = timeout / 5` bounded to 3..50, so `--max-time 25` sends `timeout 5`); curl never sends windowsize. The existing curl interop tests pass `--tftp-no-options` and are untouched by this spec
  → Decision: curl is the third-party peer for the timeout option; the tftp-hpa client built from the `tftp-hpa-7.2` release is the peer for windowsize (its client sends blksize, tsize and windowsize, never timeout)

**Key insights:** (minimal context to resume after compaction)
- The annotations are already `{gap}` (commit `e624893a7e`); the work is code, tests and records
- Three RFC2347-tagged tests use windowsize as "the unsupported option" and go red when the server supports it: they move to a truly unknown option through the `ze-test-judge` route
- Rewind is by file offset (`ReadAt`), so the window costs one block buffer whatever its size; the server maximum guards the burst on the wire, not memory
- A transfer requesting neither option runs the same code path with window 1 and the 5 s timeout, byte- and timing-identical to today

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/tftpserver/handler.go` (474L) - `parseRRQ`, `handleRRQ` (windowsize logged and ignored), `sendOACKAndWait`, `serveFile`, `sendAndWaitACK`
  → Constraint: `parseRRQ` reads blksize with `strconv.Atoi` (accepts "+1200"), records tsize and windowsize as booleans, and drops every other option, timeout included
  → Constraint: `hasOptions` is `opts.blksize > 0 || opts.tsize`; a request with only windowsize (or only timeout) gets DATA 1 with no OACK today
  → Constraint: `ackTimeout = 5 * time.Second` and `maxRetransmit = 3` are package constants used by `sendAndWaitACK` for both the OACK wait (ACK 0) and every DATA block
  → Constraint: `serveFile` reads sequentially with `f.Read` into one buffer, builds each DATA with `buildData` (allocates per block), and increments a `uint16` block that wraps 65535 to 0
  → Constraint: a transfer that exhausts its retransmissions ends silently (no ERROR) and logs at Debug
- [ ] `internal/core/tftp/tftp.go`, `oack.go`, `request.go` - shared wire facts
  → Constraint: `WindowsizeMin`/`WindowsizeMax`, `OptionWindowsize` exist; no timeout name or range exists yet; the strict decimal reader `decimal` (digits only) is unexported in `oack.go`
- [ ] `internal/plugins/tftpserver/rfc2349_tsize_test.go`, `rfc2347_option_negotiation_test.go`, `rfc2347_2348_transfer_test.go`, `rfc1350_transfer_test.go`, `client_e2e_test.go`, `tftp_curl_docker_test.go`
  → Constraint: `TestRFC2347ServerIgnoresUnacknowledgedOption`, `TestRFC2347OnlyTheClientInitiatesNegotiation` ("unsupported option only" case) and `TestRFC2347UnacknowledgedOptionIsNotUsed` ("windowsize beside blksize" case) assert that windowsize is NOT acknowledged; they become wrong and are changed only through `ze-test-judge`
  → Constraint: `rfc/discrimination/rfc2349/RFC2349-x-3.json` holds the client's records only; the server unit `rfc2349_tsize_test.go::TestRFC2349TsizeRRQReturnsActualSize` has none
  → Constraint: `client_e2e_test.go`'s header says "the unsupported windowsize falls back to lockstep", which becomes stale
- [ ] `test/plugin/init-from-tftp.ci` - `ze init --from tftp://` against Ze's own server
  → Constraint: this is the real-entry-point functional test; after the change it runs with windowsize 4 granted

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every byte and every timer of a transfer that requests neither timeout nor windowsize, which PXE firmware depends on
- A request for windowsize 1 alone keeps today's bytes: no OACK, DATA 1 first
- A duplicate or stale ACK draws no DATA (RFC 1350 Section 5)
- Four sends at most without progress, then the transfer ends and frees its slot
- Read-only: WRQ is refused with error 4

**Behavior to change:** (only what the user asked for)
- A valid requested timeout is acknowledged with the client's value and becomes the transfer's retransmission interval, for the ACK 0 wait and every window
- A requested windowsize above 1 is acknowledged as the smaller of the request and the server maximum, and the file is sent as the RFC 7440 DSND
- The three numeric options (blksize, timeout, windowsize) are read with the one strict decimal reader in `internal/core/tftp`, so "+1200" or " 4" is declined rather than read by `Atoi`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An RRQ datagram on the listening UDP socket (port 69 or the configured port), read by `serveRequests`
- Format: opcode 1, filename, mode, then NUL-terminated option/value pairs (RFC 2347)

### Transformation Path
1. `serveRequests` copies the datagram and dispatches opcode 1 to `handleRRQ`
2. `parseRRQ` reads each option name case-insensitively; blksize, timeout and windowsize values go through `tftp.ParseDecimal` and their RFC range; a value that fails is left unrequested
3. `handleRRQ` computes the grants (blksize as today; timeout as requested; windowsize as the smaller of the request and the server maximum, granted only above 1) and starts `runTransfer` under the semaphore
4. `runTransfer` dials the transfer socket; `sendOACKAndWait` sends the OACK carrying every granted option and waits for ACK 0 using the granted timeout
5. `serveFile` sends windows of `windowsize` DATA blocks read by offset, waits for the window's ACK with the granted timeout, and restarts per RFC 7440 Section 4 on a timeout or an in-window ACK
6. The transfer ends on the ACK of the final block, on the retransmission bound, or when its generation ends

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| TFTP client to server (network) | UDP, RFC 1350/2347/2348/2349/7440 octets | Yes: unit tests over loopback sockets, `init-from-tftp.ci`, curl and tftp-hpa interop |
| Plugin to core leaf | Go import of `internal/core/tftp` (constants and the decimal reader), already in place | Yes: `./le arch tier check` |

### Integration Points
- `internal/core/tftp` - gains `OptionTimeout`, `TimeoutMin`/`TimeoutMax` and the exported decimal reader that `ParseOACK` already uses, so the client and server read option values one way
- `transfer` struct in `handler.go` - gains the granted timeout and window; `runTransfer` passes them on
- `sendAndWaitACK` - keeps serving the OACK wait, with the timeout as a parameter instead of the constant
- `serveFile` - replaced by the windowed sender; the lockstep loop is deleted, not kept beside it (`ai/rules/no-layering.md`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every RRQ still passes `serveRequests` -> `handleRRQ` -> `runTransfer`; the change is inside the transfer |
| No unintended coupling (components stay isolated) | Yes | The plugin imports only the core leaf it already imports; no component import added |
| No duplicated functionality (extends existing, does not recreate) | Yes | The decimal reader and the ranges are taken from `internal/core/tftp`, shared with the client, rather than a second copy in the plugin |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The sender reuses one packet buffer per transfer and reads the payload into it at offset 4; the per-block `buildData` allocation is removed |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No command, YANG leaf or registry entry is added; the option names live in the core leaf beside the existing three |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | The only list that learns "timeout" is the option switch in `parseRRQ`, the server's own RFC 2347 option parser, which is where an option is defined; grep of `internal/` for `"windowsize"` and `OptionWindowsize` found only `internal/core/tftp`, `internal/core/fetch/tftp.go` and `parseRRQ`. The client's `grant` switch does not learn timeout because the client never requests it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No PXE ROM in use sends a timeout or windowsize option whose acknowledgement it then mishandles | iPXE and EDK2 PXE send blksize and tsize; UEFI HTTP boot takes over after the bootloader. No Ze test or doc shows a firmware sending either option | A firmware that requests windowsize but mishandles windows breaks its boot | The existing `test/install/pxe-chainload.ci` run, and a grep of the tftp captures in `test/install/` for the two option names | unvalidated |
| A-2 | Reading a block by offset (`ReadAt`) gives the same bytes as today's sequential `Read` for a file that does not change during the transfer | `os.File.ReadAt` contract; Traps already accept either outcome for a truncated file | A resent block would differ from the first send | Unit test resends a window after a timeout and compares the bytes with the first send | unvalidated |
| A-3 | The Ze client's windowsize 4 path (DRCV) is correct, so a failure of the e2e test points at the server | spec-tftp-client closed with `TestRFC7440ClientWindowsize`, `TestRFC7440ClientSequenceError` recorded | An e2e red is misattributed | The tftp-hpa client interop is a second, independent receiver | unvalidated |
| A-4 | curl 8.23.0 accepts an OACK that acknowledges its timeout and keeps downloading | curl `lib/tftp.c` `tftp_parse_option_ack` ignores option names it does not check (it parses blksize and tsize only) | The timeout interop cannot use curl | The interop test itself | unvalidated |
| A-5 | tftp-hpa 7.2's client builds from the release tarball on Ze's lab Docker host and requests windowsize 64 by default | tftp-hpa CHANGES (windowsize 64 default since 7.0); `test/interop-tftp/Dockerfile.dnsmasq` shows the source-build pattern | The windowsize interop needs another receiver (BusyBox does not implement windowsize) | Building `test/interop-tftp/Dockerfile.tftp-hpa` | unvalidated |
| A-6 | Declining "+1200"-style blksize values breaks no client in use | RFC 2348 defines the value as ASCII decimal; Ze's client and every reference client write plain digits | A client that pads its value loses blksize and falls back to 512 | `rfc2348_blksize_test.go` cases, and the PXE `.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A request carrying neither option changes on the wire or in timing | Any RFC 1350 or RFC 2347 test red without a changed assertion; `TestTFTPCurlDuplicateACKInterop` timing red | Window 1 and the 5 s default go through the same sender; the existing lockstep tests stay unchanged as the guard |
| R-2 | Three RFC2347-tagged tests assert windowsize is unsupported and go red | `TestRFC2347ServerIgnoresUnacknowledgedOption` and two siblings fail once windowsize is granted | Their claim ("an unacknowledged option is ignored") is kept by using an option no server version will support (a made-up name such as `zeunknown`) and an out-of-range windowsize; the change goes to `ze-test-judge`, and their discrimination records are re-recorded |
| R-3 | Sorcerer's Apprentice returns through windows: a duplicate ACK triggers a full window | A test that replays an old ACK sees a window in answer | The ACK of the block before the window and an ACK beyond the last block sent are ignored, as in tftp-hpa and dnsmasq; a negative test replays the old ACK and expects silence until the timeout |
| R-4 | 16-bit block wrap mis-compares ACKs once a window crosses 65535 | A transfer of more than 65535 blocks stalls or ends early | ACK position is computed modulo 2^16 from the window's first block; a unit test starts a window at block 65534 by a file of that many 8-octet blocks (blksize 8) |
| R-5 | A long negotiated timeout lets one client hold a transfer slot for up to 4 x 255 s | `max-transfers` slots stay full | The semaphore still bounds concurrency; the ceiling is owner decision D-3 |
| R-6 | A large window bursts more datagrams than the receiver's socket buffer holds, and loss turns every window into a timeout | The windowed interop is slower than lockstep | The server maximum (D-1) keeps the burst under the Linux default receive buffer: 64 x 1472 octets is about 94 KB against a 212992-octet default |
| R-7 | The final window ends on an empty block when the file is an exact multiple of blksize, and the window logic stops one block early | The "exact" e2e case or an exact-multiple unit case fails | Unit and e2e cases for a file of exactly 3 blocks and an empty file, with window 4 |
| R-8 | An in-window ACK arriving repeatedly drives restarts without bound | A transfer never ends under a looping client | Each in-window ACK advances the last acknowledged block, so restarts are bounded by the file length; the retry counter resets only on progress |
| R-9 | Timing assertions flake under load | Intermittent red on the retransmission tests | Tests negotiate `timeout 1` and assert order and lower bounds (no DATA before the window's ACK, a retransmission after at least the timeout less a margin), never an upper bound tighter than the default timeout |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A client that requests windowsize or timeout gets a broken transfer; a PXE boot fails only if its firmware requests either option (A-1) |
| How is it reverted? | Single commit revert of the server change; the core-leaf additions are additive |
| Who else touches this path? | `internal/core/fetch` (Ze's client) shares `internal/core/tftp`; spec-iface-resolution-across-the-plugin-boundary touches `tftpserver`'s interface resolution, not the handler |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| RRQ with `timeout 1` on the listening socket | → | `parseRRQ` -> `handleRRQ` grant -> `sendOACKAndWait` -> `serveFile` timer | `TestRFC2349ServerAcknowledgesTimeout` |
| RRQ with `windowsize 4` on the listening socket | → | `parseRRQ` -> `handleRRQ` grant -> `sendOACKAndWait` -> windowed `serveFile` | `TestRFC7440ServerSendsWindow` |
| `ze init --from tftp://` against Ze's server (Ze client requests windowsize 4) | → | the whole RRQ path, windowed | `test/plugin/init-from-tftp.ci`, `TestTFTPClientFetchesFromZeServer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RRQ with `timeout 1`, then ACK 0, then no ACK for DATA 1 | OACK carries `timeout` with value `1`; DATA 1 is sent again no sooner than 0.8 s and before the 5 s default would fire (RFC2349-x-1) |
| AC-2 | RRQ with `timeout 1` and `timeout 255` (the boundaries) | Each is acknowledged with the same value (RFC2349-x-1, x-2 positive) |
| AC-3 | RRQ with `timeout` `0`, `256`, `abc`, `+3` or empty, beside `tsize 0` | The OACK carries tsize and no timeout, and the transfer retransmits on the 5 s default (RFC2349-x-2 negative) |
| AC-4 | RRQ with only an invalid timeout | DATA 1 arrives with no OACK, as for an RRQ with no option (RFC 2347: an unacknowledged option is as if never requested) |
| AC-5 | RRQ with `windowsize 4`, 8 blocks of file | OACK carries `windowsize 4`; after ACK 0 the server sends DATA 1..4 and nothing more before ACK 4; after ACK 4 it sends DATA 5..8 (RFC7440-3-3, 4-1) |
| AC-6 | RRQ with a windowsize above the server maximum | OACK carries the server maximum, never more than requested (RFC7440-3-3) |
| AC-7 | RRQ with windowsize `0`, `65536` or `abc` beside blksize | The OACK carries no windowsize and the transfer is lockstep |
| AC-8 | RRQ with `windowsize 1` alone | No OACK; DATA 1 first; lockstep, byte for byte as an RRQ with no option (RFC7440-4-3 server side) |
| AC-9 | Window 4 with `timeout 1`, the window's ACK never sent | After the timeout the server resends the window from the block after the last ACK received, the same bytes (RFC7440-4-4) |
| AC-10 | Window 4 sending DATA 5..8, client sends ACK 6 | The server at once sends a full window DATA 7..10, before the timeout (RFC7440-4-5) |
| AC-11 | Window 4, client replays ACK 4 (the block before the window 5..8) or sends ACK 20 (never sent) | No DATA in answer before the timeout (Sorcerer's Apprentice, R-3) |
| AC-12 | Window 4, the client never acknowledges | The window is sent maxRetransmit+1 times in all, then the transfer ends and its slot is free |
| AC-13 | Window 4 over a file of exactly 3 blocks, an empty file, and a file whose blocks cross block number 65535 | Exact bytes delivered; an exact multiple ends with an empty block; wire block numbers wrap and ACKs across the wrap are matched (R-4, R-7) |
| AC-14 | An RRQ with neither option, and every existing RFC 1350 / RFC 2348 / curl test | Unchanged tests stay green with unchanged assertions |
| AC-15 | The three RFC2347 tests that use windowsize as the unsupported option | Rewritten to an unknown option and an out-of-range windowsize, approved by `ze-test-judge`, records re-recorded |
| AC-16 | The `{gap}` annotations on RFC2349-x-1, x-2 and RFC7440-3-3, 4-1, 4-4, 4-5 | Removed; each row carries tagged server tests in both polarities with recorded breaks; `./le rfc check` counts rfc2349 at 3 of 4 and rfc7440 at 9 of 9 mandatory proven |
| AC-17 | The server's RFC2349-x-3 tags in `rfc2349_tsize_test.go` | Each polarity carries a discrimination record on the server unit |
| AC-18 | Ze's TFTP client against Ze's server | `TestTFTPClientFetchesFromZeServer` and `init-from-tftp.ci` pass with windowsize 4 granted, and the e2e test asserts the grant from the server's Debug log line |
| AC-19 | Interop, timeout | curl 8.23.0, without `--tftp-no-options`, downloads an exact file from Ze's server, and the relayed OACK carries curl's own timeout value |
| AC-20 | Interop, windowsize | The tftp-hpa 7.2 client built from source downloads an exact multi-window file with its default windowsize 64 and with `-W 1000` (granted the server maximum), and the relayed OACK carries the granted window |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs `ze init --from tftp://<server>/<blob>` against a Ze provisioning host | Ze client RRQ (windowsize 4) -> `handleRRQ` -> OACK -> windowed `serveFile` -> digest check | `test/plugin/init-from-tftp.ci` |
| 2 | A host fetches a file with curl from a Ze TFTP server | curl RRQ (tsize, blksize, timeout) -> OACK with timeout -> lockstep with curl's timeout | `TestTFTPCurlTimeoutInterop` |
| 3 | A host fetches a file with tftp-hpa's client | RRQ (blksize, tsize, windowsize 64) -> OACK -> windowed send | `TestTFTPHPAWindowsizeInterop` |
| 4 | A PXE ROM boots from Ze (no timeout, no windowsize) | Unchanged lockstep path | `test/install/pxe-chainload.ci`, the RFC 1350 tests |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC2349ServerAcknowledgesTimeout` | `internal/plugins/tftpserver/rfc2349_timeout_test.go` | AC-1, AC-2: echo and use of the timeout; RFC2349-x-1 both polarities | |
| `TestRFC2349ServerDeclinesTimeoutOutOfRange` | `internal/plugins/tftpserver/rfc2349_timeout_test.go` | AC-3, AC-4: 0, 256, non-decimal, empty; RFC2349-x-2 both polarities | |
| `TestRFC7440ServerAcknowledgesWindowsize` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-6, AC-7: granted at most requested, capped, declined out of range; RFC7440-3-3 both polarities | |
| `TestRFC7440ServerSendsWindow` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-5: W blocks, then silence until the window's ACK; RFC7440-4-1 both polarities | |
| `TestRFC7440ServerWindowOneIsLockstep` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-8: windowsize 1 alone is the no-option transfer; RFC7440-4-3 server side | |
| `TestRFC7440ServerTimeoutRestartsWindow` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-9, AC-12: resend from the last ACK, same bytes, bounded; RFC7440-4-4 both polarities | |
| `TestRFC7440ServerInWindowACKRestartsWindow` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-10, AC-11: full new window after an in-window ACK, silence after a stale or unsent ACK; RFC7440-4-5 both polarities | |
| `TestTFTPWindowedTransferEdges` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-13: exact multiple, empty file, the 65535 wrap | |
| `TestParseDecimal` | `internal/core/tftp/oack_test.go` | The exported reader keeps the refusals `ParseOACK` relied on | |
| `TestRFC2347ServerIgnoresUnacknowledgedOption`, `TestRFC2347OnlyTheClientInitiatesNegotiation`, `TestRFC2347UnacknowledgedOptionIsNotUsed` (changed) | existing files | AC-15, via `ze-test-judge` | |
| `TestRFC2349TsizeRRQReturnsActualSize` (records only) | `internal/plugins/tftpserver/rfc2349_tsize_test.go` | AC-17 | |
| `TestTFTPClientFetchesFromZeServer` (extended) | `internal/plugins/tftpserver/client_e2e_test.go` | AC-18: grant logged as windowsize 4, exact bytes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| timeout | 1-255 | 1 and 255 | 0 | 256 |
| windowsize | 1-65535 | 1 (not acknowledged, lockstep) and 65535 (acknowledged as the server maximum) | 0 | 65536 |
| blksize (now through the shared reader) | 8-65464 | 8 and 65464 | 7 | 65465 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `init-from-tftp` | `test/plugin/init-from-tftp.ci` | `ze init --from tftp://` imports a blob from Ze's server, now windowed; unchanged file, re-run as evidence | |
| `pxe-chainload` | `test/install/pxe-chainload.ci` | PXE path with neither option stays green | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestTFTPCurlTimeoutInterop` | `internal/plugins/tftpserver/tftp_curl_docker_test.go` | curl 8.23.0 (pinned digest) | Timeout acknowledged with curl's value, exact download; red with the timeout grant broken | |
| `TestTFTPHPAWindowsizeInterop` | `internal/plugins/tftpserver/tftp_hpa_docker_test.go`, image from `test/interop-tftp/Dockerfile.tftp-hpa` | tftp-hpa 7.2 client, built from the release source | Windowed send to an independent DRCV, grant at most requested, exact multi-window download; red with the window sender reverted to lockstep timing | |

## Files to Modify
- `internal/core/tftp/tftp.go` - `OptionTimeout`, `TimeoutMin`, `TimeoutMax` with their RFC 2349 quote
- `internal/core/tftp/oack.go` - export the strict decimal reader (`ParseDecimal`), `ParseOACK` keeps using it
- `internal/plugins/tftpserver/handler.go` - timeout and windowsize in `parseRRQ`, the grants in `handleRRQ`, the granted options in `sendOACKAndWait`, `sendAndWaitACK` with a timeout parameter, `serveFile` replaced by the windowed DSND with offset reads and one reused packet buffer, the server maximum constant, a Debug line naming the granted options
- `internal/plugins/tftpserver/rfc2347_option_negotiation_test.go`, `rfc2347_2348_transfer_test.go` - AC-15 through `ze-test-judge`
- `internal/plugins/tftpserver/client_e2e_test.go` - header comment and the AC-18 grant assertion
- `internal/plugins/tftpserver/tftp_curl_docker_test.go` - `TestTFTPCurlTimeoutInterop`
- `rfc/short/rfc2349.md`, `rfc/short/rfc7440.md` - drop the `{gap}` annotations on the owned rows, rewrite the Enrolment reason and Support cells
- `docs/architecture/provisioning/tftp-server.md` - the timeout and window decisions, the retransmission rule per window, the Traps line on the timeout
- `docs/architecture/testing/interop.md` - the two new TFTP interop sections
- `features/installation.md` - the tftpserver sentence names RFC 2349 timeout and RFC 7440 windowsize as the sender

## Files to Create
- `internal/plugins/tftpserver/rfc2349_timeout_test.go` - RFC2349-x-1, x-2 server tests
- `internal/plugins/tftpserver/rfc7440_window_send_test.go` - RFC7440-3-3, 4-1, 4-3, 4-4, 4-5 server tests and the edges
- `internal/plugins/tftpserver/tftp_hpa_docker_test.go` - the windowsize interop carrier
- `test/interop-tftp/Dockerfile.tftp-hpa` - tftp-hpa 7.2 client from the release source
- `rfc/discrimination/rfc2349/RFC2349-x-1.json`, `RFC2349-x-2.json`, and the server entries added to `RFC2349-x-3.json`; `rfc/discrimination/rfc7440/RFC7440-3-3.json`, `RFC7440-4-1.json`, `RFC7440-4-4.json`, `RFC7440-4-5.json`, and server entries in `RFC7440-4-3.json` - written by `./le rfc record`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config leaf, unless Thomas chooses one at D-4 |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the wire path is covered by `init-from-tftp.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | No | No env var |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module or binary: the same listener and transfer sockets |
| Prometheus counters/metrics | No | No counter today for TFTP transfers; none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/installation.md` (the tftpserver sentence) |
| 2 | Config syntax changed? | No | No config change |
| 3 | CLI command added/changed? | No | No command |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | `docs/guide/plugins.md` lists tftpserver without option detail; nothing there becomes wrong |
| 6 | Has a user guide page? | No | The provisioning guide names no TFTP option |
| 7 | Wire format changed? | Yes | `docs/architecture/provisioning/tftp-server.md` (it is the server's wire page; no `docs/architecture/wire/` page covers TFTP) |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2349.md`, `rfc/short/rfc7440.md`; `docs/features/rfc-status.md` is generated from their Support rows |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (two TFTP interop sections) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` carries no TFTP row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/provisioning/tftp-server.md` |
| 13 | Route metadata keys added/changed? | No | Not routing |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Registration unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `handler.go` declares `docs/architecture/provisioning/tftp-server.md` (updated); `internal/core/tftp/*.go` declare `tftp-server.md` and `docs/architecture/appliance/on-device-installer.md` (the latter unaffected: the client does not change); confirm with `./le spec citation anchors spec plan/post-release/features/spec-tftp-server-timeout-and-windowsize.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No TFTP examples carry options |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- core-leaf names and the failing wiring tests
   - Tests: `TestRFC2349ServerAcknowledgesTimeout`, `TestRFC7440ServerSendsWindow`
   - Files: `internal/core/tftp/tftp.go`, `oack.go`, `handler.go` (parse and grant, OACK carries the grants)
   - Verify: OACK assertions pass; the retransmission and window assertions fail because `serveFile` is still lockstep on the constant timeout
2. **Phase: Timeout** -- the granted timeout drives `sendAndWaitACK` and the sender
   - Tests: the two RFC 2349 timeout tests
   - Files: `handler.go`
   - Verify: red then green; the RFC 1350 tests unchanged and green
3. **Phase: Windowed sender** -- replace `serveFile` with the DSND (offset reads, one packet buffer, window restart rules, retry bound per window, 16-bit ACK arithmetic)
   - Tests: the RFC 7440 server tests and `TestTFTPWindowedTransferEdges`
   - Files: `handler.go`
   - Verify: red then green; every existing test green; the per-block allocation gone
4. **Phase: Test judge and records** -- AC-15 through `ze-test-judge`, then `./le rfc record` for every new tag and the server x-3 tags, and summary edits
   - Tests: the changed RFC2347 tests; `./le rfc check`
   - Files: the two RFC2347 test files, `rfc/short/rfc2349.md`, `rfc/short/rfc7440.md`, `rfc/discrimination/*`
5. **Phase: End to end and interop** -- `client_e2e_test.go` grant assertion, `init-from-tftp.ci` rerun, curl timeout interop, tftp-hpa windowsize interop with their forced reds
   - Files: `client_e2e_test.go`, `tftp_curl_docker_test.go`, `tftp_hpa_docker_test.go`, `test/interop-tftp/Dockerfile.tftp-hpa`
6. **Phase: Docs** -- each page in the Documentation checklist, edited in the phase that changes it (steps 2, 3 and 5), checked here

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A transfer without either option is byte-identical to today's, on the same 5 s timer |
| Correctness | No DATA is sent in answer to the ACK of the block before the window or to an ACK of a block not yet sent |
| Correctness | ACK positions are computed modulo 2^16 from the window's first block |
| Rule: rfc-compliance | Every implementing function quotes its RFC sentence; every caller names the RFC section |
| Rule: no-layering | The lockstep loop is deleted, not kept beside the windowed sender |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Timeout acknowledged and used | `go test -run 'TestRFC2349Server' ./internal/plugins/tftpserver/` |
| Windowed sender | `go test -run 'TestRFC7440Server|TestTFTPWindowed' ./internal/plugins/tftpserver/` |
| Gaps closed in the ledger | `./le rfc discriminate stem rfc2349` and `stem rfc7440` show the owned rows proven, no `{gap}` on them |
| Interop | the two Docker-tagged tests green, each with a recorded forced red |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A large windowsize cannot hold memory: the sender keeps one block buffer whatever the window; the server maximum bounds the datagram burst |
| Resource exhaustion | A long timeout holds a slot for at most 4 x the granted timeout; the semaphore bounds the count (D-3) |
| Amplification | One RRQ draws at most one window before the client acknowledges ACK 0 of the OACK; no window is sent before ACK 0 |
| Input validation | Option values are digits only, in range, else declined; no panic on an empty value or an overlong number |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

Two approaches were weighed for the windowed sender:

| Approach | How it works | Gains | Costs |
|----------|--------------|-------|-------|
| A: buffer the window | Keep the window's DATA packets in memory and resend them | No file reads on a resend | Memory is windowsize x blksize per transfer (65535 x 1472 is 96 MB), so the maximum must also be a memory limit; a second buffer pool |
| B: rewind by offset (recommended) | Read block k at offset (k-1) x blksize with `ReadAt` into one reused packet buffer; a resend re-reads | One buffer whatever the window; the restart rules are just "set the next block"; removes today's per-block allocation | One `ReadAt` per resent block, served from the page cache |

B wins: it is the smaller change, its memory does not grow with the window, and
the server maximum then only has to answer the network question (R-6).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | No journal row: no defect was walked into; the gaps are summary rows this spec owns |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decline (do not acknowledge) a timeout outside 1..255 or not decimal | Clamp to 255 and acknowledge (dnsmasq) | RFC 2349 says the acknowledged value "must match the value specified by the client"; tftp-hpa `negotiate_timeout` declines |
| The granted timeout covers the ACK 0 wait and every window | Apply it to DATA only | tftp-hpa sets `rexmtval` in `negotiate_options` before the OACK is sent; RFC 2349 defines it as "The number of seconds to wait before retransmitting" with no exception for the OACK |
| Acknowledge windowsize only when the grant is above 1 | Acknowledge any requested value, 1 included (dnsmasq) | tftp-hpa `negotiate_windowsize`; it keeps today's bytes for a windowsize 1 request (AC-8), and an unacknowledged windowsize is a window of one for the client (RFC 7440 default) |
| After an in-window ACK, send a full new window from the next block, immediately | Resend only the old window's suffix (tftp-hpa) | RFC 7440 Section 4 figure (ACK n+05 answered with n+06..n+09) and dnsmasq; both are compliant, the full window keeps one rule for every window |
| Ignore the ACK of the block before the window and ACKs beyond the last block sent | Treat the first as a sequence-error notice and resend at once | tftp-hpa `tftp_xfer_send` and dnsmasq both ignore it; answering it reopens the Sorcerer's Apprentice failure; the timeout still restarts the window from that ACK, which meets RFC7440-4-4 and 4-5 |
| maxRetransmit 3 counted per window without progress; reset when the last acknowledged block advances | A total retry budget per transfer | Keeps today's four-send bound for window 1 exactly; RFC 7440 Section 6 asks for a maximum, not a value |
| One strict decimal reader for all numeric options, from `internal/core/tftp` | Keep `Atoi` for blksize | One declaration shared with the client (`ai/rules/principles.md`, declare once); the RFCs define the values as ASCII decimal |
| Interop peers: curl 8.23.0 for timeout, tftp-hpa 7.2 built from source for windowsize | dnsmasq (a server, not a client); BusyBox tftp (no windowsize) | Each is a client sending the option under test; tftp-hpa 7.2 is its latest release (`ai/META-DECISIONS.md`, latest stable release in labs) |

## Owner decisions pending

| # | Decision | Options | Recommendation | Evidence | What each answer changes |
|---|----------|---------|----------------|----------|--------------------------|
| D-1 | Server maximum windowsize | 32 (dnsmasq), 64, 256 (tftp-hpa build default), 65535 (no cap) | 64 | tftp-hpa client requests 64 by default; 64 x 1472 octets is about 94 KB, under the Linux default 212992-octet receive buffer; RFC 7440 Section 5: "Operators SHOULD test various values and SHOULD be conservative" | The grant for large requests, the burst size, AC-6 and AC-20's expected value |
| D-2 | Retransmission timeout when the client requests none | Keep 5 s (today), 1 s (tftp-hpa default, RFC 7440 Section 6 example), 2 s (dnsmasq) | Keep 5 s | PXE firmware and the curl duplicate-ACK interop are tuned to 5 s; changing it is a wire change to every PXE client (system-plugins door) | Only transfers without a timeout option; 5 s keeps them byte- and timing-identical |
| D-3 | Highest client timeout the server accepts | 255 (the RFC range, tftp-hpa, dnsmasq), or a lower ceiling such as 30 above which the option is declined | 255 | RFC 2349 lets a server decline but not counter-propose; a slot is held at most 4 x 255 s = 17 min, and `max-transfers` (default 10) bounds how many | With 255 one client may hold a slot 17 min; with a ceiling, a client asking more falls back to 5 s |
| D-4 | Make the maximum window or the default timeout configurable | No config (constants), or YANG leaves under `service tftp-server` | No config | No user has asked; the tftp-server page already rejects an option "nothing else needs"; a leaf adds YANG, docs and tests | With leaves the spec grows a config surface (Integration rows 1 to 3 become Yes) |

## Known Limitations
- The PXE boot firmware that is Ze's server's main client cannot change, so lockstep without these options must stay byte-identical (AC-14)
- RFC2349-x-4 (WRQ tsize) stays a gap: the server refuses writes, and no owner decision declines them; it is not this spec's row
- RFC7440-6-3 (SHOULD follow RFC 5405 congestion control) is not addressed beyond the window maximum and the retry bound

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-tftp-server-timeout-and-windowsize.md` only, in the same `./le commit create` script

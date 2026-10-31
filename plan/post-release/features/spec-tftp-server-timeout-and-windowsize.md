# Spec: tftp-server-timeout-and-windowsize

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | Absent options of RFCs the TFTP server implements (RFC 2349 timeout, RFC 7440 windowsize as the sender). A missing RFC feature may wait until after the first release under the owner's release timing, so post-release; Thomas ordered every open item run now |
| RFC gaps | RFC2349-x-1, RFC2349-x-2, RFC7440-3-3, RFC7440-4-1, RFC7440-4-4, RFC7440-4-5 |
| Depends | - |
| Phase | 1/6 |
| Handoff | - |
| Updated | 2026-10-31 |

Ownership: designed and implemented by a subagent of session
`bc566aa5-3f69-415d-8c8b-c6f8cdf8adae`, claimed with `./le spec claim` on
2026-10-31 once the TFTP client spec released the marker. Thomas answered the
four owner decisions on 2026-10-31 (see "Owner decisions" below), which
approved the design with a config surface added ("Config surface (D-4)").

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
| A-1 | No PXE ROM in use sends a timeout or windowsize option whose acknowledgement it then mishandles | iPXE and EDK2 PXE send blksize and tsize; UEFI HTTP boot takes over after the bootloader. No Ze test or doc shows a firmware sending either option | A firmware that requests windowsize but mishandles windows breaks its boot | The existing `test/install/pxe-chainload.ci` run, and a grep of the tftp captures in `test/install/` for the two option names | confirmed (2026-10-31): `test/install/` holds no TFTP capture and no `.ci` sends either option; a firmware that does request one now gets RFC 2349/7440 behaviour proven against tftp-hpa 7.2, curl 8.23.0 and Ze's client, and a request with neither keeps the unchanged lockstep path (every RFC 1350 test green with unchanged assertions) |
| A-2 | Reading a block by offset (`ReadAt`) gives the same bytes as today's sequential `Read` for a file that does not change during the transfer | `os.File.ReadAt` contract; Traps already accept either outcome for a truncated file | A resent block would differ from the first send | Unit test resends a window after a timeout and compares the bytes with the first send | confirmed: `TestRFC7440ServerTimeoutRestartsWindow` and `TestRFC2349ServerAcknowledgesTimeout` compare the resent bytes; `TestTFTPWindowedTransferEdges` matches 65540 blocks byte for byte |
| A-3 | The Ze client's windowsize 4 path (DRCV) is correct, so a failure of the e2e test points at the server | spec-tftp-client closed with `TestRFC7440ClientWindowsize`, `TestRFC7440ClientSequenceError` recorded | An e2e red is misattributed | The tftp-hpa client interop is a second, independent receiver | confirmed: `TestTFTPHPAWindowsizeInterop` passes with windowsize 64 granted and 64-block windows |
| A-4 | curl 8.23.0 accepts an OACK that acknowledges its timeout and keeps downloading | curl `lib/tftp.c` `tftp_parse_option_ack` ignores option names it does not check (it parses blksize and tsize only) | The timeout interop cannot use curl | The interop test itself | confirmed: `TestTFTPCurlTimeoutInterop` passes, curl requested `timeout 5` and accepted the OACK echoing it |
| A-5 | tftp-hpa 7.2's client builds from the release tarball on Ze's lab Docker host and requests windowsize 64 by default | tftp-hpa CHANGES (windowsize 64 default since 7.0); `test/interop-tftp/Dockerfile.dnsmasq` shows the source-build pattern | The windowsize interop needs another receiver (BusyBox does not implement windowsize) | Building `test/interop-tftp/Dockerfile.tftp-hpa` | broken in part, design unaffected: 7.2 has no release tarball (kernel.org lists none), so the Dockerfile clones tag `tftp-hpa-7.2` and refuses any commit but `af23362c585b`; it builds on the Colima `ze` VM and `tftp -V` answers `tftp-hpa 7.2`, which requests windowsize 64 by default |
| A-6 | Declining "+1200"-style blksize values breaks no client in use | RFC 2348 defines the value as ASCII decimal; Ze's client and every reference client write plain digits | A client that pads its value loses blksize and falls back to 512 | `rfc2348_blksize_test.go` cases, and the PXE `.ci` | confirmed: every RFC 2348 test green unchanged; tftp-hpa 7.2, curl 8.23.0 and Ze's client write plain digits |

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
| YANG schema (new RPCs/config) | Yes | `internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang`: `max-windowsize`, `timeout` (D-4) |
| YANG validation constraints | Yes | `range "1..65535"` and `range "1..255"`, `units` on both |
| YANG custom validators | N-A | Native range is sufficient |
| CLI commands/flags | No | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | Numeric leaves; YANG type drives the editor |
| Functional test for new RPC/API | Yes | No RPC; the config leaves get `test/parse/tftp-server-window-timeout.ci`, the wire path `init-from-tftp.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | No | No env var |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module or binary: the same listener and transfer sockets |
| Prometheus counters/metrics | No | No counter today for TFTP transfers; none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/installation.md` (the tftpserver sentence) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (new TFTP server section with both leaves); `docs/architecture/config/syntax.md` carries no per-service leaves, unaffected |
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

## Owner decisions

Thomas answered on 2026-10-31:

| # | Answer |
|---|--------|
| D-1 | The server maximum windowsize is 64 |
| D-2 | The timeout when the client requests none stays 5 s |
| D-3 | A client timeout up to 255 s is accepted: the whole RFC 2349 range |
| D-4 | ADD YANG leaves under `service tftp-server` for the window maximum and the default timeout, defaulting to 64 and 5 s |

The table below is the question as it was put.

| # | Decision | Options | Recommendation | Evidence | What each answer changes |
|---|----------|---------|----------------|----------|--------------------------|
| D-1 | Server maximum windowsize | 32 (dnsmasq), 64, 256 (tftp-hpa build default), 65535 (no cap) | 64 | tftp-hpa client requests 64 by default; 64 x 1472 octets is about 94 KB, under the Linux default 212992-octet receive buffer; RFC 7440 Section 5: "Operators SHOULD test various values and SHOULD be conservative" | The grant for large requests, the burst size, AC-6 and AC-20's expected value |
| D-2 | Retransmission timeout when the client requests none | Keep 5 s (today), 1 s (tftp-hpa default, RFC 7440 Section 6 example), 2 s (dnsmasq) | Keep 5 s | PXE firmware and the curl duplicate-ACK interop are tuned to 5 s; changing it is a wire change to every PXE client (system-plugins door) | Only transfers without a timeout option; 5 s keeps them byte- and timing-identical |
| D-3 | Highest client timeout the server accepts | 255 (the RFC range, tftp-hpa, dnsmasq), or a lower ceiling such as 30 above which the option is declined | 255 | RFC 2349 lets a server decline but not counter-propose; a slot is held at most 4 x 255 s = 17 min, and `max-transfers` (default 10) bounds how many | With 255 one client may hold a slot 17 min; with a ceiling, a client asking more falls back to 5 s |
| D-4 | Make the maximum window or the default timeout configurable | No config (constants), or YANG leaves under `service tftp-server` | No config | No user has asked; the tftp-server page already rejects an option "nothing else needs"; a leaf adds YANG, docs and tests | With leaves the spec grows a config surface (Integration rows 1 to 3 become Yes) |

## Config surface (D-4)

Two leaves in `ze-tftp-server-conf.yang`, under `service tftp-server`, beside
`max-transfers`, whose naming (`max-` prefix) and parse pattern they follow
(`ai/rules/config.md`, `ai/patterns/config-option.md`). No env var: neither is
a debug, bootstrap or safety knob, so no `env.MustRegister`.

| Leaf | Type and range | Default | Units | `ze:help` | `description` (summary) |
|------|----------------|---------|-------|-----------|-------------------------|
| `max-windowsize` | `uint16`, `range "1..65535"` (RFC 7440 Section 3) | 64 | blocks (in the description; YANG `units "blocks"`) | Largest RFC 7440 window the server grants | A client's windowsize request is granted as the smaller of the request and this value; a grant of 1 is not acknowledged, so 1 turns windowed sending off |
| `timeout` | `uint8`, `range "1..255"` (RFC 2349) | 5 | `units seconds` | Seconds to wait for an ACK before resending | Used when the client requests no RFC 2349 timeout; a client's own timeout of 1 to 255 seconds is acknowledged and replaces it for that transfer |

| Behaviour | Design |
|-----------|--------|
| Commit-time refusal | YANG `range` refuses an out-of-range value at parse (`validateNumericRanges`, `internal/component/config/schema.go`). Its message today reads `value "0" outside range 1..255` and names no fix, which `ai/rules/user-facing-errors.md` forbids; it gains the fix, `set a value from 1..255`, for every range leaf. `parseConfig` refuses the same out-of-range value with the leaf and the fix in its text, for a delivery that did not pass the parser (pair the check) |
| Grant | `handleRRQ` reads the running values when the RRQ arrives and stores them in the transfer; nothing in a transfer reads config after it starts |
| Reload, only these two leaves change | `tftpServer.install` swaps the values the listeners read and keeps the listeners, the semaphore and the generation: no transfer is cut, an in-flight transfer keeps the values it started with, and the next RRQ gets the new ones. A rollback is the same swap |
| Reload, any other leaf changes | Unchanged: listeners bound again, generation ended, in-flight transfers cut (tftp-server.md "Config reload") |
| `ServeConn` (fixture, no config) | Uses the YANG defaults, 64 and 5 s, through the same Go constants `parseConfig` defaults to; a test pins the constants to the YANG defaults |

Added acceptance criteria:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-21 | `max-windowsize 8`, `timeout 2` in config, validated | `ze config validate` exits 0; the server grants at most 8 and resends after 2 s for a client that asks no timeout |
| AC-22 | `max-windowsize 0`, `65536`; `timeout 0`, `256` | `ze config validate` exits 1, naming the leaf, the range and the fix |
| AC-23 | Neither leaf set | Grant capped at 64; default timeout 5 s |
| AC-24 | A reload that changes only `timeout` or `max-windowsize` while a transfer runs | The transfer completes on its starting values; the next RRQ uses the new values; no listener is rebound |
| AC-25 | `test/parse/tftp-server-window-timeout.ci` | Covers AC-21 and AC-22 through `ze config validate` |

Added tests:

| Test | File | Validates |
|------|------|-----------|
| `TestParseConfigWindowAndTimeout` | `internal/plugins/tftpserver/config_test.go` | Defaults, boundaries, refusals with the fix text; the Go defaults equal the YANG defaults |
| `TestReloadOfTransferValuesKeepsTransfers` | `internal/plugins/tftpserver/config_tx_test.go` | AC-24 |
| `TestServerUsesConfiguredWindowAndTimeout` | `internal/plugins/tftpserver/rfc7440_window_send_test.go` | AC-21, AC-23 over the socket |
| `tftp-server-window-timeout` | `test/parse/tftp-server-window-timeout.ci` | AC-25 |

Added files: `internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang` (two
leaves; the container description's "does not support the windowsize option"
is corrected), `config.go`, `config_tx.go`, `serve_conn.go`,
`internal/component/config/schema.go` (the range message),
`docs/guide/configuration.md` (a TFTP server section), and the tftp-server page.

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

---

## Implementation Summary

### What Was Implemented
- RFC 2349 timeout on the server: `parseRRQ` reads it through `optionInRange` (`tftp.ParseDecimal`, 1..255); `handleRRQ` takes it as the transfer's interval; `sendOACKAndWait` echoes the client's value (`internal/plugins/tftpserver/handler.go`).
- RFC 7440 windowsize as the sender: `grantWindowsize` (min of request and `max-windowsize`, acknowledged only above 1); `serveFile`, `windowSender.sendWindow`, `readBlock`, `awaitACK`, `windowPosition` in the new `internal/plugins/tftpserver/window_send.go`. Offset reads into one reused packet buffer; `buildData` and the lockstep loop are deleted.
- Config: YANG leaves `max-windowsize` (1..65535, 64) and `timeout` (1..255 s, 5) in `ze-tftp-server-conf.yang`; `parseConfig`/`rangedLeaf` (`config.go`); `transferLimits` in an `atomic.Pointer` read per request; `tftpServer.install` takes a limits-only path (`onlyLimitsChange`) that keeps listeners and transfers (`config_tx.go`).
- Shared core: `tftp.OptionTimeout`, `TimeoutMin`/`TimeoutMax`, exported `tftp.ParseDecimal` (`internal/core/tftp`).
- The generic config range refusal names the fix (`outsideRange`, `allDigits`, `ValidateLeafValue` in `internal/component/config/schema.go`), including a number too large for the type.

### Bugs Found/Fixed
- The config parser answered an out-of-range value with `value "0" outside range 1..255` and a uint16 overflow with `invalid uint16: "65536"`, neither naming a fix (`ai/rules/user-facing-errors.md`); both now end `set a value in <range>`, covered by `TestValidateLeafValueRangeNamesTheFix` and `test/parse/tftp-server-window-timeout.ci`.

### Documentation Updates
- `docs/architecture/provisioning/tftp-server.md`: window decisions, restart rules, timeout source, limits-only reload, Traps line; anchors to `window_send.go` and `config_tx.go`.
- `docs/guide/configuration.md`: new "TFTP Server" section with both leaves and the reload rule.
- `docs/architecture/testing/interop.md`: "Ze's TFTP server against tftp-hpa and curl options".
- `features/installation.md`: the tftpserver sentence names RFC 2349 timeout and RFC 7440 windowsize as the sender and the two leaves.
- `rfc/short/rfc2349.md`, `rfc/short/rfc7440.md`: owned `{gap}` annotations removed, Enrolment reason rewritten (the Support cells are derived since `697558b8ae`).

### Deviations from Plan
- A fourth RFC-tagged test changed: `TestRFC7440FieldsAreNULTerminatedASCIIStrings` now expects six OACK fields, because windowsize 16 is granted; approved by `ze-test-judge`.
- `TestTFTPBuildDataPacket` was deleted with `buildData`; approved as a weakening (file unit `tftpserver.handler_test` too).
- `TestTFTPTransferPanicContained`'s defective literal sets the two new fields so `blksize: -1` still reaches the panic; approved.
- The `serve` signature is kept (YANG defaults) and `serveLimited` takes the limits source, so the many existing tests that call `serve` did not change.
- The RFC1350-2-2 negative record on `TestTFTPConcurrentLimit` moved from a gomu mutant on `sendAndWaitACK` to a hand-written mutant on `windowSender.awaitACK`; the Linux-only `TestListenTFTPLoopbackRoundTrip` record moved from a mutant on `handleRRQ` to the revert route in the QEMU guest.
- tftp-hpa 7.2 ships no tarball, so `Dockerfile.tftp-hpa` clones the tag and refuses any other commit (A-5).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The skeleton said the owned rows were `{not-applicable}` | `e624893a7e` had already made them `{gap}` | Reading `rfc/short/rfc2349.md` at research | Spec Task corrected |
| assumption | Three RFC 2347 tests would need moving off windowsize | Four RFC-tagged units asserted the old behaviour (also RFC7440-3-1's field count) | First test run after the change | Fourth unit sent to the judge |
| approach | Test claims cited `handler.go:<line>` and `sendAndWaitACK` for the DATA path | The sender moved to `window_send.go`; the claims went false | Recording `TestTFTPConcurrentLimit` stayed green under an `awaitACK` revert, which led to reading the claim | Claims corrected, judged, re-recorded |
| assumption | The RFC 2347 x-1 tests still covered "no unrequested option" | With timeout and windowsize grantable, they checked only two of four options absent | The independent RFC audit judged RFC2347-x-1 weak | Both units now require exactly the requested options; judged, re-audited enforced, re-recorded |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Accept timeout 1..255, echo it, use it as the interval | Done | `handler.go` `parseRRQ`, `optionInRange`, `handleRRQ`, `sendOACKAndWait`; `window_send.go` `awaitACK` | |
| Accept windowsize, answer it no larger than requested, send as DSND | Done | `handler.go` `grantWindowsize`, `sendOACKAndWait`; `window_send.go` `serveFile`, `windowSender` | |
| Config leaves max-windowsize and timeout (D-4) | Done | `yang/ze-tftp-server-conf.yang`, `config.go` `rangedLeaf`, `config_tx.go` `onlyLimitsChange` | |
| Refusal names the fix | Done | `schema.go` `outsideRange`, `ValidateLeafValue`; `config.go` `rangedLeaf` | |
| Records for every owned row and the server RFC2349-x-3 tags | Done | `rfc/discrimination/rfc2349/`, `rfc/discrimination/rfc7440/` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1, AC-2 | Done | `TestRFC2349ServerAcknowledgesTimeout` | |
| AC-3, AC-4 | Done | `TestRFC2349ServerDeclinesTimeoutOutOfRange` | |
| AC-5 | Done | `TestRFC7440ServerSendsWindow` | |
| AC-6, AC-7 | Done | `TestRFC7440ServerAcknowledgesWindowsize` | |
| AC-8 | Done | `TestRFC7440ServerWindowOneIsLockstep` | |
| AC-9, AC-12 | Done | `TestRFC7440ServerTimeoutRestartsWindow` | |
| AC-10, AC-11 | Done | `TestRFC7440ServerInWindowACKRestartsWindow` | |
| AC-13 | Done | `TestTFTPWindowedTransferEdges`, `TestWindowPositionAcrossTheWrap` | |
| AC-14 | Done | every RFC 1350 / 2348 / curl duplicate-ACK test green with unchanged assertions | |
| AC-15 | Done | judge approvals for the four RFC-tagged units | |
| AC-16 | Done | `./le rfc tree`: rfc2349 3 of 4 (x-4 is the WRQ gap), rfc7440 9 of 9 | |
| AC-17 | Done | `rfc/discrimination/rfc2349/RFC2349-x-3.json` server entries | |
| AC-18 | Done | `TestTFTPClientFetchesFromZeServer` asserts `windowsize=4` per fetch; `init-from-tftp.ci` unchanged | |
| AC-19 | Done | `TestTFTPCurlTimeoutInterop` | forced red recorded |
| AC-20 | Done | `TestTFTPHPAWindowsizeInterop` | forced red recorded |
| AC-21, AC-23 | Done | `TestServerUsesConfiguredWindowAndTimeout`, `TestParseConfigWindowAndTimeout` | |
| AC-22, AC-25 | Done | `test/parse/tftp-server-window-timeout.ci`, `TestValidateLeafValueRangeNamesTheFix` | |
| AC-24 | Done | `TestReloadOfTransferValuesKeepsTransfers` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| RFC 2349 server tests | Done | `rfc2349_timeout_test.go` | |
| RFC 7440 server tests and edges | Done | `rfc7440_window_send_test.go` | |
| Config and reload tests | Done | `config_test.go`, `config_tx_test.go` | |
| `TestParseDecimal` | Done | `internal/core/tftp/oack_test.go` | |
| Interop | Done | `tftp_hpa_docker_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify and Create | Done | plus `window_send.go`, `leaf_range_test.go`, `test/parse/tftp-server-window-timeout.ci` |

### Audit Summary
- **Total items:** 25 ACs, 5 requirements
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The server implements the RFC 2349 timeout | interop + unit | curl 8.23.0 sends `timeout 5`, OACK echoes `timeout 5` (`TestTFTPCurlTimeoutInterop`); red with the echo removed (`scratch/interop-red-timeout.log`) |
| The server sends as the RFC 7440 DSND | interop + unit | tftp-hpa 7.2 downloads 300 blocks with windowsize 64 granted and 64-block runs (`TestTFTPHPAWindowsizeInterop`); red with `grantWindowsize` forced to 1 (`scratch/interop-red-window.log`) |
| A request with neither option is unchanged | unit + interop | RFC 1350 suite and `TestTFTPCurlDuplicateACKInterop` green unchanged |
| The ledger counts the server rows | gate | `./le rfc tree`: rfc7440 mandatory 9 of 9, rfc2349 3 of 4, rfc2347 3 of 3 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None in scope | RFC2349-x-4 (WRQ tsize) is a read-only-server gap outside this spec's owned rows | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/tftp-server-timeout-and-windowsize-bc566aa5-3f69-415d-8c8b-c6f8cdf8adae.md` (28 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 4 (round 1 the full diff; rounds 2 to 4 only the fixes) |
| Reviewer lenses used | independent `ze-review` agent: logic, window arithmetic, concurrency, docs; independent `ze-test-judge`; independent `ze-rfc-audit` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | File-level weakening finding for `handler_test.go` not approved | `handler_test.go` | Judge approved `tftpserver.handler_test` |
| 2 | ISSUE | Enrolment reason named `handleRRQ` and old lines for the tsize producer | `rfc/short/rfc2349.md` | Names `sendOACKAndWait` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `window_send.go`, `rfc2349_timeout_test.go`, `rfc7440_window_send_test.go`, `tftp_hpa_docker_test.go`, `test/parse/tftp-server-window-timeout.ci`, `test/interop-tftp/Dockerfile.tftp-hpa` | yes | `git status` lists each as new |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-25 | scoped packages green | `go test -race` over tftpserver, core/tftp, core/fetch, component/config: all `ok` (`scratch/t4.log`); parse suite green but `iface-disable-flag-refuses-value` (not this change) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze init --from tftp://` | `test/plugin/init-from-tftp.ci` | unchanged file; its server path now grants windowsize 4, which `TestTFTPClientFetchesFromZeServer` asserts |
| Config leaves | `test/parse/tftp-server-window-timeout.ci` | passes in the parse suite |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-6 | confirmed (A-5 broken in part, design unaffected) | see the Assumptions table |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| tftp-server.md, configuration.md, interop.md, installation.md | reviewer rounds 2 and 3 checked them against `install`, `onlyLimitsChange`, `bindTFTP`, `restore` | yes; `./le doc check all` reports nothing on these pages |


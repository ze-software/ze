# Spec: tftp-client

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | A new capability (a TFTP client behind `ze init --from tftp://`), so post-release under the owner's release timing. Thomas chose it on 2026-10-30 so that the client-side MUSTs of the TFTP family stop being capped `{not-applicable}` rows |
| RFC gaps | RFC2347-x-4, RFC2348-x-2, RFC7440-3-2, RFC7440-3-4, RFC7440-4-2, RFC7440-4-3 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze has a TFTP server (`internal/plugins/tftpserver`) and no TFTP client. Several
mandatory rows of the TFTP family bind the client, so they are annotated
`{not-applicable}` and cap their RFC's ceiling below 100%. `./le rfc tree` on
2026-10-30: rfc2347 mandatory 2 of 3 (ceiling 2 of 3), rfc2348 4 of 5 (ceiling
4 of 5), rfc7440 1 of 9 (ceiling 1 of 9), rfc2349 0 of 4 (ceiling 1 of 4),
rfc1350 6 of 10 (ceiling 8 of 10).

Thomas decided on 2026-10-30 to build the client. Goal: a `tftp` scheme in the
`internal/core/fetch` scheme table, beside http and https, so that
`ze init --from tftp://host/path` downloads over TFTP. The client implements the
read side of RFC 1350, option negotiation per RFC 2347, blksize per RFC 2348,
tsize per RFC 2349, and windowsize as the data receiver per RFC 7440. Each
client-side MUST row it makes provable loses its `{not-applicable}` annotation
and gets tagged tests in both polarities with recorded breaks. Done means rfc2347
reaches 3 of 3 and rfc2348 reaches 5 of 5.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform rules
  → Constraint: a package with no config lifecycle and no registry side effect is core (`internal/core/`). The client is a library that `ze init` calls, so it is core, not a plugin
  → Constraint: a feature registers and is discovered. The tftp scheme is one entry in the `schemes` table in `internal/core/fetch/scheme.go`. `ze init` help, its refusal and `fetch.Schemes()` derive from that table, so they need no edit
- [ ] `ai/doors/core.md` - core leaf tier
  → Constraint: core never imports `internal/component/` or `internal/plugins/` (`./le arch tier check`, coreDirectionGate). The client cannot reuse the server's codec in `internal/plugins/tftpserver/handler.go`. A codec both of them share has to live in core
  → Constraint: core cannot be compiled out. The client ships in every build, `ze-stripped` included, while the server stays behind `ze_pxe`
- [ ] `ai/doors/system-plugins.md` - covers tftpserver (`ze_pxe`)
  → Constraint: the PXE TFTP clients are boot firmware Ze cannot change. Server wire behaviour must not change as a side effect of the codec move
- [ ] `docs/architecture/appliance/on-device-installer.md` - the fetch package's page (`// Design:` of fetch.go)
  → Constraint: a file fetch is written 0600, bounded at 1 GiB (`fileSizeMax`), retried `retryMax` = 3 times with a doubling `retryDelay`, and checked against an optional SHA-256. The tftp Fetcher keeps every one of these properties
  → Constraint: every site that names the URL writes `redact.URL`. A tftp URL carries no userinfo, but the same redaction path is used so no new rule appears
- [ ] `docs/architecture/provisioning/tftp-server.md` - the server page
  → Constraint: the server negotiates blksize as min(requested, 1468), clamped at no less than 8. It answers tsize with the file size, ignores windowsize (lockstep), serves octet only and refuses WRQ. The end-to-end test against it therefore exercises blksize and tsize acknowledged, and windowsize not acknowledged
- [ ] `docs/architecture/storage-backends.md` - `ze init --from` imports a blob as it is
  → Constraint: `runFetchImport` (`internal/plugins/init/main.go`) resolves the Fetcher with `fetch.Resolve`, fetches into a private staging folder, then imports. The tftp scheme is reached through this path unchanged
- [ ] `docs/architecture/testing/interop.md` - interop suites and the four vacuity traps
  → Constraint: a protocol feature needs an interop run against another implementation, and its discrimination is proven by a revert, rebuild and red

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1350.md`, `rfc/full/rfc1350.txt` - base protocol
  → Constraint: §4 "each end of the connection chooses a TID for itself, to be used for the duration of that connection". The client binds its own ephemeral port, takes the server's TID from the first reply, and ignores later packets from any other TID (sending ERROR 5 to the stranger is RFC1350-4-4, a SHOULD)
  → Constraint: §2 "Each data packet contains one block of data, and must be acknowledged by an acknowledgment packet before the next packet can be sent." In lockstep the client ACKs every DATA it accepts
  → Constraint: RFC1350-5-1 (netascii receive) and RFC1350-4-2 (WRQ ACK block 0) keep `{not-applicable}`. The client requests octet only and never writes
- [ ] `rfc/short/rfc2347.md`, `rfc/full/rfc2347.txt` - option negotiation
  → Constraint: Negotiation Protocol: "If multiple options were requested, the client must use those options which were acknowledged by the server and must not use those options which were not acknowledged by the server." (RFC2347-x-4, which this spec makes provable)
  → Constraint: "An option not acknowledged by the server must be ignored by the client and server as if it were never requested." (RFC2347-x-3, client half). A DATA 1 reply to an RRQ that carried options means no option was acknowledged, so the client falls back to 512-octet lockstep
  → Constraint: "If the transfer was initiated with a Read Request, then an ACK (with the data block number set to 0) is sent by the client to confirm the values in the server's OACK packet." and "If the client rejects the OACK, then it sends an ERROR packet, with error code 8, to the server and the transfer is terminated."
  → Constraint: "If the client receives an OACK containing an unrequested option, it should respond with an ERROR packet, with error code 8, and terminate the transfer." (RFC2347-x-6, SHOULD, which the client meets)
- [ ] `rfc/short/rfc2348.md`, `rfc/full/rfc2348.txt` - blksize
  → Constraint: "The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer." (RFC2348-x-2). An OACK blksize above the requested value, or outside 8..65464, is answered with ERROR 8
  → Constraint: "The reception of a data packet with a data length less than the negotiated blocksize is the final packet." The client ends on a DATA shorter than the negotiated size, never shorter than the size it requested
- [ ] `rfc/short/rfc2349.md`, `rfc/full/rfc2349.txt` - tsize, timeout
  → Constraint: "In Read Request packets, a size of "0" is specified in the request and the size of the file, in octets, is returned in the OACK." (RFC2349-x-3, client half)
  → Constraint: "If the file is too large for the client to handle, it may abort the transfer with an Error packet (error code 3)." (RFC2349-x-5, MAY). An acknowledged tsize above `fileSizeMax` is answered with ERROR 3 before any DATA
  → Decision: the client does not request the timeout option. It is optional to request, and tftp-hpa's client does not send it either. RFC2349-x-1 and x-2 stay server rows, and their `{not-applicable}` reason (the server lacks the option) is a server gap outside this spec
- [ ] `rfc/short/rfc7440.md`, `rfc/full/rfc7440.txt` - windowsize
  → Constraint: §3 "The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer." (RFC7440-3-4) and "The valid values range MUST be between 1 and 65535 blocks, inclusive." (RFC7440-3-2)
  → Constraint: §4 "The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception." (RFC7440-4-2). The client is the DRCV of a read
  → Constraint: §4 "Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]." (RFC7440-4-3). With windowsize unacknowledged or acknowledged as 1, the client runs plain lockstep
  → Constraint: §4 "In the case of a data block sequence error, the DRCV SHOULD notify the DSND by sending an ACK corresponding to the last data block correctly received." (RFC7440-4-6, SHOULD, which the client meets)
  → Constraint: RFC7440-4-1, 4-4 and 4-5 bind the DSND. Ze's DSND is the server, which does not implement windowsize. Those rows are untouched here
- [ ] `rfc/full/rfc3617.txt` (fetched 2026-10-30, not enrolled, and no summary is written under the scope rule) - tftp URI
  → Constraint: "tftpURI = "tftp://" host "/" file [ mode ]", with mode ";mode=" netascii or octet, and octet when left unspecified. The client accepts no mode or `;mode=octet`, and refuses `;mode=netascii` by name
  → Decision: the RFC 3617 grammar has no port. The client accepts `host:port` anyway, as curl does, because tests and non-standard deployments need it. Listed for the owner in "Owner choices"

### Reference client source (META: read the reference source and quote the deciding function before approval)
- [ ] BusyBox `networking/tftp.c`, `tftp_protocol` (fetched to the session scratch from the mirror/busybox master)
  → Decision: on an OACK it reads blksize with `tftp_blksize_check(res, blksize)`, whose upper bound is the requested size, and on failure sets `G_error_pkt_reason = ERR_BAD_OPT` and sends the error. On a non-OACK first reply its comment quotes RFC 2347 ("An option not acknowledged by the server must be ignored by the client and server as if it were never requested.") and it resets `blksize = TFTP_BLKSIZE_DEFAULT`. Ze does the same on both branches
  → Decision: BusyBox does not re-ACK a duplicate DATA (the `#if 0` block, "Disabled to cope with servers with Sorcerer's Apprentice Syndrome"). Ze's client re-ACKs the duplicate of the last accepted block instead, which is the dally behaviour RFC 1350 §6 encourages. Ze's server does not answer duplicate ACKs (`sendAndWaitACK`), so the re-ACK cannot start the apprentice loop against it
- [ ] tftp-hpa `tftp/tftp.c`, `parse_oack` and the receive loop (fetched from git.kernel.org)
  → Decision: `parse_oack` refuses a repeated option, an option not requested (`badopts = optack->options | ~optreq->options`), blksize below MIN_SEGSIZE or above the requested value, windowsize below 1 or above the requested value, and any unknown option. The caller answers `nak(name, EOPTNEG, "Invalid option response")`. Ze's OACK check takes the same set of refusals, each answered with ERROR 8
  → Decision: tftp-hpa requests windowsize only when its configured maximum is above 1 (`if (optreq->window > 1)`), and blksize from path MTU discovery. Ze requests blksize 1468 (the Ethernet bound the server already declares) and a fixed windowsize (owner choice below)

**Key insights:**
- The cap on rfc2347, rfc2348 and rfc7440 is client-only MUST rows. Only a client Ze ships can prove them, and an annotation can never raise the figure (`docs/contributing/rfc-conformance-gates.md`, "Coverage figures")
- The entry point already exists: `ze init --from <url>` resolves `fetch.Resolve`, and a scheme added to the table is reachable with no other edit
- Core cannot import the server's codec, so the wire facts (opcodes, error codes, the blksize range, option names, packet encoders and parsers) move to a new core package that both sides import. Declaring them twice is the alternative this spec rejects

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/fetch/scheme.go` - the `schemes` table (http, https maps to `ToFile`), `IsRemote`, `Resolve` (refuses an unknown scheme, naming the supported list), `Schemes`
- [ ] `internal/core/fetch/fetch.go` - `ToFile` (retry ×3 with doubling delay, a 1 GiB bound, 0600, optional SHA-256), `ToDisk`, `stallReader`, `sameOrigin`, `ValidSHA256`, `CheckSHA256`
- [ ] `internal/plugins/init/main.go` - `runImport` and `runFetchImport`: `fetch.IsRemote`, `fetch.Resolve`, fetch into the staging folder, import. Help lists `fetch.Schemes()`
- [ ] `internal/plugins/tftpserver/handler.go` - `parseRRQ`, `buildOACK`, `buildData`, `buildError`, `handleRRQ` (blksize min(req,1468), windowsize logged and ignored), `sendOACKAndWait` (OACK holds only requested blksize and tsize), `serveFile` (uint16 block counter that wraps after 65535), `sendAndWaitACK` (5 s ackTimeout, 3 retransmits, duplicate ACKs unanswered)
- [ ] `internal/plugins/tftpserver/tftp_curl_docker_test.go` - the pattern for an interop package test: a `docker && linux` tag, a digest-pinned image, `labtest.Run`
- [ ] `internal/test/fixture/storage_init_from_fixture.go`, `register_storage_init_from.go`, `test/plugin/init-from-url.ci` - the `ze init --from http://` functional scenario: an in-process httptest server, a SHA-checked import, `ze start` on the imported tree
- [ ] `internal/le/interoplab/labtest/labtest.go` - `Start` and `StartContainer` (a server container, returning its address) and `Run` (a client container)

**Behavior to preserve:**
- http and https fetches, their retry, bound, mode, redaction and refusals
- The `ze init --from` refusal text for an unknown scheme, and its help, both derived from `fetch.Schemes()` (gaining `tftp` is the intended change)
- Every byte the TFTP server puts on the wire and every server decision (blksize clamp, tsize, the windowsize fallback, error codes, duplicate-ACK silence). The codec move changes where constants and encoders are declared, never what they produce

**Behavior to change:**
- `fetch.Schemes()` gains `tftp`. `ze init --from tftp://host[:port]/path` downloads over TFTP
- Six summary rows lose `{not-applicable}` (listed under Acceptance Criteria) and gain tagged tests and records

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator runs `ze init --from tftp://host[:port]/file[;mode=octet] [--sha256 <hex>]`, or any future caller of `fetch.Resolve`
- Format at entry: a URL string

### Transformation Path
1. `runFetchImport` calls `fetch.Resolve`, which finds the `tftp` entry in `schemes`
2. The tftp Fetcher parses the URL: host, port (default 69), file, mode. A netascii mode, an empty file or a bad port is refused before any packet is sent
3. The shared retry wrapper (the one ToFile uses, generalised over a one-attempt function) runs up to `retryMax` attempts
4. One attempt: bind an ephemeral UDP socket and send an RRQ (filename, octet, blksize 1468, tsize 0, windowsize W). Wait for the first reply from the server's address on any port
5. First reply: an OACK is validated (requested options only, each once, values in range and no larger than requested). An invalid one is answered with ERROR 8 and the attempt fails. A valid one yields the negotiated blksize, windowsize and tsize, and a tsize above `fileSizeMax` is answered with ERROR 3. The client sends ACK 0. A DATA 1 means no option was acknowledged: 512 octets, window 1. An ERROR fails the attempt and names the server's code and message
6. Receive loop on the server's TID: write in-order DATA to dest (0600). ACK at the end of each window (every block when W is 1), and on a sequence error ACK the last block received in order. Retransmit the last ACK on timeout, bounded by the retransmit count. Re-ACK a duplicate of the last accepted block. Block numbers wrap from 65535 to 0. Packets from another TID get ERROR 5 and are otherwise ignored
7. End on a DATA shorter than the negotiated blksize: final ACK, then dally one timeout and re-ACK a retransmitted final DATA. Then the bound check (written bytes ≤ `fileSizeMax`), and the SHA check when one was given

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Process to TFTP server (network) | UDP, RFC 1350/2347/2348/2349/7440 octets | No |
| `ze init` to core fetch | `fetch.Resolve` returns a `Fetcher` | No |
| tftpserver plugin to core tftp codec | Go import from a gated plugin into a core leaf package (allowed direction) | No |

### Integration Points
- `schemes` in `internal/core/fetch/scheme.go`: one new entry
- The ToFile retry and SHA wrapper in `fetch.go`: generalised so http and tftp share it
- New `internal/core/tftp` package: wire constants, packet encoders and parsers, the OACK validator. Imported by `internal/core/fetch` (client) and `internal/plugins/tftpserver` (server)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The client is reached only through `fetch.Resolve` |
| No unintended coupling (components stay isolated) | Yes | Core imports neither plugin. The plugin imports core, which is the allowed direction |
| No duplicated functionality (extends existing, does not recreate) | Yes | One codec in core, shared. One retry and SHA wrapper, shared |
| Zero-copy preserved where applicable (refs, not copies) | Yes | One receive buffer of blksize+4 reused for the whole transfer. DATA payload is written from that buffer straight to the file |
| Registration over hardcoding, outbound | Yes | One `schemes` entry |
| Registration over hardcoding, inbound | Yes | Searched for lists that would need to learn "tftp": in `internal/plugins/init/main.go`, `Run` builds its help from `fetch.Schemes()` and checks `--from` with `fetch.Resolve`, and `runFetchImport` resolves with `fetch.Resolve`. `docs/guide/command-reference.md` lists schemes in prose and is edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Moving the server's constants and encoders to core leaves its wire output byte-identical | The encoders are pure functions of their inputs | Server regression against PXE firmware | The existing tftpserver tests and `TestTFTPCurlDuplicateACKInterop` stay green, unchanged | unvalidated |
| A-2 | Server discrimination records whose producer bodies change in the move go stale, and re-recording them is mechanical | `rfc/discrimination` records pin `producer-sha` | Lost proof on rfc1350/2347/2348 server rows until re-recorded | `./le rfc discriminate stem <stem>` lists stale records after the move. Each is re-recorded in the same phase | unvalidated |
| A-3 | `labtest.Start` can run a dnsmasq TFTP server container that the host test reaches over UDP | `labtest.go` Start returns a netip.AddrPort | No interop run | First run of the interop test | unvalidated |
| A-4 | Removing a `{not-applicable}` row that lacks the new evidence form lowers the `not-applicable` line of `rfc/exclusion-evidence-backlog.txt` by one each, and the ratchet requires the file to be edited to match | Gates page, "The exclusion evidence ratchet" | Commit refused | `./le rfc check` before commit | unvalidated |
| A-5 | A fixture build-tagged `ze_pxe` may import tftpserver, as a `ze_bgp` fixture already exists | `internal/test/fixture` holds one `//go:build ze_bgp` file | The `.ci` cannot use Ze's server | `./le arch tier check` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Sorcerer's Apprentice: the client re-ACKs a duplicate DATA and a naive server resends, doubling traffic | Interop run shows DATA counts above blocks | Re-ACK only the duplicate of the last accepted block, never ACK twice in answer to one DATA. Ze's server ignores duplicate ACKs |
| R-2 | Window loss handling: a lost block inside a window makes the client ACK a stale block forever | Unit test with a dropped mid-window block | ACK the last in-order block (RFC7440-4-6). The DSND restarts the window from it (RFC7440-4-4). Bounded retransmits end the attempt |
| R-3 | Block number wrap on files over 65535 blocks breaks ordering | Unit test with a scripted server across the wrap | Compare block numbers modulo 2^16 against the expected next block |
| R-4 | A hostile server sends an endless stream (no short block) | Bytes written exceed the bound | Abort at `fileSizeMax` + 1 with ERROR 3, as the http path aborts at its bound |
| R-5 | The first reply comes from an unexpected host | Unit test | Accept the first reply only from the server's IP. A later packet from any other address or port gets ERROR 5 (RFC1350-4-4) |
| R-6 | Re-recording stale server records misses one | `./le rfc discriminate stem` shows stale entries | Run it for rfc1350, 2347, 2348, 2349 and 7440 at the end of the codec phase |
| R-7 | A blocked read hangs the fetch | Test with a silent server | Every read carries a deadline, and the attempt fails after bounded retransmits |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `ze init --from tftp://` fails or imports a wrong file (caught by `--sha256` when given). If the codec move goes wrong, the TFTP server mis-encodes and PXE boots fail |
| How is it reverted? | A single revert of each commit. No config or on-disk format changes |
| Who else touches this path? | The RFC 2348 agent has finished (commit 6d45083554). No one else is editing tftpserver files |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `fetch.Resolve("tftp://...")` | → | the tftp Fetcher | `TestResolveTFTPScheme` (`internal/core/fetch/scheme_test.go`) |
| `ze init --from tftp://127.0.0.1:<port>/database.zefs --sha256 <hex>` | → | `runFetchImport` → tftp Fetcher → Ze's TFTP server | `test/plugin/init-from-tftp.ci` |
| tftp Fetcher against Ze's server `serve` loop | → | negotiation and receive loop | `TestTFTPClientFetchesFromZeServer` (`internal/plugins/tftpserver/client_e2e_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `fetch.Schemes()` | Returns `http, https, tftp` |
| AC-2 | `ze init --from tftp://127.0.0.1:<port>/database.zefs --sha256 <hex>` against Ze's TFTP server | Exits 0. The tree is imported, the staging folder is removed, `ze start` runs on it |
| AC-3 | `--from tftp://h/f;mode=netascii` | Refused before any packet, naming the mode and that only octet is supported |
| AC-4 | The RRQ the client sends | Opcode 1, filename, `octet`, blksize 1468, tsize 0, windowsize W, every field NUL-terminated (RFC7440-3-1) |
| AC-5 | OACK acknowledges blksize 1024 only, though blksize, tsize and windowsize were requested (RFC2347-x-4 positive) | Transfer uses 1024-octet blocks, window 1, and no tsize check. Ends on the first DATA shorter than 1024 |
| AC-6 | Server ignores options and answers DATA 1 of 512 octets (RFC2347-x-4 negative) | Client does not use the requested 1468. It treats 512 as a full block, ACKs it and continues to the true short block. The file is complete |
| AC-7 | OACK blksize 2000 when 1468 was requested, or blksize 7 (RFC2348-x-2 negative) | Client sends ERROR 8 and the attempt fails, naming the bad option |
| AC-8 | OACK blksize ≤ 1468 (RFC2348-x-2 positive) | Client uses exactly that size |
| AC-9 | OACK windowsize 0, 65536, or above W (RFC7440-3-2, 3-4 negative) | ERROR 8 |
| AC-10 | OACK windowsize w with 1 ≤ w ≤ W (RFC7440-3-4 positive) | Client ACKs only the last block of each window of w blocks (RFC7440-4-2) |
| AC-11 | Window with block k+2 lost (RFC7440-4-6) | Client ACKs k+1, the last in-order block |
| AC-12 | windowsize unacknowledged, or acknowledged as 1 (RFC7440-4-3) | Packet trace identical to RFC 1350 lockstep: one ACK per DATA |
| AC-13 | OACK carries an option not requested, or one option twice (RFC2347-x-6) | ERROR 8 |
| AC-14 | OACK tsize above 1 GiB (RFC2349-x-5) | ERROR 3 before any DATA |
| AC-15 | DATA from another port during the transfer (RFC1350-4-4) | ERROR 5 sent to that port, transfer continues and completes |
| AC-16 | ACK lost (server retransmits DATA n) | Client re-ACKs n once per duplicate and the transfer completes. The final DATA retransmitted during the dally is re-ACKed (RFC1350-6-2) |
| AC-17 | Silent server | Attempt fails after the bounded retransmits. Fetch fails after `retryMax` attempts with an error naming the host and the timeout |
| AC-18 | Transfer over 65535 blocks (scripted server) | Block numbers wrap to 0, the file is complete |
| AC-19 | Server sends ERROR code c with message m | Fetch fails naming c and m, with no ACK sent in answer |
| AC-20 | Server bytes unchanged after the codec move | Every existing tftpserver test passes unchanged, and `TestTFTPCurlDuplicateACKInterop` passes |
| AC-21 | `./le rfc tree` after records | rfc2347 mandatory 3 of 3, rfc2348 5 of 5, and rfc7440 gains 3-2, 3-4, 4-2 and 4-3 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Seeds a router from a TFTP-hosted blob | `ze init --from tftp://` → `fetch.Resolve` → tftp Fetcher → Ze TFTP server → import | `test/plugin/init-from-tftp.ci` |
| 2 | Fetches from a third-party TFTP server | tftp Fetcher → dnsmasq | `TestTFTPClientDnsmasqInterop` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveTFTPScheme` | `internal/core/fetch/scheme_test.go` | AC-1, the wiring | |
| `TestTFTPURLParse` | `internal/core/fetch/tftp_test.go` | host, port, file, `;mode=octet`, refusal of netascii, empty file and bad port (AC-3) | |
| `TestTFTPClientRequestOptions` | `internal/core/fetch/tftp_test.go` | AC-4 | |
| `TestRFC2347ClientUsesOnlyAcknowledgedOptions` | `internal/core/fetch/rfc2347_client_test.go` | RFC2347-x-4 positive (AC-5) and negative (AC-6) | |
| `TestRFC2347ClientRejectsUnrequestedOption` | `internal/core/fetch/rfc2347_client_test.go` | RFC2347-x-6 (AC-13) | |
| `TestRFC2348ClientBlocksize` | `internal/core/fetch/rfc2348_client_test.go` | RFC2348-x-2 both polarities (AC-7, AC-8). Final packet by negotiated size (RFC2348-x-4, client half) | |
| `TestRFC2349ClientTsize` | `internal/core/fetch/rfc2349_client_test.go` | RFC2349-x-3 client half (tsize "0" in the RRQ), RFC2349-x-5 (AC-14) | |
| `TestRFC7440ClientWindowsize` | `internal/core/fetch/rfc7440_client_test.go` | RFC7440-3-2, 3-4 (AC-9, AC-10), 4-2, 4-6 (AC-11), 4-3 (AC-12) | |
| `TestRFC1350ClientTransfer` | `internal/core/fetch/rfc1350_client_test.go` | TID handling (AC-15), re-ACK and dally (AC-16), timeout (AC-17), wrap (AC-18), ERROR (AC-19) | |
| Codec tests | `internal/core/tftp/*_test.go` | Encoders and parsers moved from the server, plus the OACK validator's refusal set | |

The client tests drive a scripted UDP peer on loopback that plays the server role packet by packet, so each polarity is a chosen OACK or reply and nothing is mocked inside the client.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OACK blksize | 8..requested (1468) | 8 and 1468 | 7 | 1469 |
| OACK windowsize | 1..W | 1 and W | 0 | W+1, and 65536 |
| OACK tsize | 0..1 GiB | 1073741824 | N/A | 1073741825 |
| URL port | 1..65535 | 1 and 65535 | 0 | 65536 |
| Block number | 0..65535 wrapping | 65535 then 0 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `init-from-tftp` | `test/plugin/init-from-tftp.ci` | The fixture `storage/init-from-tftp` (build tag `ze_pxe`) serves a seed blob from Ze's TFTP server on loopback, runs `ze init --from tftp://127.0.0.1:<port>/database.zefs --sha256`, checks the import and the removed staging folder, and starts `ze` on the tree | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestTFTPClientDnsmasqInterop` | `internal/core/fetch/tftp_dnsmasq_docker_test.go` (`docker && linux`, the tftpserver curl pattern) | dnsmasq, latest stable, digest-pinned image built from its release | Ze's client downloads an exact file with blksize and tsize negotiated, and falls back to lockstep for an unacknowledged windowsize. Red under a reverted OACK validator is recorded | |

## Files to Modify
- `internal/core/fetch/scheme.go` - the `tftp` entry
- `internal/core/fetch/fetch.go` - the retry, bound and SHA wrapper shared by http and tftp
- `internal/plugins/tftpserver/handler.go` - use the core codec. Its own constants and encoders are deleted, not kept alongside (no-layering)
- `rfc/short/rfc2347.md`, `rfc2348.md`, `rfc7440.md` - drop `{not-applicable}` from the rows this spec proves, update the Enrolment reasons
- `rfc/short/rfc2349.md`, `rfc1350.md` - Enrolment reason names the client tags
- `rfc/extraction/rfc2347.json`, `rfc2348.json`, `rfc7440.json` - site reasons that say "ze has no client"
- `rfc/audit/rfc2347/RFC2347-x-4.json` and the other changed rows' audit entries - re-stamped through `audit-stamp`, never hand-edited
- `rfc/exclusion-evidence-backlog.txt` - the `not-applicable` count drops by the rows removed
- `docs/architecture/appliance/on-device-installer.md` - the tftp scheme
- `docs/architecture/provisioning/tftp-server.md` - the codec now lives in core
- `docs/guide/command-reference.md`, `docs/guide/ze-install.md` - `--from tftp://`
- `features/installation.md` - the TFTP client
- `ai/INDEX.md` - a keyword row for the TFTP client

## Files to Create
- `internal/core/tftp/` - codec: opcodes, error codes, the blksize and windowsize ranges, option names, RRQ, ACK, ERROR, DATA and OACK encode and parse, the OACK validator. Each function quotes its RFC sentence
- `internal/core/fetch/tftp.go` - URL parse, one-attempt transfer, receive loop
- `internal/core/fetch/*_client_test.go`, `tftp_test.go`, `tftp_dnsmasq_docker_test.go` - tests above
- `internal/plugins/tftpserver/client_e2e_test.go` - Ze client against Ze server
- `internal/test/fixture/storage_init_from_tftp_fixture.go` (`ze_pxe`), its registration
- `test/plugin/init-from-tftp.ci`
- The dnsmasq interop image definition beside the test, digest-pinned

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config. The scheme is chosen by the URL |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | No | `ze init --from` already exists. Its help derives the scheme list |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | Yes | `test/plugin/init-from-tftp.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No env var. Timeouts and windowsize are constants |
| Doctor check for runtime dependencies | N-A | An outbound UDP socket to an operator-named host per invocation. No listener, file path, module or binary |
| Prometheus counters/metrics | N-A | `ze init` is a one-shot CLI, not a daemon |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/installation.md` |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`--from tftp://`) |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | tftpserver's behaviour is unchanged. Its page is updated under 12 |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` |
| 7 | Wire format changed? | No | The server's bytes are unchanged. The client's wire format goes in `docs/architecture/appliance/on-device-installer.md` |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1350.md`, `rfc2347.md`, `rfc2348.md`, `rfc2349.md`, `rfc7440.md`. `docs/features/rfc-status.md` is regenerated |
| 10 | Test infrastructure changed? | No | A fixture follows the existing pattern |
| 11 | Affects daemon comparison? | No | Not a routing daemon feature |
| 12 | Internal architecture changed? | Yes | `docs/architecture/provisioning/tftp-server.md` (codec in core), `docs/architecture/appliance/on-device-installer.md` (scheme) |
| 13 | Route metadata keys? | No | None |
| 14 | Prometheus counters? | No | None |
| 15 | Registered plugin/command inventory changed? | No | None |
| 16 | Changed source files referenced by doc anchors? | Yes | Run `./le spec citation anchors spec plan/post-release/features/spec-tftp-client.md` in phase 1. Expected: on-device-installer.md (fetch.go, scheme.go), tftp-server.md (handler.go) |
| 17 | Existing docs show examples for this area? | Yes | The `ze init --from` examples in `docs/guide/command-reference.md` and `quickstart.md` are checked |

## Implementation Steps

1. **Phase: Wiring** - the `tftp` entry in `schemes` pointing at a stub Fetcher that refuses with "not implemented". `TestResolveTFTPScheme` passes. The `.ci` and `TestTFTPClientFetchesFromZeServer` fail on the stub
2. **Phase: Core codec** - create `internal/core/tftp`. Move the server's constants and encoders and parsers into it, delete them from handler.go, and point the server at core. Every tftpserver test passes unchanged (AC-20). Then run `./le rfc discriminate stem` for rfc1350, rfc2347, rfc2348, rfc2349 and rfc7440, and re-record every stale server record. This phase is its own commit
3. **Phase: Client lockstep** - URL parse, RRQ, the DATA 1 fallback, the receive loop, TIDs, timeouts, re-ACK, dally, wrap, ERROR, and the shared retry, bound and SHA wrapper. Tests for AC-3, 4, 6, 12, 15 to 19
4. **Phase: Client options** - the OACK validator in core, ACK 0, blksize, tsize, windowsize and the window receive. Tests for AC-5, 7 to 11, 13, 14
5. **Phase: End to end** - `TestTFTPClientFetchesFromZeServer`, the `ze_pxe` fixture, `init-from-tftp.ci` (AC-2)
6. **Phase: Interop** - dnsmasq image and test. Record the red under a reverted validator
7. **Phase: RFC ledger** - drop `{not-applicable}` from RFC2347-x-4, RFC2348-x-2, RFC7440-3-2, 3-4, 4-2 and 4-3. Tag the client tests (both polarities), add client tags on RFC2347-x-3, RFC2348-x-4, RFC2349-x-3, RFC2349-x-5, RFC2347-x-6, RFC7440-4-6 and RFC1350-4-4, 6-2. Run `./le rfc record` for each. Update the extraction reasons, audit stamps, the backlog count and the Enrolment reasons. `./le rfc tree` shows AC-21
8. **Phase: Docs** - each page in its phase. Listed here only as a check

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The final packet is judged against the NEGOTIATED blksize (512 when unacknowledged), never against the requested size |
| Correctness | No ACK is sent in answer to an ERROR. ERROR 8 is sent for every OACK refusal and ERROR 3 for the tsize bound |
| Correctness | Server output byte-identical after the codec move |
| Data flow | The client is reached only through `fetch.Resolve`. No second entry |
| Rule: no-layering | No server constant or encoder survives beside the core one |
| Rule: rfc-compliance | Every implementing function quotes its RFC sentence. Every caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| tftp scheme | `TestResolveTFTPScheme` |
| End to end | `./le test plugin init-from-tftp` |
| Interop | `TestTFTPClientDnsmasqInterop` output |
| Ledger | `./le rfc tree` lines for the five stems |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Every OACK field length and value checked. No index past the received length. Malformed packets are dropped or refused, never a panic |
| Resource exhaustion | The 1 GiB bound is enforced while writing. Retransmits and attempts are bounded. Read deadlines everywhere |
| Spoofing | Only the server's IP may set the TID. Only that TID is accepted afterwards |
| Integrity | TFTP has no authentication, so the docs tell operators to pass `--sha256` |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Shared codec in a new core package `internal/core/tftp`, server migrated | (b) The client declares its own opcodes and encoders in fetch. (c) The client lives in the tftpserver plugin | (b) declares every wire fact twice (`principles.md`, declare once). (c) is impossible because core cannot import a plugin, and the client would vanish from `ze_pxe`-less builds. The cost of the choice is re-recording the stale server records |
| The client is a read-only DRCV, octet only | Add WRQ (put) and netascii | No caller writes, and `ze init` imports a binary blob. RFC1350-5-1 and 4-2 stay `{not-applicable}` with a client-side reason added |
| Request blksize 1468, tsize 0, windowsize W, no timeout | Path-MTU blksize as tftp-hpa does, or request timeout | 1468 is the bound the server already declares. Requesting timeout is optional, and the reference client does not |
| The retry, bound and SHA wrapper is shared with http | A tftp copy of ToFile's loop | One retry policy, declared once |
| Interop against dnsmasq | tftp-hpa 5.2 (2011, no windowsize), or tftp-hpa from git | dnsmasq is current stable and widely deployed for PXE. tftp-hpa's release is old, and its git head would be built from source |

## Known Limitations
- The server still does not implement the RFC 2349 timeout option or RFC 7440 windowsize as DSND. RFC2349-x-1, x-2 and RFC7440-4-1, 4-4, 4-5 stay capped. Their current `{not-applicable}` reasons say "ze does not implement", which the gates page calls a `{gap}`. That is a server-side spec of its own (not written here; named in the report to the owner)
- No TFTP upload (WRQ)

## Owner choices (await Thomas)
| Choice | Recommendation | What each answer changes |
|--------|----------------|-------------------------|
| windowsize W that the client requests | 4. RFC 7440 §5 "Operators SHOULD test various values and SHOULD be conservative". With no operator knob, a small fixed value | 1 would leave RFC7440-3-4, 4-2 and 4-6 unprovable. A larger W is faster on a clean LAN and worse under loss |
| Accept `host:port` in a tftp URL, outside the RFC 3617 grammar | Accept, as curl does | Refusing it means the `.ci` and interop tests need port 69 (root) |

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
- [ ] **Commit B:** `remove plan/post-release/features/spec-tftp-client.md` only, in the same `./le commit create` script

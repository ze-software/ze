# Spec: tftp-server-uploads

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | An absent RFC feature, not a defect: the read-only server is correct for what it ships, and no release profile declares RFC 1350, 2347, 2348, 2349 or 7440 (`rfc/profiles/dhcp-provisioning.md` is a technology profile, Release n/a), so the work waits for after the first release (`ai/META-DECISIONS.md`, release timing) |
| RFC gaps | RFC1350-4-2, RFC1350-5-1, RFC2349-x-4, RFC2348-x-4, RFC7440-4-2, RFC7440-4-6 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's TFTP server (`internal/plugins/tftpserver`) answers every write request
(WRQ) with ERROR 4, "write not supported" (`serveRequests`, `handler.go`;
`TestTFTPWriteRejected`). On 2026-10-31 Thomas decided that TFTP uploads are in
scope for Ze's TFTP server. This spec adds them: a WRQ from an allowed source is
received into a dedicated upload directory, with RFC 2347 option negotiation and
the RFC 2348 blksize, RFC 2349 timeout and tsize, and RFC 7440 windowsize
options on the receiving side.

Uploads are off unless the operator turns them on, because plain TFTP carries
no authentication (RFC 1350 Security Considerations: "TFTP is often installed
with controls such that only files that have public read access are available
via TFTP and writing files via TFTP is disallowed.").

The use cases are the ones TFTP uploads serve on a network: a switch or router
copying its running config or a crash dump to a TFTP host, and a lab dropping a
file onto a Ze box. Nothing in Ze consumes an uploaded file (D-11).

### Scope decision

| Decision | Source | Where else recorded |
|----------|--------|---------------------|
| TFTP uploads (write requests) are in scope for Ze's TFTP server | Thomas, 2026-10-31 | `rfc/short/rfc1350.md` and `rfc/short/rfc2349.md`, `Enrolment reason` |

### Owner decisions this design needs

Every choice below is one only Thomas can make: no RFC sentence, agreed design
or `ai/META-DECISIONS.md` rule answers it. The design in this spec is written
on the recommendation of each row. A different answer changes what the last
column names.

| # | Decision | Options | Recommendation | What it changes |
|---|----------|---------|----------------|-----------------|
| D-1 | Where uploads land | (a) a dedicated `upload directory`, refused when it equals, sits inside, or contains `root-directory`; (b) the same directory as `root-directory`; (c) any directory, overlap allowed | (a). An upload can then never be served to a PXE ROM as a bootloader: a LAN host cannot replace `ipxe.efi` by uploading it. tftp-hpa writes into its read tree, which is how a TFTP server becomes a boot-image injection point | (b) or (c): the read path must refuse temp-file names, an upload changes what PXE clients boot, RFC 1350 site 5:2 binds (Ze returns what it received) and needs a round-trip test, and AC-22's overlap refusal goes |
| D-2 | Overwrite of an existing file | (a) leaf `overwrite`, default false; (b) never; (c) always | (a). RFC 2347, 2348 and 2349 each say TFTP "has no rename, delete, or file overwrite capabilities" (Security Considerations), so refusing is the default; a device that saves `running.cfg` under the same name every night needs the leaf | (b): leaf and AC-6 go. (c): AC-5 goes, and any allowed host can destroy any stored file |
| D-3 | `max-file-size` default and range | default 64 MiB, range 1 octet to 4 GiB; or another default | 64 MiB with that range. Config backups are kilobytes and crash dumps a few megabytes; a firmware drop needs the operator to raise it | The YANG default, the docs table, AC-7 and AC-8 boundary values |
| D-4 | Concurrent uploads | (a) leaf `max-uploads`, default 2, range 1 to 1000, each upload also taking a `max-transfers` slot; (b) uploads share `max-transfers` only | (a). Writes cost disk and reads serve boots; a cap of their own keeps uploads from starving a PXE wave | (b): leaf and AC-19's upload cap go, and a burst of uploads can take every transfer slot |
| D-5 | Disk-space reserve | (a) leaf `reserved-space`, default 128 MiB: a WRQ is admitted only when the filesystem's free space, less the sizes already admitted to running uploads and less this upload's size (its tsize, else `max-file-size`), stays above the reserve; (b) no reserve, ERROR 3 on ENOSPC only | (a). Without it an allowed host fills the filesystem Ze's own database lives on before any write fails | (b): leaf, AC-7's space refusal and the admission ledger go; ENOSPC handling stays either way |
| D-6 | Who may upload | (a) leaf-list `allow-from` of prefixes, required non-empty when uploads are enabled; (b) an empty list allows every source on the listen interfaces | (a), the `dns-resolver allow-from` model (`internal/plugins/dnsresolver/config.go` `parseAllowFrom`; verify refuses an enabled resolver with no entry). An empty list allowing everyone is how an unauthenticated write service gets left open | (b): verify stops refusing an empty list, AC-22 changes |
| D-7 | Answer to a refused source | (a) ERROR 2 "access violation"; (b) ERROR 4 as when uploads are off; (c) silence | (a). The operator who mistyped a prefix sees why; hiding that uploads exist buys little on a LAN-only service | (b) or (c): AC-3's expected answer |
| D-8 | Mode of a stored file | (a) fixed 0600; (b) fixed 0640; (c) a leaf | (a). A config backup carries secrets (keys, community strings), and Ze's process owns the file | (c) adds a leaf; AC-2 asserts the mode |
| D-9 | Subdirectories | (a) an existing subdirectory of the upload directory may be named, none is ever created; (b) flat names only | (a). Devices often name `configs/<host>.cfg`, and tftp-hpa accepts a path into an existing directory | (b): a name with a separator is refused with ERROR 2, AC-26 changes |
| D-10 | netascii | (a) WRQ accepts netascii and translates it (this spec), RRQ stays octet-only; (b) both directions in this spec; (c) WRQ refuses netascii with ERROR 4 | (a). RFC1350-5-1 binds the receiver, and Ze becomes one. Sending netascii on RRQ has no MUST row and is a separate feature | (b) adds RRQ translation, tsize for a netascii read (tftp-hpa declines tsize there), and tests; (c) leaves RFC1350-5-1 a gap, which `ai/META-DECISIONS.md` makes owed work, so it needs a reason only Thomas can give |
| D-11 | What an uploaded file triggers | (a) nothing: Ze stores it, logs it with its SHA-256 and counts it; (b) an event other plugins may subscribe to | (a). No Ze consumer exists, and a hook is a second feature with its own trust question (an unauthenticated file feeding automation) | (b) adds an EventBus event, its JSON shape and docs |

Decided without asking, with the source:

| Choice | Answer | Source |
|--------|--------|--------|
| Release bucket | post-release feature, P3 | `ai/META-DECISIONS.md`: "RFCs no release profile declares wait until after the first release"; no release profile lists the TFTP RFCs |
| Answer to a WRQ while uploads are off | ERROR 4 "write not supported", unchanged | Current behaviour, `TestTFTPWriteRejected`; dnsmasq answers every WRQ the same way (`src/tftp.c`: `tftp_err(ERR_ILL, packet, _("unsupported write request from %s"), ...)`) |
| Error code for a size the server refuses | 3 | RFC 2349: "If the file is too large for the server to handle, it may abort the transfer with an Error packet (error code 3)." tftp-hpa answers 2 here (`nak(EACCESS, "upload exceeds maximum size")`); the RFC's code wins |
| tsize echo | the client's value, unchanged, in the OACK | RFC2349-x-4 |
| When the final ACK is sent | after the file is in place under its name | tftp-hpa sends the final ACK after `write_finish` (`common/tftp-xfer.c`, `tftp_xfer_recv`); a client that saw success finds the file |
| `mail` mode | ERROR 4 | RFC 1350 section 1: "The mail mode is obsolete and should not be implemented or used." |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the platform rules every change respects
  → Constraint: a plugin may run external; the upload writer runs inside the tftpserver plugin's own process on the same host and reaches no other component, so no cross-boundary call is added
  → Constraint: a readiness check is registered by the package that owns the dependency, so the upload-directory doctor check lives in `internal/plugins/tftpserver`, not `internal/component/doctor`
  → Constraint: a goroutine is a long-lived worker, never one per event; an upload is a transfer, and transfers already run one goroutine each under the `max-transfers` semaphore (`runTransfer`), so the upload follows that existing shape rather than adding a new one
- [ ] `ai/doors/system-plugins.md` - the door for `internal/plugins/tftpserver`
  → Constraint: tftpserver compiles out with `ze_pxe` (`feature-gates.txt` line 114); the upload code lives in the same package and is gated with it, and nothing always-on imports it
  → Constraint: TFTP clients during PXE are boot firmware Ze cannot change, so the RRQ wire behaviour stays byte-identical; every existing RRQ test stays green unmodified
  → Constraint: a leaf-list is read with `configvalue.LeafList`, never a slice assertion (the door's imageserver trap)
- [ ] `ai/doors/core.md` - `internal/core`, which the design reuses
  → Decision: free space comes from `internal/core/diskspace.Free`, the one statfs answer in the tree; no second statfs call is written
  → Constraint: `./le arch fs-persistence check` refuses raw `O_CREATE`, `Rename` and `Link` writes in `internal/plugins` unless allowlisted with a reason. Uploaded files are operator data produced for another consumer, not Ze runtime state, so `internal/plugins/tftpserver/upload_store.go` gets a `fileAllowlist` entry in `internal/le/arch/fspersistence/fspersistence.go` stating that
- [ ] `ai/doors/le-tooling.md` - the fixture and test trees
  → Constraint: a `.ci` lives under `test/<suite>/` and runs through `./le test <suite>`; the `client/tftp-put` fixture registers in `internal/test/fixture/register_service_client.go` beside `client/tftp-get` and returns an error when its assertion fails
  → Constraint: a bare `go test` omits the feature tags, so the package runs through `./le job run label unit-pkg command go test -tags ...` with `ze_pxe`; the Docker interop tests carry `docker && linux`
- [ ] `ai/rules/architecture.md` - placement and persistence
  → Constraint: the raw writes are legitimate as "artifacts produced for another consumer" and carry the allowlist entry above; nothing is stored in the config or state directory, and verify refuses an upload directory inside Ze's config directory for the same reason
- [ ] `ai/rules/config.md` - config leaves
  → Constraint: every new leaf carries `ze:help` (one line, at most 96 characters and 25 words) and a different `description`; `./le doc yang-contract help-shape` refuses either missing
  → Constraint: every delivered value is a JSON string, so each coercion has a `case string:` arm; `allow-from` is read with `configvalue.LeafList` (`./le config coercion check`)
  → Constraint: leaf names are spelled out in full: `max-file-size`, `max-uploads`, `reserved-space`, `allow-from`, `directory`, `overwrite`
- [ ] `ai/patterns/config-option.md` - the YANG leaf template
  → Decision: no env var: none of the new values is an emergency override, debug knob, bootstrap value or internal safety cap
- [ ] `docs/contributing/rfc-conformance-gates.md` - tags, records and summary rows
  → Constraint: each new RFC-tagged unit owes a discrimination record from `./le rfc record` in the same commit as its code; `./le rfc check` refuses a tagged unit the tip commit added with none
  → Constraint: a summary row's text is a verbatim span of the RFC section it cites; rows are added or changed through `/ze-rfc`, never by hand in the implementing diff
- [ ] `docs/architecture/provisioning/tftp-server.md` - the TFTP server page
  → Decision: the page's "Read-only" decision ("A write request is always rejected with error code 4 ... refusing writes removes the whole write attack surface") is replaced by an "Uploads are off by default" decision naming the controls in this spec
  → Constraint: "Config reload": a reload changing only `timeout` or `max-windowsize` keeps transfers; any other change rebinds and cuts every transfer in flight. Upload leaves are not added to the limits-only set (`onlyLimitsChange`), so turning uploads off cuts the uploads in flight
  → Constraint: "Only the ACK timeout causes a retransmission", four sends of a window then the transfer ends (`maxRetransmit` 3); the receiver uses the same bound for its ACK resends
  → Constraint: "Panic containment": `transferTask` contains a panic in `runTransfer`; a contained panic mid-upload must also remove its temp file, so the cleanup is deferred inside the upload and runs before the guard's own deferred handler returns
- [ ] `docs/architecture/testing/interop.md` - "Ze's TFTP server against tftp-hpa and curl options"
  → Decision: the upload interop tests are Docker-tagged package tests in `internal/plugins/tftpserver`, beside `TestTFTPHPAWindowsizeInterop` and the curl tests, reusing `test/interop-tftp/Dockerfile.tftp-hpa` (tftp-hpa 7.2, pinned commit af23362) and `curlimages/curl:8.23.0`
  → Constraint: each interop test is proven by a recorded break: revert the behaviour, rebuild what the test drives, see red, restore, see green

- [ ] `docs/architecture/config/transaction-protocol.md` - declared by `config_tx.go`
  → Constraint: verify refuses a bad `upload` block before apply, and a failed apply rolls back to the running config; the upload state is part of what `install` builds and `restore` rebinds, so a refused reload leaves the running uploads untouched
- [ ] `docs/architecture/core-design.md` - declared by `internal/le/arch/fspersistence/fspersistence.go`
  → Constraint: runtime state belongs in managed storage; an uploaded file is operator data for another consumer, not runtime state, which is the reason the allowlist entry gives. The page itself is unaffected
- [ ] `docs/features/ai-first.md` - declared by `internal/core/diagnostic/codes.go`
  → Constraint: a new diagnostic code is declared there with its summary and example so `ze show diagnostic doctor-tftp-upload-directory` answers; the page itself is unaffected

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1350.md` - base TFTP
  → Constraint: RFC1350-4-2: "Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero." (§4)
  → Constraint: RFC1350-5-1: "A host which receives netascii mode data must translate the data to its own format." (§5)
  → Constraint: RFC1350-2-3: "All packets other than duplicate ACK's and those used for termination are acknowledged unless a timeout occurs [4]." (§5) The receiver ACKs every DATA, a duplicate of the last block included
  → Constraint: RFC1350-5-3, the length rule, now binds the server as receiver: a DATA of fewer than the block size ends the upload
  → Constraint: RFC1350-6-2 (SHOULD): "the host sending the final ACK will wait for a while before terminating in order to retransmit the final ACK if it has been lost" (§6); the server is that host in a WRQ
  → Constraint: extraction site 5:2 ("If a host receives a octet file and then returns it, the returned file must be identical to the original.") is excluded binds-another-role with a reason saying the server "refuses WRQ" and that "Ze has no TFTP client" (`rfc/extraction/rfc1350.json`, `rfc/corrections/rfc1350.md`); both become false (the client landed in fec7380fb1, uploads land here), so `/ze-rfc` re-walks the site. Under D-1 (a) the server still never returns a received file over TFTP; the stored file's byte identity is tested regardless (AC-2)
- [ ] `rfc/short/rfc2347.md` - option negotiation
  → Constraint: Negotiation Protocol, the WRQ answers: "OACK - acknowledge of Write Request and the options; ACK - acknowledge of Write Request, but not the options; ERROR - the request has been denied."
  → Constraint: "If the transfer was initiated with a Write Request, then the client begins the transfer with the first DATA packet, using the negotiated values. If the client rejects the OACK, then it sends an ERROR packet, with error code 8, to the server and the transfer is terminated."
  → Constraint: RFC2347-x-1 (no unrequested option in the OACK) and x-3 (an unacknowledged option is ignored) hold for the WRQ OACK as for the RRQ one
- [ ] `rfc/short/rfc2348.md` - blksize
  → Constraint: RFC2348-x-1 and x-3 apply to a WRQ unchanged (the RFC modifies "The TFTP Read Request or Write Request packet"); RFC2348-x-4, "The reception of a data packet with a data length less than the negotiated blocksize is the final packet.", binds the server once it receives
- [ ] `rfc/short/rfc2349.md` - timeout and tsize
  → Constraint: RFC2349-x-4: "In Write Request packets, the size of the file, in octets, is specified in the request and echoed back in the OACK." (Transfer Size Option Specification)
  → Constraint: RFC2349-x-6 (MAY): "If the file is too large for the server to handle, it may abort the transfer with an Error packet (error code 3)."
  → Constraint: RFC2349-x-1 and x-2 (timeout echoed unchanged, 1 to 255) apply to a WRQ unchanged
- [ ] `rfc/short/rfc7440.md` - windowsize
  → Constraint: RFC7440-4-2: "The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception." (§4) The server is the DRCV in a WRQ
  → Constraint: RFC7440-4-6 (SHOULD): "In the case of a data block sequence error, the DRCV SHOULD notify the DSND by sending an ACK corresponding to the last data block correctly received." (§4)
  → Constraint: RFC7440-3-3: the granted windowsize is at most the request; Ze grants at most `max-windowsize` (`grantWindowsize`), the same for both directions

### Reference implementations read
- [ ] tftp-hpa, master f2d2b5d (2026-10-12), `tftpd/tftpd.c` and `common/tftp-xfer.c`; the interop client is release 7.2
  → Decision: no create without an explicit switch: tftpd opens a WRQ target without `O_CREAT` unless `-c`/`--create` is given, and then with `O_CREAT | O_NOFOLLOW` ("O_NOFOLLOW here prevents creating a file at the end of symlink"); without `-p` it writes only a file with global write permission. Ze's `upload enabled` plays the role of `-c`, and `overwrite` replaces the permission-bit rule
  → Decision: tftpd writes in place and truncates (`ftruncate(fd, 0)`), and on failure `discard_upload` truncates an existing file or unlinks a created one. Ze writes a temp file and links or renames it, so a failed upload never damages an existing file and a reader never sees half a file
  → Decision: `--max-upload` caps every WRQ; a tsize above it is refused up front, and a tsize below it lowers the cap to the tsize ("verify up front that the upload is not too big, and cap the upload size to match the tsize"). Ze does the same: data beyond the declared tsize is refused
  → Decision: the WRQ tsize is echoed unchanged (`TSIZE_WRITE`: "WRQ: echo back the tsize specified.")
  → Decision: the receive loop ACKs the last contiguous block on any out-of-order DATA ("RFC 7440 recovery is driven by the last contiguous block, including when it falls within the current window."), ACKs at the end of each window and on the final block, writes the data out before the final ACK, and on a timeout re-sends the last ACK (or the OACK while no DATA has arrived)
  → Decision: a DATA longer than the block size ends the transfer with ERROR 4 ("data packet too large"); Ze does the same
  → Constraint: a duplicate WRQ is handled by a file lock ("A duplicate RRQ or (worse!) WRQ packet could really cause havoc..."); Ze holds an in-memory set of names being uploaded instead, since one process owns the directory
- [ ] dnsmasq, 2463798 (2026-10-26), `src/tftp.c`
  → Decision: dnsmasq has no upload support and answers every WRQ with ERROR 4; Ze's answer while uploads are off matches it

**Key insights:**
- The request loop dispatches on opcode in `serveRequests`; WRQ is one `case` today that writes ERROR 4. The upload path becomes `handleWRQ` beside `handleRRQ`, sharing the request parser (`parseRRQ`, renamed for both opcodes), the semaphore, the transfer generation and the panic guard
- `resolvePath` checks a path by `EvalSymlinks` then a prefix comparison, which is racy and needs the file to exist. Uploads use `os.Root` (go 1.27.3 in `go.mod`; already used in `internal/le` and `internal/test/runner`), whose `OpenFile`, `Link`, `Rename`, `Remove` and `Lstat` refuse any path that escapes the root, symlinks included
- Atomic no-overwrite: write `.ze-upload-<random>` in the upload directory's top level with `O_CREATE|O_EXCL`, fsync, then `Root.Link` it to the target (fails with EEXIST if any entry of that name exists, a dangling symlink included) and remove the temp name. Overwrite: `Root.Rename` over the target. Both happen before the final ACK
- `internal/core/diskspace.Free` answers free bytes for a path; the admission ledger uses it
- The `dns-resolver allow-from` leaf-list of `zt:ip-prefix` is the existing access-list pattern, refused empty when enabled
- No client in the tree can put a file: Ze's TFTP client (`internal/core/fetch/tftp.go`) only reads, and the functional fixture has `client/tftp-get` only. A `client/tftp-put` fixture is added for the `.ci` test; the interop proof uses curl and tftp-hpa

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/tftpserver/handler.go` (540L) - `parseRRQ` reads filename, mode and the blksize, timeout, tsize and windowsize options; `resolvePath` confines a read under `root-directory`; `serveRequests` dispatches RRQ to `handleRRQ` and answers WRQ with ERROR 4 "write not supported"; `handleRRQ` refuses non-octet modes with ERROR 4, names longer than 255 with ERROR 1, path escapes with ERROR 2, a full semaphore with ERROR 0 "service unavailable"; `runTransfer` dials a connected UDP socket per transfer (`udp4`), joins the generation, contains panics and runs `sendOACKAndWait` then `serveFile`; `sendAndWaitACK` resends at most `maxRetransmit` (3) times
  → Constraint: the connected per-transfer socket means the kernel drops packets from another TID, so no ERROR 5 is sent to a stranger; the upload path inherits that behaviour (RFC1350-4-4 is a SHOULD the server does not send today, unchanged here)
- [ ] `internal/plugins/tftpserver/window_send.go` - `serveFile`, `windowSender`, `windowPosition` (the 16-bit block distance used for wrap)
  → Constraint: the receiver places a DATA block by its 16-bit distance from the last acknowledged block, the same arithmetic, so an upload past block 65535 wraps to 0 the way the sender does
- [ ] `internal/plugins/tftpserver/config.go` - `tftpConfig`, `parseConfig`, `rangedLeaf`, `verifyConfig` (refuses `enabled` without `root-directory`)
- [ ] `internal/plugins/tftpserver/config_tx.go` - `parseTFTPSections`, `tftpServer.install`, `onlyLimitsChange`, `limitsOf`, `bindTFTP`, `restore`
  → Constraint: `install` either keeps everything (limits-only change) or ends the generation and rebinds; the upload state (the `os.Root`, the in-flight name set, the space ledger) is built per install and dies with its generation
- [ ] `internal/plugins/tftpserver/register.go` - registration, `Description` "TFTP server: read-only file serving for PXE boot", `ConfigureMetrics`
- [ ] `internal/plugins/tftpserver/serve_conn.go` - `ServeConn`, the read-only server the `ze init --from tftp://` fixture uses; stays read-only
- [ ] `internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang` - container description "refuses every write request, so no client can put a file on this host", `ze:help` "Read-only TFTP server (RFC 1350)"
- [ ] `internal/plugins/tftpserver/handler_test.go` - `TestTFTPWriteRejected`, `buildWRQPacket`
- [ ] `internal/plugins/tftpserver/tftp_hpa_docker_test.go`, `tftp_curl_docker_test.go` - the relay-based interop harness
- [ ] `internal/plugins/dnsresolver/config.go` - `parseAllowFrom`, the enabled-with-empty-list refusal
- [ ] `internal/core/diskspace/diskspace.go` - `Free`
- [ ] `internal/plugins/dhcpserver/doctor.go` - the plugin-owned doctor check shape
- [ ] `internal/test/fixture/service_client_fixture.go` - `serviceClientTFTPGet`
- [ ] `internal/le/arch/fspersistence/fspersistence.go` - `fileAllowlist`, the raw-write verbs it flags (`Rename`, `Link`, `O_CREATE`)
- [ ] `test/reload/tftpserver-reload.ci` - `skip-os linux`: an unprivileged Linux host cannot bind port 69

**Behavior to preserve:**
- Every RRQ behaviour and its tests, unmodified
- A WRQ while uploads are off (no `upload` container, or `upload enabled false`) gets ERROR 4 "write not supported" from the listening socket, and nothing is opened; `TestTFTPWriteRejected` stays as it is
- `ServeConn` stays read-only
- The reload semantics of the page's "Config reload" section

**Behavior to change:**
- With `service tftp-server upload enabled true`, a WRQ is answered per this spec
- The YANG container description and `ze:help`, the registration `Description`, the page's "Read-only" decision, the configuration guide's "It refuses every write request", the plugin table row in `docs/guide/plugins.md`, and the tftpserver sentence of `features/installation.md`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A UDP datagram on port 69 of a listen interface, opcode 2 (WRQ): filename, mode, then option and value pairs, each NUL-terminated
- Config: `service tftp-server upload { ... }` delivered as JSON strings through `sdk.ConfigTx`

### Transformation Path
1. `serveRequests` reads the datagram and dispatches opcode 2 to `handleWRQ` with the upload state of the running generation
2. `handleWRQ`: uploads off → ERROR 4; source not in `allow-from` → ERROR 2; parse; mode check (octet, netascii; else ERROR 4); name check and `os.Root` lookup of the parent and target (ERROR 2, 1 or 6); tsize against `max-file-size` and the space ledger (ERROR 3); in-flight name and duplicate-WRQ check; semaphore slots (`max-transfers` and `max-uploads`, else ERROR 0); temp file created; transfer goroutine started
3. `runTransfer` (shared) dials the transfer socket, joins the generation and installs the panic guard; for an upload it runs the receiver: OACK or ACK 0, then DATA blocks into the temp file (netascii translated), an ACK per window, size caps enforced as data arrives
4. On the final block: fsync, link (no overwrite) or rename (overwrite) into place, send the final ACK, dally one timeout, release the ledger entry and the name; log and count
5. On any failure: send ERROR where the RFC calls for one, close and remove the temp file, release the ledger entry, the name and both slots; count the failure

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ plugin | TFTP datagrams on the listening and per-transfer sockets | No |
| Config ↔ plugin | `sdk.ConfigTx` verify and apply of `service tftp-server` | No |
| Plugin ↔ filesystem | `os.Root` opened on `upload directory` at install | No |
| Plugin ↔ metrics | counters registered through `ConfigureMetrics` | No |
| Plugin ↔ doctor | `diagnostic.DoctorCheck` registered from `register.go` | No |

### Integration Points
- `serveRequests` - the WRQ `case` calls `handleWRQ` instead of writing ERROR 4 unconditionally
- `runTransfer` - runs the receiver for an upload, the sender for a read
- `grantWindowsize`, `optionInRange`, `buildOACK`, `sendAndWaitACK` - reused as they are
- `transferGeneration` - an upload joins it, so a reload cuts it
- `tftpGuard`, `transferTask` - contain an upload's panic

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at review: WRQ enters through `serveRequests` like RRQ |
| No unintended coupling (components stay isolated) | No | to be shown: no import outside `internal/core` and the SDK |
| No duplicated functionality (extends existing, does not recreate) | No | to be shown: request parser, OACK builder, window grant, generation, guard and `diskspace.Free` are shared |
| Zero-copy preserved where applicable (refs, not copies) | No | to be shown: one receive buffer per transfer, written straight to the file |
| Registration over hardcoding, outbound | No | to be shown: the doctor check and metrics register from the plugin |
| Registration over hardcoding, inbound | No | to be shown: no list outside the plugin learns the word "upload" except the fs-persistence allowlist, which is an allowlist by design; the searched lists are named at review |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `os.Root` refuses every escape: `..`, an absolute name, and a symlink anywhere on the path that leads outside, for `OpenFile`, `Link`, `Rename`, `Remove` | Go `os.Root` documentation; go 1.27.3 in `go.mod` | Path confinement needs an `openat2(RESOLVE_IN_ROOT)` implementation instead | `TestTFTPUploadConfinement` with each escape shape, run on Linux | unvalidated |
| A-2 | `Root.Link` of the temp file to the target fails with EEXIST when any entry of that name exists, a dangling symlink included | link(2) semantics | No-overwrite is not atomic | `TestTFTPUploadNoOverwriteRace` (two uploads of one name, one wins, the other gets ERROR 6) and the dangling-symlink case | unvalidated |
| A-3 | curl 8.23.0 `-T` sends tsize with the file size in its WRQ and starts DATA 1 on an OACK | curl's TFTP client sends tsize on upload when the size is known | x-4's interop proof rests on tftp-hpa alone | `TestTFTPCurlUploadInterop` asserts the WRQ's tsize through the relay | unvalidated |
| A-4 | tftp-hpa 7.2 `put` requests blksize, tsize and windowsize 64 by default, as it does for `get` | `test/interop-tftp/Dockerfile.tftp-hpa` comment | The windowed-receiver interop needs `-W` | `TestTFTPHPAUploadInterop` asserts the WRQ options through the relay | unvalidated |
| A-5 | Linking or renaming a temp file from the upload directory's top level into an existing subdirectory stays on one filesystem | a subdirectory is rarely a mount point | EXDEV on commit | commit failure answers ERROR 0 and removes the temp file (AC-27); the doctor check reports a subdirectory on another device | unvalidated |
| A-6 | The `ze` process user can write the configured directory | install creates nothing outside it | Every upload fails | install opens the root and probes a temp-file create, refusing the commit with the fix; doctor check | unvalidated |
| A-7 | An unprivileged Linux host cannot bind port 69, so the `.ci` upload test runs on darwin only, as `test/reload/tftpserver-reload.ci` does | that test's `skip-os linux` note | The functional proof is darwin-only; the Linux proof is the Go and Docker tests | read the runner's harness note at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An allowed host fills the filesystem Ze's database lives on | doctor reports free space under the reserve | D-5 ledger admission, ENOSPC answered with ERROR 3 and cleanup, `max-uploads` |
| R-2 | A half-written file is left under the target name | a reader sees a short file | temp name plus link or rename, only after fsync |
| R-3 | A temp file survives a crash | `.ze-upload-*` entries in the directory | install removes top-level `.ze-upload-*` entries of the upload directory before it serves (AC-21) |
| R-4 | A duplicate WRQ (the client resent it because ACK 0 was lost) starts a second transfer and a second temp file | two transfers for one client endpoint | the in-flight set is keyed by target name and holds the client endpoint; a WRQ from the same endpoint for the same name is dropped, and the running transfer resends its ACK 0 or OACK on its own timeout (AC-20) |
| R-5 | The receiver ACKs a window early or late and a windowed sender stalls | tftp-hpa interop slow or failing | unit tests for each RFC 7440 receiver rule, and the tftp-hpa windowed interop |
| R-6 | Block-number wrap past 65535 mis-places blocks | a large upload is corrupt | the receiver uses the sender's 16-bit distance arithmetic; a unit test uploads more than 65536 blocks at blksize 8 (AC-31) |
| R-7 | A reload that turns uploads off leaves an upload running | a file appears after the commit | upload leaves are outside `onlyLimitsChange`, so the generation ends and the transfer is cut (AC-11) |
| R-8 | The overlap check of D-1 is defeated by a symlink | the upload directory resolves inside the root | verify compares the paths after `filepath.EvalSymlinks`, install re-checks the opened directories, and the doctor check repeats it |
| R-9 | The netascii translator splits a CR LF across two blocks | a stray CR in the stored file | the translator carries a pending CR across blocks; AC-18 puts CR at the end of block 1 and LF at the start of block 2 |
| R-10 | A panic mid-upload leaves the temp file | temp file after a contained panic | cleanup deferred inside the upload (AC-28) |
| R-11 | The ledger leaks an admission and slowly refuses every upload | uploads refused with space available | the ledger entry is released by defer on every exit path; a unit test runs each failure path and checks the ledger returns to zero |
| R-12 | `./le arch fs-persistence check` refuses the store's writes | the gate names `upload_store.go` | the `fileAllowlist` entry with its reason lands in the same commit as the store |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | With uploads off (the default) nothing: the WRQ answer is unchanged. With uploads on, a defect could let an allowed host write outside the directory or fill the disk; the confinement, ledger and allow-list tests guard both |
| How is it reverted? | A single revert; no config migration, because `upload` is a new container that an older build refuses as unknown |
| Who else touches this path? | `plan/release/fixes/spec-iface-resolution-across-the-plugin-boundary.md` (tftpserver's `iface.Resolve` use) and `plan/release/fixes/spec-every-binder-root-stops-and-starts-around-an-address-move.md` (`bindTFTP`); neither changes the request handler |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service tftp-server upload { enabled true; directory ...; allow-from ... }` through `parseTFTPSections` and `install` | → | `bindTFTP` serving with the upload state, `handleWRQ`, the receiver | `TestTFTPUploadThroughInstall` (`config_tx_test.go`): a config applied through `sdk.ConfigTx`, then a WRQ on the bound socket stores the file |
| A WRQ on UDP 69 to a real daemon | → | the same path | `test/plugin/tftp-upload.ci` with the `client/tftp-put` fixture |
| curl `-T` and tftp-hpa `put` | → | the same path | `TestTFTPCurlUploadInterop`, `TestTFTPHPAUploadInterop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | WRQ with no `upload` container, or `upload enabled false` | ERROR 4 "write not supported" from the listening socket; no file or temp file is created (`TestTFTPWriteRejected`, unchanged) |
| AC-2 | Uploads on, source in `allow-from`, octet WRQ with no options, target absent, a 1500-octet file | ACK block 0 from a new TID (RFC1350-4-2, 4-3); every DATA is acknowledged; the last DATA (476 octets) is acknowledged only after the target exists; the stored file is byte-identical to the sent one (SHA-256 equal) and has mode 0600 (D-8) |
| AC-3 | Source outside every `allow-from` prefix | ERROR 2; nothing is created |
| AC-4 | A name with a `..` component, an absolute name, a parent that is a symlink to outside, a target that is a symlink to outside, a name over 255 octets | ERROR 2; nothing is written outside the upload directory and no temp file remains |
| AC-5 | `overwrite false`, target exists | ERROR 6 before any ACK; the existing file is unchanged |
| AC-6 | `overwrite true`, target exists | The target is replaced at the final block in one rename; a file handle opened on the old target before the upload reads the old content whole |
| AC-7 | WRQ with tsize N: N within `max-file-size` and admitted by the ledger; N above `max-file-size`; N the reserve refuses | Within: the OACK carries `tsize` with N unchanged (RFC2349-x-4). Above: ERROR 3 before any OACK or ACK, no temp file (RFC2349-x-6). Reserve: ERROR 3, no temp file |
| AC-8 | No tsize and data passes `max-file-size`; tsize N declared and data passes N | ERROR 3 at the first block past the cap; the temp file is removed and the target is absent |
| AC-9 | Client goes silent after block k | The last ACK is resent at each timeout, at most 3 times, then the transfer ends; the temp file is removed; both slots, the name and the ledger entry are released |
| AC-10 | Client sends ERROR mid-upload, or ERROR 8 in answer to the OACK | The transfer ends at once; the temp file is removed; nothing is stored |
| AC-11 | Reload that changes any `upload` leaf, or disables uploads, during an upload | The upload is cut; the temp file is removed; the target is absent; the next WRQ takes the new config |
| AC-12 | WRQ blksize 1200, 60000, 5 | OACK blksize 1200, 1468, none (RFC2348-x-1, x-3); a DATA longer than the negotiated size draws ERROR 4 and the upload ends with cleanup; a DATA shorter than the negotiated size ends the upload (RFC2348-x-4 as receiver) |
| AC-13 | WRQ windowsize 8, `max-windowsize` 64, 20 blocks | OACK windowsize 8; ACKs are sent for blocks 8, 16 and the final block 20 only (RFC7440-4-2); windowsize 1 alone draws no OACK and every block is acknowledged (RFC7440-4-3) |
| AC-14 | Inside a window, block n+2 arrives before n+1 | The ACK of block n is sent at once (RFC7440-4-6); block n+2 is not written; the upload completes when the sender restarts from n+1 |
| AC-15 | WRQ timeout 1, timeout 255, timeout 0 or 256 | 1 and 255 are echoed and used as the ACK resend interval; 0 and 256 are not acknowledged and the configured `timeout` applies (RFC2349-x-1, x-2) |
| AC-16 | The ACK of block k is lost and the client resends DATA k | ACK k is sent again; DATA k is written once (RFC1350-2-3) |
| AC-17 | The final ACK is lost and the client resends the final DATA within one timeout | The final ACK is sent again (RFC1350-6-2) |
| AC-18 | netascii WRQ whose data holds CR LF, CR NUL, and a CR at the end of block 1 followed by LF at the start of block 2 | Stored with LF for each CR LF and CR for each CR NUL, across the block boundary too (RFC1350-5-1); an octet WRQ of the same bytes stores them unchanged; a `mail` WRQ gets ERROR 4 |
| AC-19 | `max-uploads` uploads running; or every `max-transfers` slot taken | A further WRQ gets ERROR 0 "service unavailable"; RRQs are still served while transfer slots remain |
| AC-20 | Second WRQ for a name with an upload in flight: from another client endpoint; from the same endpoint | Another endpoint: ERROR 0 naming the upload in progress. Same endpoint: dropped with no answer, and the running transfer completes |
| AC-21 | `.ze-upload-*` files left in the upload directory's top level by a crash | Removed when install opens the upload directory, before the first WRQ is served; each removal is logged |
| AC-22 | Config: uploads on with no `directory`; with an empty `allow-from`; `directory` equal to, inside, or containing `root-directory` (after symlinks); `directory` inside Ze's config directory; a `directory` that does not exist or cannot be written | Each is refused at verify or install with an error naming the leaf, why, and the fix (`ai/rules/user-facing-errors.md`); the running server is unchanged |
| AC-23 | Upload succeeds | One info log line with file, client, size and SHA-256; `ze_tftp_uploads_total{result="stored"}` and `ze_tftp_upload_octets_total` increase. A refused or failed upload increases `ze_tftp_uploads_total{result="refused"}` or `{result="failed"}` |
| AC-24 | curl 8.23.0 `-T` of a 300 kB file | Stored byte-identical; the WRQ carried tsize and Ze's OACK echoed it |
| AC-25 | tftp-hpa 7.2 `put` with its default options | Stored byte-identical; OACK windowsize 64; ACKs observed only at window ends and the final block |
| AC-26 | Name `configs/r1.cfg` with `configs` an existing subdirectory; with `configs` absent | Existing: stored there. Absent: ERROR 1, and no directory is created |
| AC-27 | Link or rename into place fails (EXDEV, EEXIST race, EACCES) | ERROR 6 for EEXIST, ERROR 0 otherwise, in place of the final ACK; the temp file is removed |
| AC-28 | A panic inside the receiver | The client gets ERROR 0 on the transfer socket (existing guard), the temp file is removed, and the next upload is served |
| AC-29 | ENOSPC or EDQUOT while writing | ERROR 3; the temp file is removed |
| AC-30 | Doctor with uploads on | `doctor-tftp-upload-directory` reports a missing, unwritable or overlapping directory, and free space under `reserved-space`, each with the fix |
| AC-31 | An upload of more than 65536 blocks (blksize 8) | Block numbers wrap past 65535 and the stored file is byte-identical |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator enables uploads for a management prefix and a switch copies its config to Ze (modelled by curl and tftp-hpa) | config → install → listener → `handleWRQ` → receiver → file in the upload directory | `TestTFTPCurlUploadInterop`, `TestTFTPHPAUploadInterop`, `test/plugin/tftp-upload.ci` |
| 2 | A host outside the allow-list tries to upload | listener → `handleWRQ` → ERROR 2 | `TestTFTPUploadRefusesSource` |
| 3 | An operator turns uploads off with a commit while one runs | ConfigTx apply → generation ends → upload cut and cleaned | `TestConfigTxReloadCutsUploadInFlight` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTFTPWriteRejected` | `internal/plugins/tftpserver/handler_test.go` | AC-1, unchanged | |
| `TestRFC1350UploadAcknowledgesWRQWithBlockZero` | `internal/plugins/tftpserver/rfc1350_upload_test.go` | AC-2, RFC1350-4-2 both polarities (ACK 0 on accept; an ERROR, never ACK 0, on refusal) | |
| `TestRFC1350UploadStoresIdenticalOctets` | same | AC-2, byte identity and mode | |
| `TestRFC1350UploadAcknowledgesEveryBlock` | same | AC-16, RFC1350-2-3 as receiver | |
| `TestRFC1350UploadShortBlockEnds` | same | RFC1350-5-3 as receiver: 511 octets ends, 512 does not | |
| `TestRFC1350UploadDalliesFinalACK` | same | AC-17, RFC1350-6-2 | |
| `TestRFC1350UploadTimeoutEnds` | same | AC-9, RFC1350-7-1 as receiver | |
| `TestRFC1350UploadNetasciiTranslated` | same | AC-18, RFC1350-5-1 both polarities | |
| `TestRFC2347UploadOACK` | `internal/plugins/tftpserver/rfc2347_upload_test.go` | The WRQ OACK carries only requested options; unacknowledged options are ignored; an ERROR 8 answer ends the transfer (AC-10) | |
| `TestRFC2348UploadBlksize` | `internal/plugins/tftpserver/rfc2348_upload_test.go` | AC-12 | |
| `TestRFC2349UploadTsizeEchoed` | `internal/plugins/tftpserver/rfc2349_upload_test.go` | AC-7, RFC2349-x-4 both polarities (echoed unchanged; never another value) | |
| `TestRFC2349UploadTooLargeRefused` | same | AC-7, AC-8, RFC2349-x-6 | |
| `TestRFC2349UploadTimeout` | same | AC-15 | |
| `TestRFC7440UploadAcksWindowEnd` | `internal/plugins/tftpserver/rfc7440_upload_test.go` | AC-13, RFC7440-4-2, 4-3 | |
| `TestRFC7440UploadSequenceError` | same | AC-14, RFC7440-4-6 | |
| `TestTFTPUploadBlockWrap` | `internal/plugins/tftpserver/upload_test.go` | AC-31 | |
| `TestTFTPUploadRefusesSource` | same | AC-3 | |
| `TestTFTPUploadConfinement` | same | AC-4, A-1 | |
| `TestTFTPUploadNoOverwrite`, `TestTFTPUploadNoOverwriteRace` | same | AC-5, AC-27, A-2 | |
| `TestTFTPUploadOverwriteAtomic` | same | AC-6 | |
| `TestTFTPUploadCleansUpOnFailure` | same | AC-9, AC-10, AC-29, R-11 (ledger back to zero) | |
| `TestTFTPUploadLimits` | same | AC-19, AC-20 | |
| `TestTFTPUploadSweepsTempFiles` | same | AC-21 | |
| `TestTFTPUploadSubdirectory` | same | AC-26 | |
| `TestTFTPUploadLogsAndCounts` | same | AC-23 | |
| `TestTFTPUploadPanicCleansUp` | `internal/plugins/tftpserver/panic_guard_test.go` | AC-28 | |
| `TestTFTPUploadConfig` | `internal/plugins/tftpserver/config_test.go` | AC-22 verify refusals; `TestTFTPConfigDefaultsMatchYANG` extended to the new defaults | |
| `TestTFTPUploadThroughInstall`, `TestConfigTxReloadCutsUploadInFlight` | `internal/plugins/tftpserver/config_tx_test.go` | wiring, AC-11 | |
| `TestTFTPUploadDoctor` | `internal/plugins/tftpserver/doctor_test.go` | AC-30 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `max-file-size` (D-3) | 1 to 4294967296 octets | 1 and 4294967296 | 0 | 4294967297 |
| `max-uploads` (D-4) | 1 to 1000 | 1 and 1000 | 0 | 1001 |
| `reserved-space` (D-5) | 0 to 18446744073709551615 octets | 0 | N/A | N/A (uint64) |
| WRQ tsize | 0 to `max-file-size` | `max-file-size` | N/A (a non-decimal value is not acknowledged) | `max-file-size` + 1 → ERROR 3 |
| WRQ blksize | 8 to 65464 | 8, 65464 | 7 (not acknowledged) | 65465 (not acknowledged) |
| WRQ timeout | 1 to 255 | 1, 255 | 0 | 256 |
| WRQ windowsize | 1 to 65535 | 1, 65535 | 0 | 65536 |
| Filename length | 1 to 255 octets | 255 | 0 (malformed) | 256 → ERROR 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tftp-upload` | `test/plugin/tftp-upload.ci` | A daemon with uploads on for 127.0.0.0/8 accepts `client/tftp-put` of a file, logs it with its SHA-256, and refuses a second put of the same name with ERROR 6 | |
| `tftp-upload-config` | `test/parse/tftp-upload-config.ci` | The upload leaves parse, and an enabled upload with no `allow-from` is refused with the fix text | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestTFTPCurlUploadInterop` | `internal/plugins/tftpserver/tftp_curl_docker_test.go` | curl 8.23.0 (`-T`) | AC-24: tsize echoed, lockstep receive, stored byte-identical; break: drop tsize from the WRQ OACK | |
| `TestTFTPHPAUploadInterop` | `internal/plugins/tftpserver/tftp_hpa_docker_test.go` | tftp-hpa 7.2 client (`put`) | AC-25: windowed receive with ACKs at window ends; break: ACK every block | |

## Files to Modify
- `internal/plugins/tftpserver/handler.go` - WRQ dispatch to `handleWRQ`; the request parser named for both opcodes; `runTransfer` runs the receiver for an upload
- `internal/plugins/tftpserver/config.go` - `uploadConfig` inside `tftpConfig`, its parse with `rangedLeaf` and `configvalue.LeafList`, verify refusals
- `internal/plugins/tftpserver/config_tx.go` - install opens the upload root, sweeps temp files, builds the per-generation upload state; `bindTFTP` passes it to the listeners
- `internal/plugins/tftpserver/register.go` - `Description`, doctor check registration, metrics registration
- `internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang` - new revision, the `upload` container, the container description and `ze:help`
- `internal/plugins/tftpserver/handler_test.go` - no existing test changes; `buildWRQPacket` gains options for the new tests
- `internal/plugins/tftpserver/tftp_curl_docker_test.go`, `tftp_hpa_docker_test.go` - the two upload interop tests; the relay records the client's packets as well
- `internal/plugins/tftpserver/config_test.go`, `config_tx_test.go`, `panic_guard_test.go` - the tests named above
- `internal/core/diagnostic/codes.go` - `doctor-tftp-upload-directory`
- `internal/le/arch/fspersistence/fspersistence.go` - `fileAllowlist` entry for `internal/plugins/tftpserver/upload_store.go` with its reason
- `internal/test/fixture/service_client_fixture.go`, `register_service_client.go` - `client/tftp-put`
- `docs/architecture/provisioning/tftp-server.md` - replace "Read-only", add the upload decisions, traps and source anchors
- `docs/guide/configuration.md` - TFTP Server section: the `upload` block and its table
- `docs/guide/plugins.md` - tftpserver row
- `docs/guide/monitoring.md` - the upload counters
- `docs/functional-tests.md` - the `client/tftp-put` fixture
- `docs/architecture/testing/interop.md` - the two upload interop tests
- `features/installation.md` - the tftpserver sentence names uploads
- `rfc/short/rfc1350.md`, `rfc2347.md`, `rfc2348.md`, `rfc2349.md`, `rfc7440.md` - through `/ze-rfc`: gap annotations removed, server-receiver claims and tags added, Enrolment reasons updated; `rfc/extraction/rfc1350.json` site 5:2 re-walked
- `docs/features/rfc-status.md` - regenerated from the summaries

## Files to Create
- `internal/plugins/tftpserver/upload.go` - `handleWRQ`, the admission checks, the in-flight name set, the space ledger
- `internal/plugins/tftpserver/window_recv.go` - the receiver: OACK or ACK 0, window ACKs, sequence-error ACK, dally, size caps
- `internal/plugins/tftpserver/upload_store.go` - temp file, fsync, link or rename, cleanup, sweep, netascii translator
- `internal/plugins/tftpserver/doctor.go` - the upload-directory check
- `internal/plugins/tftpserver/rfc1350_upload_test.go`, `rfc2347_upload_test.go`, `rfc2348_upload_test.go`, `rfc2349_upload_test.go`, `rfc7440_upload_test.go`, `upload_test.go`, `doctor_test.go`
- `test/plugin/tftp-upload.ci`, `test/parse/tftp-upload-config.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/tftpserver/yang/ze-tftp-server-conf.yang`: container `upload` with `enabled`, `directory`, `allow-from`, `overwrite`, `max-file-size`, `max-uploads`, `reserved-space` |
| YANG validation constraints | Yes | `range` on the numeric leaves, `zt:ip-prefix` on `allow-from`, `units` octets |
| YANG custom validators | No | the overlap and empty-list refusals live in `verifyConfig`, as `root-directory`'s does |
| CLI commands/flags | N-A | no command is added |
| CLI grammar (keyword before value) | N-A | no command is added |
| Editor autocomplete | Yes | automatic from the YANG types |
| Functional test for new RPC/API | N-A | no RPC; the feature's functional test is `test/plugin/tftp-upload.ci` |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no env var (see Required Reading, config-option) |
| Doctor check for runtime dependencies | Yes | `internal/plugins/tftpserver/doctor.go`, code `doctor-tftp-upload-directory` in `internal/core/diagnostic/codes.go`, `TestTFTPUploadDoctor` |
| Prometheus counters/metrics | Yes | `ze_tftp_uploads_total{result="stored"\|"refused"\|"failed"}`, `ze_tftp_upload_octets_total`, registered through `ConfigureMetrics` |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/installation.md` (the tftpserver sentence) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` TFTP Server section |
| 3 | CLI command added/changed? | No | no command |
| 4 | API/RPC added/changed? | No | no RPC |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` tftpserver row |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` is the guide for it |
| 7 | Wire format changed? | No | TFTP's wire format is the RFC's; the architecture page covers the receiver |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1350.md`, `rfc2347.md`, `rfc2348.md`, `rfc2349.md`, `rfc7440.md`, generated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` names the `client/tftp-put` fixture beside `client/tftp-get` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` names no TFTP server |
| 12 | Internal architecture changed? | Yes | `docs/architecture/provisioning/tftp-server.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the registration `Description` changes; `docs/plugin-overview.md` and `docs/features/plugins.md` rows checked at implementation |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived with `./le spec citation anchors spec plan/post-release/features/spec-tftp-server-uploads.md` at implementation; `docs/architecture/provisioning/tftp-server.md` is declared by `handler.go`'s `// Design:` header |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `service { tftp-server { ... } }` example in `docs/guide/configuration.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the `upload` YANG container, its parse and verify, the WRQ dispatch to a `handleWRQ` that admits and stores through a stub receiver
   - Tests: `TestTFTPUploadThroughInstall` (fails at the stub), `TestTFTPWriteRejected` (stays green), `TestTFTPUploadConfig`
   - Files: `yang/ze-tftp-server-conf.yang`, `config.go`, `config_tx.go`, `handler.go`, `upload.go`
   - Verify: a WRQ reaches `handleWRQ` from an installed config
2. **Phase: Store** -- `os.Root`, temp file, fsync, link or rename, cleanup, sweep, confinement, the fs-persistence allowlist entry
   - Tests: `TestTFTPUploadConfinement`, `TestTFTPUploadNoOverwrite`, `TestTFTPUploadNoOverwriteRace`, `TestTFTPUploadOverwriteAtomic`, `TestTFTPUploadSweepsTempFiles`, `TestTFTPUploadSubdirectory`
   - Files: `upload_store.go`, `internal/le/arch/fspersistence/fspersistence.go`
3. **Phase: Lockstep receiver** -- ACK 0, every-block ACK, short-block end, duplicate DATA, timeout, dally, ERROR handling, size caps, netascii
   - Tests: the `TestRFC1350Upload*` set, `TestTFTPUploadCleansUpOnFailure`, `TestTFTPUploadBlockWrap`, `TestTFTPUploadPanicCleansUp`
   - Files: `window_recv.go`, `upload_store.go`
4. **Phase: Options** -- the WRQ OACK with blksize, timeout, tsize echo and windowsize; window ACKs and sequence-error ACK; tsize and ledger admission
   - Tests: `TestRFC2347UploadOACK`, `TestRFC2348UploadBlksize`, the `TestRFC2349Upload*` set, the `TestRFC7440Upload*` set
   - Files: `upload.go`, `window_recv.go`
5. **Phase: Access, limits, observability** -- `allow-from`, `max-uploads`, in-flight names, the log line, counters, doctor check
   - Tests: `TestTFTPUploadRefusesSource`, `TestTFTPUploadLimits`, `TestTFTPUploadLogsAndCounts`, `TestTFTPUploadDoctor`, `TestConfigTxReloadCutsUploadInFlight`
   - Files: `upload.go`, `register.go`, `doctor.go`, `internal/core/diagnostic/codes.go`
6. **Phase: Functional and interop** -- the `client/tftp-put` fixture, the two `.ci` tests, curl and tftp-hpa interop with recorded breaks
   - Tests: `tftp-upload`, `tftp-upload-config`, `TestTFTPCurlUploadInterop`, `TestTFTPHPAUploadInterop`
7. **Phase: RFC records** -- `/ze-rfc` for the five summaries and site 5:2, `./le rfc record` for every new tag. Each page edit lands in the phase whose code changes the page (`ai/rules/documentation.md`), not here

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each user story has a working path |
| Correctness | The final ACK is never sent before the target exists; every exit path removes the temp file and releases the ledger, the name and both semaphore slots |
| Correctness | No path reaches the filesystem except through the upload `os.Root` |
| Naming | YANG leaves spelled in full; metric names follow `docs/plugin-development/metrics.md` |
| Data flow | The RRQ path is byte-identical; the upload shares the parser, generation and guard rather than copying them |
| Rule: `ai/rules/rfc-compliance.md` | Each enforced MUST carries its quoted RFC comment; each new implementing function quotes the RFC text it implements |
| Rule: `ai/rules/user-facing-errors.md` | Every verify and install refusal names the leaf, why, and the exact fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| WRQ accepted when configured | `TestTFTPUploadThroughInstall`, `test/plugin/tftp-upload.ci` |
| WRQ refused by default | `TestTFTPWriteRejected` |
| RFC rows closed | `./le rfc check` over the five stems; RFC1350-4-2, RFC1350-5-1 and RFC2349-x-4 carry no `{gap}` |
| Interop | the two Docker tests green, each with its recorded red |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path confinement | Every open, link, rename and remove goes through the upload `os.Root`; a name is never joined to a path string |
| Fail closed | Uploads off when the container is absent; an empty `allow-from` refused at verify, never read as "everyone" |
| Resource exhaustion | `max-file-size`, the tsize cap, `max-uploads`, `max-transfers`, the space ledger and ENOSPC each end an upload with cleanup |
| Overlap with the read root | Refused at verify and install after symlink resolution, so an upload cannot become a served boot file (D-1) |
| Error leakage | ERROR messages name the refusal class, never a host path |
| Stored file mode | 0600 (D-8) |

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

- RFC 1350 section 5 allows a storage server to keep netascii untranslated ("There is no reason that such a machine needs to translate netascii into its own form of text."), yet the same section's "A host which receives netascii mode data must translate the data to its own format." is the MUST row RFC1350-5-1. The row stands; D-10 (a) implements the translation
- tftp-hpa answers a too-large upload with ERROR 2; RFC 2349 names code 3. Ze follows the RFC

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Temp file plus link or rename | Write in place and truncate (tftp-hpa) | A failed upload never damages an existing file, a reader never sees half a file, and no-overwrite is atomic through link(2) |
| `os.Root` for every file operation | `resolvePath` (EvalSymlinks then prefix check) | `resolvePath` is racy between check and open and needs the file to exist; `os.Root` confines each system call |
| Final ACK after the file is in place | ACK first, commit after | A client that saw success can rely on the file; tftp-hpa orders it the same way |
| Admission ledger of reserved sizes | Check free space once per WRQ | Concurrent uploads each pass a one-shot check and together fill the disk |
| In-flight name set | File lock (tftp-hpa) | One process owns the directory; the set also recognises a duplicate WRQ from the same endpoint |
| Upload leaves outside the limits-only reload set | Treat them as limits | Turning uploads off must cut the uploads in flight |

## Known Limitations
- RRQ stays octet-only unless D-10 is answered (b)
- The server sends no ERROR 5 to a stranger TID (RFC1350-4-4, SHOULD), because the per-transfer socket is connected and the kernel drops such packets; this is today's read-side behaviour too, unchanged here
- The server's sockets are IPv4 (`udp4`), as they are today

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-tftp-server-uploads.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

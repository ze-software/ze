# tftpserver author handoff (child: spec-rfc-verdict-fix-services, 2026-09-28)

Listing reproduced: 12 weak/wrong verdicts tag `internal/plugins/tftpserver` (rfc1350 x6, rfc2347 x2, rfc2348 x3, rfc7440 x1), none under "Blocked by". Plus the two narrowing/split rows RFC1350-2-3 and RFC2347-x-2.

New tests live in two NEW files (no existing tagged unit was edited, so no verdict file-sha shifted and no reseal is needed). Old weak units keep their tags; the judge reads the union.

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC1350-2-2 | tests | `TestRFC1350LockstepWaitsForTheACK`: + block 1 is the first 512 octets, after ACK 1 block 2 is octets 512..1023; - no datagram for 1 s while block 1 is unacked | +,- revert sendAndWaitACK | enforced | |
| RFC1350-2-3 | defect (D-8) + tests + row | `TestRFC1350DuplicateACKTriggersNoDATA`: + fresh ACK 2 answered by block 3 exactly once; - duplicate ACK 1 while block 2 outstanding gets no DATA for 1 s | +,- revert sendAndWaitACK | enforced (new; verdict was null) | red-first shown: the test run with HEAD's handler.go via overlay FAILS "DATA sent in answer to a duplicate ACK: got opcode 3 block 2". Fix in `sendAndWaitACK`: a non-matching packet no longer triggers a retransmit; reading continues to the same deadline; `isACKFor` helper; RFC 1350 §5 quote above the statement. `{gap}` removed from the row. DISAGREES with the narrowing "moved" call: RFC 1350 §5 itself states the rule ("All packets other than duplicate ACK's ... are acknowledged ... Sending a DATA packet is an acknowledgment for the first ACK packet of the previous DATA packet"), and the §front text says the 1992 revision exists to fix the Sorcerer's Apprentice bug. So the row stays on rfc1350 and was not retired to un-summarized RFC 1123. The main thread decides whether that holds |
| RFC1350-4-3 | tests | `TestRFC1350ServerChoosesOneTIDForTheTransfer`: + every DATA of 3 blocks comes from one source port, and that port is not the listener port | + revert handleRRQ | enforced | `{single-polarity: positive}` kept (the server always dials a fresh socket; there is no refusal path) |
| RFC1350-5-2 | blocked (owner) | none changed | none | stays wrong | the row binds a host that RECEIVES an octet file and returns it. Ze has no TFTP client and refuses WRQ (grep: no other TFTP code in the tree). The correct row state is binds-another-role/not-applicable, but that drops the HEAD positive tags (TestTFTPReadRequest, TestTFTPReadLargeFile), and `checkCoverageRatchet` refuses it. Per D-7, a move the gates refuse goes to the owner. Disclosed in `Support remaining` (rfc-check required it once the 2-3 text was removed) |
| RFC1350-5-3 | tests | `TestRFC1350DataFieldLengthEndsTheTransfer` (1500/1024/0-octet files, 64 KiB read buffer): + each field is at most 512 octets; a 512-octet block is followed and a non-final block is never short; a 0..511 block ends it (nothing arrives in 1 s after its ACK); the bytes equal the file | + revert serveFile | enforced | single-polarity kept (Ze is the sender) |
| RFC1350-6-1 | tests + row | `TestRFC1350LastDATARetransmittedUntilAcknowledged`: + the unacked last DATA (block 2) is retransmitted byte for byte; - after ACK 2, nothing arrives for ackTimeout+2 s | +,- revert sendAndWaitACK | enforced | `{single-polarity}` removed (a negative exists) |
| RFC1350-7-1 | tests + row | + `TestRFC1350TimeoutEndsTheTransferOfAGonePeer`: with no ACK, block 1 is sent exactly maxRetransmit+1 times, then stops, and a one-slot server serves a new RRQ (the slot is released); - `TestRFC1350TimeoutThenACKContinuesTheTransfer`: an ACK after one retransmission continues the transfer to block 2 | +,- revert sendAndWaitACK | enforced | `{single-polarity}` removed (its reason was test cost). The test takes about 27 s |
| RFC2347-x-1 | tests | `TestRFC2347OnlyTheClientInitiatesNegotiation`: + tsize-only gets OACK tsize=1500; - that OACK has no blksize; a plain RRQ and a windowsize-only RRQ get DATA 1 with no OACK | + revert sendOACKAndWait, - revert handleRRQ | enforced | |
| RFC2347-x-2 | no change | the dropped clause "must not use those options which were not acknowledged" is carried verbatim by RFC2347-x-4 (the whole sentence); x-2 is its prefix | none | n/a (not-applicable, client) | the split-needed finding's "x-3 does not carry it" is true, but x-4 does. x-2 is a redundant prefix of x-4, and it can be retired under D-2 if the main thread wants that. I did not retire it |
| RFC2347-x-3 | tests | `TestRFC2347UnacknowledgedOptionIsNotUsed`: + windowsize (unacked) is not used: after OACK/ACK 0 no block 2 arrives before ACK 1; declined blksize 7 and 65465 are absent from the OACK and block 1 is 512; - acked blksize 1024 is used (block 1 = 1024) | +,- revert handleRRQ | enforced | client half is n/a (Ze has no client) |
| RFC2348-x-3 | tests | `TestRFC2348BlksizeBoundsAreInclusive`: + 8 acked as 8 and block 1 = 8 octets; 65464 acked (as 1468, the cap); - 7 and 65465 absent from the OACK | +,- revert parseRRQ | enforced | |
| RFC2348-x-4 | tests | `TestRFC2348ShortBlockIsTheFinalPacket` at a negotiated 1024: + 600-octet file = [600] and silence (a server comparing to 512 sends an empty block 2); 2500 = [1024,1024,452] then silence; - a full 1024 block is followed | +,- revert serveFile | enforced | |
| RFC2348-x-5 | tests | `TestRFC2348ZeroLengthBlockEndsAnExactMultiple` at 1024: + 2048 = [1024,1024,0]; - 1500 = [1024,476], 512 = [512], nothing after either | +,- revert serveFile | enforced | |
| RFC7440-3-1 | tests | `TestRFC7440FieldsAreNULTerminatedASCIIStrings`: + the §3 example request parses to foobar/octet/windowsize; every OACK field is non-empty printable ASCII + exactly one NUL (4 fields); - an unterminated #blocks leaves windowsize unrecognized; a filename without a NUL is refused | +,- revert parseRRQ | enforced or weak | judge's call: the ASCII clause binds the SENDER of a request. Ze receives requests and does not refuse a non-ASCII octet in a received filename. The only fields Ze sends are the OACK fields, and the test checks those |

## Gates run here
- `./le job run label tftp-test command go test -race -count=1 ./internal/plugins/tftpserver/`: ok (26 s), after all Go edits. gofmt clean.
- The overlay run with HEAD's handler.go: the new 2-3 test is red (red-first proof for D-8).
- 22 `discriminate-record` runs (revert route): each OBSERVED red. `./le rfc discriminate stem` over rfc1350/2347/2348/2349/7440: 0 stale.
- `./le rfc check`: my stems show only the expected STALE verdicts (11 ids, awaiting re-judge) and one disclosure finding for RFC1350-5-2. I fixed that finding afterwards by editing the Support remaining text, and did not re-run the check.

## Owed (main thread)
- A re-judge (`audit-stamp ... mode rejudge`) of the 11 stale ids, plus a first verdict for RFC1350-2-3. The judge must not be the author.
- `./le go lint run` and `./le verify worktree` (not run here). The post-write hook linted the package clean after the last edit.
- Interop for the D-8 fix: no TFTP interop scenario exists (`test/interop/scenarios`). A TFTP client peer scenario is owed by the child spec's interop row. It was not created here: it needs a container client, it is a scenario design choice, and a normal fetch does not show the duplicate-ACK behavior.
- Owner: RFC1350-5-2 (see the row above).
- Approval records in `tmp/commit-rfc-approved-01a40e57.md` name the 12 NEW units (the edit hook asked for them while the files were uncommitted). No HEAD unit was changed.

## Files changed
- internal/plugins/tftpserver/handler.go
- internal/plugins/tftpserver/rfc1350_transfer_test.go (new)
- internal/plugins/tftpserver/rfc2347_2348_transfer_test.go (new)
- rfc/short/rfc1350.md (Enrolment reason, Support remaining, row annotations on 6-1, 7-1, 2-3)
- rfc/discrimination/rfc1350.json (new), rfc/discrimination/rfc2347.json (new), rfc/discrimination/rfc2348.json (new), rfc/discrimination/rfc7440.json (new)
- docs/architecture/provisioning/tftp-server.md

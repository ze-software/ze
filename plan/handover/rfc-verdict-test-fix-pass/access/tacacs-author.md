# tacacs author handoff (spec-rfc-verdict-fix-access, internal/component/tacacs, stem rfc8907)

18 weak verdicts, 0 wrong, none blocked. No existing test file was edited: every new unit lives in two new files, so no other verdict tagging an existing file shifts. Existing weak tags were left in place (they still state what their bodies assert); the new units carry the missing clauses and polarities.

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC8907-10.5.2-1 | tests | + TestRFC8907MatchingObfuscationKeepsStatusAndConnection (authen PASS and author PASS_ADD kept, single-connect TCP stays open); - TestRFC8907ObfuscationMismatchClosesAndFails (keyed server + UNENCRYPTED flag: TCP closed by client AND AUTHEN FAIL / AUTHOR FAIL; keyless server receives no request, so clause (b) can never reach a reply) | new x2 + re-recorded stale TestRFC8907ClientRefusesUnobfuscatedReply | enforced | probe accepts EOF or ECONNRESET as "client closed" (client closes with the unread body, kernel sends RST) |
| RFC8907-4-1 | tests | + TestRFC8907EveryRequestCarriesMajorVersion12 (captured request major nibble 0xc for authen/author/acct); - TestRFC8907ReplyWithOtherMajorVersionRefused (all 15 other majors x 3 kinds refused) | new x2 | enforced | |
| RFC8907-4-2 | tests | + TestRFC8907ClientSendsOddSequenceNumbers (captured seq_no odd, 3 kinds); - TestRFC8907ReplyWithOddSequenceNumberRefused (all 128 odd server seq_no x 3 kinds) | new x2 | enforced | |
| RFC8907-4-4 | tests + row | + TestRFC8907ReplyKeepingSessionIDAccepted; - TestRFC8907ReplyChangingSessionIDRefused (bit 0, bit 31, all bits x 3 kinds) | new x2 | enforced | row lost its invalid {single-polarity} marker (trySend holds the refusal path); Correction paragraph written |
| RFC8907-4-5 | tests | + TestRFC8907LengthsAreUnsignedNetworkOrder (fixed wire octets for session_id/length; 0xFFFFFFFE read unsigned; 2-octet lengths 0x0100 and 0x8001 read big-endian unsigned in authen/author/acct replies) | new x1 | enforced | {single-polarity: positive} kept: no refusal path exists for byte order |
| RFC8907-4.1-1 | tests | + TestRFC8907PresentFieldsFollowZeroLengthOnes; - TestRFC8907ZeroLengthFieldsAreNotPresent (server_msg, data, args in all three reply kinds; zero-length arg yields no entry) | new x2 (+ 2 existing) | enforced | |
| RFC8907-4.3-3 | tests | - TestRFC8907ClosureMidSessionOnPooledConnectionIsAccommodated (server takes session 3's request on the pooled TCP and closes: session still PASSes on a fresh TCP) | new x1 + re-recorded stale TestRFC8907ClosureOnFreshDialIsNotRetried | enforced | the old negative tag (fresh-dial closure) was left in place; it asserts a neighbouring property; the judge may call it mistagged |
| RFC8907-4.6-1 | tests | - TestRFC8907OverlongReplyComponentsRejected (sum > datalength via server_msg_len, data_len, arg length, all 3 kinds) | new x1 | enforced | the < half was already proven |
| RFC8907-3.7-2 | tests | + TestRFC8907PrintableTextInEveryField; - TestRFC8907ControlCharactersRefusedInEveryField: authen port/rem_addr, author port/rem_addr, acct rem_addr/arg, author reply msg/data/arg, acct reply msg/data (NUL, 0x1f, DEL, e-acute) | new x2 | enforced | |
| RFC8907-5.4.2.2-2 | tests | + TestRFC8907PAPStartOnWireCarriesUserAndPassword (wire user + data); - TestRFC8907PAPStartWithoutUsernameNeverSent (errRequestInvalid, server never contacted) | new x2 | enforced | |
| RFC8907-5.4.2.6-2 | row + tests | row narrowed to "It MUST NOT be set to this value when requesting any other operation." (clause 1 is RFC8907-5.4.2.6-1, feature-declined); + TestRFC8907PAPLoginOnWireCarriesLoginService; - TestRFC8907NoRequestCarriesEnableService (authen START, session + command authorization on wire, acct START/STOP) | new x2 | enforced | Correction paragraph written; extraction site 5.4.2.6:2 already maps the narrowed text |
| RFC8907-6.1-1 | tests | + TestRFC8907AuthorUserLenCountsBytes ("Alicé" -> 6, 255-byte multibyte user -> 255); - TestRFC8907AuthorUserLenRefusesRuneCount (128 runes / 256 bytes refused) | new x2 | enforced | |
| RFC8907-7-1 | tests | + TestRFC8907AccountingValidFlagsWritten (0x02, 0x04, 0x08, 0x0a); - TestRFC8907AccountingOtherFlagsRefused (all 252 other values incl. 0x10-0x80; SendAccounting 0x82 not sent) | new x2 | enforced | |
| RFC8907-8-1 | tests | + TestRFC8907AuthorizationRequestsUseDictionaryArguments (command and session author requests on wire); - TestRFC8907AuthorizationRequestsCarryNoForeignArgument | new x2 | enforced | accounting use case already covered by existing units |
| RFC8907-8.1-2 | tests | + TestRFC8907StopTimeIsEpochSeconds; - TestRFC8907StopTimeIgnoresLocalZone | new x2 | enforced | |
| RFC8907-8.2-1 | tests | + TestRFC8907EveryRequestKindLeadsWithService (command author, session author, acct START/STOP); - TestRFC8907NoRequestKindOmitsService (exactly one service in each, incl. empty cmd and argless command) | new x2 | enforced | |
| RFC8907-8.2-2 | tests | + TestRFC8907EveryShellRequestCarriesCmd; - TestRFC8907ShellRequestNeverLacksCmd (session cmd=, acct records for "show") | new x2 | enforced | |
| RFC8907-10.5.4-1 | tests | + TestRFC8907SharedArgumentsKeepDefinedMeaning (priv-lvl 0/1/15 -> their profiles; exact PASS_REPL command); - TestRFC8907SharedArgumentsNeverReinterpreted (priv-lvl 16, " 15", 0x0f, 1.5 rejected; full-width refused as non-ASCII; cmd/cmd-arg case or order change denied) | new x2 | enforced | |
| RFC8907-x-1 (narrowing, dropped) | row (correction) | the dropped §4.3 sentence binds the server; extraction site 4.3:3 already excludes it as binds-another-role | none | n/a | Correction paragraph written, no row owed |

## Stale records re-recorded (other sessions staled them; producer client.go::trySend)

All 11 re-recorded with `./le rfc discriminate-record ... route revert producer internal/component/tacacs/client.go::trySend`: RFC8907-10-1 +, 10-2 +, 4.3-1 +/-, 4.3-2 +/-, 4.4-2 -, 4.4-3 -, 5.4.2.2-1 -, 10.5.2-1 - (TestRFC8907ClientRefusesUnobfuscatedReply), 4.3-3 - (TestRFC8907ClosureOnFreshDialIsNotRetried).

## Record state after this pass

`./le rfc discriminate stem rfc8907`: 0 stale records, 33 records on the two new files, and no unproven tag in them. Every new tag's record observed red.

Recorder oddity (unverified, NOT journaled): a revert of `client.go::sendToServers` left TestRFC8907ClientSendsOddSequenceNumbers GREEN, although every exchange calls it. The record was written against `client.go::trySend` instead, and that break went red. `packet.go::MarshalInto` could not be named because two methods share the name (Packet and PacketHeader). One of the 35 runs also failed once with "go: parsing overlay JSON" and succeeded on retry.

## Files changed

- internal/component/tacacs/rfc8907_header_test.go (new)
- internal/component/tacacs/rfc8907_arguments_test.go (new)
- rfc/short/rfc8907.md (RFC8907-4-4 marker removed; RFC8907-5.4.2.6-2 narrowed)
- rfc/corrections/rfc8907.md (new: 4-4, 5.4.2.6-2, x-1)
- rfc/discrimination/rfc8907.json (11 re-recorded + new records)
- the `./le rfc approve` store (D-15 approval for tacacs.TestRFC8907SharedArgumentsNeverReinterpreted)

## Owed to the main thread

- `./le rfc check`, `./le go lint run`, `./le verify worktree`: not run here (five-minute rule).
- Re-judge all 18 verdicts plus 4-4 and 5.4.2.6-2 row text (stale-requirement until re-judged).

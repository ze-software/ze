# bfd/auth author handoff (spec-rfc-verdict-fix-bfd, phase 4)

Author agent, 2026-09-28. 17 verdicts from the derived listing filtered to `internal/component/bfd/auth/`. No verdict stamped, nothing committed.
No existing tagged unit was edited: every new cover is a NEW function in a NEW file. That leaves the file-sha of `rfc5880_test.go` unchanged, so no sibling verdict goes stale and no `./le rfc approve unit` or reseal is needed.
Every new tag carries a discrimination record written by `./le rfc discriminate-record ... route revert`, each with an OBSERVED red naming the unit.
Manual overlay mutants, run once each (not recorded):
- window hard-coded to 9: the four new window units go red; the old window units stay green.
- `buf[off+3] = 0` removed: only the new Reserved-byte unit goes red.
- Sign/Verify digest over `[0:off]` only: the new span units go red; the old DigestCoversWholePacket and RejectsMandatorySectionTamper stay green.

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC5880-4.3-1 | tests | + TestRFC5880ReservedByteZeroedOnTransmit: all four keyed types, buffer pre-filled with 0xFF (covers the §4.4 half) | + | enforced (row already `{single-polarity: positive}`) | the old unit used a zeroed buffer and could not fail |
| RFC5880-6.7-1 | tests | + old TestRFC5880BothSHA1VariantsSupported; - TestRFC5880SHA1TypesEnforcedDistinctly: flipped hash refused for types 4 and 5, each refuses the other's section, a repeated sequence is accepted by 4 and refused by 5 | - | enforced if the judge accepts "type aliased or unenforced" as the violation | the old negative (types 0/6/200 refused) stays tagged |
| RFC5880-6.7.2-3 | tests | + old TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID; - TestRFC5880SimplePasswordSignerNeedsPassword: NewSigner refuses nil and empty passwords | - | judgement call: the negative is a transmit-side refusal, the only refusal path the transmit rule has | old SHA1 Key ID tags stay; they belong to the split row (phase 8) |
| RFC5880-6.7.2-4 | tests | + TestRFC5880SimplePasswordSectionOfTypeOneAccepted; - TestRFC5880SimplePasswordMissingSectionOrOtherTypeDiscarded: no section (A=0, Length 24) gives ErrShortAuthBody; types 0,2,3,4,5,6,255 give ErrPasswordMismatch | +, - | enforced for the quoted Simple Password sentence | old SHA1 units stay tagged: the row's cite still names §6.7.3/§6.7.4 (split-needed, phase 8). The stack-level A=0 discard is session.Receive (RFC5880-6.8.6-10) |
| RFC5880-6.7.2-6 | tests | + TestRFC5880SimplePasswordAuthLenPlusThreeAccepted (1, 8, 16 bytes); - TestRFC5880SimplePasswordAuthLenNotPlusThreeDiscarded (0, n+2, n+4, 255, with every other field valid) | +, - | enforced for the quoted sentence | same split note as 6.7.2-4 |
| RFC5880-6.7.2-8 | tests | + old TestRFC5880SimplePasswordSectionHeader; - TestRFC5880SimplePasswordSectionOutsideProperLengthNotBuilt: 0-byte and 17-byte passwords (Auth Len 3 and 20) refused | - | enforced | the Auth Type clause has no refusal path (simpleSigner hard-codes 1) |
| RFC5880-6.7.3-1 | tests | + old TestRFC5880KeyedMD5SectionHeader; - TestRFC5880KeyedMD5OversizedKeyNotSigned: keys of 17/20/32 bytes refused for types 2 and 3 | - | judgement call: this is the only refusal on the transmit path; the Auth Type clause has none | the negative overlaps the RFC5880-6.7.3-13 negative |
| RFC5880-6.7.3-2 | tests | + TestRFC5880DigestSpansEntirePacket: Sign's digest equals crypto/md5 over bytes 0..Length with the key padded in; a hand-built packet is accepted. - TestRFC5880DigestOverPartialPacketDiscarded: digests over 24, off+4, off+8 and Length-1 bytes refused; each byte of the mandatory section, the Reserved byte and the Sequence Number flipped gives ErrDigestMismatch | +, - | enforced | types 2 and 3 |
| RFC5880-6.7.3-4 | tests (positive) / unresolved (negative) | + TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits (session package): 0xFFFFFFFE, 0xFFFFFFFF, 0, 1 on the wire through Machine.Sign and AdvanceAuthSeq, and the peer accepts each | + | weak unless the judge accepts the HEAD receive-side negative | a transmit MUST has no refusal path, and the brief bars `{single-polarity}` while a HEAD negative tag exists: the main thread must decide |
| RFC5880-6.7.3-9 | tests | + TestRFC5880KeyedWindowFollowsReceivedDetectMult: MD5 and SHA1, Detect Mult 1/5/255, equal and +3*DM accepted, wrap included; - TestRFC5880KeyedOutsideWindowDiscarded: -1, +3*DM+1, +2^31 and behind-across-wrap refused, floor unmoved | +, - | enforced | |
| RFC5880-6.7.3-10 | tests | + TestRFC5880MeticulousWindowFollowsReceivedDetectMult (+1, +3*DM, both wraps); - TestRFC5880MeticulousOutsideWindowDiscarded (equal, -1, +3*DM+1, behind-across-wrap) | +, - | enforced | types 3 and 5 |
| RFC5880-6.7.3-11 | tests | + TestRFC5880KeyedMD5IndependentDigestAccepted (packet hashed by crypto/md5, not md5Sum); - TestRFC5880KeyedMD5DigestMismatchDiscarded (flipped digest, digest under another key, floor stays 40) | +, - | enforced | types 2 and 3. The split row for SHA1 (§6.7.4) is phase 8 |
| RFC5880-6.7.3-12 | tests | + TestRFC5880KeyedMD5FirstPacketSeedsReplayFloor: authentic first packet, and a forged first packet (D-14 order kept); - TestRFC5880KeyedMD5KnownFloorNotReseeded: a forged in-window packet and a +2^31 packet leave the floor at 100 | +, - | enforced | D-14 was not reversed, and RFC5880-6.8.1-13 was not touched |
| RFC5880-6.7.4-1 | tests | + old TestRFC5880KeyedSHA1SectionHeader; - TestRFC5880KeyedSHA1OversizedKeyNotSigned (21 and 32 bytes) | - | judgement call, same as 6.7.3-1 | |
| RFC5880-6.7.4-2 | tests | same two units as 6.7.3-2, types 4 and 5 (crypto/sha1) | +, - | enforced | |
| RFC5880-6.7.4-4 | tests (positive) / unresolved (negative) | + the same session unit, type 5 | + | same as 6.7.3-4 | |
| RFC5880-6.8.1-12 | tests | + TestRFC5880NewSessionAuthSeqKnownZero (session package): a new Machine with SetAuth has AuthSeqKnown 0 and RcvAuthSeq 0 and accepts a first packet at 0x80000000; - TestRFC5880AuthSeqKnownOneDiscardsFirstPacket: with AuthSeqKnown forced to 1 the same packet is refused | +, - | judgement call: the negative builds the violating state itself, because an initialization has no refusal path | answers the judge's "not the state a session Init creates" |

Counts: tests 13 fully resolved. Judgement calls: 5 (6.7-1, 6.7.2-3, 6.7.3-1, 6.7.4-1, 6.8.1-12). Negative unresolved: 2 (6.7.3-4, 6.7.4-4). Row corrections: 0. Defects: 0. Blocked: 0.
Split-needed rows (6.7.2-2/3/4/5/6, 6.7.3-5/8/9/10/11) are phase 8 and were not touched. No journal row was written.

## Main-thread decisions

1. RFC5880-6.7.3-4 and 6.7.4-4 (transmit MUST). The row has no refusal path, and a negative tag existed at HEAD. Choose one: accept the HEAD receive-side negative, or allow `{single-polarity: positive}` beside the held negative. The coverage ratchet counts polarity loss only, so keeping the old tag loses nothing.
2. The session-package units were placed in a new file, `session/rfc5880_auth_seq_test.go`. The producers (`AdvanceAuthSeq` and the Machine's `rcvAuthSeq`) live in session, and engine was off-limits. The session-phase author should know this file exists.

## Owed gates (not run here)
- `./le rfc check`
- `./le go lint run` over bfd/auth and bfd/session
- the judge re-stamp through `./le rfc audit-stamp stem rfc5880 ... mode rejudge`

## Files changed
- internal/component/bfd/auth/rfc5880_keyed_test.go (new)
- internal/component/bfd/session/rfc5880_auth_seq_test.go (new)
- rfc/discrimination/rfc5880.json (26 new records)

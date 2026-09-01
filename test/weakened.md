# Test weakenings this commit accepts

**This file is REPLACED for each commit. It never accumulates.** Delete the rows
of the last commit, write the rows of this one. The commit gate refuses a row
naming a test the prospective commit does not weaken, so a row left behind by an
earlier commit blocks the next author rather than helping anybody. Git history
holds every past entry: `git log -p -- test/weakened.md` shows the rows of any
commit beside the change they justified.

**Several sessions share this checkout, and this is one shared path.** Write your
rows immediately before you run `./le commit create`, then read the file again
between writing them and running the script. Rows written earlier are a window
for another session to replace them, and a session that writes with `cat >`
rather than an edit replaces the file whole. The refusal is the safe outcome. The
unsafe one is silent: your commit lands carrying another session's justification,
and no gate sees it, because the file is present and the row count is plausible.
Say so on the message bus before you take the slot.

A row here is the AUTHOR's own justification. The owner's approval for changing a
test that carries an `RFC requirement:` tag is a different file,
`test/rfc-changed.md`, and a row here does not authorize one there.

`parseLedger` (`internal/le/testweakened/ledger.go`) reads the first
`| Test | Reason |` table it finds and every table row under it, so this prose is
safe above the table. Do not write a second such header anywhere in the file: the
parser refuses two tables rather than guess which one the gate should read.

**A test in a NEW file needs no row in either ledger, and the two gates disagree
about that.** The commit gate reads the file at HEAD (`committedText` in
`internal/le/commit/rfcchange.go`) and skips a path with no HEAD version, so it
computes no change for a new file and REFUSES a row naming a test in one. The
write hook reads the file on disk instead (`Proposed` in
`internal/le/testweakened/proposed.go`, where `taggedCarrier` tests the current
`oldText`), so it DEMANDS a row before it lets you edit a new file that already
carries `RFC requirement:` tags. An author who obeys the hook is then refused by
the gate. Until one of them changes, write the tags in the same edit that creates
the file, and carry no row for it.

| Test | Reason |
|------|--------|
| rfc3748_test | Assertion count 58 -> 51, and every one of the seven belongs to a test function the owner approved deleting on 2026-09-02 (rows in `test/rfc-changed.md`). `TestRFC3748PeerNeverSendsNAK` asserted the behavior this spec corrects, that the peer errors rather than Naks on an unacceptable authentication Type, which RFC 3748 Section 5.3.1 forbids. `TestRFC3748IKEv2RequiresKeyDerivingMethod` asserted the IKEv2 keyless-method rule at `eap.NewSession`, the EAP framework's constructor, when the rule binds at `NewEAPSession` and `startEAPExchange` one layer up. No property was dropped: `RFC3748-2.1-3` is re-proven with both polarities in `internal/component/ike/eap/rfc3748_nak_test.go`, `RFC3748-7.10-3` with both polarities in `internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go` against the real producers, and the accepted-method set is swept over all 256 codes by `TestNewSessionAcceptsExactlyTheThreeConfiguredMethods` (`internal/component/ike/eap/method_set_test.go`). Every re-homed polarity carries a discrimination record. |
| rfc7296_method_test | Assertion count 7 -> 5, from deleting `TestRFC7296EAPKeylessMethodsAreRefused`, approved on 2026-09-02. It discharged `RFC7296-2.16-5` by proving the requirement's antecedent never fires, its own claim reading "A keyless method never starts, which keeps the SK_pi and SK_pr AUTH mode unreachable". Both halves are now false: `eapAuthSecret` (`internal/component/ike/engine/eap_auth.go`) implements the consequent and `authentication { mode eap-md5 }` reaches it. The tag moved to `TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr` and `TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK` (`internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go`), which drive the real AUTH construction over a real MD5-Challenge exchange, so the row is discharged by proof rather than by vacuity, which is what owner ruling OR-E of 2026-07-30 required. `eapmKeyDeriving`, a hand-written third copy of which methods derive keys, went with it; `eap.TypeDerivesKey` is the single declaration. |
| TestRFC3748PeerNeverSendsNAK | DELETED. Its own claim asserted the behavior this spec corrects: that the peer "errors (never NAKs) on an unexpected type" and "must error instead". RFC 3748 Section 5.3.1 requires the opposite, a legacy Nak for an unacceptable authentication Type (4-253, 255), so the test pinned a non-conformant path and the owner approved removing it (`test/rfc-changed.md`). Nothing left the suite. Its `RFC3748-2.1-3` tag is re-claimed with BOTH polarities by `TestRFC3748PeerNaksBeforeItCommitsToAMethod` (`internal/component/ike/eap/rfc3748_nak_test.go`), which pins Section 2.1's real boundary, and its `RFC3748-7.10-3` tag by the two tests in `internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go` at the producers that enforce it. The full-exchange half, that no Response of a driven conversation carries Type 3, is held by `TestPeerDoesNotNakATypeItHandles`. Every re-homed polarity carries a discrimination record. |
| TestRFC3748IKEv2RequiresKeyDerivingMethod | DELETED, owner approved. It asserted that `eap.NewSession` refuses Types 4, 5 and 6, which is the IKEv2 keyless-method rule checked at the EAP framework's constructor rather than at IKEv2's selection point. RFC 3748 Section 5 obliges that constructor to ACCEPT Type 4, so the assertion became false when MD5-Challenge landed. The rule itself is unchanged and better covered: `RFC3748-7.10-3` now has both polarities against `NewEAPSession` and `startEAPExchange` (`internal/component/ike/engine/rfc3748_ikev2_method_selection_test.go`), sweeping every `ipsec.AuthMode` with a non-vacuity counter per producer, which is coverage those producers never had. The accepted-method set it also pinned is swept over all 256 codes by `TestNewSessionAcceptsExactlyTheThreeConfiguredMethods` (`internal/component/ike/eap/method_set_test.go`). |
| TestRFC7296EAPKeylessMethodsAreRefused | DELETED, owner approved. It discharged `RFC7296-2.16-5` by proving the requirement's antecedent never fires: its claim read "A keyless method never starts, which keeps the SK_pi and SK_pr AUTH mode unreachable". Both halves are now false. `eapAuthSecret` (`internal/component/ike/engine/eap_auth.go`) implements the consequent and `authentication { mode eap-md5 }` reaches it. The tag moved to `TestEAPAuthOfNonKeyDerivingMethodUsesSKpiAndSKpr` and `TestEAPAuthOfKeyDerivingMethodStillUsesTheMSK` (`internal/component/ike/engine/rfc7296_eap_nonkeying_auth_test.go`), which drive the real AUTH construction over a real MD5-Challenge exchange, so the row is discharged by proof rather than by vacuity, which is what owner ruling OR-E of 2026-07-30 required. Its 256-code sweep is not lost: `TestNewSessionAcceptsExactlyTheThreeConfiguredMethods` carries it, untagged. |

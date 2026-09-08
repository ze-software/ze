# RFC-tagged test changes this session's commits carry

The owner's approval for changing a test that carries an `RFC requirement:` tag.
One row per test. An author MUST NOT write a row for their own change: the row
records Thomas's decision, and an author writing it is a forgery rather than a
shortcut (`docs/contributing/rfc-implementation-guide.md`).

## The approval

Thomas approved these two on 2026-09-08. He was shown the two test names, the
statement that they were red because they REQUIRED the L bit and read the TLS
record at a hard-coded offset 5, the statement that the edit changes the header
DECODE only, and the statement that no assertion about the alert's own behaviour
was weakened. He answered: "I approve".

## Why the change was owed

RFC 9190 Section 2.1.9: "Implementations MUST NOT set the L bit in unfragmented
messages, but they MUST accept unfragmented messages with and without the L bit
set."

`tlsFragmenter.nextFragment` (`internal/core/eap/eap_tls.go`) set `eapTLSFlagL`
and the four-octet length whenever the fragment was the FIRST one. A message that
fits in one fragment is both first and last, so ze set the L bit on every
unfragmented EAP-TLS message it emitted, on both roles. That is the MUST NOT.

Both tests below asserted the violating shape. Each opened with
`if len(td) < 1+4+tlsRecordHeaderLen || td[0]&eapTLSFlagL == 0 { t.Fatalf(...) }`
and then read the record at `td[5:]`. An EAP-TLS alert is 25 octets and never
fragments, so once the producer was corrected both tests failed, and they failed
because they were pinning the defect rather than because the fix was wrong.

`ai/rules/rfc-compliance.md` governs that case directly: a test that pins
non-conformant behaviour is the violation with a green bar on top, so the code is
fixed and then the test is corrected.

## What changed in them, exactly

The header DECODE, and nothing else. `tlsBytesFromTypeData` now finds the record
behind a one-octet or five-octet EAP-TLS header, which is what Section 2.1.9's
second clause obliges this side to read. The declared-length check survives,
conditioned on the L bit actually being set. A check that the M flag is clear was
ADDED, so each test now also proves it is looking at a whole alert rather than
one fragment of a message. No assertion about the alert's ordering against
EAP-Failure moved, which is what `RFC5216-2.1.3-4` is about.

| Test | Reason |
|------|--------|
| TestEAPTLSAuthenticatorSendsTheAlertBeforeItReportsTheFailure | `internal/core/eap/eap_tls_alert_flight_test.go`. Required the L bit and read the record at offset 5; both are properties of the defect this commit fixes. Header decode made flags-aware, declared-length check kept behind an L-bit test, M-flag check added. The RFC5216-2.1.3-4 assertion, that the authenticator puts the TLS alert on the wire before it reports the failure, is untouched. |
| TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure | Same file, same cause, same edit. It scans a whole flight for the first TLS record, and the scan's `len(p.TypeData) > 5 && p.TypeData[0]&eapTLSFlagL != 0` guard would have skipped every unfragmented record once the producer was corrected, so the test would have silently found no alert rather than failing loudly. The ordering assertion against EAP-Failure is untouched. |

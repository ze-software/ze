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
| TestHubFingerprintRejectsNonHex | Deleted with the leaf it tested. It asserted that `plugin hub client certificate-fingerprint` refuses a non-hex value, and that leaf no longer exists: `ca` replaces it and takes a name, not a digest. The refusal it proved is now stronger and lives elsewhere -- `TestFingerprintConfigIsRefused` in the same file asserts the WHOLE keyword is retired and that the error names its replacement, and `test/parse/pki-ca-fingerprint-retired.ci` drives the same refusal through `ze config validate`. A pattern test on a deleted leaf cannot be kept: there is nothing to feed it. |
| startRecordingHub | Renamed and rewritten, not weakened. `recordingHub` served a certificate the test then PINNED; the hub in this file now ISSUES a leaf from a root the test mints, because that is what the client validates against. `startIssuingHub` replaces it, keeps `wroteToken` unchanged, and every assertion the old helper supported is made by a named test below. |
| TestManagedClientPinsCertificate | Deleted with the mechanism it tested. `clientTLSConfig` no longer pins a fingerprint; it builds a pool from the configured `pki ca` name. `TestManagedClientValidatesAgainstConfiguredRoot` is the replacement and asserts more: the same successful fetch, plus that the anchor is the ISSUER, which the pin could not express. |
| TestManagedClientRefusesWrongCertificate | Same deletion, same replacement. `TestManagedClientRefusesAnotherIssuer` refuses a well-formed leaf a DIFFERENT root signed and asserts the client sent no token, which is the property the old test had plus the issuer dimension. |
| TestManagedClientDefaultFailsClosed | Deleted as a top-level function and kept as a subtest. `TestManagedClientRefusesWithNoAnchor/no-ca-configured` is the same scenario, assertion for assertion: no `CA`, no `TLSInsecure`, the client must not connect and must not write its token. A second subtest was added beside it for a named entry that does not resolve, which the old test had no way to express. |
| TestManagedClientFingerprintSources | Deleted with both sources it tested. It asserted that `ze.managed.tls.certificate-fingerprint` overrides the config leaf and that hex case does not matter. The environment variable is deleted, the leaf is deleted, and `certificateFingerprint` with its `strings.ToLower` is deleted. There is no precedence left to assert and no case folding left to fold. |
| selfSignedTLSConfig | One assertion fewer because one error fewer is returned. It called `GenerateSelfSignedCert()`, which returned `(tls.Certificate, error)`, and asserted `require.NoError`. That function is deleted; the helper now takes its pair from `testServerCert`, which makes both `require.NoError` calls itself. The assertion moved one call deep, exactly as the `handshakePeerCerts` row this table replaced described for another file. |
| TestTLSListenerMultiAddr | Same shape: `StartListeners` used to be fed by `GenerateSelfSignedCert()` and its error asserted here. The certificate now comes from `testServerCert`, which asserts it. The test's own subject is unchanged -- two addresses, two listeners, different ports -- and it additionally exercises the new `GetCertificate` signature. |
| TestGenerateSelfSignedCert | Deleted with `GenerateSelfSignedCert`. The function is gone from `internal/component/plugin/ipc/tls.go`; the plugin transport serves a leaf the daemon's certificate authority issued. `TestHubAcceptorServesAnIssuedLeaf` proves the replacement completes a real handshake, which is what this test proved about the old one. |
| TestCertFingerprintComputation | Deleted with `CertFingerprint`. Nothing computes a fingerprint on this rail any more. |
| TestCertFingerprintVerification | Deleted with `TLSConfigWithFingerprint`. `TestTLSConfigWithRootValidatesTheChain` is the replacement and validates a chain rather than comparing one digest. |
| TestCertFingerprintMismatch | Deleted with the same function. `TestTLSConfigWithRootRefusesAnotherIssuer` is the replacement refusal. |
| TestCertFingerprintFallback | Deleted, and it is the reason this spec exists. It asserted that an EMPTY fingerprint yields `InsecureSkipVerify`, which is the fail-open branch being removed. `TestTLSConfigWithRootRefusesWithNoAnchor` asserts the opposite outcome for the same input, and `test/plugin/plugin-dial-no-anchor.ci` drives it from a real plugin process. |
| hubCertMaterial | One return value fewer, so one assertion fewer. It used to compute and return the served certificate's fingerprint for `pinnedConfig`; nothing pins now, so the fingerprint and the `require.NoError` that guarded its computation are both gone. The certificate and key it returns are unchanged and every caller still asserts on them. |
| TestManagedServerServesConfiguredCertificate | Two assertions fewer because the pinning setup they guarded is gone. The test previously built a pinned client config from the fingerprint; it now dials with the CA as its anchor through `dialAnchored`, which makes its own assertions. What the test proves is unchanged and checked more directly: the certificate the listener serves is byte-identical to the configured one. |
| pinnedConfig | Deleted. It built a `tls.Config` that compared a SHA-256 of the peer leaf, which is the mechanism this spec removes. `anchoredConfig` replaces it and validates against a root, and the sibling tests below it exercise both the accept and the refuse side. |

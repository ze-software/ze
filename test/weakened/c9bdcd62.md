# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| TestDeriveTLSMSKFailsClosedOnIncompleteHandshake | RENAMED AND STRENGTHENED, not weakened. `deriveTLSMSK` became `deriveTLSKeys` when EAP-TLS started exporting the EMSK RFC 3748 Section 7.10 requires beside the MSK, so the test that pins its fail-closed guard follows the name: `TestDeriveTLSKeysFailsClosedOnIncompleteHandshake`. Every assertion the old body made survives unchanged, and it asserts one more. It still requires an error rather than a usable key on an incomplete handshake, still requires the MSK to come back all-zero, and now requires the same of the EMSK, because a zero key returned with no error is the valid-looking answer a caller cannot tell from a real one. Nothing this test prevented became possible. |

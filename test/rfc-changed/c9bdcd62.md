| Test | Reason |
|------|--------|
| TestRFC5282AEADRefusesAForbiddenICVLength | Owner approved, 2026-09-14. RETARGETED, no assertion changed. DecryptIKEAEAD became an AES-GCM-only binding of OpenIKEAEAD when AES CCM landed, and it has no non-test caller left, so the test drove a wrapper rather than the path production uses. It now drives OpenIKEAEAD directly and the binding is deleted. RFC5282-3.2-2 is proven in both polarities over the same inputs, against the producer the engine really calls. |
| TestRFC5282AEADOpensTheFullLengthICV | Owner approved, 2026-09-14. Same retarget, the negative arm of the pair. Same inputs, same outcome, driven at OpenIKEAEAD. RFC5282-3.2-2 negative is unchanged in what it asserts. |
| TestRFC5282AEADEncryptionKeysCarryTheirSalt | Owner approved, 2026-09-14. Same retarget: the salt round trip sealed with encryptIKEAEAD now seals with SealIKEAEAD, which is what the engine calls. RFC5282-7.1-2 positive asserts exactly what it asserted before. |

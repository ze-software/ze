# A quoted RFC sentence is not in the RFC

A comment or a summary row quotes an RFC as authority, and the quoted sentence
does not exist in the RFC text, often with a stronger keyword than the RFC uses,
so the code and the ledger claim an obligation the standard never stated.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | config-reference-completion (found, not caused) | RFC 5216 Section 5.3 quote in four ike/eap comments and the `rfc/short/rfc5216.md` prose table | They quote "Both sides MUST perform certificate path validation." which is not in the RFC; Section 5.3 says server and peer SHOULD support RFC 3280 path validation | fixed: comments quote the SHOULD sentences byte-exact and say Ze makes it mandatory; prose table and the 2026-07-20 changelog corrected; checklist rows were already SHOULD, no figure changed |

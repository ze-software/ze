---
kind: note
level: MUST
stage:
---
The `check.py` MUST:
1. Wait for session establishment (`wait_session`)
2. Assert the specific protocol behavior being tested (route presence, capability negotiation, etc.)
3. Verify session stability after the exchange (`session_established`)
4. Use `log_pass`/`log_fail` for clear output
5. Raise on failure (AssertionError or RuntimeError)

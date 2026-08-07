---
kind: note
level:
stage:
---
External plugins connect back to the engine's TLS listener. Auth is stage 0: `#0 auth {"token":"...","name":"..."}`. Each forked plugin receives a unique random token bound to its name. A plugin cannot use its token to impersonate another. The token is cleared from the OS environment after first read (`Secret: true` on the env registration). The engine also passes its TLS certificate SHA-256 fingerprint via `ZE_PLUGIN_CERT_FP` so the SDK verifies the server identity during the TLS handshake.

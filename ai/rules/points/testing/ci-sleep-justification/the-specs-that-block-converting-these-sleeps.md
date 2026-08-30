---
kind: directive
level: MUST NOT
stage:
---
- The external-plugin refuse/warn sleeps wait on a reject-fence signal the daemon does not emit, so no deterministic wait exists for them yet and they MUST NOT be converted.

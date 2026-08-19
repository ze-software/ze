# Replacement written before it is validated

A command replaces a working artifact with material it never checked. The write
succeeds, the command reports success, and the artifact it destroyed is gone.
Nothing fails at the command, because the command is not the consumer: the
failure surfaces one or more hops later, at the boot, the push or the render
that finally reads what was written.

Two shapes carry it. The first is a write with no validation between the read
and the write. The second is a multi-file write with no rollback, where a
failure part way through leaves a set that no longer agrees with itself.

The fix is one write path, not a guard at each entry point. A guard added to
one site is a guard the next site will be missing.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | fixit-appliance-cert-replace-unvalidated | appliance TLS material | `runReplaceCert` (`internal/appliance/cmd_cert.go`) wrote `cert.pem` with a truncating write and `key.pem` through the atomic secret writer. Nothing between the read and the writes parsed the PEM, checked that the certificate and the key belonged together, or took a backup. `writeTLSSecrets` (`internal/appliance/cmd_init.go`) repeated the same two-write shape at initialization, and `--cert` without `--key` fell through to self-signed regeneration. A mismatched pair reported success, the assemble step copied it into the image, and the installed appliance disabled its web listener behind one warning line | `validateTLSPair` calls `tls.X509KeyPair` before either file is touched, and `writeTLSPair` writes both halves through `WriteSecret` and restores the previous certificate when the key write fails (`internal/appliance/cmd_cert.go`). `runReplaceCert` writes nothing itself and delegates to `writeTLSSecrets`, so one write path serves both entry points. `checkWebTLSPair` (`internal/component/doctor/checks_tls.go`) reports a pair an older `ze` had already stored |

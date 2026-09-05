# Option read after the decision it governs

An argument loop reads the positional value, acts on it, and only then walks the
keyword options. Every option that was meant to CONSTRAIN that action arrives
too late to do it. Nothing is dropped and nothing is unvalidated, so no guard
and no test sees a problem: the option is parsed, stored, and used for the
smaller job it also has. The operator gets an answer that ignored what they
asked for, and the failure surfaces one layer down, where the two values meet
and disagree, naming neither of them.

The tell is a handler whose first statement already reaches the network, a
socket, or the disk, with the option loop under it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | traceroute-source-af | `resolve ping`, `handleResolvePing` (`internal/component/ping/cmd/resolve.go`) | The handler resolves the target with `probe.ResolveTarget` before its argument loop reads `source`, so a source address cannot constrain the family the target name resolves in. An IPv6 source against a dual-stack name reaches an IPv4 destination, and the conflict surfaces at `net.ListenPacket("ip4:icmp", "<v6 source>")` as a bind failure that names the socket and neither argument. This is the same defect `handleResolveTraceroute` carried, in the sibling command | not fixed. `spec-traceroute-source-af` scoped its fix to the traceroute path and left ping for a user decision; that spec is closed and removed, so the scope decision is restated here rather than cited. The seam it needs is already built: `probe.ResolveTarget` takes a `probe.Family` and `probe.FamilyOf` derives one from a source address, so the fix is the same reorder plus one call |

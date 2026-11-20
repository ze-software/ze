# A definite non-delivery is kept as an unknown outcome

A sender records the outcome of each write it hands to a lower layer, and
treats any outcome it cannot classify as unknown: the write may or may not
have reached the peer, so later work waits until someone resolves it. The
lower layer answers with a refusal that is in fact definite (the write was
never sent, because its target no longer exists), but the classifier does not
recognise that refusal, so it is stored as unknown. Nothing resolves an
unknown entry whose target is gone, and every later operation that asks
whether all output settled fails for good.

The tell is a failure that names a session or target that has already ended,
and that repeats on every retry until the process restarts.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-20 | pipe-chain-honored-on-every-command-surface | bgp-rs `joinedOutputError` (`plugins/rs/source_lifetime.go`) | rs-control-community-strip.ci red at debda0c3dc: both sessions close on the forwarded UPDATE, the withdrawal toward the gone receiver is refused errAdjOutSession, which joinedOutputError does not count as settled; rs.unknownOutputs keeps it (never deleted), so every settlement and request quiesce fails "output completion unknown" | open: count a retired-session refusal as definite non-delivery, or drop the entry when its session retires; check rs-community-strip-multi |

---
spec: ipsec-dual-form-receive-spd-bypass
topic: IPsec
feature: ikev2-engine
found-by: review
---

## Before

The receiver for bare ESP refilled its complete-path token bucket too early,
permitting a full burst after one second. Reader replacement or retirement
could also lose a recorded failure event or reset a replacement's recovery
backoff. After the last IPsec peer was removed, the doctor could still report
the receiver's retained device failure. The `ze plugin ike` entrypoint exited
before starting its registered engine.

An ESP-group-only reload could restart an established IKE SA with no Child SA
before its owner loop was ready.

## After

Ze refills the token bucket at 5000 tokens per second, up to its existing
10000-token burst. Reader retirement and failure publication share a per-reader
lock, so a recorded failure remains reportable once and intentional retirement
stays silent. An old reader cannot reset its replacement's backoff.

The doctor reports the retained device failure while an IPsec peer is
configured. Removing the last peer silences that finding. The receiver retains
its failure record until a later open succeeds. The external IKE entrypoint
now uses the shared plugin runner and authenticated TLS connection to the hub,
so runtime doctor findings can come from the IKE child process.

For an ESP-group-only reload, Ze now gives an attached, established IKE SA's
owner loop up to two seconds to accept the request. If the owner accepts the
new group for a childless SA, Ze keeps the session and both IKE SPIs. The
existing restart rules still apply when a Child SA is installed, a responder
handshake is pending, or Child SA creation is in progress. A request that
misses the two-second deadline also requires a restart.

## Corrects

none

## Exposure

The refill defect affected bare ESP packets served by Ze's complete path.
Packets handled by the kernel without that receiver did not use this bucket.

## Upstream

none

## Asked by

none

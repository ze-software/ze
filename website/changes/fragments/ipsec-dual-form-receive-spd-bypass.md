---
spec: ipsec-dual-form-receive-spd-bypass
topic: IPsec
feature: ikev2-engine
found-by: review
---

## Before

On an IPv4 Child SA moved to UDP 4500 without NAT, Ze re-presented bare ESP
through loopback. Linux then skipped inbound policy checks. An authenticated
peer could deliver an inner packet outside that SA's traffic selectors.

The receiver for bare ESP refilled its complete-path token bucket too early,
permitting a full burst after one second. Reader replacement or retirement
could also lose a recorded failure event or reset a replacement's recovery
backoff. After the last IPsec peer was removed, the doctor could still report
the receiver's retained device failure. The `ze plugin ike` entrypoint exited
before starting its registered engine.

An ESP-group-only reload could restart an established IKE SA with no Child SA
before its owner loop was ready.

## After

Ze receives bare ESP through a TCX rewrite on the incoming link or its
policy-enabled `ze-espform` device. Both paths retain the receiving SA's
selector check and the inbound SPD decision. Each declared state carries its
complete directional selector set. Ze refuses installation when its native
kernel cannot confirm the required scope and fragment capabilities.

Normal Ze-kernel packet tests cover permitted and refused fragments,
separation between clear and protected fragments and between different SAs,
and port-sensitive disposition. These bounded observations do not establish
full RFC 4301 conformance.

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

The policy bypass affected bare ESP received on an IPv4 ESP-in-UDP Child SA
that used Ze's former loopback re-presentation path. The peer needed the
SA's valid keys. ESP already received in the kernel's configured wire form
did not take that path. Ze still refuses IPv6 outer dual-form SAs.

The refill defect affected bare ESP packets served by Ze's complete path.
Packets handled by the kernel without that receiver did not use this bucket.

## Upstream

none

## Asked by

none

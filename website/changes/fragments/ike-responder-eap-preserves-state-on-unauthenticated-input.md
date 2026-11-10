---
spec: ike-responder-eap-preserves-state-on-unauthenticated-input
topic: IPsec
feature: ipsec-eap-authentication
---

## Before

When Ze answered an IKEv2 connection, one forged IKE_AUTH datagram ended the
handshake. Anyone who saw the first exchange on the path could copy its two
SPIs and the next Message ID into a datagram whose checksum did not verify,
and Ze dropped the half-built IKE SA as if the peer had failed. The client's
genuine next message then found nothing to continue, so the tunnel did not
come up. The second authentication round of a multiple-authentication
exchange kept the handshake, but wrote one WARN log line per forged datagram.

## After

A request that arrives without valid cryptographic protection during the
responder handshake is dropped and the handshake goes on: the first
IKE_AUTH, each EAP round, and the second authentication round all keep their
state, send nothing, and log at debug only. Each drop is counted in
`ze_ipsec_error_notify_suppressed_total` under the reason
`handshake-unauthenticated`. A request that does verify but whose content
does not parse still ends the handshake, and a handshake that never
completes is still removed after 30 seconds, however many forgeries arrive.
Against strongSwan as an EAP-MSCHAPv2 client, twenty forged requests are
counted and the tunnel then establishes and passes traffic.

## Corrects

none

## Exposure

Every Ze IKEv2 responder, with any authentication method, was exposed during
the handshake to an attacker able to observe its first exchange. An
established IKE SA was not affected. When Ze initiates, a forged IKE_AUTH or
EAP response can still end the handshake; that half is not fixed yet.

## Upstream

none

## Asked by

none

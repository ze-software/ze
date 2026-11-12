---
spec: single-owner-lifecycle-4-ike-unauthenticated-input
topic: IPsec
feature: ipsec-eap-authentication
---

## Before

When Ze initiated an IKEv2 connection, one forged IKE_AUTH response ended the
handshake. Anyone who saw the first exchange on the path could copy its two
SPIs and the Message ID into a datagram whose checksum did not verify, and Ze
dropped the half-built IKE SA as if the peer had failed, then waited out a
reconnect backoff before starting again. This held for the first IKE_AUTH
response and for each EAP round. In the second round of a
multiple-authentication exchange the handshake was kept, but each forged
datagram wrote one WARN log line. On an established IKE SA such datagrams
were already dropped, but three of the places that drop them counted nothing.

## After

A response that arrives without valid cryptographic protection during the
initiator's handshake is dropped and the handshake goes on: the first
IKE_AUTH response, each EAP round and the second authentication round keep
their state, send nothing and log at debug only. Ze keeps retransmitting its
request on the usual schedule, and the peer's genuine answer then
establishes the same IKE SA. Each drop is counted in
`ze_ipsec_error_notify_suppressed_total` under the reason
`handshake-unauthenticated`, the same reason the responder uses, and a drop
on an established IKE SA is now counted under `established-unauthenticated`.
`ze doctor` warns (`ipsec-unauthenticated-input`) for a peer whose handshake
has not come up and has dropped such messages, and warns
(`ipsec-peer-malformed-message`) for a peer whose authenticated messages do
not parse. A response that does verify but whose content does not parse is
treated as before.

## Corrects

none

## Exposure

Every Ze IKEv2 initiator, with any authentication method, was exposed during
the handshake to an attacker able to observe its first exchange. An
established IKE SA was not affected, and the responder half was fixed
earlier.

## Upstream

none

## Asked by

none

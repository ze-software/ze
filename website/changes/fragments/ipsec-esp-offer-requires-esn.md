---
spec: ipsec-esp-offer-requires-esn
topic: IPsec
feature: ikev2-engine
found-by: review
---

## Before

When Ze answered an IKEv2 peer, it accepted an ESP proposal that carried no
Extended Sequence Numbers transform, although IKEv2 makes that transform
mandatory for ESP. Ze then answered with an ESN transform the peer had never
offered, so a strict peer could refuse the answer and the Child SA did not
come up, with nothing in Ze's logs pointing at the peer's incomplete offer.

## After

Ze treats an ESP proposal without an Extended Sequence Numbers transform as
unacceptable and moves on to the peer's next proposal. When no proposal is
left it answers NO_PROPOSAL_CHOSEN, the IKEv2 answer for an offer it cannot
accept. This holds for the Child SA created during authentication, for a new
Child SA, and for a Child SA rekey. Peers that offer ESN, as Ze itself
does, see no change.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
